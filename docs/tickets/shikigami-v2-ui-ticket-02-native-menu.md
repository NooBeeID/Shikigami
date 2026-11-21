---
id: shikigami-v2-ui-ticket-02-native-menu
title: Menu native, zoom, dan Open Folder
spec: docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md
status: ready
wave: 2
depends_on: [shikigami-v2-ui-ticket-01-app-install]
---
# Menu native, zoom, dan Open Folder

## Tujuan

Membuat Shikigami berperilaku seperti aplikasi macOS sungguhan: ada menu native (App, File, Edit, View, Window) dengan pintasan keyboard, zoom Cmd+= / Cmd+- / Cmd+0 yang mengubah ukuran UI dan font terminal serta diingat setelah restart, dan cara memasukkan folder sebagai project tanpa mengetik path (dialog Cmd+O, "New Workspace from Folder...", dan drag folder dari Finder). Menu native diperlukan karena WKWebView bisa menelan tombol Cmd sebelum sampai ke halaman; menu mengirim event Wails ke frontend. Epic ini juga menetapkan kontrak event menu, bridge dialog, dan konfigurasi aplikasi (`config.json`) yang dipakai tiket-tiket v2 berikutnya (sidebar toggle, persist jendela, palet perintah).

Tiket ini dibangun di atas UI tab v1 yang masih ada (`modules/workspace`, `modules/tabs`, `modules/terminal`). Tidak ada tampilan baru selain perilaku di atas.

## Hasil yang bisa dijalankan

Setelah tiket ini selesai (di atas tiket 01, tetapi tidak membutuhkan tiket 01 untuk jalan; boleh dicoba dari `build/bin/shikigami` maupun dari `Shikigami.app` bila sudah dirakit):

- Bilah menu macOS menampilkan Shikigami (About, Hide, Quit), File, Edit, View, Window dengan pintasan yang benar.
- Cmd+O membuka dialog pilih folder macOS; folder yang dipilih muncul sebagai project di sidebar v1 pada workspace aktif.
- File > "New Workspace from Folder..." (Cmd+Shift+O) membuat workspace baru bernama folder itu, berisi folder itu sebagai project, lalu menjadikannya aktif.
- Drag folder dari Finder ke jendela menambahkannya sebagai project di workspace aktif (jendela menandai area drop saat diseret).
- Cmd+= / Cmd+- memperbesar/memperkecil UI dan font semua terminal dengan langkah 10% (rentang 50-300%), Cmd+0 mengembalikan ke 100%. Nilainya tersimpan di `<DataDir>/config.json` dan dipulihkan saat aplikasi dibuka lagi.
- Copy/paste (Cmd+C / Cmd+V) tetap bekerja di terminal berkat menu Edit standar.
- Tombol "Browse" di sidebar v1 (yang sebelumnya menyembunyikan diri karena `OpenDirectoryDialog` tidak ada di bridge) sekarang muncul dan bekerja. Input path teks tetap ada sebagai cadangan.
- Item View > Toggle Left/Right Sidebar ada dan mengirim event, tetapi belum berefek sampai tiket 04 (shell-sidebars) mendengarkannya.

`make build` harus lulus dan aplikasi harus bisa dijalankan.

## Batasan

In scope:
- `app/menu.go`: menu native dan accelerator. `app/main.go`: pemasangan menu, konfigurasi jendela (judul, ukuran awal dan minimum, warna latar), aktivasi file-drop.
- `app/bridge.go`: event Wails dari Go ke frontend, method bridge `OpenDirectoryDialog`, `AppConfigGet`, `AppConfigSet`, dan hook startup.
- `app/dialog.go`: dialog direktori dan penanganan file-drop sisi Go.
- `internal/appconfig/`: baca/tulis `config.json`.
- `frontend/src/modules/appmenu/` (event bus + pembungkus bridge), `modules/zoom/`, `modules/openfolder/`.
- Perluasan kecil di `frontend/src/modules/terminal/` (satu file baru dan dua sunting minimal) yang disebut eksplisit di Wiring.

