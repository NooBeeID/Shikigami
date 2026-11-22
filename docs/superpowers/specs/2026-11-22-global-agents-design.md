# Agent global (kategori, profil, default) — desain

Status: disetujui user secara lisan untuk dikerjakan langsung (2026-11-22), pendekatan 2 "profil dengan peran". Ukuran: proyek (tiga bagian berurutan).

## Maksud
Sekarang provider (`claude`, `codex`, `shell`) tidak mengenal model, dan agent di thread atau mention selalu "provider polos". User ingin mendefinisikan agent global sekali di Settings (kategori, provider, model), menjadikannya default per kategori, lalu memakainya di mana saja: lewat mention di sesi mana pun, dan oleh Orchestrator di thread yang men-spawn agent sesuai konteks (mis. Planner di project A untuk merancang CI/CD Jenkins).

## Tujuan
- Halaman Settings "Agents": kelola kategori dan agent. Empat kategori bawaan: Orchestrator, Researcher, Planner, Executor. User dapat menambah, mengubah, menghapus kategori sendiri dan membuat agent di kategori mana pun.
- Agent = nama, kategori, provider, model (kosong = default provider). Satu agent per kategori ditandai default.
- Kategori punya teks **peran** yang dapat diedit (mis. Planner: hanya merencanakan, tidak mengubah kode). Teks peran disisipkan ke brief agent yang di-spawn.
- Bagian 1: pengelolaan di Settings, penyimpanan, dan flag model di provider.
- Bagian 2: `@nama-agent` atau `@kategori` di mention men-spawn sesi dari agent bila belum ada sesinya, lalu mengirim pesan.
- Bagian 3: master thread memakai agent Orchestrator default; `thread_spawn` menerima `agent` (nama) atau `category`; master mendapat daftar agent dan peran di prompt-nya sehingga memilih berdasarkan konteks.

## Bukan tujuan
- Prompt, batasan tool, atau izin per agent (pendekatan 3 ditolak: belum ada kebutuhan).
- Agent per workspace atau per project; v1 global saja.
- Sinkronisasi antar mesin.
- Provider baru. Daftar model tidak diambil otomatis dari provider.

## Anti-tujuan
- Agent terpilih diam-diam jatuh ke provider lain saat binary hilang; harus error yang jelas.
- Orchestrator dengan provider tanpa MCP (master tak bisa memanggil `thread_spawn`).
- Menghapus kategori atau agent yang sedang dipakai thread aktif merusak thread itu.
- Model tidak valid membuat sesi mati senyap; kegagalan peluncuran harus terlihat.

## Batasan
- Codex `Capabilities.MCP == false`, provider `shell` tanpa agent: Orchestrator hanya boleh berprovider yang MCP-nya `true` (saat ini Claude). Validasi di backend, dan dropdown UI menyaring.
- `appconfig.Patch` hanya mengenal `zoom` dan `window` (kunci lain diabaikan), jadi agent tidak disimpan di `config.json`.
- `session.SessionSpec` belum punya field model; `Launcher` meneruskan ke `LaunchContext`, dan `Command` tiap provider yang menambahkan flag.
- Komentar, log, galat, teks UI: bahasa Indonesia. Identifier: bahasa Inggris. Settings memakai registry (`registerSettingsPage`), tanpa mengubah `SettingsModal`.

## Pendekatan
- 1. Tipis (provider+model, kategori hanya label). Ditolak: Planner dan Executor tidak berbeda perilaku, Orchestrator hanya menebak dari nama.
- **2. Profil dengan peran. Dipilih.** Kategori membawa teks peran; teks itu juga bahan Orchestrator memilih agent.
- 3. Penuh (prompt, tool, izin per agent). Ditolak: berlebihan untuk kebutuhan sekarang.

## Desain

### Model data dan penyimpanan (Bagian 1, backend)
Paket baru `internal/agents`, file `<DataDir>/agents.json` (0600, tulis atomik seperti `appconfig`):
```
Category{ id, name, role, builtin bool }
Agent{ id, name, categoryId, providerId, model, isDefault bool }
```
- Seed bila berkas kosong: empat kategori bawaan (`builtin: true`, tidak dapat dihapus, nama dan peran boleh diubah) dengan teks peran awal; tanpa agent. Kategori tanpa default berarti tidak bisa dipakai otomatis (error jelas, bukan menebak).
- Invarian: nama agent unik (huruf kecil, slug); paling banyak satu default per kategori (menandai default mencabut yang lama); agent pertama di kategori menjadi default otomatis.
- Validasi: provider harus terdaftar dan bukan `shell`; kategori `orchestrator` hanya provider ber-MCP; menghapus kategori yang masih punya agent ditolak kecuali dengan opsi memindahkan/menghapus agent-nya.
- IPC: `agents.list` (kategori + agent), `agents.categoryUpsert`, `agents.categoryDelete`, `agents.upsert`, `agents.delete`, `agents.setDefault`; event `agents.changed`.
- Resolver `Resolve(ref string) (Agent, Category, error)`: ref dapat berupa nama agent atau id/nama kategori (kategori → default-nya). Satu-satunya tempat aturan ini hidup; mention dan thread memanggilnya.

