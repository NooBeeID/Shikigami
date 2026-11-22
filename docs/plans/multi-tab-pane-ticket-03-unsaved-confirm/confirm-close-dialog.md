# Plan: confirmCloseTabs dan dialog konfirmasi

## Metadata

- Tiket: `multi-tab-pane-ticket-03-unsaved-confirm` (`docs/tickets/multi-tab-pane-ticket-03-unsaved-confirm.md`)
- Fitur: `confirm-close-dialog`
- ownedPaths: `frontend/src/modules/files/confirmClose/`
- Depends on foundation: ya. `frontend/src/modules/files/tabRegistry.ts` (getFileTab, tipe FileTabHandle) dibuat oleh foundation sebelum fitur ini dikerjakan. Verifikasi file itu ada; bila belum ada, hentikan dan laporkan (jangan membuatnya, itu bukan milik fitur ini).
- Frontend-only (Svelte 5 + TypeScript). Tanpa unit test, jangan tambah vitest.

## Tujuan

Menyediakan `confirmCloseTabs(tabIds, labels?)` yang menampilkan dialog Simpan / Buang / Batal bila ada tab file kotor di `tabIds`, dan hanya mengembalikan keputusan (`true` lanjut menutup, `false` batalkan). Fungsi ini tidak menyentuh tab/layout. Spesifikasi perilaku lengkap ada di `overview.md` bagian 2.3; plan ini menerjemahkannya ke langkah kerja.

## Path yang dimiliki

HANYA boleh membuat/mengubah file di `frontend/src/modules/files/confirmClose/`:
- `index.ts` (baru)
- `ConfirmCloseDialog.svelte` (baru)

Jangan sentuh: `files/tabRegistry.ts`, `files/index.ts`, `files/editor/FileTab.svelte`, `layout/panes/FilePane.svelte`, `layout/PaneHeader.svelte`, `layout/PaneMenu.svelte`, `layout/model.ts`, atau file lain. Jangan menyambungkan ke pemanggil (itu tahap wiring).

## Kontrak

Disediakan (overview 2.3):

```ts
// frontend/src/modules/files/confirmClose/index.ts
export function confirmCloseTabs(
  tabIds: string[],
  labels?: Record<string, string>,
): Promise<boolean>;
```

Dipakai (overview 2.2, tidak boleh diubah bentuknya):

```ts
// ../tabRegistry (impor path langsung, BUKAN ../index)
import { getFileTab, type FileTabHandle } from '../tabRegistry';
// FileTabHandle { isDirty(): boolean; save(): Promise<boolean> }
```

Aturan perilaku:
- Tab kotor = `tabIds.filter((id) => getFileTab(id)?.isDirty())`. Kosong: `return true` tanpa UI.
- Label = `labels?.[id] ?? id`.
- Satu dialog sekali: panggilan kedua saat dialog terbuka mengembalikan `false` (tidak antre); dokumentasikan di komentar.
- Simpan berurutan; semua `true` -> `true`. Ada `false`: dialog tetap terbuka, galat menyebut nama file yang gagal; Batal dan Buang tetap aktif; Simpan dinonaktifkan selama menyimpan. Handle sudah tidak ada saat menyimpan (`getFileTab(id)` undefined) dianggap `true`.
- Buang -> `true`. Batal / Escape / klik latar -> `false`.
- Teks UI Indonesia: "Simpan", "Buang", "Batal", "Simpan semua", "Buang semua".

## Langkah

1. Baca `frontend/src/modules/thread/close/CloseDialog.svelte` sebagai acuan gaya dan pola overlay (jangan diubah, jangan diimpor). Pola yang ditiru: `.overlay` fixed inset 0 z-index 100, `.dlg` dengan `var(--bg-elev, #fff)`, `var(--fg, #222)`, `var(--border, #8884)`, `var(--danger, #e5484d)`, `var(--fg-muted, #888)`; `role="dialog" aria-modal="true"`; overlay `tabindex="-1"` dengan fokus awal; `onpointerdown` pada overlay dengan cek `e.target === e.currentTarget`. Pastikan `frontend/src/modules/files/tabRegistry.ts` ada.

