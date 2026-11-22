# Plan: Laci detail tiket

## Metadata
- Tiket: `lark-connector-ticket-03-lark-tickets` (`docs/tickets/lark-connector-ticket-03-lark-tickets.md`)
- Fitur: `lark-ticket-drawer` (Laci detail tiket)
- ownedPaths: `frontend/src/modules/lark/board/TicketDrawer.svelte`
- Depends on foundation: ya (`00-foundation.md`: `boardClient.ts` dibuat Foundation; komponen ini hanya mengimpornya)
- Baca juga: `overview.md` bagian 2.4, 2.6, 6, 7 (kontrak bersama; tidak diulang di sini)

## Tujuan
Membuat komponen `TicketDrawer.svelte`: laci yang meluncur dari kanan DI DALAM area pane board (bukan modal global). Laci memanggil `getTicket` untuk `recordId`, menampilkan daftar `nama: nilai` (teks multi-baris dipertahankan), keadaan memuat dan galat dengan tombol coba lagi, tutup lewat tombol dan Esc, serta tombol Salin link dan Kirim ke agen tingkat tiket. Komponen belum disambungkan ke `BoardPane`; itu tugas langkah wiring.

## Path yang dimiliki
HANYA berkas ini yang boleh dibuat/diubah:
- `frontend/src/modules/lark/board/TicketDrawer.svelte`

Jangan menyentuh `BoardPane.svelte`, `Grid.svelte`, `Kanban.svelte`, `BoardHeader.svelte`, `boardClient.ts`, `actions.ts`, store, berkas Go, `cmd_lark_tickets.go`, `frontend/package.json`, atau file lain. Jangan mengubah `overview.md`.

## Kontrak
Dipakai (tidak boleh dibuat salinan lokal; impor persis):
- Dari `./boardClient` (Foundation, overview 2.4): `getTicket(p: { workspaceId; sourceId; recordId; tableId; viewId? }): Promise<TicketDetail>`, tipe `TicketSource`, `TicketDetail`, `LarkError`, dan `parseLarkError`. `getTicket` melempar `LarkError` (`{code, message}`); gunakan `parseLarkError(e)` lagi saat menangkap agar aman untuk nilai lain.
- Dari `../actions` (tiket 02): `copyLink(url: string): Promise<void>` dan `sendToAgent(url: string): Promise<void>`. Keduanya sudah menangani toast (termasuk "Tidak ada sesi aktif"); drawer tidak membuat toast sendiri kecuali actions ternyata melempar galat (lihat langkah 6).
- `workspaceId`: dari `activeWorkspaceId` (store di `frontend/src/lib/stores/workspaces`, periksa nama dan bentuk persis di kode, kemungkinan Svelte store sehingga dibaca dengan `$activeWorkspaceId`). Props tidak membawa workspaceId, jadi diambil dari store ini. Bila `activeWorkspaceId` kosong, tampilkan galat "Workspace aktif tidak ditemukan" dengan tombol coba lagi.
- Tipe `TicketDetail`: `{ fields: { name: string; text: string }[]; url?: string; recordLinkAvailable: boolean }`.

Disediakan (untuk wiring; lihat bagian Wiring):
```ts
// props Svelte 5 ($props())
{ source: TicketSource; tableId: string; viewId?: string; recordId: string; onClose: () => void }
```
Ekspor default komponen Svelte. Tanpa event lain, tanpa bindable.

Aturan tombol tingkat tiket:
- Aktif hanya bila `detail.recordLinkAvailable === true` DAN `detail.url` tidak kosong.
- Bila `recordLinkAvailable === false`: kedua tombol `disabled`, keterangan teks "Link tiket tidak bisa dipastikan" terlihat (bukan hanya tooltip). `detail.url` (mungkin link tabel) TIDAK PERNAH diteruskan ke `copyLink`/`sendToAgent` dan tidak ditampilkan sebagai link tiket.

## Langkah
Cek dulu bahwa `frontend/src/modules/lark/board/boardClient.ts` dan `frontend/src/modules/lark/actions.ts` ada (Foundation dan tiket 02). Bila tidak ada, hentikan dan laporkan; jangan membuatnya. Cek nama persis `activeWorkspaceId` dan lihat satu komponen Svelte 5 yang ada (mis. `frontend/src/modules/grimoire/GrimoirePanel.svelte`) untuk gaya runes dan token CSS.

