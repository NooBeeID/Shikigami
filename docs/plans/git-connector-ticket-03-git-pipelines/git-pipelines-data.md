# Plan: Data pipeline gitdata dan IPC

## Metadata

- Tiket: `git-connector-ticket-03-git-pipelines` (Daftar pipeline dan detail pipeline)
- Fitur: `git-pipelines-data`
- ownedPaths: `internal/gitdata/pipelines.go`, `cmd/shikigamid/wire_git_pipelines.go`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; tipe, runner, allowlist, fixture, helper `gitDecode`/`gitErr`/`gitResolveRepo`/`mapGitError` sudah ada dari tiket 1 dan 2)

## Tujuan

Menyediakan data pipeline GitLab dan run GitHub Actions yang sudah dinormalisasi, plus dua IPC daemon (`git.pipelines.list`, `git.pipeline.get`). Sumber kebenaran kontrak ada di `docs/plans/git-connector-ticket-03-git-pipelines/overview.md` (bagian 0, 2.1-2.4); plan ini hanya merinci langkah. Jangan mengubah `overview.md`.

## Path yang dimiliki

HANYA dua file baru ini yang boleh dibuat/disentuh:
- `internal/gitdata/pipelines.go`
- `cmd/shikigamid/wire_git_pipelines.go`

Jangan menyentuh file wiring atau path fitur lain. Baca-saja (jangan ubah): `internal/gitdata/{types,runner,errors,allowlist,repo,mrs}.go`, `internal/gitdata/testdata/*`, `cmd/shikigamid/{wire_git,wire_git_mrs}.go`.

## Kontrak

Dipakai (sudah ada, jangan didefinisikan ulang):
- `gitdata.Pipeline{ID string, Ref, SHA string, Status Status, Author string, DurationSec int, UpdatedAt, URL string}`, `Job{Name, Status, DurationSec int, URL}`, `Stage{Name, Jobs []Job}`, `PipelineDetail{Pipeline; Stages []Stage}` (embedded `Pipeline`), `Repo` (`Provider`, `Host`, `Path`), `ProviderGitLab`, `ProviderGitHub`.
- `NormalizeGitLabStatus(s string) Status`, `NormalizeGitHubStatus(status, conclusion string) Status`, konstanta `StatusPending|Running|Success|Failed|Canceled|Skipped|Unknown`.
- `Run(ctx, tool, args...) ([]byte, error)` (galat sudah `*Error`), `Error{Code, Message}`, `CodeCLI`, `CodeNotFound`, `ErrorCode("invalid_params")`, `providerLabel(tool)`.
- Daemon: `RegisterModule`, `Deps`, `ipc.Server.Register`, `ipc.ErrInvalid`, `gitDecode`, `gitErr`, `gitResolveRepo(d, workspaceID, projectID)`, `mapGitError`.

Disediakan (persis, lihat overview 2.3):
```go
const PipelineLimit = 20
func ListPipelines(ctx context.Context, repo Repo) ([]Pipeline, error)          // non-nil, urut UpdatedAt turun, maks 20
func GetPipeline(ctx context.Context, repo Repo, id string) (PipelineDetail, error)
func parseGitLabPipelines(data []byte) ([]Pipeline, error)
func parseGitHubRuns(data []byte) ([]Pipeline, error)
func parseGitLabPipelineDetail(data []byte) (PipelineDetail, error)
func parseGitHubRunDetail(data []byte) (PipelineDetail, error)
```
IPC: `git.pipelines.list {workspaceId, projectId}` -> `Pipeline[]` (array non-nil); `git.pipeline.get {workspaceId, projectId, id}` -> `PipelineDetail` (`stages` selalu array). Bentuk JSON mengikuti overview 2.1 dan 2.2.

## Langkah

### 1. `internal/gitdata/pipelines.go` (package `gitdata`)

Impor: `context`, `encoding/json`, `math`, `regexp`, `sort`, `strconv`, `strings`, `time`. Tanpa `os/exec`, tanpa `internal/workspace`, tanpa `connector.RunCLITimeout`. Semua simbol privat baru diawali `pipeline` atau `parseGit` (hindari bentrok dengan `mr*`/`parseMr*`).

