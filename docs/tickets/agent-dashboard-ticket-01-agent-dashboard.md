---
id: agent-dashboard-ticket-01-agent-dashboard
title: Dashboard agent
spec: docs/superpowers/specs/2026-11-22-agent-dashboard-design.md
status: implemented
wave: 1
depends_on: []
---
# Dashboard agent

## Tujuan
User menjalankan banyak sesi agent sekaligus, tetapi tidak ada satu tempat untuk melihat siapa yang berjalan, menunggu keputusan, idle, atau selesai, dan sudah berapa lama. Epic ini menambah panel rail baru "Dashboard" yang mendaftar semua sesi agent (bukan shell) di workspace aktif, dikelompokkan Menunggu, Berjalan, Idle, Selesai, dengan klik untuk melompat ke pane sesi dan badge titik di ikon rail bila ada sesi `waiting`. Data murni dari `status.list`, event `session.status`, dan store aktivitas yang sudah ada, tanpa polling dan tanpa notifikasi macOS baru.

## Hasil yang bisa dijalankan
Setelah tiket ini selesai (tiket pertama dan satu-satunya di spec ini, tanpa tiket sebelumnya), `go build ./...` dan build frontend lulus, aplikasi berjalan, dan:
- Rail kiri punya ikon baru "Dashboard" (order 50, setelah Grimoire=40). Mengkliknya menampilkan panel dengan ringkasan jumlah per status di atas dan kelompok Menunggu, Berjalan, Idle, Selesai (kelompok kosong disembunyikan).
- Dengan tiga sesi (satu berjalan, satu menunggu, satu selesai), tiap sesi tampil di kelompok yang benar dan berpindah kelompok segera saat status berubah, tanpa muat ulang.
- Tiap baris menampilkan titik status berwarna plus label teks, nama sesi/provider, nama proyek, lama di status (mis. "3 mnt"), dan aktivitas terakhir satu baris terpotong. Baris waiting menampilkan "menunggu: <reason>" dan diberi tanda menonjol.
- Mengklik baris memfokuskan pane sesi, atau membukanya bila belum ada pane.
- Sesi yang prosesnya berakhir (source `exit`) pindah ke Selesai dalam satu event.
- Ikon rail Dashboard menampilkan titik badge bila ada sesi waiting; titik hilang saat jumlahnya 0.
- Sesi shell tidak pernah muncul. Tidak ada notifikasi macOS tambahan.
- Item rail lain (Agen, Lark, Git, Grimoire) tidak berubah perilaku.

## Batasan
In scope:
- Penambahan field `reason` dan `providerId` (omitempty) pada `SessionState` yang dikembalikan `status.list`/`status.get`.
- Modul frontend `frontend/src/modules/dashboard/` (tipe, store turunan, panel, baris, ikon, registrasi).
- Field opsional `badge` pada `RailItemSpec` dan render badge di `Rail.svelte`.
- Perluasan store status agar menyimpan `reason`/`providerId`.

Out of scope:
- Aksi cepat setuju/tolak (fase 2 di spec): menunggu spike bentuk prompt permission dan model hook sinkron (`PreToolUse`); tidak dibuat dan tidak ada tombol placeholder.
- Jumlah file berubah dari fitur review diff (opsional di spec; epic review diff terpisah, tidak ada field/placeholder).
- Membuat, menghentikan, atau memulai ulang sesi dari dashboard.
- Lintas workspace, riwayat lintas peluncuran, grafik, token, biaya.
- Penggabungan dengan monitor workflow/sub agent (`activity`); komponen `modules/activity/*` tidak diubah.
- IPC method baru, notifikasi macOS baru, polling, unit test, dependensi baru.

## Kontrak
Disediakan:
1. Daemon: `status.list` dan `status.get` mengembalikan `SessionState` `{sessionId, status, since, reason?, providerId?}`; field baru `omitempty`, JSON camelCase: `Reason string `json:"reason,omitempty"``, `ProviderID string `json:"providerId,omitempty"``. Event `session.status` tetap `StatusEvent` apa adanya `{sessionId, status, prev, source, reason, at}`; tidak ada IPC method baru.
2. `frontend/src/modules/dashboard/types.ts`:
   ```ts
   export type AgentStatus = 'waiting' | 'running' | 'idle' | 'done';
   export interface AgentRow {
     sessionId: string; title: string; providerId: string; projectName: string;
     status: AgentStatus; since: number /* ms epoch */; reason?: string; lastActivity?: string;
   }
   export const STATUS_ORDER: AgentStatus[] = ['waiting', 'running', 'idle', 'done'];
   export const STATUS_LABEL: Record<AgentStatus, string> = { waiting: 'Menunggu', running: 'Berjalan', idle: 'Idle', done: 'Selesai' };
   ```
