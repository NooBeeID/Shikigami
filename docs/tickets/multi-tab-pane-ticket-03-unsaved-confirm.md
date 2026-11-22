---
id: multi-tab-pane-ticket-03-unsaved-confirm
title: Konfirmasi tutup file belum disimpan
spec: docs/superpowers/specs/2026-11-22-multi-tab-pane-design.md
status: ready
wave: 3
depends_on: [multi-tab-pane-ticket-02-tab-dnd]
---
# Konfirmasi tutup file belum disimpan

## Tujuan
Setelah pane menjadi wadah banyak tab, satu klik tombol tutup tab atau "Tutup pane" bisa membuang sekaligus beberapa file yang suntingannya belum disimpan. Epic ini menambahkan jaring pengaman: sebelum tab file kotor ditutup, pengguna ditanya Simpan, Buang, atau Batal, dan tab file kotor diberi titik "belum disimpan". Dirty state hidup di dalam `FileTab.svelte` (`content` dibanding `savedContent`) sedangkan penutupan terjadi di luar komponen itu, sehingga dibutuhkan registri per id tab sebagai jembatan.

## Hasil yang bisa dijalankan
Di atas tiket 01 (pane berisi tab) dan 02 (drag tab, menu konteks tab):
- Buka file, ketik sesuatu tanpa menyimpan: tab file itu menampilkan titik "belum disimpan" (juga saat tab tidak aktif).
- Klik tombol tutup pada tab kotor: muncul dialog berisi nama file, tombol Simpan / Buang / Batal. Simpan menyimpan lalu menutup; Buang menutup tanpa menyimpan; Batal tidak menutup apa pun.
- "Tutup pane" pada pane berisi beberapa tab, dua di antaranya kotor: dialog menampilkan daftar dua file itu dengan tombol "Simpan semua" / "Buang semua" / Batal.
- Bila penyimpanan gagal, dialog tetap terbuka dengan pesan galat dan tab tidak tertutup.
- Menutup tab bersih (termasuk terminal) tidak memunculkan dialog. Memindah tab antar pane (drag, "Pisahkan ke pane baru") tidak memunculkan dialog.
- `npm run build` frontend dan build aplikasi lulus; aplikasi jalan.

## Batasan
In scope:
- Registri file tab (`registerFileTab`) dan store reaktif dirty per id tab di modul files.
- `FileTab.svelte` menerima id tab dan mendaftar/mencabut diri.
- `confirmCloseTabs(tabIds)` dan komponen dialognya.
- Pemanggilan `confirmCloseTabs` di tombol tutup tab, "Tutup pane", dan jalur tutup lain di `PaneHeader.svelte` / `PaneMenu.svelte` yang menghapus tab file.
- Titik belum disimpan pada tab file kotor di `PaneHeader.svelte`.

Out of scope:
- Perubahan daemon/IPC (semua frontend-only).
- Perubahan bentuk model di `layout/model.ts` (hanya boleh ditambah; epic ini tidak menyentuhnya).
- Konfirmasi untuk penutupan akibat sistem (tab file hilang karena jembatan v1, file dihapus dari disk) dan untuk `moveTab`.
- Penanganan tutup jendela / keluar aplikasi dengan file kotor: builder memeriksa apakah sudah ada; bila belum, catat sebagai item terpisah di laporan, jangan dikerjakan di sini.
- Unit test / vitest.
- Event `shikigami:tab-dirty` berbasis path (`setTabDirty` di `files/shared/shellBridge.ts`) tidak dipakai sebagai sumber titik di tab baru; jangan dihapus bila masih dipakai pemanggil lama, hanya jangan diandalkan.

## Kontrak
Disediakan epic ini (modul `frontend/src/modules/files/`, diekspor lewat `files/index.ts`):

```ts
// files/tabRegistry.ts
export interface FileTabHandle {
  isDirty(): boolean;
  save(): Promise<boolean>; // true bila berhasil (atau memang tidak ada yang perlu disimpan), false bila gagal
}
export function registerFileTab(tabId: string, handle: FileTabHandle): () => void; // mengembalikan fungsi cabut
export function unregisterFileTab(tabId: string): void;
export function getFileTab(tabId: string): FileTabHandle | undefined;
export function setFileTabDirty(tabId: string, dirty: boolean): void; // dipanggil FileTab saat dirty berubah
export const dirtyTabs: Readable<ReadonlySet<string>>; // store reaktif: id tab yang kotor
export function isTabDirty(tabId: string): boolean;

// files/confirmClose/index.ts
export function confirmCloseTabs(tabIds: string[]): Promise<boolean>;
// true = lanjut menutup (tanpa tab kotor, atau Simpan berhasil, atau Buang); false = batalkan penutupan.
```

