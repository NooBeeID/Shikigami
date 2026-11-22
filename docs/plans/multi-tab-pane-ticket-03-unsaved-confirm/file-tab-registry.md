# Plan: Registri file tab dan store dirty

## Metadata

- Tiket: `multi-tab-pane-ticket-03-unsaved-confirm` (`docs/tickets/multi-tab-pane-ticket-03-unsaved-confirm.md`)
- Fitur: `file-tab-registry`
- Overview: `docs/plans/multi-tab-pane-ticket-03-unsaved-confirm/overview.md` (baca bagian 2.2 dan 2.4)
- ownedPaths:
  - `frontend/src/modules/files/editor/FileTab.svelte`
  - `frontend/src/modules/files/tabRegistry.ts`
  - `frontend/src/modules/files/index.ts`
  - `frontend/src/modules/layout/panes/FilePane.svelte`
- Depends on foundation: ya (`00-foundation.md` membuat `files/tabRegistry.ts`). Prasyarat lain: tiket 01 dan 02 sudah ter-merge.

## Tujuan

Menjadikan dirty state tiap `FileTab` terlihat dari luar komponen lewat registri per id tab: `FileTab` mendaftar diri (handle `isDirty` + `save`) dan menyinkronkan status dirty ke store `dirtyTabs`. Fitur ini hanya menyediakan registri dan pendaftarannya; dialog konfirmasi dan pemasangan di header/menu dikerjakan fitur lain.

## Path yang dimiliki

HANYA boleh menyentuh:

- `frontend/src/modules/files/editor/FileTab.svelte`
- `frontend/src/modules/files/tabRegistry.ts`
- `frontend/src/modules/files/index.ts`
- `frontend/src/modules/layout/panes/FilePane.svelte`

JANGAN menyentuh `layout/PaneHeader.svelte`, `layout/PaneMenu.svelte` (file wiring), `files/confirmClose/` (fitur lain), `layout/model.ts`, `files/shared/shellBridge.ts`, atau file lain.

## Kontrak

Disediakan (bentuk persis di overview 2.2, jangan diubah):

```ts
// files/tabRegistry.ts
export interface FileTabHandle { isDirty(): boolean; save(): Promise<boolean>; }
export function registerFileTab(tabId: string, handle: FileTabHandle): () => void;
export function unregisterFileTab(tabId: string): void;
export function getFileTab(tabId: string): FileTabHandle | undefined;
export function setFileTabDirty(tabId: string, dirty: boolean): void;
export const dirtyTabs: Readable<ReadonlySet<string>>;
export function isTabDirty(tabId: string): boolean;
```

Ekspor dari `files/index.ts`: `registerFileTab`, `unregisterFileTab`, `getFileTab`, `dirtyTabs`, `isTabDirty`, dan `export type { FileTabHandle }`. (`setFileTabDirty` hanya dipakai `FileTab` lewat import langsung `../tabRegistry`; boleh tidak diekspor dari index.)

Dipakai: `pane: Tab` pada `FilePane.svelte` (id tab = `pane.id`, kontrak tiket 01). Perilaku `FileTab` tanpa `tabId` tidak boleh berubah, termasuk `setTabDirty(path, dirty)` lama.

## Langkah

1. Baca kode aktual: `files/tabRegistry.ts` (dari foundation), `files/index.ts`, `files/editor/FileTab.svelte`, `layout/panes/FilePane.svelte`. Bila nama aktual berbeda tipis dari kontrak (mis. tipe `pane` di `FilePane`), ikuti kode aktual dan catat di laporan.

2. Verifikasi/rampungkan `files/tabRegistry.ts` terhadap overview 2.2. Bila file belum ada (foundation belum jalan), buat sesuai `00-foundation.md`. Pastikan:
   - `Map<string, FileTabHandle>` modul-level dan `writable<ReadonlySet<string>>(new Set())`.
   - `dirtyTabs` hanya mengekspos `subscribe`.
   - `registerFileTab` mengembalikan fungsi cabut yang hanya mencabut bila `handles.get(tabId) === handle`.
   - `unregisterFileTab` menghapus handle dan memanggil `setFileTabDirty(tabId, false)`.
   - `setFileTabDirty` no-op bila status sama; bila berubah memakai `new Set(old)` lalu add/delete (imutabel).
   - `isTabDirty` memakai `get(dirtySet).has(tabId)`.
   Ubah hanya bila ada yang menyimpang.

3. `files/index.ts`: tambahkan di bawah kode yang ada, tanpa mengubah `registerTabType({ type: 'file', component: FileTab })` dan `registerFilesPanel()`:
   ```ts
   export { registerFileTab, unregisterFileTab, getFileTab, dirtyTabs, isTabDirty } from './tabRegistry';
   export type { FileTabHandle } from './tabRegistry';
   ```

4. `layout/panes/FilePane.svelte`: pada cabang non-readOnly ubah `<FileTab {refId} />` menjadi `<FileTab {refId} tabId={pane.id} />`. Cabang `ReadOnlyFileView` tidak berubah (tidak mendaftar). Bila bentuk file sudah berubah dari tiket 01 (mis. `pane` bertipe `Tab`), tetap gunakan id tab = `pane.id`.

