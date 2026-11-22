---
id: git-connector-ticket-01-git-repos
title: Connector Git dan deteksi repo
spec: docs/superpowers/specs/2026-11-22-10-git-connector-design.md
status: implemented
wave: 1
depends_on: []
---
# Connector Git dan deteksi repo

## Tujuan
Mendaftarkan dua connector, `gitlab` (CLI `glab`) dan `github` (CLI `gh`), di kerangka Connector yang sudah ada sehingga statusnya terbaca dari CLI yang sudah login tanpa token di app. Epic ini juga membangun paket `internal/gitdata` (tipe ternormalisasi, allowlist perintah baca, runner, klasifikasi galat, deteksi repo dari remote git tiap project) dan spike CLI yang hasilnya menjadi acuan semua parser tiket berikutnya. Di sisi UI, epic ini menambah item rail Git per provider beserta panel yang menampilkan satu bagian per repo/project, dan menyediakan titik ekstensi (komponen placeholder dan dua pane kind) yang diisi tiket 02 sampai 04.

## Hasil yang bisa dijalankan
Di atas kerangka Connector, rail menu utama, dan layout pane yang sudah ada, setelah tiket ini selesai:
- Pengaturan > Connector menampilkan baris **GitLab** dan **GitHub** dengan badge status ("CLI belum terpasang", "Belum login", "Tersambung" beserta nama user). Tombol **Connect** membuka dialog berisi `glab auth login` / `gh auth login` lalu **Cek ulang**. Tanpa perubahan frontend kerangka.
- Rail menampilkan item **GitLab** bila ada project di workspace aktif dengan remote GitLab (mis. `gitlab.com`), item **GitHub** bila ada remote GitHub, keduanya bila ada dua jenis, dan tidak ada item bila tidak ada remote dikenal. Isi mengikuti workspace aktif dan berganti saat workspace berganti. Item yang menjadi tidak visible membuat rail kembali ke `agents` (perilaku rail yang sudah ada).
- Membuka panel GitLab/GitHub menampilkan satu bagian per repo (judul menyebut nama project), bisa dilipat, bagian pertama terbuka. Isi bagian masih placeholder sederhana untuk daftar MR/PR dan Pipeline (diganti tiket 02 dan 03). Ada tombol **Segarkan** dan keadaan memuat/kosong/galat.
- `shikigamictl git repos --workspace <id>` mencetak repo hasil deteksi (alat debug). `shikigamictl connector status gitlab|github` mencetak status.
- `docs/git/cli-findings.md` dan fixture di `internal/gitdata/testdata/` tersedia sebagai acuan parser tiket 02 sampai 04.
- `go build ./...` dan build frontend lulus; aplikasi berjalan; sesi terminal tidak terganggu.

## Batasan
In scope:
- Adapter `internal/connector/gitlab.go` dan `github.go`.
- Paket `internal/gitdata`: tipe, allowlist untuk SEMUA perintah baca yang dibutuhkan epic ini dan tiket 02 sampai 04, runner, galat, deteksi repo.
- Spike `glab` dan `gh`, fixture, dokumen temuan.
- IPC `git.repos.list` dan `shikigamictl git repos`.
- Frontend `modules/git`: client, store, actions, ikon, panel, `ProjectSection`, placeholder `MrList`/`PipelineList`/`MrPane`/`PipelinePane`, registrasi rail dan pane kind.
- Perluasan `PaneKind` dan field leaf di `modules/layout/model.ts` tanpa mengubah perilaku lain.
- Teks UI bahasa Indonesia; identifier kode bahasa Inggris.

Out of scope:
- Daftar MR/PR, daftar pipeline, detail MR, detail pipeline (tiket 02, 03, 04); IPC `git.mrs.list`, `git.mr.get`, `git.pipelines.list`, `git.pipeline.get`.
- Perintah tulis apa pun (membuat/menyetujui/menggabung/menutup MR, retry/cancel/trigger pipeline), komentar, diff, issue, release.
- Login atau token di app, perubahan `WorkspaceConfig`, konfigurasi sumber manual, cache disk, polling otomatis.
- Mengubah kerangka connector selain menambah dua adapter.
- Menulis apa pun ke folder project user (hanya membaca remote git).
- Unit test (guard hanya build); allowlist ditegakkan di kode.

## Kontrak

### Yang disediakan

Go, `internal/connector/gitlab.go` dan `github.go` (package `connector`), pola `lark.go`:
```go
func init() { Register(gitlabConnector{}) } // ID "gitlab", Name "GitLab"
func init() { Register(githubConnector{}) } // ID "github", Name "GitHub"
func (c gitlabConnector) Status(ctx context.Context) (Status, error)
func (c gitlabConnector) ConnectGuide() Guide
```
- `Status()` memanggil `RunCLI(ctx, "glab", "auth", "status")` / `RunCLI(ctx, "gh", "auth", "status")` (PATH login-shell). Pemetaan: `IsCLIMissing(err)` -> `StateCLIMissing` ("glab belum terpasang" / "gh belum terpasang"); tidak ada host login -> `StateNotLoggedIn`; ada host login -> `StateConnected` dengan `User` = nama akun yang terbaca (host utama: `gitlab.com`/`github.com` bila ada, selain itu host pertama). Galat lain dikembalikan sebagai `error`. Bila spike menunjukkan exit non-nol berarti belum login, ikuti temuan spike. Output mentah CLI tidak dimuat di Status; tidak ada token.
- `ConnectGuide()`: langkah teks (pasang CLI bila belum ada, jalankan login, klik Cek ulang) dan `Commands: ["glab auth login"]` / `["gh auth login"]`.

