# Plan: Spike lark-cli dan fixture

## Metadata
- Tiket: `lark-connector-ticket-01-lark-connector` (`docs/tickets/lark-connector-ticket-01-lark-connector.md`)
- Fitur: `lark-cli-spike` (Spike lark-cli dan fixture)
- ownedPaths: `internal/larkdata/testdata/`, `docs/lark/`
- Depends on foundation: tidak (epic ini tidak punya foundation; lihat overview bagian 1 dan 4)
- Rujukan wajib dibaca: `docs/plans/lark-connector-ticket-01-lark-connector/overview.md` (jangan diubah), tiket di atas, dan spec `docs/superpowers/specs/2026-11-22-09-lark-connector-design.md`.

## Tujuan
Menjalankan `lark-cli` nyata (read-only, sudah login, versi 1.0.97 saat plan ditulis) lalu merekam output JSON tiap perintah baca yang dipakai epic Lark, menganonimkannya tanpa mengubah struktur atau tipe, dan menuliskan temuan sebagai kontrak data untuk tiket 02 sampai 04 serta untuk penyelarasan `larkdata-runner` dan `lark-connector-adapter` saat Wiring. Fitur ini hanya menghasilkan berkas data dan dokumen; tidak ada perubahan Go atau frontend.

## Path yang dimiliki
HANYA path berikut yang boleh dibuat atau diubah:
- `internal/larkdata/testdata/` (folder belum ada; buat)
- `docs/lark/` (folder belum ada; buat)

Dilarang menyentuh: `internal/larkdata/runner.go`, `allowlist.go`, `errors.go` (milik `larkdata-runner`), `internal/connector/*` (milik adapter dan runner), `cmd/shikigamictl/*`, `cmd/shikigamid/*`, `frontend/*`, `app/*`, `internal/provider/*`, `internal/workspace/*`, file wiring apa pun, dan `docs/plans/.../overview.md`. Jangan `git add`/commit perubahan di luar ownedPaths. Berkas sementara (output mentah sebelum anonimisasi, skrip anonimisasi) HARUS ditaruh di scratchpad sesi atau folder temp di luar repo, BUKAN di dalam repo, agar data asli tidak pernah masuk working tree.

## Kontrak
Disediakan (nama berkas persis, dirujuk overview bagian 2.6):
- `internal/larkdata/testdata/auth-status.json`, `base-table-list.json`, `base-view-list.json`, `base-record-list.json`, `base-record-get.json`, `wiki-space-list.json`, `wiki-node-list.json`, `wiki-node-get.json`, `wiki-node-get-bitable.json`, `wiki-node-list-my-library.json`, `docs-fetch.json`, `sheets-cells-get.json`, `README.md`.
- Fixture tambahan yang boleh ditambah bila berguna (nama berbasis perintah, juga dicatat di README): `base-record-list-page2.json` (halaman kedua/offset), `wiki-node-get-sheet.json`, `wiki-node-get-my-library.json`, `error-*.json` (stderr/stdout galat teranonimkan, mis. `error-node-not-found.json`).
- `docs/lark/cli-findings.md` (Indonesia): kontrak data untuk tiket 02 sampai 04. Konsumen: agent `larkdata-runner` (tabel allowlist `format []string`, `asUser bool`, pola galat), `lark-connector-adapter` (skema `auth status`), dan tiket 02 sampai 04 (parser).

Dipakai: hanya `lark-cli` nyata (binary di PATH, sudah login), `jq`, dan skill `lark-*` terpasang (`lark-shared`, `lark-base`, `lark-wiki`, `lark-doc`, `lark-sheets`) sebagai referensi flag. Tidak memakai kode Go apa pun. Nilai awal yang harus DIVERIFIKASI (bukan diasumsikan): `--format json`, `--as user`, skema `identities.user.{status,available,userName}`, flag `+view-list` dan `+cells-get` (belum terbukti di spec).

## Langkah

