# Plan: Manager thread, spawn master, dan IPC

## Metadata

- Tiket: `thread-ticket-01-thread-core` (`docs/tickets/thread-ticket-01-thread-core.md`)
- Fitur: `thread-master-daemon`
- Rujukan kontrak bersama: `docs/plans/thread-ticket-01-thread-core/overview.md` (bagian 2 kontrak, 2.2 koreksi, 2.3 signature, 2.4 IPC, 7 aturan paralel). Dokumen ini mandiri untuk langkah kerja, tetapi signature final ada di overview 2.3 dan tidak boleh diubah.
- ownedPaths:
  - `internal/thread/manager.go`
  - `internal/thread/master.go`
  - `internal/thread/prompt.go`
  - `internal/thread/ipc.go`
  - `cmd/shikigamid/wire_thread.go`
- Bergantung pada Foundation: YA. `internal/thread/types.go` (final) dan `internal/thread/store.go` (stub dengan signature final, nanti diganti implementasi penuh oleh fitur `thread-foundation`) sudah ada sebelum plan ini dikerjakan. Tidak bergantung pada `thread-ui` maupun isi `thread-foundation` selain signature `Store`.

## Tujuan

Menyediakan sisi daemon thread: `Manager` (hook, `ToolRegistry` kerangka, operasi pesan/peserta/thread), spawn sesi master (provider agent, auto-approve, cwd netral, prompt awal lewat antrean saat idle), prompt master, metode IPC `thread.*` plus `RegisterMethod`, event `thread.updated`/`thread.message`, dan modul daemon `thread` di `wire_thread.go`. Kegagalan spawn master tidak membuat panic dan tercatat sebagai pesan sistem; thread tetap dibuat.

## Path yang dimiliki

HANYA berkas berikut yang boleh dibuat/diubah:

- `internal/thread/manager.go`
- `internal/thread/master.go`
- `internal/thread/prompt.go`
- `internal/thread/ipc.go`
- `cmd/shikigamid/wire_thread.go`

Dilarang menyentuh: `internal/thread/types.go`, `internal/thread/store.go` (milik `thread-foundation`), `internal/mention/*`, `internal/session/*`, `internal/provider/*`, `cmd/shikigamid/main.go`, `modules.go`, `wire_mention.go`, semua berkas frontend (`frontend/...`), dan berkas thread tiket lain (`worktree.go`, `mcp*.go`, `spawn.go`, `router.go`, `antiloop.go`, `activity.go`, `close*.go`, `recovery.go`, `orphans.go`). Bila butuh perubahan di path lain, jangan ubah; catat di bagian "Catatan untuk wiring" di akhir plan ini (hasil kerja).

Catatan: pendaftaran modul lewat `init()` di `wire_thread.go` adalah bagian berkas ini dan memang dimiliki fitur ini (tidak perlu menyentuh `modules.go`).

## Kontrak

### Dipakai (sudah ada, jangan diubah)

Dari `internal/thread` (Foundation, `types.go`): `Thread`, `Participant`, `ThreadMessage`, konstanta `StateActive/StateClosed`, `RoleMaster/RolePeer`, `KindMessage/KindSystem/KindActivity`, `FromUser/FromSystem`, `ToUser/ToAll`, `MethodCreate/List/Get/Messages/Send/OwnedSessions`, `EventUpdated/EventMessage`, `CreateParams`, `ListParams`, `GetParams`, `MessagesParams`, `SendParams`, `OwnedSessionsResult`, `NewID()`.

Dari `store.go` (signature final): `Store`, `ErrNotFound`, `ErrInvalid`, `NewStore`, `(*Store).Create(workspaceID, title, goal)`, `Get`, `List`, `Save`, `AppendMessage`, `Messages(id, sinceTS time.Time)`, `ThreadDir`, `MasterDir`.

Dari kode yang ada (diverifikasi saat plan ditulis):

