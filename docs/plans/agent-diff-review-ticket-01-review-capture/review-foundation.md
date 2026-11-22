# Plan: Tipe, penyimpanan baseline, dan Capture

## Metadata

- Tiket: `agent-diff-review-ticket-01-review-capture` (`docs/tickets/agent-diff-review-ticket-01-review-capture.md`)
- Fitur: `review-foundation`
- ownedPaths: `internal/review/types.go`, `internal/review/store.go`, `internal/review/capture.go`
- Depends on foundation: ya. Foundation (`00-foundation.md`) sudah membuat `types.go` final dan stub `store.go`/`capture.go`. Bila berkas itu belum ada saat plan ini dikerjakan, buat dulu sesuai `overview.md` 2.3 dan `00-foundation.md`, lalu lanjutkan langkah di bawah.
- Rujukan wajib: `docs/plans/agent-diff-review-ticket-01-review-capture/overview.md` (terutama 2.1, 2.2 butir 5-6, 2.3, bagian 7). Jangan mengubah overview.

## Tujuan

Mengganti stub `Store` dengan implementasi penuh: penyimpanan baseline per file per sesi, `index.json` atomik berisi `{cwd, files[]}`, `Capture` (salin baseline sekali pada sentuhan pertama dengan validasi path), `MarkChanged` (catat `expectedHash`), `Touched`, `Files` (hitung status, buang yang sama dengan baseline), `Prune` (retensi), serta helper path. Semua signature publik tidak berubah dari overview 2.3 sehingga `review-hooks-ipc` yang dibangun terhadap stub tetap berkompilasi.

## Path yang dimiliki

HANYA berkas berikut yang boleh disentuh:

- `internal/review/types.go`
- `internal/review/store.go`
- `internal/review/capture.go`

Jangan menyentuh `internal/review/ipc.go`, `cmd/shikigamid/wire_review.go`, `internal/status/*`, apa pun di `frontend/`, `internal/fsx/*` (hanya dipakai, tidak diubah), `internal/runs`, maupun berkas tiket 02-04 (`diff*.go`, `actions*.go`, `comment.go`). Jangan menyentuh `frontend/src/modules/sidebar-right/FilesTab.svelte` (perubahan user). Jangan membuat berkas Go baru di paket ini. Tanpa dependency baru, tanpa unit test.

## Kontrak

Disediakan (signature final, persis overview 2.3; JANGAN diubah):

```go
// types.go: Status (+StatusModified/Added/Deleted), maxBaselineBytes, HashAbsent,
// IPCMethodFiles, EventChanged, FileEntry, TouchedFile, FilesResult
func NewStore(dataDir string) *Store
func (s *Store) Touched(sessionID string) []TouchedFile
func (s *Store) Files(sessionID, cwd string) ([]FileEntry, error)
func (s *Store) Prune(keepEnded int, activeSessionIDs []string)
func (s *Store) Cwd(sessionID string) string
func (s *Store) SessionDir(sessionID string) string
func (s *Store) BaselinePath(sessionID, relPath string) string
func (s *Store) HashPath(relPath string) string
func (s *Store) Capture(sessionID, cwd, path string) error
func (s *Store) MarkChanged(sessionID, cwd, path string) error
```

Format `index.json` (kontrak tiket 02-04): `{"cwd": "<abs cwd>", "files": [TouchedFile...]}`; nama field JSON `TouchedFile` tetap (`path`, `existedBefore`, `baselineSha256`, `firstTouchAt`, `reviewed`, `expectedHash`, `binary`, `tooLarge`). Penyimpanan: `<dataDir>/review/<sessionId>/index.json` dan `<dataDir>/review/<sessionId>/baseline/<sha256(relPath) hex>`. Izin berkas `0o600`, direktori `0o700`. Path relatif memakai pemisah `/`. Waktu unix milidetik.

