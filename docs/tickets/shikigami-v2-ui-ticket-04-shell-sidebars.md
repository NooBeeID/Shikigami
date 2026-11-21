---
id: shikigami-v2-ui-ticket-04-shell-sidebars
title: Bilah atas dan dua sidebar
spec: docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md
status: ready
wave: 4
depends_on: [shikigami-v2-ui-ticket-03-pane-layout]
---
# Bilah atas dan dua sidebar

## Tujuan
Bentuk ulang shell aplikasi menjadi bilah atas (tab workspace, penghitung sesi, lonceng notifikasi, pengaturan) dengan dua sidebar: kiri untuk Agent (sesi per project) dan kanan untuk File (tree project), mengelilingi area layout multi-pane dari tiket 03. Ini memenuhi spec bagian 4 dan 4b: pengguna mengawasi banyak agent dalam satu layar, membuka file sebagai pane, dan berpindah cepat lewat palet perintah Cmd+K. Epic ini juga menyediakan dua titik ekstensi baru (`registerRightTab`, `registerTopbarItem`) supaya tab "Activity" (tiket 05) dan item lain bisa ditambahkan tanpa mengubah kode shell.

## Hasil yang bisa dijalankan
Di atas hasil tiket 03 (layout pane sudah jalan), setelah tiket ini:
- Jendela memiliki bilah atas: tab workspace (klik pindah, `+` buat workspace, klik kanan ganti nama/hapus), penghitung "N sesi", lonceng notifikasi dengan badge belum dibaca, tombol pengaturan.
- Sidebar kiri "AGENT": pemilih workspace (nama, jumlah project dan sesi), sesi dikelompokkan per project dengan titik status, tombol "New Session" (pilih provider claude/codex/shell dan project). Klik sesi memfokuskan pane-nya; sesi yang belum tampil dimasukkan ke layout.
- Sidebar kanan "FILE": pemilih project, tree project, tombol "Open Folder…". Klik file membuka pane file di layout.
- Cmd+B menyembunyikan/menampilkan sidebar kiri, Cmd+Opt+B sidebar kanan (juga lewat event menu native); lebar kedua sidebar bisa diseret.
- Ringkasan di judul area tengah: nama workspace, status "Running", jumlah sesi, project, agent terhubung.
- Cmd+K membuka palet perintah untuk lompat ke project/sesi/file dan menjalankan aksi.
- `make build` lulus dan aplikasi berjalan dengan UI dua sidebar penuh; `App.svelte` tidak lagi memakai `SidebarArea`/`StatusBar` v1.

## Batasan
In scope:
- Rangka shell, state sidebar (visible, width, activeTab), registri tab kanan dan item topbar.
- Topbar, sidebar kiri, sidebar kanan (tab Files), palet perintah, lonceng notifikasi.
- Wiring `App.svelte` ke shell baru dan pensiun komponen shell v1.

Out of scope:
- Penyimpanan state UI ke daemon (`workspace.ui.get/set`) dan pemulihan saat dibuka: tiket 06. Di tiket ini state sidebar hidup di memori saja, tetapi bentuknya harus sama dengan kontrak 5 agar tiket 06 bisa menyambung.
- Tab grup sesi (tiket 07), tab "Activity" (tiket 05), Orchestration/Tasks, rail ikon, meter sumber daya.
- Pembuatan menu native dan event-nya (tiket 02); epic ini hanya berlangganan lewat event bus `modules/appmenu`.
- Mengubah isi layout/pane/terminal (tiket 03). Epic ini hanya memanggil aksi store layout.
- Memodifikasi modul `workspace`, `status`, `files`, `openfolder`, `appmenu`: hanya dipakai ulang lewat impor publiknya.

Dependensi dan fallback: tiket ini bergantung pada tiket 03 (`lib/stores/layout.ts`). Event menu (`menu:toggle-sidebar-*`) dan `modules/openfolder` berasal dari tiket 02 (wave 2, sudah selesai sebelum wave 4). Jika modul openfolder belum menyediakan fungsi yang dibutuhkan, tombol Open Folder memanggil bridge `OpenDirectoryDialog()` lalu `addProject(path)` dari `lib/stores/workspaces.ts`.

