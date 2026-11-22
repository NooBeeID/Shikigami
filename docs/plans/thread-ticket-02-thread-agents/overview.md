# Overview: thread-ticket-02-thread-agents

Tiket: `docs/tickets/thread-ticket-02-thread-agents.md`. Spec: `docs/superpowers/specs/2026-11-22-thread-design.md`. Dependensi: `thread-ticket-01-thread-core` (plan: `docs/plans/thread-ticket-01-thread-core/overview.md`, disebut "overview-01").
Dokumen ini adalah sumber kebenaran BERSAMA untuk semua plan fitur epic ini. Bila ada perbedaan dengan tiket, overview ini menang hanya pada butir yang diberi label "koreksi" (bagian 2.2); selain itu tiket menang. Tanpa dependency baru, tanpa unit test; guard hanya build.

Keadaan repo saat overview ditulis: tiket 01 baru berupa PLAN (belum ada `internal/thread/`, `cmd/shikigamid/wire_thread.go`, `frontend/src/modules/thread/`). Seluruh nama tiket 01 yang dipakai di sini (`Manager`, `Post`, `AddParticipant`, `UpdateThread`, `SendToSession`, hook, `ToolRegistry`, `RegisterMethod`, slot `registerThreadSlot`, dst.) diambil dari overview-01. Agent yang mengerjakan epic ini WAJIB membaca kode nyata tiket 01 lebih dulu; bila nama di kode berbeda, ikuti kode, sesuaikan, dan catat di bagian "Catatan untuk wiring" plan fitur masing-masing. Foundation (langkah 0) memverifikasi kode tiket 01 benar-benar ada.

## 1. Ringkasan epic dan urutan eksekusi

Master (tiket 01) kini dapat melibatkan project lain. Epic ini menambah: (a) tool MCP `shikigami-thread` (server loopback sendiri, token per sesi); (b) `thread_spawn` yang membuat git worktree + branch milik thread lalu men-spawn agent peer auto-approve di worktree itu; (c) router pesan antar peserta lewat antrean mention khusus thread (kirim saat idle, tanpa konfirmasi) dengan pencatatan ke log thread; (d) pembatas anti-loop; (e) UI panel peserta dan penyaringan sesi thread dari sidebar kiri.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): verifikasi kode tiket 01; berkas Go bersama (`agents_types.go`, `worktree_records.go`) dan stub berkompilasi untuk `worktree.go` serta `mcp_tools.go`; titik ekstensi filter sesi di `modules/sidebar-left`. Setelah selesai `go build ./...` dan build frontend lulus.
2. **Fitur paralel** (4 fitur, bagian 3): `thread-worktree`, `thread-mcp-server`, `thread-spawn-router`, `thread-participants-ui`. Signature bagian 2 TIDAK berubah.
3. **Wiring** (bagian 5): satu berkas Go baru `internal/thread/agents_install.go` merangkai semua fitur (`InstallAgents`), pemanggilan di `wire_thread.go`, prompt master, dan dua baris frontend.

## 2. Kontrak bersama

### 2.1 Paket, folder, penamaan

- Modul Go `github.com/NooBeeID/shikigami`; paket `internal/thread` (package `thread`). Berkas epic ini: `agents_types.go`, `worktree_records.go` (Foundation, tetap); `worktree.go`, `mcp.go`, `mcp_tools.go`, `launch.go`, `spawn.go`, `router.go`, `antiloop.go` (fitur); `agents_install.go` (Wiring).
- Frontend: `frontend/src/modules/thread/participants/` (Svelte 5 runes, TypeScript). Tidak ada `index.ts` registrasi baru; satu pintu: `participants/index.ts` mengekspor `installParticipants()` yang dipanggil dari `thread/index.ts` (Wiring).
- Komentar, log, pesan galat, teks UI, teks tool dan prompt: bahasa Indonesia; identifier bahasa Inggris. Logger `*slog.Logger` nullable (fallback `slog.Default()`). JSON camelCase.
- Helper tak-diekspor diberi awalan berkas: `worktree*`, `mcp*`, `launch*`, `spawn*`, `router*`, `antiloop*` (mis. `spawnResolveProject`, `routerResolveTargets`), agar tidak bentrok antar agent paralel. Tipe/konstanta bersama didefinisikan HANYA di berkas pemiliknya (bagian 2.3).
- Konvensi path (final): worktree `<DataDir>/threads/<threadId>/<projectSlug>`; branch `thread/<threadId>-<slug>` (sufiks `-2`, `-3` bila bentrok); berkas samping `<DataDir>/threads/<threadId>/worktrees.json`; URL MCP ditulis ke `<DataDir>/thread-mcp.json` (`{"url": "..."}`, atomik, seperti `mention.json`). `DataDir` = akar data (bukan `.../threads`); `store.ThreadDir(id)` = `<DataDir>/threads/<id>`. Slug `master` DICADANGKAN (direktori master tiket 01); `Slugify` menghasilkan `master-project` bila hasilnya persis `master`, dan slug tak boleh kosong (fallback `project`).

