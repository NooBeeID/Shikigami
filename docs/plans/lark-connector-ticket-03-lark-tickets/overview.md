# Overview: lark-connector-ticket-03-lark-tickets (Tiket Base grid dan kanban)

Sumber: tiket `docs/tickets/lark-connector-ticket-03-lark-tickets.md` dan spec `docs/superpowers/specs/2026-11-22-09-lark-connector-design.md`. Dokumen ini adalah sumber kebenaran kontrak bersama; plan per fitur hanya merinci langkah kerja. Module Go: `github.com/NooBeeID/shikigami`. Tiket ini berjalan di atas `lark-connector-ticket-01-lark-connector` (connector, `larkdata.Run()`, allowlist, `docs/lark/cli-findings.md`, fixture `internal/larkdata/testdata/`) dan `lark-connector-ticket-02-lark-sources` (config sumber, IPC `lark.sources.*`/`lark.tables.list`/`lark.views.list`, `store.ts`, `actions.ts`, `source.ts`, placeholder `board/BoardPane.svelte`, `registerLarkSub`).

Catatan penting saat plan ini ditulis: kode tiket 01 dan 02 belum ada di pohon kerja (`internal/larkdata/`, `docs/lark/`, `frontend/src/modules/lark/` belum ada). Semua nama tiket 01/02 di bawah diambil dari kontrak `lark-connector-ticket-02-lark-sources/overview.md` dan tiket 03. Agent implementasi wajib memeriksa nama persis di kode saat tiket 01/02 sudah merge, dan melaporkan selisih (jangan mengarang ulang).

## 1. Ringkasan epic dan urutan eksekusi

Epic mengisi pane `lark-board` (placeholder dari tiket 02) menjadi board tiket Lark Base: grid dan kanban, pemilih tabel/view, cari lokal, "Muat lagi" (50 per halaman), laci detail tiket, Salin link dan Kirim ke agen. Semua read-only lewat `lark-cli` (`--as user`, json) melalui `larkdata.Run()`. Backend: normalisasi `base +record-list`/`+record-get` jadi IPC `lark.tickets.list` dan `lark.ticket.get`. Frontend: `boardClient.ts`, `BoardHeader`, `Grid`, `Kanban`, `BoardPane`, `TicketDrawer`. Alat debug: `shikigamictl lark tickets list|get`.

Urutan:
1. **Foundation** (`00-foundation.md`, `hasFoundation=true`): skeleton tipe + signature Go (`tickets.go`, `ticketlink.go`) dan `boardClient.ts` bertipe penuh, agar tiga fitur paralel dan wiring type-check/build bersamaan tanpa menunggu satu sama lain.
2. **Fitur paralel**: `lark-tickets-data`, `lark-board-ui`, `lark-ticket-drawer`. Masing-masing hanya menyentuh `ownedPaths`-nya.
3. **Wiring**: `lark-tickets-wiring` (terakhir): sambung `TicketDrawer` ke `BoardPane.svelte`, ctl `lark tickets`, verifikasi di aplikasi.

Keputusan yang diambil di overview (dasar semua plan fitur):
- **Persist tableId/viewId ke sumber di sisi daemon TIDAK dilakukan.** Tiket menyebut "operasi per item dari tiket 2", tetapi kontrak tiket 02 hanya punya `lark.sources.get/add/remove/rename`; tidak ada operasi ubah tabel/view, dan remove+add akan mengganti ID sumber (merusak pane tersimpan). Maka persist cukup lewat props pane (`tableId`/`viewId` di `Leaf`, sudah dipersist layout oleh tiket 02). `lark-tickets-data` tidak menyimpan apa pun ke sumber; `lark-board-ui` tidak memanggil `lark.sources.*` untuk menyimpan. `TicketSource.TableID/ViewID` dipakai hanya sebagai nilai awal bila ada. Akibatnya tableId/viewId untuk `lark.ticket.get` harus dikirim frontend (lihat 2.2).
- Link Base/view (tingkat Base) dibangun di frontend (`BoardHeader`) dari `source.host`, `source.baseToken`, tabel/view aktif: `https://<host>/base/<baseToken>?table=<tableId>&view=<viewId>` (viewId dihilangkan bila kosong), kecuali `docs/lark/cli-findings.md` menetapkan format lain; ikuti findings bila ada. Link tingkat tiket hanya dari backend (`BuildTicketLink`).
- `larkdata` tidak mengimpor `internal/workspace`; sumber diteruskan sebagai argumen biasa.

## 2. Kontrak bersama

### 2.1 Go: `internal/larkdata` (tipe dan signature dibuat Foundation; isi oleh `lark-tickets-data`)

