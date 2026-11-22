---
id: welcome-screen-ticket-01-session-workspaces
title: Workspace terbuka per sesi
spec: docs/superpowers/specs/2026-11-22-welcome-screen-design.md
status: implemented
wave: 1
depends_on: []
---
# Workspace terbuka per sesi

## Tujuan
Saat ini aplikasi langsung mengaktifkan workspace pertama (atau yang tersimpan di UI state) dan topbar memuat semua workspace lama. Spec welcome-screen meminta tampilan awal kosong: tanpa tab dan tanpa workspace aktif, dengan tab hanya untuk workspace yang dibuka di sesi aplikasi itu. Epic ini menyiapkan fondasinya: backend mencatat `lastOpenedAt` lewat RPC `workspace.touch`, frontend memakai set workspace terbuka per sesi (`openWorkspaceIds`, hanya memori), dan menu tab mendapat aksi "Tutup" di samping "Hapus". Layar welcome sendiri dikerjakan di tiket 02 (`welcome-screen-ticket-02-welcome-screen`).

## Hasil yang bisa dijalankan
Tiket ini berdiri sendiri (wave 1, tanpa dependensi). Setelah selesai, `go build ./...` dan build frontend lulus, daemon dan aplikasi berjalan, dan:
- Saat aplikasi dibuka, topbar tidak menampilkan tab dan tidak ada workspace aktif. Area tengah dan panel lain kosong tanpa crash dan tanpa error di konsol (layar welcome belum ada; kosong sudah cukup di tiket ini).
- Tombol "+" di topbar membuat workspace baru, membukanya sebagai tab aktif, dan masuk mode ganti nama.
- Klik kanan pada tab memunculkan menu: Ganti nama, Tutup, Hapus. "Tutup" menyembunyikan tab tanpa menghapus data (tanpa konfirmasi). "Hapus" tetap meminta konfirmasi seperti sekarang.
- Menutup tab aktif memilih tab tetangga; menutup tab terakhir membuat tidak ada workspace aktif (topbar kosong). Dari keadaan itu "+" tetap bisa membuat workspace.
- Membuka folder lewat alur "folder baru jadi workspace" (`newWorkspaceFromFolder`) menambah tab dan mengaktifkannya.
- `shikigamictl workspace touch <id>` mengembalikan workspace dengan `lastOpenedAt` terisi, dan `workspace.list` memuat field itu.
- Workspace lama (belum pernah dibuka sejak tiket ini) tidak muncul sebagai tab saat start; ia muncul lagi hanya bila dibuat/dibuka di sesi itu. Catatan: pada tiket ini belum ada jalur UI untuk membuka kembali workspace lama yang sudah ditutup atau tersimpan (jalur itu hadir di tiket 02); itu disengaja.

## Batasan
In scope:
- Field `LastOpenedAt`, `Store.Touch`, RPC `workspace.touch`, event `workspace.changed` kind `updated`, subperintah debug `shikigamictl workspace touch`.
- Tipe TS `lastOpenedAt?: number`, store `openWorkspaceIds`, fungsi `openWorkspace` dan `closeWorkspaceTab`, perubahan `loadWorkspaces` dan `deleteWorkspace`.
- Start tanpa workspace aktif, termasuk menonaktifkan pemulihan `activeWorkspaceId` dari UI state saat startup (`modules/persist/index.ts`, lihat fitur `open-set-store`).
- Audit dan guard null pada semua konsumen `activeWorkspace`/`activeWorkspaceId`.
- `WorkspaceTabs.svelte` (hanya tab terbuka, menu Tutup) dan `newWorkspaceFromFolder`.

Out of scope:
- Komponen `Welcome.svelte`, daftar "Terakhir dibuka", tombol Open Workspace dan Create New Workspace di area tengah, dan perubahan `App.svelte` (tiket 02).
- Memulihkan tab/workspace terbuka antar-launch (ditolak spec); `openWorkspaceIds` tidak dipersist.
- Mengubah isi layout/sesi per workspace atau perilaku saat workspace ada.
- Unit test dan dependency baru.

