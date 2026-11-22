# Plan: Runner larkdata dengan allowlist

## Metadata
- Tiket: `lark-connector-ticket-01-lark-connector` (`docs/tickets/lark-connector-ticket-01-lark-connector.md`)
- Fitur: `larkdata-runner`
- Module Go: `github.com/NooBeeID/shikigami`
- ownedPaths:
  - `internal/larkdata/runner.go`
  - `internal/larkdata/allowlist.go`
  - `internal/larkdata/errors.go`
  - `internal/connector/cli.go` (hanya penambahan `RunCLITimeout` dan delegasi `RunCLI`)
- Depends on foundation: tidak (`hasFoundation=false`). Fitur ini tidak mengimpor kode fitur paralel lain.
- Kontrak bersama: lihat `docs/plans/lark-connector-ticket-01-lark-connector/overview.md` bagian 2.1, 2.3, 2.4. Jangan ubah overview.

## Tujuan
Menyediakan fondasi pemanggilan `lark-cli` yang aman untuk tiket Lark berikutnya: `connector.RunCLITimeout` (timeout per panggilan) dan paket baru `internal/larkdata` berisi `Run()` dengan allowlist 9 perintah baca, penambahan otomatis `--as user` dan flag format, timeout 20 detik, serta galat terklasifikasi berpesan Indonesia. Belum ada pemanggil `Run` di aplikasi (disengaja).

## Path yang dimiliki
HANYA path ini yang boleh disentuh:
- `internal/larkdata/runner.go` (baru)
- `internal/larkdata/allowlist.go` (baru)
- `internal/larkdata/errors.go` (baru)
- `internal/connector/cli.go` (hanya tambah `RunCLITimeout` dan ubah `RunCLI` menjadi delegasi)

Jangan sentuh: `internal/larkdata/testdata/`, `docs/lark/` (milik `lark-cli-spike`), `internal/connector/lark.go` (milik `lark-connector-adapter`), `cmd/shikigamictl/cmd_lark.go`, `cmd/shikigamid/*`, file wiring, `frontend/*`, `app/*`, `internal/provider/*`, dan perubahan git tak terkait. Jangan menjalankan perintah tulis `lark-cli`. Jangan `pkill` luas. Tanpa unit test; jangan membuat file `_test.go`.

## Kontrak
Disediakan (signature persis, sesuai overview 2.1 dan 2.3):
```go
// package connector
func RunCLITimeout(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error)

// package larkdata
const ListTimeout = 20 * time.Second
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
type Error struct { Code ErrorCode; Message string }
func (e *Error) Error() string
func AsError(err error) (*Error, bool)
```
Privat: `type spec struct { domain, command string; asUser bool; format []string }`, `check(args []string) (spec, *Error)`, `classify(err error) *Error`.

Dipakai: dari `internal/connector` hanya `RunCLITimeout`, `IsCLIMissing`, `ErrCLIMissing`; stdlib saja selain itu. Arah dependensi `larkdata -> connector`; `connector` tidak mengimpor `larkdata`.

Pesan galat (Indonesia, tetap):
- `CodeCLIMissing`: "lark-cli belum terpasang"
- `CodeNotLoggedIn`: "Belum login ke Lark. Jalankan lark-cli auth login."
- `CodePermission`: "Izin Lark kurang: <pesan CLI>"
- `CodeTimeout`: "Lark tidak merespons dalam 20 detik"
- `CodeNotAllowed`: "Perintah Lark tidak diizinkan: <detail>"
- `CodeCLI`: "Lark gagal: <pesan CLI>"; untuk stdout kosong/bukan JSON: "Balasan Lark tidak terbaca"

Nilai awal (overview 2.4): flag format `--format json`; pola belum login `not logged in`, `login`, `token`, `unauthorized`; pola izin `permission`, `scope`, `99991672`; cocok tanpa membedakan huruf besar/kecil. Bila `docs/lark/cli-findings.md` sudah ada saat Anda bekerja, baca dan ikuti (flag format per perintah, perintah tanpa `--as user`, pola galat, entri allowlist yang tak sesuai); bila belum, pakai nilai awal dan catat penyimpangan di bagian "Catatan" akhir plan ini.

