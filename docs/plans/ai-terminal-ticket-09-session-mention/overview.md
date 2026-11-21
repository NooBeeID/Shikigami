# Overview: Epic 09 Session Mention

Sumber: tiket `docs/tickets/ai-terminal-ticket-09-session-mention.md` dan spec `docs/superpowers/specs/2026-11-21-ai-terminal-design.md` (bagian 7). Tiket adalah sumber kebenaran; bila overview ini bertentangan dengan tiket, tiket menang, kecuali bagian "Keputusan" di 1.2 yang memang melengkapi celah/ketidakcocokan tiket dengan kode nyata.

## 1. Ringkasan epic dan urutan eksekusi

Epic ini memungkinkan sesi saling bertukar pesan lintas project, sesi, dan provider dalam satu workspace lewat dua jalur: (1) agent ke agent lewat server MCP loopback milik daemon (tool `list_sessions`, `send_message`, `read_output`), (2) user ke sesi lewat `@nama-sesi` di UI. Pesan masuk antrean per sesi target dan baru disuntik lewat `Provider.SendInput` saat target idle (event `status.OnIdle`, tanpa polling). Mode default `confirm_agent` mencerminkan fallback spec 9b.

Urutan eksekusi:

1. **Foundation** (satu agent, `00-foundation.md`): `internal/mention/types.go` final (tipe, error, interface, `Namer`, `Runtime`, hook `Start`). Tujuan: tiga fitur paralel tidak saling menunggu dan tidak ada definisi ganda.
2. **Fitur paralel**: `mcp-server`, `message-queue`, `mention-ui`.
3. **Wiring** (bagian 5): hampir seluruhnya verifikasi, karena semua sambungan sudah lewat registry (`RegisterModule`, `RegisterCommand`, `import.meta.glob`, `RegisterLaunchContribution`). Tidak ada file bersama yang diedit.

### 1.1 Kondisi kode saat ini (hasil membaca repo)

