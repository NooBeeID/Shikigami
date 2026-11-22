---
id: main-rail-ticket-02-threads-rail
title: Threads sebagai item rail sendiri
spec: docs/superpowers/specs/2026-11-22-08-main-rail-design.md
status: implemented
wave: 2
depends_on: [main-rail-ticket-01-main-rail]
---
# Threads sebagai item rail sendiri

## Tujuan
Threads sekarang menempel sebagai section di dalam `AgentSidebar` lewat `registerSidebarSection`. Spec rail menu utama menghendaki Threads menjadi panel sendiri dengan ikon bergaya chat di rail, sehingga Agents hanya berisi sesi agen. Epic ini memindahkan `ThreadSection` menjadi item rail `threads` (order 30) memakai registry dari tiket 1, tanpa mengubah perilaku Threads (daftar, tombol "Thread baru", badge, event, peserta). Karena panel hanya di-mount saat aktif, state panel yang berarti dipindah ke store dan subscriber event thread tetap hidup di luar komponen.

## Hasil yang bisa dijalankan
Di atas tiket 1, setelah tiket ini selesai:
- Rail kini berisi dua item: Agen (order 0) dan Thread (order 30, ikon gelembung chat, `aria-label` "Thread").
- Klik item Thread menampilkan daftar thread di panel kiri: baris dengan titik aktif/idle, judul, `ReadyBadge`, tag "ditutup", tombol "Thread baru" yang membuka `NewThreadDialog`, pesan "Belum ada thread" bila kosong. Klik baris membuka pane thread seperti sebelumnya.
- Panel Agen tidak lagi menampilkan section Threads; hanya header workspace, daftar sesi (grouping/filter tidak berubah), dan menu sesi baru.
- Saat item Agen aktif (panel Thread tidak ter-mount), thread tetap menerima event: pesan baru, peserta, aktivitas, status close/recovery tetap masuk store; saat kembali ke Thread, daftar sudah mutakhir.
- Sesi terminal dan thread yang berjalan tidak terputus saat berpindah item rail.
- Pilihan item aktif (Agen/Thread) bertahan setelah restart (mekanisme persist dari tiket 1).
- Build frontend lulus dan aplikasi berjalan.

## Batasan
In scope:
- Mengganti `registerSidebarSection({ id: 'thread-sidebar', ... })` di `modules/thread/index.ts` dengan `registerRailItem`.
- Ikon SVG inline gelembung chat sebagai komponen Svelte.
- Menyesuaikan `ThreadSection.svelte` agar menerima props `{}` dan mengambil state dari store (tidak lagi menerima `onerror`).
- Memindahkan state panel yang berarti ke store thread.
- Menghapus rendering section Threads dari `AgentSidebar` dan memutuskan nasib mekanisme `sidebarSections`.

Out of scope:
- Mengubah shell/rail/registry rail/persist (milik tiket 1), `layout/*`, `lib/stores/layout.ts`, `provider/*`, `thread/participants/terminal.ts` (perubahan belum di-commit milik pekerjaan lain; harus sudah di-commit atau dipindah sebelum implementasi).
- Mengubah perilaku daftar sesi, grouping, filter sesi, `NewSessionMenu`, `WorkspaceHeader`.
- Mengubah `ThreadPane`, `thread-client.ts`, `participants/`, `close/`, `recovery/`, `activity/`, atau backend Go.
- Item rail Lark dan Git (desain terpisah). Slot order dicadangkan saja.
- Unit test, dependensi baru.

## Kontrak
Disediakan:
- Item rail `threads` terdaftar oleh `frontend/src/modules/thread/index.ts`: `{ id: 'threads', title: 'Thread', icon: ThreadRailIcon, component: ThreadSection, order: 30 }` (tanpa `visible`, selalu tampil).
- Komponen `frontend/src/modules/thread/rail-icon.ts` (diekspor komponen/ikon) sebagai SVG inline gelembung chat. Bila `.ts` tidak dapat memuat komponen Svelte, `rail-icon.ts` mengekspor komponen dari file `.svelte` dari folder yang sama (tambahkan `ThreadRailIcon.svelte` di `frontend/src/modules/thread/`; ini bagian dari path rail-icon). Ikon menerima ukuran dan warna dari CSS induk (`currentColor`), mengikuti pola ikon rail Agen dari tiket 1.
- Slot order rail yang dicadangkan (hanya dokumentasi di komentar, tidak ada kode): lark=10, git-github=20, git-gitlab=21.

Dipakai (dari tiket 1; periksa nama persis di kode):
- `registerRailItem(spec)` dan tipe `RailItemSpec { id; title; icon: Component; component: Component<any>; order?; visible? }` dari `frontend/src/modules/shell/registry.ts`. Panel dirender dengan props `{}`, hanya panel aktif yang di-mount.
- `activeRail`/`setActiveRail` pada store `leftSidebar` (`frontend/src/lib/stores/shell.ts`); id tak dikenal atau tak visible jatuh ke `agents`.
- Dari kode thread yang ada: `threads`, `threadError`, `openThreadPane`, `subscribeThreadEvents`, `ReadyBadge`, `NewThreadDialog`, `statusStore`, `activeWorkspaceId`.

