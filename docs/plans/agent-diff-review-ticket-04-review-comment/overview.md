# Overview: agent-diff-review-ticket-04-review-comment

Tiket: `docs/tickets/agent-diff-review-ticket-04-review-comment.md`. Spec: `docs/superpowers/specs/2026-11-22-agent-diff-review-design.md`. Bergantung pada tiket 01 sampai 03 (`review-capture`, `review-diff`, `review-actions`), semuanya sudah ter-merge.

Setiap agent fitur hanya membaca dokumen ini dan tiketnya. Bagian 2 adalah kontrak. Bila kode nyata berbeda dari uraian, ikuti kode yang ada dan catat penyimpangan di laporan/pesan commit.

## 1. Ringkasan dan urutan eksekusi

Epic menutup siklus review: user mengklik nomor baris di DiffView, menulis komentar, dan komentar ditempel ke input prompt sesi agent lewat satu `Manager.Write` ke PTY (format `di <path>:<line>: <text>`, tanpa Enter). Sesi `running` ditolak dengan `[BUSY]`.

Urutan: **tanpa Foundation** (`hasFoundation=false`, tidak ada `00-foundation.md`). Alasan: tidak ada paket baru atau tipe bersama yang harus ada lebih dulu; paket `internal/review` dan modul frontend `review` sudah ada, dan kedua fitur hanya bergantung pada kontrak tertulis di bagian 2.

1. Fitur paralel: `review-comment-daemon` (Go) dan `review-comment-ui` (frontend). UI memakai tiruan lokal untuk `call('review.comment')` sampai daemon siap, lalu menghapus tiruan itu.
2. Wiring (satu agen, setelah kedua fitur selesai): satu edit aditif satu baris di `ReviewTab.svelte` + verifikasi (bagian 5).

## 2. Kontrak bersama

### 2.1 Fakta kode yang ada (dibaca dari kode)

Go (modul `github.com/NooBeeID/shikigami`):
- `internal/review`: `types.go`, `store.go` (`Store`; tidak ada struct `Service`), `capture.go`, `ipc.go`, `diff.go`, `diff_ipc.go` (pola registrasi: `RegisterDiffIPC(srv *ipc.Server, store *Store, supportedFn func(string) bool)` memakai `srv.Register(method, func(ctx context.Context, params json.RawMessage) (any, error))`, `json.Unmarshal` params, `ipc.NewError(ipc.ErrInvalidParams, ...)` untuk params rusak, `ipc.NewError(ipc.ErrInvalid, ...)` untuk validasi), `actions.go`, `actions_ipc.go`.
- `internal/ipc/errors.go`: `ErrConflict=-32001`, `ErrNotFound=-32002`, `ErrInvalid=-32003`, `ErrInvalidParams=-32602`; `ipc.NewError(code, msg)`, `ipc.Errorf(code, fmt, ...)`.
- `internal/fsx.ResolveUnder(root, rel string) (string, error)` memvalidasi path di dalam root termasuk symlink dan mengembalikan path absolut. Hanya dipakai untuk validasi; hasilnya tidak dibaca isinya.
- `internal/session`: `(*Manager).Get(id) (*Session, error)` (`Session.Cwd`), `(*Manager).Write(id string, data []byte) error` (mengembalikan `session.ErrNotRunning` bila sesi sudah berakhir, `session.ErrNotFound` bila tak dikenal). Disediakan sebagai `d.Provide("session.manager", m)` di `wire_session.go`; di-lookup dengan `d.Lookup("session.manager")` bertipe `*session.Manager` (pola: `cmd/shikigamid/wire_mention.go`).
- `internal/status`: `status.Default() *Engine`, `(*Engine).Get(id) (SessionState, bool)` dengan `SessionState.Status` bertipe `status.Status` (`StatusRunning|StatusWaiting|StatusDone|StatusIdle`). `status.IsIdle(id)` true untuk idle|done|waiting, false untuk running dan sesi tak dikenal.
- `cmd/shikigamid/wire_review.go`: di `d.OnReady`, setelah lookup `status.engine` (variabel `eng`), memanggil `review.RegisterIPC`, `review.RegisterDiffIPC`, `review.RegisterActions` lalu memasang hook listener. `review.RegisterActions(...)` adalah baris registrasi terakhir sebelum `log := d.Logger`.