## Kontrak

### Disediakan epic ini

State shell, `frontend/src/lib/stores/shell.ts`:
```ts
export interface SidebarState { visible: boolean; width: number }
export interface RightSidebarState extends SidebarState { activeTab: string }
export const leftSidebar: Writable<SidebarState>;          // default {visible:true, width:260}
export const rightSidebar: Writable<RightSidebarState>;    // default {visible:true, width:280, activeTab:'files'}
export const LEFT_MIN = 200, LEFT_MAX = 420, RIGHT_MIN = 220, RIGHT_MAX = 520;
export function toggleLeft(): void;
export function toggleRight(): void;
export function setLeftWidth(px: number): void;   // clamp ke min/max
export function setRightWidth(px: number): void;
export function setRightTab(id: string): void;
```
Bentuk sengaja sama dengan UI state kontrak 5 (`{leftSidebar:{visible,width}, rightSidebar:{visible,width,activeTab}}`) supaya tiket 06 bisa membaca/menulisnya apa adanya (tiket 06 yang menambah persist; tiket ini tidak memanggil `workspace.ui.*`).

Registri titik ekstensi, `frontend/src/modules/shell/registry.ts`:
```ts
import type { Component } from 'svelte';
export interface RightTabSpec { id: string; title: string; component: Component<any>; order?: number }
export interface TopbarItemSpec { id: string; component: Component<any>; align: 'left' | 'center' | 'right'; order?: number }
export function registerRightTab(spec: RightTabSpec): void;       // idempoten per id
export function registerTopbarItem(spec: TopbarItemSpec): void;   // idempoten per id
export function rightTabs(): Readable<RightTabSpec[]>;            // urut order lalu id
export function topbarItems(align: 'left' | 'center' | 'right'): Readable<TopbarItemSpec[]>;
```
Pola implementasi mengikuti `lib/registry.ts` (writable berisi map, derived terurut). `modules/shell/index.ts` mengekspor keduanya agar modul lain cukup `import { registerRightTab } from '../shell'`.

Registri aksi palet, `frontend/src/modules/palette/registry.ts`:
```ts
export interface PaletteAction { id: string; title: string; hint?: string; keywords?: string[]; run: () => void | Promise<void> }
export type PaletteSourceKind = 'project' | 'session' | 'file' | 'action';
export interface PaletteItem { id: string; kind: PaletteSourceKind; title: string; subtitle?: string; run: () => void | Promise<void> }
export interface PaletteSource { id: string; kind: PaletteSourceKind; list: (query: string) => PaletteItem[] | Promise<PaletteItem[]> }
export function registerPaletteAction(a: PaletteAction): void;
export function registerPaletteSource(s: PaletteSource): void;
```

Komponen yang dapat dipakai tiket lain: `modules/notifications` mengekspor `Bell.svelte`; `modules/topbar` mengekspor `SessionCounter.svelte`.

