---
id: lark-connector-ticket-03-lark-tickets
title: Tiket Base grid dan kanban
spec: docs/superpowers/specs/2026-11-22-09-lark-connector-design.md
status: implemented
wave: 3
depends_on: [lark-connector-ticket-02-lark-sources]
---
# Tiket Base grid dan kanban

## Tujuan
Tiket 2 (`lark-sources`) sudah menyediakan rail Lark, daftar sumber Base per workspace, dan pane `lark-board` yang masih berupa placeholder. Epic ini mengisi pane itu: menampilkan tiket dari Lark Base sebagai grid atau kanban, lengkap dengan pemilih tabel/view, cari lokal, laci detail tiket, serta tombol Salin link dan Kirim ke agen. Tujuannya agar user bisa melihat daftar dan detail tiket tanpa keluar app, lalu menyalin atau mengirim link ke agen sebagai konteks. Semua read-only dan semua data lewat `lark-cli` yang sudah login.

## Hasil yang bisa dijalankan
Di atas tiket 1 dan 2, setelah tiket ini selesai:
- Klik sebuah Base di bagian Tiket panel Lark membuka pane `lark-board` berisi tiket Base itu (bukan placeholder lagi).
- Header pane: pemilih tabel dan view, kontrol Grid | Kanban, kotak cari, tombol muat lagi, tombol segarkan, Salin link dan Kirim ke agen untuk Base/view, serta pemilih field tambahan (maksimal dua).
- Grid menampilkan tabel field (kolom per field, baris per tiket). Kanban menampilkan kolom per nilai field single-select (default field single-select pertama, ada pemilih field, kolom "Tanpa nilai" untuk tiket tanpa nilai). Judul kartu = field utama tabel. Bila tabel tak punya field single-select, tombol Kanban nonaktif dengan keterangan dan grid tetap jalan.
- Kotak cari menyaring tiket yang sudah dimuat (lokal). "Muat lagi" mengambil halaman berikutnya (50 tiket per halaman) bila `nextOffset` ada.
- Klik kartu/baris membuka laci detail di dalam pane: daftar `nama: nilai`, tombol Salin link dan Kirim ke agen tingkat tiket. Bila format link record tidak pasti, kedua tombol tingkat tiket nonaktif dengan keterangan, sedangkan tombol tingkat Base/view tetap aktif.
- Salin link menyalin ke clipboard; Kirim ke agen menempel link ke sesi terminal aktif tanpa Enter; tanpa sesi aktif muncul toast "Tidak ada sesi aktif".
- Pane yang dipersist dipulihkan dengan mengambil ulang data; sumber terhapus atau jaringan gagal menampilkan keadaan galat yang aman, bukan crash.
- `shikigamictl lark tickets list` dan `lark tickets get` berjalan sebagai alat debug.
- `go build ./...` dan build frontend lulus; aplikasi berjalan.

## Batasan
In scope:
- Normalisasi data tiket (`base +record-list`, `+record-get`), deteksi field single-select, field utama, pembangun link tiket.
- IPC `lark.tickets.list` dan `lark.ticket.get`; persist tableId/viewId terakhir lewat operasi per item yang sudah ada dari tiket 2.
- UI pane `lark-board` (grid, kanban, header, laci detail) dan `boardClient.ts`.
- Subcommand `shikigamictl lark tickets list/get`.
- Menerapkan aturan potong spike (`docs/lark/cli-findings.md`): kanban nonaktif tanpa single-select, link tiket nonaktif bila format record tak pasti, pengecualian `--as user` mengikuti catatan spike.

Out of scope:
- Membuat/mengubah tiket; drag kartu kanban untuk ubah status.
- Cache disk, `+record-search`, tampilan Base lain (dashboard, form, workflow).
- Mengubah kerangka Lark, rail, LarkPanel, store/client/actions, dialog Tambah Base (milik tiket 2); `runner.go`, `allowlist.go`, `tables.go`, `link.go` (milik tiket 1 dan 2). Bila `+record-list`/`+record-get` belum ada di allowlist, laporkan; jangan mengubah file allowlist.
- Wiki dan dokumen (tiket 4), `frontend/package.json`.
- Peserta thread sebagai target "Kirim ke agen".
- Unit test dan dependensi baru.

