# Plan: Banner goal tercapai, tombol Close, dialog konfirmasi, dan tampilan read-only

## Metadata

- Tiket: `thread-ticket-04-thread-close` (`docs/tickets/thread-ticket-04-thread-close.md`)
- Fitur: `thread-close-ui`
- ownedPaths: `frontend/src/modules/thread/close/`
- Depends on foundation: ya. Foundation (`00-foundation.md`) sudah menambah `closeSummary?: string` pada `Thread` dan `THREAD_METHODS.closePreview`/`THREAD_METHODS.close` di `frontend/src/modules/thread/types.ts`, serta stub `frontend/src/modules/thread/close/index.ts` (`installClose` no-op). Plan ini MENIMPA stub itu.
- Rujukan kontrak: `docs/plans/thread-ticket-04-thread-close/overview.md` bagian 2.2 (no. 10, 11), 2.3 (TypeScript), 2.4. Jangan mengubah overview.

## Tujuan

Menyediakan seluruh UI penutupan thread di frontend: banner hijau "Goal tercapai" dengan ringkasan master dan tombol Close, tombol "Tandai selesai" di header, dialog konfirmasi yang mendaftar perubahan belum di-commit dan commit belum di-merge per worktree (dari `thread.closePreview`) lalu menutup lewat `thread.close`, penanda "Thread ditutup pada ..." untuk thread closed, chip `ReadyBadge`, helper `isThreadReadOnly`, dan `installClose()` yang mendaftarkan slot. Semua data berasal dari store thread (event `thread.updated`); tanpa polling/timer.

## Path yang dimiliki

HANYA `frontend/src/modules/thread/close/`. Dilarang menyentuh: `types.ts`, `slots.ts`, `thread-client.ts`, `thread-store.ts`, `format.ts`, `ThreadTab.svelte`, `ThreadHeader.svelte`, `ThreadInput.svelte`, `ThreadSection.svelte`, `frontend/src/modules/thread/index.ts` (file wiring), semua file Go (`internal/thread/close.go`, `close_ipc.go` milik fitur `thread-close-daemon`), dan folder `participants/`, `activity/`. Butuh perubahan di luar path: jangan dikerjakan; catat di bagian "Catatan untuk wiring" pada laporan akhir.

## Kontrak

Dipakai (SUDAH ada di kode; verifikasi nama sebelum menulis):

- `../slots`: `registerThreadSlot({slot, component, order?, id?})`; komponen slot menerima props `{thread: Thread, message?: ThreadMessage}`. Registrasi dengan `id` yang sama menimpa (idempoten).
- `../types`: `Thread` (field `id`, `state: 'active'|'closed'`, `closedAt?`, `readyToClose`, `closeSummary?`, `participants`), `THREAD_METHODS` (`closePreview`, `close`).
- `../thread-client`: `errorText(e: unknown): string`, `asList<T>(res, key?)`.
- `../../../lib/ipc`: `call<T>(method, params)`.
- Token tema (hanya dengan fallback `var(...)` seperti komponen lain): `--ok #30a46c`, `--danger #e5484d`, `--fg #222`, `--fg-muted #888`, `--bg #fff`, `--bg-elev #fff`, `--border #8884`, `--bg-hover #0001`, `--accent #3b82f6`.
- Pola dialog: `NewThreadDialog.svelte` (overlay `position: fixed; inset: 0; z-index: 100`, `role="dialog" aria-modal="true"`, Escape menutup bila tidak sibuk, klik overlay menutup bila tidak sibuk).
- Daemon (fitur paralel, hanya lewat bentuk IPC): `thread.closePreview{id}` -> `ClosePreview`; `thread.close{id}` -> `Thread` (closed). Slice di respons bisa `null`; frontend menormalisasi ke `[]`.

Disediakan (signature persis, jangan diubah):

