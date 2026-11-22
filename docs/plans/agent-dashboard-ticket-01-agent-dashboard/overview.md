# Overview: agent-dashboard-ticket-01-agent-dashboard

Tiket: `docs/tickets/agent-dashboard-ticket-01-agent-dashboard.md`
Spec: `docs/superpowers/specs/2026-11-22-agent-dashboard-design.md`

## 1. Ringkasan epic dan urutan eksekusi

User menjalankan banyak sesi agent, tetapi tidak ada satu tempat untuk melihat status, lama di status, dan aktivitas terakhir tiap sesi. Epic ini menambah item rail "Dashboard" (order 50) berisi panel yang mendaftar sesi agent (bukan shell) di workspace aktif, dikelompokkan Menunggu, Berjalan, Idle, Selesai. Klik baris memfokuskan/membuka pane sesi. Ikon rail menampilkan badge titik bila ada sesi `waiting`. Data murni dari `status.list`, event `session.status`, dan store aktivitas yang sudah ada: tanpa polling, tanpa IPC method baru, tanpa notifikasi macOS baru.

Tidak ada fase Foundation terpisah (`hasFoundation=false`, tidak ada `00-foundation.md`). Tipe dan kerangka bersama sudah ditulis penuh di bagian 2, dan `dashboard-foundation` adalah fitur biasa dengan ownedPaths sendiri (pola yang sama dengan `main-rail`). Urutan eksekusi mengikuti dependensi kompilasi:

1. Tahap A (paralel): `dashboard-foundation`, `status-list-fields`.
2. Tahap B (paralel, setelah A selesai dan di-merge): `dashboard-store`, `dashboard-panel-ui`, `rail-badge`. Ketiganya memakai kontrak tahap A, bukan store nyata.
3. Tahap C (Wiring, terakhir): `dashboard-wiring`.

Catatan: working tree punya perubahan belum di-commit di `frontend/src/modules/sidebar-right/FilesTab.svelte` dan dokumen `docs/`; tidak boleh disentuh oleh fitur manapun.

## 2. Kontrak bersama

### 2.1 Daemon (Go, paket `internal/status`)

```go
// types.go
type SessionState struct {
    SessionID  string `json:"sessionId"`
    Status     Status `json:"status"`
    Since      int64  `json:"since"` // unix milidetik
    Reason     string `json:"reason,omitempty"`
    ProviderID string `json:"providerId,omitempty"`
}
```

- `StatusEvent` (event `session.status`) tidak berubah: `{sessionId, status, prev, source, reason, at}`.
- `engine.go`: struct `sessionState` mendapat `reason string`; `Set` mengisinya bersama `status`/`since` pada transisi; `Get` dan `List` mengisi `Reason: st.reason, ProviderID: st.providerID`. `handleExit` sudah memanggil `e.Set(id, StatusDone, SourceExit, "exit")`, jadi reason `exit` otomatis tersimpan (verifikasi, ubah hanya bila kosong).
- Sesi yang belum pernah bertransisi: `reason` kosong sehingga tidak diserialisasi. `ipc.go` hanya diedit bila serialisasi memerlukannya (kemungkinan tidak).
- Tidak ada IPC method baru.

### 2.2 Tipe frontend (`frontend/src/modules/dashboard/types.ts`)

```ts
export type AgentStatus = 'waiting' | 'running' | 'idle' | 'done';
export interface AgentRow {
  sessionId: string; title: string; providerId: string; projectName: string;
  status: AgentStatus; since: number /* ms epoch */; reason?: string; lastActivity?: string;
}
export const STATUS_ORDER: AgentStatus[] = ['waiting', 'running', 'idle', 'done'];
export const STATUS_LABEL: Record<AgentStatus, string> = { waiting: 'Menunggu', running: 'Berjalan', idle: 'Idle', done: 'Selesai' };
```

### 2.3 Store dashboard (`frontend/src/modules/dashboard/store.ts`)

