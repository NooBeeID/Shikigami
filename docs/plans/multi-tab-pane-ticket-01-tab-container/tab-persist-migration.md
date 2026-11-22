# Plan: Persistensi layout dua bentuk leaf

## Metadata

- Tiket: `multi-tab-pane-ticket-01-tab-container` (`docs/tickets/multi-tab-pane-ticket-01-tab-container.md`)
- Fitur: `tab-persist-migration`
- ownedPaths: `frontend/src/modules/persist/`, `frontend/src/lib/stores/persist.ts`
- Depends on foundation: ya (`00-foundation.md`). Foundation sudah membuat `Tab`/`Leaf` baru di `layout/model.ts`, mengekspor `newPaneId`, `newTabId`, `normalize`, `emptyDoc`, `clampRatio`, `activeTab`, `allTabs`, `findTab`, dan membuat versi awal `persist/hydrate.ts` (dengan ekspor ulang dari `model.ts`). Rujukan kontrak: `overview.md` bagian 2 (terutama 2.1, 2.2, 2.6). Jangan mengulang isinya di sini.
- Catatan: bila `frontend/src/modules/persist/hydrate.ts` ternyata belum ada saat mulai (foundation belum selesai), buat file itu sendiri dari kode `hydrateNode`/`hydrateDoc` di `model.ts` (jangan edit `model.ts`) lalu tulis seperti Langkah 1. Impor dari `../layout/model` mengikuti kontrak overview 2.1/2.2.

## Tujuan

1. `hydrateDoc(raw)` menerima dua bentuk leaf (lama: field konten langsung + `kind`; baru: `tabs` + `activeTabId`), menghasilkan `LayoutDoc` valid dengan invarian: tiap pane punya `tabs` tidak kosong, `activeTabId` menunjuk tab di pane itu, id pane unik, id tab unik, `focusId`/`maximizedId` menunjuk pane yang ada.
2. Autosave tetap menulis doc apa adanya (bentuk baru, `version: 1`); tidak ada perubahan format di sisi tulis.
3. `DeadSession.svelte`, `TerminalGuard.svelte`, dan `lib/stores/persist.ts` (bila perlu) kompilasi dan benar dengan tab di dalam pane: sesi dibaca dari `tab.sessionId`, penggantian sesi menarget tab, bukan pane.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `frontend/src/modules/persist/` (`hydrate.ts`, `DeadSession.svelte`, `TerminalGuard.svelte`, `restore.ts`, `autosave.ts`, `index.ts`, `register.ts`, `windowState.ts`)
- `frontend/src/lib/stores/persist.ts`

Jangan sentuh `layout/model.ts`, `layout/registry.ts`, `lib/stores/layout.ts`, `lib/stores/groups.ts`, `layout/panes/`, `App.svelte`, `layout/index.ts`, atau path fitur lain. Bila butuh perubahan di sana, ikuti kontrak apa adanya dan catat di laporan.

## Kontrak

Disediakan (overview 2.6):

- `export function hydrateDoc(raw: unknown): LayoutDoc` di `frontend/src/modules/persist/hydrate.ts`. `model.ts` mengekspor ulangnya (sudah dibuat foundation); `lib/stores/layout.ts` dan `lib/stores/groups.ts` memanggil `hydrateDoc` lewat itu. Jangan ubah signature. Tidak pernah melempar: input rusak menghasilkan `emptyDoc()` atau doc sebagian.

Dipakai (dari `../layout/model`, overview 2.1/2.2):

- Tipe: `Tab`, `Leaf`, `Node`, `Split`, `LayoutDoc`, `PaneKind`.
- Fungsi: `emptyDoc()`, `clampRatio(r)`, `normalize(doc)`, `newPaneId()`, `newTabId()`, `allTabs(doc)`, `findTab(doc, pred)`, `findLeaf(doc, paneId)`, `leaves(n)`.
- Store `layout` (`lib/stores/layout.ts`): `layout.serialize(): LayoutDoc`, `layout.hydrate(raw: unknown): void`.
- Registry: `getPaneKind`, `registerPaneKind`, tipe `PaneKindSpec` dengan `component: Component<{ pane: Tab; paneId: string; visible: boolean }>` (overview 2.5).
- `TerminalPane` (milik `tab-hosts-render`) menerima props `{ pane: Tab; paneId: string; visible: boolean }`.