- Go module bernama `aiterm` (impor `aiterm/internal/...`). Daemon: `cmd/aitermd`, CLI: `cmd/aitermctl`, UI Wails: `app/`, frontend Svelte 5 di `frontend/`.
- `cmd/aitermd/modules.go`: `RegisterModule(name, func(srv *ipc.Server, d *Deps))`. `Deps{DataDir, SocketPath, Logger, Version, Started}` dengan `d.Provide(name, v)`, `d.Lookup(name) (any, bool)`, `d.OnReady(fn)`. Modul di-wire alfabetis ("mention" sebelum "provider" dan "session"), jadi dependensi WAJIB diambil di dalam `d.OnReady` (callback jalan setelah semua modul ter-wire, sebelum `srv.Serve`). Layanan yang tersedia saat ini: `"session.manager"` bertipe `*session.Manager` (disediakan `wire_session.go`). `workspace.Store` TIDAK di-`Provide`; `Deps` tidak membawa `context.Context` (di `main.go` `ctx` dibuat setelah `wireAll`).
- `cmd/aitermctl/registry.go`: `Command{Name, Summary, Run func(ctx, c *ipc.Client, args []string) error}`, `RegisterCommand`, helper `printJSON(v)` dan `callAndPrint(c, method, params)` (hasil dicetak JSON terformat; `c.Call(method, params, &out)`). Contoh pola: `cmd/aitermctl/cmd_workspace.go`.
- `internal/ipc`: `srv.Register(method, Handler)` dengan `Handler = func(ctx, json.RawMessage) (any, error)`, `srv.Publish(topic, payload)`, `ipc.Dial(socketPath) (*Client, error)`, `Client.Call`, `Client.Subscribe(prefix, fn(topic, payload json.RawMessage)) (unsubscribe, error)`. Error: `ipc.NewError(code, msg)`, `ipc.Errorf(code, fmt, ...)`, konstanta `ipc.ErrConflict (-32001)`, `ErrNotFound (-32002)`, `ErrInvalid (-32003)`, `ErrInvalidParams (-32602)`.
- `internal/session`: `Manager` dengan `List() []Session` (urut `CreatedAt`), `Get(id) (*Session, error)`, `Attach(id) (AttachResult{Session, Scrollback []byte, Offset}, error)`, `Write(id string, data []byte) error`. `Session{ID, WorkspaceID, ProjectID, Cwd, ProviderID, Status (running|exited), CreatedAt, ExitCode}`. Sentinel `session.ErrNotFound`, `ErrNotRunning`. Event IPC `session.exit` payload `{id, exitCode}`; `session.output.<id>`. Manager TIDAK menerbitkan event pembuatan sesi.
- `internal/provider`: `Get(id) (Provider, bool)`, `Default()`, `RegisterLaunchContribution(fn func(*LaunchContext))`, `LaunchContext{Session session.SessionSpec, Provider, Env, Args, MCPServers map[string]any}`. `Provider.Capabilities()` -> `{Hooks, MCP, IdleEvent}`. **`Provider.SendInput(w io.Writer, text string) error`** (bukan `provider.SendInput(sessionID, text)` seperti tulisan tiket): menulis bracketed paste + Enter (`\x1b[200~text\x1b[201~\r`) ke `w` dalam satu `Write`. `provider.WriteInput(w, text)` helper default.
- `internal/workspace`: `Workspace{ID, Name, Projects []Project{ID, Path, Name}, Config}`. IPC `workspace.list` (hasil array `Workspace`, atau objek berkunci `workspaces`; terima keduanya).
- `internal/status` (tiket 08) BELUM ada di repo saat overview ini ditulis. Kontrak dari tiket 08: `status.IsIdle(sessionID string) bool`, `status.OnIdle(sessionID string, fn func()) (cancel func())`. `OnIdle` memanggil `fn` tepat sekali pada transisi berikutnya ke idle (idle/done/waiting); bila sudah idle saat didaftarkan, `fn` dipanggil segera di goroutine terpisah; sesi tak dikenal: `IsIdle` false. IPC `status.get {sessionId}` -> `{sessionId, status, since}`. **Prasyarat: tiket 08 harus sudah digabung sebelum `queue.go` dikompilasi.** Bila `internal/status` belum ada saat implementasi, laporkan blocker; jangan membuat paket tiruan.
- Frontend: `frontend/src/lib/ipc.ts` mengekspor `inWails`, `call<T>(method, params?)`, dan **`onEvent(topicPrefix, fn(topic, payload)) => unsubscribe`** (tidak ada ekspor `subscribe` seperti tulisan tiket; pakai `onEvent`). `lib/registry.ts`: `registerPanel({id, area: 'sidebar'|'main-tab'|'statusbar', component, order})`. `main.ts` memuat `./modules/*/index.ts` lewat `import.meta.glob` eager. Pola modul: `frontend/src/modules/terminal/index.ts`. Svelte 5 (rune `$state`, `$props`, `onMount`), tanpa dependency baru.
- Store frontend (tiket 06, diekspor dari barrel `frontend/src/lib/stores/index.ts`): `activeWorkspaceId: Writable<string|null>`, `sessions: Writable<Session[]>`, `activeTabId: Writable<string|null>` (id tab bentuk `session:<sessionId>` untuk tab sesi), `tabs`. Folder `frontend/src/styles/` belum tentu ada saat ini; ia dibuat oleh tiket 06 (`shell-layout`) dengan tema gelap/terang.

### 1.2 Keputusan yang melengkapi tiket

