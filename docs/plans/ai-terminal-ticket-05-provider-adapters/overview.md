# Overview: Epic 05 Provider Adapters

Sumber: tiket `docs/tickets/ai-terminal-ticket-05-provider-adapters.md` dan spec `docs/superpowers/specs/2026-11-21-ai-terminal-design.md` (bagian Provider, 9b). Tiket adalah sumber kebenaran; bila overview ini bertentangan dengan tiket, tiket menang, kecuali pada tiga koreksi eksplisit di bagian 2.8 (hasil membaca kode nyata).

## 1. Ringkasan epic dan urutan eksekusi

Epic ini menambah paket `internal/provider` (adapter tipis claude, codex, shell dengan model kapabilitas), titik ekstensi `RegisterLaunchContribution`, implementasi `session.Launcher` yang mendelegasikan ke provider, method IPC `provider.list`, validasi `providerId` pada `session.create`, dan perintah CLI `aitermctl provider ...`. Hanya stdlib Go; `go.mod` tidak berubah.

Urutan eksekusi:

1. **Foundation** (satu agent, lihat `00-foundation.md`): `provider.go` final (tipe, interface, helper) serta stub `registry.go` dan `launch.go` dengan signature persis kontrak, sehingga `go build ./...` hijau dan ketiga fitur bisa jalan paralel.
2. **Fitur paralel**: `provider-core`, `provider-impls`, `provider-ipc` (semua hanya butuh hasil Foundation).
3. **Wiring** (satu agent, bagian 5): dua edit kecil ke file epic 02 yang diperlukan agar launcher dan validator terpasang, verifikasi `make build`, dan smoke test.

Kondisi kode yang sudah ada (epic 01, 02, 03 sudah merge; epic 04 di worktree):
- Module `aiterm`, Go 1.26.3, deps sudah ada (`creack/pty`, dst).
- `internal/ipc` (`Server.Register` PANIC bila method sudah terdaftar; `ipc.NewError`, `ipc.Errorf`, konstanta `ipc.ErrBinary=-32000`, `ErrConflict`, `ErrNotFound`, `ErrInvalid=-32003`, `ErrInvalidParams`).
- `internal/paths` (`paths.DataDir()` membuat dir 0700).
- `internal/session`: `SessionSpec`, `Launcher`, `DefaultLauncher()`, `Manager.SetLauncher(l)`, `Manager.Create` (memanggil `Launcher.Command(spec)` SEBELUM ID sesi dibuat), `session.RegisterIPC(srv, m)`. `session.create` sudah menerima `providerId`; `Manager.Create` mengisi default `"shell"` bila kosong.
- `cmd/aitermd/modules.go`: `RegisterModule`, `Deps{DataDir, SocketPath, Logger, Version, Started}` dengan `Provide/Lookup/OnReady`. `wireAll` memasang modul berurutan ALFABETIS ("provider" dipasang SEBELUM "session"), lalu menjalankan callback `OnReady`.
- `cmd/aitermctl/registry.go`: `RegisterCommand(Command{Name, Summary, Run func(ctx, c *ipc.Client, args []string) error})` (panic bila nama dobel), helper `printJSON`, `callAndPrint`.
- `cmd/aitermctl/cmd_session.go` sudah punya flag `--provider` pada `session new`, sehingga `aitermctl session new --provider <id>` sudah jalan tanpa edit.
- `internal/provider/` belum ada.

## 2. Kontrak bersama

### 2.1 Paket dan path

- Import path: `aiterm/internal/provider` (nama paket `provider`).
- File: `internal/provider/{provider.go,registry.go,launch.go,claude.go,codex.go,shell.go,ipc.go}`, `cmd/aitermd/wire_provider.go`, `cmd/aitermctl/cmd_provider.go`.
- Identifier Go bahasa Inggris; komentar dan pesan error/CLI bahasa Indonesia (selaras kode yang ada). JSON camelCase.
- Nama tipe impl (privat): `claudeProvider`, `codexProvider`, `shellProvider`. Tipe fallback privat di registry: `fallbackProvider`. Jangan memakai nama lain agar tidak bentrok.

