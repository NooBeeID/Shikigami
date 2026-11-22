---
id: global-agents-ticket-02-agents-mention-spawn
title: Agent Global: Mention Spawn
spec: docs/superpowers/specs/2026-11-22-global-agents-design.md
status: ready
wave: 2
depends_on: [global-agents-ticket-01-agents-registry]
---
# Agent Global: Mention Spawn

## Tujuan

Setelah tiket registry, user punya agent global (kategori, provider, model, peran), tetapi mention (`@nama`) masih hanya bisa menyapa sesi yang sudah berjalan. Epic ini membuat `@nama-agent` atau `@kategori` men-spawn sesi baru dari agent itu (provider+model+brief peran) di project konteks pengirim bila belum ada sesinya, lalu mengantre pesan seperti biasa. Sesi existing yang berasal dari agent yang sama diprioritaskan sehingga tidak ada spawn ganda. Autocomplete `@` di UI ikut menampilkan agent dan kategori.

## Hasil yang bisa dijalankan

Di atas tiket 1 (build `go build ./...` dan build frontend lulus, aplikasi jalan):

- Di composer mention, mengetik `@` menampilkan sesi berjalan (seperti sekarang) dan di bawahnya agent global serta kategori yang punya default, dengan penanda "agent baru" bila belum ada sesinya.
- Mengirim `@planner buatkan rencana CI/CD` (agent bernama `planner`, atau kategori `planner` yang punya default) saat belum ada sesinya membuka sesi baru di project aktif dengan provider+model agent (`--model` tersambung lewat `SessionSpec.Model`), lalu pesan berisi teks peran kategori + pesan diantrekan/dikirim. Feedback composer menyebut bahwa sesi baru dibuka.
- `@planner` kedua memakai sesi yang sama (tidak ada sesi ganda).
- Agent (sesi lain) yang memanggil tool MCP `send_message` dengan `to: "planner"` juga men-spawn di project sesi pemanggil; mode `confirm_agent` tetap berlaku (pesan menunggu konfirmasi sebelum spawn dieksekusi).
- Galat jelas berbahasa Indonesia untuk: tidak ada project konteks, kategori tanpa agent default, agent/kategori tidak dikenal, gagal meluncurkan sesi.
- Belum ada (tiket lain): master thread memakai agent Orchestrator dan `thread_spawn` dengan `agent`/`category` (tiket 3). Halaman Settings sudah ada dari tiket 1 dan hanya dipakai sebagai sumber data.

## Batasan

In scope:
- Antarmuka kecil `AgentResolver` di paket `mention`, pencabangan resolusi target ke agent, spawn sesi lewat session manager, pelabelan sesi hasil spawn untuk penamaan dan prioritas sesi existing, brief peran.
- Parameter konteks project pada `mention.send` dan pemakaian `ProjectID`/`Cwd` pengirim pada tool MCP `send_message`.
- Perluasan MentionComposer, `mention-client.ts`, `types.ts` di frontend, termasuk klien `agents.list` milik modul mention.
- Wiring di `cmd/shikigamid/wire_mention.go` (ambil `agents.store` lewat `d.Lookup`).

Out of scope:
- Perubahan `internal/agents/`, `internal/session/`, `internal/provider/` (disediakan tiket 1). Tiket ini hanya mengisi `SessionSpec.Model`.
- Perubahan `internal/thread/` dan modul frontend `thread` / `settings` (tiket 1 dan 3). Jangan mengimpor dari `modules/settings/agents`.
- Penghapusan sesi hasil spawn otomatis, pemulihan label spawn setelah restart daemon (label hanya in-memory; sesi lama yang hidup lewat restart kembali bernama `provider-project` biasa), unit test.

## Kontrak

Dipakai dari tiket 1 (jangan didefinisikan ulang di sini):
- Go: `agents.Store.Resolve(ref string) (agents.Agent, agents.Category, error)` (satu-satunya tempat aturan resolusi: ref = nama agent, atau id/nama kategori menuju default-nya; tanpa default = error jelas). Layanan `d.Lookup("agents.store")` bertipe `*agents.Store`.
- Tipe: `Category{id,name,role,builtin}`, `Agent{id,name,categoryId,providerId,model,isDefault}`.
- IPC: `agents.list` -> `{categories: Category[], agents: Agent[]}`; event `agents.changed`.
- Sesi: `session.SessionSpec.Model string` (kosong = default provider), diteruskan Launcher ke `LaunchContext.Model`.