## Langkah
1. **`internal/connector/cli.go`**: pindahkan seluruh badan `RunCLI` ke fungsi baru
   `func RunCLITimeout(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error)`,
   dengan satu perubahan: `context.WithTimeout(ctx, timeout)` menggantikan `cliTimeout`. Tambahkan doc comment "seperti RunCLI tetapi dengan batas waktu eksplisit". Ubah `RunCLI` menjadi:
   `func RunCLI(ctx context.Context, name string, args ...string) (string, error) { return RunCLITimeout(ctx, cliTimeout, name, args...) }`.
   Pertahankan `cliTimeout = 8 * time.Second`, `stderrMax`, `lookPath`, `truncate`, dan tiga pesan galat persis: `perintah <name> melewati batas waktu`, `perintah <name> gagal: <stderr>`, `%w: <name>` dengan `ErrCLIMissing`. Jangan ubah impor, urutan pengecekan, atau hal lain di file.
2. **`internal/larkdata/errors.go`** (package `larkdata`; impor `errors`, `strings`, `context`, `github.com/NooBeeID/shikigami/internal/connector`):
   - Definisikan `ErrorCode`, enam konstanta, `Error`, `(*Error).Error()` (mengembalikan `Message`), `AsError(err) (*Error, bool)` memakai `errors.As`.
   - `classify(err error) *Error` berurutan: `err == nil` -> nil; sudah `*Error` (via `AsError`) -> kembalikan apa adanya; `connector.IsCLIMissing(err)` -> `CodeCLIMissing`; `errors.Is(err, context.DeadlineExceeded)` atau pesan mengandung "melewati batas waktu" -> `CodeTimeout`; pesan (lowercase) mengandung pola izin -> `CodePermission` "Izin Lark kurang: <pesan CLI>"; mengandung pola belum login -> `CodeNotLoggedIn`; sisanya `CodeCLI` "Lark gagal: <pesan CLI>". Periksa izin SEBELUM belum-login karena kata "token"/"login" bisa muncul di pesan izin (mis. "scope ... re-login"); `99991672` termasuk pola izin. Pola disimpan sebagai dua `[]string` paket-privat agar mudah disesuaikan Wiring. `<pesan CLI>` = `err.Error()` dengan awalan `perintah lark-cli gagal: ` dibuang (`strings.TrimPrefix`).
   - Catatan: `RunCLITimeout` mengembalikan galat timeout sebagai teks (bukan `context.DeadlineExceeded`), maka cek substring "melewati batas waktu" wajib ada.
3. **`internal/larkdata/allowlist.go`** (package `larkdata`; impor `strings`):
   - `type spec struct { domain, command string; asUser bool; format []string }`.
   - Tabel privat `allowed = []spec{...}` berisi tepat 9 entri, semuanya `asUser: true`, `format: []string{"--format", "json"}` (nilai awal; satu-satunya tempat penyesuaian per perintah): `base +table-list`, `base +view-list`, `base +record-list`, `base +record-get`, `wiki +space-list`, `wiki +node-list`, `wiki +node-get`, `docs +fetch`, `sheets +cells-get`. Beri komentar bahwa `docs +fetch` dan `sheets +cells-get` mungkin butuh flag format berbeda sesuai spike.
   - `var forbiddenFlags = []string{"--as", "--format", "--json"}` ditambah flag bypass konfirmasi `--yes` dan `--force` (tidak relevan untuk perintah baca). Cocok untuk bentuk `--flag` maupun `--flag=nilai`.
   - `check(args []string) (spec, *Error)`: tolak `len(args) == 0` (`CodeNotAllowed`, "Perintah Lark kosong"); tolak `len(args) < 2`; cari entri dengan `args[0]==domain && args[1]==command`, bila tidak ada -> `CodeNotAllowed` "Perintah Lark tidak diizinkan: <args[0]> <args[1]>"; lalu pindai `args[2:]`, bila ada token yang sama dengan atau berawalan `<flag>=` untuk salah satu `forbiddenFlags` -> `CodeNotAllowed` "Perintah Lark tidak diizinkan: flag <flag> diatur otomatis". Kembalikan `(spec, nil)` bila lolos. Tidak ada proses yang dijalankan di fungsi ini.
