---
id: ai-terminal-ticket-04-workspace-model
title: Workspace Model
spec: docs/superpowers/specs/2026-11-21-ai-terminal-design.md
status: ready
wave: 2
depends_on: [ai-terminal-ticket-01-app-base]
---
# Workspace Model

## Tujuan

Epic ini menyediakan model **workspace** (nama + daftar project + konfigurasi) dan **project** (path di disk, milik workspace), lengkap dengan penyimpanan JSON di data dir, method IPC `workspace.*` dan `config.*`, serta subcommand `aitermctl workspace ...` untuk uji manual. Ini menjawab keluhan spec bagian 1 poin 2: daftar sesi milik workspace, bukan project, sehingga berpindah project tidak mengubah daftar sesi. Epic ini tidak bergantung pada sesi, file service, maupun provider; hanya bergantung pada app-base (server IPC, registri modul, registri perintah ctl, paths). Setelah epic ini selesai, `make build` tetap lolos dan daemon sudah bisa membuat, mengubah, dan menghapus workspace lewat `aitermctl`.

## Batasan

In scope:
- Tipe `Workspace`, `Project`, `WorkspaceConfig`, `GlobalConfig`.
- `Store` dengan persistensi atomik ke `workspaces.json` dan `config.json` di data dir.
- Validasi path project (ada di disk dan berupa direktori).
- Method IPC `workspace.create/list/rename/delete/addProject/removeProject`, `config.get/set`, plus event `workspace.changed`.
- Modul daemon `wire_workspace.go` dan subcommand ctl `cmd_workspace.go`.

Out of scope:
- Sesi (`session.*`), daftar sesi, PTY. Workspace TIDAK menyimpan sesi; sesi merujuk workspace lewat `workspaceId` di epic session-pty.
- Membaca isi/tree project (epic file-service), provider, status, mention.
- Memanggil Grimoire atau memakai GrimoireURL/GrimoireKey; epic ini hanya menyimpannya (integrasi Knowledge ada di fase 4, di luar tiket).
- UI/frontend (epic ui-shell). Tidak ada file frontend yang disentuh.
- Mengedit file milik epic lain (go.mod, `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go`, `internal/ipc/`, `internal/paths/`, dsb.). Bila membutuhkan dependency baru, gunakan hanya stdlib Go; ULID dibuat dengan implementasi kecil sendiri di dalam paket `internal/workspace` (lihat catatan fitur) agar `go.mod` tidak berubah.
- Unit test (guard hanya `make build`).

## Kontrak

### Tipe Go (paket `workspace`, `internal/workspace/types.go`)

```go
type Project struct {
    ID   string `json:"id"`   // ULID
    Path string `json:"path"` // absolut, sudah dibersihkan (filepath.Clean)
    Name string `json:"name"` // default: filepath.Base(Path)
}

type WorkspaceConfig struct {
    GrimoireURL string `json:"grimoireUrl,omitempty"` // opsional, override global
    GrimoireKey string `json:"grimoireKey,omitempty"` // opsional, override global
}

type Workspace struct {
    ID       string          `json:"id"`   // ULID
    Name     string          `json:"name"`
    Projects []Project       `json:"projects"` // selalu [] (bukan null) di JSON
    Config   WorkspaceConfig `json:"config"`
}

type GlobalConfig struct {
    GrimoireURL string `json:"grimoireUrl,omitempty"`
    GrimoireKey string `json:"grimoireKey,omitempty"`
}
```

Bentuk di kawat (JSON-RPC) mengikuti kontrak bersama: `Workspace{id, name, projects[{id,path,name}], config}`.

### API Go (paket `workspace`)

```go
func NewStore(dataDir string) (*Store, error)           // memuat file bila ada; membuat dir bila belum
func (s *Store) List() []Workspace
func (s *Store) Get(id string) (Workspace, error)
func (s *Store) Create(name string) (Workspace, error)
func (s *Store) Rename(id, name string) (Workspace, error)
func (s *Store) Delete(id string) error
func (s *Store) AddProject(wsID, path, name string) (Workspace, Project, error)
func (s *Store) RemoveProject(wsID, projectID string) (Workspace, error)
func (s *Store) GetConfig(wsID string) (GlobalConfig, WorkspaceConfig, GlobalConfig /*efektif*/, error)
func (s *Store) SetGlobalConfig(c GlobalConfig) error
func (s *Store) SetWorkspaceConfig(wsID string, c WorkspaceConfig) (Workspace, error)
func (s *Store) EffectiveConfig(wsID string) (GlobalConfig, error) // workspace override global per field non-kosong
```

