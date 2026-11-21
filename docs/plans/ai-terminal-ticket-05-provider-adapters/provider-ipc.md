# Plan: Wiring provider ke sesi (IPC, modul daemon, CLI)

## Metadata

- Tiket: `ai-terminal-ticket-05-provider-adapters` (`docs/tickets/ai-terminal-ticket-05-provider-adapters.md`)
- Fitur: `provider-ipc`
- ownedPaths: `internal/provider/ipc.go`, `cmd/aitermd/wire_provider.go`, `cmd/aitermctl/cmd_provider.go`
- Depends on foundation: ya. `internal/provider/{provider.go,registry.go,launch.go}` sudah ada dari Foundation (`00-foundation.md`) dengan signature final overview bagian 2.2 sampai 2.4. Fitur ini hanya memakai `Get`, `List`, `Default`, `NewLauncher`, `Capabilities`, `Provider.Available`. Tidak menunggu `provider-core` atau `provider-impls` untuk build (registri kosong jatuh ke fallback).

Kontrak lengkap ada di `overview.md` (bagian 2.6, 2.9, 5). Plan ini tidak mengulangnya; hanya langkah konkret.

## Tujuan

Menyambungkan provider ke daemon dan CLI: method IPC `provider.list`, fungsi `Validate` untuk guard `session.create`, modul daemon `provider` (daftar IPC, pasang guard, pasang launcher lewat `OnReady`), dan perintah `aitermctl provider list|new`.

## Path yang dimiliki

HANYA tiga berkas ini yang boleh dibuat/diubah:

- `internal/provider/ipc.go`
- `cmd/aitermd/wire_provider.go`
- `cmd/aitermctl/cmd_provider.go`

Dilarang menyentuh: `internal/provider/{provider,registry,launch,claude,codex,shell}.go` (fitur lain), `internal/session/*`, `cmd/aitermd/wire_session.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go`, `cmd/aitermctl/cmd_session.go`, frontend, `go.mod`/`go.sum`. Jangan menambah `_test.go`.

## Kontrak

Disediakan (lihat overview 2.6):

- `type Info struct { ID string \`json:"id"\`; Name string \`json:"name"\`; Capabilities Capabilities \`json:"capabilities"\`; Available bool \`json:"available"\`; Path string \`json:"path,omitempty"\`; Error string \`json:"error,omitempty"\` }`
- `func RegisterIPC(srv *ipc.Server)` mendaftarkan method `provider.list`.
- `func Validate(providerID string) error` selalu mengembalikan `*ipc.Error` saat gagal (nilai `nil` murni bila sukses).
- Pesan IPC `provider.list`: params `{}`/kosong, hasil array Info (tidak pernah `null`).
- Perintah CLI `aitermctl provider list [--json]` dan `aitermctl provider new <id> [--cwd DIR] [--workspace ID] [--project ID]`.

Dipakai:

- Dari Foundation/provider-core: `provider.Get`, `provider.List`, `provider.Default`, `provider.NewLauncher`, `provider.Capabilities`, `Provider.Available() (string, error)`, `Provider.ID()`, `Provider.Name()`.
- Dari `internal/ipc`: `Server.Register(method, func(ctx context.Context, params json.RawMessage) (any, error))` (PANIC bila method dobel), `ipc.NewError`, `ipc.Errorf`, `ipc.ErrBinary` (-32000), `ipc.ErrInvalid` (-32003), `ipc.Client.Call(method, params, &out)`.
- Dari `cmd/aitermd/modules.go`: `RegisterModule(name, func(srv *ipc.Server, d *Deps))`, `Deps.Lookup`, `Deps.OnReady`, `Deps.Logger` (`*slog.Logger`).
- Dari `cmd/aitermctl/registry.go`: `RegisterCommand(Command{Name, Summary, Run})`, `printJSON(v any) error`.
- Dari `internal/session`: `session.Manager.SetLauncher(l Launcher)`, `session.CreateGuard` (variabel paket `func(providerID string) error`, DITAMBAHKAN agen Wiring; belum ada di kode saat ini, lihat bagian Wiring).

## Langkah

### 1. `internal/provider/ipc.go`

`package provider`. Import: `context`, `encoding/json`, `aiterm/internal/ipc`, `strings`.

1. Definisikan `Info` persis seperti di Kontrak, dengan komentar doc bahasa Indonesia.
2. `func infoFor(p Provider) Info`:
   - Isi `ID`, `Name`, `Capabilities` dari provider.
   - `path, err := p.Available()`; bila `err != nil`: `Available=false`, `Path=""`, `Error=err.Error()`; selain itu `Available=true`, `Path=path`.