### 0. Aturan keselamatan (berlaku di semua langkah)
- Read-only total. Dilarang menjalankan perintah tulis `lark-cli`: kirim pesan, buat/ubah/hapus dokumen, base, record, wiki, `auth login`/`logout`/`refresh`, `update`, `config`. Hanya perintah `list`/`get`/`fetch`/`status`.
- Jangan pernah menyalin token, `scope` panjang, `appId`, `openId`, email, nama user/tenant asli, judul dokumen asli, atau isi record/dokumen asli ke berkas di repo, ke `docs/lark/cli-findings.md`, ataupun ke pesan galat yang dikutip.
- Perhatikan `_notice` (info update CLI) di output: jangan jalankan `lark-cli update`; boleh dihapus dari fixture atau dibiarkan jika murni teks versi (tidak sensitif), tetapi catat keberadaannya di findings karena parser harus mengabaikannya.
- Shell pengembang adalah zsh: `$var` berisi spasi tidak dipecah. Panggil `lark-cli base +view-list --help` langsung (bukan lewat variabel string), atau jalankan lewat `bash -c`.

### 1. Siapkan ruang kerja sementara
1. Buat folder kerja di scratchpad (di luar repo), mis. `$SCRATCH/lark-spike/raw/` untuk output mentah dan `$SCRATCH/lark-spike/anon.sh` (atau `anon.jq`) untuk skrip anonimisasi. Seluruh langkah perekaman menulis ke `raw/` dulu.
2. Catat versi: `lark-cli --version` (harapan: `lark-cli version 1.0.97`). Masuk ke findings bagian "Versi".
3. Pelajari flag lewat `--help`: untuk tiap perintah di langkah 2 sampai 7 jalankan `lark-cli <domain> <perintah> --help`; juga `lark-cli base --help`, `lark-cli wiki --help`, `lark-cli docs --help`, `lark-cli sheets --help`, `lark-cli auth status --help`. Simpan ringkasan flag (wajib/opsional, default, batas) untuk tabel perintah. Cek apakah ada flag global format (`--format`, `--json`, `-o`), `--as`, `--page-size`/`--page-token`.

### 2. auth status
1. `lark-cli auth status > raw/auth-status.json` (tambahkan flag format hanya jika `--help` menunjukkan perlu). Verifikasi: `identities.user.{status,available,userName}` ada; catat juga field lain (`appId`, `brand`, `defaultAs`, `identities.bot`, `identities.user.openId/tokenStatus/scope`) sebagai sensitif.
2. Catat nilai `status` yang mungkin (`ready`, `needs_refresh`, dll. dari `--help`/skill `lark-shared`) dan `tokenStatus`.
3. Belum login: JANGAN logout. Bila ada cara aman (mis. profil/config kosong lewat env `HOME`/`--profile` sementara ke folder kosong di scratchpad yang tidak menyentuh login asli) coba rekam bentuknya, simpan sebagai `auth-status-not-logged-in.json`; bila tidak aman atau tidak pasti, cukup catat di findings dari `--help`/skill dan tandai "tidak direkam".
4. Catat exit code (`echo $?`) untuk keadaan login normal; bila belum-login dapat diamati, catat juga exit code-nya (menentukan apakah adapter memperlakukan exit non-nol sebagai `not_logged_in`).

