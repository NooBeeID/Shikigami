# Foundation: lark-connector-ticket-02-lark-sources

Dikerjakan SEBELUM fitur paralel. Baca `overview.md` (bagian 2 dan 4) lebih dulu. Hanya empat perubahan kecil; tanpa unit test. Guard: `go build ./...` dan `cd frontend && npm run build`.

## Langkah

### F1. Tipe config Lark
Berkas: `internal/workspace/types.go`.
- Tambahkan di struct `WorkspaceConfig` (sekitar baris 82, setelah `GrimoireKey`) tepat satu field: `Lark *LarkConfig `json:"lark,omitempty"``.
- Tambahkan tipe `LarkConfig`, `TicketSource`, `WikiSource` persis seperti bagian 2.1 overview (tag json camelCase). Jangan ubah field atau tipe lain.
- Periksa `cloneWorkspace`/`cloneWorkspaces` di `internal/workspace/store.go`: salinan `Config` bersifat dangkal, jadi pointer `Lark` terbagi antar salinan. Jangan diubah di Foundation; `lark-config-ipc` wajib menyalin dalam `LarkConfig` sebelum memodifikasi (tidak pernah memutasi struct yang dibagikan).

### F2. Stub `Store` Lark
Berkas baru: `internal/workspace/lark.go` (package `workspace`). Isi lima metode di bagian 2.2 overview dengan tanda tangan persis. Badan stub:
- `LarkSources` -> `return LarkConfig{Tickets: []TicketSource{}, Wiki: []WikiSource{}}, nil`
- `AddLarkTicketSource`, `AddLarkWikiSource` -> `return <zero>, errors.New("belum diimplementasi")`
- `RemoveLarkSource`, `RenameLarkSource` -> `return errors.New("belum diimplementasi")`
Tambahkan komentar berkas: "Stub Foundation; diganti implementasi oleh lark-config-ipc." Berkas ini akan ditimpa penuh oleh `lark-config-ipc`.

### F3. Sediakan store ke modul lain
Berkas: `cmd/shikigamid/wire_workspace.go`. Setelah `workspace.Register(srv, store)` tambahkan satu baris: `d.Provide("workspace.store", store)`. Tidak ada perubahan lain (jangan memindahkan pembuatan store).

### F4. Tipe-saja field Lark pada layout
Berkas: `frontend/src/modules/layout/model.ts` (hanya tipe).
- `PaneKind` menjadi `'terminal' | 'file' | 'thread' | 'lark-board' | 'lark-doc'`.
- `Leaf` dan `NewLeaf`: tambahkan field opsional `sourceId?: string; tableId?: string; viewId?: string; nodeToken?: string;`.
- JANGAN mengubah `makeLeaf`, `openPane`, loader (`sanitize` leaf sekitar baris 364) atau logika lain; itu milik `lark-pane-foundation`. Cukup tipe agar `layout.openPane({ kind: 'lark-board', sourceId, title })` lulus type-check bagi `lark-panel-ui`.
- Berkas ini berstatus modified di working tree (perubahan orang lain). Edit hanya hunk tipe di atas; jangan stage atau commit perubahan tak terkait.

### F5. Verifikasi
- `go build ./...` lulus.
- `cd frontend && npm run build` lulus (dan `npm run check` tanpa galat baru bila cepat).
- `git diff --stat` hanya menampilkan: `internal/workspace/types.go`, `internal/workspace/lark.go` (baru), `cmd/shikigamid/wire_workspace.go`, `frontend/src/modules/layout/model.ts` (hunk tipe).

## Catatan keputusan
- `wire_workspace.go` tidak ada di daftar path tiket; satu baris `Provide` adalah satu-satunya cara memakai `*workspace.Store` yang sama (Store kedua akan saling menimpa `workspaces.json`).
- Tidak ada Foundation frontend selain tipe: `source.ts` milik `lark-pane-foundation` memakai `call` langsung sehingga tidak bergantung pada `store.ts` milik `lark-panel-ui`.
