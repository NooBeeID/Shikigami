# Plan: Panel Grimoire di sidebar kiri

## Metadata

- Tiket: `grimoire-ticket-01-grimoire-build` (`docs/tickets/grimoire-ticket-01-grimoire-build.md`)
- Fitur: `panel-ui`
- Sumber kebenaran kontrak: `docs/plans/grimoire-ticket-01-grimoire-build/overview.md` (bagian 2.4 IPC, 2.6 Frontend). Jangan diubah.
- ownedPaths:
  - `frontend/src/modules/grimoire/index.ts`
  - `frontend/src/modules/grimoire/GrimoirePanel.svelte`
  - `frontend/src/modules/grimoire/store.ts`
  - `frontend/src/modules/grimoire/client.ts`
- Depends on foundation: ya. Hanya untuk `frontend/src/modules/grimoire/openPage.ts` (stub dengan signature final `openGrimoirePage(projectId: string, relPath: string): Promise<string>`). Berkas itu milik `readonly-pane`; jangan dibuat, diubah, atau ditiru. Bila belum ada di working tree saat kamu mulai, jalankan build guard setelah foundation hadir; jangan membuat berkas tiruan.

## Tujuan

Menampilkan panel **GRIMOIRE** di sidebar kiri (di bawah daftar sesi dan Files) yang mengikuti project aktif:

- Belum ada grimoire: tombol **Generate Grimoire**.
- Sedang build: status "Membangun..." dengan tombol nonaktif.
- Sudah ada: buku sebagai grup terlipat, bab sebagai item; klik bab membuka halaman lewat `openGrimoirePage`.
- Ter-refresh lewat event `grimoire.changed`, tanpa `setInterval`/polling.
- Menyediakan dua slot komentar kosong untuk tiket 2 sampai 4.

## Path yang dimiliki

HANYA empat berkas ini yang boleh dibuat/diubah:

- `frontend/src/modules/grimoire/index.ts`
- `frontend/src/modules/grimoire/GrimoirePanel.svelte`
- `frontend/src/modules/grimoire/store.ts`
- `frontend/src/modules/grimoire/client.ts`

Dilarang menyentuh: `AgentSidebar.svelte` dan apa pun di `modules/sidebar-left/`, `main.ts`, `lib/*`, `styles/theme.css` (jangan tambah token global), `openPage.ts`, `modules/layout/*`, `modules/files/*`, `package.json`/lockfile, seluruh `internal/` dan `cmd/`, serta path fitur lain. Berkas lain yang sedang berstatus modified di working tree (mis. `FilesTab.svelte`) bukan milikmu; jangan diubah atau di-stage.

## Kontrak

### Yang disediakan (nama final, sesuai overview 2.6)

`client.ts` (satu-satunya tempat memanggil `call('grimoire.*')`):

```ts
export interface GrimoireChapter { name: string; path: string; stale: boolean }
export interface GrimoireBook { name: string; tags: string[]; chapters: GrimoireChapter[] }
export interface GrimoireStatus {
  exists: boolean; books: number; pages: number; staleCount: number; building: boolean;
  indexBytes: number; estTokens: number; lawsBytes: number; llmEnabled: boolean; lastBuildCost: number; dir: string;
}
export interface GrimoireSummary { books: number; pages: number; indexBytes: number }
export function grimoireBuild(projectId: string): Promise<GrimoireSummary>
export function grimoireStatus(projectId: string): Promise<GrimoireStatus>
export function grimoireList(projectId: string): Promise<GrimoireBook[]>
export function grimoireRead(projectId: string, path: string): Promise<{ content: string; size: number }>
```

`store.ts`: state `{ projectId: string | null; exists: boolean; building: boolean; books: GrimoireBook[]; error: string | null }`, plus aksi `generate()` dan fungsi `startGrimoireStore()` / `stopGrimoireStore()` (lihat Langkah).

