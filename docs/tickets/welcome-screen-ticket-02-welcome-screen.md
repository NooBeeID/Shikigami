---
id: welcome-screen-ticket-02-welcome-screen
title: Layar welcome dan terakhir dibuka
spec: docs/superpowers/specs/2026-11-22-welcome-screen-design.md
status: ready
wave: 2
depends_on: [welcome-screen-ticket-01-session-workspaces]
---
# Layar welcome dan terakhir dibuka

## Tujuan
Setelah tiket 1, aplikasi start tanpa workspace aktif dan tanpa tab, tetapi area tengah masih kosong atau memakai tampilan lama. Epic ini menambahkan layar welcome di area tengah yang muncul tepat saat `activeWorkspaceId === null`. Dari layar itu pengguna memilih sendiri jalur masuk: Open Workspace (folder dari disk), Create New Workspace, atau salah satu dari maksimal 5 workspace yang terakhir dibuka. Menutup tab terakhir juga kembali ke layar ini.

## Hasil yang bisa dijalankan
Di atas tiket 1, setelah tiket ini selesai:
- Start aplikasi: area tengah menampilkan layar welcome (dua tombol dan bagian "Terakhir dibuka"), topbar tanpa tab.
- Klik "Open Workspace": dialog pilih folder muncul. Batal -> tidak terjadi apa-apa. Pilih folder yang path-nya sudah menjadi project di workspace tersimpan -> workspace itu dibuka sebagai tab. Pilih folder baru -> workspace baru bernama folder itu dibuat berisi project tersebut dan dibuka.
- Klik "Create New Workspace": workspace baru dengan nama default ("Workspace N") dibuat; pengguna dapat langsung mengganti namanya lewat input inline, lalu workspace terbuka sebagai tab.
- Bagian "Terakhir dibuka" memuat maksimal 5 workspace yang pernah dibuka (`lastOpenedAt > 0`) dan belum terbuka sebagai tab, terbaru dulu; tiap baris menampilkan nama dan path project pertama. Klik baris membuka workspace itu. Bila kosong tampil "Belum ada workspace terakhir".
- Workspace yang sudah dibuka sekali muncul di daftar pada peluncuran berikutnya atau setelah tab-nya ditutup.
- Menutup tab terakhir (menu tab "Tutup" dari tiket 1) mengembalikan layar welcome; workspace yang baru ditutup muncul di "Terakhir dibuka".
- `go build ./...` dan build frontend lulus; aplikasi berjalan.

## Batasan
In scope:
- Modul baru `frontend/src/modules/welcome/` (`logic.ts`, `Welcome.svelte`, `index.ts`).
- Pengubahan `frontend/src/App.svelte` agar snippet `center` memilih `Welcome` atau `LayoutArea`.
- Util `recentWorkspaces` dan `findWorkspaceByPath`, fungsi aksi `openFromDisk` dan `createNew`.

Out of scope:
- Backend Go (`workspace.touch`, `lastOpenedAt`) dan store `openWorkspaceIds`/`openWorkspace`/`closeWorkspaceTab`: milik tiket 1, hanya dipakai.
- `WorkspaceTabs.svelte`, `lib/stores/*`, `openfolder/logic.ts`: tidak diubah. Jangan mengubah mekanisme ganti nama di `WorkspaceTabs` (state `editingId` bersifat lokal komponen itu).
- Daftar lengkap semua workspace tersimpan, pemulihan tab antar-launch, shortcut keyboard, unit test, dependensi baru.
- Menyesuaikan konsumen `activeWorkspace` lain agar aman untuk `null` (sudah menjadi tanggung jawab tiket 1).