1. **`SendInput` butuh `io.Writer`.** Pengiriman ke sesi: `p, ok := provider.Get(sess.ProviderID); if !ok { p = provider.Default() }; p.SendInput(sessionWriter{mgr, sessionID}, text)`, dengan `sessionWriter.Write(b)` memanggil `Manager.Write(id, b)` dan mengembalikan `len(b)`. `sessionWriter` didefinisikan di `queue.go` (fitur `message-queue`).
2. **`SessionSpec` tidak punya ID saat `LaunchContribution` dipanggil.** `Manager.Create` memanggil `launcher.Command(spec)` SEBELUM `newID()`. Karena itu token tidak bisa langsung dipetakan ke `sessionID` saat peluncuran. Solusi (fitur `mcp-server`, `install.go`): kontribusi membuat **token tertunda** `pendingToken{token, workspaceID, projectID, providerID, cwd, issuedAt}` (urut FIFO) dan memasangnya di `MCPServers`. Pada request MCP pertama dengan token tertunda, server mengikat token ke sesi: ambil `Sessions.List()`, filter sesi `running` dengan `WorkspaceID/ProjectID/ProviderID/Cwd` sama yang belum terikat token mana pun, urut `CreatedAt`; token tertunda yang cocok kriteria diikat FIFO ke sesi tak-terikat tertua (token ke-n ke sesi ke-n). Token terikat disimpan `token -> sessionID` dan dilepas saat sesi exit (lewat `Runtime.OnSessionExit`) atau saat `Sessions.Get` menunjukkan sesi tak ada/`exited` (pengecekan malas pada request berikutnya). Token tertunda dibatasi 64 entri (buang yang tertua) dan dipangkas saat dipakai. Keterbatasan ini (tidak ada ID sesi saat launch) dilaporkan sebagai catatan untuk tiket 02/08, tanpa mengedit paket mereka.
3. **`Enqueuer` dan semua tipe bersama dipindahkan ke `types.go` (Foundation)**, bukan `tools.go`/dibuat "oleh yang lebih dulu". Ini menghindari definisi ganda. `tools.go` memakai `Enqueuer` dari `types.go` dan tidak mendefinisikannya ulang. Hook `Start` juga di `types.go` agar `wire_mention.go` bisa memanggil `mention.Start` sebelum `mcp.go` ada (lihat 2.4).
4. **`Enqueue` menerima `to` berupa sessionID yang sudah di-resolve.** Resolusi nama -> id dilakukan pemanggil lewat `Namer.Resolve(workspaceID, to)` (MCP memakai workspace sesi pemanggil; IPC memakai `workspaceId` param). `Enqueue` hanya memvalidasi bahwa sesi target ada, `running`, dan (untuk `from != ""`) berada di workspace yang sama dengan pengirim.
5. **Isi antrean = hanya pesan pending.** Event `mention.queue` dan hasil `mention.queue.list` memuat pesan berstate `queued`/`awaiting_confirm` saja; pesan `delivered`/`failed` keluar dari antrean (hasilnya terlihat pada `Message` yang dikembalikan `mention.send`/`Enqueue`). Event `mention.queue` payload `{sessionId, queue}` selalu berisi seluruh antrean pending sesi itu (UI mengganti entri sesi tersebut; `queue: []` berarti kosong).
6. **Jeda setelah kirim (bukan polling).** Setelah satu pesan terkirim, status sesi mungkin masih `idle` sesaat sebelum hook `UserPromptSubmit` tiba, sehingga `OnIdle` bisa langsung terpicu lagi dan menyuntik berturut-turut. Maka Queue menyimpan `cooldownUntil = now + 1500ms` per target; saat drain terpicu sebelum cooldown selesai, daftarkan SATU `time.AfterFunc(sisa, drain)` (timer sekali jalan, bukan `Ticker`) lalu evaluasi ulang `status.IsIdle`. Tidak ada loop periodik.
7. **Sesi `exited` tidak ikut daftar nama.** `Namer.List` dan `list_sessions` hanya memuat sesi `running`.
8. **Provider tanpa MCP.** Kontribusi MCP tetap mengisi `MCPServers` hanya bila `ctx.Provider.Capabilities().MCP == true`; selain itu tidak membuat token (sesi tetap menerima mention dari user lewat UI).
9. **Direktori project via IPC internal.** Karena `workspace.Store` tidak tersedia di `Deps`, nama project diperoleh lewat `ipc.Dial(d.SocketPath)` (malas, saat pertama dibutuhkan) memanggil `workspace.list`. Implementasi `Directory` ini ada di `wire_mention.go`. Bila gagal, `ProjectName` mengembalikan `""` dan `Namer` memakai fallback `projectId`-turunan (lihat Foundation).
10. **Status sesi untuk `names`/`list_sessions`.** Diambil lewat `StatusFunc`: `wire_mention.go` memakai `status.get` via klien IPC internal; bila gagal, turunkan dari `status.IsIdle` (`"idle"` atau `"running"`). Bila paket `internal/status` ternyata punya getter Go publik, boleh dipakai sebagai gantinya.