### Dipakai epic ini (sudah ada, jangan diubah)
- `lib/registry.ts`: titik ekstensi v1 (`registerPanel`); modul lama tetap boleh terdaftar di sana selama transisi.
- `lib/stores/workspaces.ts`: `workspaces`, `activeWorkspace`, `activeProject`, `activeProjectId`, `selectWorkspace`, `selectProject`, `createWorkspace`, `renameWorkspace`, `deleteWorkspace`, `addProject`.
- `lib/stores/sessions.ts`: `sessions`, `createSession(providerId)`, `killSession`. `lib/stores/providers.ts`: daftar provider (claude, codex, shell).
- `modules/status`: `statusStore`, `statusOf(id)`, `StatusBadge`, tipe `Status = 'running'|'waiting'|'done'|'idle'`. Lonceng dan indikator sesi berlangganan store ini (sudah event-driven dari event status daemon); tanpa polling tambahan.
- `modules/workspace`: komponen `WorkspacePicker`, `NewSession`, `SessionList`, `ProjectList` dipakai ulang bila sesuai; bila tidak cocok secara visual, bungkus, jangan salin logika.
- `modules/files/tree` (`FilesPanel`, `treeModel`): dipakai ulang untuk tree di sidebar kanan.
- Dari tiket 03: `lib/stores/layout.ts` (aksi `openPane`, `focusPane`/set fokus, `serialize`; tipe Leaf `{type:'pane', id, kind:'terminal'|'file', sessionId?, projectId?, provider?, filePath?, title}`) dan `modules/layout` (komponen area layout). Aksi yang dipakai: `openPane({kind:'terminal', sessionId, projectId, provider, title})`, `openPane({kind:'file', projectId, filePath, title})`, set fokus ke pane yang sudah memuat sesi/file itu, `tidyGrid`, serta pencarian pane berdasarkan `sessionId`/`filePath` (baca dari dokumen layout aktif). Nama persis ekspor mengikuti implementasi tiket 03; baca `lib/stores/layout.ts` sebelum menulis plan.
- Dari tiket 02: event bus `modules/appmenu` (`menu:toggle-sidebar-left`, `menu:toggle-sidebar-right`, `menu:open-folder`, `menu:zoom`), `modules/openfolder`, `modules/zoom`.
- Kontrak flag izin agent: pembuatan sesi tetap lewat `createSession`; tidak boleh menyusun argumen provider sendiri (flag `--dangerously-skip-permissions` / `--dangerously-bypass-approvals-and-sandbox` dijaga di `internal/provider`).

### Pintasan yang didaftarkan
Cmd+B, Cmd+Opt+B, Cmd+K. Pastikan tidak bentrok dengan terminal: handler keydown global memakai `metaKey` dan tidak aktif bila fokus berada di input teks (kecuali Cmd+K yang tetap membuka palet). Event menu native bila ada diutamakan; handler keyboard jangan menggandakan toggle ketika keduanya menembak (dedupe berdasarkan event yang sama, mis. `preventDefault` + flag waktu singkat).

## Fitur

Semua fitur punya `ownedPaths` terpisah dan bisa dikerjakan paralel setelah `shell-frame` (foundation) menetapkan `lib/stores/shell.ts` dan `modules/shell/registry.ts`. `shell-wiring` dikerjakan terakhir.

### shell-frame: Foundation: rangka shell dan registri panel kanan
Deskripsi:
- Buat `frontend/src/lib/stores/shell.ts` sesuai kontrak (state, clamp lebar, toggle).
- Buat `modules/shell/registry.ts` (`registerRightTab`, `registerTopbarItem`, derived terurut), `modules/shell/index.ts` (ekspor registri dan `ShellFrame`).
- Buat `modules/shell/ShellFrame.svelte`: grid CSS dengan baris topbar (tinggi tetap, mis. `var(--topbar-h)`) dan baris isi; kolom isi = sidebar kiri | area tengah | sidebar kanan. Slot via props snippet Svelte 5 (`topbar`, `left`, `center`, `right`) agar `ShellFrame` tidak mengimpor modul lain (foundation tidak bergantung pada fitur). Kolom sidebar disembunyikan lewat lebar 0 (komponen tetap ter-mount agar state internal, mis. tree yang terbuka, tidak hilang).
- Buat `modules/shell/Resizer.svelte`: pegangan seret 4px di sisi dalam sidebar, pointer events (`setPointerCapture`), memanggil `setLeftWidth`/`setRightWidth`, kursor `col-resize`, tidak memicu render ulang berat (update lewat CSS variable `--left-w`/`--right-w` pada root shell, `requestAnimationFrame`). Klik ganda mengembalikan lebar bawaan.
- Buat `modules/shell/RightPanel.svelte`: header tab dari `rightTabs()` (jika hanya satu tab, tampilkan sebagai judul "FILE" tanpa bar tab), isi = komponen tab aktif (`rightSidebar.activeTab`); bila id aktif tidak ada di registri, jatuh ke tab pertama.
- Buat `modules/shell/shortcuts.ts`: berlangganan event menu (`modules/appmenu`) dan keydown Cmd+B / Cmd+Opt+B ke `toggleLeft`/`toggleRight`, dengan dedupe.
- Tambahkan token CSS (`--topbar-h`, `--left-w`, `--right-w`) di `modules/shell/shell.css` yang diimpor ShellFrame (jangan mengubah `styles/theme.css`; bila perlu variabel global baru, definisikan di file shell sendiri).
Acceptance criteria:
- [ ] `lib/stores/shell.ts` mengekspor semua simbol kontrak; `setLeftWidth(9999)` menghasilkan `LEFT_MAX`, `setLeftWidth(0)` menghasilkan `LEFT_MIN`.
- [ ] `registerRightTab` dan `registerTopbarItem` idempoten per id dan terurut `order` lalu id.
- [ ] `ShellFrame` merender empat slot; menyembunyikan sidebar tidak meng-unmount isinya.
- [ ] Menyeret `Resizer` mengubah lebar secara mulus dan menghormati min/max.
- [ ] Cmd+B / Cmd+Opt+B dan event `menu:toggle-sidebar-left/right` memanggil toggle tepat sekali per aksi.
- [ ] Layout tetap benar (tanpa overflow) pada jendela sempit (min 900px) dengan kedua sidebar terbuka.
- [ ] `make build` lulus.
ownedPaths: `frontend/src/modules/shell/` (kecuali `App`-wiring), `frontend/src/lib/stores/shell.ts`.
Paralel: fitur lain di epic ini bergantung pada kontrak registri dari fitur ini (cukup menyalin antarmuka di atas); dapat dimulai bersamaan bila antarmuka dipatuhi, tetapi integrasi akhir menunggu foundation.

