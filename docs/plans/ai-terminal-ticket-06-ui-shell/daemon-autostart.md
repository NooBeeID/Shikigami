# Plan: Auto-start daemon dari UI

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-06-ui-shell.md` (epic 06 UI Shell)
- Fitur: `daemon-autostart`
- ownedPaths: `app/daemon.go`
- Depends on foundation: tidak (fitur Go murni; foundation hanya untuk frontend)
- Referensi: `overview.md` bagian 1.2 (keputusan 4), 2.6 (kontrak Go), 6 (build guard), 7 (aturan paralel)

## Tujuan

Memastikan daemon `aitermd` hidup sebelum UI dipakai. `app/daemon.go` menyediakan `EnsureDaemon(ctx) error` (ping, bila gagal spawn `aitermd` terlepas lalu tunggu ping) dan `DaemonStartupError() error`. Karena `app/bridge.go` belum punya hook `RegisterStartup`, `init()` menjalankan satu goroutine `EnsureDaemon` sekali jalan dan menyimpan hasilnya. UI tidak pernah mengirim sinyal ke daemon; daemon tetap hidup saat window ditutup.

## Path yang dimiliki

HANYA `app/daemon.go` (file baru) yang boleh dibuat/diubah. Dilarang menyentuh `app/main.go`, `app/bridge.go`, `go.mod`, `go.sum`, `internal/*`, `cmd/*`, `Makefile`, `frontend/*`, maupun file fitur lain. Tanpa unit test. Impor hanya stdlib + `aiterm/internal/ipc` + `aiterm/internal/paths` (module `aiterm`).

## Kontrak

Disediakan (package `main`, sesuai overview 2.6):

```go
func EnsureDaemon(ctx context.Context) error // ping; bila gagal spawn aitermd terlepas, tunggu ping (timeout 5 dtk, interval 100 ms)
func DaemonStartupError() error              // hasil EnsureDaemon dari init(); nil bila sukses atau belum selesai
```

Dipakai (sudah ada, jangan diubah):

- `paths.SocketPath() (string, error)` dan `paths.DataDir() (string, error)` (DataDir juga membuat direktori 0700; env `AITERM_DATA_DIR` meng-override).
- `ipc.Dial(socketPath) (*ipc.Client, error)`, `(*ipc.Client).Call(method string, params any, out any) error`, `(*ipc.Client).Close() error`. Method `ping` dilayani daemon (tiket 01).

Aturan perilaku:

- Socket basi (file ada, `ping` ditolak) dibiarkan; JANGAN `os.Remove` socket dari UI, daemon baru yang membersihkannya.
- Tidak pernah mengirim sinyal ke daemon dan tidak memanggil `Process.Kill`/`Signal`.
- Penantian ping hanya saat startup (satu kali), bukan polling berkelanjutan.
- Semua kegagalan dikembalikan sebagai `error` berpesan jelas (bahasa Indonesia), tidak ada `panic`.
- Nama identifier non-ekspor baru harus unik di package `main` (hindari bentrok dengan `bridge.go`/`main.go`): beri awalan `daemon`/`aitermd`, mis. `daemonPing`, `findDaemonBinary`, `spawnDaemon`.

## Langkah

1. Baca `app/bridge.go` dan `app/main.go` (hanya baca) untuk memastikan tidak ada identifier yang bentrok (`EnsureDaemon`, `DaemonStartupError`, `init`, dan helper di bawah) dan bahwa memang tidak ada hook `RegisterStartup`. Baca `internal/ipc/client.go` untuk memastikan signature `Dial`/`Call`/`Close`. Pastikan `app/dist` ada (`go:embed all:dist`), buat `.gitkeep` TIDAK perlu bila sudah ada; bila tidak ada, laporkan, jangan membuat file di luar ownedPaths.

2. Buat `app/daemon.go`, `package main`, impor: `context`, `errors`, `fmt`, `os`, `os/exec`, `path/filepath`, `sync`, `syscall`, `time`, `aiterm/internal/ipc`, `aiterm/internal/paths`.

3. Konstanta dan state paket:

   ```go
   const (
       daemonBinaryName = "aitermd"
       daemonLogName    = "aitermd.log"
       daemonWaitTimeout  = 5 * time.Second
       daemonWaitInterval = 100 * time.Millisecond
   )

   var (
       daemonStartupMu  sync.RWMutex
       daemonStartupErr error
   )
   ```

4. `func daemonPing(socketPath string) error`: `c, err := ipc.Dial(socketPath)`; bila err kembalikan err; `defer c.Close()`; `return c.Call("ping", nil, nil)`. (Bila `Call` dengan `out == nil` ternyata tidak diterima oleh implementasi, pakai `var out any`; verifikasi di langkah 1.) Tidak menghapus apa pun, tidak ada retry di dalam fungsi ini.

5. `func findDaemonBinary() (string, error)`:
   - `exe, err := os.Executable()`; bila sukses, `exe, _ = filepath.EvalSymlinks(exe)` (abaikan error, pakai `exe` asli bila gagal), `dir := filepath.Dir(exe)`.
   - Kandidat berurutan: `filepath.Join(dir, "aitermd")`, `filepath.Join(dir, "..", "Resources", "aitermd")`, `filepath.Join(dir, "..", "MacOS", "aitermd")` (bundle `.app`).
   - Kandidat diterima bila `os.Stat` sukses, bukan direktori, dan `Mode().Perm()&0o111 != 0`.
   - Bila tak ada, `exec.LookPath("aitermd")`.
   - Bila semua gagal: `fmt.Errorf("biner %s tidak ditemukan (dicari di sebelah aplikasi dan di PATH)", daemonBinaryName)`.

6. `func openDaemonLog() *os.File`: `dir, err := paths.DataDir()`; bila sukses `os.OpenFile(filepath.Join(dir, daemonLogName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)`; bila gagal pada salah satu langkah, fallback `os.OpenFile(os.DevNull, os.O_RDWR, 0)`; bila itu pun gagal kembalikan `nil` (stdio `exec.Cmd` nil berarti `/dev/null`).

7. `func spawnDaemon(bin string) error`:
   - `cmd := exec.Command(bin)`; `cmd.Stdin = nil`; `logf := openDaemonLog()`; bila `logf != nil` set `cmd.Stdout = logf`, `cmd.Stderr = logf`.
   - `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}`.
   - `err := cmd.Start()`; tutup `logf` di sisi induk setelah `Start` (anak sudah punya salinan fd), baik sukses maupun gagal.
   - Bila `Start` gagal: `fmt.Errorf("gagal menjalankan %s: %w", bin, err)`.
   - `cmd.Process.Release()` agar tidak ada hubungan induk-anak (abaikan/bungkus error Release sebagai error non-fatal: cukup abaikan). Jangan menyimpan `cmd` atau memanggil `Wait`/`Kill`/`Signal`.
   - Catatan: `Setsid` tersedia di `syscall.SysProcAttr` untuk darwin/linux (target epic ini adalah macOS; Windows ditunda sesuai spec). Jangan menambah file build-tag terpisah (di luar ownedPaths).

8. `func EnsureDaemon(ctx context.Context) error`:
   - `sock, err := paths.SocketPath()`; bila err: `fmt.Errorf("gagal menentukan path socket daemon: %w", err)`.
   - `if daemonPing(sock) == nil { return nil }` (daemon sudah jalan, tidak spawn proses kedua).
   - `bin, err := findDaemonBinary()`; bila err kembalikan err.
   - `if err := spawnDaemon(bin); err != nil { return err }`.
   - Tunggu: `deadline := time.NewTimer(daemonWaitTimeout)`, `ticker := time.NewTicker(daemonWaitInterval)`, keduanya `defer Stop()`. Loop `for { select { case <-ctx.Done(): return fmt.Errorf("menunggu daemon dibatalkan: %w", ctx.Err()); case <-deadline.C: return fmt.Errorf("daemon tidak siap dalam %s (socket: %s); periksa log di %s", daemonWaitTimeout, sock, <log path>) ; case <-ticker.C: if daemonPing(sock) == nil { return nil } } }`. Untuk `<log path>` pakai helper kecil `daemonLogPath()` yang mengembalikan `filepath.Join(DataDir, daemonLogName)` atau string `"(log tidak tersedia)"` bila DataDir gagal. Ticker ini hanya hidup selama startup dan dihentikan saat fungsi kembali; tidak ada loop setelah daemon siap.
   - Jangan hapus socket basi; jangan kirim sinyal.

9. `func DaemonStartupError() error`: `RLock`, kembalikan `daemonStartupErr`, `RUnlock`. Nilai `nil` berarti sukses atau EnsureDaemon belum selesai.

10. `func init()`: `go func() { err := EnsureDaemon(context.Background()); daemonStartupMu.Lock(); daemonStartupErr = err; daemonStartupMu.Unlock() }()`. Hanya satu goroutine, sekali jalan. Beri komentar bahwa ini pengganti hook `RegisterStartup` yang belum ada di `bridge.go` dan boleh diganti hook itu kelak. Tambahkan `recover` kecil di goroutine agar panic tak menjatuhkan app: simpan `fmt.Errorf("auto-start daemon panic: %v", r)`.

11. Format dan build: `gofmt -w app/daemon.go`; jalankan `gofmt -l app` (harus kosong), `go vet ./app/...`, lalu `go build ./...`. Bila `go build` gagal karena `app/dist` hilang, laporkan (jangan buat file di luar ownedPaths kecuali sudah ada konvensi `.gitkeep`; bila perlu, laporkan sebagai blocker).

12. Verifikasi manual opsional (tanpa unit test, tanpa mengubah repo): `go build -o /tmp-safe/aitermd ./cmd/aitermd` ke direktori scratch, set `AITERM_DATA_DIR` ke direktori pendek sementara, taruh biner di PATH, lalu jalankan potongan Go sekali pakai di luar repo yang memanggil logika yang sama; pastikan daemon naik dan proses kedua tidak muncul pada pemanggilan kedua. Bersihkan daemon uji lewat cara yang tidak berasal dari kode app (mis. `kill` manual oleh pengembang).

## Perubahan

- Dibuat: `app/daemon.go` (satu-satunya file).
- Tidak ada file lain diubah atau dihapus.

## Wiring

Yang diekspos: `EnsureDaemon(ctx context.Context) error` dan `DaemonStartupError() error` di package `main`, plus `init()` yang sudah memulai auto-start otomatis. JANGAN menyambungkan sendiri ke `app/main.go`/`app/bridge.go`.

Untuk langkah wiring/laporan akhir:

- Tidak ada edit file bersama yang dibutuhkan; `init()` sudah cukup agar daemon menyala saat app start.
- Catat di laporan: hook `RegisterStartup` tidak ada di `app/bridge.go` (kekurangan tiket 01); bila kelak ada, ganti `init()` dengan pendaftaran `EnsureDaemon` ke hook itu dan gunakan `DaemonStartupError()` bila perlu.
- UI menyelamatkan kasus ping awal yang lebih cepat dari daemon lewat layar error + "Coba lagi" (milik `shell-layout`).
- Verifikasi wiring: `app/main.go` dan `app/bridge.go` tak berubah; impor `app/daemon.go` hanya stdlib + `internal/ipc` + `internal/paths`.

## Selesai bila

- `app/daemon.go` ada dan merupakan satu-satunya file yang diubah.
- `go build ./...` sukses dan `gofmt -l app` kosong (guard fitur ini; tanpa unit test).
- `EnsureDaemon` mem-ping dulu dan tidak spawn bila daemon sudah hidup.
- Spawn memakai `Setsid: true`, stdio ke `<DataDir>/aitermd.log` (append, 0600; fallback `/dev/null`), `Process.Release()`; tidak ada `Kill`/`Signal`/`Wait`.
- Pencarian biner: sebelah executable (plus `../Resources`, `../MacOS`), lalu `PATH`; error jelas bila tidak ditemukan.
- Penantian ping 5 dtk/100 ms hanya saat startup, ticker dihentikan setelah selesai; error timeout/batal/biner hilang dikembalikan sebagai `error`, bukan panic.
- Socket basi tidak dihapus.
- `DaemonStartupError()` mengembalikan hasil `init()` secara thread-safe.
- `grep -n "STUB" app/daemon.go` kosong; tidak menyentuh `app/main.go` dan `app/bridge.go`.
