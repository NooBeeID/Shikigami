# Monitor workflow dan sub agent yang berjalan — desain

Status: menunggu persetujuan user (2026-11-22). Ukuran: proyek (daemon, IPC, UI).

## Maksud
User menjalankan banyak workflow Claude sekaligus lintas beberapa sesi. Nama bawaan Claude hanya menampilkan nama perintah (mis. `/plan-from-tiket`), tanpa tiket yang dikerjakan (T01 atau T02), dan sub agent tidak menampilkan checklist maupun progres. User bingung workflow mana untuk tiket apa, dan ingin memantau semuanya dari satu tempat tanpa berpindah pane.

## Tujuan
- Satu menu di bilah atas yang mendaftar workflow dan sub agent yang sedang berjalan di semua sesi workspace aktif.
- Tiap baris workflow menampilkan nama workflow plus identitasnya dari `args` (mis. `plan-from-tiket · <tiket>`), sesi asalnya, dan lama berjalan.
- Tiap baris menampilkan progres bila terbaca: fase berjalan dari total (workflow), checklist selesai/total (sub agent).
- Klik baris melompat ke pane sesi asalnya (memakai `focusOrOpenSession`).
- Bila format file Claude berubah atau tidak terbaca, daftar tetap jalan tanpa detail progres.

## Bukan tujuan
- Mengendalikan workflow atau sub agent (stop, retry, kirim pesan).
- Riwayat: hanya yang sedang berjalan. Yang selesai hilang dari daftar (boleh bertahan singkat; keputusan pembuat).
- Provider selain Claude.
- Grafik, token, atau biaya.

## Anti-tujuan
- Polling terus-menerus atau pembacaan file saat menu tertutup. Spec v2 menuntut ringan dan tanpa polling.
- Daftar yang salah atau basi (workflow tampil berjalan padahal sudah selesai) lebih buruk daripada daftar kosong.
- Fitur ikut rusak setiap Claude Code mengubah format file internalnya.

## Batasan
- Memakai jalur hook per sesi yang sudah ada (`internal/status/install.go`, `hooks.go`) dan IPC daemon–UI yang sudah ada. Tanpa dependency baru.
- Hook yang dipasang sekarang hanya lima event (`UserPromptSubmit`, `Notification`, `Stop`, `SessionStart`, `PostToolUse`); `SubagentStop` sudah dikenali tetapi sengaja diabaikan oleh `ClassifyEvent`. Penambahan event tidak boleh mengubah klasifikasi status sesi yang ada.
- Claude menulis berkas di `~/.claude/projects/<proyek>/<claude-session-id>/`: `workflows/wf_*.json` (berisi `args`, `phases`, `status`, `agentCount`) dan `subagents/agent-*.meta.json` (berisi `description`, `finished`) beserta transkrip `.jsonl`. Formatnya internal dan tidak berdokumen.
- Payload hook memuat `session_id` dan `transcript_path` Claude, yang menjadi penghubung ke sesi Shikigami (lewat token peluncuran per sesi yang sudah ada).

## Pendekatan
1. **Hanya hook.** Live dan resmi, tetapi hanya tahu mulai/selesai; fase dan checklist tidak terlihat. Dibuang: tidak menjawab "progres dan checklist".
2. **Hanya baca file Claude.** Paling kaya, tetapi format internal rapuh dan belum pasti ditulis selama workflow berjalan. Dibuang: kalau formatnya berubah, seluruh fitur mati.
3. **Hibrid (dipilih user).** Hook menentukan apa yang sedang berjalan, namanya (dari `tool_input` tool `Workflow`/`Agent`), dan sesi Shikigami-nya. File Claude dibaca hanya saat menu terbuka, untuk fase dan checklist, dan boleh gagal tanpa merusak daftar.

## Keputusan user
- Pendekatan hibrid.
- Menu berupa dropdown di bilah atas, bukan panel penuh (diusulkan, belum dibantah; lihat bagian keputusan saya).

## Keputusan saya (belum dikonfirmasi user)
- Daftar mencakup semua sesi di workspace aktif, bukan hanya sesi yang sedang terlihat di layout.
- Baris yang selesai dihapus saat event selesai diterima, dan juga bila sesi Shikigami-nya berakhir (mencegah entri basi).
- Pembacaan file detail hanya saat dropdown terbuka, lalu sekali tiap beberapa detik selama terbuka; berhenti saat ditutup.
- Letak menu di bilah atas berdampingan dengan lonceng; bila spec monitor resource di-merge lebih dulu, menyesuaikan tata letaknya.
- Penanda selesai: `PostToolUse` untuk `Workflow`/`Agent` bukan penanda selesai, karena tool itu bisa berjalan di background dan hook itu hanya menandai peluncuran. Sub agent berakhir pada `SubagentStop`. Workflow berakhir saat `status` di berkas `wf_*.json` bukan lagi berjalan; kalau berkas itu tidak ada saat berjalan (lihat risiko), spike menentukan penanda lain. Menambah `PreToolUse` (matcher `Workflow` dan `Agent`) dan `SubagentStart` diizinkan.
- Entri basi: status disimpan di memori daemon saja, tanpa persistensi; restart daemon mengosongkan daftar (sesi PTY ikut mati bersama daemon). Esc, crash Claude, atau event selesai yang tidak datang dibereskan lewat rekonsiliasi dengan berkas Claude saat menu dibuka. Entri yang tidak bisa dikonfirmasi dan sesinya sudah berakhir dibuang; yang sesinya masih hidup tetap tampil dengan penanda "belum terkonfirmasi".
- Agent milik workflow tampil bersarang di bawah baris workflow-nya (tertutup secara bawaan, tampil jumlahnya); sub agent mandiri tampil datar. Model data memuat tautan induk opsional.
- Identitas dari `args`: `args` diperlakukan sebagai objek. Bila ada key `ticket`, nilainya yang dipakai (tanpa path dan `.md`); bila tidak, tampilkan ringkas pasangan key=value pertama, dipotong. Aturannya generik, tanpa aturan per workflow. Untuk sub agent, identitas adalah `description`.
- Lingkup: hanya workspace aktif. Tombol menu menampilkan lencana jumlah entri berjalan. Baris yang sesinya sudah tidak ada ditampilkan redup dan tidak bisa diklik.

## Risiko yang harus diuji lebih dulu
Belum diketahui apakah `workflows/wf_*.json` sudah ada selama workflow masih berjalan, atau hanya ditulis saat selesai (semua contoh yang diperiksa berstatus `completed`). Bila hanya ditulis saat selesai, fase berjalan tidak bisa dibaca dari file dan baris workflow hanya menampilkan nama + lama berjalan. Spike singkat dengan satu workflow yang sedang jalan harus menjawab ini di awal rencana, dan rencana menyesuaikan.

## Diserahkan ke pembuat
- Bentuk event/RPC IPC, struktur paket di `internal/`, dan komponen Svelte (ikuti `frontend/src/modules/status` dan `topbar`).
- Pencocokan `session_id` Claude ke sesi Shikigami, dan penentuan folder `<claude-session-id>` dari `transcript_path`.
- Cara membaca checklist sub agent dari transkrip (mis. panggilan TodoWrite/TaskCreate terakhir) dan toleransi bila tidak ada.
- Format tampilan durasi, urutan baris, dan gaya dropdown.
