# Plan: Model Tab dan fungsi murni tab di model.ts

## Metadata

- Tiket: `docs/tickets/multi-tab-pane-ticket-01-tab-container.md` (epic `multi-tab-pane`, wave 1)
- Fitur: `tab-model-foundation`
- ownedPaths:
  - `frontend/src/modules/layout/model.ts`
  - `frontend/src/modules/layout/registry.ts`
  - `frontend/src/lib/stores/layout.ts`
- Depends on foundation: ya (`00-foundation.md`). Tipe `Tab`/`Leaf` baru, `newPaneId`, `newTabId`, `makeLeaf` (menghasilkan `Tab`), `normalize` (diekspor), `activeTab`, `allTabs`, `findTab`, stub `activateTab`/`closeTab`/`renameTab`, dan re-ekspor `hydrateDoc` dari `../persist/hydrate` sudah ada di `model.ts`. Bila saat mulai ternyata salah satunya belum ada, tambahkan sesuai `overview.md` bagian 2.1 dan 2.2 (jangan ubah bentuk kontrak).

Rujukan wajib: `overview.md` bagian 2 (kontrak 2.1 sampai 2.5), 7 (aturan paralel). Jangan mengulang isinya di sini; plan ini hanya merinci langkah.

## Tujuan

Mengisi semua operasi murni berbasis tab di `model.ts` (`openPane`, `activateTab`, `closeTab`, `renameTab`, `openThreadAgent`, `splitPane`), menyesuaikan operasi pane utuh yang tipenya terpengaruh (`movePane`, `swapPanes`, `tidyGrid`, dst.), mengubah `PaneKindSpec` di `registry.ts` agar memakai `Tab`, dan mengekspos aksi tab di store `layout.ts`. Semua fungsi model tetap murni (tidak memutasi masukan, tanpa IPC).

## Path yang dimiliki

HANYA tiga berkas ini yang boleh disentuh:

- `frontend/src/modules/layout/model.ts`
- `frontend/src/modules/layout/registry.ts`
- `frontend/src/lib/stores/layout.ts`

Jangan sentuh `persist/hydrate.ts`, file wiring (`App.svelte`, `layout/index.ts`), komponen (`LayoutArea.svelte`, `PaneHeader.svelte`, panes), pemanggil (`shell/helpers.ts`, `shortcuts.ts`, `groups.ts`, dst.), maupun berkas tiket 02/03 (`dnd.ts`, `DropOverlay.svelte`, `TabContextMenu.svelte`, `modules/files/`). Error tipe di berkas lain diabaikan dan dicatat di laporan. Jangan menambah `moveTab`, `registerFileTab`, `confirmCloseTabs`, vitest, atau unit test.

## Kontrak

Disediakan (bentuk persis di `overview.md` 2.1 sampai 2.5):

- `model.ts`: `openPane(doc, nl): LayoutDoc`, `activateTab(doc, paneId, tabId): LayoutDoc`, `closeTab(doc, paneId, tabId): LayoutDoc`, `renameTab(doc, paneId, tabId, title): LayoutDoc`, `openThreadAgent(doc, nl): { doc: LayoutDoc; id: string }` (id = id pane), `splitPane(doc, paneId, dir, content, side?): { doc: LayoutDoc; id: string }` (signature lama dipertahankan), tanpa `renamePane`.
- `registry.ts`: `PaneKindSpec` dengan `component: Component<{ pane: Tab; paneId: string; visible: boolean }>`, `titleFor?(tab: Tab)`, `onEndSession?(tab: Tab)`, `badgeFor?(tab: Tab)`.
- `layout.ts` (store): `activateTab(paneId, tabId)`, `closeTab(paneId, tabId)`, `renameTab(paneId, tabId, title)`, `openPane(nl, opts?)` (mengembalikan id pane, `opts.dir` diabaikan), `openThreadAgent(nl)` (mengembalikan id pane).

Dipakai dari foundation: tipe `Tab`, `Leaf`, `NewLeaf`, `LayoutDoc`, `newPaneId`, `newTabId`, `makeLeaf(nl): Tab`, `normalize`, `activeTab`, `allTabs`, `findTab`, `leaves`, `findLeaf`, `findLeafBy`, `pathOf`, `removeLeaf`, `replaceAt`, `getAt`, `setFocus`, `closePane`.

Catatan keputusan (dari tiket/spec):

- `openPane` model mengembalikan `LayoutDoc` (kontrak 2.3). Id pane hasilnya selalu `doc.focusId`; store memakai itu untuk nilai kembalian.
- Store `renamePane(paneId, title)` DIPERTAHANKAN sebagai pembungkus ke `M.renameTab` pada tab aktif pane itu, karena masih ada pemanggil (`LayoutArea.svelte`, `TerminalPane.svelte`, `BoardPane.svelte`) milik fitur lain yang akan migrasi sendiri. Fungsi model `renamePane` TIDAK ada.

