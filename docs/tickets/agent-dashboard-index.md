# Indeks tiket: Dashboard agent

Sumber: `docs/superpowers/specs/2026-11-22-agent-dashboard-design.md` (draft, 2026-11-22).

Spec ini menghasilkan satu tiket. Tiket dikerjakan sebagai satu epic dengan enam fitur internal.

## Tabel tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|-------|-------|------|------------|--------------|
| agent-dashboard-ticket-01-agent-dashboard | Dashboard agent | 1 | - | 6 |

Fitur di dalam tiket:
1. `dashboard-foundation`: kontrak tipe dan kerangka modul dashboard.
2. `status-list-fields`: `status.list` menyertakan `reason` dan `providerId`.
3. `dashboard-store`: store turunan `agentRows` dan `waitingCount`.
4. `dashboard-panel-ui`: `DashboardPanel` dan `AgentRow`.
5. `rail-badge`: dukungan badge pada item rail.
6. `dashboard-wiring`: registrasi dashboard ke rail.

## Urutan eksekusi

Semua tiket berada di wave 1. Di dalam tiket, urutannya sebagai berikut.

- **Tahap 1 (paralel):** `dashboard-foundation`, `status-list-fields`.
- **Tahap 2 (paralel):** `dashboard-store`, `dashboard-panel-ui`, `rail-badge`. Ketiganya memakai kontrak dari tahap 1, bukan store nyata.
- **Tahap 3:** `dashboard-wiring`, setelah semua fitur lain selesai.

## Kontrak bersama

1. **Daemon.** `status.list` dan `status.get` mengembalikan `SessionState` `{sessionId, status, since, reason?, providerId?}`. Field baru bersifat `omitempty` dan memakai nama JSON camelCase. Event `session.status` tetap `StatusEvent` apa adanya (`sessionId`, `status`, `prev`, `source`, `reason`, `at`). Tidak ada IPC method baru.
2. **Tipe frontend.** `frontend/src/modules/dashboard/types.ts` mengekspor `AgentRow` `{sessionId, title, providerId, projectName, status:'waiting'|'running'|'idle'|'done', since:number(ms), reason?:string, lastActivity?:string}`. Urutan kelompok tetap: waiting, running, idle, done.
3. **Store.** `dashboard/store.ts` mengekspor `agentRows: Readable<AgentRow[]>` yang sudah terurut, dan `waitingCount: Readable<number>`. File yang sama juga mengekspor `groupRows(rows)`.
4. **Badge rail.** `RailItemSpec` di `shell/registry.ts` mendapat field opsional `badge?: Readable<number | boolean>`. `Rail.svelte` menampilkan titik bila nilainya lebih dari 0 atau bernilai `true`.
5. **Pendaftaran dan navigasi.** Panel didaftarkan lewat `registerRailItem` di `dashboard/index.ts`. Navigasi memakai `focusOrOpenSession(sessionId)` dari `modules/shell/helpers`. Sesi dengan `providerId === 'shell'` tidak pernah masuk `agentRows`. Tidak ada IPC method baru, dan dashboard tidak menambah notifikasi macOS.

Slot order rail: agents=0, lark=10, git-github=20, git-gitlab=21, threads=30, grimoire=40, dashboard=50.

## Di luar cakupan

- Aksi cepat setuju/tolak (fase 2 di spec). Menunggu spike dan model hook sinkron `PreToolUse`.
- Jumlah file berubah dari fitur review diff.
- Membuat, menghentikan, atau memulai ulang sesi dari dashboard.
- Penggabungan dengan monitor workflow/sub agent.
