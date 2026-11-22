---
id: git-connector-ticket-02-git-mrs
title: Daftar MR/PR
spec: docs/superpowers/specs/2026-11-22-10-git-connector-design.md
status: implemented
wave: 2
depends_on: [git-connector-ticket-01-git-repos]
---
# Daftar MR/PR

## Tujuan
Tiket 1 (`git-repos`) sudah menyediakan connector `gitlab`/`github`, deteksi repo per project, rail Git, panel `GitPanel` dengan bagian per project, dan komponen placeholder `MrList.svelte`. Epic ini mengisi daftar Merge Request (GitLab) dan Pull Request (GitHub) per project: pemilih status Terbuka/Digabung/Tertutup, baris lengkap, serta aksi Buka di browser, Salin link, dan Kirim ke agen. Tujuannya agar user melihat MR/PR di dalam app tanpa keluar, lalu membagikan linknya ke agen sebagai konteks. Semua read-only dan semua data lewat `glab`/`gh` yang sudah login.

## Hasil yang bisa dijalankan
Di atas tiket 1, setelah tiket ini selesai:
- Di panel GitLab (dan GitHub bila ada), bagian per project menampilkan daftar MR/PR sungguhan (bukan placeholder), dimuat saat bagian dibuka. Bagian terlipat tidak memuat.
- Pemilih status **Terbuka | Digabung | Tertutup** (default Terbuka); pilihan per bagian disimpan di store tiket 1 sehingga bertahan saat panel di-unmount.
- Tiap baris: judul, nomor (`!12` GitLab, `#12` GitHub), penulis, branch sumber ke target, penanda draft, status review dan badge pipeline hanya bila daftar CLI memuatnya, waktu pembaruan relatif, dan tombol Buka di browser, Salin link, Kirim ke agen.
- Keadaan memuat, kosong ("Tidak ada MR terbuka" / "Tidak ada PR terbuka", menyesuaikan provider dan status), dan galat terbaca dengan tombol coba lagi (galat satu project tidak memengaruhi project lain).
- Tombol Segarkan panel memuat ulang daftar bagian yang terbuka.
- Klik baris membuka pane `git-mr` (placeholder dari tiket 1) lewat API pane yang sudah ada, dengan field leaf `provider`, `projectId`, `number`.
- Kirim ke agen menempel URL ke sesi aktif tanpa Enter; tanpa sesi aktif muncul toast "Tidak ada sesi aktif".
- `shikigamictl git mrs <workspaceId> <projectId> <state>` berjalan sebagai alat debug.
- `go build ./...` dan build frontend lulus; aplikasi berjalan. GitLab terbukti dengan data nyata `glab`; GitHub terbukti lewat fixture (verifikasi di app menunggu `gh auth login`, dicatat sebagai butir terbuka).

## Batasan
In scope:
- `ListMergeRequests` di `internal/gitdata/mrs.go` untuk GitLab dan GitHub, normalisasi ke `MergeRequest`.
- IPC `git.mrs.list`.
- Subcommand `shikigamictl git mrs`.
- UI: `MrList.svelte` (ganti placeholder), `MrRow.svelte`, `StatusBadge.svelte`, `mrClient.ts`.
- Persist pilihan status lewat store tiket 1 (hanya memakai API store yang ada).
- Membuka pane `git-mr` lewat API pane yang ada.

Out of scope:
- Detail MR/PR, isi `MrPane.svelte` (tiket 4, `git-mr-detail`); daftar pipeline dan `PipelineList.svelte` (tiket 3).
- Mengubah file tiket 1: `types.go`, `allowlist.go`, `runner.go`, `errors.go`, `repo.go`, `testdata/`, `docs/git/cli-findings.md`, `wire_git.go`, `cmd_git.go`, `store.ts`, `client.ts`, `actions.ts`, `GitPanel.svelte`, `ProjectSection.svelte`, `index.ts`, `layout/model.ts`. Bila kebutuhan (perintah belum di allowlist, tipe kurang, field store kurang) tidak terpenuhi, laporkan; jangan mengubah file itu.
- Filter milik saya/reviewer/label, pencarian, paginasi tak terbatas, polling, cache disk.
- Membuat/menyetujui/menggabung/menutup MR/PR; komentar; diff.
- Panggilan CLI per baris (tidak ada).
- Unit test dan dependensi baru.

