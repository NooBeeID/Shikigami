# Overview: thread-ticket-01-thread-core

Tiket: `docs/tickets/thread-ticket-01-thread-core.md`. Spec: `docs/superpowers/specs/2026-11-22-thread-design.md`.
Dokumen ini adalah sumber kebenaran BERSAMA untuk semua plan fitur. Bila ada perbedaan kecil dengan tiket, tiket menang kecuali dinyatakan "koreksi" di bagian 2.2 (koreksi muncul karena kode nyata berbeda dari asumsi tiket). Tanpa dependency baru, tanpa unit test; guard hanya build.

Keadaan repo saat overview ditulis: belum ada `internal/thread/`, `cmd/shikigamid/wire_thread.go`, maupun `frontend/src/modules/thread/`. Tiket thread-02 s.d. thread-05 baru berupa tiket; epic ini hanya menyediakan titik ekstensi.

## 1. Ringkasan epic dan urutan eksekusi

User menulis satu goal; daemon membuat `Thread` (disimpan di disk), men-spawn sesi master (provider agent, auto-approve, cwd netral `DataDir/threads/<id>/master`) dengan prompt awal, dan UI menampilkan daftar thread di sidebar kiri serta tab chat (aliran pesan + input ke master). Master belum punya tool thread (itu tiket thread-agents).

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): `internal/thread/types.go` final (lengkap) dan `internal/thread/store.go` berupa stub berkompilasi dengan signature final; di frontend, tiga penyesuaian host kecil (model pane, slot bagian sidebar). Setelah selesai `go build ./...` dan build frontend lulus.
2. **Fitur paralel** (3 fitur, bagian 3): `thread-foundation` (mengganti stub `store.go` dengan implementasi penuh, boleh menyempurnakan `types.go` tanpa mengubah bentuk), `thread-master-daemon`, `thread-ui`. Signature bagian 2 TIDAK berubah.
3. **Wiring** (bagian 5): verifikasi registrasi modul daemon dan modul frontend, pengecekan manual alur penuh.

## 2. Kontrak bersama

### 2.1 Paket, folder, penamaan

- Modul Go `github.com/NooBeeID/shikigami`; paket `internal/thread` (package `thread`). Berkas: `types.go`, `store.go`, `manager.go`, `master.go`, `prompt.go`, `ipc.go`; wiring daemon `cmd/shikigamid/wire_thread.go` (package `main`).
- Frontend: `frontend/src/modules/thread/` (Svelte 5 runes, TypeScript). Satu-satunya titik registrasi: `index.ts`.
- Komentar, log, pesan galat, teks UI bahasa Indonesia; identifier bahasa Inggris. Logger `*slog.Logger` nullable (cek nil, fallback `slog.Default()`).
- Helper tak-diekspor diberi awalan berkas (`store*`, `manager*`, `master*`, `prompt*`; di package `main`: `threadWire*`) agar tidak bentrok antar berkas. Konstanta/tipe didefinisikan hanya di berkas pemiliknya (bagian 2.3).
- JSON camelCase. Waktu `time.Time` UTC, disimpan RFC3339Nano.
- ID thread dan pesan: `thread.NewID()` di `types.go` = pembungkus `mention.NewMessageID()` (ULID 26 karakter, monotonik per milidetik; tanpa dependency baru, tanpa mengedit `internal/mention`).
- Konvensi path (hanya dokumentasi di komentar paket; worktree diimplementasi tiket thread-02): `DataDir/threads/<threadId>/thread.json`, `.../messages.jsonl`, `.../master/`, `.../<projectSlug>/` (worktree), branch `thread/<threadId>-<slug>`.

### 2.2 Koreksi atas tiket (WAJIB diikuti)

