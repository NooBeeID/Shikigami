---
id: settings-framework-ticket-02-connector-framework
title: Kerangka Connector
spec: docs/superpowers/specs/2026-11-22-settings-framework-design.md
status: implemented
wave: 2
depends_on: [settings-framework-ticket-01-settings-modal]
---
# Kerangka Connector

## Tujuan
Menyediakan antarmuka `Connector` generik di Go (registry, status, panduan menyambung) yang diekspos lewat IPC, plus halaman Connector dan dialog Connect di modal Pengaturan. Autentikasi memakai CLI yang sudah login di mesin pengguna (pendekatan A di spec); app tidak login sendiri dan tidak menyimpan token. Epic ini hanya wadahnya: bagian 3 (GitLab) dan bagian 4 (Lark) nanti cukup menambah satu file adapter Go per connector. Sebuah connector contoh (`fake`) disediakan agar daftar, status, dan dialog bisa diverifikasi sebelum adapter nyata ada.

## Hasil yang bisa dijalankan
Di atas tiket 1 (modal Pengaturan dengan halaman Connector berupa placeholder), setelah tiket ini selesai:
- Membuka Pengaturan > Connector menampilkan daftar connector (minimal `fake`) dengan badge status: "CLI belum terpasang", "Belum login", atau "Tersambung" beserta identitas user. Daftar dimuat saat halaman dibuka; ada keadaan memuat dan keadaan galat yang terbaca.
- Connector yang belum tersambung punya tombol **Connect**. Klik membuka dialog berisi langkah-langkah teks, perintah yang bisa disalin (tombol salin ke clipboard), dan tombol **Cek ulang** yang memanggil status lagi, memperbarui daftar, dan menandai sukses/menutup dialog bila sudah tersambung. Tidak ada tombol "Buka di terminal".
- Ketiga state `fake` bisa dicoba tanpa CLI nyata (lewat variabel lingkungan atau file penanda di DataDir), lalu "Cek ulang" mencerminkan perubahan.
- `shikigamictl connector list`, `connector status <id>`, `connector guide <id>` mencetak hasil IPC (alat debug).
- Sesi terminal yang sedang berjalan tidak terganggu saat modal dibuka atau saat status dicek.
- `go build ./...` dan `make build` lulus; aplikasi berjalan.

## Batasan
In scope:
- Paket Go `internal/connector` (tipe, registry, `RunCLI`, IPC, connector `fake`).
- Wiring daemon `wire_connector.go` dan subcommand CLI `cmd_connector.go`.
- Frontend `modules/settings/connectors/` (client, store, `ConnectorsPage`, `ConnectDialog`) dan penggantian isi `ConnectorsPlaceholder.svelte` agar merender `ConnectorsPage`.
- Pesan error dan teks UI dalam bahasa Indonesia; identifier kode dalam bahasa Inggris.

Out of scope:
- Adapter nyata GitLab/Lark, daftar MR/tiket, sidebar (bagian 3 dan 4).
- Pengelolaan server MCP (bagian 2); `McpPage` tidak disentuh.
- Menyimpan token/kredensial atau login oleh app (pendekatan B). Status tidak boleh memuat token.
- Perubahan `AppConfig` (`internal/appconfig`); ditunda ke bagian 2.
- Tombol "Buka di terminal", UI host khusus GitLab self-hosted, connector chat.
- Merombak `SettingsModal`, `Modal.svelte`, `pages.ts`, `store.ts` milik tiket 1.
- Unit test (guard hanya build).

## Kontrak

### Go: `internal/connector`
```go
type State string
const (
    StateCLIMissing  State = "cli_missing"
    StateNotLoggedIn State = "not_logged_in"
    StateConnected   State = "connected"
)

type Status struct {
    State   State  `json:"state"`
    User    string `json:"user,omitempty"`
    Message string `json:"message,omitempty"`
}

type Guide struct {
    Steps    []string `json:"steps"`
    Commands []string `json:"commands"`
}

type Connector interface {
    ID() string
    Name() string
    Status(ctx context.Context) (Status, error)
    ConnectGuide() Guide
}

func Register(c Connector)            // panic bila ID kosong/duplikat
func List() []Connector               // terurut menurut ID
func Get(id string) (Connector, bool)
func RunCLI(ctx context.Context, name string, args ...string) (stdout string, err error)
func IsCLIMissing(err error) bool     // true bila RunCLI gagal karena CLI tidak ditemukan
func RegisterIPC(srv *ipc.Server)
```
- `RunCLI`: context dengan timeout default beberapa detik (mis. 8 detik; hormati deadline context pemanggil bila lebih pendek), env proses ditambah `PATH` dari `internal/loginenv` (`loginenv.Path()`), stdout dan stderr ditangkap. `exec.ErrNotFound` dipetakan ke error sentinel (`ErrCLIMissing`) sehingga adapter memetakan ke `StateCLIMissing`. Exit code non-nol dikembalikan sebagai error yang memuat stderr (dipotong) agar adapter bisa memutuskan `not_logged_in`. Timeout menghasilkan error berpesan Indonesia.
- Adapter: satu file Go per connector dalam `internal/connector/` yang memanggil `Register(...)` dari `init()` (pola dipakai bagian 3 dan 4).