Aturan tetap (kontrak bersama):
- `subscribeThreadEvents()` dipanggil saat modul `thread/index.ts` diimpor, tidak boleh dipindah ke komponen panel; `ThreadSection` tidak boleh mendaftarkan subscriber event sendiri.
- Teks UI bahasa Indonesia: "Thread", "Thread baru", "Belum ada thread".

## Fitur

### threads-panel-state: State panel Threads ke store
Deskripsi:
Panel di-unmount saat item rail lain aktif, jadi state lokal yang berarti hilang bila tetap berupa `$state` di komponen. Saat ini `ThreadSection.svelte` hanya punya `dialogOpen` (lokal, sementara) dan belum punya filter, bagian terlipat, maupun scroll. Lakukan hal berikut:
- Inventaris state lokal `ThreadSection.svelte`. State yang transien (dialog "Thread baru" yang terbuka) boleh tetap lokal; menutup dialog saat berpindah item rail dapat diterima.
- Tambahkan di `thread-store.ts` store kecil `threadPanelState` (writable) berisi state panel yang perlu bertahan: minimal posisi scroll daftar (`scrollTop: number`). Bila implementasi menambah filter atau bagian terlipat, simpan di store yang sama. Jangan menambah fitur filter baru yang tidak ada sekarang.
- `ThreadSection.svelte` memulihkan `scrollTop` saat mount (setelah daftar dirender) dan menyimpannya ke store saat scroll/unmount. Elemen yang scroll adalah daftar `ul` atau pembungkusnya; beri pembungkus `overflow-y: auto; min-height: 0` yang mengisi tinggi panel agar daftar panjang bisa di-scroll di dalam panel penuh.
- Ganti signature props menjadi `{}`: hapus `let {}: { onerror } = $props()`, galat tetap ditampilkan lewat `threadError` yang sudah ada. Hapus pembungkus `border-bottom` section yang hanya masuk akal di dalam Agents; panel mengisi tinggi penuh dengan padding yang wajar.
- Pastikan `ThreadSection.svelte` tidak memanggil `subscribeThreadEvents` atau `listen` event apa pun.
- Tidak mengubah perilaku daftar, tombol, badge, dan pembukaan pane.

Acceptance criteria:
- [ ] `ThreadSection.svelte` tidak menerima props apa pun (`{}`) dan tidak memanggil `onerror`.
- [ ] `thread-store.ts` mengekspor store state panel; posisi scroll daftar bertahan setelah berpindah ke item rail lain dan kembali (dalam sesi yang sama).
- [ ] `ThreadSection.svelte` tidak mendaftarkan subscriber event.
- [ ] Daftar thread, tombol "Thread baru", `ReadyBadge`, tag "ditutup", dan klik baris membuka pane berperilaku sama seperti sebelumnya.
- [ ] Build frontend lulus.

ownedPaths:
- `frontend/src/modules/thread/ThreadSection.svelte`
- `frontend/src/modules/thread/thread-store.ts`

Catatan: bisa dikerjakan paralel dengan `agents-sidebar-cleanup`. `threads-rail-item` (wiring) berjalan setelah keduanya.

### agents-sidebar-cleanup: Bersihkan section Threads dari Agents
Deskripsi:
`AgentSidebar.svelte` merender semua `sidebarSections()` di antara `WorkspaceHeader` dan daftar sesi; satu-satunya pendaftar saat ini adalah modul thread. Setelah wiring, tidak ada section terdaftar. Keputusan untuk tiket ini (minim risiko): mekanisme `sidebarSections`/`registerSidebarSection` di `registry.ts` dan ekspornya di `index.ts` DIBIARKAN (spec: tetap ada untuk modul lain; membuang berisiko memutus impor modul lain), tetapi `AgentSidebar.svelte` tetap merendernya generik sehingga tidak ada section Threads yang tampil selama tidak ada yang mendaftar. Tambahkan komentar singkat di `registry.ts` di atas `SidebarSectionSpec`: bahwa Threads kini item rail, mekanisme ini tersisa untuk section opsional di dalam panel Agen, dan saat ini tidak ada pendaftar. Sebelum menyimpulkan, `grep` seluruh `frontend/src` untuk `registerSidebarSection`/`sidebarSections` dan catat pemakai lain di komentar; bila ternyata ada pemakai lain, jangan ubah perilakunya.
- Pastikan `AgentSidebar.svelte` tidak mengandung referensi ke Threads/`ThreadSection` (impor atau markup) dan perilaku daftar sesi, grouping, filter sesi tidak berubah.
- Boleh menyesuaikan jarak/tata letak minimal agar panel Agen rapi tanpa section di atas daftar sesi.
- Jangan menghapus `onerror` di `AgentSidebar`; ia masih dipakai `WorkspaceHeader`, `SessionGroups`, `NewSessionMenu`.

