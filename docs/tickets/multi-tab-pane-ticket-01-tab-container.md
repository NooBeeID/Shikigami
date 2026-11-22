---
id: multi-tab-pane-ticket-01-tab-container
title: Pane sebagai wadah tab
spec: docs/superpowers/specs/2026-11-22-multi-tab-pane-design.md
status: implemented
wave: 1
depends_on: []
---
# Pane sebagai wadah tab

## Tujuan

Saat ini membuka file atau konten lain selalu memecah pane yang fokus menjadi dua sehingga layar cepat penuh pane kecil. Epic ini mengubah pane menjadi wadah tab: `Leaf` memegang `tabs[]` dan `activeTabId`, `openPane` menambah tab ke pane fokus, dan header pane menampilkan deretan tab. Struktur split (`Split`, `ratio`, `Splitter`) tidak berubah. Epic ini adalah fondasi untuk tiket drag tab (wave 2) dan konfirmasi file belum disimpan (wave 3).

## Hasil yang bisa dijalankan

Setelah tiket ini selesai (build frontend dan build aplikasi lulus, aplikasi jalan):

- Membuka file dari file tree menambah tab di pane fokus, tidak lagi memecah pane. Bila belum ada pane sama sekali, dibuat satu pane berisi satu tab.
- Konten yang sama (sesi, file+proyek, thread, lark-board per sumber, lark-doc per sumber+node) tidak dibuka dua kali: pane dan tab yang sudah ada diaktifkan dan difokus.
- Header pane menampilkan deretan tab. Klik tab memilihnya, tombol tutup menutup tab. Tab terminal punya StatusBadge dan aksi minimize.
- Terminal di tab non-aktif tetap hidup (PTY dan scrollback utuh) saat pindah tab.
- Layout lama yang tersimpan (leaf dengan field konten langsung) termuat tanpa rusak atau kosong, dibungkus menjadi pane satu tab.
- Cmd+D / tombol bagi pada terminal membuat pane baru berisi satu tab sesi baru. Maksimalkan, swap, tidyGrid, resize, dan drag pane utuh tetap bekerja.
- Belum ada (tiket berikutnya): drag tab antar pane/reorder/split tab, menu konteks tab, dialog konfirmasi file belum disimpan, titik "belum disimpan" pada tab.

## Batasan

In scope:
- Model `Tab`, `Leaf` baru, fungsi murni tab di `model.ts`, update `PaneKindSpec`.
- Persistensi dua bentuk leaf (bentuk lama dan baru), autosave bentuk baru.
- Adaptasi seluruh pemanggil `findLeaf`/`findLeafBy`/`leaves`/`renamePane` ke pencarian tab.
- Host konten per id tab, mount tab non-aktif tetap hidup.
- Deretan tab di `PaneHeader`, aksi tutup tab, minimize terminal per tab, "Tutup pane" di menu, "Akhiri sesi" pada tab aktif.

Out of scope:
- `moveTab`, drag tab, `DropOverlay` untuk tab, menu konteks tab "Pisahkan ke pane baru" (tiket 02).
- `registerFileTab`, `confirmCloseTabs`, titik dirty pada tab (tiket 03). Tiket ini menutup tab langsung tanpa konfirmasi.
- Tab preview ala VS Code, perubahan model split, perubahan daemon/IPC, penambahan vitest atau unit test.
- Penanganan menutup jendela dengan file kotor.

## Kontrak

Semua perubahan frontend-only (Svelte 5 + TS), tanpa perubahan daemon/IPC. Kontrak berikut dibuat di `frontend/src/modules/layout/model.ts` dan tidak boleh diubah bentuknya oleh tiket lain (hanya boleh ditambah).

Tipe:
- `Tab = { id (prefix t_), kind, sessionId?, projectId?, provider?, filePath?, threadId?, workspaceId?, root?, readOnly?, threadAgent?, sourceId?, tableId?, viewId?, nodeToken?, number?, pipelineId?, title? }`
- `Leaf = { type: 'pane', id, tabs: Tab[] (tidak pernah kosong), activeTabId (selalu menunjuk tab yang ada) }`
- `NewLeaf` tetap sebagai input konten; `makeLeaf` menghasilkan `Tab`.
- `focusId` dan `maximizedId` tetap id pane. `LayoutDoc.version` tetap `1`.

