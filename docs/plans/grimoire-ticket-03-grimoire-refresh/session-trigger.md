# Plan: Pemicu build saat sesi baru

## Metadata

- Tiket: `grimoire-ticket-03-grimoire-refresh` (`docs/tickets/grimoire-ticket-03-grimoire-refresh.md`)
- Fitur: `session-trigger`
- ownedPaths: `internal/grimoire/trigger.go`, `cmd/shikigamid/wire_grimoire_refresh.go`
- Depends on foundation: ya. Stub `diff.go` dan `incremental.go` dari `00-foundation.md` (`Builder`, `NewBuilder`, `Builder.Check`, `Builder.BuildIncremental`, `Builder.Store`, `Diff.Empty`, `IncrementalOptions`, `Result`, `ApplyStale`, `StaleCount`) harus sudah ada. Fitur paralel `diff-incremental` menimpa isinya tanpa mengubah signature; plan ini hanya berkompilasi terhadap stub dan tidak menunggu implementasi nyata.

Sumber kebenaran bersama: `overview.md` di folder plan ini (bagian 2.1, 2.2 butir 3, 4, 5, 6, 2.3, 7). Plan ini tidak mengulangnya, hanya urutan kerja.

## Tujuan

1. `internal/grimoire/trigger.go`: `Trigger` single-flight per `projectId` dengan context akar sendiri (`Kick` non-blocking, `Run` sinkron, `Building`, `Close`), `RegisterRefresh`, dan helper `Refresh*` untuk handler IPC yang disambung saat Wiring.
2. `cmd/shikigamid/wire_grimoire_refresh.go`: modul daemon `grimoire-refresh` yang di `OnReady` membuat `Store`/`Builder`/`Trigger`, memanggil `RegisterRefresh`, dan memasang tepat satu `provider.RegisterLaunchContribution` yang hanya memanggil `Kick`: tanpa mengubah `LaunchContext.Args`, tanpa menunggu, melewati provider `shell`, dan tidak pernah menunda atau menggagalkan peluncuran.
3. Tanpa polling, timer, atau file watcher.

## Path yang dimiliki

HANYA dua berkas ini yang boleh dibuat/disentuh:

- `internal/grimoire/trigger.go`
- `cmd/shikigamid/wire_grimoire_refresh.go`

Dilarang menyentuh: `internal/grimoire/ipc.go`, `diff.go`, `incremental.go` dan berkas tiket 01/02 lain, `cmd/shikigamid/wire_grimoire.go`, `wire_grimoire_inject.go`, `modules.go`, `main.go`, `wire_workspace.go`, `cmd/shikigamictl/*`, apa pun di `internal/session`, `internal/provider`, `internal/ipc`, frontend, `go.mod`, `go.sum`, `overview.md`. Tidak ada berkas lain yang dibuat.

## Kontrak

Dipakai (baca kode nyata dulu; bila nama beda sedikit ikuti kode nyata dan catat):

- Dari `internal/grimoire` tiket 01: `Store` dengan `Exists(projectID) bool`, `ValidateProject(projectID) error`, `ReadState(projectID) (State, error)` (galat `ErrNotBuilt` bila belum ada), `NewStore(dataDir)`; `State`, `Book`.
- Dari stub/implementasi Foundation (overview 2.3): `NewBuilder(store, logger)`, `(*Builder).Store()`, `(*Builder).Check(projectID, projectPath) (Diff, error)`, `(*Builder).BuildIncremental(ctx, projectID, IncrementalOptions{ProjectPath, Full}) (Result, error)`, `Diff.Empty()`, `Result`, `ApplyStale(books, st)`, `StaleCount(st)`. `Builder` memiliki field `logger *slog.Logger` (nullable) yang boleh dibaca dari `trigger.go` karena satu paket.
- `cmd/shikigamid/modules.go`: `RegisterModule(name, fn func(srv *ipc.Server, d *Deps))`; `Deps{DataDir, SocketPath, Logger, OnReady(fn)}`. `srv.Publish(topic string, payload any)` (`internal/ipc/server.go`).
- `provider.RegisterLaunchContribution(func(*provider.LaunchContext))`; `LaunchContext{Session session.SessionSpec, Provider provider.Provider, Args, Env, MCPServers}`; `Provider.ID()`; konstanta `provider.IDShell`. `SessionSpec` memuat `ProjectID` dan `Cwd`. Contribution dipanggil sinkron dari jalur peluncuran, jadi tidak boleh melakukan I/O/IPC di goroutine pemanggil.
- `ipc.Dial(socket)` dan `Client.Call("workspace.list", map[string]any{}, &raw)`; bentuk respons `[]Workspace{ID, Name, Projects []Project{ID, Path, Name}}` (tag JSON `id`, `name`, `path`, `projects`), toleransi bentuk objek `{workspaces:[...]}`. Pola referensi (baca saja): `workspaceDirectory` di `cmd/shikigamid/wire_mention.go` dan `newGrimoireResolver` di `cmd/shikigamid/wire_grimoire.go`.

