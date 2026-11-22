# Plan: DiffView dan navigasi dari daftar

## Metadata

- Tiket: `agent-diff-review-ticket-02-review-diff` (`docs/tickets/agent-diff-review-ticket-02-review-diff.md`)
- Fitur: `review-diff-ui`
- ownedPaths:
  - `frontend/src/modules/review/diff.ts` (baru)
  - `frontend/src/modules/review/DiffView.svelte` (baru)
  - `frontend/src/modules/review/ReviewTab.svelte` (milik tiket 01; hanya penyisipan)
- Depends on foundation: tidak (tanpa `00-foundation.md`; kontrak ada di `overview.md` bagian 2)

## Tujuan

Pengguna bisa membuka unified diff per file dari tab Review: baris daftar menampilkan `+N -M` dan penanda biner/terlalu besar, klik baris menampilkan `DiffView` menggantikan daftar (dengan tombol kembali, path, tombol "Buka file"), tombol ikon "Buka file" di baris daftar membuka file di pane yang ada. `DiffView` menyediakan titik ekstensi (`hunkActions`, `footer`, `onLineClick`) untuk tiket 03/04, dibiarkan kosong di sini. Backend `review.diff` dikerjakan agent lain; fitur ini hanya mengonsumsi kontrak JSON-nya.

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `frontend/src/modules/review/diff.ts`
- `frontend/src/modules/review/DiffView.svelte`
- `frontend/src/modules/review/ReviewTab.svelte` (hanya penyisipan: ringkasan, tombol buka file, DiffView, state terkait)

Dilarang: `frontend/src/modules/review/{store,types,index}.ts`, `ReviewBadge.svelte`, modul `shell`/`files`/`layout`, `lib/*`, `package.json`, semua file Go (`internal/review/*`, `cmd/shikigamid/*`), dan file wiring. Jangan `git commit`, jangan tambah dependency npm, jangan tulis unit test.

## Kontrak

Mengikuti `overview.md` bagian 2.4 (diff.ts), 2.5 (DiffView), 2.6 (ReviewTab). Ringkas yang konkret:

Disediakan (nama tidak boleh berubah; dipakai tiket 03/04):
- `diff.ts`: `DiffLineKind`, `DiffLine`, `Hunk`, `DiffResult`, `fetchDiff(sessionId, path): Promise<DiffResult>`, `hunkSummary(hunk): string`.
- `DiffView.svelte` props: `{ diff: DiffResult; path: string; onLineClick?: (line: DiffLine, hunk: Hunk) => void; hunkActions?: Snippet<[Hunk]>; footer?: Snippet }`. Nama slot publik tetap `hunk-actions` dan `footer` (tulis di komentar kepala komponen).

Dipakai:
- IPC `review.diff` params `{sessionId, path}` -> `{hunks, currentHash, binary, tooLarge}` (`hunks` bisa saja null/hilang pada server lama; normalisasi defensif). Sampai backend siap, panggilan bisa gagal; UI harus menampilkan error inline, bukan melempar.
- `call` dari `../../lib/ipc` (`call<T>(method, params)`).
- `reviewFiles` store (`./store`, read-only), tipe `ReviewFile` (`./types`: `path,status,additions,deletions,reviewed,external,binary,tooLarge`).
- `openFilePane({projectId, filePath, workspaceId?, title?})` dari `../shell/helpers` (`filePath` absolut; tanpa nomor baris, jadi buka file saja).
- `layout` dari `../../lib/stores/layout`, `sessions` dari `../../lib/stores/sessions`, `activeTab`/`findLeaf` dari `../layout/model`, tipe `Session` dari `../../lib/stores/types` (`id, projectId, cwd, workspaceId`).
- Token CSS yang sudah dipakai ReviewTab: `--fg`, `--fg-muted`, `--border`, `--bg-hover`, `--accent`, `--ok`, `--warn`, `--danger` (dengan fallback seperti di file itu). Tanpa palet baru.