### topbar: Bilah atas: tab workspace, penghitung sesi, pengaturan
Deskripsi:
- `modules/topbar/Topbar.svelte`: menyusun tiga zona (kiri, tengah, kanan). Zona kiri berisi `WorkspaceTabs`; zona kanan berisi item terdaftar lewat `topbarItems('right')` (lonceng masuk di sini lewat registri, bukan impor langsung), `SessionCounter`, dan `SettingsButton`. Beri ruang di kiri untuk tombol lampu lalu-lintas macOS (inset sekitar 78px) dan jadikan area kosong draggable bila jendela memakai title bar tersembunyi (`--wails-draggable: drag`; periksa konfigurasi jendela di `app/main.go`, jangan mengubahnya; bila title bar standar, abaikan inset).
- `WorkspaceTabs.svelte`: satu tab per workspace dari `workspaces`, aktif sesuai `activeWorkspaceId`; klik memanggil `selectWorkspace`; tombol `+` membuat workspace (nama default "Workspace N" lewat `createWorkspace`, lalu masuk mode ganti nama inline); menu konteks (klik kanan) dengan "Ganti nama" (inline edit, Enter simpan, Esc batal, panggil `renameWorkspace`) dan "Hapus" (konfirmasi, `deleteWorkspace`; tidak boleh menghapus bila tersisa satu workspace). Tab yang meluber bisa di-scroll horizontal.
- `SessionCounter.svelte`: "N sesi" (jumlah sesi workspace aktif yang berstatus running), tooltip rincian per status dari `statusStore`.
- `SettingsButton.svelte`: ikon gear; di tiket ini membuka menu kecil berisi aksi yang ada: Zoom in/out/reset (memanggil event bus zoom dari `modules/appmenu`/`modules/zoom`), Toggle sidebar kiri/kanan, Susun grid. Bukan halaman pengaturan penuh.
- `CenterSummary.svelte`: ringkasan judul area tengah (dipakai sebagai slot `center` topbar atau baris judul tepat di atas area layout; pilih baris di topbar zona tengah): nama workspace, titik hijau + "Running" bila ada sesi running (selain itu "Idle"), jumlah sesi, nama project (maks 3, sisanya "+N"), dan agent terhubung (lencana provider unik dari sesi workspace aktif, mis. CL/CX/SH, sumber `modules/status`/`sessions`).
Acceptance criteria:
- [ ] Tab workspace mencerminkan `workspaces`; klik berpindah workspace dan tab aktif diberi gaya aksen.
- [ ] `+` membuat workspace dan langsung masuk mode ganti nama; ganti nama dan hapus lewat menu konteks bekerja dan tab diperbarui.
- [ ] Hapus workspace terakhir ditolak dengan pesan singkat.
- [ ] `SessionCounter` berubah saat sesi dibuat/diakhiri/berganti status tanpa polling.
- [ ] Tombol pengaturan menampilkan menu aksi dan tiap aksi bekerja (atau dinonaktifkan dengan jelas bila modul sumbernya tidak ada).
- [ ] Ringkasan tengah menampilkan nama workspace, status Running/Idle, jumlah sesi, project, dan agent terhubung.
- [ ] Item yang didaftarkan lewat `registerTopbarItem` muncul di zona yang diminta.
- [ ] `make build` lulus.
ownedPaths: `frontend/src/modules/topbar/`.
Paralel: ya, dengan fitur lain kecuali `shell-wiring`. Memakai `registerTopbarItem`/`topbarItems` dari `modules/shell/registry.ts` (kontrak, ditulis `shell-frame`).

