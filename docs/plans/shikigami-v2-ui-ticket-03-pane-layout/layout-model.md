# Plan: Foundation: model pohon layout, store, dan aksi

## Metadata

- Tiket: `shikigami-v2-ui-ticket-03-pane-layout` (`docs/tickets/shikigami-v2-ui-ticket-03-pane-layout.md`)
- Fitur: `layout-model` (Foundation: model pohon layout, store, dan aksi)
- Spec: `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md` (bagian 5)
- Kontrak sumber kebenaran: `docs/plans/shikigami-v2-ui-ticket-03-pane-layout/overview.md` bagian 2 dan 4 (baca dulu; plan ini hanya urutan kerja dan keputusan detail)
- ownedPaths:
  - `frontend/src/modules/layout/model.ts`
  - `frontend/src/modules/layout/registry.ts`
  - `frontend/src/lib/stores/layout.ts`
- Stub antarmuka (pengecualian, dibuat fitur ini lalu ditimpa pemilik fiturnya): `frontend/src/modules/layout/panes/hosts.ts`, `frontend/src/modules/layout/PaneHeader.svelte`, `frontend/src/modules/layout/dnd.ts`, `frontend/src/modules/layout/DropOverlay.svelte`
- Depends on foundation: tidak (fitur ini ADALAH foundation; dikerjakan dan digabung paling awal)

## Tujuan

Menyediakan fondasi tanpa UI untuk epic layout multi-pane: tipe pohon layout, fungsi murni aksi atas `LayoutDoc` (tidak memutasi masukan, keluaran JSON murni), registri jenis pane, dan store Svelte pembungkus yang group-ready. Selain itu membuat stub antarmuka dengan signature final supaya tiga fitur paralel (`layout-renderer`, `pane-header-dnd`, `pane-kinds`) masing-masing bisa lolos build tanpa menunggu yang lain. Tidak ada IPC, tidak ada dependency npm baru, tidak ada unit test.

## Path yang dimiliki

HANYA berkas berikut yang boleh dibuat/diubah:

- `frontend/src/modules/layout/model.ts`
- `frontend/src/modules/layout/registry.ts`
- `frontend/src/lib/stores/layout.ts`
- Stub (dibuat sekali, nanti ditimpa pemilik fitur): `frontend/src/modules/layout/panes/hosts.ts`, `frontend/src/modules/layout/PaneHeader.svelte`, `frontend/src/modules/layout/dnd.ts`, `frontend/src/modules/layout/DropOverlay.svelte`

Dilarang menyentuh: `LayoutArea.svelte`, `Splitter.svelte`, `shortcuts.ts`, `PaneMenu.svelte`, `panes/terminal.ts`, `panes/file.ts`, `panes/*.svelte`, `modules/layout/index.ts` (jangan dibuat), `lib/stores/index.ts`, `lib/stores/tabs.ts`, `lib/stores/sessions.ts`, `lib/registry.ts`, `lib/ipc.ts`, `main.ts`, `App.svelte`, `modules/shell/`, `modules/tabs/`, `modules/terminal/`, `modules/files/`, `package.json`, `Makefile`, kode Go, dan `overview.md`.

## Kontrak

Disediakan (bentuk persis di overview; jangan mengubah nama atau signature):

- `model.ts` (overview 2.2 dan 2.3): tipe `PaneKind`, `Leaf` (dengan `workspaceId?`), `Split`, `Node`, `LayoutDoc`, `DropZone`, `SplitPath`, `NewLeaf`; konstanta/fungsi `emptyDoc`, `clampRatio`, `leaves`, `findLeaf`, `findLeafBy`, `pathOf`, `openPane`, `splitPane`, `closePane`, `movePane`, `swapPanes`, `resize`, `maximize`, `restore`, `tidyGrid`, `setFocus`, `renamePane`, `hydrateDoc`.
- `registry.ts` (overview 2.5): `PaneKindSpec`, `registerPaneKind`, `paneKinds()`; tambahan helper non-reaktif `getPaneKind(kind)`.
- `lib/stores/layout.ts` (overview 2.4): `LayoutStore`, `createLayoutStore(initial?)`, `layout`.
- Stub (overview 2.6, 2.7, bagian 4): `ensureHost/getHost/attachHost/detachHost/releaseHost/syncPanes`, props `PaneHeader`, `DragState/dragState/zoneAt/startPaneDrag`, `DropOverlay` tanpa props.

