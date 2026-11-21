# Overview: Instalasi Shikigami.app ke Applications

Tiket: `docs/tickets/shikigami-v2-ui-ticket-01-app-install.md`
Spec: `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md` (bagian 7, 10, 11b)
Modul Go: `github.com/NooBeeID/shikigami` (go 1.26.3, stdlib saja untuk epic ini)

## 1. Ringkasan epic dan urutan eksekusi

Epic membuat Shikigami terpasang sebagai aplikasi macOS: `make app` merakit `build/Shikigami.app` dari biner hasil `make build`, ditandatangani ad-hoc; `make install` menyalin ke `/Applications`; `make uninstall` menghapus; `make link-cli` membuat symlink `shikigamictl`. Karena aplikasi GUI dari Finder tidak mewarisi PATH shell, daemon mengambil PATH login-shell saat start dan memakainya untuk `exec.LookPath` provider dan env semua sesi PTY. UI v1 tidak berubah.

Urutan eksekusi:
1. Foundation: tidak ada (`hasFoundation=false`, tidak ada `00-foundation.md`). Ketiga fitur tidak berbagi tipe baru atau skeleton; kontraknya cukup di bagian 2.
2. Fitur paralel: `loginenv-path`, `app-bundle-script`, `make-install-targets` (file-nya terpisah semua, aman paralel).
3. Wiring: verifikasi menyeluruh dan penyambungan akhir (bagian 5). Wiring `wire_loginenv.go` dan target Makefile sudah menjadi ownedPaths fitur masing-masing, jadi agen wiring terutama memverifikasi end-to-end.

Kondisi kode saat ini (v1):
- `cmd/shikigamid/modules.go`: `RegisterModule(name, func(srv *ipc.Server, d *Deps))` dipanggil dari `init()`, satu file `wire_*.go` per modul, urutan wire alfabetis. `Deps` punya `Logger *slog.Logger`, `Provide(name, v)`, `Lookup`, `OnReady`.
- `internal/provider/launch.go`: `Launcher.Command` menyusun env: `os.Environ()`, `TERM`, `COLORTERM`, `s.Env`, `ctx.Env`, `extra`. `withFlag`, `claudeSkipPermissionsFlag`, `codexBypassFlag` ada di sini dan di `claude.go`/`codex.go`.
- `provider.LookPath` membungkus `exec.LookPath` (jadi otomatis ikut `os.Setenv("PATH")`).
- `Makefile`: `.PHONY: build build-ui build-daemon build-app run-daemon clean check`; biner keluar di `build/bin/{shikigami,shikigamid,shikigamictl}`; `clean` menjalankan `rm -rf build ...`; `build/` ada di `.gitignore`.
- `scripts/` sudah berisi `build.sh`, `ticket-status.sh`.

## 2. Kontrak bersama

### 2.1 Go: paket `internal/loginenv` (disediakan `loginenv-path`)
```go
package loginenv

// Path mengembalikan PATH login-shell yang di-cache (sync.Once). Tidak pernah string kosong.
// Bila Resolve belum dipanggil, memicu Resolve(nil).
func Path() string
// Resolve menjalankan resolusi (idempoten) dan mengembalikan PATH final; dipanggil sekali saat daemon start.
// logger boleh nil.
func Resolve(logger *slog.Logger) string
// Merge menggabungkan beberapa string PATH (dipisah ':') dan lokasi umum tanpa duplikat;
// urutan: argumen dulu sesuai urutan, lokasi umum di belakang.
func Merge(parts ...string) string
```
Detail penting:
- Perintah: `$SHELL -l -i -c 'printf "__SHIKIGAMI_PATH__%s__END__" "$PATH"'`, fallback `/bin/zsh`, timeout 3 detik (`exec.CommandContext`), stdin `/dev/null`, stderr dibuang, ambil teks di antara penanda.
- Gagal (timeout/tidak ada shell/tanpa penanda/kosong): fallback `Merge(os.Getenv("PATH"))` + `logger.Warn`; sukses: `logger.Info` dengan jumlah entri.
- Hasil akhir selalu lewat `Merge`; lokasi umum: `/opt/homebrew/bin`, `/usr/local/bin`, `~/.nvm/versions/node/*/bin` (`filepath.Glob`, urut menurun menurut nama), `~/.local/bin`, `/usr/bin`, `/bin`, `/usr/sbin`, `/sbin`.
- Tanpa goroutine berumur panjang, tanpa polling, stdlib saja.

### 2.2 Go: `internal/provider/launch.go` (tambahan)
```go
// SetPathSource memasang sumber PATH untuk env sesi. Nil berarti memakai os.Environ apa adanya.
func SetPathSource(fn func() string)
```
Thread-safe (mutex atau `atomic.Pointer`). Di `Launcher.Command`: setelah `env = append(env, os.Environ()...)` dan sebelum `TERM`/`s.Env`/`ctx.Env`, bila source terpasang dan hasilnya tidak kosong, tambahkan `"PATH=" + fn()` (menimpa karena entri terakhir menang) agar override per sesi tetap menang. Jangan mengubah `withFlag`, konstanta flag, atau jalur penambahan flag; flag izin tidak boleh digandakan.

