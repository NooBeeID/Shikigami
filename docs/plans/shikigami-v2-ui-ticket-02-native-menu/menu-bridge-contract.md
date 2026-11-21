# Plan: Foundation: kontrak event menu dan bridge dialog

## Metadata

- Tiket: `shikigami-v2-ui-ticket-02-native-menu` (`docs/tickets/shikigami-v2-ui-ticket-02-native-menu.md`)
- Fitur: `menu-bridge-contract` (Foundation, digabung sebelum `native-menu-impl`, `zoom`, `open-folder`)
- ownedPaths: `app/bridge.go`, `internal/appconfig/`, `frontend/src/modules/appmenu/`
- Depends on foundation: tidak (fitur ini adalah foundation-nya)
- Rujukan kontrak lengkap: `docs/plans/shikigami-v2-ui-ticket-02-native-menu/overview.md` bagian 2 (jangan diubah)

## Tujuan

Menetapkan semua yang dibagi antar fitur epic ini supaya tiga fitur paralel lain bisa memakainya dan lolos build sendiri:

- Sisi Go: konstanta nama event, `Bridge.emit`, `registerStartupHook` (dijalankan di `SetContext`), method bridge `OpenDirectoryDialog`, `AppConfigGet`, `AppConfigSet`.
- Paket `internal/appconfig` untuk baca/tulis `<DataDir>/config.json`.
- Modul frontend `appmenu`: event bus, pembungkus bridge, tipe, dan `index.ts` yang meneruskan event Wails ke bus.

Tanpa unit test; guard hanya build.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `app/bridge.go` (sunting)
- `internal/appconfig/` (baru)
- `frontend/src/modules/appmenu/` (baru)

Dilarang menyentuh: `app/menu.go`, `app/main.go`, `app/dialog.go`, `frontend/src/modules/zoom/`, `frontend/src/modules/openfolder/`, `frontend/src/modules/terminal/*`, `frontend/src/main.ts`, `frontend/src/lib/ipc.ts`, `frontend/src/lib/registry.ts`, `frontend/src/lib/stores/`, `frontend/src/modules/workspace/`, `frontend/src/env.d.ts`, `frontend/package.json`, `go.mod`, `Makefile`, `internal/provider`, `cmd/`, dan `internal/*` selain `internal/appconfig/`. Jangan menambah dependency. Jangan commit/merge sendiri.

Catatan: `app/main.go` v1 memanggil `bridge.SetContext(ctx)` lewat `OnStartup`; tidak perlu diubah karena hook dijalankan di dalam `SetContext`.

## Kontrak

Semua nama final, rujuk overview bagian 2.1 sampai 2.3.

Disediakan (Go, `package main`, `app/bridge.go`):

```go
const (
    EventMenuZoom                   = "menu:zoom"
    EventMenuOpenFolder             = "menu:open-folder"
    EventMenuNewWorkspaceFromFolder = "menu:new-workspace-from-folder"
    EventMenuToggleSidebarLeft      = "menu:toggle-sidebar-left"
    EventMenuToggleSidebarRight     = "menu:toggle-sidebar-right"
    EventFilesDropped               = "files:dropped"
)
func (b *Bridge) emit(name string, data ...any)
func registerStartupHook(fn func(ctx context.Context, b *Bridge))
func (b *Bridge) OpenDirectoryDialog() (string, error)
func (b *Bridge) AppConfigGet() (appconfig.Config, error)
func (b *Bridge) AppConfigSet(patch map[string]any) (appconfig.Config, error)
```

Disediakan (`internal/appconfig`): `Window`, `Config`, `Default()`, `Store`, `Open(path)`, `(*Store).Get()`, `(*Store).Patch(map[string]any)`, `DefaultPath()`.

Disediakan (frontend, `frontend/src/modules/appmenu/index.ts`): tipe `MenuEventName`, `MenuPayloads`, `AppConfig`; fungsi `onMenuEvent`, `emitMenuEvent`, `getAppConfig`, `setAppConfig`, `pickDirectory`.

Dipakai (sudah ada): `paths.DataDir() (string, error)` dari `internal/paths`; `runtime.EventsEmit`, `runtime.OpenDirectoryDialog`, `runtime.OpenDialogOptions` dari Wails v2.17.1; `window.runtime.EventsOn` dan `window.go.main.Bridge.*` (global Wails, diakses dengan cast `(window as any)`).