### 2.2 Koreksi atas tiket dan keputusan desain (WAJIB diikuti)

1. **Tidak ada metadata `threadId` pada `session.Session`.** Struct `Session`/`SessionSpec` (`internal/session/types.go`) tidak punya field itu, dan tiket melarang mengubah di luar ownedPaths. Pengganti, dua lapis:
   - Daemon: sesi thread dikenali dari **cwd**. Master (tiket 01) ber-cwd `<DataDir>/threads/<id>/master`, peer ber-cwd worktree `<DataDir>/threads/<id>/<slug>`; keduanya di bawah `<DataDir>/threads/<threadId>/`. `launch.go` menurunkan `threadId` dari `SessionSpec.Cwd` (`ThreadIDFromCwd`, 2.3). Tidak perlu mengubah `master.go` (tiket 01), `SessionSpec.Env`, atau provider. Role (master/peer) TIDAK diturunkan dari cwd (slug bisa bentrok) melainkan dari `Participant.Role` di registri Manager saat token di-resolve.
   - UI: `session.list` tidak membawa `threadId`. Penyaringan sidebar memakai (a) set `sessionId` dari `thread.ownedSessions` + `thread.updated` (peserta semua thread aktif) DAN (b) pola cwd `/threads/<ULID 26 huruf>/` (menutup balapan sesi tampil sebelum `thread.updated` tiba). Predikat `isThreadSession(session)` di `participants/session-filter.ts`.
2. **Codex tidak mendapat MCP.** `internal/provider/codex.go` memiliki `Capabilities{MCP:false}` dan `Command` mengabaikan `MCPServers`; hanya Claude yang memakai `--mcp-config`. Karena provider tidak boleh diubah:
   - `launch.go` memasang `MCPServers["shikigami-thread"]` hanya bila `ctx.Provider.Capabilities().MCP` true (pola yang sama dengan mention); shell dan codex otomatis dilewati.
   - `spawn.go` memilih provider peer lewat `spawnResolveProvider()`: urutan `provider.IDClaude` lalu `provider.IDCodex`, yang pertama `Available()` tanpa galat DAN `Capabilities().MCP` true. Tidak ada kandidat -> galat jelas ke master ("tidak ada provider agent dengan dukungan MCP (claude) yang tersedia untuk peserta thread"). Dalam praktik = Claude. Catat keterbatasan ini di wiring; acceptance "Claude/Codex" terpenuhi sejauh provider mendukung MCP.
   - Master dari tiket 01 (`resolveMasterProvider`) mungkin jatuh ke codex; master tanpa MCP tidak bisa memanggil tool thread. Wiring mencatat peringatan, tidak mengubah `master.go`.
3. **Antrean mention thread sudah `ModeAuto`; mention TIDAK diubah.** overview-01 2.2 no. 4: `wire_thread.go` membuat instance `mention.Queue` khusus thread dengan `SetMode(mention.ModeAuto)`. Maka "tanpa konfirmasi" terpenuhi tanpa menyentuh `internal/mention` dan mode global tidak berpengaruh. Butir wiring tiket no. 3 ("tambahan aditif di antrean mention") DIBATALKAN. Konsekuensi: teks yang tiba di agent selalu berawalan `[pesan dari <nama mention pengirim>] ` (perilaku `Queue.deliver`); router menambahkan penanda `[thread] ` di depan teks (jadi: `[pesan dari claude-x] [thread] ...`) agar agent tahu harus membalas lewat `thread_send`, bukan mengetik di terminal.
4. **Pengiriman lewat `Manager.SendToSession(threadID, from, to, text)`** (overview-01 2.3): `from` = sessionId pengirim (SourceAgent) atau `""` (user). Antrean mensyaratkan pengirim dan target sesi running dan satu workspace; kegagalan dikembalikan sebagai galat (`mention.ErrNotFound`, dst.) dan diubah router menjadi galat jelas ke pemanggil plus pesan sistem di log.
5. **Dua registri tool disatukan di sisi server.** Tiket 01 punya `Manager.Tools()` (`ToolRegistry`/`ToolSpec`, handler `args map[string]any`); tiket ini punya `RegisterTool(ToolDef)` global di `mcp_tools.go` (handler `json.RawMessage`). Aturan: server MCP menyajikan UNION: tool dari registri global ditambah tool dari `mgr.Tools().Tools()` yang diadaptasi (marshal `RawMessage` -> `map[string]any`, `Caller` -> `ThreadCaller` dengan `Role` "master"/"peer"). Nama ganda: registri global menang, adaptasi diabaikan dengan log. Tiket 04 memakai `RegisterTool` (kontrak tiket).
6. **Anti-loop (aturan final).** Hitungan per thread dihitung dari pesan log `kind=message` lewat hook `Manager.OnMessage`, TIDAK di `Route` (agar tidak dobel):
   - pengirim `user` atau peserta role `master` -> hitungan di-reset ke 0 dan status jeda dicabut (pesan sistem "Thread dilanjutkan." dicatat bila sebelumnya dijeda);
   - pengirim peserta role `peer` dan `to != user` -> hitungan +1; saat hitungan mencapai `MaxAgentChain` (8) thread dijeda (state internal `AntiLoop`, bukan `Thread.state`) dan dicatat pesan sistem persis: "Thread dijeda: pesan beruntun antar agent mencapai batas. Kirim pesan untuk melanjutkan." (diposting di goroutine terpisah agar tidak masuk rekursi hook);
   - saat dijeda, `Route` dari peer ditolak SEBELUM dicatat/dikirim dengan `ErrPaused`; pesan master dan user selalu lolos (dan mereset);
   - kind selain `message` diabaikan; ketikan langsung di terminal tidak lewat router sehingga tidak dihitung; state hilang saat daemon restart (diterima, pemulihan = tiket 05); `OnClose` menghapus state thread.
