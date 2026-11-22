# Plan: Panel peserta dan penyaringan sesi thread

## Metadata

- Tiket: `thread-ticket-02-thread-agents` (`docs/tickets/thread-ticket-02-thread-agents.md`)
- Fitur: `thread-participants-ui`
- ownedPaths: `frontend/src/modules/thread/participants/`
- Depends on foundation: ya. Memakai `registerSessionFilter` / `sessionFilter` di `frontend/src/modules/sidebar-left/registry.ts` (dibuat Foundation, `00-foundation.md` langkah 5) dan slot thread tiket 01 (sudah ada di repo). Bila `registerSessionFilter` belum diekspor dari `modules/sidebar-left/index.ts` saat Anda mulai, Foundation belum selesai: berhenti dan laporkan, jangan menambahnya sendiri.
- Rujukan kontrak: `docs/plans/thread-ticket-02-thread-agents/overview.md` bagian 2.2 no. 1 (penyaringan dua lapis) dan bagian 2.4 (kontrak frontend). Tidak diulang di sini.
- Tanpa dependency baru, tanpa unit test.

## Tujuan

Menyediakan folder `participants/` yang: (1) menampilkan panel peserta di kolom kanan tab thread (avatar berwarna, nama, role, project, status kerja/idle hidup, lencana "thread"); (2) membuka/memfokuskan terminal sesi peserta saat baris diklik; (3) memberi gaya sorot pada pesan sistem "Thread dijeda"; (4) menyaring sesi milik thread dari daftar sesi interaktif sidebar kiri. Semua dipasang lewat satu fungsi `installParticipants()`; pemanggilannya dilakukan langkah wiring, bukan fitur ini.

## Path yang dimiliki

HANYA boleh membuat/mengubah berkas di `frontend/src/modules/thread/participants/`. Dilarang menyentuh: berkas tiket 01 (`modules/thread/*.ts|svelte` di luar folder ini, termasuk `index.ts`, `slots.ts`, `thread-store.ts`, `MessageItem.svelte`, `ThreadTab.svelte`), `modules/sidebar-left/*` (Foundation), `lib/*`, `modules/status/*`, `modules/layout/*`, berkas Go, dan path fitur lain. Butuh perubahan di luar folder ini: jangan dikerjakan, catat di bagian Wiring.

## Kontrak

Dipakai (nama sudah ada di kode repo; periksa ulang sebelum mengimpor, bila beda ikuti kode):

- `modules/thread/slots.ts`: `registerThreadSlot({ slot, component, order?, id? })`; nama slot `'side-panel'` dan `'message-extra'`. Komponen slot menerima props `{ thread: Thread, message?: ThreadMessage }`. Impor dari dalam folder ini: `../slots`.
- `modules/thread/types.ts`: `Thread`, `Participant { sessionId, projectId, role: 'master'|'peer', name }`, `ThreadMessage`.
- `modules/thread/thread-client.ts`: `ownedSessions(): Promise<string[]>` (IPC `thread.ownedSessions`) dan `onThreadUpdated(fn: (t: Thread) => void): () => void` (event `thread.updated`).
- `modules/thread/thread-store.ts`: `threads` (writable `Thread[]`, sudah diperbarui oleh `subscribeThreadEvents` tiket 01 saat `thread.updated`).
- `modules/thread/format.ts`: `avatarColor(key)`, `initials(name)`.
- `modules/status/store.ts`: `statusOf(id): Readable<'running'|'waiting'|'done'|'idle'>`. Aturan tampilan: `running` = kerja, selain itu idle.
- `lib/stores/sessions.ts`: `sessions` (writable `Session[]`), `loadSessions()`. `lib/stores/types.ts`: `Session { id, workspaceId, projectId, cwd, providerId, ... }` (tidak ada `threadId`).
- `lib/stores/layout.ts`: `layout` dengan `openPane(leaf)`, `setFocus(paneId)`, `leaves` (Readable<Leaf[]>); `Leaf` punya `id`, `kind`, `sessionId`.
- Store project untuk nama project: cari di `lib/stores/workspaces.ts` / `lib/stores/index.ts` (store daftar project workspace aktif; `activeProject` sudah diimpor `sessions.ts` dari `./workspaces`). Baca file itu dan pakai store daftar project yang ada; bila tidak ada pencocokan id, tampilkan kosong (jangan menebak).
- `modules/sidebar-left/index.ts` (Foundation): `registerSessionFilter(spec: { id: string; predicate: Readable<(s: Session) => boolean> })`. `true` = TAMPIL.

