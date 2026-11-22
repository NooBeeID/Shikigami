# Plan: moveTab di model.ts dan store layout

## Metadata

- Tiket: `docs/tickets/multi-tab-pane-ticket-02-tab-dnd.md`
- Fitur: `move-tab-model` (moveTab di model.ts dan store layout)
- ownedPaths: `frontend/src/modules/layout/model.ts`, `frontend/src/lib/stores/layout.ts`
- Depends on foundation: ya (`00-foundation.md` sudah menaruh stub `moveTab` di kedua berkas; plan ini menggantikan stub model dengan implementasi penuh dan memastikan method store benar)
- Prasyarat: tiket 01 (tab container) sudah ter-merge. Kontrak bersama ada di `overview.md` bagian 2.1 dan 2.2; jangan diulang di sini.

## Tujuan

Implementasi penuh fungsi murni `moveTab(doc, fromPaneId, tabId, targetPaneId, zone, index?)` di `model.ts` (menggantikan stub foundation) dan method `moveTab` di `LayoutStore`. Mencakup: center ke pane lain (sisip di index), center ke pane sendiri (reorder), zona tepi (split), penghapusan pane asal yang kosong, serta penjagaan invarian (`tabs` tidak kosong, `activeTabId` valid, id tab tidak berubah, id pane target tetap valid).

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `frontend/src/modules/layout/model.ts`
- `frontend/src/lib/stores/layout.ts`

Jangan sentuh `dnd.ts`, `DropOverlay.svelte`, `TabContextMenu.svelte`, `PaneHeader.svelte`, `LayoutArea.svelte`, `panes/hosts.ts`, atau berkas lain. Jangan mengubah bentuk tipe (`Tab`, `Leaf`, `Split`, `LayoutDoc`) dan jangan mengubah perilaku fungsi lain di kedua berkas. Jangan tambah vitest/unit test. Jangan commit.

## Kontrak

Disediakan (signature persis, lihat overview 2.2):

```ts
// model.ts
export function moveTab(
  doc: LayoutDoc,
  fromPaneId: string,
  tabId: string,
  targetPaneId: string,
  zone: DropZone,
  index?: number,
): LayoutDoc;

// lib/stores/layout.ts (interface LayoutStore + createLayoutStore)
moveTab(fromPaneId: string, tabId: string, targetPaneId: string, zone: DropZone, index?: number): void;
```

Dipakai dari kode yang sudah ada (hasil tiket 01; baca kode aktual dulu, bila nama berbeda tipis ikuti kode aktual dan catat di laporan): tipe `Tab`, `Leaf` (`tabs`, `activeTabId`), `DropZone`, `LayoutDoc`; fungsi `findLeaf`, `pathOf`, `replaceAt`, `closePane`, `splitPane` (menerima `Leaf` utuh berid sebagai pane baru), `restore`, `normalize`, `newPaneId()` (pembuat id pane `p_`; di kode pra-tiket-01 bernama `newLeafId`, pakai nama yang ada di kode aktual). Semantik tambahan yang wajib dipatuhi: murni (tidak memutasi masukan), input tak valid mengembalikan `doc` yang SAMA (referensi sama, tidak melempar), tidak memanggil `confirmCloseTabs`, `releaseHost`, atau efek samping lain.

## Langkah

1. Baca `frontend/src/modules/layout/model.ts` dan `frontend/src/lib/stores/layout.ts` (kode aktual pasca tiket 01). Konfirmasi: stub `moveTab` dari foundation ada; bentuk `closeTab` (aturan `activeTabId` tetangga: utamakan kanan, lalu kiri); perilaku `closePane`, `movePane`, `splitPane`, `replaceAt` (harus immutabel). Konfirmasi nama pembuat id pane.

