# Connector Lark (tiket Base dan wiki) per workspace

> Menumpang pada **rail menu utama** (`2026-11-22-08-main-rail-design.md`): Lark adalah satu
> item rail dengan panel sendiri. GitLab dan GitHub (MR/PR, pipeline, environment
> variable) didesain terpisah.

Bagian 4 dari fitur Connector (bagian 1 = kerangka, lihat
`2026-11-22-settings-framework-design.md`). Adapter nyata pertama di atas kerangka itu.

## Maksud dan alasan
User bekerja dengan agen di Shikigami sambil merujuk tiket (Lark Base) dan dokumen
(Lark Wiki: docs dan sheet). Sekarang untuk melihatnya harus keluar app. Tujuan: melihat
daftar dan detail tiket serta wiki di dalam app, lalu **menyalin link** atau
**mengirim link ke agen** sebagai konteks. Agen sudah punya skill `lark-*` dan CLI
untuk membaca isi link sendiri, jadi app cukup mengirim link, bukan isinya.

Sumber tiket dan wiki berbeda per proyek, jadi sumbernya diatur **per workspace**
(satu workspace biasanya satu produk atau tim).

## Tujuan
- Connector `lark` terdaftar di kerangka Connector; status memakai `lark-cli` yang sudah
  login (tidak ada login atau token di app).
- Item rail **Lark** dengan panel dua bagian: **Tiket** dan **Wiki**. Isi mengikuti
  workspace aktif dan berganti saat workspace berganti.
- **Tiket**: panel menampilkan daftar Base yang ditambahkan manual. Memilih satu Base
  membuka pane di area tengah berisi tiketnya sebagai **grid atau kanban** (bisa
  dialihkan). Klik tiket membuka detailnya (laci di pane yang sama).
- **Wiki**: panel menampilkan daftar wiki. **Wiki pribadi (`my_library`) selalu ada**
  begitu connector tersambung, tanpa ditambahkan; wiki tim ditambahkan manual. Tiap wiki
  berupa pohon node yang dibuka per level (lazy). Memilih node docs atau sheet membuka
  isinya di pane tengah.
- Pane tiket dan pane dokumen punya tombol **Salin link** dan **Kirim ke agen**.
- Base dan wiki ditambahkan lewat tombol **Tambah** di panel (bukan modal global), dengan
  cara menempel link.

## Bukan tujuan
- Read-only. Tidak ada membuat atau mengubah tiket, dokumen, atau sheet. Kanban tidak
  menyeret kartu untuk mengubah status.
- Tidak menyalin atau menyimpan isi tiket/dokumen secara permanen (tanpa cache disk).
- Tidak menyisipkan isi dokumen ke sesi agen; hanya link.
- Tidak ada tampilan Base lain selain grid/kanban record (tanpa dashboard, form, workflow).
- Tidak ada connector chat, kalender, atau fitur Lark lain.
- GitLab dan GitHub tidak dibangun di sini; rail dibangun di desain terpisah.
- Tidak ada konfigurasi lewat file di repo (`.shikigami/lark.json`); ditolak, lihat Pendekatan.
- Tidak ada penelusuran otomatis "semua Base/wiki akun" (selain wiki pribadi).

## Anti-tujuan
- Menyimpan token atau kredensial Lark di config app.
- Mengganggu sesi terminal yang berjalan saat panel Lark dimuat atau saat `lark-cli` lambat.
- Menulis apa pun ke dalam folder project user.
- Memanggil `lark-cli` dengan perintah berisiko tulis (hanya `read`).

## Pendekatan yang dipertimbangkan
Sumber data (dari mana daftar diambil):
- (A) Sumber ditambahkan manual; (B) jelajah semua yang bisa diakses; (C) hanya tempel link.
  **Dipilih A** oleh user (dengan satu pengecualian: wiki pribadi otomatis). Lark tidak punya
  "semua tiket saya"; B lambat dan berisik, C tidak punya daftar.

Tempat menyimpan sumber:
- (1) per workspace di `WorkspaceConfig`; (2) file di repo per folder project;
  (3) keduanya. **Dipilih 1** oleh user: polanya sudah ada (Grimoire), tidak menulis ke
  repo, dan satu workspace = satu produk. Opsi 3 bisa ditambah belakangan tanpa merombak.

