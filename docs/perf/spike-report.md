# Laporan Spike Pengukuran RAM/CPU (Epic 02 Session PTY)

Ringkasan: daemon `aitermd` dan sesi PTY sangat ringan (5,88 MB tanpa sesi, tambahan sekitar 0,3–0,4 MB per sesi, CPU idle di bawah 0,1%). Aplikasi Wails dengan satu tab terminal memakai 187,19 MB RSS (app + 3 proses WebKit), yaitu 0,94x dari target 200 MB yang sebenarnya berlaku untuk 6 sesi. Target 6 sesi tidak diukur langsung dan hanya diestimasi (189–212 MB), sehingga berada di sekitar batas. Rekomendasi gerbang: **(b)**, lanjut Wails lalu ukur ulang UI setelah tiket 06.

## Metadata mesin dan build

| Item | Nilai |
|---|---|
| OS | macOS 15.7.7 (24G720) |
| Arsitektur | arm64, Apple M3, 8 core |
| RAM | 16,00 GB |
| Go | go1.26.3 darwin/arm64 |
| xterm | `@xterm/xterm` ^6.0.0, `@xterm/addon-fit` ^0.11.0 |
| Waktu ukur | 2026-11-21T17:46:01Z (00:46 WIB, 2026-11-22) |
| Commit saat ukur | `d410178` |
| Sumber biner yang diukur | `build/bin/*` dibuild 00:41:21, tepat setelah merge tiket 04 (`7438829`). Biner TIDAK memuat tiket 05 (provider), yang baru ter-merge 00:45. Tiket 01–04 termuat. |
| Lokasi biner | `build/bin/aitermd`, `aitermctl`, `aiterm`; diukur dari salinan di direktori sementara |
| Data dir | `AITERM_DATA_DIR` ke direktori sementara (`mktemp -d`), data nyata pengguna tidak tersentuh |

Kondisi lingkungan: mesin TIDAK idle. Load average 8,56 / 6,02 / 7,09 (15 pengguna sesi terminal), dengan proses berat lain berjalan (Antigravity IDE helper ~20% CPU, `claude` ~14%, Terminal ~7%, Lark ~4%, WindowServer ~4%) dan workflow agent lain aktif saat ukur. Dampaknya ke CPU per proses diperkirakan kecil (diukur dari CPU time proses itu sendiri), tetapi kontaminasi tidak bisa dikesampingkan.

## Metode dan perintah

- RSS: `ps -o rss= -p PID` (KB), dibagi 1024 untuk MB.
- CPU idle: selisih `ps -o time= -p PID` (diubah ke detik) dibagi selisih waktu dinding, dikali 100: `delta_cpu / delta_wall * 100`. Interval sampel 10 detik, satu sampel per titik ukur. Kolom `%cpu` (rata-rata sejak start) sengaja TIDAK dipakai. Resolusi `ps time` 0,01 detik, jadi resolusi hasil sekitar 0,1%; nilai 0 ditulis "< 0,1%".
- Sebelum tiap titik ukur ada jeda 5 detik setelah perubahan keadaan (membuat sesi, menyalakan app) agar idle.
- Sesi dibuat dengan `aitermctl session new` (provider `shell`, `/bin/zsh -l`, cwd `/Users/ascendiz`).
- Proses WebKit milik app diidentifikasi dengan membandingkan daftar PID `com.apple.WebKit*` sebelum dan sesudah app dijalankan, dan hanya PID baru yang dihitung (3 PID: GPU, WebContent, Networking). Proses WebKit yang dipakai aplikasi lain tidak dihitung.
- Jumlah thread daemon (0 sesi): `ps -M -p PID | wc -l` mengeluarkan 8 baris (termasuk baris header).
- Diukur oleh skrip sementara di luar repo; skrip, direktori data, dan semua proses sudah dibersihkan (`pgrep -x aitermd` dan `pgrep -x aiterm` kosong setelah selesai).

## Angka mentah

| Titik | Proses | PID | RSS (KB) | RSS (MB) | CPU time awal (s) | CPU time akhir (s) | Delta wall (s) | CPU idle (%) |
|---|---|---|---|---|---|---|---|---|
| daemon, 0 sesi | aitermd | 78601 | 6016 | 5,88 | 0,01 | 0,01 | 10,030 | < 0,1 (0,000) |
| daemon, 2 sesi | aitermd | 78601 | 6912 | 6,75 | 0,01 | 0,01 | 10,023 | < 0,1 (0,000) |
| daemon, 3 sesi | aitermd | 78601 | 7072 | 6,91 | 0,01 | 0,01 | 10,023 | < 0,1 (0,000) |
| app, 1 tab | aiterm | 84845 | 92928 | 90,75 | 0,32 | 0,32 | 10,032 | < 0,1 (0,000) |
| app, 1 tab | com.apple.WebKit.GPU | 84978 | 33776 | 32,98 | 0,08 | 0,08 | 10,032 | < 0,1 (0,000) |
| app, 1 tab | com.apple.WebKit.WebContent | 84979 | 49904 | 48,73 | 0,26 | 0,28 | 10,032 | 0,199 |
| app, 1 tab | com.apple.WebKit.Networking | 84980 | 15072 | 14,72 | 0,04 | 0,04 | 10,032 | < 0,1 (0,000) |
| **app, 1 tab (jumlah)** | | | **191680** | **187,19** | | | | **0,199** |
| daemon, app terbuka (3 sesi) | aitermd | 78601 | 6752 | 6,59 | 0,01 | 0,01 | 10,029 | < 0,1 (0,000) |

Shell anak (konteks, tidak dihitung ke "tambahan per sesi"): masing-masing sekitar 5,3–6,2 MB RSS (`/bin/zsh -l`).

