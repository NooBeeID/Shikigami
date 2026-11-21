---
id: ai-terminal-ticket-01-app-base
title: App Base
spec: docs/superpowers/specs/2026-11-21-ai-terminal-design.md
status: ready
wave: 1
depends_on: []
---
# App Base

## Tujuan

Membuat skeleton proyek yang langsung bisa di-build dan dijalankan di macOS: modul Go `aiterm`, daemon placeholder `aitermd`, CLI debug `aitermctl`, aplikasi Wails dengan frontend Svelte "Hello World" yang menampilkan hasil `ping` ke daemon, serta skrip build. Epic ini juga membangun titik ekstensi (registri modul daemon, registri subcommand CLI, registri panel/tab frontend, transport IPC) yang ditumpangi semua epic lain tanpa perlu mengedit file milik epic ini. Setelah tiket ini selesai, `make build` sukses dan `aitermd` bisa dijalankan lalu di-ping lewat `aitermctl ping` maupun lewat window Wails; epic berikutnya hanya menambah file baru.

## Batasan

In scope:
- Modul Go `aiterm`, Makefile, `.gitignore`, README singkat, `scripts/build.sh`.
- `internal/paths` (DataDir dan path socket).
- `internal/ipc`: server dan client JSON-RPC 2.0 lewat unix socket, pub/sub notifikasi, method bawaan `ping` dan `daemon.info`.
- `cmd/aitermd` (daemon placeholder + registri modul), `cmd/aitermctl` (CLI + registri subcommand, subcommand `ping`).
- `app/` (Wails main + `bridge.go`), `frontend/` (Svelte + Vite + TS, Hello World, `registry.ts`, `ipc.ts`).

Out of scope (milik epic lain, jangan dibuat di sini):
- Logika PTY/sesi, workspace, file service, provider, status, mention (tiket 02-09).
- UI shell nyata (sidebar, tab, xterm.js): tiket 06 mengambil alih `frontend/src/App.svelte`.
- Menyalakan daemon otomatis dari UI (`app/daemon.go`, milik tiket 06). Di tiket ini bridge hanya tersambung ke daemon yang sudah jalan, dan menampilkan pesan jelas bila belum jalan.
- Pembungkusan `.app` bundle, ikon, code signing, Windows/Linux, unit test.

## Kontrak

### Yang disediakan epic ini

**1. Paths (`internal/paths`)**
```go
func DataDir() (string, error)    // ~/Library/Application Support/aiterm (dibuat 0700 bila belum ada)
func SocketPath() (string, error) // <DataDir>/aitermd.sock
```
Override untuk dev lewat env `AITERM_DATA_DIR` (bila di-set, dipakai sebagai DataDir). Catatan: panjang path unix socket di macOS dibatasi ~104 byte; bila `SocketPath` melebihi 100 byte, kembalikan error yang jelas.

**2. IPC (`internal/ipc`)**
- Protokol: JSON-RPC 2.0, satu pesan JSON per baris (dipisah `\n`), lewat unix socket `<DataDir>/aitermd.sock`. Pembaca harus memakai `bufio.Reader.ReadBytes('\n')` atau setara (bukan `bufio.Scanner` dengan batas default), karena pesan (mis. isi file) bisa sangat besar dan tidak ada batas ukuran.
- Notifikasi server ke klien: `{"jsonrpc":"2.0","method":"event","params":{"topic":"<string>","payload":<any>}}`.
- Kode error (konstanta Go di `internal/ipc/errors.go`):
  - `ErrBinary = -32000`, `ErrConflict = -32001`, `ErrNotFound = -32002`, `ErrInvalid = -32003`.
  - Kode standar JSON-RPC: `-32700` parse error, `-32600` invalid request, `-32601` method not found, `-32602` invalid params, `-32603` internal error.
  - Tipe `*ipc.Error{Code int, Message string, Data any}` yang mengimplementasikan `error`, dengan helper `ipc.NewError(code, msg)` dan `ipc.Errorf(code, fmt, ...)`. Handler yang mengembalikan `*ipc.Error` dipetakan apa adanya; error biasa dipetakan ke `-32603`.
