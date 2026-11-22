---
id: agent-diff-review-ticket-03-review-actions
title: Review: revert, terima, dan deteksi perubahan lain
spec: docs/superpowers/specs/2026-11-22-agent-diff-review-design.md
status: implemented
wave: 3
depends_on: [agent-diff-review-ticket-02-review-diff]
---
# Review: revert, terima, dan deteksi perubahan lain

## Tujuan
Epic ini menambahkan aksi pada hasil review: Revert (kembalikan ke baseline sesi) dan Terima (tandai ditinjau, baseline maju) per hunk maupun per file. Aksi harus aman: Revert tidak boleh menimpa pekerjaan user atau sesi lain secara diam-diam, sehingga gagal dengan `[CONFLICT]` bila isi file berubah sejak diff dihitung, dan butuh konfirmasi ekstra bila file ditandai `external` ("perubahan lain"). Setelah aksi, daftar file dan badge menyusut karena file yang identik dengan baseline hilang dari daftar. Navigasi keyboard `j/k/a/r` melengkapi alur tinjau cepat.

## Hasil yang bisa dijalankan
Di atas tiket 01 (daftar file) dan 02 (diff unified), pengguna dapat:
- Membuka file di tab Review, lalu klik "Revert" atau "Terima" pada tiap hunk dan pada header file.
- Revert selalu meminta konfirmasi; untuk file `external` konfirmasinya lebih keras (peringatan bahwa ada perubahan di luar agent) dan memakai `force=true`.
- Revert satu hunk dari tiga hunk hanya mengembalikan hunk itu; dua lainnya utuh. File `added` dihapus saat Revert penuh. File `deleted` dibuat kembali dari baseline.
- Terima membuat diff berikutnya hanya menampilkan perubahan baru; file yang seluruhnya diterima atau kembali identik dengan baseline hilang dari daftar, dan badge di `SessionRow` serta tab ikut turun.
- Bila file diedit setelah diff dibuka, Revert gagal dengan pesan jelas dan diff dimuat ulang otomatis.
- Keyboard: `j`/`k` pindah antar hunk, `a` terima hunk terpilih, `r` revert hunk terpilih (dengan konfirmasi).
- Badge `external` ("perubahan lain") tampil pada file yang bersangkutan di daftar.
- Semua jalan di folder non-git. `go build ./...` dan build frontend lulus, aplikasi berjalan.

## Batasan
In scope:
- `review.revert` dan `review.accept` di daemon, flag `external` pada `review.files`, event `review.changed` setelah aksi.
- UI `HunkActions.svelte`, `keys.ts`, serta sisipan di `DiffView.svelte` dan `ReviewTab.svelte` lewat slot.

Out of scope:
- Komentar ke agent (`review.comment`, `CommentBox`) milik tiket 04.
- Penangkapan baseline, hook, dan pencatatan `expectedHash` pasca edit agent (tiket 01; tiket ini hanya membaca nilainya). Bila field itu belum ada di index, baca `internal/review/types.go` dan `store.go` dari tiket 01 untuk nama yang sebenarnya.
- Perhitungan hunk (tiket 02; tiket ini memakai fungsinya, jangan menulis ulang).
- Commit/stage/push, diff side-by-side, atribusi perubahan `Bash`.
- Dependency baru dan unit test.

## Kontrak
Dipakai dari tiket sebelumnya (baca kode aktualnya sebelum membuat plan):
- Store review (tiket 01): index.json per sesi (path, existedBefore, sha256 baseline, waktu sentuh pertama, reviewed, expectedHash terakhir pasca PostToolUse), berkas baseline `DataDir/review/<sessionId>/baseline/<sha256(path)>`, cwd sesi, publikasi event `review.changed {sessionId,count}` ber-debounce 300 ms, `review.files`.
- Hitung hunk dan hash isi saat ini (tiket 02): hunk dengan `id`, `oldStart`, `oldLines`, `newStart`, `newLines`, `lines[]`; `currentHash` = sha256 isi file sekarang.
- Validasi path di dalam cwd dengan aturan `internal/fsx`.
- Frontend: `DiffView.svelte` menyediakan slot aksi hunk; `ReviewTab.svelte` dan `store.ts` menyediakan daftar file per sesi aktif.