`index.ts`: `registerPanel({ id: 'grimoire', area: 'sidebar', component: GrimoirePanel, order: 30 })`.

### Yang dipakai

- `call`, `onEvent` dari `../../lib/ipc` (`call<T>(method, params)`, `onEvent(topicPrefix, (topic, payload) => void)` mengembalikan fungsi unsubscribe).
- `activeProjectId` (`Writable<string | null>`) dan `activeProject` (`Readable<Project | null>`, `Project = {id, path, name}`) dari `../../lib/stores`.
- `registerPanel` dari `../../lib/registry`.
- `openGrimoirePage(projectId, relPath): Promise<string>` dari `./openPage` (milik `readonly-pane`, stub dari foundation, sudah final signature-nya). Panggil tanpa menghalangi UI: `void openGrimoirePage(...).catch(...)`; galat disimpan ke `error` store.
- Method IPC (overview 2.4): `grimoire.build {projectId}` -> `{books,pages,indexBytes}`; `grimoire.status {projectId}`; `grimoire.list {projectId}` -> array (bisa `[]`); `grimoire.read {projectId, path}`. Event `grimoire.changed` dengan payload `{projectId}` (terbit saat build mulai, sukses, gagal).
- Token tema yang ada (dari `styles/theme.css` dan pola `FilesPanel.svelte`): `--bg`, `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--danger`. Tidak ada token `--bg-sidebar`; jangan menambah token global.

## Langkah

Sebelum menulis, baca `frontend/src/modules/files/tree/registerTree.ts` dan `FilesPanel.svelte` (gaya tajuk `.title`, `.header`, `.msg`, `.row`) sebagai acuan gaya. Komentar kode dalam bahasa Indonesia, identifier Inggris. Svelte 5 (runes) dipakai di repo (`$state`, `$derived`, `$effect`); ikuti itu.

### 1. `client.ts`

Impor `call` dari `../../lib/ipc`. Definisikan tipe persis seperti Kontrak. Implementasi:

```ts
export function grimoireBuild(projectId: string) { return call<GrimoireSummary>('grimoire.build', { projectId }); }
export function grimoireStatus(projectId: string) { return call<GrimoireStatus>('grimoire.status', { projectId }); }
export async function grimoireList(projectId: string) {
  const r = await call<GrimoireBook[] | null>('grimoire.list', { projectId });
  return r ?? [];            // defensif bila server mengembalikan null
}
export function grimoireRead(projectId: string, path: string) {
  return call<{ content: string; size: number }>('grimoire.read', { projectId, path });
}
```

Beri tipe return eksplisit sesuai Kontrak (`Promise<...>`). Tanpa logika lain; tidak ada panggilan `call` di berkas lain.

### 2. `store.ts`

Impor `writable`, `get` dari `svelte/store`; `onEvent` dari `../../lib/ipc`; `activeProjectId` dari `../../lib/stores`; fungsi client.

