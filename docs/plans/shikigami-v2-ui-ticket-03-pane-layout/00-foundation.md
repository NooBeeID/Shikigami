# Foundation: layout-model (model pohon, store, registri, stub antarmuka)

Baca dulu `overview.md` (kontrak bagian 2 adalah sumber kebenaran; plan ini hanya urutan kerja). Tanpa UI nyata, tanpa IPC, tanpa dependency baru, tanpa unit test.

Berkas yang dibuat (semua di `frontend/src/`):
- `modules/layout/model.ts`
- `modules/layout/registry.ts`
- `lib/stores/layout.ts`
- Stub: `modules/layout/panes/hosts.ts`, `modules/layout/PaneHeader.svelte`, `modules/layout/dnd.ts`, `modules/layout/DropOverlay.svelte`

## Langkah

1. **`model.ts`: tipe.** Ekspor `PaneKind`, `Leaf` (dengan tambahan opsional `workspaceId`), `Split`, `Node`, `LayoutDoc`, `DropZone`, `SplitPath`, `NewLeaf` persis bagian 2.2/2.3.
2. **`model.ts`: helper.** `emptyDoc`, `clampRatio` (`[0.1, 0.9]`, NaN menjadi 0.5), `newLeafId` (internal, `p_` + id acak dari `crypto.randomUUID()` dengan fallback penghitung), `leaves` (urutan baca a lalu b, menerima `Node | LayoutDoc | null`), `findLeaf`, `findLeafBy`, `pathOf` (`''` untuk root leaf, `'a.b'` untuk simpul bersarang; jalur ke Split dipakai `resize`), `getAt(doc, path)` / `replaceAt(root, path, node)` internal yang tidak memutasi (salin sepanjang jalur).
3. **`model.ts`: aksi murni** (setiap fungsi menerima `LayoutDoc`, mengembalikan dokumen baru; jangan memutasi masukan; hasil JSON murni):
   - `openPane`: cari `sessionId` sama, atau `filePath` + `projectId` sama; bila ada, fokus dan kembalikan id lama. Selain itu buat leaf (judul bawaan `title ?? provider ?? basename(filePath) ?? 'Pane'`); pohon kosong menjadi root; selain itu `splitPane` pada pane fokus (atau leaf terakhir bila fokus tidak valid) dengan `dir` `opts.dir ?? 'h'`, side `after`; set fokus ke pane baru.
   - `splitPane`: ganti leaf target dengan `Split{dir, ratio:0.5, a,b}` sesuai `side`; leaf baru yang sudah punya `id` (bentuk `Leaf`) dipertahankan; fokus ke leaf baru.
   - `closePane`: hapus leaf, saudara menggantikan induk; root leaf yang ditutup membuat `root:null`; fokus pindah ke leaf saudara terdekat (leaf pertama dari subtree saudara); `maximizedId` dibersihkan bila sama.
   - `swapPanes`: tukar dua leaf di posisinya (objek leaf tetap, id ikut pindah); abaikan bila id sama atau tidak ada.
   - `movePane`: guard `paneId === targetId` dan id tidak ditemukan mengembalikan doc apa adanya; jika `maximizedId` ada, `restore` dulu; `center` memanggil `swapPanes`; sisi: ambil objek leaf (tanpa mengubah), `closePane`-sejenis cabut tanpa mengubah fokus, lalu `splitPane` target dengan `dir` dan `side` dari zona (`top`: v/before, `bottom`: v/after, `left`: h/before, `right`: h/after) dengan memakai objek leaf yang sama; fokus ke leaf yang dipindah.
   - `normalize` (internal): tidak ada Split dengan anak `null`; dipanggil di akhir `closePane`/`movePane`/`hydrateDoc`.
   - `resize(doc, path, ratio)`: `clampRatio`, abaikan jalur yang bukan Split.
   - `maximize`/`restore`: hanya `maximizedId`; `maximize` pada id yang tidak ada diabaikan.
   - `tidyGrid`: algoritma di overview 2.3 (rantai Split seimbang; ratio `1/(sisa)`); 0 atau 1 leaf: hasilnya sama dengan masukan terstruktur ulang; pertahankan `focusId`, bersihkan `maximizedId`.
   - `setFocus` (hanya bila id ada), `renamePane` (judul di-trim; kosong berarti batal).
   - `hydrateDoc(raw)`: terima `unknown`; validasi `version === 1`, tipe tiap simpul, `dir` ∈ {h,v}, `ratio` numerik (di-clamp), leaf wajib `id` string dan `kind` string (leaf tidak valid dibuang, Split yang kehilangan anak diganti anak yang tersisa), buang `focusId`/`maximizedId` yang tidak ada di pohon; apa pun yang rusak menghasilkan `emptyDoc()`; tidak melempar (bungkus try/catch).