Frontend (Svelte 5 runes, TS strict):
- `DiffView.svelte` props sekarang: `{diff, path, onLineClick?, hunkActions?, footer?, selectedHunkId?}`. Render baris: bila `onLineClick` ada, baris dirender `<div class="line {kind} clickable" role="button" tabindex="0" onclick=... onkeydown=...>` berisi `<span class="no">{oldNo}</span><span class="no">{newNo}</span><span class="mk">…</span><span class="tx">…</span>`; bila tidak, `<div class="line ...">` tanpa handler. Tiap hunk: `.hunk-head` lalu `{#each h.lines as ln}` (tanpa index), seluruhnya di dalam `.diff` (scroll). Tidak ada slot "di bawah baris" yang nyata, dan `DiffView` BELUM menerima `sessionId`. `ReviewTab` memanggil `<DiffView diff={diffData} path={selectedPath} selectedHunkId={selectedHunk?.id} {hunkActions} />` tanpa `onLineClick`.
- `ReviewTab.svelte`: `sessionId` bertipe `string | null` (state), `selectedPath`; `.diffpane` dipasang `attachReviewKeys` (`keys.ts` sudah mengabaikan event dari `INPUT`/`TEXTAREA`/`SELECT`/contenteditable dan event dengan modifier, jadi `j/k/a/r` tidak bentrok di textarea).
- `lib/ipc.ts`: `call<T>(method, params)`; error yang dilempar membawa pesan server (cek `e instanceof Error ? e.message : String(e)`; tag `[BUSY]` ada di awal pesan, bisa didahului prefiks transport, jadi gunakan `includes('[BUSY]')` bila perlu).
- `diff.ts`: `DiffLine {kind: 'ctx'|'add'|'del', text, oldNo, newNo}`, `Hunk {id, oldStart, oldLines, newStart, newLines, lines}`, `DiffResult {hunks, currentHash, binary, tooLarge}`.

### 2.2 IPC `review.comment` (milik `review-comment-daemon`)

```
review.comment {sessionId: string, path: string, line: number, text: string}
  -> {ok: true, sent: string, enter: false}
```
- `path` relatif cwd sesi; `line` >= 1 (UI mengirim `newNo` bila ada, kalau tidak `oldNo`); `text` apa adanya dari user.
- `sent` adalah teks persis yang ditulis ke PTY.
- Urutan di handler: params rusak -> `ipc.ErrInvalidParams`; validasi (di-trim): `sessionId` kosong, `path` kosong, `line` < 1, `text` kosong setelah trim, `text` > 2000 karakter (hitung rune) -> `ipc.ErrInvalid`; ambil sesi (cwd) -> tak ada -> `ipc.ErrNotFound`; `fsx.ResolveUnder(cwd, path)` gagal -> `ipc.ErrInvalid`; cek state -> `running` -> `ipc.ErrConflict` dengan pesan persis `[BUSY] agent sedang bekerja, tunggu selesai lalu kirim lagi`; format; satu `Write`; galat tulis karena sesi berakhir -> `ipc.ErrNotFound`.
- Aturan state: `running` ditolak; `idle`, `waiting`, `done` ditempel tanpa Enter; status tidak dikenal engine tetapi sesi masih ada di manager -> diperlakukan tidak running (tetap ditempel).
- Tidak ada pembacaan isi file proyek, tidak ada I/O jaringan, tidak mengubah state review, tidak memublikasikan event.

