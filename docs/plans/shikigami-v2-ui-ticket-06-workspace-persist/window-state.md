# Plan: Ingat posisi dan ukuran jendela

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-06-workspace-persist.md` (fitur `window-state`)
- Overview: `docs/plans/shikigami-v2-ui-ticket-06-workspace-persist/overview.md` (bagian 2.5 dan catatan khusus `window-state` di bagian 3)
- Fitur: `window-state`: Ingat posisi dan ukuran jendela
- ownedPaths: `app/window.go`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; hanya bergantung pada kode ticket-02 yang sudah ada: `app/bridge.go`, `internal/appconfig`)

## Tujuan

Posisi dan ukuran jendela utama disimpan ke appconfig (`window:{x,y,w,h}`) dan dipulihkan saat aplikasi dibuka lagi. Pemulihan divalidasi terhadap layar yang terpasang agar jendela tidak hilang di luar layar. Penyimpanan dipicu dari frontend (debounce 500 ms ada di frontend) dan sekali lagi saat jendela ditutup. Tanpa timer polling, tidak menyentuh `zoom`.

## Path yang dimiliki

HANYA `app/window.go` (file baru, `package main`). Jangan menyentuh `app/main.go`, `app/bridge.go`, `internal/appconfig/`, file wiring, atau path fitur lain (`internal/workspace/`, `frontend/...`). Baris `OnBeforeClose` di `app/main.go` dikerjakan agen Wiring.

## Kontrak

Fakta kode yang dipakai (sudah ada, jangan diubah):
- `app/bridge.go`: `registerStartupHook(fn func(ctx context.Context, b *Bridge))` (hook dijalankan di `Bridge.SetContext`, hanya boleh didaftarkan dari `init()`); `Bridge.ctx` dilindungi `ctxMu sync.RWMutex`; `(b *Bridge) AppConfigGet() (appconfig.Config, error)`; `(b *Bridge) AppConfigSet(patch map[string]any) (appconfig.Config, error)` (memanggil `Store.Patch`).
- `internal/appconfig`: `Window{X,Y,W,H int}`, `Config{Zoom int; Window *Window}`. `Patch(map{"window": map[string]any{"x":..,"y":..,"w":..,"h":..}})` menggabung per kunci window dan tidak menyentuh `zoom`. Paket ini tidak diubah.
- Wails v2 (`github.com/wailsapp/wails/v2/pkg/runtime`): `WindowGetPosition(ctx) (x, y int)`, `WindowGetSize(ctx) (w, h int)`, `WindowSetSize(ctx, w, h)`, `WindowSetPosition(ctx, x, y)`, `WindowCenter(ctx)`, `WindowIsMaximised(ctx) bool`, `WindowIsFullscreen(ctx) bool`, `ScreenGetAll(ctx) ([]runtime.Screen, error)`. `runtime.Screen` punya `Size.Width`/`Size.Height` (dan `Width`/`Height` lama); TIDAK ada origin layar (x,y). Cek bentuk persisnya di modul Wails (`go doc github.com/wailsapp/wails/v2/pkg/runtime Screen`) sebelum menulis.

Yang disediakan (sesuai overview bagian 2.5; nama dan signature harus persis):

```go
func init() { registerStartupHook(func(ctx context.Context, b *Bridge) { restoreWindow(ctx, b) }) }