Dipakai: hanya `svelte/store` (`writable`, `derived`, `get`, tipe `Readable`) dan tipe `Component` dari `svelte`. Pola `registry.ts` mengikuti `frontend/src/lib/registry.ts` (baca sebagai contoh, jangan diubah). Tidak ada pesan IPC: `model.ts` dan `lib/stores/layout.ts` dilarang mengimpor `lib/ipc`, memanggil `call(`, atau mengimpor `lib/stores/sessions`/`workspaces`.

Keputusan detail yang dikunci di sini (agar konsisten dengan pemakai):

- `SplitPath`: `''` = root, `'a'`, `'b'`, `'a.b'`. `pathOf(doc, id)` mengembalikan jalur ke leaf (`''` bila leaf adalah root, `null` bila tidak ada). `resize` menerima jalur ke simpul Split; jalur yang menunjuk leaf atau tidak ada diabaikan (doc dikembalikan apa adanya).
- Id leaf: `p_` + 8 heks pertama dari `crypto.randomUUID()` (tanpa tanda `-`); fallback penghitung modul bila `crypto.randomUUID` tidak ada. Id dibuat sekali dan tidak pernah diganti saat pindah.
- Fokus saat `closePane`: bila pane yang ditutup adalah `focusId` (atau `focusId` tidak valid sesudah penutupan), fokus pindah ke leaf pertama (urutan baca) dari subtree saudara; bila bukan, `focusId` dipertahankan. Pohon jadi kosong: `focusId` dihapus.
- `maximizedId` selalu dibersihkan bila menunjuk id yang hilang dari pohon.
- Semua fungsi mengembalikan objek baru; sepanjang jalur yang berubah disalin, subtree yang tidak berubah boleh dibagi referensinya (tetap tidak boleh dimutasi). Keluaran tidak memuat `undefined` eksplisit pada properti opsional yang merusak kesamaan JSON (hapus kunci, jangan set `undefined`).

## Langkah

Semua berkas di `frontend/src/`. Identifier Inggris, komentar Indonesia, tanpa `console.log`.

1. **`modules/layout/model.ts`, tipe.** Ekspor `PaneKind = 'terminal' | 'file'`, `Leaf`, `Split`, `Node`, `LayoutDoc`, `DropZone`, `SplitPath = string`, dan `NewLeaf { kind: PaneKind; sessionId?; projectId?; provider?; filePath?; workspaceId?; title? }` persis overview 2.2/2.3. Tidak ada impor dari `registry.ts` (cegah impor melingkar).

2. **`model.ts`, helper.**
   - `emptyDoc = (): LayoutDoc => ({ version: 1, root: null })`.
   - `clampRatio(r)`: bukan angka hingga (NaN/Infinity) menjadi 0.5, selain itu `Math.min(0.9, Math.max(0.1, r))`.
   - `newLeafId()` (internal) sesuai keputusan di Kontrak.
   - `leaves(n: Node | LayoutDoc | null): Leaf[]`: bila argumen punya `version` ambil `.root`; traversal a sebelum b.
   - `findLeaf(doc, id)`, `findLeafBy(doc, pred)`, `pathOf(doc, id)`.
   - Internal: `getAt(root, path)`, `replaceAt(root, path, node)` (menyalin sepanjang jalur), `mapLeaf(node, id, fn)`, `removeLeaf(node, id): Node | null` (mencabut leaf; Split yang kehilangan satu anak digantikan anak yang tersisa; tidak mengubah fokus), `firstLeaf(node)`, `basename(path)`.
   - `normalize(doc): LayoutDoc` (internal): tidak ada Split dengan anak `null`/`undefined`, `ratio` di-clamp, `focusId` dan `maximizedId` yang tidak ada di pohon dibuang. Dipanggil di akhir `closePane`, `movePane`, `hydrateDoc`.

