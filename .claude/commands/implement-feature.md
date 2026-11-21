---
description: Implementasi satu tiket dari plan-nya di git worktree (foundation, fitur paralel, wiring, build guard)
argument-hint: <nama-tiket> [maxAgentParalel=n|full]
---

Implementasi dari plan. Argumen: `$ARGUMENTS`

Langkah (tanpa AskUserQuestion; jangan bertanya):

1. Parse `$ARGUMENTS`: token pertama = nama tiket (tanpa `.md`). `maxAgentParalel=n|full` opsional; default `3`. `full` = semua fitur sekaligus. Nama tiket kosong = berhenti.
2. Cek plan dengan Bash: pastikan `docs/plans/<tiket>/overview.md` ada dan ada minimal satu file plan fitur lain di folder itu (selain `overview.md`, `00-foundation.md`, `progress.md`). **Bila tidak ada, langsung keluar** dengan pesan: "Plan untuk <tiket> belum ada. Jalankan /plan-from-tiket <tiket> dulu." Jangan memulai workflow.
3. Cek repo git dengan Bash (`git rev-parse HEAD`). Bila bukan repo git atau belum ada commit, berhenti dengan pesan: "Implementasi memakai git worktree. Jalankan `git init` dan commit awal dulu." Jangan menjalankan `git init` sendiri.
4. Jalankan: `Workflow({ name: 'implement-feature', args: { ticket, maxAgentParalel } })` (`maxAgentParalel` berupa angka atau string `"full"`) lalu beri tahu user bahwa workflow berjalan di background dan jangan polling.
5. Setelah selesai, laporkan: status (`OK` / `PARTIAL` / `BUILD_FAILED` / `FOUNDATION_FAILED` / `SETUP_FAILED` / `NO_PLAN`), fitur yang gagal beserta alasannya, hasil build guard, worktree (`worktree/feat-<tiket>`), branch (`feat/<tiket>`), hash commit, dan lokasi `progress.md`. Beri tahu cara merge ke branch utama: `git merge feat/<tiket>` (konflik diselesaikan saat merge) lalu `git worktree remove worktree/feat-<tiket>`. Jangan merge sendiri.

Catatan: guard hanya build (tanpa unit test). Seluruh kerja terjadi di worktree `worktree/feat-<tiket>` pada branch `feat/<tiket>`. Satu commit dibuat di akhir hanya bila build lulus. Merge ke branch utama dilakukan manual oleh user.