1. `const PipelineLimit = 20`.
2. `func pipelineRepoArg(r Repo) string` (JANGAN pakai `RepoFlag` maupun `mrRepoArg`): GitLab `gitlab.com` (EqualFold) -> `r.Path`, GitLab host lain -> `"https://" + r.Host + "/" + r.Path`; GitHub `github.com` -> `r.Path`, host lain -> `r.Host + "/" + r.Path`.
3. Helper kecil: `pipelineParseTime(s string) time.Time` (RFC3339, gagal -> zero), `pipelineUnreadable(tool string) *Error` -> `&Error{Code: CodeCLI, Message: providerLabel(tool) + " mengembalikan data yang tidak dapat dibaca"}`, `pipelineIDString(raw json.Number)`-style konversi angka JSON ke string (decode `id` sebagai `json.Number` atau `int64` lalu `strconv.FormatInt`; field hilang -> `""`).
4. `parseGitLabPipelines(data)`: trim; kosong atau `null` -> `[]Pipeline{}`. Unmarshal ke slice struct `{ID int64 "id"; Ref "ref"; SHA "sha"; Status "status"; WebURL "web_url"; UpdatedAt "updated_at"}`; gagal -> `pipelineUnreadable("glab")`. Peta: `ID` = `strconv.FormatInt`, `Status` = `NormalizeGitLabStatus`, `URL` = `web_url`, `UpdatedAt` apa adanya; `Author` kosong, `DurationSec` 0. Hasil slice non-nil.
5. `parseGitHubRuns(data)`: kosong/`null` -> `[]Pipeline{}`. Struct `{DatabaseID int64 "databaseId"; HeadBranch; HeadSha; Status; Conclusion; CreatedAt; UpdatedAt; StartedAt; URL}`; gagal -> `pipelineUnreadable("gh")`. Panggil helper bersama `pipelineFromGitHubRun(raw) Pipeline` (dipakai juga oleh detail): `ID`, `Ref`=`headBranch`, `SHA`=`headSha`, `Status`=`NormalizeGitHubStatus(status, conclusion)`, `UpdatedAt`=`updatedAt` (kosong -> `createdAt`), `URL`; `Author` kosong; `DurationSec` = `updatedAt - startedAt` detik HANYA bila status ternormalisasi bukan `StatusPending`/`StatusRunning`, kedua waktu valid, dan hasil >= 0; selain itu 0.
6. `parseGitLabPipelineDetail(data)`: trim; kosong/`null`/bukan objek JSON (karakter pertama bukan `{`) atau Unmarshal gagal -> `&Error{Code: CodeNotFound, Message: "Pipeline tidak ditemukan"}`. Struct: field pipeline seperti langkah 4 ditambah `User *struct{Username string "username"} "user"`, `Duration float64 "duration"`, `Jobs []struct{ID int64 "id"; Name; Stage; Status; Duration float64 "duration"; WebURL "web_url"} "jobs"`. `Author`=`user.username` (nil -> ""), `DurationSec`=`int(math.Round(duration))`. Pengelompokan stage: salin job, `sort.SliceStable` ascending menurut `ID`; stage berurutan menurut kemunculan pertama (slice nama + map nama -> indeks); job dalam stage tetap urut id naik; `Job{Name, Status: NormalizeGitLabStatus, DurationSec: int(math.Round(duration)), URL: web_url}`. Nama stage kosong -> `"Jobs"`. `jobs: null` -> `Stages: []Stage{}` (non-nil); tiap `Stage.Jobs` non-nil.
7. `parseGitHubRunDetail(data)`: kosong/`null` -> `pipelineUnreadable("gh")`; Unmarshal gagal -> `pipelineUnreadable("gh")`. Field run seperti langkah 5 ditambah `WorkflowName` dan `Jobs []struct{Name; Status; Conclusion; StartedAt; CompletedAt; URL}`. `Pipeline` dari `pipelineFromGitHubRun`. Satu `Stage` bernama `workflowName` (trim; kosong -> `"Jobs"`) berisi semua job sesuai urutan array; `Job.Status`=`NormalizeGitHubStatus`, `DurationSec`=`completedAt - startedAt` (kedua valid dan >= 0, selain itu 0), `URL`=`url` job. Tanpa job -> `Stages` tetap berisi satu stage dengan `Jobs: []Job{}` (atau `[]Stage{}` bila `jobs` kosong; pilih `[]Stage{}` agar UI menampilkan "Tidak ada stage"; stages selalu non-nil).
8. `func ListPipelines(ctx, repo)`: switch `repo.Provider`:
   - GitLab: `Run(ctx, "glab", "ci", "list", "--repo", arg, "--output", "json", "--per-page", "20")` lalu `parseGitLabPipelines`.
   - GitHub: `Run(ctx, "gh", "run", "list", "--repo", arg, "--limit", "20", "--json", "databaseId,headBranch,headSha,status,conclusion,createdAt,updatedAt,startedAt,url,event,displayTitle,workflowName")` lalu `parseGitHubRuns`.
   - Provider lain -> `&Error{Code: CodeCLI, Message: "Provider git tidak didukung"}`.
   Galat dikembalikan apa adanya (sudah `*Error`; jangan membungkus ulang). Lalu `sort.SliceStable` menurut `pipelineParseTime(UpdatedAt)` turun (gagal parse = zero = paling lama), potong ke `PipelineLimit`, jamin non-nil (`[]Pipeline{}`).
