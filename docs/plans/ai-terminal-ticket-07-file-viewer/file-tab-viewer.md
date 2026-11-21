# Plan: Tab viewer markdown dan teks

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-07-file-viewer.md` (epic 07 File Viewer)
- Fitur: `file-tab-viewer` (Tab viewer markdown dan teks)
- ownedPaths: `frontend/src/modules/files/viewer/`
- Depends on foundation: ya. `00-foundation.md` harus sudah dikerjakan: `shared/types.ts` (`CHUNK_SIZE`), `shared/fsClient.ts` (`fsRead`, `FsBinaryError`, `FsNotFoundError`) sudah ada, dan `viewer/` berisi stub bertanda `// STUB(foundation): diganti oleh file-tab-viewer`. Bila `shared/` belum ada, berhenti dan laporkan sebagai blocker; jangan membuatnya.

## Tujuan

Menyediakan lapisan baca file untuk tab `file`:

1. Adaptor baca `fileApi.ts` (klasifikasi jenis, baca potongan, baca semua, kelas error) di atas `shared/fsClient`.
2. `MarkdownView.svelte` dengan renderer markdown buatan sendiri yang aman (tanpa dependency, tanpa `{@html}`/`innerHTML`).
3. `PlainTextView.svelte` dengan virtual scroll tinggi baris tetap, baca berjenjang, cache LRU potongan, DOM di bawah ~200 baris, scroll horizontal. File puluhan MB harus bisa dibuka dan digulir.
4. `BinaryNotice.svelte` untuk file biner.

Komponen dirangkai oleh `editor/FileTab.svelte` milik fitur lain; fitur ini tidak merangkai apa pun.

## Path yang dimiliki

HANYA `frontend/src/modules/files/viewer/` yang boleh dibuat/diubah. Jangan sentuh `shared/`, `tree/`, `editor/`, `index.ts`, `frontend/package.json`, lockfile, `vite.config.ts`, `tsconfig.json`, `frontend/src/lib/`, `frontend/src/styles/`, modul ui-shell, maupun kode Go. Bila perlu perubahan pada `shared/`, laporkan sebagai blocker. Jangan membuat `index.ts`. File sementara untuk verifikasi (bagian Langkah 7) harus dihapus sebelum selesai.

## Kontrak

Rujukan: `overview.md` bagian 2.2 (pesan `fs.*`), 2.4 (`shared/`), 2.5 (API internal). Ringkas hanya yang dipakai fitur ini.

### Dipakai (dari `shared/`, jangan diubah)

- `import { fsRead, FsBinaryError, FsNotFoundError } from '../shared/fsClient'`
  - `fsRead(absPath, offset?, limit?): Promise<ReadResultRaw>`; `limit <= 0` berarti sampai akhir file; param IPC bernama `limit`.
  - `ReadResultRaw = { content, mtime, kind, size, offset, length, eof }`. `length` adalah panjang byte potongan yang sebenarnya (disesuaikan ke batas rune UTF-8); offset berikutnya SELALU `offset + length` dari balasan, bukan dari permintaan.
- `import { CHUNK_SIZE } from '../shared/types'` (256 KiB).
- `import { basename } from '../shared/fsClient'` (untuk ekstensi dan tampilan nama).
- `mtime` adalah token opaque `number` (overview 2.3): hanya dibandingkan dengan `===`, tidak dikonversi.
- Dilarang memanggil `call('fs.*')` langsung; hanya lewat `fsClient`.

### Disediakan (signature dikunci; boleh ditambah, tidak boleh diubah)

```ts
// viewer/fileApi.ts
export type FileKind = 'markdown' | 'text';
export function fileKind(path: string): FileKind;   // .md .markdown .txt (case-insensitive) => 'markdown', selain itu 'text'
export interface Chunk { content: string; size: number; mtime: number; eof: boolean; offset: number; length: number }
export function readChunk(path: string, offset: number, length: number): Promise<Chunk>;
export function readAll(path: string): Promise<Chunk>;   // potongan 256 KiB sampai eof
export class BinaryFileError extends Error {}
export class FileNotFoundError extends Error {}
// tambahan kompatibel:
export const MARKDOWN_MAX_BYTES = 2 * 1024 * 1024;
export function isLargeForMarkdown(size: number): boolean;   // size > MARKDOWN_MAX_BYTES

// viewer/MarkdownView.svelte   props: { source: string }
// viewer/PlainTextView.svelte  props: { path: string; size: number }
// viewer/BinaryNotice.svelte   props: { path: string }
```