4. **`registry.ts`.** `registerPaneKind` (store `writable<Record<string, PaneKindSpec>>`, menimpa kind yang sama), `paneKinds()` mengembalikan `Readable` (pola sama dengan `lib/registry.ts`), plus helper `getPaneKind(kind): PaneKindSpec | undefined` untuk pemakaian non-reaktif. Ekspor `PaneKindSpec` persis bagian 2.5. Impor tipe `Leaf` dari `./model` (tidak boleh impor melingkar: `model.ts` tidak mengimpor `registry.ts`).
5. **`lib/stores/layout.ts`.** `createLayoutStore(initial?)`: `writable<LayoutDoc>(initial ? hydrateDoc(initial) : emptyDoc())`; setiap metode memanggil fungsi murni lewat `update`; `openPane`/`splitPane` mengembalikan id dari hasil; `hydrate(raw)` = `set(hydrateDoc(raw))`; `serialize()` = salinan `JSON.parse(JSON.stringify(get(store)))`; `paneCount` dan `leaves` = `derived`. Ekspor `layout = createLayoutStore()`. JANGAN ditambah ke `lib/stores/index.ts` (bukan milik epic); modul lain mengimpor langsung dari `lib/stores/layout`.
6. **Stub antarmuka** (signature final dari overview 2.6/2.7; semua berkas ini akan ditimpa penuh oleh pemilik fitur, beri komentar `// STUB(foundation): ditimpa oleh <fitur>` di baris pertama):
   - `panes/hosts.ts`: ekspor `ensureHost`, `getHost`, `attachHost`, `detachHost`, `releaseHost`, `syncPanes` dengan `Map<string, HTMLElement>`; `ensureHost` membuat `div.pane-host` berisi teks `pane.title` tanpa me-mount komponen.
   - `PaneHeader.svelte`: `$props()` sesuai props 2.7 (termasuk `store?`), menampilkan `{index} · {pane.title}` dan tombol `×` memanggil `onClose`; atribut `data-pane-header` pada akar.
   - `dnd.ts`: tipe `DragState`, `dragState` (`readable<DragState | null>(null)`), `zoneAt` berfungsi benar sesuai aturan 25% (agar renderer bisa memanggilnya), `startPaneDrag` fungsi kosong.
   - `DropOverlay.svelte`: komponen kosong tanpa props (`<!-- stub -->`).
7. **Verifikasi.** `cd frontend && npm run check && npm run build` lulus. Periksa bahwa tidak ada impor dari `lib/stores/index.ts` ke layout (tidak boleh mengubah berkas itu) dan tidak ada IPC/`call(` di `model.ts` dan `lib/stores/layout.ts` (`grep -n "ipc\|call(" ...` kosong).
8. **Sanity murni (tanpa unit test)**: boleh memakai skrip sekali pakai di scratchpad (bukan di repo) atau pengecekan mental untuk: `tidyGrid` n=1..9 (6 jadi 3x2, 5 jadi 3+2), `movePane` 4 zona mempertahankan id, `hydrateDoc(serialize())` identik, `hydrateDoc('rusak')` jadi dokumen kosong. Jangan menambah berkas test ke repo.

## Kriteria selesai

- Semua acceptance criteria fitur `layout-model` di tiket terpenuhi; stub bagian 4 overview ada dengan signature final.
- Build guard lulus. Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