Tidak ada pesan/event baru selain tabel overview 2.1.

## Langkah

### 1. `internal/appconfig/appconfig.go`

Package `appconfig`; hanya stdlib dan `github.com/NooBeeID/shikigami/internal/paths`. Baca dulu `internal/paths` untuk memastikan `DataDir() (string, error)`.

1. Tipe:
   ```go
   type Window struct {
       X int `json:"x"`; Y int `json:"y"`; W int `json:"w"`; H int `json:"h"`
   }
   type Config struct {
       Zoom   int     `json:"zoom"`
       Window *Window `json:"window,omitempty"`
   }
   func Default() Config { return Config{Zoom: 100} }
   ```
   Konstanta `minZoom = 50`, `maxZoom = 300`, `defaultZoom = 100`; helper `clampZoom(n int) int`.
2. `type Store struct { mu sync.Mutex; path string }`; `func Open(path string) *Store` (tidak membaca disk); `func DefaultPath() (string, error)`: panggil `paths.DataDir()`, teruskan error dibungkus `fmt.Errorf("gagal menentukan DataDir: %w", err)`, hasil `filepath.Join(dir, "config.json")`.
3. `func (s *Store) readRaw() (map[string]any, error)` (dipanggil dengan mutex terkunci): `os.ReadFile`; `errors.Is(err, fs.ErrNotExist)` atau isi kosong (setelah `bytes.TrimSpace`) menjadi `map[string]any{}`, nil; error I/O lain dikembalikan dibungkus. `json.Unmarshal` ke map; bila gagal atau hasil nil, `log.Printf("appconfig: config.json rusak, memakai bawaan: %v", err)` dan kembalikan map kosong (bukan error). Gunakan decoder dengan `UseNumber()` agar angka kunci tak dikenal tidak berubah bentuk saat ditulis ulang.
4. `func configFromRaw(raw map[string]any) Config`: mulai dari `Default()`; marshal map ke JSON lalu `json.Unmarshal` ke `Config` (error diabaikan, tetap bawaan); normalisasi: `Zoom` di luar 50..300 diklamp, `Zoom == 0` menjadi 100.
5. `func (s *Store) Get() (Config, error)`: lock, `readRaw`, kembalikan `configFromRaw`.
6. `func (s *Store) Patch(patch map[string]any) (Config, error)`: lock; `raw := readRaw()`; loop kunci patch, hanya dua yang dikenal:
   - `zoom`: terima `float64`, `int`, `int64`, `json.Number`; bulatkan (`math.Round`), clamp 50..300, simpan ke `raw["zoom"]` sebagai `int`. Tipe lain diabaikan.
   - `window`: harus `map[string]any`; ambil map `window` yang ada di `raw` (bila bukan map, mulai dari map kosong); untuk kunci `x,y,w,h` saja, terima tipe numerik yang sama, bulatkan ke `int`, tulis ke map window; gabung satu tingkat. Simpan balik ke `raw["window"]`.
   - kunci lain: diabaikan. Kunci tak dikenal yang sudah ada di `raw` TIDAK dihapus.
   Lalu `writeAtomic(raw)`, kembalikan `configFromRaw(raw)`.
7. `func (s *Store) writeAtomic(raw map[string]any) error`: `os.MkdirAll(dir, 0o700)`; `json.MarshalIndent` (atau `Marshal`); `os.CreateTemp(dir, ".config-*.tmp")`; `Chmod(0o600)`; `Write`; `Close`; `os.Rename(tmp, s.path)`; pada error di tengah jalan, `os.Remove(tmp)`. Semua error dibungkus pesan Indonesia (`"gagal menulis config: %w"`).

### 2. `app/bridge.go`

Sunting file yang ada; JANGAN mengubah `Call`, `Close`, `connect`, `drop`.

1. Tambah impor `github.com/NooBeeID/shikigami/internal/appconfig`.
2. Tambah blok konstanta event (persis nama di Kontrak).
3. Hook startup:
   ```go
   var startupHooks []func(ctx context.Context, b *Bridge)
   func registerStartupHook(fn func(ctx context.Context, b *Bridge)) { startupHooks = append(startupHooks, fn) }
   ```
   Hanya dipanggil dari `init()`, tanpa mutex. Ubah `SetContext`: `ctxMu.Lock(); b.ctx = ctx; ctxMu.Unlock()`, lalu (kunci sudah dilepas) `for _, fn := range startupHooks { fn(ctx, b) }`.
