# Overview: git-connector-ticket-01-git-repos (Connector Git dan deteksi repo)

Sumber: tiket `docs/tickets/git-connector-ticket-01-git-repos.md` dan spec `docs/superpowers/specs/2026-11-22-10-git-connector-design.md`. Dokumen ini adalah sumber kebenaran untuk kontrak bersama; plan per fitur hanya merinci langkah kerja. Module Go: `github.com/NooBeeID/shikigami`. Bila ada selisih antara overview ini dan tiket, tiket menang kecuali dinyatakan lain di sini.

Keadaan kode saat ini (diverifikasi):
- `internal/connector/cli.go` SUDAH memiliki `RunCLITimeout(ctx, timeout, name, args...) (string, error)` (dari tiket Lark). Jadi `gitdata.Run` memakainya; tidak perlu timeout buatan sendiri dan `cli.go` TIDAK boleh diubah.
- Pesan galat `RunCLITimeout`: `ErrCLIMissing: <name>` (cek dengan `connector.IsCLIMissing`), `perintah <name> melewati batas waktu`, `perintah <name> gagal: <stderr terpotong 300 karakter>`. Klasifikasi `gitdata` mencocokkan substring dari pesan ini (pola `internal/larkdata/errors.go`).
- Pola acuan: `internal/connector/lark.go`, `internal/larkdata/{runner,allowlist,errors}.go`, `cmd/shikigamid/wire_lark.go`, `cmd/shikigamictl/cmd_lark.go`, `frontend/src/modules/lark/*`, `docs/lark/cli-findings.md`.
- `frontend/src/modules/layout/model.ts` memetakan field leaf secara EKSPLISIT di tiga tempat: interface `Leaf`, interface `NewLeaf`, fungsi pembuat leaf dari `NewLeaf` (`if (nl.sourceId !== undefined) leaf.sourceId = ...`), dan `hydrateNode` (daftar `for (const k of [...] as const)` hanya untuk string). Field baru `number` bertipe angka sehingga `hydrateNode` butuh cabang `typeof r.number === 'number'` terpisah; `pipelineId` string boleh masuk daftar string.
- Folder `docs/git/` belum ada; `internal/gitdata/` belum ada; `frontend/src/modules/git/` belum ada.

## 1. Ringkasan epic dan urutan eksekusi

Epic mendaftarkan dua connector (`gitlab` dengan CLI `glab`, `github` dengan CLI `gh`) di kerangka Connector yang ada; status dibaca dari CLI yang sudah login, tanpa token di app. Epic membangun paket baca-saja `internal/gitdata` (tipe ternormalisasi, allowlist, runner, klasifikasi galat, deteksi repo dari remote git tiap project), spike CLI yang menghasilkan fixture dan `docs/git/cli-findings.md`, IPC `git.repos.list`, subcommand `shikigamictl git repos`, serta frontend `modules/git` (item rail per provider, `GitPanel`, `ProjectSection`, placeholder daftar dan pane) sebagai titik ekstensi untuk tiket 02 sampai 04.

Urutan:
1. **Foundation**: TIDAK ada (`hasFoundation=false`, tidak ada `00-foundation.md`). Tidak ada kode bersama yang harus ada lebih dulu di luar kepemilikan fitur: setiap fitur memiliki berkasnya sendiri, dan kontrak antar fitur adalah teks (signature di bagian 2). Pengecualian lunak ada di bagian 1.1.
2. **Fitur paralel** (5 fitur, bagian 3): `git-cli-spike`, `git-connector-adapters`, `gitdata-foundation`, `git-repos-ipc-ctl`, `git-rail-panel-ui`.
3. **Wiring**: fitur `git-repos-wiring` dari tiket (tanpa ownedPaths) dikerjakan sebagai fase Wiring, bukan fitur paralel (bagian 5).

