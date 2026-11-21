---
description: Tampilkan status semua tiket sebagai tabel (status, plan, progress, depends, langkah berikutnya)
---

Tampilkan status tiket. Tanpa argumen; jangan bertanya.

1. Jalankan dengan Bash: `bash scripts/ticket-status.sh --md` dari root repo. Skrip hanya membaca (frontmatter tiket, folder `docs/plans/`, `git worktree list`) dan tidak mengubah apa pun. Keluarannya sudah berupa tabel markdown.
2. Tampilkan tabel itu apa adanya sebagai tabel markdown (BUKAN di dalam blok kode, supaya dirender sebagai tabel), beserta baris "Ringkasan" dan keterangan "Depends" di bawahnya. Jangan menghitung ulang atau mengarang status.
3. Tambahkan paling banyak tiga kalimat: tiket yang sudah selesai, yang tertunda atau sedang dikerjakan (sebutkan langkah manualnya bila ada), dan satu saran perintah berikutnya yang paling masuk akal dari kolom "Langkah berikutnya". Bila ada tiket yang menunggu dependensi (`NN:BELUM`), sebutkan tiket penunggunya dan dependensi yang ditunggu.
