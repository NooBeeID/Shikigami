# Overview: git-connector-ticket-04-git-mr-detail (Detail MR/PR)

Sumber: tiket `docs/tickets/git-connector-ticket-04-git-mr-detail.md` dan spec `docs/superpowers/specs/2026-11-22-10-git-connector-design.md`. Dokumen ini adalah sumber kebenaran kontrak bersama; plan per fitur hanya merinci langkah kerja. Module Go: `github.com/NooBeeID/shikigami`. Bila overview berselisih dengan tiket, tiket menang kecuali dinyatakan lain di sini (selisih yang disengaja ditandai "DIPUTUSKAN").

## 0. Keadaan kode saat ini (diverifikasi dari repo; tiket 1 sampai 3 sudah merge)

Backend (`internal/gitdata/`, sudah ada, TIDAK boleh diubah):
- `types.go` sudah memuat `MergeRequest`, `Reviewer{Name, State (omitempty)}`, `PipelineRef{ID string, Status Status, URL (omitempty)}`, dan `MergeRequestDetail{MergeRequest; Description string; Labels []string; Reviewers []Reviewer; Mergeable string (omitempty); LastPipeline *PipelineRef (omitempty)}`, konstanta `Status*`, `NormalizeGitLabStatus(s)`, `NormalizeGitHubStatus(status, conclusion)`. Jangan mendefinisikan ulang. JSON camelCase.
- `runner.go`: `Run(ctx, tool, args...) ([]byte, error)` (allowlist, timeout 20 detik, maks 3 bersamaan, galat sudah `*gitdata.Error`). Jangan memakai `os/exec` atau `connector.RunCLITimeout` langsung.
- `errors.go`: `*Error{Code, Message}`, `AsError`, kode `cli_missing|not_logged_in|permission|not_found|timeout|not_allowed|cli_error`, helper privat `providerLabel(tool)`. Kode lokal `invalid_params` legal (`ErrorCode("invalid_params")`, seperti di `mrs.go`).
- `allowlist.go` SUDAH mengizinkan `glab mr view` (flag `-R --repo -F --output -c --comments`) dan `gh pr view` (flag `-R --repo --json -c --comments`). Pakai HANYA `--repo`, `--output`, `--json` (bentuk panjang). `-c/--comments`, `--web`, `api` tidak boleh dipakai. Allowlist tidak perlu diubah; bila kurang, laporkan.
- `mrs.go` (milik tiket 2, TIDAK boleh diubah) punya helper privat sepaket yang BOLEH dipanggil (paket sama, tanpa mengubah file): `mrRepoArg(r Repo) string` (GitLab `gitlab.com` -> `r.Path`, host lain -> `https://host/path`; GitHub `github.com` -> `r.Path`, host lain -> `host/path`), `mrNormGitLabState(s) string` (`opened|merged|closed|locked` -> `open|merged|closed`, selain itu `""`), `mrNormReview(s) string`, `mrAggregateChecks([]mrCheck) Status`, tipe `mrCheck{TypeName, Status, Conclusion, State}`. `repo.go` `RepoFlag` TIDAK andal untuk glab self-hosted; jangan dipakai.
- `testdata/` (dilarang diubah): `glab-mr-view.json` (NYATA; MR merged, `reviewers[]` hanya data user, `detailed_merge_status: "not_open"`, `has_conflicts: false`, `head_pipeline{id:3001,status,web_url}`), `gh-pr-view.json` (ASUMSI; `body`, `labels[].name`, `reviewRequests[].login`, `latestReviews[].author.login/state`, `mergeable`, `mergeStateStatus`, `statusCheckRollup[]` dengan `detailsUrl` ke `.../actions/runs/<runId>/job/<jobId>`). Fixture baru boleh ditambah sebagai file baru.