- API server:
```go
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

func NewServer(socketPath string, logger *slog.Logger) *Server
func (s *Server) Register(method string, h Handler)     // panic bila method sudah terdaftar (tabrakan = bug)
func (s *Server) Publish(topic string, payload any)     // kirim notifikasi "event" ke klien yang berlangganan prefix cocok
func (s *Server) Serve(ctx context.Context) error       // hapus socket basi, listen (0600), terima koneksi; berhenti saat ctx batal dan menghapus socket
func (s *Server) Methods() []string                     // daftar method terdaftar (untuk daemon.info)
```
- API client:
```go
func Dial(socketPath string) (*Client, error)
func (c *Client) Call(method string, params any, out any) error  // out boleh nil; error server dikembalikan sebagai *ipc.Error
func (c *Client) Subscribe(topicPrefix string, fn func(topic string, payload json.RawMessage)) (unsubscribe func(), err error)
func (c *Client) Close() error
```
  `Call` aman dipakai dari banyak goroutine (id request unik, korelasi lewat map id ke channel). `Subscribe` memanggil method bawaan `events.subscribe` di server dan memfilter lokal per callback; unsubscribe memanggil `events.unsubscribe` bila tidak ada callback lain dengan prefix itu.
- Method bawaan (didaftarkan oleh `NewServer`):
  - `ping`: params diabaikan; hasil `{"pong":true,"ts":"<RFC3339Nano>"}`.
  - `daemon.info`: hasil `{"version":string,"pid":int,"startedAt":RFC3339,"dataDir":string,"socket":string,"modules":[string],"methods":[string]}`. Server memerlukan penyedia info; sediakan `(*Server).SetInfo(func() map[string]any)` yang diisi oleh daemon, dan `ipc` menggabungkan `methods` sendiri.
  - `events.subscribe` params `{"prefix":string}` dan `events.unsubscribe` params `{"prefix":string}`: pengaturan langganan per koneksi. Prefix kosong = semua topik. Topik cocok bila diawali prefix. Koneksi tanpa langganan tidak menerima event apa pun (tanpa polling, tanpa broadcast buta).
- Konkurensi: tiap request ditangani di goroutine sendiri dengan `ctx` yang dibatalkan saat koneksi putus; penulisan ke koneksi diserialisasi dengan mutex per koneksi; `Publish` tidak boleh memblokir tanpa batas oleh klien yang lambat (gunakan buffer channel per koneksi, jatuhkan koneksi yang buffer-nya penuh).

**3. Registri modul daemon (`cmd/aitermd/modules.go`)**
```go
type Deps struct {
    DataDir    string
    SocketPath string
    Logger     *slog.Logger
    Version    string
    Started    time.Time
}
func (d *Deps) Provide(name string, v any)       // bagikan layanan antar modul (mis. "session.launcher")
func (d *Deps) Lookup(name string) (any, bool)
func (d *Deps) OnReady(fn func())                 // dijalankan setelah SEMUA modul ter-wire, sebelum Serve

func RegisterModule(name string, wire func(srv *ipc.Server, d *Deps)) // dipanggil dari init() di wire_<modul>.go
```
Aturan: modul di-wire berurutan menurut nama (alfabetis, deterministik) di `main()` setelah semua `init()` selesai. Karena urutan antar modul tidak dijamin, modul yang butuh layanan modul lain harus memakai `d.Lookup` di dalam `d.OnReady` atau secara malas saat handler dipanggil. `RegisterModule` dengan nama ganda harus panic. Nama modul yang terdaftar ikut dilaporkan di `daemon.info`.

**4. Registri subcommand CLI (`cmd/aitermctl/registry.go`)**
```go
type Command struct {
    Name    string
    Summary string
    Run     func(ctx context.Context, c *ipc.Client, args []string) error
}
func RegisterCommand(cmd Command) // dipanggil dari init() di cmd_<modul>.go; panic bila nama ganda
```
`main.go` mem-parse flag global `--socket <path>` (default `paths.SocketPath()`), `help` (daftar command terurut), lalu `Dial` dan menjalankan command. Output command ke stdout berupa JSON terformat (`json.MarshalIndent`); error ke stderr dengan exit code 1. Command `ping` sudah terdaftar di tiket ini (`cmd/aitermctl/main.go` atau file registri; boleh juga `daemon.info` sebagai command `info`).