Disediakan (diekspor dari `participants/index.ts`, signature final dari overview 2.4, jangan diubah):

```ts
export function installParticipants(): () => void
export function isThreadSession(s: { id: string; cwd?: string }): boolean
export const threadSessionIds: Readable<Set<string>>
export const threadSessionPredicate: Readable<(s: Session) => boolean>  // true = bukan sesi thread
export function openParticipantTerminal(p: Participant, thread: Thread): void
```

Pola cwd: `<DataDir>/threads/<ULID 26 huruf>/...` (master dan peer sama-sama di bawahnya). Hanya `/threads/<ULID>/` yang dicocokkan, tanpa DataDir (frontend tidak mengetahuinya).

## Langkah

Semua impor relatif dari `frontend/src/modules/thread/participants/`: tiket 01 di `../`, lib di `../../../lib/`, modul lain di `../../`.

1. **Baca dulu** (jangan diubah): `modules/thread/{slots.ts,types.ts,thread-client.ts,thread-store.ts,format.ts,ThreadSlot.svelte,MessageItem.svelte,ThreadTab.svelte}`, `modules/sidebar-left/{registry.ts,index.ts,SessionGroups.svelte}`, `modules/status/store.ts`, `lib/stores/{sessions.ts,layout.ts,groups.ts (baris layout.openPane terminal),types.ts,workspaces.ts}`, `modules/layout/model.ts` (`Leaf`, `NewLeaf`). Pastikan semua nama di bagian Kontrak cocok.

2. **`session-filter.ts`**:
   - `const THREAD_CWD = /[\\/]threads[\\/][0-9A-HJKMNP-TV-Z]{26}(?:[\\/]|$)/` (ULID Crockford huruf besar).
   - `const idsStore = writable<Set<string>>(new Set())`; `export const threadSessionIds: Readable<Set<string>> = { subscribe: idsStore.subscribe }`.
   - Helper tak-diekspor `addIds(ids: string[])`: bila ada id baru, `idsStore.update` dengan `Set` baru (selalu instance baru agar derived reaktif; tidak update bila tidak ada yang baru).
   - `export function isThreadSession(s: { id: string; cwd?: string }): boolean` = `get(idsStore).has(s.id) || (!!s.cwd && THREAD_CWD.test(s.cwd))`.
   - `export const threadSessionPredicate: Readable<(s: Session) => boolean> = derived(idsStore, ($ids) => (s) => !($ids.has(s.id) || (!!s.cwd && THREAD_CWD.test(s.cwd))))`.
   - `export function startSessionSync(): () => void` (idempoten, pola seperti `subscribeThreadEvents`: panggilan kedua mengembalikan pembersih yang sama): (a) berlangganan `onThreadUpdated` lebih dulu, handler memanggil `addIds(t.participants.map(p => p.sessionId).filter(Boolean))`; (b) lalu `ownedSessions().then(addIds)` dengan `.catch(() => {})` (galat IPC/di luar Wails tidak boleh melempar; pola cwd tetap menjaga); (c) seed juga dari `get(threads)` (`addIds` seluruh peserta). Pembersih memanggil unsub `onThreadUpdated` dan mereset flag. Set tidak pernah menyusut selama sesi aplikasi (sesi yang selesai tidak perlu muncul kembali di sidebar).

3. **`terminal.ts`** (helper kecil, bukan berkas ekspor publik) berisi `openParticipantTerminal(p: Participant, thread: Thread): void`, lalu diekspor ulang dari `index.ts`:
   - Cari pane terminal yang sudah ada: `get(layout.leaves).find(l => l.kind === 'terminal' && l.sessionId === p.sessionId)`. Ada -> `layout.setFocus(l.id)` lalu return.
   - Selain itu jalankan fungsi async internal `void (async () => {...})()` (fungsi publik tetap `void`, galat ditelan dengan `console.warn`): `let s = get(sessions).find(x => x.id === p.sessionId); if (!s) { await loadSessions(); s = get(sessions).find(...) }`. Bila tetap tidak ada -> `console.warn('sesi peserta tidak ditemukan', p.sessionId)` dan return (sesi sudah mati/dihapus; jangan membuka pane kosong).
   - Setelah `await`, cek ulang pane (balapan klik ganda) seperti langkah pertama.
   - Judul: nama provider dari store `providers` bila ada (lihat `groups.ts` untuk impornya; pola `get(providers).find(...)?.name ?? s.providerId`), diawali nama peserta bila tersedia: `p.name || providerName`.
   - `layout.openPane({ kind: 'terminal', sessionId: s.id, projectId: s.projectId, provider: s.providerId, workspaceId: thread.workspaceId, title })`. (`openPane` model juga mendedupe per `sessionId`; pengecekan di atas hanya agar fokus eksplisit.)

