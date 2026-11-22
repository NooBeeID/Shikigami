# Welcome screen & workspace terbuka per sesi — design

Ukuran: perubahan kecil-menengah (frontend + satu field/RPC di backend).

## Intent

Saat aplikasi dibuka, tampilan awal harus kosong: tanpa tab workspace dan
tanpa workspace aktif. Pengguna memilih sendiri: Open Workspace (folder dari
disk), Create New Workspace, atau membuka salah satu dari 5 workspace yang
terakhir dibuka. Alasannya: pengguna tidak mau app langsung masuk ke workspace
terakhir atau memenuhi topbar dengan semua workspace lama.

## Goals

- Start → layar welcome, topbar tanpa tab.
- Tiga jalur masuk: Open Workspace, Create New Workspace, daftar "Terakhir dibuka" (maks 5).
- Tab hanya untuk workspace yang dibuka di sesi app itu.
- Menu tab punya "Tutup" (sembunyikan, data tetap) di samping "Hapus".
- Menutup tab terakhir kembali ke welcome.

## Non-goals

- Tidak memulihkan tab/workspace terbuka antar-launch (pendekatan C ditolak).
- Tidak ada daftar lengkap semua workspace tersimpan (hanya 5 terakhir).
- Tidak mengubah isi layout/sesi per workspace.

## Anti-goals

- App yang tetap otomatis mengaktifkan workspace pertama saat start.
- "Tutup" yang diam-diam menghapus data.

## Approaches

- A. Welcome di atas semua tab tersimpan: kecil, tapi topbar tidak kosong. Ditolak.
- **B. Set workspace "terbuka" per sesi di frontend + `lastOpenedAt` di backend. Dipilih.**
- C. Daemon menyimpan & memulihkan tab terbuka: bertentangan dengan "blank saat start". Ditolak.

## Design

Backend (`internal/workspace`):
- Field baru `LastOpenedAt` (`lastOpenedAt`, RFC3339/unix ms; nol untuk data lama) di `Workspace`.
- RPC `workspace.touch {id}` mengisi `LastOpenedAt = now`, persist, mengembalikan workspace; publish event "updated".
- `workspace.list` tidak berubah bentuknya selain field baru.

Frontend:
- Store baru `openWorkspaceIds` (urutan tab; hanya memori, tidak dipersist) di `lib/stores/workspaces.ts`.
- `loadWorkspaces()` tidak lagi memilih workspace pertama. Saat start `activeWorkspaceId = null`.
- `openWorkspace(id)`: tambah ke `openWorkspaceIds` jika belum, `selectWorkspace`, panggil `workspace.touch`.
- `closeWorkspaceTab(id)`: keluarkan dari `openWorkspaceIds`; jika aktif, pilih tetangga, atau `null` bila kosong (→ welcome). Tidak memanggil `workspace.delete`.
- `deleteWorkspace` juga mengeluarkan id dari `openWorkspaceIds`.
- `WorkspaceTabs` merender hanya workspace di `openWorkspaceIds`; menu tab: Ganti nama, Tutup, Hapus.
- Komponen `Welcome.svelte` (modul baru) dirender di area tengah saat `activeWorkspaceId === null`: dua tombol dan daftar terakhir.
- Open Workspace: `pickDirectory()`. Jika path sudah menjadi project di workspace tersimpan → `openWorkspace` workspace itu (cocokkan path ternormalisasi). Jika belum → `newWorkspaceFromFolder(path)` (logika yang ada) lalu `openWorkspace`.
- Create New Workspace: `createWorkspace(defaultWorkspaceName)` → `openWorkspace` → mode ganti nama (seperti tombol "+" sekarang).
- Terakhir dibuka: workspace dengan `lastOpenedAt` > 0, belum terbuka, urut menurun, 5 teratas. Baris: nama + path project pertama. Kosong → teks "Belum ada workspace terakhir".
- Tombol "+" di topbar tetap membuat workspace baru (ikut `openWorkspace`).
- Bagian lain yang mengandalkan `activeWorkspace` harus aman untuk `null` (sidebar, restore layout: `restoreWorkspace(null)` sudah ada).

## Catatan

- Workspace lama belum punya `lastOpenedAt`, jadi baru muncul di daftar terakhir setelah pernah dibuka sekali.

## Diserahkan ke builder

Tampilan visual welcome (spasi, ikon), teks kosong, shortcut keyboard, dan
pengujian (store, `workspace.touch`, logika "terakhir dibuka").
