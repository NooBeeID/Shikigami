# Temuan spike lark-cli

Kontrak data untuk tiket 02 sampai 04, `larkdata-runner`, dan `lark-connector-adapter`. Semua temuan berasal dari eksekusi nyata (read-only, `--as user`) dan fixture di `internal/larkdata/testdata/` (nama berkas dirujuk per bagian). Tidak ada data sensitif di dokumen ini; nilai pada contoh adalah nilai palsu dari fixture.

## 1. Versi dan lingkungan

- `lark-cli --version`: `lark-cli version 1.0.97`. Tanggal spike: 2026-11-22. Brand: `lark`.
- Output sukses dan galat sama-sama memuat `_notice.update` (info CLI 1.0.100 tersedia) di dalam JSON, bukan di teks terpisah. Parser harus mengabaikan kunci `_notice`. Jangan menjalankan `lark-cli update`.
- Amplop sukses (stdout, exit 0): `{"ok":true,"identity":"user","data":{...},"meta":{"count":N},"_notice":{...}}`. `meta` hanya ada pada sebagian perintah.
- Amplop galat: ditulis ke **stderr**, stdout kosong, exit code non-nol, JSON pretty-print multi-baris: `{"ok":false,"identity":"user","error":{"type","subtype","code","message","hint","log_id"},"_notice":{...}}`.
- Flag `--page-all`, `--jq`, `--dry-run`, dan `--output` ada di CLI tetapi tidak dipakai epic ini.

## 2. Tabel perintah

Semua perintah diuji dengan `--as user` dan `--format json`; semuanya menerima keduanya. Pengecualian penting: `base +record-list` dan `base +record-get` memakai format **markdown** secara default, jadi `--format json` WAJIB. `--json` bukan shorthand format pada `+record-get` (artinya body JSON `{"record_id_list":[...]}`), jadi jangan pernah memakai `--json`; pakai `--format json`.

| Perintah | Flag wajib | Flag opsional penting | Format JSON | --as user | Fixture |
|---|---|---|---|---|---|
| `auth status` | - | `--verify` (cek ke server) | default JSON (flag `--json` ada, tidak perlu) | tidak ada flag `--as` | `auth-status.json` |
| `base +table-list` | `--base-token` | - (tanpa paginasi; `data.total`) | default json, `--format json` diterima | ya | `base-table-list.json` |
| `base +view-list` | `--base-token`, `--table-id` | `--limit` (1-200, default 100), `--offset` | default json | ya | `base-view-list.json` |
| `base +record-list` | `--base-token`, `--table-id` | `--limit` (1-200), `--offset`, `--view-id`, `--field-id` (ulang) | default markdown, **`--format json` wajib** (matriks) | ya | `base-record-list.json`, `base-record-list-page2.json` |
| `base +record-get` | `--base-token`, `--table-id`, `--record-id` (ulang) | `--field-id` | default markdown, **`--format json` wajib** | ya | `base-record-get.json` |
| `wiki +space-list` | - | `--page-size` (1-50), `--page-token`, `--page-all` | default json | ya | `wiki-space-list.json` |
| `wiki +node-list` | `--space-id` | `--parent-node-token`, `--page-size` (1-50), `--page-token` | default json | ya | `wiki-node-list.json`, `wiki-node-list-my-library.json` |
| `wiki +node-get` | `--node-token` (alias lama `--token`) | `--space-id` | default json | ya | `wiki-node-get*.json` |
| `docs +fetch` | `--doc` | `--doc-format` (xml, markdown, im-markdown), `--scope`, `--detail` | default json, `--format json` diterima | ya | `docs-fetch.json` |
| `sheets +cells-get` | `--spreadsheet-token` atau `--url`; `--sheet-id` atau `--sheet-name`; `--range` | `--max-chars` (default 500000), `--skip-hidden` | default json | ya | `sheets-cells-get.json` |

Tambahan di luar daftar awal (read, terbukti): `base +field-list` (`--base-token`, `--table-id`; `base-field-list.json`) dan `sheets +workbook-info` (`--spreadsheet-token`; `sheets-workbook-info.json`).

