# Plan: Drag tab dan DropOverlay

## Metadata

- Tiket: `docs/tickets/multi-tab-pane-ticket-02-tab-dnd.md` (epic `multi-tab-pane-ticket-02-tab-dnd`)
- Fitur: `tab-dnd-overlay`
- ownedPaths: `frontend/src/modules/layout/dnd.ts`, `frontend/src/modules/layout/DropOverlay.svelte`
- Depends on foundation: ya. Foundation (`00-foundation.md`) menyediakan stub `moveTab` di `model.ts` dan `LayoutStore.moveTab` di `lib/stores/layout.ts`; fitur ini hanya memanggil `store.moveTab`. Bila stub belum ada di working tree, jangan menambahkannya (itu milik foundation/`move-tab-model`); laporkan.

Baca dulu (jangan diubah): `overview.md` bagian 2.3, 2.4, 2.6 dan catatan per fitur `tab-dnd-overlay`.

## Tujuan

Memperluas drag and drop layout agar membawa muatan tab selain pane: `startTabDrag` dengan hit-test deretan tab (sisip) dan isi pane (zona), serta `DropOverlay` yang menggambar garis vertikal sisip untuk drop di deretan tab. Perilaku `startPaneDrag` tidak boleh berubah.

## Path yang dimiliki

HANYA boleh disentuh:

- `frontend/src/modules/layout/dnd.ts`
- `frontend/src/modules/layout/DropOverlay.svelte`

Jangan sentuh `PaneHeader.svelte`, `LayoutArea.svelte` (wiring), `model.ts`, `lib/stores/layout.ts`, `TabContextMenu.svelte`, `panes/hosts.ts`, `groups/sessionDrag.ts`, atau berkas lain.

## Kontrak

Disediakan (bentuk persis di overview 2.3):

- `interface DragState { kind: 'pane' | 'tab'; paneId; tabId?; title; targetId; zone; index: number | null; rect }`
- `startTabDrag(e: PointerEvent, paneId: string, tabId: string, title: string, store: LayoutStore = layout): void`
- `startPaneDrag`, `zoneAt`, `dragState` tetap diekspor dengan perilaku lama (`startPaneDrag` mengisi `kind: 'pane'`, `index: null`).

Dipakai:

- `store.moveTab(fromPaneId, tabId, targetPaneId, zone, index?)` (overview 2.2; stub dari foundation).
- `findLeaf` dari `./model` dan `get` dari `svelte/store` untuk membaca jumlah tab pane asal: `findLeaf(get(store), paneId)?.tabs.length` (`LayoutStore` adalah store Svelte yang dapat di-`get`; cek tipe aktual).
- Konvensi DOM (overview 2.4), dipasang oleh wiring, dibaca di sini: `[data-pane-id]`, `[data-tab-strip]` (di dalam elemen pane), `[data-tab-id]` (di dalam strip). Hanya DOM yang menjadi sumber hit-test.

Pemakai lain `dragState` di repo (hasil grep): `DropOverlay.svelte` dan `modules/groups/sessionDrag.ts` (subscribe, membaca `st.paneId`). `sessionDrag.ts` di luar ownedPaths: jangan diubah; tambahan field `kind`/`tabId`/`index` bersifat aditif sehingga tetap kompatibel secara tipe. Catat di laporan sebagai temuan bahwa subscriber itu juga akan terpicu untuk drag `kind: 'tab'` (ia mencari leaf terminal dari `st.paneId`; pada model multi-tab dari tiket 01 pencariannya harus mengikuti tab) dan sebaiknya difilter/diadaptasi oleh pihak yang memilikinya.

## Langkah

Jalankan dulu `grep -rn "dragState\|DragState" frontend/src` untuk memastikan daftar pemakai masih sama, dan baca `dnd.ts`/`DropOverlay.svelte` aktual.

1. **Perluas `DragState`** di `dnd.ts`:
   ```ts
   export interface DragState {
     kind: 'pane' | 'tab';
     paneId: string;
     tabId?: string;
     title: string;
     targetId: string | null;
     zone: DropZone | null;
     index: number | null;
     rect: { x: number; y: number; w: number; h: number } | null;
   }
   ```
   Impor tambahan: `get` dari `svelte/store`, `findLeaf` dari `./model` (bersama `DropZone` yang sudah diimpor tipe-saja; pisahkan `import type` dan impor nilai).