- `internal/session`: `Manager.Create(SessionSpec{WorkspaceID, ProjectID, Cwd, ProviderID, Cols, Rows, Env})`, `Get`, `Kill`, `List`; `Session{ID, WorkspaceID, ProjectID, Status, ...}`, `session.StatusRunning`. `*session.Manager` memenuhi `SessionAPI` (2.3) dan `mention.SessionSource`.
- `internal/provider`: `provider.Get(id)`, `provider.IDClaude`, `provider.IDCodex`, `Provider.Available() (path, err)`, `Provider.ID()`. Flag auto-approve sudah diterapkan otomatis oleh `provider/claude.go` (`withFlag(... claudeSkipPermissionsFlag)`) dan `provider/codex.go` (`codexBypassFlag`) lewat `Launcher.Command` untuk setiap sesi ber-provider itu. Maka cukup memakai `ProviderID` claude/codex; JANGAN menambah flag sendiri dan jangan menduplikasi.
- `internal/mention`: `mention.NewQueue(mention.QueueConfig{Sessions, Namer, Publish, Logger})`, `(*Queue).Enqueue(ctx, from, to, text, source)`, `(*Queue).SetMode(mention.ModeAuto)`, `(*Queue).HandleExit(sessionID)`, `mention.SourceUser`, `mention.SourceAgent`, `mention.ErrNotFound`, `mention.NewNamer(src, dir, statusOf)`, `mention.Directory` (`ProjectName(workspaceID, projectID string) string`), `mention.NewMessageID()`. Perilaku penting `Enqueue`: `from == ""` wajib `source == SourceUser`; `from != ""` wajib `SourceAgent`; target harus sesi running (kalau tidak `ErrNotFound`); teks dikirim ke sesi sebagai `"[pesan dari <FromName>] <teks>"` saat target idle.
- `internal/ipc`: `Server.Register(method string, h ipc.Handler)` dengan `type Handler func(ctx context.Context, params json.RawMessage) (any, error)`, `Server.Publish(topic string, payload any)`, `ipc.NewError(code, msg)`, `ipc.Errorf`, kode `ipc.ErrNotFound`, `ipc.ErrInvalid`, `ipc.ErrInvalidParams`, `ipc.ErrInternal`, `ipc.Dial(socket)`, `Client.Call`, `Client.Subscribe`.
- `cmd/shikigamid`: `RegisterModule(name, func(srv *ipc.Server, d *Deps))`, `Deps{DataDir, SocketPath, Logger}`, `d.Provide`, `d.Lookup`, `d.OnReady`. `internal/paths.DataDir()`. `internal/status`: `status.Default()`, `status.IsIdle(id)`, `Engine.Get(id)`.
- Workspace IPC internal `workspace.list`: balikan array `[]Workspace` atau objek `{workspaces:[...]}`; `Workspace{id, name, projects:[{id, path, name}]}`.

### Disediakan (signature persis seperti overview 2.3)

`manager.go`: `ProjectInfo`, `WorkspaceSource`, `Publisher`, `Sender`, `SessionAPI`, `ManagerConfig`, `NewManager`, `ErrClosed`, `Manager` dengan metode `Create`, `Get`, `List`, `Messages`, `Post`, `AppendMessage`, `PostSystem`, `Send`, `AddParticipant`, `UpdateThread`, `SendToSession`, `OwnedSessions`, `OnStart`, `OnMessage`, `OnParticipantAdded`, `OnClose`, `FireStart`, `FireClose`, `Tools`, `Store`; `ThreadCaller`, `ToolSpec`, `ToolRegistry` (`Register`, `Tools`).
`master.go`: `(*Manager).SpawnMaster(t Thread) (Participant, error)`, `resolveMasterProvider() (provider.Provider, error)`.
`prompt.go`: `BuildMasterPrompt(t Thread, projects []ProjectInfo) string`, `var ToolsSection = func() string {...}`.
`ipc.go`: `RegisterIPC(srv *ipc.Server, m *Manager)`, `(*Manager).RegisterMethod(name string, handler func(params json.RawMessage) (any, error))`.
`wire_thread.go`: modul `"thread"`, `wireThread`, adapter `threadWorkspaceDirectory`, `d.Provide("thread.manager", mgr)`.

Perilaku IPC mengikuti overview 2.4 tanpa perubahan. Koreksi overview 2.2 no. 3, 4, 5, 6, 7, 8 WAJIB diikuti.

## Langkah

Semua komentar, log, galat, dan teks bahasa Indonesia; identifier bahasa Inggris. Logger nullable: simpan `cfg.Logger` dan bila nil pakai `slog.Default()` di `NewManager`. Helper tak-diekspor diberi awalan `manager*`, `master*`, `prompt*`, `ipc*` (di package `main`: `threadWire*`). Tanpa dependency baru, tanpa unit test.

### Langkah 1: `internal/thread/manager.go`

