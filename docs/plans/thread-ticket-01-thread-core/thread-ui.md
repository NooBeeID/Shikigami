# Plan: UI daftar thread, dialog baru, dan tab chat

## Metadata

- Tiket: `thread-ticket-01-thread-core` (`docs/tickets/thread-ticket-01-thread-core.md`)
- Fitur: `thread-ui`
- Rujukan kontrak: `docs/plans/thread-ticket-01-thread-core/overview.md` (bagian 2.2 koreksi, 2.4 kontrak IPC, 2.5 kontrak frontend, 6 guard). Jangan mengubah overview.
- ownedPaths: `frontend/src/modules/thread/`, `frontend/src/lib/registry.ts` (hanya satu baris union `'thread'`)
- Depends on foundation: YA. Foundation (`00-foundation.md`) sudah menambah `PaneKind` `'thread'`, `threadId` pada `Leaf`/`NewLeaf`, dedup `openPane` per `threadId`, serta `registerSidebarSection`/`sidebarSections` di `modules/sidebar-left/registry.ts` (diekspor dari `modules/sidebar-left/index.ts`). Fitur ini murni frontend; tidak butuh daemon thread untuk lulus build.

## Tujuan

Menyediakan seluruh UI thread: bagian "Thread" di sidebar kiri (tombol "Thread baru", daftar thread workspace aktif dengan indikator), dialog pembuatan thread (goal teks bebas), dan tab chat (header, aliran pesan satu kolom dengan avatar berwarna, input) yang diperbarui real-time lewat event `thread.updated` / `thread.message`. Menyediakan mekanisme slot (`registerThreadSlot` + `ThreadSlot.svelte`) agar tiket thread-02 s.d. thread-05 memasang komponen tanpa mengedit berkas lama. Galat IPC tampil sebagai teks terbaca, bukan crash.

## Path yang dimiliki

HANYA ini yang boleh disentuh; jangan sentuh file wiring atau path fitur lain:

- `frontend/src/modules/thread/` (semua berkas baru di bawahnya)
- `frontend/src/lib/registry.ts` (hanya mengubah satu baris union tipe)

Dilarang: `modules/layout/*`, `modules/sidebar-left/*`, `lib/stores/*`, `main.ts`, `internal/**`, `cmd/**`, `package.json`. Bila butuh perubahan di luar path ini, catat di bagian "Catatan untuk wiring" di akhir berkas plan ini versi kerja Anda (jangan ubah path itu).

## Kontrak

Dipakai dari kode yang ada (sudah diverifikasi ada):

- `lib/ipc.ts`: `call<T>(method, params)`, `onEvent(topicPrefix, (topic, payload) => void)` (mengembalikan fungsi berhenti), `inWails`.
- `lib/stores`: `activeWorkspaceId`, `activeWorkspace`, `sessions` (`lib/stores/workspaces.ts`, `sessions.ts` via `lib/stores/index.ts`); `layout` dari `lib/stores/layout.ts` (`layout.openPane(NewLeaf)`).
- `modules/layout/registry.ts`: `registerPaneKind({kind, component, titleFor})`; komponen pane menerima props `{pane: Leaf, visible: boolean}`.
- `modules/layout/model.ts` (Foundation): `Leaf.threadId?`, `NewLeaf` memuat `kind`, `threadId`, `workspaceId`, `title`.
- `modules/sidebar-left`: `registerSidebarSection({id, component, order?})`; komponen section menerima props `{onerror: (msg: string) => void}`.
- `modules/status/store.ts`: `statusStore` (`Record<sessionId, {status: 'running'|'waiting'|'done'|'idle', since}>`; sesi tak terdaftar dianggap idle) untuk indikator aktif/idle master.
- Token tema (`styles/theme.css`): `--bg`, `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--danger`, `--ok`. Tidak ada warna keras selain palet avatar HSL dari hash.
- `lib/registry.ts`: `registerTabType`, `TabTypeSpec`.

