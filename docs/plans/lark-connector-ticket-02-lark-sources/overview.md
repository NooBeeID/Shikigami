# Overview: lark-connector-ticket-02-lark-sources (Rail Lark dan sumber per workspace)

Sumber: tiket `docs/tickets/lark-connector-ticket-02-lark-sources.md` dan spec `docs/superpowers/specs/2026-11-22-09-lark-connector-design.md`. Dokumen ini adalah sumber kebenaran kontrak bersama; plan per fitur hanya merinci langkah kerja. Module Go: `github.com/NooBeeID/shikigami`. Tiket ini berjalan di atas `lark-connector-ticket-01-lark-connector` (connector `lark`, `larkdata.Run()`, `docs/lark/cli-findings.md`) dan `main-rail-ticket-01-main-rail` (`registerRailItem`).

## 1. Ringkasan epic dan urutan eksekusi

Epic menambah item rail **Lark** dengan `LarkPanel` (bagian Tiket dan Wiki). Sumber Base dan wiki disimpan per workspace di `WorkspaceConfig.Lark` lewat IPC per item (`lark.sources.get/add/remove/rename`), ditambahkan dengan menempel link (`lark.link.parse` mengurai host/token/table/view atau space/node). Epic juga meletakkan fondasi pane `lark-board` dan `lark-doc` di sistem layout (registrasi, persist field referensi, pane tak dikenal, sumber tak ada), helper `copyLink`/`sendToAgent`, dan `shikigamictl lark sources`. Isi pane masih placeholder "Segera hadir" yang ditimpa tiket 03 dan 04.

Urutan:
1. **Foundation** (`00-foundation.md`, `hasFoundation=true`): tipe `LarkConfig` + stub `Store` Lark di paket `workspace`, penyediaan `workspace.store` di daemon, dan penambahan tipe-saja field Lark pada `Leaf`/`NewLeaf`. Tanpa ini fitur Go saling menunggu kode dan fitur frontend gagal type-check.
2. **Fitur paralel**: `lark-config-ipc`, `lark-link-parser`, `lark-pane-foundation`, `lark-panel-ui`. Masing-masing hanya menyentuh `ownedPaths`-nya.
3. **Wiring**: `lark-sources-wiring` (terakhir): rail item, ctl `lark sources`, verifikasi.

Temuan penting dari kode yang ada (dasar keputusan di bawah):
- Modul daemon terdaftar lewat `init()` + `RegisterModule(name, wire)` (`cmd/shikigamid/modules.go`); `modules.go` TIDAK perlu diubah. Modul frontend dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts', { eager: true })` di `frontend/src/main.ts`; TIDAK perlu baris impor baru. Subcommand ctl didaftarkan lewat `init()` (`RegisterCommand`, dan `registerLarkSub` dari tiket 01).
- `*workspace.Store` dibuat di dalam closure `wire_workspace.go` dan belum disediakan ke modul lain; Foundation menambah satu baris `d.Provide("workspace.store", store)`. Membuat `Store` kedua akan saling menimpa data, jangan.
- `config.set` mengubah `WorkspaceConfig` yang diambil dari `GetConfig` lalu `SetWorkspaceConfig(wsID, wc)` (ganti seluruh `Config`); karena `wc` sudah memuat `Lark`, field itu terbawa. Tidak ada perubahan `config.set`.
- Galat IPC sampai ke JS hanya sebagai string pesan (`ipc.Error.Error()` = `"<message> (kode <n>)"`, `Data` hilang di jembatan Wails). Karena itu kode galat Lark dibawa di awal pesan (bagian 2.4).
- Event perubahan config: topik `workspace.changed` dengan payload `{kind:'configChanged', workspaceId, workspace}` (publisher di `internal/workspace/ipc.go`, konsumen contoh `lib/stores/init.ts`).

## 2. Kontrak bersama

### 2.1 Config Go (`internal/workspace/types.go`, dibuat oleh Foundation, persis tiket)
```go
// WorkspaceConfig: tambah satu field
Lark *LarkConfig `json:"lark,omitempty"`

type LarkConfig struct {
    Tickets []TicketSource `json:"tickets"`
    Wiki    []WikiSource   `json:"wiki"`
}
type TicketSource struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    BaseToken string `json:"baseToken"`
    Host      string `json:"host"`
    TableID   string `json:"tableId,omitempty"`
    ViewID    string `json:"viewId,omitempty"`
}
type WikiSource struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    SpaceID   string `json:"spaceId"`
    RootToken string `json:"rootToken,omitempty"`
    Host      string `json:"host"`
}
```
Wiki pribadi tidak disimpan: `spaceId` `my_library`, `sourceId` literal `"personal"`. Tanpa token.