Tambahan internal (tidak dikonsumsi fitur lain): `viewer/markdown.ts` (parser murni, menghasilkan pohon node) dan `viewer/lineIndex.ts` (pembaca berjenjang untuk `PlainTextView`).

## Langkah

Semua komponen: Svelte 5 runes (`$props`, `$state`, `$derived`, `$effect`), TypeScript, impor relatif, teks UI Indonesia, identifier Inggris. Gaya: pakai CSS variable ui-shell bila ada (baca `frontend/src/styles/`, jangan ubah) dengan fallback `inherit`/`currentColor`; font monospace untuk teks polos dan kode. Tanpa `setInterval`/`setTimeout` berulang, tanpa polling.

### 1. `viewer/fileApi.ts` (timpa stub)

- Hapus komentar STUB. Impor dari `../shared/fsClient` dan `../shared/types`.
- `fileKind(path)`: ambil `basename(path)`, ekstensi = bagian dari titik terakhir (`lastIndexOf('.')`, huruf kecil). `.md`, `.markdown`, `.txt` => `'markdown'`; selain itu (termasuk tanpa titik) `'text'`.
- `export class BinaryFileError extends Error {}` dan `FileNotFoundError` (set `this.name`).
- fungsi privat `mapError(e: unknown): never`: `FsBinaryError` => `new BinaryFileError(e.message)`; `FsNotFoundError` => `new FileNotFoundError(e.message)`; lainnya `throw e` apa adanya.
- `readChunk(path, offset, length)`: `try { const r = await fsRead(path, offset, length); return { content: r.content, size: r.size, mtime: r.mtime, eof: r.eof, offset: r.offset, length: r.length }; } catch (e) { mapError(e) }`. `length <= 0` diteruskan apa adanya (artinya sampai akhir).
- `readAll(path)`:
  - Loop: `offset = 0`; ulangi `readChunk(path, offset, CHUNK_SIZE)`, kumpulkan `content` ke array `parts`, `offset = c.offset + c.length`, berhenti saat `c.eof`.
  - Penjaga loop tak berujung: bila `c.length === 0 && !c.eof` lempar `Error('Pembacaan file tidak maju')`.
  - Konsistensi: simpan `firstMtime` dari potongan pertama. Bila potongan berikutnya `mtime !== firstMtime` (file berubah saat dibaca), ulang dari awal SATU kali; bila masih berubah, lempar `Error('File berubah saat dibaca, coba lagi')`.
  - Hasil: `{ content: parts.join(''), size: <size potongan terakhir>, mtime: <mtime potongan terakhir>, eof: true, offset: 0, length: <jumlah length semua potongan> }`.
- `MARKDOWN_MAX_BYTES` dan `isLargeForMarkdown(size)` seperti kontrak.

### 2. `viewer/BinaryNotice.svelte` (timpa stub)

Props `{ path: string }`. Tampilkan blok pesan (`role="status"`): judul "File biner tidak dapat ditampilkan." dan baris kecil monospace berisi `path` (teks biasa, bukan HTML). Tanpa tombol sunting. Tidak ada logika lain.

### 3. `viewer/markdown.ts` (baru, parser murni, tanpa DOM)

Tujuan: mengubah `string` menjadi pohon node bertipe; `MarkdownView` merender pohon dengan Svelte sehingga SEMUA teks lewat penghalusan Svelte (otomatis di-escape `& < > " '`) dan tidak ada `{@html}`/`innerHTML` sama sekali. HTML mentah di sumber diperlakukan sebagai teks literal.

Tipe yang diekspor:

```ts
export type Inline =
  | { t: 'text'; v: string }
  | { t: 'code'; v: string }
  | { t: 'strong'; c: Inline[] }
  | { t: 'em'; c: Inline[] }
  | { t: 'del'; c: Inline[] }
  | { t: 'link'; href: string; c: Inline[] }   // href SUDAH lolos isSafeHref
  | { t: 'br' };
export type Block =
  | { t: 'heading'; level: 1|2|3|4|5|6; c: Inline[] }
  | { t: 'para'; c: Inline[] }
  | { t: 'code'; lang: string; v: string }
  | { t: 'quote'; c: Block[] }
  | { t: 'list'; ordered: boolean; start: number; items: Block[][] }
  | { t: 'hr' }
  | { t: 'table'; head: Inline[][]; align: ('left'|'right'|'center'|null)[]; rows: Inline[][][] };
export function parseMarkdown(source: string): Block[];
export function isSafeHref(raw: string): boolean;
```

Aturan:
- Normalisasi baris: ganti `\r\n`/`\r` dengan `\n`, `split('\n')`.
- `isSafeHref(raw)`: `trim()`, buang karakter kontrol dan spasi putih di dalam (`/[\u0000-\u001f\u007f\s]/g`) sebelum menguji, lalu hanya terima bila `/^(https?:|mailto:)/i` cocok. Skema lain (`javascript:`, `data:`, `vbscript:`, `file:`, URL relatif, `//host`) ditolak. Jika ditolak, link dirender sebagai teks biasa (`text` node berisi teks tautan), tanpa tautan aktif.
- Blok (urutan prioritas per baris): baris kosong (pemisah); blok kode berpagar (` ``` ` atau `~~~`, diakhiri pagar sama atau akhir dokumen; isi literal, `lang` = kata pertama info string); heading ATX `#`..`######` diikuti spasi; garis pemisah (`---`, `***`, `___` dengan >=3 karakter, spasi diizinkan); kutipan (`>`: kumpulkan baris berurutan, buang awalan `> ?`, parse rekursif); daftar (`-`, `*`, `+`, atau `\d+[.)]` diikuti spasi; item = baris awal + baris berikutnya yang menjorok >= lebar penanda atau lanjutan paragraf; isi item di-parse rekursif; daftar bersarang lewat rekursi; `start` diambil dari angka pertama daftar berurutan); tabel (baris header berisi `|` diikuti baris pemisah `|---|:--:|`; sel dipecah di `|` dengan mengabaikan `|` paling luar; jumlah kolom dari header); selain itu paragraf (gabungkan baris berurutan sampai baris kosong atau awal blok lain; dua spasi di akhir baris atau `\` menghasilkan `br`, baris lain digabung dengan spasi).
- Batas rekursi: parameter `depth`, maksimum 8 (kutipan/daftar); di atas itu isi dirender sebagai paragraf teks literal. Tidak ada regex dengan backtracking katastrofik: parse inline dengan scanner karakter, bukan regex bersarang.
- Inline (`parseInline(text, depth)`, scanner dari kiri ke kanan): `\` + karakter tanda baca => karakter literal; span kode dengan backtick (panjang pagar sama, isi literal, pangkas satu spasi di tepi bila ada di kedua sisi); `**x**`/`__x__` => strong; `*x*`/`_x_` => em (garis bawah hanya bila di batas kata agar `snake_case_name` tidak menjadi miring); `~~x~~` => del; `[teks](url "judul")` => link (url = sampai `)` pertama yang seimbang kurung, buang judul; teks di-parse rekursif dengan batas kedalaman 4); `![alt](url)` => TIDAK ada gambar dan tidak ada permintaan jaringan: render sebagai `text` berisi alt (bila kosong, `[gambar]`), dan bila `isSafeHref(url)` benar, bungkus sebagai `link` ke url dengan teks `alt` agar pengguna bisa membukanya sendiri; autolink `<https://...>` dan URL telanjang `http(s)://...` boleh menjadi `link` (setelah `isSafeHref`); semua lainnya `text`. Penanda yang tidak punya pasangan tetap menjadi teks literal.
- Kinerja: satu lintasan per blok; tidak ada loop yang tidak maju (setiap iterasi memajukan indeks minimal 1).

### 4. `viewer/MarkdownView.svelte` (timpa stub)

