# Plan: Lonceng notifikasi

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-04-shell-sidebars.md` (epic shell dan sidebar, wave 4)
- Fitur: `notification-bell` (Lonceng notifikasi)
- ownedPaths: `frontend/src/modules/notifications/`
- Depends on foundation: ya. Memakai `registerTopbarItem` dan helper `focusOrOpenSession`/`sessionTitle` dari `modules/shell` (dibuat `shell-frame`, plan `00-foundation.md`). Foundation sudah tergabung sebelum plan ini dikerjakan. Bila `frontend/src/modules/shell/index.ts` belum mengekspor simbol itu, hentikan dan laporkan blocker (jangan membuat stub, jangan mengedit `modules/shell/`).
- Kontrak lengkap: `overview.md` bagian 2 (terutama 2.1, 2.3, 2.5, 2.9) dan bagian 3 (catatan `notification-bell`). Bagian 7 (aturan paralel) berlaku penuh.

## Tujuan

Menyediakan lonceng notifikasi di zona kanan topbar. Lonceng menampilkan badge jumlah belum dibaca dan dropdown daftar peristiwa "agent menunggu input" dan "agent selesai", diturunkan dari perubahan status sesi di `statusStore`. Klik entri memfokuskan (atau memasukkan ke layout) pane sesinya. Murni event-driven: tanpa polling, tanpa pendengar IPC tambahan, state hanya di memori.

## Path yang dimiliki

HANYA `frontend/src/modules/notifications/` yang boleh disentuh (file baru: `store.ts`, `Bell.svelte`, `index.ts`). Dilarang menyentuh `modules/shell/` (termasuk `register.ts`), `modules/topbar/`, `App.svelte`, path fitur lain, `modules/status/`, `modules/layout/`, `lib/stores/*`, `main.ts`, `package.json`, `styles/*`. Fitur paralel lain tidak boleh diimpor (`topbar`, `sidebar-left`, `sidebar-right`, `palette`).

## Kontrak

Dipakai (sudah ada, jangan diubah):

- `statusStore: Readable<Record<string, {status: Status; since: number}>>` dan tipe `Status = 'running'|'waiting'|'done'|'idle'` dari `../status/store`. Fakta penting: `startStatusSync()` sudah dipanggil oleh `modules/status/index.ts`; JANGAN memanggilnya lagi dan jangan memakai `onEvent`/`call` dari `lib/ipc`. Store mulai `{}`, lalu `status.list` mengisi banyak sesi sekaligus secara asinkron (snapshot awal tiba SETELAH callback subscribe pertama), dan event `session.status` menimpa satu entri. Event `source:'exit'` sudah dipetakan ke `done` oleh store status.
- `sessions` (`Readable<Session[]>`) dari `../../lib/stores` (impor `sessions` saja dari `lib/stores/index.ts`; ini diizinkan, hanya DILARANG mengedit file itu).
- Dari `../shell` (overview 2.3 dan 2.5): `registerTopbarItem(spec: TopbarItemSpec)`, `focusOrOpenSession(sessionId: string): boolean`, `sessionTitle(sessionId: string): string`.
- Tema: variabel `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok` (tanpa warna hardcode selain fallback).

Disediakan (diekspor `modules/notifications/index.ts`):

```ts
// store.ts
export type NotificationKind = 'waiting' | 'done';
export interface AppNotification { id: string; sessionId: string; kind: NotificationKind; title: string; at: number; read: boolean }
export const MAX_NOTIFICATIONS = 50;
export const notifications: Readable<AppNotification[]>;   // terbaru di indeks 0, maks 50
export const unreadCount: Readable<number>;
export function markRead(id: string): void;
export function markAllRead(): void;
export function clear(): void;
```

`index.ts` mengekspor ulang semua di atas ditambah `Bell` (default export `Bell.svelte`), dan memanggil `registerTopbarItem({ id: 'notifications', component: Bell, align: 'right', order: 10 })` (idempoten; aman diimpor berkali-kali oleh `main.ts` glob dan oleh `register.ts` wiring).

Aturan turunan notifikasi (keputusan desain, ikuti persis):

1. Simpan peta `prev: Map<sessionId, Status>`.
2. Untuk tiap `[sessionId, {status, since}]` pada update `statusStore`:
   - Sesi belum ada di `prev`: hanya catat `status` ke `prev`, TIDAK ada notifikasi (ini menutup snapshot awal, termasuk `status.list` yang tiba asinkron, dan sesi yang pertama kali terlihat).
   - Sudah ada dan `status === prev.get(id)`: abaikan (update store lain, tanpa duplikat).
   - Sudah ada dan berubah: perbarui `prev`; bila status baru `waiting` atau `done`, buat notifikasi; `running`/`idle` tidak membuat notifikasi.
3. Sesi yang hilang dari peta (bila store pernah mengosongkan entri) dihapus dari `prev` agar sesi baru ber-id sama dianggap "pertama kali terlihat".
4. `at` = `since` dari entri status bila berupa angka finite, selain itu `Date.now()`. `title` = `sessionTitle(sessionId)` diambil SAAT notifikasi dibuat (judul tidak berubah belakangan). `id` = `${sessionId}:${kind}:${at}:${counter}` (counter modul yang naik terus) agar unik. `read: false`.
5. Daftar: sisipkan di depan, potong ke `MAX_NOTIFICATIONS` (50) terbaru.

## Langkah

1. **Prasyarat (hanya baca).** Pastikan `frontend/src/modules/shell/index.ts` mengekspor `registerTopbarItem`, `focusOrOpenSession`, `sessionTitle`; pastikan `modules/status/store.ts` mengekspor `statusStore` dan `Status`. Baca satu komponen topbar-agnostik untuk gaya tombol (`styles/base.css`) agar tombol lonceng konsisten. Jalankan `cd frontend && npm install` bila `node_modules` belum ada (worktree).

2. **`frontend/src/modules/notifications/store.ts`** (BARU).
   - Impor: `writable`, `derived`, `type Readable` dari `svelte/store`; `statusStore`, `type Status` dari `../status/store`; `sessionTitle` dari `../shell`.
   - `const list = writable<AppNotification[]>([])`; `export const notifications: Readable<AppNotification[]> = { subscribe: list.subscribe }`; `export const unreadCount = derived(list, (l) => l.filter((n) => !n.read).length)`.
   - `markRead(id)`: `list.update` memetakan entri dengan id itu menjadi `read:true` (tidak mengubah bila sudah dibaca, agar referensi tidak berubah sia-sia). `markAllRead()`: semua `read:true`. `clear()`: `list.set([])`.
   - Fungsi privat `ingest(map: Record<string, {status: Status; since: number}>)` menerapkan aturan turunan 1-5 di atas. Kumpulkan notifikasi baru dalam satu array lokal lalu satu kali `list.update` per pemanggilan `ingest` (bila array tidak kosong); urutan dalam batch: sesuai iterasi, sisipkan sehingga yang terbaru ada di depan.
   - Langganan: satu `statusStore.subscribe(ingest)` di level modul, dibuat sekali saat modul dimuat, TIDAK pernah di-unsubscribe dan TIDAK dibuat ulang saat dropdown dibuka/ditutup. Jangan tambah `setInterval`/`setTimeout`.
   - Guard modul dimuat berkali-kali oleh HMR: bungkus pembuatan langganan dengan penanda modul (`let started = false`), cukup untuk ESM biasa.
   - Tanpa `console.log`.

3. **`frontend/src/modules/notifications/Bell.svelte`** (BARU, Svelte 5 runes, tanpa props wajib).
   - Impor `notifications`, `unreadCount`, `markRead`, `markAllRead`, `clear` dari `./store`; `focusOrOpenSession` dari `../shell`; `sessions` dari `../../lib/stores`.
   - State: `let open = $state(false)`. Gunakan `$notifications`, `$unreadCount`, `$sessions` (auto-subscribe).
   - Tombol lonceng (`<button class="bell" aria-label="Notifikasi" aria-haspopup="true" aria-expanded={open}>`): ikon SVG inline lonceng (tanpa dependency ikon), badge bulat `--danger` di pojok bila `$unreadCount > 0`, teks `99+` bila lebih dari 99. Beri `--wails-draggable: no-drag` pada tombol dan dropdown.
   - Dropdown (muncul bila `open`, posisi absolute di bawah tombol, rata kanan, lebar ~320px, `max-height` ~60vh dengan `overflow-y:auto`, `z-index` di atas area layout): header berisi judul "Notifikasi" dan dua tombol teks "Tandai semua dibaca" (nonaktif bila `$unreadCount===0`) dan "Bersihkan" (nonaktif bila daftar kosong). Daftar entri; state kosong "Belum ada notifikasi".
   - Baris entri: titik penanda belum dibaca (aksen), `n.title`, label jenis (`waiting` -> "menunggu input", `done` -> "selesai") dan waktu relatif dari `n.at` (fungsi lokal `relTime(at, now)`: "baru saja" < 60 dtk, "N mnt lalu", "N jam lalu", "N hari lalu"). Waktu relatif dihitung saat dropdown dibuka/render (simpan `now = $state(Date.now())`, perbarui HANYA saat dropdown dibuka dan saat daftar berubah; tanpa interval). Entri sudah dibaca tampil lebih pudar.
   - Entri aktif bila sesinya ada di `$sessions` (`$sessions.some((s) => s.id === n.sessionId)`): elemen `<button>`; klik -> `focusOrOpenSession(n.sessionId)` lalu `markRead(n.id)`, lalu tutup dropdown. Bila sesi tidak ada: entri redup (`opacity` rendah, `aria-disabled`, `disabled`), tanpa aksi klik. Bila `focusOrOpenSession` mengembalikan `false`, tetap `markRead` tetapi jangan melempar.
   - Menutup dropdown: klik di luar (listener `pointerdown` di `window` dipasang di `$effect` hanya saat `open` dan dibersihkan saat `open` false/destroy; periksa `bellEl.contains(e.target)`), tombol Escape (listener `keydown` di `$effect` yang sama). Ini listener DOM, bukan langganan event IPC.
   - Gaya scoped memakai variabel tema (`--bg-elev`, `--border`, `--fg`, `--fg-muted`, `--bg-hover`, `--accent`, `--danger`); tidak ada warna hardcode kecuali fallback. Teks UI bahasa Indonesia.

4. **`frontend/src/modules/notifications/index.ts`** (BARU).
   ```ts
   import Bell from './Bell.svelte';
   import { registerTopbarItem } from '../shell';
   export { Bell };
   export * from './store';
   registerTopbarItem({ id: 'notifications', component: Bell, align: 'right', order: 10 });
   ```
   Pastikan `store.ts` ikut termuat (ekspor ulang memicunya) sehingga langganan status aktif sejak aplikasi start (via glob `main.ts`), bukan baru saat lonceng pertama kali dirender. Jangan mengimpor `../shell/register`.

5. **Guard.** `cd frontend && npm run build` (wajib), lalu `make build` dari root bila sempat. Pastikan tidak ada galat tipe/Svelte di `modules/notifications/`. Cek `git status`/diff: hanya file di `frontend/src/modules/notifications/` yang berubah.

## Perubahan

Baru (semuanya di `frontend/src/modules/notifications/`):

- `store.ts`
- `Bell.svelte`
- `index.ts`

Tidak ada file yang diubah atau dihapus di luar folder ini.

## Wiring

Yang diekspos untuk disambungkan nanti (JANGAN disambungkan sendiri):

- `modules/notifications/index.ts` mendaftarkan `registerTopbarItem({ id: 'notifications', component: Bell, align: 'right', order: 10 })` saat dimuat. Langkah wiring (`modules/shell/register.ts`, milik `shell-wiring`) cukup `import '../notifications'` sebagai jaminan (overview bagian 5 butir 1); `Topbar` (fitur `topbar`) merendernya lewat `topbarItems('right')` (posisi sebelum `SessionCounter` karena `order < 50`).
- Ekspor tambahan yang boleh dipakai pihak lain kelak: `notifications`, `unreadCount`, `markRead`, `markAllRead`, `clear`, `Bell`.
- Sebelum wiring, lonceng tidak tampak di aplikasi; ini wajar.

## Selesai bila

- Tiga file ada di `frontend/src/modules/notifications/`, tidak ada file lain di luar path itu yang berubah.
- `store.ts` mengekspor `notifications`, `unreadCount`, `markRead`, `markAllRead`, `clear` dengan perilaku di Kontrak; satu langganan `statusStore` tingkat modul; tidak ada `setInterval`/polling, tidak ada `onEvent`/`call` IPC, tidak ada `console.log`.
- Transisi ke `waiting`/`done` membuat tepat satu entri; `running`/`idle`, update tanpa perubahan, dan kemunculan pertama sesi (termasuk snapshot `status.list`) tidak membuat entri; daftar dibatasi 50.
- `Bell.svelte`: badge menghitung belum dibaca, dropdown dengan "Tandai semua dibaca" dan "Bersihkan"; klik entri memanggil `focusOrOpenSession` lalu `markRead`; entri tanpa sesi redup dan tanpa aksi; dropdown menutup lewat klik luar/Escape.
- `index.ts` mendaftarkan item topbar `notifications` (align `right`, order `10`).
- Guard: `cd frontend && npm run build` sukses (dan `make build` bila dijalankan); tanpa unit test.
