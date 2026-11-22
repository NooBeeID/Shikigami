# Plan: review.revert, review.accept, dan flag external

## Metadata

- Tiket: `agent-diff-review-ticket-03-review-actions` (`docs/tickets/agent-diff-review-ticket-03-review-actions.md`)
- Fitur: `review-actions-daemon`
- ownedPaths: `internal/review/actions.go`, `internal/review/actions_ipc.go`
- Depends on foundation: tidak (`hasFoundation=false`; paket `review` dan semua helper yang dipakai sudah ada dari tiket 01 dan 02)
- Bahasa: komentar kode dan pesan error bahasa Indonesia; identifier bahasa Inggris. Modul Go `github.com/NooBeeID/shikigami`.

## Tujuan

Menambah sisi daemon untuk aksi review: `Revert` (kembalikan file atau hunk ke baseline sesi), `Accept` (baseline maju ke isi sekarang), deteksi `external`, serta IPC `review.revert` dan `review.accept` yang menerbitkan `review.changed` sesudah tiap aksi. Aksi harus aman: gagal `[CONFLICT]` bila isi file berubah sejak diff dihitung, dan `[EXTERNAL]` bila file diubah di luar agent dan `force != true`. Kontrak lengkap (nama, param, bentuk error, aturan perilaku) ada di `overview.md` bagian 2.2 dan 2.3; plan ini menjabarkan cara mengimplementasikannya. Bila kode nyata berbeda dari uraian, ikuti kode dan catat di laporan.

## Path yang dimiliki

HANYA dua berkas baru ini yang boleh dibuat/disentuh:

- `internal/review/actions.go`
- `internal/review/actions_ipc.go`

Dilarang menyentuh: `internal/review/{types,store,capture,ipc,diff,diff_ipc}.go`, `cmd/shikigamid/wire_review.go` (edit aditif di `ipc.go` dan `wire_review.go` adalah tugas langkah Wiring, bukan fitur ini), seluruh `frontend/`, `go.mod`, dan path fitur `review-actions-ui`. Tanpa dependency baru, tanpa unit test, tanpa `git commit`.

## Kontrak

Disediakan (persis, lihat overview 2.2 dan 2.3):

```go
// actions.go
func (s *Store) Revert(sessionID, path string, hunkIDs []string, expectHash string, force bool) (removed bool, err error)
func (s *Store) Accept(sessionID, path string, hunkIDs []string) (removed bool, err error)
func (s *Store) IsExternal(sessionID, path string) bool
func FillExternal(st *Store, sessionID string, files []FileEntry) // isi files[i].External in place; aman untuk st==nil/files kosong

// actions_ipc.go
const IPCMethodRevert = "review.revert"
const IPCMethodAccept = "review.accept"
func RegisterActions(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool)
```

IPC (JSON camelCase persis):

```
review.revert {sessionId, path, hunkIds?: string[], expectHash: string, force?: bool} -> {ok: true, removed: bool}
review.accept {sessionId, path, hunkIds?: string[]}                                    -> {ok: true, removed: bool}
```

Error semua lewat `ipc.NewError(ipc.ErrInvalid, msg)` (params rusak: `ipc.ErrInvalidParams`). Tag persis di awal pesan:

- `"[CONFLICT] file berubah sejak diff dihitung, muat ulang"`
- `"[EXTERNAL] file diubah di luar agent, butuh konfirmasi (force)"`

Dipakai dari kode yang ada (sudah dibaca, jangan diubah):

