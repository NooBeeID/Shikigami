# Plan: Skrip perakit bundle .app, ikon sementara, codesign

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-01-app-install.md` (fitur `app-bundle-script`)
- Overview: `docs/plans/shikigami-v2-ui-ticket-01-app-install/overview.md` (bagian 2.4, 2.5, 3, 6, 7)
- Fitur: `app-bundle-script`
- ownedPaths: `scripts/make-app.sh`, `scripts/make-icon.sh`, `build/app-resources/`
- Depends on foundation: tidak (`hasFoundation=false`)
- Paralel dengan `loginenv-path` dan `make-install-targets`; tidak bergantung pada keduanya.

## Tujuan

Menyediakan dua skrip bash yang merakit `build/Shikigami.app` dari tiga biner di `build/bin/`, lengkap dengan `Info.plist` valid, ikon `.icns` sementara yang selalu bisa dibuat ulang dari nol, dan tanda tangan ad-hoc yang lolos verifikasi. Skrip idempoten dan gagal dengan pesan jelas tanpa meninggalkan bundle setengah jadi. Konstanta bundle dan tata letak datar mengikuti overview 2.4 (tidak diulang di sini).

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `scripts/make-app.sh` (baru)
- `scripts/make-icon.sh` (baru)
- `build/app-resources/` (artefak turunan, di-gitignore, dibuat skrip saat runtime; jangan di-commit)

Jangan menyentuh `Makefile`, `scripts/install.sh`, `scripts/build.sh`, `internal/`, `cmd/`, `app/`, `frontend/`, `go.mod`, atau file wiring apa pun.

## Kontrak

Disediakan (rujuk overview 2.5):
- `scripts/make-icon.sh <sumber.png> <keluaran.icns>`: argumen opsional. Sumber default `build/app-resources/icon-source.png` (relatif akar repo). Keluaran default `build/app-resources/shikigami.icns`. Bila sumber adalah sumber default dan belum ada, skrip membuatnya sendiri. Bila sumber eksplisit lain tidak ada: gagal.
- `scripts/make-app.sh` (tanpa argumen, membaca env `VERSION`, default `dev`): input `build/bin/{shikigami,shikigamid,shikigamictl}`, output `build/Shikigami.app`.
- Pesan galat biner hilang (persis, stderr, exit 1): `biner <nama> belum dibuild; jalankan 'make build' dulu`.
- Keluaran sukses make-app.sh: baris terakhir stdout berisi jalur absolut bundle.

Dipakai (sudah ada di mesin, macOS bawaan, tanpa instalasi): `sips`, `iconutil`, `qlmanage`, `plutil`, `codesign`, `base64`, `mktemp`, `install`/`cp`. Tidak ada dependency baru. Pemanggil: target Makefile `app` milik fitur lain (`VERSION=$(VERSION) ./scripts/make-app.sh`).

## Langkah

### 1. `scripts/make-icon.sh`

Header: `#!/usr/bin/env bash`, `set -euo pipefail`, `cd "$(dirname "$0")/.."` (akar repo). Komentar Indonesia di kepala berkas: ikon ini SEMENTARA; untuk menggantinya cukup menimpa `build/app-resources/icon-source.png` (1024x1024) lalu jalankan ulang `make app`; catat bahwa `make clean` menghapus `build/` sehingga sumber bawaan dibuat ulang otomatis (ganti ikon permanen nanti dengan mengubah skrip ini agar menyalin dari berkas yang di-commit di luar `build/`).

Isi berurutan:
1. Konstanta: `DEFAULT_SRC="build/app-resources/icon-source.png"`, `SRC="${1:-$DEFAULT_SRC}"`, `OUT="${2:-build/app-resources/shikigami.icns}"`.
2. Fungsi `need()` seperti di `scripts/build.sh` (pesan stderr `error: '<cmd>' tidak ditemukan ...`, exit 1). Panggil untuk `sips` dan `iconutil`.
3. Fungsi `die() { echo "error: $*" >&2; exit 1; }`.
4. Fungsi `generate_default_source()` hanya dipanggil bila `SRC == DEFAULT_SRC` dan berkasnya belum ada (atau ukuran bukan 1024x1024):
   - `mkdir -p build/app-resources`.
   - Tulis `build/app-resources/icon-source.svg` memakai `cat > ... <<'SVG'` (heredoc di dalam skrip bash boleh; ini isi skrip, bukan penulisan file lewat tool): `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 1024 1024">`, `<rect x="64" y="64" width="896" height="896" rx="200" fill="#1c1c24"/>`, lingkaran aksen tipis opsional, dan `<text x="512" y="720" font-size="600" font-family="Helvetica" font-weight="700" fill="#7c5cff" text-anchor="middle">S</text>`.
   - Jalur utama: `qlmanage -t -s 1024 -o <tmpdir> build/app-resources/icon-source.svg >/dev/null 2>&1`, hasilnya `<tmpdir>/icon-source.svg.png`; pindahkan ke `build/app-resources/icon-source.png`. Sudah diuji di mesin pembuat: menghasilkan PNG 1024x1024 ber-alpha.
   - Validasi hasil: `sips -g pixelWidth -g pixelHeight` harus 1024x1024 (parse dengan `awk`). Bila `qlmanage` tidak ada, gagal, atau ukuran salah: FALLBACK ke PNG kecil tertanam. Tanam string base64 sebuah PNG kecil (mis. 64x64, kotak gelap dengan aksen; implementor membuatnya sekali, mis. dari hasil jalur utama diperkecil `sips -z 64 64`, lalu `base64 < file.png` dan tempel ke skrip sebagai variabel `FALLBACK_PNG_B64`). Dekode dengan `base64 -D`/`base64 --decode` (pilih yang didukung macOS: `base64 -D`), simpan sebagai `icon-source.png`, lalu perbesar `sips -z 1024 1024 build/app-resources/icon-source.png`. Fallback ini hanya butuh `base64` dan `sips`, jadi jalan di macOS bersih. Ukuran string base64 harus kecil (<= ~4 KB).
   - Setelah selesai, cetak ke stderr/stdout: `ikon sementara dibuat: build/app-resources/icon-source.png`.
