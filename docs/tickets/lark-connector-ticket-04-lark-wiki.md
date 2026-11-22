---
id: lark-connector-ticket-04-lark-wiki
title: Wiki pohon dan penampil dokumen
spec: docs/superpowers/specs/2026-11-22-09-lark-connector-design.md
status: implemented
wave: 3
depends_on: [lark-connector-ticket-02-lark-sources]
---
# Wiki pohon dan penampil dokumen

## Tujuan
Epic ini membuat bagian Wiki di rail Lark benar-benar berguna: pohon wiki yang dimuat per level (lazy), dengan wiki Pribadi (`my_library`) selalu di paling atas, dan pane `lark-doc` yang menampilkan isi dokumen. Alasannya, user bekerja dengan agen sambil merujuk dokumen Lark, dan sekarang harus keluar app untuk membacanya. Dengan epic ini user bisa membaca docx dan sheet di app, lalu menyalin link atau mengirim link ke agen sebagai konteks (agen membaca isinya sendiri lewat skill `lark-*`).

Epic ini menimpa dua placeholder yang dibuat tiket `lark-connector-ticket-02-lark-sources`: `sections/WikiTree.svelte` dan `doc/DocPane.svelte`. Tiket ini tidak menyentuh sisi Tiket (board) dan tidak menyentuh file lain milik tiket 02.

## Hasil yang bisa dijalankan
Di atas tiket 01 (connector dan runner) dan tiket 02 (rail Lark, sumber per workspace, placeholder):
- Bagian **Wiki** di panel Lark menampilkan **Pribadi** di paling atas lalu wiki tim yang sudah ditambahkan; tiap wiki berupa pohon node. Membuka node memuat anaknya (satu level per pemanggilan, 50 per halaman) dengan keadaan memuat, kosong, dan galat per node, plus tombol segarkan.
- Ikon node menurut tipe: docx, sheet, dan lainnya.
- Klik node docx atau sheet membuka (atau mengganti) pane `lark-doc` di area tengah. Klik node tipe lain (bitable, mindnote, dst) tetap membuka pane, berisi judul + link dengan keterangan "buka di Lark".
- Pane `lark-doc`: docx dirender dari markdown (aman, HTML disanitasi); sheet tampil sebagai tabel sel awal (50 baris x 20 kolom) dengan catatan batas rentang; header punya **Salin link** dan **Kirim ke agen** (tempel ke sesi aktif tanpa Enter; tanpa sesi aktif muncul toast "Tidak ada sesi aktif").
- Pane yang dipulihkan setelah restart mengambil ulang isinya; bila sumber sudah dihapus atau jaringan gagal, pane menampilkan keadaan galat yang aman (bukan crash).
- `shikigamictl lark wiki list` dan `lark wiki get` untuk debug dari terminal.
- Build Go dan frontend lulus, aplikasi jalan.

## Batasan
In scope:
- Data wiki dan dokumen di `internal/larkdata`, IPC `lark.wiki.list` dan `lark.wiki.get`.
- `WikiTree.svelte` lazy, pane `lark-doc` (docx, sheet, tipe lain), render markdown, tabel sheet.
- Satu dependensi markdown ringan dengan sanitasi HTML bila belum ada komponen render markdown di frontend (saat tiket ditulis, `frontend/package.json` belum punya library markdown atau sanitizer; periksa ulang sebelum menambah).
- Perintah debug `shikigamictl lark wiki list|get`.
- Keadaan "disembunyikan" untuk wiki Pribadi bila `my_library` tidak bisa di-list (temuan spike).

Out of scope:
- Menulis atau mengubah dokumen/sheet (read-only total), tanpa cache disk, tanpa unit test.
- Penampil sheet penuh (hanya rentang awal), render bitable/mindnote, rendering gambar/lampiran dalam dokumen yang butuh unduhan.
- Menyisipkan isi dokumen ke sesi agen (hanya link).
- Sisi Tiket (board, `lark-board`), parser link, config sumber, `WikiSection`, `LarkPanel`, `actions.ts`, `client.ts`, `store.ts` (milik tiket 01, 02, 03). Tiket ini hanya memakai `actions.ts` dan kontrak props.
- Deteksi tenant Lark lain (tampil sebagai galat izin, batas yang diketahui).

## Kontrak
Disediakan oleh tiket ini.

