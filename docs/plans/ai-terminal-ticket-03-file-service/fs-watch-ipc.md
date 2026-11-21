# Plan: Watch file tampil dan method IPC

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-03-file-service.md` (epic 03 File Service, wave 2, depends_on tiket 01 app-base)
- Fitur: `fs-watch-ipc`
- ownedPaths: `internal/fsx/watch.go`, `internal/fsx/ipc.go`, `cmd/aitermd/wire_fs.go`, `cmd/aitermctl/cmd_fs.go`
- Depends on foundation: ya (`00-foundation.md`). Foundation sudah menyediakan `errors.go` penuh, `ResolveUnder` penuh, tipe `Entry`/`ReadResult`/`WriteResult`, stub `List`/`Read`/`Write`/`IsBinary`, dan stub `Watcher`/`ChangeEvent` di `watch.go` (yang kamu ganti), serta fsnotify sudah ada di `go.mod`/`go.sum`.

Rujukan kontrak bersama: `docs/plans/ai-terminal-ticket-03-file-service/overview.md` bagian 2 (tipe/fungsi `fsx`, error, pesan IPC, registri app-base, konvensi) dan bagian 7 (aturan paralel). Tiket menang bila bertentangan.

## Tujuan

1. `watch.go`: `Watcher` fsnotify malas untuk file yang sedang tampil: satu goroutine pembaca, memantau FOLDER INDUK, refcount per file dan per folder, debounce 100 ms per path, tanpa ticker/polling.
2. `ipc.go`: `Register(srv *ipc.Server) (closeFn func())` mendaftarkan `fs.list`, `fs.read`, `fs.write`, `fs.watch`, `fs.unwatch`, memetakan `*fsx.Error` ke `ipc.NewError`, dan mempublikasikan `fs.changed.<absPath>`.
3. `wire_fs.go`: modul daemon `"fs"`.
4. `cmd_fs.go`: perintah `aitermctl fs ls` / `fs cat` (opsional `fs write`).

## Path yang dimiliki

HANYA path ini yang boleh dibuat/diubah:

- `internal/fsx/watch.go` (ganti stub Foundation)
- `internal/fsx/ipc.go` (baru)
- `cmd/aitermd/wire_fs.go` (baru)
- `cmd/aitermctl/cmd_fs.go` (baru)

Dilarang: file fitur lain (`tree.go`, `ignore.go`, `readwrite.go`, `binary.go`, `errors.go`), `cmd/aitermd/modules.go`, `cmd/aitermd/main.go`, `cmd/aitermctl/registry.go`, `cmd/aitermctl/main.go`, `internal/ipc/`, `go.mod`/`go.sum` (jangan `go get`/`go mod tidy`), Makefile, frontend, `overview.md`. Jangan membuat file stub sementara. Hapus file sementara/smoke yang dibuat di luar ownedPaths sebelum selesai.

## Kontrak

Disediakan (signature dikunci, lihat overview bagian 2):

```go
// watch.go
type ChangeEvent struct { Op string `json:"op"`; Mtime int64 `json:"mtime"` } // Op: write|remove|rename|create
type Watcher struct { /* privat */ }
func NewWatcher(onChange func(root, relPath string, ev ChangeEvent)) (*Watcher, error)
func (w *Watcher) Watch(root, relPath string) error
func (w *Watcher) Unwatch(root, relPath string) error
func (w *Watcher) Close() error