### IPC
- `connector.list` (tanpa param) -> `[{"id": string, "name": string, "status": Status}]`. Status tiap connector dihitung saat dipanggil, paralel dengan timeout per connector; connector yang `Status()`-nya error tidak menggagalkan seluruh daftar: statusnya diisi `cli_missing` bila `IsCLIMissing(err)`, selain itu `not_logged_in` dengan `message` berisi alasan error (Indonesia).
- `connector.status` `{id}` -> `Status`. Error `Status()` non-CLI-missing dikembalikan sebagai `not_logged_in` + `message`, bukan error IPC.
- `connector.guide` `{id}` -> `Guide` (`steps` dan `commands` selalu array, tidak pernah null).
- ID tak dikenal -> `ipc.NewError(ipc.ErrNotFound, "connector tidak dikenal: <id>")`. Param tidak valid -> `ipc.ErrInvalidParams` berpesan Indonesia.
- Respons tidak pernah memuat token/kredensial.

### Connector `fake`
- ID `fake`, Name "Contoh (fake)". State ditentukan oleh variabel lingkungan `SHIKIGAMI_FAKE_CONNECTOR` (`cli_missing|not_logged_in|connected`) atau, bila tidak diset, file penanda di DataDir `connector-fake/state` (isi: nama state). Default tanpa keduanya: `not_logged_in`. `User` terisi (mis. `fake-user`) saat `connected`. `ConnectGuide` berisi beberapa langkah teks dan perintah contoh, mis. `echo connected > <DataDir>/connector-fake/state`, sehingga pengguna dapat menyalinnya lalu menekan "Cek ulang". Tidak menyimpan token apa pun. Akses DataDir lewat `internal/paths` / `Deps.DataDir` (lihat cara modul lain menerimanya); bila DataDir tidak tersedia pada konstruktor, `RegisterIPC` atau modul wire menyetelnya lewat fungsi `fake.SetDataDir`-setara (mis. `connector.SetDataDir(dir)`) yang dipanggil dari `wire_connector.go`.

### Frontend (`frontend/src/modules/settings/connectors/`)
```ts
// client.ts
export type ConnectorState = 'cli_missing' | 'not_logged_in' | 'connected';
export interface ConnectorStatus { state: ConnectorState; user?: string; message?: string }
export interface ConnectorGuide { steps: string[]; commands: string[] }
export interface ConnectorInfo { id: string; name: string; status: ConnectorStatus }
export function listConnectors(): Promise<ConnectorInfo[]>;              // call('connector.list')
export function getConnectorStatus(id: string): Promise<ConnectorStatus>; // call('connector.status', {id})
export function getConnectorGuide(id: string): Promise<ConnectorGuide>;   // call('connector.guide', {id})
```
`store.ts` mengekspor store Svelte: daftar connector + status, `loading`, `error`, serta `loadConnectors()` dan `refreshConnector(id)` (memanggil `connector.status` lalu memperbarui satu entri).

### Dipakai dari tiket 1 dan kode yang ada
- `modules/settings` `registerSettingsPage` (halaman id `connectors`, order 10 sudah terdaftar tiket 1 dengan komponen `ConnectorsPlaceholder`) dan komponen `Modal` (props `open`, `onclose`, `title?`, `children` snippet; Esc dan klik backdrop memanggil `onclose`).
- `cmd/shikigamid` `RegisterModule(name, func(srv *ipc.Server, d *Deps))` dan `ipc.Server` (pola `wire_*.go`).
- `cmd/shikigamictl` `RegisterCommand(Command{Name, Summary, Run})`, `callAndPrint`, `printJSON`.
- `frontend/src/lib/ipc.ts` `call()`.
- `internal/loginenv` untuk PATH login-shell.

## Fitur

