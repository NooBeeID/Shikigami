# Plan: thread_spawn dengan agent/category, peserta agentId, recovery

## Metadata

- Tiket: `docs/tickets/global-agents-ticket-03-agents-orchestrator.md` (epic `global-agents-ticket-03-agents-orchestrator`)
- Fitur: `thread-spawn-agent`
- Overview (kontrak bersama, jangan diubah): `docs/plans/global-agents-ticket-03-agents-orchestrator/overview.md`
- Foundation: `docs/plans/global-agents-ticket-03-agents-orchestrator/00-foundation.md`
- Spec: `docs/superpowers/specs/2026-11-22-global-agents-design.md`
- ownedPaths: `internal/thread/mcp_tools.go`, `internal/thread/mcp.go`, `internal/thread/spawn.go`, `internal/thread/store.go`, `internal/thread/recovery.go`, `internal/thread/worktree.go`, `internal/thread/worktree_records.go`, `internal/thread/agents_types.go`
- Depends on foundation: **ya**. Foundation sudah ter-merge sebelum fitur ini dimulai. Bila saat memulai hal berikut belum ada, itu pelanggaran dependensi: hentikan dan laporkan, jangan menambalnya: `AgentResolver`, `AgentSelector`, `PeerRole`, `ErrNoAgent`, `agentAbsent`, `agentProvider`, `agentRoleLine` di `agents_types.go`; field `Participant.AgentID/AgentName/Category/ProviderID/Model` di `types.go`; `SpawnerConfig.Agents`; tanda tangan baru `Spawner.Spawn(..., sel AgentSelector)`, `BuildPeerBrief(..., role PeerRole)`, `BuildMasterPrompt(t, projects, agentsSection)`; stub `(*Manager).agentsSection()`; `ManagerConfig.Agents`; paket `internal/agents` dan `session.SessionSpec.Model`.

## Tujuan

Master dapat men-spawn peer memakai agent global lewat `thread_spawn(project, brief, agent?, category?)`. Peer berjalan dengan provider+model agent itu dan brief-nya memuat teks peran kategori. `Participant` peer menyimpan identitas agent. Peer dipakai ulang per project+agent (agent berbeda di project yang sama menjadi peserta baru dengan worktree sendiri). Setelah daemon restart, peserta dihidupkan dengan provider+model yang tercatat. `Manager.InUse(agentID)` tersedia untuk guard hapus agent (disambungkan nanti oleh Wiring).

Aturan perilaku lengkap ada di overview bagian 2.3 (resolusi `thread_spawn`) dan 2.6 (peserta, reuse, recovery, `InUse`); plan ini hanya menetapkan langkah implementasinya.

## Path yang dimiliki

HANYA berkas berikut yang boleh disentuh:

- `internal/thread/spawn.go` (utama)
- `internal/thread/mcp_tools.go` (hanya `AgentToolsSection`)
- `internal/thread/recovery.go`
- `internal/thread/worktree_records.go`
- `internal/thread/agents_types.go` (hanya menambah `InUse` di AKHIR berkas)
- `internal/thread/worktree.go`, `internal/thread/mcp.go`, `internal/thread/store.go`: boleh disentuh tetapi rencana ini tidak membutuhkan perubahan di sana (lihat Perubahan). Ubah hanya bila build/verifikasi memaksa, dan catat di laporan.

DILARANG: `cmd/shikigamid/wire_thread.go` (milik Wiring), `internal/thread/manager.go`, `master.go`, `launch.go`, `prompt.go` (milik `thread-master-agent`), `close.go`, `orphans.go`, `types.go`, `frontend/`, `internal/agents/`, `internal/session/`, `internal/provider/`, `internal/mention/`. Jangan mengubah tanda tangan atau kontrak overview bagian 2; bila perlu, catat di laporan. Jangan menambah unit test. Jangan memodifikasi `overview.md`.

## Kontrak