7. **Reuse peserta.** `thread_spawn` untuk project yang sudah peserta peer thread ini: bila sesinya masih `running` -> kirim `brief` sebagai pesan baru dari master (tanpa worktree/sesi baru); bila sesinya sudah mati -> galat jelas ke master ("agen project X sudah berhenti"; penghidupan ulang = tiket 05), TIDAK membuat worktree baru.
8. **Dedup nama peserta.** `Participant.Name` peer diisi `Namer.NameOf(sessionId)` saat didaftarkan, fallback `<providerId>-<slug>` bila kosong. Target pesan (`to`) di router di-resolve DI DALAM peserta thread saja, tanpa mengikutsertakan sesi lain di workspace: (huruf besar/kecil tak dibedakan, awalan `@` dibuang) cocok dengan: `Participant.Name`, `sessionId`, nama project atau id project peserta, kata `master`. Kata `user` dan `all` khusus. Ambigu -> galat yang mendaftar kandidat.

### 2.3 Signature Go bersama (final)

**`agents_types.go`** (Foundation; tidak diubah fitur):

```go
// Router dipenuhi oleh router.go (thread-spawn-router); dipakai handler tool (thread-mcp-server).
type Router interface {
    // Route mencatat pesan ke log thread lalu mengirimnya. from = sessionId pengirim ("user" untuk user).
    // to = nama/sessionId peserta, "master", "user" (hanya dicatat) atau "all" (semua peserta selain pengirim).
    Route(ctx context.Context, threadID, from, to, text string) (ThreadMessage, error)
}

var (
    ErrPaused        = errors.New("thread dijeda: pesan beruntun antar agent mencapai batas; tunggu pesan dari user atau master")
    ErrTargetUnknown = errors.New("target bukan peserta thread ini")
    ErrTargetAmbiguous = errors.New("target ambigu")
    ErrNotParticipant = errors.New("pemanggil bukan peserta thread ini")
)

// ParticipantView adalah bentuk keluaran tool thread_participants dan dipakai ulang tiket 03/05.
type ParticipantView struct {
    Name      string `json:"name"`
    SessionID string `json:"sessionId"`
    Role      string `json:"role"`
    Project   string `json:"project"` // nama project; kosong untuk master
    Status    string `json:"status"`  // "kerja" | "idle"
    Self      bool   `json:"self"`
}
```

**`worktree_records.go`** (Foundation; tiket 04/05 membacanya):

```go
type WorktreeRecord struct {
    ProjectID   string       `json:"projectId"`
    ProjectName string       `json:"projectName"`
    ProjectSlug string       `json:"projectSlug"`
    SessionID   string       `json:"sessionId"` // "" sampai sesi dibuat
    Info        WorktreeInfo `json:"info"`
    CreatedAt   time.Time    `json:"createdAt"`
}
func LoadWorktreeRecords(threadDir string) ([]WorktreeRecord, error) // berkas belum ada -> (nil, nil)
func UpsertWorktreeRecord(threadDir string, rec WorktreeRecord) error // kunci ProjectID; atomik (.tmp + rename); mutex paket; izin 0600
func DeleteWorktreeRecord(threadDir, projectID string) error          // idempoten
```

