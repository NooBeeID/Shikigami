---
id: lark-connector-ticket-01-lark-connector
title: Connector Lark dan spike CLI
spec: docs/superpowers/specs/2026-11-22-09-lark-connector-design.md
status: ready
wave: 1
depends_on: []
---
# Connector Lark dan spike CLI

## Tujuan
Mendaftarkan connector `lark` di kerangka Connector yang sudah ada (`internal/connector`) sehingga statusnya (CLI belum terpasang, belum login, tersambung beserta nama user) terbaca dari `lark-cli` yang sudah login di mesin, tanpa login atau token di app. Epic ini juga merekam fixture asli `lark-cli` lewat spike (bentuk JSON, tipe field, paginasi, node wiki, format URL) dan mencatat temuan serta pemotongan fitur di `docs/lark/cli-findings.md`. Terakhir, epic ini menyediakan paket `internal/larkdata` berisi runner `Run()` (allowlist perintah baca, selalu `--as user`, format json, timeout daftar 20 detik, klasifikasi galat) sebagai fondasi semua tiket Lark berikutnya (02, 03, 04).

## Hasil yang bisa dijalankan
Di atas kerangka Connector (settings-framework-ticket-02) yang sudah berisi connector `fake`, setelah tiket ini selesai:
- Pengaturan > Connector menampilkan baris baru **Lark** dengan badge status: "CLI belum terpasang", "Belum login", atau "Tersambung" beserta `userName`. Tombol **Connect** membuka dialog berisi langkah dan perintah `lark-cli auth login`, lalu **Cek ulang** memanggil status lagi. Tidak ada kode frontend baru; baris muncul otomatis dari registry.
- `shikigamictl connector list` memuat `lark`; `shikigamictl connector status lark` dan `shikigamictl lark status` mencetak status yang sama.
- Repo berisi fixture asli (teranonimkan) di `internal/larkdata/testdata/` dan dokumen temuan `docs/lark/cli-findings.md` yang menjadi acuan parser di tiket 02 sampai 04.
- Paket `internal/larkdata` dengan `Run()` siap dipakai tiket berikutnya (belum ada pemanggil di aplikasi; ini disengaja).
- `go build ./...` lulus dan aplikasi berjalan seperti sebelumnya; sesi terminal tidak terganggu.

## Batasan
In scope:
- Connector `lark` (`internal/connector/lark.go`) dengan `Status()` dan `ConnectGuide()`.
- Spike `lark-cli` nyata: rekam output, verifikasi flag, anonimkan, simpan fixture, tulis temuan.
- Paket `internal/larkdata`: `Run()`, allowlist perintah baca, klasifikasi galat, timeout 20 detik.
- Tambahan kecil dan aditif di `internal/connector/cli.go` agar timeout bisa diatur per panggilan (lihat Kontrak), tanpa mengubah perilaku `RunCLI` yang ada.
- Subcommand `shikigamictl lark status`.
- Pesan galat/teks dalam bahasa Indonesia; identifier kode dalam bahasa Inggris.

Out of scope:
- Parser link, config `WorkspaceConfig.Lark`, IPC `lark.*`, wiring daemon (tiket 02 dan seterusnya).
- Parser JSON ke tipe domain (tabel, tiket, node wiki, dokumen); dibangun di tiket 02 sampai 04 dari fixture dan temuan yang dihasilkan di sini.
- Frontend apa pun (halaman Connector sudah ada dan tidak disentuh).
- Menyimpan token/kredensial, login oleh app, perintah `lark-cli` berisiko tulis.
- Unit test (guard hanya build). Allowlist tetap ditegakkan di kode, bukan lewat test.
- Subcommand `shikigamictl lark list/get` (ditambah tiket berikutnya di file terpisah).

## Kontrak

### Yang disediakan

