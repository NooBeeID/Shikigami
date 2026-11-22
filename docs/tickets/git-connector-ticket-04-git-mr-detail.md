---
id: git-connector-ticket-04-git-mr-detail
title: Detail MR/PR
spec: docs/superpowers/specs/2026-11-22-10-git-connector-design.md
status: ready
wave: 4
depends_on: [git-connector-ticket-03-git-pipelines]
---
# Detail MR/PR

## Tujuan
Epic ini melengkapi connector Git dengan pane detail `git-mr`: pengguna bisa membaca judul, status, penulis, branch, label, reviewer beserta statusnya, deskripsi, status bisa-digabung/konflik, dan pipeline terakhir sebuah MR (GitLab) atau PR (GitHub) tanpa keluar dari app. Dari pane itu pengguna membuka di browser, menyalin link, atau mengirim link ke agen sebagai konteks. Ini juga tiket penutup connector Git: wiring akhir dan verifikasi menyeluruh seluruh spec dilakukan di sini.

## Hasil yang bisa dijalankan
Di atas tiket 01 sampai 03 (rail Git, panel, daftar MR/PR, daftar dan detail pipeline):
- Klik baris MR/PR di panel membuka pane `git-mr` yang berisi data lengkap dari `glab` nyata (GitLab), bukan placeholder lagi.
- Pane menampilkan judul, `StatusBadge` status, penulis, `source -> target` branch, label, daftar reviewer dengan statusnya, deskripsi sebagai teks biasa, status bisa-digabung atau konflik (bila CLI memuatnya), dan pipeline terakhir. Klik pipeline terakhir membuka pane `git-pipeline` untuk pipeline itu.
- Header pane punya Buka di browser, Salin link, Kirim ke agen, dan Segarkan.
- Keadaan memuat dan galat (CLI belum terpasang, belum login, izin kurang, repo/MR tidak ditemukan, timeout) tampil aman dan pulih lewat Segarkan atau ambil ulang.
- `shikigamictl git mr <workspaceId> <projectId> <number>` mencetak detail sebagai alat debug.
- `go build ./...` dan build frontend lulus; aplikasi jalan.

## Batasan
In scope:
- `git.mr.get` (IPC), normalisasi detail GitLab dan GitHub ke `MergeRequestDetail`.
- Pane `git-mr` lengkap, `ReviewerList`, klien IPC bertipe.
- Subcommand ctl `git mr`.
- Wiring dan verifikasi akhir seluruh connector Git; catatan butir terbuka verifikasi GitHub.

Out of scope:
- Aksi tulis apa pun (setujui, gabung, tutup, komentar, diff), retry/cancel pipeline.
- Render markdown deskripsi (teks biasa dengan `white-space: pre-wrap`; boleh memakai komponen markdown Lark hanya bila impor lintas modul bersih, bukan kewajiban).
- Polling otomatis, cache disk, token di config, unit test.
- Mengubah file milik tiket 01 sampai 03 selain yang tercantum di "Path yang dimiliki". `MrPane.svelte` adalah placeholder milik tiket 01 yang kepemilikannya diambil alih tiket ini tanpa mengubah props.

## Kontrak
Dipakai (dari tiket sebelumnya, jangan diubah):
- `internal/gitdata`: `Runner`/pemanggil CLI beserta allowlist (`runner.go`, `allowlist.go`), galat (`errors.go`), resolusi repo dari `projectId` dan bentuk `-R` (`repo.go`), tipe `MergeRequest`, `MergeRequestDetail`, `Repo` (`types.go`), fixture di `internal/gitdata/testdata/` dan temuan di `docs/git/cli-findings.md` (acuan parser dan bentuk field opsional). Batas 3 pemanggilan CLI bersamaan dan timeout 20 detik sudah ditangani runner.
- Frontend: `StatusBadge.svelte` (tiket 02), `actions.ts` (`openInBrowser(url)`, `copyLink(url)`, `sendToAgent(url)`), `client.ts` (pembungkus IPC dan bentuk galat), pane kind `git-mr` dan `git-pipeline` di `modules/layout/model.ts` dengan field leaf `provider`, `projectId`, `number` (MR) / `id` (pipeline), serta API pane yang ada untuk membuka pane baru (ikuti cara tiket 03 membuka `git-pipeline` dari baris pipeline).
- Galat IPC: bentuk galat connector/Lark yang sama (kode + pesan Indonesia).

