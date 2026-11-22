# Plan: Data MR/PR gitdata dan IPC git.mrs.list

## Metadata

- Tiket: `git-connector-ticket-02-git-mrs` (`docs/tickets/git-connector-ticket-02-git-mrs.md`)
- Fitur: `git-mrs-data`
- ownedPaths: `internal/gitdata/mrs.go`, `cmd/shikigamid/wire_git_mrs.go`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; semua tipe, runner, allowlist, helper wire sudah disediakan tiket 1)
- Rujukan kontrak: `docs/plans/git-connector-ticket-02-git-mrs/overview.md` bagian 0, 2.1, 2.2, 2.3, 7 (baca, jangan diubah)

## Tujuan

Menyediakan data daftar MR (GitLab) dan PR (GitHub) ternormalisasi lewat `gitdata.ListMergeRequests(ctx, repo, state)` dan mengeksposnya sebagai IPC `git.mrs.list` yang mendaftar mandiri di daemon. Read-only, semua lewat `Run()`, tanpa panggilan per baris, galat berpesan Indonesia, tanpa panic.

## Path yang dimiliki

HANYA dua file baru ini yang boleh dibuat/disentuh:
- `internal/gitdata/mrs.go`
- `cmd/shikigamid/wire_git_mrs.go`

Jangan sentuh file wiring, `types.go`, `allowlist.go`, `runner.go`, `errors.go`, `repo.go`, `testdata/`, `docs/git/cli-findings.md`, `wire_git.go`, maupun path fitur lain (ctl, ui). Membaca dan memanggil helper di file itu boleh. Jangan memanggil `RepoFlag`. Jangan menambah unit test atau dependensi. Awali helper privat paket `gitdata` dengan `mr`/`parseMr`, dan di paket `main` daemon dengan `gitMrs`.

## Kontrak

Disediakan (bentuk persis di overview 2.1-2.3):
- `func ListMergeRequests(ctx context.Context, repo Repo, state string) ([]MergeRequest, error)` di paket `gitdata`. Slice non-nil, galat bertipe `*gitdata.Error`.
- IPC `git.mrs.list`: params `{workspaceId, projectId, state}` (`state` in `open|merged|closed`), hasil `{"mrs": MergeRequest[]}` (selalu array).

Dipakai (sudah ada, jangan diubah):
- `gitdata`: `Run`, `Repo`, `MergeRequest`, `Status*`, `NormalizeGitHubStatus(status, conclusion)`, `Error`, `CodeCLI`, `ErrorCode`, `providerLabel(tool)` ("glab"->"GitLab", "gh"->"GitHub"), `ProviderGitLab/ProviderGitHub`.
- `main` daemon (`wire_git.go`): `gitDecode(params, &v)`, `gitResolveRepo(d, workspaceID, projectID) (gitdata.Repo, error)`, `mapGitError(err)`, `gitErr(code string, ipcCode int, msg string) error`; `RegisterModule`, `Deps`, `ipc.Server.Register`, `ipc.ErrInvalid`.
- Allowlist sudah mengizinkan `glab mr list` (`--repo --output --per-page --merged --closed`) dan `gh pr list` (`--repo --state --limit --json`). Pakai hanya bentuk panjang. Jika ada flag ditolak (`not_allowed`), laporkan, jangan ubah allowlist.
- Fixture acuan (baca untuk bentuk JSON): `internal/gitdata/testdata/glab-mr-list.json` (nyata), `internal/gitdata/testdata/gh-pr-list.json` (asumsi).

## Langkah

### 1. Baca acuan
Baca `internal/gitdata/types.go`, `runner.go`, `errors.go`, `allowlist.go`, kedua fixture di atas, dan `cmd/shikigamid/wire_git.go` serta `wire_lark_tickets.go` (pola registrasi mandiri). Pastikan kode `invalid_params` bisa dibuat sebagai `ErrorCode("invalid_params")` tanpa mengubah `errors.go`.

### 2. `internal/gitdata/mrs.go` (package `gitdata`)
Import: `context`, `encoding/json`, `sort`, `strings`, `time`. Tanpa `os/exec`, tanpa `internal/workspace`.

Konstanta: `const mrLimit = 50`.

a. `func mrRepoArg(r Repo) string` (privat; DIPUTUSKAN di overview 0):
- GitLab: host `gitlab.com` -> `r.Path`; host lain -> `"https://" + r.Host + "/" + r.Path`.
- GitHub: host `github.com` -> `r.Path`; host lain -> `r.Host + "/" + r.Path`.
- Bandingkan host dengan `strings.EqualFold`.

