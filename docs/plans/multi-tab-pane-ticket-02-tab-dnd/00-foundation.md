# 00-foundation: stub moveTab

Baca dulu `overview.md` (kontrak bagian 2.2). Dikerjakan SATU agent sebelum fitur paralel. Tujuan: signature `moveTab` tersedia di model dan store supaya `dnd.ts` (fitur `tab-dnd-overlay`) dan `TabContextMenu.svelte` (fitur `tab-context-menu`) bisa type-check tanpa menunggu `move-tab-model`.

Path yang boleh disentuh: `frontend/src/modules/layout/model.ts`, `frontend/src/lib/stores/layout.ts`. Prasyarat: tiket 01 sudah ter-merge (`Tab`, `Leaf.tabs`, `closeTab`, dst. ada).

## Langkah

1. Baca `frontend/src/modules/layout/model.ts` dan `frontend/src/lib/stores/layout.ts`. Lihat bentuk `movePane` (model dan store) sebagai pola.
2. Di `model.ts`, di dekat `movePane`, tambahkan stub (tanpa mengubah fungsi lain):

   ```ts
   export function moveTab(
     doc: LayoutDoc,
     fromPaneId: string,
     tabId: string,
     targetPaneId: string,
     zone: DropZone,
     index?: number,
   ): LayoutDoc {
     void fromPaneId; void tabId; void targetPaneId; void zone; void index;
     return doc; // stub: diimplementasikan fitur move-tab-model
   }
   ```
3. Di `lib/stores/layout.ts`:
   - Tambahkan ke interface `LayoutStore`: `moveTab(fromPaneId: string, tabId: string, targetPaneId: string, zone: DropZone, index?: number): void;`
   - Tambahkan ke objek hasil `createLayoutStore`: `moveTab: (from, tabId, target, zone, index) => update((d) => M.moveTab(d, from, tabId, target, zone, index)),` (sejajar dengan `movePane`).
4. Verifikasi: `cd frontend && npm run check 2>&1 | grep -E "layout/model.ts|stores/layout.ts"` tidak menampilkan error baru dari dua perubahan ini.

## Selesai bila

- `moveTab` diekspor dari `model.ts` dengan signature kontrak (stub mengembalikan `doc`).
- `LayoutStore.moveTab` ada di interface dan implementasi, memanggil `M.moveTab`.
- Tidak ada perubahan perilaku lain.
