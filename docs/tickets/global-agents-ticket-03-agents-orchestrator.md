---
id: global-agents-ticket-03-agents-orchestrator
title: Agent Global: Orchestrator di Thread
spec: docs/superpowers/specs/2026-11-22-global-agents-design.md
status: implemented
wave: 3
depends_on: [global-agents-ticket-01-agents-registry, global-agents-ticket-02-agents-mention-spawn]
---
# Agent Global: Orchestrator di Thread

## Tujuan
Bagian 3 dari spec agent global. Setelah registry agent (tiket 1) dan mention spawn (tiket 2) ada, thread masih memakai "Claude polos" untuk master dan semua peer. Epic ini membuat master thread dijalankan oleh agent Orchestrator default (provider+model dari registry), membuat tool MCP `thread_spawn` bisa memilih agent atau kategori (mis. Planner untuk merancang CI/CD di project A), membuat master tahu daftar agent beserta perannya lewat prompt, dan membuat peserta serta pemulihan thread konsisten dengan provider+model yang tercatat. Penghapusan agent yang sedang dipakai thread aktif ditolak agar thread berjalan tidak rusak.

## Hasil yang bisa dijalankan
Setelah tiket ini selesai (di atas tiket 1 dan 2), `go build ./...` dan build frontend lulus, aplikasi berjalan, dan:
- Dengan agent Orchestrator default (mis. `orch`, claude, model X) di Settings > Agent, thread baru memulai master dengan `--model X`. Header thread menampilkan agent dan model master.
- Tanpa agent Orchestrator, thread baru tetap jalan dengan Claude polos seperti sekarang, dan header thread menampilkan penanda "tanpa agent Orchestrator (Claude polos)".
- Prompt master memuat daftar agent (nama, kategori, peran satu baris) dan petunjuk memilih berdasarkan konteks. Dari goal "bikin plan CI/CD di project A", master memanggil `thread_spawn` dengan `category: "planner"`, dan peer berjalan dengan provider+model Planner serta brief yang memuat teks peran Planner.
- `thread_spawn` tanpa `agent`/`category` memakai default kategori Executor bila ada, selain itu perilaku lama.
- Baris peserta di panel peserta menampilkan agent dan kategori (mis. "planner · Planner"), dan model bila ada.
- Agent berbeda di project yang sama menjadi peserta terpisah; agent yang sama memakai ulang peserta lama.
- Setelah daemon restart, peserta yang mati dihidupkan ulang dengan provider+model yang dicatat saat spawn, bukan default saat ini.
- Menghapus agent yang dipakai thread aktif (dari Settings > Agent) ditolak dengan pesan galat yang menyebut thread-nya; thread yang berjalan tidak terganggu. Agent yang tidak dipakai tetap bisa dihapus.

## Batasan
In scope:
- Paket `internal/thread/`: pemilihan provider+model master, prompt master, parameter `thread_spawn`, field agent di `Participant`/`Thread`, pemulihan, `InUse`.
- `cmd/shikigamid/wire_thread.go`: memasang `AgentResolver` dan guard hapus.
- `frontend/src/modules/thread/`: tampilan agent/model/kategori dan penanda tanpa Orchestrator.

Out of scope:
- Mengubah `internal/agents/`, `internal/mention/`, `internal/session/`, `internal/provider/`, atau `frontend/src/modules/settings/agents/` (milik tiket lain; hanya dipakai lewat kontrak).
- Halaman Settings Agent: galat penolakan hapus tampil lewat pesan galat IPC yang sudah ada.
- Prompt/tool/izin per agent, agent per project, sinkronisasi antar mesin.
- Fallback diam-diam ke provider lain bila binary agent hilang atau provider tanpa MCP dipilih sebagai master: harus galat jelas.
- Unit test.

## Kontrak
Dipakai dari tiket 1 (jangan diubah):
- IPC/tipe: `Category{id,name,role,builtin}`, `Agent{id,name,categoryId,providerId,model,isDefault}`; id kategori bawaan `orchestrator`, `researcher`, `planner`, `executor`; event `agents.changed`; IPC `agents.list` -> `{categories, agents}` (frontend thread memanggilnya lewat klien sendiri).
- Go: `agents.Store` dengan `Resolve(ref string) (agents.Agent, agents.Category, error)` (ref = nama agent atau id/nama kategori -> default-nya; tanpa default = galat jelas), `List() ...`, `SetDeleteGuard(func(agentID string) error)`; layanan `d.Lookup("agents.store")`.
- Sesi: `session.SessionSpec.Model string` (kosong = default provider); provider claude/codex sudah menerjemahkan ke `--model`/`-m` (tiket 1).

