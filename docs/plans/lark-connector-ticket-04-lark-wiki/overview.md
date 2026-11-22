# Overview: lark-connector-ticket-04-lark-wiki (Wiki pohon dan penampil dokumen)

Sumber: tiket `docs/tickets/lark-connector-ticket-04-lark-wiki.md` dan spec `docs/superpowers/specs/2026-11-22-09-lark-connector-design.md`. Dokumen ini adalah sumber kebenaran kontrak bersama; plan per fitur hanya merinci langkah kerja. Module Go: `github.com/NooBeeID/shikigami`. Tiket ini berjalan di atas `lark-connector-ticket-01-lark-connector` (sudah merge: `internal/larkdata/{runner,allowlist,errors}.go`, `docs/lark/cli-findings.md`, fixture `internal/larkdata/testdata/`, `cmd/shikigamictl/cmd_lark.go` dengan `registerLarkSub`) dan `lark-connector-ticket-02-lark-sources` (config sumber, `store.LarkSources`, `store.ts`, `actions.ts`, `source.ts`, placeholder `doc/DocPane.svelte` dan `sections/WikiTree.svelte`, `WikiSection`). Tidak bergantung pada tiket 03.

Catatan penting saat plan ini ditulis: kode tiket 02 BELUM ada di pohon kerja (`cmd/shikigamid/wire_lark.go`, `internal/workspace/lark.go`, `frontend/src/modules/lark/` belum ada; hanya plan tiket 02 dan 03 yang ada). Semua nama tiket 02 di bawah diambil dari `docs/plans/lark-connector-ticket-02-lark-sources/overview.md`. Agent implementasi wajib memeriksa nama persis di kode saat tiket 02 sudah merge, dan melaporkan selisih (jangan mengarang ulang). Tiket ini tidak boleh dijalankan sebelum tiket 02 merge; bila belum, hentikan dan laporkan.

## 1. Ringkasan epic dan urutan eksekusi

Epic mengisi dua placeholder tiket 02: `sections/WikiTree.svelte` (pohon lazy per level, Pribadi `my_library` di paling atas oleh `WikiSection`) dan `doc/DocPane.svelte` (pane `lark-doc`: docx sebagai markdown, sheet sebagai tabel sel awal 50 x 20, tipe lain judul + link "Buka di Lark"). Semua read-only lewat `lark-cli` (`--as user`, json) melalui `larkdata.Run()`. Backend: IPC `lark.wiki.list` dan `lark.wiki.get` (hasil dinormalisasi). Alat debug: `shikigamictl lark wiki list|get`.

Urutan:
1. **Foundation** (`00-foundation.md`, `hasFoundation=true`): skeleton Go `wiki.go` dan `doc.go` (tipe + stub fungsi) agar `wire_lark_wiki.go` dan implementasi dapat dikompilasi paralel. Frontend tidak butuh Foundation (lihat bagian 4).
2. **Fitur paralel**: `lark-wiki-data`, `lark-wiki-tree-ui`, `lark-doc-pane`. Masing-masing hanya menyentuh `ownedPaths`-nya.
3. **Wiring**: `lark-wiki-wiring` (terakhir): ctl `lark wiki`, verifikasi `WikiSection` -> `WikiTree` -> pane `lark-doc`, daemon, aplikasi.

### Keputusan overview (dasar semua plan fitur)

Temuan dari `docs/lark/cli-findings.md` dan kode yang ada, yang menggantikan asumsi tiket bila berbeda:

1. **Tanpa dependensi baru.** `frontend/src/modules/files/viewer/markdown.ts` sudah berupa parser markdown murni tanpa DOM, HTML mentah diperlakukan sebagai teks literal, `href` hanya `http(s)`/`mailto`, dan `files/viewer/MarkdownView.svelte` (prop `source: string`) membuka tautan dengan `target="_blank" rel="noopener noreferrer"` plus `BrowserOpenURL`. Ini memenuhi syarat sanitasi tiket. Maka `lark-doc-pane` TIDAK menambah `marked`/`dompurify` dan `frontend/package.json` tidak diubah (tetap milik fitur ini, tetapi tidak dipakai; bila ternyata ada kebutuhan nyata, laporkan di akhir plan, jangan menambah diam-diam). `doc/MarkdownView.svelte` adalah pembungkus tipis atas `../../files/viewer/MarkdownView.svelte` (impor lintas modul `files` hanya untuk komponen ini; `files/*` tidak diubah). Gambar tidak dirender (parser tidak punya node gambar); tidak ada yang bisa merusak tata letak.
2. **Tag blok markdown Lark.** `docs +fetch --doc-format markdown` bisa memuat tag blok seperti `<title>` dan `<readonly-block>` (findings bagian 3). `FetchDocMarkdown` menghapus baris yang HANYA berisi tag pembuka/penutup `title` atau `readonly-block` (regex `^\s*</?(title|readonly-block)\b[^>]*>\s*$`); tag lain dibiarkan (tampil sebagai teks literal, aman). Frontend tidak memfilter lagi.
3. **Host untuk link.** Output wiki/auth tidak memuat URL maupun host (findings 7). `GetWikiNode` membangun URL `https://<host>/wiki/<node_token>`. Resolusi `host` di `wire_lark_wiki.go`: sumber wiki -> `WikiSource.Host`; sumber `personal` -> `Host` wiki pertama di workspace yang `Host`-nya tidak kosong, lalu `Host` Base pertama (`Tickets`), lalu kosong. Tidak ada host bawaan (spike tidak menghasilkan satu pun; jangan menebak domain). Bila `host` kosong: `url` dikembalikan kosong (`""`), `DocPane` menonaktifkan Salin link/Kirim ke agen dengan keterangan "Link tidak bisa dibuat (host belum diketahui). Tambah satu Base atau wiki dari link untuk menentukan host." Keputusan ini dicatat di keterangan tiket.
4. **Sheet butuh `sheet_id`.** `sheets +cells-get` memerlukan `--sheet-id`/`--sheet-name` dan `--range` tanpa prefiks nama sheet (findings 2). `FetchSheetCells` memanggil `sheets +workbook-info --spreadsheet-token <objToken>` (sudah di allowlist), memilih sheet pertama yang tidak `is_hidden`, lalu `sheets +cells-get --spreadsheet-token <objToken> --sheet-id <id> --range A1:T50`, hanya memakai `value` sel (abaikan `cell_styles`, `warning_message`). Hasil dipadatkan menjadi matriks string maksimal 50 x 20 (sel tanpa `value` = `""`). `truncated=true` bila `row_count > 50`, `column_count > 20`, atau `has_more` true. Bila workbook tak punya sheet terlihat: matriks kosong `[]`, `truncated=false`.
5. **`wiki +node-get` untuk `personal`.** Jangan kirim `--space-id my_library` (findings 7: diperlakukan sebagai token tidak valid); kirim hanya `--node-token`. Untuk sumber wiki biasa `--space-id <SpaceID>` boleh dikirim. `docs +fetch` memakai `--doc <objToken>` (bukan `nodeToken`).
6. **Paginasi.** Satu halaman = `--page-size 50`, tanpa `--page-token`. Penanda `data.has_more` dinormalisasi; tambahan ADITIF di luar tiket: `ListWikiNodesPage` mengembalikan `WikiPage{Nodes, HasMore}` dan IPC `lark.wiki.list` menyertakan `hasMore?: boolean`. `WikiTree` menampilkan catatan "Menampilkan 50 pertama" bila `hasMore` (tanpa tombol muat lagi; di luar lingkup tiket).
7. **`personal` disembunyikan.** `my_library` berhasil di-list (findings 10), tetapi tetap dipertahankan jalur `hidden`: bila `ListWikiNodes` untuk `my_library` mengembalikan galat `permission` atau `cli_error`, IPC mengembalikan `{nodes: [], hidden: true, hiddenReason: "<pesan Indonesia + keterangan CLI>"}` bukan galat. `cli_missing`, `not_logged_in`, `timeout` tetap galat keras (panel sudah mengajak ke Pengaturan > Connector). Untuk sumber non-personal semua galat keras.
8. **`parentToken` dan `RootToken`.** `lark.wiki.list` dengan `parentToken` kosong: sumber dengan `RootToken` -> `--parent-node-token <RootToken>`; sumber tanpa `RootToken` dan `personal` -> flag `--parent-node-token` tidak dikirim (akar space). Kontrak tiket: "dikosongkan untuk akar".
9. **Tiap klien frontend mandiri.** `wikiTreeClient.ts` dan `docClient.ts` masing-masing mendefinisikan tipe dan `parseLarkError` lokal (salinan kecil, regex yang sama dengan tiket 02 bagian 2.4). Jangan mengimpor `board/boardClient.ts` (tiket 03) maupun `client.ts` (tiket 02) agar tiket 04 tidak bergantung pada tiket 03.