## Kontrak
Disediakan (dipakai tiket 02 apa adanya):
- RPC `workspace.touch` params `{ id: string }` -> objek `Workspace` penuh dengan `lastOpenedAt` = unix ms sekarang. Persist ke disk. Publish `workspace.changed` `{ kind: "updated", workspaceId, workspace }`. Id tak dikenal -> error not found lewat `mapErr` yang sama dengan RPC workspace lain.
- Go: `Workspace.LastOpenedAt int64` dengan tag `json:"lastOpenedAt"` (0 untuk data lama; data JSON lama tanpa field itu terbaca 0). `Store.Touch(id string) (Workspace, error)`.
- TS: `Workspace.lastOpenedAt?: number` di `frontend/src/lib/stores/types.ts`.
- Di `frontend/src/lib/stores/workspaces.ts` (diekspor juga lewat `lib/stores/index.ts` bila barrel perlu):
  - `openWorkspaceIds: Writable<string[]>` (memori, urutan = urutan tab).
  - `openWorkspace(id: string): Promise<void>`: tambah ke `openWorkspaceIds` bila belum ada, `selectWorkspace(id)`, lalu `workspace.touch` best-effort (gagal touch tidak membatalkan pembukaan; entri `workspaces` diperbarui dari hasil touch).
  - `closeWorkspaceTab(id: string): Promise<void>`: keluarkan dari `openWorkspaceIds`; bila aktif, `selectWorkspace` tetangga atau set `null` bila kosong; tidak memanggil `workspace.delete`.
  - `deleteWorkspace(id)` ikut mengeluarkan id dari `openWorkspaceIds`.
  - `loadWorkspaces()` tidak lagi memilih workspace pertama: start -> `activeWorkspaceId = null`, `activeProjectId = null`, `sessions = []`.
- `newWorkspaceFromFolder(path)` di `modules/openfolder/logic.ts` memanggil `openWorkspace` (bukan `selectWorkspace`).
- Invarian untuk tiket 02: semua konsumen `activeWorkspace`/`activeWorkspaceId` aman untuk `null`; `restoreWorkspace(null)` tidak melempar.

Dipakai (sudah ada): `selectWorkspace`, `createWorkspace`, `renameWorkspace`, `loadSessions`, `call` dari `lib/ipc`, `mapErr`/`decode`/`publish` di `internal/workspace/ipc.go`, pola copy-on-write `Rename` + `persistLocked` + `cloneWorkspaces`, `defaultWorkspaceName` dan `errText` di `modules/topbar/util`.

## Fitur

Urutan internal: `workspace-touch-go`, `open-set-store`, dan `null-workspace-safety` saling paralel (foundation; tidak berbagi file). `tabs-close-menu` dikerjakan terakhir karena memakai store dari `open-set-store` dan RPC dari `workspace-touch-go`.

### workspace-touch-go: Field LastOpenedAt dan RPC workspace.touch
Di `internal/workspace/types.go` tambah `LastOpenedAt int64 `json:"lastOpenedAt"`` pada struct `Workspace` (unix ms; 0 untuk data lama). Pastikan `cloneWorkspace` (salinan struct `c := w`) ikut membawa field, load data lama tanpa field itu tetap valid (nilai nol), dan persist menulis field baru. Di `store.go` tambah `Store.Touch(id)`: kunci `s.mu`, cari `indexLocked(id)` (tidak ada -> `fmt.Errorf("%w: workspace %q", ErrNotFound, id)`), `cloneWorkspaces` lalu set `next[i].LastOpenedAt = time.Now().UnixMilli()`, `persistLocked(next, s.ui)`, assign `s.workspaces = next`, kembalikan `cloneWorkspace(next[i])`. Daftarkan RPC `workspace.touch` params `{ id }`: sebaiknya implementasi handler di file baru `internal/workspace/touch.go` (fungsi `registerTouch(srv, s)`), dan `ipc.go` hanya menambah satu baris panggilan di `Register()`. Handler memakai `decode`, `mapErr`, `publish(srv, "updated", ws.ID, &ws)` dan mengembalikan `ws`. `workspace.list` tidak berubah selain field baru. Di `cmd/shikigamictl/cmd_workspace.go` tambah subperintah `touch <id>` (`callAndPrint(c, "workspace.touch", map[string]any{"id": rest[0]})`), perbarui `workspaceUsage` dan `Summary`.
Acceptance criteria:
- [ ] `Workspace.LastOpenedAt` ada dengan tag JSON `lastOpenedAt`; file data lama tanpa field termuat dan `workspace.list` mengembalikan 0.
- [ ] `Store.Touch` mengisi unix ms sekarang, persist, dan mengembalikan salinan workspace; id tak dikenal -> `ErrNotFound`.
- [ ] RPC `workspace.touch {id}` mengembalikan workspace penuh dan mem-publish `workspace.changed` kind `updated`; id tak dikenal menghasilkan error not found yang sama dengan `workspace.rename`.
- [ ] `ipc.go` hanya bertambah satu baris `Register` pemanggil; handler ada di `touch.go`.
- [ ] `shikigamictl workspace touch <id>` mencetak workspace dengan `lastOpenedAt` terisi; argumen salah menampilkan pesan pemakaian.
- [ ] `go build ./...` lulus dan daemon berjalan.
ownedPaths: `internal/workspace/types.go`, `internal/workspace/store.go`, `internal/workspace/ipc.go`, `internal/workspace/touch.go`, `cmd/shikigamictl/cmd_workspace.go`.
Paralel: bisa dikerjakan paralel dengan `open-set-store` dan `null-workspace-safety`.

