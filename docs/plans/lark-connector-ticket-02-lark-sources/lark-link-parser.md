# Plan: Parser link Base dan wiki

## Metadata
- Tiket: `lark-connector-ticket-02-lark-sources` (`docs/tickets/lark-connector-ticket-02-lark-sources.md`)
- Fitur: `lark-link-parser`
- Module Go: `github.com/NooBeeID/shikigami`
- ownedPaths:
  - `internal/larkdata/link.go` (baru)
  - `internal/larkdata/tables.go` (baru)
  - `cmd/shikigamid/wire_lark_link.go` (baru)
- Depends on foundation: ya. Memakai `workspace.Store.LarkSources` (stub dari Foundation, implementasi nyata dari `lark-config-ipc`) dan tipe `workspace.LarkConfig`/`TicketSource`; serta `d.Provide("workspace.store", store)` di `wire_workspace.go` (Foundation F2 dan F3). Dari tiket 01: `larkdata.Run`, `larkdata.Error`, `larkdata.ErrorCode`, `larkdata.AsError`, `larkdata.Code*`.
- Kontrak bersama: `docs/plans/lark-connector-ticket-02-lark-sources/overview.md` bagian 2.2 sampai 2.5. Jangan ubah overview.

## Tujuan
Mengurai link Lark yang ditempel user (Base atau wiki, termasuk `/wiki/` yang berisi bitable) menjadi `ParsedLink` dinormalisasi, menyediakan `ListTables` dan `ListViews` untuk Base, resolusi nama default Base, dan IPC `lark.link.parse`, `lark.tables.list`, `lark.views.list`. Semua panggilan CLI hanya lewat `larkdata.Run()`.

## Path yang dimiliki
HANYA path ini yang boleh disentuh:
- `internal/larkdata/link.go` (baru)
- `internal/larkdata/tables.go` (baru)
- `cmd/shikigamid/wire_lark_link.go` (baru)

Jangan sentuh: `internal/larkdata/runner.go`, `allowlist.go`, `errors.go`, `testdata/`, `docs/lark/` (tiket 01); `internal/workspace/*`, `cmd/shikigamid/wire_lark.go`, `wire_workspace.go`, `modules.go`; `internal/larkdata/tickets.go`, `ticketlink.go` (tiket 03), `wiki.go`, `doc.go` (tiket 04); semua `frontend/*`, `cmd/shikigamictl/*`, `app/*`, `internal/provider/*`; perubahan git tak terkait. Jangan menjalankan perintah tulis `lark-cli`. Jangan `pkill` luas. Tanpa unit test (jangan membuat `_test.go`). Jangan `git add`/commit kecuali diinstruksikan langkah implementasi.

## Kontrak
Disediakan (signature persis, overview 2.5):
```go
// package larkdata, link.go
const CodeInvalidLink ErrorCode = "invalid_link"
type ParsedLink struct {
    Kind        string `json:"kind"`                  // "base" | "wiki"
    Host        string `json:"host"`
    BaseToken   string `json:"baseToken,omitempty"`
    TableID     string `json:"tableId,omitempty"`
    ViewID      string `json:"viewId,omitempty"`
    SpaceID     string `json:"spaceId,omitempty"`
    NodeToken   string `json:"nodeToken,omitempty"`
    DefaultName string `json:"defaultName,omitempty"`
}
func ParseLink(ctx context.Context, link string) (ParsedLink, error)

// package larkdata, tables.go
type Table struct { ID string `json:"id"`; Name string `json:"name"` }
type View  struct { ID string `json:"id"`; Name string `json:"name"`; Type string `json:"type,omitempty"` }
func ListTables(ctx context.Context, baseToken, host string) ([]Table, error)
func ListViews(ctx context.Context, baseToken, tableID string) ([]View, error)
```
Tambahan privat/semi-publik yang boleh dibuat di berkas sendiri: `func DefaultBaseName(ctx context.Context, baseToken, host string) (string, error)` (di `tables.go`), serta helper privat parsing JSON.

