---
description: Buat implementation plan per tiket (overview.md + satu plan per fitur) di docs/plans/<tiket>/
argument-hint: <nama-tiket> [--force]
---

Buat plan dari tiket. Argumen: `$ARGUMENTS`

Langkah (tanpa AskUserQuestion; jangan bertanya):

1. Parse `$ARGUMENTS`: token pertama = nama tiket (nama file di `docs/tickets/` tanpa `.md`; path lengkap juga boleh). `--force` = timpa plan yang sudah ada. Nama tiket kosong = berhenti, tampilkan isi `docs/tickets/` sebagai bantuan.
2. Jalankan: `Workflow({ name: 'plan-from-tiket', args: { ticket, force } })` (`force` true/false) lalu beri tahu user bahwa workflow berjalan di background dan jangan polling.
3. Setelah selesai, laporkan statusnya. `NOT_FOUND` = tiket tidak ada. `ALREADY_PLANNED` = plan sudah ada, sarankan `--force`. Bila `OK`, tampilkan daftar file di `docs/plans/<tiket>/` dan sarankan `/implement-feature <tiket>`.