Disediakan epic ini:
- Go (`internal/mention`):
  - `type AgentResolver interface { Resolve(ref string) (agents.Agent, agents.Category, error) }` (didefinisikan di paket mention; `*agents.Store` memenuhinya). Impor `internal/agents` hanya untuk tipe `Agent`/`Category`; jika `internal/agents` ternyata mengimpor `mention` (seharusnya tidak), definisikan tipe lokal ringan dan adaptasi di `wire_mention.go`.
  - `type SessionSpawner interface { Create(spec session.SessionSpec) (*session.Session, error) }` (dipenuhi `*session.Manager`).
  - `Directory` diperluas (opsional, lewat antarmuka kedua `ProjectDirectory { ProjectDir(workspaceID, projectID string) string }`) untuk mencari cwd project; `workspaceDirectory` di wire_mention.go memenuhinya dari `workspace.list`.
  - Registri label spawn in-memory: `SpawnLabels` (sessionID -> agentName) yang dibaca `Namer` untuk nama dasar `<agent>-<project>` (mis. `planner-shikigami`) menggantikan `<provider>-<project>` bagi sesi berlabel.
  - Fungsi inti `ResolveOrSpawn(ctx, rt/queue-config, ctxInfo{WorkspaceID, ProjectID, Cwd, FromSessionID}, ref string) (sessionID string, spawned bool, err error)`.
  - Field baru `QueueConfig`/`Runtime`: `Agents AgentResolver`, `Spawner SessionSpawner`.
- IPC: `mention.send` menerima tambahan opsional `projectId` (konteks project pengirim UI). Hasil `mention.send` tetap `Message`; ditambah field opsional `spawned bool` dan `spawnedSession string` pada respons (dibungkus: bila perlu `Message` + field tambahan lewat struct anonim yang menyematkan `Message`). `mention.names` tidak berubah; autocomplete agent memakai `agents.list` dari klien mention.
- Frontend (`modules/mention`): `types.ts` menambah `AgentInfo`, `CategoryInfo`, `MentionSuggestion` (union sesi | agent | kategori); `mention-client.ts` menambah `listAgents()` (memanggil `agents.list`), `matchAgentRef()` murni, dan penerusan `projectId` pada `sendMention`.

Aturan resolusi target (urutan baku, diterapkan di `mention.send` dan `send_message`):
1. `Namer.Resolve(workspaceId, ref)` seperti sekarang (id sesi, nama `provider-project`, nama `agent-project`).
2. Sesi berlabel agent: sesi running di workspace yang sama dengan label `agentName == ref` (atau label sama dengan `Agent.name` hasil resolusi kategori) dan, bila ada konteks project, di project yang sama diprioritaskan; sesi project lain di workspace yang sama dipakai bila di project konteks tidak ada.
3. `AgentResolver.Resolve(ref)`: cocok -> spawn. Galat dari resolver diteruskan apa adanya (bahasa Indonesia, sudah jelas). Tidak cocok dan galat "tidak dikenal" -> `ErrNotFound` seperti sekarang.
Hasil `ErrCrossWorkspace` dari langkah 1 tetap berlaku (tidak jatuh ke spawn).

Brief yang diantrekan ke sesi baru: `Category.role` + garis pemisah + pesan asli, contoh:
```
[Peran: <Category.name>]
<Category.role>

--- Pesan ---
<teks pesan>
```
Brief dikirim sebagai pesan pertama lewat antrean yang sama (dikirim saat sesi baru idle), bukan argumen CLI.

## Fitur

### mention-agent-resolve: Resolusi mention ke agent dan spawn sesi