Dipakai dari Foundation (overview 2.2, tidak diubah di sini):
- `AgentResolver` (`Resolve(ref) (agents.Agent, agents.Category, error)`, `List() ([]agents.Category, []agents.Agent)`), `AgentSelector{Agent, Category}`, `PeerRole{CategoryName, Text}`, `ErrNoAgent`.
- `agentAbsent(r, categoryID, resolveErr) bool`, `agentProvider(a agents.Agent) (provider.Provider, error)` (galat Indonesia sudah jadi; jangan dibungkus ulang sehingga pesannya hilang), `agentRoleLine`.
- `SpawnerConfig.Agents`, `ManagerConfig.Agents` (recovery membaca `r.m.cfg.Agents`), field `Participant` baru, `BuildPeerBrief(t, projectName, brief, wt, role PeerRole)`, `BuildMasterPrompt(t, projects, agentsSection)`, `(*Manager).agentsSection()`.
- `session.SessionSpec.Model`.

Disediakan fitur ini:
- `(*Spawner).Spawn(ctx, threadID, projectQuery, brief string, sel AgentSelector) (SpawnResult, error)` kini benar-benar memakai `sel` (tanda tangan tidak berubah dari Foundation).
- Tool MCP `thread_spawn` dengan args `{project, brief, agent?, category?}`.
- `AgentToolsSection()` memuat `thread_spawn(project, brief, agent?, category?)`.
- `func (m *Manager) InUse(agentID string) (threadTitle string, inUse bool)` di akhir `agents_types.go`.
- Helper baru internal paket (awalan `spawn`/`recovery`/`worktreeRecords`): lihat Langkah.
- Perubahan perilaku penyimpanan: `worktrees.json` boleh memuat lebih dari satu rekaman untuk `ProjectID` yang sama (beda `ProjectSlug`), satu per agent. `UpsertWorktreeRecord` berkunci `(ProjectID, ProjectSlug)`. Pembaca lain (`close.go`, `orphans.go`) hanya iterasi rekaman sehingga tetap benar.

## Langkah

Kerjakan berurutan; `go build ./internal/thread/` setelah tiap berkas besar.

### 1. `internal/thread/worktree_records.go`: rekaman per project+slug

Alasan: sebelumnya rekaman berkunci `ProjectID` saja; dua agent di project yang sama akan saling menimpa. `close.go` memanggil `DeleteWorktreeRecord(threadDir, rec.ProjectID)` setelah tiap `RemoveWorktree` sukses (file itu tidak boleh diubah), jadi tanda tangannya harus tetap.

- `UpsertWorktreeRecord`: ganti pencocokan `recs[i].ProjectID == rec.ProjectID` menjadi `recs[i].ProjectID == rec.ProjectID && recs[i].ProjectSlug == rec.ProjectSlug`. Rekaman lama (slug sama seperti slug project) tetap cocok saat diperbarui.
- `DeleteWorktreeRecord(threadDir, projectID string)`: tanda tangan tetap. Semantik baru, tulis di komentar: hapus rekaman ber-`ProjectID` tersebut yang worktree-nya sudah tidak ada di disk (`!worktreeExists(r.Info.Path)`; `Info.Path == ""` juga dihapus). Rekaman yang worktree-nya masih ada dipertahankan. Untuk kasus satu rekaman per project (perilaku lama) hasilnya sama karena `close.go` menghapus rekaman hanya setelah `RemoveWorktree` sukses. Tetap idempoten dan tidak menulis ulang berkas bila tidak ada yang berubah.
- Tambah `func DeleteWorktreeRecordSlug(threadDir, projectID, slug string) error`: hapus tepat satu rekaman berkunci `(projectID, slug)`, tanpa melihat disk; idempoten; kunci `worktreeRecordsMu` sama seperti fungsi lain. Dipakai rollback spawn.
- Tambah `func worktreeRecordsSlugTaken(recs []WorktreeRecord, slug string) bool`: true bila ada rekaman dengan `ProjectSlug == slug` (slug dipakai sebagai nama direktori worktree, jadi unik per thread).

### 2. `internal/thread/spawn.go`: konfigurasi dan resolusi agent

Impor tambahan: `internal/agents`.

a. Tipe internal:

```go
// spawnAgent hasil resolusi agent untuk satu pemanggilan Spawn.
type spawnAgent struct {
    Agent    agents.Agent    // kosong bila !HasAgent
    Category agents.Category // kosong bila !HasAgent
    HasAgent bool
}
```