Nilai awal yang DIBANTAH atau DIKOREKSI: `+view-list` dan `+cells-get` ada dan sesuai spec; flag `+cells-get` memakai `--sheet-id` atau `--sheet-name` (bukan hanya token dan range) dan `--range` tanpa prefiks nama sheet. Skema `identities.user.{status,available,userName}` terkonfirmasi. `+record-get` dan `+record-list` default markdown BUKAN json (koreksi dari nilai awal "default JSON").

### Rekomendasi tabel allowlist

Nilai literal untuk `allowlist.go` (`format` ditambahkan `Run` setelah `--as user`):

```
{domain: "base",   command: "+table-list",  asUser: true, format: ["--format","json"]}
{domain: "base",   command: "+view-list",   asUser: true, format: ["--format","json"]}
{domain: "base",   command: "+record-list", asUser: true, format: ["--format","json"]}   // wajib, default markdown
{domain: "base",   command: "+record-get",  asUser: true, format: ["--format","json"]}   // wajib, default markdown
{domain: "base",   command: "+field-list",  asUser: true, format: ["--format","json"]}   // tambahan (opsional, lihat bagian 4)
{domain: "wiki",   command: "+space-list",  asUser: true, format: ["--format","json"]}
{domain: "wiki",   command: "+node-list",   asUser: true, format: ["--format","json"]}
{domain: "wiki",   command: "+node-get",    asUser: true, format: ["--format","json"]}
{domain: "docs",   command: "+fetch",       asUser: true, format: ["--format","json"]}
{domain: "sheets", command: "+cells-get",   asUser: true, format: ["--format","json"]}
{domain: "sheets", command: "+workbook-info", asUser: true, format: ["--format","json"]} // tambahan (opsional)
```

Pemanggil tidak boleh mengirim `--json` (konflik semantik pada `+record-get`), `--format`, `--as`, `--output`, atau `--jq`; `check()` sebaiknya menolak semuanya. `auth status` tidak termasuk allowlist `larkdata` (dipakai adapter lewat `RunCLI`; tanpa `--as`).

## 3. Skema JSON ringkas

Notasi `?` = bisa null atau tidak ada.

### auth status (`auth-status.json`)
`appId`, `brand` ("lark"), `defaultAs`, `identity`, `identities.bot.{status,available,message}`, `identities.user.{status,available,message,openId,userName,tokenStatus,scope,expiresAt,refreshExpiresAt,grantedAt}`. Dikonfirmasi: `identities.user.status` = `"ready"`, `available` bool, `userName` string. Sensitif (jangan dicatat/ditampilkan): `appId`, `openId`, `scope`, token. `tokenStatus` teramati `"valid"`. Nilai `status` lain (`needs_refresh`) tidak teramati; mengikuti spec.

### base +table-list
`data.tables[].{id,name,records_count,rev}`, `data.total`. Tanpa penanda primary, tanpa URL, tanpa paginasi.

### base +view-list
`data.views[].{id,name,type,_meta}`, `data.total`. `type` teramati: `grid`, `kanban`, `form`, `not_support` (view tak dikenal; tampilkan sebagai grid atau sembunyikan). `_meta.{filter,group,sort,visible_fields,card_cover?}`: `filter` string ringkas ("3 conditions") atau null; `group` array id field; `visible_fields` string ("38 fields"). Pada view `kanban`, `_meta.group[0]` = id field pengelompokan (single-select).

### base +record-list dan +record-get (matriks)
`data.fields[]` (nama field, urutan kolom), `data.field_id_list[]`, `data.field_type_list[]`, `data.data[][]` (satu baris per record, kolom paralel dengan `fields`), `data.record_id_list[]` (paralel dengan baris), `data.has_more` (bool), `data.rev`, `data.timezone` (mis. `Asia/Jakarta`), `data.query_context.{field_scope,record_scope}` (hanya `+record-list`). `+record-get` memakai bentuk yang sama tanpa `query_context` dengan satu baris, dan `data.has_more` ikut ada. Sel kosong = `null` (bukan array kosong, bukan kunci hilang).

