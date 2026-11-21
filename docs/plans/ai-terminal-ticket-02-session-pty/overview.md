# Overview Plan: Epic 02 Session PTY

Tiket: `docs/tickets/ai-terminal-ticket-02-session-pty.md`
Spec: `docs/superpowers/specs/2026-11-21-ai-terminal-design.md`
Plan epic sebelumnya (sumber nama simbol ticket-01): `docs/plans/ai-terminal-ticket-01-app-base/overview.md`

Dokumen ini adalah desain BERSAMA. Agent per fitur hanya membaca overview ini dan tiket, jadi semua kontrak lintas fitur ada di sini. Bila overview dan tiket bertentangan, tiket menang.

## 1. Ringkasan epic dan urutan eksekusi

Epic ini bergantung pada ticket-01 (app-base) yang harus sudah tergabung: modul Go `aiterm`, `internal/ipc`, `internal/paths`, registri modul daemon, registri CLI, frontend Svelte 5 + Vite dengan `lib/registry.ts`, `lib/ipc.ts`, dan `main.ts` yang memuat `modules/*/index.ts` otomatis. Catatan: pada saat plan ini ditulis kode ticket-01 baru ada di worktree `worktree/feat-ai-terminal-ticket-01-app-base/` (belum di root); agent harus membaca kode ticket-01 yang aktual di repo kerjanya bila ada, dan bila nama simbol berbeda dari bagian 2, ikuti kode nyata.

Isi epic: PTY wrapper tipis (`creack/pty`), `Manager` sesi + scrollback 256 KiB, method IPC `session.*` + event `session.output.<id>` / `session.exit`, subcommand `aitermctl session ...`, satu tab terminal xterm.js di UI, dan spike pengukuran RAM/CPU.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): dependency (`creack/pty`, `@xterm/xterm`, `@xterm/addon-fit`) dipasang SEKALI di sini, plus stub kompilasi untuk paket `session` dan `session/pty` dengan signature persis Kontrak.
2. **Fitur paralel** (4 fitur): `pty-wrapper`, `session-manager`, `session-ipc-ctl`, `terminal-tab-ui`. Semuanya mengkompilasi terhadap stub Foundation.
3. **Wiring** (bagian 5): verifikasi gabungan, `go mod tidy`, hapus sisa stub, `make build`, smoke test.
4. **spike-measure** (fitur TERAKHIR, bukan paralel): dikerjakan setelah Wiring hijau; pengukuran manual, hanya menulis `docs/perf/spike-report.md`.

## 2. Kontrak bersama

Kontrak rinci (tipe Go, `Launcher`, `PTY`, `Manager`, scrollback, tabel method IPC, event, kontrak frontend) ada di bagian "Kontrak" tiket dan WAJIB diikuti persis. Ringkasan identifier yang dikunci:

### Paket dan folder
- `aiterm/internal/session` -> `package session` (file: `types.go`, `launcher.go`, `scrollback.go`, `manager.go`, `ipc.go`).
- `aiterm/internal/session/pty` -> `package pty` (file: `pty.go` berisi `Size` dan interface `PTY`, plus `pty_unix.go` berisi `New()` dan implementasi `creack/pty`). Nama paket `pty` bertabrakan dengan `github.com/creack/pty`: di implementasi, import library dengan alias `cpty "github.com/creack/pty"`. Di `internal/session`, import `aiterm/internal/session/pty` (tanpa alias).
- `cmd/aitermd/wire_session.go` (`package main`), `cmd/aitermctl/cmd_session.go` (`package main`).
- Frontend: `frontend/src/modules/terminal/{index.ts,TerminalTab.svelte,QuickTerminal.svelte,terminal-client.ts}`.
- Laporan: `docs/perf/spike-report.md`.