`tickets.go`:
```go
const pageSize = 50
const FieldTypeSingleSelect = "single_select" // satu-satunya konstanta tipe field yang dipakai lintas lapisan;
                                               // nilai literal final mengikuti temuan spike, tetapi TS memakai nilai yang sama persis

type Field struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Type      string `json:"type"`      // nilai tetap hasil normalisasi; single select = FieldTypeSingleSelect
    IsPrimary bool   `json:"isPrimary"`
}
type Record struct {
    ID     string            `json:"id"`
    Title  string            `json:"title"`  // nilai field utama; kosong -> "(Tanpa judul)"
    Values map[string]string `json:"values"` // key = field id, nilai = teks tampilan (multi-nilai digabung ", ")
}
type TicketPage struct {
    Fields     []Field  `json:"fields"`            // tidak pernah nil
    Records    []Record `json:"records"`           // tidak pernah nil
    NextOffset *int     `json:"nextOffset,omitempty"` // nil bila halaman terakhir
}
type TicketDetailField struct {
    Name string `json:"name"`
    Text string `json:"text"`
}
type TicketDetail struct {
    Fields              []TicketDetailField `json:"fields"` // tidak pernah nil; urutan field tabel; field kosong boleh dilewati
    URL                 string              `json:"url,omitempty"`
    RecordLinkAvailable bool                `json:"recordLinkAvailable"`
}

// Argumen netral (tanpa tipe workspace).
type TicketListQuery struct {
    BaseToken string
    TableID   string
    ViewID    string // opsional
    Offset    int
}
type TicketGetQuery struct {
    BaseToken string
    Host      string
    TableID   string
    ViewID    string // opsional
    RecordID  string
}
func ListTickets(ctx context.Context, q TicketListQuery) (TicketPage, error)
func GetTicket(ctx context.Context, q TicketGetQuery) (TicketDetail, error) // memanggil BuildTicketLink untuk URL/flag
```
`ticketlink.go`:
```go
func BuildTicketLink(host, baseToken, tableID, viewID, recordID string) (url string, recordLinkAvailable bool)
```
Bila format parameter record tidak pasti menurut spike: `recordLinkAvailable=false`, `url` = link tabel/view. Judul kosong -> `"(Tanpa judul)"`. Tepat satu field `IsPrimary` (atau nol bila spike menyatakan tak ada; fallback field pertama ditandai primary agar UI konsisten).

Aturan pemanggilan CLI: hanya `larkdata.Run()`; selalu `--as user` dan format json; perintah hanya `base +record-list` (`--base-token --table-id [--view-id] --limit 50 --offset N`) dan `base +record-get`, dan hanya bila ada di allowlist (bila tidak ada: laporkan sebagai ketergantungan tiket 01, jangan ubah `allowlist.go`); timeout daftar 20 detik (`context.WithTimeout`); galat dikembalikan sebagai `*larkdata.Error` (tiket 01) apa adanya. `runner.go`, `allowlist.go`, `errors.go`, `link.go`, `tables.go` tidak diubah.

### 2.2 IPC (json camelCase; galat memakai konvensi tiket 02 bagian 2.4)

`lark.tickets.list` params `{workspaceId, sourceId, tableId, viewId?, offset}` -> `TicketPage` (bentuk 2.1; TS di 2.4). `offset` boleh 0/hilang = awal. `tableId` kosong -> `[invalid_params]`.

`lark.ticket.get` params `{workspaceId, sourceId, recordId, tableId, viewId?}` -> `TicketDetail`. Berbeda dari tiket (di sana `tableId`/`viewId` opsional): karena sumber tidak menyimpan tabel/view terakhir (bagian 1), frontend SELALU mengirim `tableId` (dan `viewId` bila ada). Backend: `tableId` param kosong -> pakai `TicketSource.TableID`; masih kosong -> `[invalid_params]` "Tabel belum dipilih". `recordId` kosong -> `[invalid_params]`.

Galat (string `"[<code>] <pesan>"` dibungkus `ipc.NewError(<kode ipc>, ...)`, frontend mengurai dengan regex tiket 02 2.4):
- Dari `larkdata.Error`: `cli_missing`, `not_logged_in`, `permission` (pesan "Izin Lark kurang" + pesan CLI), `timeout`, `not_allowed`, `cli_error` -> `ipc.ErrInternal`.
- Sumber tidak ada di config workspace -> `[not_found] Sumber tidak ada di workspace ini` -> `ipc.ErrNotFound`.
- Parameter tidak valid -> `[invalid_params] ...` -> `ipc.ErrInvalid`.
- Tanpa panic; galat tak dikenal -> `[cli_error] <pesan>`.