### 3. Base: table, view, record
1. Pilih satu Base nyata milik user yang bisa dibaca (minta dari riwayat/URL yang ada di konteks atau dari `lark-cli` yang memberi daftar; bila tidak ada Base yang bisa dijangkau, catat di Pengecualian dan lanjutkan langkah lain). Ambil `base_token` dari URL `.../base/<token>` (host/tenant dicatat di notes sementara saja).
2. `lark-cli base +table-list --base-token <T> --as user [format]` -> `raw/base-table-list.json`. Catat path daftar tabel, `table_id`, nama, dan ada/tidaknya paginasi.
3. `lark-cli base +view-list --base-token <T> --table-id <TB> --as user` -> `raw/base-view-list.json`. Catat flag sebenarnya (hasil `--help`), `view_id`, `view_type` (grid/kanban/dll.), dan apakah view menyertakan filter/grup.
4. `lark-cli base +record-list --base-token <T> --table-id <TB> --limit <kecil, mis. 3> --as user` -> `raw/base-record-list.json`; ulangi dengan `--view-id <V>` dan dengan `--offset` (halaman kedua) -> `raw/base-record-list-page2.json`. Catat: bentuk respons (daftar field di kolom paralel vs objek per record), `has_more`, kursor/`offset`/`page_token`, batas maksimum `--limit` (uji `--limit` besar secukupnya, mis. 200 dan 500, hanya baca), urutan field (nama field vs `field_id`), `record_id`.
5. `lark-cli base +record-get --base-token <T> --table-id <TB> --record-id <R> --as user` -> `raw/base-record-get.json`. Bandingkan bentuknya dengan `+record-list` (sama atau beda).
6. Tipe field: dari `+record-list`/`+record-get` (dan, bila ada, perintah `base +field-list` yang bersifat baca; periksa `lark-cli base --help` dan skill `lark-base`) petakan SETIAP tipe field yang muncul: single-select, multi-select, user, link/URL, lampiran, tanggal, angka, checkbox, teks, formula, lookup, relasi, otomatis (nomor/created), dll. Untuk tiap tipe catat: nama tipe di JSON dan bentuk nilai (string, array of objek `{id,name}`, epoch ms, dst.) serta rekomendasi teks tampilan. Bila tidak ada tabel dengan semua tipe, catat tipe yang tidak teramati sebagai "tidak teramati".
7. Field utama (primary): cari penanda di output (mis. `is_primary`, `primary: true`, field pertama pada urutan) di `+table-list`/field-list/record. Catat cara pastinya; bila tidak ada penanda eksplisit, catat aturan turunan (mis. field pertama) dan tandai "tidak eksplisit".
8. Kanban: tentukan apakah ada tipe single-select yang dapat dikelompokkan (nama tipe, bentuk nilai, cara membedakan kolom "Tanpa nilai": null/array kosong/field hilang).
9. URL record: periksa apakah output CLI memuat URL (mis. field `url`/`link` pada table/view/record). Bila tidak, bandingkan dengan pola URL Lark yang diketahui (`https://<host>/base/<token>?table=<tbl>&view=<view>&record=<rec>`) berdasarkan dokumentasi/skill/URL nyata yang ada di konteks. Putuskan dan tulis eksplisit "pasti" atau "tidak pasti" beserta bukti (kutipan output atau sumber dokumentasi, tanpa token asli). Jangan menebak: bila tidak ada bukti, nyatakan "tidak pasti" (konsekuensi: tombol Salin link tingkat tiket dinonaktifkan, sesuai spec putaran 2).

### 4. Wiki
1. `lark-cli wiki +space-list --as user [format]` -> `raw/wiki-space-list.json`. Catat `space_id`, nama, `space_type`, paginasi.
2. `lark-cli wiki +node-list --space-id <S> --page-size 50 --as user` (akar space) -> `raw/wiki-node-list.json`; ulangi dengan `--parent-node-token <N>` pada node yang punya anak (catat bila perlu sebagai `wiki-node-list-children.json`). Catat: `node_token`, `obj_type`, `obj_token`, `title`, `has_child`, `parent_node_token`, `node_type` (origin/shortcut), kursor `page_token`/`has_more`.
3. `my_library`: `lark-cli wiki +node-list --space-id my_library --page-size 50 --as user` -> `raw/wiki-node-list-my-library.json`. Catat dengan jelas apakah `my_library` dapat di-list (diterima atau galat), dan apakah `+space-list` memuatnya. Bila gagal, simpan galatnya sebagai `error-my-library.json` dan catat keputusan potong (wiki pribadi disembunyikan dengan keterangan). Catat juga apakah `+node-get` pada `my_library` (mis. `--space-id my_library` atau `--token`) bekerja.
4. `lark-cli wiki +node-get --token <node_token> --as user` (periksa flag sebenarnya di `--help`): rekam satu node tiap tipe bila ada:
   - docx -> `raw/wiki-node-get.json`
   - sheet -> `raw/wiki-node-get-sheet.json`
   - bitable -> `raw/wiki-node-get-bitable.json` (verifikasi `obj_type: bitable` dan `obj_token` = token Base yang bisa dipakai di `base +table-list`; ini mendukung link Base di bawah wiki)
   - my_library -> `raw/wiki-node-get-my-library.json` bila bisa.
   Catat: ada tidaknya URL node di output (jika ada: nama field), `has_child`, `obj_type` yang teramati (docx, doc, sheet, bitable, mindnote, file, slides, dll.), dan format URL fallback `https://<host>/wiki/<node_token>`.
