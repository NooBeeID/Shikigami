# Plan: Tab terminal minimal di UI

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-02-session-pty.md` (epic 02 Session PTY)
- Fitur: `terminal-tab-ui`
- ownedPaths: `frontend/src/modules/terminal/`; `frontend/src/App.svelte` (HANYA satu blok `{#each}` panel `main-tab`)
- Depends on foundation: ya (`00-foundation.md` memasang `@xterm/xterm` dan `@xterm/addon-fit`, dan membuat folder `frontend/src/modules/terminal/`)
- Rujukan: `overview.md` di folder ini (kontrak bersama, aturan paralel, build guard). Jangan diubah.

## Tujuan

Membuat modul frontend `terminal` berbasis xterm.js sehingga dari window Wails pengguna bisa menekan "Sesi baru", melihat prompt shell login, mengetik, mengubah ukuran window (PTY ikut berubah), menutup/membuka ulang window dan tetap melihat sesi `running` terbaru dengan scrollback-nya, dan melihat "[sesi berakhir]" saat sesi selesai. Hanya SATU tab/sesi yang tampil. Semua event-driven, tanpa `setInterval`/polling. Komponen dan helper dirancang agar ticket-06 dapat memakainya ulang tanpa mengubahnya.

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `frontend/src/modules/terminal/index.ts`
- `frontend/src/modules/terminal/TerminalTab.svelte`
- `frontend/src/modules/terminal/QuickTerminal.svelte`
- `frontend/src/modules/terminal/terminal-client.ts`
- `frontend/src/modules/terminal/.gitkeep` (boleh dihapus)
- `frontend/src/App.svelte`: hanya menambah satu blok `{#each}` untuk panel `main-tab`; tanpa logika lain, tanpa menghapus placeholder yang ada.

DILARANG: `frontend/src/lib/` (`registry.ts`, `ipc.ts`), `frontend/src/main.ts`, `app/`, semua Go, `frontend/package.json` dan lockfile (dependency sudah dipasang Foundation; cek dulu `grep xterm frontend/package.json`; hanya bila ternyata tidak ada, tambah seminimal mungkin), dan path fitur lain. Dilarang `npm install <paket>` baru, addon webgl/canvas, dan `setInterval`.

## Kontrak

Dipakai (lihat overview bagian 2; baca kode nyata `frontend/src/lib/` dahulu, bila nama berbeda ikuti kode nyata):
- `lib/registry.ts`: `registerPanel({id, area, component, order})`, `registerTabType({type, component})`, `panels(area)` (Svelte store `Readable<PanelSpec[]>`; `PanelSpec = {id, area, component, order?}`).
- `lib/ipc.ts`: `call<T>(method, params?) => Promise<T>`; `onEvent(topicPrefix, fn(topic, payload)) => unsubscribe` (event Wails `daemon:event`; prefiks cocok dengan `startsWith`; payload bertipe `unknown`). Catatan: `onEvent` tanpa Wails mengembalikan no-op, `call` di luar Wails menolak dengan Error.
- Method IPC (params/result persis tiket): `session.create` (params `{}` => `Session`), `session.list` (`{sessions: Session[]}`, urut `createdAt` naik), `session.kill` (`{id}`), `session.write` (`{id, data: base64}`), `session.resize` (`{id, cols, rows}`), `session.attach` (`{id}` => `{session, scrollback: base64, offset}`).
- Event: `session.output.<id>` payload `{data: base64, offset: number}` (offset = total kumulatif SESUDAH potongan); `session.exit` payload `{id, exitCode}`.
- Tipe `Session` (JSON camelCase): `{id, workspaceId, projectId, cwd, providerId, status: 'running'|'exited', createdAt, exitCode?}`.
- CSS variable: `--term-bg`, `--term-fg`, `--term-cursor` dengan fallback gelap.

Disediakan (diekspor `terminal-client.ts`, dipakai ulang ticket-06 sehingga signature DIKUNCI):
```ts
export interface SessionInfo { id: string; workspaceId: string; projectId: string; cwd: string; providerId: string; status: 'running' | 'exited'; createdAt: string; exitCode?: number }
export interface OutputEvent { data: string; offset: number }
export interface AttachResult { session: SessionInfo; scrollback: Uint8Array; offset: number }
export function bytesToBase64(b: Uint8Array): string
export function base64ToBytes(s: string): Uint8Array
export function stringToBase64(s: string): string          // UTF-8 encode lalu base64
export function listSessions(): Promise<SessionInfo[]>      // selalu array
export function createSession(): Promise<SessionInfo>       // session.create tanpa parameter
export function killSession(id: string): Promise<void>
export function writeSession(id: string, data: string): Promise<void>   // data teks mentah dari xterm, di-encode di sini
export function resizeSession(id: string, cols: number, rows: number): Promise<void>
export function subscribeOutput(id: string, fn: (ev: OutputEvent) => void): () => void
export function subscribeExit(id: string, fn: (exitCode: number) => void): () => void  // memfilter payload.id === id
export function attachSession(id: string): Promise<AttachResult>        // scrollback sudah di-decode
```
Komponen: `TerminalTab.svelte` prop tunggal `sessionId: string`. `QuickTerminal.svelte` tanpa prop. `index.ts` mendaftarkan `registerTabType({type:'session', component: TerminalTab})` dan `registerPanel({id:'terminal-quick', area:'main-tab', component: QuickTerminal, order: 50})`.

## Langkah

### 0. Persiapan
- Baca `frontend/src/lib/registry.ts`, `frontend/src/lib/ipc.ts`, `frontend/src/main.ts`, `frontend/src/App.svelte` (nyata), dan `frontend/package.json` (pastikan `@xterm/xterm` dan `@xterm/addon-fit` ada). Svelte 5 dengan runes: pakai `$props()`, `$state`, `$effect`/`onMount`.
- Bila `frontend/src/lib/` belum ada di repo kerja, hentikan dan laporkan (ticket-01 belum tergabung).

### 1. `terminal-client.ts`
Tipe dan fungsi persis bagian Kontrak. Detail:
- `bytesToBase64`: proses per potongan (mis. 0x8000 byte) dengan `String.fromCharCode.apply` lalu `btoa`, agar tidak overflow stack. `base64ToBytes`: `atob` lalu `Uint8Array` dari charCode; string kosong -> `Uint8Array(0)`. `stringToBase64`: `bytesToBase64(new TextEncoder().encode(s))`.
- `listSessions`: `const r = await call<{sessions: SessionInfo[] | null}>('session.list', {}); return r?.sessions ?? []`.
- `createSession`: `call<SessionInfo>('session.create', {})`. `killSession`: `call('session.kill', {id})`. `writeSession`: `call('session.write', {id, data: stringToBase64(data)})`. `resizeSession`: `call('session.resize', {id, cols, rows})`.
- `subscribeOutput(id, fn)`: `return onEvent('session.output.' + id, (_t, p) => { const ev = p as OutputEvent; if (ev && typeof ev.data === 'string' && typeof ev.offset === 'number') fn(ev) })`. Catatan: prefiks `session.output.<id>` bisa cocok dengan ID lain hanya bila ID saling prefiks; ULID 26 karakter tetap panjang sehingga aman.
- `subscribeExit(id, fn)`: `onEvent('session.exit', (_t, p) => { const e = p as {id?: string; exitCode?: number}; if (e && e.id === id) fn(e.exitCode ?? 0) })`.
- `attachSession(id)`: `const r = await call<{session: SessionInfo; scrollback: string; offset: number}>('session.attach', {id}); return {session: r.session, scrollback: base64ToBytes(r.scrollback ?? ''), offset: r.offset ?? 0}`.
- Tanpa dependency xterm di file ini (murni helper IPC/base64).

### 2. `TerminalTab.svelte`
`<script lang="ts">` Svelte 5. Impor `Terminal` dari `@xterm/xterm`, `FitAddon` dari `@xterm/addon-fit`, `'@xterm/xterm/css/xterm.css'`, dan helper dari `./terminal-client`. Prop: `let { sessionId }: { sessionId: string } = $props();`. Markup: `<div class="term-host" bind:this={host}></div>` ditambah (opsional) baris kecil status galat bila attach gagal.

Alur di `onMount` (kembalikan fungsi cleanup; bila memakai `$effect` yang bergantung `sessionId`, pastikan cleanup berjalan saat `sessionId` berubah; sederhana dan aman: pakai `$effect` dengan `sessionId` dibaca di awal sehingga tab dibuat ulang saat prop berganti, atau pakai `{#key sessionId}` di pemanggil; pilih salah satu dan konsisten, QuickTerminal memakai `{#key}`):
1. Baca warna: `const cs = getComputedStyle(host); const val = (n, fb) => cs.getPropertyValue(n).trim() || fb;` lalu `new Terminal({ theme: { background: val('--term-bg', '#1e1e1e'), foreground: val('--term-fg', '#d4d4d4'), cursor: val('--term-cursor', '#d4d4d4') }, cursorBlink: true, fontFamily: 'Menlo, Monaco, "Courier New", monospace', fontSize: 13, scrollback: 2000, allowProposedApi: false })`. Renderer bawaan (tanpa addon lain). Atur juga `host.style.background` ke nilai `--term-bg` agar tidak ada kilatan.
2. `const fit = new FitAddon(); term.loadAddon(fit); term.open(host); fit.fit();`
3. State lokal: `let ended = false; let attachOffset: number | null = null; const pending: OutputEvent[] = []; let disposed = false;`
4. SUBSCRIBE SEBELUM ATTACH (sinkron, sebelum `await` apa pun):
   - `unsubOut = subscribeOutput(sessionId, ev => { if (attachOffset === null) pending.push(ev); else if (ev.offset > attachOffset) term.write(base64ToBytes(ev.data)); })`.
   - `unsubExit = subscribeExit(sessionId, () => markEnded())`.
   - `markEnded()`: bila `ended` return; `ended = true`; `term.write('\r\n[sesi berakhir]\r\n')`; `term.options.disableStdin = true` (input mati).
5. `attachSession(sessionId)` (async IIFE, cek `disposed` setelah await): `term.write(res.scrollback)`; `attachOffset = res.offset`; alirkan `pending`: untuk tiap event berurutan, bila `ev.offset > attachOffset` tulis `base64ToBytes(ev.data)`; kosongkan `pending`. Bila `res.session.status === 'exited'` panggil `markEnded()` (sesi yang sudah berakhir saat attach; event exit tidak akan datang lagi). Catch galat: tampilkan pesan Indonesia di baris galat ("Gagal terhubung ke sesi: ...") dan `term.write` pesan serupa; jangan melempar.
   - Catatan korektness: event dengan `offset <= attachOffset` sudah tercakup scrollback dan WAJIB dibuang; jangan menulis scrollback setelah event live.
6. Input: `term.onData(d => { if (ended) return; writeSession(sessionId, d).catch(() => {}); })`. (xterm sudah menghasilkan string UTF-8 yang di-encode di `writeSession`.)
7. Resize: `const sendResize = () => resizeSession(sessionId, term.cols, term.rows).catch(() => {})`. `ResizeObserver` pada `host`: callback memanggil `fit.fit()` (dibungkus try/catch; host yang tersembunyi/0 ukuran bisa melempar) lalu menjadwalkan `sendResize` dengan debounce ~100 ms memakai SATU `setTimeout` yang di-`clearTimeout` ulang tiap panggilan (bukan `setInterval`). Panggil juga satu kali resize awal setelah attach selesai agar ukuran PTY sama dengan terminal.
8. Cleanup (destroy): `disposed = true`; `unsubOut(); unsubExit();` `resizeObserver.disconnect(); clearTimeout(timer); term.dispose();` TANPA memanggil `killSession`.
9. CSS lokal: `.term-host { width: 100%; height: 100%; min-height: 0; }`, `:global(.xterm)` padding kecil bila perlu. Komponen mengisi tinggi induknya (induk menyediakan tinggi).

### 3. `QuickTerminal.svelte`
`<script lang="ts">` Svelte 5. State: `let activeId = $state<string | null>(null); let busy = $state(false); let errorMessage = $state('');`.
- `onMount(async () => { try { const list = await listSessions(); const running = list.filter(s => s.status === 'running'); if (running.length) activeId = running[running.length - 1].id; /* list urut createdAt naik, jadi terakhir = terbaru */ } catch (e) { errorMessage = pesan } })`. Bila `list` kosong atau tak ada yang running, `activeId` tetap `null` (tampilkan teks "Belum ada sesi. Tekan Sesi baru.").
- `async function newSession()`: `busy = true`; `const s = await createSession(); activeId = s.id` (sesi lama tetap hidup di daemon, tidak di-kill); catch -> `errorMessage` (mis. "Gagal membuat sesi: ..." ; tampilkan pesan Error apa adanya); finally `busy = false`.
- `async function closeSession()`: bila `activeId` null return; `const id = activeId; await killSession(id)`; lalu `activeId = null`; catch -> `errorMessage`. (Sesi yang sudah exited: `session.kill` sukses dan menghapusnya dari daftar.)
- Markup: toolbar dengan dua tombol "Sesi baru" (disabled saat `busy`) dan "Tutup sesi" (disabled bila `activeId === null` atau `busy`); area terminal: `{#if activeId}{#key activeId}<TerminalTab sessionId={activeId} />{/key}{:else}<p class="empty">...</p>{/if}`; bila `errorMessage` tampilkan baris galat kecil. Teks UI bahasa Indonesia.
- Layout CSS: wadah `display:flex; flex-direction:column; height:100%; min-height:0`; toolbar tinggi tetap; area terminal `flex:1; min-height:0` dan beri `min-height: 320px` agar terlihat di App placeholder yang tidak memberi tinggi. Warna toolbar mengikuti tema gelap/terang sederhana (`prefers-color-scheme`), tanpa menambah store.
- Tanpa tab bar, daftar sesi, pemilih provider/project, atau store (itu milik ticket-06).

### 4. `index.ts`
```ts
import { registerPanel, registerTabType } from '../../lib/registry';
import TerminalTab from './TerminalTab.svelte';
import QuickTerminal from './QuickTerminal.svelte';

registerTabType({ type: 'session', component: TerminalTab });
registerPanel({ id: 'terminal-quick', area: 'main-tab', component: QuickTerminal, order: 50 });
```
Sesuaikan path impor dan bentuk argumen dengan `registry.ts` nyata. Tanpa ekspor lain, tanpa efek lain.

### 5. `frontend/src/App.svelte` (perubahan minimal)
Tambahkan SATU blok di markup (di bawah placeholder yang ada, tanpa menghapus apa pun):
```svelte
{#each $mainTab as panel (panel.id)}
  <section class="panel-main-tab"><panel.component /></section>
{/each}
```
- `$mainTab` sudah ada di placeholder (`const mainTab = panels('main-tab')`); bila tidak ada, tambah deklarasi itu saja. Render komponen dinamis dengan sintaks Svelte 5 (`<panel.component />`; atau `{@const Comp = panel.component}<Comp />` bila perlu).
- Tambah satu aturan CSS kecil untuk `.panel-main-tab` (mis. `height: 70vh; margin-top: 1rem;`) agar terminal punya tinggi. Tidak ada logika lain.

### 6. Verifikasi (guard)
- `cd frontend && npm run build` sukses tanpa galat TypeScript/Svelte (jalankan juga `npm run check` bila skripnya ada).
- `grep -rn "setInterval" frontend/src/modules/terminal/` tidak menemukan apa pun.
- `git status`/diff: hanya path di bagian "Path yang dimiliki" berubah; hapus file sementara.
- Uji manual penuh (window Wails + `aitermd`) bukan bagian guard; baru bisa setelah `session-ipc-ctl` tergabung.

## Perubahan

Dibuat:
- `frontend/src/modules/terminal/terminal-client.ts`
- `frontend/src/modules/terminal/TerminalTab.svelte`
- `frontend/src/modules/terminal/QuickTerminal.svelte`
- `frontend/src/modules/terminal/index.ts`

Diubah:
- `frontend/src/App.svelte` (satu blok `{#each}` panel `main-tab` + satu aturan CSS kecil)

Opsional dihapus: `frontend/src/modules/terminal/.gitkeep`.

## Wiring

Fitur ini tidak menyambungkan apa pun secara manual. Yang diekspos untuk langkah Wiring/verifikasi (overview bagian 5, butir 5):
- `modules/terminal/index.ts` dimuat otomatis oleh `import.meta.glob` di `main.ts` (ticket-01) dan memanggil `registerTabType({type:'session', ...})` serta `registerPanel({id:'terminal-quick', area:'main-tab', order: 50, ...})`.
- `App.svelte` merender panel `main-tab` lewat satu `{#each}`.
- Ticket-06 memakai ulang `TerminalTab.svelte` dan `terminal-client.ts` (signature di bagian Kontrak) dan menghapus panel `terminal-quick`.
- Tidak ada edit ke `lib/registry.ts`, `lib/ipc.ts`, `main.ts`, `app/`, `package.json`.

## Selesai bila

- [ ] `cd frontend && npm run build` sukses (guard fitur ini); `make build` hijau pada gabungan akhir (dicek agent Wiring).
- [ ] `terminal-client.ts` mengekspor semua fungsi/tipe bagian Kontrak dengan signature persis.
- [ ] `TerminalTab` memasang subscribe output dan exit SEBELUM `session.attach`, menahan event ke buffer, membuang event ber-`offset <= offset attach`, menulis scrollback dahulu lalu buffer.
- [ ] Input `onData` -> `session.write` base64; `ResizeObserver` + fit -> `session.resize` dengan debounce `setTimeout`.
- [ ] Saat `session.exit` (atau attach sesi berstatus exited) tampil "[sesi berakhir]" dan input mati; saat destroy: unsubscribe, observer putus, `term.dispose()`, TANPA `session.kill`.
- [ ] Warna dari `--term-bg`, `--term-fg`, `--term-cursor` dengan fallback gelap; hanya renderer bawaan xterm dan `@xterm/addon-fit`.
- [ ] `QuickTerminal` punya tombol "Sesi baru" dan "Tutup sesi", memanggil `session.list` saat mount dan meng-attach sesi `running` terbaru bila ada.
- [ ] Tipe tab `session` dan panel `terminal-quick` (area `main-tab`, order 50) terdaftar lewat `index.ts`; `App.svelte` hanya mendapat satu blok `{#each}` untuk `main-tab`.
- [ ] Tidak ada `setInterval`/polling; tidak ada file di luar path yang dimiliki yang berubah.
