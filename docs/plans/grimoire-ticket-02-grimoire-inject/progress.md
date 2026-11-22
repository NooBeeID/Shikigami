# Progress: grimoire-ticket-02-grimoire-inject

- status: OK
- foundation: selesai

## Hasil per fitur

```json
[
  {
    "slug": "compose-snapshot",
    "ok": true,
    "filesChanged": [
      "worktree/feat-grimoire-ticket-02-grimoire-inject/internal/grimoire/inject.go",
      "worktree/feat-grimoire-ticket-02-grimoire-inject/internal/grimoire/snapshot.go"
    ],
    "reason": "Stub diganti implementasi penuh. Header nyata memakai {{grimoire_dir}}/books/..., jadi pengganti placeholder adalah Store.Dir. gofmt, go build dan go vet pada paket internal/grimoire lulus."
  },
  {
    "slug": "laws-ui",
    "ok": true,
    "filesChanged": [
      "worktree/feat-grimoire-ticket-02-grimoire-inject/frontend/src/modules/grimoire/laws/lawsStore.ts",
      "worktree/feat-grimoire-ticket-02-grimoire-inject/frontend/src/modules/grimoire/laws/LawsEditor.svelte",
      "worktree/feat-grimoire-ticket-02-grimoire-inject/frontend/src/modules/grimoire/laws/InjectionInfo.svelte",
      "worktree/feat-grimoire-ticket-02-grimoire-inject/frontend/src/modules/grimoire/laws/LawsSlot.svelte"
    ],
    "reason": "Fitur laws-ui selesai: empat berkas dibuat sesuai kontrak. svelte-check lulus (0 error; 4 warning lama di PaneHostRoot dan TerminalTab, bukan milik fitur ini). Worktree tidak punya node_modules, jadi check dijalankan lewat symlink sementara ke node_modules root repo, lalu symlink dihapus. npm run build tidak dijalankan. client.ts sesuai overview (grimoireStatus dan GrimoireStatus dengan indexBytes, estTokens)."
  },
  {
    "slug": "launch-contribution",
    "ok": true,
    "filesChanged": [
      "worktree/feat-grimoire-ticket-02-grimoire-inject/cmd/shikigamid/wire_grimoire_inject.go"
    ]
  },
  {
    "slug": "laws-measure-ipc",
    "ok": true,
    "filesChanged": [
      "worktree/feat-grimoire-ticket-02-grimoire-inject/internal/grimoire/laws.go",
      "worktree/feat-grimoire-ticket-02-grimoire-inject/internal/grimoire/measure.go"
    ],
    "reason": "laws.go dan measure.go sudah menimpa stub sesuai plan. gofmt bersih. go build ./... dan go vet ./internal/grimoire lulus. Signature Store tiket 01 sama dengan kontrak, jadi tidak ada penyimpangan."
  }
]
```

## Wiring

```json
{
  "ok": true,
  "filesChanged": [
    "worktree/feat-grimoire-ticket-02-grimoire-inject/internal/grimoire/ipc.go",
    "worktree/feat-grimoire-ticket-02-grimoire-inject/cmd/shikigamictl/cmd_grimoire.go",
    "worktree/feat-grimoire-ticket-02-grimoire-inject/frontend/src/modules/grimoire/GrimoirePanel.svelte"
  ],
  "reason": "Wiring selesai di worktree. (1) grimoire.status di internal/grimoire/ipc.go kini memanggil MeasureInjection dan mengisi indexBytes/estTokens/pages/lawsBytes; galat hanya dicatat. (2) shikigamictl grimoire status mencetak JSON lalu ringkasan ke stderr. (3) GrimoirePanel.svelte mengganti slot laws dengan LawsSlot dan menyembunyikan daftar buku saat editor laws terbuka. wire_grimoire_inject.go tidak diubah. Tidak ada STUB(foundation) tersisa."
}
```

## Verifikasi build

- `go build ./...` exit 0 tanpa output.
- `go vet ./internal/grimoire/... ./cmd/...` exit 0.
- `npm run check` di frontend: 0 error, 4 warning `state_referenced_locally` lama di PaneHostRoot.svelte dan TerminalTab.svelte (bukan berkas tiket).
- `npm run build` (vite) selesai; hanya peringatan ukuran chunk.
- go.mod, go.sum, package.json, package-lock.json tidak berubah (tidak ada dependency baru).

## Langkah manual yang ditunda

Tidak ada.

## Build guard terakhir

passed=true
