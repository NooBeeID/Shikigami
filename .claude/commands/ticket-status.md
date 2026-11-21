---
description: Tampilkan status semua tiket (status, plan, progress, langkah berikutnya)
---

Tampilkan status tiket. Tanpa argumen; jangan bertanya.

1. Jalankan dengan Bash: `bash scripts/ticket-status.sh` dari root repo. Skrip hanya membaca (frontmatter tiket, folder `docs/plans/`, `git worktree list`) dan tidak mengubah apa pun.
2. Tampilkan outputnya apa adanya (tabel) dalam blok kode. Jangan menghitung ulang atau mengarang status.
3. Tambahkan paling banyak tiga kalimat: tiket yang sudah selesai, yang tertunda atau sedang dikerjakan (sebutkan langkah manualnya bila ada), dan satu saran perintah berikutnya yang paling masuk akal dari kolom "LANGKAH BERIKUTNYA". Bila ada tiket yang menunggu dependensi, sebutkan tiket penunggunya.
