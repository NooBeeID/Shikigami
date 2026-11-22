# Plan: Kotak komentar di DiffView

## Metadata

- Tiket: `agent-diff-review-ticket-04-review-comment` (`docs/tickets/agent-diff-review-ticket-04-review-comment.md`)
- Fitur: `review-comment-ui` (Kotak komentar di DiffView)
- ownedPaths:
  - `frontend/src/modules/review/CommentBox.svelte` (baru)
  - `frontend/src/modules/review/DiffView.svelte` (hanya penyisipan komentar)
- Depends on foundation: tidak (`hasFoundation=false`)
- Kontrak bersama: lihat `docs/plans/agent-diff-review-ticket-04-review-comment/overview.md` bagian 2.2 (IPC `review.comment`) dan 2.5 (frontend). Jangan mengubah overview.

## Tujuan

User mengklik nomor baris di `DiffView`, kotak komentar (`CommentBox`) muncul tepat di bawah baris itu, user menulis komentar lalu menekan "Kirim ke agent" (atau Ctrl/Cmd+Enter). Komentar dikirim lewat IPC `review.comment` dan ditempel ke input prompt sesi agent tanpa Enter. Sesi `running` ditolak dengan `[BUSY]`; kotak tetap terbuka dengan teks utuh. Fitur tiket 02 dan 03 (render hunk, `hunkActions`, `onLineClick`, navigasi `j/k/a/r`) tidak boleh berubah perilakunya.

## Path yang dimiliki

HANYA boleh menyentuh:
- `frontend/src/modules/review/CommentBox.svelte` (baru)
- `frontend/src/modules/review/DiffView.svelte` (penyisipan komentar saja)

Dilarang menyentuh (termasuk file wiring): `frontend/src/modules/review/{ReviewTab.svelte,index.ts,store.ts,types.ts,ReviewBadge.svelte,diff.ts,HunkActions.svelte,keys.ts}`, seluruh kode Go (`internal/**`, `cmd/**`), `go.mod`, `frontend/package.json`, dan file docs lain. Pemakaian prop `sessionId` di `ReviewTab.svelte` dikerjakan agen Wiring, bukan di sini. Jangan menambah dependency, jangan menulis unit test, jangan `git commit`.

## Kontrak

Dipakai:
- `call<T>(method, params)` dari `frontend/src/lib/ipc.ts` (impor relatif dari modul review: `'../../lib/ipc'`). Error dilempar membawa pesan server; ambil dengan `e instanceof Error ? e.message : String(e)`. Tag `[BUSY]` bisa didahului prefiks transport, jadi deteksi dengan `includes('[BUSY]')`.
- IPC `review.comment` (dibuat agen `review-comment-daemon`, belum tentu siap): params `{sessionId: string, path: string, line: number, text: string}`, hasil `{ok: boolean, sent: string, enter: boolean}`. Error `[BUSY] agent sedang bekerja, tunggu selesai lalu kirim lagi` untuk sesi running; error lain berupa pesan apa adanya.
- Tipe dari `./diff`: `DiffLine {kind, text, oldNo, newNo}`, `Hunk {id, ... lines}`, `DiffResult {hunks, currentHash, binary, tooLarge}`.
- Token tema: `--fg`, `--fg-muted`, `--border`, `--accent`, `--ok`, `--warn`, `--danger`, `--bg-hover` (selalu dengan fallback seperti gaya `DiffView` yang ada).

Disediakan:
- `CommentBox.svelte`, props (Svelte 5 runes):
  ```ts
  let { sessionId, path, line, onclose, onsent, sending = $bindable(false) }: {
    sessionId: string; path: string; line: number;
    onclose: () => void;
    onsent?: (sent: string) => void;
    sending?: boolean;           // opsional, $bindable, true selama request berjalan
  } = $props();
  ```
- `DiffView.svelte`: prop opsional baru `sessionId?: string`. Tanpa `sessionId`, DiffView berperilaku persis seperti sekarang.

## Langkah

