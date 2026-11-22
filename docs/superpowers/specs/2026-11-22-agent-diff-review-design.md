# Review diff perubahan agent per sesi — desain

Status: draft menunggu persetujuan user (2026-11-22). Ukuran: proyek (daemon, hooks, IPC, UI).

## Maksud
Setelah agent bekerja, user ingin tahu persis apa yang diubah sesi itu, menerima atau membatalkannya per bagian, dan memberi komentar yang kembali ke agent. Sekarang file hanya bisa dilihat atau diedit, dan tidak ada konsep "perubahan milik sesi X". Review lewat `git diff` manual tidak bisa memisahkan perubahan antar sesi atau dari edit user sendiri.

## Tujuan
- Tiap sesi punya daftar file yang diubah agent (Edit, Write, MultiEdit, NotebookEdit), diperbarui langsung saat agent bekerja.
- Diff per file (baseline sesi vs kondisi sekarang), tampilan unified dengan penanda hunk.
- Aksi per hunk dan per file: Revert (kembalikan ke baseline) dan Terima (tandai sudah ditinjau, file tidak berubah).
- Komentar per baris atau hunk yang dikirim ke sesi sebagai pesan ("di `path:baris`: ...") lewat jalur `session.write`.
- Badge jumlah file berubah dan belum ditinjau di baris sesi (`SessionRow`) dan di tab review.
- Berfungsi di folder non-git.
- Klik file di daftar membuka file di pane yang ada (`openFilePane`) pada baris terkait.

## Bukan tujuan
- Commit, stage, atau push dari UI ini (modul Git yang ada sudah mengurus MR).
- Mengatribusi perubahan dari proses di luar agent (edit user, formatter, `Bash` seperti `sed -i`) ke sesi. Bila terdeteksi, ditandai "perubahan lain".
- Diff semantik atau tampilan side-by-side (v1 unified saja).
- Isolasi worktree per sesi (fitur terpisah, nomor 2). Spec ini harus tetap benar bila dua sesi berbagi satu working tree.

## Anti-tujuan
- Revert yang menimpa pekerjaan user atau sesi lain secara diam-diam. Revert harus gagal dengan pesan jelas bila file sudah berubah sejak diff dihitung.
- Menyimpan salinan seluruh repo. Snapshot hanya untuk file yang benar-benar disentuh.
- Membaca atau mengirim isi file ke luar mesin.
- Daftar yang basi: file yang sudah dikembalikan ke baseline harus hilang dari daftar.

## Batasan
- Memakai jalur hook per sesi yang ada (`internal/status/install.go`, `hooks.go`). `PreToolUse` sekarang dipasang dengan matcher `Workflow|Agent` untuk registry runs; menambah matcher tool edit harus tidak mengubah klasifikasi status sesi dan tidak mengganggu registry runs.
- `PostToolUse` sudah menjadi sumber `ActivityLog` (hanya ringkasan satu baris); payload `tool_input` memuat `file_path`.
- Hook berjalan sinkron sebelum tool dieksekusi: hook `PreToolUse` harus cepat dan tidak boleh memblokir atau menolak tool call (selalu exit 0).
- Tanpa dependency baru; diff dihitung di Go (library `diff` standar tidak ada, boleh implementasi Myers kecil atau memanggil `git diff --no-index` lewat runner ber-allowlist seperti `internal/gitdata`).
- Provider selain Claude (tanpa hooks) tidak didukung di v1; tab review menampilkan "provider tidak mendukung".
- Path harus tervalidasi berada di dalam `cwd` proyek sesi (pakai aturan yang sama dengan `internal/fsx`).

## Pendekatan
- A. `git diff` working tree vs HEAD per proyek. Sederhana, tetapi mencampur perubahan user, sesi lain, dan yang sudah dikomit. Tidak jalan di non-git. Ditolak.
- B. Snapshot seluruh tree saat sesi mulai (index sementara + `git write-tree`). Atribusi baik, tetapi mahal di repo besar dan tetap mencampur sesi paralel. Ditolak.
- **C. Baseline per file lewat hook. Dipilih.** `PreToolUse` (matcher `Edit|Write|MultiEdit|NotebookEdit`) menyalin isi file ke penyimpanan sesi sekali, pada sentuhan pertama (file baru dicatat sebagai "tidak ada"). `PostToolUse` menandai file berubah dan memicu event. Diff dihitung malas saat UI meminta.

## Desain