3. **`model.ts`, aksi murni** (signature persis overview 2.3):
   - `openPane(doc, leaf, opts?)`: jika `leaf.sessionId` sudah ada di pohon, atau `leaf.filePath` + `leaf.projectId` sama dengan leaf yang ada, kembalikan `{ doc: setFocus(doc, id), id }` (id lama). Selain itu buat Leaf (`type:'pane'`, id baru, `title = leaf.title ?? leaf.provider ?? basename(leaf.filePath) ?? 'Pane'`). Pohon kosong: jadi root + `focusId`. Selain itu `splitPane` pada `doc.focusId` (bila tidak valid pakai leaf terakhir dari `leaves`) dengan `dir = opts?.dir ?? 'h'`, side `'after'`. Pane baru difokuskan. Bila `maximizedId` ada, biarkan apa adanya (renderer yang menangani; pane baru tidak otomatis memulihkan).
   - `splitPane(doc, paneId, dir, newLeaf, side = 'after')`: `newLeaf` yang sudah berbentuk `Leaf` (punya `type === 'pane'` dan `id`) dipakai apa adanya (dipakai `movePane`); `NewLeaf` dibuatkan Leaf baru. Ganti leaf target dengan `Split { dir, ratio: 0.5, a, b }` (`after`: a = target, b = baru; `before`: a = baru, b = target). Target tidak ada: kembalikan `{ doc, id: <id leaf baru> }` tanpa perubahan pohon (atau doc apa adanya dan id leaf bawaan). Fokus ke leaf baru.
   - `closePane(doc, paneId)`: `removeLeaf`; root leaf yang ditutup menjadi `root: null`; aturan fokus dan `maximizedId` di Kontrak; lalu `normalize`. Id tidak ada: doc apa adanya.
   - `swapPanes(doc, aId, bId)`: abaikan bila id sama atau salah satu tidak ada. Tukar objek leaf pada posisi masing-masing (objek leaf dan id ikut pindah); `focusId` dan `maximizedId` tidak berubah (keduanya berbasis id).
   - `movePane(doc, paneId, targetId, zone)`: guard `paneId === targetId` atau salah satu tidak ditemukan mengembalikan doc apa adanya. Jika `maximizedId` ada, `restore` dulu. `center` memanggil `swapPanes`. Sisi: ambil objek leaf sumber (`findLeaf`), `removeLeaf` dari pohon (tanpa mengubah fokus), `splitPane` pada target dengan pemetaan `top`: `v`/`before`, `bottom`: `v`/`after`, `left`: `h`/`before`, `right`: `h`/`after`, memakai objek leaf yang sama; `focusId = paneId`; `normalize`. Ratio hasil 0.5.
   - `resize(doc, path, ratio)`: `replaceAt` pada Split di `path` dengan `ratio: clampRatio(ratio)`; bukan Split: doc apa adanya.
   - `maximize(doc, paneId)`: set `maximizedId` hanya bila id ada; `restore(doc)`: `maximizedId = null` (atau hapus kunci). Pohon tidak berubah.
   - `tidyGrid(doc)`: `list = leaves(doc)`, `n = list.length`; `n === 0` kembalikan doc (tanpa `maximizedId`); `cols = Math.ceil(Math.sqrt(n))`; potong `list` per `cols` menjadi baris urut baca (baris terakhir berisi sisa; 5 jadi 3+2, 6 jadi 3+3, 7 jadi 3+3+1, 8 jadi 3+3+2). Isi baris = rantai Split `h` berat sama: untuk `k` leaf, simpul ke-`i` (mulai 0) `{ dir:'h', ratio: 1/(k-i), a: leaf[i], b: sisa rantai }`; baris berisi satu leaf adalah leaf itu sendiri. Baris disusun serupa dengan Split `v`, ratio `1/(rows-i)`. Objek leaf dipakai ulang (id dipertahankan). `focusId` dipertahankan, `maximizedId` dibersihkan. n = 1 menghasilkan root leaf yang sama.
   - `setFocus(doc, paneId)`: hanya bila id ada. `renamePane(doc, paneId, title)`: `title.trim()`; kosong berarti doc apa adanya.
   - `hydrateDoc(raw: unknown)`: bungkus try/catch; `raw` harus objek dengan `version === 1`, selain itu `emptyDoc()`. Validasi rekursif tiap simpul: leaf wajib `type === 'pane'`, `id` string tak kosong, `kind` string tak kosong, `title` string (bila tidak string, isi `'Pane'`), properti opsional bertipe string diambil bila string, sisanya dibuang; Split wajib `dir` di {h,v}, `ratio` angka (di-clamp), `a` dan `b` divalidasi; anak tidak valid dibuang dan Split yang kehilangan satu anak diganti anak yang tersisa (kedua hilang: simpul dibuang). Id leaf duplikat: buang kemunculan kedua. `focusId`/`maximizedId` yang tidak ada di pohon dibuang (`maximizedId` hanya string atau null). Hasil akhir lewat `normalize`. Kesalahan apa pun menghasilkan `emptyDoc()`; tidak pernah melempar.

4. **`modules/layout/registry.ts`.** Impor tipe `Component` dari `svelte`, `writable`/`Readable` dari `svelte/store`, tipe `Leaf` dari `./model`. Ekspor `PaneKindSpec` persis overview 2.5 (`component: Component<{ pane: Leaf; visible: boolean }>`; gunakan `Component<any>` bila type-check menolak variansi, sejalan `lib/registry.ts`). `registerPaneKind(spec)` memakai `writable<Record<string, PaneKindSpec>>({})` dan `update((m) => ({ ...m, [spec.kind]: spec }))` (menimpa kind yang sama). `paneKinds(): Readable<Record<string, PaneKindSpec>>` mengembalikan `{ subscribe }`. `getPaneKind(kind)` = `get(store)[kind]`.