### sidebar-left-agent: Sidebar kiri Agent
Deskripsi:
- `modules/sidebar-left/AgentSidebar.svelte` berisi, dari atas: `WorkspaceHeader` (nama workspace aktif, "N project · M sesi", klik membuka dropdown ganti workspace; pakai ulang `modules/workspace/WorkspacePicker` bila bisa), daftar sesi, dan tombol "New Session" di bawah.
- `SessionGroups.svelte`: sesi workspace aktif dikelompokkan per project (header project dapat dilipat, state lipat disimpan di komponen). Tiap baris sesi: `StatusBadge` dari `modules/status`, judul sesi (judul pane bila ada di layout, selain itu nama default provider), lencana provider, penanda "di layout" bila sudah tampil sebagai pane. Menu `…` per baris: "Akhiri sesi" (`killSession`, dengan konfirmasi), "Buka di layout".
- Klik sesi: bila sudah ada pane dengan `sessionId` itu di layout aktif, fokuskan; jika belum, `openPane({kind:'terminal', sessionId, projectId, provider, title})`. Sesi yang sudah `exited` tetap terlihat redup dan klik tetap membuka pane (tiket 06 yang menangani "Mulai ulang").
- `NewSessionMenu.svelte`: tombol "New Session" membuka popover dengan pilihan project (default project aktif) dan provider (claude, codex, shell dari `lib/stores/providers.ts`); memilih membuat sesi lewat `createSession` lalu langsung membuka pane. Bila workspace tidak punya project, tampilkan ajakan "Open Folder…" yang memanggil aksi yang sama dengan sidebar kanan.
- Memakai ulang logika/komponen `modules/workspace` dan `modules/status`; jangan menduplikasi pemanggilan IPC.
Acceptance criteria:
- [ ] Header menampilkan nama workspace, jumlah project dan sesi yang benar, dan mengganti workspace lewat dropdown.
- [ ] Sesi terkelompok per project dengan indikator status yang berubah real-time.
- [ ] Klik sesi yang sudah tampil memfokuskan pane tanpa membuat pane baru; klik sesi yang belum tampil menambahkannya ke layout.
- [ ] New Session membuat sesi dengan provider dan project terpilih dan membuka pane-nya; flag izin agent tidak diutak-atik di frontend.
- [ ] "Akhiri sesi" membunuh sesi (menutup pane saja tidak, sesuai spec bagian 5).
- [ ] Sesi `exited` tampil redup dan tidak merusak daftar.
- [ ] `make build` lulus.
ownedPaths: `frontend/src/modules/sidebar-left/`.
Paralel: ya, dengan fitur lain kecuali `shell-wiring`.

