# Plan: Daftarkan Threads sebagai item rail (wiring)

## Metadata

- Tiket: `main-rail-ticket-02-threads-rail` (`docs/tickets/main-rail-ticket-02-threads-rail.md`)
- Fitur: `threads-rail-item` (WIRING, tahap B, dikerjakan terakhir setelah `threads-panel-state` dan `agents-sidebar-cleanup`)
- ownedPaths:
  - `frontend/src/modules/thread/index.ts`
  - `frontend/src/modules/thread/rail-icon.ts`
  - `frontend/src/modules/thread/ThreadRailIcon.svelte`
- Depends on foundation: tidak (tidak ada `00-foundation.md`). Bergantung pada tiket 1 (`registerRailItem`) dan pada dua fitur paralel di epic ini (lihat Kontrak).

## Tujuan

Memindahkan Threads dari section `AgentSidebar` (`registerSidebarSection({ id: 'thread-sidebar' })`) menjadi item rail `threads` (order 30, ikon gelembung chat) memakai registry rail dari tiket 1. Perilaku Threads (daftar, "Thread baru", badge, event, peserta) tidak berubah. Rujukan kontrak lengkap: `docs/plans/main-rail-ticket-02-threads-rail/overview.md` bagian 2 dan 5.

## Path yang dimiliki

HANYA tiga berkas ini yang boleh disentuh:

- `frontend/src/modules/thread/index.ts` (diubah)
- `frontend/src/modules/thread/rail-icon.ts` (baru)
- `frontend/src/modules/thread/ThreadRailIcon.svelte` (baru)

Jangan menyentuh: `ThreadSection.svelte`, `thread-store.ts` (fitur `threads-panel-state`); `sidebar-left/*` termasuk `AgentSidebar.svelte`, `registry.ts`, `index.ts`, `rail.ts` (fitur `agents-sidebar-cleanup` / tiket 1); `shell/*`, `lib/stores/shell.ts`, `App.svelte`, `layout/*`, `provider/*`, `thread/participants/*`, kode Go. Bila butuh perubahan di luar path ini, laporkan sebagai catatan, jangan edit. `overview.md` tidak boleh diubah.

## Kontrak

Dipakai (verifikasi nama persis di kode sebelum menulis):

- `registerRailItem(spec: RailItemSpec): void` dengan `RailItemSpec { id: string; title: string; icon: Component; component: Component<any>; order?: number; visible?: Readable<boolean> }` dari `frontend/src/modules/shell/registry.ts` (disediakan tiket 1; lihat overview 2.1). Impor dari `../shell/registry`, bukan barrel `../shell/index`.
  - Catatan saat plan ditulis: di worktree ini `shell/registry.ts` belum memuat `registerRailItem`. Itu prasyarat (tiket 1 harus sudah merge). Bila saat implementasi fungsi itu belum ada, HENTIKAN dan laporkan; jangan menambahkannya sendiri di `shell/*`. Bila tiket 1 menaruhnya di lokasi lain, pakai lokasi itu.
- `ThreadSection` (`./ThreadSection.svelte`), kini berprops `{}` setelah `threads-panel-state` selesai. Jangan mengubahnya.
- Pola ikon: tiru `frontend/src/modules/sidebar-left/AgentRailIcon.svelte` (tiket 1) untuk atribut dan gaya; baca saja, jangan diubah. Bila berkas itu tidak ada, pakai atribut di Langkah 1.

Disediakan:

```ts
registerRailItem({ id: 'threads', title: 'Thread', icon: ThreadRailIcon, component: ThreadSection, order: 30 });
```

Tanpa `visible` (selalu tampil). `rail-icon.ts`: `export { default as ThreadRailIcon } from './ThreadRailIcon.svelte';`. Slot order rail dicadangkan (komentar saja): agents=0, lark=10, git-github=20, git-gitlab=21, threads=30. Tidak ada pesan IPC.

## Langkah