**`worktree.go`** (stub Foundation, implementasi penuh `thread-worktree`; signature final):

```go
type WorktreeInfo struct {
    Path        string `json:"path"`
    Branch      string `json:"branch"`
    BaseBranch  string `json:"baseBranch"`  // nama branch dasar; "" bila HEAD detached
    BaseCommit  string `json:"baseCommit"`  // hash HEAD saat spawn (selalu terisi)
    ProjectPath string `json:"projectPath"`
}
type WorktreeStatus struct {
    Dirty      []string `json:"dirty"`      // baris `git status --porcelain`; selalu [] bukan null
    Unmerged   []string `json:"unmerged"`   // "<hash> <subject>" commit di branch thread yang belum di BaseBranch; selalu []
    BaseBranch string   `json:"baseBranch"`
}
var ErrNotGitRepo = errors.New("project bukan repo git; thread tidak bisa mengisolasinya")
func Slugify(name string) string // huruf kecil, [a-z0-9-], strip '-' ujung; "" -> "project"; "master" -> "master-project"
func CreateWorktree(projectPath, dataDir, threadID, projectSlug string) (WorktreeInfo, error)
func InspectWorktree(info WorktreeInfo) (WorktreeStatus, error)
func RemoveWorktree(info WorktreeInfo) error // idempoten; aman bila direktori/branch sudah hilang
```

Perilaku `Unmerged`: bandingkan terhadap `BaseBranch` bila ada dan masih ada di repo; selain itu terhadap `BaseCommit`. Semua perintah git lewat `exec.CommandContext` dengan timeout (create 30 dtk, inspect 15 dtk, remove 30 dtk), `cmd.Dir`/`git -C`, `GIT_TERMINAL_PROMPT=0`, stderr disertakan di pesan galat. Galat bila binary `git` tak ditemukan: pesan jelas.

**`mcp_tools.go`** (stub Foundation berisi tipe dan `RegisterTool` fungsional minimal; `thread-mcp-server` memperkaya):

```go
type Caller struct{ ThreadID, SessionID string; Master bool }
type ToolFunc func(ctx context.Context, c Caller, args json.RawMessage) (any, error)
type ToolDef struct {
    Name, Description string
    InputSchema       map[string]any
    MasterOnly        bool
    Handler           ToolFunc
}
func RegisterTool(d ToolDef)        // nama kosong/duplikat atau Handler nil -> panic pesan jelas; aman konkuren
func registeredTools() []ToolDef    // urut nama (tak diekspor; dipakai mcp.go)

// Diisi/diperluas oleh thread-mcp-server (bukan stub):
type ToolDeps struct { Manager *Manager; Router Router; Logger *slog.Logger }
func RegisterCommonTools(d ToolDeps)       // thread_participants, thread_send, thread_read (panggil sekali)
func AgentToolsSection() string            // teks Indonesia untuk prompt master (bagian 5 no. 3)
```

Keluaran handler (`any`): string -> teks biasa; selain itu di-marshal JSON menjadi satu blok `text`. Galat -> hasil tool `isError:true` dengan pesan galat (JSON-RPC level tidak dipakai untuk galat bisnis).

**`mcp.go` / `launch.go`** (`thread-mcp-server`):

```go
type MCPConfig struct {
    Manager  *Manager
    Sessions SessionAPI // dari manager.go
    DataDir  string
    Tools    func() []ToolDef // default: gabungan registeredTools() + adaptasi Manager.Tools()
    Logger   *slog.Logger
}
type MCPServer struct{ /* tak diekspor */ }
func NewMCPServer(cfg MCPConfig) *MCPServer
func (s *MCPServer) Start(ctx context.Context) (url string, err error) // 127.0.0.1:0, path /mcp, tulis thread-mcp.json; ctx.Done -> Shutdown
func (s *MCPServer) URL() string
func (s *MCPServer) ReleaseSession(sessionID string)                   // lepas token (dipanggil saat sesi exit)

func ThreadIDFromCwd(dataDir, cwd string) (threadID string, ok bool)   // cwd di bawah <dataDir>/threads/<id>/...
func InstallLaunchContribution(s *MCPServer, m *Manager, dataDir string, logger *slog.Logger) // sekali (sync.Once)
```

