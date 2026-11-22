# Overview: Agent Global, Mention Spawn (tiket 02)

Tiket: `docs/tickets/global-agents-ticket-02-agents-mention-spawn.md`
Spec: `docs/superpowers/specs/2026-11-22-global-agents-design.md`
Module Go: `github.com/NooBeeID/shikigami`
Depends on: `global-agents-ticket-01-agents-registry` (store agents, IPC `agents.*`, `SessionSpec.Model`). Overview tiket 1: `docs/plans/global-agents-ticket-01-agents-registry/overview.md`.

## 1. Ringkasan epic dan urutan eksekusi

`@nama-agent` atau `@kategori` di composer mention (dan `to` pada tool MCP `send_message`) men-spawn sesi baru dari agent global (provider + model + brief peran) di project konteks pengirim bila belum ada sesinya, lalu pesan diantrekan lewat `Queue` yang ada. Sesi existing (nama biasa, atau sesi hasil spawn berlabel agent) selalu diprioritaskan sehingga tidak ada spawn ganda. Autocomplete `@` di UI menampilkan agent dan kategori ber-default.

Urutan eksekusi:

1. Foundation: TIDAK ADA (`hasFoundation=false`, tidak ada `00-foundation.md`). Kedua fitur hanya bertemu di kontrak IPC `mention.send` (bagian 2.4) yang sudah tetap di dokumen ini; sisi Go dan sisi TS tidak saling mengimpor.
2. Fitur paralel: `mention-agent-resolve` (Go) dan `mention-agent-autocomplete` (frontend).
3. Wiring: pemeriksaan sambungan akhir (bagian 5), lalu build guard (bagian 6).

Catatan: `internal/agents` belum ada di working tree saat overview ini ditulis (dikerjakan tiket 1). Plan tidak boleh menambalnya; bila saat implementasi `agents.Store`/`SessionSpec.Model` belum ada, itu pelanggaran dependensi: hentikan dan laporkan.

## 2. Kontrak bersama

### 2.1 Dari tiket 1 (jangan didefinisikan ulang)

- Go: `agents.Agent{ID, Name, CategoryID, ProviderID, Model string; IsDefault bool}`, `agents.Category{ID, Name, Role string; Builtin bool}`, `(*agents.Store).Resolve(ref string) (agents.Agent, agents.Category, error)`, `agents.ErrNotFound` (ref tak dikenal; dibedakan lewat `errors.Is`). Layanan `d.Lookup("agents.store")` bertipe `*agents.Store`.
- `session.SessionSpec.Model string` (kosong = default provider).
- IPC `agents.list` -> `{categories: Category[], agents: Agent[]}`; event `agents.changed` (payload `{}`).

### 2.2 Go, paket `internal/mention` (disediakan fitur `mention-agent-resolve`)

```go
// AgentResolver dipenuhi *agents.Store. Satu-satunya tempat aturan resolusi agent.
type AgentResolver interface {
    Resolve(ref string) (agents.Agent, agents.Category, error)
}
// SessionSpawner dipenuhi *session.Manager.
type SessionSpawner interface {
    Create(spec session.SessionSpec) (*session.Session, error)
}
// ProjectDirectory opsional (antarmuka kedua di samping Directory); dipenuhi workspaceDirectory.
type ProjectDirectory interface {
    ProjectDir(workspaceID, projectID string) string
}
// SpawnLabels: registri in-memory sessionID -> agentName, aman konkuren.
// Dibaca Namer: sesi berlabel bernama <agent>-<project> (slug, duplikat -2, -3, ...)
// menggantikan <provider>-<project>. Dibersihkan lewat OnSessionExit.
type SpawnLabels struct{ /* map + mutex */ }

type ProjectContext struct {
    WorkspaceID, ProjectID, Cwd, FromSessionID string
}
func (...) ResolveOrSpawn(ctx context.Context, pc ProjectContext, ref string, text string, source string) (m Message, spawned bool, err error)
```

Bentuk persis `ResolveOrSpawn` (receiver, apakah mengantre sekaligus) ditetapkan oleh plan `mention-agent-resolve`, tetapi syaratnya: dipanggil dari `mention.send` (ipc.go) dan `toolSendMessage` (tools.go) menggantikan `Namer.Resolve` langsung; mengembalikan id sesi tujuan, `spawned`, dan galat; antre memakai `Queue.Enqueue` yang sama. Field baru: `QueueConfig.Agents AgentResolver`, `QueueConfig.Spawner SessionSpawner`, serta `Runtime.Agents`/`Runtime.Spawner` agar tool MCP ikut men-spawn. Bila `Agents == nil`, alur agent dilewati (perilaku lama, tanpa panik).

