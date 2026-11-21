# Plan: Spike pengukuran RAM/CPU dan gerbang

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-02-session-pty.md` (epic 02 Session PTY), fitur `spike-measure`.
- Spec: `docs/superpowers/specs/2026-11-21-ai-terminal-design.md` (target biaya §7 "Status sesi dan biaya", fase 0 §9, gerbang §9b).
- Kontrak bersama: `docs/plans/ai-terminal-ticket-02-session-pty/overview.md` (rujuk saja; jangan diubah).
- ownedPaths: `docs/perf/spike-report.md`
- Depends on foundation: tidak (tidak menyentuh kode atau dependency). Tetapi WAJIB dikerjakan TERAKHIR, setelah Foundation, keempat fitur paralel, dan Wiring hijau. Tidak paralel dengan fitur lain.

## Tujuan

Mengukur biaya sumber daya nyata (RSS dan CPU idle) dari daemon `aitermd` dengan 0/2/3 sesi shell dan dari aplikasi Wails (`aiterm` + proses WebKit) dengan satu tab terminal, lalu menulis `docs/perf/spike-report.md` berisi angka hasil ukur sungguhan, perbandingan dengan target spec §7, keterbatasan, dan keputusan gerbang (a/b/c) dengan alasan dan tindak lanjut.

Target spec §7 (semua [call], diverifikasi di sini):
- daemon idle < 30 MB RSS;
- tambahan per sesi < ~10 MB di luar proses agent (di luar proses shell itu sendiri);
- UI < 200 MB dengan 6 sesi;
- CPU idle ~ 0% (lulus bila < 1,0%).

Aturan keras: tidak ada angka karangan, tidak ada placeholder (`TODO`, `XX`, `?`) tersisa di laporan. Bila suatu pengukuran benar-benar tidak bisa dilakukan (mis. window Wails tidak bisa dijalankan di lingkungan agent), tulis itu jujur sebagai "tidak terukur" beserta alasan, dan jangan mengisi angka; lihat langkah 9 untuk penanganannya.

## Path yang dimiliki

HANYA boleh dibuat/diubah: `docs/perf/spike-report.md`.

Dilarang menyentuh: file wiring, `go.mod`/`go.sum`, `package.json`, kode apa pun di `internal/`, `cmd/`, `frontend/`, `app/`, `Makefile`, `overview.md`, path fitur lain. Tidak membuat skrip atau file ukur permanen. File sementara (log ps, skrip bantu satu kali pakai) ditaruh di direktori scratchpad/`mktemp -d` di luar repo dan dihapus sebelum selesai. Data dir uji memakai `AITERM_DATA_DIR` ke direktori sementara supaya data nyata pengguna tidak tersentuh.

## Kontrak

Tidak ada kode yang disediakan. Yang dipakai (rujuk overview bagian 2 dan tiket "Kontrak"):

- Biner hasil `make build`: `build/bin/aitermd`, `build/bin/aitermctl` (bila `make build` menaruhnya di tempat lain, cari lokasi nyata dan catat di laporan), dan `build/bin/aiterm` (app Wails).
- Subcommand CLI: `aitermctl session new [--cwd DIR]` (mencetak ID dan JSON `Session`), `aitermctl session list`, `aitermctl session kill <id>`.
- Method IPC `session.create|list|kill|write|resize|attach`, event `session.output.<id>` dan `session.exit`; tombol UI "Sesi baru" dan "Tutup sesi" di panel `terminal-quick`.
- Env `AITERM_DATA_DIR` (dari ticket-01) untuk memisahkan data dir dan socket `<DataDir>/aitermd.sock`. Bila app Wails tidak membaca env itu dari lingkungan peluncurnya, jalankan dengan `AITERM_DATA_DIR=... build/bin/aiterm` dari shell yang sama dengan daemon.
- Bentuk dokumen keluaran: lihat langkah 8.

## Langkah

Semua perintah dijalankan di root repo. Gunakan `ps` saja (tanpa Instruments/Activity Monitor sebagai sumber angka). Satuan: RSS dari `ps -o rss=` dalam KB, bagi 1024 untuk MB (catat dua desimal).

### 1. Prasyarat dan metadata mesin
1. Pastikan Wiring hijau: `make build` sukses dan `build/bin/` berisi biner di atas. Bila gagal, hentikan dan laporkan (`ok:false`); jangan memperbaiki kode fitur lain.
2. Kumpulkan metadata (simpan keluarannya untuk laporan):
   - `sw_vers`
   - `uname -m`
   - `sysctl -n hw.memsize` (bagi 1024^3 untuk GB)
   - `sysctl -n machdep.cpu.brand_string`
   - `sysctl -n hw.ncpu`
   - `go version`, `git rev-parse --short HEAD` bila repo git (bila bukan git, tulis "bukan repo git"), tanggal `date -u +%FT%TZ`
   - Versi `@xterm/xterm` dari `frontend/package.json`.
3. Tutup aplikasi berat lain sebisanya agar CPU idle tidak terkontaminasi; catat apa yang masih berjalan secara garis besar (`ps -A -o %cpu=,comm= | sort -rn | head -5`).
4. Siapkan lingkungan: `export AITERM_DATA_DIR="$(mktemp -d)"`; pastikan tidak ada `aitermd` lain (`pgrep -x aitermd`).

### 2. Helper pengukuran (ketik di shell, tidak disimpan ke repo)
Definisikan fungsi shell sementara di sesi shell agent:
- `rss_mb PID`: `ps -o rss= -p PID | awk '{printf "%.2f", $1/1024}'`.
- `cputime_s PID`: ambil `ps -o time= -p PID` (format macOS `[[HH:]MM:]SS.cc`, mis. `0:00.12` atau `1:02:03.45`), ubah ke detik dengan awk: pecah pada `:`, hitung `jam*3600 + menit*60 + detik`.
- `cpu_idle_pct PID WAIT`: sampel `t0=$(cputime_s)` dan `w0=$(date +%s.%N)`-setara (macOS `date` tidak punya `%N`; pakai `python3 -c 'import time;print(time.time())'` atau `perl -MTime::HiRes=time -e 'print time'`), `sleep WAIT` (WAIT >= 5, pakai 10), sampel kedua, lalu `delta_cpu / delta_wall * 100`. Catat `delta_cpu`, `delta_wall`, dan hasil persen. JANGAN memakai kolom `%cpu` (rata-rata sejak start) sebagai CPU idle. Karena resolusi `ps -o time=` hanya 0,01 detik, interval 10 detik memberi resolusi ~0,1%; bila hasil 0 tulis "< 0,1%" bukan "0".
- Sampel CPU dilakukan sekali per titik ukur (bukan pemantauan berkelanjutan, bukan loop panjang).

### 3. Baseline daemon (0 sesi)
1. Jalankan `build/bin/aitermd` di background dengan `AITERM_DATA_DIR` tadi; simpan PID (`DPID`). Tunggu socket muncul dengan satu perintah (`until [ -S "$AITERM_DATA_DIR/aitermd.sock" ]; do sleep 0.2; done`, dibatasi timeout beberapa detik) lalu `sleep 5` agar idle.
2. Verifikasi dengan `aitermctl session list` (kosong) bahwa daemon sehat.
3. Catat RSS (`rss_mb $DPID`) dan CPU idle (`cpu_idle_pct $DPID 10`). Catat juga jumlah thread bila mudah (`ps -M -p $DPID | wc -l`) sebagai info tambahan.

### 4. Daemon dengan 2 dan 3 sesi
1. `aitermctl session new` dua kali (catat ID). Biarkan idle 5 detik. Catat RSS daemon dan CPU idle daemon (titik "2 sesi"). Catat juga RSS proses shell anak untuk konteks saja (`ps -o pid,ppid,rss,command -p <pid>` lewat pohon proses: `pgrep -P $DPID`); RSS shell TIDAK dihitung ke "tambahan per sesi" karena target dinyatakan di luar proses agent/shell.
2. Buat sesi ke-3, idle 5 detik, catat RSS dan CPU idle daemon (titik "3 sesi").
3. Hitung tambahan per sesi: `(RSS_3 - RSS_0) / 3` dan `(RSS_2 - RSS_0) / 2`; laporkan keduanya.
4. Variasi bermanfaat dan murah: kirim output ke satu sesi hingga scrollback penuh (mis. `aitermctl session attach <id>` lalu `seq 1 200000`, lepas dengan Ctrl+]) sehingga ring buffer 256 KiB terisi, lalu ukur RSS lagi sebagai titik "3 sesi, scrollback penuh". Karena `attach` interaktif sulit diotomasi dari agent, bila tidak praktis dilewati, tulis di keterbatasan bahwa titik ini tidak diukur. Bila dilakukan, tegaskan bahwa ini kasus terburuk scrollback (256 KiB x 3 = 768 KiB) dan RSS bisa naik tidak linear karena alokasi Go runtime.
5. Biarkan daemon dan 3 sesi hidup untuk langkah 5.

### 5. Aplikasi Wails dengan satu tab terminal
1. Dengan daemon tetap hidup (3 sesi), jalankan `AITERM_DATA_DIR=... build/bin/aiterm &`, simpan PID app (`APID`). Bila agent tidak dapat membuka window GUI (lingkungan headless/tanpa izin), jangan memalsukan: tandai langkah ini "tidak terukur", lihat langkah 9.
2. Di window, pastikan satu tab terminal tampil (panel `terminal-quick` meng-attach sesi `running` terbaru saat mount, atau tekan "Sesi baru"). Ketik `ls` untuk memastikan hidup. Tunggu 10 detik idle tanpa menyentuh window.
3. Identifikasi pohon proses app: `ps -axo pid,ppid,rss,command | grep -E 'aiterm|com.apple.WebKit'`. Proses WebKit milik app diidentifikasi lewat hubungan induk/anak dengan `APID` (anak langsung) bila terlihat; di macOS proses `com.apple.WebKit.WebContent`/`com.apple.WebKit.Networking` sering di-spawn oleh launchd/XPC sehingga ppid-nya BUKAN `APID`. Cara menentukan milik app tanpa menebak: bandingkan daftar PID `com.apple.WebKit.*` SEBELUM dan SESUDAH app dijalankan (ambil daftar sebelum langkah 5.1), dan hanya hitung PID yang baru muncul; catat bahwa proses WebKit yang dipakai bersama aplikasi lain (mis. Safari) tidak dihitung. Tuliskan metode identifikasi yang dipakai di laporan.
4. Catat RSS tiap proses (`aiterm`, WebContent, Networking, GPU bila ada) dan jumlahnya. Catat catatan bahwa RSS menghitung halaman bersama (shared) sehingga penjumlahan bisa melebihi pemakaian fisik riil; sebut ini sebagai bias ke atas.
5. CPU idle: untuk tiap proses di atas ukur `cpu_idle_pct PID 10` dan jumlahkan; jangan menyentuh window selama interval. Catat juga CPU idle daemon pada kondisi app terbuka.
6. Opsional (hanya bila murah): tutup window, tunggu, buka lagi, pastikan sesi tetap `running` (sudah bagian acceptance ticket; bukan keluaran spike).

### 6. Pembersihan
- Tutup app Wails; `aitermctl session kill <id>` untuk semua sesi; hentikan daemon (`kill $DPID`); hapus `AITERM_DATA_DIR` sementara dan file sementara lain; pastikan `pgrep -x aitermd` kosong. Pastikan hanya `docs/perf/spike-report.md` yang berubah di repo (bandingkan dengan daftar file sebelum mulai, mis. `find . -newer <penanda> -type f -not -path './node_modules/*' -not -path './frontend/node_modules/*'` atau `git status --porcelain` bila repo git).

### 7. Evaluasi terhadap target
Hitung status per baris (LULUS / MELESET TIPIS / MELESET JAUH / TIDAK TERUKUR):
- Daemon idle (0 sesi): LULUS bila < 30 MB.
- Tambahan per sesi: LULUS bila < ~10 MB (pakai nilai terburuk dari dua perhitungan di langkah 4.3).
- UI (satu tab): bandingkan jumlah RSS app + WebKit dengan 200 MB. Ini indikatif karena target untuk 6 sesi; ekstrapolasi eksplisit (lihat keterbatasan): UI_6 ~ UI_1tab + 5 x (tambahan per tab yang dapat diperkirakan). Karena hanya satu tab dan hanya satu instance `Terminal` xterm.js yang ada, per-tab tidak terukur; tuliskan asumsi ekstrapolasi (mis. perkiraan konservatif memakai tambahan per sesi daemon sebagai proksi untuk scrollback sisi UI, ditambah catatan bahwa xterm.js menyimpan buffer sendiri ~ ribuan baris) sebagai ESTIMASI yang diberi label jelas, bukan hasil ukur, dan beri rentang (bawah-atas) bukan satu angka.
- CPU idle: LULUS bila delta persen < 1,0% untuk daemon dan untuk jumlah proses app+WebKit; tambah catatan bila tampak naik antar sampel.
Rasio meleset = angka terukur / target.

### 8. Tulis `docs/perf/spike-report.md`
Buat dengan Write tool (satu kali, atau Write ulang bila perlu; bukan heredoc). Bahasa Indonesia, identifier bahasa Inggris. Kerangka isi (urutan wajib):

1. `# Laporan Spike Pengukuran RAM/CPU (Epic 02 Session PTY)`; ringkasan 3-5 baris: apa yang diukur, hasil utama, keputusan gerbang terpilih.
2. `## Metadata mesin dan build`: tabel/daftar hasil langkah 1.2 dan catatan lingkungan (aplikasi lain yang berjalan, tanggal, commit bila ada, lokasi biner).
3. `## Metode dan perintah`: perintah persis yang dipakai (`ps -o rss= -p`, `ps -o time= -p`, rumus `delta_cpu / delta_wall * 100`, interval sampel dalam detik, jumlah sampel per titik = 1, metode identifikasi proses WebKit, `AITERM_DATA_DIR`), serta alasan tidak memakai kolom `%cpu`.
4. `## Angka mentah`: tabel per titik ukur: titik, PID, RSS (KB), RSS (MB), CPU time awal, CPU time akhir, delta wall, CPU idle (%). Isi hanya dari hasil nyata. Sertakan baris per proses untuk app dan WebKit.
5. `## Perbandingan dengan target spec §7`: tabel `Metrik | Target | Terukur | Rasio | Status (lulus/meleset)`: daemon idle (0 sesi), tambahan per sesi (2 dan 3 sesi), UI (satu tab; indikatif), UI 6 sesi (ESTIMASI, rentang, dilabeli bukan hasil ukur), CPU idle daemon, CPU idle app+WebKit. Perbandingan dengan Orca opsional dan manual: bila tidak dilakukan, tulis satu kalimat "tidak dilakukan" (bukan angka).
6. `## Keterbatasan`: wajib memuat (a) hanya 2-3 sesi PTY, (b) hanya satu tab terminal di UI dan satu instance xterm.js, (c) target "UI < 200 MB dengan 6 sesi" tidak diukur langsung dan hanya diekstrapolasi (jelaskan asumsi) serta diulang setelah ticket-06 (UI shell lengkap, banyak tab) bila hasil mendekati batas, (d) sesi berisi shell kosong, bukan agent claude/codex yang sebenarnya, (e) RSS menghitung halaman shared dan memakai satu sampel per titik (variansi tidak diketahui), (f) tidak ada pembanding Orca bila tidak dilakukan, (g) pengukuran di satu mesin dan satu waktu, (h) titik yang gagal diukur, bila ada.
7. `## Keputusan gerbang (spec §9b)`: checklist tiga opsi dengan TEPAT SATU dicentang `[x]`, dua lainnya `[ ]`:
   - `(a)` semua target lulus atau mendekati: lanjut Wails.
   - `(b)` satu/dua target meleset tipis (< ~1,5x): lanjut Wails dengan daftar optimasi lalu ukur ulang.
   - `(c)` UI jauh meleset (> 1,5x target atau CPU idle terus naik): pertimbangkan UI Swift native + SwiftTerm, daemon Go tidak berubah.
   Setelah checklist: `Alasan` (merujuk baris tabel dan rasio konkret) dan `Tindak lanjut` (daftar konkret: mis. untuk (b) buang instance `Terminal` tab tak aktif, kecilkan `ScrollbackSize`, kurangi riwayat xterm; tentukan kapan ukur ulang, yaitu setelah ticket-06; untuk (a) catat bahwa pengukuran 6 sesi tetap diulang setelah ticket-06; untuk (c) langkah berikut untuk keputusan manusia). Tegaskan bahwa keputusan akhir gerbang ada di pembuat, laporan ini hanya rekomendasi berbasis angka.