## Langkah

### 1. `diff.ts`

Buat file baru:
- Ekspor tipe persis seperti overview 2.4.
- `fetchDiff(sessionId, path)`: `const raw = await call<unknown>('review.diff', { sessionId, path })`, lalu `diffNormalize(raw)`. Biarkan error dari `call` merambat (ReviewTab yang menangkap).
- `diffNormalize(raw: unknown): DiffResult` (privat, awalan `diff`): objek non-object -> `{hunks: [], currentHash: '', binary: false, tooLarge: false}`. `hunks` = `Array.isArray` ? map `diffNormalizeHunk` : `[]`. Hunk: `id` string (fallback `String(index)`), empat angka via `typeof === 'number' ? v : 0`, `lines` array disaring dan dinormalisasi (`kind` harus `'ctx'|'add'|'del'` selain itu buang baris; `text` string, `oldNo`/`newNo` angka fallback 0). `currentHash` string, `binary`/`tooLarge` `=== true`.
- `hunkSummary(hunk)`: hitung `add` dan `del` dari `lines`, kembalikan `"+${add} -${del}"`.
- Tidak ada state global.

### 2. `DiffView.svelte`

Buat file Svelte 5 runes (`<script lang="ts">`, `$props()`, import `type { Snippet } from 'svelte'`, tipe dari `./diff`). Komentar kepala (Indonesia) menjelaskan kontrak ekstensi dan nama slot publik `hunk-actions`/`footer`.

Perilaku:
- Status (urutan prioritas): `diff.binary` -> "File biner, tidak ada diff"; `diff.tooLarge` -> "File terlalu besar (> 1 MB), diff tidak ditampilkan"; `diff.hunks.length === 0` -> "Tidak ada perubahan terhadap baseline". Status ditampilkan sebagai blok pesan terpusat (gaya `.empty` ReviewTab). `footer` tetap dirender di bawah pesan bila disediakan.
- Bila ada hunk: wadah `.diff` (`overflow: auto`, `min-height: 0`, `flex: 1`) berisi per hunk (`{#each diff.hunks as h (h.id)}`; bila id bisa duplikat, key `h.id + ':' + i`):
  - Header `.hunk-head`: kiri `@@ -{oldStart},{oldLines} +{newStart},{newLines} @@` (monospace, `--fg-muted`) dan `hunkSummary`; kanan `{#if hunkActions}{@render hunkActions(h)}{/if}` di dalam `<span class="actions">` yang HANYA dirender bila `hunkActions` ada (tanpa pembungkus kosong yang mengubah tampilan).
  - Baris datar: `{#each h.lines as ln}` (tanpa komponen anak per baris). Tiap baris satu elemen grid/flex: kolom nomor lama (`ln.oldNo || ''`), nomor baru (`ln.newNo || ''`), penanda (`+`, `-`, atau spasi), teks. `white-space: pre`, `font-family` monospace (ikuti font mono yang dipakai FilesTab/viewer bila ada variabel; bila tidak, `ui-monospace, SFMono-Regular, Menlo, monospace`), `font-size: 12px`.
  - Lebar blok: gunakan `min-width: max-content` pada baris agar latar warna meluas penuh saat scroll horizontal; scroll horizontal ada di `.diff`, bukan di halaman.
  - Warna: `.add` latar `color-mix(in srgb, var(--ok, #16a34a) 15%, transparent)`, `.del` latar `color-mix(in srgb, var(--danger, #dc2626) 15%, transparent)`, penanda +/- berwarna `--ok`/`--danger`; nomor baris `--fg-muted`; `.ctx` tanpa latar. Header hunk latar `color-mix(in srgb, var(--accent, currentColor) 8%, transparent)` dan `border-bottom: 1px solid var(--border, ...)`. Harus terbaca di tema gelap dan terang (hanya token + transparansi).
  - `onLineClick`: bila diberikan, baris (atau minimal nomor baris) mendapat `class:clickable`, `cursor: pointer`, `role="button"`, `tabindex="0"`, handler `onclick` dan `onkeydown` (Enter/Space) yang memanggil `onLineClick(ln, h)`. Bila tidak diberikan, tanpa role/tabindex/kursor pointer (elemen biasa).
