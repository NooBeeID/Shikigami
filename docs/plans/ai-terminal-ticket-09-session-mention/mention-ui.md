# Plan: @nama-sesi di UI

## Metadata

- Tiket: `ai-terminal-ticket-09-session-mention` (`docs/tickets/ai-terminal-ticket-09-session-mention.md`)
- Fitur: `mention-ui` (@nama-sesi di UI)
- ownedPaths: `frontend/src/modules/mention/`
- Depends on foundation: tidak. Foundation hanya `internal/mention/types.go` (Go); fitur ini murni frontend dan hanya bergantung pada nama method/event IPC (overview 2.5) serta `lib/ipc.ts`, `lib/registry.ts`, `lib/stores`.
- Rujukan: `overview.md` di folder ini (bagian 2.5 pesan IPC, 2.7 pola nama, 2.8 konvensi, 7 aturan paralel `mention-ui`).

## Tujuan

Menyediakan modul frontend Svelte 5 `modules/mention/` dengan dua panel:

1. `MentionComposer.svelte` (sidebar, order 30): kotak pesan dengan autocomplete `@nama-sesi`, kirim lewat `mention.send` (`from` kosong = user), opsi menyertakan output terakhir sesi aktif.
2. `QueueBadge.svelte` (statusbar, order 20): lencana jumlah pesan tertunda dan daftar antrean dengan tombol Batalkan/Konfirmasi, diperbarui hanya lewat event `mention.queue` (tanpa polling).

Seluruhnya harus lolos `npm run build` walau backend mention belum hidup (error IPC jadi pesan singkat, bukan crash) dan juga di luar Wails (`inWails === false`).

## Path yang dimiliki

HANYA `frontend/src/modules/mention/`. Dilarang menyentuh: `frontend/src/App.svelte`, `frontend/src/lib/*` (termasuk `lib/stores`, `lib/ipc.ts`, `lib/registry.ts`), `frontend/src/styles/*`, `frontend/src/modules/<modul lain>/`, `frontend/package.json`, file Go apa pun, dan file wiring. Bila butuh perubahan di luar path ini, cari solusi lokal dan catat sebagai catatan di laporan.

## Kontrak

### Dipakai (sudah ada di repo, jangan diubah)

- `frontend/src/lib/ipc.ts`: `inWails: boolean`, `call<T>(method, params?) => Promise<T>` (menolak dengan `Error('bukan di dalam Wails')` bila tanpa bridge), `onEvent(topicPrefix, fn(topic, payload)) => unsubscribe` (cocok per prefix; tidak ada ekspor `subscribe`).
- `frontend/src/lib/registry.ts`: `registerPanel({id, area: 'sidebar'|'main-tab'|'statusbar', component, order})`.
- Barrel `frontend/src/lib/stores` (impor `'../../lib/stores'`): `activeWorkspaceId: Writable<string|null>`, `sessions: Writable<Session[]>`, `activeTabId: Writable<string|null>` (tab sesi berbentuk `session:<sessionId>`), tipe `Session {id, workspaceId, projectId, cwd, providerId, status: 'running'|'exited', createdAt, exitCode?}`.
- `session.attach` `{id}` -> `{session, scrollback: string (base64), offset}`. Scrollback adalah byte mentah terminal dalam base64 (lihat `modules/terminal/terminal-client.ts`, `attachSession`). Jangan mengimpor dari modul `terminal` (modul lain); tulis dekoder base64 kecil sendiri.
- Event `session.exit` payload `{id, exitCode}`.
- Layout: footer statusbar (`modules/shell/StatusBar.svelte`) bertinggi `var(--statusbar-h)` (24px) dengan `overflow: hidden`; daftar antrean TIDAK boleh bergantung pada `position: absolute` di dalamnya (akan terpotong), pakai `position: fixed` (lihat langkah 7).
- Variabel CSS yang ada di `frontend/src/styles/theme.css`: `--bg`, `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--danger`, `--ok`. Selalu tulis dengan nilai cadangan, mis. `var(--bg-elev, #24242a)`.

### IPC yang dipanggil (overview 2.5; nama persis, camelCase)

