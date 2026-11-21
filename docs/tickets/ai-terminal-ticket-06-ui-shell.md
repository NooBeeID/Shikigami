---
id: ai-terminal-ticket-06-ui-shell
title: UI Shell
spec: docs/superpowers/specs/2026-11-21-ai-terminal-design.md
status: ready
wave: 4
depends_on: [ai-terminal-ticket-02-session-pty, ai-terminal-ticket-04-workspace-model, ai-terminal-ticket-05-provider-adapters]
---
# UI Shell

## Tujuan

Epic ini menyempurnakan UI Wails di atas terminal dasar yang sudah dibuat tiket 02 (komponen `TerminalTab` xterm.js dan satu tab terminal minimal di `frontend/src/modules/terminal/`), dan mengganti placeholder Hello World dari tiket 01: tata letak shell, sidebar workspace dan project, daftar sesi milik workspace, banyak tab sesi dalam satu webview (memakai `TerminalTab` dari tiket 02, bukan membuat terminal dari nol), pembuatan sesi dengan pilihan provider dan project, serta auto-start daemon. Epic ini menutup MVP 1a pada spec (bagian 9): pembuat bisa menjalankan sesi Claude/Codex/shell dari aplikasi baru. Semua anti-goal spec dijaga: daftar sesi tidak berubah saat pindah project (anti-goal 2), tanpa polling (anti-goal 3), dan tetap ringan (satu webview, tanpa Electron, tanpa dependency berat).

Tiket ini berdiri sendiri dan tidak boleh merusak build. Tiket 01 sampai 05 (termasuk terminal dasar dari tiket 02) sudah ada dan lolos `make build`; setelah tiket ini selesai `make build` tetap lolos. Epic sesudahnya (file viewer, status, mention) hanya menambah panel lewat registry, tanpa menyentuh file di tiket ini.

## Batasan

In scope:
- `app/daemon.go`: auto-start daemon `aitermd` terlepas dari UI.
- `frontend/src/App.svelte` (diambil alih penuh dari placeholder tiket 01): layout sidebar / main-tab / statusbar yang dirender dari registry panel dan registry tipe tab.
- Store state di `frontend/src/lib/stores/` (workspace aktif, project aktif, daftar sesi, tab terbuka) berbasis event IPC.
- Styles dasar ringan, gelap dan terang (mengikuti `prefers-color-scheme`), di `frontend/src/styles/`.
- Modul `shell` (komponen `SidebarArea` dan `StatusBar`), `tabs` (tab bar dan konten banyak tab sesi), `workspace` (sidebar).
- Definisi CSS variables tema terminal (`--term-bg`, `--term-fg`, `--term-cursor`) di `styles/`; komponen terminal dari tiket 02 sudah membacanya.
- Pembersihan panel sementara tiket 02: menghapus pendaftaran panel `terminal-quick` dan `QuickTerminal.svelte` (satu-satunya sentuhan ke `modules/terminal/`, lihat fitur `session-tabs`).
- Dialog pilih folder untuk menambah project.

Out of scope:
- Membuat ulang komponen terminal, helper attach/subscribe/write/resize, atau pendaftaran tipe tab `session`: sudah ada dari tiket 02 (`modules/terminal/TerminalTab.svelte`, `terminal-client.ts`, `registerTabType`). Tiket ini hanya memakainya.
- Tree file, viewer/editor file, tipe tab `file` (tiket 07). Registry tab sudah mendukung tipe `file`; bila belum ada komponennya, shell menampilkan placeholder "tipe tab tidak tersedia" tanpa error.
- Indikator status sesi (running/waiting/done/idle) dan notifikasi (tiket 08). Statusbar hanya merender panel `statusbar` yang terdaftar; tanpa panel, statusbar kosong.
- Mention `@sesi` (tiket 09).
- Knowledge/Grimoire dan penataan project (fase 4 dan 5), Windows, syntax highlighting.
- Mengubah file milik epic lain: `frontend/src/lib/registry.ts`, `frontend/src/lib/ipc.ts`, `frontend/src/main.ts`, `frontend/package.json`, `app/main.go`, `app/bridge.go`, dan semua `internal/*`. Di `frontend/src/modules/terminal/` (milik tiket 02) hanya `index.ts` (hapus panel `terminal-quick`) dan `QuickTerminal.svelte` (dihapus) yang boleh disentuh; `TerminalTab.svelte` dan `terminal-client.ts` tidak diubah. Epic ini hanya memakai API-nya.
- Unit test (guard hanya build).