Temuan spike (`docs/git/cli-findings.md` 3.2, 3.5, 10) yang mengikat:
- GitLab `reviewers[]` TIDAK punya state review. DIPUTUSKAN (selaras tiket): `Reviewer.State` GitLab = `"pending"` (bukan kosong).
- `glab mr view` memuat `head_pipeline` (id angka, status, web_url; `null` bila tidak ada) dan `pipeline` (fallback bila `head_pipeline` null).
- DIPUTUSKAN (selisih dengan temuan 10, ikut tiket): `Mergeable` DINORMALISASI ke `mergeable | conflict | unknown`, atau kosong (dihilangkan). GitLab: `has_conflicts == true` -> `conflict` (prioritas); `detailed_merge_status == "mergeable"` -> `mergeable`; `"not_open"` atau kosong -> kosong; nilai lain (`unchecked`, `checking`, `ci_must_pass`, `not_approved`, `discussions_not_resolved`, `draft_status`, `need_rebase`, dst.) -> `unknown`. GitHub: `MERGEABLE` -> `mergeable`, `CONFLICTING` -> `conflict`, `UNKNOWN` -> `unknown`, kosong -> kosong (`mergeStateStatus` boleh diabaikan). Untuk MR berstate `merged`/`closed` di GitLab hasilnya kosong lewat `not_open`; di GitHub `mergeable` untuk PR tertutup dikosongkan juga (state bukan `open` -> kosong).
- GitHub `lastPipeline` (ASUMSI): dari `statusCheckRollup`; cari `detailsUrl` bermotif `.../actions/runs/<digits>`; `ID` = digit run PERTAMA yang ditemukan, `URL` = `detailsUrl` dipotong sampai `/actions/runs/<id>`, `Status` = `mrAggregateChecks(rollup)`. Tidak ada id bermakna (rollup kosong atau tanpa `detailsUrl` bermotif) -> `LastPipeline` nil. Karena id adalah run GitHub, klik di UI membuka pane `git-pipeline` yang memanggil `gh run view <id>` (valid).

Frontend (`frontend/src/modules/git/`, sudah ada):
- `client.ts`: `Provider`, `Repo`, `GitError {code, message}`, `parseGitError(x)`.
- `mrClient.ts`: tipe `MergeRequest` (diekspor; `state: MergeRequestState`, `reviewState?`, `pipelineStatus?` string), pola klien `call<...>(..., params)` dari `../../lib/ipc` + `throw parseGitError(e)`. Dipakai ulang sebagai tipe dasar; JANGAN diubah.
- `actions.ts`: `openInBrowser(url)`, `copyLink(url)`, `sendToAgent(url)` (toast "Tidak ada sesi aktif" sudah di dalamnya).
- `StatusBadge.svelte`: props `{status: string; kind?: 'pipeline'|'review'|'mr'}`. Tabel `mr`: open/merged/closed (Terbuka/Digabung/Tertutup); `review`: `approved`, `changes_requested`, `review_required` (Disetujui/Perlu perubahan/Belum direview); `pipeline`: pending..unknown. Status review `pending` TIDAK ada di tabel `review`, jadi `ReviewerList` merender label sendiri (lihat 2.5), bukan lewat `StatusBadge kind="review"`.
- `PipelinePane.svelte` (selesai di tiket 3) membaca `pane.provider`, `pane.projectId`, `pane.pipelineId`. DIPUTUSKAN (selisih tiket yang menyebut `id`): field leaf id pipeline bernama `pipelineId`.
- Cara membuka pane (pola `MrList.openMr` dan `PipelineList`): `layout.openPane({ kind: 'git-pipeline', provider, projectId, pipelineId, title })` dengan `layout` dari `../../lib/stores/layout`. `MrList.openMr` SUDAH membuka `git-mr` dengan `provider`, `projectId`, `number`, `title` (`!12 judul`/`#12 judul`).
- Pola galat/muat/seq: ikuti `PipelinePane.svelte` (fase `loading|ready|error|missing`, `seq` + `$effect` dengan `untrack`, cleanup `seq++`, `activeWorkspaceId` lewat `get()` dari `../../lib/stores/workspaces`, `openSettings('connectors')` dari `../settings` untuk `cli_missing`/`not_logged_in`, tombol "Coba lagi").
- `MrPane.svelte` saat ini placeholder dengan props `{ pane: Leaf; visible: boolean }` dan pembungkus `<div class="git-pane" hidden={!visible}>`. Props TIDAK berubah. Terdaftar di `index.ts` sebagai kind `git-mr` (tidak boleh diubah).
- Daemon (`cmd/shikigamid`): helper paket `main` sudah ada di `wire_git.go`: `gitErr(code, ipcCode, msg)`, `mapGitError(err)`, `gitDecode(params, &v)`, `gitResolveRepo(d, workspaceID, projectID)`. Pola modul mandiri: `RegisterModule("git-pipelines", func(srv *ipc.Server, d *Deps){...})` di `init()` (lihat `wire_git_pipelines.go`). Nama modul terpakai: `git`, `git-mrs`, `git-pipelines`.
- ctl (`cmd/shikigamictl`): `registerGitSub(name, fn)` dengan `fn func(ctx context.Context, c *ipc.Client, args []string) error`; `c.Call(method, params, &res) error`. Pola `cmd_git_mrs.go` (argumen posisional, usage, `gitMrsOneLine`).