```ts
// closeApi.ts
export interface CloseUncommitted { path: string; status: string }
export interface CloseCommit { sha: string; subject: string }
export interface CloseWorktreeEntry {
  participant: { sessionId: string; name: string; projectId: string };
  path: string; branch: string; baseBranch: string;
  uncommitted: CloseUncommitted[]; unmergedCommits: CloseCommit[]; error?: string;
}
export interface ClosePreview { threadId: string; worktrees: CloseWorktreeEntry[] }
export function fetchClosePreview(threadId: string): Promise<ClosePreview>;
export function closeThread(threadId: string): Promise<Thread>;
// readOnly.ts
export function isThreadReadOnly(thread: Thread): boolean;
// closeDialogStore.ts
export const closeDialogFor: Readable<string | null>;
export function openCloseDialog(threadId: string): void;
export function closeCloseDialog(): void;
// index.ts
export function installClose(): () => void;
export { default as ReadyBadge } from './ReadyBadge.svelte';
export { isThreadReadOnly } from './readOnly';
```

Slot yang didaftarkan `installClose()` (id tetap):

| slot | id | komponen | order |
|---|---|---|---|
| `header-actions` | `close-button` | `CloseButton` | 100 |
| `above-input` | `close-banner` | `CloseBanner` | -10 |
| `above-input` | `close-notice` | `ClosedNotice` | 0 |

## Langkah

Berurutan. Sebelum mulai: pastikan `frontend/src/modules/thread/close/index.ts` (stub Foundation) ada dan `types.ts` memuat `closeSummary` dan `THREAD_METHODS.closePreview/close`. Bila Foundation belum ada, laporkan blocker; jangan menyentuh `types.ts`. Semua komentar dan teks UI bahasa Indonesia; identifier bahasa Inggris. Svelte 5 runes (`$props`, `$state`, `$derived`, `$effect`); tidak ada `setInterval`/`setTimeout`/polling.

1. **`closeApi.ts`**
   - Impor `call` dari `'../../../lib/ipc'`, `THREAD_METHODS` dan tipe `Thread` dari `'../types'`.
   - Ekspor tipe pada bagian Kontrak.
   - `fetchClosePreview(threadId)`: `const res = await call<Partial<ClosePreview>>(THREAD_METHODS.closePreview, { id: threadId })`. Kembalikan `{ threadId: res?.threadId ?? threadId, worktrees: asArray(res?.worktrees).map(normEntry) }`. `normEntry` menjamin `uncommitted` dan `unmergedCommits` selalu array (null -> `[]`), `participant` punya default `{sessionId:'', name:'', projectId:''}`, `baseBranch` string (default `''`), `error` tetap opsional. Helper privat `asArray<T>(v: unknown): T[]` (`Array.isArray(v) ? v : []`).
   - `closeThread(threadId)`: `const t = await call<Thread>(THREAD_METHODS.close, { id: threadId }); return { ...t, participants: Array.isArray(t?.participants) ? t.participants : [] }`.
   - Galat dilempar apa adanya; komponen menampilkannya lewat `errorText`.

2. **`readOnly.ts`**: `export function isThreadReadOnly(thread: Thread): boolean { return thread.state === 'closed'; }` (impor tipe saja).

3. **`closeDialogStore.ts`**
   - `const current = writable<string | null>(null)`.
   - `export const closeDialogFor: Readable<string | null> = { subscribe: current.subscribe }`.
   - `openCloseDialog(threadId)` -> `current.set(threadId)`; `closeCloseDialog()` -> `current.set(null)`.

4. **`ReadyBadge.svelte`** (props `{ thread }: { thread: Thread }`): `{#if thread.readyToClose && thread.state === 'active'}<span class="chip" title="Master mengusulkan penutupan thread">siap ditutup</span>{/if}`. Gaya: font 11px, `color: var(--ok, #30a46c)`, `border: 1px solid var(--ok, #30a46c)`, `border-radius: 10px`, `padding: 0 7px`, `flex: none`, `white-space: nowrap`. Tidak merender apa pun bila syarat tidak terpenuhi.