### 2.2 provider.go (milik Foundation, final; fitur provider-core hanya boleh menambah helper, tidak mengubah signature)

```go
package provider

const (
    IDClaude = "claude"
    IDCodex  = "codex"
    IDShell  = "shell"
)

type Capabilities struct {
    Hooks     bool `json:"hooks"`
    MCP       bool `json:"mcp"`
    IdleEvent bool `json:"idleEvent"`
}

type Provider interface {
    ID() string
    Name() string
    Capabilities() Capabilities
    Command(ctx *LaunchContext) (cmd string, args []string, env []string)
    SendInput(w io.Writer, text string) error // bracketed paste + Enter
    Available() (path string, err error)
}

// BracketedPaste mengembalikan "\x1b[200~" + text + "\x1b[201~" + "\r" (Enter SUDAH termasuk).
func BracketedPaste(text string) []byte

// WriteInput menulis BracketedPaste(text) ke w dalam SATU panggilan Write
// (tanpa Enter tambahan). Dipakai oleh SendInput ketiga provider.
func WriteInput(w io.Writer, text string) error

// LookPath membungkus exec.LookPath; error: `binary "<name>" tidak ditemukan di PATH`.
func LookPath(name string) (string, error)

// BinaryError membangun error `provider "<id>": binary "<bin>" tidak ditemukan di PATH; pasang atau tambahkan ke PATH`.
func BinaryError(providerID, bin string) error
```

Catatan penting: `BracketedPaste` sudah diakhiri `\r`, jadi `SendInput` cukup `WriteInput(w, text)`. Jangan menulis `\r` kedua.

### 2.3 registry.go

```go
func Register(p Provider)          // PANIC bila id kosong atau sudah terdaftar
func Get(id string) (Provider, bool)
func List() []Provider             // urut stabil: claude, codex, shell, lalu id lain urut alfabet
func Default() Provider            // provider "shell"; bila belum terdaftar, fallbackProvider (login shell $SHELL -l, tidak panic)
```

Peta dilindungi `sync.RWMutex`. Urutan `List()` ditentukan oleh daftar urutan tetap, BUKAN urutan `init()` file.

### 2.4 launch.go

```go
type LaunchContext struct {
    Session    session.SessionSpec
    Provider   Provider
    Env        map[string]string // tidak pernah nil saat diberikan ke contribution/provider
    Args       []string
    MCPServers map[string]any    // tidak pernah nil saat diberikan; nama -> konfigurasi server MCP
}

type LaunchContribution func(*LaunchContext)
func RegisterLaunchContribution(fn LaunchContribution) // nil diabaikan; aman konkuren dan aman dipanggil dari init()

type Launcher struct{}
var _ session.Launcher = Launcher{}
func (Launcher) Command(s session.SessionSpec) (string, []string, []string)
func NewLauncher() session.Launcher // return Launcher{}
```

Alur `Launcher.Command(s)`:
1. `p, ok := Get(s.ProviderID)`; bila `!ok` (termasuk kosong) -> `p = Default()`.
2. `ctx := &LaunchContext{Session: s, Provider: p, Env: map{}, MCPServers: map{}}` (`Args` nil).
3. Jalankan semua contribution berurutan sesuai urutan registrasi (salin slice di bawah RLock, panggil di luar lock).
4. `cmd, args, extra := p.Command(ctx)`.
5. Env akhir (urutan, entri belakangan menang karena os/exec membuang duplikat dengan mempertahankan yang terakhir): `os.Environ()`, lalu `TERM=xterm-256color`, `COLORTERM=truecolor` (sama dengan `session.DefaultLauncher`), lalu `s.Env`, lalu `ctx.Env` sebagai `KEY=VALUE` terurut kunci, lalu `extra` dari provider. Jadi provider cukup mengembalikan env tambahan khusus provider (umumnya `nil`); Launcher yang menggabungkan `ctx.Env`.
6. Kembalikan `cmd, args, env`.

Contribution dijalankan SEBELUM `Provider.Command`. Provider membaca `ctx.Args`, `ctx.Env`, `ctx.MCPServers` yang sudah diperkaya.

### 2.5 Helper env

