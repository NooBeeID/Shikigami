---
id: ai-terminal-ticket-05-provider-adapters
title: Provider Adapters
spec: docs/superpowers/specs/2026-11-21-ai-terminal-design.md
status: ready
wave: 3
depends_on: [ai-terminal-ticket-02-session-pty]
---
# Provider Adapters

## Tujuan

Epic ini menyediakan adapter tipis per provider (claude, codex, generic shell) dengan model kapabilitas (hooks, MCP, idle event), sesuai spec bagian "Provider" dan "Provider memakai model kapabilitas" (9b). Epic ini juga menyediakan titik ekstensi `RegisterLaunchContribution` agar epic status dan mention nanti bisa menambah env, args, atau konfigurasi MCP/hooks tanpa mengedit file provider. Hasil akhirnya: sesi bisa dibuat dengan provider tertentu (`session.create` dengan `providerId`), dan daemon bisa menjawab `provider.list`.

Tiket ini harus bisa jalan dan lolos `make build` sendiri setelah tiket 01 dan 02 selesai. Tanpa epic lain (status, mention, UI), provider tetap berfungsi: sesi dibuat dengan `providerId` yang valid dan perintahnya diluncurkan lewat PTY.

## Batasan

In scope:
- Paket `internal/provider/`: interface `Provider`, tipe `Capabilities`, `LaunchContext`, registri provider, registri launch contribution.
- Tiga implementasi: `claude`, `codex`, `shell`.
- Implementasi `session.Launcher` (dari tiket 02) yang mendelegasikan ke provider, dan pemasangannya ke `session.Manager`.
- Method IPC `provider.list`; penerimaan `providerId` pada `session.create`.
- Perintah CLI debug `aitermctl provider list` dan `aitermctl session new --provider`.
- Pengiriman input ke PTY dengan bracketed paste + Enter (fungsi `SendInput` milik provider).
- Pengecekan keberadaan binary dan error yang jelas bila tidak ada.

Out of scope:
- Logika deteksi idle, status, dan hook handler (milik tiket 08 session-status).
- Server MCP lokal, antrean mention, routing pesan (milik tiket 09 session-mention). Epic ini hanya menyediakan field `MCPServers` dan titik kontribusi.
- Integrasi Grimoire / knowledge dan penataan project (fase 4 dan 5, di luar cakupan tiket).
- UI pemilih provider di frontend (milik tiket 06 ui-shell).
- Windows.
- Unit test (guard hanya `make build`).
- Mengedit file milik epic lain, termasuk `internal/session/`, `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go`. Bila perlu perubahan di file tersebut, hanya lewat kontrak (lihat bagian Wiring).

## Kontrak

### Yang dipakai dari epic lain

Dari tiket 01 (app-base):
- `internal/ipc`: `srv.Register(method string, func(ctx context.Context, params json.RawMessage) (any, error))`, kode error `-32000 ERR_BINARY`, `-32001 ERR_CONFLICT`, `-32002 ERR_NOT_FOUND`, `-32003 ERR_INVALID`, dan `client.Call(method, params, &out)`.
- `cmd/aitermd/modules.go`: `RegisterModule("<modul>", func(srv *ipc.Server, d *Deps){...})`.
- `cmd/aitermctl/registry.go`: `RegisterCommand(...)`.

Dari tiket 02 (session-pty):
- `internal/session/launcher.go`: `type Launcher interface { Command(s SessionSpec) (cmd string, args []string, env []string) }` dengan default login shell.
- `session.Manager` dan cara memasang Launcher (agent implementasi harus membaca `internal/session/` untuk nama persisnya, mis. `Manager.SetLauncher(l Launcher)` atau option konstruktor). Bila `session.Manager` hanya menyediakan satu jalur pemasangan, pakai jalur itu; jangan mengedit paket session.
- `SessionSpec` / `Session{id, workspaceId, projectId, cwd, providerId, status, createdAt}`. Kontrak spec menyatakan `session.create` menyimpan `providerId`; epic ini memakai field tersebut.
- Kemampuan menulis ke PTY sesi (`session.write`) untuk mengirim input.

Catatan: Launcher hanya mengembalikan `cmd, args, env`, tanpa error. Pengecekan binary yang gagal harus dilakukan sebelum sesi dibuat (lihat `provider-ipc`: validasi di handler `session.create`), bukan di dalam `Command`.

### Yang disediakan epic ini

Paket `internal/provider` (Go, module `aiterm`):