## 2. Kontrak bersama

### 2.1 Go: `internal/larkdata` (tipe dan signature dibuat Foundation; isi oleh `lark-wiki-data`)

`wiki.go`:
```go
const wikiPageSize = 50
const PersonalSpaceID = "my_library"

type WikiNode struct {
    Token    string `json:"token"`
    Title    string `json:"title"`
    ObjType  string `json:"objType"`
    ObjToken string `json:"objToken"`
    HasChild bool   `json:"hasChild"`
}
type WikiPage struct {
    Nodes   []WikiNode `json:"nodes"`   // tidak pernah nil
    HasMore bool       `json:"hasMore"`
}
type WikiNodeInfo struct {
    Title    string `json:"title"`
    ObjType  string `json:"objType"`
    ObjToken string `json:"objToken"`
    URL      string `json:"url"` // kosong bila host kosong
}
func ListWikiNodes(ctx context.Context, spaceID, parentToken string) ([]WikiNode, error)  // pembungkus ListWikiNodesPage
func ListWikiNodesPage(ctx context.Context, spaceID, parentToken string) (WikiPage, error)
func GetWikiNode(ctx context.Context, spaceID, nodeToken, host string) (WikiNodeInfo, error)
```
`doc.go`:
```go
const sheetMaxRows = 50
const sheetMaxCols = 20

type SheetCells struct {
    Cells     [][]string `json:"cells"` // tidak pernah nil; maks 50 x 20
    Truncated bool       `json:"truncated"`
}
func FetchDocMarkdown(ctx context.Context, objToken string) (string, error)
func FetchSheetCells(ctx context.Context, objToken string) (SheetCells, error)
```
Aturan pemanggilan CLI: hanya `larkdata.Run()` (perintah dan flag format ditambahkan otomatis); perintah hanya `wiki +node-list`, `wiki +node-get`, `docs +fetch`, `sheets +workbook-info`, `sheets +cells-get`, semuanya sudah ada di allowlist (JANGAN mengubah `allowlist.go`; bila ternyata ada yang kurang, laporkan). Argumen jangan memuat `--as`, `--format`, `--json`, `--output`, `--jq`. Timeout memakai `ListTimeout` bawaan runner (20 detik); tambahkan `context.WithTimeout` hanya bila `ctx` tanpa batas. Parser mengabaikan `_notice`; sukses di `data`, galat sudah diklasifikasi runner menjadi `*larkdata.Error` dan dikembalikan apa adanya. Bentuk JSON mengikuti `docs/lark/cli-findings.md` bagian 3 (jangan menebak): node-list `data.nodes[].{node_token,obj_token,obj_type,title,has_child}` + `data.has_more`; node-get langsung di `data` `{obj_token,obj_type,title,...}`; docs-fetch `data.document.content`; cells-get `data.ranges[0].cells[][].value`, `has_more`; workbook-info `data.sheets[].{sheet_id,is_hidden,row_count,column_count}`. `larkdata` tidak mengimpor `internal/workspace`. Berkas tiket 01/02 (`runner.go`, `allowlist.go`, `errors.go`, `link.go`, `tables.go`, `testdata/`) tidak diubah; tipe `wiki.go`/`doc.go` tidak bentrok dengan `tickets.go` (tiket 03: gunakan nama berawalan `wiki`/`sheet`/`doc`, mis. `wikiPageSize`, bukan `pageSize`).

