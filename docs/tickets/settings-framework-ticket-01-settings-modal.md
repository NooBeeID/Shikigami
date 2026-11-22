---
id: settings-framework-ticket-01-settings-modal
title: Modal Pengaturan
spec: docs/superpowers/specs/2026-11-22-settings-framework-design.md
status: implemented
wave: 1
depends_on: []
---
# Modal Pengaturan

## Tujuan
Menyediakan wadah UI pengaturan: modal penuh yang dibuka dari entri "Semua pengaturan…" di dropdown gear, dengan sidebar kiri (Connector, MCP) dan konten di kanan. Spec memilih modal (bukan pane layout) karena pengaturan jarang dibuka sambil bekerja dan form lebih natural di modal; konsekuensinya dibutuhkan komponen modal generik baru.

Tiket ini hanya membangun kerangka dan titik ekstensinya: registri halaman pengaturan (`registerSettingsPage`) sehingga tiket berikutnya (`settings-framework-ticket-02-connector-framework`) cukup mengganti halaman Connector placeholder dengan isi nyata tanpa mengubah file milik tiket ini. Halaman MCP hanya kerangka kosong (isinya dikerjakan di bagian 2 spec). Modal tidak boleh mengganggu sesi terminal yang berjalan.

## Hasil yang bisa dijalankan
Di atas kode yang sudah ada (topbar, registri panel `lib/registry.ts`, tema CSS):
- Dropdown gear memiliki grup baru (dipisah separator) berisi item "Semua pengaturan…". Mengkliknya menutup dropdown dan membuka modal pengaturan.
- Modal tampil di tengah layar di atas backdrop gelap, dengan judul "Pengaturan", sidebar kiri berisi dua halaman: "Connector" (order 10) dan "MCP" (order 20), dan area konten di kanan. Halaman aktif awal adalah "Connector".
- Halaman Connector menampilkan placeholder (teks singkat bahwa daftar connector segera hadir). Halaman MCP menampilkan judul "MCP" dan teks "Pengelolaan server MCP segera hadir".
- Klik item sidebar mengganti halaman di kanan. Tombol panah atas/bawah memindahkan fokus dan pilihan antar item sidebar.
- Modal ditutup dengan Esc, klik di luar dialog (backdrop), atau tombol tutup (x). Setelah ditutup, fokus kembali ke elemen pemicu (tombol gear).
- Saat modal terbuka maupun tertutup, sesi terminal yang berjalan tidak di-unmount, tidak di-restart, dan tidak kehilangan output; saat modal tertutup input keyboard terminal tidak ditangkap modal.
- `make build` dan `go build ./...` lulus dan aplikasi bisa dijalankan.

## Batasan
In scope:
- Store pengaturan (`settingsOpen`, `activePage`, `openSettings(pageId?)`, `closeSettings()`).
- Registri halaman pengaturan (`registerSettingsPage`) dengan daftar turunan terurut.
- Komponen `Modal.svelte` generik (dapat dipakai ulang oleh dialog lain, mis. dialog Connect di tiket 02).
- `SettingsModal.svelte`, `McpPage.svelte` (kerangka), `ConnectorsPlaceholder.svelte` (didaftarkan sebagai id `connectors`).
- Pemasangan host modal lewat `registerPanel` area `statusbar` di `modules/settings/index.ts`.
- Penambahan satu grup menu pada array `groups` di `SettingsButton.svelte`.
- Teks UI bahasa Indonesia; identifier kode bahasa Inggris.

Out of scope:
- Daftar connector, status, tombol Connect, dialog cara menyambung (tiket 02).
- Pengelolaan server MCP dan perubahan `AppConfig` (bagian 2 spec; kontrak: tiket ini tidak mengubah `internal/appconfig`).
- Perubahan backend Go apa pun, pesan IPC, atau CLI.
- Tombol "Buka di terminal", pencarian pengaturan, halaman pengaturan lain di luar Connector dan MCP.
- Menyimpan token atau kredensial apa pun.
- Mengubah file milik modul lain selain penambahan grup di `SettingsButton.svelte`.

## Kontrak
Yang disediakan (dipakai tiket 02):

1. `frontend/src/modules/settings/store.ts`
   - `export const settingsOpen: Writable<boolean>` (awal `false`).
   - `export const activePage: Writable<string>` (awal `'connectors'`).
   - `export function openSettings(pageId?: string): void`: set `settingsOpen=true`; bila `pageId` diberikan dan terdaftar, `activePage=pageId`; bila tidak diberikan atau tidak terdaftar, pakai halaman pertama terurut. Simpan `document.activeElement` sebagai pemicu untuk dikembalikan fokusnya (dikelola Modal).
   - `export function closeSettings(): void`: set `settingsOpen=false`.
