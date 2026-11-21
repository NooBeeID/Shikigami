# Plan: Menu native macOS dengan accelerator

## Metadata

- Tiket: `shikigami-v2-ui-ticket-02-native-menu` (`docs/tickets/shikigami-v2-ui-ticket-02-native-menu.md`)
- Fitur: `native-menu-impl`
- ownedPaths: `app/menu.go`, `app/main.go`
- Depends on foundation: ya (`menu-bridge-contract`, plan `00-foundation.md`, sudah tergabung sebelum plan ini dikerjakan). Plan ini memakai `Bridge.emit` dan konstanta `Event*` dari `app/bridge.go`.

Kontrak lengkap ada di `overview.md` (bagian 2.1, 2.2, 2.5); dokumen ini hanya langkah kerja.

## Tujuan

Membuat menu native macOS (App, File, Edit, View, Window) dengan accelerator, dan memasangnya di `wails.Run` bersama pengaturan jendela (judul, ukuran, warna latar) dan aktivasi file-drop. Menu hanya meng-emit event Wails; frontend yang memutuskan aksi.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `app/menu.go` (baru)
- `app/main.go` (ubah)

Jangan sentuh `app/bridge.go`, `app/dialog.go`, `app/daemon.go`, `internal/*`, `frontend/*`, `go.mod`, `Makefile`, atau file fitur lain. Tanpa dependency baru.

## Kontrak

Dipakai (dari foundation, `app/bridge.go`, `package main`):

- Konstanta: `EventMenuZoom`, `EventMenuOpenFolder`, `EventMenuNewWorkspaceFromFolder`, `EventMenuToggleSidebarLeft`, `EventMenuToggleSidebarRight`.
- `func (b *Bridge) emit(name string, data ...any)` (no-op bila ctx nil).
- `func (b *Bridge) SetContext(ctx context.Context)` (menjalankan hook startup).
- `func NewBridge() *Bridge`.

Disediakan:

- `func buildMenu(b *Bridge) *menu.Menu` di `app/menu.go`.

Payload zoom: `map[string]string{"action": "in" | "out" | "reset"}`. Event lain tanpa payload.

Pintasan (overview 2.5): Open Folder Cmd+O; New Workspace from Folder Cmd+Shift+O; Zoom In/Out/Actual Size Cmd+= / Cmd+- / Cmd+0; Toggle Left Sidebar Cmd+B; Toggle Right Sidebar Cmd+Option+B. DILARANG: Cmd+D, Cmd+Shift+D, Cmd+K.

Signature Wails v2.17.1 yang sudah diverifikasi dari `$GOMODCACHE`:

- Paket `github.com/wailsapp/wails/v2/pkg/menu`: `menu.NewMenu() *Menu`, `(*Menu).Append(*MenuItem)`, `menu.NewMenuFromItems(first *MenuItem, rest ...*MenuItem) *Menu`, `menu.Text(label string, accelerator *keys.Accelerator, click Callback) *MenuItem`, `menu.Separator() *MenuItem`, `menu.SubMenu(label string, m *Menu) *MenuItem`, `menu.AppMenu()`, `menu.EditMenu()`, `menu.WindowMenu()` (semua `*MenuItem`, berbasis role), `type Callback func(*CallbackData)`.
- Paket `.../pkg/menu/keys`: `keys.CmdOrCtrl(key string) *Accelerator`, `keys.Shift(key string)`, `keys.OptionOrAlt(key string)`, `keys.Combo(key string, m1, m2 Modifier, rest ...Modifier) *Accelerator`, modifier `keys.CmdOrCtrlKey`, `keys.OptionOrAltKey`, `keys.ShiftKey`.
- `options.App` punya `Menu *menu.Menu`, `MinWidth`, `MinHeight`, `BackgroundColour *options.RGBA`, `DragAndDrop *options.DragAndDrop{EnableFileDrop, DisableWebViewDrop, ...}`; `options.NewRGBA(r, g, b, a uint8) *RGBA`.
- `runtime.WindowToggleFullscreen` TIDAK ada di v2.17.1 (hanya `WindowFullscreen`/`WindowUnfullscreen`). Karena itu item "Enter Full Screen" opsional DILEWATI; jangan dibuat.

## Langkah

### Langkah 1: `app/menu.go`