### Simbol dari ticket-01 yang dipakai (ditetapkan di plan ticket-01)
```go
// aiterm/internal/ipc
type Handler func(ctx context.Context, params json.RawMessage) (any, error)
func NewError(code int, msg string) *Error
func Errorf(code int, format string, args ...any) *Error
const ( ErrBinary = -32000; ErrConflict = -32001; ErrNotFound = -32002; ErrInvalid = -32003; ErrInvalidParams = -32602 )
func (s *Server) Register(method string, h Handler)
func (s *Server) Publish(topic string, payload any)
func Dial(socketPath string) (*Client, error)
func (c *Client) Call(method string, params any, out any) error
func (c *Client) Subscribe(topicPrefix string, fn func(topic string, payload json.RawMessage)) (unsubscribe func(), err error)
func (c *Client) Close() error
// aiterm/internal/paths
func SocketPath() (string, error)
```
Daemon: `RegisterModule(name string, wire func(srv *ipc.Server, d *Deps))` di `cmd/aitermd/modules.go`. CLI: `RegisterCommand(Command{Name, Summary, Run})` di `cmd/aitermctl/registry.go`; baca signature `Run` dari kode nyata. Frontend: `registerPanel`, `registerTabType` (`lib/registry.ts`), `call`, `onEvent` (`lib/ipc.ts`).

### Tipe dan API epic ini (ringkas, rinci di tiket)
```go
// session
type Status string // "running" | "exited"; konstanta StatusRunning, StatusExited
type Session struct{ ID, WorkspaceID, ProjectID, Cwd, ProviderID string; Status Status; CreatedAt time.Time; ExitCode *int } // tag JSON camelCase sesuai tiket
type SessionSpec struct{ WorkspaceID, ProjectID, Cwd, ProviderID string; Cols, Rows uint16; Env []string }
type Launcher interface{ Command(s SessionSpec) (cmd string, args []string, env []string) }
func DefaultLauncher() Launcher
const ScrollbackSize = 256 * 1024
type AttachResult struct{ Session Session; Scrollback []byte; Offset uint64 }
func NewManager(pub func(topic string, payload any)) *Manager
// metode: Create, List, Get, Kill, Write, Resize, Attach, SetLauncher, Shutdown
// error domain (var di types.go): ErrNotFound, ErrInvalid, ErrNotRunning (errors.New)

// session/pty
type Size struct{ Cols, Rows uint16 }
type PTY interface{ Start(cmd string, args []string, env []string, cwd string, size Size) error; Read(p []byte) (int, error); Write(p []byte) (int, error); Resize(size Size) error; Close() error; Wait() (exitCode int, err error) }
func New() PTY

// session/ipc.go (satu-satunya file session yang boleh impor internal/ipc)
func RegisterIPC(srv *ipc.Server, m *Manager)
```
Pemetaan error di `RegisterIPC`: `session.ErrNotFound` -> `ipc.ErrNotFound` (-32002), `session.ErrInvalid` -> `ipc.ErrInvalid` (-32003), `session.ErrNotRunning` -> `ipc.ErrConflict` (-32001); base64 tidak valid -> `ipc.ErrInvalid`; JSON params rusak -> `ipc.ErrInvalidParams`.

### Pesan IPC (tetap sesuai tiket)
- Method: `session.create|list|kill|write|resize|attach` (params/result persis tabel tiket, `data` dan `scrollback` base64 std encoding).
- Event: `session.output.<id>` payload `{"data": base64, "offset": uint64}` (offset = total byte kumulatif SESUDAH potongan); `session.exit` payload `{"id": string, "exitCode": number}` tepat sekali per sesi.
- Alur klien: subscribe SEBELUM `session.attach`; buang event dengan `offset <= offset attach`.
- Kunci catatan: `session.list` selalu mengembalikan `{"sessions": []}` (bukan `null`) saat kosong.

