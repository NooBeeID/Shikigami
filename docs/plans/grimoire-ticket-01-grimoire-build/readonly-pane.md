# Plan: Pane file read-only berroot grimoire

## Metadata

- Tiket: `grimoire-ticket-01-grimoire-build` (`docs/tickets/grimoire-ticket-01-grimoire-build.md`)
- Fitur: `readonly-pane`
- Overview (sumber kebenaran kontrak bersama): `docs/plans/grimoire-ticket-01-grimoire-build/overview.md` (bagian 2.6 dan 3). Baca sebelum mulai; jangan diubah.
- ownedPaths:
  - `frontend/src/modules/layout/model.ts`
  - `frontend/src/modules/layout/panes/file.ts`
  - `frontend/src/modules/layout/panes/FilePane.svelte`
  - `frontend/src/modules/layout/panes/ReadOnlyFileView.svelte` (baru)
  - `frontend/src/modules/files/viewer/fileApi.ts`
  - `frontend/src/modules/grimoire/openPage.ts` (stub Foundation, ditimpa penuh)
- Depends on foundation: ya. Foundation hanya menyediakan stub `frontend/src/modules/grimoire/openPage.ts` dengan signature final (`export async function openGrimoirePage(projectId: string, relPath: string): Promise<string>`, baris pertama `// STUB(foundation): ditimpa oleh readonly-pane`). Berkas lain di ownedPaths sudah ada di repo dan diubah aditif.

## Tujuan

Leaf pane `file` dapat membawa `root` eksplisit dan flag `readOnly`, sehingga halaman grimoire (di `<DataDir>/grimoire/<projectId>/`, di luar `Project.Path`) tampil sebagai Markdown read-only lewat `fs.read` berroot. Tanpa jenis pane baru, tanpa `FileTab`, tanpa `fs.write`, tanpa `fs.watch`. Helper `openGrimoirePage` membuka pane itu (atau memfokuskan yang sudah ada). Pane file biasa tidak berubah perilakunya.

## Path yang dimiliki

HANYA enam path pada Metadata yang boleh disentuh. Jangan menyentuh `frontend/src/modules/files/shared/fsClient.ts`, `FileTab.svelte`, `MarkdownView.svelte`, `PlainTextView.svelte`, `shell/helpers.ts`, `lib/stores/*`, `registry.ts`, `main.ts`, apa pun di `modules/grimoire/` selain `openPage.ts` (itu milik `panel-ui`), `internal/*`, `cmd/*`, `package.json`/lockfile, atau berkas wiring. Bila build gagal karena berkas fitur lain, jangan perbaiki; laporkan.

## Kontrak

Disediakan (signature final, jangan diubah; lihat overview 2.6):

- `model.ts`: `Leaf` dan `NewLeaf` memperoleh `root?: string` dan `readOnly?: boolean`.
- `fileApi.ts`: `readChunk(path: string, offset: number, length: number, root?: string): Promise<Chunk>` dan `readAll(path: string, root?: string): Promise<Chunk>`. Dengan `root`, `path` adalah path absolut, rel dihitung dengan memotong `root + '/'`, lalu `call('fs.read', {root, path: rel, offset, limit})`.
- `ReadOnlyFileView.svelte` props: `{ root: string; path: string; projectId?: string }`.
- `openPage.ts`: `export async function openGrimoirePage(projectId: string, relPath: string): Promise<string>` (mengembalikan id pane).

Dipakai (sudah ada, baca aslinya):

- `files/shared/fsClient.ts`: `classifyError`, `FsBinaryError`, `FsNotFoundError`, `basename` (diekspor; jangan diubah).
- `files/shared/types.ts`: `CHUNK_SIZE`, `ReadResultRaw` (`content,size,mtime,eof,offset,length`).
- `lib/ipc`: `call`, `onEvent(topicPrefix, fn)`; `lib/stores/layout`: `layout` (`openPane`, `setFocus`); `layout/model.ts`: `findLeafBy`.
- `files/viewer/MarkdownView.svelte` (prop `source`), `BinaryNotice.svelte` (prop `path`), `markdown.ts`.
- IPC `grimoire.status` -> field `dir` (string absolut direktori grimoire project; diisi fitur `ipc-ctl`). Event `grimoire.changed` dengan payload `{projectId}`.
- Catatan: `layout.openPane` (model `openPane`) sudah memfokuskan leaf yang `filePath` dan `projectId`-nya sama, jadi dedupe dasar sudah ada; `openGrimoirePage` tetap memeriksa sendiri agar id pane yang dikembalikan pasti benar.

## Langkah