### 1.1 Ketergantungan lunak antar fitur (urutan kerja yang disarankan)
- `git-cli-spike` idealnya selesai PERTAMA: fixture dan temuan menentukan flag allowlist, pola galat, bentuk `Status()` adapter, dan field opsional tipe. Fitur lain memulai dari nilai awal di bagian 2 dan menyelaraskan dengan `docs/git/cli-findings.md` bila berkas itu sudah ada saat mereka bekerja; bila belum, pakai nilai awal dan biarkan Wiring menyelaraskan (bagian 5, butir 6).
- `git-repos-ipc-ctl` memakai `gitdata.DetectRepos`, `gitdata.Repo`, `gitdata.ProjectInput`, `gitdata.Error`/`ErrorCode`, `gitdata.AsError`. Agent ini menulis kodenya terhadap signature di bagian 2. `go build ./...` baru lulus utuh setelah `gitdata-foundation` ada; bila `internal/gitdata` belum ada di cabang kerja saat agent ini membangun, agent TIDAK membuat stub di `internal/gitdata` (bukan miliknya); ia melaporkan build gagal hanya karena paket belum ada, dan guard diulang di Wiring.
- `git-rail-panel-ui` hanya bergantung pada bentuk JSON `Repo` dan nama IPC `git.repos.list` (bagian 2); tidak bergantung pada fitur Go.
- `git-connector-adapters` hanya memakai `connector.RunCLI` yang sudah ada; independen dari `gitdata`.

## 2. Kontrak bersama

### 2.1 Konvensi penamaan
- Go: package `connector` (adapter), `gitdata`, `main` (`cmd/shikigamid`, `cmd/shikigamictl`). Identifier Go Inggris; pesan ke user Indonesia.
- JSON ke frontend: camelCase. Kode galat: snake_case persis nilai `ErrorCode`.
- Provider: string `"gitlab"` | `"github"` (sama dengan ID connector dan sufiks id rail).
- Frontend: folder `frontend/src/modules/git/`; id rail `git-gitlab` (order 20) dan `git-github` (order 21); pane kind `git-mr` dan `git-pipeline`; field leaf `provider`, `projectId` (sudah ada), `number` (baru, number), `pipelineId` (baru, string). Nama `number` dan `pipelineId` adalah kontrak untuk tiket 02 sampai 04.
- Teks UI Indonesia; identifier kode Inggris.

### 2.2 Go: `internal/connector/gitlab.go` dan `github.go` (milik `git-connector-adapters`)
```go
type gitlabConnector struct{}
func init() { Register(gitlabConnector{}) }
func (gitlabConnector) ID() string   // "gitlab"
func (gitlabConnector) Name() string // "GitLab"
func (gitlabConnector) Status(ctx context.Context) (Status, error)
func (gitlabConnector) ConnectGuide() Guide

type githubConnector struct{}   // ID "github", Name "GitHub"; bentuk sama
```
- `Status` memanggil `RunCLI(ctx, "glab", "auth", "status")` / `RunCLI(ctx, "gh", "auth", "status")`.
- Pemetaan: `IsCLIMissing(err)` -> `StateCLIMissing` (Message "glab belum terpasang" / "gh belum terpasang"); tidak ada host login (atau keluaran tak terparse, tanpa panic) -> `StateNotLoggedIn` (Message Indonesia); ada host login -> `StateConnected` dengan `User` = nama akun (host utama `gitlab.com`/`github.com` bila ada, selain itu host pertama). Galat lain dikembalikan sebagai `error`, kecuali spike menunjukkan exit non-nol berarti belum login (ikuti temuan). Perhatikan: `glab`/`gh auth status` bisa exit non-nol saat belum login; `RunCLI` mengembalikan galat `perintah ... gagal: <stderr>` dan stderr berisi teks status, jadi parsing harus mempertimbangkan teks pada galat itu.
- Tidak memuat output mentah CLI atau token; tanpa cache; jangan pernah memakai `--show-token`.
- `ConnectGuide`: `Steps` Indonesia (pasang CLI bila belum ada, jalankan login, klik Cek ulang), `Commands: []string{"glab auth login"}` / `[]string{"gh auth login"}`.
- Parser daftar host login adalah milik adapter (privat di `connector`); `gitdata/repo.go` punya parser sendiri untuk host (tidak mengimpor privat adapter). Kedua parser mengikuti format yang sama dari `docs/git/cli-findings.md`.