### sidebar-right-files: Sidebar kanan File dan Open Folder
Deskripsi:
- `modules/sidebar-right/FilesTab.svelte`: pemilih project di atas (dropdown dari `activeWorkspace.projects`, sinkron dengan `activeProjectId` lewat `selectProject`), lalu tree project memakai `modules/files/tree` (`FilesPanel`/`treeModel`; baca antarmukanya, bungkus tanpa mengubahnya), dan tombol "Open Folder…" di bawah (atau di header tab).
- Klik file di tree membuka pane file: `openPane({kind:'file', projectId, filePath, title: basename})`; bila pane file dengan `filePath` yang sama sudah ada di layout, fokuskan saja. Klik folder hanya memperluas/menutup. Klik ganda tidak diperlukan. Bila `FilesPanel` v1 membuka lewat `openTab`, sediakan callback/props atau wrapper agar pembukaan file diarahkan ke layout (jangan memodifikasi `modules/files`; jika tidak ada titik masuknya, tangkap event pilih file dari treeModel dan hentikan jalur tab).
- Open Folder: panggil modul `openfolder` (tiket 02) bila menyediakan fungsi; jika tidak, panggil bridge `OpenDirectoryDialog()` lalu `addProject(path)`; project baru langsung terpilih di pemilih. Bila dialog dibatalkan (string kosong), tidak terjadi apa-apa. Tampilkan juga di state kosong ("Belum ada project") dan sediakan input path teks sebagai cadangan sesuai spec bagian 6.
- Mendaftarkan diri di registri: `registerRightTab({ id: 'files', title: 'Files', component: FilesTab, order: 10 })` dilakukan di `modules/sidebar-right/index.ts` (lihat Wiring; side-effect import).
Acceptance criteria:
- [ ] Pemilih project mencerminkan project workspace aktif dan mengganti tree saat dipilih.
- [ ] Tree menampilkan isi project dan memuat anak folder secara malas (perilaku `modules/files/tree` dipertahankan).
- [ ] Klik file membuka pane file di layout (bukan tab); klik file yang sama memfokuskan pane yang ada.
- [ ] Open Folder membuka dialog macOS, menambahkan project, dan tree langsung menampilkannya.
- [ ] State kosong tanpa project menampilkan ajakan dan input path cadangan.
- [ ] `make build` lulus.
ownedPaths: `frontend/src/modules/sidebar-right/`.
Paralel: ya, dengan fitur lain kecuali `shell-wiring`; memakai `registerRightTab` dari kontrak `shell-frame`.

### command-palette: Palet perintah Cmd+K
Deskripsi:
- `modules/palette/Palette.svelte`: overlay modal di tengah atas (backdrop, input fokus otomatis, daftar hasil dikelompokkan per jenis: Aksi, Sesi, Project, File). Cmd+K membuka/menutup, Esc menutup, panah atas/bawah memilih, Enter menjalankan, klik menjalankan. Pencarian fuzzy sederhana (substring + urutan karakter) ditulis sendiri di `modules/palette/fuzzy.ts` tanpa dependency baru; hasil per kelompok dibatasi (mis. 8).
- `modules/palette/registry.ts` sesuai kontrak. Sumber bawaan di `sources.ts`: project (dari `activeWorkspace.projects`, aksi `selectProject`), sesi (dari `sessions`, aksi sama dengan klik sesi di sidebar kiri: fokus atau masukkan ke layout; gunakan fungsi util layout dari tiket 03 langsung, jangan impor dari `sidebar-left`), file (pencarian nama file di project aktif; gunakan layanan file yang sudah ada lewat `lib/ipc` bila ada metode daftar/cari, jika tidak, batasi ke file yang sudah dimuat di `treeModel`, dan catat batasan itu di komentar kode).
- Aksi bawaan di `actions.ts` (didaftarkan lewat `registerPaletteAction`): "Sesi baru: claude/codex/shell" (project aktif), "Open Folder…", "Zoom in/out/reset", "Toggle sidebar kiri/kanan", "Susun grid".
- `modules/palette/index.ts`: mendaftarkan aksi/sumber bawaan dan menyediakan `mountPalette()` atau komponen `Palette` yang dipasang oleh wiring; juga `openPalette()`/`closePalette()` serta store `paletteOpen`. Handler keydown Cmd+K dipasang oleh komponen saat mount dan dilepas saat destroy.
- Tidak boleh mencuri input terminal selain Cmd+K; saat palet tertutup fokus dikembalikan ke elemen sebelumnya (pane terminal terakhir).
Acceptance criteria:
- [ ] Cmd+K membuka palet dari mana pun, termasuk saat fokus di terminal; Esc menutup dan mengembalikan fokus.
- [ ] Mengetik menyaring hasil lintas aksi, sesi, project, dan file; navigasi keyboard penuh.
- [ ] Memilih project/sesi/file mengubah project aktif, memfokuskan pane sesi, atau membuka pane file.
- [ ] Setiap aksi bawaan berjalan; aksi baru dari modul lain muncul setelah `registerPaletteAction` tanpa mengubah kode palet.
- [ ] Tanpa dependency npm baru.
- [ ] `make build` lulus.
ownedPaths: `frontend/src/modules/palette/`.
Paralel: ya, dengan fitur lain kecuali `shell-wiring`.