Disediakan:
- IPC `git.mr.get`
  - Request: `{ workspaceId: string, projectId: string, number: number }`
  - Response: `MergeRequestDetail` (json camelCase):
    `MergeRequest{number, title, author, sourceBranch, targetBranch, state ("open"|"merged"|"closed"), draft, reviewState, pipelineStatus, updatedAt, url}` ditambah `description: string`, `labels: string[]`, `reviewers: {name, state}[]` (state ternormalisasi, misal `approved | changes_requested | pending`), `mergeable?: "mergeable" | "conflict" | "unknown"` (kosong/dihilangkan bila CLI tidak memuat), `lastPipeline?: {id, status, url}` (hilang bila tidak ada). Bentuk persis field opsional mengikuti `docs/git/cli-findings.md`.
  - Galat: kode + pesan Indonesia dari `errors.go`.
- Go: `gitdata.GetMergeRequest(ctx, ...) (MergeRequestDetail, error)` di `internal/gitdata/mrdetail.go`. GitLab memakai `glab mr view <number> -F json -R <repo>`; GitHub memakai `gh pr view <number> --json ... -R <repo>`. Kedua perintah wajib masuk allowlist (hanya baca).
- Frontend: `mrDetailClient.ts` mengekspor `getMrDetail(workspaceId, projectId, number): Promise<MergeRequestDetail>` bertipe; `ReviewerList.svelte` (props `{reviewers}`); `MrPane.svelte` (props tetap dari tiket 01).
- ctl: `shikigamictl git mr <workspaceId> <projectId> <number>`.

## Fitur

### git-mr-detail-data: Data detail MR/PR dan IPC git.mr.get
Deskripsi:
- `internal/gitdata/mrdetail.go`: `GetMergeRequest` menyelesaikan `projectId` ke `Repo` (lewat `repo.go`, tanpa mengimpor `internal/workspace`), memanggil CLI sesuai provider lewat runner/allowlist, lalu menormalisasi ke `MergeRequestDetail`.
  - GitLab: `description`, `labels`, `reviewers` (status dari data approval bila tersedia di keluaran; bila tidak, `pending`), status merge dari field status merge/konflik (`detailed_merge_status`/`has_conflicts` atau padanannya sesuai temuan), `LastPipeline` dari field pipeline/head pipeline.
  - GitHub: `body`, `labels`, `reviewRequests` + `latestReviews` (dipetakan ke `approved | changes_requested | pending`), `mergeable`/`mergeStateStatus`, `LastPipeline` dari `statusCheckRollup` bila ada (id dan url sebisanya; bila tidak ada id yang bermakna, hilangkan `lastPipeline`).
  - Field yang tidak ada di keluaran CLI dibiarkan kosong/hilang, bukan galat. Bagian dasar `MergeRequest` dinormalisasi dengan aturan yang sama seperti `mrs.go` (pakai ulang fungsi bersama bila ada, tanpa mengubah `mrs.go`; bila tidak bisa dipakai ulang, tulis helper lokal di `mrdetail.go`).
  - Status pipeline dipetakan ke himpunan `pending|running|success|failed|canceled|skipped|unknown`.
- `cmd/shikigamid/wire_git_mr_detail.go`: mendaftarkan `git.mr.get` sendiri (pola `wire_git*.go`/`wire_lark*.go`, lewat `init()`/registrasi mandiri sehingga tidak mengedit `wire_git.go`), memvalidasi parameter, memetakan galat ke bentuk galat IPC bersama.
- Parser diuji dari fixture di `internal/gitdata/testdata/` (tambahkan fixture detail MR bila belum ada di file baru, jangan mengubah fixture milik tiket 01), tanpa unit test.

Acceptance criteria:
- [ ] `GetMergeRequest` mengembalikan `MergeRequestDetail` terisi dari keluaran `glab mr view -F json` nyata (diperiksa lewat ctl).
- [ ] Jalur GitHub (`gh pr view --json`) menormalisasi fixture dengan benar; field hilang tidak menyebabkan galat atau panic.
- [ ] Perintah masuk allowlist dan tidak ada perintah tulis.
- [ ] `git.mr.get {workspaceId, projectId, number}` terdaftar mandiri dan membalas detail atau galat berkode.
- [ ] `projectId` tak dikenal, repo tak ditemukan, CLI hilang, belum login, dan timeout menghasilkan galat Indonesia yang terbaca.
- [ ] `go build ./...` lulus.