2. `frontend/src/modules/settings/pages.ts`
   - `export interface SettingsPageSpec { id: string; title: string; order?: number; component: Component<any>; }`
   - `export function registerSettingsPage(spec: SettingsPageSpec): void`: mendaftarkan atau menimpa berdasarkan `id` (mendaftar ulang id yang sama menggantikan komponen; ini yang dipakai tiket 02 untuk id `connectors`).
   - `export const settingsPages: Readable<SettingsPageSpec[]>`: derived, urut `order ?? 0` naik lalu `id` (localeCompare), pola sama dengan `panels()` di `lib/registry.ts`.
   - Halaman terdaftar di tiket ini: `connectors` (title "Connector", order 10), `mcp` (title "MCP", order 20).
3. `frontend/src/modules/settings/Modal.svelte`: props `open: boolean`, `onclose: () => void`, `title?: string`, `children: Snippet`. Esc dan klik backdrop memanggil `onclose`. `role="dialog"`, `aria-modal="true"`, `aria-label` dari `title`.
4. `frontend/src/modules/settings/index.ts`: re-export `openSettings`, `closeSettings`, `settingsOpen`, `activePage`, `registerSettingsPage`, `settingsPages`, `Modal`; mendaftarkan halaman bawaan; memanggil `registerPanel({ id: 'settings-modal', area: 'statusbar', order: 95, component: SettingsModalHost })`.

Yang dipakai (sudah ada):
- `registerPanel` dan tipe `PanelSpec` dari `frontend/src/lib/registry.ts` (area `statusbar`).
- Variabel CSS `--bg-elev`, `--border`, `--fg`, `--fg-muted`, `--bg-hover` (fallback seperti di `SettingsButton.svelte`, mis. `var(--bg-elev, #222)`).
- Modul dimuat otomatis lewat `import.meta.glob('./modules/*/index.ts', { eager: true })` di `frontend/src/main.ts`; tidak perlu mengubah `main.ts`.

Kontrak yang diharapkan tiket 02 (jangan dilanggar): memanggil `registerSettingsPage({ id: 'connectors', title: 'Connector', order: 10, component })` dan menimpa placeholder; memakai `Modal.svelte` untuk dialog; memakai `openSettings('connectors')` bila perlu.

## Fitur

### settings-foundation: Store, registri halaman, dan komponen Modal generik
Deskripsi:
- `store.ts`: dua writable (`settingsOpen`, `activePage`) dan fungsi `openSettings(pageId?)`, `closeSettings()` sesuai bagian Kontrak. Resolusi halaman awal memakai `get(settingsPages)` dari `pages.ts`; hindari impor siklus (pages.ts tidak mengimpor store.ts; store.ts boleh mengimpor pages.ts).
- `pages.ts`: store internal `Record<string, SettingsPageSpec>` (pola `panelStore` di `lib/registry.ts`), `registerSettingsPage`, dan derived `settingsPages` terurut `order ?? 0` lalu `id`.
- `Modal.svelte` (Svelte 5, runes, snippet `children`): bila `open` false, tidak merender apa pun ke DOM (tidak ada elemen fixed penangkap pointer/keyboard, tidak ada listener global aktif). Bila `open` true: backdrop `position: fixed; inset: 0` dengan z-index di atas dropdown (>1000, mis. 2000) dan latar semi transparan; dialog di tengah dengan latar `var(--bg-elev)`, border `1px solid var(--border)`, radius 8px, bayangan, ukuran sekitar 760x520 dengan `max-width: calc(100vw - 48px)` dan `max-height: calc(100vh - 48px)`; header berisi `title` dan tombol tutup (x, `aria-label="Tutup"`). Esc ditangani lewat listener `keydown` di `window` (capture) yang hanya terpasang saat terbuka, memanggil `onclose` dan `stopPropagation` agar Esc tidak diteruskan ke terminal di belakang. Klik backdrop (target tepat backdrop, bukan isi dialog) memanggil `onclose`; gunakan `pointerdown` pada backdrop dan cek `e.target === e.currentTarget` agar seleksi teks yang berakhir di luar dialog tidak menutup. Fokus: saat buka, simpan `document.activeElement`, lalu fokus ke dialog (`tabindex="-1"`) atau elemen fokusable pertama; fokus trap ringan: Tab dan Shift+Tab berputar di dalam dialog; saat tutup (open berubah ke false atau komponen unmount), kembalikan fokus ke elemen tersimpan bila masih ada di DOM.
- Modal tidak mengubah state sesi, tidak meng-unmount pane, dan tidak memanggil fungsi terminal; hanya overlay.