IPC (didaftarkan hanya di `wire_lark_link.go`, `RegisterModule("lark-link", ...)`; hasil json camelCase):
- `lark.link.parse {link}` -> `ParsedLink`.
- `lark.tables.list {workspaceId, sourceId}` -> `{tables:[{id,name}]}` (array kosong `[]`, bukan null).
- `lark.views.list {workspaceId, sourceId, tableId}` -> `{views:[{id,name,type?}]}`.

Dipakai:
- Tiket 01: `larkdata.Run(ctx, args...) ([]byte, error)` (sudah menambah `--as user` dan `--format json`, timeout 20 detik, menolak flag `--as`/`--format`/`--json` dari pemanggil; JSON valid terjamin pada sukses), `larkdata.AsError(err) (*Error, bool)`, `Error{Code, Message}`, kode `CodeCLIMissing`, `CodeNotLoggedIn`, `CodePermission`, `CodeTimeout`, `CodeNotAllowed`, `CodeCLI`. Tipe `ErrorCode` dan `*Error` milik `errors.go`; fitur ini hanya menambah konstanta `CodeInvalidLink` di `link.go`.
- Foundation/`lark-config-ipc`: `(*workspace.Store).LarkSources(wsID string) (workspace.LarkConfig, error)` (slice tak pernah nil; workspace tak ada -> galat yang dibungkus `workspace.ErrNotFound`). `d.Lookup("workspace.store")` mengembalikan `*workspace.Store` (modul berjalan alfabetis; `lark-link` sebelum `workspace`, jadi Lookup HARUS malas, di dalam handler, bukan saat wire).
- `ipc.NewError(code int, msg string)`, `ipc.ErrInvalid`, `ipc.ErrNotFound`, `ipc.ErrInternal`; `srv.Register(method, func(ctx, params json.RawMessage) (any, error))`.

Konvensi galat (overview 2.4): semua galat handler berbentuk `ipc.NewError(<kode ipc>, "[<code>] <pesan>")`. Pemetaan kode ipc: `invalid_link`, `invalid_params` -> `ipc.ErrInvalid`; `not_found` -> `ipc.ErrNotFound`; galat `larkdata` (kode apa pun) -> `ipc.ErrInternal`, pesan = `larkdata.Error.Message` dengan awalan `[<Code>] `. Galat non-`larkdata` tak terduga -> `ipc.ErrInternal` dengan `[cli_error] <pesan>`.

Pesan tetap: `invalid_link` "Link tidak dikenali"; `not_found` "Sumber tidak ditemukan"; `invalid_params` "Parameter tidak lengkap: <nama>".

## Langkah

### 1. Cek temuan CLI (sebelum menulis kode parsing)
Bentuk JSON `lark-cli` belum diketahui pasti; tiket 01 (spike) menghasilkan fixture dan `docs/lark/cli-findings.md`. Di worktree/branch kerja:
1. Bila `docs/lark/cli-findings.md` dan `internal/larkdata/testdata/{base-table-list,base-view-list,wiki-node-get,wiki-node-get-bitable}.json` ada: BACA, dan jadikan acuan nama field (path daftar, `table_id`/`view_id`, `view_type`, `obj_type`, `obj_token`, `space_id`, `title`, nama flag `+node-get`, ada tidaknya nama Base di `+table-list`). Aturan pemotongan: bila `+node-get` tidak memberi `obj_token` untuk bitable, link `/wiki/` bitable ditolak (lihat langkah 3 butir 5) dan dicatat di "Catatan" akhir.
2. Bila berkas itu belum ada, pakai nilai awal di bawah dan tulis parser TOLERAN (langkah 2, helper `pickString`/`findList`), lalu catat di "Catatan" akhir plan bagian mana yang belum terverifikasi. Jangan menjalankan `lark-cli` selain perintah baca singkat bila ingin memastikan bentuk (opsional, `lark-cli` sudah login di mesin ini).
3. Bila `base +view-list` atau `wiki +node-get` tidak ada di allowlist `internal/larkdata/allowlist.go` (cek `check`), JANGAN mengedit file itu: `Run()` akan mengembalikan `CodeNotAllowed`; biarkan galat itu diteruskan dan laporkan sebagai ketergantungan ke tiket 01 di "Catatan".

