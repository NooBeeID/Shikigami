# Plan: Toggle LLM, estimasi, dan biaya di panel

## Metadata

- Tiket: `grimoire-ticket-04-grimoire-llm` (`docs/tickets/grimoire-ticket-04-grimoire-llm.md`)
- Fitur: `llm-ui` (Toggle LLM, estimasi, dan biaya di panel)
- ownedPaths: `frontend/src/modules/grimoire/llm/`
- Depends on foundation: tidak (foundation tiket ini hanya stub Go; fitur ini murni frontend, kontraknya hanya IPC dan boleh di-mock)
- Dokumen acuan (baca, jangan diubah): `overview.md` (bagian 2.2 butir 6 dan 8, 2.4, 2.7), tiket, spec desain grimoire.

## Tujuan

Menyediakan UI opt-in ringkasan LLM untuk panel Grimoire sebagai satu komponen tunggal `LlmSettings.svelte` yang nanti ditanam oleh langkah Wiring:

- toggle "Ringkasan LLM" (default mati); menyalakan harus lewat pratinjau estimasi "N modul, ~T token" dan konfirmasi, mematikan langsung;
- input `maxModules` (kosong = tanpa batas, harus integer >= 1);
- progres per modul saat build berjalan, digerakkan hanya oleh event `grimoire.changed`;
- baris biaya build terakhir dan catatan mode statik.

Tanpa polling/timer, tanpa dependency baru, memakai token tema yang sudah ada. Daemon lama yang belum mengirim field baru dianggap mati dan UI tetap render.

## Path yang dimiliki

HANYA `frontend/src/modules/grimoire/llm/` (folder baru; semua berkas baru di dalamnya).

Dilarang menyentuh: `GrimoirePanel.svelte`, `client.ts`, `store.ts`, `openPage.ts`, `index.ts`, folder `laws/` dan `stale/`, berkas Go, `package.json`/lockfile, `lib/ipc.ts`, `lib/stores/`, dan path fitur lain (`settings-estimate`, `llm-summarizer`). Jangan menanam komponen ke panel (itu Wiring).

## Kontrak

Dipakai (overview 2.4; jangan diubah, panggil langsung lewat `call` tanpa mengedit `client.ts`):

| Method | Params | Hasil |
|---|---|---|
| `grimoire.settings.get` | `{projectId}` | `{llm:boolean, maxModules:number\|null}` |
| `grimoire.settings.set` | `{projectId, llm?, maxModules?}` | settings terbaru; `maxModules: null` eksplisit = hapus batas; `maxModules < 1` -> galat `invalid` |
| `grimoire.estimate` | `{projectId}` | `{modules, estTokens, overLimit?}` (tanpa LLM, tanpa menulis) |
| `grimoire.status` | `{projectId}` | field lama (`exists`, `building`, ...) + aditif `llmEnabled?`, `lastBuildCost?: {modules,estTokens,durationMs,failed,at,skipped?,note?}\|null`, `llmProgress?: {current,done,failed}\|null` |

Event: `grimoire.changed` dengan payload `{projectId}` (terbit saat settings berubah dan di awal/akhir tiap modul diringkas). Satu-satunya pemicu pembaruan.

Impor yang dipakai: `call`, `onEvent` dari `../../../lib/ipc` (signature: `call<T>(method, params?) => Promise<T>`, `onEvent(topicPrefix, (topic, payload) => void) => () => void`). Tidak perlu `activeProjectId`: project datang lewat prop `projectId`.

Disediakan (overview 2.7, nama persis):

```ts
// llmStore.ts
export interface LlmCost { modules: number; estTokens: number; durationMs: number; failed: number; at: string; skipped?: number; note?: string }
export interface LlmProgress { current: string; done: number; failed: number }
export interface LlmEstimate { modules: number; estTokens: number; overLimit?: number }
export interface LlmState {
  projectId: string | null; loaded: boolean;
  llm: boolean; maxModules: number | null;
  building: boolean; progress: LlmProgress | null; lastBuildCost: LlmCost | null;
  estimate: LlmEstimate | null; busy: boolean; error: string | null;
}
export const llmState: Readable<LlmState>
export function startLlmStore(projectId: string): () => void
export function requestEnable(): Promise<void>
export function confirmEnable(): Promise<void>
export function cancelEnable(): void
export function disableLlm(): Promise<void>
export function setMaxModules(v: number | null): Promise<void>
export function formatTokens(n: number): string
export function formatDuration(ms: number): string
```