b. `func ListMergeRequests(ctx context.Context, repo Repo, state string) ([]MergeRequest, error)`:
- Validasi `state` in `open|merged|closed`; selain itu `return []MergeRequest{}, &Error{Code: ErrorCode("invalid_params"), Message: "Status MR tidak dikenal: " + state}`.
- Switch `repo.Provider`: `ProviderGitLab` -> `mrListGitLab`, `ProviderGitHub` -> `mrListGitHub`, lainnya -> `&Error{Code: CodeCLI, Message: "Provider git tidak didukung"}`.
- Pada sukses pastikan hasil non-nil (`[]MergeRequest{}` bila kosong), lalu urutkan lewat `mrSortByUpdated`. Pada galat kembalikan `nil, err`.

c. GitLab `func mrListGitLab(ctx, repo, state) ([]MergeRequest, error)`:
- args: `"mr","list","--repo",mrRepoArg(repo),"--output","json","--per-page","50"`; tambah `"--merged"` bila state `merged`, `"--closed"` bila `closed`, tanpa flag bila `open`. Jangan `--all`.
- `out, err := Run(ctx, "glab", args...)`; bila err kembalikan apa adanya (sudah `*Error`).
- Trim output; kosong atau `null` -> `[]MergeRequest{}`.
- Unmarshal ke `[]struct` pointer-toleran dengan field: `IID int json:"iid"`, `Title`, `State`, `Draft bool`, `SourceBranch json:"source_branch"`, `TargetBranch json:"target_branch"`, `UpdatedAt json:"updated_at"`, `WebURL json:"web_url"`, `Author *struct{Username string json:"username"}`. Gagal unmarshal -> `&Error{Code: CodeCLI, Message: providerLabel("glab") + " mengembalikan data yang tidak dapat dibaca"}`.
- Normalisasi state lewat `mrNormGitLabState(s)`: `opened`->`open`, `merged`->`merged`, `closed`/`locked`->`closed`, lainnya -> `""` (elemen dibuang).
- Filter sabuk pengaman: buang elemen yang state ternormalisasinya != `state` diminta.
- `ReviewState` dan `PipelineStatus` dibiarkan kosong (spike: daftar GitLab tidak memuatnya).
- Author nil -> string kosong (tanpa panic).

d. GitHub `func mrListGitHub(ctx, repo, state) ([]MergeRequest, error)`:
- args: `"pr","list","--repo",mrRepoArg(repo),"--state",state,"--limit","50","--json","number,title,author,headRefName,baseRefName,state,isDraft,reviewDecision,statusCheckRollup,updatedAt,url"`.
- Kosong/`null` -> `[]`. Struct parse: `Number int`, `Title`, `Author *struct{Login string}`, `HeadRefName`, `BaseRefName`, `State`, `IsDraft bool`, `ReviewDecision`, `StatusCheckRollup []struct{TypeName string json:"__typename"; Status, Conclusion, State string}`, `UpdatedAt`, `URL`. JSON rusak -> galat `CodeCLI` "GitHub mengembalikan data yang tidak dapat dibaca".
- State: `strings.ToLower` dari `OPEN/MERGED/CLOSED`; di luar tiga nilai -> buang elemen. Filter sabuk pengaman sama seperti GitLab.
- `ReviewState`: `mrNormReview(s)` = lowercase untuk `APPROVED|CHANGES_REQUESTED|REVIEW_REQUIRED` (`approved|changes_requested|review_required`); `""` atau lainnya -> `""`.
- `PipelineStatus`: `mrAggregateChecks(checks)`.

e. `func mrAggregateChecks(...) Status` (agregasi, overview 2.2):
- Slice kosong -> `""` (tanpa badge).
- Per entri: bila `TypeName == "StatusContext"` (atau `Status` kosong dan `State` terisi) -> petakan `State` lowercase: `success`->success, `failure`/`error`->failed, `pending`/`expected`->pending, lainnya unknown. Selain itu `NormalizeGitHubStatus(Status, Conclusion)`.
- Agregasi berurutan: ada `failed` -> `failed`; lalu ada `running` -> `running`; lalu ada `pending` -> `pending`; semua `success`/`skipped` dengan minimal satu `success` -> `success`; selain itu (mis. `canceled`, semua `skipped`, `unknown`) -> `unknown`.

f. `func mrSortByUpdated(list []MergeRequest)`: `sort.SliceStable` menurun menurut waktu `time.Parse(time.RFC3339, UpdatedAt)`; gagal parse diperlakukan paling lama (zero time). Catatan: `time.RFC3339` menerima pecahan detik (mis. `2026-11-20T14:36:25.415Z`).