3. `frontend/src/modules/dashboard/store.ts` mengekspor `agentRows: Readable<AgentRow[]>` (sudah terurut kelompok lalu `since`), `waitingCount: Readable<number>`, dan `groupRows(rows): Record<AgentStatus, AgentRow[]>`.
4. `RailItemSpec` di `frontend/src/modules/shell/registry.ts` mendapat `badge?: Readable<number | boolean>`. `Rail.svelte` merender titik bila nilai `> 0` atau `true`.
5. Pendaftaran lewat `registerRailItem` di `dashboard/index.ts`; navigasi memakai `focusOrOpenSession(sessionId)` dari `modules/shell/helpers` (juga diekspor `modules/shell`). Sesi dengan `providerId === 'shell'` tidak pernah masuk `agentRows`.
6. Slot order rail: agents=0, lark=10, git-github=20, git-gitlab=21, threads=30, grimoire=40, dashboard=50.

Dipakai (sudah ada): `sessions` (`lib/stores/sessions.ts`, sudah difilter workspace aktif oleh `loadSessions`; tetap filter `workspaceId === $activeWorkspaceId`), `activeWorkspace` (`lib/stores/workspaces.ts`, `projects[]` untuk nama proyek), `providers` store (nama provider), `statusStore`/`startStatusSync` (`modules/status/store.ts`), `activityEntries`/`ensureActivityStore` (`modules/activity/store.ts`, entri `{ts, sessionId, tool, summary}` terurut terbaru lebih dulu), `focusOrOpenSession`/`sessionTitle` (`modules/shell/helpers.ts`), `registerRailItem`, gaya badge titik di `modules/notifications/Bell.svelte`.

## Fitur

Urutan internal: `dashboard-foundation` dan `status-list-fields` dikerjakan lebih dulu (saling paralel); lalu `dashboard-store`, `dashboard-panel-ui`, `rail-badge` paralel (panel UI dan badge memakai kontrak foundation, bukan store nyata); terakhir `dashboard-wiring`.

### dashboard-foundation: Kontrak tipe dan kerangka modul dashboard
Buat `frontend/src/modules/dashboard/types.ts` berisi `AgentRow`, `AgentStatus`, konstanta urutan kelompok `STATUS_ORDER` (waiting, running, idle, done), dan `STATUS_LABEL` teks Indonesia (Menunggu, Berjalan, Idle, Selesai) seperti di bagian Kontrak. Buat stub `store.ts` yang mengekspor `agentRows: Readable<AgentRow[]>` dan `waitingCount: Readable<number>` bernilai kosong (`readable([])`, `readable(0)`), plus `groupRows` fungsional sederhana, agar UI dan wiring bisa dibangun paralel. Perluas `RailItemSpec` di `shell/registry.ts` dengan `badge?: Readable<number | boolean>` (tambah field saja; `railItems()` dan item lain tidak berubah perilaku). Build harus lulus dan aplikasi tetap jalan.
Acceptance criteria:
- [ ] `types.ts` mengekspor `AgentRow`, `AgentStatus`, `STATUS_ORDER`, `STATUS_LABEL` sesuai kontrak.
- [ ] `store.ts` stub mengekspor `agentRows`, `waitingCount`, `groupRows` dengan tipe kontrak.
- [ ] `RailItemSpec.badge` opsional ada; item tanpa badge dan `railItems()` tidak berubah.
- [ ] Build frontend lulus.
ownedPaths: `frontend/src/modules/dashboard/types.ts`, `frontend/src/modules/dashboard/store.ts`, `frontend/src/modules/shell/registry.ts`.
Dikerjakan lebih dulu; paralel dengan `status-list-fields`. Mendahului semua fitur frontend lain.