Disediakan epic ini:
- `review.revert {sessionId, path, hunkIds?, expectHash, force?}` -> `{ok:true, removed:bool}` (`removed` true bila file hilang dari daftar). Error `"[CONFLICT] ..."` bila sha256 isi sekarang != `expectHash`. Error `"[EXTERNAL] ..."` bila file `external` dan `force` bukan true. `hunkIds` kosong/absen berarti seluruh file.
- `review.accept {sessionId, path, hunkIds?}` -> `{ok:true, removed:bool}`.
- `review.files` item kini punya `external` yang benar: `external = (sha256 isi sekarang != expectedHash dari edit agent terakhir)`. File yang tidak ada lagi dibandingkan sebagai "tidak ada" (hash kosong).
- Event `review.changed` dipublikasikan setelah revert/accept.
- Frontend: komponen `HunkActions.svelte` (props: `hunk`, `path`, `currentHash`, `external`, `scope: 'hunk'|'file'`, callback `onDone`), modul `keys.ts` (`attachReviewKeys(handlers)` mengembalikan fungsi lepas).

Aturan perilaku inti:
- Revert penuh: file `modified` -> tulis isi baseline; `added` (tidak ada sebelumnya) -> hapus file; `deleted` -> buat ulang dengan isi baseline (buat direktori induk bila perlu). Sesudahnya entri dibuang dari index dan baseline dihapus.
- Revert sebagian: balik-terapkan hunk terpilih pada isi sekarang (ganti baris `newStart..newStart+newLines` dengan baris lama dari hunk). Terapkan dari hunk paling bawah ke atas agar nomor baris tetap valid. Baseline tidak berubah. Bila hasil identik dengan baseline, perlakukan seperti revert penuh (entri hilang).
- Accept penuh: baseline <- isi sekarang, `reviewed=true`, `expectedHash` <- hash sekarang; karena identik, entri dibuang dari daftar. File `deleted` yang diterima: entri dibuang. File `added` yang diterima: entri dibuang (baseline baru = isi sekarang).
- Accept sebagian: terapkan maju hunk terpilih pada isi baseline (hasilnya baseline baru) sehingga diff berikutnya hanya memuat hunk sisanya; entri dibuang bila baseline baru == isi sekarang.
- Penulisan file atomik (tulis ke berkas sementara di direktori yang sama lalu rename), pertahankan mode file asli.
- Semua aksi ditulis serial per sesi (mutex di store) agar tidak balapan dengan hook Capture.

## Fitur