2. Buat `ConfirmCloseDialog.svelte` (`<script lang="ts">`, Svelte 5 runes). Props:
   ```ts
   let { items, onSave, onDiscard, onCancel }: {
     items: { id: string; label: string }[];
     onSave: () => Promise<{ ok: true } | { ok: false; failedLabels: string[] }>;
     onDiscard: () => void;
     onCancel: () => void;
   } = $props();
   ```
   (Bentuk props boleh disesuaikan selama internal folder ini. Logika menyimpan sebaiknya di `index.ts`; komponen hanya tampilan + status `busy`/`error`.)
   - State: `busy = $state(false)`, `error = $state('')`, `overlay`/`primaryBtn` referensi elemen.
   - `const multi = $derived(items.length > 1)`.
   - ID unik untuk judul: `const titleId = 'confirm-close-title-' + Math.random().toString(36).slice(2)`; `aria-labelledby={titleId}` pada elemen dialog.
   - Judul: satu file `Simpan perubahan "<label>"?` / banyak `{items.length} file belum disimpan`. Teks bantu: "Perubahan akan hilang bila ditutup tanpa disimpan." Satu file: tampilkan label di judul/teks; banyak file: `<ul>` daftar label (`overflow-wrap: anywhere`, `max-height` + scroll).
   - Galat: `{#if error}<div class="err" role="alert">{error}</div>{/if}`.
   - Tombol (urutan: Batal, Buang/Buang semua, Simpan/Simpan semua sebagai utama): label `multi ? 'Simpan semua' : 'Simpan'`, `multi ? 'Buang semua' : 'Buang'`, `'Batal'`. Saat `busy`: Simpan menampilkan "Menyimpan..." dan `disabled`; Simpan dinonaktifkan selama `busy`. Batal dan Buang tetap aktif (sesuai kontrak, termasuk saat menyimpan tidak ada penguncian lain; bila `busy`, Batal/Buang tetap boleh dipilih dan hasilnya menang).
   - `async function save()`: bila `busy` return; `busy = true; error = ''`; `const r = await onSave()`; bila `!r.ok` set `error = 'Gagal menyimpan: ' + r.failedLabels.join(', ') + '. Pilih Batal untuk tetap membuka tab, atau Buang untuk menutup tanpa menyimpan.'`; `finally busy = false`. Bila `ok`, `onSave` sudah menyelesaikan promise di `index.ts`, komponen tidak perlu berbuat apa pun.
   - Keyboard: handler `onkeydown` di overlay: `Escape` -> `onCancel()` (preventDefault, stopPropagation agar tidak menutup hal lain); `Tab` -> focus trap sederhana (siklus di antara tombol-tombol dalam `.dlg`: ambil `querySelectorAll('button:not(:disabled)')`, Shift+Tab di pertama pindah ke terakhir dan sebaliknya).
   - Fokus awal: `$effect`/`onMount` -> `tick`/`Promise.resolve().then(() => primaryBtn?.focus())`; fokus ke tombol Simpan (utama). Hindari Enter menjalankan aksi destruktif: fokus awal ke Simpan, bukan Buang.
   - Klik latar: `onpointerdown={(e) => { if (e.target === e.currentTarget) onCancel(); }}`; tambahkan `<!-- svelte-ignore a11y_no_static_element_interactions -->` bila svelte-check meminta.
   - `<style>` meniru pola CloseDialog: hanya `var(--token, fallback)`, benar di tema gelap/terang. Tombol utama (Simpan) pakai `var(--accent, #3b82f6)` dengan teks `#fff`; Buang memakai `var(--danger, #e5484d)` sebagai border/teks (jangan latar penuh agar tidak mengundang klik tak sengaja). Cek token yang ada dengan `grep -rn "\-\-accent" frontend/src | head` dan pakai token yang benar-benar ada; fallback tetap disertakan.