### base +field-list
`data.fields[].{id,name,type,multiple?,options?[{name,hue,lightness}],style?,default_value?,expression?,link_table?,bidirectional?}`, `data.total`. Urutan di sini BERBEDA dari urutan kolom `+record-list`; cocokkan lewat `id`/`name`, bukan indeks.

### wiki +space-list
`data.spaces[].{space_id,name,description,space_type,visibility,open_sharing}`, `data.has_more`, `data.page_token`, `meta.count`. Catatan: kunci daftar adalah `spaces` (bukan `items`). `space_id` string numerik panjang. `space_type` teramati `team`.

### wiki +node-list
`data.nodes[].{node_token,obj_token,obj_type,node_type,title,has_child,parent_node_token,space_id}`, `data.has_more`, `data.page_token` (string kosong pada halaman terakhir). `parent_node_token` kosong pada akar. `node_type` teramati `origin`.

### wiki +node-get
Node langsung di `data` (tanpa `data.node`): `{node_token,obj_token,obj_type,node_type,origin_node_token,parent_node_token,space_id,title,has_child,creator,owner,node_create_time,obj_create_time,obj_edit_time,updated_at}`. Waktu `*_time` berupa string epoch detik; `updated_at` ISO8601. Tidak ada URL di output.

### docs +fetch
`data.document.{content,document_id,revision_id}`. Dengan `--doc-format markdown`, `content` berisi teks markdown (bisa memuat tag blok seperti `<title>` dan `<readonly-block>`; parser sebaiknya menampilkannya apa adanya atau menyaringnya). Tanpa judul terpisah dan tanpa paginasi pada pembacaan penuh (`--scope full` default). Menerima `obj_token` docx; node_token wiki docx juga diterima.

### sheets +workbook-info
`data.{token,revision,create_time,modified_time,sheets[]}`; `sheets[].{sheet_id,sheet_name,index,is_hidden,resource_type,row_count,column_count,merged_cells_count,chart_count,pivot_table_count,float_image_count}`.

### sheets +cells-get
`data.{ranges[],has_more,returned_cell_count,approx_char_count,revision,warning_message}`; `ranges[].{actual_range,cells[][]}`; sel = `{cell_styles{...}, value?}`. `value` bertipe string (angka juga string) dan hilang pada sel kosong. `warning_message` adalah teks tetap berbahasa Mandarin yang harus diabaikan. Cek `has_more` dan `actual_range` sebelum membaca; hasil bisa terpotong oleh `--max-chars` (default 500000, besar: 50 x 20 sel bisa mencapai ~60 KB karena `cell_styles`). Rekomendasi: baca rentang kecil (mis. `A1:T50`) dan pakai hanya `value`.

## 4. Tipe field dan bentuk nilai

Nama tipe ada di `data.field_type_list[]` (record) dan `type` (field-list).

| Tipe (nama di JSON) | Bentuk nilai | Teks tampilan | Dipakai kanban? |
|---|---|---|---|
| `text` | string; bisa memuat markdown link `[teks](url)`; `style.type` `plain` atau `url` | string apa adanya | tidak |
| `number` | angka JSON; `style.{percentage,precision}` | angka; tambahkan `%` bila `percentage` | tidak |
| `select` (tunggal) | array string berisi satu elemen, mis. `["Opsi 1"]` | elemen pertama | **ya** (field pengelompokan view kanban) |
| `select` (jamak) | array string; `multiple: true` hanya di `+field-list` | gabung dengan koma | tidak |
| `user` | array `{id,name}` | gabung `name` | tidak |
| `created_by` | array `{id,name}` | gabung `name` | tidak |
| `link` (relasi) | array `{id}` (id record tertaut, tanpa judul) | jumlah tertaut, mis. "2 tertaut" | tidak |
| `attachment` | array `{file_token,name,size}` | gabung `name` | tidak |
| `datetime`, `created_at`, `updated_at` | string ISO8601 beroffset (mis. `2026-01-31T00:00:00.000+07:00`), bukan epoch | format tanggal setempat | tidak |
| `checkbox` | `true` atau `false` | centang atau kosong | tidak |
| `formula` | angka atau string (teramati keduanya), bisa null | `String(nilai)` | tidak |
| `lookup` | teramati satu nilai skalar string; bentuk lain tidak teramati | `String(nilai)` jika skalar; array digabung koma | tidak |
| `auto_number` | string, mis. `TKT-0001` | string | tidak |
| Tipe lain (rating, progress, currency, phone, location, group_chat, stage, button, dll.) | tidak teramati | fallback: `JSON.stringify` terpotong | tidak |