1. **`model.ts` (aditif).**
   - Pada `interface Leaf` tambahkan `root?: string;` dan `readOnly?: boolean;` (setelah `workspaceId`). Pada `interface NewLeaf` tambahkan field yang sama.
   - Pada `makeLeaf`, setelah baris `workspaceId`, tambahkan: `if (nl.root !== undefined) leaf.root = nl.root;` dan `if (nl.readOnly !== undefined) leaf.readOnly = nl.readOnly;`.
   - Pada `hydrateNode` (penyimpanan/pemulihan layout), tambahkan `'root'` ke daftar kunci string (`for (const k of ['sessionId', ..., 'workspaceId', 'root'] as const)`) dan setelah loop: `if (typeof r.readOnly === 'boolean') leaf.readOnly = r.readOnly;`. Ini perlu agar pane read-only tidak berubah menjadi pane file biasa (yang gagal di `splitPath`) setelah layout dipulihkan. Hanya penambahan; perilaku leaf lama tidak berubah. Jangan ubah fungsi lain.
2. **`fileApi.ts`.**
   - Impor tambahan: `call` dari `'../../../lib/ipc'`; `classifyError` dari `'../shared/fsClient'` (gabungkan dengan impor yang ada); `type ReadResultRaw` dari `'../shared/types'` bila perlu.
   - Tambahkan helper privat `relUnderRoot(root: string, path: string): string`: normalisasi `root` (buang `/` di ujung, kecuali bila root hanya `/`), bila `path` diawali `root + '/'` kembalikan sisanya; selain itu `throw new Error('path di luar root: ' + path)`.
   - Ubah `readChunk(path, offset, length, root?)`: bila `root` terisi, `const rel = relUnderRoot(root, path); const r = await call<ReadResultRaw>('fs.read', { root, path: rel, offset, limit: length });` di dalam `try`, dan galatnya lewat `mapError(classifyError(e))`; kembalikan objek `Chunk` yang bentuknya sama seperti jalur lama (mtime dipakai apa adanya). Tanpa `root`: jalur lama (`fsRead`) persis.
   - Ubah `readAllOnce(path, root?)` dan `readAll(path, root?)` agar meneruskan `root` ke `readChunk`. Semua pemanggil lama (tanpa argumen `root`) tetap valid.
3. **`ReadOnlyFileView.svelte` (baru).** Svelte 5 runes, `<script lang="ts">`, props `let { root, path, projectId }: { root: string; path: string; projectId?: string } = $props();`.
   - State: `status: 'loading' | 'ready' | 'notfound' | 'binary' | 'error'`, `content`, `size`, `errorMsg`, penghitung generasi `gen` agar hasil basi dibuang.
   - `load()`: naikkan `gen`, set `loading` (jangan kosongkan konten saat reload agar tidak berkedip bila sudah `ready`), `readAll(path, root)`. Sukses -> `ready`. Tangkap `BinaryFileError` -> `binary`, `FileNotFoundError` -> `notfound`, lainnya -> `error` dengan pesan.
   - `$effect` yang memanggil `load()` saat `root`/`path` berubah (gunakan `untrack` untuk state internal).
   - Langganan `onEvent('grimoire.changed', (_t, payload) => ...)` di `$effect` dengan cleanup (`return off`): bila `projectId` tak ada atau `payload?.projectId === projectId`, panggil `load()`. Tanpa `setInterval`, tanpa `fs.watch`, tanpa `fs.write`.
   - Render: `loading` -> teks "Memuat..."; `notfound` -> "Halaman tidak ditemukan. Bangun ulang grimoire." ; `binary` -> `<BinaryNotice {path} />`; `error` -> pesan galat; `ready` -> bila `fileKind(path) === 'markdown'` dan `size <= MARKDOWN_MAX_BYTES` (dari `fileApi.ts`) render `<MarkdownView source={content} />`, selain itu `<pre class="plain">{content}</pre>` (teks polos; JANGAN memakai `PlainTextView` karena ia membaca tanpa root). Bungkus dengan wadah `div` yang dapat di-scroll (`overflow:auto; height:100%; padding`) memakai token CSS yang sudah dipakai `FilePane` (`var(--fg-muted, #888)` dsb); tanpa token global baru. Tanpa tombol edit/save.
