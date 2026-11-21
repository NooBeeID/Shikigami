# Overview Plan: Epic 08 Session Status

Tiket: `docs/tickets/ai-terminal-ticket-08-session-status.md`
Spec: `docs/superpowers/specs/2026-11-21-ai-terminal-design.md` (bagian "Status sesi dan biaya", fase 3)

Dokumen ini adalah desain BERSAMA. Agent per fitur hanya membaca overview ini dan tiket. Bila overview dan tiket bertentangan, tiket menang, KECUALI untuk dua temuan kode nyata di bagian 2.1 (ID sesi belum ada saat launch, dan ipc server tidak punya langganan in-process): di sana kode yang sudah ada adalah sumber kebenaran dan overview ini menetapkan solusinya.

## 1. Ringkasan epic dan urutan eksekusi

Epic ini memberi tiap sesi status `running | waiting | done | idle` berbasis event hooks provider (Claude: UserPromptSubmit, Notification, Stop, SessionStart), dengan cadangan deteksi output-diam (timer sekali jalan, tanpa ticker) untuk provider tanpa hooks. Status terbit sebagai event IPC `session.status`, tampil sebagai lencana di UI, memicu notifikasi macOS, dan menyediakan `status.IsIdle` / `status.OnIdle` untuk epic mention (tiket 09).

Kondisi kode saat plan ditulis (dibaca langsung dari repo):
- `internal/ipc` ada: `Server.Register`, `Server.Publish(topic, payload)`, `Client.Call`, `Client.Subscribe(prefix, fn(topic, payload json.RawMessage))`, `ipc.Dial(socket)`, `ipc.ErrNotFound = -32002`, `ipc.ErrInvalid = -32003`, `ipc.NewError/Errorf`.
- `internal/session` ada: `Manager` (`Get(id) (*Session, error)`, `List() []Session`), `Session{ID, WorkspaceID, ProjectID, Cwd, ProviderID, Status("running"|"exited"), CreatedAt, ExitCode}`, `SessionSpec` (TANPA ID). Event `session.output.<id>` payload `{data(base64), offset}` dan `session.exit` payload `{id, exitCode}`. `session.list` mengembalikan `{"sessions":[...]}`.
- `internal/provider` ada: `RegisterLaunchContribution(func(*LaunchContext))`, `Get(id) (Provider, bool)`, `Capabilities{Hooks, MCP, IdleEvent}` (claude: Hooks true; shell dan codex: false), `IDClaude = "claude"`.
- `cmd/aitermd/modules.go`: `RegisterModule(name, func(srv *ipc.Server, d *Deps))`, `Deps{DataDir, SocketPath, Logger, ...}`, `d.Provide/Lookup`, `d.OnReady(fn)`. Modul dipasang alfabetis; `session.manager` (`*session.Manager`) disediakan modul `session` lewat `d.Provide`.
- `cmd/aitermctl/registry.go`: `RegisterCommand(Command{Name, Summary, Run func(ctx, c *ipc.Client, args []string) error})`, helper `printJSON`, `callAndPrint`. Catatan: `main.go` aitermctl MELAKUKAN `ipc.Dial` sebelum menjalankan command dan keluar kode 1 bila daemon mati. Untuk `status hook` (harus selalu exit 0) lihat 2.7.
- `frontend/src/lib/registry.ts`: `registerPanel({id, area: 'sidebar'|'main-tab'|'statusbar', component, order?})`. `frontend/src/lib/ipc.ts`: `call<T>(method, params?)`, `onEvent(topicPrefix, fn(topic, payload)) => unsubscribe`. Bridge Wails (`app/bridge.go`) merelay SEMUA topik ke event `daemon:event`, jadi `session.status` otomatis sampai ke UI.
- `frontend/src/main.ts` memuat `./modules/*/index.ts` lewat `import.meta.glob` (eager).
- Epic ui-shell (tiket 06) belum ada di working tree utama (hanya di `worktree/feat-ai-terminal-ticket-06-ui-shell/`, berisi `frontend/src/lib/stores/{sessions,tabs,...}.ts`). Fitur frontend harus membaca kode shell yang ada saat dikerjakan; jangan mengeditnya.

