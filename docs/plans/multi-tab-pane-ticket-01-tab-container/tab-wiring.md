# Plan: Wiring dan verifikasi build tiket tab-container

## Metadata

- Tiket: `docs/tickets/multi-tab-pane-ticket-01-tab-container.md`
- Fitur: `tab-wiring`
- ownedPaths: `frontend/src/App.svelte`, `frontend/src/modules/layout/index.ts`
- Depends on foundation: ya (`00-foundation.md`), dan dikerjakan SETELAH semua fitur paralel lain selesai (`tab-model-foundation`, `tab-persist-migration`, `tab-callers`, `tab-hosts-render`, `tab-header-ui`).

## Tujuan

Menyambungkan hasil semua fitur tiket: pastikan pendaftaran kind dan `App.svelte` tidak memakai API lama, `LayoutArea` memakai `allTabs` untuk `syncPanes`, header pane menerima aksi tab, lalu bereskan sisa error tipe lintas fitur seminimal mungkin sampai build guard lulus. Verifikasi skenario manual dan catat hasilnya di pesan commit.

Baca `overview.md` (bagian 2 kontrak, 5 wiring checklist, 6 build guard) lebih dulu; plan ini tidak mengulangnya.

## Path yang dimiliki

HANYA ini yang boleh diedit sebagai pekerjaan utama:

- `frontend/src/App.svelte`
- `frontend/src/modules/layout/index.ts`

Pengecualian terbatas: bila `npm run check` masih menampilkan error tipe lintas fitur yang tidak bisa diselesaikan di dua file di atas, boleh mengedit file penyebab error secukupnya (perubahan minimal, hanya agar kompilasi lulus sesuai kontrak overview 2). Jangan merombak perilaku, jangan menyentuh file tiket 02/03 (`layout/dnd.ts`, `layout/DropOverlay.svelte`, `layout/TabContextMenu.svelte`, `modules/files/`), jangan mengubah bentuk kontrak, jangan mengubah `overview.md`, jangan menambah vitest/unit test. Catat setiap file di luar dua path utama yang diedit pada laporan dan pesan commit.

## Kontrak

Dipakai (disediakan fitur lain, lihat overview):

- `model.ts`: `Tab`, `Leaf`, `activeTab`, `allTabs`, `findTab`, `findLeaf`, `findLeafBy`, `leaves`, `newPaneId`, `newTabId`, `renameTab`, `activateTab`, `closeTab`, ekspor ulang `hydrateDoc` (overview 2.1-2.3, 2.6).
- Store layout: `activateTab(paneId, tabId)`, `closeTab(paneId, tabId)`, `renameTab(paneId, tabId, title)`, `openPane(nl, opts?)`, `openThreadAgent(nl)` mengembalikan id pane (overview 2.4).
- `PaneKindSpec` dengan `component: Component<{ pane: Tab; paneId: string; visible: boolean }>` (overview 2.5).
- `hosts.ts`: `syncPanes(entries: { pane: Leaf; tab: Tab }[], ctx)`, `releaseHost(tabId)` (overview 2.7).
- `shell/helpers.ts` mengembalikan `{ paneId, tabId }` (overview 2.8).

Disediakan: tidak ada kontrak baru. Fitur ini hanya memverifikasi dan menyambung.

## Langkah

1. Pastikan semua fitur lain sudah selesai: `cd /Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal && git status` dan periksa bahwa `model.ts`, `registry.ts`, `lib/stores/layout.ts`, `persist/hydrate.ts`, `layout/panes/hosts.ts`, `LayoutArea.svelte`, `PaneHeader.svelte` sudah berubah. Bila ada yang belum, jangan menulis ulang fitur itu; catat di laporan.

2. `frontend/src/modules/layout/index.ts` (saat ini hanya `import './panes/terminal'; import './panes/file';`):
   - Pertahankan kedua import tersebut. Tambahkan import kind lain di folder `panes/` hanya bila `tab-hosts-render` membuat file registrasi baru (`ls frontend/src/modules/layout/panes/*.ts`).
   - Kind `thread`, `lark-board`, `lark-doc`, `git-mr`, `git-pipeline` didaftarkan oleh `modules/thread/index.ts`, `modules/lark/index.ts`, `modules/git/index.ts`; jangan dipindah ke sini. Pastikan modul-modul itu masih diimpor dari jalur startup (cek `grep -rn "modules/lark'\|modules/git'\|modules/thread'\|shell/register" frontend/src`); jangan menambah pendaftaran ganda.
   - Pastikan tidak ada ekspor ke simbol yang dihapus (`renamePane`, field konten `Leaf`). Bila index.ts mengekspor sesuatu dari model/store, sesuaikan dengan nama baru.

3. `frontend/src/App.svelte`:
   - Jalankan `grep -n "renamePane\|findLeaf\|findLeafBy\|leaves(\|opts.dir\|{ dir\|\.sessionId\|\.filePath\|\.kind" frontend/src/App.svelte`. Saat ini App.svelte hanya mengimpor `LayoutArea` dan tidak diketahui memakai API lama; bila grep kosong, tidak ada perubahan wajib.
   - Bila ada pemakaian API lama: `renamePane(id, t)` menjadi `renameTab(paneId, tabId, t)` (atau `renameTab` pada `activeTab(findLeaf(doc, paneId))`); akses field konten pada leaf diganti `activeTab(leaf).<field>` atau `findTab`/`allTabs`; `openPane(nl, { dir })` menjadi `openPane(nl)`.
   - Pastikan `<LayoutArea />` dipasang tanpa prop yang sudah dihapus `tab-hosts-render`/`tab-header-ui`. Bila header host (`shell/LayoutHeaderHost.svelte`) memerlukan prop baru dari App, ikuti kontrak yang ada di berkas itu.