### 2.2 IPC (json camelCase)

`lark.wiki.list` params `{workspaceId, sourceId, parentToken?}` (`sourceId` = ID `WikiSource` atau literal `"personal"`) -> 
```
{nodes: [{token, title, objType, objToken, hasChild}], hasMore?: boolean, hidden?: boolean, hiddenReason?: string}
```
`nodes` selalu array (bukan null). Satu level, maks 50.

`lark.wiki.get` params `{workspaceId, sourceId, nodeToken}` -> `{title, objType, url, markdown?, cells?, truncated?}`:
- `objType='docx'`: `markdown` terisi (string, boleh kosong).
- `objType='sheet'`: `cells` (matriks string) dan `truncated` (boolean).
- tipe lain: hanya `title`, `objType`, `url`.
- `url` selalu ada sebagai string (boleh `""` bila host tidak diketahui, keputusan 1.3).
Urutan: `GetWikiNode` dulu (judul, tipe, `objToken`, URL), lalu `FetchDocMarkdown`/`FetchSheetCells` dengan `objToken` hanya untuk `docx`/`sheet`. `nodeToken` kosong -> `[invalid_params]`.

Galat: string `"[<code>] <pesan>"` dibungkus `ipc.NewError(<kode ipc>, ...)` (konvensi tiket 02 bagian 2.4; frontend mengurai dengan regex `^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$`, tanpa awalan `code='unknown'`):
- Dari `larkdata.Error`: `cli_missing`, `not_logged_in`, `permission` (pesan "Izin Lark kurang" + pesan CLI), `timeout`, `not_allowed`, `cli_error` -> `ipc.ErrInternal`.
- `sourceId` tidak ada di config workspace (atau workspace tak ada) -> `[source_not_found] Sumber tidak ada di workspace ini` -> `ipc.ErrNotFound`. Kode `source_not_found` didefinisikan sebagai konstanta lokal `wire_lark_wiki.go`.
- Parameter tidak valid -> `[invalid_params] ...` -> `ipc.ErrInvalid`.
- Tanpa panic; galat tak dikenal -> `[cli_error] <pesan>`.
- Tidak ada publikasi event dan tidak ada tulis ke store.

### 2.3 Daemon: `cmd/shikigamid/wire_lark_wiki.go`
- `RegisterModule("lark-wiki", ...)` di `init()` (nama unik; `modules.go` tidak diubah). Pola sama dengan `wire_lark.go`/`wire_lark_link.go` tiket 02.
- Store diambil malas di handler: `d.Lookup("workspace.store")` + type assert ke `*workspace.Store` (disediakan Foundation tiket 02). Resolusi sumber lewat `store.LarkSources(wsID)` lalu cari `Wiki[i].ID == sourceId`; `personal` tidak dicari di config (spaceId `my_library`, `RootToken` kosong, host sesuai 1.3).
- Semua helper lokal di file ini memakai awalan `wiki` yang unik di paket `main` (mis. `wikiStore`, `wikiResolve`, `wikiErr`); jangan memakai nama milik `wire_lark.go` (`larkStore`), `wire_lark_link.go` (`linkStore`), atau `wire_lark_tickets.go` (`tickets*`).

### 2.4 Frontend

Tipe lokal (didefinisikan di tiap klien, bagian keputusan 1.9):
```ts
type LarkError = { code: string; message: string };
type WikiNode = { token: string; title: string; objType: string; objToken: string; hasChild: boolean };
type WikiListResult = { nodes: WikiNode[]; hasMore?: boolean; hidden?: boolean; hiddenReason?: string };
type WikiGetResult = { title: string; objType: string; url: string; markdown?: string; cells?: string[][]; truncated?: boolean };
```
`wikiTreeClient.ts` (milik `lark-wiki-tree-ui`): `export function listWikiNodes(workspaceId: string, sourceId: string, parentToken?: string): Promise<WikiListResult>` -> `call('lark.wiki.list', ...)` dari `frontend/src/lib/ipc.ts`; mengekspor juga `parseLarkError` dan tipe di atas. Melempar `LarkError` saat gagal; `nodes` dijamin array (`?? []`). `workspaceId` diambil dari `activeWorkspaceId` (`lib/stores/workspaces`) oleh komponen.