### 2.3 Go: paket `internal/gitdata` (milik `gitdata-foundation`)
Impor hanya stdlib dan `internal/connector`. TIDAK mengimpor `internal/workspace`. Arah dependensi: `gitdata -> connector`. Berkas: `doc.go`, `types.go`, `allowlist.go`, `runner.go`, `errors.go`, `repo.go` (folder `testdata/` milik spike). Tiket 02 sampai 04 menambah `mrs.go`, `pipelines.go`, `mrdetail.go` sendiri; jadi paket harus tetap kompilasi bila berkas itu ditambah (jangan memakai nama privat umum yang rawan bentrok: awali helper privat dengan makna jelas, mis. `classify`, `check`, `parseRemoteURL`).

`types.go` (tag json camelCase; field opsional `omitempty`):
```go
type Provider string
const ( ProviderGitLab Provider = "gitlab"; ProviderGitHub Provider = "github" )

type RepoProject struct { ID string `json:"id"`; Name string `json:"name"` }

type Repo struct {
    ProjectID   string        `json:"projectId"`
    ProjectName string        `json:"projectName"`
    Projects    []RepoProject `json:"projects"`
    Provider    Provider      `json:"provider"`
    Host        string        `json:"host"`
    Path        string        `json:"path"`
    WebURL      string        `json:"webUrl"`
}

type Status string
const (
    StatusPending Status = "pending"; StatusRunning Status = "running"; StatusSuccess Status = "success"
    StatusFailed Status = "failed"; StatusCanceled Status = "canceled"; StatusSkipped Status = "skipped"
    StatusUnknown Status = "unknown"
)

type MergeRequest struct {
    Number int `json:"number"`; Title string `json:"title"`; Author string `json:"author"`
    SourceBranch string `json:"sourceBranch"`; TargetBranch string `json:"targetBranch"`
    State string `json:"state"` // "open"|"merged"|"closed"
    Draft bool `json:"draft"`; ReviewState string `json:"reviewState,omitempty"`
    PipelineStatus Status `json:"pipelineStatus,omitempty"`
    UpdatedAt string `json:"updatedAt"` // RFC3339
    URL string `json:"url"`
}
type Reviewer struct { Name string `json:"name"`; State string `json:"state,omitempty"` }
type PipelineRef struct { ID string `json:"id"`; Status Status `json:"status"`; URL string `json:"url,omitempty"` }
type MergeRequestDetail struct {
    MergeRequest
    Description string `json:"description"`; Labels []string `json:"labels"`; Reviewers []Reviewer `json:"reviewers"`
    Mergeable string `json:"mergeable,omitempty"`; LastPipeline *PipelineRef `json:"lastPipeline,omitempty"`
}
type Pipeline struct {
    ID string `json:"id"`; Ref string `json:"ref"`; SHA string `json:"sha"`; Status Status `json:"status"`
    Author string `json:"author,omitempty"`; DurationSec int `json:"durationSec"`
    UpdatedAt string `json:"updatedAt"`; URL string `json:"url"`
}
type Job struct { Name string `json:"name"`; Status Status `json:"status"`; DurationSec int `json:"durationSec"`; URL string `json:"url,omitempty"` }
type Stage struct { Name string `json:"name"`; Jobs []Job `json:"jobs"` }
type PipelineDetail struct { Pipeline; Stages []Stage `json:"stages"` }
```
Nama field dan nama tag di atas TIDAK boleh diubah; hanya field opsional (`omitempty`, `Mergeable`, dst.) boleh disesuaikan bentuknya mengikuti `docs/git/cli-findings.md`, dan perubahan dicatat di temuan.

