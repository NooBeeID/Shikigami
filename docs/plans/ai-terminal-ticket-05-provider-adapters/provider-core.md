# Plan: Interface, kapabilitas, registri

## Metadata

- Tiket: `ai-terminal-ticket-05-provider-adapters` (`docs/tickets/ai-terminal-ticket-05-provider-adapters.md`)
- Fitur: `provider-core`
- ownedPaths: `internal/provider/provider.go`, `internal/provider/registry.go`, `internal/provider/launch.go`
- Depends on foundation: ya (`00-foundation.md`). Foundation sudah menulis ketiga file ini sebagai tipe final (provider.go) dan stub fungsional (registry.go, launch.go). Fitur ini merapikan dan melengkapi stub, bukan menulis dari nol. Bila saat dikerjakan ternyata file belum ada (Foundation belum merge), buat file sesuai langkah di bawah dengan signature yang sama.

Kontrak lengkap ada di `overview.md` bagian 2.2, 2.3, 2.4, 2.5. Plan ini merujuknya dan tidak mengulang semuanya.

## Tujuan

Menghasilkan inti paket `internal/provider` yang final: interface dan helper (`provider.go`), registri provider yang aman konkuren dengan urutan `List()` stabil, panic registrasi ganda, dan `Default()` dengan `fallbackProvider` (`registry.go`), serta registri launch contribution dan `Launcher` yang memenuhi `session.Launcher` dengan alur 2.4 overview (`launch.go`). Paket harus kompilasi dan berperilaku benar walau registri kosong (belum ada claude/codex/shell), sehingga fitur `provider-impls` dan `provider-ipc` tidak terblokir.

## Path yang dimiliki

HANYA path berikut yang boleh disentuh:

- `internal/provider/provider.go`
- `internal/provider/registry.go`
- `internal/provider/launch.go`

Dilarang: `claude.go`, `codex.go`, `shell.go`, `ipc.go` (fitur lain), `cmd/aitermd/wire_*.go`, `cmd/aitermctl/*`, `internal/session/`, `internal/ipc/`, `go.mod`/`go.sum`, `overview.md`, frontend, dan berkas `_test.go` (tanpa unit test). Jangan menyambungkan launcher ke `session.Manager` (itu urusan Wiring).

## Kontrak

Disediakan (signature persis, JANGAN diubah):

- `provider.go` (overview 2.2): konstanta `IDClaude`, `IDCodex`, `IDShell`; `type Capabilities struct{Hooks, MCP, IdleEvent bool}` dengan tag json `hooks`, `mcp`, `idleEvent`; `type Provider interface{ID, Name, Capabilities, Command(*LaunchContext), SendInput(io.Writer, string) error, Available}`; `BracketedPaste(text string) []byte`; `WriteInput(w io.Writer, text string) error`; `LookPath(name string) (string, error)`; `BinaryError(providerID, bin string) error`.
- `registry.go` (overview 2.3): `Register(p Provider)`, `Get(id string) (Provider, bool)`, `List() []Provider`, `Default() Provider`; tipe privat `fallbackProvider`.
- `launch.go` (overview 2.4, 2.5): `LaunchContext{Session session.SessionSpec; Provider Provider; Env map[string]string; Args []string; MCPServers map[string]any}`, `type LaunchContribution func(*LaunchContext)`, `RegisterLaunchContribution(fn LaunchContribution)`, `type Launcher struct{}`, `var _ session.Launcher = Launcher{}`, `(Launcher) Command(s session.SessionSpec) (string, []string, []string)`, `NewLauncher() session.Launcher`, helper privat `envList(m map[string]string) []string`.

Dipakai dari kode yang sudah ada: `aiterm/internal/session` (`SessionSpec{WorkspaceID, ProjectID, Cwd, ProviderID string; Cols, Rows uint16; Env []string}`, interface `Launcher`). Perhatikan `SessionSpec.Env` bertipe `[]string` (bentuk `KEY=VALUE`), sedangkan `LaunchContext.Env` bertipe `map[string]string`. Import hanya stdlib plus `aiterm/internal/session`. `session` tidak boleh mengimpor `provider` (hindari siklus).

Dipakai oleh fitur lain (jangan ubah perilakunya): `provider-impls` memakai `Provider`, `Register`, `WriteInput`, `LookPath`, `BinaryError`, `LaunchContext`; `provider-ipc` memakai `Get`, `List`, `Default`, `NewLauncher`.

## Langkah

### 1. Verifikasi dan rapikan `provider.go`

1. Buka `internal/provider/provider.go`. Pastikan `package provider`, import `fmt`, `io`, `os/exec`, dan isinya sama dengan overview 2.2 (lihat `00-foundation.md` Langkah 1). Periksa satu per satu:
   - `BracketedPaste` mengembalikan `[]byte("\x1b[200~" + text + "\x1b[201~" + "\r")`; Enter SUDAH termasuk.
   - `WriteInput` memanggil `w.Write(BracketedPaste(text))` tepat SATU kali, mengembalikan error-nya saja (jangan menulis `\r` tambahan).
   - `LookPath` membungkus `exec.LookPath`; saat gagal mengembalikan `fmt.Errorf("binary %q tidak ditemukan di PATH", name)`; saat sukses mengembalikan path.
   - `BinaryError` mengembalikan `fmt.Errorf("provider %q: binary %q tidak ditemukan di PATH; pasang atau tambahkan ke PATH", providerID, bin)`.
   - Setiap simbol publik punya komentar doc bahasa Indonesia.
