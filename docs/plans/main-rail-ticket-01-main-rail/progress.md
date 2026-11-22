# Progress: main-rail-ticket-01-main-rail

## Ringkasan eksekusi

- status: OK
- foundation: tidak ada
- wiring: ok (rail.ts, AgentRailIcon.svelte, sidebar-left/index.ts, shell/register.ts, App.svelte)
- langkah manual yang ditunda: tidak ada
- build guard terakhir: passed=true (go build ./... exit 0; npm run build exit 0)
- catatan: commit dasar branch ini adalah e2c0622 (implementasi sebagian sebelumnya). Commit ini melengkapi wiring dan status tiket.

## Hasil per fitur (JSON)

```json
[
  {
    "slug": "rail-registry-store",
    "ok": true,
    "filesChanged": [
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/shell/registry.ts",
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/lib/stores/shell.ts",
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/shell/index.ts"
    ],
    "reason": "Ketiga ownedPaths sudah berisi implementasi sesuai plan (sudah ada di e2c0622). Tidak ada edit baru. npm run build di frontend lulus."
  },
  {
    "slug": "rail-state-persist-go",
    "ok": true,
    "filesChanged": [
      "worktree/feat-main-rail-ticket-01-main-rail/internal/workspace/types.go"
    ],
    "reason": "types.go sudah memuat field ActiveRail (json activeRail) dan default \"agents\" (sudah ada di e2c0622). Tidak ada edit baru. go build ./... lulus."
  },
  {
    "slug": "agents-rail-item",
    "ok": true,
    "filesChanged": [
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/sidebar-left/AgentRailIcon.svelte",
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/sidebar-left/rail.ts",
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/sidebar-left/index.ts",
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/shell/register.ts",
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/App.svelte"
    ],
    "reason": "Wiring sudah terpasang (berkas belum di-commit sebelum eksekusi ini, sekarang ikut di commit). Item agents didaftarkan lewat registerRailItem di sidebar-left/rail.ts, dan register.ts mengimpor sidebar-left sehingga item terdaftar sekali. App.svelte memakai RailPanelHost di snippet left."
  },
  {
    "slug": "rail-state-persist-ui",
    "ok": true,
    "filesChanged": [
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/persist/restore.ts"
    ],
    "reason": "applyUi memuat activeRail dengan fallback 'agents' (sudah ada di e2c0622). uiPayload menyalin leftSidebar utuh dan leftSidebar.subscribe memicu simpan. Tidak ada edit baru di eksekusi ini."
  },
  {
    "slug": "rail-ui",
    "ok": true,
    "filesChanged": [
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/shell/Rail.svelte",
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/shell/RailPanelHost.svelte",
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/shell/ShellFrame.svelte",
      "worktree/feat-main-rail-ticket-01-main-rail/frontend/src/modules/shell/shell.css"
    ],
    "reason": "Rail.svelte, RailPanelHost.svelte, grid empat kolom di ShellFrame, dan CSS rail sudah ada (e2c0622). Tidak ada edit baru. Catatan: Resizer.svelte menghitung lebar kiri dari clientX mentah, padahal panel kini bergeser 48px oleh rail, sehingga lebar saat drag bisa 48px lebih besar dari posisi kursor. Berkas itu di luar ownedPaths dan tidak diubah."
  }
]
```

## Wiring

- ok: true
- Item `agents` terdaftar di registry melalui `sidebar-left/rail.ts`, memakai `AgentRailIcon.svelte`.
- `sidebar-left/index.ts` mengimpor `./rail`. `shell/register.ts` mengimpor `../sidebar-left`.
- `App.svelte` memakai `RailPanelHost` di snippet `left`.

## Build

- `go build ./...` di worktree: exit 0 tanpa output.
- `npm run build` di frontend/: exit 0 (vite build selesai). Warning yang ada bersifat lama: `state_referenced_locally` di file layout dan peringatan chunk lebih dari 500 kB.
- Worktree tidak punya node_modules. Build dijalankan dengan symlink sementara ke node_modules root, lalu symlink dihapus.

## Batasan verifikasi

- Verifikasi hanya build dan pemeriksaan keberadaan berkas. Tidak ada uji UI manual (rail, perpindahan panel, restore activeRail setelah restart).
- `npm run check` (svelte-check) tidak dijalankan pada eksekusi ini.

## Tindak lanjut

- Uji manual di aplikasi berjalan: klik item rail, buka/tutup sidebar, dan pastikan activeRail tersimpan dan dipulihkan.
- Resizer kiri: sesuaikan perhitungan lebar dengan offset 48px rail (di luar tiket ini).