**5. Registri frontend (`frontend/src/lib/registry.ts`)**
```ts
import type { Component } from 'svelte';

export type PanelArea = 'sidebar' | 'main-tab' | 'statusbar';
export interface PanelSpec { id: string; area: PanelArea; component: Component<any>; order?: number; }
export interface TabTypeSpec { type: 'session' | 'file'; component: Component<any>; }

export function registerPanel(spec: PanelSpec): void;        // id ganda: spec baru menimpa yang lama
export function registerTabType(spec: TabTypeSpec): void;
export function panels(area: PanelArea): Readable<PanelSpec[]>;   // svelte store, terurut menurut order lalu id
export function tabTypes(): Readable<Record<string, TabTypeSpec>>;
```
Registri berbasis Svelte store agar UI shell reaktif terhadap registrasi yang datang belakangan. Modul frontend dimuat otomatis di `main.ts` lewat `import.meta.glob('./modules/*/index.ts', { eager: true })` (folder `modules/` boleh belum ada; glob kosong tidak boleh membuat build gagal). Tiap modul memanggil `registerPanel`/`registerTabType` saat modul di-import.

**6. Bridge Wails dan wrapper IPC frontend**
- `app/bridge.go`: struct `Bridge` di-bind ke Wails. Method `Call(method string, params json.RawMessage) (json.RawMessage, error)` meneruskan ke daemon lewat `ipc.Client` (koneksi malas, tersambung ulang bila putus). Bridge juga memanggil `Subscribe("")` pada koneksi daemon dan meneruskan setiap event ke frontend lewat `runtime.EventsEmit(ctx, "daemon:event", {topic, payload})`. Bila daemon belum jalan, `Call` mengembalikan error berpesan jelas (mis. "daemon tidak berjalan di <socket>") dan tidak panik.
- `frontend/src/lib/ipc.ts`:
```ts
export function call<T = unknown>(method: string, params?: unknown): Promise<T>;
export function onEvent(topicPrefix: string, fn: (topic: string, payload: unknown) => void): () => void; // mengembalikan fungsi unsubscribe
export const inWails: boolean;
```
  `call` memakai `window.go.main.Bridge.Call` dan `onEvent` memakai `window.runtime.EventsOn('daemon:event', ...)`. Tidak bergantung pada folder `wailsjs/` hasil generate (agar build tidak membutuhkan Wails CLI); akses lewat `window` dengan tipe deklarasi sendiri. Bila `inWails` false (mis. `vite dev` di browser) `call` menolak dengan error "bukan di dalam Wails".

### Yang dipakai epic ini
Tidak ada dari epic lain. Dependensi eksternal: Go (versi terpasang), Node + npm, `github.com/wailsapp/wails/v2`. Wails CLI dan Xcode Command Line Tools (cgo untuk WebKit) hanya diperlukan untuk `build-app`.

### Konvensi penamaan untuk epic lain (dijaga di tiket ini)
Method `ping`, `daemon.info`, `events.subscribe`, `events.unsubscribe` milik epic ini. Prefix `session.*`, `workspace.*`, `config.*`, `fs.*`, `provider.*`, `status.*`, `mention.*` dicadangkan untuk epic masing-masing; `ipc` tidak membatasi, tetapi `Register` panik pada nama ganda.

## Fitur

### go-skeleton-build: Go module dan skrip build

Deskripsi:
- `go.mod` dengan `module aiterm` dan versi Go sesuai toolchain terpasang; `go.sum` dihasilkan `go mod tidy`.
- `internal/paths` sesuai kontrak (DataDir, SocketPath, env override `AITERM_DATA_DIR`, direktori dibuat dengan mode 0700).
- `Makefile` dengan target: `build` (urut: `build-ui`, `build-daemon`, `build-app`), `build-daemon` (`go build -o build/bin/aitermd ./cmd/aitermd` dan `build/bin/aitermctl ./cmd/aitermctl`), `build-ui` (`cd frontend && npm install && npm run build`), `build-app` (lihat catatan di bawah), `run-daemon`, `clean`, `check` (`go build ./...`).
- Catatan `build-app`: `//go:embed` tidak bisa merujuk ke luar direktori paketnya, sedangkan frontend berada di `frontend/`. Jadi `build-app` menyalin `frontend/dist/` ke `app/dist/` lalu menjalankan `go build -tags desktop,production -o build/bin/aiterm ./app`. Agar `go build ./...` tetap sukses sebelum frontend pernah di-build, `app/dist/.gitkeep` ada di repo (embed memakai `all:dist`) dan `.gitignore` mengabaikan isi `app/dist/` kecuali `.gitkeep`.
- `scripts/build.sh`: pembungkus `make build` dengan `set -euo pipefail`, cek keberadaan `go` dan `npm`, pesan error jelas bila hilang.
- `.gitignore`: `build/`, `frontend/node_modules/`, `frontend/dist/`, `app/dist/*` (kecuali `.gitkeep`), `.DS_Store`, `*.sock`.
- `README.md` singkat: apa ini, prasyarat, `make build`, cara menjalankan daemon (`build/bin/aitermd`), mencoba `build/bin/aitermctl ping`, dan menjalankan app.

