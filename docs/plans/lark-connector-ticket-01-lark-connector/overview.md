# Overview: lark-connector-ticket-01-lark-connector (Connector Lark dan spike CLI)

Sumber: tiket `docs/tickets/lark-connector-ticket-01-lark-connector.md` dan spec `docs/superpowers/specs/2026-11-22-09-lark-connector-design.md`. Dokumen ini adalah sumber kebenaran untuk kontrak bersama; plan per fitur hanya merinci langkah kerja. Module Go: `github.com/NooBeeID/shikigami`. `lark-cli` terpasang di mesin pengembang (versi 1.0.97) dan sudah login, jadi spike bisa merekam output nyata.

## 1. Ringkasan epic dan urutan eksekusi

Epic ini mendaftarkan connector `lark` di kerangka Connector yang sudah ada (`internal/connector`: `types.go`, `registry.go`, `cli.go`, `ipc.go`, `fake.go`). Status (CLI belum terpasang / belum login / tersambung + nama user) dibaca dari `lark-cli auth status`; app tidak login dan tidak menyimpan token. Epic juga menyediakan paket `internal/larkdata` (runner `Run()` dengan allowlist perintah baca, `--as user`, format json, timeout 20 detik, klasifikasi galat), merekam fixture asli lewat spike beserta `docs/lark/cli-findings.md`, dan subcommand `shikigamictl lark status`. Tidak ada frontend baru; baris Lark muncul otomatis di Pengaturan > Connector dari registry.

Urutan:
1. **Foundation**: TIDAK ada (`hasFoundation=false`, tidak ada `00-foundation.md`). Keempat fitur tidak saling mengimpor kode: adapter hanya memakai `RunCLI` yang sudah ada; runner menambah `RunCLITimeout` miliknya sendiri dan paket baru `larkdata`; ctl hanya memakai `ipc.Client` dan IPC `connector.status` yang sudah ada; spike hanya menulis fixture dan dokumen. Kontrak antar fitur adalah teks (signature di bagian 2), bukan kode bersama.
2. **Fitur paralel**: `lark-cli-spike`, `lark-connector-adapter`, `larkdata-runner`, `lark-ctl-status`. Satu-satunya ketergantungan lunak: flag/pola galat di `larkdata-runner` (dan skema di adapter) idealnya dicocokkan dengan `docs/lark/cli-findings.md` dari spike. Agent runner dan adapter memulai dengan nilai awal dari spec/`--help`; bila `cli-findings.md` sudah ada saat mereka bekerja, ikuti temuan itu; bila belum, pakai nilai awal (bagian 2.4) dan biarkan Wiring menyelaraskan.
3. **Wiring**: verifikasi akhir dan sinkronisasi runner/adapter dengan temuan spike (bagian 5). Hampir semua sambungan otomatis lewat `init()`.

## 2. Kontrak bersama

### 2.1 Go: `internal/connector` (sudah ada, ditambah)
Tipe yang dipakai (tidak diubah): `State` (`StateCLIMissing="cli_missing"`, `StateNotLoggedIn="not_logged_in"`, `StateConnected="connected"`), `Status{State, User, Message}`, `Guide{Steps, Commands}`, `Connector` (`ID`, `Name`, `Status`, `ConnectGuide`), `Register`, `Get`, `RunCLI`, `IsCLIMissing`, `ErrCLIMissing`.

Tambahan aditif di `internal/connector/cli.go` (milik `larkdata-runner`, satu-satunya perubahan di file itu):
```go
// RunCLITimeout seperti RunCLI tetapi dengan batas waktu eksplisit.
func RunCLITimeout(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error)
```
`RunCLI` dipertahankan dan menjadi `return RunCLITimeout(ctx, cliTimeout, name, args...)`. Seluruh badan fungsi lama pindah ke `RunCLITimeout` dengan `context.WithTimeout(ctx, timeout)`. Perilaku, konstanta `cliTimeout = 8s`, `stderrMax`, dan pesan galat (`perintah <name> melewati batas waktu`, `perintah <name> gagal: <stderr>`, `ErrCLIMissing: <name>`) identik.