### 2.2 `Store` Lark (`internal/workspace/lark.go`; stub oleh Foundation, implementasi oleh `lark-config-ipc`)
Metode pada `*Store` (mengikuti konvensi `GetConfig`/`SetWorkspaceConfig`), semuanya mengambil `s.mu`, menyalin dalam (deep copy slice `Tickets`/`Wiki`) sebelum mengubah, menulis lewat `persistLocked` (sudah atomik):
```go
func (s *Store) LarkSources(wsID string) (LarkConfig, error)                          // slice tak pernah nil
func (s *Store) AddLarkTicketSource(wsID string, src TicketSource) (TicketSource, error) // ID = newID() (ULID) dibuat di sini
func (s *Store) AddLarkWikiSource(wsID string, src WikiSource) (WikiSource, error)
func (s *Store) RemoveLarkSource(wsID, sourceID string) error
func (s *Store) RenameLarkSource(wsID, sourceID, name string) error
```
Galat memakai sentinel yang sudah ada: workspace/sumber tak ada -> `ErrNotFound`; nama kosong/field wajib kosong -> `ErrInvalid`; duplikat (`baseToken`, atau `spaceId`+`rootToken`) -> `ErrConflict`. Dibungkus `fmt.Errorf("%w: pesan Indonesia", ErrX)`. `internal/workspace` tidak mengimpor `larkdata`.

### 2.3 Daemon: akses store dan publikasi event
- `cmd/shikigamid/wire_workspace.go` (Foundation): `d.Provide("workspace.store", store)` setelah `workspace.Register`.
- Modul Lark mengambil store secara malas saat handler dipanggil (modul berjalan alfabetis, `lark` sebelum `workspace`):
```go
func larkStore(d *Deps) (*workspace.Store, error) // d.Lookup("workspace.store") + type assert; gagal -> ipc.NewError(ipc.ErrInternal, "workspace store tidak tersedia")
```
Helper itu dimiliki `wire_lark.go`; `wire_lark_link.go` menulis helper sendiri (nama berbeda, mis. `linkStore`) agar tidak saling impor dalam paket `main`.
- Setelah tiap mutasi, handler memanggil `store.Get(wsID)` lalu `srv.Publish("workspace.changed", map[string]any{"kind": "configChanged", "workspaceId": wsID, "workspace": &ws})` (bentuk sama persis dengan `publish()` di `internal/workspace/ipc.go`).
- Modul didaftarkan di `init()` file masing-masing: `RegisterModule("lark", ...)` di `wire_lark.go`, `RegisterModule("lark-link", ...)` di `wire_lark_link.go`.

### 2.4 Konvensi galat IPC Lark
Semua galat handler Lark dikembalikan sebagai `ipc.NewError(<kode ipc>, "[<code>] <pesan Indonesia>")`. Frontend mengurai dari string galat dengan regex `^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$` menjadi `{code, message}`; tanpa awalan, `code='unknown'` dan pesan apa adanya.
- Kode dari `larkdata` (tiket 01): `cli_missing`, `not_logged_in`, `permission`, `timeout`, `not_allowed`, `cli_error`. Pesan mengikuti `larkdata.Error.Message`.
- Kode baru tiket ini: `invalid_link` ("Link tidak dikenali"), `not_found` ("Sumber tidak ditemukan"), `invalid_name` ("Nama tidak boleh kosong"), `duplicate` ("Sumber sudah ada"), `invalid_params`.
- Pemetaan kode ipc (angka): `not_found` -> `ipc.ErrNotFound`, `duplicate` -> `ipc.ErrConflict`, `invalid_*`/`invalid_link` -> `ipc.ErrInvalid`, galat larkdata -> `ipc.ErrInternal`.
- `invalid_link` didefinisikan di `internal/larkdata/link.go` sebagai `const CodeInvalidLink ErrorCode = "invalid_link"` (tipe `ErrorCode` dan `*Error` milik tiket 01 di `errors.go`; fitur ini hanya menambah konstanta di berkas sendiri). Mengembalikan `&Error{Code: CodeInvalidLink, Message: "Link tidak dikenali"}`.

