# Plan: Pane lark-doc penampil dokumen

## Metadata

- Tiket: `lark-connector-ticket-04-lark-wiki` (`docs/tickets/lark-connector-ticket-04-lark-wiki.md`)
- Fitur: `lark-doc-pane` (Pane lark-doc penampil dokumen)
- ownedPaths:
  - `frontend/src/modules/lark/doc/DocPane.svelte`
  - `frontend/src/modules/lark/doc/MarkdownView.svelte`
  - `frontend/src/modules/lark/doc/SheetView.svelte`
  - `frontend/src/modules/lark/doc/docClient.ts`
  - `frontend/package.json` (TIDAK diubah, lihat overview keputusan 1.1)
- Depends on foundation: **tidak**. Frontend mandiri; `docClient.ts` mendefinisikan tipe dan `parseLarkError` sendiri (overview 1.9). Prasyarat: tiket 02 sudah merge (ada `modules/lark/actions.ts`, `modules/lark/doc/DocPane.svelte` placeholder, tipe `Leaf` dengan `sourceId`/`nodeToken`). Bila `actions.ts` atau placeholder `DocPane.svelte` belum ada, hentikan dan laporkan.

## Tujuan

Mengganti placeholder "Segera hadir" pada pane `lark-doc` dengan penampil dokumen read-only: header (judul, segarkan, Salin link, Kirim ke agen), isi menurut `objType` (docx sebagai markdown, sheet sebagai tabel sel, tipe lain judul + link "Buka di Lark"), keadaan memuat dan galat aman dengan tombol coba lagi. Data diambil lewat IPC `lark.wiki.get` (kontrak overview 2.2). Tanpa dependensi baru.

## Path yang dimiliki

HANYA lima path di Metadata yang boleh disentuh (empat berkas di `frontend/src/modules/lark/doc/` dan `frontend/package.json`, yang tidak diubah). Jangan sentuh: `sections/WikiSection.svelte`, `sections/WikiTree.svelte`, `wikiTreeClient.ts`, `actions.ts`, `source.ts`, `client.ts`, `store.ts`, `index.ts`, `LarkIcon.svelte`, `board/*`, `modules/files/*` (termasuk `files/viewer/MarkdownView.svelte` dan `markdown.ts`: hanya diimpor, tidak diubah), `modules/layout/*`, `modules/settings/*`, `lib/*`, semua berkas Go, file wiring (`cmd/shikigamictl/cmd_lark_wiki.go`), dan `package-lock.json`.

## Kontrak

Dipakai (periksa nama persis di kode; bila beda dari yang tertulis, pakai nama di kode dan laporkan selisih, jangan mengarang):

- IPC `lark.wiki.get` params `{workspaceId, sourceId, nodeToken}` -> `{title, objType, url, markdown?, cells?, truncated?}` (overview 2.2). `url` selalu string, boleh `""` bila host tidak diketahui. Galat berbentuk string `"[<code>] <pesan>"`; kode yang mungkin: `cli_missing`, `not_logged_in`, `permission`, `timeout`, `not_allowed`, `cli_error`, `source_not_found`, `invalid_params`.
- `call` dan `onEvent` dari `frontend/src/lib/ipc.ts` (`call<T>(method, params?)` menolak dengan `Error`).
- `activeWorkspaceId` dari `frontend/src/lib/stores/workspaces` (writable `string | null`), diimpor langsung dari berkas.
- `copyLink(url: string): Promise<void>` dan `sendToAgent(url: string): Promise<void>` dari `../actions` (toast ditangani di dalamnya, termasuk "Tidak ada sesi aktif"; komponen tidak membuat toast sendiri).
- `Leaf` dari `../../layout/model` (field `sourceId?`, `nodeToken?`, `title?`).
- `openSettings(pageId?: string)` dari `../../settings` (opsional; bila ada, tombol "Buka Pengaturan" memanggil `openSettings('connectors')`).
- `onEvent('workspace.changed', ...)` dengan payload `{kind: 'configChanged'}` (bila ada di kode; dipakai agar pane yang menampilkan galat `source_not_found` bisa dicoba ulang otomatis setelah config berubah).
- Renderer aman `../../files/viewer/MarkdownView.svelte` (prop `source: string`; HTML mentah jadi teks literal, `href` hanya `http(s)`/`mailto`, tautan `target="_blank" rel="noopener noreferrer"` + `BrowserOpenURL`).