- Props `{ source: string }`. `const blocks = $derived(parseMarkdown(source));`.
- Render rekursif memakai `{#snippet blockView(b: Block)}` dan `{#snippet inlineView(nodes: Inline[])}` yang memanggil dirinya sendiri (`{@render ...}`) dan `{#each}`/`{#if b.t === ...}`. Peta elemen: `h1`..`h6`, `p`, `pre><code` (blok kode, `overflow-x:auto`), `blockquote`, `ul`/`ol` (`start` untuk `ol`) + `li`, `hr`, `table` (`thead`/`tbody`, `text-align` dari `align` lewat `style` terkontrol atau kelas), `strong`, `em`, `del`, `code`, `br`, `a`.
- Tautan: `<a href={n.href} target="_blank" rel="noopener noreferrer" onclick={openExternal}>`. `openExternal(e, href)`: bila `(window as any).runtime?.BrowserOpenURL` fungsi, `e.preventDefault()` lalu panggil dengan `href`; selain itu biarkan perilaku bawaan. Karena `href` hanya berasal dari `isSafeHref`, tidak ada skema berbahaya.
- Tanpa `{@html}`, tanpa `innerHTML`, tanpa `<img>`. Kontainer `div.markdown` dengan `overflow:auto`, `max-width` wajar, `word-break` aman untuk teks panjang; gaya lokal (`<style>`) memakai CSS variable ui-shell dengan fallback. Kontainer menggulir sendiri (`height:100%`).
- Komponen TIDAK memutuskan ambang file besar; `FileTab` memakai `isLargeForMarkdown(size)` dan merender `PlainTextView` bila perlu (banner "file besar, ditampilkan sebagai teks" juga dirender `FileTab`).

### 5. `viewer/lineIndex.ts` (baru) dan `viewer/PlainTextView.svelte` (timpa stub)

#### 5a. `lineIndex.ts`: pembaca berjenjang

Kelas `LineReader` (tanpa DOM, tanpa Svelte) yang dipakai komponen:

```ts
export interface ChunkMeta { index: number; offset: number; byteLen: number; firstLine: number; lineCount: number }
export class LineReader {
  constructor(path: string, size: number, opts?: { maxCached?: number; maxRows?: number });
  readonly chunks: ChunkMeta[];          // indeks, berurutan, hanya metadata (kecil)
  get indexedLines(): number;            // jumlah baris yang sudah terindeks
  get complete(): boolean;               // true bila eof tercapai
  get totalLines(): number;              // = indexedLines bila complete, selain itu perkiraan
  get maxCols(): number;                 // kolom terlebar yang pernah terlihat (dibatasi 10000)
  get mtime(): number | null;            // mtime potongan pertama
  stale: boolean;                        // true bila mtime potongan berikutnya berbeda
  ensureIndexed(row: number): Promise<void>;     // baca berurutan sampai baris `row` terindeks atau eof
  getLines(from: number, to: number): Promise<string[] | null>;  // baris [from,to); null bila ada potongan belum tersedia
  peekLines(from: number, to: number): (string | undefined)[];   // sinkron dari cache, undefined bila tidak ada
}
```

