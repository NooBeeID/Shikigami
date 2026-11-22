# Plan: Data detail MR/PR dan IPC git.mr.get

## Metadata

- Tiket: `git-connector-ticket-04-git-mr-detail` (`docs/tickets/git-connector-ticket-04-git-mr-detail.md`)
- Fitur: `git-mr-detail-data`
- ownedPaths:
  - `internal/gitdata/mrdetail.go` (baru)
  - `cmd/shikigamid/wire_git_mr_detail.go` (baru)
  - `internal/gitdata/testdata/` (hanya file fixture detail MR BARU, opsional; nama berawalan `glab-mr-view-` atau `gh-pr-view-`; jangan ubah file yang sudah ada)
- Depends on foundation: tidak (tidak ada `00-foundation.md`; semua tipe, runner, allowlist, helper wiring sudah ada dari tiket 1 sampai 3)
- Guard: `go build ./...` (tanpa unit test)
- Kontrak bersama: `docs/plans/git-connector-ticket-04-git-mr-detail/overview.md` (bagian 0, 2.1, 2.2, 2.3, 7). Baca bagian itu bila perlu; jangan diubah.

## Tujuan

Menyediakan `gitdata.GetMergeRequest(ctx, repo, number)` yang memanggil `glab mr view` / `gh pr view` lalu menormalisasi ke `MergeRequestDetail`, dan mendaftarkan IPC `git.mr.get` sebagai modul mandiri `git-mr-detail` di daemon. Read-only. Field hilang tidak membuat galat atau panic.

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `internal/gitdata/mrdetail.go`
- `cmd/shikigamid/wire_git_mr_detail.go`
- file fixture baru di `internal/gitdata/testdata/` (opsional)

Dilarang menyentuh: `mrs.go`, `pipelines.go`, `types.go`, `allowlist.go`, `runner.go`, `errors.go`, `repo.go`, `wire_git.go`, `wire_git_mrs.go`, `wire_git_pipelines.go`, fixture yang sudah ada, `cmd/shikigamictl/*`, `frontend/*`, file wiring, dan path fitur lain. Membaca dan memanggil helper di file-file itu boleh.

## Kontrak

Disediakan (bentuk persis, lihat overview 2.1 sampai 2.3):
```go
// internal/gitdata/mrdetail.go, package gitdata
func GetMergeRequest(ctx context.Context, repo Repo, number int) (MergeRequestDetail, error)
func parseGitLabMrDetail(data []byte) (MergeRequestDetail, error)
func parseGitHubPrDetail(data []byte) (MergeRequestDetail, error)
```
IPC `git.mr.get`: params `{workspaceId, projectId, number}`; hasil objek `MergeRequestDetail` langsung (camelCase, `labels`/`reviewers` selalu array). Galat lewat `mapGitError`.

Dipakai (jangan diubah, paket sama `gitdata`):
- Dari `mrs.go`: `mrRepoArg(Repo) string`, `mrNormGitLabState(string) string`, `mrNormReview(string) string`, `mrAggregateChecks([]mrCheck) Status`, tipe `mrCheck{TypeName, Status, Conclusion, State}` (json `__typename`, `status`, `conclusion`, `state`).
- Dari `types.go`: `MergeRequest`, `MergeRequestDetail`, `Reviewer`, `PipelineRef`, `Repo`, `Provider*`, `Status*`, `NormalizeGitLabStatus(s) Status`, `NormalizeGitHubStatus(status, conclusion) Status`.
- Dari `runner.go`: `Run(ctx, tool string, args ...string) ([]byte, error)`.
- Dari `errors.go`: `*Error{Code, Message}`, `CodeNotFound`, `CodeCLI`, `providerLabel(tool)`; kode lokal `ErrorCode("invalid_params")`.
- Dari `cmd/shikigamid/wire_git.go`: `gitErr(code, ipcCode, msg)`, `mapGitError(err)`, `gitDecode(params, &v)`, `gitResolveRepo(d, workspaceID, projectID) (gitdata.Repo, error)`. Contoh pola: `wire_git_pipelines.go`.