// ipc.go (satu-satunya file fsx yang mengimpor aiterm/internal/ipc)
func Register(srv *ipc.Server) (closeFn func())
```

Method IPC (params objek JSON; hasil sesuai tabel tiket):
- `fs.list {root, relDir}` -> `{entries: Entry[]}`
- `fs.read {root, path, offset?, limit?}` -> `ReadResult`
- `fs.write {root, path, content, expectedMtime}` -> `WriteResult`
- `fs.watch {root, path}` -> `{}` ; `fs.unwatch {root, path}` -> `{}`
- Event: topik `fs.changed.<absPath>`, payload `{root, path, op, mtime}` (`path` relatif ke `root`, pemisah `/`).

Dipakai (jangan diubah, panggil hanya lewat signature ini; boleh masih stub saat kamu mengerjakan):
- `fsx`: `List`, `Read`, `Write`, `ResolveUnder`, `Error`, `ErrBinary/ErrConflict/ErrNotFound/ErrInvalid`, `IsCode`, `Wrap`, `Entry`, `ReadResult`, `WriteResult`.
- `github.com/fsnotify/fsnotify` hanya di `watch.go`.
- `internal/ipc` (plan tiket 01): `ipc.Server` dengan `Register(method string, h ipc.Handler)` dan `Publish(topic string, payload any)`; `ipc.Handler = func(ctx context.Context, params json.RawMessage) (any, error)`; `ipc.NewError(code int, msg string) *ipc.Error` (field `Data any`); konstanta int `ipc.ErrBinary/ErrConflict/ErrNotFound/ErrInvalid/ErrInternal`; klien `ipc.Dial(socketPath)` dan `client.Call(method, params, &out)`. BACA `internal/ipc` aktual dulu; bila signature sedikit berbeda (mis. `Register` mengembalikan sesuatu, `Publish` tanpa error), ikuti kode aktual tanpa mengubah `internal/ipc`.
- `cmd/aitermd/modules.go`: `RegisterModule(name string, wire func(srv *ipc.Server, d *Deps))`. `cmd/aitermctl/registry.go`: `RegisterCommand(Command{Name, Summary, Run})`; baca signature `Run` aktual dan cara mendapat klien (`paths.SocketPath()` + `ipc.Dial`).

Error: `Code` pada `*fsx.Error` sama dengan konstanta `ipc.Err*` (-32000..-32003). Message IPC memakai `err.Error()` (memuat detail dari `Wrap`).

## Langkah

Urutan: `watch.go` dulu (build `go build ./internal/fsx/...`), lalu `ipc.go`, `wire_fs.go`, `cmd_fs.go` (build `go build ./...`).

### 1. `internal/fsx/watch.go`

Hapus stub Foundation dan komentar `// STUB(foundation)`. `package fsx`; import `os`, `path/filepath`, `strings`, `sync`, `time`, `github.com/fsnotify/fsnotify`. Pertahankan tipe `ChangeEvent` (tag json `op`, `mtime`).

Struktur internal:

```go
const debounceDelay = 100 * time.Millisecond

type watchEntry struct {
    root, rel string   // untuk callback
    count     int      // refcount file
    timer     *time.Timer // nil bila tidak ada debounce tertunda
    pending   string   // op terakumulasi selama jendela debounce
}

type Watcher struct {
    mu       sync.Mutex
    fw       *fsnotify.Watcher
    onChange func(root, relPath string, ev ChangeEvent)
    files    map[string]*watchEntry // key: absPath file (hasil ResolveUnder)
    dirs     map[string]int         // key: absPath folder induk -> refcount (jumlah file terpantau di folder itu)
    closed   bool
    done     chan struct{}
}
```

`NewWatcher(onChange)`: `fsnotify.NewWatcher()`, isi struct, jalankan SATU goroutine `go w.loop()`, kembalikan. Tanpa ticker.

`Watch(root, relPath)`:
1. `abs, err := ResolveUnder(root, relPath)`; teruskan error.
2. `os.Stat(abs)`: tidak ada -> `Wrap(ErrNotFound, "%s", relPath)`; folder atau bukan file biasa (`!Mode().IsRegular()`) -> `Wrap(ErrInvalid, "bukan file biasa: %s", relPath)`.
3. Lock `mu`; bila `closed` -> `Wrap(ErrInvalid, "watcher ditutup")`. Bila `files[abs]` ada: `count++`, return nil. Bila baru: `dir := filepath.Dir(abs)`; bila `dirs[dir] == 0` panggil `w.fw.Add(dir)` (error -> kembalikan `Wrap(ErrInvalid, "gagal memantau: %v", err)`, tanpa mengubah peta); lalu `dirs[dir]++`, `files[abs] = &watchEntry{root: root, rel: filepath.ToSlash(relPath) dinormalisasi (Clean, tanpa awalan ./), count: 1}`.
4. Tidak rekursi, tidak memantau folder lain.