Komponen: `LlmSettings.svelte` props `{ projectId: string | null; exists: boolean }`; `LlmProgress.svelte` props `{ building: boolean; progress: LlmProgress | null }`; `LlmCost.svelte` props `{ cost: LlmCost | null }`.

Catatan: nama tipe `LlmProgress` dipakai oleh interface di `llmStore.ts` dan nama komponen `LlmProgress.svelte`. Di `LlmSettings.svelte` impor komponen sebagai `LlmProgressView` (`import LlmProgressView from './LlmProgress.svelte'`) dan tipe dengan `import type`, agar tidak bentrok. Sama untuk `LlmCost` (`LlmCostView`). Di `LlmProgress.svelte`/`LlmCost.svelte` sendiri, impor tipe dengan alias (`import type { LlmProgress as LlmProgressData }`).

## Langkah

Pelajari dulu pola nyata: `frontend/src/modules/grimoire/stale/staleStore.ts` (store + `seq` guard + `startXStore` mengembalikan fungsi berhenti), `stale/StaleSlot.svelte` dan `stale/BuildIndicator.svelte` (props runes, gaya, token tema), `laws/`. Ikuti gayanya (tipe longgar `Raw*`, galat dipotong 200 karakter, `$effect` mengembalikan fungsi berhenti).

### 1. `llmStore.ts`

Tanpa efek samping saat impor modul (hanya `writable` dan variabel modul).

State dan tipe:
- `import { writable, get, type Readable } from 'svelte/store'`; `import { call, onEvent } from '../../../lib/ipc'`.
- Ekspor tiga interface bagian Kontrak dan `LlmState`. `initial(projectId: string | null): LlmState` = `{projectId, loaded:false, llm:false, maxModules:null, building:false, progress:null, lastBuildCost:null, estimate:null, busy:false, error:null}`.
- `const state = writable<LlmState>(initial(null))`; `export const llmState: Readable<LlmState> = { subscribe: state.subscribe }`.
- Variabel modul: `let currentId: string | null = null; let seq = 0;`.
- `msg(e: unknown): string` (pola `staleMsg`: pesan galat dipotong 200 karakter + `...`).

Tipe mentah (longgar, semua `unknown`): `RawSettings = { llm?: unknown; maxModules?: unknown }`, `RawStatus = { building?: unknown; llmEnabled?: unknown; lastBuildCost?: unknown; llmProgress?: unknown }`.

Normalizer murni (tidak mengekspor kecuali perlu):
- `asMaxModules(v: unknown): number | null` -> `Number.isInteger(v) && v >= 1 ? v : null`.
- `asCost(v: unknown): LlmCost | null`: bukan objek non-null -> `null` (termasuk `lastBuildCost` bertipe `number` dari daemon lama/placeholder 0, per overview 2.2 butir 6). Angka (`modules`, `estTokens`, `durationMs`, `failed`) yang bukan number -> 0; `at` bukan string -> `''`; `skipped` hanya disertakan bila number > 0; `note` hanya bila string tak kosong.
- `asProgress(v: unknown): LlmProgress | null`: bukan objek -> `null`; `current` string (default `''`), `done`/`failed` number (default 0).

Pemuatan:
- `async function load(id: string, withSettings: boolean): Promise<void>`: `const my = ++seq`; panggil `call<RawStatus|null>('grimoire.status', {projectId:id})` dan, bila `withSettings`, `call<RawSettings|null>('grimoire.settings.get', {projectId:id})` secara paralel (`Promise.allSettled` agar status yang sukses tetap dipakai walau settings gagal dan sebaliknya). Setelah await, bila `my !== seq || currentId !== id` -> buang hasil. Terapkan: `building = status?.building === true`; `progress = asProgress(status?.llmProgress)`; `lastBuildCost = asCost(status?.lastBuildCost)`; `llm`: bila settings berhasil -> `settings?.llm === true`, `maxModules = asMaxModules(settings?.maxModules)`; bila settings tidak diminta atau gagal, jangan timpa `llm`/`maxModules` yang ada, kecuali `llmEnabled` dari status adalah boolean dan settings tidak tersedia (fallback awal: `llm = status.llmEnabled === true`). Selalu `loaded: true`. Galat: bila KEDUA panggilan gagal, set `error: msg(...)` dan `loaded: true`; bila hanya satu gagal, tetap tampilkan data yang ada dan catat galatnya di `error`. Sukses penuh -> `error: null`. Jangan menyentuh `estimate`, `busy` (dikelola aksi).
- `export function startLlmStore(projectId: string): () => void`: `currentId = projectId; seq++; state.set(initial(projectId)); void load(projectId, true);` lalu `const off = onEvent('grimoire.changed', (_t, payload) => {...})` dengan penjagaan bentuk payload persis seperti `staleStore.ts` (objek non-null dengan `projectId` string yang sama dengan `currentId`, selain itu abaikan) lalu `void load(pid, true)` (event dipakai untuk status DAN settings karena `settings.set` juga mempublikasikannya). Kembalikan fungsi berhenti: `off()`; bila `currentId === projectId` -> `currentId = null; seq++; state.set(initial(null))`. Tanpa `setInterval`/`setTimeout`.

