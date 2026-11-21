# Plan: Bilah atas: tab workspace, penghitung sesi, pengaturan, ringkasan tengah

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-04-shell-sidebars.md` (fitur `topbar`)
- Fitur: `topbar`
- ownedPaths: `frontend/src/modules/topbar/`
- Depends on foundation: ya (`shell-frame`: `modules/shell/registry.ts`, `helpers.ts`, `index.ts`, `shell.css`, `lib/stores/shell.ts` harus sudah ada). Bila tidak ada, hentikan dan laporkan blocker.
- Kontrak lengkap ada di `overview.md` (bagian 2 dan 3); jangan diulang di sini, hanya dirujuk.

## Tujuan

Membuat bilah atas shell v2 sebagai satu komponen `Topbar` dengan tiga zona: kiri (tab workspace), tengah (ringkasan workspace aktif), kanan (item registri, penghitung sesi, tombol pengaturan). Seluruhnya frontend, memakai store dan registri yang sudah ada, tanpa polling, tanpa dependency baru, tanpa unit test. Belum disambungkan ke `App.svelte` (itu tugas wiring).

## Path yang dimiliki

HANYA `frontend/src/modules/topbar/`. Dilarang menyentuh: `modules/shell/` (termasuk `register.ts`), `lib/stores/shell.ts`, `App.svelte`, `main.ts`, `lib/stores/index.ts`, `styles/*`, modul `workspace/status/appmenu/zoom/layout/...`, `app/main.go`, dan path fitur lain. Bila butuh perubahan di luar path ini, laporkan sebagai blocker. Jangan mengimpor modul fitur paralel (`sidebar-left`, `sidebar-right`, `palette`, `notifications`).

## Kontrak

Disediakan (merujuk overview 2.7, 2.8, 3):

- `modules/topbar/Topbar.svelte`: default export, tanpa props wajib; dipakai wiring sebagai snippet `topbar` `ShellFrame`.
- `modules/topbar/SessionCounter.svelte` (tanpa props), dapat dipakai tiket lain.
- `modules/topbar/index.ts`: bebas efek samping (hanya re-export), karena dimuat `import.meta.glob` di `main.ts`. Ekspor: `Topbar`, `WorkspaceTabs`, `SessionCounter`, `SettingsButton`, `CenterSummary`.

Dipakai (semua sudah ada, jangan diubah):

- `../shell`: `topbarItems(align)` (Readable<TopbarItemSpec[]>, sudah terurut), `zoom('in'|'out'|'reset')`, `tidyGrid()`. Dari `../../lib/stores/shell`: `toggleLeft`, `toggleRight`, `leftSidebar`, `rightSidebar` (untuk label menu "Tampilkan/Sembunyikan").
- `../../lib/stores`: `workspaces`, `activeWorkspaceId`, `activeWorkspace`, `selectWorkspace`, `createWorkspace`, `renameWorkspace`, `deleteWorkspace`, `sessions`, tipe `Session`, `Workspace`, `Project`.
- `../status/store`: `statusStore`, tipe `Status`. Jangan memanggil `startStatusSync()` dan jangan membuat pendengar `session.status` sendiri.
- Tema: variabel `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok`; `--topbar-h` dan `--topbar-inset-left` dari `shell.css` (foundation). Tanpa warna hardcode (fallback boleh).

Aturan zona (overview 2.8): kiri = `WorkspaceTabs` + `topbarItems('left')`; tengah = `topbarItems('center')` + `CenterSummary`; kanan = item registri `order < 50`, `SessionCounter`, item registri `order >= 50`, `SettingsButton` paling akhir. Lonceng notifikasi TIDAK diimpor; ia datang dari registri (`align:'right'`, `order:10`).

## Langkah

1. **Prasyarat (baca saja).** Pastikan ada `modules/shell/registry.ts`, `helpers.ts`, `index.ts`, `lib/stores/shell.ts`. Baca `modules/workspace/WorkspacePicker.svelte` untuk gaya menu/inline-error yang sudah dipakai, `lib/stores/workspaces.ts`, dan `modules/status/store.ts` (`statusStore` bentuk `Record<sessionId,{status,since}>`). `modules/topbar/` belum ada; buat.

2. **`modules/topbar/util.ts`** (helper murni, tanpa efek samping):
   - `providerBadge(providerId: string): string` : `claude`->`CL`, `codex`->`CX`, `shell`->`SH`, lainnya 2 huruf pertama kapital (`''` -> `'??'`).
   - `defaultWorkspaceName(list: Workspace[]): string` : `Workspace N` dengan N = `list.length + 1`, naikkan N sampai nama belum dipakai.
   - `errText(e: unknown): string` : pesan singkat (maks 120 karakter), bawaan `'terjadi kesalahan'`.
   - `countByStatus(workspaceSessions: Session[], map: Record<string,{status:Status}>): Record<Status, number>` : hanya sesi `status==='running'` (bukan `exited`) yang dihitung; sesi tanpa entri di `map` dihitung `idle`.
   - `sessionsOfActive(all: Session[], wsId: string | null): Session[]` : filter `workspaceId === wsId`.

3. **`SessionCounter.svelte`.** `$derived` dari `$sessions`, `$activeWorkspaceId`, `$statusStore`. Teks: `"{N} sesi"` dengan N = jumlah sesi workspace aktif yang `Session.status==='running'` DAN status `statusStore` `running` (sesi tanpa entri status tidak dihitung running). Titik hijau (`--ok`) bila N > 0, redup bila 0. Tooltip (`title` atribut, multi-baris) rincian: `running X`, `waiting X`, `done X`, `idle X` dari `countByStatus` (sesi `exited` dihitung terpisah sebagai `berakhir X` hanya bila > 0). Perbarui otomatis lewat store; tanpa timer. `--wails-draggable: no-drag`.

4. **`CenterSummary.svelte`.** Dari `$activeWorkspace`, `$sessions`, `$statusStore`. Isi satu baris, ellipsis bila sempit (`min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap`):
   - nama workspace (tebal); tanpa workspace aktif -> render kosong.
   - titik hijau + `Running` bila ada sesi workspace aktif yang running (definisi sama dengan langkah 3), selain itu titik redup + `Idle`.
   - `"{n} sesi"` : jumlah semua sesi workspace aktif yang belum `exited`.
   - nama project: maks 3 pertama dari `activeWorkspace.projects` dipisah ` · `, sisanya `+N` bila lebih dari 3; tanpa project -> `tanpa project`.
   - lencana provider unik dari sesi workspace aktif yang belum `exited` (urut kemunculan), via `providerBadge`, masing-masing kapsul kecil dengan `title` = providerId.
   - Elemen non-interaktif; area ini ikut draggable (jangan beri `no-drag`).

5. **`SettingsButton.svelte`.** Tombol ikon gear (SVG inline, tanpa library) dengan `aria-haspopup="menu"` dan `aria-expanded`. Klik membuka popover menu (posisi absolute di bawah tombol, rata kanan). Item, urut:
   - `Perbesar` -> `zoom('in')`; `Perkecil` -> `zoom('out')`; `Reset zoom` -> `zoom('reset')`.
   - pemisah.
   - `Sembunyikan/Tampilkan sidebar kiri` (label dari `$leftSidebar.visible`) -> `toggleLeft()`; sama untuk kanan -> `toggleRight()`.
   - pemisah.
   - `Susun grid` -> `tidyGrid()`.
   - Setiap pemanggilan dibungkus `try/catch`; bila melempar, tampilkan pesan singkat (`errText`) di baris bawah menu dan jangan menutup. Menu menutup setelah aksi sukses, saat klik di luar (`pointerdown` di `window` dipasang di `$effect` hanya selama menu terbuka, dilepas di cleanup), dan saat `Escape`. Navigasi panah atas/bawah antar item dan Enter opsional tetapi disarankan. Hint pintasan di sisi kanan item sebagai teks redup (`⌘=`, `⌘-`, `⌘0`, `⌘B`, `⌥⌘B`), hanya label. Tidak menambah handler `menu:zoom`/`menu:open-folder` atau pintasan keyboard (overview 2.1, 2.6).

6. **`WorkspaceTabs.svelte`** (bagian terbesar).
   - Kontainer tab: `display:flex; overflow-x:auto; min-width:0` (scroll horizontal, scrollbar disembunyikan tipis), `role="tablist"`. Satu `<button role="tab">` per item `$workspaces`, `aria-selected` sesuai `$activeWorkspaceId`; tab aktif bergaya aksen (`--accent` pada garis bawah/teks). Klik -> `selectWorkspace(id)` (abaikan bila sudah aktif), error -> `notice`.
   - State lokal (`$state`): `editingId: string | null`, `draft: string`, `menu: {id, x, y} | null`, `confirmId: string | null`, `notice: string`, `busy: boolean`.
   - **Tambah**: tombol `+` (tetap terlihat di luar area scroll). Klik -> `createWorkspace(defaultWorkspaceName($workspaces))`, lalu `selectWorkspace(ws.id)`, lalu masuk mode rename inline (`editingId = ws.id`, `draft = ws.name`, input fokus dan seluruh teks terpilih). Abaikan klik saat `busy`. Gagal -> `notice`.
   - **Rename inline**: tab yang diedit merender `<input>` (bukan button). Enter -> commit; Esc -> batal; `blur` -> commit (kecuali baru dibatalkan lewat Esc; pakai flag). Commit: `trim()`; kosong atau sama dengan nama lama -> batal tanpa panggilan; selain itu `renameWorkspace(id, name)`; gagal -> `notice`, keluar mode edit. Pastikan commit tidak terpanggil dua kali (Enter lalu blur).
   - **Menu konteks**: `oncontextmenu` pada tab -> `preventDefault()`, `menu = {id, x: clientX, y: clientY}` (`position:fixed`, clamp ke viewport). Item `Ganti nama` -> `editingId = id; draft = nama; menu = null`. Item `Hapus` -> bila `$workspaces.length <= 1`: `notice = 'Tidak bisa menghapus workspace terakhir'` dan tidak memanggil store (item boleh tetap tampil tetapi dengan `aria-disabled` dan teks redup); selain itu `confirmId = id` dan menu menutup. Menutup menu: `pointerdown` di luar, `Escape`, scroll/resize window (listener dipasang di `$effect` hanya saat `menu` tidak null).
   - **Konfirmasi hapus**: popover inline kecil dekat tab (atau baris di bawah topbar) berisi `Hapus "{nama}"?` dengan tombol `Hapus` (variabel `--danger`) dan `Batal`. `Hapus` -> `deleteWorkspace(confirmId)` (store sudah memindahkan workspace aktif bila perlu); gagal -> `notice`. Tutup dengan Esc/Batal/klik luar. Jaga ulang pemeriksaan "workspace terakhir" tepat sebelum memanggil store.
   - **Notice**: teks kecil `--danger` di sebelah tab/ popover, hilang otomatis dengan satu `setTimeout` sekali jalan (3 detik; bersihkan timer sebelumnya dan saat destroy). Tidak ada `setInterval`.
   - Tab aktif di-`scrollIntoView({inline:'nearest', block:'nearest'})` saat berganti (di `$effect`, guard `typeof el.scrollIntoView === 'function'`).
   - Semua kontrol `--wails-draggable: no-drag`; label tab dipotong ellipsis (maks lebar ~160px) dengan `title` nama penuh.

7. **`Topbar.svelte`.** Root `<header class="topbar">`: `height: var(--topbar-h)`, `display:grid`, `grid-template-columns: auto minmax(0,1fr) auto`, `padding-left: var(--topbar-inset-left, 0px)`, latar `--bg-elev`, garis bawah `--border`, `--wails-draggable: drag` pada root (kontrol anak `no-drag`), `min-width:0`. Tiga zona:
   - `left`: `<WorkspaceTabs />` lalu `{#each $leftItems as it (it.id)}{@const C = it.component}<C />{/each}`. Catatan: `WorkspaceTabs` perlu `min-width:0` agar scroll horizontal bekerja; beri zona kiri `min-width:0; max-width:50%` agar tengah dan kanan tidak terdesak.
   - `center`: item `topbarItems('center')` lalu `<CenterSummary />`, rata tengah, `min-width:0`.
   - `right`: `$rightItems.filter(order<50)`, `<SessionCounter />`, `$rightItems.filter(order>=50)`, `<SettingsButton />`. `order` undefined dianggap 0. Gunakan `$derived` atas store `topbarItems('right')` (buat store sekali di level skrip, bukan di dalam `$derived` agar tidak membuat derived baru tiap evaluasi). Item registri dibungkus `<div class="slot">` dengan `no-drag`.
   - Impor registri hanya dari `../shell` (bukan dari berkas dalamnya). Tanpa props wajib.

8. **`index.ts`.** Hanya re-export: `export { default as Topbar } from './Topbar.svelte';` dan sejenis untuk empat komponen lain. Tanpa `registerTopbarItem`, tanpa impor `register.ts`, tanpa efek samping.

9. **Gaya dan teks.** Teks UI bahasa Indonesia (label baku `Running`/`Idle` tetap). Svelte 5 runes dan snippet; tanpa `console.log`; komentar ringkas. Tanpa warna hardcode kecuali fallback `var(--x, #...)`. Pastikan layout tidak overflow horizontal pada jendela 900px (zona tengah menyusut dengan ellipsis).

10. **Guard.** `cd frontend && npm run build` (jalankan `npm install` dulu bila `node_modules` belum ada), lalu `make build` dari root bila sempat. Perbaiki galat tipe/svelte-check pada berkas milik sendiri. `Topbar` belum dipakai `App.svelte`, jadi tampilan aplikasi belum berubah; itu wajar.

## Perubahan

Dibuat (semua di `frontend/src/modules/topbar/`):

- `util.ts`
- `SessionCounter.svelte`
- `CenterSummary.svelte`
- `SettingsButton.svelte`
- `WorkspaceTabs.svelte`
- `Topbar.svelte`
- `index.ts`

Tidak ada berkas lain yang diubah atau dihapus.

## Wiring

Yang diekspos untuk langkah wiring (JANGAN disambungkan di plan ini):

- `Topbar` dari `modules/topbar/Topbar.svelte` (default export, tanpa props wajib) untuk snippet `topbar` di `ShellFrame` pada `App.svelte`.
- `SessionCounter` tersedia lewat `modules/topbar/index.ts` untuk tiket lain.
- Item topbar dari modul lain (mis. lonceng) muncul otomatis di zona yang diminta lewat `registerTopbarItem`; tidak ada langkah wiring tambahan di sisi topbar. Zona kanan: `order < 50` sebelum `SessionCounter`, `order >= 50` sesudahnya, `SettingsButton` terakhir.
- `index.ts` bebas efek samping, jadi aman dimuat `import.meta.glob` di `main.ts`; tidak perlu entri di `register.ts`.

## Selesai bila

- Ketujuh berkas ada di `frontend/src/modules/topbar/` dan tidak ada perubahan di luar path itu.
- `Topbar` merender tiga zona sesuai aturan urutan; item `registerTopbarItem` dengan `align` `left`/`center`/`right` muncul di zona yang diminta dan terurut `order` lalu id.
- Tab mencerminkan `workspaces`; klik berpindah; tab aktif beraksen; `+` membuat workspace dan langsung masuk rename inline; Enter simpan, Esc batal; menu konteks `Ganti nama`/`Hapus` bekerja; hapus workspace terakhir ditolak dengan pesan singkat tanpa memanggil `deleteWorkspace`.
- `SessionCounter` berubah saat sesi dibuat/diakhiri/berganti status tanpa polling; tooltip memuat rincian running/waiting/done/idle.
- `SettingsButton` menampilkan menu Zoom (in/out/reset), toggle sidebar kiri/kanan, Susun grid; setiap aksi memanggil helper/store yang benar dan menu menutup dengan klik luar/Esc.
- `CenterSummary` menampilkan nama workspace, Running/Idle, jumlah sesi, project maks 3 (+N), dan lencana provider CL/CX/SH.
- `modules/topbar/index.ts` hanya berisi re-export (tanpa efek samping); tidak ada `setInterval`, `console.log`, atau dependency baru.
- Guard: `cd frontend && npm run build` sukses (dan `make build` bila dijalankan) tanpa galat pada path fitur ini.