Dipakai dari tiket 2: tidak ada API baru; hanya pola `AgentResolver` yang sama (antarmuka kecil didefinisikan sendiri per paket).

Disediakan epic ini:
1. Go, `internal/thread/agents_types.go` (hanya ditambah):
   ```go
   // AgentResolver dipenuhi *agents.Store; didefinisikan di sini agar tak ada siklus modul.
   type AgentResolver interface {
       Resolve(ref string) (agents.Agent, agents.Category, error)
       List() ([]agents.Agent, []agents.Category) // sesuaikan dengan bentuk List() sebenarnya di tiket 1
   }
   ```
   Bila impor `internal/agents` dari `internal/thread` membuat siklus, definisikan tipe cermin lokal (`AgentInfo`, `CategoryInfo`) dan adaptor di `wire_thread.go`. Pilih satu; jangan ubah `internal/agents`.
2. `ManagerConfig.Agents AgentResolver` (opsional; nil = perilaku lama penuh). Setter `SpawnerConfig.Agents` / `AgentsConfig.Agents` meneruskan resolver yang sama.
3. Field baru (semua `omitempty`, JSON camelCase) pada `Participant`: `AgentID string json:"agentId"`, `AgentName string json:"agentName"`, `Category string json:"category"` (id kategori), `ProviderID string json:"providerId"`, `Model string json:"model"`. Pada `Thread`: `NoOrchestrator bool json:"noOrchestrator,omitempty"` (true bila master berjalan tanpa agent Orchestrator).
4. Tool MCP `thread_spawn` args: `{project: string, brief: string, agent?: string, category?: string}` (`agent` = nama agent; `category` = id/nama kategori). Kedua kosong -> Executor default bila ada, selain itu perilaku lama. Keduanya diisi -> `agent` dipakai, galat bila kategorinya tidak cocok dengan `category`.
5. `func (m *Manager) InUse(agentID string) (threadTitle string, inUse bool)`: true bila ada thread berstatus `active` dengan peserta (master atau peer, termasuk yang `dead`) ber-`AgentID == agentID`. Guard hapus mengubahnya menjadi galat Indonesia: `agent "<nama>" dipakai thread aktif "<judul>"`.
6. Frontend `modules/thread/types.ts`: `Participant` mendapat `agentId?, agentName?, category?, providerId?, model?`; `Thread` mendapat `noOrchestrator?`. Tidak ada IPC thread baru.

## Fitur

### thread-master-agent: Master dengan agent Orchestrator default
Deskripsi:
- `resolveMasterProvider` (master.go) diganti alur baru di `SpawnMaster`: bila `Agents != nil`, panggil `Agents.Resolve("orchestrator")`. Sukses -> provider = `provider.Get(agent.ProviderID)`, model = `agent.Model`. Wajib: provider terdaftar, `Available()` nil, dan `Capabilities().MCP == true`; bila tidak, kembalikan galat jelas (mis. `agent orchestrator "x": provider "codex" tidak mendukung MCP, master tidak bisa memanggil thread_spawn`) tanpa fallback ke provider lain. Galat tampil ke user lewat jalur `managerSpawnSafe` / `Create` yang sudah ada.
- Bila `Resolve("orchestrator")` mengembalikan galat karena tidak ada agent Orchestrator (default tidak ada), pertahankan perilaku sekarang (`resolveMasterProvider`: claude lalu codex) dan set `Thread.NoOrchestrator = true` lewat `UpdateThread` sebelum thread dipublikasikan (`thread.updated`). Bedakan "tidak ada agent" dari galat lain (mis. buat sentinel lokal atau cek `List()`); galat lain tetap menggagalkan, bukan fallback.
- `session.SessionSpec{... ProviderID, Model}` diisi dari agent. `Participant` master menyimpan `AgentID`, `AgentName`, `Category: "orchestrator"`, `ProviderID`, `Model` (kosong untuk Claude polos kecuali `ProviderID`).
- `launch.go`: pastikan kontribusi MCP thread tetap hanya memakai `ctx.Provider.Capabilities().MCP` (sudah demikian); tambahkan log peringatan bila sesi di cwd thread berprovider tanpa MCP. Tidak mengubah perilaku lain.
- `BuildMasterPrompt` (prompt.go): tambah parameter/varian untuk daftar agent. Perubahan tanda tangan harus memperbarui semua pemanggil (manager.go, recovery.go). Bagian baru "Agent yang tersedia": satu baris per agent `- <nama> (kategori <Nama Kategori>)<, default> : <peran satu baris>` (peran = `Category.role` dipadatkan ke satu baris dan dipotong, mis. 160 rune), diikuti petunjuk: pilih agent atau kategori berdasarkan sifat pekerjaan (perencanaan -> Planner, riset -> Researcher, pengubahan kode -> Executor), isi `agent` atau `category` pada `thread_spawn`, dan bila tidak yakin biarkan kosong. Tanpa agent terdaftar bagian ini dihilangkan (perilaku lama). Data diambil dari `Agents.List()`; bila resolver nil atau galat, lewati bagian itu tanpa menggagalkan thread.
- Teks `AgentToolsSection` (mcp_tools.go, milik fitur sebelah) diperbarui oleh fitur `thread-spawn-agent`; fitur ini tidak menyentuhnya.