1. **Hook UI sebenarnya bukan `registerPanel`/`registerTabType`.** Tiket menyuruh memakai `lib/registry.ts` (`registerPanel` area `sidebar`, `registerTabType`). Di kode nyata tidak ada komponen yang merender `panels()` atau `tabTypes()` (peninggalan v1; `App.svelte` memakai `AgentSidebar` dan `LayoutArea`). Area tengah adalah pohon pane (`modules/layout`): isi tab dirender lewat `registerPaneKind({kind, component, titleFor})` dari `modules/layout/registry.ts` dan dibuka lewat `layout.openPane(...)` dari `lib/stores/layout.ts`. Maka:
   - Tab thread = pane `kind: 'thread'` (komponen `ThreadPane.svelte`, props `{pane: Leaf, visible: boolean}`, membaca `pane.threadId`, merender `ThreadTab`).
   - Bagian "Thread" di sidebar kiri didaftarkan lewat slot baru `registerSidebarSection` (dibuat Foundation di `modules/sidebar-left/registry.ts`, dirender `AgentSidebar.svelte` di antara `WorkspaceHeader` dan daftar sesi).
   - `index.ts` tetap memanggil `registerTabType({type: 'thread', component: ThreadTab})` dan `thread-ui` tetap memperluas union `TabTypeSpec.type` di `lib/registry.ts` (satu baris) agar acceptance tiket terpenuhi dan jalur v1 tetap konsisten; hasilnya tidak dirender siapa pun saat ini. `registerPanel` TIDAK dipanggil (akan menjadi panel yang tidak pernah tampil / berisiko dobel render).
2. **Perluasan model pane diperlukan.** `PaneKind` hanya `'terminal' | 'file'`, `Leaf`/`NewLeaf` tidak punya field id thread, `hydrateNode` membuang field tak dikenal, dan `openPane` hanya menghindari duplikat lewat `sessionId`/`filePath`. Foundation menambah: `'thread'` pada `PaneKind`, `threadId?: string` pada `Leaf` dan `NewLeaf`, `threadId` pada daftar kunci di `hydrateNode`, `makeLeaf` menyalin `threadId`, dan dedup `openPane` berdasar `threadId`. Ini keluar dari ownedPaths tiket, tetapi tanpa ini tab thread tidak bisa dibuka/dipulihkan; dikerjakan Foundation, bukan fitur paralel.
3. **"Provider default aplikasi" = bukan `provider.Default()`.** `provider.Default()` mengembalikan provider shell (bukan agent LLM). Master memakai fungsi `resolveMasterProvider()` di `master.go`: urutan preferensi `provider.IDClaude` lalu `provider.IDCodex`, yang pertama `Available()` tanpa galat (lewat `provider.Get(id)`); bila tidak ada, `SpawnMaster` mengembalikan galat jelas ("tidak ada provider agent (claude/codex) yang tersedia") yang dicatat sebagai pesan sistem. Tidak ada konfigurasi default per-user di `internal/appconfig` saat ini.
4. **Antrean mention untuk thread = instance `mention.Queue` KHUSUS thread, bukan instance global.** `wire_mention.go` tidak mem-`Provide` antreannya, dan tidak boleh diedit. `wire_thread.go` membuat `mention.NewQueue(mention.QueueConfig{Sessions: mgr, Namer: namer, Publish: noop, Logger: d.Logger})`, lalu `q.SetMode(mention.ModeAuto)` (agent-ke-agent tanpa konfirmasi di antrean ini saja; mode global instance mention biasa tidak tersentuh) dan menyambungkan `q.HandleExit` ke event `session.exit` (pola `subscribeExit` di `wire_mention.go`, ditulis ulang di `wire_thread.go`). `Publish` diberi fungsi kosong agar item thread tidak muncul di badge antrean mention UI. Pesan user ke master memakai `Enqueue(ctx, "", masterSessionID, text, mention.SourceUser)`. Antrean ini mengirim `"[pesan dari <FromName>] <teks>"` (perilaku `deliver`), jadi prompt awal master dikirim lewat jalur yang sama dan akan berawalan "[pesan dari user]"; `prompt.go` harus menyusun teks yang tetap wajar dengan awalan itu. Tidak ada perubahan pada `internal/mention`.
5. **Direktori workspace/project untuk thread.** `workspaceDirectory` di `wire_mention.go` hanya memetakan nama project dan tidak mengekspos path. `wire_thread.go` membuat adapter sendiri (`threadWorkspaceDirectory`) yang memanggil IPC internal `workspace.list` lewat `ipc.Dial(d.SocketPath)` (meniru pola cache 5 detik dan decode ganda: array `[]Workspace` atau objek `{workspaces: [...]}`) dan memenuhi interface `WorkspaceSource` (2.3). `Namer` untuk antrean thread memakai adapter yang sama sebagai `mention.Directory` (`ProjectName(workspaceID, projectID)`).
6. **Satu modul daemon, `d.Provide("thread.manager", mgr)`.** Setelah dibuat, `wireThread` mem-`Provide("thread.manager", *Manager)` (sekali) agar modul tiket berikut (`thread-agents`, dst.) mengambilnya lewat `d.Lookup` di dalam `d.OnReady`. Modul diberi nama `"thread"`.
7. **Pemberitahuan pesan sistem untuk spawn master.** `Create` mengembalikan `Thread` setelah semua langkah (termasuk `SpawnMaster`) selesai atau gagal; kegagalan spawn bukan galat `thread.create` (thread tetap dibuat dan dikembalikan, dengan pesan sistem galat).
8. **`thread.send` ke master yang belum ada.** Bila master gagal di-spawn atau sesinya sudah mati (tidak ada `Participant` master atau `Enqueue` mengembalikan `mention.ErrNotFound`), pesan user TETAP dicatat ke log, lalu dicatat pesan sistem "gagal meneruskan ke master: <alasan>"; `thread.send` tidak mengembalikan galat dalam kasus itu (galat hanya untuk validasi: thread tidak ada, closed, teks kosong).

