# Connector Git (GitLab dan GitHub: MR/PR dan pipeline) per project

> Menumpang pada **rail menu utama** (`2026-11-22-08-main-rail-design.md`): Git adalah item
> rail dengan panel sendiri (`git-gitlab`, `git-github`). Pola mengikuti Connector Lark
> (`2026-11-22-09-lark-connector-design.md`). Kerangka connector ada di
> `2026-11-22-settings-framework-design.md`. Ini menggantikan "bagian 3 (Connector GitLab)"
> di rencana awal kerangka, dan sekarang mencakup GitHub juga.

## Maksud dan alasan
User bekerja dengan agen di Shikigami sambil merujuk Merge Request (GitLab), Pull Request
(GitHub), dan status pipeline CI. Untuk melihatnya sekarang harus keluar app. Tujuan: melihat
daftar dan detail MR/PR serta pipeline di dalam app, lalu **membuka di browser**,
**menyalin link**, atau **mengirim link ke agen** sebagai konteks. Agen sudah punya `glab`
dan `gh`, jadi app mengirim link, bukan isinya.

Berbeda dari Lark, sumbernya **tidak ditambahkan manual**: repo diturunkan dari remote git
tiap project di workspace aktif.

## Tujuan
- Dua connector terdaftar di kerangka Connector: `gitlab` (CLI `glab`) dan `github` (CLI
  `gh`). Status memakai CLI yang sudah login; tidak ada login atau token di app. Muncul di
  Pengaturan > Connector dengan dialog Connect (`glab auth login`, `gh auth login`).
- **Deteksi repo per project**: untuk tiap project di workspace aktif, baca remote git
  (`origin`, atau remote pertama bila tidak ada `origin`), urai host dan path repo, tentukan
  provider. Hasilnya menentukan `visible` item rail.
- Item rail **GitLab** muncul bila ada project dengan remote GitLab; **GitHub** bila ada
  remote GitHub; keduanya bila ada dua jenis; tidak ada remote dikenal berarti tidak tampil.
  Isi mengikuti workspace aktif dan berganti saat workspace berganti.
- **Panel** (per provider): satu bagian per project yang remote-nya cocok provider itu, bisa
  dilipat. Tiap bagian punya dua daftar: **MR/PR** dan **Pipeline**.
  - MR/PR: pemilih status **Terbuka | Digabung | Tertutup** (default Terbuka). Baris:
    judul, nomor (`!12` GitLab, `#12` GitHub), penulis, branch sumber ke target, penanda
    draft, status review (disetujui / diminta perubahan / belum), badge status pipeline, waktu
    pembaruan.
  - Pipeline: daftar terbaru (usul 20). Baris: ref (branch/tag), status, sha pendek, penulis
    pemicu bila ada, durasi, waktu.
- Klik baris membuka **pane tengah** (dua jenis pane, `git-mr` dan `git-pipeline`):
  - `git-mr`: judul, status, penulis, branch, label, reviewer dan statusnya, deskripsi,
    status bisa-digabung atau konflik bila tersedia, pipeline terakhir (klik membuka
    `git-pipeline`).
  - `git-pipeline`: status keseluruhan, ref, sha, durasi, daftar **stage** berisi **job**
    (nama, status, durasi, link job).
- Header tiap pane dan tiap baris panel punya **Buka di browser**, **Salin link**, dan
  **Kirim ke agen**. Kirim ke agen memakai `pasteToSession` ke sesi aktif (jalur yang sama
  dengan Lark), URL plus spasi, tanpa Enter otomatis. Tanpa sesi aktif: toast "Tidak ada
  sesi aktif".
- Tombol **Segarkan** di panel dan di pane. Data dimuat saat panel/pane dibuka.
- Keadaan: memuat, kosong ("Tidak ada MR terbuka"), galat terbaca (CLI belum terpasang,
  belum login, izin kurang, repo tidak ditemukan, timeout) dengan ajakan ke Pengaturan >
  Connector bila relevan.

