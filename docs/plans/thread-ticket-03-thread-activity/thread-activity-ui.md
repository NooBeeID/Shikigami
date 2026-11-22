# Plan: Kartu aktivitas buka-tutup per agent

## Metadata

- Tiket: `thread-ticket-03-thread-activity` (`docs/tickets/thread-ticket-03-thread-activity.md`)
- Fitur: `thread-activity-ui`
- ownedPaths: `frontend/src/modules/thread/activity/`
- Depends on foundation: ya (`00-foundation.md`: field opsional `count?`/`items?` pada `ThreadMessage` di `frontend/src/modules/thread/types.ts`). Bila saat implementasi field itu belum ada di `types.ts`, JANGAN mengubah `types.ts`; hentikan dan laporkan bahwa Foundation belum selesai.
- Rujukan kontrak: `docs/plans/thread-ticket-03-thread-activity/overview.md` bagian 2.2 (no. 4, 5, 7, 8) dan 2.4. Jangan mengubah overview.

## Tujuan

Menampilkan pesan `kind=activity` di aliran thread sebagai satu kartu buka-tutup per rentetan aksi satu agent ("<nama>: N aksi", status "mengerjakan"/"selesai", penanda warna agent, default tertutup, daftar aksi satu baris dengan ellipsis dan `title`). Pesan diskusi dan sistem tidak pernah dilipat. Kartu sepenuhnya diturunkan dari daftar `ThreadMessage` (state yang disimpan hanya buka/tutup). Fitur ini hanya menyediakan modul; penyambungan ke `MessageList.svelte` dilakukan langkah Wiring.

## Path yang dimiliki

HANYA boleh membuat/mengubah berkas di `frontend/src/modules/thread/activity/`:

- `groupActivity.ts`
- `ActivityCard.svelte`
- `ActivityFlowItem.svelte`
- `index.ts`

Dilarang menyentuh: `MessageList.svelte`, `MessageItem.svelte`, `thread-store.ts`, `types.ts`, `format.ts`, `index.ts` modul thread (induk), berkas apa pun di `internal/`, `cmd/`, dan path fitur lain (`internal/thread/activity.go` milik `thread-activity-daemon`). Butuh perubahan di luar folder: jangan dikerjakan, catat di "Wiring/Catatan".

## Kontrak

Dipakai (sudah ada di kode tiket 01; verifikasi nama persis dulu):

- `import type { Thread, ThreadMessage } from '../types'` (`ThreadMessage` punya `count?: number; items?: string[]` berkat Foundation).
- `import { participantName } from '../thread-store'` dengan signature `participantName(thread: Thread | undefined, id: string): string`.
- `import { avatarColor, formatTime } from '../format'`: `avatarColor(key: string): string` (kembalian `hsl(...)`), `formatTime(ts: string): string`.
- Format pesan activity dari daemon (overview 2.3): `kind='activity'`, `from=<sessionId>`, `text='<nama>: <Tool> <target>'`, `count`, `items` (tanpa awalan nama, maks 50 terbaru), `ts` = waktu aksi terakhir, id stabil selama penggabungan.
- Token tema seperti `MessageItem.svelte`: `var(--fg, #222)`, `var(--fg-muted, #888)`, `var(--bg-elev, #fff)`, `var(--border, #8884)`.

Disediakan (signature final, jangan diubah; sesuai overview 2.4):

```ts
// groupActivity.ts
export const ACTIVITY_RUNNING_MS = 30_000;
export interface ActivityGroup {
  type: 'activity';
  key: string;        // id pesan activity pertama di grup
  from: string;
  count: number;
  items: string[];
  lastTs: string;
  last: boolean;      // tidak ada item aliran lain sesudah grup
  finished: boolean;  // setelah grup ada pesan kind='message' dari agent yang sama
  messages: ThreadMessage[];
}
export interface PlainFlowItem { type: 'message'; key: string; message: ThreadMessage }
export type FlowItem = ActivityGroup | PlainFlowItem;
export function groupActivity(messages: ThreadMessage[], names?: (id: string) => string): FlowItem[];
export function isRunning(group: ActivityGroup, now: number, followedByOwnMessage: boolean, threadClosed: boolean): boolean;
```

```ts
// ActivityCard.svelte props
{ agentName: string; color: string; items: string[]; count: number; running: boolean }
// ActivityFlowItem.svelte props
{ thread: Thread; group: ActivityGroup }
```

