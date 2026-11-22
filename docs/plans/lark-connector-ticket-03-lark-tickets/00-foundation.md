# Foundation: lark-connector-ticket-03-lark-tickets

Dikerjakan SEBELUM fitur paralel. Baca `overview.md` (bagian 2 dan 4) lebih dulu. Dua kelompok berkas baru; tanpa unit test. Guard: `go build ./...` dan `cd frontend && npm run build`.

Prasyarat: tiket 01 dan 02 sudah merge (ada `internal/larkdata/`, `frontend/src/modules/lark/board/BoardPane.svelte`). Bila belum, hentikan dan laporkan.

## Langkah

### F1. Skeleton Go `tickets.go`
Berkas baru: `internal/larkdata/tickets.go` (package `larkdata`). Isi persis bagian 2.1 overview: konstanta `pageSize = 50` dan `FieldTypeSingleSelect = "single_select"` (bila `docs/lark/cli-findings.md` menyebut nilai tipe single-select normalisasi lain, tetap pakai literal ini sebagai nilai NORMALISASI; pemetaan dari tipe CLI dikerjakan `lark-tickets-data`), tipe `Field`, `Record`, `TicketPage`, `TicketDetailField`, `TicketDetail`, `TicketListQuery`, `TicketGetQuery` (tag json camelCase), serta stub:
- `ListTickets(ctx, q TicketListQuery) (TicketPage, error)` -> `return TicketPage{Fields: []Field{}, Records: []Record{}}, errors.New("belum diimplementasi")`
- `GetTicket(ctx, q TicketGetQuery) (TicketDetail, error)` -> `return TicketDetail{Fields: []TicketDetailField{}}, errors.New("belum diimplementasi")`
Komentar berkas: "Stub Foundation; diganti implementasi oleh lark-tickets-data." Berkas akan ditimpa penuh oleh `lark-tickets-data`. Pakai `const pageSize` agar tidak "unused" (konstanta tak terpakai tidak error di Go, aman).

### F2. Skeleton Go `ticketlink.go`
Berkas baru: `internal/larkdata/ticketlink.go`:
`func BuildTicketLink(host, baseToken, tableID, viewID, recordID string) (url string, recordLinkAvailable bool)` -> stub `return "", false`. Komentar yang sama (diganti `lark-tickets-data`).

### F3. `boardClient.ts`
Berkas baru: `frontend/src/modules/lark/board/boardClient.ts`. Implementasi LENGKAP dan berfungsi sesuai bagian 2.4 overview (bukan stub), supaya fitur UI dan laci langsung bisa memakainya:
- Ekspor `FIELD_TYPE_SINGLE_SELECT`, tipe `TicketSource`, `LarkError`, `BoardField`, `BoardRecord`, `TicketsPage`, `TicketDetail`, `TableInfo`, `ViewInfo` persis nama di 2.4.
- `parseLarkError(e)`: ambil string dari `Error.message` atau `String(e)`, cocokkan regex `^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$`; cocok -> `{code, message}`, tidak -> `{code:'unknown', message: <string asli>}`.
- `listTickets`, `getTicket`, `listTables`, `listViews` memakai `call` dari `../../../lib/ipc` (periksa jalur relatif; berkas ada di `frontend/src/lib/ipc.ts`). Method: `lark.tickets.list`, `lark.ticket.get`, `lark.tables.list` (ambil `.tables`), `lark.views.list` (ambil `.views`). Tangkap galat dan lempar ulang objek `LarkError` hasil `parseLarkError`. Pastikan array hasil tidak null (`?? []`) dan `records[].values` tidak null.
- Tidak mengimpor komponen Svelte atau `store.ts`.

### F4. Verifikasi
- `go build ./...` lulus.
- `cd frontend && npm run build` lulus (dan `npm run check` tanpa galat baru).
- `git diff --stat` / `git status` hanya menampilkan tiga berkas baru: `internal/larkdata/tickets.go`, `internal/larkdata/ticketlink.go`, `frontend/src/modules/lark/board/boardClient.ts`. Jangan men-stage perubahan tak terkait.

## Catatan keputusan
- Foundation sengaja menyentuh berkas milik `lark-tickets-data` dan `lark-board-ui`: tanpa tipe/signature ini, `wire_lark_tickets.go`, `TicketDrawer.svelte`, `Grid.svelte`, dan `Kanban.svelte` tidak bisa build paralel.
- Tidak ada Foundation untuk `TicketDrawer.svelte`: tidak ada berkas yang mengimpornya sampai tahap Wiring.