## Bukan tujuan
- **Read-only.** Tidak ada membuat, menyetujui, menggabung, menutup MR/PR; tidak ada retry,
  cancel, atau trigger pipeline; tidak ada komentar atau diff.
- Tidak ada environment variable / CI variable (sempat disebut di spec Lark; ditunda).
- Tidak ada issue, release, atau Actions di luar daftar run/pipeline.
- Tidak ada polling otomatis atau notifikasi saat status berubah (hanya Segarkan manual).
- Tidak ada konfigurasi sumber manual dan tidak ada perubahan `WorkspaceConfig`.
- Tidak ada cache disk dan tidak menyimpan isi MR/PR secara permanen.
- Tidak mengubah kerangka connector selain menambah dua adapter.
- Tidak ada filter "milik saya" / reviewer / label di versi ini.

## Anti-tujuan
- Menyimpan token atau kredensial GitLab/GitHub di config app.
- Menjalankan perintah `glab`/`gh` berisiko tulis (hanya perintah baca, lewat allowlist).
- Mengganggu sesi terminal yang berjalan saat panel dimuat atau CLI lambat.
- Menulis apa pun ke folder project user (membaca remote git saja).
- Item rail muncul untuk provider yang tidak punya project di workspace aktif.

## Batasan
- Go module `github.com/NooBeeID/shikigami`. Adapter connector mengikuti `internal/connector`
  (`Register` dari `init()`, `RunCLI` dengan PATH login-shell, `Status`, `ConnectGuide`).
- Rail dan registry sudah ada (`registerRailItem` dengan `visible?: Readable<boolean>`,
  `registerPaneKind` di `modules/layout/registry.ts`; `PaneKind` di
  `modules/layout/model.ts` perlu `'git-mr' | 'git-pipeline'`).
- Di mesin pengembangan saat desain ini ditulis: `glab` sudah login ke `gitlab.com`, `gh`
  belum login. Jalur GitHub diuji dengan fixture; verifikasi di aplikasi untuk GitHub
  menunggu `gh auth login`.
- Teks UI bahasa Indonesia, mengikuti app.
- Satu pemanggilan CLI punya timeout; usul 20 detik untuk daftar, sama dengan Lark.

## Pendekatan yang dipertimbangkan
Tampilan hasil:
- (1) Panel saja: daftar di panel, aksi per baris, tanpa pane tengah. Murah, tetapi detail
  (deskripsi MR, job pipeline) tidak terlihat di app.
- (2) Panel ditambah pane tengah seperti Lark. **Dipilih oleh user.** Konsekuensi:
  perintah detail tambahan (`glab mr view`, `gh pr view`, detail pipeline/run) dan
  normalisasi dua kali lebih banyak.
- (3) Hanya badge jumlah di rail. Dicoret: terlalu tipis.

Sumber data (keputusan saya, mohon dicek):
- (a) Subcommand JSON bawaan CLI (`glab mr list -F json -R <repo>`, `gh pr list --json ...
  -R <repo>`, `gh run list --json ...`). **Dipilih.** Flag `-R/--repo`, `-F/--output json`,
  `--json` sudah dicek ada. `-R` berarti tidak perlu mengatur direktori kerja pemanggilan.
- (b) `glab api` / `gh api` (REST mentah): bentuk seragam, tetapi lebih banyak
  normalisasi dan lebih mudah salah paham izin. Dicadangkan bila subcommand tidak memuat
  field yang dibutuhkan.

Asal repo:
- Diturunkan dari remote git project. **Dipilih.** Selaras dengan desain rail; tanpa
  konfigurasi baru dan tanpa risiko sumber basi. Tradeoff: project tanpa remote atau dengan
  remote tak dikenal tidak muncul.

## Desain

### Go
- `internal/connector/gitlab.go` dan `github.go`: adapter, `Register` dari `init()`.
  `Status()` memanggil `glab auth status` / `gh auth status`, memetakan ke `cli_missing`,
  `not_logged_in`, `connected` (dengan `User`). Tidak memuat output mentah CLI. Panduan
  Connect: `glab auth login` / `gh auth login`, lalu Cek ulang.
