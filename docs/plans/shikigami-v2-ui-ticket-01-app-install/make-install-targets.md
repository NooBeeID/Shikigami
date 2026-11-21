# Plan: Target make app, install, uninstall, link-cli

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-01-app-install.md` (`shikigami-v2-ui-ticket-01-app-install`)
- Fitur: `make-install-targets`
- ownedPaths: `Makefile`, `scripts/install.sh`
- Depends on foundation: tidak (`hasFoundation=false`)
- Konteks: `docs/plans/shikigami-v2-ui-ticket-01-app-install/overview.md` (bagian 2.4, 2.5, 3, 6, 7). Jangan memodifikasi overview.

## Tujuan

Menyediakan antarmuka pemasangan: target Makefile `app`, `install`, `uninstall`, `link-cli` dan skrip `scripts/install.sh install|uninstall|link-cli` yang menutup app, menghentikan `shikigamid` lama, menyalin bundle ke `/Applications` dengan `ditto`, memverifikasi codesign, menghapus bundle tanpa menyentuh data dir, dan membuat symlink `shikigamictl` ke `/usr/local/bin`. Semua idempoten, tanpa `sudo` otomatis, target lama tidak berubah.

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `Makefile`
- `scripts/install.sh`

Jangan sentuh `scripts/make-app.sh`, `scripts/make-icon.sh`, `build/app-resources/` (fitur `app-bundle-script`), `internal/loginenv/`, `internal/provider/launch.go`, `cmd/shikigamid/wire_loginenv.go` (fitur `loginenv-path`), maupun `app/`, `frontend/`, `cmd/shikigamid/modules.go`, `go.mod`, `overview.md`.

## Kontrak

Disediakan:
- Target Makefile: `app` (`app: build`), `install` (`install: app`), `uninstall`, `link-cli`; semuanya masuk `.PHONY`.
- `scripts/install.sh <install|uninstall|link-cli>`, `set -euo pipefail`, executable.

Dipakai (disediakan fitur `app-bundle-script`, belum tentu ada saat Anda menulis; jangan membuatnya):
- `scripts/make-app.sh` tanpa argumen, membaca env `VERSION`, menghasilkan `build/Shikigami.app` (tata letak datar `Contents/MacOS/{shikigami,shikigamid,shikigamictl}`, lihat overview 2.4).

Konstanta di `install.sh`:
```bash
APP_NAME="Shikigami"
INSTALL_DIR="/Applications"
APP_DEST="$INSTALL_DIR/$APP_NAME.app"
LINK_PATH="/usr/local/bin/shikigamictl"
DATA_DIR="$HOME/Library/Application Support/shikigami"   # hanya dicetak, tidak pernah dihapus/diubah
```
Fakta kode v1: `cmd/shikigamictl` TIDAK punya perintah `shutdown`, jadi hentikan daemon hanya dengan `pkill -x shikigamid`.

## Langkah

1. `Makefile`: ubah baris `.PHONY` menjadi `.PHONY: build build-ui build-daemon build-app run-daemon clean check app install uninstall link-cli`. Tambahkan di akhir berkas (tab untuk recipe; jangan mengubah target lama):
   ```make
   app: build
   	VERSION=$(VERSION) ./scripts/make-app.sh

   install: app
   	./scripts/install.sh install

   uninstall:
   	./scripts/install.sh uninstall

   link-cli:
   	./scripts/install.sh link-cli
   ```
2. Buat `scripts/install.sh` (shebang `#!/usr/bin/env bash`, `set -euo pipefail`, `cd "$(dirname "$0")/.."` ke akar repo, komentar dan pesan Bahasa Indonesia, pesan error ke stderr). Isi:
   - Konstanta di atas, plus `SRC_APP="build/Shikigami.app"`.
   - `die()` mencetak `error: ...` ke stderr dan `exit 1`.
   - `quit_app()`: `osascript -e 'tell application "Shikigami" to quit' >/dev/null 2>&1 || true`, lalu tunggu singkat (`sleep 1`) bila `pgrep -x shikigami` ada (loop maks ~3 detik dengan `sleep 0.5`).
   - `stop_daemon()`: `pkill -x shikigamid >/dev/null 2>&1 || true`; loop maks 6 kali `sleep 0.5` (~3 detik) sampai `pgrep -x shikigamid` kosong; bila masih hidup cetak peringatan ke stderr (jangan gagal, jangan `kill -9`).
   - `require_writable()`: bila `[ ! -w "$INSTALL_DIR" ]`, `die` dengan pesan jelas ("$INSTALL_DIR tidak bisa ditulis; jalankan dengan hak yang cukup, mis. tambahkan pengguna ke grup admin atau salin manual"), tanpa memanggil `sudo`.
   - `cmd_install()`: pastikan `[ -d "$SRC_APP" ]` kalau tidak `die "build/Shikigami.app belum ada; jalankan 'make app' dulu"` (tidak memanggil `make`); `require_writable`; `quit_app`; `stop_daemon`; `rm -rf "$APP_DEST"`; `ditto "$SRC_APP" "$APP_DEST"`; `codesign --verify --deep --strict "$APP_DEST"` (gagal -> `die`); cetak jalur terpasang dan instruksi `open -a Shikigami`.
   - `cmd_uninstall()`: `quit_app`; `stop_daemon`; bila `[ -e "$APP_DEST" ]` dan `[ -w "$INSTALL_DIR" ]` maka `rm -rf "$APP_DEST"` (bila ada tetapi tidak writable, `die` dengan pesan jelas; bila tidak ada, cetak "belum terpasang" dan tetap sukses); hapus symlink `LINK_PATH` hanya bila `[ -L "$LINK_PATH" ]` dan `readlink` hasilnya mengandung `Shikigami.app` atau berakhir `/build/bin/shikigamictl` (gagal hapus karena izin -> peringatan stderr plus saran `sudo rm`, bukan menjalankan sudo, tetap exit 0); cetak bahwa data dipertahankan beserta `$DATA_DIR`. Tidak ada perintah yang menyentuh `$DATA_DIR`.
   - `cmd_link_cli()`: target = `$APP_DEST/Contents/MacOS/shikigamictl` bila executable, kalau tidak `"$(pwd)/build/bin/shikigamictl"` (absolut) bila executable, kalau tidak `die "shikigamictl belum ada; jalankan 'make build' atau 'make install' dulu"`. Bila `/usr/local/bin` tidak ada atau tidak writable: cetak instruksi (`sudo mkdir -p /usr/local/bin && sudo ln -sf "<target>" /usr/local/bin/shikigamictl`) ke stderr dan `exit 1`. Selain itu `ln -sf "$target" "$LINK_PATH"` (idempoten), cetak hasilnya.
   - Dispatcher `case "${1:-}" in install) ...;; uninstall) ...;; link-cli) ...;; *) usage ke stderr, exit 2;; esac`.
