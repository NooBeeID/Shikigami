---
id: agent-diff-review-ticket-04-review-comment
title: Review: komentar ke agent
spec: docs/superpowers/specs/2026-11-22-agent-diff-review-design.md
status: ready
wave: 4
depends_on: [agent-diff-review-ticket-03-review-actions]
---
# Review: komentar ke agent

## Tujuan

Epic ini menutup siklus review: setelah user melihat diff perubahan agent, ia bisa menulis komentar pada satu baris diff dan mengirimkannya kembali ke sesi agent sebagai pesan ("di `path:baris`: teks"). Pengiriman memakai jalur `session.write` yang sudah ada (tulis ke PTY sesi), sehingga tidak ada kanal baru ke agent. Karena keadaan prompt agent tidak bisa dipastikan, perilaku default dibuat aman: pesan hanya ditempel ke input agent tanpa Enter, dan ditolak bila sesi sedang `running`.

## Hasil yang bisa dijalankan

Di atas tiket 01 sampai 03 (daftar file, DiffView, Revert/Terima):
- Di DiffView, klik nomor baris membuka kotak komentar di bawah baris itu (textarea + tombol "Kirim ke agent" + tombol "Batal").
- Menekan "Kirim ke agent" saat sesi `idle`/`waiting`: teks `di path:baris: teks` muncul di input prompt agent di terminal sesi, tanpa Enter. User menekan Enter sendiri di terminal. Kotak komentar menutup dan menampilkan konfirmasi singkat "Ditempel ke input agent, tekan Enter di sesi".
- Saat sesi `running`: pengiriman ditolak, kotak komentar tetap terbuka dengan teks utuh dan pesan "Agent sedang bekerja, tunggu selesai lalu kirim lagi".
- Build lulus dan aplikasi jalan; semua fitur tiket 01 sampai 03 tetap berfungsi.

## Batasan

In scope:
- Method IPC `review.comment`, penyusunan pesan, dan penulisan ke PTY lewat layanan session.
- Aturan state sesi (running ditolak; idle/waiting ditempel tanpa Enter).
- Komponen `CommentBox.svelte` dan penyisipannya ke `DiffView.svelte` (perubahan kecil, lihat Wiring).
- Komentar per baris. Komentar "per hunk" cukup dengan memakai nomor baris pertama hunk (user mengklik baris mana pun di hunk itu).

Out of scope:
- Mengirim Enter otomatis, antrean pesan saat sesi running, atau mode konfigurasi (v1 tidak ada pengaturan; bila nanti dibutuhkan jadi tiket lain).
- Menyimpan riwayat komentar atau menampilkan komentar yang sudah terkirim di diff.
- Membaca atau mengirim isi file ke luar mesin: pesan hanya memuat path relatif, nomor baris, dan teks yang diketik user. Isi baris diff tidak disertakan.
- Mengubah file milik epic lain di luar dua titik sentuh kecil yang disebut eksplisit (`wire_review.go`, `DiffView.svelte`).
- Dependency baru dan unit test.

## Kontrak

### Yang dipakai

- Dari tiket 01: modul `internal/review` (paket Go, `types.go`, `ipc.go` dengan fungsi registrasi IPC), `cmd/shikigamid/wire_review.go` (`RegisterModule("review", ...)`, `d.Lookup("status.engine")`), validasi path di dalam cwd sesi dengan aturan `internal/fsx`, store frontend `frontend/src/modules/review/store.ts` (sesi aktif).
- Dari tiket 02: `DiffView.svelte` dengan handler klik nomor baris dan slot/area sisipan di bawah baris (nama persis prop/slot: baca file itu sebelum mengedit). Tipe `DiffLine {kind, text, oldNo, newNo}` dan `Hunk` di `types.ts`.
- Dari tiket 03: tidak ada API baru yang dipakai selain keberadaan `HunkActions.svelte` dan `keys.ts`; komentar tidak boleh bentrok dengan shortcut `j/k/a/r` (saat fokus di textarea, shortcut itu dinonaktifkan; cek bagaimana `keys.ts` mengabaikan event dari elemen input dan, bila tidak, hentikan propagasi keydown di textarea `CommentBox`).
- Dari `internal/session`: `Manager.Get(id) (*Session, error)` dan `Manager.Write(id string, data []byte) error` (lihat `internal/session/manager.go`). Layanan didapat lewat `d.Lookup("session.manager")` seperti di `cmd/shikigamid/wire_mention.go`. Memanggil `Manager.Write` langsung setara dengan `session.write` (method IPC itu hanya membungkus `m.Write` dengan data base64) dan menghindari putaran IPC internal.
- Dari `internal/status`: `status.IsIdle(sessionID) bool` (true untuk idle|done|waiting, false untuk running dan sesi tak dikenal) dan `Engine.Get(sessionID)` (status eksplisit: `StatusRunning|StatusWaiting|StatusIdle|StatusDone`). Baca `internal/status/engine.go` untuk nama konstanta dan tipe persisnya.