## Kontrak
Disediakan (IPC, hasil dinormalisasi; galat memakai kode connector yang sama dengan pesan Indonesia):

`lark.tickets.list` params `{workspaceId, sourceId, tableId, viewId?, offset}` hasil:
```ts
{
  fields: { id: string; name: string; type: string; isPrimary: boolean }[];
  records: { id: string; title: string; values: Record<string, string> }[]; // key = field id, nilai = teks tampilan
  nextOffset?: number; // tidak ada bila halaman terakhir
}
```
Konstanta Go `pageSize = 50`. Nilai select/user/link/lampiran/tanggal/centang dijadikan teks tampilan (multi-nilai digabung `, `). `type` memakai nilai tetap hasil normalisasi; tipe single-select dipetakan ke literal `"single_select"` (nama literal final mengikuti temuan spike, tetapi harus satu konstanta Go `FieldTypeSingleSelect` dan satu konstanta TS yang sama).

`lark.ticket.get` params `{workspaceId, sourceId, recordId}` hasil:
```ts
{ fields: { name: string; text: string }[]; url?: string; recordLinkAvailable: boolean }
```
Bila `recordLinkAvailable=false`, `url` boleh berisi link tabel/view (untuk keperluan tampilan), tetapi UI tiket tidak boleh memakainya untuk Salin/Kirim tingkat tiket. `tableId`/`viewId` untuk membangun link berasal dari sumber (`TicketSource.TableID/ViewID`) atau, bila perlu, parameter opsional `tableId`/`viewId` tambahan pada `lark.ticket.get` (opsional; frontend mengirim yang aktif di pane).

Go (`internal/larkdata`):
- `ListTickets(ctx, src workspace-neutral params) (TicketPage, error)` dan `GetTicket(...) (TicketDetail, error)`; tipe `Field`, `Record`, `TicketPage`, `TicketDetail` mengikuti bentuk di atas (json camelCase).
- `BuildTicketLink(host, baseToken, tableID, viewID, recordID string) (url string, recordLinkAvailable bool)`; bila format record tak pasti, `recordLinkAvailable=false` dan `url` = link tabel/view.
- Semua panggilan lewat `Run()` di `runner.go` (tiket 1), selalu `--as user` dan format json; timeout daftar 20 detik; arah dependensi `larkdata -> connector`, `larkdata` tidak mengimpor `workspace` (nilai sumber diteruskan sebagai argumen biasa).

`cmd/shikigamid/wire_lark_tickets.go` mengimpor `larkdata` dan `workspace`, mencari sumber (`TicketSource`) lewat ID pada config workspace, lalu mendaftarkan kedua IPC dengan pola pendaftaran yang sama dengan `wire_lark.go`. Sumber tidak ditemukan -> galat terbaca "Sumber tidak ada di workspace ini". Menyimpan tableId/viewId terakhir memakai operasi per item dari tiket 2 (bukan mengganti seluruh `LarkConfig`).

Frontend:
- `boardClient.ts` mengekspor `listTickets(params)`, `getTicket(params)` (pembungkus IPC bertipe), memakai `lark.tables.list`/`lark.views.list` (tiket 2) untuk pemilih; tipe TS sama dengan kontrak di atas.
- `TicketDrawer.svelte` props: `{ source, tableId, viewId?, recordId, onClose }`; memanggil `getTicket`.
- `Grid.svelte` props: `{ fields, records, extraFieldIds?, onOpen(recordId) }`. `Kanban.svelte` props: `{ fields, records, groupFieldId, extraFieldIds, onOpen(recordId) }`. `BoardHeader.svelte` props/callback mengikuti kebutuhan header di atas.
- `BoardPane.svelte` tetap titik ekstensi dari tiket 2 (props pane `{sourceId, tableId?, viewId?}`).

