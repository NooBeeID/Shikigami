# Plan: Wiring tiket dan ctl

## Metadata
- Tiket: `lark-connector-ticket-03-lark-tickets`
- Fitur: `lark-tickets-wiring` (WIRING, dikerjakan TERAKHIR, setelah `lark-tickets-data`, `lark-board-ui`, `lark-ticket-drawer` selesai)
- ownedPaths:
  - `frontend/src/modules/lark/board/BoardPane.svelte` (HANYA penyambungan laci; tanpa menulis ulang board)
  - `cmd/shikigamictl/cmd_lark_tickets.go` (baru)
- Depends on foundation: ya (`00-foundation.md`: `tickets.go`/`ticketlink.go` skeleton dan `boardClient.ts`). Juga bergantung pada hasil ketiga fitur paralel dan pada tiket 01 (`registerLarkSub`, `larkdata.Run()`) serta tiket 02 (IPC `lark.sources.*`, `BoardPane` placeholder).

## Tujuan
Menyambungkan hasil fitur lain: `TicketDrawer` dirender di dalam `BoardPane` saat sebuah tiket dipilih (klik baris Grid / kartu Kanban), memastikan daemon menjawab `lark.tickets.list` dan `lark.ticket.get`, menyediakan `shikigamictl lark tickets list|get` sebagai alat debug, lalu memverifikasi alur penuh di aplikasi. Rujukan kontrak: `overview.md` bagian 2.2 (IPC), 2.4 (frontend), 2.5 (ctl), 5 (wiring checklist), 6 (build guard). Wiring hanya menyambung dan memverifikasi; kekurangan fitur lain dilaporkan, bukan ditambal besar-besaran.

## Path yang dimiliki
HANYA ini yang boleh disentuh:
- `frontend/src/modules/lark/board/BoardPane.svelte`: tambah impor `TicketDrawer`, render kondisional, penyambungan `onOpen` dan penutupan laci. Jangan menulis ulang logika muat/stale guard/persist milik `lark-board-ui`.
- `cmd/shikigamictl/cmd_lark_tickets.go` (baru).

Jangan sentuh: `Grid.svelte`, `Kanban.svelte`, `BoardHeader.svelte`, `boardClient.ts`, `TicketDrawer.svelte`, `internal/larkdata/*`, `cmd/shikigamid/wire_lark_tickets.go`, `cmd/shikigamid/modules.go`, `cmd/shikigamictl/cmd_lark.go`, `cmd_lark_sources.go`, `frontend/package.json`, `layout.ts`/`model.ts`, `docs/lark/`. Working tree punya perubahan tak terkait (`docs/*`, `frontend/src/modules/sidebar-left/*`, `frontend/src/modules/thread/*`, dll); jangan disentuh atau di-stage. Jangan `pkill` luas; matikan hanya proses yang diluncurkan sendiri. Jangan menjalankan perintah tulis `lark-cli`.

## Kontrak
Dipakai (detail di overview 2.2, 2.4, 2.5):
- Frontend: `TicketDrawer` dari `./TicketDrawer.svelte` dengan props `{ source: TicketSource, tableId: string, viewId?: string, recordId: string, onClose: () => void }`; tipe `TicketSource` dari `./boardClient`. Dari `BoardPane` hasil `lark-board-ui`: state `selectedRecordId`, callback `onOpenRecord(recordId)`, nilai sumber aktif (`TicketSource` hasil `resolveSource`), `tableId` aktif, `viewId` aktif. Nama persis HARUS dibaca dari berkas (nama di sini hanya patokan overview).
- IPC daemon: `lark.tickets.list` params `{workspaceId, sourceId, tableId, viewId?, offset}` dan `lark.ticket.get` params `{workspaceId, sourceId, recordId, tableId, viewId?}`; hasil `TicketPage` / `TicketDetail` (camelCase) seperti overview 2.1/2.2.
- Go ctl: `registerLarkSub(name string, fn larkSubFunc)` dari `cmd_lark.go` (tiket 01), `type larkSubFunc func(ctx context.Context, c *ipc.Client, args []string) error`; `callAndPrint`/`printJSON` dari `registry.go`; `(*ipc.Client).Call(method string, params any, out any) error`.