5. Tipe yang tidak ditemukan di akun (mis. tidak ada sheet/bitable di wiki) dicatat "tidak teramati", bukan dikarang.

### 5. Dokumen dan sheet
1. `lark-cli docs +fetch --doc <obj_token atau URL> --doc-format markdown --as user` (flag sebenarnya dari `--help`) -> `raw/docs-fetch.json`. Catat path konten markdown, judul, ada tidaknya paginasi/pemotongan, ukuran, dan perilaku terhadap docx di wiki (apakah menerima `obj_token` dari node wiki atau butuh URL wiki).
2. `lark-cli sheets +cells-get ... --as user` (flag `--spreadsheet-token`/`--range`/`--sheet-id` dari `--help`; rentang awal 50 baris x 20 kolom, mis. `A1:T50`) -> `raw/sheets-cells-get.json`. Catat flag nyata, format rentang, bentuk sel (array 2D nilai, tipe sel, nilai kosong/terpotong), cara mendapat daftar sheet bila perlu, dan batas ukuran. Jika perintah ini tidak ada/tidak sesuai, catat di Pengecualian (sheet dipotong ke "judul dan link") dan sertakan alternatif yang ditemukan (mis. `sheets +read`) tanpa menyimpannya sebagai fixture berkas bernama kontrak.

### 6. Penerimaan `--as user` dan format
Untuk tiap perintah (auth status, 4 base, 3 wiki, docs +fetch, sheets +cells-get) catat hasil uji: (a) menerima `--as user` ya/tidak (bila ditolak, simpan pesan galat), (b) flag format json yang berlaku (`--format json`, `--json`, atau default sudah JSON tanpa flag), (c) apakah output default sudah JSON murni di stdout (tanpa teks tambahan; `_notice` hanya di dalam JSON atau di stderr). Hasilnya mengisi kolom tabel yang langsung dipakai `allowlist.go` (`asUser bool`, `format []string`).

### 7. Pola galat
Hasilkan galat dengan cara AMAN (read-only):
- Token node tidak ditemukan: `wiki +node-get` dengan token palsu (mis. `wikcnXXXXXXXXXXXXXXXXXXXXXXX`) -> rekam stdout/stderr dan exit code sebagai `raw/error-node-not-found.json`.
- Izin/scope kurang: coba baca resource yang diketahui tidak dapat diakses (mis. token Base palsu atau milik tenant lain) -> `raw/error-permission.json`; catat kode (mis. `99991672`, `type`/`subtype` di objek `error`), teks pesan, dan exit code. Bila tidak bisa dipicu aman, ambil pola dari `--help`/skill `lark-shared` dan tandai "dari dokumentasi, tidak direkam".
- Belum login: dari langkah 2.3 atau dari dokumentasi.
- Timeout: tidak dipicu; catat bahwa galat timeout berasal dari `connector.RunCLITimeout`, bukan dari CLI.
Catat: apakah galat ditulis ke stdout sebagai JSON (`{"ok":false,"error":{type,subtype,message}}`) atau stderr, exit code non-nol atau nol, dan substring stabil untuk pencocokan klasifikasi (`not logged in`, `permission`, `scope`, `99991672`, `not found`, dsb.) untuk `larkdata-runner`. Penting: jika galat muncul di stdout dengan exit code non-nol, catat bahwa `RunCLI` hanya membungkus stderr; sertakan rekomendasi agar klasifikasi juga membaca stdout atau agar `larkdata-runner` menyesuaikan (masuk ke Pengecualian untuk Wiring, jangan mengubah kode di fitur ini).