Acceptance criteria:
- [ ] Master baru dengan agent Orchestrator default diluncurkan dengan provider+model agent itu (terlihat di argumen proses `--model`).
- [ ] Tanpa agent Orchestrator, master tetap Claude polos dan `Thread.noOrchestrator == true` terlihat di `thread.get`/`thread.list`.
- [ ] Orchestrator berprovider tanpa MCP atau provider tak tersedia menghasilkan galat jelas dan thread tidak berjalan setengah jadi (tidak ada sesi yatim; sesi yang sudah dibuat di-kill).
- [ ] Prompt master memuat daftar agent (nama, kategori, peran satu baris) dan petunjuk pemilihan; tanpa agent, prompt sama seperti sebelumnya.
- [ ] `Participant` master menyimpan `agentId`, `providerId`, `model`.
- [ ] `go build ./...` lulus.

ownedPaths: `internal/thread/manager.go`, `internal/thread/master.go`, `internal/thread/launch.go`, `internal/thread/prompt.go`, `cmd/shikigamid/wire_thread.go` (hanya bagian pemasangan resolver; bagian guard dikerjakan fitur `thread-spawn-agent`, jadi koordinasikan satu blok `AgentResolver` bersama).

Paralel: bisa dikerjakan paralel dengan `thread-spawn-agent` dan `thread-agent-ui`. Titik temu: field `Participant`/`Thread` baru di types.go dan antarmuka `AgentResolver` di agents_types.go dimiliki `thread-spawn-agent`; fitur ini boleh memakainya setelah field itu ada (kerjakan definisi tipe itu lebih dulu bila berjalan bersamaan, cukup beberapa baris).

### thread-spawn-agent: thread_spawn dengan agent/category, peserta agentId, recovery
Deskripsi:
- Definisi tipe: tambah `AgentResolver`, field `Participant`/`Thread` baru (lihat Kontrak) di `agents_types.go` dan `types.go`; `store.go` menyimpannya apa adanya (JSON `omitempty`, berkas lama tetap terbaca).
- `thread_spawn` (spawn.go `RegisterTool`, schema input; mcp_tools.go/mcp.go hanya bila registrasi skema perlu diperluas): tambah properti `agent` (string, nama agent) dan `category` (string). Deskripsi tool menyebut keduanya opsional. Perbarui `AgentToolsSection` (mcp_tools.go) menjadi `thread_spawn(project, brief, agent?, category?)`.
- `Spawner.Spawn` menerima agent terpilih (ubah tanda tangan menjadi `Spawn(ctx, threadID, projectQuery, brief string, sel AgentSelector)` dengan `AgentSelector{Agent, Category string}`; perbarui handler tool dan pemanggil lain). Resolusi: `agent` -> `Resolve(agent)`; `category` -> `Resolve(category)`; keduanya -> agent dipakai dan kategorinya harus cocok; kosong -> `Resolve("executor")`, bila galat "tidak ada default/kategori" jatuh ke perilaku lama (`spawnResolveProvider`: claude lalu codex, model kosong), galat lain dikembalikan. Pilihan eksplisit yang gagal (agent tak dikenal, kategori tanpa default, provider tak tersedia) dikembalikan sebagai galat tool yang jelas; tidak fallback diam-diam. Orchestrator sebagai agen peer tidak dilarang, tetapi provider tanpa MCP ditolak karena peer perlu `thread_send`/`thread_read`.
- Provider yang dipilih agent wajib `Available()` dan MCP true; `session.SessionSpec` diisi `ProviderID` dan `Model`. Penamaan peserta tetap lewat `Namer`; fallback `<provider>-<slug>` diganti `<agent>-<slug>` bila agent dipakai.
- `Participant` peer menyimpan `AgentID`, `AgentName`, `Category`, `ProviderID`, `Model`.
- Reuse: cocokkan `ProjectID` dan `AgentID`. Agent sama di project sama -> reuse (perilaku lama). Agent berbeda di project sama -> peserta baru (worktree sudah ada untuk project itu: pakai ulang rekaman worktree atau buat worktree/branch tambahan dengan slug unik `<project>-<agent>`; putuskan sesuai `worktree.go` agar tidak ada dua sesi berebut worktree yang sama, bila tidak aman gunakan worktree baru). Peserta lama tanpa `AgentID` (thread lama) dianggap cocok hanya dengan pilihan "tanpa agent" (perilaku lama).
- `BuildPeerBrief`: tambah parameter `rolePrompt string` (teks `Category.role`); bila tidak kosong, sisipkan bagian "Peranmu (<Nama Kategori>):" sebelum "Tugas dari master:". Perbarui pemanggil di recovery.go. Peer ber-peran Planner tidak mengubah kode: aturan itu datang dari teks peran, bukan dari kode.
- Recovery (recovery.go `respawnOne`): master dan peer dihidupkan ulang dengan `ProviderID` dan `Model` yang tercatat di `Participant` (bukan `resolveMasterProvider`/`spawnResolveProvider`/default agent saat ini). Provider tercatat tak tersedia -> `fail(...)` dengan galat jelas (tanpa fallback). Peserta lama tanpa `ProviderID` memakai logika lama. Sesi pengganti mewarisi `AgentID`, `AgentName`, `Category`, `ProviderID`, `Model`. Teks peran untuk peer dipulihkan dari kategori terkini via `Agents.Resolve`/`List` berdasarkan `Category` bila tersedia, bila tidak kosong. `BuildMasterPrompt` untuk master dipulihkan dengan daftar agent terkini.
- Guard hapus: `Manager.InUse(agentID)` (lihat Kontrak) memeriksa semua thread aktif lewat `managerAllThreads()`. Di `wire_thread.go`, setelah `agents.store` didapat: `store.SetDeleteGuard(func(id string) error { if title, ok := m.InUse(id); ok { return fmt.Errorf(...) }; return nil })`. Guard tidak boleh memegang kunci thread terlalu lama atau menyebabkan deadlock dengan `agents`; baca-saja. Peserta berstatus `dead` pada thread aktif tetap dihitung pakai karena bisa dihidupkan ulang. Thread `closed` tidak menghalangi.

