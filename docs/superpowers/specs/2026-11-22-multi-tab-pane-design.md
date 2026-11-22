# Multi-tab pane — design

Status: draft, menunggu persetujuan. Tanggal: 2026-11-22.

## Intent

Sekarang membuka file (atau konten lain) selalu memecah pane yang fokus menjadi dua, sehingga layar cepat penuh pane kecil. Yang diminta: **satu pane bisa punya banyak tab**. Membuka file menambah tab di pane yang sedang aktif. Pane baru hanya dibuat kalau belum ada pane sama sekali.

Keputusan pengguna:
- Tab campur: satu pane boleh berisi terminal dan file sekaligus.
- Pendekatan A: pane menjadi wadah tab, struktur split tidak berubah.

## Goals

- Pane berisi `tabs[]` dan satu tab aktif. Semua jenis konten (terminal, file, thread, lark-board, lark-doc, git-mr, git-pipeline) bisa menjadi tab.
- `openPane` menambah tab ke pane fokus, bukan memecah pane.
- Konten yang sama tidak dibuka dua kali: bila sudah ada di tab mana pun, pane dan tab itu diaktifkan.
- Header pane menampilkan deretan tab; tab bisa dipilih dan ditutup.
- Terminal di tab non-aktif tetap hidup (sesi PTY dan state xterm tidak hilang).
- Tab bisa dipindahkan ke pane lain dengan drag, bisa dijatuhkan di tepi pane untuk menjadi pane baru (split), dan urutan tab dalam satu pane bisa diatur dengan drag.
- Menutup tab atau pane yang berisi file belum disimpan meminta konfirmasi: Simpan, Buang, atau Batal.
- Layout lama yang sudah tersimpan tetap terbaca.

## Non-goals (versi pertama)

- Tab "preview" yang menimpa diri sendiri ala VS Code.
- Mengubah model split (`Split`, `ratio`, `Splitter`).

## Anti-goals

- Terminal tidak boleh ter-restart atau kehilangan scrollback saat berpindah tab, atau saat tab dipindah ke pane lain.
- Menutup tab terminal tidak boleh membiarkan sesi yatim (perilaku "tutup = akhiri sesi" yang ada sekarang dipertahankan).
- Layout yang tersimpan tidak boleh rusak atau kosong setelah upgrade.

## Approaches

- **A. Pane jadi wadah tab (dipilih).** `Leaf` memegang `tabs[]` dan `activeTabId`. Pohon split tidak berubah. Pane tetap satuan layout, tab satuan konten.
- B. Node `tabgroup` baru di pohon. Ditolak: semua penelusur pohon harus belajar node baru dan fokus jadi ambigu (pane, grup, atau tab).

## Desain

### Model (`modules/layout/model.ts`)

- Tipe baru `Tab`: semua field konten yang sekarang ada di `Leaf` (`kind`, `sessionId`, `projectId`, `provider`, `filePath`, `threadId`, `workspaceId`, `root`, `readOnly`, `threadAgent`, `sourceId`, `tableId`, `viewId`, `nodeToken`, `number`, `pipelineId`, `title`) ditambah `id` (prefix `t_`).
- `Leaf` menjadi `{ type: 'pane', id, tabs: Tab[], activeTabId }`. Invarian: `tabs` tidak pernah kosong (pane tanpa tab dihapus dari pohon), `activeTabId` selalu menunjuk tab yang ada.
- `NewLeaf` tetap menjadi input pembuatan konten; `makeLeaf` menghasilkan `Tab`.
- Helper murni: `activeTab(leaf)`, `allTabs(doc)` (pasangan pane+tab), `findTab(doc, pred)`.
- `openPane(doc, nl)`:
  1. Cari tab yang sama di semua pane dengan aturan dedupe yang ada sekarang (sessionId, filePath+projectId, threadId, lark-board per sourceId, lark-doc per sourceId+nodeToken). Ketemu: aktifkan tab itu, fokus ke pane-nya.
  2. Tidak ada pohon (`root` null): buat pane baru berisi satu tab.
  3. Selain itu: tambah tab di pane fokus dan aktifkan. Bila `focusId` tidak valid tapi ada pane, pakai pane terakhir (perilaku sekarang). Ini tafsir saya atas "belum ada pane aktif".
  4. Parameter `opts.dir` tidak lagi dipakai.
