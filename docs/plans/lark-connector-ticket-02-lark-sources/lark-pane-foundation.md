# Plan: Fondasi pane lark-* di layout

## Metadata

- Tiket: `lark-connector-ticket-02-lark-sources` (`docs/tickets/lark-connector-ticket-02-lark-sources.md`)
- Fitur: `lark-pane-foundation`
- Kontrak bersama: `overview.md` (folder ini), terutama bagian 2.6 dan 2.8. Jangan mengubah `overview.md`.
- Depends on foundation: **ya** (`00-foundation.md` F4 menambah tipe `PaneKind` `'lark-board' | 'lark-doc'` dan field opsional `sourceId`, `tableId`, `viewId`, `nodeToken` pada `Leaf`/`NewLeaf` di `layout/model.ts`). Bila saat implementasi tipe itu belum ada, tambahkan sendiri di `model.ts` (file ini milik fitur ini) dengan bentuk persis seperti di atas.
- ownedPaths:
  - `frontend/src/modules/lark/index.ts` (hanya registrasi pane dan ekspor)
  - `frontend/src/modules/lark/source.ts` (baru)
  - `frontend/src/modules/lark/actions.ts` (baru)
  - `frontend/src/modules/lark/board/BoardPane.svelte` (baru)
  - `frontend/src/modules/lark/doc/DocPane.svelte` (baru)
  - `frontend/src/modules/layout/model.ts`
  - `frontend/src/lib/stores/layout.ts`
  - `frontend/src/modules/layout/` (renderer pane tak dikenal; seminimal mungkin)

## Tujuan

Meletakkan fondasi pane `lark-board` dan `lark-doc` di sistem layout: registrasi jenis pane dengan komponen placeholder "Segera hadir", pemeriksaan sumber ("Sumber tidak ada di workspace ini"), persist field referensi Lark lewat loader layout, dedupe `openPane`, keadaan "Pane tidak dikenal" yang tidak membuang pane, dan helper `copyLink`/`sendToAgent`. Isi pane sungguhan ditimpa tiket 03 dan 04. Perilaku pane yang ada (terminal, file, thread) tidak boleh berubah.

## Path yang dimiliki

HANYA path di Metadata yang boleh disentuh. Jangan sentuh: `LarkPanel.svelte`, `LarkIcon.svelte`, `client.ts`, `store.ts`, `AddSourceDialog.svelte`, `sections/` (milik `lark-panel-ui`), penambahan `registerRailItem` di `index.ts` (milik `lark-sources-wiring`), file Go apa pun, `cmd/shikigamictl/*`. `frontend/package.json` tidak diubah.

Catatan pohon kerja: `model.ts`, `lib/stores/layout.ts`, `LayoutArea.svelte`, `PaneHeader.svelte` saat ini modified (uncommitted, perubahan `threadAgent`/`openThreadAgent`/`onMinimize` milik orang lain). Edit hanya hunk sendiri; jangan mengubah, memformat ulang, atau men-stage hunk tak terkait. Bila bekerja di worktree dari commit yang belum memuat perubahan itu, laporkan sebagai risiko merge, jangan diperbaiki.

## Kontrak

Disediakan (merujuk `overview.md` 2.6):
- Jenis pane `lark-board` (props `{ sourceId, tableId?, viewId? }`) dan `lark-doc` (props `{ sourceId | 'personal', nodeToken }`), tersimpan di `Leaf`.
- Kunci dedupe `openPane`: `lark-board` = `kind + sourceId`; `lark-doc` = `kind + sourceId + nodeToken`.
- `frontend/src/modules/lark/source.ts`: `export type SourceStatus = 'ok' | 'missing' | 'error'` dan `export async function resolveSource(sourceId: string): Promise<SourceStatus>`.
- `index.ts` mengekspor ulang: `export { resolveSource } from './source'` (dan tipe `SourceStatus`).
- `frontend/src/modules/lark/actions.ts`: `export async function copyLink(url: string): Promise<void>` dan `export async function sendToAgent(url: string): Promise<void>`.
- `BoardPane.svelte` dan `DocPane.svelte`: props `{ pane: Leaf; visible: boolean }` (sesuai `PaneKindSpec.component`). Signature ini tidak boleh berubah; tiket 03/04 menimpa isinya dan mengimpor `resolveSource` dari `../source`.

