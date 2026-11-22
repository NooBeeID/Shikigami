# Plan: Wiring dan verifikasi daftar MR/PR

## Metadata
- Tiket: `git-connector-ticket-02-git-mrs` (`docs/tickets/git-connector-ticket-02-git-mrs.md`)
- Fitur: `git-mrs-wiring` (fitur penutup, tanpa file sendiri)
- ownedPaths: (kosong) hanya verifikasi dan pelaporan
- Depends on foundation: tidak (epic ini tidak punya foundation; lihat overview bagian 4). Bergantung pada selesainya `git-mrs-data`, `git-mrs-ctl`, dan `git-mrs-ui`; jalankan HANYA setelah ketiganya selesai dan ter-merge ke worktree kerja.

## Tujuan
Memastikan tiga fitur paralel tersambung dan berjalan end-to-end tanpa menambah file: IPC `git.mrs.list` terdaftar di daemon dan terjawab, subcommand `shikigamictl git mrs` terdaftar, `MrList` tampil di `ProjectSection`, klik baris membuka pane `git-mr` yang dipersist dan dipulihkan, dan perilaku di app terbukti dengan data nyata `glab` (GitLab). Jalur GitHub dibuktikan lewat fixture dan dicatat sebagai butir terbuka. `go build ./...` dan build frontend lulus.

## Path yang dimiliki
Tidak ada. Fitur ini TIDAK boleh membuat atau mengubah file apa pun di repo (termasuk file milik `git-mrs-data`, `git-mrs-ctl`, `git-mrs-ui`, file tiket 1, dan `overview.md`). Semua temuan kekurangan dilaporkan di hasil akhir, bukan diperbaiki. Artefak sementara (log, keluaran) ditaruh di scratchpad/di luar repo.

## Kontrak
Tidak menyediakan apa pun; hanya memverifikasi kontrak overview, merujuk bagian berikut (jangan diulang):
- Overview 2.1 (IPC `git.mrs.list`, hasil bungkus `{"mrs": [...]}`, `mrs` selalu array).
- Overview 2.3 (`RegisterModule("git-mrs", ...)` di `cmd/shikigamid/wire_git_mrs.go`; modul lain bernama `git` di `wire_git.go`, nama tidak boleh bentrok).
- Overview 2.4 (`registerGitSub("mrs", gitMrs)` di `cmd/shikigamictl/cmd_git_mrs.go`; bentuk `shikigamictl git mrs <workspaceId> <projectId> open|merged|closed`).
- Overview 2.5 dan 2.6 (`MrList` props `{provider, repo}`; pane `git-mr` dengan leaf `provider/projectId/number`, judul `!n`/`#n` + judul MR).
- Overview 5 (wiring checklist) dan 6 (build guard).

## Langkah
Kerjakan berurutan. Bila satu langkah gagal karena kekurangan di file milik fitur lain, catat (file, baris, gejala, usulan perbaikan) lalu lanjutkan langkah lain yang masih bisa dijalankan.

1. Pra-syarat. Dari akar repo: `git status` dan `git log --oneline -8`; pastikan file berikut ada dan bukan placeholder: `internal/gitdata/mrs.go`, `cmd/shikigamid/wire_git_mrs.go`, `cmd/shikigamictl/cmd_git_mrs.go`, `frontend/src/modules/git/MrList.svelte`, `MrRow.svelte`, `StatusBadge.svelte`, `mrClient.ts`. Bila ada yang hilang, berhenti dan laporkan (jangan membuatnya).
2. Build Go. `go build ./...` dari akar repo harus lulus. Jalankan juga `go vet ./internal/gitdata/... ./cmd/...` sebagai pemeriksaan tambahan (tidak memblokir bila hanya peringatan lama di luar epic ini).
3. Verifikasi registrasi daemon (checklist 5.1). 
   - `grep -rn 'RegisterModule("git' cmd/shikigamid` harus menampilkan tepat satu `"git"` dan satu `"git-mrs"`; pastikan tidak ada nama modul ganda di seluruh `cmd/shikigamid`.
   - Baca `wire_git_mrs.go`: `init()` memanggil `RegisterModule("git-mrs", ...)`; handler `git.mrs.list` memvalidasi `workspaceId`/`projectId`/`state`, memakai `gitResolveRepo` dan `mapGitError`, dan mengembalikan `mrs` non-nil. Pastikan tidak ada nama simbol yang bentrok dengan tiket lain dan tidak ada impor `internal/workspace` di `internal/gitdata`.
   - Pastikan `internal/gitdata/mrs.go` hanya memakai `Run(...)` (tanpa `os/exec`, tanpa `connector.RunCLITimeout` langsung) dan hanya flag panjang yang ada di allowlist (`--repo --output --per-page --merged --closed`; `--state --limit --json`).