### 2.3 Daemon: `cmd/shikigamid/wire_loginenv.go` (milik `loginenv-path`)
`init()` memanggil `RegisterModule("loginenv", func(srv *ipc.Server, d *Deps) {...})` yang: `p := loginenv.Resolve(d.Logger)`; `os.Setenv("PATH", p)`; `provider.SetPathSource(loginenv.Path)`; `d.Provide("loginenv.path", loginenv.Path)`. Tidak mengedit `modules.go`.

### 2.4 Konstanta bundle
| Kunci | Nilai |
|---|---|
| Nama bundle | `Shikigami` -> `build/Shikigami.app` |
| `CFBundleName` / `CFBundleDisplayName` | `Shikigami` |
| `CFBundleIdentifier` | `id.noobee.shikigami` |
| `CFBundleExecutable` | `shikigami` |
| `CFBundleIconFile` | `shikigami` |
| `CFBundlePackageType` | `APPL` |
| `LSMinimumSystemVersion` | `11.0` |
| `NSHighResolutionCapable` | `true` |
| `CFBundleShortVersionString` / `CFBundleVersion` | dari env `VERSION` (default `dev` -> `0.0.0-dev`; selain `dev` dipakai apa adanya) |

Tata letak datar (wajib, karena autostart v1 mencari `shikigamid` di folder yang sama dengan biner app):
```
build/Shikigami.app/Contents/Info.plist
build/Shikigami.app/Contents/MacOS/{shikigami,shikigamid,shikigamictl}   (mode 755)
build/Shikigami.app/Contents/Resources/shikigami.icns
```

### 2.5 Skrip dan target (antarmuka antar fitur)
- `scripts/make-icon.sh <sumber.png> <keluaran.icns>`: sumber default `build/app-resources/icon-source.png`; bila sumber default belum ada, skrip membuatnya sendiri dari nol (build/ dihapus oleh `make clean` dan di-gitignore, jadi harus selalu bisa dibuat ulang tanpa dependency baru).
- `scripts/make-app.sh` (tanpa argumen, `set -euo pipefail`, `cd` ke akar repo): input `build/bin/{shikigami,shikigamid,shikigamictl}`, output `build/Shikigami.app`; biner hilang -> pesan stderr "biner <nama> belum dibuild; jalankan 'make build' dulu", exit 1, tanpa bundle setengah jadi. Memanggil `scripts/make-icon.sh`. Membaca `VERSION` dari env.
- `scripts/install.sh install|uninstall|link-cli` (`set -euo pipefail`, idempoten). `install` memakai `build/Shikigami.app` yang sudah dirakit (tidak memanggil `make`).
- Makefile: target `app` (`app: build` -> `VERSION=$(VERSION) ./scripts/make-app.sh`), `install` (`install: app` -> `./scripts/install.sh install`), `uninstall`, `link-cli`; semuanya masuk `.PHONY`.
- Konstanta bersama untuk skrip: `APP_NAME=Shikigami`, `INSTALL_DIR=/Applications`, `LINK_PATH=/usr/local/bin/shikigamictl`, data dir `~/Library/Application Support/shikigami` (tidak boleh disentuh).

### 2.6 Konvensi
- Komentar dan pesan pengguna dalam bahasa Indonesia; identifier kode bahasa Inggris. Komentar Go gaya `// Nama ...` seperti v1.
- Tanpa dependency baru (Go/npm). Tanpa unit test. `go.mod` tidak disentuh.
- Flag izin agent tidak boleh ditambahkan di `loginenv` maupun jalur baru mana pun.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `loginenv-path` | Paket `internal/loginenv` (Path/Resolve/Merge), `SetPathSource` + `PATH=` di `Launcher.Command`, modul daemon `loginenv` | `internal/loginenv/`, `internal/provider/launch.go`, `cmd/shikigamid/wire_loginenv.go` |
| `app-bundle-script` | `make-icon.sh` (iconset -> `.icns`, sumber ikon sementara dibuat otomatis), `make-app.sh` (rakit bundle, `Info.plist`, `plutil -lint`, ad-hoc codesign + verify) | `scripts/make-app.sh`, `scripts/make-icon.sh`, `build/app-resources/` |
| `make-install-targets` | Target Makefile `app/install/uninstall/link-cli` dan `scripts/install.sh` (tutup app, hentikan daemon, `ditto`, verify, uninstall aman data, symlink CLI) | `Makefile`, `scripts/install.sh` |