Disediakan (signature final, jangan diubah; lihat overview 2.3 untuk semantik penuh):

```go
func NewTrigger(b *Builder, publish func(event string, payload any)) *Trigger
func (t *Trigger) Kick(projectID, projectPath string)
func (t *Trigger) Run(ctx context.Context, projectID, projectPath string, full bool) (res Result, started bool, err error)
func (t *Trigger) Building(projectID string) bool
func (t *Trigger) Close()
type RefreshDeps struct{ Builder *Builder; Trigger *Trigger }
func RegisterRefresh(d RefreshDeps)
func RefreshActive() bool
func RefreshRun(ctx context.Context, projectID, projectPath string, full bool) (Result, bool, error)
func RefreshBuilding(projectID string) bool
func RefreshStaleCount(projectID string) int
func RefreshApplyStale(projectID string, books []Book) []Book
```

Event: `publish("grimoire.changed", map[string]any{"projectId": id})` (topik/payload tiket 01). Efek samping wiring daemon: modul `grimoire-refresh` terdaftar dan satu launch contribution. Tidak ada method IPC baru.

## Langkah

### A. `internal/grimoire/trigger.go` (package `grimoire`)

Helper tak-diekspor berawalan `trigger*`/`refresh*`; jangan definisikan ulang nama dari berkas lain. Komentar, galat, dan log bahasa Indonesia. Import: `context`, `errors`, `fmt`, `log/slog`, `sync`, `sync/atomic`.

1. **Tipe `Trigger`.** Field: `b *Builder`, `publish func(string, any)`, `mu sync.Mutex`, `building map[string]struct{}`, `root context.Context`, `cancel context.CancelFunc`.
2. **`NewTrigger(b, publish)`.** `root, cancel := context.WithCancel(context.Background())` (koreksi 2.2 butir 4: tanpa context daemon). Isi map. `publish` boleh nil.
3. **`triggerLog(t) *slog.Logger`.** Kembalikan `t.b.logger` bila `t.b != nil` dan tidak nil, selain itu `slog.Default()`.
4. **`(*Trigger).emit(projectID)`.** Bila `t.publish != nil`, panggil `t.publish("grimoire.changed", map[string]any{"projectId": projectID})`; bungkus `recover` agar publish yang panik tidak menjatuhkan goroutine.
5. **`(*Trigger).acquire(projectID) bool`.** Di bawah `mu`: bila `t.root.Err() != nil` (sudah Close) atau id sudah ada di `building`, kembalikan false; selain itu tandai dan kembalikan true. **`release(projectID)`**: hapus dari map di bawah `mu`.
6. **`Building(projectID)`.** Baca map di bawah `mu`; `t == nil` aman (false).
7. **Inti bersama** `func (t *Trigger) execute(ctx context.Context, projectID, projectPath string, full bool) (Result, error)`; dipanggil hanya setelah `acquire` sukses dan `emit` awal terbit. Urutan:
   - `defer t.release(projectID)` dan `defer t.emit(projectID)` (emit akhir; dipasang sehingga terbit juga saat galat atau panic; urutan defer: release lebih dulu dieksekusi agar `Building` sudah false saat panel membaca status setelah event, jadi daftarkan `emit` DULU lalu `release` supaya release jalan lebih awal). Tambahkan `defer func(){ if r := recover(); r != nil { log Error; err = fmt.Errorf("panic build grimoire: %v", r) } }()` dengan named return agar panic menjadi galat dan tidak membunuh daemon.
   - `projectPath == ""` -> `Result{}, errors.New("projectPath wajib")`.
   - `!full`: `d, err := t.b.Check(projectID, projectPath)`; `err != nil` -> kembalikan galat; `d.Empty()` -> kembalikan `Result{}, nil` TANPA memanggil `BuildIncremental` dan tanpa menulis apa pun (emit akhir tetap terbit sekali lewat defer, agar indikator menghilang).
   - `full` melewati `Check`.
   - `t.b.BuildIncremental(ctx, projectID, IncrementalOptions{ProjectPath: projectPath, Full: full})`.