Dipakai dari tiket sebelumnya (jangan diubah bentuknya):
- `Tab` (id prefix `t_`, `kind`, `filePath`, `projectId`, `title`, ...) dan `Leaf = { type:'pane', id, tabs: Tab[], activeTabId }` dari `layout/model.ts`; helper `activeTab`, `allTabs`, `findTab`; operasi `closeTab(doc, paneId, tabId)`, `closePane`.
- `PaneKindSpec.component` menerima `{ pane: Tab; visible: boolean }` plus `paneId` bila dibutuhkan; host konten di-key id tab dan tab non-aktif tetap ter-mount (`visible=false`) sehingga state kotor `FileTab` tetap ada.
- `PaneHeader.svelte` dan `PaneMenu.svelte` sudah punya deretan tab, tombol tutup tab, aksi "Tutup pane", dan menu konteks tab dari tiket 01 dan 02; epic ini hanya menambahkan pemanggilan konfirmasi dan titik dirty.

Aturan pemanggil: `confirmCloseTabs` hanya dipanggil dari aksi tutup yang dipicu pengguna. Penutupan sistem dan `moveTab` tidak memanggilnya.

## Fitur

### file-tab-registry: Registri file tab dan store dirty
Deskripsi:
- Buat `files/tabRegistry.ts` sesuai kontrak di atas: `Map<string, FileTabHandle>` plus `writable<Set<string>>` untuk id tab kotor. Update store harus membuat Set baru (imutabel) agar reaktif. `unregisterFileTab` juga menghapus id dari set dirty.
- Ekspor dari `files/index.ts`: `registerFileTab`, `unregisterFileTab`, `getFileTab`, `dirtyTabs`, `isTabDirty`. Tambahan ekspor ke index tidak boleh mengubah registrasi `registerTabType` / `registerFilesPanel` yang ada.
- `layout/panes/FilePane.svelte` meneruskan id tab ke `FileTab` lewat prop baru opsional `tabId` (`pane.id` dari prop `pane: Tab`, karena kontrak registry memberi `Tab`). Cabang `readOnly` (`ReadOnlyFileView`) tidak perlu mendaftar: tidak ada suntingan.
- `files/editor/FileTab.svelte`: terima prop opsional `tabId?: string`. Bila ada, di `onMount` panggil `registerFileTab(tabId, { isDirty: () => dirty, save })`, di `onDestroy` cabut. `save` yang didaftarkan membungkus fungsi `save()` yang ada menjadi `Promise<boolean>`: kembalikan `true` bila tidak kotor (tidak ada yang disimpan) atau penyimpanan sukses, `false` bila gagal (galat tetap ditampilkan di UI file seperti sekarang). Tambah `$effect` yang memanggil `setFileTabDirty(tabId, dirty)` setiap `dirty` berubah. Perilaku `FileTab` tanpa `tabId` (jalur lama) tidak boleh berubah.
- Perhatikan `save()` yang ada: `if (saving || mode !== 'edit' || !dirty) return;`. Pembungkus harus menangani `saving` sedang berjalan (tunggu hingga selesai atau kembalikan hasil akhirnya), dan mode bukan `edit` dengan `dirty` true (mis. harus tetap bisa disimpan; builder memeriksa apakah dirty mungkin true di luar mode edit, bila ya simpan tetap dilakukan lewat jalur tulis yang sama).
Acceptance criteria:
- [ ] `tabRegistry.ts` ada dan semua fungsi kontrak terekspor lewat `files/index.ts`.
- [ ] `FileTab` dengan `tabId` mendaftar saat mount dan mencabut saat destroy; tab file non-aktif tetap terdaftar dan `isDirty()` benar.
- [ ] `dirtyTabs` berisi id tab saat konten diubah dan kosong kembali setelah simpan.
- [ ] `save()` terdaftar mengembalikan `false` pada kegagalan tulis, `true` pada sukses.
- [ ] `FileTab` tanpa `tabId` berperilaku persis seperti sebelumnya.
- [ ] Build frontend lulus.
ownedPaths:
- `frontend/src/modules/files/editor/FileTab.svelte`
- `frontend/src/modules/files/tabRegistry.ts`
- `frontend/src/modules/files/index.ts`
- `frontend/src/modules/layout/panes/FilePane.svelte`