Konvensi nama simbol baru: paket `gitdata` berawalan `mrDetail` (tipe/helper) atau tepat dua parser di atas; hindari awalan `parseGit` lain, `pipeline*`, dan nama yang sudah ada di `mrs.go` (`mrCheck`, `mrRepoArg`, `mrNorm*`, `mrList*`, `mrSortByUpdated`, `mrLimit`, `mrAggregateChecks`). Paket `main` berawalan `gitMrDetail`.

## Langkah

### 1. Verifikasi awal (baca saja)
- Baca `internal/gitdata/mrs.go`, `types.go`, `errors.go`, `pipelines.go` (untuk gaya galat/not found), `cmd/shikigamid/wire_git.go` dan `wire_git_pipelines.go`.
- Lihat `internal/gitdata/testdata/glab-mr-view.json` dan `gh-pr-view.json` untuk bentuk field (khususnya `head_pipeline`, `pipeline`, `detailed_merge_status`, `has_conflicts`, `reviewRequests`, `latestReviews`, `statusCheckRollup[].detailsUrl`).
- Pastikan `allowlist.go` sudah mengizinkan `glab mr view` (`--repo`, `--output`) dan `gh pr view` (`--repo`, `--json`). Bila ternyata tidak, JANGAN ubah; laporkan di hasil.

### 2. `internal/gitdata/mrdetail.go`: `GetMergeRequest`
Import: `context`, `encoding/json`, `regexp`, `strconv`, `strings`.

```go
func GetMergeRequest(ctx context.Context, repo Repo, number int) (MergeRequestDetail, error)
```
- `number <= 0` -> `MergeRequestDetail{}` + `&Error{Code: ErrorCode("invalid_params"), Message: "Nomor MR tidak valid"}`.
- `switch repo.Provider`:
  - `ProviderGitLab`: `Run(ctx, "glab", "mr", "view", strconv.Itoa(number), "--repo", mrRepoArg(repo), "--output", "json")`; sukses -> `parseGitLabMrDetail(out)`.
  - `ProviderGitHub`: `Run(ctx, "gh", "pr", "view", strconv.Itoa(number), "--repo", mrRepoArg(repo), "--json", "number,title,body,author,headRefName,baseRefName,state,isDraft,reviewDecision,reviewRequests,latestReviews,labels,mergeable,statusCheckRollup,updatedAt,url")`; sukses -> `parseGitHubPrDetail(out)`.
  - default: `&Error{Code: CodeCLI, Message: "Provider git tidak didukung"}`.
- Galat `Run` dikembalikan apa adanya (sudah `*Error`). Dilarang `os/exec`, `-c/--comments`, `--web`, `api`.
- Pastikan hasil akhir (juga bila parser sukses) `Labels` dan `Reviewers` non-nil (helper `mrDetailEnsure(d *MergeRequestDetail)` yang mengisi `[]string{}` / `[]Reviewer{}` bila nil), dipanggil di dalam kedua parser.