### 2.3 Daemon: `cmd/shikigamid/wire_lark_tickets.go`
- Modul didaftarkan `RegisterModule("lark-tickets", ...)` di `init()` (nama unik; `modules.go` tidak diubah). Pola sama dengan `wire_lark.go`/`wire_lark_link.go`.
- Store diambil malas saat handler dipanggil: `d.Lookup("workspace.store")` + type assert ke `*workspace.Store` (disediakan Foundation tiket 02). Helper lokal di file ini harus bernama unik di paket `main`, pakai awalan `tickets` (mis. `ticketsStore`, `ticketsSource`, `ticketsErr`); jangan memakai nama `larkStore`/`linkStore` (milik `wire_lark.go`/`wire_lark_link.go`).
- Mencari sumber: `store.LarkSources(wsID)` lalu cari `Tickets[i].ID == sourceId`.
- Tidak ada publikasi event dan tidak ada tulis ke store (read-only).

### 2.4 Frontend: tipe dan `boardClient.ts` (`frontend/src/modules/lark/board/boardClient.ts`, dibuat Foundation, boleh diperluas `lark-board-ui`)
`boardClient.ts` adalah satu-satunya tempat tipe TS epic ini; `Grid`, `Kanban`, `BoardHeader`, `BoardPane`, `TicketDrawer` mengimpor tipe dari `./boardClient` (tidak ada salinan lokal). Ekspor (nama persis):
```ts
export const FIELD_TYPE_SINGLE_SELECT = 'single_select'; // sama dengan Go FieldTypeSingleSelect
export type TicketSource = { id: string; name: string; baseToken: string; host: string; tableId?: string; viewId?: string };
export type LarkError = { code: string; message: string };
export type BoardField = { id: string; name: string; type: string; isPrimary: boolean };
export type BoardRecord = { id: string; title: string; values: Record<string, string> };
export type TicketsPage = { fields: BoardField[]; records: BoardRecord[]; nextOffset?: number };
export type TicketDetail = { fields: { name: string; text: string }[]; url?: string; recordLinkAvailable: boolean };
export type TableInfo = { id: string; name: string };
export type ViewInfo = { id: string; name: string; type?: string };
export function parseLarkError(e: unknown): LarkError;          // regex ^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$ ; tanpa awalan code='unknown'
export function listTickets(p: { workspaceId: string; sourceId: string; tableId: string; viewId?: string; offset: number }): Promise<TicketsPage>;
export function getTicket(p: { workspaceId: string; sourceId: string; recordId: string; tableId: string; viewId?: string }): Promise<TicketDetail>;
export function listTables(p: { workspaceId: string; sourceId: string }): Promise<TableInfo[]>;
export function listViews(p: { workspaceId: string; sourceId: string; tableId: string }): Promise<ViewInfo[]>;
```
Semua fungsi memakai `call` dari `frontend/src/lib/ipc.ts` (`call<T>(method, params)`), melempar `LarkError` (hasil `parseLarkError`) saat gagal; `listTables`/`listViews` membungkus `lark.tables.list` -> `.tables` dan `lark.views.list` -> `.views`.

Komponen (props Svelte 5 `$props()`, gaya sama dengan file lain):
- `Grid.svelte`: `{ fields, records, extraFieldIds?, onOpen(recordId) }`.
- `Kanban.svelte`: `{ fields, records, groupFieldId, extraFieldIds, onOpen(recordId) }`.
- `BoardHeader.svelte`: props/callback ditentukan `lark-board-ui` (hanya dipakai `BoardPane`).
- `TicketDrawer.svelte`: `{ source: TicketSource, tableId: string, viewId?: string, recordId: string, onClose: () => void }`.
- `BoardPane.svelte`: `{ pane: Leaf; visible: boolean }` (kontrak tiket 02), membaca `pane.sourceId/tableId/viewId`. Menyediakan `selectedRecordId` dan callback `onOpenRecord(recordId)` (hasil `lark-board-ui`); `lark-tickets-wiring` menambah render `TicketDrawer`.

Dari tiket 02 (periksa nama persis di kode): `copyLink(url)` dan `sendToAgent(url)` dari `../actions`; `larkSources`/`larkState` dari `../store`; `resolveSource` dari `../source`; `layout` dari `frontend/src/lib/stores/layout.ts` (untuk menyimpan tableId/viewId ke props pane; gunakan fungsi update leaf yang ada, laporkan bila tidak ada, jangan mengubah `layout.ts`/`model.ts`); `activeWorkspaceId` dari `lib/stores/workspaces`.

