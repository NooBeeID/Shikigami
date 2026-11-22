# Plan: Tombol hunk, konfirmasi, dan keyboard

## Metadata

- Tiket: `agent-diff-review-ticket-03-review-actions` (`docs/tickets/agent-diff-review-ticket-03-review-actions.md`)
- Fitur: `review-actions-ui`
- Kontrak bersama: `docs/plans/agent-diff-review-ticket-03-review-actions/overview.md` (bagian 2.2 IPC, 2.4 kontrak komponen, 2.5 konvensi, 7 aturan paralel). Baca bagian itu sebelum mulai; plan ini tidak mengulangnya.
- ownedPaths:
  - `frontend/src/modules/review/HunkActions.svelte` (baru)
  - `frontend/src/modules/review/keys.ts` (baru)
  - `frontend/src/modules/review/DiffView.svelte` (hanya sisipan)
  - `frontend/src/modules/review/ReviewTab.svelte` (hanya sisipan)
- Depends on foundation: tidak (`hasFoundation=false`).

## Tujuan

Pengguna dapat meninjau perubahan agent langsung dari tab Review: tombol Revert dan Terima pada tiap hunk dan pada header file, konfirmasi inline untuk Revert (tanpa modal, peringatan keras untuk file `external`), penanganan `[CONFLICT]`/`[EXTERNAL]`, badge "perubahan lain", dan navigasi keyboard `j/k/a/r` pada panel diff. Setelah aksi, diff dan daftar file dimuat ulang tanpa reload manual. Daemon (`review.revert`/`review.accept`) dikerjakan agent lain; sampai ia ter-wiring, panggilan IPC gagal dan itu wajar.

## Path yang dimiliki

HANYA path di ownedPaths di atas yang boleh disentuh. Jangan sentuh file wiring, Go, `store.ts`, `types.ts`, `diff.ts`, `index.ts`, `ReviewBadge.svelte`, modul lain, `package.json`, maupun path fitur `review-actions-daemon`. Untuk `DiffView.svelte` dan `ReviewTab.svelte` hanya sisipan aditif: jangan ubah perilaku, markup, atau gaya yang sudah ada selain yang tertulis di langkah. Tanpa dependency npm baru, tanpa unit test, tanpa `git commit`.

## Kontrak

Dipakai (sudah ada di kode, jangan diubah):
- `call<T>(method, params)` dari `../../lib/ipc`.
- `diff.ts`: tipe `Hunk` (`id`, ...), `DiffResult {hunks, currentHash, binary, tooLarge}`.
- `store.ts`: `refreshReview(sessionId)`, `reviewFiles`. `types.ts`: `ReviewFile` (sudah punya `external`, `binary`, `tooLarge`).
- `DiffView.svelte` yang ada: prop `hunkActions?: Snippet<[Hunk]>` dirender di `.hunk-head .actions`.
- `ReviewTab.svelte` yang ada: `sessionId`, `selectedPath`, `diffData`, `selectedFile`, `loadDiff({silent?})`, `backToList()`, `visible`/`result.files`, `.dhead`, `.diffpane`.
- IPC persis overview 2.2:
  - `review.revert {sessionId, path, hunkIds?, expectHash, force?}` -> `{ok, removed}`
  - `review.accept {sessionId, path, hunkIds?}` -> `{ok, removed}`
  - Error berawalan `[CONFLICT]` atau `[EXTERNAL]` (periksa dengan `message.includes('[CONFLICT]')`, karena transport bisa menambah awalan; ambil `e instanceof Error ? e.message : String(e)`).

Disediakan (persis overview 2.4):
- `HunkActions.svelte`: `export interface ReviewActionResult { removed: boolean; conflict?: boolean }` (di `<script lang="ts" module>`); props `{sessionId, path, currentHash, external, scope: 'hunk'|'file', hunk?, onDone}`; metode publik `accept(): Promise<void>`, `startRevert(): void`, `confirmRevert(): Promise<void>`, `cancel(): void`, getter `pending` (true bila konfirmasi Revert terbuka) dan getter `busy`.
- `keys.ts`: `ReviewKeyHandlers {target, next, prev, accept, revert, confirm, cancel, isConfirming?}` dan `attachReviewKeys(handlers): () => void`.

## Langkah