Algoritma (kunci agar benar):
- Konstanta: `ROW_BUDGET`: `maxRows` bawaan 500_000 (membatasi tinggi spacer <= ~9 juta px agar di bawah batas tinggi elemen browser; bila terlampaui, indeks berhenti, `truncated = true`, dan komponen menampilkan catatan "Hanya 500.000 baris pertama ditampilkan"). `maxCached` bawaan 6 potongan (sekitar 1,5 MiB teks mentah per potongan, jadi memori terbatas). Panjang baris yang ditampilkan dipotong di 10000 karakter (sisanya diganti `…`) agar satu baris minifikasi tidak merusak lebar.
- Potongan berurutan dan SELALU berawal di awal baris. Potongan ke-i dibaca dengan `readChunk(path, offset_i, CHUNK_SIZE)`. Dari `content` ambil bagian sampai `\n` terakhir (inklusif) sebagai baris-baris lengkap; sisa setelah `\n` terakhir adalah baris parsial yang TIDAK dikonsumsi (akan dibaca ulang di awal potongan berikutnya). Bila `eof`, konsumsi seluruh isi (baris terakhir tanpa `\n` ikut dihitung; bila isi berakhir dengan `\n`, tidak ada baris kosong tambahan). Bila tidak ada `\n` sama sekali dan bukan `eof` (baris raksasa), konsumsi seluruh potongan sebagai satu baris pecahan agar tidak macet.
- `byteLen` = jumlah byte UTF-8 teks yang dikonsumsi, dihitung `new TextEncoder().encode(consumed).length` (atau penghitung manual tanpa alokasi besar). Offset potongan berikutnya = `offset_i + byteLen`. (Offset selalu pada batas rune karena berakhir di `\n` atau akhir potongan yang sudah disesuaikan oleh daemon.)
- Pisahkan baris dengan `\n`, buang `\r` di akhir tiap baris (CRLF). Simpan `firstLine`, `lineCount` ke `chunks`. Isi baris (`string[]`) masuk cache LRU `Map<number, string[]>` (urutan sisip Map = urutan pakai; saat dipakai, `delete` lalu `set` ulang; saat ukuran > `maxCached`, hapus kunci pertama). Potongan yang sedang terlihat jangan dibuang: `getLines` menandai potongan yang dipakai sebagai terbaru.
- Cache miss (potongan tua dibuang lalu digulir kembali): baca ulang dari `offset` meta dengan `readChunk(path, meta.offset, meta.byteLen)` dan pecah dengan aturan yang sama (karena `byteLen` berakhir di batas baris, hasilnya identik). Gunakan peta `inflight: Map<number, Promise<string[]>>` untuk deduplikasi.
- `ensureIndexed(row)`: dengan satu mutex (promise berantai) agar hanya satu pembacaan berurutan berjalan; baca potongan berikutnya berulang sampai `indexedLines > row` atau `complete` atau `truncated`. Loncatan jauh ke bawah membaca potongan perantara berurutan (offset berurutan, sesuai kriteria) dan hanya menyimpan metadata plus cache terbatas; ini kompromi yang disengaja (tidak ada byte-offset acak karena nomor baris butuh indeks berurutan). Pembacaan hanya dipicu dari gulir/mount, tidak ada pekerjaan latar atau timer.
- `totalLines` perkiraan saat belum lengkap: `max(indexedLines, ceil(size / avgBytesPerLine))`, `avgBytesPerLine = bytesIndexed / indexedLines` (minimum 1); dibatasi `maxRows`.
- Deteksi perubahan: simpan `mtime` potongan pertama; bila potongan lain memiliki `mtime` berbeda, set `stale = true` (komponen menampilkan catatan "File berubah saat dibaca; muat ulang tab untuk melihat isi terbaru"), hentikan pengindeksan lanjutan, jangan melempar.
- Error dari `readChunk` (`BinaryFileError`, `FileNotFoundError`, lainnya) dilempar dari `ensureIndexed`/`getLines` agar komponen menampilkannya.

#### 5b. `PlainTextView.svelte`

