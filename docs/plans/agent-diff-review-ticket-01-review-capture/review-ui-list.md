# Plan: Tab Review dan badge sesi

## Metadata

- Tiket: `agent-diff-review-ticket-01-review-capture` (`docs/tickets/agent-diff-review-ticket-01-review-capture.md`)
- Fitur: `review-ui-list`
- ownedPaths:
  - `frontend/src/modules/review/index.ts`
  - `frontend/src/modules/review/ReviewTab.svelte`
  - `frontend/src/modules/review/store.ts`
  - `frontend/src/modules/review/types.ts`
  - `frontend/src/modules/review/ReviewBadge.svelte`
- Depends on foundation: ya, tetapi hanya untuk kontrak JSON (overview 2.3 dan 2.4). Foundation tidak punya kerja frontend, jadi fitur ini tidak memakai kode Go dan tidak menunggu backend selesai. Backend belum tentu ada saat dikerjakan: panggilan `review.files` yang gagal harus menjadi state kosong, bukan galat.

Baca `overview.md` bagian 2.2 (butir 7, 8, 9), 2.4, dan 2.5 sebagai kontrak bersama. Plan ini tidak mengulangnya.

## Tujuan

Menyediakan modul frontend `review`: tab kanan "Review" (setelah Files order 10, sebelum Activity order 20) yang menampilkan daftar file yang diubah agent pada sesi aktif, dan komponen `ReviewBadge` untuk jumlah file belum ditinjau per sesi. Belum ada diff, aksi, atau pembukaan file; hanya titik kait `selected` dan `onselect`.

## Path yang dimiliki

HANYA lima berkas di `frontend/src/modules/review/` (folder belum ada; buat):

- `types.ts`, `store.ts`, `ReviewTab.svelte`, `ReviewBadge.svelte`, `index.ts`

Jangan menyentuh: `frontend/src/modules/sidebar-left/SessionRow.svelte`, `frontend/src/main.ts` (wiring, milik `review-capture-wiring`), `frontend/src/modules/sidebar-right/FilesTab.svelte` (ada perubahan user yang belum di-commit), `modules/activity/*`, `modules/runs/*`, `modules/shell/*`, `lib/*`, semua berkas `internal/` dan `cmd/`, serta berkas tiket 02-04 (`DiffView.svelte`, `diff.ts`, `HunkActions.svelte`, `keys.ts`, `CommentBox.svelte`). Jangan menambah dependency npm, jangan membuat unit test.

## Kontrak

Disediakan (nama dan bentuk tetap, lihat overview 2.5):

```ts
// types.ts
export type ReviewStatus = 'modified' | 'added' | 'deleted';
export interface ReviewFile { path: string; status: ReviewStatus; additions: number; deletions: number; reviewed: boolean; external: boolean; binary: boolean; tooLarge: boolean }
export interface ReviewFilesResult { supported: boolean; files: ReviewFile[] }

// store.ts
export const reviewFiles: Readable<ReviewFilesResult>;        // sesi aktif; awal { supported: true, files: [] }
export const reviewCounts: Readable<Record<string, number>>;  // sessionId -> jumlah belum ditinjau
export function refreshReview(sessionId: string): Promise<void>;
export function startReviewSync(): () => void;                // idempoten

// ReviewBadge.svelte: props { sessionId: string }
// ReviewTab.svelte: tanpa props wajib; prop opsional onselect?: (file: ReviewFile) => void
// index.ts: registerRightTab({ id: 'review', title: 'Review', order: 15, component: ReviewTab }); ekspor ReviewBadge, reviewFiles, reviewCounts, refreshReview, tipe
```

Dipakai (sudah ada di repo):

