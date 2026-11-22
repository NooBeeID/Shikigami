# Indeks tiket: Connector Git (MR/PR dan pipeline)

Sumber: `docs/superpowers/specs/2026-11-22-10-git-connector-design.md`

## Ringkasan tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|-------|-------|------|------------|--------------|
| git-connector-ticket-01-git-repos | Connector Git dan deteksi repo | 1 | - | 6 |
| git-connector-ticket-02-git-mrs | Daftar MR/PR | 2 | git-connector-ticket-01-git-repos | 4 |
| git-connector-ticket-03-git-pipelines | Daftar pipeline dan detail pipeline | 3 | git-connector-ticket-02-git-mrs | 5 |
| git-connector-ticket-04-git-mr-detail | Detail MR/PR | 4 | git-connector-ticket-03-git-pipelines | 4 |

Fitur per tiket:

- **git-connector-ticket-01-git-repos**: git-cli-spike, git-connector-adapters, gitdata-foundation, git-repos-ipc-ctl, git-rail-panel-ui, git-repos-wiring
- **git-connector-ticket-02-git-mrs**: git-mrs-data, git-mrs-ctl, git-mrs-ui, git-mrs-wiring
- **git-connector-ticket-03-git-pipelines**: git-pipelines-data, git-pipelines-ctl, git-pipelines-list-ui, git-pipeline-pane-ui, git-pipelines-wiring
- **git-connector-ticket-04-git-mr-detail**: git-mr-detail-data, git-mr-detail-ctl, git-mr-pane-ui, git-mr-detail-wiring

Total: 19 fitur.

## Prasyarat sebelum implementasi

- `main-rail-ticket-01-main-rail` sudah di-merge. Yang dipakai: `registerRailItem` (dengan `visible` berupa `Readable<boolean>`) dan `registerPaneKind` di `frontend/src/modules/layout/registry.ts`, serta `PaneKind` di `frontend/src/modules/layout/model.ts`.
- Kerangka connector (`internal/connector`: `Register` dari `init()`, `RunCLI` dengan PATH login-shell, `Status`, `ConnectGuide`) sudah tersedia dari `settings-framework`.
- `lark-connector` sudah di-merge bila pola `actions.ts` (`openInBrowser`, `copyLink`, `sendToAgent`) dan `pasteToSession` dipakai ulang. Tiket 01 git mengekspor fungsi aksi bersama ini di `frontend/src/modules/git/actions.ts`.
- Perubahan working tree yang belum di-commit harus sudah di-commit sebelum tiket 01 dikerjakan.

## Urutan eksekusi per wave

Tiket dalam satu wave bisa dijalankan paralel. Wave berikutnya menunggu wave sebelumnya selesai dan di-merge. Karena keempat tiket berantai (setiap tiket bergantung pada tiket sebelumnya), setiap wave berisi satu tiket.

- **Wave 1**: git-connector-ticket-01-git-repos
  - Paralel: `git-cli-spike`, `git-connector-adapters`, `gitdata-foundation`, `git-rail-panel-ui` (memakai kontrak tipe dan IPC, bukan implementasi Go).
  - Lalu: `git-repos-ipc-ctl` setelah `gitdata-foundation`, dan `git-repos-wiring` terakhir.
- **Wave 2**: git-connector-ticket-02-git-mrs (menunggu wave 1)
  - Paralel: `git-mrs-data`, `git-mrs-ui` (memakai kontrak IPC `git.mrs.list`).
  - Lalu: `git-mrs-ctl` setelah `git-mrs-data`, dan `git-mrs-wiring` terakhir.
- **Wave 3**: git-connector-ticket-03-git-pipelines (menunggu wave 2)
  - Paralel: `git-pipelines-data`, `git-pipelines-list-ui`, `git-pipeline-pane-ui`.
  - Lalu: `git-pipelines-ctl` setelah `git-pipelines-data`, dan `git-pipelines-wiring` terakhir.
- **Wave 4**: git-connector-ticket-04-git-mr-detail (menunggu wave 3)
  - Paralel: `git-mr-detail-data`, `git-mr-pane-ui`.
  - Lalu: `git-mr-detail-ctl` setelah `git-mr-detail-data`, dan `git-mr-detail-wiring` terakhir.

Ringkasan jalur paralel maksimum: wave 1 (4 fitur paralel), wave 2 sampai 4 (3 fitur paralel per wave).

## Kontrak bersama

### Konvensi

- Go module `github.com/NooBeeID/shikigami`.
- Teks UI berbahasa Indonesia; identifier (kode, nama fungsi, nama field) berbahasa Inggris.
- Semua operasi read-only. Tidak ada token atau kredensial di config app. Tidak ada polling; data dimuat saat panel/pane dibuka dan lewat tombol Segarkan.
- Tidak ada unit test. Guard verifikasi per tiket: `go build ./...` dan build frontend.
- Paket `internal/gitdata` tidak mengimpor `internal/workspace`. Arah dependensi: `gitdata -> connector`.
- Galat IPC memakai bentuk galat connector/Lark yang sama (kode + pesan Indonesia).
- Setiap file `wire_git*.go` mendaftarkan IPC-nya sendiri (pola `wire_lark*.go`). Setiap `cmd_git*.go` melakukan registrasi mandiri.