## 2. Kontrak bersama

### 2.1 Struktur folder

```
internal/mention/types.go            (Foundation)   tipe, error, interface, Namer, Runtime, Start hook
internal/mention/mcp.go              (mcp-server)   HTTP server JSON-RPC, init() mengisi startMCP
internal/mention/tools.go            (mcp-server)   tiga tool + resolve pemanggil dari token
internal/mention/install.go          (mcp-server)   token tertunda/terikat + LaunchContribution
internal/mention/queue.go            (message-queue) Queue, sessionWriter, mode, drain
internal/mention/ipc.go              (message-queue) RegisterIPC: method mention.*
cmd/aitermd/wire_mention.go          (message-queue) RegisterModule("mention", ...)
cmd/aitermctl/cmd_mention.go         (message-queue) perintah `aitermctl mention ...`
frontend/src/modules/mention/        (mention-ui)   index.ts, MentionComposer.svelte, QueueBadge.svelte, + helper lokal
```

### 2.2 Tipe Go (`internal/mention/types.go`, final dari Foundation)

```go
package mention

type State string
const (
    StateQueued          State = "queued"
    StateAwaitingConfirm State = "awaiting_confirm"
    StateDelivered       State = "delivered"
    StateFailed          State = "failed"
)

const (
    SourceAgent = "agent"
    SourceUser  = "user"
)
const FromUser = "user" // nilai Message.From untuk mention user

type Mode string
const (
    ModeAuto         Mode = "auto"
    ModeConfirmAgent Mode = "confirm_agent" // default
    ModeUserOnly     Mode = "user_only"
)

type Message struct {
    ID        string    `json:"id"`        // ULID
    From      string    `json:"from"`      // "user" atau sessionID pengirim
    FromName  string    `json:"fromName"`  // nama mention pengirim, "user" untuk user
    To        string    `json:"to"`        // sessionID target
    Text      string    `json:"text"`
    CreatedAt time.Time `json:"createdAt"` // RFC3339
    State     State     `json:"state"`
    Source    string    `json:"source"`    // "agent" | "user"
    Error     string    `json:"error,omitempty"` // terisi bila state failed
}

type NameInfo struct {
    SessionID   string `json:"sessionId"`
    Name        string `json:"name"`
    ProjectName string `json:"projectName"`
    ProviderID  string `json:"providerId"`
    Status      string `json:"status"` // running|waiting|done|idle
}
```

Error sentinel: `ErrNotFound` ("sesi tidak ditemukan"), `ErrInvalid` ("masukan tidak valid"), `ErrConflict` ("antrean penuh"), `ErrCrossWorkspace` ("bukan workspace yang sama"), `ErrUserOnly` ("mode user_only: pesan dari agent ditolak"). Pemetaan ke IPC (dipakai `ipc.go`): `ErrNotFound -> ipc.ErrNotFound`, `ErrInvalid` dan `ErrCrossWorkspace` dan `ErrUserOnly -> ipc.ErrInvalid`, `ErrConflict -> ipc.ErrConflict`. Pemetaan ke tool MCP: `isError: true` dengan teks pesan error (`err.Error()`).

### 2.3 Interface

```go
// SessionSource dipenuhi langsung oleh *session.Manager.
type SessionSource interface {
    List() []session.Session
    Get(id string) (*session.Session, error)
    Attach(id string) (session.AttachResult, error)
    Write(id string, data []byte) error
}

// Directory memberi nama project; diimplementasikan di wire_mention.go.
type Directory interface {
    ProjectName(workspaceID, projectID string) string
}

// StatusFunc mengembalikan status sesi (running|waiting|done|idle).
type StatusFunc func(sessionID string) string

// Enqueuer dipenuhi oleh *Queue (message-queue), dipakai tools.go (mcp-server).
// from == "" berarti user (source harus SourceUser); selain itu from = sessionID pengirim (source SourceAgent).
// to = sessionID target yang sudah di-resolve.
type Enqueuer interface {
    Enqueue(ctx context.Context, from, to, text, source string) (Message, error)
}
```