### status-list-fields: status.list menyertakan reason dan providerId
Backend. Di `internal/status/types.go` tambah `Reason` dan `ProviderID` (omitempty) ke `SessionState`. Di `internal/status/engine.go` tambah field `reason string` ke `sessionState`; set di `Set` bersama `status`/`since` pada transisi (nilai `reason` argumen), dan kosongkan/awali `""` di `bindLocked`. Isi `SessionState{..., Reason: st.reason, ProviderID: st.providerID}` di `Engine.Get` dan `Engine.List` (state sudah menyimpan `providerID`). Pastikan jalur exit memanggil `Set(..., SourceExit, "exit")` sehingga sesi exit memiliki reason `"exit"` (periksa pemanggil exit di engine; ubah hanya bila reason kosong). `ipc.go` hanya diedit bila serialisasi `status.get`/`status.list` perlu penyesuaian (kemungkinan tidak). Hanya penambahan field, bentuk lain tidak berubah sehingga klien lama tetap jalan. Daemon dan app harus tetap build.
Acceptance criteria:
- [ ] `SessionState` punya `reason` dan `providerId` dengan tag `omitempty` camelCase.
- [ ] `status.list`/`status.get` menyertakan `reason` terakhir dari transisi dan `providerId` sesi.
- [ ] Sesi exit memiliki `reason: "exit"`.
- [ ] Sesi yang belum pernah bertransisi tidak menyertakan `reason` (omitempty), field lain tidak berubah.
- [ ] Event `session.status` tidak berubah bentuk.
- [ ] `go build ./...` lulus.
ownedPaths: `internal/status/types.go`, `internal/status/engine.go`, `internal/status/ipc.go`.
Paralel dengan semua fitur frontend (tak ada ketergantungan kode); `dashboard-store` hanya mengonsumsi field baru secara opsional.

### dashboard-store: Store turunan agentRows dan waitingCount
Ganti stub `dashboard/store.ts` dengan implementasi nyata. Perluas `modules/status/store.ts` secara kompatibel: `SessionStatus` menjadi `{status, since, reason?, providerId?}`; handler event menyimpan `reason` dari payload (reason `exit` untuk source exit), pembaruan `status.list` menyimpan `reason`/`providerId` bila ada; `statusOf` dan konsumen lain (`SidebarBadges`, `StatusBar`) tetap jalan tanpa perubahan. Store dashboard: satu `derived` dari `[sessions, statusStore, activityEntries, activeWorkspace, providers]` yang: membuang sesi `providerId === 'shell'` dan yang `workspaceId` bukan workspace aktif; memetakan status (sesi tanpa entri status dianggap `idle`; sesi `exited` dari store sesi tanpa entri status dianggap `done`); mengisi `projectName` dari `activeWorkspace.projects` (fallback ke `basename(cwd)`), `title` dari `sessionTitle`/nama provider, `reason` dan `lastActivity` (summary entri aktivitas terbaru per `sessionId`, dihitung satu kali lewat `Map` dengan satu lintasan, bukan per-baris). Ekspor `agentRows` terurut menurut indeks `STATUS_ORDER` lalu `since` menurun (terbaru dulu; untuk waiting yang terlama menunggu boleh lebih dulu, pilih satu dan konsisten: waiting naik by `since`, lainnya menurun), `waitingCount` (hanya status waiting) dan `groupRows`. Tanpa polling dan tanpa `setInterval`; hanya subscribe store/event yang ada. Sesi dengan event source `exit` otomatis `done` dalam satu event karena handler status sudah memetakannya. Ringan untuk puluhan sesi: derived tunggal; `ensureActivityStore()` dipanggil saat panel dipasang (di UI/wiring), bukan di modul store, sehingga panel tertutup tidak menambah kerja.
Acceptance criteria:
- [ ] `agentRows` tidak memuat sesi provider `shell` maupun sesi workspace lain.
- [ ] Urutan: waiting, running, idle, done; di dalam kelompok terurut `since`.
- [ ] `waitingCount` menghitung hanya status waiting dan menjadi 0 saat tak ada.
- [ ] Event `session.status` dengan `source: 'exit'` membuat baris pindah ke `done` dalam satu pembaruan store.
- [ ] `reason` dan `lastActivity` terisi bila tersedia; kosong tidak menimbulkan error.
- [ ] Tidak ada timer/polling; perubahan `modules/status/store.ts` kompatibel (`statusOf`, `SidebarBadges`, `StatusBar` tetap berfungsi).
- [ ] Build frontend lulus.
ownedPaths: `frontend/src/modules/dashboard/store.ts`, `frontend/src/modules/status/store.ts`.
Paralel dengan `dashboard-panel-ui`, `rail-badge`, `status-list-fields`; menggantikan stub dari `dashboard-foundation`.