Go, `internal/gitdata` (package `gitdata`; hanya mengimpor `internal/connector` dan stdlib; TIDAK mengimpor `internal/workspace`; path dan id diteruskan sebagai argumen):

`types.go` (json camelCase, dipakai tiket lain tanpa diubah):
```go
type Provider string
const ( ProviderGitLab Provider = "gitlab"; ProviderGitHub Provider = "github" )

type RepoProject struct { ID string `json:"id"`; Name string `json:"name"` }

// Satu Repo = satu kunci host+path; project yang berbagi remote digabung.
type Repo struct {
    ProjectID   string        `json:"projectId"`   // project pertama (rujukan utama untuk panggilan tiket lain)
    ProjectName string        `json:"projectName"` // nama project pertama
    Projects    []RepoProject `json:"projects"`    // semua project yang berbagi remote ini (termasuk yang pertama)
    Provider    Provider      `json:"provider"`
    Host        string        `json:"host"`        // mis. gitlab.com
    Path        string        `json:"path"`        // OWNER/REPO atau GROUP/SUBGROUP/REPO
    WebURL      string        `json:"webUrl"`      // https://HOST/PATH
}

type MergeRequest struct {
    Number int; Title, Author, SourceBranch, TargetBranch string
    State string // "open"|"merged"|"closed"
    Draft bool; ReviewState string; PipelineStatus Status
    UpdatedAt string // RFC3339
    URL string
}
type MergeRequestDetail struct {
    MergeRequest
    Description string; Labels []string; Reviewers []Reviewer
    Mergeable string // bentuk final mengikuti spike
    LastPipeline *PipelineRef
}
type Reviewer struct { Name, State string }
type PipelineRef struct { ID string; Status Status; URL string }
type Pipeline struct { ID, Ref, SHA string; Status Status; Author string; DurationSec int; UpdatedAt, URL string }
type PipelineDetail struct { Pipeline; Stages []Stage }
type Stage struct { Name string; Jobs []Job }
type Job struct { Name string; Status Status; DurationSec int; URL string }

type Status string
const (
    StatusPending Status = "pending"; StatusRunning Status = "running"; StatusSuccess Status = "success"
    StatusFailed Status = "failed"; StatusCanceled Status = "canceled"; StatusSkipped Status = "skipped"
    StatusUnknown Status = "unknown"
)
```
(Field di atas ditulis tanpa tag demi ringkas; implementasi memberi tag json camelCase, mis. `number`, `sourceBranch`, `pipelineStatus`, `durationSec`, `lastPipeline`. Field opsional memakai `omitempty`. Bentuk persis field opsional final mengikuti `docs/git/cli-findings.md`; bila spike mengubah bentuk, perbarui `types.go` dan catat di temuan, jangan mengubah nama field yang sudah tercantum.)

`errors.go`:
```go
type ErrorCode string
const (
    CodeCLIMissing   ErrorCode = "cli_missing"
    CodeNotLoggedIn  ErrorCode = "not_logged_in"
    CodePermission   ErrorCode = "permission"
    CodeNotFound     ErrorCode = "not_found"   // repo tidak ditemukan
    CodeTimeout      ErrorCode = "timeout"
    CodeNotAllowed   ErrorCode = "not_allowed"
    CodeCLI          ErrorCode = "cli_error"
)
type Error struct { Code ErrorCode; Message string } // Message Indonesia
func (e *Error) Error() string
func AsError(err error) (*Error, bool)
```

`runner.go` dan `allowlist.go`:
```go
const CallTimeout = 20 * time.Second
const MaxConcurrent = 3

// Run menjalankan satu perintah baca. tool = "glab" | "gh" | "git".
// Mengembalikan stdout. Dibatasi semaphore global MaxConcurrent.
func Run(ctx context.Context, tool string, args ...string) ([]byte, error)
```
- Allowlist eksplisit (tolak sisanya dengan `CodeNotAllowed` sebelum proses dijalankan): `glab auth status`, `gh auth status`, `glab mr list`, `glab mr view`, `glab ci list` / `glab pipeline list` / `glab ci get` (nama final dari spike), `gh pr list`, `gh pr view`, `gh run list`, `gh run view`, `git remote get-url`, `git remote` (daftar nama remote, bila perlu memilih remote pertama). Flag baca saja (`-R/--repo`, `-F json`, `--json`, `--limit`, dan sejenisnya dari spike); tolak flag yang berisiko tulis dan subcommand di luar daftar.
- `Run` memakai `connector.RunCLITimeout` bila sudah ada di `internal/connector/cli.go`; bila belum ada, `gitdata` mengimplementasikan timeout 20 detik dengan `context.WithTimeout` di sekeliling `RunCLI`-setara tanpa mengubah `cli.go` (file itu tidak dimiliki epic ini; lihat Batasan kepemilikan).

`repo.go`:
```go
// ProjectInput = pasangan yang diteruskan pemanggil (tanpa impor workspace).
type ProjectInput struct { ID, Name, Path string }

// DetectRepos membaca remote git tiap project (git -C Path remote get-url <remote>),
// mengurai host+path, menentukan provider, dan menggabung project berbagi remote.
func DetectRepos(ctx context.Context, projects []ProjectInput) ([]Repo, error)

// RepoFlag mengembalikan nilai untuk -R: OWNER/REPO (host bawaan) atau HOST/PATH (self-hosted).
func RepoFlag(r Repo) string
```

IPC (didaftarkan `cmd/shikigamid/wire_git.go`):
- `git.repos.list` params `{workspaceId: string}` -> `Repo[]` (json array, bentuk `Repo` di atas). Galat memakai bentuk galat Lark: `ipc.NewError(kode, "[<code>] <pesan Indonesia>")` dengan code dari `gitdata.ErrorCode` (`cli_missing`, `not_logged_in`, `permission`, `not_found`, `timeout`, `cli_error`) dan `not_found`/`invalid_params` untuk workspace tidak ada/parameter salah.

