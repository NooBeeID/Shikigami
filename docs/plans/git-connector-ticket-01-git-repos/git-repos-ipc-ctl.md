# Plan: IPC git.repos.list dan shikigamictl git repos

## Metadata
- Tiket: `git-connector-ticket-01-git-repos` (`docs/tickets/git-connector-ticket-01-git-repos.md`)
- Fitur: `git-repos-ipc-ctl`
- ownedPaths: `cmd/shikigamid/wire_git.go`, `cmd/shikigamictl/cmd_git.go`
- Depends on foundation: tidak (epic tanpa `00-foundation.md`). Ketergantungan lunak: memakai paket `internal/gitdata` dari fitur paralel `gitdata-foundation` (lihat Kontrak). Rujukan kontrak bersama: `docs/plans/git-connector-ticket-01-git-repos/overview.md` bagian 2.3, 2.4, 2.5 (jangan diubah).

## Tujuan
Menyediakan lapisan IPC dan CLI debug untuk deteksi repo Git:
1. Daemon: modul `git` yang mendaftarkan metode IPC `git.repos.list {workspaceId}` -> `Repo[]`, plus helper bersama (`gitStore`, `gitErr`, `mapGitError`, `gitDecode`, `gitResolveRepo`) yang nanti dipakai file `wire_git_*.go` tiket 02 sampai 04.
2. `shikigamictl`: command `git` dengan dispatcher subcommand yang bisa diperluas (`registerGitSub`) dan subcommand `repos --workspace <id>`.

## Path yang dimiliki
HANYA dua berkas ini yang boleh dibuat/diubah:
- `cmd/shikigamid/wire_git.go`
- `cmd/shikigamictl/cmd_git.go`

Jangan menyentuh: `internal/gitdata/*` (milik `gitdata-foundation`; JANGAN membuat stub atau salinan walau paket belum ada), `cmd/shikigamid/modules.go`, `cmd/shikigamid/wire_lark*.go`, `cmd/shikigamictl/{main,registry,cmd_lark*}.go`, berkas wiring pusat, frontend, `internal/connector/*`. Jangan menambah `*_test.go`. Jangan commit.

## Kontrak

### Yang dipakai (sudah ada di kode, diverifikasi)
- `RegisterModule(name string, fn func(srv *ipc.Server, d *Deps))` dan `d.Lookup("workspace.store")` di `cmd/shikigamid` (pola `wire_lark.go`: `RegisterModule("lark", func(srv *ipc.Server, d *Deps) {...})` dipanggil dari `init()`).
- `ipc.NewError(code int, msg string)`; konstanta `ipc.ErrInternal`, `ipc.ErrNotFound`, `ipc.ErrInvalid`, `ipc.ErrConflict` (di `internal/ipc/errors.go`). `srv.Register(method, func(ctx context.Context, params json.RawMessage) (any, error))`.
- `workspace.Store.Get(id string) (workspace.Workspace, error)`; `workspace.Workspace.Projects []workspace.Project{ID, Path, Name}`; `workspace.ErrNotFound` (galat `errors.Is`).
- `RegisterCommand(Command{Name, Summary, Run func(ctx, c *ipc.Client, args []string) error})` dan `callAndPrint(c *ipc.Client, method string, params any) error` di `cmd/shikigamictl` (`registry.go`).
- Dari `internal/gitdata` (fitur paralel; tulis kode terhadap signature ini, sesuai overview 2.3):
  ```go
  type ProjectInput struct { ID, Name, Path string }
  type Repo struct { ProjectID, ProjectName string; Projects []RepoProject; Provider Provider; Host, Path, WebURL string }
  type RepoProject struct { ID, Name string }
  type ErrorCode string
  type Error struct { Code ErrorCode; Message string }
  func (e *Error) Error() string
  func AsError(err error) (*Error, bool)
  func DetectRepos(ctx context.Context, projects []ProjectInput) ([]Repo, error)
  ```
  Konstanta kode: `CodeCLIMissing`, `CodeNotLoggedIn`, `CodePermission`, `CodeNotFound`, `CodeTimeout`, `CodeNotAllowed`, `CodeCLI`.

