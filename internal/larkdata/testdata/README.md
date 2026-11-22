# Fixture lark-cli

Folder ini berisi hasil rekaman `lark-cli` nyata (read-only, `--as user`) yang sudah dianonimkan. Fixture menjadi acuan parser tiket 02 sampai 04 dan klasifikasi galat `larkdata`. Temuan lengkap ada di `docs/lark/cli-findings.md`.

- Versi `lark-cli` saat rekam: `1.0.97` (brand `lark`).
- Tanggal rekam: 2026-11-22.
- Semua fixture adalah JSON valid (`jq .`), diindentasi 2 spasi.

## Daftar berkas

Placeholder: `<BASE_TOKEN>`, `<TABLE_ID>`, `<VIEW_ID>`, `<RECORD_ID>`, `<SPACE_ID>`, `<NODE_TOKEN>`, `<OBJ_TOKEN>`, `<SPREADSHEET_TOKEN>`, `<SHEET_ID>`. Kolom perintah menulis flag persis seperti dipakai saat rekam (stdout kecuali disebut galat).

| Berkas | Perintah | Catatan |
|---|---|---|
| `auth-status.json` | `lark-cli auth status` | Tanpa `--as` dan tanpa flag format (default sudah JSON). `scope` dipendekkan. |
| `base-table-list.json` | `lark-cli base +table-list --base-token <BASE_TOKEN> --as user --format json` | Base dengan 5 tabel. |
| `base-view-list.json` | `lark-cli base +view-list --base-token <BASE_TOKEN> --table-id <TABLE_ID> --as user --format json` | Berisi view `grid`, `kanban`, `form`, `not_support`. |
| `base-record-list.json` | `lark-cli base +record-list --base-token <BASE_TOKEN> --table-id <TABLE_ID> --limit 3 --as user --format json` | Bentuk matriks (`data.data[][]`). Tabel punya 38 field dengan tipe attachment, auto_number, created_by, datetime, formula, link, number, select (single dan multi), text, user. |
| `base-record-list-page2.json` | `lark-cli base +record-list --base-token <BASE_TOKEN> --table-id <TABLE_ID> --limit 3 --offset 3 --as user --format json` | Halaman kedua lewat `--offset`. |
| `base-record-get.json` | `lark-cli base +record-get --base-token <BASE_TOKEN> --table-id <TABLE_ID> --record-id <RECORD_ID> --as user --format json` | Bentuk sama dengan `+record-list` (satu baris). Awas: `--json` pada perintah ini berarti body JSON, bukan format. |
| `base-field-list.json` | `lark-cli base +field-list --base-token <BASE_TOKEN> --table-id <TABLE_ID> --as user --format json` | Tambahan: definisi tipe field (`multiple`, `options`). Belum ada di allowlist awal. |
| `wiki-space-list.json` | `lark-cli wiki +space-list --as user --format json` | Daftar ada di `data.spaces`, bukan `items`. `my_library` tidak muncul. |
| `wiki-node-list.json` | `lark-cli wiki +node-list --space-id <SPACE_ID> --page-size 6 --as user --format json` | Akar space; `has_more: true` dengan `page_token`. |
| `wiki-node-list-my-library.json` | `lark-cli wiki +node-list --space-id my_library --page-size 5 --as user --format json` | `my_library` berhasil di-list. |
| `wiki-node-get.json` | `lark-cli wiki +node-get --node-token <NODE_TOKEN> --as user --format json` | Node `docx`. Node langsung di `data` (tanpa `data.node`). |
| `wiki-node-get-sheet.json` | `lark-cli wiki +node-get --node-token <NODE_TOKEN> --as user --format json` | Node `sheet`. |
| `wiki-node-get-bitable.json` | `lark-cli wiki +node-get --node-token <NODE_TOKEN> --as user --format json` | Node `bitable`; `obj_token` = `<BASE_TOKEN>` yang sama dengan fixture Base. |
| `wiki-node-get-my-library.json` | `lark-cli wiki +node-get --node-token <NODE_TOKEN> --as user --format json` | Node dari `my_library` (`space_id` berbeda dari space tim). |
| `docs-fetch.json` | `lark-cli docs +fetch --doc <OBJ_TOKEN> --doc-format markdown --as user --format json` | Isi di `data.document.content`. Isi dokumen diganti teks palsu. |
| `sheets-workbook-info.json` | `lark-cli sheets +workbook-info --spreadsheet-token <SPREADSHEET_TOKEN> --as user --format json` | Tambahan: cara mendapat `sheet_id` untuk `+cells-get`. |
| `sheets-cells-get.json` | `lark-cli sheets +cells-get --spreadsheet-token <SPREADSHEET_TOKEN> --sheet-id <SHEET_ID> --range A1:F12 --as user --format json` | Rentang dikecilkan ke 12 baris x 6 kolom agar fixture kecil; bentuk sama untuk rentang lebih besar. |
| `error-node-not-found.json` | `lark-cli wiki +node-get --node-token <TOKEN_PALSU> --as user --format json` | Galat di stderr, stdout kosong, exit code 1. Berkas ini hanya envelope stderr. |
| `error-permission.json` | `lark-cli base +table-list --base-token <TOKEN_PALSU> --as user --format json` | Galat di stderr, exit code 1, kode `91402` `NOTEXIST`. Bukan galat izin sungguhan; lihat findings. |
| `error-not-configured.json` | `HOME=<folder kosong> lark-cli auth status` | Galat di stderr, exit code 3. Keadaan "belum dikonfigurasi", bukan "terkonfigurasi tapi belum login". |
| `error-invalid-limit.json` | `lark-cli base +record-list --base-token <BASE_TOKEN> --table-id <TABLE_ID> --limit 500 --as user --format json` | Galat validasi di stderr, exit code 2 (batas `--limit` 1 sampai 200). |

