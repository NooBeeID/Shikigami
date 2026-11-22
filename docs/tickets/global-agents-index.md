# Global Agents: Index Tiket

Sumber: `docs/superpowers/specs/2026-11-22-global-agents-design.md`

## Ringkasan tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|---|---|---|---|---|
| global-agents-ticket-01-agents-registry | Agent Global: Registry dan Settings | 1 | - | 3 |
| global-agents-ticket-02-agents-mention-spawn | Agent Global: Mention Spawn | 2 | ticket-01 | 2 |
| global-agents-ticket-03-agents-orchestrator | Agent Global: Orchestrator di Thread | 3 | ticket-01, ticket-02 | 3 |

Total: 3 tiket, 8 fitur.

Fitur per tiket:

- **ticket-01 (Registry dan Settings):** agents-store-ipc, provider-model-flag, agents-settings-page
- **ticket-02 (Mention Spawn):** mention-agent-resolve, mention-agent-autocomplete
- **ticket-03 (Orchestrator di Thread):** thread-master-agent, thread-spawn-agent, thread-agent-ui

## Urutan eksekusi per wave

Wave yang sama bisa dikerjakan paralel. Setiap wave di sini hanya berisi satu tiket, jadi urutannya berurutan:

1. **Wave 1:** global-agents-ticket-01-agents-registry
2. **Wave 2:** global-agents-ticket-02-agents-mention-spawn (menunggu ticket-01)
3. **Wave 3:** global-agents-ticket-03-agents-orchestrator (menunggu ticket-01 dan ticket-02)

## Kontrak bersama

### IPC daemon

Dibuat oleh tiket agents-registry, dipakai oleh tiket 2 dan 3.

| Method | Payload | Hasil |
|---|---|---|
| `agents.list` | - | `{categories: Category[], agents: Agent[]}` |
| `agents.categoryUpsert` | `{id?, name, role}` | `Category` |
| `agents.categoryDelete` | `{id, agents?: 'move' \| 'delete', moveTo?: categoryId}` | - |
| `agents.upsert` | `{id?, name, categoryId, providerId, model, isDefault}` | `Agent` |
| `agents.delete` | `{id}` | - |
| `agents.setDefault` | `{id}` | - |

Event: `agents.changed`.

Tipe:

- `Category{id, name, role, builtin: bool}`
- `Agent{id, name, categoryId, providerId, model, isDefault: bool}`

Aturan:

- Nama agent berupa slug huruf kecil dan unik.
- Id kategori bawaan: `orchestrator`, `researcher`, `planner`, `executor`.

### Go: paket `internal/agents`

- `Store` dengan:
  - `Resolve(ref string) (Agent, Category, error)`: satu-satunya tempat aturan resolusi. `ref` adalah nama agent atau id/nama kategori, lalu fallback ke default. Jika tidak ada default, kembalikan error yang jelas.
  - `List()`.
  - `SetDeleteGuard(func(agentID string) error)`: menolak penghapusan agent yang dipakai thread aktif. Dipasang oleh tiket 3.
- Layanan didaftarkan di Deps: `d.Provide("agents.store", *agents.Store)`.
- Paket mention dan thread mendefinisikan antarmuka kecil sendiri:

  ```go
  type AgentResolver interface {
      Resolve(ref string) (agents.Agent, agents.Category, error)
  }
  ```

  Antarmuka ini diambil lewat `d.Lookup` di `OnReady`, sehingga tidak ada siklus modul.

### Sesi dan provider

- `session.SessionSpec.Model string`: kosong berarti default provider.
- `provider.LaunchContext.Model`.
- Flag CLI: claude memakai `--model <m>`, codex memakai `-m <m>`.
- Diteruskan oleh Launcher. Fitur lain hanya mengisi `Spec.Model`.

### Frontend

- Halaman settings didaftarkan dengan `registerSettingsPage({id: 'agents', title: 'Agent', order: 15})`.
- Modul mention dan thread memanggil IPC `agents.list` lewat klien masing-masing. Keduanya tidak mengimpor dari `modules/settings/agents`.
- Brief peran: teks `Category.role` disisipkan ke brief agent yang di-spawn oleh mention (tiket 2) dan ke `BuildPeerBrief` (tiket 3).

### Verifikasi tiap tiket

- Setiap tiket harus lulus `go build ./...` dan build frontend.
- Tanpa unit test.