Catatan dependency npm: `@xterm/xterm` dan `@xterm/addon-fit` sudah ditambahkan oleh tiket 02 (fitur `terminal-tab-ui`). Tiket ini tidak menambah dependency dan tidak mengedit `frontend/package.json`. Bila dependency itu tidak ada saat implementasi, laporkan sebagai blocker.

## Kontrak

### Yang dipakai (dari epic lain)

IPC dari frontend lewat `frontend/src/lib/ipc.ts` (tiket 01): `call(method, params)` yang membungkus `Call` di `app/bridge.go`, dan `subscribe(topicPrefix, fn)` yang menerima event Wails `daemon:event` berbentuk `{topic, payload}`. Nama persis ekspor mengikuti file itu; baca dulu sebelum menulis store. Error JSON-RPC: -32000 ERR_BINARY, -32001 ERR_CONFLICT, -32002 ERR_NOT_FOUND, -32003 ERR_INVALID.

Dari tiket 02 (`frontend/src/modules/terminal/`): komponen `TerminalTab.svelte` (prop `sessionId: string`; mengurus subscribe-dulu, attach, scrollback, input, resize, penanganan `session.exit`, dan pembersihan saat destroy tanpa membunuh sesi), helper `terminal-client.ts`, dan `registerTabType({type: 'session', component: TerminalTab})`. Nama ekspor persis dibaca dari kode tiket 02.

Registry (`frontend/src/lib/registry.ts`, tiket 01): `registerPanel({id, area: 'sidebar'|'main-tab'|'statusbar', component, order})`, `registerTabType({type: 'session'|'file', component})`, dan getter untuk membaca daftar yang terdaftar (urut `order`). Modul dimuat otomatis dari `frontend/src/modules/<modul>/index.ts` lewat `import.meta.glob`.

Method daemon yang dipakai (nama dan bentuk final mengikuti tiket pemiliknya; tiket ini mengikuti kontrak spec bersama):
- `session.create` (workspaceId, projectId, providerId) -> Session; `session.list` (workspaceId) -> Session[]; `session.kill`(id); `session.write`(id, data base64); `session.resize`(id, cols, rows); `session.attach`(id) -> scrollback base64.
- `workspace.*`: list, create, rename, delete workspace; tambah dan hapus project (path folder). Nama method persis ada di `internal/workspace` dan `cmd/aitermctl/cmd_workspace.go` (tiket 04).
- `provider.list` -> daftar provider {id, name, ...} (tiket 05).
- `daemon.info`, `ping` (tiket 01).

Event yang dipakai: `session.output.<id>` (payload base64), `session.exit` (payload memuat id sesi), serta event perubahan workspace dan sesi bila tiket 02/04 menerbitkannya. Bila suatu event tidak ada, store memuat ulang lewat `session.list` / `workspace.*` setelah aksi sendiri (bukan polling berkala).

Tipe: `Session{id, workspaceId, projectId, cwd, providerId, status, createdAt}`, `Workspace{id, name, projects[{id,path,name}], config}`. Id berupa string ULID.

### Yang disediakan

Go (`app/daemon.go`, package yang sama dengan `app/main.go`):
- `func EnsureDaemon(ctx context.Context) error`: cek socket `<DataDir>/aitermd.sock` (path dari `internal/paths`); coba `ping` lewat `internal/ipc`; bila gagal, spawn `aitermd` terlepas lalu tunggu sampai `ping` berhasil (timeout 5 detik, interval 100 ms, ini penantian startup satu kali, bukan polling berkelanjutan). Mengembalikan error jelas bila timeout.
- Pendaftaran ke siklus startup Wails: lewat `init()` di `daemon.go` memanggil hook startup yang disediakan `app/bridge.go` (lihat Wiring). Epic ini tidak mengedit `app/main.go`.