Go, `internal/connector/lark.go` (package `connector`):
```go
func init() { Register(larkConnector{}) } // ID "lark", Name "Lark"
func (larkConnector) Status(ctx context.Context) (Status, error)
func (larkConnector) ConnectGuide() Guide
```
- `Status()` menjalankan `RunCLI(ctx, "lark-cli", "auth", "status")` (tambahkan flag format json bila spike menunjukkan perlu), lalu parse `identities.user.{status, available, userName}`.
  - `IsCLIMissing(err)` -> `Status{State: StateCLIMissing, Message: "lark-cli belum terpasang"}`.
  - JSON tak terparse atau `identities.user.available == false` atau identitas `user` tidak ada -> `StateNotLoggedIn`.
  - Selain itu `StateConnected` dengan `User = userName`. `status == "needs_refresh"` dihitung connected (CLI menyegarkan sendiri).
  - Galat lain (timeout, exit non-nol yang bukan "belum login") dikembalikan sebagai `error` (kerangka menampilkannya sebagai galat terbaca), kecuali spike menunjukkan exit non-nol berarti belum login; ikuti temuan spike.
- `ConnectGuide()`: langkah teks (pasang `lark-cli` bila belum ada, jalankan login, klik Cek ulang) dan `Commands: ["lark-cli auth login"]`. Status tidak boleh memuat token.

Go, `internal/connector/cli.go` (tambahan aditif; satu-satunya perubahan di file ini):
```go
// RunCLITimeout seperti RunCLI tetapi dengan batas waktu eksplisit.
func RunCLITimeout(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error)
```
`RunCLI` dipertahankan dan didelegasikan ke `RunCLITimeout(ctx, cliTimeout, ...)` (perilaku identik). Alasan: `RunCLI` memakai konstanta 8 detik, sedangkan daftar Lark butuh 20 detik.

Go, `internal/larkdata/runner.go` dan `allowlist.go` dan `errors.go` (package `larkdata`):
```go
const ListTimeout = 20 * time.Second

// Run menjalankan satu perintah baca lark-cli dan mengembalikan stdout (JSON).
// args = perintah + flag TANPA `--as user` dan tanpa flag format (ditambah otomatis),
// mis. Run(ctx, "base", "+record-list", "--base-token", t, "--table-id", id, "--limit", "50").
func Run(ctx context.Context, args ...string) ([]byte, error)

// Galat terklasifikasi.
type ErrorCode string
const (
    CodeCLIMissing    ErrorCode = "cli_missing"
    CodeNotLoggedIn   ErrorCode = "not_logged_in"
    CodePermission    ErrorCode = "permission"   // izin/scope kurang, pesan CLI diteruskan
    CodeTimeout       ErrorCode = "timeout"
    CodeNotAllowed    ErrorCode = "not_allowed"  // perintah di luar allowlist
    CodeCLI           ErrorCode = "cli_error"    // galat CLI lain
)
type Error struct { Code ErrorCode; Message string } // Message bahasa Indonesia; CLI detail ditambahkan bila ada
func (e *Error) Error() string
func AsError(err error) (*Error, bool)
```
- Allowlist (di `allowlist.go`, daftar eksplisit pasangan `<domain> <perintah>`; tolak sisanya dengan `CodeNotAllowed` sebelum proses dijalankan): `base +table-list`, `base +view-list`, `base +record-list`, `base +record-get`, `wiki +space-list`, `wiki +node-list`, `wiki +node-get`, `docs +fetch`, `sheets +cells-get`. Tambahan hanya bila spike membuktikan perlu (mis. `auth status` tetap dipakai connector, bukan lewat `Run`).
- Nama flag format ( `--format json` atau `--json`) dan penerimaan `--as user` ditetapkan per perintah dari hasil spike; simpan sebagai tabel per perintah di `allowlist.go` (field `format []string`, `asUser bool`). Perintah yang tidak menerima `--as user` dicatat pengecualian di temuan dan ditandai `asUser: false` di tabel.
- `Run` selalu menambahkan `--as user` (kecuali pengecualian) dan flag format ke argumen, memanggil `connector.RunCLITimeout(ctx, ListTimeout, "lark-cli", full...)`, lalu mengklasifikasi galat: `connector.IsCLIMissing` -> `CodeCLIMissing`; pesan mengandung tanda belum login/token (pola dikonfirmasi di spike) -> `CodeNotLoggedIn`; pesan mengandung tanda izin/scope kurang (pola dikonfirmasi di spike) -> `CodePermission` dengan pesan "Izin Lark kurang: <pesan CLI>"; timeout (deadline) -> `CodeTimeout` "Lark tidak merespons dalam 20 detik". Kontrak ini dipakai tiket 02 sampai 04 untuk memetakan galat ke kode connector yang sama di IPC.