### Model di provider (Bagian 1, backend)
- `SessionSpec.Model string`; `LaunchContext` membawa model; `claudeProvider.Command` menambah `--model <m>`, `codexProvider.Command` menambah `-m <m>` bila tidak kosong (dijamin tak duplikat seperti `withFlag`). Model kosong = perilaku sekarang.

### Halaman Settings "Agents" (Bagian 1, frontend)
Modul `frontend/src/modules/settings/agents/` didaftarkan `registerSettingsPage({ id: 'agents', title: 'Agent', order: 15 })` dengan `client.ts` + `store.ts` (pola `connectors`, `loadSeq` untuk respons basi).
- Kiri: daftar kategori (+ tombol "Kategori baru"). Kanan: peran kategori terpilih (textarea) dan daftar agent kategori itu (nama, provider, model, penanda default, edit/hapus) + "Agent baru".
- Form agent: nama, provider (dropdown dari `provider.list`, hanya yang tersedia dan sesuai aturan Orchestrator), model (teks bebas dengan placeholder "default provider"), jadikan default.
- Aksesibilitas: mengikuti pola tablist/modal yang ada; status default tidak hanya lewat warna.

### Mention spawn (Bagian 2)
- Di `mention.Resolve`: bila target tak cocok dengan sesi mana pun, coba `agents.Resolve`. Cocok → buat sesi di project konteks pengirim (workspace yang sama) dengan provider+model agent, beri brief peran + pesan, lalu antrekan seperti biasa. Sesi yang sudah ada dengan nama itu tetap diprioritaskan (tidak spawn ganda).
- Mode konfirmasi mention tetap berlaku: agent yang meminta spawn lewat mention agent mengikuti `confirm_agent`.
- Tidak ada project konteks (pengirim tanpa project): galat jelas.
- Detail pemilihan project, penamaan sesi (`agent-project` lewat `Namer`), dan UI autocomplete `@` ditentukan di plan.

### Orchestrator di thread (Bagian 3)
- `Manager` memulai master dengan agent Orchestrator default (provider+model dari `agents.Resolve("orchestrator")`); tanpa agent Orchestrator, perilaku sekarang (Claude polos) dipertahankan dan ditandai di UI.
- `thread_spawn` mendapat `agent` dan `category` opsional (default: kategori Executor bila ada, selain itu perilaku lama). `Spawner` memakai agent terpilih untuk provider+model, dan `BuildPeerBrief` menyisipkan teks peran kategori.
- `BuildMasterPrompt` memuat daftar agent (nama, kategori, peran satu baris) dan petunjuk memilihnya berdasarkan konteks.
- Peserta menyimpan `agentId`; reuse peserta per project tetap, tetapi agent berbeda di project yang sama membuat peserta baru.
- Pemulihan thread (`recovery`) memakai provider+model yang dicatat saat spawn, bukan default saat ini, agar konsisten setelah restart.

## Kriteria penerimaan
- Bagian 1: user membuat agent Planner (claude, model X) di Settings, menandainya default; restart aplikasi, data tetap ada. Sesi diluncurkan dengan `--model X`. Orchestrator tidak dapat berprovider codex.
- Bagian 2: `@planner` tanpa sesi bernama itu membuka sesi baru dengan provider+model default Planner dan mengirim pesan beserta peran; `@planner` kedua memakai sesi yang sama.
- Bagian 3: thread dengan Orchestrator default memulai master dengan model-nya; dari goal "bikin plan CI/CD di project A", master memanggil `thread_spawn` dengan kategori Planner dan peer berjalan memakai provider+model+peran Planner.
- Menghapus agent yang dipakai thread aktif ditolak atau diberi peringatan; thread berjalan tidak rusak.

## Dibiarkan ke builder
Teks peran awal empat kategori, bentuk persis galat IPC, tata letak visual halaman (ikut gaya `ConnectorsPage`), dan strategi migrasi bila skema `agents.json` berubah.
