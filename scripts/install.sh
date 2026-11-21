#!/usr/bin/env bash
# Pemasangan Shikigami.app ke /Applications. Pemakaian: install.sh <install|uninstall|link-cli>
set -euo pipefail
cd "$(dirname "$0")/.."

APP_NAME="Shikigami"
INSTALL_DIR="/Applications"
APP_DEST="$INSTALL_DIR/$APP_NAME.app"
LINK_PATH="/usr/local/bin/shikigamictl"
DATA_DIR="$HOME/Library/Application Support/shikigami"   # hanya dicetak, tidak pernah dihapus/diubah
SRC_APP="build/Shikigami.app"

die() {
  echo "error: $*" >&2
  exit 1
}

# Tutup aplikasi bila sedang berjalan.
quit_app() {
  osascript -e 'tell application "Shikigami" to quit' >/dev/null 2>&1 || true
  local i
  if pgrep -x shikigami >/dev/null 2>&1; then
    sleep 1
    for i in 1 2 3 4 5 6; do
      pgrep -x shikigami >/dev/null 2>&1 || break
      sleep 0.5
    done
  fi
}

# Hentikan daemon lama (cmd/shikigamictl tidak punya perintah shutdown).
stop_daemon() {
  local i
  pkill -x shikigamid >/dev/null 2>&1 || true
  for i in 1 2 3 4 5 6; do
    pgrep -x shikigamid >/dev/null 2>&1 || return 0
    sleep 0.5
  done
  echo "peringatan: shikigamid masih berjalan setelah ~3 detik" >&2
}

require_writable() {
  if [ ! -w "$INSTALL_DIR" ]; then
    die "$INSTALL_DIR tidak bisa ditulis; jalankan dengan hak yang cukup, mis. tambahkan pengguna ke grup admin atau salin manual"
  fi
}

cmd_install() {
  [ -d "$SRC_APP" ] || die "build/Shikigami.app belum ada; jalankan 'make app' dulu"
  require_writable
  quit_app
  stop_daemon
  rm -rf "$APP_DEST"
  ditto "$SRC_APP" "$APP_DEST"
  codesign --verify --deep --strict "$APP_DEST" || die "verifikasi codesign gagal untuk $APP_DEST"
  echo "terpasang: $APP_DEST"
  echo "jalankan dengan: open -a $APP_NAME"
}

cmd_uninstall() {
  quit_app
  stop_daemon
  if [ -e "$APP_DEST" ]; then
    [ -w "$INSTALL_DIR" ] || die "$INSTALL_DIR tidak bisa ditulis; tidak dapat menghapus $APP_DEST tanpa hak yang cukup"
    rm -rf "$APP_DEST"
    echo "dihapus: $APP_DEST"
  else
    echo "belum terpasang: $APP_DEST"
  fi
  if [ -L "$LINK_PATH" ]; then
    local tgt
    tgt="$(readlink "$LINK_PATH" || true)"
    case "$tgt" in
      *Shikigami.app*|*/build/bin/shikigamictl)
        if rm -f "$LINK_PATH" 2>/dev/null; then
          echo "symlink dihapus: $LINK_PATH"
        else
          echo "peringatan: gagal menghapus $LINK_PATH; hapus manual dengan: sudo rm $LINK_PATH" >&2
        fi
        ;;
    esac
  fi
  echo "data dipertahankan: $DATA_DIR"
}

cmd_link_cli() {
  local target
  if [ -x "$APP_DEST/Contents/MacOS/shikigamictl" ]; then
    target="$APP_DEST/Contents/MacOS/shikigamictl"
  elif [ -x "$(pwd)/build/bin/shikigamictl" ]; then
    target="$(pwd)/build/bin/shikigamictl"
  else
    die "shikigamictl belum ada; jalankan 'make build' atau 'make install' dulu"
  fi
  if [ ! -d /usr/local/bin ] || [ ! -w /usr/local/bin ]; then
    echo "/usr/local/bin tidak ada atau tidak bisa ditulis; jalankan manual:" >&2
    echo "  sudo mkdir -p /usr/local/bin && sudo ln -sf \"$target\" $LINK_PATH" >&2
    exit 1
  fi
  ln -sf "$target" "$LINK_PATH"
  echo "symlink: $LINK_PATH -> $target"
}

case "${1:-}" in
  install) cmd_install ;;
  uninstall) cmd_uninstall ;;
  link-cli) cmd_link_cli ;;
  *)
    echo "pemakaian: $0 <install|uninstall|link-cli>" >&2
    exit 2
    ;;
esac