- `store.go`: `Store.storeLock(id) func()` (pakai `defer s.storeLock(id)()`), `storeLoad(id) storeIndex`, `storeSave(id, idx) error` (keduanya TIDAK mengunci), `storeIndex{Cwd, Files []TouchedFile}`, `storeValidID`, `storeHashBytes`, `storeHashFile`, `BaselinePath(id, rel)`, `Touched(id)` dan `Cwd(id)` (MENGUNCI; hanya untuk di luar lock aksi), `Files(id, cwd)`.
- `types.go`: `TouchedFile{Path, ExistedBefore, BaselineSha256, FirstTouchAt, Reviewed, ExpectedHash, Binary, TooLarge}`, `HashAbsent = "absent"`, `EventChanged`, `FileEntry.External`.
- `capture.go`: `captureWriteAtomic(dst, data)` (HANYA untuk berkas baseline di data dir; jangan untuk file pengguna).
- `diff.go`: `ComputeDiff(old, cur []byte) DiffResult` (nil = sisi tidak ada; `Hunk{ID,OldStart,OldLines,NewStart,NewLines,Lines}`; `DiffResult.Binary/TooLarge`), `diffSplitLines`, `MaxDiffBytes`. Jangan menulis ulang perhitungan hunk.
- `fsx.ResolveUnder(cwd, rel)` (validasi path + symlink, mengembalikan path absolut).
- `ipc`: `srv.Register(method, func(ctx, params json.RawMessage) (any, error))`, `srv.Publish(topic, payload)`, `ipc.NewError`, `ipc.ErrInvalid`, `ipc.ErrInvalidParams`. Pola handler: lihat `diff_ipc.go` (params di-trim, `supportedFn == nil || !supportedFn(id)` ditangani).

Konvensi: helper privat berawalan `actions` (mis. `actionsApplyReverse`) agar tidak bentrok dengan `store*`/`capture*`/`diff*`. Fungsi publik baru hanya yang tercantum di atas.

## Langkah

### 1. `internal/review/actions.go` (paket `review`)

Imports yang dibutuhkan: `errors`, `fmt`, `os`, `path/filepath`, `sort`, `strings`, `github.com/NooBeeID/shikigami/internal/fsx`.

**1.1 Sentinel error (privat).**

```go
var (
    actionsErrConflict = errors.New("[CONFLICT] file berubah sejak diff dihitung, muat ulang")
    actionsErrExternal = errors.New("[EXTERNAL] file diubah di luar agent, butuh konfirmasi (force)")
)
```

**1.2 Helper baris.** Pecah memakai pemisah yang sama dengan `diffSplitLines` tetapi MEMPERTAHANKAN terminator, sehingga indeks baris identik dengan nomor hunk (hunk memakai 1-based `OldStart/NewStart` dari `diffSplitLines`):

- `actionsSplitKeep(b []byte) []string`: `len(b)==0` -> `nil`; `strings.SplitAfter(string(b), "\n")`; buang elemen terakhir bila `""`. (Cek: `"a\nb"` -> `["a\n","b"]`; `"a\n"` -> `["a\n"]`; `"\n"` -> `["\n"]`.)
- `actionsJoin(lines []string) []byte`: gabung; untuk setiap baris KECUALI yang terakhir, bila tidak berakhir `"\n"` tambahkan `"\n"` (pengaman bila potongan akhir berkas tanpa newline ternyata tidak lagi di akhir). Baris terakhir dipakai apa adanya, sehingga ada/tidaknya newline akhir mengikuti sisi sumber. Hasil kosong -> `[]byte{}`.

**1.3 Helper terap hunk (murni, tanpa I/O).** Keduanya memakai salinan slice baris dan memproses hunk dari paling bawah ke atas agar indeks tetap valid; periksa batas dan kembalikan error `"hunk di luar rentang, muat ulang diff"` (jangan panik):

- `actionsApplyReverse(cur, base []byte, hunks []Hunk) ([]byte, error)`: `curLines := actionsSplitKeep(cur)`, `baseLines := actionsSplitKeep(base)`. Urut `hunks` menurun menurut `NewStart` (tie: `OldStart`). Untuk tiap hunk: `cs := h.NewStart-1`, atau `0` bila `h.NewLines == 0` (sisi kosong, `NewStart==0`); `bs := h.OldStart-1`, atau `0` bila `h.OldLines == 0`. Ganti `curLines[cs : cs+h.NewLines]` dengan `baseLines[bs : bs+h.OldLines]` (potongan baris asli baseline; ini juga menjaga CRLF dan newline persis seperti baseline). Return `actionsJoin(curLines)`.
- `actionsApplyForward(base, cur []byte, hunks []Hunk) ([]byte, error)`: kebalikannya. Urut menurun menurut `OldStart` (tie: `NewStart`). Ganti `baseLines[bs : bs+h.OldLines]` dengan `curLines[cs : cs+h.NewLines]` memakai rumus `bs`/`cs` yang sama. Hasilnya baseline baru.