2. Bila ada penyimpangan dari signature, perbaiki agar sama dengan kontrak. Jangan menambah method ke interface `Provider` dan jangan mengubah field `Capabilities`.
3. Penambahan helper privat bersifat opsional dan hanya bila benar-benar dipakai oleh `registry.go`/`launch.go`. Jangan menambah simbol publik baru.

### 2. Lengkapi `registry.go`

Import: `os`, `sort`, `sync`.

1. State paket:
   ```go
   var (
       regMu    sync.RWMutex
       registry = map[string]Provider{}
   )
   // urutan tetap untuk List(); id lain menyusul urut alfabet.
   var listOrder = []string{IDClaude, IDCodex, IDShell}
   ```
2. `func Register(p Provider)`: panic bila `p == nil`; ambil `id := p.ID()`; panic `"provider: id kosong"` bila kosong; di bawah `regMu.Lock()` panic `"provider: id sudah terdaftar: " + id` bila sudah ada; simpan. Gunakan `defer regMu.Unlock()` (panic di dalam lock aman karena defer). Catatan komentar: dipanggil dari `init()` file impl.
3. `func Get(id string) (Provider, bool)`: `RLock`, lookup, kembalikan.
4. `func List() []Provider`: `RLock`; bangun slice hasil: pertama iterasi `listOrder` dan tambahkan yang terdaftar; lalu kumpulkan id lain (tidak ada di `listOrder`), `sort.Strings`, tambahkan. Selalu kembalikan slice non-nil (`make([]Provider, 0, len(registry))`). Urutan TIDAK boleh bergantung pada urutan `init()` atau iterasi map.
5. `func Default() Provider`: `RLock`; bila `registry[IDShell]` ada kembalikan; jika tidak kembalikan `fallbackProvider{}`. Tidak panic.
6. `type fallbackProvider struct{}` (privat) dengan komentar bahwa ini dipakai hanya bila provider `shell` belum terdaftar (mis. paket dibuild tanpa `shell.go`):
   - `ID()` -> `IDShell`; `Name()` -> `"Shell"`; `Capabilities()` -> `Capabilities{}`.
   - helper privat `fallbackShell() string`: `os.Getenv("SHELL")`, bila kosong `"/bin/zsh"`.
   - `Command(ctx *LaunchContext)` -> `fallbackShell()`, args `append([]string{"-l"}, ctx.Args...)` (tangani `ctx == nil` dengan aman: bila nil, args hanya `-l`), env `nil`.
   - `SendInput(w io.Writer, text string) error` -> `WriteInput(w, text)` (butuh import `io`).
   - `Available()` -> `LookPath(fallbackShell())`; bila error, bungkus dengan `BinaryError(IDShell, shell)`; path dikembalikan bila sukses.
   - Jangan beri nama tipe lain dan jangan beri nama sama dengan `shellProvider` (milik provider-impls).
7. Jangan membuat fungsi publik tambahan (mis. `Reset`/`Unregister`). Tanpa unit test.

### 3. Lengkapi `launch.go`

Import: `os`, `sort`, `sync`, `aiterm/internal/session`.

1. Tipe `LaunchContext` dan `LaunchContribution` persis overview 2.4, dengan komentar doc: `Env` dan `MCPServers` tidak pernah nil saat diberikan ke contribution/provider; `Args` boleh nil.
2. State contribution:
   ```go
   var (
       contribMu     sync.RWMutex
       contributions []LaunchContribution
   )
   ```