4. **`ParticipantRow.svelte`** (props `{ participant: Participant; thread: Thread; projectName: string }`, Svelte 5 runes):
   - Status hidup: karena `statusOf(id)` mengembalikan store baru per panggilan, berlangganan di `$effect` seperti `ThreadSlot.svelte`:
     ```ts
     let status = $state<string>('idle');
     $effect(() => statusOf(participant.sessionId).subscribe((v) => (status = v)));
     ```
     (fungsi unsubscribe dikembalikan sebagai cleanup; effect jalan ulang bila `participant.sessionId` berubah). `const working = $derived(status === 'running')`.
   - Tampilan: `<button type="button" class="row">` (elemen interaktif, `onclick={() => openParticipantTerminal(participant, thread)}`, `title="Buka terminal"`): avatar bulat `style:background={avatarColor(participant.name || participant.sessionId)}` berisi `initials(label)`; kolom teks: baris atas `label` (`participant.name`, fallback `master` untuk role master, selain itu `sessionId.slice(0, 8)`) + lencana kecil "thread"; baris bawah muted `role` dan `projectName` (dilewati bila kosong, dipisah " · "); di kanan titik status (`.dot.working` berwarna aksen, `.dot` abu) plus teks `kerja`/`idle` (juga `aria-label`).
   - Gaya: pakai variabel CSS yang sudah dipakai `MessageItem.svelte` (`--fg`, `--fg-muted`, `--bg-elev`) dengan fallback hex; lencana kecil font 10px, border tipis, radius 8px. Teks panjang `overflow: hidden; text-overflow: ellipsis`. Hover memberi latar `--bg-elev`.

5. **`ParticipantsPanel.svelte`** (props `{ thread: Thread; message?: ThreadMessage }` sesuai kontrak slot; `message` diabaikan):
   - `const rows = $derived(sortParticipants(thread.participants))`: master dulu, peer menyusul urut nama (fungsi lokal di berkas ini).
   - Nama project: berlangganan store daftar project dalam `$effect` ke `$state` (peta `projectId -> name`); `projectName = map[p.projectId] ?? ''`. Master (projectId kosong) -> kosong.
   - Markup: judul "Peserta" + jumlah, daftar `{#each rows as p (p.sessionId)}<ParticipantRow ... />{/each}`, keadaan kosong "Belum ada peserta". Bereaksi otomatis terhadap `thread.participants` (prop berasal dari `threadById` tiket 01 yang diperbarui `thread.updated`), jadi peserta baru muncul tanpa reload; JANGAN memanggil `getThread`/polling sendiri.
   - Tidak membuat kolom/`<aside>`: `ThreadTab` sudah membungkus slot `side-panel` dalam `<aside>`.

6. **`MessageExtra.svelte`** (props `{ thread: Thread; message?: ThreadMessage }`): render hanya bila `message?.kind === 'system'` dan `message.text.startsWith('Thread dijeda')`; selain itu tidak merender apa pun. Isi: blok callout kompak (latar kuning pudar, border kiri 3px kuning, teks kecil) berlabel "Dijeda" dengan petunjuk "Kirim pesan sebagai user atau master untuk melanjutkan." JANGAN mengulang teks pesan (baris sistem asli sudah dirender `MessageItem`). Warna lewat variabel dengan fallback; aman untuk tema gelap (`color-mix` atau rgba transparan, bukan warna solid terang).