3. Buat `index.ts`:
   ```ts
   import { mount, unmount } from 'svelte';
   import { getFileTab } from '../tabRegistry';
   import ConfirmCloseDialog from './ConfirmCloseDialog.svelte';

   // Hanya satu dialog sekali. Panggilan kedua saat dialog terbuka langsung `false` (tidak antre).
   let open = false;

   export function confirmCloseTabs(tabIds: string[], labels?: Record<string, string>): Promise<boolean> {
     const dirtyIds = tabIds.filter((id) => getFileTab(id)?.isDirty());
     if (dirtyIds.length === 0) return Promise.resolve(true);
     if (open) return Promise.resolve(false);
     open = true;
     const items = dirtyIds.map((id) => ({ id, label: labels?.[id] ?? id }));
     return new Promise<boolean>((resolve) => {
       let instance: ReturnType<typeof mount> | undefined;
       let settled = false;
       const finish = (result: boolean) => {
         if (settled) return;
         settled = true;
         if (instance) void unmount(instance);
         open = false;
         resolve(result);
       };
       const onSave = async () => { /* lihat di bawah */ };
       instance = mount(ConfirmCloseDialog, {
         target: document.body,
         props: { items, onSave, onDiscard: () => finish(true), onCancel: () => finish(false) },
       });
     });
   }
   ```
   `onSave`: loop berurutan (`for ... of items`, `await`, bukan `Promise.all`): `const h = getFileTab(item.id); const ok = h ? await h.save().catch(() => false) : true;` kumpulkan `failedLabels` untuk yang `false`. Henti setelah gagal pertama atau lanjutkan semua? Putuskan: lanjutkan menyimpan semua file, kumpulkan semua yang gagal (agar file yang bisa disimpan tetap tersimpan), lalu bila `failedLabels.length === 0` panggil `finish(true)` dan kembalikan `{ ok: true }`, selain itu `{ ok: false, failedLabels }` tanpa menyelesaikan promise. Pada percobaan Simpan berikutnya, file yang sudah bersih dianggap `true` oleh `save()` (handle mengembalikan true bila tidak kotor), jadi aman diulang.
   - Bungkus `mount` dalam try/catch: bila melempar, `open = false` lalu `resolve(false)`.
   - Jaga `finish` idempoten (Buang saat Simpan sedang berjalan lalu Simpan selesai tidak boleh resolve ulang; cek `settled` sebelum `finish(true)` dari `onSave`).
   - Komentar kode berbahasa Indonesia; jelaskan label (`labels` atau fallback id), semantik `true`/`false`, dan kebijakan satu dialog.

4. Build: `cd frontend && npm run build`. Jalankan juga `cd frontend && npm run check` bila bersih di baseline; pastikan tidak ada galat baru dari `confirmClose/`. Perbaiki hingga lulus.

5. Tinjau diri: grep memastikan tidak ada impor dari `../index`, layout store, atau `shellBridge` di folder ini; tidak ada `localStorage`/store global baru.

## Perubahan

- Baru: `frontend/src/modules/files/confirmClose/index.ts`
- Baru: `frontend/src/modules/files/confirmClose/ConfirmCloseDialog.svelte`
- Tidak ada file lain yang diubah.

## Wiring

Yang diekspos untuk langkah wiring (JANGAN disambungkan di sini): `confirmCloseTabs(tabIds, labels?)` dari `frontend/src/modules/files/confirmClose/index.ts`. Agent `unsaved-wiring` akan mengimpornya langsung dari path `files/confirmClose` di `PaneHeader.svelte` dan `PaneMenu.svelte`, selalu meneruskan `labels` (`Tab.title` atau basename `Tab.filePath`). Fitur ini tidak mengekspor apa pun dari `files/index.ts`.

## Selesai bila

- `confirmClose/index.ts` dan `ConfirmCloseDialog.svelte` ada; hanya path itu yang berubah (`git status` bersih di luar folder tersebut untuk pekerjaan Anda).
- Tanpa tab kotor: resolve `true` seketika tanpa UI. Satu file: Simpan / Buang / Batal; banyak file: judul berisi jumlah, daftar, "Simpan semua" / "Buang semua" / Batal.
- Simpan berurutan; sukses -> `true`; gagal -> dialog tetap terbuka dengan galat menyebut nama file, Batal/Buang aktif, Simpan nonaktif selama menyimpan. Buang -> `true`; Batal/Escape/latar -> `false`. Panggilan kedua saat terbuka -> `false`.
- Dialog `role="dialog"`, `aria-modal`, `aria-labelledby`, fokus awal, Escape, focus trap; dibersihkan dari DOM (`unmount`) setelah selesai.
- Guard: `cd frontend && npm run build` sukses (dan tidak ada galat baru di `npm run check` dari folder ini). Tanpa unit test.