### Yang disediakan

IPC `review.comment`:
- Params: `{sessionId: string, path: string, line: number, text: string}`. `path` relatif cwd sesi, `line` >= 1 (nomor baris di file kondisi sekarang, yaitu `newNo`; untuk baris `del` UI mengirim `oldNo` hanya bila `newNo` tidak ada, dan pesan tetap memakai angka itu).
- Result sukses: `{ok: true, sent: string, enter: false}` dengan `sent` berisi teks persis yang ditulis ke PTY (agar UI bisa menampilkannya).
- Error (kode dari `internal/ipc`):
  - `ERR_INVALID`: `sessionId` kosong, `path` kosong atau keluar dari cwd (aturan `internal/fsx`), `line` < 1, `text` kosong setelah trim, atau `text` lebih dari 2000 karakter.
  - `ERR_NOT_FOUND`: sesi tidak ada atau sudah berakhir.
  - `ERR_CONFLICT` dengan pesan diawali `[BUSY] `: sesi `running`. Pesan: `[BUSY] agent sedang bekerja, tunggu selesai lalu kirim lagi`.

Format pesan yang ditulis ke PTY (satu baris, tanpa newline dan tanpa Enter di akhir):
`di <path>:<line>: <text>`
- Normalisasi `text`: trim; ganti `\r` dan `\n` (juga rangkaian) dengan satu spasi; buang karakter kontrol lain (< 0x20 dan 0x7f, termasuk ESC) agar tidak menyuntik escape sequence atau perintah terminal. Tidak menambahkan `\r`.
- `path` yang dipakai di pesan adalah path relatif tervalidasi, dengan karakter kontrol dibuang juga.

Fungsi Go di `internal/review/comment.go` (nama internal, boleh disesuaikan selama konsisten):
- `type Writer interface { Write(id string, data []byte) error }` dan `type StateReader func(sessionID string) (running bool, known bool)` agar `comment.go` tidak mengimpor `cmd`.
- `func FormatComment(path string, line int, text string) (string, error)` menyusun dan menormalisasi pesan.
- `func (s *Service) Comment(ctx context.Context, p CommentParams) (CommentResult, error)` menerapkan validasi, aturan state, dan penulisan. `Service` dipakai bersama tiket lain di `internal/review`; bila `types.go`/`ipc.go` sudah punya struct layanan dari tiket 01, tambahkan method ini ke struct itu dari `comment.go` tanpa mengedit file tiket 01 (method boleh didefinisikan di file lain dalam paket yang sama). Bila hanya tersedia fungsi biasa, buat fungsi `RegisterCommentIPC(srv *ipc.Server, deps CommentDeps)` di `comment.go` dan panggil dari `wire_review.go`.

Frontend (`CommentBox.svelte`):
- Props: `sessionId: string`, `path: string`, `line: number`, `onclose: () => void`, `onsent?: (sent: string) => void`.
- Memanggil `call('review.comment', {sessionId, path, line, text})` dari `frontend/src/lib/ipc.ts`.

## Fitur

### review-comment-daemon: review.comment lewat session.write