`Unwatch(root, relPath)`:
1. `abs` via `ResolveUnder` (target boleh sudah hilang; `ResolveUnder` tidak gagal untuk target tak ada). Error -> kembalikan.
2. Lock. Entry tidak ada -> return nil (idempoten). `count--`; bila masih > 0 return nil.
3. Refcount 0: `timer.Stop()` bila ada, `delete(files, abs)`, `dirs[dir]--`; bila 0: `delete(dirs, dir)` dan `w.fw.Remove(dir)` (abaikan error `fsnotify.ErrNonExistentWatch` dan error karena folder sudah hilang).

`loop()` (satu-satunya goroutine, `defer close(w.done)`):
```go
for {
    select {
    case ev, ok := <-w.fw.Events:
        if !ok { return }
        w.handle(ev)
    case _, ok := <-w.fw.Errors:
        if !ok { return }   // error fsnotify diabaikan (tanpa logger di fsx)
    }
}
```
`handle(ev)`: `abs := filepath.Clean(ev.Name)`; lock; `e := files[abs]`; bila nil -> return (saring: hanya file terpantau). Petakan `ev.Op` ke op string: `Write` -> `"write"`, `Create` -> `"create"`, `Remove` -> `"remove"`, `Rename` -> `"rename"`; abaikan event `Chmod` murni (op 0 setelah dipetakan). Gabungkan ke `e.pending` dengan prioritas sederhana: event terakhir menang kecuali `create`+`write` dalam jendela yang sama tetap `write`. Jadwalkan debounce: bila `e.timer == nil` -> `e.timer = time.AfterFunc(debounceDelay, func(){ w.fire(abs) })`, selain itu `e.timer.Reset(debounceDelay)`. Ini timer per path aktif, bukan ticker.

`fire(abs)`: lock; ambil `e := files[abs]`; nil atau `closed` -> return; ambil `op := e.pending`, kosongkan `pending`, `e.timer = nil`; salin `root, rel`; unlock (JANGAN memanggil callback sambil memegang lock). `fi, err := os.Stat(abs)`; bila err (hilang) -> `mtime = 0` dan bila `op` bukan `remove`/`rename` set `op = "remove"`; bila ada -> `mtime = fi.ModTime().UnixNano()`. Panggil `w.onChange(root, rel, ChangeEvent{Op: op, Mtime: mtime})`. Catatan editor atomic-rename: bila file tertimpa lewat rename (op `create` pada path terpantau setelah `rename`/`remove`), hasil akhir stat ada, jadi op yang dilaporkan `write` bila sebelumnya ada kombinasi remove/rename lalu create (file ada lagi); sederhanakan: setelah stat, bila file ada dan `op` adalah `remove`/`rename`, laporkan `"write"`.

`Close()`: lock; bila sudah `closed` return nil; set `closed = true`; stop semua timer; kosongkan peta; unlock; `w.fw.Close()` (menutup channel sehingga `loop` keluar); tunggu `<-w.done`. Kembalikan error dari `fw.Close()`.

Build: `go build ./internal/fsx/...`; `gofmt -l internal/fsx` kosong untuk file milikmu.

### 2. `internal/fsx/ipc.go`

`package fsx`; import `context`, `encoding/json`, `errors`, `path/filepath`, `sync`, `aiterm/internal/ipc`.

Struct params (tag json persis nama di tiket): 
```go
type listParams  struct { Root string `json:"root"`; RelDir string `json:"relDir"` }
type readParams  struct { Root string `json:"root"`; Path string `json:"path"`; Offset int64 `json:"offset"`; Limit int64 `json:"limit"` }
type writeParams struct { Root string `json:"root"`; Path string `json:"path"`; Content string `json:"content"`; ExpectedMtime int64 `json:"expectedMtime"` }
type watchParams struct { Root string `json:"root"`; Path string `json:"path"` }
type changedPayload struct { Root string `json:"root"`; Path string `json:"path"`; Op string `json:"op"`; Mtime int64 `json:"mtime"` }
```