Nilai awal flag: `base +table-list --base-token <T>`; `base +view-list --base-token <T> --table-id <TB>`; `wiki +node-get --token <node_token>`. Jangan menambahkan `--as`, `--format`, `--json` (ditambah `Run()`).

### 2. `internal/larkdata/tables.go` (package `larkdata`)
Impor: `context`, `encoding/json`, `strings`.
1. Tipe `Table` dan `View` persis kontrak.
2. Helper privat tolerans JSON (dipakai juga `link.go`, jadi definisikan di SINI sekali; `link.go` hanya memakainya):
   - `func decodeJSON(b []byte) (any, error)`: `json.Unmarshal` ke `any`; galat -> `&Error{Code: CodeCLI, Message: "Balasan Lark tidak terbaca"}`.
   - `func pickString(m map[string]any, keys ...string) string`: kembalikan nilai string tak kosong pertama dari `keys` (cek juga bila nilainya `float64`/angka besar: ubah ke string tanpa notasi ilmiah via `json.Number`-sejenis `strconv`; `space_id` numerik dapat muncul sebagai string, utamakan string).
   - `func findObject(v any, keys ...string) map[string]any`: bila `v` objek yang punya salah satu `keys` bernilai objek, kembalikan objek itu (mis. pembungkus `data`, `node`); bila `v` objek yang tidak memiliki pembungkus, kembalikan `v` sendiri; mengikuti satu atau dua tingkat pembungkus `data`.
   - `func findList(v any, keys ...string) []map[string]any`: telusuri maksimal dua tingkat (`v`, lalu `v["data"]`) mencari array di bawah salah satu `keys` (mis. `tables`, `items`, `views`); bila `v` sendiri array, pakai itu; elemen non-objek diabaikan. Tidak pernah panic; mengembalikan slice kosong non-nil bila tak ketemu.
3. `func ListTables(ctx context.Context, baseToken, host string) ([]Table, error)`:
   - `baseToken` kosong -> `&Error{Code: CodeCLI, Message: "Base token kosong"}`. `host` hanya dipakai untuk konsistensi tanda tangan (dokumentasikan di komentar: `+table-list` tidak butuh host); jangan dipakai sebagai flag.
   - `out, err := Run(ctx, "base", "+table-list", "--base-token", baseToken)`; `err != nil` -> `return nil, err` (sudah `*Error`).
   - `findList(v, "tables", "items", "data")`; tiap elemen -> `Table{ID: pickString(e, "table_id", "id"), Name: pickString(e, "name", "table_name", "title")}`; lewati elemen tanpa ID. Hasil non-nil (`[]Table{}`).
4. `func ListViews(ctx context.Context, baseToken, tableID string) ([]View, error)`:
   - Salah satu argumen kosong -> `&Error{Code: CodeCLI, Message: "Base token atau table id kosong"}`.
   - `Run(ctx, "base", "+view-list", "--base-token", baseToken, "--table-id", tableID)`; normalisasi: `View{ID: pickString(e, "view_id", "id"), Name: pickString(e, "view_name", "name", "title"), Type: pickString(e, "view_type", "type")}`; lewati elemen tanpa ID; hasil non-nil.
5. `func DefaultBaseName(ctx context.Context, baseToken, host string) (string, error)`: resolusi nama default Base dari `+table-list` (hanya satu panggilan `Run`; tidak ada perintah info Base di allowlist). Aturan: (a) bila JSON respons memiliki nama Base di level objek (kunci `base_name`, `app_name`, atau `name` pada objek `base`/`app` di dalam respons), pakai itu; (b) selain itu nama tabel pertama; (c) selain itu literal `"Base"`. Implementasi: jangan memanggil `Run` dua kali; buat helper privat `listTablesRaw(ctx, baseToken) (v any, tables []Table, err error)` yang dipakai `ListTables` dan `DefaultBaseName`. Galat dikembalikan apa adanya (pemanggil memutuskan).

