# Overview Plan: Epic 01 App Base

Tiket: `docs/tickets/ai-terminal-ticket-01-app-base.md`
Spec: `docs/superpowers/specs/2026-11-21-ai-terminal-design.md`

Dokumen ini adalah desain BERSAMA. Agent per fitur hanya membaca overview ini dan tiket, jadi semua kontrak yang dipakai lintas fitur ada di sini. Bila overview dan tiket bertentangan, tiket menang.

## 1. Ringkasan epic dan urutan eksekusi

Kondisi repo saat ini: kosong (hanya `docs/` dan `.claude/`; bukan git repo). Tidak ada kode. Toolchain terpasang: Go 1.26.3, Node 24.15.0, npm 11.12.1. Wails CLI TIDAK terpasang (tidak dibutuhkan untuk guard; `build-app` memakai `go build -tags desktop,production`).

Epic membuat skeleton yang bisa di-build di macOS: modul Go `aiterm`, daemon `aitermd`, CLI `aitermctl`, app Wails + frontend Svelte Hello World, serta titik ekstensi (registri modul daemon, registri subcommand CLI, registri panel frontend, transport IPC JSON-RPC lewat unix socket).

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): `go.mod`, dependensi Wails, stub kompilasi untuk `internal/paths` dan `internal/ipc` (signature persis Kontrak), `app/dist/.gitkeep`. Selesai sebelum fitur paralel.
2. **Fitur paralel** (4 fitur, tabel di bagian 3): `go-skeleton-build`, `ipc-transport`, `daemon-hello`, `wails-hello-ui`. Semuanya mengkompilasi terhadap stub Foundation, jadi tidak saling menunggu.
3. **Wiring** (bagian 5): `go mod tidy`, hapus sisa stub, `make build`, smoke test.

## 2. Kontrak bersama

Seluruh kontrak rinci (paths, IPC, registri modul daemon, registri CLI, registri frontend, bridge Wails, `ipc.ts`) ada di bagian "Kontrak" tiket dan WAJIB diikuti persis, termasuk signature. Ringkasan identifier yang dikunci:

### Paket dan folder
- Modul Go: `module aiterm` (import path: `aiterm/internal/paths`, `aiterm/internal/ipc`).
- Go package: `internal/paths` -> `package paths`; `internal/ipc` -> `package ipc`; `cmd/aitermd` -> `package main`; `cmd/aitermctl` -> `package main`; `app` -> `package main`.
- Biner keluaran: `build/bin/aitermd`, `build/bin/aitermctl`, `build/bin/aiterm`.
- Folder frontend: `frontend/` (Svelte 5 + Vite + TS). Aset embed app: `app/dist/` (disalin dari `frontend/dist/`, `//go:embed all:dist`).

### Signature Go yang dikunci (dipakai lintas fitur)

```go
// internal/paths
func DataDir() (string, error)
func SocketPath() (string, error)

// internal/ipc
type Handler func(ctx context.Context, params json.RawMessage) (any, error)
type Error struct { Code int; Message string; Data any }   // implements error
func NewError(code int, msg string) *Error
func Errorf(code int, format string, args ...any) *Error
const ( ErrBinary = -32000; ErrConflict = -32001; ErrNotFound = -32002; ErrInvalid = -32003 )
// plus kode standar: CodeParseError -32700, CodeInvalidRequest -32600, CodeMethodNotFound -32601, CodeInvalidParams -32602, CodeInternal -32603
func NewServer(socketPath string, logger *slog.Logger) *Server
func (s *Server) Register(method string, h Handler)
func (s *Server) Publish(topic string, payload any)
func (s *Server) Serve(ctx context.Context) error
func (s *Server) Methods() []string
func (s *Server) SetInfo(fn func() map[string]any)
func Dial(socketPath string) (*Client, error)
func (c *Client) Call(method string, params any, out any) error
func (c *Client) Subscribe(topicPrefix string, fn func(topic string, payload json.RawMessage)) (unsubscribe func(), err error)
func (c *Client) Close() error
```

Nama konstanta kode standar JSON-RPC (tiket tidak menamai; ditetapkan di sini): `ErrParse = -32700`, `ErrInvalidRequest = -32600`, `ErrMethodNotFound = -32601`, `ErrInvalidParams = -32602`, `ErrInternal = -32603`. Semua di `internal/ipc/errors.go` bersama empat kode kustom (`ErrBinary`, `ErrConflict`, `ErrNotFound`, `ErrInvalid`).

`daemon-hello` dan `wails-hello-ui` hanya boleh memakai signature di atas dari `ipc`; jangan memakai simbol internal lain.