b. `func (sp *Spawner) spawnResolveAgent(sel AgentSelector) (spawnAgent, error)` (tanpa memanggil provider; validasi provider dilakukan di langkah create supaya jalur reuse tidak butuh provider):

- Normalisasi: `sel.Agent`/`sel.Category` di-`strings.TrimSpace`.
- `sp.cfg.Agents == nil`:
  - sel kosong -> `spawnAgent{}` (perilaku lama).
  - sel terisi -> galat `fmt.Errorf("%w: registry agent tidak tersedia; kosongkan agent/category pada thread_spawn", ErrInvalid)` (pilihan eksplisit tidak boleh diabaikan diam-diam).
- sel kosong: `a, c, err := Resolve("executor")`.
  - `err == nil` -> `{a, c, true}`.
  - `err != nil && agentAbsent(sp.cfg.Agents, "executor", err)` -> `spawnAgent{}` (perilaku lama).
  - lainnya -> `fmt.Errorf("agent default Executor: %w", err)`.
- hanya `Agent`: `Resolve(Agent)`; galat -> `fmt.Errorf("agent %q: %w", sel.Agent, err)`.
- hanya `Category`: `Resolve(Category)`; galat -> `fmt.Errorf("kategori %q: %w", sel.Category, err)` (galat "kategori tanpa default" dari resolver sudah jelas; jangan fallback).
- keduanya: `Resolve(Agent)`; lalu cocokkan `strings.EqualFold(sel.Category, c.ID) || strings.EqualFold(sel.Category, c.Name)`; tidak cocok -> `fmt.Errorf("%w: agent %q berkategori %q (%s), tidak cocok dengan category %q", ErrInvalid, a.Name, c.Name, c.ID, sel.Category)`.
- Semua galat eksplisit dikembalikan apa adanya (tanpa `ErrNoAgent` dan tanpa fallback); sesi tidak dibuat karena panggilan ini terjadi sebelum worktree/sesi.

c. `NewSpawner`: tidak ada perubahan (`Agents` opsional, nil aman). Hati-hati nil interface: cek `sp.cfg.Agents == nil` saja.

### 3. `internal/thread/spawn.go`: alur `Spawn`

Ubah isi `Spawn` (tanda tangan sudah dari Foundation; hapus `_ = sel`):

1. Setelah `spawnResolveProject` dan `managerMaster`, panggil `ag, err := sp.spawnResolveAgent(sel)`; galat -> return (sebelum reuse, sebelum worktree/sesi).
2. `agentID := ""` bila `!ag.HasAgent`, selain itu `ag.Agent.ID`.
3. Reuse: kondisi menjadi `p.Role == RolePeer && p.ProjectID == project.ID && p.AgentID == agentID` (peserta lama tanpa `AgentID` hanya cocok dengan pilihan "tanpa agent"). Sisa blok reuse tidak berubah.
4. Setelah cek `hasMaster`: tentukan provider dan model.
   - `ag.HasAgent`: `prov, err := agentProvider(ag.Agent)` (galat -> return, tanpa fallback); `model := ag.Agent.Model`.
   - selain itu: `spawnResolveProvider()` seperti lama; `model := ""`.
5. Slug worktree (agar dua agent tidak berebut satu worktree). Hitung `projSlug := Slugify(project.Name)`. Baca `recs, _ := LoadWorktreeRecords(threadDir)` (abaikan galat baca; galat dianggap tak ada rekaman). `slug := projSlug`. Bila `worktreeRecordsSlugTaken(recs, slug)`:
   - bila `ag.HasAgent`: `slug = Slugify(project.Name + "-" + ag.Agent.Name)`;
   - bila masih `worktreeRecordsSlugTaken`: tambahkan sufiks `-2`, `-3`, ... (loop sampai bebas, batas 20; lewat batas -> galat), memotong dasar bila lewat batas `worktreeSlugMax` agar tetap valid (cukup: hitung `base := slug`, coba `Slugify(base + "-" + strconv.Itoa(i))`).
   `threadDir` harus dideklarasikan sebelum langkah ini (pindahkan `threadDir := mgr.Store().ThreadDir(threadID)` ke atas). Peserta lama tanpa agent yang sudah memakai `projSlug` tidak terganggu.
