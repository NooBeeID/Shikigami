---
id: git-connector-ticket-03-git-pipelines
title: Daftar pipeline dan detail pipeline
spec: docs/superpowers/specs/2026-11-22-10-git-connector-design.md
status: ready
wave: 3
depends_on: [git-connector-ticket-02-git-mrs]
---
# Daftar pipeline dan detail pipeline

## Tujuan
Tiket 1 (`git-repos`) menyediakan connector GitLab/GitHub, deteksi repo, rail Git, panel per provider, dan komponen placeholder `PipelineList`/`PipelinePane`. Tiket 2 (`git-mrs`) menyediakan daftar MR/PR dan `StatusBadge`. Epic ini mengisi sisi pipeline: daftar 20 pipeline (GitLab) atau run (GitHub Actions) terbaru per project di panel, dan pane `git-pipeline` berisi stage dan job. Tujuannya agar user melihat status CI tanpa keluar app, lalu membuka di browser, menyalin link, atau mengirim link (pipeline atau job) ke agen sebagai konteks. Semua read-only lewat `glab`/`gh` yang sudah login.

## Hasil yang bisa dijalankan
Di atas tiket 1 dan 2, setelah tiket ini selesai:
- Di panel Git (GitLab atau GitHub), bagian Pipeline tiap project menampilkan 20 pipeline/run terbaru tanpa filter (bukan placeholder lagi). Baris: ref (branch/tag), `StatusBadge`, sha pendek (7 karakter), penulis pemicu bila ada, durasi, waktu relatif, serta tombol Buka di browser, Salin link, Kirim ke agen.
- Keadaan memuat, kosong ("Tidak ada pipeline"), dan galat terbaca (CLI belum terpasang, belum login, izin kurang, repo tidak ditemukan, timeout) tampil di bagian itu; galat satu project tidak memengaruhi project lain. Tombol Segarkan panel memuat ulang daftar pipeline.
- Klik baris pipeline membuka pane tengah `git-pipeline`: header (status keseluruhan, ref, sha, durasi, Buka/Salin/Kirim ke agen, Segarkan) dan daftar stage berisi job (nama, status, durasi, link job). Tiap baris job punya Buka/Salin/Kirim ke agen memakai URL job.
- GitLab: stage dan job dari glab. GitHub: job dari `gh run view --json jobs`, dikelompokkan sesuai temuan spike (default satu stage bernama workflow/"Jobs").
- Pane yang dipersist dipulihkan dengan mengambil ulang data; repo hilang atau CLI gagal menampilkan keadaan galat yang aman, bukan crash.
- `shikigamictl git pipelines` dan `git pipeline` berjalan sebagai alat debug.
- `go build ./...` dan build frontend lulus; aplikasi berjalan. Jalur GitHub dibuktikan lewat fixture; verifikasi nyata menunggu `gh auth login` (butir terbuka, tidak memblokir).

## Batasan
In scope:
- `internal/gitdata/pipelines.go`: `ListPipelines`, `GetPipeline`, parser dari fixture, normalisasi status.
- IPC `git.pipelines.list` dan `git.pipeline.get` lewat `wire_git_pipelines.go` (registrasi mandiri).
- Subcommand `shikigamictl git pipelines` dan `git pipeline`.
- UI: `PipelineList.svelte`, `PipelineRow.svelte`, `pipelineClient.ts`, `PipelinePane.svelte`, `StageList.svelte`.
- Memakai `StatusBadge`, `openInBrowser`/`copyLink`/`sendToAgent` dan kerangka panel/pane dari tiket sebelumnya tanpa mengubahnya.