Out of scope:
- Perilaku nyata toggle sidebar (tiket 04), palet perintah Cmd+K (tiket 04), split/maksimalkan pane dan pintasannya Cmd+D / Cmd+Shift+D (tiket 03).
- Menerapkan posisi/ukuran jendela tersimpan: field `window` di `config.json` didefinisikan di sini, tetapi `app/window.go` (menulis dan memulihkannya) milik tiket 06.
- Mengubah `workspaces.json`, API `workspace.*` di daemon, atau modul daemon apa pun (epic ini tidak menyentuh `cmd/` maupun `internal/` selain `internal/appconfig/`).
- Menu konteks, pengaturan (settings UI), tema, banyak jendela, Windows/Linux.
- Mengubah `frontend/src/lib/ipc.ts`, `frontend/src/lib/registry.ts`, `frontend/src/main.ts` (modul dimuat otomatis lewat glob), dan `frontend/package.json` (tanpa dependency baru).
- Unit test (guard hanya build).
- Flag izin agent (`--dangerously-skip-permissions`, `--dangerously-bypass-approvals-and-sandbox`) tidak disentuh: epic ini tidak berhubungan dengan `internal/provider`.

## Kontrak

### Yang dipakai (sudah ada)

- Wails v2.17.1 (`go.mod`): `runtime.EventsEmit`, `runtime.OpenDirectoryDialog`, `runtime.OnFileDrop`, `options.App.Menu`, `options.App.DragAndDrop{EnableFileDrop: true}`, paket `pkg/menu` dan `pkg/menu/keys` (`menu.AppMenu()`, `menu.EditMenu()`, `menu.WindowMenu()`, `keys.CmdOrCtrl`, `keys.Combo`, `keys.OptionOrAlt`, `keys.Shift`). Signature persis dibaca dari modul Go di `$GOMODCACHE`.
- `app/bridge.go` v1: `Bridge` dengan `SetContext(ctx)`, `Call`, `Close`; ctx disimpan di `ctxMu`. `app/main.go` v1: `wails.Run` dengan `OnStartup: bridge.SetContext`.
- `internal/paths.DataDir()` untuk lokasi `config.json`.
- Frontend: `modules/workspace/errors.ts` sudah memeriksa `window.go.main.Bridge.OpenDirectoryDialog` (fungsi `hasNativeFolderDialog`, `pickFolder`); menambahkan method itu ke `Bridge` otomatis mengaktifkan tombol Browse v1.
- Store v1 di `frontend/src/lib/stores` (diimpor dari `lib/stores`): `activeWorkspace`, `activeWorkspaceId`, `workspaces`, `loadWorkspaces()`, `selectWorkspace(id)`, `createWorkspace(name)`, `addProject(path, name?)`. Daemon yang dipanggil lewat `lib/ipc.ts` (`call`, `onEvent`) tidak diubah.

### Yang disediakan

**Event Wails (Go ke frontend)**, nama final dan tidak berubah (dipakai tiket lain):

| Event | Payload | Pengirim |
| --- | --- | --- |
| `menu:zoom` | `{action: 'in' \| 'out' \| 'reset'}` | menu View |
| `menu:open-folder` | tanpa payload | menu File |
| `menu:new-workspace-from-folder` | tanpa payload | menu File |
| `menu:toggle-sidebar-left` | tanpa payload | menu View (placeholder, dipakai tiket 04) |
| `menu:toggle-sidebar-right` | tanpa payload | menu View (placeholder, dipakai tiket 04) |
| `files:dropped` | `{paths: string[]}` (hanya path absolut direktori) | file-drop Wails |

Konstanta nama event di Go didefinisikan sekali di `app/bridge.go`: `EventMenuZoom`, `EventMenuOpenFolder`, `EventMenuNewWorkspaceFromFolder`, `EventMenuToggleSidebarLeft`, `EventMenuToggleSidebarRight`, `EventFilesDropped`.