Tidak ada pesan IPC baru. Format simpan `workspace.layout.set` tetap `{ workspaceId, groupId, layout }` dengan `layout` = hasil `layout.serialize()` (bentuk baru).

## Langkah

### 1. `frontend/src/modules/persist/hydrate.ts` (tulis ulang penuh)

Impor:

```ts
import {
  clampRatio, emptyDoc, newPaneId, newTabId, normalize,
  type LayoutDoc, type Leaf, type Node, type PaneKind, type Tab,
} from '../layout/model';
```

Impor dari `model.ts` hanya dipakai di dalam fungsi (bukan top-level), sehingga siklus dengan ekspor ulang `model.ts` aman.

Helper privat:

```ts
const isStr = (v: unknown): v is string => typeof v === 'string';
const isObj = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);

// Field konten string yang disalin dari bentuk lama maupun baru.
const STR_KEYS = ['sessionId', 'projectId', 'provider', 'filePath', 'workspaceId', 'root',
  'threadId', 'sourceId', 'tableId', 'viewId', 'nodeToken', 'pipelineId'] as const;

// Bentuk lama: objek leaf dengan field konten langsung. Bentuk baru: objek tab.
// Keduanya punya field konten yang sama; beda hanya `id` (tab) dan wrapper pane.
function hydrateTab(raw: unknown, usedTabIds: Set<string>, fallbackId?: string): Tab | null
```

`hydrateTab(raw, usedTabIds)`:
- `raw` bukan objek atau `kind` bukan string non-kosong: `return null` (tab tanpa kind dibuang).
- `id`: bila `raw.id` string non-kosong, berprefix `t_`, dan belum ada di `usedTabIds`, pakai itu; selain itu `newTabId()`. Pastikan hasil akhir tidak ada di `usedTabIds` (ulangi `newTabId()` sampai unik), lalu `usedTabIds.add(id)`. Untuk bentuk lama, panggil dengan `raw` yang `id`-nya adalah id pane (`p_...`): karena tidak berprefix `t_`, otomatis dibuatkan `newTabId()` (id tab baru, sesuai kontrak). Implementasi: untuk bentuk lama berikan salinan `{ ...raw, id: undefined }` supaya jelas.
- `title`: `isStr(raw.title) && raw.title ? raw.title : 'Pane'`.
- Salin `STR_KEYS` bila `isStr`; `readOnly`/`threadAgent` bila boolean; `number` bila `typeof === 'number' && Number.isFinite`.
- `kind` disalin sebagai `PaneKind` (string terbuka; jangan validasi terhadap daftar kind, kind plugin bisa belum terdaftar saat hydrate).
- Kembalikan `Tab`.

`hydrateLeaf(raw: Record<string, unknown>, usedPaneIds: Set<string>, usedTabIds: Set<string>): Leaf | null`:
- Id pane: `isStr(raw.id) && raw.id && !usedPaneIds.has(raw.id)` pakai itu; selain itu `newPaneId()` (id bentrok atau kosong dibuatkan baru, bukan pane dibuang; ini beda dari perilaku lama yang membuang duplikat). Tambah ke `usedPaneIds`.
- Deteksi bentuk: `Array.isArray(raw.tabs)` = bentuk baru; selain itu bentuk lama.
- Bentuk lama: `const t = hydrateTab({ ...raw, id: undefined }, usedTabIds)`. Bila `null` kembalikan `null` (dan JANGAN menambah id pane ke `usedPaneIds`; pindahkan `usedPaneIds.add` ke setelah leaf pasti valid). Hasil: `{ type: 'pane', id, tabs: [t], activeTabId: t.id }`.
- Bentuk baru: iterasi `raw.tabs`, `hydrateTab(item, usedTabIds)`, kumpulkan yang non-null. Bila hasil kosong kembalikan `null`. `activeTabId`: bila `isStr(raw.activeTabId)` dan ada di `tabs` hasil hydrate, pakai itu. Catatan dedupe: bila id tab asli di-regenerate karena bentrok, `activeTabId` lama yang menunjuk id itu menunjuk tab pertama yang memegang id tersebut (karena yang pertama mempertahankan id); aman. Selain itu `tabs[0].id`.
- Jangan salin field konten ke level pane.