8. `## Catatan tambahan` (opsional): temuan menarik (mis. thread count, perilaku Go GC) tanpa angka tak terukur.

Aturan pemilihan opsi: gunakan rasio terukur dan hasil langkah 7 secara mekanis, bukan perasaan. Bila ada baris UI "TIDAK TERUKUR" dan semua baris daemon lulus, pilih berdasarkan angka yang ada dan nyatakan di alasan bahwa keputusan bersifat sementara sampai UI terukur (biasanya (b) dengan tindak lanjut "ukur ulang UI"), bukan (a) diam-diam.

### 9. Bila sebagian pengukuran tidak dapat dilakukan
- Daemon (langkah 3-4) harus bisa diukur di lingkungan agent; bila tidak, laporkan `ok:false`.
- Bila hanya window Wails yang tidak bisa dibuka/dijangkau (headless), tetap tulis laporan dengan baris UI bertanda "tidak terukur" beserta alasan dan cara mengulang (perintah persis langkah 5), pilih gerbang sesuai aturan di langkah 8, dan set `needsHumanInput=true` dengan alasan "pengukuran UI perlu dilakukan manual oleh manusia". Jangan mengarang angka UI.

### 10. Verifikasi akhir
1. Baca ulang `docs/perf/spike-report.md` dengan Read tool: pastikan semua bagian ada, tidak ada placeholder, angka di tabel konsisten dengan keluaran ukur, tepat satu opsi gerbang tercentang, keterbatasan memuat 2-3 sesi, satu tab, dan ekstrapolasi 6 sesi.
2. Pastikan tidak ada file lain berubah dan tidak ada proses `aitermd`/`aiterm` tersisa.
3. `make build` tetap hijau (hanya dijalankan sebagai konfirmasi; tidak ada perubahan kode).