`func envList(m map[string]string) []string` (privat, di `launch.go`, milik provider-core; urut kunci, `KEY=VALUE`). Provider impl tidak memakainya.

### 2.6 ipc.go (milik provider-ipc)

```go
func RegisterIPC(srv *ipc.Server)             // mendaftarkan "provider.list"
func Validate(providerID string) error        // dipakai guard session.create
```

`provider.list`: params `{}` (atau kosong), hasil array (tidak pernah null; kosong -> `[]`):

```json
[{"id":"claude","name":"Claude Code","capabilities":{"hooks":true,"mcp":true,"idleEvent":true},"available":true,"path":"/usr/local/bin/claude"},
 {"id":"codex","name":"Codex","capabilities":{"hooks":false,"mcp":false,"idleEvent":false},"available":false,"error":"provider \"codex\": binary \"codex\" tidak ditemukan di PATH; pasang atau tambahkan ke PATH"}]
```

Tipe: `type Info struct { ID string \`json:"id"\`; Name string \`json:"name"\`; Capabilities Capabilities \`json:"capabilities"\`; Available bool \`json:"available"\`; Path string \`json:"path,omitempty"\`; Error string \`json:"error,omitempty"\` }`. Bila `available:false`, `path` kosong dan `error` terisi.

`Validate(id)`:
- `id == ""` -> pakai `shell` (cek ketersediaan shell juga).
- `Get(id)` gagal -> `ipc.Errorf(ipc.ErrInvalid, "providerId %q tidak dikenal; yang tersedia: claude, codex, shell", id)` (daftar dari `List()`).
- `Available()` error -> `ipc.NewError(ipc.ErrBinary, <pesan error>)`; pesan tetap berbentuk `provider "claude": binary "claude" tidak ditemukan di PATH; pasang atau tambahkan ke PATH`.
- Sukses -> `nil`. Selalu mengembalikan `*ipc.Error` untuk kegagalan, supaya diteruskan apa adanya oleh `mapErr` milik session.

### 2.7 Konfigurasi MCP claude (milik provider-impls)

`claudeProvider.Command(ctx)`:
- `cmd = "claude"`, `args = ctx.Args` (salinan).
- Bila `len(ctx.MCPServers) > 0`: `dir := filepath.Join(<paths.DataDir()>, "run")`, `MkdirAll(dir, 0o700)`, tulis `mcp-<16 hex acak crypto/rand>.json` mode `0o600` berisi `{"mcpServers": <ctx.MCPServers>}`, lalu tambahkan `--mcp-config <path>` di akhir args. Gagal menulis -> `slog.Warn` dan lanjut tanpa flag (Command tidak punya error). Berkas tidak dibersihkan oleh epic ini (kecil; pembersihan diserahkan ke epic hilir/daemon restart).
- Return env `nil`.

### 2.8 Koreksi terhadap tiket (hasil membaca kode nyata)

1. **Nama berkas MCP**: tiket menyebut `mcp-<sessionId>.json`, tetapi `Manager.Create` memanggil `Launcher.Command` sebelum ID sesi ada dan `SessionSpec` tidak punya ID. Pakai `mcp-<hex acak>.json`.
2. **Akses `session.Manager` dari `wire_provider.go` tidak mungkin tanpa edit kecil**: `wire_session.go` membuat `m` sebagai variabel lokal tanpa `d.Provide`, dan `session.create` tidak punya hook validasi. Diselesaikan di Wiring (bagian 5) dengan dua edit minimal yang eksplisit. Tiket 08/09 membutuhkan akses Manager juga, jadi `d.Provide("session.manager", m)` bermanfaat bagi mereka.
3. **`session new --provider`** sudah ada di `cmd_session.go` (epic 02), jadi `cmd_provider.go` hanya wajib `provider list`; `provider new` opsional (lihat 2.9).

### 2.9 CLI (`cmd/aitermctl/cmd_provider.go`)

