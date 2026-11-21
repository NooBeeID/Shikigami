---
description: Pecah spec menjadi tiket epic paralel di docs/tickets/
argument-hint: [nama-atau-path-spec]
---

Buat tiket dari spec. Argumen: `$ARGUMENTS`

Langkah (tanpa AskUserQuestion; jangan bertanya):

1. Tentukan spec:
   - Bila `$ARGUMENTS` kosong: lihat `docs/superpowers/specs/*.md`. Bila tepat satu, pakai itu. Bila lebih dari satu atau tidak ada, berhenti dan tampilkan daftarnya.
   - Bila berisi path yang ada, pakai itu. Bila berisi nama (mis. `ai-terminal`), cari file di `docs/superpowers/specs/` yang namanya memuat teks itu; bila tidak tepat satu, berhenti dan tampilkan kandidat.
2. `specSlug` = nama file tanpa `.md`, tanpa awalan tanggal `YYYY-MM-DD-`, tanpa akhiran `-design` (contoh: `2026-11-21-ai-terminal-design.md` menjadi `ai-terminal`).
3. Jalankan: `Workflow({ name: 'create-ticket-from-spec', args: { specPath, specSlug } })` lalu beri tahu user bahwa workflow berjalan di background dan jangan polling.
4. Setelah selesai, tampilkan: daftar tiket (`docs/tickets/<specSlug>-ticket-NN-<epic>.md`), wave tiap tiket, dan peringatan tumpang tindih path bila ada.