### connector-core: Paket Go connector, registry, IPC, dan connector fake
Deskripsi: buat `internal/connector` dengan berkas `types.go` (State, Status, Guide, interface Connector), `registry.go` (`Register`/`List`/`Get` dengan mutex; panic bila ID kosong atau duplikat; `List` terurut), `cli.go` (`RunCLI`, `ErrCLIMissing`, `IsCLIMissing`; timeout default; PATH dari `loginenv.Path()`; stderr dipotong untuk pesan), `ipc.go` (`RegisterIPC(srv)` mendaftarkan `connector.list`, `connector.status`, `connector.guide` sesuai Kontrak; evaluasi status paralel dengan timeout per connector), `fake.go` (connector contoh yang mendaftar diri di `init()`). Buat `cmd/shikigamid/wire_connector.go`: `init()` memanggil `RegisterModule("connector", func(srv *ipc.Server, d *Deps){ connector.SetDataDir(d.DataDir); connector.RegisterIPC(srv) })` (pola `wire_status.go`; tanpa `Provide` kecuali diperlukan). Pesan error bahasa Indonesia.

Acceptance criteria:
- [ ] `internal/connector` terdiri dari `types.go`, `registry.go`, `cli.go`, `ipc.go`, `fake.go` dan terkompilasi.
- [ ] `Register` panic untuk ID duplikat atau kosong; `List` terurut dan `Get` mengembalikan `(c, ok)`.
- [ ] `RunCLI` menerapkan timeout, memetakan `exec.ErrNotFound` ke `ErrCLIMissing`, memakai PATH login-shell, dan mengembalikan stdout.
- [ ] `connector.list` mengembalikan minimal `fake` dengan `id`, `name`, `status`; `connector.status` dan `connector.guide` bekerja untuk `fake`; `steps`/`commands` tidak pernah `null`.
- [ ] ID tak dikenal menghasilkan error IPC `ErrNotFound` dengan pesan Indonesia.
- [ ] `fake` dapat diatur ke ketiga state lewat env `SHIKIGAMI_FAKE_CONNECTOR` atau file penanda di DataDir, dan tidak menulis token apa pun.
- [ ] Modul `connector` terdaftar lewat `wire_connector.go` dan method IPC muncul di daftar method server.
- [ ] Kegagalan satu connector tidak menggagalkan `connector.list`.

ownedPaths: `internal/connector/`, `cmd/shikigamid/wire_connector.go`.
Paralel: bisa dikerjakan paralel dengan `connector-cli` dan `connector-ui` berdasarkan kontrak IPC di atas.

### connector-cli: Perintah shikigamictl connector
Deskripsi: `cmd/shikigamictl/cmd_connector.go` mendaftarkan `RegisterCommand(Command{Name:"connector", Summary:"Connector layanan luar: list|status|guide", Run: runConnector})` dari `init()`. `connector list` memanggil `connector.list` dan mencetak tabel (`ID`, `NAMA`, `STATE`, `USER`, `PESAN`) dengan `text/tabwriter`, atau `(tidak ada connector)` bila kosong. `connector status <id>` memanggil `connector.status {id}` dan `connector guide <id>` memanggil `connector.guide {id}`; keduanya mencetak JSON lewat `callAndPrint`. Argumen salah mencetak `penggunaan: shikigamictl connector list|status <id>|guide <id>` sebagai error (pola `statusUsage`).

Acceptance criteria:
- [ ] `shikigamictl connector list` mencetak tabel berisi connector `fake`.
- [ ] `shikigamictl connector status fake` dan `connector guide fake` mencetak JSON hasil IPC.
- [ ] ID tak dikenal menampilkan pesan error Indonesia dari daemon dengan exit code non-nol.
- [ ] Tanpa argumen atau argumen salah menampilkan usage.
- [ ] `shikigamictl` tanpa argumen mencantumkan `connector` di daftar command.

ownedPaths: `cmd/shikigamictl/cmd_connector.go`.
Paralel: bisa dikerjakan paralel dengan fitur lain; verifikasi end-to-end setelah `connector-core` jadi.

