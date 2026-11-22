# Overview: Agent Global, Orchestrator di Thread (tiket 03)

Tiket: `docs/tickets/global-agents-ticket-03-agents-orchestrator.md`
Spec: `docs/superpowers/specs/2026-11-22-global-agents-design.md`
Module Go: `github.com/NooBeeID/shikigami`
Depends on: tiket 01 (`internal/agents`, `agents.list`, `SessionSpec.Model`) dan tiket 02. Overview tiket 01: `docs/plans/global-agents-ticket-01-agents-registry/overview.md`.

## 1. Ringkasan epic dan urutan eksekusi

Master thread dijalankan oleh agent Orchestrator default (provider+model dari registry); `thread_spawn` menerima `agent`/`category`; prompt master memuat daftar agent; peserta menyimpan `agentId`/`providerId`/`model` dan dipulihkan dengan nilai tercatat; agent yang dipakai thread aktif tidak bisa dihapus; UI thread menampilkan agent, model, kategori, dan penanda "tanpa agent Orchestrator".

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`, `hasFoundation=true`): tipe bersama, field baru, antarmuka `AgentResolver`, helper provider, dan perubahan tanda tangan fungsi lintas-fitur (dengan nilai netral di semua pemanggil) agar `go build ./...` hijau sebelum fitur paralel. Hanya sisi Go; frontend tidak butuh foundation.
2. **Fitur paralel**: `thread-spawn-agent`, `thread-master-agent` (Go), `thread-agent-ui` (frontend).
3. **Wiring**: satu agen menyambungkan `wire_thread.go` (resolver + guard) dan verifikasi akhir (bagian 5), lalu build guard.

Catatan: `internal/agents` ditulis tiket 01. Bila saat implementasi paket itu atau `SessionSpec.Model` belum ada, itu pelanggaran dependensi: hentikan dan laporkan; jangan menambalnya.

## 2. Kontrak bersama

### 2.1 Dari tiket 01 (jangan diubah)

- Go `internal/agents`: `Agent{ID, Name, CategoryID, ProviderID, Model string; IsDefault bool}`, `Category{ID, Name, Role string; Builtin bool}`, `(*Store).List() ([]Category, []Agent)` (KATEGORI dulu, lalu agent; ini bentuk sebenarnya dari tiket 01, bukan urutan yang tertulis di contoh tiket 03), `(*Store).Resolve(ref) (Agent, Category, error)`, `(*Store).SetDeleteGuard(func(agentID string) error)`, `agents.ErrNotFound`.
- Id kategori bawaan: `orchestrator`, `researcher`, `planner`, `executor`. IPC `agents.list` -> `{categories, agents}`; event `agents.changed`.
- `session.SessionSpec.Model string`. Layanan daemon `d.Lookup("agents.store")` bertipe `*agents.Store`.

### 2.2 Go, paket `internal/thread` (didefinisikan Foundation, dipakai semua fitur Go)

Di `internal/thread/agents_types.go` (hanya ditambah):

```go
// AgentResolver dipenuhi *agents.Store. Impor internal/agents langsung (agents tidak mengimpor thread);
// bila ternyata siklik, ganti dengan tipe cermin AgentInfo/CategoryInfo dan adaptor di wire_thread.go (pilih satu).
type AgentResolver interface {
    Resolve(ref string) (agents.Agent, agents.Category, error)
    List() ([]agents.Category, []agents.Agent)
}

// AgentSelector: pilihan eksplisit pada thread_spawn. Keduanya kosong = default Executor.
type AgentSelector struct{ Agent, Category string }

// ErrNoAgent: tidak ada agent untuk kategori (bukan galat lain). Dibungkus bersama galat resolver.
var ErrNoAgent = errors.New("tidak ada agent default untuk kategori")