### 1. Baca dulu
Baca `DiffView.svelte`, `keys.ts` (hanya baca) dan satu komponen kecil (`HunkActions.svelte`) untuk mengikuti gaya. Struktur DiffView saat ini: props `{diff, path, onLineClick, hunkActions, footer, selectedHunkId}`; `{#each h.lines as ln}` di dalam `{#each diff.hunks as h, i}`; dua cabang render baris (`{#if onLineClick}` dengan `div.line.clickable role="button"`, `{:else}` `div.line` polos), masing-masing berisi `span.no` (oldNo), `span.no` (newNo), `span.mk`, `span.tx`; seluruhnya di `.diff` (scroll `overflow:auto`).

### 2. Buat `CommentBox.svelte`
Isi:
- State: `let text = $state('')`, `let error = $state<string | null>(null)`, `let done = $state(false)`, `let ta: HTMLTextAreaElement | undefined = $state()`. `sending` dari prop bindable.
- Autofokus: `$effect(() => { ta?.focus(); })` yang hanya berjalan sekali saat mount (jangan bergantung pada `text`). Aman: `onMount(() => ta?.focus())` dari `svelte`.
- `const canSend = $derived(text.trim().length > 0 && !sending && !done)`.
- Timer konfirmasi: simpan `let timer: ReturnType<typeof setTimeout> | undefined`; `onDestroy(() => clearTimeout(timer))`.
- `async function send()`:
  ```ts
  if (!canSend) return;
  sending = true; error = null;
  try {
    const res = await call<{ ok: boolean; sent: string; enter: boolean }>('review.comment', { sessionId, path, line, text });
    onsent?.(res.sent);
    done = true;
    timer = setTimeout(() => onclose(), 2000);
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    error = msg.includes('[BUSY]') ? 'Agent sedang bekerja, tunggu selesai lalu kirim lagi' : msg;
  } finally {
    sending = false;
  }
  ```
  Kirim `text` apa adanya (tanpa trim; daemon yang menormalisasi). Teks dipertahankan saat gagal.
- `onkeydown` pada textarea: SELALU `e.stopPropagation()` lebih dulu (supaya tidak mencapai `keys.ts` atau shortcut global). Lalu: `Escape` -> `e.preventDefault(); onclose()` (abaikan bila `sending`? tidak, tetap tutup kecuali `done` sudah menjadwalkan; cukup `clearTimeout(timer); onclose()`); `(e.ctrlKey || e.metaKey) && e.key === 'Enter'` -> `e.preventDefault(); void send()`. Enter polos = baris baru (default textarea).
- Hentikan juga propagasi `keydown` dan `click` di elemen akar kotak (`<div class="cbox" onkeydown={(e) => e.stopPropagation()} onclick={(e) => e.stopPropagation()}>`, beri `<!-- svelte-ignore a11y_no_static_element_interactions -->`/`a11y_click_events_have_key_events` bila svelte-check mengeluh) agar tombol Batal/Kirim juga tidak memicu handler di atasnya.
- Markup:
  - Label `path:line` (`<div class="lbl">{path}:{line}</div>`, monospace, `--fg-muted`).
  - `<textarea bind:this={ta} bind:value={text} maxlength="2000" rows="3" placeholder="Komentar untuk agent..." disabled={sending || done} aria-label="Komentar untuk {path}:{line}" onkeydown={onKeydown}>`.
  - Baris tombol: "Kirim ke agent" (`disabled={!canSend}`, label "Mengirim..." saat `sending`), "Batal" (`onclick={() => { clearTimeout(timer); onclose(); }}`, nonaktif saat `sending`), dan hitung karakter opsional `{text.length}/2000` rata kanan.
  - Area status `<div class="msg" role="status" aria-live="polite">`: bila `done` -> teks "Ditempel ke input agent, tekan Enter di sesi" (kelas `ok`); bila `error` -> teks error (kelas `err`, `--danger`; untuk `[BUSY]` boleh `--warn`).
- Gaya (scoped `<style>`): `.cbox` dengan `border: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent))`, `background: var(--bg, canvas)`, padding 8px, `border-radius: 4px`, font non-monospace sistem ukuran 12-13px, `white-space: normal`; textarea lebar penuh, `resize: vertical`, `background: transparent`, `color: var(--fg, inherit)`, border token, `:focus-visible` outline `--accent`; tombol utama berlatar `--accent`, tombol sekunder berborder; `:disabled` opacity 0.5. Semua warna lewat token + fallback agar benar di tema gelap/terang. Tanpa warna hard-coded selain fallback.