Single versus multi select: kedua bentuk sama-sama array string di record; bedanya hanya `multiple` di `+field-list`. Untuk kanban, kolom dikelompokkan dari `_meta.group[0]` pada view `kanban`: nilai `["X"]` -> kolom "X"; "Tanpa nilai" = sel `null` (teramati 411 dari 2000 sel select bernilai null; array kosong tidak teramati). Bila tipe field pengelompokan bukan `select`, kanban tidak dapat dibuat untuk view itu.

## 5. Field utama (primary)

Tidak ada penanda eksplisit (`is_primary` atau serupa) di `+table-list`, `+view-list`, `+field-list`, maupun record. Aturan turunan: field **pertama** pada `data.fields[]` (kolom pertama matriks `+record-list`/`+record-get`) adalah field utama (pada tabel uji, "Task Name" bertipe `text`). Status: **tidak eksplisit**. Jangan memakai urutan `+field-list` (urutannya berbeda dan field pertamanya bukan field utama). Bila field pertama kosong atau bukan teks, fallback ke `record_id`.

## 6. Paginasi

- `+record-list`: `--limit` 1 sampai 200 (default 100; di atas 200 atau 0 -> galat validasi exit 2, lihat `error-invalid-limit.json`); halaman berikutnya lewat `--offset` (offset = jumlah baris yang dilewati; tidak ada kursor di respons). `data.has_more` true bila masih ada baris; `--offset` melewati akhir -> `data` kosong dan `has_more: false`. Contoh: `base-record-list.json` (limit 3) dan `base-record-list-page2.json` (offset 3), dua-duanya `has_more: true`.
- `+view-list`: `--limit` 1 sampai 200, `--offset`; respons memuat `data.total`.
- `+table-list`: tanpa flag paginasi; `data.total` ada (teramati 5).
- `+space-list` dan `+node-list`: `--page-size` 1 sampai 50 (default 50), kursor `data.page_token` + `data.has_more`; halaman berikutnya `--page-token <nilai>`. `--page-token` menyiratkan satu halaman. Pada halaman terakhir `has_more:false` dan `page_token:""` (`+node-list`) atau berisi string (`+space-list`, abaikan bila `has_more` false). `--page-all` tersedia (batas `--page-limit` default 10) tetapi tidak disarankan untuk wiki besar; teramati satu space akar berisi lebih dari 50 node.

## 7. Wiki

- Bentuk node: lihat bagian 3. `obj_type` teramati: `docx`, `sheet`, `bitable`, `file`, `slides`. Tidak teramati: `doc`, `mindnote`.
- URL node: TIDAK ada di output. Fallback `https://<host>/wiki/<node_token>`; `<host>` tidak ada di output wiki/auth. Host hanya terlihat di `base +base-get` (`data.base.url`) dan di tautan dalam isi record/dokumen; jadi tombol salin link wiki memerlukan host yang dikonfigurasi atau diturunkan, atau fitur dipotong (lihat bagian 10).
- `my_library`: `wiki +node-list --space-id my_library --as user` BERHASIL (`wiki-node-list-my-library.json`, `has_more` true pada page-size kecil). `+space-list` TIDAK memuatnya (hanya space tim; help CLI menyebut API tidak mengembalikan `my_library`). `space_id` nyata pada node my_library berbeda dari alias `my_library`. `+node-get` pada node my_library bekerja memakai `--node-token` (`wiki-node-get-my-library.json`); `--space-id my_library` pada `+node-get` dengan token tidak valid diperlakukan sebagai token tidak valid. `my_library` hanya valid dengan `--as user` (dengan bot -> galat validasi).
- Link Base di bawah wiki: node `obj_type: bitable` punya `obj_token` = token Base yang langsung dipakai di `base +table-list --base-token` (diuji nyata; fixture: `wiki-node-get-bitable.json` dan `base-table-list.json` merujuk token yang sama). `+node-get` juga menerima token Base mentah (`obj_token`) dan mengembalikan node wiki-nya.
- `docs +fetch` pada docx wiki: menerima `obj_token` node (dipakai di fixture). `--doc` juga menerima node_token wiki, tetapi pakai `obj_token` agar tidak bergantung pada deteksi server.
- `sheets +cells-get` pada sheet wiki: pakai `obj_token` node `sheet` sebagai `--spreadsheet-token`; daftar sheet dari `sheets +workbook-info` (butuh `sheet_id` atau nama sheet).

