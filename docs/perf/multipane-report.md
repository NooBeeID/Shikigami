# Laporan Pengukuran RAM/CPU Multi-Pane

Ringkasan: dengan **1 pane shell** UI memakai 193,47 MB (CPU idle < 0,1%). Dengan **7 pane terlihat** (1 shell + 6 TUI Claude Code asli) UI memakai **330,58 MB** idle dan **338,30 MB** setelah output, yaitu 1,65x–1,69x dari target 200 MB dan **melewati ambang 300 MB (1,5x)**. CPU tidak idle: 20,6% (idle) dan 13,5% (setelah output) untuk app + WebKit. Menurut spec bagian 8, keputusan opsi UI native dibawa ke pembuat. Penting: skenario yang terukur BUKAN "6 shell kosong", melainkan 6 TUI Claude Code yang menggambar ulang; lihat bagian Keterbatasan.

## Metadata mesin dan build

| Item | Nilai |
|---|---|
| OS | macOS 15.7.7 (24G720) |
| Arsitektur | arm64, Apple M3, 8 core |
| RAM | 16,00 GB |
| Go | go1.26.3 darwin/arm64 |
| xterm | `@xterm/xterm` ^6.0.0, `@xterm/addon-fit` ^0.11.0 |
| Waktu ukur | 2026-11-21T19:05Z (02:05–02:09 WIB, 2026-11-22) |
| Commit saat ukur | `ba89023` |
| Biner | `build/bin/*` dibuild 01:51:23 dari `c701824` (tiket v2-03 ter-merge); diukur dari salinan sementara |
| Data dir | `SHIKIGAMI_DATA_DIR` sementara; data nyata dan daemon pengguna (PID 28415) tidak disentuh |

## Kondisi lingkungan

Load average di awal 2,42 (lebih tenang dibanding percobaan sebelumnya). Proses lain saat mulai: Antigravity IDE helper (~100% CPU), Lark renderer (~80%), sesi `claude` lain, Orca. CPU per proses diukur dari CPU time proses itu sendiri, tetapi kontaminasi tidak bisa dikesampingkan.

## Konfigurasi mitigasi aktif (dikonfirmasi dari kode)

- `scrollback={1000}` pada pane terminal (`modules/layout/panes/TerminalPane.svelte`).
- Tanpa addon WebGL: `grep -rniE webgl` pada `modules/terminal` dan `modules/layout` menghasilkan 0 baris, jadi renderer xterm adalah DOM.
- Pane tersembunyi tidak merender: tidak teruji, karena semua pane terlihat.

## Metode dan perintah

Sama dengan `docs/perf/spike-report.md`: RSS dari `ps -o rss= -p PID`; CPU idle dari selisih `ps -o time= -p PID` dibagi selisih waktu dinding dikali 100, interval 10 detik, satu sampel per titik; kolom `%cpu` tidak dipakai; PID WebKit milik app ditentukan dengan membandingkan daftar `com.apple.WebKit*` sebelum dan sesudah app dijalankan (3 PID baru: GPU, WebContent, Networking). Jeda 5 detik setelah perubahan keadaan.

Cara membuat pane: workspace, project, dan satu sesi shell dibuat lewat `shikigamictl`; app dijalankan; Cmd+D (bagi kanan) dan Cmd+Shift+D (bagi bawah) dikirim selang-seling lewat `osascript` setelah memastikan Shikigami menjadi aplikasi terdepan, dan hasil tiap tombol diverifikasi dari jumlah sesi di daemon. Titik C: `yes | head -n 2000` dikirim lewat `session.write` ke sesi **shell** saja (sesi Claude tidak diketik). Perbedaan dari spike v1: biner `shikigami*`, layout multi-pane, dan sebagian pane berisi Claude Code, bukan shell kosong.

Pembersihan: hanya PID app, daemon, dan 7 proses anak milik pengukuran ini yang dihentikan (semuanya terverifikasi berhenti); daemon pengguna tetap hidup; folder sementara dihapus.

## Angka mentah

| Titik | Proses | PID | RSS (KB) | RSS (MB) | CPU awal (s) | CPU akhir (s) | Delta wall (s) | CPU idle (%) |
|---|---|---|---|---|---|---|---|---|
| A, 1 pane shell | shikigami | 17233 | 94288 | 92,08 | 0,43 | 0,43 | 10,037 | < 0,1 |
| A, 1 pane shell | WebKit.GPU | 17497 | 38288 | 37,39 | 0,11 | 0,11 | 10,037 | < 0,1 |
| A, 1 pane shell | WebKit.WebContent | 17498 | 50240 | 49,06 | 0,29 | 0,29 | 10,037 | < 0,1 |
| A, 1 pane shell | WebKit.Networking | 17499 | 15296 | 14,94 | 0,03 | 0,03 | 10,037 | < 0,1 |
| **A (jumlah)** | | | **198112** | **193,47** | | | | **< 0,1** |
| B, 7 pane idle | shikigami | 17233 | 107616 | 105,09 | 3,84 | 4,23 | 10,025 | 3,890 |
| B, 7 pane idle | WebKit.GPU | 17497 | 48416 | 47,28 | 2,42 | 2,96 | 10,025 | 5,387 |
| B, 7 pane idle | WebKit.WebContent | 17498 | 167808 | 163,88 | 7,23 | 8,37 | 10,025 | 11,372 |
| B, 7 pane idle | WebKit.Networking | 17499 | 14672 | 14,33 | 0,05 | 0,05 | 10,025 | < 0,1 |
| **B (jumlah)** | | | **338512** | **330,58** | | | | **20,649** |
| C, 7 pane + output | shikigami | 17233 | 108432 | 105,89 | 5,05 | 5,32 | 10,019 | 2,695 |
| C, 7 pane + output | WebKit.GPU | 17497 | 48736 | 47,59 | 4,06 | 4,39 | 10,019 | 3,294 |
| C, 7 pane + output | WebKit.WebContent | 17498 | 174576 | 170,48 | 10,73 | 11,48 | 10,019 | 7,486 |
| C, 7 pane + output | WebKit.Networking | 17499 | 14672 | 14,33 | 0,05 | 0,05 | 10,019 | < 0,1 |
| **C (jumlah)** | | | **346416** | **338,30** | | | | **13,475** |