1. Prasyarat: pastikan `threads-panel-state` dan `agents-sidebar-cleanup` selesai, `thread-store.ts` mengekspor `threadPanelState`, `ThreadSection.svelte` berprops `{}`. Pastikan `registerRailItem` ada di `shell/registry.ts` (`grep -n registerRailItem frontend/src/modules/shell/registry.ts`).
2. Buat `frontend/src/modules/thread/ThreadRailIcon.svelte`: komponen tanpa props berisi satu `<svg>` gelembung chat:
   - atribut: `viewBox="0 0 24 24"`, `fill="none"`, `stroke="currentColor"`, `stroke-width="1.8"`, `stroke-linecap="round"`, `stroke-linejoin="round"`, `aria-hidden="true"`, `width="100%"`/`height="100%"` atau tanpa ukuran bila `AgentRailIcon.svelte` begitu (ukuran dari CSS induk). Samakan stroke-width dan ukuran dengan `AgentRailIcon.svelte`.
   - isi path contoh: `<path d="M21 12a8 8 0 0 1-11.6 7.1L4 20.5l1.4-4.6A8 8 0 1 1 21 12Z" />` ditambah dua/tiga titik opsional `<circle cx="8.5" cy="12" r="0.6" />`, `<circle cx="12" cy="12" r="0.6" />`, `<circle cx="15.5" cy="12" r="0.6" />` (atau cukup path gelembung). Tanpa warna hard-coded, tanpa `<style>` yang menetapkan warna.
3. Buat `frontend/src/modules/thread/rail-icon.ts` berisi tepat: `export { default as ThreadRailIcon } from './ThreadRailIcon.svelte';`
4. Edit `frontend/src/modules/thread/index.ts`:
   - hapus baris `import { registerSidebarSection } from '../sidebar-left/registry';`
   - hapus baris `registerSidebarSection({ id: 'thread-sidebar', component: ThreadSection, order: 0 });`
   - tambah `import { registerRailItem } from '../shell/registry';` dan `import { ThreadRailIcon } from './rail-icon';`
   - di tempat baris yang dihapus, tambah komentar lalu pemanggilan level modul:
     ```ts
     // Slot order rail (dicadangkan): agents=0, lark=10, git-github=20, git-gitlab=21, threads=30.
     registerRailItem({ id: 'threads', title: 'Thread', icon: ThreadRailIcon, component: ThreadSection, order: 30 });
     ```
   - biarkan tak berubah: impor `ThreadSection`, `registerPaneKind`, `registerTabType`, `subscribeThreadEvents()`, `installParticipants()`, `installClose()`, `installRecovery()` (urutan sama) dan seluruh ekspor di bawahnya.
5. Cek siklus: `shell/registry.ts` tidak mengimpor modul thread. Bila vite/tsc melaporkan siklus impor, impor dari path file langsung.
6. Jalankan guard (lihat Selesai bila). Verifikasi manual bila memungkinkan: rail memuat Agen dan Thread; Agen tanpa section Threads; klik Thread menampilkan daftar; buat thread, buka pane; pindah ke Agen lalu kembali, daftar mutakhir dan scroll bertahan; sesi terminal tidak terputus; pilih Thread, restart, item aktif tetap Thread; toggle kiri hanya menyembunyikan panel. Bila tidak bisa menjalankan aplikasi, catat bagian yang tidak terverifikasi.

## Perubahan

- Baru: `frontend/src/modules/thread/ThreadRailIcon.svelte`
- Baru: `frontend/src/modules/thread/rail-icon.ts`
- Diubah: `frontend/src/modules/thread/index.ts` (hapus `registerSidebarSection` dan impornya; tambah impor `registerRailItem`, `ThreadRailIcon`; tambah komentar slot dan `registerRailItem(...)`)

## Wiring

Fitur ini adalah langkah wiring itu sendiri. Yang diekspos: item rail `threads` melalui `registerRailItem` saat modul `thread/index.ts` diimpor. Impor modul thread oleh bootstrap aplikasi sudah ada; pastikan tidak terhapus, tetapi jangan mengubah `App.svelte`, `shell/*`, atau `sidebar-left/rail.ts` (milik tiket 1). Tidak ada sambungan lain yang perlu dibuat.

## Selesai bila

- `index.ts` tidak lagi mendaftarkan `thread-sidebar` dan tidak mengimpor `sidebar-left/registry`; mendaftarkan item rail `threads` (title "Thread", order 30, tanpa `visible`) dengan komentar slot order lark=10, git-github=20, git-gitlab=21.
- `ThreadRailIcon.svelte` memakai `currentColor`, `viewBox="0 0 24 24"`, `fill="none"`, `aria-hidden="true"`; `rail-icon.ts` hanya re-export.
- `subscribeThreadEvents()` dan semua `install*`/`register*` lain tetap di level modul; tidak ada pemanggilan di komponen.
- Hanya tiga path milik fitur ini yang berubah (`git status` memastikan).
- Guard: `cd frontend && npm run build` sukses (tambahan bila tersedia: `npm run check`); `go build ./...` dari root tidak patah. Tanpa unit test.