6. `CreateWorktree(project.Path, sp.cfg.DataDir, threadID, slug)` memakai `slug` di atas (branch otomatis `thread/<threadID>-<slug>`, direktori `<dataDir>/threads/<threadID>/<slug>`; `worktree.go` tidak perlu diubah karena sudah menerima slug bebas dan menangani bentrok nama branch).
7. `rollback`: ganti `DeleteWorktreeRecord(threadDir, project.ID)` dengan `DeleteWorktreeRecordSlug(threadDir, project.ID, slug)`.
8. `WorktreeRecord{ProjectID, ProjectName, ProjectSlug: slug, Info: wt}` tidak berubah selain `slug`.
9. `Sessions.Create(session.SessionSpec{..., ProviderID: prov.ID(), Model: model})`.
10. Penamaan peserta: `Namer` dulu; fallback `<agent.Name>-<projSlug>` bila `ag.HasAgent`, selain itu `prov.ID() + "-" + projSlug` (bukan slug sufiks worktree).
11. `part := Participant{SessionID, ProjectID, Role: RolePeer, Name: name, ProviderID: prov.ID(), Model: model}`; bila `ag.HasAgent` isi juga `AgentID: ag.Agent.ID`, `AgentName: ag.Agent.Name`, `Category: ag.Category.ID`. (`ProviderID` selalu dicatat, juga untuk jalur lama, supaya recovery stabil.)
12. Brief: `role := PeerRole{}`; bila `ag.HasAgent`: `PeerRole{CategoryName: ag.Category.Name, Text: ag.Category.Role}`. Kirim `BuildPeerBrief(t, project.Name, brief, wt, role)`.
13. Pesan sistem `"master men-spawn "+project.Name` tambah keterangan agent bila ada: `"master men-spawn <project> (agent <nama>, <kategori>)"`.
14. `SpawnResult` tidak berubah bentuk.

### 4. `internal/thread/spawn.go`: tool `thread_spawn`

- `InputSchema.properties` tambah:
  - `"agent": {"type":"string","description":"Opsional. Nama agent global yang dipakai (lihat daftar agent di prompt). Bila diisi bersama category, kategori agent harus cocok."}`
  - `"category": {"type":"string","description":"Opsional. Id atau nama kategori (mis. planner, researcher, executor); memakai agent default kategori itu."}`
  - `required` tetap `["project","brief"]`.
- `Description` tool: tambahkan kalimat bahwa `agent` dan `category` opsional; keduanya kosong memakai agent default Executor bila ada.
- Handler: struct input tambah `Agent string json:"agent"` dan `Category string json:"category"`; panggil `sp.Spawn(ctx, c.ThreadID, in.Project, in.Brief, AgentSelector{Agent: in.Agent, Category: in.Category})`.
- Pesan hasil: bila `res.Participant.AgentName != ""` sebut agent dan kategori, mis. `peserta %s (project %s, agent %s, kategori %s) dibuat; worktree: ...`; untuk reuse: `agen %s untuk project %s sudah ada; ...` (boleh tambah agent). Tanpa agent: teks lama persis.

### 5. `internal/thread/mcp_tools.go`

Ubah satu baris di `AgentToolsSection`: `- thread_spawn(project, brief, agent?, category?): men-spawn agent peer di project workspace, di git worktree dan branch milik thread (auto-approve). agent = nama agent, category = id/nama kategori (mis. planner); keduanya opsional, kosong berarti agent default Executor. Project non-git ditolak; spawn ulang project yang sama dengan agent yang sama memakai peserta yang sudah ada, agent berbeda membuat peserta baru.` Baris lain tidak diubah. `mcp.go` dan `store.go` tidak berubah (schema tool didaftarkan dari `spawn.go`; `store.go` menyimpan struct apa adanya sehingga field baru ikut serialisasi dan berkas lama tetap terbaca). Verifikasi dengan membaca bahwa tidak ada salinan field-per-field `Participant` di `store.go`/`mcp.go` yang membuang field baru; bila ada, tambahkan field.

### 6. `internal/thread/recovery.go`

Impor tambahan: `internal/provider`.

a. Helper:

```go
// recoveryProvider memvalidasi provider tercatat: terdaftar dan tersedia. Tanpa fallback.
func recoveryProvider(id string) (provider.Provider, error)
```
Galat Indonesia: `provider tercatat "<id>" tidak terdaftar` / `provider tercatat "<id>" tidak tersedia: <sebab>`.

b. Helper peran:

```go
// recoveryPeerRole memulihkan PeerRole dari kategori terkini; resolver nil/tak ada kategori -> PeerRole{}.
func (r *Recovery) recoveryPeerRole(old Participant) PeerRole
```
Bila `old.Category == ""` atau `r.m.cfg.Agents == nil` -> `PeerRole{}`; selain itu iterasi kategori dari `r.m.cfg.Agents.List()` (nilai kembalian pertama) dan cocokkan `Category.ID == old.Category`; ketemu -> `PeerRole{CategoryName: c.Name, Text: c.Role}`.

c. `respawnOne`: tambah variabel `model string` dan `provID string` ke blok `var (...)`.
- Master: bila `old.ProviderID != ""` -> `prov, err := recoveryProvider(old.ProviderID)`; galat -> `fail(err)`; `model = old.Model`. Bila kosong -> `resolveMasterProvider()` seperti lama. Prompt: `BuildMasterPrompt(t, projects, r.m.agentsSection()) + "\n\n" + BuildRecoveryContext(...)` (pastikan sudah demikian dari Foundation).
- Peer: sama; `ProviderID` tercatat -> `recoveryProvider`; kosong -> `spawnResolveProvider()`. Rekaman worktree dicari lewat helper baru `recoveryFindRecord(recs []WorktreeRecord, old Participant) *WorktreeRecord`: urutan pencocokan (1) `rec.SessionID` cocok dengan `old.SessionID` atau salah satu `old.PreviousSessionIDs` (pakai `recoveryMatch`), (2) fallback: bila tepat SATU rekaman ber-`ProjectID == old.ProjectID`, pakai itu (kompatibel thread lama), (3) selain itu nil. Pesan galat "worktree tidak ditemukan" tetap seperti lama. Prompt: `BuildPeerBrief(t, peerRec.ProjectName, brief, peerRec.Info, r.recoveryPeerRole(old))`.
- `Sessions.Create(session.SessionSpec{..., ProviderID: providerID, Model: model})`.
- Provider tercatat gagal -> `fail(...)` memakai `RecoveryFailedPrefix` (sudah ditangani `fail`), tanpa membuat sesi.
- `UpdateThread` yang menyalin peserta: field agent (`AgentID`, `AgentName`, `Category`, `ProviderID`, `Model`) otomatis terwarisi karena peserta dimutasi in-place (hanya `SessionID`/`Dead`/`PreviousSessionIDs` diubah). Tambahkan backfill untuk peserta lama: bila `th.Participants[i].ProviderID == ""` isi dengan `providerID` hasil resolusi logika lama. Jangan menimpa nilai yang sudah ada.
- Hal lain di `respawnOne` tidak berubah.

### 7. `internal/thread/agents_types.go`: `InUse` (tambah di AKHIR berkas saja)

```go
// InUse melaporkan apakah agentID dipakai thread aktif (peserta Dead tetap dihitung karena bisa dihidupkan ulang).
func (m *Manager) InUse(agentID string) (threadTitle string, inUse bool)
```
- `agentID == ""` -> `("", false)`.
- Iterasi `m.managerAllThreads()` (baca saja; jangan memegang kunci thread atau memanggil `UpdateThread`/`Get` berkunci). Untuk thread `State == StateActive`, cek tiap `Participant.AgentID == agentID`; ketemu -> `(t.Title, true)`; judul kosong -> pakai `t.ID`. Thread `closed` dilewati. Tambah impor yang dibutuhkan hanya bila belum ada. Jangan mengubah isi berkas yang sudah ada.

### 8. Verifikasi

- `go build ./...` (dari root repo) lulus; `go vet ./internal/thread/` bersih.
- `grep -rn "\.Spawn(\|BuildPeerBrief(\|BuildMasterPrompt(" --include='*.go' .` (abaikan direktori `worktree/`): semua pemanggil memakai tanda tangan baru.
- Tinjau diff: tanpa `Agents` (resolver nil) dan tanpa `agent`/`category`, jalur spawn sama seperti sebelumnya (provider claude lalu codex, model kosong, slug project polos, nama `<provider>-<slug>`), selain `ProviderID` kini ikut tercatat di peserta.
- Tidak ada penyentuhan berkas di luar Path yang dimiliki (`git status`).

