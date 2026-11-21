# Overview: Menu native, zoom, dan Open Folder

Tiket: `docs/tickets/shikigami-v2-ui-ticket-02-native-menu.md`
Spec: `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md` (bagian 6, 9)
Module Go: `github.com/NooBeeID/shikigami` (Go 1.26, Wails v2.17.1). Frontend: Svelte + Vite di `frontend/`.

## 1. Ringkasan epic dan urutan eksekusi

Shikigami mendapat menu native macOS (App, File, Edit, View, Window), zoom UI + font terminal (Cmd+= / Cmd+- / Cmd+0, tersimpan di `<DataDir>/config.json`), dan Open Folder (dialog Cmd+O, New Workspace from Folder, drop folder dari Finder). Menu hanya meng-emit event Wails; frontend yang memutuskan aksi. Epic ini tidak menyentuh daemon (`cmd/`, `internal/*` selain `internal/appconfig/`), `internal/provider`, `lib/ipc.ts`, `lib/registry.ts`, `main.ts`, `package.json`.

Urutan:

1. **Foundation** `menu-bridge-contract` (plan: `00-foundation.md`), digabung lebih dulu.
2. **Fitur paralel** (tidak berbagi file): `native-menu-impl`, `zoom`, `open-folder`.
3. **Wiring** (satu agent, setelah semua tergabung): perluasan modul terminal + verifikasi build penuh.

Tiap fitur paralel harus lolos build sendiri walau fitur lain belum ada: event yang belum dikirim/didengar tidak menimbulkan error, dan simbol lintas fitur yang belum ada (mis. `setTerminalFontScale`) ditangani sesuai bagian 3 (zoom).

## 2. Kontrak bersama

### 2.1 Event Wails (Go ke frontend), nama final

| Event | Payload | Pengirim |
| --- | --- | --- |
| `menu:zoom` | `{action: 'in' \| 'out' \| 'reset'}` | menu View |
| `menu:open-folder` | tanpa payload | menu File |
| `menu:new-workspace-from-folder` | tanpa payload | menu File |
| `menu:toggle-sidebar-left` | tanpa payload | menu View (placeholder tiket 04) |
| `menu:toggle-sidebar-right` | tanpa payload | menu View (placeholder tiket 04) |
| `files:dropped` | `{paths: string[]}` | file-drop Wails |

Keputusan perilaku `files:dropped` (ambiguitas tiket, diputuskan di sini): callback drop Go SELALU emit `files:dropped` bila `paths` yang dijatuhkan tidak kosong, dengan `paths` hanya berisi direktori (bisa `[]` bila semuanya file biasa). Frontend: bila `paths` kosong, tampilkan toast "Hanya folder yang bisa ditambahkan". Bila callback menerima `paths` kosong dari Wails, tidak emit apa pun.

### 2.2 Go, `package main` (folder `app/`)

Didefinisikan di `app/bridge.go` (foundation):

```go
const (
    EventMenuZoom                   = "menu:zoom"
    EventMenuOpenFolder             = "menu:open-folder"
    EventMenuNewWorkspaceFromFolder = "menu:new-workspace-from-folder"
    EventMenuToggleSidebarLeft      = "menu:toggle-sidebar-left"
    EventMenuToggleSidebarRight     = "menu:toggle-sidebar-right"
    EventFilesDropped               = "files:dropped"
)
func (b *Bridge) emit(name string, data ...any)                    // no-op bila ctx nil; aman dari goroutine mana pun
func registerStartupHook(fn func(ctx context.Context, b *Bridge))  // dipanggil dari init(); dijalankan berurutan di SetContext setelah ctx tersimpan
func (b *Bridge) OpenDirectoryDialog() (string, error)             // "" bila dibatalkan
func (b *Bridge) AppConfigGet() (appconfig.Config, error)
func (b *Bridge) AppConfigSet(patch map[string]any) (appconfig.Config, error)
```

Didefinisikan di `app/menu.go` (native-menu-impl): `func buildMenu(b *Bridge) *menu.Menu`.

Paket `internal/appconfig` (foundation):

```go
type Window struct{ X, Y, W, H int }   // json x,y,w,h
type Config struct {
    Zoom   int     `json:"zoom"`               // 50..300, bawaan 100
    Window *Window `json:"window,omitempty"`
}
func Default() Config
type Store struct{ /* mutex, path */ }
func Open(path string) *Store
func (s *Store) Get() (Config, error)
func (s *Store) Patch(patch map[string]any) (Config, error)
func DefaultPath() (string, error)     // <paths.DataDir()>/config.json
```