3. `func RegisterIPC(srv *ipc.Server)`:
   ```go
   srv.Register("provider.list", func(ctx context.Context, params json.RawMessage) (any, error) {
       ps := List()
       out := make([]Info, 0, len(ps)) // tidak pernah nil -> JSON []
       for _, p := range ps {
           out = append(out, infoFor(p))
       }
       return out, nil
   })
   ```
   Params diabaikan (boleh kosong/`{}`/`null`); jangan error bila params berisi apa pun.
4. `func Validate(providerID string) error`:
   - `var p Provider`. Bila `providerID == ""`: `p = Default()` (ini provider shell; ketersediaannya tetap dicek).
   - Selain itu `p, ok := Get(providerID)`; bila `!ok`: kumpulkan id dari `List()` ke slice, `strings.Join(ids, ", ")`, kembalikan `ipc.Errorf(ipc.ErrInvalid, "providerId %q tidak dikenal; yang tersedia: %s", providerID, joined)`.
   - `if _, err := p.Available(); err != nil { return ipc.NewError(ipc.ErrBinary, err.Error()) }`.
   - Sukses: `return nil`. Hati-hati: jangan mengembalikan `*ipc.Error` bernilai nil lewat tipe `error` (typed-nil); semua return gagal berupa literal pointer non-nil, sukses berupa literal `nil`.
5. Tidak mengimpor `session` di file ini (tidak perlu).

### 2. `cmd/aitermd/wire_provider.go`

`package main`. Import: `aiterm/internal/ipc`, `aiterm/internal/provider`, `aiterm/internal/session`.

```go
func init() {
    RegisterModule("provider", func(srv *ipc.Server, d *Deps) {
        provider.RegisterIPC(srv)
        session.CreateGuard = provider.Validate
        d.OnReady(func() {
            v, ok := d.Lookup("session.manager")
            if !ok {
                d.Logger.Warn("provider: layanan session.manager tidak tersedia; launcher provider tidak dipasang")
                return
            }
            m, ok := v.(*session.Manager)
            if !ok || m == nil {
                d.Logger.Warn("provider: session.manager bertipe tidak terduga; launcher provider tidak dipasang")
                return
            }
            m.SetLauncher(provider.NewLauncher())
        })
    })
}
```

Catatan wajib di komentar kode:
- Modul dipasang alfabetis ("provider" sebelum "session"), jadi launcher WAJIB dipasang lewat `OnReady`, bukan langsung.
- Impor `internal/provider` di sini menjalankan `init()` claude/codex/shell sehingga registri terisi.
- `d.Logger` bisa saja nil pada `Deps` nilai nol; lindungi dengan helper kecil `if d.Logger != nil` sebelum `Warn` agar tidak panic.
- `session.CreateGuard` dan `d.Provide("session.manager", m)` disediakan oleh agen Wiring (edit ke `wire_session.go` dan `session/ipc.go`), bukan fitur ini.

### 3. `cmd/aitermctl/cmd_provider.go`

`package main`. Import: `context`, `errors`, `flag`, `fmt`, `os`, `text/tabwriter`, `aiterm/internal/ipc`. TIDAK mengimpor `aiterm/internal/provider`.

1. `const providerUsage = "penggunaan: aitermctl provider list|new ..."`; komentar file mencatat pilihan: `session new --provider` sudah disediakan epic 02 di `cmd_session.go`, sehingga `provider new <id>` hanya pintasan ke `session.create`.
2. Struct respons lokal (nama unik agar tidak bentrok dengan `sessionInfo` di `cmd_session.go`):
   ```go
   type providerCaps struct { Hooks, MCP, IdleEvent bool } // dengan tag json hooks, mcp, idleEvent
   type providerInfo struct {
       ID string `json:"id"`; Name string `json:"name"`
       Capabilities providerCaps `json:"capabilities"`
       Available bool `json:"available"`; Path string `json:"path"`; Error string `json:"error"`
   }
   ```
3. `init()`: `RegisterCommand(Command{Name: "provider", Summary: "Daftar dan uji provider: list|new", Run: runProvider})`.
4. `runProvider(ctx, c, args)`: `len(args)==0` -> `errors.New(providerUsage)`; switch `args[0]`: `"list"` -> `providerList(c, args[1:])`, `"new"` -> `providerNew(c, args[1:])`; default -> `errors.New(providerUsage)`.
5. `providerList(c, args)`:
   - `flag.NewFlagSet("provider list", flag.ContinueOnError)`, flag `--json` (bool). Argumen posisional tersisa (`fs.NArg() > 0`) -> error `penggunaan: aitermctl provider list [--json]`.
   - `var raw json.RawMessage` (tambahkan import `encoding/json`); `c.Call("provider.list", map[string]any{}, &raw)`.
   - Bila `--json`: unmarshal raw ke `any`, `printJSON(v)`.
   - Selain itu: unmarshal ke `[]providerInfo`, cetak lewat `tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)` dengan header `ID\tNAME\tHOOKS\tMCP\tIDLE\tAVAILABLE\tPATH`. Helper `yesNo(b bool) string` -> `ya`/`tidak` (beri nama unik, mis. `providerYesNo`). Kolom PATH: `Path` bila tersedia; bila tidak tersedia, pesan error ringkas (`Error`, dipotong bila lebih dari ~60 karakter dengan `...`; bila `Error` kosong tulis `-`). Akhiri dengan `tw.Flush()`.