### 3. `parseGitLabMrDetail(data []byte) (MergeRequestDetail, error)`
- `trimmed := strings.TrimSpace(string(data))`. Kosong, `null`, atau tidak diawali `{` -> `&Error{Code: CodeNotFound, Message: "MR tidak ditemukan"}`. `json.Unmarshal` gagal -> `&Error{Code: CodeCLI, Message: providerLabel("glab") + " mengembalikan data yang tidak dapat dibaca"}`.
- Struct mentah `mrDetailGitLabRaw` (semua pointer/nilai toleran): `IID int "iid"`, `Title`, `State`, `Draft bool`, `SourceBranch`, `TargetBranch`, `UpdatedAt "updated_at"`, `WebURL "web_url"`, `Description`, `HasConflicts bool "has_conflicts"`, `DetailedMergeStatus "detailed_merge_status"`, `Author *struct{Username string "username"}`, `Labels []json.RawMessage "labels"`, `Reviewers []struct{Username string "username"} "reviewers"` (elemen non-objek bisa membuat Unmarshal gagal; gunakan `[]json.RawMessage` lalu decode per elemen bila ingin lebih toleran), `HeadPipeline *mrDetailGitLabPipe "head_pipeline"`, `Pipeline *mrDetailGitLabPipe "pipeline"`. Tipe `mrDetailGitLabPipe{ID json.Number/RawMessage "id"; Status string "status"; WebURL string "web_url"}`. Karena `id` angka, decode lewat `json.Number` atau `json.RawMessage` lalu konversi ke string (terima angka dan string; hilang/0/kosong = tidak bermakna).
- Pemetaan:
  - `MergeRequest`: `Number=iid`, `Title`, `Author=author.username` (nil -> ""), `SourceBranch`, `TargetBranch`, `State=mrNormGitLabState(state)` (tak dikenal tetap `""`, bukan galat), `Draft`, `UpdatedAt`, `URL=web_url`. `ReviewState` dibiarkan kosong.
  - `Description`.
  - `Labels`: untuk tiap elemen RawMessage, hanya yang JSON string diambil (abaikan objek/lainnya), trim, lewati kosong.
  - `Reviewers`: tiap `username` tak kosong -> `Reviewer{Name: username, State: "pending"}`.
  - `Mergeable` via helper `mrDetailGitLabMergeable(hasConflicts bool, detailed string) string`: `hasConflicts` -> `"conflict"`; lalu `strings.ToLower(TrimSpace(detailed))`: `"mergeable"` -> `"mergeable"`; `""` atau `"not_open"` -> `""`; lainnya -> `"unknown"`.
  - `LastPipeline`: pilih `head_pipeline` bila punya id bermakna, selain itu `pipeline` bila punya id bermakna, selain itu nil. `&PipelineRef{ID: idString, Status: NormalizeGitLabStatus(status), URL: web_url}`. Bila `LastPipeline != nil`, set `PipelineStatus = LastPipeline.Status`.
- Panggil `mrDetailEnsure`, kembalikan.

### 4. `parseGitHubPrDetail(data []byte) (MergeRequestDetail, error)`
- Pengecekan kosong/`null`/bukan objek -> `&Error{Code: CodeNotFound, Message: "PR tidak ditemukan"}`; JSON rusak -> `&Error{Code: CodeCLI, Message: providerLabel("gh") + " mengembalikan data yang tidak dapat dibaca"}`.
- Struct mentah `mrDetailGitHubRaw`: `Number int`, `Title`, `Body`, `State`, `IsDraft bool`, `ReviewDecision`, `HeadRefName`, `BaseRefName`, `Mergeable`, `UpdatedAt`, `URL`, `Author *struct{Login string}`, `Labels []struct{Name string}`, `ReviewRequests []struct{Login, Slug, Name string}` (json `login`, `slug`, `name`), `LatestReviews []struct{Author *struct{Login string "login"} "author"; State string "state"}`, `StatusCheckRollup []mrDetailGitHubCheck`. Untuk `mrAggregateChecks` butuh `[]mrCheck`; tambah field `DetailsURL string "detailsUrl"` dengan tipe sendiri `mrDetailGitHubCheck` yang meng-embed `mrCheck` (embed struct di-flatten oleh encoding/json) plus `DetailsURL`, lalu bangun `[]mrCheck` untuk agregasi.
- Pemetaan:
  - Dasar: `Number`, `Title`, `Author=author.login`, `SourceBranch=headRefName`, `TargetBranch=baseRefName`, `State=strings.ToLower(TrimSpace(state))` (hanya `open|merged|closed`, selain itu `""`), `Draft=isDraft`, `ReviewState=mrNormReview(reviewDecision)`, `PipelineStatus=mrAggregateChecks(checks)`, `UpdatedAt`, `URL`.
  - `Description=body`; `Labels` dari `labels[].name` (tak kosong).
  - `Reviewers` via helper `mrDetailGitHubReviewers(latest, requests)`: pertama iterasi `latestReviews` (nama `author.login`, lewati kosong): `APPROVED` -> `approved`, `CHANGES_REQUESTED` -> `changes_requested`, lainnya (`COMMENTED`, `DISMISSED`, `PENDING`, kosong) -> `pending`; lalu `reviewRequests` (nama = `login`, fallback `slug`, fallback `name`; state `pending`) yang namanya belum muncul. Tanpa duplikat (pakai `map[string]bool`); nama di kedua daftar memakai state dari review.
  - `Mergeable` via `mrDetailGitHubMergeable(state, mergeable string) string`: bila `State != "open"` -> `""`; `MERGEABLE` -> `mergeable`; `CONFLICTING` -> `conflict`; `UNKNOWN` -> `unknown`; kosong/lainnya -> `""` (nilai lain tak dikenal boleh `unknown`; ikuti overview: kosong -> kosong). `mergeStateStatus` diabaikan.
  - `LastPipeline` via helper `mrDetailGitHubLastPipeline(checks []mrDetailGitHubCheck, status Status) *PipelineRef`: regexp paket `mrDetailRunPattern = regexp.MustCompile(`^(.*?/actions/runs/([0-9]+))`)`; iterasi checks berurutan, pada `detailsUrl` pertama yang cocok kembalikan `&PipelineRef{ID: m[2], Status: status, URL: m[1]}` (Status = hasil `mrAggregateChecks`). Tidak ada yang cocok -> nil.