1. **Buat `TicketDrawer.svelte`** (Svelte 5, `<script lang="ts">`). Impor `getTicket`, `parseLarkError`, tipe `TicketSource`, `TicketDetail`, `LarkError` dari `./boardClient`; `copyLink`, `sendToAgent` dari `../actions`; `activeWorkspaceId` dari `../../../lib/stores/workspaces` (sesuaikan jalur relatif); `get`/`$` sesuai gaya file lain.
2. **Props dan state**:
   - `let { source, tableId, viewId, recordId, onClose }: { source: TicketSource; tableId: string; viewId?: string; recordId: string; onClose: () => void } = $props();`
   - `let status = $state<'loading' | 'ready' | 'error'>('loading'); let detail = $state<TicketDetail | null>(null); let error = $state<LarkError | null>(null);`
   - `let seq = 0;` (bukan reaktif; penghitung stale guard). `let retryTick = $state(0);` untuk coba lagi.
3. **Pengambilan data** dengan `$effect`: baca `source.id`, `tableId`, `viewId`, `recordId`, `retryTick` (agar efek bergantung pada semuanya, termasuk `$activeWorkspaceId`). Isi efek:
   - `const mine = ++seq;` lalu SEGERA `detail = null; error = null; status = 'loading';` (jangan pernah menampilkan data tiket lama saat `recordId` berganti).
   - Bila `recordId` kosong atau workspace kosong: `status='error'`, `error` = `{code:'invalid_params', message: ...}` Indonesia, return.
   - `getTicket({ workspaceId, sourceId: source.id, recordId, tableId, viewId: viewId || undefined })` lalu `.then(d => { if (mine !== seq) return; detail = d; status = 'ready'; })` dan `.catch(e => { if (mine !== seq) return; error = parseLarkError(e); status = 'error'; })`.
   - Fungsi cleanup efek: `return () => { seq++; }` agar respons yang tiba setelah unmount atau setelah dependensi berganti diabaikan.
   - Jangan menulis ke state yang dibaca efek di dalam efek secara sinkron selain yang diperlukan di atas (hindari loop); bungkus pembacaan dependensi non-pemicu dengan `untrack` bila perlu.
4. **Tutup**:
   - Tombol tutup (ikon X, `aria-label="Tutup detail tiket"`) memanggil `onClose()`.
   - Esc: handler `onkeydown` pada elemen laci (`tabindex="-1"`, `role="dialog"`, `aria-modal="false"`, `aria-label="Detail tiket"`): `Escape` memanggil `onClose()` dan `stopPropagation()` (agar tidak menutup hal lain). Agar Esc bekerja tanpa fokus di dalam laci, tambahkan listener `keydown` di `window` lewat `<svelte:window onkeydown={...}>` yang hanya bereaksi pada `Escape` bila `!e.defaultPrevented`; satu mekanisme saja (jangan dobel panggil `onClose`). Pilih `<svelte:window>` dan hapus handler elemen bila redundan.
   - Saat mount, pindahkan fokus ke tombol tutup atau ke laci (`$effect` sekali + `bind:this`) supaya keyboard langsung berguna; saat unmount tidak perlu mengembalikan fokus (pemilik pane yang mengatur).
5. **Markup**:
   - Wadah `<aside class="drawer">` yang `position: absolute; top:0; right:0; bottom:0; width: min(420px, 100%);` (pemilik pane harus `position: relative`; catat di Wiring). Animasi masuk `transform: translateX(100%) -> 0` ~150ms; hormati `@media (prefers-reduced-motion: reduce)` (tanpa animasi).
   - Header: judul "Detail tiket" (`<h2>`), tombol tutup.
   - Badan (`overflow:auto`), berdasarkan `status`:
     - `loading`: teks "Memuat tiket..." dengan `role="status"` dan `aria-live="polite"`.
     - `error`: pesan `error.message` (fallback "Gagal memuat tiket"), `role="alert"`, plus tombol "Coba lagi" yang menaikkan `retryTick`.
     - `ready`: bila `detail.fields` kosong tampilkan "Tiket ini tidak punya isi untuk ditampilkan". Selain itu `<dl>` dengan pasangan `<dt>` (nama) dan `<dd>` (teks). `<dd>` memakai `white-space: pre-wrap; overflow-wrap: anywhere;` agar multi-baris dipertahankan dan nilai panjang dibungkus. Render teks dengan interpolasi biasa (`{f.text}`), TIDAK `{@html}`. Kunci `{#each}` memakai indeks plus nama (`(i)`), karena nama field bisa duplikat.
   - Footer aksi (tampil hanya saat `status === 'ready'`): tombol "Salin link" dan "Kirim ke agen". `const canLink = $derived(detail?.recordLinkAvailable === true && !!detail?.url);`. Kedua tombol `disabled={!canLink}`; bila `detail.recordLinkAvailable === false` tampilkan keterangan `<p class="hint" id="ticket-link-hint">Link tiket tidak bisa dipastikan</p>` dan beri tombol `aria-describedby="ticket-link-hint"`. Bila link tersedia, keterangan disembunyikan.
