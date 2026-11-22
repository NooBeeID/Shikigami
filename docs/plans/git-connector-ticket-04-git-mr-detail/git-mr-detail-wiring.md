# Plan: Wiring dan verifikasi akhir Git connector

## Metadata

- Tiket: `git-connector-ticket-04-git-mr-detail` (Detail MR/PR; tiket penutup connector Git)
- Fitur: `git-mr-detail-wiring`
- ownedPaths: tidak ada (fitur ini tidak memiliki file sendiri dan tidak membuat file di repo)
- Depends on foundation: tidak (`hasFoundation=false`)
- Prasyarat urutan: dikerjakan SETELAH `git-mr-detail-data`, `git-mr-detail-ctl`, dan `git-mr-pane-ui` selesai dan ter-merge ke worktree ini. Bila salah satu belum ada, hentikan dan laporkan; jangan menulis penggantinya.

Rujukan kontrak: `docs/plans/git-connector-ticket-04-git-mr-detail/overview.md` (terutama bagian 2, 5, 6, 7), tiket `docs/tickets/git-connector-ticket-04-git-mr-detail.md` (bagian Wiring), dan spec `docs/superpowers/specs/2026-11-22-10-git-connector-design.md`. Dokumen itu tidak boleh diubah.

## Tujuan

Menutup epic dan seluruh connector Git: memastikan tiga fitur paralel tersambung lewat titik registrasi mandiri (tanpa edit file wiring tiket sebelumnya), build Go dan frontend lulus, aplikasi berjalan, dan SELURUH spec Git connector terbukti dengan `glab` nyata (`gitlab.com`). Jalur GitHub dibuktikan lewat fixture dan dicatat sebagai butir terbuka di laporan penutup. Kekurangan di file milik fitur/tiket lain TIDAK diperbaiki di sini; dilaporkan (atau dikembalikan ke fitur pemilik path itu).

## Path yang dimiliki

Tidak ada. HANYA ini yang boleh disentuh: nol file repo. Aturan turunannya:

- Dilarang mengedit `cmd/shikigamid/wire_git.go`, `cmd/shikigamictl/cmd_git.go`, dan semua file tiket 1 sampai 3 serta file fitur paralel (`internal/gitdata/*.go`, `wire_git*.go`, `cmd_git*.go`, `frontend/src/modules/git/*`, `layout/model.ts`, dsb.). Membaca, `grep`, dan menjalankan diperbolehkan.
- Dilarang mengubah `overview.md`, tiket, spec, `docs/git/cli-findings.md`.
- Skrip sementara (mis. fixture GitHub via `go run`) hanya di scratchpad, bukan di repo. Tanpa unit test, tanpa dependensi baru, tanpa commit.
- Temuan kekurangan: catat per item (file, gejala, fitur pemilik) di laporan akhir; bila perbaikan wajib agar acceptance lulus, minta perbaikan lewat fitur pemilik path itu (bukan dikerjakan di plan ini).

## Kontrak

Yang diverifikasi (disediakan fitur lain, lihat overview bagian 2):

- IPC `git.mr.get` `{workspaceId, projectId, number}` -> objek `MergeRequestDetail` (camelCase; `labels`/`reviewers` selalu array; `mergeable?`, `lastPipeline?` hilang bila kosong), didaftarkan lewat `RegisterModule("git-mr-detail", ...)` di `cmd/shikigamid/wire_git_mr_detail.go`. Nama modul terpakai lain: `git`, `git-mrs`, `git-pipelines`.
- Go: `gitdata.GetMergeRequest(ctx, repo Repo, number int) (MergeRequestDetail, error)`, parser murni `parseGitLabMrDetail` dan `parseGitHubPrDetail` di `internal/gitdata/mrdetail.go`.
- ctl: `registerGitSub("mr", gitMrDetail)` dari `init()` di `cmd/shikigamictl/cmd_git_mr_detail.go`; bentuk `shikigamictl git mr <workspaceId> <projectId> <number>`.
- Frontend: `MrPane.svelte` (props `{pane: Leaf; visible: boolean}`, kind `git-mr` di `index.ts`), `ReviewerList.svelte`, `mrDetailClient.ts` (`getMrDetail(workspaceId, projectId, number)`).
- Pembukaan pane: `MrList.openMr` -> `layout.openPane({kind:'git-mr', provider, projectId, number, title})`; `MrPane` -> `layout.openPane({kind:'git-pipeline', provider, projectId, pipelineId, title})`; `PipelinePane` membaca `pane.pipelineId`.

