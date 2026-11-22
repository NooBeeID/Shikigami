# Plan: Master dengan agent Orchestrator default

## Metadata

- Tiket: `global-agents-ticket-03-agents-orchestrator` (`docs/tickets/global-agents-ticket-03-agents-orchestrator.md`)
- Fitur: `thread-master-agent`
- Overview (kontrak bersama, dibaca bersama plan ini): `docs/plans/global-agents-ticket-03-agents-orchestrator/overview.md`, terutama bagian 2.2, 2.4, 2.5
- ownedPaths: `internal/thread/manager.go`, `internal/thread/master.go`, `internal/thread/launch.go`, `internal/thread/prompt.go`
- Depends on foundation: **ya** (`00-foundation.md` sudah ter-merge sebelum plan ini dikerjakan). Foundation menyediakan: `AgentResolver`, `agentAbsent`, `agentProvider`, `agentRoleLine`, `ErrNoAgent` di `agents_types.go`; field `Participant.AgentID/AgentName/Category/ProviderID/Model` dan `Thread.NoOrchestrator` di `types.go`; `ManagerConfig.Agents`; tanda tangan `BuildMasterPrompt(t, projects, agentsSection string)`; stub `func (m *Manager) agentsSection() string { return "" }` di `prompt.go`.
- Prasyarat keras: bila simbol foundation di atas belum ada di kode, atau `internal/agents` / `session.SessionSpec.Model` belum ada, hentikan dan laporkan; jangan menambalnya.

## Tujuan

Master thread dijalankan oleh agent Orchestrator default dari registry (provider+model), divalidasi lewat `agentProvider` tanpa fallback; peserta master menyimpan identitas agent; tanpa agent Orchestrator master tetap Claude polos dan `Thread.NoOrchestrator=true` dipublikasikan; sesi yang sudah dibuat di-kill bila langkah berikutnya gagal. Prompt master memuat daftar agent lewat `Manager.agentsSection()`. `launch.go` mencatat peringatan bila sesi di cwd thread berprovider tanpa MCP.

## Path yang dimiliki

HANYA berkas berikut yang boleh diubah:

- `internal/thread/master.go`
- `internal/thread/prompt.go`
- `internal/thread/launch.go`
- `internal/thread/manager.go` (kemungkinan tidak perlu diubah; hanya bila ada kebutuhan kecil, lihat Langkah 5)

DILARANG menyentuh: `cmd/shikigamid/wire_thread.go` (milik Wiring), `internal/thread/agents_types.go`, `types.go`, `spawn.go`, `recovery.go`, `mcp_tools.go`, `mcp.go`, `store.go`, `worktree*.go` (fitur lain/foundation), `internal/agents/`, `internal/session/`, `internal/provider/`, `internal/mention/`, `frontend/`. Jangan mengubah overview.md. Jangan menambah unit test. Kebutuhan di luar ownedPaths dicatat di laporan, bukan dikerjakan.

## Kontrak

Dipakai (dari foundation/tiket 01, jangan diubah):
- `m.cfg.Agents AgentResolver` dengan `Resolve(ref) (agents.Agent, agents.Category, error)` dan `List() ([]agents.Category, []agents.Agent)` (kategori dulu, lalu agent).
- `agentAbsent(r AgentResolver, categoryID string, resolveErr error) bool`, `agentProvider(a agents.Agent) (provider.Provider, error)` (galat Indonesia sudah diformat, MCP wajib true), `agentRoleLine(role string) string`.
- `session.SessionSpec{WorkspaceID, ProjectID, Cwd, ProviderID, Model}`.
- `BuildMasterPrompt(t Thread, projects []ProjectInfo, agentsSection string) string`: `agentsSection` kosong = prompt persis seperti lama; tidak kosong disisipkan oleh foundation sebagai blok sendiri setelah blok project dan sebelum "Peranmu". Fitur ini TIDAK mengubah tanda tangan itu.
- `UpdateThread` sudah memanggil `managerPublishThread` (event `thread.updated`) setelah menyimpan; tidak perlu publikasi manual tambahan.