Urutan eksekusi:
1. **Foundation** (`00-foundation.md`): satu agent. Membuat `internal/status/types.go` (tipe bersama), stub kompilasi untuk semua fungsi kontrak di file milik tiap fitur, dan `cmd/aitermd/wire_status.go` + stub `cmd/aitermctl/cmd_status.go` sehingga `go build ./...` hijau sejak awal.
2. **Fitur paralel** (3): `status-engine`, `hook-receiver`, `status-notify-ui`. Tiap fitur mengganti stub miliknya dengan implementasi nyata dan meninggalkan build hijau.
3. **Wiring** (bagian 5): verifikasi sambungan, build penuh, smoke test manual.

## 2. Kontrak bersama

### 2.1 Dua temuan kode nyata dan keputusannya

**T1. ID sesi belum ada saat launch contribution berjalan.** `session.Manager.Create` memanggil `Launcher.Command(spec)` SEBELUM `newID()`; `LaunchContext.Session` adalah `SessionSpec` tanpa ID. Maka `AITERM_SESSION_ID=<id sesi>` dan berkas `status-hooks-<sessionId>.json` seperti di tiket tidak mungkin memakai ID sesi asli, dan paket `internal/session` dilarang diedit. Keputusan:
- Launch contribution membuat **launch token** acak (16 hex, `crypto/rand`). Nilai token dipakai sebagai pengganti `<id>` di: env `AITERM_SESSION_ID=<token>`, nama berkas `<DataDir>/run/status-hooks-<token>.json`, dan argumen `--session <token>` pada command hook.
- Contribution memanggil `status.RegisterLaunch(LaunchInfo{Token, WorkspaceID, ProjectID, Cwd, ProviderID, At})` (milik engine) untuk mencatat peluncuran tertunda.
- Engine mengikat token ke ID sesi asli secara malas ("binding"): saat sebuah ID sesi baru terlihat pertama kali (output pertama, `status.list`, atau hook yang tokennya belum terikat), engine mengambil `Session` lewat `SessionSource` dan mencocokkan dengan launch tertunda yang `ProviderID`, `Cwd`, `ProjectID`, `WorkspaceID`-nya sama; pilih yang tertua (FIFO). Satu token hanya terikat satu sesi.
- `sessionId` pada `status.hook`, `status.get` dan CLI boleh berisi ID sesi asli ATAU launch token; engine memetakan lewat `Engine.ResolveSessionID(idOrToken) (string, bool)`. Event `session.status` dan `status.list` SELALU memakai ID sesi asli.
- Keterbatasan yang diterima (catat di komentar kode): dua sesi diluncurkan serentak dengan spec identik bisa tertukar ikatannya; dampaknya hanya status tertukar antara dua sesi yang sama persis.

**T2. `ipc.Server` tidak punya langganan in-process.** `Server.Publish` hanya mengirim ke koneksi klien; `session.Manager` dibuat dengan `srv.Publish` di `wire_session.go` (tidak boleh diedit) dan tidak punya observer. Keputusan: engine membuka SATU klien IPC internal ke socketnya sendiri (`ipc.Dial(paths.SocketPath())`) dan `Subscribe("session.output.", ...)` serta `Subscribe("session.exit", ...)`.
- Socket belum listening saat `wireAll` berjalan (`Serve` dipanggil sesudahnya), jadi koneksi dibuat MALAS lewat `Engine.ensureStream()` (idempoten, dilindungi mutex, tanpa loop retry): dipanggil dari `RegisterLaunch`, dari handler `status.*`, dan dari `Track`. Bila Dial gagal, kembalikan error/diam dan coba lagi pada panggilan berikutnya (event-driven, bukan polling). Sesi hanya bisa dibuat lewat `session.create` (socket sudah hidup), sehingga stream pasti tersambung sebelum output pertama sesi.
- Tidak ada goroutine per sesi dan tidak ada ticker.