5. `files/editor/FileTab.svelte`:
   - Import: ubah `import { onDestroy } from 'svelte'` menjadi `import { onDestroy, onMount } from 'svelte'`; tambah `import { registerFileTab, setFileTabDirty } from '../tabRegistry';`.
   - Komponen memakai `let props: Record<string, unknown> = $props();` dan `decodeFileTab(props)` (hanya membaca `refId`, mengabaikan kunci lain). Pertahankan pola ini; baca id dengan `const tabId = $derived(typeof props.tabId === 'string' ? props.tabId : undefined);`. Jangan mengganti ke destructuring `$props()` dan jangan mengubah `decodeFileTab`.
   - Tambah `saveForClose` setelah `save()`:
     ```ts
     // Dipakai registri tutup tab: tulis langsung lewat doWrite (tanpa guard mode di save()),
     // karena dirty bisa saja true saat mode bukan 'edit'.
     async function saveForClose(): Promise<boolean> {
       // Tunggu penulisan yang sedang berjalan, lalu evaluasi hasil akhirnya.
       while (saving) await new Promise<void>((r) => setTimeout(r, 25));
       if (!dirty) return true;
       await doWrite();
       // doWrite menangkap semua galat dan tidak melempar; kegagalan (saveError,
       // banner conflict/removed) terlihat sebagai dirty yang masih true.
       return !dirty;
     }
     ```
     Alternatif polling yang lebih rapi (promise yang di-resolve di `finally` `doWrite`) boleh dipakai, asalkan `doWrite` tidak berubah perilakunya. Jangan ubah `doWrite()` dan `save()`.
   - Daftarkan di `onMount` (tambahkan blok baru, jangan mengganggu `onDestroy` yang ada selain menambah pencabutan):
     ```ts
     let unregister: (() => void) | undefined;
     onMount(() => {
       const id = tabId;
       if (!id) return;
       unregister = registerFileTab(id, { isDirty: () => dirty, save: saveForClose });
     });
     ```
     Di `onDestroy` yang ada tambahkan `unregister?.();` (sebelum/sesudah `setTabDirty(path, false)`; baris lama tetap). Catatan: `isDirty` membaca `$derived` `dirty` saat dipanggil sehingga selalu nilai terkini; `tabId` tetap per instance karena host di-key id tab.
   - Tambah `$effect` berdampingan dengan `$effect(() => { setTabDirty(path, dirty); })` yang lama (jangan dihapus):
     ```ts
     $effect(() => {
       const id = tabId;
       if (id) setFileTabDirty(id, dirty);
     });
     ```
     Pencabutan via `unregisterFileTab` sudah menghapus id dari set dirty saat destroy.
   - Tanpa `tabId`: tidak ada registrasi, tidak ada efek tambahan; semua jalur lama tetap sama.

6. Build: `cd frontend && npm run build`. Jalankan juga `cd frontend && npm run check` bila bersih di baseline; bila baseline sudah punya galat, pastikan tidak ada galat baru dari file yang disentuh.

7. Periksa kebenaran statis (tanpa unit test): `git diff` hanya berisi empat path milik; tidak ada perubahan pada `setTabDirty` lama, `doWrite`, `save`.

## Perubahan

- Diubah: `frontend/src/modules/files/editor/FileTab.svelte` (import, `tabId` derived, `saveForClose`, registrasi `onMount`/`onDestroy`, `$effect` dirty)
- Diubah: `frontend/src/modules/files/index.ts` (ekspor registri)
- Diubah: `frontend/src/modules/layout/panes/FilePane.svelte` (`tabId={pane.id}`)
- Diverifikasi, diubah hanya bila menyimpang: `frontend/src/modules/files/tabRegistry.ts` (dibuat foundation)

## Wiring

Yang diekspos untuk langkah wiring (`unsaved-wiring`) dan `confirm-close-dialog`; JANGAN menyambungkannya sendiri:

- `files/index.ts`: `dirtyTabs` (dipakai `PaneHeader.svelte` untuk titik belum disimpan), `isTabDirty`, `registerFileTab`, `unregisterFileTab`, `getFileTab`, tipe `FileTabHandle`.
- `files/tabRegistry.ts`: `getFileTab` / `FileTabHandle` (diimpor langsung oleh `confirmClose/` dari `../tabRegistry`).
- Setiap tab file non-readOnly terdaftar di registri dengan id `pane.id` selama ter-mount; `save()` terdaftar mengembalikan `Promise<boolean>`.
- Tidak ada perubahan pada `PaneHeader.svelte`, `PaneMenu.svelte`, atau pemanggilan `confirmCloseTabs` di fitur ini.

## Selesai bila

- [ ] `tabRegistry.ts` ada dan sesuai kontrak; semua fungsi/tipe kontrak terekspor lewat `files/index.ts`, registrasi `registerTabType`/`registerFilesPanel` tidak berubah.
- [ ] `FilePane.svelte` meneruskan `tabId={pane.id}` hanya pada cabang non-readOnly.
- [ ] `FileTab` dengan `tabId` mendaftar saat mount, mencabut saat destroy; tab non-aktif tetap terdaftar dan `isDirty()` benar.
- [ ] `dirtyTabs` berisi id tab saat konten diubah dan kembali kosong setelah simpan atau destroy.
- [ ] `save()` terdaftar mengembalikan `true` bila sukses atau tidak kotor, `false` bila gagal tulis (conflict/removed/saveError); menunggu `saving` yang sedang berjalan; tidak lewat guard mode.
- [ ] `FileTab` tanpa `tabId` berperilaku persis seperti sebelumnya (`setTabDirty(path, ...)` lama utuh).
- [ ] Guard: `cd frontend && npm run build` sukses (tanpa unit test, tanpa vitest); tidak ada galat `svelte-check` baru dari file yang disentuh.
- [ ] Hanya path di "Path yang dimiliki" yang berubah.