## Kontrak
Disediakan:

IPC `git.mrs.list` params `{workspaceId: string, projectId: string, state: "open" | "merged" | "closed"}` hasil (json camelCase, tipe `MergeRequest` milik tiket 1 di `internal/gitdata/types.go`):
```ts
{
  mrs: {
    number: number;
    title: string;
    author: string;
    sourceBranch: string;
    targetBranch: string;
    state: "open" | "merged" | "closed";
    draft: boolean;
    reviewState?: string;     // hanya bila daftar CLI memuatnya (bentuk nilai mengikuti docs/git/cli-findings.md)
    pipelineStatus?: string;  // pending|running|success|failed|canceled|skipped|unknown; hanya bila daftar CLI memuatnya
    updatedAt: string;        // ISO-8601
    url: string;
  }[];
}
```
Nama bungkus hasil (`mrs`) dan bentuk persis field opsional mengikuti `docs/git/cli-findings.md`; bila spike tiket 1 menunjukkan `glab mr list` tidak memuat review/pipeline, field itu dikosongkan (dihilangkan di JSON) dan baris tidak menampilkan kolom itu. Galat IPC memakai bentuk galat connector/Lark yang sama (kode + pesan Indonesia).

Go (`internal/gitdata/mrs.go`):
- `ListMergeRequests(ctx context.Context, repo Repo, state string) ([]MergeRequest, error)`; GitLab: `glab mr list -F json -R <repo>` dengan flag filter state (open default; `--merged`, `--closed` atau `--all` + penyaringan menurut temuan spike), GitHub: `gh pr list --json <field> -R <repo> --state open|merged|closed [--limit N]`.
- `-R` berbentuk `OWNER/REPO` (GitLab: `GROUP/SUBGROUP/REPO`) untuk host bawaan dan `HOST/PATH` untuk host self-hosted (helper dari tiket 1 bila ada; periksa nama persisnya di `repo.go`).
- Parser dibangun dari fixture `internal/gitdata/testdata/`; status pipeline dinormalisasi lewat konstanta/fungsi status tiket 1 (pemetaan GitHub `queued/in_progress/completed`+conclusion ke himpunan sama). Tanpa panggilan per baris. Batas jumlah baris (usul 50) bila CLI mendukung `--per-page`/`--limit`.
- Semua panggilan lewat `Run()`/runner tiket 1 (timeout 20 detik, maks 3 bersamaan, allowlist). `gitdata` tidak mengimpor `internal/workspace`; arah dependensi `gitdata -> connector`.

`cmd/shikigamid/wire_git_mrs.go`: mendaftarkan `git.mrs.list` dengan pola `wire_lark*.go` (registrasi mandiri). Memvalidasi params, mencari project di workspace dan repo-nya (memakai hasil deteksi repo tiket 1), memanggil `gitdata.ListMergeRequests`, mengubah galat ke bentuk galat Lark (CLI belum terpasang, belum login, izin kurang, repo tidak ditemukan, timeout, "Project tidak ditemukan di workspace ini").

Frontend (`frontend/src/modules/git/`):
- `mrClient.ts`: `listMergeRequests(params: {workspaceId, projectId, state}): Promise<MergeRequest[]>`, pembungkus IPC bertipe; tipe `MergeRequest` TS sama dengan kontrak di atas (diekspor dari file ini).
- `MrList.svelte` props (tidak berubah dari placeholder tiket 1): `{provider, repo}` dengan `repo` memuat `projectId`/`workspaceId` sebagaimana didefinisikan tiket 1 (periksa nama persis di `ProjectSection.svelte` dan `client.ts`). Event segarkan dari panel diikuti lewat mekanisme yang disediakan tiket 1 (store/prop).
- `MrRow.svelte` props: `{ mr: MergeRequest, provider: "gitlab" | "github", onOpen(mr) }`.
- `StatusBadge.svelte` props: `{ status: string, kind?: "pipeline" | "review" | "mr" }` (kecil, hanya menampilkan label Indonesia + warna token tema); komponen ini dipakai ulang oleh tiket 3 dan 4, jadi props tidak boleh dibuat spesifik MR.