7. **`index.ts`**:
   ```ts
   export function installParticipants(): () => void
   ```
   Idempoten (flag modul; panggilan kedua mengembalikan pembersih yang sama). Isi: `registerThreadSlot({ slot: 'side-panel', component: ParticipantsPanel, order: 0, id: 'participants' })`; `registerThreadSlot({ slot: 'message-extra', component: MessageExtra, id: 'participants-msg' })`; `registerSessionFilter({ id: 'thread-sessions', predicate: threadSessionPredicate })` (impor dari `../../sidebar-left`, BUKAN `../../sidebar-left/registry`); `const stopSync = startSessionSync()`; kembalikan fungsi yang memanggil `stopSync()` dan mereset flag (registrasi slot/filter bersifat id-idempoten dan dibiarkan). Ekspor ulang: `isThreadSession`, `threadSessionIds`, `threadSessionPredicate`, `openParticipantTerminal`. Jangan memanggil `installParticipants()` dari berkas ini.

8. **Guard**: `cd frontend && npm run check && npm run build`. Perbaiki semua galat/peringatan di berkas folder ini. Bila galat berasal dari berkas di luar folder (mis. Foundation belum selesai), laporkan alih-alih memperbaikinya. Pastikan `git status` hanya menunjukkan berkas baru di `frontend/src/modules/thread/participants/` (dan `package.json`/lockfile tidak berubah).

## Perubahan

Baru (semua di `frontend/src/modules/thread/participants/`):

- `session-filter.ts`
- `terminal.ts`
- `ParticipantRow.svelte`
- `ParticipantsPanel.svelte`
- `MessageExtra.svelte`
- `index.ts`

Tidak ada berkas lama yang diubah.

## Wiring

Disediakan untuk langkah wiring (JANGAN disambungkan oleh fitur ini):

- Satu-satunya titik pasang: `installParticipants()` dari `frontend/src/modules/thread/participants/index.ts`. Wiring menambahkan di `frontend/src/modules/thread/index.ts` (milik tiket 01): `import { installParticipants } from './participants'; installParticipants();` (overview bagian 5 no. 4). Letakkan setelah `subscribeThreadEvents()`.
- Prasyarat yang sudah dikerjakan Foundation (cukup diverifikasi): `registerSessionFilter`/`sessionFilter` di `modules/sidebar-left` dan penerapan di `SessionGroups.svelte`.
- Catatan untuk wiring/verifikasi manual: (a) sesi master/peer tidak tampil di sidebar kiri meski `thread.updated` terlambat, berkat pola cwd; (b) kolom kanan `ThreadTab` muncul otomatis begitu slot `side-panel` terdaftar (`hasThreadSlot`), yang berarti kolom itu baru tampil setelah `installParticipants()` dipanggil; (c) lencana "thread" hanya tampil di panel peserta karena sesi thread memang disembunyikan dari sidebar; (d) `session.list` tidak membawa `threadId`, jadi penyaringan murni bergantung pada `ownedSessions` + event + pola cwd (overview 2.2 no. 1); (e) bila nama simbol tiket 01 di kode berbeda dari yang tertulis di Kontrak, catat penyesuaian di sini.

## Selesai bila

- `cd frontend && npm run check && npm run build` sukses tanpa galat baru.
- Seluruh perubahan hanya di `frontend/src/modules/thread/participants/`; tanpa dependency baru; `index.ts` thread dan `modules/sidebar-left` tidak disentuh.
- `installParticipants()` mendaftarkan slot `side-panel` (id `participants`, order 0), slot `message-extra` (id `participants-msg`), dan filter sesi `thread-sessions`, serta memulai sinkron set sesi thread; idempoten dan mengembalikan fungsi berhenti.
- Panel menampilkan avatar berwarna, nama, role, project, titik status kerja/idle yang bereaksi terhadap `statusOf(sessionId)`, dan lencana "thread"; peserta baru muncul lewat pembaruan `thread.participants` tanpa reload.
- Klik baris memfokuskan pane terminal yang ada, atau membuka pane terminal baru (memuat `loadSessions()` bila sesi belum dikenal); sesi yang tidak ada tidak membuka pane kosong.
- `isThreadSession` benar untuk id di set maupun cwd berpola `/threads/<ULID>/`, salah untuk sesi biasa; `threadSessionPredicate` memancarkan fungsi baru saat set berubah.
- `MessageExtra` hanya menyorot pesan sistem berawalan "Thread dijeda" dan tidak merender apa pun untuk pesan lain.