Kontrak IPC (overview 2.4), semuanya JSON camelCase:

- `thread.create{workspaceId, goal, title?}` -> `Thread`; `thread.list{workspaceId}` -> `Thread[]`; `thread.get{id}` -> `Thread`; `thread.messages{id, sinceTs?}` -> `ThreadMessage[]`; `thread.send{id, text}` -> `ThreadMessage`; `thread.ownedSessions{}` -> `{sessionIds: string[]}`.
- Event: `thread.updated` (payload `Thread`), `thread.message` (payload `ThreadMessage`).

Disediakan fitur ini (publik modul, dipakai tiket berikut):

- `types.ts`: `Thread`, `Participant`, `ThreadMessage`, `ThreadState`, `ThreadRole`, `ThreadKind`, `THREAD_METHODS`, `THREAD_EVENTS`.
- `thread-client.ts`: `asList`, `errorText`, `createThread`/`listThreads`/`getThread`/`listMessages`/`sendThread`/`ownedSessions`, `onThreadUpdated`, `onThreadMessage`.
- `thread-store.ts`: `threads`, `messagesByThread`, `threadError`, `loadThreads`, `loadMessages`, `createThread`, `sendMessage`, `openThreadPane`, `subscribeThreadEvents`, `threadById`, `participantName`.
- `slots.ts`: `registerThreadSlot({slot, component, order?})`, `threadSlots(name)`; `ThreadSlot.svelte` props `{name, thread, message?}`. Komponen slot menerima props `{thread: Thread, message?: ThreadMessage}`.
- Nama slot: `'header-actions' | 'above-input' | 'message-extra' | 'side-panel'`.

## Langkah

Semua teks UI, komentar, dan galat bahasa Indonesia; identifier bahasa Inggris. Svelte 5 runes (`$props`, `$state`, `$derived`, `$effect`), TypeScript. Tanpa dependency baru.

### 1. `frontend/src/lib/registry.ts`

Ubah satu baris: `export interface TabTypeSpec { type: 'session' | 'file' | 'thread'; component: Component<any>; }`. Tidak ada perubahan lain.

### 2. `frontend/src/modules/thread/types.ts`

Cermin `types.go`:

```ts
export type ThreadState = 'active' | 'closed';
export type ThreadRole = 'master' | 'peer';
export type ThreadKind = 'message' | 'system' | 'activity';
export interface Participant { sessionId: string; projectId: string; role: ThreadRole; name: string }
export interface Thread {
  id: string; workspaceId: string; title: string; goal: string; state: ThreadState;
  createdAt: string; closedAt?: string; participants: Participant[]; readyToClose: boolean;
}
export interface ThreadMessage {
  id: string; threadId: string; from: string; to: string; text: string; ts: string; kind: ThreadKind;
}
export const THREAD_METHODS = { create: 'thread.create', list: 'thread.list', get: 'thread.get',
  messages: 'thread.messages', send: 'thread.send', ownedSessions: 'thread.ownedSessions' } as const;
export const THREAD_EVENTS = { updated: 'thread.updated', message: 'thread.message' } as const;
export const FROM_USER = 'user'; export const FROM_SYSTEM = 'system'; export const TO_USER = 'user'; export const TO_ALL = 'all';
```

Komentar: bentuk dicerminkan dari `internal/thread/types.go`; tiket lain hanya menambah field opsional.

### 3. `thread-client.ts`

Meniru `modules/mention/mention-client.ts`:

- `export function asList<T>(res: unknown, key?: string): T[]` (array langsung atau `res[key]`, selain itu `[]`).
- `export function errorText(e: unknown): string`: ekstrak pesan aman (try/catch); kosong -> "Gagal menghubungi daemon"; berisi "bukan di dalam Wails" -> "Hanya tersedia di aplikasi desktop"; `/ERR_INVALID|tidak valid/i` -> bila pesan memuat "ditutup" kembalikan "Thread sudah ditutup", selain itu "Masukan tidak valid (goal/pesan kosong atau workspace tidak dikenal)"; `/ERR_NOT_FOUND|tidak ditemukan/i` -> "Thread tidak ditemukan"; selain itu `msg.slice(0, 160)`.
- Pembungkus tipis atas `call`: `listThreads(workspaceId): Promise<Thread[]>` (pakai `asList`, normalisasi `participants` null -> `[]`), `getThread(id)`, `createThreadRpc(workspaceId, goal, title?)`, `listMessages(id, sinceTs?)` (`asList`), `sendThread(id, text): Promise<ThreadMessage>`, `ownedSessions(): Promise<string[]>` (`asList(res, 'sessionIds')`).
- `onThreadUpdated(fn: (t: Thread) => void): () => void` dan `onThreadMessage(fn: (m: ThreadMessage) => void): () => void` lewat `onEvent('thread.updated' | 'thread.message', ...)`; validasi payload objek dengan `id` string (thread) / `id` dan `threadId` string (pesan), abaikan selain itu; normalisasi `participants`.

### 4. `thread-store.ts`

- State: `export const threads = writable<Thread[]>([])` (hanya workspace aktif, urut `createdAt` menurun), `export const messagesByThread = writable<Record<string, ThreadMessage[]>>({})`, `export const threadError = writable('')`.
- Helper murni: `sortThreads`, `mergeMessages(list, incoming)` (dedup by `id`, urut `ts` naik; ts sama dipatahkan `id` leksikografis).
- `loadThreads(workspaceId: string | null): Promise<void>`: bila null -> `threads.set([])`; bila `!inWails` set `threadError('Hanya tersedia di aplikasi desktop')`; selain itu `listThreads`, set, kosongkan error; tangkap galat -> `threadError(errorText(e))`. Pakai penghitung generasi agar respons lama tidak menimpa respons baru saat workspace berganti.
- `loadMessages(threadId): Promise<void>`: muat penuh `listMessages(threadId)` lalu `mergeMessages` dengan yang sudah ada (event yang tiba lebih dulu tidak hilang). Kembalikan galat lewat `threadError`.
- `createThread(goal: string): Promise<Thread>`: trim; kosong -> `throw new Error('Goal tidak boleh kosong')`; workspace aktif dari `get(activeWorkspaceId)` (null -> `throw new Error('Pilih workspace terlebih dahulu')`); panggil RPC; upsert ke `threads`; `openThreadPane(t)`; kembalikan `Thread`. Galat dilempar mentah; pemanggil (dialog) memakai `errorText`.
- `sendMessage(threadId, text): Promise<void>`: trim, tolak kosong; `sendThread`; masukkan `ThreadMessage` hasil ke `messagesByThread` lewat `mergeMessages` (event `thread.message` yang menyusul tidak menduplikasi karena dedup by id). Galat dilempar mentah.
- `openThreadPane(t: Thread): void`: `layout.openPane({kind: 'thread', threadId: t.id, workspaceId: t.workspaceId, title: t.title})`.
- `threadById(id): Readable<Thread | undefined>` (derived dari `threads`) dan `participantName(thread: Thread | undefined, id: string): string`: `'user'` -> "user"; `'system'` -> "sistem"; `'all'` -> "semua"; cocokkan `participants[].sessionId` -> `name`; peran master tanpa nama -> "master"; fallback 8 karakter pertama id.
- `subscribeThreadEvents(): () => void`: idempoten (referensi tunggal; panggilan kedua mengembalikan pembersih yang sama). Berlangganan `onThreadUpdated` (upsert thread ke `threads` hanya bila `workspaceId` sama dengan workspace aktif; bila thread tidak ada di daftar dan cocok workspace, tambahkan) dan `onThreadMessage` (merge ke `messagesByThread[threadId]` bila array thread itu sudah dimuat ATAU buat entri baru; dedup by id). Berlangganan dahulu, baru muat awal: `loadThreads(get(activeWorkspaceId))`. Juga `activeWorkspaceId.subscribe` -> `loadThreads` saat berganti. Pembersih melepas semua langganan.
- Semua akses store di luar komponen memakai `get` dari `svelte/store`. Jangan memakai timer/polling.