### 2.3 Signature Go bersama (final)

**`types.go`** (ditulis penuh oleh Foundation; fitur `thread-foundation` boleh menambah komentar atau field opsional `omitempty`, tidak mengubah yang ada):

```go
package thread

const (
    StateActive = "active"
    StateClosed = "closed"

    RoleMaster = "master"
    RolePeer   = "peer"

    KindMessage  = "message"
    KindSystem   = "system"
    KindActivity = "activity"

    FromUser   = "user"
    FromSystem = "system"
    ToUser     = "user"
    ToAll      = "all"

    MethodCreate        = "thread.create"
    MethodList          = "thread.list"
    MethodGet           = "thread.get"
    MethodMessages      = "thread.messages"
    MethodSend          = "thread.send"
    MethodOwnedSessions = "thread.ownedSessions"
    // Ditambah tiket thread-close: "thread.closePreview", "thread.close" (lewat RegisterMethod).

    EventUpdated = "thread.updated"
    EventMessage = "thread.message"
)

type Thread struct {
    ID           string        `json:"id"`
    WorkspaceID  string        `json:"workspaceId"`
    Title        string        `json:"title"`
    Goal         string        `json:"goal"`
    State        string        `json:"state"`
    CreatedAt    time.Time     `json:"createdAt"`
    ClosedAt     *time.Time    `json:"closedAt,omitempty"`
    Participants []Participant `json:"participants"` // selalu [] (bukan null)
    ReadyToClose bool          `json:"readyToClose"`
}
type Participant struct {
    SessionID string `json:"sessionId"`
    ProjectID string `json:"projectId"` // kosong untuk master
    Role      string `json:"role"`
    Name      string `json:"name"`
}
type ThreadMessage struct {
    ID       string    `json:"id"`
    ThreadID string    `json:"threadId"`
    From     string    `json:"from"`
    To       string    `json:"to"`
    Text     string    `json:"text"`
    TS       time.Time `json:"ts"`
    Kind     string    `json:"kind"`
}

type CreateParams   struct{ WorkspaceID, Goal, Title string }   // json: workspaceId, goal, title(omitempty)
type ListParams     struct{ WorkspaceID string }                 // json: workspaceId
type GetParams      struct{ ID string }                          // json: id
type MessagesParams struct{ ID, SinceTS string }                 // json: id, sinceTs(omitempty; RFC3339/RFC3339Nano)
type SendParams     struct{ ID, Text string }                    // json: id, text
type OwnedSessionsResult struct{ SessionIDs []string }           // json: sessionIds (selalu [])

func NewID() string // pembungkus mention.NewMessageID()
```

(Tag json ditulis eksplisit di kode; blok di atas meringkas. Foundation menulis semua field param dengan tag `json:"workspaceId"`, dst.)

**`store.go`** (stub di Foundation, implementasi penuh oleh `thread-foundation`; signature final):