### open-set-store: Store openWorkspaceIds, openWorkspace, closeWorkspaceTab
Di `frontend/src/lib/stores/types.ts` tambah `lastOpenedAt?: number` ke `Workspace`. Di `workspaces.ts`:
- `openWorkspaceIds = writable<string[]>([])`.
- `loadWorkspaces()`: set `workspaces`; jangan lagi memilih `list[0]`. Bila `activeWorkspaceId` tidak ada di daftar baru (termasuk saat start), set `activeWorkspaceId = null`, `activeProjectId = null`, `sessions.set([])` (atau `loadSessions()` yang aman untuk null), dan buang id yang hilang dari `openWorkspaceIds`. Bila workspace aktif masih ada, pertahankan perilaku sekarang (validasi `activeProjectId`).
- `openWorkspace(id)`: `openWorkspaceIds.update` tambah di akhir bila belum ada; `await selectWorkspace(id)`; lalu `try { const ws = await call('workspace.touch', { id }); replaceWorkspace(ws) } catch {}` (gagal touch diabaikan). Normalisasi hasil RPC seperti `createWorkspace` (objek langsung atau `{workspace}`).
- `closeWorkspaceTab(id)`: ambil urutan sebelum dihapus, keluarkan id dari `openWorkspaceIds`; bila id aktif, pilih tetangga (tab di kanan, kalau tidak ada tab di kiri) lewat `selectWorkspace`, atau `activeWorkspaceId = null`, `activeProjectId = null`, `sessions.set([])` bila tidak tersisa. Tidak memanggil `workspace.delete`.
- `deleteWorkspace(id)`: setelah `workspace.delete`, keluarkan id dari `openWorkspaceIds`; bila id aktif, pilih tetangga dari workspace terbuka (bukan `rest[0]` dari seluruh daftar), atau null bila tidak ada.
- Ekspor lewat `lib/stores/index.ts` bila barrel perlu.
- `init.ts`: handler `workspace.changed` memanggil `loadWorkspaces()`; pastikan setelah perubahan di atas ia tidak mengaktifkan workspace sendiri (event `updated` dari `workspace.touch` tidak boleh mengubah workspace aktif atau `openWorkspaceIds`).
- `modules/persist/index.ts` (penambahan kepemilikan epic ini, tidak dimiliki epic lain): di `run()` hapus pemilihan `target` dari `ui.activeWorkspaceId` / `list[0]`. Start harus `target = null`: tetap `applyUi(ui)` (lebar/visibilitas sidebar tetap dipulihkan), jangan `selectWorkspace`, panggil `restoreWorkspace(null)`, lalu lanjutkan sisa alur (`enableWrites`, `startWatching`, subscriber `activeWorkspaceId`). Tanpa perubahan ini start akan tetap masuk ke workspace tersimpan dan tujuan epic gagal. `uiPayload()` di `autosave.ts` sudah aman untuk null (menulis `''`); jangan ubah file itu.
Acceptance criteria:
- [ ] Setelah start (daftar workspace tidak kosong) `activeWorkspaceId` dan `activeProjectId` bernilai `null`, `sessions` kosong, `openWorkspaceIds` kosong.
- [ ] `openWorkspace(id)` menambah id sekali saja (idempotent), mengaktifkan workspace, dan memanggil `workspace.touch`; bila touch gagal workspace tetap terbuka dan aktif tanpa error tak tertangani.
- [ ] `closeWorkspaceTab(id)` tidak memanggil `workspace.delete`; menutup tab aktif memilih tetangga, menutup tab terakhir menghasilkan `activeWorkspaceId === null`; menutup tab non-aktif tidak mengubah workspace aktif.
- [ ] `deleteWorkspace` mengeluarkan id dari `openWorkspaceIds` dan memilih tetangga dari workspace terbuka, atau null.
- [ ] Event `workspace.changed` (termasuk `updated` dari touch) tidak mengaktifkan workspace dan tidak menambah `openWorkspaceIds`.
- [ ] `persist/index.ts` tidak lagi memilih workspace saat start; sidebar tetap terpulihkan dan autosave tidak menimpa data workspace.
- [ ] Build frontend lulus (tipe `lastOpenedAt` terdefinisi).
ownedPaths: `frontend/src/lib/stores/workspaces.ts`, `frontend/src/lib/stores/types.ts`, `frontend/src/lib/stores/index.ts`, `frontend/src/lib/stores/init.ts`, `frontend/src/modules/persist/index.ts`.
Paralel: bisa dikerjakan paralel dengan `workspace-touch-go` dan `null-workspace-safety` (RPC `workspace.touch` dipanggil lewat `call`, tidak perlu backend siap untuk build; touch yang gagal ditangani best-effort).