- Paket data baru `internal/gitdata` membungkus `connector.RunCLI`. Hanya perintah baca,
  lewat allowlist (ikuti pola `internal/larkdata/allowlist.go`). Tipe ternormalisasi, bukan
  JSON mentah CLI:
  - `Repo { ProjectID, ProjectName, Provider ("gitlab"|"github"), Host, Path, WebURL }`
  - `MergeRequest { Number, Title, Author, SourceBranch, TargetBranch, State ("open"|"merged"|"closed"), Draft, ReviewState, PipelineStatus, UpdatedAt, URL }`
  - `MergeRequestDetail` = `MergeRequest` + `Description`, `Labels`, `Reviewers`,
    `Mergeable`, `LastPipeline` (id, status, url bila ada).
  - `Pipeline { ID, Ref, SHA, Status, Author, DurationSec, UpdatedAt, URL }`
  - `PipelineDetail` = `Pipeline` + `Stages []{Name, Jobs []{Name, Status, DurationSec, URL}}`
  - Status pipeline dinormalisasi: `pending | running | success | failed | canceled |
    skipped | unknown` (GitHub Actions: `queued/in_progress/completed` + conclusion
    dipetakan ke himpunan yang sama).
- **Deteksi repo**: `git remote get-url` (hanya membaca) di folder tiap project; urai
  host dan path (ssh dan https). `github.com` berarti `github`; `gitlab.com` berarti
  `gitlab`. Host lain (self-hosted) dianggap provider yang CLI-nya sudah login ke host itu
  (GitLab: host terdaftar di `glab`; GitHub Enterprise: host terdaftar di `gh`); tidak cocok
  berarti tidak dikenal. Hasil deteksi tidak disimpan ke disk.
- **IPC** (didaftarkan di daemon lewat `wire_git.go`, pola `wire_lark.go`):
  `git.repos.list` {workspaceId}; `git.mrs.list` {workspaceId, projectId, state};
  `git.mr.get` {workspaceId, projectId, number}; `git.pipelines.list` {workspaceId,
  projectId}; `git.pipeline.get` {workspaceId, projectId, id}. Galat memakai bentuk galat
  yang sama dengan Lark.
- `shikigamictl git ...` (repos/mrs/mr/pipelines/pipeline) sebagai alat debug, sejalan
  dengan `connector` dan `lark`.

### Frontend (`frontend/src/modules/git/`)
- Mendaftarkan dua item rail: `git-gitlab` (order 20) dan `git-github` (order 21), masing-
  masing dengan `visible` dari store repo (ada project cocok di workspace aktif), ikon SVG
  inline sederhana. Mendaftarkan `registerPaneKind` untuk `git-mr` dan `git-pipeline`.
- `GitPanel` (dipakai kedua provider dengan parameter provider): bagian per project, daftar
  MR/PR dengan pemilih status, daftar Pipeline, tombol Segarkan, keadaan memuat/kosong/galat.
  State yang berarti (bagian terlipat, status terpilih) di store, karena panel di-unmount saat
  rail lain aktif (hasil pemeriksaan desain rail).
- `MrPane`, `PipelinePane`, dan aksi bersama (Buka di browser, Salin link, Kirim ke agen)
  mengikuti `modules/lark/actions.ts`.
- Item rail jatuh ke `agents` bila menjadi tidak visible (perilaku rail yang sudah ada).

## Keputusan saya (mohon dicek)
- Tampilan MR dibatasi tiga status; daftar pipeline 20 terbaru tanpa filter.
- Tidak ada polling; Segarkan manual saja.
- Deskripsi MR/PR ditampilkan sebagai teks biasa dulu; builder boleh memakai ulang komponen
  markdown Lark bila impor lintas modul bersih.