Disediakan: subcommand `shikigamictl lark tickets list` dan `lark tickets get` (flag di langkah 4). Tidak ada API frontend baru.

## Langkah

### 0. Pra-pemeriksaan (jangan membuat pengganti sendiri)
Jalankan:
- `ls frontend/src/modules/lark/board/` (harus ada `BoardPane.svelte`, `Grid.svelte`, `Kanban.svelte`, `BoardHeader.svelte`, `boardClient.ts`, `TicketDrawer.svelte`)
- `ls cmd/shikigamid/wire_lark_tickets.go internal/larkdata/tickets.go internal/larkdata/ticketlink.go`
- `grep -rn "func registerLarkSub" cmd/shikigamictl/`
- `grep -n "TicketDrawer\|selectedRecordId\|onOpenRecord\|onOpen" frontend/src/modules/lark/board/BoardPane.svelte`

Aturan blokir:
- Bila `registerLarkSub` tidak ada (tiket 01 belum merge): JANGAN membuat dispatcher `lark` dan jangan menulis `cmd_lark_tickets.go` (tidak akan terkompilasi). Kerjakan bagian frontend, lalu laporkan blokir (`ok:false`, alasan "tiket 01 belum merge: registerLarkSub tidak ada").
- Bila `TicketDrawer.svelte` atau `boardClient.ts` atau `BoardPane.svelte` hasil `lark-board-ui` belum ada (masih placeholder tanpa `selectedRecordId`): hentikan dan laporkan; jangan membangun board sendiri.
- Bila `tickets.go` masih berisi stub Foundation (`belum diimplementasi`) atau `wire_lark_tickets.go` tidak ada: laporkan sebagai kekurangan `lark-tickets-data`; kerjakan sisanya.

### 1. Sambungkan laci di `BoardPane.svelte`
Baca berkas penuh dulu, lalu ubah dengan Edit (bukan menimpa). Tambahan minimal:
1. Impor: `import TicketDrawer from './TicketDrawer.svelte';` (gabungkan dengan blok impor yang ada; bila `TicketSource` belum diimpor sebagai tipe dari `./boardClient`, tambahkan).
2. State: bila `lark-board-ui` sudah menyediakan `let selectedRecordId = $state<string | null>(null)` dan `onOpenRecord(recordId)` yang mengisinya, pakai apa adanya. Bila `onOpenRecord` hanya stub kosong/log, isi dengan `selectedRecordId = recordId`. Jangan membuat state ganda.
3. `onOpen` Grid dan Kanban: pastikan elemen `<Grid ... onOpen={onOpenRecord} />` dan `<Kanban ... onOpen={onOpenRecord} />` terhubung (tambahkan prop bila belum).
4. Render laci di dalam kontainer pane (kontainer harus `position: relative` agar laci meluncur di dalam area pane; bila belum, tambah satu aturan CSS minimal pada kontainer akar pane, tanpa mengubah gaya lain):
```svelte
{#if selectedRecordId && source && activeTableId}
  <TicketDrawer
    {source}
    tableId={activeTableId}
    viewId={activeViewId || undefined}
    recordId={selectedRecordId}
    onClose={() => (selectedRecordId = null)}
  />
{/if}
```
`source`, `activeTableId`, `activeViewId` adalah nama patokan; gunakan nama variabel nyata di berkas untuk sumber aktif, tabel aktif, dan view aktif (nilai SAMA dengan yang dipakai untuk `listTickets`). `source` harus bertipe `TicketSource`.
5. Tutup laci saat konteks berganti: kosongkan `selectedRecordId` pada saat `pane.sourceId`, tabel aktif, atau view aktif berganti. Cara: bila ada fungsi yang menangani ganti tabel/view/sumber (mis. `selectTable`, `selectView`, atau `$effect` pemuat ulang), tambahkan satu baris `selectedRecordId = null` di sana; bila tidak ada titik tunggal, tambahkan `$effect` kecil yang membaca `pane.sourceId`, tabel aktif, dan view aktif lalu mengosongkan seleksi. Segarkan manual (tombol segarkan) TIDAK wajib menutup laci.
6. Tidak ada siklus impor: `BoardPane -> TicketDrawer -> boardClient`; `boardClient` tidak mengimpor komponen. Verifikasi dengan `grep -n "^import" frontend/src/modules/lark/board/TicketDrawer.svelte frontend/src/modules/lark/board/boardClient.ts`.
7. Jangan mengubah bagian lain (muat data, stale guard, header, persist). Bila ditemukan cacat di bagian itu: catat di laporan; perbaikan hanya bila memblokir sambungan dan kecil.