**Go, `app/bridge.go`:**
- `func (b *Bridge) emit(name string, data ...any)`: `runtime.EventsEmit` memakai ctx yang tersimpan; tidak melakukan apa-apa (tanpa panic) bila ctx belum di-set. Aman dipanggil dari goroutine mana pun.
- Hook startup: `func registerStartupHook(fn func(ctx context.Context, b *Bridge))` dan pemanggilannya di dalam `SetContext` (setelah ctx tersimpan, dijalankan berurutan). Fitur lain mendaftar lewat `init()` di file masing-masing sehingga `main.go` tidak perlu tahu.
- Method bridge (otomatis tersedia di `window.go.main.Bridge`):
  - `OpenDirectoryDialog() (string, error)`: dialog direktori macOS (judul "Pilih folder project", `CanCreateDirectories: true`); mengembalikan path absolut atau string kosong bila dibatalkan (pembatalan bukan error).
  - `AppConfigGet() (appconfig.Config, error)`.
  - `AppConfigSet(patch map[string]any) (appconfig.Config, error)`: gabungkan patch ke konfigurasi tersimpan, simpan, kembalikan hasil lengkap.

**Go, `internal/appconfig`:**
```go
type Window struct { X, Y, W, H int }          // json: x, y, w, h
type Config struct {
    Zoom   int     `json:"zoom"`              // persen, 50..300, bawaan 100
    Window *Window `json:"window,omitempty"`  // belum dipakai tiket ini
}
func Default() Config
type Store struct{ /* mutex, path */ }
func Open(path string) *Store                  // tidak membaca disk sampai Get/Patch
func (s *Store) Get() (Config, error)          // file tidak ada atau rusak -> Default() (file rusak dicatat lewat log, tidak error)
func (s *Store) Patch(patch map[string]any) (Config, error)
func DefaultPath() (string, error)             // <DataDir>/config.json
```
Aturan `Patch`: gabung dangkal per kunci tingkat atas (kunci `window` digabung satu tingkat ke dalam); kunci tak dikenal diabaikan; `zoom` di luar 50..300 di-clamp (bukan error), dibulatkan ke integer; tulis atomik (file sementara di folder yang sama lalu `rename`), izin 0600, dilindungi mutex. Field baru yang ditambahkan tiket lain harus tetap lolos dari baca-tulis (jangan membuang kunci yang tak dikenal dari file yang ada: baca file ke `map[string]any` dahulu, gabung, tulis kembali).

**Frontend, `frontend/src/modules/appmenu/index.ts`** (diekspor, dipakai modul lain tanpa menyentuh file modul lain):
```ts
export type MenuEventName =
  | 'menu:zoom' | 'menu:open-folder' | 'menu:new-workspace-from-folder'
  | 'menu:toggle-sidebar-left' | 'menu:toggle-sidebar-right' | 'files:dropped';
export interface MenuPayloads {
  'menu:zoom': { action: 'in' | 'out' | 'reset' };
  'files:dropped': { paths: string[] };
  /* event lain: undefined */
}
export function onMenuEvent<K extends MenuEventName>(name: K, fn: (p: MenuPayloads[K]) => void): () => void; // mengembalikan unsubscribe
export function emitMenuEvent<K extends MenuEventName>(name: K, p?: MenuPayloads[K]): void;                // untuk pemicu dari dalam UI (mis. tombol, palet perintah)
export interface AppConfig { zoom: number; window?: { x: number; y: number; w: number; h: number } }
export function getAppConfig(): Promise<AppConfig>;                     // di luar Wails: mengembalikan {zoom: 100}
export function setAppConfig(patch: Partial<AppConfig>): Promise<AppConfig>;
export function pickDirectory(): Promise<string | null>;                // null bila dibatalkan atau di luar Wails
```
Event bus lokal kecil (satu `Map` nama ke set callback). `appmenu` mendaftarkan satu `window.runtime.EventsOn` per nama event saat modul dimuat dan meneruskannya ke bus; bila `window.runtime` tidak ada (mode browser/dev), tidak ada error dan bus tetap bisa dipicu lewat `emitMenuEvent`. Pembungkus bridge memanggil `window.go.main.Bridge.*` langsung (tanpa mengubah `lib/ipc.ts`) dan menelan error menjadi nilai bawaan.

