# Plan: Paket Go connector, registry, IPC, dan connector fake

## Metadata
- Tiket: `settings-framework-ticket-02-connector-framework` (`docs/tickets/settings-framework-ticket-02-connector-framework.md`)
- Fitur: `connector-core`
- ownedPaths: `internal/connector/`, `cmd/shikigamid/wire_connector.go`
- Depends on foundation: tidak (`hasFoundation=false`; paket Go murni, hanya memakai `internal/ipc`, `internal/loginenv`, `internal/paths` yang sudah ada)
- Module Go: `github.com/NooBeeID/shikigami`

## Tujuan
Menyediakan paket `internal/connector`: tipe `State/Status/Guide/Connector`, registry, `RunCLI` (pemanggil CLI dengan timeout dan PATH login-shell), IPC `connector.list|status|guide`, dan connector contoh `fake`; plus modul daemon `wire_connector.go`. Adapter nyata (GitLab/Lark) kelak cukup menambah satu file di paket ini. Autentikasi memakai CLI yang sudah login; tidak ada token/kredensial disimpan atau dikembalikan.

## Path yang dimiliki
HANYA path ini yang boleh disentuh:
- `internal/connector/` (buat tepat lima berkas: `types.go`, `registry.go`, `cli.go`, `ipc.go`, `fake.go`)
- `cmd/shikigamid/wire_connector.go` (berkas baru)

Jangan menyentuh `cmd/shikigamictl/`, `frontend/`, `internal/appconfig`, `internal/ipc`, `internal/loginenv`, `internal/paths`, `cmd/shikigamid/modules.go`/`main.go`, maupun `wire_*.go` lain. Jangan menambah unit test. Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.

## Kontrak
Persis sesuai `overview.md` bagian 2.1 (tipe Go dan aturan perilaku), 2.2 (IPC) dan 2.3 (wiring daemon). Ringkasan nama yang HARUS ada:

Disediakan:
- Tipe: `State` (`StateCLIMissing="cli_missing"`, `StateNotLoggedIn="not_logged_in"`, `StateConnected="connected"`), `Status{State,User,Message}` (tag json `state`, `user,omitempty`, `message,omitempty`), `Guide{Steps,Commands []string}` (json `steps`, `commands`), interface `Connector{ID,Name,Status(ctx),ConnectGuide}`.
- Fungsi/var: `Register`, `List`, `Get`, `RunCLI`, `IsCLIMissing`, `ErrCLIMissing`, `SetDataDir`, `RegisterIPC`.
- Method IPC: `connector.list` (`[{id,name,status}]`), `connector.status` (`{id}` -> `Status`), `connector.guide` (`{id}` -> `Guide`).
- Connector `fake`: ID `fake`, Name `Contoh (fake)`.
- Pesan error IPC: ID tak dikenal `ipc.NewError(ipc.ErrNotFound, "connector tidak dikenal: <id>")`; param rusak/`id` kosong `ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh {\"id\": string}")`.

Dipakai (sudah ada di repo):
- `ipc.Server.Register(method string, h ipc.Handler)` dengan `type Handler func(ctx context.Context, params json.RawMessage) (any, error)`; `ipc.NewError(code int, msg string) *ipc.Error`; konstanta `ipc.ErrNotFound`, `ipc.ErrInvalidParams`.
- `loginenv.Path() string`.
- `paths.DataDir() (string, error)` (fallback DataDir fake).
- `RegisterModule(name string, func(srv *ipc.Server, d *Deps))` dan `Deps.DataDir` di `package main` pada `cmd/shikigamid` (pola `wire_status.go`).

## Langkah
Sebelum menulis, lihat sekilas `internal/ipc/errors.go`, `internal/ipc/server.go` dan `cmd/shikigamid/wire_status.go` untuk memastikan signature. Semua komentar/pesan bahasa Indonesia, identifier bahasa Inggris.

1. `internal/connector/types.go` (`package connector`): definisikan `State`, tiga konstanta, `Status`, `Guide`, `Connector` persis seperti overview 2.1. Tambahkan komentar doc singkat.

2. `internal/connector/registry.go`:
   - `var (mu sync.RWMutex; registry = map[string]Connector{})`.
   - `func Register(c Connector)`: ambil `id := c.ID()`; `panic("connector: ID kosong")` bila kosong; `panic("connector: ID duplikat: " + id)` bila sudah ada; simpan di bawah `mu.Lock()`.
   - `func List() []Connector`: `RLock`, salin value ke slice, `sort.Slice` menurut `ID()` naik; kembalikan salinan (slice tidak nil).
   - `func Get(id string) (Connector, bool)`: `RLock`, lookup map.