Dipakai (sudah ada di kode):
- `registerPaneKind`, `PaneKindSpec` dari `frontend/src/modules/layout/registry.ts` (`{ kind, component, titleFor? }`).
- `call` dari `frontend/src/lib/ipc.ts`; `activeWorkspaceId` dari `frontend/src/lib/stores/workspaces.ts`.
- IPC `lark.sources.get {workspaceId}` -> `{tickets: TicketSource[], wiki: WikiSource[]}` (milik `lark-config-ipc`; hanya dipanggil, tidak diimplementasi di sini). Tipe lokal minimal ditulis di `source.ts` (tanpa berkas tipe bersama).
- `showToast(text, kind?)` dari `frontend/src/modules/openfolder/toast-store.ts`.
- `pasteToSession(sessionId, text): boolean` dari `frontend/src/modules/terminal/index.ts` (re-ekspor `paste.ts`).
- `layout` (store) dari `frontend/src/lib/stores/layout.ts`.

## Langkah

1. **Tipe di `layout/model.ts`** (lewati bagian yang sudah ditambah Foundation). Pastikan `PaneKind = 'terminal' | 'file' | 'thread' | 'lark-board' | 'lark-doc'`, serta `Leaf` dan `NewLeaf` memiliki `sourceId?: string; tableId?: string; viewId?: string; nodeToken?: string;`.

2. **`makeLeaf` (model.ts)**: setelah baris `readOnly`, tambahkan salinan field: `if (nl.sourceId !== undefined) leaf.sourceId = nl.sourceId;` dan sama untuk `tableId`, `viewId`, `nodeToken`. Judul default `makeLeaf` tidak diubah (pemanggil mengirim `title`).

3. **Dedupe `openPane` (model.ts)**: di predikat `findLeafBy` dalam `openPane`, tambahkan sebelum `return false`:
   ```ts
   if (nl.kind === 'lark-board' && l.kind === 'lark-board' && nl.sourceId !== undefined && l.sourceId === nl.sourceId) return true;
   if (nl.kind === 'lark-doc' && l.kind === 'lark-doc' && nl.sourceId !== undefined && l.sourceId === nl.sourceId && (l.nodeToken ?? '') === (nl.nodeToken ?? '')) return true;
   ```
   Kondisi `sessionId`/`filePath`/`threadId` yang ada tidak diubah. Pane yang cocok difokuskan lewat `setFocus` (perilaku `openPane` yang ada).

4. **Loader (model.ts, `hydrateNode`)**: tambahkan `'sourceId', 'tableId', 'viewId', 'nodeToken'` ke daftar kunci string yang disalin (`for (const k of [...] as const)`). Jangan menambah validasi `kind`: loader sudah menerima `kind` string apa pun dan tidak boleh menyaring jenis tak dikenal (itu yang membuat pane tak dikenal tetap tersimpan ulang). Pastikan tidak ada kode lain yang membuang leaf berdasarkan `kind` (cek `modules/persist/*` dengan grep; saat ini tidak ada, tidak perlu diubah).

5. **`lib/stores/layout.ts`**: tidak ada perubahan fungsional. `openPane(leaf: NewLeaf)` sudah meneruskan seluruh `NewLeaf`; field baru lewat tipe. Ubah file ini hanya bila type-check menuntut. Verifikasi `layout.openPane({ kind: 'lark-board', sourceId: 'x', title: 'T' })` lulus type-check.

6. **Pane tak dikenal (`layout/panes/hosts.ts`)**: ini renderer yang sebenarnya (bukan `LayoutArea.svelte`), jadi `LayoutArea.svelte` tidak perlu disentuh. Di `mountInto`, ganti teks `'Jenis pane tidak dikenal'` menjadi `` `Pane tidak dikenal: ${entry.kind}` `` (pakai `showText`). Tidak ada perubahan lain: leaf tetap ada di pohon dan ikut `serialize()`. Bila `registerPaneKind` untuk kind itu datang belakangan, `updateEntry` sudah memasang ulang saat `entry.instance` null; jangan diubah.

