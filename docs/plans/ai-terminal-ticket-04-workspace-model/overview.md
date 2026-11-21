# Overview: Epic 04 Workspace Model

Sumber: tiket `docs/tickets/ai-terminal-ticket-04-workspace-model.md` dan spec `docs/superpowers/specs/2026-11-21-ai-terminal-design.md`. Tiket adalah sumber kebenaran; bila overview ini bertentangan dengan tiket, tiket menang.

## 1. Ringkasan epic dan urutan eksekusi

Epic ini menyediakan model workspace (nama + daftar project + konfigurasi) dan project (path di disk), penyimpanan JSON atomik di DataDir, method IPC `workspace.*` dan `config.*`, event `workspace.changed`, serta subcommand `aitermctl workspace ...`. Workspace TIDAK menyimpan sesi. Hanya stdlib Go; `go.mod` tidak berubah.

Urutan eksekusi:

1. **Foundation** (satu agent, lihat `00-foundation.md`): `internal/workspace/types.go` final (tipe, `newID`, error sentinel) plus stub `store.go` dan `config.go` dengan signature persis kontrak, sehingga `go build ./...` hijau.
2. **Fitur paralel**: `workspace-store` dan `workspace-ipc-ctl`.
3. **Wiring**: tidak ada edit file bersama (registrasi lewat `init()`); agen wiring hanya memverifikasi (bagian 5).

Kondisi kode yang sudah ada (app-base, sudah merge): module `aiterm`, Go 1.26.3; `internal/ipc`, `internal/paths`, `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go` sudah ada. Paket `internal/workspace/` belum ada.

## 2. Kontrak bersama

### 2.1 Paket dan path

- Import path: `aiterm/internal/workspace` (nama paket `workspace`).
- File: `internal/workspace/{types.go,store.go,config.go,ipc.go}`, `cmd/aitermd/wire_workspace.go`, `cmd/aitermctl/cmd_workspace.go`.
- Penamaan: identifier Go bahasa Inggris; komentar boleh Indonesia (selaras kode app-base). JSON camelCase sesuai tag di tiket.

### 2.2 Tipe (`types.go`, milik Foundation; final, tidak diubah fitur)

Persis seperti tiket bagian Kontrak: `Project{ID,Path,Name}`, `WorkspaceConfig{GrimoireURL,GrimoireKey}`, `Workspace{ID,Name,Projects,Config}`, `GlobalConfig{GrimoireURL,GrimoireKey}` dengan tag json yang sama (`id`, `path`, `name`, `projects`, `config`, `grimoireUrl,omitempty`, `grimoireKey,omitempty`).

Tambahan di `types.go`:

```go
var (
    ErrNotFound = errors.New("not found")
    ErrInvalid  = errors.New("invalid argument")
    ErrConflict = errors.New("conflict")
)
func newID() string // ULID 26 karakter Crockford base32 (48-bit ms + 80-bit crypto/rand)
```

Pembungkus error: store memakai `fmt.Errorf("%w: detail", ErrX)`; pemanggil memakai `errors.Is`. `ipc.go` memetakan memakai `errors.Is`, pesan IPC = `err.Error()`.

### 2.3 API Store (persis tiket; milik `workspace-store`)

```go
func NewStore(dataDir string) (*Store, error)
func (s *Store) List() []Workspace
func (s *Store) Get(id string) (Workspace, error)
func (s *Store) Create(name string) (Workspace, error)
func (s *Store) Rename(id, name string) (Workspace, error)
func (s *Store) Delete(id string) error
func (s *Store) AddProject(wsID, path, name string) (Workspace, Project, error)
func (s *Store) RemoveProject(wsID, projectID string) (Workspace, error)
func (s *Store) GetConfig(wsID string) (GlobalConfig, WorkspaceConfig, GlobalConfig, error)
func (s *Store) SetGlobalConfig(c GlobalConfig) error
func (s *Store) SetWorkspaceConfig(wsID string, c WorkspaceConfig) (Workspace, error)
func (s *Store) EffectiveConfig(wsID string) (GlobalConfig, error)
```