`Enqueue` mengembalikan `Message` dengan `State`: `delivered` (target idle dan mode mengizinkan, terkirim), `queued`, atau `awaiting_confirm`. Posisi antrean untuk hasil tool `send_message` dihitung `tools.go` lewat interface kedua di `types.go`:

```go
// QueueReader dipenuhi *Queue; dipakai tools.go untuk menghitung position.
type QueueReader interface {
    Position(messageID string) int // 1-based di antrean pending target; 0 bila bukan pending
}
```

`Runtime` (dibuat `wire_mention.go`, dikonsumsi `mcp.go`/`install.go`):

```go
type Runtime struct {
    DataDir       string
    Logger        *slog.Logger
    Sessions      SessionSource
    Namer         *Namer
    Enqueuer      Enqueuer
    Reader        QueueReader
    OnSessionExit func(fn func(sessionID string)) // pendaftaran callback exit; boleh dipanggil beberapa kali
}
```

`Namer` (di `types.go`):

```go
func NewNamer(src SessionSource, dir Directory, statusOf StatusFunc) *Namer
func (n *Namer) NameOf(sessionID string) string                       // "" bila sesi tak ada
func (n *Namer) ResolveName(workspaceID, name string) (string, error) // nama -> sessionID, ErrNotFound bila tak ada
func (n *Namer) Resolve(workspaceID, to string) (string, error)       // `to` berupa id sesi ATAU nama; ErrNotFound / ErrCrossWorkspace
func (n *Namer) List(workspaceID string) []NameInfo                   // sesi running di workspace, urut CreatedAt
func NewMessageID() string                                            // ULID
```

Hook start (di `types.go`):

```go
var startMCP func(ctx context.Context, rt *Runtime) (url string, err error) // diisi init() di mcp.go
func Start(ctx context.Context, rt *Runtime) (string, error)                // memanggil startMCP bila ada; bila nil: ("", nil)
func URL() string                                                           // alamat MCP yang aktif ("" bila belum jalan); thread-safe
func setURL(u string)                                                       // dipakai Start
```

### 2.4 Siapa memanggil siapa (agar build lolos pada setiap tahap)

- `wire_mention.go` memanggil `mention.Start(ctx, rt)`. Selama `mcp.go` belum ada, `Start` mengembalikan `("", nil)`; setelah `mcp.go` ada, `init()` di `mcp.go` mengisi `startMCP = startServer` sehingga tidak ada edit di `wire_mention.go`. `startServer` mem-bind server, `setURL`, menulis `<DataDir>/mention.json`, dan mendaftarkan `LaunchContribution` (sekali) yang memakai `rt`.
- `mcp-server` bergantung pada `types.go` (Foundation) dan paket `session`/`provider`, bukan pada `queue.go`.
- `message-queue` bergantung pada `types.go`, `provider`, `session`, `status`, bukan pada `mcp.go`. `*Queue` harus memenuhi `Enqueuer` dan `QueueReader` (tambahkan `var _ Enqueuer = (*Queue)(nil)` dan `var _ QueueReader = (*Queue)(nil)`).
- `mention-ui` hanya bergantung pada nama method/event IPC (2.5).

### 2.5 Pesan IPC (dari tiket, ditegaskan)

| Method | Params | Hasil |
|---|---|---|
| `mention.send` | `{workspaceId, to, text, from?}` | `Message`. `from` kosong = user. `to` = nama mention atau id sesi. `ERR_NOT_FOUND` sesi tak ada; `ERR_INVALID` teks kosong/lintas workspace/`user_only` untuk agent; `ERR_CONFLICT` antrean target penuh (50). |
| `mention.queue.list` | `{sessionId?}` | `Message[]` pending (kosong = semua sesi). |
| `mention.queue.cancel` | `{id}` | `{ok:true}`; hanya state `queued`/`awaiting_confirm`, selain itu `ERR_INVALID`; id tak ada `ERR_NOT_FOUND`. |
| `mention.confirm` | `{id}` | `Message` (jadi `queued`, picu kirim bila idle); bukan `awaiting_confirm` -> `ERR_INVALID`. |
| `mention.names` | `{workspaceId}` | `NameInfo[]` (sesi running). |
| `mention.config` | `{mode?}` | `{mode}`; mode tak dikenal `ERR_INVALID`. |
| `mention.info` | tanpa | `{url, sessions: n}` (`n` = jumlah token terikat/sesi ber-MCP; `url` dari `mention.URL()`). |

