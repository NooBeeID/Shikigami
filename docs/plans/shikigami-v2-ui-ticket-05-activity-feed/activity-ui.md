# Plan: UI: tab Activity di sidebar kanan

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-05-activity-feed.md`
- Fitur: `activity-ui`
- ownedPaths: `frontend/src/modules/activity/`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; kontrak IPC di overview bagian 2 cukup)
- Overview: `docs/plans/shikigami-v2-ui-ticket-05-activity-feed/overview.md` (baca bagian 2.2, 2.3, 2.5, 3, 7; jangan diubah)

## Tujuan

Menambah tab "Activity" di panel kanan (order 20, setelah "Files") yang menampilkan tool call terbaru agent claude: jam, nama sesi, lencana tool, ringkasan satu baris. Data: satu kali `activity.list` lalu event `activity.event`. Hanya tampilan, tanpa Approve/Reject, tanpa polling.

## Path yang dimiliki

HANYA `frontend/src/modules/activity/` (semua file baru). Dilarang menyentuh `modules/shell/**`, `modules/sidebar-right/**`, `modules/status/**`, `main.ts`, `App.svelte`, `lib/**`, file Go, `Makefile`, atau path fitur lain. Kebutuhan perubahan di luar path dicatat di laporan, tidak dikerjakan.

## Kontrak

Dipakai (sudah ada di repo, fakta terverifikasi):
- `registerRightTab({ id, title, component, order })` dari `../shell` (idempoten per id).
- `focusOrOpenSession(sessionId: string): boolean` dan `sessionTitle(sessionId: string): string` dari `../shell`. Catatan: `sessionTitle` untuk sesi yang tidak ada di store mengembalikan `id.slice(0, 6)`, bukan 8, jadi tangani sendiri (langkah 4).
- `call<T>(method, params?)` dan `onEvent(topicPrefix, fn(topic, payload)): unsubscribe` dari `../../lib/ipc`. `call` menolak (reject) di luar Wails; `onEvent` di luar Wails mengembalikan no-op.
- `sessions` (svelte store, `Session[]` dengan field `id`) dari `../../lib/stores`.
- IPC `activity.list` params `{ limit: 100 }` -> `ActivityEntry[]` terbaru -> terlama; event `activity.event` payload `ActivityEntry` (overview 2.2).
- Variabel CSS tema: `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok` (fallback boleh).

Disediakan:
- `export interface ActivityEntry { ts: number; sessionId: string; tool: string; summary: string }` (di `types.ts`, diekspor ulang `index.ts`).
- Tab terdaftar `{ id: 'activity', title: 'Activity', order: 20, component: ActivityTab }`.

## Langkah

1. `types.ts`: ekspor `ActivityEntry` persis seperti kontrak di atas.

2. `store.ts` (pakai `writable` svelte/store atau runes `$state` di `.svelte.ts`; pilih `writable` agar konsisten dengan `lib/stores`):
   - Konstanta `MAX_ENTRIES = 300`, `INITIAL_LIMIT = 100`.
   - `export const activityEntries: Readable<ActivityEntry[]>` (terbaru di depan).
   - Fungsi internal `keyOf(e) = \`${e.ts}|${e.sessionId}|${e.tool}|${e.summary}\``.
   - Fungsi internal `isEntry(x: unknown): x is ActivityEntry` (validasi tipe field; abaikan payload tak valid).
   - Fungsi internal `pushLive(e)`: bila `keyOf(e)` sudah ada di daftar, abaikan; selain itu taruh di depan, `slice(0, MAX_ENTRIES)`.
   - `export function ensureActivityStore(): void`, dijaga flag modul `started` (mount ulang tidak memanggil ulang):
     a. Set `started = true`.
     b. Subscribe dulu: `onEvent('activity.event', (_t, payload) => { if (isEntry(payload)) pushLive(payload) })` (simpan unsubscribe di variabel modul; tidak perlu dilepas, dibuat sekali).
     c. Lalu `call<ActivityEntry[]>('activity.list', { limit: INITIAL_LIMIT }).then(list => merge).catch(() => {})`. Gagal IPC diam-diam: tanpa `console.error`, tanpa retry, tanpa timer.
     d. `merge(list)`: bila bukan array, abaikan. Gabungkan riwayat dengan entri live yang sudah masuk: buang riwayat yang kuncinya sudah ada, gabung, urutkan `ts` menurun (stabil), `slice(0, MAX_ENTRIES)`.
   - Dilarang `setInterval`/`setTimeout`/polling.

3. `ActivityRow.svelte` (props: `entry: ActivityEntry`, `name: string`, `alive: boolean`, `onclick: () => void`):
   - Elemen `<button type="button">` padat satu baris; `disabled={!alive}` dan kelas `dead` (opacity ~0.5, `cursor: default`, tanpa hover).
   - Kolom: jam `HH:MM:SS` dari `new Date(entry.ts)` dengan `toLocaleTimeString([], { hour12: false })` atau format manual dua digit (monospace, warna `--fg-muted`); nama sesi (`name`, ellipsis, lebar maks sekitar 80px); lencana tool (`entry.tool`, chip kecil, `--bg-elev` + `--border`); ringkasan (`entry.summary`, `flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis`) dengan `title={entry.summary}` (tooltip teks penuh; bila kosong tampilkan `-` dan tooltip kosong).
   - Hanya CSS variabel tema; tanpa teks/tombol Approve/Reject.

4. `ActivityTab.svelte`:
   - Script `lang="ts"`. Impor `activityEntries`, `ensureActivityStore` dari `./store`; `sessions` dari `../../lib/stores`; `focusOrOpenSession`, `sessionTitle` dari `../shell`; `ActivityRow`.
   - Saat mount panggil `ensureActivityStore()` (di `onMount` atau `$effect` sekali).
   - Fungsi `alive(id)`: `$sessions.some(s => s.id === id)` (pakai auto-subscription `$sessions`).
   - Fungsi `nameOf(id)`: bila `alive(id)` -> `sessionTitle(id) || id.slice(0, 8)`; bila tidak ada di store -> `id.slice(0, 8)`.
   - Filter: state `filter = $state('')` ('' = semua). `<select>` dengan opsi pertama value `''` teks "Semua sesi", lalu satu opsi per sessionId unik di `$activityEntries` (label `nameOf(id)`, urut kemunculan pertama). Bila `filter` tidak lagi ada di opsi (mis. entri terbuang), kembalikan ke `''`. `visible = filter ? entries.filter(e => e.sessionId === filter) : entries` (di klien, tanpa IPC).
   - Klik baris: `if (alive(e.sessionId)) focusOrOpenSession(e.sessionId)`; baris `alive=false` dinonaktifkan (tanpa aksi).
   - Keadaan kosong (`visible.length === 0`): teks "Belum ada aktivitas" di tengah, warna `--fg-muted`. Tidak ada error mencolok untuk kegagalan IPC.
   - Auto-scroll menempel atas: kontainer daftar `overflow-y: auto` dengan `bind:this`. Karena entri baru di depan, pertahankan posisi bila pengguna sudah menggulir: sebelum render catat `scrollTop`; bila `scrollTop <= 4` biarkan di atas (tetap 0); bila `scrollTop > 4` setelah `tick()` kompensasi `scrollTop += (scrollHeight baru - scrollHeight lama)` agar baris yang dilihat tidak bergeser. Implementasi dengan `$effect.pre` menyimpan `prevHeight/prevTop` dan `$effect` pasca-render menyesuaikan (memantau panjang `visible`). Tanpa timer.
   - Daftar dirender dengan `{#each visible as e (keyOfRow)}` (key unik: gabungan `ts|sessionId|tool|summary|indeks-duplikat` bila perlu; bila ragu pakai indeks untuk key karena maks 300 baris). Tanpa virtualisasi.
   - Tata letak: kolom penuh tinggi (`display: flex; flex-direction: column; height: 100%; min-height: 0`), header filter di atas (padat, `--border` bawah), daftar `flex: 1`.

5. `index.ts`:
   ```ts
   import { registerRightTab } from '../shell';
   import ActivityTab from './ActivityTab.svelte';
   registerRightTab({ id: 'activity', title: 'Activity', order: 20, component: ActivityTab });
   export { ActivityTab };
   export type { ActivityEntry } from './types';
   ```
   Tidak melakukan IPC saat modul dimuat (store malas, dimulai saat tab dipasang). Pola sama dengan `modules/sidebar-right/index.ts`.

6. Guard: `cd frontend && npm run build` harus lulus tanpa error tipe/svelte-check. Periksa dengan `grep -rn "setInterval\|setTimeout\|Approve\|Reject" frontend/src/modules/activity/` bahwa hasilnya kosong.

## Perubahan

Dibuat (semua di `frontend/src/modules/activity/`):
- `types.ts`
- `store.ts`
- `ActivityRow.svelte`
- `ActivityTab.svelte`
- `index.ts`

Tidak ada file yang diubah di luar path ini.

## Wiring

Yang diekspos: `index.ts` mendaftarkan tab lewat `registerRightTab` dan mengekspor `ActivityTab` serta tipe `ActivityEntry`. `main.ts` sudah memuat `modules/*/index.ts` lewat glob eager, jadi tidak ada edit yang diperlukan; JANGAN menambah impor di `modules/shell/register.ts`, `main.ts`, atau `App.svelte` (itu urusan agen Wiring, hanya bila terbukti glob tidak memuatnya). Sisi daemon (event `activity.event`, `activity.list`) disambungkan oleh fitur `activity-daemon` dan Wiring; UI hanya mengonsumsi kontrak.

## Selesai bila

- `cd frontend && npm run build` sukses tanpa error tipe (guard; tanpa unit test).
- Hanya file di `frontend/src/modules/activity/` yang dibuat/diubah.
- `registerRightTab` dipanggil dengan `id: 'activity'`, `title: 'Activity'`, `order: 20`.
- `activity.list` dipanggil tepat sekali (`{ limit: 100 }`), `onEvent('activity.event')` dibuat sekali, dijaga flag modul; tanpa `setInterval`/polling.
- Store maks 300 entri, terbaru di depan, dedupe riwayat vs event, filter sesi di klien.
- Baris: jam HH:MM:SS, nama sesi (`sessionTitle` atau 8 char id), lencana tool, ringkasan ellipsis + tooltip; klik memanggil `focusOrOpenSession`; sesi hilang -> baris nonaktif.
- Kosong menampilkan "Belum ada aktivitas"; kegagalan IPC diam-diam; tidak ada teks/tombol Approve/Reject.