## Kontrak
Disediakan:
- Komponen `Welcome.svelte` (default export di `modules/welcome/index.ts` sebagai `Welcome`), tanpa props, dirender oleh `App.svelte` saat `$activeWorkspaceId === null`.
- `frontend/src/modules/welcome/logic.ts`:
  - `recentWorkspaces(workspaces: Workspace[], openIds: string[], limit = 5): Workspace[]` murni: filter `(lastOpenedAt ?? 0) > 0` dan `!openIds.includes(id)`, urut `lastOpenedAt` menurun, `slice(0, limit)`.
  - `findWorkspaceByPath(workspaces: Workspace[], path: string): Workspace | undefined` murni: bandingkan path ternormalisasi (buang slash akhir `/` atau `\`, kecuali root `/`) dengan `projects[].path` tiap workspace; kembalikan yang pertama cocok.
  - `openFromDisk(): Promise<void>`: `pickDirectory()`; hasil kosong/batal -> return; bila `findWorkspaceByPath(get(workspaces), path)` ada -> `openWorkspace(ws.id)`; bila tidak -> `newWorkspaceFromFolder(path)` (sudah memanggil `openWorkspace`). Error ditangkap dan ditampilkan lewat `showToast(errMessage(e), 'error')`. Guard `busy` agar tidak berjalan ganda.
  - `createNew(): Promise<Workspace | null>`: `createWorkspace(defaultWorkspaceName(get(workspaces)))`; mengembalikan workspace baru, TIDAK membuka (lihat fitur welcome-ui). Error -> toast, kembalikan `null`.
  - `openRecent(id: string): Promise<void>`: `openWorkspace(id)` dengan penanganan error toast.
- Penanda ekspor di `index.ts`: `export { default as Welcome } from './Welcome.svelte'`.

Dipakai (dari tiket 1 dan kode yang ada; periksa nama persis di kode):
- Dari `lib/stores` (tiket 1): `workspaces`, `activeWorkspaceId`, `openWorkspaceIds`, `openWorkspace(id)`, `createWorkspace(name)`, `renameWorkspace(id, name)`, tipe `Workspace` dengan `lastOpenedAt?: number` dan `projects: {path}[]`.
- `newWorkspaceFromFolder(path)`, `basename`, `errMessage` dari `modules/openfolder/logic.ts` (sudah memakai `openWorkspace` setelah tiket 1).
- `pickDirectory()` dari `modules/appmenu`.
- `showToast` dari `modules/openfolder/toast-store` (komponen `Toast` sudah dirender di `App.svelte`).
- `defaultWorkspaceName(list)` dan `errText` dari `modules/topbar/util.ts`.
- Tema: variabel CSS di `styles/theme.css` (mis. `--fg-muted`).

Aturan tetap:
- Teks UI bahasa Indonesia untuk status/kosong: "Belum ada workspace terakhir", "Terakhir dibuka". Label tombol: "Open Workspace" dan "Create New Workspace".
- Identifier kode dalam bahasa Inggris.

## Fitur

### welcome-logic: Logika terakhir dibuka dan pencocokan path
Deskripsi:
Buat `frontend/src/modules/welcome/logic.ts` seperti kontrak di atas. `recentWorkspaces` dan `findWorkspaceByPath` murni (tanpa store) agar mudah diuji nanti. Normalisasi path: ulangi buang `/` atau `\` di akhir selama panjang > 1. Fungsi aksi memakai `get()` dari `svelte/store` untuk membaca `workspaces`. `openFromDisk` memakai flag `busy` modul agar klik ganda saat dialog terbuka diabaikan. Semua error lewat `showToast(errMessage(e), 'error')`; tidak ada error yang dilempar ke UI. Workspace baru dari `createNew` belum ditambahkan ke `openWorkspaceIds` (supaya UI bisa menampilkan input nama dulu dan tidak berpindah layar sebelum waktunya).
Acceptance criteria:
- [ ] `recentWorkspaces` hanya memuat workspace dengan `lastOpenedAt > 0` (undefined/0 dikecualikan) dan id tidak ada di `openIds`.
- [ ] Hasil terurut `lastOpenedAt` menurun dan dipotong ke `limit` (default 5).
- [ ] `findWorkspaceByPath` cocok walau salah satu path memiliki slash akhir; mengembalikan `undefined` bila tidak ada.
- [ ] `openFromDisk`: batal dialog tidak memanggil store apa pun; path yang sudah menjadi project memanggil `openWorkspace` workspace itu tanpa membuat workspace baru; path baru memanggil `newWorkspaceFromFolder`.
- [ ] `createNew` mengembalikan workspace baru bernama default unik, atau `null` + toast bila gagal.
- [ ] Tidak ada perubahan file di luar `modules/welcome/logic.ts`; `npm run build` (atau `npm run check` bila ada) frontend lulus.
ownedPaths:
- `frontend/src/modules/welcome/logic.ts`

Dapat dikerjakan paralel dengan welcome-ui dan welcome-wiring selama kontrak tanda tangan fungsi di atas dipegang; welcome-ui bergantung pada nama ekspor `logic.ts`.

### welcome-ui: Komponen Welcome.svelte
Deskripsi:
Buat `Welcome.svelte` dan `index.ts` di `frontend/src/modules/welcome/`. Tata letak: kolom di tengah area (vertikal dan horizontal), judul singkat (mis. nama aplikasi atau "Selamat datang"), dua tombol berdampingan atau bertumpuk dengan ikon SVG inline (folder untuk Open Workspace, plus untuk Create New Workspace), lalu bagian "Terakhir dibuka". Daftar dihitung reaktif: `recentWorkspaces($workspaces, $openWorkspaceIds)`. Baris berisi nama workspace (tebal) dan path project pertama (`projects[0]?.path`, teks `--fg-muted`, dipotong ellipsis; kosong bila workspace tanpa project). Klik baris -> `openRecent(id)`. Daftar kosong -> teks "Belum ada workspace terakhir".

Create New Workspace dan mode ganti nama: `WorkspaceTabs.svelte` tidak boleh diubah, dan `editingId`-nya lokal, sehingga mode ganti nama dilakukan inline di Welcome SEBELUM workspace dibuka. Alur: klik tombol -> `createNew()` -> bila hasil bukan `null`, Welcome menampilkan input teks (terisi nama default, fokus dan terseleksi, pola seperti `focusSelect` di `WorkspaceTabs`) di tempat tombol. Enter atau blur -> bila nama (di-trim) tidak kosong dan beda, `renameWorkspace(id, name)` (error -> toast, tetap lanjut); lalu `openWorkspace(id)`. Escape -> batalkan penggantian nama tetapi tetap `openWorkspace(id)` dengan nama default (workspace sudah terlanjur dibuat). Guard `committing` agar Enter lalu blur tidak memproses dua kali. Selama input aktif, tombol lain dinonaktifkan.

Visual: mengikuti variabel tema yang ada (`--fg`, `--fg-muted`, `--bg*`, `--border` atau padanannya di `styles/theme.css`; periksa nama persis). Komponen memenuhi tinggi area (`height: 100%`), tidak menambah scrollbar horizontal, daftar tidak melebihi 5 baris sehingga tidak perlu scroll. Tombol memakai elemen `<button>` dengan `aria-label`/teks jelas; fokus keyboard terlihat. Gunakan Svelte 5 (runes `$state`, `onclick`) sesuai kode yang ada.
Acceptance criteria:
- [ ] Dua tombol "Open Workspace" dan "Create New Workspace" tampil dengan ikon, dan memanggil `openFromDisk()` dan alur `createNew()`.
- [ ] Bagian "Terakhir dibuka" menampilkan nama + path project pertama untuk tiap entri, maksimal 5, memperbarui diri saat `workspaces` atau `openWorkspaceIds` berubah.
- [ ] Teks "Belum ada workspace terakhir" tampil saat daftar kosong.
- [ ] Klik baris membuka workspace itu (tab muncul, Welcome hilang).
- [ ] Setelah Create New, input nama inline muncul terfokus; Enter/blur mengganti nama dan membuka tab dengan nama itu; Escape membuka tab dengan nama default; tidak ada pemrosesan ganda.
- [ ] Tampilan konsisten dengan tema terang/gelap yang ada; tidak ada warna hard-coded yang bertabrakan.
- [ ] `index.ts` mengekspor `Welcome`; build frontend lulus.
ownedPaths:
- `frontend/src/modules/welcome/Welcome.svelte`
- `frontend/src/modules/welcome/index.ts`

Bergantung pada nama ekspor `logic.ts` (kontrak di atas), tetapi dapat dikerjakan paralel dengan welcome-logic bila kontrak tanda tangan dipegang; welcome-wiring bergantung pada `Welcome` dari `index.ts`.

### welcome-wiring: Render Welcome saat tidak ada workspace aktif
Deskripsi:
Di `frontend/src/App.svelte` impor `activeWorkspaceId` dari `./lib/stores` dan `Welcome` dari `./modules/welcome`. Ubah snippet `center` menjadi:
`{#snippet center()}{#if $activeWorkspaceId === null}<Welcome />{:else}<LayoutArea />{/if}{/snippet}`.
Jangan mengubah bagian App.svelte yang lain (koneksi, toast, palette, settings). Lalu verifikasi end-to-end secara manual di aplikasi (lihat bagian Wiring). Bila ada konsumen `activeWorkspace` yang crash saat `null` (mis. error konsol saat welcome), catat sebagai temuan untuk tiket 1; jangan memperbaikinya di luar path milik tiket ini kecuali berada di `App.svelte`.
Acceptance criteria:
- [ ] Saat `activeWorkspaceId === null`, area tengah merender `Welcome`; selain itu `LayoutArea`.
- [ ] Tidak ada error di konsol saat welcome tampil (topbar, sidebar, panel kanan tidak crash karena workspace null).
- [ ] Alur: start -> welcome tanpa tab; Open Workspace dari disk; Create New; buka dari Terakhir dibuka; tutup tab terakhir -> welcome kembali, semuanya berfungsi.
- [ ] `go build ./...` dan build frontend lulus.
ownedPaths:
- `frontend/src/App.svelte`

Bergantung pada `Welcome` dari fitur welcome-ui untuk dapat dikompilasi; dikerjakan terakhir (setelah welcome-logic dan welcome-ui).

## Path yang dimiliki
- `frontend/src/modules/welcome/logic.ts`
- `frontend/src/modules/welcome/Welcome.svelte`
- `frontend/src/modules/welcome/index.ts`
- `frontend/src/App.svelte`

Hanya path ini yang boleh disentuh oleh tiket ini.

## Wiring
Langkah penutup tiket ini (fitur welcome-wiring): di `App.svelte`, snippet `center` merender `<Welcome />` bila `$activeWorkspaceId === null`, selain itu `<LayoutArea />`. Titik ekstensi yang dipakai dari tiket 1 adalah state-nya saja (`activeWorkspaceId` bernilai `null` saat start dan setelah `closeWorkspaceTab` menutup tab terakhir; `openWorkspaceIds`, `openWorkspace`, `lastOpenedAt` via `workspace.touch`). Tidak ada registry yang perlu didaftarkan; `Welcome` tidak memakai `registerPanel` karena menggantikan area tengah, bukan menambah panel.

Verifikasi manual di aplikasi yang berjalan (tiket 1 harus sudah terimplementasi):
1. Jalankan app: layar welcome, tanpa tab di topbar, "Belum ada workspace terakhir" bila belum ada riwayat.
2. Open Workspace -> pilih folder baru -> tab workspace baru dengan nama folder, welcome hilang.
3. Tutup tab (menu "Tutup") -> welcome kembali; workspace tadi muncul di "Terakhir dibuka" dengan path project pertama.
4. Klik baris itu -> tab terbuka lagi; entri hilang dari daftar (karena sudah terbuka).
5. Open Workspace -> pilih folder yang sama (juga varian dengan slash akhir) -> workspace yang ada dibuka, tidak ada workspace duplikat.
6. Create New Workspace -> input nama inline -> ganti nama -> tab terbuka dengan nama baru.
7. Buka lebih dari 5 workspace lalu tutup semua -> daftar hanya menampilkan 5 terbaru.
8. Restart app -> kembali ke welcome tanpa tab, daftar Terakhir dibuka terisi dari riwayat.

## Guard
Cukup build berhasil, tanpa unit test:
- `go build ./...`
- Build frontend (`cd frontend && npm run build`, dan `npm run check` bila skripnya ada).