Frontend, store di `frontend/src/lib/stores/` (Svelte store, state di satu tempat):
- `workspaces`: daftar Workspace, `activeWorkspaceId`, `activeProjectId` (project aktif hanya menentukan cwd sesi baru).
- `sessions`: sesi milik workspace aktif (bukan project). Mengganti `activeProjectId` TIDAK boleh mengubah isi atau urutan `sessions`.
- `tabs`: daftar tab terbuka `{id, type, title, refId}` dan `activeTabId`. Membuka sesi membuat tab `type: 'session'` dengan `refId = sessionId`.
- `providers`: hasil `provider.list`.
- Aksi: `loadWorkspaces()`, `selectWorkspace(id)`, `selectProject(id)`, `addProject(path)`, `removeProject(id)`, `createSession(providerId)` (memakai workspace dan project aktif), `killSession(id)`, `openTab(...)`, `closeTab(id)`.

Frontend, panel yang didaftarkan:
- modul `workspace`: `registerPanel({id: 'workspace-sidebar', area: 'sidebar', component, order: 10})`.
- modul `terminal` (milik tiket 02): tidak ada pendaftaran baru; pendaftaran `registerTabType({type: 'session', ...})` dari tiket 02 dipakai apa adanya.
- modul `shell`: komponen `SidebarArea` dan `StatusBar` (internal, dipakai `App.svelte`).
- modul `tabs`: komponen `TabBar` dan `TabContent` (internal, dipakai `App.svelte`) yang merender komponen tab dari registry tipe tab, sehingga tab `session` memakai `TerminalTab` dari tiket 02.

## Fitur

Semua fitur di bawah bisa dikerjakan paralel, kecuali disebut sebaliknya. Pembagian path tidak tumpang tindih. Antarmuka antar fitur (nama store dan aksi di atas, serta komponen `TabBar`/`TabContent` dari modul `tabs`) disepakati dari bagian Kontrak sehingga masing-masing bisa dibuat tanpa menunggu yang lain. Agar build tetap lolos pada tiap tahap, tiap fitur hanya mengimpor yang sudah ada di Kontrak; `shell-layout` memuat store lebih dulu (foundation) sehingga fitur lain mengimpornya. Terminal itu sendiri tidak dikerjakan di sini: tiket 02 sudah menyediakannya.

### daemon-autostart: Auto-start daemon dari UI

Deskripsi: `app/daemon.go` memastikan daemon hidup sebelum UI dipakai. Langkah: (1) bangun path socket dari `internal/paths`; (2) coba `ping` lewat klien `internal/ipc`; sukses -> selesai; (3) bila gagal, cari biner `aitermd` (urutan: sebelah biner app di dalam bundle, lalu `$PATH`); (4) spawn dengan `exec.Cmd` dan `SysProcAttr{Setsid: true}`, stdin/stdout/stderr diarahkan ke file log di `<DataDir>/aitermd.log` (atau `/dev/null`), lalu `cmd.Process.Release()` agar tidak ada hubungan induk-anak; (5) tunggu socket siap dengan `ping` ulang (timeout 5 detik, interval 100 ms); (6) bila socket lama basi (file ada tetapi `ping` ditolak), biarkan daemon baru yang membersihkannya, jangan hapus dari UI. Daemon tidak boleh mati saat window ditutup: UI tidak pernah mengirim sinyal ke daemon dan tidak memanggil Kill pada prosesnya.

Acceptance criteria:
- [ ] `go build ./...` lolos dengan `app/daemon.go` ada.
- [ ] Bila daemon belum jalan, memanggil `EnsureDaemon` memulai `aitermd` dan `ping` berhasil sebelum fungsi kembali.
- [ ] Bila daemon sudah jalan, tidak ada proses kedua yang di-spawn.
- [ ] Proses daemon yang di-spawn punya sesi sendiri (setsid) dan tetap hidup setelah aplikasi Wails ditutup.
- [ ] Error timeout atau biner tidak ditemukan dikembalikan sebagai error yang bisa ditampilkan UI (bukan panic).
- [ ] Tidak ada polling berkelanjutan setelah daemon siap.
- [ ] Tidak mengedit `app/main.go` dan `app/bridge.go`.