Deskripsi detail:
- Buat `internal/review/comment.go` berisi `FormatComment`, tipe params/result, dan handler `review.comment` sesuai kontrak di atas.
- Urutan di handler: validasi params, validasi path di dalam cwd sesi (ambil cwd dari `session.Manager.Get`; pakai aturan `internal/fsx` seperti tiket 01, jangan menulis ulang aturan), cek state, tulis.
- Aturan state: ambil status dari status engine. `running` ditolak dengan `[BUSY]`. `idle`, `waiting`, dan `done` menempelkan pesan tanpa Enter. Keputusan ini adalah jawaban atas pertanyaan terbuka spec: default aman (tempel tanpa Enter, user menekan Enter sendiri). Status tidak diketahui oleh engine (sesi tanpa state) diperlakukan sebagai tidak running selama sesi masih ada di `session.Manager`, tetap ditempel tanpa Enter.
- Hanya satu `Manager.Write` dengan byte pesan (UTF-8). Jangan mengirim lebih dari satu write dan jangan membaca isi file di path itu. Tidak ada I/O jaringan.
- Tidak mengubah state review (baseline, ditinjau, dan sebagainya).
- Daftarkan IPC di `cmd/shikigamid/wire_review.go` lewat perluasan kecil: tambahkan satu blok di dalam `d.OnReady` yang meng-`Lookup("session.manager")` dan memanggil registrasi `review.comment`. Bila `session.manager` tidak tersedia, log warning dan lewati (modul review lain tetap jalan). Jangan mengubah registrasi, hook listener, atau event milik tiket lain di file itu.

Acceptance criteria:
- [ ] `review.comment` terdaftar dan bisa dipanggil lewat IPC (mis. dengan `shikigamictl` bila ada perintah generik, atau lewat UI).
- [ ] Sesi idle/waiting: teks `di path:baris: teks` muncul di input prompt sesi tanpa Enter; agent tidak mulai bekerja sampai user menekan Enter.
- [ ] Sesi running: error `[BUSY] ...` dan tidak ada byte yang ditulis ke PTY.
- [ ] `text` multi-baris atau mengandung ESC/kontrol dinormalisasi menjadi satu baris tanpa karakter kontrol.
- [ ] Path di luar cwd (`../x`, absolut di luar cwd, symlink keluar sesuai aturan fsx) ditolak `ERR_INVALID`; sesi tak dikenal `ERR_NOT_FOUND`.
- [ ] Tidak ada pembacaan isi file proyek dan tidak ada panggilan jaringan di `comment.go`.
- [ ] `go build ./...` lulus.

ownedPaths: `internal/review/comment.go`, `cmd/shikigamid/wire_review.go` (hanya perluasan kecil untuk registrasi comment).

Bisa dikerjakan paralel dengan `review-comment-ui`. Satu-satunya kesepakatan: nama method `review.comment`, params, dan bentuk error di bagian Kontrak. UI dapat dikembangkan terhadap kontrak itu dan baru diuji ujung ke ujung saat keduanya selesai.

### review-comment-ui: Kotak komentar di DiffView

Deskripsi detail:
- Buat `CommentBox.svelte` (Svelte, mengikuti gaya komponen modul review lain, termasuk token tema yang dipakai `DiffView.svelte`): textarea dengan fokus otomatis, label baris `path:baris`, tombol "Kirim ke agent" (nonaktif bila teks kosong atau sedang mengirim), tombol "Batal", dan area pesan status.
- `Ctrl/Cmd+Enter` di textarea sama dengan "Kirim ke agent"; `Esc` menutup. Event keydown di textarea tidak boleh memicu shortcut `j/k/a/r` milik `keys.ts` (hentikan propagasi bila perlu).
- Alur kirim: `await call('review.comment', ...)`. Sukses: panggil `onsent(result.sent)`, tampilkan "Ditempel ke input agent, tekan Enter di sesi" selama sekitar 2 detik, lalu `onclose()`. Gagal dengan pesan berawalan `[BUSY]`: tampilkan "Agent sedang bekerja, tunggu selesai lalu kirim lagi", pertahankan teks, kotak tetap terbuka. Error lain: tampilkan pesan error apa adanya, kotak tetap terbuka.
- Sisipkan ke `DiffView.svelte` (perubahan kecil dan eksplisit, file ini milik tiket 02): pada handler klik nomor baris yang disediakan tiket 02, simpan state `commentTarget = {hunkId, lineIndex, line}` lokal; render `<CommentBox>` tepat di bawah baris itu lewat slot/area sisipan yang sudah ada. Hanya satu kotak terbuka pada satu waktu. Nomor baris yang dipakai: `newNo` bila ada, kalau tidak `oldNo`. Jangan mengubah struktur render hunk, logika diff, atau slot aksi hunk (`HunkActions`) selain menambah penyisipan ini. Beri nomor baris petunjuk visual bisa diklik (kursor pointer, tooltip "Komentar ke agent").
- `CommentBox` membutuhkan `sessionId` dan `path`: ambil dari props yang sudah diterima `DiffView` (sesi aktif dan file terbuka). Bila `DiffView` belum menerima keduanya sebagai prop, tambahkan di titik pemakaian dengan perubahan sekecil mungkin dan catat di commit.
- Bila diff yang ditampilkan dimuat ulang (mis. setelah revert atau `review.changed`), tutup kotak komentar yang terbuka agar nomor baris tidak basi, tetapi jangan membuang teks bila pengiriman sedang berjalan.