- Item rail dan panel per provider memakai satu komponen dengan parameter provider.
- Project dengan remote tak dikenal diabaikan diam-diam (tanpa pesan).
- Self-hosted dikenali hanya bila CLI sudah login ke host itu. Bila kedua CLI login ke host
  yang sama, GitLab diutamakan (jarang terjadi). Remote self-hosted yang CLI-nya belum login
  tetap tersembunyi dan tidak memunculkan ajakan login; ini batasan yang diketahui.
  `gitlab.com` dan `github.com` selalu dikenali, jadi panelnya tampil dengan ajakan login
  bila CLI belum login.
- **Path project**: `Project.Path` (absolut) sudah ada di model workspace. `git remote
  get-url` dijalankan dari folder itu; git sendiri menemukan akar repo, jadi project di
  subfolder monorepo ikut bekerja. Satu project = satu repo (remote `origin`, atau remote
  pertama).
- **Project berbagi remote yang sama** digabung menjadi satu bagian di panel (kunci:
  host + path repo); judul bagian menyebut nama project-nya.
- **Bentuk `-R`**: `OWNER/REPO` (atau `GROUP/SUBGROUP/REPO`) untuk host bawaan, dan
  `HOST/PATH` untuk host self-hosted. GitLab dengan subgroup berarti path lebih dari dua
  segmen; ini wajar dan tidak dipotong.
- **Kapan deteksi berjalan**: saat app mulai, saat workspace berganti, saat project
  ditambah atau dihapus, dan saat Segarkan. Hasilnya di memori; membaca remote adalah operasi
  lokal yang cepat. Item rail baru muncul setelah deteksi pertama selesai (tanpa flicker
  karena item Git tidak tampil dulu). Daftar host login dari CLI di-cache singkat di memori.
- **Kolom baris MR**: wajib: judul, nomor, penulis, branch, status, draft, waktu, URL.
  Opsional (tampil hanya bila daftar CLI sudah memuatnya, tanpa panggilan tambahan per
  baris): status review dan badge pipeline. Bila spike menunjukkan GitLab tidak
  memuatnya di daftar, kolom itu dihilangkan di baris dan hanya muncul di detail.
  Tidak ada panggilan per baris.
- **Beban panggilan**: bagian project dimuat saat dibuka (bagian pertama terbuka secara
  default, lainnya terlipat), maksimal 3 pemanggilan CLI berjalan bersamaan, dan galat satu
  bagian tidak memengaruhi bagian lain.
- **Kirim ke agen**: pane dan baris MR/pipeline mengirim URL MR/pipeline; baris job
  mengirim URL job.
- **GitHub tanpa login**: jalur GitHub boleh merge dengan fixture sebagai bukti. Verifikasi
  di aplikasi sungguhan dicatat sebagai butir terbuka sampai `gh auth login` dijalankan,
  dan tidak memblokir merge.

## Yang perlu dipastikan lewat spike awal (bagian dari plan, bukan keputusan user)
1. Apakah `glab mr list -F json` memuat status pipeline terakhir tiap MR. Bila tidak,
   badge pipeline di baris MR butuh satu panggilan tambahan (batasi jumlah panggilan).
2. Perintah dan bentuk keluaran detail pipeline GitLab (stage dan job) dan detail run
   GitHub (`gh run view --json jobs`).
3. Cara membaca daftar host yang sudah login dari `glab` dan `gh`.
4. Bentuk keluaran `gh` tanpa login di mesin pengembangan: fixture dibuat dari dokumentasi
   dan keluaran `glab`; catat sebagai asumsi sampai `gh auth login` dijalankan.
Hasil spike dicatat di dokumen temuan (pola `docs/lark/cli-findings.md`).

## Diserahkan ke pembangun
- Gaya visual panel, baris, pane, badge status (ikuti variabel CSS yang ada).
- Nama pesan galat, aksesibilitas (fokus, urutan tab).
- Bentuk persis JSON ternormalisasi setelah spike, dan nama berkas.
- Memecah pekerjaan menjadi tiket paralel (usul awal: foundation tipe dan IPC; adapter
  GitLab; adapter GitHub; UI panel; UI pane; wiring dan verifikasi).
