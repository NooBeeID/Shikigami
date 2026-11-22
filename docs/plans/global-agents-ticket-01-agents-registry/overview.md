# Overview: Agent Global, Registry dan Settings (tiket 01)

Tiket: `docs/tickets/global-agents-ticket-01-agents-registry.md`
Spec: `docs/superpowers/specs/2026-11-22-global-agents-design.md`
Module Go: `github.com/NooBeeID/shikigami`

## 1. Ringkasan epic dan urutan eksekusi

Tiket ini membangun fondasi agent global: registry kategori+agent persisten (`<DataDir>/agents.json`), jalur `Model` dari `SessionSpec` ke flag CLI provider, dan halaman Settings "Agent". Tiket 02 (mention spawn) dan 03 (orchestrator) memakai `Store.Resolve`, IPC `agents.*`, dan `SessionSpec.Model`.

Urutan eksekusi:

1. Foundation: TIDAK ADA. Kontrak (tipe, IPC, nama berkas) sudah tetap di dokumen ini dan di tiket; ketiga fitur tidak saling mengimpor kode pada saat dikerjakan. (`hasFoundation=false`, tidak ada `00-foundation.md`.)
2. Fitur paralel: `agents-store-ipc`, `provider-model-flag`, `agents-settings-page`.
3. Wiring: satu agen menyambungkan sisa (lihat bagian 5), lalu build guard.

## 2. Kontrak bersama

### 2.1 Tipe (JSON camelCase)

```
Category { id, name, role string; builtin bool }
Agent    { id, name, categoryId, providerId, model string; isDefault bool }
```

Go (`internal/agents`, tag JSON persis):

```go
type Category struct {
    ID      string `json:"id"`
    Name    string `json:"name"`
    Role    string `json:"role"`
    Builtin bool   `json:"builtin"`
}
type Agent struct {
    ID         string `json:"id"`
    Name       string `json:"name"`
    CategoryID string `json:"categoryId"`
    ProviderID string `json:"providerId"`
    Model      string `json:"model"`
    IsDefault  bool   `json:"isDefault"`
}
```

TypeScript (`frontend/src/modules/settings/agents/client.ts`): `Category`, `Agent`, `AgentsSnapshot { categories: Category[]; agents: Agent[] }` dengan field yang sama.

Berkas `agents.json`: `{ "version": 1, "categories": [...], "agents": [...] }`, mode 0600, tulis atomik (temp di direktori sama, fsync, rename; tiru `appconfig.write`).

Aturan data:
- Id kategori bawaan: `orchestrator`, `researcher`, `planner`, `executor` (`builtin=true`, tidak bisa dihapus, nama/peran boleh diubah). Kategori buatan: id = slug nama + sufiks unik bila bentrok.
- Nama agent: slug `[a-z0-9-]`, unik global. Normalisasi: lowercase, spasi menjadi `-`; kosong atau karakter ilegal = galat.
- Paling banyak satu default per kategori; agent pertama di kategori otomatis default; mengubah `categoryId` mengikuti aturan default tujuan.
- Provider agent: terdaftar di `provider.Get`, bukan `shell`; kategori `orchestrator` hanya provider dengan `Capabilities().MCP == true`.

### 2.2 IPC daemon (request/response)

| Method | Param | Hasil |
|---|---|---|
| `agents.list` | - | `{categories: Category[], agents: Agent[]}` (array tidak pernah null) |
| `agents.categoryUpsert` | `{id?, name, role}` | `Category` |
| `agents.categoryDelete` | `{id, agents?: 'move'\|'delete', moveTo?: string}` | `{}` |
| `agents.upsert` | `{id?, name, categoryId, providerId, model, isDefault}` | `Agent` |
| `agents.delete` | `{id}` | `{}` |
| `agents.setDefault` | `{id}` | `{}` |

Event: topic `agents.changed`, payload `{}`, dipublikasikan lewat `srv.Publish("agents.changed", map[string]any{})` (`internal/ipc/server.go`) setelah tiap mutasi sukses. Galat backend berbahasa Indonesia dan dikembalikan apa adanya ke UI.

### 2.3 Go API `internal/agents`

```go
type ProviderLookup interface { Get(id string) (provider.Provider, bool) }
func DefaultPath(dataDir string) string            // <dataDir>/agents.json
func Open(path string, reg ProviderLookup) *Store  // reg nil => registry global provider.Get
func (s *Store) List() ([]Category, []Agent)       // salinan; kategori urut dibuat, agent urut nama
func (s *Store) Resolve(ref string) (Agent, Category, error)
func (s *Store) SetDeleteGuard(fn func(agentID string) error)
func RegisterIPC(srv *ipc.Server, s *Store)
var ErrNotFound = errors.New(...)                  // ref tak dikenal; dibedakan lewat errors.Is
```

Urutan `Resolve`: (1) nama agent persis (setelah slug); (2) id kategori; (3) nama kategori case-insensitive; kategori dipetakan ke agent default-nya; tanpa default/agent = galat "kategori <nama> belum punya agent default". Layanan: `d.Provide("agents.store", store)` (tipe `*agents.Store`).

### 2.4 Sesi dan provider