`hydrateNode(raw, usedPaneIds, usedTabIds): Node | null`:
- `raw.type === 'pane'`: `hydrateLeaf`.
- `raw.type === 'split'`: `dir` harus `'h'|'v'` dan `ratio` number (sama seperti kode lama; selain itu `null`). Hydrate `a` lalu `b` (urut, agar pembagian id deterministik). Bila salah satu `null` kembalikan yang lain (split runtuh); bila keduanya `null` kembalikan `null`. Selain itu `{ type: 'split', dir, ratio: clampRatio(ratio), a, b }`.
- Selain itu `null`.
- Hindari rekursi tak terbatas dari JSON siklik: input berasal dari JSON, jadi tidak perlu; cukup batasi kedalaman 64 dengan parameter `depth` (lewat batas = `null`) untuk keamanan.

`export function hydrateDoc(raw: unknown): LayoutDoc`:

```ts
export function hydrateDoc(raw: unknown): LayoutDoc {
  try {
    if (!isObj(raw)) return emptyDoc();
    if (raw.version !== 1) return emptyDoc();
    const root = hydrateNode(raw.root, new Set(), new Set(), 0);
    const doc: LayoutDoc = { version: 1, root };
    if (isStr(raw.focusId)) doc.focusId = raw.focusId;
    if (isStr(raw.maximizedId)) doc.maximizedId = raw.maximizedId;
    else if (raw.maximizedId === null) doc.maximizedId = null;
    return JSON.parse(JSON.stringify(normalize(doc)));
  } catch {
    return emptyDoc();
  }
}
```

`normalize` (milik model) sudah mereset `focusId`/`maximizedId` yang tidak menunjuk id pane valid dan meruntuhkan split berisi anak null. Karena foundation/`tab-model-foundation` bisa mengubah `normalize`, jangan bergantung penuh padanya: setelah `normalize`, tambahkan pengecekan lokal yang idempoten:

```ts
const out = normalize(doc);
const ids = new Set(collectPaneIds(out.root));  // helper privat lokal; jangan memakai leaves() bila ragu
if (out.focusId && !ids.has(out.focusId)) delete out.focusId;
if (out.maximizedId && !ids.has(out.maximizedId)) out.maximizedId = null;
```

(`maximizedId` tidak valid direset ke `null`/dihapus; pakai `delete out.maximizedId` agar sama dengan perilaku `normalize` yang hanya menyimpan `maximizedId` valid atau `null` eksplisit. Pilih `delete` untuk kedua kasus tidak valid, kecuali nilai asli memang `null`.) Helper `collectPaneIds(n: Node | null): string[]` ditulis lokal (rekursif sederhana) di `hydrate.ts`.

Aturan perilaku yang harus terpenuhi (periksa manual dengan membaca kode):
- Layout lama `{version:1, root:{type:'pane', id:'p_1', kind:'terminal', sessionId:'s1', title:'x'}}` menjadi pane `p_1` dengan satu tab (`id` `t_...`, `kind` `terminal`, `sessionId` `s1`), `activeTabId` = id tab itu.
- Bentuk baru round-trip: hydrate lalu serialize menghasilkan isi sama (id pane/tab yang valid dan unik dipertahankan).
- `tabs` berisi item tanpa `kind` dibuang; sisanya dipertahankan; `activeTabId` salah menjadi tab pertama.
- Pane tanpa tab valid dibuang dan split induknya runtuh ke anak yang tersisa; bila semua pane habis hasil `root: null`.
- Id pane ganda: pane kedua mendapat `newPaneId()`; id tab ganda (lintas pane juga): yang kedua mendapat `newTabId()`.
- Semua konten valid tidak pernah hilang karena id bentrok.