### 2.2 Paket, folder, penamaan
- Go module `aiterm`. Paket baru `internal/status/` (package `status`). File: `types.go` (Foundation), `engine.go`, `idle.go`, `ipc.go` (status-engine), `hooks.go`, `install.go` (hook-receiver), `notify.go` (status-notify-ui).
- `cmd/aitermd/wire_status.go` (dibuat Foundation final, dimiliki `status-engine`), `cmd/aitermctl/cmd_status.go` (stub Foundation, dimiliki `status-engine`).
- Frontend: `frontend/src/modules/status/` (milik `status-notify-ui`).
- Identifier kode Inggris; komentar, pesan error, teks UI Indonesia. Hanya stdlib dan paket proyek; `go.mod` tidak berubah. Tanpa unit test.

### 2.3 Tipe Go bersama (`internal/status/types.go`, dibuat Foundation, TIDAK diubah fitur mana pun)

```go
package status

import "time"

type Status string

const (
    StatusRunning Status = "running"
    StatusWaiting Status = "waiting"
    StatusDone    Status = "done"
    StatusIdle    Status = "idle"
)

const (
    SourceHook   = "hook"
    SourceOutput = "output"
    SourceExit   = "exit"
)

// Nama event hook yang dikenal status.hook.
const (
    HookUserPromptSubmit = "UserPromptSubmit"
    HookSessionStart     = "SessionStart"
    HookNotification     = "Notification"
    HookStop             = "Stop"
    HookSubagentStop     = "SubagentStop"
)

// Jeda output-diam (konstanta; dapat diubah di sini).
const (
    SilentDelay          = 3 * time.Second  // provider tanpa hooks
    SilentDelayWithHooks = 30 * time.Second // provider berhooks, hanya cadangan sebelum hook pertama
)

type StatusEvent struct {
    SessionID string `json:"sessionId"`
    Status    Status `json:"status"`
    Prev      Status `json:"prev"`
    Source    string `json:"source"`           // SourceHook | SourceOutput | SourceExit
    Reason    string `json:"reason,omitempty"` // mis. "Stop", "Notification", "silent", "exit"
    At        int64  `json:"at"`               // unix milidetik
}

// SessionState adalah bentuk hasil status.get / elemen status.list.
type SessionState struct {
    SessionID string `json:"sessionId"`
    Status    Status `json:"status"`
    Since     int64  `json:"since"` // unix milidetik
}

// LaunchInfo mencatat peluncuran sesi yang menunggu ikatan ke ID sesi asli (lihat T1).
type LaunchInfo struct {
    Token       string
    WorkspaceID string
    ProjectID   string
    Cwd         string
    ProviderID  string
    At          time.Time
}
```

### 2.4 API Go lintas fitur (signature persis; Foundation membuat stub, pemilik mengimplementasi)

Milik `status-engine` (`engine.go`, `idle.go`, `ipc.go`):
```go
var ErrUnknownSession = errors.New("sesi tidak dikenal")

// SessionSource dipenuhi *session.Manager; dipasang malas (lihat Wiring).
type SessionSource interface {
    Get(id string) (*session.Session, error)
    List() []session.Session
}

type Engine struct { /* mutex, peta state, launch tertunda, subscriber, klien stream */ }

func NewEngine(srv *ipc.Server) *Engine           // publish lewat srv.Publish
func SetDefault(e *Engine)                         // singleton paket
func Default() *Engine                             // nil bila belum di-set

func (e *Engine) SetSessionSource(src SessionSource)
func (e *Engine) RegisterLaunch(info LaunchInfo)   // catat launch tertunda + ensureStream
func (e *Engine) ResolveSessionID(idOrToken string) (string, bool)
func (e *Engine) Track(sessionID string) bool      // daftarkan state awal idle (senyap); false bila sesi tak dikenal
func (e *Engine) Set(sessionID string, st Status, source, reason string) bool // true bila transisi nyata
func (e *Engine) Apply(idOrToken, event string, payload map[string]any) error // ErrUnknownSession bila tak dikenal
func (e *Engine) OnOutput(sessionID string)
func (e *Engine) Get(sessionID string) (SessionState, bool)
func (e *Engine) List() []SessionState
func (e *Engine) Lookup(sessionID string) (session.Session, bool) // metadata sesi untuk judul notifikasi
func (e *Engine) Subscribe(fn func(StatusEvent)) (cancel func())
func (e *Engine) Forget(sessionID string)          // hentikan timer, buang state, panggil CleanupHooks untuk token terikat

func RegisterIPC(srv *ipc.Server, eng *Engine)     // status.get, status.list, status.hook
func RegisterLaunch(info LaunchInfo)               // fungsi paket -> Default().RegisterLaunch (no-op bila nil)
func IsIdle(sessionID string) bool
func OnIdle(sessionID string, fn func()) (cancel func())
```
Milik `hook-receiver` (`hooks.go`, `install.go`):
```go
type HookPayload struct { /* field opsional, toleran field tak dikenal */ }
func ParseHookEvent(name string, stdin []byte) (event string, payload map[string]any)
func ClassifyEvent(event string) (Status, bool) // false untuk event yang tidak mengubah status/tak dikenal
func InstallHooks()                             // memanggil provider.RegisterLaunchContribution
func CleanupHooks(token string)                 // hapus <DataDir>/run/status-hooks-<token>.json
```
`ClassifyEvent` (tabel pemetaan; Foundation menulis versi final karena kecil, `hook-receiver` boleh menyempurnakan di `hooks.go`): UserPromptSubmit, SessionStart -> running; Notification -> waiting; Stop -> done; SubagentStop dan lainnya -> `("", false)`.