```go
type Store struct { /* tak diekspor */ }
var (
    ErrNotFound = errors.New("thread tidak ditemukan")  // didefinisikan di store.go
    ErrInvalid  = errors.New("masukan thread tidak valid")
)
func NewStore(baseDir string) *Store                       // baseDir = DataDir/threads (diinjeksi); membuat direktori bila belum ada
func (s *Store) Create(workspaceID, title, goal string) (Thread, error) // ID baru NewID(), State active, Participants []; buat thread.json, messages.jsonl kosong, direktori master
func (s *Store) Get(id string) (Thread, error)             // ErrNotFound bila tidak ada
func (s *Store) List(workspaceID string) ([]Thread, error) // createdAt menurun, termasuk closed; thread.json rusak dilewati (log)
func (s *Store) Save(t Thread) error                       // atomik: tulis .tmp lalu rename
func (s *Store) AppendMessage(m ThreadMessage) error       // append satu baris JSON; mutex per thread; m.ID/TS diisi pemanggil
func (s *Store) Messages(id string, sinceTS time.Time) ([]ThreadMessage, error) // sinceTS nol = semua; hanya TS > sinceTS; urut naik; baris rusak dilewati (log)
func (s *Store) ThreadDir(id string) string
func (s *Store) MasterDir(id string) string                // ThreadDir(id)/master
```

`Store.Create` tidak menetapkan judul turunan; judul diturunkan di manager (`deriveTitle`). Izin berkas 0600, direktori 0700.

**`manager.go`** (milik `thread-master-daemon`):

```go
// Antarmuka yang dipenuhi adapter di wire_thread.go.
type ProjectInfo struct{ ID, Name, Path string }
type WorkspaceSource interface {
    Projects(workspaceID string) (projects []ProjectInfo, ok bool) // ok=false: workspace tak dikenal / daemon workspace tak terjangkau
}
type Publisher func(topic string, payload any)               // srv.Publish
type Sender interface {                                       // dipenuhi *mention.Queue (instance thread)
    Enqueue(ctx context.Context, from, to, text, source string) (mention.Message, error)
}
type SessionAPI interface {                                   // dipenuhi *session.Manager
    Create(spec session.SessionSpec) (*session.Session, error)
    Get(id string) (*session.Session, error)
    Kill(id string) error
    List() []session.Session
}

type ManagerConfig struct {
    Store      *Store
    Sessions   SessionAPI
    Sender     Sender
    Workspaces WorkspaceSource
    Namer      *mention.Namer // untuk Participant.Name; boleh nil -> "master"
    Publish    Publisher
    Logger     *slog.Logger
}
func NewManager(cfg ManagerConfig) *Manager

func (m *Manager) Create(workspaceID, goal, title string) (Thread, error)
func (m *Manager) Get(id string) (Thread, error)
func (m *Manager) List(workspaceID string) ([]Thread, error)
func (m *Manager) Messages(id string, sinceTS time.Time) ([]ThreadMessage, error)
func (m *Manager) Post(threadID string, msg ThreadMessage) (ThreadMessage, error) // isi ID/TS/ThreadID bila kosong; tolak bila closed; catat -> publish thread.message -> hook OnMessage. AppendMessage = alias.
func (m *Manager) AppendMessage(threadID string, msg ThreadMessage) (ThreadMessage, error)
func (m *Manager) PostSystem(threadID, text string) // pesan sistem From=FromSystem To=ToAll Kind=KindSystem; tidak menolak thread closed (kegagalan dicatat log saja)
func (m *Manager) Send(threadID, text string) (ThreadMessage, error)      // thread.send (pesan user ke master)
func (m *Manager) AddParticipant(threadID string, p Participant) (Thread, error) // simpan, publish thread.updated, hook OnParticipantAdded
func (m *Manager) UpdateThread(threadID string, fn func(*Thread)) (Thread, error) // baca-ubah-simpan di bawah mutex; publish thread.updated
func (m *Manager) SendToSession(threadID, from, to, text string) (mention.Message, error) // antrean thread; from "" = user (SourceUser) selain itu sessionId (SourceAgent)
func (m *Manager) OwnedSessions() []string                 // semua Participant.SessionID dari semua thread (termasuk closed? tidak: hanya state active)
func (m *Manager) OnStart(fn func(*Manager))
func (m *Manager) OnMessage(fn func(Thread, ThreadMessage))
func (m *Manager) OnParticipantAdded(fn func(Thread, Participant))
func (m *Manager) OnClose(fn func(Thread))
func (m *Manager) FireStart()                               // dipanggil wire_thread.go di d.OnReady setelah semua metode terdaftar
func (m *Manager) FireClose(t Thread)                       // dipanggil tiket thread-close
func (m *Manager) Tools() *ToolRegistry
func (m *Manager) Store() *Store

type ThreadCaller struct{ ThreadID, SessionID, Role string }
type ToolSpec struct {
    Name, Description string
    MasterOnly        bool
    Handler           func(ctx context.Context, caller ThreadCaller, args map[string]any) (any, error)
}
type ToolRegistry struct { /* mutex + map; urutan Tools() = urut nama */ }
func (r *ToolRegistry) Register(name string, tool ToolSpec) // nama kosong/duplikat -> panic dengan pesan jelas
func (r *ToolRegistry) Tools() []ToolSpec
```