- Setelah semua hunk: `{#if footer}{@render footer()}{/if}`.
- Prop `path` dipakai untuk `aria-label` wadah diff (mis. `Diff {path}`).

### 3. `ReviewTab.svelte` (hanya penyisipan; jangan ubah logika tiket 01)

Pertahankan semua markup/logika/gaya yang ada (`onlyUnreviewed`, `selected`, `$effect` penyegar, `pick`, `onselect`). Tambahkan:

Impor baru: `onMount`/`onDestroy` bila perlu; `get` atau auto-subscribe `$layout`/`$sessions`; `layout` dari `../../lib/stores/layout`; `sessions` dari `../../lib/stores/sessions`; `activeTab`, `findLeaf` dari `../layout/model`; `openFilePane` dari `../shell/helpers`; `fetchDiff`, `type DiffResult` dari `./diff`; `DiffView from './DiffView.svelte'`.

a) Sesi aktif lokal (karena `store.ts` tidak mengekspor id sesi aktif; jangan ubah `store.ts`):
- `let sessionId = $state<string | null>(null)`.
- `$effect` yang membaca `$layout` dan meniru `reviewActiveFromLayout` di `store.ts`: `doc.root === null` -> `null`; fokus leaf -> `activeTab(leaf)`; bila `tab.kind === 'terminal' && tab.sessionId` -> id itu; selain itu pertahankan nilai sebelumnya. (Baca `store.ts` dan `../layout/model` untuk nama aktual; ikuti kode bila berbeda.) Set `sessionId` hanya bila berubah.
- `const session = $derived($sessions.find((s) => s.id === sessionId) ?? null)` untuk `projectId`, `cwd`, `workspaceId`. (Cek bentuk store `sessions`: bila berupa array pakai `find`; ikuti bentuk aktual.)

b) State diff:
- `let selectedPath = $state<string | null>(null)`; `let diffData = $state<DiffResult | null>(null)`; `let diffLoading = $state(false)`; `let diffError = $state<string | null>(null)`; `let diffSeq = 0` (non-reaktif); `let listEl: HTMLUListElement | undefined = $state()`; `let savedScroll = 0`.
- `openDiff(f: ReviewFile)`: simpan `savedScroll = listEl?.scrollTop ?? 0`, set `selectedPath = f.path`, `diffData = null`, lalu `loadDiff()`. Tetap panggil `pick(f)` (agar `selected` dan `onselect` tiket 01 tetap jalan). Ganti `onclick={() => pick(f)}` pada tombol baris menjadi `openDiff(f)` (perubahan minimal; `openDiff` memanggil `pick`).
- `loadDiff(opts?: { silent?: boolean })`: bila `!sessionId || !selectedPath` return; `const seq = ++diffSeq; const sid = sessionId; const path = selectedPath;` bila bukan silent dan `diffData === null` set `diffLoading = true`; `diffError = null`; `try { const d = await fetchDiff(sid, path); if (seq !== diffSeq) return; diffData = d; } catch (e) { if (seq !== diffSeq) return; diffError = e instanceof Error ? e.message : String(e); } finally { if (seq === diffSeq) diffLoading = false; }`. Respons basi (seq berbeda) diabaikan seluruhnya.
- `backToList()`: `diffSeq++`, `selectedPath = null`, `diffData = null`, `diffError = null`, `diffLoading = false`. Setelah DOM kembali tampil, pulihkan `listEl.scrollTop = savedScroll` lewat `tick()` / `requestAnimationFrame` (dan karena daftar tidak di-unmount, scroll praktis terjaga).
- Refetch saat daftar berubah: `$effect` yang bergantung pada `result.files` (objek ber-referensi baru tiap `review.files` selesai) dan `selectedPath`: 
  - bila `selectedPath` ada dan file tidak ada lagi di `result.files` (atau `!result.supported`) -> `backToList()`;
  - selain itu `loadDiff({ silent: true })` (tanpa mengosongkan `diffData`, agar tidak berkedip). Pastikan effect tidak melakukan loop: jangan baca `diffData`/`diffLoading` secara reaktif di dalam effect ini (bungkus dengan `untrack` dari `svelte`).
