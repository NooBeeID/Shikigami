---
id: shikigami-v2-ui-ticket-01-app-install
title: Instalasi Shikigami.app ke Applications
spec: docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md
status: implemented
wave: 1
depends_on: []
---
# Instalasi Shikigami.app ke Applications

## Tujuan

Epic ini membuat Shikigami bisa dipasang sebagai aplikasi macOS sungguhan: bundle `Shikigami.app` dirakit dari biner hasil `make build`, ditandatangani ad-hoc, disalin ke `/Applications`, dan dibuka dari Launchpad/Spotlight/Dock (spec bagian 7). Aplikasi GUI yang dibuka dari Finder tidak mewarisi PATH shell, sehingga `claude`/`codex` (nvm, Homebrew) tidak ditemukan. Karena itu epic ini juga membuat daemon mengambil PATH login-shell pengguna saat start dan memakainya untuk mencari biner provider dan untuk environment semua sesi PTY. Epic ini dikerjakan pertama karena kecil, mandiri, dan tidak mengubah UI v1.

## Hasil yang bisa dijalankan

Setelah tiket ini selesai (di atas kode v1 yang sudah ada):
- `make app` menghasilkan `build/Shikigami.app` yang lolos `codesign --verify --deep --strict`.
- `make install` menghentikan `shikigamid` lama lalu menyalin bundle ke `/Applications/Shikigami.app` (menimpa versi lama). Aplikasi muncul di Launchpad/Spotlight dan terbuka dengan UI v1 yang tetap jalan.
- Dari aplikasi yang dibuka lewat Finder atau `open -a Shikigami`, sesi `claude`, `codex`, dan `shell` berhasil start (PATH login-shell terpakai).
- `make uninstall` menghapus bundle; data di `~/Library/Application Support/shikigami` tidak tersentuh.
- `make link-cli` (opsional) membuat symlink `shikigamictl` ke `/usr/local/bin`.
- `make build` dan semua perintah yang sudah ada tetap berfungsi.

## Batasan

In scope:
- Paket `internal/loginenv` dan penyambungannya (PATH login-shell) ke pencarian biner dan env PTY.
- Skrip perakit bundle, pembuat ikon sementara, ad-hoc codesign.
- Target Makefile `app`, `install`, `uninstall`, `link-cli` dan `scripts/install.sh`.

Out of scope:
- Developer ID, notarization, DMG, distribusi ke mesin lain.
- Desain ikon final (hanya ikon sementara yang bisa diganti nanti).
- Menu native, zoom, Open Folder (tiket 02), perubahan UI apa pun.
- Wails CLI; bundle dirakit dengan skrip sendiri.
- Mengubah `internal/provider/provider.go`, `claude.go`, `codex.go`, `shell.go` (bukan milik epic ini; cukup `launch.go`).
- Unit test (guard hanya build).
- Menambah dependency Go/npm baru (stdlib saja).

## Kontrak

Disediakan epic ini:

Go, paket `internal/loginenv`:
```go
// Path mengembalikan PATH login-shell yang di-cache (sync.Once). Tidak pernah string kosong.
func Path() string
// Resolve menjalankan resolusi (idempoten) dan mengembalikan PATH final; dipanggil sekali saat daemon start.
func Resolve(logger *slog.Logger) string
// Merge menggabungkan PATH login-shell, PATH proses saat ini, dan lokasi umum
// (/opt/homebrew/bin, /usr/local/bin, ~/.nvm/versions/node/*/bin, ~/.local/bin, /usr/bin, /bin, /usr/sbin, /sbin)
// tanpa duplikat, urutan: login-shell dulu.
func Merge(parts ...string) string
```
Go, `internal/provider/launch.go` (tambahan, tidak mengubah API yang ada):
```go
// SetPathSource memasang sumber PATH untuk env sesi. Nil berarti memakai os.Environ apa adanya.
func SetPathSource(fn func() string)
```
Perilaku: `Launcher.Command` menyusun env dari `os.Environ()`, lalu bila `SetPathSource` terpasang menambahkan `PATH=<nilai>` setelahnya (menimpa), sebelum `s.Env`/`ctx.Env` sehingga override per sesi tetap menang. Flag izin agent tidak disentuh: `withFlag` di `claude.go`/`codex.go` tetap satu-satunya penambah flag (`--dangerously-skip-permissions` untuk claude, `--dangerously-bypass-approvals-and-sandbox` untuk codex), tidak digandakan.