Catatan pesan galat: `RunCLITimeout` membungkus stderr CLI dalam pesan `perintah lark-cli gagal: <stderr terpotong 300 karakter>`; klasifikasi di `larkdata` mencocokkan substring dari pesan itu.

### 2.2 Go: `internal/connector/lark.go` (milik `lark-connector-adapter`)
```go
type larkConnector struct{}
func init() { Register(larkConnector{}) }
func (larkConnector) ID() string   // "lark"
func (larkConnector) Name() string // "Lark"
func (larkConnector) Status(ctx context.Context) (Status, error)
func (larkConnector) ConnectGuide() Guide
```
- `Status` menjalankan `RunCLI(ctx, "lark-cli", "auth", "status")` (tambah flag format json hanya bila spike menunjukkan perlu), decode ke struct privat `{Identities struct{ User *struct{ Status string; Available bool; UserName string } } }` dengan tag json `identities`, `user`, `status`, `available`, `userName`.
- Aturan: `IsCLIMissing(err)` -> `Status{State: StateCLIMissing, Message: "lark-cli belum terpasang"}`; JSON tak terparse, identitas `user` tidak ada, atau `available == false` -> `StateNotLoggedIn` (Message Indonesia, mis. "Belum login ke Lark. Ikuti langkah di dialog Connect."); selain itu `StateConnected` dengan `User = userName` (`status == "needs_refresh"` tetap connected). Galat lain dikembalikan sebagai `error`, kecuali spike menunjukkan exit non-nol berarti belum login. Tidak memuat token atau output mentah. Tidak ada cache.
- `ConnectGuide`: `Steps` teks Indonesia (pasang `lark-cli` bila belum ada, jalankan login, tekan Cek ulang), `Commands: []string{"lark-cli auth login"}`.

### 2.3 Go: paket `internal/larkdata` (milik `larkdata-runner`)
Berkas: `runner.go`, `allowlist.go`, `errors.go` (folder `testdata/` milik spike). Impor hanya stdlib dan `internal/connector`. Arah dependensi: `larkdata -> connector`; `workspace` tidak mengimpor `larkdata`.
```go
const ListTimeout = 20 * time.Second

// Run menjalankan satu perintah baca lark-cli dan mengembalikan stdout (JSON).
// args = perintah + flag TANPA `--as user` dan tanpa flag format (ditambah otomatis),
// mis. Run(ctx, "base", "+record-list", "--base-token", t, "--table-id", id, "--limit", "50").
func Run(ctx context.Context, args ...string) ([]byte, error)

type ErrorCode string
const (
    CodeCLIMissing  ErrorCode = "cli_missing"
    CodeNotLoggedIn ErrorCode = "not_logged_in"
    CodePermission  ErrorCode = "permission"
    CodeTimeout     ErrorCode = "timeout"
    CodeNotAllowed  ErrorCode = "not_allowed"
    CodeCLI         ErrorCode = "cli_error"
)
type Error struct { Code ErrorCode; Message string } // Message bahasa Indonesia
func (e *Error) Error() string
func AsError(err error) (*Error, bool)
```
Internal (privat, tidak dipakai fitur lain): `type spec struct { domain, command string; asUser bool; format []string }`, `check(args []string) (spec, *Error)`, `classify(err error) *Error`.

Allowlist awal (9 entri; sesuaikan dengan temuan spike): `base +table-list`, `base +view-list`, `base +record-list`, `base +record-get`, `wiki +space-list`, `wiki +node-list`, `wiki +node-get`, `docs +fetch`, `sheets +cells-get`. Tolak args kosong, pasangan di luar tabel, dan flag `--as`/`--format`/`--json` dari pemanggil, dengan `CodeNotAllowed` sebelum proses dijalankan. `Run` menambah `--as user` (kecuali entri `asUser:false`) dan flag format dari tabel, memanggil `connector.RunCLITimeout(ctx, ListTimeout, "lark-cli", full...)`. Klasifikasi: `connector.IsCLIMissing` -> `CodeCLIMissing`; deadline -> `CodeTimeout` "Lark tidak merespons dalam 20 detik"; pesan mengandung pola belum login -> `CodeNotLoggedIn`; pola izin/scope -> `CodePermission` "Izin Lark kurang: <pesan CLI>"; sisanya `CodeCLI`. Stdout kosong atau bukan JSON valid -> `CodeCLI` "Balasan Lark tidak terbaca".