Fungsi pemetaan status (di `types.go`):
```go
func NormalizeGitLabStatus(s string) Status
func NormalizeGitHubStatus(status, conclusion string) Status
```
Nilai tak dikenal -> `unknown`; GitLab `manual`/`scheduled`/`created`/`waiting_for_resource`/`preparing`/`pending` -> `pending`; `running` -> `running`; `success` -> `success`; `failed` -> `failed`; `canceled` -> `canceled`; `skipped` -> `skipped`. GitHub: `queued`/`waiting`/`requested`/`pending` -> `pending`; `in_progress` -> `running`; `completed` dengan conclusion `success` -> `success`, `failure`/`timed_out`/`startup_failure` -> `failed`, `cancelled`/`canceled` -> `canceled`, `skipped`/`neutral` -> `skipped`; selain itu `unknown`. Sesuaikan final dengan temuan spike.

`errors.go`:
```go
type ErrorCode string
const (
    CodeCLIMissing  ErrorCode = "cli_missing"
    CodeNotLoggedIn ErrorCode = "not_logged_in"
    CodePermission  ErrorCode = "permission"
    CodeNotFound    ErrorCode = "not_found"
    CodeTimeout     ErrorCode = "timeout"
    CodeNotAllowed  ErrorCode = "not_allowed"
    CodeCLI         ErrorCode = "cli_error"
)
type Error struct { Code ErrorCode; Message string } // Message Indonesia
func (e *Error) Error() string
func AsError(err error) (*Error, bool)
```
Pesan: `cli_missing` ("glab belum terpasang" / "gh belum terpasang" / "git belum terpasang"); `not_logged_in` ("Belum login ke GitLab/GitHub. Jalankan `glab auth login`/`gh auth login` di Pengaturan > Connector"); `permission` ("Izin kurang: <pesan CLI>"); `not_found` ("Repo tidak ditemukan atau tidak ada akses"); `timeout` ("GitLab/GitHub tidak merespons dalam 20 detik"); `cli_error` (pesan CLI diringkas).

`runner.go` dan `allowlist.go`:
```go
const CallTimeout = 20 * time.Second
const MaxConcurrent = 3

// Run menjalankan satu perintah baca. tool = "glab" | "gh" | "git". Mengembalikan stdout.
// Global dibatasi semaphore MaxConcurrent; ctx batal saat menunggu giliran -> galat terklasifikasi (timeout).
// Perintah di luar allowlist ditolak dengan CodeNotAllowed SEBELUM proses dijalankan.
func Run(ctx context.Context, tool string, args ...string) ([]byte, error)
```
- Memakai `connector.RunCLITimeout(ctx, CallTimeout, tool, args...)`; tidak mengubah direktori kerja untuk `glab`/`gh` (selalu `-R`); `git` memakai `-C <path>`.
- Allowlist eksplisit (tool + subcommand + flag yang diizinkan): `glab auth status`, `gh auth status`, `glab mr list`, `glab mr view`, `glab ci list` / `glab pipeline list` / `glab ci get` (nama final dari spike; sampai ada temuan, izinkan ketiganya), `gh pr list`, `gh pr view`, `gh run list`, `gh run view`, `git remote get-url`, `git remote` (hanya daftar nama). Flag baca saja (`-R/--repo`, `-F/--output json`, `--json`, `--limit`, `-P/--per-page`, `--state`/`-s`/`--merged`/`--closed`/`--all`, `--hostname`, dan sejenis dari spike). Tolak flag `--show-token`, flag berisiko tulis, dan semua subcommand di luar tabel (`mr create|merge|approve|close`, `ci run|retry|cancel`, `pr create|merge|review|close`, `run rerun|cancel`, `api`, dll.). Untuk `git`, argumen `-C <path>` di depan diterima lalu `remote` / `remote get-url <nama>`.
- Tiket 02 sampai 04 memakai allowlist ini tanpa mengubahnya, jadi tabel harus mencakup semua perintah baca di atas beserta flag yang mereka perlukan (termasuk flag filter state MR/PR dan flag batas jumlah).