Dipakai: `fsx.ResolveUnder(root, rel string) (string, error)` (`internal/fsx/readwrite.go`; menolak rel absolut, `..`, dan symlink keluar root) dan `fsx.IsBinary(sample []byte) bool` (`internal/fsx/binary.go`; sampel 8 KiB pertama). Import `github.com/NooBeeID/shikigami/internal/fsx`. Pemanggil di tiket lain: `review-hooks-ipc` (Capture, MarkChanged, Files, Prune, Touched, Cwd) dan tiket 02-04 (Cwd, BaselinePath, HashPath, SessionDir, Touched; kelak menulis ulang index lewat berkas baru di paket, jadi helper index `storeLoad`/`storeSave` boleh dibiarkan tak-diekspor dan tidak dipakai mereka sekarang).

Konvensi penamaan helper (overview 2.1): helper tak-diekspor diawali `store*` di `store.go` dan `capture*` di `capture.go`. Komentar dan pesan galat bahasa Indonesia, identifier bahasa Inggris.

## Langkah

1. **`types.go`.** Baca berkas yang ada. Pastikan isinya identik secara bentuk dengan blok overview 2.3 (konstanta, `FileEntry`, `TouchedFile`, `FilesResult`). Boleh memperjelas komentar (Indonesia) dan menambah komentar paket tentang layout penyimpanan. Jangan mengubah nama, tipe, tag JSON, atau nilai konstanta. Tidak menambah field wajib. Tipe `indexFile` TIDAK ditaruh di sini (lihat langkah 2).

