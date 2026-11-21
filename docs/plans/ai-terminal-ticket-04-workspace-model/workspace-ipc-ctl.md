# Plan: Method IPC dan CLI

## Metadata

- Tiket: `ai-terminal-ticket-04-workspace-model` (`docs/tickets/ai-terminal-ticket-04-workspace-model.md`)
- Fitur: `workspace-ipc-ctl` (Method IPC dan CLI)
- ownedPaths: `internal/workspace/ipc.go`, `cmd/aitermd/wire_workspace.go`, `cmd/aitermctl/cmd_workspace.go`
- Depends on foundation: ya. Build terhadap `types.go` final dan stub `store.go`/`config.go` dari Foundation (`00-foundation.md`). Jangan membuat stub sendiri. Bila stub sudah diganti `workspace-store`, signature sama sehingga tidak ada konflik.

## Tujuan

Mengekspos `*workspace.Store` lewat 8 method IPC (`workspace.create|list|rename|delete|addProject|removeProject`, `config.get|set`), memetakan error sentinel ke kode IPC, mem-publish event `workspace.changed`, mendaftarkan modul daemon via `init()`, dan menyediakan subcommand `aitermctl workspace ...` untuk uji manual. Kontrak lengkap (tabel pesan, payload event, API Store) ada di `overview.md` bagian 2 dan tiket bagian Kontrak; plan ini tidak mengulanginya.

## Path yang dimiliki

HANYA tiga file ini yang boleh dibuat/diubah:

- `internal/workspace/ipc.go`
- `cmd/aitermd/wire_workspace.go`
- `cmd/aitermctl/cmd_workspace.go`

Dilarang menyentuh: `types.go`, `store.go`, `config.go` (milik Foundation/`workspace-store`), `go.mod`, `go.sum`, `Makefile`, `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go`, `internal/ipc/`, `internal/paths/`, `frontend/`, `app/`, dan path epic lain. Tanpa unit test, tanpa dependency baru (stdlib + `aiterm/internal/ipc` + `aiterm/internal/paths`).

## Kontrak

Dipakai dari Foundation/`workspace-store` (paket `aiterm/internal/workspace`): tipe `Workspace`, `Project`, `WorkspaceConfig`, `GlobalConfig`; sentinel `ErrNotFound`, `ErrInvalid`, `ErrConflict`; `NewStore(dataDir string) (*Store, error)` dan semua method Store pada overview 2.3 (`List`, `Get`, `Create`, `Rename`, `Delete`, `AddProject`, `RemoveProject`, `GetConfig`, `SetGlobalConfig`, `SetWorkspaceConfig`, `EffectiveConfig`). Semantik penting: `GetConfig("")` mengembalikan `(global, WorkspaceConfig{}, global, nil)`; `Set*` menerima nilai LENGKAP; validasi URL ada di dalam Store (mengembalikan `ErrInvalid`).

Dipakai dari app-base (sudah ada, terverifikasi dari kode):
- `ipc.Handler = func(ctx context.Context, params json.RawMessage) (any, error)`; `(*ipc.Server).Register(method string, h ipc.Handler)`; `(*ipc.Server).Publish(topic string, payload any)`.
- Konstanta `ipc.ErrConflict` (-32001), `ipc.ErrNotFound` (-32002), `ipc.ErrInvalid` (-32003), `ipc.ErrInvalidParams` (-32602); `ipc.NewError(code, msg) *ipc.Error`. Error non-`*ipc.Error` otomatis menjadi -32603.
- `paths.DataDir() (string, error)` dari `aiterm/internal/paths`.
- Daemon: `RegisterModule(name string, wire func(srv *ipc.Server, d *Deps))`; `Deps` punya `DataDir string` dan `Logger *slog.Logger` (nilai nol aman, jadi `Logger` bisa nil: guard sebelum dipakai).
- CLI: `RegisterCommand(Command{Name, Summary, Run func(ctx context.Context, c *ipc.Client, args []string) error})`; helper `printJSON(v any) error` dan `callAndPrint(c *ipc.Client, method string, params any) error`; `c.Call(method, params, &out)`. `main.go` sudah mencetak `*ipc.Error` sebagai `galat <kode>: <pesan>` ke stderr dan keluar 1, jadi `Run` cukup mengembalikan error. Catatan: flag global `--socket` diparse `main` sebelum nama command, sehingga args yang diterima `Run` hanya setelah `workspace`.

Disediakan fitur ini: `func Register(srv *ipc.Server, s *Store)` di `ipc.go`; modul daemon `"workspace"`; command `"workspace"`.

## Langkah

### 1. `internal/workspace/ipc.go`

`package workspace`. Import: `context`, `encoding/json`, `errors`, `aiterm/internal/ipc`.