Helper murni:
- `activeTab(leaf): Tab`
- `allTabs(doc): { pane: Leaf; tab: Tab }[]`
- `findTab(doc, pred): { pane: Leaf; tab: Tab } | null`

Operasi (semua `LayoutDoc -> LayoutDoc`, murni):
- `openPane(doc, nl)`: dedupe lintas semua pane lalu aktifkan tab dan fokus pane; root null membuat pane baru; selain itu tambah tab di kanan tab aktif pada pane fokus (focusId tidak valid -> pane terakhir). `opts.dir` diabaikan. Kontrak publik store `openPane(nl, opts?)` tidak berubah.
- `activateTab(doc, paneId, tabId)`
- `closeTab(doc, paneId, tabId)`: tab terakhir = `closePane`; tab aktif ditutup mengaktifkan tetangga.
- `renameTab(doc, paneId, tabId, title)` (pengganti `renamePane`, pada tab aktif bila dipanggil lewat jalur lama).
- `openThreadAgent(doc, nl)`: tab `threadAgent: true`; ganti sesi bila ada, tambah tab bila belum.

Registry (`layout/registry.ts`): `PaneKindSpec.component` menerima `{ pane: Tab; visible: boolean }` plus `paneId` bila dibutuhkan (nama prop tetap `pane`, tipenya `Tab`); `titleFor`, `onEndSession`, `badgeFor` menerima `Tab`.

Persistensi: `hydrateDoc` menerima bentuk lama (leaf dengan field konten, dibungkus jadi pane satu tab, id pane dipertahankan, id tab baru) dan bentuk baru; autosave menulis bentuk baru.

Host konten (`panes/hosts.ts`): di-key id tab; tab non-aktif tetap ter-mount (`visible=false`); `releaseHost` saat tab ditutup.

Pemanggil: helper di `shell/helpers.ts` mengembalikan `{ paneId, tabId }`; `openPane(nl, opts?)` publik tidak berubah.

Yang dipakai dari tiket lain: tidak ada.

## Fitur

Urutan: `tab-model-foundation` dikerjakan lebih dulu (fitur lain bergantung pada tipe barunya). Setelah itu `tab-persist-migration`, `tab-callers`, `tab-hosts-render`, dan `tab-header-ui` bisa dikerjakan paralel karena ownedPaths-nya terpisah, dengan syarat tipe/signature di bagian Kontrak dipatuhi persis. `tab-wiring` terakhir.

Catatan umum: karena `Leaf` berubah bentuk, build penuh baru lulus bila semua fitur selesai. Tiap fitur boleh meninggalkan error tipe di file milik fitur lain selama sementara; verifikasi build akhir ada di `tab-wiring`.

### tab-model-foundation: Model Tab dan fungsi murni tab di model.ts

Deskripsi:
- Di `frontend/src/modules/layout/model.ts` definisikan `Tab` (semua field konten `Leaf` lama ditambah `id` berprefix `t_`). Ubah `Leaf` menjadi `{ type: 'pane', id, tabs, activeTabId }` dengan invarian tabs tidak kosong dan activeTabId valid.
- `makeLeaf` menghasilkan `Tab` dari `NewLeaf`. Buat pembangkit id tab (prefix `t_`) di samping pembangkit id pane yang sudah ada.
- Tambahkan `activeTab`, `allTabs`, `findTab`. `leaves`, `findLeaf`, `findLeafBy` tetap ada untuk penelusuran pane.
- Tulis ulang `openPane` sesuai Kontrak. Dedupe memakai aturan sekarang: `sessionId`; `filePath`+`projectId`; `threadId`; lark-board per `sourceId`; lark-doc per `sourceId`+`nodeToken`. Tab baru disisipkan tepat di kanan tab aktif pane fokus lalu diaktifkan; pane difokus.
- Tambah `activateTab`, `closeTab` (tab terakhir memanggil `closePane`; bila tab aktif ditutup pilih tetangga, utamakan yang di kanan lalu kiri), `renameTab`. Hapus/ganti `renamePane` menjadi `renameTab` pada tab aktif.
- `splitPane` (Cmd+D terminal): pane baru berisi satu tab sesi baru; pane asal tidak berubah. Pada tab aktif non-terminal tidak melakukan apa-apa.
- `openThreadAgent`: cari tab `threadAgent: true` di semua pane; bila ada ganti `threadId`/sesinya di tab itu dan aktifkan; bila belum, tambahkan seperti `openPane`.
- `movePane`, `swapPanes`, `maximize`, `restore`, `tidyGrid`, `resize`, `closePane` tetap bekerja pada pane utuh (sesuaikan hanya bila tipe Leaf memaksa).
- `registry.ts`: ubah `PaneKindSpec` sesuai Kontrak. `frontend/src/lib/stores/layout.ts`: ekspos store untuk `activateTab`, `closeTab`, `renameTab`, dan sesuaikan `openPane`/`openThreadAgent` agar memakai fungsi model baru; `opts.dir` diterima tetapi diabaikan.
- `hydrateDoc` di model.ts bukan milik fitur ini (ada di `tab-persist-migration`), tetapi bagian dalam model.ts yang berbagi fungsi normalisasi tetap harus kompilasi.

