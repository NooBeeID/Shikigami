# Plan: Adaptasi pemanggil ke pencarian tab

## Metadata

- Tiket: `multi-tab-pane-ticket-01-tab-container` (`docs/tickets/multi-tab-pane-ticket-01-tab-container.md`)
- Fitur: `tab-callers`
- Dibaca dahulu: `overview.md` (kontrak bagian 2.2, 2.3, 2.4, 2.8, 6, 7) di folder yang sama. Jangan diubah.
- depends on foundation: ya (`00-foundation.md`). Tipe `Tab`/`Leaf` baru serta `activeTab`, `allTabs`, `findTab`, `findLeaf`, `leaves` sudah diekspor dari `frontend/src/modules/layout/model.ts`. Store layout (`activateTab`, `closeTab`, `renameTab`) disediakan fitur `tab-model-foundation` yang berjalan paralel; ikuti kontrak overview 2.4 persis.
- ownedPaths:
  - `frontend/src/modules/shell/helpers.ts`
  - `frontend/src/modules/tabs/index.ts`
  - `frontend/src/modules/layout/shortcuts.ts`
  - `frontend/src/lib/stores/groups.ts`
  - `frontend/src/modules/groups/sessionDrag.ts`
  - `frontend/src/modules/openfolder/logic.ts`
  - `frontend/src/modules/grimoire/openPage.ts`
  - `frontend/src/modules/thread/thread-store.ts`

## Tujuan

Semua pemanggil di path yang dimiliki berhenti membaca field konten (`kind`, `sessionId`, `filePath`, dst.) langsung dari `Leaf`. Pencarian konten memakai `findTab`/`allTabs`, konten dibaca dari `tab`, dan operasi menutup memakai id pane/tab yang benar. Helper shell mengembalikan `{ paneId, tabId }`. Perilaku pengguna tidak berubah selain: konten baru menjadi tab di pane fokus.

## Path yang dimiliki

HANYA delapan berkas di ownedPaths di atas yang boleh disentuh. Jangan menyentuh `App.svelte`, `layout/index.ts`, `layout/model.ts`, `layout/registry.ts`, `lib/stores/layout.ts`, `persist/`, `layout/panes/`, `PaneHeader`, `PaneMenu`, `LayoutHeaderHost`, maupun pemanggil lain (`FilesTab.svelte`, `palette/sources.ts`, `SessionRow.svelte`, dst.). Bila butuh perubahan di luar itu, ikuti kontrak apa adanya dan catat di laporan. Jangan menambah vitest/unit test. Jangan commit.

## Kontrak

Dipakai (dari overview, jangan didefinisikan ulang):

- `model.ts`: `type Tab`, `type Leaf` (`tabs`, `activeTabId`), `type NewLeaf`, `type LayoutDoc`, `activeTab(leaf): Tab`, `allTabs(doc): { pane: Leaf; tab: Tab }[]`, `findTab(doc, pred: (tab: Tab, pane: Leaf) => boolean): { pane: Leaf; tab: Tab } | null`, `findLeaf(doc, paneId): Leaf | null`, `leaves(node)`, `closePane(doc, paneId)`, `hydrateDoc` (re-ekspor dari `model.ts`, tetap diimpor dari sana).
- Store `layout` (`lib/stores/layout.ts`): `openPane(nl, opts?) : string` (mengembalikan id PANE; `opts.dir` diterima tapi diabaikan), `setFocus(paneId)`, `activateTab(paneId, tabId)`, `closeTab(paneId, tabId)`, `closePane(paneId)`, `splitPane(paneId, dir, nl, 'after')`, `restore()`.

Disediakan (diekspor `shell/helpers.ts`, overview 2.8):

```ts
export interface TabRef { paneId: string; tabId: string }
export function findSessionPane(sessionId: string): TabRef | null;
export function findFilePane(projectId: string, filePath: string): TabRef | null;
export function focusOrOpenSession(sessionId: string): TabRef | null;   // sebelumnya boolean
export function openFilePane(a: { projectId: string; filePath: string; workspaceId?: string; title?: string }): TabRef; // sebelumnya string
```

