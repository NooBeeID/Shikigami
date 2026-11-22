# Overview: settings-framework-ticket-02-connector-framework (Kerangka Connector)

Sumber: tiket `docs/tickets/settings-framework-ticket-02-connector-framework.md` dan spec `docs/superpowers/specs/2026-11-22-settings-framework-design.md`. Dokumen ini adalah sumber kebenaran untuk kontrak bersama; plan per fitur hanya merinci langkah kerja. Module Go: `github.com/NooBeeID/shikigami`.

## 1. Ringkasan epic dan urutan eksekusi

Tiket ini menyediakan antarmuka `Connector` generik di Go (registry, status, panduan menyambung) yang diekspos lewat IPC (`connector.list|status|guide`), perintah debug `shikigamictl connector`, serta halaman Connector dan dialog Connect di modal Pengaturan (dibuat tiket 1). Autentikasi memakai CLI yang sudah login di mesin pengguna; app tidak login dan tidak menyimpan token. Bagian 3 (GitLab) dan 4 (Lark) kelak cukup menambah satu file adapter per connector di `internal/connector/`.

Urutan:
1. **Foundation**: TIDAK ada (`hasFoundation=false`, tidak ada `00-foundation.md`). Alasannya: ketiga fitur hanya terhubung lewat kontrak IPC/JSON (teks), bukan lewat impor kode. `connector-cli` hanya memakai `ipc.Client` dan tidak mengimpor `internal/connector`; `connector-ui` hanya memakai `call()`. Prasyarat luar: tiket 1 sudah digabung (ada `modules/settings/` dengan `Modal.svelte`, `pages.ts`, `ConnectorsPlaceholder.svelte`).
2. **Fitur paralel**: `connector-core`, `connector-cli`, `connector-ui`.
3. **Wiring**: verifikasi akhir dan sambungan (bagian 5). Hampir semua wiring sudah berada di ownedPaths fitur (`wire_connector.go`, `init()` `RegisterCommand`, `ConnectorsPlaceholder.svelte`), jadi Wiring terutama memastikan dan menguji end-to-end.

## 2. Kontrak bersama