### 3. `internal/larkdata/link.go` (package `larkdata`)
Impor: `context`, `net/url`, `strings`.
1. `const CodeInvalidLink ErrorCode = "invalid_link"` dan `func invalidLink() *Error { return &Error{Code: CodeInvalidLink, Message: "Link tidak dikenali"} }`. Tipe `ParsedLink` persis kontrak.
2. `func ParseLink(ctx context.Context, link string) (ParsedLink, error)`:
   - `strings.TrimSpace`; kosong -> `invalidLink()`.
   - `url.Parse`; galat, atau skema bukan `http`/`https` (bandingkan huruf kecil), atau `u.Hostname() == ""` -> `invalidLink()`. `host` = `u.Host` apa adanya (pertahankan port bila ada; jangan diubah huruf besar/kecilnya selain apa yang ditempel; tidak ada daftar domain Lark karena tenant bisa `*.larksuite.com`, `*.feishu.cn`, dst.).
   - Pecah `u.Path` per `/` tanpa elemen kosong. Cari segmen penanda pertama `base` atau `wiki` yang diikuti segmen token tak kosong. Toleran terhadap awalan path lain (mis. `/app/...` atau awalan bahasa) dan terhadap fragmen serta query ekstra (abaikan). Tidak ada penanda -> `invalidLink()`.
3. Cabang `base`: `baseToken` = segmen sesudah `base`; `q := u.Query()`; `TableID = q.Get("table")`, `ViewID = q.Get("view")`. Bila `table`/`view` tak ada di query, periksa fragmen (`u.Fragment` diurai sebagai query bila memuat `=`) sebagai fallback. Balikan `ParsedLink{Kind:"base", Host, BaseToken, TableID, ViewID}`. Lalu resolusi nama default: `name, err := DefaultBaseName(ctx, baseToken, host)`. Penanganan galat: bila `AsError(err)` berkode `CodeCLIMissing`, `CodeNotLoggedIn`, atau `CodePermission`, kembalikan galat itu (panel menampilkan ajakan ke Pengaturan > Connector atau pesan izin); galat lain (timeout, `cli_error`, `not_allowed`) -> abaikan, `DefaultName = "Base"`. Sukses -> `DefaultName = name`.
4. Cabang `wiki`: `nodeToken` = segmen sesudah `wiki`. Panggil `out, err := Run(ctx, "wiki", "+node-get", "--token", nodeToken)`; `err != nil` -> kembalikan galat apa adanya (kode connector diteruskan; bila token tak ada CLI mungkin mengembalikan `cli_error`, tidak diubah). Uraikan: `n := findObject(v, "node")` lalu `spaceID := pickString(n, "space_id")`, `objType := strings.ToLower(pickString(n, "obj_type"))`, `objToken := pickString(n, "obj_token")`, `title := pickString(n, "title")`.
5. Percabangan hasil wiki:
   - `objType == "bitable"` (terima juga `"base"`): bila `objToken == ""` -> `&Error{Code: CodeCLI, Message: "Wiki berisi Base tetapi token Base tidak tersedia"}` (aturan pemotongan, pesan jelas); selain itu `ParsedLink{Kind:"base", Host: host, BaseToken: objToken, DefaultName: title}`. Bila `title == ""` panggil `DefaultBaseName(ctx, objToken, host)` dengan penanganan galat sama seperti butir 3, fallback `"Base"`. Query `table`/`view` pada link wiki dipertahankan (`TableID`/`ViewID`) bila ada.
   - selain itu: `spaceID == ""` -> `&Error{Code: CodeCLI, Message: "Balasan Lark tidak terbaca"}`; selain itu `ParsedLink{Kind:"wiki", Host: host, SpaceID: spaceID, NodeToken: nodeToken, DefaultName: title}`. Bila `title == ""`, `DefaultName = "Wiki"`. `NodeToken` = token dari URL (node_token yang diminta), dijadikan `rootToken` oleh pemanggil.
6. Link space tanpa node (mis. `/wiki/space/<spaceId>`) TIDAK didukung kecuali spike membuktikan format pastinya: segmen sesudah `wiki` bernilai `space` -> kembalikan `invalidLink()` (jangan salah dianggap node token `space`). Jika `cli-findings.md` mendokumentasikan format link space, tambahkan cabang dan catat di "Catatan".
7. Tidak ada `panic`: semua indeks slice dijaga panjangnya; semua galat dikembalikan sebagai `*Error` (hindari mengembalikan `*Error` nil terbungkus di `error`: gunakan `return ParsedLink{}, invalidLink()` hanya pada cabang galat).