Catatan kompatibilitas pemanggil di luar path ini (jangan disentuh): semua pemanggil `focusOrOpenSession` hanya memakai nilainya sebagai truthy/falsy (`SessionRow`: `!focusOrOpenSession(...)`), jadi `TabRef | null` kompatibel. Pemanggil `openFilePane` (`FilesTab`, `palette/sources`) mengabaikan nilai kembalian. `sessionTitle` tetap mengembalikan `string`.

## Langkah

Sebelum mulai: jalankan `grep -rnE "findLeafBy|findLeaf\b|leaves\(|renamePane|opts\.dir|\{ dir" frontend/src/modules/shell/helpers.ts frontend/src/modules/tabs/index.ts frontend/src/modules/layout/shortcuts.ts frontend/src/lib/stores/groups.ts frontend/src/modules/groups/sessionDrag.ts frontend/src/modules/openfolder/logic.ts frontend/src/modules/grimoire/openPage.ts frontend/src/modules/thread/thread-store.ts` untuk memastikan daftar titik ubah di bawah masih lengkap.

### 1. `frontend/src/modules/shell/helpers.ts`

1. Impor: ganti `import { findLeafBy, type Leaf } from '../layout/model'` menjadi `import { activeTab, findLeaf, findTab } from '../layout/model';`.
2. Tambah dan ekspor `interface TabRef { paneId: string; tabId: string }`.
3. `findSessionPane(sessionId)`: `const r = findTab(get(layout), (t) => t.kind === 'terminal' && t.sessionId === sessionId); return r ? { paneId: r.pane.id, tabId: r.tab.id } : null;`
4. `findFilePane(projectId, filePath)`: `findTab(get(layout), (t) => t.kind === 'file' && !!t.filePath && t.filePath === filePath && t.projectId === projectId)`, petakan ke `TabRef` seperti di atas.
5. `sessionTitle`: ganti `pane.title` dengan judul tab. Karena `findSessionPane` kini hanya mengembalikan id, cari tab langsung: `const r = findTab(get(layout), (t) => t.kind === 'terminal' && t.sessionId === sessionId); if (r) return r.tab.title;`. Sisa fungsi tidak berubah.
6. Tambah helper privat `refOfPane(paneId: string): TabRef | null`: `const p = findLeaf(get(layout), paneId); return p ? { paneId: p.id, tabId: activeTab(p).id } : null;`. Dipakai setelah `layout.openPane` (dedupe maupun tab baru sama-sama menjadikan tab target sebagai tab aktif pane yang dikembalikan, sesuai overview 2.3).
7. `focusOrOpenSession(sessionId): TabRef | null`:
   - sesi tidak ada di store `sessions`: `return null`.
   - `const ref = findSessionPane(sessionId); if (ref) { layout.activateTab(ref.paneId, ref.tabId); return ref; }` (store `activateTab` sudah memanggil `setFocus(paneId)`).
   - selain itu: `const paneId = layout.openPane({ kind: 'terminal', sessionId, projectId: s.projectId, provider: s.providerId, title: sessionTitle(sessionId) }); return refOfPane(paneId);`
8. `openFilePane(a): TabRef`:
   - `const ref = findFilePane(a.projectId, a.filePath); if (ref) { layout.activateTab(ref.paneId, ref.tabId); return ref; }`
   - selain itu: `const paneId = layout.openPane({...isi yang sama seperti sekarang...}); return refOfPane(paneId) ?? { paneId, tabId: '' };` (fallback hanya untuk memuaskan tipe; secara praktik `refOfPane` selalu ada).
9. `createAndOpenSession`, `openFolder`, `zoom`, `tidyGrid`, `basename` tidak berubah.

### 2. `frontend/src/modules/tabs/index.ts` (jembatan v1)