## Perubahan

- Dibuat: `docs/perf/spike-report.md` (satu-satunya file; `docs/perf/` sudah dibuat Foundation, buat bila belum ada). `docs/perf/.gitkeep` dari Foundation boleh dibiarkan.
- Tidak ada file lain yang dibuat atau diubah.

## Wiring

Tidak ada. Fitur ini tidak mengekspos apa pun untuk disambungkan dan tidak boleh menyentuh file wiring. Keluarannya hanya dokumen yang dibaca manusia untuk memutuskan gerbang spec §9b.

## Selesai bila

- `docs/perf/spike-report.md` ada dan memuat metadata mesin, perintah ukur, dan tabel angka hasil pengukuran sungguhan (tanpa angka karangan atau placeholder).
- Tabel membandingkan daemon idle, tambahan per sesi, UI, dan CPU idle terhadap target spec §7 dengan status lulus/meleset per baris; CPU idle dihitung dari delta CPU time (bukan `%cpu`), interval sampel >= 5 detik.
- Keterbatasan tertulis: 2-3 sesi, satu tab UI, ekstrapolasi (berlabel estimasi) ke 6 sesi, dan pengulangan setelah ticket-06 bila mendekati batas.
- Keputusan gerbang dinyatakan eksplisit: tepat satu pilihan (a/b/c) tercentang, dengan alasan berbasis angka dan tindak lanjut.
- Hanya `docs/perf/spike-report.md` yang dibuat/diubah; proses dan data dir sementara dibersihkan; `make build` tetap hijau (guard fitur ini: dokumen, bukan build path).