2. **`store.go`: struct dan index.** Ganti isi stub:
   - `type Store struct { dataDir string; mu sync.Mutex; locks map[string]*sync.Mutex }`. `NewStore(dataDir)` menginisialisasi `locks`.
   - `func (s *Store) storeLock(sessionID string) func()`: ambil/buat `*sync.Mutex` untuk sesi dari `locks` di bawah `s.mu`, `Lock()` mutex sesi, kembalikan fungsi unlock. Dipakai sebagai `defer s.storeLock(id)()`. Mutex per sesi melindungi baca-ubah-tulis index; Capture dan MarkChanged mengambilnya, jangan ambil dua kali dalam jalur yang sama (hindari deadlock: helper `storeLoad`/`storeSave` tidak mengunci sendiri).
   - `type storeIndex struct { Cwd string \`json:"cwd"\`; Files []TouchedFile \`json:"files"\` }` (tak-diekspor, di `store.go`).
   - `func (s *Store) storeIndexPath(sessionID string) string` = `filepath.Join(s.SessionDir(sessionID), "index.json")`.
   - `func (s *Store) storeLoad(sessionID string) storeIndex`: baca berkas; galat baca atau JSON rusak -> `storeIndex{Files: []TouchedFile{}}`; pastikan `Files` non-nil. Tanpa panic.
   - `func (s *Store) storeSave(sessionID string, idx storeIndex) error`: `os.MkdirAll(SessionDir, 0o700)`, `json.Marshal`, tulis ke `index.json.tmp` (`0o600`), `os.Rename` ke `index.json`; bila rename gagal hapus berkas tmp. Galat dikembalikan terbungkus `fmt.Errorf("review: simpan index: %w", err)`.
   - Validasi `sessionID` di semua method publik: kosong, mengandung `/`, `\` , atau sama dengan `.`/`..` -> perlakukan sebagai tidak valid (Capture/MarkChanged return nil tanpa efek; getter kembalikan nilai kosong non-nil). Helper `storeValidID(id string) bool`. Tujuan: mencegah penulisan di luar `dataDir/review`. `SessionDir` tetap `filepath.Join(s.dataDir, "review", sessionID)` (tidak melakukan validasi, agar sama dengan stub).

3. **`store.go`: getter dan helper path.** Pertahankan `SessionDir`, `BaselinePath`, `HashPath` dari stub (sha256 hex dari `relPath`). Implementasi:
   - `Touched(sessionID)`: di bawah lock sesi, `storeLoad`, kembalikan salinan `Files` (`append([]TouchedFile{}, idx.Files...)`); id tidak valid -> `[]TouchedFile{}`.
   - `Cwd(sessionID)`: `storeLoad(sessionID).Cwd` di bawah lock; "" bila tidak ada.

4. **`store.go`: `Files(sessionID, cwd string) ([]FileEntry, error)`.** Hasil selalu non-nil (`make([]FileEntry, 0, ...)`). Bila `cwd == ""` pakai `s.Cwd(sessionID)`; bila tetap kosong kembalikan `[]FileEntry{}, nil`. Muat index di bawah lock (salin entri lalu lepas lock sebelum I/O file kerja). Untuk tiap `TouchedFile` t:
   - `abs, err := fsx.ResolveUnder(cwd, t.Path)`; galat -> lewati entri (tanpa galat).
   - `os.Lstat`/`os.Stat(abs)`: tentukan `exists` (hanya file biasa; bila direktori anggap tidak ada dan lewati entri; galat selain not-exist -> lewati).
   - Bila ada, hitung hash isi sekarang lewat helper `storeHashFile(abs) (string, error)` (sha256 streaming `io.Copy`, hex). Galat baca -> lewati entri.
   - Status: `!t.ExistedBefore && exists` -> `StatusAdded`; `t.ExistedBefore && !exists` -> `StatusDeleted`; `t.ExistedBefore && exists && hash != t.BaselineSha256` -> `StatusModified`; kombinasi lain (sama dengan baseline, atau dibuat lalu hilang) -> dibuang.
   - Tambahkan `FileEntry{Path: t.Path, Status, Reviewed: t.Reviewed, Binary: t.Binary, TooLarge: t.TooLarge}` (`Additions`/`Deletions` 0, `External` false).
   - Urutkan `sort.Slice` menurut `Path` naik. Return `entries, nil`. Binary/tooLarge memakai jalur yang sama (hash saja); tidak membaca baseline.
   - `storeHashFile` dan `storeHashBytes` didefinisikan di `store.go`; `capture.go` memakainya.

5. **`store.go`: `Prune(keepEnded int, activeSessionIDs []string)`.** Baca `<dataDir>/review` (`os.ReadDir`; tidak ada -> return). Untuk tiap sub-direktori: bila namanya ada di set aktif -> lewati. Sisanya: hitung `lastTouch` = maksimum `FirstTouchAt` dari index sesi (baca lewat `storeLoad`, tanpa lock panjang), fallback ke `ModTime` direktori (`UnixMilli`) bila index kosong. Urutkan menurun menurut `lastTouch`; `keepEnded < 0` dianggap 0; hapus (`os.RemoveAll(filepath.Join(...))`) semua kecuali `keepEnded` teratas. Ambil `storeLock` sesi saat menghapus tiap direktori, dan hapus entri `locks` sesi itu sesudahnya. Hanya hapus nama yang lolos `storeValidID`. Galat hanya diabaikan (best effort); tanpa panic dan tanpa logger.

6. **`capture.go`: normalisasi path.** Helper `captureRel(cwd, path string) (rel string, ok bool)`:
   - `cwd == ""` atau `path == ""` -> `false`.
   - `cwd = filepath.Clean(cwd)`; bila `path` absolut, `rel, err := filepath.Rel(cwd, filepath.Clean(path))`; galat -> `false`. Bila relatif, `rel = filepath.Clean(path)`.
   - `rel == "."`, `rel == ".."`, atau diawali `".."+string(filepath.Separator)` -> `false`.
   - `rel = filepath.ToSlash(rel)`; kembalikan `rel, true`. Validasi akhir lewat `fsx.ResolveUnder(cwd, rel)` dilakukan pemanggil (langkah 7/8) karena hasil `abs` dipakai untuk membuka file.
   - Catatan: bila `cwd` itu sendiri berupa symlink (mis. `/var` -> `/private/var` di macOS) dan path absolut hook memakai bentuk berbeda, `filepath.Rel` bisa gagal. Fallback: bila hasil diawali `..`, coba ulang dengan `filepath.EvalSymlinks` pada `cwd` dan pada direktori induk `path` (jangan pada `path` bila belum ada); bila tetap di luar, `false`.

7. **`capture.go`: `Capture(sessionID, cwd, path string) error`.**
   - Id tidak valid atau `captureRel` false -> `return nil`. `abs, err := fsx.ResolveUnder(cwd, rel)`; galat (di luar root, symlink keluar) -> `return nil`.
   - `defer s.storeLock(sessionID)()`. `idx := s.storeLoad(sessionID)`; bila ada entri dengan `Path == rel` -> `return nil` (jangan menyalin ulang, baseline pertama tidak boleh tertimpa).
   - `fi, err := os.Stat(abs)`:
     - `os.IsNotExist(err)` -> entri `TouchedFile{Path: rel, ExistedBefore: false, BaselineSha256: "", FirstTouchAt: time.Now().UnixMilli()}`; tanpa salinan.
     - galat lain -> `return nil` (jangan menulis entri setengah jadi).
     - `!fi.Mode().IsRegular()` (direktori, socket, dsb.) -> `return nil` tanpa efek.
     - Ada dan reguler: `ExistedBefore: true`. Baca sampel 8 KiB (`captureSample(abs)` lewat `os.Open` + `io.ReadFull` toleran EOF) -> `binary := fsx.IsBinary(sample)`; `tooLarge := fi.Size() > maxBaselineBytes`.
       - Hitung `BaselineSha256` dengan `storeHashFile(abs)` (isi penuh, streaming, juga untuk biner/besar). Galat baca -> `return nil` tanpa entri.
       - Hanya bila `!binary && !tooLarge`: salin isi ke `s.BaselinePath(sessionID, rel)`: `os.MkdirAll(dir, 0o700)`, tulis atomik (`captureWriteAtomic(dst string, data []byte)`: tmp `dst+".tmp"` `0o600` lalu `os.Rename`). Untuk menghindari ketidakcocokan hash dan salinan akibat file berubah antar-baca, baca isi sekali dengan `os.ReadFile` (file <= 1 MiB), hitung hash dengan `storeHashBytes` dari byte yang sama, dan pakai byte itu untuk deteksi biner (sampel = 8 KiB pertama dari byte tadi) dan salinan. Jalur streaming hanya untuk file yang `tooLarge`; untuk file biner kecil juga cukup `ReadFile`. Atur urutan: stat -> bila `Size() > maxBaselineBytes` jalur streaming (sampel via `captureSample`, hash via `storeHashFile`, flag `TooLarge`, `Binary` dari sampel); selain itu `ReadFile` -> `Binary = fsx.IsBinary(data)`.
       - Galat menulis salinan baseline -> `return fmt.Errorf("review: tulis baseline: %w", err)` dan JANGAN menyimpan entri (agar sentuhan berikutnya bisa mencoba lagi).
     - `FirstTouchAt: time.Now().UnixMilli()`.
   - Tambahkan entri ke `idx.Files`; bila `idx.Cwd == ""` set `idx.Cwd = filepath.Clean(cwd)`; `storeSave`. Kembalikan galat simpan bila ada.
   - Tidak ada panic pada input apa pun (cek batas slice, nil). Target < 20 ms untuk file < 1 MiB: satu `ReadFile`, satu hash, dua tulisan kecil, tanpa jaringan dan tanpa memuat apa pun selain index sesi.

8. **`capture.go`: `MarkChanged(sessionID, cwd, path string) error`.**
   - Id tidak valid, `captureRel` false, atau `ResolveUnder` galat -> `nil`.
   - `defer s.storeLock(sessionID)()`; `storeLoad`; cari entri `Path == rel`; tidak ada -> `nil` tanpa efek (Capture tidak pernah terpanggil, mis. hook Pre terlewat).
   - `os.Stat(abs)`: not-exist -> `ExpectedHash = HashAbsent`; reguler -> `ExpectedHash = storeHashFile(abs)` (galat baca -> `return nil` tanpa mengubah); bukan reguler -> `return nil`.
   - `storeSave`; kembalikan galatnya.
   - `MarkChanged` tidak mengubah `Reviewed`, `Binary`, `TooLarge`, atau baseline. (Catatan untuk tiket 03: bila nanti edit baru terjadi pada file yang sudah `Reviewed`, itu urusan `actions.go`; plan ini TIDAK mereset `Reviewed`.)

9. **Impor dan kebersihan.** `store.go`: `crypto/sha256`, `encoding/hex`, `encoding/json`, `fmt`, `io`, `os`, `path/filepath`, `sort`, `strings`, `sync`, serta `fsx`. `capture.go`: `fmt`, `io`, `os`, `path/filepath`, `strings`, `time`, `fsx`. Hapus komentar `// TODO review-foundation`. Jalankan `gofmt -l internal/review` (harus kosong) dan `go vet ./internal/review/`.

