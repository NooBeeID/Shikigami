# Shikigami v2 UI: Index Tiket

Sumber: `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md`

## Ringkasan Tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|---|---|---|---|---|
| shikigami-v2-ui-ticket-01-app-install | Instalasi Shikigami.app ke Applications | 1 | - | 3 |
| shikigami-v2-ui-ticket-02-native-menu | Menu native, zoom, dan Open Folder | 2 | 01-app-install | 4 |
| shikigami-v2-ui-ticket-03-pane-layout | Layout multi-pane (split, drag and drop, resize) | 3 | 02-native-menu | 6 |
| shikigami-v2-ui-ticket-04-shell-sidebars | Bilah atas dan dua sidebar | 4 | 03-pane-layout | 7 |
| shikigami-v2-ui-ticket-05-activity-feed | Feed aktivitas (tab Activity) | 5 | 04-shell-sidebars | 2 |
| shikigami-v2-ui-ticket-06-workspace-persist | Save workspace otomatis dan pemulihan | 5 | 04-shell-sidebars | 4 |
| shikigami-v2-ui-ticket-07-session-groups | Grup sesi sebagai tab | 6 | 06-workspace-persist | 2 |

Total: 7 tiket, 28 fitur.

Fitur per tiket:
- 01: loginenv-path, app-bundle-script, make-install-targets
- 02: menu-bridge-contract, native-menu-impl, zoom, open-folder
- 03: layout-model, layout-renderer, pane-header-dnd, pane-kinds, layout-wiring, perf-6-pane
- 04: shell-frame, topbar, sidebar-left-agent, sidebar-right-files, command-palette, notification-bell, shell-wiring
- 05: activity-daemon, activity-ui
- 06: schema-v2-daemon, autosave-restore, restart-dead-session, window-state
- 07: groups-store, group-tabs-ui

## Urutan Eksekusi per Wave

Tiket dalam wave yang sama boleh dikerjakan paralel, dengan syarat dependensinya sudah selesai.

1. Wave 1: 01-app-install
2. Wave 2: 02-native-menu (setelah 01)
3. Wave 3: 03-pane-layout (setelah 02)
4. Wave 4: 04-shell-sidebars (setelah 03)
5. Wave 5 (paralel): 05-activity-feed dan 06-workspace-persist (keduanya setelah 04)
6. Wave 6: 07-session-groups (setelah 06)

Pane-layout (tiket 03) diakhiri dengan laporan `docs/perf/multipane-report.md`.

## Kontrak Bersama

Semua tiket wajib mematuhi kontrak berikut. Tiket baru menambah file/modul sendiri dan hanya mendaftar lewat registri yang sudah ada. Perluasan file milik tiket lain wajib disebut di bagian Wiring tiket yang bersangkutan.

### 1. Titik ekstensi (wajib dipakai)
- Daemon: registri modul di `cmd/shikigamid/modules.go`, satu file `wire_*.go` per modul, lewat `Deps.Provide`. Registri IPC ada di `internal/ipc`.
- UI: `lib/registry.ts` (`registerPanel`, `registerTabType`). Registri baru yang ditambahkan oleh tiket tertentu:
  - `registerPaneKind` di `modules/layout/registry.ts` (tiket 03, pane-layout)
  - `registerRightTab` dan `registerTopbarItem` di `modules/shell` (tiket 04, shell-sidebars)

### 2. Flag izin agent
- `claude --dangerously-skip-permissions`
- `codex --dangerously-bypass-approvals-and-sandbox`
- Kedua flag wajib dipertahankan dan tidak boleh digandakan di jalur kode mana pun.

### 3. Event menu native (tiket 02)
- Event Wails:
  - `menu:zoom` dengan payload `{action: 'in' | 'out' | 'reset'}`
  - `menu:open-folder`
  - `menu:new-workspace-from-folder`
  - `menu:toggle-sidebar-left`
  - `menu:toggle-sidebar-right`
  - `files:dropped` dengan payload `{paths: string[]}`
- Bridge Go: `OpenDirectoryDialog(): string`, `AppConfigGet()`, `AppConfigSet(patch)`.
- `config.json` di data dir dengan field `zoom: number` dan `window: {x, y, w, h}`.
- Frontend memakai event bus `modules/appmenu`.

### 4. Pohon layout (tiket 03)
- `Node = Leaf | Split`
- Leaf: `{type: 'pane', id, kind: 'terminal' | 'file', sessionId?, projectId?, provider?, filePath?, title}`
- Split: `{type: 'split', dir: 'h' | 'v', ratio: number (0..1), a: Node, b: Node}`
- Dokumen layout per grup: `{version: 1, root: Node | null, focusId?: string, maximizedId?: string | null}`
- Aksi store: `openPane`, `splitPane`, `closePane` (tidak mematikan sesi), `movePane` (zona `top | bottom | left | right | center`), `resize`, `maximize`, `tidyGrid`, `hydrate(doc)`, `serialize()`.
- Elemen DOM dan instance xterm per pane harus stabil. Pane dipindah, tidak dibuat ulang.

### 5. Skema workspaces.json v2 (tiket 06)
- Versi dinaikkan ke v2. Migrasi otomatis dari v1.
- `Workspace = {id, name, projects[], groups: [{id, name, order, layout: LayoutDoc | null}], activeGroupId}`
- Grup bawaan `Utama` tidak bisa dihapus.
- UI state: `{leftSidebar: {visible, width}, rightSidebar: {visible, width, activeTab}, activeWorkspaceId}`
- Method IPC: `workspace.layout.get` / `workspace.layout.set` (`{workspaceId, groupId, layout?}`), `workspace.ui.get` / `workspace.ui.set`.
- Sesi hanya dimiliki tepat satu grup. Menghapus grup memindahkan sesinya ke `Utama` (dipakai juga oleh tiket 07).

### 6. Aktivitas (tiket 05)
- IPC `activity.list` dengan parameter `{sessionId?, limit?}` mengembalikan `[{ts, sessionId, tool, summary}]`.
- Event `activity.event`.
- Tidak ada aksi Approve/Reject.

### 7. Aturan verifikasi setiap tiket
- Setiap tiket harus lulus `make build`. Ini satu-satunya guard. Tidak ada unit test.
- Aplikasi harus tetap bisa dijalankan setelah setiap tiket.
- `modules/tabs` v1 boleh berupa shim sampai shell baru aktif (tiket 04).