Acceptance criteria:
- [ ] `go.mod` ada dengan `module aiterm`; `go mod tidy` bersih.
- [ ] `internal/paths` mengembalikan DataDir di `~/Library/Application Support/aiterm` dan membuatnya bila belum ada; env `AITERM_DATA_DIR` meng-override.
- [ ] `make build-daemon` menghasilkan `build/bin/aitermd` dan `build/bin/aitermctl`.
- [ ] `make build` sukses dari checkout bersih (setelah `go mod tidy` dan `npm install`).
- [ ] `go build ./...` sukses bahkan sebelum `build-ui` dijalankan (berkat `app/dist/.gitkeep`).
- [ ] `.gitignore` dan `README.md` ada; README memuat langkah build dan jalan.

ownedPaths: `go.mod`, `go.sum`, `Makefile`, `.gitignore`, `README.md`, `scripts/build.sh`, `internal/paths/`, `app/dist/.gitkeep`.

Paralel: dapat dikerjakan paralel dengan fitur lain di epic ini; fitur lain memakai `internal/paths` dan `go.mod`, jadi sepakati dulu signature `paths` di atas (bisa distub sementara). Target `build` baru lulus penuh setelah ketiga fitur selesai.

### ipc-transport: Transport IPC JSON-RPC

Deskripsi: implementasi `internal/ipc` sesuai bagian Kontrak: `server.go` (listener, loop koneksi, dispatch, subscription per koneksi, Publish dengan buffer), `client.go` (Dial, Call, Subscribe, reader goroutine yang memisahkan response dan notifikasi `event`), `errors.go` (kode dan tipe `Error`), `protocol.go` (struct pesan request/response/notification), dan `builtin.go` (`ping`, `daemon.info`, `events.subscribe`, `events.unsubscribe`). Tanpa polling: semuanya digerakkan I/O dan channel. Tangani penutupan: server berhenti bersih saat ctx batal (tutup listener, tutup koneksi, hapus file socket); client menolak `Call` yang sedang menunggu dengan error saat koneksi putus. Socket basi (file ada tetapi tidak ada yang listen) dihapus saat start; bila socket masih hidup (ada daemon lain), `Serve` mengembalikan error "daemon sudah berjalan".

Acceptance criteria:
- [ ] `go build ./internal/ipc/...` sukses; API persis sesuai Kontrak.
- [ ] Request tanpa `id` diperlakukan sebagai notifikasi (tanpa respons); method tak dikenal menghasilkan `-32601`; JSON rusak menghasilkan `-32700`.
- [ ] `Register` dengan nama ganda panik; handler yang mengembalikan `*ipc.Error` dipetakan dengan kode yang sama.
- [ ] `Publish(topic, payload)` hanya sampai ke koneksi dengan prefix langganan yang cocok; prefix kosong menerima semua.
- [ ] Pesan sangat besar (beberapa MB per baris) bisa dikirim dan diterima tanpa terpotong.
- [ ] Socket dibuat dengan izin 0600 dan dihapus saat shutdown; socket basi dibersihkan saat start.
- [ ] `Call` aman dipanggil konkuren; koneksi putus membangunkan semua `Call` yang menunggu dengan error.

ownedPaths: `internal/ipc/`.

Paralel: bisa dikerjakan paralel dengan `go-skeleton-build` dan `wails-hello-ui`. `daemon-hello` bergantung pada API ini, jadi implementasikan signature Kontrak lebih dulu (stub) agar `daemon-hello` bisa mulai.

### daemon-hello: Daemon placeholder dan registri modul

