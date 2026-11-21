# AI Terminal: Indeks Tiket

Sumber: `docs/superpowers/specs/2026-11-21-ai-terminal-design.md`

## Prinsip

- Setiap tiket harus langsung bisa dijalankan dan lolos `make build` sendiri. Tiket 1 menghasilkan base dan Hello World. Tiket 2 menambahkan sesi beserta satu tab terminal minimal di UI dan spike pengukuran, dan seterusnya.
- Epic berantai lewat wave. Tiap epic tidak boleh merusak build.
- Epic dalam wave yang sama saling independen dan bisa dikerjakan paralel.
- Konvensi umum: Go module `aiterm`, macOS saja, guard hanya `make build` (`go build ./...` plus build wails/vite). Tanpa unit test.

## Tabel Tiket

| No | Tiket | Judul | Wave | depends_on | Jumlah fitur |
|----|-------|-------|------|------------|--------------|
| 1 | ai-terminal-ticket-01-app-base | App Base | 1 | - | 4 |
| 2 | ai-terminal-ticket-02-session-pty | Session PTY | 2 | 01 | 5 |
| 3 | ai-terminal-ticket-03-file-service | File Service | 2 | 01 | 3 |
| 4 | ai-terminal-ticket-04-workspace-model | Workspace Model | 2 | 01 | 2 |
| 5 | ai-terminal-ticket-05-provider-adapters | Provider Adapters | 3 | 02 | 3 |
| 6 | ai-terminal-ticket-06-ui-shell | UI Shell | 4 | 02, 04, 05 | 4 |
| 7 | ai-terminal-ticket-07-file-viewer | File Viewer | 5 | 06, 03 | 3 |
| 8 | ai-terminal-ticket-08-session-status | Session Status | 5 | 06, 05, 02 | 3 |
| 9 | ai-terminal-ticket-09-session-mention | Session Mention | 6 | 08, 06, 05 | 3 |

Catatan: jumlah fitur per tiket mengikuti daftar fitur di tiap file tiket (revisi: tiket 2 dan 6 berbeda dari daftar awal). Total 9 tiket (tiket 10 perf-check dihapus; spike pengukurannya dipindah ke tiket 2).

Fitur per tiket:
- Tiket 2: `pty-wrapper`, `session-manager`, `session-ipc-ctl`, `terminal-tab-ui` (satu tab terminal minimal di UI, xterm.js), `spike-measure` (fitur TERAKHIR; menulis `docs/perf/spike-report.md` dan gerbang spec §9b).
- Tiket 4: `workspace-store`, `workspace-ipc-ctl`.
- Tiket 6: `daemon-autostart`, `shell-layout`, `workspace-sidebar`, `session-tabs` (banyak tab di atas `TerminalTab` dari tiket 2; terminal tidak dibuat ulang).

Catatan konflik: tiket 2 menambah dependency (`creack/pty` di `go.mod`; `@xterm/xterm` dan `@xterm/addon-fit` di `frontend/package.json`) dan membuat perubahan minimal di `frontend/src/App.svelte` (merender panel `main-tab`), semuanya file milik tiket 01. Konflik `go.mod`, `package.json`, dan `App.svelte` antar tiket diselesaikan manual saat merge dari git worktree; tidak dicegah. Tiket 6 menimpa `App.svelte` sepenuhnya.

## Urutan Eksekusi per Wave

Wave yang sama bisa dikerjakan paralel. Wave berikutnya baru dimulai setelah dependensinya selesai.

- **Wave 1:** 01 App Base
- **Wave 2:** 02 Session PTY (termasuk tab terminal minimal dan spike pengukuran sebagai fitur terakhir), 03 File Service, 04 Workspace Model (paralel)
- **Wave 3:** 05 Provider Adapters
- **Wave 4:** 06 UI Shell
- **Wave 5:** 07 File Viewer, 08 Session Status (paralel)
- **Wave 6:** 09 Session Mention

## Kontrak Bersama

1. **IPC (dibuat app-base, `internal/ipc`)**
   - JSON-RPC 2.0, satu pesan JSON per baris.
   - Transport: unix socket di `<DataDir>/aitermd.sock`. `DataDir` = `~/Library/Application Support/aiterm`, dari `internal/paths`.
   - Notifikasi server ke klien: `{"method":"event","params":{"topic":string,"payload":any}}`.
   - API Go: `srv.Register(method string, func(ctx, params json.RawMessage) (any, error))`, `srv.Publish(topic, payload)`, `client.Call(method, params, &out)`, `client.Subscribe(topicPrefix, fn)`.
   - Method bawaan: `ping`, `daemon.info`.
   - Kode error: `-32000` ERR_BINARY, `-32001` ERR_CONFLICT, `-32002` ERR_NOT_FOUND, `-32003` ERR_INVALID.