Milik `status-notify-ui` (`notify.go`):
```go
func StartNotifier(srv *ipc.Server, eng *Engine)  // eng.Subscribe; tanpa return
```

### 2.5 Pesan IPC dan event (namespace `status.*`)
- `status.get` `{"sessionId"}` -> `SessionState`; tak dikenal -> `ipc.ErrNotFound` (-32002); `sessionId` kosong -> `ipc.ErrInvalidParams`.
- `status.list` `{}` -> `[]SessionState` (array JSON langsung, bukan dibungkus objek; `[]` bila kosong, bukan `null`). Sebelum menjawab, engine `Track`-kan semua sesi `running` dari `SessionSource.List()` agar sesi baru tanpa event pun muncul (status awal `idle`).
- `status.hook` `{"sessionId","event","payload"}` -> `{"ok":true}`. Event tak dikenal: `ok:true` diabaikan. ID/token tak dikenal: -32002. `sessionId` atau `event` kosong: `ipc.ErrInvalidParams`.
- Event server: topik `session.status`, payload `StatusEvent`; satu per transisi nyata, tanpa pengulangan.
- Pemetaan hook: lihat `ClassifyEvent` di 2.4; `session.exit` -> `done` (`source:"exit"`, `reason:"exit"`), lalu `Forget`.

### 2.6 Aturan mesin status (dipegang `status-engine`)
- State awal sesi saat `Track`: `idle`, `since=now`, tanpa event. Transisi `idle -> running` pertama menerbitkan event normal.
- Sesi "berhooks" = `provider.Get(providerID)` ada dan `Capabilities().Hooks == true`, DAN sesi terikat ke launch token (artinya hooks dipasang). Sesi tanpa ikatan (mis. daemon restart) diperlakukan sebagai tanpa hooks.
- Output: tanpa hooks -> output memanggil `OnOutput`: bila status bukan `running` -> `Set(running, "output", "output")`; reset satu timer sekali jalan (`time.AfterFunc` lalu `Reset`) berjeda `SilentDelay`; saat habis -> `Set(idle, "output", "silent")`. Berhooks dan belum ada hook pertama -> sama, jeda `SilentDelayWithHooks`, dan tidak pernah menimpa `waiting`/`done`. Berhooks dan sudah ada hook (`hookSeen`) -> timer dihentikan dan output diabaikan untuk status.
- Timer dihentikan saat status `done`, saat `Forget`, dan tidak dibuat sebelum output pertama. Dilarang `time.Ticker` dan loop tidur-bangun di seluruh `internal/status/`.
- `Set` dengan status sama = tidak ada event (return false). Event dipublish via `srv.Publish("session.status", ev)` DI LUAR mutex; subscriber internal dan waiters `OnIdle` dibangunkan di luar mutex.
- Echo keystroke user terhitung output; terima keterbatasan itu dan beri komentar kode.
- `IsIdle`: true untuk `idle|done|waiting`, false untuk `running` dan sesi tak dikenal. `OnIdle(id, fn)`: bila sudah idle -> panggil `fn` di goroutine terpisah; selain itu panggil sekali pada transisi berikutnya ke status idle-menurut-IsIdle; `cancel` membatalkan; aman konkuren.
- Notifikasi/subscriber menerima semua transisi, termasuk `source:"exit"`.