4. `func (b *Bridge) emit(name string, data ...any)`: `ctxMu.RLock()`, salin ctx, `RUnlock()`; bila `ctx == nil` return; `runtime.EventsEmit(ctx, name, data...)`. Tidak mengambil `b.mu`. Refaktor `relay` agar memanggil `b.emit("daemon:event", map[string]any{"topic": topic, "payload": payload})` (perilaku identik).
5. Tambah field pada struct `Bridge`: `cfgOnce sync.Once`, `cfg *appconfig.Store`, `cfgErr error`. Helper:
   ```go
   func (b *Bridge) configStore() (*appconfig.Store, error) {
       b.cfgOnce.Do(func() {
           p, err := appconfig.DefaultPath()
           if err != nil { b.cfgErr = fmt.Errorf("gagal menentukan lokasi config: %w", err); return }
           b.cfg = appconfig.Open(p)
       })
       return b.cfg, b.cfgErr
   }
   ```
6. Method bridge (tambah tipe pembantu `currentCtx()` bila perlu, memakai `ctxMu.RLock`):
   - `OpenDirectoryDialog() (string, error)`: ctx nil menjadi `errors.New("aplikasi belum siap")`; `runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: "Pilih folder project", CanCreateDirectories: true})`; error dibungkus `"gagal membuka dialog folder: %w"`; hasil kosong (batal) menjadi `"", nil`.
   - `AppConfigGet() (appconfig.Config, error)`: `configStore()` lalu `Get()`.
   - `AppConfigSet(patch map[string]any) (appconfig.Config, error)`: `configStore()` lalu `Patch(patch)`.
   Verifikasi nama field `OpenDialogOptions` (`Title`, `CanCreateDirectories`) dari `$GOMODCACHE` Wails v2.17.1 bila build gagal.

### 3. `frontend/src/modules/appmenu/`

Empat file. Tanpa impor modul lain (hanya antar-file di folder ini); tanpa `registerPanel`; tanpa menyunting `env.d.ts` (pakai `(window as any)`).

1. `types.ts`: ekspor `MenuEventName`, `MenuPayloads`, `AppConfig` persis seperti overview 2.3. Tipe `MenuPayloads` hanya memuat `'menu:zoom'` dan `'files:dropped'`; untuk event tanpa payload, tipe callback `fn: (p: MenuPayloads[K]) => void` memerlukan akses yang aman: definisikan pembantu `type PayloadOf<K extends MenuEventName> = K extends keyof MenuPayloads ? MenuPayloads[K] : undefined` dan pakai di `index.ts`/`bus.ts` (signature publik tetap kompatibel dengan kontrak). Ekspor juga `ALL_EVENTS: readonly MenuEventName[]` berisi enam nama (boleh di `types.ts`).
2. `bus.ts`: `const listeners = new Map<string, Set<(p: any) => void>>()`; `export function on(name, fn): () => void` (tambah ke set, kembalikan unsubscribe yang menghapus dan membersihkan set kosong); `export function emit(name, payload?)` (salin set sebelum iterasi; tiap callback dibungkus try/catch dengan `console.error('appmenu: handler gagal', name, e)` sehingga satu error tidak menghentikan yang lain).
3. `bridge.ts`: helper `function api(): any { return (window as any).go?.main?.Bridge }`.
   - `getAppConfig(): Promise<AppConfig>`: bila `api()?.AppConfigGet` ada, `await` dan kembalikan hasil (normalisasi `zoom` numerik, bila tidak valid 100); bila tidak ada atau reject: `{zoom: 100}`.
   - `setAppConfig(patch): Promise<AppConfig>`: bila `AppConfigSet` ada, panggil dengan `patch`; gagal/tidak ada: `{zoom: 100, ...patch}`.
   - `pickDirectory(): Promise<string | null>`: bila `OpenDirectoryDialog` ada, hasil string tidak kosong dikembalikan, selain itu atau saat reject: `null`.
   Tidak melempar error ke pemanggil.