`docClient.ts` (milik `lark-doc-pane`): `export function getWikiNode(workspaceId: string, sourceId: string, nodeToken: string): Promise<WikiGetResult>` -> `call('lark.wiki.get', ...)`; mengekspor `parseLarkError` dan tipe.

Komponen:
- `WikiTree.svelte`: props `{ source, onOpen(nodeToken: string) }` dengan `source: WikiSource | { id: 'personal'; name: string; spaceId: 'my_library'; host: string }` (kontrak tiket 02, TIDAK berubah). Semua node memanggil `onOpen(node.token)`; panah hanya membuka/menutup.
- `DocPane.svelte`: props `{ pane: Leaf; visible: boolean }` (kontrak `PaneKindSpec.component` tiket 02), membaca `pane.sourceId` dan `pane.nodeToken`. Registrasi pane dan loader layout tidak diubah.
- `MarkdownView.svelte` (doc): props `{ source: string }`. `SheetView.svelte`: props `{ cells: string[][]; truncated: boolean }`.

Dari tiket 02 (periksa nama persis di kode): `copyLink(url)` dan `sendToAgent(url)` dari `../actions`; `resolveSource` dari `../source` (opsional); `activeWorkspaceId` dari `lib/stores/workspaces`; event `workspace.changed` (payload `kind === 'configChanged'`) lewat `onEvent` dari `lib/ipc`; `showToast` bila perlu. `LarkIcon.svelte` tidak diubah.

Aturan UI umum: teks bahasa Indonesia; tanpa polling otomatis (ambil saat mount/pemulihan, ganti sumber/node, tombol segarkan); respons usang diabaikan (penghitung `seq`); komponen tidak melempar galat keluar (keadaan galat + tombol coba lagi); gaya memakai token tema yang ada (terang/gelap); galat `cli_missing`/`not_logged_in` menyarankan Pengaturan > Connector lewat `openSettings('connectors')` dari `../settings` bila fungsi itu ada, tanpa mengubah modul settings.

### 2.5 ctl (Wiring)
`cmd/shikigamictl/cmd_lark_wiki.go` memanggil `registerLarkSub("wiki", fn)` dari `init()` (mekanisme `cmd_lark.go`: `type larkSubFunc func(ctx, c *ipc.Client, args []string) error`; tiru `cmd_lark_sources.go` tiket 02 untuk pemanggilan IPC dan cetak). Sub-subcommand, flag lewat `flag.NewFlagSet`:
- `lark wiki list --workspace <id> --source <id|personal> [--parent <token>]` -> `lark.wiki.list`; cetak ringkas satu baris per node (`token<TAB>objType<TAB>hasChild<TAB>title`), lalu `hasMore`, atau keterangan `hidden`.
- `lark wiki get --workspace <id> --source <id|personal> --node <token>` -> `lark.wiki.get`; cetak `title`, `objType`, `url`, lalu markdown atau tabel sel ringkas (TSV) dan `truncated`.
Jangan mengubah `cmd_lark.go`; bila `registerLarkSub` tidak ada, laporkan, jangan membuat dispatcher sendiri.

### 2.6 Konvensi
- Teks UI dan galat: bahasa Indonesia; identifier kode: bahasa Inggris.
- Tanpa token di config; tanpa cache disk (cache pohon hanya di memori komponen); read-only total (jangan jalankan perintah tulis `lark-cli`); tanpa unit test; tanpa dependensi baru (keputusan 1.1).
- Satu berkas satu pemilik.

## 3. Daftar fitur