### 4. `cmd/shikigamid/wire_lark_link.go` (package `main`)
Impor: `context`, `encoding/json`, `bytes`, `errors`, `strings`, `internal/ipc`, `internal/larkdata`, `internal/workspace`.
1. `func init() { RegisterModule("lark-link", func(srv *ipc.Server, d *Deps) { ... }) }` mendaftarkan tiga method. Jangan memakai nama helper dari `wire_lark.go` (paket `main` bersama): semua helper berawalan `link` (`linkStore`, `linkErr`, `linkDecode`, `linkSource`).
2. `func linkStore(d *Deps) (*workspace.Store, error)`: `v, ok := d.Lookup("workspace.store")`; type assert `*workspace.Store`; gagal -> `ipc.NewError(ipc.ErrInternal, "[cli_error] workspace store tidak tersedia")`. Dipanggil di dalam handler, tidak saat wire.
3. `func linkDecode(params json.RawMessage, v any) error`: params kosong/`null` dianggap `{}`; galat `Unmarshal` -> `ipc.NewError(ipc.ErrInvalid, "[invalid_params] params tidak valid")`.
4. `func linkErr(err error) error`: `*larkdata.Error` (via `larkdata.AsError`) -> `ipc.NewError(ipc.ErrInvalid, "[invalid_link] Link tidak dikenali")` bila `Code == larkdata.CodeInvalidLink` (gunakan `e.Message` agar tetap sinkron), selain itu `ipc.NewError(ipc.ErrInternal, "["+string(e.Code)+"] "+e.Message)`. Galat lain -> `ipc.NewError(ipc.ErrInternal, "[cli_error] "+err.Error())`. `err == nil` -> `nil`. Bila `err` sudah `*ipc.Error`, kembalikan apa adanya.
5. `func linkSource(d *Deps, wsID, sourceID string) (workspace.TicketSource, error)`: validasi `wsID`/`sourceID` tak kosong (`[invalid_params] Parameter tidak lengkap: workspaceId` / `sourceId`, `ipc.ErrInvalid`); `store, err := linkStore(d)`; `cfg, err := store.LarkSources(wsID)`; `errors.Is(err, workspace.ErrNotFound)` -> `ipc.NewError(ipc.ErrNotFound, "[not_found] Sumber tidak ditemukan")`, galat lain -> `ipc.ErrInternal` `[cli_error] <pesan>`; cari di `cfg.Tickets` yang `ID == sourceID`; tak ketemu -> `ipc.NewError(ipc.ErrNotFound, "[not_found] Sumber tidak ditemukan")`.
6. Handler `lark.link.parse`: params `struct{ Link string `json:"link"` }`; `link` kosong (setelah `TrimSpace`) -> `[invalid_link] Link tidak dikenali`; `p, err := larkdata.ParseLink(ctx, params.Link)`; kembalikan `p` (struct `ParsedLink`; json camelCase dari tag). Galat via `linkErr`.
7. Handler `lark.tables.list`: params `{workspaceId, sourceId}`; `src := linkSource(...)`; `tables, err := larkdata.ListTables(ctx, src.BaseToken, src.Host)`; hasil `map[string]any{"tables": tables}` (pastikan `tables` non-nil; `ListTables` sudah menjamin, tambahkan guard `if tables == nil { tables = []larkdata.Table{} }`).
8. Handler `lark.views.list`: params `{workspaceId, sourceId, tableId}`; `tableId` kosong -> `[invalid_params] Parameter tidak lengkap: tableId`; `views, err := larkdata.ListViews(ctx, src.BaseToken, tableId)`; hasil `map[string]any{"views": views}` (guard non-nil).
9. Timeout: 20 detik sudah ditegakkan `Run()` (tidak perlu `context.WithTimeout` tambahan; teruskan `ctx` handler).
10. Tanpa penerbitan event (fitur ini read-only). Tanpa `exec` langsung; tanpa impor `os/exec`.

