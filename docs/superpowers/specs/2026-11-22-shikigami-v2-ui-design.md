# Shikigami v2: UI Multi-Pane, Dua Sidebar, dan Instalasi ke Applications

Status: draft untuk persetujuan. Melanjutkan spec `2026-11-21-ai-terminal-design.md` (sembilan tiket pertama sudah ter-merge).
Tanda **[call]** = keputusan saya (bukan hasil kesepakatan), silakan koreksi.

## 1. Intent dan alasan

Setelah mencoba hasil v1, pembuat menemukan kekurangan yang membuatnya belum nyaman dipakai sehari-hari:

1. Tidak ada **save workspace**: workspace tidak terasa tersimpan, dan tata letak tidak diingat.
2. Tidak ada **open folder**: project hanya bisa ditambah dengan mengetik path.
3. Tidak bisa **drag & drop untuk mengatur layout** (split terminal): hanya tab.
4. Tidak bisa **zoom in/out** dengan Cmd+ dan Cmd-.
5. Sidebar hanya satu. Diinginkan **dua sidebar**: kiri untuk agent, kanan untuk folder/file.
6. Tampilan perlu diperbaiki mengikuti referensi (lihat bagian 4).
7. Belum ada cara **install ke Applications**: aplikasi hanya biner di `build/bin/`.

Tujuan v2: Shikigami terasa seperti aplikasi macOS sungguhan yang dipakai untuk mengawasi banyak agent sekaligus dalam satu layar (seperti referensi: beberapa terminal terlihat bersamaan), tetap ringan.

## 2. Goals

- **Layout multi-pane**: beberapa terminal terlihat bersamaan, dibagi (split) horizontal/vertikal, dengan drag & drop untuk menyusun ulang dan menyeret pembatas untuk mengubah ukuran.
- **Dua sidebar**: kiri = agent/sesi (per workspace), kanan = folder dan file (tree project). Keduanya bisa disembunyikan dan diubah lebarnya.
- **Bilah atas** dengan tab workspace dan tombol tambah, penghitung sesi, dan pengaturan.
- **Zoom** dengan Cmd+= / Cmd+- / Cmd+0.
- **Open folder** lewat dialog macOS (Cmd+O), drag folder dari Finder ke jendela, dan buat workspace dari folder.
- **Save workspace**: workspace beserta layout, pane, dan file yang terbuka tersimpan otomatis dan dipulihkan saat aplikasi dibuka lagi.
- **Instalasi**: `Shikigami.app` yang bisa dipasang ke `/Applications` dengan satu perintah dan dibuka dari Launchpad/Spotlight.
- Tetap ringan: tidak ada polling, tidak ada dependency besar.

## 3. Non-goals dan anti-goals

Non-goals (v2):
- Kanban/Task, avatar/akun, dan fitur lain di bilah atas referensi yang tidak terkait terminal.
- Windows/Linux (tetap macOS saja), banyak jendela, editor tema.
- Distribusi ke mesin lain (notarization/DMG); lihat bagian 7, hanya instalasi lokal.
- Syntax highlighting dan fitur IDE.

Anti-goals (gagal walau fungsinya jalan):
- Memindahkan pane mematikan atau mereset sesi/terminalnya (instance xterm dipindah, bukan dibuat ulang).
- Memori UI membengkak karena banyak terminal tampil bersamaan melampaui target (lihat bagian 8).
- Aplikasi yang dibuka dari Finder tidak bisa menemukan `claude`/`codex` karena PATH.

## 4. Desain UI

