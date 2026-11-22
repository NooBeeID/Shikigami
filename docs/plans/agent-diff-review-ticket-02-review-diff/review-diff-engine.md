# Plan: Mesin diff dan IPC review.diff

## Metadata

- Tiket: `agent-diff-review-ticket-02-review-diff` (`docs/tickets/agent-diff-review-ticket-02-review-diff.md`)
- Fitur: `review-diff-engine`
- ownedPaths: `internal/review/diff.go`, `internal/review/diff_ipc.go`
- Depends on foundation: tidak (tiket ini tanpa Foundation; hanya bergantung pada kode tiket 01 yang sudah ter-merge dan kontrak overview bagian 2)
- Dokumen acuan: `docs/plans/agent-diff-review-ticket-02-review-diff/overview.md` (bagian 2.1, 2.2, 2.3, 2.7, 6, 7). Jangan diubah.

## Tujuan

Menyediakan sisi Go dari diff review di paket `internal/review`, murni tanpa dependency baru:

1. `ComputeDiff` (Myers dengan batas biaya, konteks 3 baris, gabung hunk, id hunk stabil, deteksi biner/terlalu besar, hash isi sekarang).
2. `DiffForFile` yang membaca baseline lewat `Store` tiket 01 dan isi file sekarang.
3. IPC `review.diff` (`RegisterDiffIPC`) dan `FillDiffStats` (pengisi `additions/deletions/binary/tooLarge` untuk `review.files`, dengan cache memori).

Fitur ini hanya berkompilasi dan menyediakan fungsi publik. Penyambungan ke `ipc.go` dan `wire_review.go` dikerjakan langkah Wiring, bukan di sini.

## Path yang dimiliki

HANYA dua file ini yang boleh dibuat/diubah:

- `internal/review/diff.go`
- `internal/review/diff_ipc.go`

Dilarang menyentuh: `internal/review/{types,store,capture,ipc}.go`, `cmd/shikigamid/wire_review.go` (file wiring), seluruh `frontend/` (milik fitur `review-diff-ui`), `internal/fsx/*`, `internal/ipc/*`, `go.mod`, `go.sum`. Tidak menulis unit test, tidak menjalankan `git commit`.

## Kontrak

Sumber kebenaran: overview bagian 2.2 dan 2.3. Ringkasan nama yang HARUS persis (tag JSON camelCase):

Disediakan (publik):
- `const MaxDiffBytes = 1 << 20`, `const ContextLines = 3`
- `type DiffLine {Kind, Text string; OldNo, NewNo int}` (json `kind,text,oldNo,newNo`)
- `type Hunk {ID string; OldStart, OldLines, NewStart, NewLines int; Lines []DiffLine}` (json `id,oldStart,oldLines,newStart,newLines,lines`)
- `type DiffResult {Hunks []Hunk; CurrentHash string; Binary, TooLarge bool; Additions, Deletions int}` (json `hunks,currentHash,binary,tooLarge,additions,deletions`)
- `func ComputeDiff(old, cur []byte) DiffResult`
- `func DiffForFile(st *Store, sessionID, path string) (DiffResult, error)`
- `const IPCMethodDiff = "review.diff"` (di `diff_ipc.go`, bukan `types.go`)
- `func RegisterDiffIPC(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool)`
- `func FillDiffStats(st *Store, sessionID string, files []FileEntry)`

Dipakai dari tiket 01 (paket yang sama; jangan diubah):
- `Store.Touched(sessionID) []TouchedFile`, `Store.Cwd(sessionID) string`, `Store.BaselinePath(sessionID, relPath) string`
- `TouchedFile` (`Path`, `ExistedBefore`, `BaselineSha256`, `Binary`, `TooLarge`), `FileEntry` (`Path`, `Additions`, `Deletions`, `Binary`, `TooLarge`, ...)
- helper privat: `storeHashBytes([]byte) string`, `storeHashFile(path string) (string, error)`, `storeValidID(string) bool`, `captureSample(path string) ([]byte, error)` (baca <= 8 KiB pertama)
- `fsx.ResolveUnder(root, rel) (string, error)`, `fsx.IsBinary(sample []byte) bool` (impor `github.com/NooBeeID/shikigami/internal/fsx`)
- `ipc.Server.Register(method, func(ctx context.Context, params json.RawMessage) (any, error))`, `ipc.NewError(ipc.ErrInvalid | ipc.ErrInvalidParams, msg)` (impor `github.com/NooBeeID/shikigami/internal/ipc`); pola persis seperti `RegisterIPC` di `internal/review/ipc.go` (baca file itu sebagai contoh gaya, jangan diubah).