### Yang disediakan (nama WAJIB persis; package `main`)
Di `cmd/shikigamid/wire_git.go`:
```go
func init()                                                      // RegisterModule("git", ... registerGitRepos(srv, d))
func registerGitRepos(srv *ipc.Server, d *Deps)                  // mendaftar "git.repos.list"
func gitStore(d *Deps) (*workspace.Store, error)
func gitErr(code string, ipcCode int, msg string) error          // ipc.NewError(ipcCode, "["+code+"] "+msg)
func mapGitError(err error) error                                // *gitdata.Error -> gitErr; lainnya -> ipc.ErrInternal
func gitDecode(params json.RawMessage, v any) error
func gitResolveRepo(d *Deps, workspaceID, projectID string) (gitdata.Repo, error)
```
Di `cmd/shikigamictl/cmd_git.go`:
```go
type gitSubFunc func(ctx context.Context, c *ipc.Client, args []string) error
var gitSubcommands = map[string]gitSubFunc{}
func registerGitSub(name string, fn gitSubFunc)  // panic bila name == "", fn == nil, atau nama ganda
func gitUsage() string
func runGit(ctx context.Context, c *ipc.Client, args []string) error
func gitRepos(ctx context.Context, c *ipc.Client, args []string) error
```
Nama-nama ini TIDAK boleh bentrok dengan helper Lark (`larkStore`, `larkErr`, `mapLarkStoreErr`, `larkDecode`, `publishLarkChanged`, `ticketsErr`, `ticketsMapErr`, `larkSubFunc`, `larkSubcommands`, `registerLarkSub`, `larkUsage`, `runLark`). Sudah diperiksa: tidak ada identifier berawalan `git...`/`registerGit...`/`runGit` di `cmd/`.

Kontrak IPC: metode `git.repos.list`, params `{"workspaceId": string}`, hasil JSON array `Repo[]` (bentuk `Repo` di overview 2.3; `[]` bila kosong, tidak pernah `null`).

Kontrak galat: `ipc.NewError(ipcCode, "[<code>] <pesan Indonesia>")`.
- `invalid_params` (`ipc.ErrInvalid`): params tidak bisa didecode ("Parameter tidak valid") atau `workspaceId` kosong ("workspaceId wajib diisi").
- `not_found` (`ipc.ErrNotFound`): workspace tidak ada ("Workspace tidak ditemukan"); juga `gitResolveRepo` bila project tidak ada.
- galat `*gitdata.Error`: kode = `string(e.Code)`, pesan = `e.Message`; pemetaan kode ipc: `not_found` -> `ipc.ErrNotFound`, `not_allowed` -> `ipc.ErrInvalid`, sisanya (`cli_missing`, `not_logged_in`, `permission`, `timeout`, `cli_error`) -> `ipc.ErrInternal`.
- galat tak terklasifikasi -> `ipc.NewError(ipc.ErrInternal, err.Error())`.

## Langkah

### 1. `cmd/shikigamid/wire_git.go`
`package main`. Impor: `bytes`, `context`, `encoding/json`, `errors`, `strings`, `github.com/NooBeeID/shikigami/internal/gitdata`, `github.com/NooBeeID/shikigami/internal/ipc`, `github.com/NooBeeID/shikigami/internal/workspace`. Satu-satunya file ini; jangan memuat handler tiket lain.

1. `init()`:
   ```go
   func init() {
       RegisterModule("git", func(srv *ipc.Server, d *Deps) { registerGitRepos(srv, d) })
   }
   ```
2. `gitStore(d *Deps) (*workspace.Store, error)`: tiru `larkStore`: `v, ok := d.Lookup("workspace.store")`; type-assert ke `*workspace.Store`; bila `!ok`, bukan store, atau nil -> `ipc.NewError(ipc.ErrInternal, "workspace store tidak tersedia")`.
3. `gitErr(code string, ipcCode int, msg string) error { return ipc.NewError(ipcCode, "["+code+"] "+msg) }`.
4. `mapGitError(err error) error`:
   - `nil` -> `nil`.
   - bila `errors.As(err, &ie)` dengan `ie *ipc.Error` (sudah galat ipc) -> kembalikan apa adanya.
   - bila `e, ok := gitdata.AsError(err); ok`: tentukan `ipcCode` lewat `switch e.Code` (`gitdata.CodeNotFound` -> `ipc.ErrNotFound`; `gitdata.CodeNotAllowed` -> `ipc.ErrInvalid`; default `ipc.ErrInternal`); kembalikan `gitErr(string(e.Code), ipcCode, e.Message)`.
   - selain itu -> `ipc.NewError(ipc.ErrInternal, err.Error())`.