2. Di `model.ts`, tambahkan dua helper privat (tidak diekspor) tepat di atas `moveTab`:

   ```ts
   // Ganti satu leaf di pohon dengan hasil fn; doc lain tidak berubah.
   function updateLeaf(doc: LayoutDoc, paneId: string, fn: (l: Leaf) => Leaf): LayoutDoc;

   // activeTabId setelah tab `tabId` dicabut dari `tabs` (daftar SEBELUM pencabutan).
   // Bila tab yang dicabut bukan yang aktif: activeTabId lama dipertahankan.
   // Bila aktif: tetangga kanan, lalu kiri (aturan yang sama dengan closeTab).
   function activeAfterRemoval(tabs: Tab[], activeTabId: string, tabId: string): string;
   ```

   `updateLeaf`: `const path = pathOf(doc, paneId); const l = findLeaf(doc, paneId); if (path === null || !l) return doc; return { ...doc, root: replaceAt(doc.root, path, fn(l)) };`. Bila `closeTab` sudah punya helper setara untuk aturan tetangga, ekstrak-pakai ulang HANYA bila tanpa mengubah perilaku `closeTab`; kalau ragu, tulis `activeAfterRemoval` terpisah (jangan edit `closeTab`).

3. Ganti isi stub `moveTab` dengan implementasi berikut (urutan logika wajib dipatuhi):

   a. Validasi. `const src = findLeaf(doc, fromPaneId); const tab = src?.tabs.find((t) => t.id === tabId); const tgt = findLeaf(doc, targetPaneId);` Bila `!src || !tab || !tgt` kembalikan `doc`.

   b. Zona `center`, `fromPaneId === targetPaneId` (reorder):
      - `from = src.tabs.findIndex(t => t.id === tabId)`; `raw = index ?? src.tabs.length`; `raw` di-clamp ke `0..src.tabs.length` (index dihitung terhadap daftar SEBELUM pencabutan; NaN/non-integer: bulatkan ke bawah, NaN dianggap akhir).
      - `ins = raw > from ? raw - 1 : raw` (koreksi karena tab dicabut dari posisi sebelum index). Susun `rest = tabs tanpa tab`, lalu `next = [...rest.slice(0, ins), tab, ...rest.slice(ins)]`.
      - `updateLeaf(doc, fromPaneId, l => ({ ...l, tabs: next, activeTabId: tab.id }))`, lalu `focusId = fromPaneId`; bungkus dengan `normalize`. Tidak ada pane dihapus; `maximizedId` tidak diubah.

   c. Zona `center`, pane berbeda:
      - Cabut dari pane asal lebih dulu. Bila `src.tabs.length === 1`: `d = closePane(doc, fromPaneId)` (otomatis membereskan `focusId`, `maximizedId`, dan menciutkan Split). Bila lebih dari satu: `d = updateLeaf(doc, fromPaneId, l => ({ ...l, tabs: l.tabs.filter(t => t.id !== tabId), activeTabId: activeAfterRemoval(l.tabs, l.activeTabId, tabId) }))`.
      - Sisipkan di target pada `ins = clamp(index ?? tgt.tabs.length, 0, tgt.tabs.length)` (floor, NaN = akhir): `d = updateLeaf(d, targetPaneId, l => ({ ...l, tabs: [...l.tabs.slice(0, ins), tab, ...l.tabs.slice(ins)], activeTabId: tab.id }))`. Id pane target tidak berubah oleh `closePane`, jadi tetap valid.
      - `return normalize({ ...d, focusId: targetPaneId })`.

   d. Zona tepi (`top|bottom|left|right`):
      - Bila `fromPaneId === targetPaneId && src.tabs.length <= 1` kembalikan `doc` (tidak ada gunanya; jangan pernah memecah pane menjadi dirinya sendiri).
      - `let d = doc.maximizedId ? restore(doc) : doc` (pola `movePane`).
      - Cabut tab dari asal TANPA menyentuh target: satu tab -> `d = closePane(d, fromPaneId)` (hanya mungkin bila asal != target, sudah dijamin); lebih dari satu -> `updateLeaf` seperti langkah c. Target (termasuk bila target = asal dengan >1 tab) tetap ada di pohon sehingga id-nya valid untuk dipecah.
      - Buat pane baru: `const fresh: Leaf = { type: 'pane', id: newPaneId(), tabs: [tab], activeTabId: tab.id };`
      - `dir = zone === 'top' || zone === 'bottom' ? 'v' : 'h'`; `side = zone === 'top' || zone === 'left' ? 'before' : 'after'` (sama persis dengan `movePane`). `const res = splitPane(d, targetPaneId, dir, fresh, side);` (mekanisme split dipakai ulang; jangan menulis ulang `splitNode`/`replaceAt` untuk split).
      - `return normalize({ ...res.doc, focusId: fresh.id });`

   e. Zona selain lima nilai `DropZone` (nilai liar saat runtime): kembalikan `doc`.