**Frontend, modul terminal (perluasan kecil, lihat Wiring):** file baru `frontend/src/modules/terminal/font-scale.ts`:
```ts
export const BASE_FONT_SIZE = 13;
export function registerTerminalInstance(term: { options: { fontSize?: number } }, refit: () => void): () => void; // mengembalikan unregister
export function setTerminalFontScale(percent: number): void;   // menyimpan skala, set fontSize = round(13 * percent / 100) ke semua instance terdaftar, lalu panggil refit masing-masing
export function currentTerminalFontSize(): number;             // untuk instance yang baru dibuat
```
Diekspor ulang dari `modules/terminal/index.ts` sebagai `setTerminalFontScale`.

**Pintasan (macOS, accelerator menu native):**

| Item | Pintasan |
| --- | --- |
| File > Open Folder... | Cmd+O |
| File > New Workspace from Folder... | Cmd+Shift+O |
| View > Zoom In | Cmd+= |
| View > Zoom Out | Cmd+- |
| View > Actual Size | Cmd+0 |
| View > Toggle Left Sidebar | Cmd+B |
| View > Toggle Right Sidebar | Cmd+Option+B |
| Edit (standar) | Cmd+Z/Shift+Z/X/C/V/A |
| Window (standar) | Cmd+M, Cmd+W, dll. dari `menu.WindowMenu()` |

Cmd+D, Cmd+Shift+D, dan Cmd+K sengaja tidak dipakai (milik tiket 03 dan 04).

**File konfigurasi:** `<DataDir>/config.json`, contoh `{"zoom":110}`; `window` ditambahkan oleh tiket 06.

## Fitur

Urutan: `menu-bridge-contract` adalah foundation dan digabung lebih dulu (tiga fitur lain mengimpor nama dari kontraknya). Setelah itu `native-menu-impl`, `zoom`, dan `open-folder` bisa dikerjakan paralel karena tidak berbagi file; mereka hanya bergantung pada nama ekspor yang tertulis di Kontrak, sehingga tiap fitur tetap lolos build sendiri-sendiri (event yang belum dikirim atau didengar tidak membuat error).

### menu-bridge-contract: Foundation: kontrak event menu dan bridge dialog

Deskripsi: tetapkan semua yang dibagi antar fitur epic ini.
(1) `app/bridge.go`: konstanta nama event, `emit`, mekanisme `registerStartupHook` yang dijalankan di `SetContext`, method `OpenDirectoryDialog`, `AppConfigGet`, `AppConfigSet` dengan `appconfig.Store` yang dibuat malas (`sync.Once`, path dari `appconfig.DefaultPath()`; bila `DataDir` gagal, kembalikan error yang jelas, jangan panic).
(2) `internal/appconfig`: tipe, `Store`, `Patch` sesuai aturan di Kontrak.
(3) `frontend/src/modules/appmenu/`: `index.ts` (ekspor di Kontrak), `bus.ts` (bus), `bridge.ts` (pembungkus `OpenDirectoryDialog`/`AppConfigGet`/`AppConfigSet`), `types.ts`. `index.ts` meneruskan semua event dari tabel ke bus. Modul ini tidak mendaftar panel apa pun (tidak ada UI).
Pembungkus frontend `getAppConfig`/`setAppConfig` memanggil method bridge dan bila method tidak ada (mode dev) mengembalikan `{zoom: 100}` tanpa melempar error.