### review-actions-daemon: review.revert, review.accept, dan flag external
Deskripsi: implementasi `internal/review/actions.go` (logika murni di atas store: `Revert(sessionID, path, hunkIDs, expectHash, force)`, `Accept(sessionID, path, hunkIDs)`, `IsExternal(sessionID, path)`) dan `actions_ipc.go` (registrasi handler IPC `review.revert` dan `review.accept` lewat mekanisme registrasi modul yang sama dengan `ipc.go` tiket 01, tanpa mengubah `ipc.go` atau `wire_review.go`; bila registrasi hanya bisa lewat `wire_review.go`, sediakan fungsi `RegisterActions(...)` di `actions_ipc.go` yang dipanggil dari registrasi yang sudah ada sebagaimana disiapkan tiket 01; lihat bagian Wiring). Langkah revert: validasi path di cwd; muat file saat ini (baca isi, hitung sha256); bandingkan dengan `expectHash`, beda -> `[CONFLICT] file berubah sejak diff dihitung, muat ulang`; bila `external` dan `!force` -> `[EXTERNAL] ...`; terapkan sesuai aturan perilaku inti; perbarui index; publikasikan `review.changed` dengan count baru. Hunk dicocokkan lewat `hunkIds` terhadap hunk yang dihitung ulang dari baseline vs isi sekarang (id stabil karena isi sekarang sudah terkonfirmasi sama dengan `expectHash`); id tak dikenal -> error jelas. File biner atau di atas batas ukuran hanya mendukung aksi seluruh file (hunkIds terisi -> error). Flag `external` diisi pada pembuatan daftar `review.files` dengan menyambungkan `IsExternal` di sana (perubahan kecil pada pembentuk item; tetap tidak mengubah kontrak). Jalan di non-git (hanya I/O file biasa).
Acceptance criteria:
- [ ] Revert hunk tunggal dari tiga hunk hanya mengembalikan hunk itu; dua lainnya utuh dan baseline tidak berubah.
- [ ] Revert penuh file `added` menghapus file dan membuangnya dari daftar; file `deleted` dibuat ulang dengan isi baseline; file `modified` kembali identik byte demi byte dengan baseline.
- [ ] `expectHash` tidak cocok -> error diawali `[CONFLICT]`, file tidak berubah.
- [ ] File `external` tanpa `force` -> `[EXTERNAL]`; dengan `force=true` revert berjalan (tetap lolos cek `expectHash`).
- [ ] Accept penuh membuat file hilang dari `review.files`; accept hunk membuat diff berikutnya hanya memuat hunk sisanya.
- [ ] `review.files` menandai `external=true` bila isi berubah setelah edit agent terakhir dan `false` sebaliknya.
- [ ] Path di luar cwd atau symlink keluar ditolak oleh validasi `fsx`; tidak ada penulisan di luar cwd.
- [ ] Event `review.changed` dipublikasikan setelah tiap aksi dengan count terbaru.
- [ ] Berjalan di folder non-git; `go build ./...` lulus.
ownedPaths: `internal/review/actions.go`, `internal/review/actions_ipc.go`.
Bisa dikerjakan paralel dengan `review-actions-ui` (kontrak IPC di atas sudah tetap); UI hanya dapat diuji ujung-ke-ujung setelah keduanya selesai.

### review-actions-ui: Tombol hunk, konfirmasi, dan keyboard
Deskripsi: `HunkActions.svelte` merender tombol "Revert" dan "Terima" untuk satu hunk (`scope='hunk'`) atau seluruh file (`scope='file'`, dipasang di header file pada `ReviewTab.svelte`). Revert membuka konfirmasi inline (mengikuti pola konfirmasi di `FilesTab`: tombol berubah jadi "Yakin?" / "Batal" tanpa modal baru), dan untuk file `external` menampilkan peringatan "File ini diubah di luar agent; revert akan menimpa perubahan itu" lalu mengirim `force:true`. Panggilan: `review.revert` dengan `expectHash = currentHash` dari `review.diff` yang sedang ditampilkan, `review.accept` untuk Terima. Penanganan error: pesan diawali `[CONFLICT]` -> tampilkan "File berubah sejak diff dihitung, dimuat ulang" lalu muat ulang `review.diff` dan daftar file; `[EXTERNAL]` -> munculkan konfirmasi keras; error lain tampil sebagai pesan singkat. Selama panggilan, tombol nonaktif (cegah klik ganda). Setelah sukses panggil `onDone` agar `DiffView` memuat ulang diff dan store daftar file disegarkan; bila `removed` true, pilih file berikutnya atau tampilkan keadaan kosong. Badge `external` ("perubahan lain") ditampilkan pada baris file di `ReviewTab.svelte`. `keys.ts`: `attachReviewKeys` memasang listener keyboard pada panel diff (hanya saat fokus berada di panel, dan tidak saat fokus di input/textarea): `j`/`k` pindah hunk terpilih (sorot visual dan `scrollIntoView`), `a` terima hunk terpilih, `r` memulai konfirmasi revert hunk terpilih (`r` lagi atau Enter mengonfirmasi, Esc membatalkan). Kunci tidak boleh bentrok dengan shortcut yang ada di `FilesTab` atau shell; periksa pola di `FilesTab.svelte`. Wiring ke `DiffView.svelte` (isi slot aksi hunk) dan `ReviewTab.svelte` (aksi file, badge external, pasang keys) adalah sisipan minimal pada berkas milik tiket sebelumnya.
Acceptance criteria:
- [ ] Tiap hunk menampilkan tombol Revert dan Terima; header file menampilkan aksi untuk seluruh file.
- [ ] Revert selalu lewat konfirmasi; file `external` menampilkan peringatan tambahan dan mengirim `force:true`.
- [ ] `[CONFLICT]` menampilkan pesan jelas dan memuat ulang diff tanpa aksi lanjutan otomatis.
- [ ] Setelah aksi sukses, hunk/file menghilang, daftar dan badge menyusut, tanpa reload manual.
- [ ] `j/k/a/r` bekerja sesuai deskripsi dan tidak aktif saat mengetik di input.
- [ ] Tombol nonaktif saat panggilan berjalan; tidak ada error tak tertangani di konsol.
- [ ] Teks UI berbahasa Indonesia; identifier kode berbahasa Inggris; mengikuti token tema yang ada.
- [ ] Build frontend lulus.
ownedPaths: `frontend/src/modules/review/HunkActions.svelte`, `frontend/src/modules/review/keys.ts`, `frontend/src/modules/review/DiffView.svelte` (hanya sisipan slot aksi), `frontend/src/modules/review/ReviewTab.svelte` (hanya sisipan aksi file, badge external, pemasangan keys).
Bisa dikerjakan paralel dengan `review-actions-daemon`; bila UI selesai lebih dulu, panggilan IPC akan gagal sampai daemon selesai, jadi uji penuh setelah keduanya merge.

