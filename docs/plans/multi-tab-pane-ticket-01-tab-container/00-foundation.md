# 00-foundation: tipe Tab/Leaf dan kerangka bersama

Baca dulu `overview.md` (kontrak bagian 2). Dikerjakan SATU agent sebelum fitur paralel. Tujuan: kontrak tipe tersedia agar fitur paralel punya sandaran. Build penuh BELUM perlu lulus (pemanggil lain belum diadaptasi).

Path yang boleh disentuh: `frontend/src/modules/layout/model.ts`, `frontend/src/modules/persist/hydrate.ts` (baru).

## Langkah

1. Baca `frontend/src/modules/layout/model.ts` penuh.
2. Definisikan `Tab` dan ubah `Leaf` sesuai overview 2.1 (pindahkan semua field konten ke `Tab`, tambah `id` `t_`). `NewLeaf` tidak berubah.
3. Ganti nama `newLeafId` menjadi `newPaneId` dan ekspor; tambah `newTabId` (prefix `t_`, algoritma sama). Ubah `makeLeaf` agar menghasilkan `Tab` (id dari `newTabId`). Ekspor `normalize`.
4. Tambah implementasi trivial `activeTab`, `allTabs`, `findTab` (overview 2.2). `leaves`, `findLeaf`, `findLeafBy` dipertahankan (tipe `Leaf` baru).
5. Tambah stub dengan signature kontrak untuk fungsi yang akan diisi `tab-model-foundation`: `activateTab`, `closeTab`, `renameTab` (stub boleh `return doc`). Hapus `renamePane`. Operasi lain yang tidak kompilasi karena tipe baru dibiarkan (diperbaiki `tab-model-foundation`); jangan memperbaiki perilaku di sini.
6. Pindahkan `hydrateNode` dan `hydrateDoc` dari `model.ts` ke `frontend/src/modules/persist/hydrate.ts` (impor `emptyDoc`, `clampRatio`, `normalize`, `newTabId`, tipe dari `../layout/model`). Versi awal: untuk leaf bentuk lama bungkus menjadi pane satu tab (id pane dipertahankan, `newTabId()`), untuk bentuk baru baca `tabs`/`activeTabId` secara dasar. Penyempurnaan (validasi, dedupe id tab, runtuh pane kosong) dikerjakan `tab-persist-migration`.
7. Di `model.ts` tambahkan `export { hydrateDoc } from '../persist/hydrate';` di tempat fungsi lama.
8. Verifikasi: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E "layout/model.ts|persist/hydrate.ts"` tidak menampilkan error di dua berkas itu yang berasal dari definisi tipe (error dari fungsi operasi lama di model.ts yang menunggu `tab-model-foundation` boleh tersisa; catat daftarnya di laporan).

## Selesai bila

- `Tab`, `Leaf` baru, `newPaneId`, `newTabId`, `makeLeaf`, `normalize`, `activeTab`, `allTabs`, `findTab` diekspor dengan signature kontrak.
- Stub `activateTab`, `closeTab`, `renameTab` ada; `renamePane` tidak ada.
- `hydrateDoc` ada di `persist/hydrate.ts` dan diekspor ulang dari `model.ts`.