Format pesan (satu baris, tanpa newline, tanpa `\r`, tanpa Enter): `di <path>:<line>: <text>`
- `text`: trim; rangkaian `\r`/`\n` diganti satu spasi; karakter kontrol lain (< 0x20 dan 0x7f, termasuk ESC) dibuang; trim lagi; bila kosong setelahnya -> error `ErrInvalid`.
- `path`: path relatif tervalidasi (yang sudah di-trim) dengan karakter kontrol dibuang.

### 2.3 API Go (milik `review-comment-daemon`, berkas `internal/review/comment.go`)

Karena paket belum punya struct `Service`, pakai fungsi registrasi (bukan method):

```go
const IPCMethodComment = "review.comment"
const commentMaxRunes = 2000

// ErrSessionGone dikembalikan adaptor Write/Cwd bila sesi tidak ada atau sudah berakhir.
var ErrSessionGone = errors.New("sesi tidak ada atau sudah berakhir")

// StateReader melaporkan apakah sesi sedang running; known=false bila engine tidak mengenal sesi.
type StateReader func(sessionID string) (running bool, known bool)

type CommentDeps struct {
    Cwd   func(sessionID string) (cwd string, err error) // err != nil (apa pun) = sesi tak dikenal
    Write func(sessionID string, data []byte) error      // err yang membungkus ErrSessionGone = sesi berakhir
    State StateReader
}

type CommentParams struct { SessionID string `json:"sessionId"`; Path string `json:"path"`; Line int `json:"line"`; Text string `json:"text"` }
type CommentResult struct { OK bool `json:"ok"`; Sent string `json:"sent"`; Enter bool `json:"enter"` }

func FormatComment(path string, line int, text string) (string, error)
func Comment(ctx context.Context, deps CommentDeps, p CommentParams) (CommentResult, error) // validasi + state + tulis
func RegisterCommentIPC(srv *ipc.Server, deps CommentDeps)
```
Helper privat berawalan `comment` (mis. `commentSanitize`). Paket `review` tidak boleh mengimpor `session`, `status`, atau `cmd`; adaptor ke tipe nyata dibuat di `wire_review.go` (bagian 2.4).

### 2.4 Adaptor di `wire_review.go` (milik `review-comment-daemon`)

Tambahkan satu blok di dalam `d.OnReady` modul `review`, tepat setelah `review.RegisterActions(...)` (tanpa mengubah baris lain):
- `v, ok := d.Lookup("session.manager")`; `mgr, ok2 := v.(*session.Manager)`; bila tidak ada, `log warning` ("review: session.manager tidak tersedia, review.comment dilewati") lalu lewati blok (jangan `return` dari callback; registrasi lain dan hook listener di bawahnya tetap harus jalan).
- `Cwd`: `mgr.Get(id)` -> `s.Cwd`.
- `Write`: `mgr.Write`; bila `errors.Is(err, session.ErrNotRunning) || errors.Is(err, session.ErrNotFound)` kembalikan `fmt.Errorf("%w", review.ErrSessionGone)`.
- `State`: `eng.Get(id)` -> `(st.Status == status.StatusRunning, true)`; `(false, false)` bila tidak dikenal.
- Panggil `review.RegisterCommentIPC(srv, review.CommentDeps{...})`. Tambah import `session` dan `fmt`/`errors` bila belum ada; boleh membuat helper privat berawalan `reviewWire...` di berkas yang sama.

### 2.5 Frontend (milik `review-comment-ui`)

