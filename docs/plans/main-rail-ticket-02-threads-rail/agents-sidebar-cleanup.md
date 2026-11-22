# Plan: Bersihkan section Threads dari Agents

## Metadata

- Tiket: `main-rail-ticket-02-threads-rail` (`docs/tickets/main-rail-ticket-02-threads-rail.md`)
- Fitur: `agents-sidebar-cleanup`
- ownedPaths:
  - `frontend/src/modules/sidebar-left/AgentSidebar.svelte`
  - `frontend/src/modules/sidebar-left/registry.ts`
  - `frontend/src/modules/sidebar-left/index.ts`
- Depends on foundation: tidak (epic ini tidak punya Foundation; lihat `overview.md` bagian 4). Prasyarat di luar epic: tiket 1 (`main-rail-ticket-01-main-rail`) sudah merge.

## Tujuan

Pastikan `AgentSidebar.svelte` bebas dari referensi Threads dan tetap merender `sidebarSections()` secara generik (tanpa pendaftar, tidak ada yang tampil), dengan `onerror` dipertahankan. Dokumentasikan keputusan di `registry.ts`: Threads kini item rail, mekanisme `registerSidebarSection`/`sidebarSections` tetap ada tanpa pendaftar. Perilaku daftar sesi, grouping, filter, dan menu sesi baru tidak berubah. Keputusan desain (sudah final, lihat overview 2.3): mekanisme tidak dihapus dan ekspor di `index.ts` tidak berubah.

## Path yang dimiliki

HANYA tiga berkas ini yang boleh disentuh:

- `frontend/src/modules/sidebar-left/AgentSidebar.svelte`
- `frontend/src/modules/sidebar-left/registry.ts`
- `frontend/src/modules/sidebar-left/index.ts` (default: tidak diubah)

Jangan sentuh: file wiring (`frontend/src/modules/thread/index.ts`, `rail-icon.ts`, `ThreadRailIcon.svelte`), path fitur lain (`ThreadSection.svelte`, `thread-store.ts`), `sidebar-left/rail.ts`, `shell/*`, `App.svelte`, `layout/*`, kode Go. `modules/thread/index.ts` masih memanggil `registerSidebarSection({ id: 'thread-sidebar', ... })` sampai tahap wiring; itu disengaja dan harus tetap valid dikompilasi.

## Kontrak

Dipakai (tidak diubah; lihat overview 2.3):

- `registerSidebarSection(spec: SidebarSectionSpec): void` dan `sidebarSections(): Readable<SidebarSectionSpec[]>` di `registry.ts`, beserta ekspornya di `index.ts`. Signature dan perilaku tetap persis.
- `SidebarSectionSpec { id: string; component: Component<any>; order?: number }` tetap.
- `AgentSidebar.svelte`: props tetap tanpa props; `onerror` lokal (`err` state, banner galat) tetap dan tetap diteruskan ke `WorkspaceHeader`, `SessionGroups`, `NewSessionMenu`, `sessionListWrapper`, dan tiap section generik.

Disediakan: tidak ada API baru. Satu-satunya keluaran adalah komentar keputusan di `registry.ts`.

Temuan `grep` saat plan ditulis (verifikasi ulang di Langkah 1): pemanggil `registerSidebarSection` di `frontend/src` hanya `modules/thread/index.ts` (baris `thread-sidebar`, akan dihapus oleh fitur wiring `threads-rail-item`). Pemakai `sidebarSections` hanya `AgentSidebar.svelte`. `index.ts` sidebar-left hanya mengekspor ulang.

## Langkah

1. Verifikasi kondisi sekarang (tanpa mengubah apa pun). Jalankan dari root repo:
   - `grep -rn "registerSidebarSection\|sidebarSections" frontend/src`
   - `grep -n -i "thread" frontend/src/modules/sidebar-left/AgentSidebar.svelte` (harus kosong)
   Catat setiap pemakai selain yang disebut di Kontrak; bila ada pemakai lain, JANGAN ubah perilakunya, cukup catat di komentar Langkah 3.
   Catatan: tiket 1 mungkin sudah mengubah `AgentSidebar.svelte` (mis. label "AGENT", tata letak). Baca berkas apa adanya dan bangun di atasnya; jangan mengembalikan ke versi lama.

