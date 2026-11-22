# Plan: Data wiki dan dokumen larkdata dan IPC

## Metadata
- Tiket: `lark-connector-ticket-04-lark-wiki` (`docs/tickets/lark-connector-ticket-04-lark-wiki.md`)
- Fitur: `lark-wiki-data`
- ownedPaths: `internal/larkdata/wiki.go`, `internal/larkdata/doc.go`, `cmd/shikigamid/wire_lark_wiki.go`
- Depends on foundation: ya. `00-foundation.md` sudah membuat stub `wiki.go` dan `doc.go` (tipe, konstanta, signature). Plan ini menimpa isi kedua berkas itu dan membuat `wire_lark_wiki.go`. Bila stub belum ada, buat berkas dari kontrak overview 2.1 lalu lanjut.

Prasyarat: tiket 01 dan 02 sudah merge (`internal/larkdata/{runner,allowlist,errors}.go`, `internal/workspace/lark.go` dengan `Store.LarkSources`, `WikiSource`/`TicketSource`, `d.Provide("workspace.store", ...)` di `wire_workspace.go`). Bila `Store.LarkSources` atau tipe `WikiSource` belum ada, hentikan dan laporkan. Periksa nama persis di kode; bila beda dari overview, pakai nama di kode dan laporkan selisih.

## Tujuan
Menyediakan lapisan data wiki dan dokumen Lark yang ternormalisasi (read-only, lewat `larkdata.Run()`, selalu `--as user`) serta dua IPC daemon, `lark.wiki.list` dan `lark.wiki.get`, dengan resolusi sumber, host, dan pemetaan galat berkode berbahasa Indonesia. Dipakai oleh `WikiTree` dan `DocPane` (frontend) dan ctl debug (wiring). Detail keputusan (host, `sheet_id`, `hasMore`, `hidden`, tag markdown) ada di `overview.md` bagian 1 dan 2; plan ini hanya merinci cara membangunnya.

## Path yang dimiliki
HANYA tiga berkas ini yang boleh dibuat/diubah:
- `internal/larkdata/wiki.go`
- `internal/larkdata/doc.go`
- `cmd/shikigamid/wire_lark_wiki.go`

Jangan sentuh: `runner.go`, `allowlist.go`, `errors.go`, `link.go`, `tables.go`, `tickets.go`, `ticketlink.go`, `testdata/`, `internal/workspace/*`, `wire_lark.go`, `wire_lark_link.go`, `wire_lark_tickets.go`, `wire_workspace.go`, `modules.go`, `cmd/shikigamictl/*`, frontend, `docs/lark/`, `overview.md`. Jangan stage perubahan tak terkait di working tree.

## Kontrak
Disediakan (persis overview 2.1 dan 2.2, tidak boleh diubah selain aditif):
- `larkdata`: konstanta `wikiPageSize = 50`, `PersonalSpaceID = "my_library"`, `sheetMaxRows = 50`, `sheetMaxCols = 20`; tipe `WikiNode`, `WikiPage{Nodes, HasMore}`, `WikiNodeInfo{Title, ObjType, ObjToken, URL}`, `SheetCells{Cells, Truncated}`; fungsi `ListWikiNodes`, `ListWikiNodesPage`, `GetWikiNode(ctx, spaceID, nodeToken, host)`, `FetchDocMarkdown(ctx, objToken)`, `FetchSheetCells(ctx, objToken)`.
- IPC `lark.wiki.list` `{workspaceId, sourceId, parentToken?}` -> `{nodes, hasMore?, hidden?, hiddenReason?}` dan `lark.wiki.get` `{workspaceId, sourceId, nodeToken}` -> `{title, objType, url, markdown?, cells?, truncated?}`.
- Galat IPC: `ipc.NewError(<kode ipc>, "[<code>] <pesan>")`. Kode: `cli_missing`, `not_logged_in`, `permission`, `timeout`, `not_allowed`, `cli_error` (dari `*larkdata.Error`, kode ipc `ipc.ErrInternal`), `source_not_found` (`ipc.ErrNotFound`), `invalid_params` (`ipc.ErrInvalid`).
- Modul daemon: `RegisterModule("lark-wiki", ...)` di `init()`.