Acceptance criteria:
- [ ] `openSettings()` membuka dan memilih halaman pertama terurut; `openSettings('mcp')` memilih `mcp`; id tak dikenal jatuh ke halaman pertama.
- [ ] `closeSettings()` menutup; `settingsOpen` bisa di-subscribe.
- [ ] `registerSettingsPage` dengan id sama menimpa komponen; `settingsPages` terurut `order` lalu `id`.
- [ ] `Modal.svelte` tidak merender apa pun dan tidak memasang listener global saat `open=false`.
- [ ] Esc dan klik backdrop memanggil `onclose`; klik di dalam dialog tidak menutup.
- [ ] Dialog punya `role="dialog"`, `aria-modal="true"`, fokus masuk saat buka, kembali ke pemicu saat tutup, Tab berputar dalam dialog.
- [ ] Gaya hanya memakai variabel CSS yang ada dengan fallback; `svelte-check` tanpa error baru.

ownedPaths:
- `frontend/src/modules/settings/store.ts`
- `frontend/src/modules/settings/pages.ts`
- `frontend/src/modules/settings/Modal.svelte`

Catatan paralel: fitur ini dikerjakan lebih dulu karena dipakai fitur lain. `settings-shell-pages` dan `settings-gear-entry` dapat dikerjakan paralel bila memegang kontrak di atas (nama ekspor dan props sudah tetap), tetapi baru bisa dibangun penuh setelah fitur ini ada.

### settings-shell-pages: SettingsModal, halaman MCP kerangka, placeholder Connector
Deskripsi:
- `SettingsModal.svelte`: memakai `Modal` dengan `open={$settingsOpen}`, `onclose={closeSettings}`, `title="Pengaturan"`. Isi: layout dua kolom. Sidebar kiri (lebar sekitar 180px, border kanan `var(--border)`) berisi daftar tombol dari `$settingsPages` (`role="tablist"` `aria-orientation="vertical"`, tiap item `role="tab"`, `aria-selected`, `tabindex` roving: 0 untuk aktif, -1 lainnya). Panah atas/bawah memindahkan fokus dan memilih halaman (wrap-around), Home/End ke pertama/terakhir. Item aktif diberi latar `var(--bg-hover)`. Konten kanan (`role="tabpanel"`) merender `<svelte:component>` atau `{@const C = ...}<C />` untuk halaman dengan `id === $activePage`; bila `activePage` tidak ada di daftar (mis. halaman dihapus), tampilkan halaman pertama. Konten bisa discroll (`overflow:auto`).
- `McpPage.svelte`: judul "MCP" dan teks "Pengelolaan server MCP segera hadir". Tanpa logika.
- `ConnectorsPlaceholder.svelte`: judul "Connector" dan teks "Daftar connector segera hadir". Akan diganti tiket 02 lewat registrasi id `connectors` yang sama.
- `index.ts`: impor halaman, panggil `registerSettingsPage({ id: 'connectors', title: 'Connector', order: 10, component: ConnectorsPlaceholder })` dan `registerSettingsPage({ id: 'mcp', title: 'MCP', order: 20, component: McpPage })`. Host panel: komponen kecil (mis. `SettingsModalHost.svelte` boleh dibuat di dalam `index.ts` hanya jika tidak perlu berkas baru; bila perlu berkas, gunakan `SettingsModal.svelte` sendiri sebagai komponen panel) yang merender `SettingsModal` ketika `$settingsOpen` true; daftarkan dengan `registerPanel({ id: 'settings-modal', area: 'statusbar', order: 95, component })`. Karena modal memakai `position: fixed`, penempatan di container statusbar tidak memengaruhi tata letak statusbar; pastikan host tidak menambah lebar atau tinggi pada statusbar saat tertutup (tidak merender elemen saat tertutup).
- Re-export API publik modul (lihat Kontrak butir 4).

Acceptance criteria:
- [ ] Sidebar menampilkan "Connector" lalu "MCP" sesuai order; item aktif ditandai.
- [ ] Klik dan panah atas/bawah (serta Home/End) berpindah halaman; konten kanan mengikuti.
- [ ] Halaman MCP menampilkan teks "Pengelolaan server MCP segera hadir"; halaman Connector menampilkan placeholder.
- [ ] Mendaftarkan ulang id `connectors` dengan komponen lain mengganti isi halaman tanpa mengubah `SettingsModal.svelte` (verifikasi manual atau lewat pembacaan kode).
- [ ] Saat `settingsOpen` false tidak ada elemen modal di DOM dan statusbar tidak berubah.
- [ ] Panel `settings-modal` terdaftar di area `statusbar`; build frontend lulus.

ownedPaths:
- `frontend/src/modules/settings/SettingsModal.svelte`
- `frontend/src/modules/settings/McpPage.svelte`
- `frontend/src/modules/settings/ConnectorsPlaceholder.svelte`
- `frontend/src/modules/settings/index.ts`

Catatan paralel: bisa dikerjakan paralel dengan `settings-gear-entry`. Bergantung pada nama ekspor `settings-foundation` (kontrak sudah tetap); build penuh menunggu `settings-foundation` selesai.