Dipakai dari tiket 1 (periksa nama persis di kode): `actions.ts` (`openInBrowser(url)`, `copyLink(url)`, `sendToAgent(url)`), `store.ts` (state bagian terlipat/status terpilih), `client.ts`, `GitPanel`/`ProjectSection`, pane kind `git-mr` dan field leaf persist `provider/projectId/number` di `layout/model.ts`, `types.go`, runner, allowlist, fixture, `docs/git/cli-findings.md`.

Aturan tetap: teks UI bahasa Indonesia, identifier kode bahasa Inggris; tanpa token di config; tanpa polling otomatis (muat saat bagian dibuka, saat status berganti, dan lewat Segarkan); maks 3 panggilan CLI bersamaan (bagian terlipat tidak memuat); tanpa unit test.

## Fitur

### git-mrs-data: Data MR/PR gitdata dan IPC git.mrs.list
Deskripsi:
Baca `docs/git/cli-findings.md` dan fixture `internal/gitdata/testdata/` dari tiket 1 terlebih dulu; parser dibangun dari fixture, bukan tebakan.
- `internal/gitdata/mrs.go`: `ListMergeRequests` untuk dua provider. Petakan field CLI ke `MergeRequest`: Number (GitLab `iid`, GitHub `number`), Title, Author (username/login), SourceBranch/TargetBranch, State ternormalisasi (`opened`->`open`, `merged`, `closed`; GitHub `OPEN/MERGED/CLOSED` huruf kecil), Draft, ReviewState (GitHub `reviewDecision` bila ada; GitLab hanya bila daftar memuat approval), PipelineStatus (GitHub dari `statusCheckRollup` bila disertakan di `--json`, GitLab dari `head_pipeline`/`pipeline` bila ada) lewat normalisasi status tiket 1, UpdatedAt, URL (`web_url`/`url`). Bila field opsional tidak ada di keluaran daftar, biarkan kosong; jangan memanggil CLI per baris.
- Pemetaan filter state ke flag CLI sesuai temuan spike; state tidak dikenal -> galat validasi.
- Urutan hasil: terbaru diperbarui lebih dulu (urutkan di Go bila CLI tidak menjamin).
- `cmd/shikigamid/wire_git_mrs.go`: daftarkan `git.mrs.list` mengikuti pola `wire_lark*.go`; cari project dan repo (hasil deteksi tiket 1), panggil gitdata, ubah galat ke bentuk galat Lark dengan pesan Indonesia. Galat satu project tidak memengaruhi project lain (tiap panggilan independen, tanpa state bersama yang rusak).
- Tanpa unit test; guard hanya build.

Acceptance criteria:
- [ ] `git.mrs.list` untuk project GitLab nyata mengembalikan daftar `mrs` ternormalisasi untuk state `open`, `merged`, dan `closed`.
- [ ] Untuk GitHub, parser menghasilkan `MergeRequest` benar dari fixture di `testdata/` (state, draft, nomor, URL, branch).
- [ ] `state` hasil hanya `open|merged|closed`; `pipelineStatus` hanya anggota himpunan status tiket 1 atau kosong.
- [ ] Field `reviewState`/`pipelineStatus` terisi hanya bila daftar CLI memuatnya; tidak ada panggilan CLI per baris.
- [ ] Setiap panggilan lewat runner tiket 1 (allowlist, timeout 20 detik, maks 3 bersamaan); tidak ada perintah tulis.
- [ ] Galat (CLI hilang, belum login, izin kurang, repo tidak ditemukan, timeout, project tidak ada) menjadi galat IPC berpesan Indonesia, tanpa panic.
- [ ] `gitdata` tidak mengimpor `internal/workspace`; `go build ./...` lulus.

ownedPaths:
- `internal/gitdata/mrs.go`
- `cmd/shikigamid/wire_git_mrs.go`

