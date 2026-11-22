# Plan: Spawn peserta, perutean pesan, dan anti-loop

## Metadata

- Tiket: `thread-ticket-02-thread-agents` (`docs/tickets/thread-ticket-02-thread-agents.md`)
- Fitur: `thread-spawn-router`
- Rujukan wajib: `docs/plans/thread-ticket-02-thread-agents/overview.md` (bagian 2.2 koreksi dan 2.3 signature adalah kontrak; jangan diubah), spec `docs/superpowers/specs/2026-11-22-thread-design.md`
- ownedPaths: `internal/thread/spawn.go`, `internal/thread/router.go`, `internal/thread/antiloop.go`
- Depends on foundation: ya. Foundation sudah menyediakan `agents_types.go` (`Router`, `ErrPaused`, `ErrTargetUnknown`, `ErrTargetAmbiguous`, `ErrNotParticipant`, `ParticipantView`), `worktree_records.go` (`WorktreeRecord`, `LoadWorktreeRecords`, `UpsertWorktreeRecord`, `DeleteWorktreeRecord`), stub `worktree.go` (`WorktreeInfo`, `CreateWorktree`, `RemoveWorktree`, `Slugify`, `ErrNotGitRepo`) dan stub `mcp_tools.go` (`Caller`, `ToolDef`, `RegisterTool`). Langkah 0 di bawah memeriksa keberadaannya.
- Tanpa dependency baru, tanpa unit test. Komentar, log, galat, pesan sistem, teks brief: bahasa Indonesia. Identifier: bahasa Inggris.

## Tujuan

Tiga berkas Go di paket `internal/thread`:

1. `spawn.go`: `Spawner` dan tool MCP `thread_spawn` (master saja) yang membuat worktree + sesi peer auto-approve, mendaftarkannya sebagai peserta, mengirim brief awal; dengan reuse peserta dan rollback penuh bila gagal.
2. `router.go`: `NewRouter` yang mengimplementasi `Router.Route`: validasi, anti-loop, resolusi target di dalam peserta thread, pencatatan ke log, pengiriman lewat antrean mention thread (ModeAuto) dengan awalan `[thread] `.
3. `antiloop.go`: `AntiLoop` yang menghitung pesan beruntun antar agent lewat hook `OnMessage`, menjeda thread di `MaxAgentChain` (8), menolak peer saat dijeda.

## Path yang dimiliki

HANYA berkas ini yang boleh dibuat/diubah:

- `internal/thread/spawn.go`
- `internal/thread/router.go`
- `internal/thread/antiloop.go`

Dilarang menyentuh: `worktree.go`, `mcp.go`, `mcp_tools.go`, `launch.go` (fitur lain, dikerjakan paralel oleh agent lain), `agents_types.go`, `worktree_records.go` (Foundation), `agents_install.go`, `wire_thread.go` (wiring), berkas tiket 01 (`manager.go`, `master.go`, `prompt.go`, `types.go`, `store.go`, `ipc.go`), `internal/mention`, `internal/provider`, `internal/session`, frontend. Butuh perubahan di luar itu: jangan dikerjakan, catat di "Wiring".

Helper tak-diekspor wajib berawalan berkas: `spawn*`, `router*`, `antiloop*` (hindari bentrok dengan agent paralel). Tipe/konstanta hanya yang disebut di Kontrak; jangan menambah ekspor baru yang dipakai fitur lain.

## Kontrak

### Dipakai (sudah ada di kode tiket 01; nama sudah diverifikasi terhadap kode nyata)

Dari `manager.go` / `types.go` / `store.go`:

- `*Manager`: `Get(id) (Thread, error)`, `Post(threadID, ThreadMessage) (ThreadMessage, error)` (menolak thread closed dengan `ErrClosed`, mengisi ID/TS/Kind/To/From default, memicu hook `OnMessage`), `PostSystem(threadID, text)` (tidak mengembalikan galat, memicu hook `OnMessage` juga), `AddParticipant(threadID, Participant) (Thread, error)`, `UpdateThread(threadID, func(*Thread)) (Thread, error)`, `SendToSession(threadID, from, to, text) (mention.Message, error)` (`from` "" = user, selain itu sessionId; sumber `SourceAgent` bila from tak kosong), `OnMessage(func(Thread, ThreadMessage))`, `OnClose(func(Thread))`, `Store() *Store` (`ThreadDir(id)`).
- Field privat `Manager.cfg` (`ManagerConfig`: `Namer *mention.Namer` boleh nil, `Sessions`, `Workspaces`) boleh dibaca dari berkas dalam paket yang sama; Spawner memakai `SpawnerConfig` untuk `Sessions`/`Workspaces` dan `mgr.cfg.Namer` untuk nama.
- Tipe: `Thread`, `Participant{SessionID, ProjectID, Role, Name}`, `ThreadMessage{ID, ThreadID, From, To, Text, TS, Kind}`; konstanta `StateActive`, `StateClosed`, `RoleMaster`, `RolePeer`, `KindMessage`, `KindSystem`, `FromUser` ("user"), `FromSystem`, `ToUser`, `ToAll`; galat `ErrClosed`, `ErrInvalid`; `SessionAPI` (`Create/Get/Kill/List`), `WorkspaceSource.Projects(workspaceID) ([]ProjectInfo, bool)`, `ProjectInfo{ID, Name, Path}`.
- `internal/session`: `session.SessionSpec{WorkspaceID, ProjectID, Cwd, ProviderID}`, `session.Session` (`ID`, `Status`, `ProviderID`), `session.StatusRunning`.
- `internal/provider`: `provider.IDClaude`, `provider.IDCodex`, `provider.Get(id) (Provider, bool)`, `Provider.Available()`, `Provider.Capabilities().MCP`, `Provider.ID()`.
- `internal/mention`: hanya galat `mention.ErrNotFound` bila perlu dikenali (tidak wajib).

Dari Foundation (overview 2.3): `Router`, `ErrPaused`, `ErrTargetUnknown`, `ErrTargetAmbiguous`, `ErrNotParticipant`; `WorktreeInfo`, `CreateWorktree(projectPath, dataDir, threadID, projectSlug)`, `RemoveWorktree(info)`, `Slugify(name)`, `ErrNotGitRepo`; `WorktreeRecord`, `UpsertWorktreeRecord(threadDir, rec)`, `DeleteWorktreeRecord(threadDir, projectID)`; `Caller{ThreadID, SessionID, Master}`, `ToolDef`, `RegisterTool(d)`.

### Disediakan (signature FINAL, persis overview 2.3; jangan diubah)

```go
// spawn.go
type SpawnerConfig struct {
    Manager    *Manager
    Sessions   SessionAPI
    Workspaces WorkspaceSource
    Router     Router
    DataDir    string
    Logger     *slog.Logger
}
type Spawner struct{ /* tak diekspor */ }
func NewSpawner(cfg SpawnerConfig) *Spawner
func (sp *Spawner) RegisterTool() // RegisterTool(ToolDef{Name:"thread_spawn", MasterOnly:true,...}); sekali
func BuildPeerBrief(t Thread, projectName, brief string, wt WorktreeInfo) string

// router.go
type RouterConfig struct {
    Manager *Manager
    Guard   *AntiLoop
    Logger  *slog.Logger
}
func NewRouter(cfg RouterConfig) Router

// antiloop.go
const MaxAgentChain = 8
type AntiLoop struct{ /* tak diekspor */ }
func NewAntiLoop(m *Manager) *AntiLoop   // mendaftarkan sendiri m.OnMessage dan m.OnClose
func (a *AntiLoop) Paused(threadID string) bool
func (a *AntiLoop) Check(t Thread, fromSessionID string) error // ErrPaused bila dijeda dan pengirim peer
func (a *AntiLoop) Reset(threadID string)
```

Tambahan internal (tak diekspor, di `spawn.go`, bukan kontrak lintas fitur): metode `(sp *Spawner) Spawn(ctx, threadID, project, brief string) (SpawnResult, error)` BOLEH diekspor sebagai metode tipe `Spawner` untuk dipakai handler tool; `SpawnResult` didefinisikan di `spawn.go`.

