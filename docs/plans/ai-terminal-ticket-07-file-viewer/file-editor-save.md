# Plan: Editor, simpan, dan konflik

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-07-file-viewer.md` (epic 07 File Viewer)
- Fitur: `file-editor-save` (Editor, simpan, dan konflik)
- ownedPaths:
  - `frontend/src/modules/files/editor/`
  - `frontend/src/modules/files/index.ts`
- Depends on foundation: ya. `shared/` (adaptor `fs.*`, adaptor ui-shell, tipe) dan stub `viewer/*`, `tree/*`, `editor/*` sudah dibuat Foundation. Mulai dari stub `editor/writeApi.ts` dan `editor/FileTab.svelte`, lalu TIMPA keduanya. Rujukan desain bersama: `docs/plans/ai-terminal-ticket-07-file-viewer/overview.md` (bagian 2 dan 7). Jangan mengulang isinya; plan ini hanya memakai kontraknya.

## Tujuan

Membuat komponen tab `file` yang merangkai viewer (dari fitur `file-tab-viewer`) dengan mode sunting `<textarea>` polos, simpan Cmd+S berbasis `expectedMtime`, penanda belum disimpan, banner konflik (`ERR_CONFLICT`) dengan pilihan Muat ulang / Timpa (dengan konfirmasi) / Batal, banner perubahan eksternal lewat `watchFile` (hanya saat tab tampil), dan peringatan sunting file > 5 MiB. Juga membuat `index.ts` modul yang mendaftarkan tipe tab `file` dan panel "Files". Semua label UI bahasa Indonesia; identifier bahasa Inggris. Tanpa dependency baru, tanpa timer/polling, tanpa `innerHTML`, tanpa unit test.

## Path yang dimiliki

HANYA path ini yang boleh dibuat/diubah:

- `frontend/src/modules/files/editor/writeApi.ts` (timpa stub)
- `frontend/src/modules/files/editor/FileTab.svelte` (timpa stub)
- `frontend/src/modules/files/index.ts` (baru)

Dilarang menyentuh: `frontend/src/modules/files/shared/`, `viewer/`, `tree/` (pakai apa adanya, termasuk stub; bila butuh perubahan kontrak, laporkan sebagai blocker), file wiring lain, `frontend/package.json`, `frontend/src/lib/`, `modules/shell|tabs|workspace|terminal`, `frontend/src/styles/`, kode Go, serta path fitur lain. Jangan memanggil `call('fs.*')` langsung dan jangan mengimpor apa pun dari ui-shell langsung; semuanya lewat `shared/fsClient.ts` dan `shared/shellBridge.ts`. Jangan commit.

## Kontrak

### Disediakan fitur ini

```ts
// editor/writeApi.ts
export class ConflictError extends Error { currentMtime?: number }
export function writeFile(path: string, content: string, expectedMtime: number): Promise<{ mtime: number }>;

// editor/FileTab.svelte   props: diterima dari TabContent (bentuk tidak diasumsikan), didekode lewat decodeFileTab
// index.ts: registerTabType({ type: 'file', component: FileTab }); registerFilesPanel();
```

### Dipakai (signature dari overview 2.4 dan 2.5)

- `shared/fsClient.ts`: `fsWrite(absPath, content, expectedMtime): Promise<{mtime; size}>`, `watchFile(absPath, onChange): () => void`, `FsConflictError` (field `currentMtime?`), `basename`.
- `shared/shellBridge.ts`: `decodeFileTab(props): FileTabData` dan `setTabDirty(path, dirty): void`.
- `shared/types.ts`: `FileTabData`, `FileChangedEvent` (`op`, `mtime`).
- `viewer/fileApi.ts`: `fileKind(path): 'markdown'|'text'`, `readChunk(path, offset, length): Promise<Chunk>`, `readAll(path): Promise<Chunk>`, `BinaryFileError`, `FileNotFoundError`. `Chunk` = `{content, size, mtime, eof, offset, length}`.
- Komponen viewer: `viewer/MarkdownView.svelte` (props `{source}`), `viewer/PlainTextView.svelte` (props `{path, size}`), `viewer/BinaryNotice.svelte` (props `{path}`).
- `tree/registerTree.ts`: `registerFilesPanel(): void`.
- `registerTabType` dari `frontend/src/lib/registry.ts` (baca ekspor dan signature sebenarnya sebelum menulis `index.ts`).
- Konstanta lokal (didefinisikan di `FileTab.svelte`): `EDIT_WARN_BYTES = 5 * 1024 * 1024`, `MARKDOWN_MAX_BYTES = 2 * 1024 * 1024`.

Semantik `mtime`: token opaque bertipe number; bandingkan dengan `===` dan kirim balik apa adanya (overview 2.3). Jangan konversi satuan.

## Langkah

1. **Baca konteks (read-only).** Baca `shared/types.ts`, `shared/fsClient.ts`, `shared/shellBridge.ts` (terutama `decodeFileTab` dan `setTabDirty`), stub `viewer/*`, `tree/registerTree.ts`, dan `frontend/src/lib/registry.ts` (bentuk `registerTabType`). Lihat satu komponen Svelte 5 yang sudah ada di `frontend/src/modules/` untuk konvensi runes dan variabel CSS (`frontend/src/styles/`, hanya dibaca). Bila signature kontrak ternyata berbeda dari yang tertulis di sini, ikuti kode `shared/` dan catat di laporan.

2. **`editor/writeApi.ts`** (hapus penanda STUB):
   - `export class ConflictError extends Error { currentMtime?: number; constructor(message = 'File berubah di disk', currentMtime?: number) }`, set `this.name = 'ConflictError'`.
   - `export async function writeFile(path, content, expectedMtime): Promise<{ mtime: number }>`: `try { const r = await fsWrite(path, content, expectedMtime); return { mtime: r.mtime }; } catch (e) { if (e instanceof FsConflictError) throw new ConflictError(e.message, e.currentMtime); throw e; }`.

3. **`editor/FileTab.svelte`** (timpa stub; Svelte 5 runes, `<script lang="ts">`).

   a. Props dan data: `let props: Record<string, unknown> = $props();` lalu `const data = decodeFileTab(props)` (bila props reaktif, bungkus `$derived`). `path = data.path`. `kind = fileKind(path)`. Bila `$props()` tidak dapat dikumpulkan sebagai satu objek, gunakan `$props()` dengan rest (`let { ...rest } = $props()`); tidak ada fitur lain yang tahu skema tab.

   b. State (`$state`):
   - `phase: 'loading' | 'ready' | 'binary' | 'notfound' | 'error'`, `errorMessage: string`
   - `mode: 'view' | 'edit'` (awal `view`)
   - `content: string` (isi yang tampil/disunting), `savedContent: string` (isi terakhir dibaca/disimpan), `baseMtime: number`, `size: number`
   - `saving: boolean`, `justSaved: boolean` (label "Tersimpan" hilang lewat satu `setTimeout` sekali pakai 2 detik, dibersihkan saat destroy; ini bukan polling)
   - `banner: null | { type: 'conflict' | 'external' | 'removed'; currentMtime?: number }`
   - `confirmOverwrite: boolean` (konfirmasi inline untuk Timpa)
   - `largeWarn: boolean` (peringatan sebelum masuk sunting)
   - `reloadKey: number` (dinaikkan untuk me-remount `PlainTextView` setelah simpan/muat ulang)
   - `visible: boolean`
   - Turunan: `dirty = $derived(phase === 'ready' && content !== savedContent)`, `isMarkdownKind = kind === 'markdown'`, `largeMarkdown = isMarkdownKind && size > MARKDOWN_MAX_BYTES`.

   c. Muat awal `async function load()`: set `phase='loading'`; `try`:
   - Probe `const probe = await readChunk(path, 0, 1)` untuk mendapat `size`, `mtime`, dan memicu `BinaryFileError`/`FileNotFoundError` lebih awal. Set `size = probe.size`, `baseMtime = probe.mtime`.
   - Bila `kind === 'markdown'` dan `size <= MARKDOWN_MAX_BYTES`: `const all = await readAll(path)`; `content = savedContent = all.content`; `baseMtime = all.mtime`; `size = all.size`.
   - Bila `kind === 'text'` atau markdown besar: jangan baca penuh; `content = savedContent = ''` (isi dibaca viewer malas atau saat masuk mode sunting).
   - `phase = 'ready'`; `banner = null`.
   - `catch`: `BinaryFileError` -> `phase='binary'`; `FileNotFoundError` -> `phase='notfound'`; lainnya -> `phase='error'`, `errorMessage = e instanceof Error ? e.message : String(e)`.
   - Panggil `load()` sekali saat mount (di `$effect` yang hanya bergantung `path`, atau `onMount`).

   d. `function reload()` (dipakai Muat ulang): buang suntingan, `confirmOverwrite=false`, `banner=null`, panggil `load()` ulang; bila `mode==='edit'` baca ulang penuh (`readAll`) dan set `content`, `savedContent`, `baseMtime`; naikkan `reloadKey`. Jangan menyentuh suntingan sebelum pengguna memilih Muat ulang.

   e. Mode sunting:
   - `async function enterEdit()`: bila `size > EDIT_WARN_BYTES` dan belum dikonfirmasi, set `largeWarn = true` dan berhenti. Tombol di peringatan: "Tetap sunting" (set `largeWarn=false`, lanjut) dan "Batal". Tulis pesan satu baris: "File besar (X MiB). Menyunting memuat seluruh isi ke memori. Lanjutkan?".
   - Lanjut: `const all = await readAll(path)`; `content = savedContent = all.content`; `baseMtime = all.mtime`; `size = all.size`; `mode = 'edit'`. `BinaryFileError`/`FileNotFoundError` ditangani seperti di `load()`.
   - Untuk kind `text` tombol "Edit" ada; untuk kind `markdown` ada pasangan tombol "Tampilan" / "Edit" (state aktif ditandai `aria-pressed`). Untuk kind `text`, tombol "Tampilan" juga ada untuk kembali dari sunting, tetapi `disabled` bila `dirty` (judul/tooltip: "Simpan dulu sebelum kembali ke tampilan") karena `PlainTextView` membaca dari disk. Setelah kembali dari sunting untuk kind `text`, naikkan `reloadKey`.
   - Kembali ke `view` untuk markdown boleh saat dirty; `MarkdownView` merender `content` (isi sunting sekarang). Markdown besar (`largeMarkdown`) di mode `view` memakai `PlainTextView` dengan banner "File besar, ditampilkan sebagai teks".
   - `<textarea bind:value={content} spellcheck="false" wrap="off">` memenuhi sisa tinggi, font monospace, `tab-size: 4`, tanpa highlighting. Nonaktifkan (`readonly`) saat `saving`.

   f. Dirty: `$effect(() => { setTabDirty(path, dirty); })`. Di cleanup destroy (`onDestroy` atau return `$effect`) panggil `setTabDirty(path, false)`. Bilah file menampilkan titik "●" (dengan `title="Belum disimpan"` dan teks sembunyi untuk pembaca layar) di samping `basename(path)` saat `dirty`.

   g. Simpan `async function save()`:
   - Guard: `if (saving || mode !== 'edit' || !dirty) return`; `saving = true`.
   - `try { const r = await writeFile(path, content, baseMtime); baseMtime = r.mtime; savedContent = content; size = new TextEncoder().encode(content).length; justSaved = true; banner = null; confirmOverwrite = false; ... } catch (e) { if (e instanceof ConflictError) { banner = { type: 'conflict', currentMtime: e.currentMtime }; } else if (e instanceof FileNotFoundError ... ) { banner = { type: 'removed' } } else { saveError = pesan } } finally { saving = false }`.
   - Kegagalan simpan lain tampil sebagai baris galat kecil di bilah ("Gagal menyimpan: ...") tanpa membuang suntingan.
   - Handler Cmd+S: `onkeydown` pada elemen akar komponen (`tabindex="-1"`): `if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') { e.preventDefault(); void save(); }`. Karena terpasang pada elemen akar, otomatis hanya aktif saat fokus di dalam tab ini; tab tersembunyi tidak menerima key event. Tombol "Simpan" di bilah juga memanggil `save()` dan `disabled` saat `saving || !dirty`.

   h. Banner konflik (`banner.type === 'conflict'`), teks "File berubah di disk" (bila dirty tambah "Suntingan Anda belum hilang"):
   - "Muat ulang": `reload()` (buang suntingan, baca ulang, perbarui `baseMtime`).
   - "Timpa": set `confirmOverwrite = true` menampilkan konfirmasi inline "Timpa file di disk dengan suntingan Anda?" dengan tombol "Ya, timpa" dan "Batal". Pada "Ya, timpa": `const fresh = await readChunk(path, 0, 1); baseMtime = fresh.mtime;` lalu `await writeFile(path, content, baseMtime)` memakai alur `save()` yang sama (faktorkan `doWrite()`); jika gagal konflik lagi, banner konflik tetap tampil. Tidak boleh menimpa tanpa konfirmasi.
   - "Batal": `banner = null`, `confirmOverwrite = false`; tetap dengan suntingan (`baseMtime` tidak diubah, jadi simpan berikutnya konflik lagi, itu diharapkan).

   i. Banner perubahan eksternal (`banner.type === 'external'`), teks "File berubah di disk" dengan tombol "Muat ulang" (sama dengan `reload()`) dan "Abaikan" (`banner = null`). Bila `dirty`, banner memakai gaya konflik yang sama dan HANYA menawarkan pilihan (tidak menyentuh suntingan); sediakan juga "Timpa" seperti di (h) agar perilaku konsisten. Untuk `op === 'remove'` gunakan `type: 'removed'` dengan teks "File dihapus di disk" dan tombol "Tutup banner" (tidak ada Muat ulang).

   j. Watch hanya saat tampil:
   - Variabel `let unwatch: (() => void) | null = null`.
   - `function startWatch()`: `if (unwatch || phase !== 'ready') return; unwatch = watchFile(path, onFsChanged)`.
   - `function stopWatch()`: `unwatch?.(); unwatch = null` (fungsi pengembalian `watchFile` idempoten).
   - `function onFsChanged(ev: FileChangedEvent)`: abaikan bila `saving`; abaikan bila `ev.op !== 'remove' && ev.mtime === baseMtime` (event akibat simpan sendiri); `op==='remove'` -> `banner = {type:'removed'}`; selain itu `banner = {type:'external', currentMtime: ev.mtime}`. Jangan memuat ulang otomatis.
   - Deteksi tampil: `visible` dari `IntersectionObserver` pada elemen akar (`bind:this={rootEl}`; elemen yang `display:none` atau terlepas melaporkan `isIntersecting=false`) digabung `document.visibilityState` via `document.addEventListener('visibilitychange', ...)`. `$effect` mengamati `visible && phase === 'ready'`: bila benar `startWatch()`, bila tidak `stopWatch()`. Saat kembali tampil dari tersembunyi, tidak ada cek ulang otomatis di luar event; pembaruan disk selama tersembunyi terdeteksi saat simpan (konflik) dan saat event berikutnya. Boleh juga satu kali cek `readChunk(path,0,1)` saat kembali tampil: bila `mtime !== baseMtime`, tampilkan banner `external`. Itu satu panggilan, bukan polling.
   - Destroy: `onDestroy` -> `stopWatch()`, `observer.disconnect()`, lepas listener `visibilitychange`, `clearTimeout` label "Tersimpan", `setTabDirty(path, false)`. Tidak ada `setInterval`.
   - Bila komponen di-unmount saat tersembunyi oleh ui-shell, jalur destroy cukup; bila di-keep-alive, jalur `visible=false` yang menutup watch. Keduanya ditangani.

   k. Render (semua label Indonesia):
   - Akar: `<div class="file-tab" bind:this={rootEl} tabindex="-1" onkeydown={onKeydown}>`.
   - Bilah file (`header`): nama (`basename(path)`) + titik dirty, path penuh sebagai `title`, label "Tersimpan" saat `justSaved`, tombol "Simpan" (hanya mode edit), kelompok "Tampilan"/"Edit".
   - Banner (konflik/eksternal/dihapus) di bawah bilah, `role="alert"`.
   - Peringatan file besar (`largeWarn`) di bawah bilah.
   - Konten menurut `phase`: `loading` -> "Memuat..."; `binary` -> `<BinaryNotice {path} />` (tanpa tombol sunting, sembunyikan kelompok tombol); `notfound` -> "File tidak ditemukan: <path>. Anda bisa menutup tab ini."; `error` -> pesan galat + tombol "Coba lagi" (`load()`); `ready` -> bila `mode==='edit'` textarea; selain itu `kind==='markdown' && !largeMarkdown` -> `<MarkdownView source={content} />`; selain itu `{#key reloadKey}<PlainTextView {path} {size} />{/key}` (untuk markdown besar tambahkan banner "File besar, ditampilkan sebagai teks").
   - Styling: variabel CSS ui-shell bila ada (cek `frontend/src/styles/`) dengan fallback `inherit`/`currentColor`; layout flex kolom tinggi penuh, konten `overflow:auto`/`min-height:0`; monospace untuk textarea. CSS ke dalam `<style>` komponen saja.

4. **`index.ts`** (`frontend/src/modules/files/index.ts`):
   ```ts
   import { registerTabType } from '../../lib/registry';
   import FileTab from './editor/FileTab.svelte';
   import { registerFilesPanel } from './tree/registerTree';
   registerTabType({ type: 'file', component: FileTab });
   registerFilesPanel();
   ```
   Sesuaikan path impor dan bentuk argumen dengan `frontend/src/lib/registry.ts` yang sebenarnya (misalnya bila `registerTabType` membutuhkan field tambahan). Jangan memanggil apa pun selain dua registrasi ini.

5. **Verifikasi.**
   - `cd frontend && npm run build` harus lolos (dan `npm run check` bila ada dan bersih untuk file milik fitur ini).
   - `grep -rn "setInterval\|innerHTML\|call('fs\." frontend/src/modules/files/editor frontend/src/modules/files/index.ts` harus kosong.
   - `grep -n "STUB(foundation)" frontend/src/modules/files/editor` harus kosong.
   - Telaah manual alur: sunting -> dirty tampil; Cmd+S -> `fsWrite` dengan `expectedMtime = baseMtime` dan `baseMtime` terbarui dari balasan; konflik tidak menimpa; Timpa butuh konfirmasi; `watchFile` hanya aktif saat `visible`.
   - `git status`/`git diff --stat` hanya menunjukkan path milik fitur ini (dan lockfile tidak berubah).

## Perubahan

- Diubah (menimpa stub Foundation): `frontend/src/modules/files/editor/writeApi.ts`, `frontend/src/modules/files/editor/FileTab.svelte`
- Dibuat: `frontend/src/modules/files/index.ts`
- Tidak ada perubahan lain.

## Wiring

Fitur ini mengekspos, untuk disambungkan oleh langkah wiring (tidak disambungkan sendiri; tidak ada edit ke registry, shell, `App.svelte`, atau `main.ts`):

- `frontend/src/modules/files/index.ts`: dimuat otomatis oleh `import.meta.glob` di `main.ts`; memanggil `registerTabType({type:'file', component: FileTab})` dan `registerFilesPanel()`.
- `editor/FileTab.svelte` sebagai komponen tab `file`; hanya mengonsumsi data tab lewat `decodeFileTab`, jadi harus konsisten dengan `openFileTab` di `shared/shellBridge.ts` (diperiksa oleh langkah wiring).
- `editor/writeApi.ts` (`ConflictError`, `writeFile`), tidak ada konsumen luar.
- Efek samping ke ui-shell hanya lewat `setTabDirty` (event `aiterm:tab-dirty` bila shell tidak punya API).
- Ketergantungan yang harus sudah tergantikan saat wiring: `viewer/*` (fitur `file-tab-viewer`) dan `tree/*` (fitur `file-tree-panel`); sebelum itu `index.ts` tetap membangun dengan stub Foundation.

## Selesai bila

- [ ] `editor/writeApi.ts` dan `editor/FileTab.svelte` tidak lagi memuat `STUB(foundation)` dan signature sesuai kontrak; `index.ts` ada dan persis mendaftarkan tab `file` serta panel Files.
- [ ] Mengetik di mode sunting memunculkan "●" di bilah file dan memanggil `setTabDirty(path, true)`; setelah simpan, penanda hilang dan `setTabDirty(path, false)`.
- [ ] Cmd+S memanggil `writeFile(path, content, baseMtime)` dan memperbarui `baseMtime` dari balasan; tombol/handler ganda dinonaktifkan selama `saving`.
- [ ] `ConflictError` menampilkan banner "File berubah di disk" tanpa menimpa; Muat ulang membaca ulang dan memperbarui `baseMtime`; Timpa hanya setelah konfirmasi dan memakai mtime terbaru dari `readChunk(path,0,1)`; Batal mempertahankan suntingan.
- [ ] Event `fs.changed` dengan mtime berbeda dari `baseMtime` memunculkan banner (suntingan dirty tidak disentuh); event dari simpan sendiri diabaikan.
- [ ] `watchFile` dipanggil hanya saat tab tampil dan fungsi unwatch dipanggil saat disembunyikan atau dihancurkan; tidak ada `setInterval`/polling.
- [ ] Peringatan satu baris muncul sebelum sunting bila `size` > 5 MiB; pengguna dapat melanjutkan atau batal.
- [ ] Tab `.md`/`.txt` dapat berganti Tampilan/Edit; mode sunting tanpa highlighting; biner menampilkan `BinaryNotice`; file hilang menampilkan pesan tidak ditemukan.
- [ ] Guard: `cd frontend && npm run build` sukses dengan perubahan hanya di path fitur ini (tanpa unit test; `package.json` dan lockfile tidak berubah).