Fakta penting: untuk baseline biner / > 1 MiB / file baru tidak ada salinan di `BaselinePath`, sehingga flag entri diperiksa dulu. Salinan hilang untuk entri `ExistedBefore=true` non-biner non-besar adalah error.

## Langkah

Konvensi: komentar bahasa Indonesia; helper privat berawalan `diff` (mis. `diffSplitLines`, `diffMyers`, `diffGroupHunks`, `diffHunkID`) agar tidak bentrok dengan `store*`/`capture*`; pesan error Indonesia. Sebelum menulis, baca sekilas `internal/review/{store,capture,ipc,types}.go` untuk memastikan nama aktual masih sesuai; bila berbeda, ikuti kode dan catat di laporan.

### Langkah 1 - `internal/review/diff.go`: tipe dan konstanta

Buat file `package review` dengan impor `crypto/sha256`, `encoding/hex`, `errors`/`fmt`, `io`, `os`, `strconv` (secukupnya), `github.com/NooBeeID/shikigami/internal/fsx`. Deklarasikan `MaxDiffBytes`, `ContextLines`, `DiffLine`, `Hunk`, `DiffResult` persis seperti kontrak (tag JSON sama). Tambah konstanta privat `diffMaxCost = 2000` (batas D Myers; nilai ini menjaga memori trace sekitar D^2 int dan waktu O(N*D) wajar untuk file ~1 MB; boleh disetel bila perlu, jelaskan di komentar).

### Langkah 2 - pemecah baris

`func diffSplitLines(b []byte) []string`:
- `len(b)==0` -> `nil` (nol baris).
- Pisah dengan `\n`; bila isi berakhir `\n`, buang elemen kosong terakhir. Baris tanpa newline akhir diperlakukan biasa (tanpa penanda khusus).
- `Text` menyimpan baris apa adanya (termasuk `\r` bila ada) tanpa `\n`.
- Kunci perbandingan: `diffKey(line) = strings.TrimSuffix(line, "\r")` (normalisasi CRLF hanya untuk perbandingan). Siapkan slice kunci paralel untuk Myers; atau cukup lakukan perbandingan lewat `diffKey` bila CRLF terdeteksi. Pakai pemetaan baris ke int id (map string->int) agar perbandingan Myers O(1) per langkah.

### Langkah 3 - Myers dengan batas biaya

`func diffMyers(a, b []int) (ops []diffOp, ok bool)` dengan `type diffOp struct{ kind byte; ai, bi int }` (`kind` `'='`, `'-'`, `'+'`; `ai`/`bi` indeks 0-based di a/b yang bersangkutan).
- Pertama pangkas prefiks dan sufiks yang sama (`=`) untuk memperkecil masalah; jalankan Myers pada bagian tengah.
- Algoritma Myers klasik (greedy forward, array `v` berukuran `2*dmax+1` dengan offset), simpan salinan `v` per `d` untuk backtrack (simpan hanya rentang `[-d, d]`). Bila `d` melebihi `diffMaxCost` sebelum mencapai ujung -> kembalikan `ok=false`.
- Backtrack menghasilkan urutan operasi maju (balik hasil mundur).
- Kasus a kosong atau b kosong ditangani langsung (semua `+` atau semua `-`) tanpa Myers.
- Fallback (`ok=false`): `ComputeDiff` memakai "semua baris lama `-`, lalu semua baris baru `+`" sebagai ops; hasilnya tetap lewat pengelompokan hunk yang sama (tanpa konteks bersama, satu hunk). Pastikan fallback tidak mengalokasikan memori kuadrat.

### Langkah 4 - pengelompokan hunk dan nomor baris