Acceptance criteria:
- [ ] `AgentSidebar.svelte` tidak mereferensikan Threads; setelah wiring panel Agen tidak menampilkan section Threads.
- [ ] `registry.ts` memuat komentar keputusan tentang `sidebarSections`; ekspor di `index.ts` tidak berubah (atau, bila dibersihkan, tidak ada impor yang patah).
- [ ] Daftar sesi, grouping, filter sesi, menu sesi baru tidak berubah perilakunya.
- [ ] Build frontend lulus.

ownedPaths:
- `frontend/src/modules/sidebar-left/AgentSidebar.svelte`
- `frontend/src/modules/sidebar-left/registry.ts`
- `frontend/src/modules/sidebar-left/index.ts`

Catatan: bisa dikerjakan paralel dengan `threads-panel-state`. File `sidebar-left/rail.ts` milik tiket 1, jangan disentuh.

### threads-rail-item: Daftarkan Threads sebagai item rail
Deskripsi:
Fitur WIRING; dikerjakan setelah dua fitur lain selesai.
- Buat `frontend/src/modules/thread/rail-icon.ts` (dan `ThreadRailIcon.svelte` di folder yang sama bila diperlukan, lihat Kontrak): SVG inline gelembung chat 24x24 (`viewBox`), `stroke="currentColor"`, `fill="none"`, `aria-hidden="true"`, tanpa dependensi baru. Meniru pola ikon rail Agen (`sidebar-left/rail.ts`, tiket 1) untuk ukuran dan gaya.
- Di `frontend/src/modules/thread/index.ts`: hapus impor dan pemanggilan `registerSidebarSection` untuk `thread-sidebar`; impor `registerRailItem` dari `../shell/registry` (atau barrel shell bila tiket 1 mengekspornya di sana) dan panggil `registerRailItem({ id: 'threads', title: 'Thread', icon: ThreadRailIcon, component: ThreadSection, order: 30 })`.
- Biarkan `registerPaneKind`, `registerTabType`, `subscribeThreadEvents()`, `installParticipants()`, `installClose()`, `installRecovery()` di tempatnya, dipanggil saat modul diimpor.
- Tambahkan komentar singkat bahwa order rail dicadangkan: lark=10, git-github=20, git-gitlab=21.
- Verifikasi manual (lihat Wiring).

Acceptance criteria:
- [ ] `index.ts` tidak lagi mendaftarkan `thread-sidebar`; mendaftarkan item rail `threads` dengan order 30 dan judul "Thread".
- [ ] Rail menampilkan ikon gelembung chat dengan `aria-label` "Thread"; item bisa difokus dan dipilih dengan keyboard (dari komponen rail tiket 1).
- [ ] `subscribeThreadEvents()` tetap dipanggil di level modul; tidak ada pemanggilan di komponen.
- [ ] Klik Thread menampilkan daftar thread; berpindah ke Agen dan kembali tidak memutus sesi terminal atau thread.
- [ ] Build frontend lulus.

ownedPaths:
- `frontend/src/modules/thread/index.ts`
- `frontend/src/modules/thread/rail-icon.ts`

Catatan: bergantung pada `threads-panel-state` dan `agents-sidebar-cleanup` (urutan terakhir). `ThreadRailIcon.svelte` bila dibuat dianggap bagian dari path `rail-icon.ts`.

## Path yang dimiliki
Hanya ini yang boleh disentuh:
- `frontend/src/modules/thread/index.ts`
- `frontend/src/modules/thread/ThreadSection.svelte`
- `frontend/src/modules/thread/thread-store.ts`
- `frontend/src/modules/thread/rail-icon.ts` (beserta `ThreadRailIcon.svelte` bila diperlukan)
- `frontend/src/modules/sidebar-left/AgentSidebar.svelte`
- `frontend/src/modules/sidebar-left/registry.ts`
- `frontend/src/modules/sidebar-left/index.ts` (hanya bila `sidebarSections` dibersihkan; default tidak diubah)

## Wiring
Langkah penutup tiket ini, memakai titik ekstensi `registerRailItem` dari tiket 1 (tiket 1 sudah merge sebagai prasyarat):
1. Pastikan `threads-panel-state` dan `agents-sidebar-cleanup` sudah selesai.
2. Selesaikan `threads-rail-item`: item `threads` terdaftar di `modules/thread/index.ts`. Modul thread sudah diimpor oleh bootstrap aplikasi saat ini; pastikan impor itu tetap ada (jangan ubah `App.svelte`/shell, milik tiket 1).
3. Jalankan aplikasi dan periksa: rail memuat Agen dan Thread; Agen tanpa section Threads; klik Thread menampilkan daftar; buat thread baru, buka pane, kirim pesan; pindah ke Agen sementara thread menerima pesan lalu kembali ke Thread dan daftar mutakhir; sesi terminal tidak terputus; pilih Thread, restart aplikasi, item aktif tetap Thread; sembunyikan panel dengan toggle kiri, rail tetap tampil.
4. Tiket ini berjalan sendiri di atas tiket 1; tidak bergantung pada tiket Lark atau Git.

## Guard
- `cd frontend && npm run build` (atau perintah build frontend yang dipakai proyek) lulus tanpa galat tipe/svelte-check.
- `go build ./...` lulus (tiket ini tidak mengubah Go; cukup memastikan tidak patah).
- Tanpa unit test.