4. Pastikan hasil memenuhi invarian: tiap Leaf yang disentuh punya `tabs` tidak kosong dan `activeTabId` ada di `tabs`; objek `tab` dipakai ulang (id tidak berubah, tidak di-clone dengan id baru); doc masukan tidak dimutasi (selalu spread/filter/slice, tidak `push`/`splice`).

5. Di `lib/stores/layout.ts`: pastikan `LayoutStore` punya `moveTab(fromPaneId: string, tabId: string, targetPaneId: string, zone: DropZone, index?: number): void;` dan `createLayoutStore` punya `moveTab: (from, tabId, target, zone, index) => update((d) => M.moveTab(d, from, tabId, target, zone, index)),` sejajar dengan `movePane`. Bila foundation sudah menambahkannya, cukup verifikasi (jangan duplikasi). Autosave yang ada otomatis jalan lewat langganan store; jangan menambah mekanisme simpan.

6. Verifikasi (tanpa unit test): dari `frontend`, jalankan `npm run check 2>&1 | grep -E "layout/model.ts|stores/layout.ts"` sehingga tidak ada error dari dua berkas ini. Telusuri kasus ini secara mental/dengan membaca kode: (i) center pane lain, asal 1 tab -> asal hilang, fokus target; (ii) center pane lain, asal 3 tab, tab aktif di tengah -> aktif tetangga kanan; (iii) reorder `index` = jumlah tab memindah ke akhir, `index` = posisi sendiri atau sesudahnya tidak mengubah urutan; (iv) tepi ke pane lain dengan asal 1 tab -> asal hilang dulu, target dipecah, id target utuh; (v) tepi ke pane sendiri, 1 tab -> doc sama; 2+ tab -> terpecah; (vi) input tak valid -> `doc` sama.

## Perubahan

- `frontend/src/modules/layout/model.ts`: ubah (stub `moveTab` diganti implementasi; tambah helper privat `updateLeaf` dan `activeAfterRemoval`). Tidak ada fungsi lain yang diubah.
- `frontend/src/lib/stores/layout.ts`: ubah hanya bila perlu (verifikasi/lengkapi method `moveTab` di interface dan implementasi).

## Wiring

Yang diekspos untuk langkah wiring dan fitur lain (JANGAN disambungkan di sini):

- `moveTab(doc, fromPaneId, tabId, targetPaneId, zone, index?)` dari `model.ts`.
- `store.moveTab(fromPaneId, tabId, targetPaneId, zone, index?)` pada `LayoutStore`, dipanggil nanti oleh `dnd.ts` (fitur `tab-dnd-overlay`) dan `TabContextMenu.svelte` (fitur `tab-context-menu`, aksi "Pisahkan ke pane baru" = `moveTab(paneId, tabId, paneId, 'right')`). Tidak ada penyambungan ke `PaneHeader.svelte`/`LayoutArea.svelte` di fitur ini.

## Selesai bila

- `moveTab` di `model.ts` adalah implementasi penuh (bukan stub), diekspor dengan signature kontrak, murni dan tidak memutasi masukan.
- Semua perilaku langkah 3 terpenuhi: center pane lain (index clamp, default akhir, aktif, fokus target, asal kosong dihapus via `closePane`, `activeTabId` asal valid), reorder dengan koreksi index, tepi (top/left sebelum, bottom/right sesudah, fokus pane baru, urutan cabut lalu pecah), tepi pada pane asal hanya bila tab > 1, input tak valid mengembalikan `doc` yang sama.
- Id tab tidak berubah; invarian `tabs` tidak kosong dan `activeTabId` valid; `focusId`/`maximizedId` tidak menunjuk pane yang sudah hilang.
- `LayoutStore.moveTab` ada di interface dan implementasi, memakai pola `movePane`.
- Tidak ada perubahan pada fungsi lain, tipe, atau berkas di luar ownedPaths.
- Guard: build path fitur ini sukses (`npm run check` tanpa error pada `layout/model.ts` dan `stores/layout.ts`; `cd frontend && npm run build` tidak gagal karena dua berkas ini). Tanpa unit test.