Dipakai (periksa di kode): `larkdata.Run(ctx, args...) ([]byte, error)` (mengembalikan `*larkdata.Error` terklasifikasi; helper `larkdata.AsError`, `larkdata.Code*`); `workspace.Store.LarkSources(wsID) (workspace.LarkConfig, error)` dengan `Wiki []WikiSource{ID, Name, SpaceID, RootToken, Host}` dan `Tickets []TicketSource{..., Host}`; sentinel `workspace.ErrNotFound`; `d.Lookup("workspace.store")`; `srv.Register(method, func(ctx context.Context, params json.RawMessage) (any, error))`; `ipc.NewError`, `ipc.ErrNotFound`, `ipc.ErrInvalid`, `ipc.ErrInternal`; `RegisterModule(name, func(srv *ipc.Server, d *Deps))`.

Bentuk JSON CLI (dari `docs/lark/cli-findings.md` bagian 3, jangan menebak; boleh cek fixture `internal/larkdata/testdata/`):
- `wiki +node-list`: `data.nodes[].{node_token,obj_token,obj_type,title,has_child}`, `data.has_more`.
- `wiki +node-get`: langsung di `data`: `{obj_token,obj_type,title,...}`.
- `docs +fetch`: `data.document.content`.
- `sheets +workbook-info`: `data.sheets[].{sheet_id,is_hidden,row_count,column_count}`.
- `sheets +cells-get`: `data.ranges[0].cells[][].value` (string, hilang pada sel kosong), `data.has_more`. Abaikan `cell_styles`, `warning_message`, `_notice`.

## Langkah

Aturan umum untuk semua kode Go di plan ini: argumen `Run` tidak boleh memuat `--as`, `--format`, `--json`, `--output`, `--jq`, `--yes`, `--force` (diblokir allowlist). Galat dari `Run` dikembalikan apa adanya (sudah `*larkdata.Error`). Bila `ctx` tanpa deadline, bungkus dengan `context.WithTimeout(ctx, ListTimeout)`. Pesan galat buatan sendiri memakai `&Error{Code: CodeCLI, Message: "..."}` berbahasa Indonesia. Tidak ada unit test, tidak ada dependensi baru; hanya pustaka standar. Nama helper privat berawalan `wiki`/`sheet`/`doc` (hindari bentrok dengan `tickets.go`, `link.go`, `tables.go`).

### 1. `internal/larkdata/wiki.go` (timpa stub Foundation)
Impor: `context`, `encoding/json`, `fmt`, `strconv`. Hapus komentar "Stub Foundation".

1. Konstanta dan tipe persis overview 2.1 (`wikiPageSize`, `PersonalSpaceID`, `WikiNode`, `WikiPage`, `WikiNodeInfo`, tag json camelCase).
2. Helper privat `wikiCtx(ctx context.Context) (context.Context, context.CancelFunc)`: bila `ctx.Deadline()` tidak ada, kembalikan `context.WithTimeout(ctx, ListTimeout)`; selain itu `ctx` dan no-op cancel.
3. Struct privat untuk decode:
   ```go
   type wikiListResp struct {
       Data struct {
           Nodes []struct {
               NodeToken string `json:"node_token"`
               ObjToken  string `json:"obj_token"`
               ObjType   string `json:"obj_type"`
               Title     string `json:"title"`
               HasChild  bool   `json:"has_child"`
           } `json:"nodes"`
           HasMore bool `json:"has_more"`
       } `json:"data"`
   }
   type wikiGetResp struct {
       Data struct {
           ObjToken string `json:"obj_token"`
           ObjType  string `json:"obj_type"`
           Title    string `json:"title"`
       } `json:"data"`
   }
   ```