3. `func RegisterLaunchContribution(fn LaunchContribution)`: bila `fn == nil` return; `contribMu.Lock()`, `append`, `Unlock`. Komentar: aman konkuren dan aman dipanggil dari `init()` paket lain; dijalankan berurutan sesuai urutan registrasi sebelum `Provider.Command`.
4. Fungsi internal `applyContributions(ctx *LaunchContext)`: `RLock`, salin slice (`append([]LaunchContribution(nil), contributions...)`), `RUnlock`, lalu panggil tiap fungsi di luar lock (supaya contribution boleh memanggil `RegisterLaunchContribution` tanpa deadlock). Pertahankan urutan registrasi.
5. `func envList(m map[string]string) []string`: ambil kunci, `sort.Strings`, hasilkan `KEY=VALUE` (`k + "=" + m[k]`); map kosong/nil -> slice kosong (`nil` boleh).
6. `type Launcher struct{}`; tepat satu asersi compile-time `var _ session.Launcher = Launcher{}`.
7. `func (Launcher) Command(s session.SessionSpec) (string, []string, []string)` alur persis overview 2.4:
   1. `p, ok := Get(s.ProviderID)`; bila `!ok` (id kosong atau tak dikenal) `p = Default()`. Tidak boleh panic.
   2. `ctx := &LaunchContext{Session: s, Provider: p, Env: map[string]string{}, MCPServers: map[string]any{}}` (`Args` nil).
   3. `applyContributions(ctx)`.
   4. `cmd, args, extra := p.Command(ctx)`.
   5. Gabung env dengan urutan (entri belakangan menang karena `os/exec` membuang duplikat dan mempertahankan yang terakhir): `os.Environ()`, `"TERM=xterm-256color"`, `"COLORTERM=truecolor"`, `s.Env`, `envList(ctx.Env)`, `extra`. Prealokasi kapasitas.
   6. `return cmd, args, env`.
   - Edge: contribution bisa menimpa `ctx.Env`/`ctx.MCPServers` menjadi nil; sebelum `p.Command` pastikan keduanya di-normalisasi kembali ke map non-nil (`if ctx.Env == nil { ctx.Env = map[string]string{} }`, sama untuk `MCPServers`) sehingga provider tidak pernah menerima nil. Lakukan normalisasi ini di `applyContributions` setelah loop.
8. `func NewLauncher() session.Launcher { return Launcher{} }` dengan komentar bahwa dipasang oleh `wire_provider.go` lewat `Manager.SetLauncher` (saat wiring, bukan di sini).
9. Catatan komentar kode: pengecekan binary TIDAK dilakukan di `Command` (tidak punya error); dilakukan oleh `Validate` di fitur provider-ipc.

### 4. Verifikasi build

Jalankan dari root repo:

1. `gofmt -l internal/provider cmd` harus kosong (jalankan `gofmt -w` pada tiga file milik sendiri bila perlu).
2. `go build ./...` harus hijau. Bila `provider-impls`/`provider-ipc` sudah ada di worktree dan gagal build karena sebab di luar tiga file ini, jangan memperbaiki file mereka; pastikan penyebabnya bukan signature di file milik sendiri.
3. Opsional: `go vet ./internal/provider/...`.
4. Periksa tidak ada impor `aiterm/internal/provider` di bawah `internal/session/` (`grep -rn "internal/provider" internal/session` kosong).
5. Periksa tidak ada berkas `_test.go` baru.

## Perubahan

- `internal/provider/provider.go`: diverifikasi terhadap kontrak; hanya dikoreksi bila menyimpang (tanpa perubahan signature).
- `internal/provider/registry.go`: dilengkapi/dirapikan (RWMutex, `listOrder` stabil, panic registrasi ganda/nil/kosong, `Default` dengan `fallbackProvider`).
- `internal/provider/launch.go`: dilengkapi/dirapikan (`RegisterLaunchContribution`, `applyContributions` dengan salinan slice dan normalisasi map, `Launcher.Command` alur 2.4, `NewLauncher`, `envList`, asersi compile-time).

Tidak ada file lain yang dibuat atau diubah.

## Wiring

Yang diekspos untuk langkah wiring dan fitur lain (JANGAN disambungkan di fitur ini):

- `provider.NewLauncher()` mengembalikan `session.Launcher`; dipasang nanti oleh `cmd/aitermd/wire_provider.go` lewat `d.OnReady` -> `(*session.Manager).SetLauncher(...)` (overview bagian 5, butir 3).
- `provider.Register`, `provider.WriteInput`, `provider.LookPath`, `provider.BinaryError`, `provider.LaunchContext` dipakai `provider-impls` dari `init()` file claude/codex/shell.
- `provider.Get`, `provider.List`, `provider.Default` dipakai `provider-ipc` (`provider.list`, `Validate`).
- `provider.RegisterLaunchContribution` dipakai epic hilir (tiket 08 dan 09) dari paket/`wire_*.go` mereka sendiri.

Tidak ada edit ke `wire_session.go`, `internal/session/ipc.go`, atau file epic lain di fitur ini.

## Selesai bila

- Signature `provider.go`, `registry.go`, `launch.go` sama dengan overview 2.2, 2.3, 2.4, 2.5; ada `var _ session.Launcher = Launcher{}`.
- `List()` selalu non-nil dan urut claude, codex, shell, lalu id lain alfabet, tidak bergantung urutan `init()`.
- `Register` panic untuk nil, id kosong, dan id ganda; akses registri dan contribution dilindungi mutex; contribution dipanggil di luar lock, berurutan, sebelum `Provider.Command`.
- `Launcher.Command` dengan `ProviderID` kosong/tak dikenal jatuh ke `Default()` tanpa panic; `Env` dan `MCPServers` selalu non-nil bagi provider; urutan gabung env sesuai overview 2.4.
- Guard: `gofmt -l internal/provider cmd` kosong dan `go build ./...` sukses (opsional `go vet ./internal/provider/...`). Tanpa unit test, tanpa `_test.go`, tanpa dependency baru, tidak ada file di luar ownedPaths yang berubah.
