# Overview: git-connector-ticket-02-git-mrs (Daftar MR/PR)

Sumber: tiket `docs/tickets/git-connector-ticket-02-git-mrs.md` dan spec `docs/superpowers/specs/2026-11-22-10-git-connector-design.md`. Dokumen ini adalah sumber kebenaran kontrak bersama; plan per fitur hanya merinci langkah kerja. Module Go: `github.com/NooBeeID/shikigami`. Bila overview ini berselisih dengan tiket, tiket menang kecuali dinyatakan lain di sini (selisih yang disengaja ditandai "DIPUTUSKAN").

## 0. Keadaan kode saat ini (diverifikasi dari repo; tiket 1 sudah merge)

Backend (`internal/gitdata/`, sudah ada, TIDAK boleh diubah):
- `types.go`: `Provider` (`ProviderGitLab`/`ProviderGitHub`), `Repo{ProjectID, ProjectName, Projects, Provider, Host, Path, WebURL}`, `Status` + konstanta `StatusPending|Running|Success|Failed|Canceled|Skipped|Unknown`, `MergeRequest{Number int, Title, Author, SourceBranch, TargetBranch, State string, Draft bool, ReviewState string (omitempty), PipelineStatus Status (omitempty), UpdatedAt string, URL string}`, `NormalizeGitLabStatus(s)`, `NormalizeGitHubStatus(status, conclusion)`.
- `runner.go`: `Run(ctx, tool, args...) ([]byte, error)`; allowlist, `CallTimeout` 20 detik, semaphore `MaxConcurrent` 3, galat sudah `*gitdata.Error` (via `classify`). Pakai ini, jangan memanggil `connector.RunCLITimeout` langsung.
- `errors.go`: `*Error{Code, Message}`, `AsError`, kode `cli_missing|not_logged_in|permission|not_found|timeout|not_allowed|cli_error`.
- `repo.go`: `RepoFlag(r Repo)` mengembalikan `Path` untuk `gitlab.com`/`github.com` dan `Host/Path` untuk self-hosted. Selain itu, `docs/git/cli-findings.md` bagian 9 menyatakan `HOST/PATH` tidak andal untuk `glab` self-hosted (harus URL penuh `https://HOST/PATH`); `repo.go` belum disesuaikan dan TIDAK boleh disentuh. DIPUTUSKAN: `mrs.go` memakai helper privat sendiri `mrRepoArg(r Repo) string`: GitLab host `gitlab.com` -> `r.Path`; GitLab host lain -> `"https://" + r.Host + "/" + r.Path`; GitHub host `github.com` -> `r.Path`; GitHub host lain -> `r.Host + "/" + r.Path` (format `[HOST/]OWNER/REPO`). Jangan memanggil `RepoFlag`.
- `allowlist.go` sudah mengizinkan: `glab mr list` dengan flag `-R --repo -F --output -P --per-page -p --page --merged --closed --all --draft --source-branch --target-branch -A --author --label -l`; `gh pr list` dengan flag `-R --repo --json -L --limit -s --state -b --base -H --head -A --author --draft -d -S --search -l --label`. Gunakan HANYA bentuk panjang (`--merged`, `--closed`, `--per-page`, `--state`, `--limit`, `--repo`, `--output`). Flag `-M` dan `-c` TIDAK ada di allowlist; jangan dipakai. Bila perintah/flag yang dibutuhkan belum diizinkan, laporkan; jangan ubah file itu.
- `docs/git/cli-findings.md` + `testdata/glab-mr-list.json` (NYATA) dan `testdata/gh-pr-list.json` (ASUMSI) adalah acuan parser.

Backend daemon/ctl (sudah ada, tidak boleh diubah, dipakai ulang):
- `cmd/shikigamid/wire_git.go`: modul `git` terdaftar via `RegisterModule("git", ...)`. Helper paket `main` yang BOLEH dipanggil dari file baru: `gitDecode(params, &v)`, `gitResolveRepo(d, workspaceID, projectID) (gitdata.Repo, error)` (galat `not_found`: "Project tidak ditemukan atau tidak punya remote dikenal"), `mapGitError(err) error`, `gitErr(code, ipcCode, msg) error`. `ipc.NewError` membentuk pesan `"[kode] pesan"`.
- `cmd/shikigamictl/cmd_git.go`: induk `git` dengan `registerGitSub(name string, fn gitSubFunc)` dan `type gitSubFunc func(ctx context.Context, c *ipc.Client, args []string) error`; `callAndPrint` ada di `registry.go`. Subcommand `repos` memakai flag `--workspace`.