`CommentBox.svelte`:
```ts
// $props
{ sessionId: string; path: string; line: number; onclose: () => void; onsent?: (sent: string) => void }
```
- `call<{ok: boolean; sent: string; enter: boolean}>('review.comment', {sessionId, path, line, text})` dari `../../lib/ipc`.
- Teks UI Indonesia: sukses "Ditempel ke input agent, tekan Enter di sesi" (~2 detik, lalu `onclose()`); `[BUSY]` -> "Agent sedang bekerja, tunggu selesai lalu kirim lagi" (teks dipertahankan, kotak terbuka); error lain: pesan apa adanya.
- Textarea autofokus; `Ctrl/Cmd+Enter` = kirim; `Esc` = tutup (`onclose`); semua `keydown` di textarea memanggil `e.stopPropagation()` agar tidak mencapai `keys.ts` atau shortcut global. Tombol "Kirim ke agent" nonaktif bila teks kosong (trim) atau sedang mengirim; "Batal". Teks komentar maksimal 2000 karakter (`maxlength`).
- Gaya: token tema yang ada (`--fg`, `--fg-muted`, `--border`, `--accent`, `--ok`, `--warn`, `--danger`, `--bg-hover`), benar di tema gelap/terang, tanpa dependency baru.

Sisipan di `DiffView.svelte` (aditif; perilaku lain tidak berubah):
- Prop baru opsional `sessionId?: string`. Fitur komentar aktif hanya bila `sessionId` terisi (bila kosong, DiffView berperilaku persis seperti sekarang).
- Karena `ReviewTab` tidak memberi `onLineClick`, DiffView menangani klik sendiri: bila `sessionId` ada, nomor baris (kedua `span.no`, atau cukup elemen nomor) menjadi pemicu komentar dengan `cursor: pointer` dan `title="Komentar ke agent"`. Baris tetap memanggil `onLineClick` bila diberikan (jangan memutus perilaku itu). Pemicu komentar tidak boleh bentrok dengan klik/Enter/Space di baris `clickable`; gunakan `stopPropagation` pada klik nomor bila baris `clickable`.
- State lokal `commentTarget = {hunkId: string, lineIndex: number, line: number} | null`; `line = ln.newNo || ln.oldNo` (>= 1). Hanya satu kotak terbuka; klik baris lain memindahkan. Karena `{#each h.lines as ln}` tanpa index, ubah menjadi `{#each h.lines as ln, li}` (satu-satunya perubahan pada struktur each).
- Render `<CommentBox>` tepat setelah elemen baris target di dalam `{#each}` yang sama (bukan slot baru), dibungkus `div.comment-slot` sticky-left (mis. `position: sticky; left: 0; width` secukupnya) supaya terlihat meski `.diff` bergulir horizontal. `onclose` -> `commentTarget = null`.
- Tutup kotak saat `diff` dimuat ulang (efek pada `diff.currentHash` atau identitas `diff`) kecuali pengiriman sedang berjalan. Karena `CommentBox` memegang state kirim, DiffView menerima flag lewat callback opsional internal: cara sederhana yang diizinkan adalah `CommentBox` mengekspor properti `sending` lewat `bind:sending` (prop `$bindable` opsional `sending?: boolean`, boleh ditambahkan ke props kontrak selama opsional); DiffView menunda penutupan bila `sending` true.
- `sessionId` hanya dari prop; jangan mengimpor store layout/sesi di DiffView.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `review-comment-daemon` | `FormatComment`, `Comment`, `RegisterCommentIPC`, tipe params/result/deps di `comment.go`; blok registrasi `review.comment` + adaptor `session.manager`/status engine di `wire_review.go` (perluasan kecil) | `internal/review/comment.go`, `cmd/shikigamid/wire_review.go` (hanya perluasan registrasi comment) |
| `review-comment-ui` | `CommentBox.svelte` dan penyisipan komentar di `DiffView.svelte` (prop `sessionId?`, state `commentTarget`, render kotak di bawah baris, tutup saat reload, petunjuk klik di nomor baris) | `frontend/src/modules/review/CommentBox.svelte`, `frontend/src/modules/review/DiffView.svelte` (hanya penyisipan komentar) |

Plan masing-masing: `docs/plans/agent-diff-review-ticket-04-review-comment/review-comment-daemon.md` dan `review-comment-ui.md`.

## 4. Foundation