Catatan: bisa dikerjakan paralel dengan `git-mrs-ctl` dan `git-mrs-ui` (kontrak IPC di atas adalah antarmuka bersama; frontend dan ctl memakai bentuk yang sama tanpa menunggu backend). Registrasi mandiri mengikuti pola `wire_lark*.go`; jangan mengubah file di luar ownedPaths, laporkan bila pola itu ternyata butuh perubahan di file lain.

### git-mrs-ctl: shikigamictl git mrs
Deskripsi:
Subcommand `git mrs <workspaceId> <projectId> <state>` di `cmd/shikigamictl/cmd_git_mrs.go` sebagai alat debug. Memanggil IPC `git.mrs.list` ke daemon dan mencetak ringkas satu baris per MR/PR: nomor (`!n`/`#n` menurut provider bila diketahui, kalau tidak `n`), state, penanda draft, judul, penulis, `sourceBranch->targetBranch`, `pipelineStatus`/`reviewState` bila ada, dan URL; diakhiri jumlah baris. State tidak valid atau argumen kurang -> pesan penggunaan. Registrasi mandiri mengikuti pola `cmd_lark_tickets.go` (cek cara induk perintah `git` didaftarkan di `cmd_git.go` tiket 1 dan tambahkan subcommand dari file ini tanpa mengubah `cmd_git.go`).

Acceptance criteria:
- [ ] `shikigamictl git mrs <workspaceId> <projectId> open` mencetak daftar MR terbaca terhadap daemon yang berjalan.
- [ ] State `merged` dan `closed` diterima; state lain ditolak dengan pesan penggunaan.
- [ ] Galat daemon dicetak sebagai pesan terbaca dengan kode keluar bukan nol, tanpa panic.
- [ ] Terdaftar mandiri tanpa mengubah `cmd_git.go`; `go build ./...` lulus.

ownedPaths:
- `cmd/shikigamictl/cmd_git_mrs.go`

Catatan: bisa dikerjakan paralel dengan `git-mrs-data` dan `git-mrs-ui`.

### git-mrs-ui: UI daftar MR/PR di panel
Deskripsi:
Ganti isi placeholder `MrList.svelte` (kepemilikan berpindah dari tiket 1 ke epic ini; props tidak berubah) dan buat komponen pendukung.
- `mrClient.ts`: pembungkus IPC bertipe `git.mrs.list` dan tipe `MergeRequest` TS (lihat Kontrak). Memakai pemanggil IPC yang sama dengan `client.ts` tiket 1 (impor helper-nya, jangan menyalin).
- `StatusBadge.svelte`: badge kecil untuk status. Pemetaan label Indonesia: pipeline `pending` "Menunggu", `running` "Berjalan", `success` "Berhasil", `failed` "Gagal", `canceled` "Dibatalkan", `skipped` "Dilewati", `unknown` "Tidak diketahui"; review "Disetujui", "Perlu perubahan", "Belum direview"; MR `open` "Terbuka", `merged` "Digabung", `closed` "Tertutup". Warna lewat variabel CSS tema yang ada (terang/gelap), tidak hanya warna (ikon/teks juga).
- `MrRow.svelte`: judul (dipotong dengan ellipsis, `title` penuh di tooltip), nomor (`!12` GitLab, `#12` GitHub), penulis, `sourceBranch -> targetBranch`, penanda "Draft" bila draft, `StatusBadge` review dan pipeline hanya bila ada di data, waktu pembaruan relatif ("3 jam lalu", pakai util waktu relatif yang sudah ada di app bila ada, kalau tidak fungsi kecil lokal), tombol ikon Buka di browser / Salin link / Kirim ke agen memakai `openInBrowser`/`copyLink`/`sendToAgent` dari `actions.ts` tiket 1 (klik tombol tidak ikut membuka pane). Klik baris memanggil `onOpen(mr)`; dapat diakses keyboard (Enter/Spasi).
- `MrList.svelte`: pemilih status segmented Terbuka | Digabung | Tertutup (default Terbuka; pilihan dibaca/ditulis lewat store tiket 1, kunci per project/bagian sesuai yang disediakan store; bila store tidak punya tempat untuk status, laporkan dan pakai state lokal sebagai cadangan sementara). Muat saat komponen tampil (bagian dibuka) dan saat status berganti; ikut tombol Segarkan panel (mekanisme dari tiket 1). Keadaan: memuat, kosong ("Tidak ada MR terbuka"/"Tidak ada MR digabung"/"Tidak ada MR tertutup" untuk GitLab, "PR" untuk GitHub), galat terbaca dengan tombol "Coba lagi" (termasuk ajakan ke Pengaturan > Connector untuk kode CLI belum terpasang/belum login, memakai pola pesan galat tiket 1). Respons usang (setelah status berganti atau segarkan) diabaikan. Hormati batas 3 panggilan bersamaan: komponen hanya memanggil saat tampil, bagian terlipat tidak dimuat; antrean/semaphore diurus runner backend.
- Klik baris membuka pane `git-mr` dengan API pane yang sudah ada (mis. fungsi buka pane di modul layout yang dipakai Lark untuk membuka `lark-board`; cek polanya di `modules/lark/`) dengan leaf `{kind: 'git-mr', provider, projectId, number}`. Pane placeholder tiket 1 yang tampil; isinya diisi tiket 4.
- Style mengikuti variabel CSS yang ada; tidak ada dependensi baru.

