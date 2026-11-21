# Foundation: Epic 03 File Service

Dikerjakan SATU agent sebelum fitur paralel. Baca `overview.md` (bagian 2 dan 4) dan tiket terlebih dahulu. Tujuan: `go build ./internal/fsx/...` hijau dengan semua simbol kontrak sudah ada (sebagian stub), sehingga `fs-tree`, `fs-readwrite`, `fs-watch-ipc` bisa dikerjakan bersamaan.

Batasan: hanya buat path berikut: `go.mod`, `go.sum` (hanya baris fsnotify), `internal/fsx/errors.go`, `readwrite.go`, `binary.go`, `tree.go`, `ignore.go`, `watch.go`. JANGAN membuat `ipc.go`, `cmd/aitermd/wire_fs.go`, `cmd/aitermctl/cmd_fs.go`.

## Langkah

1. **Dependensi.** Dari root repo/worktree: `go get github.com/fsnotify/fsnotify` (versi stabil terbaru). Jangan jalankan `go mod tidy` (belum ada import; akan membuang require). Pastikan `go.mod` tetap `module aiterm` dan baris lain tidak berubah.

2. **`internal/fsx/errors.go` (versi lengkap, bukan stub).** `package fsx`. Isi persis sesuai overview bagian 2 "Error `fsx`":
   - `type Error struct { Code int; Message string; Data any }`, `Error() string` mengembalikan `Message`.
   - `func (e *Error) Is(target error) bool`: `t, ok := target.(*Error)`; `return ok && t.Code == e.Code`.
   - Sentinel `ErrBinary` (-32000, `binary file not supported`), `ErrConflict` (-32001, `file changed on disk`), `ErrNotFound` (-32002, `not found`), `ErrInvalid` (-32003, `invalid argument`).
   - `IsCode(err error, code int) bool` memakai `errors.As`.
   - `Wrap(base *Error, format string, args ...any) error` = `fmt.Errorf("%w: %s", base, fmt.Sprintf(format, args...))`. Catatan: `errors.As` pada hasilnya mengembalikan `base` (sentinel), jadi `IsCode` dan pemetaan IPC memakai `Code` sentinel; pesan detail ada di `err.Error()`. `ipc.go` memakai `err.Error()` sebagai message.
   - `NewConflict(diskMtime int64) error`: `&Error{Code: -32001, Message: fmt.Sprintf("file changed on disk (disk mtime %d)", diskMtime), Data: map[string]int64{"currentMtime": diskMtime}}`.

3. **`internal/fsx/readwrite.go`.** Deklarasikan tipe `Kind` (`KindMarkdown = "markdown"`, `KindText = "text"`), `ReadResult`, `WriteResult` dengan tag json sesuai tiket. Implementasikan `ResolveUnder` LENGKAP sesuai semantik overview bagian 2 ("Keputusan semantik ResolveUnder") dan tiket (fitur fs-readwrite, butir `ResolveUnder`): cek root absolut/ada, tolak `rel` absolut/NUL, `filepath.Clean(filepath.Join(root, rel))`, cek prefiks `root` atau `root + separator`, lalu EvalSymlinks pada root dan pada target (atau pada folder induk terdekat yang ada bila target belum ada) dan cek ulang masih di bawah root nyata. Kembalikan path hasil Clean (bukan hasil EvalSymlinks). Error memakai `Wrap(ErrInvalid, ...)` / `Wrap(ErrNotFound, ...)`.
   Tambahkan stub berikut dengan komentar `// STUB(foundation): diganti oleh fs-readwrite`:
   ```go
   func Read(root, relPath string, offset, limit int64) (*ReadResult, error) { return nil, Wrap(ErrInvalid, "Read belum diimplementasi") }
   func Write(root, relPath, content string, expectedMtime int64) (*WriteResult, error) { return nil, Wrap(ErrInvalid, "Write belum diimplementasi") }
   ```
   (Gunakan error, bukan `panic`, supaya build dan jalannya program aman.)

4. **`internal/fsx/binary.go`.** Stub: `func IsBinary(sample []byte) bool { return false }` dengan komentar STUB.

5. **`internal/fsx/tree.go` + `ignore.go`.** `tree.go`: tipe `EntryType`, konstanta `EntryFile/EntryDir/EntrySymlink`, `Entry` (tag json: `name,type,size,mtime`), stub `func List(root, relDir string) ([]Entry, error) { return nil, Wrap(ErrInvalid, "List belum diimplementasi") }`. `ignore.go`: `var DefaultIgnore []string` (kosong) dan `func IsIgnored(name string) bool { return false }`, keduanya bertanda STUB.

6. **`internal/fsx/watch.go`.** Tanpa import fsnotify. Tipe `ChangeEvent{Op string; Mtime int64}` (tag json `op`, `mtime`), `type Watcher struct{}`, dan stub: `NewWatcher(onChange func(root, relPath string, ev ChangeEvent)) (*Watcher, error)` mengembalikan `&Watcher{}, nil`; `Watch`, `Unwatch` mengembalikan `Wrap(ErrInvalid, "belum diimplementasi")`; `Close` mengembalikan nil. Bertanda STUB.

7. **Verifikasi.** `go build ./internal/fsx/...` dan `gofmt -l internal/fsx` (harus kosong). Semua tanda `STUB(foundation)` harus unik per file agar mudah dicari. Jangan menulis file lain.

## Keluaran yang diharapkan
- `go.mod`/`go.sum` memuat fsnotify.
- `internal/fsx/` berisi 6 file di atas, build hijau, `errors.go` dan `ResolveUnder` final.