Impor: `internal/mention` mengimpor `internal/agents` hanya untuk tipe. Jika ternyata siklik, definisikan tipe lokal ringan dan adaptasi di `wire_mention.go`.

### 2.3 Aturan resolusi target (urutan baku)

1. `Namer.Resolve(workspaceId, ref)` seperti sekarang (id sesi, `provider-project`, `agent-project`). `ErrCrossWorkspace` tetap berlaku (tidak jatuh ke spawn).
2. Sesi berlabel agent: sesi running di workspace yang sama dengan label `agentName == ref` (atau sama dengan `Agent.name` hasil resolusi kategori); prioritas project konteks, lalu project lain di workspace yang sama.
3. `AgentResolver.Resolve(ref)`: cocok -> spawn; galat dari resolver diteruskan apa adanya; `agents.ErrNotFound` -> `ErrNotFound` mention seperti sekarang.

Spawn: `SessionSpec{WorkspaceID, ProjectID, Cwd, ProviderID: agent.ProviderID, Model: agent.Model}` (Cols/Rows default); catat `SpawnLabels` sebelum antre; serialkan per kunci `workspace|project|agentName` dengan mutex peta; coba ulang `Enqueue` sampai sesi `Running` (maks ~2 dtk, interval 100 ms). Mode: `user_only` + sumber agent -> `ErrUserOnly` SEBELUM spawn; `confirm_agent` -> spawn tetap segera, pesan pertama `awaiting_confirm` oleh `Queue.Enqueue` (beri komentar bahwa sesi kosong menunggu konfirmasi). Gagal spawn -> "Gagal membuka sesi agent @<ref>: <sebab>", label dibersihkan. Tanpa project konteks -> `ErrInvalid` dengan teks "Tidak ada project konteks untuk membuka sesi agent @<ref>; pilih project dulu", lolos `toIPCError` sebagai `ipc.ErrInvalid` dengan pesan sama, tanpa spawn.

Brief (pesan pertama):
```
[Peran: <Category.name>]
<Category.role>

--- Pesan ---
<teks pesan>
```

### 2.4 IPC `mention.send` (titik temu Go dan frontend)

Params (tambahan, semua opsional): `projectId string` (project aktif pengirim UI). Params lama (`workspaceId`, `to`, `text`/`body`, `from`) tidak berubah.
Hasil: `Message` yang ada, ditambah field opsional `spawned bool` dan `spawnedSession string` (nama sesi, mis. `planner-shikigami`), lewat struct yang menyematkan `Message` (JSON digabung/flat, `omitempty`). `mention.names` tidak berubah. Frontend memperlakukan kedua field sebagai opsional.
Untuk jalur MCP `send_message`: konteks project dari `ProjectID`/`Cwd` sesi pemanggil (`caller` di tools.go diperluas); deskripsi parameter `to` menyebut bahwa nama agent atau kategori boleh dan akan membuka sesi baru bila belum ada.

### 2.5 Frontend, `frontend/src/modules/mention/`

- `types.ts`: `AgentInfo {id,name,categoryId,providerId,model,isDefault}`, `CategoryInfo {id,name,role,builtin}`, `MentionSuggestion` = `{kind:'session', ...NameInfo}` | `{kind:'agent', name, categoryName, providerId, model, isDefault, hasSession}` | `{kind:'category', name, defaultAgent, hasSession}`.
- `mention-client.ts`: `listAgents()` (IPC `agents.list`, klien sendiri milik modul), `buildSuggestions(names, agents, categories)` murni (sesi, lalu agent, lalu kategori ber-default; tanpa duplikasi nama bentrok; `hasSession` = ada sesi bernama berawalan `<nama>-`), `matchAgentRef()` murni, `onAgentsChanged(fn)` via `onEvent('agents.changed', ...)`, `sendMention(ws, name, body, projectId)`.
- Dilarang mengimpor dari `modules/settings/agents` atau `modules/thread`.
- Teks UI, komentar, galat: bahasa Indonesia; identifier: bahasa Inggris.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `mention-agent-resolve` | Backend Go: `AgentResolver`/`SessionSpawner`/`ProjectDirectory`, `SpawnLabels` + integrasi `Namer`, `ResolveOrSpawn` (urutan resolusi, spawn, brief, anti-ganda, mode konfirmasi, galat Indonesia), param `projectId` dan respons `spawned` di `mention.send`, konteks project di `send_message`, deskripsi tool; wiring `wireMention` | `internal/mention/`, `cmd/shikigamid/wire_mention.go` |
| `mention-agent-autocomplete` | Frontend: tipe, `listAgents`/`buildSuggestions`/`matchAgentRef`/`onAgentsChanged`, `MentionComposer.svelte` (saran agent/kategori dengan label "agent baru", validasi submit, kirim `projectId`, feedback spawn, `errorText`, placeholder baru) | `frontend/src/modules/mention/` |