IPC (nama dan bentuk persis, hasil dinormalisasi, bukan JSON mentah CLI):
- `lark.wiki.list` `{workspaceId, sourceId | 'personal', parentToken?}` -> `{nodes: [{token, title, objType, objToken, hasChild}], hidden?: boolean, hiddenReason?: string}`. `sourceId='personal'` memakai space `my_library`. `parentToken` kosong = akar (untuk sumber dengan `RootToken`, akarnya adalah `RootToken`). Satu halaman maksimal 50 node per level (`--page-size 50`).
- `lark.wiki.get` `{workspaceId, sourceId | 'personal', nodeToken}` -> `{title, objType, url, markdown?, cells?}`. `markdown` terisi untuk `objType='docx'`; `cells` (matriks string, baris x kolom, plus `truncated` bila rentang terpotong) untuk `objType='sheet'`; tipe lain hanya `title`, `objType`, `url`.
- Galat: kode connector yang sama dari tiket 01 (`cli_missing`, `not_logged_in`, dst) plus pesan Indonesia; galat izin/scope kurang menjadi "Izin Lark kurang" dengan pesan dari CLI; sumber tidak ditemukan di workspace -> kode `source_not_found`, pesan "Sumber tidak ada di workspace ini".

Go (paket `internal/larkdata`, dipanggil lewat `Run()` dari `runner.go` milik tiket 01, selalu `--as user`, format json, allowlist perintah baca):
- `wiki.go`: `ListWikiNodes(ctx, spaceID, parentToken string) ([]WikiNode, error)`; `GetWikiNode(ctx, spaceID, nodeToken, host string) (WikiNodeInfo, error)`; tipe `WikiNode{Token, Title, ObjType, ObjToken string; HasChild bool}`.
- `doc.go`: `FetchDocMarkdown(ctx, objToken string) (string, error)`; `FetchSheetCells(ctx, objToken string) (SheetCells, error)`; konstanta `sheetMaxRows = 50`, `sheetMaxCols = 20` (konstanta, bukan setelan).
- Perintah CLI yang dipakai: `wiki +node-list` (`--space-id --parent-node-token --page-size 50`), `wiki +node-get`, `docs +fetch --doc-format markdown`, `sheets +cells-get`. Perintah-perintah ini harus sudah ada di allowlist tiket 01; bila belum ada, JANGAN mengubah `allowlist.go`, lapor di keadaan galat dan catat di keterangan tiket (kontrak tiket 01 mencakup perintah wiki/docs/sheets yang tertulis di spec).
- Bentuk JSON dan nama field CLI mengikuti fixture dan temuan `docs/lark/cli-findings.md` dari spike tiket 01 (field `obj_type`, `obj_token`, `has_child`, URL node). Jangan menebak: baca temuan itu dulu.

Frontend:
- `WikiTree.svelte` menerima props `{source, onOpen(nodeToken)}` dari `WikiSection` (kontrak tiket 02, tidak diubah). `source` adalah sumber wiki atau sumber Pribadi (`id: 'personal'`). Node docx/sheet/lainnya memanggil `onOpen(nodeToken)`.
- `wikiTreeClient.ts`: pembungkus IPC `lark.wiki.list` untuk pohon (workspaceId diambil dari store workspace aktif yang sudah ada).
- Pane kind `lark-doc` sudah didaftarkan tiket 02 dengan props `{sourceId | 'personal', nodeToken}` dan field persist `sourceId`/`nodeToken`. Tiket ini hanya menimpa isi `doc/DocPane.svelte`; tidak mengubah registrasi atau loader layout.
- `docClient.ts`: pembungkus IPC `lark.wiki.get`.
- Memakai `actions.ts` (tiket 02): `copyLink(url)` dan `sendToAgent(url)`.

Dipakai dari tiket lain: `connector.RunCLI`/`Run()` (tiket 01), `workspace.LarkConfig`/`WikiSource` dan penentuan sumber per workspace (tiket 02), `registerPaneKind`, `registerRailItem` (rail utama), event `configChanged`.

## Fitur