| Pemanggilan | Params | Hasil |
|---|---|---|
| `mention.names` | `{workspaceId}` | `NameInfo[]` = `{sessionId, name, projectName, providerId, status}` (status `running\|waiting\|done\|idle`) |
| `mention.send` | `{workspaceId, to, text}` (tanpa `from`) | `Message` |
| `mention.queue.list` | `{}` (semua sesi) | `Message[]` pending |
| `mention.queue.cancel` | `{id}` | `{ok:true}` |
| `mention.confirm` | `{id}` | `Message` |
| event `mention.queue` | payload `{sessionId, queue: Message[]}` (seluruh antrean pending sesi itu; `queue: []` = kosong) | - |

`Message = {id, from, fromName, to, text, createdAt, state: 'queued'|'awaiting_confirm'|'delivered'|'failed', source: 'agent'|'user', error?}`. Balasan list bisa array langsung atau objek berkunci; tangani keduanya dan `null` (helper lokal, jangan impor `normalizeList` dari `lib/stores/util`, tidak diekspor oleh barrel).

Kode error IPC datang sebagai `Error.message` dari `call`; petakan ke pesan singkat Indonesia (langkah 2).

### Disediakan (untuk wiring)

- `frontend/src/modules/mention/index.ts` yang memanggil `registerPanel` dua kali: `mention-composer` (sidebar, order 30) dan `mention-queue` (statusbar, order 20). Ditemukan otomatis oleh `import.meta.glob('./modules/*/index.ts')` di `main.ts`.

## Langkah

Semua file di `frontend/src/modules/mention/`. Komentar dan teks UI berbahasa Indonesia, identifier Inggris. Svelte 5 rune (`$state`, `$derived`, `$effect`, `$props`), `<script lang="ts">`. Tanpa `setInterval`, tanpa dependency baru.

### 1. `types.ts`

Ekspor tipe lokal: `MessageState`, `Message`, `NameInfo` (bentuk di tabel di atas), dan `QueueEvent = {sessionId: string; queue: Message[]}`.

### 2. `mention-client.ts` (pembungkus IPC + helper murni)

Ekspor:

- `function asList<T>(res: unknown, key?: string): T[]`: array langsung -> itu; objek dengan `res[key]` array -> itu; selain itu `[]`.
- `function errorText(e: unknown): string`: ambil `e.message`; jika berisi `bukan di dalam Wails` -> `'Hanya tersedia di aplikasi desktop'`; jika mengandung `ERR_NOT_FOUND`/`not found`/`tidak ditemukan` -> `'Sesi tidak ditemukan'`; `ERR_CONFLICT`/`penuh` -> `'Antrean penuh (maks 50)'`; `user_only` -> `'Mode user_only: pesan dari agent ditolak'`; `ERR_INVALID`/`tidak valid`/`workspace` -> `'Pesan tidak valid (kosong atau beda workspace)'`; selain itu pesan aslinya dipotong 120 karakter; kosong -> `'Gagal menghubungi daemon'`. Jangan pernah melempar.
- `listNames(workspaceId: string): Promise<NameInfo[]>`: `asList(await call('mention.names', {workspaceId}))`.
- `sendMention(workspaceId: string, to: string, text: string): Promise<Message>`: `call<Message>('mention.send', {workspaceId, to, text})`.
- `listQueue(): Promise<Message[]>`: `asList(await call('mention.queue.list', {}))`.
- `cancelMessage(id: string): Promise<void>` dan `confirmMessage(id: string): Promise<void>`: memanggil `mention.queue.cancel` / `mention.confirm`.
- `onQueueEvent(fn: (e: QueueEvent) => void): () => void`: `onEvent('mention.queue', (_t, p) => {...})`; validasi bentuk payload (`sessionId` string, `queue` array atau null -> `[]`) sebelum memanggil `fn`; mengembalikan unsubscribe.
- `onSessionExit(fn: () => void): () => void`: `onEvent('session.exit', () => fn())`.
- `function stripAnsi(s: string): string`: buang CSI (`/\x1b\[[0-?]*[ -\/]*[@-~]/g`), OSC (`/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g`), escape dua karakter lain (`/\x1b[@-Z\\-_]/g`), normalkan `\r\n` dan `\r` jadi `\n`, lalu buang karakter kontrol selain `\n` dan `\t` (`/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g`).
- `function decodeBase64(b64: string): string`: `atob` -> `Uint8Array` -> `new TextDecoder('utf-8', {fatal:false}).decode`; bungkus try/catch, gagal -> `''`.
- `async function lastOutput(sessionId: string, lines = 30): Promise<string>`: `call('session.attach', {id: sessionId})`, dekode `scrollback`, `stripAnsi`, pecah per `\n`, buang baris kosong di ekor, ambil `lines` terakhir, `join('\n')`, `trimEnd()`. Batasi hasil maksimal 4000 karakter (ambil bagian akhir).
- `function parseMention(input: string): {name: string; text: string} | null`: pada `input.trimStart()`, cocokkan `/^@([a-z0-9-]+)(?:\s+([\s\S]*))?$/i`; bila cocok kembalikan `{name: m[1].toLowerCase(), text: (m[2] ?? '').trim()}`; selain itu `null`. (Pola nama valid `[a-z0-9-]+`, overview 2.7.)
- `function mentionQuery(input: string, caret: number): {start: number; query: string} | null`: ambil `input.slice(0, caret)`; cocokkan `/(^|\s)@([a-z0-9-]*)$/i`; bila cocok kembalikan `start` = indeks `@` dan `query` huruf kecil; selain itu `null`. Dipakai untuk membuka/menutup autocomplete.