ownedPaths: `app/daemon.go`. Paralel dengan fitur lain.

### shell-layout: Layout, store, dan area panel

Deskripsi: `App.svelte` menggantikan placeholder (dan menimpa perubahan sementara tiket 02 yang hanya merender panel `main-tab`) dan merender tiga area: sidebar kiri (panel `sidebar` dari registry, urut `order`), area utama dengan tab bar dan konten tab aktif (komponen `TabBar` dan `TabContent` dari modul `tabs`, fitur `session-tabs`), dan statusbar bawah (panel `statusbar`). Modul `shell` berisi komponen `SidebarArea` dan `StatusBar`. Styles juga mendefinisikan `--term-bg`, `--term-fg`, `--term-cursor` untuk gelap dan terang. Store di `lib/stores/` seperti pada Kontrak, diinisialisasi saat `App.svelte` mount: panggil `ping`/`daemon.info` untuk memastikan koneksi, `loadWorkspaces()`, `provider.list`, lalu `session.list` untuk workspace aktif, dan berlangganan event IPC (`session.exit` dsb.) agar store ter-update tanpa polling. Bila koneksi daemon gagal, tampilkan layar error singkat dengan tombol "Coba lagi". Styles di `frontend/src/styles/` memakai CSS variables untuk warna, otomatis gelap/terang lewat `prefers-color-scheme`, tanpa framework CSS berat. Perilaku tab (placeholder tipe tak dikenal, tutup tab) dimiliki fitur `session-tabs`.

Acceptance criteria:
- [ ] `App.svelte` menampilkan tiga area dan tidak lagi menampilkan teks Hello World.
- [ ] Panel sidebar dan statusbar dirender dari registry sesuai `order`; tanpa panel terdaftar, area kosong tanpa error.
- [ ] `--term-bg`, `--term-fg`, `--term-cursor` terdefinisi untuk gelap dan terang.
- [ ] Store `workspaces`, `sessions`, `tabs`, `providers` ada dengan aksi pada Kontrak.
- [ ] Mengubah `activeProjectId` tidak mengubah isi `sessions`.
- [ ] Store ter-update lewat event IPC dan aksi sendiri, tanpa `setInterval`.
- [ ] Gelap/terang bekerja mengikuti sistem.
- [ ] Kegagalan koneksi daemon ditampilkan sebagai pesan dengan tombol coba lagi.
- [ ] Build frontend lolos (`vite build`) walaupun modul `workspace` dan `tabs` belum ada (area kosong; impor `TabBar`/`TabContent` dari `modules/tabs/` mengikuti Kontrak), sehingga fitur ini bisa digabung lebih dulu.
- [ ] Tidak mengedit `lib/registry.ts`, `lib/ipc.ts`, `main.ts`.

ownedPaths: `frontend/src/App.svelte`, `frontend/src/modules/shell/`, `frontend/src/lib/stores/`, `frontend/src/styles/`. Fitur ini menyediakan store; `workspace-sidebar` dan `session-tabs` mengimpor store tersebut, jadi pada saat penggabungan, sepakati nama ekspor dari Kontrak (boleh dikerjakan paralel selama nama ekspor mengikuti Kontrak).

### workspace-sidebar: Sidebar workspace dan project

Deskripsi: panel sidebar (`modules/workspace/`) yang menampilkan: (1) pemilih workspace (daftar, buat baru dengan nama, ganti nama, hapus); (2) daftar project dalam workspace aktif, dengan tombol "Tambah project" yang membuka dialog pilih folder (dialog native Wails `OpenDirectoryDialog` lewat runtime Wails; bila tidak tersedia dari frontend, pakai input path teks sebagai cadangan), tombol hapus project, dan penanda project aktif; memilih project hanya mengubah `activeProjectId` (menentukan cwd sesi baru); (3) daftar sesi milik workspace aktif (nama/provider, project asal, tanda belum/ sudah exit), klik membuka atau memfokuskan tab sesi, tombol tutup/kill sesi; daftar sesi ini sama persis ketika berpindah project; (4) tombol "Sesi baru" dengan pilihan provider dari `provider.list` (claude, codex, shell) yang memanggil `createSession(providerId)` memakai workspace dan project aktif, lalu langsung membuka tabnya (`openTab`, dirender oleh `session-tabs`). Bila workspace belum punya project, tombol sesi baru dinonaktifkan dengan petunjuk "Tambah project dulu". Modul mendaftar diri di `modules/workspace/index.ts` lewat `registerPanel`.