Disediakan (untuk Wiring dan fitur `thread-spawn-agent`/recovery):
- `func (m *Manager) agentsSection() string` di `prompt.go` (isi nyata, menggantikan stub). `recovery.go` sudah memanggilnya lewat stub foundation, jadi pemulihan otomatis mendapat daftar agent.
- Perilaku `SpawnMaster` per overview 2.4 (lihat Langkah 2).
- Format baris agent per overview 2.5: `- <nama> (kategori <Nama Kategori>)<, default> : <agentRoleLine(role)>`.

## Langkah

### 1. `internal/thread/master.go`: helper resolusi master

Tambah (dekat `resolveMasterProvider`, yang TETAP dipertahankan dan dipakai `recovery.go`):

```go
// masterChoice hasil pemilihan provider master.
type masterChoice struct {
	Prov           provider.Provider
	Model          string
	Agent          agents.Agent // nol bila Claude polos
	NoOrchestrator bool         // true: registry ada tetapi tak ada agent Orchestrator
}

// masterResolve memilih provider+model master.
//   - m.cfg.Agents == nil: resolveMasterProvider(), NoOrchestrator=false (tanpa registry tak ada konsep Orchestrator).
//   - Resolve("orchestrator") sukses: agentProvider(agent) (galat -> dikembalikan apa adanya, TANPA fallback),
//     Model = agent.Model.
//   - Resolve galat dan agentAbsent(m.cfg.Agents, "orchestrator", err): resolveMasterProvider(), NoOrchestrator=true.
//   - Resolve galat lain: dibungkus fmt.Errorf("agent orchestrator: %w", err), tanpa fallback.
func (m *Manager) masterResolve() (masterChoice, error)
```

Impor `internal/agents` di `master.go` (agents tidak mengimpor thread; foundation sudah mengimpornya di paket yang sama).

Catatan galat: pesan dari `agentProvider` sudah memuat nama agent dan sebab (tidak terdaftar / tidak tersedia / tidak mendukung MCP). Cukup tambahkan konteks bila perlu, mis. `fmt.Errorf("master: %w", err)`; jangan menimpa pesannya. Galat ini tampil lewat `managerSpawnSafe` -> `Create` -> `PostSystem("gagal membuat master: ...")` yang sudah ada.

### 2. `internal/thread/master.go`: ubah `SpawnMaster`

Alur baru (signature tetap `func (m *Manager) SpawnMaster(t Thread) (Participant, error)`):

1. `ch, err := m.masterResolve()`; galat -> `return Participant{}, err` (belum ada sesi, tidak ada yang di-kill).
2. `MkdirAll(dir)` seperti sekarang.
3. `Sessions.Create(session.SessionSpec{WorkspaceID: t.WorkspaceID, ProjectID: "", Cwd: dir, ProviderID: ch.Prov.ID(), Model: ch.Model})`.
4. Sejak titik ini setiap galat WAJIB `_ = m.cfg.Sessions.Kill(s.ID)` sebelum return (pola yang sudah ada untuk `AddParticipant`; terapkan juga ke langkah 5).
5. Bila `ch.NoOrchestrator`: `_, err := m.UpdateThread(t.ID, func(x *Thread) { x.NoOrchestrator = true })`; galat -> kill sesi + return galat. `UpdateThread` sudah memublikasikan `thread.updated` (diverifikasi di `manager.go`, `UpdateThread` memanggil `managerPublishThread`); tidak perlu panggilan tambahan. Urutan: sebelum `AddParticipant` agar klien menerima flag lebih dulu atau bersamaan dengan peserta master. Salin juga ke variabel lokal `t.NoOrchestrator = true` agar prompt memakai nilai terkini.
6. Bangun `part`:
   - Agent dipakai: `Participant{SessionID: s.ID, ProjectID: "", Role: RoleMaster, Name: "master", AgentID: ch.Agent.ID, AgentName: ch.Agent.Name, Category: "orchestrator", ProviderID: ch.Prov.ID(), Model: ch.Model}`.
   - Claude polos (fallback atau `Agents == nil`): hanya `ProviderID: ch.Prov.ID()` yang terisi di antara field baru (`Model` kosong).