g. Semua panggilan hanya lewat `Run(ctx, "glab"|"gh", ...)`; tidak ada panggilan per baris; tidak ada panic (tanpa indeks tanpa cek, pointer nil dicek).

### 3. `cmd/shikigamid/wire_git_mrs.go` (package `main`)
Import: `context`, `encoding/json`, `strings`, `gitdata`, `ipc`.

```go
func init() {
    RegisterModule("git-mrs", func(srv *ipc.Server, d *Deps) { registerGitMrs(srv, d) })
}

func registerGitMrs(srv *ipc.Server, d *Deps) {
    srv.Register("git.mrs.list", func(ctx context.Context, params json.RawMessage) (any, error) { ... })
}
```
Isi handler:
1. `var p struct{ WorkspaceID string json:"workspaceId"; ProjectID string json:"projectId"; State string json:"state" }`; `gitDecode(params, &p)` (return err bila ada).
2. Validasi: trim `WorkspaceID`/`ProjectID` kosong -> `gitErr("invalid_params", ipc.ErrInvalid, "workspaceId wajib diisi")` / `"projectId wajib diisi"`. `State` bukan `open|merged|closed` -> `gitErr("invalid_params", ipc.ErrInvalid, "state harus open, merged, atau closed")`.
3. `repo, err := gitResolveRepo(d, p.WorkspaceID, p.ProjectID)`; err -> return `nil, err` (sudah berbentuk `*ipc.Error`; boleh dibungkus `mapGitError`).
4. `mrs, err := gitdata.ListMergeRequests(ctx, repo, p.State)`; err -> `nil, mapGitError(err)`.
5. `if mrs == nil { mrs = []gitdata.MergeRequest{} }`; `return map[string]any{"mrs": mrs}, nil`.
Tanpa state bersama (tiap panggilan independen). Nama modul `git-mrs` harus unik; verifikasi dengan grep `RegisterModule(` di `cmd/shikigamid/`.

### 4. Guard
Dari akar repo: `gofmt -l internal/gitdata cmd/shikigamid` (harus kosong) lalu `go build ./...`. Perbaiki sampai lulus. Jangan menjalankan atau menulis unit test.

## Perubahan

- Dibuat: `internal/gitdata/mrs.go`
- Dibuat: `cmd/shikigamid/wire_git_mrs.go`
- Tidak ada file lain yang diubah.

## Wiring

Fitur ini tidak menyambungkan apa pun sendiri. Yang diekspos untuk langkah wiring (`git-mrs-wiring`):
- Modul daemon `git-mrs` terdaftar mandiri lewat `init()` + `RegisterModule`, sehingga IPC `git.mrs.list` otomatis tersedia saat daemon dibangun (tanpa edit file lain).
- Fungsi `gitdata.ListMergeRequests` untuk dipakai fitur lain bila perlu.
- Konsumen IPC (ctl `shikigamictl git mrs`, UI `mrClient.ts`) memakai kontrak overview 2.1; verifikasi end-to-end dengan data nyata `glab` dilakukan oleh wiring.
- Butir untuk dilaporkan ke wiring bila relevan: self-hosted GitLab memakai URL penuh lewat `mrRepoArg`; flag yang ditolak allowlist (bila ada).

## Selesai bila

- [ ] `internal/gitdata/mrs.go` dan `cmd/shikigamid/wire_git_mrs.go` ada, hanya dua file itu yang berubah (`git status` menunjukkan hanya keduanya untuk fitur ini).
- [ ] `ListMergeRequests` mendukung GitLab (`--merged`/`--closed`/tanpa flag, `--per-page 50`) dan GitHub (`--state`, `--limit 50`, `--json` daftar field), semua lewat `Run()`; `mrRepoArg` sesuai aturan self-hosted; tanpa `RepoFlag`, `os/exec`, atau `internal/workspace`.
- [ ] State hasil hanya `open|merged|closed`; `reviewState` hanya GitHub dan lowercase snake_case; `pipelineStatus` hanya GitHub, anggota himpunan Status atau kosong; hasil urut `updatedAt` menurun; slice tidak pernah nil.
- [ ] Handler `git.mrs.list` memvalidasi params, memakai `gitDecode`/`gitResolveRepo`/`mapGitError`, mengembalikan `{mrs: [...]}` (selalu array); galat berpesan Indonesia, tanpa panic.
- [ ] Guard: `go build ./...` sukses (tanpa unit test).