CLI debug, `cmd/shikigamictl/cmd_git.go`: `shikigamictl git repos --workspace <id>`; registrasi mandiri lewat `RegisterCommand` dari `init()`, dispatcher subcommand yang bisa diperluas (`registerGitSub(name, fn)` dengan peta, panic bila nama ganda atau kosong, meniru `cmd_lark.go`) sehingga tiket 02 sampai 04 menambah `mrs`, `pipelines`, `pipeline`, `mr` dari file mereka sendiri.

Frontend, `frontend/src/modules/git/`:
- `client.ts`: `listRepos(workspaceId): Promise<Repo[]>` (pembungkus IPC bertipe `git.repos.list`), tipe TS `Repo`, `RepoProject`, `Provider`, dan `parseGitError(x): {code, message}` (pola `parseLarkError`; kode diambil dari awalan `[code]` pesan). Tipe `MergeRequest`, `Pipeline`, dan sebagainya ditambah tiket lain di file klien mereka.
- `store.ts`: store `gitRepos` (Repo[] untuk workspace aktif), `gitState` (`loading|ready|error` + `code`, `message`), `refreshRepos()`, `repoKey(r)` (`host + '/' + path`), `reposFor(provider)` (turunan), state UI di store: `collapsed` (peta repoKey -> boolean; bagian pertama terbuka default) dan `selectedState` (peta repoKey -> 'open'|'merged'|'closed', default `open`, dipakai tiket 02). Deteksi berjalan saat app mulai (inisialisasi modul), saat `activeWorkspaceId` berganti, saat event `workspace.changed` dengan `kind` `projectAdded`/`projectRemoved` (dan `deleted`/`configChanged` bila memengaruhi workspace aktif), dan lewat `refreshRepos()` (Segarkan). Pola `modules/lark/store.ts` (penjaga urutan `seq`).
- `actions.ts`: `openInBrowser(url)`, `copyLink(url)`, `sendToAgent(url)` (pola `modules/lark/actions.ts`; `sendToAgent` memakai `pasteToSession` ke sesi aktif, URL + satu spasi tanpa Enter; tanpa sesi aktif: toast "Tidak ada sesi aktif").
- `GitIcon.svelte`: SVG inline, prop `provider` opsional untuk membedakan GitLab/GitHub.
- `GitPanel.svelte`: prop `provider`. `ProjectSection.svelte`: props `{provider, repo}`, judul menyebut nama project (bila banyak project berbagi remote, sebut semua nama dipisah koma) dan path repo, memuat `<MrList {provider} {repo} />` dan `<PipelineList {provider} {repo} />`.
- `MrList.svelte`, `PipelineList.svelte`: placeholder dengan props `{provider, repo}` yang TETAP (tiket 02/03 mengambil alih file tanpa mengubah props). `MrPane.svelte`, `PipelinePane.svelte`: placeholder pane; menerima leaf (provider, projectId, number/id) dan menampilkan teks sementara.
- `index.ts`: `registerRailItem` untuk `git-gitlab` (order 20) dan `git-github` (order 21), `visible` dari store (ada repo untuk provider itu di workspace aktif), dan `registerPaneKind` untuk `git-mr` dan `git-pipeline`. Modul dimuat otomatis oleh `main.ts` (`import.meta.glob('./modules/*/index.ts', { eager: true })`), jadi tidak perlu impor tambahan.

`frontend/src/modules/layout/model.ts`: `PaneKind` menjadi `... | 'git-mr' | 'git-pipeline'`; `Leaf` dan `NewLeaf` mendapat field dipersist `number?: number` (MR) dan `pipelineId?: string` (pipeline); `provider` dan `projectId` sudah ada di `Leaf`/`NewLeaf`. Nama field `number` dan `pipelineId` adalah kontrak untuk tiket 02 sampai 04. Tanpa perubahan perilaku lain (termasuk persist/restore layout yang menyalin field leaf; periksa fungsi serialisasi di file itu dan tambahkan field baru bila dipetakan eksplisit, seperti `sourceId`/`tableId` Lark).

### Yang dipakai
- `connector.Connector`, `Register`, `State*`, `Status`, `Guide`, `RunCLI`, `IsCLIMissing` (ada). `RunCLITimeout` bila sudah ada dari tiket Lark; bila tidak, lihat catatan di `Run`.
- `internal/ipc` (`NewError`, `Server.Publish`), `RegisterModule`/`Deps.Lookup("workspace.store")` di `cmd/shikigamid`, `workspace.Store`/`Project{ID, Path, Name}` (hanya di `wire_git.go`).
- IPC `connector.list|status|guide` (tidak diubah).
- Frontend: `registerRailItem` (`modules/shell/registry`), `registerPaneKind` (`modules/layout/registry`), `activeWorkspaceId`, `onEvent('workspace.changed', ...)`, `pasteToSession`, toast, `connectors` store untuk ajakan ke Pengaturan > Connector.
- `callAndPrint` dan `RegisterCommand` di `cmd/shikigamictl`.
- `glab` yang sudah login ke `gitlab.com` di mesin pengembang (spike); `gh` belum login (fixture dari dokumentasi).

