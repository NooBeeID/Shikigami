---
description: Implementasi satu tiket dari plan-nya di git worktree, lalu squash merge ke branch utama bila build lulus
argument-hint: <nama-tiket> [maxAgentParalel=n|full]
---

Implementasi dari plan. Argumen: `$ARGUMENTS`

Langkah (tanpa AskUserQuestion; jangan bertanya):

1. Parse `$ARGUMENTS`: token pertama = nama tiket (tanpa `.md`). `maxAgentParalel=n|full` opsional; default `3`. `full` = semua fitur sekaligus. Nama tiket kosong = berhenti.
2. Cek plan dengan Bash: pastikan `docs/plans/<tiket>/overview.md` ada dan ada minimal satu file plan fitur lain di folder itu (selain `overview.md`, `00-foundation.md`, `progress.md`). **Bila tidak ada, langsung keluar** dengan pesan: "Plan untuk <tiket> belum ada. Jalankan /plan-from-tiket <tiket> dulu." Jangan memulai workflow.
3. Cek repo git dengan Bash (`git rev-parse HEAD`). Bila bukan repo git atau belum ada commit, berhenti dengan pesan: "Implementasi memakai git worktree. Jalankan `git init` dan commit awal dulu." Jangan menjalankan `git init` sendiri.
4. Jalankan: `Workflow({ name: 'implement-feature', args: { ticket, maxAgentParalel } })` (`maxAgentParalel` berupa angka atau string `"full"`) lalu beri tahu user bahwa workflow berjalan di background dan jangan polling.
5. Setelah selesai, laporkan: status (`OK` / `PARTIAL` / `BUILD_FAILED` / `FOUNDATION_FAILED` / `SETUP_FAILED` / `NO_PLAN`), fitur yang gagal beserta alasannya, hasil build guard, dan hasil merge (`merge.outcome`):
   - `MERGED`: tampilkan hash commit merge di branch utama, hasil build setelah merge (`postMergeBuildPassed`), dan apakah worktree + branch sudah dihapus (`cleaned`).
   - `CONFLICT` / `BLOCKED` / `SKIPPED` / `FAILED`: sebutkan alasannya. Worktree `worktree/feat-<tiket>` dan branch `feat/<tiket>` dibiarkan utuh. Beri tahu cara manual: `git merge --squash feat/<tiket>` dari checkout utama, selesaikan konflik, commit, lalu `git worktree remove worktree/feat-<tiket>` dan `git branch -D feat/<tiket>`.

   Bila `deferredManual` tidak kosong, sebutkan langkah manual yang ditunda (mis. `spike-measure`) dan ingatkan user untuk menjalankannya sendiri dengan daemon/app berjalan. Fitur manual tidak membuat status `PARTIAL` dan tidak menghalangi merge, tetapi status tiket di frontmatter tetap `in-progress` sampai langkah itu selesai.

Catatan:
- Fitur yang menurut `overview.md` dikerjakan manual/terakhir (butuh daemon atau aplikasi berjalan, mis. pengukuran) dilewati workflow dan dilaporkan sebagai `deferredManual`.
- Guard hanya build (tanpa unit test). Seluruh kerja terjadi di worktree `worktree/feat-<tiket>` pada branch `feat/<tiket>`.
- Satu commit dibuat di branch fitur hanya bila build lulus. **Squash merge otomatis ke branch utama hanya bila status `OK`** (semua fitur selesai dan build lulus), lalu worktree dan branch dihapus. Status selain `OK` tidak di-merge.
- Merge tidak pernah memakai `--force`, tidak menyelesaikan konflik sendiri (konflik = batal dan lapor), dan tidak melakukan push.
- Jalankan satu tiket per waktu untuk merge ke branch yang sama; tidak ada penguncian bila dua run merge bersamaan.