Disediakan (tipe dan fungsi di `docClient.ts`):

```ts
export type LarkError = { code: string; message: string };
export type WikiGetResult = {
  title: string; objType: string; url: string;
  markdown?: string; cells?: string[][]; truncated?: boolean;
};
export function parseLarkError(e: unknown): LarkError;
export function getWikiNode(workspaceId: string, sourceId: string, nodeToken: string): Promise<WikiGetResult>;
```

Komponen: `DocPane.svelte` props `{ pane: Leaf; visible: boolean }` (TIDAK boleh berubah; kontrak `PaneKindSpec.component`), `MarkdownView.svelte` (doc) props `{ source: string }`, `SheetView.svelte` props `{ cells: string[][]; truncated: boolean }`.

## Langkah

### 1. `docClient.ts`

Berkas baru/timpa `frontend/src/modules/lark/doc/docClient.ts`. Impor `call` dari `'../../../lib/ipc'`. Jangan mengimpor `../client` atau `../board/*`.

- Ekspor tipe `LarkError` dan `WikiGetResult` persis seperti Kontrak.
- `parseLarkError(e: unknown): LarkError`: `const msg = e instanceof Error ? e.message : String(e)`; regex `/^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$/s`; cocok -> `{ code: m[1], message: m[2] || msg }`; tidak cocok -> `{ code: 'unknown', message: msg }`. Bila `e` sudah objek dengan `code` dan `message` string, kembalikan apa adanya.
- `getWikiNode(workspaceId, sourceId, nodeToken)`: `call<WikiGetResult>('lark.wiki.get', { workspaceId, sourceId, nodeToken })`; bila gagal `throw parseLarkError(e)` (melempar objek `LarkError`, bukan `Error`). Normalisasi hasil: `title ?? ''`, `objType ?? ''`, `url ?? ''`; `cells` dijadikan `[]` bila `objType === 'sheet'` dan null; `truncated` jadi boolean (`!!`).

### 2. `MarkdownView.svelte` (doc)

Pembungkus tipis. Isi:

```svelte
<script lang="ts">
  // Pembungkus atas renderer files yang sudah aman (tanpa HTML mentah, href dibatasi). Jangan tambah {@html}.
  import FilesMarkdownView from '../../files/viewer/MarkdownView.svelte';
  let { source }: { source: string } = $props();
</script>

<div class="doc-markdown">
  <FilesMarkdownView {source} />
</div>

<style>
  .doc-markdown { flex: 1; min-height: 0; min-width: 0; overflow: auto; }
</style>
```

Catatan: renderer files memakai `height: 100%` + `overflow: auto`; pembungkus harus punya tinggi dari flex induk (`flex: 1; min-height: 0`) agar scroll bekerja. Jangan mengubah berkas `files/*`. Markdown kosong (`''`) dirender kosong oleh renderer; keadaan "Dokumen kosong" ditangani `DocPane` (langkah 4).

### 3. `SheetView.svelte`

Props `{ cells: string[][]; truncated: boolean }` via `$props()`. Perilaku:

- `colCount = Math.max(0, ...cells.map(r => r.length))` (hitung aman untuk matriks kosong -> 0).
- Fungsi `colLabel(i: number): string` mengubah indeks 0-based ke label spreadsheet: 0->A, 25->Z, 26->AA, dst (basis-26 bijektif; maksimum praktis T pada 20 kolom, tetapi implementasikan umum).
- Bila `cells.length === 0`: tampilkan teks "Sheet kosong" (peran status) dan, bila `truncated`, tetap tampilkan catatan.
- Tabel `<table>` dalam wadah `overflow: auto` (kedua arah, `max-height: 100%`): `<thead>` dengan sel pojok kosong lalu `<th scope="col">` per kolom (A, B, C...); `<tbody>` baris dengan `<th scope="row">` nomor baris (1-based) lalu `<td>` sel. Baris yang lebih pendek dilengkapi sel kosong sampai `colCount`. Isi sel dirender sebagai teks biasa (`{cell}`, tanpa `{@html}`), `white-space: pre-wrap; overflow-wrap: anywhere; max-width: 32ch`.
- Header kolom dan nomor baris `position: sticky` (thead `top: 0`, kolom nomor `left: 0`) dengan latar `var(--bg-elev, ...)`.
- Bila `truncated`: catatan di bawah tabel (di luar area scroll): "Menampilkan 50 baris x 20 kolom pertama" (teks persis; angka literal sesuai konstanta backend).
- Gaya memakai token tema: `var(--fg)`, `var(--fg-muted)`, `var(--border)`, `var(--bg-elev)`, fallback seperti `files/viewer/MarkdownView.svelte`. Jangan hardcode warna terang/gelap.
- Aksesibilitas: `<table aria-label="Isi sheet">`.

### 4. `DocPane.svelte`

Timpa placeholder. Pertahankan persis signature `let { pane, visible }: { pane: Leaf; visible: boolean } = $props();` (`Leaf` dari `'../../layout/model'`). Impor: `get` dari `svelte/store`; `activeWorkspaceId` dari `'../../../lib/stores/workspaces'`; `onEvent` dari `'../../../lib/ipc'`; `copyLink`, `sendToAgent` dari `'../actions'`; `openSettings` dari `'../../settings'` (bila export tidak ada, buang tombol pengaturan dan laporkan); `getWikiNode`, `parseLarkError`, tipe dari `'./docClient'`; `MarkdownView` dari `'./MarkdownView.svelte'`; `SheetView` dari `'./SheetView.svelte'`.

State (rune Svelte 5):
- `type State = { status: 'loading' } | { status: 'error'; code: string; message: string } | { status: 'ready'; data: WikiGetResult }`; `let state = $state<State>({ status: 'loading' })`.
- `let seq = 0` (bukan reaktif) untuk stale guard.
- `const wsId = $derived($activeWorkspaceId)` (gunakan `$activeWorkspaceId` auto-subscribe di komponen, bukan `get`, agar reaktif saat workspace berganti), `const sourceId = $derived(pane.sourceId ?? '')`, `const nodeToken = $derived(pane.nodeToken ?? '')`.

Fungsi `load()`:
- `const my = ++seq`. Bila `wsId` kosong: `state = { status:'error', code:'no_workspace', message:'Tidak ada workspace aktif' }` dan return. Bila `sourceId` kosong atau `nodeToken` kosong: galat `invalid_params` dengan pesan "Dokumen ini belum punya referensi node" (kasus wiki sumber dengan `rootToken` kosong dibuka dari panel: node akar tidak punya token; tampilkan pesan ini, jangan memanggil IPC).
- `state = { status:'loading' }`; `try { const data = await getWikiNode(wsId, sourceId, nodeToken); if (my !== seq) return; state = { status:'ready', data }; } catch (x) { if (my !== seq) return; const e = parseLarkError(x); state = { status:'error', code: e.code, message: e.message }; }`. Seluruh `load` dibungkus try/catch; tidak ada galat keluar komponen.
- Saat segarkan (tombol), tampilkan isi lama dengan indikator memuat kecil bila `state.status === 'ready'` (jangan kosongkan; simpan `previous` di variabel lokal `$state` atau tandai `refreshing = $state(false)`). Saat ganti node/workspace: reset ke `loading` penuh.

Pemicu ambil ulang (tanpa polling):
- `$effect(() => { wsId; sourceId; nodeToken; load(); })` sehingga jalan saat mount (termasuk pemulihan dari persist, karena pane di-mount ulang dengan `pane` yang dipulihkan) dan saat `nodeToken`/`sourceId`/workspace berganti. Gunakan `untrack` untuk membaca `state` di dalam `load` agar efek tidak berulang.
- `$effect` kedua: `const off = onEvent('workspace.changed', (_t, p) => { if ((p as any)?.kind === 'configChanged' && state.status === 'error' && (state.code === 'source_not_found' || state.code === 'invalid_params')) load(); }); return off;` (hanya mencoba ulang saat sedang galat sumber; tidak memuat ulang dokumen yang sudah tampil).
- `visible` tidak memicu ambil ulang; boleh diabaikan (tetap di props demi kontrak). Jangan menambah timer.