Aksi (semua menolak bekerja bila `currentId` null; simpan `const id = currentId` di awal dan cek `currentId === id` sebelum menulis state sesudah `await`):
- `requestEnable()`: abaikan bila `busy` atau `llm` sudah true. `busy: true, error: null`; `call<LlmEstimateRaw>('grimoire.estimate', {projectId:id})`; normalisasi ke `LlmEstimate` (`modules`/`estTokens` number default 0; `overLimit` hanya bila number > 0); `state.update(... estimate, busy:false)`. Galat -> `error: msg(e)`, `busy:false`, `estimate` tetap null (tidak bisa menyalakan tanpa estimasi). Tidak menyimpan flag.
- `confirmEnable()`: abaikan bila `estimate` null atau `busy`. `busy:true`; `call('grimoire.settings.set', {projectId:id, llm:true})`; sukses -> `llm = res?.llm === true` (fallback `true` bila respons tak berbentuk), `maxModules` dari respons bila ada kunci, `estimate:null`, `busy:false`, `error:null`. Galat -> `error: msg(e)`, `busy:false`, `estimate` DIPERTAHANKAN (agar bisa coba lagi atau batal). Event `grimoire.changed` yang menyusul akan menyegarkan state.
- `cancelEnable()`: `state.update(s => ({...s, estimate:null}))` (sinkron, tanpa IPC).
- `disableLlm()`: `busy:true`; `settings.set {projectId:id, llm:false}`; sukses -> `llm:false`, `estimate:null`, `busy:false`; galat -> `error`, `busy:false`. Tanpa konfirmasi.
- `setMaxModules(v: number | null)`: bila `v !== null && (!Number.isInteger(v) || v < 1)` -> `state.update(error: 'maxModules harus integer >= 1')` dan RETURN tanpa IPC. Selain itu `call('grimoire.settings.set', {projectId:id, maxModules: v})` (untuk `null` kirim `maxModules: null` secara eksplisit, jangan buang kuncinya). Sukses -> `maxModules` dari respons (`asMaxModules`), `error:null`. Galat -> `error: msg(e)`. Tidak memakai `busy` global agar toggle tidak terkunci saat mengetik (boleh `busy` ringan; putuskan sederhana: jangan set `busy`).

Format:
- `formatTokens(n: number): string`: `n < 1000` -> `String(Math.round(n))`; `< 1_000_000` -> `${(n/1000).toFixed(n < 10_000 ? 1 : 0)}k` (buang `.0`: `1.0k` -> `1k`); selain itu `${(n/1e6).toFixed(1)}M`. Nilai tidak finite/negatif -> `'0'`.
- `formatDuration(ms: number): string`: `< 1000` -> `${Math.round(ms)} ms`; `< 60_000` -> `${(ms/1000).toFixed(1)} dtk`; selain itu `${m} mnt ${s} dtk` (m = lantai menit, s = sisa detik, bulatkan). Tidak finite/negatif -> `'0 ms'`.

### 2. `LlmProgress.svelte`

Props runes: `let { building, progress }: { building: boolean; progress: LlmProgressData | null } = $props();` (tipe diimpor dengan alias). Render hanya bila `building && progress`:

```svelte
<span class="llm-progress" role="status" aria-live="polite"><span class="pulse" aria-hidden="true"></span>
  {#if progress.current}Meringkas {progress.current} ({progress.done} selesai{#if progress.failed > 0}, {progress.failed} gagal{/if}){:else}Meringkas modul ({progress.done} selesai...){/if}
</span>
```

Gaya: `font-size: 11px; color: var(--fg-muted, inherit)`, titik `.pulse` seperti `stale/BuildIndicator.svelte` (animasi dengan nama keyframes unik `llm-pulse`, matikan di `prefers-reduced-motion`). `.current` dipotong dengan ellipsis (`min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap`). Tidak ada timer.

### 3. `LlmCost.svelte`