### 2. Konfirmasi daemon tanpa mengubah `modules.go`
- `grep -n 'RegisterModule("lark-tickets"' cmd/shikigamid/wire_lark_tickets.go` harus berada di `init()`. Pastikan nama unik: `grep -rn "RegisterModule(" cmd/shikigamid | grep -i lark` (tidak ada nama ganda; pola sama dengan `wire_lark.go` dan `wire_lark_link.go`).
- Pastikan helper di `wire_lark_tickets.go` tidak bentrok nama (`go build ./...` lulus).
- `modules.go` TIDAK diubah. Hanya bila build atau pembacaan `modules.go` membuktikan daftar modul eksplisit yang butuh baris tambahan, laporkan (bukan ubah) sebagai temuan, karena `modules.go` di luar ownedPaths; ikuti pola `wire_lark.go` dan catat alasannya.
- Konfirmasi dijawab saat daemon berjalan: lihat langkah 5 (`shikigamictl lark tickets list/get`).

### 3. Pra-baca pola ctl
Baca `cmd/shikigamictl/cmd_lark.go` (tiket 01) dan `cmd_lark_sources.go` (tiket 02) untuk pola `registerLarkSub`, penguraian flag, dan galat usage; ikuti gaya itu. Baca `registry.go` (sudah ada: `printJSON`, `callAndPrint`). Jangan mengubah file-file itu.

### 4. Buat `cmd/shikigamictl/cmd_lark_tickets.go`
`package main`. Impor: `context`, `errors`, `flag`, `fmt`, `io`, `os`, `strings`, `text/tabwriter` (opsional), `github.com/NooBeeID/shikigami/internal/ipc`. JANGAN impor `internal/larkdata` atau `internal/workspace`; definisikan struct respons lokal ber-awalan `larkTickets` agar tidak bentrok nama di paket `main` (`larkTicketsField`, `larkTicketsRecord`, `larkTicketsPage`, `larkTicketsDetailField`, `larkTicketsDetail`).