Detail perilaku dan acceptance criteria per fitur ada di bagian "Fitur" tiket; plan per fitur harus mengikutinya.

## 4. Foundation

Tidak ada. Tipe lintas-fitur hanya kontrak JSON bagian 2.4 dan 2.5; Go dan TS masing-masing menulis sisinya sendiri. Dependensi ke tiket 1 sudah tersedia sebelum tiket ini dimulai.

## 5. Wiring checklist

Sebagian besar wiring ada di path milik fitur (`wire_mention.go` milik `mention-agent-resolve`). Setelah kedua fitur selesai, agen wiring memeriksa dan, bila ada yang kurang, melengkapinya (hanya di `ownedPaths` tiket ini):

1. `cmd/shikigamid/wire_mention.go`, dalam `wireMention`: `d.Lookup("agents.store")`, cast ke `mention.AgentResolver`; bila tidak ada, `d.Logger.Warn(...)` dan fitur agent nonaktif (mention lama jalan). Set `QueueConfig.Agents`, `QueueConfig.Spawner: mgr`, dan `Runtime.Agents`/`Runtime.Spawner` agar tool MCP ikut spawn.
2. `workspaceDirectory` juga memenuhi `mention.ProjectDirectory` (`ProjectDir` dari `workspace.list`; periksa nama field path sebenarnya di `internal/workspace`, mis. `Path string \`json:"path"\``; perluas struct `proj` dan cache).
3. Buat `SpawnLabels`, berikan ke `Namer` (`NewNamer` atau setter) dan ke `Queue`/spawn, daftarkan pembersihan label lewat `hub.add` (dan `Runtime.OnSessionExit`) saat sesi keluar.
4. Panggil `mention.RegisterIPC(srv, q, namer, opts)` dengan opsi baru (param `projectId`, respons `spawned`) dan `mention.Start(ctx, rt)` dengan `Runtime` baru.
5. Frontend: `MentionComposer.svelte` sudah terpasang di aplikasi; pastikan `frontend/src/modules/mention/index.ts` tetap mengekspor registrasi komponen seperti sekarang (tidak ada modul baru yang didaftarkan).
6. Pastikan kontrak 2.4 konsisten dua sisi: nama field JSON `projectId`, `spawned`, `spawnedSession` sama persis di Go (tag) dan TS.
7. Verifikasi manual bila memungkinkan: Settings > Agent buat `planner` (claude, default); `@planner tulis rencana` membuka `planner-<project>`; `@planner` kedua tidak membuat sesi ketiga; `@kategoritanpadefault` galat jelas; daftar `@` menampilkan "agent baru".

## 6. Build guard

Satu-satunya guard, tanpa unit test baru:

- `go build ./...` (dari root repo; cek juga tidak ada impor siklik `internal/agents` <-> `internal/mention`)
- Frontend: `cd frontend && npm run build` dan `npm run check` bila skripnya ada

## 7. Aturan paralel

- Tiap agen hanya menyentuh `ownedPaths` fiturnya. `mention-agent-resolve`: `internal/mention/`, `cmd/shikigamid/wire_mention.go`. `mention-agent-autocomplete`: `frontend/src/modules/mention/`.
- Dilarang menyentuh `internal/agents/`, `internal/session/`, `internal/provider/`, `cmd/shikigamid/wire_agents.go`, `frontend/src/modules/settings/agents/` (tiket 1), `internal/thread/`, `cmd/shikigamid/wire_thread.go`, `frontend/src/modules/thread/` (tiket 3), serta file wiring lain di luar `wire_mention.go`.
- Jangan menambah unit test (berkas `*_test.go` yang sudah ada di `internal/mention/` tidak boleh dirusak; jika build test lama pecah karena perubahan signature, sesuaikan seperlunya di dalam paket yang sama).
- Fitur frontend tidak bergantung pada implementasi Go, hanya kontrak 2.4/2.5; fitur Go tidak bergantung pada kode TS.
- Kebutuhan di luar ownedPaths dicatat di laporan; jangan diedit.