### Pesan/teks persis

- Pesan sistem spawn: `master men-spawn <nama project>`.
- Pesan sistem gagal spawn: `gagal men-spawn <nama project>: <alasan>`.
- Pesan sistem jeda (persis): `Thread dijeda: pesan beruntun antar agent mencapai batas. Kirim pesan untuk melanjutkan.`
- Pesan sistem lanjut: `Thread dilanjutkan.`
- Pesan sistem gagal forward: `gagal meneruskan ke <nama>: <alasan>`.
- Awalan injeksi router: `[thread] ` (antrean mention menambah `[pesan dari <nama>] ` di depannya).
- Galat provider: `tidak ada provider agent dengan dukungan MCP (claude) yang tersedia untuk peserta thread`.
- Galat reuse mati: `agen project <nama> sudah berhenti`.

## Langkah

### Langkah 0: verifikasi prasyarat

1. Pastikan ada: `internal/thread/{agents_types,worktree_records,worktree,mcp_tools}.go` dan simbol Foundation di atas. Jalankan `go build ./...` (harus lulus sebelum mulai). Bila `agents_types.go` atau stub belum ada (Foundation belum selesai), berhenti dan laporkan; jangan membuatnya (bukan milik fitur ini).
2. Baca ulang `manager.go` bahwa signature di bagian "Dipakai" cocok. Bila beda, ikuti kode dan catat di "Wiring".

### Langkah 1: `internal/thread/antiloop.go`

Implementasi (aturan overview 2.2 no. 6):

1. Konstanta `MaxAgentChain = 8`; konstanta tak diekspor `antiloopPauseText` (teks jeda persis) dan `antiloopResumeText`.
2. `type AntiLoop struct { mu sync.Mutex; state map[string]*antiloopState; mgr *Manager }`, `antiloopState{count int; paused bool}`.
3. `NewAntiLoop(m *Manager) *AntiLoop`: buat instance, lalu `m.OnMessage(a.antiloopOnMessage)` dan `m.OnClose(a.antiloopOnClose)`. Fungsi mendaftarkan sendiri; pemanggil tidak perlu memanggil apa pun.
4. `antiloopOnMessage(t Thread, msg ThreadMessage)`:
   - abaikan bila `msg.Kind != KindMessage`;
   - tentukan peran pengirim: `msg.From == FromUser` -> reset; selain itu cari `Participant` dengan `SessionID == msg.From` di `t.Participants`; `Role == RoleMaster` -> reset; `Role == RolePeer` dan `msg.To != ToUser` -> tambah; selain itu (from tak dikenal, mis. system) abaikan;
   - reset: `count = 0`; bila sebelumnya `paused`, set `paused=false` dan catat `PostSystem(t.ID, antiloopResumeText)` di goroutine terpisah (`go`), karena hook dipanggil dari dalam `Post` dan PostSystem memicu hook lagi;
   - tambah: `count++`; bila `count >= MaxAgentChain` dan belum `paused`: set `paused=true`, lalu `go a.mgr.PostSystem(t.ID, antiloopPauseText)`. Pesan sistem tidak mengubah hitungan karena kind-nya system.
   - Jangan memegang `a.mu` saat memanggil `PostSystem` (kunci dilepas dulu; panggil di goroutine).
5. `antiloopOnClose(t Thread)`: hapus `state[t.ID]`.
6. `Paused(threadID) bool`: baca di bawah mutex.
7. `Check(t Thread, fromSessionID string) error`: bila `fromSessionID` adalah peserta `RolePeer` di `t.Participants` dan thread dijeda -> `ErrPaused`; selain itu (user, master, bukan peserta) nil. Tidak mengubah hitungan.
8. `Reset(threadID)`: hitungan 0, `paused=false` (tanpa pesan sistem; dipakai pemanggil lain bila perlu).
9. Logger: pakai `m.log` bila perlu; `slog.Default()` jika nil.

### Langkah 2: `internal/thread/router.go`