Acceptance criteria:
- [ ] `go build ./...` lolos; `internal/appconfig` hanya memakai pustaka standar dan `internal/paths`.
- [ ] `Patch({"zoom": 999})` menghasilkan `zoom: 300`; `Patch({"zoom": 10})` menghasilkan 50; file dibuat atomik dengan izin 0600.
- [ ] File `config.json` yang rusak atau kosong tidak membuat `Get` gagal (mengembalikan bawaan) dan tidak menghapus kunci tak dikenal pada `Patch` berikutnya bila file valid.
- [ ] Panggilan `emit` sebelum `SetContext` tidak panic dan tidak mengirim apa-apa.
- [ ] Hook yang didaftarkan lewat `registerStartupHook` dipanggil tepat sekali per `SetContext` dengan ctx yang benar.
- [ ] `OpenDirectoryDialog` mengembalikan string kosong (bukan error) saat dibatalkan.
- [ ] `modules/appmenu/index.ts` mengekspor semua simbol di Kontrak; `onMenuEvent` mengembalikan fungsi unsubscribe yang berfungsi; di luar Wails modul dimuat tanpa error.
- [ ] Build frontend (`vite build`) lolos tanpa dependency baru.
- [ ] Tidak mengedit `lib/ipc.ts`, `lib/registry.ts`, `main.ts`.

ownedPaths: `app/bridge.go`, `internal/appconfig/`, `frontend/src/modules/appmenu/`. Foundation: digabung sebelum tiga fitur lain.

### native-menu-impl: Menu native macOS dengan accelerator

Deskripsi: `app/menu.go` menyediakan `func buildMenu(b *Bridge) *menu.Menu` yang menyusun: menu App (`menu.AppMenu()`), File, Edit (`menu.EditMenu()`), View, Window (`menu.WindowMenu()`).
- File: "Open Folder..." (Cmd+O) memanggil `b.emit(EventMenuOpenFolder)`; "New Workspace from Folder..." (Cmd+Shift+O) memanggil `b.emit(EventMenuNewWorkspaceFromFolder)`.
- View: "Zoom In" (Cmd+=) memanggil `b.emit(EventMenuZoom, map[string]string{"action":"in"})`; "Zoom Out" (Cmd+-) dengan `"out"`; "Actual Size" (Cmd+0) dengan `"reset"`; pemisah; "Toggle Left Sidebar" (Cmd+B) dan "Toggle Right Sidebar" (Cmd+Option+B) memicu event toggle masing-masing. Tambahan boleh: "Enter Full Screen" lewat `runtime.WindowToggleFullscreen`.
- Callback tidak boleh memblokir (hanya `emit`); dialog tidak dibuka dari Go di callback menu, karena frontend yang memutuskan aksi (supaya palet perintah tiket 04 bisa memicu aksi yang sama lewat `emitMenuEvent`).
- Cmd+= pada papan ketik US adalah tombol `=`; pakai accelerator `keys.CmdOrCtrl("=")`. Bila dengan itu "Zoom In" tidak menyala pada Cmd+Shift+= (Cmd++), tambahkan item tersembunyi/duplikat hanya bila terbukti perlu.

`app/main.go`: set `Menu: buildMenu(bridge)`, `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}`, `Title: "Shikigami"`, ukuran awal 1200x800, `MinWidth: 800`, `MinHeight: 500`, `BackgroundColour` gelap (cocokkan `--bg` tema gelap di `frontend/src/styles/theme.css`) agar tidak berkedip putih saat start. `OnStartup` tetap memanggil `bridge.SetContext(ctx)` (yang menjalankan hook startup). Jangan menghapus `Bind`, `OnShutdown`, atau `AssetServer`.

Acceptance criteria:
- [ ] `go build ./...` lolos.
- [ ] Di macOS, menu File, Edit, View, Window tampil di bilah menu dengan pintasan seperti tabel di Kontrak.
- [ ] Memilih tiap item mengirim event dengan nama dan payload sesuai Kontrak (dibuktikan lewat `console.log` sementara atau `onMenuEvent` di devtools; log sementara dibuang sebelum selesai).
- [ ] Cmd+C dan Cmd+V tetap bekerja di terminal xterm (Edit menu ada).
- [ ] Cmd+= / Cmd+- / Cmd+0 sampai ke frontend sebagai event menu walau fokus ada di terminal.
- [ ] Menu tidak memakai Cmd+D, Cmd+Shift+D, atau Cmd+K.
- [ ] Jendela berjudul "Shikigami", 1200x800, tidak bisa lebih kecil dari 800x500.
- [ ] `EnableFileDrop` aktif (tanpa mematikan drag-and-drop internal webview yang dipakai tiket 03 nanti: jangan set `DisableWebViewDrop`).