Daemon, `cmd/shikigamid/wire_loginenv.go`: `RegisterModule("loginenv", ...)` yang memanggil `loginenv.Resolve`, lalu `os.Setenv("PATH", hasil)` (supaya `exec.LookPath` di `provider.LookPath` dan child process mewarisinya) dan `provider.SetPathSource(loginenv.Path)`. Menyediakan layanan `d.Provide("loginenv.path", loginenv.Path)`.

Skrip dan target:
- `scripts/make-app.sh` (tanpa argumen): input `build/bin/shikigami`, `build/bin/shikigamid`, `build/bin/shikigamictl`; output `build/Shikigami.app`. Exit non-nol dengan pesan jelas bila ada biner hilang ("jalankan 'make build' dulu").
- `scripts/make-icon.sh <sumber.png> <keluaran.icns>`; sumber default `build/app-resources/icon-source.png`.
- `scripts/install.sh install|uninstall|link-cli`.
- Target Makefile: `app`, `install`, `uninstall`, `link-cli`.

Konstanta bundle: nama `Shikigami`, `CFBundleIdentifier=id.noobee.shikigami`, `CFBundleExecutable=shikigami`, `CFBundleIconFile=shikigami`, `LSMinimumSystemVersion` (pakai 11.0 kecuali go.mod/Wails menuntut lain), `NSHighResolutionCapable=true`, `CFBundlePackageType=APPL`, `CFBundleShortVersionString` dari `VERSION` Makefile (default `dev` -> `0.0.0-dev`).

Dipakai epic ini (sudah ada): `RegisterModule`, `Deps.Provide` (`cmd/shikigamid/modules.go`), `provider.RegisterLaunchContribution`/`Launcher` (`internal/provider/launch.go`), target `build`, `build-ui`, `build-daemon`, `build-app` di `Makefile`. Daemon-autostart v1 mencari `shikigamid` di folder yang sama dengan biner app, jadi tata letak `Contents/MacOS/` wajib datar.

## Fitur

### loginenv-path: PATH login-shell untuk daemon dan provider

Deskripsi:
1. Buat `internal/loginenv/loginenv.go`. `Resolve` menjalankan `exec.CommandContext(ctx, shell, "-l", "-i", "-c", `printf %s "$PATH"`)` dengan `shell = os.Getenv("SHELL")` (fallback `/bin/zsh`), timeout 3 detik. Stdin diarahkan ke `/dev/null`; stderr dibuang. Shell interaktif bisa mencetak banner atau noise sebelum PATH, jadi cetak dengan penanda: `printf '__SHIKIGAMI_PATH__%s__END__' "$PATH"` lalu ambil teks di antara penanda (lebih tangguh daripada mengambil seluruh stdout). Sama sekali jangan menambahkan flag izin agent di sini.
2. Bila gagal (timeout, shell tidak ada, keluaran tanpa penanda atau kosong), fallback ke `Merge(os.Getenv("PATH"), lokasi umum)`. Catat via `logger.Warn` dengan alasan; bila sukses `logger.Info` dengan jumlah entri PATH (bukan seluruh nilai bila panjang).
3. Hasil di-cache (`sync.Once`); `Path()` sebelum `Resolve` dipanggil memicu `Resolve(nil)` (logger nil aman). Hasil akhir selalu melewati `Merge` agar lokasi umum (Homebrew, `/usr/local/bin`, `~/.nvm/versions/node/*/bin` via `filepath.Glob`, terurut menurun menurut nama, `~/.local/bin`) ikut sebagai cadangan di belakang PATH login-shell.
4. `internal/provider/launch.go`: tambahkan `SetPathSource` (guard dengan mutex atau `atomic.Pointer`), dan di `Launcher.Command` tambahkan `PATH=` setelah `os.Environ()` dan sebelum `s.Env`. `Command` provider (claude/codex/shell) tetap dipanggil apa adanya. Verifikasi dengan membaca kode bahwa `claudeSkipPermissionsFlag` dan `codexBypassFlag` serta `withFlag` tidak berubah dan tidak ada jalur baru yang menambah flag.
5. `cmd/shikigamid/wire_loginenv.go`: modul `loginenv` sesuai bagian Kontrak. Urutan modul alfabetis dan `Resolve` harus selesai sebelum sesi pertama dibuat; karena `Resolve` sinkron di wire dan dibatasi 3 detik, start daemon tertunda paling lama 3 detik, dan itu dapat diterima. Setelah `os.Setenv("PATH", ...)`, `provider.LookPath` (yang memakai `exec.LookPath`) otomatis ikut PATH baru tanpa mengubah `provider.go`.