### 5. Verifikasi
- `gofmt -l internal/larkdata cmd/shikigamid` tidak menampilkan berkas milik fitur ini.
- `go vet ./internal/larkdata/ ./cmd/shikigamid/` (abaikan temuan di berkas milik orang lain).
- `go build ./...` lulus.
- Baca ulang kode: tidak ada `os/exec`; tidak ada panggilan CLI selain `Run()`; tidak ada `panic`; semua balikan galat `*Error` hanya pada cabang non-nil.
- Opsional (hanya baca, tanpa menulis): bila `lark-cli` login, uji manual lewat `shikigamictl` setelah daemon dijalankan oleh tahap Wiring, bukan di sini.

## Perubahan
- Baru: `internal/larkdata/link.go`, `internal/larkdata/tables.go`, `cmd/shikigamid/wire_lark_link.go`.
- Tidak ada berkas lain diubah.

## Wiring
Fitur ini mendaftarkan modul lewat `init()` di `wire_lark_link.go` (`RegisterModule("lark-link", ...)`), jadi `modules.go` TIDAK perlu diubah. Yang diekspos untuk langkah wiring (`lark-sources-wiring`), jangan disambungkan di sini:
- IPC `lark.link.parse`, `lark.tables.list`, `lark.views.list` (dipanggil `client.ts` milik `lark-panel-ui` dan `cmd_lark_sources.go` ctl: `lark sources add` memanggil `lark.link.parse`).
- `larkdata.ParseLink`, `ListTables`, `ListViews`, `DefaultBaseName`, `CodeInvalidLink`, serta helper privat `decodeJSON`/`pickString`/`findObject`/`findList` di `tables.go` yang dapat dipakai ulang tiket 03/04 (sama paket).
- Wiring memverifikasi: nama modul `lark-link` unik, `go build ./...`, tiga method terjawab oleh daemon, `[invalid_link] Link tidak dikenali` muncul untuk link tak valid, dan `lark.tables.list`/`lark.views.list` memetakan galat ke kode connector yang sama. Bila `LarkSources` masih stub (belum diganti `lark-config-ipc`), `tables.list`/`views.list` akan mengembalikan `[not_found]`; ini diharapkan sampai fitur itu digabung.

## Selesai bila
Guard: `go build ./...` lulus (tanpa unit test).
- `ParseLink` mengurai link Base dengan/tanpa `table` dan `view` menjadi `host`, `baseToken`, `tableId?`, `viewId?`, serta `defaultName`.
- Link `/wiki/<token>`: `obj_type` bitable -> `kind:"base"` dengan `baseToken = obj_token` dan `host` link asli; selain itu `kind:"wiki"` dengan `spaceId`, `nodeToken`, `defaultName = judul`.
- Link tak dikenal, bukan http(s), atau kosong -> `*Error` `CodeInvalidLink` "Link tidak dikenali" (tanpa panic); lewat IPC menjadi `[invalid_link] Link tidak dikenali`.
- `ListTables` dan `ListViews` mengembalikan daftar dinormalisasi non-nil; `lark.tables.list`/`lark.views.list` mengambil `baseToken` dan `host` dari sumber lewat `store.LarkSources`.
- Galat CLI diteruskan dengan kode connector yang sama dalam format `[code] pesan`.
- Semua panggilan CLI lewat `larkdata.Run()`; tidak ada `exec` langsung; `runner.go`, `allowlist.go`, `errors.go` tidak diubah.
- IPC terdaftar hanya di `wire_lark_link.go`; tidak ada berkas di luar ownedPaths yang berubah.

## Catatan
(Isi saat implementasi bila menyimpang: nama field JSON yang ternyata berbeda dari tebakan awal, ketergantungan allowlist ke tiket 01 untuk `base +view-list`/`wiki +node-get`, format link space yang didukung atau tidak, keputusan pemotongan link `/wiki/` bitable. Catatan perencanaan: saat plan ini ditulis `internal/larkdata/`, `docs/lark/` dan fixture belum ada di pohon kerja; bentuk JSON CLI belum terverifikasi sehingga parser sengaja toleran. Tidak ada perintah info Base di allowlist awal, jadi nama default Base berasal dari `+table-list` atau judul node wiki.)