### 1. `keys.ts` (baru)

```ts
export interface ReviewKeyHandlers {
  target: HTMLElement;
  next(): void; prev(): void; accept(): void; revert(): void; confirm(): void; cancel(): void;
  isConfirming?: () => boolean;
}
export function attachReviewKeys(handlers: ReviewKeyHandlers): () => void
```
- Pasang satu listener `keydown` pada `handlers.target` (bukan `window`), sehingga hanya aktif bila fokus/target event ada di dalam panel diff. Kembalikan fungsi yang memanggil `removeEventListener`.
- Handler: abaikan bila `e.defaultPrevented`, `e.ctrlKey || e.metaKey || e.altKey` (shift juga diabaikan untuk huruf polos), atau `e.target` (cast `HTMLElement`) adalah `input`, `textarea`, `select`, atau `isContentEditable`.
- Peta: `j` -> `next`, `k` -> `prev`, `a` -> `accept`, `r` -> `revert`. `Enter` -> `confirm` dan `Escape` -> `cancel` HANYA bila `handlers.isConfirming?.() === true`. Panggil `e.preventDefault()` (dan `stopPropagation`) hanya untuk tombol yang benar-benar ditangani. Tanpa dependency; tanpa sentuhan ke `modules/layout/shortcuts.ts`.

### 2. `HunkActions.svelte` (baru)

- `<script lang="ts" module>` mengekspor `ReviewActionResult`. `<script lang="ts">` dengan `$props()` bertipe sesuai kontrak; import `call` dari `../../lib/ipc` dan `type Hunk` dari `./diff`.
- State `$state`: `confirming` (boolean, konfirmasi Revert terbuka), `externalConfirm` (boolean, konfirmasi keras setelah `[EXTERNAL]`), `busy`, `errMsg: string | null`.
- Turunan: `hunkIds = scope === 'hunk' && hunk ? [hunk.id] : undefined`. `needsStrongWarn = external || externalConfirm`.
- `async function accept()`: bila `busy` return; `busy = true; errMsg = null`; `await call('review.accept', {sessionId, path, hunkIds})`; sukses -> `onDone({removed: !!res?.removed})`; `catch` -> `handleError(e)`; `finally busy = false`. Tidak boleh ada unhandled rejection (semua `await` dalam try/catch; tombol memanggil `void accept()`).
- `startRevert()`: bila `busy` return; `confirming = true; errMsg = null`.
- `cancel()`: `confirming = false; externalConfirm = false; errMsg = null` (abaikan bila `busy`).
- `async function confirmRevert()`: bila `!confirming || busy` return. `force = external || externalConfirm`; panggil `review.revert {sessionId, path, hunkIds, expectHash: currentHash, force: force || undefined}` (kirim `force: true` hanya bila true). Sukses -> `confirming = false; externalConfirm = false; onDone({removed})`.
- `handleError(e)`: `msg = e instanceof Error ? e.message : String(e)`.
  - `[CONFLICT]`: `confirming = false; externalConfirm = false; errMsg = 'File berubah sejak diff dihitung, dimuat ulang'; onDone({removed: false, conflict: true})`. Tidak ada aksi lanjutan otomatis.
  - `[EXTERNAL]` (hanya dari revert): `externalConfirm = true; confirming = true; errMsg = null` (UI menampilkan peringatan keras; pengguna harus menekan "Yakin?" lagi, yang kemudian mengirim `force:true`).
  - Lainnya: `errMsg = msg` dipangkas ke satu baris singkat (mis. `slice(0, 120)`), `confirming = false`.
