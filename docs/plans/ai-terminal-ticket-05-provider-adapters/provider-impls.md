# Plan: Adapter claude, codex, shell

## Metadata

- Tiket: `ai-terminal-ticket-05-provider-adapters` (`docs/tickets/ai-terminal-ticket-05-provider-adapters.md`)
- Fitur: `provider-impls` (Adapter claude, codex, shell)
- ownedPaths: `internal/provider/claude.go`, `internal/provider/codex.go`, `internal/provider/shell.go`
- Bergantung pada Foundation: ya (`00-foundation.md`; `provider.go`, `registry.go`, `launch.go` sudah ada dan `go build ./...` hijau sebelum fitur ini dimulai)

## Tujuan

Menyediakan tiga implementasi `Provider` (`claudeProvider`, `codexProvider`, `shellProvider`) yang masing-masing mendaftar sendiri ke registri lewat `init()`. Hanya stdlib plus `aiterm/internal/paths`. Tanpa unit test, tanpa berkas `_test.go`.

## Path yang dimiliki

HANYA tiga berkas ini yang boleh dibuat/diubah:

- `internal/provider/claude.go`
- `internal/provider/codex.go`
- `internal/provider/shell.go`

Dilarang menyentuh `provider.go`, `registry.go`, `launch.go`, `ipc.go`, `cmd/aitermd/*`, `cmd/aitermctl/*`, `internal/session/`, `go.mod`/`go.sum`, frontend, atau file wiring apa pun. Jangan memodifikasi `overview.md`.

## Kontrak

Dipakai (disediakan Foundation di `provider.go`, `registry.go`, `launch.go`; JANGAN didefinisikan ulang di sini):

- `type Provider interface { ID() string; Name() string; Capabilities() Capabilities; Command(ctx *LaunchContext) (cmd string, args []string, env []string); SendInput(w io.Writer, text string) error; Available() (path string, err error) }`
- `type Capabilities struct { Hooks, MCP, IdleEvent bool }`
- `type LaunchContext struct { Session session.SessionSpec; Provider Provider; Env map[string]string; Args []string; MCPServers map[string]any }`
- Konstanta `IDClaude`, `IDCodex`, `IDShell`.
- `func Register(p Provider)` (panic bila id ganda).
- `func WriteInput(w io.Writer, text string) error` (sudah menulis bracketed paste + `\r` dalam SATU Write; jangan menulis `\r` kedua).
- `func LookPath(name string) (string, error)`.
- `func BinaryError(providerID, bin string) error`.
- Dari `aiterm/internal/paths`: `paths.DataDir()` (membuat dir 0700). Cek signature persis (nilai kembalian, apakah ada error) dengan membaca `internal/paths` sebelum memakainya.

Disediakan (privat, nama tipe wajib persis agar tidak bentrok dengan fitur lain):

- `claudeProvider`, `codexProvider`, `shellProvider` (struct kosong).
- Var paket privat `claudeCapabilities`, `codexCapabilities`, `shellCapabilities`.
- Registrasi lewat `init()`: `Register(claudeProvider{})`, `Register(codexProvider{})`, `Register(shellProvider{})`. Urutan `List()` ditentukan registry, bukan urutan `init()`.

Perilaku (rujuk overview 2.7 dan 2.10; tidak diulang penuh di sini). Catatan: nama berkas MCP memakai hex acak, bukan sessionId (overview 2.8 butir 1). Env hasil `Command` provider cukup `nil`; `Launcher` yang menggabungkan `ctx.Env`.

## Langkah

### 1. Baca konteks (tanpa mengubah)

Baca `internal/provider/provider.go`, `registry.go`, `launch.go` hasil Foundation untuk memastikan signature, dan `internal/paths` untuk signature `DataDir`. Bila signature berbeda dari kontrak, ikuti kode nyata dan tulis catatan di komentar.

### 2. `internal/provider/shell.go`

- `package provider`; import `io`, `os`.
- `var shellCapabilities = Capabilities{}` dengan komentar: idle dideteksi lewat output-diam, semua kapabilitas false.
- `type shellProvider struct{}`.
- `func init() { Register(shellProvider{}) }`.
- `func shellPath() string`: `os.Getenv("SHELL")`, fallback `"/bin/zsh"` bila kosong.
- Method:
  - `ID() string` -> `IDShell`
  - `Name() string` -> `"Shell"`
  - `Capabilities() Capabilities` -> `shellCapabilities`
  - `Command(ctx *LaunchContext) (string, []string, []string)`: `args := append([]string{"-l"}, ctx.Args...)`; return `shellPath(), args, nil`. MCP diabaikan.
  - `SendInput(w io.Writer, text string) error` -> `WriteInput(w, text)`
  - `Available() (string, error)`: `p, err := LookPath(shellPath())`; bila err -> `return "", BinaryError(IDShell, shellPath())`; sukses `return p, nil`.

### 3. `internal/provider/codex.go`