Protokol server = salinan perilaku `internal/mention/mcp.go` (POST saja, `Authorization: Bearer`, 401 untuk token tak dikenal, JSON-RPC 2.0, `initialize` -> `serverInfo.name = "shikigami-thread"`, `ping`, `tools/list`, `tools/call`; batch tidak didukung; body maks 1 MiB). Helper JSON-RPC disalin dan diberi awalan `mcp` (tidak mengimpor `internal/mention`; tipe di sana tak diekspor). `tools/list` memfilter `MasterOnly` untuk peer; `tools/call` ke tool master-only oleh peer ditolak (`isError`). Token manager meniru `tokenManager` mention: `issue` saat `LaunchContribution` (menyimpan `threadId`, cwd, providerId, workspaceId, projectId), `lookup` mengikat token ke sesi running dengan `Cwd`/`ProviderID`/`WorkspaceID` yang sama dan belum terikat (FIFO; cwd unik per peserta sehingga tak ambigu), melepas bila sesi tak lagi running; maksimum 64 token tertunda. `Caller` di-resolve dari token -> sessionId -> `Participant` di `Manager.Get(threadID)`; thread selalu dari token, tidak pernah dari argumen. Sesi belum terdaftar sebagai peserta -> tool menjawab galat "sesi belum terdaftar sebagai peserta thread; coba lagi sebentar"; `tools/list` untuk sesi demikian hanya menampilkan tool non-master. Thread `closed` -> galat tool "thread sudah ditutup".

Konfigurasi MCP yang dipasang (format identik dengan mention): `MCPServers["shikigami-thread"] = {"type":"http","url":<URL>,"headers":{"Authorization":"Bearer <token>"}}`; syarat pasang: `ctx.Provider != nil && ctx.Provider.Capabilities().MCP`, `ThreadIDFromCwd(...)` ok, thread ada dan `State == active`, URL server sudah ada.

**Tool MCP** (nama dan argumen final):

| tool | peran | argumen | hasil |
|---|---|---|---|
| `thread_participants` | semua | `{}` | JSON array `ParticipantView` (status via `status.IsIdle`; `Self` true untuk pemanggil) |
| `thread_send` | semua | `{to: string, text: string}` | teks konfirmasi: "terkirim ke <nama>" atau "dicatat (user)"; galat dari router apa adanya |
| `thread_read` | semua | `{last_n?: integer 1..200, default 20}` | JSON array `ThreadMessage` terakhir, urut naik, hanya `kind` message dan system (activity dilewati) |
| `thread_spawn` | master | `{project: string, brief: string}` | teks: peserta, worktree, branch, atau catatan "dipakai ulang" |

**`spawn.go`** (`thread-spawn-router`):

```go
type SpawnerConfig struct {
    Manager    *Manager
    Sessions   SessionAPI
    Workspaces WorkspaceSource // dari manager.go
    Router     Router
    DataDir    string
    Logger     *slog.Logger
}
type Spawner struct{ /* tak diekspor */ }
func NewSpawner(cfg SpawnerConfig) *Spawner
func (sp *Spawner) RegisterTool() // RegisterTool(ToolDef{Name:"thread_spawn", MasterOnly:true, ...}); sekali
func BuildPeerBrief(t Thread, projectName, brief string, wt WorktreeInfo) string // aturan thread + brief (Indonesia, ringkas)
```

Urutan `Spawn` (di bawah mutex per thread): validasi thread active -> resolusi project dalam workspace thread (`Workspaces.Projects(t.WorkspaceID)`; cocok id, nama (case-insens), atau `Slugify(nama)`; ambigu/tak ada -> galat yang mendaftar project yang tersedia) -> reuse (2.2 no. 7) -> `CreateWorktree` (galat diteruskan apa adanya, mis. `ErrNotGitRepo`) -> `UpsertWorktreeRecord` (SessionID kosong) -> `Sessions.Create(SessionSpec{WorkspaceID, ProjectID: project.ID, Cwd: wt.Path, ProviderID})` -> `Manager.AddParticipant(Participant{Role: RolePeer, ProjectID, SessionID, Name})` -> update record dengan SessionID -> `PostSystem("master men-spawn <nama project>")` -> kirim brief awal lewat jalur router internal (log: teks `brief` master apa adanya, `from`=master, `to`=peer; injeksi: `BuildPeerBrief`). Rollback bila langkah apa pun SETELAH worktree gagal: `UpdateThread` buang peserta (bila sudah ditambah), `Sessions.Kill`, `RemoveWorktree`, `DeleteWorktreeRecord`, `PostSystem("gagal men-spawn <project>: <alasan>")`, galat ke master. Aturan thread dalam brief (wajib ada): bekerja HANYA di worktree sendiri (`<path>`) dan jangan menyentuh working tree utama project atau path lain di luarnya; jangan meminta izin; commit pekerjaan di branch `<branch>`; komunikasi hanya lewat tool `thread_send`/`thread_read`/`thread_participants` (bukan mengetik ke terminal agent lain); laporkan hasil ke master; jangan mem-push/merge kecuali diminta.