Deskripsi:
- Di `internal/mention`, tambahkan alur `ResolveOrSpawn` yang dipanggil oleh handler `mention.send` (ipc.go) dan `toolSendMessage` (tools.go) menggantikan pemanggilan `Namer.Resolve` langsung. Urutan: sesi existing -> sesi berlabel agent -> `AgentResolver.Resolve` -> spawn.
- Konteks project: untuk `mention.send` dari UI pakai `projectId` dari params (opsional) bersama `workspaceId`; untuk `from` terisi atau tool MCP pakai `ProjectID`/`Cwd` sesi pengirim (`caller` di tools.go diperluas membawa `ProjectID` dan `Cwd`). Cwd project dari `ProjectDirectory.ProjectDir` bila tidak ada sesi pengirim. Tanpa project konteks (ProjectID kosong dan cwd tak ditemukan) kembalikan galat `ErrInvalid` terbungkus teks "Tidak ada project konteks untuk membuka sesi agent @<ref>; pilih project dulu" dan JANGAN spawn. Galat ini harus lolos `toIPCError` sebagai `ipc.ErrInvalid` dengan pesan yang sama.
- Spawn: `Spawner.Create(session.SessionSpec{WorkspaceID, ProjectID, Cwd, ProviderID: agent.ProviderID, Model: agent.Model})` (Cols/Rows default). Setelah sukses catat label di `SpawnLabels` sebelum langkah antre, sehingga `Namer.List`/`NameOf` langsung memberi nama `<agent>-<project>`. Nama berlabel dihitung seperti `baseName` (huruf kecil, non-alfanumerik jadi `-`, duplikat diberi akhiran `-2`, dst) dan ikut `snapshot` yang sama agar konsisten.
- Antre: setelah spawn, `Enqueue(ctx, fromID, newSessionID, brief, source)` dengan `fromID == ""`/`SourceUser` untuk UI dan sesi pengirim/`SourceAgent` untuk MCP. `Enqueue` sudah menolak sesi non-running; bila sesi baru belum `Running` pada saat itu, coba ulang singkat (maks ~2 detik, interval 100 ms) sebelum menyerah dengan galat peluncuran.
- Mode konfirmasi: untuk `SourceAgent` dan mode `confirm_agent`, spawn TIDAK dilakukan sampai dikonfirmasi: pesan masuk antrean `awaiting_confirm` terhadap sesi terdaftar-tertunda, atau (implementasi sederhana yang diterima) spawn tetap dilakukan segera tetapi pesan pertama berstatus `awaiting_confirm` oleh `Queue.Enqueue` seperti biasa. Pilih yang kedua; dokumentasikan dengan komentar bahwa sesi kosong menunggu konfirmasi. Mode `user_only` menolak agent dengan `ErrUserOnly` SEBELUM spawn (periksa mode di awal alur agent-source agar tidak ada sesi yatim).
- Anti spawn ganda: serialkan `ResolveOrSpawn` per kunci `workspace|project|agentName` dengan mutex peta agar dua `@planner` beruntun tidak membuat dua sesi.
- Kegagalan peluncuran (binary hilang, model tidak valid yang membuat proses langsung keluar, `Create` galat): kembalikan galat jelas "Gagal membuka sesi agent @<ref>: <sebab>". Untuk jalur MCP ini otomatis menjadi hasil `isError` ke agen pengirim ("dilaporkan ke pengirim"). Untuk UI menjadi galat IPC yang ditampilkan composer. Bila sesi sempat dibuat namun langsung keluar sebelum pesan terkirim, `HandleExit` yang ada menandai pesan `failed`; pastikan label di `SpawnLabels` dibersihkan lewat `OnSessionExit`.
- Kategori tanpa default atau agent tidak ada: galat dari `Store.Resolve` diteruskan; jangan menebak provider lain (anti-tujuan spec).
- `wire_mention.go`: di `wireMention`, `d.Lookup("agents.store")`, cast ke `mention.AgentResolver`; bila tidak ada, log `Warn` dan fitur agent nonaktif (mention lama tetap jalan). Set `rt`/`QueueConfig` field `Agents`, `Spawner: mgr`, dan jadikan `workspaceDirectory` juga `ProjectDirectory` (ambil `path` project dari `workspace.list`; perluas struct parsing `proj` dengan field path, mis. `Path string \`json:"path"\``, cek nama field sebenarnya di `internal/workspace`).
- Perluas teks deskripsi tool `send_message` (parameter `to`) menyebut bahwa nama agent atau kategori boleh dipakai dan akan membuka sesi baru bila belum ada. `list_sessions` tidak berubah.