6. **Aksi**: `async function onCopy() { if (!canLink || !detail?.url) return; try { await copyLink(detail.url); } catch { /* actions sudah menampilkan toast; abaikan */ } }` dan `onSend` serupa dengan `sendToAgent`. Pegang `busy` kecil ($state) untuk mencegah klik ganda selama promise berjalan. Hanya `detail.url` milik tiket aktif (state `detail` sudah di-reset saat ganti `recordId`, sehingga link tiket lama tidak bisa terkirim).
7. **Gaya** (`<style>` lokal, hanya token tema yang ada; ikuti pola `GrimoirePanel.svelte`): `color: var(--fg)`, `background: var(--bg-elev, var(--bg))`, `border-left: 1px solid var(--border)`, teks sekunder `var(--fg-muted)` (nama field di `<dt>`), hover `var(--bg-hover)`, fokus `outline: 2px solid var(--accent)` untuk `:focus-visible` pada semua tombol, bayangan lembut via `color-mix(in srgb, black 25%, transparent)`. Tidak ada warna heksa hard-code kecuali sebagai fallback di `var(...)`. Pastikan terbaca di tema terang dan gelap. Tombol disabled: `opacity:.5; cursor:not-allowed`. Target klik minimal 28px. `z-index` secukupnya di dalam pane (mis. 5), tidak menutupi di luar pane.
8. **Aksesibilitas**: urutan Tab logis (tutup, lalu isi, lalu aksi); semua tombol adalah `<button type="button">`; tidak ada klik pada elemen non-interaktif; teks status memakai `aria-live`. Tidak ada penyimpanan ke disk atau `localStorage`.
9. **Verifikasi** (bagian Selesai bila).

## Perubahan
- BARU: `frontend/src/modules/lark/board/TicketDrawer.svelte`
- Tidak ada berkas lain yang dibuat atau diubah.

## Wiring
Fitur ini TIDAK menyambungkan apa pun. Yang diekspos untuk langkah `lark-tickets-wiring`:
- Komponen default `frontend/src/modules/lark/board/TicketDrawer.svelte` dengan props `{ source: TicketSource, tableId: string, viewId?: string, recordId: string, onClose: () => void }`.
- Penyambung (di `BoardPane.svelte`) merender: `{#if selectedRecordId}<TicketDrawer {source} {tableId} {viewId} recordId={selectedRecordId} {onClose} />{/if}`, dengan `onClose` mengosongkan `selectedRecordId`. Boleh memakai `{#key}` tidak wajib: komponen sudah mengambil ulang sendiri saat `recordId` berganti.
- Syarat tata letak untuk penyambung: elemen induk laci (area isi pane) harus `position: relative` dan `overflow: hidden` agar laci `position:absolute` menempel di dalam pane.
- Penyambung mengosongkan seleksi saat sumber/tabel/view berganti (laci sendiri tidak melakukannya).
- Komponen mengambil `workspaceId` sendiri dari `activeWorkspaceId`; penyambung tidak perlu meneruskannya.
- Tidak ada selisih kontrak yang diketahui. Bila ketika dikerjakan nama `activeWorkspaceId`, jalur `actions`, atau bentuk `TicketSource` ternyata berbeda dari kontrak, catat selisihnya di laporan akhir agar wiring menyelaraskan.

## Selesai bila
- [ ] `TicketDrawer.svelte` ada, berprops persis `{ source, tableId, viewId?, recordId, onClose }`, dan memanggil `getTicket` dari `./boardClient` tanpa salinan tipe/fungsi lokal.
- [ ] Menampilkan daftar `nama: nilai` dengan baris baru dipertahankan, keadaan memuat dan galat (dengan "Coba lagi").
- [ ] Tutup lewat tombol dan Esc masing-masing memanggil `onClose` tepat sekali.
- [ ] Ganti `recordId` mereset tampilan (tanpa data lama) dan mengambil ulang; respons usang (seq) dan respons setelah unmount diabaikan.
- [ ] Salin link dan Kirim ke agen memakai `detail.url` lewat `copyLink`/`sendToAgent`; nonaktif dengan keterangan "Link tiket tidak bisa dipastikan" bila `recordLinkAvailable=false`; link tabel tidak pernah dikirim sebagai link tiket.
- [ ] Gaya hanya memakai token tema (terang dan gelap), fokus terlihat, dapat dioperasikan dengan keyboard.
- [ ] Guard: `cd frontend && npm run build` sukses dan `npm run check` tanpa galat baru dari berkas ini. Tanpa unit test.
- [ ] `git status` hanya menampilkan `TicketDrawer.svelte` sebagai perubahan milik fitur ini (jangan stage perubahan tak terkait; jangan `pkill` luas).