**`router.go` / `antiloop.go`** (`thread-spawn-router`):

```go
type RouterConfig struct {
    Manager  *Manager
    Guard    *AntiLoop
    Logger   *slog.Logger
}
func NewRouter(cfg RouterConfig) Router // implementasi Router (2.3 agents_types.go)

const MaxAgentChain = 8
type AntiLoop struct{ /* tak diekspor */ }
func NewAntiLoop(m *Manager) *AntiLoop       // mendaftarkan sendiri hook m.OnMessage dan m.OnClose
func (a *AntiLoop) Paused(threadID string) bool
func (a *AntiLoop) Check(t Thread, fromSessionID string) error // ErrPaused bila dijeda dan pengirim peer
func (a *AntiLoop) Reset(threadID string)
```

`Route`: validasi thread active dan teks tidak kosong (trim) -> identifikasi pengirim (`from` = sessionId peserta; `"user"` diizinkan) -> `Guard.Check` -> resolve target (2.2 no. 8) -> `Manager.Post(ThreadMessage{From, To (sessionId target, "user" atau "all"), Text, Kind: KindMessage})` -> untuk setiap target sesi (selain pengirim): `Manager.SendToSession(threadID, from, targetSessionID, "[thread] "+text)`; kegagalan satu target: pesan sistem "gagal meneruskan ke <nama>: <alasan>" dan galat dikembalikan (target lain tetap dicoba). `to == "user"`: hanya dicatat. Pengirim `user`: `SendToSession(..., from "", ...)`. Pesan ke diri sendiri ditolak. Pesan user dari `thread.send` (tiket 01) TIDAK dialihkan ke `Route`: `Manager.Send` tiket 01 sudah mencatat + mengirim ke master, dan hook `OnMessage` anti-loop menangkapnya untuk reset.

**Nama IPC/event:** tidak ada tambahan. Epic ini tidak menambah method IPC dan tidak menambah event; memakai `thread.updated`, `thread.message`, `thread.ownedSessions` dari tiket 01.

### 2.4 Kontrak frontend

Memakai (dari tiket 01, nama final di overview-01 2.5): `registerThreadSlot({slot, component, order?, id?})` dari `modules/thread/slots.ts`; `ThreadSlot` nama `'side-panel'` (kolom kanan tab), `'message-extra'` (per pesan, props `{thread, message}`); `threads`, `messagesByThread`, `participantName`, `avatarColor`, `initials` dari `thread-store.ts`/`format.ts`; `THREAD_METHODS`/`THREAD_EVENTS`, `call`/`onEvent` dari `lib/ipc.ts`; `statusOf(sessionId)` dari `modules/status/store.ts`; `layout` dari `lib/stores/layout.ts` (`openPane`, `setFocus`, `leaves`). Tiket 01 sudah merender pesan `system` (baris tengah, muted) dan pesan `message` (avatar, label `pengirim -> penerima`) tanpa dilipat; epic ini hanya MENAMBAH gaya lewat slot, tidak menggantikan renderer.

Disediakan `participants/` (semua diekspor dari `participants/index.ts`):

```ts
export function installParticipants(): () => void  // registerThreadSlot side-panel (id 'participants', order 0) dan message-extra (id 'participants-msg'); registerSessionFilter; mulai sinkron set sesi thread; kembalikan fungsi berhenti
export function isThreadSession(s: { id: string; cwd?: string }): boolean   // sinkron, baca snapshot store
export const threadSessionIds: Readable<Set<string>>                          // dari thread.ownedSessions + thread.updated
export const threadSessionPredicate: Readable<(s: Session) => boolean>        // true = TAMPIL di sidebar (bukan sesi thread)
export function openParticipantTerminal(p: Participant, thread: Thread): void // fokus bila pane sesi sudah ada, selain itu layout.openPane({kind:'terminal', sessionId, projectId, provider, workspaceId, title})
```

Titik ekstensi sidebar (Foundation, `modules/sidebar-left/registry.ts`, diekspor lewat `modules/sidebar-left/index.ts`):

```ts
export interface SessionFilterSpec { id: string; predicate: Readable<(s: Session) => boolean> } // true = tampil
export function registerSessionFilter(spec: SessionFilterSpec): void
export const sessionFilter: Readable<(s: Session) => boolean> // AND semua predicate terdaftar; tanpa terdaftar = () => true
```

`SessionGroups.svelte` menerapkan `$sessionFilter` DAN prop `filter` yang ada (komposisi AND), sehingga berlaku untuk daftar polos maupun yang dibungkus modul `groups`. Predikat reaktif: store `predicate` memancarkan fungsi baru bila himpunan berubah.

