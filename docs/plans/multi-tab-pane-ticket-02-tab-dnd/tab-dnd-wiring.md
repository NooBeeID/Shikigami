# Plan: Wiring drag dan menu tab ke header dan area layout

## Metadata

- Tiket: `multi-tab-pane-ticket-02-tab-dnd` (`docs/tickets/multi-tab-pane-ticket-02-tab-dnd.md`)
- Fitur: `tab-dnd-wiring` (dikerjakan TERAKHIR, setelah `move-tab-model`, `tab-dnd-overlay`, `tab-context-menu` selesai dan ter-merge)
- ownedPaths: `frontend/src/modules/layout/PaneHeader.svelte`, `frontend/src/modules/layout/LayoutArea.svelte`
- Depends on foundation: ya (`00-foundation.md`, stub `moveTab`; pada saat fitur ini jalan sudah diganti implementasi penuh oleh `move-tab-model`)
- Rujukan: `overview.md` (kontrak 2.2 sampai 2.6, wiring checklist bagian 5, build guard bagian 6). Spec: `docs/superpowers/specs/2026-11-22-multi-tab-pane-design.md`.

## Tujuan

Menyambungkan tiga fitur paralel ke UI: tab di header bisa di-drag (`startTabDrag`), area kosong header tetap menggeser pane utuh, klik kanan tab membuka `TabContextMenu`, dan `LayoutArea` memastikan host tab (terminal/file) tidak di-remount saat tab pindah pane, dengan refit/visible terminal terpicu di slot baru. Lalu build guard dan verifikasi manual end-to-end, termasuk persistensi setelah restart.

## Path yang dimiliki

HANYA boleh menyentuh:

- `frontend/src/modules/layout/PaneHeader.svelte`
- `frontend/src/modules/layout/LayoutArea.svelte`

Jangan menyentuh: `dnd.ts`, `DropOverlay.svelte`, `TabContextMenu.svelte`, `model.ts`, `lib/stores/layout.ts`, `panes/hosts.ts`, `PaneMenu.svelte`, `modules/files/`, `modules/terminal/`, `overview.md`. Celah di luar path (mis. `panes/hosts.ts`) hanya DILAPORKAN, tidak diubah. Jangan menambah vitest. Jangan commit kecuali alur `implement-feature` meminta. Jangan `pkill` pola luas (matikan hanya PID yang Anda luncurkan sendiri).

## Kontrak

Dipakai (disediakan fitur lain, jangan diubah):

- `startTabDrag(e: PointerEvent, paneId: string, tabId: string, title: string, store?: LayoutStore): void` dari `./dnd` (overview 2.3). Menelan klik setelah drag benar-benar dimulai; klik biasa (gerak < 4px) tidak ditelan.
- `TabContextMenu.svelte` dengan props `{ paneId, tabId, x, y, onClose, onCloseTab, onMinimizeTab? }` (overview 2.5). Item "Minimize" hanya muncul bila `onMinimizeTab` diberikan.
- `store.moveTab(fromPaneId, tabId, targetPaneId, zone, index?)` (overview 2.2); dipanggil `dnd.ts` dan menu, bukan kode fitur ini.
- Konvensi DOM (overview 2.4): elemen pane `data-pane-id` (sudah ada di `LayoutArea`), wadah tab `data-tab-strip` di dalam elemen pane, tiap tab `data-tab-id` di dalam strip.
- Dari tiket 01 (baca kode AKTUAL di working tree dulu; bila nama beda tipis, ikuti kode aktual dan catat di laporan): `PaneHeader` punya `pane.tabs`, `closeTabAction(t: Tab)`, `minimizeTab(t: Tab)`, `isTermTab(t: Tab)`, `<div role="tablist">` dengan `<div role="tab" data-tab-id>`, `onHeaderDown` dengan guard `closest('button, input, .pane-menu, [role=tab]')`. `LayoutArea` punya `syncPanes(allTabs(doc), visibleTabIds)`, action `hostSlot(node, { tab, paneId, visible })`, `{#key at.id}` membungkus `.pane-slot`, dan `{#key n.id}` membungkus pane. `hosts.ts`: `ensureHost(tab, paneId, visible)`, `attachHost(tabId, slot)`, `detachHost(tabId)`, `releaseHost(tabId)`, `syncPanes(entries, visibleTabIds)`.

