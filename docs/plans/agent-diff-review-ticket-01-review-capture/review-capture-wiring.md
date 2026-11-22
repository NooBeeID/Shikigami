# Plan: Wiring tiket review-capture

## Metadata

- Tiket: `agent-diff-review-ticket-01-review-capture` (`docs/tickets/agent-diff-review-ticket-01-review-capture.md`)
- Fitur: `review-capture-wiring`
- ownedPaths: `frontend/src/modules/sidebar-left/SessionRow.svelte`, `frontend/src/main.ts`
- Depends on foundation: ya (`00-foundation.md`). Fitur ini juga baru dikerjakan setelah `review-foundation`, `review-hooks-ipc`, dan `review-ui-list` selesai; semua kontrak ada di `overview.md` (bagian 2, 5, 6).

## Tujuan

Menutup tiket: memasang `ReviewBadge` di baris sesi sidebar kiri dengan perubahan minimal, lalu memverifikasi registrasi modul daemon dan frontend serta alur penuh di aplikasi yang berjalan (sesi Claude edit file, edit dua kali satu entri, sesi shell tidak didukung, folder non-git). Fitur ini tidak menambah logika baru.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `frontend/src/modules/sidebar-left/SessionRow.svelte`
- `frontend/src/main.ts` (hanya bila glob tidak mencakup `modules/review/index.ts`; keadaan saat plan ditulis: sudah tercakup, jadi TIDAK diubah)

Dilarang menyentuh: `cmd/shikigamid/modules.go` dan `wire_review.go` (hanya diverifikasi), `internal/review/*`, `internal/status/*`, `frontend/src/modules/review/*`, `frontend/src/modules/sidebar-right/FilesTab.svelte` (ada perubahan user yang belum di-commit), `internal/runs`, dan file tiket 02-04.

## Kontrak

Dipakai (disediakan fitur lain, lihat overview 2.4, 2.5):

- `frontend/src/modules/review/index.ts` mengekspor `ReviewBadge` (komponen Svelte, props `{ sessionId: string }`, tidak merender apa pun bila count 0), memanggil `registerRightTab({ id: 'review', title: 'Review', order: 15, component: ReviewTab })` dan `startReviewSync()`.
- `cmd/shikigamid/wire_review.go` memuat `func init() { RegisterModule("review", ...) }` dan `d.Provide("review.store", store)`.
- IPC `review.files`, event `review.changed`, matcher `PreToolUse` `Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit`.

Disediakan: tidak ada simbol baru.

## Langkah

1. Pra-cek. Pastikan ketiga fitur lain sudah ada:
   - `ls internal/review/ipc.go cmd/shikigamid/wire_review.go frontend/src/modules/review/index.ts frontend/src/modules/review/ReviewBadge.svelte`.
   - `grep -n "ReviewBadge" frontend/src/modules/review/index.ts` harus menemukan `export { ReviewBadge }` (atau setara). Bila belum ada, berhenti dan laporkan `ok:false` dengan alasan (ketergantungan belum selesai); jangan membuat file milik fitur lain.
2. Edit `frontend/src/modules/sidebar-left/SessionRow.svelte`, tepat dua perubahan:
   - Blok import, tepat setelah baris `import StatusBadge from '../status/StatusBadge.svelte';` tambahkan satu baris: `import { ReviewBadge } from '../review';`
   - Di dalam `<button type="button" class="main" ...>`, tepat setelah `<span class="title">{title}</span>` dan sebelum `<span class="prov">{badge}</span>`, tambahkan satu baris: `<ReviewBadge sessionId={session.id} />`
   - Jangan mengubah CSS, logika, atau elemen lain. Badge tidak merender apa pun saat count 0, sehingga tata letak flex (`.main`, `gap: 10px`) tidak berubah saat count 0.