### lark-wiki-data: Data wiki dan dokumen larkdata dan IPC
Deskripsi:
- `internal/larkdata/wiki.go`: memanggil `wiki +node-list` dengan `--space-id`, `--parent-node-token` (dikosongkan untuk akar), `--page-size 50`, `--as user`, format json. Hasil dinormalisasi ke `WikiNode` (title, token, `obj_type` -> `objType`, `obj_token` -> `objToken`, `has_child` -> `hasChild`). Sumber `personal` memakai space id `my_library`. `wiki +node-get` dipakai untuk judul, tipe, dan URL node; bila CLI tidak memberi URL, fallback `https://<host>/wiki/<node_token>` dengan `host` dari sumber (untuk `personal`, pakai host dari wiki sumber lain di workspace bila ada, atau host default hasil spike; catat keputusan di keterangan).
- Paginasi: bila CLI mengembalikan penanda halaman berikutnya pada level yang sama dan jumlah node melebihi 50, tangani sesuai temuan spike; minimal 50 node pertama muncul dan sisanya dilaporkan lewat `hasMore` bila CLI mendukung. Bila tak jelas, cukup halaman pertama dan catat.
- `internal/larkdata/doc.go`: `docs +fetch --doc-format markdown` untuk docx (argumen dari `objToken`, bukan `nodeToken`); `sheets +cells-get` untuk sheet dengan rentang awal 50 baris x 20 kolom (konstanta; format rentang mengikuti hasil spike untuk flag `sheets +cells-get`, mis. `A1:T50`); tipe lain tidak memanggil CLI, hanya metadata. Timeout daftar 20 detik (sesuai runner).
- Keadaan "disembunyikan": bila `my_library` tidak bisa di-list (temuan spike atau galat khusus), `lark.wiki.list` untuk `personal` mengembalikan `hidden: true` dan `hiddenReason` (bahasa Indonesia, berisi keterangan dari CLI), bukan galat keras, agar UI bisa menampilkan keterangan.
- `cmd/shikigamid/wire_lark_wiki.go`: mendaftarkan IPC `lark.wiki.list` dan `lark.wiki.get` (file sendiri; tidak mengedit `wire_lark.go`). Resolusi `sourceId` -> `SpaceID`/`RootToken`/`Host` lewat helper pembaca config workspace dari tiket 02 (tanpa mengimpor `workspace` dari `larkdata`; wire mengimpor keduanya). `personal` tidak dicari di config.
- Galat dipetakan ke pesan Indonesia; identifier kode tetap Inggris.
Acceptance criteria:
- [ ] `go build ./...` lulus.
- [ ] `lark.wiki.list` untuk `personal` mengembalikan node akar `my_library`, atau `hidden:true` + `hiddenReason` bila tak bisa di-list.
- [ ] `lark.wiki.list` dengan `parentToken` mengembalikan anak satu level dengan `hasChild` benar.
- [ ] `lark.wiki.get` docx mengembalikan `markdown` dan `url`; sheet mengembalikan `cells` maksimal 50 x 20 dengan penanda terpotong; tipe lain hanya judul/tipe/url.
- [ ] Hanya perintah baca di allowlist yang dipanggil, selalu `--as user`.
- [ ] `sourceId` tak dikenal menghasilkan galat `source_not_found` dengan pesan Indonesia, bukan panic.
ownedPaths: `internal/larkdata/wiki.go`, `internal/larkdata/doc.go`, `cmd/shikigamid/wire_lark_wiki.go`
Paralel: bisa dikerjakan paralel dengan semua fitur lain di epic ini; fitur frontend memakai kontrak IPC di atas (boleh dites dengan data tiruan lokal sementara).

### lark-wiki-tree-ui: Pohon wiki lazy di panel
Deskripsi:
- Timpa `sections/WikiTree.svelte` (placeholder tiket 02). Props `{source, onOpen(nodeToken)}` tidak berubah. Akar dimuat saat komponen tampil/dibuka; setiap node dengan `hasChild` bisa dibuka dan memuat anaknya saat pertama dibuka (cache di memori komponen saja, tanpa disk). Keadaan per node: memuat, kosong ("Belum ada isi"), galat terbaca (pesan dari IPC, tombol coba lagi), tombol segarkan di akar dan per node.
- Ikon menurut `objType`: docx, sheet, lainnya (ikon inline atau yang sudah ada di `LarkIcon.svelte`, tanpa mengubah file itu; bila perlu ikon baru, taruh di dalam `WikiTree.svelte`).
- Untuk sumber `personal`: bila hasil `hidden:true`, tampilkan `hiddenReason` sebagai keterangan, bukan galat.
- Klik node: `onOpen(node.token)`. Semua tipe memanggil `onOpen` (docx/sheet membuka isi, tipe lain membuka pane judul + link; keputusan tampilan ada di DocPane). Klik panah hanya membuka/menutup.
- `wikiTreeClient.ts`: `listWikiNodes(workspaceId, sourceId, parentToken?)` yang memanggil `lark.wiki.list`. Pohon di-reset saat `source` atau workspace berganti, dan disegarkan saat event `configChanged`.
- Aksesibilitas dasar: peran tree/treeitem, dapat dioperasikan keyboard (panah, Enter).
Acceptance criteria:
- [ ] Build frontend lulus.
- [ ] Pribadi tampil paling atas (urutan dijamin `WikiSection` tiket 02; WikiTree tidak mengubahnya), pohon dimuat lazy per level.
- [ ] Keadaan memuat/kosong/galat per node dan segarkan berfungsi.
- [ ] Klik node memanggil `onOpen(nodeToken)`; ikon sesuai tipe.
- [ ] Berganti workspace/sumber mengosongkan pohon lama dan memuat ulang.
- [ ] Tidak ada perubahan pada `WikiSection` atau berkas milik tiket lain.
ownedPaths: `frontend/src/modules/lark/sections/WikiTree.svelte`, `frontend/src/modules/lark/wikiTreeClient.ts`
Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini.

