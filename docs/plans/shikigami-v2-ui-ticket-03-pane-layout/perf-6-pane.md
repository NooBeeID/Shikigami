# Plan: Ukur ulang RAM/CPU dengan 6 pane terlihat

## Metadata

- Tiket: `shikigami-v2-ui-ticket-03-pane-layout` (`docs/tickets/shikigami-v2-ui-ticket-03-pane-layout.md`)
- Fitur: `perf-6-pane` (Wiring, dikerjakan TERAKHIR setelah `layout-wiring` tergabung; butuh aplikasi yang jalan)
- ownedPaths: `docs/perf/multipane-report.md`
- Depends on foundation: ya (secara tidak langsung). Foundation `layout-model` dan semua fitur lain (`layout-renderer`, `pane-header-dnd`, `pane-kinds`, `layout-wiring`) harus sudah tergabung agar `build/bin/shikigami` menampilkan grid pane. Fitur ini tidak mengimpor kode apa pun.

## Tujuan

Mengukur RAM (RSS) dan CPU idle UI Shikigami dengan 6 pane terminal terlihat bersamaan (grid 3x2 lewat "Susun grid"), memakai prosedur yang sama dengan `docs/perf/spike-report.md`, lalu menulis `docs/perf/multipane-report.md` (bahasa Indonesia) sebagai gerbang spec bagian 8: bandingkan dengan target 200 MB, ambang 1,5x (300 MB), dan baseline v1 187,19 MB. Laporan HANYA melaporkan angka dan rekomendasi data; keputusan opsi UI native (spec v1 bagian 9b opsi c) tetap di pembuat. Tidak ada kode yang diubah.

## Path yang dimiliki

HANYA boleh dibuat/ditulis: `docs/perf/multipane-report.md`.

Dilarang menyentuh: `docs/perf/spike-report.md` (hanya dibaca), `overview.md`, plan lain, semua kode `frontend/`, `app/`, `internal/`, `cmd/`, `Makefile`, `package.json`, `App.svelte`, `modules/*`. Dilarang menambah kode, skrip, atau berkas ukur ke repo. Skrip ukur ditulis di scratchpad (di luar repo) atau berupa perintah shell sekali pakai. Bila `make build` mengubah artefak build (`build/`, `frontend/dist`, `app/dist`), itu keluaran build biasa dan boleh; jangan edit sumber. Data dir pengguna nyata (default daemon) TIDAK boleh disentuh.

## Kontrak

Dipakai (tidak disediakan, lihat overview bagian 2 dan 6):
- Hasil build `make build`: `build/bin/shikigamid` (daemon), `build/bin/shikigamictl` (CLI), `build/bin/shikigami` (app Wails).
- Variabel lingkungan `SHIKIGAMI_DATA_DIR` (`internal/paths/paths.go`, konstanta `EnvDataDir`): arahkan ke direktori sementara dari `mktemp -d`. PERHATIAN: path socket Unix dibatasi panjangnya; pakai `mktemp -d /tmp/shk-perf.XXXXXX` (pendek), bukan path panjang di scratchpad, kecuali panjang socket masih muat (pesan galat daemon: "path socket terlalu panjang"). Pengecualian aturan scratchpad ini sah karena batas panjang socket; hapus direktorinya di akhir.
- CLI: `shikigamictl session new` (provider `shell`; cek `shikigamictl --help` / `session new --help` untuk flag sebenarnya, mis. provider dan cwd; baca `cmd/shikigamictl` bila perlu, tanpa mengubah). Perintah daemon: `shikigamid`.
- UI: aksi "Susun grid" (menu `...` pada header pane, `tidyGrid`: 6 pane menjadi 3x2) dan klik sesi di sidebar yang membuka pane lewat shim tab (layout-wiring). Mitigasi aktif yang harus dikonfirmasi lalu dinyatakan di laporan: `scrollback: 1000` pada pane terminal, tanpa addon WebGL (`grep -rni webgl frontend/src/modules/terminal frontend/src/modules/layout` kosong), pane tersembunyi `visible=false` tidak merender.
- Prosedur ukur (dari `docs/perf/spike-report.md`, wajib sama): RSS `ps -o rss= -p PID` (KB, bagi 1024 untuk MB); hanya PID `com.apple.WebKit*` BARU milik app (bandingkan daftar sebelum/sesudah app jalan); jeda 5 detik setelah perubahan keadaan; CPU idle = selisih `ps -o time= -p PID` (ke detik) dibagi selisih waktu dinding, dikali 100, interval 10 detik, satu sampel per titik; JANGAN pakai kolom `%cpu`; resolusi 0,01 s, nilai 0 ditulis "< 0,1%".
- Angka acuan: target 200 MB, ambang 300 MB (1,5x), baseline v1 187,19 MB (1 tab; app 90,75 + GPU 32,98 + WebContent 48,73 + Networking 14,72).

