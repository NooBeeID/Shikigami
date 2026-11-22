# Plan: Penanda stale dan progres di panel

## Metadata

- Tiket: `grimoire-ticket-03-grimoire-refresh` (`docs/tickets/grimoire-ticket-03-grimoire-refresh.md`)
- Fitur: `stale-ui` (Penanda stale dan progres di panel)
- ownedPaths: `frontend/src/modules/grimoire/stale/`
- Depends on foundation: tidak (murni frontend; foundation hanya stub Go `diff.go`/`incremental.go`). Bergantung pada kode tiket 01 yang sudah ada: `frontend/src/modules/grimoire/{client.ts,store.ts,GrimoirePanel.svelte}`, hanya dibaca sebagai referensi gaya dan nama impor.

Rujukan: `overview.md` bagian 2.4 (IPC aditif) dan 2.6 (kontrak frontend) adalah sumber kebenaran; plan ini tidak mengulangnya, hanya menjabarkan langkah.

## Tujuan

Menyediakan komponen Svelte kecil dan store untuk menandai bab/buku stale, menampilkan indikator "sedang membangun", dan tombol "Perbarui" (build inkremental, tanpa `full`) di panel Grimoire. Store hanya menyegarkan diri lewat event `grimoire.changed` untuk project aktif; tanpa `setInterval`/`setTimeout`/polling. Field IPC yang belum dikirim backend diperlakukan `false`/`0`.

## Path yang dimiliki

HANYA `frontend/src/modules/grimoire/stale/` (folder baru, semua berkas di dalamnya). Dilarang menyentuh: `GrimoirePanel.svelte`, `client.ts`, `store.ts`, `index.ts`, `openPage.ts` (semua milik tiket 01/Wiring), `frontend/src/lib/*`, `package.json`/lockfile, apa pun di `internal/` dan `cmd/`, serta path fitur lain (`diff-incremental`, `session-trigger`). Tidak membuat berkas di luar folder ini.

## Kontrak

Dipakai (tanpa mengubah):
- `call` dan `onEvent` dari `../../../lib/ipc`. Signature nyata: `call<T>(method, params?) => Promise<T>`; `onEvent(topicPrefix, (topic, payload) => void) => () => void` (mengembalikan fungsi unsubscribe).
- Method IPC langsung lewat `call` (JANGAN impor/ubah `client.ts` untuk fungsi baru; boleh `import type` dari `../client` bila perlu, tetapi lebih aman mendefinisikan tipe lokal):
  - `grimoire.status {projectId}` -> `{exists, staleCount?, building?, ...}`
  - `grimoire.list {projectId}` -> `[{name, tags, chapters:[{name, path, stale?}]}] | null`
  - `grimoire.build {projectId}` (tanpa `full`) -> hasil build atau `{building: true}`
- Event `grimoire.changed` dengan payload `{projectId: string}`.
- Token tema (sudah dipakai `GrimoirePanel.svelte`): `--bg`, `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--danger`, selalu dengan fallback seperti di panel (mis. `var(--danger, inherit)`). Tidak ada warna keras (`#rrggbb`, `rgb()`), tidak ada token global baru.

Disediakan (persis sesuai overview 2.6):

```ts
// staleStore.ts
export interface StaleState {
  building: boolean; staleCount: number;
  staleChapters: Set<string>; // path bab stale
  staleBooks: Set<string>;    // nama buku yang memuat >=1 bab stale
  error: string | null;
}
export const staleState: Readable<StaleState>;
export function startStaleStore(projectId: string): () => void;
export function runUpdate(projectId: string): Promise<void>;
export function isChapterStale(s: StaleState, path: string): boolean;
export function isBookStale(s: StaleState, book: string): boolean;
```

Props komponen: `StaleBadge {label?, title?}`, `BuildIndicator {building}`, `UpdateButton {projectId, building, staleCount, onupdate?}`, `StaleSlot {projectId: string|null, exists: boolean}`.

## Langkah

Semua berkas di `frontend/src/modules/grimoire/stale/`. Svelte 5 runes (`$props`, `$state`, `$derived`, `$effect`), `<script lang="ts">`, teks UI bahasa Indonesia, identifier bahasa Inggris, komentar bahasa Indonesia.

1. **Baca referensi (tanpa mengubah).** `GrimoirePanel.svelte` (gaya CSS, class `.stale`, `.msg`, `.icon`, `.header`), `store.ts` (pola `onEvent('grimoire.changed')`, guard `loadSeq` untuk respons usang), `client.ts` (bentuk `GrimoireStatus`/`GrimoireBook`). Bila nama nyata berbeda dari yang dikutip di sini, ikuti kode nyata dan catat singkat di ringkasan akhir.