### 2.5 IPC (hasil dinormalisasi, json camelCase)
`wire_lark.go` (milik `lark-config-ipc`):
- `lark.sources.get {workspaceId}` -> `{tickets: TicketSource[], wiki: WikiSource[]}` (array kosong `[]`, bukan null; tanpa wiki pribadi).
- `lark.sources.add {workspaceId, kind:'ticket', name, baseToken, host, tableId?, viewId?}` atau `{workspaceId, kind:'wiki', name, spaceId, rootToken?, host}` -> sumber yang dibuat (dengan `id`). `name` kosong -> `[invalid_name]`. `baseToken`/`host`/`spaceId` kosong -> `[invalid_params]`.
- `lark.sources.remove {workspaceId, sourceId}` -> `{}`.
- `lark.sources.rename {workspaceId, sourceId, name}` -> `{}`.

`wire_lark_link.go` (milik `lark-link-parser`):
- `lark.link.parse {link}` -> `{kind:'base'|'wiki', host, baseToken?, tableId?, viewId?, spaceId?, nodeToken?, defaultName?}`.
- `lark.tables.list {workspaceId, sourceId}` -> `{tables:[{id,name}]}` (`sourceId` = TicketSource, lewat `store.LarkSources`).
- `lark.views.list {workspaceId, sourceId, tableId}` -> `{views:[{id,name,type?}]}`.

Go internal (`internal/larkdata`):
```go
// link.go
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
// tables.go
type Table struct { ID string `json:"id"`; Name string `json:"name"` }
type View  struct { ID string `json:"id"`; Name string `json:"name"`; Type string `json:"type,omitempty"` }
func ListTables(ctx context.Context, baseToken, host string) ([]Table, error)
func ListViews(ctx context.Context, baseToken, tableID string) ([]View, error)
```
Semua panggilan CLI hanya lewat `larkdata.Run()` (tiket 01); tanpa `exec` langsung; `runner.go`, `allowlist.go`, `errors.go` tidak diubah. Bila `base +view-list` atau `wiki +node-get` tidak ada di allowlist, laporkan sebagai ketergantungan tiket 01 di catatan akhir plan.

### 2.6 Frontend: tipe dan jenis pane
Tipe TS yang dipakai bersama (masing-masing fitur menulis salinan lokal di berkasnya sendiri; tidak ada berkas tipe bersama):
```ts
type TicketSource = { id: string; name: string; baseToken: string; host: string; tableId?: string; viewId?: string };
type WikiSource   = { id: string; name: string; spaceId: string; rootToken?: string; host: string };
type LarkConfig   = { tickets: TicketSource[]; wiki: WikiSource[] };
type LarkError    = { code: string; message: string };
```
- Jenis pane: `lark-board` props `{ sourceId, tableId?, viewId? }`, `lark-doc` props `{ sourceId | 'personal', nodeToken }`. Field disimpan di `Leaf`: `sourceId`, `tableId`, `viewId`, `nodeToken` (Foundation menambah tipe; `lark-pane-foundation` menambah loader/dedupe).
- Membuka pane (dipakai `lark-panel-ui`): `layout.openPane({ kind: 'lark-board', sourceId: s.id, title: s.name })` dan `layout.openPane({ kind: 'lark-doc', sourceId: s.id /* atau 'personal' */, nodeToken: s.rootToken ?? '', title: s.name })` dengan `layout` dari `frontend/src/lib/stores/layout.ts`. Kunci dedupe (pane yang sama difokuskan, bukan digandakan), ditambahkan `lark-pane-foundation` di `openPane` pada `model.ts`: `lark-board` = `kind + sourceId`; `lark-doc` = `kind + sourceId + nodeToken`.
- `lark-pane-foundation` membuat berkas baru `frontend/src/modules/lark/source.ts` (satu keputusan overview, di luar daftar path tiket tetapi masih di bawah folder modul dan hanya miliknya): berisi `resolveSource(sourceId): Promise<'ok' | 'missing' | 'error'>` dan `useSourceCheck`-sejenis logika. `index.ts` mengekspor ulang `resolveSource`. Alasan: `BoardPane`/`DocPane` diimpor oleh `index.ts`, sehingga mengimpor balik dari `index.ts` membuat siklus; tiket 03/04 yang menimpa pane mengimpor dari `../source`. `source.ts` memakai `call` dari `lib/ipc` + `activeWorkspaceId` langsung (tidak mengimpor `store.ts` milik `lark-panel-ui`).
- `actions.ts`: `copyLink(url: string): Promise<void>`, `sendToAgent(url: string): Promise<void>`.
  - `copyLink`: `navigator.clipboard.writeText`, bila gagal fallback `document.execCommand('copy')` lewat textarea sementara; sukses -> `showToast('Link disalin')`, gagal total -> `showToast('Gagal menyalin link', 'error')`.
  - `sendToAgent`: sesi aktif = leaf terfokus bila `kind==='terminal'` dengan `sessionId`, selain itu leaf terminal pertama di `layout.leaves`; teks yang ditempel `url + ' '` (tanpa Enter); `pasteToSession` dari `../terminal` (`modules/terminal/paste.ts`); tanpa sesi atau `pasteToSession` false -> `showToast('Tidak ada sesi aktif', 'error')`.
  - `showToast` dari `frontend/src/modules/openfolder/toast-store.ts`.
