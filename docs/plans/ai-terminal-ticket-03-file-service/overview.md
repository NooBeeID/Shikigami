# Overview Plan: Epic 03 File Service

Tiket: `docs/tickets/ai-terminal-ticket-03-file-service.md`
Spec: `docs/superpowers/specs/2026-11-21-ai-terminal-design.md` (bagian "Cara kerja baca file")

Dokumen ini adalah desain BERSAMA. Agent per fitur hanya membaca overview ini dan tiket, jadi semua kontrak lintas fitur ada di sini. Bila overview dan tiket bertentangan, tiket menang.

## 1. Ringkasan epic dan urutan eksekusi

Epic ini backend saja: layanan file di daemon `aitermd` lewat IPC JSON-RPC (`fs.list`, `fs.read`, `fs.write`, `fs.watch`, `fs.unwatch`, event `fs.changed.<absPath>`), plus perintah debug `aitermctl fs ls/cat`. Seluruh logika ada di package `internal/fsx`. Tanpa kode frontend, tanpa polling, tanpa goroutine latar selain satu pembaca event fsnotify yang dibuat malas saat `fs.watch` pertama.

Kondisi kode saat plan ditulis: tiket 01 (app-base) berstatus sedang dikerjakan di worktree `worktree/feat-ai-terminal-ticket-01-app-base`. Yang sudah ada di sana hanyalah stub kompilasi (`internal/ipc/stub.go`, `internal/paths/paths.go`, `go.mod` module `aiterm`, Go 1.26.3). Implementasi epic ini dikompilasi terhadap signature `internal/ipc` yang dikunci di plan tiket 01 (lihat bagian 2), jadi tidak bergantung pada isi implementasi aslinya. Bila `cmd/aitermd/modules.go` / `cmd/aitermctl/registry.go` belum ada saat build `fs-watch-ipc`, itu prasyarat tiket 01 (`depends_on`), bukan tugas epic ini.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): dependensi fsnotify di `go.mod`/`go.sum`, `internal/fsx/errors.go` lengkap (kontrak error), `ResolveUnder` lengkap, dan stub kompilasi untuk simbol lain `internal/fsx` agar semua fitur bisa build sendirian.
2. **Fitur paralel** (3 fitur, bagian 3): `fs-tree`, `fs-readwrite`, `fs-watch-ipc`. Karena Foundation menyediakan stub, ketiganya bisa dikerjakan bersamaan; `fs-watch-ipc` bagian `ipc.go`/`wire_fs.go`/`cmd_fs.go` baru bisa diverifikasi penuh setelah `fs-tree` dan `fs-readwrite` selesai (urutan internal agent itu: `watch.go` dulu, sisanya belakangan).
3. **Wiring** (bagian 5): `go mod tidy`, pastikan tidak ada stub tersisa, build penuh, smoke test manual.

## 2. Kontrak bersama

Kontrak lengkap (tipe Go, tabel method IPC, payload event, kode error) ada di bagian "Kontrak" tiket dan WAJIB diikuti persis. Ringkasan identifier dan keputusan tambahan yang dikunci di sini:

### Paket dan folder
- Modul Go: `module aiterm`. Import path: `aiterm/internal/fsx`, `aiterm/internal/ipc`.
- `internal/fsx` -> `package fsx`. Hanya `internal/fsx/ipc.go` yang boleh mengimpor `aiterm/internal/ipc`. File lain di `fsx` hanya stdlib (dan `github.com/fsnotify/fsnotify` khusus `watch.go`).
- `cmd/aitermd/wire_fs.go` -> `package main`; `cmd/aitermctl/cmd_fs.go` -> `package main`.
- Epic ini TIDAK mengimpor `internal/workspace` atau package epic lain.