Deskripsi:
- `cmd/aitermd/modules.go`: `Deps`, `RegisterModule`, `Provide`/`Lookup`, `OnReady`, dan fungsi internal `wireAll(srv, deps)` yang menjalankan modul terurut nama lalu callback `OnReady`.
- `cmd/aitermd/main.go`: tentukan DataDir/SocketPath via `internal/paths`, buat logger `slog` (stderr), buat `ipc.NewServer`, `SetInfo` (version, pid, startedAt, dataDir, socket, modules), `wireAll`, lalu `Serve` dengan ctx yang dibatalkan oleh SIGINT/SIGTERM. Versi di-set lewat variabel `version` (default `"dev"`, bisa di-override `-ldflags`). Daemon placeholder tidak melakukan apa pun selain melayani IPC dan menulis log start/stop.
- `cmd/aitermctl/registry.go`: `Command`, `RegisterCommand`, daftar terurut. `cmd/aitermctl/main.go`: parsing flag `--socket`, `help`, dispatch, output JSON, exit code. Daftarkan subcommand `ping` (memanggil `ping`) dan `info` (memanggil `daemon.info`) lewat `init()` di file milik epic ini (`main.go`).

Acceptance criteria:
- [ ] `go build ./cmd/...` sukses.
- [ ] `build/bin/aitermd` berjalan, membuat socket di DataDir, dan berhenti bersih saat SIGINT (socket terhapus).
- [ ] Menjalankan daemon kedua saat yang pertama hidup gagal dengan pesan "daemon sudah berjalan" (exit code non-nol).
- [ ] `build/bin/aitermctl ping` mencetak JSON `{"pong":true,...}`; `aitermctl info` mencetak info daemon termasuk `modules` (kosong di tiket ini).
- [ ] `aitermctl help` mencantumkan command terdaftar; command tak dikenal menghasilkan pesan dan exit code 1; daemon mati menghasilkan pesan jelas, bukan panic.
- [ ] Menambah file `cmd/aitermd/wire_x.go` dengan `init(){ RegisterModule(...) }` cukup untuk mendaftarkan modul tanpa mengedit file lain (diverifikasi manual dengan modul contoh sementara lalu dihapus).

ownedPaths: `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go`.

Paralel: bergantung pada kontrak `internal/ipc` dan `internal/paths`; bisa dikerjakan paralel bila memakai signature Kontrak, dan baru bisa dijalankan penuh setelah `ipc-transport` selesai.

### wails-hello-ui: Wails app dan frontend Hello World

Deskripsi:
- `wails.json`: `name: "aiterm"`, `frontend:dir: "frontend"`, `frontend:install: "npm install"`, `frontend:build: "npm run build"`, `outputfilename: "aiterm"`. Dipertahankan untuk tooling Wails (mis. `wails dev`); guard build tidak bergantung pada Wails CLI.
- `app/main.go` (`package main`): `//go:embed all:dist` untuk aset, `wails.Run` dengan judul "AI Terminal", ukuran awal 1200x800, `Bind: []any{bridge}`, `OnStartup` menyimpan ctx ke bridge, `OnShutdown` menutup koneksi daemon. Tidak mengatur daemon autostart (itu tiket 06).
- `app/bridge.go`: `Bridge` sesuai Kontrak (Call, koneksi malas + reconnect, relay event ke `daemon:event`). Event relay dimulai begitu koneksi berhasil; bila koneksi putus, coba sambung ulang secara event-driven pada `Call` berikutnya (tanpa timer polling).
- Frontend: `package.json` (Svelte 5, Vite, TypeScript, `@sveltejs/vite-plugin-svelte`, `svelte-check` opsional; skrip `build` = `vite build`, `dev` = `vite`), `vite.config.ts` (`base: './'`), `tsconfig.json` (termasuk `vite/client` types agar `import.meta.glob` terbaca), `index.html`, `src/main.ts` (import glob modul eager lalu mount `App`), `src/App.svelte` (judul "Hello World", tombol/auto-call `ping` saat mount, menampilkan hasil atau pesan galat daemon, dan menampilkan jumlah panel terdaftar per area sebagai bukti registri hidup), `src/lib/registry.ts`, `src/lib/ipc.ts` sesuai Kontrak.
- `App.svelte` hanyalah placeholder; tiket 06 menggantinya total, jadi jangan menaruh logika yang dipakai modul lain di dalamnya (taruh di `lib/`).