1. Impor: `context`, `errors`, `fmt`, `log/slog`, `os`, `path/filepath`, `sort`, `strings`, `sync`, `time`, `encoding/json`, `github.com/NooBeeID/shikigami/internal/ipc`, `.../internal/mention`, `.../internal/session`.
2. Tulis tipe-tipe antarmuka dan konfigurasi persis seperti overview 2.3: `ProjectInfo`, `WorkspaceSource`, `Publisher`, `Sender`, `SessionAPI`, `ManagerConfig`. Tambah `var ErrClosed = errors.New("thread sudah ditutup")`. Tambah assertion kompilasi: `var _ SessionAPI = (*session.Manager)(nil)` dan `var _ Sender = (*mention.Queue)(nil)`.
3. Tipe tool: `ThreadCaller`, `ToolSpec`, `ToolRegistry` persis overview. `ToolRegistry{mu sync.Mutex; tools map[string]ToolSpec}`. `Register(name, tool)`: panic `"thread: nama tool kosong"` bila `strings.TrimSpace(name)==""`, panic `fmt.Sprintf("thread: tool %q sudah terdaftar", name)` bila duplikat; simpan dengan `tool.Name = name`. `Tools()` mengembalikan salinan terurut nama (selalu slice non-nil). Dokumentasikan di komentar bahwa tiket thread-agents memakainya.
4. Struct `Manager`:
   ```go
   type Manager struct {
       cfg   ManagerConfig
       tools *ToolRegistry
       log   *slog.Logger

       hookMu             sync.Mutex
       onStart            []func(*Manager)
       onMessage          []func(Thread, ThreadMessage)
       onParticipantAdded []func(Thread, Participant)
       onClose            []func(Thread)

       lockMu  sync.Mutex
       locks   map[string]*sync.Mutex // mutex per thread untuk baca-ubah-simpan

       methodMu sync.Mutex             // dipakai ipc.go (RegisterMethod)
       methods  map[string]func(json.RawMessage) (any, error)
       srv      *ipc.Server
   }
   ```
   `NewManager(cfg)`: isi logger default, `tools: &ToolRegistry{tools: map...}`, `locks`, `methods` map. `Tools()` dan `Store()` (kembalikan `cfg.Store`) trivial.
5. Helper `managerLock(id string) *sync.Mutex` (buat bila belum ada, di bawah `lockMu`).
6. Helper `managerPublish(topic string, payload any)`: no-op bila `cfg.Publish == nil`. Helper `managerNormalize(t Thread) Thread`: pastikan `Participants` non-nil (`[]Participant{}`). Helper `managerPublishThread(t Thread)` memublikasikan `EventUpdated` dengan `managerNormalize(t)`.
7. Hook (didokumentasikan di komentar: boleh didaftarkan banyak pihak, dipanggil berurutan pendaftaran di goroutine pemanggil, panic dipulihkan dan dicatat):
   - `OnStart/OnMessage/OnParticipantAdded/OnClose` menambah ke slice di bawah `hookMu`.
   - Helper generik `managerSafe(name string, fn func())` dengan `defer recover()` yang `m.log.Error("thread: hook panic", "hook", name, "panic", r)`.
   - `FireStart()`: salin slice di bawah lock, jalankan tiap hook lewat `managerSafe`. `FireClose(t Thread)` serupa untuk `onClose`. Helper tak-diekspor `managerFireMessage(t, msg)` dan `managerFireParticipant(t, p)`.
8. Operasi baca: `Get(id)` -> `cfg.Store.Get(id)` lalu `managerNormalize`. `List(workspaceID)` -> `cfg.Store.List` (normalisasi tiap thread; kembalikan `[]Thread{}` bila nil). `Messages(id, sinceTS)` -> `cfg.Store.Messages` (kembalikan `[]ThreadMessage{}` bila nil); sebelum itu pastikan thread ada (`Get`) agar id tak dikenal menghasilkan `ErrNotFound`.
9. `Post(threadID, msg)`:
   - `t, err := m.Get(threadID)`; galat dikembalikan apa adanya. `t.State == StateClosed` -> `return ThreadMessage{}, ErrClosed`.
   - Isi: `msg.ThreadID = threadID` (selalu), `ID` bila kosong `NewID()`, `TS` bila nol `time.Now().UTC()` (selain itu `.UTC()`), `Kind` bila kosong `KindMessage`, `To` bila kosong `ToAll`, `From` bila kosong `FromSystem`.
   - `cfg.Store.AppendMessage(msg)`; galat dikembalikan.
   - `managerPublish(EventMessage, msg)`, lalu `managerFireMessage(t, msg)`.
   - Kembalikan `msg`.
   `AppendMessage(threadID, msg)` = `return m.Post(threadID, msg)` (alias, dokumentasikan).
10. `PostSystem(threadID, text string)`: bentuk `ThreadMessage{From: FromSystem, To: ToAll, Text: text, Kind: KindSystem}`. Tidak boleh menolak thread closed dan tidak lewat `Post`: ambil thread via `Get` (galat -> log `Warn`, return), isi ID/TS/ThreadID, `AppendMessage` ke store (galat -> log `Warn`, return), publish `EventMessage`, panggil `managerFireMessage`. Tidak mengembalikan galat.
11. `UpdateThread(threadID, fn)`: kunci `managerLock(threadID)`; `t, err := Store.Get`; salin `t.ID` sebagai `id0`; panggil `fn(&t)`; paksa `t.ID = id0` (fn tidak boleh mengubah ID); normalisasi `Participants`; `Store.Save(t)`; unlock; `managerPublishThread(t)`; kembalikan `t`. Jangan memanggil hook di bawah lock.
12. `AddParticipant(threadID, p)`:
    - validasi: `p.SessionID` kosong atau `p.Role` bukan `RoleMaster`/`RolePeer` -> `fmt.Errorf("%w: peserta tidak valid", ErrInvalid)`.
    - `UpdateThread` dengan fn yang: tolak (set variabel galat `ErrClosed` di closure) bila `State == StateClosed` — karena `UpdateThread` tidak punya jalur galat dari fn, cek dulu `Get` dan `ErrClosed` sebelum memanggil `UpdateThread`; ganti peserta bila `SessionID` sama (idempoten), selain itu append.
    - Setelah sukses panggil `managerFireParticipant(t, p)` dan kembalikan `t`.