```go
const larkTicketsUsage = `penggunaan:
  shikigamictl lark tickets list --workspace <id> --source <id> --table <id> [--view <id>] [--offset N]
  shikigamictl lark tickets get  --workspace <id> --source <id> --record <id> [--table <id>] [--view <id>]`

func init() { registerLarkSub("tickets", runLarkTickets) }

func runLarkTickets(ctx context.Context, c *ipc.Client, args []string) error
```
Struct respons (tag json camelCase, sesuai overview 2.1):
```go
type larkTicketsField struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Type      string `json:"type"`
    IsPrimary bool   `json:"isPrimary"`
}
type larkTicketsRecord struct {
    ID     string            `json:"id"`
    Title  string            `json:"title"`
    Values map[string]string `json:"values"`
}
type larkTicketsPage struct {
    Fields     []larkTicketsField  `json:"fields"`
    Records    []larkTicketsRecord `json:"records"`
    NextOffset *int                `json:"nextOffset"`
}
type larkTicketsDetailField struct {
    Name string `json:"name"`
    Text string `json:"text"`
}
type larkTicketsDetail struct {
    Fields              []larkTicketsDetailField `json:"fields"`
    URL                 string                   `json:"url"`
    RecordLinkAvailable bool                     `json:"recordLinkAvailable"`
}
```
Perilaku:
- `len(args)==0` atau aksi (`args[0]`) bukan `list`/`get`: `errors.New(larkTicketsUsage)`.
- Sisa argumen diurai dengan `flag.NewFlagSet("lark tickets <aksi>", flag.ContinueOnError)`, `fs.SetOutput(io.Discard)`; galat parse dibungkus bersama usage.
- Wajib: `list` -> `--workspace`, `--source`, `--table`; `get` -> `--workspace`, `--source`, `--record`. Kosong (setelah `strings.TrimSpace`) -> `fmt.Errorf("--<flag> wajib\n%s", larkTicketsUsage)`. `--offset` bertipe int, default 0, negatif -> galat.
- `list`: params `map[string]any{"workspaceId": ws, "sourceId": src, "tableId": table, "offset": offset}` ditambah `"viewId": view` hanya bila tidak kosong. `var out larkTicketsPage; c.Call("lark.tickets.list", params, &out)`. Cetak ke stdout:
  1. baris `fields:` lalu per field `  <id><TAB><name><TAB><type>` ditambah `<TAB>(primary)` bila `IsPrimary`;
  2. baris `records:` lalu per record `  <id><TAB><title>`;
  3. baris `nextOffset: <N>` bila ada, selain itu `nextOffset: -` (halaman terakhir).
- `get`: params `{"workspaceId", "sourceId", "recordId"}` ditambah `"tableId"` dan `"viewId"` hanya bila tidak kosong. `var out larkTicketsDetail; c.Call("lark.ticket.get", params, &out)`. Cetak per baris `<name>: <text>` (teks multi-baris dipertahankan apa adanya), lalu `url: <url>` (atau `url: -` bila kosong) dan `recordLinkAvailable: <true|false>`.
- Galat IPC dari `c.Call` dikembalikan apa adanya (sudah berawalan `[kode]`); tidak dipetakan ulang.
- Jangan mengubah `cmd_lark.go`; tidak ada `RegisterCommand("lark")` di berkas ini; tidak ada panic.
- Perbedaan dari tiket: flag `--table` pada `get` ada karena sumber tidak menyimpan tabel terakhir (overview 2.2); bila kosong, daemon memakai `TicketSource.TableID` atau menjawab `[invalid_params]`.

### 5. Verifikasi build
- `go build ./...` dari root repo harus lulus; `go vet ./cmd/shikigamictl/` bila cepat.
- `cd frontend && npm run build` lulus dan `npm run check` tanpa galat baru dibanding sebelum perubahan (catat galat lama bila ada).
- `git status`/`git diff --stat`: perubahan milik fitur ini hanya `BoardPane.svelte` dan `cmd_lark_tickets.go`; jangan men-stage perubahan tak terkait.