4. Verifikasi sambungan lintas fitur lewat grep (baca hasilnya, jangan asal ubah):
   - `grep -rn "syncPanes" frontend/src` memastikan hanya `LayoutArea.svelte` memanggil dan argumennya berasal dari `allTabs(doc)`.
   - `grep -rn "findLeafBy\|findLeaf\|leaves(\|renamePane" frontend/src` memastikan tidak ada akses field konten (`sessionId`, `filePath`, `kind`, `title`, dst.) langsung pada `Leaf` dan tidak ada sisa `renamePane`.
   - `grep -rn "opts.dir\|{ dir" frontend/src` memastikan `dir` tidak lagi diteruskan ke `openPane`.
   - `grep -rn "activateTab\|closeTab" frontend/src/modules/shell/LayoutHeaderHost.svelte frontend/src/modules/layout/PaneHeader.svelte` memastikan header menerima aksi tab dari store.
   - Perhatikan registrasi kind di luar ownedPaths (`modules/lark/index.ts`, `modules/git/index.ts`, `modules/thread/index.ts`) yang memakai parameter `(leaf) => leaf.title`: kompilasi tetap valid karena tipenya `Tab`; ubah nama parameter ke `tab` hanya bila svelte-check mengeluh.

5. Jalankan build guard dari root repo dan perbaiki iteratif:
   - `cd /Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal/frontend && npm run check`. Kelompokkan error per berkas. Untuk tiap error: perbaiki di berkas penyebab dengan perubahan terkecil yang sesuai kontrak overview 2 (mis. ganti `leaf.sessionId` menjadi `activeTab(leaf).sessionId`, sesuaikan tipe prop komponen pane yang terlewat seperti `lark/doc/DocPane.svelte` menjadi `pane: Tab`). Komponen yang menerima `pane` dari registry harus bertipe `Tab`.
   - Ulangi `npm run check` sampai nol error (warning yang sudah ada sebelum tiket boleh tersisa).
   - `cd frontend && npm run build` sampai lulus.
   - `cd /Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal && go build ./...` sampai lulus. Frontend-only, jadi kegagalan Go biasanya karena `frontend/dist` belum ada atau embed; jalankan `npm run build` dulu. Bila proyek punya target aplikasi (`make build-ui`, `make build-app`; lihat `Makefile`), jalankan hanya yang tidak memerlukan signing/instalasi, dan catat bila dilewati.

6. Verifikasi manual bila aplikasi bisa dijalankan (`make run-daemon` / `make app` sesuai `Makefile`; tanpa `pkill` pola luas, matikan hanya PID yang Anda luncurkan sendiri). Skenario, catat lulus/gagal masing-masing:
   - buka file dari file tree berulang kali: tab bertambah di pane fokus, pane tidak terpecah;
   - buka file yang sama lagi: tab yang ada diaktifkan, tidak ada tab ganda;
   - pindah antar tab terminal dan file: terminal tetap hidup, scrollback utuh, ukuran benar;
   - tutup tab (tab terakhir menutup pane); minimize terminal (sesi tetap di sidebar);
   - Cmd+D pada terminal: pane baru satu tab; pada tab file tidak melakukan apa-apa;
   - sesi baru (Cmd+T), thread agent, lark, git, grimoire, drag sesi dari sidebar menghasilkan tab terlihat di header;
   - muat ulang aplikasi dengan layout tersimpan bentuk lama (leaf dengan field konten) dan bentuk baru.
   Bila aplikasi tidak bisa dijalankan di lingkungan agent, tulis eksplisit di laporan dan pesan commit bahwa verifikasi manual tidak dilakukan beserta alasannya; jangan mengklaim lulus.

7. Pesan commit (hanya bila alur `implement-feature` memintanya; jangan commit sendiri di luar itu): sertakan ringkasan hasil `npm run check`, `npm run build`, `go build ./...`, daftar skenario manual beserta hasil, dan daftar file di luar ownedPaths yang diperbaiki.

## Perubahan

- Diubah (bila perlu): `frontend/src/modules/layout/index.ts` (import kind, tanpa ekspor ke simbol lama).
- Diubah (bila perlu): `frontend/src/App.svelte` (hanya bila memakai API lama atau prop berubah).
- Kemungkinan diubah minimal di luar ownedPaths: berkas lintas fitur yang masih error tipe setelah semua fitur selesai (mis. `modules/lark/doc/DocPane.svelte`, parameter `titleFor` di `modules/lark/index.ts`, `modules/git/index.ts`, `modules/thread/index.ts`). Wajib dicatat.
- Tidak ada berkas baru.

## Wiring

Fitur ini adalah langkah wiring itu sendiri; tidak ada yang diekspos untuk disambungkan nanti. Titik ekstensi untuk tiket 02/03 (`PaneHeader` deretan tab, `model.ts` untuk `moveTab`) dibiarkan tersedia dan tidak ditambah di sini.

## Selesai bila

- `cd frontend && npm run check` nol error.
- `cd frontend && npm run build` lulus.
- `go build ./...` lulus.
- `layout/index.ts` masih mengimpor pendaftaran kind; `App.svelte` dan seluruh `frontend/src` tidak memakai `renamePane`, `opts.dir`, atau field konten langsung pada `Leaf`.
- `LayoutArea` memakai `allTabs` untuk `syncPanes`; header menerima `activateTab`/`closeTab` lewat `LayoutHeaderHost`.
- Hasil verifikasi manual (atau alasan tidak dilakukan) tercatat di laporan dan pesan commit.
- Tidak ada unit test/vitest yang ditambahkan; tidak ada berkas tiket 02/03 yang disentuh.