Acceptance criteria:
- [ ] `cd frontend && npm install && npm run build` sukses dan menghasilkan `frontend/dist/`.
- [ ] `go build ./...` sukses setelah `frontend/dist` disalin ke `app/dist` (dan juga sebelumnya, berkat `.gitkeep`).
- [ ] `make build-app` menghasilkan `build/bin/aiterm`; menjalankannya membuka window yang menampilkan "Hello World".
- [ ] Dengan `aitermd` berjalan, window menampilkan hasil ping (`pong`); tanpa daemon, window menampilkan pesan "daemon tidak berjalan" tanpa crash.
- [ ] `registerPanel`/`registerTabType` tersedia dan reaktif; `import.meta.glob('./modules/*/index.ts')` tidak membuat build gagal saat folder `modules/` belum ada.
- [ ] `ipc.ts` tidak mengimpor `wailsjs/` hasil generate; `onEvent` menerima event `daemon:event` dari bridge (diverifikasi dengan `aitermctl` + modul sementara yang mem-publish, atau cukup lewat review kode bila belum ada publisher).

ownedPaths: `wails.json`, `app/main.go`, `app/bridge.go`, `frontend/package.json`, `frontend/vite.config.ts`, `frontend/tsconfig.json`, `frontend/index.html`, `frontend/src/main.ts`, `frontend/src/App.svelte`, `frontend/src/lib/registry.ts`, `frontend/src/lib/ipc.ts`.

Paralel: bagian frontend murni (`frontend/*`) dapat dikerjakan paralel dengan semua fitur lain. `app/*` memakai `internal/ipc` (butuh signature Kontrak) dan baru bisa diuji penuh setelah `ipc-transport` dan `daemon-hello` selesai.

## Path yang dimiliki

Hanya path berikut yang boleh disentuh epic ini:
- `go.mod`, `go.sum`, `Makefile`, `wails.json`, `.gitignore`, `README.md`
- `scripts/build.sh`
- `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`
- `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go`
- `app/main.go`, `app/bridge.go`, `app/dist/.gitkeep` (tambahan agar embed tidak gagal sebelum frontend di-build)
- `internal/ipc/`, `internal/paths/`
- `frontend/package.json`, `frontend/vite.config.ts`, `frontend/tsconfig.json`, `frontend/index.html`
- `frontend/src/main.ts`, `frontend/src/App.svelte`, `frontend/src/lib/registry.ts`, `frontend/src/lib/ipc.ts`
- Artefak build (`build/`, `frontend/dist/`, `frontend/node_modules/`, `app/dist/*`) bukan kode sumber dan di-gitignore.

Tiket lain akan menambah file baru di `cmd/aitermd/wire_*.go`, `cmd/aitermctl/cmd_*.go`, `frontend/src/modules/*`, `internal/*`; tiket ini tidak boleh membuatnya.

## Wiring

Epic ini adalah dasar, jadi "wiring" berarti memastikan titik ekstensi siap dipakai epic lain tanpa mengedit file epic ini:
- Daemon: `RegisterModule` + `Deps.Provide/Lookup/OnReady` di `modules.go`; `main.go` memanggil `wireAll` sebelum `Serve`. File `wire_<modul>.go` milik epic lain otomatis ikut ter-compile karena satu package `main`.
- CLI: `RegisterCommand` di `registry.go`; `cmd_<modul>.go` milik epic lain otomatis ikut.
- Frontend: `main.ts` memuat `./modules/*/index.ts` otomatis; modul memakai `registerPanel`/`registerTabType`.
- Wails: `Bridge.Call` dan event `daemon:event` adalah satu-satunya jalur UI ke daemon; `lib/ipc.ts` adalah satu-satunya pembungkusnya.
- `Makefile` target `build` sudah memanggil `go build` untuk `./cmd/...` dan `./app`, sehingga epic lain tidak perlu mengubahnya.
- Tiket 06 (UI shell) mengambil alih `frontend/src/App.svelte` sepenuhnya dan menambah `app/daemon.go` untuk autostart daemon; tiket ini cukup tidak menyertakan keduanya.

## Guard

Cukup build berhasil, tanpa unit test:
- `go build ./...`
- `make build` (urut: `cd frontend && npm install && npm run build`, `go build` daemon dan CLI, lalu build app Wails dengan embed `app/dist`)
- Smoke manual (tidak otomatis): jalankan `build/bin/aitermd`, lalu `build/bin/aitermctl ping` harus mencetak `pong`; buka `build/bin/aiterm` dan pastikan Hello World + hasil ping tampil.