**1.4 Helper penulisan file pengguna (atomik, pertahankan mode).**

`actionsWriteFile(abs string, data []byte) error`:
- `MkdirAll(filepath.Dir(abs), 0o755)`.
- Tentukan target: bila `os.Lstat(abs)` menunjukkan symlink, `target, _ = filepath.EvalSymlinks(abs)` (jangan mengganti symlink dengan berkas biasa; path sudah divalidasi `ResolveUnder`); selain itu target = `abs`.
- Mode: `os.Stat(target)` ada -> `fi.Mode().Perm()`; tidak ada -> `0o644`.
- `os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".review-tmp-*")` (direktori yang sama agar `rename` atomik), tulis `data`, `Close`, `os.Chmod(tmp, mode)`, `os.Rename(tmp, target)`. Pada setiap galat: `os.Remove(tmp)` lalu kembalikan error terbungkus (`fmt.Errorf("tulis file: %w", err)`).
- JANGAN memakai `captureWriteAtomic` (mode 0600).

**1.5 Helper status file dan resolusi.**

- `actionsCurrentHash(abs string) (hash string, exists bool, err error)`: `os.Stat`; not-exist -> `("", false, nil)`; bukan file biasa -> error `"bukan file biasa"`; selain itu `storeHashFile(abs)` -> `(hash, true, nil)`. (Konvensi hash: `""` = file tidak ada, sama dengan `DiffResult.CurrentHash` dan `expectHash`.)
- `actionsLocate(idx storeIndex, path string) (abs string, i int, err error)`: `path` kosong -> error `"path wajib diisi"`; cari `idx.Files[i].Path == path`, tidak ketemu -> `fmt.Errorf("file tidak ada dalam daftar review sesi ini: %s", path)`; `idx.Cwd == ""` -> `"cwd sesi tidak diketahui"`; `fsx.ResolveUnder(idx.Cwd, path)` galat -> `fmt.Errorf("path tidak valid: %w", err)`. Dipakai di bawah lock; JANGAN memanggil `Cwd`/`Touched`/`Files`.
- `actionsExternal(t TouchedFile, curHash string, exists bool) bool`: `t.ExpectedHash == ""` -> `false`; `now := curHash`, dan bila `!exists` maka `now = HashAbsent`; return `now != t.ExpectedHash`.
- `(s *Store) actionsDrop(sessionID string, idx storeIndex, i int) error`: buang `idx.Files[i]` (`append(idx.Files[:i], idx.Files[i+1:]...)`), `s.storeSave`; bila sukses `os.Remove(s.BaselinePath(sessionID, path))` best effort (abaikan galat). Simpan index DULU baru hapus baseline.
- `actionsIDSet(ids []string) map[string]bool` / helper pemilih: trim tiap id, buang yang kosong, dedupe. `actionsSelect(res DiffResult, ids map[string]bool) ([]Hunk, error)`: kembalikan hunk yang id-nya terpilih; id yang tidak ada di `res.Hunks` -> `fmt.Errorf("hunkId tidak dikenal: %s (diff mungkin sudah berubah, muat ulang)", id)`. `all := len(selected) == len(res.Hunks)` ditentukan pemanggil.

**1.6 `Revert`.**

```go
func (s *Store) Revert(sessionID, path string, hunkIDs []string, expectHash string, force bool) (removed bool, err error)
```

Urutan (semua di bawah `defer s.storeLock(sessionID)()`; awal: `!storeValidID(sessionID)` -> error `"sessionId tidak valid"`):