Pemakai (sudah dicek): berkas ini hanya dimuat lewat efek samping impor (subscribe di level modul). Pastikan lagi dengan `grep -rnE "modules/tabs|from '\.\./tabs|from '\./tabs" frontend/src` dan cari impor `./modules/tabs` di `App.svelte`/`main.ts`; bila tidak ada pemakai sama sekali, cukup pastikan berkas kompilasi dengan tipe baru (langkah di bawah tetap dikerjakan seminimal mungkin agar kompilasi). Jangan mengubah `App.svelte` bila impor tak ditemukan; catat di laporan.

Penamaan: berkas mengimpor `Tab` dan `closeTab` dari store v1. Hindari bentrok dengan tipe layout: impor tipe layout dengan alias, `import type { Tab as LayoutTab, NewLeaf, LayoutDoc } from '../layout/model'` dan gunakan `layout.closeTab(paneId, tabId)` (metode store), bukan fungsi `closeTab` v1 (itu tetap `closeTab` impor dari `lib/stores`).

Ubah dengan perilaku berikut:

1. Ganti `import { leaves }` dengan `import { allTabs, findTab, activeTab, findLeaf } from '../layout/model';`. Hapus tipe `Leaf` dari impor jika tidak terpakai.
2. `tabIdOfLeaf(l: Leaf)` diganti `tabIdOfLayoutTab(t: LayoutTab): string | null` dengan isi sama tetapi membaca dari `t` (`t.kind === 'terminal' && t.sessionId` -> `session:${...}`; `t.kind === 'file' && t.filePath !== undefined` -> `file:${JSON.stringify({ path, projectId, workspaceId })}`).
3. `leafOfTab(tab, doc)` diganti `layoutTabOf(tab: Tab, doc: LayoutDoc): { pane: Leaf; tab: LayoutTab } | null` memakai `findTab(doc, (t) => tab.type === 'session' ? t.kind === 'terminal' && t.sessionId === tab.refId : t.kind === 'file' && tabIdOfLayoutTab(t) === tab.id)`. (Impor `type Leaf` bila dipakai pada tipe kembalian.)
4. `newLeafOfTab` tidak berubah (menghasilkan `NewLeaf`).
5. `ensurePane(tab)`: bila `layoutTabOf` ketemu: `layout.activateTab(r.pane.id, r.tab.id)` (sudah memfokus pane); selain itu `layout.openPane(nl)`.
6. `focusedTabId(doc)`: tab aktif pane fokus: `const f = doc.focusId ? findLeaf(doc, doc.focusId) : null; return f ? tabIdOfLayoutTab(activeTab(f)) : null;`
7. Subscriber `tabs` (tab -> pane): iterasi `allTabs(doc)` (bukan `leaves`); untuk tab layout yang `known` dan hilang dari store v1, panggil `layout.closeTab(pane.id, tab.id)` (BUKAN `closePane`), lalu `known.delete(tid)`. Karena `closeTab` pada tab terakhir sudah memanggil `closePane`, tidak perlu cabang khusus. Hitung snapshot daftar yang akan ditutup lebih dulu (kumpulkan `{paneId, tabId}` dari satu pemanggilan `allTabs(get(layout))`) lalu tutup satu per satu, karena doc berubah antar pemanggilan.
   Bagian "tab baru tanpa pane": `layoutTabOf(t, get(layout))` menggantikan `leafOfTab`; `const id = layout.openPane(nl); if (t.id === active) layout.setFocus(id);` tetap.
8. Subscriber `activeTabId`: tidak berubah (memanggil `ensurePane`).
9. Subscriber `layout` (pane -> tab): `present` diisi dari `allTabs(doc)` + `tabIdOfLayoutTab(tab)` (semua tab, termasuk non-aktif; tab non-aktif tetap dianggap ada). Logika `closed`, `tid = focusedTabId(doc)`, `known` tidak berubah.
10. Sentuhan akhir: pastikan tidak ada lagi `leaves(` atau akses `.kind/.sessionId/.filePath` pada `Leaf`.

### 3. `frontend/src/modules/layout/shortcuts.ts`