### Konvensi penamaan
- ID sesi: ULID 26 karakter, fungsi privat `newID()` di `types.go`.
- Default `ProviderID` = `"shell"`; default size 80x24; batas 64 sesi exited yang disimpan.
- Pesan galat dan teks UI bahasa Indonesia ("[sesi berakhir]", "Sesi baru", "Tutup sesi"); identifier kode bahasa Inggris.
- Tanpa polling: tidak ada `time.Ticker` dan tidak ada `setInterval`; semua event-driven. Timeout 2 detik di `Kill` memakai `time.After`/`time.Timer` satu kali, bukan ticker.
- Variabel CSS terminal: `--term-bg`, `--term-fg`, `--term-cursor` dengan fallback gelap.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `pty-wrapper` | Interface `PTY`, `Size`, dan implementasi `creack/pty` (Start/Read/Write/Resize/Close idempoten/Wait dengan kode `128+signal`). Tidak ada goroutine di paket ini. | `internal/session/pty/` |
| `session-manager` | Tipe, `Launcher` default login shell, ring buffer scrollback 256 KiB, `Manager` (Create/List/Get/Kill/Write/Resize/Attach/SetLauncher/Shutdown), ULID, satu goroutine pembaca per sesi, event output/exit via `pub`. | `internal/session/types.go`, `internal/session/launcher.go`, `internal/session/scrollback.go`, `internal/session/manager.go` |
| `session-ipc-ctl` | `RegisterIPC` (enam method, pemetaan error), modul daemon `session`, subcommand `aitermctl session new|list|attach|kill`. | `internal/session/ipc.go`, `cmd/aitermd/wire_session.go`, `cmd/aitermctl/cmd_session.go` |
| `terminal-tab-ui` | Modul frontend `terminal` (xterm.js): `TerminalTab`, `terminal-client`, `QuickTerminal`, `index.ts`; satu tab/sesi. | `frontend/src/modules/terminal/` (+ pengecualian: `frontend/src/App.svelte` hanya blok `{#each}` panel `main-tab`; dependency xterm sudah dipasang Foundation) |
| `spike-measure` | Pengukuran manual RAM/CPU dan laporan gerbang. DIKERJAKAN TERAKHIR setelah Wiring, tidak paralel. | `docs/perf/spike-report.md` |

Catatan `ownedPaths` `terminal-tab-ui`: tiket memberi pengecualian `frontend/package.json` + lockfile untuk dua dependency xterm. Karena Foundation sudah memasangnya, agent fitur ini TIDAK perlu menyentuh `package.json`/lockfile; hanya boleh bila ternyata Foundation gagal memasangnya (cek dulu, tambah seminimal mungkin).

## 4. Foundation

Ada Foundation: `docs/plans/ai-terminal-ticket-02-session-pty/00-foundation.md`. Isi singkat:
- Verifikasi prasyarat ticket-01 (ada `internal/ipc`, `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go`, `frontend/src/lib/registry.ts`); catat nama simbol nyata.
- `go get github.com/creack/pty` (hanya Foundation yang mengubah `go.mod`/`go.sum`).
- `cd frontend && npm install @xterm/xterm @xterm/addon-fit` (hanya Foundation yang mengubah `package.json`/lockfile).
- Stub kompilasi dengan signature persis Kontrak: `internal/session/pty/pty.go` (interface final), `internal/session/pty/pty_unix.go` (stub `New()`), `internal/session/{types,launcher,scrollback,manager}.go`. Pemilik fitur menimpa isi file stubnya; `session-ipc-ctl` mengkompilasi terhadap stub `Manager`.
- Stub `internal/session/ipc.go` TIDAK dibuat (milik `session-ipc-ctl`, dibuat dari nol).
- Direktori kosong: `frontend/src/modules/terminal/`, `docs/perf/`.

## 5. Wiring checklist

Epic ini hampir tidak punya file wiring bersama (pendaftaran lewat `init()` di file milik fitur dan `import.meta.glob` ticket-01). Satu agent Wiring setelah keempat fitur paralel selesai, SEBELUM `spike-measure`:

