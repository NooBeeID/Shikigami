# Plan: Item rail Agents dan wiring App

## Metadata

- Tiket: `docs/tickets/main-rail-ticket-01-main-rail.md` (`main-rail-ticket-01-main-rail`)
- Overview: `docs/plans/main-rail-ticket-01-main-rail/overview.md` (kontrak di bagian 2, wiring checklist di bagian 5)
- Fitur: `agents-rail-item` (tahap C, Wiring, dikerjakan terakhir)
- ownedPaths:
  - `frontend/src/modules/sidebar-left/rail.ts`
  - `frontend/src/modules/sidebar-left/index.ts`
  - `frontend/src/modules/shell/register.ts`
  - `frontend/src/App.svelte`
  - `frontend/src/modules/sidebar-left/AgentRailIcon.svelte` (berkas baru)
- Depends on foundation: tidak (`hasFoundation=false`). Tetapi fitur ini bergantung pada hasil tahap A dan B yang sudah di-merge: `registerRailItem` di `modules/shell/registry.ts`, serta `Rail.svelte` dan `RailPanelHost.svelte` di `modules/shell/`.

## Tujuan

Mendaftarkan item rail `agents` (title "Agen", order 0, komponen `AgentSidebar`) ke registry rail, memastikan modulnya terimpor tepat sekali saat startup, dan mengganti isi snippet `left` di `App.svelte` dari `AgentSidebar` menjadi `RailPanelHost`. Threads tetap section di dalam `AgentSidebar` (tidak diubah).

## Path yang dimiliki

HANYA berkas berikut yang boleh disentuh:

- `frontend/src/modules/sidebar-left/rail.ts` (baru)
- `frontend/src/modules/sidebar-left/AgentRailIcon.svelte` (baru)
- `frontend/src/modules/sidebar-left/index.ts`
- `frontend/src/modules/shell/register.ts`
- `frontend/src/App.svelte`

Pengecualian: `frontend/src/modules/shell/index.ts` boleh diedit hanya bila ekspor komponen `Rail`/`RailPanelHost` memang dibutuhkan (lihat Langkah 5). Opsional; lebih baik tidak diedit.

Jangan sentuh: `modules/shell/registry.ts`, `lib/stores/shell.ts`, `Rail.svelte`, `RailPanelHost.svelte`, `ShellFrame.svelte`, `shell.css`, `modules/persist/*`, `internal/workspace/*`, `AgentSidebar.svelte` dan berkas lain di `sidebar-left/`, `layout/*`, `lib/stores/layout.ts`, `provider/*`, `thread/*`, `app/*`, panel kanan.

## Kontrak

Dipakai (disediakan fitur lain; lihat overview bagian 2.1, 2.4, 2.5):

- `registerRailItem(spec: RailItemSpec): void` dari `frontend/src/modules/shell/registry.ts`. `RailItemSpec = { id: string; title: string; icon: Component; component: Component<any>; order?: number; visible?: Readable<boolean> }`. Mendaftar id yang sama menimpa (idempoten).
- `RailPanelHost.svelte` di `frontend/src/modules/shell/RailPanelHost.svelte`: tanpa props, me-mount komponen item rail aktif.
- `AgentSidebar` dari `./AgentSidebar.svelte` (sudah ada, tidak diubah).

Disediakan:

- Item rail terdaftar: `{ id: 'agents', title: 'Agen', icon: AgentRailIcon, component: AgentSidebar, order: 0 }`.
- `AgentRailIcon.svelte`: komponen Svelte tanpa props, SVG inline 20px, `currentColor`.

Aturan impor penting: `rail.ts` mengimpor `registerRailItem` dari `../shell/registry` (BUKAN `../shell` / `../shell/index`) untuk menghindari siklus, karena `NewSessionMenu.svelte` dan `SessionRow.svelte` di `sidebar-left` sudah mengimpor `../shell`.

## Langkah

Prasyarat: pastikan `modules/shell/registry.ts` mengekspor `registerRailItem` dan `modules/shell/RailPanelHost.svelte` ada. Bila tidak ada, itu berarti tahap A/B belum di-merge; jangan membuatnya sendiri, laporkan.

1. Buat `frontend/src/modules/sidebar-left/AgentRailIcon.svelte`. Tanpa props, tanpa `<script>` wajib. Isi:
   ```svelte
   <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor"
        stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
     <rect x="4" y="7" width="16" height="12" rx="3" />
     <path d="M12 7V4" />
     <circle cx="12" cy="3.5" r="0.75" fill="currentColor" />
     <circle cx="9" cy="13" r="1" fill="currentColor" stroke="none" />
     <circle cx="15" cy="13" r="1" fill="currentColor" stroke="none" />
   </svg>
   ```
   Tanpa dependensi baru. `aria-hidden` karena label ada di tombol rail.