13. `SendToSession(threadID, from, to, text)`: `strings.TrimSpace(text)==""` -> `ErrInvalid`; pastikan thread ada dan `active` (`ErrNotFound`/`ErrClosed`); `source := mention.SourceUser` bila `from == ""` selain itu `mention.SourceAgent`; kembalikan `m.cfg.Sender.Enqueue(context.Background(), from, to, text, source)`. Galat `Sender == nil` -> `errors.New("antrean pesan thread tidak tersedia")`. Dokumentasikan: pencatatan ke log thread dilakukan pemanggil (`Send`, router tiket thread-agents), bukan di sini.
14. `Send(threadID, text)` (jalur `thread.send`, koreksi overview 2.2 no. 8):
    - `strings.TrimSpace(text)==""` -> `fmt.Errorf("%w: teks kosong", ErrInvalid)`.
    - `t, err := m.Get(threadID)`; `State == StateClosed` -> `ErrClosed`.
    - Cari `Participant` ber-`Role == RoleMaster` (helper `managerMaster(t) (Participant, bool)`). `to := ToAll` bila tidak ada, selain itu `master.SessionID`.
    - `rec, err := m.Post(threadID, ThreadMessage{From: FromUser, To: to, Text: text, Kind: KindMessage})`; galat dikembalikan.
    - Bila tidak ada master: `m.PostSystem(threadID, "gagal meneruskan ke master: master belum ada")`; kembalikan `rec, nil`.
    - Kalau ada: `_, err = m.SendToSession(threadID, "", master.SessionID, text)`; bila `err != nil` (termasuk `mention.ErrNotFound` = sesi master mati): `m.PostSystem(threadID, "gagal meneruskan ke master: "+err.Error())`. Kembalikan `rec, nil` (galat pengiriman bukan galat `Send`).
15. `OwnedSessions() []string`: `managerAllThreads()` lalu kumpulkan `Participant.SessionID` (tidak kosong) dari thread ber-state `active` saja; hasil terurut unik; selalu slice non-nil. `managerAllThreads() []Thread` TIDAK memakai `Store.List("")` (semantik workspace kosong tidak dijamin kontrak): baca `filepath.Dir(m.cfg.Store.ThreadDir("x"))` dengan `os.ReadDir`, untuk tiap entri direktori panggil `m.cfg.Store.Get(entry.Name())`, lewati galat (log `Debug`). Galat `ReadDir` (mis. direktori belum ada) -> kembalikan nil.
16. `deriveTitle(goal string) string` (tak diekspor, awalan `manager`: `managerDeriveTitle`): baris pertama non-kosong dari goal, `TrimSpace`, potong 60 rune; bila terpotong tambahkan `…`. Bila hasil kosong -> `"Thread baru"`.
17. `Create(workspaceID, goal, title)`:
    - `goal = strings.TrimSpace(goal)`; kosong -> `fmt.Errorf("%w: goal tidak boleh kosong", ErrInvalid)`. `workspaceID` kosong -> `ErrInvalid`.
    - `m.cfg.Workspaces == nil` atau `projects, ok := Workspaces.Projects(workspaceID)`; `!ok` -> `fmt.Errorf("%w: workspace tidak dikenal", ErrInvalid)`. Simpan `projects` untuk prompt (SpawnMaster mengambil ulang sendiri, jadi cukup validasi di sini).
    - `title = strings.TrimSpace(title)`; kosong -> `managerDeriveTitle(goal)`.
    - `t, err := Store.Create(workspaceID, title, goal)`; galat dikembalikan (dibungkus `fmt.Errorf("buat thread: %w", err)` dengan `%w`).
    - `managerPublishThread(t)` (agar UI langsung melihat thread).
    - `p, err := m.SpawnMaster(t)`. Gagal: `m.PostSystem(t.ID, "gagal membuat master: "+err.Error())` dan lanjut tanpa mengembalikan galat. Sukses: `m.PostSystem(t.ID, "master dibuat")`. `SpawnMaster` sudah menyimpan Participant lewat `AddParticipant`, jadi jangan memanggil `AddParticipant` lagi di sini (parameter `p` hanya untuk log).
    - Bungkus pemanggilan `SpawnMaster` dalam fungsi dengan `defer recover()` yang mengubah panic menjadi galat ("panic saat membuat master: ...") sehingga spawn tidak pernah mematikan daemon.
    - Akhir: `t2, err := m.Get(t.ID)`; bila err, kembalikan `t` (nilai awal). Kembalikan `t2, nil`.