### notification-bell: Lonceng notifikasi
Deskripsi:
- `modules/notifications/store.ts`: berlangganan `statusStore` dari `modules/status` (event-driven; tanpa polling, tanpa pendengar IPC kedua) dan menurunkan peristiwa dari perubahan status per sesi: transisi ke `waiting` ("menunggu input") dan ke `done` ("selesai"). Transisi ke `running`/`idle` tidak membuat notifikasi. Hindari duplikat (satu notifikasi per transisi, abaikan snapshot awal saat store pertama kali terisi). Entri: `{id, sessionId, kind:'waiting'|'done', title, at, read}`. Batas 50 entri terbaru, disimpan di memori saja. Store mengekspor `notifications`, `unreadCount`, `markRead(id)`, `markAllRead()`, `clear()`.
- `modules/notifications/Bell.svelte`: ikon lonceng dengan badge jumlah belum dibaca; klik membuka dropdown daftar (judul sesi, jenis, waktu relatif), "Tandai semua dibaca" dan "Bersihkan". Klik entri menandainya dibaca dan membuka/memfokuskan pane sesinya lewat layout store tiket 03 (jika sesi tidak ada lagi, entri redup tanpa aksi). Jangan berlangganan ulang ke event setelah dropdown ditutup.
- `modules/notifications/index.ts`: `registerTopbarItem({ id: 'notifications', component: Bell, align: 'right', order: 10 })`.
- Pemakaian ulang: judul sesi dari `sessions`/judul pane; status dari `modules/status`.
Acceptance criteria:
- [ ] Saat sesi berpindah ke `waiting` atau `done`, badge bertambah dan entri muncul di dropdown tanpa memuat ulang.
- [ ] Snapshot status awal saat start tidak menghasilkan banjir notifikasi.
- [ ] Klik entri memfokuskan pane sesi (membukanya di layout bila belum tampil) dan menandainya dibaca.
- [ ] Tandai semua dibaca / bersihkan bekerja; daftar dibatasi 50.
- [ ] Tidak ada timer polling; hanya langganan store/event.
- [ ] `make build` lulus.
ownedPaths: `frontend/src/modules/notifications/`.
Paralel: ya, dengan fitur lain kecuali `shell-wiring`; terdaftar di topbar lewat registri sehingga tidak bergantung pada fitur `topbar` untuk dikembangkan.

### shell-wiring: Wiring: aktifkan shell baru
Deskripsi: lihat bagian Wiring. Dikerjakan terakhir, setelah semua fitur lain selesai; tidak paralel dengan fitur lain.
Acceptance criteria:
- [ ] Lihat daftar periksa di bagian Wiring.
ownedPaths: `frontend/src/App.svelte`, `frontend/src/modules/shell/`.

## Path yang dimiliki
Hanya path berikut yang boleh disentuh epic ini:
- `frontend/src/modules/shell/`
- `frontend/src/modules/topbar/`
- `frontend/src/modules/sidebar-left/`
- `frontend/src/modules/sidebar-right/`
- `frontend/src/modules/palette/`
- `frontend/src/modules/notifications/`
- `frontend/src/lib/stores/shell.ts`
- Tambahan khusus wiring (satu-satunya perluasan di luar daftar epik): `frontend/src/App.svelte` (dimiliki fitur `shell-wiring`).

Perluasan file tiket lain yang diperlukan harus berupa pendaftaran lewat registri (bukan edit file mereka) dan disebut di Wiring. Bila `frontend/src/lib/stores/index.ts` perlu mengekspor `shell.ts`, jangan diubah: impor langsung dari `lib/stores/shell`.