`func diffGroupHunks(ops []diffOp, oldLines, newLines []string) []Hunk`:
1. Bangun daftar `DiffLine` datar dari ops dengan `OldNo`/`NewNo` berbasis 1: `=` -> `ctx` (keduanya), `-` -> `del` (`NewNo=0`), `+` -> `add` (`OldNo=0`). `Text` dari baris asli. Hitung `Additions`/`Deletions` di `ComputeDiff` dari sini.
2. Cari indeks op yang bukan `=`. Tanpa perubahan -> `[]Hunk{}`.
3. Kelompokkan perubahan berurutan: dua perubahan digabung bila jumlah baris `ctx` di antaranya <= `2*ContextLines` (6). Rentang hunk = dari `(perubahanPertama - ContextLines)` sampai `(perubahanTerakhir + ContextLines)` dalam indeks op, dijepit ke `[0, len-1]`. Konteks maksimum 3 baris di tiap ujung.
4. Isi `OldStart/OldLines/NewStart/NewLines`: `OldLines` = jumlah `ctx`+`del`, `NewLines` = jumlah `ctx`+`add` di hunk. `OldStart` = `OldNo` baris pertama yang memuatnya; bila `OldLines==0` maka `OldStart=0` (sisi kosong; konvensi unified diff). Sama untuk `NewStart`. Untuk sisi tidak kosong berbasis 1.
5. Jika hunk punya `OldLines>0` tetapi baris pertamanya `add` (tanpa konteks di depan), `OldStart` = `OldNo` baris lama pertama di hunk; jika tidak ada sama sekali, 0. Gunakan pencarian `OldNo>0` pertama, bukan asumsi posisi.
6. `ID = diffHunkID(h)`.

`func diffHunkID(h Hunk) string`: sha256 atas string `"<oldStart>,<oldLines>,<newStart>,<newLines>\n"` diikuti, per baris, `Kind + "\x00" + Text + "\n"`; kembalikan 12 hex pertama (`hex.EncodeToString(sum[:])[:12]`). Deterministik: input sama -> id sama; isi berbeda -> id berbeda. Hitung id setelah `Lines` terisi.

### Langkah 5 - `ComputeDiff`

`func ComputeDiff(old, cur []byte) DiffResult` (murni, tanpa IO):
1. `res := DiffResult{Hunks: []Hunk{}}`. `CurrentHash = ""` bila `cur == nil`, selain itu `storeHashBytes(cur)` (hash dihitung walau biner/besar; `cur` kosong non-nil tetap di-hash).
2. `Binary`: `fsx.IsBinary(old)` atau `fsx.IsBinary(cur)` (fungsi itu sendiri hanya memeriksa 8 KiB pertama). `TooLarge`: `len(old) > MaxDiffBytes || len(cur) > MaxDiffBytes`. Jika salah satu true -> kembalikan hasil tanpa hunk (hash tetap terisi, `Additions/Deletions=0`).
3. Pecah baris kedua sisi, petakan ke id int, jalankan `diffMyers`; bila `!ok` pakai fallback. Kelompokkan hunk, hitung `Additions/Deletions`.
4. Jaminan: `ComputeDiff(nil, x)` -> satu hunk semua `add` (`OldStart/OldLines=0`); `ComputeDiff(x, nil)` -> satu hunk semua `del` (`NewStart/NewLines=0`); `ComputeDiff(x, x)` -> `Hunks` kosong non-nil; kedua sisi kosong -> tanpa hunk. Tidak panik pada input apa pun.
5. Catatan hash: untuk `old == x` dan `cur == nil` (terhapus) `CurrentHash` kosong.

### Langkah 6 - `DiffForFile`