- Metode publik: `export function accept`, `startRevert`, `confirmRevert`, `cancel` (fungsi biasa yang di-export) dan `export function pending() ...`: karena Svelte 5 tidak mengizinkan `export` getter, sediakan `export function isPending(): boolean { return confirming; }` DAN `export function isBusy(): boolean { return busy; }`. Catat penyimpangan nama ini (kontrak menyebut `get pending`) di laporan; `ReviewTab` memakai `isPending()`.
- Markup (satu `<span class="ha" class:file={scope==='file'}>`):
  - Bila `!confirming`: tombol `Revert` (`onclick={startRevert}`) dan `Terima` (`onclick={() => void accept()}`), keduanya `disabled={busy}`, `type="button"`, `title`/`aria-label` dalam bahasa Indonesia ("Revert hunk ini", "Terima hunk ini", atau "... file ini" untuk scope file).
  - Bila `confirming`: bila `needsStrongWarn` tampilkan teks peringatan `File ini diubah di luar agent; revert akan menimpa perubahan itu` (kelas `warn`, warna `--warn`/`--danger`); tombol `Yakin?` (kelas `danger`, `onclick={() => void confirmRevert()}`, `disabled={busy}`) dan `Batal` (`onclick={cancel}`, `disabled={busy}`). Untuk file `external` peringatan tampil sejak konfirmasi pertama; teks tombol keras boleh `Yakin, timpa?` hanya bila `needsStrongWarn`.
  - Bila `errMsg`: `<span class="err" role="alert">{errMsg}</span>`.
- Gaya: kecil dan kompak agar muat di `.hunk-head` (tinggi ~20px, `font-size: 11px`, padding `1px 6px`), tombol latar transparan dengan border `var(--border, ...)`, hover `var(--bg-hover, ...)`, `.danger` pakai `var(--danger, #dc2626)`, focus-visible `outline: 1px solid var(--accent, currentColor)`, `disabled` opacity 0.5 dan `cursor: default`. Tanpa modal, tanpa dependency. Peringatan panjang pada scope hunk boleh `white-space: normal; max-width: 260px`.

### 3. Sisipan `DiffView.svelte`

Hanya tiga tambahan aditif:
1. Prop opsional `selectedHunkId?: string` pada destructuring `$props()` dan tipenya.
2. Pada `<div class="hunk-head">` tambahkan `class:selected={h.id === selectedHunkId}` dan atribut `data-hunk-id={h.id}`.
3. CSS: `.hunk-head.selected { background: color-mix(in srgb, var(--accent, currentColor) 22%, transparent); box-shadow: inset 2px 0 0 var(--accent, currentColor); }`.
Perbarui komentar header file dengan satu baris tentang `selectedHunkId`. Jangan ubah yang lain.

### 4. Sisipan `ReviewTab.svelte`

Semua aditif; kode yang ada tidak diubah perilakunya.

a. Import: `import HunkActions, { type ReviewActionResult } from './HunkActions.svelte';`, `import { attachReviewKeys } from './keys';`; tambah `refreshReview` pada import `./store` yang sudah ada (`import { reviewFiles, refreshReview } from './store'`); `import type { Hunk } from './diff'` bila belum (gabung ke import `./diff` yang ada).

b. State baru:
```ts
let selectedHunkIndex = $state(0);
let diffPaneEl: HTMLElement | undefined = $state();
let hunkRefs = $state<Record<string, HunkActions | undefined>>({});   // id hunk -> instance
let fileActions: HunkActions | undefined = $state();
const selectedHunk = $derived(diffData?.hunks[selectedHunkIndex] ?? null);
```
Registri per hunk: di dalam snippet, `bind:this={hunkRefs[h.id]}` pada tiap `HunkActions` hunk (objek `$state` biasa, bukan Map). Entri basi dibersihkan di efek pada butir h.

c. Reset indeks: `$effect` yang memantau `diffData` (dan `selectedPath`) lalu meng-clamp `selectedHunkIndex` ke `[0, max(0, hunks.length-1)]`; saat `selectedPath` berubah (di `openDiff`) set `selectedHunkIndex = 0` (sisipkan satu baris di `openDiff`, atau lakukan di efek yang memantau `selectedPath` lewat `untrack`). Efek ini tidak boleh memicu `loadDiff`.

d. Seleksi dan scroll:
```ts
async function moveHunk(delta: number) {
  const n = diffData?.hunks.length ?? 0;
  if (n === 0) return;
  selectedHunkIndex = Math.min(n - 1, Math.max(0, selectedHunkIndex + delta));
  await tick();
  diffPaneEl?.querySelector(`[data-hunk-id="${CSS.escape(selectedHunk!.id)}"]`)
    ?.scrollIntoView({ block: 'nearest' });
}
```
Tidak membatalkan konfirmasi hunk lain secara otomatis kecuali: sebelum pindah panggil `hunkRefs[selectedHunk.id]?.cancel()`.