Frontend (`frontend/src/modules/git/`, sudah ada):
- `client.ts`: `Provider`, `Repo` (TIDAK punya `workspaceId`; hanya `projectId`, `projectName`, `projects`, `provider`, `host`, `path`, `webUrl`), `GitError {code, message}`, `parseGitError(x)` (mengurai `"[kode] pesan"` dan objek), pemanggil IPC `call` dari `../../lib/ipc`.
- `store.ts`: `repoKey(r)` = `host + '/' + path`; `selectedState = writable<Record<string, 'open'|'merged'|'closed'>>({})` (peta di memori, kunci `repoKey`, SUDAH ADA untuk tiket ini); `collapsed`, `refreshRepos()`, `gitRepos`, `gitState`. Workspace aktif: `activeWorkspaceId` dari `../../lib/stores/workspaces` (store Svelte).
- `actions.ts`: `openInBrowser(url)`, `copyLink(url)`, `sendToAgent(url)` (toast "Tidak ada sesi aktif" sudah ditangani di dalamnya).
- `ProjectSection.svelte` sudah merender `<MrList {provider} {repo} />` di dalam `{#if open}`, jadi bagian terlipat tidak memuat. `MrList` saat ini placeholder (props `{provider: Provider; repo: Repo}` lewat `$props()`, Svelte 5 runes).
- Mekanisme Segarkan: tombol di `GitPanel` memanggil `refreshRepos()`, yang MENGGANTI `gitRepos` dengan objek `Repo` baru; `ProjectSection` di-key oleh `repoKey`, jadi komponen anak tidak di-remount tetapi prop `repo` menerima objek baru (identitas berubah). DIPUTUSKAN: `MrList` memuat ulang saat identitas objek `repo` berubah (efek yang bergantung pada `repo`), selain saat mount dan saat status berganti. Tidak ada perubahan di file tiket 1.
- Pembukaan pane: `layout.openPane(leaf: NewLeaf)` dari `../../lib/stores/layout` (pola `modules/lark/sections/TicketSection.svelte`: `layout.openPane({ kind: 'lark-board', sourceId, title })`). `NewLeaf` sudah punya `provider`, `projectId`, `number`, `pipelineId`, `title`. Pane kind `git-mr` sudah terdaftar di `modules/git/index.ts` (komponen `MrPane` placeholder).
- Belum ada util waktu relatif khusus; tulis fungsi kecil lokal di `MrRow.svelte` (grep terakhir menunjukkan tidak ada util bersama).
- Stile: variabel CSS yang dipakai tiket 1 (`--fg`, `--fg-muted`, `--bg-elev`, `--bg-hover`, `--border`, `--danger`, dengan fallback gelap). Untuk status gunakan variabel yang sudah ada di tema (cek `frontend/src/modules/shell/shell.css` / global CSS untuk `--success`/`--warn`/`--accent`); bila tidak ada, pakai fallback `var(--success, #4caf7a)` dst. dan jangan menambah file global.

## 1. Ringkasan epic dan urutan eksekusi

Epic mengisi daftar MR (GitLab) dan PR (GitHub) per project di panel Git: pemilih status Terbuka/Digabung/Tertutup, baris lengkap, serta aksi Buka di browser / Salin link / Kirim ke agen, klik baris membuka pane `git-mr` placeholder. Read-only, semua data lewat `glab`/`gh`.

Urutan eksekusi:
1. Foundation: TIDAK ADA. Semua tipe, runner, allowlist, store, aksi, rute pane sudah disediakan tiket 1; kontrak IPC di bagian 2 cukup sebagai antarmuka bersama. Tidak ada `00-foundation.md`.
2. Fitur paralel (tiga agen, tanpa saling menunggu): `git-mrs-data`, `git-mrs-ctl`, `git-mrs-ui`.
3. Wiring (`git-mrs-wiring`): verifikasi dan penyambungan, setelah tiga fitur selesai.

## 2. Kontrak bersama

### 2.1 IPC `git.mrs.list`

