# Plan: Store turunan agentRows dan waitingCount

## Metadata

- Tiket: `docs/tickets/agent-dashboard-ticket-01-agent-dashboard.md` (id `agent-dashboard-ticket-01-agent-dashboard`)
- Fitur: `dashboard-store` (tahap B)
- ownedPaths: `frontend/src/modules/dashboard/store.ts`, `frontend/src/modules/status/store.ts`
- Depends on foundation: ya. Membutuhkan `frontend/src/modules/dashboard/types.ts` dan stub `dashboard/store.ts` dari fitur `dashboard-foundation` (tahap A, sudah di-merge). Tidak ada `00-foundation.md` terpisah. Bila `types.ts` belum ada, hentikan dan laporkan; jangan membuatnya.
- Rujukan: `overview.md` bagian 2.2, 2.3, 2.4, 6, 7.

## Tujuan

Mengganti stub `dashboard/store.ts` dengan store turunan nyata: `agentRows` (derived tunggal), `waitingCount`, dan `groupRows`, tanpa mengubah ekspor/tipe kontrak. Memperluas `modules/status/store.ts` secara kompatibel agar `SessionStatus` menyimpan `reason` dan `providerId`. Tanpa timer/polling; `ensureActivityStore()` tidak dipanggil di store.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `frontend/src/modules/dashboard/store.ts`
- `frontend/src/modules/status/store.ts`

Jangan sentuh: `dashboard/types.ts`, `dashboard/index.ts`, komponen Svelte dashboard, `shell/registry.ts`, `shell/Rail.svelte`, `internal/status/*`, `SidebarBadges`, `StatusBar`, `App.svelte`, `sidebar-right/FilesTab.svelte`, dan `overview.md`.

## Kontrak

Disediakan (bentuk tetap, lihat overview 2.3 dan 2.4):

```ts
// dashboard/store.ts
export const agentRows: Readable<AgentRow[]>;
export const waitingCount: Readable<number>;
export function groupRows(rows: AgentRow[]): Record<AgentStatus, AgentRow[]>;

// status/store.ts
export interface SessionStatus { status: Status; since: number; reason?: string; providerId?: string; }
```

Dipakai (sudah ada, jangan ubah):

- `AgentRow`, `AgentStatus`, `STATUS_ORDER` dari `./types`.
- `sessions` (`Writable<Session[]>`), `activeWorkspaceId`, `activeWorkspace` (`Readable<Workspace | null>`, `projects[]` berisi `{id, path, name}`), `providers` (`Writable<Provider[]>`, `{id, name}`) dari `../../lib/stores`. Tipe `Session`: `{id, workspaceId, projectId, cwd, providerId, status: 'running' | 'exited', ...}`.
- `statusStore` dan tipe `Status` dari `../status/store`.
- `activityEntries` dari `../activity/store` (`ActivityEntry {ts, sessionId, tool, summary}`, terbaru lebih dulu). Hanya impor `activityEntries`; JANGAN impor/panggil `ensureActivityStore`.
- `sessionTitle(sessionId)` dan `basename(path)` dari `../shell/helpers`. Bila impor ini memunculkan siklus atau error build, jangan ubah helpers; hitung `title` sendiri di store (nama provider dari `providers`, fallback `providerId`) dan `basename` dengan fungsi lokal kecil.

## Langkah

### 1. Perluas `frontend/src/modules/status/store.ts`

Ubah hanya yang perlu; `statusOf`, `startStatusSync`, signature ekspor lain tidak berubah.