8. **`Kick(projectID, projectPath string)`.** Non-blocking, tidak pernah panik, tidak mengembalikan galat:
   - `t == nil || t.b == nil` -> return.
   - `st := t.b.Store()`; `st.ValidateProject(projectID) != nil` atau `!st.Exists(projectID)` -> return tanpa efek (project tanpa grimoire tidak pernah dipicu; pembuatan pertama hanya lewat Generate/`grimoire build`).
   - `!t.acquire(projectID)` -> return (single-flight; project lain tidak saling menghalangi karena kunci per id).
   - `t.emit(projectID)` (event mulai), lalu `go func(){ if _, err := t.execute(t.root, projectID, projectPath, false); err != nil && !errors.Is(err, context.Canceled) { log Warn "grimoire: build latar belakang gagal" dengan project dan err } }()`. Tidak ada Wait/channel yang ditunggu pemanggil.
9. **`Run(ctx, projectID, projectPath, full)`.** Sinkron untuk IPC/CLI:
   - `t == nil || t.b == nil` -> `Result{}, false, errors.New("trigger belum siap")`.
   - `ValidateProject` gagal -> kembalikan galatnya (jaga `errors.Is(err, ErrInvalidProject)` bila itu yang dikembalikan store). Tidak ada syarat `Exists`: Run boleh membuat grimoire pertama.
   - `!t.acquire(projectID)` -> `Result{}, false, nil` (pemanggil memetakannya ke `building: true`).
   - Gabungkan context: `rctx, cancel := context.WithCancel(ctx)`; `stop := context.AfterFunc(t.root, cancel)`; `defer stop(); defer cancel()` (`ctx` nil diganti `context.Background()`), sehingga `Close()` juga membatalkan Run.
   - `t.emit(projectID)`; `res, err := t.execute(rctx, projectID, projectPath, full)`; kembalikan `res, true, err`.
10. **`Close()`.** `t != nil` -> `t.cancel()`. Idempoten.
11. **Pasangan aktif.** `type RefreshDeps struct{ Builder *Builder; Trigger *Trigger }`; `var refreshActive atomic.Pointer[RefreshDeps]`. `RegisterRefresh(d)` menyimpan salinan (`refreshActive.Store(&d)`), panggilan ulang menimpa; tidak mendaftarkan method IPC apa pun (koreksi 2.2 butir 5). Fungsi bantu tak-diekspor `refreshCurrent() (RefreshDeps, bool)` mengembalikan ok hanya bila pointer ada dan `Builder` serta `Trigger` tidak nil.
12. **Helper `Refresh*`** (aman dipanggil sebelum `RegisterRefresh`):
    - `RefreshActive() bool` = `ok` dari `refreshCurrent`.
    - `RefreshRun(...)`: belum aktif -> `Result{}, false, errors.New("refresh grimoire belum terdaftar")`; selain itu `d.Trigger.Run(...)`.
    - `RefreshBuilding(id) bool`: belum aktif -> false; selain itu `Trigger.Building(id)`.
    - `RefreshStaleCount(id) int`: belum aktif atau `ReadState` gagal (termasuk `ErrNotBuilt`) -> 0; selain itu `StaleCount(state)`.
    - `RefreshApplyStale(id, books) []Book`: belum aktif atau `ReadState` gagal -> `books` apa adanya; selain itu `ApplyStale(books, state)`.
13. Jalankan `gofmt -l internal/grimoire` dan `go build ./...` (lihat Selesai bila).

### B. `cmd/shikigamid/wire_grimoire_refresh.go` (package `main`)

