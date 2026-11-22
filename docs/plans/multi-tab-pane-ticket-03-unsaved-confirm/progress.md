# Progress: multi-tab-pane-ticket-03-unsaved-confirm

- status: OK
- foundation: selesai
- wiring: selesai

## Hasil per fitur

```json
[
  {
    "slug": "confirm-close-dialog",
    "ok": true,
    "filesChanged": [
      "worktree/feat-multi-tab-pane-ticket-03-unsaved-confirm/frontend/src/modules/files/confirmClose/index.ts",
      "worktree/feat-multi-tab-pane-ticket-03-unsaved-confirm/frontend/src/modules/files/confirmClose/ConfirmCloseDialog.svelte"
    ]
  },
  {
    "slug": "file-tab-registry",
    "ok": true,
    "filesChanged": [
      "worktree/feat-multi-tab-pane-ticket-03-unsaved-confirm/frontend/src/modules/files/editor/FileTab.svelte",
      "worktree/feat-multi-tab-pane-ticket-03-unsaved-confirm/frontend/src/modules/files/index.ts",
      "worktree/feat-multi-tab-pane-ticket-03-unsaved-confirm/frontend/src/modules/layout/panes/FilePane.svelte"
    ],
    "reason": "tabRegistry.ts dari foundation sudah sesuai kontrak, tidak diubah. npm run build lulus. npm run check tidak dijalankan."
  },
  {
    "slug": "unsaved-wiring",
    "ok": true,
    "filesChanged": [
      "worktree/feat-multi-tab-pane-ticket-03-unsaved-confirm/frontend/src/modules/layout/PaneHeader.svelte"
    ],
    "reason": "Hanya PaneHeader.svelte yang diubah. Tombol tutup tab, tombol Delete, menu konteks \"Tutup tab\" dan \"Tutup pane\" melewati guardedClose dengan flag closing. Tab kotor menampilkan titik dengan lebar tetap dan label sr-only. PaneMenu.svelte tidak diubah. Akhiri sesi, minimize, move, split dan reorder tidak dibungkus."
  }
]
```

## Hasil wiring

```json
{
  "ok": true,
  "filesChanged": [
    "worktree/feat-multi-tab-pane-ticket-03-unsaved-confirm/frontend/src/modules/layout/PaneHeader.svelte"
  ],
  "reason": "Wiring terpasang di PaneHeader.svelte. minimizeTab dan endActiveSession sengaja tidak dilindungi sesuai pengecualian di overview."
}
```

## Build

- `cd frontend && npm run build`: lulus, "✓ built in 1.66s" (diverifikasi ulang).
- `cd frontend && npm run check`: 0 ERRORS, 5 WARNINGS lama (state_referenced_locally di PaneHostRoot.svelte dan TerminalTab.svelte, tidak terkait perubahan ini).
- `make build`: lulus (build-ui, shikigamid, shikigamictl, app).
- `go build ./...`: tidak ada output galat.

## Temuan terbuka (di luar cakupan tiket ini)

- Tidak ada penanganan tutup jendela/keluar aplikasi untuk file kotor (tidak ada onCloseRequested, OnBeforeClose, atau pemeriksaan file kotor di beforeunload).
- Penutupan lewat caller lain (shell, groups, bridge v1) belum diperiksa.

## Langkah manual yang ditunda

- Tidak ada yang ditunda oleh workflow. Verifikasi e2e manual (checklist langkah 10 di plan) belum dijalankan dan perlu dilakukan dengan app berjalan.