### Kontrak lintas epic yang disediakan untuk tiket 02 sampai 04
- Tipe Go dan TS di atas; `gitdata.Run`, allowlist lengkap, `RepoFlag`, `CodeNotFound` dan galat terklasifikasi; semaphore 3 pemanggilan berlaku global di `Run`.
- Titik ekstensi frontend: `GitPanel -> ProjectSection -> MrList/PipelineList` (props `{provider, repo}`), pane kind `git-mr` dan `git-pipeline`, `actions.ts`, store (`selectedState`, `collapsed`), `registerGitSub` di ctl.
- Tiket 02 sampai 04 menambah fungsi pemanggil `gitdata` di file mereka sendiri (`mrs.go`, `pipelines.go`, `mrdetail.go`), memakai `Run`, `RepoFlag`, dan fixture di `internal/gitdata/testdata/`.

## Fitur

Catatan paralel umum: `git-cli-spike` dikerjakan PERTAMA karena fixture dan temuannya menentukan allowlist dan field opsional; fitur lain membaca hasilnya. Setelah spike selesai, `git-connector-adapters`, `gitdata-foundation`, dan `git-rail-panel-ui` bisa dikerjakan paralel (UI hanya bergantung pada bentuk `Repo` dan nama IPC dari Kontrak). `git-repos-ipc-ctl` bergantung pada `DetectRepos` dari `gitdata-foundation` (kerangkanya bisa ditulis paralel terhadap tanda tangan di Kontrak). `git-repos-wiring` dikerjakan terakhir.

### git-cli-spike: Spike CLI glab dan gh
Deskripsi:
Jalankan `glab` nyata (login ke `gitlab.com`) dan rekam keluaran. Semua panggilan read-only. Pola dokumen mengikuti `docs/lark/cli-findings.md`. Anonimkan sebelum disimpan: ganti nama user, email, nama grup/repo, judul dan isi MR/pipeline, SHA, dan URL dengan data palsu yang konsisten (struktur dan tipe tidak boleh berubah); tidak boleh ada token atau secret. Yang diukur dan dicatat:
1. Apakah `glab mr list -F json` memuat status pipeline terakhir tiap MR (field `head_pipeline`/`pipeline`) dan status review/approval. Putusan: kolom opsional baris MR (badge pipeline, status review) aktif atau dihilangkan dari baris (hanya di detail). Tanpa panggilan per baris.
2. Perintah dan bentuk keluaran daftar pipeline GitLab (`glab ci list` atau `glab pipeline list`/`glab api` bila subcommand tidak memuat field) dan detail pipeline (stage dan job; `glab ci get`/`glab ci view` dengan `-F json`), serta `gh run list --json ...` dan `gh run view --json jobs`. Catat nama subcommand final.
3. Cara membaca daftar host yang login dari `glab auth status` dan `gh auth status` (flag `--hostname`, `--show-token` TIDAK dipakai, bentuk keluaran teks atau JSON, exit code saat belum login), dipakai deteksi host self-hosted dan `Status()` adapter.
4. Bentuk keluaran `gh` tanpa login: fixture dibuat dari dokumentasi `gh` dan keluaran `glab`, dicatat eksplisit sebagai ASUMSI sampai `gh auth login` dijalankan.
Catat juga: flag `-R/--repo`, `-F json`, `--json` final dan daftar field yang tersedia untuk `gh pr list`, `gh pr view`, `gh run list`, `gh run view`; bentuk `-R` untuk GitLab subgroup dan host self-hosted (`HOST/PATH`); pemetaan status GitLab (`created, waiting_for_resource, preparing, pending, running, success, failed, canceled, skipped, manual, scheduled`) dan GitHub (`queued, in_progress, completed` + `conclusion`) ke status ternormalisasi; pola pesan galat stderr untuk belum login, izin kurang, repo tidak ditemukan; keluaran `git remote get-url origin` untuk ssh dan https.
Fixture di `internal/gitdata/testdata/` (JSON valid; nama berbasis perintah): `glab-mr-list.json`, `glab-mr-view.json`, `glab-pipeline-list.json`, `glab-pipeline-get.json`, `glab-auth-status.txt` (atau `.json` sesuai temuan), `gh-pr-list.json`, `gh-pr-view.json`, `gh-run-list.json`, `gh-run-view.json`, `gh-auth-status.txt`, dan `README.md` kecil berisi perintah penghasil tiap berkas (tanpa token) serta penanda fixture asumsi (GitHub).
Dokumen `docs/git/cli-findings.md` (bahasa Indonesia) bagian: versi `glab` dan `gh`, tabel perintah (flag wajib, format, bentuk `-R`), skema JSON ringkas per perintah (path field yang dipakai tiket 02 sampai 04), pemetaan status, host login, pola galat, remote git, dan bagian **Pengecualian dan pemotongan fitur** yang menyatakan hasil tiap asumsi (badge pipeline di baris MR, status review di baris MR, detail pipeline GitLab, GitHub tanpa login) beserta keputusan potong/aktif.
Tanpa perubahan Go atau frontend dalam fitur ini. Tanpa unit test.

Acceptance criteria:
- [ ] Semua perintah di atas dijalankan nyata untuk `glab` (GitHub memakai fixture asumsi yang ditandai) dan fixture disimpan (JSON valid, `jq .` lulus untuk berkas JSON).
- [ ] Fixture teranonimkan: tidak ada token, email, nama user/grup/repo asli, atau isi MR/pipeline asli (periksa dengan grep terhadap nilai asli yang diketahui).
- [ ] `internal/gitdata/testdata/README.md` mencatat perintah penghasil tiap fixture dan menandai fixture asumsi.
- [ ] `docs/git/cli-findings.md` memuat: jawaban tegas untuk keempat pertanyaan spike, flag final (`-R`, `-F json`, `--json`) dan field tersedia, pemetaan status, daftar host login, pola galat, bentuk `git remote get-url`.
- [ ] Bagian "Pengecualian dan pemotongan fitur" ada dengan keputusan eksplisit untuk badge pipeline dan status review di baris MR.
- [ ] Tidak ada perubahan Go atau frontend dalam fitur ini.