- `call<T>(method, params)` dan `onEvent(topicPrefix, (topic, payload) => void): () => void` dari `frontend/src/lib/ipc.ts` (impor relatif `../../lib/ipc`).
- IPC `review.files {sessionId}` -> `{supported, files}`; event `review.changed {sessionId, count}` (overview 2.4). `count` adalah jumlah file belum ditinjau.
- `layout` dari `../../lib/stores/layout` (store dokumen `LayoutDoc`: field `focusId?: string`, `root`), `findLeaf(doc, id): Leaf | null` dari `../layout/model`. `Leaf` punya `kind` (`'terminal'` untuk pane sesi) dan `sessionId?`.
- `sessions` (Writable<Session[]>; `Session` punya `id`, `status: 'running' | 'exited'`) dari `../../lib/stores/sessions`, atau lewat `../../lib/stores`.
- `registerRightTab` dari `../shell` (pola: lihat `modules/activity/index.ts`).
- Pola store dan nomor muat basi: `modules/runs/store.ts` (baca sebagai referensi; jangan diubah).

## Langkah

### 1. `types.ts`

Salin tiga tipe persis dari bagian Kontrak. Komentar bahasa Indonesia.

### 2. `store.ts`

Impor: `derived`, `get`, `writable`, `type Readable` dari `svelte/store`; `call`, `onEvent` dari `../../lib/ipc`; `layout` dari `../../lib/stores/layout`; `sessions` dari `../../lib/stores/sessions`; `findLeaf` dari `../layout/model`; tipe dari `./types`.

State internal:

- `const EMPTY: ReviewFilesResult = { supported: true, files: [] }`.
- `const filesStore = writable<ReviewFilesResult>(EMPTY)`; ekspor `reviewFiles: Readable<ReviewFilesResult> = { subscribe: filesStore.subscribe }`.
- `const countsStore = writable<Record<string, number>>({})`; ekspor `reviewCounts` sama polanya.
- `let activeSessionId: string | null = null` (modul-lokal) dan `let loadSeq = 0`.

Fungsi pembantu (tak diekspor, awalan `review` untuk menghindari tabrakan nama):

- `reviewIsFile(v: unknown): v is ReviewFile`: cek objek, `path` string, `status` salah satu dari tiga nilai. Field lain diberi default saat normalisasi.
- `reviewNormalize(res: unknown): ReviewFilesResult`: bila `res` bukan objek atau `files` bukan array -> `{ supported: true, files: [] }`. Selain itu `supported = res.supported !== false`; `files` = entri valid hasil `reviewIsFile`, dinormalisasi: `additions`/`deletions` angka (default 0), `reviewed`/`external`/`binary`/`tooLarge` boolean (default false). Bila `supported === false`, paksa `files: []`.
- `reviewUnreviewed(files: ReviewFile[]): number`: jumlah entri `reviewed === false`.
- `reviewSetCount(sessionId: string, n: number)`: `countsStore.update`; bila `n <= 0` hapus kuncinya, selain itu set (jangan buat objek baru bila nilai sama).
- `reviewActiveFromLayout(doc: LayoutDoc): string | null`: `const leaf = doc.focusId ? findLeaf(doc, doc.focusId) : null`; bila `leaf && leaf.kind === 'terminal' && leaf.sessionId` -> `leaf.sessionId`; selain itu `undefined`-setara (lihat langkah sinkronisasi: pertahankan sesi sebelumnya). Pakai tipe kembalian `string | undefined` agar "tidak berubah" bisa dibedakan dari "tidak ada".

`export async function refreshReview(sessionId: string): Promise<void>`:

1. Bila `sessionId` kosong, return.
2. `const isActive = sessionId === activeSessionId; const seq = isActive ? ++loadSeq : loadSeq;`
3. `try { const res = reviewNormalize(await call<unknown>('review.files', { sessionId })); ... } catch (e) { console.warn('review.files gagal:', e); res = { supported: true, files: [] } }`. Galat IPC jadi state kosong dan tidak pernah dilempar.
4. Perbarui count: bila `res.supported` -> `reviewSetCount(sessionId, reviewUnreviewed(res.files))`, selain itu `reviewSetCount(sessionId, 0)`.
5. Perbarui `filesStore` hanya bila `sessionId === activeSessionId` dan (`!isActive` atau `seq === loadSeq`) -> `filesStore.set(res)`. Intinya: balasan basi (sesi aktif sudah berganti, atau ada muat lebih baru) dibuang; hitungan badge tetap diperbarui.

`function reviewHandleChanged(_topic: string, payload: unknown)`:

- Validasi `payload` objek dengan `sessionId` string dan `count` angka; selain itu abaikan.
- `reviewSetCount(sessionId, count)`.
- Bila `sessionId === activeSessionId` -> `void refreshReview(sessionId)` (daftar ikut live).

Sinkronisasi sesi aktif: `function reviewSetActive(next: string | null)`: bila sama dengan `activeSessionId` return; set `activeSessionId = next`; `loadSeq++`; bila `next` null -> `filesStore.set(EMPTY)`; selain itu `filesStore.set(EMPTY)` lalu `void refreshReview(next)` (hindari daftar sesi lama tampil sesaat).

Hitungan awal (butir 7/8 overview): `const reviewSeen = new Set<string>()` (sesi yang sudah diminta). `function reviewSeed(list: Session[])`: untuk setiap `s` dengan `s.status !== 'exited'` dan `!reviewSeen.has(s.id)`: tambahkan ke `reviewSeen`, panggil `void refreshReview(s.id)` (galat sudah ditelan di dalamnya). Sesi `supported:false` otomatis tidak diberi count (langkah 4 `refreshReview`). Bersihkan: untuk id di `reviewSeen` yang sudah tidak ada di `list`, hapus dari `reviewSeen` dan hapus kuncinya dari `countsStore` (sesi dihapus).

`export function startReviewSync(): () => void` (idempoten, mengikuti pola `startRunsSync`):

```ts
let unsub: (() => void) | null = null;
export function startReviewSync(): () => void {
  const stop = () => { if (unsub) { unsub(); unsub = null; loadSeq++; } };
  if (unsub) return stop;
  // Berlangganan event dulu agar tidak ada yang hilang, baru muat awal.
  const off = onEvent('review.changed', reviewHandleChanged);
  const offLayout = layout.subscribe((doc) => {
    const next = reviewActiveFromLayout(doc);
    if (next !== undefined) reviewSetActive(next);
  });
  const offSessions = sessions.subscribe(reviewSeed);
  unsub = () => { off(); offLayout(); offSessions(); };
  return stop;
}
```

Catatan perilaku `reviewActiveFromLayout`: fokus pada leaf terminal bersesi -> kembalikan id-nya; fokus pada leaf non-terminal (mis. file) atau `focusId` tak ada -> kembalikan `undefined` agar sesi aktif sebelumnya dipertahankan; bila `doc.root === null` (tidak ada pane sama sekali) -> kembalikan `null` (sesi aktif dikosongkan). Bila `activeSessionId` sudah mengarah ke sesi yang hilang dari layout karena pane ditutup dan fokus pindah ke pane non-terminal, perilaku "pertahankan" diterima.

Pastikan `subscribe` pada `layout` dan `sessions` memanggil callback langsung (itu sekaligus muat awal); tidak perlu panggilan muat terpisah. Jangan memanggil `get(...)` di jalur panas kecuali perlu.

### 3. `ReviewBadge.svelte`

Svelte 5 runes, `<script lang="ts">`:

```svelte
<script lang="ts">
  import { reviewCounts } from './store';
  let { sessionId }: { sessionId: string } = $props();
  const count = $derived($reviewCounts[sessionId] ?? 0);
</script>

{#if count > 0}
  <span class="review-badge" role="status" aria-label="{count} file belum ditinjau" title="{count} file belum ditinjau">{count}</span>
{/if}
```

Gaya (blok `<style>`): pil kecil `display: inline-flex; align-items: center; justify-content: center; min-width: 16px; height: 16px; padding: 0 5px; border-radius: 8px; font-size: 11px; font-weight: 600; line-height: 1; flex: none;`, `background: var(--accent, #3b82f6); color: var(--accent-fg, #fff);`. Tidak merender apa pun (tanpa elemen pembungkus) saat count 0 sehingga tata letak baris sesi tidak berubah. Bila count > 99, tampilkan `99+` (aria-label tetap memuat angka asli).

### 4. `ReviewTab.svelte`

Svelte 5 runes. Props: `let { onselect }: { onselect?: (file: ReviewFile) => void } = $props();` (opsional; dirender `<C />` tanpa props oleh `RightPanel`).

State dan turunan:

- `let onlyUnreviewed = $state(false)`; `let selected = $state<ReviewFile | null>(null)` (titik kait; tidak ada efek lain).
- `const result = $derived($reviewFiles)`; `const visible = $derived(onlyUnreviewed ? result.files.filter((f) => !f.reviewed) : result.files)`; `const unreviewed = $derived(result.files.filter((f) => !f.reviewed).length)`.
- `$effect`: bila `selected` tidak lagi ada di `result.files` (cocokkan `path`), set `selected = null`. Saat memilih ulang, simpan objek terbaru dari daftar (cari berdasarkan `path`) agar tidak basi.
- Helper murni: `splitPath(p: string): { dir: string; name: string }` (pisah di `/` terakhir; `dir` memuat garis miring akhir atau kosong); `statusLetter(s: ReviewStatus): 'M' | 'A' | 'D'`; `statusLabel(s)`: 'Diubah' / 'Baru' / 'Dihapus' (untuk `title`/`aria-label`).
- `function pick(f: ReviewFile) { selected = f; onselect?.(f); }`.

Markup (tiga keadaan, gunakan `{#if}`):

1. `!result.supported`: `<div class="empty">Provider sesi ini tidak mendukung review perubahan</div>`.
2. `result.files.length === 0`: `<div class="empty">Belum ada file yang diubah agent di sesi ini</div>`.
3. Daftar:
   - Header: `<label class="filter"><input type="checkbox" bind:checked={onlyUnreviewed} /> Belum ditinjau</label>` dan ringkasan jumlah (`{unreviewed} dari {result.files.length} belum ditinjau`), tinggi dan padding seperti `.head` di `ActivityTab.svelte`.
   - Bila `visible.length === 0` (filter menyembunyikan semua): pesan `Semua file sudah ditinjau`.
   - Daftar `<ul class="list" role="listbox" aria-label="File yang diubah">` dengan `{#each visible as f (f.path)}` berisi `<li>` yang memuat `<button class="row" class:selected={selected?.path === f.path} class:reviewed={f.reviewed} role="option" aria-selected={selected?.path === f.path} title={f.path} onclick={() => pick(f)}>`:
     - `<span class="st st-{f.status}" title={statusLabel(f.status)} aria-label={statusLabel(f.status)}>{statusLetter(f.status)}</span>`
     - `<span class="path"><span class="name">{name}</span>{#if dir}<span class="dir">{dir}</span>{/if}</span>` (nama tebal `font-weight: 600`, direktori redup `color: var(--fg-muted, #888)`; `overflow: hidden; text-overflow: ellipsis; white-space: nowrap`)
     - `{#if f.reviewed}<span class="done" title="Sudah ditinjau" aria-label="Sudah ditinjau">&#10003;</span>{/if}`
     - Bila `f.binary` atau `f.tooLarge`: tanda kecil redup `biner` / `besar` (opsional, `title` menjelaskan; tidak boleh memengaruhi logika).
     - `additions`/`deletions` TIDAK ditampilkan (selalu 0 sampai tiket 02).

Gaya `<style>` memakai token yang sama dengan `FilesTab`/`ActivityTab`: `var(--fg, inherit)`, `var(--fg-muted, #888)`, `var(--border, #444)` (atau `color-mix(in srgb, currentColor 20%, transparent)`), `var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent))`, `var(--accent, #3b82f6)`, `var(--danger, ...)`. Kontainer `.review { display:flex; flex-direction:column; height:100%; min-height:0; font-size:14px; color: var(--fg, inherit); }`; `.list { flex:1; min-height:0; overflow-y:auto; margin:0; padding:0; list-style:none; }`; `.row { display:flex; align-items:center; gap:8px; width:100%; height:28px; padding:0 8px; background:none; border:none; color:inherit; font:inherit; text-align:left; cursor:pointer; }`; hover dan `.selected` memakai `--bg-hover`; `.row:focus-visible { outline: 1px solid var(--accent, currentColor); outline-offset:-1px; }`; `.reviewed .path { opacity: 0.6 }`; huruf status `width: 14px; text-align:center; font-weight:700; font-size:12px; flex:none`, warna: M `var(--warn, #d97706)`, A `var(--ok, #16a34a)`, D `var(--danger, #dc2626)`; `.empty { flex:1; display:flex; align-items:center; justify-content:center; padding: 12px; text-align:center; color: var(--fg-muted, #888); font-size:14px; }`. Dukung navigasi keyboard dasar lewat fokus tombol bawaan (Tab); `j/k` milik tiket 03, jangan dibuat.