Error sentinel yang dipakai paket (dipetakan ke kode IPC di `ipc.go`): `ErrNotFound`, `ErrInvalid`, `ErrConflict`.

```go
func Register(srv *ipc.Server, s *Store)   // internal/workspace/ipc.go
```

Konsumen lain di masa depan (ui-shell, provider-adapters, session-pty) hanya memakai IPC, bukan paket ini secara langsung. `EffectiveConfig` disediakan agar epic fase 4 bisa memakainya tanpa mengubah paket ini.

### Penyimpanan

- `<DataDir>/workspaces.json`: `{"version":1,"workspaces":[Workspace,...]}`.
- `<DataDir>/config.json`: `{"version":1,"global":GlobalConfig}`.
- Penulisan atomik: tulis ke file temp di dir yang sama (`os.CreateTemp`), `Sync`, lalu `os.Rename`. Izin file `0600` (menyimpan API key), dir `0700`.
- File belum ada = state kosong (bukan error). File korup (JSON tidak valid) = `NewStore` mengembalikan error yang jelas, tidak menimpa file.
- Seluruh akses ke store dilindungi `sync.RWMutex`.

### Pesan IPC (JSON-RPC 2.0, via `srv.Register`)

| Method | Params | Hasil | Error |
|---|---|---|---|
| `workspace.create` | `{"name":string}` | `Workspace` | -32003 bila nama kosong/hanya spasi |
| `workspace.list` | `{}` (atau kosong) | `Workspace[]` (urut sesuai pembuatan) | - |
| `workspace.rename` | `{"id":string,"name":string}` | `Workspace` | -32002 id tak ada, -32003 nama kosong |
| `workspace.delete` | `{"id":string}` | `{"ok":true}` | -32002 id tak ada |
| `workspace.addProject` | `{"workspaceId":string,"path":string,"name"?:string}` | `{"workspace":Workspace,"project":Project}` | -32002 workspace tak ada, -32003 path bukan direktori/tidak ada/bukan absolut, -32001 path sama sudah ada di workspace itu |
| `workspace.removeProject` | `{"workspaceId":string,"projectId":string}` | `Workspace` | -32002 workspace/project tak ada |
| `config.get` | `{"workspaceId"?:string}` | tanpa workspaceId: `{"global":GlobalConfig}`; dengan workspaceId: `{"global":GlobalConfig,"workspace":WorkspaceConfig,"effective":GlobalConfig}` | -32002 workspace tak ada |
| `config.set` | `{"workspaceId"?:string,"grimoireUrl"?:string,"grimoireKey"?:string}` | sama dengan `config.get` setelah perubahan | -32002, -32003 (URL non-kosong yang bukan http/https) |

Semantik `config.set`: field yang tidak dikirim tidak diubah; field dikirim sebagai string kosong menghapus nilai (untuk workspace berarti kembali memakai global). Tanpa `workspaceId` = mengubah config global.

Penghapusan workspace hanya menghapus catatan workspace dan project-nya; TIDAK menghapus apa pun di disk. (Pembersihan sesi milik workspace yang dihapus adalah urusan UI/epic sesi; epic ini tidak mengetahui sesi.)

### Event

- Topik `workspace.changed` lewat `srv.Publish("workspace.changed", payload)` setiap kali `workspace.create/rename/delete/addProject/removeProject` atau `config.set` (level workspace) berhasil. Payload: `{"kind":"created|renamed|deleted|projectAdded|projectRemoved|configChanged","workspaceId":string,"workspace":Workspace|null}` (`workspace` null untuk `deleted`). `config.set` global memakai `kind:"configChanged"` dengan `workspaceId` kosong.

### Yang dipakai dari epic lain (disediakan app-base)

- `internal/ipc`: `*ipc.Server` dengan `Register(method, func(ctx, params json.RawMessage) (any, error))` dan `Publish(topic, payload)`; konstanta/bentuk error -32000..-32003 (cek nama identifier sebenarnya di `internal/ipc` sebelum memakai; bila error dibuat lewat tipe khusus, gunakan itu; jangan mengubah paket ipc). Klien: `client.Call(method, params, &out)`.
- `internal/paths`: fungsi yang mengembalikan DataDir (`~/Library/Application Support/aiterm`). Baca `internal/paths` untuk nama fungsinya.
- `cmd/aitermd/modules.go`: `RegisterModule(name string, func(srv *ipc.Server, d *Deps))`. Baca `Deps` untuk mengetahui apakah sudah memuat DataDir; bila tidak, ambil dari `internal/paths`.
- `cmd/aitermctl/registry.go`: `RegisterCommand(...)`; baca signature-nya sebelum menulis `cmd_workspace.go`.