6. `providerNew(c, args)`:
   - FlagSet `"provider new"` dengan `--cwd`, `--workspace`, `--project` (string). Karena `flag` berhenti di argumen posisional pertama, ambil `<id>` dengan memisahkan: bila `args[0]` tidak berawalan `-`, `id := args[0]; rest := args[1:]`; selain itu parse dulu semua lalu `id := fs.Arg(0)`. Sederhana dan aman: loop pisahkan satu argumen posisional dari flag, lalu `fs.Parse(rest)`; wajib tepat satu id, kalau tidak -> error `penggunaan: aitermctl provider new <id> [--cwd DIR] [--workspace ID] [--project ID]`.
   - Bangun `params := map[string]any{"providerId": id}`, tambahkan `cwd`/`workspaceId`/`projectId` hanya bila tidak kosong.
   - `c.Call("session.create", params, &raw)`; unmarshal ke struct lokal `struct{ ID string \`json:"id"\` }`; `fmt.Println(out.ID)`. Error `*ipc.Error` dikembalikan apa adanya (kode -32003/-32000 terlihat via `Error()`).
   - Impor `ipc` dipakai untuk tipe `*ipc.Client` pada signature.

### 4. Verifikasi

1. `gofmt -l internal/provider cmd` harus kosong.
2. `go build ./internal/provider/... ./cmd/aitermctl/...` harus hijau.
3. `go vet ./internal/provider/...` (opsional) tanpa error.
4. `go build ./cmd/aitermd`: SATU-SATUNYA kegagalan yang dapat diterima sebelum Wiring adalah `undefined: session.CreateGuard` pada `wire_provider.go` (variabel itu ditambahkan agen Wiring). Selain itu harus bersih. Jangan menambah stub `CreateGuard` sendiri dan jangan mengedit `internal/session/`. Bila `session.CreateGuard` sudah ada (Wiring sudah berjalan), `go build ./...` harus hijau penuh.

## Perubahan

Dibuat:
- `internal/provider/ipc.go`
- `cmd/aitermd/wire_provider.go`
- `cmd/aitermctl/cmd_provider.go`

Tidak ada berkas yang diubah.

## Wiring

Yang diekspos untuk disambungkan langkah wiring (JANGAN dikerjakan di fitur ini):
- Agen Wiring menambah `d.Provide("session.manager", m)` di `cmd/aitermd/wire_session.go` (ganti `_ *Deps` jadi `d *Deps`) dan `var CreateGuard func(providerID string) error` plus pemanggilannya di handler `session.create` pada `internal/session/ipc.go`. `wire_provider.go` sudah berisi pemakai kedua simbol itu (`session.CreateGuard = provider.Validate` dan `d.Lookup("session.manager")` dalam `OnReady`).
- `provider.RegisterIPC` terdaftar otomatis lewat modul `provider` (cek `daemon.info` memuat `provider`).
- `aitermctl provider` terdaftar otomatis lewat `init()`; tanpa edit `registry.go`.
- Epic hilir (tiket 06) memakai `provider.list`; tiket 08/09 memakai `RegisterLaunchContribution` (milik provider-core).

## Selesai bila

- Ketiga berkas ada, hanya ketiganya yang berubah, dan `gofmt -l internal/provider cmd` kosong.
- `go build ./internal/provider/... ./cmd/aitermctl/...` sukses; `go build ./cmd/aitermd` sukses, atau hanya gagal pada `undefined: session.CreateGuard` bila Wiring belum berjalan (setelah Wiring, `go build ./...` hijau).
- `Validate("")` memakai shell; id tak dikenal -> `*ipc.Error` kode `ipc.ErrInvalid` dengan daftar id tersedia; binary tidak ada -> `*ipc.Error` kode `ipc.ErrBinary` dengan pesan `provider "<id>": binary "<bin>" tidak ditemukan di PATH; pasang atau tambahkan ke PATH`.
- `provider.list` mengembalikan array `Info` (tidak pernah `null`).
- `wire_provider.go` tidak panic bila `session.manager` tidak ada (hanya `Warn`).
- `cmd_provider.go` tidak mengimpor `internal/provider`.
- Tanpa unit test; guard hanyalah build di atas.