Disediakan: hanya berkas laporan `docs/perf/multipane-report.md`. Tidak ada API.

## Langkah

1. **Baca konteks (tanpa ubah).** Baca `docs/perf/spike-report.md` penuh (struktur laporan, tabel, bagian keterbatasan, keputusan gerbang) dan bagian 8 spec. Catat commit sekarang (`git rev-parse --short HEAD`, bila repo git; bila bukan git, tulis "tidak ada repo git" dan sebut waktu build).
2. **Prasyarat.** Pastikan `layout-wiring` sudah ada: `grep -n "LayoutArea" frontend/src/App.svelte` ada isi, dan berkas `frontend/src/modules/layout/LayoutArea.svelte`, `PaneHeader.svelte`, `PaneMenu.svelte` ada tanpa teks `STUB(foundation)`. Bila belum, hentikan dan laporkan blocker (`ok:false`, alasan); jangan menulis laporan dari angka karangan.
3. **Metadata mesin.** Kumpulkan: `sw_vers`, `uname -m`, `sysctl -n machdep.cpu.brand_string`, `sysctl -n hw.ncpu`, RAM (`sysctl -n hw.memsize`), `go version`, versi xterm dari `frontend/package.json` (`@xterm/xterm`, `@xterm/addon-fit`), waktu ukur (UTC + WIB), commit.
4. **Kondisi lingkungan.** Catat `uptime` (load average) dan 5 proses CPU teratas (`ps -Aro pid,comm,time | head` atau `top -l 1 -o cpu -n 5`) sebelum dan sesudah ukur; sebutkan jika mesin tidak idle dan potensi kontaminasi.
5. **Build.** Jalankan `make build` dari root repo (mencakup `go build` dan build frontend + type-check). Catat waktu build dan bila gagal, hentikan dan laporkan. Jalankan `grep -rni webgl frontend/src/modules/terminal frontend/src/modules/layout` dan `grep -rn "scrollback" frontend/src/modules/layout frontend/src/modules/terminal` untuk mengonfirmasi mitigasi (scrollback 1000 di pane terminal, tanpa WebGL).
6. **Siapkan lingkungan sementara.** Salin tiga biner ke direktori sementara (`mktemp -d /tmp/shk-perf-bin.XXXXXX`) dan pakai salinan itu (seperti spike). `export SHIKIGAMI_DATA_DIR=$(mktemp -d /tmp/shk-perf.XXXXXX)`. Pastikan `pgrep -x shikigami` dan `pgrep -x shikigamid` kosong sebelum mulai; bila ada proses milik pengguna yang sedang jalan, JANGAN dimatikan: gunakan data dir sementara sehingga daemon terpisah, dan catat PID baru saja yang dihitung. Bila app tidak bisa berjalan berdampingan dengan instance pengguna (mis. single-instance lock), catat sebagai keterbatasan dan minta pengguna menutupnya via `needsHumanInput` hanya bila benar-benar buntu.
7. **Daftar WebKit sebelum.** Simpan `pgrep -f com.apple.WebKit` (PID + nama) sebagai daftar "sebelum".
8. **Jalankan daemon** (`SHIKIGAMI_DATA_DIR=... shikigamid &`, atau biarkan app memulainya bila itu perilaku v1; cek `app/` read-only) lalu buat 6 sesi shell: `shikigamictl session new` enam kali (provider `shell`, cwd direktori sementara atau `$HOME`; sama seperti spike). Verifikasi `shikigamictl session list` menampilkan 6 `running`.
9. **Titik A: UI 1 pane.** Jalankan app (`build/bin/shikigami` salinan) dengan data dir yang sama. Jeda 5 detik. Bandingkan daftar WebKit untuk mendapat PID baru (GPU, WebContent, Networking). Buka tepat 1 pane (klik 1 sesi di sidebar; bila perlu tutup pane lain dengan "Tutup pane", yang tidak membunuh sesi). Jeda 5 detik, lalu ukur: untuk tiap PID (app + 3 WebKit baru) ambil `ps -o rss=` dan `ps -o time=` awal, tunggu 10 detik, ambil `ps -o time=` akhir, hitung delta wall dari timestamp presisi (`python3 -c 'import time;print(time.time())'` atau `gdate`/`perl -MTime::HiRes`).
10. **Titik B: 6 pane idle.** Buka kelima sesi lain (klik di sidebar), lalu "Susun grid" lewat menu `...` pada header pane sehingga tersusun 3x2. Jeda 5 detik, ukur dengan prosedur yang sama. Konfirmasi enam pane terlihat dan grid 3x2: ambil tangkapan layar (`screencapture -x` ke scratchpad, hanya untuk verifikasi, bukan masuk repo) dan lihat dengan Read; bila ada, sebutkan di keterbatasan bahwa verifikasi visual dilakukan.
11. **Menggerakkan UI.** Interaksi klik dilakukan dengan `osascript` (System Events, butuh izin Accessibility) atau `cliclick` bila tersedia; skill `computer-use`/browser tidak perlu. Bila otomasi UI tidak mungkin (izin ditolak, tidak ada akses layar), JANGAN mengubah kode sumber untuk memicu aksi dan JANGAN mengarang angka. Fallback bertahap: (a) minta pengguna lewat `needsHumanInput: true` (alasan konkret: perlu klik manual "Susun grid" dan memberi izin Accessibility) hanya bila sungguh buntu; (b) jika pengguna tidak dapat, ukur apa yang bisa (titik 1 pane) dan tulis titik 6 pane sebagai "TIDAK TERUKUR" dengan estimasi berlabel ESTIMASI beserta dasarnya, dan status gerbang "TIDAK TERUKUR". Acceptance menuntut angka nyata, jadi fallback ini hanya untuk keadaan tidak ada jalan lain.
12. **Titik C: 6 pane dengan output ringan.** Kirim beban ke tiap sesi tanpa mengubah layout, mis. lewat `shikigamictl` bila ada perintah input/send (cek `--help`), atau `osascript` mengetik `yes | head -n 20000` + Enter di tiap pane (klik pane lalu ketik). Tunggu output selesai, jeda 5 detik sampai idle lagi, ukur dengan prosedur yang sama. Catat juga ukuran daemon (`shikigamid` RSS dan CPU) pada titik B dan C sebagai konteks (bukan bagian jumlah UI).
13. **Titik tambahan (opsional, bila murah):** RSS daemon dengan 6 sesi idle, dan 6 pane dengan satu pane dimaksimalkan (pane lain `visible=false`) untuk menunjukkan efek mitigasi "tersembunyi tidak merender". Beri label jelas sebagai titik tambahan di luar tiga titik wajib.
14. **Hitung.** Jumlah RSS UI = app + tiga WebKit baru, per titik (KB dan MB 2 desimal, koma sebagai pemisah desimal di laporan). CPU idle UI = jumlah CPU idle per proses. Rasio terhadap 200 MB, 300 MB, dan baseline 187,19 MB (`MB / referensi`, 2 desimal). Status: LULUS bila <= 200 MB; "LEWAT target 200 MB, di bawah ambang 300 MB" bila 200 sampai 300 MB; "LEWAT ambang 300 MB" bila > 300 MB. Untuk CPU idle, target spike < 1,0%.
15. **Tulis `docs/perf/multipane-report.md`** (Write tool, bahasa Indonesia, angka koma desimal seperti spike) dengan bagian berurutan:
    1. `# Laporan Pengukuran RAM/CPU Multi-Pane (6 Pane Terlihat)` + paragraf **Ringkasan** (angka kunci: UI 1 pane, 6 pane idle, 6 pane output ringan; rasio terhadap 200/300 MB dan baseline; status gerbang).
    2. `## Metadata mesin dan build`: tabel OS, arsitektur/chip/core, RAM, Go, xterm/addon-fit, waktu ukur, commit, sumber dan waktu build biner, lokasi biner (salinan sementara), data dir (`SHIKIGAMI_DATA_DIR` sementara, data nyata tidak tersentuh).
    3. `## Kondisi lingkungan`: load average, proses berat lain, kemungkinan kontaminasi.
    4. `## Konfigurasi mitigasi aktif`: scrollback 1000, tanpa WebGL (hasil grep), pane tersembunyi tidak merender (instance ditahan), pane dibuat dengan "Susun grid" 3x2; sebut apa yang dikonfirmasi lewat kode/grep versus lewat pengamatan.
    5. `## Metode dan perintah`: perintah sebenarnya yang dijalankan (RSS, CPU idle, deteksi PID WebKit baru, jeda 5 detik, interval 10 detik, tanpa `%cpu`, cara membuat sesi, cara memicu output ringan `yes | head -n 20000`) dan **perbedaan dari spike v1** (6 sesi bukan 2-3, 6 pane tampil, nama biner baru `shikigami*`, sumber build, metode memicu aksi UI), atau "tidak ada perbedaan" bila benar.
    6. `## Angka mentah`: tabel kolom sama seperti spike (Titik, Proses, PID, RSS KB, RSS MB, CPU time awal s, CPU time akhir s, Delta wall s, CPU idle %) untuk titik A, B, C (app + tiap WebKit + baris **jumlah** tebal) plus baris daemon.
    7. `## Perbandingan dengan target`: tabel (Metrik, Referensi, Terukur, Rasio, Status) untuk setiap titik terhadap 200 MB, 300 MB, dan baseline 187,19 MB, ditambah baris CPU idle terhadap 1,0% dan selisih (delta MB per pane tambahan = (B - A) / 5). Angka perkiraan wajib berlabel ESTIMASI.
    8. `## Keterbatasan`: satu sampel per titik, variansi tidak diketahui; RSS menghitung halaman shared (bias ke atas); mesin tidak idle bila itu kasusnya; shell kosong/output sintetis bukan agent nyata; verifikasi visual atau tidak; otomasi UI; 1 mesin; scrollback penuh pada 6 pane hanya sebagian diuji lewat output 20000 baris; dsb., hanya yang benar.
    9. `## Kesimpulan gerbang`: kotak centang gaya spike atau pernyataan setara (bandingkan hasil dengan 200 MB dan 300 MB). Bila hasil titik mana pun > 300 MB, tulis jelas: "Melewati ambang 1,5x; sesuai spec bagian 8, keputusan opsi UI native (spec v1 bagian 9b opsi c) dibawa ke pembuat." Bila <= 300 MB, tulis bahwa ambang gerbang tidak terlewati. DILARANG menulis keputusan memilih/menolak opsi UI native; hanya rekomendasi data (mis. daftar tindak lanjut optimasi berbasis angka). Tutup dengan kalimat bahwa keputusan akhir ada di pembuat.