3. `internal/connector/cli.go`:
   - `var ErrCLIMissing = errors.New("CLI belum terpasang")`.
   - Konstanta: `cliTimeout = 8 * time.Second`, `stderrMax = 300`.
   - `func RunCLI(ctx context.Context, name string, args ...string) (string, error)`:
     - `ctx, cancel := context.WithTimeout(ctx, cliTimeout); defer cancel()` (deadline pemanggil yang lebih pendek otomatis tetap berlaku).
     - `cmd := exec.CommandContext(ctx, name, args...)`; `cmd.Env = append(os.Environ(), "PATH="+loginenv.Path())`; `cmd.Stdout`/`cmd.Stderr` ke dua `bytes.Buffer` terpisah.
     - Catatan resolusi biner: `exec.Command` me-resolve `name` dengan PATH proses daemon, bukan `cmd.Env`. Agar PATH login-shell dipakai, sebelum membuat command lakukan `exec.LookPath` terhadap PATH login: set sementara tidak aman, jadi cari manual: iterasi `filepath.SplitList(loginenv.Path())`, cek `filepath.Join(dir, name)` ada, bukan direktori, dan mode eksekusi (`info.Mode()&0o111 != 0`); bila `name` sudah mengandung pemisah path, pakai apa adanya. Bila tidak ditemukan di mana pun -> `return "", fmt.Errorf("%w: %s", ErrCLIMissing, name)`. Bila ketemu, pakai path absolut itu untuk `exec.CommandContext`. Tetap petakan juga `errors.Is(err, exec.ErrNotFound)` dan `errors.Is(err, os.ErrNotExist)` dari `cmd.Run()` ke `ErrCLIMissing` sebagai jaring pengaman.
     - Setelah `cmd.Run()`: bila `ctx.Err() == context.DeadlineExceeded` -> `fmt.Errorf("perintah %s melewati batas waktu", name)`. Bila error lain (`*exec.ExitError`) -> `fmt.Errorf("perintah %s gagal: %s", name, truncate(strings.TrimSpace(stderr), stderrMax))` (bila stderr kosong pakai `err.Error()`). Sukses -> `return stdout.String(), nil` apa adanya (tanpa trim).
   - `func truncate(s string, n int) string`: potong berdasarkan rune, tambahkan `"..."` bila terpotong.
   - `func IsCLIMissing(err error) bool { return errors.Is(err, ErrCLIMissing) }`.

4. `internal/connector/ipc.go`:
   - `var dataDir string` dengan `sync.RWMutex` kecil (atau `atomic.Value`) dan `func SetDataDir(dir string)` + helper internal `getDataDir() string` (dibaca fake). Letakkan `SetDataDir` di berkas ini atau `fake.go` (pilih satu; kontrak tidak mengikat berkas).
   - Konstanta `perConnectorTimeout = 10 * time.Second`.
   - Tipe `type info struct { ID string \`json:"id"\`; Name string \`json:"name"\`; Status Status \`json:"status"\` }`.
   - `func resolveStatus(ctx context.Context, c Connector) Status`: `ctx, cancel := context.WithTimeout(ctx, perConnectorTimeout)`; `st, err := c.Status(ctx)`; bila `err != nil`: `IsCLIMissing(err)` -> `Status{State: StateCLIMissing}`; selain itu `Status{State: StateNotLoggedIn, Message: err.Error()}`. Sukses -> `st`. Lindungi dari panic adapter dengan `defer recover` yang menghasilkan `Status{State: StateNotLoggedIn, Message: "status connector gagal dihitung"}`. Pastikan Message tidak berisi token (jangan sertakan env/argumen).
   - `func normalizeGuide(g Guide) Guide`: ganti `nil` pada `Steps`/`Commands` dengan `[]string{}`.
   - `func parseID(params json.RawMessage) (string, error)`: unmarshal `struct{ID string \`json:"id"\`}`; gagal atau `ID==""` -> `ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh {\"id\": string}")`.
   - `func lookup(id string) (Connector, error)`: `Get`; tidak ada -> `ipc.NewError(ipc.ErrNotFound, "connector tidak dikenal: "+id)`.
   - `func RegisterIPC(srv *ipc.Server)` mendaftarkan:
     - `connector.list`: `cs := List()`; `out := make([]info, len(cs))`; satu goroutine per connector dengan `sync.WaitGroup` mengisi `out[i] = info{c.ID(), c.Name(), resolveStatus(ctx, c)}` (tulis ke indeks sendiri, tanpa race); `wg.Wait()`; kembalikan `out` (selalu slice non-nil, serialisasi `[]`).
     - `connector.status`: `parseID` -> `lookup` -> `resolveStatus(ctx, c)`.
     - `connector.guide`: `parseID` -> `lookup` -> `normalizeGuide(c.ConnectGuide())`.
     - Error dikembalikan sebagai `*ipc.Error` langsung (cek pola `builtin.go` agar tipe pengembalian `error` benar, hindari typed-nil).