Event server: topik `mention.queue`, payload `{sessionId, queue: Message[]}` (lihat 1.2 butir 5). Nama field JSON memakai camelCase seperti di atas.

### 2.6 Server MCP

- Bind `127.0.0.1:0` (hanya loopback), path `/mcp`. URL: `http://127.0.0.1:PORT/mcp`, ditulis ke `<DataDir>/mention.json` sebagai `{"url":"..."}`.
- Auth: header `Authorization: Bearer <token>`; token tak dikenal atau tak ada -> HTTP 401. Token = 32 byte acak (`crypto/rand`, hex).
- JSON-RPC 2.0 lewat POST: `initialize` (balas `protocolVersion` "2025-03-26", `capabilities: {tools: {}}`, `serverInfo {name:"aiterm-mention", version:"1"}`), `notifications/initialized` (HTTP 202 tanpa body), `tools/list` (tepat tiga tool dengan `inputSchema`), `tools/call`. Method lain: error JSON-RPC -32601. GET pada `/mcp` -> 405.
- Hasil `tools/call`: `{"content":[{"type":"text","text":"<JSON atau teks>"}], "isError": false}`. Untuk `list_sessions` dan `send_message`, `text` berisi JSON dari struct hasil; `read_output` berisi teks polos.
- Konfigurasi sesi: `MCPServers["aiterm-mention"] = map[string]any{"type":"http","url":<url>,"headers":map[string]any{"Authorization":"Bearer <token>"}}`.
- Tool:
  - `list_sessions()` -> `[{id, name, projectName, providerId, status, self}]` untuk sesi workspace pemanggil.
  - `send_message(to string, text string)` -> `{queued, delivered, position}`. `to` nama atau id. `text` wajib tidak kosong.
  - `read_output(to string, last_n int)` -> `last_n` baris terakhir (default 50, maks 500), escape ANSI (CSI, OSC, dan karakter kontrol selain `\n`/`\t`) dibuang, `\r\n` dinormalkan. Sumber: `Sessions.Attach(id).Scrollback`.

### 2.7 Penamaan sesi

`<providerId>-<projectName>` -> huruf kecil, karakter di luar `[a-z0-9]` menjadi `-`, `-` berurutan dilipat, `-` di tepi dibuang. Project kosong/tak diketahui memakai segmen `noproject`. Bentrok dalam workspace (hanya sesi running): sesi tertua (`CreatedAt`, lalu `ID`) tetap bernama dasar, berikutnya `-2`, `-3`, dst. Pola nama valid: `[a-z0-9-]+` (dipakai UI untuk parse `@nama`).

### 2.8 Konvensi

- Komentar kode dan pesan error/user-facing berbahasa Indonesia; identifier Inggris.
- Frontend: nama file komponen PascalCase, helper huruf kecil; impor relatif (`../../lib/ipc`, `../../lib/registry`, `../../lib/stores`). Tanpa `setInterval`, tanpa dependency baru. Styling hanya CSS variables dengan nilai cadangan, mis. `var(--bg-elevated, #1e1e1e)`, karena nama variabel final milik tiket 06 (`frontend/src/styles/`); lihat file itu bila sudah ada dan pakai nama yang ada.
- Dilarang di seluruh epic: `time.Ticker`, `time.Tick`, loop tidur-bangun, `setInterval`. Timer sekali jalan (`time.AfterFunc`) hanya untuk cooldown (1.2 butir 6) dan retry langganan exit yang dibatasi 5 kali.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `mcp-server` | Server MCP loopback (net/http), JSON-RPC, tiga tool, token tertunda/terikat, `LaunchContribution`, `read_output`, tulis `mention.json`, isi `startMCP` lewat `init()`. Mengonsumsi `Runtime`, `Namer`, `Enqueuer`, `QueueReader`. Tidak berisi logika antrean. | `internal/mention/mcp.go`, `internal/mention/tools.go`, `internal/mention/install.go` |
| `message-queue` | `Queue` (FIFO per target, batas 50, mode, cooldown, `OnIdle` tanpa dobel, `sessionWriter`, cleanup saat exit), `RegisterIPC` method `mention.*` + event `mention.queue`, modul daemon `wire_mention.go` (Directory, StatusFunc, OnSessionExit, `Runtime`, `mention.Start`), CLI `aitermctl mention`. | `internal/mention/queue.go`, `internal/mention/ipc.go`, `cmd/aitermd/wire_mention.go`, `cmd/aitermctl/cmd_mention.go` |
| `mention-ui` | Modul frontend: `MentionComposer.svelte` (sidebar, autocomplete `@`, opsi sertakan output terakhir sesi aktif), `QueueBadge.svelte` + daftar antrean (statusbar, event `mention.queue`, batal/konfirmasi), `index.ts` registrasi panel. | `frontend/src/modules/mention/` |