Acceptance criteria:
- [ ] `go build ./...` lulus; paket `internal/loginenv` hanya memakai stdlib.
- [ ] Bila shell login valid, `Path()` berisi direktori tempat `claude`/`codex` berada walau daemon dijalankan dengan `env -i PATH=/usr/bin:/bin ./build/bin/shikigamid`.
- [ ] Bila `SHELL` menunjuk biner yang tidak ada atau shell menggantung lebih dari 3 detik, daemon tetap start dalam kurang lebih 3 detik dengan PATH fallback dan log peringatan.
- [ ] Noise di stdout shell interaktif (banner, motd) tidak merusak PATH hasil (penanda dipakai).
- [ ] Sesi yang dibuat setelahnya mewarisi `PATH` login-shell di env PTY (cek `echo $PATH` di sesi `shell`).
- [ ] `--dangerously-skip-permissions` (claude) dan `--dangerously-bypass-approvals-and-sandbox` (codex) tetap ada tepat satu kali di argumen; tidak ada perubahan di `withFlag`.
- [ ] Tidak ada polling atau goroutine berumur panjang.

ownedPaths: `internal/loginenv/`, `internal/provider/launch.go`, `cmd/shikigamid/wire_loginenv.go`.
Paralel: fitur ini bisa dikerjakan paralel dengan `app-bundle-script` dan `make-install-targets`.

### app-bundle-script: Skrip perakit bundle .app, ikon sementara, codesign

Deskripsi:
1. `scripts/make-icon.sh <sumber.png> <keluaran.icns>`: buat iconset sementara (`mktemp -d`, `shikigami.iconset`) berisi ukuran 16, 32, 128, 256, 512 beserta varian `@2x` memakai `sips -z <h> <w>`, lalu `iconutil -c icns`. Bersihkan direktori sementara dengan `trap`. Gagal jelas bila `sips`/`iconutil` tidak ada atau sumber tidak ada.
2. Gambar sumber: `build/app-resources/icon-source.png` (1024x1024). Bila belum ada, skrip membuatnya otomatis tanpa dependency baru: tulis `build/app-resources/icon-source.svg` (kotak membulat gelap beraksen, huruf "S" atau simbol sederhana) lalu konversi ke PNG dengan `qlmanage -t -s 1024 -o` (bawaan macOS) atau `sips -s format png`; bila konversi SVG tidak tersedia, fallback ke PNG polos yang dihasilkan dengan skrip Swift/Python bawaan sistem atau `base64` dari PNG kecil yang disimpan sebagai berkas teks di `build/app-resources/`. Pilih satu cara yang berjalan di macOS bersih tanpa instalasi tambahan, dan dokumentasikan di komentar skrip bahwa ikon ini sementara dan cukup mengganti `icon-source.png` untuk menggantinya. Catatan: `build/` ada di `make clean`; karena itu berkas sumber ikon yang di-commit harus aman dari `clean` (bila `clean` menghapus `build/`, simpan sumber di `build/app-resources/` hanya bila `.gitignore` tidak mengecualikannya, jika tidak, buat ulang otomatis tiap kali). Skrip wajib sanggup membuat ulang sumbernya dari nol.
3. `scripts/make-app.sh` (`set -euo pipefail`, `cd` ke akar repo): periksa tiga biner di `build/bin/`; bila ada yang hilang, cetak "biner <nama> belum dibuild; jalankan 'make build' dulu" ke stderr dan exit 1. Hapus `build/Shikigami.app` lama (idempoten), buat `Contents/MacOS` dan `Contents/Resources`, salin biner (mode 755), tulis `Info.plist` (heredoc di dalam skrip bash diperbolehkan; kunci sesuai bagian Kontrak), panggil `make-icon.sh` ke `Contents/Resources/shikigami.icns`, lalu `codesign --force --deep -s - build/Shikigami.app`. Akhiri dengan `codesign --verify --deep --strict` dan cetak jalur bundle.
4. Validasi `Info.plist` dengan `plutil -lint`. Versi dibaca dari env `VERSION` (default `dev`).
5. Jangan menaruh biner tambahan di luar `Contents/MacOS/`.