Daemon (konteks, bukan bagian jumlah UI): A, 1 sesi: 11,59 MB, < 0,1%. B, 8 sesi (1 shell + 7 Claude): 16,41 MB, 1,198%. C: 18,47 MB, 0,999%.

Pengukuran 1 pane sebelumnya (percobaan awal hari yang sama, 6 sesi di sidebar, mesin lebih sibuk) menghasilkan 116,94 MB. Selisih dengan 193,47 MB menunjukkan **variansi antar-ukur yang besar**; satu sampel per titik tidak cukup untuk menyimpulkan angka tunggal.

## Perbandingan dengan target

| Metrik | Referensi | Terukur | Rasio | Status |
|---|---|---|---|---|
| A, 1 pane | 200 MB | 193,47 MB | 0,97x | LULUS (tipis) |
| B, 7 pane idle | 200 MB | 330,58 MB | 1,65x | LEWAT target |
| B | ambang 300 MB | 330,58 MB | 1,10x | **LEWAT ambang** |
| B | baseline v1 187,19 MB | 330,58 MB | 1,77x | lebih tinggi |
| C, 7 pane + output | 200 MB | 338,30 MB | 1,69x | LEWAT target |
| C | ambang 300 MB | 338,30 MB | 1,13x | **LEWAT ambang** |
| Tambahan memori per pane | (B − A) / 6 pane | 22,85 MB per pane | n/a | informasi |
| CPU idle app + WebKit, B | 1,0% | 20,649% | 20,6x | LEWAT |
| CPU idle app + WebKit, C | 1,0% | 13,475% | 13,5x | LEWAT |

## Keterbatasan

1. **Skenario bukan 6 shell idle.** B dan C terukur dengan 7 pane: 1 shell + 6 TUI Claude Code asli (pane 2 sampai 7). Dua sesi Claude pertama dibuat dari klik di window; Cmd+D lalu membagi pane yang sedang fokus dan mewarisi provider-nya. CPU 13–20% kemungkinan besar berasal dari TUI Claude yang menggambar ulang (animasi, status line), bukan dari idle sebenarnya; penyebab persisnya tidak diukur dan tidak dibuktikan.
2. Titik C hanya memberi output ke 1 sesi shell, bukan ke semua pane.
3. Satu sampel per titik; variansi tidak diketahui dan terbukti besar (lihat 116,94 MB vs 193,47 MB untuk 1 pane).
4. RSS menghitung halaman shared (bias ke atas).
5. Mesin tidak sepenuhnya idle (proses lain aktif).
6. Verifikasi manual UI lain di plan tidak dijalankan: drag header ke tepi dan tengah, seret pembatas, maksimalkan, "Susun grid", dan Akhiri sesi vs Tutup pane. Yang teramati: Cmd+D dan Cmd+Shift+D bekerja (5 dari 5), header pane menampilkan nomor, lencana provider, status, dan kontrol, dan layout campuran terbentuk di layar.
7. Mitigasi "pane tersembunyi tidak merender" tidak teruji.

## Temuan lain: beban output besar memutus koneksi UI

Pada percobaan sebelumnya, `yes | head -n 20000` ke 8 sesi sekaligus membuat daemon mencatat `klien lambat, koneksi dijatuhkan` untuk `session.output.*`, dan pane menampilkan "koneksi ke daemon terputus" tanpa pulih sendiri. Pada percobaan ini, beban 2000 baris ke satu sesi tidak memicu pemutusan (0 catatan). Ambang pemutusan perlu ditangani di tiket terpisah (kandidat: perbesar buffer klien, gabungkan event output, sambung ulang otomatis).

## Kesimpulan gerbang

Hasil di skenario 7 pane (6 di antaranya Claude Code): 330,58 MB idle dan 338,30 MB dengan output, di atas target 200 MB (1,65x–1,69x) dan **di atas ambang 300 MB (1,10x–1,13x)**. Sesuai spec bagian 8, **ambang 1,5x terlewati; keputusan opsi UI native (spec v1 bagian 9b opsi c) dibawa ke pembuat.** Laporan ini tidak memilih atau menolak opsi tersebut.

Data yang relevan untuk keputusan: tambahan sekitar 22,85 MB per pane (didominasi proses WebContent: 49,06 MB pada 1 pane menjadi 163,88 MB pada 7 pane), dan CPU 13–20% dengan 6 TUI Claude terlihat. Tindak lanjut yang bisa diukur: ukur ulang dengan 6 shell kosong untuk memisahkan efek pane dari efek TUI Claude, uji scrollback lebih kecil atau renderer berbeda, dan ukur dengan pane yang diminimalkan atau disembunyikan. Keputusan akhir ada di pembuat.