- Panel (`lark-panel-ui`) untuk galat `cli_missing`/`not_logged_in` memanggil `openSettings('connectors')` dari `../settings`.
- `store.ts` (milik `lark-panel-ui`): `larkSources: Writable<LarkConfig>`, `larkState: Writable<{ status: 'loading'|'ready'|'error'; code?: string; message?: string }>`, `refreshSources(): Promise<void>`. Subscriber `activeWorkspaceId` (`lib/stores/workspaces`) dan `onEvent('workspace.changed', ...)` (filter `payload.kind === 'configChanged'`, dan `workspaceId` kosong atau sama dengan workspace aktif) dipasang di level modul; urutan balapan dijaga dengan penghitung `seq` seperti `connectors/store.ts`.
- `client.ts` (milik `lark-panel-ui`): fungsi bertipe `larkSourcesGet(workspaceId)`, `larkSourceAdd(params)`, `larkSourceRemove`, `larkSourceRename`, `larkLinkParse(link)`, `larkTablesList`, `larkViewsList` memakai `call` dari `lib/ipc`; galat dinormalisasi lewat konvensi 2.4 menjadi `LarkError`.
- Titik ekstensi tiket 03/04: `board/BoardPane.svelte` (props `pane: Leaf; visible: boolean` sesuai `PaneKindSpec.component`, membaca `pane.sourceId/tableId/viewId`), `doc/DocPane.svelte` (idem, `pane.sourceId/nodeToken`), `sections/WikiTree.svelte` props `{ source, onOpen(nodeToken: string) }` dengan `source: WikiSource | { id: 'personal'; name: 'Pribadi'; spaceId: 'my_library'; host: string }`. Props `{ source, onOpen }` TIDAK boleh berubah.
- Rail item (Wiring): `registerRailItem({ id: 'lark', title: 'Lark', icon: LarkIcon, component: LarkPanel, order: 10, visible })` impor dari `../shell/registry`; `visible` adalah `Readable<boolean>` dari store connector (`modules/settings/connectors/store.ts`, `connectors`): true bila connector `lark` ada di daftar dan `status.state !== 'cli_missing'`; bila daftar belum dimuat (kosong) tampilkan true (panel yang menampilkan galat). Catatan: `main-rail` menetapkan `visible?: Readable<boolean>`.

### 2.7 ctl (Wiring)
`cmd/shikigamictl/cmd_lark_sources.go` memanggil `registerLarkSub("sources", fn)` dari `init()` (mekanisme `cmd_lark.go` tiket 01: `type larkSubFunc func(ctx, c *ipc.Client, args []string) error`). Sub-subcommand `get|add|remove|rename`, flag lewat `flag.NewFlagSet`, keluaran JSON via `callAndPrint`/`printJSON`:
- `lark sources get --workspace <id>`
- `lark sources add --workspace <id> --kind ticket|wiki --link <url> [--name <n>]`: panggil `lark.link.parse`, cek `kind` hasil cocok (`base` untuk `ticket`, `wiki` untuk `wiki`, bila tidak galat jelas), nama = `--name` lalu `defaultName`, lalu `lark.sources.add` (wiki: `rootToken` = `nodeToken`).
- `lark sources remove --workspace <id> --source <sid>`
- `lark sources rename --workspace <id> --source <sid> --name <n>`