Disediakan: tidak ada API baru. Hanya perilaku UI.

## Langkah

Prasyarat: pastikan tiket 01 dan tiga fitur paralel sudah ada di working tree (`grep -n "export function moveTab" frontend/src/modules/layout/model.ts` harus berisi implementasi, bukan stub `void fromPaneId`; `ls frontend/src/modules/layout/TabContextMenu.svelte`; `grep -n "export function startTabDrag" frontend/src/modules/layout/dnd.ts`). Bila ada yang belum ada, berhenti dan laporkan `ok:false` dengan alasan. Baca penuh `PaneHeader.svelte`, `LayoutArea.svelte`, `dnd.ts`, `TabContextMenu.svelte` (baca saja) sebelum mengedit.

### A. `PaneHeader.svelte`

1. Impor: `import { startTabDrag } from './dnd';` (gabungkan dengan impor `startPaneDrag` yang sudah ada) dan `import TabContextMenu from './TabContextMenu.svelte';`.
2. State lokal: `let ctxMenu = $state<{ tabId: string; x: number; y: number } | null>(null);`
3. Handler tab (tulis berdekatan, mudah digabung dengan tiket 03):
   ```ts
   function onTabDown(e: PointerEvent, t: Tab) {
     if (e.button !== 0 || editing) return;
     if ((e.target as HTMLElement).closest('button, input')) return; // tombol tutup/minimize tab
     e.stopPropagation(); // jangan naik ke onHeaderDown (drag pane)
     startTabDrag(e, pane.id, t.id, t.title, store);
   }
   function onTabContext(e: MouseEvent, t: Tab) {
     e.preventDefault();
     e.stopPropagation();
     ctxMenu = { tabId: t.id, x: e.clientX, y: e.clientY };
   }
   ```
   Jangan `preventDefault` pada pointerdown (klik tetap memilih tab; fokus normal). `Tab` diimpor sebagai tipe dari `./model` bila belum.
4. Markup: pada wadah `role="tablist"` tambahkan atribut `data-tab-strip`. Pada tiap elemen `role="tab"` pastikan `data-tab-id={t.id}` ada (tiket 01 sudah memasang; tambah bila belum), lalu tambahkan `onpointerdown={(e) => onTabDown(e, t)}` dan `oncontextmenu={(e) => onTabContext(e, t)}`. Pertahankan `onclick` yang ada (`selectTab`/`activateTab` + fokus pane) dan handler keyboard.
5. `onHeaderDown`: biarkan guard `[role=tab]` apa adanya (tab ditangani `onTabDown` dengan stopPropagation; guard tetap sebagai jaring pengaman). Area kosong header (spacer, nomor, ruang kosong tablist, jarak antar tab) tetap memanggil `startPaneDrag`. Jangan ubah logikanya. Catatan: bila `onTabDown` memanggil `stopPropagation`, `onHeaderDown` memang tidak terpanggil; jangan menghapus guard.
6. Render menu di akhir markup header (sejajar dengan blok `PaneMenu`):
   ```svelte
   {#if ctxMenu}
     {@const ct = pane.tabs.find((x) => x.id === ctxMenu!.tabId)}
     {#if ct}
       <TabContextMenu
         paneId={pane.id}
         tabId={ct.id}
         x={ctxMenu.x}
         y={ctxMenu.y}
         onClose={() => (ctxMenu = null)}
         onCloseTab={() => closeTabAction(ct)}
         onMinimizeTab={isTermTab(ct) ? () => minimizeTab(ct) : undefined}
       />
     {:else}
       {void (ctxMenu = null)}
     {/if}
   {/if}
   ```
   Bila `{@const}` di dalam `{#if}` bermasalah di versi Svelte yang dipakai, ganti dengan `$derived` (`const ctxTab = $derived(ctxMenu ? pane.tabs.find(...) : undefined)`) dan `$effect` yang menutup menu bila `ctxMenu && !ctxTab`. Hindari efek samping langsung di template bila svelte-check protes.
7. Jangan mengubah tombol header, struktur, gaya, logika dirty/konfirmasi (tiket 03), `closeTabAction`, `minimizeTab`. Tambahan gaya hanya bila perlu: tab bisa `cursor: grab` (opsional, jangan merusak gaya lama).
8. Hindari sisa kelas pane-dragging yang membuat menu/ghost tertutup; tidak ada perubahan di sini.

