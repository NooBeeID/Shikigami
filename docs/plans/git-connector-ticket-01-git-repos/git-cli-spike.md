# Plan: Spike CLI glab dan gh

## Metadata
- Tiket: `git-connector-ticket-01-git-repos` (`docs/tickets/git-connector-ticket-01-git-repos.md`)
- Fitur: `git-cli-spike`
- ownedPaths: `docs/git/cli-findings.md`, `internal/gitdata/testdata/`
- Depends on foundation: tidak (epic tidak punya foundation; fitur ini tidak bergantung pada fitur lain dan sebaiknya selesai PERTAMA)
- Kontrak bersama: lihat `docs/plans/git-connector-ticket-01-git-repos/overview.md` (jangan diubah). Fitur ini hanya menghasilkan dokumen dan fixture; tanpa perubahan Go atau frontend.

## Tujuan
Menjalankan `glab` nyata (login `gitlab.com`, hanya perintah baca), merekam dan menganonimkan keluarannya menjadi fixture, membuat fixture GitHub dari dokumentasi `gh` (ditandai ASUMSI karena `gh` belum login), dan menulis `docs/git/cli-findings.md` sebagai acuan parser, allowlist, klasifikasi galat, dan adapter fitur lain serta tiket 02 sampai 04. Dokumen harus menjawab tegas empat pertanyaan spike di spec (badge pipeline/review di daftar MR, perintah pipeline GitLab dan GitHub, cara membaca host login, bentuk `gh` tanpa login) dan memuat keputusan potong/aktif fitur.

## Path yang dimiliki
HANYA path ini yang boleh dibuat atau diubah:
- `docs/git/cli-findings.md` (folder `docs/git/` belum ada; buat)
- `internal/gitdata/testdata/` (folder belum ada; buat; isi hanya berkas fixture dan `README.md`)

Jangan sentuh: file Go apa pun (termasuk `internal/gitdata/*.go`, `internal/connector/*`), frontend, `cmd/*`, `overview.md`, tiket, spec, file wiring. Jangan membuat `*_test.go`. Jangan commit.
Skrip bantu dan keluaran mentah (belum teranonimkan) disimpan HANYA di scratchpad sesi (di luar repo), bukan di path repo, dan tidak boleh tersalin ke repo.

## Kontrak
Disediakan (dikonsumsi fitur lain; nama berkas persis):
- Fixture di `internal/gitdata/testdata/`: `glab-mr-list.json`, `glab-mr-view.json`, `glab-pipeline-list.json`, `glab-pipeline-get.json`, `glab-auth-status.txt`, `gh-pr-list.json`, `gh-pr-view.json`, `gh-run-list.json`, `gh-run-view.json`, `gh-auth-status.txt`, `README.md`. Tambahan boleh bila temuan memerlukan (mis. `glab-ci-jobs.json`, `glab-auth-status-loggedout.txt`, `gh-auth-status-loggedout.txt`, `git-remote-urls.txt`), semuanya terdaftar di `README.md`.
- `docs/git/cli-findings.md` (Indonesia) yang menetapkan nilai final untuk: nama subcommand dan flag allowlist (`gitdata-foundation`: `allowlist.go`), `NormalizeGitLabStatus`/`NormalizeGitHubStatus` (`types.go`), pola stderr untuk `classify` (`errors.go`), format parser daftar host login (parser privat adapter `gitlab.go`/`github.go` dan `repo.go`), bentuk `Status()` adapter (exit code saat belum login), bentuk `-R`, serta bentuk field opsional tipe (`ReviewState`, `PipelineStatus`, `Mergeable`, `LastPipeline`).
- Tipe target ternormalisasi yang jadi tujuan pemetaan field ada di overview bagian 2.3 (`MergeRequest`, `MergeRequestDetail`, `Pipeline`, `PipelineDetail`, `Stage`, `Job`, `Status`); dokumen temuan memetakan path field CLI ke field tipe itu, tanpa mengubah nama field tipe. Perubahan bentuk field opsional dicatat di temuan saja.