5. **`lib/stores/layout.ts`.** Impor fungsi dan tipe dari `../../modules/layout/model` (path relatif dari `lib/stores`). `createLayoutStore(initial?)`:
   - `const store = writable<LayoutDoc>(initial ? hydrateDoc(initial) : emptyDoc())`.
   - Metode aksi: `store.update((d) => fn(d, ...))`. `openPane`/`splitPane` menangkap `id` dari hasil di dalam `update` lalu mengembalikannya. `hydrate(raw)` = `store.set(hydrateDoc(raw))`. `serialize()` = `JSON.parse(JSON.stringify(get(store)))`.
   - `paneCount = derived(store, (d) => leaves(d).length)`, `leaves = derived(store, (d) => leaves(d))` (hindari bentrok nama dengan fungsi impor: impor sebagai `leavesOf`).
   - Kembalikan objek `LayoutStore` dengan `subscribe: store.subscribe`. Ekspor interface `LayoutStore` persis overview 2.4 (impor tipe `NewLeaf`, `Leaf`, `DropZone`, `SplitPath`, `LayoutDoc`) dan `export const layout = createLayoutStore()`. JANGAN menambahkannya ke `lib/stores/index.ts`.

6. **Stub antarmuka** (baris pertama tiap berkas: `// STUB(foundation): ditimpa oleh <pemilik>`; untuk `.svelte` pakai komentar HTML). Signature final, jangan lebih dari yang dibutuhkan:
   - `modules/layout/panes/hosts.ts` (pemilik `pane-kinds`): `Map<string, HTMLElement>`; `ensureHost(leaf, visible)` membuat `div.pane-host` (style `width:100%;height:100%;min-height:0;display:flex;flex-direction:column`) berisi teks `leaf.title`, tanpa me-mount komponen, menyimpan dan mengembalikan host yang sama pada panggilan berikutnya; `getHost`; `attachHost(paneId, slot)` (`appendChild` bila belum anak slot); `detachHost` (`host.remove()`); `releaseHost` (remove + delete dari Map); `syncPanes(leaves, visibleIds)` (ensureHost tiap leaf, releaseHost untuk id yang hilang). Argumen tak terpakai boleh diberi awalan `_`.
   - `modules/layout/PaneHeader.svelte` (pemilik `pane-header-dnd`): `$props()` bertipe `{ pane: Leaf; index: number; focused: boolean; maximized: boolean; store?: LayoutStore; onSplit: (dir: 'h'|'v') => void; onMaximizeToggle: () => void; onClose: () => void; onRename: (title: string) => void; onEndSession: () => void; onTidy: () => void }`; akar `<div data-pane-header>` menampilkan `{index} · {pane.title}` dan tombol `×` (`onclick={onClose}`, `aria-label="Tutup pane"`). `lang="ts"`.
   - `modules/layout/dnd.ts` (pemilik `pane-header-dnd`): ekspor `interface DragState { paneId; title; targetId: string | null; zone: DropZone | null; rect: { x; y; w; h } | null }`, `dragState = readable<DragState | null>(null)`, `zoneAt(rect, x, y)` berfungsi benar, dan `startPaneDrag(e, paneId, title, store?)` yang berbadan kosong. Aturan `zoneAt`: hitung `rx = (x-rect.x)/rect.w`, `ry = (y-rect.y)/rect.h`; jarak tepi `top = ry`, `bottom = 1-ry`, `left = rx`, `right = 1-rx`; bila min jarak > 0.25 hasil `center`; selain itu sisi dengan jarak terkecil, seri dipecahkan urutan `top`, `bottom`, `left`, `right`.
   - `modules/layout/DropOverlay.svelte` (pemilik `pane-header-dnd`): hanya `<!-- stub -->` tanpa props.