```go
// provider.go
type Capabilities struct {
    Hooks     bool `json:"hooks"`
    MCP       bool `json:"mcp"`
    IdleEvent bool `json:"idleEvent"`
}

type Provider interface {
    ID() string                    // "claude" | "codex" | "shell"
    Name() string                  // nama tampilan
    Capabilities() Capabilities
    Command(ctx *LaunchContext) (cmd string, args []string, env []string)
    SendInput(w io.Writer, text string) error // bracketed paste + Enter
    Available() (path string, err error)      // cek binary di PATH
}

// launch.go
type LaunchContext struct {
    Session    session.SessionSpec  // atau tipe Session dari paket session
    Provider   Provider
    Env        map[string]string
    Args       []string
    MCPServers map[string]any       // nama -> konfigurasi server MCP
}

type LaunchContribution func(*LaunchContext)
func RegisterLaunchContribution(fn LaunchContribution)
```

```go
// registry.go
func Register(p Provider)
func Get(id string) (Provider, bool)
func List() []Provider            // urut stabil: claude, codex, shell
func Default() Provider           // shell
```

Implementasi `session.Launcher` (di `launch.go`):

```go
type Launcher struct{}  // memenuhi session.Launcher
func (Launcher) Command(s session.SessionSpec) (string, []string, []string)
```

Alur `Launcher.Command`: cari provider dari `s.ProviderID` (kosong atau tidak dikenal jatuh ke `shell`), bangun `LaunchContext`, panggil semua contribution terdaftar berurutan sesuai urutan registrasi, lalu panggil `provider.Command(ctx)` dan gabungkan hasilnya (env dari `ctx.Env` diubah menjadi `KEY=VALUE`, ditambahkan di atas `os.Environ()`).

Contribution dijalankan sebelum `Command` milik provider; provider membaca `ctx.Args`, `ctx.Env`, `ctx.MCPServers` yang sudah diperkaya, lalu menerjemahkannya ke bentuk spesifik provider (mis. claude: `--mcp-config <file>`; shell: mengabaikan MCP).

Pesan IPC:
- `provider.list` params `{}` hasil:
  ```json
  [{"id":"claude","name":"Claude Code","capabilities":{"hooks":true,"mcp":true,"idleEvent":true},"available":true,"path":"/usr/local/bin/claude"}, ...]
  ```
  Field `path` dikosongkan dan `available:false` bila binary tidak ada; bisa ditambah field `error` berisi pesan.
- `session.create` (milik tiket 02) menerima tambahan `providerId` (opsional, default `shell`). Bila `providerId` tidak dikenal: error `-32003 ERR_INVALID`. Bila binary provider tidak ada: error `-32000 ERR_BINARY` dengan pesan jelas (mis. `binary "claude" tidak ditemukan di PATH`).

Catatan penamaan: kontrak bersama menyebut `ERR_BINARY` untuk file biner; epic ini memakainya juga untuk "binary provider tidak ada" karena kodenya sama-sama kode error bawaan IPC dan tidak ada kode lain yang cocok. Pesan error membedakannya.

CLI (`cmd/aitermctl/cmd_provider.go`):
- `aitermctl provider list` mencetak tabel id, nama, kapabilitas, tersedia.
- `aitermctl session new --provider <id>`: lihat catatan di fitur `provider-ipc` tentang kepemilikan `cmd_session.go`.

## Fitur

Ketiga fitur ini bisa dikerjakan paralel dengan batas: `provider-impls` dan `provider-ipc` bergantung pada tipe dan interface di `provider-core`. Bila dikerjakan paralel, sepakati dulu signature di bagian Kontrak di atas, lalu `provider-core` harus selesai lebih dulu atau minimal file `provider.go` terbit lebih dulu agar build lolos.

### provider-core: Interface, kapabilitas, registri

Deskripsi:
Mendefinisikan `Provider`, `Capabilities`, `LaunchContext`, registri provider, dan registri launch contribution, serta `Launcher` yang memenuhi `session.Launcher`.

Rincian:
- `provider.go`: tipe `Capabilities`, interface `Provider` seperti di Kontrak, helper `BracketedPaste(text string) []byte` yang mengembalikan `ESC[200~ + text + ESC[201~` diikuti `\r`, dan helper `LookPath(name string) (string, error)` yang membungkus `exec.LookPath` dengan error berbahasa jelas.
- `registry.go`: peta provider terproteksi mutex; `Register`, `Get`, `List` (urut stabil), `Default`. Registrasi ganda untuk id sama menimpa atau panic (pilih panic agar bug cepat ketahuan).
- `launch.go`: `LaunchContext`, `RegisterLaunchContribution`, penyimpan slice contribution terproteksi mutex, fungsi internal `applyContributions(*LaunchContext)`, dan `Launcher` yang mengimplementasi `session.Launcher`. Contribution aman dipanggil dari init() epic lain (daftar ditambah saat init, dibaca saat launch).
- Fungsi `NewLauncher() session.Launcher` untuk dipasang oleh `wire_provider.go`.

