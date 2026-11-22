# Plan: Wiring dan verifikasi pipeline

## Metadata
- Tiket: `git-connector-ticket-03-git-pipelines` (`docs/tickets/git-connector-ticket-03-git-pipelines.md`)
- Fitur: `git-pipelines-wiring` (fitur penutup, tanpa file sendiri)
- ownedPaths: (kosong) hanya verifikasi dan pelaporan
- Depends on foundation: tidak (epic ini tidak punya foundation; lihat overview bagian 4). Bergantung pada selesainya `git-pipelines-data`, `git-pipelines-ctl`, `git-pipelines-list-ui`, dan `git-pipeline-pane-ui`; jalankan HANYA setelah keempatnya selesai dan ter-merge ke worktree kerja.

## Tujuan
Memastikan empat fitur paralel tersambung dan berjalan end-to-end tanpa menambah file: modul daemon `git-pipelines` terdaftar mandiri dan menjawab `git.pipelines.list` serta `git.pipeline.get`; subcommand `shikigamictl git pipelines` dan `git pipeline` terdaftar; `PipelineList` tampil di `ProjectSection`; klik `PipelineRow` membuka pane `git-pipeline` dengan `pipelineId`; `PipelinePane` memakai `StageList` dan `pipelineClient.ts`; pane dipersist dan dipulihkan; perilaku terbukti dengan data `glab` nyata (GitLab); GitHub dibuktikan lewat fixture dan dicatat sebagai butir terbuka (menunggu `gh auth login`). `go build ./...` dan build frontend lulus, aplikasi berjalan.

## Path yang dimiliki
Tidak ada. Fitur ini TIDAK boleh membuat atau mengubah file apa pun di repo (termasuk file milik empat fitur lain, file wiring/tiket 1, 2, 4, dan `overview.md`). Jangan menyentuh file wiring atau path fitur lain. Semua temuan kekurangan dilaporkan di hasil akhir (file, baris, gejala, usulan perbaikan), bukan diperbaiki. Artefak sementara (log, keluaran, skrip `go run` uji fixture, biner ctl) ditaruh di scratchpad/di luar repo.

## Kontrak
Tidak menyediakan apa pun; hanya memverifikasi kontrak overview, merujuk bagian berikut (jangan diulang):
- Overview 2.1 dan 2.2: IPC `git.pipelines.list` (hasil ARRAY langsung, tidak pernah `null`, maks 20) dan `git.pipeline.get` (`id` digit saja, `stages` selalu array).
- Overview 2.3: `ListPipelines`, `GetPipeline`, empat parser murni di `internal/gitdata/pipelines.go`.
- Overview 2.4: `RegisterModule("git-pipelines", ...)` di `cmd/shikigamid/wire_git_pipelines.go`.
- Overview 2.5: `registerGitSub("pipelines", gitPipelines)` dan `registerGitSub("pipeline", gitPipeline)`; flag `--workspace`, `--project`, `--id`.
- Overview 2.6: `pipelineClient.ts` (`listPipelines`, `getPipeline`), `PipelineList` props `{provider, repo}`, `PipelineRow` props `{pipeline, provider, onOpen}`, `PipelinePane` props `{pane, visible}` membaca `pane.pipelineId`, `StageList` props `{stages}`.
- Overview 5 (wiring checklist) dan 6 (build guard); selisih yang disengaja ditandai "DIPUTUSKAN" di overview 0.

## Langkah
Kerjakan berurutan. Bila satu langkah gagal karena kekurangan di file milik fitur lain, catat (file, baris, gejala, usulan perbaikan) lalu lanjutkan langkah lain yang masih bisa dijalankan.