Acceptance criteria:
- [ ] `thread_spawn` dengan `category: "planner"` men-spawn peer dengan provider+model default Planner; brief peer memuat teks peran Planner.
- [ ] `thread_spawn` dengan `agent: "<nama>"` memakai agent itu; agent tak dikenal atau kategori tanpa default menghasilkan galat tool yang jelas, tanpa spawn sesi.
- [ ] `thread_spawn` tanpa `agent`/`category` memakai default Executor bila ada, selain itu perilaku lama (tanpa regresi pada thread tanpa registry).
- [ ] `Participant` menyimpan `agentId`, `providerId`, `model` dan lolos serialisasi ulang; `thread.json` lama tetap terbaca.
- [ ] Agent berbeda di project sama membuat peserta baru; agent sama di project sama memakai ulang peserta.
- [ ] Setelah restart daemon, peserta mati dihidupkan dengan provider+model yang tercatat; provider tercatat hilang menghasilkan pesan sistem `RecoveryFailedPrefix` yang jelas.
- [ ] `agents.delete` pada agent yang dipakai thread aktif ditolak dengan galat yang menyebut nama thread; agent yang tidak dipakai bisa dihapus; thread berjalan tidak rusak.
- [ ] `go build ./...` lulus.

ownedPaths: `internal/thread/mcp_tools.go`, `internal/thread/mcp.go`, `internal/thread/spawn.go`, `internal/thread/types.go`, `internal/thread/store.go`, `internal/thread/recovery.go`, `internal/thread/agents_types.go`, serta pemasangan guard di `cmd/shikigamid/wire_thread.go` (blok terpisah dari resolver).

Paralel: bisa dikerjakan paralel dengan `thread-agent-ui` (kontrak field JSON sudah tetap di bagian Kontrak). Dengan `thread-master-agent`: kerjakan definisi tipe/field lebih dulu (lihat catatan di sana), sisanya independen; perubahan `BuildPeerBrief`/`BuildMasterPrompt` yang memengaruhi recovery.go harus selaras dengan tanda tangan baru dari fitur master.