- `export interface GrimoireState { projectId: string | null; exists: boolean; building: boolean; books: GrimoireBook[]; error: string | null }`.
- `export const grimoire = writable<GrimoireState>({ projectId: null, exists: false, building: false, books: [], error: null })` (jangan ekspor `set`; ekspor hanya `subscribe` lewat objek `{ subscribe: grimoire.subscribe }` bila rapi, atau cukup ekspor writable internal sebagai `grimoireState` readonly).
- `refresh(projectId: string): Promise<void>` (internal): panggil `grimoireStatus(projectId)`; bila hasil bukan untuk project aktif saat selesai (cek `get(activeProjectId) !== projectId`), buang hasil (mencegah balapan saat ganti project cepat). Set `exists`, `building` dari status. Bila `exists` true, `grimoireList(projectId)` lalu set `books`; bila false set `books: []`. Galat: set `error` berisi `e.message` ringkas, jangan melempar.
- Penanganan balapan: simpan penghitung/token `loadSeq`; tiap `refresh` menaikkannya dan hanya hasil dengan seq terbaru yang diterapkan.
- Ganti project: saat `activeProjectId` berubah (subscribe), reset state ke `{projectId: id, exists:false, building:false, books:[], error:null}` lalu `refresh(id)` bila `id` tidak null. Bila `id` null, state kosong tanpa panggilan IPC.
- Langganan event idempoten: modul-level `let unsubEvent: (() => void) | null = null; let unsubProject: (() => void) | null = null;`.
  - `export function startGrimoireStore(): void`: bila sudah berlangganan (`unsubEvent` tidak null) langsung return. Selain itu `unsubProject = activeProjectId.subscribe(...)` dan `unsubEvent = onEvent('grimoire.changed', (_t, payload) => {...})`. Handler: ambil `projectId` dari `payload` secara aman (`typeof payload === 'object' && payload && 'projectId' in payload`), abaikan bila tidak sama dengan `get(activeProjectId)`, lalu `void refresh(projectId)`.
  - `export function stopGrimoireStore(): void`: panggil kedua unsubscribe, set ke null.
  - Dipanggil dari komponen: `$effect(() => { startGrimoireStore(); return () => stopGrimoireStore(); })`. Tidak ada subscribe di level modul saat impor (modul dimuat eager lewat `import.meta.glob`; jangan melakukan IPC saat impor).
- `export async function generate(): Promise<void>`: ambil `projectId = get(activeProjectId)`; return bila null atau `get(grimoire).building` true. Set `building: true, error: null`; `try { await grimoireBuild(projectId); } catch (e) { set error }`; `finally` panggil `await refresh(projectId)` (status server menjadi sumber kebenaran `building`; event juga datang tapi `refresh` idempoten, jadi dua refresh tidak merusak). Pastikan `building` tidak tertinggal true bila `refresh` gagal: setelah `finally`, bila project masih aktif dan refresh gagal, set `building: false`.
- Tidak ada `setInterval`/`setTimeout` polling di berkas ini.

### 3. `index.ts`

```ts
import { registerPanel } from '../../lib/registry';
import GrimoirePanel from './GrimoirePanel.svelte';

export function registerGrimoirePanel(): void {
  registerPanel({ id: 'grimoire', area: 'sidebar', component: GrimoirePanel, order: 30 });
}
registerGrimoirePanel();
```

Panggilan langsung di level modul diperlukan karena `main.ts` hanya mengimpor `modules/*/index.ts` lewat glob eager; verifikasi dengan membaca `modules/files/tree/` dan `modules/sidebar-left/index.ts` bagaimana modul lain mendaftar (bila ada modul yang memanggil fungsi register dari `index.ts` mereka, ikuti pola persis itu). Jangan mengedit `main.ts`.

### 4. `GrimoirePanel.svelte`

Script (`lang="ts"`, runes):

- Impor `grimoire`, `startGrimoireStore`, `stopGrimoireStore`, `generate` dari `./store`; `activeProject` dari `../../lib/stores`; `openGrimoirePage` dari `./openPage`; tipe `GrimoireChapter`.
- Langganan store ke state lokal runes: `let s = $state(get-or-subscribe)`; pola sederhana: `$effect(() => grimoire.subscribe((v) => { state = v; }))` ditambah `$effect(() => { startGrimoireStore(); return () => stopGrimoireStore(); })`.
- `let expanded = $state<Record<string, boolean>>({})`: status lipat di memori komponen, default terlipat (undefined = terlipat). Reset ke `{}` saat `projectId` berubah.
- `toggleBook(name)`; `openChapter(c: GrimoireChapter)`: `if (!s.projectId) return; void openGrimoirePage(s.projectId, c.path).catch(...)`; pada galat set pesan galat lokal `openError` (ditampilkan ringkas di panel; jangan menulis ke store bila tidak ada API publik untuknya).
- `rebuild()` memanggil `generate()`.

Markup (urut):