### Langkah 2: `internal/thread/master.go`

1. Impor: `errors`, `fmt`, `os`, `time`, `context`, `.../internal/provider`, `.../internal/session`, `.../internal/mention`.
2. `resolveMasterProvider() (provider.Provider, error)` (koreksi overview 2.2 no. 3): untuk `id` dalam urutan `[]string{provider.IDClaude, provider.IDCodex}`: `p, ok := provider.Get(id)`; `!ok` lewati; `if _, err := p.Available(); err == nil { return p, nil }`. Tak ada: `errors.New("tidak ada provider agent (claude/codex) yang tersedia")`. JANGAN memakai `provider.Default()` (itu shell).
3. `(m *Manager) SpawnMaster(t Thread) (Participant, error)`:
   1. `p, err := resolveMasterProvider()`; galat dikembalikan (`Participant{}, err`).
   2. `dir := m.cfg.Store.MasterDir(t.ID)`; `os.MkdirAll(dir, 0o700)` (galat dibungkus "siapkan direktori master: %w").
   3. `s, err := m.cfg.Sessions.Create(session.SessionSpec{WorkspaceID: t.WorkspaceID, ProjectID: "", Cwd: dir, ProviderID: p.ID()})`; galat -> `fmt.Errorf("buat sesi master: %w", err)`. Auto-approve dijamin launcher (lihat Kontrak); tidak ada flag tambahan. Hanya sesi yang dibuat di sini yang boleh di-`Kill` kode thread.
   4. `part := Participant{SessionID: s.ID, ProjectID: "", Role: RoleMaster, Name: "master"}`. `AddParticipant(t.ID, part)`; gagal -> `_ = m.cfg.Sessions.Kill(s.ID)` (sesi milik sendiri) lalu kembalikan galat. Field `Namer` pada `ManagerConfig` tidak dipakai di sini (nama master tetap "master", sesuai tiket).
   5. Ambil proyek: `var projects []ProjectInfo; if m.cfg.Workspaces != nil { projects, _ = m.cfg.Workspaces.Projects(t.WorkspaceID) }`. `prompt := BuildMasterPrompt(t, projects)`.
   6. Luncurkan goroutine `go m.masterSendInitialPrompt(t.ID, s.ID, prompt)` (tidak memblokir `Create`; tangkap panic dengan `defer recover()` + log).
   7. Kembalikan `part, nil`.
4. `(m *Manager) masterSendInitialPrompt(threadID, sessionID, prompt string)`:
   - Kirim lewat `m.SendToSession(threadID, "", sessionID, prompt)`. `Queue` menunda pengiriman sampai sesi idle (`status.OnIdle`), jadi tidak mengganggu sesi lain. Teks akan berawalan "[pesan dari user] " (koreksi overview 2.2 no. 4; prompt di `prompt.go` sudah disusun wajar dengan awalan itu).
   - Sesi yang baru dibuat mungkin belum dikenal mesin status; bila `Enqueue` mengembalikan `mention.ErrNotFound`, coba ulang hingga 5 kali dengan jeda 300 ms (`time.Sleep`). Galat lain tidak diulang.
   - Gagal akhir: `m.PostSystem(threadID, "gagal mengirim prompt awal ke master: "+err.Error())`. Sukses: `m.PostSystem(threadID, "prompt awal dikirim ke master")` (ringkas; jangan mencatat seluruh prompt).

### Langkah 3: `internal/thread/prompt.go`