### 3. `MentionComposer.svelte` (sidebar)

Impor: `onMount`/`onDestroy` dari `svelte`, `get` tidak perlu; `activeWorkspaceId`, `sessions`, `activeTabId` dari `'../../lib/stores'`; `inWails` dari `'../../lib/ipc'`; helper dari `./mention-client`; tipe dari `./types`.

State (`$state`): `text = ''`, `names: NameInfo[] = []`, `namesError = ''`, `menuOpen = false`, `menuIndex = 0`, `query = ''`, `mentionStart = -1`, `includeOutput = false`, `busy = false`, `feedback: {kind: 'ok'|'err'; msg: string} | null`. `textareaEl: HTMLTextAreaElement | undefined` lewat `bind:this`.

Perilaku:

- Tampilan: judul kecil "Mention sesi", `<textarea rows="3" placeholder="@nama-sesi pesan...">`, baris opsi `<label><input type="checkbox" bind:checked={includeOutput}> Sertakan output terakhir sesi aktif</label>`, tombol "Kirim", area `feedback` (`role="status"`), dan menu autocomplete.
- Muat nama `async function refreshNames()`: bila `!inWails` set `namesError = 'Hanya tersedia di aplikasi desktop'` dan `names = []`; bila `$activeWorkspaceId` kosong -> `names = []`; selain itu `listNames(ws)` dalam try/catch (`namesError = errorText(e)`, `names = []`). Tangani balasan basi: simpan `ws` lokal dan abaikan hasil bila `$activeWorkspaceId !== ws` saat selesai.
- Pemicu muat ulang (tanpa timer): `$effect(() => { $activeWorkspaceId; void refreshNames(); })` (workspace berganti); `$effect(() => { $sessions; void refreshNames(); })` (daftar sesi store berubah); dan di `onMount` `const off = onSessionExit(() => void refreshNames())` dibersihkan di `onDestroy`/return cleanup. Gabung dua efek bila lebih rapi, tetapi jangan baca `names` di dalam efek yang menulisnya (hindari loop).
- Autocomplete: pada `oninput` dan `onclick`/`onkeyup` textarea hitung `mentionQuery(text, textareaEl.selectionStart)`. Bila ada hasil: `menuOpen = true`, `query`, `mentionStart`, `menuIndex = 0`; selain itu `menuOpen = false`. `const filtered = $derived(names.filter(n => n.name.startsWith(query)))`; menu terbuka hanya bila `filtered.length > 0`. Tiap item menampilkan `@name`, `projectName`, `providerId`, dan lencana `status` (warna: `running` -> `var(--accent, #5b94ff)`, `waiting` -> `#e0a030`, `done`/`idle` -> `var(--ok, #4cc788)`). Menu: `role="listbox"`, item `role="option"` dengan `aria-selected`; posisi di bawah textarea (`position: absolute` di dalam wadah `position: relative` panel sidebar; sidebar bukan footer, jadi aman), `max-height` dengan scroll.
- Keyboard di `onkeydown` textarea: bila `menuOpen && filtered.length`: `ArrowDown`/`ArrowUp` ubah `menuIndex` (melingkar, `preventDefault`), `Enter` atau `Tab` memilih (`preventDefault`), `Escape` menutup. Bila menu tertutup: `Enter` tanpa Shift dan (Ctrl atau Meta) -> kirim; `Enter` biasa menyisipkan baris baru (pesan boleh multi-baris). Klik mouse pada item memakai `onmousedown` dengan `preventDefault` (agar textarea tidak kehilangan fokus) lalu pilih.
- `function pick(n: NameInfo)`: ganti `text.slice(mentionStart, caret)` dengan `'@' + n.name + ' '`, pertahankan sisa teks setelah caret, tutup menu, fokuskan textarea dan pindahkan caret ke setelah sisipan (`queueMicrotask`/`tick` lalu `setSelectionRange`).
- `async function submit()`: guard `busy`. Bersihkan `feedback`. Validasi berurutan, masing-masing menetapkan `feedback = {kind:'err', ...}` dan return tanpa memanggil IPC:
  - `!inWails` -> 'Hanya tersedia di aplikasi desktop'.
  - `!$activeWorkspaceId` -> 'Pilih workspace dulu'.
  - `parseMention(text)` null -> 'Awali pesan dengan @nama-sesi'.
  - Cek `names.some(n => n.name === parsed.name)`; bila `names` tidak memuatnya -> `Sesi @${name} tidak ada di workspace ini` (tetap boleh jatuh ke server bila `names` kosong karena gagal dimuat? Tidak: bila `namesError` ada, tetap kirim agar server yang memutuskan; hanya blokir bila `names` berhasil dimuat dan tidak memuat nama).
  - Tanpa `parsed.text` dan `!includeOutput` -> 'Isi pesan tidak boleh kosong'.
  Bangun `body = parsed.text`. Bila `includeOutput`: tentukan sesi aktif dari `$activeTabId` (`startsWith('session:')` -> `slice(8)`); bila tidak ada tab sesi aktif -> `feedback err 'Tidak ada sesi aktif untuk diambil outputnya'` dan return. Ambil `out = await lastOutput(id, 30)` dalam try/catch (gagal -> pesan `errorText`). Jika `out` kosong -> err 'Output sesi aktif kosong'. Susun: `body = (parsed.text ? parsed.text + '\n\n' : '') + '--- output terakhir sesi aktif ---\n' + out + '\n--- selesai ---'`.
  Kirim: `busy = true`; `const msg = await sendMention($activeWorkspaceId, parsed.name, body)`; sukses -> `feedback = {kind:'ok', msg: stateLabel(msg.state, parsed.name)}` dengan `stateLabel`: `delivered` -> `Terkirim ke @nama`, `queued` -> `Diantre untuk @nama (menunggu idle)`, `awaiting_confirm` -> `Menunggu konfirmasi`, `failed` -> `Gagal: ${msg.error ?? 'tidak diketahui'}`; kosongkan `text` dan `includeOutput` tetap (preferensi) bila state bukan `failed`. `catch (e)` -> `feedback err errorText(e)` dan teks dipertahankan. `finally busy = false`.