Params (json camelCase): `{"workspaceId": string, "projectId": string, "state": "open" | "merged" | "closed"}`. `state` kosong atau di luar tiga nilai -> galat validasi `invalid_params` (`ipc.ErrInvalid`). `workspaceId`/`projectId` kosong -> `invalid_params`.

Hasil: objek BUNGKUS `{"mrs": MergeRequest[]}`. `mrs` SELALU array (tidak pernah `null`; kosong = `[]`). Elemen sesuai `gitdata.MergeRequest`:
```ts
type MergeRequestState = 'open' | 'merged' | 'closed';
interface MergeRequest {
  number: number;
  title: string;
  author: string;
  sourceBranch: string;
  targetBranch: string;
  state: MergeRequestState;
  draft: boolean;
  reviewState?: string;     // hanya GitHub dan hanya bila ada: 'approved' | 'changes_requested' | 'review_required'
  pipelineStatus?: string;  // 'pending'|'running'|'success'|'failed'|'canceled'|'skipped'|'unknown'; hanya bila ada
  updatedAt: string;        // RFC3339
  url: string;
}
```
Hasil spike (`cli-findings.md` bagian 3.1, 10): GitLab TIDAK memuat review/pipeline di daftar, jadi `reviewState`/`pipelineStatus` dihilangkan (omitempty) untuk GitLab dan baris GitLab tanpa badge. GitHub memuat keduanya (ASUMSI, fixture). `reviewState` bernilai lowercase snake_case: `APPROVED`->`approved`, `CHANGES_REQUESTED`->`changes_requested`, `REVIEW_REQUIRED`->`review_required`, `""`->dihilangkan.

Galat IPC: bentuk Lark/git yang sama, pesan `"[kode] pesan Indonesia"` dengan kode dari `gitdata.ErrorCode` ditambah `invalid_params`. Pemetaan lewat `mapGitError` (not_found -> `ipc.ErrNotFound`, not_allowed -> `ipc.ErrInvalid`, lainnya `ipc.ErrInternal`).

### 2.2 Go (`internal/gitdata/mrs.go`, package `gitdata`)

```go
func ListMergeRequests(ctx context.Context, repo Repo, state string) ([]MergeRequest, error)
```
- Mengembalikan slice non-nil; galat bertipe `*gitdata.Error`. `state` tidak dikenal -> DIPUTUSKAN: tambahkan kode lokal tanpa mengubah `errors.go`: kembalikan `&Error{Code: "invalid_params", Message: "Status MR tidak dikenal: <state>"}` (`ErrorCode` adalah string, jadi nilai baru legal di `mrs.go`). `wire_git_mrs.go` memvalidasi `state` lebih dulu, sehingga ini hanya sabuk pengaman.
- Perintah GitLab: `glab mr list --repo <arg> --output json --per-page 50` ditambah: `open` -> tanpa flag state; `merged` -> `--merged`; `closed` -> `--closed`. Jangan pakai `--all`. Filter `state` hasil tetap diperiksa di Go (buang elemen yang state ternormalisasinya tidak sama dengan `state` diminta, sabuk pengaman terhadap perilaku flag). `glab` bisa mengembalikan `null` atau keluaran kosong untuk nol hasil -> `[]`.
- Perintah GitHub: `gh pr list --repo <arg> --state open|merged|closed --limit 50 --json number,title,author,headRefName,baseRefName,state,isDraft,reviewDecision,statusCheckRollup,updatedAt,url`.
- Pemetaan (lihat `cli-findings.md` bagian 3.1, 4, 6 untuk detail normalisasi):
  - GitLab: `iid`->Number, `title`, `author.username`->Author, `source_branch`, `target_branch`, `state` (`opened`->`open`, `merged`, `closed`, `locked`->`closed`), `draft`, `updated_at` (apa adanya), `web_url`.
  - GitHub: `number`, `title`, `author.login`, `headRefName`->SourceBranch, `baseRefName`->TargetBranch, `state` (`OPEN/MERGED/CLOSED` -> lowercase), `isDraft`, `reviewDecision`->ReviewState (lihat 2.1), `statusCheckRollup[]`->PipelineStatus (agregasi: kosong -> kosong/tanpa badge; ada `failed` -> `failed`; lalu `running`; lalu `pending`; semua `success`/`skipped` dengan minimal satu `success` -> `success`; selain itu `unknown`; entri `CheckRun` lewat `NormalizeGitHubStatus(status, conclusion)`, entri `StatusContext` lewat `state` dengan `success`->`success`, `failure`/`error`->`failed`, `pending`/`expected`->`pending`), `updatedAt`, `url`.
