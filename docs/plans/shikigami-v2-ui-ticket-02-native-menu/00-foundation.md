# Foundation: menu-bridge-contract

Fitur ini digabung SEBELUM `native-menu-impl`, `zoom`, dan `open-folder`. Baca `overview.md` untuk kontrak lengkap; dokumen ini hanya langkah kerja.

ownedPaths: `app/bridge.go`, `internal/appconfig/`, `frontend/src/modules/appmenu/`.

Guard: `go build ./...` dan `cd frontend && npm run build`.

## Langkah 1: `internal/appconfig/appconfig.go`

1. Package `appconfig`, hanya stdlib dan `github.com/NooBeeID/shikigami/internal/paths`.
2. Tipe `Window{X, Y, W, H int}` (tag json `x,y,w,h`), `Config{Zoom int; Window *Window}` (tag `zoom`, `window,omitempty`), `Default()` mengembalikan `Config{Zoom: 100}`.
3. `Store{mu sync.Mutex; path string}`, `Open(path)` tanpa membaca disk, `DefaultPath()` = `filepath.Join(paths.DataDir(), "config.json")` (baca signature `paths.DataDir` dahulu; teruskan error-nya).
4. Helper internal `readRaw() (map[string]any, error)`: file tidak ada, kosong, atau JSON rusak menjadi map kosong (rusak dicatat dengan `log.Printf`, bukan error). Error I/O selain not-exist dikembalikan.
5. `Get()`: baca raw, decode ke `Config` (mulai dari `Default()`), normalisasi `Zoom` (clamp 50..300; 0 atau tak valid menjadi 100). Dikunci mutex.
6. `Patch(patch)`: kunci mutex; baca raw; untuk tiap kunci patch: hanya `zoom` dan `window` yang dikenal, lainnya diabaikan. `zoom`: terima `float64`/`int`/`json.Number`, bulatkan, clamp 50..300. `window`: map, gabung satu tingkat ke map `window` yang ada (kunci x,y,w,h). Kunci tak dikenal yang sudah ada di file tetap dipertahankan. Tulis atomik: `os.CreateTemp` di folder yang sama, `Chmod 0600`, tulis, `Close`, `os.Rename`; `MkdirAll` folder (0700) dulu. Kembalikan `Config` hasil gabungan (lewat jalur yang sama dengan `Get`).

## Langkah 2: `app/bridge.go`

1. Tambah konstanta: `EventMenuZoom = "menu:zoom"`, `EventMenuOpenFolder = "menu:open-folder"`, `EventMenuNewWorkspaceFromFolder = "menu:new-workspace-from-folder"`, `EventMenuToggleSidebarLeft = "menu:toggle-sidebar-left"`, `EventMenuToggleSidebarRight = "menu:toggle-sidebar-right"`, `EventFilesDropped = "files:dropped"`.
2. Hook startup: variabel paket `startupHooks []func(ctx context.Context, b *Bridge)` dan `registerStartupHook(fn)` (hanya dipanggil dari `init()`, tanpa mutex). Ubah `SetContext`: simpan ctx di bawah `ctxMu`, lepas kunci, lalu jalankan semua hook berurutan dengan ctx itu.
3. `func (b *Bridge) emit(name string, data ...any)`: baca ctx lewat `ctxMu.RLock`; bila nil, return; selain itu `runtime.EventsEmit(ctx, name, data...)`. Jangan mengambil `b.mu`. Boleh refaktor `relay` agar memakai `emit` bila perilakunya identik (`daemon:event` tetap).
4. Field `cfgOnce sync.Once`, `cfg *appconfig.Store`, `cfgErr error` pada `Bridge`; helper `configStore()` membuat store malas (`appconfig.DefaultPath()` lalu `appconfig.Open`); error dibungkus pesan jelas (`"gagal menentukan lokasi config: %w"`), tidak panic.
5. Method bridge: `OpenDirectoryDialog() (string, error)` memakai `runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: "Pilih folder project", CanCreateDirectories: true})`; ctx nil menjadi error "aplikasi belum siap"; hasil kosong (dibatalkan) menjadi `"", nil`. `AppConfigGet() (appconfig.Config, error)` dan `AppConfigSet(patch map[string]any) (appconfig.Config, error)` mendelegasikan ke store.
6. Jangan mengubah `Call`, `Close`, `connect`, `drop`.

## Langkah 3: `frontend/src/modules/appmenu/`

1. `types.ts`: `MenuEventName`, `MenuPayloads`, `AppConfig` persis seperti Kontrak.
2. `bus.ts`: `Map<string, Set<callback>>`, fungsi `on(name, fn)` (mengembalikan unsubscribe), `emit(name, payload)`. Error dalam satu callback tidak menghentikan callback lain (try/catch + `console.error`).
3. `bridge.ts`: akses `(window as any).go?.main?.Bridge`; `getAppConfig`, `setAppConfig`, `pickDirectory`. Bila method tidak ada atau promise ditolak: `getAppConfig` mengembalikan `{zoom: 100}`; `setAppConfig` mengembalikan `{zoom: 100, ...patch}`; `pickDirectory` mengembalikan `null` (string kosong juga menjadi `null`). Tanpa melempar error.
4. `index.ts`: ekspor `onMenuEvent`, `emitMenuEvent`, `getAppConfig`, `setAppConfig`, `pickDirectory`, dan tipe. Saat modul dimuat, bila `window.runtime?.EventsOn` ada, daftarkan satu `EventsOn(name, payload => bus.emit(name, payload))` per nama event di daftar `ALL_EVENTS` (enam nama). Modul tidak memanggil `registerPanel`. Tidak mengimpor modul lain.
5. Cek `frontend/src/env.d.ts`; bila perlu deklarasi `window.runtime`/`window.go`, gunakan cast `(window as any)` di dalam modul alih-alih menyunting file itu.

## Langkah 4: Verifikasi

- `go build ./...` dan `cd frontend && npm run build` lulus.
- Pastikan `lib/ipc.ts`, `lib/registry.ts`, `main.ts`, `package.json` tidak berubah.