### B. `LayoutArea.svelte`

1. Verifikasi `DropOverlay`: `grep -c "<DropOverlay" LayoutArea.svelte` harus 1 (satu-satunya, di dalam `.layout-area`, di luar `{#if doc.root}`). Jangan duplikasi. Juga `grep -rn "<DropOverlay" frontend/src` harus hanya satu hasil.
2. Verifikasi elemen pane membawa `data-pane-id={n.id}` dan `PaneHeader` (jadi `[data-tab-strip]`) berada DI DALAM elemen itu (sudah demikian di kode lama; periksa tidak berubah oleh tiket 01).
3. Verifikasi pemasangan host:
   - `{#key at.id}` (id TAB aktif) membungkus `.pane-slot` dengan `use:hostSlot`. Bila tiket 01 mengikat key ke id pane saja, perbaiki ke id tab.
   - Action `hostSlot` harus: (a) `destroy()` hanya `detachHost(id)` bila `getHost(id)?.parentElement === node` (agar slot lama yang hancur belakangan tidak melepas host yang sudah dipasang ke slot baru); (b) tidak memanggil `releaseHost`; (c) punya `update(np)` yang `ensureHost` + `attachHost` ulang bila `tab`/`paneId`/`visible` berubah. Tambahkan bagian yang kurang.
   - `$effect.pre(() => syncPanes(...))` tidak boleh melepas host tab yang masih ada di `allTabs(doc)` doc baru. Karena tab yang dipindah tetap ada di doc baru, `releaseHost` tidak boleh terpanggil. Tidak ada `$effect`/`onDestroy` lain di `LayoutArea` yang memanggil `releaseHost`: `grep -n releaseHost LayoutArea.svelte` harus kosong (hanya `hosts.ts` yang boleh memanggil lewat `syncPanes`).
   - Skenario kritis: tab dari pane A dipindah ke pane B dan pane A hilang. Pastikan urutan tidak membuang host: `syncPanes` dijalankan dengan `allTabs(doc)` yang sudah memuat tab di pane B, jadi tidak di-release. Slot A hancur (destroy tidak melepas karena `parentElement` sudah slot B atau host masih di A lalu dipasang B oleh `update`/action baru). Slot B baru dibuat (`{#key at.id}` atau `{#key n.id}` pane baru) dan `attachHost` memasang host tab itu.
4. Temuan di `panes/hosts.ts` (jangan edit, laporkan): periksa `ensureHost`/`updateEntry` memperbarui `paneId` pada entry yang sudah ada (komponen pane memakai `paneId` untuk `renameTab(paneId, tabId, ...)`). Bila `paneId` entry basi setelah pindah pane, catat sebagai temuan dan jelaskan dampaknya (judul terminal tidak ter-rename atau salah pane). Cari jalan lewat sisi `LayoutArea` (mis. memanggil `ensureHost(tab, paneId, visible)` dengan `paneId` baru dari action `update`, yang sudah dilakukan butir 3c) tanpa menyentuh `hosts.ts`.
5. Refit/visible saat slot berubah: tab yang pindah mengubah `paneId`/slot dan `visible` (tab aktif di pane baru = `visible: true`; ia aktif karena `moveTab` mengaktifkannya). Pastikan:
   - `visibleTabIds` memuat id tab aktif tiap pane (atau hanya tab aktif pane yang di-maximize), sehingga `visible` benar di slot baru.
   - Setelah `attachHost` ke slot baru, picu refit tanpa mengubah `hosts.ts` atau `TerminalTab`: di `hostSlot` (setelah `attachHost` pada mount dan pada `update`), jadwalkan `requestAnimationFrame(() => requestAnimationFrame(() => window.dispatchEvent(new Event('resize'))))` dengan guard agar tidak menumpuk (batalkan rAF sebelumnya pada `destroy`). Ini pengaman tambahan di atas jalur utama (`TerminalTab` `applyVisible(true)` memanggil `safeFit` dan `ResizeObserver` memanggil refit saat host dipasang). Baca `modules/terminal/TerminalTab.svelte` (baca saja) untuk memastikan jalur utama memang ada dan catat di laporan. Jangan mencuri fokus.
6. Jangan mengubah `closeAndEnd`, `onClose`, `onMinimize`, `onRename`, dan handler `PaneHeader` lain di `LayoutArea`, kecuali perbaikan di butir 3 sampai 5 menuntutnya.