2. `AgentSidebar.svelte`:
   - Pertahankan `import { sessionListWrapper, sidebarSections } from './registry'`, `const sections = sidebarSections();`, `let err`, `const onerror`, blok `{#each $sections as s (s.id)} {@const S = s.component} <S {onerror} /> {/each}`, blok `sessionListWrapper`/`SessionGroups`, banner galat, dan `NewSessionMenu`.
   - Pastikan tidak ada impor, identifier, markup, atau komentar yang menyebut Threads / `ThreadSection` / `thread-sidebar`. Bila ditemukan (kemungkinan besar tidak ada), hapus.
   - Tambahkan komentar HTML/skrip singkat di dekat `{#each $sections ...}`: section generik opsional di panel Agen, saat ini tanpa pendaftar.
   - Penyesuaian jarak minimal (opsional): karena `{#each}` kosong tidak merender apa pun, tidak ada celah tambahan di antara `WorkspaceHeader` dan daftar sesi. Ubah CSS hanya bila tampilan jelas janggal; jangan mengubah `.side`, `.label`, `.err`, `.msg` tanpa alasan kuat dan jangan mengubah perilaku daftar/grouping/filter.

3. `registry.ts`: tambahkan blok komentar tepat di atas baris `export interface SidebarSectionSpec`, tanpa mengubah kode apa pun. Isi (Bahasa Indonesia, boleh disesuaikan redaksi):

   ```ts
   // Keputusan (tiket main-rail-ticket-02-threads-rail): Threads kini item rail
   // (registerRailItem, id 'threads'), bukan section di panel Agen. Mekanisme
   // registerSidebarSection/sidebarSections sengaja DIBIARKAN sebagai titik ekstensi
   // untuk section opsional di dalam panel Agen; saat ini TIDAK ADA pendaftar.
   // Pemakai di frontend/src (hasil grep): AgentSidebar.svelte (merender generik) dan
   // ekspor ulang di index.ts; satu-satunya pemanggil register lama
   // (modules/thread/index.ts, 'thread-sidebar') dihapus oleh wiring threads-rail-item.
   // Jangan ubah perilaku mekanisme ini tanpa memeriksa ulang pemakai (grep).
   ```

   Bila Langkah 1 menemukan pemakai lain, tambahkan namanya ke komentar ini.

4. `index.ts`: tidak diubah (default). Hanya bila tanpa sengaja terlihat impor patah, laporkan; jangan membersihkan ekspor `registerSidebarSection`, `sidebarSections`, `SidebarSectionSpec`.

5. Jalankan guard (bagian "Selesai bila"). Karena `thread/index.ts` belum diubah (tahap A), build harus lulus dengan wiring lama.

## Perubahan

- Diubah: `frontend/src/modules/sidebar-left/registry.ts` (hanya tambah komentar keputusan di atas `SidebarSectionSpec`).
- Diubah (hanya bila perlu): `frontend/src/modules/sidebar-left/AgentSidebar.svelte` (komentar kecil di dekat `{#each $sections}`, pembersihan referensi Threads bila ternyata ada, penyesuaian jarak minimal opsional).
- Tidak diubah: `frontend/src/modules/sidebar-left/index.ts`.
- Tidak ada berkas baru.

## Wiring

Fitur ini tidak mengekspos apa pun baru dan tidak menyambungkan apa pun. Untuk langkah wiring (`threads-rail-item`, tahap B): setelah fitur ini selesai, `AgentSidebar` merender `sidebarSections()` generik sehingga penghapusan `registerSidebarSection({ id: 'thread-sidebar', ... })` di `modules/thread/index.ts` otomatis membuat section Threads hilang dari panel Agen tanpa perubahan lain di `sidebar-left`. Jangan lakukan penghapusan itu di fitur ini.

## Selesai bila

- `AgentSidebar.svelte` tidak mengandung referensi Threads (impor, markup, komentar); `grep -i thread` pada berkas itu kosong.
- `AgentSidebar.svelte` tetap merender `sidebarSections()` generik dan `onerror` tetap ada serta diteruskan ke semua anak.
- `registry.ts` memuat komentar keputusan di atas `SidebarSectionSpec`; kode di dalamnya tidak berubah.
- Ekspor di `index.ts` tidak berubah.
- Hanya berkas dalam ownedPaths yang berubah (`git status` tidak menunjukkan berkas lain dari fitur ini).
- Guard: `cd frontend && npm run build` sukses (tambahan bila tersedia: `npm run check`); `go build ./...` dari root repo tetap lulus. Tanpa unit test.