- Urut hasil: `UpdatedAt` menurun (terbaru dulu) di Go; gagal-parse waktu diperlakukan sebagai paling lama; sort stabil.
- Tanpa panggilan per baris. Hanya `Run(ctx, "glab"|"gh", ...)`; tanpa import `internal/workspace` dan tanpa `os/exec`.
- Parser toleran: field hilang/null tidak panic; JSON rusak -> `&Error{Code: CodeCLI, Message: "<GitLab|GitHub> mengembalikan data yang tidak dapat dibaca"}` (pakai `providerLabel`).

### 2.3 `cmd/shikigamid/wire_git_mrs.go`

Registrasi mandiri: `func init() { RegisterModule("git-mrs", func(srv *ipc.Server, d *Deps) { registerGitMrs(srv, d) }) }` (nama modul `git-mrs` harus unik; pola `wire_lark_tickets.go`). Handler `git.mrs.list`: decode params (`gitDecode`), validasi, `gitResolveRepo(d, workspaceID, projectID)`, `gitdata.ListMergeRequests(ctx, repo, state)` dengan `ctx` handler, `mapGitError`, kembalikan `map[string]any{"mrs": mrs}` (pastikan `mrs` non-nil). Tanpa state bersama.

### 2.4 `cmd/shikigamictl/cmd_git_mrs.go`

Subcommand `git mrs` lewat `func init() { registerGitSub("mrs", gitMrs) }`. DIPUTUSKAN: bentuk argumen posisional sesuai tiket: `shikigamictl git mrs <workspaceId> <projectId> <state>` (berbeda dari `git repos --workspace`, tiket menang). Pesan penggunaan: `penggunaan: shikigamictl git mrs <workspaceId> <projectId> open|merged|closed`. Satu baris per MR, lalu `<n> baris`. Nomor `!n` bila `url` memuat `/-/merge_requests/`, `#n` bila memuat `/pull/`, selain itu `n`.

### 2.5 Frontend

`mrClient.ts` (milik `git-mrs-ui`; ekspor tipe dan fungsi):
```ts
export type MergeRequestState = 'open' | 'merged' | 'closed';
export interface MergeRequest { /* sama dengan 2.1 */ }
export async function listMergeRequests(params: { workspaceId: string; projectId: string; state: MergeRequestState }): Promise<MergeRequest[]>;
```
Memanggil `call<{ mrs: MergeRequest[] | null }>('git.mrs.list', params)` dari `../../lib/ipc`, melempar `parseGitError(e)` dari `./client` (impor, jangan salin), mengembalikan `r?.mrs ?? []`. `workspaceId` diambil `MrList` dari `activeWorkspaceId` (`get()` dari `svelte/store`); bila `null`, tampilkan keadaan kosong tanpa memanggil IPC.

Komponen (Svelte 5 runes, `$props()`):
- `StatusBadge.svelte`: `{ status: string; kind?: 'pipeline' | 'review' | 'mr' }`. Tabel label (Indonesia) per `kind`:
  - `pipeline` (default bila kind tidak diberikan dan status termasuk himpunan pipeline): `pending` Menunggu, `running` Berjalan, `success` Berhasil, `failed` Gagal, `canceled` Dibatalkan, `skipped` Dilewati, `unknown` Tidak diketahui.
  - `review`: `approved` Disetujui, `changes_requested` Perlu perubahan, `review_required` Belum direview.
  - `mr`: `open` Terbuka, `merged` Digabung, `closed` Tertutup.
  - Status di luar tabel: tampilkan teks status apa adanya dengan warna netral. Badge memuat ikon/glyph kecil (bukan hanya warna) dan `title`. Komponen tidak mengimpor apa pun dari MR (dipakai ulang tiket 3 dan 4).
- `MrRow.svelte`: `{ mr: MergeRequest; provider: 'gitlab' | 'github'; onOpen: (mr: MergeRequest) => void }`.
- `MrList.svelte`: props tetap `{ provider: Provider; repo: Repo }`.