## Fitur

### workspace-store: Store dan persistensi

Deskripsi: implementasi tipe, store in-memory yang didukung file JSON, dan konfigurasi global. Tidak ada dependensi ke `internal/ipc` di fitur ini (kecuali tidak perlu); hanya stdlib, sehingga bisa dikerjakan dan di-build sendiri.

Detail:
- `types.go`: tipe di bagian Kontrak. Tambahkan helper `newID()` (ULID: 48-bit timestamp ms + 80-bit acak `crypto/rand`, di-encode Crockford base32 26 karakter, monotonik dalam milidetik yang sama tidak wajib). Letakkan helper di `types.go` agar tidak menambah file di luar ownedPaths.
- `store.go`: `Store`, `NewStore`, operasi CRUD workspace/project, `persist()` atomik. Setiap mutasi: ubah salinan/in-memory, `persist()`, bila persist gagal kembalikan error dan batalkan perubahan in-memory (rollback). `List`/`Get` mengembalikan salinan dalam (deep copy `Projects`) agar pemanggil tidak bisa memutasi state.
- Validasi project: `filepath.IsAbs`, `os.Stat` berhasil, `IsDir()`. Path disimpan setelah `filepath.Clean` dan `filepath.EvalSymlinks` (bila gagal EvalSymlinks tapi Stat sukses, pakai hasil Clean). Duplikat = path hasil normalisasi sama di workspace yang sama -> `ErrConflict`. Path yang sama boleh ada di workspace berbeda. Nama kosong -> `filepath.Base(path)`.
- Nama workspace: di-`TrimSpace`; kosong -> `ErrInvalid`. Nama duplikat diperbolehkan (id yang membedakan).
- `config.go`: `GlobalConfig`, baca/tulis `config.json`, `EffectiveConfig` (field workspace non-kosong menimpa global), validasi URL (kosong atau skema http/https dengan host).
- Tidak ada polling dan tidak ada goroutine latar; hanya bekerja saat dipanggil (selaras anti-goal spec bagian 4).

Acceptance criteria:
- [ ] `NewStore` pada dir kosong sukses tanpa membuat error; pada dir yang belum ada membuatnya dengan izin 0700.
- [ ] `Create` -> restart (`NewStore` baru pada dir sama) -> `List` masih memuat workspace tersebut.
- [ ] Penulisan atomik (temp file + rename); tidak pernah menulis langsung ke `workspaces.json`.
- [ ] File JSON korup membuat `NewStore` error dan file asli tidak berubah.
- [ ] `AddProject` menolak path relatif, path yang tidak ada, dan path yang berupa file; menolak duplikat di workspace yang sama dengan `ErrConflict`.
- [ ] `Projects` serialisasi sebagai `[]`, bukan `null`.
- [ ] `EffectiveConfig` memberi prioritas workspace per field, jatuh ke global bila kosong.
- [ ] File `config.json` dan `workspaces.json` berizin 0600.
- [ ] `go build ./...` lolos hanya dengan fitur ini terpasang.

ownedPaths:
- `internal/workspace/types.go`
- `internal/workspace/store.go`
- `internal/workspace/config.go`

Paralel: fitur ini tidak bergantung pada `workspace-ipc-ctl` dan dapat dikerjakan lebih dulu atau bersamaan. Fitur `workspace-ipc-ctl` bergantung pada signature API Go di bagian Kontrak; ikuti signature tersebut persis agar kedua fitur bisa dikerjakan paralel dan bertemu tanpa konflik.

### workspace-ipc-ctl: Method IPC dan CLI

Deskripsi: mengekspos Store lewat IPC, mem-publish `workspace.changed`, mendaftarkan modul daemon, dan menyediakan subcommand `aitermctl workspace ...` untuk uji manual.

Detail:
- `internal/workspace/ipc.go`: `Register(srv, store)` mendaftarkan 8 method di tabel Kontrak. Tiap handler: decode `params` dengan `json.Unmarshal` (params kosong/`null` diperlakukan sebagai `{}` bila method tak butuh parameter), panggil Store, petakan error: `ErrNotFound`->-32002, `ErrInvalid`->-32003, `ErrConflict`->-32001, JSON params rusak->-32003, error lain -> error internal bawaan ipc. Setelah mutasi sukses, `srv.Publish("workspace.changed", ...)`.
- `cmd/aitermd/wire_workspace.go`:
  ```go
  func init() {
      RegisterModule("workspace", func(srv *ipc.Server, d *Deps) {
          // buat Store dari DataDir, workspace.Register(srv, store)
      })
  }
  ```
  Bila `NewStore` gagal, log error dan lewati registrasi modul (daemon tetap hidup); jangan `panic`.