Format dan perilaku panel: baris peserta = avatar berwarna (`avatarColor(name)`), nama, role (master/peer), nama project, titik status kerja/idle dari `statusOf(sessionId)` (`running` = kerja; selain itu idle), lencana kecil "thread"; klik baris -> `openParticipantTerminal`. Peserta baru muncul lewat `thread.updated` (store `threads` tiket 01 sudah memperbarui; panel membaca `thread.participants` secara reaktif, tanpa reload). Jika `session.list` frontend belum memuat sesi itu, `openParticipantTerminal` memanggil `loadSessions()` dari `lib/stores/sessions.ts` lebih dulu (periksa bagaimana pane terminal mengambil sesi; ikuti `lib/stores/groups.ts` baris `layout.openPane({ kind: 'terminal', ... })`).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `thread-worktree` | `Slugify`, `CreateWorktree`, `InspectWorktree`, `RemoveWorktree` lewat `os/exec` + `git`; `ErrNotGitRepo`; penanganan bentrok branch dengan sufiks; menggantikan stub Foundation. | `internal/thread/worktree.go` |
| `thread-mcp-server` | `MCPServer` loopback `shikigami-thread` (token per sesi, JSON-RPC, filter peran), `RegisterTool` penuh + union dengan `Manager.Tools()`, `RegisterCommonTools` (`thread_participants`/`thread_send`/`thread_read`), `AgentToolsSection`, `ThreadIDFromCwd`, `InstallLaunchContribution`. | `internal/thread/mcp.go`, `internal/thread/mcp_tools.go`, `internal/thread/launch.go` |
| `thread-spawn-router` | `Spawner` + tool `thread_spawn`, `BuildPeerBrief`, rollback, `worktrees.json` lewat helper Foundation; `Router`/`NewRouter`; `AntiLoop` (+ `MaxAgentChain`). | `internal/thread/spawn.go`, `internal/thread/router.go`, `internal/thread/antiloop.go` |
| `thread-participants-ui` | `ParticipantsPanel`, `ParticipantRow`, `MessageExtra` (gaya pesan sistem "Thread dijeda"), `session-filter.ts` (`isThreadSession`, set sesi thread), `openParticipantTerminal`, `installParticipants`. | `frontend/src/modules/thread/participants/` |

Dependensi antar fitur setelah Foundation: tidak ada (semua paralel; lintas fitur hanya lewat signature bagian 2.3 dan stub Foundation).

## 4. Foundation

Ada (`00-foundation.md`). Isi ringkas:

- Langkah 0: verifikasi artefak tiket 01 (berkas Go, frontend, nama API); hentikan dan laporkan bila tidak ada.
- Go: `agents_types.go`, `worktree_records.go` (implementasi penuh); `worktree.go` dan `mcp_tools.go` sebagai stub berkompilasi dengan signature final (stub `worktree.go` mengembalikan galat "belum diimplementasikan"; `RegisterTool` stub sudah fungsional minimal).
- Frontend: `registerSessionFilter`/`sessionFilter` di `modules/sidebar-left/registry.ts`, ekspor di `index.ts`, penerapan di `SessionGroups.svelte`.
- Guard: `go build ./...` dan `cd frontend && npm run check && npm run build`; perilaku aplikasi tanpa modul thread tidak berubah.

## 5. Wiring checklist (setelah semua fitur selesai)

Hanya berkas berikut yang boleh disentuh agent wiring:

1. **`internal/thread/agents_install.go` (BARU, milik wiring)**:
   ```go
   type AgentsConfig struct {
       Manager       *Manager
       Sessions      SessionAPI
       Workspaces    WorkspaceSource
       DataDir       string
       Logger        *slog.Logger
       OnSessionExit func(fn func(sessionID string)) // boleh dipanggil beberapa kali
   }
   type Agents struct{ MCP *MCPServer; Router Router; Spawner *Spawner; Guard *AntiLoop }
   func InstallAgents(ctx context.Context, cfg AgentsConfig) (*Agents, error)
   ```
   Urutan di dalam: `NewMCPServer` -> `Start(ctx)` (galat -> kembalikan galat) -> `cfg.OnSessionExit(srv.ReleaseSession)` -> `InstallLaunchContribution(srv, mgr, dataDir, logger)` -> `NewAntiLoop(mgr)` -> `NewRouter` -> `RegisterCommonTools(ToolDeps{...})` -> `NewSpawner(...).RegisterTool()` -> `ToolsSection = AgentToolsSection` (variabel paket tiket 01 di `prompt.go`; bila di kode berbeda, ikuti kode) -> kembalikan `*Agents`. Aman dipanggil hanya sekali (guard `sync.Once`; panggilan kedua mengembalikan galat).
