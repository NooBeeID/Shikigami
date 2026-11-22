---
id: main-rail-ticket-01-main-rail
title: Rail menu utama dan panel kiri
spec: docs/superpowers/specs/2026-11-22-08-main-rail-design.md
status: ready
wave: 1
depends_on: []
---
# Rail menu utama dan panel kiri

## Tujuan
Sidebar kiri saat ini hanya berisi `AgentSidebar` dan tidak punya tempat untuk fitur baru (Lark, Git). Epic ini menambah rail ikon 48px di tepi kiri `ShellFrame` sebagai menu utama, plus panel kiri yang isinya mengikuti item rail aktif. Item didaftarkan lewat registry `registerRailItem` (pola `registerRightTab`) supaya modul lain bisa menambah item tanpa mengubah shell. Pada tiket ini hanya item `agents` yang terdaftar, dan Threads masih menempel sebagai section di dalam `AgentSidebar` sehingga aplikasi tetap utuh.

## Hasil yang bisa dijalankan
Setelah tiket ini selesai (tiket pertama, tanpa tiket sebelumnya), `go build ./...` dan build frontend lulus, aplikasi berjalan, dan:
- Di tepi kiri jendela tampak rail 48px dengan satu ikon "Agen" yang berstatus aktif; di sebelahnya panel kiri menampilkan `AgentSidebar` persis seperti sebelumnya (termasuk section Threads).
- Tombol rail bisa difokus dengan Tab dan dipilih dengan Enter/Spasi; `aria-label` "Agen", `aria-current` pada item aktif.
- Toggle sidebar kiri (shortcut yang ada) hanya menyembunyikan panel; rail 48px tetap tampil. Resizer kiri tetap mengatur lebar panel.
- Pilihan item aktif tersimpan di state UI dan bertahan setelah restart. State UI lama tanpa `activeRail` terbaca sebagai `agents`.
- Sesi terminal dan thread yang berjalan tidak terputus.

## Batasan
In scope:
- Registry item rail, store `activeRail`, resolusi fallback ke `agents`.
- Field Go `ActiveRail` pada `SidebarState` dan jalur persist/restore frontend.
- Komponen `Rail.svelte`, `RailPanelHost.svelte`, grid empat kolom `ShellFrame`, CSS.
- Item bawaan `agents` dan wiring `App.svelte`.

Out of scope:
- Memindahkan Threads menjadi item rail sendiri dan membersihkan `AgentSidebar` (tiket main-rail-ticket-02-threads-rail).
- Isi Lark dan Git, deteksi remote Git (desain lain); tidak ada item placeholder Git/Lark.
- Panel kanan, area layout/pane, dan file `layout/*`, `lib/stores/layout.ts`, `provider/*`, `thread/*` (perubahan belum di-commit di sana tidak disentuh).
- Drag-reorder atau pengaturan item rail, resize rail.
- Unit test dan dependency baru.

Prasyarat worktree: perubahan belum di-commit di `layout/*`, `lib/stores/layout.ts`, `provider/*`, dan `thread/participants/terminal.ts` harus di-commit atau dipindah sebelum implementasi dimulai agar worktree mulai dari dasar bersih.

## Kontrak
Disediakan:
- `frontend/src/modules/shell/registry.ts`:
  `interface RailItemSpec { id: string; title: string; icon: Component; component: Component<any>; order?: number; visible?: Readable<boolean> }`, `registerRailItem(spec)`, `railItems(): Readable<RailItemSpec[]>`. `railItems` hanya mengembalikan item visible (tanpa `visible` = selalu tampil), terurut `order` (default 0) lalu `id`. Panel dirender dengan props `{}` dan mengambil state dari store; hanya panel aktif yang di-mount.
- `frontend/src/lib/stores/shell.ts`: `SidebarState` diperluas `activeRail?: string`; `leftSidebar` = `{ visible, width, activeRail }` default `activeRail: 'agents'`; `setActiveRail(id)`; `resolveActiveRail(items, id)` mengembalikan `'agents'` bila id tak dikenal atau item tak visible.
- Go `internal/workspace/types.go`: `SidebarState.ActiveRail string `json:"activeRail,omitempty"`` (bermakna hanya untuk sidebar kiri).
- Item bawaan `agents` (title "Agen", order 0, komponen `AgentSidebar`).
- `Rail.svelte` dan `RailPanelHost.svelte`.

Konvensi untuk tiket lain: order agents=0, lark=10, git-github/git-gitlab=20/21, threads=30. Ikon berupa komponen Svelte SVG inline tanpa dependensi baru; teks UI bahasa Indonesia. Subscriber event thread (`subscribeThreadEvents` di `modules/thread/index.ts`) didaftarkan saat modul diimpor, bukan saat komponen mount, dan tidak boleh dipindah ke komponen panel.