Yang dipakai: build Go (`go build ./...`), build frontend (`cd frontend && npm run build`, opsional `npm run check`), Makefile (`make build-daemon`, `make build-ui`, `make build-app`, `make run-daemon`), `glab` nyata yang sudah login ke `gitlab.com`.

## Langkah

Kerjakan berurutan. Catat hasil tiap langkah (lulus/gagal + bukti singkat) untuk laporan akhir. Semua perintah dijalankan dari akar repo kecuali disebut lain.

1. Pemeriksaan prasyarat. Pastikan file hasil tiga fitur ada: `ls internal/gitdata/mrdetail.go cmd/shikigamid/wire_git_mr_detail.go cmd/shikigamictl/cmd_git_mr_detail.go frontend/src/modules/git/MrPane.svelte frontend/src/modules/git/ReviewerList.svelte frontend/src/modules/git/mrDetailClient.ts`. Pastikan `MrPane.svelte` bukan placeholder lagi (memuat `getMrDetail`). Bila hilang, hentikan dan laporkan fitur mana yang belum selesai.
2. Verifikasi wiring daemon (butir 1 overview 5):
   - `grep -rn 'RegisterModule("git' cmd/shikigamid` harus menampilkan empat nama unik: `git`, `git-mrs`, `git-pipelines`, `git-mr-detail`.
   - `grep -n 'git.mr.get' cmd/shikigamid/wire_git_mr_detail.go` menunjukkan `srv.Register("git.mr.get", ...)`; `grep -rn '"git.mr.get"' cmd/shikigamid` tidak muncul di file lain.
   - `git status --short cmd/shikigamid/wire_git.go cmd/shikigamictl/cmd_git.go` kosong, dan `git diff --stat HEAD -- cmd/shikigamid/wire_git.go cmd/shikigamictl/cmd_git.go` kosong (tidak diedit).
   - Baca handler: validasi `workspaceId`/`projectId` kosong dan `number <= 0` -> `invalid_params`; `Labels`/`Reviewers` non-nil.
3. Verifikasi wiring ctl (butir 2): `grep -n 'registerGitSub("mr"' cmd/shikigamictl/*.go`. Setelah build (langkah 7) jalankan `./build/bin/shikigamictl git` (tanpa argumen: daftar subcommand mencantumkan `mr`), `./build/bin/shikigamictl git mr` dan `./build/bin/shikigamictl git mr ws proj abc` (keduanya mencetak `penggunaan: shikigamictl git mr <workspaceId> <projectId> <number>`, exit non-nol).
4. Verifikasi wiring frontend (butir 3 dan 4):
   - `grep -n "git-mr\|git-pipeline\|MrPane\|PipelinePane" frontend/src/modules/git/index.ts` menunjukkan `git-mr` -> `MrPane` dan `git-pipeline` -> `PipelinePane` tetap terdaftar; props `MrPane` masih `{ pane, visible }` dan wrapper `<div class="git-pane" hidden={!visible}>`.
   - `sed -n '55,80p' frontend/src/modules/git/MrList.svelte` (atau baca `openMr`): membuka `kind: 'git-mr'` dengan `provider`, `projectId`, `number`, `title`.
   - Di `MrPane.svelte`: klik pipeline terakhir memanggil `layout.openPane({ kind: 'git-pipeline', provider: pane.provider, projectId: pane.projectId, pipelineId: <lastPipeline.id>, title })`; `grep -n "pipelineId" frontend/src/modules/git/PipelinePane.svelte` mengonfirmasi pembacaan field yang sama.
   - `grep -n "import" frontend/src/modules/git/MrPane.svelte`: hanya mengimpor `getMrDetail`/tipe dari `./mrDetailClient` (tipe dasar `MergeRequest` dari `./mrClient`), tanpa `{@html}` (`grep -n "@html" frontend/src/modules/git/MrPane.svelte` kosong).
   - Setiap celah (mis. field tak cocok) dicatat sebagai temuan untuk pemilik path; jangan diubah di sini.
