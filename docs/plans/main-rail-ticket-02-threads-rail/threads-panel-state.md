# Plan: State panel Threads ke store

## Metadata

- Tiket: `docs/tickets/main-rail-ticket-02-threads-rail.md` (fitur `threads-panel-state`)
- Overview: `docs/plans/main-rail-ticket-02-threads-rail/overview.md` (kontrak bersama di bagian 2, aturan paralel di bagian 7)
- Fitur: `threads-panel-state` (tahap A, paralel dengan `agents-sidebar-cleanup`)
- ownedPaths:
  - `frontend/src/modules/thread/ThreadSection.svelte`
  - `frontend/src/modules/thread/thread-store.ts`
- Depends on foundation: tidak (epic ini tidak punya foundation)

## Tujuan

Panel Threads akan menjadi item rail yang di-unmount saat item rail lain aktif. Agar posisi scroll daftar thread bertahan, state panel dipindah ke store `threadPanelState`. `ThreadSection.svelte` disesuaikan menjadi panel penuh tanpa props: pembungkus scroll sendiri, tanpa `border-bottom` section, memulihkan `scrollTop` saat mount dan menyimpannya saat scroll/unmount. Perilaku daftar, tombol "Thread baru", `ReadyBadge`, tag "ditutup", dan pembukaan pane tidak berubah. Kode harus tetap dapat dikompilasi dengan wiring lama (`thread/index.ts` belum diubah dan masih memberi `onerror`; Svelte menerima props ekstra tanpa galat build).

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `frontend/src/modules/thread/ThreadSection.svelte`
- `frontend/src/modules/thread/thread-store.ts`

Jangan menyentuh `thread/index.ts`, `rail-icon.ts`, `ThreadRailIcon.svelte` (fitur `threads-rail-item`), `sidebar-left/*` (fitur `agents-sidebar-cleanup`), `shell/*`, `layout/*`, `thread/participants/*`, `thread-client.ts`, kode Go, maupun file lain. Butuh perubahan di luar ownedPaths: laporkan sebagai catatan, jangan edit.

## Kontrak

Disediakan (kontrak tetap, lihat overview bagian 2.2):

```ts
// thread-store.ts
export interface ThreadPanelState { scrollTop: number }
export const threadPanelState: Writable<ThreadPanelState>; // default { scrollTop: 0 }
```

- `ThreadSection.svelte`: props `{}` (tanpa `onerror`); galat tetap lewat store `threadError`.
- Teks UI tidak berubah: "Thread", "Thread baru", "Belum ada thread", tag "ditutup".

Dipakai (sudah ada, tidak diubah): `threads`, `threadError`, `openThreadPane` dari `thread-store.ts`; `activeWorkspaceId` dari `../../lib/stores`; `statusStore` dari `../status/store`; `ReadyBadge` dari `./close`; `NewThreadDialog.svelte`.

Aturan: `ThreadSection.svelte` tidak boleh memanggil `subscribeThreadEvents` atau `listen` event apa pun. Hanya `ThreadSection.svelte` yang membaca `threadPanelState`.

## Langkah

1. `thread-store.ts`: pada baris impor 2, `writable` dan `type Readable` sudah diimpor; tambahkan `type Writable` ke impor `svelte/store` bila belum ada (`import { derived, get, writable, type Readable, type Writable } from 'svelte/store';`). Tepat setelah `export const threadError = writable('');` tambahkan:
   ```ts
   // State panel Threads yang harus bertahan saat panel di-unmount (item rail lain aktif).
   export interface ThreadPanelState { scrollTop: number }
   export const threadPanelState: Writable<ThreadPanelState> = writable<ThreadPanelState>({ scrollTop: 0 });
   ```
   Jangan menambah field lain (filter/lipatan) dan jangan mengubah kode lain di file ini.

2. `ThreadSection.svelte`, script:
   - Impor tambahan: `import { onMount } from 'svelte';` dan `threadPanelState` ke impor `./thread-store` (`import { openThreadPane, threadError, threadPanelState, threads } from './thread-store';`). Hapus penggunaan `onerror`.
   - Ganti `let {}: { onerror: (msg: string) => void } = $props();` dengan tidak ada deklarasi props sama sekali (komponen tanpa props; hapus baris itu).
   - Tambah `let scroller: HTMLDivElement | undefined = $state();` (atau `$state<HTMLDivElement>()` sesuai gaya file lain), dan:
     ```ts
     function saveScroll(): void {
       if (scroller) threadPanelState.set({ scrollTop: scroller.scrollTop });
     }

     onMount(() => {
       // Pulihkan setelah daftar dirender; elemen scroller sudah ada saat onMount.
       if (scroller) scroller.scrollTop = $threadPanelState.scrollTop;
       return saveScroll; // simpan saat unmount
     });
     ```
     Catatan: pada unmount, elemen mungkin sudah lepas dari DOM sehingga `scrollTop` terbaca 0. Karena itu simpan juga di handler `onscroll` (langkah 3) sehingga store selalu mutakhir, dan buat `saveScroll` tidak menimpa nilai dengan 0 akibat elemen terlepas: di cleanup, jangan menyimpan ulang; cukup andalkan `onscroll`. Implementasi disarankan: cleanup `onMount` dihilangkan atau hanya memanggil `saveScroll` bila `scroller?.isConnected`. Kriteria tiket "simpan saat scroll/unmount" terpenuhi lewat `onscroll` ditambah cleanup yang aman.
     Bila daftar thread belum terisi saat mount (thread dimuat async) dan `scrollTop` tersimpan melebihi tinggi konten, browser akan menjepit nilainya; tambahkan `import { tick } from 'svelte';` dan `await tick()` sebelum menetapkan `scrollTop` bila perlu (onMount boleh memakai fungsi async internal; jangan jadikan callback `onMount` itu sendiri async bila ingin mengembalikan fungsi cleanup).