5. **`CloseBanner.svelte`** (props `{ thread }`)
   - Render hanya bila `thread.readyToClose && thread.state === 'active'`.
   - Struktur: `<div class="banner" role="status">` berisi judul "Goal tercapai", `{#if thread.closeSummary}<div class="summary">{thread.closeSummary}</div>{/if}`, dan `<button type="button" class="close-btn" onclick={() => openCloseDialog(thread.id)}>Close</button>`.
   - Gaya: `border: 1px solid var(--ok, #30a46c)`, `border-left-width: 4px`, `background: color-mix(in srgb, var(--ok, #30a46c) 12%, transparent)`, `border-radius: 6px`, `margin: 6px 12px`, `padding: 8px 10px`, `display: flex; align-items: flex-start; gap: 10px`; teks `color: var(--fg, #222)`; judul bold 12px berwarna `var(--ok, #30a46c)`; `.summary` `white-space: pre-wrap; overflow-wrap: anywhere; font-size: 12px`, teks dibungkus div `flex: 1; min-width: 0`; tombol `flex: none`, gaya tombol seperti `NewThreadDialog` (border `var(--border, #8884)`), `:hover` `background: var(--bg-hover, #0001)`.

6. **`CloseButton.svelte`** (props `{ thread }`)
   - Impor `CloseDialog`, `closeDialogFor`, `openCloseDialog`.
   - `{#if thread.state === 'active'}<button type="button" class="btn" title="Tutup thread ini" onclick={() => openCloseDialog(thread.id)}>Tandai selesai</button>{/if}`.
   - SELALU render `<CloseDialog {thread} open={$closeDialogFor === thread.id} />` di luar `{#if}` (host dialog walau tombol disembunyikan untuk thread closed, supaya dialog yang sedang terbuka tidak hilang mendadak saat event closed tiba; dialog sendiri menutup diri lewat `closeCloseDialog()` setelah sukses).
   - Gaya tombol: font 12px, `border: 1px solid var(--border, #8884)`, `border-radius: 6px`, `padding: 2px 10px`, transparan, hover `var(--bg-hover, #0001)`.

7. **`CloseDialog.svelte`** (props `{ thread, open }: { thread: Thread; open: boolean }`)
   - State: `status = $state<'idle'|'loading'|'ready'|'error'>('idle')`, `preview = $state<ClosePreview | null>(null)`, `loadError = $state('')`, `busy = $state(false)`, `closeError = $state('')`, `let gen = 0` (non-reaktif, guard generasi).
   - `async function load()`: `const my = ++gen; status='loading'; loadError=''; try { const p = await fetchClosePreview(thread.id); if (my !== gen) return; preview = p; status='ready'; } catch (e) { if (my !== gen) return; loadError = errorText(e); status='error'; }`.
   - `$effect`: bergantung hanya pada `open`. Bila `open` berubah menjadi true: reset `preview=null; closeError=''; busy=false;` lalu `void load()` (panggilan `fetchClosePreview` SEKALI per pembukaan; bungkus pemanggilan `load()` dengan `untrack` dari `svelte` agar `thread`/state lain tidak memicu ulang efek). Bila `open` false: `gen++` (membatalkan respons usang) dan `status='idle'`. Tombol "Coba lagi" (hanya di status `error`) memanggil `load()`.
   - `async function confirmClose()`: `if (busy || status !== 'ready') return; busy = true; closeError = ''; try { await closeThread(thread.id); closeCloseDialog(); } catch (e) { closeError = errorText(e); } finally { busy = false; }`. Dialog TIDAK menutup pada galat; tombol aktif lagi karena `busy=false`. Guard `busy` mencegah klik ganda (tambahan `disabled={busy}`).
   - `cancel()`: `if (busy) return; closeCloseDialog();` (tidak memanggil `closeThread`).
   - Escape (`onkeydown` pada overlay, `tabindex="-1"` dan fokuskan overlay/tombol Batal saat dibuka agar keydown terterima): menutup lewat `cancel()`. Klik overlay (`e.target === e.currentTarget`) memanggil `cancel()`.
   - Markup (dalam `{#if open}`): overlay + `div.dlg role="dialog" aria-modal="true" aria-label="Tutup thread"`:
     - `h3` "Tutup thread" dan teks ringkas "Periksa pekerjaan yang belum dipindahkan sebelum menutup.".
     - Peringatan merah permanen (selalu tampil): `<div class="danger" role="alert">` berisi "Semua worktree dan branch thread ini akan dihapus PERMANEN dan semua agent thread dimatikan. Perubahan yang belum di-commit dan commit yang belum di-merge akan hilang." Gaya: `color: var(--danger, #e5484d)`, `border: 1px solid var(--danger, #e5484d)`, `background: color-mix(in srgb, var(--danger, #e5484d) 10%, transparent)`, `border-radius: 6px`, padding 8px, font 12px.
     - Area isi dengan `max-height: 50vh; overflow-y: auto; overflow-x: hidden` (tanpa scroll horizontal):
       - `loading`: "Memeriksa worktree...".
       - `error`: pesan `loadError` (warna danger) + tombol "Coba lagi". Tombol "Lanjut tutup" nonaktif selama status bukan `ready`.
       - `ready` dengan `preview.worktrees.length === 0`: "Tidak ada worktree peserta; tidak ada yang akan hilang.".
       - `ready` per entri (`{#each preview.worktrees as w (w.path || w.participant.sessionId)}`): kartu berisi nama peserta (`w.participant.name`, fallback `w.participant.sessionId`), project (`w.participant.projectId`), branch (`w.branch`, ditambah "dari `w.baseBranch`" bila tidak kosong), path dalam `<code>`. Lalu:
         - bila `w.error`: peringatan (warna danger) "Tidak bisa diperiksa: " + `w.error` (dan jangan klaim bersih).
         - selain itu bila `uncommitted.length === 0 && unmergedCommits.length === 0`: "Bersih: tidak ada yang akan hilang." (warna `var(--ok, #30a46c)`).
         - selain itu dua daftar bertajuk "Perubahan belum di-commit (N)" (tiap baris: `status` monospace lalu `path`) dan "Commit belum di-merge (N)" (tiap baris: `sha` monospace lalu `subject`). Daftar kosong dari salah satu daftar tidak dirender.
       - Semua teks panjang `overflow-wrap: anywhere`, `code`/mono `font-family: ui-monospace, monospace; font-size: 11px`.
     - `{#if closeError}<div class="err" role="alert">{closeError}</div>{/if}`.
     - Tombol: "Batal" (`onclick={cancel}`, `disabled={busy}`) dan "Lanjut tutup" (`class="danger-btn"`, `onclick={confirmClose}`, `disabled={busy || status !== 'ready'}`, label `{busy ? 'Menutup...' : 'Lanjut tutup'}`). Gaya `danger-btn`: `background: var(--danger, #e5484d); color: #fff; border-color: transparent` (warna `#fff` konsisten dengan `button.primary` di `NewThreadDialog`).
   - Gaya dasar overlay/dlg/button disalin dari `NewThreadDialog.svelte` (lebar `min(560px, 92vw)`, `max-height: 90vh`, `flex-direction: column`).
   - Dialog tidak memasang polling/timer. Setelah `closeThread` sukses, UI thread berubah menjadi closed dari event `thread.updated` yang sudah disubscribe store; dialog cukup memanggil `closeCloseDialog()`.