Dipakai (periksa nama persis di kode):
- Dari tiket 1: `larkdata.Run()` (runner.go), allowlist perintah baca, `docs/lark/cli-findings.md`, fixture di `internal/larkdata/testdata/`.
- Dari tiket 2: `lark.tables.list`/`lark.views.list`, operasi `lark.sources.*`, `frontend/src/modules/lark/client.ts` (config sumber), `store.ts` (sumber workspace aktif), `actions.ts` (`copyLink(url)`, `sendToAgent(url)`), `BoardPane.svelte` placeholder, jenis pane `lark-board` dan field persist `sourceId/tableId/viewId`, event `configChanged`.

Aturan tetap: teks UI bahasa Indonesia, identifier kode bahasa Inggris; tanpa token di config; tanpa polling otomatis (segarkan saat pane dibuka/dipulihkan, sumber/tabel/view berganti, dan lewat tombol segarkan).

## Fitur

### lark-tickets-data: Data tiket larkdata dan IPC
Deskripsi:
Baca `docs/lark/cli-findings.md` dan fixture `internal/larkdata/testdata/` dari tiket 1; parser dibangun dari fixture, bukan tebakan.
- `internal/larkdata/tickets.go`: panggil `base +record-list` (`--base-token --table-id [--view-id] --limit 50 --offset N`) dan `base +record-get`; normalisasi ke `TicketPage`/`TicketDetail`. Daftar field dengan tipe (deteksi single-select), tentukan field utama tabel (`isPrimary`) menurut temuan spike, judul record = nilai field utama (kosong -> "(Tanpa judul)"). Nilai select/user/link/lampiran/tanggal/boolean/angka diubah ke teks tampilan. Hitung `nextOffset` dari kursor/jumlah menurut bentuk paginasi di fixture (tidak ada bila halaman terakhir). Untuk `ticket.get`, `fields` berupa daftar `{name, text}` berurutan menurut urutan field tabel, field kosong boleh dilewati.
- `internal/larkdata/ticketlink.go`: `BuildTicketLink` memakai host sumber, `baseToken`, `tableId`, `viewId`, `recordId` sesuai format URL Base dari spike. Bila spike tidak memastikan parameter record, kembalikan `recordLinkAvailable=false` dan link tabel/view.
- `cmd/shikigamid/wire_lark_tickets.go`: daftarkan `lark.tickets.list` dan `lark.ticket.get` (validasi parameter, cari sumber, panggil larkdata, ubah galat ke kode connector + pesan Indonesia termasuk "Izin Lark kurang" dengan pesan CLI, timeout, "Sumber tidak ada di workspace ini"). Simpan `tableId`/`viewId` terakhir ke sumber lewat operasi per item bila berubah (best effort; kegagalan simpan tidak menggagalkan list).
- Terapkan pengecualian `--as user` dan batas spike seperti dicatat di findings.
- Tanpa unit test; guard hanya build.

Acceptance criteria:
- [ ] `lark.tickets.list` mengembalikan `fields`, `records`, `nextOffset?` sesuai kontrak untuk tabel nyata, dengan 50 record per halaman dan `offset` bekerja.
- [ ] Field single-select terdeteksi lewat `type` dan satu konstanta bersama; tepat satu field berflag `isPrimary` (atau nol bila spike menyatakan tak ada, dengan fallback field pertama).
- [ ] Nilai select, user, link, lampiran muncul sebagai teks terbaca, bukan JSON mentah.
- [ ] `lark.ticket.get` mengembalikan `fields[{name,text}]`, `url?`, `recordLinkAvailable`; bila format record tak pasti, flag false.
- [ ] Setiap panggilan CLI lewat `Run()` dengan `--as user`, format json, timeout daftar 20 detik; tidak ada perintah di luar allowlist.
- [ ] Galat CLI (belum login, izin kurang, timeout, sumber tidak ada) menjadi galat IPC berpesan Indonesia, tanpa panic.
- [ ] `larkdata` tidak mengimpor `internal/workspace`; `go build ./...` lulus.

ownedPaths:
- `internal/larkdata/tickets.go`
- `internal/larkdata/ticketlink.go`
- `cmd/shikigamid/wire_lark_tickets.go`