Props: `let { cost }: { cost: LlmCostData | null } = $props();`. Impor `formatTokens`, `formatDuration` dari `./llmStore`. Render hanya bila `cost` non-null:
- Baris utama: `Build terakhir: {cost.modules} modul, ~{formatTokens(cost.estTokens)} token, {formatDuration(cost.durationMs)}` + `, {cost.failed} gagal` bila `failed > 0` + `, {cost.skipped} dilewati` bila `skipped` terisi. Beri `title` berisi `cost.at` (waktu RFC3339) bila tidak kosong. Kata "token" berarti perkiraan: tulis `~` dan beri `title` "Perkiraan, bukan tagihan".
- Bila `cost.note`: baris kedua `{cost.note}` (mis. mode statik karena claude tidak ada/belum login), gaya `var(--fg-muted)`; bila `cost.failed > 0` baris utama bagian gagal berwarna `var(--danger, inherit)`.
- Gaya: `font-size: 11px; color: var(--fg-muted, inherit)`; teks panjang boleh membungkus (`overflow-wrap: anywhere`). Tanpa warna keras.

### 4. `LlmSettings.svelte`

Props: `let { projectId, exists }: { projectId: string | null; exists: boolean } = $props();`. `exists` hanya untuk keterangan tambahan; toggle TIDAK bergantung padanya (settings bisa diatur sebelum Generate). Boleh dipakai untuk menampilkan hint kecil "Generate dulu untuk memakai ringkasan" bila `!exists && s.llm` (opsional, satu baris teks muted).

Struktur skrip:
- `let s = $state<LlmState>(...initial lokal setara...)`; `$effect(() => llmState.subscribe((v) => { s = v; }));` (pola `StaleSlot`).
- `$effect(() => { if (!projectId) return; const stop = startLlmStore(projectId); return stop; });` (berhenti saat project berganti/null/unmount).
- `let draft = $state('')` untuk input maxModules; sinkronkan dari store tanpa menimpa ketikan: `let editing = $state(false)`; `$effect(() => { if (!editing) draft = s.maxModules === null ? '' : String(s.maxModules); });`.
- `function onToggle()`: bila `s.busy` return; `s.llm ? void disableLlm() : void requestEnable()`. Toggle adalah `<button type="button" role="switch" aria-checked={s.llm}>` (bukan checkbox bawaan, agar keadaan visual selalu mengikuti store, bukan klik optimistis). Selama `s.estimate` non-null (menunggu konfirmasi) saklar tetap tampil mati.
- `function commitMax()`: `editing = false`; `const t = draft.trim()`; kosong -> `void setMaxModules(null)` hanya bila `s.maxModules !== null`; selain itu `const n = Number(t)`; jika `!Number.isInteger(n) || n < 1` -> panggil `setMaxModules(n)` agar store mengisi `error` dan tidak memanggil IPC, lalu biarkan `draft` apa adanya agar pengguna memperbaiki; jika valid dan `n !== s.maxModules` -> `void setMaxModules(n)`. Dipicu `onchange`/`onblur` dan `Enter` (`onkeydown` -> `commitMax()`; `Escape` -> `editing = false`, kembalikan `draft` dari store). Input: `<input type="number" min="1" step="1" inputmode="numeric" placeholder="tanpa batas" aria-label="Batas modul per build">`, `onfocus={() => editing = true}`.
- Cabang tampilan, semua di dalam `{#if projectId}` (kosong total bila null):
  1. Baris saklar: label "Ringkasan LLM" + saklar; `disabled` bila `s.busy` atau `!s.loaded`.
  2. Bila `s.estimate`: kotak pratinjau (`role="group" aria-label="Pratinjau estimasi"`): teks `{estimate.modules} modul, ~{formatTokens(estimate.estTokens)} token`; bila `overLimit` > 0 tambah `({overLimit} modul di atas batas dilewati)`; bila `estimate.modules === 0` teks `Tidak ada modul antrean saat ini` (tetap boleh dikonfirmasi). Dua tombol: "Konfirmasi" -> `confirmEnable()`, "Batal" -> `cancelEnable()`; keduanya `disabled` saat `s.busy`. Tambah catatan muted: "Perkiraan; biaya nyata bergantung pada claude".
  3. Baris `maxModules`: label "Batas modul" + input; hint muted "kosong = tanpa batas".
  4. `<LlmProgressView building={s.building} progress={s.progress} />`.
  5. `<LlmCostView cost={s.lastBuildCost} />`.
  6. Bila `s.error`: `<div class="err" role="alert" title={s.error}>{s.error}</div>` (ellipsis satu baris seperti `.err` panel).