1. Pra-syarat. Dari akar repo: `git status` dan `git log --oneline -8`. Pastikan file berikut ada dan bukan placeholder: `internal/gitdata/pipelines.go`, `cmd/shikigamid/wire_git_pipelines.go`, `cmd/shikigamictl/cmd_git_pipelines.go`, `frontend/src/modules/git/PipelineList.svelte`, `PipelineRow.svelte`, `PipelinePane.svelte`, `StageList.svelte`, `pipelineClient.ts`. Bila ada yang hilang atau masih placeholder, berhenti dan laporkan (jangan membuatnya).
2. Build Go. `go build ./...` dari akar repo harus lulus. Jalankan juga `go vet ./internal/gitdata/... ./cmd/...` (tidak memblokir bila hanya peringatan lama di luar epic ini).
3. Verifikasi registrasi daemon (checklist 5.1), hanya membaca.
   - `grep -rn 'RegisterModule("git' cmd/shikigamid` harus menampilkan `"git"`, `"git-mrs"`, dan `"git-pipelines"` masing-masing sekali; tidak ada nama ganda di seluruh `cmd/shikigamid`.
   - Baca `wire_git_pipelines.go`: `init()` memanggil `RegisterModule("git-pipelines", ...)`; `srv.Register` mendaftarkan `git.pipelines.list` dan `git.pipeline.get`; handler memvalidasi `workspaceId`/`projectId`/`id` (`id` `^[0-9]+$`, selain itu `invalid_params`), memakai `gitDecode`, `gitResolveRepo`, `mapGitError`, dan memastikan hasil list non-nil. Pastikan `wire_git.go` tidak berubah (`git diff --stat -- cmd/shikigamid/wire_git.go` kosong) dan tidak ada simbol bentrok (helper privat berawalan `gitPipelines`).
   - Baca `internal/gitdata/pipelines.go`: hanya memakai `Run(...)` (tanpa `os/exec`, tanpa `connector.RunCLITimeout`, tanpa impor `internal/workspace`); hanya flag panjang yang ada di allowlist (`--repo --output --per-page --pipeline-id`; `--limit --json`); memakai `pipelineRepoArg` (bukan `RepoFlag`/`mrRepoArg`); `NormalizeGitLabStatus`/`NormalizeGitHubStatus` dipakai, tidak didefinisikan ulang; tidak ada `--log`, `--log-failed`, `--with-variables`, `--web`.
4. Verifikasi registrasi ctl (checklist 5.2). Baca `cmd_git_pipelines.go`: `init()` memanggil `registerGitSub("pipelines", ...)` dan `registerGitSub("pipeline", ...)`; tidak mengimpor `gitdata`. Bangun ctl ke scratchpad (`go build -o <scratchpad>/shikigamictl ./cmd/shikigamictl`). Jalankan: `shikigamictl git` tanpa argumen (daftar subcommand harus mencantumkan `pipelines` dan `pipeline`, bila `cmd_git.go` mencetak daftar dinamis); `git pipelines` tanpa flag -> `penggunaan: shikigamictl git pipelines --workspace <id> --project <id>`; `git pipeline --workspace a --project b` tanpa `--id` -> `penggunaan: shikigamictl git pipeline --workspace <id> --project <id> --id <id>`. Kode keluar bukan nol, tanpa panic.
5. Verifikasi IPC terhadap daemon nyata. Jalankan daemon dengan cara yang dipakai repo (cek `Makefile`/`README`/`docs`); bila ada daemon dev yang sedang berjalan, pakai itu. Matikan HANYA PID yang Anda luncurkan sendiri; jangan `pkill` pola luas. Cari `workspaceId` dan `projectId` project dengan remote gitlab.com lewat `shikigamictl git repos --workspace <id>` (atau daftar workspace via ctl yang ada). Lalu:
   - `shikigamictl git pipelines --workspace <ws> --project <proj>`: mencetak maks 20 baris (id, ref, status, sha 7 karakter, durasi `-` untuk GitLab list, url) diakhiri `<n> baris`; respons < 20 detik; urut terbaru dulu.
   - Ambil satu id dari hasil, jalankan `shikigamictl git pipeline --workspace <ws> --project <proj> --id <id>`: header, lalu `[stage] <nama>` dan job (nama, status, durasi, url job); stage berurutan menurut aturan overview 0 (job id naik).
   - Galat: `--project` ngawur -> `[not_found] ...` terbaca; `--id abc` atau `--id 1;x` -> `invalid_params`; id digit tidak ada -> `[not_found] Pipeline tidak ditemukan`. Kode keluar bukan nol, tanpa panic, tanpa output mentah CLI.