ownedPaths: `app/menu.go`, `app/main.go`. Paralel dengan `zoom` dan `open-folder` setelah foundation.

### zoom: Zoom UI dan font terminal

Deskripsi: `modules/zoom/` berlangganan `menu:zoom` lewat `onMenuEvent`. State: `percent` (integer), bawaan 100. Aksi: `in` = +10, `out` = -10, `reset` = 100, selalu di-clamp ke 50..300. Efek (satu fungsi `applyZoom(percent)`):
1. Skala UI: set `--ui-scale` (= percent/100) di `document.documentElement` dan terapkan ke seluruh UI (pendekatan yang disarankan: CSS `zoom: var(--ui-scale)` pada elemen `#app`; WKWebView mendukung `zoom`). Gaya ada di `modules/zoom/zoom.css` yang diimpor dari `index.ts`.
2. Font terminal: panggil `setTerminalFontScale(percent)` dari `modules/terminal` (lihat Wiring). Agar terminal tidak terskala dua kali oleh CSS `zoom`, kontainer xterm diberi `zoom: calc(1 / var(--ui-scale))` (aturan ada di `zoom.css`, selektor terhadap kelas kontainer terminal yang ada di `TerminalTab.svelte`; baca file itu untuk nama kelasnya) sehingga ukuran sel dihitung xterm pada skala 1 dan font efektifnya 13 x skala. Bila saat implementasi pendekatan ini tidak berjalan baik, pembuat boleh memilih pendekatan lain, dengan syarat: tidak ada skala ganda, `fit` dihitung ulang sehingga kolom/baris benar, dan hasilnya terlihat pada sidebar maupun terminal.
3. Persisten: setelah perubahan, simpan `setAppConfig({zoom: percent})` dengan debounce sekitar 300 ms (bukan tiap tekan tombol); saat modul dimuat, `getAppConfig()` lalu `applyZoom(config.zoom)`. Sebelum konfigurasi terbaca, tampilan memakai 100% (tanpa kedip berarti: terapkan segera begitu terbaca).
4. Instance terminal yang dibuat sesudah zoom (sesi baru) langsung memakai ukuran font saat itu lewat `currentTerminalFontSize()`.
Tidak ada tombol UI; hanya menu native/pintasan. Modul mengekspor `getZoomPercent(): number` dan store baca-saja `zoomPercent` untuk dipakai tiket lain (mis. palet perintah).

Acceptance criteria:
- [ ] `vite build` lolos.
- [ ] Cmd+= dari 100% menghasilkan 110%, dan terus sampai berhenti di 300%; Cmd+- berhenti di 50%; Cmd+0 kembali ke 100%.
- [ ] Teks sidebar dan font terminal sama-sama berubah ukuran; jumlah kolom/baris terminal dihitung ulang (output tidak terpotong dan tidak ada ruang kosong besar), `session.resize` terkirim lewat jalur resize terminal yang ada.
- [ ] Nilai tersimpan di `<DataDir>/config.json` dan dipulihkan setelah aplikasi ditutup dan dibuka lagi.
- [ ] Mengetik cepat Cmd+= berkali-kali menghasilkan paling banyak sedikit penulisan config (debounce), bukan satu per tekan.
- [ ] Sesi baru yang dibuka setelah zoom memakai ukuran font yang sama dengan terminal lain.
- [ ] Zoom tidak mengubah isi `workspaces.json` dan tidak mengganggu sesi (tidak ada reattach atau reset terminal).
- [ ] Tidak ada polling atau `setInterval`.
- [ ] Hanya menyentuh `modules/zoom/`; perubahan di `modules/terminal/` hanya yang tercantum di Wiring.

ownedPaths: `frontend/src/modules/zoom/`. Paralel dengan `native-menu-impl` dan `open-folder` setelah foundation. Penyambungan ke terminal dilakukan di Wiring.

### open-folder: Open Folder dan drop folder