`repo.go`:
```go
type ProjectInput struct { ID, Name, Path string }
func DetectRepos(ctx context.Context, projects []ProjectInput) ([]Repo, error)
func RepoFlag(r Repo) string // host bawaan -> PATH; self-hosted -> HOST/PATH
```
- Remote: `git -C <path> remote` -> pakai `origin` bila ada, jika tidak remote pertama; lalu `git -C <path> remote get-url <nama>`. Project tanpa remote atau path tak ada diabaikan diam-diam.
- Urai URL: `git@host:owner/repo.git`, `ssh://git@host[:port]/owner/repo.git`, `https://[user@]host/owner/repo[.git]`; buang `.git`, slash ujung, kredensial, port; subgroup tidak dipotong.
- Provider: `github.com` -> github; `gitlab.com` -> gitlab (tanpa cek login). Host lain: dikenali bila `glab` login ke host itu (GitLab diutamakan) atau `gh` login ke host itu; daftar host login di-cache memori TTL sekitar 60 detik (mutex). Tidak cocok -> diabaikan diam-diam.
- Penggabungan: kunci `host + "/" + path`; project berbagi remote digabung, `Projects` urut input, `ProjectID/ProjectName` = project pertama; urutan hasil stabil mengikuti project pertama tiap repo. `WebURL = "https://" + host + "/" + path`.
- Pemanggilan git per project lewat `Run` (semaphore sama); galat satu project tidak menggagalkan project lain. `DetectRepos` mengembalikan slice kosong non-nil bila tidak ada hasil.

### 2.4 Go: IPC dan helper `cmd/shikigamid/wire_git.go` (milik `git-repos-ipc-ctl`)
Registrasi mandiri: `func init() { RegisterModule("git", func(srv *ipc.Server, d *Deps) { registerGitRepos(srv, d) }) }`.
- Metode IPC `git.repos.list`, params `{workspaceId: string}` -> `Repo[]` (array kosong `[]`, bukan null).
- Galat: `ipc.NewError(ipcCode, "[<code>] <pesan Indonesia>")`; kode dari `gitdata.ErrorCode` untuk galat gitdata; `not_found` (workspace tidak ada) dan `invalid_params` (workspaceId kosong / params salah).
- Helper bersama untuk file wire_git_* tiket lain (package `main`, nama WAJIB persis dan tidak boleh bentrok dengan `larkStore`/`larkErr`/`mapLarkStoreErr`/helper decode Lark):
```go
func gitStore(d *Deps) (*workspace.Store, error)                  // d.Lookup("workspace.store")
func gitErr(code string, ipcCode int, msg string) error           // ipc.NewError(ipcCode, "["+code+"] "+msg)
func mapGitError(err error) error                                 // *gitdata.Error -> gitErr; lainnya -> internal
func gitDecode(params json.RawMessage, v any) error               // decode params; salah -> invalid_params (periksa signature decode di wire_lark.go dan ikuti bentuknya)
func gitResolveRepo(d *Deps, workspaceID, projectID string) (gitdata.Repo, error) // deteksi lalu cari repo yang memuat projectID; tidak ada -> not_found "Project tidak ditemukan atau tidak punya remote dikenal"
```
- Berkas ini TIDAK memuat handler tiket lain.