- `session.SessionSpec.Model string` (kosong = default provider; JSON `model,omitempty` bila struct itu ber-tag JSON, ikuti gaya yang ada; `SessionSpec` saat ini tanpa tag, jadi cukup tambah field).
- `provider.LaunchContext.Model string`.
- Claude: `--model <m>`; codex: `-m <m>`; tanpa duplikat bila `ctx.Args` sudah memuat `--model`/`-m`/`--model=...` (Args yang sudah ada menang, dokumentasikan di komentar). Model kosong = args byte per byte sama dengan sekarang. Helper flag bernilai di `internal/provider/launch.go` (serupa `withFlag`).
- `Launcher` meneruskan `Spec.Model` ke `LaunchContext.Model`.

### 2.5 Frontend

- `registerSettingsPage({ id: 'agents', title: 'Agent', order: 15, component: AgentsPage })` di `frontend/src/modules/settings/agents/index.ts`.
- Klien memakai `call` dari `frontend/src/lib/ipc` (lihat `connectors/client.ts`). Data dimuat saat halaman pertama dibuka, bukan saat startup.
- Modul lain tidak boleh mengimpor dari `modules/settings/agents`.
- Teks UI, komentar, galat: bahasa Indonesia; identifier: bahasa Inggris.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `agents-store-ipc` | Paket Go `internal/agents`: tipe, penyimpanan atomik 0600, seed, invarian, validasi, `Resolve`, `SetDeleteGuard`, `RegisterIPC` + event; plus modul daemon `wire_agents.go` | `internal/agents/`, `cmd/shikigamid/wire_agents.go` |
| `provider-model-flag` | `Model` di `SessionSpec` dan `LaunchContext`, diteruskan `Launcher`, flag claude/codex, helper tanpa duplikat, pastikan keluar awal sesi terlihat | `internal/session/types.go`, `internal/session/launcher.go`, `internal/provider/launch.go`, `internal/provider/claude.go`, `internal/provider/codex.go` |
| `agents-settings-page` | Modul Svelte halaman Settings Agent: `client.ts`, `store.ts` (`loadSeq`, event `agents.changed`), `AgentsPage.svelte`, form/dialog agent dan kategori, `index.ts` | `frontend/src/modules/settings/agents/` |

Detail perilaku dan acceptance criteria per fitur ada di bagian "Fitur" tiket; plan per fitur harus mengikutinya.

## 4. Foundation

Tidak ada. Alasan: tidak ada skeleton paket atau tipe yang harus dibagi lebih dulu; tipe didefinisikan di bagian 2 dan masing-masing fitur menulis sisinya sendiri (Go di `agents-store-ipc`, TS di `agents-settings-page`). `agents-store-ipc` hanya memakai `provider.Get` dan `Capabilities` yang sudah ada, jadi tidak menunggu `provider-model-flag`.

## 5. Wiring checklist

Dikerjakan setelah ketiga fitur selesai:

1. Backend: pastikan `cmd/shikigamid/wire_agents.go` (dimiliki `agents-store-ipc`) mendaftarkan `RegisterModule("agents", ...)` lewat `init()`, memanggil `agents.Open(agents.DefaultPath(d.DataDir), nil)`, `d.Provide("agents.store", store)`, dan `agents.RegisterIPC(srv, store)`. File `wire_*.go` di package `main` otomatis ikut build; `modules.go` tidak diubah.
2. Frontend: tambahkan tepat satu baris `import './agents';` di `frontend/src/modules/settings/index.ts` (setelah impor yang ada). Tidak ada perubahan lain di berkas itu; `SettingsModal.svelte` tidak disentuh. Posisi tab ditentukan `order: 15` (antara Connector 10 dan MCP 20).
3. Peluncuran sesi: verifikasi `Launcher` meneruskan `Spec.Model` ke `LaunchContext.Model` dan Command claude/codex memakainya (baca kode; ubah hanya bila fitur belum melakukannya).
4. Jalankan build guard (bagian 6). Bila memungkinkan, uji manual: Settings tab "Agent" tampil, buat agent Planner (claude, model X) default, restart, data tetap; Orchestrator + codex ditolak (dropdown tidak menawarkan, backend menolak).

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- `go build ./...` (dari root repo)
- Frontend: `cd frontend && npm run build` dan `npm run check` (svelte-check)

## 7. Aturan paralel

- Tiap agen hanya menyentuh `ownedPaths` fiturnya. Dilarang menyentuh path fitur lain, `internal/session`/`internal/provider` (selain `provider-model-flag`), `appconfig`, `SettingsModal.svelte`, `modules.go`, dan `frontend/src/modules/settings/index.ts` (itu milik wiring).
- Jangan menambah unit test.
- `agents-settings-page` bergantung hanya pada kontrak IPC bagian 2.2, bukan pada implementasi Go; `agents-store-ipc` tidak mengubah `internal/session` maupun `internal/provider`.
- Build per fitur: fitur Go cukup `go build ./...` (ketergantungan lintas fitur tidak ada); fitur frontend `npm run build` + `npm run check`.
- Bila menemukan kebutuhan di luar ownedPaths, catat di laporan; jangan edit.
