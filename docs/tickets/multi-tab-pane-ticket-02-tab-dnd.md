---
id: multi-tab-pane-ticket-02-tab-dnd
title: Drag tab, reorder, split, dan menu konteks tab
spec: docs/superpowers/specs/2026-11-22-multi-tab-pane-design.md
status: ready
wave: 2
depends_on: [multi-tab-pane-ticket-01-tab-container]
---
# Drag tab, reorder, split, dan menu konteks tab

## Tujuan

Setelah tiket 1, satu pane sudah bisa memuat banyak tab, tetapi tab hanya bisa dipilih dan ditutup. Epic ini menambah cara mengatur tab: memindahkan tab antar pane dengan drag, mengurutkan tab dalam satu pane, menjatuhkan tab di tepi pane agar menjadi pane baru (split), dan memisahkan tab lewat menu konteks tanpa drag (jalur klik/keyboard). Semua itu tidak boleh me-remount isi tab, sehingga terminal tidak restart dan scrollback tidak hilang.

## Hasil yang bisa dijalankan

Di atas hasil tiket 1, pengguna bisa:

- Menekan dan menggeser sebuah tab (ambang 4px) untuk memulai drag tab. Klik biasa tanpa geser tetap memilih tab.
- Menjatuhkan tab di tengah area isi pane lain: tab pindah menjadi tab di pane itu dan aktif.
- Menjatuhkan tab di tepi (atas/bawah/kiri/kanan) area isi pane mana pun: pane target terpecah dan tab menjadi pane baru.
- Menjatuhkan tab di deretan tab sebuah pane: tab disisipkan di posisi itu; di pane yang sama berarti reorder.
- Klik kanan tab: menu konteks dengan "Pisahkan ke pane baru", "Tutup tab", dan (khusus terminal) "Minimize".
- Bila pane asal menjadi kosong setelah tab dipindah, pane itu hilang dari layout.
- Terminal yang dipindah antar pane tetap hidup (tidak restart, scrollback utuh).
- Drag pane utuh lewat area kosong header tetap bekerja seperti sebelumnya.

Build frontend dan build aplikasi harus lulus dan aplikasi harus jalan.

## Batasan

In scope:

- `moveTab` sebagai fungsi murni di `model.ts` dan ekspornya lewat store layout.
- Perluasan `dnd.ts` dan `DropOverlay.svelte` untuk muatan tab.
- Komponen `TabContextMenu.svelte`.
- Penyambungan ke `PaneHeader.svelte` dan `LayoutArea.svelte`.
- Verifikasi bahwa host tab di-key id tab sehingga pindah pane tidak me-remount (kontrak dari tiket 1; di sini hanya diverifikasi dan diperbaiki bila ada celah di `LayoutArea.svelte`).

Out of scope:

- Mengubah bentuk model (`Tab`, `Leaf`, `Split`); hanya boleh menambah `moveTab`.
- Konfirmasi file belum disimpan (tiket 3). `moveTab` tidak pernah memanggil `confirmCloseTabs` karena tab tidak ditutup. Aksi "Tutup tab" di menu konteks memakai jalur tutup tab yang sama dengan tombol tutup di PaneHeader (apa pun yang ada saat tiket ini dikerjakan); tiket 3 yang menambahkan konfirmasi pada jalur itu.
- Tab "preview", perubahan model split, dukungan drag tab ke luar jendela.
- Unit test (tidak menambah vitest).
- Navigasi keyboard antar tab dan `role="tablist"` kecuali sudah disediakan tiket 1; menu konteks sendiri tetap harus bisa dioperasikan keyboard.

## Kontrak

Dipakai dari tiket 1 (tidak diubah bentuknya):

- Tipe `Tab` (id prefix `t_`, `kind`, field konten lain), `Leaf = { type:'pane', id, tabs: Tab[], activeTabId }`, `Split`, `LayoutDoc`, `DropZone = 'top'|'bottom'|'left'|'right'|'center'`.
- Helper murni `activeTab`, `allTabs`, `findTab`; operasi `activateTab`, `closeTab`, `closePane`, `setFocus`, `splitPane`, `movePane`.
- Host konten di `panes/hosts.ts` di-key id tab; tab non-aktif tetap ter-mount (`visible=false`); `releaseHost` hanya dipanggil saat tab ditutup, bukan saat dipindah.
- Store layout (`frontend/src/lib/stores/layout.ts`, tipe `LayoutStore`) yang mengekspos operasi model sebagai method.

Disediakan epic ini:

```ts
// frontend/src/modules/layout/model.ts
export function moveTab(
  doc: LayoutDoc,
  fromPaneId: string,
  tabId: string,
  targetPaneId: string,
  zone: DropZone,          // 'center' | 'top' | 'bottom' | 'left' | 'right'
  index?: number,          // hanya bermakna untuk zone 'center'; default: akhir
): LayoutDoc;

// frontend/src/lib/stores/layout.ts  (pada LayoutStore)
moveTab(fromPaneId: string, tabId: string, targetPaneId: string, zone: DropZone, index?: number): void;
```

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
// startPaneDrag, zoneAt, dragState tetap diekspor dengan perilaku lama.
```

Konvensi DOM untuk hit-test drag (ditambahkan di PaneHeader/LayoutArea, dibaca `dnd.ts`):

- Elemen pane punya `data-pane-id` (sudah ada, dipakai drag pane).
- Wadah deretan tab di PaneHeader punya `data-tab-strip` dan berada di dalam elemen `data-pane-id` pane itu.
- Tiap tab punya `data-tab-id`, dipakai menghitung index sisip (tengah horizontal tab: pointer di kiri tengah = sisip sebelum, kanan = sesudah).

```ts
// frontend/src/modules/layout/TabContextMenu.svelte
// props: { paneId: string; tabId: string; x: number; y: number; onClose: () => void }
```

Semantik `moveTab` (lihat fitur move-tab-model untuk rincian): pure, tidak mengubah doc masukan, mengembalikan doc baru; input tidak valid (pane/tab tidak ada, target tidak ada) mengembalikan doc yang sama tanpa melempar galat.

## Fitur

### move-tab-model: moveTab di model.ts

Deskripsi. Tambah `moveTab` di `model.ts` sebagai fungsi murni, tanpa mengubah fungsi lain, lalu ekspos lewat store layout (`layout.ts`) sebagai method `moveTab` dengan pola yang sama seperti `movePane` (menerapkan hasil ke doc, memicu autosave yang sudah ada). Perilaku:

1. Validasi: pane asal harus ada dan memuat `tabId`; pane target harus ada. Bila tidak, kembalikan `doc` apa adanya.
2. Zona `center`:
   - Target bukan pane asal: cabut tab dari pane asal, sisipkan di pane target pada `index` (di-clamp ke 0..jumlah tab; default akhir), set `activeTabId` target ke tab itu, fokus (`focusId`) ke pane target. Pane asal dihapus dari pohon bila tabnya habis; bila tidak habis dan tab yang dicabut tadi aktif, aktifkan tetangganya (aturan sama dengan `closeTab`).
   - Target = pane asal: reorder. Posisikan ulang tab ke `index` (hitung `index` terhadap daftar sebelum pencabutan; koreksi bila tab dicabut dari posisi sebelum `index`). Tab menjadi aktif. Tidak ada pane yang dihapus.
3. Zona tepi (`top|bottom|left|right`):
   - Target bukan pane asal: cabut tab dari asal (aturan penghapusan pane kosong di atas), lalu pecah pane target memakai mekanisme split yang sama dengan `movePane`/`splitPane` (pane baru berisi satu tab itu, ratio default sama seperti split yang ada, arah sesuai zona: top/left menaruh pane baru sebelum, bottom/right sesudah). Fokus ke pane baru, tab itu aktif.
   - Target = pane asal: hanya bila pane asal punya lebih dari satu tab; pane asal dipecah dan tab menjadi pane baru. Bila hanya satu tab, kembalikan `doc` apa adanya (tidak ada gunanya).
4. Penghapusan pane kosong harus memakai logika yang sama dengan `closePane` supaya pohon split tetap normal (Split dengan satu anak diciutkan), `focusId` dan `maximizedId` yang menunjuk pane terhapus dibereskan seperti `closePane`. Catatan: pada kasus pane asal kosong dan target dipecah, hati-hati bahwa target tidak boleh ikut hilang; hitung penghapusan dan pemecahan dengan urutan yang menjaga id target tetap valid (id pane target dan id tab dipertahankan; pane baru mendapat id baru).
5. Id tab tidak berubah saat dipindah (inilah yang membuat host tidak di-remount).
6. Invarian pasca-operasi: tiap Leaf punya `tabs` tidak kosong dan `activeTabId` valid.

Acceptance criteria:

- [ ] `moveTab` diekspor dari `model.ts`, murni, tanpa efek samping, tidak memutasi doc masukan.
- [ ] Center ke pane lain: tab pindah ke `index` (default akhir), aktif di pane target, pane asal dihapus bila kosong, pane asal menjaga `activeTabId` valid bila tidak kosong.
- [ ] Center ke pane sendiri dengan `index`: urutan tab berubah sesuai posisi sisip yang dimaksud, tidak ada tab hilang/ganda.
- [ ] Zona tepi ke pane lain: pane target terpecah, tab ada di pane baru, arah sesuai zona.
- [ ] Zona tepi ke pane sendiri: memecah hanya bila tab lebih dari satu; bila satu tab, doc tidak berubah.
- [ ] Id tab tidak berubah; pane target tidak pernah ikut terhapus.
- [ ] Masukan tak valid mengembalikan doc yang sama tanpa galat.
- [ ] `layout.ts` mengekspos `moveTab(fromPaneId, tabId, targetPaneId, zone, index?)` dan layout hasilnya tersimpan lewat autosave yang ada.
- [ ] `npm run build` frontend lulus.

ownedPaths: `frontend/src/modules/layout/model.ts`, `frontend/src/lib/stores/layout.ts`.

Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini. Fitur lain hanya memanggil `store.moveTab` sesuai tanda tangan di bagian Kontrak; selama tanda tangan itu dipatuhi, tidak perlu menunggu implementasinya selesai.

### tab-dnd-overlay: Drag tab dan DropOverlay

Deskripsi. Perluas `dnd.ts` dan `DropOverlay.svelte`:

- `DragState` mendapat `kind`, `tabId?`, dan `index`. `startPaneDrag` dipertahankan (kind `'pane'`, perilaku identik, termasuk tidak menjatuhkan ke pane sendiri). Perhatikan semua pemakai `DragState`/`dragState` yang ada (cari dengan grep) tetap kompatibel.
- `startTabDrag(e, paneId, tabId, title, store)`: pola sama dengan `startPaneDrag` (hanya tombol kiri, satu drag aktif sekaligus, ambang gerak 4px, pointer capture, ghost berisi judul tab, kelas `pane-dragging` di body, Escape/blur/pointercancel membatalkan, klik setelah drag ditelan supaya tidak memilih tab). Hindari duplikasi berlebihan: ekstrak bagian bersama bila praktis, tanpa mengubah perilaku drag pane.
- Hit-test saat bergerak (`elementFromPoint`):
  1. Bila pointer berada di dalam `[data-tab-strip]` sebuah pane (termasuk pane asal): target = pane itu, zona `center`, `index` dihitung dari posisi relatif terhadap elemen `[data-tab-id]` di strip itu (kiri tengah tab = sebelum, kanan = sesudah; di area kosong strip sesudah tab terakhir = akhir). `rect` = kotak strip, sehingga overlay menampilkan penanda posisi sisip.
  2. Bila tidak, dan pointer di dalam `[data-pane-id]`: target = pane itu (untuk drag tab, pane asal juga boleh jadi target), `zone = zoneAt(rect, x, y)`, `index = null`. Zona `center` di pane asal tidak berarti apa-apa (tab sudah di sana): perlakukan sebagai tanpa target. Zona tepi di pane asal hanya valid bila pane asal punya lebih dari satu tab; bila tidak, tanpa target.
  3. Selain itu tanpa target.
- Saat dilepas dengan target dan zona valid: panggil `store.moveTab(paneId, tabId, targetId, zone, index ?? undefined)`. Tanpa target: tidak melakukan apa-apa. Setelah drag yang dimulai (melewati 4px), klik berikutnya ditelan.
- `DropOverlay.svelte`: untuk `kind === 'tab'` dengan `index !== null`, gambar garis vertikal penanda sisip (posisi dihitung dari elemen tab di strip: tepi kiri tab pada `index`, atau tepi kanan tab terakhir bila `index` = jumlah tab) alih-alih kotak setengah pane. Untuk zona lain, perilaku kotak lama dipertahankan. Overlay tetap `pointer-events: none`, mengikuti dark/light lewat variabel CSS yang sudah dipakai, dan menghormati `prefers-reduced-motion`.

Acceptance criteria:

- [ ] Menekan dan menggeser tab kurang dari 4px lalu melepas tidak memulai drag; klik tetap memilih tab.
- [ ] Menggeser tab lebih dari 4px memulai drag tab: ghost dengan judul tab muncul, overlay tampil di pane/strip yang ditunjuk.
- [ ] Di area isi pane: tengah menampilkan kotak penuh, tepi menampilkan setengah pane di sisi itu; melepas memanggil `moveTab` dengan zona yang sesuai.
- [ ] Di deretan tab: tampil garis penanda sisip; melepas memanggil `moveTab(..., 'center', index)`; di pane yang sama berarti reorder.
- [ ] Zona `center` di pane asal (isi) dan zona tepi di pane asal yang hanya satu tab tidak menampilkan overlay dan tidak melakukan apa-apa saat dilepas.
- [ ] Escape, blur, dan pointercancel membatalkan drag tanpa efek; state dan kelas body dibersihkan.
- [ ] Drag pane utuh (`startPaneDrag`) berperilaku sama seperti sebelumnya.
- [ ] `npm run build` frontend lulus.

ownedPaths: `frontend/src/modules/layout/dnd.ts`, `frontend/src/modules/layout/DropOverlay.svelte`.

Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini. Bergantung hanya pada tanda tangan `store.moveTab` di Kontrak dan konvensi atribut `data-tab-strip`/`data-tab-id` yang dipasang fitur wiring.

### tab-context-menu: Menu konteks tab

Deskripsi. Buat `TabContextMenu.svelte` (props: `paneId`, `tabId`, `x`, `y`, `onClose`). Menu muncul di koordinat klik kanan (di-clamp supaya tidak keluar viewport), berisi:

- "Pisahkan ke pane baru": memanggil `store.moveTab(paneId, tabId, paneId, 'right')` lalu menutup menu. Dinonaktifkan (disabled) bila pane hanya punya satu tab (tidak ada yang bisa dipisah).
- "Tutup tab": memanggil jalur tutup tab yang sama dengan tombol tutup di PaneHeader (fungsi/handler yang sudah ada dari tiket 1; jangan menulis ulang logika `closeTab`, termasuk akhiri-sesi untuk terminal).
- "Minimize" (hanya tampil bila tab aktif/ditunjuk berkind terminal): memakai aksi minimize per tab dari tiket 1 (tab dilepas, sesi tetap hidup dan bisa dibuka dari sidebar).

Perilaku: tutup bila klik di luar, Escape, scroll, atau blur jendela; item bisa difokus dan dioperasikan dengan panah atas/bawah, Enter, dan Escape (`role="menu"`, `role="menuitem"`, fokus awal pada item pertama yang aktif); styling mengikuti variabel tema yang dipakai `PaneMenu.svelte` (cek komponen itu untuk gaya dan pola penutupan yang konsisten, tanpa mengubahnya). Komponen hanya menerima props dan memanggil store/handler; tidak mengetahui cara PaneHeader menyimpannya.

Acceptance criteria:

- [ ] Komponen ada, menerima props di Kontrak, dan tampil di (x, y) tanpa keluar viewport.
- [ ] "Pisahkan ke pane baru" memanggil `moveTab` zona `right` ke pane yang sama; disabled untuk pane satu tab.
- [ ] "Tutup tab" dan (untuk terminal) "Minimize" memanggil aksi tab yang sudah ada, tidak menduplikasi logikanya.
- [ ] Menu tertutup pada klik luar, Escape, scroll, blur, dan setelah memilih item.
- [ ] Dapat dioperasikan dengan keyboard; atribut ARIA menu terpasang; mengikuti tema terang/gelap.
- [ ] `npm run build` frontend lulus.

ownedPaths: `frontend/src/modules/layout/TabContextMenu.svelte`.

Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini. Tidak terlihat di aplikasi sampai fitur wiring menyambungkannya.

### tab-dnd-wiring: Wiring drag dan menu tab ke header dan area layout

Deskripsi. Langkah penutup fitur: sambungkan hasil tiga fitur di atas ke UI. Fitur ini bergantung pada ketiga fitur lain (kerjakan setelah ketiganya selesai).

- `PaneHeader.svelte`:
  - Pasang `data-tab-strip` pada wadah deretan tab dan `data-tab-id` pada tiap tab.
  - `onpointerdown` pada tab (tombol kiri, bukan pada tombol tutup) memanggil `startTabDrag(e, paneId, tab.id, judulTab)`. Pointerdown pada area kosong header tetap memanggil `startPaneDrag`. Pastikan pointerdown tab tidak ikut memicu drag pane (stopPropagation atau pemeriksaan target) dan klik biasa tetap menjalankan `activateTab` + fokus pane.
  - `oncontextmenu` pada tab: `preventDefault`, simpan `{ tabId, x, y }` ke state lokal, render `TabContextMenu` bila state ada, tutup lewat `onClose`.
  - Tidak mengubah tombol dan struktur header lain dari tiket 1; PaneHeader juga disentuh tiket 3 (belakangan), jadi batasi perubahan di sini pada wiring di atas dan rapikan agar mudah digabung.
- `LayoutArea.svelte`:
  - Pastikan `DropOverlay` terpasang sekali di level area layout dan elemen pane membawa `data-pane-id` (sudah ada untuk drag pane).
  - Verifikasi bahwa pemasangan host tab aktif ke slot pane dikunci id tab (`{#key}` pada id tab, bukan id pane), sehingga ketika tab pindah pane, host yang sama dilepas dari slot lama dan dipasang di slot baru tanpa `unmount`/`releaseHost`. Bila ada celah (mis. effect yang memanggil `releaseHost` atau `syncPanes` yang membuang host karena pane asal hilang sebelum pane tujuan menerimanya), perbaiki di berkas ini atau, jika sumbernya di `panes/hosts.ts`, laporkan sebagai temuan tanpa menyentuh berkas itu (di luar kepemilikan tiket ini) dan cari jalan lewat sisi `LayoutArea`.
  - Bila slot pane target berubah (pane asal dihapus, pane baru muncul), pastikan refit/visible terminal terpicu saat dipasang kembali.
- Verifikasi end-to-end secara manual di aplikasi: pindah tab file dan tab terminal antar pane, reorder, drop di tepi, menu konteks, pane asal kosong menghilang, restart aplikasi lalu layout hasil pemindahan terbaca (autosave).

Acceptance criteria:

- [ ] Drag tab dari header ke pane lain (tengah) memindahkan tab; ke tepi memecah pane; ke deretan tab menyisipkan; dalam pane sendiri mengurutkan.
- [ ] Klik kanan tab membuka `TabContextMenu`; "Pisahkan ke pane baru" memecah ke kanan.
- [ ] Klik biasa pada tab tetap memilih tab; drag area kosong header tetap memindahkan pane utuh.
- [ ] Memindahkan tab terminal antar pane tidak me-restart sesi: proses tetap berjalan, scrollback utuh, input tetap diterima setelah dipindah.
- [ ] Tab file yang dipindah tidak kehilangan perubahan yang belum disimpan (komponen tidak di-remount).
- [ ] Pane asal yang kosong hilang dari layout; fokus dan maximize tidak menunjuk pane yang sudah tidak ada.
- [ ] Layout setelah pemindahan bertahan setelah aplikasi dimulai ulang.
- [ ] `npm run build` frontend lulus dan build aplikasi lulus; aplikasi jalan.

ownedPaths: `frontend/src/modules/layout/PaneHeader.svelte`, `frontend/src/modules/layout/LayoutArea.svelte`.

Paralel: tidak. Fitur ini dikerjakan terakhir, setelah move-tab-model, tab-dnd-overlay, dan tab-context-menu selesai, karena menyambungkan ketiganya.

## Path yang dimiliki

Hanya path berikut yang boleh disentuh epic ini:

- `frontend/src/modules/layout/dnd.ts`
- `frontend/src/modules/layout/DropOverlay.svelte`
- `frontend/src/modules/layout/TabContextMenu.svelte`
- `frontend/src/modules/layout/PaneHeader.svelte`
- `frontend/src/modules/layout/LayoutArea.svelte`
- `frontend/src/modules/layout/model.ts`
- `frontend/src/lib/stores/layout.ts` (hanya menambah method `moveTab` pada store; ikut dimiliki lewat fitur move-tab-model)

Temuan di luar path ini (mis. `panes/hosts.ts`) dilaporkan, bukan diubah.

## Wiring

Tiket ini tidak bergantung pada tiket selain tiket 1 untuk bisa dijalankan. Langkah penutup (fitur tab-dnd-wiring):

1. `PaneHeader.svelte` memanggil `startTabDrag` dari pointerdown tab, memasang `data-tab-strip`/`data-tab-id`, dan merender `TabContextMenu` dari klik kanan tab.
2. `LayoutArea.svelte` memastikan `DropOverlay` aktif untuk drag tab, elemen pane membawa `data-pane-id`, dan host tab dikunci id tab sehingga pindah pane tidak me-remount.
3. `store.moveTab` (dari `layout.ts`) dipanggil oleh `dnd.ts` dan `TabContextMenu.svelte`.
4. Jalankan aplikasi dan uji alur di bagian Hasil yang bisa dijalankan, terutama bahwa terminal tidak restart setelah dipindah.

## Guard

- `cd frontend && npm run build` berhasil (termasuk `svelte-check`/tipe bila ada di skrip build).
- Build aplikasi berhasil (mis. `go build ./...` dari root dan/atau perintah build Wails/aplikasi yang dipakai proyek).
- Tanpa unit test (tidak menambah vitest).
