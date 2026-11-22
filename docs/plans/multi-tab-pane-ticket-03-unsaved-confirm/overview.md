# Overview: multi-tab-pane-ticket-03-unsaved-confirm

Tiket: `docs/tickets/multi-tab-pane-ticket-03-unsaved-confirm.md`
Spec: `docs/superpowers/specs/2026-11-22-multi-tab-pane-design.md`
Plan tiket sebelumnya (kontrak yang dipakai): `docs/plans/multi-tab-pane-ticket-01-tab-container/overview.md`, `docs/plans/multi-tab-pane-ticket-02-tab-dnd/overview.md`

Semua perubahan frontend-only (Svelte 5 + TypeScript) di `frontend/`. Tanpa perubahan daemon/IPC/Go. Tanpa unit test (jangan tambah vitest).

Catatan prasyarat: tiket 01 dan 02 HARUS sudah di-merge sebelum epic ini dikerjakan. Pada saat plan ini ditulis, `PaneHeader.svelte`, `PaneMenu.svelte`, dan `FilePane.svelte` di repo masih bentuk lama (`Leaf` berisi field konten langsung, `onClose` tunggal, belum ada deretan tab). Semua rujukan ke `Tab`, `Leaf.tabs`, `closeTab`, `allTabs`, `findTab`, menu konteks tab, dan "Tutup pane" per-tab mengikuti kontrak tiket 01/02. Agent WAJIB membaca kode aktual di working tree dulu; bila nama aktual berbeda tipis dari kontrak, ikuti kode aktual dan catat di laporan.

## 1. Ringkasan epic dan urutan eksekusi

Setelah pane menjadi wadah banyak tab, satu klik tutup tab atau "Tutup pane" bisa membuang beberapa file yang belum disimpan. Epic ini menambah konfirmasi Simpan / Buang / Batal sebelum tab file kotor ditutup, plus titik "belum disimpan" pada tab file kotor. Dirty state hidup di `FileTab.svelte` (`content` vs `savedContent`) sedangkan penutupan terjadi di luar, jadi dipakai registri per id tab sebagai jembatan.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`, satu agent, sekuensial, kecil): buat `files/tabRegistry.ts` lengkap sesuai kontrak (kecil, murni TS). Alasan: `confirm-close-dialog` mengimpor `getFileTab` dari file ini, dan `file-tab-registry` memiliki file ini; dengan foundation, keduanya bisa paralel tanpa stub dan tanpa konflik.
2. **Fitur paralel** (setelah foundation): `file-tab-registry` dan `confirm-close-dialog`. ownedPaths terpisah.
3. **Wiring** (`unsaved-wiring`, terakhir, setelah keduanya selesai): `PaneHeader.svelte` dan `PaneMenu.svelte` memanggil konfirmasi dan menampilkan titik dirty; build guard; verifikasi manual end-to-end.

## 2. Kontrak bersama

### 2.1 Dari tiket 01/02 (jangan diubah bentuknya)

- `Tab` (id prefix `t_`, `kind`, `filePath`, `projectId`, `title`, ...), `Leaf = { type:'pane', id, tabs: Tab[], activeTabId }` dari `frontend/src/modules/layout/model.ts`; helper `activeTab`, `allTabs`, `findTab`; operasi `closeTab(doc, paneId, tabId)`, `closePane`. Epic ini TIDAK menyentuh `layout/model.ts`.
- `PaneKindSpec.component` menerima `{ pane: Tab; visible: boolean }` (+ `paneId` bila ada). Host di-key id tab; tab non-aktif tetap ter-mount (`visible=false`), sehingga state kotor `FileTab` tetap ada. Karena `pane` di `FilePane.svelte` bertipe `Tab`, id tab = `pane.id`.
- `PaneHeader.svelte` / `PaneMenu.svelte` sudah punya deretan tab, tombol tutup tab, "Tutup pane", dan menu konteks tab (tiket 01/02). Epic ini hanya membungkus aksi tutup dan menambah titik dirty.

### 2.2 `files/tabRegistry.ts` (dibuat foundation, dirampungkan/dikonsumsi `file-tab-registry`)

```ts
// frontend/src/modules/files/tabRegistry.ts
import type { Readable } from 'svelte/store';