### 2.5 Go: CLI debug `cmd/shikigamictl/cmd_git.go` (milik `git-repos-ipc-ctl`)
- `init()`: `RegisterCommand(Command{Name: "git", Summary: "Connector Git: repo, MR, pipeline", Run: runGit})`.
- `var gitSubcommands = map[string]func(...)` dan `func registerGitSub(name string, fn ...)` (panic bila nama kosong atau ganda; tiru `registerLarkSub` di `cmd_lark.go`, termasuk signature fungsi subcommand yang dipakai di sana). Tiket 02 sampai 04 memanggil `registerGitSub` dari `init()` file mereka sendiri.
- Subcommand `repos`: `--workspace <id>` wajib (kurang -> galat penggunaan); memanggil `callAndPrint(c, "git.repos.list", map[string]any{"workspaceId": id})`.
- Tanpa argumen atau subcommand tak dikenal -> galat penggunaan berisi daftar subcommand terurut, bukan panic.
- `shikigamictl connector status gitlab|github` sudah ditangani `cmd_connector.go` (tidak diubah).

### 2.6 Frontend `frontend/src/modules/git/` (milik `git-rail-panel-ui`)
`client.ts`:
```ts
export type Provider = 'gitlab' | 'github';
export interface RepoProject { id: string; name: string }
export interface Repo { projectId: string; projectName: string; projects: RepoProject[]; provider: Provider; host: string; path: string; webUrl: string }
export function listRepos(workspaceId: string): Promise<Repo[]>      // IPC 'git.repos.list' {workspaceId}
export function parseGitError(x: unknown): { code: string; message: string } // kode dari awalan "[code] " (pola parseLarkError)
```
`store.ts`: `gitRepos` (writable `Repo[]`), `gitState` (`{status: 'loading'|'ready'|'error', code?: string, message?: string}`), `refreshRepos(): Promise<void>` (penjaga urutan `seq`, hasil basi dibuang), `repoKey(r)` = `r.host + '/' + r.path`, `reposFor(provider)` (derived), `collapsed` (peta repoKey -> boolean; bagian pertama terbuka default), `selectedState` (peta repoKey -> 'open'|'merged'|'closed', default `open`; dipakai tiket 02). Pemicu deteksi: inisialisasi modul, `activeWorkspaceId` berganti (kosongkan repo + `loading`), event `workspace.changed` dengan `kind` `projectAdded`/`projectRemoved` (dan `deleted`/`configChanged` bila memengaruhi workspace aktif), tombol Segarkan. Ikuti `modules/lark/store.ts`.
`actions.ts`: `openInBrowser(url)`, `copyLink(url)`, `sendToAgent(url)`; signature `(url: string) => void | Promise<void>`; ikuti `modules/lark/actions.ts`. `sendToAgent`: `pasteToSession` ke sesi aktif, URL + satu spasi, tanpa Enter; tanpa sesi aktif toast "Tidak ada sesi aktif".
Komponen: `GitIcon.svelte` (prop `provider?`), `GitPanel.svelte` (prop `provider`), `ProjectSection.svelte` (props `{provider, repo}`), `MrList.svelte` dan `PipelineList.svelte` (props `{provider, repo}` TETAP; placeholder "Segera hadir"), `MrPane.svelte` dan `PipelinePane.svelte` (placeholder; menerima leaf, tanpa IPC).
`index.ts`: `registerRailItem` `git-gitlab` (order 20) dan `git-github` (order 21) dengan `visible` = derived (ada repo provider itu), `registerPaneKind` untuk `git-mr` dan `git-pipeline`. Dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts', { eager: true })` di `main.ts`. Acuan bentuk: `modules/lark/index.ts`.
`modules/layout/model.ts`: `PaneKind` ditambah `'git-mr' | 'git-pipeline'`; `Leaf` dan `NewLeaf` mendapat `number?: number` dan `pipelineId?: string`; pembuat leaf menyalin keduanya; `hydrateNode` memulihkan `pipelineId` (string) dan `number` (cek `typeof === 'number'`); layout lama tetap termuat; tidak mengubah fungsi lain.