Acceptance criteria:
- [ ] `Tab`, `Leaf` baru, `activeTab`, `allTabs`, `findTab` diekspor dengan signature di Kontrak.
- [ ] `openPane` pada doc kosong membuat satu pane satu tab; pada doc berisi menambah tab di pane fokus tanpa membuat pane baru.
- [ ] `openPane` untuk konten yang sudah ada (kelima aturan dedupe) hanya mengaktifkan tab dan fokus pane.
- [ ] `closeTab` pada tab terakhir menghapus pane; pada tab aktif mengaktifkan tetangga; invarian activeTabId terjaga.
- [ ] `renameTab`, `activateTab`, `openThreadAgent` berperilaku sesuai Kontrak.
- [ ] `splitPane` pada terminal membuat pane baru berisi satu tab sesi baru.
- [ ] `PaneKindSpec` menerima `Tab`; store layout mengekspos aksi tab baru.
- [ ] Semua fungsi tetap murni (tidak mengubah doc masukan).

ownedPaths:
- `frontend/src/modules/layout/model.ts`
- `frontend/src/modules/layout/registry.ts`
- `frontend/src/lib/stores/layout.ts`

Paralel: dikerjakan dahulu; fitur lain menunggu tipe selesai (atau mengikuti Kontrak persis).

### tab-persist-migration: Persistensi layout dua bentuk leaf

Deskripsi:
- `hydrateDoc` (di `modules/persist/`, dan bila fungsi hydrate berada di `model.ts` koordinasikan: ia milik fondasi hanya untuk tipe, logikanya disesuaikan di sini lewat modul persist) menerima dua bentuk leaf: bentuk baru (`tabs` + `activeTabId`) dan bentuk lama (field konten langsung). Bentuk lama dibungkus menjadi pane dengan satu tab; id pane dipertahankan, id tab dibuat baru; `activeTabId` menunjuk tab itu.
- Bentuk baru: buang tab tidak valid, `activeTabId` yang tidak ada diganti tab pertama, pane tanpa tab dibuang dari pohon (split yang kehilangan anak runtuh ke anak yang tersisa).
- Dedupe id pada pane dan pada tab (id bentrok dibuatkan baru). Normalisasi per pane dan per tab. `focusId`/`maximizedId` yang tidak menunjuk pane valid direset.
- `LayoutDoc.version` tetap `1`. `autosave` menulis bentuk baru.
- Sesuaikan `Restore`, `TerminalGuard`, `DeadSession` (di `modules/persist/`) dan `lib/stores/persist.ts` agar mencari leaf lewat `allTabs`/`findTab` dan memakai `tab.sessionId`.
- Bila `hydrateDoc` tinggal di `model.ts`, pindahkan fungsi tersebut ke modul persist dan ekspor ulang dari lokasi lama; jika tidak memungkinkan tanpa menyentuh model.ts, bagian itu dikoordinasikan dengan `tab-model-foundation` (satu agent saja yang mengedit model.ts pada satu waktu).

Acceptance criteria:
- [ ] Layout lama (JSON dengan leaf berisi `kind`, `sessionId`, dst.) termuat menjadi pane satu tab dengan id pane sama.
- [ ] Layout bentuk baru termuat dan round-trip lewat autosave tanpa perubahan.
- [ ] Input rusak (tab tanpa kind, activeTabId salah, id ganda) tidak membuat crash maupun layout kosong bila masih ada konten valid.
- [ ] Restore sesi terminal, TerminalGuard, dan DeadSession bekerja dengan tab di dalam pane.
- [ ] `focusId` dan `maximizedId` tetap id pane.

ownedPaths:
- `frontend/src/modules/persist/`
- `frontend/src/lib/stores/persist.ts`