1. `export interface SessionStatus { status: Status; since: number; reason?: string; providerId?: string; }`
2. `handler` (event `session.status`): tambah `reason?: unknown` pada tipe payload lokal. Hitung `const reason = p.source === 'exit' ? 'exit' : (typeof p.reason === 'string' && p.reason ? p.reason : undefined);`. Pertahankan `providerId` entri lama: `store.update((m) => ({ ...m, [id]: { status, since, reason, providerId: m[id]?.providerId } }))`. Pemetaan status `exit` -> `done` yang sudah ada dipertahankan, satu `store.update` per event (agar pindah ke `done` dalam satu pembaruan).
3. `startStatusSync` pada `status.list`: lebarkan tipe hasil menjadi `Array<{ sessionId: string; status: Status; since: number; reason?: string; providerId?: string }>`. Saat menulis entri: `next[it.sessionId] = { status: it.status, since: it.since, reason: typeof it.reason === 'string' && it.reason ? it.reason : undefined, providerId: typeof it.providerId === 'string' && it.providerId ? it.providerId : undefined };`. Pertahankan aturan "lewati bila `cur && cur.since >= it.since`" tetapi saat `cur` ada dan dilewati serta `cur.providerId` kosong sedangkan `it.providerId` ada, isi `providerId` saja: `next[id] = { ...cur, providerId: it.providerId }`. Field `reason`/`providerId` tak ada (daemon lama) tidak boleh menimbulkan error.
4. Jangan ubah penanganan `isStatus`, pembersih `unsub`, atau perilaku idempoten.

### 2. Ganti isi `frontend/src/modules/dashboard/store.ts`

Hapus stub; ekspor dan tipe tetap sama. Struktur:

```ts
import { derived, type Readable } from 'svelte/store';
import { sessions, providers, activeWorkspaceId, activeWorkspace } from '../../lib/stores';
import { statusStore, type Status } from '../status/store';
import { activityEntries } from '../activity/store';
import { sessionTitle, basename } from '../shell/helpers';
import { STATUS_ORDER, type AgentRow, type AgentStatus } from './types';
```

(Sesuaikan jalur impor dengan struktur repo; `lib/stores` mengekspor `sessions`, `providers`, `activeWorkspaceId`, `activeWorkspace` lewat `index.ts`, dan `activeWorkspace` berasal dari `workspaces.ts`.)

1. `const STATUS_RANK: Record<AgentStatus, number>` dibangun dari `STATUS_ORDER` (indeks).
2. `groupRows(rows)`: buat `Record<AgentStatus, AgentRow[]>` dengan keempat kunci selalu ada (`[]` awal, iterasi `STATUS_ORDER`), lalu satu lintasan `for (const r of rows) out[r.status].push(r)`. Urutan input dipertahankan; fungsi murni, tidak mengubah `rows`.
3. `agentRows = derived([sessions, statusStore, activityEntries, activeWorkspace, providers, activeWorkspaceId], ([$sessions, $status, $entries, $ws, $providers, $wsId]) => { ... })`. Catatan: `activeWorkspaceId` adalah tambahan sumber ke daftar yang dikontrakkan, dibutuhkan untuk filter `workspaceId`; tetap satu `derived`. Isi:
   - Bangun `Map<string, string>` `lastBySession` dalam SATU lintasan atas `$entries` (terbaru lebih dulu): `if (!m.has(e.sessionId)) m.set(e.sessionId, e.summary)`. Tidak boleh ada `.find`/`.filter` per baris atas `$entries`.
   - Bangun `Map` nama proyek dari `$ws?.projects ?? []` (`id -> name`) dan `Map` provider (`id -> name`) satu kali.
   - Untuk tiap `s` di `$sessions`: lewati bila `s.providerId === 'shell'` atau `s.workspaceId !== $wsId` (juga bila `$wsId` null: hasil kosong).
   - Status: `const st = $status[s.id]`; bila `st` ada pakai `st.status`; bila tidak ada: `s.status === 'exited' ? 'done' : 'idle'`. Tipe `Status` (`running|waiting|done|idle`) sudah identik dengan `AgentStatus`; cast aman via fungsi `toAgentStatus(x: Status): AgentStatus` bila diperlukan.
   - `since`: `st?.since ?? (Date.parse(s.createdAt) || 0)` (sesi tanpa entri status memakai waktu pembuatan; `NaN` jatuh ke 0).
   - `projectName`: `projectNames.get(s.projectId) ?? basename(s.cwd)`.
   - `title`: `sessionTitle(s.id)` (memakai judul pane, lalu nama provider). Bila gagal/kosong: `providerNames.get(s.providerId) ?? s.providerId`. Tidak ada side effect.
   - `reason`: `st?.reason || undefined`; `lastActivity`: `lastBySession.get(s.id)` (kosong -> `undefined`, tidak error).
   - Hasil baris: `{ sessionId: s.id, title, providerId: s.providerId, projectName, status, since, reason, lastActivity }`.
   - Urut: `rows.sort((a, b) => STATUS_RANK[a.status] - STATUS_RANK[b.status] || (a.status === 'waiting' ? a.since - b.since : b.since - a.since))` (waiting naik, terlama menunggu dulu; kelompok lain menurun, terbaru dulu). Pakai satu pilihan ini secara konsisten dan beri komentar satu baris yang menjelaskannya.