### lark-doc-pane: Pane lark-doc penampil dokumen
Deskripsi:
- Timpa `doc/DocPane.svelte`. Props `{sourceId | 'personal', nodeToken}`. Pane hanya menyimpan referensi; saat mount dan saat pulih dari persist, ambil ulang lewat `docClient.getWikiNode(...)` (`lark.wiki.get`).
- Header: judul (dari hasil), tombol segarkan, **Salin link** (`copyLink(url)`) dan **Kirim ke agen** (`sendToAgent(url)`), keduanya dari `../actions` milik tiket 02. Tombol nonaktif selama `url` belum ada.
- Isi menurut `objType`:
  - `docx`: `MarkdownView.svelte` merender `markdown`. Periksa dulu apakah frontend sudah punya komponen render markdown (cari di `frontend/src`); bila ada, pakai. Bila belum, tambah di `frontend/package.json` satu pustaka markdown ringan (mis. `marked`) ditambah sanitizer HTML (mis. `dompurify`), render lalu sanitasi sebelum `{@html}`. Tautan keluar dibuka dengan `target="_blank" rel="noopener noreferrer"`. Gambar yang gagal dimuat tidak boleh merusak tata letak.
  - `sheet`: `SheetView.svelte` menampilkan tabel sel dari `cells` (header kolom A, B, C..., nomor baris), dengan catatan "Menampilkan 50 baris x 20 kolom pertama" bila terpotong.
  - lainnya (bitable, mindnote, dst): judul dan link dengan keterangan "Buka di Lark" (tautan membuka URL di browser).
- Keadaan: memuat, galat aman (jaringan gagal, CLI belum login/terpasang, izin kurang, "Sumber tidak ada di workspace ini" bila `source_not_found`), tombol coba lagi. Tidak pernah melempar galat ke luar komponen.
- Bila `nodeToken` berubah (klik node lain di pohon mengganti pane), muat ulang dan batalkan hasil permintaan lama (guard urutan permintaan).
- `docClient.ts`: `getWikiNode(workspaceId, sourceId, nodeToken)` memanggil `lark.wiki.get`; tipe hasil `{title, objType, url, markdown?, cells?}`.
- Tema gelap/terang mengikuti token CSS aplikasi yang sudah ada.
Acceptance criteria:
- [ ] Build frontend lulus.
- [ ] docx tampil sebagai markdown ter-render, HTML berbahaya (mis. `<script>`, atribut `on*`) tersanitasi.
- [ ] sheet tampil sebagai tabel dengan catatan batas rentang.
- [ ] Tipe lain menampilkan judul + link dengan keterangan "Buka di Lark".
- [ ] Salin link dan Kirim ke agen bekerja; tanpa sesi aktif toast "Tidak ada sesi aktif" (perilaku dari `actions.ts`).
- [ ] Pulih dari persist: isi diambil ulang; sumber hilang atau jaringan gagal menghasilkan keadaan galat, bukan crash.
- [ ] Dependensi baru (bila ditambah) hanya di `frontend/package.json` dan lockfile; versi dipatok.
ownedPaths: `frontend/src/modules/lark/doc/DocPane.svelte`, `frontend/src/modules/lark/doc/MarkdownView.svelte`, `frontend/src/modules/lark/doc/SheetView.svelte`, `frontend/src/modules/lark/doc/docClient.ts`, `frontend/package.json`
Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini. Catatan: `frontend/package.json` hanya boleh diubah oleh fitur ini di dalam epic ini; lockfile (mis. `package-lock.json`) ikut berubah sebagai efek instalasi dependensi dan itu diizinkan.