1. `idx := s.storeLoad(sessionID)`; `abs, i, err := actionsLocate(idx, path)`; `t := idx.Files[i]`. `ids := actionsIDSet(hunkIDs)`. Bila `len(ids) > 0 && (t.Binary || t.TooLarge)` -> error `"file biner/terlalu besar hanya mendukung aksi seluruh file"`.
2. `curHash, exists, err := actionsCurrentHash(abs)`. Bila `curHash != expectHash` (di mana `expectHash` hex atau `""` untuk file tidak ada) -> `actionsErrConflict`.
3. Bila `actionsExternal(t, curHash, exists) && !force` -> `actionsErrExternal`.
4. Cabang seluruh file bila `len(ids) == 0` (setelah langkah 5 bila semua hunk terpilih, lompat ke sini):
   - `!t.ExistedBefore`: bila `exists` -> `os.Remove(abs)` (abaikan `os.IsNotExist`); lalu `actionsDrop`, `removed=true`.
   - `t.ExistedBefore`:
     - bila `exists && curHash == t.BaselineSha256` -> tidak ada yang ditulis; `actionsDrop`, `removed=true` (sudah identik).
     - bila `t.Binary || t.TooLarge` -> error `"baseline tidak tersimpan untuk file biner/terlalu besar, tidak bisa dikembalikan"` (celah tiket 01; jangan diperbaiki di sini).
     - `base, err := os.ReadFile(s.BaselinePath(sessionID, path))`; galat -> `fmt.Errorf("baseline tidak ditemukan: %w", err)`. `actionsWriteFile(abs, base)` (mencakup `modified` dan `deleted` yang dibuat ulang beserta direktori induknya). Lalu `actionsDrop`, `removed=true`.
5. Cabang sebagian (`len(ids) > 0`):
   - `cur`: bila `exists` -> `os.ReadFile(abs)` dan verifikasi `storeHashBytes(cur) == curHash` (kalau beda -> `actionsErrConflict`, pengaman balapan); bila tidak ada -> `cur = nil`.
   - `base`: bila `t.ExistedBefore` -> baca baseline (galat seperti di atas); selain itu `nil`.
   - `res := ComputeDiff(base, cur)`. `res.Binary || res.TooLarge` -> error file biner/terlalu besar seperti langkah 1.
   - `sel, err := actionsSelect(res, ids)`. Bila `len(sel) == len(res.Hunks)` (semua hunk terpilih) -> perlakukan sebagai revert penuh (kembali ke cabang 4 dengan `ids` dikosongkan; jangan menghitung ulang cek `[CONFLICT]`/`[EXTERNAL]`).
   - `out, err := actionsApplyReverse(cur, base, sel)`; `actionsWriteFile(abs, out)`; `newHash := storeHashBytes(out)`.
   - Bila `t.ExistedBefore && newHash == t.BaselineSha256` -> `actionsDrop`, `removed=true`. Selain itu `idx.Files[i].ExpectedHash = newHash` (aksi pengguna sendiri tidak boleh jadi `external`), `storeSave`, `removed=false`. Baseline tidak berubah.
6. Galat dari `storeSave`/`actionsDrop` dikembalikan sebagai `fmt.Errorf("simpan index: %w", err)`.

**1.7 `Accept`.**

```go
func (s *Store) Accept(sessionID, path string, hunkIDs []string) (removed bool, err error)
```

Tanpa `expectHash` dan tidak menulis file pengguna. Di bawah `defer s.storeLock(sessionID)()`:

1. `idx := s.storeLoad`, `abs, i, err := actionsLocate(idx, path)`, `t := idx.Files[i]`, `ids := actionsIDSet(hunkIDs)`; `len(ids) > 0 && (t.Binary || t.TooLarge)` -> error yang sama dengan Revert.
2. Seluruh file (`len(ids) == 0`, atau semua hunk terpilih): `actionsDrop`, `removed=true` (baseline <- isi sekarang identik, jadi entri dan berkas baseline dibuang; berlaku untuk `modified`, `added`, `deleted`, serta file `Binary`/`TooLarge` tanpa menyalin apa pun; file tidak ada dan `!ExistedBefore` juga dibuang).
3. Sebagian: `curHash, exists, err := actionsCurrentHash(abs)`; `cur` = `os.ReadFile(abs)` bila `exists` (verifikasi `storeHashBytes(cur) == curHash`, beda -> `actionsErrConflict`) atau `nil`; `base` seperti Revert; `res := ComputeDiff(base, cur)`; biner/terlalu besar -> error; `sel, err := actionsSelect(res, ids)`; bila semua hunk terpilih -> langkah 2. Selain itu `out, err := actionsApplyForward(base, cur, sel)`; `captureWriteAtomic(s.BaselinePath(sessionID, path), out)`; set `idx.Files[i].BaselineSha256 = storeHashBytes(out)`, `ExistedBefore = true`; `ExpectedHash` dan `Reviewed` TIDAK diubah. Bila `storeHashBytes(out) == curHash` (baseline baru identik isi sekarang) -> `actionsDrop`, `removed=true`; selain itu `storeSave`, `removed=false`.