### 2.7 CLI `aitermctl status ...` dan pemasangan hooks
- Subperintah: `list`, `get <sessionId>`, `watch`, `hook --session <idOrToken> --event <Nama>`. Mendaftar sebagai satu command `status` lewat `RegisterCommand`.
- `hook`: baca stdin sampai EOF (boleh kosong; batasi ~1 MiB), `payload` = JSON object bila valid, selain itu `{}`. Panggil `status.hook`, lalu SELALU `return nil`; error dicetak ke stderr saja. Karena `main.go` aitermctl keluar kode 1 bila Dial gagal sebelum command jalan, `Run` untuk `hook` tidak bisa mencegahnya. Solusi: command hook yang ditulis ke berkas hooks dibungkus `... || true` (lihat bawah), jadi agent tidak pernah gagal karena daemon mati. Dilarang mengedit `main.go` aitermctl.
- Konfigurasi hooks Claude (dimiliki `hook-receiver`): berkas `<DataDir>/run/status-hooks-<token>.json` (dir 0700, berkas 0600) berisi `{"hooks": {"<Event>": [{"hooks": [{"type":"command","command":"<cmd>"}]}]}}` untuk event `UserPromptSubmit`, `Notification`, `Stop`, `SessionStart`. `<cmd>` = `'<aitermctl absolut dengan quote tunggal>' status hook --session <token> --event <Event> || true` (quote tunggal aman untuk spasi di `Application Support`; escape `'` di path dengan `'\''`). Lokasi aitermctl: direktori `os.Executable()` (nama `aitermctl`), fallback `exec.LookPath("aitermctl")`, fallback string `aitermctl`. Argumen `--settings <path>` ditambahkan ke `ctx.Args`. Agent hook-receiver wajib memverifikasi skema `hooks` dan bahwa `--settings` menerima path berkas terhadap dokumentasi Claude Code yang berlaku, lalu mencatat hasilnya di komentar kode.
- Env ke SEMUA provider: `AITERM_SESSION_ID=<token>` (lihat T1) dan `AITERM_SOCK=<DataDir>/aitermd.sock` (`paths.SocketPath()`).
- Pembersihan: `Engine.Forget` memanggil `CleanupHooks(token)`. Tambahan: `InstallHooks()` (dipanggil sekali saat wiring) menghapus semua berkas `status-hooks-*.json` sisa daemon sebelumnya (satu kali `filepath.Glob`, bukan polling).

### 2.8 Frontend (milik `status-notify-ui`)
Folder `frontend/src/modules/status/`: `index.ts`, `store.ts`, `StatusBadge.svelte`, `SidebarBadges.svelte`, `StatusBar.svelte`.
- Tipe: `type Status = 'running'|'waiting'|'done'|'idle'`; `interface SessionStatus { status: Status; since: number }`.
- `store.ts`: `export const statusStore: Readable<Record<string, SessionStatus>>`; `startStatusSync(): () => void` (satu langganan global `onEvent('session.status', ...)` + muat awal `call('status.list')`; idempoten; mengembalikan fungsi pembersih); `statusOf(id): Readable<Status>` bila perlu. Sesi tak ada di store dianggap `idle`. Event `session.status` dengan `source:"exit"` ditampilkan sebagai `done` sampai shell menghapus sesi. Tanpa `setInterval`/polling.
- `StatusBadge.svelte`: props `sessionId: string`, `compact?: boolean`. Titik berwarna + label aksesibel (`aria-label`/`title`): running biru berdenyut (animasi CSS saja), waiting kuning, done hijau, idle abu. Warna: pakai CSS variable shell bila ada, dengan fallback hex.
- `index.ts`: `registerPanel({id:'status-badges', area:'sidebar', order: 90, component: SidebarBadges})` dan `registerPanel({id:'status-summary', area:'statusbar', order: 50, component: StatusBar})`, lalu `startStatusSync()`. Cara panel menerima data sesi mengikuti kode shell yang nyata; bila registry tidak mengizinkan lencana di baris sesi milik shell, ekspor `StatusBadge` dan tampilkan daftar ringkas sesi + lencana di panel sidebar modul ini (catat pilihan di komentar).
- Ringkasan statusbar: mis. "2 berjalan, 1 menunggu"; kosong bila tidak ada sesi.