ownedPaths:
- `docs/git/cli-findings.md`
- `internal/gitdata/testdata/`

### git-connector-adapters: Adapter connector gitlab dan github
Deskripsi:
Buat `internal/connector/gitlab.go` dan `internal/connector/github.go` mengikuti pola `lark.go`/`fake.go`: tipe privat, `init()` memanggil `Register`. `Status()` menjalankan `glab auth status` / `gh auth status` lewat `RunCLI` (PATH login-shell) dan memetakan ke `cli_missing`, `not_logged_in`, `connected` (dengan `User`) sesuai Kontrak, mengikuti bentuk keluaran dan exit code dari `docs/git/cli-findings.md` bila sudah ada (bila belum, mulai dari asumsi: exit non-nol atau teks "not logged in" berarti belum login, lalu sesuaikan). Parsing tahan terhadap keluaran tak terduga (tidak panic; tak terparse berarti `not_logged_in`). Status tidak memuat output mentah CLI atau token. Tidak ada cache. `ConnectGuide()` berisi langkah teks dan perintah `glab auth login` / `gh auth login`, lalu Cek ulang. Kerangka connector tidak diubah.

Acceptance criteria:
- [ ] `connector.Get("gitlab")` dan `connector.Get("github")` mengembalikan connector dengan `Name()` "GitLab" dan "GitHub".
- [ ] CLI tidak ada -> `cli_missing`; belum login -> `not_logged_in`; login -> `connected` dengan `User`.
- [ ] `ConnectGuide()` berisi langkah teks dan perintah login masing-masing.
- [ ] Status tidak memuat token atau output mentah CLI.
- [ ] Pengaturan > Connector menampilkan baris GitLab dan GitHub tanpa perubahan frontend.
- [ ] `go build ./...` lulus.

ownedPaths:
- `internal/connector/gitlab.go`
- `internal/connector/github.go`

### gitdata-foundation: Paket gitdata, tipe, runner, allowlist, deteksi repo
Deskripsi:
Paket `internal/gitdata` (hanya mengimpor `internal/connector` dan stdlib):
- `doc.go`: dokumentasi paket (read-only, arah dependensi `gitdata -> connector`, tidak mengimpor `workspace`).
- `types.go`: semua tipe dan konstanta status di Kontrak, dengan tag json camelCase. Tambahkan fungsi pemetaan status yang bisa dipakai tiket lain: `NormalizeGitLabStatus(string) Status` dan `NormalizeGitHubStatus(status, conclusion string) Status` (nilai tak dikenal menjadi `unknown`; `manual`/`scheduled` GitLab dan `waiting`/`requested` GitHub menjadi `pending`; `cancelled` dan `canceled` menjadi `canceled`; sesuaikan pemetaan final dengan temuan spike).
- `allowlist.go`: tabel eksplisit (tool + subcommand) untuk semua perintah baca di Kontrak, dengan flag yang diizinkan per entri. Fungsi privat `check(tool string, args []string) *Error`; tolak args kosong, subcommand di luar tabel, dan flag berisiko tulis (`CodeNotAllowed`) sebelum proses dijalankan. Subcommand tulis seperti `mr create|merge|approve|close`, `ci run|retry|cancel`, `pr create|merge|review|close`, `run rerun|cancel` tidak ada di tabel.
- `runner.go`: `Run` sesuai Kontrak, `CallTimeout = 20 * time.Second`, semaphore (channel berkapasitas 3, `MaxConcurrent`) yang menunggu giliran dengan menghormati `ctx` (ctx batal saat menunggu -> `CodeTimeout`/galat ctx terklasifikasi), mengembalikan stdout sebagai `[]byte`. Tidak mengubah direktori kerja untuk `glab`/`gh` (selalu `-R`); `git` memakai `-C <path>`.
- `errors.go`: `Error`, `ErrorCode`, `AsError`, `classify(err, tool)` dengan pesan Indonesia terbaca: `cli_missing` ("glab belum terpasang" / "gh belum terpasang" / "git belum terpasang"), `not_logged_in` ("Belum login ke GitLab/GitHub. Jalankan `glab auth login`/`gh auth login` di Pengaturan > Connector"), `permission` ("Izin kurang: <pesan CLI>"), `not_found` ("Repo tidak ditemukan atau tidak ada akses"), `timeout` ("GitLab/GitHub tidak merespons dalam 20 detik"), `cli_error` (pesan CLI diringkas). Pola pencocokan pesan stderr dari `docs/git/cli-findings.md`; sampai spike selesai pakai pola wajar ("not logged in", "authentication", "401", "403", "404", "not found", "Could not resolve") dan sesuaikan.
- `repo.go`: `ProjectInput`, `DetectRepos`, `RepoFlag`, dan helper privat:
  - baca remote: `git -C <path> remote` untuk daftar nama; pakai `origin` bila ada, jika tidak remote pertama; lalu `git -C <path> remote get-url <nama>`. Project tanpa remote atau path tak ada diabaikan diam-diam (bukan galat).
  - urai URL: `git@host:owner/repo.git`, `ssh://git@host[:port]/owner/repo.git`, `https://[user@]host/owner/repo[.git]`; buang `.git`, slash ujung, kredensial dan port; path boleh lebih dari dua segmen (subgroup GitLab tidak dipotong).
  - provider: `github.com` -> `github`; `gitlab.com` -> `gitlab` (selalu dikenali tanpa cek login). Host lain: dikenali bila `glab` login ke host itu (GitLab diutamakan) atau `gh` login ke host itu (GitHub Enterprise), memakai daftar host login dari `glab auth status`/`gh auth status` yang di-cache di memori singkat (TTL sekitar 60 detik, dilindungi mutex); tidak cocok berarti diabaikan diam-diam.
  - penggabungan: kunci `host + "/" + path`; project berbagi remote digabung menjadi satu `Repo` dengan `Projects` berisi semuanya dalam urutan input; `ProjectID/ProjectName` = project pertama. Urutan hasil stabil mengikuti project pertama tiap repo.
  - `WebURL = "https://" + host + "/" + path`.
  - `RepoFlag`: host bawaan (`gitlab.com`/`github.com`) -> `path`; host lain -> `host + "/" + path`.
  - Pemanggilan `git` per project berjalan dengan batas semaphore yang sama; galat satu project tidak menggagalkan project lain.