5. Verifikasi read-only dan tanpa token:
   - `grep -nE 'create|merge|approve|close|note|retry|cancel|delete|update' internal/gitdata/allowlist.go`: tidak ada subcommand tulis (kata `--merged`/`--closed` sebagai flag filter daftar, `update`/`merge` sebagai subcommand tidak boleh ada; periksa tiap hit manual).
   - Pastikan entri allowlist untuk detail hanya `glab mr view` dan `gh pr view` tanpa `-c/--comments`, `--web`, `api`.
   - `grep -rniE 'token|PRIVATE-TOKEN|GITLAB_TOKEN|GH_TOKEN' internal/gitdata cmd/shikigamid/wire_git*.go cmd/shikigamictl/cmd_git*.go frontend/src/modules/git internal/workspace` tidak menunjukkan penyimpanan token; pastikan `WorkspaceConfig` tidak punya field kredensial git.
   - `grep -rn 'os/exec' internal/gitdata` kosong (semua lewat `Run`).
6. Siapkan kasus nyata. Pilih workspace/project dengan remote `gitlab.com` yang punya MR (Terbuka dan Digabung) dan pipeline. Ambil `workspaceId` dan `projectId` lewat `./build/bin/shikigamictl git repos` (sesuai bentuk di `cmd_git.go`) dengan daemon berjalan. Pastikan `glab auth status` menunjukkan login `gitlab.com`.
7. Build:
   - `go build ./...` harus lulus (tanpa output galat).
   - `cd frontend && npm run build` harus lulus; jalankan juga `npm run check` bila tersedia dan catat bila ada galat baru di file `modules/git` (galat lama di luar epic hanya dicatat).
   - `make build-daemon` (menghasilkan `build/bin/shikigamid` dan `build/bin/shikigamictl`).
8. Jalankan aplikasi. Matikan daemon lama HANYA bila PID-nya diluncurkan sendiri oleh sesi ini (jangan `pkill` pola luas). Mulai daemon (`make run-daemon` di latar belakang, simpan PID) dan aplikasi desktop lewat `make build` lalu `./build/bin/shikigami`, atau mode dev yang berlaku di README/Makefile. Bila tidak ada layar/GUI, verifikasi yang butuh UI ditandai "tidak dapat diverifikasi di lingkungan ini" dan jalur backend tetap dibuktikan lewat ctl; jangan mengklaim UI lulus tanpa melihatnya.
9. Verifikasi backend dengan `glab` nyata lewat ctl: `./build/bin/shikigamictl git mr <ws> <proj> <number>` untuk (a) MR terbuka, (b) MR digabung, (c) nomor tak ada (galat `not_found` Indonesia, exit non-nol), (d) `projectId` tak dikenal (galat terbaca). Periksa keluaran: judul, status, penulis, branch, label, reviewer (state `pending` untuk GitLab), `mergeable`, pipeline `<id> <status> <url>`, deskripsi apa adanya. Opsional: `git.mr.get` mentah memastikan `labels: []`/`reviewers: []` tidak `null`.
10. Verifikasi spec lengkap di aplikasi (centang satu per satu, catat bukti):
    - Rail: item GitLab tampil bila ada project remote GitLab; item GitHub TIDAK tampil tanpa remote GitHub; isi berganti saat workspace berganti.
    - Panel: satu bagian per project (digabung per host+path), bisa dilipat, daftar MR dengan pemilih Terbuka/Digabung/Tertutup dan daftar Pipeline termuat.
    - Klik baris MR membuka pane `git-mr` lengkap (header, badge status, penulis, branch, label, reviewer, mergeable, pipeline terakhir, deskripsi); klik pipeline terakhir membuka pane `git-pipeline` yang benar; klik baris pipeline di panel membuka pane dengan stage dan job.
    - Aksi Buka/Salin/Kirim ke agen bekerja di baris dan header pane; tanpa sesi aktif muncul toast "Tidak ada sesi aktif".
    - Segarkan di panel dan di pane mengambil ulang; keadaan memuat, kosong ("Tidak ada MR terbuka"), dan galat terbaca. Simulasikan galat tanpa mengubah repo: CLI hilang (jalankan daemon dengan PATH tanpa `glab`), belum login (`glab auth logout` hanya bila pengguna mengizinkan; bila tidak, tandai tidak diuji), repo/MR tidak ditemukan (nomor tak ada), izin kurang dan timeout (catat bila tidak dapat disimulasikan). Ajakan ke Pengaturan > Connector muncul untuk `cli_missing`/`not_logged_in`. Tidak ada polling (jaringan/CLI tidak dipanggil berulang saat diam; periksa lewat log daemon).
    - Deskripsi MR berisi HTML/markdown mentah tampil sebagai teks biasa (tidak dieksekusi).
    - Pemulihan layout: dengan pane `git-mr` dan `git-pipeline` terbuka, tutup lalu buka ulang aplikasi; pane terpulihkan dan memuat ulang datanya tanpa galat.