// agentAbsent: true bila Resolve(categoryID) gagal karena kategori tidak punya agent sama sekali
// (errors.Is(err, agents.ErrNotFound) untuk id kategori itu, atau tak ada Agent ber-CategoryID tsb di List()).
// Galat lain => false (tetap menggagalkan, bukan fallback).
func agentAbsent(r AgentResolver, categoryID string, resolveErr error) bool

// agentProvider memvalidasi provider agent: terdaftar (provider.Get), Available() nil,
// Capabilities().MCP == true. Galat Indonesia, tanpa fallback:
//   agent <nama>: provider "<id>" tidak terdaftar
//   agent <nama>: provider "<id>" tidak tersedia: <sebab>
//   agent <nama>: provider "<id>" tidak mendukung MCP, tidak bisa memakai tool thread
func agentProvider(a agents.Agent) (provider.Provider, error)

// agentRoleLine: Category.Role dipadatkan ke satu baris (whitespace -> satu spasi), dipotong 160 rune + "…".
func agentRoleLine(role string) string
```

Konfigurasi (semua opsional, nil = perilaku lama penuh tanpa panik):
- `ManagerConfig.Agents AgentResolver` (manager.go)
- `SpawnerConfig.Agents AgentResolver` (spawn.go) dan `AgentsConfig.Agents AgentResolver` (agents_install.go, meneruskan ke `SpawnerConfig`)
- Recovery membaca resolver dari `m.cfg.Agents` (tidak ada konfigurasi baru).

Field baru (JSON camelCase, `omitempty`) di `types.go`:
- `Participant`: `AgentID string json:"agentId,omitempty"`, `AgentName string json:"agentName,omitempty"`, `Category string json:"category,omitempty"` (id kategori), `ProviderID string json:"providerId,omitempty"`, `Model string json:"model,omitempty"`.
- `Thread`: `NoOrchestrator bool json:"noOrchestrator,omitempty"`.
- `store.go` tidak berubah perilaku: field ikut serialisasi struct; berkas lama tetap terbaca.

Tanda tangan baru (Foundation mengubah semua pemanggil dengan nilai netral; fitur mengisi isinya):
- `BuildMasterPrompt(t Thread, projects []ProjectInfo, agentsSection string) string`. `agentsSection` kosong = prompt persis seperti lama; bila tidak kosong disisipkan sebagai blok sendiri setelah blok project dan sebelum "Peranmu". Pemanggil (manager/master.go, recovery.go) memakai `m.agentsSection()`.
- `func (m *Manager) agentsSection() string` (prompt.go): Foundation membuat stub `return ""`; `thread-master-agent` mengisinya (daftar agent + petunjuk, bagian 2.5). Resolver nil/galat/kosong -> `""`.
- `BuildPeerBrief(t Thread, projectName, brief string, wt WorktreeInfo, role PeerRole) string` dengan `type PeerRole struct{ CategoryName, Text string }` (di `agents_types.go`). Nilai nol = persis seperti lama; `Text` tidak kosong -> bagian `Peranmu (<CategoryName>):\n<Text>\n` disisipkan sebelum "Tugas dari master:".
- `func (sp *Spawner) Spawn(ctx context.Context, threadID, projectQuery, brief string, sel AgentSelector) (SpawnResult, error)`.

### 2.3 Tool MCP `thread_spawn`

Args: `{project: string, brief: string, agent?: string, category?: string}`. `agent` = nama agent; `category` = id/nama kategori. Keduanya kosong -> `Resolve("executor")`; galat "tidak ada" (`agentAbsent`) -> perilaku lama (`spawnResolveProvider`: claude lalu codex, model kosong); galat lain dikembalikan. Hanya `agent` -> `Resolve(agent)`. Hanya `category` -> `Resolve(category)` (default kategori). Keduanya -> agent dipakai, galat bila `Category.ID`/`Category.Name` hasil resolusi tidak cocok dengan `category` (banding id atau nama, case-insensitive). Pilihan eksplisit yang gagal -> galat tool jelas, tanpa spawn sesi dan tanpa fallback. `agentProvider` wajib lolos (MCP true).

### 2.4 Perilaku master (`SpawnMaster`)

- `Agents != nil`: `Resolve("orchestrator")`.
  - Sukses: `agentProvider(agent)` (galat -> gagal, tidak fallback), `SessionSpec{ProviderID, Model}` dari agent. Peserta master: `AgentID`, `AgentName`, `Category:"orchestrator"`, `ProviderID`, `Model`.
  - Galat dan `agentAbsent(Agents, "orchestrator", err)`: perilaku lama (`resolveMasterProvider`), peserta master hanya `ProviderID` terisi, `UpdateThread(... NoOrchestrator=true)` dan publikasi `thread.updated` SEBELUM thread terlihat user (di `SpawnMaster`; `Create` sudah memanggil `managerPublishThread` sebelum spawn, jadi `UpdateThread` harus memicu `thread.updated`; verifikasi di kode, bila `UpdateThread` tidak memublikasikan, panggil `managerPublishThread`).
  - Galat lain: gagal (lewat `managerSpawnSafe`/`Create` yang sudah ada menjadi pesan sistem).
- `Agents == nil`: perilaku lama, `NoOrchestrator` TIDAK diset (tanpa registry, tidak ada konsep Orchestrator).
- Kegagalan setelah sesi dibuat (mis. `AddParticipant` gagal): `Sessions.Kill(s.ID)`, tidak ada sesi yatim.

### 2.5 Prompt master, bagian "Agent yang tersedia"

Satu baris per agent: `- <nama> (kategori <Nama Kategori>)<, default> : <agentRoleLine(role)>`; ", default" bila `IsDefault`. Urutan: kategori menurut `List()`, agent di dalamnya menurut nama. Diikuti petunjuk: pilih agent atau kategori menurut sifat pekerjaan (perencanaan -> Planner, riset -> Researcher, pengubahan kode -> Executor), isi `agent` atau `category` pada `thread_spawn`, bila tidak yakin biarkan kosong. Tanpa agent terdaftar: bagian dihilangkan. Teks `AgentToolsSection` (mcp_tools.go) menjadi `thread_spawn(project, brief, agent?, category?)`, dimiliki `thread-spawn-agent`.

### 2.6 Peserta, reuse, pemulihan, guard hapus

- Reuse peer: cocok bila `ProjectID` sama DAN `AgentID` sama. Peserta lama tanpa `AgentID` hanya cocok dengan pilihan "tanpa agent" (perilaku lama: resolusi menghasilkan `AgentID == ""`). Agent berbeda di project sama -> peserta baru. Nama peserta: lewat `Namer`; fallback `<agent>-<slug>` bila agent dipakai, selain itu `<provider>-<slug>`.
- Recovery (`respawnOne`): pakai `ProviderID`+`Model` tercatat pada `Participant`. Provider tercatat tak terdaftar/tak tersedia -> `fail(...)` jelas (`RecoveryFailedPrefix`), tanpa fallback. `ProviderID` kosong (peserta lama) -> logika lama. Sesi pengganti mewarisi `AgentID`, `AgentName`, `Category`, `ProviderID`, `Model` (termasuk `UpdateThread` yang menyalin peserta). `PeerRole` dipulihkan dari kategori terkini (`Agents.List()` cocokkan `Category` id); tak ada resolver/kategori -> `PeerRole{}`. Prompt master pemulihan: `BuildMasterPrompt(t, projects, m.agentsSection())`.
- `func (m *Manager) InUse(agentID string) (threadTitle string, inUse bool)` (dimiliki `thread-spawn-agent`, ditaruh di akhir `agents_types.go`). Iterasi `managerAllThreads()` (baca saja, tanpa menahan kunci thread lama); true bila `State == StateActive` dan ada peserta (termasuk `Dead`) ber-`AgentID == agentID`. `AgentID` kosong -> false. Thread `closed` tidak dihitung.
- Galat guard (dibuat di `wire_thread.go` saat Wiring): `agent "<nama>" dipakai thread aktif "<judul>"`. Nama agent diambil dari `Agents.List()` berdasarkan id; fallback ke id.

### 2.7 Kontrak frontend (`frontend/src/modules/thread/`)

`types.ts`: `Participant` + `agentId?, agentName?, category?, providerId?, model?` (string); `Thread` + `noOrchestrator?: boolean`. Tidak ada IPC thread baru. UI memanggil `agents.list` (-> `{categories: {id,name,role,builtin}[], agents: {id,name,categoryId,providerId,model,isDefault}[]}`) lewat klien thread sendiri dan memuat ulang pada event `agents.changed`; kegagalan -> id kategori mentah. Dilarang impor dari `modules/settings/agents`.

### 2.8 Konvensi

Komentar, log, galat, teks UI: Indonesia. Identifier: Inggris. Fungsi internal paket thread memakai awalan nama file/fitur (pola yang ada: `spawnX`, `managerX`, `recoveryX`); helper baru bersama diawali `agent`. Tanpa unit test.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `thread-spawn-agent` | `thread_spawn` dengan `agent`/`category` (schema + `AgentToolsSection`), `Spawner.Spawn(..., sel)`, resolusi+validasi provider, peserta peer menyimpan agent/provider/model, reuse per project+agent (worktree aman), `BuildPeerBrief` + `PeerRole`, recovery memakai provider+model tercatat, `Manager.InUse` | `internal/thread/mcp_tools.go`, `internal/thread/mcp.go`, `internal/thread/spawn.go`, `internal/thread/store.go`, `internal/thread/recovery.go`, `internal/thread/worktree.go`, `internal/thread/worktree_records.go` (dua terakhir hanya bila perlu worktree per agent), plus `internal/thread/agents_types.go` untuk `InUse` saja (tambahan di akhir berkas) |
| `thread-master-agent` | `SpawnMaster` dengan Orchestrator default (provider+model, validasi MCP, `NoOrchestrator`, kill sesi bila gagal), isi `agentsSection` + `BuildMasterPrompt`, log peringatan di launch.go | `internal/thread/manager.go`, `internal/thread/master.go`, `internal/thread/launch.go`, `internal/thread/prompt.go` |
| `thread-agent-ui` | Tipe TS, normalisasi store/client, cache `agents.list`, `ParticipantRow` (agent · kategori · model), `ThreadHeader` (master + penanda tanpa Orchestrator) | `frontend/src/modules/thread/` |

Penyimpangan dari tiket (disengaja): `cmd/shikigamid/wire_thread.go` TIDAK dimiliki fitur mana pun; seluruh isinya (resolver dan guard) dikerjakan di Wiring (bagian 5), karena dua fitur sama-sama menyentuhnya dan itu file wiring. Definisi tipe/field dan tanda tangan dipindah ke Foundation agar dua fitur Go tidak saling menunggu. Isi `InUse` tetap milik `thread-spawn-agent`; `recovery.go` memakai `agentsSection` yang diisi `thread-master-agent` lewat stub Foundation.

## 4. Foundation

Ada: `00-foundation.md` (sisi Go saja). Isi: tipe/helper bagian 2.2 (kecuali `InUse` dan isi `agentsSection`), field `Participant`/`Thread`, `Agents` pada `ManagerConfig`/`SpawnerConfig`/`AgentsConfig`, perubahan tanda tangan `BuildMasterPrompt`/`BuildPeerBrief`/`Spawn` dengan nilai netral di seluruh pemanggil, serta stub `agentsSection`. Selesai bila `go build ./...` lulus dan perilaku identik dengan sebelumnya.

## 5. Wiring checklist

Dikerjakan setelah ketiga fitur selesai. Satu-satunya file yang diubah: `cmd/shikigamid/wire_thread.go` (+ verifikasi).

1. Impor `internal/agents`. Setelah `session.Manager` didapat dan sebelum `thread.NewManager`: `v, ok := d.Lookup("agents.store")`; bila `ok` dan `*agents.Store` non-nil, simpan `var resolver thread.AgentResolver = store` dan `var agentStore *agents.Store`. Bila tidak: `d.Logger.Warn("thread: agents.store tidak tersedia, thread memakai Claude polos")` dan lanjut (resolver nil; jangan memberi interface berisi nil-pointer).
2. Isi `thread.ManagerConfig.Agents = resolver` dan `thread.AgentsConfig.Agents = resolver` (Recovery ikut lewat `m.cfg.Agents`).
3. Setelah `thread.NewManager` dan SEBELUM `m.FireStart()`: bila `agentStore != nil`, `agentStore.SetDeleteGuard(func(id string) error { if title, ok := m.InUse(id); ok { return fmt.Errorf("agent %q dipakai thread aktif %q", name(id), title) }; return nil })`. Nama dari `agentStore.List()` (fallback id). Tambah impor `fmt`. Guard tidak boleh memanggil metode `agents.Store` yang mengunci saat dipanggil dari dalam `Delete` (deadlock): ambil nama agent lewat snapshot yang dibuat sebelum guard berjalan atau cukup pakai id bila `List()` tidak aman; verifikasi di `internal/agents` apakah guard dipanggil saat kunci dipegang, dan bila ya jangan panggil `List()` di dalam guard (simpan cache nama dari `agents.changed` atau pakai id).
4. Pastikan urutan pemasangan lain tidak berubah: `RegisterIPC`, `InstallAgents`, `InstallActivity`, `RegisterClose`, `NewRecovery`, `SetPreSend`, `OnStart`, `FireStart`.
5. Cari semua pemanggil `Spawn(`, `BuildPeerBrief(`, `BuildMasterPrompt(` (`grep -rn`) di repo; semuanya harus memakai tanda tangan baru. Frontend: tidak ada registrasi baru; tidak ada perubahan `index.ts`.
6. Verifikasi manual bila memungkinkan: buat agent Orchestrator + Planner + Executor di Settings; thread baru (header menampilkan agent+model; tanpa Orchestrator tampil penanda); minta master men-spawn Planner; restart daemon dan kirim pesan ke peer; hapus agent yang dipakai (ditolak, menyebut judul thread) dan yang tidak dipakai (berhasil).

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` (dari root repo)
- Frontend: `cd frontend && npm run build` dan `npm run check`

Fitur Go cukup `go build ./...`; fitur frontend `npm run build` + `npm run check`.

## 7. Aturan paralel

- Tiap agen hanya menyentuh `ownedPaths` fiturnya (bagian 3). Dilarang menyentuh path fitur lain, `cmd/shikigamid/wire_thread.go` (milik Wiring), `internal/agents/`, `internal/mention/`, `internal/session/`, `internal/provider/`, `frontend/src/modules/settings/`.
- Foundation selesai dan ter-merge sebelum fitur paralel dimulai; fitur tidak mengubah tanda tangan/kontrak bagian 2 (bila perlu perubahan, catat di laporan, jangan edit).
- `thread-agent-ui` hanya bergantung pada nama field JSON (bagian 2.2 dan 2.7), bukan pada kode Go.
- Dua fitur Go memakai berkas berbeda: `recovery.go` hanya milik `thread-spawn-agent`; `prompt.go`/`master.go`/`manager.go` hanya milik `thread-master-agent`. Titik temu hanya lewat `agentsSection()`, `agentProvider`, `agentAbsent`, `agentRoleLine` dari Foundation.
- Jangan menambah unit test. Kebutuhan di luar ownedPaths dicatat di laporan.