1. `type routerImpl struct{ mgr *Manager; guard *AntiLoop; log *slog.Logger }`; `NewRouter(cfg RouterConfig) Router`. `cfg.Manager` nil -> panic pesan jelas; `cfg.Guard` boleh nil (lewati cek); logger nil -> `slog.Default()`. Tambah `var _ Router = (*routerImpl)(nil)`.
2. `Route(ctx, threadID, from, to, text) (ThreadMessage, error)` berurutan:
   1. `text = strings.TrimSpace(text)`; kosong -> `fmt.Errorf("%w: teks kosong", ErrInvalid)`.
   2. `t, err := mgr.Get(threadID)`; `t.State != StateActive` -> `ErrClosed`.
   3. Identifikasi pengirim: `from == FromUser` diizinkan (kirim sebagai user); selain itu harus cocok dengan `Participant.SessionID` (via `routerFindParticipant(t, from)`), kalau tidak `ErrNotParticipant`.
   4. `Guard.Check(t, from)`; galat (ErrPaused) dikembalikan SEBELUM apa pun dicatat/dikirim.
   5. Resolusi target `routerResolveTargets(t, from, to) (targets []Participant, logTo string, err error)`:
      - normalisasi `to`: trim, lower-case, buang awalan `@`; kosong -> `ErrTargetUnknown` dibungkus pesan "target kosong";
      - `user` -> targets kosong, `logTo = ToUser`;
      - `all` -> semua peserta kecuali pengirim (kosong -> galat "tidak ada peserta lain"), `logTo = ToAll`;
      - `master` -> peserta role master;
      - selain itu cocokkan terhadap SEMUA peserta thread saja (bukan sesi lain di workspace), tidak membedakan huruf besar/kecil, dengan: `Participant.Name`, `SessionID`, nama project peserta atau `ProjectID` peserta (nama project didapat dari `mgr.cfg.Workspaces.Projects(t.WorkspaceID)`, boleh gagal -> abaikan pencocokan nama project), dan `Slugify(nama project)`; 
      - tepat 1 peserta cocok -> target; 0 -> `fmt.Errorf("%w: %q; peserta: %s", ErrTargetUnknown, to, daftarNama)`; >1 (peserta unik, dedup per SessionID) -> `fmt.Errorf("%w: %q cocok dengan: %s", ErrTargetAmbiguous, to, kandidat)`;
      - target sama dengan pengirim -> galat "tidak bisa mengirim pesan ke diri sendiri" (`ErrInvalid` dibungkus); untuk `all`, pengirim dikeluarkan saja.
      - `logTo` untuk target tunggal = `SessionID` target.
   6. `rec, err := mgr.Post(threadID, ThreadMessage{From: from, To: logTo, Text: text, Kind: KindMessage})`; galat -> kembalikan.
   7. Untuk setiap target sesi: `mgr.SendToSession(threadID, sendFrom, target.SessionID, "[thread] "+text)` dengan `sendFrom = from` (kosong "" bila `from == FromUser`). Kegagalan satu target: `mgr.PostSystem(threadID, "gagal meneruskan ke "+nama+": "+err.Error())`, simpan galat pertama, lanjutkan target lain. Kembalikan `(rec, errGabungan)` bila ada galat (galat dibungkus jelas, mis. `fmt.Errorf("gagal meneruskan ke %s: %w", nama, err)`); record tetap dikembalikan.
   8. `to == user`: berhenti setelah Post (hanya dicatat).
3. Nama peserta untuk pesan: `Participant.Name`, fallback `SessionID`.
4. Catatan: urutan Post lalu SendToSession berarti hook `OnMessage` anti-loop berjalan sebelum pengiriman; itu sengaja (sesuai overview 2.2 no. 6). Jangan menghitung hitungan di `Route`.
5. Pesan user dari `thread.send` TIDAK dialihkan ke `Route` (tiket 01 `Manager.Send` sudah mencatat dan mengirim; hook anti-loop menangkapnya). Jangan mengubah itu.

### Langkah 3: `internal/thread/spawn.go`