Fixture dan dokumen:
- `internal/larkdata/testdata/*.json` (nama berbasis perintah, mis. `auth-status.json`, `base-table-list.json`, `base-view-list.json`, `base-record-list.json`, `base-record-get.json`, `wiki-space-list.json`, `wiki-node-list.json`, `wiki-node-get.json`, `wiki-node-get-bitable.json`, `wiki-node-list-my-library.json`, `docs-fetch.json`, `sheets-cells-get.json`) dan `README.md` kecil di folder itu: perintah persis (tanpa token) yang menghasilkan tiap berkas.
- `docs/lark/cli-findings.md`: temuan terstruktur (lihat acceptance criteria fitur `lark-cli-spike`). Dokumen ini adalah kontrak data untuk tiket 02 sampai 04.

CLI, `cmd/shikigamictl/cmd_lark.go`: `shikigamictl lark status` (memanggil IPC `connector.status` dengan `{id: "lark"}` dan mencetak hasilnya). Registrasi lewat `RegisterCommand` dari `init()` (konvensi `registry.go`); ekstensi `list/get` ditambah tiket lain di file terpisah, jadi `cmd_lark.go` memakai dispatcher subcommand yang bisa diperluas (lihat fitur `lark-ctl-status`).

### Yang dipakai
- `connector.Connector`, `connector.Register`, `connector.State*`, `connector.Status`, `connector.Guide`, `connector.RunCLI`, `connector.IsCLIMissing` (sudah ada).
- IPC `connector.list`, `connector.status`, `connector.guide` (sudah ada, tidak diubah).
- `callAndPrint` dan `RegisterCommand` di `cmd/shikigamictl`.
- `lark-cli` nyata yang sudah login di mesin pengembang (hanya untuk spike).

### Kontrak lintas epic yang dipenuhi di sini
- Connector id `lark`; schema status `identities.user.{status,available,userName}`.
- `internal/larkdata` hanya memanggil `connector.RunCLITimeout`/`RunCLI` lewat `Run()`; arah dependensi `larkdata -> connector`; `workspace` tidak mengimpor `larkdata`.

## Fitur

Catatan paralel: spike (`lark-cli-spike`), adapter (`lark-connector-adapter`), dan CLI status (`lark-ctl-status`) tidak saling bergantung dan bisa dikerjakan paralel. `larkdata-runner` idealnya mengikuti hasil spike agar flag sesuai (spec: "Dibangun setelah spike"); namun kerangkanya (allowlist, klasifikasi, `RunCLITimeout`) bisa dimulai paralel dengan nilai flag dari `--help`, lalu disesuaikan begitu `cli-findings.md` selesai.