Out of scope:
- Retry, cancel, trigger pipeline; log job; artefak; CI variable; filter pipeline; paginasi lebih dari 20; polling.
- Mengubah `types.go`, `allowlist.go`, `runner.go`, `errors.go`, `repo.go`, `testdata/`, `docs/git/cli-findings.md`, `wire_git.go`, `cmd_git.go`, `store.ts`, `client.ts`, `actions.ts`, `GitPanel.svelte`, `ProjectSection.svelte`, `modules/layout/model.ts` (tiket 1); `MrList`, `MrRow`, `StatusBadge`, `mrClient` (tiket 2); `MrPane` dan detail MR (tiket 4). Bila `glab`/`gh` subcommand pipeline belum ada di allowlist, atau tipe `Pipeline`/`PipelineDetail` kurang field, laporkan; jangan mengubah file itu.
- Unit test dan dependensi baru.

## Kontrak
Dipakai dari tiket sebelumnya (jangan didefinisikan ulang):
- Tipe Go di `internal/gitdata/types.go`: `Repo`, `Pipeline{ID, Ref, SHA, Status, Author, DurationSec, UpdatedAt, URL}`, `PipelineDetail` = `Pipeline` + `Stages []{Name, Jobs []{Name, Status, DurationSec, URL}}`. JSON camelCase; bentuk field opsional persis mengikuti `docs/git/cli-findings.md`.
- Runner dan allowlist `gitdata` (bungkus `connector.RunCLI`, timeout 20 detik, maksimal 3 pemanggilan bersamaan), pembentuk `-R` (`OWNER/REPO` atau `GROUP/SUBGROUP/REPO` untuk host bawaan; `HOST/PATH` untuk self-hosted), resolusi `projectId` -> `Repo` (deteksi repo di memori), bentuk galat connector/Lark (kode + pesan Indonesia).
- Status ternormalisasi: `pending | running | success | failed | canceled | skipped | unknown`. GitHub `queued`/`in_progress`/`completed`+conclusion dipetakan ke himpunan sama (queued/waiting -> pending; in_progress -> running; success -> success; failure/timed_out/startup_failure/action_required -> failed; cancelled -> canceled; skipped/neutral -> skipped; lainnya -> unknown). Gunakan helper pemetaan di `types.go` bila tiket 1 sudah menyediakannya; bila belum, definisikan fungsi tak-diekspor di `pipelines.go`.
- Frontend: `StatusBadge.svelte` (tiket 2); `actions.ts` (`openInBrowser(url)`, `copyLink(url)`, `sendToAgent(url)`); komponen placeholder `PipelineList.svelte` (props `{provider, repo}`) dan `PipelinePane.svelte` milik tiket 1 yang kini diambil alih isinya tanpa mengubah props; pane kind `git-pipeline` terdaftar tiket 1 dengan field leaf dipersist `provider`, `projectId`, `id`; tombol Segarkan panel dan mekanisme pemuatan per bagian dari `ProjectSection`/store tiket 1 (ikuti cara `MrList` tiket 2 mengikutinya).

Disediakan:

IPC `git.pipelines.list` params `{workspaceId: string, projectId: string}` hasil `Pipeline[]` (maksimal 20, urut terbaru dulu):
```ts
type PipelineStatus = 'pending'|'running'|'success'|'failed'|'canceled'|'skipped'|'unknown';
interface Pipeline { id: string; ref: string; sha: string; status: PipelineStatus; author?: string; durationSec?: number; updatedAt: string; url: string }
```
IPC `git.pipeline.get` params `{workspaceId, projectId, id: string}` hasil:
```ts
interface PipelineDetail extends Pipeline { stages: { name: string; jobs: { name: string; status: PipelineStatus; durationSec?: number; url?: string }[] }[] }
```
Go: `const PipelineLimit = 20`; `func ListPipelines(ctx, repo Repo) ([]Pipeline, error)`; `func GetPipeline(ctx, repo Repo, id string) (PipelineDetail, error)`. Parser murni terpisah dari pemanggilan CLI (`parseGitLabPipelines`, `parseGitHubRuns`, `parseGitLabPipelineDetail`, `parseGitHubRunDetail`) agar bisa diuji dengan fixture di `internal/gitdata/testdata/`.