### 3. Sisipkan ke `DiffView.svelte` (aditif)
a. Import: `import CommentBox from './CommentBox.svelte';`.
b. Props: tambahkan `sessionId` ke destructuring dan `sessionId?: string;` ke tipe.
c. State (di bawah `status`):
   ```ts
   let commentTarget = $state<{ hunkId: string; lineIndex: number; line: number } | null>(null);
   let commentSending = $state(false);
   function openComment(e: MouseEvent, h: Hunk, li: number, ln: DiffLine) {
     if (!sessionId) return;
     e.stopPropagation();           // jangan memicu onLineClick/baris clickable
     const line = ln.newNo || ln.oldNo;
     if (!line || line < 1) return;
     commentTarget = { hunkId: h.id, lineIndex: li, line };   // klik baris lain memindahkan kotak
   }
   ```
   Catatan: bila baris yang sama diklik lagi saat `commentSending` true, jangan memindahkan (`if (commentSending) return;` di awal agar kotak yang sedang mengirim tidak hilang).
d. Tutup saat diff dimuat ulang kecuali sedang mengirim:
   ```ts
   $effect(() => {
     diff;                       // dependensi: identitas diff (dan dengan itu currentHash)
     if (!commentSending) commentTarget = null;   // baca commentSending TANPA menjadikannya dependensi
   });
   ```
   Pastikan `commentSending` dibaca di dalam `untrack(() => commentSending)` (impor `untrack` dari `svelte`) supaya efek hanya berjalan saat `diff` berubah, bukan saat flag berubah. Bila pengiriman selesai sukses, `CommentBox` menutup dirinya lewat `onclose`; bila diff dimuat ulang selagi mengirim, kotak dibiarkan terbuka sampai `onclose` dipanggil. Tutup juga bila `sessionId` menjadi kosong (`{#if sessionId && commentTarget ...}` pada render cukup).
e. Ubah `{#each h.lines as ln}` menjadi `{#each h.lines as ln, li}` (satu-satunya perubahan struktur each).
f. Pada KEDUA cabang baris (clickable dan polos), beri kedua `span.no` atribut pemicu, tanpa mengubah elemen/urutan lain:
   ```svelte
   <span class="no" class:cm={!!sessionId} title={sessionId ? 'Komentar ke agent' : undefined}
         onclick={sessionId ? (e) => openComment(e, h, li, ln) : undefined}>{ln.oldNo || ''}</span>
   ```
   (sama untuk span newNo). Tambahkan `<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->` di atas bila perlu. Bila `sessionId` kosong, atribut bernilai `undefined` sehingga DOM dan perilaku identik dengan sekarang. Aksesibilitas keyboard: baris `clickable` tetap memakai Enter/Space seperti semula (tidak diubah); pemicu komentar v1 cukup lewat klik.
g. Render kotak tepat setelah elemen baris target, di dalam `{#each h.lines as ln, li}` yang sama (setelah penutup `{#if onLineClick}...{/if}`):
   ```svelte
   {#if sessionId && commentTarget && commentTarget.hunkId === h.id && commentTarget.lineIndex === li}
     <div class="comment-slot">
       <CommentBox {sessionId} {path} line={commentTarget.line} bind:sending={commentSending}
                   onclose={() => { commentTarget = null; }} />
     </div>
   {/if}
   ```
   Key target adalah `(hunkId, lineIndex)`; `line` diambil dari `commentTarget.line` yang dihitung saat klik.
   Setelah `onclose`, `commentSending` kembali false (komponen unmount selagi `sending=false` dari `finally`); untuk keamanan, di `onclose` set juga `commentSending = false`.
h. CSS tambahan (append di akhir `<style>`, jangan ubah aturan lama):
   ```css
   .no.cm { cursor: pointer; }
   .no.cm:hover { color: var(--accent, currentColor); text-decoration: underline; }
   .comment-slot { position: sticky; left: 0; width: min(100%, 640px); box-sizing: border-box; padding: 4px 8px 6px 64px; white-space: normal; }
   ```
   Sesuaikan `padding-left` supaya kotak sejajar dengan area teks dan tetap terlihat saat `.diff` bergulir horizontal (`.line` memakai `min-width: max-content`, jadi slot perlu `position: sticky; left: 0` dan lebar terbatas, bukan `max-content`). Pastikan `.comment-slot` tidak ikut `white-space: pre`.