- Sesi berganti: `$effect` pada `sessionId`; bila berubah dan `selectedPath !== null` -> `backToList()` (gunakan variabel `prevSessionId` non-reaktif agar tidak memicu saat inisialisasi). Juga `selected = null` sudah ditangani effect tiket 01 saat daftar kosong.

c) Tombol Buka file:
- `openInPane(f: ReviewFile | string, e?: Event)`: `e?.stopPropagation()`; bila `!session` return; `const rel = typeof f === 'string' ? f : f.path`; `const abs = session.cwd.replace(/\/+$/, '') + '/' + rel`; `openFilePane({ projectId: session.projectId, filePath: abs, workspaceId: session.workspaceId })`. Tidak memodifikasi shell; tanpa nomor baris.
- Di baris daftar: tombol ikon kecil (`<button class="open" type="button" aria-label="Buka file {f.path}" title="Buka file" onclick={(e) => openInPane(f, e)}>` dengan ikon SVG inline kecil atau glyph) di ujung baris. Karena `<button>` tidak boleh bersarang di `<button class="row">`, ubah struktur baris minimal: bungkus di `<li class="item">` (flex) berisi tombol `.row` (flex: 1, tetap `role="option"` dan semua atribut lama) dan tombol `.open` sebagai saudaranya; pindahkan efek hover/selected ke `.item` bila perlu. Atribut `role="option"` pada `.row` dalam `ul role="listbox"` dipertahankan; tombol `.open` di luar `.row` tetap fokus-able. `stopPropagation` tetap dipanggil. Nonaktifkan (`disabled`) bila `session` null.

d) Ringkasan per baris dan penanda:
- Setelah `.path`, sebelum `done`: bila `!f.binary && !f.tooLarge` tampilkan `<span class="stat"><span class="plus">+{f.additions}</span> <span class="minus">-{f.deletions}</span></span>`; `.plus` warna `--ok`, `.minus` `--danger`, `font-size: 12px`, `flex: none`, tabular nums.
- Penanda: ubah teks yang ada menjadi "biner" dan "terlalu besar" (saat ini "besar"; ganti teks menjadi "terlalu besar", `title` tetap).

e) Tampilan diff menggantikan daftar:
- Markup daftar yang ada (head + list/empty) dibungkus `<div class="listpane" hidden={selectedPath !== null}>` (atau `class:hidden`; gunakan `display: none` via CSS bukan `{#if}` agar filter `onlyUnreviewed` dan scroll tidak hilang). Pastikan `.listpane` menjaga layout flex kolom `flex: 1; min-height: 0`.
- Di atasnya (saudara), `{#if selectedPath !== null}` blok `.diffpane` (flex kolom, `flex: 1`, `min-height: 0`):
  - Header `.dhead` tinggi 32px: tombol kembali (`aria-label="Kembali ke daftar"`, teks/ikon "<" dengan label "Kembali") memanggil `backToList`; path (ellipsis, `title`), ringkasan `+N -M` file terpilih bila ada, tombol "Buka file" (ikon + `aria-label="Buka file"`) memanggil `openInPane(selectedPath)`.
  - Isi: `diffLoading && !diffData` -> teks "Memuat diff..."; `diffError` (dan belum ada `diffData`, atau tampilkan di atas diff lama) -> pesan error inline "Gagal memuat diff: {diffError}" + tombol "Coba lagi" yang memanggil `loadDiff()`; `diffData` -> `<DiffView diff={diffData} path={selectedPath} />` (tanpa `hunkActions`, `footer`, `onLineClick`).