### Registri (milik fitur, dipakai epic lain)
- Daemon (`cmd/aitermd/modules.go`): `Deps{DataDir, SocketPath, Logger, Version, Started}`, `Provide`, `Lookup`, `OnReady`, `RegisterModule(name, wire)`, internal `wireAll(srv, deps)`; modul di-wire urut alfabetis; nama ganda panic.
- CLI (`cmd/aitermctl/registry.go`): `Command{Name, Summary, Run}`, `RegisterCommand`; nama ganda panic. Command `ping` dan `info` didaftarkan lewat `init()` di `cmd/aitermctl/main.go`.
- Frontend (`frontend/src/lib/registry.ts`): `registerPanel`, `registerTabType`, `panels(area)`, `tabTypes()` (Svelte store).
- Frontend IPC (`frontend/src/lib/ipc.ts`): `call`, `onEvent`, `inWails`; memakai `window.go.main.Bridge.Call` dan `window.runtime.EventsOn('daemon:event', ...)`; tanpa `wailsjs/`.

### Pesan IPC
- JSON-RPC 2.0, satu pesan per baris (`\n`), unix socket `<DataDir>/aitermd.sock`, socket mode 0600, DataDir mode 0700.
- Event server->klien: `{"jsonrpc":"2.0","method":"event","params":{"topic":string,"payload":any}}`.
- Method bawaan: `ping` -> `{"pong":true,"ts":RFC3339Nano}`; `daemon.info` -> `{version,pid,startedAt,dataDir,socket,modules,methods}`; `events.subscribe` / `events.unsubscribe` params `{"prefix":string}`.
- Bridge Wails -> frontend: event Wails `daemon:event` dengan data `{topic, payload}`.
- Pembaca baris memakai `bufio.Reader.ReadBytes('\n')`, bukan `bufio.Scanner`.

### Konvensi penamaan
- Prefix method dicadangkan epic lain: `session.*`, `workspace.*`, `config.*`, `fs.*`, `provider.*`, `status.*`, `mention.*`.
- File epic lain: `cmd/aitermd/wire_<modul>.go`, `cmd/aitermctl/cmd_<modul>.go`, `frontend/src/modules/<modul>/index.ts`, `internal/<modul>/`. Epic ini TIDAK membuatnya.
- Error pesan ke pengguna dalam bahasa Indonesia ("daemon sudah berjalan", "daemon tidak berjalan di <socket>", "bukan di dalam Wails"); identifier kode bahasa Inggris.
- Env override: `AITERM_DATA_DIR`. Versi daemon: variabel `version` (default `"dev"`, `-ldflags "-X main.version=..."`).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `go-skeleton-build` | `go.mod` final, `internal/paths` penuh, Makefile, `scripts/build.sh`, `.gitignore`, `README.md`, `app/dist/.gitkeep` | `go.mod`, `go.sum`, `Makefile`, `.gitignore`, `README.md`, `scripts/build.sh`, `internal/paths/`, `app/dist/.gitkeep` |
| `ipc-transport` | seluruh `internal/ipc` (server, client, protocol, errors, builtin) menggantikan stub Foundation | `internal/ipc/` |
| `daemon-hello` | daemon `aitermd` (main + registri modul) dan CLI `aitermctl` (main + registri) | `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go` |
| `wails-hello-ui` | `wails.json`, `app/main.go`, `app/bridge.go`, frontend Svelte (config, `main.ts`, `App.svelte`, `registry.ts`, `ipc.ts`) | `wails.json`, `app/main.go`, `app/bridge.go`, `frontend/package.json`, `frontend/vite.config.ts`, `frontend/tsconfig.json`, `frontend/index.html`, `frontend/src/main.ts`, `frontend/src/App.svelte`, `frontend/src/lib/registry.ts`, `frontend/src/lib/ipc.ts` |

Catatan: `frontend/package-lock.json`, `frontend/src/vite-env.d.ts` (atau `env.d.ts`), dan `frontend/svelte.config.js` adalah file turunan/pendukung yang secara wajar dihasilkan `wails-hello-ui`; agent itu boleh membuatnya (semuanya di dalam `frontend/` yang tidak dimiliki fitur lain).

## 4. Foundation

Ada Foundation: lihat `docs/plans/ai-terminal-ticket-01-app-base/00-foundation.md`. Isi singkat:
- `go.mod` (`module aiterm`, versi Go sesuai toolchain, `go get github.com/wailsapp/wails/v2` agar `go.sum` memuat Wails sebelum `app/` ditulis).
- Stub `internal/paths/paths.go` dan `internal/ipc/stub.go` yang kompilasi dengan signature Kontrak (isi `panic("belum diimplementasi")` atau implementasi minimal). `go-skeleton-build` menimpa isi `paths.go`; `ipc-transport` MENGHAPUS `stub.go` begitu implementasinya ada.
- `app/dist/.gitkeep`.
- Direktori kosong: `cmd/aitermd`, `cmd/aitermctl`, `app`, `frontend`, `scripts`, `build`.