5. `gitDecode(params json.RawMessage, v any) error`: tiru `larkDecode`: `t := bytes.TrimSpace(params)`; kosong atau `null` -> `{}`; `json.Unmarshal(t, v)` gagal -> `gitErr("invalid_params", ipc.ErrInvalid, "Parameter tidak valid")`.
6. Fungsi privat `gitDetect(ctx context.Context, d *Deps, workspaceID string) ([]gitdata.Repo, error)` (boleh dipakai `gitResolveRepo` dan handler):
   - `strings.TrimSpace(workspaceID) == ""` -> `gitErr("invalid_params", ipc.ErrInvalid, "workspaceId wajib diisi")`.
   - `st, err := gitStore(d)`; galat -> kembalikan.
   - `ws, err := st.Get(workspaceID)`; bila `errors.Is(err, workspace.ErrNotFound)` -> `gitErr("not_found", ipc.ErrNotFound, "Workspace tidak ditemukan")`; galat lain -> `ipc.NewError(ipc.ErrInternal, err.Error())`. (Jangan memakai `mapLarkStoreErr`.)
   - Bangun `inputs := make([]gitdata.ProjectInput, 0, len(ws.Projects))` dari `ws.Projects` (`ID`, `Name`, `Path`).
   - `repos, err := gitdata.DetectRepos(ctx, inputs)`; galat -> `mapGitError(err)`.
   - Bila `repos == nil` -> `repos = []gitdata.Repo{}`. Kembalikan.
7. `gitResolveRepo(d *Deps, workspaceID, projectID string) (gitdata.Repo, error)`: tidak menerima ctx (signature kontrak), gunakan `context.Background()` ke `gitDetect`. Setelah dapat `repos`, cari repo dengan `r.ProjectID == projectID` atau ada `p.ID == projectID` di `r.Projects`; ketemu -> kembalikan. Tidak ketemu (termasuk `projectID` kosong) -> `gitdata.Repo{}, gitErr("not_found", ipc.ErrNotFound, "Project tidak ditemukan atau tidak punya remote dikenal")`. Catatan: handler tiket lain yang punya `ctx` boleh memanggil `gitDetect` sendiri; signature `gitResolveRepo` tetap persis kontrak.
8. `registerGitRepos(srv *ipc.Server, d *Deps)`:
   ```go
   srv.Register("git.repos.list", func(ctx context.Context, params json.RawMessage) (any, error) {
       var p struct { WorkspaceID string `json:"workspaceId"` }
       if err := gitDecode(params, &p); err != nil { return nil, err }
       repos, err := gitDetect(ctx, d, p.WorkspaceID)
       if err != nil { return nil, err }
       return repos, nil
   })
   ```
   Hasil selalu slice non-nil sehingga JSON `[]`.

