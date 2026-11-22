# Overview: git-connector-ticket-03-git-pipelines (Daftar pipeline dan detail pipeline)

Sumber: tiket `docs/tickets/git-connector-ticket-03-git-pipelines.md` dan spec `docs/superpowers/specs/2026-11-22-10-git-connector-design.md`. Dokumen ini adalah sumber kebenaran kontrak bersama; plan per fitur hanya merinci langkah kerja. Module Go: `github.com/NooBeeID/shikigami`. Bila overview ini berselisih dengan tiket, tiket menang kecuali dinyatakan lain di sini (selisih yang disengaja ditandai "DIPUTUSKAN").

## 0. Keadaan kode saat ini (diverifikasi dari repo; tiket 1 dan 2 sudah merge)

Backend (`internal/gitdata/`, sudah ada, TIDAK boleh diubah):
- `types.go` sudah memuat `Pipeline{ID string, Ref, SHA, Status Status, Author string (omitempty), DurationSec int, UpdatedAt string, URL}`, `Job{Name, Status, DurationSec int, URL (omitempty)}`, `Stage{Name, Jobs []Job}`, `PipelineDetail{Pipeline; Stages []Stage}`, konstanta `Status*`, `NormalizeGitLabStatus(s)` dan `NormalizeGitHubStatus(status, conclusion)`. Helper pemetaan SUDAH ADA; jangan mendefinisikan ulang. JSON camelCase.
- DIPUTUSKAN (selisih dengan tiket): `DurationSec` bertipe `int` tanpa `omitempty`, jadi "tidak tersedia" direpresentasikan `0` di wire (tiket menginginkan "tidak ada"; `types.go` tidak boleh diubah). Konsekuensi: UI memperlakukan `durationSec` undefined/null/`0` sebagai tidak tersedia dan menampilkan "-". Tipe TS tetap `durationSec?: number`.
- `runner.go`: `Run(ctx, tool, args...) ([]byte, error)`; allowlist, `CallTimeout` 20 detik, semaphore `MaxConcurrent` 3, galat sudah `*gitdata.Error` (via `classify`). Pakai ini, jangan memanggil `connector.RunCLITimeout` langsung, tanpa `os/exec`.
- `errors.go`: `*Error{Code, Message}`, `AsError`, kode `cli_missing|not_logged_in|permission|not_found|timeout|not_allowed|cli_error`, helper privat `providerLabel(tool)`. `ErrorCode` adalah string sehingga kode lokal baru (mis. `invalid_params`) legal tanpa mengubah `errors.go`.
- `repo.go` `RepoFlag` TIDAK andal untuk glab self-hosted (lihat `docs/git/cli-findings.md` bagian 9) dan tidak boleh disentuh. DIPUTUSKAN: `pipelines.go` memakai helper privat sendiri `pipelineRepoArg(r Repo) string` dengan aturan sama seperti `mrRepoArg` di `mrs.go`: GitLab `gitlab.com` -> `r.Path`; GitLab host lain -> `"https://" + r.Host + "/" + r.Path`; GitHub `github.com` -> `r.Path`; GitHub host lain -> `r.Host + "/" + r.Path`. Jangan memanggil `RepoFlag` maupun `mrRepoArg` (privat milik tiket 2).
- `allowlist.go` sudah mengizinkan: `glab ci list` (flag `-R --repo -F --output -P --per-page -p --page --status -s --ref -r --orderBy -o --sort`), `glab ci get` (flag `-R --repo -F --output -b --branch -p --pipeline-id -d --with-job-details`), `gh run list` (flag `-R --repo --json -L --limit -b --branch -s --status -w --workflow -e --event -u --user -c --commit`), `gh run view` (flag `-R --repo --json -j --job --attempt`). Gunakan HANYA bentuk panjang (`--repo`, `--output`, `--per-page`, `--pipeline-id`, `--json`, `--limit`). Flag `--log`, `--log-failed`, `--with-variables`, `--web` ditolak; jangan dipakai. `glab api`/`gh api` TIDAK diizinkan dan tidak diperlukan. Bila sesuatu kurang, laporkan; jangan ubah file itu.
- `testdata/` sudah punya fixture: `glab-pipeline-list.json` (NYATA), `glab-pipeline-get.json`, `glab-pipeline-get-nojobs.json` (NYATA), `gh-run-list.json`, `gh-run-view.json` (ASUMSI). Bentuk acuan parser. Dilarang mengubahnya.