Catatan semantik yang dipakai `ipc.go`:
- `GetConfig("")` (wsID kosong) mengembalikan `(global, WorkspaceConfig{}, global, nil)`; `ipc.go` memakai hanya elemen pertama bila `workspaceId` tidak dikirim. Dengan wsID tidak ada -> `ErrNotFound`.
- `SetGlobalConfig` dan `SetWorkspaceConfig` menerima nilai LENGKAP (bukan patch). Logika "field tidak dikirim tidak diubah" dikerjakan di `ipc.go`: ambil nilai saat ini lewat `GetConfig`, timpa field yang dikirim (pointer `*string` di struct params), lalu panggil Set. Validasi URL (kosong atau http/https dengan host) dilakukan di dalam `SetGlobalConfig`/`SetWorkspaceConfig` dan mengembalikan `ErrInvalid`.
- `List()` mengembalikan `[]Workspace` non-nil (`[]` saat kosong), urutan pembuatan; `Projects` selalu non-nil.
- Persistensi: `<DataDir>/workspaces.json` `{"version":1,"workspaces":[...]}`; `<DataDir>/config.json` `{"version":1,"global":{...}}`; atomik (CreateTemp + Sync + Rename), file 0600, dir 0700; file korup -> `NewStore` error tanpa menimpa; `sync.RWMutex`; rollback in-memory bila persist gagal.

### 2.4 `Register` (milik `workspace-ipc-ctl`)

```go
func Register(srv *ipc.Server, s *Store) // internal/workspace/ipc.go
```

### 2.5 Pesan IPC dan event

Method, params, hasil, dan kode error sesuai tabel di tiket (bagian "Pesan IPC"): `workspace.create|list|rename|delete|addProject|removeProject`, `config.get|set`. Kode error memakai konstanta `ipc.ErrConflict` (-32001), `ipc.ErrNotFound` (-32002), `ipc.ErrInvalid` (-32003) dari `internal/ipc/errors.go`. Handler mengembalikan `*ipc.Error` via `ipc.NewError(code, msg)` / `ipc.Errorf(code, fmt, ...)`; error lain dikembalikan apa adanya (menjadi -32603). `ipc.Handler` = `func(ctx context.Context, params json.RawMessage) (any, error)`.

Event: `srv.Publish("workspace.changed", payload)` dengan payload `{"kind":"created|renamed|deleted|projectAdded|projectRemoved|configChanged","workspaceId":string,"workspace":Workspace|null}`; config global: `kind:"configChanged"`, `workspaceId:""`, `workspace:null`.

### 2.6 Wiring ke app-base (sudah ada, jangan diubah)

- Daemon: `RegisterModule(name string, wire func(srv *ipc.Server, d *Deps))`; `Deps.DataDir` sudah ada. `wire_workspace.go` memakai `d.DataDir` (bila kosong, fallback `paths.DataDir()`), `d.Logger` untuk log error.
- CLI: `RegisterCommand(Command{Name, Summary, Run func(ctx context.Context, c *ipc.Client, args []string) error})`. Helper yang boleh dipakai: `printJSON(v any) error` dan `callAndPrint(c *ipc.Client, method string, params any) error`. Klien: `c.Call(method, params, &out)`. Galat RPC dikembalikan dari `Run` (main yang mencetak ke stderr dan keluar non-nol; periksa `cmd/aitermctl/main.go` untuk memastikan).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `workspace-store` | Implementasi final `Store` dan konfigurasi: CRUD workspace/project, validasi path (IsAbs, Stat, IsDir, Clean+EvalSymlinks, duplikat -> ErrConflict), persist atomik, rollback, deep copy, `config.go` (baca/tulis `config.json`, `EffectiveConfig`, validasi URL). Hanya stdlib, tanpa dependensi ke `internal/ipc`. | `internal/workspace/store.go`, `internal/workspace/config.go` |
| `workspace-ipc-ctl` | `Register` 8 method IPC + publish `workspace.changed`, pemetaan error, modul daemon `wire_workspace.go`, subcommand `aitermctl workspace ...` (list, create, rename, delete, add-project, remove-project, config get/set). | `internal/workspace/ipc.go`, `cmd/aitermd/wire_workspace.go`, `cmd/aitermctl/cmd_workspace.go` |