## Perbandingan dengan target spec §7

| Metrik | Target | Terukur | Rasio | Status |
|---|---|---|---|---|
| Daemon idle (0 sesi) | < 30 MB | 5,88 MB | 0,20x | LULUS |
| Tambahan per sesi (2 sesi) | < ~10 MB | (6,75 − 5,88) / 2 = 0,43 MB | 0,04x | LULUS |
| Tambahan per sesi (3 sesi) | < ~10 MB | (6,91 − 5,88) / 3 = 0,34 MB | 0,03x | LULUS |
| UI satu tab (indikatif) | < 200 MB (untuk 6 sesi) | 187,19 MB (app + WebKit) | 0,94x | LULUS (indikatif) |
| UI 6 sesi | < 200 MB | **ESTIMASI** 189–212 MB, bukan hasil ukur | 0,95–1,06x | TIDAK TERUKUR (estimasi di sekitar batas) |
| CPU idle daemon | < 1,0% | < 0,1% | < 0,1x | LULUS |
| CPU idle app + WebKit | < 1,0% | 0,199% | 0,20x | LULUS |

Perbandingan dengan Orca: tidak dilakukan.

Dasar estimasi UI 6 sesi (label ESTIMASI, bukan hasil ukur): batas bawah = 187,19 MB + 5 sesi tambahan di daemon (sekitar 0,4 MB) ≈ 189 MB, dengan asumsi hanya tab aktif yang punya instance `Terminal` xterm.js (sesuai rencana tiket 06). Batas atas = 187,19 MB + 5 × ~5 MB per instance xterm.js (buffer, DOM, canvas) ≈ 212 MB, dengan asumsi semua tab tetap ter-mount. Angka ~5 MB per instance adalah tebakan konservatif, tidak diukur.

## Keterbatasan

1. Hanya 2–3 sesi PTY yang diukur.
2. Hanya satu tab terminal di UI dan satu instance xterm.js. Apakah tab itu benar-benar tampil dan ter-attach ke sesi di window TIDAK dikonfirmasi secara visual (agent tidak melihat layar). Yang terkonfirmasi: app berjalan, memuat halaman (WebContent aktif), dan daemon memiliki 3 sesi `running` yang dapat di-attach saat mount.
3. Target "UI < 200 MB dengan 6 sesi" tidak diukur langsung, hanya diestimasi (asumsi di atas). Harus diukur ulang setelah tiket 06 (UI shell lengkap, banyak tab), karena estimasi berada di sekitar batas.
4. Sesi berisi shell kosong (`zsh -l`), bukan agent claude/codex sungguhan yang mengeluarkan output deras.
5. RSS menghitung halaman shared (WebKit memakai memori bersama), jadi jumlah RSS bias ke atas dari pemakaian fisik riil. Hanya satu sampel per titik, variansi tidak diketahui.
6. Tidak ada pembanding Orca.
7. Diukur di satu mesin pada satu waktu, dalam kondisi mesin sibuk (load average 8,56; proses lain dan workflow agent aktif).
8. Titik "3 sesi, scrollback penuh" tidak diukur (membutuhkan `attach` interaktif). RSS daemon pada kasus terburuk scrollback (256 KiB × 3) tidak diketahui.
9. Biner yang diukur dibuild dari source `7438829` (tiket 01–04), tidak memuat tiket 05.
10. Interaksi nyata dengan window (mengetik `ls`, menekan "Sesi baru") tidak dilakukan.

## Keputusan gerbang (spec §9b)

- [ ] (a) Semua target lulus atau mendekati: lanjut Wails.
- [x] (b) Satu/dua target meleset tipis (< ~1,5x): lanjut Wails dengan daftar optimasi lalu ukur ulang.
- [ ] (c) UI jauh meleset (> 1,5x target atau CPU idle terus naik): pertimbangkan UI Swift native + SwiftTerm, daemon Go tidak berubah.

**Alasan:** semua baris daemon dan CPU lulus dengan margin besar (rasio 0,03–0,20x). UI satu tab sudah 187,19 MB (0,94x dari 200 MB), dan estimasi 6 sesi berada di 189–212 MB (rasio 0,95–1,06x), yaitu di sekitar batas. Target UI 6 sesi tidak terukur, sehingga keputusan ini sementara dan bukan (a). Tidak ada baris yang mendekati ambang (c) (1,5x).

**Tindak lanjut:**
1. Ukur ulang UI dengan banyak tab setelah tiket 06 selesai, memakai prosedur yang sama dengan 6 sesi nyata.
2. Optimasi yang sudah direncanakan: mount instance `Terminal` hanya untuk tab aktif, kecilkan scrollback xterm.js, tanpa addon WebGL.
3. Selidiki proses `aiterm` sendiri (90,75 MB RSS) sebagai kontributor terbesar tunggal: tentukan berapa yang berasal dari runtime Go + framework Wails dibanding aset frontend yang di-embed.
4. Bila ukuran ulang pasca-tiket 06 melewati 1,5x (300 MB) atau CPU idle terus naik, bawa ke pembuat untuk mempertimbangkan opsi (c).

Keputusan akhir gerbang ada di pembuat; laporan ini hanya rekomendasi berbasis angka.

## Catatan tambahan

- Daemon memakai 8 baris di `ps -M` (termasuk header) saat idle, jumlah thread kecil.
- RSS daemon naik hanya sekitar 1,0 MB (5,88 → 6,91 MB) dari 0 ke 3 sesi, sangat jauh di bawah target; scrollback 256 KiB per sesi baru terisi bila ada output.
- Membuka app tidak mengubah RSS maupun CPU daemon secara berarti (5,88 → 6,59 MB, CPU tetap < 0,1%).
