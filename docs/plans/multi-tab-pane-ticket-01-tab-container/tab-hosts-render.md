# Plan: Host konten per tab dan komponen pane

## Metadata

- Tiket: `multi-tab-pane-ticket-01-tab-container` (`docs/tickets/multi-tab-pane-ticket-01-tab-container.md`)
- Fitur: `tab-hosts-render`
- Overview: `docs/plans/multi-tab-pane-ticket-01-tab-container/overview.md` (kontrak bagian 2.1, 2.2, 2.4, 2.5, 2.7)
- ownedPaths:
  - `frontend/src/modules/layout/panes/`
  - `frontend/src/modules/layout/LayoutArea.svelte`
  - `frontend/src/modules/lark/board/BoardPane.svelte`
  - `frontend/src/modules/git/MrPane.svelte`
  - `frontend/src/modules/git/PipelinePane.svelte`
  - `frontend/src/modules/thread/ThreadPane.svelte`
- Depends on foundation: ya. Tipe `Tab`/`Leaf` baru, `activeTab`, `allTabs`, `findTab`, `leaves`, `findLeaf` sudah ada di `layout/model.ts` (lihat `00-foundation.md`). Aksi store `renameTab`/`activateTab`/`closeTab` dan `PaneKindSpec` baru dikerjakan agent `tab-model-foundation` secara paralel; patuhi kontrak overview 2.4 dan 2.5 persis, jangan menyunting `model.ts`, `registry.ts`, `lib/stores/layout.ts`.

## Tujuan

Isi pane (terminal, file, thread, lark, git) di-host per id TAB, bukan per id pane. Semua tab semua pane tetap ter-mount; hanya tab aktif di pane yang terlihat yang `visible=true` dan terpasang di slot pane. Pindah tab atau (kelak) pindah tab antar pane tidak me-remount isi, sehingga terminal tidak restart dan scrollback utuh. Semua komponen pane menerima `pane: Tab` dan `paneId`.

## Path yang dimiliki

HANYA path berikut yang boleh disentuh:

- `frontend/src/modules/layout/panes/` (`hosts.ts`, `PaneHostRoot.svelte`, `TerminalPane.svelte`, `FilePane.svelte`, `ReadOnlyFileView.svelte`, `terminal.ts`, `file.ts`)
- `frontend/src/modules/layout/LayoutArea.svelte`
- `frontend/src/modules/lark/board/BoardPane.svelte`
- `frontend/src/modules/git/MrPane.svelte`
- `frontend/src/modules/git/PipelinePane.svelte`
- `frontend/src/modules/thread/ThreadPane.svelte`

Dilarang menyentuh: `PaneHeader.svelte`, `PaneMenu.svelte`, `LayoutHeaderHost.svelte`, `model.ts`, `registry.ts`, `lib/stores/layout.ts`, `persist/*`, `App.svelte`, `layout/index.ts`, `lark/index.ts`, `lark/doc/DocPane.svelte`, `git/index.ts`, `thread/index.ts`, `modules/terminal/*`, `modules/files/*` (termasuk registri dirty; `FilePane` hanya diubah agar menerima `Tab`), `layout/dnd.ts`, `layout/DropOverlay.svelte`. Tidak ada unit test/vitest.

## Kontrak

Dipakai (dari overview, jangan diubah):
- `Tab`, `Leaf`, `activeTab(leaf)`, `allTabs(doc)`, `findTab(doc, pred)`, `leaves(n)`, `findLeaf(doc, paneId)` dari `../model` (overview 2.1, 2.2).
- Store `layout` (`lib/stores/layout.ts`): `renameTab(paneId, tabId, title)`, `activateTab(paneId, tabId)`, `closeTab(paneId, tabId)`, `closePane(paneId)`, `setFocus`, `maximize`, `restore`, `tidyGrid`, `splitPane...` (overview 2.4).
- `PaneKindSpec` (overview 2.5): `component: Component<{ pane: Tab; paneId: string; visible: boolean }>`, `titleFor/onEndSession/badgeFor` menerima `Tab`.

Disediakan (dipakai hanya oleh `LayoutArea.svelte`, fitur yang sama):

```ts
// layout/panes/hosts.ts, semua di-key id TAB
export function ensureHost(tab: Tab, paneId: string, visible: boolean): HTMLElement;
export function getHost(tabId: string): HTMLElement | null;
export function attachHost(tabId: string, slot: HTMLElement): void;
export function detachHost(tabId: string): void;   // lepas dari DOM, TIDAK unmount
export function releaseHost(tabId: string): void;  // unmount + hapus entry
export function syncPanes(
  entries: { pane: Leaf; tab: Tab }[],   // hasil allTabs(doc)
  visibleTabIds: ReadonlySet<string>,    // id tab yang visible (tab aktif di pane yang terlihat)
): void;
```

