# Plan: Data tiket larkdata dan IPC

## Metadata
- Tiket: `lark-connector-ticket-03-lark-tickets` (`docs/tickets/lark-connector-ticket-03-lark-tickets.md`)
- Fitur: `lark-tickets-data` (Data tiket larkdata dan IPC)
- Kontrak bersama: `docs/plans/lark-connector-ticket-03-lark-tickets/overview.md` (bagian 2.1, 2.2, 2.3, 2.6, 6, 7). Baca dulu; plan ini tidak mengulangnya.
- ownedPaths:
  - `internal/larkdata/tickets.go`
  - `internal/larkdata/ticketlink.go`
  - `cmd/shikigamid/wire_lark_tickets.go`
- Depends on foundation: ya. `00-foundation.md` sudah membuat `internal/larkdata/tickets.go` dan `ticketlink.go` berisi tipe, konstanta, dan stub. Plan ini MENIMPA penuh kedua berkas itu dengan implementasi (tipe dan signature tidak boleh berubah). `wire_lark_tickets.go` dibuat baru di sini.
- Juga bergantung pada tiket 01 (`larkdata.Run()`, `*larkdata.Error`, allowlist, `docs/lark/cli-findings.md`, fixture `internal/larkdata/testdata/`) dan tiket 02 (`workspace.Store.LarkSources`, layanan `workspace.store`, `TicketSource`).

## Tujuan
Mengubah keluaran `lark-cli base +record-list` dan `+record-get` menjadi `TicketPage` dan `TicketDetail` yang stabil (field bertipe, field utama, judul, teks tampilan, `nextOffset`), membangun link tiket (`BuildTicketLink`), dan mengekspos dua IPC daemon `lark.tickets.list` dan `lark.ticket.get` dengan galat berformat `[code] pesan Indonesia`. Read-only, tanpa menulis ke store, `larkdata` tidak mengimpor `internal/workspace`.

## Path yang dimiliki
HANYA tiga berkas ini yang boleh disentuh:
- `internal/larkdata/tickets.go` (timpa stub Foundation)
- `internal/larkdata/ticketlink.go` (timpa stub Foundation)
- `cmd/shikigamid/wire_lark_tickets.go` (baru)

Jangan menyentuh: `runner.go`, `allowlist.go`, `errors.go`, `link.go`, `tables.go`, `testdata/`, `docs/lark/`, `modules.go`, `wire_lark.go`, `wire_lark_link.go`, `wire_workspace.go`, `internal/workspace/*`, `cmd/shikigamictl/*`, apa pun di `frontend/`, dan berkas docs/plans lain. Jangan men-stage perubahan tak terkait di working tree.

## Kontrak
Disediakan (nama persis, dari overview 2.1; jangan ubah):
- `larkdata`: `const pageSize = 50`, `const FieldTypeSingleSelect = "single_select"`, tipe `Field`, `Record`, `TicketPage`, `TicketDetailField`, `TicketDetail`, `TicketListQuery{BaseToken, TableID, ViewID, Offset}`, `TicketGetQuery{BaseToken, Host, TableID, ViewID, RecordID}`.
- `func ListTickets(ctx context.Context, q TicketListQuery) (TicketPage, error)`
- `func GetTicket(ctx context.Context, q TicketGetQuery) (TicketDetail, error)`
- `func BuildTicketLink(host, baseToken, tableID, viewID, recordID string) (url string, recordLinkAvailable bool)`
- IPC `lark.tickets.list` params `{workspaceId, sourceId, tableId, viewId?, offset}` -> `TicketPage`; `lark.ticket.get` params `{workspaceId, sourceId, recordId, tableId, viewId?}` -> `TicketDetail` (overview 2.2). Galat: overview 2.2 (kode -> `ipc.ErrInternal`/`ErrNotFound`/`ErrInvalid`).
- Modul daemon `RegisterModule("lark-tickets", ...)`.