Aturan `Patch`: gabung dangkal; `window` digabung satu tingkat; kunci tak dikenal pada patch diabaikan; kunci tak dikenal yang sudah ada di file dipertahankan (baca ke `map[string]any`, gabung, tulis); `zoom` di-clamp 50..300 dan dibulatkan; tulis atomik (temp file satu folder + rename), izin 0600; mutex. File rusak/kosong/tidak ada pada `Get` menjadi `Default()`.

### 2.3 Frontend, `frontend/src/modules/appmenu/index.ts` (foundation)

```ts
export type MenuEventName =
  | 'menu:zoom' | 'menu:open-folder' | 'menu:new-workspace-from-folder'
  | 'menu:toggle-sidebar-left' | 'menu:toggle-sidebar-right' | 'files:dropped';
export interface MenuPayloads {
  'menu:zoom': { action: 'in' | 'out' | 'reset' };
  'files:dropped': { paths: string[] };
}
export function onMenuEvent<K extends MenuEventName>(name: K, fn: (p: MenuPayloads[K]) => void): () => void;
export function emitMenuEvent<K extends MenuEventName>(name: K, p?: MenuPayloads[K]): void;
export interface AppConfig { zoom: number; window?: { x: number; y: number; w: number; h: number } }
export function getAppConfig(): Promise<AppConfig>;           // luar Wails: {zoom: 100}
export function setAppConfig(patch: Partial<AppConfig>): Promise<AppConfig>;
export function pickDirectory(): Promise<string | null>;       // null bila batal / luar Wails
```

File modul: `index.ts`, `bus.ts`, `bridge.ts`, `types.ts`. Modul lain mengimpor dari `'../appmenu'` (arah impor hanya zoom/openfolder menuju appmenu, tidak sebaliknya). `appmenu` mendaftarkan satu `window.runtime.EventsOn` per nama event; tanpa `window.runtime` tidak error.

### 2.4 Frontend, ekstensi terminal (dibuat di Wiring, dipakai `zoom`)

`frontend/src/modules/terminal/font-scale.ts`:

```ts
export const BASE_FONT_SIZE = 13;
export function registerTerminalInstance(term: { options: { fontSize?: number } }, refit: () => void): () => void;
export function setTerminalFontScale(percent: number): void;   // simpan skala; fontSize = round(13*percent/100) ke semua instance; panggil refit masing-masing
export function currentTerminalFontSize(): number;
```

Diekspor ulang dari `modules/terminal/index.ts`: `setTerminalFontScale, currentTerminalFontSize, registerTerminalInstance`.

### 2.5 Pintasan

| Item | Pintasan |
| --- | --- |
| File > Open Folder... | Cmd+O |
| File > New Workspace from Folder... | Cmd+Shift+O |
| View > Zoom In / Zoom Out / Actual Size | Cmd+= / Cmd+- / Cmd+0 |
| View > Toggle Left / Right Sidebar | Cmd+B / Cmd+Option+B |
| Edit (standar), Window (standar) | dari `menu.EditMenu()`, `menu.WindowMenu()` |

Dilarang dipakai: Cmd+D, Cmd+Shift+D, Cmd+K.

### 2.6 Konvensi

- File konfigurasi: `<DataDir>/config.json`, contoh `{"zoom":110}`.
- Bahasa pesan UI/log: Indonesia; identifier kode: Inggris.
- Store v1 (impor dari `lib/stores`): `activeWorkspace`, `activeWorkspaceId`, `workspaces`, `loadWorkspaces()`, `selectWorkspace(id)`, `createWorkspace(name)`, `addProject(path, name?)`. Baca file sumbernya untuk signature tepat; jangan diedit.
- Warna latar jendela: samakan dengan `--bg` tema gelap di `frontend/src/styles/theme.css` (`#1b1b1f`, RGB 27,27,31).
- Tanpa polling dan tanpa `setInterval`. Tanpa dependency baru.

## 3. Daftar fitur

### menu-bridge-contract (Foundation)
- Scope: konstanta event, `emit`, `registerStartupHook` (dijalankan di `SetContext`), method bridge `OpenDirectoryDialog`/`AppConfigGet`/`AppConfigSet` dengan `appconfig.Store` malas (`sync.Once`); paket `internal/appconfig`; modul `appmenu` (bus, bridge wrapper, types, index).
- ownedPaths: `app/bridge.go`, `internal/appconfig/`, `frontend/src/modules/appmenu/`
- Plan: `00-foundation.md`.