`func Register(srv *ipc.Server) (closeFn func())`:
- State lokal closure: `var mu sync.Mutex; var w *Watcher`. Helper `getWatcher() (*Watcher, error)` membuat `Watcher` MALAS pada panggilan `fs.watch` pertama dengan `NewWatcher(func(root, rel string, ev ChangeEvent){ abs := filepath.Join(root, filepath.FromSlash(rel)) (Clean); srv.Publish("fs.changed."+abs, changedPayload{root, rel, ev.Op, ev.Mtime}) })`. Daemon idle tanpa watch tidak membuat goroutine fsx.
- `fs.unwatch` ketika `w == nil` -> hasil `struct{}{}` tanpa membuat watcher.
- Daftarkan lima handler lewat `srv.Register`. Pola tiap handler: `decode(params, &p)` (`json.Unmarshal`; error atau `params` kosong untuk method yang butuh params -> `ipc.NewError(ipc.ErrInvalid, "invalid params: ...")`), panggil fungsi `fsx`, `return nil, toIPCError(err)` bila error.
  - `fs.list` -> `List(p.Root, p.RelDir)`; hasil `map[string]any{"entries": entries}`; pastikan `entries` non-nil agar JSON `[]`, bukan `null`.
  - `fs.read` -> `Read(...)` hasil `*ReadResult`.
  - `fs.write` -> `Write(...)` hasil `*WriteResult`. TIDAK mengirim event manual.
  - `fs.watch` -> `getWatcher()` lalu `w.Watch(p.Root, p.Path)`; hasil `struct{}{}`.
  - `fs.unwatch` -> `w.Unwatch(p.Root, p.Path)`; hasil `struct{}{}`.
- `toIPCError(err error) error`: `nil` -> nil; bila `errors.As(err, &fe *Error)`: `ie := ipc.NewError(fe.Code, err.Error())` (Code sentinel sama dengan konstanta `ipc.Err*`; pakai `fe.Code` langsung); bila `fe.Data != nil` set `ie.Data = fe.Data` (konflik membawa `{currentMtime}`). Catatan: `Wrap` membungkus sentinel sehingga `errors.As` mengembalikan sentinel tanpa `Data`; bila konflik berasal dari `NewConflict`, `errors.As` menemukan `*Error` dengan `Data` terisi, jadi aman. Error lain -> `ipc.NewError(ipc.ErrInternal, err.Error())`.
- `closeFn`: lock; bila `w != nil` -> `w.Close()`, `w = nil`; idempoten.
- Pastikan konstanta `ipc.Err*` sama nilainya dengan `fsx` (-32000..-32003); bila `internal/ipc` aktual tidak mengekspos nama itu persis, pakai literal `fe.Code`.

### 3. `cmd/aitermd/wire_fs.go`

`package main`; import `aiterm/internal/fsx`, `aiterm/internal/ipc`.
```go
func init() {
    RegisterModule("fs", func(srv *ipc.Server, d *Deps) {
        closeFn := fsx.Register(srv)
        _ = closeFn // ganti dengan pendaftaran ke mekanisme shutdown Deps bila ada
    })
}
```
Baca `cmd/aitermd/modules.go` aktual: bila `Deps` punya hook shutdown/cleanup (mis. `d.OnShutdown(func())`), daftarkan `closeFn`; bila tidak, biarkan dan hapus `_ =` yang tidak perlu (tetap valid Go: `fsx.Register(srv)` tanpa menyimpan hasil). Jangan edit `modules.go`/`main.go`.

### 4. `cmd/aitermctl/cmd_fs.go`

`package main`; `init()` memanggil `RegisterCommand` untuk perintah `fs` (Name `"fs"`, Summary bahasa Indonesia, mis. "Operasi file service (ls, cat, write)"). Ikuti signature `Command.Run` di `registry.go` aktual dan cara memperoleh klien; bila aitermctl sudah menyediakan helper klien, pakai itu, selain itu `ipc.Dial(paths.SocketPath())`.