### 2.6 Pane `git-mr`

`layout.openPane({ kind: 'git-mr', provider, projectId: repo.projectId, number: mr.number, title: <'!' atau '#'> + number + ' ' + title })`. Field `workspaceId` tidak diisi (tiket menetapkan leaf `provider/projectId/number`); tiket 4 menentukan sisanya.

### 2.7 Konvensi

Teks UI bahasa Indonesia; identifier kode bahasa Inggris; tanpa dependensi baru; tanpa unit test; tanpa polling; tanpa token di config; maks 3 panggilan CLI bersamaan (runner yang menjaga; UI hanya memuat saat bagian tampil).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `git-mrs-data` | `gitdata.ListMergeRequests` (GitLab + GitHub), IPC `git.mrs.list` | `internal/gitdata/mrs.go`, `cmd/shikigamid/wire_git_mrs.go` |
| `git-mrs-ctl` | `shikigamictl git mrs <workspaceId> <projectId> <state>` | `cmd/shikigamictl/cmd_git_mrs.go` |
| `git-mrs-ui` | `MrList`, `MrRow`, `StatusBadge`, `mrClient` | `frontend/src/modules/git/MrList.svelte`, `MrRow.svelte`, `StatusBadge.svelte`, `mrClient.ts` |
| `git-mrs-wiring` | verifikasi dan penyambungan (tanpa file sendiri) | (tidak ada) |

Rincian per fitur (acceptance criteria lengkap ada di tiket; plan per fitur wajib memetakannya):
- `git-mrs-data`: `mrs.go` + `wire_git_mrs.go` sesuai 2.2 dan 2.3. Galat dari CLI hilang/belum login/izin/repo tidak ada/timeout/project tidak ada menjadi galat IPC berpesan Indonesia tanpa panic. Tidak ada perintah tulis; semua lewat `Run`.
- `git-mrs-ctl`: sesuai 2.4; state selain `open|merged|closed` atau argumen kurang -> pesan penggunaan; galat daemon dikembalikan sebagai `error` (kode keluar bukan nol ditangani `main.go` ctl); tidak mengubah `cmd_git.go`.
- `git-mrs-ui`: sesuai 2.5/2.6. `MrList`:
  - Status terpilih dibaca/ditulis lewat `selectedState` (`store.ts`) dengan kunci `repoKey(repo)`, default `open`; bertahan saat unmount selama app hidup.
  - Memuat saat mount, saat status berganti, dan saat identitas objek `repo` berubah (Segarkan). Pakai penghitung urutan (`let seq = 0`) sehingga respons usang diabaikan; jangan memuat saat unmount (hentikan efek).
  - Keadaan: memuat; kosong (`Tidak ada MR terbuka|digabung|tertutup` GitLab, `Tidak ada PR ...` GitHub); galat dengan pesan terbaca + tombol "Coba lagi" (+ "Buka Pengaturan > Connector" via `openSettings('connectors')` dari `../settings` untuk kode `cli_missing`/`not_logged_in`, pola `GitPanel.svelte`).
  - Segmented control Terbuka | Digabung | Tertutup dengan `role="tablist"`/`aria-pressed` yang dapat diakses keyboard.
  - `MrRow`: judul dipotong ellipsis dengan `title` penuh; nomor `!`/`#`; penulis; `sourceBranch -> targetBranch`; penanda "Draft"; `StatusBadge kind="review"` hanya bila `reviewState`; `StatusBadge kind="pipeline"` hanya bila `pipelineStatus`; waktu relatif ("3 jam lalu", "baru saja", "2 hari lalu", dst.) dari `updatedAt`; tiga tombol ikon (Buka di browser, Salin link, Kirim ke agen) dengan `aria-label`, klik tombol memanggil `event.stopPropagation()`; baris `role="button"`/`tabindex="0"` dengan Enter/Spasi memanggil `onOpen`.
  - Klik baris -> `layout.openPane(...)` (2.6).

## 4. Foundation

Tidak ada. Alasan: tipe `MergeRequest`, `Status`, runner, allowlist, `selectedState`, `actions.ts`, pane `git-mr`, field leaf `number`/`provider`/`projectId`, serta titik ekstensi `ProjectSection` -> `MrList` sudah disediakan tiket 1 dan terverifikasi ada di repo. Output: `hasFoundation=false`, tidak ada `00-foundation.md`.