5. Alur utama:
   - `[ "$SRC" = "$DEFAULT_SRC" ] && [ ! -f "$SRC" ] && generate_default_source`.
   - `[ -f "$SRC" ] || die "sumber ikon tidak ditemukan: $SRC"`.
   - `WORK="$(mktemp -d)"`; `trap 'rm -rf "$WORK"' EXIT`.
   - `ICONSET="$WORK/shikigami.iconset"`; `mkdir -p "$ICONSET"`.
   - Loop ukuran `16 32 128 256 512`: `sips -z $s $s "$SRC" --out "$ICONSET/icon_${s}x${s}.png"` dan varian retina `d=$((s*2))`: `sips -z $d $d "$SRC" --out "$ICONSET/icon_${s}x${s}@2x.png"`. Redirect stdout `sips` ke `/dev/null`.
   - `mkdir -p "$(dirname "$OUT")"`; tulis ke berkas sementara dulu: `iconutil -c icns "$ICONSET" -o "$WORK/shikigami.icns"` lalu `mv "$WORK/shikigami.icns" "$OUT"` (supaya keluaran tidak setengah jadi bila `iconutil` gagal). Catatan: `mv` lintas volume tetap aman; bila `$WORK` berbeda volume, pakai `cp` lalu hapus.
   - Cetak `ikon dibuat: $OUT`.

### 2. `scripts/make-app.sh`

Header: `#!/usr/bin/env bash`, `set -euo pipefail`, `cd "$(dirname "$0")/.."`. Komentar Indonesia singkat yang menjelaskan input, output, tata letak datar, dan bahwa `VERSION` dibaca dari env.

Isi berurutan:
1. Konstanta: `APP_NAME="Shikigami"`, `BUNDLE_ID="id.noobee.shikigami"`, `EXEC_NAME="shikigami"`, `ICON_NAME="shikigami"`, `MIN_OS="11.0"`, `BIN_DIR="build/bin"`, `APP_DIR="build/${APP_NAME}.app"`, `BINARIES=(shikigami shikigamid shikigamictl)`.
2. Versi: `VERSION="${VERSION:-dev}"`; `if [ "$VERSION" = "dev" ]; then APP_VERSION="0.0.0-dev"; else APP_VERSION="$VERSION"; fi`. Pakai nilai ini untuk `CFBundleShortVersionString` dan `CFBundleVersion`. Escape XML minimal untuk nilai versi (ganti `&`, `<`, `>`) agar plist tidak rusak oleh `VERSION` aneh.
3. Cek prasyarat SEBELUM menyentuh apa pun: untuk tiap nama di `BINARIES`, `[ -f "$BIN_DIR/$n" ] && [ -x "$BIN_DIR/$n" ]` bila tidak: `echo "biner $n belum dibuild; jalankan 'make build' dulu" >&2; exit 1`. Cek juga `need` untuk `codesign`, `plutil`, `sips`, `iconutil`. Karena pengecekan dilakukan sebelum `rm -rf` bundle lama, bundle lama yang valid tidak rusak bila biner hilang.
4. Rakit di direktori staging, bukan langsung ke tujuan: `STAGE="build/.${APP_NAME}.app.tmp"`; `rm -rf "$STAGE"`; `trap 'rm -rf "$STAGE"' EXIT` (dihapus saat gagal; setelah `mv` sukses staging sudah tidak ada, `rm -rf` aman). Langkah di staging:
   - `mkdir -p "$STAGE/Contents/MacOS" "$STAGE/Contents/Resources"`.
   - Salin tiap biner dengan `install -m 755 "$BIN_DIR/$n" "$STAGE/Contents/MacOS/$n"` (tata letak datar, jangan ada biner di luar `Contents/MacOS/`).
   - Tulis `$STAGE/Contents/Info.plist` dengan heredoc tanpa kutip (`<<PLIST`) agar `${...}` terisi. Kunci: `CFBundleName`, `CFBundleDisplayName` = `Shikigami`; `CFBundleIdentifier` = `id.noobee.shikigami`; `CFBundleExecutable` = `shikigami`; `CFBundleIconFile` = `shikigami`; `CFBundlePackageType` = `APPL`; `CFBundleInfoDictionaryVersion` = `6.0`; `CFBundleShortVersionString` dan `CFBundleVersion` = `$APP_VERSION`; `LSMinimumSystemVersion` = `11.0`; `NSHighResolutionCapable` = `<true/>`. Header plist standar (`<?xml ...?>`, DOCTYPE Apple, `<plist version="1.0"><dict>...`).
   - `plutil -lint "$STAGE/Contents/Info.plist"` (gagal berarti `set -e` menghentikan; staging dibersihkan oleh trap).
   - Ikon: `./scripts/make-icon.sh "build/app-resources/icon-source.png" "$STAGE/Contents/Resources/${ICON_NAME}.icns"` (sumber default akan dibuat otomatis bila belum ada). Pastikan berkas hasilnya ada, kalau tidak `die`.
   - `codesign --force --deep -s - "$STAGE"`.