```ts
export const agentRows: Readable<AgentRow[]>;     // terurut kelompok (STATUS_ORDER) lalu since
export const waitingCount: Readable<number>;      // hanya status waiting
export function groupRows(rows: AgentRow[]): Record<AgentStatus, AgentRow[]>; // semua 4 kunci selalu ada, urutan input dipertahankan
```

- Stub foundation: `agentRows = readable<AgentRow[]>([])`, `waitingCount = readable(0)`, `groupRows` fungsional (bukan stub kosong). `dashboard-store` menggantikan isi file, ekspor dan tipe tidak boleh berubah.
- Implementasi nyata: satu `derived` dari `[sessions, statusStore, activityEntries, activeWorkspace, providers]`. Aturan: buang `providerId === 'shell'` dan `workspaceId !== $activeWorkspaceId`; sesi tanpa entri status dianggap `idle`, sesi `exited` di store sesi tanpa entri status dianggap `done`; `projectName` dari `activeWorkspace.projects` (fallback `basename(cwd)`); `title` dari `sessionTitle`/nama provider; `lastActivity` = summary entri aktivitas terbaru per `sessionId` lewat satu lintasan `Map`. Urutan di dalam kelompok: `waiting` naik menurut `since` (terlama menunggu dulu), kelompok lain menurun menurut `since` (terbaru dulu). Satu pilihan ini dipakai konsisten.
- Tidak ada `setInterval`/polling di store. `ensureActivityStore()` TIDAK dipanggil di modul store; dipanggil `DashboardPanel` di `onMount`.

### 2.4 Status store (`frontend/src/modules/status/store.ts`, dimiliki `dashboard-store`)

Perluasan kompatibel: `SessionStatus` menjadi `{ status: Status; since: number; reason?: string; providerId?: string }`. Handler `session.status` menyimpan `reason` payload (source `exit` memetakan status `done` dengan reason `exit`, sudah dipetakan handler). Pembaruan `status.list` menyimpan `reason`/`providerId` bila ada. `statusOf`, `SidebarBadges`, `StatusBar` tidak berubah.

### 2.5 Badge rail (`frontend/src/modules/shell/registry.ts`, dimiliki `dashboard-foundation`)

```ts
export interface RailItemSpec {
  id: string; title: string; icon: Component; component: Component<any>;
  order?: number; visible?: Readable<boolean>;
  badge?: Readable<number | boolean>; // baru: titik tampil bila > 0 atau true
}
```

`railItems()` dan perilaku item lain tidak berubah. `Rail.svelte` (milik `rail-badge`) merender titik bila nilai `> 0` atau `true`, posisi absolut di sudut tombol, selaras gaya titik di `modules/notifications/Bell.svelte`. `aria-label` tombol menjadi `"<title> (ada sesi menunggu)"` saat badge aktif; tanpa badge tetap `title`. Karena `badge` adalah store per item di dalam `{#each}`, gunakan sub-komponen kecil `frontend/src/modules/shell/RailBadge.svelte` (props `{ badge?: Readable<number | boolean> }`, subscribe lewat `$badge`/`$effect` dan dibersihkan saat unmount) yang juga dimiliki `rail-badge`; `aria-label` memerlukan status aktif di `Rail.svelte`, jadi pola yang dianjurkan adalah komponen `RailButton`-lokal atau mengangkat nilai badge via callback prop `onchange`. Agent `rail-badge` memilih satu pola, selama item tanpa badge identik dengan perilaku sebelumnya. Gaya ditulis di blok `<style>` komponen; `shell.css` tidak disentuh.

### 2.6 Pendaftaran, navigasi, nama

