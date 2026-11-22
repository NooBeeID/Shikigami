# Foundation: lark-connector-ticket-04-lark-wiki

Dikerjakan SEBELUM fitur paralel. Baca `overview.md` (bagian 2.1 dan 4) lebih dulu. Dua berkas Go baru; tanpa unit test. Guard: `go build ./...`.

Prasyarat: tiket 01 dan 02 sudah merge (ada `internal/larkdata/runner.go`, `cmd/shikigamid/wire_lark.go`, `internal/workspace/lark.go`, `frontend/src/modules/lark/doc/DocPane.svelte`). Bila belum, hentikan dan laporkan.

## Langkah

### F1. Skeleton Go `wiki.go`
Berkas baru: `internal/larkdata/wiki.go` (package `larkdata`). Isi persis bagian 2.1 overview:
- Konstanta `wikiPageSize = 50` dan `PersonalSpaceID = "my_library"`.
- Tipe `WikiNode`, `WikiPage`, `WikiNodeInfo` dengan tag json camelCase sesuai overview.
- Stub:
  - `ListWikiNodesPage(ctx, spaceID, parentToken string) (WikiPage, error)` -> `return WikiPage{Nodes: []WikiNode{}}, errors.New("belum diimplementasi")`
  - `ListWikiNodes(ctx, spaceID, parentToken string) ([]WikiNode, error)` -> memanggil `ListWikiNodesPage` dan mengembalikan `p.Nodes` (implementasi final boleh sama; ini sudah benar).
  - `GetWikiNode(ctx, spaceID, nodeToken, host string) (WikiNodeInfo, error)` -> `return WikiNodeInfo{}, errors.New("belum diimplementasi")`
Komentar berkas: "Stub Foundation; diganti implementasi oleh lark-wiki-data." Berkas akan ditimpa penuh oleh `lark-wiki-data`. Cegah galat "unused import" (hanya impor `context` dan `errors`).

### F2. Skeleton Go `doc.go`
Berkas baru: `internal/larkdata/doc.go`:
- Konstanta `sheetMaxRows = 50`, `sheetMaxCols = 20`; tipe `SheetCells{Cells [][]string \`json:"cells"\`; Truncated bool \`json:"truncated"\`}`.
- Stub: `FetchDocMarkdown(ctx, objToken string) (string, error)` -> `return "", errors.New("belum diimplementasi")`; `FetchSheetCells(ctx, objToken string) (SheetCells, error)` -> `return SheetCells{Cells: [][]string{}}, errors.New("belum diimplementasi")`.
Komentar yang sama (diganti `lark-wiki-data`).

### F3. Verifikasi
- `go build ./...` lulus.
- `git status` hanya menampilkan dua berkas baru: `internal/larkdata/wiki.go`, `internal/larkdata/doc.go`. Pastikan nama tidak bentrok dengan `tickets.go`/`ticketlink.go` (tiket 03) bila sudah ada. Jangan men-stage perubahan tak terkait.
- Tidak ada perubahan frontend; tidak ada perubahan di `allowlist.go`, `runner.go`, atau `errors.go`.