### 2.8 Konvensi
- Teks UI dan galat: bahasa Indonesia; identifier kode: bahasa Inggris.
- Tidak menyimpan token; tidak memanggil CLI di luar `larkdata.Run()`; read-only total; tanpa cache disk; tanpa unit test; tanpa dependensi baru (`frontend/package.json` tidak diubah).
- Satu file satu pemilik; tiap fitur Go mendaftarkan IPC di file `wire_lark*.go` miliknya sendiri.

## 3. Daftar fitur

| Slug | Scope | ownedPaths |
|---|---|---|
| `lark-config-ipc` | Implementasi `Store` Lark (menggantikan stub Foundation) dan IPC `lark.sources.get/add/remove/rename` + publikasi `configChanged`. | `internal/workspace/lark.go`, `cmd/shikigamid/wire_lark.go` (`internal/workspace/types.go` hanya bila Foundation belum menambahkannya) |
| `lark-link-parser` | `ParseLink`, `ListTables`, `ListViews`, nama default, IPC `lark.link.parse`, `lark.tables.list`, `lark.views.list`. | `internal/larkdata/link.go`, `internal/larkdata/tables.go`, `cmd/shikigamid/wire_lark_link.go` |
| `lark-pane-foundation` | Registrasi pane `lark-board`/`lark-doc`, placeholder, `resolveSource`, persist field referensi, dedupe, keadaan pane tak dikenal, `actions.ts`. | `frontend/src/modules/lark/index.ts` (hanya registrasi pane + ekspor), `frontend/src/modules/lark/source.ts` (baru), `frontend/src/modules/lark/actions.ts`, `frontend/src/modules/lark/board/BoardPane.svelte`, `frontend/src/modules/lark/doc/DocPane.svelte`, `frontend/src/modules/layout/model.ts`, `frontend/src/lib/stores/layout.ts`, renderer pane tak dikenal di `frontend/src/modules/layout/` (mis. `LayoutArea.svelte`, seminimal mungkin) |
| `lark-panel-ui` | `LarkPanel`, bagian Tiket/Wiki, `WikiTree` placeholder, `AddSourceDialog`, `client.ts`, `store.ts`, `LarkIcon`. | `frontend/src/modules/lark/LarkPanel.svelte`, `LarkIcon.svelte`, `client.ts`, `store.ts`, `AddSourceDialog.svelte`, `frontend/src/modules/lark/sections/` |
| `lark-sources-wiring` | WIRING: rail item `lark`, `shikigamictl lark sources`, verifikasi akhir. | `cmd/shikigamictl/cmd_lark_sources.go`, `frontend/src/modules/lark/index.ts` (hanya penambahan rail item), `cmd/shikigamid/modules.go` (hanya bila ternyata perlu; lihat bagian 5) |

Acceptance criteria rinci tiap fitur ada di tiket.

## 4. Foundation

Ada (`00-foundation.md`). Isi: (1) tipe Lark di `types.go`; (2) `lark.go` berisi stub kelima metode `Store` agar `lark-link-parser` dan `lark-config-ipc` bisa build paralel; (3) `d.Provide("workspace.store", store)` di `wire_workspace.go`; (4) field Lark tipe-saja di `Leaf`/`NewLeaf` dan dua jenis pane di `PaneKind` pada `layout/model.ts` agar `lark-panel-ui` dan `lark-pane-foundation` type-check bersamaan. Foundation menyentuh tiga path di luar daftar tiket (`wire_workspace.go` satu baris; selebihnya milik fitur) dan dicatat sebagai keputusan sadar: tidak ada cara lain mengakses `Store` tanpa membuat Store kedua.

Prasyarat pohon kerja: `frontend/src/modules/layout/*`, `frontend/src/lib/stores/layout.ts` saat ini berstatus modified (uncommitted) di `git status`. Foundation dan `lark-pane-foundation` hanya menambah hunk sendiri; jangan men-stage atau meng-commit hunk tak terkait milik orang lain. Bila implementasi berjalan di worktree dari commit yang belum memuat perubahan itu, laporkan sebagai risiko merge (bukan diperbaiki sendiri).

