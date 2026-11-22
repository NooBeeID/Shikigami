# Overview: multi-tab-pane-ticket-02-tab-dnd

Tiket: `docs/tickets/multi-tab-pane-ticket-02-tab-dnd.md`
Spec: `docs/superpowers/specs/2026-11-22-multi-tab-pane-design.md`
Plan tiket sebelumnya (kontrak yang dipakai): `docs/plans/multi-tab-pane-ticket-01-tab-container/overview.md`

Semua perubahan frontend-only (Svelte 5 + TypeScript) di `frontend/`. Tanpa perubahan daemon/IPC/Go. Tanpa unit test (jangan tambah vitest).

Catatan prasyarat: tiket 01 (wave 1) HARUS sudah di-merge sebelum epic ini dikerjakan. Pada saat plan ini ditulis, kode di repo masih bentuk lama (`Leaf` berisi field konten langsung); semua rujukan ke `Tab`, `Leaf.tabs`, `activeTabId`, `activateTab`, `closeTab`, `allTabs`, `findTab`, `store.closeTab`, `PaneKindSpec.onEndSession(tab)` mengikuti kontrak tiket 01 (overview tiket 01 bagian 2). Agent WAJIB membaca kode aktual di working tree dulu; bila nama aktual berbeda tipis dari kontrak tiket 01, ikuti kode aktual dan catat di laporan.

## 1. Ringkasan epic dan urutan eksekusi

Setelah tiket 01, satu pane memuat banyak tab, tetapi tab hanya bisa dipilih dan ditutup. Epic ini menambah: drag tab antar pane, reorder dalam satu pane, drop di tepi pane (split), sisip di deretan tab, dan menu konteks tab ("Pisahkan ke pane baru", "Tutup tab", "Minimize"). Tab yang dipindah tidak boleh di-remount (id tab dipertahankan; host di-key id tab dari tiket 01), sehingga terminal tidak restart.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`, satu agent, sekuensial, kecil): stub `moveTab` di `model.ts` dan `lib/stores/layout.ts` dengan signature kontrak, supaya `dnd.ts` dan `TabContextMenu.svelte` bisa type-check tanpa menunggu implementasi.
2. **Fitur paralel** (setelah foundation): `move-tab-model`, `tab-dnd-overlay`, `tab-context-menu`. ownedPaths terpisah.
3. **Wiring** (`tab-dnd-wiring`, terakhir, setelah ketiganya selesai): sambungkan ke `PaneHeader.svelte` dan `LayoutArea.svelte`, verifikasi host tidak di-remount, build guard, verifikasi manual.

## 2. Kontrak bersama

### 2.1 Dari tiket 01 (tidak diubah bentuknya)

- `Tab` (id prefix `t_`), `Leaf = { type:'pane', id, tabs: Tab[], activeTabId }`, `Split`, `LayoutDoc`, `DropZone = 'top'|'bottom'|'left'|'right'|'center'`.
- Helper murni di `model.ts`: `activeTab`, `allTabs`, `findTab`, `findLeaf`, `leaves`, `newPaneId`, `newTabId`, `normalize`.
- Operasi model: `activateTab`, `closeTab`, `closePane`, `setFocus`, `splitPane`, `movePane`, `openPane`, dll.
- Store `LayoutStore` (`frontend/src/lib/stores/layout.ts`) dengan `activateTab(paneId, tabId)`, `closeTab(paneId, tabId)`, `closePane`, `movePane`, `setFocus`, `subscribe`, `paneCount`, `leaves`, dll.
- Host konten (`panes/hosts.ts`) di-key id tab; tab non-aktif tetap ter-mount (`visible=false`); `releaseHost` hanya saat tab ditutup, bukan saat dipindah.

### 2.2 `moveTab` (disediakan epic ini)

```ts
// frontend/src/modules/layout/model.ts  (implementasi: move-tab-model; stub: foundation)
export function moveTab(
  doc: LayoutDoc,
  fromPaneId: string,
  tabId: string,
  targetPaneId: string,
  zone: DropZone,          // 'center' | 'top' | 'bottom' | 'left' | 'right'
  index?: number,          // hanya untuk zone 'center'; default: akhir
): LayoutDoc;