Dipakai: `Resizer.svelte`, `toggleLeft`, `setLeftWidth`, jalur patch UI yang ada (`leftSidebar` di autosave), `AgentSidebar`.

## Fitur

Urutan internal: `rail-registry-store` dan `rail-state-persist-go` dulu (foundation, saling paralel), lalu `rail-state-persist-ui` dan `rail-ui` paralel, terakhir `agents-rail-item` (wiring).

### rail-registry-store: Registry item rail dan state activeRail
Di `modules/shell/registry.ts` tambah `RailItemSpec`, `registerRailItem`, dan `railItems()`. `railItems()` menggabungkan store `visible` tiap item (mis. `derived` dari array store, atau gabungan rekursif) lalu menyaring yang visible dan mengurutkan menurut `order` lalu `id`. Di `lib/stores/shell.ts` tambah `activeRail` ke state `leftSidebar` (default `'agents'`), `setActiveRail(id)`, dan `resolveActiveRail(items, id)`. `SidebarState` diberi `activeRail?: string` agar kode lain tetap kompatibel. Ekspor via `modules/shell/index.ts` bila perlu. Tidak menyentuh UI.
Acceptance criteria:
- [ ] `registerRailItem` dan `railItems` terekspor; item tanpa `visible` selalu muncul.
- [ ] Item dengan `visible` false tidak muncul dan muncul lagi saat store berubah.
- [ ] Urutan hasil: `order` naik, lalu `id`.
- [ ] `leftSidebar` default `activeRail: 'agents'`; `setActiveRail` mengubah field tanpa mengubah `visible`/`width`.
- [ ] `resolveActiveRail` mengembalikan `'agents'` untuk id kosong, tak dikenal, atau tak visible.
- [ ] Build frontend lulus.
ownedPaths: `frontend/src/modules/shell/registry.ts`, `frontend/src/lib/stores/shell.ts`, `frontend/src/modules/shell/index.ts`.
Paralel dengan `rail-state-persist-go`; mendahului `rail-state-persist-ui`, `rail-ui`, `agents-rail-item`.

### rail-state-persist-go: Field activeRail di UIState (daemon)
Di `internal/workspace/types.go` tambah `ActiveRail string `json:"activeRail,omitempty"`` pada `SidebarState`. `defaultLeftSidebar()` mengisi `ActiveRail: "agents"`. `UIPatch` membawa `*SidebarState` utuh sehingga field baru ikut; periksa `store.go`/`ipc.go` bahwa merge patch UI tidak menjatuhkan field baru dan edit `store.go` hanya bila memang menjatuhkannya. JSON lama tanpa `activeRail` tetap terbaca (kosong dianggap `agents` oleh frontend).
Acceptance criteria:
- [ ] Field `ActiveRail` ada dengan tag json `activeRail,omitempty`.
- [ ] Default sidebar kiri berisi `ActiveRail: "agents"`.
- [ ] Patch UI yang membawa `activeRail` tersimpan dan terbaca kembali setelah restart daemon.
- [ ] File state lama tanpa `activeRail` tetap dimuat tanpa error.
- [ ] `go build ./...` lulus.
ownedPaths: `internal/workspace/types.go`, `internal/workspace/store.go` (hanya bila merge menjatuhkan field).
Paralel dengan `rail-registry-store`.

### rail-state-persist-ui: Persist dan restore activeRail di frontend
Di `modules/persist/restore.ts` baca `leftSidebar.activeRail` dari state tersimpan; nilai bukan string atau kosong menjadi `'agents'` (migrasi state lama); nilai tak dikenal dibiarkan, store yang melakukan fallback via `resolveActiveRail`. Di `modules/persist/autosave.ts` pastikan `activeRail` ikut dalam patch `leftSidebar` saat berubah (perubahan `activeRail` saja harus memicu simpan). Memakai store dari `rail-registry-store` dan field Go dari `rail-state-persist-go`.
Acceptance criteria:
- [ ] Restore state tanpa `activeRail`, kosong, atau non-string menghasilkan `activeRail === 'agents'`.
- [ ] Mengganti item aktif memicu autosave dan patch berisi `activeRail`.
- [ ] Restore tidak mengubah `visible` dan `width` yang ada.
- [ ] Build frontend lulus.
ownedPaths: `frontend/src/modules/persist/restore.ts`, `frontend/src/modules/persist/autosave.ts`.
Paralel dengan `rail-ui`; menunggu foundation.