Subperintah (argumen dari `args` yang diterima `Run`; kurang argumen -> cetak usage ke stderr, exit non-nol):
- `fs ls <root> [relDir]`: `client.Call("fs.list", map[string]any{"root": root, "relDir": relDir}, &out)` dengan struct lokal bertag `entries` (`struct{ Entries []struct{ Name, Type string; Size int64 } `json:"entries"` }`) agar `cmd_fs.go` tidak bergantung pada `fsx`. Cetak satu entry per baris: `<tipe>\t<ukuran>\t<nama>`.
- `fs cat <root> <path>`: `fs.read` tanpa offset/limit; cetak `content` ke stdout apa adanya (tanpa newline tambahan).
- Opsional `fs write <root> <path> <expectedMtime>`: baca seluruh stdin (`io.ReadAll(os.Stdin)`), `fs.write`; cetak `mtime`/`size` hasil ke stdout. `expectedMtime` di-parse `strconv.ParseInt`; gagal -> pesan "parameter tidak valid".
- Penanganan error: bila error dari `Call` bertipe `*ipc.Error` (atau punya `Code`; baca definisi aktual, gunakan `errors.As`), terjemahkan ke pesan Indonesia dan cetak ke stderr dengan kode keluar non-nol: `-32000` -> "file biner tidak didukung"; `-32001` -> "file berubah di disk (mtime disk: N)" (N dari `Data.currentMtime` bila ada, selain itu dari message); `-32002` -> "tidak ditemukan"; `-32003` -> "parameter tidak valid"; lainnya -> pesan asli. Tambahkan detail message asli di belakang bila membantu.
- Subperintah tidak dikenal -> usage ke stderr, exit non-nol. Cara menandai exit non-nol mengikuti konvensi `Run` aktual (return error atau `os.Exit`).

## Perubahan

- Diubah: `internal/fsx/watch.go` (stub Foundation diganti implementasi penuh).
- Dibuat: `internal/fsx/ipc.go`, `cmd/aitermd/wire_fs.go`, `cmd/aitermctl/cmd_fs.go`.
- Tidak ada perubahan lain (termasuk `go.mod`/`go.sum`).

## Wiring

Fitur ini TIDAK menyambungkan apa pun sendiri di luar ownedPaths. Yang diekspos untuk langkah wiring (overview bagian 5):
- `fsx.Register(srv *ipc.Server) (closeFn func())` dipanggil dari `init()` di `wire_fs.go` lewat `RegisterModule("fs", ...)`; modul `fs` otomatis muncul di `daemon.info` tanpa edit `modules.go`/`main.go`.
- Perintah `fs` terdaftar lewat `RegisterCommand` di `init()` `cmd_fs.go`; tanpa edit `registry.go`/`main.go`.
- Event `fs.changed.<absPath>` (payload `{root, path, op, mtime}`) untuk konsumen File Viewer (di luar epic).
- Langkah wiring nanti: `go mod tidy` (fsnotify jadi require langsung), pastikan tidak ada `STUB(foundation)` tersisa, build penuh, smoke manual.

Bila `internal/ipc` atau registri `cmd/*` belum ada di repo (tiket 01 belum merge), tetap tulis semua file sesuai signature overview dan laporkan bahwa build penuh menunggu tiket 01.

## Selesai bila

- [ ] `watch.go`: satu goroutine pembaca, folder induk dipantau dengan refcount per file dan per folder, filter hanya file terpantau, debounce 100 ms per path via `time.AfterFunc` (tanpa ticker/polling), callback tidak dipanggil sambil memegang lock, `Close()` menghentikan goroutine dan timer.
- [ ] `Unwatch` sampai refcount 0 menghentikan timer, menghapus file dari peta, dan `Remove` folder induk bila refcount folder 0.
- [ ] `Register` mendaftarkan kelima method; `Watcher` dibuat malas saat `fs.watch` pertama; `fs.write` tidak mengirim event manual; `*fsx.Error` dipetakan ke `ipc.NewError` dengan kode benar (konflik membawa `Data` `{currentMtime}`); error non-fsx menjadi `ipc.ErrInternal`.
- [ ] `wire_fs.go` dan `cmd_fs.go` hanya mendaftar lewat `RegisterModule`/`RegisterCommand` di `init()`.
- [ ] Guard: `go build ./internal/fsx/...` sukses setelah `watch.go`; `go build ./...` sukses setelah `ipc.go`, `wire_fs.go`, `cmd_fs.go` (bila tiket 01 sudah ada; bila belum, laporkan). `gofmt -l` kosong untuk keempat file. Tanpa unit test.
- [ ] Tidak ada file di luar ownedPaths yang berubah; tidak ada file sementara tersisa.