### null-workspace-safety: Bagian UI aman saat tidak ada workspace aktif
Audit (grep `activeWorkspace`, `activeWorkspaceId`, `activeProject`) dan perbaiki konsumen agar tidak crash dan tidak mencetak error di konsol saat nilainya `null`; aksi yang butuh workspace dinonaktifkan atau disembunyikan. Cakupan: sidebar kiri (`modules/sidebar-left`: `WorkspaceHeader`, `AgentSidebar`, `SessionGroups`, `NewSessionMenu`), `modules/workspace`, panel kanan (`modules/sidebar-right`) dan `modules/files` (pohon file kosong tanpa project), `modules/lark/source.ts`, `modules/grimoire`, topbar `CenterSummary.svelte` dan `SessionCounter.svelte`, `modules/layout`, serta `restoreWorkspace(null)` di `lib/stores/groups.ts` dan layout (`lib/stores/sessions.ts` untuk `loadSessions` saat tanpa workspace). Hanya perbaikan defensif null (optional chaining, early return, `disabled`, `{#if}`); perilaku saat workspace ada tidak boleh berubah. Perubahan di tiap file terbatas pada guard null.
Acceptance criteria:
- [ ] Dengan `activeWorkspaceId = null`, tidak ada exception atau error di konsol dari file yang disebut di atas (diverifikasi dengan menjalankan app dari keadaan start kosong).
- [ ] Tombol/aksi yang butuh workspace (sesi baru, tambah project, buka file, Lark/Grimoire) nonaktif atau tersembunyi, bukan melempar error.
- [ ] `restoreWorkspace(null)` dan `loadSessions()` tanpa workspace menghasilkan state kosong tanpa error.
- [ ] Dengan workspace aktif, tampilan dan perilaku identik dengan sebelumnya.
- [ ] Build frontend lulus.
ownedPaths: `frontend/src/modules/sidebar-left`, `frontend/src/modules/sidebar-right`, `frontend/src/modules/workspace`, `frontend/src/modules/files`, `frontend/src/modules/lark/source.ts`, `frontend/src/modules/grimoire`, `frontend/src/modules/topbar/CenterSummary.svelte`, `frontend/src/modules/topbar/SessionCounter.svelte`, `frontend/src/modules/layout`, `frontend/src/lib/stores/groups.ts`, `frontend/src/lib/stores/sessions.ts`.
Paralel: bisa dikerjakan paralel dengan `workspace-touch-go` dan `open-set-store` (tidak berbagi file; state null bisa diuji dengan `activeWorkspaceId.set(null)`).

