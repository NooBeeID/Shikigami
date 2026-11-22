# Progress: grimoire-ticket-01-grimoire-build

Worktree: `worktree/feat-grimoire-ticket-01-grimoire-build` (branch `feat/grimoire-ticket-01-grimoire-build`)

- status: OK
- foundation: selesai

## Hasil per fitur

```json
[
  {"slug":"contract-types","ok":true,"filesChanged":["worktree/feat-grimoire-ticket-01-grimoire-build/internal/grimoire/types.go","worktree/feat-grimoire-ticket-01-grimoire-build/internal/grimoire/store.go"]},
  {"slug":"ipc-ctl","ok":true,"filesChanged":["worktree/feat-grimoire-ticket-01-grimoire-build/internal/grimoire/ipc.go","worktree/feat-grimoire-ticket-01-grimoire-build/cmd/shikigamid/wire_grimoire.go","worktree/feat-grimoire-ticket-01-grimoire-build/cmd/shikigamictl/cmd_grimoire.go"]},
  {"slug":"pages-index","ok":true,"filesChanged":["worktree/feat-grimoire-ticket-01-grimoire-build/internal/grimoire/pages.go","worktree/feat-grimoire-ticket-01-grimoire-build/internal/grimoire/index.go","worktree/feat-grimoire-ticket-01-grimoire-build/internal/grimoire/build.go"]},
  {"slug":"panel-ui","ok":true,"filesChanged":["frontend/src/modules/grimoire/client.ts","frontend/src/modules/grimoire/store.ts","frontend/src/modules/grimoire/index.ts","frontend/src/modules/grimoire/GrimoirePanel.svelte"],"needsHumanInput":false},
  {"slug":"readonly-pane","ok":true,"filesChanged":["worktree/feat-grimoire-ticket-01-grimoire-build/frontend/src/modules/layout/model.ts","worktree/feat-grimoire-ticket-01-grimoire-build/frontend/src/modules/layout/panes/FilePane.svelte","worktree/feat-grimoire-ticket-01-grimoire-build/frontend/src/modules/layout/panes/ReadOnlyFileView.svelte","worktree/feat-grimoire-ticket-01-grimoire-build/frontend/src/modules/files/viewer/fileApi.ts","worktree/feat-grimoire-ticket-01-grimoire-build/frontend/src/modules/grimoire/openPage.ts"]},
  {"slug":"scan-modules","ok":true,"filesChanged":["worktree/feat-grimoire-ticket-01-grimoire-build/internal/grimoire/scan.go","worktree/feat-grimoire-ticket-01-grimoire-build/internal/grimoire/modules.go","worktree/feat-grimoire-ticket-01-grimoire-build/internal/grimoire/hash.go"],"reason":"scan.go, modules.go, hash.go ditulis penuh sesuai plan scan-modules. Deviasi kecil di hash.go: isi tiap berkas dicerna dengan SHA-256 per berkas, lalu digest itu ditulis ke hash utama bersama path dan panjang, bukan menulis isi mentah. Hasil tetap deterministik."}
]
```

## Wiring

- status: OK, tidak ada perubahan tambahan dari fase wiring.
- `wire_grimoire.go` memanggil `RegisterModule("grimoire")` dan `grimoire.Register`.
- `cmd_grimoire.go` memanggil `RegisterCommand`.
- `index.ts` memanggil `registerPanel` (id `grimoire`, sidebar, order 30).
- `FilePane` mengarahkan leaf `readOnly` + root ke `ReadOnlyFileView`.
- `go.mod`, `go.sum`, dan `package.json` tidak berubah.

## Langkah manual yang ditunda

Tidak ada.

## Build guard

Dijalankan ulang pada worktree saat penyelesaian:

- `go build ./...`: exit 0
- `go vet ./internal/grimoire/... ./cmd/...`: exit 0
- `npm run check` (svelte-check): exit 0
- `npm run build` (vite): exit 0
- `git diff --stat -- go.mod go.sum frontend/package.json frontend/package-lock.json`: kosong, tidak ada dependency baru

Build guard terakhir: passed=true.
