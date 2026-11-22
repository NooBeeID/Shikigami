# Multi-Tab Pane: Index Tiket

Sumber: `docs/superpowers/specs/2026-11-22-multi-tab-pane-design.md`

Pane menjadi wadah tab. Setiap pane menampung satu atau lebih tab, tab bisa dipindahkan antar pane, dan file yang belum disimpan dikonfirmasi sebelum ditutup.

## Ringkasan Tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|---|---|---|---|---|
| multi-tab-pane-ticket-01-tab-container | Pane sebagai wadah tab | 1 | - | 6 |
| multi-tab-pane-ticket-02-tab-dnd | Drag tab, reorder, split, dan menu konteks tab | 2 | multi-tab-pane-ticket-01-tab-container | 4 |
| multi-tab-pane-ticket-03-unsaved-confirm | Konfirmasi tutup file belum disimpan | 3 | multi-tab-pane-ticket-02-tab-dnd | 3 |

## Fitur per Tiket

**Ticket 01: Pane sebagai wadah tab** (wave 1)
- tab-model-foundation
- tab-persist-migration
- tab-callers
- tab-hosts-render
- tab-header-ui
- tab-wiring

**Ticket 02: Drag tab, reorder, split, dan menu konteks tab** (wave 2)
- move-tab-model
- tab-dnd-overlay
- tab-context-menu
- tab-dnd-wiring

**Ticket 03: Konfirmasi tutup file belum disimpan** (wave 3)
- file-tab-registry
- confirm-close-dialog
- unsaved-wiring

## Urutan Eksekusi

- **Wave 1:** `multi-tab-pane-ticket-01-tab-container`
- **Wave 2:** `multi-tab-pane-ticket-02-tab-dnd` (setelah wave 1 selesai)
- **Wave 3:** `multi-tab-pane-ticket-03-unsaved-confirm` (setelah wave 2 selesai)

Setiap wave hanya berisi satu tiket, sehingga tidak ada eksekusi paralel. Ketiga tiket berurutan karena menyentuh `PaneHeader.svelte` secara bersamaan.

## Kontrak Bersama

- Semua perubahan bersifat frontend-only (Svelte 5 + TypeScript). Tidak ada perubahan daemon atau IPC.
- **Model** ada di `frontend/src/modules/layout/model.ts`, dibuat di tiket tab-container. Bentuknya tidak diubah setelahnya, hanya boleh ditambah.
  - `Tab = { id (prefix t_), kind, sessionId?, projectId?, provider?, filePath?, threadId?, workspaceId?, root?, readOnly?, threadAgent?, sourceId?, tableId?, viewId?, nodeToken?, number?, pipelineId?, title? }`
  - `Leaf = { type: 'pane', id, tabs: Tab[] (tidak pernah kosong), activeTabId (selalu menunjuk tab yang ada) }`
  - Pohon split (`Split`, `ratio`, `Splitter`) tidak berubah. `focusId` dan `maximizedId` tetap berupa id pane.
  - Helper murni: `activeTab(leaf)`, `allTabs(doc) -> {pane, tab}[]`, `findTab(doc, pred)`.
  - Operasi: `openPane(doc, nl)` (dedupe lintas pane, menambah tab di pane fokus, `opts.dir` diabaikan), `activateTab(doc, paneId, tabId)`, `closeTab(doc, paneId, tabId)`, `renameTab(doc, paneId, tabId, title)`.
  - Tiket tab-dnd menambah `moveTab(doc, fromPaneId, tabId, targetPaneId, zone: center|top|bottom|left|right, index?)` tanpa mengubah operasi lain.
- **PaneKindSpec** (`layout/registry.ts`): `component` menerima `{ pane: Tab; visible: boolean }` (nama prop tetap `pane` agar komponen kind lama tetap kompatibel; tipenya `Tab`), beserta `paneId` bila dibutuhkan. `titleFor`, `onEndSession`, dan `badgeFor` menerima `Tab`.
- **Persistensi:** `LayoutDoc.version` tetap `1`. `hydrateDoc` menerima bentuk lama (leaf dengan field konten, dibungkus menjadi pane satu tab, id pane dipertahankan, id tab baru) dan bentuk baru. Autosave menulis bentuk baru.
- **Host konten** (`panes/hosts.ts`) di-key dengan id tab. Tab non-aktif tetap ter-mount dengan `visible=false`. `releaseHost` dipanggil saat tab ditutup. Dengan begitu, memindahkan tab antar pane tidak me-remount isi.
- **Pencarian pemanggil:** helper di `shell/helpers.ts` mengembalikan `{ paneId, tabId }`. Kontrak publik `openPane(nl, opts?)` tidak berubah.
- **Unsaved-confirm** menyediakan di modul `files`:
  - `registerFileTab(tabId, { isDirty(): boolean, save(): Promise<boolean> })`
  - store dirty per id tab
  - `confirmCloseTabs(tabIds): Promise<boolean>`
  - `PaneHeader` dan `PaneMenu` memanggil `confirmCloseTabs` sebelum `closeTab` atau aksi Tutup pane. Penutupan akibat sistem dan `moveTab` tidak memanggilnya.
- **Urutan file bersama:** `PaneHeader.svelte` disentuh berurutan oleh tab-container (wave 1), tab-dnd (wave 2), dan unsaved-confirm (wave 3).
- **Verifikasi:** tanpa unit test (tidak menambah vitest). Guard hanya build frontend dan build aplikasi.