1. Helper pemetaan error:
   ```go
   func mapErr(err error) error {
       switch {
       case err == nil: return nil
       case errors.Is(err, ErrNotFound): return ipc.NewError(ipc.ErrNotFound, err.Error())
       case errors.Is(err, ErrInvalid):  return ipc.NewError(ipc.ErrInvalid, err.Error())
       case errors.Is(err, ErrConflict): return ipc.NewError(ipc.ErrConflict, err.Error())
       default: return err // menjadi -32603 oleh ipc
       }
   }
   ```
2. Helper decode params: `func decode(params json.RawMessage, v any) error`. Bila `len(bytes.TrimSpace(params))==0` atau `null`, perlakukan sebagai `{}` (kembalikan nil, v tetap nol). Bila `json.Unmarshal` gagal, kembalikan `ipc.NewError(ipc.ErrInvalid, "params tidak valid: "+err.Error())` (tiket: JSON rusak -> -32003; pakai `bytes` di import).
3. Helper publish: `func publish(srv *ipc.Server, kind, wsID string, ws *Workspace)` -> `srv.Publish("workspace.changed", map[string]any{"kind": kind, "workspaceId": wsID, "workspace": ws})`. `ws` bertipe `*Workspace` sehingga nil menjadi JSON `null` (untuk `deleted` dan config global). Salinan `Workspace` hasil Store dipakai (ambil alamat variabel lokal).
4. `func Register(srv *ipc.Server, s *Store)`: daftarkan 8 method. Tiap handler: decode, validasi param wajib (id/workspaceId kosong biarkan Store menjawab `ErrNotFound`/`ErrInvalid`), panggil Store, `mapErr`, publish bila mutasi sukses.
   - `workspace.create`: params `{Name string "name"}`; `s.Create(p.Name)`; publish `created`; return `Workspace`.
   - `workspace.list`: params diabaikan (decode ke struct kosong, toleran `null`/kosong); return `s.List()` (sudah non-nil). Tanpa publish.
   - `workspace.rename`: `{ID "id", Name "name"}`; `s.Rename`; publish `renamed`; return `Workspace`.
   - `workspace.delete`: `{ID "id"}`; `s.Delete`; publish `deleted` dengan `workspace` nil dan workspaceId = id; return `map[string]bool{"ok": true}`.
   - `workspace.addProject`: `{WorkspaceID "workspaceId", Path "path", Name "name"}` (Name string biasa; kosong = default dari Store); `s.AddProject`; publish `projectAdded`; return `map[string]any{"workspace": ws, "project": proj}`.
   - `workspace.removeProject`: `{WorkspaceID "workspaceId", ProjectID "projectId"}`; `s.RemoveProject`; publish `projectRemoved`; return `Workspace`.
   - `config.get`: params `{WorkspaceID *string/ string "workspaceId"}`. Bentuk respons lewat helper `configResult(s, wsID string) (any, error)`: bila `wsID==""` -> `{"global": g}` (elemen pertama `GetConfig("")`); bila tidak -> `{"global": g, "workspace": wc, "effective": eff}`.
   - `config.set`: params struct `{WorkspaceID string "workspaceId"; GrimoireURL *string "grimoireUrl"; GrimoireKey *string "grimoireKey"}`. Logika patch: panggil `s.GetConfig(wsID)` untuk nilai saat ini (error -> `mapErr`). Tanpa workspaceId: salin `global`, timpa field non-nil, `s.SetGlobalConfig(g)`; publish `configChanged` dengan workspaceId `""` dan workspace nil. Dengan workspaceId: salin `wc`, timpa field non-nil, `s.SetWorkspaceConfig(wsID, wc)`; publish `configChanged` dengan workspaceId dan workspace hasil Set. Return `configResult(s, wsID)` setelah perubahan. Pointer `*string` membedakan "tidak dikirim" dari string kosong (menghapus nilai).
5. Jangan log atau menyertakan `GrimoireKey` di pesan error/log apa pun. Tidak ada goroutine, tidak ada state tambahan.

### 2. `cmd/aitermd/wire_workspace.go`

`package main`. Import: `aiterm/internal/ipc`, `aiterm/internal/paths`, `aiterm/internal/workspace`.

```go
func init() {
    RegisterModule("workspace", func(srv *ipc.Server, d *Deps) {
        dir := d.DataDir
        if dir == "" {
            p, err := paths.DataDir()
            if err != nil { logWorkspaceErr(d, err); return }
            dir = p
        }
        store, err := workspace.NewStore(dir)
        if err != nil { logWorkspaceErr(d, err); return } // jangan panic; daemon tetap hidup
        workspace.Register(srv, store)
    })
}
```
`logWorkspaceErr(d *Deps, err error)` (nama unik, huruf kecil, hindari bentrok dengan paket main lain): bila `d.Logger != nil` panggil `d.Logger.Error("modul workspace dilewati", "err", err)`. Jangan log config/key. Jangan mengubah `modules.go`; jangan memakai `d.Provide`.

### 3. `cmd/aitermctl/cmd_workspace.go`