Dipakai: `glab` (terpasang di `/opt/homebrew/bin/glab`, versi terlihat 1.97.0 dan login ke `gitlab.com`), `gh` (terpasang, BELUM login), `git`, `jq`. Pola dokumen acuan: `docs/lark/cli-findings.md` (struktur bagian: versi dan lingkungan, tabel perintah, skema ringkas, pemetaan, galat, rekomendasi allowlist). Fixture pola: `internal/larkdata/testdata/` (hanya dibaca sebagai acuan gaya).

## Langkah

### 1. Persiapan dan keamanan
1. Buat folder kerja di scratchpad (mis. `<scratchpad>/git-spike/raw/`), dan folder repo `internal/gitdata/testdata/` serta `docs/git/`.
2. Catat versi: `glab --version`, `gh --version`, `git --version`, `jq --version`, tanggal. Cek login: `glab auth status` (stdout+stderr, exit code via `echo $?`), `gh auth status` (stdout+stderr, exit code).
3. Aturan keras: HANYA perintah baca. Dilarang: `glab mr create|merge|approve|close|note|update`, `glab ci run|retry|cancel|trigger`, `glab api` dengan method selain GET, `gh pr create|merge|review|close|comment`, `gh run rerun|cancel`, `gh auth login|logout|refresh|token`, `glab auth login|logout`. Dilarang memakai `--show-token`/`-t`. Jangan mengubah direktori kerja repo user (gunakan `-R`). Jangan menjalankan `glab check-update`/`gh extension` atau pembaruan.
4. Pilih satu repo `gitlab.com` yang bisa diakses akun `glab` sebagai target spike, lebih disukai yang punya MR (berbagai state) dan pipeline dengan beberapa stage dan job (cari lewat `glab repo list`/`glab mr list -R <path>`; boleh memakai repo yang remote-nya ada di workspace pengembang, dibaca lewat `git -C <path> remote get-url origin`). Nilai asli (user, grup, repo, judul, SHA, URL) hanya ada di keluaran mentah scratchpad.

### 2. Pertanyaan 1: daftar MR (glab) dan kolom opsional
1. Jalankan, simpan mentah di scratchpad: `glab mr list -R <path> -F json` (state default terbuka), lalu variasi: `--merged`, `--closed`, `--all`, `--per-page N`/`-P N` (konfirmasi nama flag batas jumlah dan filter state lewat `glab mr list --help`). Catat flag yang valid persis.
2. Periksa dengan `jq`: apakah tiap elemen memuat objek pipeline terakhir (`head_pipeline`, `pipeline`) beserta `status`/`web_url`/`id`; apakah ada info review/approval (`reviewers`, `approved_by`, `approvals_before_merge`, `detailed_merge_status`, `user_notes_count`, dst.). Catat daftar field top-level yang ada untuk satu elemen (mis. `jq '.[0]|keys'`) dan tipe tiap field yang akan dipakai (iid, title, author.username/name, source_branch, target_branch, state, draft/work_in_progress, updated_at, web_url, labels, reviewers, merge_status/detailed_merge_status, has_conflicts).
3. Putusan tegas untuk dokumen: badge pipeline di baris MR AKTIF atau DIHILANGKAN dari baris; status review di baris MR AKTIF atau DIHILANGKAN dari baris (hanya detail). Kriteria: aktif bila field tersedia konsisten di daftar tanpa panggilan per baris; bila field hanya kadang ada atau butuh panggilan tambahan, hilangkan dan tetapkan sumber di detail. Catat pemetaan `reviewState` (mis. disetujui / diminta perubahan / belum) dan sumbernya bila aktif.

### 3. Detail MR (glab)
Jalankan `glab mr view <iid> -R <path> -F json` untuk satu MR (cari flag yang menghasilkan JSON; bila tidak mendukung `-F json`, gunakan bentuk yang didukung dan catat, mis. `--output json`). Catat path field: description, labels, reviewers (+ state), merge_status/detailed_merge_status/has_conflicts (sumber `Mergeable`), head_pipeline/pipeline (sumber `LastPipeline`: id, status, web_url), author, web_url. Bila field penting hanya ada lewat `glab api`, catat bahwa itu cadangan dan JANGAN tambahkan `api` ke rekomendasi allowlist kecuali tak ada jalan lain (bila terpaksa, nyatakan eksplisit sebagai risiko dan batasi pada GET endpoint tertentu).