### 2.9 Notifikasi macOS (milik `status-notify-ui`, `notify.go`)
- `StartNotifier(srv, eng)` memanggil `eng.Subscribe`. Notifikasi hanya pada event dengan `Status` `waiting` atau `done`, `Prev != Status`, DAN `Source != SourceExit` (keputusan overview: transisi dari exit tidak dinotifikasi, karena exit massal saat daemon berhenti akan menyebabkan banjir notifikasi; sesi Claude yang selesai sudah mengirim notifikasi lewat hook `Stop`).
- `osascript -e 'display notification "<msg>" with title "<title>"'` lewat `exec.Command` dengan argumen terpisah (bukan shell); escape `\` lalu `"` untuk AppleScript. Pesan: "Menunggu input Anda" / "Selesai". Judul: `<Nama Provider> · <basename(cwd)>` dari `eng.Lookup`, fallback `Sesi <8 karakter pertama id>`.
- Debounce maksimal satu notifikasi per sesi per 2 detik (peta `lastNotified` dibersihkan di `Forget`-setara: hapus entri saat event `done` dari exit tidak datang, cukup batasi ukuran peta). Kriteria "jendela app fokus": abaikan, beri komentar. `AITERM_NOTIFY=0` mematikan notifikasi (dibaca saat `StartNotifier`). Proses di-spawn hanya saat ada event notifikasi; jalankan di goroutine sekali pakai (`cmd.Run`), bukan loop.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `status-engine` | State machine status per sesi, idle fallback, `IsIdle`/`OnIdle`, IPC `status.*`, stream output/exit via klien internal, binding launch token, CLI `aitermctl status`, wiring modul daemon | `internal/status/engine.go`, `internal/status/idle.go`, `internal/status/ipc.go`, `cmd/aitermd/wire_status.go`, `cmd/aitermctl/cmd_status.go` |
| `hook-receiver` | Parser payload hook, `ClassifyEvent`, `InstallHooks` (env + berkas hooks Claude + `--settings`), `CleanupHooks` | `internal/status/hooks.go`, `internal/status/install.go` |
| `status-notify-ui` | Notifikasi macOS (`StartNotifier`) dan seluruh modul frontend status (store, badge, sidebar, statusbar) | `internal/status/notify.go`, `frontend/src/modules/status/` |

Pembagian tidak berubah dari tiket. Penyesuaian tersirat dari temuan T1/T2 hanya menambah isi (binding token, klien stream) di dalam file milik `status-engine`, dan `RegisterLaunch` dipanggil `hook-receiver` dari `install.go`.

## 4. Foundation

Ada (`hasFoundation = true`), detail langkah di `00-foundation.md`. Isi singkat:
1. `internal/status/types.go` sesuai 2.3 (satu-satunya definisi tipe; engine.go TIDAK mendefinisikan ulang `Status`/`StatusEvent`).
2. Stub (komentar `// STUB(foundation): diganti oleh <slug>`) dengan signature persis 2.4 di `engine.go`, `idle.go`, `ipc.go`, `hooks.go`, `install.go`, `notify.go`. `ClassifyEvent` langsung final.
3. `cmd/aitermd/wire_status.go` versi final (memanggil semua fungsi kontrak) dan stub `cmd/aitermctl/cmd_status.go` (command `status` berisi pesan "belum diimplementasi").
4. `make build` / `go build ./...` hijau.

Frontend tidak butuh Foundation (modul `status/` tidak ada sebelum fitur `status-notify-ui` membuat `index.ts`).

## 5. Wiring checklist (untuk agen wiring setelah semua fitur selesai)