## Langkah

### 1. model.ts: periksa kondisi awal

Baca `model.ts` penuh. Pastikan hasil foundation ada (lihat Metadata). Bila `Leaf` masih berbentuk lama, hentikan perubahan dan sempurnakan dulu sesuai overview 2.1 (jangan berspekulasi lebih jauh dari itu).

### 2. model.ts: helper internal

Tambahkan (tidak diekspor, kecuali disebut):

- `function mapLeaf(doc: LayoutDoc, paneId: string, fn: (l: Leaf) => Leaf): LayoutDoc` menelusuri pohon dan mengganti leaf dengan id `paneId` memakai hasil `fn` (struktur baru; node lain dipakai ulang). Tidak mengubah `doc.root` bila pane tidak ada.
- `function wrapTab(tab: Tab): Leaf` mengembalikan `{ type: 'pane', id: newPaneId(), tabs: [tab], activeTabId: tab.id }`.
- `function sameContent(nl: NewLeaf, t: Tab): boolean` berisi lima aturan dedupe persis dari kode lama:
  1. `nl.sessionId !== undefined && t.sessionId === nl.sessionId`
  2. `nl.filePath !== undefined && t.filePath === nl.filePath && t.projectId === nl.projectId`
  3. `nl.threadId !== undefined && t.threadId === nl.threadId`
  4. `nl.kind === 'lark-board' && t.kind === 'lark-board' && nl.sourceId !== undefined && t.sourceId === nl.sourceId`
  5. `nl.kind === 'lark-doc' && t.kind === 'lark-doc' && nl.sourceId !== undefined && t.sourceId === nl.sourceId && (t.nodeToken ?? '') === (nl.nodeToken ?? '')`
- `function focusPaneId(doc: LayoutDoc): string | null`: `doc.focusId` bila `findLeaf` menemukannya; kalau tidak id pane terakhir dari `leaves(doc)`; kalau tidak ada pane `null`.
- `function revealTab(doc: LayoutDoc, paneId: string, tabId: string): LayoutDoc` = `setFocus(activateTab(doc, paneId, tabId), paneId)`.

### 3. model.ts: `activateTab`

Signature `export function activateTab(doc: LayoutDoc, paneId: string, tabId: string): LayoutDoc`. Ganti stub foundation. Bila pane atau tab tidak ada, atau `activeTabId` sudah sama, kembalikan `doc` apa adanya. Selain itu `mapLeaf(doc, paneId, (l) => ({ ...l, activeTabId: tabId }))`. Tidak mengubah `focusId`.

### 4. model.ts: `renameTab`

Signature `export function renameTab(doc: LayoutDoc, paneId: string, tabId: string, title: string): LayoutDoc`. `const t = (title ?? '').trim()`; bila kosong, pane/tab tidak ada, atau judul sama, kembalikan `doc`. Selain itu `mapLeaf` dengan `tabs.map((x) => (x.id === tabId ? { ...x, title: t } : x))`. Hapus `renamePane` dari model bila masih tersisa.

### 5. model.ts: `closeTab`

Signature `export function closeTab(doc: LayoutDoc, paneId: string, tabId: string): LayoutDoc`.

- Pane atau tab tidak ada: kembalikan `doc`.
- `leaf.tabs.length === 1`: `return closePane(doc, paneId)` (aturan fokus/maximized dari `closePane` yang ada).
- Selain itu: `idx = tabs.findIndex(...)`, `tabs = tabs.filter(...)`. `activeTabId`: bila tab yang ditutup bukan tab aktif, tetap. Bila tab aktif: `tabs[idx] ?? tabs[idx - 1]` (setelah filter, `tabs[idx]` adalah tetangga kanan; jika tidak ada pakai kiri). Terapkan lewat `mapLeaf`. `focusId` dan `maximizedId` tidak berubah.

### 6. model.ts: `openPane`

Signature `export function openPane(doc: LayoutDoc, nl: NewLeaf): LayoutDoc` (parameter ketiga `opts` dihapus dari model).

1. Dedupe: `const hit = findTab(doc, (t) => sameContent(nl, t))`. Ketemu: `return revealTab(doc, hit.pane.id, hit.tab.id)`.
2. `const tab = makeLeaf(nl)`.
3. `!doc.root`: `const pane = wrapTab(tab); return { ...doc, root: pane, focusId: pane.id }`.
4. `const pid = focusPaneId(doc)`; bila `null` (pohon ada tetapi tanpa leaf, tidak seharusnya terjadi) perlakukan seperti langkah 3 (ganti `root` dengan pane baru).
5. `mapLeaf(doc, pid, (l) => { const at = l.tabs.findIndex((x) => x.id === l.activeTabId); const i = at < 0 ? l.tabs.length : at + 1; return { ...l, tabs: [...l.tabs.slice(0, i), tab, ...l.tabs.slice(i)], activeTabId: tab.id }; })`, lalu set `focusId: pid`.