Acceptance criteria:
- [ ] Dijalankan setelah `make build`, menghasilkan `build/Shikigami.app` dengan `Contents/MacOS/{shikigami,shikigamid,shikigamictl}`, `Contents/Info.plist`, `Contents/Resources/shikigami.icns`.
- [ ] `plutil -lint build/Shikigami.app/Contents/Info.plist` OK dan `CFBundleIdentifier` bernilai `id.noobee.shikigami`.
- [ ] `codesign --verify --deep --strict build/Shikigami.app` exit 0.
- [ ] Dijalankan dua kali berturut-turut tetap sukses (idempoten, tidak ada sisa bundle lama).
- [ ] Bila `build/bin/shikigamid` dihapus, skrip gagal dengan pesan jelas dan exit non-nol tanpa meninggalkan bundle setengah jadi.
- [ ] `.icns` valid (`file build/Shikigami.app/Contents/Resources/shikigami.icns` melaporkan "Mac OS X icon") dan ikon tampil di Finder.
- [ ] Skrip executable (`chmod +x`).

ownedPaths: `scripts/make-app.sh`, `scripts/make-icon.sh`, `build/app-resources/`.
Paralel: bisa dikerjakan paralel dengan `loginenv-path`; tidak bergantung pada `make-install-targets`.

### make-install-targets: Target make app, install, uninstall, link-cli

Deskripsi:
1. `Makefile`: tambahkan ke `.PHONY` dan definisikan:
   - `app: build` lalu `./scripts/make-app.sh` (`VERSION` diteruskan).
   - `install: app` lalu `./scripts/install.sh install`.
   - `uninstall:` memanggil `./scripts/install.sh uninstall`.
   - `link-cli:` memanggil `./scripts/install.sh link-cli` (tidak bergantung `install`, tetapi menautkan ke biner di bundle terpasang bila ada, kalau tidak ke `build/bin/shikigamictl`).
   Jangan mengubah target yang sudah ada (`build`, `build-ui`, `build-daemon`, `build-app`, `run-daemon`, `clean`, `check`) selain menambah `.PHONY`.
2. `scripts/install.sh install`: tutup aplikasi bila berjalan (`osascript -e 'tell application "Shikigami" to quit'` dibungkus agar tidak gagal), hentikan daemon lama (`pkill -x shikigamid || true`, lalu tunggu singkat hingga proses hilang, maksimal sekitar 3 detik; bila ada perintah resmi `shikigamictl shutdown` atau serupa di v1, boleh dicoba lebih dulu), hapus `/Applications/Shikigami.app` lama, salin dengan `ditto build/Shikigami.app /Applications/Shikigami.app`, jalankan `codesign --verify --deep --strict` pada hasil salinan, dan cetak instruksi membuka (`open -a Shikigami`). Bila `/Applications` tidak bisa ditulis, cetak pesan jelas (bukan menjalankan `sudo` otomatis).
3. `uninstall`: hentikan daemon dan app, hapus `/Applications/Shikigami.app`, hapus symlink `link-cli` bila menunjuk ke Shikigami; tidak pernah menyentuh `~/Library/Application Support/shikigami`. Cetak pesan bahwa data dipertahankan beserta jalurnya.
4. `link-cli`: `ln -sf` ke `/usr/local/bin/shikigamictl`; bila direktori tidak ada atau tidak bisa ditulis, cetak instruksi (mis. `sudo ln -sf ...`) dan exit non-nol.
5. Semua perintah aman dijalankan berulang (idempoten) dan memakai `set -euo pipefail`.