6. Build frontend. `cd frontend && npm run build` harus lulus; jalankan juga `npm run check` dan laporkan galat yang berasal dari file epic ini (`PipelineList`, `PipelineRow`, `PipelinePane`, `StageList`, `pipelineClient`); peringatan lama di luar epic hanya dicatat.
7. Verifikasi titik sambung UI statis (checklist 5.3 dan 5.4), hanya membaca.
   - `ProjectSection.svelte` merender `<PipelineList {provider} {repo} />` di dalam `{#if open}`; `PipelineList.svelte` menerima persis `{provider: Provider; repo: Repo}` via `$props()`.
   - `PipelineList` memuat hanya saat tampil, memakai penghitung `seq` untuk respons usang, `$effect` bergantung pada `repo` (Segarkan panel memuat ulang), galat terisolasi dengan "Coba lagi" dan tautan `openSettings('connectors')` untuk `cli_missing`/`not_logged_in`, empty `Tidak ada pipeline`.
   - `PipelineRow` memanggil `onOpen`; `PipelineList` meneruskan ke `layout.openPane({ kind: 'git-pipeline', provider, projectId: repo.projectId, pipelineId: p.id, title })` (field bernama `pipelineId`, bukan `id`).
   - `modules/git/index.ts` mendaftarkan pane kind `git-pipeline`; `modules/layout/model.ts` memetakan `provider/projectId/pipelineId` di `Leaf`, `NewLeaf`, pembuat leaf, persist, dan `hydrateNode`.
   - `PipelinePane` membaca `pane.pipelineId` (bukan `pane.id`), membungkus `<div class="git-pane" hidden={!visible}>`, merender `StageList`, dan `PipelinePane`/`StageList` hanya mengimpor `getPipeline`/tipe dari `./pipelineClient` (`grep -rn "git.pipeline" frontend/src` hanya menemukan `pipelineClient.ts` sebagai pemanggil IPC).
   - Aksi baris job di `StageList` memakai URL job, bukan URL pipeline; nonaktif dengan `title` "Job ini tidak punya link" bila `url` kosong.
8. Verifikasi di app (checklist 5.6; data nyata `glab`, remote gitlab.com). Jalankan app memakai skill `run` atau cara yang dipakai repo. Bila GUI tidak bisa dijalankan di lingkungan ini, catat sebagai butir terbuka dengan alasan; jangan dianggap lulus. Periksa dan catat lulus/gagal:
   1. Rail GitLab muncul; bagian project terbuka menampilkan 20 pipeline terbaru (bukan placeholder); GitLab tanpa kolom penulis/durasi di daftar.
   2. Segarkan panel memuat ulang daftar; bagian terlipat tidak memicu panggilan (amati log daemon atau proses `glab` yang berjalan).
   3. Aksi baris Buka di browser, Salin link (tempel untuk periksa URL pipeline), Kirim ke agen (dengan sesi aktif URL tertempel tanpa Enter; tanpa sesi aktif toast "Tidak ada sesi aktif"). Klik tombol tidak membuka pane.
   4. Klik baris (dan Enter/Spasi saat fokus) membuka pane `git-pipeline` berjudul `Pipeline <id> (<ref>)` dengan header lengkap (status, ref, sha, durasi, penulis, Buka/Salin/Kirim, Segarkan) dan `StageList` berisi stage dan job nyata.
   5. Aksi pada header memakai URL pipeline; aksi pada baris job memakai URL job (tempel dan bandingkan); tanpa sesi aktif toast yang sama.
   6. Pipeline tanpa job (`jobs: null`) menampilkan "Tidak ada stage" tanpa crash; pipeline MR (`ref` = `refs/merge-requests/<iid>/head`) tampil apa adanya.
   7. Segarkan di pane mengambil ulang data.
   8. Restart app: pane `git-pipeline` dipulihkan dengan `provider/projectId/pipelineId` sama dan memuat ulang data. Simulasi repo hilang (mis. pane dengan `projectId` tak dikenal, atau workspace aktif tanpa repo itu) -> keadaan aman "Project atau pipeline tidak ditemukan", tombol aksi nonaktif, tidak crash.
   9. Galat aman: jalankan app dengan `PATH` tanpa `glab` (tanpa mengubah file repo) -> pesan terbaca + "Coba lagi" (+ tautan Pengaturan > Connector untuk `cli_missing`/`not_logged_in`) di daftar dan di pane; galat satu project tidak memengaruhi project lain.
   10. Beban: buka banyak bagian sekaligus tidak melebihi 3 proses `glab` bersamaan.