Semua identifier tak-diekspor berawalan `refresh` (`refreshProjectDirectory`, `refreshProjectEntry`, `refreshOnce`, `refreshApply`, `refreshProjectFor`, `refreshLogger`). JANGAN memakai nama `projectDirectory`, `projectEntry`, atau awalan `grimoireInject*` (dipakai `wire_grimoire_inject.go`), dan jangan bergantung pada berkas itu; semua helper didefinisikan sendiri di sini. Import: `context`-bila-perlu, `encoding/json`, `errors`, `log/slog`, `path/filepath`, `strings`, `sync`, `time`, `internal/grimoire`, `internal/ipc`, `internal/provider`, `internal/session`.

1. **Modul.**
   ```go
   func init() {
       RegisterModule("grimoire-refresh", func(srv *ipc.Server, d *Deps) {
           d.OnReady(func() { wireGrimoireRefresh(srv, d) })
       })
   }
   ```
   Nama unik (bukan `grimoire`/`grimoire-inject`); layanan lain tidak diambil saat `RegisterModule`.
2. **Direktori project malas** `refreshProjectDirectory{socket string; mu sync.Mutex; client *ipc.Client; entries []refreshProjectEntry; fetchedAt time.Time}` dengan `refreshProjectEntry{ID, Path string}`:
   - `list() ([]refreshProjectEntry, bool)`: kunci `mu`; bila `entries == nil` atau cache lebih tua 5 detik panggil `refreshLocked()`; refresh gagal dan masih ada cache lama -> pakai cache lama; tidak ada -> `nil, false`.
   - `refreshLocked() bool`: dial malas `ipc.Dial(socket)` (socket kosong -> false); `client.Call("workspace.list", map[string]any{}, &raw)` dengan `raw json.RawMessage` (params `map[string]any{}`, bukan nil); unmarshal ke `[]ws`, fallback objek `{workspaces:[...]}`; `Call` gagal -> `_ = client.Close()`, `client = nil`, false. Isi `entries` dari semua `Projects`, set `fetchedAt`.
   - `resolve(spec session.SessionSpec) (id, path string, ok bool)`: ambil `list()`; bila `spec.ProjectID != ""` cari entri ber-`ID` sama dan ambil `Path`-nya (path kosong -> ok false); selain itu cocokkan `spec.Cwd` (`filepath.Clean`) terhadap `Path` entri: sama persis atau `cwd` berada di bawah `path+separator` (tangani path `/`), pilih awalan terpanjang; tidak ketemu -> false. `Cwd` kosong dan `ProjectID` kosong -> false.
3. **Logika contribution** `refreshApply(ctx *provider.LaunchContext, dir *refreshProjectDirectory, trig *grimoire.Trigger, logger *slog.Logger)`; berjalan sinkron di jalur peluncuran, jadi HARUS cepat dan tidak mengubah `ctx` sama sekali:
   1. `ctx == nil || ctx.Provider == nil || trig == nil` -> return. `ctx.Provider.ID() == provider.IDShell` -> return.
   2. Salin `spec := ctx.Session` (nilai) sebelum goroutine, lalu `go func(){ ... }()` yang memuat SEMUA pekerjaan lain (lookup `workspace.list` bisa memblokir; tidak boleh menunda peluncuran). Di dalam goroutine: `defer recover` -> log Warn; `id, path, ok := dir.resolve(spec)`; `!ok` -> return diam-diam; `trig.Kick(id, path)` (Kick sendiri menyaring project tanpa grimoire). Tidak ada `Wait`, channel yang ditunggu, atau penulisan ke `ctx.Args`/`Env`/`MCPServers`/`Session`. Galat hanya ke log.
   3. `logger` nil -> `slog.Default()` lewat `refreshLogger`.
   Catatan urutan: kontribusi inject tiket 2 membaca `index.md` secara sinkron dan penulisan build atomik, jadi snapshot sesi ini selalu dari `index.md` utuh; jangan mengandalkan urutan kontribusi.