1. Impor `fmt`, `strings`.
2. `var ToolsSection = func() string { return "Tool thread (untuk men-spawn peserta, mengirim pesan antar peserta, dan mengusulkan penutupan) segera tersedia; sampai saat itu kamu belum bisa memanggilnya, jadi balas user langsung di terminal ini." }`. Komentar: tiket thread-agents mengganti variabel ini; mengembalikan string kosong menghilangkan bagian tersebut.
3. `BuildMasterPrompt(t Thread, projects []ProjectInfo) string`: bahasa Indonesia, ringkas tapi lengkap, tanpa jargon implementasi, dibangun dengan `strings.Builder`. Karena antrean menambahkan awalan "[pesan dari user] ", kalimat pertama harus wajar setelah awalan itu (mis. "Ini briefing awal untukmu."). Isi berurutan:
   - Pembuka: "Ini briefing awal untukmu. Kamu adalah master dari sebuah thread, yaitu diskusi beberapa agent lintas project dalam satu workspace untuk mencapai satu goal."
   - Judul: `Judul thread: <t.Title>`.
   - Goal verbatim: baris `Goal dari user:` lalu `t.Goal` apa adanya (tanpa dimodifikasi).
   - Daftar project: bila kosong, "Workspace ini belum punya project terdaftar."; selain itu tiap baris `- <Name> (id: <ID>) path: <Path>`.
   - Peran: merutekan pekerjaan, memutuskan project mana yang perlu dilibatkan, meneruskan instruksi user ke agent yang sesuai, mengusulkan penutupan saat goal tercapai; TIDAK menutup thread sendiri (hanya user yang menutup).
   - Aturan: pesan user datang lewat thread dan akan tiba di terminal ini; kamu bekerja di direktori netral milik thread dan TIDAK mengedit file project mana pun; jangan menyentuh sesi lain milik user; jawab singkat dan jelas.
   - `ToolsSection()` bila hasilnya tidak kosong (`strings.TrimSpace`), dipisahkan baris kosong.
   - Penutup: "Mulailah dengan memahami goal, lalu sampaikan rencana singkatmu ke user."
   Helper tak-diekspor bila perlu diberi awalan `prompt`.

### Langkah 4: `internal/thread/ipc.go`

1. Impor: `context`, `encoding/json`, `errors`, `fmt`, `strings`, `time`, `.../internal/ipc`.
2. `ipcDecode(raw json.RawMessage, v any) error`: kosong atau `"null"` -> nil; unmarshal gagal -> `ipc.Errorf(ipc.ErrInvalidParams, "params tidak valid: %v", err)`.
3. `ipcToError(err error) error` (pola `internal/mention/ipc.go`): nil -> nil; sudah `*ipc.Error` (`errors.As`) -> kembalikan; `errors.Is(err, ErrNotFound)` -> `ipc.NewError(ipc.ErrNotFound, err.Error())`; `errors.Is(err, ErrInvalid)` -> `ipc.NewError(ipc.ErrInvalid, err.Error())`; `errors.Is(err, ErrClosed)` -> `ipc.NewError(ipc.ErrInvalid, "thread sudah ditutup")`; selain itu `ipc.NewError(ipc.ErrInternal, err.Error())`.
4. `(m *Manager) RegisterMethod(name string, handler func(params json.RawMessage) (any, error))`:
   - Tolak nama kosong atau handler nil (panic dengan pesan jelas, sama gaya `ToolRegistry.Register`) dan duplikat (panic).
   - Di bawah `methodMu`: simpan di `m.methods[name]`; bila `m.srv != nil` langsung `m.srv.Register(name, ipcAdapt(handler))`. Bila `RegisterIPC` belum dipanggil, hanya tersimpan.
   - `ipcAdapt(h) ipc.Handler` = `func(ctx context.Context, raw json.RawMessage) (any, error) { v, err := h(raw); return v, ipcToError(err) }`.
   - Dokumentasikan di komentar: dipakai tiket thread-close dkk. untuk `thread.closePreview`/`thread.close`.
5. `RegisterIPC(srv *ipc.Server, m *Manager)`:
   - Di bawah `methodMu`: `m.srv = srv`; daftarkan semua `m.methods` yang sudah tersimpan (`srv.Register(name, ipcAdapt(h))`).
   - Daftarkan metode bawaan (pakai `srv.Register(Method..., ...)` langsung, handler memakai `ipcDecode` dan `ipcToError`):
     - `MethodCreate`: decode `CreateParams`; `t, err := m.Create(p.WorkspaceID, p.Goal, p.Title)`; kembalikan `t`. Goal kosong/workspace tak dikenal sudah menjadi `ErrInvalid` dari `Create`.
     - `MethodList`: decode `ListParams`; `list, err := m.List(p.WorkspaceID)`; kembalikan array langsung (`[]Thread{}` bila kosong).
     - `MethodGet`: decode `GetParams`; `id` kosong -> `ErrInvalid`; `m.Get`.
     - `MethodMessages`: decode `MessagesParams`; `SinceTS` tidak kosong -> `time.Parse(time.RFC3339Nano, ...)` (RFC3339 juga lolos oleh layout ini); gagal -> `ErrInvalid` dengan pesan "sinceTs tidak valid"; kembalikan array langsung.
     - `MethodSend`: decode `SendParams`; `m.Send(p.ID, p.Text)`; kembalikan `ThreadMessage`. Thread closed -> `ErrInvalid` "thread sudah ditutup"; teks kosong -> `ErrInvalid`.
     - `MethodOwnedSessions`: kembalikan `OwnedSessionsResult{SessionIDs: m.OwnedSessions()}` (tag json `sessionIds`; pastikan non-nil).
   - Tambahan: bila `RegisterIPC` dipanggil dua kali pada server berbeda tidak didukung; cukup dokumentasikan bahwa dipanggil sekali.