Deskripsi:
- `app/dialog.go` (package `main`): fungsi internal untuk dialog direktori dipakai `Bridge.OpenDirectoryDialog` bila diperlukan (boleh kosong bila `bridge.go` sudah memanggil runtime langsung; jangan duplikasi), dan penanganan file-drop: lewat `init()` memanggil `registerStartupHook(func(ctx, b){ runtime.OnFileDrop(ctx, func(x, y int, paths []string){ ... }) })`. Callback menyaring `paths` menjadi hanya direktori (`os.Stat(...).IsDir()`, path dibersihkan dengan `filepath.Clean`, duplikat dibuang); bila hasil tidak kosong, `b.emit(EventFilesDropped, map[string]any{"paths": dirs})`. Bila semua yang dijatuhkan adalah file biasa, tidak ada event (frontend tidak mengetahuinya; penolakan ditampilkan lewat jalur lain: kirim juga `files:dropped` dengan `paths: []` agar UI bisa memberi tahu "Hanya folder yang bisa ditambahkan"; pilih satu perilaku ini dan terapkan konsisten dengan frontend).
- `frontend/src/modules/openfolder/index.ts` (dimuat otomatis oleh glob) mendengarkan:
  - `menu:open-folder`: `pickDirectory()`; bila `null` berhenti diam-diam; bila tidak ada workspace aktif, jalankan alur "new workspace from folder"; selain itu `addProject(path)` (store v1, mengikuti API `workspace.*` yang sudah ada). Project yang sudah ada di workspace ditampilkan sebagai pesan, bukan crash.
  - `menu:new-workspace-from-folder`: `pickDirectory()`; `createWorkspace(basename(path))`, `selectWorkspace(ws.id)`, lalu `addProject(path)`. `basename` menangani trailing slash dan root.
  - `files:dropped`: untuk tiap path (berurutan), `addProject(path)`; kegagalan satu path tidak menghentikan yang lain; di akhir tampilkan ringkasan bila ada yang gagal. Bila tidak ada workspace aktif, buat workspace dari folder pertama seperti alur di atas.
- Umpan balik singkat: komponen kecil `Toast.svelte` didaftarkan lewat `registerPanel({id: 'openfolder-toast', area: 'statusbar', component: Toast, order: 90})` yang menampilkan pesan error/ringkasan beberapa detik (memakai `errMessage` bergaya v1; boleh menyalin logika kecilnya, jangan mengedit file `modules/workspace/`).
- Area drop: `modules/openfolder/drop.css` (diimpor dari `index.ts`) memberi `--wails-drop-target: drop` pada `body` dan gaya sorot untuk kelas `.wails-drop-target-active` (garis putus beraksen di tepi jendela), sesuai mekanisme file-drop Wails v2.
- Input path teks v1 dan tombol Browse v1 tetap berfungsi tanpa diubah.

Acceptance criteria:
- [ ] `go build ./...` dan `vite build` lolos.
- [ ] Cmd+O membuka dialog folder macOS; memilih folder menambahkannya ke project workspace aktif dan terlihat di sidebar v1; Batal tidak memunculkan error.
- [ ] "New Workspace from Folder..." membuat workspace bernama folder (mis. `/Users/x/code/core` menjadi "core"), menjadikannya aktif, dan project-nya berisi folder itu.
- [ ] Menyeret satu atau beberapa folder dari Finder menambahkan semuanya sebagai project; file biasa diabaikan dengan pesan; area drop tersorot saat menyeret.
- [ ] Menambahkan folder yang sudah menjadi project tidak menggandakan dan tidak crash (pesan jelas).
- [ ] Tanpa workspace aktif, Cmd+O atau drop tetap menghasilkan workspace yang valid.
- [ ] Input path teks v1 dan tombol Browse v1 masih bekerja (Browse sekarang muncul karena bridge punya `OpenDirectoryDialog`).
- [ ] Tidak mengedit file di `modules/workspace/`, `lib/stores/`, atau `lib/ipc.ts`.

ownedPaths: `app/dialog.go`, `frontend/src/modules/openfolder/`. Paralel dengan `native-menu-impl` dan `zoom` setelah foundation.

