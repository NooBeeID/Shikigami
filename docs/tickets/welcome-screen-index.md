# Welcome Screen: Indeks Tiket

Sumber: `docs/superpowers/specs/2026-11-22-welcome-screen-design.md`

## Ringkasan tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|---|---|---|---|---|
| `welcome-screen-ticket-01-session-workspaces` | Workspace terbuka per sesi | 1 | - | 4 |
| `welcome-screen-ticket-02-welcome-screen` | Layar welcome dan terakhir dibuka | 2 | `welcome-screen-ticket-01-session-workspaces` | 3 |

### Fitur per tiket

- **ticket-01** (Workspace terbuka per sesi): `workspace-touch-go`, `open-set-store`, `null-workspace-safety`, `tabs-close-menu`
- **ticket-02** (Layar welcome dan terakhir dibuka): `welcome-logic`, `welcome-ui`, `welcome-wiring`

## Urutan eksekusi

- **Wave 1**: `welcome-screen-ticket-01-session-workspaces` (tidak ada dependensi)
- **Wave 2**: `welcome-screen-ticket-02-welcome-screen` (menunggu ticket-01 selesai)

Tiap wave tidak memiliki tiket paralel. Wave 2 baru dimulai setelah wave 1 selesai.

## Kontrak bersama

### RPC `workspace.touch`

- Request: `{id: string}`
- Response: `Workspace` (objek workspace penuh, dengan `lastOpenedAt` diisi unix ms saat ini)
- Perilaku: persist ke penyimpanan, lalu publish event `workspace.changed` dengan payload `{kind: "updated", workspaceId, workspace}`
- Error: id tidak dikenal mengembalikan kode `not found`, sama seperti RPC workspace lainnya

### Field `Workspace.lastOpenedAt`

- Go: `LastOpenedAt int64 \`json:"lastOpenedAt"\`` (0 untuk data lama)
- TypeScript: `lastOpenedAt?: number`

### Store frontend `lib/stores/workspaces.ts`

- `openWorkspaceIds: Writable<string[]>`: disimpan di memori saja, tidak dipersist. Urutannya sama dengan urutan tab.
- `openWorkspace(id): Promise<void>`: menambahkan id ke `openWorkspaceIds` bila belum ada, memanggil `selectWorkspace`, dan memanggil `workspace.touch` secara best-effort.
- `closeWorkspaceTab(id): Promise<void>`: mengeluarkan id dari `openWorkspaceIds`. Bila tab tersebut aktif, pilih tetangganya atau `null` bila tidak ada. Tidak memanggil `workspace.delete`.
- `deleteWorkspace`: ikut mengeluarkan id dari `openWorkspaceIds`.
- `loadWorkspaces()`: tidak lagi memilih workspace pertama. Saat start, `activeWorkspaceId = null`.

### Lainnya

- `newWorkspaceFromFolder(path)` di `modules/openfolder/logic.ts` memanggil `openWorkspace`.
- Welcome dirender di area tengah (`App.svelte`) tepat saat `activeWorkspaceId === null`.
- Semua konsumen `activeWorkspace` harus aman terhadap nilai `null`.

### Kriteria daftar "terakhir dibuka"

- `lastOpenedAt > 0`
- id tidak ada di `openWorkspaceIds`
- diurutkan menurun berdasarkan `lastOpenedAt`
- maksimal 5 item