4. `waitingCount = derived(agentRows, ($r) => $r.filter((r) => r.status === 'waiting').length)`; angka 0 bila tidak ada.
5. Larangan: tidak ada `setInterval`/`setTimeout`/polling, tidak memanggil `ensureActivityStore()`, tidak ada `onEvent`/`call` sendiri. Tidak ada impor Svelte komponen. Komentar secukupnya, bahasa Indonesia, identifier bahasa Inggris.

### 3. Verifikasi

Jalankan dari `frontend/`: `npm run build`, lalu `npm run check` bila skrip tersedia. Perbaiki error tipe pada path milik fitur ini saja. Bila error berasal dari file di luar path yang dimiliki (mis. komponen lain yang memakai `SessionStatus`), laporkan, jangan ubah. Periksa dengan `git diff --stat` bahwa hanya dua file di atas yang berubah.

## Perubahan

- Diubah: `frontend/src/modules/status/store.ts` (field `reason`/`providerId` pada `SessionStatus`, handler event, pembaruan `status.list`).
- Diubah (menggantikan stub): `frontend/src/modules/dashboard/store.ts` (derived tunggal `agentRows`, `waitingCount`, `groupRows`).
- Tidak ada file baru.

## Wiring

Fitur ini hanya mengekspos `agentRows`, `waitingCount`, dan `groupRows` dari `dashboard/store.ts`. Langkah wiring nanti (`dashboard-wiring`) memakai `waitingCount` sebagai `badge` item rail, dan `dashboard-panel-ui` mengonsumsi `agentRows`/`groupRows`; `DashboardPanel` akan memanggil `ensureActivityStore()` di `onMount`. JANGAN membuat `dashboard/index.ts`, memanggil `registerRailItem`, atau memanggil `startStatusSync()`/`ensureActivityStore()` dari fitur ini.

## Selesai bila

- `agentRows` tidak memuat sesi `shell` maupun sesi workspace lain; sesi tanpa entri status jadi `idle`, sesi `exited` tanpa entri status jadi `done`.
- Urutan: waiting, running, idle, done; di dalam kelompok `since` (waiting naik, lainnya menurun).
- `waitingCount` hanya menghitung `waiting` dan bernilai 0 bila tidak ada.
- Event `session.status` dengan `source: 'exit'` menghasilkan satu `store.update` yang membuat status `done` dengan `reason: 'exit'`.
- `reason`/`lastActivity` terisi bila ada; nilai kosong tidak menimbulkan error; `lastActivity` dihitung lewat satu `Map` satu lintasan.
- Tidak ada timer/polling; `ensureActivityStore` tidak dipanggil di store; `statusOf`, `SidebarBadges`, `StatusBar` tetap berfungsi tanpa perubahan.
- Ekspor dan tipe kontrak tidak berubah; hanya dua path milik fitur ini yang berubah.
- Guard: build frontend (`npm run build` di `frontend/`, plus `npm run check` bila ada) sukses. Tanpa unit test.