4. **Pemasangan sekali.** `var refreshOnce sync.Once`; `installGrimoireRefresh(dir, trig, logger)` memanggil `refreshOnce.Do(func(){ provider.RegisterLaunchContribution(func(c *provider.LaunchContext){ refreshApply(c, dir, trig, logger) }) })`.
5. **`wireGrimoireRefresh(srv *ipc.Server, d *Deps)`** (di `OnReady`):
   1. `dataDir := d.DataDir`; kosong -> `paths.DataDir()` seperti di `wire_grimoire.go` (galat -> log Warn "modul grimoire-refresh dilewati", return). Tambah import `internal/paths`.
   2. `store := grimoire.NewStore(dataDir)`; `b := grimoire.NewBuilder(store, d.Logger)`; `trig := grimoire.NewTrigger(b, srv.Publish)`.
   3. `grimoire.RegisterRefresh(grimoire.RefreshDeps{Builder: b, Trigger: trig})`.
   4. `installGrimoireRefresh(&refreshProjectDirectory{socket: d.SocketPath}, trig, d.Logger)`.
   Tanpa `d.Provide` (nama layanan bisa bentrok), tanpa `main.go`/`modules.go`, tanpa hook berhenti (goroutine selesai saat proses keluar; tulisan atomik).

### C. Verifikasi

1. `gofmt -l internal/grimoire cmd/shikigamid` kosong; `go build ./...` lulus; opsional `go vet ./internal/grimoire/... ./cmd/...`.
2. `grep -nE "time\.(NewTicker|Tick|After)|setInterval|fsnotify" internal/grimoire/trigger.go cmd/shikigamid/wire_grimoire_refresh.go` kosong.
3. `git status --short` menunjukkan hanya dua berkas milik fitur ini sebagai perubahan baru; `git diff --stat -- go.mod go.sum internal/session internal/provider internal/ipc` kosong.
4. Bila build gagal karena berkas fitur lain (`diff.go`, `incremental.go`, `wire_grimoire_inject.go`), jangan diperbaiki; laporkan.

## Perubahan

- Dibuat: `internal/grimoire/trigger.go`
- Dibuat: `cmd/shikigamid/wire_grimoire_refresh.go`
- Diubah: tidak ada.

## Wiring

Yang diekspos untuk langkah wiring akhir (overview bagian 5, butir 2, 3, 6, 7); JANGAN disambungkan di sini:

- Modul daemon `grimoire-refresh` sudah mandiri dan terdaftar lewat `init()`; `modules.go` dan `main.go` tidak berubah.
- Handler IPC di `internal/grimoire/ipc.go` (berkas tiket 01) nanti memakai: `RefreshActive()`, `RefreshRun(ctx, id, path, full)` (hasil `started == false` dipetakan ke `{building: true}`), `RefreshBuilding(id)` (`grimoire.status.building`), `RefreshStaleCount(id)` (`grimoire.status.staleCount`), `RefreshApplyStale(id, books)` (`grimoire.list` `chapters[].stale`).
- Peringatan untuk wiring: `Trigger` sudah mem-publish `grimoire.changed` saat mulai dan selesai; handler `grimoire.build` tidak boleh mem-publish lagi bila `RefreshActive()` agar event tidak ganda.
- Wiring memverifikasi: tidak ada tabrakan nama di `package main` dengan `wire_grimoire_inject.go`, uji manual sesi baru tidak tertunda, dua sesi cepat di project sama hanya satu build, sesi `shell` dan project tanpa grimoire tidak memicu build, argumen launch sesi tidak berubah.

## Selesai bila

- Kedua berkas ada, `gofmt -l` kosong, `go build ./...` sukses (terhadap stub Foundation atau implementasi fitur paralel; kegagalan akibat berkas fitur lain dilaporkan, bukan diperbaiki).
- `trigger.go` memuat semua tipe/fungsi bagian "Disediakan" dengan signature persis; `Kick` tidak melakukan apa pun untuk project tanpa grimoire, single-flight per id, publish `grimoire.changed` saat mulai dan selesai (juga no-op, tanpa menulis), tidak memblokir; `Run` sinkron dengan `started=false` bila sedang building, `full` melewati `Check`, dan dibatalkan oleh `Close()`; `building` selalu dibersihkan (juga saat galat/panic).
- `wire_grimoire_refresh.go` memuat `RegisterModule("grimoire-refresh", ...)` dari `init()`; di `OnReady` membuat Store/Builder/Trigger, `RegisterRefresh`, dan satu launch contribution via `sync.Once` yang melewati provider `shell`, bekerja di goroutine, hanya memanggil `Kick`, dan tidak menyentuh `LaunchContext`.
- Tidak ada ticker/timer/watcher, tidak ada perubahan di path terlarang, `go.mod`/`go.sum` tak berubah, tanpa unit test (guard hanya build).