3. Verifikasi `frontend/src/main.ts`: harus berisi `import.meta.glob('./modules/*/index.ts', { eager: true });`. Glob ini mencakup `./modules/review/index.ts`, maka JANGAN mengubah `main.ts`. Ubah hanya bila glob itu tidak ada (tambahkan glob yang sama, tanpa import eksplisit modul review).
4. Verifikasi modul daemon (baca saja): `grep -n "RegisterModule(\"review\"" cmd/shikigamid/*.go` menemukan tepat satu hasil di `wire_review.go` di dalam `func init()`; `grep -n "review.store" cmd/shikigamid/*.go` menemukan satu `d.Provide`; `modules.go` tidak mengandung referensi khusus `review` dan tidak diubah. Bila tidak sesuai, laporkan sebagai `ok:false` dengan rincian; bukan tugas fitur ini memperbaikinya.
5. Verifikasi `modules/review/index.ts` (baca saja): `registerRightTab` dengan `order: 15` dan `startReviewSync()` dipanggil.
6. Guard dari root repo: `go build ./...`; lalu `cd frontend && npm run build` dan `cd frontend && npm run check`. Semua harus lulus. Galat yang berasal dari file milik fitur lain: laporkan, jangan perbaiki di file itu.
7. Uji alur penuh di aplikasi (gunakan skill `run`: `make run-daemon` / jalankan app; hanya matikan PID proses yang kamu luncurkan sendiri, jangan `pkill` pola luas). Sesi harus dibuat SETELAH daemon baru berjalan karena matcher hook hanya dipakai sesi baru:
   a. Buka folder proyek (git) dan sesi Claude baru. Minta agent membuat file baru dan mengedit file yang sudah ada. Harapan: tab Review (setelah Files, sebelum Activity) menampilkan file dengan penanda A/M; badge angka muncul di `SessionRow` (setelah judul, sebelum penanda provider CL); badge ikut berubah setelah sekitar 300 ms.
   b. Minta agent mengedit file yang sama dua kali. Harapan: tetap satu entri. Bila isi dikembalikan sama dengan baseline, entri hilang dan badge menyusut.
   c. Buka sesi shell (provider `shell`). Harapan: tab Review menampilkan "Provider sesi ini tidak mendukung review perubahan" tanpa galat; tidak ada badge di barisnya.
   d. Ulangi (a) pada folder tanpa `.git`. Harapan: sama.
   e. Pastikan baris sesi tanpa perubahan tampil identik dengan sebelumnya (tidak ada ruang tambahan).
   Bila lingkungan tidak memungkinkan menjalankan aplikasi GUI atau agent Claude, jalankan sebanyak mungkin (build, grep verifikasi) dan catat bagian yang tidak teruji pada laporan akhir.
8. Pastikan `git status` hanya menunjukkan perubahan di `SessionRow.svelte` (dan `main.ts` bila terpaksa) dari fitur ini; perubahan lain milik fitur paralel atau user, jangan di-revert atau di-stage.

## Perubahan

- Diubah: `frontend/src/modules/sidebar-left/SessionRow.svelte` (satu import, satu elemen).
- Diubah hanya bila perlu: `frontend/src/main.ts` (diharapkan tidak berubah).
- Tidak ada file dibuat.

## Wiring

Fitur ini adalah langkah wiring itu sendiri; tidak ada yang diekspos untuk disambungkan setelahnya. Tiket 02-04 akan memakai `reviewFiles`, `reviewCounts`, `review.store` dan titik kait `onselect` yang sudah terpasang oleh fitur lain.

## Selesai bila

- `SessionRow.svelte` memuat tepat satu import `ReviewBadge` dari `'../review'` dan satu elemen `<ReviewBadge sessionId={session.id} />` di posisi yang ditentukan; diff file itu hanya dua baris tambahan.
- `main.ts` tidak berubah (atau berubah minimal hanya bila glob tidak ada).
- Verifikasi daemon dan frontend (langkah 3-5) sesuai.
- `go build ./...`, `npm run build`, dan `npm run check` lulus.
- Alur penuh langkah 7 terkonfirmasi, atau bagian yang tidak bisa diuji dicatat secara eksplisit.