Dipakai (periksa NAMA PERSIS di kode saat implementasi; kode tiket 01/02 belum ada saat plan ditulis; laporkan selisih, jangan mengarang):
- Tiket 01: `larkdata.Run(ctx, args ...)` (cek signature: apakah menerima `[]string` dan mengembalikan `[]byte`/stdout, atau sudah mem-parse JSON), `*larkdata.Error` dengan kode `cli_missing|not_logged_in|permission|timeout|not_allowed|cli_error` dan pesan (cek nama field/metode, mis. `Code`, `Message`), allowlist yang memuat `base +record-list` dan `base +record-get`, `docs/lark/cli-findings.md`, fixture `internal/larkdata/testdata/`. Bila `+record-list`/`+record-get` tidak ada di allowlist: hentikan bagian itu dan laporkan sebagai ketergantungan tiket 01 (jangan ubah `allowlist.go`).
- Tiket 02: `workspace.Store` dengan `LarkSources(wsID string) (...)` yang memuat `Tickets []TicketSource` (field `ID`, `Name`, `BaseToken`, `Host`, `TableID`, `ViewID`); layanan `d.Lookup("workspace.store")`.
- Daemon: `ipc.Server.Register(method string, h ipc.Handler)` dengan `Handler func(ctx context.Context, params json.RawMessage) (any, error)`; `ipc.NewError(code int, msg string)`; konstanta `ipc.ErrInternal`, `ipc.ErrNotFound`, `ipc.ErrInvalid`; `RegisterModule(name string, fn func(srv *ipc.Server, d *Deps))`; `(*Deps).Lookup(name) (any, bool)`.

## Langkah

### 1. Pelajari sumber kebenaran bentuk data (wajib sebelum menulis parser)
1. Baca `docs/lark/cli-findings.md` dan fixture `+record-list`/`+record-get` di `internal/larkdata/testdata/`. Catat: (a) bentuk JSON respons (apakah dibungkus `data`), (b) cara daftar field dan tipenya dikirim (nama key, nilai tipe single select, penanda field utama), (c) bentuk nilai tiap tipe (select, multi-select, user, link, lampiran, tanggal, boolean, angka, teks panjang), (d) bentuk paginasi (`has_more`, `page_token`/kursor, atau hitung dari offset+jumlah), (e) format URL Base dan apakah parameter record terverifikasi, (f) catatan pengecualian `--as user`.
2. Parser dibangun dari fixture, bukan tebakan. Bila fixture tidak ada, hentikan dan laporkan (jangan merekam sendiri dengan perintah tulis; perintah baca hanya lewat `Run()`).
3. Bila temuan spike membantah sesuatu (mis. tidak ada tipe single-select), terapkan aturan potong dari overview 5.6: lanjutkan, laporkan di catatan akhir.

### 2. Timpa `internal/larkdata/tickets.go`
Package `larkdata`. Pertahankan persis konstanta dan tipe dari overview 2.1 (tag json camelCase, `NextOffset *int` dengan `omitempty`). Hapus komentar "Stub Foundation". Tambahkan helper tak diekspor (semua nama berawalan `ticket`/`normalize` agar tidak bentrok dengan berkas larkdata lain; cek bentrok dengan `go build`):
- `const ticketListTimeout = 20 * time.Second`.
- `func ListTickets(ctx, q) (TicketPage, error)`:
  1. Validasi `q.BaseToken` dan `q.TableID` tidak kosong; bila kosong kembalikan galat biasa (`errors.New`); handler IPC sudah memvalidasi lebih dulu sehingga ini hanya pengaman.
  2. `ctx, cancel := context.WithTimeout(ctx, ticketListTimeout)`.
  3. Susun argumen CLI: `base +record-list --base-token <BaseToken> --table-id <TableID> [--view-id <ViewID>] --limit 50 --offset <Offset>` ditambah `--as user` dan flag format json sesuai temuan spike (nama flag format persis dari findings; pakai konstanta `pageSize` untuk 50). Urutan/posisi flag `--as` mengikuti aturan `Run()`: bila `Run()` sudah menambahkan `--as user` dan format json sendiri, jangan menggandakan; ikuti kode tiket 01.
  4. Panggil `Run(ctx, ...)`; bila galat kembalikan apa adanya (sudah `*larkdata.Error`).
  5. Parse JSON ke struct sementara (tipe lokal tak diekspor) lalu normalisasi ke `TicketPage`.