### 2.1 Go: `internal/connector` (package `connector`)
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
func IsCLIMissing(err error) bool
var ErrCLIMissing error               // sentinel
func SetDataDir(dir string)           // dipanggil wire_connector.go; dibaca fake
func RegisterIPC(srv *ipc.Server)
```
Berkas (tepat lima): `types.go`, `registry.go`, `cli.go`, `ipc.go`, `fake.go`.

Aturan perilaku:
- `registry.go`: map + `sync.RWMutex`; `List` mengembalikan salinan terurut `ID` naik.
- `RunCLI`: timeout default 8 detik (`context.WithTimeout`; deadline pemanggil yang lebih pendek tetap berlaku); `cmd.Env = append(os.Environ(), "PATH="+loginenv.Path())`; stdout dan stderr ditangkap terpisah. `exec.ErrNotFound` (via `errors.Is`) -> `ErrCLIMissing` (dibungkus bila perlu; `IsCLIMissing` memakai `errors.Is`). Exit non-nol -> error berisi stderr yang dipotong (mis. 300 karakter) dengan pesan Indonesia. Timeout (`context.DeadlineExceeded`) -> error berpesan Indonesia (mis. "perintah <name> melewati batas waktu"). Stdout dikembalikan apa adanya (trim dilakukan adapter).
- `fake`: ID `fake`, Name `Contoh (fake)`, mendaftar di `init()`. State dari env `SHIKIGAMI_FAKE_CONNECTOR` (`cli_missing|not_logged_in|connected`); bila kosong/tidak valid, baca file `<DataDir>/connector-fake/state` (isi di-trim); default `not_logged_in`. `User="fake-user"` saat `connected`. `Message` boleh berisi petunjuk Indonesia untuk dua state lain. Guide berisi beberapa langkah teks dan perintah contoh, mis. `mkdir -p "<DataDir>/connector-fake" && echo connected > "<DataDir>/connector-fake/state"` (DataDir dari `SetDataDir`; bila kosong pakai `paths.DataDir()`). Tidak menyimpan/menulis token apa pun.
- Pesan error dan teks Indonesia; tidak ada token/kredensial di respons.

### 2.2 IPC
| Method | Params | Hasil |
|---|---|---|
| `connector.list` | tanpa | `[{"id": string, "name": string, "status": Status}]` (selalu array, bukan null) |
| `connector.status` | `{"id": string}` | `Status` |
| `connector.guide` | `{"id": string}` | `Guide` (`steps`, `commands` selalu array) |

- `connector.list`: status tiap connector dihitung saat dipanggil, paralel (goroutine + `sync.WaitGroup`), timeout per connector (mis. 10 detik, `context.WithTimeout` dari ctx handler), urutan hasil sama dengan `List()`. `Status()` error: `IsCLIMissing(err)` -> `{state: cli_missing}`; selain itu `{state: not_logged_in, message: <alasan Indonesia>}`. Satu kegagalan tidak menggagalkan daftar.
- `connector.status`: aturan error yang sama (jadi `Status`, bukan error IPC).
- ID tak dikenal: `ipc.NewError(ipc.ErrNotFound, "connector tidak dikenal: <id>")`. Param tidak valid (JSON rusak, `id` kosong): `ipc.NewError(ipc.ErrInvalidParams, "...")` berpesan Indonesia.
- Pola handler: `srv.Register("connector.list", func(ctx context.Context, params json.RawMessage) (any, error) {...})` (`ipc.Handler`).

### 2.3 Daemon wiring (`cmd/shikigamid/wire_connector.go`)
```go
func init() {
    RegisterModule("connector", func(srv *ipc.Server, d *Deps) {
        connector.SetDataDir(d.DataDir)
        connector.RegisterIPC(srv)
    })
}
```
Pola sama dengan `wire_status.go`. Tanpa `Provide`.

### 2.4 CLI (`cmd/shikigamictl/cmd_connector.go`)
`RegisterCommand(Command{Name: "connector", Summary: "Connector layanan luar: list|status|guide", Run: runConnector})` dari `init()`; signature `func runConnector(ctx context.Context, c *ipc.Client, args []string) error`. Konstanta `connectorUsage = "penggunaan: shikigamictl connector list|status <id>|guide <id>"` (nama konstanta unik agar tidak bentrok dengan `statusUsage` dll.). `list`: `c.Call("connector.list", nil, &rows)` lalu tabel `text/tabwriter` kolom `ID`, `NAMA`, `STATE`, `USER`, `PESAN`; kosong -> `(tidak ada connector)`. `status <id>` -> `callAndPrint(c, "connector.status", map[string]any{"id": id})`; `guide <id>` -> `connector.guide` serupa. Argumen salah -> `errors.New(connectorUsage)`. Semua nama tipe/fungsi di berkas ini diberi prefix `connector` agar tidak bentrok dalam package `main` (mis. `connectorRow`, `connectorList`).

### 2.5 Frontend (`frontend/src/modules/settings/connectors/`)
```ts
// client.ts
export type ConnectorState = 'cli_missing' | 'not_logged_in' | 'connected';
export interface ConnectorStatus { state: ConnectorState; user?: string; message?: string }
export interface ConnectorGuide { steps: string[]; commands: string[] }
export interface ConnectorInfo { id: string; name: string; status: ConnectorStatus }
export function listConnectors(): Promise<ConnectorInfo[]>;               // call('connector.list')
export function getConnectorStatus(id: string): Promise<ConnectorStatus>; // call('connector.status', { id })
export function getConnectorGuide(id: string): Promise<ConnectorGuide>;   // call('connector.guide', { id })
```
```ts
// store.ts (svelte/store)
export const connectors: Writable<ConnectorInfo[]>;   // awal []
export const connectorsLoading: Writable<boolean>;    // awal false
export const connectorsError: Writable<string | null>; // awal null
export function loadConnectors(): Promise<void>;       // set loading, isi daftar, set error bila gagal (e.message)
export function refreshConnector(id: string): Promise<ConnectorStatus>; // connector.status lalu perbarui HANYA entri id; kembalikan status baru; error dilempar ulang
```
Komponen:
- `ConnectorsPage.svelte` (tanpa props): `onMount(loadConnectors)`; keadaan "Memuat...", galat + tombol "Coba lagi", kosong "Belum ada connector"; baris: nama, badge ("CLI belum terpasang" | "Belum login" | "Tersambung"; tersambung dengan user -> "Tersambung sebagai <user>"), `message` bila ada; tombol **Connect** hanya untuk `cli_missing`/`not_logged_in`, membuka `ConnectDialog` untuk connector itu (state lokal `dialogId`).
- `ConnectDialog.svelte` props: `{ connector: ConnectorInfo; open: boolean; onclose: () => void }`. Memakai `Modal` dari `'../Modal.svelte'` (props `open`, `onclose`, `title?`, `children`). Memuat `getConnectorGuide` saat dibuka; langkah (`<ol>`), perintah dalam blok kode dengan **Salin** (`navigator.clipboard.writeText`, "Tersalin" singkat, kegagalan -> pesan); **Cek ulang** memanggil `refreshConnector(id)` (disabled selama memuat); `connected` -> penanda sukses lalu tutup (`onclose` setelah jeda singkat); belum -> tampilkan status/`message` terbaru; tombol **Tutup**. Tanpa tombol "Buka di terminal".
- `ConnectorsPlaceholder.svelte` (`modules/settings/`, dibuat tiket 1): ganti isi menjadi pembungkus tipis `<ConnectorsPage />` (`import ConnectorsPage from './connectors/ConnectorsPage.svelte'`). Nama file/komponen tetap.
- Jangan membuat `index.ts` di `connectors/` (hindari glob modul; folder ini bukan modul). Impor lintas file memakai path relatif.
- Gaya: variabel CSS yang ada dengan fallback (`--bg-elev`, `--border`, `--fg`, `--fg-muted`, `--bg-hover`). Svelte 5 runes, TypeScript. IPC di UI tidak boleh mengubah fokus/ukuran pane terminal.

### 2.6 Konvensi penamaan
- Go: modul daemon `wire_<nama>.go` + `RegisterModule`; CLI `cmd_<nama>.go` + `RegisterCommand`; adapter masa depan `internal/connector/<id>.go` dengan `Register` di `init()`.
- Teks UI, pesan error, dan komentar bahasa Indonesia; identifier bahasa Inggris.
- Status tidak pernah memuat token; epic tidak menyentuh `internal/appconfig`.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `connector-core` | Paket Go `internal/connector` (types, registry, `RunCLI`, IPC `connector.list/status/guide`, connector `fake`) dan modul daemon `wire_connector`. | `internal/connector/`, `cmd/shikigamid/wire_connector.go` |
| `connector-cli` | Subcommand `shikigamictl connector list\|status\|guide` (tabel dan JSON dari IPC). | `cmd/shikigamictl/cmd_connector.go` |
| `connector-ui` | `client.ts`, `store.ts`, `ConnectorsPage.svelte`, `ConnectDialog.svelte`, dan penggantian isi `ConnectorsPlaceholder.svelte`. | `frontend/src/modules/settings/connectors/`, `frontend/src/modules/settings/ConnectorsPlaceholder.svelte` |

Acceptance criteria per fitur ada di tiket (bagian Fitur) dan menjadi acuan plan masing-masing. Verifikasi end-to-end lintas fitur dilakukan pada tahap Wiring.

## 4. Foundation

Tidak ada. Tidak perlu skeleton paket atau tipe bersama karena kontrak antar-fitur hanya berupa bentuk JSON IPC (bagian 2.2) yang didefinisikan di dokumen ini. Setiap fitur dapat dikompilasi sendiri: `connector-core` (Go murni), `connector-cli` (hanya `ipc`), `connector-ui` (butuh tiket 1 sudah ada, `Modal.svelte` dan `ConnectorsPlaceholder.svelte`).

## 5. Wiring checklist

Dikerjakan setelah ketiga fitur selesai dan digabung. Tidak ada berkas wiring di luar ownedPaths; langkah ini memastikan:
1. `cmd/shikigamid/wire_connector.go` ada, memanggil `RegisterModule("connector", ...)` dengan `connector.SetDataDir(d.DataDir)` lalu `connector.RegisterIPC(srv)`; method `connector.list`, `connector.status`, `connector.guide` muncul di daftar method server (cek lewat daemon berjalan / builtin daftar method).
2. `internal/connector/fake.go` mendaftar `fake` dari `init()` (satu-satunya `Register` di epic ini).
3. `cmd/shikigamictl/cmd_connector.go` mendaftar command `connector` dari `init()`; `shikigamictl` tanpa argumen mencantumkan `connector`.
4. `ConnectorsPlaceholder.svelte` merender `ConnectorsPage`; `SettingsModal.svelte`, `pages.ts`, `store.ts`, `index.ts`, `Modal.svelte`, `SettingsButton.svelte`, `McpPage.svelte`, `internal/appconfig` TIDAK berubah (`git diff` pada path tersebut kosong).
5. Verifikasi manual: jalankan app dan daemon; Pengaturan > Connector menampilkan `fake` (default `not_logged_in`); klik Connect, jalankan perintah contoh (atau set env/file penanda ke `connected`), tekan Cek ulang, badge menjadi "Tersambung sebagai fake-user" dan dialog menandai sukses lalu menutup; coba juga `cli_missing`; `shikigamictl connector list`, `connector status fake`, `connector guide fake`, `connector status nope` (galat Indonesia, exit non-nol); sesi terminal berjalan tidak terganggu.

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- `go build ./...`
- `make build` (termasuk `cd frontend && npm install && npm run build`)
- Selama fase paralel: fitur Go cukup `go build ./...`; `connector-ui` cukup `cd frontend && npm run build`.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, `internal/appconfig`, `McpPage.svelte`, `SettingsModal.svelte`, `Modal.svelte`, `pages.ts`, `store.ts`, `index.ts` di `modules/settings`, `SettingsButton.svelte`, `App.svelte`, `main.ts`.
- Patuhi nama ekspor, signature, nama method IPC, dan bentuk JSON di bagian 2 persis; bila perlu hal di luar kontrak, buat lokal di berkas milik sendiri.
- `connector-cli` tidak boleh mengimpor `internal/connector`; `connector-ui` tidak boleh bergantung pada kode Go.
- Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