export interface FileTabHandle {
  isDirty(): boolean;
  save(): Promise<boolean>; // true bila berhasil atau memang tidak ada yang perlu disimpan; false bila gagal
}
export function registerFileTab(tabId: string, handle: FileTabHandle): () => void; // mengembalikan fungsi cabut
export function unregisterFileTab(tabId: string): void; // juga menghapus id dari set dirty
export function getFileTab(tabId: string): FileTabHandle | undefined;
export function setFileTabDirty(tabId: string, dirty: boolean): void; // set BARU tiap perubahan (imutabel)
export const dirtyTabs: Readable<ReadonlySet<string>>;
export function isTabDirty(tabId: string): boolean;
```

Detail implementasi: `Map<string, FileTabHandle>` modul-level + `writable<ReadonlySet<string>>(new Set())`. `setFileTabDirty` no-op bila status tidak berubah; kalau berubah, `update` membuat `new Set(old)` lalu add/delete. Fungsi cabut dari `registerFileTab` hanya mencabut bila handle di map masih handle yang sama (hindari mencabut registrasi baru pada remount cepat).

Ekspor dari `files/index.ts` (dimiliki `file-tab-registry`): `registerFileTab`, `unregisterFileTab`, `getFileTab`, `dirtyTabs`, `isTabDirty`, dan `export type { FileTabHandle }`. Tambahan ekspor tidak boleh mengubah `registerTabType({ type: 'file', ... })` dan `registerFilesPanel()` yang ada.

### 2.3 `files/confirmClose/` (dimiliki `confirm-close-dialog`)

```ts
// frontend/src/modules/files/confirmClose/index.ts
export function confirmCloseTabs(
  tabIds: string[],
  labels?: Record<string, string>, // tabId -> nama tampilan; opsional
): Promise<boolean>;
// true = lanjut menutup (tanpa tab kotor, atau Simpan berhasil, atau Buang)
// false = batalkan penutupan (Batal / Escape / klik latar / dialog lain sedang terbuka)
```

Keputusan desain (agar tidak ambigu):
- Tab kotor = `tabIds.filter((id) => getFileTab(id)?.isDirty())`. Kosong: `return true` seketika, tanpa UI.
- Label: `labels?.[id]` bila ada, jika tidak fallback ke `id` tab. Pemanggil (`unsaved-wiring`) SELALU meneruskan `labels` dari `Tab.title` atau basename `Tab.filePath`. Fungsi ini tidak mengimpor layout store.
- Hanya satu dialog sekaligus: panggilan kedua saat dialog terbuka langsung mengembalikan `false` (tidak antre). Dokumentasikan di komentar kode. Pemanggil juga mencegah klik ganda.
- Fungsi ini TIDAK menyentuh tab/layout; hanya mengembalikan keputusan.
- Pemasangan imperatif: `mount(ConfirmCloseDialog, { target: document.body, props })` (Svelte 5) lalu `unmount` setelah resolve. Tidak ada store global, tidak ada perubahan di `App`/shell.
- `ConfirmCloseDialog.svelte` props: `{ items: { id: string; label: string }[]; onSave: ...; ... }`, bentuk persisnya bebas selama internal ke folder `confirmClose/`. Perilaku wajib:
  - `role="dialog"`, `aria-modal="true"`, `aria-labelledby`; fokus awal ke overlay/tombol utama; Escape = Batal; klik latar = Batal.
  - Satu file: tombol "Simpan" / "Buang" / "Batal". Banyak file: judul memuat jumlah, daftar file, tombol "Simpan semua" / "Buang semua" / "Batal".
  - Simpan: `save()` tiap file kotor berurutan (`getFileTab(id)?.save()`); semua `true` -> resolve `true`. Ada yang `false`: dialog tetap terbuka, pesan galat menyebut nama file yang gagal, Batal dan Buang tetap aktif, tombol Simpan dinonaktifkan selama menyimpan. Handle yang sudah tidak ada saat menyimpan (tab ter-unmount) diperlakukan sebagai `true`.
  - Buang: resolve `true`. Batal/Escape/latar: resolve `false`.
  - Gaya memakai token CSS yang ada (`var(--...)`, lihat pola `thread/close/CloseDialog.svelte`), benar di tema gelap/terang.

### 2.4 Perubahan `FilePane.svelte` dan `FileTab.svelte` (dimiliki `file-tab-registry`)

- `FileTab.svelte` prop baru opsional `tabId?: string`. Bila ada: di `onMount` panggil `registerFileTab(tabId, { isDirty: () => dirty, save: saveForClose })`, di `onDestroy` cabut. `$effect` memanggil `setFileTabDirty(tabId, dirty)` tiap `dirty` berubah. Tanpa `tabId`: perilaku lama persis, termasuk `setTabDirty(path, dirty)` event lama (jangan dihapus, jangan diandalkan).
- `saveForClose(): Promise<boolean>` (nama bebas) membungkus `doWrite()`:
  1. Bila `saving` sedang berjalan: tunggu hingga selesai (mis. polling `$effect`/promise yang di-resolve di `finally` `doWrite`), lalu evaluasi hasil akhir.
  2. Bila `!dirty`: `true`.
  3. Bila `dirty`: panggil `doWrite()` langsung (JANGAN lewat `save()` yang memblokir `mode !== 'edit'`); secara praktik `dirty` hanya mungkin di mode edit, tetapi jalur ini tetap aman.
  4. Hasil = `!dirty` setelah `doWrite()`. Ini menangani kegagalan yang tidak melempar: `saveError` terisi, atau `banner` conflict/removed (doWrite menangkap semua galat dan tidak melempar). Galat tetap tampil di UI file seperti sekarang.
- `FilePane.svelte`: teruskan `tabId={pane.id}` ke `<FileTab>` pada cabang non-readOnly. `ReadOnlyFileView` tidak mendaftar.

### 2.5 Titik dirty dan pemanggil tutup (dimiliki `unsaved-wiring`)

- `PaneHeader.svelte`: import `dirtyTabs` dari `../files` dan `confirmCloseTabs` dari `../files/confirmClose`. Pada item tab, bila `$dirtyTabs.has(tab.id)`: tampilkan titik `●` dengan `title="Belum disimpan"` dan teks sr-only "Belum disimpan", tidak menggeser tombol tutup (mis. di sebelah judul / mengganti ikon status tab file). Tab non-file tidak berubah.
- Semua jalur tutup yang dipicu pengguna di `PaneHeader.svelte`/`PaneMenu.svelte` memanggil `await confirmCloseTabs(ids, labels)` dan lanjut hanya bila `true`: tombol tutup tab (`[tabId]`), "Tutup pane" (semua id tab pane), "Tutup tab lain"/"Tutup semua" bila ada di menu konteks tab tiket 02. Handler async dengan guard anti klik ganda (flag `closing`).
- JANGAN panggil di: `moveTab`, "Pisahkan ke pane baru", reorder, minimize tab terminal, "Akhiri sesi", penutupan sistem, serta pemanggil `closeTab`/`closePane` di modul lain (shell, groups, jembatan v1).
- Jendela/keluar aplikasi dengan file kotor: wiring memeriksa apakah sudah ada penanganan; bila belum, hanya CATAT di laporan sebagai item terpisah.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `file-tab-registry` | Ekspor di `files/index.ts`; `FilePane` meneruskan `tabId`; `FileTab` mendaftar/mencabut dan menyinkronkan dirty; `save` terdaftar `Promise<boolean>` (rampungkan `tabRegistry.ts` dari foundation bila perlu) | `frontend/src/modules/files/editor/FileTab.svelte`, `frontend/src/modules/files/tabRegistry.ts`, `frontend/src/modules/files/index.ts`, `frontend/src/modules/layout/panes/FilePane.svelte` |
| `confirm-close-dialog` | `confirmCloseTabs` + `ConfirmCloseDialog.svelte` (mount imperatif, satu dialog sekali) | `frontend/src/modules/files/confirmClose/` |
| `unsaved-wiring` (tahap penutup, bukan paralel) | Pasang konfirmasi di tombol tutup tab, "Tutup pane", jalur tutup massal; titik dirty di tab; verifikasi e2e | `frontend/src/modules/layout/PaneHeader.svelte`, `frontend/src/modules/layout/PaneMenu.svelte` |

Tiket menyebut `unsaved-wiring` sebagai fitur ketiga yang menunggu dua lainnya. Ia dijalankan pada tahap Wiring dan isinya adalah wiring checklist di bagian 5.

## 4. Foundation

Ada, kecil: `00-foundation.md`. Isi: buat `frontend/src/modules/files/tabRegistry.ts` lengkap sesuai 2.2, lalu pastikan `cd frontend && npm run build` lulus. Foundation TIDAK menyentuh `files/index.ts`, `FileTab.svelte`, `FilePane.svelte`, `confirmClose/` (itu milik fitur). Setelah foundation, `file-tab-registry` hanya perlu memverifikasi/menyesuaikan `tabRegistry.ts` bila perlu, serta mengerjakan sisanya.

## 5. Wiring checklist (untuk agen `unsaved-wiring`, setelah kedua fitur selesai)

1. Verifikasi `files/index.ts` mengekspor `dirtyTabs`, `isTabDirty`, `registerFileTab`, dst., dan `FilePane` meneruskan `tabId={pane.id}` (hasil `file-tab-registry`); `files/confirmClose/index.ts` mengekspor `confirmCloseTabs` (hasil `confirm-close-dialog`). Jangan edit file fitur lain; bila kurang, laporkan.
2. `PaneHeader.svelte`: impor `confirmCloseTabs` dari `../files/confirmClose` dan `dirtyTabs` dari `../files`. Bungkus handler tutup tab: `await confirmCloseTabs([tab.id], { [tab.id]: label })`, lanjut `closeTab` hanya bila `true`. Guard klik ganda.
3. `PaneHeader.svelte` + `PaneMenu.svelte`: "Tutup pane" memanggil `confirmCloseTabs(pane.tabs.map(t => t.id), labels)` sebelum menutup. Telusuri SEMUA pemanggil `closeTab`/`closePane` di kedua file dan lindungi tiap jalur pemicu pengguna (tutup tab lain, tutup semua, dst.). Pengecualian lihat 2.5.
4. Titik dirty di item tab `PaneHeader.svelte` (aktif maupun non-aktif) sesuai 2.5.
5. Jalankan aplikasi dan uji: buka dua file, ubah keduanya; tutup satu tab (dialog satu file); tutup pane (dialog dua file); uji Simpan, Buang, Batal; simulasi galat simpan (file read-only di disk) bila memungkinkan; tab bersih dan terminal menutup tanpa dialog; drag/split tab tidak memunculkan dialog dan tidak me-remount (state kotor tetap).
6. Periksa dan catat (jangan kerjakan) penanganan tutup jendela/keluar aplikasi dengan file kotor.

## 6. Build guard

Satu-satunya guard (tanpa unit test, tanpa vitest):

- `cd frontend && npm run build` (vite build). Jalankan juga `cd frontend && npm run check` (svelte-check) bila bersih di baseline; bila baseline sudah punya galat, pastikan tidak ada galat baru dari file yang disentuh.
- Build aplikasi: `make build` (atau minimal `make build-ui && make build-app`), dan `go build ./...` bila tersedia. Wajib hanya di tahap Wiring; fitur paralel cukup build frontend.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya. Jangan menyentuh path fitur lain atau file wiring (`PaneHeader.svelte`, `PaneMenu.svelte`).
- Jangan sentuh `layout/model.ts`, `layout/registry.ts`, `layout/LayoutArea.svelte`, `layout/dnd.ts`, `layout/DropOverlay.svelte`, `layout/TabContextMenu.svelte`, `files/shared/shellBridge.ts`, atau file lain di luar daftar.
- `confirm-close-dialog` hanya mengimpor `getFileTab` (dan tipe `FileTabHandle`) dari `../tabRegistry` (path langsung, bukan `files/index.ts`, untuk menghindari impor sirkular dan karena `index.ts` dimiliki fitur lain). `unsaved-wiring` mengimpor `confirmCloseTabs` dari `files/confirmClose`.
- Jangan mengubah bentuk kontrak di bagian 2; bila perlu menyimpang, catat di laporan fitur.
- Perilaku `FileTab` tanpa `tabId` tidak boleh berubah.
- Tulis kode dengan komentar/UI berbahasa Indonesia, identifier berbahasa Inggris.