- `registerRailItem({ id: 'dashboard', title: 'Dashboard', icon: DashboardRailIcon, component: DashboardPanel, order: 50, badge: waitingCount })` di `dashboard/index.ts`.
- Navigasi: `focusOrOpenSession(sessionId)` dari `modules/shell/helpers` (juga diekspor `modules/shell`). Impor langsung dari `../shell/helpers` atau `../shell` mengikuti pola modul lain; hindari siklus impor.
- Slot order rail: agents=0, lark=10, git-github=20, git-gitlab=21, threads=30, grimoire=40, dashboard=50.
- Folder modul: `frontend/src/modules/dashboard/` berisi `types.ts`, `store.ts`, `DashboardPanel.svelte`, `AgentRow.svelte`, `DashboardRailIcon.svelte`, `index.ts`. Komponen Svelte memakai PascalCase; identifier kode bahasa Inggris, teks UI bahasa Indonesia.
- Teks UI: "Menunggu", "Berjalan", "Idle", "Selesai", "Belum ada sesi agent", "+N lainnya", "menunggu: <reason>" (tanpa reason: "menunggu"), durasi "baru saja", "N dtk", "N mnt", "N jam".
- Bentuk komponen panel: `DashboardPanel` tanpa props (dirender dengan `{}`), hanya mengimpor `agentRows`/`groupRows` dari `./store`. `AgentRow.svelte` props: `{ row: AgentRow; now: number }`, mengimpor tipe dari `./types`. `DashboardRailIcon.svelte` tanpa props, SVG inline `currentColor` mengikuti `GrimoireRailIcon.svelte`.
- Timer panel: satu `setInterval` (1000 atau 5000 ms) memperbarui `now`, hidup hanya saat panel terpasang dan `!document.hidden`, dibersihkan di cleanup `$effect`/`onDestroy`.
- Sesi shell tidak pernah muncul; tidak ada notifikasi macOS baru; tidak ada dependensi baru; tidak ada IPC method baru.

### 2.7 Pesan IPC

Tidak ada pesan baru. Dipakai yang sudah ada: `status.list` (`SessionState` diperluas, lihat 2.1), event `session.status`, `activity.list`/`activity.event` (lewat `activityEntries`/`ensureActivityStore`).

## 3. Daftar fitur

| Slug | Scope | ownedPaths |
|------|-------|------------|
| `dashboard-foundation` | `types.ts`, stub `store.ts` (`agentRows`, `waitingCount`, `groupRows` fungsional), field `badge?` pada `RailItemSpec`. | `frontend/src/modules/dashboard/types.ts`, `frontend/src/modules/dashboard/store.ts`, `frontend/src/modules/shell/registry.ts` |
| `status-list-fields` | Backend: `Reason` dan `ProviderID` pada `SessionState`, `reason` pada `sessionState`, isi di `Get`/`List`, reason `exit`. | `internal/status/types.go`, `internal/status/engine.go`, `internal/status/ipc.go` |
| `dashboard-store` | Store turunan nyata `agentRows`/`waitingCount`/`groupRows`; perluasan `status/store.ts`. | `frontend/src/modules/dashboard/store.ts`, `frontend/src/modules/status/store.ts` |
| `dashboard-panel-ui` | `DashboardPanel` (ringkasan, kelompok, Selesai maks 10 dan dapat diciutkan, timer, `ensureActivityStore` di `onMount`), `AgentRow`, `DashboardRailIcon`; aksesibilitas. | `frontend/src/modules/dashboard/DashboardPanel.svelte`, `frontend/src/modules/dashboard/AgentRow.svelte`, `frontend/src/modules/dashboard/DashboardRailIcon.svelte` |
| `rail-badge` | `Rail.svelte` merender `RailItemSpec.badge` (titik, `aria-label`); boleh menambah `RailBadge.svelte`. | `frontend/src/modules/shell/Rail.svelte`, `frontend/src/modules/shell/RailBadge.svelte` (hanya bila dibuat) |
| `dashboard-wiring` | `dashboard/index.ts` memanggil `registerRailItem`; pastikan `statusStore` terisi dan verifikasi end-to-end. | `frontend/src/modules/dashboard/index.ts` |