Catatan kepemilikan: `types.go` di tiket tercantum milik `workspace-store`, tetapi karena dibutuhkan kedua fitur, Foundation menulisnya versi final. Agen `workspace-store` TIDAK menulis ulang `types.go` (boleh membaca; hanya mengubah bila ada bug nyata, dan tidak mengubah signature/tag). `store.go` dan `config.go` dari Foundation berupa stub yang diganti total oleh `workspace-store`.

## 4. Foundation

`hasFoundation=true`. Rincian di `00-foundation.md`. Isi: `types.go` final, `store.go` stub (tipe `Store` + semua method kontrak mengembalikan `fmt.Errorf("%w: belum diimplementasi", ErrInvalid)`), `config.go` stub (bila perlu helper kosong). Hasil: `go build ./internal/workspace/...` dan `go build ./...` hijau, sehingga `workspace-ipc-ctl` bisa dibuild terhadap API final tanpa menunggu `workspace-store`.

## 5. Wiring checklist

Tidak ada file bersama yang diedit. Setelah kedua fitur selesai, agen wiring memverifikasi:

1. `cmd/aitermd/wire_workspace.go` memanggil `RegisterModule("workspace", ...)` di `init()` dan memanggil `workspace.Register(srv, store)`; `modules.go` tidak diubah.
2. `cmd/aitermctl/cmd_workspace.go` memanggil `RegisterCommand` untuk `workspace` di `init()`; `registry.go` tidak diubah.
3. Tidak ada stub `STUB`/"belum diimplementasi" tersisa di `store.go` dan `config.go` (`grep -rn "belum diimplementasi" internal/workspace`).
4. `go.mod`/`go.sum` tidak berubah dibanding sebelum epic.
5. Nama method IPC di `ipc.go` cocok dengan tabel tiket (8 method) dan topik event `workspace.changed`.
6. Jalankan build guard (bagian 6). Uji manual opsional: `AITERM_DATA_DIR=<dir pendek> ./build/bin/aitermd &`, lalu `aitermctl workspace create demo`, `workspace list`, `workspace add-project <id> <path-ada>`.

Bila ada celah (mis. salah satu file hilang atau tidak ter-build), wiring memperbaikinya seminimal mungkin tanpa menyentuh file bersama.

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- `go build ./...`
- `make build` (hanya di tahap Wiring/akhir; frontend tidak diubah)
- Per fitur/foundation cukup `go build ./...` dan `gofmt -l internal/workspace cmd` (harus kosong); `go vet ./internal/workspace/...` opsional.

## 7. Aturan paralel

- Tiap agent hanya membuat/mengubah `ownedPaths` fiturnya. Dilarang menyentuh path fitur lain, `types.go` (kecuali bug nyata), `go.mod`, `go.sum`, `Makefile`, `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go`, `internal/ipc/`, `internal/paths/`, `frontend/`, `app/`, dan path epic lain.
- Patuhi signature bagian 2 persis; jangan menambah method publik baru di `Store` yang dipakai lintas fitur.
- Tanpa dependency baru (stdlib saja), tanpa goroutine latar/polling, tanpa unit test.
- `workspace-ipc-ctl` membuild terhadap stub Foundation; jangan membuat stub sendiri. Bila stub Foundation sudah diganti `workspace-store`, tidak ada konflik karena signature sama.
- Jangan log nilai `GrimoireKey` di daemon.