i. Perbarui komentar header di atas file (tambah satu baris: `sessionId: bila ada, klik nomor baris membuka CommentBox di bawah baris`).

### 4. Tiruan sementara untuk `review.comment` (opsional, hanya bila perlu pengujian lokal)
Bila daemon belum siap, panggilan IPC gagal dan UI menampilkan galat; itu wajar. Bila ingin menguji alur sukses secara lokal, boleh mengganti sementara `call` dengan tiruan di `CommentBox.svelte`, tetapi HARUS dihapus dan impor `call` dari `../../lib/ipc` dipulihkan sebelum selesai. Pastikan tidak ada sisa tiruan (`grep -n "mock\|TODO\|fake" CommentBox.svelte`).

### 5. Verifikasi
- `cd frontend && npm run build` harus sukses. Opsional `npm run check`; jangan memperbaiki galat di luar ownedPaths. Bila `check` mengeluh pada file yang dimiliki (a11y, tipe), perbaiki di file itu.
- Tinjau manual diff `DiffView.svelte`: seluruh perubahan bersifat aditif (import, prop, state, efek, `, li`, atribut pada `span.no`, blok `{#if ...CommentBox}`, CSS tambahan). Tanpa `sessionId` DOM tidak berubah.

## Perubahan

- Baru: `frontend/src/modules/review/CommentBox.svelte`
- Diubah: `frontend/src/modules/review/DiffView.svelte` (prop `sessionId?`, state `commentTarget`/`commentSending`, `openComment`, efek tutup-saat-reload, `, li` pada each, atribut pemicu pada `span.no`, render `CommentBox` dalam `div.comment-slot`, CSS tambahan, komentar header)

## Wiring

Yang diekspos untuk langkah wiring (JANGAN disambungkan di sini):
- Prop opsional `sessionId?: string` pada `DiffView`. Agen Wiring menambah satu prop `sessionId={sessionId ?? undefined}` pada `<DiffView ... />` di `frontend/src/modules/review/ReviewTab.svelte` (titik pemakaian, sekitar baris 275). Tanpa edit itu, fitur komentar tidak aktif dan DiffView berperilaku seperti sebelumnya.
- `CommentBox` memanggil `review.comment` dengan `{sessionId, path, line, text}`; daemon didaftarkan oleh fitur `review-comment-daemon` di `wire_review.go`.

## Selesai bila

- [ ] `CommentBox.svelte` ada dengan props sesuai kontrak (`sessionId`, `path`, `line`, `onclose`, `onsent?`, plus `sending` bindable opsional); textarea autofokus, `maxlength=2000`, "Kirim ke agent" nonaktif bila teks kosong (trim) atau sedang mengirim, "Batal", Ctrl/Cmd+Enter mengirim, Esc menutup, semua `keydown` di textarea `stopPropagation`.
- [ ] Sukses: `onsent(sent)`, pesan "Ditempel ke input agent, tekan Enter di sesi" sekitar 2 detik lalu `onclose()`. `[BUSY]`: pesan "Agent sedang bekerja, tunggu selesai lalu kirim lagi", teks dipertahankan, kotak terbuka. Error lain: pesan apa adanya.
- [ ] `DiffView` menerima `sessionId?`; klik nomor baris (bila `sessionId` ada) membuka satu `CommentBox` tepat di bawah baris itu dengan `line = newNo || oldNo`; klik baris lain memindahkan; nomor baris `cursor: pointer` + `title="Komentar ke agent"`; klik nomor tidak memicu `onLineClick`.
- [ ] Kotak tertutup saat `diff` dimuat ulang kecuali sedang mengirim.
- [ ] Tanpa `sessionId`, DiffView identik dengan perilaku sebelumnya; `hunkActions`, `footer`, `onLineClick`, `selectedHunkId` tidak berubah.
- [ ] Tampilan benar di tema gelap/terang (hanya token tema + fallback), tanpa dependency baru.
- [ ] Tidak ada file di luar ownedPaths yang berubah; tidak ada sisa tiruan.
- [ ] Guard: `cd frontend && npm run build` sukses. Tanpa unit test.