### 4. Pertanyaan 2: pipeline GitLab dan GitHub
1. Daftar pipeline GitLab: coba `glab ci list -R <path> -F json` dan `glab pipeline list` bila ada (lihat `glab ci --help`, `glab pipeline --help`); catat subcommand final yang bekerja, flag jumlah (`--per-page`/`-P`), field: id, iid, status, ref, sha, source, created_at/updated_at, web_url, user/author (bila ada), duration (bila ada).
2. Detail pipeline GitLab (stage dan job): coba `glab ci get -R <path> -p <id> -F json` (mungkin juga `--with-job-details`), `glab ci view` (kemungkinan interaktif; JANGAN dipakai bila meminta TTY, catat saja), dan `glab ci list`/`glab ci get` JSON. Tentukan apakah stage dan job tersedia lewat subcommand non-interaktif. Bila job tidak ada dalam keluaran, cek jalur cadangan baca-saja (`glab api projects/:id/pipelines/<id>/jobs` hanya GET) dan catat sebagai pengecualian bila dipakai. Catat path field job: name, stage, status, duration, web_url, started_at/finished_at.
3. GitHub: fixture dari dokumentasi (`gh pr list --json`, `gh pr view --json`, `gh run list --json`, `gh run view --json jobs`; field resmi dari `gh pr list --help`, `gh run list --help`, `gh run view --help`, yang bisa dibaca tanpa login). Catat daftar field JSON tersedia yang diminta untuk tiap perintah (mis. PR: number,title,author,headRefName,baseRefName,state,isDraft,reviewDecision,statusCheckRollup,updatedAt,url,labels,reviewRequests,body,mergeable; run: databaseId,displayTitle,headBranch,headSha,status,conclusion,event,workflowName,createdAt,updatedAt,startedAt,url; run view: jobs{name,status,conclusion,startedAt,completedAt,url,steps}).
4. Jangan menjalankan `gh` terhadap server untuk pengambilan data karena belum login; keluaran tanpa login (`gh auth status`, `gh pr list -R x/y` tanpa login) boleh dijalankan dan direkam sebagai galat nyata.

### 5. Pertanyaan 3: host login dan galat
1. `glab auth status` dan `glab auth status --hostname gitlab.com`: rekam stdout, stderr (glab sering menulis ke stderr), exit code. Tentukan: bentuk teks (baris host, "Logged in to <host> as <user>", token masked), apakah ada opsi JSON (`-F json`/`--json`), cara menguraikan daftar host dan user. JANGAN pakai `--show-token`; pastikan fixture tidak memuat token (walau termask, ganti dengan `****`).
2. Rekam keadaan belum login: bila aman, simulasikan tanpa logout dengan environment kosong: `GITLAB_CONFIG_DIR=<scratchpad>/empty-glab-config glab auth status` (dan `HOME` sementara bila perlu) untuk mendapat teks belum login dan exit code. Jangan memanggil `glab auth logout`.
3. `gh auth status` (belum login): rekam stdout, stderr, exit code NYATA (teks "You are not logged into any GitHub hosts. To log in, run: gh auth login" diharapkan). Bentuk keadaan login (untuk `gh-auth-status.txt`) dibuat dari dokumentasi dan ditandai ASUMSI. Konfirmasi flag `--hostname` ada via `gh auth status --help`.
4. Pola galat stderr: jalankan perintah baca nyata yang sengaja salah, semuanya read-only: repo tidak ada (`glab mr list -R <grup>/tidak-ada-xyz`) untuk `not_found`; repo/project tanpa izin (bila ada contoh aman, mis. repo privat orang lain yang tidak bisa diakses; bila tidak ada, catat sebagai tidak teramati); host tak terjangkau/tak dikenal (`glab mr list -R tidak-ada.example.invalid/a/b`) untuk jaringan; CLI tidak ada ditangani `RunCLI` (`ErrCLIMissing`, bukan stderr). Untuk galat belum login gunakan env kosong dari langkah 5.2. Catat substring stderr yang stabil untuk tiap kelas (not_logged_in, permission, not_found, timeout/jaringan) dan exit code.
5. Catat apakah `glab`/`gh auth status` exit non-nol saat belum login dan apakah teks status ada di stdout atau stderr (adapter dan `repo.go` memakai teks pada galat `perintah <name> gagal: <stderr terpotong 300>`; catat bahwa teks lebih dari 300 karakter terpotong dan apa konsekuensinya untuk host kedua dan seterusnya, serta apakah `RunCLI` hanya mengembalikan stdout saat sukses).