// frontend/src/lib/stores/layout.ts  (pada interface LayoutStore dan createLayoutStore)
moveTab(fromPaneId: string, tabId: string, targetPaneId: string, zone: DropZone, index?: number): void;
```

Semantik ringkas (rincian di tiket bagian fitur `move-tab-model`): murni, tidak memutasi masukan; input tak valid mengembalikan `doc` yang sama tanpa melempar; id tab dan id pane target dipertahankan; pane asal kosong dihapus dengan logika `closePane`; invarian tiap Leaf `tabs` tidak kosong dan `activeTabId` valid. Store: `moveTab: (from, tabId, target, zone, index) => update((d) => M.moveTab(d, from, tabId, target, zone, index))` (pola `movePane`; autosave yang ada otomatis jalan karena berlangganan store).

`moveTab` tidak pernah memanggil `confirmCloseTabs` (tiket 03) dan tidak memanggil `releaseHost`.

### 2.3 `dnd.ts`

```ts
// frontend/src/modules/layout/dnd.ts
export interface DragState {
  kind: 'pane' | 'tab';
  paneId: string;            // pane asal
  tabId?: string;            // hanya kind 'tab'
  title: string;
  targetId: string | null;   // pane target
  zone: DropZone | null;
  index: number | null;      // posisi sisip di deretan tab; null bila bukan drop di deretan tab
  rect: { x: number; y: number; w: number; h: number } | null;
}
export function startTabDrag(e: PointerEvent, paneId: string, tabId: string, title: string, store?: LayoutStore): void;
// startPaneDrag, zoneAt, dragState tetap diekspor dengan perilaku lama (startPaneDrag mengisi kind: 'pane', index: null).
```

Pemakai `DragState`/`dragState` yang ada: hanya `dnd.ts` dan `DropOverlay.svelte` (diverifikasi dengan grep; agent `tab-dnd-overlay` tetap menjalankan `grep -rn "dragState\|DragState" frontend/src` ulang).

### 2.4 Konvensi DOM untuk hit-test drag

Dipasang oleh `tab-dnd-wiring` di `PaneHeader`/`LayoutArea`, dibaca oleh `dnd.ts` dan `DropOverlay.svelte`:

- Elemen pane punya `data-pane-id` (sudah ada).
- Wadah deretan tab di PaneHeader punya atribut `data-tab-strip` dan berada di dalam elemen `data-pane-id` pane itu (header berada di dalam elemen pane; verifikasi di `LayoutArea`).
- Tiap tab punya `data-tab-id="<tab.id>"` (tiket 01 sudah memasang `data-tab-id`; wiring memverifikasi dan menambah bila belum) dan berada di dalam `[data-tab-strip]`.
- Index sisip: pointer di kiri tengah tab = sisip sebelum tab itu, kanan tengah = sesudah; area kosong strip setelah tab terakhir = jumlah tab. Index dihitung terhadap daftar tab SEBELUM pencabutan (konsisten dengan `moveTab`).
- Penanda sisip di `DropOverlay`: garis vertikal di tepi kiri `[data-tab-id]` ke-`index` (dalam strip target), atau tepi kanan tab terakhir bila `index` = jumlah tab. Strip kosong (tidak mungkin, pane selalu punya tab) tidak perlu ditangani.

### 2.5 `TabContextMenu.svelte`

```ts
// frontend/src/modules/layout/TabContextMenu.svelte
// props:
interface Props {
  paneId: string;
  tabId: string;
  x: number;
  y: number;
  onClose: () => void;            // tutup menu (dipanggil setelah item dipilih, klik luar, Escape, scroll, blur)
  onCloseTab: () => void;         // TAMBAHAN epic ini (opsional di kontrak tiket, wajib di sini): jalur tutup tab yang sama dengan tombol tutup di PaneHeader
  onMinimizeTab?: () => void;     // TAMBAHAN: aksi minimize per tab dari PaneHeader; item "Minimize" hanya dirender bila prop ini ada
}
```

Keputusan desain (tiket tidak menentukan cara menjangkau handler yang hidup di PaneHeader): menu tidak menduplikasi logika tutup/minimize dan tidak mengimpor registry sendiri. `PaneHeader` sudah punya `closeTabAction(tab)` (tutup layout + akhiri sesi terminal) dan `minimizeTab(tab)` dari tiket 01; wiring meneruskannya sebagai `onCloseTab`/`onMinimizeTab`. Dengan begitu tiket 03 yang menambah konfirmasi pada `closeTabAction` otomatis berlaku untuk menu. Props asli kontrak (`paneId, tabId, x, y, onClose`) tetap persis; dua prop tambahan hanya menambah.

Perilaku menu:

- "Pisahkan ke pane baru": `store.moveTab(paneId, tabId, paneId, 'right')` lalu `onClose()`. Disabled bila pane hanya punya satu tab (baca jumlah tab dari store `layout` lewat `get(layout)` + `findLeaf`, atau `$layout`).
- "Tutup tab": `onCloseTab()` lalu `onClose()`.
- "Minimize": hanya bila `onMinimizeTab` ada; `onMinimizeTab()` lalu `onClose()`.
- Posisi di-clamp ke viewport (ukur setelah mount seperti `PaneMenu.place()`), `role="menu"`, item `role="menuitem"` (`<button>`), fokus awal item aktif pertama, panah atas/bawah (lewati item disabled), Enter (aktifkan, default button), Escape tutup; tutup pada pointerdown di luar, scroll (capture), blur window. Gaya dan pola penutupan mengikuti `PaneMenu.svelte` (baca, jangan ubah); gunakan variabel tema yang sama (`--bg-elev`, `--border`, `--fg`, `--accent`, dst. sesuai yang dipakai `PaneMenu`).

### 2.6 Konvensi

- Prefix id: pane `p_`, tab `t_`. Kode berbahasa Inggris, komentar boleh Indonesia (gaya repo).
- Fungsi model murni; tidak memutasi masukan.
- Svelte 5 runes (`$props`, `$state`, `$derived`, `$effect`), mengikuti gaya komponen layout yang ada.
- Klik biasa pada tab (gerak < 4px) tetap `activateTab` + fokus pane; setelah drag yang dimulai, klik berikutnya ditelan.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `move-tab-model` | `moveTab` murni di `model.ts` (aturan center/tepi, pane asal kosong, reorder, invarian) dan method `moveTab` di store layout. Mengganti stub foundation dengan implementasi penuh. Tidak mengubah fungsi lain. | `frontend/src/modules/layout/model.ts`, `frontend/src/lib/stores/layout.ts` |
| `tab-dnd-overlay` | `DragState` baru, `startTabDrag` (hit-test strip vs isi pane, index sisip, validasi pane asal), ekstraksi bagian bersama dengan `startPaneDrag` tanpa mengubah perilakunya, `DropOverlay` garis sisip untuk `kind==='tab'` dengan `index !== null`. | `frontend/src/modules/layout/dnd.ts`, `frontend/src/modules/layout/DropOverlay.svelte` |
| `tab-context-menu` | Komponen `TabContextMenu.svelte` sesuai 2.5 (props, item, keyboard, ARIA, penutupan, tema). | `frontend/src/modules/layout/TabContextMenu.svelte` |
| `tab-dnd-wiring` | DIKERJAKAN TERAKHIR. Pasang `data-tab-strip`/`data-tab-id`, `onpointerdown` tab ke `startTabDrag`, `oncontextmenu` + render `TabContextMenu` di `PaneHeader`; verifikasi/perbaiki `LayoutArea` (DropOverlay terpasang sekali, `data-pane-id`, host di-key id tab tanpa remount, refit terminal saat slot berubah); build guard; verifikasi manual. | `frontend/src/modules/layout/PaneHeader.svelte`, `frontend/src/modules/layout/LayoutArea.svelte` |

Detail dan acceptance criteria tiap fitur ada di bagian Fitur tiket. Bila ada selisih kecil, kontrak di dokumen ini yang menang.

Catatan per fitur:

- `move-tab-model`: urutan hitung untuk kasus pane asal kosong + target dipecah: cabut tab (hapus pane asal via logika `closePane`) lebih dulu, baru pecah pane target yang id-nya tetap valid; pane baru berisi tab yang sama (id tab tidak berubah) dengan id pane baru dari `newPaneId()`. Zona tepi pada target = pane asal dengan >1 tab: cabut tab dari asal, lalu pecah pane asal itu dengan pane baru berisi tab tersebut (top/left = pane baru sebelum, bottom/right = sesudah). Pakai ulang mekanisme split `movePane`/`splitPane` (jangan menulis ulang `splitNode`; bila perlu ekstrak helper privat di `model.ts` TANPA mengubah perilaku fungsi lain). `activeTabId` pasca cabut mengikuti aturan `closeTab` (utamakan tetangga kanan, lalu kiri). `focusId` = pane tujuan (target pada center, pane baru pada split); `maximizedId` yang menunjuk pane terhapus dibereskan seperti `closePane`.
- `tab-dnd-overlay`: sumber kebenaran hit-test hanya DOM (`elementFromPoint`, `closest`). Untuk menentukan "pane asal punya lebih dari satu tab" baca store: `get(store).` lalu `findLeaf(doc, paneId)?.tabs.length` (impor `get` dari `svelte/store`, `findLeaf` dari `./model`). Validasi dilakukan saat bergerak (overlay tidak tampil untuk target tidak valid) dan diulang saat dilepas. `startPaneDrag` tetap tidak boleh menjatuhkan ke pane sendiri.
- `tab-context-menu`: tidak mengimpor `PaneHeader`; hanya `layout` store, `findLeaf`, dan props.
- `tab-dnd-wiring`: bila celah ada di `panes/hosts.ts` (di luar ownedPaths), laporkan sebagai temuan dan cari jalan lewat sisi `LayoutArea`; jangan edit `hosts.ts`. Batasi perubahan `PaneHeader` pada wiring saja (tiket 03 menyentuhnya sesudahnya); tulis rapi agar mudah digabung.

## 4. Foundation

Ada (`hasFoundation = true`), lihat `00-foundation.md`. Isi: stub `moveTab` dengan signature kontrak di `model.ts` dan `lib/stores/layout.ts` (interface + implementasi stub pemanggil model). Foundation menyentuh dua berkas yang kemudian dimiliki `move-tab-model`; setelah foundation selesai kepemilikan kembali ke fitur itu. Tidak ada tipe lain yang dibuat di foundation (`DragState` dimiliki penuh `tab-dnd-overlay`).

## 5. Wiring checklist (dibaca agent `tab-dnd-wiring`)

Jalankan setelah `move-tab-model`, `tab-dnd-overlay`, dan `tab-context-menu` selesai.

1. `PaneHeader.svelte`:
   - Wadah deretan tab (`role="tablist"` dari tiket 01) diberi `data-tab-strip`. Tiap tab (`role="tab"`) memiliki `data-tab-id={t.id}`.
   - `onpointerdown` pada elemen tab: abaikan bila `e.button !== 0` atau target berada di dalam tombol (tutup/minimize: `(e.target as HTMLElement).closest('button')`); selain itu panggil `startTabDrag(e, pane.id, t.id, t.title)` (impor dari `./dnd`). Panggil `e.stopPropagation()` agar tidak naik ke handler `startPaneDrag` di wadah header. Jangan `preventDefault` (klik harus tetap memilih tab dan fokus tombol/tab tetap normal).
   - Klik biasa tetap `store.activateTab(pane.id, t.id)` (+ fokus pane) lewat handler `onclick` yang ada; `startTabDrag` menelan klik hanya setelah drag benar-benar dimulai.
   - Pointerdown pada area kosong header tetap memanggil `startPaneDrag` (jangan diubah).
   - `oncontextmenu` pada tab: `e.preventDefault()`, simpan `ctxMenu = { tabId: t.id, x: e.clientX, y: e.clientY }` di `$state` lokal. Render `{#if ctxMenu}<TabContextMenu paneId={pane.id} tabId={ctxMenu.tabId} x={ctxMenu.x} y={ctxMenu.y} onClose={() => (ctxMenu = null)} onCloseTab={() => closeTabAction(tabById)} onMinimizeTab={isTermTab(tabById) ? () => minimizeTab(tabById) : undefined} />{/if}`. `closeTabAction`/`minimizeTab`/`isTermTab` adalah fungsi yang SUDAH ada di `PaneHeader` dari tiket 01 (nama persis dari plan tiket 01 `tab-header-ui`; cek nama aktual di kode). Cari tab dengan `pane.tabs.find(...)`; bila tidak ditemukan tutup menu.
   - Tidak mengubah tombol dan struktur header lain; jangan menyentuh logika dirty/konfirmasi (tiket 03).
2. `LayoutArea.svelte`:
   - `DropOverlay` dirender tepat sekali di level area layout (jangan duplikasi), elemen pane membawa `data-pane-id`, dan header berada di dalam elemen itu supaya `closest('[data-pane-id]')` dari strip menemukan pane yang benar.
   - Verifikasi pemasangan host tab aktif ke slot pane dikunci id tab (`{#key}` pada id tab, bukan id pane). Cek tidak ada `$effect`/`syncPanes` yang memanggil `releaseHost` ketika tab pindah pane (tab tetap ada di `allTabs` doc baru, jadi harus tidak dilepas). Perbaiki di `LayoutArea` bila ada celah; bila sumber di `panes/hosts.ts`, laporkan sebagai temuan tanpa menyentuhnya.
   - Pastikan refit/visible terminal terpicu ketika host dipasang ke slot pane baru (mis. `visible` ikut slot aktif; trigger refit setelah attach, sama seperti mekanisme yang dipakai saat pergantian tab aktif di tiket 01).
3. `store.moveTab` dipanggil oleh `dnd.ts` dan `TabContextMenu.svelte` (tidak ada wiring tambahan; cukup pastikan impor `layout` store yang sama dengan yang dipakai `LayoutArea`).
4. Jalankan build guard (bagian 6) sampai lulus.
5. Verifikasi manual (catat di pesan commit bila dijalankan): drag tab file ke tengah pane lain; drag ke tepi (4 sisi); drag ke deretan tab (sisip) dan dalam pane sendiri (reorder); drop di tengah isi pane asal = tidak terjadi apa-apa; pane asal kosong hilang dan `focus`/`maximize` valid; tab terminal dipindah (proses jalan, scrollback utuh, input diterima); tab file kotor tetap kotor setelah dipindah; klik kanan tab (Pisahkan, Tutup, Minimize); klik biasa memilih tab; drag area kosong header memindah pane utuh; restart aplikasi dan layout tetap.

## 6. Build guard

Satu-satunya guard (dari root repo `/Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal`):

- `cd frontend && npm run check` (svelte-check; error tipe nol)
- `cd frontend && npm run build` (vite build)
- `go build ./...` (dan/atau perintah build aplikasi proyek bila ada, mis. wails/Makefile; cek `Makefile`/README)

Selama fase paralel, agent fitur cukup memastikan tidak ada error baru di ownedPaths-nya (filter output `npm run check` ke path sendiri). Build penuh lulus diwajibkan di `tab-dnd-wiring`.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya. Jangan menyentuh path fitur lain, `PaneHeader.svelte`/`LayoutArea.svelte` (milik wiring), `panes/hosts.ts`, `PaneMenu.svelte` (baca saja), `modules/files/` (tiket 03), atau berkas lain di luar daftar epic ini.
- Fase paralel hanya `move-tab-model`, `tab-dnd-overlay`, `tab-context-menu`. `tab-dnd-wiring` dijalankan sendirian sesudahnya.
- Jangan mengubah bentuk model (`Tab`, `Leaf`, `Split`); hanya `moveTab` yang boleh ditambah di `model.ts`. Jangan mengubah perilaku fungsi lain di `model.ts`/`layout.ts`.
- Bila butuh perubahan di berkas milik fitur lain, ikuti kontrak apa adanya dan catat di laporan; jangan edit. Temuan di luar path epic (mis. `panes/hosts.ts`) dilaporkan, bukan diubah.
- Jangan menambah vitest/unit test. Jangan commit sebelum diminta alur `implement-feature`; jangan pkill pola luas.