### 7. model.ts: `splitPane` dan helper split internal

- Ubah internal: `splitWith(doc, paneId, dir, leaf: Leaf, side): { doc; id }` berisi logika split lama (membuat `Split` dengan `a`/`b` sesuai `side`, `ratio: 0.5`, `focusId: leaf.id`), tetapi menerima `Leaf` utuh. `movePane` memakai ini.
- Publik `splitPane(doc, paneId, dir, content: NewLeaf | Leaf, side = 'after')`:
  - Pane target tidak ada: `{ doc, id: '' }`.
  - `content` adalah Leaf (`type === 'pane'` dan punya `tabs`): `splitWith` langsung (jalur kompatibilitas, dipakai internal dan pemanggil lama).
  - `content` adalah `NewLeaf`: bila `activeTab(target).kind !== 'terminal'` kembalikan `{ doc, id: '' }` (Cmd+D pada tab non-terminal tidak melakukan apa-apa; pane asal tidak berubah). Selain itu `splitWith(doc, paneId, dir, wrapTab(makeLeaf(content)), side)`.
  - Pembeda Leaf vs NewLeaf: `'tabs' in content`.

### 8. model.ts: `openThreadAgent`

Signature tetap `export function openThreadAgent(doc: LayoutDoc, nl: NewLeaf): { doc: LayoutDoc; id: string }`; `id` = id pane yang memuat tab.

1. `sameSession = nl.sessionId !== undefined ? findTab(doc, (t) => t.sessionId === nl.sessionId) : null`. Ada: `{ doc: revealTab(doc, pane.id, tab.id), id: pane.id }`.
2. `slot = findTab(doc, (t) => !!t.threadAgent)`. Ada: `mapLeaf` pada `slot.pane.id` dengan tab `slot.tab.id` diganti `{ ...t, sessionId: nl.sessionId, projectId: nl.projectId, provider: nl.provider, workspaceId: nl.workspaceId, title: nl.title ?? t.title }`; lalu `revealTab`. `id: slot.pane.id`.
3. Belum ada: `const next = openPane(doc, { ...nl, threadAgent: true }); return { doc: next, id: next.focusId ?? '' }`.

### 9. model.ts: operasi pane utuh

Sesuaikan hanya bila tipe memaksa:

- `swapPanes`, `movePane`, `maximize`, `restore`, `resize`, `closePane`, `setFocus`, `removeLeaf`: pakai `Leaf` baru apa adanya (objek pane dibawa utuh bersama `tabs`). `movePane` memanggil `splitWith` (langkah 7), bukan `splitPane`.
- `normalize`/`normalizeNode`: jangan ubah perilakunya; pastikan tetap diekspor (kontrak 2.2). Tambahan opsional yang aman: jaga invarian dengan membuang pane dengan `tabs.length === 0`, dan memperbaiki `activeTabId` yang tidak ada menjadi `tabs[0].id`, di dalam `normalizeNode`. Hanya lakukan bila murah dan tidak mengubah hasil untuk masukan valid.
- `tidyGrid`: tidak butuh perubahan perilaku (bekerja pada `leaves` utuh); hanya perbaiki tipe bila error.
- Hapus sisa kode `hydrateNode`/`hydrateDoc`/`isStr` lama dari model bila foundation belum menghapusnya (kini ada di `persist/hydrate.ts`; re-ekspor `export { hydrateDoc } from '../persist/hydrate';` harus tersisa).

### 10. registry.ts

Ubah impor ke `import type { Tab } from './model';` (hapus impor `Leaf` bila tidak dipakai) dan `PaneKindSpec` menjadi:

```ts
export interface PaneKindSpec {
  kind: string;
  component: Component<{ pane: Tab; paneId: string; visible: boolean }>;
  titleFor?: (tab: Tab) => string;
  onEndSession?: (tab: Tab) => Promise<void>;
  badgeFor?: (tab: Tab) => string | null;
}
```

Fungsi `registerPaneKind`, `paneKinds`, `getPaneKind` tidak berubah.

### 11. lib/stores/layout.ts