Fixture yang tidak direkam: `auth-status-not-logged-in.json` (terkonfigurasi tetapi belum login; tidak aman dipicu tanpa logout). Alasan dan penggantinya ada di findings bagian Pengecualian.

## Aturan anonimisasi

Hanya nilai yang diganti; kunci, struktur, urutan, tipe data, dan panjang array dipertahankan (diverifikasi dengan membandingkan seluruh path dan tipe antara output mentah dan hasil). Pemetaan konsisten antarberkas:

- Token Base: `bascnFAKE000000000000000001` (prefix dan panjang asli dipertahankan). Dokumen `doxcnFAKE...`, sheet `shtcnFAKE...`, node wiki `wikcnFAKE...`, tabel `tblFAKE...`, view `vewFAKE...`, field `fldFAKE...`, record `recFAKE...`.
- `space_id` numerik: `7000000000000000001` dan seterusnya. `open_id` menjadi `ou_FAKE...`, `appId` menjadi `cli_fake...`.
- Nama user menjadi `Pengguna Contoh N`; judul menjadi `Judul Contoh N`; nama field `Kolom N` (`Kolom 1` adalah field pertama pada matriks record); opsi select `Opsi N`; nilai teks `Teks Contoh N`; auto_number `TKT-000N`; lampiran `berkas-contoh-N.png`.
- Tanggal dipindah ke Januari 2026 dengan format asli; angka diganti angka palsu bertipe sama (bulat tetap bulat). Host menjadi `example.larksuite.com`. `scope` dipendekkan. `log_id` diganti.
- Dipertahankan: nama tipe field, `has_more`, bentuk `page_token`, `obj_type`, `has_child`, `node_type`, kode galat, pesan galat CLI (tidak berisi data pengguna), `_notice` (hanya teks versi).

## Menambah fixture baru

1. Rekam di folder sementara di luar repo (mis. scratchpad). Jangan pernah menaruh output mentah di repo.
2. Anonimkan dengan aturan di atas (skrip di luar repo).
3. Verifikasi: `jq . berkas.json`, bandingkan path dan tipe dengan output mentah, lalu `grep -rFf <daftar nilai asli> internal/larkdata/testdata` harus kosong, dan tidak ada `ou_` asli, email, token akses, atau host tenant asli.
4. Catat perintah penghasilnya di tabel ini.