### lark-cli-spike: Spike lark-cli dan fixture
Deskripsi:
Jalankan `lark-cli` nyata (sudah login di mesin) dan rekam output JSON. Semua panggilan read-only. Anonimkan sebelum disimpan: ganti token Base/wiki/doc, nama user, email, open_id, nama tenant, judul/isi dokumen dan nilai record dengan data palsu yang konsisten (struktur dan tipe tidak boleh berubah). Jangan menyimpan token atau secret apa pun. Perintah yang dijalankan (lengkapi flag dari `--help` tiap perintah):
- `lark-cli auth status` (periksa juga bentuk saat belum login bila bisa disimulasikan tanpa logout yang merusak; bila tidak aman, cukup catat dari dokumentasi/`--help`).
- `base +table-list`, `base +view-list`, `base +record-list` (dengan `--limit` kecil dan `--offset`, serta `--view-id`), `base +record-get`.
- `wiki +space-list`, `wiki +node-list` (akar space dan `--parent-node-token`, `--page-size 50`), `wiki +node-get` (termasuk node bertipe docx, sheet, bitable, dan `my_library`).
- `docs +fetch --doc-format markdown`, `sheets +cells-get` (rentang awal 50 baris x 20 kolom).
Verifikasi dan catat:
- Flag `--help` untuk `base +view-list` dan `sheets +cells-get` (yang belum diverifikasi spec), plus flag format json per perintah.
- Apakah tiap perintah menerima `--as user`; daftar pengecualian.
- Tipe field (single-select, user, link, lampiran, dst.): nama tipe di JSON dan bentuk nilai tiap tipe (agar tiket 03 bisa membuat teks tampilan dan kolom kanban).
- Bagaimana field utama (primary) tabel ditandai.
- Kursor/paginasi `+record-list` (`has_more`, `offset`, atau `page_token`) dan batas `--limit`.
- Wiki: `obj_type` dan `obj_token` node, ada tidaknya `has_child`, URL node dari `+node-get`, perilaku `my_library` (apakah `+node-list` bisa dipakai pada `--space-id my_library`).
- Format URL record Base yang dapat dipastikan (mis. parameter `table`, `view`, `record`) dibandingkan dengan perilaku Lark nyata atau output CLI; nyatakan eksplisit "pasti" atau "tidak pasti".
- Pola pesan galat stderr untuk: belum login, scope/izin kurang, token node tidak ditemukan (dipakai klasifikasi galat di `larkdata-runner`).
- Link Base di bawah wiki: `+node-get` memberi `obj_type: bitable` dan `obj_token` Base.
Tulis `docs/lark/cli-findings.md` (bahasa Indonesia) dengan bagian: versi `lark-cli`, tabel perintah (flag wajib, format, `--as user` ya/tidak), skema JSON ringkas per perintah (path field penting yang dipakai tiket 02 sampai 04), tipe field dan bentuk nilai, paginasi, wiki, URL, pola galat, dan bagian **Pengecualian dan pemotongan fitur** yang menyatakan hasil tiap asumsi (kanban tanpa single-select, `my_library` tak bisa di-list, format link record tak pasti, perintah tanpa `--as user`, `+view-list`/`+cells-get` tak sesuai) beserta keputusan potong sesuai aturan pemotongan kontrak (pekerjaan lain lanjut).
Tanpa unit test.

Acceptance criteria:
- [ ] Semua perintah di atas direkam dari `lark-cli` nyata dan disimpan di `internal/larkdata/testdata/` (JSON valid, `python -m json.tool`/`jq .` lulus).
- [ ] Fixture teranonimkan: tidak ada token asli, email, open_id, nama user/tenant asli, atau isi dokumen asli (periksa dengan grep terhadap nilai asli yang diketahui sebelum selesai).
- [ ] `internal/larkdata/testdata/README.md` mencatat perintah penghasil tiap fixture.
- [ ] `docs/lark/cli-findings.md` memuat: flag `+view-list` dan `+cells-get`, penerimaan `--as user` per perintah, tipe field dan bentuk nilai, penanda field utama, kursor paginasi, bentuk node wiki, perilaku `my_library`, status kepastian format URL record, pola galat belum-login/izin-kurang.
- [ ] Bagian "Pengecualian dan pemotongan fitur" ada dan menyatakan keputusan eksplisit untuk kanban, wiki pribadi, dan link tiket (aktif/dipotong beserta alasan).
- [ ] Tidak ada perubahan pada Go atau frontend dalam fitur ini.

ownedPaths:
- `internal/larkdata/testdata/`
- `docs/lark/`

### lark-connector-adapter: Adapter connector lark
Deskripsi:
Buat `internal/connector/lark.go` mengikuti pola `fake.go`: tipe `larkConnector`, `init()` memanggil `Register`. `Status()` dan `ConnectGuide()` sesuai Kontrak. Gunakan struct kecil privat untuk decode JSON (`identities.user.status`, `.available`, `.userName`); jangan menambah field yang memuat token. Bila spike (`docs/lark/cli-findings.md`) sudah ada dan membantah skema (mis. flag format wajib atau exit code non-nol saat belum login), sesuaikan sesuai temuan; bila belum ada, implementasikan sesuai skema spec dan sesuaikan nanti. Parsing dibuat tahan terhadap field tambahan/hilang (tidak panic). Tidak ada cache; tiap panggilan `Status()` menjalankan CLI.
Karena `RunCLI` memiliki batas 8 detik, panggilan status cukup memakainya langsung.

