# Plan: Wiring, daftarkan dashboard ke rail

## Metadata

- Tiket: `docs/tickets/agent-dashboard-ticket-01-agent-dashboard.md` (id `agent-dashboard-ticket-01-agent-dashboard`)
- Fitur: `dashboard-wiring` (Tahap C, terakhir)
- ownedPaths: `frontend/src/modules/dashboard/index.ts`
- Depends on foundation: ya. Tidak ada Foundation terpisah, tetapi fitur ini baru boleh dikerjakan setelah SEMUA fitur lain epic ini di-merge: `dashboard-foundation`, `status-list-fields`, `dashboard-store`, `dashboard-panel-ui`, `rail-badge`.
- Rujukan: `overview.md` di folder yang sama (bagian 2.5, 2.6, 5, 6).

## Tujuan

Mendaftarkan item rail "Dashboard" (order 50, badge `waitingCount`) lewat satu file `dashboard/index.ts`, lalu memverifikasi end-to-end bahwa dashboard berfungsi seperti "Hasil yang bisa dijalankan" di tiket: tiga sesi tampil di kelompok benar, klik baris memfokuskan/membuka pane, shell tidak muncul, badge muncul/hilang, tidak ada timer saat panel tertutup, tidak ada notifikasi macOS baru.

## Path yang dimiliki

HANYA boleh membuat/mengubah:

- `frontend/src/modules/dashboard/index.ts`

Jangan menyentuh: `App.svelte`, `main.ts`, `modules/shell/register.ts`, `modules/shell/registry.ts`, `Rail.svelte`, `shell.css`, `modules/status/*`, file lain di `modules/dashboard/` (types.ts, store.ts, DashboardPanel.svelte, AgentRow.svelte, DashboardRailIcon.svelte), `internal/status/*`, dan perubahan belum di-commit di `sidebar-right/FilesTab.svelte`. Bila verifikasi menemukan bug di file fitur lain, laporkan, jangan perbaiki di sini.

## Kontrak

Dipakai (disediakan fitur lain, lihat overview 2.2 sampai 2.6; jangan diubah):

- `registerRailItem(spec: RailItemSpec)` dari `../shell/registry`; `RailItemSpec` punya `badge?: Readable<number | boolean>` (disediakan `dashboard-foundation`, dirender `rail-badge`).
- `default` dari `./DashboardPanel.svelte` (tanpa props), `default` dari `./DashboardRailIcon.svelte` (tanpa props), keduanya dari `dashboard-panel-ui`.
- `waitingCount: Readable<number>` dari `./store` (nyata dari `dashboard-store`).
- `startStatusSync()` sudah dipanggil oleh `frontend/src/modules/status/index.ts` (verifikasi sudah ada: baris `startStatusSync();`).
- `ensureActivityStore()` dipanggil `DashboardPanel` di `onMount` (verifikasi, jangan dipanggil di `index.ts`).

Disediakan: tidak ada ekspor baru. Efek samping satu kali: item rail `{ id: 'dashboard', title: 'Dashboard', icon: DashboardRailIcon, component: DashboardPanel, order: 50, badge: waitingCount }`.

Fakta pemuatan modul (sudah diverifikasi): `frontend/src/main.ts` memuat `import.meta.glob('./modules/*/index.ts', { eager: true })`, sehingga `modules/dashboard/index.ts` otomatis ikut dimuat tanpa edit file lain. Pola contoh: `frontend/src/modules/grimoire/index.ts`.

## Langkah

1. Pastikan prasyarat sudah di-merge: `ls frontend/src/modules/dashboard/` harus memuat `types.ts`, `store.ts`, `DashboardPanel.svelte`, `AgentRow.svelte`, `DashboardRailIcon.svelte`; `grep -n "badge" frontend/src/modules/shell/registry.ts frontend/src/modules/shell/Rail.svelte`; `grep -n "reason" internal/status/types.go`. Bila ada yang hilang, berhenti dan laporkan `ok:false` dengan alasan (jangan membuat file fitur lain).
2. Verifikasi kait yang sudah ada tanpa mengubahnya:
   - `grep -n "startStatusSync" frontend/src/modules/status/index.ts` menunjukkan pemanggilan di tingkat modul.
   - `grep -n "ensureActivityStore" frontend/src/modules/dashboard/*` hanya muncul di `DashboardPanel.svelte` dalam `onMount`, tidak di `store.ts` atau `index.ts`.
   - `grep -rn "setInterval" frontend/src/modules/dashboard/` hanya di `DashboardPanel.svelte`, di dalam cleanup `$effect`/`onDestroy`, dengan pengecekan `document.hidden`.
   - `grep -rniE "notif|Notification" frontend/src/modules/dashboard/` tidak menunjukkan pemanggilan notifikasi macOS.