4. **`internal/larkdata/runner.go`** (package `larkdata`; impor `context`, `encoding/json`, `strings`, `time`, `connector`):
   - `const ListTimeout = 20 * time.Second` dengan doc comment.
   - `func Run(ctx context.Context, args ...string) ([]byte, error)`:
     a. `s, e := check(args)`; bila `e != nil` kembalikan `nil, e` (jangan tabrakan nil-interface: kembalikan `error` literal `e` hanya di cabang non-nil).
     b. Susun `full`: `args` lalu (bila `s.asUser`) `"--as", "user"`, lalu `s.format...`. Salin ke slice baru (jangan memutasi `args` pemanggil).
     c. `out, err := connector.RunCLITimeout(ctx, ListTimeout, "lark-cli", full...)`; bila `err != nil` -> `return nil, classify(err)`.
     d. `trimmed := strings.TrimSpace(out)`; bila kosong atau `!json.Valid([]byte(trimmed))` -> `return nil, &Error{Code: CodeCLI, Message: "Balasan Lark tidak terbaca"}`.
     e. `return []byte(trimmed), nil`.
   - Pesan timeout harus tetap "Lark tidak merespons dalam 20 detik" (selaras `ListTimeout`).
5. Jalankan `gofmt -l internal/larkdata internal/connector` (harus kosong) dan `go vet ./internal/larkdata/ ./internal/connector/`, lalu `go build ./...`. Perbaiki galat hanya di path yang dimiliki. Jangan menjalankan `lark-cli` tulis; bila ingin uji manual cukup panggilan baca singkat atau lewati.
6. Verifikasi manual bentuk (tanpa file test): periksa dengan membaca kode bahwa (i) `RunCLI` hanya satu baris delegasi, (ii) tidak ada jalur di `Run` yang menjalankan proses sebelum `check` lolos, (iii) `--as user` dan format ditambahkan di akhir sehingga tidak bisa ditimpa pemanggil (flag itu sudah ditolak oleh `check`).

## Perubahan
- Baru: `internal/larkdata/errors.go`, `internal/larkdata/allowlist.go`, `internal/larkdata/runner.go`.
- Diubah: `internal/connector/cli.go` (tambah `RunCLITimeout`; `RunCLI` delegasi).
- Tidak ada perubahan lain (tidak ada git add/commit kecuali diinstruksikan oleh langkah implementasi).

## Wiring
Fitur ini mengekspos, untuk dipakai tiket 02 sampai 04 dan disinkronkan oleh langkah wiring (jangan disambungkan di sini):
- `larkdata.Run`, `larkdata.ListTimeout`, `larkdata.Error`/`ErrorCode` (6 kode)/`AsError` sebagai kontrak pemetaan galat ke kode connector di IPC.
- `connector.RunCLITimeout`.
- Titik penyesuaian bagi Wiring setelah `docs/lark/cli-findings.md` selesai: tabel `allowed` dan `forbiddenFlags` di `allowlist.go` (format/`asUser` per perintah, entri yang terbukti tak ada) serta dua daftar pola di `errors.go`.
Tidak ada pemanggil di aplikasi dan tidak ada perubahan di `cmd/shikigamid`; paket cukup ter-build lewat `go build ./...`.

## Selesai bila
- `connector.RunCLITimeout` ada; `RunCLI` hanya mendelegasi dengan `cliTimeout` (8 detik), pesan galat identik.
- `larkdata.Run` menolak args kosong, pasangan di luar 9 entri, dan flag `--as`/`--format`/`--json` dari pemanggil dengan `CodeNotAllowed` sebelum proses berjalan.
- Panggilan yang lolos memuat `--as user` dan flag format dari tabel; timeout efektif `ListTimeout` 20 detik.
- Enam kode galat terklasifikasi dengan pesan Indonesia; pesan izin kurang memuat pesan CLI; stdout kosong/bukan JSON -> `CodeCLI` "Balasan Lark tidak terbaca".
- Hanya perintah baca di allowlist.
- `gofmt` bersih dan `go build ./...` lulus. Tidak ada file di luar ownedPaths yang berubah.

## Catatan
(Isi bila menyimpang dari kontrak, mis. nilai flag format yang dipakai bila `cli-findings.md` sudah ada.)