4. `func ListWikiNodesPage(ctx context.Context, spaceID, parentToken string) (WikiPage, error)`:
   - `spaceID == ""` -> `WikiPage{Nodes: []WikiNode{}}, &Error{Code: CodeCLI, Message: "Space wiki kosong"}`.
   - args: `"wiki", "+node-list", "--space-id", spaceID, "--page-size", strconv.Itoa(wikiPageSize)`; tambahkan `"--parent-node-token", parentToken` HANYA bila `parentToken != ""` (akar space = flag tidak dikirim). Tanpa `--page-token`.
   - `out, err := Run(ctx2, args...)`; galat -> `WikiPage{Nodes: []WikiNode{}}, err`.
   - `json.Unmarshal` ke `wikiListResp`; gagal -> `&Error{Code: CodeCLI, Message: "Balasan daftar wiki tidak terbaca"}`.
   - Petakan tiap node ke `WikiNode{Token: node_token, Title, ObjType, ObjToken, HasChild}`. `Nodes` tidak pernah nil (inisialisasi `make([]WikiNode, 0, len(...))`). `HasMore = data.has_more`.
5. `func ListWikiNodes(ctx, spaceID, parentToken) ([]WikiNode, error)`: panggil `ListWikiNodesPage`, kembalikan `p.Nodes, err`.
6. `func GetWikiNode(ctx context.Context, spaceID, nodeToken, host string) (WikiNodeInfo, error)`:
   - `nodeToken == ""` -> `&Error{Code: CodeCLI, Message: "Token node kosong"}`.
   - args: `"wiki", "+node-get", "--node-token", nodeToken`; tambahkan `"--space-id", spaceID` hanya bila `spaceID != "" && spaceID != PersonalSpaceID` (overview 1.5: jangan kirim `my_library`).
   - decode `wikiGetResp` (node langsung di `data`); gagal decode -> `CodeCLI` "Balasan node wiki tidak terbaca".
   - `URL`: bila `host != ""` -> `fmt.Sprintf("https://%s/wiki/%s", host, nodeToken)`; selain itu `""`. Bersihkan `host` dari awalan `https://`/`http://` dan garis miring akhir sebelum dipakai (pakai `strings.TrimPrefix`/`TrimRight`; tambah impor `strings`).
   - Kembalikan `WikiNodeInfo{Title, ObjType, ObjToken, URL}`.

### 2. `internal/larkdata/doc.go` (timpa stub Foundation)
Impor: `context`, `encoding/json`, `fmt`, `regexp`, `strings`. Hapus komentar "Stub Foundation".

1. Konstanta `sheetMaxRows = 50`, `sheetMaxCols = 20`; tipe `SheetCells`.
2. `var docBlockTagRe = regexp.MustCompile(`^\s*</?(title|readonly-block)\b[^>]*>\s*$`)`.
3. `func FetchDocMarkdown(ctx context.Context, objToken string) (string, error)`:
   - `objToken == ""` -> `"", &Error{Code: CodeCLI, Message: "Token dokumen kosong"}`.
   - `Run(ctx2, "docs", "+fetch", "--doc", objToken, "--doc-format", "markdown")` (pakai `wikiCtx` dari `wiki.go`, satu paket).
   - decode `{data:{document:{content string}}}`; gagal -> `CodeCLI` "Balasan dokumen tidak terbaca".
   - Saring: pecah per baris (`strings.Split(content, "\n")`), buang baris yang cocok `docBlockTagRe` (baris yang HANYA berisi tag pembuka/penutup `title` atau `readonly-block`); tag lain dibiarkan. Sambung kembali dengan `"\n"`. Normalisasi `\r\n` ke `\n` lebih dulu. Kembalikan string (boleh kosong).