### 5. `slots.ts` dan `ThreadSlot.svelte`

`slots.ts`:

```ts
export type ThreadSlotName = 'header-actions' | 'above-input' | 'message-extra' | 'side-panel';
export interface ThreadSlotSpec { slot: ThreadSlotName; component: Component<any>; order?: number; id?: string }
export function registerThreadSlot(spec: ThreadSlotSpec): void
export function threadSlots(name: ThreadSlotName): Readable<ThreadSlotSpec[]>
```

Store `writable<ThreadSlotSpec[]>`; registrasi menambah (bila `id` diberikan dan sudah ada, ganti). `threadSlots` = derived terfilter per nama, terurut `order ?? 0` lalu urutan registrasi. Komentar panjang di kepala berkas menjelaskan kontrak slot untuk tiket lain (nama slot, props `{thread, message?}`, bahwa `message-extra` dirender per pesan dengan `message`, `side-panel` dirender di kolom kanan tab dan kolom itu hanya muncul bila ada komponen terdaftar).

`ThreadSlot.svelte`: props `{name: ThreadSlotName, thread: Thread, message?: ThreadMessage}`; `const list = $derived(threadSlots(name))`-style (gunakan `$: ` setara runes: buat store sekali per `name` dengan `$derived` + `$list` via `get`/subscribe; pola aman: `const store = $derived(threadSlots(name));` lalu `{#each $store as s (s.id ?? s.order ...)}` — bila auto-subscription pada derived store tidak diizinkan type-check, gunakan `$effect` yang subscribe dan menyimpan ke `$state`). Render `{@const C = s.component}<C {thread} {message} />`. Tanpa slot terdaftar tidak ada keluaran DOM.

Tambahkan fungsi kecil `hasThreadSlot(name): Readable<boolean>` di `slots.ts` untuk keputusan menampilkan kolom `side-panel`.

### 6. `ThreadSection.svelte` (sidebar kiri)

Props `{onerror}: {onerror: (msg: string) => void}`. Layout: judul "Thread" (kecil, `--fg-muted`, huruf besar kecil gaya sidebar yang ada), tombol "Thread baru" (type button; disabled bila tidak ada workspace aktif). Daftar `$threads`: tiap baris tombol dengan judul (ellipsis), indikator titik kecil: master berstatus `running` -> `--ok` berkedip pelan atau solid ("aktif"), selain itu `--fg-muted` ("idle"); thread `closed` -> baris dengan `opacity: 0.5` dan label "ditutup". Master = `participants.find(p => p.role === 'master')`; status dibaca dari `statusStore` (`$statusStore[sessionId]?.status`). Klik -> `openThreadPane(t)` (dedup oleh Foundation memfokuskan tab yang ada). State kosong: teks muted "Belum ada thread". `threadError` non-kosong ditampilkan satu baris kecil `--danger`; juga panggil `onerror` hanya untuk galat aksi (bukan galat muat berulang). Mengelola dialog: `let dialogOpen = $state(false)`; render `<NewThreadDialog open={dialogOpen} onclose={() => (dialogOpen = false)} />`. Gunakan `$effect` yang memanggil `loadThreads($activeWorkspaceId)` hanya bila perlu (store sudah memuat saat workspace berganti, jadi cukup pastikan `subscribeThreadEvents` aktif; jangan menggandakan pemuatan). Gaya: padding horizontal selaras `WorkspaceHeader`/`SessionRow` (lihat berkasnya untuk ukuran font dan hover `--bg-hover`).

### 7. `NewThreadDialog.svelte`