## 1. Ringkasan epic dan urutan eksekusi

Epic mengisi pane tengah `git-mr` (placeholder dari tiket 1) dengan detail MR/PR nyata: judul, status, penulis, branch, label, reviewer + status, deskripsi (teks biasa), bisa-digabung/konflik, pipeline terakhir (klik membuka `git-pipeline`), aksi Buka/Salin/Kirim ke agen, Segarkan. Ditambah IPC `git.mr.get`, ctl `shikigamictl git mr`, lalu wiring dan verifikasi akhir SELURUH connector Git (tiket penutup). Read-only.

Urutan eksekusi:
1. Foundation: TIDAK ADA (`hasFoundation=false`, tanpa `00-foundation.md`). Semua tipe, runner, allowlist, fixture, `StatusBadge`, `actions.ts`, pane kind, field leaf, dan helper wiring sudah ada.
2. Fitur paralel (tiga agen): `git-mr-detail-data`, `git-mr-detail-ctl`, `git-mr-pane-ui`.
3. Wiring: `git-mr-detail-wiring` setelah tiga fitur selesai.

## 2. Kontrak bersama

### 2.1 IPC `git.mr.get`

Params: `{"workspaceId": string, "projectId": string, "number": number}`. `workspaceId`/`projectId` kosong -> `invalid_params` (`ipc.ErrInvalid`); `number` harus bilangan bulat > 0 (decode ke `int`; nol/negatif -> `invalid_params`, mencegah injeksi argumen CLI).

Hasil: OBJEK `MergeRequestDetail` langsung (bukan pembungkus), json camelCase. Slice SELALU array (`labels: []`, `reviewers: []`, tidak pernah `null`):
```ts
export type MergeableState = 'mergeable' | 'conflict' | 'unknown';
export type ReviewerState = 'approved' | 'changes_requested' | 'pending';
export interface MrReviewer { name: string; state: string }       // state: ReviewerState; toleran nilai lain/kosong
export interface MrLastPipeline { id: string; status: string; url?: string }  // status: PipelineStatus
export interface MergeRequestDetail extends MergeRequest {        // MergeRequest dari ./mrClient
  description: string;
  labels: string[];
  reviewers: MrReviewer[];
  mergeable?: MergeableState;      // hilang bila tidak ada
  lastPipeline?: MrLastPipeline;   // hilang bila tidak ada
}
```
Galat IPC: `"[kode] pesan Indonesia"` lewat `mapGitError` (not_found -> `ipc.ErrNotFound`, not_allowed -> `ipc.ErrInvalid`, lainnya `ipc.ErrInternal`). Tanpa output mentah CLI.