Aturan UI umum: teks bahasa Indonesia; tanpa polling otomatis (ambil saat mount/pemulihan, ganti sumber/tabel/view, tombol segarkan); respons usang diabaikan (penghitung `seq`); pane tak crash pada galat (keadaan galat dengan tombol coba lagi); gaya memakai token tema yang ada (terang/gelap).

### 2.5 ctl (Wiring)
`cmd/shikigamictl/cmd_lark_tickets.go` mendaftar lewat `registerLarkSub("tickets", fn)` dari `init()` (mekanisme `cmd_lark.go` tiket 01: `type larkSubFunc func(ctx, c *ipc.Client, args []string) error`, sama seperti `cmd_lark_sources.go` tiket 02). Sub-subcommand:
- `lark tickets list --workspace <id> --source <id> --table <id> [--view <id>] [--offset N]` -> memanggil `lark.tickets.list`; cetak ringkas: daftar field (nama, tipe, penanda primary), tiap record `id<TAB>title`, dan `nextOffset`.
- `lark tickets get --workspace <id> --source <id> --record <id> [--table <id>] [--view <id>]` -> `lark.ticket.get`; cetak `nama: nilai` per baris, `url`, `recordLinkAvailable`. Flag `--table` ditambahkan karena sumber tidak menyimpan tabel terakhir (2.2).
Flag lewat `flag.NewFlagSet`; jangan mengubah `cmd_lark.go`; bila `registerLarkSub` tidak ada, laporkan.

### 2.6 Konvensi
- Teks UI dan galat: bahasa Indonesia; identifier kode: bahasa Inggris.
- Tanpa token di config; tanpa cache disk; read-only total (jangan jalankan perintah tulis `lark-cli`); tanpa unit test; tanpa dependensi baru (`frontend/package.json` tidak diubah).
- Satu berkas satu pemilik.

## 3. Daftar fitur

| Slug | Scope | ownedPaths |
|---|---|---|
| `lark-tickets-data` | Normalisasi `+record-list`/`+record-get` ke `TicketPage`/`TicketDetail`, deteksi single-select dan field utama, `nextOffset`, `BuildTicketLink`, IPC `lark.tickets.list` dan `lark.ticket.get` dengan pemetaan galat. | `internal/larkdata/tickets.go`, `internal/larkdata/ticketlink.go`, `cmd/shikigamid/wire_lark_tickets.go` |
| `lark-board-ui` | Pane board: `BoardPane` (state, muat/ulang, stale guard, persist tabel/view ke props pane), `BoardHeader`, `Grid`, `Kanban`, penyempurnaan `boardClient.ts`, Salin/Kirim tingkat Base/view. Tanpa mengimpor `TicketDrawer`; sediakan `selectedRecordId` dan `onOpenRecord`. | `frontend/src/modules/lark/board/BoardPane.svelte`, `Grid.svelte`, `Kanban.svelte`, `BoardHeader.svelte`, `boardClient.ts` |
| `lark-ticket-drawer` | Laci detail tiket di dalam pane: ambil `getTicket`, daftar `nama: nilai`, memuat/galat, Esc/tutup, Salin/Kirim tingkat tiket. | `frontend/src/modules/lark/board/TicketDrawer.svelte` |
| `lark-tickets-wiring` | WIRING: `TicketDrawer` ke `BoardPane`, verifikasi IPC terdaftar, `shikigamictl lark tickets`, verifikasi akhir di aplikasi. | `frontend/src/modules/lark/board/BoardPane.svelte` (hanya penyambungan laci), `cmd/shikigamictl/cmd_lark_tickets.go` |

Acceptance criteria rinci tiap fitur ada di tiket. Penyimpangan dari tiket: hanya soal persist ke sumber dan parameter `lark.ticket.get` (bagian 1 dan 2.2), keduanya konsekuensi kontrak tiket 02.

## 4. Foundation

Ada (`00-foundation.md`). Isi: (1) `internal/larkdata/tickets.go` dan `ticketlink.go` berisi tipe, konstanta, dan stub fungsi sesuai 2.1 supaya `wire_lark_tickets.go` dapat dikompilasi paralel dengan implementasi; (2) `frontend/src/modules/lark/board/boardClient.ts` lengkap dan berfungsi sesuai 2.4 supaya `Grid`, `Kanban`, `BoardHeader`, `BoardPane`, `TicketDrawer` type-check tanpa saling menunggu. Foundation menyentuh path milik fitur (stub/awal yang kemudian ditimpa pemilik); itu keputusan sadar, sama dengan pola Foundation tiket 02. Tidak ada perubahan di luar ketiga berkas itu.