### 8. Anonimkan fixture
1. Tulis skrip `jq`/Python di scratchpad (di luar repo) yang membaca `raw/*.json` dan menulis ke `internal/larkdata/testdata/*.json`. Aturan: ganti HANYA nilai, bukan kunci, struktur, urutan, tipe data, atau panjang array. Ganti secara konsisten (pemetaan nilai asli -> palsu yang sama di semua berkas):
   - token Base/tabel/view/record/node/obj/space/spreadsheet -> pola palsu dengan prefix dan panjang sama (`bascnFAKE0000000000000000001`, `tblFAKE0000000001`, `vewFAKE0001`, `recFAKE0000000001`, `wikcnFAKE0000000000000000001`, `7000000000000000001` untuk space_id numerik, dst.), konsisten antarberkas agar hubungan (mis. `obj_token` node bitable = token Base di `base-table-list.json`) tetap terjaga.
   - nama user, email, `open_id`/`user_id`/`union_id` (`ou_...` -> `ou_fake...`), nama tenant/host (host tetap berbentuk domain valid, mis. `example.larksuite.com`), `appId` (`cli_fake...`), scope panjang -> dikosongkan atau disingkat ke beberapa scope umum yang palsu, URL (host dan token diganti).
   - judul node/dokumen/tabel/view/field dan nilai record, isi markdown dokumen, isi sel sheet -> teks palsu netral berbahasa Indonesia atau Inggris ("Tiket Contoh 1", "Dokumen Contoh", "Kolom A"), dengan tipe nilai yang sama (angka tetap angka, tanggal tetap epoch, opsi select tetap objek `{id,name}` dengan `name` palsu, lampiran tetap objek dengan `file_token`/`name`/`size`/`type` palsu).
   - Pertahankan: nama tipe field, penanda primary, `has_more`, `page_token`/`offset` (ganti nilai kursor tetapi pertahankan bentuk), `obj_type`, `has_child`, kode galat (mis. `99991672`).
2. Output tiap berkas: JSON valid, diformat `jq .` (indentasi 2 spasi), diakhiri newline.
3. Verifikasi (wajib, sebelum menulis dokumen):
   - `for f in internal/larkdata/testdata/*.json; do jq . "$f" >/dev/null || echo "INVALID $f"; done` tanpa keluaran.
   - Bandingkan struktur: untuk tiap berkas, `jq '[paths|map(tostring)|join(".")]|unique' raw/X.json` vs berkas hasil harus identik kecuali indeks array yang dikurangi tidak ada (panjang array harus sama).
   - Grep nilai asli yang diketahui: kumpulkan daftar nilai sensitif dari `raw/` (nama user, email, `ou_`, `base_token`, node/obj token, host tenant, judul-judul) ke berkas daftar di scratchpad, lalu `grep -rFf <daftar> internal/larkdata/testdata docs/lark` HARUS kosong. Tambahan: `grep -rEi 'ou_[a-z0-9]{20,}|u-[A-Za-z0-9]{20,}|t-[A-Za-z0-9]{20,}|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}' internal/larkdata/testdata docs/lark` (kecuali email palsu `@example.com`) harus kosong, dan tidak ada token akses/refresh (`access_token`, `refresh_token`, `Bearer`).
4. Hapus `raw/` dari scratchpad setelah verifikasi lulus (opsional tetapi disarankan); jangan pernah menyalin ke repo.

### 9. Tulis `internal/larkdata/testdata/README.md`
Bahasa Indonesia. Isi:
- Tujuan folder: fixture hasil rekaman `lark-cli` nyata yang telah dianonimkan, acuan parser tiket 02 sampai 04.
- Versi `lark-cli` saat rekam dan tanggal.
- Tabel: nama berkas | perintah persis yang menghasilkannya (dengan placeholder `<BASE_TOKEN>`, `<TABLE_ID>`, `<NODE_TOKEN>` dll., tanpa token nyata, tanpa `--profile`) | catatan (mis. halaman kedua, node tipe apa).
- Aturan anonimisasi (apa yang diganti, apa yang dipertahankan; pemetaan placeholder palsu yang dipakai agar hubungan antarberkas jelas, mis. `obj_token` node bitable sama dengan token di `base-table-list.json`).
- Peringatan: jangan commit output mentah; cara menambah fixture baru (rekam, anonimkan, verifikasi `jq .` dan grep).
- Fixture yang tidak dapat direkam (jika ada) dan alasannya, dengan rujukan ke bagian Pengecualian di findings.