- Props `{ path: string; size: number }`. Konstanta: `ROW_HEIGHT = 18` (px, harus sama dengan `line-height` CSS), `OVERSCAN = 20`, `MAX_DOM_ROWS = 190`.
- Struktur DOM: `div.viewport` (`overflow:auto`, `height:100%`, `position:relative`, `onscroll`, `bind:clientHeight={viewH}`, `tabindex="0"`) berisi `div.spacer` (`height: totalRows*ROW_HEIGHT px`, `width: calc(var(--cols) * 1ch + gutterWidth)`, `position:relative`). Baris-baris terlihat dirender `position:absolute; top: row*ROW_HEIGHT; height: ROW_HEIGHT; white-space:pre` dalam `{#each visibleRows as r (r.row)}`. Gutter nomor baris memakai `position:sticky; left:0` di tiap baris (lebar `ch` dari jumlah digit `totalLines`). Teks tidak dibungkus; scroll horizontal otomatis dari lebar spacer.
- State: `scrollTop` (`$state`, diperbarui di `onscroll`), `viewH`, `reader` (`$state.raw`), `rows` (peta `row -> string` untuk jendela saat ini), `error` (`unknown | null`), `loading`.
- Jendela: `first = max(0, floor(scrollTop/ROW_HEIGHT) - OVERSCAN)`, `last = min(totalRows, ceil((scrollTop+viewH)/ROW_HEIGHT) + OVERSCAN)`, lalu klem `last - first <= MAX_DOM_ROWS` (jika viewport sangat tinggi, kurangi overscan; jumlah node baris harus selalu < 200).
- `$effect` reaktif terhadap `first`/`last`/`reader`: (1) `await reader.ensureIndexed(last)`; (2) `const lines = await reader.getLines(first, min(last, reader.indexedLines))`; (3) simpan ke `rows` hanya bila jendela belum berubah (token generasi, buang hasil basi). Selama menunggu tampilkan baris yang tersedia dari `reader.peekLines` dan sisanya sebagai placeholder `…` (tinggi tetap). Total tinggi spacer memakai `reader.totalLines` (perkiraan sampai lengkap); setelah `complete`, tinggi menyesuaikan dan posisi gulir dijaga (klem `scrollTop`).
- `$effect` pada perubahan `path`/`size`: buat `LineReader` baru, reset `scrollTop`/`viewport.scrollTop = 0`, `error = null`; baca potongan pertama. Fungsi pembersih menaikkan token generasi (membatalkan hasil lama). Komponen tidak menyimpan referensi besar setelah dihancurkan.
- File kosong (`size === 0` atau 0 baris): tampilkan "(file kosong)".
- Error: `BinaryFileError` => render `<BinaryNotice {path} />` (impor `./BinaryNotice.svelte`); `FileNotFoundError` => pesan "File tidak ditemukan." plus path; error lain => pesan inline dengan tombol "Coba lagi" yang membuat ulang `LineReader`. Tidak ada throw ke konsol saja.
- Footer kecil (opsional, satu baris) menampilkan "Baris N dari ~M" bila perkiraan, dan catatan `truncated`/`stale` dari `reader`.
- A11y dasar: `role="region"` dengan `aria-label={"Isi file " + basename(path)}`, fokusable agar bisa digulir dengan keyboard. Pilih/salin teks bekerja untuk baris yang ada di DOM (batasan wajar virtual scroll).
- Tanpa `{@html}`: baris dirender sebagai `{line}` biasa.

### 6. Rapikan stub

- Pastikan tidak ada `STUB(foundation)` tersisa di `viewer/`: `grep -rn "STUB(foundation)" frontend/src/modules/files/viewer/` harus kosong.
- Pastikan tidak ada `innerHTML`, `{@html}`, `setInterval`, `call('fs.` di `viewer/`.

### 7. Verifikasi build

`npm run build` saja tidak cukup: tanpa `files/index.ts` Vite tidak pernah mengimpor `viewer/`, jadi file tidak ikut dikompilasi. Lakukan:

1. `cd frontend && npm run check` bila skrip ada (svelte-check; abaikan kegagalan lama di luar `viewer/`, perbaiki semua di dalam `viewer/`).
2. Buat file sementara `frontend/src/modules/files/viewer/_probe.ts` yang mengimpor semua modul: `import './fileApi'; import './markdown'; import './lineIndex'; import MarkdownView from './MarkdownView.svelte'; import PlainTextView from './PlainTextView.svelte'; import BinaryNotice from './BinaryNotice.svelte'; export { MarkdownView, PlainTextView, BinaryNotice };`. Lalu jalankan kompilasi tanpa mengubah konfigurasi: `cd frontend && npx vite build --ssr src/modules/files/viewer/_probe.ts --outDir "$(mktemp -d)"` (keluaran ke direktori sementara, jangan ke `dist`). Harus sukses.
3. HAPUS `_probe.ts` setelah verifikasi.
4. `cd frontend && npm run build` harus lolos; `git status` hanya menampilkan perubahan di `frontend/src/modules/files/viewer/` (lockfile dan `package.json` tidak berubah; bila `npm install` mengubah lockfile, kembalikan).
5. Tinjauan manual terhadap parser (tanpa unit test; telusuri kode atau jalankan cuplikan sementara di luar repo): masukan `<script>alert(1)</script>`, `[x](javascript:alert(1))`, `[x](  JaVaScRiPt:alert(1))`, `![a](http://evil/x.png)`, `**tebal** dan _miring_`, blok kode, daftar bersarang, tabel, serta masukan jahat berulang (`*`.repeat(10000), `[`.repeat(10000), `>`.repeat(1000)) tidak menggantung dan tidak menghasilkan tautan/elemen berbahaya.