1. Impor: `import { activeTab, findLeaf, type NewLeaf } from './model';`
2. `splitPaneWithNewSession`: `const pane = findLeaf(get(store), paneId); if (!pane) return; const src = activeTab(pane); if (src.kind !== 'terminal') return;` (Cmd+D hanya untuk tab aktif terminal; non-terminal = tidak melakukan apa-apa). Sisa memakai `src.provider`, `src.sessionId`, `src.projectId` (sekarang dari tab aktif) tanpa perubahan.
3. Cabang akhir: `if (findLeaf(get(store), paneId)) store.splitPane(paneId, dir, nl, 'after'); else store.openPane(nl);` (tanpa `{ dir }`).
4. `splitFocused` dan `installLayoutShortcuts` tidak berubah.

### 4. `frontend/src/lib/stores/groups.ts`

Fungsi `moveSession` menutup SEMUA pane/tab terminal dengan sesi tertentu. Perbaikan:

1. Impor: `import { allTabs, closePane as closePaneDoc, findTab, hydrateDoc } from '../../modules/layout/model';` (hapus `findLeafBy`, `leaves` bila tak terpakai lagi; `type LayoutDoc` bila perlu).
2. `isTerm` menjadi predikat tab: `const isTerm = (t: { kind: string; sessionId?: string }) => t.kind === 'terminal' && t.sessionId === sessionId;`
3. Grup aktif: ganti loop `findLeafBy` dengan `for (let r = findTab(get(layout), isTerm); r; r = findTab(get(layout), isTerm)) { layout.closeTab(r.pane.id, r.tab.id); }`. (Tab terakhir pane otomatis menutup pane; tab non-terminal lain di pane yang sama dipertahankan.)
4. Grup lain (doc hasil `hydrateDoc`): operasi murni di doc. Karena `closeTab` model tidak diekspor dalam kontrak untuk dipakai di sini kecuali dari `model.ts`, impor `closeTab as closeTabDoc` dari `../../modules/layout/model` (kontrak 2.3 menjamin ada) dan lakukan `for (let r = findTab(doc, isTerm); r; r = findTab(doc, isTerm)) { doc = closeTabDoc(doc, r.pane.id, r.tab.id); closed = true; }`. `closePaneDoc` boleh dihapus dari impor bila tak lagi dipakai.
5. Cek akhir `moveSession`: `!findTab(get(layout), isTerm)` menggantikan `!findLeafBy(...)`. Sisanya (openPane terminal baru) tidak berubah.
6. Berkas ini juga mengimpor `hydrateDoc` dari `model.ts`; tetap valid lewat re-ekspor foundation. Jangan mengubah bagian lain berkas.

### 5. `frontend/src/modules/groups/sessionDrag.ts`

Di `watchPaneDrag`, drag header pane utuh (`st.paneId`): sesi dari tab aktif pane itu.

1. Impor: `import { activeTab, findLeaf } from '../layout/model';`
2. Ganti blok `const leaf = findLeaf(...)`: `const pane = findLeaf(get(layout), st.paneId); const tab = pane ? activeTab(pane) : null; if (tab?.kind !== 'terminal' || !tab.sessionId) return; const sid = tab.sessionId;`
3. Sisa logika (hover tracker, event window) tidak berubah. Jangan memperluas ke drag per tab (itu tiket 02). Bila ada akses lain ke field konten leaf di berkas, ubah sama.

### 6. `frontend/src/modules/openfolder/logic.ts`

1. Impor: `import { activeTab, findLeaf } from '../layout/model';`
2. `terminalSessionAt(x, y)`: setelah `paneId` ditemukan: `const pane = findLeaf(get(layout), paneId); if (!pane) return null; const tab = activeTab(pane); return tab.kind === 'terminal' && tab.sessionId ? tab.sessionId : null;` (hanya tab aktif yang terlihat di titik jatuh).

### 7. `frontend/src/modules/grimoire/openPage.ts`

1. Impor: `import { findTab } from '../layout/model';` (ganti `findLeafBy`).
2. `const existing = findTab(get(layout), (t) => t.kind === 'file' && t.filePath === filePath && t.projectId === projectId); if (existing) { layout.activateTab(existing.pane.id, existing.tab.id); return existing.pane.id; }`
3. Tipe kembalian `Promise<string>` (id pane) dan `layout.openPane({...})` tidak berubah.