Hook boleh didaftarkan banyak pihak; dipanggil berurutan pendaftaran di goroutine pemanggil, panic di hook dipulihkan (`recover`) dan dicatat. Error sentinel sisi manager: pakai `ErrNotFound`, `ErrInvalid` (store.go) dan `ErrClosed = errors.New("thread sudah ditutup")` (di `manager.go`).

**`master.go`**: `func (m *Manager) SpawnMaster(t Thread) (Participant, error)`; `func resolveMasterProvider() (provider.Provider, error)`. **`prompt.go`**: `func BuildMasterPrompt(t Thread, projects []ProjectInfo) string` dan variabel paket `var ToolsSection = func() string { ... }` (default: kalimat "tool thread segera tersedia"; tiket thread-agents menggantinya). **`ipc.go`**: `func RegisterIPC(srv *ipc.Server, m *Manager)` dan `func (m *Manager) RegisterMethod(name string, handler func(params json.RawMessage) (any, error))` (daftar `map` di manager; bila `RegisterIPC` belum dipanggil, tersimpan dan dipasang saat `RegisterIPC`; setelah itu langsung `srv.Register`). Galat dipetakan ke `ipc.NewError(ipc.ErrNotFound|ErrInvalid|ErrInternal, ...)`. `RegisterMethod` didefinisikan di `ipc.go` (milik `thread-master-daemon`); karena butuh state pada `Manager`, field `methods`/`srv` ada di struct `Manager` di `manager.go` (satu agent memiliki kedua berkas).

### 2.4 Kontrak IPC (perilaku)

- `thread.create{workspaceId, goal, title?}` -> `Thread`. Goal kosong (setelah trim) atau workspace tak dikenal -> `ipc.ErrInvalid`. Judul kosong -> baris pertama goal, trim, dipotong 60 rune (ditambah `…` bila terpotong).
- `thread.list{workspaceId}` -> `Thread[]` (array JSON langsung, `[]` bila kosong). `thread.get{id}` -> `Thread`; tidak ada -> `ipc.ErrNotFound`.
- `thread.messages{id, sinceTs?}` -> `ThreadMessage[]` (array langsung); `sinceTs` tak valid -> `ErrInvalid`.
- `thread.send{id, text}` -> `ThreadMessage`; closed -> `ErrInvalid` dengan pesan "thread sudah ditutup"; teks kosong -> `ErrInvalid`.
- `thread.ownedSessions{}` -> `{"sessionIds": [...]}`.
- Event `thread.updated` payload `Thread` penuh; `thread.message` payload `ThreadMessage` (publish lewat `Publisher`, yaitu `srv.Publish`, pola `internal/mention/ipc.go`).

### 2.5 Kontrak frontend

`types.ts` (dibuat `thread-ui`, cermin 2.3): `Thread`, `Participant`, `ThreadMessage`, konstanta `THREAD_METHODS`, `THREAD_EVENTS` (`thread.updated`, `thread.message`), `ThreadState`, `ThreadRole`, `ThreadKind`. Pola klien/galat/`asList` meniru `modules/mention/mention-client.ts` (`call` dan `onEvent` dari `lib/ipc.ts`; `errorText` versi thread di `thread-client.ts`).