- Tombol "Kirim" `disabled={busy}`. Feedback ok bertahan sampai pengiriman berikutnya atau perubahan teks (kosongkan di `oninput`); jangan pakai `setTimeout`/`setInterval` untuk menghapusnya.
- Semua teks yang dirender lewat interpolasi Svelte (tanpa `{@html}`).

Styling (blok `<style>` komponen): kontainer `padding: 8px`, `border-top: 1px solid var(--border, #35353d)`; textarea `width: 100%`, `box-sizing: border-box`, `resize: vertical`, `background: var(--bg, #1b1b1f)`, `color: var(--fg, #e6e6ea)`, `border: 1px solid var(--border, #35353d)`, `border-radius: 4px`, `font: inherit`; menu `background: var(--bg-elev, #24242a)`, item aktif `var(--bg-hover, #2f2f37)`; teks sekunder `var(--fg-muted, #9b9ba6)`; error `var(--danger, #ff6b81)`; ok `var(--ok, #4cc788)`; tombol memakai `var(--accent, #5b94ff)`.

### 4. `queue-store.ts` (state antrean bersama, tanpa timer)

Satu store modul-level agar `QueueBadge` dan daftar berbagi data:

- `import { writable, derived } from 'svelte/store'`.
- `const bySession = writable<Record<string, Message[]>>({})`; `export const queueError = writable('')`.
- `export const pendingMessages = derived(bySession, m => Object.values(m).flat().sort((a, b) => a.createdAt.localeCompare(b.createdAt)))`; `export const pendingCount = derived(pendingMessages, l => l.length)`.
- `export function applyQueueEvent(e: QueueEvent)`: `bySession.update(m => { const n = {...m}; if (e.queue.length === 0) delete n[e.sessionId]; else n[e.sessionId] = e.queue; return n; })`.
- `export async function loadInitial()`: bila `!inWails` -> `queueError.set('Hanya tersedia di aplikasi desktop')` dan return; `const list = await listQueue()` (try/catch -> `queueError.set(errorText(e))`); kelompokkan per `to` (sessionId target) dan `bySession.set(...)`. Perhatian balapan: event yang tiba sebelum balasan awal tidak boleh tertimpa data basi. Gunakan penanda `receivedEvent: Set<string>` (sessionId yang sudah menerima event); saat menerapkan hasil awal, lewati sessionId yang ada di set tersebut (event lebih baru menang).
- `export function startQueueSync(): () => void`: pasang `onQueueEvent(e => { seen.add(e.sessionId); applyQueueEvent(e); })` SEBELUM memanggil `void loadInitial()`; kembalikan fungsi yang memanggil unsubscribe. Penghitung referensi sederhana (`refs`) agar bila dipanggil dua kali, langganan hanya satu dan dilepas saat referensi terakhir selesai. Tidak ada timer.
- `export async function doCancel(id: string): Promise<string>` dan `doConfirm(id: string): Promise<string>`: panggil `cancelMessage`/`confirmMessage`; kembalikan `''` bila sukses atau `errorText(e)` bila gagal. Tidak mengubah store secara manual: pembaruan datang dari event `mention.queue` (sumber kebenaran backend).