### thread-agent-ui: Tampilan agent di thread
Deskripsi:
- `types.ts`: tambah field baru pada `Participant` dan `Thread` (lihat Kontrak). `thread-store.ts`/`thread-client.ts`: pastikan field baru ikut dalam normalisasi/penyalinan thread dari `thread.get`/`thread.list`/event `thread.updated`; field yang tak ada diperlakukan sebagai kosong (thread lama). Hanya bila diperlukan memetakan id kategori ke nama: panggil IPC `agents.list` lewat klien thread sendiri (jangan impor dari `modules/settings/agents`), cache sederhana, muat ulang pada event `agents.changed`; kegagalan memuat tidak boleh merusak tampilan (jatuh ke id kategori mentah).
- `participants/ParticipantRow.svelte`: baris `sub` menampilkan `role · agentName · kategori` bila `agentName` ada, plus model bila ada (mis. `peer · api-gateway · planner (Planner) · claude-opus`); tanpa agent tampilan lama. Teks tidak hanya lewat warna; potong dengan ellipsis dan `title` penuh.
- `ThreadHeader.svelte`: tampilkan provider+model master (dari peserta master: `agentName`, `providerId`, `model`) sebagai teks kecil/badge. Bila `thread.noOrchestrator` true, tampilkan penanda teks "tanpa agent Orchestrator (Claude polos)" dengan `title` penjelasan (atur di Settings > Agent). Aksesibel (teks, bukan hanya warna).
- Galat penolakan hapus agent: tidak ada UI baru di thread; jika thread menerima galat IPC terkait (mis. spawn gagal karena agent), pesan sistem sudah tampil di log thread lewat `PostSystem`. Pastikan pesan sistem baru dari backend ("gagal men-spawn ...: agent ...") tampil biasa di `MessageItem` tanpa perubahan khusus.
- Gaya mengikuti komponen yang ada (`--fg`, `--fg-muted`, ukuran font yang sama).

Acceptance criteria:
- [ ] Baris peserta menampilkan agent, kategori, dan model bila field ada; thread lama tanpa field tampil seperti sebelumnya tanpa galat konsol.
- [ ] Header menampilkan provider/model master dan penanda "tanpa agent Orchestrator (Claude polos)" tepat saat `noOrchestrator` true.
- [ ] Kegagalan `agents.list` tidak merusak halaman thread.
- [ ] Tidak ada impor dari `modules/settings/agents` dan tidak ada perubahan di luar `frontend/src/modules/thread/`.
- [ ] Build frontend lulus tanpa galat tipe.

ownedPaths: `frontend/src/modules/thread/` (khususnya `types.ts`, `thread-store.ts`, `thread-client.ts`, `ThreadHeader.svelte`, `participants/ParticipantRow.svelte`).

Paralel: bisa dikerjakan paralel dengan dua fitur lain; hanya bergantung pada nama field JSON di Kontrak, bukan pada kode backend.

## Path yang dimiliki
Hanya path berikut yang boleh disentuh tiket ini:
- `internal/thread/`
- `cmd/shikigamid/wire_thread.go`
- `frontend/src/modules/thread/`

## Wiring
Langkah penutup tiket ini (tidak bergantung pada tiket lain selain dua dependensi yang sudah wave sebelumnya):
1. Di `wireThread` (`cmd/shikigamid/wire_thread.go`), setelah `session.manager` didapat, ambil `d.Lookup("agents.store")`. Bila ada dan bertipe `*agents.Store` (atau memenuhi `thread.AgentResolver`), teruskan ke `thread.ManagerConfig.Agents`, `thread.AgentsConfig.Agents` (Spawner), dan `thread.NewRecovery` via manager. Bila tidak ada, catat peringatan `thread: agents.store tidak tersedia, thread memakai Claude polos` dan lanjut tanpa resolver (aplikasi tetap jalan).
2. Setelah `thread.NewManager` terbentuk, pasang guard: `store.SetDeleteGuard(...)` memanggil `m.InUse`. Hanya dipasang bila store ada; urutan pemasangan tidak boleh menunda `m.FireStart()`.
3. Pastikan `thread.RegisterIPC`, `InstallAgents`, `InstallLaunchContribution`, `RegisterClose`, recovery, dan aktivitas tetap terpasang tanpa perubahan urutan (tanda tangan `Spawn`/`BuildPeerBrief`/`BuildMasterPrompt` yang berubah harus diikuti di semua pemanggil).
4. Frontend: tidak ada registrasi baru; modul thread yang sudah terdaftar memakai tipe dan komponen yang diperluas. Verifikasi manual: buat agent Orchestrator + Planner + Executor di Settings, buat thread, periksa header, minta master men-spawn Planner, restart daemon dan kirim pesan ke peer, lalu coba hapus agent yang dipakai.

## Guard
- `go build ./...` berhasil.
- Build frontend berhasil (`npm run build` / perintah build di `frontend/`, tanpa galat tipe `svelte-check` bila ada).
- Tanpa unit test.