### Tipe dan fungsi `fsx` (dari tiket, tidak boleh berubah signature)
```go
type EntryType string // "file" | "dir" | "symlink"  (const EntryFile, EntryDir, EntrySymlink)
type Entry struct { Name string; Type EntryType; Size int64; Mtime int64 }   // tag json: name,type,size,mtime
type Kind string      // "markdown" | "text"          (const KindMarkdown, KindText)
type ReadResult struct { Content string; Mtime int64; Kind Kind; Size, Offset, Length int64; EOF bool }
type WriteResult struct { Mtime int64; Size int64 }
type ChangeEvent struct { Op string; Mtime int64 }  // Op: "write"|"remove"|"rename"|"create"

func List(root, relDir string) ([]Entry, error)
func Read(root, relPath string, offset, limit int64) (*ReadResult, error)
func Write(root, relPath, content string, expectedMtime int64) (*WriteResult, error)
func ResolveUnder(root, rel string) (abs string, err error)
func IsBinary(sample []byte) bool
func IsIgnored(name string) bool ; var DefaultIgnore []string
func NewWatcher(onChange func(root, relPath string, ev ChangeEvent)) (*Watcher, error)
func (w *Watcher) Watch(root, relPath string) error
func (w *Watcher) Unwatch(root, relPath string) error
func (w *Watcher) Close() error
func Register(srv *ipc.Server) (closeFn func())   // ipc.go
```
Konstanta `EntryFile/EntryDir/EntrySymlink` dan `KindMarkdown/KindText` dipilih di sini (tiket hanya menyebut nilai string). `Entry`/`EntryType` dideklarasikan di `tree.go`; `ReadResult`/`WriteResult`/`Kind` di `readwrite.go`; `Watcher`/`ChangeEvent` di `watch.go`; `Error` dan sentinel di `errors.go`.

### Error `fsx` (errors.go, dikunci oleh Foundation)
```go
type Error struct { Code int; Message string; Data any }  // Data tambahan (opsional), tidak diserialisasi sendiri
func (e *Error) Error() string
func (e *Error) Is(target error) bool   // true bila target adalah *Error dengan Code sama -> errors.Is(err, fsx.ErrNotFound) bekerja pada error yang dibungkus
var ErrBinary   = &Error{Code: -32000, Message: "binary file not supported"}
var ErrConflict = &Error{Code: -32001, Message: "file changed on disk"}
var ErrNotFound = &Error{Code: -32002, Message: "not found"}
var ErrInvalid  = &Error{Code: -32003, Message: "invalid argument"}
func IsCode(err error, code int) bool           // errors.As ke *Error lalu bandingkan Code
func Wrap(base *Error, format string, args ...any) error // fmt.Errorf("%w: ...") sehingga errors.Is/As bekerja dan pesan detail ada
func NewConflict(diskMtime int64) error         // *Error{Code:-32001, Message memuat mtime disk, Data: map[string]int64{"currentMtime": diskMtime}}
```
`Data` adalah penambahan aditif atas struct di tiket (tiket meminta `data` error berisi `{currentMtime}` bila ipc mendukung; `ipc.Error` punya field `Data`).

### Keputusan semantik `ResolveUnder` (dikunci agar fitur konsisten)
- `root` bukan absolut -> `ErrInvalid`; `root` tidak ada / bukan folder -> `ErrNotFound`.
- `rel` absolut, mengandung NUL, atau hasil Clean keluar root -> `ErrInvalid`. `rel` `""` atau `"."` -> mengembalikan root (dibersihkan).
- Mengembalikan path absolut hasil `filepath.Clean(filepath.Join(root, rel))` (bukan hasil EvalSymlinks) agar topik event `fs.changed.<absPath>` stabil; EvalSymlinks hanya dipakai untuk memeriksa bahwa target nyata tetap di bawah EvalSymlinks(root) (keluar -> `ErrInvalid`).
- Target tidak ada TIDAK menghasilkan error di `ResolveUnder` selama folder induk terdekat yang ada masih di bawah root (setelah EvalSymlinks); pemanggil memutuskan `ErrNotFound`. Dengan begitu `Write` file baru bisa memakainya.
- `List` pada entry symlink melaporkan `symlink` dengan `os.ReadDir` + `Lstat` semantik (tanpa mengikuti).

### Pesan IPC
Sesuai tabel tiket. Ringkas:
- `fs.list {root, relDir}` -> `{entries: Entry[]}`; `fs.read {root, path, offset?, limit?}` -> `ReadResult`; `fs.write {root, path, content, expectedMtime}` -> `WriteResult`; `fs.watch {root, path}` -> `{}`; `fs.unwatch {root, path}` -> `{}`.
- Event: topik `fs.changed.<absPath>`, payload `{root, path, op, mtime}` (`path` relatif ke `root`, pemisah `/`).
- Kode error `-32000..-32003` dipetakan lewat `ipc.NewError(code, msg)` (signature `internal/ipc`: `NewError(code int, msg string) *ipc.Error`, field `Data any`). Konstanta kode di `ipc`: `ipc.ErrBinary/ErrConflict/ErrNotFound/ErrInvalid` (int). Error decode params -> `ipc.ErrInvalid`. Error lain yang bukan `*fsx.Error` -> kode `ipc.ErrInternal` (-32603) dengan `err.Error()`.
- Handler `ipc.Handler = func(ctx context.Context, params json.RawMessage) (any, error)`. Hasil `fs.watch`/`fs.unwatch` adalah `struct{}{}` (menghasilkan `{}`).