Acceptance criteria:
- [ ] Semua tipe di Kontrak ada dengan tag json camelCase dan konstanta status ternormalisasi persis (`pending|running|success|failed|canceled|skipped|unknown`).
- [ ] `Run` menolak perintah di luar allowlist dengan `CodeNotAllowed` tanpa menjalankan proses; tidak ada perintah tulis di allowlist.
- [ ] Timeout efektif 20 detik; paling banyak 3 proses CLI berjalan bersamaan (semaphore menghormati `ctx`).
- [ ] Galat terklasifikasi dengan pesan Indonesia untuk `cli_missing`, `not_logged_in`, `permission`, `not_found`, `timeout`, `not_allowed`, `cli_error`.
- [ ] `DetectRepos` menangani ssh dan https, `origin` atau remote pertama, `github.com`/`gitlab.com` tanpa cek login, host self-hosted hanya bila CLI login ke host itu, GitLab diutamakan, subgroup tidak dipotong, project tanpa remote diabaikan, dan project berbagi remote digabung per host+path.
- [ ] `RepoFlag` berbentuk `OWNER/REPO`/`GROUP/SUBGROUP/REPO` untuk host bawaan dan `HOST/PATH` untuk self-hosted.
- [ ] Paket tidak mengimpor `internal/workspace`.
- [ ] `go build ./...` lulus.

ownedPaths:
- `internal/gitdata/doc.go`
- `internal/gitdata/types.go`
- `internal/gitdata/allowlist.go`
- `internal/gitdata/runner.go`
- `internal/gitdata/errors.go`
- `internal/gitdata/repo.go`

### git-repos-ipc-ctl: IPC git.repos.list dan shikigamictl git repos
Deskripsi:
`cmd/shikigamid/wire_git.go` (pola `wire_lark.go`, registrasi mandiri): `init()` memanggil `RegisterModule("git", func(srv *ipc.Server, d *Deps) { registerGitRepos(srv, d) })`. Handler `git.repos.list` `{workspaceId}`: ambil `workspace.Store` dari `d.Lookup("workspace.store")`, baca workspace (galat `not_found` bila tidak ada, `invalid_params` bila `workspaceId` kosong), ubah `Project{ID, Name, Path}` menjadi `[]gitdata.ProjectInput`, panggil `gitdata.DetectRepos`, kembalikan `[]gitdata.Repo` (array kosong, bukan null, bila tidak ada). Galat `gitdata` dipetakan ke bentuk galat Lark (`ipc.NewError(kode, "[code] pesan")`, fungsi pemetaan privat bernama khusus git agar tidak bentrok dengan `larkErr`/`mapLarkStoreErr`; helper decode parameter juga milik file ini, jangan memakai ulang nama dari `wire_lark.go`). File ini tidak boleh memuat handler tiket lain; tiket 02 sampai 04 menambah `wire_git_mrs.go`, `wire_git_pipelines.go`, `wire_git_mr_detail.go` yang mendaftar sendiri. Sediakan helper yang bisa dipakai file-file itu (sama paket `main`): `gitStore(d)`, `gitErr(code, ipcCode, msg)`, `mapGitError(err)`, `gitDecode(params, v)`, dan `gitResolveRepo(d, workspaceId, projectId) (gitdata.Repo, error)` yang menjalankan deteksi dan mencari repo yang memuat `projectId` (galat `not_found` "Project tidak ditemukan atau tidak punya remote dikenal").

`cmd/shikigamictl/cmd_git.go`: `init()` memanggil `RegisterCommand(Command{Name: "git", Summary: "Connector Git: repo, MR, pipeline", Run: runGit})`. `runGit` membaca `args[0]`; peta `gitSubcommands` dan `registerGitSub(name, fn)` didefinisikan di sini (panic bila nama ganda atau kosong). Subcommand `repos` menerima `--workspace <id>` (bila tidak diberikan, galat penggunaan) dan memanggil `callAndPrint(c, "git.repos.list", map[string]any{"workspaceId": id})`. Pesan penggunaan memuat daftar subcommand terurut. Tanpa argumen atau subcommand tak dikenal -> galat penggunaan, bukan panic.

Acceptance criteria:
- [ ] `git.repos.list` terdaftar lewat `RegisterModule` dari `init()` di `wire_git.go` dan mengembalikan `Repo[]` sesuai Kontrak untuk workspace yang ada.
- [ ] Workspace tidak ada -> galat `[not_found] ...`; parameter salah -> `[invalid_params] ...`; galat `gitdata` terpetakan ke kode dan pesan Indonesia.
- [ ] Project berbagi remote muncul sebagai satu `Repo` dengan `projects` berisi semuanya.
- [ ] Helper bersama (`gitStore`, `gitErr`, `mapGitError`, `gitDecode`, `gitResolveRepo`) ada dan tidak bentrok nama dengan file lain di paket `main`.
- [ ] `shikigamictl git repos --workspace <id>` mencetak hasil; `shikigamictl git` tanpa argumen/subcommand tak dikenal menampilkan penggunaan; `registerGitSub` diekspos tanpa perlu mengedit `cmd_git.go`.
- [ ] `shikigamictl` (bantuan) menampilkan `git` di daftar command.
- [ ] `go build ./...` lulus.

