# Plan: PATH login-shell untuk daemon dan provider

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-01-app-install.md` (`shikigami-v2-ui-ticket-01-app-install`)
- Fitur: `loginenv-path`
- ownedPaths: `internal/loginenv/`, `internal/provider/launch.go`, `cmd/shikigamid/wire_loginenv.go`
- Depends on foundation: tidak (tidak ada `00-foundation.md`)
- Rujukan: `overview.md` bagian 2.1 (paket loginenv), 2.2 (SetPathSource), 2.3 (modul daemon), 2.6 (konvensi), 6 (guard), 7 (aturan paralel)

## Tujuan

Aplikasi GUI yang dibuka dari Finder tidak mewarisi PATH shell, sehingga `claude`/`codex` tidak ditemukan. Fitur ini membuat daemon mengambil PATH login-shell pengguna saat start (dengan timeout 3 detik dan fallback), memasangnya ke `os.Setenv("PATH")` agar `provider.LookPath` dan child process ikut, dan menambahkannya eksplisit ke env setiap sesi PTY lewat `provider.SetPathSource`. Flag izin agent tidak boleh berubah.

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `internal/loginenv/` (baru; berkas `loginenv.go`)
- `internal/provider/launch.go` (diubah)
- `cmd/shikigamid/wire_loginenv.go` (baru)

Jangan sentuh: `cmd/shikigamid/modules.go`, `internal/provider/provider.go`, `claude.go`, `codex.go`, `shell.go`, `go.mod`, `Makefile`, `scripts/`, `build/app-resources/`, `app/`, `frontend/`, `internal/workspace/`, `internal/appconfig/`, `internal/status/`, `overview.md`. Jangan menambah dependency; jangan menulis unit test.

## Kontrak

Disediakan (signature persis seperti overview 2.1-2.3):

```go
// internal/loginenv
func Path() string
func Resolve(logger *slog.Logger) string
func Merge(parts ...string) string

// internal/provider/launch.go
func SetPathSource(fn func() string)

// cmd/shikigamid/wire_loginenv.go
// RegisterModule("loginenv", ...) ; d.Provide("loginenv.path", loginenv.Path)
```

Dipakai (sudah ada di v1): `RegisterModule(name string, wire func(srv *ipc.Server, d *Deps))`, `Deps.Logger *slog.Logger`, `Deps.Provide(name string, v any)` di `cmd/shikigamid/modules.go`; `Launcher.Command`, `withFlag`, `claudeSkipPermissionsFlag`, `codexBypassFlag` di `launch.go`. Modul `loginenv` terurut alfabetis sebelum `provider` dan `session`, jadi `Resolve` selesai sebelum sesi pertama dibuat.

Nama modul dan nama layanan: `"loginenv"` dan `"loginenv.path"` (nilai berupa `func() string`).

## Langkah

1. Buat `internal/loginenv/loginenv.go`, `package loginenv`, impor stdlib saja (`context`, `log/slog`, `os`, `os/exec`, `path/filepath`, `sort`, `strings`, `sync`, `time`). Komentar bahasa Indonesia gaya `// Nama ...`.
   - Konstanta: `markerStart = "__SHIKIGAMI_PATH__"`, `markerEnd = "__END__"`, `timeout = 3 * time.Second`, `defaultShell = "/bin/zsh"`.
   - Variabel paket: `once sync.Once`, `resolved string`.
2. Implementasi `Merge(parts ...string) string`:
   - Pecah tiap argumen dengan `strings.Split(p, ":")`, buang entri kosong, dedup memakai `map[string]bool` sambil menjaga urutan: entri argumen dulu sesuai urutan, lalu lokasi umum di belakang.
   - Lokasi umum berurutan: `/opt/homebrew/bin`, `/usr/local/bin`, hasil `filepath.Glob(home + "/.nvm/versions/node/*/bin")` (urut menurun menurut nama dengan `sort.Sort(sort.Reverse(sort.StringSlice(m)))`), `home + "/.local/bin"`, `/usr/bin`, `/bin`, `/usr/sbin`, `/sbin`. `home` dari `os.UserHomeDir()`; bila gagal atau kosong, lewati lokasi berbasis home.
   - Hasil `strings.Join(out, ":")`. Tidak pernah kosong karena lokasi umum selalu ikut.
3. Implementasi helper tak-terekspor `fromLoginShell() (string, error)`:
   - `shell := os.Getenv("SHELL")`; kosong -> `defaultShell`.
   - `ctx, cancel := context.WithTimeout(context.Background(), timeout)`; `defer cancel()`.
   - `cmd := exec.CommandContext(ctx, shell, "-l", "-i", "-c", `printf "__SHIKIGAMI_PATH__%s__END__" "$PATH"`)`; `cmd.Stdin = nil` (Go membuka `/dev/null`), `cmd.Stderr = nil` (dibuang), ambil `cmd.Output()`. Set `cmd.WaitDelay = 500 * time.Millisecond` agar pipe anak shell yang menggantung tidak menahan `Output` melewati timeout.
   - Bila error (termasuk `ctx.Err()`), kembalikan error dengan alasan (timeout / gagal jalan). Output tetap diperiksa hanya bila error nil.
   - Ekstrak teks di antara penanda: `strings.Index` untuk `markerStart`, lalu `strings.Index` untuk `markerEnd` setelahnya (pakai `LastIndex` untuk awal bila banner mungkin memuat penanda? tidak perlu; pakai `Index` pertama). Tanpa penanda atau hasil kosong setelah `TrimSpace` -> error "keluaran tanpa penanda" / "PATH kosong".