## 5. Wiring checklist (untuk `git-mrs-wiring`)

Tanpa file milik sendiri; hanya verifikasi, dan koreksi kecil BILA ada kekurangan tanpa menyentuh file tiket lain (laporkan bila kekurangan ada di file di luar ownedPaths epic ini):
1. `cmd/shikigamid/wire_git_mrs.go` terdaftar mandiri: `RegisterModule("git-mrs", ...)` ada, nama modul tidak bentrok; `go build ./...` lulus; daemon dijalankan dan `git.mrs.list` terjawab (cek lewat `shikigamictl git mrs ...`).
2. `cmd/shikigamictl/cmd_git_mrs.go` terdaftar mandiri: `registerGitSub("mrs", ...)` dipanggil dari `init()`; `shikigamictl git` tanpa argumen mencantumkan `mrs`.
3. `MrList.svelte` sudah dipakai `ProjectSection.svelte` (sudah: `<MrList {provider} {repo} />`); tidak perlu mengubah file tiket 1. Pastikan props `{provider, repo}` cocok dan komponen tampil di panel GitLab/GitHub.
4. Klik baris membuka pane `git-mr` (terdaftar di `modules/git/index.ts`); leaf `provider/projectId/number` dipersist dan dipulihkan setelah restart (`layout/model.ts` sudah memetakan field itu di `Leaf`, `NewLeaf`, pembuat leaf, dan `hydrateNode`).
5. Verifikasi di app (GitLab, data nyata `glab`, project dengan remote gitlab.com): daftar Terbuka/Digabung/Tertutup, ganti status, bertahan setelah panel di-unmount (pindah rail lalu kembali), Salin link, Kirim ke agen (tanpa sesi aktif -> toast "Tidak ada sesi aktif"), tombol Segarkan, galat CLI/izin tampil aman, bagian terlipat tidak memanggil IPC. Jalur GitHub dibuktikan lewat fixture; catat butir terbuka: "verifikasi daftar PR di app menunggu `gh auth login`" di laporan akhir (tidak memblokir).
6. Selisih yang perlu dilaporkan (bukan diperbaiki di file tiket 1): `RepoFlag` self-hosted `HOST/PATH` vs URL penuh (sudah dihindari lewat `mrRepoArg`); catat bila ada self-hosted nyata yang bermasalah.

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` (dari akar repo)
- Frontend: `cd frontend && npm run build` (tambahkan `npm run check` bila skrip itu ada di `frontend/package.json`)
- Fitur yang hanya menyentuh Go cukup menjalankan `go build ./...`; fitur frontend menjalankan build frontend. Wiring menjalankan keduanya dan menjalankan aplikasi.

## 7. Aturan paralel

- Tiap agen hanya menyentuh `ownedPaths` fiturnya. Jangan menyentuh path fitur lain, file wiring, atau file tiket 1/3/4 (`types.go`, `allowlist.go`, `runner.go`, `errors.go`, `repo.go`, `testdata/`, `docs/git/cli-findings.md`, `wire_git.go`, `cmd_git.go`, `store.ts`, `client.ts`, `actions.ts`, `GitPanel.svelte`, `ProjectSection.svelte`, `index.ts`, `layout/model.ts`, `PipelineList.svelte`, `MrPane.svelte`, dan seterusnya). Membaca dan memanggil helper di file itu diperbolehkan.
- Bila kebutuhan tidak terpenuhi tanpa mengubah file di luar ownedPaths (perintah belum di allowlist, tipe kurang, field store kurang), laporkan di hasil fitur; jangan ubah file itu.
- Jangan mendefinisikan simbol dengan nama yang mungkin bentrok di paket yang sama: di paket `gitdata` awali helper privat dengan `mr`/`parseMr`; di paket `main` (daemon) awali dengan `gitMrs`; di paket `main` (ctl) awali dengan `gitMrs`.
- Tanpa unit test, tanpa dependensi baru, tanpa commit di luar proses `implement-feature`.
- Ketiga fitur paralel memakai kontrak bagian 2 apa adanya dan tidak menunggu satu sama lain; frontend dan ctl tidak bergantung pada backend selesai untuk build.
