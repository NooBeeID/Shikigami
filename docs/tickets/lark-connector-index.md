# Indeks tiket: Connector Lark (tiket Base dan wiki)

Sumber: `docs/superpowers/specs/2026-11-22-09-lark-connector-design.md`

## Ringkasan tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|-------|-------|------|------------|--------------|
| lark-connector-ticket-01-lark-connector | Connector Lark dan spike CLI | 1 | - | 4 |
| lark-connector-ticket-02-lark-sources | Rail Lark dan sumber per workspace | 2 | lark-connector-ticket-01-lark-connector | 5 |
| lark-connector-ticket-03-lark-tickets | Tiket Base grid dan kanban | 3 | lark-connector-ticket-02-lark-sources | 4 |
| lark-connector-ticket-04-lark-wiki | Wiki pohon dan penampil dokumen | 3 | lark-connector-ticket-02-lark-sources | 4 |

Fitur per tiket:

- **lark-connector-ticket-01-lark-connector**: lark-cli-spike, lark-connector-adapter, larkdata-runner, lark-ctl-status
- **lark-connector-ticket-02-lark-sources**: lark-config-ipc, lark-link-parser, lark-pane-foundation, lark-panel-ui, lark-sources-wiring
- **lark-connector-ticket-03-lark-tickets**: lark-tickets-data, lark-board-ui, lark-ticket-drawer, lark-tickets-wiring
- **lark-connector-ticket-04-lark-wiki**: lark-wiki-data, lark-wiki-tree-ui, lark-doc-pane, lark-wiki-wiring

## Prasyarat sebelum implementasi

- `main-rail-ticket-01-main-rail` sudah di-merge. Yang dipakai: `registerRailItem` dan `railItems` di `frontend/src/modules/shell/registry.ts`, dengan props panel `{}`.
- Perubahan working tree yang belum di-commit harus sudah di-commit sebelum `lark-pane-foundation` dikerjakan: `frontend/src/modules/layout/*`, `frontend/src/lib/stores/layout.ts`, `internal/provider/*`, `frontend/src/modules/thread/participants/terminal.ts`.

## Urutan eksekusi per wave

Tiket dalam wave yang sama bisa dijalankan paralel. Wave berikutnya menunggu wave sebelumnya selesai dan di-merge.

- **Wave 1**: lark-connector-ticket-01-lark-connector
  - Paralel: `lark-cli-spike`, `lark-connector-adapter`, `lark-ctl-status`. `larkdata-runner` bisa mulai paralel dengan kerangka (allowlist, klasifikasi galat, `RunCLITimeout`), lalu flag disesuaikan setelah `docs/lark/cli-findings.md` selesai.
- **Wave 2**: lark-connector-ticket-02-lark-sources (menunggu wave 1)
  - Paralel: `lark-config-ipc`, `lark-link-parser`, `lark-pane-foundation` (setelah prasyarat commit layout), `lark-panel-ui` (memakai kontrak IPC, bukan implementasi Go).
  - Lalu: `lark-sources-wiring` (terakhir, setelah fitur di atas).
- **Wave 3**: lark-connector-ticket-03-lark-tickets dan lark-connector-ticket-04-lark-wiki (paralel satu sama lain, menunggu wave 2)
  - Tiket 03: `lark-tickets-data`, `lark-board-ui`, dan `lark-ticket-drawer` paralel. Lalu `lark-tickets-wiring` terakhir (menyentuh `BoardPane.svelte` setelah `lark-board-ui` selesai).
  - Tiket 04: `lark-wiki-data`, `lark-wiki-tree-ui`, dan `lark-doc-pane` paralel. `lark-wiki-wiring` bagian ctl paralel, verifikasi akhir setelah tiga fitur lain selesai.
  - Tiket 03 dan 04 tidak saling bergantung, jadi bisa dijalankan bersamaan dengan worktree terpisah.

Ringkasan jalur paralel maksimum: wave 1 (4 fitur), wave 2 (3 fitur paralel), wave 3 (tiket 03 dan 04 sekaligus, masing-masing 3 fitur paralel).

## Kontrak bersama

### 1) Go

- Connector id `lark`, file `internal/connector/lark.go`.
- Status memakai schema `lark-cli auth status`, field `identities.user.{status, available, userName}`.
- Paket `internal/larkdata` hanya memanggil `connector.RunCLI` lewat `Run()` di `runner.go`:
  - Allowlist perintah baca, selalu `--as user` dan format json, timeout daftar 20 detik.