### 2.2 Go (`internal/gitdata/mrdetail.go`, package `gitdata`)

```go
func GetMergeRequest(ctx context.Context, repo Repo, number int) (MergeRequestDetail, error)
// parser murni (tanpa pemanggilan CLI), untuk fixture:
func parseGitLabMrDetail(data []byte) (MergeRequestDetail, error)
func parseGitHubPrDetail(data []byte) (MergeRequestDetail, error)
```
- DIPUTUSKAN (selisih kecil dengan tiket yang menulis `GetMergeRequest(ctx, ...)`): menerima `Repo` yang sudah diselesaikan, sama seperti `ListPipelines`/`GetPipeline`. Resolusi `projectId` -> `Repo` dilakukan di `wire_git_mr_detail.go` lewat `gitResolveRepo`, jadi `gitdata` tidak mengimpor `internal/workspace`.
- Perintah GitLab: `glab mr view <number> --repo <mrRepoArg> --output json` (`number` lewat `strconv.Itoa`). Perintah GitHub: `gh pr view <number> --repo <mrRepoArg> --json number,title,body,author,headRefName,baseRefName,state,isDraft,reviewDecision,reviewRequests,latestReviews,labels,mergeable,statusCheckRollup,updatedAt,url`.
- Hasil: `Labels` dan `Reviewers` non-nil; galat bertipe `*gitdata.Error`. Output kosong/`null`/bukan objek JSON -> `&Error{Code: CodeNotFound, Message: "MR tidak ditemukan"}` (GitHub: "PR tidak ditemukan"); JSON rusak -> `&Error{Code: CodeCLI, Message: providerLabel(tool) + " mengembalikan data yang tidak dapat dibaca"}`. Parser toleran: field hilang/null tidak panic.
- Pemetaan GitLab: bagian dasar seperti `mrListGitLab` (`iid`->Number, `title`, `author.username`, `source_branch`, `target_branch`, `mrNormGitLabState(state)` -> State, `draft`, `updated_at`, `web_url`); State tak dikenal dibiarkan `""` (bukan galat). `description` -> Description; `labels` (array string; elemen bisa objek bila bentuk lain, abaikan non-string) -> Labels; `reviewers[].username` -> `{Name, State: "pending"}`; `Mergeable` sesuai aturan bagian 0; `LastPipeline` dari `head_pipeline` lalu fallback `pipeline`: `id` angka -> string, `Status` = `NormalizeGitLabStatus`, `URL` = `web_url`; objek `null`/tanpa id -> nil. `PipelineStatus` (field dasar) diisi dari `LastPipeline.Status` bila ada (kolom opsional; boleh kosong). `ReviewState` GitLab kosong.
- Pemetaan GitHub: dasar seperti `mrListGitHub` (`author.login`, `headRefName`, `baseRefName`, `state` huruf kecil, `isDraft`, `mrNormReview(reviewDecision)` -> ReviewState, `PipelineStatus` = `mrAggregateChecks`, `updatedAt`, `url`); `body` -> Description; `labels[].name` -> Labels; Reviewers: gabungan `reviewRequests[]` (nama = `login`, fallback `slug`, fallback `name`; state `pending`) dan `latestReviews[]` (nama = `author.login`; `APPROVED` -> `approved`, `CHANGES_REQUESTED` -> `changes_requested`, `COMMENTED`/lainnya -> `pending`); nama yang ada di kedua daftar memakai state dari review; tanpa duplikat, urutan: `latestReviews` dulu lalu `reviewRequests` yang belum muncul. `Mergeable` dan `LastPipeline` sesuai bagian 0.
- Konstruksi struct JSON mentah dan helper baru memakai awalan `mrDetail`/`parseGitLabMr`/`parseGitHubPr` agar tidak bentrok dengan `mr*` tiket 2 (hindari nama persis `mrCheck`, `mrRepoArg`, dst. yang sudah ada) dan `pipeline*`/`parseGit*` tiket 3 (jangan memakai awalan `parseGit` selain dua parser bernama di atas).
- Hanya `Run(ctx, "glab"|"gh", ...)`; tanpa `os/exec`, tanpa import `internal/workspace`.