- Normalisasi field: iterasi daftar field sesuai fixture, `Field.ID`, `Field.Name`, `Field.Type` = hasil `normalizeFieldType(raw)`; tipe single-select CLI dipetakan ke `FieldTypeSingleSelect`, tipe lain dipetakan ke string stabil (mis. `text`, `multi_select`, `user`, `link`, `attachment`, `date`, `checkbox`, `number`, `other`; nilai tepatnya bebas, hanya single-select yang dipakai lintas lapisan). `IsPrimary` dari penanda spike. Pastikan tepat satu `IsPrimary`: bila nol atau lebih dari satu, tandai hanya field pertama (fallback; bila spike menyatakan tak ada field utama, tetap fallback field pertama). `Fields` tidak pernah nil (`[]Field{}`).
- Normalisasi record: `Record.ID`, `Record.Values[fieldID] = ticketDisplayText(rawValue)`, `Values` tidak pernah nil; `Title = Values[primaryFieldID]` setelah `strings.TrimSpace`; kosong -> `"(Tanpa judul)"`. `Records` tidak pernah nil.
- `func ticketDisplayText(v any) string` (atau berbasis `json.RawMessage` bila lebih cocok dengan fixture): string apa adanya; angka tanpa notasi ilmiah (format ulang `float64` bulat sebagai integer); boolean -> `"Ya"`/`"Tidak"` (centang); select -> teks opsi; array -> gabung elemen yang sudah diubah dengan `", "` (lewati elemen kosong); objek user -> nama (fallback id/email sesuai fixture); objek link -> teks atau url; lampiran -> nama berkas; tanggal -> teks/format sesuai fixture (bila timestamp ms, format `2006-01-02` zona lokal sesuai temuan); `nil` -> `""`. Tipe tak dikenal: ambil key nama/teks yang umum (`name`, `text`, `title`, `value`), terakhir fallback ke `json.Marshal` ringkas. Tidak boleh menghasilkan JSON mentah untuk tipe yang dikenal.
- `nextOffset`: hitung sesuai bentuk paginasi di fixture. Contoh bila API memberi `has_more`: `NextOffset = &(Offset + len(records))` hanya bila `has_more` true dan `len(records) > 0`; bila API tidak memberi `has_more`, `nextOffset` ada hanya bila `len(records) == pageSize` (dokumentasikan asumsi di komentar). Nil bila halaman terakhir. Hindari loop tak terbatas (jangan beri `nextOffset` bila halaman kosong).
- `func GetTicket(ctx, q) (TicketDetail, error)`:
  1. Validasi `BaseToken`, `TableID`, `RecordID` tidak kosong.
  2. Panggil `base +record-get --base-token --table-id --record-id <RecordID>` (nama flag persis dari findings/allowlist, plus `--as user` dan format json) lewat `Run()`. Timeout: pakai timeout bawaan `Run()`; bila perlu bungkus `context.WithTimeout` 20 detik juga (aman).
  3. Karena `+record-get` dapat mengembalikan nilai per nama atau per id field, dan urutan field tabel dibutuhkan: bila respons tidak membawa daftar field berurutan, ambil urutan dari `+record-list` bukan opsi (hanya perintah allowlist yang boleh); prioritaskan urutan yang ada di respons `+record-get` sesuai fixture. Bentuk `Fields []TicketDetailField{Name, Text}` berurutan menurut urutan field tabel (urutan kemunculan di fixture), lewati field dengan teks kosong setelah `TrimSpace`. Nilai lewat `ticketDisplayText`. `Fields` tidak pernah nil.
  4. `url, ok := BuildTicketLink(q.Host, q.BaseToken, q.TableID, q.ViewID, q.RecordID)`; isi `TicketDetail.URL = url`, `RecordLinkAvailable = ok`.
- Tidak ada panic: semua type assertion memakai bentuk `v, ok`; JSON tak sesuai -> kembalikan galat `fmt.Errorf` yang dibungkus sebagai `*larkdata.Error` kode `cli_error` bila tipe itu punya konstruktor yang dapat dipakai (cek `errors.go`; bila tidak, galat biasa, handler IPC memetakannya ke `[cli_error]`).
- Impor hanya stdlib dan paket larkdata sendiri. TIDAK mengimpor `internal/workspace`.