### 2. `frontend/src/modules/persist/TerminalGuard.svelte`

Adaptasi ke props baru:

```svelte
<script lang="ts">
  import type { Tab } from '../layout/model';
  import TerminalPane from '../layout/panes/TerminalPane.svelte';
  import DeadSession from './DeadSession.svelte';
  import { persistStatus } from '../../lib/stores/persist';
  import { sessions } from '../../lib/stores/sessions';

  let { pane, paneId, visible }: { pane: Tab; paneId: string; visible: boolean } = $props();
  // dead: sama seperti sekarang, memakai pane.sessionId (sekarang field Tab)
</script>

{#if dead}
  <div class="guard"><DeadSession tab={pane} {paneId} /></div>
{:else}
  <TerminalPane {pane} {paneId} {visible} />
{/if}
```

Logika `dead` tidak berubah (`pane.sessionId`). Tab terminal non-aktif tetap di-mount host dengan `visible=false`; guard tetap merender `DeadSession` di dalam host tab itu, jadi tidak ada perubahan lain. `.guard` CSS tetap.

### 3. `frontend/src/modules/persist/DeadSession.svelte`

Ganti props dari `leaf: Leaf` menjadi `{ tab: Tab; paneId: string; onRestarted? }`:

- `import type { Tab, Node } from '../layout/model'` (ganti `Leaf`).
- Ganti semua `leaf.projectId`, `leaf.provider`, `leaf.kind` menjadi `tab.projectId`, `tab.provider`, `tab.kind`. Blok `{#if tab.kind === 'terminal'}` tetap.
- Ganti `replaceSessionId` agar menarget TAB di dalam pane:

```ts
function replaceSessionId(node: Node | null, paneId: string, tabId: string, newId: string, found: { v: boolean }): Node | null {
  if (!node) return null;
  if (node.type === 'pane') {
    if (node.id !== paneId) return node;
    return {
      ...node,
      tabs: node.tabs.map((t) => {
        if (t.id !== tabId) return t;
        found.v = true;
        return { ...t, sessionId: newId };
      }),
    };
  }
  return { ...node, a: replaceSessionId(node.a, paneId, tabId, newId, found)!, b: replaceSessionId(node.b, paneId, tabId, newId, found)! };
}
```

- Di `restart()`: `doc.root = replaceSessionId(doc.root, paneId, tab.id, s.id, found)`; sisanya sama (`layout.serialize()` lalu `layout.hydrate(doc)` bila `found.v`). Jangan mengubah `activeTabId`, id pane, atau id tab, sehingga host (di-key id tab) tidak di-remount dan `TerminalGuard` otomatis beralih dari `DeadSession` ke `TerminalPane` karena `pane.sessionId` baru ada di `sessions`.
- Pemisahan alasan memakai `paneId` dan `tab.id` (bukan pencarian berdasar `sessionId`), karena dua tab bisa tidak punya `sessionId`.
- Tidak perlu `allTabs`/`findTab` di sini karena target sudah diketahui dari props. Hanya bila tab tidak ditemukan (`found.v` false, mis. tab ditutup saat mulai ulang) layout tidak diubah, perilaku sama seperti sekarang.

### 4. `frontend/src/modules/persist/restore.ts`

- `sanitizeDoc` tetap tipis (kloning JSON); tidak membuang tab sesi mati (sesi mati ditangani `DeadSession`). Tidak ada akses field konten leaf di sini; cukup perbarui komentar: "Tidak ada tab yang dibuang". Biarkan `hydrateDoc` dilakukan oleh `layout.hydrate(doc)` seperti sekarang.
- Tidak ada perubahan perilaku lain. `liveIds` tetap dihitung (dipertahankan untuk kompatibilitas parameter).

### 5. `frontend/src/modules/persist/autosave.ts`, `index.ts`, `register.ts`, `windowState.ts`