16. **Bersihkan** (wajib, jalankan juga bila langkah sebelumnya gagal): hentikan hanya proses yang Anda mulai (`kill` PID app dan daemon yang dicatat; jangan `pkill` massal yang bisa mematikan instance pengguna), tunggu, lalu `pgrep -x shikigami` dan `pgrep -x shikigamid` untuk memastikan tidak ada proses sisa milik Anda; `rm -rf` direktori `SHIKIGAMI_DATA_DIR` sementara dan direktori biner salinan (hanya path hasil `mktemp` Anda sendiri, periksa variabelnya tidak kosong sebelum `rm -rf`); hapus tangkapan layar dan skrip sementara di scratchpad. Tulis di laporan (Metode) bahwa pembersihan dilakukan dan hasil verifikasinya.
17. **Baca ulang** `docs/perf/multipane-report.md` dengan Read: pastikan semua 9 bagian ada, angka tabel konsisten dengan hasil mentah (jumlah dan rasio dihitung ulang), tidak ada keputusan opsi UI native, label ESTIMASI di tempat yang tepat, bahasa Indonesia. Perbaiki bila perlu.

## Perubahan

- Dibuat: `docs/perf/multipane-report.md`.
- Tidak ada berkas lain yang dibuat atau diubah di repo (artefak keluaran `make build` seperti `build/bin/*`, `frontend/dist`, `app/dist` bukan perubahan sumber).