2. **Penamaan method dan topik per modul (tanpa tabrakan)**
   - `session.*`: create, list, kill, write, resize, attach. Event `session.output.<id>` berisi base64. Event `session.exit`.
   - `workspace.*` dan `config.*`.
   - `fs.*`: list, read, write, watch, unwatch. Event `fs.changed.<path>`.
   - `provider.list`.
   - `status.*`. Event `session.status`.
   - `mention.*`.
   - Id berupa string (ULID).
   - Model `Session`: `{id, workspaceId, projectId, cwd, providerId, status, createdAt}`.
   - Model `Workspace`: `{id, name, projects[{id,path,name}], config}`.

3. **Tanpa edit file bersama (registri via `init()`)**
   - Daemon: tiap epic menambah `cmd/aitermd/wire_<modul>.go` berisi `init(){ RegisterModule("<modul>", func(srv *ipc.Server, d *Deps){...}) }`. `RegisterModule` dibuat app-base di `cmd/aitermd/modules.go`.
   - CLI debug: tiap epic menambah `cmd/aitermctl/cmd_<modul>.go` dengan `init(){ RegisterCommand(...) }`.
   - Frontend: tiap epic menaruh kode di `frontend/src/modules/<modul>/index.ts`. File ini dimuat otomatis dengan `import.meta.glob`. Pendaftaran lewat `registerPanel({id, area:'sidebar'|'main-tab'|'statusbar', component, order})` dan `registerTabType({type:'session'|'file', component})` di `frontend/src/lib/registry.ts`. Registry area dan tab dibuat app-base dan dirender oleh ui-shell.

4. **Session.Launcher (session-pty, `internal/session/launcher.go`)**
   - Interface: `Launcher{ Command(s SessionSpec) (cmd string, args []string, env []string) }`. Default: login shell.
   - provider-adapters memasang implementasi provider dan menyediakan `RegisterLaunchContribution(func(*LaunchContext))` di `internal/provider/launch.go`.
   - session-status dan session-mention memakai hook ini untuk menambah env, args, dan konfigurasi MCP/hooks tanpa mengedit file provider.
   - Model `LaunchContext{Session, Provider, Env map, Args []string, MCPServers map}`.

5. **Idle API (session-status, `internal/status`)**
   - `IsIdle(sessionID) bool` dan `OnIdle(sessionID, fn)`. Dipakai oleh session-mention.
   - Nilai status event: `running | waiting | done | idle`.

6. **Kepemilikan file bersama**
   - `frontend/src/App.svelte` dimiliki app-base (placeholder Hello World), lalu diambil alih sepenuhnya oleh ui-shell. Epic lain tidak boleh mengeditnya.
   - `app/bridge.go` (milik app-base): daemon IPC client di sisi Wails. Mengekspos `Call(method, params)` dan meneruskan event ke frontend sebagai event Wails `daemon:event`.
   - `frontend/src/lib/ipc.ts` membungkus bridge tersebut.
   - Pengecualian yang diketahui (diselesaikan manual saat merge): tiket 2 menambah `creack/pty` di `go.mod`/`go.sum` dan `@xterm/xterm` + `@xterm/addon-fit` di `frontend/package.json`, serta mengedit `App.svelte` secara minimal.

8. **Terminal frontend (session-pty, `frontend/src/modules/terminal/`)**
   - Tiket 2 memiliki modul `terminal`: `TerminalTab.svelte` (prop `sessionId`), `terminal-client.ts`, pendaftaran `registerTabType({type:'session'})`, dan panel sementara `terminal-quick` (satu tab, tombol "Sesi baru").
   - Tiket 6 memakai ulang `TerminalTab` untuk banyak tab, tidak membuat terminal baru, dan hanya menghapus panel `terminal-quick`. Warna terminal dibaca dari CSS variables `--term-bg`, `--term-fg`, `--term-cursor` (fallback bawaan di komponen; didefinisikan tiket 6).

7. **Di luar cakupan tiket (fase belakang)**
   - Knowledge/Grimoire (fase 4) dan penataan project (fase 5).
   - Windows.
   - Syntax highlighting.