- Arah dependensi: `larkdata -> connector`. `workspace` tidak mengimpor `larkdata`. `wire_lark*.go` mengimpor keduanya.

### 2) Config

`WorkspaceConfig.Lark *LarkConfig` di `internal/workspace/types.go`:

- `LarkConfig{Tickets []TicketSource, Wiki []WikiSource}`
- `TicketSource{ID, Name, BaseToken, Host, TableID?, ViewID?}`
- `WikiSource{ID, Name, SpaceID, RootToken?, Host}`
- Json camelCase seperti spec. ID berupa ULID dibuat di Go. Tanpa token.
- Wiki pribadi tidak disimpan: `spaceId` `my_library`, `sourceId` literal `"personal"`.

### 3) IPC

Nama method dan hasil dinormalisasi. Galat memakai kode connector yang sama dengan pesan Indonesia. Event `configChanged` dipakai ulang. Setiap tiket mendaftarkan IPC di file `wire_lark*.go` sendiri.

| Method | Parameter | Hasil |
|--------|-----------|-------|
| `lark.sources.get` | `{workspaceId}` | daftar sumber |
| `lark.sources.add` | `{workspaceId, kind:'ticket'\|'wiki', ...}` | sumber baru (ID dari Go) |
| `lark.sources.remove` | `{workspaceId, sourceId}` | - |
| `lark.sources.rename` | `{workspaceId, sourceId, name}` | - |
| `lark.link.parse` | `{link}` | token, host, table, view |
| `lark.tables.list` | `{workspaceId, sourceId}` | tabel |
| `lark.views.list` | `{workspaceId, sourceId, tableId}` | view |
| `lark.tickets.list` | `{workspaceId, sourceId, tableId, viewId?, offset}` | `{fields[{id,name,type,isPrimary}], records[{id,title,values}], nextOffset?}` |
| `lark.ticket.get` | `{workspaceId, sourceId, recordId}` | `{fields:[{name,text}], url?, recordLinkAvailable}` |
| `lark.wiki.list` | `{workspaceId, sourceId\|'personal', parentToken?}` | `nodes[{token,title,objType,objToken,hasChild}]` |
| `lark.wiki.get` | `{workspaceId, sourceId, nodeToken}` | `{title,objType,url,markdown?,cells?}` |

Catatan: `lark.sources.get/set` dari spec awal diganti operasi per item (`add`, `remove`, `rename`) agar dua edit bersamaan tidak saling menimpa.

### 4) Frontend

- Modul `frontend/src/modules/lark/`.
- Item rail id `lark`, order 10, panel `LarkPanel`.
- Jenis pane `lark-board` dengan props `{sourceId, tableId?, viewId?}` dan `lark-doc` dengan props `{sourceId | 'personal', nodeToken}`.
- Field Leaf baru `sourceId`, `tableId`, `viewId`, `nodeToken` ditambahkan ke daftar field persist loader layout oleh `lark-sources`.
- `actions.ts`: `copyLink(url)` dan `sendToAgent(url)` (`pasteToSession`, tanpa Enter, toast "Tidak ada sesi aktif").
- Titik ekstensi: `board/BoardPane.svelte`, `doc/DocPane.svelte`, dan `sections/WikiTree.svelte` dibuat sebagai placeholder oleh `lark-sources`, lalu ditimpa `lark-tickets` dan `lark-wiki`. `sections/WikiSection` menyerahkan props `{source, onOpen(nodeToken)}` ke `WikiTree`.

### 5) Aturan pemotongan

Bila spike (`docs/lark/cli-findings.md`) membantah asumsi, fitur terkait dipotong dan dilaporkan. Pekerjaan lain lanjut:

- Kanban dinonaktifkan bila tidak ada field single-select.
- Wiki pribadi disembunyikan bila `my_library` gagal di-list.
- Link tiket dinonaktifkan bila format link record tidak pasti.

Batasan umum: read-only total, tanpa cache disk, tanpa unit test. Guard hanya build Go (`go build ./...`) dan build frontend.

## Catatan

- Fitur frontend memakai kontrak IPC di atas, sehingga bisa dikerjakan paralel dengan implementasi Go (boleh diuji dengan data tiruan lokal sementara).
- Tiket 01 tidak menyentuh frontend. Halaman Pengaturan > Connector memuat Lark otomatis dari registry.