3. `chmod +x scripts/install.sh`.
4. Verifikasi sintaks: `bash -n scripts/install.sh`; `make -n app install uninstall link-cli` (dry run, memastikan resep terbaca dan target lama utuh).

## Perubahan

- Diubah: `Makefile` (`.PHONY` + 4 target baru; target lama identik).
- Dibuat: `scripts/install.sh` (executable).

## Wiring

Tidak ada penyambungan di luar ownedPaths. Yang diekspos: target `make app/install/uninstall/link-cli` dan `scripts/install.sh`, yang memanggil `scripts/make-app.sh` (milik `app-bundle-script`) lewat target `app`. Langkah wiring nanti (overview bagian 5, butir 2, 3, 5) hanya memverifikasi bit executable dan alur end-to-end; fitur ini tidak mengedit file fitur lain.

## Selesai bila

- `bash -n scripts/install.sh` lulus dan berkas executable.
- `make -n app install uninstall link-cli` mencetak resep yang benar; `.PHONY` memuat keempat target; target lama tidak berubah selain `.PHONY`.
- Guard build path fitur ini: `make app` sukses bila `scripts/make-app.sh` sudah ada (butuh `make build` sekali); bila belum ada, cukup dua cek di atas dan catat bahwa verifikasi end-to-end menunggu `app-bundle-script`. `go build ./...` tetap lulus.
- Bila bundle tersedia: `make install` menghasilkan `/Applications/Shikigami.app` yang lolos `codesign --verify --deep --strict`; `make uninstall` dua kali sukses dan `~/Library/Application Support/shikigami` tidak tersentuh; `make link-cli` membuat symlink atau gagal dengan instruksi jelas; tidak ada `sudo` otomatis di skrip.
- Tanpa unit test, tanpa dependency baru.
