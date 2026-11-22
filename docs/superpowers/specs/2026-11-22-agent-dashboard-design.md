# Dashboard dan antrean agent — desain

Status: draft menunggu persetujuan user (2026-11-22). Ukuran: menengah (frontend dominan, sedikit backend).

## Maksud
User menjalankan banyak sesi agent sekaligus. Status tiap sesi sudah ada (`running`, `waiting`, `done`, `idle` di `internal/status`) dan notifikasi macOS sudah ada, tetapi tidak ada satu tempat untuk melihat semuanya: siapa yang jalan, siapa yang menunggu keputusan, siapa yang selesai atau error, dan sudah berapa lama. User harus berpindah pane satu per satu untuk tahu.

## Tujuan
- Satu tampilan (panel atau menu bilah atas) yang mendaftar semua sesi agent di workspace aktif, dikelompokkan per status dengan prioritas: Menunggu → Berjalan → Idle → Selesai.
- Tiap baris menampilkan: nama sesi/provider, proyek, status dan lama di status itu (`since`), aktivitas terakhir (dari `ActivityLog`), dan jumlah file berubah bila fitur review diff tersedia.
- Klik baris melompat ke pane sesi (`focusOrOpenSession`).
- Sesi "Menunggu" diberi tanda menonjol dan dihitung di badge ikon dashboard; badge ikut hilang saat sesi tidak lagi menunggu.
- Ringkasan di bagian atas: jumlah sesi per status.
- Pembaruan waktu nyata dari event `status` yang sudah ada, tanpa polling.
- Opsional fase 2: aksi cepat di baris "Menunggu" (setuju/tolak permission) lewat `session.write`, bila spike membuktikan aman (lihat Risiko).

## Bukan tujuan
- Menggantikan monitor workflow dan sub agent (`2026-11-22-activity-monitor-design.md`); dashboard ini berada di level sesi. Keduanya boleh berbagi komponen baris.
- Membuat, menghentikan, atau memulai ulang sesi dari dashboard di v1 (kecuali "buka sesi").
- Riwayat lintas peluncuran aplikasi, grafik, token, atau biaya.
- Lintas workspace (v1 hanya workspace aktif).

## Anti-tujuan
- Status basi: sesi tampil "Berjalan" padahal sudah berakhir lebih buruk daripada tidak tampil.
- Polling atau timer berat saat panel tertutup. Hitung lama dari `since` di sisi UI dengan satu timer ringan hanya saat terlihat.
- Notifikasi ganda: dashboard tidak boleh menambah notifikasi macOS baru; ia hanya menampilkan state.
- Tombol approve yang menekan tombol di prompt yang salah.

## Batasan
- Sumber data hanya yang sudah ada: `status.list`, event status, `activity.event`, `session.list`. Frontend sudah memiliki store sesi (`lib/stores/sessions.ts`) dan indikator status (`status/SidebarBadges.svelte`).
- Event `Notification` dari hook menjadi pemicu `waiting`; payload-nya belum tentu membedakan "butuh izin tool" dari "agent menunggu input". Dashboard tidak boleh mengklaim lebih dari yang diketahui: label berdasarkan `Reason`.
- Sesi shell (provider `shell`) tidak punya status agent; tidak masuk daftar.
- Harus ringan untuk sampai puluhan sesi.

## Pendekatan
- A. Perluas `SessionGroups` di sidebar kiri dengan pengelompokan status. Murah, tetapi sidebar sudah penuh dan pengelompokan berdasarkan status bertabrakan dengan pengelompokan berdasarkan grup. Ditolak.
- **B. Panel baru sebagai item rail (`registerRail`), dengan store turunan gabungan status+aktivitas. Dipilih.**
- C. Jendela terpisah. Bertentangan dengan arah pane layout. Ditolak.

## Desain

Backend: tidak ada IPC baru yang wajib. Dua tambahan kecil bila diperlukan:
- `status.list` menyertakan `reason` terakhir dan `providerId` agar baris dapat memberi label "menunggu: Notification" tanpa pencarian tambahan. Ini penambahan field, bukan perubahan bentuk.
- Hitung "lama di status" memakai `since` (sudah ada).

Frontend (modul baru `frontend/src/modules/dashboard/`):
- `store.ts`: store turunan `agentRows` = gabungan `sessions` (bukan shell) × status × entri aktivitas terbaru × (opsional) `review.files` count. Subscribe ke event, tanpa polling.
- `DashboardPanel.svelte`: ringkasan + kelompok status + daftar baris. Kelompok kosong disembunyikan; "Selesai" dibatasi (mis. 10 terbaru) dan dapat diciutkan.
- `AgentRow.svelte`: baris dengan titik status (warna dan label, tidak hanya warna), nama, proyek, lama, aktivitas terakhir satu baris terpotong.
- Item rail dan badge: jumlah sesi `waiting`; titik saja bila > 0 (selaras gaya badge lonceng yang baru).
- Aksesibilitas: daftar dengan `role="list"`, status diumumkan lewat label teks, navigasi panah.
- Fase 2 (aksi cepat): tombol Setuju/Tolak pada baris `waiting` yang `Reason` dan konteks promptnya teridentifikasi pasti; mengirim keystroke lewat `session.write`. Hanya aktif setelah spike.

Alur data: event status/aktivitas → store turunan → panel diperbarui; klik baris → `focusOrOpenSession(sessionId)`.

## Kriteria penerimaan
- Dengan tiga sesi (jalan, menunggu, selesai), panel menampilkan kelompok dan urutan yang benar dan berubah segera saat status berubah, tanpa muat ulang.
- Sesi yang proses/PTY-nya berakhir pindah ke "Selesai" (source `exit`) dalam satu event.
- Badge menghitung hanya sesi `waiting` dan hilang saat 0.
- Klik baris memfokuskan pane sesi yang tepat, atau membukanya bila belum ada pane.
- Panel tertutup tidak menjalankan timer atau subscription yang bekerja berat.
- Sesi shell tidak muncul.
- Dengan 30 sesi, render tetap lancar (tanpa jank terukur saat event beruntun).

## Risiko dan pertanyaan terbuka
- Approve dari dashboard: perlu spike. Tanpa mengetahui bentuk prompt permission Claude Code yang persis (pilihan angka, Enter), mengirim keystroke berisiko menyetujui hal yang salah. Alternatif yang lebih aman: hook `PreToolUse` yang meminta keputusan ke aplikasi dan mengembalikan `permissionDecision`. Itu mengubah model hook (sinkron, menahan agent) dan harus dipertimbangkan sebagai tiket tersendiri.
- Sesi yang di-`attach` ulang atau daemon yang restart: pastikan `status.list` pulih (sudah ada `ensureStream`); uji kasus ini.
- Tumpang tindih tampilan dengan monitor workflow/sub agent: tentukan apakah keduanya digabung di satu panel dengan dua tab.