2. Buat `frontend/src/modules/sidebar-left/rail.ts`:
   ```ts
   import { registerRailItem } from '../shell/registry';
   import AgentSidebar from './AgentSidebar.svelte';
   import AgentRailIcon from './AgentRailIcon.svelte';

   registerRailItem({ id: 'agents', title: 'Agen', icon: AgentRailIcon, component: AgentSidebar, order: 0 });
   ```
   Panggilan di level modul (efek samping). Tidak ada ekspor lain.

3. Edit `frontend/src/modules/sidebar-left/index.ts`: tambahkan `import './rail';` sebagai baris pertama (impor efek samping, dengan komentar singkat "daftarkan item rail 'agents'"). Biarkan ekspor yang ada (`AgentSidebar`, registry, tipe) tidak berubah.

4. Edit `frontend/src/modules/shell/register.ts`: tambahkan `import '../sidebar-left';` di samping impor efek samping yang ada (`../sidebar-right`, `../notifications`, `../palette`). Perbarui komentar di atasnya agar menyebut item rail `agents`. Verifikasi bahwa tidak ada impor lain yang sudah memuat `sidebar-left/index.ts` (cek `grep -rn "sidebar-left'" frontend/src`); bila `./rail` sampai dimuat dua kali itu tidak berbahaya karena ES module hanya mengevaluasi sekali dan `registerRailItem` idempoten, tetapi pastikan jalur impornya satu. Pastikan `modules/shell/index.ts` tidak mengimpor `register.ts` (tetap begitu), sehingga tidak ada siklus `shell/index -> register -> sidebar-left -> shell/index`.

5. Edit `frontend/src/App.svelte`:
   - Hapus baris `import AgentSidebar from './modules/sidebar-left/AgentSidebar.svelte';`.
   - Tambahkan `import RailPanelHost from './modules/shell/RailPanelHost.svelte';` (impor langsung; tidak perlu mengedit `modules/shell/index.ts`).
   - Ubah `{#snippet left()}<AgentSidebar />{/snippet}` menjadi `{#snippet left()}<RailPanelHost />{/snippet}`.
   - Tetap pertahankan `import './modules/shell/register';` (ini yang memuat item `agents`).
   - Opsi: bila ingin impor lewat `./modules/shell`, boleh menambah `export { default as Rail } from './Rail.svelte'; export { default as RailPanelHost } from './RailPanelHost.svelte';` di `modules/shell/index.ts` dan mengimpor dari sana; default plan ini adalah impor langsung sehingga `index.ts` tidak disentuh.

6. Jalankan guard (lihat Selesai bila). Bila `go build ./...` atau `npm run build` gagal karena sebab di luar ownedPaths, laporkan, jangan perbaiki di luar ownedPaths. Opsional: `npm run check` di `frontend/`; kegagalan yang sudah ada sebelumnya hanya dilaporkan.

## Perubahan

Dibuat:
- `frontend/src/modules/sidebar-left/rail.ts`
- `frontend/src/modules/sidebar-left/AgentRailIcon.svelte`

Diubah:
- `frontend/src/modules/sidebar-left/index.ts` (tambah `import './rail';`)
- `frontend/src/modules/shell/register.ts` (tambah `import '../sidebar-left';`)
- `frontend/src/App.svelte` (hapus impor `AgentSidebar`, impor `RailPanelHost`, snippet `left` merender `RailPanelHost`)

## Wiring

Fitur ini sendiri adalah langkah wiring (tahap C), jadi tidak ada yang diekspos untuk disambungkan nanti. Yang diekspos ke tiket lain: pola pendaftaran item rail (`registerRailItem` dipanggil dari `rail.ts` modul masing-masing dan modulnya diimpor sebagai efek samping dari `modules/shell/register.ts`), serta item `agents` dengan order 0 sebagai default aktif. Tiket main-rail-ticket-02-threads-rail akan menambah item `threads` (order 30) dan membersihkan `AgentSidebar`; itu di luar plan ini. Tidak ada registrasi Go/binding Wails baru dan tidak ada rute.

## Selesai bila

- `frontend/src/modules/sidebar-left/rail.ts` ada dan memanggil `registerRailItem` untuk `agents` (title "Agen", order 0, komponen `AgentSidebar`, ikon `AgentRailIcon`), mengimpor dari `../shell/registry`.
- Item `agents` terdaftar tepat sekali lewat satu jalur impor (`register.ts` -> `sidebar-left/index.ts` -> `rail.ts`).
- `App.svelte` tidak lagi mengimpor atau merender `AgentSidebar`; snippet `left` merender `RailPanelHost`.
- Tidak ada berkas di luar ownedPaths yang berubah (cek `git status`/`git diff --stat` terhadap daftar di "Path yang dimiliki").
- Guard lulus: `go build ./...` (root repo) dan `npm run build` di `frontend/`. Tanpa unit test.
- Verifikasi manual bila memungkinkan: rail terlihat dengan "Agen" aktif, section Threads masih tampil di panel Agen, toggle kiri hanya menyembunyikan panel, restart mempertahankan item aktif, sesi terminal dan thread tidak terputus.