5. `internal/connector/fake.go`:
   - `type fakeConnector struct{}`; `ID() "fake"`; `Name() "Contoh (fake)"`.
   - `init() { Register(fakeConnector{}) }` (satu-satunya `Register` di epic ini).
   - Helper `fakeStateDir() string`: `getDataDir()`; bila kosong pakai `paths.DataDir()` (abaikan error -> string kosong); kembalikan `filepath.Join(dir, "connector-fake")`.
   - Helper `validState(s string) (State, bool)` untuk `cli_missing|not_logged_in|connected`.
   - `Status(ctx)`: baca env `SHIKIGAMI_FAKE_CONNECTOR` (trim); bila valid pakai; bila kosong/tidak valid baca file `filepath.Join(fakeStateDir(), "state")` (trim, valid); default `StateNotLoggedIn`. Hasil: `connected` -> `Status{State: StateConnected, User: "fake-user"}`; `cli_missing` -> `Message: "CLI contoh belum terpasang (simulasi)"`; `not_logged_in` -> `Message: "Belum login. Ikuti langkah di dialog Connect."`. Hanya membaca, tidak pernah menulis file/token.
   - `ConnectGuide()`: `Steps` beberapa langkah Indonesia (mis. "Ini connector contoh untuk menguji tampilan.", "Salin perintah di bawah lalu jalankan di terminal.", "Kembali ke sini dan tekan Cek ulang."); `Commands` berisi satu perintah dengan DataDir aktual: `mkdir -p "<dir>" && echo connected > "<dir>/state"` (di mana `<dir>` = `fakeStateDir()`; bila DataDir tak diketahui pakai `$HOME/.shikigami`-agnostik: gunakan placeholder `<DataDir>/connector-fake`). Tambahkan opsional perintah kedua untuk reset: `echo not_logged_in > "<dir>/state"`. Slice tidak pernah nil.

6. `cmd/shikigamid/wire_connector.go` (`package main`):
   ```go
   package main

   import (
       "github.com/NooBeeID/shikigami/internal/connector"
       "github.com/NooBeeID/shikigami/internal/ipc"
   )

   func init() {
       RegisterModule("connector", func(srv *ipc.Server, d *Deps) {
           connector.SetDataDir(d.DataDir)
           connector.RegisterIPC(srv)
       })
   }
   ```
   Tanpa `Provide`/`OnReady`.

7. Build dan rapikan: `gofmt -l internal/connector cmd/shikigamid/wire_connector.go` (harus kosong), `go vet ./internal/connector/ ./cmd/shikigamid/`, lalu `go build ./...`. Pastikan tidak ada identifier di `package main` yang bentrok (hanya `init`).

## Perubahan
Dibuat:
- `internal/connector/types.go`
- `internal/connector/registry.go`
- `internal/connector/cli.go`
- `internal/connector/ipc.go`
- `internal/connector/fake.go`
- `cmd/shikigamid/wire_connector.go`

Tidak ada berkas yang diubah.

## Wiring
Fitur ini mengekspos:
- `connector.RegisterIPC(srv)` dan `connector.SetDataDir(dir)` untuk modul daemon; keduanya SUDAH dipanggil oleh `wire_connector.go` (berkas milik fitur ini), jadi tidak ada sambungan manual tersisa di luar ownedPaths.
- Method IPC `connector.list`, `connector.status`, `connector.guide` untuk dipakai `connector-cli` (lewat `ipc.Client`) dan `connector-ui` (lewat `call()`).
- `Register(Connector)` untuk adapter masa depan (dipanggil dari `init()` file adapter); hanya `fake` yang terdaftar sekarang.

Langkah wiring (tahap akhir epic, bukan dikerjakan di sini): verifikasi method muncul di daftar method server dan end-to-end sesuai overview bagian 5. Jangan menyambungkan apa pun di luar ownedPaths.

## Selesai bila
- `internal/connector/` berisi tepat `types.go`, `registry.go`, `cli.go`, `ipc.go`, `fake.go`; `cmd/shikigamid/wire_connector.go` ada.
- `go build ./...` sukses (guard fitur ini; tanpa unit test), `gofmt` dan `go vet` pada path fitur bersih.
- `Register` panic untuk ID kosong/duplikat; `List` terurut dan salinan; `Get` mengembalikan `(c, ok)`.
- `RunCLI`: timeout 8 detik, PATH dari `loginenv.Path()`, CLI tak ditemukan -> `ErrCLIMissing` (`IsCLIMissing` true), exit non-nol -> error Indonesia dengan stderr dipotong 300 karakter, stdout dikembalikan apa adanya.
- `connector.list` selalu array berisi minimal `fake` (paralel, timeout per connector, kegagalan satu connector tidak menggagalkan daftar); `connector.status` dan `connector.guide` bekerja; `steps`/`commands` tidak pernah `null`; ID tak dikenal -> `ErrNotFound` "connector tidak dikenal: <id>"; param rusak -> `ErrInvalidParams` Indonesia.
- `fake` mengikuti env `SHIKIGAMI_FAKE_CONNECTOR`, lalu file `<DataDir>/connector-fake/state`, default `not_logged_in`, `fake-user` saat `connected`; tidak menulis token apa pun.
- `git status` hanya menunjukkan berkas di ownedPaths yang berubah.