Acceptance criteria:
- [ ] `go build ./...` lulus; `mention` tidak membuat impor siklik (`internal/agents` tidak mengimpor `internal/mention`).
- [ ] `mention.send` dengan `to: "planner"` tanpa sesi bernama itu dan dengan `projectId` valid membuat sesi dengan `ProviderID` dan `Model` agent default Planner dan mengantre brief (peran + pesan).
- [ ] `to` kedua kalinya yang sama memakai sesi yang sama (jumlah sesi tidak bertambah).
- [ ] Sesi hasil spawn muncul di `mention.names` bernama `<agent>-<project>`; sesi non-agent tetap bernama `<provider>-<project>`.
- [ ] Nama sesi existing (mis. `claude-shikigami`) tetap diprioritaskan bila ref sama dengan namanya.
- [ ] Tanpa project konteks: galat Indonesia yang jelas, tidak ada sesi baru.
- [ ] Kategori tanpa default: galat dari `Store.Resolve` ditampilkan; tidak ada sesi baru.
- [ ] Jalur MCP `send_message` men-spawn di project sesi pemanggil; hasil `queued`/`delivered`/`position` tetap valid; mode `confirm_agent` menahan pesan, mode `user_only` menolak tanpa spawn.
- [ ] Kegagalan `Create` mengembalikan "Gagal membuka sesi agent ..." dan tidak meninggalkan label/sesi yatim.
- [ ] Bila `agents.store` tidak tersedia, mention lama tetap berfungsi tanpa panik.

ownedPaths: `internal/mention/`, `cmd/shikigamid/wire_mention.go`

Catatan paralel: dapat dikerjakan paralel dengan `mention-agent-autocomplete`. Satu-satunya titik temu adalah bentuk respons `mention.send` (`spawned`, `spawnedSession` opsional) dan parameter `projectId` yang sudah ditetapkan di bagian Kontrak; frontend cukup menganggapnya opsional.

### mention-agent-autocomplete: Autocomplete @ untuk agent di UI

Deskripsi:
- `types.ts`: tambah `AgentInfo {id,name,categoryId,providerId,model,isDefault}`, `CategoryInfo {id,name,role,builtin}`, `MentionSuggestion` = `{kind:'session', ...NameInfo}` | `{kind:'agent', name, categoryName, providerId, model, isDefault, hasSession:boolean}` | `{kind:'category', name, defaultAgent, hasSession:boolean}`.
- `mention-client.ts`: `listAgents(): Promise<{categories: CategoryInfo[]; agents: AgentInfo[]}>` memanggil IPC `agents.list` (modul ini punya klien sendiri; dilarang mengimpor dari `modules/settings/agents`). Fungsi murni `buildSuggestions(names, agents, categories)`: sesi dulu; lalu agent (nama slug); lalu kategori yang punya default (nama kategori huruf kecil slug, sama dengan id bawaan seperti `planner`). Entri agent/kategori yang bentrok nama dengan sesi existing tidak diduplikasi. `hasSession` benar bila ada sesi dengan nama berawalan `<agent>-` atau label agen itu (cukup: nama sesi diawali `<nama>-`, sesuai penamaan hasil spawn). Tambah `onAgentsChanged(fn)` memakai `onEvent('agents.changed', ...)`.
- `MentionComposer.svelte`: muat `listAgents()` bersama `listNames`, segarkan pada `agents.changed`; `filtered` memakai `MentionSuggestion[]` terfilter prefix; item agent/kategori menampilkan `@nama`, `kategori · provider · model` (model kosong tampil "default"), dan label teks "agent baru" bila `!hasSession` (jangan hanya warna; teks tetap terbaca). `aria-selected`, navigasi panah/Enter/Tab/Escape tetap sama; `key` item unik per `kind+nama`.
- Validasi submit: hapus galat "Sesi @x tidak ada di workspace ini" bila `parsed.name` cocok dengan agent atau kategori ber-default pada daftar lokal (selebihnya pertahankan perilaku lama). Bila `listAgents` gagal, jatuh ke perilaku lama (anggap tanpa agent) tanpa merusak daftar sesi.
- Kirim: `sendMention(ws, name, body, projectId)` dengan `projectId` project aktif (ambil dari store aktif di `lib/stores`, cek nama store sebenarnya, mis. project aktif workspace; bila tidak ada kirim kosong dan biarkan backend membalas galat). Pesan feedback: bila respons `spawned` benar tampilkan "Sesi baru @<nama-sesi> dibuka dan pesan diantrekan/terkirim" (pakai `stateLabel` yang ada untuk bagian state); galat backend (project konteks, kategori tanpa default, gagal peluncuran) ditampilkan apa adanya lewat `errorText`, dengan perluasan pemetaan di `errorText` agar pesan Indonesia dari backend tidak tertimpa label generik "Pesan tidak valid (kosong atau beda workspace)": bila pesan mengandung "project konteks", "agent", atau "Gagal membuka sesi", tampilkan pesan aslinya (maks 160 karakter).
- Placeholder composer diubah menjadi "@nama-sesi atau @agent pesan...".