4. **`FilePane.svelte`.** Impor `ReadOnlyFileView from './ReadOnlyFileView.svelte'`. Ubah blok render menjadi: `{#if pane.readOnly && pane.root && pane.filePath}` -> `<ReadOnlyFileView root={pane.root} path={pane.filePath} projectId={pane.projectId} />`; `{:else if pane.filePath}` -> `<FileTab {refId} />` (persis lama); `{:else}` -> `.empty` lama. Jangan ubah `refId` dan gaya lama.
5. **`file.ts`.** Biarkan `titleFor` apa adanya (judul = nama berkas; opsional, boleh tanpa perubahan). Tidak ada keharusan mengubah berkas ini; bila tidak ada perubahan, tetap dicatat di Perubahan sebagai "tidak diubah".
6. **`openPage.ts`.** Timpa stub seluruhnya (hapus komentar STUB).
   - Impor: `get` dari `'svelte/store'`; `layout` dari `'../../lib/stores/layout'`; `findLeafBy` dari `'../layout/model'`; `call` dari `'../../lib/ipc'`. JANGAN mengimpor `client.ts` (milik `panel-ui`, bisa belum ada) dan jangan `shell/helpers.ts`.
   - `export async function openGrimoirePage(projectId: string, relPath: string): Promise<string>`:
     1. Validasi: `projectId` kosong atau `relPath` kosong -> `throw new Error('projectId dan path halaman wajib diisi')`; `relPath` yang mengandung segmen `..` atau diawali `/` -> `throw new Error('path halaman tidak valid')`.
     2. `const st = await call<{ dir?: string }>('grimoire.status', { projectId });` bila `!st?.dir` -> `throw new Error('direktori grimoire tidak tersedia; pastikan daemon sudah diperbarui')`.
     3. `const dir = st.dir.replace(/\/+$/, ''); const filePath = dir + '/' + relPath;`
     4. Cari pane ada: `findLeafBy(get(layout), (l) => l.kind === 'file' && l.filePath === filePath && l.projectId === projectId)`; bila ada, `layout.setFocus(existing.id)` dan `return existing.id`.
     5. Selain itu: `return layout.openPane({ kind: 'file', projectId, filePath, root: dir, readOnly: true, title: <nama bab> })` dengan judul = nama berkas tanpa `.md` (`relPath.split('/').pop()` lalu buang akhiran `.md`).
   - Galat dibiarkan dilempar; pemanggil (`panel-ui`) yang menangkapnya.
7. **Guard.** `cd frontend && npm run check` (bila ada) lalu `npm run build`. Pastikan `git diff --stat -- frontend/package.json frontend/package-lock.json` kosong dan hanya enam ownedPaths yang berubah.

## Perubahan

- Diubah: `frontend/src/modules/layout/model.ts` (field `root?`/`readOnly?` di `Leaf`/`NewLeaf`, penyalinan di `makeLeaf`, pemulihan di `hydrateNode`).
- Diubah: `frontend/src/modules/layout/panes/FilePane.svelte` (cabang read-only).
- Diubah: `frontend/src/modules/files/viewer/fileApi.ts` (`root` opsional di `readChunk`/`readAll`).
- Dibuat: `frontend/src/modules/layout/panes/ReadOnlyFileView.svelte`.
- Ditimpa: `frontend/src/modules/grimoire/openPage.ts` (menggantikan stub Foundation).
- Tidak diubah (boleh): `frontend/src/modules/layout/panes/file.ts`.

## Wiring

Tidak ada titik registrasi baru; jangan menyambungkan apa pun sendiri. Yang diekspos untuk langkah wiring dan fitur lain:

- `openGrimoirePage(projectId, relPath): Promise<string>` di `modules/grimoire/openPage.ts`, dipakai `GrimoirePanel.svelte` (`panel-ui`) dengan pola `void openGrimoirePage(...).catch(...)`.
- Kontrak lintas fitur yang harus ada agar berfungsi: field `dir` pada hasil `grimoire.status` (`ipc-ctl`) dan event `grimoire.changed {projectId}`. Bila `dir` belum ada saat dikerjakan paralel, `openGrimoirePage` melempar galat yang jelas (bukan diperbaiki di sini).
- Verifikasi wiring (oleh langkah wiring): `FilePane.svelte` mengarahkan leaf `readOnly && root` ke `ReadOnlyFileView.svelte`; pane read-only tidak pernah memanggil `fs.write`/`fs.watch`.

## Selesai bila

- `Leaf`/`NewLeaf` punya `root?` dan `readOnly?`; `makeLeaf` dan `hydrateNode` menyalinnya; leaf lama tidak berubah perilakunya.
- Leaf `file` dengan `readOnly` dan `root` menampilkan Markdown lewat `fs.read` berroot (`root` = direktori grimoire) tanpa `FileTab`, `fs.write`, `fs.watch`, dan memuat ulang pada `grimoire.changed` untuk project terkait.
- `readChunk`/`readAll` menerima `root` opsional; pemanggilan lama tanpa `root` tetap bekerja.
- `openGrimoirePage` mengembalikan id pane, membuka pane read-only baru atau memfokuskan pane yang sudah ada untuk halaman yang sama.
- Guard: build frontend (`npm run check` bila ada, lalu `npm run build`) sukses; tidak ada perubahan di luar ownedPaths, tidak ada dependency baru, tanpa unit test.