| Slug | Scope | ownedPaths |
|---|---|---|
| `lark-wiki-data` | Implementasi `ListWikiNodes(Page)`, `GetWikiNode`, `FetchDocMarkdown`, `FetchSheetCells` (menimpa stub Foundation), normalisasi, keadaan `hidden` Pribadi, IPC `lark.wiki.list` dan `lark.wiki.get` dengan resolusi sumber, host, dan pemetaan galat. | `internal/larkdata/wiki.go`, `internal/larkdata/doc.go`, `cmd/shikigamid/wire_lark_wiki.go` |
| `lark-wiki-tree-ui` | `WikiTree.svelte` lazy (memuat/kosong/galat per node, segarkan, ikon per tipe, hidden Pribadi, keyboard/ARIA, reset saat sumber/workspace berganti dan saat `configChanged`) dan `wikiTreeClient.ts`. | `frontend/src/modules/lark/sections/WikiTree.svelte`, `frontend/src/modules/lark/wikiTreeClient.ts` |
| `lark-doc-pane` | `DocPane.svelte` (header judul/segarkan/Salin link/Kirim ke agen, status memuat/galat, stale guard, pulih dari persist), `MarkdownView.svelte` (pembungkus renderer files), `SheetView.svelte`, `docClient.ts`. | `frontend/src/modules/lark/doc/DocPane.svelte`, `frontend/src/modules/lark/doc/MarkdownView.svelte`, `frontend/src/modules/lark/doc/SheetView.svelte`, `frontend/src/modules/lark/doc/docClient.ts`, `frontend/package.json` (tidak diubah, keputusan 1.1) |
| `lark-wiki-wiring` | WIRING: ctl `shikigamictl lark wiki list|get`, verifikasi `WikiSection` memakai `WikiTree` dan `onOpen` membuka pane `lark-doc`, verifikasi daemon dan alur akhir di aplikasi. | `cmd/shikigamictl/cmd_lark_wiki.go` |

Acceptance criteria rinci tiap fitur ada di tiket. Penyimpangan dari tiket: (a) tanpa dependensi markdown/sanitizer baru karena renderer aman sudah ada (1.1); (b) `hasMore` aditif pada list (1.6); (c) `url` kosong bila host tak diketahui, tombol dinonaktifkan (1.3); (d) tombol sheet memakai `workbook-info` untuk `sheet_id` (1.4). Semuanya konsekuensi temuan spike atau kode yang ada.

## 4. Foundation

Ada (`00-foundation.md`). Isi: skeleton `internal/larkdata/wiki.go` dan `doc.go` (tipe, konstanta, dan stub fungsi sesuai 2.1) sehingga `wire_lark_wiki.go` dapat dikompilasi paralel dengan implementasi. Foundation menyentuh dua path milik fitur `lark-wiki-data` (stub yang kemudian ditimpa pemilik); keputusan sadar, pola yang sama dengan tiket 03. Frontend TIDAK memerlukan Foundation: tiap klien mandiri (1.9) dan placeholder tiket 02 sudah memberi kontrak props. Tidak ada perubahan di luar kedua berkas itu.

## 5. Wiring checklist