## Path yang dimiliki

Hanya ini yang boleh disentuh oleh tiket ini:

- `app/menu.go`
- `app/dialog.go`
- `app/main.go`
- `app/bridge.go`
- `frontend/src/modules/appmenu/`
- `frontend/src/modules/zoom/`
- `frontend/src/modules/openfolder/`
- `internal/appconfig/`

Perluasan terbatas di file tiket lain, hanya yang tertulis di Wiring: `frontend/src/modules/terminal/font-scale.ts` (baru), `frontend/src/modules/terminal/index.ts` (satu baris ekspor), `frontend/src/modules/terminal/TerminalTab.svelte` (registrasi instance dan ukuran font awal).

## Wiring

Langkah penutup, dikerjakan setelah keempat fitur tergabung, agar hasilnya terlihat dan jalan tanpa bergantung pada tiket lain:

1. **Modul dimuat otomatis**: `appmenu`, `zoom`, dan `openfolder` punya `index.ts` sehingga `import.meta.glob('./modules/*/index.ts')` di `main.ts` memuatnya; tidak ada penyuntingan `main.ts`. Pastikan `zoom` dan `openfolder` mengimpor `onMenuEvent`/`getAppConfig`/`setAppConfig`/`pickDirectory` dari `../appmenu` (bukan sebaliknya), sehingga tidak ada impor melingkar.
2. **Satu-satunya perluasan di modul terminal** (titik ekstensi untuk zoom; disebut eksplisit karena file itu milik tiket lain, dan perubahannya sengaja minimal):
   - Tambah `frontend/src/modules/terminal/font-scale.ts` sesuai Kontrak.
   - Di `modules/terminal/index.ts` tambahkan `export { setTerminalFontScale, currentTerminalFontSize, registerTerminalInstance } from './font-scale';` tanpa mengubah pendaftaran tipe tab `session` yang ada.
   - Di `TerminalTab.svelte`: set `fontSize: currentTerminalFontSize()` saat `new Terminal({...})` menggantikan angka 13 tetap; setelah terminal dan fit addon dibuat panggil `registerTerminalInstance(term, () => fit.fit())` dan simpan fungsi unregister yang dipanggil di cleanup `onDestroy` yang sudah ada. Tidak ada perubahan lain (attach, input, resize, penanganan `session.exit` tidak disentuh). Tiket 03 (pane-layout) yang kemudian merombak modul ini wajib mempertahankan `font-scale.ts` dan pendaftaran instance ini.
3. **Pemasangan Go**: `app/main.go` memasang `buildMenu(bridge)` dan `EnableFileDrop`; `app/dialog.go` mendaftarkan penanganan drop lewat `registerStartupHook`, dijalankan dari `Bridge.SetContext` pada `OnStartup`.
4. **Toast** `openfolder-toast` terdaftar sebagai panel `statusbar` lewat `registerPanel` (titik ekstensi tiket 01 pertama); statusbar v1 sudah merender panel itu.
5. **Verifikasi manual di aplikasi nyata** (bukan hanya build): jalankan `make build`, buka `build/bin/shikigami` (atau `Shikigami.app` bila tiket 01 sudah tergabung dan dirakit), lalu periksa: menu dan pintasan tampil; Cmd+O menambah project; New Workspace from Folder; drop folder dari Finder; Cmd+= / Cmd+- / Cmd+0 saat fokus di terminal; tutup dan buka lagi, zoom tetap; Cmd+C/Cmd+V di terminal; Browse v1 muncul. Toggle sidebar hanya terbukti sampai level event (tidak ada efek visual sampai tiket 04).
6. Pastikan flag izin agent di `internal/provider` tidak tersentuh (epic ini tidak mengubah berkas itu).

## Guard

Cukup build berhasil, tanpa unit test:

- `go build ./...`
- `cd frontend && npm run build` (`vite build`)
- `make build` (rantai penuh: frontend, daemon/ctl, aplikasi Wails) lulus, dan aplikasi bisa dijalankan.
