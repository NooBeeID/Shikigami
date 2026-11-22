# Progress: thread-ticket-05-thread-recovery

Worktree: `worktree/feat-thread-ticket-05-thread-recovery` (branch `feat/thread-ticket-05-thread-recovery`)

- status: OK
- foundation: selesai

## Hasil per fitur (JSON)

```json
[
  {
    "slug": "thread-lazy-respawn",
    "ok": true,
    "filesChanged": [
      "worktree/feat-thread-ticket-05-thread-recovery/internal/thread/recovery.go"
    ],
    "reason": "recovery.go sudah diimplementasikan penuh sesuai plan. `go build` dan `go vet` pada paket internal/thread lulus, dan saya tidak menjalankan build seluruh project. Catatan wiring: ikuti bagian Wiring plan di cmd/shikigamid/wire_thread.go, yaitu `rec := thread.NewRecovery(m)`, `m.SetPreSend(rec.PreSend)`, lalu `m.OnStart` yang memanggil `rec.MarkDeadOnStart()`, semuanya sebelum `m.FireStart()`. Nama helper dan tanda tangan di kode sama dengan asumsi plan, jadi tidak ada penyesuaian."
  },
  {
    "slug": "thread-orphan-cleanup",
    "ok": true,
    "filesChanged": [
      "worktree/feat-thread-ticket-05-thread-recovery/internal/thread/orphans.go",
      "worktree/feat-thread-ticket-05-thread-recovery/frontend/src/modules/thread/recovery/index.ts",
      "worktree/feat-thread-ticket-05-thread-recovery/frontend/src/modules/thread/recovery/util.ts",
      "worktree/feat-thread-ticket-05-thread-recovery/frontend/src/modules/thread/recovery/DeadBadge.svelte",
      "worktree/feat-thread-ticket-05-thread-recovery/frontend/src/modules/thread/recovery/RecoveryPanel.svelte",
      "worktree/feat-thread-ticket-05-thread-recovery/frontend/src/modules/thread/recovery/RecoveryNotice.svelte"
    ],
    "reason": "CleanOrphans dan UI recovery sudah diimplementasikan. `go build ./internal/thread/` dan `npm run build` lulus. Wiring dikerjakan pada langkah terpisah. Deviasi kecil dari plan: orphansListLeftovers juga menerima (m, gitBin, repos), dan orphansRemoveWorktree mengembalikan (removed bool, err). Entri stale yang sudah tidak ada di disk hanya di-prune dan tidak dihitung."
  }
]
```

## Wiring

- status: ok
- filesChanged:
  - `worktree/feat-thread-ticket-05-thread-recovery/cmd/shikigamid/wire_thread.go`
  - `worktree/feat-thread-ticket-05-thread-recovery/frontend/src/modules/thread/index.ts`
- catatan: wire_thread.go memasang `NewRecovery`, `SetPreSend`, hook `OnStart` untuk `MarkDeadOnStart`, dan hook `OnStart` untuk `CleanOrphans` (dalam goroutine dengan recover), semuanya sebelum `FireStart`. index.ts memanggil `installRecovery()` setelah `installClose()`.

## Langkah manual yang ditunda

- tidak ada

## Build guard

- passed: true
- `go build ./...` exit 0
- `npm run build` (frontend) exit 0; hanya warning svelte `state_referenced_locally` dan peringatan ukuran chunk yang sudah ada sebelumnya
- `make build-ui` dan `make build-daemon` berhasil tanpa error

Verifikasi ulang pada akhir eksekusi: `go build ./...` dan `npm run build` di worktree lulus.