Acceptance criteria:
- [ ] Daftar MR/PR memuat dan tampil untuk status Terbuka secara default; berganti ke Digabung/Tertutup memuat ulang dan pilihan bertahan setelah panel di-unmount lalu dibuka lagi.
- [ ] Baris menampilkan judul, nomor (`!` GitLab, `#` GitHub), penulis, branch sumber ke target, penanda draft, waktu relatif; review dan badge pipeline muncul hanya bila datanya ada.
- [ ] Buka di browser, Salin link, Kirim ke agen bekerja per baris; tanpa sesi aktif muncul toast "Tidak ada sesi aktif"; klik tombol tidak membuka pane.
- [ ] Klik baris membuka pane `git-mr` dengan `provider/projectId/number` benar dan dipersist.
- [ ] Keadaan memuat, kosong (teks sesuai provider dan status), dan galat dengan coba lagi tampil; galat satu bagian tidak memengaruhi bagian lain.
- [ ] Segarkan panel memuat ulang daftar; respons usang tidak menimpa data baru; bagian terlipat tidak memanggil IPC.
- [ ] `StatusBadge` tidak spesifik MR (dapat dipakai pipeline/review) dan terbaca di tema terang dan gelap.
- [ ] Build frontend lulus.

ownedPaths:
- `frontend/src/modules/git/MrList.svelte`
- `frontend/src/modules/git/MrRow.svelte`
- `frontend/src/modules/git/StatusBadge.svelte`
- `frontend/src/modules/git/mrClient.ts`

Catatan: bisa dikerjakan paralel dengan `git-mrs-data` dan `git-mrs-ctl` (memakai tipe kontrak IPC di atas). Verifikasi dengan data nyata menunggu `git-mrs-data` dan dikerjakan di `git-mrs-wiring`.

### git-mrs-wiring: Wiring dan verifikasi daftar MR/PR
Deskripsi:
Fitur penutup epic; berjalan setelah `git-mrs-data`, `git-mrs-ctl`, dan `git-mrs-ui` selesai. Tidak punya file sendiri; hanya memverifikasi dan menyambungkan bila ada kekurangan kecil.
- Pastikan `wire_git_mrs.go` terdaftar mandiri di daemon (IPC `git.mrs.list` tampil dan terjawab); bila registrasi butuh baris di file di luar ownedPaths, ikuti pola mandiri `wire_lark*.go` dan laporkan, jangan menyentuh file tiket lain.
- Pastikan `MrList.svelte` sudah dipakai oleh `ProjectSection` (titik ekstensi tiket 1) sehingga daftar muncul di panel GitLab/GitHub tanpa perubahan di file tiket 1.
- Verifikasi di app: panel GitLab menampilkan MR nyata dari `glab` untuk project dengan remote gitlab.com; ganti status; salin dan kirim link; klik baris membuka pane `git-mr` placeholder; pane dipulihkan setelah restart; CLI/izin galat tampil aman. Jalur GitHub dibuktikan lewat fixture; catat butir terbuka "verifikasi daftar PR di app menunggu `gh auth login`" di laporan akhir (tidak memblokir selesai).
- `go build ./...` dan build frontend lulus; aplikasi berjalan.