Paralel: bisa dikerjakan paralel dengan fitur lain setelah tipe di `tab-model-foundation` tersedia.

### tab-callers: Adaptasi pemanggil ke pencarian tab

Deskripsi:
- `shell/helpers.ts`: `findSessionPane`, `findFilePane`, `openFilePane`, `focusOrOpenSession` mencari lewat `findTab`/`allTabs` dan mengembalikan `{ paneId, tabId }`. Pemanggil di dalam berkas yang dimiliki fitur ini disesuaikan.
- `tabs/index.ts` (jembatan v1 antara store `tabs`/`activeTabId` dan layout): `tabIdOfLeaf`, `leafOfTab`, `focusedTabId` bekerja pada tab dalam pane; `focusedTabId` = tab aktif pane fokus. Menutup tab v1 hanya menutup tab layout yang cocok (`closeTab`), bukan seluruh pane. Sebelum mengubah, builder memastikan jembatan masih terpakai (grep pemakai); jika tidak terpakai, cukup pastikan kompilasi.
- `layout/shortcuts.ts`: fallback `openPane(nl, { dir })` menjadi `openPane(nl)`; Cmd+D/bagi hanya berlaku untuk tab aktif terminal.
- `lib/stores/groups.ts`, `groups/sessionDrag.ts`, `openfolder/logic.ts`, `grimoire/openPage.ts`, `thread/thread-store.ts`: pindahkan pemakaian `findLeaf`/`findLeafBy`/`leaves` ke pencarian tab (`findTab`/`allTabs`), baca konten dari `tab`, bukan `leaf`.
- Panel yang memanggil `openPane` (lark, git) disesuaikan hanya bila tipe berubah; kontrak `openPane` tidak berubah sehingga pemanggil lain tidak perlu disentuh.

Acceptance criteria:
- [ ] Tidak ada lagi akses field konten (`sessionId`, `filePath`, dst.) langsung pada `Leaf` di berkas milik fitur ini.
- [ ] `focusOrOpenSession` mengembalikan `{ paneId, tabId }` dan memfokus tab yang ada atau membuka tab baru.
- [ ] Menutup tab v1 lewat jembatan hanya menutup tab layout yang cocok.
- [ ] Drag sesi dari sidebar (sessionDrag), buka folder, buka halaman grimoire, dan thread-store tetap membuka konten sebagai tab.
- [ ] Berkas-berkas di atas kompilasi dengan tipe baru.

ownedPaths:
- `frontend/src/modules/shell/helpers.ts`
- `frontend/src/modules/tabs/index.ts`
- `frontend/src/modules/layout/shortcuts.ts`
- `frontend/src/lib/stores/groups.ts`
- `frontend/src/modules/groups/sessionDrag.ts`
- `frontend/src/modules/openfolder/logic.ts`
- `frontend/src/modules/grimoire/openPage.ts`
- `frontend/src/modules/thread/thread-store.ts`

Paralel: bisa dikerjakan paralel dengan fitur lain setelah tipe tersedia.

### tab-hosts-render: Host konten per tab dan komponen pane

Deskripsi:
- `panes/hosts.ts` di-key dengan id tab. `syncPanes` menerima semua tab semua pane (dari `allTabs`), membuat host untuk tab baru, `releaseHost` untuk tab yang hilang.
- Tab non-aktif tetap ter-mount dan hanya disembunyikan (`visible=false`) agar terminal tetap hidup; tab aktif di pane yang tidak ter-maximize/terlihat mengikuti aturan visible sekarang.
- `LayoutArea.svelte`: pasang host tab aktif ke slot pane; host tab lain dilepas dari DOM tanpa unmount. Gunakan kunci `{#key}` sehingga pergantian tab dan perpindahan tab antar pane tidak me-remount isi (terminal tidak restart).
- Komponen pane (`TerminalPane`, `FilePane`, `BoardPane`, `MrPane`, `PipelinePane`, `ThreadPane`, `ReadOnlyFileView`, dll) menerima prop `pane: Tab` (dan `paneId` bila perlu). `TerminalPane` memanggil `renameTab(paneId, tabId, title)` dari `onTitle` (ganti `renamePane`) dan memverifikasi refit (fit xterm + resize PTY) saat tab dipasang kembali dan `visible` berubah jadi true.
- `panes/terminal.ts` dan `panes/file.ts` mendaftarkan kind dengan `PaneKindSpec` baru.