3. `ThreadSection.svelte`, markup. Struktur baru (isi list/tombol/badge/tag/dialog identik dengan sebelumnya):
   ```svelte
   <section>
     <div class="head"> ... tidak berubah ... </div>
     {#if $threadError}<div class="err">{$threadError}</div>{/if}
     <div class="scroll" bind:this={scroller} onscroll={saveScroll}>
       <ul> ... {#each} ... {:else}<li class="empty">Belum ada thread</li>{/each} </ul>
     </div>
     <NewThreadDialog open={dialogOpen} onclose={() => (dialogOpen = false)} />
   </section>
   ```
   `head` dan `err` tetap di luar `.scroll` (header tetap terlihat saat daftar di-scroll). `dialogOpen` tetap `$state` lokal (transien).

4. `ThreadSection.svelte`, style:
   - `section`: ganti dengan `section { display: flex; flex-direction: column; height: 100%; min-height: 0; padding: 8px 12px; box-sizing: border-box; }` (hapus `border-bottom`).
   - Tambah `.scroll { flex: 1; min-height: 0; overflow-y: auto; }`.
   - `ul`: pertahankan; boleh `margin: 4px 0 0`.
   - Aturan lain (`.head`, `.label`, `.new`, `.row`, `.dot`, `.title`, `.tag`, `.empty`, `.err`) tidak diubah. `.head` dan `.err` tidak menyusut: tambahkan `flex: none` bila perlu.

5. Verifikasi bahwa file tidak memuat `subscribeThreadEvents`, `listen`, atau `onerror` (cek dengan `grep`).

6. Jalankan guard (lihat "Selesai bila"). Bila `svelte-check` mengeluh soal `onerror` ekstra dari `thread/index.ts` yang masih lama, itu di luar ownedPaths: catat, jangan edit (tahap wiring akan menyelesaikannya; di Svelte 5 props ekstra pada komponen yang tidak mendeklarasikan props tidak menggagalkan `vite build`).

## Perubahan

- Diubah: `frontend/src/modules/thread/thread-store.ts` (tambah `ThreadPanelState`, `threadPanelState`, impor `Writable`).
- Diubah: `frontend/src/modules/thread/ThreadSection.svelte` (tanpa props, pembungkus `.scroll`, pulihkan/simpan `scrollTop`, hapus `border-bottom`, layout kolom penuh tinggi).
- Tidak ada file baru.

## Wiring

Yang diekspos untuk langkah wiring (`threads-rail-item`, tahap B), tanpa disambungkan oleh fitur ini:

- `thread-store.ts` mengekspor `threadPanelState` dan tipe `ThreadPanelState`.
- `ThreadSection.svelte` kini berprops `{}` dan siap didaftarkan sebagai `component` item rail `threads` (`registerRailItem({ id: 'threads', title: 'Thread', icon: ThreadRailIcon, component: ThreadSection, order: 30 })`) di `thread/index.ts`.

Jangan mengubah `thread/index.ts`, mendaftarkan item rail, atau membuat ikon.

## Selesai bila

- [ ] `ThreadSection.svelte` tidak mendeklarasikan props dan tidak memanggil `onerror`.
- [ ] `thread-store.ts` mengekspor `threadPanelState` (default `{ scrollTop: 0 }`) dan `ThreadPanelState`.
- [ ] `ThreadSection.svelte` memulihkan `scrollTop` saat mount dan menyimpannya saat scroll (dan unmount secara aman); pembungkus `.scroll` memakai `overflow-y: auto; min-height: 0` dan mengisi tinggi panel; `border-bottom` section dihapus.
- [ ] Tidak ada `subscribeThreadEvents`/`listen` di `ThreadSection.svelte`.
- [ ] Daftar, tombol "Thread baru", `ReadyBadge`, tag "ditutup", dan klik baris membuka pane berperilaku sama.
- [ ] Hanya dua ownedPaths yang berubah (`git status` memastikan).
- [ ] Guard: `cd frontend && npm run build` sukses (tambahan bila ada: `npm run check`); `go build ./...` dari root tidak patah. Tanpa unit test.