4. Implementasi `Resolve(logger *slog.Logger) string`:
   - `once.Do`: panggil `fromLoginShell`. Sukses -> `resolved = Merge(p)`; `logger.Info("loginenv: PATH login-shell dipakai", "entries", n)` dengan `n` = jumlah entri `strings.Split(resolved, ":")` (jangan log seluruh nilai). Gagal -> `resolved = Merge(os.Getenv("PATH"))`; `logger.Warn("loginenv: PATH login-shell gagal, memakai fallback", "reason", err.Error(), "entries", n)`.
   - Guard logger nil di setiap pemanggilan (fungsi kecil `logInfo`/`logWarn` atau cek `if logger != nil`).
   - Kembalikan `resolved` (idempoten: pemanggilan kedua mengembalikan cache tanpa menjalankan shell lagi).
5. Implementasi `Path() string`: `return Resolve(nil)` (memicu resolusi bila belum, kalau sudah mengembalikan cache). Tanpa goroutine, tanpa polling.
6. Edit `internal/provider/launch.go` (hanya tambahan, jangan ubah `withFlag`, konstanta flag, `applyContributions`, `RegisterLaunchContribution`):
   - Tambah impor `sync/atomic`.
   - Tambah di dekat blok var:
     ```go
     var pathSource atomic.Pointer[func() string]

     // SetPathSource memasang sumber PATH untuk env sesi. Nil berarti memakai os.Environ apa adanya.
     func SetPathSource(fn func() string) {
         if fn == nil {
             pathSource.Store(nil)
             return
         }
         pathSource.Store(&fn)
     }
     ```
   - Di `Launcher.Command`, tepat setelah `env = append(env, os.Environ()...)` dan sebelum baris `TERM=...`, sisipkan:
     ```go
     if src := pathSource.Load(); src != nil {
         if p := (*src)(); p != "" {
             env = append(env, "PATH="+p)
         }
     }
     ```
     Urutan akhir: `os.Environ()`, `PATH=`, `TERM/COLORTERM`, `s.Env`, `ctx.Env`, `extra`.
   - Verifikasi dengan membaca bahwa `claudeSkipPermissionsFlag`, `codexBypassFlag`, `withFlag` tidak berubah (diff hanya menambah impor, var, fungsi, dan blok `PATH=`), dan tidak ada jalur baru yang menambah flag.
7. Buat `cmd/shikigamid/wire_loginenv.go`, `package main`, impor `os`, `internal/ipc`, `internal/loginenv`, `internal/provider`:
   ```go
   func init() {
       RegisterModule("loginenv", func(srv *ipc.Server, d *Deps) {
           p := loginenv.Resolve(d.Logger)
           _ = os.Setenv("PATH", p)
           provider.SetPathSource(loginenv.Path)
           d.Provide("loginenv.path", loginenv.Path)
       })
   }
   ```
   Komentar Indonesia: Resolve sinkron dan dibatasi 3 detik sehingga start daemon tertunda paling lama 3 detik; modul alfabetis sebelum `provider`/`session`. Jangan mengedit `modules.go`; pastikan tidak ada nama modul `loginenv` lain (`grep -rn '"loginenv"' cmd/`).
8. Jalankan `gofmt -l internal/loginenv internal/provider cmd/shikigamid` (harus kosong) dan `go vet ./internal/loginenv ./internal/provider ./cmd/shikigamid`, lalu `go build ./...`.

## Perubahan

Dibuat:
- `internal/loginenv/loginenv.go`
- `cmd/shikigamid/wire_loginenv.go`

Diubah:
- `internal/provider/launch.go` (impor `sync/atomic`, `pathSource`, `SetPathSource`, blok `PATH=` di `Command`)

## Wiring

Fitur ini sudah mencakup `init()` `RegisterModule("loginenv", ...)` di `wire_loginenv.go` (ownedPaths-nya sendiri). Yang diekspos untuk langkah wiring/verifikasi akhir:
- Modul daemon `loginenv` (terdaftar otomatis lewat `init()`; `modules.go` tidak diubah).
- Layanan `d.Provide("loginenv.path", loginenv.Path)`.
- `provider.SetPathSource` terpasang ke `loginenv.Path`.
Jangan menyambungkan apa pun di luar ownedPaths; verifikasi end-to-end (`env -i ...`, `SHELL=/tidak/ada`, `echo $PATH` di sesi `shell`) dilakukan agen wiring.

## Selesai bila

- [ ] `go build ./...` sukses (guard fitur ini; tanpa unit test).
- [ ] `internal/loginenv` hanya mengimpor stdlib; `go.mod` tidak berubah.
- [ ] Signature `Path`, `Resolve`, `Merge`, `SetPathSource` sama persis dengan Kontrak; `Path()` tidak pernah kosong.
- [ ] Perintah memakai `$SHELL -l -i -c` dengan penanda `__SHIKIGAMI_PATH__...__END__`, fallback `/bin/zsh`, timeout 3 detik, stdin `/dev/null`, stderr dibuang; gagal -> `Merge(os.Getenv("PATH"))` + `Warn`; hasil di-cache `sync.Once`.
- [ ] `Launcher.Command` menambahkan `PATH=` setelah `os.Environ()` dan sebelum `TERM`/`s.Env`/`ctx.Env`; `withFlag` dan konstanta flag tidak berubah, flag tidak digandakan.
- [ ] Tidak ada goroutine berumur panjang atau polling; hanya tiga ownedPaths yang berubah.