### 8. `frontend/src/modules/thread/thread-store.ts`

Sudah dicek: berkas hanya memakai `layout.openPane({ kind: 'thread', ... })` (kontrak `openPane` tidak berubah), tidak ada `findLeaf`/`findLeafBy`/`leaves`/`openThreadAgent`. Jalankan grep (lihat Sebelum mulai) untuk memastikan. Bila memang tidak ada, JANGAN ubah berkas ini. Bila grep menemukan pemakaian `openThreadAgent` (mengembalikan id pane) atau akses field konten leaf, sesuaikan memakai `findTab`/id pane dan catat di laporan.

## Perubahan

Diubah (tidak ada berkas dibuat/dihapus):

- `frontend/src/modules/shell/helpers.ts` (tipe `TabRef`, empat fungsi, `sessionTitle`)
- `frontend/src/modules/tabs/index.ts` (jembatan v1 ke `allTabs`/`findTab`/`layout.closeTab`)
- `frontend/src/modules/layout/shortcuts.ts` (tab aktif terminal, `openPane(nl)` tanpa dir)
- `frontend/src/lib/stores/groups.ts` (`moveSession` memakai `findTab`/`closeTab`)
- `frontend/src/modules/groups/sessionDrag.ts` (sesi dari tab aktif pane)
- `frontend/src/modules/openfolder/logic.ts` (`terminalSessionAt` dari tab aktif)
- `frontend/src/modules/grimoire/openPage.ts` (`findTab` + `activateTab`)
- `frontend/src/modules/thread/thread-store.ts` (kemungkinan tanpa perubahan; hanya verifikasi)

## Wiring

Fitur ini hanya mengekspos untuk langkah wiring:

- `shell/helpers.ts`: `TabRef`, `findSessionPane`, `findFilePane`, `focusOrOpenSession`, `openFilePane` dengan tipe kembalian baru. Pemanggil eksternal sudah kompatibel (lihat Kontrak); wiring cukup memverifikasi `npm run check` tidak mengeluh pada pemanggil tersebut.
- `shell/index` (re-ekspor dari `../shell`) bila mengekspor `*` dari helpers: `TabRef` ikut terekspor otomatis; jangan edit berkas itu di sini.
- `modules/tabs/index.ts` tetap dimuat lewat efek samping impor seperti sekarang; tidak ada sambungan baru.

JANGAN menyambungkan apa pun di `App.svelte` atau `layout/index.ts`.

## Selesai bila

- Tidak ada akses field konten (`kind`, `sessionId`, `filePath`, `projectId`, `title`, dst.) langsung pada `Leaf`, dan tidak ada pemakaian `findLeafBy`, `leaves(` (kecuali telah digantikan), `renamePane`, atau `openPane(..., { dir })` di delapan berkas ownedPaths. Cek: `grep -nE "findLeafBy|leaves\(|renamePane|\{ dir" <delapan berkas>` kosong.
- `focusOrOpenSession` mengembalikan `TabRef | null`, memfokus tab yang ada atau membuka tab baru; `openFilePane` mengembalikan `TabRef`.
- Menutup tab v1 lewat jembatan memanggil `layout.closeTab(paneId, tabId)` pada tab layout yang cocok, bukan `closePane`; `focusedTabId` = tab aktif pane fokus.
- Cmd+D/`splitFocused` hanya bekerja bila tab aktif adalah terminal.
- Guard: `cd frontend && npm run check 2>&1 | grep -E "shell/helpers.ts|modules/tabs/index.ts|layout/shortcuts.ts|stores/groups.ts|groups/sessionDrag.ts|openfolder/logic.ts|grimoire/openPage.ts|thread/thread-store.ts"` tidak menampilkan error baru (error di berkas milik fitur lain diabaikan selama fase paralel; build penuh diverifikasi `tab-wiring`). Tanpa unit test.
