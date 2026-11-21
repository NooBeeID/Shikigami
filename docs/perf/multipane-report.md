# Laporan Pengukuran RAM/CPU Multi-Pane (PARSIAL: 6 pane tidak terukur)

Ringkasan: hanya titik **1 pane** yang berhasil diukur dengan angka nyata (UI 116,94 MB, CPU idle < 0,1%). Titik **6 pane tidak terukur**: macOS menolak pengiriman tombol otomatis (osascript error 1002, izin Accessibility untuk mengirim input belum diberikan), sehingga 5 pane tambahan tidak bisa dibuat tanpa pengguna. Saat mencoba beban output, ditemukan **galat nyata**: daemon memutus koneksi UI ("klien lambat, koneksi dijatuhkan") dan pane tidak tersambung kembali. Status gerbang spec bagian 8: **TIDAK TERUKUR**. Tiket tetap `in-progress`.

## Metadata mesin dan build

| Item | Nilai |
|---|---|
| OS | macOS 15.7.7 (24G720) |
| Arsitektur | arm64, Apple M3, 8 core |
| RAM | 16,00 GB |
| Go | go1.26.3 darwin/arm64 |
| xterm | `@xterm/xterm` ^6.0.0, `@xterm/addon-fit` ^0.11.0 |
| Waktu ukur | 2026-11-21T18:56Z (01:56–02:00 WIB, 2026-11-22) |
| Commit | `c701824` (tiket v2-03 ter-merge) |
| Biner | `build/bin/*` dibuild 01:51:23, setelah merge tiket 03; diukur dari salinan sementara |
| Data dir | `SHIKIGAMI_DATA_DIR` sementara (data nyata dan daemon pengguna tidak disentuh) |

## Kondisi lingkungan

Mesin **tidak idle**: load average 6,9–13,2 selama sesi. Proses berat lain: VM Virtualization (~190% CPU), Antigravity IDE helper (~100–130%), sesi `claude` lain, Orca, Terminal. Implementasi tiket 04 juga berjalan di awal sesi. CPU per proses diukur dari CPU time proses itu sendiri sehingga kontaminasi diperkirakan kecil, tetapi tidak bisa dikesampingkan.

## Konfigurasi mitigasi aktif (dikonfirmasi dari kode)

- `scrollback={1000}` pada pane terminal (`modules/layout/panes/TerminalPane.svelte`).
- Tanpa addon WebGL: `grep -rniE webgl` pada `modules/terminal` dan `modules/layout` menghasilkan 0 baris.
- Pane tersembunyi `visible=false` tidak merender: tidak diamati langsung (hanya dari plan dan kode).

## Metode dan perintah

Sama dengan `docs/perf/spike-report.md`: RSS dari `ps -o rss= -p PID`; CPU idle dari selisih `ps -o time= -p PID` dibagi selisih waktu dinding, dikali 100, interval 10 detik, satu sampel per titik; kolom `%cpu` tidak dipakai; PID WebKit milik app ditentukan dengan membandingkan daftar `com.apple.WebKit*` sebelum dan sesudah app dijalankan (3 PID baru: GPU, WebContent, Networking).

Perbedaan dari spike v1: biner bernama `shikigami*`; aplikasi dijalankan dengan workspace, project, dan sesi yang dibuat lewat `shikigamictl`; app terbuka dengan satu pane otomatis (sesi pertama). Pembersihan: hanya PID app dan daemon milik pengukuran ini yang dihentikan; daemon milik pengguna (PID 28415) tetap hidup; folder sementara dihapus.

## Angka mentah (titik yang valid)