- Tidak perlu diubah: autosave memakai `JSON.stringify(layout.serialize())` yang sekarang otomatis bentuk baru; `version` tetap 1.
- Baca ulang `autosave.ts` dan pastikan tidak ada akses langsung ke field konten leaf (`grep -n "sessionId\|\.kind\|filePath" frontend/src/modules/persist/*.ts`). Bila ada, ganti dengan `allTabs`/`findTab` dari `../layout/model`. (Pada pembacaan awal tidak ada.)
- `register.ts`: `registerPaneKind({ ...orig, component: TerminalGuard })` tetap; tipe `component` baru sudah cocok dengan props `TerminalGuard` (`pane: Tab; paneId: string; visible: boolean`).

### 6. `frontend/src/lib/stores/persist.ts`

Tidak memegang leaf/layout; tidak perlu diubah. Verifikasi dengan membaca ulang bahwa tidak ada pemakaian `Leaf`. Jangan ubah ekspor (`persistStatus`, `lastSavedAt`, `setPersistStatus`, `markSaved`, `setFlushHandler`, `flushNow`).

### 7. Verifikasi

Dari `frontend/`:

```
npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E "modules/persist/|stores/persist.ts"
```

Harus kosong (error tipe di file milik fitur lain diabaikan; error yang berasal dari file di luar ownedPaths tidak diperbaiki). Bila error di `persist/` berasal dari `TerminalPane` yang belum menerima `paneId`/`Tab` (milik `tab-hosts-render`), catat di laporan dan jangan edit file itu.

## Perubahan

- Ubah/tulis ulang: `frontend/src/modules/persist/hydrate.ts` (dibuat foundation; di sini disempurnakan).
- Ubah: `frontend/src/modules/persist/TerminalGuard.svelte` (props `pane: Tab`, `paneId`; teruskan ke `DeadSession`/`TerminalPane`).
- Ubah: `frontend/src/modules/persist/DeadSession.svelte` (props `tab`, `paneId`; `replaceSessionId` menarget tab).
- Ubah (komentar saja bila perlu): `frontend/src/modules/persist/restore.ts`.
- Tidak berubah: `autosave.ts`, `index.ts`, `register.ts`, `windowState.ts`, `lib/stores/persist.ts` (hanya diverifikasi).

## Wiring

Yang diekspos untuk langkah `tab-wiring` (jangan disambungkan di sini):

- `hydrateDoc` dari `persist/hydrate.ts` (sudah dikonsumsi lewat ekspor ulang `model.ts` oleh `lib/stores/layout.ts` dan `lib/stores/groups.ts`; tidak ada yang perlu diubah di sana).
- `TerminalGuard` terdaftar di `persist/register.ts` sebagai pembungkus kind `terminal` (tidak berubah); `layout/index.ts`/`App.svelte` tidak perlu mengubah impor persist.
- `DeadSession` sekarang memakai props `{ tab, paneId, onRestarted? }` (bukan `leaf`). Bila ada pemakai lain di luar `persist/` (cek `grep -rn "DeadSession" frontend/src`), catat di laporan agar `tab-wiring` menyesuaikan; jangan edit di sini.

## Selesai bila

- `hydrateDoc` memenuhi semua aturan di Langkah 1: bentuk lama menjadi pane satu tab (id pane sama, id tab baru berprefix `t_`), bentuk baru round-trip, tab tanpa kind dibuang, `activeTabId` salah diperbaiki, pane tanpa tab runtuh, id pane dan tab ganda dibuatkan baru, `focusId`/`maximizedId` tidak valid direset; tidak pernah melempar.
- `version` tetap `1`; tidak ada perubahan pada sisi tulis autosave.
- `TerminalGuard` dan `DeadSession` memakai `Tab` + `paneId`; mulai ulang sesi mengganti `sessionId` pada tab yang benar tanpa mengubah id pane/tab.
- `npx svelte-check` terfilter ke `modules/persist/` dan `lib/stores/persist.ts` tidak menampilkan error baru dari kode di ownedPaths (error yang bersumber dari file fitur lain dicatat di laporan).
- Tidak ada file di luar ownedPaths yang diubah; tidak ada unit test ditambahkan.