Temuan spike (`docs/git/cli-findings.md` bagian 3.3-3.5, 4, 10) yang mengikat desain:
- GitLab `glab ci list -F json` memuat per elemen: `id` (angka), `iid`, `status`, `source`, `ref`, `sha`, `web_url`, `updated_at`, `created_at`. TIDAK ada author dan durasi. Jadi di daftar GitLab `Author` kosong dan `DurationSec` 0; UI daftar GitLab tidak menampilkan kolom itu (sembunyikan, bukan "-"). Pipeline MR berformat `ref` = `refs/merge-requests/<iid>/head`; tampilkan apa adanya.
- GitLab `glab ci get --repo <arg> --pipeline-id <id> --output json` memuat objek pipeline lengkap: `user.username`, `duration` (int detik; running = 0), `started_at`, `finished_at`, `web_url`, `ref`, `sha`, `status`, `updated_at`, serta `jobs[]` (atau `null` pada pipeline tanpa job). Job: `id`, `name`, `stage`, `status`, `duration` (float detik), `web_url`. Urutan `jobs` dari server adalah id MENURUN dan tidak ada daftar stage terurut. DIPUTUSKAN pengelompokan: urutkan job ascending menurut `id`; stage berurutan menurut kemunculan pertama pada urutan itu; job dalam stage tetap urut id naik; durasi job `float` dibulatkan ke int (`math.Round`); `jobs: null` -> `Stages: []` (slice non-nil). Pipeline tak ada: glab menulis galat ke stdout/exit non-nol; ditangani `classify` (bila hasil bukan JSON objek valid -> `not_found`, lihat 2.2).
- GitHub (ASUMSI sampai `gh auth login`): `gh run list --json` field `databaseId,headBranch,headSha,status,conclusion,createdAt,updatedAt,startedAt,url,event,displayTitle,workflowName`; TIDAK ada actor/penulis pemicu, jadi `Author` kosong untuk GitHub. `gh run view <id> --json ...,jobs`: job memuat `databaseId, name, status, conclusion, startedAt, completedAt, url`. GitHub tidak punya stage; DIPUTUSKAN satu `Stage` bernama `workflowName` (fallback `"Jobs"` bila kosong) berisi semua job sesuai urutan array.