Catatan terhadap tiket: `internal/mention/types.go` tercantum sebagai milik `message-queue` di tiket, tetapi dibuat dan difinalkan oleh Foundation (1.2 butir 3). Agen `message-queue` TIDAK mengedit `types.go`; bila butuh tipe lokal tambahan, definisikan di `queue.go`/`ipc.go`. Tidak ada fitur yang ditambah atau digabung.

## 4. Foundation

`hasFoundation=true`. Rincian langkah di `00-foundation.md`. Isinya hanya `internal/mention/types.go` (bagian 2.2 sampai 2.4 secara utuh, termasuk implementasi `Namer`, `NewMessageID`, `Start`, `URL`). Tidak ada skeleton frontend. Guard Foundation: `go build ./...` lolos (membutuhkan paket `session` dan `provider`, sudah ada; `types.go` tidak mengimpor `status`).

## 5. Wiring checklist

Semua sambungan sudah terjadi lewat registry di dalam file milik fitur; agen Wiring terutama memverifikasi, dan hanya menyentuh file bersama bila ada celah nyata.

1. Daemon: `cmd/aitermd/wire_mention.go` memanggil `RegisterModule("mention", ...)` dari `init()`; di dalam callback, semua kerja dilakukan di `d.OnReady` (lookup `session.manager` bertipe `*session.Manager`; bila tak ada, `d.Logger.Warn` lalu return tanpa panic). Verifikasi: `grep RegisterModule cmd/aitermd/wire_mention.go`, nama modul `"mention"` tidak bentrok.
2. `d.OnReady` membangun `Directory`, `StatusFunc`, `Namer`, `Queue`, memanggil `mention.RegisterIPC(srv, queue, namer, ...)`, menyusun `mention.Runtime`, memanggil `mention.Start(context.Background(), rt)`. Verifikasi: `mcp.go` mengisi `startMCP` di `init()` (bila `mcp.go` ada, `Start` mengembalikan URL non-kosong; bila belum, no-op).
3. `LaunchContribution`: didaftarkan `startServer` lewat `provider.RegisterLaunchContribution` tepat sekali (guard `sync.Once`). Verifikasi: sesi baru dari provider ber-`Capabilities().MCP` mendapat `MCPServers["aiterm-mention"]`.
4. CLI: `cmd/aitermctl/cmd_mention.go` memanggil `RegisterCommand(Command{Name:"mention", ...})` dari `init()`. Verifikasi: `aitermctl` tanpa argumen mencantumkan `mention`.
5. Frontend: `frontend/src/modules/mention/index.ts` dimuat `import.meta.glob('./modules/*/index.ts')`; memanggil `registerPanel` dua kali (`mention-composer` sidebar order 30, `mention-queue` statusbar order 20). Verifikasi: kedua id terdaftar, tidak ada edit di `App.svelte`/`lib/*`.
6. Kecocokan kontrak: nama method/event di `frontend/src/modules/mention/` dan `cmd_mention.go` sama persis dengan `ipc.go` (2.5): `mention.send|queue.list|queue.cancel|confirm|names|config|info`, event `mention.queue`.
7. Tidak ada `time.Ticker`/`time.Tick`/`setInterval` (`grep -rn "time.Tick\|NewTicker\|setInterval" internal/mention cmd/aitermd/wire_mention.go cmd/aitermctl/cmd_mention.go frontend/src/modules/mention`).
8. Tidak ada perubahan `go.mod`, `go.sum`, `frontend/package.json`, `internal/provider|status|session|workspace|ipc`.
9. Verifikasi manual (opsional, bukan guard): dua sesi shell satu workspace, `aitermctl mention names <workspaceId>`, `aitermctl mention send <nama> "halo"`; `aitermctl mention info` menampilkan URL; `curl` ke URL dengan token sesi mengembalikan `tools/list`.

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- `go build ./...` (mencakup `internal/mention/`, `cmd/aitermd/wire_mention.go`, `cmd/aitermctl/cmd_mention.go`).
- `cd frontend && npm run build` (vite build; jalankan juga `npm run check` bila skrip tersedia di `package.json`).
- Gabungan: `make build` (berjalan `npm install` + build frontend, `go build` daemon dan CLI, build app Wails).