## 8. URL record

Status: **tidak pasti**. Bukti: tidak ada field URL di `+table-list`, `+view-list`, `+record-list`, `+record-get`, maupun `+field-list`. URL Base (`https://<host>/base/<base_token>`) hanya ada di `base +base-get` (tidak di allowlist). Dokumentasi skill `lark-base` hanya memuat pola `https://<host>/base/<base_token>?table=<table_id>` (tanpa `view` atau `record`). Pola `...&view=<view_id>&record=<record_id>` tidak terbukti oleh output atau dokumentasi. Anjuran: tombol Salin link tingkat tiket (record) dinonaktifkan; tautan tingkat Base dan tabel (`?table=`) boleh dipakai bila host diketahui (mis. dari `base +base-get`, dengan menambah entri allowlist) atau dari tautan yang ada di data.

## 9. Pola galat

Semua galat: stdout kosong, JSON di stderr, jadi `RunCLITimeout` (yang membungkus stderr dalam pesan `perintah lark-cli gagal: <stderr terpotong 300 karakter>`) sudah cukup; stdout tidak perlu dibaca untuk klasifikasi. JSON stderr pretty-print multi-baris dan `message` muncul dalam ~200 karakter pertama, jadi pemotongan 300 karakter masih memuat `type`, `subtype`, `code`, dan `message`; `hint` dan `_notice` bisa terpotong.

| Situasi | Saluran | Exit code | Pola substring stabil | Kode larkdata yang disarankan |
|---|---|---|---|---|
| Belum dikonfigurasi (tanpa app) | stderr | 3 | `"type": "config"`, `not_configured`, `not configured` | `not_logged_in` |
| Terkonfigurasi tetapi belum login | tidak direkam | tidak direkam | dari dokumentasi skill: `"type": "authorization"`; `login`; `token` | `not_logged_in` |
| Izin atau scope kurang | tidak direkam (tidak aman dipicu) | non-nol | dari dokumentasi skill: `missing_scope`, `"type": "authorization"`, `99991679`, `99991672`, `permission` | `permission` |
| Resource tidak ada atau token Base palsu | stderr | 1 | `NOTEXIST`, `91402` (bukan galat izin) | `cli_error` |
| Token node wiki tidak valid | stderr | 1 | `token is invalid`, `131013`, `invalid token length`, `131016`, `invalid_parameters` | `cli_error` |
| Space wiki tidak ada | stderr | 1 | `not_found`, `131005`, `not found: space not found` | `cli_error` |
| Argumen tidak valid (mis. `--limit 500`) | stderr | 2 | `"type": "validation"`, `invalid_argument`, `must be between` | `cli_error` (bug pemanggil) |
| Perintah salah | stderr | non-nol | `unknown command` | `cli_error` |
| CLI tidak ada | - | - | `connector.IsCLIMissing` (`ErrCLIMissing`) | `cli_missing` |
| Timeout | - | - | konteks melewati batas (`connector.RunCLITimeout`), bukan dari CLI | `timeout` |

Peringatan klasifikasi: pola awal `token` untuk belum-login terlalu longgar karena `token is invalid` (token wiki/Base salah) juga mengandung `token`; itu galat pemanggil, bukan belum login. Anjuran: cocokkan `not_configured`/`not configured`, `"type": "authorization"` (selain `missing_scope`), dan `login`; jangan mencocokkan `token` saja. Klasifikasi `permission` sebaiknya mendahului dan memeriksa `missing_scope`, `99991679`, `99991672`, `permission`.