2. **Ekstrak mesin drag bersama** (privat, tidak diekspor) dari badan `startPaneDrag`, tanpa mengubah perilakunya:
   ```ts
   interface DragSpec {
     base: Pick<DragState, 'kind' | 'paneId' | 'tabId' | 'title'>;
     resolve(x: number, y: number): Pick<DragState, 'targetId' | 'zone' | 'index' | 'rect'>;  // hit-test per jenis
     commit(target: DragState): void;                                                          // dipanggil saat lepas dengan target valid
   }
   function runDrag(e: PointerEvent, spec: DragSpec): void
   ```
   `runDrag` berisi semua yang sekarang ada di `startPaneDrag`: cek `e.button !== 0 || active`, `active = true`, `startX/startY/pointerId/source/prevUserSelect`, `started/ghost/cur/done`, `setCur`, `begin`, `onMove`, `swallowClick`, `onUp`, `onKey`, `onCancel`, `cleanup`, pendaftaran listener (pointermove, pointerup, pointercancel, keydown capture, blur, lostpointercapture). Perbedaan yang dijadikan parameter:
   - `begin()`: `cur = { ...spec.base, targetId: null, zone: null, index: null, rect: null }`.
   - `onMove`: setelah ghost diposisikan, `const r = spec.resolve(ev.clientX, ev.clientY); setCur({ ...spec.base, ...r });`.
   - `setCur`: tambahkan `cur.index === next.index` dan `cur.kind`/`tabId` tidak perlu dibandingkan (konstan selama drag) ke kondisi pembanding duplikat yang ada, supaya pergeseran index memperbarui overlay.
   - `onUp`: `if (target && target.targetId && target.zone) spec.commit(target);` (tetap hanya bila `wasStarted`; urutan `cleanup()` lalu commit lalu `swallowClick()` dipertahankan).
   - Ghost, kelas body `pane-dragging`, `userSelect`, ambang 4px, pointer capture, penanganan Escape/blur/pointercancel/lostpointercapture: identik dengan kode lama (salin apa adanya ke `runDrag`).

3. **Tulis ulang `startPaneDrag` sebagai pembungkus tipis** dengan signature lama `(e, paneId, title, store = layout)`:
   - `base = { kind: 'pane', paneId, title }`.
   - `resolve`: logika lama persis: `elementFromPoint` → `closest('[data-pane-id]')`; bila ada dan `dataset.paneId !== paneId` maka `{ targetId, zone: zoneAt(rect, x, y), index: null, rect }`, selain itu semua `null`.
   - `commit: (t) => store.movePane(paneId, t.targetId!, t.zone!)`.
   Pastikan perilaku drag pane tidak berubah (tidak menjatuhkan ke pane sendiri; tanpa pengenalan strip).

4. **Tambah `startTabDrag(e, paneId, tabId, title, store = layout)`** (diekspor):
   - `base = { kind: 'tab', paneId, tabId, title }`.
   - Helper `sourceTabCount(): number` = `findLeaf(get(store), paneId)?.tabs.length ?? 0`.
   - `resolve(x, y)` (hit-test `el = document.elementFromPoint(x, y)`):
     1. `const strip = el?.closest('[data-tab-strip]') as HTMLElement | null`. Bila ada: `pane = strip.closest('[data-pane-id]') as HTMLElement | null`; bila `pane?.dataset.paneId` tidak ada, anggap tanpa target. Kalau ada: `index = tabInsertIndex(strip, x)`, `rect` = `getBoundingClientRect()` strip (`{x:left, y:top, w:width, h:height}`), `zone: 'center'`, `targetId = pane.dataset.paneId`. Pane asal diperbolehkan (reorder).
     2. Bila tidak di strip: `pane = el?.closest('[data-pane-id]')`. Bila ada dan punya `dataset.paneId`: `rect` = rect pane, `zone = zoneAt(rect, x, y)`, `index: null`. Validasi pane asal (`targetId === paneId`): bila `zone === 'center'` maka tanpa target; bila zona tepi dan `sourceTabCount() <= 1` maka tanpa target. Pane lain: selalu valid.
     3. Selain itu: `{ targetId: null, zone: null, index: null, rect: null }`.
   - Fungsi privat `tabInsertIndex(strip: HTMLElement, x: number): number`: ambil `strip.querySelectorAll<HTMLElement>('[data-tab-id]')`; untuk tiap tab ke-`i` hitung `r = tab.getBoundingClientRect()`; bila `x < r.left + r.width / 2` kembalikan `i`; bila tidak ada yang memenuhi kembalikan jumlah tab (area kosong setelah tab terakhir atau kanan tengah tab terakhir). Index mengacu pada daftar tab SEBELUM pencabutan (konsisten dengan `moveTab`). Jangan mengecualikan tab yang sedang di-drag dari hitungan.
   - Tidak ada validasi no-op tambahan untuk reorder (sisip pada posisi semula dibiarkan; `moveTab` menanganinya).
   - `commit: (t) => { if (t.targetId && t.zone) store.moveTab(paneId, tabId, t.targetId, t.zone, t.index ?? undefined); }`. Validasi diulang saat lepas: panggil `resolve` terhadap state `cur` terakhir sudah cukup karena `cur` hanya memuat hasil `resolve` valid; namun untuk kasus tepi/center di pane asal pastikan `commit` juga menolak (guard ulang `sourceTabCount()` dan `targetId === paneId && zone === 'center' && index === null`) agar tidak ada panggilan sia-sia bila jumlah tab berubah di tengah drag.
   - Klik setelah drag yang sudah dimulai ditelan oleh `swallowClick` bersama (klik biasa < 4px tidak memulai drag, sehingga klik tetap memilih tab).