11. Verifikasi jalur GitHub lewat fixture (tanpa `gh` login): tulis skrip `go run` sementara di scratchpad `/private/tmp/claude-501/-Users-ascendiz-Documents-Projects-noobeeid-research-ai-terminal/728d9570-9382-4bc0-a6cd-e86eb4eda959/scratchpad/` yang dikompilasi sebagai test-satu-kali di dalam paket `gitdata` TANPA menambah file di repo (mis. salin `internal/gitdata` ke direktori sementara di scratchpad dan beri modul replace, atau jalankan `go test` terhadap file `_test.go` sementara di scratchpad lewat overlay `-overlay`), memanggil `parseGitHubPrDetail` dengan `internal/gitdata/testdata/gh-pr-view.json` dan fixture detail baru (bila fitur data menambahkannya). Periksa: Description, Labels, Reviewers (gabungan tanpa duplikat), Mergeable, LastPipeline (id run), tanpa panic pada field hilang/`null`. Pastikan tidak ada file sementara tersisa di repo (`git status --short` hanya menampilkan perubahan milik fitur lain). Pastikan menjalankan jalur GitHub tanpa login tidak crash (galat `not_logged_in`/`cli_missing` terbaca).
12. Pembersihan: hentikan daemon/aplikasi yang diluncurkan sendiri (hanya PID sendiri), hapus artefak sementara di scratchpad bila perlu. Verifikasi `git status --short` bersih dari file yang tidak diharapkan (artefak `build/`, `frontend/dist`, `app/dist` mengikuti `.gitignore`; jangan ditambahkan).
13. Susun laporan penutup (jawaban akhir agen, bukan file di repo), berisi: hasil tiap langkah 2 sampai 11; daftar centang spec; butir terbuka WAJIB: "verifikasi jalur GitHub (`gh`) di aplikasi menunggu `gh auth login`; dibuktikan hanya lewat fixture (asumsi bentuk `gh pr view --json`); tidak memblokir merge"; selisih yang diputuskan overview untuk dilaporkan (`Leaf.pipelineId` vs `id` di tiket; `GetMergeRequest` menerima `Repo`; `Mergeable` dinormalisasi tiga nilai; `Reviewer.State` GitLab `pending`; `RepoFlag` self-hosted dihindari lewat `mrRepoArg`); serta temuan kekurangan per file beserta fitur pemilik.

## Perubahan

Tidak ada file dibuat atau diubah di repo oleh fitur ini. Satu-satunya keluaran: laporan verifikasi (langkah 13) dan artefak sementara di scratchpad. Bila ditemukan cacat di file fitur lain, perubahan dilakukan oleh fitur pemilik path (bukan fitur ini) dan dicatat di laporan.

## Wiring

Fitur ini adalah langkah wiring; ia tidak mengekspos apa pun baru. Yang diperiksa tersambung (semua mandiri, tanpa edit wiring lama):

- Daemon: `RegisterModule("git-mr-detail", ...)` -> `git.mr.get`.
- ctl: `registerGitSub("mr", ...)` -> `shikigamictl git mr`.
- Frontend: kind `git-mr` -> `MrPane` (sudah di `index.ts`), `MrList.openMr` -> `git-mr`, `MrPane` -> `git-pipeline` (`pipelineId`).

## Selesai bila

- Langkah 2 sampai 5 lulus: semua registrasi mandiri ada dan unik, `wire_git.go` serta `cmd_git.go` tidak berubah, `MrPane` tetap kind `git-mr` dengan props sama, tidak ada perintah tulis di allowlist, tidak ada token di config, tidak ada `{@html}`.
- `go build ./...` lulus dan `cd frontend && npm run build` lulus (guard; tanpa unit test).
- Aplikasi dijalankan dan `shikigamictl git mr` mencetak detail MR `gitlab.com` nyata; pane `git-mr` tampil dengan data nyata.
- Seluruh daftar centang langkah 10 terisi (lulus, atau ditandai tidak dapat diuji dengan alasan jelas); jalur GitHub dibuktikan lewat fixture tanpa panic.
- Tidak ada file repo yang diubah oleh fitur ini; tidak ada file sementara tersisa di repo.
- Laporan penutup memuat butir terbuka GitHub, daftar selisih, dan temuan kekurangan per fitur pemilik.