Acceptance criteria:
- [ ] `go build ./...` lolos dengan paket `internal/provider` hanya berisi file core ini (tanpa impl, registri kosong dengan fallback aman).
- [ ] `Provider`, `Capabilities`, `LaunchContext`, `LaunchContribution`, `RegisterLaunchContribution`, `Register`, `Get`, `List`, `Default` ada dengan signature di Kontrak.
- [ ] `Launcher` memenuhi `session.Launcher` (ada asersi compile-time `var _ session.Launcher = Launcher{}`).
- [ ] Contribution dipanggil berurutan sesuai registrasi, sebelum `Provider.Command`.
- [ ] `providerId` kosong atau tak dikenal pada `Launcher.Command` jatuh ke provider shell, tanpa panic.
- [ ] Akses registri aman konkuren (mutex).
- [ ] Helper `BracketedPaste` menghasilkan urutan `\x1b[200~<text>\x1b[201~\r`.

ownedPaths: `internal/provider/provider.go`, `internal/provider/registry.go`, `internal/provider/launch.go`.

Paralel: harus selesai (atau minimal `provider.go`) sebelum `provider-impls` dan `provider-ipc` bisa dibuild; selain itu independen.

### provider-impls: Adapter claude, codex, shell

Deskripsi:
Tiga implementasi `Provider` yang mendaftar sendiri lewat `init()` ke registri (`Register(...)`).

Rincian:
- `claude.go`: ID `claude`, nama `Claude Code`, perintah `claude`, kapabilitas `{Hooks:true, MCP:true, IdleEvent:true}`. `Command` mengembalikan `claude` dengan `ctx.Args`; bila `ctx.MCPServers` tidak kosong, tulis berkas konfigurasi MCP sementara di `<DataDir>/run/mcp-<sessionId>.json` (pakai `internal/paths`) dan tambahkan `--mcp-config <path>`; env dari `ctx.Env`. Konfigurasi hooks tidak ditulis di sini (milik tiket 08 lewat contribution yang menambah `Args`/`Env`/berkas). `Available()` memakai `LookPath("claude")`.
- `codex.go`: ID `codex`, nama `Codex`, perintah `codex`. Spec 9b: kemampuan diverifikasi; default `{Hooks:false, MCP:false, IdleEvent:false}` sehingga memakai deteksi output-diam. Tuliskan konstanta kapabilitas di satu tempat dengan komentar bahwa nilai ini diverifikasi manual terhadap CLI codex terpasang, dan field dapat diaktifkan kemudian tanpa mengubah interface. `Available()` memakai `LookPath("codex")`.
- `shell.go`: ID `shell`, nama `Shell`, kapabilitas semua `false` (idle lewat output-diam). `Command` mengembalikan login shell: `$SHELL` (fallback `/bin/zsh`) dengan args `-l`, ditambah `ctx.Args`. `Available()` selalu tersedia selama shell ditemukan.
- `SendInput(w io.Writer, text string)` di ketiganya memakai `BracketedPaste`, lalu Enter (`\r`) sebagai penutup. Boleh dibagi lewat helper bersama di `provider.go`/`launch.go`, namun implementasi tiap provider memanggilnya sendiri.
- Error binary tidak ada: pesan jelas berbahasa Indonesia, mis. `provider "claude": binary "claude" tidak ditemukan di PATH; pasang atau tambahkan ke PATH`.
- Semua env hasil `Command` berbentuk `KEY=VALUE`.

Acceptance criteria:
- [ ] `go build ./...` lolos dengan ketiga file.
- [ ] Setelah impor paket, `List()` mengembalikan tepat `claude`, `codex`, `shell` dalam urutan itu.
- [ ] Kapabilitas claude: hooks, MCP, idle event true; codex: semua false (default, dengan komentar verifikasi); shell: semua false.
- [ ] `shell.Command` memakai login shell (`-l`).
- [ ] `claude.Command` menambah `--mcp-config` hanya bila `MCPServers` tidak kosong.
- [ ] `Available()` mengembalikan error jelas bila binary tidak ada di PATH; tidak panic.
- [ ] `SendInput` menulis bracketed paste lalu Enter ke writer.

ownedPaths: `internal/provider/claude.go`, `internal/provider/codex.go`, `internal/provider/shell.go`.

Paralel: bergantung pada interface dari `provider-core`; ketiga file impl saling independen dan bisa dikerjakan paralel satu sama lain.

### provider-ipc: Wiring provider ke sesi

Deskripsi:
Menyambungkan provider ke daemon dan CLI: method `provider.list`, penerimaan `providerId` pada `session.create`, pemasangan Launcher ke `session.Manager`, serta perintah CLI.