### 1) Tipe Go ternormalisasi

Dimiliki oleh git-repos di `internal/gitdata/types.go`. Tiket lain hanya memakai. Json memakai camelCase.

- `Repo{ProjectID, ProjectName, Provider "gitlab"|"github", Host, Path, WebURL}`. Project yang berbagi remote digabung per host+path, membawa daftar projectId dan nama.
- `MergeRequest{Number, Title, Author, SourceBranch, TargetBranch, State "open"|"merged"|"closed", Draft, ReviewState, PipelineStatus, UpdatedAt, URL}`
- `MergeRequestDetail` = `MergeRequest` + `Description`, `Labels`, `Reviewers`, `Mergeable`, `LastPipeline{id, status, url}`
- `Pipeline{ID, Ref, SHA, Status, Author, DurationSec, UpdatedAt, URL}`
- `PipelineDetail` = `Pipeline` + `Stages []{Name, Jobs []{Name, Status, DurationSec, URL}}`
- Status pipeline: `pending | running | success | failed | canceled | skipped | unknown`. Status GitHub (`queued`, `in_progress`, `completed` + conclusion) dipetakan ke himpunan yang sama.
- Bentuk persis field opsional mengikuti `docs/git/cli-findings.md`.
- Fixture di `internal/gitdata/testdata/` dan hasil spike di `docs/git/cli-findings.md` (dibuat oleh git-repos) adalah acuan semua parser.

### 2) Eksekusi CLI

- Hanya perintah baca, lewat allowlist (pola `internal/larkdata/allowlist.go`).
- Maksimal 3 pemanggilan CLI bersamaan.
- Timeout 20 detik per pemanggilan.
- Flag `-R` berbentuk `OWNER/REPO` (atau `GROUP/SUBGROUP/REPO`) untuk host bawaan (`gitlab.com`, `github.com`), dan `HOST/PATH` untuk host self-hosted.

### 3) IPC

| Method | Parameter | Didaftarkan di | Tiket |
|--------|-----------|----------------|-------|
| `git.repos.list` | `{workspaceId}` | `wire_git.go` | 01 |
| `git.mrs.list` | `{workspaceId, projectId, state}` | `wire_git_mrs.go` | 02 |
| `git.pipelines.list` | `{workspaceId, projectId}` | `wire_git_pipelines.go` | 03 |
| `git.pipeline.get` | `{workspaceId, projectId, id}` | `wire_git_pipelines.go` | 03 |
| `git.mr.get` | `{workspaceId, projectId, number}` | `wire_git_mr_detail.go` | 04 |

### 4) Frontend: `frontend/src/modules/git/`

- Titik ekstensi yang dibuat oleh git-repos:
  - `GitPanel` memuat `ProjectSection`, yang memuat `MrList` dan `PipelineList` dengan props `{provider, repo}`.
  - Dua item rail: `git-gitlab` (order 20) dan `git-github` (order 21), masing-masing `visible` dari store repo.
  - Pane kind `git-mr` dan `git-pipeline` di `PaneKind` (`modules/layout/model.ts`). Field leaf yang dipersist: `provider`, `projectId`, `number` (untuk MR) atau `id` (untuk pipeline).
  - Komponen placeholder `MrList`, `PipelineList`, `MrPane`, `PipelinePane` dimiliki git-repos. Tiket pemiliknya (02, 03, 04) mengambil alih kepemilikan file placeholder itu dan menggantinya, tanpa mengubah props.
- `actions.ts` (tiket 01) mengekspor `openInBrowser(url)`, `copyLink(url)`, dan `sendToAgent(url)`. `sendToAgent` memakai `pasteToSession` ke sesi aktif: URL plus spasi, tanpa Enter. Tanpa sesi aktif, tampilkan toast "Tidak ada sesi aktif".
- `StatusBadge.svelte` (dibuat di git-mrs) dipakai ulang oleh `git-pipelines` dan `git-mr-detail`.
- Deteksi repo: hasil di memori. Deteksi berjalan saat app mulai, saat workspace berganti, saat project ditambah atau dihapus, dan saat Segarkan.

## Ringkasan isi per tiket

- **01 git-repos**: CLI spike dan adapter `gitlab`/`github`, paket `gitdata` dengan tipe ternormalisasi, IPC `git.repos.list`, rail dan panel kerangka, lalu wiring deteksi repo.
- **02 git-mrs**: data daftar MR/PR per status (Terbuka, Digabung, Tertutup), IPC `git.mrs.list`, UI daftar dengan `StatusBadge`, lalu wiring.
- **03 git-pipelines**: data daftar dan detail pipeline, IPC `git.pipelines.list` dan `git.pipeline.get`, UI daftar pipeline, pane `git-pipeline` (stage dan job), lalu wiring.
- **04 git-mr-detail**: data detail MR (`MergeRequestDetail`), IPC `git.mr.get`, pane `git-mr` (judul, label, reviewer, deskripsi, pipeline terakhir), lalu wiring.
