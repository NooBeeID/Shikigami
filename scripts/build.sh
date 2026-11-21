#!/usr/bin/env bash
# Pembungkus `make build` dengan pemeriksaan prasyarat.
set -euo pipefail

cd "$(dirname "$0")/.."

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: '$1' tidak ditemukan di PATH. $2" >&2
    exit 1
  fi
}

need go "Pasang Go dari https://go.dev/dl/"
need npm "Pasang Node.js (termasuk npm) dari https://nodejs.org/"
need make "Pasang Xcode Command Line Tools: xcode-select --install"

make build