Acceptance criteria:
- [ ] Host ter-key id tab; dua tab di satu pane keduanya ter-mount, hanya tab aktif yang terlihat.
- [ ] Pindah tab bolak-balik pada tab terminal tidak me-restart sesi, scrollback utuh, ukuran terminal benar setelah dipasang kembali.
- [ ] Menutup tab memanggil `releaseHost` dan membersihkan host.
- [ ] Semua komponen pane kind menerima `Tab` dan kompilasi.
- [ ] Judul terminal dari `onTitle` memperbarui judul tab yang benar.

ownedPaths:
- `frontend/src/modules/layout/panes/`
- `frontend/src/modules/layout/LayoutArea.svelte`
- `frontend/src/modules/lark/board/BoardPane.svelte`
- `frontend/src/modules/git/MrPane.svelte`
- `frontend/src/modules/git/PipelinePane.svelte`
- `frontend/src/modules/thread/ThreadPane.svelte`

Paralel: bisa dikerjakan paralel dengan fitur lain setelah tipe tersedia. Menyentuh `PaneHeader` tidak diizinkan di sini.

### tab-header-ui: Deretan tab di header pane

Deskripsi:
- `PaneHeader.svelte` memuat deretan tab di tempat judul: judul tab, `StatusBadge` sesi untuk terminal, tombol tutup. Pane satu tab tampil mirip tampilan sekarang. Overflow: scroll horizontal saat tab banyak, tab aktif di-scroll ke tampak. Aksesibilitas: `role="tablist"` / `role="tab"`, `aria-selected`, navigasi keyboard (panah kiri/kanan, Home/End, Delete menutup).
- Klik tab = `activateTab` + fokus pane. Badge, chip proyek, dan nomor pane mengikuti tab aktif.
- Tombol header: bagi kanan, bagi bawah, maksimalkan, menu. Tombol "Tutup pane" dipindah ke `PaneMenu` dan menutup semua tab di pane (terminal diakhiri seperti perilaku sekarang). Tanpa konfirmasi (tiket 03 menambahkannya).
- Minimize terminal jadi aksi per tab: tab dilepas dari pane (`closeTab` tanpa mengakhiri sesi), sesi tetap hidup dan bisa dibuka dari sidebar. Hanya tampil untuk tab terminal; bila tab terakhir, pane ikut hilang.
- "Akhiri sesi" di `PaneMenu` bekerja pada tab aktif lalu menutup tab itu (lewat `onEndSession` spec kind).
- Cmd+D/tombol bagi pada tab aktif non-terminal tidak melakukan apa-apa (tombol disembunyikan atau disabled).
- Area kosong header tetap memulai drag pane utuh (perilaku dnd pane yang ada tidak berubah; ambang 4px). Drag per tab belum ada di tiket ini.
- `shell/LayoutHeaderHost.svelte` disesuaikan bila tipe header berubah.

Acceptance criteria:
- [ ] Deretan tab tampil; tab aktif tersorot; klik memilih tab dan memfokus pane.
- [ ] Tombol tutup pada tab menutup tab itu; tab terakhir menutup pane (terminal diakhiri sesinya seperti sekarang bila itu jalur tutup).
- [ ] "Tutup pane" ada di menu dan menutup semua tab.
- [ ] Minimize hanya muncul pada tab terminal dan sesi tetap hidup/tersedia di sidebar.
- [ ] "Akhiri sesi" mengakhiri sesi tab aktif dan menutup tab itu.
- [ ] Overflow tab bisa di-scroll horizontal; keyboard dan role tablist berfungsi.
- [ ] Drag dari area kosong header tetap memindah pane utuh.

ownedPaths:
- `frontend/src/modules/layout/PaneHeader.svelte`
- `frontend/src/modules/layout/PaneMenu.svelte`
- `frontend/src/modules/shell/LayoutHeaderHost.svelte`

Paralel: bisa dikerjakan paralel dengan fitur lain setelah tipe dan aksi store tersedia.

### tab-wiring: Wiring dan verifikasi build tiket tab-container

Deskripsi:
- Sambungkan semua fitur: pastikan seluruh pemanggil kompilasi dengan tipe baru, `layout/index.ts` mendaftarkan kind (import `./panes/*`) dan `App.svelte` memakai API layout baru bila ada yang berubah.
- Verifikasi skenario: buka file dari file tree menambah tab di pane fokus (tidak memecah); konten sama tidak terbuka dua kali; terminal tetap hidup saat pindah tab; layout lama termuat.
- Jalankan build frontend lalu build aplikasi sampai lulus. Hanya menyentuh berkas lintas fitur yang diperlukan untuk menyambung.