Props semua komponen pane: `{ pane: Tab; paneId: string; visible: boolean }` (komponen yang tidak butuh `paneId`/`visible` cukup tidak mendestruktur-nya).

## Langkah

1. **`panes/PaneHostRoot.svelte`.** Ganti tipe `Leaf` menjadi `Tab`. Props: `{ component: Component<{ pane: Tab; paneId: string; visible: boolean }>; pane: Tab; paneId: string; visible: boolean }`. State `cur = $state({ pane, paneId, visible })`. `export function update(p: Tab, id: string, v: boolean) { cur = { pane: p, paneId: id, visible: v }; }`. Render `<Comp pane={cur.pane} paneId={cur.paneId} visible={cur.visible} />`. CSS tidak berubah.

2. **`panes/hosts.ts`.** Tulis ulang dengan kunci id tab:
   - `Entry`: `{ host, kind, instance: { update(pane: Tab, paneId: string, visible: boolean): void } | null, unmountFn, tab: Tab, paneId: string, visible: boolean }`.
   - `mountInto` meneruskan `props: { component: spec.component, pane: entry.tab, paneId: entry.paneId, visible: entry.visible }`.
   - `updateEntry(entry, tab, paneId, visible)`: simpan `entry.tab/paneId/visible`; bila `tab.kind !== entry.kind` unmount + mount ulang (logika sekarang); bila `instance` null coba mount ulang bila kind sudah terdaftar; selain itu `entry.instance.update(tab, paneId, visible)` dalam try/catch.
   - `ensureHost(tab, paneId, visible)`: `entries.get(tab.id)`; bila ada `updateEntry` dan kembalikan host; bila belum buat `div.pane-host` dengan style yang sama seperti sekarang, simpan di `entries` dengan kunci `tab.id`, `mountInto`.
   - `getHost/attachHost/detachHost/releaseHost` identik dengan versi lama tetapi parameter adalah `tabId`. `detachHost` hanya `host.remove()` (tanpa unmount).
   - `syncPanes(entries, visibleTabIds)`: untuk tiap `{ pane, tab }` panggil `ensureHost(tab, pane.id, visibleTabIds.has(tab.id))`; kumpulkan set id tab; `releaseHost` untuk setiap kunci di map yang tidak ada di set. (Tab non-aktif tetap dipanggil `ensureHost` dengan `visible=false`, jadi tetap ter-mount.)
   - Host baru tidak boleh di-mount dobel: kunci hanya id tab, bukan `paneId`, sehingga perubahan `paneId` pada tab yang sama (pindah antar pane, tiket 02) hanya memanggil `update`, tanpa remount.

3. **`panes/TerminalPane.svelte`.** Props `{ pane, paneId, visible }: { pane: Tab; paneId: string; visible: boolean }`. `onTitle` memanggil `layout.renameTab(paneId, pane.id, t)` (bukan `renamePane`), filter judul tetap (buang ikon/spinner di awal). Tetap `{#key pane.sessionId}` membungkus `TerminalTab`. Refit saat `visible` jadi true: `modules/terminal/TerminalTab.svelte` (bukan milik fitur ini, jangan diubah) sudah menjalankan `safeFit()` + `sendResize()` di `applyVisible(true)` dan `ResizeObserver` memanggil `refitSettled` saat host dipasang kembali ke DOM, jadi jalur utamanya sudah ada. Verifikasi dengan membaca kode itu, lalu tambahkan pengaman di `TerminalPane`: `$effect` yang saat `visible` berubah dari false ke true menunggu `tick()` dan dua `requestAnimationFrame`, lalu bila `document.activeElement` tidak berada di dalam terminal dan pane fokus, tidak melakukan apa-apa lain (jangan mencuri fokus). Pengaman yang boleh dipakai hanya yang tidak mengubah layout (mis. `window.dispatchEvent(new Event('resize'))`, tidak berbahaya dan memicu pendengar resize lain). Jangan membungkus `TerminalTab` dengan div baru agar CSS `.pane-root > :global(*)` tidak rusak. Catat di laporan hasil verifikasi bahwa refit berjalan lewat `applyVisible`/`ResizeObserver`.

4. **`panes/FilePane.svelte`.** Hanya tipe: `let { pane }: { pane: Tab; paneId?: string; visible?: boolean } = $props();` (tetap hanya memakai `pane`). Jangan menambah registri dirty, jangan mengubah `FileTab`. `refId` tetap dari `pane.filePath/projectId/workspaceId`.