Catatan: bisa dikerjakan paralel dengan `lark-board-ui` dan `lark-ticket-drawer` (kontrak IPC di atas adalah antarmuka bersama; frontend memakai tipe yang sama tanpa menunggu backend). Pendaftaran `wire_lark_tickets.go` ke daemon mengikuti pola registrasi mandiri `wire_lark.go` (tiap file wire_lark*.go mendaftarkan IPC-nya sendiri); jangan mengubah file di luar ownedPaths, dan laporkan bila pola itu ternyata butuh perubahan di file lain.

### lark-board-ui: Pane board grid dan kanban
Deskripsi:
Timpa placeholder `board/BoardPane.svelte` dengan pane lengkap, kecuali titik sambung laci (dikerjakan di `lark-tickets-wiring`; di fitur ini cukup menyediakan callback `onOpenRecord(recordId)` dan state `selectedRecordId`, tanpa mengimpor `TicketDrawer`).
- `boardClient.ts`: pembungkus IPC bertipe untuk `lark.tickets.list`, `lark.ticket.get`, ditambah `lark.tables.list` dan `lark.views.list`.
- `BoardHeader.svelte`: pemilih tabel dan view (muat saat pane dibuka; ganti tabel -> reset view dan data), kontrol Grid|Kanban, kotak cari, tombol "Muat lagi" (aktif bila `nextOffset`), tombol segarkan, Salin link dan Kirim ke agen untuk Base/view (link dari `BuildTicketLink`-nya tabel/view bila tersedia dari sumber; bila belum ada, bangun dari `https://<host>/base/<baseToken>?table=<tableId>&view=<viewId>` sesuai temuan spike, atau nonaktifkan dengan keterangan), pemilih field tambahan (maks dua, `fields` selain judul), pemilih field kanban.
- `Grid.svelte`: tabel field; kolom judul tetap di kiri, baris klik memanggil `onOpen`; baris disaring oleh kotak cari (cocokkan teks semua nilai tampilan, case-insensitive).
- `Kanban.svelte`: kolom per nilai unik field single-select ber-urutan kemunculan (atau urutan opsi bila tersedia), kolom "Tanpa nilai" untuk nilai kosong; kartu berisi judul (field utama) dan maksimal dua field tambahan sebagai metadata; kartu klik memanggil `onOpen`. Kanban dinonaktifkan dengan keterangan "Tabel ini tidak punya field single-select" bila tak ada field `single_select`.
- `BoardPane.svelte`: menerima props pane `{sourceId, tableId?, viewId?}`, mengambil sumber dari store tiket 2, menentukan tabel/view awal (props -> `TicketSource.TableID/ViewID` -> tabel pertama), menyimpan pilihan tabel/view kembali ke props pane (agar persist) dan, lewat operasi per item, ke sumber. Keadaan: memuat, kosong ("Tidak ada tiket"), galat terbaca dengan tombol coba lagi (termasuk "Sumber tidak ada di workspace ini" dan jaringan gagal), tidak crash. Ambil ulang saat mount (termasuk pemulihan), saat `sourceId`/tabel/view berganti, dan lewat segarkan. Salin link/Kirim memakai `copyLink`/`sendToAgent` dari `actions.ts` (tiket 2).
- Stale response: abaikan respons yang datang setelah tabel/view berganti.

Acceptance criteria:
- [ ] Pane memuat tiket Base: tabel/view bisa diganti, data diambil ulang, halaman berikutnya ditambahkan lewat "Muat lagi".
- [ ] Grid menampilkan field sebagai kolom dan tiket sebagai baris; judul = field utama.
- [ ] Kanban mengelompokkan menurut field single-select terpilih (default yang pertama), ada kolom "Tanpa nilai", kartu menampilkan judul dan maks dua field tambahan; nonaktif dengan keterangan bila tak ada single-select.
- [ ] Kotak cari menyaring data termuat secara lokal tanpa panggilan IPC.
- [ ] Salin link/Kirim ke agen tingkat Base/view bekerja; tanpa sesi aktif muncul toast "Tidak ada sesi aktif".
- [ ] Keadaan memuat/kosong/galat aman; sumber terhapus atau jaringan gagal tidak membuat pane crash; pane dipulihkan mengambil ulang data.
- [ ] Respons usang (setelah ganti tabel/view) tidak menimpa data baru.
- [ ] Build frontend lulus.