TS (`pipelineClient.ts`): `listPipelines(params: {workspaceId; projectId}): Promise<Pipeline[]>` dan `getPipeline(params: {workspaceId; projectId; id}): Promise<PipelineDetail>`; tipe `Pipeline`, `PipelineDetail`, `PipelineStage`, `PipelineJob` diekspor dari file ini. Ini satu-satunya pembungkus IPC pipeline; `PipelinePane` memakainya dan tidak membuat salinan.

Pembentukan data per provider (final mengikuti `docs/git/cli-findings.md`):
- GitLab list: `glab ci list -F json -R <repo>` atau `glab api` bila subcommand tak memuat field yang dibutuhkan (cadangan di spec), dibatasi 20 (`--per-page`/`-P` sesuai temuan). Detail: stage dan job dari perintah hasil spike (mis. `glab ci get -F json` atau `glab api projects/:id/pipelines/:id/jobs`), job dikelompokkan per field `stage` dengan urutan kemunculan stage dipertahankan.
- GitHub list: `gh run list --json databaseId,headBranch,headSha,status,conclusion,createdAt,updatedAt,startedAt,url,event,displayTitle,workflowName -L 20 -R <repo>`; `Ref` = `headBranch`, `Author` dari field pemicu bila ada (kosongkan bila tak ada), `DurationSec` dari `updatedAt - startedAt` bila status selesai. Detail: `gh run view <id> --json jobs,...`; GitHub tanpa stage: satu stage bernama workflow (atau "Jobs") berisi semua job, kecuali temuan spike menunjukkan pengelompokan lain. URL job dari field `url` job.
- `DurationSec` bernilai tidak ada (bukan 0) bila belum tersedia; UI menampilkan "-".

## Fitur

### git-pipelines-data: Data pipeline gitdata dan IPC
Deskripsi:
- `internal/gitdata/pipelines.go`: `ListPipelines` dan `GetPipeline` sesuai Kontrak. Memakai runner dan allowlist tiket 1 (hanya perintah baca). Normalisasi ke `Pipeline`/`PipelineDetail` (status ternormalisasi, `DurationSec`, `URL`, URL job). Hasil `ListPipelines` dipotong ke `PipelineLimit` dan diurutkan terbaru dulu (`UpdatedAt` turun) bila CLI tidak menjamin urutan. Id yang tidak ada atau repo tidak ditemukan dipetakan ke galat connector yang sama (pesan Indonesia, tanpa output mentah CLI).
- `cmd/shikigamid/wire_git_pipelines.go`: mendaftarkan `git.pipelines.list` `{workspaceId, projectId}` dan `git.pipeline.get` `{workspaceId, projectId, id}`; resolusi `projectId` -> `Repo` lewat jalur yang sama dipakai `wire_git_mrs.go` (ikuti pola; jangan menyalin logika deteksi). Registrasi mandiri mengikuti pola `wire_lark*.go` (tidak mengubah `wire_git.go`). `projectId` tak dikenal -> galat terbaca.
- Tanpa unit test; parser dibuat murni dan dicek dengan fixture secara manual/lewat `go build` saja.

Acceptance criteria:
- [ ] `ListPipelines` mengembalikan paling banyak 20 `Pipeline` ternormalisasi untuk repo GitLab (glab nyata) dan GitHub (fixture).
- [ ] `GetPipeline` mengembalikan `PipelineDetail` dengan stage berisi job (nama, status, durasi, URL job) untuk GitLab; untuk GitHub job terkelompok sesuai temuan spike.
- [ ] Semua status CLI dipetakan ke tujuh nilai ternormalisasi; nilai tak dikenal menjadi `unknown`.
- [ ] Hanya perintah baca yang dipanggil (lewat allowlist); tidak ada token/kredensial yang disimpan atau dicatat.
- [ ] `git.pipelines.list` dan `git.pipeline.get` terdaftar di daemon dan mengembalikan galat terbaca (kode + pesan Indonesia) untuk CLI hilang/belum login/repo tak ditemukan/timeout.
- [ ] `go build ./...` lulus.

ownedPaths:
- `internal/gitdata/pipelines.go`
- `cmd/shikigamid/wire_git_pipelines.go`