Frontend (`frontend/src/modules/git/`, sudah ada):
- `client.ts`: `Provider`, `Repo` (`projectId`, `projectName`, `projects`, `provider`, `host`, `path`, `webUrl`; TANPA `workspaceId`), `GitError {code, message}`, `parseGitError(x)`.
- `store.ts`: `repoKey(r)`, `refreshRepos()`, dll. Workspace aktif: `activeWorkspaceId` dari `../../lib/stores/workspaces`.
- `actions.ts`: `openInBrowser(url)`, `copyLink(url)`, `sendToAgent(url)` (toast "Tidak ada sesi aktif" sudah di dalamnya).
- `StatusBadge.svelte` (tiket 2): props `{status: string; kind?: 'pipeline'|'review'|'mr'}`; default (tanpa `kind`) untuk status pipeline yang dikenal memakai tabel pipeline. Pakai `<StatusBadge status={x.status} kind="pipeline" />`.
- `mrClient.ts` dan `MrList.svelte` adalah pola acuan: klien memanggil `call<...>('...', params)` dari `../../lib/ipc`, melempar `parseGitError(e)`; list memakai `seq` untuk membuang respons usang, `$effect` bergantung pada `repo` (identitas objek berubah saat Segarkan panel) dengan `untrack`, `activeWorkspaceId` lewat `get()`, galat dengan tombol "Coba lagi" dan "Buka Pengaturan > Connector" via `openSettings('connectors')` dari `../settings` untuk kode `cli_missing`/`not_logged_in`. Tiket 3 mengikuti pola persis ini.
- `ProjectSection.svelte` merender `PipelineList` (props `{provider, repo}`) hanya saat bagian terbuka, jadi bagian terlipat tidak memuat. `PipelineList.svelte` dan `PipelinePane.svelte` saat ini placeholder.
- Pane: `PipelinePane` menerima props `{ pane: Leaf; visible: boolean }` (tipe `Leaf` dari `../layout/model`). DIPUTUSKAN (selisih dengan tiket): field leaf id pipeline di model layout bernama `pipelineId` (bukan `id`; `Leaf.id` sudah dipakai untuk id leaf itu sendiri). Pane membaca `pane.provider`, `pane.projectId`, `pane.pipelineId`. `layout/model.ts` sudah memetakan `provider`, `projectId`, `pipelineId` di `NewLeaf`, pembuat leaf, persist, dan `hydrateNode`; pane kind `git-pipeline` sudah terdaftar di `modules/git/index.ts` (`titleFor` memakai `leaf.title` atau `'Pipeline ' + leaf.pipelineId`). Tidak ada perubahan di file itu.
- Pembukaan pane: `layout.openPane({ kind: 'git-pipeline', provider, projectId: repo.projectId, pipelineId: p.id, title })` dari `../../lib/stores/layout`. `workspaceId` leaf TIDAK diisi; pane mengambil workspace aktif dari `activeWorkspaceId` (`get()`), sama seperti list. Dedup `openPane` hanya berdasarkan `filePath`+`projectId` sehingga membuka id yang sama dua kali bisa membuat pane kedua; ini diterima ("memfokuskan bila API mendukung" tidak berlaku).

## 1. Ringkasan epic dan urutan eksekusi

Epic mengisi bagian Pipeline (GitLab) / Actions (GitHub) per project di panel Git dengan 20 pipeline/run terbaru, dan pane tengah `git-pipeline` berisi header + stage + job. Aksi per baris dan header: Buka di browser, Salin link, Kirim ke agen (job memakai URL job). Read-only lewat `glab`/`gh`.

Urutan eksekusi:
1. Foundation: TIDAK ADA. Tipe, runner, allowlist, fixture, `StatusBadge`, `actions.ts`, pane kind, field leaf, dan titik ekstensi `ProjectSection` -> `PipelineList` sudah ada. Tidak ada `00-foundation.md` (`hasFoundation=false`).
2. Fitur paralel (empat agen, tanpa saling menunggu): `git-pipelines-data`, `git-pipelines-ctl`, `git-pipelines-list-ui`, `git-pipeline-pane-ui`.
3. Wiring (`git-pipelines-wiring`): verifikasi dan penyambungan setelah empat fitur selesai.

## 2. Kontrak bersama

### 2.1 IPC `git.pipelines.list`

Params: `{"workspaceId": string, "projectId": string}`. Kosong -> galat `invalid_params` (`ipc.ErrInvalid`).

Hasil: ARRAY JSON `Pipeline[]` langsung (bukan objek pembungkus; berbeda dengan `git.mrs.list` sesuai bunyi tiket), maksimal 20, terbaru dulu, SELALU array (kosong = `[]`, tidak pernah `null`). Klien tetap toleran terhadap `null`.
```ts
export type PipelineStatus = 'pending'|'running'|'success'|'failed'|'canceled'|'skipped'|'unknown';
export interface Pipeline {
  id: string;            // GitLab: id pipeline; GitHub: databaseId run
  ref: string;           // branch/tag
  sha: string;           // sha penuh (UI memotong 7 karakter)
  status: PipelineStatus;
  author?: string;       // hanya bila ada (GitLab daftar: tidak ada; GitHub: tidak ada)
  durationSec?: number;  // 0/undefined = tidak tersedia -> UI "-"
  updatedAt: string;     // RFC3339
  url: string;
}
```