| Titik | Proses | PID | RSS (KB) | RSS (MB) | CPU awal (s) | CPU akhir (s) | Delta wall (s) | CPU idle (%) |
|---|---|---|---|---|---|---|---|---|
| A, 1 pane | shikigami | 51128 | 73056 | 71,34 | 1,50 | 1,50 | 10,064 | < 0,1 |
| A, 1 pane | WebKit.GPU | 51142 | 20864 | 20,38 | 0,65 | 0,65 | 10,064 | < 0,1 |
| A, 1 pane | WebKit.WebContent | 51143 | 18144 | 17,72 | 1,77 | 1,77 | 10,064 | < 0,1 |
| A, 1 pane | WebKit.Networking | 51144 | 7680 | 7,50 | 0,06 | 0,06 | 10,064 | < 0,1 |
| **A, 1 pane (jumlah)** | | | **119744** | **116,94** | | | | **< 0,1** |
| daemon, 8 sesi | shikigamid | 41636 | 5696 | 5,56 | 0,08 | 0,08 | 10,030 | < 0,1 |

Titik "B (6 pane idle)" dan "C (6 pane + output)" dari proses ukur **dibuang**: tombol Cmd+D tidak terkirim (ditolak macOS), sehingga UI tetap 1 pane; pada titik C koneksi UI juga sudah terputus. Angkanya tidak mewakili 6 pane dan tidak dipakai.

## Perbandingan dengan target

| Metrik | Referensi | Terukur | Rasio | Status |
|---|---|---|---|---|
| UI 1 pane | 200 MB | 116,94 MB | 0,58x | LULUS (indikatif) |
| UI 1 pane | baseline v1 187,19 MB | 116,94 MB | 0,62x | lebih rendah (kondisi ukur berbeda) |
| UI 6 pane | 200 MB / ambang 300 MB | tidak terukur | n/a | **TIDAK TERUKUR** |
| CPU idle UI 1 pane | 1,0% | < 0,1% | < 0,1x | LULUS |

Angka 1 pane lebih rendah dari baseline v1 (187,19 MB). Jangan dibaca sebagai perbaikan nyata: kondisi ukur berbeda (jumlah sesi, saat ukur setelah peluncuran, mesin sibuk), dan sampel hanya satu.

## Temuan: koneksi UI diputus oleh beban output

Saat `yes | head -n 20000` dikirim ke semua sesi (8 sesi) lewat `session.write`, log daemon mencatat `klien lambat, koneksi dijatuhkan` untuk topik `session.output.*` (01:59:05). Setelah itu pane menampilkan "Gagal terhubung ke sesi: koneksi ke daemon terputus" dan tidak pulih. Beban 20000 baris per sesi adalah beban "ringan" menurut plan pengukuran ini. Artinya perlindungan klien lambat terlalu agresif atau UI tidak menyambung ulang, dan agent yang menghasilkan output besar (mis. `cat` berkas besar) berisiko memutus UI. Ini perlu ditangani di tiket terpisah (kandidat: perbesar buffer klien atau gabungkan event output di daemon/bridge, dan sambung ulang otomatis di UI).

## Keterbatasan

1. Titik 6 pane tidak terukur karena izin pengiriman tombol macOS belum diberikan; layout 6 pane tidak pernah terbentuk.
2. Satu sampel per titik; variansi tidak diketahui. RSS menghitung halaman shared (bias ke atas).
3. Mesin sibuk selama ukur (lihat kondisi lingkungan).
4. Sesi berisi shell kosong, bukan agent nyata.
5. Perilaku "pane tersembunyi tidak merender" tidak diamati.
6. Verifikasi manual UI di plan (split Cmd+D dan Cmd+Shift+D, drag header ke empat tepi dan tengah, seret pembatas, maksimalkan, Susun grid) **tidak dijalankan**.

## Kesimpulan gerbang

Gerbang spec bagian 8 (ambang 300 MB untuk 6 pane): **TIDAK TERUKUR**. Tidak ada rekomendasi opsi UI native. Untuk menyelesaikan: berikan izin Accessibility (Pengaturan Sistem, Privasi & Keamanan, Aksesibilitas) ke aplikasi terminal yang menjalankan pengukuran agar tombol bisa dikirim, atau susun 6 pane secara manual lalu ukur dengan prosedur di atas. Keputusan akhir gerbang ada di pembuat.