7. **Sanity murni (tanpa unit test, TIDAK masuk repo).** Opsional namun disarankan: bundel `model.ts` dengan `esbuild` yang sudah ada di `frontend/node_modules` (tanpa menambah dependency) ke berkas di scratchpad, jalankan dengan `node` dari scratchpad, lalu periksa: `tidyGrid` n=1..9 (6 menjadi 3x2, 5 menjadi 3+2, id leaf identik sebelum dan sesudah); `movePane` untuk 4 zona mempertahankan set id dan menghasilkan pohon tanpa Split berkaki satu; `swapPanes` menukar posisi; `closePane` pada leaf di tengah pohon; `hydrateDoc(JSON.parse(JSON.stringify(doc)))` sama dengan `doc` (bandingkan `JSON.stringify`); `hydrateDoc('rusak')`, `hydrateDoc(null)`, `hydrateDoc({version:2})` menjadi `emptyDoc()`; masukan tidak berubah setelah tiap aksi (bandingkan JSON sebelum/sesudah). Bila tidak memakai skrip, lakukan pengecekan mental pada kasus yang sama. Hapus berkas sementara di scratchpad.

8. **Verifikasi.**
   - `cd frontend && npm run check && npm run build` lulus (bila `check` sudah punya galat bawaan sebelum fitur ini, bandingkan; tidak boleh ada galat baru di berkas fitur ini).
   - `grep -n "ipc\|call(" frontend/src/modules/layout/model.ts frontend/src/lib/stores/layout.ts` kosong.
   - `grep -rn "stores/index\|stores'" frontend/src/modules/layout/model.ts frontend/src/modules/layout/registry.ts frontend/src/lib/stores/layout.ts` kosong (tidak ada impor dari barrel store).
   - `grep -rni "webgl" frontend/src/modules/layout` kosong.
   - Pastikan `model.ts` tidak mengimpor `registry.ts` dan tidak ada `console.log` tersisa.

## Perubahan

Dibuat (semua baru; tidak ada berkas lama yang diubah):

- `frontend/src/modules/layout/model.ts`
- `frontend/src/modules/layout/registry.ts`
- `frontend/src/lib/stores/layout.ts`
- `frontend/src/modules/layout/panes/hosts.ts` (stub)
- `frontend/src/modules/layout/PaneHeader.svelte` (stub)
- `frontend/src/modules/layout/dnd.ts` (stub)
- `frontend/src/modules/layout/DropOverlay.svelte` (stub)

## Wiring

Fitur ini tidak menyambungkan apa pun ke aplikasi (tanpa `index.ts`, tanpa edit `App.svelte`, `main.ts`, atau `lib/registry.ts`). Yang diekspos untuk langkah wiring dan fitur lain:

- `layout` dari `lib/stores/layout.ts` (dipakai shim `modules/tabs/index.ts` oleh `layout-wiring`: `layout.openPane`, `layout.setFocus`, `layout.closePane`, `layout.leaves`, subscribe).
- `createLayoutStore`, `LayoutStore`, `hydrate`/`serialize` (untuk tiket 06 nanti).
- `registerPaneKind`, `paneKinds`, `getPaneKind`, `PaneKindSpec` dari `modules/layout/registry.ts` (dipanggil `pane-kinds` dan dibaca `layout-renderer`).
- Tipe dan fungsi model dari `modules/layout/model.ts` (dipakai renderer, header, dnd).
- Stub `hosts.ts`, `PaneHeader.svelte`, `dnd.ts`, `DropOverlay.svelte` yang akan ditimpa penuh oleh `pane-kinds` dan `pane-header-dnd` dengan signature ekspor yang sama; `layout-wiring` memastikan teks placeholder sudah tidak ada.

## Selesai bila

- `model.ts`, `registry.ts`, `lib/stores/layout.ts` ada dan mengekspor semua nama pada Kontrak dengan signature persis overview 2.2 sampai 2.5; tipe `Leaf`, `Split`, `Node`, `LayoutDoc`, `DropZone` sesuai tiket (plus `workspaceId?`).
- Semua aksi murni (masukan tidak dimutasi), tanpa IPC; `closePane`, `movePane` (4 zona), `swapPanes`, `splitPane` menghasilkan pohon valid tanpa Split berkaki satu dan id leaf yang dipindah tidak berubah.
- `tidyGrid` n=1..9 sesuai aturan (6 menjadi 3x2, 5 menjadi 3+2) dengan id dipertahankan; `hydrateDoc(serialize())` identik; `hydrateDoc` pada data rusak tidak melempar dan menghasilkan dokumen kosong/valid.
- `createLayoutStore()` menghasilkan store independen; `layout` adalah instans bawaan; `paneCount` dan `leaves` benar.
- Keempat stub ada dengan signature final dan komentar `STUB(foundation)`.
- Guard: `cd frontend && npm run check && npm run build` sukses; grep pada langkah 8 bersih. Tidak ada berkas test di repo, tidak ada perubahan di luar Path yang dimiliki, tidak ada commit/merge sendiri kecuali diminta alur `implement-feature`.