### C. Build guard

Dari root repo `/Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal`:

1. `cd frontend && npm run check` (nol error tipe; error di luar ownedPaths yang berasal dari fitur lain dilaporkan dan, bila memblokir, perbaiki hanya yang ada di path Anda).
2. `cd frontend && npm run build`
3. `go build ./...` (dan perintah build aplikasi bila ada di `Makefile`/README).

Ulangi sampai lulus.

### D. Verifikasi manual end-to-end

Jalankan aplikasi (skill `run` atau perintah dev proyek; matikan hanya proses yang Anda luncurkan). Catat hasil tiap butir di laporan:

1. Siapkan dua pane, masing-masing beberapa tab (campur terminal dan file). Drag tab file ke tengah isi pane lain: tab pindah, aktif, pane asal hilang bila kosong.
2. Drag tab ke tepi (kiri/kanan/atas/bawah) isi pane: pane terpecah, tab di pane baru, arah sesuai.
3. Drag tab ke deretan tab pane lain: sisip di posisi garis penanda. Dalam pane sendiri: reorder.
4. Drop di tengah isi pane asal, atau tepi pane asal satu tab: tidak terjadi apa-apa, overlay tidak tampil.
5. Pane asal kosong hilang; `focus` dan `maximize` valid (maximize pane lalu pindahkan tab terakhirnya: tidak ada layar kosong).
6. Tab terminal dipindah: proses tetap jalan (mis. `sleep`/`top`), scrollback utuh, input diterima, ukuran terminal pas (refit).
7. Tab file kotor dipindah: tetap kotor, isi belum tersimpan tidak hilang.
8. Klik kanan tab: "Pisahkan ke pane baru" (memecah ke kanan; disabled untuk pane satu tab), "Tutup tab", "Minimize" (hanya terminal). Menu tertutup pada klik luar/Escape/scroll.
9. Klik biasa tab memilih tab dan fokus pane. Drag area kosong header tetap memindah pane utuh.
10. Restart aplikasi: layout hasil pemindahan terbaca sama (autosave).

## Perubahan

- Diubah: `frontend/src/modules/layout/PaneHeader.svelte` (impor `startTabDrag` dan `TabContextMenu`, state `ctxMenu`, `onTabDown`, `onTabContext`, atribut `data-tab-strip`/`data-tab-id`, `onpointerdown`/`oncontextmenu` pada tab, render menu).
- Diubah (hanya bila verifikasi menemukan celah): `frontend/src/modules/layout/LayoutArea.svelte` (perbaikan `{#key}`/`hostSlot` destroy-update, pengaman refit). Bila tidak ada celah, tidak diubah dan dilaporkan "terverifikasi".
- Tidak ada berkas baru.

## Wiring

Fitur ini ADALAH langkah wiring; tidak ada yang diekspos untuk disambungkan kemudian. Catatan untuk tiket 03 (menyentuh `PaneHeader` sesudahnya): `closeTabAction` tetap satu-satunya jalur tutup tab sehingga konfirmasi dirty yang ditambahkan di sana otomatis berlaku untuk menu konteks (lewat `onCloseTab`). Tulis perubahan `PaneHeader` terkumpul (handler tab berdekatan, blok menu di akhir) agar mudah digabung.

## Selesai bila

- Guard: `npm run check`, `npm run build` frontend, dan `go build ./...` lulus (tanpa unit test).
- `data-tab-strip` ada pada wadah tab dan `data-tab-id` pada tiap tab; pointerdown tab memanggil `startTabDrag` dengan `stopPropagation`; area kosong header tetap `startPaneDrag`; klik kanan tab membuka `TabContextMenu` dengan `onCloseTab`/`onMinimizeTab` dari handler `PaneHeader` yang ada.
- `DropOverlay` terpasang tepat sekali; `data-pane-id` ada; host di-key id tab; tidak ada `releaseHost` atau remount saat tab pindah pane; refit/visible terminal terpicu di slot baru.
- Temuan di `panes/hosts.ts` (bila ada) dilaporkan, `hosts.ts` tidak diubah.
- Semua butir verifikasi manual D lulus dan hasilnya dicatat, termasuk persistensi setelah restart.
- Tidak ada berkas di luar ownedPaths yang berubah (`git status` hanya menunjukkan dua berkas ini untuk fitur ini).