### 2.2 IPC `git.pipeline.get`

Params: `{"workspaceId": string, "projectId": string, "id": string}`. Kosong -> `invalid_params`. `id` wajib berupa digit (`^[0-9]+$`), selain itu `invalid_params` (mencegah injeksi argumen CLI).

Hasil: objek `PipelineDetail`:
```ts
export interface PipelineJob { name: string; status: PipelineStatus; durationSec?: number; url?: string }
export interface PipelineStage { name: string; jobs: PipelineJob[] }
export interface PipelineDetail extends Pipeline { stages: PipelineStage[] }  // stages selalu array
```
Galat IPC: `"[kode] pesan Indonesia"` lewat `mapGitError` (not_found -> `ipc.ErrNotFound`, not_allowed -> `ipc.ErrInvalid`, lainnya `ipc.ErrInternal`). Tanpa output mentah CLI.

### 2.3 Go (`internal/gitdata/pipelines.go`, package `gitdata`)

```go
const PipelineLimit = 20
func ListPipelines(ctx context.Context, repo Repo) ([]Pipeline, error)
func GetPipeline(ctx context.Context, repo Repo, id string) (PipelineDetail, error)
// parser murni (tanpa pemanggilan CLI), untuk fixture:
func parseGitLabPipelines(data []byte) ([]Pipeline, error)
func parseGitHubRuns(data []byte) ([]Pipeline, error)
func parseGitLabPipelineDetail(data []byte) (PipelineDetail, error)
func parseGitHubRunDetail(data []byte) (PipelineDetail, error)
```
- `ListPipelines` mengembalikan slice non-nil; galat bertipe `*gitdata.Error`. Hasil diurutkan `UpdatedAt` menurun (gagal-parse waktu = paling lama; `sort.SliceStable`) lalu dipotong `PipelineLimit`.
- Perintah GitLab list: `glab ci list --repo <arg> --output json --per-page 20`. Output kosong/`null` -> `[]`.
- Perintah GitLab detail: `glab ci get --repo <arg> --pipeline-id <id> --output json`. Output bukan JSON objek (mis. teks "No pipeline with the given ID") -> `&Error{Code: CodeNotFound, Message: "Pipeline tidak ditemukan"}`.
- Perintah GitHub list: `gh run list --repo <arg> --limit 20 --json databaseId,headBranch,headSha,status,conclusion,createdAt,updatedAt,startedAt,url,event,displayTitle,workflowName`.
- Perintah GitHub detail: `gh run view <id> --repo <arg> --json databaseId,headBranch,headSha,status,conclusion,createdAt,updatedAt,startedAt,url,event,displayTitle,workflowName,jobs`.
- Pemetaan GitLab list: `id` -> `ID` (angka ke string), `ref`, `sha`, `status` -> `NormalizeGitLabStatus`, `updated_at` -> `UpdatedAt` (apa adanya), `web_url` -> `URL`; `Author` kosong, `DurationSec` 0.
- Pemetaan GitLab detail: semua field di atas ditambah `user.username` -> `Author`, `duration` -> `DurationSec`, job: `name`, `status` -> `NormalizeGitLabStatus`, `duration` dibulatkan, `web_url` -> `URL`, pengelompokan stage sesuai bagian 0.
- Pemetaan GitHub: `databaseId` -> `ID`, `headBranch` -> `Ref`, `headSha` -> `SHA`, `NormalizeGitHubStatus(status, conclusion)`, `updatedAt` -> `UpdatedAt` (bila kosong pakai `createdAt`), `url`; `DurationSec` = `updatedAt - startedAt` dalam detik hanya bila status ternormalisasi bukan `pending`/`running` dan kedua waktu valid dan hasil >= 0, selain itu 0. Job: `NormalizeGitHubStatus(status, conclusion)`, `DurationSec` = `completedAt - startedAt` (valid dan >= 0, selain itu 0), `URL` = `url` job. Satu stage (lihat bagian 0).
- Parser toleran: field hilang/null tidak panic; JSON rusak -> `&Error{Code: CodeCLI, Message: "<GitLab|GitHub> mengembalikan data yang tidak dapat dibaca"}` (pakai `providerLabel`).
- Hanya `Run(ctx, "glab"|"gh", ...)`; tanpa import `internal/workspace`, tanpa `os/exec`.
- Awali helper privat baru di paket `gitdata` dengan `pipeline`/`parseGit` (jangan bentrok dengan `mr*`/`parseMr*` tiket 2).