`index.ts` mengekspor: `ActivityFlowItem` (default Svelte), `groupActivity`, `ACTIVITY_RUNNING_MS`, `isRunning`, dan tipe `ActivityGroup`, `PlainFlowItem`, `FlowItem`.

## Langkah

### 1. `groupActivity.ts` (fungsi murni, tanpa import Svelte)

1. Ekspor `ACTIVITY_RUNNING_MS = 30_000` dan tipe seperti bagian Kontrak.
2. Helper tak-diekspor `activityItemsOf(m: ThreadMessage, names?: (id: string) => string): string[]`:
   - bila `m.items` ada dan panjang > 0 -> salinan `m.items` (apa adanya);
   - bila tidak: ambil `m.text`, ganti newline dengan spasi, `trim`; HANYA buang awalan yang tepat sama dengan `names(m.from) + ': '` (bila `names` diberikan dan teks diawali itu); jika `names` tidak ada atau awalan tak cocok, pakai teks utuh. Teks kosong -> `[]`.
3. Helper tak-diekspor `activityCountOf(m, items)`: `Math.max(1, m.count ?? items.length)` (jika `count` bukan angka valid > 0 pakai `Math.max(1, items.length)`).
4. `groupActivity(messages, names)`:
   - Dedup di awal: `Map<string, ThreadMessage>` per `id`; versi TERAKHIR menang tetapi POSISI urut mengikuti kemunculan pertama id itu (iterasi, simpan indeks pertama; ganti nilai pada indeks itu). Tidak mengurutkan ulang (store tiket 01 sudah mengurutkan menurut `ts`).
   - Iterasi berurutan membangun `FlowItem[]`: pesan non-activity -> `{ type: 'message', key: m.id, message: m }`. Pesan `kind==='activity'`: bila item aliran terakhir adalah `ActivityGroup` dengan `from` sama -> tambahkan ke grup itu (`messages.push`, `items.push(...activityItemsOf)`, `count += activityCountOf`, `lastTs = m.ts`); selain itu mulai grup baru (`key = m.id`, `last=false`, `finished=false`). Item aliran lain di antaranya (message/system/activity agent lain) otomatis memutus karena tidak lagi menjadi item terakhir.
   - Setelah selesai, hitung per grup: `last = (indeks grup === flow.length - 1)`; `finished = ada item sesudahnya bertipe 'message' dengan message.kind === 'message' && message.from === group.from` (pindai dari indeks grup+1 ke akhir; O(n) per grup dapat diterima, atau satu lintasan mundur dengan `Set<string>` berisi `from` pesan diskusi yang sudah terlihat).
   - Kembalikan array.
5. `isRunning(group, now, followedByOwnMessage, threadClosed)`: `!threadClosed && !followedByOwnMessage && !group.finished && (now - Date.parse(group.lastTs)) < ACTIVITY_RUNNING_MS`; bila `Date.parse` NaN -> `false`. Catatan: `followedByOwnMessage` disediakan agar kompatibel dengan signature overview; `ActivityFlowItem` mengirim `group.finished` untuk argumen ini.
6. Ekspor juga helper kecil `activityRemainingMs(group, now): number` (`Math.max(0, ACTIVITY_RUNNING_MS - (now - Date.parse(group.lastTs)))`, NaN -> 0) untuk penjadwalan timer; hanya dipakai di folder ini.

### 2. `ActivityCard.svelte`

Svelte 5 runes, `<script lang="ts">`. Props: `{ agentName, color, items, count, running }` via `$props()`.