Tempat menu:
- **Rail ikon paling kiri** (usul user), panel di kolom kedua, hasil sebagai pane tengah.
  Menggantikan usul awal tab di panel kanan.

## Desain

### Konfigurasi (per workspace)
`WorkspaceConfig` (`internal/workspace/types.go`) ditambah:
```go
Lark *LarkConfig `json:"lark,omitempty"`

type LarkConfig struct {
    Tickets []TicketSource `json:"tickets"` // boleh beberapa
    Wiki    []WikiSource   `json:"wiki"`
}
type TicketSource struct {
    ID        string `json:"id"`        // ULID
    Name      string `json:"name"`      // label di UI
    BaseToken string `json:"baseToken"` // diurai dari link Base yang ditempel
    Host      string `json:"host"`      // host link asli, untuk membangun URL
    TableID   string `json:"tableId,omitempty"` // tabel terakhir dipilih (default saat dibuka)
    ViewID    string `json:"viewId,omitempty"`  // view terakhir dipilih
}
type WikiSource struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    SpaceID   string `json:"spaceId"`
    RootToken string `json:"rootToken,omitempty"` // kosong = akar space
    Host      string `json:"host"`
}
```
Hanya ID dan URL, tanpa token. Wiki pribadi tidak disimpan (selalu ada, `spaceId`
`my_library`). Sumber tiket = satu Base; tabel dan view dipilih di header pane. Workspace
tanpa Base menampilkan keadaan kosong "Belum ada Base. Tambah Base." Perubahan dipublikasikan sebagai `configChanged`
seperti config Grimoire sehingga panel ikut menyegarkan diri.

### Go
- `internal/connector/lark.go`: adapter, `Register` dari `init()`. `Status()` memanggil
  `lark-cli auth status` (JSON). `cli_missing` bila CLI tidak ada; `not_logged_in` bila
  identitas `user` tidak tersedia; selain itu `connected` dengan `User` = `userName`.
  Token berstatus `needs_refresh` dihitung tersambung (CLI menyegarkan sendiri).
  Panduan Connect: `lark-cli auth login` dan cek ulang.
- Paket data baru (mis. `internal/larkdata`) membungkus `connector.RunCLI`, hanya
  perintah `read`, selalu dengan `--format json` / `--json`:
  - tabel: `base +table-list`; view: `base +view-list`
  - daftar tiket: `base +record-list` (`--base-token --table-id [--view-id] --limit --offset`)
  - detail tiket: `base +record-get`
  - wiki: `wiki +space-list`, `wiki +node-list` (`--space-id --parent-node-token`), `wiki +node-get`
  - isi dokumen: `docs +fetch --doc-format markdown`
  - isi sheet: `sheets +cells-get` (rentang awal terbatas, mis. 50 baris x 20 kolom)
  - Timeout daftar lebih longgar dari default `RunCLI` (usul 20 detik).
- Parser link: dari link Base/Wiki yang ditempel user diambil token, host, table, view.
- IPC (terdaftar di daemon lewat `wire_lark.go`):
  `lark.sources.get` {workspaceId} dan `lark.sources.add`/`.remove`/`.rename` (lihat putaran 2), `lark.tickets.list` {workspaceId, sourceId, tableId, viewId, offset},
  `lark.ticket.get` {workspaceId, sourceId, recordId}, `lark.wiki.list` {workspaceId, sourceId|"personal", parentToken},
  `lark.wiki.get` {workspaceId, sourceId, nodeToken}, `lark.tables.list` / `lark.views.list`
  (untuk header board dan dialog Tambah Base). Hasil dinormalisasi (bukan JSON mentah CLI) agar frontend stabil.
- `shikigamictl lark ...` sebagai alat debug (list/get), sejalan dengan `connector` di tiket 2.

### Frontend (`frontend/src/modules/lark/`)
- Mendaftarkan item rail `lark` (`registerRailItem`, lihat desain rail) dengan panel
  `LarkPanel`, dan dua jenis pane lewat `registerPaneKind`: `lark-board` dan `lark-doc`.
- **LarkPanel**: dua bagian yang bisa dilipat, **Tiket** dan **Wiki**.
  - Tiket: daftar Base (nama + ikon), tombol **Tambah Base**, hapus/ganti nama per item.
  - Wiki: **Pribadi** di paling atas, lalu wiki tim yang ditambahkan; tiap wiki berupa
    pohon lazy (ikon menurut tipe: docx, sheet, lainnya); tombol **Tambah wiki**.
  - Keadaan: memuat, kosong, galat terbaca (CLI belum terpasang / belum login / izin
    kurang / timeout) dengan ajakan menuju Pengaturan > Connector bila relevan; tombol segarkan.