Hunk id yang tidak dikenal (diff sudah bergeser) memberi error jelas dari `actionsSelect`; tidak ada tag khusus.

**1.8 `IsExternal` dan `FillExternal`.** Keduanya BOLEH memakai `Touched`/`Cwd` karena dipanggil di luar lock aksi.

- `func (s *Store) IsExternal(sessionID, path string) bool`: `!storeValidID` -> false; cari entri di `s.Touched(sessionID)`; tidak ada -> false; `cwd := s.Cwd(sessionID)`, kosong -> false; delegasikan ke helper `actionsExternalFor(cwd string, t TouchedFile) bool`.
- `actionsExternalFor(cwd string, t TouchedFile) bool`: `t.ExpectedHash == ""` -> false; `fsx.ResolveUnder` galat -> false; `os.Stat`: not-exist -> now = `HashAbsent`; bukan file biasa atau galat lain -> false; file biasa -> `storeHashFile`, galat -> false. Return `now != t.ExpectedHash`.
- `func FillExternal(st *Store, sessionID string, files []FileEntry)`: `st == nil || len(files) == 0 || !storeValidID(sessionID)` -> return; satu kali `st.Touched` (jadikan map path -> `TouchedFile`) dan satu kali `st.Cwd`; `cwd == ""` -> return; untuk tiap `files[i]` set `files[i].External = actionsExternalFor(cwd, t)` bila entri ditemukan. Galat per file diabaikan (tetap `false`).

### 2. `internal/review/actions_ipc.go` (paket `review`)

Imports: `context`, `encoding/json`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`.

- Konstanta `IPCMethodRevert = "review.revert"` dan `IPCMethodAccept = "review.accept"`.
- Tipe respons privat `actionsResponse{OK bool `json:"ok"`; Removed bool `json:"removed"`}`.
- Struct params privat (dipakai kedua metode): `SessionID string "sessionId"`, `Path string "path"`, `HunkIDs []string "hunkIds"`, `ExpectHash string "expectHash"` (hanya revert), `Force bool "force"` (hanya revert).
- `RegisterActions(srv *ipc.Server, store *Store, supportedFn func(string) bool)` mendaftarkan kedua metode lewat helper privat `actionsHandle(srv, store, supportedFn, revert bool) func(ctx, params) (any, error)`:
  1. `json.Unmarshal` gagal -> `ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: ...")` (sebutkan bentuk yang diminta per metode).
  2. Trim `sessionId` dan `path`; kosong -> `ipc.NewError(ipc.ErrInvalid, "sessionId wajib diisi")` / `"path wajib diisi"`. Trim `expectHash`.
  3. `supportedFn == nil || !supportedFn(id)` -> `ipc.NewError(ipc.ErrInvalid, "provider sesi tidak mendukung review")`.
  4. Panggil `store.Revert(id, path, p.HunkIDs, p.ExpectHash, p.Force)` atau `store.Accept(id, path, p.HunkIDs)`. Bila err: `ipc.NewError(ipc.ErrInvalid, err.Error())` (pesan `[CONFLICT]`/`[EXTERNAL]` harus lolos utuh dengan tag di awal; jangan menambah awalan apa pun).
  5. Sukses: panggil helper privat `actionsPublish(srv, store, id)`: `cwd := store.Cwd(id)`; `files, _ := store.Files(id, cwd)`; `n` = jumlah `!f.Reviewed` (rumus sama dengan `cmd/shikigamid/wire_review.go`); `srv.Publish(EventChanged, map[string]any{"sessionId": id, "count": n})` langsung tanpa debounce. Dipanggil SETELAH aksi selesai dan lock dilepas (jangan dari dalam `Store.Revert/Accept`). Galat `Files` diabaikan (count 0 hanya bila gagal).
  6. Return `actionsResponse{OK: true, Removed: removed}`.