Foundation sengaja tidak membuat file milik fitur selain stub di atas dan `go.mod`/`.gitkeep`.

## 5. Wiring checklist

Dikerjakan SATU agent setelah keempat fitur selesai. File wiring tidak dimiliki fitur mana pun.

1. Pastikan tidak tersisa stub: `internal/ipc/stub.go` sudah dihapus; tidak ada `panic("belum diimplementasi")` di `internal/`.
2. `go mod tidy` (membersihkan dan menyelesaikan `go.sum` untuk Wails) lalu verifikasi `go.mod` memuat `module aiterm` dan `github.com/wailsapp/wails/v2`.
3. Registri daemon: `cmd/aitermd/main.go` memanggil `wireAll(srv, deps)` sebelum `srv.Serve`, dan `SetInfo` melaporkan `modules` dari registri.
4. Registri CLI: `cmd/aitermctl/main.go` memanggil `RegisterCommand` untuk `ping` dan `info` di `init()`; `help` mendaftar terurut.
5. Frontend: `src/main.ts` memuat `import.meta.glob('./modules/*/index.ts', { eager: true })` lalu me-mount `App`; `App.svelte` memakai `lib/registry.ts` dan `lib/ipc.ts`.
6. Wails: `app/main.go` mem-bind `Bridge` (`Bind: []any{bridge}`), `OnStartup` menyimpan ctx, `OnShutdown` menutup koneksi; `//go:embed all:dist` valid; `frontend/dist` -> `app/dist` disalin oleh target `build-app`.
7. `Makefile`: target `build` = `build-ui`, `build-daemon`, `build-app` berurutan; `build-app` menyalin `frontend/dist/` ke `app/dist/` (jangan menghapus `.gitkeep`) lalu `go build -tags desktop,production -o build/bin/aiterm ./app`.
8. Kebersihan: `.gitignore` sesuai tiket; hanya path dari tiket yang ada; tidak ada file epic lain.
9. Jalankan Build guard (bagian 6) dan smoke manual: `build/bin/aitermd` jalan di background, `build/bin/aitermctl ping` mencetak `pong`, `aitermctl info` ada `modules: []`, `aitermctl help`, daemon kedua gagal "daemon sudah berjalan", SIGINT menghapus socket (gunakan `AITERM_DATA_DIR` ke direktori sementara). Window Wails tidak bisa diotomasi; cukup konfirmasi `build/bin/aiterm` terbentuk.

## 6. Build guard

Tanpa unit test. Satu-satunya guard:
- `go build ./...` (harus sukses bahkan sebelum frontend di-build, berkat `app/dist/.gitkeep`).
- `go vet ./...` boleh dipakai sebagai pemeriksaan tambahan, bukan guard wajib.
- Frontend: `cd frontend && npm install && npm run build`.
- Penuh: `make build`.

Per fitur, guard lokal yang boleh dijalankan agent paralel:
- `go-skeleton-build`: `go build ./internal/paths/...`, `make build-daemon` (hanya lulus setelah `daemon-hello` ada), `bash -n scripts/build.sh`.
- `ipc-transport`: `go build ./internal/ipc/...` (plus smoke manual lewat file Go sementara di luar ownedPaths, dihapus sesudahnya).
- `daemon-hello`: `go build ./cmd/...`.
- `wails-hello-ui`: `cd frontend && npm install && npm run build`; `go build ./app/...` (setelah `app/dist` ada).

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya. Dilarang menyentuh path fitur lain, file wiring, atau membuat file epic lain (`wire_*.go`, `cmd_*.go`, `frontend/src/modules/`).
- Dilarang menjalankan `go mod tidy`, `go get`, atau mengedit `go.mod`/`go.sum` kecuali `go-skeleton-build` (pemilik). Fitur lain yang butuh dependensi baru: tidak ada yang dibutuhkan (hanya stdlib dan Wails yang sudah ada dari Foundation).
- Dilarang mengubah signature Kontrak. Bila stub Foundation terasa kurang, kerjakan terhadap Kontrak di overview ini, bukan menebak.
- `go build` agent paralel bisa gagal sementara karena fitur lain belum selesai (mis. `daemon-hello` sebelum `ipc-transport`). Karena Foundation menyediakan stub yang kompilasi, kegagalan build akibat file fitur lain yang setengah jadi bukan urusan agent ini; build ulang hanya paket sendiri (`./internal/ipc/...`, `./cmd/...`, `./app/...`).
- Pesan galat dan teks UI dalam bahasa Indonesia; identifier kode dalam bahasa Inggris. Tanpa polling, tanpa timer interval; semua event-driven.
- Hapus file sementara/smoke test yang dibuat di luar ownedPaths sebelum selesai.