6. Event: tidak ada kode tambahan; publikasi `thread.updated`/`thread.message` dilakukan `Manager` lewat `ManagerConfig.Publish` (diisi `srv.Publish` di wiring).

### Langkah 5: `cmd/shikigamid/wire_thread.go`

Package `main`. Impor: `context` (bila perlu), `encoding/json`, `path/filepath`, `sync`, `time`, `.../internal/ipc`, `.../internal/mention`, `.../internal/paths`, `.../internal/session`, `.../internal/status`, `.../internal/thread`.

1. Registrasi:
   ```go
   func init() {
       RegisterModule("thread", func(srv *ipc.Server, d *Deps) {
           d.OnReady(func() { wireThread(srv, d) })
       })
   }
   ```
2. Adapter `threadWorkspaceDirectory` (koreksi overview 2.2 no. 5), meniru `workspaceDirectory` di `wire_mention.go` tetapi ditulis ULANG (jangan memakai atau mengedit tipe itu): struct dengan `socket string`, `mu sync.Mutex`, `client *ipc.Client`, `cache map[string][]thread.ProjectInfo` (kunci workspaceId), `fetchedAt time.Time`.
   - `refreshLocked() bool`: dial `socket` bila `client == nil`; `Call("workspace.list", struct{}{}, &raw)`; gagal -> reset `client = nil` (agar dial ulang pada panggilan berikut) dan `false`; decode ganda (array `[]ws` atau objek `{workspaces:[...]}`) dengan `ws{ID, Projects []struct{ID, Name, Path string}}` (tag json `id`, `projects`, `name`, `path`); isi `cache` dan `fetchedAt`.
   - `Projects(workspaceID string) ([]thread.ProjectInfo, bool)`: kunci mutex; refresh bila `cache == nil` atau lebih dari 5 detik; gagal refresh -> `nil, false`; `ps, ok := cache[workspaceID]`; kembalikan salinan `ps` dan `ok` (workspace ada tetapi tanpa project -> `[]ProjectInfo{}`, `true`). Karena cache 5 detik, workspace yang baru dibuat bisa ditolak singkat; pada `!ok` dengan cache lama paksa satu kali refresh sebelum menyerah (cek `time.Since(fetchedAt) > 0` lalu refresh ulang sekali).
   - `ProjectName(workspaceID, projectID string) string`: pakai cache yang sama (cari `ID == projectID`); memenuhi `mention.Directory` untuk Namer.
3. `wireThread(srv *ipc.Server, d *Deps)`:
   1. `v, ok := d.Lookup("session.manager"); mgr, ok2 := v.(*session.Manager)`; bila tidak ada: `d.Logger.Warn("thread: session.manager tidak tersedia, modul dinonaktifkan")` dan `return` (tanpa panic).
   2. `dataDir := d.DataDir`; kosong -> `paths.DataDir()` (bila galat: log `Warn`, return). `store := thread.NewStore(filepath.Join(dataDir, "threads"))`.
   3. `dir := &threadWorkspaceDirectory{socket: d.SocketPath}`. `statusOf` sama gaya `wireMention` (pakai `status.Default()`/`status.IsIdle`). `namer := mention.NewNamer(mgr, dir, statusOf)`.
   4. Antrean KHUSUS thread (jangan mengambil/menyentuh antrean mention global): `q := mention.NewQueue(mention.QueueConfig{Sessions: mgr, Namer: namer, Publish: func(string, any) {}, Logger: d.Logger})`; `_ = q.SetMode(mention.ModeAuto)` (log `Warn` bila galat). `Publish` kosong supaya item thread tidak muncul di badge antrean mention.
   5. Langganan `session.exit` -> `q.HandleExit`, ditulis ulang di sini dengan pola `subscribeExit` (fungsi `threadWireSubscribeExit(d *Deps, onExit func(string))`: dial `d.SocketPath`, `c.Subscribe("session.exit", ...)` dengan payload `{id}`, retry sampai 5 kali setiap 200 ms via `time.AfterFunc`, `d.Logger.Warn` bila tetap gagal). Jangan memakai `exitHub` milik `wire_mention.go`.
   6. `m := thread.NewManager(thread.ManagerConfig{Store: store, Sessions: mgr, Sender: q, Workspaces: dir, Namer: namer, Publish: srv.Publish, Logger: d.Logger})`.
   7. `d.Provide("thread.manager", m)` (sekali; modul tiket berikut memakai `d.Lookup("thread.manager")` di dalam `d.OnReady`).
   8. `thread.RegisterIPC(srv, m)`.
   9. `m.FireStart()` (terakhir, setelah semua metode terdaftar).
   10. Seluruh `wireThread` memakai `defer recover()` yang mencatat `d.Logger.Error` agar kegagalan modul tidak menjatuhkan daemon.