- **Pane `lark-board`** (`{ sourceId, tableId?, viewId? }`): header berisi pemilih tabel
  dan view, kontrol **Grid | Kanban**, kotak cari (filter lokal pada data termuat), tombol
  muat lagi, **Salin link** dan **Kirim ke agen** untuk Base/view. Grid = tabel field.
  Kanban = kolom per nilai satu field single-select (default: field single-select
  pertama; pemilih field di header); tanpa field single-select, kanban dinonaktifkan
  dengan keterangan. Judul kartu = field utama tabel; satu-dua field tambahan sebagai
  metadata. Klik kartu/baris membuka **laci detail** (daftar nama: nilai) dengan
  **Salin link** dan **Kirim ke agen** untuk tiket itu.
- **Pane `lark-doc`** (`{ sourceId | 'personal', nodeToken }`): docx dirender dari
  markdown; sheet sebagai tabel sel awal; tipe lain (bitable, mindnote, dst): judul dan
  link dengan keterangan "buka di Lark". Header: **Salin link** dan **Kirim ke agen**.
- **Kirim ke agen**: `pasteToSession` ke sesi aktif (jalur sama dengan drag-drop path
  file), tanpa Enter otomatis. Tanpa sesi aktif: toast "Tidak ada sesi aktif".
- Dialog **Tambah Base / Tambah wiki**: tempel link -> app mengurai token dan host; untuk
  Base ditawarkan nama (default dari Base), untuk wiki space atau node dijadikan akar.
  Boleh beberapa; bisa dihapus dan diberi nama.

### Link
- Tiket: URL Base milik sumber dengan parameter tabel/view/record. **Format persis
  parameter record belum diverifikasi**; builder memastikan dari perilaku Lark, dengan
  fallback link ke tabel/view bila format record tak bisa dipastikan.
- Wiki: URL node dari `wiki +node-get` bila ada, atau `https://<host>/wiki/<node_token>`
  dengan `host` dari sumber.

### Galat dan batas
- Semua panggilan lewat `RunCLI` (PATH login-shell, timeout, stderr dipotong). Pesan galat
  UI dalam bahasa Indonesia; identifier kode dalam bahasa Inggris.
- Daftar dibatasi per halaman (usul 50 untuk tiket); wiki per level (`--page-size 50`).
- Tidak ada polling otomatis; segarkan saat panel/pane dibuka, saat sumber/workspace berganti,
  dan lewat tombol segarkan.

## Panggilan saya (belum disetujui user, silakan koreksi)
- Banyak Base dan wiki per workspace (list), bukan satu.
- Kanban dikelompokkan menurut satu field single-select; kolom tanpa nilai di kolom "Tanpa nilai".
- Laci detail tiket di dalam pane board, bukan pane terpisah.
- Judul tiket = field utama tabel; field tambahan (maks dua) dipilih di header board; rincian UI ke builder.
- Cari = filter lokal pada data yang sudah dimuat (tanpa `+record-search` di tahap ini).
- Detail sheet hanya rentang awal; bukan penampil sheet penuh.
- "Kirim ke agen" menempel link tanpa menekan Enter.
- Timeout daftar 20 detik.

## Hasil pemeriksaan builder (panggilan saya, silakan koreksi)
- **Tiket pertama = spike CLI dengan fixture.** Flag untuk `base +record-list`,
  `+record-get`, `+table-list`, `wiki +node-list`, `+node-get`, dan `docs +fetch` sudah
  dicocokkan dengan `--help` (lark-cli 1.0.97). `base +view-list` dan `sheets +cells-get`
  baru terbukti ada, flag-nya belum. Bentuk JSON (field utama, tipe field, nilai select/
  user/link/lampiran, kursor paginasi, `obj_type`/`obj_token` node wiki, URL node) direkam
  sebagai fixture di spike; parser `larkdata` dibangun dari fixture itu. Fixture dianonimkan
  sebelum di-commit.
- **Allowlist di kode.** `larkdata` hanya boleh memanggil daftar perintah baca yang
  tertulis eksplisit; ada test yang menolak perintah di luar daftar.