## Wiring

Tidak ada. Fitur ini tidak mengekspos API atau titik ekstensi dan tidak menyambungkan apa pun. Ia adalah langkah ke-8 checklist wiring overview (bagian 5) dan hanya menghasilkan dokumen laporan yang dirujuk oleh tiket.

## Selesai bila

- `docs/perf/multipane-report.md` ada dengan semua bagian: ringkasan, metadata mesin dan build, kondisi lingkungan, konfigurasi mitigasi, metode dan perintah (dengan perbedaan dari spike), tabel data mentah, tabel pembanding, keterbatasan, kesimpulan gerbang.
- Angka adalah hasil ukur nyata untuk 1 pane, 6 pane idle, dan 6 pane dengan output ringan; angka perkiraan (bila ada) berlabel ESTIMASI.
- Tabel pembanding memuat rasio dan status LULUS/LEWAT terhadap 200 MB, 300 MB, dan baseline 187,19 MB.
- Laporan menyatakan mitigasi aktif (scrollback 1000, tanpa WebGL, pane tersembunyi tidak merender).
- Tidak ada keputusan opsi UI native; bila > 300 MB hanya dinyatakan dibawa ke pembuat.
- Proses dan direktori sementara dibersihkan dan diverifikasi; data dir pengguna nyata tidak tersentuh; tidak ada berkas lain di repo yang berubah (kecuali keluaran build).
- Guard: `make build` sukses (dijalankan pada langkah 5) dan `grep -rni webgl frontend/src/modules/terminal frontend/src/modules/layout` tidak menemukan import addon WebGL. Tanpa unit test.
