#!/usr/bin/env bash
# Membuat berkas .icns dari PNG sumber (1024x1024).
#
# Pemakaian: scripts/make-icon.sh [sumber.png] [keluaran.icns]
#
# Sumber ikon: assets/icon/shikigami.svg (di-commit). Skrip merendernya ke
# build/app-resources/icon-source.png (1024x1024, latar transparan) setiap kali
# PNG itu belum ada, ukurannya salah, atau SVG-nya lebih baru. 'make clean'
# menghapus build/, sehingga PNG dibuat ulang otomatis dari SVG.
# Untuk mengganti ikon, ubah assets/icon/shikigami.svg lalu jalankan 'make app'.
set -euo pipefail

cd "$(dirname "$0")/.."

ICON_SVG="assets/icon/shikigami.svg"
DEFAULT_SRC="build/app-resources/icon-source.png"
SRC="${1:-$DEFAULT_SRC}"
OUT="${2:-build/app-resources/shikigami.icns}"

# PNG kecil (48x48) tertanam, dipakai bila qlmanage tidak tersedia/gagal.
FALLBACK_PNG_B64="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"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: '$1' tidak ditemukan di PATH. $2" >&2
    exit 1
  fi
}

die() { echo "error: $*" >&2; exit 1; }

need sips "Perintah ini bawaan macOS."
need iconutil "Perintah ini bawaan macOS."

size_ok() {
  local w h
  w="$(sips -g pixelWidth "$1" 2>/dev/null | awk '/pixelWidth/ {print $2}')"
  h="$(sips -g pixelHeight "$1" 2>/dev/null | awk '/pixelHeight/ {print $2}')"
  [ "$w" = "1024" ] && [ "$h" = "1024" ]
}

generate_fallback() {
  local png="build/app-resources/icon-source.png"
  printf '%s' "$FALLBACK_PNG_B64" | base64 -D > "$png"
  sips -z 1024 1024 "$png" >/dev/null
}

generate_default_source() {
  local svg="build/app-resources/icon-source.svg" tmp ok=0
  mkdir -p build/app-resources
  [ -f "$ICON_SVG" ] || die "sumber ikon tidak ditemukan: $ICON_SVG"
  cp "$ICON_SVG" "$svg"
  # Render via Swift/AppKit agar latar transparan (qlmanage memberi latar putih opak).
  if command -v swift >/dev/null 2>&1; then
    tmp="$(mktemp -d)"
    if swift scripts/render-icon.swift "$svg" "$tmp/icon.png" >/dev/null 2>&1 \
       && size_ok "$tmp/icon.png"; then
      mv "$tmp/icon.png" build/app-resources/icon-source.png
      ok=1
    fi
    rm -rf "$tmp"
  fi
  if [ "$ok" -ne 1 ]; then
    generate_fallback
  fi
  echo "sumber ikon dibuat dari $ICON_SVG: build/app-resources/icon-source.png"
}

if [ "$SRC" = "$DEFAULT_SRC" ]; then
  if [ ! -f "$SRC" ] || ! size_ok "$SRC" || [ "$ICON_SVG" -nt "$SRC" ]; then
    generate_default_source
  fi
fi
[ -f "$SRC" ] || die "sumber ikon tidak ditemukan: $SRC"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
ICONSET="$WORK/shikigami.iconset"
mkdir -p "$ICONSET"

for s in 16 32 128 256 512; do
  d=$((s * 2))
  sips -z "$s" "$s" "$SRC" --out "$ICONSET/icon_${s}x${s}.png" >/dev/null
  sips -z "$d" "$d" "$SRC" --out "$ICONSET/icon_${s}x${s}@2x.png" >/dev/null
done

mkdir -p "$(dirname "$OUT")"
iconutil -c icns "$ICONSET" -o "$WORK/shikigami.icns"
cp "$WORK/shikigami.icns" "$OUT.tmp" && mv "$OUT.tmp" "$OUT"
echo "ikon dibuat: $OUT"