- Import `io`.
- `var codexCapabilities = Capabilities{Hooks: false, MCP: false, IdleEvent: false}` dengan komentar bahasa Indonesia: nilai default ini diverifikasi manual terhadap CLI codex terpasang; jika terbukti mendukung hooks/MCP/idle event, cukup ubah var ini tanpa mengubah interface (sementara itu idle dideteksi lewat output-diam).
- `type codexProvider struct{}`; `func init() { Register(codexProvider{}) }`.
- Method:
  - `ID()` -> `IDCodex`; `Name()` -> `"Codex"`; `Capabilities()` -> `codexCapabilities`
  - `Command(ctx)`: `return "codex", append([]string(nil), ctx.Args...), nil` (MCP diabaikan)
  - `SendInput` -> `WriteInput(w, text)`
  - `Available()`: `p, err := LookPath("codex")`; err -> `"", BinaryError(IDCodex, "codex")`; sukses `p, nil`.

### 4. `internal/provider/claude.go`

- Import: `crypto/rand`, `encoding/hex`, `encoding/json`, `io`, `log/slog`, `os`, `path/filepath`, `aiterm/internal/paths`.
- `var claudeCapabilities = Capabilities{Hooks: true, MCP: true, IdleEvent: true}`.
- `type claudeProvider struct{}`; `func init() { Register(claudeProvider{}) }`.
- `ID()` -> `IDClaude`; `Name()` -> `"Claude Code"`; `Capabilities()` -> `claudeCapabilities`; `SendInput` -> `WriteInput(w, text)`; `Available()`: `LookPath("claude")`, err -> `"", BinaryError(IDClaude, "claude")`.
- Helper privat `func writeMCPConfig(servers map[string]any) (string, error)`:
  1. `dir := filepath.Join(<paths.DataDir()>, "run")`; `os.MkdirAll(dir, 0o700)`.
  2. 16 hex acak: `b := make([]byte, 8)` lewat `rand.Read` (8 byte = 16 karakter hex), `hex.EncodeToString`.
  3. `path := filepath.Join(dir, "mcp-"+hexstr+".json")`.
  4. `data, err := json.Marshal(map[string]any{"mcpServers": servers})`.
  5. `os.WriteFile(path, data, 0o600)`; kembalikan `path, nil`. Setiap error dibungkus `fmt.Errorf` konteks Indonesia (tambah import `fmt`). Karena `WriteFile` tunduk pada umask, setelah menulis panggil juga `os.Chmod(path, 0o600)` agar mode pasti 0600.
- `Command(ctx *LaunchContext) (string, []string, []string)`:
  1. `args := append([]string(nil), ctx.Args...)`.
  2. Bila `len(ctx.MCPServers) > 0`: `path, err := writeMCPConfig(ctx.MCPServers)`; bila err -> `slog.Warn("provider claude: gagal menulis konfigurasi MCP; lanjut tanpa --mcp-config", "err", err)`; bila sukses `args = append(args, "--mcp-config", path)`.
  3. `return "claude", args, nil`.
- Berkas MCP tidak dibersihkan oleh epic ini (catat di komentar).

### 5. Verifikasi

1. `gofmt -l internal/provider cmd` kosong.
2. `go build ./...` hijau.
3. Opsional: `go vet ./internal/provider/...`.
4. Pastikan tidak ada berkas `_test.go` dan tidak ada file lain berubah.

## Perubahan

Dibuat:
- `internal/provider/claude.go`
- `internal/provider/codex.go`
- `internal/provider/shell.go`

Tidak ada file yang diubah.

## Wiring

Fitur ini tidak menyambungkan apa pun sendiri. Yang diekspos:

- Efek samping `init()`: begitu paket `aiterm/internal/provider` diimpor (oleh `cmd/aitermd/wire_provider.go`, milik fitur `provider-ipc`, dikerjakan di langkah wiring), `Register` memasukkan `claude`, `codex`, `shell` ke registri sehingga `List()`/`Get()`/`Default()` dan `provider.list`/`Launcher` dapat memakainya.
- Tidak ada edit ke `wire_session.go`, `internal/session/ipc.go`, atau file lain; itu tugas agen Wiring (overview bagian 5).

## Selesai bila

- [ ] Tiga berkas ada dan hanya itu yang berubah.
- [ ] `gofmt -l internal/provider cmd` kosong dan `go build ./...` sukses (guard; tanpa unit test).
- [ ] Kapabilitas: claude semua true; codex semua false (dengan komentar verifikasi manual); shell semua false.
- [ ] `claude.Command` menambah `--mcp-config <path>` hanya bila `MCPServers` tidak kosong; berkas `<DataDir>/run/mcp-<16 hex>.json` mode 0600 berisi `{"mcpServers": ...}`; kegagalan tulis hanya `slog.Warn`, tidak panic.
- [ ] `shell.Command` = `$SHELL` (fallback `/bin/zsh`) dengan args `-l` + `ctx.Args`.
- [ ] `SendInput` ketiganya memanggil `WriteInput` (tanpa `\r` tambahan).
- [ ] `Available()` mengembalikan `BinaryError` bila binary tidak ada, tanpa panic.
- [ ] Setelah impor paket, `List()` berisi tepat claude, codex, shell dalam urutan itu.