### 3. Timpa `internal/larkdata/ticketlink.go`
- `func BuildTicketLink(host, baseToken, tableID, viewID, recordID string) (url string, recordLinkAvailable bool)`.
- Format URL Base mengikuti `docs/lark/cli-findings.md`; default bila findings tak menetapkan lain: dasar `https://<host>/base/<baseToken>` dengan query `table=<tableID>` dan `view=<viewID>` (hilangkan `view` bila kosong; hilangkan `table` bila kosong). Escape nilai query dengan `net/url`. Host kosong -> kembalikan `"", false`. Bila host sudah memuat skema (`http://`/`https://`), pakai apa adanya, jangan dobel; hapus slash akhir.
- Parameter record: bila findings memastikan format (mis. `record=<recordID>`), tambahkan dan `recordLinkAvailable = recordID != ""`. Bila findings TIDAK memastikan (atau tidak menyebut), jangan menebak: kembalikan link tabel/view dan `recordLinkAvailable=false`. Tulis komentar yang menyebut sumber keputusan (findings) pada kode.
- Fungsi murni, tanpa CLI.

### 4. Buat `cmd/shikigamid/wire_lark_tickets.go`
Package `main`. Impor: `context`, `encoding/json`, `errors`, `fmt`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`, `github.com/NooBeeID/shikigami/internal/larkdata`, `github.com/NooBeeID/shikigami/internal/workspace`. Semua helper lokal berawalan `tickets` (jangan memakai `larkStore`/`linkStore`/`larkErr`, milik file lain).
- `func init() { RegisterModule("lark-tickets", func(srv *ipc.Server, d *Deps) { ... }) }` mendaftarkan dua handler via `srv.Register`. Tanpa publikasi event dan tanpa tulis ke store.
- `func ticketsStore(d *Deps) (*workspace.Store, error)`: `v, ok := d.Lookup("workspace.store")`; type assert ke `*workspace.Store`; gagal -> `ipc.NewError(ipc.ErrInternal, "[cli_error] Penyimpanan workspace belum siap")`. Dipanggil malas di dalam handler, bukan di saat registrasi.
- Param struct lokal: `ticketsListParams{WorkspaceID string \`json:"workspaceId"\`; SourceID string \`json:"sourceId"\`; TableID string \`json:"tableId"\`; ViewID string \`json:"viewId"\`; Offset int \`json:"offset"\`}` dan `ticketsGetParams{WorkspaceID, SourceID, RecordID, TableID, ViewID}` (tag camelCase).
- `func ticketsSource(store *workspace.Store, wsID, sourceID string) (workspace.TicketSource, error)`: `store.LarkSources(wsID)` (sesuaikan dengan signature/return sebenarnya dari tiket 02), cari `Tickets[i].ID == sourceID`; tidak ada -> `ticketsErr("not_found", "Sumber tidak ada di workspace ini")`.
- `func ticketsErr(code, msg string) error`: bungkus `ipc.NewError(<kode ipc>, "["+code+"] "+msg)`; pemetaan: `invalid_params` -> `ipc.ErrInvalid`, `not_found` -> `ipc.ErrNotFound`, lainnya -> `ipc.ErrInternal`.
- `func ticketsMapErr(err error) error`: `errors.As(err, &*larkdata.Error)`; bentuk `"[" + code + "] " + pesan Indonesia`. Pesan: `cli_missing` -> "lark-cli belum terpasang"; `not_logged_in` -> "Belum login ke Lark. Jalankan lark-cli auth login"; `permission` -> "Izin Lark kurang: " + pesan CLI; `timeout` -> "Permintaan ke Lark melewati batas waktu"; `not_allowed` -> "Perintah tidak diizinkan"; `cli_error`/lainnya -> pesan asli. Gunakan pesan dari `larkdata.Error` bila sudah berbahasa Indonesia agar tidak ganda (periksa isi `errors.go`). Bukan `*larkdata.Error` -> `[cli_error] <err.Error()>`. Semua -> `ipc.ErrInternal`. Hindari pesan ganda berprefiks `[code]`; bila pesan sudah diawali `[`, jangan tambah ulang.
- Handler `lark.tickets.list`: unmarshal params (gagal -> `[invalid_params] Parameter tidak valid`); `workspaceId`, `sourceId` kosong -> `[invalid_params]`; `tableId` (trim) kosong -> `[invalid_params] Tabel belum dipilih`; `offset < 0` -> 0. Ambil store, cari sumber, panggil `larkdata.ListTickets(ctx, larkdata.TicketListQuery{BaseToken: src.BaseToken, TableID: p.TableID, ViewID: p.ViewID, Offset: p.Offset})`; galat lewat `ticketsMapErr`; sukses kembalikan `TicketPage` langsung.
- Handler `lark.ticket.get`: params sama pola; `recordId` kosong -> `[invalid_params] ID tiket kosong`; `tableId` param kosong -> pakai `src.TableID`; masih kosong -> `[invalid_params] Tabel belum dipilih`; `viewId` param kosong -> boleh pakai `src.ViewID` bila ada. Panggil `larkdata.GetTicket(ctx, larkdata.TicketGetQuery{BaseToken: src.BaseToken, Host: src.Host, TableID: tableID, ViewID: viewID, RecordID: p.RecordID})`.
- Tanpa `panic`; `ipc.Server` sudah memulihkan panic, tetapi handler tetap tidak boleh mengandalkannya.

### 5. Guard
- Jalankan `gofmt -l internal/larkdata cmd/shikigamid` (harus kosong untuk berkas milik sendiri) dan `go vet ./internal/larkdata ./cmd/shikigamid`.
- Jalankan `go build ./...` dari root repo; harus lulus.
- Pastikan `grep -n "internal/workspace" internal/larkdata/tickets.go internal/larkdata/ticketlink.go` kosong.
- `git status` hanya menunjukkan tiga berkas milik fitur ini (ditambah berkas Foundation yang sudah ada); jangan stage yang lain.
- Opsional verifikasi manual bila daemon dapat dijalankan: panggil kedua IPC langsung (jangan `pkill` luas; matikan hanya PID yang diluncurkan sendiri) terhadap Base nyata dan bandingkan dengan fixture. Wiring/ctl bukan bagian fitur ini.

## Perubahan
- Diubah (timpa stub Foundation): `internal/larkdata/tickets.go`, `internal/larkdata/ticketlink.go`.
- Dibuat: `cmd/shikigamid/wire_lark_tickets.go`.
- Tidak ada perubahan lain. Tanpa unit test, tanpa dependensi baru (`go.mod` tidak berubah).

## Wiring
Fitur ini tidak menyambungkan apa pun sendiri. Yang diekspos untuk langkah wiring (`lark-tickets-wiring`):
- Modul daemon `lark-tickets` yang mendaftar sendiri lewat `init()`; `modules.go` TIDAK diubah. Wiring hanya memverifikasi `lark.tickets.list` dan `lark.ticket.get` terdaftar dan terjawab.
- IPC `lark.tickets.list` dan `lark.ticket.get` dengan bentuk params/hasil dan galat `"[code] pesan"` sesuai overview 2.2, dipakai `boardClient.ts` dan `shikigamictl lark tickets`.
- API Go `larkdata.ListTickets`, `larkdata.GetTicket`, `larkdata.BuildTicketLink` untuk pemakai lain bila ada.
- Bila selama implementasi ditemukan selisih dengan kontrak (nama di tiket 01/02, nilai literal tipe, pola registrasi yang butuh perubahan di file lain), catat di bagian laporan akhir agar Wiring menyelaraskan; jangan mengubah kontrak sendiri.

## Selesai bila
- [ ] `ListTickets` mengembalikan `fields`, `records`, `nextOffset?` sesuai kontrak (50 per halaman, `offset` bekerja, `nextOffset` nil di halaman terakhir, tak pernah nil untuk slice).
- [ ] Single-select terdeteksi lewat `FieldTypeSingleSelect`; tepat satu field `IsPrimary` (fallback field pertama); judul kosong menjadi "(Tanpa judul)".
- [ ] Nilai select/user/link/lampiran/tanggal/boolean/angka menjadi teks terbaca, bukan JSON mentah; multi-nilai digabung ", ".
- [ ] `GetTicket` mengembalikan `fields[{name,text}]` berurutan, `url`, `recordLinkAvailable`; false bila format record tak pasti menurut findings.
- [ ] Semua panggilan CLI lewat `Run()` dengan `--as user`, format json, timeout daftar 20 detik, hanya perintah allowlist.
- [ ] Galat CLI dan sumber tidak ada menjadi galat IPC `"[code] pesan Indonesia"` dengan kode ipc yang benar; tanpa panic; tidak ada tulis ke store.
- [ ] `larkdata` tidak mengimpor `internal/workspace`; hanya ownedPaths yang berubah.
- [ ] Guard: `go build ./...` lulus.