Props `{open: boolean, onclose: () => void}`. Overlay modal (`role="dialog"`, `aria-modal`, fokus otomatis ke textarea saat dibuka, `Escape` menutup, klik latar menutup bila tidak sedang mengirim). Isi: judul "Thread baru", label "Goal", textarea bebas (placeholder contoh "Jelaskan tujuan yang melibatkan beberapa project..."), pesan galat `--danger` (`errorText`), tombol "Batal" dan "Buat" (disabled bila goal kosong setelah trim atau sedang mengirim; teks "Membuat..." saat berjalan). `Ctrl/Cmd+Enter` = Buat. Aksi Buat: validasi goal kosong -> galat "Goal tidak boleh kosong"; `await createThread(goal)` (store membuka tab); sukses -> reset field dan `onclose()`; gagal -> `error = errorText(e)` dialog tetap terbuka dan goal tidak hilang. Cegah kirim ganda lewat flag `busy`. Reset state saat `open` berubah menjadi true.

### 8. `ThreadPane.svelte` dan `ThreadTab.svelte`

`ThreadPane.svelte`: props `{pane, visible}: {pane: Leaf; visible: boolean}`; `{#if pane.threadId}<ThreadTab threadId={pane.threadId} {visible} />{:else}<div class="empty">Thread tidak tersedia</div>{/if}`.

`ThreadTab.svelte`: props `{threadId: string, visible?: boolean}`. Ambil `thread` dari `$threads` (cari by id). Jika tidak ada (mis. pemulihan layout, workspace lain, atau belum dimuat): panggil `getThread(threadId)` sekali di `$effect`, tampilkan "Memuat thread..." selama berjalan, upsert hasilnya ke `threads` bila `workspaceId` aktif; kalau galat tampilkan `errorText` dan tombol "Coba lagi". Pada mount (dan saat `threadId` berubah) panggil `loadMessages(threadId)`; pastikan `subscribeThreadEvents()` sudah dipanggil `index.ts` (jangan memanggilnya di sini). Struktur: kolom (`display:flex; flex-direction: column; height:100%; min-height:0`) berisi `ThreadHeader`, area tengah dua kolom (kiri: `MessageList`; kanan: `<ThreadSlot name="side-panel" .../>` dibungkus aside lebar tetap ~260px yang hanya dirender bila `hasThreadSlot('side-panel')`), lalu `<ThreadSlot name="above-input" />`, lalu `ThreadInput`. Kirim pesan: handler `onsend(text)` memanggil `sendMessage`, galat ditampilkan di atas input (`errorText`), teks input TIDAK dihapus saat gagal.

### 9. `ThreadHeader.svelte`

Props `{thread: Thread}`. Menampilkan judul (ellipsis), lencana status: "aktif" (`--ok`) atau "ditutup" (`--fg-muted`), dan bila `thread.readyToClose` lencana kecil "siap ditutup" (banner sebenarnya milik thread-close lewat slot). Di kanan: `<ThreadSlot name="header-actions" {thread} />`. Goal ditampilkan sebagai satu baris muted yang bisa dibuka-tutup (`<details>`), teks polos.

### 10. `MessageList.svelte`

Props `{thread: Thread, messages: ThreadMessage[]}`. Kontainer gulir (`overflow-y:auto`, `min-height:0`, `flex:1`). Gulir otomatis ke bawah saat jumlah pesan bertambah KECUALI user sedang menggulir ke atas: lacak `stick = $state(true)`; `onscroll` menghitung `stick = scrollHeight - scrollTop - clientHeight < 48`; `$effect` terhadap `messages.length` memanggil `tick()` lalu `el.scrollTop = el.scrollHeight` hanya bila `stick`. Saat pertama kali muat, gulir ke bawah. Tampilkan tombol kecil "Ke pesan terbaru" saat `!stick` dan ada pesan baru. State kosong: "Belum ada pesan". Render `{#each messages as m (m.id)}<MessageItem {thread} message={m} />{/each}`.

### 11. `MessageItem.svelte`