ownedPaths:
- `cmd/shikigamid/wire_git.go`
- `cmd/shikigamictl/cmd_git.go`

### git-rail-panel-ui: Rail Git, panel per provider, titik ekstensi pane
Deskripsi:
Implementasikan frontend `modules/git` sesuai Kontrak (client, store, actions, ikon, panel, section, placeholder, registrasi). Rincian perilaku:
- Store: `refreshRepos()` memanggil `listRepos(activeWorkspaceId)` dengan penjaga urutan agar hasil basi dibuang; saat workspace berganti, kosongkan repo dan set `loading`, lalu muat. Item rail tidak tampil sampai deteksi pertama selesai (tanpa flicker karena `visible` false saat `loading`/kosong). Galat deteksi menyembunyikan item rail tetapi tidak mengganggu aplikasi (galat dicatat di `gitState`; panel yang sudah terbuka menampilkan galat dengan ajakan ke Pengaturan > Connector bila kode `cli_missing`/`not_logged_in`).
- `visible` per provider: `derived(gitRepos, rs => rs.some(r => r.provider === provider))`.
- `GitPanel`: header dengan judul provider dan tombol **Segarkan** (memanggil `refreshRepos`); keadaan memuat ("Memuat repo..."), kosong ("Tidak ada project dengan remote GitLab/GitHub di workspace ini"), galat terbaca; daftar `ProjectSection` untuk repo `provider` itu, urutan sesuai store. Provider yang CLI-nya belum login tetap tampil (karena `gitlab.com`/`github.com` selalu dikenali) dan isi bagian mengarahkan ke Pengaturan > Connector pada tiket berikutnya; di tiket ini placeholder boleh menyertakan satu kalimat ajakan umum.
- `ProjectSection`: header yang bisa diklik untuk lipat/buka, judul "<nama project> - <path>" (banyak project: nama digabung koma), state terlipat disimpan di store (`collapsed`), bagian pertama terbuka default, lainnya terlipat; isi hanya dirender bila terbuka (agar tiket 02/03 memuat data saat bagian dibuka dan menghormati batas 3 pemanggilan). Menyediakan slot bagi dua daftar: bagian "Merge Request"/"Pull Request" (judul menurut provider) berisi `<MrList/>` dan bagian "Pipeline"/"Actions" berisi `<PipelineList/>`.
- Placeholder: `MrList` dan `PipelineList` menampilkan teks "Segera hadir" dengan props `{provider, repo}`; `MrPane` dan `PipelinePane` menampilkan judul pane dan teks sementara tanpa memanggil IPC.
- `actions.ts`: tiga fungsi ber-signature `(url: string) => void | Promise<void>`; salin link memakai clipboard app dan toast konfirmasi; ikuti implementasi `modules/lark/actions.ts`.
- `model.ts`: perluasan seperti di Kontrak; jangan mengubah fungsi lain.
- Gaya: ikuti variabel CSS yang ada; aksesibilitas dasar (tombol bisa difokus, `aria-expanded` pada header bagian).
Tanpa unit test.

Acceptance criteria:
- [ ] Rail menampilkan `git-gitlab` (order 20) dan `git-github` (order 21) hanya bila ada repo provider itu di workspace aktif; hilang bila tidak ada; kembali ke `agents` bila item aktif menjadi tidak visible.
- [ ] Deteksi berjalan saat app mulai, saat workspace berganti, saat project ditambah/dihapus, dan saat Segarkan; hasil basi dibuang.
- [ ] `GitPanel` (satu komponen untuk dua provider) menampilkan satu `ProjectSection` per repo dengan judul menyebut nama project; bagian pertama terbuka, lainnya terlipat; state terlipat bertahan setelah panel di-unmount lalu dibuka lagi.
- [ ] Keadaan memuat, kosong, dan galat tampil; tombol Segarkan bekerja.
- [ ] `MrList`/`PipelineList` placeholder memakai props `{provider, repo}` dan dirender di `ProjectSection`; `MrPane`/`PipelinePane` placeholder terdaftar sebagai pane kind `git-mr` dan `git-pipeline`.
- [ ] `PaneKind` memuat `'git-mr' | 'git-pipeline'`; `Leaf` dan `NewLeaf` memuat `number?` dan `pipelineId?` dan field itu ikut dipersist; layout lama tetap termuat.
- [ ] `actions.ts` mengekspor `openInBrowser`, `copyLink`, `sendToAgent` (tanpa sesi aktif: toast "Tidak ada sesi aktif"; URL + spasi, tanpa Enter).
- [ ] Build frontend lulus dan perilaku modul lain tidak berubah.

ownedPaths:
- `frontend/src/modules/git/index.ts`
- `frontend/src/modules/git/store.ts`
- `frontend/src/modules/git/client.ts`
- `frontend/src/modules/git/actions.ts`
- `frontend/src/modules/git/GitIcon.svelte`
- `frontend/src/modules/git/GitPanel.svelte`
- `frontend/src/modules/git/ProjectSection.svelte`
- `frontend/src/modules/git/MrList.svelte`
- `frontend/src/modules/git/PipelineList.svelte`
- `frontend/src/modules/git/MrPane.svelte`
- `frontend/src/modules/git/PipelinePane.svelte`
- `frontend/src/modules/layout/model.ts`