`func DiffForFile(st *Store, sessionID, path string) (DiffResult, error)`:
1. Validasi: `storeValidID(sessionID)` dan `path != ""`, selain itu error jelas ("sessionId tidak valid", "path wajib diisi").
2. Cari entri di `st.Touched(sessionID)` dengan `Path == path`; tidak ada -> error `"file tidak ada dalam daftar review sesi ini: <path>"`. `cwd := st.Cwd(sessionID)`; kosong -> error `"cwd sesi tidak diketahui"`.
3. `abs, err := fsx.ResolveUnder(cwd, path)`; error dibungkus `fmt.Errorf("path tidak valid: %w", err)`.
4. `os.Stat(abs)`: tidak ada (`os.IsNotExist`) -> `curExists=false`; ada tetapi bukan file biasa -> error `"bukan file biasa"`; galat lain -> error.
5. Hash: bila `curExists`, `CurrentHash, err = storeHashFile(abs)` (streaming, dipakai pada jalur biner/besar). Bila `!curExists`, hash `""`.
6. Penentuan tanpa hunk, tanpa membaca isi penuh: `tooLarge := entry.TooLarge || (curExists && size > MaxDiffBytes)`; `binary := entry.Binary`; bila `curExists && !tooLarge`, sampel `captureSample(abs)` lalu `binary = binary || fsx.IsBinary(sample)`. Jika `binary || tooLarge` -> kembalikan `DiffResult{Hunks: []Hunk{}, CurrentHash: hash, Binary: binary, TooLarge: tooLarge}` (nol error, tidak menyentuh salinan baseline).
7. Baseline: bila `entry.ExistedBefore`, `os.ReadFile(st.BaselinePath(sessionID, path))`; gagal -> error `"baseline tidak ditemukan: ..."` (kasus ini memang error). Bila `!entry.ExistedBefore`, `old = nil`.
8. Isi sekarang: `curExists` -> `os.ReadFile(abs)` (galat -> error); setelah baca, bila `len(cur) > MaxDiffBytes` (file tumbuh di antara Stat dan baca) -> kembalikan `TooLarge:true` tanpa hunk dengan hash dari byte yang dibaca. Tidak ada -> `cur = nil`.
9. `res := ComputeDiff(old, cur)`; timpa `res.CurrentHash` dengan hash dari langkah 5 hanya bila `curExists` dan tidak ada ketidakcocokan; lebih sederhana dan konsisten: pakai hash dari byte yang benar-benar dibaca (hasil `ComputeDiff` sudah menghitungnya), dan buang hasil hash streaming pada jalur normal. Kembalikan `res, nil`.

### Langkah 7 - `internal/review/diff_ipc.go`: IPC `review.diff`

`package review`; impor `context`, `encoding/json`, `strings`, `sync`, `os`, `fsx`, `ipc`.

- `const IPCMethodDiff = "review.diff"`.
- `type diffResponse struct { Hunks []Hunk \`json:"hunks"\`; CurrentHash string \`json:"currentHash"\`; Binary bool \`json:"binary"\`; TooLarge bool \`json:"tooLarge"\` }` (tanpa additions/deletions di respons IPC).
- `func RegisterDiffIPC(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool)` memakai `srv.Register(IPCMethodDiff, ...)`:
  1. Decode `{sessionId, path}`; gagal -> `ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh {\"sessionId\": string, \"path\": string}")`.
  2. `sessionId` dan `path` di-trim; salah satu kosong -> `ipc.NewError(ipc.ErrInvalid, "sessionId wajib diisi")` / `"path wajib diisi"`.
  3. `supportedFn == nil || !supportedFn(id)` -> `diffResponse{Hunks: []Hunk{}}` (bukan error).
  4. `DiffForFile`; galat -> `ipc.NewError(ipc.ErrInvalid, "gagal menghitung diff: "+err.Error())` (mencakup path di luar cwd, sesi/entri tak dikenal).
  5. Bentuk `diffResponse`; pastikan `Hunks` non-nil.

### Langkah 8 - `FillDiffStats` dengan cache

Di `diff_ipc.go`:
- Cache paket-level: `type diffCacheKey struct{ session, path, hash, baseline string }`, `type diffCacheVal struct{ add, del int; binary, tooLarge bool }`, `var diffCache = struct{ mu sync.Mutex; m map[diffCacheKey]diffCacheVal }{m: map...}`. Kunci kontrak adalah `(sessionId, path, currentHash)`; tambahkan `baselineSha256` entri ke kunci sebagai pengaman bila baseline kelak berubah (superset aman; catat di komentar). Batasi ukuran (mis. 4096 entri): bila terlampaui, kosongkan map.
- `func FillDiffStats(st *Store, sessionID string, files []FileEntry)`:
  1. `st == nil` atau `len(files)==0` atau id tidak valid -> return (aman untuk nil/kosong).
  2. `cwd := st.Cwd(sessionID)`; kosong -> return. Buat peta `path -> TouchedFile` dari `st.Touched` sekali.
  3. Untuk tiap indeks `i`: ambil entri; tidak ada -> lanjut. `abs, err := fsx.ResolveUnder(cwd, files[i].Path)`; galat -> lanjut. Hash saat ini: `os.Stat`; ada -> `storeHashFile(abs)`, tidak ada -> `""`; galat -> lanjut (file tetap 0/0).
  4. Cek cache dengan kunci `(sessionID, path, hash, entry.BaselineSha256)`; hit -> pakai nilainya. Miss -> `DiffForFile`; galat -> lanjut tanpa mengisi dan tanpa cache. Sukses -> simpan nilai (`Additions`, `Deletions`, `Binary`, `TooLarge`) ke cache.
  5. Tulis in place: `files[i].Additions = add`, `files[i].Deletions = del`, `files[i].Binary = files[i].Binary || binary`, `files[i].TooLarge = files[i].TooLarge || tooLarge`.
  6. Tidak pernah mengembalikan error dan tidak panik; satu file gagal tidak memengaruhi file lain. Pertimbangkan `recover` per file hanya bila perlu; utamakan penanganan galat eksplisit.