- Edge: kondisi `!result.supported` atau `result.files.length === 0` menampilkan pesan kosong yang sudah ada tanpa membuka diff (effect di langkah b mengembalikan ke daftar).

f) Gaya: tambahkan kelas baru (`.listpane`, `.item`, `.open`, `.stat`, `.plus`, `.minus`, `.diffpane`, `.dhead`, `.back`, `.err`, `.retry`) memakai token yang sama dan pola border/hover file itu; jangan ubah kelas lama selain yang disebut di atas.

### 4. Verifikasi lokal

- Selama backend belum siap boleh memakai data tiruan sementara (mis. variabel sementara di `ReviewTab` atau `diff.ts`) untuk melihat tampilan; HAPUS semua data tiruan sebelum selesai (grep `mock`/`TODO` sebelum melapor).
- `cd frontend && npm run build` harus sukses. Opsional: `npm run check` bila hijau di baseline; jangan memperbaiki galat di luar ownedPaths.

## Perubahan

- Baru: `frontend/src/modules/review/diff.ts`
- Baru: `frontend/src/modules/review/DiffView.svelte`
- Diubah (penyisipan saja): `frontend/src/modules/review/ReviewTab.svelte` (impor, state sesi aktif lokal, state diff, `openDiff/loadDiff/backToList/openInPane`, ringkasan `+N -M`, penanda "biner"/"terlalu besar", tombol Buka file, pembungkus `.listpane`, blok `.diffpane`, gaya)

## Wiring

Yang diekspos untuk langkah wiring (tidak ada penyambungan oleh fitur ini):
- `fetchDiff`, `hunkSummary`, tipe `DiffResult/Hunk/DiffLine/DiffLineKind` dari `diff.ts` (dipakai tiket 03/04).
- `DiffView.svelte` dengan snippet `hunkActions` dan `footer` serta prop `onLineClick` (tiket 03/04 yang mengisi; di sini kosong).
- `ReviewTab.svelte` sudah memakai `fetchDiff` dan `DiffView` sendiri; wiring hanya memverifikasi bentuk JSON `review.diff` (overview bagian 5 butir 3 dan 4). Tidak ada edit Go atau file lain oleh fitur ini; `review.diff` dan `FillDiffStats` disambung agent wiring.

## Selesai bila

- `cd frontend && npm run build` sukses (satu-satunya guard; tanpa unit test).
- Hanya tiga path ownedPaths yang berubah (`git status` menunjukkan tidak ada file lain di luar docs); tanpa perubahan `package.json`/lockfile.
- Daftar menampilkan `+N -M` dan penanda "biner"/"terlalu besar"; klik baris menampilkan `DiffView` menggantikan daftar; tombol kembali mengembalikan daftar dengan filter `onlyUnreviewed` dan scroll terjaga (daftar disembunyikan, bukan di-unmount).
- `DiffView` menampilkan header `@@ -a,b +c,d @@`, dua kolom nomor, warna add/del/ctx dengan token tema, scroll horizontal di blok diff, tiga pesan status, dan tanpa `hunkActions`/`footer`/`onLineClick` tampilan tidak berubah (tanpa elemen pembungkus kosong, tanpa kursor pointer).
- Tombol "Buka file" (baris daftar dan header diff) memanggil `openFilePane` dengan path absolut `cwd + '/' + path`, `stopPropagation` aktif, dan tidak membuka diff.
- `fetchDiff` diulang saat `reviewFiles` berubah; respons basi diabaikan (token `diffSeq`); kembali ke daftar bila sesi berganti atau file hilang; state loading dan error dengan tombol "Coba lagi".
- Tidak ada data tiruan tersisa, tidak ada dependency baru, `store.ts`/`types.ts`/`index.ts` tidak berubah.