5. **Perbarui `DropOverlay.svelte`**:
   - Ganti `box` dengan turunan yang menghasilkan dua jenis penanda: `marker` (garis sisip) dan `box` (kotak lama).
   - Bila `s.kind === 'tab' && s.index !== null && s.rect && s.targetId`: hitung garis dari DOM (bukan dari `rect` saja): cari `strip` = `document.querySelector('[data-pane-id="<targetId>"] [data-tab-strip]')` (escape id dengan `CSS.escape`); `tabs = strip.querySelectorAll('[data-tab-id]')`. Bila `index < tabs.length`: `x = tabs[index].getBoundingClientRect().left`; bila `index >= tabs.length` dan ada tab: `x = tabs[tabs.length-1].getBoundingClientRect().right`; tinggi/posisi vertikal garis = rect strip (`s.rect.y`, `s.rect.h`). Bila strip/tab tidak ditemukan, jatuh ke `s.rect` dan gambar garis di `s.rect.x` agar tidak crash. Garis digambar `position: fixed`, lebar 2px, `left: x - 1px`, `top`, `height`, warna `var(--accent, #4f8cff)`, `pointer-events: none`, `z-index: 9999`.
   - Untuk kasus lain (`kind === 'pane'`, atau `kind === 'tab'` dengan `index === null`): perilaku kotak lama (switch zona `top|bottom|left|right|default`) tidak diubah.
   - Perhitungan dilakukan di `$derived.by` yang bergantung pada `$dragState`; karena state berubah tiap pergeseran dan `index` ikut tercatat di `setCur`, garis ikut diperbarui.
   - CSS: tambahkan kelas `.drop-marker` dengan `transition: left 80ms` dan di dalam `@media (prefers-reduced-motion: reduce)` set `transition: none` (bersama `.drop-zone`). Tema terang/gelap otomatis lewat variabel CSS `--accent` yang sudah dipakai; jangan menambah warna hardcode selain fallback seperti yang ada.

6. **Verifikasi tipe dan build path sendiri** (lihat Selesai bila). Tinjau ulang diff `startPaneDrag` terhadap versi lama untuk memastikan urutan operasi dan listener tidak berubah.

## Perubahan

- Diubah: `frontend/src/modules/layout/dnd.ts` (DragState, mesin drag bersama privat, `startPaneDrag` sebagai pembungkus, `startTabDrag`, helper `tabInsertIndex`).
- Diubah: `frontend/src/modules/layout/DropOverlay.svelte` (garis sisip untuk drag tab dengan index, kotak lama untuk selain itu, CSS reduced-motion).
- Tidak ada berkas dibuat/dihapus.

## Wiring

Yang diekspos untuk langkah `tab-dnd-wiring` (jangan disambungkan di sini):

- `startTabDrag(e, paneId, tabId, title)` dari `./dnd`, dipanggil dari `onpointerdown` tab di `PaneHeader.svelte` (abaikan tombol tutup/minimize, `stopPropagation`, tanpa `preventDefault`).
- Kebutuhan DOM: `data-tab-strip` pada wadah deretan tab (di dalam elemen `data-pane-id`) dan `data-tab-id` pada tiap tab di strip; `DropOverlay` dirender sekali di `LayoutArea.svelte`.
- `store.moveTab` dipanggil dari `dnd.ts` saat lepas.
- Temuan untuk dilaporkan: `modules/groups/sessionDrag.ts` ikut subscribe `dragState` dan akan menerima drag `kind: 'tab'` (lihat Kontrak).

## Selesai bila

- `DragState` punya `kind`, `tabId?`, `index`; `startTabDrag` diekspor dengan signature kontrak; `startPaneDrag`, `zoneAt`, `dragState` tetap diekspor dan perilaku drag pane identik.
- Hit-test: strip → center + index (+ rect strip); isi pane → `zoneAt`, pane asal tanpa target untuk center atau tepi bila tab tunggal; selain itu tanpa target. Lepas memanggil `store.moveTab(paneId, tabId, targetId, zone, index ?? undefined)`; Escape/blur/pointercancel membatalkan dan membersihkan state serta kelas body; klik setelah drag ditelan.
- `DropOverlay` menggambar garis vertikal untuk `kind === 'tab'` dengan `index !== null`, kotak lama untuk zona lain, `pointer-events: none`, tema dan reduced-motion dihormati.
- Guard (tanpa unit test): `cd frontend && npm run check 2>&1 | grep -E "layout/dnd.ts|layout/DropOverlay.svelte"` tidak menampilkan error; `cd frontend && npm run build` sukses untuk path fitur ini (error di berkas milik fitur paralel lain atau akibat tiket 01 yang belum merge dicatat, bukan diperbaiki).
- Hanya dua berkas ownedPaths yang berubah (`git status` memastikannya).