### 2.4 `cmd/shikigamid/wire_git_pipelines.go`

Registrasi mandiri: `func init() { RegisterModule("git-pipelines", func(srv *ipc.Server, d *Deps) { registerGitPipelines(srv, d) }) }` (nama modul `git-pipelines` harus unik; pola `wire_git_mrs.go`). Handler memakai helper paket `main` yang sudah ada: `gitDecode`, `gitErr`, `gitResolveRepo(d, workspaceID, projectID)`, `mapGitError`. `ListPipelines` dan `GetPipeline` dipanggil dengan `ctx` handler. Pastikan hasil list non-nil. `projectId` tak dikenal -> `gitResolveRepo` mengembalikan galat `not_found` terbaca. Helper privat diawali `gitPipelines`.

### 2.5 `cmd/shikigamictl/cmd_git_pipelines.go`

Registrasi mandiri: `func init() { registerGitSub("pipelines", gitPipelines); registerGitSub("pipeline", gitPipeline) }` (`gitSubFunc` = `func(ctx context.Context, c *ipc.Client, args []string) error`). DIPUTUSKAN: bentuk flag sesuai tiket (`--workspace`, `--project`, `--id`) lewat `flag.NewFlagSet` dengan `SetOutput(io.Discard)`; flag wajib hilang -> kembalikan `errors.New(<usage>)`:
- `penggunaan: shikigamictl git pipelines --workspace <id> --project <id>`
- `penggunaan: shikigamictl git pipeline --workspace <id> --project <id> --id <id>`
Memanggil IPC `git.pipelines.list` / `git.pipeline.get` (cara memanggil: ikuti `cmd_git_mrs.go`, `c.Call` ke hasil struct lokal berawalan `gitPipelines`). Hasil list: satu baris per pipeline dipisah tab (`id`, `ref`, `status`, sha 7 karakter, durasi `Ns` atau `-`, `url`), diakhiri `<n> baris`. Detail: baris header (`id ref status sha durasi url`), lalu per stage baris `[stage] <nama>` dan per job `  <nama>\t<status>\t<durasi>\t<url>`. Bersihkan tab/newline dari string (pola `gitMrsOneLine`, tapi simbol baru berawalan `gitPipelines`). Galat daemon dikembalikan sebagai `error`.

### 2.6 Frontend

`pipelineClient.ts` (milik `git-pipelines-list-ui`; SATU-SATUNYA pembungkus IPC pipeline). Mengekspor tipe `PipelineStatus`, `Pipeline`, `PipelineJob`, `PipelineStage`, `PipelineDetail` (bentuk di 2.1 dan 2.2) dan:
```ts
export async function listPipelines(params: { workspaceId: string; projectId: string }): Promise<Pipeline[]>;
export async function getPipeline(params: { workspaceId: string; projectId: string; id: string }): Promise<PipelineDetail>;
```
Implementasi: `call<Pipeline[] | null>('git.pipelines.list', params)` dan `call<PipelineDetail>('git.pipeline.get', params)` dari `../../lib/ipc`; try/catch lalu `throw parseGitError(e)` (impor dari `./client`); `listPipelines` mengembalikan `Array.isArray(r) ? r : []`; `getPipeline` menjamin `stages` array (`{...r, stages: r.stages ?? []}`). Signature ini dipegang persis; `git-pipeline-pane-ui` mengimpornya meski file belum ada saat dikerjakan (jangan membuatnya; build penuh baru lulus setelah `git-pipelines-list-ui` selesai, itu normal).