### native-menu-impl
- Scope: `app/menu.go` dengan `buildMenu(b *Bridge) *menu.Menu`: App (`menu.AppMenu()`), File (Open Folder Cmd+O, New Workspace from Folder Cmd+Shift+O), Edit (`menu.EditMenu()`), View (Zoom In `keys.CmdOrCtrl("=")`, Zoom Out `keys.CmdOrCtrl("-")`, Actual Size `keys.CmdOrCtrl("0")`, pemisah, Toggle Left Sidebar Cmd+B, Toggle Right Sidebar `keys.Combo("b", keys.CmdOrCtrlKey, keys.OptionOrAltKey)`, opsional Enter Full Screen), Window (`menu.WindowMenu()`). Callback hanya `b.emit(...)`, tanpa dialog dan tanpa blocking; payload zoom `map[string]string{"action": ...}`. `app/main.go`: `Menu: buildMenu(bridge)`, `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}` (jangan set `DisableWebViewDrop`), `Title: "Shikigami"`, 1200x800, `MinWidth: 800`, `MinHeight: 500`, `BackgroundColour` gelap, pertahankan `Bind`, `OnShutdown`, `AssetServer`, `OnStartup` memanggil `bridge.SetContext(ctx)`. Verifikasi signature `pkg/menu`/`keys` dari `$GOMODCACHE`.
- ownedPaths: `app/menu.go`, `app/main.go`

### zoom
- Scope: `frontend/src/modules/zoom/` (`index.ts`, `zoom.css`, boleh `Zoom` helper lain di folder itu). Langganan `onMenuEvent('menu:zoom')`; `percent` bawaan 100, in +10, out -10, reset 100, clamp 50..300. `applyZoom(percent)`: set `--ui-scale` di `document.documentElement`, CSS `zoom: var(--ui-scale)` pada `#app`, kontainer terminal (kelas `.term-host` di `TerminalTab.svelte`) diberi `zoom: calc(1 / var(--ui-scale))` agar tidak terskala ganda, panggil `setTerminalFontScale(percent)`. Simpan lewat `setAppConfig({zoom})` dengan debounce ~300 ms (`setTimeout`, bukan `setInterval`); saat dimuat, `getAppConfig()` lalu `applyZoom`. Ekspor `getZoomPercent(): number` dan store baca-saja `zoomPercent` (`svelte/store` `readable`/`derived`). Tidak ada UI.
- Ketergantungan lintas fitur: `setTerminalFontScale` berasal dari `modules/terminal` yang baru diperluas di Wiring. Agar build `zoom` lolos sebelum Wiring, impor secara dinamis dan toleran: `import('../terminal').then(m => m.setTerminalFontScale?.(p)).catch(() => {})` dengan cast tipe longgar (`as any`) sehingga TypeScript tidak gagal karena ekspor belum ada. Jangan menyunting `modules/terminal/`.
- ownedPaths: `frontend/src/modules/zoom/`

### open-folder
- Scope: `app/dialog.go` (`init()` memanggil `registerStartupHook` yang memasang `runtime.OnFileDrop(ctx, ...)`; saring ke direktori lewat `os.Stat().IsDir()`, `filepath.Clean`, buang duplikat; emit `EventFilesDropped` dengan `map[string]any{"paths": dirs}` sesuai keputusan 2.1; jangan duplikasi dialog yang sudah ada di `bridge.go`, file boleh hanya berisi drop handler). `frontend/src/modules/openfolder/`: `index.ts` (mendengarkan `menu:open-folder`, `menu:new-workspace-from-folder`, `files:dropped` via `onMenuEvent`; memakai `pickDirectory` dan store v1; `basename` menangani trailing slash dan root; folder duplikat menjadi pesan; kegagalan satu path tidak menghentikan yang lain, ringkasan di akhir; tanpa workspace aktif, buat workspace dari folder pertama), `Toast.svelte` + store pesan kecil (didaftarkan `registerPanel({id: 'openfolder-toast', area: 'statusbar', component: Toast, order: 90})`), `drop.css` (`body { --wails-drop-target: drop }` dan sorot `.wails-drop-target-active` dengan garis putus beraksen), diimpor dari `index.ts`. Tidak mengedit `modules/workspace/`, `lib/stores/`, `lib/ipc.ts`; baca `modules/workspace/errors.ts` untuk gaya `errMessage`.
- ownedPaths: `app/dialog.go`, `frontend/src/modules/openfolder/`