### 2.3 `cmd/shikigamid/wire_git_mr_detail.go`

Registrasi mandiri: `func init() { RegisterModule("git-mr-detail", func(srv *ipc.Server, d *Deps) { registerGitMrDetail(srv, d) }) }` (nama modul `git-mr-detail` harus unik). Handler `git.mr.get`: `gitDecode` ke struct `{WorkspaceID string; ProjectID string; Number int}`, validasi (2.1), `gitResolveRepo`, `gitdata.GetMergeRequest(ctx, repo, p.Number)`, `mapGitError`. Pastikan `Labels`/`Reviewers` non-nil sebelum dikembalikan. Helper privat berawalan `gitMrDetail`. Tidak mengedit `wire_git.go`.

### 2.4 `cmd/shikigamictl/cmd_git_mr_detail.go`

Registrasi mandiri: `func init() { registerGitSub("mr", gitMrDetail) }`. Bentuk posisional sesuai tiket (BEDA dari `git pipelines` yang berflag): `shikigamictl git mr <workspaceId> <projectId> <number>`. Usage: `penggunaan: shikigamictl git mr <workspaceId> <projectId> <number>`. Argumen != 3, kosong, atau `number` bukan bilangan bulat > 0 -> `errors.New(<usage>)`. Panggil `c.Call("git.mr.get", map[string]any{"workspaceId":..., "projectId":..., "number": n}, &res)` ke struct lokal berawalan `gitMrDetail` (jangan mengimpor `gitdata`). Galat daemon dikembalikan sebagai `error` (main sudah menangani exit non-nol); bila metode tidak dikenal daemon (galat method not found), bungkus pesan jelas: `daemon belum mendukung git.mr.get (perbarui shikigamid)`. Keluaran teks, satu informasi per baris: `!12|#12 <judul>` (prefiks `!` bila `url` memuat `/-/merge_requests/`, `#` bila `/pull/`), `status: <state>[ (draft)]`, `penulis: @<author>`, `branch: <source> -> <target>`, `label: a, b` (atau `-`), `reviewer: <nama> (<state>), ...` (atau `-`), `mergeable: <nilai>` (atau `-`), `pipeline: <id> <status> <url>` (atau `-`), `url: <url>`, baris kosong, lalu `deskripsi:` dan isi deskripsi apa adanya (tab/newline dalam deskripsi dipertahankan; field satu-baris dibersihkan dari tab/newline pola `gitMrsOneLine`, simbol baru berawalan `gitMrDetail`).

### 2.5 Frontend

`mrDetailClient.ts` (milik `git-mr-pane-ui`; SATU-SATUNYA pembungkus IPC detail MR). Mengekspor tipe `MergeableState`, `ReviewerState`, `MrReviewer`, `MrLastPipeline`, `MergeRequestDetail` (bentuk 2.1; `MergeRequest` diimpor `import type` dari `./mrClient`) dan:
```ts
export async function getMrDetail(workspaceId: string, projectId: string, number: number): Promise<MergeRequestDetail>;
```
Implementasi: `call<MergeRequestDetail>('git.mr.get', { workspaceId, projectId, number })` dari `../../lib/ipc`; try/catch lalu `throw parseGitError(e)`; jamin `labels`/`reviewers` array (`Array.isArray(x) ? x : []`) dan `description` string.