4. Verifikasi registrasi ctl (checklist 5.2). Baca `cmd_git_mrs.go`: `init()` memanggil `registerGitSub("mrs", gitMrs)`. Bangun ctl (`go build -o <scratchpad>/shikigamictl ./cmd/shikigamictl`), jalankan `shikigamictl git` tanpa argumen dan pastikan daftar subcommand mencantumkan `mrs` (bila `cmd_git.go` tidak mencetak daftar dinamis, cukup pastikan `git mrs` tanpa argumen mencetak pesan `penggunaan: shikigamictl git mrs <workspaceId> <projectId> open|merged|closed`). Uji juga `git mrs a b salah` (state tidak valid) -> pesan penggunaan dan kode keluar bukan nol.
5. Verifikasi IPC terhadap daemon nyata. Jalankan daemon dengan cara yang dipakai repo (cek `Makefile`/`README`/`docs`; bila ada daemon dev yang sedang berjalan gunakan itu, jangan membunuh proses yang bukan Anda luncurkan sendiri dan jangan memakai `pkill` pola luas). Cari `workspaceId` dan `projectId` project dengan remote gitlab.com lewat `shikigamictl git repos --workspace <workspaceId>` (atau daftar workspace via ctl yang ada). Lalu:
   - `shikigamictl git mrs <ws> <proj> open`, `merged`, `closed`: masing-masing mencetak baris MR (nomor `!n`, state, judul, penulis, `src->tgt`, URL) dan `<n> baris`; periksa waktu respons wajar (< 20 detik) dan `state` hasil sesuai permintaan.
   - Galat: `projectId` ngawur -> galat `[not_found] ...` terbaca; `state` kosong/ngawur lewat IPC langsung (bila bisa) -> `invalid_params`; kode keluar bukan nol dan tanpa panic.
   - Daemon yang dijalankan sendiri dimatikan lewat PID yang Anda luncurkan.
6. Build frontend. `cd frontend && npm run build` harus lulus; jalankan juga `npm run check` (skrip `svelte-check` ada di `frontend/package.json`) dan laporkan galat yang berasal dari file epic ini (`MrList`, `MrRow`, `StatusBadge`, `mrClient`); peringatan lama di luar epic dicatat saja.
7. Verifikasi titik sambung UI statis (checklist 5.3 dan 5.4), hanya membaca:
   - `ProjectSection.svelte` merender `<MrList {provider} {repo} />` di dalam `{#if open}`; `MrList.svelte` menerima persis `{provider: Provider; repo: Repo}` via `$props()`.
   - `MrList` memuat hanya saat tampil (bukan saat terlipat), memakai `selectedState` dengan kunci `repoKey(repo)`, penghitung urutan untuk respons usang, dan memuat ulang saat identitas `repo` berubah (Segarkan).
   - Klik baris memanggil `layout.openPane({ kind: 'git-mr', provider, projectId: repo.projectId, number: mr.number, title })`; `modules/git/index.ts` mendaftarkan pane kind `git-mr`; `modules/layout/model.ts` memetakan `provider/projectId/number` di `Leaf`, `NewLeaf`, pembuat leaf, dan `hydrateNode` (persist dan pulih).