### git-repos-wiring: Wiring dan verifikasi deteksi repo
Deskripsi:
Fitur penutup tanpa file milik sendiri. Pastikan semua bagian tersambung dan terlihat di aplikasi (lihat bagian Wiring). Hanya menyentuh titik impor/registrasi yang sudah ada; bila ternyata perlu mengubah file di luar `ownedPaths` epic ini, laporkan alasannya alih-alih mengubahnya diam-diam.

Acceptance criteria:
- [ ] `wire_git.go` terpakai daemon (`RegisterModule` dijalankan saat daemon start) dan `git.repos.list` bisa dipanggil lewat `shikigamictl git repos`.
- [ ] Adapter `gitlab` dan `github` muncul di Pengaturan > Connector dengan dialog Connect yang benar.
- [ ] Item rail muncul sesuai project dengan remote GitLab/GitHub dan hilang bila tidak ada; kembali ke `agents` bila item aktif menjadi tidak visible.
- [ ] `go build ./...` dan build frontend lulus; aplikasi berjalan.
- [ ] Verifikasi GitLab dengan `glab` login ke `gitlab.com`; jalur GitHub diverifikasi dengan fixture/remote GitHub tanpa login (butir terbuka sampai `gh auth login` dijalankan, tidak memblokir).

ownedPaths: (tidak ada; hanya verifikasi dan penyesuaian kecil di titik impor/registrasi yang sudah ada, dilaporkan bila di luar ownedPaths)

## Path yang dimiliki
Hanya ini yang boleh disentuh oleh tiket ini:
- `internal/connector/gitlab.go`
- `internal/connector/github.go`
- `internal/gitdata/doc.go`
- `internal/gitdata/types.go`
- `internal/gitdata/allowlist.go`
- `internal/gitdata/runner.go`
- `internal/gitdata/errors.go`
- `internal/gitdata/repo.go`
- `internal/gitdata/testdata/`
- `docs/git/cli-findings.md`
- `cmd/shikigamid/wire_git.go`
- `cmd/shikigamictl/cmd_git.go`
- `frontend/src/modules/git/index.ts`
- `frontend/src/modules/git/store.ts`
- `frontend/src/modules/git/client.ts`
- `frontend/src/modules/git/actions.ts`
- `frontend/src/modules/git/GitIcon.svelte`
- `frontend/src/modules/git/GitPanel.svelte`
- `frontend/src/modules/git/ProjectSection.svelte`
- `frontend/src/modules/git/MrList.svelte`
- `frontend/src/modules/git/PipelineList.svelte`
- `frontend/src/modules/git/MrPane.svelte`
- `frontend/src/modules/git/PipelinePane.svelte`
- `frontend/src/modules/layout/model.ts`

Catatan kepemilikan: `internal/connector/cli.go` tidak dimiliki epic ini. `gitdata.Run` memakai `connector.RunCLITimeout` bila sudah ada (ditambahkan tiket Lark 01); bila belum ada di cabang kerja, `gitdata` menerapkan timeout sendiri tanpa mengubah `cli.go`. Setelah tiket ini, `MrList.svelte` (tiket 02), `PipelineList.svelte` dan `PipelinePane.svelte` (tiket 03), serta `MrPane.svelte` (tiket 04) berpindah kepemilikan ke tiket masing-masing tanpa mengubah props.

## Wiring
Tiket ini tidak bergantung pada tiket lain; hasilnya terlihat dan jalan sendiri.
1. Adapter: `gitlab.go` dan `github.go` terdaftar otomatis lewat `init()`; paket `internal/connector` sudah dimuat daemon (connector `fake` dan `lark` terdaftar dengan cara yang sama), jadi IPC `connector.list/status/guide` dan Pengaturan > Connector langsung memuat GitLab dan GitHub. Periksa bahwa pemuatan paket memang terjadi di binary daemon.
2. IPC: `wire_git.go` mendaftar sendiri lewat `RegisterModule("git", ...)` dari `init()` (pola `wire_lark.go`); tidak mengubah file registrasi pusat. Pastikan modul benar-benar dipanggil saat daemon start (cek `shikigamictl git repos`).
3. CLI: `cmd_git.go` terdaftar otomatis lewat `RegisterCommand` dari `init()`; tidak mengubah `main.go`/`registry.go`.
4. Frontend: `modules/git/index.ts` dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts', { eager: true })` di `frontend/src/main.ts`; tidak perlu impor tambahan. Dua rail item dan dua pane kind terdaftar di sana. Pane kind baru memakai `PaneKind` yang diperluas di `model.ts`.
5. Verifikasi akhir: jalankan `go build ./...` dan build frontend; jalankan aplikasi. Dengan workspace berisi project yang remote-nya `gitlab.com` (dan `glab` login), pastikan item GitLab muncul, panel menampilkan bagian per project dengan placeholder, Segarkan bekerja, menghapus project terakhir bermremote itu menyembunyikan item dan rail kembali ke `agents`. Untuk GitHub, gunakan project dengan remote `github.com` (panel tampil walau `gh` belum login); verifikasi login sungguhan adalah butir terbuka. Buka Pengaturan > Connector dan cek baris GitLab/GitHub serta dialog Connect.
6. Perbarui bagian ringkasan di akhir `docs/git/cli-findings.md` bila ada keputusan yang berubah setelah adapter/runner selesai (mis. allowlist final, pemetaan status).

## Guard
- `go build ./...` lulus.
- Build frontend lulus (perintah build di `frontend/`, mis. `npm run build`).
- `make build` (bila ada) lulus; aplikasi berjalan dan rail/panel Git serta Pengaturan > Connector berfungsi.
- Tanpa unit test (kebijakan epic).
- Pemeriksaan manual fixture: JSON valid dan teranonimkan; fixture GitHub ditandai asumsi.