9. `func GetPipeline(ctx, repo, id)`: validasi `id` dengan `regexp.MustCompile("^[0-9]+$")` (paket-level var `pipelineIDPattern`); tidak cocok -> `PipelineDetail{Stages: []Stage{}}, &Error{Code: ErrorCode("invalid_params"), Message: "ID pipeline tidak valid"}`. Lalu:
   - GitLab: `Run(ctx, "glab", "ci", "get", "--repo", arg, "--pipeline-id", id, "--output", "json")` -> `parseGitLabPipelineDetail`.
   - GitHub: `Run(ctx, "gh", "run", "view", id, "--repo", arg, "--json", "databaseId,headBranch,headSha,status,conclusion,createdAt,updatedAt,startedAt,url,event,displayTitle,workflowName,jobs")` -> `parseGitHubRunDetail`.
   - Provider lain -> galat tidak didukung seperti di atas.
   Hanya bentuk flag panjang; jangan memakai `--log`, `--log-failed`, `--web`, `--with-variables`, `glab api`, `gh api`.

### 2. `cmd/shikigamid/wire_git_pipelines.go` (package `main`)

Ikuti pola `wire_git_mrs.go`. Impor: `context`, `encoding/json`, `regexp`, `strings`, `internal/gitdata`, `internal/ipc`. Simbol privat diawali `gitPipelines`.

1. `func init() { RegisterModule("git-pipelines", func(srv *ipc.Server, d *Deps) { registerGitPipelines(srv, d) }) }`. Pastikan nama `git-pipelines` unik (`grep -rn 'RegisterModule("git' cmd/shikigamid`).
2. `func registerGitPipelines(srv *ipc.Server, d *Deps)`:
   - `srv.Register("git.pipelines.list", ...)`: struct params `{WorkspaceID "workspaceId"; ProjectID "projectId"}`; `gitDecode`; `workspaceId`/`projectId` kosong (TrimSpace) -> `gitErr("invalid_params", ipc.ErrInvalid, "workspaceId wajib diisi" / "projectId wajib diisi")`; `gitResolveRepo` lalu `gitdata.ListPipelines(ctx, repo)`, galat lewat `mapGitError`; `nil` -> `[]gitdata.Pipeline{}`; kembalikan slice LANGSUNG (bukan map pembungkus).
   - `srv.Register("git.pipeline.get", ...)`: params ditambah `ID "id"`; validasi workspace/project kosong seperti di atas; `id` kosong atau tidak cocok `^[0-9]+$` (var paket `gitPipelinesIDPattern`) -> `gitErr("invalid_params", ipc.ErrInvalid, "id pipeline wajib berupa angka")`; resolve repo, `gitdata.GetPipeline(ctx, repo, p.ID)`, galat lewat `mapGitError`; jamin `Stages` non-nil (`if d.Stages == nil { d.Stages = []gitdata.Stage{} }`); kembalikan struct detail.
3. Jangan mengubah `wire_git.go`; jangan menambah logika deteksi repo.

## Perubahan

- Dibuat: `internal/gitdata/pipelines.go`
- Dibuat: `cmd/shikigamid/wire_git_pipelines.go`
- Tidak ada file lain diubah (tanpa unit test, tanpa dependensi baru, tanpa commit).

## Wiring

Yang diekspos untuk langkah wiring (`git-pipelines-wiring`), tanpa menyambungkannya sendiri:
- `RegisterModule("git-pipelines")` di `init()` daemon (registrasi mandiri; wiring hanya memverifikasi dan menjawab IPC).
- IPC `git.pipelines.list` dan `git.pipeline.get` untuk dipanggil `pipelineClient.ts` (UI) dan `shikigamictl git pipelines|pipeline` (ctl).
- Fungsi Go `gitdata.ListPipelines`, `gitdata.GetPipeline`, dan empat parser privat untuk verifikasi fixture (`glab-pipeline-list.json`, `glab-pipeline-get.json`, `glab-pipeline-get-nojobs.json`, `gh-run-list.json`, `gh-run-view.json`) lewat skrip sementara di scratchpad oleh wiring, bukan di repo.

## Selesai bila

- Kedua file ada dan hanya keduanya yang berubah (`git status` menunjukkan hanya dua path ini untuk fitur ini).
- `go build ./...` dari akar repo sukses (guard tunggal; tanpa unit test).
- Semua signature persis seperti bagian Kontrak; `ListPipelines` mengembalikan slice non-nil, maks 20, urut `UpdatedAt` turun; `GetPipeline` menolak id non-digit dengan `invalid_params`.
- GitLab: stage diturunkan dari job urut id naik; `jobs: null` -> `stages: []`; output non-JSON pada `ci get` -> `not_found` "Pipeline tidak ditemukan". GitHub: satu stage bernama workflow atau "Jobs".
- Status hanya lewat `NormalizeGitLabStatus`/`NormalizeGitHubStatus`; tidak ada pemanggilan `RepoFlag`, `os/exec`, atau `connector.RunCLITimeout`; tidak ada token/output mentah CLI di galat.
- Selisih yang dicatat (jangan diperbaiki): `DurationSec` bertipe `int` tanpa `omitempty` sehingga 0 = tidak tersedia.