5. Promosikan ke tujuan secara atomik: `rm -rf "$APP_DIR"`; `mv "$STAGE" "$APP_DIR"`. Setelah dipindah, lakukan verifikasi akhir pada jalur final: `codesign --verify --deep --strict "$APP_DIR"` (pada kegagalan: `rm -rf "$APP_DIR"` lalu `echo "error: verifikasi codesign gagal" >&2; exit 1`, supaya tidak ada bundle setengah jadi). Catatan: tanda tangan ad-hoc tidak mengikat path, jadi `mv` setelah sign aman; verifikasi akhir di jalur final membuktikannya. Bila verifikasi di jalur final ternyata gagal akibat `mv`, ubah urutan: `mv` dulu lalu `codesign --force --deep -s -` pada jalur final, baru verify.
6. Cetak ringkasan: `plutil -extract CFBundleIdentifier raw "$APP_DIR/Contents/Info.plist"` untuk konfirmasi, lalu baris terakhir `echo "$(pwd)/$APP_DIR"`.
7. `chmod +x scripts/make-app.sh scripts/make-icon.sh`.

## Perubahan

- Baru: `scripts/make-icon.sh` (executable)
- Baru: `scripts/make-app.sh` (executable)
- Baru saat runtime (bukan di-commit, turunan): `build/app-resources/icon-source.svg`, `build/app-resources/icon-source.png`, `build/app-resources/shikigami.icns`
- Tidak ada perubahan file lain.

## Wiring

Fitur ini tidak menyambungkan apa pun sendiri. Yang diekspos untuk langkah wiring/fitur lain:
- `./scripts/make-app.sh` dipanggil oleh target Makefile `app` (`app: build` -> `VERSION=$(VERSION) ./scripts/make-app.sh`), milik `make-install-targets`.
- Artefak `build/Shikigami.app` dikonsumsi `scripts/install.sh install` (`ditto` ke `/Applications`).
- Pastikan keduanya executable (`chmod +x`), sesuai checklist wiring overview bagian 5 butir 3.

## Selesai bila

Guard: build path fitur ini sukses, tanpa unit test.
- [ ] `bash -n scripts/make-app.sh` dan `bash -n scripts/make-icon.sh` lulus.
- [ ] Setelah `make build` (atau biner ada di `build/bin/`), `./scripts/make-app.sh` exit 0 dan menghasilkan `build/Shikigami.app/Contents/{Info.plist,MacOS/{shikigami,shikigamid,shikigamictl},Resources/shikigami.icns}`, biner bermode 755, tidak ada biner lain di luar `Contents/MacOS/`.
- [ ] `plutil -lint build/Shikigami.app/Contents/Info.plist` OK; `CFBundleIdentifier` = `id.noobee.shikigami`; `VERSION` tak diset menghasilkan `0.0.0-dev`, `VERSION=1.2.3` menghasilkan `1.2.3`.
- [ ] `codesign --verify --deep --strict build/Shikigami.app` exit 0.
- [ ] `file build/Shikigami.app/Contents/Resources/shikigami.icns` melaporkan "Mac OS X icon".
- [ ] Dijalankan dua kali berturut-turut tetap sukses; setelah `rm -rf build/app-resources` (simulasi `make clean`) tetap sukses karena ikon sumber dibuat ulang.
- [ ] Verifikasi jalur fallback ikon: paksa jalur `qlmanage` gagal sementara (mis. dengan `PATH` tanpa `qlmanage` atau uji fungsi fallback terpisah) dan pastikan PNG 1024x1024 tetap dihasilkan.
- [ ] Menghapus sementara `build/bin/shikigamid` (pulihkan setelahnya) membuat skrip gagal dengan stderr `biner shikigamid belum dibuild; jalankan 'make build' dulu`, exit non-nol, dan bundle lama tidak rusak serta tidak ada `build/.Shikigami.app.tmp` tersisa.
- [ ] Kedua skrip executable. Tidak ada file di luar ownedPaths yang berubah.