1. Tipe:
   ```go
   type SpawnResult struct {
       Reused      bool
       Participant Participant
       ProjectName string
       Worktree    WorktreeInfo // kosong bila Reused
   }
   type Spawner struct {
       cfg  SpawnerConfig
       log  *slog.Logger
       mu   sync.Mutex               // melindungi map di bawah
       locks map[string]*sync.Mutex  // mutex per thread
       once sync.Once                // untuk RegisterTool
   }
   ```
   `NewSpawner`: `cfg.Manager`, `cfg.Sessions`, `cfg.Workspaces`, `cfg.Router`, `cfg.DataDir` wajib non-nil/non-kosong -> panic pesan jelas bila kosong; logger nil -> `slog.Default()`.
2. `spawnResolveProvider() (provider.Provider, error)`: iterasi `[]string{provider.IDClaude, provider.IDCodex}`; ambil yang `provider.Get` ok, `Available()` tanpa galat, dan `Capabilities().MCP` true. Tidak ada -> galat teks persis di Kontrak.
3. `spawnResolveProject(projects []ProjectInfo, query string) (ProjectInfo, error)`: cocok id persis, atau nama case-insensitive (trim), atau `Slugify(nama) == Slugify(query)`. Tepat satu -> hasil. Nol atau lebih dari satu -> galat yang mendaftar project yang tersedia (`nama (id)` dipisah koma); `query` kosong -> galat. Workspace tak dikenal (`ok=false`) -> galat "workspace thread tidak dikenal".
4. `func (sp *Spawner) Spawn(ctx context.Context, threadID, projectQuery, brief string) (SpawnResult, error)` dengan mutex per thread (`spawnLock(threadID)`), brief kosong (trim) -> `ErrInvalid` dibungkus. Urutan:
   1. `t := Manager.Get(threadID)`; `State != StateActive` -> `ErrClosed`.
   2. `projects, ok := Workspaces.Projects(t.WorkspaceID)`; `project := spawnResolveProject(...)` (project di luar workspace -> galat jelas, tidak ada worktree/sesi dibuat).
   3. Reuse: cari di `t.Participants` peserta `RolePeer` dengan `ProjectID == project.ID`. Bila ada: `Sessions.Get(sessionId)`; bila sesi ada dan `Status == session.StatusRunning` -> kirim `brief` lewat `Router.Route(ctx, threadID, masterSessionID, peer.SessionID, brief)` (master dicari dari `t.Participants`; tanpa master -> galat) lalu kembalikan `SpawnResult{Reused: true,...}`. Bila sesi mati/tak ada -> galat `agen project <nama> sudah berhenti` (JANGAN membuat worktree baru). Bila Route gagal, galat diteruskan.
   4. `prov := spawnResolveProvider()` SEBELUM membuat worktree (gagal lebih awal tanpa efek samping).
   5. `slug := Slugify(project.Name)`; `wt, err := CreateWorktree(project.Path, sp.cfg.DataDir, threadID, slug)`; galat diteruskan apa adanya (mis. `ErrNotGitRepo`) tanpa membungkus yang menghilangkan `errors.Is`; gunakan `fmt.Errorf("... %w", err)` agar tetap bisa dikenali. Gagal di sini: belum ada yang perlu di-rollback; boleh `PostSystem("gagal men-spawn <nama>: <alasan>")`.
   6. Sejak titik ini gunakan variabel pelacak rollback (`createdSessionID string`, `addedParticipant bool`, `recordWritten bool`) dan satu fungsi `rollback(reason error)`.
   7. `UpsertWorktreeRecord(Manager.Store().ThreadDir(threadID), WorktreeRecord{ProjectID, ProjectName, ProjectSlug: slug, SessionID: "", Info: wt})`; galat -> rollback.
   8. `s, err := Sessions.Create(session.SessionSpec{WorkspaceID: t.WorkspaceID, ProjectID: project.ID, Cwd: wt.Path, ProviderID: prov.ID()})`; galat -> rollback. Auto-approve datang dari flag provider; cwd di bawah `<DataDir>/threads/<id>/` yang dipakai `launch.go` (fitur lain) untuk memasang MCP; tidak ada metadata sesi tambahan.
   9. Nama: `name := ""`; bila `mgr.cfg.Namer != nil` -> `Namer.NameOf(s.ID)`; kosong -> `prov.ID() + "-" + slug`. 
   10. `Manager.AddParticipant(threadID, Participant{SessionID: s.ID, ProjectID: project.ID, Role: RolePeer, Name: name})`; galat -> rollback. Tandai `addedParticipant`.
   11. Update record dengan `SessionID: s.ID` (`UpsertWorktreeRecord` lagi); galat -> rollback.
   12. `Manager.PostSystem(threadID, "master men-spawn "+project.Name)`.
   13. Kirim brief awal: log bernilai teks `brief` master apa adanya (from=master, to=peer), injeksi bernilai `BuildPeerBrief(t, project.Name, brief, wt)`. Karena `Router.Route` mencatat teks yang sama dengan yang diinjeksi, brief awal dikirim lewat jalur internal: `Manager.Post(threadID, ThreadMessage{From: master.SessionID, To: s.ID, Text: brief, Kind: KindMessage})` lalu `sendWithRetry` ke `Manager.SendToSession(threadID, master.SessionID, s.ID, BuildPeerBrief(...))`. Sesi baru mungkin belum terdaftar di antrean: ulangi sampai 5 kali dengan jeda 300 ms bila `errors.Is(err, mention.ErrNotFound)` (pola `masterSendInitialPrompt`), hormati `ctx.Done()`. Gagal kirim brief setelah retry -> rollback (peserta tidak bisa dipakai tanpa brief). Hitungan anti-loop: `Post` dari master memicu reset (benar).
   14. Kembalikan `SpawnResult`.
   `rollback(reason)`: urutan aman, setiap langkah tidak menggagalkan langkah berikut (log galat sekunder): bila `addedParticipant` -> `UpdateThread` buang peserta ber-SessionID itu; bila `createdSessionID != ""` -> `Sessions.Kill`; `RemoveWorktree(wt)`; `DeleteWorktreeRecord(threadDir, project.ID)`; `PostSystem("gagal men-spawn <nama>: <reason>")`; kembalikan `fmt.Errorf("gagal men-spawn %s: %w", project.Name, reason)`. Rollback tidak boleh menggantung pada ctx yang sudah batal.
