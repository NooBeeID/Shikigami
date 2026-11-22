# Foundation: Agent Global, Orchestrator di Thread (tiket 03)

Overview: `docs/plans/global-agents-ticket-03-agents-orchestrator/overview.md` (bagian 2.2 adalah kontrak; ikuti persis).
Tujuan: menyiapkan tipe, field, helper, dan tanda tangan baru di `internal/thread/` agar fitur Go paralel (`thread-spawn-agent`, `thread-master-agent`) bisa dikerjakan tanpa saling menunggu. Perilaku runtime TIDAK berubah. Hanya Go. Tanpa unit test.

Prasyarat: `internal/agents` (tiket 01) dan `session.SessionSpec.Model` sudah ada. Bila belum: hentikan dan laporkan.

## Langkah

### 1. `internal/thread/agents_types.go` (hanya tambah di akhir berkas)
Tambah impor `internal/agents`, `internal/provider`, `fmt`, `strings` seperlunya. Tulis persis dari overview 2.2:
- `AgentResolver` (`Resolve`, `List() ([]agents.Category, []agents.Agent)`). Bila impor `internal/agents` membuat siklus, pakai tipe cermin lokal dan catat di laporan.
- `AgentSelector{Agent, Category string}`, `PeerRole{CategoryName, Text string}`, `ErrNoAgent`.
- `agentAbsent(r AgentResolver, categoryID string, resolveErr error) bool`: false bila `resolveErr == nil` atau `r == nil`; true bila `errors.Is(resolveErr, agents.ErrNotFound)` atau `r.List()` tidak memuat agent ber-`CategoryID == categoryID`; selain itu false.
- `agentProvider(a agents.Agent) (provider.Provider, error)`: `provider.Get(a.ProviderID)`; tak ada -> galat "tidak terdaftar"; `Available()` galat -> "tidak tersedia: <sebab>"; `Capabilities().MCP == false` -> "tidak mendukung MCP, tidak bisa memakai tool thread". Format galat sesuai overview.
- `agentRoleLine(role string) string`: `strings.Fields` digabung spasi, potong 160 rune + "…".
Jangan menambah `InUse` (milik `thread-spawn-agent`).

### 2. `internal/thread/types.go`
Tambah field `Participant` (`AgentID`, `AgentName`, `Category`, `ProviderID`, `Model`) dan `Thread.NoOrchestrator` dengan tag JSON `omitempty` persis seperti overview 2.2, di blok komentar "Ditambah epic global-agents-ticket-03". Jangan mengubah field lain.

### 3. Konfigurasi `Agents`
- `manager.go`: `ManagerConfig.Agents AgentResolver` (opsional; komentar: nil = perilaku lama).
- `spawn.go`: `SpawnerConfig.Agents AgentResolver`.
- `agents_install.go`: `AgentsConfig.Agents AgentResolver`, diteruskan ke `NewSpawner(SpawnerConfig{..., Agents: cfg.Agents})`.
Hati-hati nil interface: jangan menyentuh `wire_thread.go` (milik Wiring).

### 4. Tanda tangan baru dengan nilai netral
- `prompt.go`: `BuildMasterPrompt(t, projects, agentsSection string)`; bila `strings.TrimSpace(agentsSection) != ""` tulis sebagai blok sendiri (diakhiri baris kosong) setelah blok project dan sebelum paragraf "Peranmu". Tambah stub `func (m *Manager) agentsSection() string { return "" }` dengan komentar bahwa `thread-master-agent` mengisinya.
- `spawn.go`: `BuildPeerBrief(t, projectName, brief, wt, role PeerRole)`; bila `strings.TrimSpace(role.Text) != ""` sisipkan `Peranmu (<CategoryName>):\n<Text>\n` sebelum "Tugas dari master:". `Spawner.Spawn(ctx, threadID, projectQuery, brief string, sel AgentSelector)`; abaikan `sel` untuk saat ini (`_ = sel`). Handler tool `thread_spawn` memanggil `Spawn(..., AgentSelector{})`.
- Perbarui semua pemanggil dengan nilai netral: `master.go` (`BuildMasterPrompt(t, projects, m.agentsSection())`), `recovery.go` (`BuildMasterPrompt(t, projects, m.agentsSection())` lewat `r.m`, dan `BuildPeerBrief(..., PeerRole{})`). Gunakan `grep -rn "BuildMasterPrompt(\|BuildPeerBrief(\|\.Spawn(" /Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal --include=*.go` untuk memastikan tidak ada pemanggil tertinggal.

### 5. Verifikasi
- `go build ./...` lulus; `go vet ./internal/thread/` bersih.
- Tinjau diff: tidak ada perubahan perilaku (semua nilai netral; `agentsSection()` kosong menghasilkan prompt identik dengan sebelumnya).
- Commit foundation sebelum fitur paralel dimulai.