Tidak ada (`hasFoundation=false`), sehingga tidak ada `00-foundation.md`.

## 5. Wiring checklist

Dikerjakan SETELAH kedua fitur selesai dan ter-merge. Hanya edit aditif minimal.

1. `frontend/src/modules/review/ReviewTab.svelte` (milik tiket 01 sampai 03; titik pemakaian `DiffView`, kira-kira baris 275): tambahkan satu prop `sessionId={sessionId ?? undefined}` pada `<DiffView ... />`. Ini satu-satunya edit pada file itu; tiket menyebut perubahan di titik pemakaian bila `DiffView` belum menerima `sessionId`. Catat di pesan commit.
2. Pastikan `wire_review.go` sudah memuat blok registrasi `review.comment` (dikerjakan fitur daemon) dan tidak mengubah registrasi, hook listener, atau event lain. Pastikan `go build ./...` lulus.
3. Verifikasi kontrak: `CommentBox` memanggil `review.comment` dengan `{sessionId, path, line, text}` persis; handler mengembalikan `{ok, sent, enter}`; pesan `[BUSY] ...` dikenali UI. Perbaiki ketidakcocokan kecil dan catat di commit.
4. Verifikasi manual bila aplikasi dapat dijalankan: sesi Claude mengedit file, buka tab Review, buka diff, klik nomor baris (kotak muncul di bawah baris, hanya satu kotak), kirim saat sesi idle/waiting (teks `di path:baris: teks` muncul di input terminal tanpa Enter, kotak menutup dengan konfirmasi), kirim saat running (ditolak dengan pesan, teks utuh), `Esc`/Batal menutup, mengetik `j/k/a/r` di textarea tidak memicu navigasi, `Ctrl/Cmd+Enter` mengirim, teks multi-baris/ESC dinormalisasi, kotak menutup saat diff dimuat ulang.
5. Pastikan fitur tiket 01 sampai 03 tetap jalan (daftar file, badge, diff, Revert/Terima, `j/k/a/r`) dan tampilan benar di tema gelap/terang.
6. Jalankan Build guard (bagian 6).

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` dari root repo.
- `cd frontend && npm run build` (Vite). Opsional: `npm run check` bila hijau di baseline; jangan memperbaiki galat di luar ownedPaths.

`review-comment-daemon` menjalankan `go build ./...`; `review-comment-ui` menjalankan `npm run build` di `frontend/`; Wiring menjalankan keduanya.

## 7. Aturan paralel

- Tiap agen hanya menyentuh `ownedPaths` fiturnya. Agen daemon tidak menyentuh frontend; agen UI tidak menyentuh Go.
- Dilarang menyentuh: `internal/review/{store,capture,ipc,types,diff,diff_ipc,actions,actions_ipc}.go`, `frontend/src/modules/review/{index.ts,ReviewTab.svelte,store.ts,types.ts,ReviewBadge.svelte,diff.ts,HunkActions.svelte,keys.ts}`, `internal/session/`, `internal/status/`, `internal/ipc/`, `go.mod`, `frontend/package.json`. Edit satu baris di `ReviewTab.svelte` hanya dilakukan agen Wiring.
- Di `wire_review.go` dan `DiffView.svelte` hanya penyisipan komentar; jangan mengubah registrasi, hook listener, event, struktur render hunk, logika diff, atau slot `hunkActions`.
- Tidak menambah dependency, tidak menulis unit test, tidak menjalankan `git commit` (orkestrator yang menangani).
- UI: gunakan tiruan lokal sementara untuk `call('review.comment')` bila daemon belum siap dan hapus sebelum selesai; panggilan IPC gagal sampai daemon ter-wiring itu wajar. Daemon: `RegisterCommentIPC` harus berkompilasi dan terdaftar lewat blok wiring miliknya sendiri.
- Bila kontrak terasa kurang, ikuti tiket dan kode yang ada, catat penyimpangan di laporan, jangan mengubah berkas fitur lain.