Komponen (Svelte 5 runes, `$props()`):
- `ReviewerList.svelte`: props `{ reviewers: MrReviewer[] }` (tipe `import type` dari `./mrDetailClient`). Daftar nama + label status: `approved` -> "Disetujui" (nada baik), `changes_requested` -> "Diminta perubahan" (nada buruk), selain itu (termasuk `pending`/kosong) -> "Belum" (netral/warn). Warna lewat variabel CSS (`--success`, `--danger`, `--warn`, `--fg-muted`) dengan fallback; boleh gaya lokal sederhana, bukan `StatusBadge kind="review"`. Kosong -> teks "Belum ada reviewer".
- `MrPane.svelte`: props `{ pane: Leaf; visible: boolean }` tetap; wrapper luar tetap `<div class="git-pane" hidden={!visible}>`. Memuat lewat `getMrDetail` saat mount/dipulihkan dan saat Segarkan, dengan pola `PipelinePane` (fase `loading|ready|error|missing`, `seq`, `$effect` bergantung pada `pane.provider`, `pane.projectId`, `pane.number` dengan `untrack`). `provider`/`projectId`/`number` kosong atau `activeWorkspaceId` null -> keadaan aman "Project atau MR tidak ditemukan" dengan tombol aksi nonaktif. Urutan isi dan teks sesuai tiket (Terbuka/Digabung/Tertutup lewat `StatusBadge kind="mr"` + penanda "Draf"; nomor `!12`/`#12` dari `pane.provider`; "Bisa digabung" untuk `mergeable`, "Konflik" untuk `conflict`, `unknown`/hilang disembunyikan; pipeline terakhir `StatusBadge kind="pipeline"` + `#id`, klik -> `layout.openPane({kind:'git-pipeline', provider: pane.provider, projectId: pane.projectId, pipelineId: lastPipeline.id, title: 'Pipeline ' + id})`; deskripsi `white-space: pre-wrap; overflow-wrap: anywhere`, tanpa `{@html}`, "Tidak ada deskripsi" bila kosong; label chip disembunyikan bila kosong). Aksi Buka/Salin/Kirim ke agen memakai `detail.url` (aktif hanya saat `ready` dan url ada). Waktu pembaruan: fungsi relatif Indonesia lokal ("baru saja", "3 jam lalu", "2 hari lalu") di file ini.
- Judul header: `detail.title` bila ada, sebelum data datang `pane.title`, fallback "Merge Request".

### 2.6 Konvensi

Teks UI bahasa Indonesia; identifier kode bahasa Inggris; tanpa dependensi baru; tanpa unit test; tanpa polling; tanpa token di config; hanya perintah baca lewat allowlist. Stile: variabel CSS yang dipakai tiket 1 sampai 3 (`--fg`, `--fg-muted`, `--bg-elev`, `--bg-hover`, `--border`, `--danger`, `--accent`) dengan fallback; warna status hanya lewat `StatusBadge` (kecuali `ReviewerList`, lihat 2.5). Jangan menambah file global atau komponen bersama baru di luar ownedPaths.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `git-mr-detail-data` | `gitdata.GetMergeRequest` (GitLab + GitHub, parser murni, normalisasi reviewer/mergeable/lastPipeline) dan IPC `git.mr.get` terdaftar mandiri; fixture detail baru bila perlu | `internal/gitdata/mrdetail.go`, `cmd/shikigamid/wire_git_mr_detail.go`, (fixture baru opsional di `internal/gitdata/testdata/`, nama file baru berawalan `glab-mr-view-`/`gh-pr-view-`) |
| `git-mr-detail-ctl` | `shikigamictl git mr <workspaceId> <projectId> <number>` (alat debug) | `cmd/shikigamictl/cmd_git_mr_detail.go` |
| `git-mr-pane-ui` | isi `MrPane`, `ReviewerList`, klien `mrDetailClient.ts` | `frontend/src/modules/git/MrPane.svelte`, `frontend/src/modules/git/ReviewerList.svelte`, `frontend/src/modules/git/mrDetailClient.ts` |
| `git-mr-detail-wiring` | wiring dan verifikasi akhir seluruh connector Git (tanpa file sendiri; dikerjakan setelah tiga fitur lain) | (tidak ada) |