### connector-ui: Halaman Connector dan dialog Connect
Deskripsi: folder `frontend/src/modules/settings/connectors/` berisi:
- `client.ts` dan `store.ts` sesuai Kontrak.
- `ConnectorsPage.svelte`: daftar connector; tiap baris menampilkan nama, badge status (teks Indonesia: "CLI belum terpasang", "Belum login", "Tersambung"; untuk tersambung tampilkan identitas user, mis. "Tersambung sebagai <user>"), dan `message` bila ada. Tombol **Connect** hanya pada yang belum tersambung. Memanggil `loadConnectors()` saat halaman di-mount/dibuka; menampilkan "Memuat..." dan pesan galat dengan tombol "Coba lagi"; keadaan kosong "Belum ada connector". Gaya mengikuti variabel CSS yang ada (`--bg-elev`, `--border`, dst).
- `ConnectDialog.svelte`: memakai `Modal` generik dari tiket 1. Saat dibuka memuat `connector.guide`; menampilkan langkah (daftar bernomor) dan perintah dalam blok kode dengan tombol **Salin** (`navigator.clipboard.writeText`, umpan balik "Tersalin" singkat; tangani kegagalan clipboard dengan pesan). Tombol **Cek ulang** memanggil `refreshConnector(id)` (menonaktifkan tombol selama memuat); bila state menjadi `connected`, tampilkan penanda sukses lalu tutup dialog; bila belum, tampilkan status terbaru dan `message`. Tombol **Tutup**; Esc dan klik backdrop menutup (dari `Modal`). Fokus dan urutan tab wajar; tanpa tombol "Buka di terminal".
- Wiring: ubah `ConnectorsPlaceholder.svelte` menjadi pembungkus tipis yang merender `ConnectorsPage` (komponen tetap dipakai oleh registrasi halaman `connectors` dari tiket 1, sehingga `SettingsModal`, `pages.ts`, dan `index.ts` tidak diubah).
- Panggilan IPC saat dialog/halaman terbuka tidak boleh mengganggu terminal yang berjalan (tanpa mengubah fokus/ukuran pane).

Acceptance criteria:
- [ ] `client.ts` memanggil `connector.list`, `connector.status`, `connector.guide` lewat `call()` dengan tipe sesuai Kontrak.
- [ ] `store.ts` menyimpan daftar + status, `loading`, `error`, dan `refreshConnector(id)` hanya memperbarui satu entri.
- [ ] Halaman Connector menampilkan badge ketiga state dengan teks Indonesia dan identitas user saat tersambung.
- [ ] Tombol Connect muncul hanya untuk state `cli_missing` dan `not_logged_in`, dan membuka `ConnectDialog` untuk connector tersebut.
- [ ] Dialog menampilkan langkah dan perintah; tombol Salin menyalin perintah ke clipboard.
- [ ] "Cek ulang" memperbarui store dan badge di halaman; dialog menandai sukses/menutup saat `connected`.
- [ ] Esc dan klik backdrop menutup dialog; tidak ada tombol "Buka di terminal".
- [ ] `ConnectorsPlaceholder.svelte` merender `ConnectorsPage`; halaman muncul di modal Pengaturan > Connector.
- [ ] Frontend build (`npm run build` di `frontend/`) lulus tanpa error.

ownedPaths: `frontend/src/modules/settings/connectors/`, `frontend/src/modules/settings/ConnectorsPlaceholder.svelte`.
Paralel: bisa dikerjakan paralel dengan `connector-core` berdasarkan kontrak IPC; verifikasi end-to-end setelah keduanya jadi.

## Path yang dimiliki
Hanya path berikut yang boleh disentuh:
- `internal/connector/`
- `cmd/shikigamid/wire_connector.go`
- `cmd/shikigamictl/cmd_connector.go`
- `frontend/src/modules/settings/connectors/`
- `frontend/src/modules/settings/ConnectorsPlaceholder.svelte` (hanya mengganti isi agar merender `ConnectorsPage`; berkas ini dibuat tiket 1)

Jangan menyentuh `internal/appconfig`, `McpPage.svelte`, `SettingsModal.svelte`, `Modal.svelte`, `pages.ts`, `store.ts`, `index.ts` di `modules/settings`, maupun `SettingsButton.svelte`.

## Wiring
Langkah penutup tiket ini:
1. Daemon: `cmd/shikigamid/wire_connector.go` mendaftarkan modul `connector` lewat `RegisterModule` sehingga `connector.list/status/guide` aktif; connector `fake` mendaftar diri dari `init()` di `internal/connector/fake.go`.
2. CLI: `cmd/shikigamictl/cmd_connector.go` mendaftarkan command `connector` lewat `RegisterCommand` dari `init()`.
3. Frontend: `ConnectorsPlaceholder.svelte` (sudah dipakai `registerSettingsPage` id `connectors` dari tiket 1) merender `ConnectorsPage`, jadi Pengaturan > Connector langsung menampilkan daftar dan dialog Connect tanpa mengubah berkas tiket 1 lainnya.
4. Verifikasi manual: jalankan app, buka Pengaturan > Connector, lihat `fake` pada state default `not_logged_in`, klik Connect, jalankan perintah contoh dari dialog (atau set env/file penanda ke `connected`), tekan Cek ulang dan pastikan badge berubah menjadi Tersambung serta dialog menandai sukses. Cek juga `shikigamictl connector list`.
Tiket ini bergantung pada tiket 1 hanya untuk modal dan registrasi halaman; seluruh fitur di dalamnya terlihat dan jalan setelah tiket 1 selesai.

## Guard
- `go build ./...` lulus.
- `make build` lulus (termasuk build frontend).
- Tanpa unit test.