### lark-wiki-wiring: Wiring wiki dan ctl
Deskripsi:
- Alat debug `shikigamictl lark wiki list` (`--workspace`, `--source` atau `personal`, `--parent`) dan `lark wiki get` (`--workspace`, `--source`, `--node`) di `cmd/shikigamictl/cmd_lark_wiki.go`, memanggil IPC `lark.wiki.list`/`lark.wiki.get` lewat klien daemon yang dipakai perintah ctl lain, mencetak JSON atau tabel ringkas. Subperintah dipasang lewat mekanisme registrasi subcommand `lark` yang disediakan tiket 01 di `cmd_lark.go` (tanpa mengedit file itu; bila registrasi lewat `init()`, ikuti pola yang sama).
- Memverifikasi wiring end-to-end (lihat bagian Wiring): `WikiSection` memakai `WikiTree` baru dan klik node membuka pane `lark-doc`; `wire_lark_wiki.go` terdaftar di daemon (lewat `init()` atau pola registrasi `wire_lark*.go` yang dipakai tiket 02; tiap tiket mendaftarkan IPC di file wire sendiri).
Acceptance criteria:
- [ ] `go build ./...` lulus, `shikigamictl lark wiki list|get` muncul di bantuan.
- [ ] `shikigamictl lark wiki list --source personal` memanggil daemon dan mencetak node.
- [ ] Verifikasi manual alur Wiring di bawah berhasil.
ownedPaths: `cmd/shikigamictl/cmd_lark_wiki.go`
Paralel: bagian ctl bisa dikerjakan paralel dengan fitur lain; verifikasi akhir (langkah Wiring) dikerjakan setelah tiga fitur lain selesai.

## Path yang dimiliki
Hanya path berikut yang boleh disentuh oleh tiket ini:
- `internal/larkdata/wiki.go`
- `internal/larkdata/doc.go`
- `cmd/shikigamid/wire_lark_wiki.go`
- `cmd/shikigamictl/cmd_lark_wiki.go`
- `frontend/src/modules/lark/doc/` (DocPane.svelte, MarkdownView.svelte, SheetView.svelte, docClient.ts)
- `frontend/src/modules/lark/sections/WikiTree.svelte`
- `frontend/src/modules/lark/wikiTreeClient.ts`
- `frontend/package.json` (dan lockfile sebagai efek instalasi dependensi)

Bila perlu perubahan di luar daftar (mis. perintah belum ada di allowlist, `WikiSection` perlu diubah), jangan diubah: laporkan sebagai temuan/pemotongan.

## Wiring
Langkah penutup tiket ini, memakai titik ekstensi yang sudah dibuat tiket 02 (tanpa menunggu tiket 03 dan tanpa mengedit file tiket lain):
1. Pastikan `WikiSection` (tiket 02) sudah merender `WikiTree` dengan props `{source, onOpen}`. Karena placeholder `WikiTree` ditimpa di tempat yang sama dan kontrak props tetap, tidak ada edit di `WikiSection`; cukup verifikasi bahwa `onOpen(nodeToken)` membuka pane `lark-doc` `{sourceId|'personal', nodeToken}` (mengganti pane `lark-doc` yang sudah terbuka bila itu perilaku `WikiSection`, tanpa mengubahnya).
2. Pastikan `wire_lark_wiki.go` terdaftar di daemon (build dan jalankan daemon, panggil `lark.wiki.list` lewat ctl).
3. Verifikasi build dan jalan: `go build ./...` dan build frontend lulus; jalankan aplikasi dan: buka wiki Pribadi, jelajah pohon beberapa level, buka satu docx (markdown ter-render) dan satu sheet (tabel sel), buka node tipe lain (judul + link), tekan Salin link (clipboard terisi), tekan Kirim ke agen dengan sesi terminal aktif (link tertempel tanpa Enter) dan tanpa sesi aktif (toast "Tidak ada sesi aktif").
4. Bila temuan spike (`docs/lark/cli-findings.md`) membantah asumsi (mis. `my_library` tak bisa di-list, `sheets +cells-get` beda flag), potong fitur terkait (wiki Pribadi disembunyikan dengan keterangan, sheet menjadi judul + link), laporkan, dan lanjutkan sisa pekerjaan.
Tiket ini bisa dijalankan sendiri setelah tiket 02 selesai; tidak bergantung pada tiket 03.

## Guard
Cukup build berhasil, tanpa unit test:
- `go build ./...`
- Build frontend (mis. `npm run build` di `frontend/`, termasuk pemeriksaan tipe `svelte-check` bila skrip tersedia).