Acceptance criteria:
- [ ] `lark.go` terdaftar lewat `init()`; `connector.Get("lark")` mengembalikan connector dengan `Name() == "Lark"`.
- [ ] CLI tidak ada -> `cli_missing`; user tidak tersedia/identitas hilang/JSON tak valid -> `not_logged_in`; tersedia -> `connected` dengan `User = userName`; `needs_refresh` -> `connected`.
- [ ] `ConnectGuide()` berisi langkah teks dan perintah `lark-cli auth login`.
- [ ] Status tidak memuat token atau output mentah CLI.
- [ ] Pengaturan > Connector (framework) menampilkan baris Lark tanpa perubahan frontend.
- [ ] `go build ./...` lulus.

ownedPaths:
- `internal/connector/lark.go`

### larkdata-runner: Runner larkdata dengan allowlist
Deskripsi:
Paket baru `internal/larkdata` (hanya mengimpor `internal/connector` dan stdlib). Tambahkan dahulu `RunCLITimeout` di `internal/connector/cli.go` (perubahan aditif, `RunCLI` mendelegasi dengan `cliTimeout`; pesan galat tetap sama). Lalu:
- `allowlist.go`: tabel eksplisit perintah baca yang diizinkan (`domain` + `perintah`), dengan atribut per perintah dari spike (`asUser`, flag format). Fungsi privat `check(args []string) (spec, *Error)`; argumen pertama dan kedua harus cocok dengan entri; flag yang berisiko tulis atau bukan bagian perintah baca ditolak (`CodeNotAllowed`). Tolak pula args kosong. Pastikan pemanggil tidak bisa menyelipkan `--as bot` atau `--format` sendiri (tolak flag `--as` dan flag format dari pemanggil, runner yang menambahnya).
- `errors.go`: tipe `Error`/`ErrorCode`, `AsError`, dan fungsi `classify(err error) *Error` dengan pesan Indonesia (lihat Kontrak). Pola pencocokan pesan galat (belum login, izin kurang) berasal dari `docs/lark/cli-findings.md`; sampai spike selesai, gunakan pola awal yang wajar (mis. kata "not logged in", "login", "permission", "scope", "99991672") dan sesuaikan.
- `runner.go`: `Run` sesuai Kontrak, `ListTimeout = 20 * time.Second`, mengembalikan stdout sebagai `[]byte`. Stdout kosong atau bukan JSON valid dikembalikan sebagai `CodeCLI` "Balasan Lark tidak terbaca".
Tanpa unit test.
Fitur ini menyentuh `internal/connector/cli.go`; file itu dimiliki fitur ini saja di epic ini.

Acceptance criteria:
- [ ] `connector.RunCLITimeout` ada; `RunCLI` berperilaku identik (timeout 8 detik, pesan galat sama).
- [ ] `larkdata.Run` menolak perintah di luar allowlist dengan `CodeNotAllowed` tanpa menjalankan proses.
- [ ] Setiap panggilan yang lolos memuat `--as user` (kecuali pengecualian dari spike) dan flag format json sesuai tabel per perintah; pemanggil tidak bisa menimpa keduanya.
- [ ] Timeout efektif 20 detik (`ListTimeout`), bukan 8 detik.
- [ ] Galat terklasifikasi (`cli_missing`, `not_logged_in`, `permission`, `timeout`, `not_allowed`, `cli_error`) dengan pesan Indonesia; pesan izin kurang memuat pesan CLI.
- [ ] Hanya perintah baca di allowlist (9 entri di Kontrak, atau sesuai temuan spike); tidak ada perintah tulis.
- [ ] `go build ./...` lulus.

ownedPaths:
- `internal/larkdata/runner.go`
- `internal/larkdata/allowlist.go`
- `internal/larkdata/errors.go`
- `internal/connector/cli.go` (hanya penambahan `RunCLITimeout`)