Komponen (Svelte 5 runes, `$props()`):
- `PipelineList.svelte`: props tetap `{ provider: Provider; repo: Repo }`. Pola `MrList`. Empty: `Tidak ada pipeline` (GitHub juga `Tidak ada pipeline`). Dirender `PipelineRow` per item dengan key `p.id`. Bila `listPipelines` melempar, tampilkan galat terisolasi di bagian ini.
- `PipelineRow.svelte`: `{ pipeline: Pipeline; provider: Provider; onOpen: (p: Pipeline) => void }`. Kolom: ref (ikon branch/tag glyph sederhana; ref berawalan `refs/tags/` atau tampak tag boleh diberi glyph tag, selain itu branch), `StatusBadge`, `sha.slice(0,7)`, `author` hanya bila ada, durasi hanya bila `durationSec` > 0 (GitLab daftar: sembunyikan kolom durasi karena selalu 0; GitHub: "-" bila tidak ada), waktu relatif Indonesia ("baru saja", "3 jam lalu", "2 hari lalu"; fungsi lokal di file ini), tiga tombol ikon dengan `aria-label` yang memakai `actions.ts` pada `pipeline.url`, `event.stopPropagation()`. Baris `role="button"`/`tabindex="0"`, Enter/Spasi memanggil `onOpen`.
- `PipelinePane.svelte`: props `{ pane: Leaf; visible: boolean }` (tidak berubah dari placeholder). Memuat `getPipeline` lewat `pipelineClient.ts` saat pertama tampil/dipulihkan (mount) dan saat Segarkan; mengabaikan respons usang (`seq`) saat `pane.pipelineId` berubah; tidak memuat saat `pane.provider`/`projectId`/`pipelineId` kosong (keadaan aman "Project atau pipeline tidak ditemukan", tombol aksi nonaktif); `activeWorkspaceId` null -> keadaan aman yang sama. Wrapper luar tetap `<div class="git-pane" hidden={!visible}>`. Header: judul (`pane.title` atau "Pipeline <id>"), `StatusBadge` keseluruhan, ref, sha 7 karakter, durasi, penulis bila ada, tombol Buka/Salin/Kirim ke agen (URL pipeline), Segarkan. Di bawahnya `StageList`.
- `StageList.svelte`: props `{ stages: PipelineStage[] }`. Tiap stage satu bagian dengan nama dan jumlah job; tiap job satu baris: nama (link ke `url` via `openInBrowser`, teks biasa bila tanpa url), `StatusBadge`, durasi ("-" bila 0/tidak ada), tombol Buka/Salin/Kirim ke agen untuk URL job (nonaktif dengan `title` "Job ini tidak punya link" bila `url` kosong). Stage tanpa job: "Tidak ada job". `stages` kosong: "Tidak ada stage". Tipe diimpor dengan `import type` dari `./pipelineClient`.

Format durasi (konsisten di list/pane/stage; tiap file mendefinisikan fungsi lokal sendiri karena file bersama tidak boleh dibuat): `<60` -> `"Ns"`; `<3600` -> `"Nm Ns"` (mis. `1m 23s`); selain itu `"Nj Nm"`; tidak tersedia -> `"-"`.

### 2.7 Konvensi

