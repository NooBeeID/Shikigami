# Foundation: Epic 08 Session Status

Dikerjakan SATU agent sebelum fitur paralel. Baca `overview.md` (bagian 2, 4, 5) dan tiket dulu. Tujuan: `go build ./...` hijau dengan semua signature kontrak sudah ada (sebagian stub) sehingga `status-engine`, `hook-receiver`, dan `status-notify-ui` bisa dikerjakan bersamaan dan masing-masing hanya mengganti stub miliknya.

Batasan path: hanya buat `internal/status/*.go`, `cmd/aitermd/wire_status.go`, `cmd/aitermctl/cmd_status.go`. Jangan membuat apa pun di `frontend/`. Jangan sentuh `go.mod`/`go.sum` atau path terlarang di overview bagian 7.

## Langkah

1. **Baca kode nyata** untuk memastikan signature: `internal/ipc/server.go` (`Register`, `Publish`), `internal/ipc/errors.go`, `internal/session/types.go` dan `manager.go` (`Get`, `List`), `internal/provider/launch.go` dan `provider.go`, `cmd/aitermd/modules.go`, `cmd/aitermd/wire_session.go`, `cmd/aitermctl/registry.go`. Bila ada perbedaan dengan overview (nama method, tipe), sesuaikan stub dengan kode nyata dan catat di "Catatan Foundation" di bawah.

2. **`internal/status/types.go`**: salin persis tipe dan konstanta dari overview 2.3 (`Status` + 4 konstanta, `Source*`, `Hook*`, `SilentDelay`, `SilentDelayWithHooks`, `StatusEvent`, `SessionState`, `LaunchInfo`). Beri komentar paket di file ini (`// Paket status ...`, Indonesia).

3. **Stub per file** dengan signature persis overview 2.4. Tiap file stub diawali komentar `// STUB(foundation): diganti oleh <slug>`; badan fungsi minimal (kembalikan nilai nol, `ErrUnknownSession`, atau no-op) tetapi aman dipanggil (tidak panic, `Engine` nil-safe tidak wajib).
   - `engine.go` (`status-engine`): `ErrUnknownSession`, `SessionSource`, `Engine` (struct kosong), `NewEngine`, `SetDefault`, `Default` (simpan di variabel paket dengan mutex atau `atomic.Pointer`), seluruh metode `Engine` di 2.4 (`SetSessionSource`, `RegisterLaunch`, `ResolveSessionID`, `Track`, `Set`, `Apply`, `OnOutput`, `Get`, `List`, `Lookup`, `Subscribe`, `Forget`) dan fungsi paket `RegisterLaunch`.
   - `idle.go` (`status-engine`): fungsi paket `IsIdle` dan `OnIdle`.
   - `ipc.go` (`status-engine`): `RegisterIPC(srv, eng)` kosong.
   - `hooks.go` (`hook-receiver`): `HookPayload` (field minimal), `ParseHookEvent` (kembalikan `name, map[string]any{}`), `ClassifyEvent` LANGSUNG FINAL sesuai tabel (UserPromptSubmit/SessionStart -> running, Notification -> waiting, Stop -> done, lainnya `("", false)`).
   - `install.go` (`hook-receiver`): `InstallHooks()` kosong dan `CleanupHooks(token string)` kosong.
   - `notify.go` (`status-notify-ui`): `StartNotifier(srv, eng)` kosong.
   Gunakan impor yang benar-benar dipakai agar `go vet` bersih (mis. `_ = srv`).

4. **`cmd/aitermd/wire_status.go`** (versi final, urutan persis overview bagian 5 butir 1): `init(){ RegisterModule("status", func(srv *ipc.Server, d *Deps){ ... }) }` memanggil `status.NewEngine`, `status.SetDefault`, `d.Provide("status.engine", eng)`, `status.RegisterIPC`, `status.InstallHooks()`, `status.StartNotifier`, dan `d.OnReady` yang memasang `eng.SetSessionSource` dari `d.Lookup("session.manager")` (type-assert `*session.Manager`, log warning bila gagal; `d.Logger` boleh nil-check). Pastikan `*session.Manager` memenuhi `status.SessionSource`.

5. **`cmd/aitermctl/cmd_status.go`** stub: `init(){ RegisterCommand(Command{Name:"status", Summary:"Status sesi: list|get|watch|hook", Run: ...}) }` dengan `Run` mengembalikan `errors.New("status: belum diimplementasi")`. Awali dengan `// STUB(foundation): diganti oleh status-engine`.

6. **Build.** `gofmt -l internal/status cmd` kosong, `go build ./...` lolos, `go vet ./internal/status/... ./cmd/...` bersih bila tersedia. Jalankan `aitermd` singkat (atau cukup build) untuk memastikan tidak ada panic pada `init()` (nama modul `status` unik, command `status` unik).

7. Pastikan perubahan hanya pada path di batasan di atas.

## Catatan Foundation
(diisi agent Foundation: perbedaan signature terhadap overview bila ada, dan keputusan stub)