- **Identitas selalu `--as user`** pada setiap panggilan (akses dan `my_library` mengikuti
  user, bukan bot). Galat izin/scope kurang jadi keadaan UI sendiri: "Izin Lark kurang"
  dengan pesan dari CLI.
- **Skema `lark-cli auth status`**: `identities.user.{status, available, userName}`;
  `connected` bila `identities.user.available` true. Dicek ulang di spike.
- **Pane `lark-board` dan `lark-doc` hanya menyimpan referensi** (sourceId, tableId/viewId, nodeToken). Saat dipulihkan
  isi diambil ulang; bila sumber sudah dihapus atau jaringan gagal, pane menampilkan
  keadaan galat yang aman, bukan crash.
- **"Kirim ke agen" hanya untuk sesi terminal** lewat `pasteToSession` (sama dengan
  drag-drop). Peserta thread tidak termasuk. Pane `lark-*` ditambahkan **setelah** perubahan
  `layout/*` yang sedang berjalan di working tree masuk, agar tidak bentrok.
- **Link Base di bawah wiki (`/wiki/...` berisi bitable) didukung**: parser memanggil
  `wiki +node-get` untuk mendapat `obj_token` Base bila link berbentuk wiki.
- Paginasi dan ukuran rentang sheet (50 x 20) adalah konstanta, bukan setelan.

## Hasil pemeriksaan builder, putaran 2 (panggilan saya, silakan koreksi)
- **Aturan bila spike CLI membantah sesuatu**: fitur yang bergantung padanya dipotong dan
  dilaporkan, pekerjaan lain jalan terus. Contoh: tanpa tipe field single-select di
  output, kanban dinonaktifkan (grid tetap); bila `my_library` tak bisa di-list, wiki
  pribadi disembunyikan dengan keterangan; bila `--as user` tak diterima di satu
  perintah, perintah itu dicatat sebagai pengecualian. `lark-cli` sudah login di mesin
  ini, jadi spike bisa merekam fixture asli.
- **Link tiket yang tidak bisa dipastikan formatnya**: tombol Salin link dan Kirim ke agen
  **tingkat tiket dinonaktifkan** (dengan keterangan), tombol tingkat Base/view tetap
  aktif. Mengirim link tabel yang tidak menunjuk tiket dianggap menyesatkan.
- **Penyimpanan config = operasi per item**: IPC `lark.sources.add`, `.remove`, `.rename`
  (bukan mengganti seluruh `LarkConfig`), ID dibuat di Go (ULID); dua edit bersamaan tidak
  saling menimpa. Menggantikan `lark.sources.get/set` di daftar IPC di atas (tetap ada
  `get`).
- **Pemulihan pane**: loader layout menerima jenis pane apa pun, tetapi hanya menyalin
  daftar field tetap (`sessionId`, `projectId`, `filePath`, `threadId`, dst.). Field
  referensi Lark (`sourceId`, `tableId`, `viewId`, `nodeToken`) ditambahkan ke daftar itu.
  Jenis pane tak dikenal (build tanpa modul Lark) dirender sebagai keadaan "pane tidak
  dikenal", tidak dibuang diam-diam.
- **Pane yang dibuka lalu workspace berganti**: pane tetap terbuka; bila `sourceId` tidak
  ditemukan di workspace itu, pane menampilkan "Sumber tidak ada di workspace ini".
- **Banyak tenant Lark/Feishu**: `lark-cli` melayani satu tenant per profil. Sumber dari
  tenant lain tampil sebagai galat izin; dicatat sebagai batas yang diketahui, tanpa
  deteksi khusus.

## Diserahkan ke builder
- Cara menyimpan `LarkConfig`: `config.set` di `internal/workspace/ipc.go` saat ini
  memetakan field per field, jadi perlu IPC khusus atau perluasan; builder memilih yang
  paling kecil dan memastikan tidak merusak config Grimoire.
- Arah dependensi paket (`internal/connector`, `internal/larkdata`, `internal/workspace`).
- Cara mendaftarkan jenis pane `lark-board` dan `lark-doc` di sistem layout dan persist-nya (pane yang
  dipersist harus bisa dipulihkan atau jatuh ke keadaan galat yang aman).
- Pemilihan komponen render markdown yang sudah ada di frontend (bila ada).
- Pemecahan menjadi tiket (go + frontend) dan urutan wave.