1. Pastikan tidak tersisa stub: tidak ada `panic("belum diimplementasi")` di `internal/session/`; `pty_unix.go` berisi implementasi nyata; tidak ada import `github.com/creack/pty` di luar `internal/session/pty/`; paket `session` selain `ipc.go` tidak mengimpor `internal/ipc`.
2. `go mod tidy`; verifikasi `go.mod` memuat `github.com/creack/pty` dan `go.sum` lengkap. Bila `go.mod` juga memuat `golang.org/x/term`, tidak apa-apa; bila tidak, pastikan `cmd_session.go` tidak mengimpornya.
3. Daemon: `cmd/aitermd/wire_session.go` memanggil `RegisterModule("session", ...)` dalam `init()` dan modul muncul di `daemon.info` (`modules` memuat `session`, `methods` memuat enam method `session.*`).
4. CLI: `aitermctl help` mencantumkan `session`.
5. Frontend: `frontend/src/modules/terminal/index.ts` memanggil `registerTabType({type:'session', ...})` dan `registerPanel({id:'terminal-quick', area:'main-tab', ...})`; `App.svelte` merender panel `main-tab` (satu `{#each}`); tidak ada edit ke `lib/registry.ts`, `lib/ipc.ts`, `main.ts`, atau `app/`.
6. Daemon berhenti: bila ticket-01 menyediakan hook shutdown/`OnReady` yang mudah dipakai tanpa mengedit file milik ticket lain, panggil `Manager.Shutdown`; bila tidak, biarkan (tiket menyatakan cukup disediakan). Jangan mengedit `cmd/aitermd/main.go`/`modules.go`.
7. Jalankan Build guard (bagian 6) lalu smoke manual dengan `AITERM_DATA_DIR` ke direktori sementara: `aitermd` di background; `aitermctl session new` mencetak ID; `session list` menampilkan `running`; `session attach <id>` menjalankan `ls`, lepas dengan Ctrl+]; `session list` masih `running`; attach ulang menampilkan scrollback; `session kill <id>`; `session.write` ke ID palsu -> not found. Window Wails tidak bisa diotomasi; cukup konfirmasi `build/bin/aiterm` terbentuk dan bundle frontend memuat modul terminal.
8. Kebersihan: hanya path dari tiket (bagian "Path yang dimiliki") yang berubah, ditambah pengecualian go.mod/go.sum/package.json/lockfile/App.svelte; tidak ada file smoke tersisa.
9. Setelah Wiring hijau, jalankan `spike-measure` sebagai langkah terpisah terakhir.

## 6. Build guard

Tanpa unit test. Satu-satunya guard:
- `go build ./...` (harus lolos pada gabungan ticket-01 + epic ini).
- Frontend: `cd frontend && npm install && npm run build`.
- Penuh: `make build`.
- Tambahan opsional: `go vet ./internal/session/... ./cmd/aitermd/... ./cmd/aitermctl/...`.

Guard lokal per fitur:
- `pty-wrapper`: `go build ./internal/session/pty/...`; `grep -r "creack/pty" --include=*.go .` hanya cocok di `internal/session/pty/`.
- `session-manager`: `go build ./internal/session/...` (butuh `pty` kompilasi; Foundation menyediakan stub).
- `session-ipc-ctl`: `go build ./internal/session/... ./cmd/...`.
- `terminal-tab-ui`: `cd frontend && npm run build`.
- `spike-measure`: bukan guard build; hasilnya dokumen.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya. Dilarang menyentuh path fitur lain, `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go`, `internal/ipc/`, `internal/paths/`, `frontend/src/lib/`, `frontend/src/main.ts`, `app/`.
- Dilarang menjalankan `go get`, `go mod tidy`, `npm install <paket>`, atau mengedit `go.mod`/`go.sum`/`package.json`/lockfile (semua sudah dikerjakan Foundation; `go mod tidy` oleh Wiring). `golang.org/x/term` tidak boleh ditambahkan; bila belum ada di `go.mod`, `session-ipc-ctl` memakai `stty` lewat `os/exec` atau mode baris.
- Dilarang mengubah signature Kontrak. Bila stub Foundation terasa kurang, kerjakan terhadap Kontrak, bukan menebak.
- Kegagalan `go build` akibat file fitur lain yang setengah jadi bukan urusan agent ini; build ulang hanya paket sendiri.
- `terminal-tab-ui` tidak menambah logika di `App.svelte` selain satu blok `{#each}` untuk panel `main-tab`; tanpa addon xterm berat (tanpa webgl), tanpa `setInterval`.
- Tanpa polling, tanpa ticker; semua event-driven.
- Hapus file sementara/smoke test di luar ownedPaths sebelum selesai.
- `spike-measure` hanya menulis `docs/perf/spike-report.md` dengan angka hasil ukur nyata; tidak ada angka karangan.