7. **`lark/source.ts` (baru)**:
   ```ts
   import { get } from 'svelte/store';
   import { call } from '../../lib/ipc';
   import { activeWorkspaceId } from '../../lib/stores/workspaces';

   export type SourceStatus = 'ok' | 'missing' | 'error';
   type SourceRef = { id: string };
   type SourcesResp = { tickets?: SourceRef[]; wiki?: SourceRef[] };

   export async function resolveSource(sourceId: string): Promise<SourceStatus> { ... }
   ```
   Perilaku: `sourceId === 'personal'` -> `'ok'` tanpa IPC. `activeWorkspaceId` kosong -> `'error'`. Selain itu `call<SourcesResp>('lark.sources.get', { workspaceId })`; bila `tickets` atau `wiki` memuat `id === sourceId` -> `'ok'`, bila tidak -> `'missing'`; galat IPC (try/catch) -> `'error'`. Jangan mengimpor `store.ts` atau `client.ts` (milik `lark-panel-ui`) agar tidak ada ketergantungan silang. Tambahkan juga helper reaktif kecil untuk komponen: `export function watchSource(sourceId: () => string | undefined, onChange: (s: SourceStatus | 'checking') => void): () => void` yang memanggil `resolveSource` saat dipasang dan setiap `activeWorkspaceId` berganti, dengan penghitung `seq` untuk membuang hasil basi; mengembalikan fungsi unsubscribe. (Sumber yang dihapus saat pane terbuka baru terdeteksi saat workspace berganti atau pane dibuka ulang; ini diterima, tanpa polling.)

8. **`lark/actions.ts` (baru)**:
   - `copyLink(url)`: coba `await navigator.clipboard.writeText(url)`; bila tidak ada/ditolak, fallback textarea sementara (`position:fixed; opacity:0`, `select()`, `document.execCommand('copy')`, lepas elemen). Sukses -> `showToast('Link disalin')`; gagal total -> `showToast('Gagal menyalin link', 'error')`.
   - `sendToAgent(url)`: ambil `get(layout)`; leaf terfokus (`doc.focusId`) bila `kind === 'terminal'` dan punya `sessionId`, selain itu leaf terminal pertama dengan `sessionId` dari `get(layout.leaves)`. Hanya `kind === 'terminal'` (bukan thread). Tanpa leaf, atau `pasteToSession(sessionId, url + ' ')` mengembalikan `false` -> `showToast('Tidak ada sesi aktif', 'error')`. Tanpa Enter otomatis.

9. **`lark/board/BoardPane.svelte` (baru)**: `let { pane, visible }: { pane: Leaf; visible: boolean } = $props();` (`Leaf` dari `../../layout/model`). Gunakan `watchSource` (atau `resolveSource`) pada `pane.sourceId` dengan `$effect`. Render: status `'missing'` -> teks "Sumber tidak ada di workspace ini"; `'error'` -> "Tidak dapat memeriksa sumber"; `'ok'`/memeriksa -> judul "Tiket Lark" + nama `pane.title` dan "Segera hadir". Ada komentar bahwa file ini ditimpa tiket `lark-tickets` dan props jangan diubah. Gaya memakai variabel tema yang ada (`var(--fg-muted)`, `var(--bg)`), root `flex:1; min-height:0; padding`. Tanpa `scoped` tergantung gaya komponen lain; pakai `<style>` biasa.

10. **`lark/doc/DocPane.svelte` (baru)**: sama dengan langkah 9, judul "Dokumen Lark", memeriksa `pane.sourceId` (`'personal'` selalu ok), menampilkan "Segera hadir".