3. Buat `frontend/src/modules/dashboard/index.ts` dengan isi persis berikut:

```ts
import { registerRailItem } from '../shell/registry';
import DashboardPanel from './DashboardPanel.svelte';
import DashboardRailIcon from './DashboardRailIcon.svelte';
import { waitingCount } from './store';

// Slot order rail: agents=0, lark=10, git-github=20, git-gitlab=21, threads=30, grimoire=40, dashboard=50.
// Badge titik pada ikon rail menyala saat ada sesi agent berstatus waiting.
export function registerDashboardPanel(): void {
  registerRailItem({
    id: 'dashboard',
    title: 'Dashboard',
    icon: DashboardRailIcon,
    component: DashboardPanel,
    order: 50,
    badge: waitingCount,
  });
}
registerDashboardPanel();
```

   Jangan impor `../status/store` di sini bila `status/index.ts` sudah memanggil `startStatusSync()` (modul `status` dimuat glob yang sama); tambahkan impor `../status` hanya bila verifikasi langkah 2 menunjukkan pemanggilan itu tidak ada, dan catat alasannya di komentar.
4. Build guard, jalankan dari root repo: `go build ./...`; lalu `cd frontend && npm run build`; jalankan juga `npm run check` bila skrip ada. Semua harus lulus.
5. Verifikasi pola glob menjangkau modul baru: `grep -n "import.meta.glob" frontend/src/main.ts` dan konfirmasi `modules/dashboard/index.ts` cocok dengan `./modules/*/index.ts`.
6. Build wails dan jalankan aplikasi (`wails dev` atau `wails build` lalu buka app; gunakan skill `run` bila tersedia). Matikan hanya proses yang Anda luncurkan sendiri (jangan `pkill` pola luas).
7. Verifikasi manual end-to-end:
   - Rail menampilkan ikon "Dashboard" setelah Grimoire; item rail lain (Agen, Lark, Git, Threads, Grimoire) berperilaku sama.
   - Buka tiga sesi agent (satu berjalan, satu menunggu via prompt permission/Notification, satu diakhiri sehingga source `exit`) plus satu sesi shell. Tiap sesi muncul di kelompok yang benar, berpindah tanpa muat ulang, sesi exit menjadi Selesai dalam satu event, sesi shell tidak pernah muncul.
   - Klik baris memfokuskan pane yang ada, atau membukanya bila belum ada pane.
   - Badge titik di ikon Dashboard muncul saat ada waiting (juga saat panel belum pernah dibuka, membuktikan `startStatusSync` aktif) dan hilang saat 0; `aria-label` tombol menyebut "(ada sesi menunggu)" saat aktif.
   - Panel tertutup (pindah ke item rail lain): tidak ada timer. Cek di devtools bahwa tidak ada interval dari panel dashboard yang berjalan (mis. memantau `setInterval` lewat Performance/Timers atau memastikan DOM panel tidak ada), dan timer berhenti saat `document.hidden`.
   - Tidak ada notifikasi macOS tambahan di luar yang sudah ada sebelumnya.
8. Catat hasil verifikasi ringkas (lulus/gagal per butir) di laporan akhir, bukan di file baru.

## Perubahan

- Dibuat: `frontend/src/modules/dashboard/index.ts`
- Tidak ada file lain yang diubah.

## Wiring

Fitur ini ADALAH langkah wiring penutup. Yang diekspos: item rail `dashboard` terdaftar sekali via `registerRailItem` (efek samping saat modul dimuat oleh `import.meta.glob` di `main.ts`), plus fungsi `registerDashboardPanel()` mengikuti pola `registerGrimoirePanel()`. Tidak ada wiring lanjutan: jangan edit `App.svelte`, `main.ts`, atau `shell/register.ts`.

## Selesai bila

- `frontend/src/modules/dashboard/index.ts` ada dan memanggil `registerRailItem` tepat sekali dengan `id: 'dashboard'`, `order: 50`, `badge: waitingCount`, dan komentar slot rail.
- Tidak ada file selain `index.ts` yang berubah (`git status` untuk path fitur ini hanya menunjukkan file itu).
- Guard: `go build ./...` dan `npm run build` di `frontend/` lulus (plus `npm run check` bila ada); build wails lulus dan aplikasi jalan.
- Verifikasi manual langkah 7 lulus seluruhnya, termasuk badge, klik baris, shell tersembunyi, tanpa timer saat panel tertutup, dan tanpa notifikasi macOS baru.
- Tanpa unit test, tanpa dependensi baru.