### settings-gear-entry: Entri "Semua pengaturan…" di dropdown gear
Deskripsi:
- Di `frontend/src/modules/topbar/SettingsButton.svelte`, tambahkan impor `import { openSettings } from '../settings';` (lewat barrel `index.ts` modul settings; jangan mengimpor file dalamnya langsung dan jangan membuat impor dari settings ke topbar agar tidak terjadi siklus).
- Tambahkan satu grup baru pada array `groups` (setelah grup "Susun grid", sehingga otomatis dipisah separator oleh `{#if gi > 0}` yang sudah ada): `[{ label: 'Semua pengaturan…', run: () => openSettings() }]`. Fungsi `act()` yang sudah ada menutup dropdown setelah `run()` sukses.
- Fokus: karena dropdown ditutup dan modal dibuka pada klik yang sama, pemicu yang dikembalikan fokusnya saat modal tutup adalah elemen yang aktif saat `openSettings` dipanggil (item menu yang akan dihapus dari DOM). Agar fokus kembali ke tombol gear, fitur ini harus memanggil `openSettings()` setelah memindahkan fokus ke tombol gear (mis. beri `bind:this={gearBtn}`, panggil `gearBtn?.focus()` sebelum `openSettings()` di `run`), tanpa mengubah perilaku lain.
- Jangan mengubah bagian lain berkas (markup, gaya, logika menu) selain impor, ref tombol gear, dan array `groups`. Tidak ada unmount pane, jadi sesi terminal yang berjalan tidak terganggu.

Acceptance criteria:
- [ ] Dropdown gear menampilkan grup baru berisi "Semua pengaturan…" dengan separator di atasnya.
- [ ] Klik item menutup dropdown dan membuka modal pengaturan di halaman Connector.
- [ ] Navigasi panah dan Esc pada dropdown tetap bekerja seperti sebelumnya.
- [ ] Setelah modal ditutup, fokus kembali ke tombol gear.
- [ ] Tidak ada impor siklus; `svelte-check` dan build frontend lulus.
- [ ] Sesi terminal yang sedang berjalan tetap berjalan sebelum, selama, dan sesudah modal dibuka.

ownedPaths:
- `frontend/src/modules/topbar/SettingsButton.svelte` (perluasan terbatas: impor, ref tombol, array `groups`)

Catatan paralel: bisa dikerjakan paralel dengan fitur lain di epic ini; hasilnya baru terlihat penuh setelah `settings-foundation` dan `settings-shell-pages` ada.

## Path yang dimiliki
Hanya path berikut yang boleh disentuh oleh tiket ini:
- `frontend/src/modules/settings/index.ts`
- `frontend/src/modules/settings/store.ts`
- `frontend/src/modules/settings/pages.ts`
- `frontend/src/modules/settings/Modal.svelte`
- `frontend/src/modules/settings/SettingsModal.svelte`
- `frontend/src/modules/settings/McpPage.svelte`
- `frontend/src/modules/settings/ConnectorsPlaceholder.svelte`
- `frontend/src/modules/topbar/SettingsButton.svelte`

Catatan kepemilikan lintas tiket: `ConnectorsPlaceholder.svelte` juga disentuh tiket 02 (diganti/dihapus di sana); `frontend/src/modules/settings/connectors/` milik tiket 02 dan tidak boleh dibuat di sini.

## Wiring
Langkah penutup tiket ini, tanpa bergantung pada tiket lain:
1. `modules/settings/index.ts` mendaftarkan halaman `connectors` (placeholder) dan `mcp` lewat `registerSettingsPage`, lalu `registerPanel({ id: 'settings-modal', area: 'statusbar', order: 95, component })` sehingga host modal ikut dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts')` di `main.ts` dan dirender oleh container panel `statusbar` yang sudah ada (cek bahwa komponen yang me-render `panels('statusbar')` di shell memang memuat panel baru ini; bila host statusbar membatasi tata letak, modal tetap `position: fixed` sehingga tidak terpengaruh).
2. `SettingsButton.svelte` memanggil `openSettings()` dari `'../settings'` lewat grup baru di dropdown gear.
3. Verifikasi manual di aplikasi yang berjalan: buka gear, klik "Semua pengaturan…", ganti halaman Connector/MCP dengan klik dan panah, tutup dengan Esc, klik backdrop, dan tombol x; pastikan fokus kembali ke gear dan terminal yang berjalan tetap menerima input sesudah modal tertutup.

## Guard
- `go build ./...` berhasil (tidak ada perubahan Go; hanya memastikan tidak rusak).
- `make build` berhasil, termasuk build frontend (`vite build`). Bila tersedia, `npm run check` (svelte-check) di `frontend/` tanpa error baru.
- Tanpa unit test.