### 6. Remote git dan bentuk `-R`
1. Jalankan `git -C <path> remote`, `git -C <path> remote get-url origin` pada project nyata berbentuk ssh dan https bila ada; bila hanya satu bentuk, bentuk lain didokumentasikan dari spesifikasi git. Catat keluaran untuk: `git@host:owner/repo.git`, `ssh://git@host:port/owner/repo.git`, `https://user@host/owner/repo.git`, `https://host/owner/repo` dan subgroup (`group/sub/repo`). Ini cukup sebagai tabel contoh teranonimkan di dokumen (tidak perlu fixture file kecuali berguna).
2. Uji bentuk `-R` GitLab: `OWNER/REPO` untuk gitlab.com; subgroup `GROUP/SUBGROUP/REPO` (bila ada repo subgroup yang bisa diakses; bila tidak ada, catat sebagai tidak teramati dan perlakukan sebagai asumsi); self-hosted `HOST/PATH` (dari `glab mr list --help`/dokumentasi `-R`, diverifikasi nyata bila ada host self-hosted login; jika tidak, ASUMSI). Untuk `gh`: `OWNER/REPO` dan `HOST/OWNER/REPO` (dari help; ASUMSI untuk GHE).

### 7. Anonimkan dan simpan fixture
1. Tulis skrip `jq` (atau skrip kecil) di scratchpad yang MENGGANTI nilai asli secara konsisten tanpa mengubah struktur/tipe: username, nama, email, avatar_url, id numerik (project id, user id, pipeline id, job id: ganti dengan angka palsu konsisten berpola, mis. 1001, 2001, 3001), iid/number (boleh dipertahankan atau diubah konsisten), grup/repo (mis. `acme-group/acme-app`, subgroup `acme-group/platform/acme-app` untuk satu fixture bila perlu menunjukkan subgroup), judul dan deskripsi (teks palsu mis. "Perbaiki validasi formulir", deskripsi singkat palsu), nama branch (`feature/contoh-...`; `main` boleh), label (palsu), SHA (40 hex palsu konsisten, dan `short_id`), seluruh URL (`https://gitlab.com/acme-group/acme-app/-/merge_requests/N` dst., host `gitlab.com` boleh dipertahankan), timestamp (boleh dipertahankan atau digeser konsisten; harus tetap RFC3339), dan hapus field yang berpotensi sensitif (token, email, avatar, IP, runner info, trace). Jaga urutan, jumlah elemen (2 sampai 4 elemen per daftar, mencakup beragam state/status agar pemetaan status dapat dilihat: untuk pipeline minimal success, failed, running, canceled/skipped/manual; untuk MR minimal satu draft dan satu dengan pipeline/dengan review bila field ada), dan nilai null/kosong yang nyata (mis. pipeline null).
2. Simpan hasil ke repo (nama persis):
   - `internal/gitdata/testdata/glab-mr-list.json`
   - `internal/gitdata/testdata/glab-mr-view.json`
   - `internal/gitdata/testdata/glab-pipeline-list.json`
   - `internal/gitdata/testdata/glab-pipeline-get.json` (bentuk final mengikuti subcommand terpilih di langkah 4; bila job butuh sumber kedua, simpan sebagai berkas tambahan mis. `glab-ci-jobs.json` dan jelaskan)
   - `internal/gitdata/testdata/glab-auth-status.txt` (teks stdout+stderr nyata, teranonimkan; sertakan keadaan belum login sebagai `glab-auth-status-loggedout.txt`)
   - `internal/gitdata/testdata/gh-pr-list.json`, `gh-pr-view.json`, `gh-run-list.json`, `gh-run-view.json`, `gh-auth-status.txt` (ASUMSI, dibuat dari dokumentasi/help `gh` dengan data palsu; strukturnya mengikuti field resmi, `gh-auth-status.txt` bentuk login ASUMSI; tambah `gh-auth-status-loggedout.txt` berisi keluaran NYATA belum login)
   File JSON harus JSON murni (tanpa komentar); penanda ASUMSI untuk fixture GitHub ada di `README.md` dan di dokumen temuan (bukan di dalam JSON). File `.txt` fixture asumsi boleh diberi baris pertama `# ASUMSI: ...` HANYA bila README menyebut itu; lebih baik tanpa baris tambahan agar parser tes menerimanya persis, jadi gunakan README sebagai penanda utama.