Acceptance criteria:
- [ ] Build frontend lulus (`npm run build` di `frontend/`, termasuk `svelte-check` bila skrip proyek menjalankannya).
- [ ] Mengetik `@` menampilkan sesi, agent, dan kategori ber-default; mengetik `@pla` menyaring ke yang berawalan `pla`.
- [ ] Agent/kategori tanpa sesi menampilkan label teks "agent baru".
- [ ] Mengirim `@planner halo` tidak lagi ditolak oleh validasi lokal "Sesi @planner tidak ada di workspace ini" bila `planner` agent/kategori valid; `@asal-asalan` tetap ditolak.
- [ ] Setelah spawn berhasil feedback menyebut sesi baru dibuka; galat backend tampil jelas dan tidak mengosongkan teks pesan.
- [ ] Daftar agent diperbarui saat event `agents.changed` tanpa reload.
- [ ] Tidak ada impor dari `modules/settings/agents` atau `modules/thread`.
- [ ] Di luar Wails (`inWails` false) perilaku lama (pesan "Hanya tersedia di aplikasi desktop") tetap.

ownedPaths: `frontend/src/modules/mention/`

Catatan paralel: dapat dikerjakan paralel dengan `mention-agent-resolve`; kontrak antar-keduanya hanya parameter `projectId` dan field opsional `spawned`/`spawnedSession` pada respons `mention.send`. Untuk mencoba end-to-end keduanya harus sudah tergabung.

## Path yang dimiliki

Hanya ini yang boleh disentuh:
- `internal/mention/`
- `cmd/shikigamid/wire_mention.go`
- `frontend/src/modules/mention/`

Jangan menyentuh `internal/agents/`, `cmd/shikigamid/wire_agents.go`, `frontend/src/modules/settings/agents/` (tiket 1) maupun `internal/thread/`, `cmd/shikigamid/wire_thread.go`, `frontend/src/modules/thread/` (tiket 3). Bila field yang dibutuhkan ternyata belum ada di kontrak tiket 1 (mis. `SessionSpec.Model`), itu pelanggaran dependensi: hentikan dan laporkan, jangan menambalnya di path lain.

## Wiring

Penutup tiket ini, semuanya di path yang dimiliki:
1. `cmd/shikigamid/wire_mention.go`: dalam `wireMention` (dijalankan lewat `d.OnReady`), ambil `agents.store` dengan `d.Lookup`, pasang sebagai `AgentResolver`, pasang `mgr` sebagai `Spawner`, perluas `workspaceDirectory` agar mengimplementasikan `ProjectDir`, dan daftarkan `SpawnLabels` ke `Namer` serta `hub.add` pembersihan label saat sesi keluar. Daftarkan ulang `mention.RegisterIPC` dengan opsi baru (`projectId`, respons `spawned`) dan `mention.Start(rt)` dengan `Runtime` yang membawa `Agents`/`Spawner` agar tool MCP `send_message` ikut men-spawn.
2. Frontend: `MentionComposer.svelte` sudah dipasang di aplikasi; cukup pastikan versi barunya memuat `agents.list` dan mengirim `projectId`. Tidak ada modul baru yang perlu didaftarkan di `index.ts` selain ekspor yang sudah ada (periksa `frontend/src/modules/mention/index.ts` agar tetap mengekspor registrasi komponen seperti sekarang).
3. Verifikasi manual: jalankan aplikasi; di Settings > Agent buat agent `planner` (claude, model apa saja, default); di composer ketik `@planner tulis rencana`; sesi baru muncul bernama `planner-<project>`; kirim `@planner` lagi, tidak muncul sesi ketiga; `@kategoritanpadefault` menampilkan galat jelas.

Tiket ini tidak bergantung pada tiket 3 untuk berjalan; ia hanya bergantung pada tiket 1 yang sudah ada (store agents, IPC, `SessionSpec.Model`).

## Guard

Cukup build berhasil, tanpa unit test:
- `go build ./...` dari root repo.
- Build frontend (`npm run build` di `frontend/`, dan `npm run check` bila ada).
- Aplikasi dapat dijalankan dan composer mention tampil tanpa galat konsol saat daftar agent kosong maupun terisi.