## 10. Pengecualian dan pemotongan fitur

- **Kanban**: AKTIF. Ada view `kanban` dan tipe `select` untuk pengelompokan; `_meta.group[0]` memberi id field pengelompokan; "Tanpa nilai" = `null`. View kanban dengan field kelompok non-select (tidak teramati) cukup jatuh ke grid.
- **Wiki pribadi `my_library`**: AKTIF. List dan get bekerja dengan `--as user`; hanya tidak muncul di `+space-list`, jadi tambahkan entri "Wiki pribadi" secara manual di UI yang memakai alias `my_library`.
- **Link tiket (record)**: DIPOTONG: URL record tidak pasti (bagian 8), tombol Salin link tingkat tiket dinonaktifkan; tautan tingkat Base/view tetap aktif bila host diketahui. Link node wiki juga memerlukan host (tidak ada di output wiki).
- **Perintah tanpa `--as user`**: hanya `auth status` (tidak ada flag `--as`); tandai `asUser:false` bila suatu hari dimasukkan ke allowlist. Semua perintah baca lain menerima `--as user`.
- **`+view-list` dan `+cells-get`**: AKTIF; sesuai spec dengan koreksi flag (bagian 2). `+cells-get` memerlukan `--sheet-id` atau `--sheet-name`, sehingga fitur sheet perlu memanggil `sheets +workbook-info` (tambahan allowlist) untuk mendapat `sheet_id` sebelum membaca sel; bila tidak ingin menambah entri, sheet dipotong ke "judul dan link" (DIPOTONG: pratinjau sel dinonaktifkan).
- **Format `+record-list` dan `+record-get`**: DEVIASI dari nilai awal. Default markdown; `--format json` wajib (hasilnya matriks, bukan objek per record). Parser tiket 02 harus memakai matriks (`fields` + `data`).
- **Batas `--limit`**: maksimum 200 per panggilan, tidak 500 atau 2000 (hanya `ndjson`). Memuat lebih banyak butuh `--offset` berulang.
- **Pesan galat di stderr**: tidak ada galat yang muncul di stdout; `RunCLITimeout` cukup (tidak perlu mengubah `larkdata-runner`).
- **Pola galat belum login/izin**: tidak direkam (hanya keadaan belum dikonfigurasi, exit 3). Pola tidak terkonfirmasi ditandai "dari dokumentasi" di bagian 9. Fixture `auth-status-not-logged-in.json` tidak dibuat (DIPOTONG dari lingkup rekaman, tanpa dampak fungsional: adapter memperlakukan exit non-nol bergejala konfigurasi sebagai not_logged_in).
- **Judul dokumen**: `docs +fetch` tidak memberi judul terpisah; gunakan `title` dari node wiki.
- **`has_more` kursor**: `+record-list` tidak mengembalikan kursor; paginasi memakai `--offset` dari klien.

## 11. Ringkasan untuk Wiring

1. `allowlist.go`: ganti nilai `format` semua entri menjadi `["--format","json"]` (khususnya `base +record-list` dan `base +record-get` yang default markdown); `asUser: true` untuk semua. Tolak flag `--json`, `--output`, `--jq`, selain `--as`/`--format`. Pertimbangkan menambah `base +field-list` dan `sheets +workbook-info`.
2. `errors.go`: pola belum login `not_configured`, `not configured`, `"type": "authorization"`, `login` (hindari `token` saja); pola izin `missing_scope`, `99991679`, `99991672`, `permission`. Galat resource (`131013`, `131005`, `NOTEXIST`) tetap `cli_error`.
3. `lark.go` (adapter): `auth status` tanpa flag tambahan (default JSON, tanpa `--as`). Exit non-nol dengan pesan `not configured` -> `StateNotLoggedIn`. `identities.user.available == false` -> `not_logged_in`; `status: ready` atau `needs_refresh` -> `connected` dengan `userName`. Abaikan `_notice`.
4. Parser tiket 02: pakai matriks `fields`/`data`/`record_id_list`, field utama = kolom pertama, tanggal ISO8601, select = array string.