- Fungsi baru: `moveTab(doc, fromPaneId, tabId, targetPaneId, zone, index?)`. Zona `center` memasukkan tab ke pane target (di `index`, default di akhir) dan mengaktifkannya. Zona `top/bottom/left/right` memecah pane target dan menaruh tab di pane baru. Bila pane asal jadi kosong, pane itu dihapus dari pohon. Memindah tab ke pane asalnya sendiri dengan `index` berarti reorder; dengan zona tepi berarti memecah pane asal (hanya bila pane asal punya lebih dari satu tab).
- Fungsi baru: `activateTab(doc, paneId, tabId)`, `closeTab(doc, paneId, tabId)` (tab terakhir ditutup = `closePane`; bila tab aktif ditutup, aktifkan tetangganya), `renameTab`.
- `splitPane` / Cmd+D pada terminal: pane baru berisi satu tab sesi baru, tidak mengubah pane asal.
- `openThreadAgent`: slot "agent thread" menjadi tab dengan `threadAgent: true`. Bila sudah ada, sesinya diganti di tab itu; bila belum, ditambahkan sebagai tab seperti `openPane`.
- `movePane`, `swapPanes`, `maximize`, `tidyGrid`, `resize`: tetap bekerja pada pane utuh.
- `renamePane` menjadi `renameTab` pada tab aktif.

### Persistensi

- `LayoutDoc.version` tetap `1`. `hydrateDoc` menerima dua bentuk leaf: bentuk baru (`tabs`) dan bentuk lama (field konten langsung) yang dibungkus menjadi pane dengan satu tab. Id pane lama dipertahankan. Id tab dibuat baru.
- `autosave` menulis bentuk baru. Versi aplikasi lama tidak bisa membaca bentuk baru; tidak ada dukungan downgrade.
- Dedupe id dan normalisasi dijalankan per pane dan per tab.

### Host konten (`panes/hosts.ts`)

- Host di-key dengan id tab, bukan id pane. `syncPanes` menerima semua tab dari semua pane.
- Tab non-aktif tetap ter-mount dan hanya disembunyikan (`visible=false`), seperti pane yang di-minimize sekarang. Ini yang menjaga terminal tetap hidup.
- `LayoutArea` memasang host tab aktif ke slot pane. Host tab lain dilepas dari DOM tapi tidak di-unmount.
- Tab yang ditutup melepas host-nya (`releaseHost`).

### UI (`PaneHeader.svelte`, `LayoutArea.svelte`)

- Header pane memuat deretan tab di tempat judul. Tiap tab: judul, indikator status sesi untuk terminal (`StatusBadge`), tombol tutup. Tab file yang kotor menampilkan titik "belum disimpan". Pane dengan satu tab tampil mirip sekarang.
- Tombol header: bagi kanan, bagi bawah, maksimalkan, menu. Tombol "tutup pane" memindah ke menu dan menutup semua tab di pane itu; terminal di dalamnya diakhiri seperti sekarang. Tombol minimize terminal menjadi aksi per tab (sesi tetap jalan).
- Badge, chip proyek, dan nomor pane mengikuti tab aktif.
- Area kosong header tetap memulai drag pane utuh. Menekan dan menggeser sebuah tab memulai drag tab (ambang gerak 4px seperti drag pane, supaya klik biasa tetap memilih tab).
- `dnd.ts` dan `DropOverlay` diperluas agar bisa membawa muatan `tab` selain `pane`. Menjatuhkan tab di area isi pane memakai zona yang sudah ada (tengah = jadi tab, tepi = split). Menjatuhkan di deretan tab sebuah pane menyisipkan tab di posisi itu.
- Menu konteks tab menyediakan "Pisahkan ke pane baru" (ke kanan) sebagai jalan lewat keyboard/klik untuk memecah tab tanpa drag.
- Host tab dikunci dengan id tab, jadi memindah tab antar pane tidak me-remount isi; terminal tidak ter-restart.
- Klik tab = `activateTab` + fokus pane.

### Pemanggil yang disesuaikan

- `modules/shell/helpers.ts`: `findSessionPane`, `findFilePane`, `openFilePane`, `focusOrOpenSession` mencari tab dan mengembalikan `{ paneId, tabId }`.
- `modules/tabs/index.ts` (jembatan lama store `tabs`/`activeTabId` dengan layout): `tabIdOfLeaf`, `leafOfTab`, `focusedTabId` bekerja pada tab di dalam pane.
- `modules/layout/shortcuts.ts`: fallback `store.openPane(nl, { dir })` tidak lagi memakai `dir`.
- `lib/stores/groups.ts`, `modules/groups/sessionDrag.ts`, `modules/openfolder/logic.ts`, `modules/grimoire/openPage.ts`: memakai `findLeaf`/`findLeafBy`/`leaves` dan harus dipindah ke pencarian tab.
- Kontrak registry pane kind (`PaneKindSpec`, komponen pane, `onEndSession`, `badgeFor`, `titleFor`): menerima `Tab`, bukan `Leaf`. `TerminalPane` memanggil `renamePane` dari `onTitle`; ganti ke `renameTab(paneId, tabId)`.
- Pemanggil `openPane` lain (file tree, palette, lark, git, grimoire, thread, groups) tidak perlu diubah karena kontraknya sama.