- State buka/tutup `let open = $state(false)` (default tertutup). Tombol header `<button type="button" class="head" aria-expanded={open} aria-controls={listId} onclick={toggle}>`; `listId` unik per instans (`` `act-${Math.random().toString(36).slice(2, 9)}` ``). Elemen `button` memberi dukungan Enter/Space bawaan keyboard.
- Persistensi opsional: komponen ini tidak tahu `key`, jadi persistensi `localStorage` dilakukan oleh `ActivityFlowItem` (langkah 3) atau dilewati; ActivityCard cukup mendukung prop opsional `open?: boolean` dan `onToggle?: (open: boolean) => void` (ditambahkan sebagai prop opsional, tidak merusak kontrak) bila perlu dikendalikan. Sederhana: gunakan `$bindable` tidak; cukup state lokal + callback `onToggle` opsional yang dipanggil tiap toggle, dan prop `initialOpen?: boolean` (default `false`) sebagai nilai awal.
- Header berisi: titik warna (`<span class="dot" style:background={color} aria-hidden="true">`), label `{agentName}: {count} aksi`, status `<span class="status" class:run={running}>{running ? 'mengerjakan' : 'selesai'}</span>` (pakai `role="status"` TIDAK dipakai agar tidak berisik), dan caret (`aria-hidden`, `▸`/`▾`).
- Daftar saat `open`: `<ul id={listId} class="items">` dengan `{#each items as it, i (i)}<li class="item" title={it}>{it}</li>{/each}`. Tiap `li` satu baris: `white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0`. Bila `items` kosong tampilkan `<li class="item empty">Tidak ada rincian</li>`. Bila `count > items.length` tampilkan baris pertama `<li class="more">… {count - items.length} aksi lebih lama tidak ditampilkan</li>` (items dibatasi 50 terbaru oleh daemon).
- Gaya (scoped): kontainer `.card { border: 1px solid var(--border, #8884); border-radius: 6px; background: var(--bg-elev, #fff); min-width: 0; max-width: 100%; overflow: hidden; margin: 2px 8px }`; header `display:flex; align-items:center; gap:8px; width:100%; font: inherit; font-size: 12px; color: var(--fg, #222); background: none; border: 0; padding: 6px 8px; cursor: pointer; text-align:left; min-width:0`; label `flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-weight:600`; status `font-size:11px; color: var(--fg-muted, #888)`, saat `.run` pakai `color: var(--fg, #222)`; `:focus-visible { outline: 2px solid currentColor; outline-offset: -2px }`; titik `width:8px;height:8px;border-radius:50%;flex:none`; item `font-size:12px; color: var(--fg-muted, #888); padding: 2px 8px 2px 24px; font-family: ui-monospace, monospace`. Tidak ada warna keras selain fallback var. Tanpa scroll horizontal: tidak ada lebar tetap, semua teks `ellipsis`.
- Animasi "mengerjakan" opsional: titik berdenyut dengan `@keyframes` dan dimatikan lewat `@media (prefers-reduced-motion: reduce)`.

### 3. `ActivityFlowItem.svelte`

Props `{ thread, group }` via `$props()`.

- Turunan: `const agentName = $derived(participantName(thread, group.from));`, `const color = $derived(avatarColor(group.from));`, `const closed = $derived(thread?.state === 'closed');`.
- Waktu: `let now = $state(Date.now());`.
- Status: `const running = $derived(isRunning(group, now, group.finished, closed));`.
- Timer satu kali: `$effect(() => { ... })` yang membaca `group.lastTs`, `group.finished`, `closed`; bila sudah tidak running pada `Date.now()` -> `now = Date.now()` dan selesai; bila masih running -> `const id = setTimeout(() => { now = Date.now(); }, activityRemainingMs(group, Date.now()) + 50); return () => clearTimeout(id);`. Efek berjalan ulang tiap `lastTs` berubah (pembaruan pesan ber-id sama), sehingga timer dijadwalkan ulang; cleanup membatalkan timer lama. Tanpa `setInterval`/polling. `now` harus disegarkan juga sebelum evaluasi: set `now = Date.now()` di awal efek TIDAK boleh membuat loop (menulis state yang dibaca efek); baca `now` hanya lewat `untrack` atau gunakan `Date.now()` langsung di efek, dan tulis `now` hanya di callback timer/cabang selesai.
- Persistensi buka-tutup (opsional, aman): fungsi lokal `readOpen(key)`/`writeOpen(key, v)` dibungkus try/catch pada `localStorage` dengan kunci `thread.activity.open.${group.key}`; hasil baca dipakai sebagai `initialOpen`, `onToggle` menulis. Bila `localStorage` melempar, perilaku tetap benar (tertutup default).
- Render: `<ActivityCard {agentName} {color} items={group.items} count={group.count} {running} initialOpen={readOpen(group.key)} onToggle={(v) => writeOpen(group.key, v)} />` (bila ada waktu, bungkus dalam `<div class="flow">` kecil; jangan menambah elemen yang menimbulkan scroll horizontal). Tidak merender slot `ThreadSlot` (itu urusan `MessageItem`).

### 4. `index.ts`

```ts
export { default as ActivityFlowItem } from './ActivityFlowItem.svelte';
export { groupActivity, isRunning, ACTIVITY_RUNNING_MS } from './groupActivity';
export type { ActivityGroup, PlainFlowItem, FlowItem } from './groupActivity';
```