### 10. Tulis `docs/lark/cli-findings.md`
Bahasa Indonesia, tanpa data sensitif. Bagian wajib berurutan:
1. **Versi dan lingkungan**: `lark-cli --version`, tanggal spike, catatan `_notice` update, brand (lark/feishu) tanpa host tenant asli.
2. **Tabel perintah**: kolom `Perintah | Flag wajib | Flag opsional penting | Format JSON | --as user | Fixture`. Baris: `auth status`, `base +table-list`, `base +view-list`, `base +record-list`, `base +record-get`, `wiki +space-list`, `wiki +node-list`, `wiki +node-get`, `docs +fetch`, `sheets +cells-get`. Kolom "Format JSON" dan "--as user" diisi dari uji nyata langkah 6 (ya/tidak/default), bukan asumsi. Tambahkan bagian "Rekomendasi tabel allowlist" berisi blok kecil per perintah: `domain`, `command`, `asUser`, `format` (nilai literal yang harus dipakai `allowlist.go`).
3. **Skema JSON ringkas per perintah**: path field penting (mis. `data.items[].table_id`) yang dipakai tiket 02 sampai 04, dengan contoh kunci (nilai fiktif dari fixture) dan nullability. Mencakup `auth status` (konfirmasi atau koreksi `identities.user.{status,available,userName}`).
4. **Tipe field dan bentuk nilai**: tabel `Tipe (nama di JSON) | Bentuk nilai | Teks tampilan | Dipakai kanban?`, termasuk single-select, multi-select, user, link, lampiran, tanggal, angka, checkbox, teks, formula/lookup/relasi, otomatis. Tipe tak teramati ditandai demikian.
5. **Field utama (primary)**: cara penandaannya (eksplisit/turunan).
6. **Paginasi**: `+record-list` (`has_more`, `offset`/`page_token`, batas `--limit`, perilaku saat melebihi), `+node-list`, `+space-list`; contoh dari `base-record-list-page2.json`.
7. **Wiki**: bentuk node (`node_token`, `obj_type`, `obj_token`, `has_child`, `parent_node_token`), daftar `obj_type` teramati, URL node (ada di output atau fallback `https://<host>/wiki/<node_token>`), `my_library` (list/get berhasil atau tidak), link Base di bawah wiki (`obj_type: bitable` -> `obj_token`), `docs +fetch` pada docx wiki, `sheets +cells-get` pada sheet wiki.
8. **URL record**: status eksplisit **pasti** atau **tidak pasti**, bukti, dan format yang dianjurkan (atau fallback ke link tabel/view).
9. **Pola galat**: tabel `Situasi | Saluran (stdout/stderr) | Exit code | Pola substring stabil | Kode larkdata yang disarankan` untuk belum login, izin/scope kurang, token tidak ditemukan, CLI tidak ada (dari `IsCLIMissing`), timeout (dari `RunCLITimeout`). Sertakan rekomendasi tegas bila klasifikasi perlu membaca stdout.
10. **Pengecualian dan pemotongan fitur**: pernyataan eksplisit AKTIF atau DIPOTONG beserta alasan dan bukti untuk tiap asumsi:
    - Kanban (ada single-select atau tidak; bila tidak: kanban dinonaktifkan, grid tetap).
    - Wiki pribadi `my_library` (dapat di-list atau tidak; bila tidak: disembunyikan dengan keterangan).
    - Link tiket (format record pasti atau tidak; bila tidak pasti: tombol tingkat tiket dinonaktifkan, tingkat Base/view tetap aktif).
    - Perintah tanpa `--as user` (daftar, tandai `asUser:false`).
    - `+view-list` dan `+cells-get` (sesuai spec atau tidak, flag nyata; bila tidak sesuai: entri allowlist diganti/dihapus dan fitur turunannya dipotong).
    - Deviasi lain dari spec (mis. flag format berbeda, galat di stdout) dan dampaknya pada `larkdata-runner`/`lark-connector-adapter` untuk disinkronkan di Wiring.
    Setiap butir memuat keputusan satu kalimat: "AKTIF" atau "DIPOTONG: <dampak>" sesuai aturan pemotongan (pekerjaan lain jalan terus).
11. **Ringkasan untuk Wiring**: daftar singkat perubahan yang disarankan pada `allowlist.go`, `errors.go`, dan `lark.go` (tanpa melakukannya di fitur ini).