Catatan: bisa dikerjakan paralel dengan `confirm-close-dialog` (hanya bergantung pada tipe `FileTabHandle`/fungsi `getFileTab` dari kontrak di atas; `confirm-close-dialog` boleh memakai stub tipe yang sama sampai file ini ada). `unsaved-wiring` menunggu keduanya.

### confirm-close-dialog: confirmCloseTabs dan dialog
Deskripsi:
- Buat `files/confirmClose/index.ts` mengekspor `confirmCloseTabs(tabIds)` dan komponen `ConfirmCloseDialog.svelte` di folder yang sama.
- Algoritma: ambil tab kotor = `tabIds.filter(id => getFileTab(id)?.isDirty())`. Kosong: `return true` tanpa UI. Bila ada: tampilkan dialog dan kembalikan Promise yang resolve setelah pengguna memilih.
- Dialog: daftar file kotor (nama dari `Tab.title` / `filePath`; karena fungsi hanya menerima id tab, pemanggil tidak perlu meneruskan nama: ambil label dari registri atau lewat parameter opsional kedua `labels?: Record<string,string>`; jelaskan pilihan di kode). Judul tunjukkan jumlah file. Tombol: satu file = "Simpan" / "Buang" / "Batal"; banyak file = "Simpan semua" / "Buang semua" / "Batal".
  - Simpan: panggil `save()` tiap file kotor berurutan; bila semuanya `true` resolve `true` dan tutup dialog; bila ada yang `false`, resolve tidak dipanggil, dialog tetap terbuka, tampilkan pesan galat yang menyebut file yang gagal (Batal dan Buang tetap bisa dipilih), tombol dinonaktifkan selama menyimpan.
  - Buang: resolve `true` (tanpa menyimpan).
  - Batal / Escape / klik latar: resolve `false`.
- Pemasangan dialog: komponen dialog thread (`thread/close/CloseDialog.svelte`) khusus thread dan memakai store sendiri, jadi tidak dipakai ulang (boleh meniru gaya visual/pola overlay-nya: `role="dialog"`, `aria-modal`, fokus awal ke overlay, Escape menutup). Agar tidak menyentuh path lain, dialog dipasang imperatif: `mount(ConfirmCloseDialog, { target: document.body, props })` dari Svelte 5 dan `unmount` setelah resolve. Hanya satu dialog pada satu waktu; panggilan kedua saat dialog terbuka menunggu atau mengembalikan `false` (pilih salah satu dan dokumentasikan).
- Gaya memakai token CSS yang sudah ada di proyek (`var(--...)`), tampil benar di tema gelap/terang yang ada.
Acceptance criteria:
- [ ] Tanpa tab kotor, `confirmCloseTabs` resolve `true` seketika tanpa UI.
- [ ] Satu file kotor menampilkan tombol Simpan / Buang / Batal; banyak file menampilkan "Simpan semua" / "Buang semua" / Batal beserta daftar file.
- [ ] Simpan sukses resolve `true`; Simpan gagal mempertahankan dialog dengan galat dan tidak resolve `true`.
- [ ] Buang resolve `true`; Batal/Escape resolve `false`; tidak ada tab yang disentuh oleh fungsi ini (hanya mengembalikan keputusan).
- [ ] Dialog bisa dioperasikan keyboard (fokus awal, Escape) dan dibersihkan dari DOM setelah selesai.
- [ ] Build frontend lulus.
ownedPaths:
- `frontend/src/modules/files/confirmClose/`

Catatan: bisa dikerjakan paralel dengan `file-tab-registry` (lihat catatan di sana). Ekspor `confirmCloseTabs` ke luar modul lewat `files/confirmClose/index.ts`; pemanggil (`unsaved-wiring`) mengimpornya langsung dari path `files/confirmClose` karena `files/index.ts` dimiliki fitur `file-tab-registry`.