Teks UI bahasa Indonesia; identifier kode bahasa Inggris; tanpa dependensi baru; tanpa unit test; tanpa polling; tanpa token di config; maks 3 panggilan CLI bersamaan (runner yang menjaga; UI hanya memuat saat bagian tampil). Stile: variabel CSS yang dipakai tiket 1/2 (`--fg`, `--fg-muted`, `--bg-elev`, `--bg-hover`, `--border`, `--danger`) dengan fallback; warna status hanya lewat `StatusBadge`. Jangan menambah file global atau komponen bersama baru.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `git-pipelines-data` | `gitdata.ListPipelines`/`GetPipeline` (GitLab + GitHub, parser murni, normalisasi), IPC `git.pipelines.list` dan `git.pipeline.get` | `internal/gitdata/pipelines.go`, `cmd/shikigamid/wire_git_pipelines.go` |
| `git-pipelines-ctl` | `shikigamictl git pipelines` dan `git pipeline` (alat debug) | `cmd/shikigamictl/cmd_git_pipelines.go` |
| `git-pipelines-list-ui` | `pipelineClient.ts`, isi `PipelineList`, `PipelineRow` di panel | `frontend/src/modules/git/PipelineList.svelte`, `frontend/src/modules/git/PipelineRow.svelte`, `frontend/src/modules/git/pipelineClient.ts` |
| `git-pipeline-pane-ui` | isi `PipelinePane` dan `StageList` (header, stage, job, aksi) | `frontend/src/modules/git/PipelinePane.svelte`, `frontend/src/modules/git/StageList.svelte` |
| `git-pipelines-wiring` | verifikasi dan penyambungan (tanpa file sendiri) | (tidak ada) |

Rincian (acceptance criteria lengkap ada di tiket; plan per fitur wajib memetakannya):
- `git-pipelines-data`: sesuai 2.1-2.4. Galat CLI hilang/belum login/izin/repo tak ada/timeout/project tak ada -> galat IPC berpesan Indonesia, tanpa panic, tanpa output mentah. Hanya perintah baca lewat `Run`. Tidak ada token/kredensial disimpan atau dicatat.
- `git-pipelines-ctl`: sesuai 2.5; tidak mengubah `cmd_git.go`; bisa dikompilasi tanpa backend (tanpa mengimpor `gitdata`).
- `git-pipelines-list-ui`: sesuai 2.6; mengganti isi placeholder `PipelineList` tanpa mengubah props; klik baris membuka pane `git-pipeline` (2.6) dengan `title` mis. `"Pipeline " + id + " (" + ref + ")"`; tombol aksi memakai URL pipeline.
- `git-pipeline-pane-ui`: sesuai 2.6; aksi job memakai URL job, bukan URL pipeline.

## 4. Foundation

Tidak ada. Alasan: semua tipe Go, runner, allowlist, fixture, `StatusBadge`, `actions.ts`, pane kind `git-pipeline`, field leaf `provider/projectId/pipelineId`, dan titik ekstensi `ProjectSection` -> `PipelineList` sudah ada dari tiket 1 dan 2 (diverifikasi). Output: `hasFoundation=false`, tanpa `00-foundation.md`.

## 5. Wiring checklist (untuk `git-pipelines-wiring`)