### 5. `QueueBadge.svelte` (statusbar)

- `onMount`: `const stop = startQueueSync(); return stop;` (cleanup saat destroy).
- Impor `pendingMessages`, `pendingCount`, `queueError` dan `sessions`/`activeWorkspaceId` bila perlu untuk nama target (lihat di bawah).
- Tombol lencana (`<button class="badge" aria-haspopup="dialog" aria-expanded={open}>`) berteks `Antrean: {$pendingCount}`; bila `$pendingCount === 0` tampil redup (`var(--fg-muted, #9b9ba6)`), bila > 0 beraksen (`var(--accent, #5b94ff)`) dan, bila ada pesan `awaiting_confirm`, titik penanda `#e0a030`. Atribut `title` menjelaskan; bila `$queueError` ada, tampilkan sebagai `title` dan ikon kecil peringatan, tidak crash.
- Klik mengalihkan `open`. Saat `open`, render daftar sebagai popover `position: fixed` (footer statusbar memotong anak absolut karena `overflow: hidden`). Hitung posisi saat membuka dari `btn.getBoundingClientRect()`: `left = Math.min(rect.left, window.innerWidth - lebar - 8)`, `bottom = window.innerHeight - rect.top + 4`; lebar tetap `min(420px, 100vw - 16px)`, `max-height: 50vh`, `overflow: auto`. Tutup dengan `Escape` (listener `keydown` pada `window` dipasang via `<svelte:window onkeydown>`) dan klik di luar (`<svelte:window onpointerdown>` memeriksa `popoverEl.contains(e.target)` dan `btn.contains(e.target)`).
- Isi popover: bila daftar kosong, teks "Tidak ada pesan tertunda". Tiap pesan (`{#each $pendingMessages as m (m.id)}`): baris atas `dari {m.fromName || m.from}` -> `ke {targetLabel(m.to)}`, lencana state (`queued` = "Antre", `awaiting_confirm` = "Menunggu konfirmasi"), baris teks `{preview(m.text)}` (satu baris, potong 100 karakter + `…`, ganti newline dengan spasi, `title` berisi teks penuh), dan tombol: "Konfirmasi" hanya bila `m.state === 'awaiting_confirm'`; "Batalkan" untuk `queued` dan `awaiting_confirm`. `targetLabel(id)`: gunakan nama mention bila diketahui; karena `mention.queue` hanya membawa id, buat peta lokal `nameById` dari `listNames($activeWorkspaceId)` yang dimuat saat popover dibuka dan setiap `$pendingMessages` berubah selama terbuka (bukan timer); bila tak ada, tampilkan 8 karakter pertama id sesi.
- Aksi: `async function act(fn, id)` set `rowBusy[id] = true`, `const err = await fn(id)`, `rowError[id] = err`, `rowBusy[id] = false`. Tombol `disabled` saat `rowBusy[id]`. Galat per baris tampil kecil berwarna `var(--danger, #ff6b81)`. Baris hilang sendiri ketika event `mention.queue` baru tiba.
- Komponen berada dalam footer setinggi 24px: `font-size: 12px`, `line-height: 1`, tombol tanpa padding vertikal besar, tanpa border tebal.
- Teks dirender tanpa `{@html}`.