### unsaved-wiring: Wiring konfirmasi dan titik belum disimpan
Deskripsi:
- Di `layout/PaneHeader.svelte`: sebelum memanggil `closeTab(paneId, tabId)` dari tombol tutup tab, panggil `await confirmCloseTabs([tabId])`; lanjut hanya bila `true`. Handler menjadi async; cegah klik ganda saat dialog terbuka.
- Di `PaneHeader.svelte` / `PaneMenu.svelte`: "Tutup pane" memanggil `confirmCloseTabs(semua id tab di pane)` sebelum menutup. Jalur lain di kedua file yang menghapus tab file milik pengguna (mis. "Tutup tab lain", "Tutup semua" di menu konteks tab dari tiket 02, bila ada) juga memanggilnya dengan id tab yang akan ditutup. Builder menelusuri semua pemanggil `closeTab` / `closePane` di kedua file dan memastikan tiap jalur yang dipicu pengguna terlindungi.
- Jangan panggil konfirmasi di: `moveTab`, "Pisahkan ke pane baru", reorder, minimize tab terminal (tab terminal tidak kotor), "Akhiri sesi", dan penutupan sistem. Bila `closePane`/`closeTab` dipanggil dari modul lain (shell, groups, jembatan v1), jangan ubah, itu di luar path yang dimiliki.
- Titik belum disimpan: di item tab pada `PaneHeader.svelte`, baca `dirtyTabs` dari `files` (impor dari modul files) dan tampilkan titik kecil (mis. `●` dengan `title="Belum disimpan"`, `aria-label`/teks sr-only "Belum disimpan") pada tab yang id-nya ada di set. Letakkan di tempat yang tidak menggeser tombol tutup (mis. titik menggantikan ikon status di tab file atau di sebelah judul); pada tab non-file tidak ada perubahan.
- Verifikasi end-to-end: buka dua file, ubah keduanya, tutup satu tab (dialog satu file), tutup pane (dialog dua file), uji Simpan, Buang, Batal, dan simulasi galat simpan (mis. file dibuat read-only di disk) bila memungkinkan.
Acceptance criteria:
- [ ] Tombol tutup tab file kotor membuka dialog; Batal membiarkan tab terbuka, Buang menutup tanpa menyimpan, Simpan menyimpan lalu menutup.
- [ ] "Tutup pane" dan jalur tutup massal di PaneHeader/PaneMenu mengumpulkan semua tab file kotor dalam satu dialog.
- [ ] Tab bersih dan tab terminal menutup tanpa dialog dan tanpa jeda yang terasa.
- [ ] `moveTab`/drag/split tab tidak memunculkan dialog dan tidak me-remount isi (state kotor tetap).
- [ ] Titik belum disimpan muncul pada tab file kotor (aktif maupun non-aktif) dan hilang setelah simpan.
- [ ] Build frontend dan build aplikasi lulus; aplikasi jalan.
ownedPaths:
- `frontend/src/modules/layout/PaneHeader.svelte`
- `frontend/src/modules/layout/PaneMenu.svelte`

Catatan: bergantung pada `file-tab-registry` dan `confirm-close-dialog`; dikerjakan setelah keduanya (tahap penutup epic), bukan paralel.

## Path yang dimiliki
Hanya ini yang boleh disentuh:
- `frontend/src/modules/files/` (termasuk `editor/FileTab.svelte`, `tabRegistry.ts`, `index.ts`, `confirmClose/`)
- `frontend/src/modules/layout/panes/FilePane.svelte`
- `frontend/src/modules/layout/PaneHeader.svelte`
- `frontend/src/modules/layout/PaneMenu.svelte`

Jangan menyentuh `layout/model.ts`, `layout/registry.ts`, `layout/LayoutArea.svelte`, `layout/dnd.ts`, `layout/DropOverlay.svelte`, `layout/TabContextMenu.svelte`, atau file lain di luar daftar ini. `PaneHeader.svelte` sudah diubah berurutan oleh tiket 01 dan 02; bangun di atas hasilnya.

## Wiring
Langkah penutup tiket ini (setelah ketiga fitur selesai, dalam fitur `unsaved-wiring`):
1. `FilePane.svelte` meneruskan `pane.id` (id tab) sebagai `tabId` ke `FileTab` sehingga setiap tab file terdaftar di registri saat ter-mount.
2. `PaneHeader.svelte` dan `PaneMenu.svelte` mengimpor `confirmCloseTabs` dari `files/confirmClose` dan `dirtyTabs` dari `files`, lalu memanggil konfirmasi sebelum `closeTab` / tutup pane dan menampilkan titik dirty pada tab.
3. Tidak ada titik ekstensi tiket 01 yang baru: tombol tutup tab dan menu pane sudah ada dari tiket 01/02; epic ini hanya membungkus aksi tutupnya. Tiket ini hanya bergantung pada tiket 01 dan 02 (sudah selesai di wave sebelumnya) dan dapat dijalankan sendiri tanpa tiket lain.
4. Jalankan aplikasi dan lakukan skenario end-to-end pada bagian "Hasil yang bisa dijalankan".

## Guard
- `cd frontend && npm run build` berhasil (termasuk `svelte-check`/type check bila skrip build memuatnya).
- Build aplikasi berhasil (mis. `go build ./...` dan/atau perintah build aplikasi yang dipakai proyek).
- Tanpa unit test dan tanpa menambah vitest.
