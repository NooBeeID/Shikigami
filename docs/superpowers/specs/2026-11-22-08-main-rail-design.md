# Rail menu utama (sidebar kiri dua lapis)

Fondasi untuk Lark dan Git (desain terpisah: `2026-11-22-09-lark-connector-design.md`, dan
Git yang menyusul). Dibangun lebih dulu karena keduanya menumpang di atasnya.

## Maksud dan alasan
Sidebar kiri sekarang hanya berisi sesi agen (`AgentSidebar`), dengan Threads menempel
sebagai section di dalamnya. Fitur baru (Lark, Git) tidak punya rumah. User ingin sidebar
kiri berlapis dua: **rail ikon paling kiri sebagai menu utama**, dan **panel** di
sebelahnya yang isinya mengikuti menu terpilih. Isi tiap menu tampil di panel; hasil yang
dibuka (kanban, dokumen, MR, dst.) tampil sebagai pane di area tengah.

## Tujuan
- Rail ikon vertikal di tepi kiri dengan item: **Agents**, **Lark**, **Git**, **Threads**.
- Memilih item menampilkan panelnya di kolom kedua; item aktif ditandai.
- **Agents** = isi `AgentSidebar` yang ada sekarang (tanpa Threads).
- **Threads** = `ThreadSection` dipindah dari `AgentSidebar` menjadi panel sendiri
  (ikon bergaya chat). Perilaku Threads tidak berubah, hanya tempatnya.
- **Lark** dan **Git** hadir sebagai panel yang didaftarkan modul masing-masing (desain
  terpisah). Sebelum modul itu ada, rail tidak menampilkan itemnya.
- **Item Git tergantung project**: rail menampilkan GitHub bila ada project di workspace
  aktif dengan remote GitHub, GitLab bila remote GitLab, dan keduanya bila ada dua
  jenis. Tanpa remote yang dikenal, item Git tidak tampil.
- Item rail dan panelnya didaftarkan lewat registry (pola `registerRightTab`), supaya
  modul lain menambah item tanpa mengubah shell.
- Rail dan item aktif dipersist bersama state UI sidebar.

## Bukan tujuan
- Tidak mengubah panel kanan (tetap Files) maupun area layout/pane.
- Tidak mengubah isi atau perilaku Agents dan Threads selain tempat tampilnya.
- Tidak ada drag-reorder atau pengaturan item rail oleh user.
- Tidak membangun isi Lark atau Git (desain lain).

## Anti-tujuan
- Memutus sesi terminal atau thread yang berjalan saat berpindah item rail.
- Kehilangan state panel (scroll, filter) hanya karena berpindah item rail dan kembali
  (usul: panel di-mount ulang boleh, tetapi state yang berarti disimpan di store).
- Memperlambat startup: panel yang tidak aktif tidak melakukan kerja berat.

## Pendekatan yang dipertimbangkan
- (a) Rail sebagai kolom baru di `ShellFrame` dengan registry item+panel; panel memakai
  lebar `leftSidebar` yang ada. (b) Tab ikon di atas panel kiri yang sama. (c) Rail
  menggantikan sidebar kiri (panel tampil sebagai overlay). **Dipilih a** (sesuai
  rancangan user): (b) bukan "menu utama paling kiri", (c) mengubah perilaku resize dan
  toggle yang sudah ada.

## Desain
- `ShellFrame`: grid menjadi `rail | panel | center | right`. Rail lebar tetap (~48px),
  selalu terlihat; toggle sidebar kiri (`toggleLeft`, shortcut yang ada) hanya
  menyembunyikan **panel**, rail tetap.
- Registry baru (di `modules/shell/registry.ts`):
  `registerRailItem({ id, title, icon, component, order, visible?: Readable<boolean> })`.
  `visible` dipakai Git agar item muncul sesuai project; tanpanya item selalu tampil.
  `railItems()` mengembalikan yang `visible`, terurut.
- State: `leftSidebar` ditambah `activeRail: string` (default `agents`), dipersist
  lewat `UIState.LeftSidebar` (`internal/workspace/types.go`) dengan field baru
  `activeRail,omitempty`; nilai tak dikenal jatuh ke `agents`. Bila item aktif menjadi
  tidak visible (mis. ganti workspace), jatuh ke `agents`.
- Item bawaan: `agents` (order 0, `AgentSidebar` tanpa section Threads), `threads`
  (order 30, `ThreadSection`), serta `lark` (order 10) dan `git-*` (order 20, 21) yang
  didaftarkan modul masing-masing.
- Pemindahan Threads: `registerSidebarSection({ id: 'thread-sidebar', ... })` di
  `modules/thread/index.ts` diganti `registerRailItem`. Mekanisme `sidebarSections`
  tetap ada untuk modul lain; builder memutuskan apakah dibiarkan atau dibersihkan.
- Ikon: SVG inline sederhana (tanpa dependensi baru); Threads bergaya gelembung chat.
- Keyboard: item rail bisa difokus dan dipilih dengan keyboard; `aria-label` per item.
- Teks UI bahasa Indonesia ("Agen", "Thread", "Lark", "Git").

## Panggilan saya (silakan koreksi)
- Rail selalu terlihat walau panel disembunyikan.
- Urutan: Agents, Lark, Git, Threads (sesuai urutan user).
- Deteksi remote Git (GitHub/GitLab) dibangun di desain Git; rail hanya menerima
  `visible` dari sana. Self-hosted GitLab ditangani di desain Git.
- Lebar rail 48px, tetap, tidak bisa di-resize.

## Hasil pemeriksaan builder
- **Panel di-unmount saat tidak aktif** (hanya panel aktif yang dirender). Aman untuk
  Threads: `ThreadSection` tidak mendaftarkan subscriber sendiri; event thread
  disubscribe saat modul diimpor (`subscribeThreadEvents()` di `modules/thread/index.ts`),
  bukan saat komponen mount. Test memastikan event thread tetap diterima saat item rail
  lain aktif. State yang berarti (filter, bagian terlipat, scroll) disimpan di store.
- Panel menerima props `{}` (panel mengambil state dari store, seperti `FilesTab`).
- **Rail dibangun dengan `visible` didukung dan diuji memakai item palsu**; tidak ada
  item Git placeholder. Item Git didaftarkan oleh desain Git nanti.
- **Urutan terhadap perubahan `layout/*`**: working tree saat ini berisi perubahan belum
  di-commit di `layout/*`, `lib/stores/layout.ts`, `provider/*`, dan `thread/*` (bukan dari
  pekerjaan ini). Rail menyentuh `ShellFrame`, `lib/stores/shell.ts`, dan state UI, bukan
  `layout/*`; implementasi dimulai setelah perubahan itu di-commit atau dipindah, agar
  worktree tiket mulai dari dasar yang bersih.

## Diserahkan ke builder
- Rincian grid/CSS `ShellFrame` dan interaksi dengan `Resizer` kiri.
- Memindahkan Threads tanpa memutus subscriber event dan peserta yang terdaftar.
- Migrasi state UI lama (tanpa `activeRail`) dan test-nya.
- Memutuskan nasib `sidebarSections` dan section lain yang mungkin menempel di Agents.