Catatan per fitur:
- `app-bundle-script`: karena `build/` ada di `.gitignore` dan dihapus `make clean`, `build/app-resources/icon-source.png` hanya artefak turunan; `make-icon.sh` wajib bisa membuatnya ulang (mis. SVG -> PNG via `qlmanage -t -s 1024` atau `sips`, atau generator Python/Swift bawaan sistem; pilih satu yang jalan di macOS bersih dan verifikasi nyata). Dokumentasikan di komentar skrip bahwa ikon sementara dan cukup mengganti `icon-source.png`.
- `make-install-targets`: boleh mencoba `shikigamictl shutdown` bila ada di v1 (cek `cmd/shikigamictl`), selalu dibungkus `|| true`; lalu `pkill -x shikigamid || true` dan tunggu maks ~3 detik. Jangan pernah `sudo` otomatis. `link-cli` menautkan ke `/Applications/Shikigami.app/Contents/MacOS/shikigamictl` bila ada, kalau tidak ke `build/bin/shikigamictl` (path absolut).
- Hanya `launch.go` yang boleh diubah di `internal/provider/`.

## 4. Foundation

Tidak ada. Tidak ada tipe/skeleton bersama yang harus ada lebih dulu: `internal/loginenv` baru dan hanya dipakai oleh `wire_loginenv.go` (fitur yang sama), sedangkan fitur skrip/Makefile hanya berkomunikasi lewat jalur file dan nama target di 2.4-2.5. Karena itu `00-foundation.md` tidak ditulis.

## 5. Wiring checklist

Dikerjakan setelah ketiga fitur selesai. Agen wiring hanya boleh menyentuh file yang diperlukan untuk memperbaiki sambungan; bila ada bug di fitur, laporkan/perbaiki seminimal mungkin.
1. Daemon: pastikan `cmd/shikigamid/wire_loginenv.go` terdaftar via `init()` (`RegisterModule("loginenv", ...)`) dan tidak ada duplikat nama modul; `modules.go` tidak diubah. Pastikan `provider.SetPathSource(loginenv.Path)` dan `d.Provide("loginenv.path", loginenv.Path)` terpanggil.
2. Makefile: `app`, `install`, `uninstall`, `link-cli` ada di `.PHONY`; `app` memanggil `./scripts/make-app.sh`, `install` memanggil `./scripts/install.sh install`, dst; target lama tidak berubah perilaku.
3. Skrip: `scripts/make-app.sh`, `scripts/make-icon.sh`, `scripts/install.sh` executable (`chmod +x`).
4. Verifikasi flag izin: `grep` bahwa `claudeSkipPermissionsFlag`, `codexBypassFlag`, `withFlag` tidak berubah dan flag muncul sekali per sesi.
5. Verifikasi menyeluruh (laporkan di catatan implementasi, jangan mengubah path terlarang):
   - `make build && make app`; `codesign --verify --deep --strict build/Shikigami.app`; `plutil -lint .../Info.plist`; `file .../shikigami.icns`; `make app` kedua kali sukses; hapus `build/bin/shikigamid` sementara -> gagal dengan pesan jelas.
   - `make install` -> `/Applications/Shikigami.app` valid; `open -a Shikigami` -> jendela UI v1 muncul, `pgrep -fl shikigamid` menunjuk ke `/Applications/Shikigami.app/Contents/MacOS/`.
   - Sesi `shell` di UI: `echo $PATH` berisi PATH login-shell; sesi `claude`/`codex` start tanpa error "binary tidak ditemukan".
   - `env -i PATH=/usr/bin:/bin ./build/bin/shikigamid`: `loginenv` menemukan direktori `claude`/`codex`; dengan `SHELL=/tidak/ada` daemon start <= ~3 detik dengan log peringatan.
   - `make uninstall` (dua kali) menghapus bundle dan tidak menyentuh `~/Library/Application Support/shikigami`.
6. Pastikan `make build`, `make check`, dan `go build ./...` tetap lulus, dan `build/bin/shikigami` tanpa bundle tetap jalan seperti v1.

## 6. Build guard

Satu-satunya guard (tanpa unit test):
- `go build ./...` (alias `make check`)
- `make build`
- `make app`
Untuk fitur: `loginenv-path` cukup `go build ./...`; `app-bundle-script` dan `make-install-targets` memakai `make app` (butuh `make build` sudah dijalankan sekali). Untuk skrip tambahan `bash -n scripts/<nama>.sh`.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain.
- Dilarang untuk semua: `app/`, `frontend/`, `internal/workspace/`, `internal/appconfig/`, `internal/status/`, `internal/provider/provider.go`, `claude.go`, `codex.go`, `shell.go`, `cmd/shikigamid/modules.go`, `go.mod`.
- `Makefile` hanya milik `make-install-targets`; tidak ada fitur lain yang mengeditnya. `cmd/shikigamid/wire_loginenv.go` hanya milik `loginenv-path`.
- Tidak ada file bersama yang diedit lebih dari satu fitur, sehingga tidak ada konflik merge; ketergantungan antar fitur hanya pada nama berkas/target di 2.5. Verifikasi end-to-end `make-install-targets` baru dilakukan setelah `app-bundle-script` ada.
- Jangan menambah dependency, tidak menulis unit test, tidak mengubah UI.