### Konfirmasi file belum disimpan

Dirty state hidup di dalam `FileTab.svelte` (`content` dibanding `savedContent`) dan `save()` juga di sana, sedangkan penutupan tab terjadi di luar komponen itu. Jembatannya:

- Modul `files` menyediakan registri per id tab: `registerFileTab(tabId, { isDirty(): boolean, save(): Promise<boolean> })`. `FilePane` meneruskan id tab ke `FileTab`, yang mendaftar saat mount dan mencabut saat destroy. Karena host tab non-aktif tetap ter-mount, state kotornya tetap ada.
- Registri yang sama juga menjadi sumber titik "belum disimpan" di tab (store reaktif per id tab). Event `shikigami:tab-dirty` berbasis path tidak dipakai untuk ini karena tidak bisa membedakan file yang sama di proyek berbeda.
- Fungsi `confirmCloseTabs(tabIds): Promise<boolean>` dipanggil sebelum penutupan. Tanpa tab kotor, langsung `true`. Bila ada, tampil dialog dengan daftar file kotor dan tiga pilihan:
  - **Simpan**: panggil `save()` tiap file kotor. Bila ada yang gagal, penutupan dibatalkan dan pesan galat ditampilkan; tab tetap terbuka.
  - **Buang**: lanjut menutup tanpa menyimpan.
  - **Batal**: tidak menutup apa pun.
  Untuk beberapa file, tombolnya "Simpan semua" dan "Buang semua".
- Dipakai oleh: tombol tutup tab, "Tutup pane" (semua tab di pane), dan jalur tutup lain yang menghapus tab file. Memindah tab antar pane tidak memicu konfirmasi karena tab tidak ditutup.
- Penutupan yang dipicu sistem (mis. tab file hilang karena jembatan v1, atau file dihapus dari disk) tidak meminta konfirmasi.
- Dialog memakai komponen dialog yang sudah ada di proyek bila ada; bila tidak, dibuat dialog sederhana (builder memeriksa).

## Constraints

- Frontend Svelte 5 + TypeScript. Perubahan model harus tetap fungsi murni di `model.ts`.
- Tidak ada runner tes frontend saat ini. Rencana implementasi memutuskan apakah menambah vitest untuk `model.ts` (saran saya: ya, karena logika murni dan banyak kasus tepi).
- Menutup jendela atau keluar dari aplikasi dengan file kotor belum tercakup; builder memeriksa apakah sudah ada penanganannya dan mengangkatnya sebagai item terpisah bila belum.

## Dibiarkan ke builder

- Tampilan visual tab (lebar, overflow/scroll horizontal saat tab banyak, menu konteks tab).
- Urutan penyisipan tab baru (default saya: di kanan tab aktif).
- Pemetaan kunci `{#key n.id}` di `LayoutArea` agar pergantian tab tidak me-remount isi.
- Aksesibilitas tab (`role="tablist"`, navigasi keyboard).

## Keputusan saya (bukan dari pengguna)

- Minimize tab terminal: tab dilepas dari pane, sesi tetap hidup dan tetap bisa dibuka dari sidebar (sama seperti sekarang). Tidak ada status "minimized". Hanya berlaku untuk tab terminal. Bila itu tab terakhir, pane ikut hilang.
- Jembatan lama `modules/tabs/index.ts`: menutup tab v1 hanya menutup tab layout yang cocok, bukan seluruh pane. `focusedTabId` = tab aktif dari pane fokus. Builder memastikan jembatan ini memang masih terpakai sebelum mengubahnya.
- "Akhiri sesi" di menu pane bekerja pada tab aktif, lalu menutup tab itu.
- Cmd+D dan tombol bagi pada tab aktif non-terminal: tidak melakukan apa-apa (sama dengan sekarang).
- `TerminalPane` sudah menerima `visible`; builder memverifikasi refit saat tab dipasang kembali.
- Id pane dipertahankan saat migrasi, id tab dibuat baru. `focusId` dan `maximizedId` tetap id pane.
- Tes: tambah vitest sebagai devDependency hanya untuk `model.ts`.