### dashboard-panel-ui: DashboardPanel dan AgentRow
Buat `DashboardPanel.svelte`: ringkasan jumlah per status di atas (empat chip, teks "Menunggu 2" dll.), lalu kelompok per `STATUS_ORDER` dari `agentRows` (via `groupRows`); kelompok kosong disembunyikan; kelompok Selesai dibatasi 10 terbaru dengan header yang dapat diciutkan (tombol dengan `aria-expanded`) dan keterangan "+N lainnya" bila dipotong; keadaan kosong total menampilkan teks "Belum ada sesi agent". Panel memanggil `ensureActivityStore()` di `onMount`. Satu timer ringan (`setInterval` 1000 ms atau 5000 ms yang memperbarui `now = Date.now()`) hanya hidup saat panel terpasang dan dibersihkan di `onDestroy`/cleanup `$effect`; hentikan juga saat `document.hidden`. Buat `AgentRow.svelte`: titik status berwarna (CSS variabel tema yang ada) plus label teks status (tidak hanya warna), nama sesi/provider, proyek, lama di status dari `since` dan `now` (format "baru saja", "N dtk", "N mnt", "N jam"), aktivitas terakhir satu baris terpotong (`text-overflow: ellipsis`), label "menunggu: <reason>" untuk waiting (tanpa reason: "menunggu"). Klik baris (elemen `<button>`) memanggil `focusOrOpenSession(sessionId)` dalam try/catch. Sesi waiting diberi tanda menonjol (garis kiri beraksen dan latar tipis). Aksesibilitas: wadah `role="list"`, baris `role="listitem"`, status diumumkan lewat teks (`aria-label` gabungan nama, status, lama), navigasi panah atas/bawah memindahkan fokus antar baris. Buat `DashboardRailIcon.svelte` (SVG inline mengikuti `GrimoireRailIcon.svelte`, tanpa dependensi). Teks UI bahasa Indonesia. Komponen hanya mengimpor `agentRows`/`groupRows` dari `./store`, jadi dapat dibangun terhadap stub foundation.
Acceptance criteria:
- [ ] Ringkasan jumlah per status tampil dan akurat.
- [ ] Kelompok kosong disembunyikan; Selesai maksimal 10 baris dan dapat diciutkan.
- [ ] Baris menampilkan titik + label teks status, nama, proyek, lama, aktivitas terakhir terpotong satu baris.
- [ ] Baris waiting menampilkan "menunggu: <reason>" dan tampak menonjol.
- [ ] Klik baris memanggil `focusOrOpenSession(sessionId)`.
- [ ] `role="list"`/`listitem`, label status berupa teks, panah atas/bawah memindahkan fokus.
- [ ] Timer hanya aktif saat panel terpasang dan terlihat, dibersihkan saat dilepas; tidak ada subscription tersisa.
- [ ] Tidak ada dependensi baru; build frontend lulus.
ownedPaths: `frontend/src/modules/dashboard/DashboardPanel.svelte`, `frontend/src/modules/dashboard/AgentRow.svelte`, `frontend/src/modules/dashboard/DashboardRailIcon.svelte`.
Paralel dengan `dashboard-store`, `rail-badge`, `status-list-fields`; menunggu `dashboard-foundation`.

### rail-badge: Dukungan badge pada item rail
Ubah `shell/Rail.svelte` agar merender `RailItemSpec.badge`. Karena `badge` adalah store per item di dalam `{#each}`, buat komponen kecil lokal (snippet atau sub-komponen di file yang sama; bila perlu sub-komponen terpisah, tambahkan `RailBadge.svelte` di folder shell dan catat di path yang dimiliki) yang `subscribe` ke store badge. Titik tampil bila nilai `> 0` atau `true`, selaras gaya titik lonceng di `modules/notifications/Bell.svelte` (posisi absolut di sudut tombol), hilang saat 0/false. `aria-label` tombol menjadi `"<title> (ada sesi menunggu)"` saat badge aktif; tanpa badge tetap `title` saja. Item tanpa `badge` tidak berubah. Subscribe hanya lewat store, tanpa timer. Gaya ditulis di blok `<style>` komponen atau class baru di `shell.css` hanya jika file itu sudah dimiliki tiket lain; utamakan gaya di dalam `Rail.svelte`.
Acceptance criteria:
- [ ] Item dengan badge bernilai `> 0`/`true` menampilkan titik; hilang saat `0`/`false`.
- [ ] `aria-label` menyebut ada sesi menunggu saat badge aktif.
- [ ] Item tanpa `badge` identik dengan perilaku sebelumnya (label, `aria-current`, klik).
- [ ] Tidak ada timer; subscription dibersihkan saat unmount.
- [ ] Build frontend lulus.
ownedPaths: `frontend/src/modules/shell/Rail.svelte`.
Paralel dengan `dashboard-store`, `dashboard-panel-ui`; menunggu field `badge` dari `dashboard-foundation`.