1. `package main`; impor `github.com/wailsapp/wails/v2/pkg/menu` dan `github.com/wailsapp/wails/v2/pkg/menu/keys`.
2. Definisikan `func buildMenu(b *Bridge) *menu.Menu`:
   - `m := menu.NewMenu()`.
   - `m.Append(menu.AppMenu())`.
   - File: `file := menu.NewMenu()`; item `menu.Text("Open Folder...", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) { b.emit(EventMenuOpenFolder) })`; item `menu.Text("New Workspace from Folder...", keys.Combo("o", keys.CmdOrCtrlKey, keys.ShiftKey), func(...) { b.emit(EventMenuNewWorkspaceFromFolder) })`; `m.Append(menu.SubMenu("File", file))`.
   - `m.Append(menu.EditMenu())`.
   - View: `view := menu.NewMenu()`; item "Zoom In" `keys.CmdOrCtrl("=")` emit `EventMenuZoom` dengan `map[string]string{"action": "in"}`; "Zoom Out" `keys.CmdOrCtrl("-")` dengan `"out"`; "Actual Size" `keys.CmdOrCtrl("0")` dengan `"reset"`; `view.Append(menu.Separator())`; "Toggle Left Sidebar" `keys.CmdOrCtrl("b")` emit `EventMenuToggleSidebarLeft`; "Toggle Right Sidebar" `keys.Combo("b", keys.CmdOrCtrlKey, keys.OptionOrAltKey)` emit `EventMenuToggleSidebarRight`; `m.Append(menu.SubMenu("View", view))`.
   - `m.Append(menu.WindowMenu())`; `return m`.
3. Callback hanya memanggil `b.emit(...)`: tanpa dialog, tanpa blocking, tanpa lock lain. Boleh helper kecil `emitZoom := func(action string) menu.Callback`.
4. Cmd++ (Cmd+Shift+=): jangan tambah item duplikat. Tambahkan hanya bila terbukti perlu saat uji manual (Wiring/verifikasi manual); di plan ini tidak dibuat.
5. Pastikan tidak ada accelerator `d`, `k` dengan Cmd pada menu mana pun yang dibuat file ini.

### Langkah 2: `app/main.go`

Sunting `wails.Run(&options.App{...})` (struktur saat ini sudah punya `Title: "Shikigami"`, `Width: 1200`, `Height: 800`, `AssetServer`, `OnStartup`, `OnShutdown`, `Bind`). Pertahankan semuanya, lalu tambahkan:

- `MinWidth: 800`, `MinHeight: 500`.
- `BackgroundColour: options.NewRGBA(27, 27, 31, 255)` (samakan dengan `--bg` tema gelap `#1b1b1f` di `frontend/src/styles/theme.css`).
- `Menu: buildMenu(bridge)`.
- `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}` (jangan set `DisableWebViewDrop`).

Ketentuan: `bridge := NewBridge()` dibuat sebelum `wails.Run` (sudah begitu); `OnStartup` tetap memanggil `bridge.SetContext(ctx)`; `OnShutdown` tetap `bridge.Close()`; `Bind: []any{bridge}` tidak berubah. Tambahkan impor `github.com/wailsapp/wails/v2/pkg/options` hanya jika belum ada (sudah ada). Tidak ada impor menu di `main.go` (cukup `buildMenu`).

### Langkah 3: Verifikasi

- `gofmt -l app/` kosong.
- `go build ./...` lulus (butuh foundation tergabung agar `emit` dan konstanta `Event*` ada; bila foundation belum ada di checkout, hentikan dan laporkan, jangan mendefinisikannya sendiri).
- `go vet ./app/...` bersih bila tersedia.
- Pastikan hanya `app/menu.go` dan `app/main.go` yang berubah.

## Perubahan

- `app/menu.go`: baru, berisi `buildMenu`.
- `app/main.go`: diubah, menambah `Menu`, `DragAndDrop`, `MinWidth`, `MinHeight`, `BackgroundColour`.

## Wiring

Fitur ini mengekspos `buildMenu(bridge)` dan sudah dipasang di `app/main.go` (milik fitur ini, jadi bukan penyambungan lintas fitur). Yang disambungkan nanti oleh langkah Wiring (jangan dikerjakan di sini): `app/dialog.go` mendaftarkan handler drop lewat `registerStartupHook` (berjalan dari `bridge.SetContext` di `OnStartup`), dan frontend (`appmenu`, `zoom`, `openfolder`) mendengarkan event menu. Verifikasi manual di aplikasi nyata (menu tampil, pintasan, Cmd+= saat fokus di terminal, Cmd+C/V di terminal, Cmd++ bila perlu) dilakukan agen Wiring lewat `make build`.

## Selesai bila

- `app/menu.go` mengekspos `buildMenu(b *Bridge) *menu.Menu` dengan menu App, File, Edit, View, Window sesuai pintasan di Kontrak, dan setiap callback hanya `b.emit(...)` dengan nama event dan payload sesuai Kontrak.
- Tidak ada Cmd+D, Cmd+Shift+D, Cmd+K, dan tidak ada item full screen.
- `app/main.go` memasang `Menu`, `EnableFileDrop: true` (tanpa `DisableWebViewDrop`), `Title: "Shikigami"`, 1200x800, `MinWidth: 800`, `MinHeight: 500`, `BackgroundColour` RGB 27,27,31; `Bind`, `OnStartup`, `OnShutdown`, `AssetServer` tetap.
- Guard: `go build ./...` sukses. Tanpa unit test.
- Tidak ada file di luar `app/menu.go` dan `app/main.go` yang berubah; tidak commit/merge sendiri.