3. Pastikan keluaran JSON identik dengan bentuk asli (key sama, tipe sama). Format tulis dengan `jq .` (indent 2 spasi, newline akhir).

### 8. Verifikasi anonimisasi dan validitas
1. `for f in internal/gitdata/testdata/*.json; do jq . "$f" >/dev/null || echo "BAD $f"; done` harus tanpa "BAD".
2. Susun daftar nilai asli yang diketahui di scratchpad (username akun glab, nama lengkap, email, nama grup/repo target, host self-hosted bila ada, sebagian judul MR, SHA asli, awalan token seperti `glpat-`, `gho_`, `ghp_`). Jalankan `grep -rniF -f <daftar> internal/gitdata/testdata docs/git/cli-findings.md` (daftar di scratchpad, TIDAK disalin ke repo). Hasil harus kosong. Tambah pemeriksaan pola umum: `grep -rniE 'glpat-|ghp_|gho_|github_pat_|@[a-z0-9.-]+\.[a-z]{2,}|BEGIN (RSA|OPENSSH)|Bearer ' internal/gitdata/testdata docs/git` harus kosong (alamat email palsu `@example.com`-pun jangan dipakai; hindari pola email sama sekali).
3. Hapus tidak perlu: keluaran mentah tetap di scratchpad saja; pastikan `git status` hanya menampilkan `docs/git/` dan `internal/gitdata/testdata/` baru (selain item yang sudah ada sebelumnya).

### 9. Tulis `internal/gitdata/testdata/README.md`
Isi (Indonesia, ringkas): tujuan folder; tabel `berkas | perintah penghasil | asal (NYATA atau ASUMSI)`; perintah penghasil ditulis TANPA token dan dengan placeholder `<owner>/<repo>`, `<iid>`, `<id>`; catatan anonimisasi (apa yang diganti, bahwa struktur dipertahankan); penanda jelas bahwa SEMUA fixture `gh-*` adalah ASUMSI sampai `gh auth login` dijalankan, kecuali `gh-auth-status-loggedout.txt` yang NYATA; catatan bahwa berkas `.txt` direkam sebagai stdout+stderr gabungan (sebutkan urutannya bila relevan); catatan untuk tiket 02 sampai 04 (fixture dipakai sebagai acuan bentuk, bukan unit test karena epic tanpa unit test).