- `cmd/aitermctl/cmd_workspace.go`: `init(){ RegisterCommand(...) }` untuk perintah `workspace` dengan sub-aksi:
  - `workspace list`
  - `workspace create <name>`
  - `workspace rename <id> <name>`
  - `workspace delete <id>`
  - `workspace add-project <workspaceId> <path> [name]`
  - `workspace remove-project <workspaceId> <projectId>`
  - `workspace config get [workspaceId]`
  - `workspace config set [--workspace <id>] [--url <u>] [--key <k>]`
  Output berupa JSON ter-indentasi dari hasil RPC ke stdout; error RPC dicetak ke stderr dengan kode keluar non-nol. Gunakan `client.Call` dari `internal/ipc`; ikuti pola yang dipakai `registry.go`/perintah `ping` bawaan app-base. Kunci API pada output `config get` tidak perlu disamarkan di versi awal (alat debug lokal), namun jangan di-log oleh daemon.

Acceptance criteria:
- [ ] Setelah daemon jalan, `aitermctl workspace create demo` mengembalikan JSON Workspace dengan id ULID.
- [ ] `aitermctl workspace list` menampilkan workspace itu, dan tetap ada setelah daemon di-restart.
- [ ] `workspace add-project` dengan path valid mengembalikan project; path tak ada -> error -32003; duplikat -> -32001; workspace tak ada -> -32002.
- [ ] `workspace rename`, `delete`, `remove-project` bekerja sesuai tabel Kontrak.
- [ ] `config set` global lalu `config get <wsId>` menunjukkan `effective` mengikuti global; setelah `config set --workspace <id> --url ...`, `effective` memakai nilai workspace.
- [ ] Setiap mutasi mem-publish `workspace.changed` dengan payload sesuai Kontrak (dapat dicek lewat subscriber/klien debug app-base bila tersedia; minimal terlihat di kode).
- [ ] Modul terdaftar lewat `init()` tanpa mengedit `modules.go`; perintah terdaftar tanpa mengedit `registry.go`.
- [ ] `go build ./...` dan `make build` lolos.

ownedPaths:
- `internal/workspace/ipc.go`
- `cmd/aitermd/wire_workspace.go`
- `cmd/aitermctl/cmd_workspace.go`

Paralel: dapat dikerjakan bersamaan dengan `workspace-store` selama signature API Go di Kontrak dipatuhi. Untuk build yang lolos di tengah jalan, penulis fitur ini boleh membuat stub minimal sementara, namun file final `types.go`/`store.go`/`config.go` tetap milik `workspace-store`. Pengujian end-to-end baru bermakna setelah kedua fitur ada.

## Path yang dimiliki

Hanya path berikut yang boleh dibuat/diubah oleh epic ini:
- `internal/workspace/` (types.go, store.go, config.go, ipc.go)
- `cmd/aitermd/wire_workspace.go`
- `cmd/aitermctl/cmd_workspace.go`

Dilarang menyentuh: `go.mod`, `go.sum`, `Makefile`, `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go`, `internal/ipc/`, `internal/paths/`, `frontend/`, `app/`, dan path epic lain. Tidak ada dependency baru di `go.mod`.

## Wiring

- Registrasi daemon: otomatis lewat `init()` di `cmd/aitermd/wire_workspace.go` (`RegisterModule("workspace", ...)`); tidak ada edit ke file bersama.
- Registrasi CLI: otomatis lewat `init()` di `cmd/aitermctl/cmd_workspace.go` (`RegisterCommand(...)`).
- Ke luar epic: epic ui-shell (wave 4) akan memakai `workspace.*`, `config.*`, dan event `workspace.changed` lewat `frontend/src/lib/ipc.ts`; epic session-pty menyimpan `workspaceId`/`projectId` sebagai string id tanpa memvalidasi ke paket ini (tidak ada impor lintas epic). Kontrak bentuk Workspace/Project di atas bersifat tetap; perubahan harus dikoordinasikan.
- Tidak ada langkah manual setelah merge selain `make build`.

## Guard

Cukup build berhasil, tanpa unit test:
- [ ] `go build ./...`
- [ ] `make build` (go build + wails/vite build, tetap lolos karena frontend tidak diubah)
- [ ] Opsional uji manual: jalankan `aitermd`, lalu `aitermctl workspace create demo`, `workspace list`, `workspace add-project <id> <path-ada>`.