### 2.4 Nilai awal sebelum temuan spike tersedia
- Nama flag format: awal `--format json`; tabel per perintah di `allowlist.go` (field `format []string`, `asUser bool`) menjadi satu tempat penyesuaian.
- Pola galat awal: belum login (`not logged in`, `login`, `token`, `unauthorized`), izin (`permission`, `scope`, `99991672`). Dicocokkan tanpa membedakan huruf besar/kecil.
- Skema `auth status`: `identities.user.{status, available, userName}`.

### 2.5 CLI: `cmd/shikigamictl/cmd_lark.go` (milik `lark-ctl-status`)
```go
type larkSubFunc func(ctx context.Context, c *ipc.Client, args []string) error
var larkSubcommands = map[string]larkSubFunc{}
func registerLarkSub(name string, fn larkSubFunc) // panic bila name kosong, fn nil, atau nama ganda
func runLark(ctx context.Context, c *ipc.Client, args []string) error
```
`init()` memanggil `RegisterCommand(Command{Name: "lark", Summary: "Connector Lark: status", Run: runLark})` dan `registerLarkSub("status", ...)` yang memanggil `callAndPrint(c, "connector.status", map[string]any{"id": "lark"})`. Pesan penggunaan memuat daftar subcommand terurut dari peta (`penggunaan: shikigamictl lark status`, tumbuh otomatis). Tanpa argumen atau subcommand tak dikenal -> `errors.New(usage)`. `registerLarkSub` dan peta dipakai tiket 02 sampai 04 dari file mereka sendiri (mis. `cmd_lark_tickets.go`), jadi namanya tidak boleh berubah.

### 2.6 Fixture dan dokumen (milik `lark-cli-spike`)
- `internal/larkdata/testdata/*.json`: `auth-status.json`, `base-table-list.json`, `base-view-list.json`, `base-record-list.json`, `base-record-get.json`, `wiki-space-list.json`, `wiki-node-list.json`, `wiki-node-get.json`, `wiki-node-get-bitable.json`, `wiki-node-list-my-library.json`, `docs-fetch.json`, `sheets-cells-get.json`, ditambah `README.md` (perintah penghasil tiap berkas, tanpa token).
- `docs/lark/cli-findings.md` (Indonesia): versi `lark-cli`, tabel perintah (flag wajib, format, `--as user` ya/tidak), skema JSON ringkas per perintah, tipe field dan bentuk nilai, penanda field utama, paginasi, wiki, URL, pola galat, dan bagian **Pengecualian dan pemotongan fitur**. Dokumen ini kontrak data untuk tiket 02 sampai 04.

### 2.7 Konvensi
- Teks UI, pesan galat, dokumen: bahasa Indonesia. Identifier kode: bahasa Inggris.
- Connector id `lark`; tidak ada token/kredensial di status, log, fixture, atau dokumen.
- Read-only total; tanpa cache disk; tanpa unit test.
- Aturan pemotongan (spec putaran 2): bila spike membantah asumsi, fitur terkait dipotong dan dicatat di findings; pekerjaan lain lanjut.

## 3. Daftar fitur