### 2.7 Artefak spike (milik `git-cli-spike`)
- `internal/gitdata/testdata/`: `glab-mr-list.json`, `glab-mr-view.json`, `glab-pipeline-list.json`, `glab-pipeline-get.json`, `glab-auth-status.txt` (atau `.json`), `gh-pr-list.json`, `gh-pr-view.json`, `gh-run-list.json`, `gh-run-view.json`, `gh-auth-status.txt`, `README.md`.
- `docs/git/cli-findings.md` (Indonesia): versi CLI, tabel perintah, skema ringkas, pemetaan status, host login, pola galat, remote git, bagian "Pengecualian dan pemotongan fitur".
- Fixture teranonimkan (tanpa token, email, nama user/grup/repo asli, isi MR/pipeline asli); fixture GitHub ditandai ASUMSI. Hanya perintah baca. Tidak ada perubahan Go atau frontend.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `git-cli-spike` | Jalankan `glab` nyata (read-only), rekam dan anonimkan fixture, tulis `docs/git/cli-findings.md` (jawab 4 pertanyaan spike, flag final, pemetaan status, host login, pola galat, remote git, pemotongan fitur). Fixture GitHub dari dokumentasi, ditandai asumsi. | `docs/git/cli-findings.md`, `internal/gitdata/testdata/` |
| `git-connector-adapters` | Adapter `gitlab` dan `github` di `internal/connector` (Status, ConnectGuide) sesuai 2.2. | `internal/connector/gitlab.go`, `internal/connector/github.go` |
| `gitdata-foundation` | Paket `internal/gitdata`: tipe, normalisasi status, allowlist, runner dengan semaphore, galat terklasifikasi, deteksi repo, `RepoFlag` sesuai 2.3. | `internal/gitdata/doc.go`, `types.go`, `allowlist.go`, `runner.go`, `errors.go`, `repo.go` |
| `git-repos-ipc-ctl` | `wire_git.go` (IPC `git.repos.list` + helper bersama) dan `cmd_git.go` (`shikigamictl git repos`, `registerGitSub`) sesuai 2.4 dan 2.5. | `cmd/shikigamid/wire_git.go`, `cmd/shikigamictl/cmd_git.go` |
| `git-rail-panel-ui` | Frontend `modules/git` (client, store, actions, ikon, panel, section, placeholder, registrasi rail dan pane kind) dan perluasan `model.ts` sesuai 2.6. | `frontend/src/modules/git/index.ts`, `store.ts`, `client.ts`, `actions.ts`, `GitIcon.svelte`, `GitPanel.svelte`, `ProjectSection.svelte`, `MrList.svelte`, `PipelineList.svelte`, `MrPane.svelte`, `PipelinePane.svelte`, `frontend/src/modules/layout/model.ts` |

Fitur tiket `git-repos-wiring` (tanpa ownedPaths) tidak masuk daftar paralel; ia identik dengan fase Wiring (bagian 5).

## 4. Foundation

Tidak ada. Alasan: tidak ada skeleton atau tipe yang harus ada sebelum fitur paralel karena (a) tipe bersama dimiliki `gitdata-foundation` dan frontend memakai tipe TS sendiri di `client.ts`; (b) `git-repos-ipc-ctl` bergantung pada signature teks di 2.3, bukan pada kode yang sudah ada; (c) membuat stub di `internal/gitdata` oleh agent lain melanggar kepemilikan path. Konsekuensinya dicatat di bagian 1.1 (build `git-repos-ipc-ctl` bisa gagal sementara hanya karena `gitdata` belum ada). Tidak ada `00-foundation.md`.

## 5. Wiring checklist