Props `{thread: Thread, message: ThreadMessage}`.

- `kind === 'system'` atau `kind === 'activity'`: satu baris terpusat/muted kecil berisi teks (dan waktu), tanpa avatar. Setelah itu tetap render `<ThreadSlot name="message-extra" {thread} {message} />` (thread-activity memasang tampilan kartu lewat slot ini; di sini `activity` hanya baris sistem minimal).
- `kind === 'message'`: avatar bulat (inisial 1-2 huruf dari nama pengirim) dengan warna deterministik: `hue = hash(senderKey) % 360`, `background: hsl(hue 55% 45%)`, teks putih; `senderKey` = `message.from` (sessionId atau 'user'). Label `"{pengirim} -> {penerima}"` memakai `participantName(thread, ...)` (tulis panah sebagai teks `→` boleh, tetapi gunakan karakter yang ada di berkas lain; pakai `->` bila ragu), waktu lokal `HH:MM` dari `ts` (`toLocaleTimeString` dengan `hour:'2-digit', minute:'2-digit'`; `ts` tak valid -> kosong). Teks dirender sebagai teks polos lewat interpolasi Svelte `{message.text}` dengan CSS `white-space: pre-wrap; overflow-wrap: anywhere` (TIDAK memakai `{@html}`). Pesan dari user diberi gaya sedikit berbeda (latar `--bg-elev`). Lalu `<ThreadSlot name="message-extra" {thread} {message} />`.
- Fungsi hash (`hashString`) dan `avatarColor`/`initials` ditaruh di `types.ts`? Tidak: taruh sebagai fungsi ekspor di `thread-store.ts`-bebas berkas kecil `format.ts` (di dalam folder modul) agar `MessageItem` dan tiket lain (panel peserta) memakai warna yang sama. Ekspor: `avatarColor(key: string): string`, `initials(name: string): string`, `formatTime(ts: string): string`.

### 12. `ThreadInput.svelte`

Props `{disabled: boolean, busy?: boolean, error?: string, onsend: (text: string) => Promise<void> | void}`. Textarea auto-tinggi sederhana (maks ~8 baris); `Enter` tanpa Shift mengirim (abaikan saat `isComposing` / `e.isComposing` untuk IME), `Shift+Enter` baris baru. Tombol "Kirim" (disabled bila kosong/disabled/busy). `disabled` (thread closed) -> placeholder "Thread sudah ditutup", textarea `disabled`. Setelah `onsend` sukses (tidak melempar) kosongkan; bila melempar, pertahankan teks dan beri tahu pemanggil lewat `error` prop (pemanggil yang menampilkan). `ThreadTab` membungkus: `async function handleSend(text) { try { sendError=''; await sendMessage(...) } catch (e) { sendError = errorText(e); throw e } }` — `ThreadInput` menangkap lemparan itu agar tidak menjadi unhandled rejection dan tidak mengosongkan teks.

### 13. `index.ts` (satu-satunya titik registrasi)

```ts
import { registerPaneKind } from '../layout/registry';
import { registerSidebarSection } from '../sidebar-left';   // bila impor barrel menyebabkan siklus, impor dari '../sidebar-left/registry'
import { registerTabType } from '../../lib/registry';
import ThreadPane from './ThreadPane.svelte';
import ThreadSection from './ThreadSection.svelte';
import ThreadTab from './ThreadTab.svelte';
import { subscribeThreadEvents } from './thread-store';

registerPaneKind({ kind: 'thread', component: ThreadPane, titleFor: (leaf) => leaf.title || 'Thread' });
registerSidebarSection({ id: 'thread-sidebar', component: ThreadSection, order: 0 });
registerTabType({ type: 'thread', component: ThreadTab });   // jalur legacy; tidak dirender siapa pun saat ini
subscribeThreadEvents();
```