## 4. Foundation

Ada: `menu-bridge-contract`. Alasan: tiga fitur lain mengimpor nama dari `app/bridge.go` (konstanta event, `emit`, `registerStartupHook`) dan `modules/appmenu` (`onMenuEvent`, `pickDirectory`, dst.). Plan langkah ada di `00-foundation.md`. `hasFoundation = true`.

## 5. Wiring checklist

Dikerjakan satu agent setelah foundation dan ketiga fitur tergabung. Satu-satunya file yang boleh disentuh di luar ownedPaths adalah tiga file terminal di bawah.

1. Modul `appmenu`, `zoom`, `openfolder` punya `index.ts`, sehingga dimuat otomatis oleh glob `main.ts`; JANGAN edit `main.ts`. Pastikan tidak ada impor melingkar (zoom dan openfolder mengimpor dari `../appmenu`, bukan sebaliknya).
2. Buat `frontend/src/modules/terminal/font-scale.ts` sesuai 2.4 (registry `Set` instance, skala saat ini bawaan 100).
3. `frontend/src/modules/terminal/index.ts`: tambah satu baris `export { setTerminalFontScale, currentTerminalFontSize, registerTerminalInstance } from './font-scale';` tanpa mengubah registrasi tipe tab `session`.
4. `frontend/src/modules/terminal/TerminalTab.svelte`: pada `new Terminal({...})` ganti `fontSize: 13` dengan `currentTerminalFontSize()`; setelah terminal dan fit addon dibuat panggil `registerTerminalInstance(term, () => fit.fit())`, simpan fungsi unregister, panggil di cleanup `onDestroy`/cleanup yang sudah ada. Tidak ada perubahan lain (attach, input, resize, `session.exit` tidak disentuh). Pastikan `fit.fit()` memicu jalur resize yang ada (`session.resize`); bila refit hanya lewat `ResizeObserver`, pastikan tetap terpanggil setelah perubahan font. Cek kelas kontainer (`.term-host`) cocok dengan selektor di `zoom.css`; bila beda, sesuaikan `zoom.css` (itu file milik `zoom`, boleh disunting oleh Wiring untuk penyelarasan ini saja).
5. Pasang di Go: pastikan `app/main.go` memakai `buildMenu(bridge)` dan `EnableFileDrop`, `OnStartup` memanggil `bridge.SetContext` (menjalankan hook), dan `app/dialog.go` terdaftar via `init()` + `registerStartupHook`.
6. Pastikan Toast `openfolder-toast` terdaftar di area `statusbar` (statusbar v1 merendernya).
7. Pastikan `internal/provider` tidak berubah (flag izin agent). Pastikan `lib/ipc.ts`, `lib/registry.ts`, `main.ts`, `package.json`, `modules/workspace/`, `lib/stores/` tidak berubah.
8. Jalankan seluruh build guard (bagian 6). Verifikasi manual di aplikasi nyata dari `build/bin/shikigami`: menu dan pintasan tampil; Cmd+O menambah project; New Workspace from Folder; drop folder Finder; Cmd+= / Cmd+- / Cmd+0 saat fokus di terminal; tutup-buka, zoom tetap; Cmd+C/Cmd+V di terminal; Browse v1 muncul. Toggle sidebar hanya terbukti sampai level event.

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- `go build ./...`
- `cd frontend && npm run build`
- `make build` (rantai penuh) lulus dan aplikasi bisa dijalankan. Agent fitur paralel cukup menjalankan `go build ./...` dan/atau `cd frontend && npm run build` sesuai sisi yang disentuh; `make build` dijalankan agen Wiring.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya. Dilarang menyentuh path fitur lain, file wiring (`modules/terminal/*`), `main.ts`, `lib/ipc.ts`, `lib/registry.ts`, `lib/stores/`, `modules/workspace/`, `package.json`, `go.mod`, `Makefile`, `internal/provider`, dan seluruh `cmd/` serta `internal/*` selain `internal/appconfig/` (milik foundation).
- Fitur paralel hanya boleh memakai nama ekspor yang tertulis di bagian 2; bila nama itu belum ada karena fitur lain, ikuti penanganan toleran di bagian 3. Jangan mengubah kontrak sepihak.
- Dilarang menambah dependency (Go maupun npm).
- Tanpa unit test; hapus `console.log` sementara sebelum selesai.
- Jangan commit/merge sendiri kecuali diminta oleh alur `implement-feature`.