Rincian (acceptance criteria lengkap di tiket; plan per fitur wajib memetakannya):
- `git-mr-detail-data`: sesuai 2.1 sampai 2.3. Galat CLI hilang/belum login/izin/repo atau MR tak ada/timeout/project tak dikenal -> galat IPC berpesan Indonesia, tanpa panic. Hanya perintah baca. Diverifikasi lewat `shikigamictl git mr` bila daemon berjalan, dan fixture untuk GitHub (skrip `go run` sementara di scratchpad, bukan di repo).
- `git-mr-detail-ctl`: sesuai 2.4; tidak mengubah `cmd_git.go`; kompilasi tanpa backend.
- `git-mr-pane-ui`: sesuai 2.5; dikembangkan terhadap kontrak 2.1; build penuh lulus bahkan tanpa fitur data selesai (hanya mengimpor `mrClient.ts` untuk tipe).

## 4. Foundation

Tidak ada. Alasan: tipe Go (`MergeRequestDetail`, `Reviewer`, `PipelineRef`), runner, allowlist (`glab mr view`, `gh pr view` sudah ada), fixture dasar, `StatusBadge`, `actions.ts`, pane kind `git-mr`/`git-pipeline`, field leaf `provider/projectId/number/pipelineId`, helper wiring daemon dan ctl sudah ada dari tiket 1 sampai 3 (diverifikasi). Output: `hasFoundation=false`, tanpa `00-foundation.md`.

## 5. Wiring checklist (untuk `git-mr-detail-wiring`)

Tanpa file milik sendiri; hanya verifikasi dan koreksi kecil di path epic ini (kekurangan di file tiket lain dilaporkan, bukan diubah):
1. `cmd/shikigamid/wire_git_mr_detail.go`: `RegisterModule("git-mr-detail", ...)` ada, nama tidak bentrok (`grep -rn 'RegisterModule("git' cmd/shikigamid`); `git.mr.get` terdaftar lewat `srv.Register`; `wire_git.go` tidak berubah.
2. `cmd/shikigamictl/cmd_git_mr_detail.go`: `registerGitSub("mr", ...)` dari `init()`; `shikigamictl git` tanpa argumen mencantumkan `mr`; argumen kurang atau `number` bukan angka -> pesan penggunaan; `cmd_git.go` tidak berubah.
3. `MrPane.svelte` tetap terdaftar sebagai kind `git-mr` di `index.ts` dengan props sama; `MrList.openMr` membuka `git-mr` dengan `provider`, `projectId`, `number`, `title`. Bila ada celah, laporkan sebagai perbaikan di file pemiliknya.
4. `MrPane` -> `git-pipeline`: `layout.openPane({kind:'git-pipeline', provider, projectId, pipelineId, title})`; `PipelinePane` membaca `pane.pipelineId`; `MrPane` hanya mengimpor `getMrDetail`/tipe dari `mrDetailClient.ts` (tipe dasar dari `mrClient.ts`).
5. `go build ./...` dan build frontend lulus; aplikasi dijalankan dan dicoba langsung (bagian 6).
6. Verifikasi menyeluruh seluruh spec dengan app berjalan dan `glab` nyata (`gitlab.com`), centang satu per satu (daftar lengkap ada di tiket bagian Wiring butir 3):
   - Rail: item GitLab tampil bila ada project remote GitLab; GitHub tidak tampil tanpa remote GitHub; berganti saat workspace berganti.
   - Panel: bagian per project (gabungan host+path), bisa dilipat, daftar MR (Terbuka/Digabung/Tertutup) dan Pipeline termuat.
   - Klik baris MR membuka pane `git-mr` lengkap; klik pipeline terakhir membuka pane `git-pipeline`; klik baris pipeline membuka pane dengan stage dan job.
   - Aksi Buka/Salin/Kirim ke agen di baris dan header pane; toast "Tidak ada sesi aktif" tanpa sesi.
   - Segarkan di panel dan pane; keadaan memuat, kosong, galat (CLI hilang, belum login, izin, repo tidak ditemukan, timeout) terbaca; tanpa polling.
   - Read-only: tidak ada perintah tulis di `allowlist.go` (`grep` pola `create|merge|approve|close|note|retry|cancel|delete|update`), tidak ada token di config.
   - Layout tersimpan dengan pane `git-mr`/`git-pipeline` terpulihkan dan memuat ulang saat app dibuka ulang.
   - Deskripsi MR yang berisi HTML/markdown mentah tampil sebagai teks biasa.