- `mrDetailEnsure`, kembalikan.

### 5. `cmd/shikigamid/wire_git_mr_detail.go` (package `main`)
Import: `context`, `encoding/json`, `strings`, `internal/gitdata`, `internal/ipc` (module path `github.com/NooBeeID/shikigami`).

```go
func init() {
    RegisterModule("git-mr-detail", func(srv *ipc.Server, d *Deps) { registerGitMrDetail(srv, d) })
}

func registerGitMrDetail(srv *ipc.Server, d *Deps) {
    srv.Register("git.mr.get", func(ctx context.Context, params json.RawMessage) (any, error) { ... })
}
```
Handler, mengikuti pola `git.pipeline.get`:
1. `var p struct{ WorkspaceID string `json:"workspaceId"`; ProjectID string `json:"projectId"`; Number int `json:"number"` }`; `gitDecode(params, &p)` (kembalikan galatnya langsung).
2. `workspaceId` kosong -> `gitErr("invalid_params", ipc.ErrInvalid, "workspaceId wajib diisi")`; `projectId` kosong -> idem "projectId wajib diisi"; `p.Number <= 0` -> `gitErr("invalid_params", ipc.ErrInvalid, "number wajib berupa bilangan bulat lebih dari 0")`.
3. `repo, err := gitResolveRepo(d, p.WorkspaceID, p.ProjectID)`; err -> `mapGitError(err)`.
4. `det, err := gitdata.GetMergeRequest(ctx, repo, p.Number)`; err -> `mapGitError(err)`.
5. Pastikan `det.Labels`/`det.Reviewers` non-nil (isi slice kosong), kembalikan `det` (objek langsung, bukan pembungkus).
Tidak mengedit `wire_git.go`; nama modul `git-mr-detail` unik (terpakai: `git`, `git-mrs`, `git-pipelines`). Bila perlu helper, awalan `gitMrDetail`.

### 6. Fixture baru (opsional)
Bila ingin memverifikasi parser dengan data yang melebihi `gh-pr-view.json` (mis. review campuran, PR tertutup, tanpa `statusCheckRollup`), tambahkan file BARU `internal/gitdata/testdata/gh-pr-view-<varian>.json` / `glab-mr-view-<varian>.json`. Jangan edit fixture lama atau `testdata/README.md`. Tanpa unit test di repo.