`package main`. Import: `context`, `errors`/`fmt`, `flag`, `aiterm/internal/ipc`.

`init()` memanggil `RegisterCommand(Command{Name: "workspace", Summary: "kelola workspace dan project (list, create, rename, delete, add-project, remove-project, config)", Run: runWorkspace})`.

`func runWorkspace(ctx context.Context, c *ipc.Client, args []string) error`: bila `len(args)==0` kembalikan error usage (`workspaceUsage` string multi-baris yang mendaftar semua sub-aksi). Switch `args[0]`:
- `list`: `callAndPrint(c, "workspace.list", struct{}{})`.
- `create <name>`: butuh tepat 1 argumen (atau gabungkan sisa argumen dengan spasi); `workspace.create` `{"name":...}`.
- `rename <id> <name>`: `workspace.rename` `{"id","name"}`.
- `delete <id>`: `workspace.delete` `{"id"}`.
- `add-project <workspaceId> <path> [name]`: path dikirim apa adanya (jika relatif, ubah ke absolut dengan `filepath.Abs` di sisi CLI agar nyaman; error server tetap -32003 bila tidak ada); `workspace.addProject` `{"workspaceId","path","name"?}` (kirim `name` hanya bila ada).
- `remove-project <workspaceId> <projectId>`: `workspace.removeProject`.
- `config`: sub-aksi `args[1]`:
  - `get [workspaceId]`: `config.get` dengan `{}` atau `{"workspaceId":id}`.
  - `set [--workspace <id>] [--url <u>] [--key <k>]`: pakai `flag.NewFlagSet("workspace config set", flag.ContinueOnError)` dengan `--workspace`, `--url`, `--key`; gunakan `fs.Visit` untuk mendeteksi flag yang benar-benar diberikan (agar `--url ""` menghapus nilai, dan flag yang tidak diberikan tidak dikirim). Bangun `map[string]any` hanya dengan field yang diberikan (`workspaceId`, `grimoireUrl`, `grimoireKey`); panggil `config.set`. Bila tidak ada flag perubahan sama sekali, kembalikan error usage.
- default: error `sub-aksi tidak dikenal` + usage.

Argumen kurang -> `fmt.Errorf` usage singkat (main mencetaknya ke stderr, exit 1). Output sukses selalu lewat `callAndPrint` (JSON ter-indentasi ke stdout). Galat RPC dikembalikan apa adanya sehingga `main` mencetak kode dan keluar non-nol.

### 4. Verifikasi

- `gofmt -l internal/workspace cmd` harus kosong.
- `go build ./...` hijau (terhadap stub Foundation maupun store final).
- `go vet ./internal/workspace/... ./cmd/...` opsional.
- `git`/`go.mod` tidak berubah.

## Perubahan

Dibuat (semua baru):
- `internal/workspace/ipc.go`
- `cmd/aitermd/wire_workspace.go`
- `cmd/aitermctl/cmd_workspace.go`

Tidak ada file yang diubah.

## Wiring

Yang diekspos untuk langkah wiring (tidak disambungkan sendiri oleh fitur ini, tidak ada edit file bersama):
- `workspace.Register(srv *ipc.Server, s *Store)` di `internal/workspace/ipc.go`, mendaftarkan 8 method dan memublikasikan topik `workspace.changed`.
- `init()` di `cmd/aitermd/wire_workspace.go` memanggil `RegisterModule("workspace", ...)`.
- `init()` di `cmd/aitermctl/cmd_workspace.go` memanggil `RegisterCommand` untuk `workspace`.
- Agen wiring cukup memverifikasi checklist overview bagian 5 (tidak ada perubahan `modules.go`/`registry.go`, nama method cocok tiket, tidak ada stub tersisa setelah `workspace-store` selesai).

## Selesai bila

- Ketiga file ada dan hanya ketiganya yang dibuat/diubah.
- `go build ./...` sukses dan `gofmt -l internal/workspace cmd` kosong (guard; tanpa unit test).
- 8 method terdaftar dengan nama persis tiket; `ErrNotFound`/`ErrInvalid`/`ErrConflict` dipetakan ke -32002/-32003/-32001 via `errors.Is`; JSON params rusak -> -32003; error lain tidak dibungkus.
- Setiap mutasi sukses (`create`, `rename`, `delete`, `addProject`, `removeProject`, `config.set`) memanggil `srv.Publish("workspace.changed", ...)` dengan payload sesuai overview 2.5 (`workspace` null untuk `deleted` dan config global).
- `config.set` memakai pointer `*string` (field tak dikirim tidak diubah, string kosong menghapus).
- `wire_workspace.go` log dan skip (tanpa panic) bila `NewStore` gagal; `GrimoireKey` tidak pernah di-log.
- `aitermctl workspace` mendukung `list`, `create`, `rename`, `delete`, `add-project`, `remove-project`, `config get`, `config set`.