ownedPaths:
- `frontend/src/modules/lark/board/BoardPane.svelte`
- `frontend/src/modules/lark/board/Grid.svelte`
- `frontend/src/modules/lark/board/Kanban.svelte`
- `frontend/src/modules/lark/board/BoardHeader.svelte`
- `frontend/src/modules/lark/board/boardClient.ts`

Catatan: bisa dikerjakan paralel dengan `lark-tickets-data` dan `lark-ticket-drawer`. `BoardPane.svelte` juga dipakai `lark-tickets-wiring` sesudah fitur ini selesai (lihat Wiring), jadi fitur lain tidak boleh menyentuh file itu.

### lark-ticket-drawer: Laci detail tiket
Deskripsi:
Komponen `board/TicketDrawer.svelte`, laci di dalam pane board (panel meluncur dari kanan di dalam area pane, bukan modal global).
- Props: `{ source, tableId, viewId?, recordId, onClose }`. Saat `recordId` berubah, ambil `lark.ticket.get` lewat `getTicket` dari `boardClient.ts`. Karena `boardClient.ts` dimiliki `lark-board-ui`, komponen ini memakai tipe/fungsi itu sebagai kontrak (`getTicket(params): Promise<{fields:{name,text}[]; url?:string; recordLinkAvailable:boolean}>`); bila belum ada saat dikerjakan paralel, impor sesuai kontrak dan jangan membuat salinan sendiri.
- Isi: daftar `nama: nilai` (nilai panjang boleh dibungkus, teks multi-baris dipertahankan), keadaan memuat/galat (dengan coba lagi), tombol tutup (juga Esc).
- Tombol Salin link dan Kirim ke agen tingkat tiket memakai `url` dan `copyLink`/`sendToAgent` dari `actions.ts` (tiket 2). Bila `recordLinkAvailable=false`, kedua tombol nonaktif dengan keterangan "Link tiket tidak bisa dipastikan"; jangan pernah mengirim link tabel sebagai link tiket.
- Style mengikuti token tema yang ada (terang/gelap); keyboard dapat diakses; tidak menyimpan data ke disk.

Acceptance criteria:
- [ ] Laci menampilkan daftar `nama: nilai` dari `lark.ticket.get` untuk `recordId` yang diberikan, dengan keadaan memuat dan galat.
- [ ] Salin link dan Kirim ke agen bekerja dengan `url` tiket; nonaktif dengan keterangan bila `recordLinkAvailable=false`.
- [ ] Tutup lewat tombol dan Esc memanggil `onClose`.
- [ ] Berganti `recordId` mengambil ulang tanpa menampilkan data lama dan mengabaikan respons usang.
- [ ] Build frontend lulus.

ownedPaths:
- `frontend/src/modules/lark/board/TicketDrawer.svelte`

Catatan: bisa dikerjakan paralel dengan `lark-tickets-data` dan `lark-board-ui`. Titik sambung ke `BoardPane` dikerjakan di `lark-tickets-wiring`.