### Registri app-base yang dipakai (signature dari plan tiket 01)
- Daemon: `RegisterModule(name string, wire func(srv *ipc.Server, d *Deps))`, `Deps{DataDir, SocketPath, Logger, Version, Started}`; modul di-wire urut alfabetis; nama ganda panic. Nama modul epic ini: `"fs"`. Tidak ada hook shutdown di `Deps` pada plan tiket 01; agent baca `cmd/aitermd/modules.go` aktual, bila ada mekanisme cleanup pakai untuk `closeFn`, bila tidak biarkan.
- CLI: `RegisterCommand(Command{Name, Summary, Run})`; baca `cmd/aitermctl/registry.go` aktual untuk signature `Run` dan cara memperoleh klien (`ipc.Dial(socketPath)`, `client.Call(method, params, &out)`; socket dari `paths.SocketPath()`).

### Konvensi penamaan
- Prefix method `fs.*` milik epic ini. File: `cmd/aitermd/wire_fs.go`, `cmd/aitermctl/cmd_fs.go`, `internal/fsx/`.
- Pesan error ke pengguna CLI dalam bahasa Indonesia ("file biner tidak didukung", "tidak ditemukan", "file berubah di disk (mtime disk: N)", "parameter tidak valid"); pesan `Message` error `fsx`/IPC memakai teks bahasa Inggris dari tiket (mis. `binary file not supported`). Identifier kode bahasa Inggris.
- Waktu selalu Unix nanodetik UTC (`int64`), `time.Time.UnixNano()`.
- Path IPC memakai `/`; di dalam `fsx` gunakan `filepath.FromSlash`/`ToSlash` saat konversi.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `fs-tree` | `List` satu level malas (`os.ReadDir`), urut folder dulu lalu alfabetis tanpa peka huruf besar, symlink tidak diikuti, `DefaultIgnore` + `IsIgnored` berbasis nama, tanpa goroutine/cache | `internal/fsx/tree.go`, `internal/fsx/ignore.go` |
| `fs-readwrite` | `errors.go` (final), `ResolveUnder` anti traversal/symlink, `IsBinary`, `Read` (offset/limit, batas UTF-8, kind), `Write` atomik dengan kunci konflik mtime | `internal/fsx/readwrite.go`, `internal/fsx/binary.go`, `internal/fsx/errors.go` |
| `fs-watch-ipc` | `Watcher` fsnotify (folder induk, refcount, debounce 100 ms, malas), `ipc.go` (`Register` lima method + publish event), modul daemon `wire_fs.go`, perintah `aitermctl fs ls/cat` (+ opsional `fs write`) | `internal/fsx/watch.go`, `internal/fsx/ipc.go`, `cmd/aitermd/wire_fs.go`, `cmd/aitermctl/cmd_fs.go` |

Catatan koordinasi:
- Foundation sudah menulis `errors.go` dan `ResolveUnder` (di `readwrite.go`) versi lengkap dan stub untuk simbol lain. `fs-readwrite` memiliki file-file itu dan boleh menyempurnakannya TANPA mengubah signature/semantik di bagian 2. Fitur lain memakai tipe `Error`/`ResolveUnder` apa adanya.
- Tiap fitur mengganti stub Foundation di file miliknya dengan implementasi penuh. Stub bertanda komentar `// STUB(foundation): diganti oleh fs-<slug>`; hapus komentar itu saat mengganti.
- `fs-watch-ipc` memanggil `List/Read/Write/ResolveUnder` hanya lewat signature di atas.

## 4. Foundation

Ada Foundation: `docs/plans/ai-terminal-ticket-03-file-service/00-foundation.md`. Isi ringkas:
- `go get github.com/fsnotify/fsnotify` (versi stabil terbaru, mis. v1.8.x) menambah require di `go.mod`/`go.sum` (pengecualian tunggal tiket). Tanpa `go mod tidy` di tahap ini (belum ada import, tidy akan membuangnya); tidy dijalankan di Wiring.
- `internal/fsx/errors.go` penuh, `internal/fsx/readwrite.go` berisi `ResolveUnder` penuh + tipe `ReadResult/WriteResult/Kind` + stub `Read`/`Write`, `binary.go` stub `IsBinary`, `tree.go` stub `List` + tipe `Entry/EntryType`, `ignore.go` stub, `watch.go` stub `Watcher`/`ChangeEvent` (tanpa import fsnotify).
- Foundation TIDAK membuat `ipc.go`, `wire_fs.go`, `cmd_fs.go`.