5. **`panes/ReadOnlyFileView.svelte`.** Tidak ada perubahan tipe `Leaf` di berkas ini (menerima `root/path/projectId`); biarkan kecuali ada impor `Leaf`. Pastikan tetap kompilasi.

6. **`panes/terminal.ts` dan `panes/file.ts`.** Ganti `Leaf` menjadi `Tab` (import type dari `../model`) di `providerIdOf`, `titleFor`, `badgeFor`, `onEndSession`. Nama parameter boleh jadi `tab`. Logika isi tidak berubah. `component` tetap komponen masing-masing (tipe cocok dengan `PaneKindSpec` baru).

7. **Komponen kind lain.** Di `BoardPane.svelte`, `MrPane.svelte`, `PipelinePane.svelte`, `ThreadPane.svelte`: ganti `import type { Leaf }` menjadi `Tab` dan props menjadi `{ pane, visible }: { pane: Tab; paneId?: string; visible: boolean }` untuk Mr/Pipeline/Thread (tidak butuh `paneId`; boleh deklarasikan opsional agar kompatibel dengan `PaneHostRoot`, atau cukup tidak mendestruktur karena Svelte tidak menolak prop ekstra pada komponen bertipe `Component<{...}>`; pilih bentuk yang lolos `svelte-check`). `BoardPane` menerima `{ pane, paneId, visible }: { pane: Tab; paneId: string; visible: boolean }`.

8. **`BoardPane.svelte` `persistPane()`.** Ganti penelusuran pohon manual dengan `findTab(get(layout), (t) => t.id === pane.id)?.tab`. Pertahankan pola mutasi lokal yang ada (set `tab.tableId`, set/hapus `tab.viewId`) karena store tidak punya setter props tab, lalu pancarkan store lewat `layout.renameTab(paneId, pane.id, tab.title || 'Tiket Lark')` (ganti `layout.renamePane(pane.id, ...)`). Hapus impor `Node`/`Leaf` yang tak terpakai, tambahkan impor `findTab` dari `../../layout/model`. Perbarui komentar di atasnya. Store `writable` memancarkan ulang walau objek sama, jadi autosave tetap terpicu.

9. **`LayoutArea.svelte`.**
   - Impor `emptyDoc, leaves, findLeaf, pathOf, allTabs, activeTab` plus tipe `Leaf, Node, LayoutDoc`; `ensureHost, getHost, attachHost, detachHost, syncPanes` dari `./panes/hosts`.
   - `const tabs = $derived(allTabs(doc));`
   - `visibleTabIds = $derived(new Set(all.filter((l) => !maxId || l.id === maxId).map((l) => activeTab(l).id)))`.
   - `$effect.pre(() => { syncPanes(tabs, visibleTabIds); });`
   - Action `hostSlot(node, p: { tab: Tab; paneId: string; visible: boolean })`: `ensureHost(p.tab, p.paneId, p.visible); attachHost(p.tab.id, node);` dan `destroy()` hanya `detachHost(id)` bila `getHost(id)?.parentElement === node` (jangan melepas host yang sudah dipindah ke slot lain). Sediakan juga `update(np)` yang melakukan ensure+attach untuk props baru.
   - Di snippet `renderNode` (cabang pane): hitung `const at = activeTab(n)` via `{@const at = activeTab(n)}`. Bungkus slot dengan `{#key at.id}<div class="pane-slot" use:hostSlot={{ tab: at, paneId: n.id, visible: visibleTabIds.has(at.id) }}></div>{/key}` sehingga pergantian tab membuat slot baru yang memasang host tab itu; slot lama dihancurkan dan hanya melepas host-nya dari DOM tanpa unmount. `{#key n.id}` pada level pane dipertahankan.
   - `closeAndEnd(pane: Leaf)`: `store.closePane(pane.id)` lalu untuk setiap `tab` di `pane.tabs` dengan `kind === 'terminal' && tab.sessionId` jalankan `endSession(tab)` (tutup pane = akhiri semua sesi terminal di dalamnya, seperti perilaku sekarang). `endSession(tab: Tab)` memakai `getPaneKind(tab.kind)?.onEndSession?.(tab)` dalam try/catch.
   - Props `PaneHeader`: pertahankan prop yang ada (`pane={n}`, `index`, `focused`, `maximized`, `store`, `onSplit`, `onMaximizeToggle`, `onClose`, `onMinimize`, `onEndSession`, `onTidy`). Adaptasi callback: `onSplit` tetap `splitPaneWithNewSession(n.id, dir, store)`; `onEndSession={() => void endSession(activeTab(n))}`; `onMinimize={() => store.closeTab(n.id, activeTab(n).id)}`; `onRename={(t) => store.renameTab(n.id, activeTab(n).id, t)}`. Fitur ini TIDAK menentukan prop baru `PaneHeader` (milik `tab-header-ui`); aksi tab (`activateTab`, `closeTab`) diharapkan diambil header dari prop `store`. Bila kontrak prop `PaneHeader` akhirnya berbeda, penyesuaian jadi tugas wiring (jangan edit `PaneHeader`).
   - `onpointerdowncapture` fokus pane, `maxPath`, `inMax`, CSS tidak berubah.