1. Elemen akar `<section class="grimoire" aria-label="Grimoire">`.
2. `.header` dengan `<span class="title">GRIMOIRE</span>` (teks literal kapital; gaya sama dengan `.title` di `FilesPanel.svelte`: `font-weight:600; text-transform:uppercase; font-size:11px; color:var(--fg-muted)`). Saat `exists && !building`, tombol kecil `Bangun ulang` (`<button class="icon" type="button" title="Bangun ulang grimoire" aria-label="Bangun ulang grimoire">`) di sisi kanan header.
3. `<!-- slot: laws (tiket 2) -->` tepat di bawah header, persis teks komentar itu, tanpa elemen.
4. Badan menurut keadaan (`{#if}` berurutan):
   - Tanpa project aktif (`!$activeProject` atau `!s.projectId`): `<div class="msg">Pilih project</div>`.
   - `s.building`: `<div class="msg" role="status">Membangun...</div>` dan tombol Generate (bila ditampilkan) `disabled`. Bila belum ada grimoire, tampilkan tombol Generate nonaktif di bawah status; bila sudah ada (bangun ulang), tampilkan daftar lama tetap terlihat dan status di atasnya.
   - `!s.exists`: tombol `<button class="generate" type="button" onclick={generate}>Generate Grimoire</button>`.
   - `s.exists` dengan `s.books.length === 0`: `<div class="msg">Grimoire kosong</div>` (project tanpa modul tetap sah).
   - `s.exists`: daftar buku. Tiap buku: `<button class="book" type="button" aria-expanded={!!expanded[b.name]} onclick={() => toggleBook(b.name)}>` berisi panah (`&#9656;`/`&#9662;`), nama buku (`.name`, ellipsis), jumlah bab; bila `expanded`, `<ul class="chapters">` dengan `<li>` berisi `<button class="chapter" type="button" onclick={() => openChapter(c)} title={c.path}>` + nama bab (`.name`, ellipsis) + penanda `stale` (`<span class="stale" title="Usang">!</span>`) hanya bila `c.stale`.
5. Galat: bila `s.error` atau `openError`, `<div class="msg err" role="alert">` dengan teks dipotong ellipsis (`white-space:nowrap; overflow:hidden; text-overflow:ellipsis`, atribut `title` berisi teks penuh).
6. `<!-- slot: stale/llm (tiket 3, 4) -->` di bagian bawah akar, persis teks komentar itu, tanpa elemen.

Gaya (`<style>` scoped, tanpa token baru):

- `.grimoire { height:100%; display:flex; flex-direction:column; font-size:12px; color:var(--fg, inherit); background: color-mix(in srgb, var(--bg, #1b1b1f) 88%, black); border-top: 1px solid var(--border, transparent); min-height:0; }` (latar sedikit lebih gelap dari sidebar, turunan token `--bg`).
- `.header { display:flex; align-items:center; justify-content:space-between; padding:8px 12px; flex:none; }`.
- `.book`, `.chapter`, `.generate`: reset `button` (`background:none; border:none; color:inherit; font:inherit; text-align:left; cursor:pointer; width:100%`); tinggi baris 28px; `display:flex; align-items:center; gap:4px; overflow:hidden`; `:hover`/`:focus-visible` memakai `var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent))` dan outline `1px solid var(--accent, currentColor)` dengan `outline-offset:-1px`. `.chapter` diberi `padding-left:26px`; `.book` `padding-left:12px`. `.name { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap }`.
- `.generate { margin:8px 12px; width:auto; justify-content:center; border:1px solid var(--border, currentColor); border-radius:4px; background:var(--bg-elev, transparent); } .generate:disabled { opacity:.6; cursor:default; }`.
- `.stale { color: var(--danger, inherit); flex:none; }`; `.err { color: var(--danger, inherit); }`; `.msg { padding:12px; color: var(--fg-muted, inherit); }`.
- `.chapters { list-style:none; margin:0; padding:0; }`; daftar buku scrollable: bungkus dalam `<div class="list" style>` dengan `flex:1; overflow:auto; min-height:0`.
- Keyboard: semua item adalah `<button>` sehingga Tab/Enter/Space bekerja; tidak perlu `role` tambahan.