### Langkah 9 - guard

Jalankan `gofmt -l internal/review` (harus kosong untuk dua file ini), `go vet ./internal/review/` bila cepat, lalu `go build ./...` dari root repo. Perbaiki hanya galat di dua file milik fitur ini. Verifikasi cepat opsional tanpa menulis file test di repo: skrip sekali pakai di luar repo (scratchpad) tidak wajib; cukup tinjau logika terhadap kriteria di bagian "Selesai bila".

## Perubahan

- Dibuat: `internal/review/diff.go`
- Dibuat: `internal/review/diff_ipc.go`
- Tidak ada file lain yang dibuat atau diubah (tanpa `go.mod`, tanpa test, tanpa wiring).

## Wiring

Yang diekspos untuk disambungkan nanti oleh langkah Wiring (jangan disambungkan oleh fitur ini):

1. `review.RegisterDiffIPC(srv, store, reviewWireSupported(eng))` — satu baris di `cmd/shikigamid/wire_review.go`, tepat setelah `review.RegisterIPC(...)`.
2. `FillDiffStats(store, id, files)` — satu panggilan di handler `review.files` pada `internal/review/ipc.go`, sebelum `return FilesResult{Supported: true, Files: files}`.

Fitur ini tidak mengedit kedua file itu. Sampai Wiring, kedua fungsi publik tidak dipanggil siapa pun tetapi tetap berkompilasi.

## Selesai bila

- `internal/review/diff.go` dan `internal/review/diff_ipc.go` ada, terformat `gofmt`, dan hanya dua file itu yang berubah.
- `go build ./...` dari root repo sukses (satu-satunya guard; tanpa unit test; tanpa dependency baru di `go.mod`/`go.sum`).
- Perilaku sesuai kontrak (diperiksa lewat tinjauan kode):
  - `ComputeDiff(nil, x)` satu hunk semua `add`; `ComputeDiff(x, nil)` semua `del`; `ComputeDiff(x, x)` tanpa hunk; `Hunks` selalu non-nil.
  - `oldNo`/`newNo` benar (`ctx` keduanya, `add` hanya `newNo`, `del` hanya `oldNo`); sisi kosong memberi start/lines 0.
  - Perubahan berjarak <= 6 baris satu hunk, lebih jauh dua hunk; konteks maksimum 3 baris.
  - Id hunk 12 hex, deterministik, berbeda untuk isi berbeda.
  - Biner (NUL / non-UTF-8 lewat `fsx.IsBinary`) -> `binary:true`; > `MaxDiffBytes` -> `tooLarge:true`; keduanya tanpa hunk namun `currentHash` tetap terisi (hash streaming pada `DiffForFile`).
  - Myers dengan batas `diffMaxCost` dan fallback hapus-semua/tambah-semua; tidak menggantung pada file mendekati 1 MiB.
  - `DiffForFile` memeriksa flag entri sebelum membaca salinan baseline (file baru/biner/besar tidak error), memvalidasi path lewat `fsx.ResolveUnder`, dan memberi error jelas untuk sesi/entri/cwd tak dikenal atau path di luar cwd.
  - `review.diff` mengembalikan `{hunks,currentHash,binary,tooLarge}` dengan `hunks` array; sesi tidak didukung -> hasil kosong, bukan error; params salah -> `ipc.ErrInvalidParams`; input kosong/tidak valid -> `ipc.ErrInvalid` berbahasa Indonesia.
  - `FillDiffStats` mengisi in place, OR untuk `Binary`/`TooLarge`, memakai cache memori, toleran galat per file, aman untuk nil/kosong, tanpa return error.
- Berjalan di folder non-git (tidak memakai `git`/`internal/gitdata`).