Acceptance criteria:
- [ ] Workspace bisa dibuat, dipilih, diganti nama, dihapus dari sidebar.
- [ ] Project bisa ditambah lewat pilih folder dan dihapus; daftar mengikuti workspace aktif.
- [ ] Daftar sesi hanya bergantung pada workspace aktif: berpindah antar project tidak mengubah daftar sesi (anti-goal 2 terpenuhi).
- [ ] Sesi baru dibuat dengan provider terpilih dan cwd = path project aktif, dan tab terminalnya terbuka.
- [ ] Tombol sesi baru nonaktif bila tidak ada project.
- [ ] Error dari daemon (mis. ERR_INVALID, ERR_NOT_FOUND) tampil sebagai pesan singkat di sidebar, tidak membuat UI crash.
- [ ] Panel terdaftar lewat `registerPanel` area `sidebar`, tanpa mengedit file di luar `modules/workspace/`.
- [ ] Build frontend lolos.

ownedPaths: `frontend/src/modules/workspace/`. Paralel dengan fitur lain; bergantung hanya pada nama store di Kontrak.

### session-tabs: Banyak tab sesi di atas terminal tiket 02

Deskripsi: modul `tabs` (`modules/tabs/`) berisi `TabBar` dan `TabContent` yang merender daftar tab dari store `tabs` dan memilih komponen dari registry tipe tab berdasar `type`. Tab `session` memakai `TerminalTab` dari tiket 02 (prop `sessionId = refId`); tidak ada kode xterm, attach, subscribe, base64, atau resize baru di sini, semuanya sudah dilakukan `TerminalTab`. Fitur ini hanya mengatur: (1) banyak tab sekaligus dengan tab aktif dan tab bar (judul tab dari provider dan project/cwd sesi, penanda sesi berakhir saat `session.exit` lewat store); (2) hanya tab aktif yang di-mount (`TerminalTab` di-destroy saat tab tidak aktif dan di-mount lagi saat dibuka; scrollback dipulihkan dari daemon lewat attach milik `TerminalTab`) untuk menghemat RAM; (3) tutup tab (`closeTab`) tidak mematikan sesi, sesi tetap hidup dan bisa dibuka lagi dari sidebar; (4) tipe tab tak terdaftar (mis. `file` sebelum tiket 07) menampilkan placeholder "tipe tab tidak tersedia", bukan error; (5) pembersihan: hapus pendaftaran panel `terminal-quick` dari `modules/terminal/index.ts` dan hapus `QuickTerminal.svelte` (panel sementara tiket 02 yang digantikan sidebar dan tab bar); jangan ubah `TerminalTab.svelte` dan `terminal-client.ts`.

Acceptance criteria:
- [ ] Beberapa sesi bisa dibuka sebagai tab sekaligus; berpindah tab menampilkan terminal sesi yang benar dengan scrollback.
- [ ] Hanya tab aktif yang memiliki instance terminal; tab tidak aktif tidak.
- [ ] Menutup tab tidak mematikan sesi; membuka ulang dari sidebar menampilkan scrollback lagi.
- [ ] `session.exit` menandai tab sebagai berakhir tanpa error (terminal sendiri menampilkan "[sesi berakhir]" lewat `TerminalTab`).
- [ ] Tipe tab tak dikenal menampilkan placeholder.
- [ ] Tidak ada kode terminal duplikat dari tiket 02 (tidak ada impor `@xterm/*` di `modules/tabs/`).
- [ ] Panel `terminal-quick` dan `QuickTerminal.svelte` sudah dihapus; `TerminalTab.svelte` dan `terminal-client.ts` tidak berubah.
- [ ] Build frontend lolos.