Dikerjakan setelah semua fitur selesai (fitur `git-repos-wiring`). Hanya menyentuh titik impor/registrasi yang sudah ada; bila perlu mengubah file di luar ownedPaths epic, laporkan alasannya, jangan ubah diam-diam.
1. Adapter: `gitlab.go` dan `github.go` terdaftar lewat `init()`; pastikan paket `internal/connector` termuat di binary `shikigamid` (sama seperti `fake`/`lark`), sehingga `connector.list/status/guide` dan Pengaturan > Connector memuat GitLab dan GitHub dengan dialog Connect yang benar (`glab auth login` / `gh auth login`).
2. IPC: `wire_git.go` mendaftar lewat `RegisterModule("git", ...)` dari `init()`; tidak ada file registrasi pusat yang diubah. Cek `shikigamictl git repos --workspace <id>` mengembalikan hasil (daemon benar-benar memanggil modul saat start).
3. CLI: `cmd_git.go` terdaftar lewat `RegisterCommand` dari `init()`; `shikigamictl` (bantuan) menampilkan `git`; `shikigamictl git` tanpa argumen menampilkan penggunaan; tidak mengubah `main.go`/`registry.go`.
4. Frontend: `modules/git/index.ts` dimuat otomatis oleh `import.meta.glob` di `main.ts`; dua rail item dan dua pane kind terdaftar; `PaneKind` diperluas di `model.ts`. Tidak perlu impor tambahan.
5. Verifikasi akhir: `go build ./...` dan build frontend; jalankan app. Workspace dengan project remote `gitlab.com` (`glab` login): item GitLab muncul, panel menampilkan bagian per project dengan placeholder, bagian pertama terbuka, Segarkan bekerja, menghapus project terakhir bermremote itu menyembunyikan item dan rail kembali ke `agents`. GitHub: project dengan remote `github.com` menampilkan panel walau `gh` belum login (verifikasi login sungguhan adalah butir terbuka, tidak memblokir). Buka Pengaturan > Connector, cek baris GitLab/GitHub dan dialog Connect. Sesi terminal tidak terganggu.
6. Sinkronisasi dengan `docs/git/cli-findings.md`: cocokkan allowlist final, pemetaan status, pola galat `classify`, parser host login adapter dan `repo.go` dengan temuan; perbarui bagian ringkasan di akhir `cli-findings.md` bila keputusan berubah. Bila perlu menyentuh berkas milik fitur lain untuk penyelarasan, itu dilakukan di fase ini dan dilaporkan.
7. Pastikan tidak ada impor `internal/workspace` di `internal/gitdata` (`grep -rn "internal/workspace" internal/gitdata` kosong) dan tidak ada perintah tulis di allowlist.

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- `go build ./...` (dari root repo)
- Build frontend: `cd frontend && npm run build` (dan opsional `npm run check`)
- `make build` bila ada dan lulus (target `build-ui`, `build-daemon`, `build-app`)
- Pemeriksaan manual fixture: `jq . <berkas.json>` lulus untuk tiap JSON; fixture teranonimkan; fixture GitHub ditandai asumsi.

Fitur Go menjalankan `go build ./...`; fitur frontend menjalankan build frontend; spike tidak mengubah kode sehingga guard-nya `jq` dan grep anonimisasi. Jangan menambah berkas `*_test.go`.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, `internal/connector/cli.go`, `cmd/shikigamid/modules.go`, `cmd/shikigamictl/{main,registry}.go`, `frontend/src/main.ts`, atau berkas wiring pusat lain.
- Kontrak di bagian 2 adalah satu-satunya antarmuka antar fitur; jangan mengubah nama/signature di dalamnya. Bila kontrak terbukti salah, laporkan alih-alih menyimpang.
- Jangan membuat stub atau salinan berkas milik fitur lain agar build lulus.
- Hanya perintah baca; tidak ada token/secret di kode, fixture, atau dokumen; tidak menulis ke folder project user.
- Jangan commit; commit dan merge ditangani langkah implement-feature.
- Tidak ada unit test; guard hanya build (bagian 6).
- Teks UI dan pesan galat Indonesia; identifier kode Inggris.
