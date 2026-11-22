# Monitor resource (CPU dan memori) — desain pendek

Status: disetujui user (2026-11-22). Ukuran: perubahan kecil.

## Maksud
User hanya ingin tahu pemakaian CPU dan memori Shikigami, tanpa perlu mengukur manual dengan `ps` seperti di `docs/perf/`.

## Tujuan
- Bilah status menampilkan satu ringkasan: total CPU % dan total memori semua proses milik Shikigami.
- Klik ringkasan itu membuka dropdown kecil berisi rincian per proses: daemon, app + WebKit, dan tiap sesi PTY beserta proses anaknya.

## Bukan tujuan
- Pemakaian seluruh mesin, disk, dan jaringan.
- Grafik atau riwayat.
- Panel penuh atau tab baru.

## Anti-tujuan
- Melanggar prinsip ringan dan tanpa polling pada spec v2. Sampling tidak boleh berjalan saat tidak ada yang melihat.

## Batasan
- Tanpa dependency baru (tidak memakai gopsutil). Pakai `ps` atau `libproc` dari Go.
- Memakai IPC daemon–UI yang sudah ada.
- Spec v2 baris 85 mengeluarkan meter resource dan baris 190 menyisakannya sebagai pertanyaan terbuka. Fitur ini menjawabnya: masuk dalam bentuk terbatas (sampling hemat, hanya saat terlihat).

## Pendekatan
1. **Sampling di daemon, diminta UI lewat IPC (dipilih).** Daemon memegang PID tiap sesi, jadi bisa menghitung pohon proses per sesi. Daemon hanya mengukur saat diminta; UI meminta tiap 5 detik dan hanya saat jendela terlihat (revisi dari "daemon sampling sendiri": lebih sederhana, dan deteksi jendela terlihat cukup di UI lewat `visibilitychange`).
2. Hanya satu angka total. Dibuang: terlalu kasar, tidak menunjuk sesi yang berat.
3. Panel penuh dengan grafik riwayat. Dibuang: berlebihan untuk kebutuhan "cuma ingin tahu".

## Keputusan user
- Ringkasan di bilah status.
- Klik membuka dropdown kecil untuk detail (bukan panel).

## Keputusan saya (belum dikonfirmasi user)
- Interval sampling 5 detik.
- Berhenti sampling saat jendela tersembunyi atau UI tidak tersambung.
- Bagian "app + WebKit" memakai PID aplikasi Wails dan proses WebKit anaknya, dihitung di sisi yang bisa mengaksesnya (app atau daemon); diputuskan saat menulis rencana.
- CPU dihitung dari selisih CPU time dibagi selisih waktu dinding, seperti metode di `docs/perf/spike-report.md`, bukan kolom `%cpu` dari `ps`.
- Memori memakai RSS; angka bias ke atas karena halaman shared (sudah dicatat di laporan perf).

## Diserahkan ke pembuat
- Bentuk event/RPC IPC, struktur paket di `internal/`, dan komponen Svelte.
- Cara mendeteksi jendela terlihat atau tersembunyi.
- Format angka dan gaya dropdown mengikuti komponen yang ada di `frontend/src/modules/status` dan `topbar`.