Rincian:
- `internal/provider/ipc.go`: fungsi `RegisterIPC(srv *ipc.Server)` yang mendaftarkan `provider.list`; fungsi `Validate(providerID string) error` yang mengembalikan error IPC `ERR_INVALID` untuk id tak dikenal dan `ERR_BINARY` bila binary tidak ada (dipakai oleh jalur `session.create`).
- `cmd/aitermd/wire_provider.go`: `init(){ RegisterModule("provider", func(srv *ipc.Server, d *Deps){ ... }) }` yang memanggil `provider.RegisterIPC(srv)`, memasang `provider.NewLauncher()` ke `session.Manager` milik `d` (lihat `Deps` di `cmd/aitermd/modules.go` dan API `Manager` di tiket 02), dan memasang validator provider ke jalur `session.create` bila tiket 02 menyediakan hook validasi. Urutan modul: provider harus terpasang setelah modul `session`; bila `RegisterModule` tidak menjamin urutan, pasang launcher dengan cara yang tidak bergantung urutan (mis. Manager membaca launcher saat sesi dibuat, bukan saat konstruksi).
- `cmd/aitermctl/cmd_provider.go`: `init(){ RegisterCommand(...) }` untuk subperintah `provider list` (memanggil `provider.list`, mencetak tabel). Untuk `session new --provider <id>`: `cmd/aitermctl/cmd_session.go` milik tiket 02, jadi epic ini tidak mengeditnya. Sediakan di `cmd_provider.go` perintah `session new --provider` hanya bila registri CLI mendukung subperintah/flag yang digabung; bila tidak, sediakan `aitermctl provider new <id> [--cwd ...]` yang memanggil `session.create` dengan `providerId`, dan flag `--provider` pada `session new` tercakup bila tiket 02 sudah meneruskan flag tak dikenal sebagai param `providerId`. Catat pilihan yang diambil di komentar kode.

Acceptance criteria:
- [ ] `make build` lolos (go build ./... plus build frontend yang sudah ada).
- [ ] `aitermctl provider list` menampilkan claude, codex, shell dengan kapabilitas dan status tersedia.
- [ ] `provider.list` mengembalikan JSON sesuai Kontrak.
- [ ] `session.create` dengan `providerId:"shell"` membuat sesi yang meluncurkan login shell; tanpa `providerId` memakai `shell`.
- [ ] `session.create` dengan `providerId` tak dikenal mengembalikan `-32003`; dengan binary tidak ada mengembalikan `-32000` dengan pesan jelas.
- [ ] Launcher provider terpasang ke `session.Manager` hanya lewat `wire_provider.go` tanpa mengedit `internal/session/`, `modules.go`, atau `registry.go`.
- [ ] Sesi claude/codex meluncurkan binary masing-masing bila terpasang.

ownedPaths: `internal/provider/ipc.go`, `cmd/aitermd/wire_provider.go`, `cmd/aitermctl/cmd_provider.go`.

Paralel: bergantung pada `provider-core` dan `provider-impls` (butuh registri dan impl untuk `provider.list`); dikerjakan terakhir atau setelah kontraknya disepakati.

## Path yang dimiliki

Hanya path berikut yang boleh disentuh:
- `internal/provider/` (termasuk `provider.go`, `registry.go`, `launch.go`, `claude.go`, `codex.go`, `shell.go`, `ipc.go`)
- `cmd/aitermd/wire_provider.go`
- `cmd/aitermctl/cmd_provider.go`

Dilarang menyentuh `go.mod`/`go.sum` kecuali dependensi baru benar-benar perlu (mestinya tidak; hanya stdlib). Bila perlu, hindari dan pakai stdlib. Tidak ada perubahan di `internal/session/`, `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go`, frontend, atau file milik epic lain.

## Wiring

Setelah semua fitur selesai:
1. Modul `provider` terdaftar otomatis lewat `init()` di `cmd/aitermd/wire_provider.go` (`RegisterModule("provider", ...)`); tidak ada edit `modules.go`.
2. Launcher provider dipasang ke `session.Manager` sehingga `session.create` dengan `providerId` meluncurkan provider terkait; default tetap login shell.
3. `provider.list` terdaftar di `ipc.Server`.
4. Perintah CLI terdaftar lewat `init()` di `cmd/aitermctl/cmd_provider.go` (`RegisterCommand`).
5. Untuk epic hilir (tidak dikerjakan di sini): tiket 08 (session-status) dan 09 (session-mention) memanggil `provider.RegisterLaunchContribution(...)` dari file `wire_*.go`/paket mereka sendiri untuk menambah env/args/`MCPServers`/hooks; tiket 06 (ui-shell) memakai `provider.list` untuk pemilih provider.

## Guard

Cukup build berhasil: `make build` (menjalankan `go build ./...` dan build wails/vite). Tanpa unit test. Pastikan juga `go vet ./internal/provider/...` tidak melaporkan error bila tersedia (opsional). Build harus tetap hijau setelah tiap fitur digabung, tidak hanya di akhir epic.