`RegisterCommand(Command{Name:"provider", Summary:"Daftar dan uji provider: list|new", Run: runProvider})`.
- `aitermctl provider list`: memanggil `provider.list`, mencetak tabel `tabwriter` dengan header `ID  NAME  HOOKS  MCP  IDLE  AVAILABLE  PATH` (nilai `ya`/`tidak`; bila tak tersedia, kolom PATH berisi pesan error ringkas). Flag `--json` mencetak JSON mentah via `printJSON`.
- `aitermctl provider new <id> [--cwd DIR] [--workspace ID] [--project ID]`: memanggil `session.create` dengan `providerId`, mencetak ID sesi. Komentar kode wajib mencatat: `session new --provider` sudah disediakan epic 02, perintah ini hanya pintasan.
- Argumen salah -> error `penggunaan: aitermctl provider list|new ...`.
- cmd_provider.go TIDAK mengimpor `internal/provider` (hanya `internal/ipc` dan stdlib); definisikan struct respons lokal sendiri.

### 2.10 Konvensi kapabilitas

Satu tempat per provider: `claudeCapabilities`, `codexCapabilities`, `shellCapabilities` (var paket privat). Codex default semua `false` dengan komentar: nilai diverifikasi manual terhadap CLI codex terpasang, dapat diaktifkan kemudian tanpa mengubah interface.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|------|-------|------------|
| `provider-core` | Lengkapi `registry.go` (mutex, urutan stabil, panic registrasi ganda, fallback Default) dan `launch.go` (contribution, `Launcher` alur 2.4, `NewLauncher`). `provider.go` sudah final dari Foundation. | `internal/provider/provider.go`, `internal/provider/registry.go`, `internal/provider/launch.go` |
| `provider-impls` | Tiga provider yang `Register` lewat `init()`: `claude.go` (MCP config), `codex.go` (kapabilitas default false), `shell.go` (login shell `-l`). | `internal/provider/claude.go`, `internal/provider/codex.go`, `internal/provider/shell.go` |
| `provider-ipc` | `ipc.go` (`RegisterIPC`, `Info`, `Validate`), `wire_provider.go` (modul + pemasangan launcher + guard), `cmd_provider.go` (CLI). | `internal/provider/ipc.go`, `cmd/aitermd/wire_provider.go`, `cmd/aitermctl/cmd_provider.go` |

Catatan `provider-core`: `provider.go` dimiliki fitur ini sesuai tiket, tetapi isinya sudah final dari Foundation; agent core hanya memverifikasi dan boleh menambah helper privat, tanpa mengubah signature bagian 2.2.

## 4. Foundation

Ada, lihat `00-foundation.md`. Isi: `internal/provider/provider.go` final, stub `registry.go` dan `launch.go` (signature persis bagian 2.3 dan 2.4, perilaku minimal yang sudah benar tetapi tanpa kerapian akhir), dan verifikasi `go build ./...`.

Setelah Foundation, ketiga fitur tidak saling menunggu: `provider-impls` hanya butuh `Provider`, `Register`, `WriteInput`, `LookPath`, `BinaryError`; `provider-ipc` hanya butuh `Get`, `List`, `Default`, `NewLauncher`.

## 5. Wiring checklist

Dikerjakan SETELAH ketiga fitur digabung. Baca dulu `cmd/aitermd/wire_session.go` dan `internal/session/ipc.go`; bila salah satu edit di bawah ternyata sudah ada (mis. dari epic lain), jangan duplikat.

1. **Edit minimal `cmd/aitermd/wire_session.go`** (satu-satunya edit file epic lain bersama no. 2): ganti parameter `_ *Deps` menjadi `d *Deps` dan setelah `m := session.NewManager(srv.Publish)` tambahkan `d.Provide("session.manager", m)`.
2. **Edit minimal `internal/session/ipc.go`**: tambahkan variabel paket
   ```go
   // CreateGuard, bila non-nil, dipanggil oleh session.create sebelum Manager.Create.
   // Error dikembalikan apa adanya ke klien (boleh *ipc.Error). Dipasang saat wiring, sebelum Serve.
   var CreateGuard func(providerID string) error
   ```
   dan di handler `session.create`, setelah `decodeParams` dan sebelum `m.Create`: `if CreateGuard != nil { if err := CreateGuard(p.ProviderID); err != nil { return nil, err } }`.