Catatan: bisa dikerjakan paralel dengan fitur lain di epic ini. Fitur UI memakai kontrak tipe di atas, bukan kode Go.

### git-pipelines-ctl: shikigamictl git pipelines dan pipeline
Deskripsi:
Subcommand `shikigamictl git pipelines --workspace <id> --project <id>` (cetak ringkas: id, ref, status, sha pendek, durasi, url per baris) dan `shikigamictl git pipeline --workspace <id> --project <id> --id <id>` (cetak header lalu stage dan job: nama, status, durasi, url). Memanggil IPC daemon seperti subcommand `git` lain dari tiket 1/2 (`cmd_git.go`). Registrasi mandiri dari file ini (mis. lewat `init()` atau pola yang dipakai `cmd_git_mrs.go`) tanpa mengubah `cmd_git.go`. Hanya alat debug; galat daemon dicetak terbaca dan keluar non-nol.

Acceptance criteria:
- [ ] `shikigamictl git pipelines` mencetak daftar pipeline dari daemon yang berjalan.
- [ ] `shikigamictl git pipeline` mencetak stage dan job untuk id yang diberikan.
- [ ] Flag wajib yang hilang menghasilkan pesan penggunaan, bukan panic.
- [ ] Registrasi tidak mengubah file di luar ownedPaths.
- [ ] `go build ./...` lulus.

ownedPaths:
- `cmd/shikigamictl/cmd_git_pipelines.go`

Catatan: bisa dikerjakan paralel dengan fitur lain; secara runtime butuh IPC dari `git-pipelines-data`, tetapi dapat dikompilasi tanpanya (nama IPC dan bentuk hasil ada di Kontrak).

### git-pipelines-list-ui: UI daftar pipeline di panel
Deskripsi:
- `pipelineClient.ts`: `listPipelines` dan `getPipeline` bertipe, membungkus IPC `git.pipelines.list` dan `git.pipeline.get` dengan cara yang sama seperti `mrClient.ts` (klien IPC tiket 1). Mengekspor tipe pada Kontrak.
- `PipelineList.svelte`: ganti isi placeholder (props `{provider, repo}` tidak berubah). Memuat `listPipelines` untuk project `repo` saat bagian dibuka/dimount sesuai mekanisme pemuatan bagian tiket 1 (hormati batas 3 panggilan bersamaan dan galat terisolasi per bagian), keadaan memuat/kosong/galat dengan ajakan ke Pengaturan > Connector bila galat berkaitan CLI/login, dan memuat ulang saat Segarkan panel. Respons usang (ganti workspace/repo saat memuat) diabaikan. Repo yang dipakai bersama beberapa project (digabung) memakai project id pertama sesuai `repo`.
- `PipelineRow.svelte`: ref (ikon branch/tag sederhana), `StatusBadge` (tiket 2), sha pendek, penulis bila ada, durasi (`1m 23s`, "-" bila tak ada), waktu relatif, tombol Buka/Salin/Kirim ke agen memakai `actions.ts` (URL pipeline). Klik baris (di luar tombol) membuka pane `git-pipeline` dengan field leaf `provider`, `projectId`, `id` lewat API layout yang sama dipakai `MrRow` untuk membuka `git-mr`. Teks UI Indonesia; token tema terang/gelap; dapat diakses keyboard (Enter pada baris membuka pane).

Acceptance criteria:
- [ ] Bagian Pipeline menampilkan 20 pipeline terbaru dengan kolom yang disebut, tanpa filter.
- [ ] Keadaan memuat, kosong ("Tidak ada pipeline"), dan galat tampil; galat satu project tidak memengaruhi project lain.
- [ ] Segarkan panel memuat ulang daftar pipeline.
- [ ] Tombol Buka/Salin/Kirim ke agen per baris memakai URL pipeline; tanpa sesi aktif muncul toast "Tidak ada sesi aktif" (dari `actions.ts`).
- [ ] Klik baris membuka pane `git-pipeline` dengan `provider`, `projectId`, `id` yang benar; membuka id yang sama lagi memfokuskan pane yang ada bila API layout mendukung.
- [ ] Props `PipelineList` tidak berubah dari placeholder; tidak ada salinan klien IPC lain.
- [ ] Build frontend lulus.