1. `cmd/aitermd/wire_status.go` memuat persis urutan ini di dalam `RegisterModule("status", func(srv, d){...})`:
   - `eng := status.NewEngine(srv)`; `status.SetDefault(eng)`; `d.Provide("status.engine", eng)`
   - `status.RegisterIPC(srv, eng)`
   - `status.InstallHooks()`
   - `status.StartNotifier(srv, eng)`
   - `d.OnReady(func(){ v, ok := d.Lookup("session.manager"); m, ok2 := v.(*session.Manager); if ok && ok2 { eng.SetSessionSource(m) } else { log warning } })`
   Tidak ada edit `modules.go`; modul bernama `status` unik. Periksa nama modul tidak bentrok.
2. `cmd/aitermctl/cmd_status.go`: `RegisterCommand` dengan `Name: "status"`; `aitermctl help` menampilkannya; `aitermctl status list|get|watch|hook` bekerja.
3. Launch contribution terpasang (`InstallHooks` dipanggil tepat sekali) sehingga sesi baru: env `AITERM_SESSION_ID`/`AITERM_SOCK` di semua provider; `--settings` hanya claude. Pastikan tidak ada stub tersisa: `grep -rn "STUB(foundation)" internal/status cmd frontend/src/modules/status` harus kosong.
4. Stream engine: setelah `session.create`, `session.output.*` dan `session.exit` sampai ke engine (klien internal) dan status berubah (cek dengan `aitermctl status watch` di terminal lain dan sesi shell).
5. Frontend: `frontend/src/modules/status/index.ts` ada dan dimuat glob; panel `status-badges` (sidebar) dan `status-summary` (statusbar) terdaftar; tidak ada edit `App.svelte`, `lib/`, `modules/shell/`.
6. Epic hilir (tidak dikerjakan): tiket 09 memakai `status.IsIdle(id)` dan `status.OnIdle(id, fn)`; ID yang dipakai adalah ID sesi asli (bukan token). `AITERM_SESSION_ID` di env sesi berisi launch token; bila tiket 09 butuh ID asli, gunakan `Engine.ResolveSessionID`.
7. Tidak ada polling: `grep -rnE "time\.(NewTicker|Tick)\(|setInterval" internal/status frontend/src/modules/status` harus kosong.
8. Smoke test manual (bila memungkinkan): jalankan `aitermd`, buat sesi shell, ketik perintah (status running lalu idle setelah ~3 detik), `aitermctl status hook --session <id> --event Notification` pada sesi shell (waiting, satu notifikasi macOS), `Stop` (done), `session kill` (state dibuang, `status get` -> -32002).

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- Tiap fitur dan wiring: `go build ./...` (cepat) dan, bila menyentuh frontend, `cd frontend && npm run build`.
- Gerbang akhir: `make build` (menjalankan `npm install && npm run build`, `go build` aitermd/aitermctl, dan build app Wails). Opsional: `go vet ./internal/status/...`.
- Build harus hijau setelah tiap fitur digabung, tidak hanya di akhir.

## 7. Aturan paralel
- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, `types.go`, atau file wiring milik orang lain.
- `internal/status/types.go` dimiliki Foundation dan tidak diubah setelahnya. Bila fitur merasa butuh tipe/konstanta baru, definisikan sebagai identifier privat (huruf kecil) di file miliknya sendiri.
- Signature di 2.4 adalah kontrak: jangan diubah. Boleh menambah fungsi/metode baru yang tidak diekspor, atau yang diekspor di file milik sendiri selama tidak dipakai fitur lain.
- Dilarang menyentuh: `internal/session/`, `internal/provider/`, `internal/ipc/`, `internal/paths/`, `cmd/aitermd/modules.go`, `cmd/aitermd/main.go`, `cmd/aitermctl/registry.go`, `cmd/aitermctl/main.go`, `app/`, `frontend/src/App.svelte`, `frontend/src/lib/`, `frontend/src/modules/shell/`, `go.mod`/`go.sum`, dan file epic lain.
- Tidak ada polling (ticker, loop tidur-bangun) di mana pun; hanya event dan timer sekali jalan.
- Tanpa unit test.
