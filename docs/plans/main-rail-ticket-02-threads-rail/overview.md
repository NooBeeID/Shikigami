# Overview: main-rail-ticket-02-threads-rail

Tiket: `docs/tickets/main-rail-ticket-02-threads-rail.md`
Spec: `docs/superpowers/specs/2026-11-22-08-main-rail-design.md`
Prasyarat: `main-rail-ticket-01-main-rail` sudah merge (registry `registerRailItem`, store `activeRail`, komponen rail, item `agents`). Plan tiket 1: `docs/plans/main-rail-ticket-01-main-rail/overview.md`.

## 1. Ringkasan epic dan urutan eksekusi

Threads sekarang menempel sebagai section di `AgentSidebar` lewat `registerSidebarSection({ id: 'thread-sidebar' })` di `modules/thread/index.ts`. Epic ini memindahkannya menjadi item rail `threads` (order 30, ikon gelembung chat) memakai registry dari tiket 1. Perilaku Threads (daftar, "Thread baru", badge, event, peserta) tidak berubah.

Tidak ada Foundation (`hasFoundation=false`, tidak ada `00-foundation.md`): semua perubahan bersifat lokal per file dan tidak ada tipe/paket baru yang dipakai lintas fitur. Urutan:

1. Tahap A (paralel): `threads-panel-state`, `agents-sidebar-cleanup`. Keduanya tidak saling bergantung, dan kode tetap valid dikompilasi dengan wiring lama (index.ts masih mendaftarkan section).
2. Tahap B (Wiring, terakhir): `threads-rail-item`. Baru dikerjakan setelah A selesai.

Prasyarat worktree: perubahan belum di-commit di `layout/*`, `lib/stores/layout.ts`, `provider/*`, `thread/participants/terminal.ts`, `app/*` bukan milik epic ini dan tidak boleh disentuh.

## 2. Kontrak bersama

### 2.1 Yang dipakai dari tiket 1 (verifikasi nama persis di kode sebelum menulis)

```ts
// frontend/src/modules/shell/registry.ts
export interface RailItemSpec {
  id: string; title: string; icon: Component; component: Component<any>;
  order?: number; visible?: Readable<boolean>;
}
export function registerRailItem(spec: RailItemSpec): void;
```

- Impor `registerRailItem` dari `../shell/registry` (bukan barrel `../shell/index`, hindari siklus impor), kecuali kode tiket 1 menunjukkan lokasi lain.
- Panel dirender dengan props `{}`; hanya panel aktif yang di-mount (di-unmount saat item lain aktif).
- Ikon rail: komponen Svelte SVG inline, `viewBox="0 0 24 24"`, `fill="none"`, `stroke="currentColor"`, `aria-hidden="true"`, ukuran dari CSS induk. Tiru `frontend/src/modules/sidebar-left/AgentRailIcon.svelte` (tiket 1) untuk atribut stroke dan gaya; jangan menyentuh berkas itu.
- `aria-label` item rail berasal dari `title` oleh komponen rail tiket 1; tidak perlu dibuat di sini.

### 2.2 Yang disediakan epic ini

Item rail (didaftarkan di `frontend/src/modules/thread/index.ts`):

```ts
registerRailItem({ id: 'threads', title: 'Thread', icon: ThreadRailIcon, component: ThreadSection, order: 30 });
```

Tanpa `visible` (selalu tampil). Slot order yang dicadangkan, hanya sebagai komentar di `index.ts`: lark=10, git-github=20, git-gitlab=21 (agents=0, threads=30).

Ikon: `frontend/src/modules/thread/ThreadRailIcon.svelte` (SVG gelembung chat). `frontend/src/modules/thread/rail-icon.ts` hanya re-export: `export { default as ThreadRailIcon } from './ThreadRailIcon.svelte';`. `index.ts` mengimpor `ThreadRailIcon` dari `./rail-icon`. Keduanya dimiliki `threads-rail-item`.

Store panel (di `frontend/src/modules/thread/thread-store.ts`, dimiliki `threads-panel-state`):

```ts
export interface ThreadPanelState { scrollTop: number }
export const threadPanelState: Writable<ThreadPanelState>; // default { scrollTop: 0 }
```

Hanya `scrollTop` yang wajib. Jangan menambah filter/bagian terlipat baru yang tidak ada sekarang. Nama `threadPanelState` dan field `scrollTop` adalah kontrak tetap; hanya `ThreadSection.svelte` yang membacanya.

`ThreadSection.svelte`: props `{}` (tanpa `onerror`); galat tetap lewat store `threadError`. Teks UI: "Thread", "Thread baru", "Belum ada thread", tag "ditutup".

### 2.3 Aturan tetap lintas fitur

- `subscribeThreadEvents()` tetap dipanggil di level modul `thread/index.ts` (saat diimpor). `ThreadSection.svelte` tidak boleh memanggilnya atau `listen` event apa pun.
- `registerPaneKind`, `registerTabType`, `installParticipants()`, `installClose()`, `installRecovery()` tetap di tempatnya, urutan tidak berubah.
- Mekanisme `registerSidebarSection`/`sidebarSections` di `sidebar-left/registry.ts` dan ekspornya di `sidebar-left/index.ts` DIBIARKAN. `AgentSidebar.svelte` tetap merendernya generik (tanpa referensi ke Threads). Bila `grep` menemukan pemakai lain, jangan ubah perilakunya; catat di komentar.
- `onerror` di `AgentSidebar` dipertahankan (dipakai `WorkspaceHeader`, `SessionGroups`, `NewSessionMenu`).
- Teks UI bahasa Indonesia; identifier kode bahasa Inggris. Tanpa dependensi baru. Tanpa IPC baru dan tanpa perubahan Go.