ownedPaths: `internal/gitdata/mrdetail.go`, `cmd/shikigamid/wire_git_mr_detail.go`.
Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini (kontrak `git.mr.get` di atas adalah titik temu dengan UI).

### git-mr-detail-ctl: shikigamictl git mr
Deskripsi:
- `cmd/shikigamictl/cmd_git_mr_detail.go`: subcommand `git mr <workspaceId> <projectId> <number>` yang memanggil IPC `git.mr.get` lewat klien ctl yang ada dan mencetak detail (judul, status, penulis, branch, label, reviewer + status, mergeable, pipeline terakhir, deskripsi) atau pesan galat. Registrasi mandiri, tanpa mengedit `cmd_git.go`; ikuti pola `cmd_git_mrs.go` dan `cmd_git_pipelines.go` untuk cara bergabung ke grup `git`. Bila daemon belum punya `git.mr.get` (fitur data belum jadi), tampilkan galat yang jelas, bukan panic.

Acceptance criteria:
- [ ] `shikigamictl git mr <ws> <proj> <n>` mencetak detail MR nyata dari GitLab.
- [ ] Argumen kurang atau `number` bukan angka menghasilkan pesan penggunaan.
- [ ] Galat IPC dicetak terbaca dan exit code bukan nol.
- [ ] `go build ./...` lulus.

ownedPaths: `cmd/shikigamictl/cmd_git_mr_detail.go`.
Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini.

### git-mr-pane-ui: Pane git-mr
Deskripsi:
- Ganti isi placeholder `MrPane.svelte` (props tidak berubah; menerima `provider`, `projectId`, `number` dari leaf pane). Muat data lewat `mrDetailClient.ts` saat mount dan saat Segarkan.
- Isi pane, berurutan:
  - Header: judul, nomor (`!12` GitLab, `#12` GitHub), tombol Buka di browser, Salin link, Kirim ke agen (memakai `actions.ts` dengan `url` MR), dan Segarkan.
  - `StatusBadge` status (Terbuka/Digabung/Tertutup, plus penanda draft), penulis, `branch sumber -> branch target`, waktu pembaruan.
  - Label sebagai chip (bagian disembunyikan bila kosong).
  - `ReviewerList.svelte`: daftar reviewer dengan nama dan status (disetujui / diminta perubahan / belum); teks "Belum ada reviewer" bila kosong.
  - Status bisa-digabung: "Bisa digabung" atau "Konflik" bila `mergeable` ada; disembunyikan bila tidak tersedia.
  - Pipeline terakhir: `StatusBadge` + id; klik membuka pane `git-pipeline` dengan `provider`, `projectId`, `id` lewat API pane yang ada (cara sama dengan tiket 03). Disembunyikan bila tidak ada.
  - Deskripsi: teks biasa dengan `white-space: pre-wrap` dan `overflow-wrap: anywhere` (tanpa `{@html}`, tanpa render markdown kecuali impor komponen markdown Lark bersih); "Tidak ada deskripsi" bila kosong.
- `mrDetailClient.ts`: pembungkus IPC bertipe untuk `git.mr.get`, dengan tipe TS `MergeRequestDetail` sesuai kontrak dan penerjemahan galat memakai bentuk di `client.ts`.
- Keadaan: memuat (skeleton/teks "Memuat..."), galat terbaca dengan tombol "Coba lagi" yang mengambil ulang, dan ajakan ke Pengaturan > Connector untuk galat CLI hilang/belum login (ikuti komponen/pola galat tiket 02 dan 03). Respons basi diabaikan bila pane diganti atau diambil ulang saat permintaan lama masih berjalan (guard nomor permintaan). Gaya mengikuti variabel CSS yang ada; teks Indonesia.

Acceptance criteria:
- [ ] Pane menampilkan semua bagian di atas dengan data nyata GitLab; bagian opsional tersembunyi rapi bila datanya tidak ada.
- [ ] Deskripsi tampil sebagai teks biasa; HTML/markdown mentah di dalamnya tidak dieksekusi.
- [ ] Klik pipeline terakhir membuka pane `git-pipeline` yang benar.
- [ ] Buka/Salin/Kirim ke agen bekerja; tanpa sesi aktif muncul toast "Tidak ada sesi aktif".
- [ ] Segarkan mengambil ulang; galat dan pemulihan (Coba lagi) bekerja; tidak ada polling.
- [ ] Pane yang dipulihkan dari layout tersimpan memuat ulang datanya dengan benar.
- [ ] Build frontend lulus.

