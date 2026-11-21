# Foundation: Epic 05 Provider Adapters

Dikerjakan SATU agent sebelum fitur paralel. Baca `overview.md` (kontrak bagian 2) dan tiket dulu. Tujuan: paket `internal/provider` yang kompilasi dengan tipe final dan stub, sehingga `provider-core`, `provider-impls`, `provider-ipc` bisa jalan paralel.

Hanya sentuh: `internal/provider/provider.go`, `internal/provider/registry.go`, `internal/provider/launch.go`. Hanya stdlib plus `aiterm/internal/session`. Tanpa unit test.

## Langkah 1: provider.go (final)

Buat `internal/provider/provider.go`, `package provider`, sesuai overview 2.2 persis:

1. Konstanta `IDClaude="claude"`, `IDCodex="codex"`, `IDShell="shell"`.
2. `type Capabilities struct` dengan tag json `hooks`, `mcp`, `idleEvent`.
3. `type Provider interface` dengan enam method (ID, Name, Capabilities, Command, SendInput, Available).
4. `func BracketedPaste(text string) []byte`: `"\x1b[200~" + text + "\x1b[201~" + "\r"`.
5. `func WriteInput(w io.Writer, text string) error`: `_, err := w.Write(BracketedPaste(text))` (satu kali Write).
6. `func LookPath(name string) (string, error)`: `exec.LookPath`; bila gagal kembalikan `fmt.Errorf("binary %q tidak ditemukan di PATH", name)`.
7. `func BinaryError(providerID, bin string) error`: `fmt.Errorf("provider %q: binary %q tidak ditemukan di PATH; pasang atau tambahkan ke PATH", providerID, bin)`.

Komentar doc bahasa Indonesia pada tiap simbol publik. `Provider.Command` memakai `*LaunchContext` (didefinisikan di launch.go).

## Langkah 2: registry.go (stub fungsional)

Buat `internal/provider/registry.go`:

- `var (regMu sync.RWMutex; registry = map[string]Provider{})`.
- `Register(p Provider)`: panic bila `p == nil`, id kosong, atau id sudah ada (`panic("provider: id sudah terdaftar: " + id)`).
- `Get(id string) (Provider, bool)`.
- `List() []Provider`: urutan tetap claude, codex, shell, lalu id lain urut alfabet; hanya yang terdaftar.
- `Default() Provider`: `registry["shell"]` bila ada, jika tidak `fallbackProvider{}`.
- `type fallbackProvider struct{}`: ID `shell`, Name `Shell`, Capabilities nol, `Command` -> `$SHELL` (fallback `/bin/zsh`), args `-l` + `ctx.Args`, env nil; `SendInput` -> `WriteInput`; `Available` -> `LookPath` dari shell tersebut.

(provider-core akan merapikan dan melengkapi; stub ini sudah harus benar secara fungsional.)

## Langkah 3: launch.go (stub fungsional)

Buat `internal/provider/launch.go` dengan import `aiterm/internal/session`:

- `LaunchContext`, `LaunchContribution`, `RegisterLaunchContribution` (slice + `sync.RWMutex`; nil diabaikan).
- `Launcher struct{}`, `var _ session.Launcher = Launcher{}`, `NewLauncher() session.Launcher`.
- `Launcher.Command` mengikuti alur overview 2.4 (lookup provider atau `Default()`, ctx dengan map tidak-nil, jalankan contribution, `p.Command(ctx)`, gabung env: `os.Environ()`, `TERM=xterm-256color`, `COLORTERM=truecolor`, `s.Env`, `ctx.Env` terurut, extra provider).
- Helper privat `envList(map[string]string) []string`.

## Langkah 4: Verifikasi

1. `gofmt -l internal/provider` kosong.
2. `go build ./...` hijau (paket hanya berisi tiga file ini; registri kosong jatuh ke fallback).
3. Pastikan tidak ada impor dari `internal/provider` oleh `internal/session` (tanpa siklus).

Selesai: laporkan bahwa signature bagian 2.2, 2.3, 2.4 overview sudah ada dan build hijau.