## Path yang dimiliki
Hanya path berikut yang boleh disentuh:
- `internal/review/actions.go`
- `internal/review/actions_ipc.go`
- `frontend/src/modules/review/HunkActions.svelte`
- `frontend/src/modules/review/keys.ts`
- `frontend/src/modules/review/DiffView.svelte` (hanya sisipan lewat slot)
- `frontend/src/modules/review/ReviewTab.svelte` (hanya sisipan lewat slot/titik ekstensi)

Pengecualian sempit yang diizinkan hanya bila tidak ada titik ekstensi lain: satu baris pemanggilan registrasi di pembentuk item `review.files` (untuk mengisi `external`) dan di registrasi IPC yang sudah disediakan tiket 01. Jangan mengubah perilaku lain di `store.go`, `capture.go`, `ipc.go`, `types.go`, `diff.go`, `wire_review.go`.

## Wiring
Langkah penutup tiket ini, tanpa bergantung pada tiket 04:
1. Daemon: daftarkan handler `review.revert` dan `review.accept` lewat titik registrasi IPC modul review dari tiket 01 (fungsi `RegisterActions` dari `actions_ipc.go` dipanggil dari tempat handler `review.files`/`review.diff` didaftarkan). Sambungkan `IsExternal` ke pembentuk item `review.files` sehingga flag `external` terisi.
2. Frontend: isi slot aksi hunk di `DiffView.svelte` dengan `HunkActions.svelte`, tambahkan aksi file dan badge `external` di `ReviewTab.svelte`, pasang `attachReviewKeys` pada panel diff (lepas saat unmount).
3. Pastikan setelah aksi, store daftar file disegarkan (lewat `review.changed` atau pemanggilan ulang `review.files`) sehingga `ReviewBadge` di `SessionRow` ikut turun.
4. Jalankan aplikasi: sesi Claude mengedit file, buka tab Review, uji Revert hunk, Revert file, Terima, `[CONFLICT]` (edit file dari luar saat diff terbuka), dan file `external`.

## Guard
Cukup build berhasil, tanpa unit test:
- `go build ./...`
- build frontend (perintah build di `frontend/`, mis. `npm run build`)