3. **`wire_provider.go`** (dibuat provider-ipc) sudah harus berisi: `RegisterModule("provider", ...)` yang memanggil `provider.RegisterIPC(srv)`; `session.CreateGuard = provider.Validate`; dan `d.OnReady(func(){ v, ok := d.Lookup("session.manager"); ... v.(*session.Manager).SetLauncher(provider.NewLauncher()) })`. Modul "provider" terpasang sebelum "session" (alfabetis), maka pemasangan launcher WAJIB lewat `OnReady`, bukan langsung. Bila `Lookup` gagal: `d.Logger.Warn` dan lanjut (tidak panic). Verifikasi file itu sesuai; perbaiki bila menyimpang.
4. **Pastikan impor efek samping**: `cmd/aitermd` mengimpor `aiterm/internal/provider` (lewat `wire_provider.go`) sehingga `init()` claude/codex/shell menjalankan `Register`. `cmd/aitermctl` tidak perlu mengimpor paket provider.
5. `provider.list` terdaftar di `ipc.Server` (cek `daemon.info` bagian `modules` memuat `provider`).
6. `cmd_provider.go` terdaftar lewat `init()`; `aitermctl` tanpa argumen mencantumkan `provider`.
7. Tidak ada edit lain di `internal/session/`, `modules.go`, `registry.go`, `cmd_session.go`, frontend, `go.mod`.
8. **Verifikasi**: `gofmt -l internal cmd` kosong, `go build ./...`, `go vet ./internal/provider/...` (opsional), lalu `make build`. Smoke test manual (dengan `AITERM_DATA_DIR` pendek, mis. `/tmp/aitp`): jalankan `build/bin/aitermd` di background;
   - `build/bin/aitermctl provider list` menampilkan claude, codex, shell;
   - `aitermctl session new --provider shell` dan tanpa `--provider` membuat sesi dengan `providerId:"shell"`;
   - `aitermctl session new --provider nope` gagal dengan kode -32003;
   - jalankan daemon dengan `PATH` yang tidak memuat `claude` lalu `session new --provider claude` gagal dengan kode -32000 dan pesan `binary "claude" tidak ditemukan di PATH`;
   - bila `claude`/`codex` terpasang, sesi meluncurkan binary-nya;
   - hentikan daemon dan bersihkan `AITERM_DATA_DIR`.
9. Untuk epic hilir (tidak dikerjakan di sini): tiket 08 dan 09 memanggil `provider.RegisterLaunchContribution(...)` dari `wire_*.go`/paket mereka sendiri; tiket 06 memakai `provider.list`.

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- Per fitur, di worktree fitur: `gofmt -l internal/provider cmd` (harus kosong) dan `go build ./...`. Opsional: `go vet ./internal/provider/...`.
- Akhir epic (agen wiring): `make build` (frontend `npm install && npm run build`, `go build` daemon/ctl, build wails app). Build harus hijau setelah tiap fitur digabung.
- Catatan: direktori `worktree/` berisi module Go tersendiri; `./...` dari root tidak memasukkannya.

## 7. Aturan paralel

- Tiap agent HANYA menyentuh `ownedPaths` fiturnya (bagian 3). Dilarang menyentuh path fitur lain, `internal/session/`, `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go`, `cmd/aitermctl/cmd_session.go`, `cmd/aitermd/wire_session.go`, frontend, `go.mod`/`go.sum`.
- Edit ke file epic lain (bagian 5, butir 1 dan 2) HANYA dilakukan agen Wiring.
- Jangan mengubah signature di bagian 2; bila terasa keliru, tulis catatan di komentar kode dan tetap patuhi kontrak.
- Hanya stdlib (ditambah `aiterm/internal/ipc`, `aiterm/internal/paths`, `aiterm/internal/session`). Dependency baru dilarang.
- Import: `provider` boleh mengimpor `session`, `ipc`, `paths`; `session` TIDAK boleh mengimpor `provider` (hindari siklus).
- Registrasi lewat `init()`; tidak ada edit file registrasi bersama.
- Tanpa unit test; jangan menambah berkas `_test.go`.