4. `func FetchSheetCells(ctx context.Context, objToken string) (SheetCells, error)`:
   - `objToken == ""` -> galat `CodeCLI` "Token spreadsheet kosong" dengan `SheetCells{Cells: [][]string{}}`.
   - Langkah A: `Run(ctx2, "sheets", "+workbook-info", "--spreadsheet-token", objToken)`; decode `data.sheets[].{sheet_id,is_hidden,row_count,column_count}`. Pilih sheet pertama dengan `is_hidden == false`. Bila tidak ada sheet terlihat -> `SheetCells{Cells: [][]string{}, Truncated: false}, nil`.
   - Langkah B: range `fmt.Sprintf("A1:%s%d", colName(sheetMaxCols), sheetMaxRows)` = `A1:T50` (tulis helper privat `sheetColName(n int) string` untuk 1..26 -> huruf; cukup `string(rune('A'+n-1))` karena maks 20, tapi tetap bungkus dalam fungsi). `Run(ctx2, "sheets", "+cells-get", "--spreadsheet-token", objToken, "--sheet-id", sheetID, "--range", rng)`. Jangan prefiks nama sheet pada range.
   - decode `data.ranges[0].cells` sebagai `[][]struct{ Value string `json:"value"` }` (sel tanpa `value` -> `""`; bila `value` ternyata bukan string pada fixture, ubah ke `json.RawMessage` dan konversi ke string; cek fixture `testdata/sheets-cells-get.json`) dan `data.has_more`. `ranges` kosong -> matriks kosong.
   - Padatkan: potong ke maks `sheetMaxRows` baris dan `sheetMaxCols` kolom; baris dibiarkan sepanjang kolom hasil (tanpa padding paksa), matriks tidak pernah nil.
   - `Truncated = row_count > sheetMaxRows || column_count > sheetMaxCols || has_more` (row/column count dari sheet terpilih di workbook-info).
   - Kembalikan `SheetCells`.

