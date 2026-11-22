# Plan: Perintah shikigamictl connector

## Metadata
- Tiket: `settings-framework-ticket-02-connector-framework` (Kerangka Connector)
- Fitur: `connector-cli`
- ownedPaths: `cmd/shikigamictl/cmd_connector.go`
- Depends on foundation: tidak (tidak ada foundation; fitur hanya memakai `ipc.Client` dan helper yang sudah ada di `cmd/shikigamictl/registry.go`)

## Tujuan
Menambah subcommand debug `shikigamictl connector list|status <id>|guide <id>` yang memanggil method IPC `connector.list|status|guide` (kontrak: overview bagian 2.2 dan 2.4). Fitur ini berdiri sendiri: tidak mengimpor `internal/connector`, sehingga bisa dikompilasi sebelum `connector-core` ada. Verifikasi end-to-end baru bisa setelah `connector-core` jadi.

## Path yang dimiliki
HANYA berkas ini yang boleh dibuat/diubah:
- `cmd/shikigamictl/cmd_connector.go` (baru)

Jangan sentuh `cmd/shikigamictl/registry.go`, `main.go`, berkas `cmd_*.go` lain, `cmd/shikigamid/*`, `internal/connector/`, frontend, maupun overview.md. Tidak ada berkas wiring yang perlu diubah: pendaftaran command terjadi dari `init()` di berkas ini sendiri.

## Kontrak
Dipakai (sudah ada di package `main` pada `cmd/shikigamictl`):
- `type Command struct { Name, Summary string; Run func(ctx context.Context, c *ipc.Client, args []string) error }` dan `RegisterCommand(cmd Command)` (panic bila nama duplikat).
- `callAndPrint(c *ipc.Client, method string, params any) error` (memanggil IPC, mencetak JSON terformat).
- `(*ipc.Client).Call(method string, params any, out any) error` dari `github.com/NooBeeID/shikigami/internal/ipc`.
- Pola referensi: `cmd_status.go` (`statusUsage`, `statusList` dengan `tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)`).

Method IPC yang dipanggil (diimplementasikan `connector-core`, lihat overview 2.2):
- `connector.list` (tanpa param) -> `[{"id": string, "name": string, "status": {"state": string, "user"?: string, "message"?: string}}]`
- `connector.status` `{"id": string}` -> `Status` (dicetak sebagai JSON apa adanya)
- `connector.guide` `{"id": string}` -> `Guide` (dicetak sebagai JSON apa adanya)

Disediakan:
- Command `connector` (Summary persis `Connector layanan luar: list|status|guide`).
- Konstanta `connectorUsage = "penggunaan: shikigamictl connector list|status <id>|guide <id>"`.
- Semua identifier baru di berkas ini diawali `connector` (package `main` dibagi dengan banyak berkas): `connectorUsage`, `connectorRow`, `connectorStatusInfo`, `runConnector`, `connectorList`.

## Langkah
1. Buat `cmd/shikigamictl/cmd_connector.go`, `package main`. Impor: `context`, `errors`, `fmt`, `os`, `text/tabwriter`, dan `github.com/NooBeeID/shikigami/internal/ipc`. Jangan impor `internal/connector`.
2. Konstanta:
   ```go
   const connectorUsage = "penggunaan: shikigamictl connector list|status <id>|guide <id>"
   ```
3. Tipe lokal untuk dekode `connector.list` (hanya field yang ditampilkan):
   ```go
   type connectorStatusInfo struct {
       State   string `json:"state"`
       User    string `json:"user"`
       Message string `json:"message"`
   }
   type connectorRow struct {
       ID     string              `json:"id"`
       Name   string              `json:"name"`
       Status connectorStatusInfo `json:"status"`
   }
   ```
4. `init()`:
   ```go
   func init() {
       RegisterCommand(Command{
           Name:    "connector",
           Summary: "Connector layanan luar: list|status|guide",
           Run:     runConnector,
       })
   }
   ```
5. `func runConnector(ctx context.Context, c *ipc.Client, args []string) error` (parameter `ctx` tidak dipakai; boleh `_ context.Context` agar bersih, signature tipe tetap sama):
   - `len(args) == 0` -> `return errors.New(connectorUsage)`.
   - `switch args[0]`:
     - `"list"`: bila `len(args) != 1` -> usage; selain itu `return connectorList(c)`.
     - `"status"`: bila `len(args) != 2` atau `args[1] == ""` -> usage; selain itu `return callAndPrint(c, "connector.status", map[string]any{"id": args[1]})`.
     - `"guide"`: sama dengan `status` tetapi method `"connector.guide"`.
   - default (subcommand tak dikenal) -> `return errors.New(connectorUsage)`.
6. `func connectorList(c *ipc.Client) error`:
   - `var rows []connectorRow`; `c.Call("connector.list", nil, &rows)`; kembalikan error bila ada (pesan Indonesia dari daemon diteruskan apa adanya).
   - `len(rows) == 0` -> `fmt.Println("(tidak ada connector)")`, `return nil`.
   - Selain itu `tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)`; header `ID\tNAMA\tSTATE\tUSER\tPESAN`; tiap baris `fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Name, r.Status.State, r.Status.User, r.Status.Message)`; `return tw.Flush()`.
   - Catatan: `User`/`Message` kosong dicetak string kosong (kolom kosong); tidak perlu placeholder. Karena `PESAN` kolom terakhir, tabwriter tidak memadatinya, aman bila kosong.
7. Jalankan `gofmt -l cmd/shikigamictl/cmd_connector.go` (harus kosong) dan `go vet ./cmd/shikigamictl/`.

## Perubahan
- Dibuat: `cmd/shikigamictl/cmd_connector.go`
- Diubah: tidak ada.

## Wiring
Tidak ada yang perlu disambungkan oleh langkah wiring: `init()` di berkas ini memanggil `RegisterCommand` sendiri (overview 5 butir 3). Yang diekspos: command `connector` pada daftar command `shikigamictl` (muncul otomatis di bantuan tanpa argumen lewat `sortedCommands()`), yang bergantung pada method IPC `connector.list|status|guide` dari `connector-core` (modul daemon `wire_connector.go`). Jangan menyunting `registry.go` atau `main.go`.

## Selesai bila
- `go build ./cmd/shikigamictl/...` sukses (guard fitur ini; lalu `go build ./...` bila fitur lain sudah ada). Tanpa unit test.
- `cmd/shikigamictl/cmd_connector.go` tidak mengimpor `internal/connector` dan seluruh identifier barunya berawalan `connector` (tidak ada bentrokan simbol di package `main`).
- `git status` hanya menunjukkan `cmd/shikigamictl/cmd_connector.go` sebagai berkas baru dari fitur ini.
- Perilaku (diverifikasi manual setelah `connector-core` tersedia): `connector list` mencetak tabel dengan baris `fake`; `connector status fake` dan `connector guide fake` mencetak JSON; `connector status nope` menampilkan galat Indonesia dari daemon dengan exit non-nol; tanpa argumen/argumen salah mencetak `penggunaan: shikigamictl connector list|status <id>|guide <id>`; `shikigamictl` tanpa argumen mencantumkan `connector`.