9. Jalur GitHub lewat fixture (hanya baca; checklist 5.7). Pastikan `internal/gitdata/testdata/gh-run-list.json` dan `gh-run-view.json` ada. Buktikan `parseGitHubRuns` dan `parseGitHubRunDetail` dengan skrip `go run` sementara di scratchpad yang diletakkan di luar repo; karena parser privat paket `gitdata`, bila tidak bisa diimpor dari luar, buktikan lewat penelusuran kode terhadap fixture (jangan menambah file di repo, jangan membuat file `_test.go`). Periksa: `databaseId` -> `ID`, `headBranch` -> `Ref`, `NormalizeGitHubStatus(status, conclusion)`, `updatedAt` fallback `createdAt`, durasi hanya bila bukan `pending`/`running`, `Author` kosong, satu `Stage` bernama `workflowName` (fallback `"Jobs"`), job `URL`. Tanpa login `gh`, buka panel GitHub di app (bila rail muncul) dan pastikan galat `not_logged_in` tampil aman. Catat butir terbuka: "verifikasi pipeline/Actions GitHub di app menunggu `gh auth login`" (tidak memblokir).
10. Catat selisih yang diketahui (checklist 5.8), jangan diperbaiki: `DurationSec` bertipe `int` tanpa `omitempty` sehingga 0 = tidak tersedia (tiket menginginkan "tidak ada"); `Leaf.pipelineId` vs `id` di tiket; `RepoFlag` self-hosted tidak andal (dihindari lewat `pipelineRepoArg`); `openPane` tidak men-dedup berdasarkan `pipelineId` sehingga id sama bisa membuka pane kedua (tiket meminta fokus "bila API layout mendukung"); hasil `git.pipelines.list` berupa array langsung berbeda dengan pembungkus objek `git.mrs.list`. Laporkan juga bila ada perintah allowlist atau field tipe yang kurang.
11. Pemeriksaan akhir: `git status` memastikan hanya perubahan milik empat fitur lain yang ada dan fitur ini tidak menambah file atau perubahan apa pun (`docs/plans/...` tidak dihitung). Pastikan daemon/app yang Anda luncurkan sudah dimatikan lewat PID sendiri.

## Perubahan
Tidak ada file dibuat atau diubah oleh fitur ini. Keluaran hanyalah laporan akhir: temuan, hasil tiap butir langkah 8, bukti fixture GitHub, selisih langkah 10, butir terbuka, dan usulan perbaikan untuk pemilik file bila ada kekurangan.

## Wiring
Fitur ini adalah langkah wiring itu sendiri. Tidak ada yang disambungkan lewat penyuntingan file: seluruh sambungan sudah mandiri (`RegisterModule("git-pipelines", ...)`, `registerGitSub("pipelines"|"pipeline", ...)`, titik ekstensi `<PipelineList {provider} {repo} />` di `ProjectSection`, pane kind `git-pipeline` di `index.ts`, field leaf di `layout/model.ts`). Bila salah satu ternyata membutuhkan baris di file yang bukan milik fitur ini, JANGAN mengubahnya; laporkan file, baris yang dibutuhkan, dan alasannya, serta tandai `ok:false` atau butir terbuka sesuai tingkat keparahan.

## Selesai bila
- `go build ./...` lulus; `cd frontend && npm run build` lulus (dan `npm run check` tidak menambah galat dari file epic ini).
- `git.pipelines.list` dan `git.pipeline.get` terjawab daemon yang berjalan; `shikigamictl git pipelines` dan `git pipeline` mencetak hasil; flag wajib hilang dan id tidak valid ditolak dengan pesan terbaca.
- Daftar pipeline muncul di panel GitLab dengan data nyata `glab`; klik baris membuka pane `git-pipeline` dengan stage dan job nyata; aksi Buka/Salin/Kirim ke agen terverifikasi untuk baris pipeline, header pane, dan baris job (URL job; toast tanpa sesi); Segarkan, bagian terlipat tanpa panggilan, dan galat aman terverifikasi; pane dipulihkan setelah restart, repo hilang -> keadaan galat aman.
- GitHub dibuktikan lewat fixture dan tidak crash tanpa login; butir terbuka "verifikasi pipeline/Actions GitHub di app menunggu `gh auth login`" dicatat di laporan akhir.
- Tidak ada file di luar ownedPaths (kosong) yang diubah oleh fitur ini. Guard: build Go dan build frontend sukses; tanpa unit test.