7. `AddParticipant` seperti sekarang (galat -> kill + return).
8. Prompt: `BuildMasterPrompt(t, projects, m.agentsSection())` (foundation sudah memakai bentuk ini; pertahankan). Sisanya (`masterSendInitialPrompt`) tidak berubah.

Jangan memanggil `Sessions.Kill` untuk kegagalan sebelum sesi dibuat; jangan fallback saat validasi Orchestrator gagal.

### 3. `internal/thread/prompt.go`: isi `agentsSection()`

Ganti stub foundation:

```go
// agentsSection menghasilkan bagian "Agent yang tersedia" untuk prompt master.
// Resolver nil, galat, atau tanpa agent -> "" (bagian dihilangkan, perilaku lama).
func (m *Manager) agentsSection() string
```

Isi:
- `m.cfg.Agents == nil` -> `""`. Bungkus pemanggilan `List()` dengan `defer recover()` ringan bila perlu agar tidak menggagalkan thread (kembalikan `""`).
- `cats, ags := m.cfg.Agents.List()`; `len(ags) == 0` -> `""`.
- Urutan: iterasi `cats` sesuai urutan `List()`; di tiap kategori, agent ber-`CategoryID == cat.ID` diurutkan menurut nama (`sort.SliceStable`, banding `strings.ToLower`). Agent yang kategorinya tidak ada di `cats` ditaruh di akhir (jaga-jaga), dengan nama kategori = `CategoryID`.
- Judul: `Agent yang tersedia:` lalu satu baris per agent: `- <Name> (kategori <Category.Name>)` + `, default` bila `IsDefault` + ` : ` + `agentRoleLine(cat.Role)`. Bila peran kosong, tulis tanpa suffix ` : ...` (hindari ` : ` menggantung).
- Petunjuk (satu paragraf pendek, Indonesia): pilih agent atau kategori menurut sifat pekerjaan (perencanaan -> Planner, riset -> Researcher, pengubahan kode -> Executor); isi parameter `agent` (nama agent) atau `category` (id/nama kategori) pada `thread_spawn`; bila tidak yakin biarkan kosong (sistem memakai default).
- Kembalikan string tanpa newline akhir berlebih (foundation menambahkan pemisah baris kosong). Impor `sort`.
- Jangan mengubah `ToolsSection`/`AgentToolsSection` (milik `thread-spawn-agent`).

### 4. `internal/thread/launch.go`: log peringatan provider tanpa MCP

Di closure `provider.RegisterLaunchContribution`, ubah urutan guard agar peringatan hanya muncul untuk sesi di cwd thread yang aktif:

1. `ctx == nil || ctx.Provider == nil` -> return (tanpa log).
2. `ThreadIDFromCwd(dataDir, ctx.Session.Cwd)`; tidak cocok -> return.
3. `m.Get(id)`; galat atau `State != StateActive` -> return.
4. Bila `!ctx.Provider.Capabilities().MCP`: `logger.Warn("thread: provider tanpa MCP di cwd thread; tool thread tidak dipasang", "thread", id, "provider", ctx.Provider.ID(), "session", ctx.Session.ID)` lalu return.
5. Sisanya (URL, token, `ctx.MCPServers[mcpServerName]`) TIDAK diubah.

Perilaku fungsional tetap sama (provider tanpa MCP tetap tidak mendapat kontribusi); hanya ditambah log. Pastikan nama field sesi yang dipakai di log (`ctx.Session.ID`) memang ada di `session.Session`; bila tidak, hilangkan atribut itu.

### 5. `internal/thread/manager.go`