### lark-ctl-status: shikigamictl lark (status)
Deskripsi:
`cmd/shikigamictl/cmd_lark.go`: `init()` memanggil `RegisterCommand(Command{Name: "lark", Summary: "Connector Lark: status", Run: runLark})`. `runLark` membaca subcommand `args[0]`; `status` memanggil `callAndPrint(c, "connector.status", map[string]any{"id": "lark"})`. Gunakan peta `larkSubcommands map[string]func(ctx, c, args) error` sehingga tiket berikutnya menambah subcommand dari file mereka sendiri (mis. `cmd_lark_tickets.go`, `cmd_lark_wiki.go`) dengan fungsi `registerLarkSub(name, fn)` yang dipanggil dari `init()` mereka (definisikan `registerLarkSub` dan peta di `cmd_lark.go`; panic bila nama ganda atau kosong, meniru `RegisterCommand`). Pesan penggunaan memuat daftar subcommand terurut dari peta, contoh `penggunaan: shikigamictl lark status` (daftar tumbuh otomatis). Tanpa argumen atau subcommand tak dikenal -> galat penggunaan.

Acceptance criteria:
- [ ] `shikigamictl lark status` mencetak hasil IPC `connector.status` untuk `lark`.
- [ ] `shikigamictl lark` tanpa argumen dan dengan subcommand tak dikenal menampilkan pesan penggunaan (bukan panic).
- [ ] `registerLarkSub` diekspos untuk dipakai file tiket lain tanpa mengedit `cmd_lark.go`.
- [ ] `shikigamictl` (bantuan) menampilkan `lark` di daftar command.
- [ ] `go build ./...` lulus.

ownedPaths:
- `cmd/shikigamictl/cmd_lark.go`

## Path yang dimiliki
Hanya ini yang boleh disentuh oleh tiket ini:
- `internal/connector/lark.go`
- `internal/connector/cli.go` (hanya penambahan `RunCLITimeout` dan delegasi `RunCLI`; tidak mengubah perilaku)
- `internal/larkdata/runner.go`
- `internal/larkdata/allowlist.go`
- `internal/larkdata/errors.go`
- `internal/larkdata/testdata/`
- `docs/lark/`
- `cmd/shikigamictl/cmd_lark.go`

Catatan: `internal/larkdata/errors.go` dan `internal/connector/cli.go` tidak ada di daftar path awal dekomposisi tetapi diperlukan fitur `larkdata-runner`; tidak ada epic lain yang memilikinya.

## Wiring
Tiket ini tidak bergantung pada tiket lain dan tidak ada frontend baru.
1. Connector `lark` terdaftar otomatis lewat `init()` di `internal/connector/lark.go`; paket `internal/connector` sudah diimpor daemon (connector `fake` terdaftar dengan cara yang sama), sehingga IPC `connector.list/status/guide` dan halaman Pengaturan > Connector langsung memuat Lark. Periksa bahwa pemuatan paket memang terjadi di binary daemon (cek impor di `cmd/shikigamid`); bila ternyata hanya lewat blank import yang sudah ada, tidak perlu perubahan apa pun.
2. `cmd/shikigamictl/cmd_lark.go` terdaftar otomatis lewat `init()`; tidak mengubah `main.go`/`registry.go`.
3. Verifikasi akhir: jalankan aplikasi, buka Pengaturan > Connector, pastikan baris Lark tampil dengan status yang benar dan dialog Connect berisi `lark-cli auth login`; jalankan `shikigamictl connector status lark` dan `shikigamictl lark status`. `internal/larkdata` belum punya pemanggil di app, hanya harus ter-build.
4. Tulis ringkasan di akhir `docs/lark/cli-findings.md` bila ada perubahan keputusan setelah adapter/runner selesai (mis. flag yang akhirnya dipakai tabel allowlist).

## Guard
- `go build ./...` lulus.
- `make build` (bila ada) lulus; aplikasi berjalan dan halaman Pengaturan > Connector menampilkan Lark.
- Tanpa unit test (kebijakan epic).
- Pemeriksaan manual fixture: semua JSON valid dan teranonimkan.