8. **`ClosedNotice.svelte`** (props `{ thread }`)
   - Render hanya bila `thread.state === 'closed'`.
   - Teks: `Thread ditutup pada {when}` dengan `when = formatClosedAt(thread.closedAt)`; fungsi lokal: bila `closedAt` kosong atau `new Date(closedAt)` tidak valid kembalikan `''` (teks menjadi "Thread ditutup"); selain itu `d.toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })`. (`formatTime` dari `../format` hanya jam, tidak cocok untuk tanggal, jadi tidak dipakai.)
   - `role="status"`; gaya: `margin: 6px 12px`, `padding: 6px 10px`, `font-size: 12px`, `color: var(--fg-muted, #888)`, `border: 1px solid var(--border, #8884)`, `border-radius: 6px`.

9. **`index.ts`** (timpa stub Foundation sepenuhnya)
   - Impor `registerThreadSlot` dari `'../slots'`, tiga komponen (`CloseButton`, `CloseBanner`, `ClosedNotice`).
   - `let stopper: (() => void) | null = null;`
   - `export function installClose(): () => void`: bila `stopper` ada, kembalikan; selain itu `registerThreadSlot({ slot: 'header-actions', component: CloseButton, order: 100, id: 'close-button' })`, `registerThreadSlot({ slot: 'above-input', component: CloseBanner, order: -10, id: 'close-banner' })`, `registerThreadSlot({ slot: 'above-input', component: ClosedNotice, order: 0, id: 'close-notice' })`; `const stop = () => { if (stopper === stop) stopper = null; }; stopper = stop; return stop;` (pola `installParticipants`; registrasi slot tidak punya API unregister, jadi pembersih hanya mereset flag).
   - Re-ekspor: `export { default as ReadyBadge } from './ReadyBadge.svelte'; export { isThreadReadOnly } from './readOnly';`.
   - Tidak mengekspor hal lain yang dipakai fitur lain (aturan paralel overview 7); `closeApi`/`closeDialogStore` hanya diimpor internal.