| Slug | Scope | ownedPaths |
|---|---|---|
| `lark-cli-spike` | Jalankan `lark-cli` nyata (read-only), rekam dan anonimkan fixture, tulis `cli-findings.md`. Tanpa perubahan Go/frontend. | `internal/larkdata/testdata/`, `docs/lark/` |
| `lark-connector-adapter` | `larkConnector` (`Status`, `ConnectGuide`) terdaftar via `init()`. | `internal/connector/lark.go` |
| `larkdata-runner` | `RunCLITimeout` di `cli.go`; paket `larkdata`: `Run`, allowlist, klasifikasi galat, `ListTimeout`. | `internal/larkdata/runner.go`, `internal/larkdata/allowlist.go`, `internal/larkdata/errors.go`, `internal/connector/cli.go` (hanya penambahan `RunCLITimeout` dan delegasi `RunCLI`) |
| `lark-ctl-status` | `shikigamictl lark status` dengan dispatcher subcommand `registerLarkSub`. | `cmd/shikigamictl/cmd_lark.go` |

Acceptance criteria rinci tiap fitur ada di tiket.

## 4. Foundation

Tidak ada. Alasan di bagian 1. `00-foundation.md` tidak ditulis (`hasFoundation=false`).

## 5. Wiring checklist

Dikerjakan setelah semua fitur selesai:
1. Konfirmasi `internal/connector/lark.go` terdaftar: `cmd/shikigamid/wire_connector.go` sudah mengimpor `internal/connector` (sudah dicek: ada), jadi `init()` `lark` ikut ter-link. Tidak perlu mengubah file wiring daemon.
2. Konfirmasi `cmd_lark.go` terdaftar via `init()`; `main.go` dan `registry.go` tidak diubah. `shikigamictl` (bantuan) memuat `lark`.
3. `internal/larkdata` belum punya pemanggil di app dan memang tidak diimpor siapa pun; cukup ter-build lewat `go build ./...`.
4. Sinkronkan dengan temuan spike: baca `docs/lark/cli-findings.md`; bila flag format, penerimaan `--as user`, atau pola galat berbeda dari nilai awal (bagian 2.4), perbarui tabel di `allowlist.go`, pola di `errors.go`, dan skema/aturan exit di `lark.go` (satu-satunya perubahan lintas fitur yang diizinkan di tahap Wiring). Hapus allowlist entri yang terbukti tidak ada (mis. `+view-list`/`+cells-get` tak sesuai) dan catat di findings.
5. Verifikasi akhir: jalankan aplikasi, buka Pengaturan > Connector, pastikan baris Lark tampil dengan status benar dan dialog Connect memuat `lark-cli auth login`; jalankan `shikigamictl connector list`, `shikigamictl connector status lark`, `shikigamictl lark status`, `shikigamictl lark` (usage), dan `shikigamictl lark foo` (usage).
6. Periksa fixture: semua JSON valid (`jq . file`), tidak ada token/email/open_id/nama asli (grep).
7. Tambahkan ringkasan keputusan akhir di bagian akhir `docs/lark/cli-findings.md` bila ada perubahan setelah adapter/runner selesai.

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` (dari root repo) harus lulus.
- `make build` bila target ada dan relevan; tidak ada frontend baru, jadi build frontend tidak diubah.
- Untuk `lark-cli-spike`: pemeriksaan manual `jq .` pada tiap fixture dan grep nilai asli yang diketahui; tidak ada perubahan Go.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, `cmd/shikigamid/*`, `frontend/*`, `internal/workspace/*`, atau file wiring.
- `internal/connector/cli.go` dimiliki hanya `larkdata-runner`; fitur lain tidak mengedit file itu. `lark-connector-adapter` cukup memakai `RunCLI` apa adanya (jangan menunggu `RunCLITimeout`).
- Jangan mengubah signature kontrak di bagian 2; bila perlu menyimpang, catat di akhir plan fitur dan biarkan Wiring menyelaraskan.
- Working tree punya perubahan tak terkait (`app/*`, `frontend/*`, `internal/provider/*`); jangan disentuh atau di-stage.
- Jangan mem-pkill luas; matikan hanya proses yang diluncurkan sendiri.
- Jangan menjalankan perintah tulis `lark-cli` (kirim pesan, buat/ubah dokumen, `auth logout`, dsb.); spike read-only.
