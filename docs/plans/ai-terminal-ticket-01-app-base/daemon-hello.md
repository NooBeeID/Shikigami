# Plan: Daemon placeholder dan registri modul

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-01-app-base.md` (epic 01 App Base)
- Fitur: `daemon-hello`
- ownedPaths: `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go`
- Depends on foundation: ya. Foundation (`00-foundation.md`) menyediakan `go.mod` (`module aiterm`), stub `internal/paths` dan `internal/ipc` yang kompilasi dengan signature Kontrak. Plan ini dikerjakan terhadap signature tersebut; implementasi nyata `ipc` datang dari fitur `ipc-transport` secara paralel.
- Kontrak lengkap ada di `overview.md` bagian 2 dan bagian "Kontrak" tiket. Plan ini tidak mengulangnya; ia hanya menetapkan implementasi `cmd/aitermd` dan `cmd/aitermctl`.

## Tujuan

Membuat daemon placeholder `aitermd` yang hanya melayani IPC (lewat `ipc.Server`) dan menulis log start/stop, beserta registri modul (`RegisterModule`, `Deps.Provide/Lookup/OnReady`, `wireAll`) yang menjadi titik ekstensi epic lain lewat file `wire_<modul>.go`. Juga membuat CLI debug `aitermctl` dengan registri subcommand (`Command`, `RegisterCommand`), flag global `--socket`, `help`, subcommand `ping` dan `info`, output JSON terformat, dan exit code yang benar. Hasil akhir: epic lain cukup menambah file baru tanpa mengedit keempat file ini.

## Path yang dimiliki

HANYA empat file ini yang boleh dibuat/diubah:

- `cmd/aitermd/main.go`
- `cmd/aitermd/modules.go`
- `cmd/aitermctl/main.go`
- `cmd/aitermctl/registry.go`

Dilarang: menyentuh `go.mod`/`go.sum` (jangan `go mod tidy`/`go get`; hanya stdlib + `aiterm/internal/paths` + `aiterm/internal/ipc` yang dipakai), `internal/*`, `Makefile`, `app/*`, `frontend/*`, atau membuat file epic lain (`wire_*.go`, `cmd_*.go`). Bila perlu menguji registri dengan modul contoh, buat file sementara (mis. `cmd/aitermd/wire_zz_tmp.go`) lalu HAPUS sebelum selesai. Hanya pakai simbol `ipc` yang tercantum di overview bagian 2 (jangan simbol internal lain).

## Kontrak

Dipakai (dari `internal/paths` dan `internal/ipc`, lihat overview bagian 2):
- `paths.DataDir() (string, error)`, `paths.SocketPath() (string, error)`
- `ipc.NewServer(socketPath, logger) *Server`, `(*Server).SetInfo(func() map[string]any)`, `(*Server).Serve(ctx) error`
- `ipc.Dial(socketPath) (*Client, error)`, `(*Client).Call(method, params, out) error`, `(*Client).Close()`
- `*ipc.Error{Code, Message, Data}` (dikenali dengan `errors.As`)
- Method server `ping` dan `daemon.info` (bawaan `ipc`; daemon hanya mengisi info lewat `SetInfo`, `ipc` menggabungkan `methods` sendiri).

Disediakan (persis sesuai tiket, bagian Kontrak butir 3 dan 4):

```go
// cmd/aitermd/modules.go (package main)
type Deps struct {
    DataDir    string
    SocketPath string
    Logger     *slog.Logger
    Version    string
    Started    time.Time
}
func (d *Deps) Provide(name string, v any)
func (d *Deps) Lookup(name string) (any, bool)
func (d *Deps) OnReady(fn func())
func RegisterModule(name string, wire func(srv *ipc.Server, d *Deps))
func wireAll(srv *ipc.Server, d *Deps)          // internal
func moduleNames() []string                      // internal, terurut, tidak pernah nil

// cmd/aitermctl/registry.go (package main)
type Command struct {
    Name    string
    Summary string
    Run     func(ctx context.Context, c *ipc.Client, args []string) error
}
func RegisterCommand(cmd Command)
func sortedCommands() []Command                  // internal
```

Variabel `version` di `cmd/aitermd/main.go`: `var version = "dev"` (override `-ldflags "-X main.version=..."`).

## Langkah

Kerjakan berurutan. Semua pesan ke pengguna dalam bahasa Indonesia; identifier bahasa Inggris. Tanpa polling dan tanpa timer interval.

### 1. `cmd/aitermd/modules.go`

`package main`. Import: `log/slog`, `sort`, `sync`, `time`, `fmt`, `aiterm/internal/ipc`.

- State paket (tanpa goroutine; registrasi terjadi di `init()` yang berurutan, tetapi lindungi dengan `sync.Mutex` bila murah):
  - `var modules = map[string]func(srv *ipc.Server, d *Deps){}`
- `Deps` persis sesuai Kontrak, ditambah field privat: `services map[string]any`, `ready []func()`, `mu sync.Mutex`. `Deps` selalu dipakai lewat pointer; nilai nol harus aman (inisialisasi `services` malas di `Provide`).
- `func (d *Deps) Provide(name string, v any)`: kunci mutex, buat map bila nil, simpan. Nama ganda: `panic(fmt.Sprintf("layanan %q sudah disediakan", name))` (tabrakan = bug).
- `func (d *Deps) Lookup(name string) (any, bool)`: kunci mutex, baca map.
- `func (d *Deps) OnReady(fn func())`: kunci mutex, append ke `d.ready`.
- `func RegisterModule(name string, wire func(srv *ipc.Server, d *Deps))`: panic bila `name == ""`, `wire == nil`, atau nama sudah ada (`panic(fmt.Sprintf("modul %q sudah terdaftar", name))`); simpan ke `modules`.
- `func moduleNames() []string`: kembalikan nama modul terurut alfabetis (`sort.Strings`), selalu slice non-nil (`make([]string, 0, len(modules))`) agar JSON menjadi `[]` bukan `null`.
- `func wireAll(srv *ipc.Server, d *Deps)`:
  1. Untuk tiap nama di `moduleNames()` (urut alfabetis): `d.Logger.Info("wire modul", "module", name)` lalu `modules[name](srv, d)`.
  2. Setelah SEMUA modul ter-wire: salin `d.ready` (di bawah mutex), lalu jalankan tiap callback berurutan sesuai urutan `OnReady` dipanggil. Callback yang didaftarkan `OnReady` saat sedang menjalankan fase ready ikut dijalankan (loop sampai habis), agar tidak hilang.
  - Panic dari modul dibiarkan naik (bug harus terlihat saat start).
- Komentar singkat di atas `wireAll`: urutan antar modul hanya alfabetis; modul yang butuh layanan modul lain memakai `d.Lookup` di dalam `d.OnReady` atau secara malas saat handler dipanggil.

### 2. `cmd/aitermd/main.go`

`package main`. Import: `context`, `errors`, `fmt`, `log/slog`, `os`, `os/signal`, `syscall`, `time`, `aiterm/internal/ipc`, `aiterm/internal/paths`.

- `var version = "dev"` dengan komentar bahwa nilai di-override lewat `-ldflags "-X main.version=<v>"`.
- `func main()`: panggil `os.Exit(run())` agar `defer` di `run` berjalan. Definisikan `func run() int`:
  1. `logger := slog.New(slog.NewTextHandler(os.Stderr, nil))`.
  2. `dataDir, err := paths.DataDir()`; bila error: `logger.Error("gagal menentukan data dir", "err", err)`, return 1.
  3. `socketPath, err := paths.SocketPath()`; error ditangani sama (pesan error dari `paths` sudah jelas untuk path terlalu panjang), return 1.
  4. `deps := &Deps{DataDir: dataDir, SocketPath: socketPath, Logger: logger, Version: version, Started: time.Now()}`.
  5. `srv := ipc.NewServer(socketPath, logger)`.
  6. `srv.SetInfo(func() map[string]any { return map[string]any{"version": deps.Version, "pid": os.Getpid(), "startedAt": deps.Started.Format(time.RFC3339), "dataDir": deps.DataDir, "socket": deps.SocketPath, "modules": moduleNames()} })`. Kunci harus persis `version, pid, startedAt, dataDir, socket, modules` (kunci `methods` ditambahkan oleh `ipc`, jangan diisi di sini).
  7. `wireAll(srv, deps)` (dipanggil setelah semua `init()` selesai, otomatis karena di dalam `main`; sebelum `Serve`).
  8. `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM); defer stop()`.
  9. `logger.Info("aitermd mulai", "version", version, "pid", os.Getpid(), "socket", socketPath, "modules", moduleNames())`.
  10. `err = srv.Serve(ctx)`. Bila `err != nil && !errors.Is(err, context.Canceled)`: `logger.Error("daemon berhenti dengan galat", "err", err)` (logger sudah menulis ke stderr, jadi tidak perlu cetak ganda); return 1. Pesan "daemon sudah berjalan" datang dari `ipc` (`Serve` mengembalikannya) dan muncul di stderr lewat log tersebut; exit code 1.
  11. Sukses (ctx dibatalkan, `Serve` kembali nil): `logger.Info("aitermd berhenti")`, return 0. Penghapusan socket dilakukan oleh `Serve` (tanggung jawab `ipc`); `main` tidak menghapus file socket sendiri.
- Tidak ada logika lain: tanpa goroutine tambahan, tanpa timer.

### 3. `cmd/aitermctl/registry.go`

`package main`. Import: `context`, `fmt`, `sort`, `aiterm/internal/ipc`.

- `type Command struct { Name string; Summary string; Run func(ctx context.Context, c *ipc.Client, args []string) error }`.
- `var commands = map[string]Command{}`.
- `func RegisterCommand(cmd Command)`: panic bila `cmd.Name == ""`, `cmd.Run == nil`, atau nama sudah terdaftar (`panic(fmt.Sprintf("command %q sudah terdaftar", cmd.Name))`).
- `func sortedCommands() []Command`: semua command terurut menurut `Name`.
- `func lookupCommand(name string) (Command, bool)`.
- Helper `func printJSON(v any) error`: `json.MarshalIndent(v, "", "  ")` lalu tulis ke stdout dengan newline akhir. Letakkan di sini (tambahkan import `encoding/json`, `os`). Helper ini juga boleh dipakai `cmd_<modul>.go` epic lain; beri komentar singkat.
- Helper `func callAndPrint(c *ipc.Client, method string, params any) error`: `var out json.RawMessage`; `c.Call(method, params, &out)`; lalu `printJSON(out)` (RawMessage di-indent ulang: unmarshal ke `any` lalu `printJSON`, supaya format konsisten dan urutan kunci stabil). Dipakai `ping` dan `info`.

### 4. `cmd/aitermctl/main.go`

`package main`. Import: `context`, `errors`, `flag`, `fmt`, `io`, `os`, `os/signal`, `syscall`, `aiterm/internal/ipc`, `aiterm/internal/paths`.

- `init()` mendaftarkan dua command (tidak ada registrasi lain di `registry.go`):
  - `RegisterCommand(Command{Name: "ping", Summary: "Cek daemon hidup (method ping)", Run: ...})` yang memanggil `callAndPrint(c, "ping", nil)`.
  - `RegisterCommand(Command{Name: "info", Summary: "Tampilkan info daemon (method daemon.info)", Run: ...})` yang memanggil `callAndPrint(c, "daemon.info", nil)`.
- `func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }`.
- `func run(args []string, stdout, stderr io.Writer) int`:
  1. `fs := flag.NewFlagSet("aitermctl", flag.ContinueOnError)`; `fs.SetOutput(stderr)`; `socket := fs.String("socket", "", "path unix socket daemon (default: <DataDir>/aitermd.sock)")`. `fs.Usage` memanggil `printHelp(stderr)`.
  2. `fs.Parse(args)`: bila error, return 1 (flag.ErrHelp: tampilkan help ke stdout, return 0). Flag global harus berada SEBELUM nama command (`aitermctl --socket X ping`); sisa argumen `rest := fs.Args()` dengan `rest[0]` sebagai nama command dan `rest[1:]` diteruskan ke `Run`.
  3. Bila `len(rest) == 0` atau `rest[0]` adalah `help`, `-h`, `--help`: `printHelp(stdout)`, return 0 (tanpa Dial; `help` tidak butuh daemon). Bila tanpa argumen sama sekali, help ke stdout dan exit code 0 juga boleh; tetapkan 0.
  4. `cmd, ok := lookupCommand(rest[0])`; bila tidak ada: `fmt.Fprintf(stderr, "command tidak dikenal: %q\nJalankan \"aitermctl help\" untuk daftar command.\n", rest[0])`, return 1. Cek ini SEBELUM Dial.
  5. Tentukan socket: bila `*socket == ""` pakai `paths.SocketPath()` (error -> cetak ke stderr `aitermctl: <err>`, return 1).
  6. `c, err := ipc.Dial(sock)`; bila error: cetak `fmt.Fprintf(stderr, "daemon tidak berjalan di %s\n", sock)` (tambahkan `: <err>` hanya bila berguna untuk debug), return 1. Jangan panik.
  7. `defer c.Close()`. `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM); defer stop()`.
  8. `err = cmd.Run(ctx, c, rest[1:])`. Bila error: bila `errors.As(err, &ipcErr)` dengan `*ipc.Error`, cetak `aitermctl: galat <code>: <message>` ke stderr; selain itu `aitermctl: <err>`. Return 1. Sukses: return 0.
- `func printHelp(w io.Writer)`: cetak `Penggunaan: aitermctl [--socket <path>] <command> [argumen]`, baris `Flag:` untuk `--socket`, lalu `Command:` dengan tiap `sortedCommands()` sebagai `  <name>  <summary>` (rata kiri dengan lebar nama maksimum), dan `help` sendiri.
- Output sukses command: JSON terformat ke stdout (lewat `printJSON`); galat hanya ke stderr; exit code 0/1 sebagaimana di atas.

### 5. Verifikasi build (guard)

1. `gofmt -l cmd/aitermd cmd/aitermctl` harus kosong.
2. `go build ./cmd/...` harus sukses. Bila gagal karena `internal/ipc` fitur lain setengah jadi, itu bukan urusan plan ini; build ulang hanya `./cmd/...` setelah `ipc-transport` selesai. Jangan memodifikasi `internal/` demi membuat build lulus.
3. `go vet ./cmd/...` (opsional).
4. Smoke manual opsional bila `ipc` sudah nyata (tidak wajib), semuanya dengan `AITERM_DATA_DIR=$(mktemp -d)` dan dibersihkan sesudahnya: `go run ./cmd/aitermd &`, `go run ./cmd/aitermctl ping`, `info` (ada `"modules": []`), `help`, command tak dikenal (exit 1), daemon kedua gagal "daemon sudah berjalan" (exit non-nol), `kill -INT` menghapus socket, dan `aitermctl ping` saat daemon mati menampilkan "daemon tidak berjalan di ..." tanpa panic. Uji registri modul dengan `cmd/aitermd/wire_zz_tmp.go` sementara (`func init(){ RegisterModule("zz", func(srv *ipc.Server, d *Deps){}) }`) yang muncul di `modules` pada `info`, lalu HAPUS file itu.
5. Hapus semua file sementara dan biner yang dibuat di luar ownedPaths.

## Perubahan

Dibuat (semuanya baru):
- `cmd/aitermd/modules.go`
- `cmd/aitermd/main.go`
- `cmd/aitermctl/registry.go`
- `cmd/aitermctl/main.go`

Tidak ada file lain yang dibuat atau diubah.

## Wiring

Yang diekspos untuk disambungkan nanti (JANGAN disambungkan oleh fitur ini di luar keempat file; langkah wiring epic yang memverifikasi):
- Daemon: `RegisterModule(name, wire)`, `Deps.Provide/Lookup/OnReady` untuk file `cmd/aitermd/wire_<modul>.go` milik epic lain (otomatis ter-compile karena satu package `main`). `main()` sudah memanggil `wireAll(srv, deps)` sebelum `Serve`, dan `SetInfo` melaporkan `modules` dari `moduleNames()`.
- CLI: `RegisterCommand(Command)` untuk `cmd/aitermctl/cmd_<modul>.go` milik epic lain, plus helper `printJSON` dan `callAndPrint`. `ping` dan `info` terdaftar lewat `init()` di `cmd/aitermctl/main.go`.
- Versi: `Makefile` (milik `go-skeleton-build`) boleh memakai `-ldflags "-X main.version=..."`; variabel `version` sudah tersedia.
- Wiring checklist overview bagian 5 butir 3 dan 4 sudah dipenuhi oleh isi file ini; langkah wiring hanya memverifikasi.

## Selesai bila

- [ ] Hanya empat ownedPaths yang dibuat; tidak ada `wire_*.go`/`cmd_*.go`/file sementara tersisa; `go.mod`/`go.sum` tidak berubah.
- [ ] `gofmt -l` pada kedua direktori kosong.
- [ ] Guard: `go build ./cmd/...` sukses (terhadap stub Foundation atau `ipc` nyata).
- [ ] Signature `Deps`, `Provide`, `Lookup`, `OnReady`, `RegisterModule`, `Command`, `RegisterCommand` persis sesuai Kontrak; `RegisterModule`/`RegisterCommand` panic pada nama ganda; modul di-wire alfabetis; `OnReady` jalan setelah semua modul ter-wire dan sebelum `Serve`.
- [ ] `version` default `"dev"` dan dapat di-override `-ldflags "-X main.version=..."`.
- [ ] `info` melaporkan `modules` sebagai array (kosong `[]` bila belum ada modul); `main` memakai `signal.NotifyContext` untuk SIGINT/SIGTERM dan keluar bersih (exit 0) saat dibatalkan, exit 1 pada galat `Serve` (termasuk "daemon sudah berjalan").
- [ ] `aitermctl` mendukung `--socket`, `help` (daftar terurut), `ping`, `info`; command tak dikenal dan daemon mati menghasilkan pesan Indonesia di stderr dengan exit code 1, tanpa panic.