ownedPaths: `frontend/src/modules/git/MrPane.svelte`, `frontend/src/modules/git/ReviewerList.svelte`, `frontend/src/modules/git/mrDetailClient.ts`.
Paralel: bisa dikerjakan paralel dengan fitur lain di epic ini; dikembangkan terhadap kontrak `git.mr.get`, dan pengujian dengan data nyata menunggu fitur data jadi.

### git-mr-detail-wiring: Wiring dan verifikasi akhir Git connector
Deskripsi: langkah penutup, tidak memiliki path sendiri (hanya memeriksa dan, bila perlu, memperbaiki lewat fitur pemilik path di atas). Lihat bagian Wiring. Fitur ini dikerjakan setelah tiga fitur lain selesai.

Acceptance criteria:
- [ ] Lihat daftar di bagian Wiring; seluruhnya tercentang.

ownedPaths: tidak ada.
Paralel: tidak; harus berjalan setelah `git-mr-detail-data`, `git-mr-detail-ctl`, dan `git-mr-pane-ui` selesai.

## Path yang dimiliki
Hanya ini yang boleh disentuh:
- `internal/gitdata/mrdetail.go`
- `cmd/shikigamid/wire_git_mr_detail.go`
- `cmd/shikigamictl/cmd_git_mr_detail.go`
- `frontend/src/modules/git/MrPane.svelte`
- `frontend/src/modules/git/ReviewerList.svelte`
- `frontend/src/modules/git/mrDetailClient.ts`
- (fixture detail MR baru, bila perlu, sebagai file baru di `internal/gitdata/testdata/` tanpa mengubah file yang sudah ada)

## Wiring
Tiket ini menyambungkan fiturnya lewat titik ekstensi tiket 01, tanpa bergantung pada tiket lain untuk berjalan di atas tiket 01 sampai 03:
1. `wire_git_mr_detail.go` mendaftarkan `git.mr.get` mandiri; `cmd_git_mr_detail.go` mendaftarkan subcommand mandiri. Tidak ada edit ke `wire_git.go` atau `cmd_git.go`.
2. `MrPane.svelte` sudah terdaftar sebagai pane kind `git-mr` oleh tiket 01 (`registerPaneKind`); tiket ini hanya mengganti isinya dengan props sama. Pastikan `MrList`/`MrRow` membuka pane `git-mr` dengan `provider`, `projectId`, `number` sehingga pane baru ini tampil; bila ada celah, laporkan sebagai perbaikan di file pemiliknya, bukan mengubah kontrak.
3. Verifikasi menyeluruh seluruh spec dengan app berjalan dan data `glab` nyata (`gitlab.com`):
   - [ ] Rail: item GitLab tampil bila ada project remote GitLab, GitHub tidak tampil bila tidak ada remote GitHub; berganti saat workspace berganti.
   - [ ] Panel: bagian per project (gabungan per host+path), bisa dilipat, daftar MR (pemilih Terbuka/Digabung/Tertutup) dan Pipeline termuat.
   - [ ] Klik baris MR membuka pane `git-mr` lengkap; klik pipeline terakhir membuka pane `git-pipeline`; klik baris pipeline membuka pane pipeline dengan stage dan job.
   - [ ] Aksi Buka/Salin/Kirim ke agen bekerja di baris dan header pane; toast "Tidak ada sesi aktif" tanpa sesi.
   - [ ] Segarkan di panel dan pane; keadaan memuat, kosong, galat (CLI hilang, belum login, izin, repo tidak ditemukan, timeout) terbaca; tanpa polling.
   - [ ] Semua read-only: tidak ada perintah tulis di allowlist, tidak ada token di config.
   - [ ] Layout tersimpan dengan pane `git-mr`/`git-pipeline` terpulihkan saat app dibuka ulang.
4. `go build ./...` dan build frontend lulus; aplikasi dijalankan dan dicoba langsung.
5. Catat butir terbuka: jalur GitHub (`gh`) diverifikasi hanya dengan fixture karena `gh` belum login di mesin pengembangan; verifikasi di aplikasi menunggu `gh auth login` dan tidak memblokir merge. Catat di bagian butir terbuka `docs/git/cli-findings.md` hanya bila tiket 01 sudah mengizinkan; bila tidak, cantumkan di laporan penutup implementasi.

## Guard
- `go build ./...` berhasil.
- Build frontend berhasil (perintah build di `frontend/`).
- Tanpa unit test. Aplikasi harus jalan dan pane `git-mr` terlihat bekerja dengan data nyata.