8. Verifikasi di app (checklist 5.5; data nyata `glab`, project remote gitlab.com). Jalankan app memakai skill `run` atau cara yang dipakai repo (bangun frontend lalu jalankan app desktop/dev; bila tidak bisa menjalankan GUI di lingkungan ini, catat sebagai butir terbuka dengan alasan, jangan dianggap lulus). Periksa satu per satu dan catat lulus/gagal:
   1. Rail GitLab muncul; bagian project terbuka menampilkan daftar MR (bukan placeholder), default Terbuka.
   2. Ganti ke Digabung lalu Tertutup: daftar memuat ulang, teks kosong sesuai provider/status bila tidak ada data ("Tidak ada MR digabung", dst.).
   3. Pilihan status bertahan setelah pindah rail lalu kembali (panel di-unmount).
   4. Baris menampilkan judul terpotong dengan tooltip, `!n`, penulis, `src -> tgt`, penanda Draft bila ada, waktu relatif; GitLab tanpa badge review/pipeline.
   5. Buka di browser, Salin link (tempel untuk memeriksa URL), Kirim ke agen: dengan sesi aktif URL tertempel tanpa Enter; tanpa sesi aktif muncul toast "Tidak ada sesi aktif". Klik tombol tidak membuka pane.
   6. Klik baris (dan Enter/Spasi saat fokus) membuka pane `git-mr` placeholder dengan judul `!n judul`.
   7. Restart app: pane `git-mr` dipulihkan dengan `provider/projectId/number` yang sama.
   8. Segarkan di panel memuat ulang daftar bagian yang terbuka; bagian terlipat tidak memicu panggilan (amati lewat log daemon atau proses `glab` yang berjalan).
   9. Galat aman: simulasikan tanpa mengubah file repo, mis. jalankan app dengan `PATH` tanpa `glab`, atau project tanpa izin/repo salah bila ada; tampil pesan terbaca + "Coba lagi" (+ tautan ke Pengaturan > Connector untuk `cli_missing`/`not_logged_in`), galat satu project tidak memengaruhi project lain, tidak ada crash.
   10. Beban: buka banyak bagian sekaligus tidak melebihi 3 proses `glab` bersamaan (runner yang menjaga).
9. Jalur GitHub lewat fixture (hanya baca). Pastikan `internal/gitdata/testdata/gh-pr-list.json` ada dan baca `mrs.go` untuk memastikan pemetaan GitHub (state lowercase, `isDraft`, `headRefName`/`baseRefName`, `reviewDecision` -> snake_case lowercase, agregasi `statusCheckRollup`, `url`) sesuai overview 2.2. Karena tanpa unit test dan tanpa file baru, buktikan dengan penelusuran kode terhadap fixture; bila ada cara tanpa menulis ke repo (mis. skrip `go run` sementara di scratchpad yang mengimpor `internal/gitdata` lewat `go run` dari modul) pakai itu, tetapi jangan meninggalkan file di repo. Catat butir terbuka: "verifikasi daftar PR di app menunggu `gh auth login`" (tidak memblokir).
10. Catat selisih yang diketahui (checklist 5.6): `RepoFlag` self-hosted `HOST/PATH` vs URL penuh (dihindari lewat `mrRepoArg`; laporkan bila ada self-hosted nyata yang bermasalah atau bila `mrRepoArg` tidak ada).
11. Pemeriksaan akhir: `git status` memastikan hanya perubahan milik tiga fitur lain yang ada dan fitur ini tidak menambah file/perubahan apa pun (`docs/plans/...` plan ini tidak dihitung).

## Perubahan
Tidak ada file dibuat atau diubah oleh fitur ini. Keluaran fitur hanyalah laporan akhir (temuan, hasil tiap butir langkah 8, butir terbuka, usulan perbaikan untuk pemilik file bila ada kekurangan).

## Wiring
Fitur ini adalah langkah wiring itu sendiri. Tidak ada yang disambungkan lewat penyuntingan file: seluruh sambungan sudah mandiri (`RegisterModule("git-mrs", ...)`, `registerGitSub("mrs", ...)`, titik ekstensi `<MrList {provider} {repo} />` di `ProjectSection`, pane `git-mr` di `index.ts`). Bila salah satu ternyata membutuhkan baris di file yang bukan milik fitur ini, JANGAN mengubahnya; laporkan file, baris yang dibutuhkan, dan alasannya, serta tandai `ok:false` atau butir terbuka sesuai tingkat keparahan.

## Selesai bila
- `go build ./...` lulus; `cd frontend && npm run build` lulus (dan `npm run check` tidak menambah galat dari file epic ini).
- `git.mrs.list` terjawab daemon yang berjalan; `shikigamictl git mrs <ws> <proj> open|merged|closed` mencetak hasil; state tidak valid ditolak dengan pesan penggunaan.
- Daftar MR muncul di panel GitLab dengan data nyata `glab`; pemilih status, ganti status, bertahan saat unmount, Salin link, Kirim ke agen (toast tanpa sesi), Segarkan, dan galat aman terverifikasi; klik baris membuka pane `git-mr` yang dipersist dan dipulihkan setelah restart.
- GitHub dibuktikan lewat fixture dan butir terbuka "verifikasi daftar PR di app menunggu `gh auth login`" dicatat di laporan akhir.
- Tidak ada file di luar ownedPaths (kosong) yang diubah oleh fitur ini. Guard: build Go dan build frontend sukses; tanpa unit test.