Acceptance criteria:
- [ ] Build frontend lulus tanpa error tipe (`svelte-check`/`tsc` bila ada di skrip proyek).
- [ ] Build aplikasi lulus (`go build ./...` dan/atau perintah build aplikasi proyek).
- [ ] Skenario verifikasi di atas dilakukan secara manual di aplikasi dan hasilnya dicatat pada pesan commit/PR.

ownedPaths:
- `frontend/src/App.svelte`
- `frontend/src/modules/layout/index.ts`

Paralel: dikerjakan terakhir, setelah semua fitur lain di epic ini selesai.

## Path yang dimiliki

Hanya path ini yang boleh disentuh oleh tiket ini:
- `frontend/src/modules/layout/model.ts`
- `frontend/src/modules/layout/registry.ts`
- `frontend/src/modules/layout/LayoutArea.svelte`
- `frontend/src/modules/layout/PaneHeader.svelte`
- `frontend/src/modules/layout/PaneMenu.svelte`
- `frontend/src/modules/layout/shortcuts.ts`
- `frontend/src/modules/layout/panes/`
- `frontend/src/lib/stores/layout.ts`
- `frontend/src/lib/stores/groups.ts`
- `frontend/src/modules/persist/`
- `frontend/src/modules/shell/helpers.ts`
- `frontend/src/modules/tabs/index.ts`
- `frontend/src/modules/groups/sessionDrag.ts`
- `frontend/src/modules/openfolder/logic.ts`
- `frontend/src/modules/grimoire/openPage.ts`
- `frontend/src/modules/thread/thread-store.ts`
- `frontend/src/modules/lark/board/BoardPane.svelte`
- `frontend/src/modules/git/MrPane.svelte`

Tambahan yang diperlukan oleh deskripsi fitur (dan hanya untuk itu): `frontend/src/lib/stores/persist.ts`, `frontend/src/modules/shell/LayoutHeaderHost.svelte`, `frontend/src/modules/git/PipelinePane.svelte`, `frontend/src/modules/thread/ThreadPane.svelte`, `frontend/src/App.svelte`, `frontend/src/modules/layout/index.ts`.

Berkas yang dimiliki tiket lain dan tidak boleh disentuh di sini: `layout/dnd.ts`, `layout/DropOverlay.svelte`, `layout/TabContextMenu.svelte` (tiket 02); `frontend/src/modules/files/`, `layout/panes/FilePane.svelte` untuk registri dirty (tiket 03; di tiket ini FilePane hanya diubah agar menerima `Tab`).

## Wiring

Langkah penutup tiket ini (dikerjakan di `tab-wiring`):
1. Pastikan `layout/index.ts` masih mengimpor pendaftaran kind (`./panes/terminal`, `./panes/file`, dan kind lain yang terdaftar) dengan `PaneKindSpec` baru, dan `App.svelte` tidak lagi memakai API lama (`renamePane`, akses field konten `Leaf`, `opts.dir`).
2. Pastikan `LayoutArea` memakai `allTabs` untuk `syncPanes` dan header menerima aksi tab dari store, sehingga seluruh alur (buka file, sesi baru, thread agent, lark, git, grimoire, drag sesi dari sidebar) menghasilkan tab yang terlihat di header.
3. Jalankan aplikasi: buka file dari file tree beberapa kali (tab bertambah di pane fokus), buka file yang sama lagi (tab yang ada diaktifkan), pindah antar tab terminal dan file (terminal tetap hidup), tutup tab, minimize terminal, Cmd+D pada terminal, muat ulang aplikasi dengan layout tersimpan (lama dan baru).
4. Tiket ini tidak bergantung pada tiket lain untuk bisa dijalankan; titik ekstensi untuk tiket 02 (header tab, model) dan 03 (tombol tutup tab/pane di `PaneHeader`/`PaneMenu`) sudah tersedia.

## Guard

Tanpa unit test (jangan menambah vitest). Cukup build:
- Build frontend (skrip build di `frontend/package.json`, mis. `npm run build` dan `npm run check` bila ada) lulus.
- Build aplikasi (`go build ./...` dan/atau perintah build aplikasi proyek) lulus.