## Perubahan

Diubah:
- `internal/thread/spawn.go`: `spawnAgent`, `spawnResolveAgent`, alur `Spawn` (resolusi agent, validasi provider, reuse per project+agent, slug worktree unik per agent, `Model` di `SessionSpec`, field agent di `Participant`, `PeerRole` di brief, rollback slug-aware), schema+handler tool `thread_spawn`.
- `internal/thread/mcp_tools.go`: teks `AgentToolsSection`.
- `internal/thread/worktree_records.go`: kunci `(ProjectID, ProjectSlug)` pada `UpsertWorktreeRecord`, semantik baru `DeleteWorktreeRecord`, tambah `DeleteWorktreeRecordSlug`, `worktreeRecordsSlugTaken`.
- `internal/thread/recovery.go`: `recoveryProvider`, `recoveryPeerRole`, `recoveryFindRecord`, provider+model tercatat, `Model` di `SessionSpec`, backfill `ProviderID`.
- `internal/thread/agents_types.go`: `(*Manager).InUse` di akhir berkas.

Tidak diubah (dicek saja): `internal/thread/mcp.go`, `internal/thread/store.go`, `internal/thread/worktree.go`.

Dibuat: tidak ada berkas baru.

Catatan untuk laporan: `close.go` (di luar ownedPaths) tetap memanggil `DeleteWorktreeRecord(threadDir, rec.ProjectID)`; semantik baru ("hapus yang worktree-nya sudah hilang") membuatnya tetap benar untuk beberapa rekaman per project tanpa perubahan di sana.

## Wiring

Fitur ini TIDAK menyentuh `cmd/shikigamid/wire_thread.go`. Yang diekspos untuk langkah Wiring (overview bagian 5):
- `(*Manager).InUse(agentID string) (string, bool)`: dipakai guard `agentStore.SetDeleteGuard(...)` untuk galat `agent "<nama>" dipakai thread aktif "<judul>"`.
- Pemasok resolver tidak baru: `SpawnerConfig.Agents` diisi lewat `AgentsConfig.Agents` (Foundation) dan recovery membaca `ManagerConfig.Agents`; keduanya diisi Wiring dengan resolver yang sama.
- Tool `thread_spawn` dan `AgentToolsSection` sudah terpasang lewat `InstallAgents`/`RegisterTool` yang ada; tidak ada registrasi baru.

## Selesai bila

- `go build ./...` dari root repo lulus (guard utama; tanpa unit test) dan `go vet ./internal/thread/` bersih.
- `thread_spawn` menerima `agent` dan `category`; keduanya kosong -> `Resolve("executor")`, tak ada agent -> perilaku lama; pilihan eksplisit gagal (agent tak dikenal, kategori tanpa default, kategori tidak cocok, provider tak terdaftar/tak tersedia/tanpa MCP) -> galat tool jelas tanpa membuat worktree/sesi.
- Peer baru: `SessionSpec` memuat `ProviderID` dan `Model` agent; `Participant` menyimpan `AgentID`, `AgentName`, `Category`, `ProviderID`, `Model`; brief memuat bagian `Peranmu (<Kategori>):` bila agent dipakai.
- Reuse hanya bila `ProjectID` dan `AgentID` sama; agent berbeda di project sama membuat peserta + worktree baru dengan slug unik (tidak ada dua sesi di worktree yang sama), dan `worktrees.json` menyimpan keduanya.
- Recovery memakai `ProviderID`+`Model` tercatat, gagal jelas (`RecoveryFailedPrefix`) bila provider tercatat hilang, peserta lama tanpa `ProviderID` memakai logika lama, field agent terwarisi, `PeerRole` dipulihkan dari kategori terkini.
- `Manager.InUse` ada di akhir `agents_types.go` sesuai kontrak.
- Tidak ada berkas di luar Path yang dimiliki yang berubah (terutama `wire_thread.go`, `manager.go`, `master.go`, `launch.go`, `prompt.go`).