2. **`staleStore.ts`** (tanpa efek saat impor modul: tidak ada `onEvent`/`call` di level modul).
   - `import { writable, type Readable } from 'svelte/store'`; `import { call, onEvent } from '../../../lib/ipc'`.
   - Tipe lokal longgar: `type RawChapter = { path?: unknown; stale?: unknown }`, `type RawBook = { name?: unknown; chapters?: unknown }`, `type RawStatus = { exists?: unknown; staleCount?: unknown; building?: unknown }`.
   - `const initial = (): StaleState => ({ building: false, staleCount: 0, staleChapters: new Set(), staleBooks: new Set(), error: null })`.
   - `const state = writable<StaleState>(initial())`; `export const staleState: Readable<StaleState> = { subscribe: state.subscribe }`.
   - Variabel modul: `let currentId: string | null = null`, `let seq = 0`.
   - `async function load(id: string)`: `const my = ++seq`; `st = await call<RawStatus | null>('grimoire.status', { projectId: id })`; bila `my !== seq || currentId !== id` return (usang). `building = st?.building === true`, `staleCount = typeof st?.staleCount === 'number' ? st.staleCount : 0`. Bila `st?.exists !== true` set state kosong (kecuali `building`). Selain itu `books = await call<RawBook[] | null>('grimoire.list', { projectId: id })`, cek usang lagi, `books ?? []`; bangun `staleChapters` (path string dari bab dengan `stale === true`) dan `staleBooks` (nama buku bila ada bab stale). Field `stale` yang tak ada = false. `state.set({ building, staleCount, staleChapters, staleBooks, error: null })`. `catch (e)`: bila usang abaikan, selain itu `state.update(s => ({ ...s, error: staleMsg(e) }))`. `staleMsg` memotong ke 200 karakter (pola `msg` di `store.ts`).
   - `export function startStaleStore(projectId: string): () => void`: `currentId = projectId; seq++; state.set(initial()); void load(projectId);` lalu `const off = onEvent('grimoire.changed', (_t, payload) => { ... })` yang memeriksa `typeof payload === 'object' && payload && 'projectId' in payload`, `pid === currentId` (project lain diabaikan), lalu `void load(pid)`. Kembalikan fungsi: `off(); if (currentId === projectId) { currentId = null; seq++; state.set(initial()); }`. Tidak ada timer apa pun.
   - `export async function runUpdate(projectId: string): Promise<void>`: set `error: null` dan `building: true` secara optimistis (hanya bila `currentId === projectId`); `await call('grimoire.build', { projectId })` (TANPA field `full`); `catch` -> `error` (hanya bila masih project aktif). Di `finally` panggil `load(projectId)` bila `currentId === projectId` agar status server (sumber kebenaran) menimpa `building` optimistis; event `grimoire.changed` tetap menjadi jalur refresh utama. Respons `{building: true}` bukan galat.
   - `export const isChapterStale = (s: StaleState, path: string) => s.staleChapters.has(path)`; `export const isBookStale = (s: StaleState, book: string) => s.staleBooks.has(book)`.

3. **`StaleBadge.svelte`**: props `{ label = 'stale', title = 'Bab ini belum diperbarui' }` via `$props()`. Render `<span class="stale-badge" {title}>` berisi titik (`&#9679;` kecil) dan `{label}`. CSS: `color: var(--danger, inherit)`, `font-size: 10px`, `flex: none`, `white-space: nowrap`, gaya sejalan dengan `.stale` panel.

4. **`BuildIndicator.svelte`**: props `{ building }`. `{#if building}<span class="build-indicator" role="status" aria-live="polite"><span class="pulse" aria-hidden="true"></span>Sedang membangun...</span>{/if}`. Animasi hanya CSS: `@keyframes stale-pulse { 0%,100% { opacity: .35 } 50% { opacity: 1 } }`; `.pulse` lingkaran 6px berwarna `var(--accent, currentColor)`. Tambahkan `@media (prefers-reduced-motion: reduce) { .pulse { animation: none } }`. Warna teks `var(--fg-muted, inherit)`. Nama keyframes diberi awalan unik (`stale-pulse`).

5. **`UpdateButton.svelte`**: props `{ projectId, building, staleCount, onupdate }`. Handler `async function click() { if (building) return; onupdate?.(); await runUpdate(projectId); }` (impor `runUpdate` dari `./staleStore`). Tombol `type="button"`, `disabled={building}`; label: `building ? 'Membangun...' : 'Perbarui'` ditambah ` ({staleCount} stale)` hanya bila `!building && staleCount > 0`. Gaya tombol kecil: `border: 1px solid var(--border, currentColor)`, `background: var(--bg-elev, transparent)`, `color: var(--fg, inherit)`, hover `var(--bg-hover, ...)`, `:focus-visible` outline `var(--accent, currentColor)`, `:disabled { opacity: .6; cursor: default }`, `border-radius: 4px`, `font-size: 11px`.