- `thread-store.ts`: store Svelte untuk `threads` (per workspace aktif, urut createdAt menurun), `messagesByThread`, fungsi `loadThreads(workspaceId)`, `loadMessages(threadId)`, `createThread(goal)`, `sendMessage(threadId, text)`, `openThreadPane(thread)`, `subscribeThreadEvents()` (dipanggil sekali dari `index.ts`; mengembalikan fungsi berhenti). Dedup pesan by `id`, urut `ts` naik.
- `slots.ts`: `registerThreadSlot({slot: 'header-actions'|'above-input'|'message-extra'|'side-panel', component, order?})`; `ThreadSlot.svelte` props `{name, thread, message?}` merender semua komponen slot itu terurut `order`. Komponen slot menerima props `{thread: Thread, message?: ThreadMessage}`.
- Membuka tab: `layout.openPane({kind: 'thread', threadId: t.id, workspaceId: t.workspaceId, title: t.title})` (dedup per `threadId` dari Foundation).
- `index.ts` (satu-satunya titik registrasi): `registerPaneKind({kind: 'thread', component: ThreadPane, titleFor: (leaf) => leaf.title || 'Thread'})`, `registerSidebarSection({id: 'thread-sidebar', component: ThreadSection, order: 0})`, `registerTabType({type: 'thread', component: ThreadTab})`, lalu `subscribeThreadEvents()`.
- Slot sidebar (dibuat Foundation di `modules/sidebar-left/registry.ts`): `interface SidebarSectionSpec { id: string; component: Component<any>; order?: number }`, `registerSidebarSection(spec)`, `sidebarSections(): Readable<SidebarSectionSpec[]>` (terurut `order` lalu `id`). `AgentSidebar.svelte` merender semua section (props `{onerror}`) di antara `<WorkspaceHeader>` dan daftar sesi.
- Token tema: pakai variabel CSS yang sudah ada (`--bg-elev`, `--border`, `--fg-muted`, `--danger`, dst.; periksa `frontend/src/styles/theme.css`); jangan menambah warna keras kecuali palet avatar deterministik (HSL dari hash).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `thread-foundation` | Implementasi penuh `Store` (`store.go`) menggantikan stub Foundation; penyempurnaan komentar/dokumentasi `types.go` (bentuk tidak berubah); komentar paket konvensi path/branch. | `internal/thread/types.go`, `internal/thread/store.go` |
| `thread-master-daemon` | `Manager` (hook, `ToolRegistry`, `Post`/`AddParticipant`/`UpdateThread`/`SendToSession`/`OwnedSessions`), `SpawnMaster` (provider agent, cwd master, auto-approve lewat launcher, simpan Participant master, kirim prompt awal saat idle), `BuildMasterPrompt`, IPC `thread.*` + `RegisterMethod`, publish event, `wire_thread.go` (store, antrean mention khusus thread, adapter workspace, `Provide`, `OnStart`). | `internal/thread/manager.go`, `internal/thread/master.go`, `internal/thread/prompt.go`, `internal/thread/ipc.go`, `cmd/shikigamid/wire_thread.go` |
| `thread-ui` | Modul frontend: client, store, bagian sidebar, dialog, pane/tab chat, header, daftar & item pesan, input, slot render, `index.ts` (registrasi pane kind + section sidebar + tab type legacy), perluasan union `TabTypeSpec.type`. | `frontend/src/modules/thread/`, `frontend/src/lib/registry.ts` (hanya union `'thread'`) |

Dependensi antar fitur setelah Foundation: tidak ada (ketiganya paralel). `thread-master-daemon` mengkompilasi terhadap signature `Store` (stub Foundation sampai `thread-foundation` selesai); `thread-ui` murni frontend dan hanya bergantung pada kontrak IPC (boleh dikembangkan tanpa daemon).

## 4. Foundation

Ada (`00-foundation.md`). Isi ringkas:

- Go: `internal/thread/types.go` lengkap (2.3) dan `internal/thread/store.go` stub (signature final, badan mengembalikan `errors.New("store thread belum diimplementasikan")`, termasuk `ErrNotFound`/`ErrInvalid`). Tidak ada berkas Go lain.
- Frontend: (a) `modules/layout/model.ts` (PaneKind/Leaf/NewLeaf/hydrate/makeLeaf/openPane dedup, koreksi 2.2 no. 2); (b) `modules/sidebar-left/registry.ts` + `AgentSidebar.svelte` (slot `registerSidebarSection`, koreksi 2.2 no. 1).
- Guard: `go build ./...` dan `cd frontend && npm run check && npm run build` lulus; perilaku aplikasi tanpa modul thread tidak berubah (slot kosong tidak merender apa pun).

## 5. Wiring checklist (dikerjakan setelah semua fitur selesai)

Sebagian besar sudah ditanam oleh fitur; agen wiring MEMVERIFIKASI dan menambal bila ada yang kurang. Hanya file wiring/registrasi berikut yang boleh disentuh:

1. `cmd/shikigamid/wire_thread.go` memuat `func init() { RegisterModule("thread", func(srv *ipc.Server, d *Deps) { d.OnReady(func() { wireThread(srv, d) }) }) }`. `wireThread` (a) mengambil `session.Manager` lewat `d.Lookup("session.manager")` (jika tidak ada: log peringatan, modul nonaktif, jangan panic); (b) membuat `thread.NewStore(filepath.Join(dataDir, "threads"))` dengan `dataDir = d.DataDir` (fallback `paths.DataDir()`); (c) membuat adapter workspace, `mention.Namer`, antrean khusus thread `ModeAuto`, langganan `session.exit` -> `q.HandleExit` (2.2 no. 4 dan 5); (d) `thread.NewManager(...)`; (e) `d.Provide("thread.manager", mgr)`; (f) `thread.RegisterIPC(srv, mgr)`; (g) `mgr.FireStart()`. Tidak mengubah `main.go`/`modules.go`/`wire_mention.go`.
2. Tidak ada tambahan yang perlu di `cmd/shikigamictl` (CLI di luar scope).
3. Frontend: `frontend/src/modules/thread/index.ts` ter-import otomatis lewat `import.meta.glob('./modules/*/index.ts')` di `main.ts` (tidak diedit). Verifikasi `registerPaneKind({kind:'thread'})`, `registerSidebarSection`, `registerTabType`, dan `subscribeThreadEvents` dipanggil di `index.ts`, serta union `TabTypeSpec.type` di `lib/registry.ts` memuat `'thread'`.
4. Verifikasi bagian Foundation masih utuh: `PaneKind` memuat `'thread'`, `threadId` dipertahankan oleh `hydrateDoc` (muat ulang layout ter-persist tidak menghilangkan tab thread), `AgentSidebar` merender section.
5. Verifikasi manual alur penuh di aplikasi (`make build` lalu jalankan `make app` atau daemon + UI): bagian "Thread" tampil di sidebar kiri di atas daftar sesi; buat thread dengan goal; tab thread terbuka; master muncul sebagai sesi terminal dengan prompt awal; kirim pesan dari tab thread dan lihat terkirim saat idle; tutup dan buka ulang aplikasi, thread dan riwayat masih ada; kegagalan spawn (mis. PATH tanpa claude/codex) muncul sebagai pesan sistem, bukan crash.
6. Pastikan fitur lain tidak berubah: sesi biasa, mention (mode `confirm_agent`, badge antrean tidak menampilkan item thread), grup tab, dan panel lain.

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- Go: `go build ./...` (dari root repo; harus lulus di akhir Foundation dan setelah tiap fitur Go).
- Frontend: `cd frontend && npm run check && npm run build` (`check` = `svelte-check --tsconfig ./tsconfig.json`; `build` = `vite build`). Bila `node_modules` belum ada: `npm install` dulu (tidak menambah dependency).
- `make check` (= `go build ./...`) setara guard Go. Jangan menjalankan `make app`/`make install` di dalam agen fitur.
- Fitur Go cukup menjalankan guard Go; `thread-ui` cukup guard frontend; Wiring menjalankan keduanya.

## 7. Aturan paralel

- Setiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, berkas Foundation (kecuali fitur pemiliknya: `types.go`/`store.go` milik `thread-foundation`), maupun file wiring (`main.go`, `modules.go`, `wire_mention.go`, `lib/stores/*`, `modules/layout/*`, `modules/sidebar-left/*`).
- `thread-foundation` menimpa penuh `store.go` stub; tidak mengubah signature bagian 2.3 dan tidak mengubah bentuk/tag json di `types.go`.
- Bila agent membutuhkan perubahan di path yang bukan miliknya, JANGAN mengubahnya; catat di bagian "Catatan untuk wiring" di akhir plan fiturnya.
- Pengecualian tiket ("tambahan aditif di `internal/mention`" dan "penyesuaian render sidebar") sudah ditiadakan/diambil alih Foundation (2.2 no. 1 dan 4): `internal/mention` TIDAK diedit.
- Sesi interaktif milik user tidak pernah ditulis atau di-kill oleh kode thread; hanya sesi yang dibuat thread (`Participant.SessionID`) yang boleh disentuh.
- Tanpa dependency baru (`go.mod`, `package.json` tidak berubah). Tanpa unit test.