Jangan memanggil `registerPanel`. Re-ekspor API publik untuk tiket lain: `export { registerThreadSlot } from './slots'; export type { ThreadSlotName } from './slots'; export { threads, messagesByThread, openThreadPane, participantName } from './thread-store'; export * from './types'; export { avatarColor, initials, formatTime } from './format';`. Karena `ThreadTab` memakai prop `threadId` (bukan `{pane}`), tanda tangan `registerTabType` (`Component<any>`) tetap valid.

### 14. Verifikasi guard

`cd frontend && npm run check && npm run build` (jalankan `npm install` dulu hanya bila `node_modules` tidak ada; jangan mengubah `package.json`). Perbaiki semua galat type-check di berkas modul thread. Periksa `git status`: hanya `frontend/src/modules/thread/**` dan `frontend/src/lib/registry.ts` yang berubah.

## Perubahan

Diubah:

- `frontend/src/lib/registry.ts` (union `'thread'`)

Dibuat di `frontend/src/modules/thread/`:

- `types.ts`, `format.ts`, `thread-client.ts`, `thread-store.ts`
- `slots.ts`, `ThreadSlot.svelte`
- `ThreadSection.svelte`, `NewThreadDialog.svelte`
- `ThreadPane.svelte`, `ThreadTab.svelte`, `ThreadHeader.svelte`
- `MessageList.svelte`, `MessageItem.svelte`, `ThreadInput.svelte`
- `index.ts`

## Wiring

Fitur ini TIDAK menyambungkan apa pun sendiri di luar ownedPaths. Yang diekspos untuk langkah wiring:

- `frontend/src/modules/thread/index.ts` ter-import otomatis oleh `import.meta.glob('./modules/*/index.ts')` di `main.ts` (tidak diedit). Wiring memverifikasi bahwa `index.ts` memanggil `registerPaneKind({kind:'thread'})`, `registerSidebarSection({id:'thread-sidebar'})`, `registerTabType({type:'thread'})`, dan `subscribeThreadEvents()`, serta union `TabTypeSpec.type` memuat `'thread'`.
- Ketergantungan pada Foundation yang harus tetap utuh: `PaneKind` memuat `'thread'`, `threadId` dipertahankan `hydrateDoc`, `AgentSidebar` merender section.
- Titik ekstensi untuk tiket thread-02..05: `registerThreadSlot` (nama slot `header-actions`, `above-input`, `message-extra`, `side-panel`) dan komponen `ThreadSlot`; helper `participantName`, `avatarColor`, `initials`, `formatTime`; store `threads` dan `messagesByThread`.
- Tab thread yang ter-persist dipulihkan lewat `hydrateDoc`; `ThreadTab` memuat ulang thread via `getThread` bila belum ada di daftar.
- Daemon tidak wajib ada saat build; pengecekan manual alur penuh baru bermakna setelah `thread-master-daemon` selesai.

## Selesai bila

- `cd frontend && npm run check && npm run build` lulus (guard; tanpa unit test).
- `git status` hanya menampilkan perubahan di `frontend/src/modules/thread/` dan `frontend/src/lib/registry.ts`; `registerPanel` tidak dipanggil di modul thread; union `TabTypeSpec.type` memuat `'thread'`.
- Bagian "Thread" (tombol "Thread baru" + daftar thread workspace aktif dengan indikator aktif/idle, thread closed redup) tampil lewat `registerSidebarSection`.
- Dialog membuat thread lewat `thread.create`, membuka pane `kind:'thread'` lewat `layout.openPane`, dan galat IPC tampil lewat `errorText`.
- Tab thread menampilkan header, aliran pesan (avatar berwarna deterministik, "pengirim -> penerima", gaya sistem, teks polos), gulir otomatis kecuali user menggulir ke atas, pesan real-time via `thread.message` dengan dedup by `id`, input Enter/Shift+Enter yang dinonaktifkan untuk thread closed.
- `registerThreadSlot` + `ThreadSlot` tersedia dan terdokumentasi di komentar `slots.ts`.