Backend (paket baru `internal/review`):
- Penyimpanan di direktori data aplikasi: `review/<sessionId>/baseline/<hash(path)>` (isi) dan `index.json` (path, ada-tidaknya sebelumnya, sha256 baseline, waktu sentuh pertama, status ditinjau). Dibersihkan saat sesi berakhir lewat batas simpan (mis. 20 sesi terakhir, sejalan `activityEndedKeep`).
- Hook baru ditangani di `status.hook`: event `PreToolUse` dengan `tool_name` edit → `review.Capture(sessionId, path)`. Jalur `ClassifyEvent` tidak berubah.
- IPC:
  - `review.files {sessionId}` → `[{path, status: modified|added|deleted, additions, deletions, reviewed, external}]`.
  - `review.diff {sessionId, path}` → hunk terstruktur `[{id, oldStart, oldLines, newStart, newLines, lines[]}]` plus hash isi saat ini.
  - `review.revert {sessionId, path, hunkIds?, expectHash}` → menerapkan balik hunk (atau seluruh file bila `hunkIds` kosong). Gagal `[CONFLICT]` bila hash isi sekarang ≠ `expectHash`.
  - `review.accept {sessionId, path, hunkIds?}` → tandai ditinjau; baseline diperbarui ke isi sekarang untuk hunk itu agar diff berikutnya hanya menunjukkan perubahan baru.
  - `review.comment {sessionId, path, line, text}` → menyusun pesan dan menulisnya lewat `session.write` ke PTY sesi (diakhiri Enter hanya bila sesi `idle` atau `waiting`; bila `running`, ditolak dengan saran menunggu).
- Event `review.changed {sessionId, count}` dipublikasikan setelah `PostToolUse` edit, didebounce (mis. 300 ms) agar tidak membanjiri.
- Deteksi "perubahan lain": saat `review.files`, bila hash isi sekarang ≠ hash yang diharapkan setelah edit agent terakhir (dicatat di `PostToolUse`), file ditandai `external`. Revert tetap diizinkan hanya dengan konfirmasi eksplisit di UI.
- File biner atau di atas batas ukuran (mis. 1 MB): masuk daftar tanpa diff, hanya aksi Revert seluruh file.

Frontend (modul baru `frontend/src/modules/review/`):
- Tab kanan "Review" didaftarkan lewat `registerRightTab` (mengikuti `sidebar-right`), memantau sesi aktif.
- `ReviewTab.svelte`: daftar file + ringkasan `+/-`, filter "belum ditinjau".
- `DiffView.svelte`: unified diff, tombol per hunk (Revert/Terima), klik nomor baris untuk menulis komentar, tombol "Kirim ke agent".
- Badge di `SessionRow` dari event `review.changed`.
- Navigasi keyboard: `j/k` antar hunk, `a` terima, `r` revert (dengan konfirmasi), mengikuti pola `FilesTab`.

Alur data: PreToolUse → baseline tersimpan → agent menulis file → PostToolUse → `review.changed` → UI memanggil `review.files` → user membuka file → `review.diff` → Revert/Terima/Komentar.

## Kriteria penerimaan
- Dua edit berturut-turut pada file yang sama dalam satu sesi menghasilkan satu diff terhadap baseline pertama.
- File baru yang dibuat agent muncul sebagai `added`; Revert menghapusnya.
- Revert satu hunk dari tiga hunk hanya mengembalikan hunk itu; dua lainnya utuh.
- Bila user mengedit file setelah diff dibuka, Revert gagal dengan `[CONFLICT]` dan diff dimuat ulang.
- Di folder non-git, semua alur di atas tetap jalan.
- Hook tidak menambah latensi terukur pada tool call (target < 20 ms untuk file < 1 MB).
- Provider tanpa hooks menampilkan pesan, tanpa error.

## Risiko dan pertanyaan terbuka
- `Bash` yang mengubah file (`sed -i`, generator) tidak tertangkap hook edit. v1 menerima celah ini; opsi lanjutan: bandingkan `git status` sebelum dan sesudah `Bash`.
- Dua sesi menyunting file yang sama di working tree yang sama: baseline masing-masing benar, tetapi Revert satu sesi bisa menyentuh hasil sesi lain. Pengaman: cek `expectHash` dan ditandai `external`.
- Format payload `tool_input` tiap tool edit (terutama `MultiEdit`, `NotebookEdit`) belum diverifikasi; perlu spike singkat sebelum tiket hook.
- Menulis komentar lewat `session.write` bergantung pada keadaan prompt agent; perlu keputusan apakah cukup ditempel ke input tanpa Enter (user menekan Enter sendiri) sebagai perilaku default yang lebih aman.
