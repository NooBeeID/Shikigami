# Plan: Render Welcome saat tidak ada workspace aktif

## Metadata
- Tiket: `welcome-screen-ticket-02-welcome-screen` (`docs/tickets/welcome-screen-ticket-02-welcome-screen.md`)
- Fitur: `welcome-wiring` (gelombang B, dikerjakan terakhir)
- ownedPaths: `frontend/src/App.svelte`
- Depends on foundation: tidak (`hasFoundation=false`). Bergantung pada fitur `welcome-logic` dan `welcome-ui` yang harus sudah merge (butuh `Welcome` dari `modules/welcome/index.ts`) dan pada tiket 1 yang sudah terimplementasi.

## Tujuan
Membuat area tengah aplikasi merender `Welcome` saat `$activeWorkspaceId === null`, selain itu `LayoutArea`. Lalu memverifikasi alur end-to-end secara manual, menjalankan build guard, dan mencatat temuan crash null (konsumen `activeWorkspace` di luar `App.svelte`) sebagai temuan untuk tiket 1 tanpa memperbaikinya.

## Path yang dimiliki
HANYA `frontend/src/App.svelte` yang boleh diubah. Jangan sentuh file fitur lain (`modules/welcome/logic.ts`, `Welcome.svelte`, `index.ts`), `WorkspaceTabs.svelte`, `lib/stores/*`, `modules/openfolder/*`, `modules/topbar/*`, `modules/persist/*`, kode Go, dan jangan memodifikasi `overview.md`. Jangan commit/merge di luar alur implement-feature.

## Kontrak
Dipakai (disediakan fitur/tiket lain; rujuk overview bagian 2):
- `activeWorkspaceId` (store) dari `./lib/stores` (barrel; tiket 1: bernilai `null` saat start dan setelah tab terakhir ditutup).
- `Welcome` dari `./modules/welcome` (`index.ts`: `export { default as Welcome } from './Welcome.svelte'`), tanpa props.
- `LayoutArea` dari `./modules/layout/LayoutArea.svelte` (sudah diimpor).

Disediakan: tidak ada API baru. Hanya perilaku render snippet `center`. Tanpa `registerPanel`, tanpa rute baru.

## Langkah
1. Pastikan prasyarat ada: `ls frontend/src/modules/welcome/` harus memuat `logic.ts`, `Welcome.svelte`, `index.ts`; `grep -n "activeWorkspaceId" frontend/src/lib/stores/*.ts` memastikan ekspor ada dan `openWorkspaceIds` dari tiket 1 ada. Bila nama ekspor berbeda sedikit, ikuti kode. Bila `modules/welcome` belum ada, berhenti dan laporkan (jangan membuatnya).
2. Edit `frontend/src/App.svelte`, baris impor store. Dari:
   `import { connection, initStores } from './lib/stores';`
   menjadi:
   `import { connection, initStores, activeWorkspaceId } from './lib/stores';`
3. Tambah impor setelah impor `LayoutArea`:
   `import { Welcome } from './modules/welcome';`
4. Ubah snippet `center` dari
   `{#snippet center()}<LayoutArea />{/snippet}`
   menjadi persis:
   `{#snippet center()}{#if $activeWorkspaceId === null}<Welcome />{:else}<LayoutArea />{/if}{/snippet}`
5. Jangan ubah bagian lain (koneksi, `initStores`, Toast, Palette, SettingsModal, ZoomHud, style).
6. Build guard:
   - `go build ./...` di root repo (harus lulus; tanpa perubahan Go).
   - `cd frontend && npm run build`, lalu `npm run check` (svelte-check; ada di skrip).
   Perbaiki hanya error yang berasal dari `App.svelte`. Error di file lain dicatat sebagai temuan, bukan diperbaiki.
7. Verifikasi manual end-to-end di aplikasi yang berjalan (daemon dibangun ulang agar `workspace.touch` tersedia; tiket 1 aktif). Perhatikan konsol devtools di setiap langkah:
   1. Start: welcome tanpa tab; "Belum ada workspace terakhir" bila belum ada riwayat; tanpa error konsol.
   2. Open Workspace -> folder baru -> tab dengan nama folder, welcome hilang; batal dialog -> tidak ada efek.
   3. Tutup tab (menu "Tutup") -> welcome kembali; workspace tadi muncul di "Terakhir dibuka" dengan path project pertama.
   4. Klik baris itu -> tab terbuka; entri hilang dari daftar.
   5. Open Workspace -> folder yang sama (termasuk varian slash akhir) -> workspace yang ada dibuka, tanpa duplikat.
   6. Create New Workspace -> input inline -> ganti nama (Enter/blur) -> tab dengan nama baru; Escape -> tab dengan nama default.
   7. Buka lebih dari 5 workspace lalu tutup semua -> daftar hanya 5 terbaru.
   8. Restart app -> welcome tanpa tab; daftar terisi dari riwayat.
   Bila tidak mungkin menjalankan GUI di lingkungan ini, laporkan langkah mana yang tidak terverifikasi, jangan klaim lulus.
8. Catat temuan: bila saat welcome tampil ada error konsol atau crash dari konsumen `activeWorkspace` yang tidak aman `null` (topbar, sidebar, panel kanan, dsb.), tuliskan di laporan akhir (file, gejala, pesan error) sebagai temuan untuk tiket 1. Jangan memperbaikinya di luar `App.svelte`, dan jangan menulis file laporan baru.

## Perubahan
- Diubah: `frontend/src/App.svelte` (dua impor, satu snippet `center`).
- Tidak ada file dibuat atau dihapus.

## Wiring
Fitur ini adalah langkah wiring itu sendiri: ia menyambungkan `Welcome` (dari `welcome-ui`) ke `App.svelte` berdasarkan state `activeWorkspaceId` dari tiket 1. Tidak ada yang diekspos untuk disambungkan lagi; tidak ada registry yang perlu didaftarkan.

## Selesai bila
- [ ] `App.svelte` mengimpor `activeWorkspaceId` dan `Welcome`, dan snippet `center` persis seperti langkah 4; tidak ada perubahan lain (`git diff --stat` hanya menunjukkan `frontend/src/App.svelte`).
- [ ] `go build ./...` lulus.
- [ ] `cd frontend && npm run build` lulus, dan `npm run check` tidak menambah error dari `App.svelte`.
- [ ] Verifikasi manual 8 skenario dilaporkan (lulus, atau alasan jelas bila tidak dapat dijalankan).
- [ ] Temuan crash null (bila ada) dicatat untuk tiket 1 dan tidak diperbaiki di luar `App.svelte`.
