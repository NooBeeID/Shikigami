# Foundation: Epic 02 Session PTY

Baca dulu `overview.md` di folder ini dan tiket `docs/tickets/ai-terminal-ticket-02-session-pty.md`. Tujuan: pasang dependency sekali dan sediakan stub kompilasi supaya empat fitur paralel tidak saling menunggu dan tidak berebut `go.mod`/`package.json`.

Tanpa unit test. Guard: `go build ./...`.

## Langkah

### 1. Verifikasi prasyarat ticket-01
- Pastikan ada: `go.mod` (`module aiterm`), `internal/ipc/`, `internal/paths/`, `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go`, `frontend/package.json`, `frontend/src/lib/registry.ts`, `frontend/src/lib/ipc.ts`, `frontend/src/main.ts`, `frontend/src/App.svelte`.
- Bila ticket-01 belum tergabung di repo kerja, hentikan dan laporkan (jangan menulis ulang ticket-01).
- Catat nama simbol nyata (`ipc.ErrNotFound` dst., signature `RegisterCommand`, `RegisterModule`) bila berbeda dari overview; tulis temuan di laporan akhir agar agent fitur tahu.

### 2. Dependency Go
- `go get github.com/creack/pty` (versi stabil terbaru, v1.1.x). Jangan ubah baris lain di `go.mod`.
- Jangan menambah `golang.org/x/term`.

### 3. Dependency frontend
- `cd frontend && npm install @xterm/xterm @xterm/addon-fit`. Hanya dua dependency itu.
- Pastikan `package.json` dan `package-lock.json` berubah hanya untuk keduanya.

### 4. Stub paket `internal/session/pty`
`internal/session/pty/pty.go` (FINAL, bukan stub; `pty-wrapper` boleh merapikan komentar tapi tidak mengubah signature):
```go
package pty

type Size struct{ Cols, Rows uint16 }

type PTY interface {
    Start(cmd string, args []string, env []string, cwd string, size Size) error
    Read(p []byte) (int, error)
    Write(p []byte) (int, error)
    Resize(size Size) error
    Close() error
    Wait() (exitCode int, err error)
}
```
`internal/session/pty/pty_unix.go` (STUB, ditimpa `pty-wrapper`): `func New() PTY` mengembalikan tipe `stub` yang mengimplementasi semua metode dengan `panic("belum diimplementasi")` (atau error). Harus kompilasi.

### 5. Stub paket `internal/session`
Semua signature persis tiket/overview; isi minimal yang kompilasi. Paket ini TIDAK mengimpor `internal/ipc`.
- `types.go`: `Status`, konstanta `StatusRunning = "running"`, `StatusExited = "exited"`, `Session` (tag JSON sesuai tiket), `SessionSpec`, `AttachResult`, var error `ErrNotFound`, `ErrInvalid`, `ErrNotRunning` (`errors.New`, pesan Indonesia), `func newID() string` stub.
- `launcher.go`: interface `Launcher`, `func DefaultLauncher() Launcher` stub.
- `scrollback.go`: `const ScrollbackSize = 256 * 1024`, tipe `scrollback` stub dengan `Append([]byte)` dan `Snapshot() ([]byte, uint64)`.
- `manager.go`: `Manager`, `NewManager(pub func(topic string, payload any)) *Manager`, metode `Create, List, Get, Kill, Write, Resize, Attach, SetLauncher, Shutdown` dengan signature Kontrak; isi stub (`return nil, ErrNotFound` / `panic("belum diimplementasi")`).
- JANGAN membuat `ipc.go` (milik `session-ipc-ctl`).

### 6. Direktori
- Buat `frontend/src/modules/terminal/.gitkeep` dan `docs/perf/.gitkeep` (hapus `.gitkeep` hanya bila fitur pemilik menaruh file nyata; tidak wajib).

### 7. Verifikasi
- `go build ./...` sukses.
- `cd frontend && npm run build` sukses.
- Laporkan daftar file yang dibuat dan versi `creack/pty`, `@xterm/xterm`, `@xterm/addon-fit` yang terpasang.
