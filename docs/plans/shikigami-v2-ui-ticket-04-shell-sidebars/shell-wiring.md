# Plan: Wiring: aktifkan shell baru

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-04-shell-sidebars.md`
- Fitur: `shell-wiring` (dikerjakan TERAKHIR, sendiri, setelah semua fitur lain tergabung)
- ownedPaths:
  - `frontend/src/App.svelte`
  - `frontend/src/modules/shell/register.ts`
  - `frontend/src/modules/shell/SidebarArea.svelte`
  - `frontend/src/modules/shell/StatusBar.svelte`
- Depends on foundation: ya (`shell-frame`, plan `00-foundation.md`), ditambah semua fitur paralel (`topbar`, `sidebar-left-agent`, `sidebar-right-files`, `command-palette`, `notification-bell`) dan tiket 02/03 (sudah tergabung).

## Tujuan

Mengaktifkan shell v2: `App.svelte` merender `ShellFrame` (topbar, sidebar kiri Agent, area layout, panel kanan) plus `Palette` dan toast Open Folder, tanpa lagi memakai `SidebarArea`/`StatusBar` v1. Connection state dan `initStores` dipertahankan apa adanya. Checklist rujukan: `overview.md` bagian 5 (jangan diulang; ikuti). Kontrak komponen: `overview.md` bagian 2.7.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `frontend/src/App.svelte`
- `frontend/src/modules/shell/register.ts` (baru)
- `frontend/src/modules/shell/SidebarArea.svelte` (hapus atau shim)
- `frontend/src/modules/shell/StatusBar.svelte` (hapus atau shim)

Dilarang menyentuh: file lain di `modules/shell/` (foundation: `ShellFrame`, `RightPanel`, `registry.ts`, `helpers.ts`, `index.ts`, dll.), path fitur lain, `main.ts`, `lib/stores/*`, `styles/*`, `modules/openfolder` (termasuk `Toast.svelte`), `modules/status/StatusBar.svelte` (itu komponen lain, bukan shell v1), `modules/tabs`, `package.json`, `Makefile`, kode Go.

## Kontrak

Dipakai (semua sudah ada saat wiring dimulai; lihat overview 2.3, 2.4, 2.7, 2.8):

- `ShellFrame` dan `RightPanel` dari `./modules/shell` (`index.ts` foundation). `ShellFrame` props: `{ topbar, left, center, right }` bertipe `Snippet`. `ShellFrame` sudah memanggil `installShellShortcuts()` sendiri.
- `Topbar` dari `./modules/topbar/Topbar.svelte`; `AgentSidebar` dari `./modules/sidebar-left/AgentSidebar.svelte`; `LayoutArea` dari `./modules/layout/LayoutArea.svelte`; `Palette` dari `./modules/palette/Palette.svelte`. Semua tanpa props wajib.
- Side-effect: `../sidebar-right` (mendaftar `registerRightTab({id:'files',...})`), `../notifications` (mendaftar `registerTopbarItem({id:'notifications',...})`), `../palette` (aksi dan sumber palet bawaan).
- `Toast` dari `./modules/openfolder/Toast.svelte` (membaca store `toast` sendiri; elemen `<span>` inline tanpa posisi).
- `connection`, `initStores` dari `./lib/stores`.

Disediakan: tidak ada simbol baru. `register.ts` tanpa ekspor.

## Langkah

Prasyarat: pastikan berkas berikut ada: `modules/shell/{index.ts,ShellFrame.svelte,RightPanel.svelte,registry.ts}`, `modules/topbar/Topbar.svelte`, `modules/sidebar-left/AgentSidebar.svelte`, `modules/sidebar-right/index.ts`, `modules/palette/{index.ts,Palette.svelte}`, `modules/notifications/index.ts`, `modules/layout/LayoutArea.svelte`, `modules/openfolder/Toast.svelte`. Bila ada yang hilang, hentikan dan laporkan blocker (jangan membuat stub).

1. **`frontend/src/modules/shell/register.ts` (BARU).** Isi hanya tiga impor side-effect, tanpa ekspor, dengan komentar singkat bahwa berkas ini sengaja tidak diimpor oleh `modules/shell/index.ts` (hindari siklus):
   ```ts
   // Side-effect: daftarkan tab kanan 'files', item topbar 'notifications', aksi/sumber palet.
   // Jangan diimpor dari modules/shell/index.ts (hindari siklus); App.svelte yang mengimpornya.
   import '../sidebar-right';
   import '../notifications';
   import '../palette';
   ```
2. **`frontend/src/App.svelte` bagian `<script>`.**
   - Pertahankan impor `./styles/theme.css`, `./styles/base.css`, `onMount`/`onDestroy`, `connection`/`initStores`, serta fungsi `start()`, `onMount`, `onDestroy` persis seperti sekarang.
   - Hapus impor `SidebarArea` dan `StatusBar` (`LayoutArea` tetap diimpor). Catatan: `TabBar`/`TabContent` sudah tidak diimpor di App.svelte saat ini; pastikan memang tidak ada.
   - Tambah impor: `import { ShellFrame, RightPanel } from './modules/shell';`, `import './modules/shell/register';`, `import Topbar from './modules/topbar/Topbar.svelte';`, `import AgentSidebar from './modules/sidebar-left/AgentSidebar.svelte';`, `import Palette from './modules/palette/Palette.svelte';`, `import Toast from './modules/openfolder/Toast.svelte';`.
3. **`App.svelte` markup.** Cabang `error` dan `connecting` TIDAK diubah. Ganti isi cabang `{:else}`:
   ```svelte
   {:else}
     <ShellFrame>
       {#snippet topbar()}<Topbar />{/snippet}
       {#snippet left()}<AgentSidebar />{/snippet}
       {#snippet center()}<LayoutArea />{/snippet}
       {#snippet right()}<RightPanel />{/snippet}
     </ShellFrame>
     <Palette />
     <div class="toast-host"><Toast /></div>
   {/if}
   ```
   `Palette` dan `toast-host` berada di dalam cabang `{:else}` yang sama sebagai sibling `ShellFrame`.
4. **`App.svelte` gaya.** Pertahankan `.center`, `.msg`, `h2`. Hapus `.shell`, `.side`, `.main`, `.content`, `.status`. Tambah `.toast-host { position: fixed; right: 12px; bottom: 12px; z-index: 50; pointer-events: none; }` (toast v1 adalah `<span>` inline tanpa posisi; wrapper ini membuat umpan balik Open Folder tetap terlihat setelah statusbar v1 pensiun). Jangan mengubah `Toast.svelte`. Pastikan `ShellFrame` mengisi tinggi penuh (root `.shell-frame` sudah `height:100%`); bila `#app`/`body` perlu `height:100%`, itu sudah diatur `base.css`, jangan diubah.
5. **Pensiun v1.** Jalankan `grep -rn "SidebarArea\|shell/StatusBar\|from './StatusBar" frontend/src` (abaikan `modules/status/StatusBar.svelte` dan impor `./StatusBar.svelte` di `modules/status/index.ts`, yang merujuk berkas lain milik modul `status`). Bila setelah langkah 2 tidak ada impor tersisa ke `modules/shell/SidebarArea.svelte` dan `modules/shell/StatusBar.svelte`, HAPUS keduanya. Bila masih ada pengimpor di luar path ini, ubah berkas yang diimpor menjadi shim kosong (`<!-- shim: shell v1 dipensiunkan; tidak dirender -->`) tanpa `<script>` panel registry.
6. **Registrasi `registerPanel` lama dibiarkan** (workspace-sidebar, status-badges, status-summary, panel files, openfolder-toast): tidak dirender lagi, tidak diubah modulnya.
7. **Side-effect modul lain.** `main.ts` sudah memuat `./modules/*/index.ts` lewat `import.meta.glob` eager, mencakup `appmenu`, `openfolder`, `zoom`, `layout`, `status`. Tidak menambah impor untuk itu di `App.svelte`. Bila `modules/layout/index.ts` atau `modules/zoom/index.ts` ternyata tidak ada, laporkan blocker.
8. **Guard.** `cd frontend && npm run build` lalu `make build` penuh dari root repo. Perbaiki galat tipe pada berkas yang dimiliki. Cek manual (bukan guard) sesuai overview bagian 5 butir 6: tab `registerRightTab` dummy dari modul lain muncul tanpa mengubah kode shell (hapus setelahnya); Cmd+B, Cmd+Opt+B, Cmd+K; klik sesi memfokuskan pane; klik file membuka pane file; Open Folder memunculkan toast; layar error/connecting tetap berfungsi.

## Perubahan

- BARU: `frontend/src/modules/shell/register.ts`
- UBAH: `frontend/src/App.svelte` (impor, cabang `{:else}`, CSS)
- HAPUS (atau shim bila masih diimpor): `frontend/src/modules/shell/SidebarArea.svelte`, `frontend/src/modules/shell/StatusBar.svelte`

## Wiring

Fitur ini ADALAH langkah wiring: tidak ada yang diekspos untuk disambungkan kemudian. Hal yang disiapkan untuk tiket lain: `registerRightTab` siap dipakai tiket 05 (tab Activity) tanpa mengubah shell, dan `registerTopbarItem` siap untuk item topbar baru.

## Selesai bila

- `App.svelte` memakai `ShellFrame` dengan snippet `topbar/left/center/right`, merender `<Palette />` dan `<Toast />`, tanpa impor `SidebarArea`/`StatusBar` v1 atau `TabBar`/`TabContent`.
- `register.ts` ada dan diimpor `App.svelte`; tab `files` dan item `notifications` terdaftar lewat registri (bukan impor langsung `RightPanel` -> `FilesTab`).
- `initStores`/`start`/`dispose` dan cabang error/connecting tidak berubah.
- `SidebarArea.svelte` dan `StatusBar.svelte` v1 dihapus (atau shim kosong bila masih diimpor), tidak ada impor tersisa yang rusak.
- Tidak ada berkas di luar `ownedPaths` yang diubah.
- Guard: `make build` dari root lulus (termasuk `npm run build` di `frontend/`).