5. `BuildPeerBrief(t Thread, projectName, brief string, wt WorktreeInfo) string`: teks Indonesia ringkas yang memuat: konteks (judul thread dan goal, nama project), lalu blok "Aturan thread" WAJIB berisi: (a) bekerja HANYA di worktree sendiri `wt.Path`, jangan menyentuh working tree utama project (`wt.ProjectPath`) atau path lain di luarnya; (b) jangan meminta izin (mode auto-approve); (c) commit pekerjaan di branch `wt.Branch`; (d) komunikasi hanya lewat tool `thread_send`, `thread_read`, `thread_participants` (bukan mengetik ke terminal agent lain); balas pesan yang berawalan `[thread]` dengan `thread_send`, bukan mengetik di terminal; (e) laporkan hasil ke master (`to: "master"`); (f) jangan mem-push atau merge kecuali diminta. Lalu "Tugas dari master:" diikuti `brief`. Tanpa baris kosong berlebih; tidak boleh diawali karakter yang tidak wajar setelah awalan `[pesan dari ...]`.
6. `RegisterTool()`: dengan `sp.once.Do` memanggil `RegisterTool(ToolDef{Name: "thread_spawn", Description: <Indonesia>, InputSchema: {"type":"object","properties":{"project":{"type":"string"...},"brief":{"type":"string"...}},"required":["project","brief"]}, MasterOnly: true, Handler: ...})`. Handler: bila `!c.Master` -> galat "tool hanya untuk master"; decode `args` (`json.RawMessage`) ke struct `{Project, Brief string}` (galat decode -> galat jelas); panggil `Spawn(ctx, c.ThreadID, ...)`; hasil string: baru -> `peserta <nama> (project <nama>) dibuat; worktree: <path>; branch: <branch>`; reuse -> `agen <nama> untuk project <nama> sudah ada; brief dikirim sebagai pesan baru (dipakai ulang)`. Thread selalu dari `c.ThreadID`, tidak pernah dari argumen. `RegisterTool` panik pada duplikat; `sync.Once` mencegahnya.
7. `gofmt`, impor rapi.