### 10. Tulis `docs/git/cli-findings.md`
Bahasa Indonesia, mengikuti struktur `docs/lark/cli-findings.md`. Judul `# Temuan spike glab dan gh`. Bagian wajib (nomor boleh disesuaikan):
1. **Versi dan lingkungan**: versi `glab`, `gh`, `git`; tanggal; status login (glab login `gitlab.com` tanpa menyebut akun; gh belum login); catatan pesan "update tersedia"/notice di stderr bila ada dan cara mengabaikannya; bahwa nilai contoh berasal dari fixture teranonimkan.
2. **Tabel perintah** (kolom: perintah, flag wajib, flag opsional penting, format JSON, bentuk `-R`, fixture, asal NYATA/ASUMSI) untuk: `glab auth status`, `gh auth status`, `glab mr list`, `glab mr view`, perintah daftar pipeline GitLab final, perintah detail pipeline GitLab final, `gh pr list`, `gh pr view`, `gh run list`, `gh run view`, `git remote`, `git remote get-url`. Nyatakan flag final: `-R/--repo`, `-F/--output json` (glab), `--json <fields>` (gh), flag batas jumlah (`--per-page`/`-P` glab, `--limit`/`-L` gh), flag filter state MR/PR (`--merged`, `--closed`, `--all`, `--state`), `--hostname` untuk auth status. Cantumkan juga flag yang HARUS ditolak allowlist (`--show-token`, flag tulis, `api`).
3. **Rekomendasi tabel allowlist**: daftar baris literal (tool, subcommand, flag diizinkan) untuk `allowlist.go`, termasuk flag yang diperlukan tiket 02 sampai 04; nyatakan subcommand pipeline final dan mana yang diizinkan.
4. **Skema JSON ringkas per perintah**: path field yang dipakai dan target field tipe ternormalisasi (tabel pemetaan field CLI -> `MergeRequest`/`Pipeline`/dst. untuk GitLab dan GitHub); tandai tiap kolom NYATA atau ASUMSI.
5. **Jawaban empat pertanyaan spike** (tegas, satu butir per pertanyaan, di bagian tersendiri): (1) badge pipeline dan status review di daftar MR: ada/tidak, putusan aktif/dihilangkan; (2) perintah dan bentuk daftar/detail pipeline GitLab, `gh run list/view` final; (3) cara membaca host login (bentuk keluaran, exit code, apakah `--hostname`, `--show-token` tidak dipakai); (4) bentuk `gh` tanpa login (keluaran nyata yang direkam) dan fixture GitHub berstatus ASUMSI.
6. **Pemetaan status**: tabel GitLab (`created, waiting_for_resource, preparing, pending, running, success, failed, canceled, skipped, manual, scheduled` plus nilai tambahan yang teramati) dan GitHub (`queued, in_progress, completed` + conclusion `success, failure, timed_out, startup_failure, cancelled, skipped, neutral, action_required, stale`) ke `pending|running|success|failed|canceled|skipped|unknown`; tandai selisih dari nilai awal overview bagian 2.3 bila ada. Sertakan pemetaan state MR/PR (`opened/merged/closed/locked` -> `open|merged|closed`) dan `reviewState` bila aktif.
7. **Host login**: bentuk teks keluaran login dan belum login, di stdout atau stderr, exit code, aturan parse (regex/aturan tekstual yang disarankan untuk mengambil host dan user, tahan terhadap perubahan format dan teks terpotong 300 karakter pada galat `RunCLI`), aturan pemilihan host utama untuk `Status()` (`gitlab.com`/`github.com` bila ada, selain itu host pertama), putusan pemetaan `Status()`: apakah exit non-nol berarti belum login (ikuti temuan nyata).
8. **Pola galat**: tabel kelas galat (not_logged_in, permission, not_found, timeout/jaringan) -> contoh substring stderr yang stabil, exit code, dan rekomendasi pola `classify`; tandai yang tidak teramati sebagai ASUMSI dengan alasan.
9. **Remote git**: tabel contoh `git remote get-url origin` (ssh scp-like, ssh://, https dengan kredensial, https biasa, subgroup, port) dan hasil urai (host, path) yang diharapkan; catatan `-R` untuk subgroup dan self-hosted.
10. **Pengecualian dan pemotongan fitur**: tabel eksplisit (asumsi/keputusan): badge pipeline di baris MR (AKTIF/DIHILANGKAN + alasan), status review di baris MR (AKTIF/DIHILANGKAN), detail pipeline GitLab stage/job (tersedia lewat subcommand mana, atau pemotongan), `Mergeable` di detail MR, `LastPipeline`, GitHub tanpa login (fixture ASUMSI, verifikasi nyata menunggu `gh auth login`, tidak memblokir), serta perubahan bentuk field opsional tipe bila ada. Tiap baris memuat keputusan, dampak ke tiket 02 sampai 04, dan fitur yang harus menyesuaikan.
11. **Ringkasan keputusan** di akhir dokumen (bagian ini dirujuk Wiring butir 6 untuk diperbarui nanti): daftar singkat nilai final untuk allowlist, pemetaan status, pola galat, parser host login, bentuk `-R`.

Dokumen tidak memuat token, email, nama user/grup/repo asli, atau isi MR/pipeline asli; contoh hanya dari fixture teranonimkan. Tandai setiap klaim yang tidak diverifikasi nyata sebagai ASUMSI.

### 11. Verifikasi akhir
Jalankan pemeriksaan langkah 8 lagi terhadap dokumen dan folder testdata; pastikan semua berkas bernama persis ada (`ls internal/gitdata/testdata`), `docs/git/cli-findings.md` ada dan memuat semua bagian di atas, dan `git status --short` hanya menunjukkan path milik fitur ini. Jalankan `go build ./...` hanya sebagai sanity (tidak boleh terpengaruh; kegagalan karena fitur lain bukan tanggung jawab fitur ini, laporkan saja). Baca ulang berkas dokumen sebelum melapor.

## Perubahan
Dibuat (semua baru, tanpa perubahan berkas yang sudah ada):
- `docs/git/cli-findings.md`
- `internal/gitdata/testdata/README.md`
- `internal/gitdata/testdata/glab-mr-list.json`
- `internal/gitdata/testdata/glab-mr-view.json`
- `internal/gitdata/testdata/glab-pipeline-list.json`
- `internal/gitdata/testdata/glab-pipeline-get.json`
- `internal/gitdata/testdata/glab-auth-status.txt`
- `internal/gitdata/testdata/gh-pr-list.json`
- `internal/gitdata/testdata/gh-pr-view.json`
- `internal/gitdata/testdata/gh-run-list.json`
- `internal/gitdata/testdata/gh-run-view.json`
- `internal/gitdata/testdata/gh-auth-status.txt`
- Opsional bila temuan memerlukan: `glab-auth-status-loggedout.txt`, `gh-auth-status-loggedout.txt`, `glab-ci-jobs.json`, `git-remote-urls.txt` (semua terdaftar di README fixture)

Tidak ada file Go, frontend, atau wiring yang dibuat/diubah.

## Wiring
Tidak ada yang disambungkan; fitur ini hanya dokumen dan fixture. Yang diekspos untuk langkah Wiring dan fitur lain:
- `docs/git/cli-findings.md` bagian "Rekomendasi tabel allowlist", "Pemetaan status", "Pola galat", "Host login", dan "Ringkasan keputusan" (Wiring butir 6 di overview mencocokkan allowlist, `NormalizeGit*Status`, `classify`, parser host adapter dan `repo.go` dengan bagian ini, dan memperbarui ringkasan di akhir dokumen bila keputusan berubah).
- Fixture `internal/gitdata/testdata/*` sebagai acuan bentuk parser tiket 02 sampai 04 dan adapter.
JANGAN mengubah file milik fitur lain untuk menyelaraskan; selisih dengan nilai awal overview dicatat di dokumen temuan saja.

## Selesai bila
- Semua perintah `glab` di langkah 2 sampai 6 dijalankan nyata (read-only) dan fixture `glab-*` tersimpan; fixture `gh-*` dibuat dari dokumentasi dan ditandai ASUMSI (kecuali keluaran `gh` belum-login yang nyata).
- `jq .` lulus untuk semua `*.json` di `internal/gitdata/testdata/`.
- Pemeriksaan anonimisasi (grep nilai asli di scratchpad dan pola token/email) kosong untuk `internal/gitdata/testdata/` dan `docs/git/`.
- `internal/gitdata/testdata/README.md` mencatat perintah penghasil tiap berkas, asal NYATA/ASUMSI, dan penanda fixture GitHub ASUMSI.
- `docs/git/cli-findings.md` memuat: jawaban tegas empat pertanyaan spike, flag final (`-R`, `-F json`, `--json`) dan field tersedia, tabel allowlist, pemetaan status, host login, pola galat, bentuk `git remote get-url`, bagian "Pengecualian dan pemotongan fitur" dengan keputusan eksplisit untuk badge pipeline dan status review di baris MR, dan ringkasan keputusan.
- Tidak ada perubahan Go atau frontend; `git status --short` hanya menunjukkan `docs/git/` dan `internal/gitdata/testdata/` (selain yang sudah ada sebelumnya); tidak ada `*_test.go`.
- Guard fitur ini: pemeriksaan `jq` dan grep anonimisasi di atas lulus; `go build ./...` tidak terpengaruh (tidak ada Go diubah).