### tabs-close-menu: WorkspaceTabs hanya tab terbuka dan menu Tutup
Dikerjakan setelah `open-set-store` (dan idealnya `workspace-touch-go`). Di `WorkspaceTabs.svelte`: render `$openWorkspaceIds` berurutan dengan memetakan ke entri `$workspaces` (lewati id yang tidak ditemukan); tanpa tab terbuka tidak ada tab sama sekali, hanya tombol "+". `pick(id)` tetap memakai `selectWorkspace` (tab sudah terbuka). `add()`: `createWorkspace(defaultWorkspaceName($workspaces))`, lalu `openWorkspace(ws.id)`, lalu `startEdit(ws.id, ws.name)`. Menu konteks: Ganti nama, Tutup (`closeWorkspaceTab(id)`, tanpa konfirmasi, tutup menu, error lewat `say(errText(e))`), Hapus (konfirmasi `confirmId` yang sudah ada, memakai `deleteWorkspace`). Sesuaikan tinggi menu pada klamp posisi (`window.innerHeight - 80`) bila item bertambah. Teks UI bahasa Indonesia, atribut aksesibilitas (role menu/menuitem) mengikuti yang sudah ada. Di `modules/openfolder/logic.ts` ubah `newWorkspaceFromFolder` menjadi: `createWorkspace(basename(path))`, `openWorkspace(ws.id)`, lalu `addProject(path)`; hapus ganjalan `selectWorkspace`/`activeWorkspaceId.set` yang tidak lagi perlu (pastikan `addProject` tetap melihat workspace baru sebagai aktif). Tidak membuat file di luar ownedPaths.
Acceptance criteria:
- [ ] Saat start topbar hanya berisi tombol "+" (tanpa tab).
- [ ] Tab hanya untuk id di `openWorkspaceIds`, sesuai urutan; tab baru muncul di kanan.
- [ ] Tombol "+" membuat workspace, membuka dan mengaktifkannya, dan masuk mode ganti nama.
- [ ] Menu tab berisi Ganti nama, Tutup, Hapus; Tutup menyembunyikan tab tanpa konfirmasi dan data workspace tetap (`workspace.list` masih memuatnya); Hapus tetap meminta konfirmasi dan menghapus data.
- [ ] Menutup tab terakhir menghasilkan topbar kosong dan tidak ada workspace aktif tanpa error.
- [ ] `newWorkspaceFromFolder(path)` membuat workspace, membukanya sebagai tab aktif (lewat `openWorkspace`), dan menambahkan project.
- [ ] Build frontend lulus.
ownedPaths: `frontend/src/modules/topbar/WorkspaceTabs.svelte`, `frontend/src/modules/openfolder/logic.ts`.
Paralel: bergantung pada `open-set-store` (store) dan `workspace-touch-go` (RPC, boleh belum siap untuk build karena touch best-effort); kerjakan setelahnya. Dengan `null-workspace-safety` bisa paralel (tidak berbagi file).

## Path yang dimiliki
Hanya path ini yang boleh disentuh epic ini:
- `internal/workspace/types.go`
- `internal/workspace/store.go`
- `internal/workspace/ipc.go`
- `internal/workspace/touch.go`
- `cmd/shikigamictl/cmd_workspace.go`
- `frontend/src/lib/stores/workspaces.ts`
- `frontend/src/lib/stores/types.ts`
- `frontend/src/lib/stores/index.ts`
- `frontend/src/lib/stores/init.ts`
- `frontend/src/lib/stores/groups.ts`
- `frontend/src/lib/stores/sessions.ts`
- `frontend/src/modules/persist/index.ts` (penambahan epic ini; wajib agar start tidak memulihkan workspace tersimpan)
- `frontend/src/modules/topbar/WorkspaceTabs.svelte`
- `frontend/src/modules/topbar/CenterSummary.svelte`
- `frontend/src/modules/topbar/SessionCounter.svelte`
- `frontend/src/modules/openfolder/logic.ts`
- `frontend/src/modules/sidebar-left` (guard null saja)
- `frontend/src/modules/sidebar-right` (guard null saja)
- `frontend/src/modules/workspace` (guard null saja)
- `frontend/src/modules/files` (guard null saja)
- `frontend/src/modules/lark/source.ts` (guard null saja)
- `frontend/src/modules/grimoire` (guard null saja)
- `frontend/src/modules/layout` (guard null saja)

Tidak boleh disentuh (milik tiket 02): `frontend/src/modules/welcome`, `frontend/src/App.svelte`.

## Wiring
Penutup tiket ini adalah fitur `tabs-close-menu`, yang menyambungkan seluruh epic ke aplikasi. Titik sambung: `WorkspaceTabs.svelte` (sudah dirender di topbar) membaca `openWorkspaceIds` dan memanggil `openWorkspace`/`closeWorkspaceTab`; `newWorkspaceFromFolder` memakai `openWorkspace`; `openWorkspace` memanggil RPC `workspace.touch`; `persist/index.ts` dan `loadWorkspaces` memastikan start tanpa workspace aktif. Tidak ada registrasi baru di `App.svelte`. Hasilnya langsung terlihat: aplikasi mulai dengan topbar kosong, "+" membuat dan membuka workspace, menu tab punya Tutup dan Hapus, dan menutup tab terakhir kembali ke keadaan kosong. Daemon harus dibangun ulang agar RPC `workspace.touch` tersedia; bila belum, aplikasi tetap berjalan karena touch best-effort. Tiket ini tidak bergantung pada tiket lain untuk berjalan.

## Guard
Cukup build berhasil: `go build ./...` dan build frontend (`npm run build` di `frontend/`, atau `wails build` bila dipakai proyek). Tanpa unit test.