e. Handler tombol keyboard (dipasang lewat `attachReviewKeys`): `accept` -> `void hunkRefs[selectedHunk?.id]?.accept()`; `revert` -> bila `isPending()` -> `void confirmRevert()` else `startRevert()`; `confirm` -> `confirmRevert()`; `cancel` -> `cancel()`; `isConfirming` -> `hunkRefs[selectedHunk?.id]?.isPending() === true`; `next`/`prev` -> `moveHunk(+1/-1)`. Semuanya aman bila `selectedHunk` null (tidak melakukan apa-apa). Pasang di `$effect`:
```ts
$effect(() => {
  const el = diffPaneEl;
  if (!el) return;
  return attachReviewKeys({ target: el, next, prev, accept, revert, confirm, cancel, isConfirming });
});
```
Fungsi lepas dikembalikan sebagai cleanup efek (jalan saat unmount atau `diffPaneEl` berubah). Pada `.diffpane` tambahkan `bind:this={diffPaneEl}` dan `tabindex="0"` (tambahkan `outline: none` pada `.diffpane:focus-visible` atau gunakan outline halus `1px solid var(--accent)`; `svelte-ignore a11y_no_noninteractive_tabindex` bila diperlukan). Agar fokus ada di panel saat membuka diff, panggil `diffPaneEl?.focus()` setelah `tick()` di akhir `openDiff` hanya bila elemen sudah ada (opsional; pengguna juga bisa mengklik panel). Klik di area panel yang bukan kontrol memindahkan fokus ke `.diffpane` karena `tabindex`.

f. `onDone`:
```ts
async function onActionDone(r: ReviewActionResult) {
  const sid = sessionId; if (!sid) return;
  const idx = visible.findIndex((f) => f.path === selectedPath);   // sebelum refresh
  const nextPath = r.removed ? (visible[idx + 1] ?? visible[idx - 1])?.path ?? null : null;
  await Promise.all([loadDiff({ silent: true }), refreshReview(sid)]);
  if (r.removed) {
    const next = nextPath ? $reviewFiles.files.find((f) => f.path === nextPath) : undefined;
    if (next) openDiff(next); else await backToList();
  }
}
```
Catatan: efek yang ada ("file hilang -> backToList") bisa berjalan lebih dulu saat `refreshReview` selesai; supaya `openDiff(next)` tetap benar, hitung `nextPath` sebelum refresh (sudah) dan panggil `openDiff(next)` setelahnya; `backToList` ganda tidak berbahaya karena idempoten. Bila `r.conflict`, cukup `loadDiff` + `refreshReview` (jalur yang sama, `removed` false). Bila `loadDiff` gagal karena file sudah tidak ada, `diffError` yang ada menanganinya dan efek daftar mengembalikan ke daftar.

g. Aksi file di `.dhead`: sisipkan tepat sebelum tombol `.open` (di dalam `.dhead`), hanya bila `diffData && selectedFile`:
```svelte
<HunkActions bind:this={fileActions} scope="file" sessionId={sessionId!} path={selectedPath}
  currentHash={diffData.currentHash} external={selectedFile.external} onDone={onActionDone} />
```
Untuk file `binary`/`tooLarge` aksi file tetap tampil (hanya aksi seluruh file yang didukung daemon).

h. Aksi hunk lewat snippet pada `DiffView` yang sudah dirender: ubah satu baris menjadi
```svelte
<DiffView diff={diffData} path={selectedPath} selectedHunkId={selectedHunk?.id} {hunkActions} />
```
dan definisikan di markup (di dalam blok `{#if diffData}` atau tingkat atas komponen, sebelum penggunaan):
```svelte
{#snippet hunkActions(h: Hunk)}
  <HunkActions bind:this={hunkRefs[h.id]} scope="hunk" hunk={h} sessionId={sessionId!} path={selectedPath!}
    currentHash={diffData!.currentHash} external={selectedFile?.external ?? false} onDone={onActionDone} />
{/snippet}
```
Efek pembersihan: saat `diffData` berubah, hapus kunci `hunkRefs` yang id-nya tidak ada lagi di `diffData.hunks`.