Acceptance criteria:
- [ ] `make app` lulus dari kondisi bersih (`make clean && make app`) dan menghasilkan bundle valid.
- [ ] `make install` menghasilkan `/Applications/Shikigami.app`; `codesign --verify --deep --strict /Applications/Shikigami.app` lulus; menimpa instalasi lama tanpa error.
- [ ] `shikigamid` lama yang sedang berjalan dihentikan sebelum penyalinan; setelah `open -a Shikigami` daemon baru dari bundle yang berjalan (`pgrep -fl shikigamid` menunjuk ke path dalam `/Applications/Shikigami.app/Contents/MacOS/`).
- [ ] `make uninstall` menghapus bundle dan tidak menghapus apa pun di `~/Library/Application Support/shikigami`.
- [ ] `make uninstall` pada kondisi belum terpasang tidak gagal (idempoten).
- [ ] `make link-cli` membuat symlink yang bisa dieksekusi, atau gagal dengan instruksi jelas bila izin kurang.
- [ ] Target lama (`make build`, `make check`, `make run-daemon`) tidak berubah perilakunya.

ownedPaths: `Makefile`, `scripts/install.sh`.
Bergantung pada `app-bundle-script` untuk pengujian end-to-end (memanggil `scripts/make-app.sh`), tetapi file-nya bisa ditulis paralel; jalankan verifikasi setelah keduanya ada.

## Path yang dimiliki

Hanya path ini yang boleh disentuh epic ini:
- `scripts/make-app.sh`
- `scripts/make-icon.sh`
- `scripts/install.sh`
- `build/app-resources/`
- `Makefile`
- `internal/loginenv/`
- `internal/provider/launch.go`
- `cmd/shikigamid/wire_loginenv.go`

Dilarang menyentuh (dimiliki epic lain atau bukan bagian tiket ini): `app/`, `frontend/`, `internal/workspace/`, `internal/appconfig/`, `internal/status/`, `internal/provider/provider.go`, `claude.go`, `codex.go`, `shikigamid/modules.go`, `go.mod`.

## Wiring

Tiket ini berdiri sendiri dan tidak bergantung pada tiket lain. Langkah penutup:
1. Pasang `loginenv` ke daemon lewat titik ekstensi yang sudah ada: `cmd/shikigamid/wire_loginenv.go` memanggil `RegisterModule("loginenv", ...)` dari `init()` (satu file `wire_*.go` per modul, tanpa mengedit `modules.go`). Modul memanggil `loginenv.Resolve`, `os.Setenv("PATH", ...)`, `provider.SetPathSource(loginenv.Path)`, dan `d.Provide("loginenv.path", loginenv.Path)`.
2. Sambungkan `scripts/make-app.sh` dan `scripts/install.sh` ke `Makefile` lewat target `app`, `install`, `uninstall`, `link-cli`.
3. Verifikasi menyeluruh (manual, hasil dilaporkan di catatan implementasi):
   - `make build && make app`, lalu `codesign --verify --deep --strict build/Shikigami.app`.
   - `make install`, lalu buka dari Spotlight atau `open -a Shikigami`: jendela UI v1 muncul, daemon ter-autostart dari bundle.
   - Buat satu sesi `shell` dari UI dan jalankan `echo $PATH` (berisi PATH login-shell), lalu buat sesi `claude` dan/atau `codex` bila terpasang: sesi start tanpa error "binary tidak ditemukan".
   - `make uninstall` membersihkan bundle; data tetap ada.
4. Pastikan `make build` dan `go build ./...` tetap lulus, dan aplikasi dari `build/bin/shikigami` (tanpa bundle) tetap bisa dijalankan seperti v1.

## Guard

Cukup build berhasil, tanpa unit test:
- `go build ./...` (atau `make check`)
- `make build`
- `make app` (merakit dan memverifikasi bundle)