### 6. Jalankan aplikasi dan periksa (skill `run`)
Jalankan aplikasi dengan skill `run` (catat PID yang diluncurkan sendiri; matikan hanya itu di akhir). Baca `docs/lark/cli-findings.md` lebih dulu untuk aturan potong spike. Periksa seluruh checklist overview bagian 5 butir 5, catat hasil per butir:
1. Klik Base di panel Lark membuka board berisi tiket (bukan placeholder).
2. Ganti tabel dan view; data diambil ulang.
3. Grid dan Kanban (kolom "Tanpa nilai"); tabel tanpa single-select: tombol Kanban nonaktif dengan keterangan.
4. Cari lokal menyaring tanpa panggilan baru; "Muat lagi" menambah halaman bila `nextOffset` ada.
5. Klik baris Grid dan kartu Kanban membuka laci dengan detail tiket yang benar; tutup lewat tombol dan Esc kembali ke daftar; ganti tabel/view/sumber menutup laci.
6. Salin link dan Kirim ke agen, tingkat Base/view dan tingkat tiket; tanpa sesi aktif toast "Tidak ada sesi aktif"; tombol tingkat tiket nonaktif dengan "Link tiket tidak bisa dipastikan" bila `recordLinkAvailable=false`.
7. Restart: pane dipulihkan dan data diambil ulang.
8. Hapus sumber (lewat panel atau `shikigamictl lark sources remove`) lalu pane menampilkan galat "Sumber tidak ada di workspace ini".
9. Jaringan gagal/belum login: galat terbaca dengan tombol coba lagi, tidak crash.
10. ctl terhadap daemon yang berjalan: `shikigamictl lark tickets list --workspace <id> --source <id> --table <id>` lalu `shikigamictl lark tickets get ... --record <id> --table <id>` mencetak hasil terbaca; ini sekaligus membuktikan `lark.tickets.list` dan `lark.ticket.get` terdaftar dan terjawab. Cek juga galat: `--table` kosong pada `list` -> usage; `tableId` kosong ke daemon -> `[invalid_params]`.
Butir yang butuh login Lark nyata atau Base nyata yang tidak tersedia: tandai "tidak dapat diverifikasi" dengan alasan; jangan dikarang. Bila `cli-findings.md` memicu aturan potong, verifikasi bahwa perilaku potong muncul (bukan memperbaikinya).
Perbaikan lintas fitur hanya bila memblokir sambungan dan kecil; catat tiap perbaikan di laporan. Selisih nama dengan overview (mis. nama state/callback di `BoardPane`) dilaporkan.

## Perubahan
- Baru: `cmd/shikigamictl/cmd_lark_tickets.go`
- Diubah: `frontend/src/modules/lark/board/BoardPane.svelte` (impor `TicketDrawer`, render kondisional, sambung `onOpen`, tutup laci saat konteks berganti)
- Tidak diubah (dikonfirmasi): `cmd/shikigamid/modules.go`, `cmd/shikigamid/wire_lark_tickets.go`, `cmd/shikigamictl/cmd_lark.go`, `Grid.svelte`, `Kanban.svelte`, `BoardHeader.svelte`, `boardClient.ts`, `TicketDrawer.svelte`, `frontend/package.json`

## Wiring
Fitur ini adalah langkah wiring itu sendiri. Ia tidak mengekspos apa pun untuk disambungkan lagi. Yang diharapkan sudah tersedia dari fitur lain: `selectedRecordId` dan `onOpenRecord` di `BoardPane` serta `onOpen` Grid/Kanban (`lark-board-ui`), `TicketDrawer` (`lark-ticket-drawer`), pendaftaran `lark-tickets` lewat `init()` (`lark-tickets-data`), `registerLarkSub` (tiket 01).

## Selesai bila
- Klik baris Grid dan kartu Kanban membuka `TicketDrawer` di dalam pane dengan `source`, `tableId`, `viewId`, `recordId` yang benar; tutup (tombol, Esc, `onClose`) mengosongkan seleksi; ganti sumber/tabel/view menutup laci; tidak ada siklus impor; logika board lain tidak berubah.
- `lark.tickets.list` dan `lark.ticket.get` terjawab oleh daemon yang berjalan; `modules.go` tidak diubah.
- `shikigamictl lark tickets list|get` terdaftar lewat `registerLarkSub("tickets", ...)`, `cmd_lark.go` tidak berubah, keluaran terbaca sesuai langkah 4.
- Guard: `go build ./...` lulus; `cd frontend && npm run build` lulus dan `npm run check` tanpa galat baru. Tanpa unit test.
- Aplikasi dijalankan dan checklist langkah 6 diperiksa; hasil per butir (lulus, gagal, tidak dapat diverifikasi dan alasannya) dicatat di laporan akhir. Blokir dari prasyarat (tiket 01 atau fitur lain belum selesai) dilaporkan, bukan ditambal.
