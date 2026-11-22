# Kerangka Pengaturan (Semua pengaturan, Connector, MCP) — Desain

Bagian 1 dari 4 untuk fitur Connector. Bagian lain (dikerjakan terpisah, berurutan):
2. Menu MCP, 3. Connector GitLab, 4. Connector Lark.

## Maksud dan alasan
User ingin menyiapkan koneksi ke layanan luar (git provider, Lark, Jira, dst) dan
mendaftarkan server MCP dari dalam app. Connector dipakai oleh agen dan oleh UI app
(mis. daftar MR GitLab dan status pipeline di sidebar; tiket Lark Base dan wiki).
Bagian ini hanya menyediakan wadahnya: modal pengaturan, sub-menu Connector dan MCP,
antarmuka connector generik, dan dialog cara menyambung.

## Tujuan
- Entri "Semua pengaturan…" di dropdown gear (`SettingsButton.svelte`) membuka modal penuh.
- Modal: sidebar kiri (Connector, MCP), konten di kanan. Bisa ditutup dengan Esc dan klik di luar.
- Halaman Connector menampilkan daftar connector dengan status: belum terpasang CLI,
  belum login, atau tersambung (beserta identitas user).
- Connector yang belum tersambung punya tombol **Connect**; klik membuka dialog cara
  menyambung (langkah-langkah dan perintah yang bisa disalin, tombol "Cek ulang" yang
  menjalankan `Status()` lagi).
- Antarmuka `Connector` generik sehingga bagian 3 dan 4 hanya menambah adapter.
- Halaman MCP di bagian ini hanya kerangka kosong (isi dikerjakan di bagian 2).

## Bukan tujuan
- Tidak ada adapter GitLab/Lark, daftar MR/tiket, atau sidebar (bagian 3 dan 4).
- Tidak ada pengelolaan server MCP (bagian 2).
- App tidak login sendiri dan tidak menyimpan token layanan; itu pendekatan B, ditunda.
- Tidak ada connector chat.

## Anti-tujuan
- Menyimpan token atau kredensial layanan di config app.
- Menutup atau mengganggu sesi terminal yang sedang berjalan saat modal dibuka.

## Pendekatan yang dipertimbangkan
- Wadah UI: (a) pane di area layout, (b) modal penuh. **Dipilih b**: pengaturan jarang
  dibuka sambil bekerja dan form lebih natural di modal. Konsekuensi: butuh komponen modal baru.
- Autentikasi: (A) pakai CLI yang sudah login, (B) app login sendiri dan simpan token di
  Keychain. **Dipilih A** untuk tahap ini; antarmuka dibuat generik agar B bisa ditambah nanti.

## Desain
- Backend Go: paket `internal/connector` dengan antarmuka
  `Connector { ID() string; Name() string; Status(ctx) (Status, error); ConnectGuide() Guide }`.
  `Status` berisi state (`cli_missing`, `not_logged_in`, `connected`), identitas user, dan pesan.
  `Guide` berisi langkah teks dan perintah yang bisa disalin. Registry mendaftarkan connector
  dan diekspos lewat IPC: `connector.list`, `connector.status`, `connector.guide`.
- Frontend: modul `modules/settings` berisi `SettingsModal.svelte`, `ConnectorsPage.svelte`,
  `ConnectDialog.svelte`, `McpPage.svelte` (kerangka), dan store untuk status connector.
  Gear menambah grup baru dengan item "Semua pengaturan…".
- Konfigurasi: bagian ini hanya menambah state UI ringan; konfigurasi MCP dan connector
  disimpan di file config app (perluasan `AppConfig`) saat bagian 2 membutuhkannya.
- Untuk pengujian, disediakan satu connector contoh (fake) agar daftar, status, dan dialog
  bisa diverifikasi sebelum adapter nyata ada.

## Keputusan saya (mohon dicek)
- Tombol "Buka di terminal" di dialog (menjalankan perintah login di sesi terminal baru) saya
  **tidak** masukkan sekarang; cukup salin perintah dan "Cek ulang". Bisa ditambah kemudian.
- GitLab self-hosted dianggap sudah ditangani `glab` sendiri; tidak ada UI host khusus.
- Teks UI memakai bahasa Indonesia, mengikuti app.

## Diserahkan ke pembangun
- Gaya visual modal dan dialog (ikuti variabel CSS yang ada: `--bg-elev`, `--border`, dst).
- Nama pesan error dan detail aksesibilitas (fokus, Esc, urutan tab).
- Cara memanggil CLI dengan timeout dan penanganan CLI tidak ditemukan.