10. **Verifikasi lokal.** Dari `frontend/`: `npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E "layout/panes/|layout/LayoutArea.svelte|lark/board/BoardPane.svelte|git/MrPane.svelte|git/PipelinePane.svelte|thread/ThreadPane.svelte"` tidak menampilkan error baru. Error dari file milik fitur lain diabaikan.

## Perubahan

Diubah:
- `frontend/src/modules/layout/panes/hosts.ts`
- `frontend/src/modules/layout/panes/PaneHostRoot.svelte`
- `frontend/src/modules/layout/panes/TerminalPane.svelte`
- `frontend/src/modules/layout/panes/FilePane.svelte`
- `frontend/src/modules/layout/panes/terminal.ts`
- `frontend/src/modules/layout/panes/file.ts`
- `frontend/src/modules/layout/LayoutArea.svelte`
- `frontend/src/modules/lark/board/BoardPane.svelte`
- `frontend/src/modules/git/MrPane.svelte`
- `frontend/src/modules/git/PipelinePane.svelte`
- `frontend/src/modules/thread/ThreadPane.svelte`

Opsional (hanya bila perlu kompilasi): `layout/panes/ReadOnlyFileView.svelte`. Tidak ada berkas baru.

## Wiring

Yang diekspos untuk langkah wiring (jangan disambungkan sendiri):
- `syncPanes(allTabs(doc), visibleTabIds)` dan API host per id tab, hanya dipakai `LayoutArea.svelte` (sudah tersambung di dalam fitur ini).
- Semua komponen pane menerima `{ pane: Tab; paneId: string; visible: boolean }`, sesuai `PaneKindSpec` baru.
- Catatan untuk `tab-wiring` (berkas di luar ownedPaths yang akan kena error tipe bila tidak diadaptasi; laporkan, jangan edit):
  - `modules/lark/doc/DocPane.svelte` (`pane: Leaf` menjadi `Tab`), `modules/lark/index.ts`, `modules/thread/index.ts`, `modules/git/index.ts` (`titleFor: (leaf) => ...` menjadi parameter `Tab`).
  - `modules/persist/TerminalGuard.svelte` (milik `tab-persist-migration`) harus meneruskan `paneId` ke `TerminalPane` agar judul terminal memperbarui tab yang benar.
  - Prop `PaneHeader` yang ditetapkan `tab-header-ui` mungkin berbeda dari yang dipakai `LayoutArea` (lihat langkah 9); sesuaikan di wiring bila berbeda.
  - `lib/stores/layout.ts` harus mengekspos `closeTab` dan `renameTab` pada tipe `LayoutStore` (kontrak 2.4).

## Selesai bila

- `hosts.ts` ter-key id tab; `syncPanes` menerima `{ pane, tab }[]`; tab non-aktif tetap ter-mount dengan `visible=false`; `releaseHost(tabId)` dipanggil untuk tab yang hilang dari doc.
- `LayoutArea` memasang host tab aktif ke slot pane lewat `{#key activeTab.id}` tanpa unmount host tab lain.
- Semua komponen pane (`TerminalPane`, `FilePane`, `BoardPane`, `MrPane`, `PipelinePane`, `ThreadPane`) menerima `pane: Tab` (dan `paneId` bila dibutuhkan); `TerminalPane` memanggil `renameTab(paneId, pane.id, title)` dan refit saat `visible` jadi true terverifikasi; `BoardPane.persistPane` tidak memakai `renamePane`.
- `panes/terminal.ts` dan `panes/file.ts` memakai `PaneKindSpec` baru (`Tab`).
- Tidak ada file di luar ownedPaths yang diubah; `PaneHeader` dan registri dirty tidak tersentuh.
- Guard: `svelte-check` yang difilter ke ownedPaths di atas tidak menampilkan error (error dari file fitur lain diabaikan) dan tidak ada unit test baru. Build penuh diverifikasi di `tab-wiring`.