func restoreWindow(ctx context.Context, b *Bridge)
func saveWindow(ctx context.Context, b *Bridge)
func (b *Bridge) WindowSaveState() error                        // ter-bind ke frontend: window.go.main.Bridge.WindowSaveState()
func onBeforeCloseWindow(ctx context.Context, b *Bridge) bool   // simpan lalu return false
```

Konsumen: `autosave-restore` memanggil `window.go.main.Bridge.WindowSaveState()` (dibungkus try/catch di frontend, kegagalan diabaikan). Wiring memasang `OnBeforeClose: func(ctx context.Context) bool { return onBeforeCloseWindow(ctx, bridge) }` di `app/main.go`.

## Langkah

1. Buat `app/window.go`, `package main`. Import: `context`, `errors`, `github.com/wailsapp/wails/v2/pkg/runtime`, dan `github.com/NooBeeID/shikigami/internal/appconfig` bila perlu untuk tipe `appconfig.Window`. Hindari nama top-level yang bisa bentrok dengan file lain di `package main` (awali konstanta dengan `win`, mis. `winDefaultW`).

2. Konstanta:
   - `winDefaultW = 1200`, `winDefaultH = 800` (sama dengan `main.go`), `winMinW = 800`, `winMinH = 500`, `winMinVisible = 100` (piksel minimum irisan agar jendela dianggap terlihat).

3. `init()` seperti di Kontrak (satu-satunya pendaftaran; jangan membuat goroutine/timer).

4. Helper murni `func windowVisible(w appconfig.Window, screens []runtime.Screen) bool`:
   - Wails v2 tidak memberi origin layar, jadi gunakan area valid konservatif: `totalW` = jumlah lebar semua layar, `maxH` = tinggi terbesar. Area valid = persegi `x ∈ [-totalW, totalW]`, `y ∈ [-maxH, maxH]` untuk titik asal layar, dan layar-layar yang mungkin berada di kiri/atas primary dianggap terwakili oleh rentang negatif itu. Implementasi sederhana yang dapat dipertanggungjawabkan: bangun satu persegi kandidat `[-(totalW - primaryW), primaryW + (totalW - primaryW)] x [-(maxH - primaryH), maxH]`, lalu jendela valid bila irisannya dengan persegi itu minimal `winMinVisible` x `winMinVisible`. Pada layar tunggal ini sama dengan `[0,W] x [0,H]`, jadi posisi sisa monitor yang dicabut (mis. x=2400 pada layar 1440 lebar, atau x negatif) ditolak. Primary = layar dengan `IsPrimary`, fallback layar pertama.
   - Daftar layar kosong atau `ScreenGetAll` error: kembalikan `true` (jangan memblokir restore tanpa data), tetapi tetap terapkan klem ukuran di langkah 5.
   - Tulis komentar bahwa ini aproksimasi karena API tidak menyediakan origin per layar.

5. `restoreWindow(ctx, b)`:
   - `cfg, err := b.AppConfigGet()`; bila `err != nil` atau `cfg.Window == nil`: panggil `applyDefaultWindow(ctx)` lalu return (instalasi baru tidak error).
   - Klem: `w := max(cfg.Window.W, winMinW)`, `h := max(cfg.Window.H, winMinH)`. Bila `cfg.Window.W/H <= 0` perlakukan sebagai tidak valid -> default.
   - `screens, _ := runtime.ScreenGetAll(ctx)`; bila `!windowVisible(...)` -> `applyDefaultWindow(ctx)`; return.
   - Valid: `runtime.WindowSetSize(ctx, w, h)` lalu `runtime.WindowSetPosition(ctx, cfg.Window.X, cfg.Window.Y)`.
   - `applyDefaultWindow(ctx)`: `runtime.WindowSetSize(ctx, winDefaultW, winDefaultH)` lalu `runtime.WindowCenter(ctx)`.
   - Tidak mengembalikan error; semua kegagalan jatuh ke default (tanpa panic). Jangan menyentuh field `zoom`.

6. `saveWindow(ctx, b)`:
   - `if ctx == nil { return }`.
   - Lewati bila `runtime.WindowIsMaximised(ctx) || runtime.WindowIsFullscreen(ctx)` (ukuran normal tersimpan tidak ditimpa). Lewati juga bila minimised bila API tersedia di versi Wails yang dipakai (`WindowIsMinimised`), karena ukuran bisa nol.
   - `x, y := runtime.WindowGetPosition(ctx)`; `w, h := runtime.WindowGetSize(ctx)`.
   - Klem `w >= winMinW`, `h >= winMinH`. Bila `w <= 0 || h <= 0` setelah baca, lewati.
   - `_, _ = b.AppConfigSet(map[string]any{"window": map[string]any{"x": x, "y": y, "w": w, "h": h}})`. Patch HANYA berisi kunci `window`. Error diabaikan (best effort); boleh dicatat dengan `log.Printf` bila repo memakai pola itu.

7. Metode ter-bind:
   ```go
   func (b *Bridge) WindowSaveState() error {
       b.ctxMu.RLock()
       ctx := b.ctx
       b.ctxMu.RUnlock()
       if ctx == nil {
           return errors.New("aplikasi belum siap")
       }
       saveWindow(ctx, b)
       return nil
   }
   ```
   Tidak ada debounce/timer di Go; frontend yang men-debounce 500 ms.

8. `onBeforeCloseWindow(ctx, b) bool`: `saveWindow(ctx, b); return false` (false = jangan cegah penutupan).

9. Jalankan `gofmt -l app/` dan `go build ./...` (pada mesin dengan Wails desktop, bila perlu `go vet -tags desktop,production ./app` dengan `CGO_LDFLAGS="-framework UniformTypeIdentifiers"` sesuai `make build`). Perbaiki error kompilasi hanya di `app/window.go`. Bila ada nama bentrok dengan file lain di `package main`, ganti nama di `window.go` saja.

## Perubahan

- Baru: `app/window.go`
- Tidak ada file lain yang diubah.

## Wiring

Diekspos untuk langkah Wiring (jangan disambungkan sendiri):
- Restore saat start: sudah terpasang otomatis lewat `init()` + `registerStartupHook` di `window.go` (tidak perlu edit `main.go`/`bridge.go`). Wiring cukup memverifikasi tidak bentrok dengan hook lain.
- Simpan saat tutup: Wiring menambah satu baris di `app/main.go` pada `options.App`: `OnBeforeClose: func(ctx context.Context) bool { return onBeforeCloseWindow(ctx, bridge) },`.
- Simpan saat berubah: metode `(*Bridge).WindowSaveState() error` (ter-bind Wails sebagai `window.go.main.Bridge.WindowSaveState`) dipanggil `autosave-restore` dari frontend dengan debounce 500 ms; tidak perlu wiring Go tambahan.

## Selesai bila

- `app/window.go` ada dengan keempat fungsi/metode bertanda tangan persis seperti Kontrak, dan `init()` mendaftarkan hook restore.
- Instalasi tanpa field `window` memakai 1200x800 di tengah tanpa error; posisi di luar semua layar jatuh ke default di tengah; ukuran tersimpan tidak pernah di bawah 800x500; maximised/fullscreen tidak menimpa ukuran tersimpan.
- Patch appconfig hanya berisi kunci `window` (`zoom` tidak tersentuh); tidak ada goroutine atau timer polling.
- Hanya `app/window.go` yang dibuat/diubah.
- Guard: `go build ./...` sukses (dan, bila memungkinkan, build app dengan tag `desktop,production` sesuai `make build`). Tanpa unit test.

## Asumsi

- Wails v2 tidak menyediakan origin (x,y) per layar sehingga validasi irisan layar berupa aproksimasi konservatif (langkah 4). Bila versi Wails yang dipakai ternyata menyediakan origin, gunakan irisan persis per layar.
