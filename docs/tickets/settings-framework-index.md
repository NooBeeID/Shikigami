# Indeks Tiket: Kerangka Pengaturan (Semua pengaturan, Connector, MCP)

Sumber spec: `docs/superpowers/specs/2026-11-22-settings-framework-design.md`

Epic ini adalah bagian 1 dari 4 untuk fitur Connector. Bagian 2 (Menu MCP), 3 (Connector GitLab), dan 4 (Connector Lark) dikerjakan terpisah dan berurutan setelah epic ini.

## Ringkasan tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|---|---|---|---|---|
| settings-framework-ticket-01-settings-modal | Modal Pengaturan | 1 | - | 3 |
| settings-framework-ticket-02-connector-framework | Kerangka Connector | 2 | settings-framework-ticket-01-settings-modal | 3 |

Fitur per tiket:

- **Modal Pengaturan** (ticket-01): `settings-foundation`, `settings-shell-pages`, `settings-gear-entry`
- **Kerangka Connector** (ticket-02): `connector-core`, `connector-cli`, `connector-ui`

## Urutan eksekusi

- **Wave 1:** `settings-framework-ticket-01-settings-modal`
- **Wave 2:** `settings-framework-ticket-02-connector-framework` (menunggu ticket-01 selesai)

Tidak ada tiket yang paralel dalam epic ini karena setiap wave hanya berisi satu tiket.

## Kontrak bersama

### 1. Frontend settings (epic settings-modal -> connector-framework)

- `modules/settings/store.ts` mengekspor `openSettings(pageId?: string)`, `closeSettings()`, serta store `settingsOpen` dan `activePage`.
- `modules/settings/pages.ts` mengekspor `registerSettingsPage({ id: string; title: string; order?: number; component: Component })`.
- Halaman terdaftar: `connectors` (order 10) dan `mcp` (order 20).
- `Modal.svelte` menerima props: `open: boolean`, `onclose: () => void`, `title?: string`, dan `children` snippet. Tombol Esc dan klik backdrop memanggil `onclose`.
- Host modal dipasang lewat `registerPanel` pada area `statusbar`.

### 2. Backend Go: paket `internal/connector`

```go
type State string // "cli_missing" | "not_logged_in" | "connected"

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
```

- `connector.Register(c)` dipanggil dari `init()` setiap adapter. Satu file per connector. Bagian 3 (GitLab) dan bagian 4 (Lark) hanya menambah adapter dengan cara ini.

### 3. IPC

- `connector.list` (tanpa parameter) mengembalikan `[{id, name, status: Status}]`.
- `connector.status` dengan parameter `{id}` mengembalikan `Status`.
- `connector.guide` dengan parameter `{id}` mengembalikan `Guide`.
- ID yang tidak dikenal menghasilkan error IPC bawaan "not found" dengan pesan berbahasa Indonesia.
- `Status` tidak pernah memuat token atau kredensial. App tidak menyimpan token layanan.

### 4. Konfigurasi

- Epic ini tidak mengubah `AppConfig` (`internal/appconfig`).
- Perluasan config untuk MCP dan connector ditunda ke bagian 2.

### 5. Konvensi

- Modul daemon didaftarkan lewat `RegisterModule` di `wire_<nama>.go`.
- Perintah CLI didaftarkan lewat `RegisterCommand` di `cmd_<nama>.go`.
- Panel frontend didaftarkan lewat `registerPanel` di `modules/<nama>/index.ts`.
- Guard verifikasi hanya build: `go build ./...` dan `make build`. Tidak ada unit test.