6. **`StaleSlot.svelte`**: props `{ projectId, exists }`. `<script module lang="ts">` mengekspor ulang `export { staleState, isChapterStale, isBookStale } from './staleStore';` agar panel bisa `import { staleState } from './stale/StaleSlot.svelte'` atau langsung dari `./stale/staleStore`. Di instance script: `let s = $state<StaleState>(...)` diisi `$effect(() => staleState.subscribe((v) => { s = v; }))`; efek kedua: `$effect(() => { if (!exists || !projectId) return; const stop = startStaleStore(projectId); return stop; })` (berhenti saat project berganti/unmount/exists false; `projectId` dan `exists` dibaca di dalam efek agar reaktif). Template: `{#if exists && projectId}<div class="stale-slot"><BuildIndicator building={s.building} /><UpdateButton {projectId} building={s.building} staleCount={s.staleCount} />{#if s.error}<span class="err" title={s.error}>{s.error}</span>{/if}</div>{/if}`. CSS: flex, `gap: 8px`, `align-items: center`, `justify-content: space-between`, `padding: 4px 12px`, `.err` `color: var(--danger, inherit)` dengan ellipsis. Komponen kosong bila `exists` false atau `projectId` null (tombol Generate tiket 01 tidak disentuh; "Perbarui" hanya saat grimoire ada).

7. **Verifikasi bebas polling.** `grep -nE "setInterval|setTimeout|requestAnimationFrame|fsnotify" frontend/src/modules/grimoire/stale` harus kosong; `grep -nE "#[0-9a-fA-F]{3,8}\b|rgb\(" frontend/src/modules/grimoire/stale` harus kosong.

8. **Build guard.** `cd frontend && npm run check && npm run build`. Bila gagal karena berkas di luar `stale/`, jangan diperbaiki; laporkan. Tanpa unit test, tanpa dependency baru (`git diff --stat -- frontend/package.json frontend/package-lock.json` kosong).

## Perubahan

Dibuat (semua baru, di `frontend/src/modules/grimoire/stale/`):
- `staleStore.ts`
- `StaleBadge.svelte`
- `BuildIndicator.svelte`
- `UpdateButton.svelte`
- `StaleSlot.svelte`

Tidak ada berkas lain yang diubah.

## Wiring

Yang diekspos untuk langkah wiring (JANGAN disambungkan di plan ini; `GrimoirePanel.svelte` dan `client.ts` tidak disentuh):
- `StaleSlot.svelte` (`projectId: string | null`, `exists: boolean`): pengganti komentar `<!-- slot: stale/llm (tiket 3, 4) -->` di header panel (di panel nyata saat ini komentarnya mungkin `<!-- slot: laws (tiket 2) -->`; ikuti keadaan nyata). Variabel panel: `s.projectId`, `s.exists`.
- `StaleBadge.svelte`: dipakai panel pada baris bab bila `c.stale` (sudah ada di `GrimoireChapter`) dan pada header buku bila ada bab stale (`b.chapters.some((c) => c.stale)` atau `isBookStale($staleState, b.name)`). Panel sudah memakai kelas `.stale` sendiri; wiring menggantinya dengan `StaleBadge`.
- `staleState`, `isChapterStale`, `isBookStale` dari `./stale/staleStore` (juga diekspor ulang lewat `<script module>` di `StaleSlot.svelte`).
- Catatan untuk wiring: panel punya indikator "Membangun..." dan tombol ikon bangun-ulang sendiri (tiket 01); wiring memutuskan apakah dibuang agar tidak ganda. Store stale memuat `grimoire.status`/`grimoire.list` sendiri (dua pemanggilan tambahan per event), diterima demi kemandirian komponen.
- Bergantung pada backend (Wiring IPC) mengirim `staleCount`, `building`, `chapters[].stale`; sebelum itu semua diperlakukan `0`/`false`.

## Selesai bila

- Kelima berkas ada di `frontend/src/modules/grimoire/stale/` dengan signature/props sesuai bagian Kontrak; tidak ada berkas lain berubah (`git status` hanya menunjukkan folder ini dari fitur ini).
- `startStaleStore` tidak melakukan apa pun saat impor, hanya merespons event `grimoire.changed` untuk `projectId` yang sama, mengabaikan project lain, dan fungsi berhentinya melepas langganan.
- `UpdateButton` memanggil `grimoire.build` tanpa `full`, nonaktif berlabel "Membangun..." saat `building`, menampilkan `(N stale)` bila `staleCount > 0`.
- `BuildIndicator` tidak merender apa pun saat `building` false; animasi murni CSS.
- Tidak ada `setInterval`/`setTimeout`/polling, tidak ada warna keras, hanya token tema dengan fallback.
- Guard: `cd frontend && npm run check && npm run build` sukses (path fitur ini bebas galat); tanpa unit test; tanpa dependency baru.