## 5. Wiring checklist

Dikerjakan `lark-sources-wiring` setelah keempat fitur selesai:
1. `frontend/src/modules/lark/index.ts`: tambahkan `registerRailItem({...})` (kontrak 2.6) di samping registrasi pane milik `lark-pane-foundation` (jangan hapus/ubah bagian itu). Tidak perlu baris impor di `main.ts` (glob `./modules/*/index.ts` sudah memuat). Impor `registerRailItem` dari `../shell/registry`; `LarkPanel` dan `LarkIcon` dari berkas `lark-panel-ui`.
2. Daemon: konfirmasi `wire_lark.go` dan `wire_lark_link.go` mendaftar lewat `init()`; `modules.go` TIDAK diubah. Pastikan `go build ./...` dan nama modul `lark`/`lark-link` unik. Semua method di 2.5 terjawab (cek lewat `shikigamictl` atau pemanggilan IPC).
3. ctl: buat `cmd/shikigamictl/cmd_lark_sources.go` (kontrak 2.7). Jangan mengubah `cmd_lark.go`; bila `registerLarkSub` tidak ada (tiket 01 belum merge), laporkan, jangan membuat dispatcher sendiri.
4. Pastikan `index.ts` tidak membuat siklus impor (rail item mengimpor `LarkPanel`, bukan sebaliknya).
5. Jalankan aplikasi (skill `run`) dan periksa: rail memuat item Lark; panel menampilkan Tiket dan Wiki (Pribadi paling atas); tempel link Base valid -> nama default terisi -> simpan -> Base muncul; klik membuka pane `lark-board` placeholder; tambah wiki dari link node -> muncul -> klik membuka `lark-doc` placeholder; ganti nama dan hapus bekerja; ganti workspace mengganti isi; restart: sumber dan pane terbuka tetap ada; hapus Base lalu pane terbuka menampilkan "Sumber tidak ada di workspace ini"; link tak valid menampilkan galat inline; tanpa login `lark-cli` panel menampilkan ajakan ke Pengaturan > Connector; `shikigamictl lark sources get --workspace <id>` sama dengan isi panel; ubah lewat ctl terlihat di panel.
6. Aturan pemotongan: baca `docs/lark/cli-findings.md`. Bila `wiki +node-get` tidak memberi `obj_token` bitable -> link `/wiki/` bitable ditolak dengan pesan jelas (`lark-link-parser`); bila `my_library` tak bisa di-list -> Pribadi disembunyikan dengan keterangan (`lark-panel-ui`). Wiring hanya memverifikasi dan melaporkan; perbaikan kecil lintas fitur diizinkan hanya bila blokir tersambung, dan dicatat.

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- `go build ./...` dari root repo lulus.
- `cd frontend && npm run build` lulus, dan `npm run check` (svelte-check) tanpa galat baru.
- Untuk fitur Go: cukup `go build ./...`; untuk fitur frontend: build + check frontend. Aplikasi dijalankan hanya pada tahap Wiring.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain atau file wiring (`cmd_lark_sources.go`, penambahan rail di `index.ts`).
- `internal/connector/*`, `internal/larkdata/runner.go`, `allowlist.go`, `errors.go`, `testdata/`, `docs/lark/`, `cmd/shikigamictl/cmd_lark.go` milik tiket 01; `larkdata/tickets.go`, `ticketlink.go`, `board/*` selain placeholder awal milik tiket 03; `larkdata/wiki.go`, `doc.go`, `doc/*` selain placeholder awal, `frontend/package.json` milik tiket 04; shell/rail milik `main-rail`. Jangan disentuh.
- `internal/workspace/ipc.go` (`config.set`) tidak boleh diubah.
- Hanya `lark-pane-foundation` yang menyentuh `frontend/src/modules/layout/*` dan `lib/stores/layout.ts`.
- Jangan mengubah signature kontrak di bagian 2; bila perlu menyimpang, catat di akhir plan fitur agar Wiring menyelaraskan.
- Working tree punya perubahan tak terkait (`app/*`, `internal/provider/*`, `docs/*`); jangan disentuh atau di-stage.
- Jangan `pkill` luas; matikan hanya proses yang diluncurkan sendiri.
- Jangan menjalankan perintah tulis `lark-cli`.