### 6. `index.ts`

```ts
import { registerPanel } from '../../lib/registry';
import MentionComposer from './MentionComposer.svelte';
import QueueBadge from './QueueBadge.svelte';

registerPanel({ id: 'mention-composer', area: 'sidebar', component: MentionComposer, order: 30 });
registerPanel({ id: 'mention-queue', area: 'statusbar', component: QueueBadge, order: 20 });
```

Tidak mendaftar `registerTabType`.

### 7. Verifikasi build

- `cd frontend && npm run build` harus lolos. Jalankan juga `npm run check` (svelte-check) dan perbaiki error/peringatan di file milik sendiri (jangan memperbaiki file modul lain).
- `grep -rn "setInterval\|setTimeout" frontend/src/modules/mention` harus kosong (tanpa timer sama sekali; `queueMicrotask`/`tick` boleh).
- `grep -rn "@html" frontend/src/modules/mention` harus kosong.
- Pastikan tidak ada impor dari `modules/<modul lain>` dan tidak ada perubahan di luar `frontend/src/modules/mention/` (periksa daftar file yang berubah).

## Perubahan

Dibuat (semua baru, di `frontend/src/modules/mention/`):

- `index.ts`
- `types.ts`
- `mention-client.ts`
- `queue-store.ts`
- `MentionComposer.svelte`
- `QueueBadge.svelte`

Tidak ada file yang diubah di luar path ini.

## Wiring

Yang diekspos untuk disambungkan nanti (jangan disambungkan sendiri, dan memang tidak ada yang perlu diedit):

- `modules/mention/index.ts` dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts')` di `main.ts`; ia mendaftarkan panel `mention-composer` (sidebar, order 30) dan `mention-queue` (statusbar, order 20), yang dirender oleh `SidebarArea.svelte` dan `StatusBar.svelte` lewat `panels(area)`.
- Kontrak ke backend (diverifikasi langkah wiring terhadap `internal/mention/ipc.go`): method `mention.names`, `mention.send`, `mention.queue.list`, `mention.queue.cancel`, `mention.confirm` dan event `mention.queue` dengan payload `{sessionId, queue}`; ditambah `session.attach` dan event `session.exit` yang sudah ada.
- Catatan untuk wiring: nama variabel CSS yang dipakai adalah yang ada di `frontend/src/styles/theme.css`, dengan nilai cadangan; tidak ada perubahan tema diperlukan.

## Selesai bila

- Keenam file ada di `frontend/src/modules/mention/` dan tidak ada file lain di repo yang disentuh.
- `cd frontend && npm run build` sukses (dan `npm run check` tanpa error baru dari path ini), juga bila backend mention belum ada.
- Mengetik `@` di composer membuka autocomplete dari `mention.names` workspace aktif; panah, Enter/Tab, Escape, dan klik bekerja.
- `@nama teks` memanggil `mention.send` dengan `{workspaceId, to: 'nama', text: 'teks'}` tanpa `from`, dan hasilnya (terkirim/antre/menunggu konfirmasi/gagal) tampil singkat; error IPC dan `inWails === false` menjadi pesan singkat tanpa crash.
- Opsi "Sertakan output terakhir sesi aktif" menambahkan output dari `session.attach` (base64 didekode, ANSI dibuang) ke pesan.
- Lencana statusbar menampilkan jumlah pesan pending: muat awal `mention.queue.list`, selanjutnya hanya event `mention.queue` lewat `onEvent`; tombol Konfirmasi (hanya `awaiting_confirm`) dan Batalkan (`queued`/`awaiting_confirm`) bekerja dan daftar ter-update dari event.
- Daftar nama dimuat ulang saat `activeWorkspaceId` berubah, saat store `sessions` berubah, dan saat event `session.exit`.
- Tidak ada `setInterval`/`setTimeout`; semua langganan (`onEvent`, event store) dilepas saat komponen di-destroy; tidak ada dependency baru; panel terdaftar hanya lewat `registerPanel` di `index.ts`.