### dashboard-wiring: Wiring, daftarkan dashboard ke rail
Dikerjakan terakhir setelah fitur lain. Buat `frontend/src/modules/dashboard/index.ts` yang memanggil `registerRailItem({ id: 'dashboard', title: 'Dashboard', icon: DashboardRailIcon, component: DashboardPanel, order: 50, badge: waitingCount })` (slot 50 bebas; tambahkan komentar slot rail seperti di `grimoire/index.ts`). Modul dimuat otomatis oleh `import.meta.glob` shell, jadi tidak ada edit `App.svelte`. Pastikan `startStatusSync()` aktif (sudah dipanggil saat modul `status` diimpor; impor `../status/store` atau pastikan modul status termuat agar `statusStore` terisi walau panel belum dibuka) dan `ensureActivityStore()` dipanggil saat panel dipasang. Pastikan tidak ada pemanggilan notifikasi macOS. Verifikasi end-to-end: build wails berhasil, aplikasi jalan, tiga sesi (jalan, menunggu, selesai) tampil di kelompok yang benar dan berubah tanpa muat ulang, klik baris memfokuskan atau membuka pane, sesi shell tidak muncul, badge hilang saat 0, panel tertutup tidak menjalankan timer.
Acceptance criteria:
- [ ] Item rail `dashboard` terdaftar sekali (order 50) dengan `badge: waitingCount`.
- [ ] Dashboard muncul di rail tanpa mengubah `App.svelte` atau item rail lain.
- [ ] Verifikasi manual: tiga sesi tampil di kelompok benar dan berpindah tanpa muat ulang; sesi exit menjadi Selesai; klik baris memfokuskan/membuka pane; shell tidak muncul; badge muncul saat ada waiting dan hilang saat 0.
- [ ] Panel tertutup (item rail lain aktif) tidak punya timer berjalan.
- [ ] Tidak ada notifikasi macOS baru.
- [ ] `go build ./...` dan build frontend lulus; aplikasi berjalan.
ownedPaths: `frontend/src/modules/dashboard/index.ts`.
Dikerjakan terakhir (bergantung pada semua fitur lain di epic ini).

## Path yang dimiliki
Hanya path berikut yang boleh disentuh:
- `frontend/src/modules/dashboard/` (types.ts, store.ts, DashboardPanel.svelte, AgentRow.svelte, DashboardRailIcon.svelte, index.ts)
- `internal/status/engine.go`
- `internal/status/types.go`
- `internal/status/ipc.go`
- `frontend/src/modules/shell/registry.ts`
- `frontend/src/modules/shell/Rail.svelte`
- `frontend/src/modules/status/store.ts`

## Wiring
Langkah penutup tiket ini adalah fitur `dashboard-wiring`: `dashboard/index.ts` memanggil `registerRailItem` (titik ekstensi shell yang sudah ada dari tiket main-rail, ada di repo saat ini) dengan komponen `DashboardPanel`, ikon `DashboardRailIcon`, `order: 50`, dan `badge: waitingCount`, dimuat otomatis oleh `import.meta.glob`. Tiket ini tidak bergantung pada tiket lain: semua yang dibutuhkan (`registerRailItem`, `focusOrOpenSession`, `status.list`, event `session.status`, `activity.list`/`activity.event`) sudah ada, dan perubahan backend hanya penambahan field opsional. Setelah wiring, dashboard terlihat di rail dan berfungsi end-to-end seperti di "Hasil yang bisa dijalankan".

## Guard
Cukup build berhasil, tanpa unit test:
- `go build ./...` lulus (daemon dan app).
- Build frontend lulus (`npm run build` di `frontend/`, termasuk `svelte-check` bila skrip build menjalankannya).
- Build wails berhasil dan aplikasi jalan untuk verifikasi manual pada fitur `dashboard-wiring`.