## Perubahan

Dibuat/diubah, semuanya di `frontend/src/modules/files/viewer/`:

- `fileApi.ts` (ubah: timpa stub)
- `BinaryNotice.svelte` (ubah: timpa stub)
- `MarkdownView.svelte` (ubah: timpa stub)
- `PlainTextView.svelte` (ubah: timpa stub)
- `markdown.ts` (baru)
- `lineIndex.ts` (baru)
- `_probe.ts` (sementara, dihapus sebelum selesai; TIDAK boleh tersisa)

Tidak ada perubahan di tempat lain.

## Wiring

Yang diekspos untuk disambungkan langkah wiring / `file-editor-save` (fitur ini tidak menyambungkannya sendiri, tidak membuat `index.ts`):

- `editor/FileTab.svelte` mengimpor: `fileKind`, `readAll`, `readChunk`, `BinaryFileError`, `FileNotFoundError`, `isLargeForMarkdown` dari `../viewer/fileApi`; `MarkdownView` (props `source`), `PlainTextView` (props `path`, `size`), `BinaryNotice` (props `path`).
- Pembagian tugas: `.md/.markdown/.txt` => `readAll` lalu `MarkdownView`, kecuali `isLargeForMarkdown(size)` => `PlainTextView` dengan banner "file besar, ditampilkan sebagai teks" (dirender `FileTab`). Jenis lain => `PlainTextView` langsung (tanpa `readAll`). `FileTab` bertugas menangkap `BinaryFileError`/`FileNotFoundError` dari `readAll`; `PlainTextView` menangani errornya sendiri untuk pembacaan berjenjang.
- Komponen viewer harus diberi tinggi oleh induknya (kontainer `FileTab` harus punya tinggi terbatas, `min-height:0`) agar virtual scroll dan scroll internal bekerja.
- `index.ts` modul tetap milik `file-editor-save`; tidak ada registrasi di fitur ini. Wiring checklist overview bagian 5 butir 1 (`STUB(foundation)` kosong) mensyaratkan stub `viewer/` sudah tergantikan.

## Selesai bila

- [ ] `fileKind` mengklasifikasi `.md`, `.markdown`, `.txt` (tanpa memandang huruf besar/kecil) sebagai `markdown`, selainnya `text`.
- [ ] `readChunk`/`readAll` hanya memakai `shared/fsClient` (`fsRead`), memetakan `FsBinaryError` ke `BinaryFileError` dan `FsNotFoundError` ke `FileNotFoundError`; `readAll` membaca 256 KiB per potongan, maju dengan `offset + length`, aman dari loop tak berujung dan dari file yang berubah saat dibaca.
- [ ] `MarkdownView` mendukung heading, paragraf, tebal/miring, kode inline, blok kode berpagar, daftar berurutan/tak berurutan (bersarang), kutipan, garis pemisah, tautan, tabel sederhana; `<script>`/HTML di sumber tampil sebagai teks; `javascript:` dan skema selain http/https/mailto tidak menjadi tautan; tidak ada `<img>`/permintaan jaringan otomatis; tidak ada `{@html}`/`innerHTML`.
- [ ] `PlainTextView` menggulir file puluhan MB dengan jumlah node baris di DOM < ~200, tinggi baris tetap, scroll horizontal, pembacaan potongan berurutan hanya saat dibutuhkan gulir, cache LRU potongan terbatas, error ditampilkan inline (biner memakai `BinaryNotice`).
- [ ] `BinaryNotice` menampilkan pesan dan path tanpa tombol sunting.
- [ ] Tidak ada dependency baru, tidak ada `setInterval`/polling, tidak ada `call('fs.` langsung, tidak ada `STUB(foundation)` tersisa di `viewer/`.
- [ ] Guard: kompilasi probe SSR sukses (Langkah 7.2), `_probe.ts` sudah dihapus, `cd frontend && npm run build` lolos, dan `git status` hanya menunjukkan path di `frontend/src/modules/files/viewer/`. Tanpa unit test.