### 5. Verifikasi

- `cd frontend && npm run check && npm run build`. Perbaiki hanya galat di keempat berkasmu. Bila galat berasal dari berkas fitur lain atau stub (mis. `openPage.ts` belum ada), jangan memperbaikinya; laporkan.
- `git diff --stat -- go.mod go.sum frontend/package.json frontend/package-lock.json` kosong.
- `git status` menunjukkan hanya empat berkas di `frontend/src/modules/grimoire/` (ditambah `openPage.ts` dari foundation/`readonly-pane` bila ada) yang berubah karena pekerjaanmu.
- `grep -n "setInterval\|setTimeout" frontend/src/modules/grimoire/{store.ts,GrimoirePanel.svelte,client.ts,index.ts}` kosong.
- `grep -n "call(" frontend/src/modules/grimoire/*.ts*` hanya menemukan `client.ts` (dan `openPage.ts` bila ada; itu bukan milikmu).

## Perubahan

Dibuat (baru):

- `frontend/src/modules/grimoire/client.ts`
- `frontend/src/modules/grimoire/store.ts`
- `frontend/src/modules/grimoire/GrimoirePanel.svelte`
- `frontend/src/modules/grimoire/index.ts`

Tidak ada berkas yang diubah di luar empat path ini.

## Wiring

Yang diekspos untuk langkah wiring (tanpa disambungkan sendiri):

- `modules/grimoire/index.ts` mendaftarkan panel `grimoire` (area `sidebar`, `order: 30`) lewat `registerPanel`; dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts', { eager: true })` di `main.ts` (tidak diedit).
- `GrimoirePanel.svelte` mengimpor `openGrimoirePage` dari `./openPage`; implementasi nyata disediakan `readonly-pane` (menimpa stub foundation). Nama method IPC di `client.ts` harus identik dengan yang didaftarkan `ipc-ctl`: `grimoire.build`, `grimoire.status`, `grimoire.list`, `grimoire.read`; event `grimoire.changed` dengan payload `{projectId}`.
- Dua slot komentar di `GrimoirePanel.svelte` (`<!-- slot: laws (tiket 2) -->`, `<!-- slot: stale/llm (tiket 3, 4) -->`) sebagai titik tambah tiket berikutnya.

Yang TIDAK dilakukan: tidak mengedit `AgentSidebar`, `main.ts`, `registry.ts`, atau `openPage.ts`.

## Selesai bila

- Keempat berkas ada, hanya di `frontend/src/modules/grimoire/`, dengan tipe dan signature client persis seperti Kontrak.
- Panel terdaftar `id: 'grimoire'`, `area: 'sidebar'`, `order: 30`; tajuk **GRIMOIRE**; latar diturunkan dari token tema (`color-mix` atas `--bg`) tanpa token global baru.
- Keadaan tampil benar: "Pilih project", tombol Generate Grimoire, "Membangun..." dengan tombol nonaktif, daftar buku terlipat dengan bab, tombol "Bangun ulang"; klik bab memanggil `openGrimoirePage(projectId, chapter.path)` tanpa `await` yang memblokir dan galatnya ditampilkan ringkas.
- Store mengikuti `activeProjectId`, memuat ulang lewat `grimoire.changed` hanya bila `projectId` payload sama dengan project aktif, langganan idempoten dan dibersihkan, tanpa polling, hasil usang diabaikan.
- Dua komentar slot hadir persis.
- Guard: `cd frontend && npm run check && npm run build` sukses untuk path fitur ini (galat dari berkas fitur lain hanya dilaporkan); tidak ada perubahan dependency; tanpa unit test.