- Gaya: wadah `padding: 4px 12px; display:flex; flex-direction:column; gap:6px; font-size:12px; flex:none`. Saklar: tombol 28x16 dengan `border: 1px solid var(--border, currentColor)`, latar `var(--bg-elev, transparent)`, knob bulat; aktif memakai `var(--accent, currentColor)`; `:hover`/`:focus-visible` memakai `var(--bg-hover, ...)` dan outline `var(--accent, currentColor)` seperti `.chapter` di panel; `disabled` opacity 0.6. Kotak pratinjau: `border: 1px solid var(--border, currentColor); border-radius: 4px; background: var(--bg-elev, transparent); padding: 8px`. Tombol aksi mengikuti gaya `.generate` panel (border, radius 4px, tinggi 24-28px). Input: `width: 72px; background: var(--bg-elev, transparent); color: inherit; border: 1px solid var(--border, currentColor); border-radius: 4px; font: inherit; padding: 2px 6px`. Hanya token yang tercantum di overview 2.7; tanpa token global baru, tanpa warna keras (selain fallback bawaan `var(--x, ...)`).
- Layar sempit: baris label+kontrol memakai `display:flex; gap:8px; align-items:center; justify-content:space-between; min-width:0`.

### 5. Verifikasi (guard)

Tanpa unit test. Jalankan dari `frontend/`: `npm run check` dan `npm run build`. Pastikan nol galat/warning baru di berkas `llm/`. Cek `grep -rnE "setInterval|setTimeout|requestAnimationFrame" frontend/src/modules/grimoire/llm` kosong, `git status` hanya menunjukkan berkas baru di `frontend/src/modules/grimoire/llm/`, dan `git diff --stat -- frontend/package.json frontend/package-lock.json` kosong. Bila `check`/`build` gagal karena berkas di luar `llm/`, jangan diperbaiki; laporkan. Karena komponen belum ditanam, `svelte-check` tetap memeriksa berkas ini; bila ingin uji visual cepat tanpa menyentuh berkas lain, jangan membuat berkas tiruan di luar ownedPaths (mock hanya di kepala/dalam sesi, tidak di-commit).

## Perubahan

Dibuat (semua baru, di `frontend/src/modules/grimoire/llm/`):
- `llmStore.ts`
- `LlmSettings.svelte`
- `LlmProgress.svelte`
- `LlmCost.svelte`

Tidak ada berkas yang diubah.

## Wiring

Yang diekspos untuk langkah Wiring (overview bagian 5 butir 6; JANGAN disambungkan oleh fitur ini):
- Satu komponen: `frontend/src/modules/grimoire/llm/LlmSettings.svelte` dengan props `{ projectId: string | null; exists: boolean }`.
- Cara pasang di `GrimoirePanel.svelte`: tambah `import LlmSettings from './llm/LlmSettings.svelte';` dan ganti komentar slot llm dengan `<LlmSettings projectId={s.projectId} exists={s.exists} />` (nama variabel state nyata panel). Komponen mengelola langganan event dan siklus hidup sendiri; panel tidak perlu memanggil `startLlmStore`.
- `llmStore.ts` mandiri (tipe sendiri, tidak mengimpor `client.ts`/`store.ts`); tidak butuh perubahan `client.ts`.
- Bergantung pada backend Wiring: field aditif `llmEnabled`, `lastBuildCost` (objek atau null), `llmProgress` pada `grimoire.status`. Bila belum ada, UI menganggap mati/tanpa progres/tanpa biaya dan tetap render.

## Selesai bila

- Keempat berkas ada di `frontend/src/modules/grimoire/llm/` dan tidak ada perubahan di luar folder itu.
- Signature `llmStore.ts` dan props komponen persis seperti bagian Kontrak.
- Perilaku: toggle mati -> klik memanggil `grimoire.estimate` dan menampilkan pratinjau; flag baru disimpan (`settings.set {llm:true}`) hanya setelah Konfirmasi; Batal menutup pratinjau tanpa IPC set; mematikan langsung `settings.set {llm:false}`; `maxModules` < 1 atau bukan integer ditolak tanpa IPC dan memunculkan galat; kosong mengirim `maxModules: null`; progres hanya tampil saat `building` dan `progress` non-null; biaya tersembunyi bila `lastBuildCost` null (atau bukan objek); pergantian/penghapusan project menghentikan langganan dan memuat ulang data project baru; pembaruan hanya dari `grimoire.changed` (tanpa timer/polling).
- Guard: `cd frontend && npm run check && npm run build` sukses tanpa galat dari berkas `llm/`; tanpa dependency baru; tanpa unit test.