Header (selalu tampil, `role="toolbar"`, `aria-label="Aksi dokumen"`):
- Judul: `state.status==='ready' ? data.title : (pane.title || 'Dokumen Lark')`, satu baris dengan elipsis, `title` atribut berisi teks penuh.
- Tombol "Segarkan" (ikon teks atau SVG inline kecil, `aria-label="Segarkan"`) memanggil `load()` (nonaktif saat `loading`).
- Tombol "Salin link": `onclick={() => copyLink(data.url)}`; **nonaktif bila `state.status !== 'ready'` atau `data.url === ''`**.
- Tombol "Kirim ke agen": `onclick={() => sendToAgent(data.url)}`; kondisi nonaktif sama.
- Bila `ready` dan `data.url === ''`: tampilkan keterangan kecil di bawah header (atau `title` tombol dan teks `fg-muted`): "Link tidak bisa dibuat (host belum diketahui). Tambah satu Base atau wiki dari link untuk menentukan host." (teks persis dari overview 1.3). Tombol nonaktif tetap memakai atribut `disabled` dan `title` yang sama.
- `copyLink`/`sendToAgent` dipanggil dengan `void ... .catch(() => {})` agar tidak ada penolakan tak tertangani.

Isi (di bawah header, `flex: 1; min-height: 0; display: flex; flex-direction: column`):
- `loading`: teks "Memuat dokumen..." (`role="status"`).
- `error`: kotak galat `role="alert"` dengan pesan terjemahan per `code`, plus tombol "Coba lagi" (memanggil `load()`):
  - `source_not_found`: "Sumber tidak ada di workspace ini" (tanpa tombol pengaturan; tombol "Coba lagi" tetap ada).
  - `cli_missing`: "lark-cli belum terpasang." + tombol "Buka Pengaturan" (`openSettings('connectors')`).
  - `not_logged_in`: "Belum masuk ke Lark." + tombol "Buka Pengaturan".
  - `permission`: "Izin Lark kurang" diikuti `message` dari backend (pesan CLI).
  - `timeout`: "Permintaan ke Lark melewati batas waktu."
  - `no_workspace`, `invalid_params`: tampilkan `message`.
  - lainnya (`cli_error`, `not_allowed`, `unknown`): "Gagal memuat dokumen." + `message`.
  - Untuk semua kode tampilkan juga `message` backend dalam baris kecil bila berbeda dari teks terjemahan (jangan duplikat).
- `ready`:
  - `objType === 'docx'`: bila `(data.markdown ?? '').trim() === ''` tampilkan "Dokumen kosong"; selain itu `<MarkdownView source={data.markdown ?? ''} />`.
  - `objType === 'sheet'`: `<SheetView cells={data.cells ?? []} truncated={!!data.truncated} />`.
  - lainnya: blok tengah dengan judul, keterangan "Tipe dokumen ini ({data.objType}) belum bisa ditampilkan di sini.", dan bila `data.url !== ''` tautan "Buka di Lark" `<a href={data.url} target="_blank" rel="noopener noreferrer" onclick={...}>` yang membuka lewat `(window as any).runtime?.BrowserOpenURL` bila tersedia (`e.preventDefault()` hanya bila fungsi ada, seperti `files/viewer/MarkdownView.svelte`); bila `url === ''` teks "Buka di Lark" tidak tampil sebagai tautan, tampilkan keterangan host belum diketahui di atas.
  - Jaga keamanan `href`: hanya render tautan bila `data.url` diawali `https://` atau `http://`.