## Wiring
Langkah penutup tiket ini (fitur `shell-wiring`), dikerjakan setelah semua fitur lain selesai. Tiket ini tidak boleh bergantung pada tiket lain selain 03 (dan 02 yang sudah selesai lebih dulu) untuk bisa dijalankan.
1. Buat `modules/shell/register.ts` (side-effect) yang mengimpor `../sidebar-right`, `../notifications`, dan `../palette` agar `registerRightTab({id:'files', ...})`, `registerTopbarItem({id:'notifications', ...})`, dan aksi palet terdaftar lewat registri. Item topbar lain (workspace tabs, counter, settings, ringkasan) dirender langsung oleh `Topbar` karena milik modul topbar yang sama.
2. Ubah `frontend/src/App.svelte`: ganti struktur `.shell` v1 (`TabBar`, `TabContent`, `SidebarArea`, `StatusBar`) dengan `<ShellFrame>` yang menerima snippet `topbar={Topbar}`, `left={AgentSidebar}`, `center={<LayoutArea>}` (komponen area layout dari `modules/layout`, tiket 03), `right={RightPanel}`, serta `<Palette />` di lapisan atas. Pertahankan penanganan `connection.state` (error/connecting) dan `initStores()` apa adanya.
3. Pensiunkan modul shell lama: `modules/shell/SidebarArea.svelte` dan `modules/shell/StatusBar.svelte` (v1) dihapus dari pemakaian dan dari folder (keduanya milik epic ini). Registrasi `registerPanel` area `sidebar`/`statusbar` dari modul lain (`workspace-sidebar`, `status-badges`, `status-summary`, `files` panel) menjadi tidak dirender; jangan mengubah modul tersebut. Jika sebuah modul lain masih mengimpor komponen shell lama, pertahankan file itu sebagai shim kosong dan catat di komentar.
4. `modules/tabs` v1 (`TabBar`, `TabContent`) tidak dipakai lagi oleh `App.svelte`, tetapi tidak dihapus (milik tiket 03, boleh menjadi shim).
5. Pastikan `modules/openfolder`, `modules/appmenu`, `modules/zoom`, `modules/layout` diimpor (side-effect) bila belum diimpor oleh entry lain, supaya event menu dan layout aktif; periksa `frontend/src/main.ts` sebelum menambah impor, dan bila perlu menambah impor di `App.svelte` (file milik epic ini) alih-alih `main.ts`.
Kondisi terlihat: aplikasi dibuka menampilkan topbar penuh, sidebar Agent di kiri, sidebar File di kanan dengan tab "Files", area layout di tengah; Cmd+B / Cmd+Opt+B / Cmd+K bekerja; klik sesi memfokuskan pane; klik file membuka pane file; `registerRightTab` siap dipakai tiket 05 tanpa mengubah shell.
Acceptance criteria wiring:
- [ ] `App.svelte` memakai `ShellFrame`, tanpa impor `SidebarArea`/`StatusBar` v1 atau `TabBar`/`TabContent`.
- [ ] Semua item (topbar, dua sidebar, palet, lonceng) terlihat dan berfungsi di aplikasi yang berjalan.
- [ ] Tab kanan "Files" dirender lewat registri (bukan impor langsung `RightPanel` -> `FilesTab`).
- [ ] Menambahkan `registerRightTab` dummy dari modul lain terlihat sebagai tab baru tanpa mengubah kode shell (verifikasi manual lalu hapus).
- [ ] Layar error/connecting tetap berfungsi.
- [ ] `make build` lulus dan aplikasi bisa dijalankan.

## Guard
Cukup build berhasil, tanpa unit test:
- `make build` (mencakup `go build ./...` dan build frontend, `npm run build`/`vite build` di `frontend/`), dan aplikasi bisa dijalankan (`make dev` atau biner hasil build) menampilkan UI dua sidebar.
- Cek manual singkat yang disarankan (bukan guard): toggle dan seret kedua sidebar, buat/ganti nama/hapus workspace dari tab, buat sesi dari sidebar kiri, buka file dari sidebar kanan, Cmd+K, dan pemicu notifikasi dengan sesi yang berpindah ke waiting/done.