Tidak ada perubahan wajib: `ManagerConfig.Agents` sudah ada dari foundation, `UpdateThread` sudah memublikasikan `thread.updated`, `Create`/`managerSpawnSafe` sudah mengubah galat `SpawnMaster` menjadi pesan sistem. Verifikasi tiga hal ini dengan membaca kodenya. Ubah `manager.go` hanya bila salah satu ternyata tidak benar (mis. `UpdateThread` tidak memublikasikan: tambahkan `m.managerPublishThread(t)` di jalur yang dimiliki fitur ini) dan catat di laporan.

### 6. Verifikasi

- `go build ./...` dari root repo lulus; `go vet ./internal/thread/` bersih.
- `grep -rn "BuildMasterPrompt(" --include=*.go .` : semua pemanggil memakai tiga argumen (jangan edit `recovery.go`; bila ada yang tertinggal, catat di laporan).
- Tinjau manual diff: dengan `Agents == nil` perilaku `SpawnMaster` identik dengan sebelumnya (provider claude/codex, `Model` kosong, `NoOrchestrator` tidak diset, peserta hanya mengisi `ProviderID`).

## Perubahan

Diubah:
- `internal/thread/master.go`: tipe `masterChoice`, `(*Manager).masterResolve`, `SpawnMaster` baru (provider+model dari agent, `NoOrchestrator`, kill sesi bila gagal, peserta master berisi identitas agent); `resolveMasterProvider` dipertahankan.
- `internal/thread/prompt.go`: isi `(*Manager).agentsSection()`.
- `internal/thread/launch.go`: log peringatan provider tanpa MCP pada sesi di cwd thread aktif.
- `internal/thread/manager.go`: tidak diubah kecuali Langkah 5 memaksa.

Tidak ada berkas baru.

## Wiring

Yang diekspos untuk langkah Wiring (jangan disambungkan di sini):
- `Manager` memakai `ManagerConfig.Agents`; Wiring (`cmd/shikigamid/wire_thread.go`, overview bagian 5 langkah 1-2) mengisinya dengan `*agents.Store` (nil bila store tidak tersedia, jangan interface berisi nil-pointer). Tanpa itu `SpawnMaster` berperilaku lama dan `NoOrchestrator` tak pernah diset.
- `agentsSection()` otomatis dipakai `SpawnMaster` dan `recovery.go` (via stub foundation yang kini terisi); tidak ada sambungan tambahan.
- `InstallLaunchContribution` (sudah dipanggil dari wiring) tidak berubah tanda tangan; log peringatan aktif otomatis.

## Selesai bila

- `go build ./...` sukses (guard utama; `go vet ./internal/thread/` bersih). Tanpa unit test.
- Hanya `master.go`, `prompt.go`, `launch.go` (dan `manager.go` bila terpaksa) yang berubah; `git diff --stat` tidak memuat berkas lain.
- `SpawnMaster` dengan `Agents != nil` dan Orchestrator valid membuat sesi ber-`ProviderID`+`Model` agent, dan peserta master menyimpan `AgentID`, `AgentName`, `Category:"orchestrator"`, `ProviderID`, `Model`.
- Orchestrator berprovider tak terdaftar/tak tersedia/tanpa MCP -> galat jelas dari `agentProvider`, tanpa fallback, tanpa sesi dibuat; kegagalan setelah sesi dibuat memanggil `Sessions.Kill`.
- Tanpa agent Orchestrator (`agentAbsent` true) -> Claude polos, `Thread.NoOrchestrator=true` tersimpan dan `thread.updated` terbit; `Agents == nil` -> `NoOrchestrator` tidak diset.
- `agentsSection()` mengembalikan daftar agent + petunjuk sesuai format overview 2.5, dan `""` bila resolver nil/galat/tanpa agent (prompt identik dengan lama).
- `launch.go` mencatat `Warn` untuk provider tanpa MCP di cwd thread aktif tanpa mengubah perilaku lain.