7. GitHub: dibuktikan lewat fixture (`parseGitHubPrDetail` dengan `testdata/gh-pr-view.json`, skrip `go run` sementara di scratchpad) dan tidak crash tanpa login. Catat butir terbuka di laporan penutup implementasi (bukan di `docs/git/cli-findings.md` kecuali tiket 1 mengizinkan, dan file itu bukan milik epic ini): "verifikasi jalur GitHub (`gh`) di aplikasi menunggu `gh auth login`" (tidak memblokir merge).
8. Selisih yang dilaporkan (bukan diperbaiki di file tiket lain): `Leaf.pipelineId` vs `id` di tiket, `GetMergeRequest` menerima `Repo`, `Mergeable` dinormalisasi ke tiga nilai (temuan 10 menyimpan nilai mentah), `Reviewer.State` GitLab `pending`, `RepoFlag` self-hosted dihindari lewat `mrRepoArg`.

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` (dari akar repo)
- Frontend: `cd frontend && npm run build` (boleh ditambah `npm run check`; skrip `check` ada di `frontend/package.json`)
- Fitur Go saja (`git-mr-detail-data`, `git-mr-detail-ctl`) cukup `go build ./...`. Fitur frontend (`git-mr-pane-ui`) menjalankan build frontend. Wiring menjalankan keduanya lalu menjalankan aplikasi.

## 7. Aturan paralel

- Tiap agen hanya menyentuh `ownedPaths` fiturnya. Jangan menyentuh path fitur lain, file wiring, atau file tiket 1 sampai 3: `types.go`, `allowlist.go`, `runner.go`, `errors.go`, `repo.go`, `mrs.go`, `pipelines.go`, fixture yang sudah ada di `testdata/`, `docs/git/cli-findings.md`, `wire_git.go`, `wire_git_mrs.go`, `wire_git_pipelines.go`, `cmd_git.go`, `cmd_git_mrs.go`, `cmd_git_pipelines.go`, `store.ts`, `client.ts`, `actions.ts`, `mrClient.ts`, `pipelineClient.ts`, `MrList`, `MrRow`, `PipelineList`, `PipelineRow`, `PipelinePane`, `StageList`, `StatusBadge`, `GitPanel.svelte`, `ProjectSection.svelte`, `index.ts`, `layout/model.ts`. Membaca dan memanggil helper di file itu diperbolehkan.
- Bila kebutuhan tidak terpenuhi tanpa mengubah file di luar ownedPaths (perintah belum di allowlist, tipe kurang, dst.), laporkan di hasil fitur; jangan ubah file itu.
- Simbol privat: paket `gitdata` berawalan `mrDetail`/`parseGitLabMr`/`parseGitHubPr`; paket `main` daemon berawalan `gitMrDetail`; paket `main` ctl berawalan `gitMrDetail`.
- Tanpa unit test, tanpa dependensi baru, tanpa commit di luar proses `implement-feature`.
- Ketiga fitur paralel memakai kontrak bagian 2 apa adanya dan tidak menunggu satu sama lain; ctl dan UI tidak bergantung pada backend selesai untuk build.
