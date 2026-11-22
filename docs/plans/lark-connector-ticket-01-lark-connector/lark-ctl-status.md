# Plan: shikigamictl lark (status)

## Metadata
- Tiket: `lark-connector-ticket-01-lark-connector`
- Fitur: `lark-ctl-status` (shikigamictl lark (status))
- ownedPaths: `cmd/shikigamictl/cmd_lark.go`
- Depends on foundation: tidak (tidak ada foundation di epic ini; fitur ini tidak mengimpor kode fitur lain)

## Tujuan
Menambah command `shikigamictl lark` dengan dispatcher subcommand yang bisa diperluas. Subcommand `status` mencetak hasil IPC `connector.status` untuk connector `lark`. Tiket 02 sampai 04 akan menambah subcommand dari file mereka sendiri lewat `registerLarkSub`, tanpa mengedit `cmd_lark.go`. Kontrak lengkap ada di `overview.md` bagian 2.5.

## Path yang dimiliki
HANYA file ini yang boleh disentuh:
- `cmd/shikigamictl/cmd_lark.go` (file baru)

Jangan menyentuh `main.go`, `registry.go`, `cmd_connector.go`, file wiring, atau path fitur lain (`internal/connector/*`, `internal/larkdata/*`, `docs/lark/`). Working tree punya perubahan tak terkait (`app/*`, `frontend/*`, `internal/provider/*`); jangan disentuh atau di-stage.

## Kontrak
Disediakan (nama tidak boleh berubah; dipakai tiket 02 sampai 04), package `main`:
```go
type larkSubFunc func(ctx context.Context, c *ipc.Client, args []string) error
var larkSubcommands = map[string]larkSubFunc{}
func registerLarkSub(name string, fn larkSubFunc) // panic bila name kosong, fn nil, atau nama ganda
func runLark(ctx context.Context, c *ipc.Client, args []string) error
```
- `args` yang diteruskan ke `larkSubFunc` adalah sisa argumen setelah nama subcommand (`args[1:]`).
- Pesan penggunaan dibangun dari peta, terurut alfabetis, nama dipisah `|`: `penggunaan: shikigamictl lark status` (format pasti ada di Langkah 4).

Dipakai (sudah ada di `cmd/shikigamictl`):
- `RegisterCommand(Command{Name, Summary, Run})` dan `Command` di `registry.go` (panic bila nama kosong/Run nil/ganda; pola yang ditiru).
- `callAndPrint(c *ipc.Client, method string, params any) error` di `registry.go`.
- `ipc.Client` dari `github.com/NooBeeID/shikigami/internal/ipc`.
- IPC `connector.status` dengan params `{"id": "lark"}` (sudah ada di daemon; tidak diubah).

## Langkah
1. Buat `cmd/shikigamictl/cmd_lark.go`, `package main`, impor: `context`, `errors`, `fmt`, `sort`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`.
2. Definisikan tipe dan peta:
   ```go
   type larkSubFunc func(ctx context.Context, c *ipc.Client, args []string) error

   var larkSubcommands = map[string]larkSubFunc{}
   ```
3. Definisikan `registerLarkSub(name string, fn larkSubFunc)`: `panic("nama subcommand lark kosong")` bila `name == ""`; `panic(fmt.Sprintf("subcommand lark %q tidak punya fungsi", name))` bila `fn == nil`; `panic(fmt.Sprintf("subcommand lark %q sudah terdaftar", name))` bila sudah ada di peta; selain itu simpan ke peta.
4. Definisikan `larkUsage() string`: kumpulkan kunci peta, `sort.Strings`, kembalikan `"penggunaan: shikigamictl lark " + strings.Join(names, "|")`. Dengan hanya `status` terdaftar hasilnya `penggunaan: shikigamictl lark status`. Dihitung saat dipanggil (bukan konstanta) agar ikut tumbuh ketika file lain mendaftar dari `init()`.
5. Definisikan `runLark(ctx, c, args)`: bila `len(args) == 0` kembalikan `errors.New(larkUsage())`; cari `fn, ok := larkSubcommands[args[0]]`; bila tidak ada kembalikan `errors.New(larkUsage())` (tidak boleh panic); selain itu `return fn(ctx, c, args[1:])`.
6. Definisikan `init()`:
   ```go
   func init() {
       RegisterCommand(Command{Name: "lark", Summary: "Connector Lark: status", Run: runLark})
       registerLarkSub("status", larkStatus)
   }
   ```
   Catatan: urutan init antar file di package yang sama tidak dijamin dipakai; karena `larkSubcommands` diinisialisasi sebagai variabel package-level (bukan di `init()`), file tiket lain aman memanggil `registerLarkSub` dari `init()` mereka.
7. Definisikan `larkStatus(_ context.Context, c *ipc.Client, args []string) error`: bila `len(args) != 0` kembalikan `errors.New(larkUsage())`; selain itu `return callAndPrint(c, "connector.status", map[string]any{"id": "lark"})`.
8. Jalankan `gofmt -l cmd/shikigamictl` (harus kosong) dan `go build ./cmd/shikigamictl/...`.

## Perubahan
- Dibuat: `cmd/shikigamictl/cmd_lark.go`
- Diubah: tidak ada.

## Wiring
Tidak ada yang perlu disambungkan manual. Fitur ini mengekspos:
- Command `lark` yang terdaftar otomatis lewat `init()` (`main.go` dan `registry.go` tidak diubah; bantuan `shikigamictl` memuat `lark` dari registry).
- `registerLarkSub(name, fn)`, `larkSubcommands`, `larkSubFunc` untuk dipakai tiket 02 sampai 04 dari file mereka sendiri (mis. `cmd_lark_tickets.go`, `cmd_lark_wiki.go`).
Verifikasi runtime (`shikigamictl lark status`, `shikigamictl lark`, `shikigamictl lark foo`) dilakukan di langkah wiring; jangan menjalankan atau mengubah file wiring di sini. Hasil `lark status` tergantung connector `lark` dari fitur adapter.

## Selesai bila
- [ ] `cmd/shikigamictl/cmd_lark.go` ada dan berisi `larkSubFunc`, `larkSubcommands`, `registerLarkSub`, `runLark`, `init()` sesuai Kontrak.
- [ ] `registerLarkSub` panic untuk nama kosong, fn nil, dan nama ganda.
- [ ] Tanpa argumen atau subcommand tak dikenal mengembalikan `errors.New("penggunaan: shikigamictl lark status")` (daftar terurut dari peta), bukan panic.
- [ ] `status` memanggil `callAndPrint(c, "connector.status", map[string]any{"id": "lark"})`.
- [ ] `gofmt -l cmd/shikigamictl` kosong dan `go build ./cmd/shikigamictl/...` sukses (guard fitur ini; `go build ./...` penuh bergantung pada fitur paralel dan diverifikasi di wiring). Tanpa unit test.
- [ ] Tidak ada file lain yang diubah (`git status` hanya menambah `cmd/shikigamictl/cmd_lark.go` dari fitur ini).