## 5. Wiring checklist

Dikerjakan SATU agent setelah ketiga fitur selesai.

1. Tidak ada stub tersisa: `grep -rn "STUB(foundation)" internal/fsx` kosong; tidak ada `panic("belum diimplementasi")` di `internal/fsx`.
2. `go mod tidy`; verifikasi `go.mod` memuat `github.com/fsnotify/fsnotify` sebagai require langsung (bukan `// indirect`) dan `go.sum` lengkap. Bila epic wave 2 lain juga menambah require, gabungkan hasil tidy tanpa menghapus baris mereka.
3. Daemon: `cmd/aitermd/wire_fs.go` memanggil `RegisterModule("fs", ...)` di `init()` dan di dalamnya `fsx.Register(srv)`; `closeFn` didaftarkan ke mekanisme shutdown `Deps` bila ada. Tidak ada edit ke `modules.go`/`main.go`: verifikasi bahwa `daemon.info` melaporkan modul `fs` dan method `fs.*` (`aitermctl info`).
4. CLI: `cmd/aitermctl/cmd_fs.go` mendaftarkan perintah `fs` lewat `RegisterCommand` di `init()`; `aitermctl help` menampilkannya. Tidak ada edit ke `registry.go`/`main.go`.
5. Tidak ada file di luar path tiket yang berubah (cek daftar file baru/diubah; `go.mod`/`go.sum` hanya baris fsnotify dan hasil tidy).
6. Jalankan Build guard (bagian 6), lalu smoke manual (opsional, dengan `AITERM_DATA_DIR` ke direktori sementara): jalankan `build/bin/aitermd` di background; `aitermctl fs ls <root>` (ignore bekerja, folder dulu), `aitermctl fs cat <root> <file>`, file biner -> pesan jelas dan exit non-nol, path `../x` -> invalid, file tak ada -> not found; uji event `fs.changed.` dengan klien sementara (di luar ownedPaths, hapus sesudahnya) memanggil `fs.watch` lalu `echo >> file` dan memastikan tepat satu event setelah debounce.

## 6. Build guard

Tanpa unit test. Satu-satunya guard:
- `go build ./...` harus lolos.
- `make build` (go build + build frontend milik app-base) harus tetap hijau; epic ini tidak menyentuh frontend.
- `go vet ./internal/fsx/...` boleh sebagai pemeriksaan tambahan, bukan guard wajib.

Guard lokal per fitur (agent paralel):
- `fs-tree`: `go build ./internal/fsx/...`
- `fs-readwrite`: `go build ./internal/fsx/...`
- `fs-watch-ipc`: `go build ./internal/fsx/...` untuk `watch.go`; setelah `ipc.go` ditulis, `go build ./...` (butuh `internal/ipc` dari tiket 01 dan registri `cmd/*`).

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya. Dilarang menyentuh file fitur lain, `cmd/aitermd/modules.go`, `cmd/aitermd/main.go`, `cmd/aitermctl/registry.go`, `cmd/aitermctl/main.go`, `internal/ipc/`, Makefile, atau frontend.
- Dilarang menjalankan `go get`/`go mod tidy` atau mengedit `go.mod`/`go.sum` (Foundation dan Wiring yang melakukannya).
- Dilarang mengubah signature atau semantik kontrak di bagian 2 dan tiket. Bila stub Foundation terasa kurang, ikuti kontrak, bukan menebak.
- Build agent paralel bisa gagal sementara karena file fitur lain belum diganti dari stub atau setengah jadi; build hanya paket sendiri dan jangan memperbaiki file fitur lain.
- `fs-watch-ipc` tidak membuat file stub sementara; stub dari Foundation sudah cukup. Bila `internal/ipc`/registri `cmd/*` belum ada di repo (tiket 01 belum merge), selesaikan `watch.go` dan tulis `ipc.go`/`wire_fs.go`/`cmd_fs.go` sesuai signature di bagian 2, dan laporkan bahwa build penuh menunggu tiket 01.
- Tanpa polling, ticker, atau timer interval. Satu-satunya timer adalah debounce per path aktif di `watch.go`.
- Hapus file sementara/smoke test yang dibuat di luar ownedPaths sebelum selesai.