### 3. Verifikasi

- `gofmt -l internal/review` harus kosong; `go vet ./internal/review/` (opsional).
- `go build ./...` dari root repo harus sukses. `RegisterActions` dan `FillExternal` berkompilasi walau belum dipanggil (fungsi publik, tidak ada error "unused").
- Tinjau mandiri (tanpa unit test): tidak ada pemanggilan `Touched/Cwd/Files` di dalam `Revert/Accept/actions*` yang memegang lock; semua penulisan berkas pengguna lewat `actionsWriteFile`; tidak ada penulisan di luar hasil `ResolveUnder`.

## Perubahan

- Dibuat: `internal/review/actions.go` (Revert, Accept, IsExternal, FillExternal, helper `actions*`).
- Dibuat: `internal/review/actions_ipc.go` (`IPCMethodRevert`, `IPCMethodAccept`, `RegisterActions`, publikasi `review.changed`).
- Tidak ada berkas lain yang diubah.

## Wiring

Fitur ini hanya mengekspos; penyambungan dikerjakan agen Wiring (overview bagian 5) dan TIDAK dilakukan di sini:

1. `internal/review/ipc.go`: di handler `review.files`, tepat setelah `FillDiffStats(store, id, files)`, tambahkan `FillExternal(store, id, files)`.
2. `cmd/shikigamid/wire_review.go`: tepat setelah `review.RegisterDiffIPC(srv, store, reviewWireSupported(eng))`, tambahkan `review.RegisterActions(srv, store, reviewWireSupported(eng))`.

Yang diekspos: `(*Store).Revert`, `(*Store).Accept`, `(*Store).IsExternal`, `FillExternal`, `RegisterActions`, `IPCMethodRevert`, `IPCMethodAccept`.

## Selesai bila

- [ ] `internal/review/actions.go` dan `internal/review/actions_ipc.go` ada dengan signature persis seperti di Kontrak; tidak ada berkas lain yang berubah.
- [ ] Revert penuh: `modified` menulis isi baseline (identik byte demi byte), `added` menghapus file, `deleted` membuat ulang dari baseline (termasuk direktori induk); entri dan berkas baseline dibuang, `removed=true`.
- [ ] Revert sebagian membalik-terapkan hanya hunk terpilih dari bawah ke atas (hunk lain utuh, baseline tidak berubah, `ExpectedHash` diperbarui); semua hunk terpilih atau hasil identik baseline diperlakukan sebagai revert penuh.
- [ ] Accept penuh membuang entri dan baseline; Accept sebagian menulis baseline baru (maju-terapkan hunk terpilih) sehingga diff berikutnya hanya memuat sisanya.
- [ ] Urutan cek: validasi param -> path -> `[CONFLICT]` -> `[EXTERNAL]` -> terapkan; pesan error persis seperti kontrak; `force=true` tetap lolos cek `expectHash`.
- [ ] File biner/terlalu besar: `hunkIds` terisi -> error; Revert penuh tanpa salinan baseline -> error jelas; Revert penuh file `added` dan Accept penuh tetap diizinkan.
- [ ] Penulisan file pengguna atomik (tmp di direktori yang sama + rename) dengan mode asli dipertahankan; `captureWriteAtomic` hanya dipakai untuk baseline.
- [ ] Semua aksi memegang `storeLock` per sesi dan tidak memanggil `Touched/Cwd/Files` saat memegang lock; `IsExternal`/`FillExternal` memakai definisi `external` (hash sekarang atau `HashAbsent` != `ExpectedHash`; `ExpectedHash` kosong -> false).
- [ ] IPC `review.revert`/`review.accept` mengembalikan `{ok:true, removed}` dan menerbitkan `review.changed {sessionId,count}` sesudah tiap aksi sukses.
- [ ] Guard: `go build ./...` sukses (tanpa dependency baru dan tanpa unit test).