### lark-tickets-wiring: Wiring tiket dan ctl
Deskripsi:
Fitur penutup epic; berjalan setelah tiga fitur lain selesai (bergantung pada `lark-tickets-data`, `lark-board-ui`, `lark-ticket-drawer`).
- Di `BoardPane.svelte`: impor `TicketDrawer`, render saat `selectedRecordId` ada (props `source`, `tableId`, `viewId`, `recordId`, `onClose` yang mengosongkan seleksi), sambungkan klik kartu/baris (`onOpen` dari `Grid`/`Kanban`) ke seleksi itu. Tutup laci saat tabel/view/sumber berganti.
- Pastikan `wire_lark_tickets.go` terdaftar di daemon (memeriksa bahwa IPC tampil dan terjawab); bila registrasi memerlukan baris di file di luar ownedPaths, ikuti pola registrasi mandiri yang dipakai `wire_lark.go` dan laporkan.
- `cmd/shikigamictl/cmd_lark_tickets.go`: subcommand `shikigamictl lark tickets list --workspace <id> --source <id> --table <id> [--view <id>] [--offset N]` (cetak ringkas field, judul record, dan nextOffset) dan `shikigamictl lark tickets get --workspace <id> --source <id> --record <id>` (cetak `nama: nilai`, url, recordLinkAvailable), memanggil IPC daemon seperti subcommand `lark` yang sudah ada dari tiket 1 (`cmd_lark.go`; cek cara induk perintah `lark` didaftarkan dan tambahkan subcommand tanpa mengubah `cmd_lark.go` bila memungkinkan lewat registrasi di file ini).
- Verifikasi build dan jalan: tambah Base, buka board, ganti Grid/Kanban, cari, muat lagi, buka detail, salin dan kirim link ke sesi aktif; pulihkan pane setelah restart; sumber dihapus -> keadaan galat aman.

Acceptance criteria:
- [ ] Klik kartu (kanban) dan baris (grid) membuka laci dengan detail tiket yang benar; menutup laci mengembalikan ke daftar.
- [ ] `lark.tickets.list` dan `lark.ticket.get` terdaftar dan terjawab oleh daemon yang berjalan.
- [ ] `shikigamictl lark tickets list` dan `lark tickets get` berjalan terhadap daemon dan mencetak hasil terbaca.
- [ ] Alur penuh tambah Base -> board -> detail -> salin/kirim link berjalan di aplikasi; pane dipulihkan setelah restart.
- [ ] `go build ./...` dan build frontend lulus; aplikasi berjalan.

ownedPaths:
- `frontend/src/modules/lark/board/BoardPane.svelte`
- `cmd/shikigamictl/cmd_lark_tickets.go`

Catatan: tidak paralel; jalan terakhir setelah `lark-tickets-data`, `lark-board-ui`, dan `lark-ticket-drawer`. `BoardPane.svelte` dipakai bergantian dengan `lark-board-ui` (selesai lebih dulu).

## Path yang dimiliki
Hanya path ini yang boleh disentuh:
- `internal/larkdata/tickets.go`
- `internal/larkdata/ticketlink.go`
- `cmd/shikigamid/wire_lark_tickets.go`
- `cmd/shikigamictl/cmd_lark_tickets.go`
- `frontend/src/modules/lark/board/` (`BoardPane.svelte`, `Grid.svelte`, `Kanban.svelte`, `BoardHeader.svelte`, `boardClient.ts`, `TicketDrawer.svelte`)

Epic lain (jangan disentuh): tiket 1 (`connector/lark.go`, `larkdata/runner.go`, `allowlist.go`, `testdata/`, `docs/lark/`, `cmd_lark.go`), tiket 2 (`workspace/lark.go`, `larkdata/link.go`, `tables.go`, `wire_lark.go`, `modules/lark/` selain `board/`), tiket 4 (wiki, doc, `frontend/package.json`).

## Wiring
Penutup epic ini adalah fitur `lark-tickets-wiring`. Tiket ini tidak butuh titik sambung baru: pane `lark-board` sudah terdaftar oleh tiket 2 (jenis pane, field persist, item rail, daftar Base yang membuka pane), dan `BoardPane.svelte` adalah titik ekstensinya. Wiring menyambungkan: (1) Grid/Kanban ke `TicketDrawer` di dalam `BoardPane.svelte`; (2) IPC tiket ke daemon lewat `wire_lark_tickets.go`; (3) `shikigamictl lark tickets` ke daemon. Hasilnya terlihat dan jalan di aplikasi: klik Base di panel Lark -> board tiket -> klik tiket -> detail -> Salin link/Kirim ke agen. Tiket ini hanya bergantung pada tiket 1 dan 2 yang sudah selesai dan tidak bergantung pada tiket 4.

## Guard
Cukup build berhasil; tanpa unit test:
- `go build ./...`
- Build frontend (`npm run build` di `frontend/`, atau perintah build/check yang dipakai tiket sebelumnya)