### 7. Build dan verifikasi
- `go build ./...` dari akar repo harus lulus; jalankan juga `go vet ./internal/gitdata/ ./cmd/shikigamid/` bila cepat.
- Verifikasi parser lewat skrip `go run` SEMENTARA di scratchpad (bukan di repo). Karena parser privat, skrip tidak bisa memanggilnya dari luar paket; opsi: salin sementara file uji `_test`-like ke scratchpad tidak mungkin, jadi cukup tulis di scratchpad satu program yang meng-copy `mrdetail.go` + stub minimal, ATAU (lebih sederhana) pakai `go test` sementara di scratchpad dengan menyalin paket ke direktori sementara. Jangan meninggalkan file uji di repo. Periksa: `glab-mr-view.json` menghasilkan Number 32, State `merged`, `Reviewers=[{dev-alpha pending}]`, `Mergeable=""`, `LastPipeline.ID="3001"`; `gh-pr-view.json` menghasilkan reviewer ter-merge tanpa duplikat, `LastPipeline` dari run id pertama (atau nil bila tidak ada), `Labels`/`Reviewers` non-nil; input `{}`/field hilang tidak panic.
- Bila daemon berjalan dan ada project GitLab nyata, coba `shikigamictl git mr <ws> <proj> <n>` (milik fitur lain; hanya bila sudah ada). Jangan `pkill` pola luas; matikan hanya proses yang Anda luncurkan sendiri.

## Perubahan

Dibuat:
- `internal/gitdata/mrdetail.go`
- `cmd/shikigamid/wire_git_mr_detail.go`
- (opsional) file fixture baru di `internal/gitdata/testdata/`

Diubah: tidak ada.

## Wiring

Fitur ini sepenuhnya mandiri; JANGAN menyambungkan apa pun sendiri. Yang diekspos untuk langkah wiring (`git-mr-detail-wiring`):
- Modul daemon `git-mr-detail` terdaftar lewat `init()` + `RegisterModule`, dengan metode IPC `git.mr.get` (params `{workspaceId, projectId, number}`, hasil `MergeRequestDetail`).
- `gitdata.GetMergeRequest(ctx, repo, number)` sebagai API Go.
Wiring nanti memeriksa: tidak ada bentrok nama modul (`grep -rn 'RegisterModule("git' cmd/shikigamid`), `wire_git.go` tidak berubah, ctl `git mr` dan `MrPane` memakai kontrak yang sama.

## Selesai bila

- [ ] `internal/gitdata/mrdetail.go` memuat `GetMergeRequest`, `parseGitLabMrDetail`, `parseGitHubPrDetail` dengan signature persis di atas; hanya `Run(ctx, "glab"|"gh", ...)`, tanpa `os/exec`, tanpa impor `internal/workspace`.
- [ ] Perintah memakai hanya `--repo`, `--output json` (GitLab) dan `--repo`, `--json <fields>` (GitHub); tidak ada perintah tulis.
- [ ] GitLab: reviewer `pending`; `Mergeable` mengikuti aturan (`has_conflicts` -> conflict, `mergeable`, `not_open`/kosong -> kosong, lainnya unknown); `LastPipeline` dari `head_pipeline` lalu `pipeline`, nil bila tidak ada id.
- [ ] GitHub: reviewer gabungan `latestReviews` + `reviewRequests` tanpa duplikat; `Mergeable` dinormalisasi dan dikosongkan untuk state bukan `open`; `LastPipeline` dari run id pertama di `detailsUrl`, nil bila tidak ada.
- [ ] `Labels` dan `Reviewers` selalu non-nil; field hilang/null tidak menyebabkan galat atau panic; output kosong/`null` -> `not_found` ("MR tidak ditemukan"/"PR tidak ditemukan"); JSON rusak -> `cli_error` berpesan Indonesia.
- [ ] `cmd/shikigamid/wire_git_mr_detail.go` mendaftarkan modul `git-mr-detail` dengan `git.mr.get`; `workspaceId`/`projectId` kosong dan `number <= 0` -> `invalid_params`; memakai `gitDecode`, `gitResolveRepo`, `mapGitError`.
- [ ] Tidak ada file di luar ownedPaths yang berubah (`git status` hanya menampilkan dua file Go baru dan paling banyak fixture baru).
- [ ] Guard: `go build ./...` sukses. Tanpa unit test.