ownedPaths:
- `frontend/src/modules/git/PipelineList.svelte`
- `frontend/src/modules/git/PipelineRow.svelte`
- `frontend/src/modules/git/pipelineClient.ts`

Catatan: bisa dikerjakan paralel dengan fitur lain; `git-pipeline-pane-ui` mengimpor `pipelineClient.ts` dari fitur ini sebagai kontrak, jadi kontrak `getPipeline(params)` pada Kontrak harus dipegang persis.

### git-pipeline-pane-ui: Pane git-pipeline
Deskripsi:
- `PipelinePane.svelte`: ganti isi placeholder (props pane dari tiket 1: field leaf `provider`, `projectId`, `id`). Memuat `getPipeline` dari `pipelineClient.ts` (kontrak dari `git-pipelines-list-ui`; jangan membuat klien lain) saat dibuka dan saat dipulihkan dari persist. Header: `StatusBadge` keseluruhan, ref, sha pendek, durasi, penulis bila ada, tombol Buka/Salin/Kirim ke agen (URL pipeline), dan tombol Segarkan. Keadaan memuat, galat terbaca, dan keadaan aman bila repo/project hilang (pesan, bukan crash; tombol aksi nonaktif). Respons usang diabaikan saat id berganti.
- `StageList.svelte`: props `{stages}`; tiap stage satu bagian dengan nama dan jumlah job; tiap job baris: nama, `StatusBadge`, durasi, link nama job ke `url` (memakai `openInBrowser`), tombol Buka/Salin/Kirim ke agen memakai URL job (nonaktif dengan keterangan bila job tak punya URL). Stage tanpa job menampilkan "Tidak ada job". Pipeline tanpa stage menampilkan "Tidak ada stage".
- Gaya mengikuti token tema; teks UI Indonesia; keyboard dapat diakses.

Acceptance criteria:
- [ ] Pane menampilkan header lengkap (status, ref, sha, durasi, aksi, Segarkan) dari `git.pipeline.get`.
- [ ] `StageList` menampilkan stage berurutan dengan job (nama, status, durasi, link job).
- [ ] Aksi baris job memakai URL job (bukan URL pipeline); nonaktif bila URL tidak ada.
- [ ] Memuat saat dibuka dan setelah pane dipulihkan; Segarkan mengambil ulang.
- [ ] Keadaan memuat, galat, dan repo hilang tampil aman tanpa crash.
- [ ] `getPipeline` diimpor dari `pipelineClient.ts`; tidak ada salinan klien.
- [ ] Build frontend lulus.

ownedPaths:
- `frontend/src/modules/git/PipelinePane.svelte`
- `frontend/src/modules/git/StageList.svelte`

Catatan: bisa dikerjakan paralel dengan fitur lain; bila `pipelineClient.ts` belum ada saat dikerjakan, pakai kontrak tipe pada Kontrak dan jangan membuat file itu (milik `git-pipelines-list-ui`).

### git-pipelines-wiring: Wiring dan verifikasi pipeline
Deskripsi:
Fitur penutup epic; berjalan setelah empat fitur lain selesai. Tidak memiliki path sendiri: hanya memeriksa dan, bila ada celah kecil di dalam path epic ini, memperbaikinya di file milik fitur terkait.
- Pastikan `git.pipelines.list` dan `git.pipeline.get` terdaftar mandiri di daemon (tanpa mengubah `wire_git.go`) dan terjawab.
- Pastikan bagian Pipeline di panel menampilkan data dan klik baris membuka pane `git-pipeline` yang memuat stage dan job; pane dipulihkan setelah restart.
- Verifikasi dengan data glab nyata (login `gitlab.com`); GitHub lewat fixture (verifikasi aplikasi nyata dicatat sebagai butir terbuka sampai `gh auth login`).
- Bila titik sambung tiket 1 (pane kind, field persist, `PipelineList` di `ProjectSection`) tidak ada atau berbeda dari kontrak, laporkan; jangan mengubah file tiket lain.