### 11. Verifikasi akhir
- Jalankan ulang semua pemeriksaan langkah 8.3 terhadap hasil akhir termasuk `docs/lark/cli-findings.md` dan `README.md`.
- Pastikan semua 12 berkas nama kontrak ada di `internal/larkdata/testdata/`. Bila satu tidak dapat direkam, buat berkas tidak ditulis; jelaskan di README dan Pengecualian, jangan membuat fixture karangan yang tidak berasal dari output nyata.
- `git status --short` hanya boleh menambah path di dalam `internal/larkdata/testdata/` dan `docs/lark/` (perubahan lain di working tree sudah ada sebelumnya dan jangan disentuh atau di-stage).
- Baca ulang `cli-findings.md` dan `README.md` untuk memastikan tidak ada nilai sensitif.

## Perubahan
Dibuat (baru):
- `internal/larkdata/testdata/auth-status.json`
- `internal/larkdata/testdata/base-table-list.json`
- `internal/larkdata/testdata/base-view-list.json`
- `internal/larkdata/testdata/base-record-list.json`
- `internal/larkdata/testdata/base-record-get.json`
- `internal/larkdata/testdata/wiki-space-list.json`
- `internal/larkdata/testdata/wiki-node-list.json`
- `internal/larkdata/testdata/wiki-node-get.json`
- `internal/larkdata/testdata/wiki-node-get-bitable.json`
- `internal/larkdata/testdata/wiki-node-list-my-library.json`
- `internal/larkdata/testdata/docs-fetch.json`
- `internal/larkdata/testdata/sheets-cells-get.json`
- `internal/larkdata/testdata/README.md`
- Opsional (dicatat di README): `base-record-list-page2.json`, `wiki-node-get-sheet.json`, `wiki-node-get-my-library.json`, `wiki-node-list-children.json`, `auth-status-not-logged-in.json`, `error-*.json`
- `docs/lark/cli-findings.md`

Diubah: tidak ada. Tidak ada perubahan Go, frontend, `overview.md`, atau file wiring.

## Wiring
Fitur ini tidak mengekspos kode. Yang diekspos untuk langkah Wiring (overview bagian 5, butir 4, 6, 7):
- `docs/lark/cli-findings.md` bagian "Rekomendasi tabel allowlist", "Pola galat", dan "Ringkasan untuk Wiring": dasar penyelarasan `allowlist.go` (`asUser`, `format`), pola `errors.go`, dan skema/aturan exit `lark.go`.
- Fixture di `internal/larkdata/testdata/` untuk parser tiket 02 sampai 04.
Jangan mengedit `allowlist.go`, `errors.go`, `lark.go`, atau `cli.go` untuk menyelaraskannya; cukup dokumentasikan perbedaannya.

## Selesai bila
- [ ] 12 fixture nama kontrak ada, direkam dari `lark-cli` nyata (bukan karangan), dan valid (`jq .` lulus untuk semua).
- [ ] Fixture teranonimkan: grep nilai asli yang diketahui dan pola `ou_`/email/token/host tenant kosong pada `internal/larkdata/testdata/` dan `docs/lark/`; struktur, tipe, dan panjang array tidak berubah dibanding output mentah.
- [ ] `internal/larkdata/testdata/README.md` mencatat perintah penghasil tiap fixture tanpa token.
- [ ] `docs/lark/cli-findings.md` memuat semua bagian pada langkah 10, termasuk flag `+view-list` dan `+cells-get`, `--as user` per perintah, tipe field dan bentuk nilai, penanda field utama, kursor paginasi, bentuk node wiki, perilaku `my_library`, status URL record (pasti/tidak pasti), pola galat belum-login/izin-kurang, dan bagian "Pengecualian dan pemotongan fitur" dengan keputusan eksplisit untuk kanban, wiki pribadi, dan link tiket.
- [ ] Tidak ada perintah tulis `lark-cli` yang dijalankan; tidak ada output mentah atau skrip anonimisasi di dalam repo.
- [ ] Guard: `git status --short` menunjukkan perubahan baru hanya di ownedPaths, dan `go build ./...` dari root repo tetap sukses (tidak ada perubahan Go; tanpa unit test).
