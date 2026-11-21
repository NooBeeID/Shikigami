#!/usr/bin/env bash
# Merakit build/Shikigami.app dari build/bin/{shikigami,shikigamid,shikigamictl}.
# Tata letak datar: ketiga biner di Contents/MacOS/ (autostart mencari
# shikigamid di folder yang sama dengan biner app). Versi dibaca dari env VERSION
# (default 'dev' -> 0.0.0-dev). Bundle ditandatangani ad-hoc.
set -euo pipefail

cd "$(dirname "$0")/.."

APP_NAME="Shikigami"
BUNDLE_ID="id.noobee.shikigami"
EXEC_NAME="shikigami"
ICON_NAME="shikigami"
MIN_OS="11.0"
BIN_DIR="build/bin"
APP_DIR="build/${APP_NAME}.app"
BINARIES=(shikigami shikigamid shikigamictl)

VERSION="${VERSION:-dev}"
if [ "$VERSION" = "dev" ]; then APP_VERSION="0.0.0-dev"; else APP_VERSION="$VERSION"; fi
APP_VERSION="${APP_VERSION//&/&amp;}"
APP_VERSION="${APP_VERSION//</&lt;}"
APP_VERSION="${APP_VERSION//>/&gt;}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: '$1' tidak ditemukan di PATH. $2" >&2
    exit 1
  fi
}

for n in "${BINARIES[@]}"; do
  if [ ! -f "$BIN_DIR/$n" ] || [ ! -x "$BIN_DIR/$n" ]; then
    echo "biner $n belum dibuild; jalankan 'make build' dulu" >&2
    exit 1
  fi
done
for c in codesign plutil sips iconutil; do need "$c" "Perintah ini bawaan macOS."; done

STAGE="build/.${APP_NAME}.app.tmp"
rm -rf "$STAGE"
trap 'rm -rf "$STAGE"' EXIT

mkdir -p "$STAGE/Contents/MacOS" "$STAGE/Contents/Resources"
for n in "${BINARIES[@]}"; do
  install -m 755 "$BIN_DIR/$n" "$STAGE/Contents/MacOS/$n"
done

cat > "$STAGE/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>${APP_NAME}</string>
  <key>CFBundleDisplayName</key><string>${APP_NAME}</string>
  <key>CFBundleIdentifier</key><string>${BUNDLE_ID}</string>
  <key>CFBundleExecutable</key><string>${EXEC_NAME}</string>
  <key>CFBundleIconFile</key><string>${ICON_NAME}</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
  <key>CFBundleShortVersionString</key><string>${APP_VERSION}</string>
  <key>CFBundleVersion</key><string>${APP_VERSION}</string>
  <key>LSMinimumSystemVersion</key><string>${MIN_OS}</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST
plutil -lint "$STAGE/Contents/Info.plist" >/dev/null

./scripts/make-icon.sh "build/app-resources/icon-source.png" "$STAGE/Contents/Resources/${ICON_NAME}.icns"
if [ ! -f "$STAGE/Contents/Resources/${ICON_NAME}.icns" ]; then
  echo "error: ikon gagal dibuat" >&2
  exit 1
fi

codesign --force --deep -s - "$STAGE" >/dev/null 2>&1

rm -rf "$APP_DIR"
mv "$STAGE" "$APP_DIR"

if ! codesign --verify --deep --strict "$APP_DIR" 2>/dev/null; then
  rm -rf "$APP_DIR"
  echo "error: verifikasi codesign gagal" >&2
  exit 1
fi

echo "bundle id: $(plutil -extract CFBundleIdentifier raw "$APP_DIR/Contents/Info.plist")"
echo "$(pwd)/$APP_DIR"