ownedPaths: `frontend/src/modules/tabs/`. Sentuhan terbatas pada milik tiket 02: `frontend/src/modules/terminal/index.ts` (hapus panel `terminal-quick`) dan `frontend/src/modules/terminal/QuickTerminal.svelte` (hapus). Paralel dengan fitur lain; bergantung pada nama store `tabs`, `sessions` dari Kontrak dan `TerminalTab` dari tiket 02.

## Path yang dimiliki

Hanya path berikut yang boleh disentuh epic ini:
- `frontend/src/App.svelte`
- `frontend/src/modules/shell/`
- `frontend/src/modules/tabs/`
- `frontend/src/modules/workspace/`
- `frontend/src/lib/stores/`
- `frontend/src/styles/`
- `app/daemon.go`
- Sentuhan terbatas pada milik tiket 02: `frontend/src/modules/terminal/index.ts` (hapus panel `terminal-quick`) dan `frontend/src/modules/terminal/QuickTerminal.svelte` (hapus).

## Wiring

- Tidak ada file bersama yang diedit. Modul frontend ditemukan otomatis oleh `import.meta.glob` di `main.ts` (tiket 01): cukup ada `index.ts` di `modules/shell/` (bila dibutuhkan) dan `modules/workspace/` yang memanggil `registerPanel`. Tipe tab `session` sudah didaftarkan oleh `modules/terminal/index.ts` (tiket 02). `modules/tabs/` berisi komponen internal yang diimpor `App.svelte`, tanpa pendaftaran.
- `App.svelte` mengimpor store dari `lib/stores/` dan membaca registry dari `lib/registry.ts`; tidak ada panggilan langsung ke Wails selain lewat `lib/ipc.ts` (dan dialog folder lewat runtime Wails).
- Hook startup Go: `app/daemon.go` mendaftarkan `EnsureDaemon` dari `init()` lewat fungsi pendaftaran startup yang diasumsikan disediakan `app/bridge.go` tiket 01 (misalnya `RegisterStartup(func(ctx context.Context) error)`, dipanggil di Wails OnStartup oleh `app/main.go` sebelum UI memuat). Verifikasi nama persisnya dengan membaca `app/bridge.go` terlebih dulu. Bila hook semacam itu tidak ada, jangan edit `app/main.go`/`bridge.go`: sediakan `EnsureDaemon` saja, pastikan build tetap lolos, dan laporkan sebagai kekurangan di tiket 01 (UI tetap menampilkan layar error + "Coba lagi" bila daemon belum hidup).
- Dependency `@xterm/xterm` dan `@xterm/addon-fit` ditambahkan oleh tiket 02 di `frontend/package.json`; tiket ini tidak menyentuhnya. Laporkan bila belum ada.
- Panel dari tiket sesudahnya (file viewer, status, mention) muncul otomatis di area yang sesuai tanpa perubahan di epic ini.

## Hasil yang bisa dijalankan

Setelah tiket ini (di atas terminal dasar tiket 02, yang sebelumnya hanya menampilkan satu tab): buka `build/bin/aiterm`; daemon menyala otomatis bila belum jalan; sidebar menampilkan workspace dan project (tambah folder lewat dialog); "Sesi baru" dengan pilihan provider (claude, codex, shell) membuat sesi di cwd project aktif dan membukanya sebagai tab baru; beberapa sesi terbuka sebagai banyak tab, menutup tab tidak mematikan sesi dan sesi bisa dibuka lagi dari sidebar; berpindah project tidak mengubah daftar sesi; tema mengikuti gelap/terang sistem. Terminal di setiap tab adalah `TerminalTab` dari tiket 02.

## Guard

Cukup build berhasil, tanpa unit test:
- `go build ./...` (mencakup `app/daemon.go`).
- Build frontend (`vite build`, termasuk type check bila dikonfigurasi di tiket 01).
- Seluruhnya lewat `make build`; harus lolos setelah tiap fitur digabung, termasuk bila hanya sebagian fitur epic ini yang sudah ada.