4. `index.ts`: ekspor ulang tipe dari `types.ts` dan `getAppConfig`, `setAppConfig`, `pickDirectory` dari `bridge.ts`; definisikan
   ```ts
   export function onMenuEvent<K extends MenuEventName>(name: K, fn: (p: MenuPayloads[K]) => void): () => void
   export function emitMenuEvent<K extends MenuEventName>(name: K, p?: MenuPayloads[K]): void
   ```
   (implementasi mendelegasikan ke `bus.on`/`bus.emit` dengan cast longgar agar kompatibel dengan kontrak). Saat modul dimuat: `const rt = (window as any).runtime; if (rt?.EventsOn) { for (const name of ALL_EVENTS) rt.EventsOn(name, (payload: unknown) => bus.emit(name, payload)); }` tepat satu `EventsOn` per nama, dibungkus try/catch agar tanpa `window.runtime` tidak error. Pastikan impor `window` aman bila `window` tak terdefinisi tidak perlu (Vite browser).

### 4. Verifikasi

- `gofmt -l app internal/appconfig` tidak menampilkan apa pun; `go vet ./internal/appconfig/ ./app/` bila cepat.
- `go build ./...` lulus.
- `cd frontend && npm run build` lulus (tanpa dependency baru).
- Tinjau diff: tidak ada perubahan di luar ownedPaths; hapus `console.log` sementara.

## Perubahan

Dibuat:
- `internal/appconfig/appconfig.go`
- `frontend/src/modules/appmenu/types.ts`
- `frontend/src/modules/appmenu/bus.ts`
- `frontend/src/modules/appmenu/bridge.ts`
- `frontend/src/modules/appmenu/index.ts`

Diubah:
- `app/bridge.go` (konstanta, `startupHooks`/`registerStartupHook`, `SetContext` menjalankan hook, `emit`, `relay` memakai `emit`, field `cfgOnce/cfg/cfgErr`, `configStore`, tiga method bridge baru)

## Wiring

Yang diekspos untuk disambungkan nanti (JANGAN disambungkan sendiri):

- `native-menu-impl` memakai `Event*` dan `b.emit`; `main.go` tetap memanggil `bridge.SetContext(ctx)` di `OnStartup`, yang menjalankan hook.
- `open-folder` (`app/dialog.go`) mendaftar lewat `registerStartupHook` di `init()` dan memakai `EventFilesDropped`.
- Frontend `zoom` dan `openfolder` mengimpor dari `'../appmenu'`: `onMenuEvent`, `getAppConfig`, `setAppConfig`, `pickDirectory`.
- Method `OpenDirectoryDialog` otomatis mengaktifkan tombol Browse v1 (`modules/workspace/errors.ts`) tanpa menyunting file itu.
- `modules/appmenu/index.ts` dimuat otomatis oleh glob `main.ts` (tidak diedit).

## Selesai bila

- [ ] `go build ./...` sukses.
- [ ] `cd frontend && npm run build` sukses, tanpa dependency baru.
- [ ] `internal/appconfig` hanya memakai stdlib dan `internal/paths`.
- [ ] `Patch({"zoom": 999})` menghasilkan 300, `Patch({"zoom": 10})` menghasilkan 50 (dibuktikan lewat pembacaan kode, tanpa unit test); file ditulis atomik dengan izin 0600.
- [ ] File rusak/kosong/tidak ada tidak membuat `Get` gagal; kunci tak dikenal pada file valid dipertahankan oleh `Patch`.
- [ ] `emit` sebelum `SetContext` tidak panic dan tidak mengirim apa pun; hook terdaftar dipanggil tepat sekali per `SetContext` dengan ctx yang benar dan secara berurutan.
- [ ] `OpenDirectoryDialog` mengembalikan `""` dengan error nil saat dibatalkan; `AppConfigGet/Set` mengembalikan error jelas (bukan panic) bila `DataDir` gagal.
- [ ] `appmenu/index.ts` mengekspor semua simbol Kontrak; `onMenuEvent` mengembalikan unsubscribe yang berfungsi; modul dimuat tanpa error di luar Wails.
- [ ] Hanya path di ownedPaths yang berubah; `lib/ipc.ts`, `lib/registry.ts`, `main.ts`, `package.json`, `go.mod` tidak berubah.