Gaya: `<style>` biasa; root `.doc-pane { flex: 1; min-height: 0; min-width: 0; display: flex; flex-direction: column; color: var(--fg, inherit); background: var(--bg, transparent); }`; header `border-bottom: 1px solid var(--border, currentColor)` dengan padding 6px 10px, tombol kecil bergaya seperti `.icon` di `grimoire/GrimoirePanel.svelte` (hover `var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent))`, fokus `outline: 1px solid var(--accent, currentColor)`), galat memakai `var(--danger, inherit)`, teks sekunder `var(--fg-muted, inherit)`. Tombol nonaktif `opacity: 0.5; cursor: not-allowed`. Semua teks UI bahasa Indonesia.

### 5. `frontend/package.json`

Tidak diubah (overview 1.1). Jangan menjalankan `npm install` untuk paket baru. Bila saat implementasi ternyata renderer files tidak memadai (mis. impor lintas modul gagal build), jangan menambah `marked`/`dompurify` diam-diam: laporkan di akhir.

### 6. Verifikasi

Dari `frontend/`: `npm run build` dan `npm run check` (svelte-check) tanpa galat baru. Periksa `git status`: hanya berkas milik fitur ini yang berubah (`docClient.ts`, `MarkdownView.svelte`, `SheetView.svelte` baru; `DocPane.svelte` ditimpa). Pastikan `frontend/package.json` dan lockfile tidak muncul sebagai berubah. Jangan men-stage perubahan tak terkait.

## Perubahan

- Dibuat: `frontend/src/modules/lark/doc/docClient.ts`, `frontend/src/modules/lark/doc/MarkdownView.svelte`, `frontend/src/modules/lark/doc/SheetView.svelte`.
- Ditimpa: `frontend/src/modules/lark/doc/DocPane.svelte` (placeholder tiket 02 diganti implementasi penuh; props tetap).
- Tidak diubah: `frontend/package.json`.

## Wiring

Tidak ada yang disambungkan oleh fitur ini. Yang diekspos untuk langkah wiring (`lark-wiki-wiring`):
- `doc/DocPane.svelte` sudah terdaftar sebagai komponen pane `lark-doc` oleh tiket 02 (`registerPaneKind` di `modules/lark/index.ts`); karena props dan jalur berkas tetap, tidak ada edit registrasi.
- `WikiTree` (fitur lain) membuka pane lewat `layout.openPane({ kind: 'lark-doc', sourceId, nodeToken, title })`; `DocPane` membaca `pane.sourceId` dan `pane.nodeToken` dan memuat ulang saat `nodeToken` berganti.
- Bergantung pada daemon `lark.wiki.get` (fitur `lark-wiki-data`); sampai wiring selesai, pane menampilkan keadaan galat aman bila IPC belum ada.

## Selesai bila

- [ ] `cd frontend && npm run build` sukses; `npm run check` tanpa galat baru.
- [ ] `DocPane` menampilkan keadaan memuat, galat (dengan teks per kode: `source_not_found`, `cli_missing`, `not_logged_in`, `permission`, `timeout`, lainnya) dan tombol "Coba lagi"; tidak ada galat yang lolos keluar komponen.
- [ ] docx dirender lewat `doc/MarkdownView.svelte` -> `files/viewer/MarkdownView.svelte` (HTML mentah tidak dirender); tidak ada `{@html}` baru.
- [ ] sheet tampil sebagai tabel dengan header kolom A, B, C..., nomor baris, dan catatan "Menampilkan 50 baris x 20 kolom pertama" saat `truncated`.
- [ ] Tipe lain menampilkan judul + tautan "Buka di Lark" (hanya `http(s)`).
- [ ] Salin link dan Kirim ke agen memanggil `copyLink`/`sendToAgent` dari `../actions`; nonaktif bila `url` kosong, dengan keterangan host belum diketahui.
- [ ] Ambil ulang saat mount/pemulihan dan saat `nodeToken`/`sourceId`/workspace berganti; respons usang diabaikan lewat `seq`; tanpa polling.
- [ ] `frontend/package.json`, lockfile, dan semua path di luar ownedPaths tidak berubah.
- [ ] Selisih nama dari tiket 02 (bila ada) dan keputusan menyimpang dilaporkan di akhir pekerjaan.