Acceptance criteria:
- [ ] Klik nomor baris di DiffView membuka `CommentBox` di bawah baris itu; klik baris lain memindahkan kotak (satu kotak saja).
- [ ] "Kirim ke agent" memanggil `review.comment` dengan `{sessionId, path, line, text}` yang benar, lalu menutup dan menampilkan konfirmasi saat sukses.
- [ ] Saat sesi running kotak tetap terbuka, teks utuh, dan muncul pesan penolakan yang menyarankan menunggu.
- [ ] Mengetik di textarea tidak memicu shortcut `j/k/a/r`; `Esc` dan "Batal" menutup tanpa mengirim.
- [ ] Fitur tiket 02 dan 03 (render hunk, tombol Revert/Terima, navigasi keyboard) tidak berubah perilakunya.
- [ ] Build frontend lulus dan tampilan benar di tema gelap dan terang.

ownedPaths: `frontend/src/modules/review/CommentBox.svelte`, `frontend/src/modules/review/DiffView.svelte` (hanya penyisipan kecil untuk komentar).

Bisa dikerjakan paralel dengan `review-comment-daemon`. Kedua fitur menyentuh file yang berbeda; UI hanya bergantung pada kontrak `review.comment` di atas.

## Path yang dimiliki

Hanya ini yang boleh disentuh:
- `internal/review/comment.go` (baru)
- `frontend/src/modules/review/CommentBox.svelte` (baru)
- `cmd/shikigamid/wire_review.go` (perluasan kecil: registrasi `review.comment`)
- `frontend/src/modules/review/DiffView.svelte` (perubahan kecil: penyisipan `CommentBox` dan state target komentar)

Dilarang menyentuh: `internal/review/store.go`, `capture.go`, `ipc.go`, `types.go`, `diff.go`, `diff_ipc.go`, `actions.go`, `actions_ipc.go`, `frontend/src/modules/review/{index.ts,ReviewTab.svelte,store.ts,types.ts,ReviewBadge.svelte,diff.ts,HunkActions.svelte,keys.ts}`, `internal/session/`, `internal/status/`, `internal/ipc/`, `go.mod`, `frontend/package.json`.

## Wiring

Penutup tiket ini (tidak butuh tiket lain di luar 01 sampai 03 yang sudah ada):
1. Di `wire_review.go`, dalam callback `d.OnReady` modul `review`, tambahkan registrasi `review.comment` memakai `session.manager` dari `d.Lookup("session.manager")` dan status engine yang sudah di-lookup untuk modul review. Tanpa mengubah urutan atau isi registrasi lain.
2. Di `DiffView.svelte`, sambungkan handler klik nomor baris (titik ekstensi dari tiket 02) ke state `commentTarget` dan render `CommentBox` di bawah baris terkait.
3. Verifikasi manual ujung ke ujung: jalankan aplikasi, biarkan satu sesi Claude mengedit file, buka tab Review, buka diff, klik nomor baris, kirim komentar saat sesi idle (teks muncul di input terminal tanpa Enter), lalu saat sesi running (ditolak dengan pesan).
4. Pastikan semua perilaku tiket 01 sampai 03 masih berjalan (daftar file, badge, diff, revert, terima).

## Guard

- `go build ./...` lulus.
- Build frontend lulus (`npm run build` di `frontend/`, atau `make build` bila itu target yang dipakai repo).
- Tanpa unit test dan tanpa dependency baru.