Tanpa file milik sendiri; hanya verifikasi dan koreksi kecil di dalam path epic ini (laporkan bila kekurangan ada di file tiket lain; jangan ubah file itu):
1. `cmd/shikigamid/wire_git_pipelines.go`: `RegisterModule("git-pipelines", ...)` ada, nama modul tidak bentrok dengan modul lain (`grep -rn 'RegisterModule("git' cmd/shikigamid`); `git.pipelines.list` dan `git.pipeline.get` terdaftar di `srv.Register`; `wire_git.go` tidak diubah.
2. `cmd/shikigamictl/cmd_git_pipelines.go`: `registerGitSub("pipelines", ...)` dan `registerGitSub("pipeline", ...)` dari `init()`; `shikigamictl git` tanpa argumen mencantumkan keduanya; flag wajib hilang -> pesan penggunaan.
3. `ProjectSection.svelte` sudah merender `<PipelineList {provider} {repo} />` (tiket 1); pastikan props cocok dan bagian Pipeline tampil di panel GitLab/GitHub.
4. `PipelineRow` -> `layout.openPane({kind:'git-pipeline', provider, projectId, pipelineId, title})`; pastikan `PipelinePane` membaca `pane.pipelineId` (bukan `pane.id`) dan memuat `StageList`; `PipelinePane` dan `StageList` hanya mengimpor `getPipeline`/tipe dari `pipelineClient.ts`.
5. Persist: leaf `provider/projectId/pipelineId` dipersist dan dipulihkan setelah restart (sudah dipetakan di `layout/model.ts`); pane yang dipulihkan memuat ulang data; repo hilang/CLI gagal -> keadaan galat aman (bukan crash).
6. Verifikasi aplikasi nyata (GitLab, `glab` login `gitlab.com`): 20 pipeline terbaru, Segarkan panel, klik baris -> stage dan job, aksi Buka/Salin/Kirim ke agen untuk baris pipeline, header pane, dan baris job (URL job; tanpa sesi aktif -> toast "Tidak ada sesi aktif"), bagian terlipat tidak memanggil IPC, galat CLI tampil aman. `shikigamictl git pipelines --workspace ... --project ...` dan `git pipeline ... --id ...` mencetak hasil.
7. GitHub: dibuktikan lewat fixture (`parseGitHubRuns`, `parseGitHubRunDetail` dengan `testdata/gh-run-list.json` dan `gh-run-view.json`, mis. lewat skrip `go run` sementara di scratchpad, tidak di repo) dan tidak crash tanpa login. Catat butir terbuka di laporan akhir: "verifikasi pipeline/Actions GitHub di app menunggu `gh auth login`" (tidak memblokir).
8. Selisih yang dilaporkan, bukan diperbaiki di file tiket lain: `DurationSec` int tanpa omitempty (0 = tidak tersedia), `Leaf.pipelineId` vs `id` di tiket, `RepoFlag` self-hosted (dihindari lewat `pipelineRepoArg`).

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` (dari akar repo)
- Frontend: `cd frontend && npm run build` (tambahkan `npm run check` bila perlu; skrip `check` ada di `frontend/package.json`)
- Fitur yang hanya menyentuh Go (`git-pipelines-data`, `git-pipelines-ctl`) cukup `go build ./...`. Fitur frontend menjalankan build frontend; `git-pipeline-pane-ui` bisa gagal impor `pipelineClient.ts` bila `git-pipelines-list-ui` belum selesai (dikerjakan paralel); dalam kasus itu verifikasi penuh dilakukan di wiring. Wiring menjalankan keduanya dan menjalankan aplikasi.

## 7. Aturan paralel

- Tiap agen hanya menyentuh `ownedPaths` fiturnya. Jangan menyentuh path fitur lain, file wiring, atau file tiket 1/2/4 (`types.go`, `allowlist.go`, `runner.go`, `errors.go`, `repo.go`, `mrs.go`, `testdata/`, `docs/git/cli-findings.md`, `wire_git.go`, `wire_git_mrs.go`, `cmd_git.go`, `cmd_git_mrs.go`, `store.ts`, `client.ts`, `actions.ts`, `mrClient.ts`, `MrList`, `MrRow`, `StatusBadge`, `GitPanel.svelte`, `ProjectSection.svelte`, `index.ts`, `layout/model.ts`, `MrPane.svelte`, dan seterusnya). Membaca dan memanggil helper di file itu diperbolehkan.
- Bila kebutuhan tidak terpenuhi tanpa mengubah file di luar ownedPaths (perintah belum di allowlist, tipe kurang, field store kurang), laporkan di hasil fitur; jangan ubah file itu.
- Simbol privat: paket `gitdata` berawalan `pipeline`/`parseGit` (selain empat parser bernama di 2.3); paket `main` daemon berawalan `gitPipelines`; paket `main` ctl berawalan `gitPipelines`/`gitPipeline`.
- Tanpa unit test, tanpa dependensi baru, tanpa commit di luar proses `implement-feature`.
- Keempat fitur paralel memakai kontrak bagian 2 apa adanya dan tidak menunggu satu sama lain; ctl dan UI tidak bergantung pada backend selesai untuk build.