Tumpang tindih path: `dashboard/store.ts` dimiliki `dashboard-foundation` (stub, tahap A) lalu `dashboard-store` (tahap B); tidak pernah paralel, jadi aman. `shell/registry.ts` hanya `dashboard-foundation`.

## 4. Foundation

Tidak ada Foundation terpisah (`hasFoundation=false`). Kerja bersama yang harus ada sebelum tahap B adalah fitur tahap A: `dashboard-foundation` (tipe, stub store, field `badge`) dan `status-list-fields`. Isi minimum `dashboard-foundation`: sesuai 2.2, 2.3 (stub), 2.5 (hanya field di registry). Kerangka paket tidak perlu karena folder `dashboard/` dibuat oleh `types.ts`.

## 5. Wiring checklist

Dikerjakan `dashboard-wiring` setelah semua fitur lain di-merge:

- [ ] Buat `frontend/src/modules/dashboard/index.ts`: impor `registerRailItem` dari `../shell/registry`, `DashboardPanel`, `DashboardRailIcon`, `waitingCount` dari `./store`; panggil `registerRailItem({ id: 'dashboard', title: 'Dashboard', icon: DashboardRailIcon, component: DashboardPanel, order: 50, badge: waitingCount })` sekali, dengan komentar slot rail seperti `grimoire/index.ts` (agents=0, lark=10, git-github=20, git-gitlab=21, threads=30, grimoire=40, dashboard=50).
- [ ] Modul dimuat otomatis oleh `import.meta.glob` shell; JANGAN edit `App.svelte`, `shell/register.ts`, atau item rail lain. Verifikasi bahwa pola glob memang menjangkau `modules/dashboard/index.ts`.
- [ ] Pastikan `startStatusSync()` aktif walau panel belum dibuka (impor `../status/store` atau pastikan modul `status` termuat) supaya `statusStore` dan badge terisi.
- [ ] `ensureActivityStore()` dipanggil di `DashboardPanel` (`onMount`), bukan di `index.ts` maupun `store.ts`.
- [ ] Pastikan tidak ada pemanggilan notifikasi macOS baru.
- [ ] Verifikasi manual end-to-end: tiga sesi (jalan, menunggu, selesai) tampil di kelompok benar dan berpindah tanpa muat ulang; sesi exit menjadi Selesai; klik baris memfokuskan/membuka pane; shell tidak muncul; badge muncul saat ada waiting dan hilang saat 0; panel tertutup tidak punya timer.

## 6. Build guard

Tanpa unit test. Satu-satunya guard:

- `go build ./...` (daemon dan app) dari root repo.
- Build frontend: `npm run build` di `frontend/` (`vite build`); jalankan juga `npm run check` (`svelte-check`) bila tersedia untuk menangkap kesalahan tipe.
- Build wails dan menjalankan aplikasi hanya pada `dashboard-wiring` untuk verifikasi manual.

Fitur frontend-saja menjalankan build frontend; `status-list-fields` menjalankan `go build ./...`.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, `App.svelte`, `shell/register.ts`, `shell.css`, file wiring, atau perubahan yang belum di-commit di `sidebar-right/FilesTab.svelte`.
- Kode ditulis terhadap kontrak bagian 2, bukan terhadap implementasi fitur lain. Ekspor, tipe, nama berkas, dan props di bagian 2 tidak boleh diubah; bila kontrak tampak keliru, laporkan alih-alih mengubahnya.
- Tahap B baru dimulai setelah tahap A di-merge; `dashboard-wiring` baru setelah semua fitur lain di-merge.
- Tanpa dependensi baru, tanpa polling di store, tanpa IPC method baru, tanpa notifikasi macOS baru, tanpa unit test.
- Fase 2 (aksi cepat setuju/tolak) dan jumlah file berubah dari review diff dilarang: tidak ada tombol atau field placeholder.