4. Jangan mengubah `main.go`, `modules.go`, `wire_mention.go`.

### Langkah 6: verifikasi

1. `gofmt -l internal/thread cmd/shikigamid` tidak mencetak berkas milik fitur ini.
2. `go vet ./internal/thread/... ./cmd/shikigamid/...` (abaikan temuan di berkas milik fitur lain).
3. `go build ./...` lulus. Bila gagal karena berkas fitur lain yang belum selesai (mis. stub store), jangan mengubahnya; catat di "Catatan untuk wiring".
4. `git status` menunjukkan perubahan hanya pada lima berkas ownedPaths.

## Perubahan

Dibuat (semua berkas baru):

- `internal/thread/manager.go`: tipe antarmuka/konfigurasi, `Manager`, hook, `ToolRegistry`, operasi thread/pesan/peserta, `Create`, `Send`, `SendToSession`, `OwnedSessions`, `ErrClosed`.
- `internal/thread/master.go`: `resolveMasterProvider`, `SpawnMaster`, pengiriman prompt awal.
- `internal/thread/prompt.go`: `BuildMasterPrompt`, `ToolsSection`.
- `internal/thread/ipc.go`: `RegisterIPC`, `RegisterMethod`, pemetaan galat.
- `cmd/shikigamid/wire_thread.go`: registrasi modul `thread`, adapter workspace, antrean thread, langganan `session.exit`, `Provide`, `FireStart`.

Tidak ada berkas yang diubah.

## Wiring

Fitur ini sudah menanam semua yang perlu lewat `init()` di `wire_thread.go` (modul `"thread"` memanggil `wireThread` di `d.OnReady`), sehingga langkah wiring hanya MEMVERIFIKASI, bukan menyambung baru. Jangan menyambungkan apa pun di luar ownedPaths. Yang diekspos untuk dipakai langkah wiring dan tiket berikut:

- `d.Provide("thread.manager", *thread.Manager)`: tiket thread-agents/activity/close/recovery mengambilnya lewat `d.Lookup("thread.manager")` di dalam `d.OnReady`.
- `thread.RegisterIPC(srv, mgr)` dipanggil di `wireThread`; metode tambahan tiket lain memakai `mgr.RegisterMethod(name, handler)` (aman dipanggil sebelum maupun sesudah `RegisterIPC`).
- Hook: `OnStart` (dipicu `FireStart` di akhir `wireThread`), `OnMessage`, `OnParticipantAdded`, `OnClose` (dipicu `FireClose(t)` oleh tiket thread-close).
- `mgr.Tools()` mengembalikan `*ToolRegistry` untuk `Register(...)` oleh thread-agents; `thread.ToolsSection` dapat diganti thread-agents.
- Event publish: `thread.updated` (Thread penuh), `thread.message` (ThreadMessage) lewat `srv.Publish`.
- Verifikasi oleh agen wiring: `go build ./...`; daemon memuat modul `thread` (log `wire modul module=thread`); `thread.create` mengembalikan Thread dengan master terdaftar di `participants` dan pesan sistem; tanpa claude/codex di PATH hasilnya Thread dengan pesan sistem galat (bukan crash); `mention` global tetap `confirm_agent`.

## Selesai bila

- Lima berkas ownedPaths ada dan tidak ada berkas lain yang diubah (`git status`).
- `go build ./...` lulus (guard; tanpa unit test).
- `Create` menolak goal kosong dan workspace tak dikenal (`ErrInvalid`), membuat thread, men-spawn master dengan provider claude lalu codex yang tersedia (bukan `provider.Default()`), cwd `Store.MasterDir(id)`, `ProjectID` kosong, tanpa flag auto-approve tambahan; Participant master tersimpan; prompt awal dikirim via antrean saat idle; kegagalan spawn/prompt tercatat sebagai pesan sistem tanpa panic.
- `Send` mencatat pesan user ke log, publish `thread.message`, meneruskan ke master via antrean thread (mode `auto`, instance terpisah dari mention global), dan kegagalan penerusan menjadi pesan sistem; thread closed/teks kosong ditolak.
- Metode IPC `thread.create/list/get/messages/send/ownedSessions` sesuai overview 2.4; `RegisterMethod`, hook, `ToolRegistry`, dan event terdokumentasi di komentar.
- `internal/mention`, `main.go`, `modules.go`, `wire_mention.go`, dan sesi interaktif user tidak tersentuh (kode thread hanya me-`Kill` sesi yang dibuatnya sendiri).

## Catatan untuk wiring

(Diisi agen implementasi bila ada perubahan di luar ownedPaths yang dibutuhkan; kosong bila tidak ada.)