Jangan memanggil `startReviewSync()` dari komponen; itu tugas `index.ts`.

### 5. `index.ts`

```ts
import { registerRightTab } from '../shell';
import ReviewTab from './ReviewTab.svelte';
import ReviewBadge from './ReviewBadge.svelte';
import { startReviewSync } from './store';

registerRightTab({ id: 'review', title: 'Review', order: 15, component: ReviewTab });

// Idempoten; modul ini dimuat sekali oleh glob di main.ts.
startReviewSync();

export { ReviewTab, ReviewBadge };
export { reviewFiles, reviewCounts, refreshReview } from './store';
export type { ReviewFile, ReviewStatus, ReviewFilesResult } from './types';
```

Catatan: `startReviewSync` TIDAK diekspor dari `index.ts` (kontrak overview hanya mengekspor `reviewFiles, reviewCounts, refreshReview`); pembersih hasilnya tidak diperlukan. Bila impor `../shell` menimbulkan siklus saat build, ikuti cara `modules/activity/index.ts` (impor yang sama persis) dan jangan mengubah modul shell.

### 6. Guard

Dari `frontend/`: `npm run build` lalu `npm run check`. Perbaiki semua galat tipe dan peringatan svelte-check yang berasal dari lima berkas milik fitur ini. Galat yang berasal dari berkas lain (mis. `FilesTab.svelte` yang sedang diubah user) bukan tanggung jawab fitur ini; catat di laporan, jangan diperbaiki.

## Perubahan

Dibuat (semua baru, di `frontend/src/modules/review/`):

- `types.ts`
- `store.ts`
- `ReviewTab.svelte`
- `ReviewBadge.svelte`
- `index.ts`

Tidak ada berkas yang diubah atau dihapus.

## Wiring

Yang diekspos untuk disambungkan oleh `review-capture-wiring` (JANGAN disambungkan di fitur ini):

- `import { ReviewBadge } from '../review';` untuk `frontend/src/modules/sidebar-left/SessionRow.svelte`, dipakai sebagai `<ReviewBadge sessionId={session.id} />` (tepat satu import dan satu elemen; badge tidak merender apa pun saat count 0).
- `modules/review/index.ts` ter-load otomatis oleh `import.meta.glob('./modules/*/index.ts', { eager: true })` di `main.ts`; efek sampingnya (`registerRightTab` dan `startReviewSync()`) jalan saat itu. Fitur ini tidak mengubah `main.ts`.
- `reviewFiles`, `reviewCounts`, `refreshReview`, dan tipe `ReviewFile`/`ReviewStatus`/`ReviewFilesResult` untuk tiket 02-04. Titik kait pemilihan file: prop `onselect` dan state `selected` di `ReviewTab.svelte` (tiket 02 yang menampilkan diff dan tiket lanjutan yang memanggil `openFilePane`).

## Selesai bila

- Kelima berkas ada di `frontend/src/modules/review/` dengan nama ekspor dan signature persis seperti bagian Kontrak; tidak ada berkas lain yang disentuh (`git status` hanya menambah lima berkas itu dari fitur ini).
- `cd frontend && npm run build` sukses.
- `cd frontend && npm run check` tidak melaporkan galat dari berkas milik fitur ini.
- Secara kode (tanpa perlu backend): tab terdaftar dengan `id: 'review'`, `order: 15`; `supported=false` menampilkan "Provider sesi ini tidak mendukung review perubahan"; daftar kosong menampilkan "Belum ada file yang diubah agent di sesi ini"; filter "Belum ditinjau" menyembunyikan `reviewed=true`; `ReviewBadge` tersembunyi saat count 0; galat IPC `review.files` menjadi state kosong tanpa melempar; `startReviewSync` idempoten.
- Tanpa unit test dan tanpa dependency baru.