### 3. `cmd/shikigamid/wire_lark_wiki.go` (baru)
`package main`. Impor: `context`, `encoding/json`, `errors`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`, `.../internal/larkdata`, `.../internal/workspace`.

1. Konstanta lokal: `const wikiCodeSourceNotFound = "source_not_found"`, `wikiCodeInvalidParams = "invalid_params"`, `wikiPersonalID = "personal"`.
2. `func init() { RegisterModule("lark-wiki", func(srv *ipc.Server, d *Deps) { srv.Register("lark.wiki.list", wikiListHandler(d)); srv.Register("lark.wiki.get", wikiGetHandler(d)) }) }`.
3. Helper (semua berawalan `wiki`; JANGAN memakai nama `larkStore`, `linkStore`, `tickets*`):
   - `wikiErr(code int, tag, msg string) *ipc.Error` -> `ipc.NewError(code, "["+tag+"] "+msg)`.
   - `wikiMapErr(err error) error`: bila `*ipc.Error` kembalikan apa adanya; bila `larkdata.AsError(err)` ok -> `wikiErr(ipc.ErrInternal, string(le.Code), le.Message)`; selain itu `wikiErr(ipc.ErrInternal, "cli_error", err.Error())`. Pesan `permission` sudah berisi "Izin Lark kurang: ..." dari runner.
   - `wikiStore(d *Deps) (*workspace.Store, error)`: `d.Lookup("workspace.store")` + type assert `*workspace.Store`; gagal -> `wikiErr(ipc.ErrInternal, "cli_error", "workspace store tidak tersedia")`.
   - `type wikiTarget struct { SpaceID, RootToken, Host string; Personal bool }`.
   - `wikiResolve(d *Deps, wsID, sourceID string) (wikiTarget, error)`:
     - `wsID == "" || sourceID == ""` -> `wikiErr(ipc.ErrInvalid, wikiCodeInvalidParams, "workspaceId dan sourceId wajib diisi")`.
     - ambil store, `cfg, err := store.LarkSources(wsID)`; `errors.Is(err, workspace.ErrNotFound)` -> `wikiErr(ipc.ErrNotFound, wikiCodeSourceNotFound, "Sumber tidak ada di workspace ini")`; galat lain -> `wikiMapErr`.
     - `sourceID == "personal"`: `wikiTarget{SpaceID: larkdata.PersonalSpaceID, Personal: true, Host: wikiHostFallback(cfg)}`; JANGAN mencari di config.
     - selain itu cari `cfg.Wiki[i].ID == sourceID`; tidak ketemu -> `source_not_found` seperti di atas; ketemu -> `wikiTarget{SpaceID, RootToken, Host: src.Host}`.
   - `wikiHostFallback(cfg workspace.LarkConfig) string`: `Host` wiki pertama yang tidak kosong (`strings.TrimSpace`), lalu `Host` Base pertama (`cfg.Tickets`) yang tidak kosong, lalu `""`. Tidak ada host bawaan; jangan menebak domain.
4. `wikiListHandler(d *Deps) ipc.Handler`-compatible closure `func(ctx context.Context, params json.RawMessage) (any, error)`:
   - decode `struct{WorkspaceID string `json:"workspaceId"`; SourceID string `json:"sourceId"`; ParentToken string `json:"parentToken"`}`; gagal decode -> `invalid_params` ("Parameter tidak valid").
   - `t, err := wikiResolve(...)`; err -> kembalikan.
   - `parent := strings.TrimSpace(ParentToken)`; bila `parent == "" && t.RootToken != ""` -> `parent = t.RootToken` (personal punya `RootToken` kosong sehingga flag tidak dikirim).
   - `page, err := larkdata.ListWikiNodesPage(ctx, t.SpaceID, parent)`.
   - Bila `err != nil`:
     - `t.Personal` dan `larkdata.AsError(err)` dengan `Code` ∈ {`CodePermission`, `CodeCLI`} -> sukses dengan `map[string]any{"nodes": []larkdata.WikiNode{}, "hidden": true, "hiddenReason": "Wiki Pribadi tidak bisa ditampilkan: " + le.Message}`. (Pesan Indonesia + keterangan CLI.) Hanya berlaku untuk list pada `personal`, baik akar maupun anak.
     - selain itu `return nil, wikiMapErr(err)` (`cli_missing`, `not_logged_in`, `timeout`, `not_allowed` tetap galat keras; non-personal semua galat keras).
   - Sukses: `map[string]any{"nodes": page.Nodes, "hasMore": page.HasMore}`; pastikan `page.Nodes` non-nil.
5. `wikiGetHandler(d *Deps)` dengan closure sama:
   - decode `{workspaceId, sourceId, nodeToken}`; `nodeToken` kosong (setelah `TrimSpace`) -> `wikiErr(ipc.ErrInvalid, wikiCodeInvalidParams, "nodeToken wajib diisi")`. Urutan validasi: decode, `nodeToken` kosong, lalu `wikiResolve`.
   - `info, err := larkdata.GetWikiNode(ctx, t.SpaceID, nodeToken, t.Host)`; err -> `wikiMapErr`.
   - Hasil dasar `res := map[string]any{"title": info.Title, "objType": info.ObjType, "url": info.URL}` (`url` selalu string, boleh `""`).
   - `switch info.ObjType`: `"docx"` -> `md, err := larkdata.FetchDocMarkdown(ctx, info.ObjToken)`; err -> `wikiMapErr`; `res["markdown"] = md`. `"sheet"` -> `sc, err := larkdata.FetchSheetCells(ctx, info.ObjToken)`; `res["cells"] = sc.Cells; res["truncated"] = sc.Truncated`. Tipe lain: tidak memanggil CLI.
   - Tanpa publikasi event, tanpa tulis ke store, tanpa panic (tidak ada `panic`; semua galat dikembalikan).
6. Tipe handler: gunakan literal `func(ctx context.Context, params json.RawMessage) (any, error)` sesuai `srv.Register`; jika `ipc.Handler` ada sebagai tipe, boleh dipakai.

### 4. Verifikasi
- `gofmt -l internal/larkdata cmd/shikigamid` tidak menampilkan berkas milik fitur ini.
- `go build ./...` dari root repo lulus. Bila build gagal karena berkas di luar ownedPaths (mis. tiket 02 belum lengkap), jangan memperbaikinya; laporkan.
- `go vet ./internal/larkdata ./cmd/shikigamid` bersih untuk berkas fitur ini.
- Pemeriksaan statis cepat: `grep -n '"--as"\|"--format"\|--json' internal/larkdata/wiki.go internal/larkdata/doc.go` tidak menemukan apa pun; perintah yang dipakai hanya `wiki +node-list`, `wiki +node-get`, `docs +fetch`, `sheets +workbook-info`, `sheets +cells-get`.
- Opsional (hanya baca, bila `lark-cli` login): panggil hasil lewat daemon setelah wiring selesai; bukan syarat selesai.
- Catat di akhir pekerjaan penyimpangan dari overview bila ada (mis. perbedaan nama store/tipe tiket 02, `value` sel non-string).

## Perubahan
- Diubah (timpa stub Foundation): `internal/larkdata/wiki.go`
- Diubah (timpa stub Foundation): `internal/larkdata/doc.go`
- Dibuat: `cmd/shikigamid/wire_lark_wiki.go`

Tidak ada perubahan lain (tanpa `allowlist.go`, `modules.go`, frontend, ctl, `package.json`).

## Wiring
Yang diekspos untuk disambungkan nanti oleh `lark-wiki-wiring` dan fitur frontend (jangan disambungkan di sini):
- IPC `lark.wiki.list` dan `lark.wiki.get` terdaftar otomatis lewat `init()` modul `lark-wiki` (tanpa edit `modules.go`); dipakai `wikiTreeClient.ts`, `docClient.ts`, dan `cmd/shikigamictl/cmd_lark_wiki.go`.
- Bentuk hasil dan galat berkode sesuai overview 2.2; `hasMore` aditif pada list; `url` bisa `""` bila host tak diketahui.
- Fungsi `larkdata` (`ListWikiNodes`, `ListWikiNodesPage`, `GetWikiNode`, `FetchDocMarkdown`, `FetchSheetCells`) tersedia untuk pemanggil Go lain bila diperlukan.
- Plan ini tidak membuat ctl, tidak mengubah `WikiSection`, dan tidak mengubah `modules.go`.

## Selesai bila
- [ ] `go build ./...` dari root repo lulus (guard utama; tanpa unit test).
- [ ] `wiki.go` dan `doc.go` tidak lagi berisi stub "belum diimplementasi"; signature dan nama sesuai overview 2.1.
- [ ] `ListWikiNodesPage` mengirim `--page-size 50`, `--parent-node-token` hanya bila tak kosong, `Nodes` tidak pernah nil, `HasMore` dari `data.has_more`.
- [ ] `GetWikiNode` tidak mengirim `--space-id my_library`; `URL` kosong bila host kosong.
- [ ] `FetchDocMarkdown` menyaring baris tag `title`/`readonly-block` saja; `FetchSheetCells` memakai `workbook-info` untuk `sheet_id` (sheet pertama tak tersembunyi), range `A1:T50`, matriks maks 50 x 20, `Truncated` sesuai aturan, hanya `value` sel dipakai.
- [ ] `lark.wiki.list` untuk `personal` memberi `hidden: true` + `hiddenReason` bila galat `permission`/`cli_error`; galat lain tetap galat keras; non-personal semua galat keras.
- [ ] `sourceId` tak dikenal/workspace tak ada -> `[source_not_found] Sumber tidak ada di workspace ini` (`ipc.ErrNotFound`); parameter salah/`nodeToken` kosong -> `[invalid_params] ...` (`ipc.ErrInvalid`); galat `larkdata` -> `[<code>] <pesan Indonesia>`; tanpa panic.
- [ ] Hanya perintah baca dari allowlist yang dipanggil lewat `larkdata.Run()`; tidak ada `--as`/`--format` manual.
- [ ] `git status` menunjukkan hanya tiga path milik fitur ini yang berubah oleh pekerjaan ini; `overview.md` dan berkas fitur lain tidak tersentuh.