### 2.4 Pesan IPC

Tidak ada. Epic ini hanya frontend; persist `activeRail` sudah disediakan tiket 1.

## 3. Daftar fitur

| Slug | Scope | ownedPaths |
|------|-------|-----------|
| `threads-panel-state` | Tambah store `threadPanelState` di `thread-store.ts`; ubah `ThreadSection.svelte`: props `{}`, hapus `onerror` dan pembungkus `border-bottom`, pembungkus scroll `overflow-y: auto; min-height: 0` setinggi panel, pulihkan `scrollTop` saat mount (setelah daftar dirender, mis. `tick`/`onMount`) dan simpan saat scroll/unmount. Tanpa subscriber event. Perilaku daftar/tombol/badge/pane tidak berubah. | `frontend/src/modules/thread/ThreadSection.svelte`, `frontend/src/modules/thread/thread-store.ts` |
| `agents-sidebar-cleanup` | `AgentSidebar.svelte` bebas referensi Threads dan tetap merender `sidebarSections()` generik; tambah komentar keputusan di atas `SidebarSectionSpec` di `registry.ts` (Threads kini item rail, mekanisme tersisa untuk section opsional di panel Agen, saat ini tanpa pendaftar, plus catatan hasil `grep` pemakai lain di `frontend/src`); `index.ts` tidak berubah kecuali ada pembersihan (default: tidak diubah). Jarak/tata letak minimal boleh disesuaikan. | `frontend/src/modules/sidebar-left/AgentSidebar.svelte`, `frontend/src/modules/sidebar-left/registry.ts`, `frontend/src/modules/sidebar-left/index.ts` |
| `threads-rail-item` | WIRING. Buat `ThreadRailIcon.svelte` dan `rail-icon.ts`; di `thread/index.ts` hapus impor/pemanggilan `registerSidebarSection` dan daftarkan item rail `threads`; tambah komentar slot order; verifikasi manual. | `frontend/src/modules/thread/index.ts`, `frontend/src/modules/thread/rail-icon.ts`, `frontend/src/modules/thread/ThreadRailIcon.svelte` |

Fitur `threads-rail-item` adalah fitur wiring dan dijalankan terakhir (tahap B).

## 4. Foundation

Tidak ada. Kontrak di bagian 2 cukup bagi tiap agent untuk menulis kode secara independen. Tidak ada berkas `00-foundation.md`.

## 5. Wiring checklist (`threads-rail-item`, setelah kedua fitur lain selesai dan di-merge)

1. Pastikan `threads-panel-state` dan `agents-sidebar-cleanup` sudah selesai; `thread-store.ts` mengekspor `threadPanelState`, `ThreadSection.svelte` berprops `{}`.
2. Buat `thread/ThreadRailIcon.svelte` dan `thread/rail-icon.ts` (bagian 2.2).
3. Edit `thread/index.ts`:
   - hapus `import { registerSidebarSection } from '../sidebar-left/registry'` dan baris `registerSidebarSection({ id: 'thread-sidebar', ... })`;
   - tambah `import { registerRailItem } from '../shell/registry'` dan `import { ThreadRailIcon } from './rail-icon'`;
   - panggil `registerRailItem({ id: 'threads', title: 'Thread', icon: ThreadRailIcon, component: ThreadSection, order: 30 })` di level modul;
   - tambah komentar: order rail dicadangkan lark=10, git-github=20, git-gitlab=21;
   - biarkan semua baris lain (`registerPaneKind`, `registerTabType`, `subscribeThreadEvents()`, `install*`) dan ekspor di bawahnya.
4. Jangan mengubah `App.svelte`, `shell/*`, `sidebar-left/rail.ts` (milik tiket 1); impor modul thread oleh bootstrap sudah ada, pastikan tidak terhapus.
5. Cek bahwa `shell/registry.ts` tidak mengimpor balik modul thread (hindari siklus). Bila `tsc`/vite melaporkan siklus, impor dari path file langsung seperti di 2.1.
6. Jalankan build guard (bagian 6). Verifikasi manual bila memungkinkan: rail memuat Agen dan Thread; Agen tanpa section Threads; klik Thread menampilkan daftar; buat thread, buka pane; pindah ke Agen lalu kembali, daftar mutakhir dan scroll bertahan; sesi terminal tidak terputus; pilih Thread, restart, item aktif tetap Thread; toggle kiri hanya menyembunyikan panel.

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- `cd frontend && npm run build` (vite build). Tambahan bila tersedia: `cd frontend && npm run check` (svelte-check).
- `go build ./...` dari root repo (tiket ini tidak mengubah Go; hanya memastikan tidak patah).

Tiap agent fitur menjalankan guard frontend sebelum selesai. Pada tahap A, build harus lulus dengan wiring lama (index.ts belum diubah).

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya; jangan menyentuh path fitur lain.
- `threads-panel-state` dan `agents-sidebar-cleanup` tidak berbagi berkas dan tidak saling menunggu.
- `thread/index.ts`, `rail-icon.ts`, `ThreadRailIcon.svelte` hanya disentuh `threads-rail-item` (wiring) pada tahap B.
- Dilarang menyentuh: `shell/*`, `lib/stores/shell.ts`, `sidebar-left/rail.ts`, `App.svelte`, `layout/*`, `lib/stores/layout.ts`, `provider/*`, `thread/participants/*`, `ThreadPane`, `thread-client.ts`, `close/`, `recovery/`, `activity/`, dan kode Go.
- Bila butuh perubahan di luar ownedPaths, laporkan sebagai catatan; jangan edit.