10. **Guard.** Dari root repo: `go build ./...` harus lulus. Verifikasi manual cepat opsional (tanpa commit berkas uji; pakai skrip sementara di scratchpad atau `go run` dengan berkas `_scratch` di luar repo): Capture dua kali pada file yang sama tidak menimpa baseline; file baru -> `added`; path `../x` dan symlink keluar cwd diabaikan; file > 1 MiB dan biner masuk dengan flag dan tanpa berkas di `baseline/`; `Files` membuang entri yang sama dengan baseline; index rusak diperlakukan kosong. Jangan menambahkan berkas `_test.go` ke repo.

## Perubahan

- Diubah: `internal/review/types.go` (hanya komentar bila perlu; bentuk final tidak berubah).
- Diubah (menggantikan stub): `internal/review/store.go`.
- Diubah (menggantikan stub): `internal/review/capture.go`.
- Tidak ada berkas dibuat atau dihapus; tidak ada perubahan `go.mod`.

## Wiring

Yang diekspos untuk disambungkan nanti (JANGAN disambungkan sendiri):

- `review-hooks-ipc` (`wire_review.go`, `ipc.go`) memanggil `NewStore(d.DataDir)`, `Capture` pada `PreToolUse`, `MarkChanged` pada `PostToolUse`, `Files` (hitung `count` dan `review.files`), `Cwd` (fallback cwd untuk `review.files`), dan `Prune(20, activeIDs)` saat `OnReady`.
- Tiket 02-04 memakai `Cwd`, `SessionDir`, `BaselinePath`, `HashPath`, `Touched` dan membaca `index.json` (`{cwd, files[]}`) langsung.
- `d.Provide("review.store", store)` dan seluruh pendaftaran IPC adalah urusan `review-hooks-ipc`.

## Selesai bila

- `go build ./...` lulus dari root repo; `gofmt -l internal/review` kosong; `go vet ./internal/review/` bersih.
- Semua simbol overview 2.3 ada dengan signature persis (termasuk `Prune(keepEnded int, activeSessionIDs []string)`), tanpa perubahan pada berkas di luar `ownedPaths`.
- Perilaku sesuai acceptance criteria tiket: Capture kedua tidak menimpa baseline; file baru `existedBefore=false` tanpa salinan dan tampil `added`; path di luar cwd atau symlink keluar diabaikan tanpa panic dan tanpa menulis di luar `dataDir`; biner dan > 1 MiB tercatat dengan flag tanpa salinan isi; `Files` membuang entri yang sama dengan baseline; `index.json` atomik dan JSON rusak/tidak ada diperlakukan kosong.
- Tidak ada unit test, dependency baru, atau berkas baru di repo.