i. Badge `external`: di baris daftar (`.row`) tambahkan setelah tag biner/terlalu besar `{#if f.external}<span class="tag ext" title="File diubah di luar agent">perubahan lain</span>{/if}`; di `.dhead` setelah `.stat` tambahkan badge sama bila `selectedFile?.external`. CSS `.tag.ext { color: var(--warn, #d97706); }` (aditif).

j. Jangan ubah `loadDiff`, `backToList`, efek-efek yang ada, atau gaya lain. `diffpane` mengikuti `flex` yang ada; tambahan CSS hanya `.diffpane:focus-visible { outline: 1px solid color-mix(in srgb, var(--accent, currentColor) 50%, transparent); outline-offset: -1px; }`.

### 5. Data tiruan dan pembersihan

Bila ingin menguji UI sebelum daemon siap, boleh memakai stub lokal sementara, tetapi HAPUS sebelum selesai. Hasil akhir memakai `call` nyata.

### 6. Guard

`cd frontend && npm run build`. Opsional `npm run check` bila hijau di baseline (jangan memperbaiki galat di luar ownedPaths). Pastikan tidak ada `console.error`/unhandled rejection dari kode baru.

## Perubahan

- Baru: `frontend/src/modules/review/HunkActions.svelte`
- Baru: `frontend/src/modules/review/keys.ts`
- Diubah (sisipan): `frontend/src/modules/review/DiffView.svelte` (prop `selectedHunkId`, kelas `selected`, `data-hunk-id`, CSS sorot)
- Diubah (sisipan): `frontend/src/modules/review/ReviewTab.svelte` (import, state hunk terpilih, registri `hunkRefs`, `attachReviewKeys`, `onActionDone`, `HunkActions` file di `.dhead` dan hunk via snippet `hunkActions`, badge "perubahan lain", `bind:this` + `tabindex` pada `.diffpane`)

## Wiring

Fitur ini tidak menyambungkan apa pun sendiri (tidak ada file wiring di ownedPaths). Yang diekspos untuk langkah wiring (overview bagian 5, butir 3 dan 4):
- Komponen `HunkActions` yang memanggil `review.revert`/`review.accept` dengan param dan nama field respons persis overview 2.2 (`ok`, `removed`; `expectHash`, `hunkIds`, `force`), dengan props termasuk `sessionId`.
- `attachReviewKeys` terpasang di `.diffpane` dan dilepas lewat cleanup `$effect` saat unmount.
- Setelah aksi, `onActionDone` memanggil `loadDiff` + `refreshReview(sessionId)`; event `review.changed` dari daemon akan memicu pendengar yang sudah ada di `store.ts` sehingga `ReviewBadge` ikut turun.
- Pembacaan `ReviewFile.external` (diisi daemon lewat `FillExternal` di wiring `ipc.go`) untuk badge dan `force`.
Edit Go (`ipc.go`, `wire_review.go`) adalah tugas agent Wiring, bukan fitur ini.

## Selesai bila

- `HunkActions.svelte` dan `keys.ts` ada dengan API persis bagian Kontrak (penyimpangan `isPending()/isBusy()` dicatat di laporan).
- Tiap hunk menampilkan Revert dan Terima; header file menampilkan aksi file; badge "perubahan lain" tampil di daftar dan `.dhead` untuk file `external`.
- Revert selalu lewat konfirmasi inline ("Yakin?"/"Batal"); file `external` menampilkan peringatan keras dan mengirim `force:true`; `[EXTERNAL]` memunculkan konfirmasi keras lalu kirim ulang dengan `force:true`; `[CONFLICT]` menampilkan "File berubah sejak diff dihitung, dimuat ulang" dan memanggil `onDone({removed:false, conflict:true})` sehingga induk memuat ulang diff.
- Tombol nonaktif selama panggilan; tidak ada unhandled rejection.
- `j/k/a/r` (plus Enter/Esc saat konfirmasi) hanya aktif di panel diff, tidak saat fokus di input/textarea/select/contenteditable, dan tidak dengan modifier; hunk terpilih tersorot dan di-scroll ke tampilan.
- Setelah `removed`, `ReviewTab` pindah ke file berikutnya atau `backToList()`.
- Perubahan di `DiffView.svelte` dan `ReviewTab.svelte` murni sisipan; tidak ada file di luar ownedPaths yang berubah; teks UI Indonesia, token tema yang ada.
- Guard: `cd frontend && npm run build` sukses.