Referensi: tangkapan layar aplikasi serupa (tema gelap; bilah atas; grid 3x2 terminal; tiap pane punya header; pane fokus diberi bingkai berwarna). Yang diambil adalah pola tata letaknya saja, bukan merek atau elemen non-terminal.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ ● ● ●   [ Core ▾ ] [ SDK-A ] [ SDK-B ] [+]            6 sesi    ⚙           │ bilah atas
├───────────┬────────────────────────────────────────────────┬─────────────────┤
│ AGENT     │ ┌─ 1 · debug scroll ─ CL ● ──── ⤢ ✕ ┐┌─ 2 · rollback ─ CX ● ─ ⤢ ✕ ┐│ FILE            │
│           │ │ terminal                           ││ terminal                    ││ ▾ core          │
│ core      │ │                                    ││                             ││   ▸ cmd         │
│  ● debug  │ └────────────────────────────────────┘└─────────────────────────────┘│   ▸ internal    │
│  ○ rollbk │ ┌─ 3 · email check ─ CL ● ───── ⤢ ✕ ─────────────────────────────┐ │   README.md     │
│ sdk-a     │ │ terminal                                                       │ │ ▸ sdk-a         │
│  ● tests  │ └────────────────────────────────────────────────────────────────┘ │                 │
│ [+ sesi]  │                                                                      │ (klik file →    │
│           │                                                                      │  pane file)     │
└───────────┴────────────────────────────────────────────────┴─────────────────┘
```

- **Bilah atas**: tab workspace (klik pindah, `+` buat workspace, menu konteks ganti nama/hapus), penghitung sesi, tombol pengaturan.
- **Sidebar kiri (Agent)**: sesi milik workspace aktif, dikelompokkan per project, dengan indikator status (berjalan / menunggu / selesai / idle dari tiket 08), tombol `+ sesi` dengan pilihan provider (claude, codex, shell) dan project. Klik sesi memfokuskan pane-nya; sesi yang belum tampil di layout dimasukkan ke layout.
- **Sidebar kanan (File)**: tree project di workspace aktif (pemilih project di atasnya), klik file membuka pane file. Tombol "Open Folder…".
- **Area tengah**: layout pane (bagian 5). Sidebar bisa ditoggle (Cmd+B kiri, Cmd+Opt+B kanan **[call]**) dan lebarnya bisa diseret.
- **Header pane** (mengikuti referensi): nomor, judul (bisa diganti nama), lencana provider, titik status, tombol maksimalkan/pulihkan, tutup, dan menu `…`. Pane yang fokus diberi bingkai beraksen.
- **Tema**: gelap sebagai bawaan; warna lewat CSS variables (sudah ada `--term-*` dari tiket 06). Terang mengikuti sistem **[call]**.

### 4b. Referensi kedua dan unsur yang diambil

Gambar referensi kedua (rail ikon, sidebar kiri dengan workspace/project/grup sesi, tab grup, sesi dikelompokkan per project, panel kanan Orchestration/Tasks/Activity, palet perintah) jauh lebih luas daripada yang diminta pembuat untuk v2. Pemetaannya:

**Diadopsi di v2 [call]** (selaras kebutuhan dan ringan):
- **Ringkasan di judul area tengah**: nama grup/workspace, status "Running", jumlah sesi, project, dan agent yang terhubung (dari mesin status tiket 08).
- **Header pane yang lebih kaya**: chip path project/subfolder, tombol split, maksimalkan, dan menu `…` (menyempurnakan header di bagian 4).
- **Sidebar kiri**: pemilih workspace di atas (nama, jumlah project dan sesi), daftar sesi, dan tombol "New Session". Pohon folder tetap di **sidebar kanan** sesuai permintaan pembuat (di referensi kedua pohon itu ada di kiri).
- **Palet perintah (Cmd+K)**: cari dan lompat ke project, sesi, atau file, dan jalankan aksi (sesi baru, open folder, zoom).
- **Lonceng notifikasi** di bilah atas: daftar peristiwa dari mesin status (agent menunggu input, selesai).

**Keputusan pembuat (sudah dijawab):**
- **Grup sesi sebagai tab: MASUK v2.** Tab Development / CI / Services / Experiments dengan jumlah sesi; tiap grup punya layout sendiri. Ini menggantikan "layout bernama" dan menambah satu tingkat di model data (workspace → grup → layout).
- **Mode tampilan per project: DITUNDA.** v2 membangun split bebas saja; bagian per project yang bisa dilipat (dengan jalankan/hentikan semua) belakangan.
- **Meter sumber daya (CPU/Memori): kini masuk v2 dalam bentuk terbatas** (sampling hanya saat jendela terlihat, UI polling tiap 5 detik); lihat `docs/superpowers/specs/2026-11-22-resource-monitor-design.md`. Disk tetap tidak masuk.
- **Feed aktivitas: MASUK v2 tanpa Approve/Reject.** Tab "Activity" di panel kanan di samping "Files" menampilkan tool call terbaru dari hooks (tiket 08). Tombol Approve/Reject tidak dibuat (lihat bagian 11b: agent selalu berjalan tanpa izin, jadi tidak ada permintaan izin yang perlu disetujui).

**Tidak masuk v2** (calon spec lain): Orchestration (Agent Plan, progres tugas, agent Orchestrator/Coder/Tester), Tasks, composer "Instruct agents" yang menyiarkan ke banyak agent, dan rail ikon navigasi (Agents/Tasks/Knowledge). Ini mengarah ke lapisan orkestrasi dan knowledge/hooks (fase 4–5 spec pertama), dan lebih baik jadi spec v3 setelah v2 stabil.

## 5. Layout, drag & drop, dan split

**Model layout (pohon pembagian)** **[call]**: layout adalah pohon biner. Daun = pane (terminal atau file), simpul = pembagian horizontal/vertikal dengan rasio. Mirip iTerm/tmux, sehingga "split terminal" bermakna sama.

Perilaku:
- **Split**: dari menu pane atau pintasan (Cmd+D bagi kanan, Cmd+Shift+D bagi bawah **[call]**) membuat pane baru di samping pane saat ini (sesi baru dengan project yang sama).
- **Drag & drop**: seret header pane ke pane lain. Zona drop: tepi atas/bawah/kiri/kanan = bagi pane target di sisi itu; tengah = tukar posisi. Ada penanda zona saat menyeret.
- **Ubah ukuran**: seret pembatas antar pane.
- **Maksimalkan** satu pane sementara (tombol di header atau klik ganda header), lalu pulihkan tanpa mengubah layout.
- **Rapikan**: aksi "Susun grid" menata semua pane menjadi grid otomatis (seperti referensi, 3x2 dst.) **[call]**.
- **Pane file**: file dari sidebar kanan dibuka sebagai pane di layout yang sama (viewer/editor dari tiket 07), bukan hanya tab **[call]**.
- Menutup pane **tidak** membunuh sesi (sesuai v1: sesi tetap hidup di daemon dan tetap muncul di sidebar kiri); aksi terpisah "Akhiri sesi" yang membunuhnya.
- Memindah/men-split pane **tidak boleh** membuat ulang xterm atau memutus attach ke sesi; yang dipindah adalah elemen DOM yang sama.

Implementasi: kode sendiri (CSS grid/flex + pointer events), tanpa library drag & drop besar **[call]**.

## 5b. Grup sesi (definisi)

Fokus Shikigami: **sesi Claude dan file**. Grup sesi bukan jenis sesi (bukan "CI" atau "Services" yang menjalankan runner/servis); nama-nama itu hanya contoh di gambar referensi.

**Definisi** **[call]**: grup sesi = satu **konteks kerja** milik sebuah workspace, diberi nama oleh pengguna, berisi sekumpulan sesi dan file yang dibuka, dengan layout pane sendiri. Contoh pemakaian: satu grup per fitur, per cabang, atau per tugas ("fitur-approval", "bugfix-login", "review-pr-12"), masing-masing dengan beberapa sesi Claude dan file yang relevan berdampingan.

Aturan:
- **Tidak ada grup bawaan bermakna.** Setiap workspace otomatis punya satu grup bernama "Utama". Workspace lama dari v1 menjadi grup "Utama" itu.
- **Tab grup** tampil di atas area layout, dengan jumlah sesi di tiap tab. Aksi: buat (`+`), ganti nama, hapus, urutkan dengan drag.
- **Satu sesi milik satu grup** pada satu waktu; sesi dipindahkan antar grup lewat menu sesi atau dengan menyeretnya ke tab grup lain. Pane file milik grup tempat ia dibuka.
- **Hapus grup**: pane-nya ditutup, dan sesi di dalamnya dipindahkan ke grup "Utama" (tidak dibunuh). Grup "Utama" tidak bisa dihapus.
- **Sidebar kiri** menampilkan sesi milik grup aktif lebih dulu, lalu grup lain (dilipat), sehingga semua sesi workspace tetap terlihat.
- **Tersimpan otomatis** bersama workspace (bagian 6); grup aktif dan layout tiap grup dipulihkan saat dibuka.
- **Yang tidak ada di v2**: grup lintas workspace, satu sesi muncul di banyak grup, jalankan/hentikan semua sesi dalam grup, dan template grup.
- Pilihan provider (claude, codex, shell) tetap ada di "+ sesi", tetapi alur utamanya dirancang untuk sesi Claude (lencana, status, mention).

## 6. Zoom, Open Folder, dan Save Workspace

**Zoom** **[call]**:
- Cmd+= memperbesar, Cmd+- memperkecil, Cmd+0 mengembalikan; langkah 10%, rentang 50%–300%.
- Dipasang lewat **menu native** (View) dengan accelerator, karena WKWebView bisa menelan tombol itu sebelum sampai ke halaman. Menu mengirim event ke frontend.
- Skala berlaku ke UI (variabel skala di root) dan ukuran font semua terminal sekaligus; tersimpan di konfigurasi (global, bukan per workspace) dan dipulihkan saat dibuka.

**Open Folder**:
- Menu File > "Open Folder…" (Cmd+O) memakai dialog direktori macOS dari sisi Go (Wails runtime `OpenDirectoryDialog`) yang dipanggil lewat bridge; ini menggantikan input path teks (tiket 06 menyebut dialog tidak tersedia dari JS, tetapi sisi Go menyediakannya).
- Hasilnya menambahkan folder sebagai project ke workspace aktif. "New Workspace from Folder…" membuat workspace baru bernama folder itu.
- Drag folder dari Finder ke jendela menambahkannya sebagai project (memakai file-drop Wails v2) **[call]**.
- Input path teks tetap ada sebagai cadangan.

**Save workspace** **[call]**:
- Workspace disimpan **otomatis** (debounce sebentar setelah perubahan) ke `workspaces.json` (sudah ada dari tiket 04): daftar project, layout pohon, pane (jenis, sesi/berkas yang ditampilkan, judul), sidebar tampil/sembunyi dan lebarnya, workspace aktif.
- Saat aplikasi dibuka: layout dipulihkan. Pane terminal yang sesinya masih hidup di daemon di-attach kembali (termasuk scrollback). Bila daemon sudah mati (mis. setelah reboot), pane menampilkan "Sesi berakhir" dengan tombol **Mulai ulang** yang menjalankan sesi baru di project dan provider yang sama (tanpa melanjutkan percakapan agent).
- Pane file dipulihkan dengan membuka ulang file yang sama (perubahan belum tersimpan tidak dipulihkan).
- Tidak ada tombol "Save" wajib; aksi "Simpan layout sebagai…" (layout bernama) ditunda **[call]**.

## 7. Instalasi ke Applications

Target: `Shikigami.app` di `/Applications`, dibuka dari Launchpad/Spotlight/Dock.

**Cara** **[call]**: skrip build sendiri (tanpa Wails CLI) yang merakit bundle dari biner hasil `make build`:
- `build/Shikigami.app/Contents/MacOS/shikigami` (app), `shikigamid` (daemon, di folder yang sama sehingga `daemon-autostart` menemukannya), dan `shikigamictl`.
- `Contents/Info.plist`: nama "Shikigami", bundle ID `id.noobee.shikigami` **[call]**, `LSMinimumSystemVersion`, `NSHighResolutionCapable`, ikon.
- `Contents/Resources/shikigami.icns`: ikon. Belum ada desain ikon, jadi **ikon sementara** dibuat otomatis dari satu gambar sumber dan bisa diganti kemudian.
- **Tanda tangan ad-hoc** (`codesign --force --deep -s -`) agar bundle valid untuk dijalankan di mesin sendiri.
- `make app` merakit bundle; `make install` menyalin ke `/Applications` (menimpa versi lama, menghentikan `shikigamid` lama dulu); `make uninstall` menghapusnya. Data di `~/Library/Application Support/shikigami` tidak disentuh.
- Opsional: symlink `shikigamictl` ke `/usr/local/bin` (lewat target terpisah) untuk memakai CLI dari terminal.

**PATH saat dibuka dari Finder (wajib)**: aplikasi GUI tidak mewarisi PATH shell, sehingga `claude`/`codex` (mis. dari nvm/Homebrew) tidak ditemukan. Daemon harus memperoleh PATH login-shell pengguna saat start (mis. menjalankan `$SHELL -l -i -c` untuk mencetak PATH, dengan timeout) dan memakainya untuk mencari biner provider dan untuk environment sesi. Ini bagian dari tiket instalasi, bukan opsional.

**Yang tidak dicakup**: Developer ID, notarization, dan DMG untuk dibagikan ke mesin lain (di mesin lain Gatekeeper akan memperingatkan bundle yang tidak ternotarisasi). Cukup untuk dipakai di mesin pembuat.

## 8. Risiko memori dan keputusan gerbang

Laporan spike v1 mencatat UI satu tab sudah 187 MB (0,94x dari target 200 MB) dan estimasi 6 sesi 189–212 MB **dengan asumsi hanya tab aktif yang punya instance xterm**. Layout multi-pane justru menampilkan banyak terminal sekaligus sehingga asumsi itu tidak berlaku lagi.

Persyaratan **[call]**:
- Setelah layout multi-pane jadi, **ukur ulang** UI dengan 6 pane terlihat (prosedur sama dengan spike v1) dan catat di `docs/perf/`.
- Mitigasi yang disiapkan: scrollback xterm kecil (mis. 1000 baris), tanpa addon WebGL, dan pane yang tersembunyi/diminimalkan tidak merender (instance ditahan, bukan dihancurkan).
- Gerbang: bila hasil ukur melewati 1,5x target (300 MB), keputusan opsi UI native (spec v1 §9b opsi c) dibawa ke pembuat. Hasil ukur dilaporkan, keputusan tetap di pembuat.

## 9. Perubahan arsitektur

- **Model workspace**: `workspaces.json` ditambah grup sesi per workspace, dan satu dokumen layout per grup (workspace → grup → layout; versi skema dinaikkan, dengan migrasi dari versi lama yang tanpa grup/layout: workspace lama menjadi satu grup bawaan).
- **IPC/bridge baru**: simpan/ambil layout (`workspace.layout.get/set` atau bagian dari `workspace.*`), `dialog.openFolder` (dipanggil lewat bridge Wails, bukan daemon), event drop folder, event zoom dari menu native.
- **Frontend**: `modules/shell` dirombak menjadi bilah atas + dua sidebar + area layout; `modules/tabs` digantikan manajer layout (pohon pane). Modul terminal, file, status, dan mention dipakai ulang sebagai isi pane.
- **Aplikasi**: menu native macOS (File, View, Window) dengan accelerator; konfigurasi jendela (judul, ukuran awal, ingat posisi **[call]**).

## 10. Fase pengerjaan **[call]**

1. **Instalasi**: bundle `.app`, ikon sementara, tanda tangan ad-hoc, `make install/uninstall`, dan PATH login-shell. Dikerjakan pertama karena kecil, mandiri, dan membuat aplikasi bisa dipakai dari Applications.
2. **Menu native, zoom, dan Open Folder**: menu, Cmd+= / Cmd+- / Cmd+0, dialog folder, drop folder.
3. **Layout multi-pane**: pohon pane, split, drag & drop, ubah ukuran, maksimalkan, header pane. Diakhiri dengan ukur ulang memori (gerbang bagian 8).
4. **Bilah atas dan dua sidebar**: tab workspace, sidebar Agent (kiri) dan File (kanan), pane file di layout, palet perintah (Cmd+K), dan lonceng notifikasi.
5. **Save workspace**: simpan otomatis layout, pulihkan saat dibuka, "Mulai ulang" untuk sesi yang mati.
6. **Grup sesi dan feed aktivitas**: tab grup sesi dengan layout sendiri per grup (model workspace → grup → layout; sebaiknya sudah dipertimbangkan sejak fase 3 dan 5 agar data tidak perlu dimigrasi dua kali), dan tab "Activity" (tool call dari hooks, tanpa Approve/Reject). Mode tampilan per project tidak termasuk; meter sumber daya masuk dalam bentuk terbatas (lihat docs/superpowers/specs/2026-11-22-resource-monitor-design.md). Orchestration/Tasks dipisah ke spec v3.

Urutan 3 dan 4 bisa bertukar atau berjalan paralel bila kontrak layout sudah disepakati; tiket diputuskan oleh `/create-ticket-from-spec`.

## 11. Pertanyaan terbuka

1. **Pemulihan sesi setelah reboot**: saat ini "Mulai ulang" memulai sesi agent baru tanpa melanjutkan percakapan. Apakah perlu opsi melanjutkan percakapan (mis. `claude --continue`) di versi ini, atau cukup di versi berikutnya?
2. **Ikon dan bundle ID**: ikon sementara dibuat otomatis dan bundle ID `id.noobee.shikigami` adalah tebakan saya. Ada desain ikon atau ID yang diinginkan?
3. **Pane file di layout** (bukan hanya tab): setuju, atau file tetap sebagai tab terpisah?
4. **Grup sesi sebagai tab** (tiap grup punya layout sendiri, seperti Development / CI / Services di referensi kedua): masuk v2 atau cukup satu layout per workspace dulu?
5. **Meter sumber daya** (CPU/Memori/Disk di sidebar kiri): **kini masuk v2 dalam bentuk terbatas** (sampling hanya saat jendela terlihat, UI polling tiap 5 detik; lihat `docs/superpowers/specs/2026-11-22-resource-monitor-design.md`).
6. **Feed aktivitas dan Approve/Reject**: masuk v2 sebagai tab "Activity" (tanpa Approve/Reject dulu, karena belum terbukti), atau seluruhnya dipindah ke spec v3?

**Jawaban pembuat (semua terjawab):**
1. Pemulihan sesi: **sesi baru saja** (tanpa melanjutkan percakapan agent).
2. Ikon dan ID: **ikon sementara** dan bundle ID **`id.noobee.shikigami`**.
3. Pane file: **pane di layout**.
4. Grup sesi sebagai tab: **masuk v2**.
5. Meter sumber daya: **tidak masuk v2**. (Direvisi 2026-11-22: kini masuk dalam bentuk terbatas; lihat docs/superpowers/specs/2026-11-22-resource-monitor-design.md.)
6. Feed aktivitas: **masuk v2 tanpa Approve/Reject**.
7. Mode tampilan per project: **belakangan** (v2 split bebas dulu).

## 11b. Agent selalu berjalan tanpa pemeriksaan izin

Keputusan pembuat: agent **selalu** dijalankan dengan flag yang menonaktifkan prompt izin.
- `claude`: `--dangerously-skip-permissions`.
- `codex`: `--dangerously-bypass-approvals-and-sandbox` (flag ini dan flag claude di atas sudah dicek ada di `--help` kedua CLI pada mesin pembuat).
- Provider `shell` tidak terpengaruh.
- Tidak ada toggle atau pengaturan untuk mematikannya di v2 **[call]**; flag tidak ditambahkan dua kali bila sudah ada di argumen.
- Sudah diterapkan di kode v1 (`internal/provider`, commit `62cb05f`), dan harus dipertahankan saat adapter provider diubah di v2 (mis. saat PATH login-shell ditambahkan).

Konsekuensi yang perlu diketahui: karena agent tidak lagi meminta izin, status "menunggu" dari mesin status tidak akan muncul akibat permintaan izin (hanya dari agent yang menunggu input pengguna), dan fitur Approve/Reject tidak punya objek untuk disetujui. Ini sebab Approve/Reject tidak dibuat di v2. Agent yang berjalan tanpa izin dapat menjalankan perintah apa saja di project yang dipilih; itu risiko yang diterima pembuat.

## 12. Diserahkan ke builder

Struktur data pohon layout dan format serialisasinya, detail algoritma zona drop, pustaka kecil untuk pembatas bila dipilih, cara merakit `.icns` dari gambar sumber, urutan pintasan keyboard yang tidak bentrok dengan terminal (mis. Cmd+D, Cmd+B), serta judul default pane.