11. **`lark/index.ts`**: file ini juga disentuh `lark-sources-wiring` (rail item) dan mungkin belum ada; buat bila belum ada, dengan bagian terpisah dan berkomentar agar mudah digabung:
    ```ts
    import { registerPaneKind } from '../layout/registry';
    import BoardPane from './board/BoardPane.svelte';
    import DocPane from './doc/DocPane.svelte';

    // --- lark-pane-foundation: registrasi pane dan ekspor ---
    registerPaneKind({ kind: 'lark-board', component: BoardPane, titleFor: (leaf) => leaf.title || 'Tiket Lark' });
    registerPaneKind({ kind: 'lark-doc', component: DocPane, titleFor: (leaf) => leaf.title || 'Dokumen Lark' });
    export { resolveSource } from './source';
    export type { SourceStatus } from './source';
    // --- akhir lark-pane-foundation ---
    ```
    Bila file sudah ada (ditulis fitur lain), tambahkan blok ini tanpa menghapus isi yang ada. Jangan menambah `registerRailItem`. Tidak perlu impor di `main.ts` (glob `./modules/*/index.ts`). Hindari siklus: `index.ts` mengimpor pane, pane mengimpor `../source`, bukan `../index`.

12. **Verifikasi**: `cd frontend && npm run build`. Jalankan juga `npm run check` bila cepat dan pastikan tidak ada galat baru pada path fitur ini. `git diff --stat` hanya memuat path di Metadata (hunk di `model.ts`/`layout.ts` hanya hunk sendiri).

## Perubahan

Dibuat:
- `frontend/src/modules/lark/source.ts`
- `frontend/src/modules/lark/actions.ts`
- `frontend/src/modules/lark/board/BoardPane.svelte`
- `frontend/src/modules/lark/doc/DocPane.svelte`
- `frontend/src/modules/lark/index.ts` (bila belum ada; selain itu diubah)

Diubah:
- `frontend/src/modules/layout/model.ts` (tipe bila belum, `makeLeaf`, dedupe `openPane`, loader `hydrateNode`)
- `frontend/src/modules/layout/panes/hosts.ts` (teks pane tak dikenal; satu baris)
- `frontend/src/lib/stores/layout.ts` (hanya bila type-check menuntut)

## Wiring

Yang diekspos untuk langkah wiring dan fitur lain (tidak disambungkan sendiri):
- Jenis pane `lark-board` / `lark-doc` terdaftar saat modul `lark` dimuat; `lark-panel-ui` membukanya lewat `layout.openPane({ kind: 'lark-board', sourceId, title })` dan `layout.openPane({ kind: 'lark-doc', sourceId, nodeToken, title })`.
- `resolveSource` (diekspor ulang dari `lark/index.ts`, didefinisikan di `lark/source.ts`) dan `copyLink`/`sendToAgent` dari `lark/actions.ts` untuk tiket 03/04.
- `lark-sources-wiring` menambah `registerRailItem` di `lark/index.ts` di luar blok fitur ini.

## Selesai bila

- [ ] `cd frontend && npm run build` sukses (guard path fitur ini); tanpa unit test.
- [ ] `lark-board` dan `lark-doc` terdaftar; membuka lewat `layout.openPane` menampilkan "Segera hadir"; judul tab dari `leaf.title`.
- [ ] Membuka pane `lark-board` yang sama (`sourceId` sama) atau `lark-doc` yang sama (`sourceId` + `nodeToken` sama) memfokuskan pane yang ada, tidak menggandakan.
- [ ] Field `sourceId`, `tableId`, `viewId`, `nodeToken` bertahan setelah `serialize()` lalu `hydrate()`.
- [ ] Pane dengan `kind` tak dikenal tampil "Pane tidak dikenal: <kind>", tidak dibuang dari layout dan tetap ikut tersimpan ulang.
- [ ] `sourceId` yang tidak ada di workspace aktif -> "Sumber tidak ada di workspace ini"; `'personal'` selalu ok.
- [ ] `copyLink` menyalin (dengan fallback) dan menampilkan toast "Link disalin"; `sendToAgent` menempel `url + ' '` ke sesi terminal aktif tanpa Enter, atau toast "Tidak ada sesi aktif".
- [ ] Perilaku pane terminal, file, dan thread tidak berubah; tidak ada hunk tak terkait yang diubah atau di-stage.