Dikerjakan `lark-wiki-wiring` setelah tiga fitur lain selesai:
1. `frontend/src/modules/lark/sections/WikiSection.svelte` (tiket 02, JANGAN diubah): verifikasi merender `WikiTree` dengan props `{source, onOpen}`, dan `onOpen(nodeToken)` membuka pane `lark-doc` lewat `layout.openPane({ kind: 'lark-doc', sourceId, nodeToken, title })` (Pribadi: `sourceId: 'personal'`). Karena placeholder ditimpa di jalur yang sama dan props tetap, tidak ada edit. Bila `WikiSection` ternyata memanggil `onOpen` dengan bentuk berbeda atau tidak membuka pane, laporkan sebagai temuan, jangan diubah.
2. Daemon: `wire_lark_wiki.go` mendaftar lewat `init()` (`RegisterModule("lark-wiki", ...)`); `modules.go` TIDAK diubah. Pastikan nama modul unik, `go build ./...` lulus, `lark.wiki.list` dan `lark.wiki.get` terjawab (lewat `shikigamictl lark wiki ...`).
3. ctl: buat `cmd/shikigamictl/cmd_lark_wiki.go` (kontrak 2.5); `shikigamictl lark` menampilkan `wiki` di bantuan.
4. Tidak ada siklus impor: `DocPane` -> `MarkdownView` (doc) -> `files/viewer/MarkdownView`; `DocPane` -> `docClient`; `WikiTree` -> `wikiTreeClient`; klien tidak mengimpor komponen.
5. Jalankan aplikasi (skill `run`) dan periksa: panel Lark bagian Wiki menampilkan Pribadi di atas; buka pohon beberapa level (memuat per level, keadaan kosong "Belum ada isi", segarkan); buka satu docx (markdown ter-render, tautan membuka browser) dan satu sheet (tabel sel dengan catatan batas bila terpotong); buka node tipe lain (judul + "Buka di Lark"); Salin link (clipboard terisi); Kirim ke agen dengan sesi terminal aktif (link tertempel tanpa Enter) dan tanpa sesi aktif (toast "Tidak ada sesi aktif"); restart: pane `lark-doc` dipulihkan dan isi diambil ulang; hapus sumber wiki lalu pane terbuka menampilkan "Sumber tidak ada di workspace ini"; jaringan gagal (mis. matikan jaringan) -> galat dengan coba lagi, tidak crash; ganti workspace mengosongkan pohon lama; `shikigamictl lark wiki list --source personal` dan `lark wiki get` mencetak hasil terbaca.
6. Aturan potong spike: baca `docs/lark/cli-findings.md`. `my_library` tidak bisa di-list -> Pribadi menampilkan keterangan `hiddenReason` (sudah ada di desain, 1.7); `sheets +cells-get` beda flag -> sheet menjadi judul + link (potong `FetchSheetCells`, lapor). Wiring hanya memverifikasi dan melaporkan; perbaikan kecil lintas fitur diizinkan hanya bila blokir tersambung, dan dicatat.

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- Fitur Go (`lark-wiki-data`, `lark-wiki-wiring` bagian ctl, Foundation): `go build ./...` dari root repo.
- Fitur frontend (`lark-wiki-tree-ui`, `lark-doc-pane`): `cd frontend && npm run build`, dan `npm run check` (svelte-check) tanpa galat baru.
- Wiring: keduanya, lalu aplikasi dijalankan (hanya pada tahap Wiring).

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain atau file wiring (`cmd_lark_wiki.go`).
- Jangan menyentuh: `internal/connector/*`, `internal/larkdata/runner.go`, `allowlist.go`, `errors.go`, `link.go`, `tables.go`, `tickets.go`, `ticketlink.go`, `testdata/`, `docs/lark/`, `cmd/shikigamictl/cmd_lark.go`, `cmd_lark_sources.go`, `cmd_lark_tickets.go` (tiket 01, 02, 03); `internal/workspace/*`, `wire_lark.go`, `wire_lark_link.go`, `wire_lark_tickets.go`, `wire_workspace.go`, `modules.go`; di frontend: `modules/lark/` selain path yang dimiliki (khususnya `sections/WikiSection.svelte`, `LarkPanel.svelte`, `LarkIcon.svelte`, `actions.ts`, `client.ts`, `store.ts`, `source.ts`, `index.ts`, `board/*`), `modules/files/*`, `modules/layout/*`, `lib/stores/layout.ts`; shell/rail milik `main-rail`.
- `lark-wiki-data` hanya boleh mengubah signature stub Foundation secara aditif; jangan mengubah nama/signature di bagian 2. Bila perlu menyimpang, catat di akhir plan fitur agar Wiring menyelaraskan.
- Working tree bisa punya perubahan tak terkait; jangan disentuh atau di-stage.
- Jangan `pkill` luas; matikan hanya proses yang diluncurkan sendiri.
- Jangan menjalankan perintah tulis `lark-cli`.