Acceptance criteria:
- [ ] `git.mrs.list` terdaftar dan terjawab oleh daemon yang berjalan; `shikigamictl git mrs ...` mencetak hasil.
- [ ] Daftar MR muncul di panel GitLab dengan data nyata `glab`, pemilih status berfungsi, aksi baris berfungsi.
- [ ] Klik baris membuka pane `git-mr`; pane dipersist dan dipulihkan.
- [ ] GitHub terbukti lewat fixture; butir terbuka verifikasi GitHub dicatat sampai `gh auth login`.
- [ ] `go build ./...` dan build frontend lulus; aplikasi berjalan.

ownedPaths: (tidak ada; hanya verifikasi dan pelaporan)

Catatan: tidak paralel; jalan terakhir setelah tiga fitur lain di epic ini selesai.

## Path yang dimiliki
Hanya path ini yang boleh disentuh:
- `internal/gitdata/mrs.go`
- `cmd/shikigamid/wire_git_mrs.go`
- `cmd/shikigamictl/cmd_git_mrs.go`
- `frontend/src/modules/git/MrList.svelte`
- `frontend/src/modules/git/MrRow.svelte`
- `frontend/src/modules/git/StatusBadge.svelte`
- `frontend/src/modules/git/mrClient.ts`

Epic lain (jangan disentuh): tiket 1 (`connector/gitlab.go`, `connector/github.go`, `gitdata/doc.go|types.go|allowlist.go|runner.go|errors.go|repo.go|testdata/`, `docs/git/cli-findings.md`, `wire_git.go`, `cmd_git.go`, `modules/git/index.ts|store.ts|client.ts|actions.ts|GitIcon.svelte|GitPanel.svelte|ProjectSection.svelte|PipelineList.svelte|MrPane.svelte|PipelinePane.svelte`, `modules/layout/model.ts`), tiket 3 (`gitdata/pipelines.go`, `wire_git_pipelines.go`, `cmd_git_pipelines.go`, `PipelineList/PipelineRow/PipelinePane/StageList/pipelineClient`), tiket 4 (`gitdata/mrdetail.go`, `wire_git_mr_detail.go`, `cmd_git_mr_detail.go`, `MrPane/ReviewerList/mrDetailClient`).

## Wiring
Penutup epic ini adalah fitur `git-mrs-wiring`. Tiket ini tidak butuh titik sambung baru: panel Git, `ProjectSection`, `MrList.svelte` (placeholder dengan props `{provider, repo}`), jenis pane `git-mr` beserta field persist `provider/projectId/number`, `actions.ts`, dan store sudah disediakan tiket 1. Wiring menyambungkan: (1) `MrList.svelte` yang sudah berisi ke `ProjectSection` lewat titik ekstensi yang ada (tanpa mengubah file tiket 1); (2) IPC `git.mrs.list` ke daemon lewat `wire_git_mrs.go` yang mendaftar mandiri; (3) `shikigamictl git mrs` ke daemon lewat `cmd_git_mrs.go` yang mendaftar mandiri; (4) klik baris ke pane `git-mr`. Hasilnya terlihat dan jalan di aplikasi: buka panel GitLab -> bagian project -> daftar MR -> ganti status -> salin/kirim link -> klik baris membuka pane `git-mr`. Tiket ini hanya bergantung pada tiket 1 yang sudah selesai dan tidak bergantung pada tiket 3 atau 4.

## Guard
Cukup build berhasil; tanpa unit test:
- `go build ./...`
- Build frontend (`npm run build` di `frontend/`, atau perintah build/check yang dipakai tiket sebelumnya)