Acceptance criteria:
- [ ] Kedua IPC terdaftar mandiri dan terjawab oleh daemon yang berjalan; `shikigamictl git pipelines`/`pipeline` mencetak hasil.
- [ ] Daftar pipeline tampil di panel GitLab; klik baris membuka pane detail dengan stage dan job dari data glab nyata.
- [ ] Aksi Buka/Salin/Kirim ke agen berjalan untuk baris pipeline, header pane, dan baris job (URL job).
- [ ] Jalur GitHub berjalan terhadap fixture (parser) dan tidak crash tanpa login.
- [ ] Pane yang dipersist dipulihkan setelah restart; repo hilang -> keadaan galat aman.
- [ ] `go build ./...` dan build frontend lulus; aplikasi berjalan.

ownedPaths: (tidak ada)

Catatan: tidak paralel; jalan terakhir setelah `git-pipelines-data`, `git-pipelines-ctl`, `git-pipelines-list-ui`, dan `git-pipeline-pane-ui`.

## Path yang dimiliki
Hanya path ini yang boleh disentuh:
- `internal/gitdata/pipelines.go`
- `cmd/shikigamid/wire_git_pipelines.go`
- `cmd/shikigamictl/cmd_git_pipelines.go`
- `frontend/src/modules/git/PipelineList.svelte` (placeholder tiket 1, kini milik tiket ini)
- `frontend/src/modules/git/PipelineRow.svelte`
- `frontend/src/modules/git/PipelinePane.svelte` (placeholder tiket 1, kini milik tiket ini)
- `frontend/src/modules/git/StageList.svelte`
- `frontend/src/modules/git/pipelineClient.ts`

Epic lain (jangan disentuh): tiket 1 (`connector/gitlab.go`, `github.go`, `gitdata/doc.go`, `types.go`, `allowlist.go`, `runner.go`, `errors.go`, `repo.go`, `testdata/`, `docs/git/cli-findings.md`, `wire_git.go`, `cmd_git.go`, `modules/git/` index/store/client/actions/GitIcon/GitPanel/ProjectSection, `MrPane`/`MrList` placeholder, `modules/layout/model.ts`), tiket 2 (`mrs.go`, `wire_git_mrs.go`, `cmd_git_mrs.go`, `MrList`, `MrRow`, `StatusBadge`, `mrClient.ts`), tiket 4 (`mrdetail.go`, `wire_git_mr_detail.go`, `cmd_git_mr_detail.go`, `MrPane`, `ReviewerList`, `mrDetailClient.ts`).

## Wiring
Penutup epic ini adalah fitur `git-pipelines-wiring`. Titik ekstensi sudah disediakan tiket 1: `ProjectSection` merender `PipelineList` (props `{provider, repo}`), pane kind `git-pipeline` terdaftar dengan field leaf `provider`, `projectId`, `id`, dan komponen `PipelinePane` sudah terpasang sebagai placeholder. Karena isi placeholder diganti tanpa mengubah props, tidak ada titik sambung baru di file tiket lain. Wiring menyambungkan: (1) IPC pipeline ke daemon lewat `wire_git_pipelines.go` mandiri; (2) `PipelineRow` ke pane `git-pipeline` lewat API layout; (3) `PipelinePane` ke `StageList` dan `pipelineClient.ts`; (4) `shikigamictl git pipelines/pipeline` ke daemon. Hasilnya terlihat dan jalan: buka panel Git -> bagian Pipeline -> klik pipeline -> stage dan job -> Buka/Salin/Kirim ke agen. Tiket ini hanya bergantung pada tiket 1 dan 2 yang sudah selesai (StatusBadge dari tiket 2) dan tidak bergantung pada tiket 4.

## Guard
Cukup build berhasil; tanpa unit test:
- `go build ./...`
- Build frontend (`npm run build` di `frontend/`, atau perintah build/check yang dipakai tiket sebelumnya)