### 2. `cmd/shikigamictl/cmd_git.go`
`package main`. Impor: `context`, `errors`, `flag`, `fmt`, `io`, `sort`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`.

1. `type gitSubFunc func(ctx context.Context, c *ipc.Client, args []string) error` dan `var gitSubcommands = map[string]gitSubFunc{}`.
2. `registerGitSub(name string, fn gitSubFunc)`: tiru `registerLarkSub` persis: panic bila `name == ""`, `fn == nil`, atau nama sudah ada di peta (`fmt.Sprintf("subcommand git %q sudah terdaftar", name)`).
3. `gitUsage() string`: kumpulkan nama dari `gitSubcommands`, `sort.Strings`, kembalikan `"penggunaan: shikigamictl git " + strings.Join(names, "|")`.
4. `runGit(ctx, c, args)`: `len(args) == 0` -> `errors.New(gitUsage())`; subcommand tak dikenal -> `errors.New(gitUsage())`; selain itu `fn(ctx, c, args[1:])`. Tidak boleh panic pada input apa pun.
5. `gitRepos(_ context.Context, c *ipc.Client, args []string) error`:
   - `fs := flag.NewFlagSet("git repos", flag.ContinueOnError)`; `fs.SetOutput(io.Discard)`; `ws := fs.String("workspace", "", "")`.
   - `fs.Parse(args)` galat -> `fmt.Errorf("%v\n%s", err, gitReposUsage)`.
   - `fs.NArg() != 0` atau `strings.TrimSpace(*ws) == ""` -> `fmt.Errorf("--workspace wajib\n%s", gitReposUsage)` (untuk NArg != 0 pakai pesan `argumen tak dikenal`).
   - `return callAndPrint(c, "git.repos.list", map[string]any{"workspaceId": strings.TrimSpace(*ws)})`.
   - Konstanta `gitReposUsage = "penggunaan: shikigamictl git repos --workspace <id>"`.
6. `init()`:
   ```go
   func init() {
       RegisterCommand(Command{Name: "git", Summary: "Connector Git: repo, MR, pipeline", Run: runGit})
       registerGitSub("repos", gitRepos)
   }
   ```
   Urutan inisialisasi antar file `init()` dalam paket tidak berpengaruh karena peta `gitSubcommands` diinisialisasi sebagai variabel paket (sebelum semua `init()`). Tiket 02 sampai 04 memanggil `registerGitSub` dari `init()` file mereka sendiri; JANGAN mendaftarkan `mrs`, `mr`, `pipelines`, `pipeline` di sini.

### 3. Guard
Jalankan `gofmt -l cmd/shikigamid/wire_git.go cmd/shikigamictl/cmd_git.go` (harus kosong) dan `go vet ./cmd/shikigamid ./cmd/shikigamictl` bila paket bisa dibangun, lalu `go build ./...` dari root repo. Bila `internal/gitdata` belum ada di cabang kerja, `go build` akan gagal hanya pada impor `internal/gitdata` di `wire_git.go`: laporkan itu apa adanya (jangan membuat stub). `cmd/shikigamictl` tidak mengimpor `gitdata`, jadi `go build ./cmd/shikigamictl` harus lulus sendiri.

## Perubahan
- Dibuat: `cmd/shikigamid/wire_git.go`
- Dibuat: `cmd/shikigamictl/cmd_git.go`
- Tidak ada berkas lain yang diubah.

## Wiring
Yang diekspos untuk langkah Wiring (JANGAN disambungkan sendiri; tidak ada file pusat yang diubah):
- Daemon: `init()` di `wire_git.go` mendaftar `RegisterModule("git", ...)` otomatis; Wiring cukup memverifikasi daemon memanggil modul itu saat start dan `shikigamictl git repos --workspace <id>` mengembalikan hasil.
- CLI: `init()` di `cmd_git.go` mendaftar command `git` lewat `RegisterCommand`; Wiring memverifikasi `shikigamictl` (bantuan) menampilkan `git`, dan `shikigamictl git` tanpa argumen menampilkan `penggunaan: shikigamictl git repos`.
- Untuk tiket 02 sampai 04 (di luar tiket ini): helper `gitStore`, `gitErr`, `mapGitError`, `gitDecode`, `gitResolveRepo`, serta `registerGitSub`/`gitSubFunc`.
- Frontend memakai nama IPC `git.repos.list` dan bentuk `Repo[]` (tidak ada keterkaitan kode).

## Selesai bila
- [ ] `cmd/shikigamid/wire_git.go` dan `cmd/shikigamictl/cmd_git.go` ada; tidak ada berkas lain yang berubah (`git status` hanya menunjukkan dua berkas ini baru, ditambah berkas fitur lain bila ada di worktree).
- [ ] `git.repos.list` terdaftar lewat `RegisterModule("git", ...)` dari `init()` dan mengembalikan `[]` (bukan `null`) bila tidak ada repo.
- [ ] Workspace tidak ada -> `[not_found] Workspace tidak ditemukan`; params salah -> `[invalid_params] Parameter tidak valid`; `workspaceId` kosong -> `[invalid_params] workspaceId wajib diisi`; `*gitdata.Error` terpetakan ke `[<code>] <pesan>`.
- [ ] Kelima helper ada dengan signature persis kontrak dan tidak bentrok nama di paket `main`.
- [ ] `registerGitSub` panic untuk nama kosong atau ganda; `runGit` tanpa argumen atau subcommand tak dikenal mengembalikan galat penggunaan (bukan panic) dengan daftar subcommand terurut; `repos` tanpa `--workspace` mengembalikan galat penggunaan.
- [ ] Guard: `gofmt -l` kosong untuk dua berkas; `go build ./cmd/shikigamictl` lulus; `go build ./...` lulus (bila `internal/gitdata` sudah ada; bila belum, dilaporkan sebagai kegagalan karena paket belum ada, bukan kesalahan fitur ini). Tanpa unit test.