Tidak ada efek samping modul (tanpa registrasi); folder ini tidak merender apa pun sebelum Wiring.

### 5. Uji lokal tanpa mengubah berkas di luar folder

Tanpa unit test. Verifikasi logika `groupActivity` dengan data uji lokal SEMENTARA di luar repo (mis. skrip di scratchpad yang mengimpor `groupActivity.ts` lewat `npx tsx`/`vite-node` bila tersedia tanpa dependency baru; bila tidak tersedia, cukup tinjau kode dan andalkan `npm run check`). Skenario: (a) 3 pesan activity agent A beruntun -> 1 grup `count` sesuai jumlah; (b) A, lalu B, lalu A -> 3 grup; (c) A, pesan diskusi, A -> 2 grup, grup pertama `last=false`; (d) pesan ber-id sama muncul dua kali -> versi terakhir dipakai dan tidak ganda; (e) activity tanpa `items` dengan `text='proj: Edit a.go'` dan `names=()=>'proj'` -> `items=['Edit a.go']`, `count=1`; (f) pesan diskusi dari A setelah grup A -> `finished=true`. Jangan menambahkan berkas uji ke repo.

### 6. Guard

`cd frontend && npm run check && npm run build` (tanpa dependency baru). Perbaiki semua galat tipe di berkas folder ini. Bila kegagalan berasal dari berkas lain, hanya laporkan. Pastikan `git status` hanya menunjukkan berkas baru di `frontend/src/modules/thread/activity/`.

## Perubahan

Dibuat (semua baru):

- `frontend/src/modules/thread/activity/groupActivity.ts`
- `frontend/src/modules/thread/activity/ActivityCard.svelte`
- `frontend/src/modules/thread/activity/ActivityFlowItem.svelte`
- `frontend/src/modules/thread/activity/index.ts`

Tidak ada berkas lain yang diubah.

## Wiring

Fitur ini TIDAK menyambungkan apa pun. Yang diekspos untuk langkah Wiring (overview bagian 5 no. 3), dari `frontend/src/modules/thread/activity/index.ts`:

- `ActivityFlowItem` (komponen, props `{ thread: Thread; group: ActivityGroup }`) dan `groupActivity(messages, names?) => FlowItem[]`.
- Pemakaian yang diharapkan di `MessageList.svelte`: `const flow = $derived(groupActivity(messages, (id) => participantName(thread, id)));` lalu `{#each flow as item (item.key)}` dengan cabang `item.type === 'activity'` -> `<ActivityFlowItem {thread} group={item} />`, selain itu `<MessageItem {thread} message={item.message} />`. Efek tempel-ke-bawah memakai `flow.length` ditambah `count` grup terakhir bila aktivitas. Kunci `item.key` stabil (id pesan activity pertama) sehingga kartu tidak dibuat ulang saat pesan ber-id sama diperbarui.
- Catatan: `group.finished`/`group.last` sudah dihitung `groupActivity`; wiring tidak perlu menghitung ulang.
- Catatan untuk wiring bila kode berbeda saat implementasi (mis. nama ekspor tiket 01 berubah): tulis di sini.

## Selesai bila

- Keempat berkas ada di `frontend/src/modules/thread/activity/` dan hanya itu yang berubah.
- `groupActivity` murni dan sesuai aturan: dedup id (versi terakhir menang), grup hanya untuk activity beruntun satu agent, item lain memutus, pesan diskusi/sistem tidak masuk kartu, `count`/`items` memakai nilai daemon atau diturunkan dari `text` bila `items` kosong, `last` dan `finished` benar.
- Kartu tertutup default; header adalah `<button>` dengan `aria-expanded` yang mengikuti; daftar satu baris per item dengan ellipsis dan `title`; status "mengerjakan" selama < 30 detik dari `lastTs` dan belum `finished`/closed, otomatis beralih ke "selesai" lewat satu `setTimeout` (tanpa polling); thread `closed` selalu "selesai".
- Gaya hanya memakai token tema dengan fallback, tanpa lebar tetap/scroll horizontal; penanda warna memakai `avatarColor(group.from)`.
- `index.ts` mengekspor semua simbol di bagian Kontrak.
- Guard: `cd frontend && npm run check && npm run build` sukses tanpa galat tipe dan tanpa dependency baru (build path fitur ini sukses); tanpa unit test.