10. **Guard**: jalankan `cd frontend && npm run check && npm run build`; perbaiki semua galat/warning type check di file milik fitur ini (termasuk peringatan a11y Svelte: gunakan `<!-- svelte-ignore a11y_no_static_element_interactions -->` pada overlay seperti `NewThreadDialog`). Kegagalan akibat file fitur lain hanya dilaporkan. Pastikan `git status` hanya menunjukkan perubahan di `frontend/src/modules/thread/close/` dan tidak ada perubahan `package.json`.

## Perubahan

Dibuat di `frontend/src/modules/thread/close/`:

- `closeApi.ts`
- `readOnly.ts`
- `closeDialogStore.ts`
- `ReadyBadge.svelte`
- `CloseBanner.svelte`
- `CloseButton.svelte`
- `CloseDialog.svelte`
- `ClosedNotice.svelte`

Diubah (menimpa stub Foundation): `frontend/src/modules/thread/close/index.ts`.

Tidak ada perubahan di luar folder itu.

## Wiring

Yang diekspos untuk langkah wiring (JANGAN disambungkan oleh fitur ini):

- `installClose()` dari `frontend/src/modules/thread/close/index.ts`: wiring menambah `import { installClose } from './close';` dan `installClose();` tepat setelah `installParticipants();` di `frontend/src/modules/thread/index.ts`.
- `ReadyBadge` (diekspor dari `./close`): wiring menambah `import { ReadyBadge } from './close';` di `ThreadSection.svelte` dan menyisipkan `<ReadyBadge thread={t} />` di dalam `<button class="row">` setelah `.title`, sebelum `{#if t.state === 'closed'}`.
- `isThreadReadOnly` diekspor untuk pemakaian opsional; `ThreadTab.svelte` sudah menonaktifkan `ThreadInput` saat closed, jadi tidak ada perubahan input.
- Setelah `installClose()` dipanggil: slot `header-actions` (`CloseButton`, host `CloseDialog`) dan `above-input` (`CloseBanner`, `ClosedNotice`) aktif otomatis lewat `ThreadSlot`.
- Ketergantungan runtime: `thread.closePreview`/`thread.close` baru berfungsi setelah `thread-close-daemon` terpasang lewat `RegisterClose(m)`; sebelum itu dialog menampilkan galat load dengan "Coba lagi" (perilaku yang diharapkan).
- Catatan untuk wiring: `ThreadHeader.svelte` sudah punya badge "siap ditutup" sendiri; biarkan.

## Selesai bila

- Semua berkas di bagian Perubahan ada dengan signature persis bagian Kontrak; stub `index.ts` Foundation sudah ditimpa dan `installClose()` idempoten (panggilan ganda tidak menggandakan slot karena `id` tetap).
- Banner hanya tampil saat `readyToClose && state === 'active'` dan menampilkan `closeSummary` serta tombol Close; `CloseButton` hanya menampilkan tombol saat `active`, namun selalu me-mount `CloseDialog`.
- `CloseDialog` memanggil `fetchClosePreview` sekali per pembukaan (guard generasi), menangani loading, error (dengan "Coba lagi"), worktree bersih, entri `error`, dan menampilkan peringatan merah penghapusan permanen; "Batal" tidak memanggil `thread.close`; "Lanjut tutup" memanggil `closeThread` sekali, tidak bisa ditekan ganda, menutup dialog saat sukses, dan pada galat menampilkan `errorText` tanpa menutup dialog.
- Tidak ada `setInterval`/`setTimeout`/polling; tidak ada warna hard-code di luar fallback `var(...)` (kecuali `#fff` pada teks tombol utama mengikuti pola `NewThreadDialog`).
- Guard: `cd frontend && npm run check && npm run build` sukses; tidak ada perubahan di luar `frontend/src/modules/thread/close/` dan tidak ada dependency baru. Tanpa unit test.