### Langkah 4: guard

1. `gofmt -l internal/thread` kosong; `go vet ./internal/thread/...`; `go build ./...` dari akar repo; `go build ./cmd/shikigamid`.
2. `git status`: hanya tiga berkas ownedPaths yang berubah/baru dari fitur ini.
3. Bila build gagal karena berkas fitur lain (mis. `mcp.go` paralel belum selesai), pastikan galat jelas bukan dari tiga berkas ini, lalu laporkan.

## Perubahan

Dibuat (baru):

- `internal/thread/antiloop.go`
- `internal/thread/router.go`
- `internal/thread/spawn.go`

Tidak ada berkas yang diubah.

## Wiring

Yang diekspos untuk dirangkai langkah wiring (`agents_install.go` / `wire_thread.go`); fitur ini TIDAK menyambungkannya sendiri:

- `NewAntiLoop(mgr)` mendaftarkan hook `OnMessage`/`OnClose` sendiri; wiring cukup memanggilnya SEKALI (sebelum `mgr.FireStart()` dan sebelum ada pesan).
- `NewRouter(RouterConfig{Manager, Guard, Logger})` mengembalikan `Router`; hasilnya diberikan ke `ToolDeps.Router` (`thread-mcp-server`) dan `SpawnerConfig.Router`.
- `NewSpawner(SpawnerConfig{...}).RegisterTool()` dipanggil sekali (didaftarkan ke registri global `RegisterTool`; `sync.Once` di dalamnya hanya menjaga per-instance).
- Urutan di `InstallAgents`: `NewAntiLoop` -> `NewRouter` -> `RegisterCommonTools` -> `NewSpawner(...).RegisterTool()`.
- Syarat runtime: `Manager.cfg.Sender` harus `*mention.Queue` khusus thread ber-`ModeAuto` (sudah dipasang tiket 01), sehingga `SendToSession` terkirim saat idle tanpa konfirmasi; mention tidak diubah.
- Keterbatasan yang perlu dicatat wiring: peer hanya Claude (Codex tidak punya MCP, lihat overview 2.2 no. 2); sesi peer harus dapat MCP `shikigami-thread` lewat `launch.go` berdasarkan cwd worktree di `<DataDir>/threads/<id>/<slug>`; pesan "dijeda" dan "dilanjutkan" bertipe `system` agar bisa diberi gaya UI; state anti-loop hilang saat restart (tiket 05).
- Tidak ada perubahan di luar ownedPaths yang diminta fitur ini. Bila nama di kode tiket 01 saat implementasi berbeda dari yang dirujuk di atas, ikuti kode dan catat di sini.

## Selesai bila

- Tiga berkas ownedPaths ada, `gofmt -l internal/thread` kosong, `go vet ./internal/thread/...` bersih.
- `go build ./...` dan `go build ./cmd/shikigamid` sukses (guard; tanpa unit test).
- Signature persis overview 2.3 (`NewSpawner`, `RegisterTool`, `BuildPeerBrief`, `NewRouter`, `NewAntiLoop`, `Paused`, `Check`, `Reset`, `MaxAgentChain = 8`).
- Secara baca-kode terpenuhi: spawn membuat worktree + record + sesi (cwd worktree, provider MCP-capable) + peserta + pesan sistem + brief berisi aturan thread; reuse bila running, galat bila mati; rollback lengkap (peserta, Kill, RemoveWorktree, DeleteWorktreeRecord, pesan sistem, galat); router meresolusi target hanya di peserta thread dan mengirim `[thread] ` lewat `SendToSession`; anti-loop jeda tepat di 8 dengan teks persis dan tolak peer dengan `ErrPaused`; pesan user/master mereset.
- Tidak ada berkas di luar ownedPaths yang berubah.