- Impor tipe tetap dari model (`Leaf`, `Tab` bila perlu). Interface `LayoutStore`:
  - `openPane(leaf: NewLeaf, opts?: { dir?: 'h' | 'v' }): string;` (tetap; `opts` diterima dan diabaikan, beri komentar).
  - Tambah `activateTab(paneId: string, tabId: string): void;`, `closeTab(paneId: string, tabId: string): void;`, `renameTab(paneId: string, tabId: string, title: string): void;`.
  - `renamePane(paneId: string, title: string): void;` dipertahankan (pembungkus).
  - Lainnya tidak berubah.
- Implementasi:
  - `openPane(leaf)`: `let id = ''; update((d) => { const next = M.openPane(d, leaf); id = next.focusId ?? ''; return next; }); return id;` (jangan teruskan `opts`).
  - `activateTab: (paneId, tabId) => update((d) => M.setFocus(M.activateTab(d, paneId, tabId), paneId))`.
  - `closeTab: (paneId, tabId) => update((d) => M.closeTab(d, paneId, tabId))`.
  - `renameTab: (paneId, tabId, title) => update((d) => M.renameTab(d, paneId, tabId, title))`.
  - `renamePane: (paneId, title) => update((d) => { const p = M.findLeaf(d, paneId); return p ? M.renameTab(d, paneId, M.activeTab(p).id, title) : d; })`.
  - `openThreadAgent`, `splitPane` tetap seperti sekarang (sudah memakai fungsi model; hasil `id` = id pane).
  - `leaves` dan `paneCount` tetap `M.leaves(d)`.

### 12. Verifikasi lokal

Dari `frontend/`: `npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E "layout/model.ts|layout/registry.ts|stores/layout.ts"` harus kosong. Error di berkas lain (pemanggil, komponen) diabaikan dan dicatat di laporan.

## Perubahan

Diubah:
- `frontend/src/modules/layout/model.ts` (operasi tab murni, helper internal, `splitWith`, hapus sisa hydrate/`renamePane`)
- `frontend/src/modules/layout/registry.ts` (`PaneKindSpec` memakai `Tab`)
- `frontend/src/lib/stores/layout.ts` (aksi tab baru, `openPane` mengabaikan `opts.dir`, `renamePane` jadi pembungkus)

Tidak ada berkas dibuat atau dihapus.

## Wiring

Yang diekspos untuk disambungkan nanti oleh fitur lain dan `tab-wiring` (JANGAN disambungkan di sini):

- Store `layout`: `activateTab(paneId, tabId)`, `closeTab(paneId, tabId)`, `renameTab(paneId, tabId, title)` untuk `tab-header-ui` dan `tab-hosts-render` (`TerminalPane` memanggil `renameTab`).
- Store `renamePane(paneId, title)` bertahan sebagai pembungkus agar `LayoutArea.svelte`/`BoardPane.svelte` tidak patah sebelum diadaptasi; `tab-wiring` memeriksa sisa pemakaiannya.
- `openPane(nl, opts?)` store tidak berubah bentuk; pemanggil lama tidak perlu disentuh.
- `PaneKindSpec` baru untuk `tab-hosts-render` (`panes/terminal.ts`, `panes/file.ts`) dan kind lain.
- Model: `openPane`, `activateTab`, `closeTab`, `renameTab`, `openThreadAgent`, `splitPane` dipakai oleh store; `persist/hydrate.ts` memakai `normalize`/`newTabId` yang sudah diekspor foundation.

## Selesai bila

- `openPane` pada doc kosong menghasilkan satu pane satu tab; pada doc berisi menambah tab tepat di kanan tab aktif pane fokus tanpa pane baru; `focusId` tidak valid memakai pane terakhir.
- Kelima aturan dedupe hanya mengaktifkan tab dan memfokus pane (tanpa tab baru).
- `closeTab` pada tab terakhir menghapus pane (via `closePane`); pada tab aktif mengaktifkan tetangga kanan lalu kiri; `activeTabId` selalu valid.
- `activateTab`, `renameTab` (trim, kosong = tak berubah), `openThreadAgent` (id = id pane; tiga cabang) sesuai kontrak.
- `splitPane` dengan `NewLeaf` pada tab aktif terminal membuat pane baru satu tab; pada non-terminal tidak melakukan apa-apa; jalur `Leaf` utuh dipakai `movePane`.
- `PaneKindSpec` menerima `Tab` dan `paneId`; store `layout` mengekspos `activateTab`/`closeTab`/`renameTab`; tidak ada fungsi model `renamePane`.
- Semua fungsi murni (tidak memutasi `doc` masukan).
- Guard: `npx svelte-check` (dari `frontend/`) tidak menampilkan error baru di tiga ownedPaths. Build penuh `npm run check`/`npm run build`/`go build ./...` baru wajib lulus di `tab-wiring`. Tanpa unit test.