### rail-ui: Komponen rail dan grid ShellFrame empat kolom
Buat `modules/shell/Rail.svelte`: kolom 48px tetap, daftar tombol dari `railItems()`, item aktif ber-class `active`, klik memanggil `setActiveRail`, tombol `<button>` yang bisa difokus dan dipilih dengan keyboard, `aria-label` = `title`, `aria-current="true"` untuk aktif, ikon dirender dari `spec.icon`. Buat `RailPanelHost.svelte` yang hanya me-mount komponen item aktif (hasil `resolveActiveRail`, props `{}`) sehingga panel tak aktif tidak di-mount, dengan efek yang me-reset `activeRail` ke `'agents'` bila item aktif jadi tidak visible. Ubah `ShellFrame.svelte`: grid menjadi `rail | panel | center | right`; rail selalu terlihat; `--left-w` mengatur lebar panel saja (0 saat `toggleLeft` menyembunyikan) dan Resizer kiri tetap menempel di panel. Gaya di `shell.css`. Snippet `left` dipertahankan sebagai konten panel (App mengisinya dengan `RailPanelHost`); bila snippet `left` tak diberikan, ShellFrame merender `RailPanelHost` sebagai fallback sehingga props tetap kompatibel.
Acceptance criteria:
- [ ] Rail 48px tampil walau panel disembunyikan; tidak bisa di-resize.
- [ ] Hanya komponen panel aktif yang ter-mount.
- [ ] Item aktif ditandai visual dan `aria-current`; navigasi keyboard Tab/Enter/Spasi berfungsi.
- [ ] Item yang menjadi tidak visible saat aktif membuat state kembali ke `agents`.
- [ ] Resize panel kiri dan toggle tetap berfungsi; panel kanan dan center tak berubah.
- [ ] Teks/ikon tanpa dependensi baru; build frontend lulus.
ownedPaths: `frontend/src/modules/shell/Rail.svelte`, `frontend/src/modules/shell/RailPanelHost.svelte`, `frontend/src/modules/shell/ShellFrame.svelte`, `frontend/src/modules/shell/shell.css`.
Paralel dengan `rail-state-persist-ui`; menunggu foundation.

### agents-rail-item: Item rail Agents dan wiring App
Wiring tiket. Buat `modules/sidebar-left/rail.ts` yang memanggil `registerRailItem({ id: 'agents', title: 'Agen', icon: <ikon SVG inline>, component: AgentSidebar, order: 0 })` dan impor sebagai efek samping dari `sidebar-left/index.ts` atau `modules/shell/register.ts`. Ubah `App.svelte` agar snippet `left` merender `RailPanelHost` (bukan `AgentSidebar` langsung). Threads tetap sebagai `sidebarSection` di dalam `AgentSidebar` pada tiket ini.
Acceptance criteria:
- [ ] Item `agents` terdaftar sekali (order 0) dan menjadi default aktif.
- [ ] `App.svelte` tidak lagi mengimpor/merender `AgentSidebar` langsung.
- [ ] Section Threads masih tampil di dalam panel Agen.
- [ ] Verifikasi manual: app berjalan, rail terlihat dengan Agen aktif, sesi terminal dan thread tidak terputus, toggle kiri hanya menyembunyikan panel, restart mempertahankan item aktif.
- [ ] `go build ./...` dan build frontend lulus.
ownedPaths: `frontend/src/modules/sidebar-left/rail.ts`, `frontend/src/modules/sidebar-left/index.ts`, `frontend/src/modules/shell/register.ts`, `frontend/src/App.svelte`.
Dikerjakan terakhir (bergantung pada semua fitur lain di epic ini).

## Path yang dimiliki
Hanya path berikut yang boleh disentuh:
- `frontend/src/modules/shell/registry.ts`
- `frontend/src/lib/stores/shell.ts`
- `frontend/src/modules/shell/Rail.svelte`
- `frontend/src/modules/shell/RailPanelHost.svelte`
- `frontend/src/modules/shell/ShellFrame.svelte`
- `frontend/src/modules/shell/shell.css`
- `frontend/src/modules/shell/index.ts`
- `frontend/src/modules/shell/register.ts`
- `frontend/src/modules/sidebar-left/rail.ts`
- `frontend/src/modules/sidebar-left/index.ts`
- `frontend/src/modules/persist/restore.ts`
- `frontend/src/modules/persist/autosave.ts`
- `frontend/src/App.svelte`
- `internal/workspace/types.go`
- `internal/workspace/store.go` (hanya bila merge menjatuhkan field baru)

## Wiring
Penutup tiket ini adalah fitur `agents-rail-item`: daftarkan item `agents` lewat `registerRailItem`, pastikan modulnya terimpor saat startup, dan ganti isi snippet `left` di `App.svelte` dengan `RailPanelHost`. Hasilnya langsung terlihat: rail dengan item Agen aktif, panel Agen berisi `AgentSidebar` (Threads masih di dalamnya), toggle kiri menyembunyikan panel saja, dan pilihan bertahan setelah restart. Tiket ini tidak bergantung pada tiket lain untuk berjalan.

## Guard
Cukup build berhasil: `go build ./...` dan build frontend (`npm run build` di `frontend/`, atau `wails build` bila dipakai proyek). Tanpa unit test.