2. **`cmd/shikigamid/wire_thread.go` (milik tiket 01, perubahan minimal)**: di `wireThread`, SETELAH `thread.NewManager(...)`, `d.Provide(...)`, dan `thread.RegisterIPC`, dan SEBELUM `mgr.FireStart()`, panggil `thread.InstallAgents(context.Background(), thread.AgentsConfig{...})` dengan `Sessions: sm`, adapter workspace yang sama dengan `WorkspaceSource` tiket 01, `DataDir: dataDir`, `Logger: d.Logger`, `OnSessionExit`: bila `wire_thread.go` sudah punya fan-out sesi-exit (mirip `hub.add` di `wire_mention.go`), pakai itu; bila langganan `session.exit` hanya memanggil `q.HandleExit`, ubah menjadi fan-out kecil (slice fungsi + mutex) agar `q.HandleExit` dan `srv.ReleaseSession` sama-sama terpanggil. Galat `InstallAgents` -> `d.Logger.Warn` (modul thread tetap jalan tanpa tool; jangan panic).
3. **Prompt master**: `ToolsSection` diisi `AgentToolsSection()` (dilakukan di `InstallAgents`, butir 1). Verifikasi teksnya menyebut `thread_spawn`, `thread_send`, `thread_read`, `thread_participants` dan tetap wajar dengan awalan `[pesan dari user]`. Cek manual bahwa master Claude benar-benar menampilkan server `shikigami-thread` (mis. `/mcp` di sesi master).
4. **Frontend**: di `frontend/src/modules/thread/index.ts` (milik tiket 01) tambah satu impor dan satu panggilan: `import { installParticipants } from './participants'; installParticipants();`. Perubahan di `modules/sidebar-left` sudah dikerjakan Foundation (disebut eksplisit: `registry.ts`, `index.ts`, `SessionGroups.svelte`); wiring hanya memverifikasi keberadaannya.
5. **Tidak ada** perubahan di `internal/mention`, `internal/provider`, `internal/session`, `cmd/shikigamid/wire_mention.go`, `master.go`.
6. **Verifikasi manual** (jalankan aplikasi): buat thread, minta master "libatkan project <git>"; cek `<DataDir>/threads/<id>/<slug>` dan `git -C <project> branch` memuat `thread/<id>-<slug>`, working tree utama bersih; peserta muncul di panel kanan dengan status hidup; sesi master/peer TIDAK muncul di sidebar kiri; pesan bolak-balik terlihat di aliran sebagai `pengirim -> penerima`; spawn project non-git ditolak dengan pesan jelas tanpa worktree; spawn kedua project sama memakai ulang agent; paksa 8 pesan peer-ke-peer lalu pastikan jeda dan pesan user melanjutkan; mode global mention `confirm_agent` tidak mempengaruhi pesan thread.

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- `go build ./...` (dari akar repo) — dijalankan setiap agent fitur Go setelah selesai dan oleh Wiring.
- Frontend: `cd frontend && npm run check && npm run build` (opsional `make build` bila tersedia). Dijalankan oleh `thread-participants-ui`, Foundation, dan Wiring.
- Aplikasi tetap jalan: `go build ./cmd/shikigamid` tanpa galat; tidak ada dependency baru di `go.mod`/`package.json`.

## 7. Aturan paralel

- Tiap agent HANYA menyentuh `ownedPaths` fiturnya (bagian 3). Dilarang menyentuh path fitur lain, berkas Foundation (`agents_types.go`, `worktree_records.go`, sidebar-left), berkas wiring (`agents_install.go`, `wire_thread.go`, `thread/index.ts`), berkas tiket 01, `internal/mention`, `internal/provider`, `internal/session`.
- Bila butuh perubahan di luar `ownedPaths`, JANGAN dikerjakan: catat di bagian "Catatan untuk wiring" pada plan fitur.
- Kompilasi lintas fitur hanya mengandalkan signature bagian 2.3 dan stub Foundation. Jangan mengubah signature; jangan menambah ekspor baru yang dipakai fitur lain. Helper internal wajib berawalan berkas (2.1).
- Fitur Go yang butuh nama di kode tiket 01 yang berbeda dari overview-01: ikuti kode, jangan mengedit berkas tiket 01.
- Jangan menyimpan state global selain yang tertulis di kontrak (registri `RegisterTool`, token manager `MCPServer`, `sync.Once` instalasi).
- Setiap agent menjalankan guard bagian 6 yang relevan sebelum melapor selesai; kegagalan build akibat berkas fitur lain diabaikan hanya bila jelas bukan milik fiturnya (laporkan).