Tiap agen fitur menjalankan guard yang relevan dengan path-nya (Go: `go build ./...`; frontend: `npm run build` di `frontend/`) sebelum melapor selesai; build harus lolos walau fitur lain belum ada.

## 7. Aturan paralel

- Tiap agen HANYA menyentuh `ownedPaths` fiturnya (bagian 3). `internal/mention/types.go` milik Foundation dan terkunci setelah Foundation selesai.
- Dilarang menyentuh: path fitur lain, `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go`, `cmd/aitermd/main.go`, `cmd/aitermctl/main.go`, `internal/provider|status|session|workspace|ipc|paths`, `frontend/src/App.svelte`, `frontend/src/lib/*`, `frontend/src/modules/<modul lain>/`, `go.mod`, `go.sum`, `frontend/package.json`.
- Bila butuh perubahan pada file terlarang atau pada `types.go`, jangan edit: cari solusi lokal di file milik sendiri dan catat di laporan sebagai blocker/catatan.
- Antarmuka antar fitur hanya lewat 2.2 sampai 2.6. Jangan menebak API fitur lain; baca bagian ini.
- Tanpa unit test; guard hanya build (bagian 6).
- Detail per-fitur yang harus dipatuhi:
  - `mcp-server`: pakai stdlib saja; tidak ada goroutine selain yang dibuat `http.Server`; bind hanya `127.0.0.1`; `send_message` hanya validasi lalu `Resolve` + `Enqueue` (position dari `Reader.Position`); error tool lewat `isError:true`; lepas token saat exit dan secara malas.
  - `message-queue`: `Queue` aman konkuren (satu mutex), satu pendaftaran `OnIdle` per target (flag `armed` + simpan `cancel`), batal `OnIdle` saat antrean kosong/sesi exit; `SendInput` gagal -> `state: failed`, lanjut pesan lain; `Enqueue` memeriksa mode (`ErrUserOnly` untuk agent di `user_only`; `awaiting_confirm` untuk agent di `confirm_agent` dan TIDAK mendaftarkan `OnIdle` sampai dikonfirmasi); langganan `session.exit` di `wire_mention.go` lewat klien IPC internal dengan retry terbatas 5 kali (`time.AfterFunc`, jeda 200ms) karena server belum `Serve` saat `OnReady`, dan `OnSessionExit` meneruskan ke semua callback terdaftar; pesan tertunda sesi exit menjadi `failed` dan antrean dibersihkan (event `mention.queue` dengan `queue: []`).
  - `mention-ui`: tangani `inWails == false` dan error IPC sebagai pesan singkat; muat awal `mention.queue.list`, selanjutnya hanya event `onEvent('mention.queue', ...)`; daftar nama dimuat ulang saat `activeWorkspaceId` berubah dan saat `onEvent('session.exit', ...)` terbit, serta saat daftar `sessions` store berubah (subscribe store, bukan timer); unsubscribe di `onDestroy`/cleanup efek.