## 5. Wiring checklist

Dikerjakan `lark-tickets-wiring` setelah tiga fitur lain selesai:
1. `frontend/src/modules/lark/board/BoardPane.svelte`: impor `TicketDrawer` dari `./TicketDrawer.svelte`; render saat `selectedRecordId` ada dengan props `source` (TicketSource aktif), `tableId`, `viewId`, `recordId={selectedRecordId}`, `onClose` yang mengosongkan seleksi. Hubungkan `onOpen` Grid/Kanban (lewat `onOpenRecord` yang sudah disiapkan `lark-board-ui`) ke seleksi. Kosongkan seleksi saat `sourceId`/tabel/view berganti. Hanya tambahkan bagian ini; jangan menulis ulang logika board.
2. Daemon: `wire_lark_tickets.go` mendaftar lewat `init()` (`RegisterModule("lark-tickets", ...)`); `modules.go` TIDAK diubah. Konfirmasi nama modul unik dan `lark.tickets.list` serta `lark.ticket.get` terjawab (lewat `shikigamictl lark tickets ...` atau panggilan IPC langsung). Bila registrasi ternyata butuh perubahan di file lain, ikuti pola `wire_lark.go` dan laporkan.
3. ctl: buat `cmd/shikigamictl/cmd_lark_tickets.go` (kontrak 2.5).
4. Pastikan tidak ada siklus impor (`BoardPane` -> `TicketDrawer` -> `boardClient`; `boardClient` tidak mengimpor komponen).
5. Jalankan aplikasi (skill `run`) dan periksa: klik Base di panel Lark membuka board berisi tiket (bukan placeholder); ganti tabel/view; Grid dan Kanban (kolom "Tanpa nilai"; tabel tanpa single-select -> tombol Kanban nonaktif dengan keterangan); cari lokal; Muat lagi menambah halaman bila `nextOffset`; klik kartu/baris membuka laci, tutup (tombol dan Esc) kembali ke daftar; Salin link dan Kirim ke agen (tingkat Base/view dan tingkat tiket; tanpa sesi aktif toast "Tidak ada sesi aktif"; link tiket nonaktif dengan "Link tiket tidak bisa dipastikan" bila `recordLinkAvailable=false`); restart: pane dipulihkan dan data diambil ulang; hapus sumber -> keadaan galat "Sumber tidak ada di workspace ini"; jaringan gagal -> galat dengan coba lagi, tidak crash; `shikigamictl lark tickets list/get` mencetak hasil terbaca.
6. Aturan potong spike: baca `docs/lark/cli-findings.md`; terapkan kanban nonaktif tanpa single-select, link tiket nonaktif bila format record tak pasti. Wiring hanya memverifikasi dan melaporkan; perbaikan kecil lintas fitur diizinkan hanya bila blokir tersambung, dan dicatat.

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- Fitur Go: `go build ./...` dari root repo.
- Fitur frontend: `cd frontend && npm run build`, dan `npm run check` (svelte-check) tanpa galat baru.
- Foundation dan Wiring: keduanya. Aplikasi dijalankan hanya pada tahap Wiring.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain atau file wiring (`cmd_lark_tickets.go`, penyambungan laci di `BoardPane.svelte`).
- Pengecualian satu arah: `lark-ticket-drawer` dan komponen board memakai `boardClient.ts` buatan Foundation sebagai kontrak; hanya `lark-board-ui` yang boleh mengubahnya, dan hanya secara aditif (jangan mengubah nama/signature di 2.4).
- Jangan menyentuh: `internal/connector/*`, `internal/larkdata/runner.go`, `allowlist.go`, `errors.go`, `link.go`, `tables.go`, `testdata/`, `docs/lark/`, `cmd/shikigamictl/cmd_lark.go`, `cmd_lark_sources.go` (tiket 01 dan 02); `internal/workspace/*`, `wire_lark.go`, `wire_lark_link.go`, `wire_workspace.go`, `modules/lark/` selain `board/`, `frontend/src/modules/layout/*`, `lib/stores/layout.ts` (tiket 02); wiki dan dokumen (`wiki.go`, `doc.go`, `doc/*`, `frontend/package.json`, tiket 04); shell/rail milik `main-rail`.
- Jangan mengubah signature kontrak di bagian 2; bila perlu menyimpang, catat di akhir plan fitur agar Wiring menyelaraskan.
- Working tree punya perubahan tak terkait (`docs/*`, dll); jangan disentuh atau di-stage.
- Jangan `pkill` luas; matikan hanya proses yang diluncurkan sendiri.
- Jangan menjalankan perintah tulis `lark-cli`.
