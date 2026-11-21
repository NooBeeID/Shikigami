# Plan: Go module dan skrip build

## Metadata

- Tiket: `ai-terminal-ticket-01-app-base` (`docs/tickets/ai-terminal-ticket-01-app-base.md`)
- Fitur: `go-skeleton-build`
- Overview bersama: `docs/plans/ai-terminal-ticket-01-app-base/overview.md` (kontrak lintas fitur ada di sana; jangan diubah)
- ownedPaths: `go.mod`, `go.sum`, `Makefile`, `.gitignore`, `README.md`, `scripts/build.sh`, `internal/paths/`, `app/dist/.gitkeep`
- Bergantung pada foundation: YA. Foundation (`00-foundation.md`) sudah/akan membuat `go.mod` awal (`module aiterm` + Wails di `go.sum`), stub `internal/paths/paths.go`, dan `app/dist/.gitkeep`. Fitur ini MENYEMPURNAKAN file-file itu. Bila saat dimulai file tersebut belum ada (mis. Foundation belum dijalankan), buat sendiri dengan isi yang dijelaskan di bawah; hasil akhirnya sama.
- Working directory: root repo `/Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal`. Bukan git repo; jangan `git init` dan jangan commit.

## Tujuan

Menyediakan dasar build proyek: `go.mod` final (`module aiterm`), paket `internal/paths` penuh (DataDir dan SocketPath), `Makefile` dengan target `build`, `build-ui`, `build-daemon`, `build-app`, `run-daemon`, `clean`, `check`, pembungkus `scripts/build.sh`, `.gitignore`, `README.md` singkat, dan `app/dist/.gitkeep` agar `go build ./...` sukses sebelum frontend pernah di-build. Fitur ini satu-satunya pemilik `go.mod`/`go.sum`.

## Path yang dimiliki

HANYA path ini yang boleh dibuat atau diubah:

- `go.mod`, `go.sum`
- `Makefile`
- `.gitignore`
- `README.md`
- `scripts/build.sh`
- `internal/paths/` (`paths.go`; boleh menambah file lain di dalam folder ini bila perlu, tanpa unit test)
- `app/dist/.gitkeep`

Dilarang menyentuh: `internal/ipc/`, `cmd/**`, `app/main.go`, `app/bridge.go`, `wails.json`, `frontend/**`, `docs/**` (termasuk `overview.md`), atau membuat file epic lain (`wire_*.go`, `cmd_*.go`, `frontend/src/modules/`). Fitur paralel lain sedang mengerjakan path tersebut. Bila build paket lain gagal karena fitur lain belum selesai, itu bukan urusan fitur ini.

## Kontrak

Disediakan (signature dikunci di overview bagian 2; jangan diubah):

```go
// package paths  (import path: aiterm/internal/paths)
func DataDir() (string, error)    // ~/Library/Application Support/aiterm, atau $AITERM_DATA_DIR; dibuat 0700
func SocketPath() (string, error) // <DataDir>/aitermd.sock; error bila > 100 byte
```

- Env override: `AITERM_DATA_DIR` (bila terisi, dipakai sebagai DataDir).
- Nama berkas socket: `aitermd.sock`. Mode direktori 0700 (socket 0600 diurus `ipc-transport`, bukan di sini).
- Batas panjang `SocketPath`: lebih dari 100 byte menghasilkan error jelas berbahasa Indonesia.
- Output build (dipakai fitur/wiring lain): `build/bin/aitermd`, `build/bin/aitermctl`, `build/bin/aiterm`.
- Modul: `module aiterm`; dependensi `github.com/wailsapp/wails/v2` harus ada di `go.mod`/`go.sum`.
- Target Makefile `build-daemon` mengkompilasi `./cmd/aitermd` dan `./cmd/aitermctl` (milik `daemon-hello`); `build-ui` memakai `frontend/` (milik `wails-hello-ui`); `build-app` mengkompilasi `./app` (milik `wails-hello-ui`).
- Variabel versi daemon: `version` di `package main` cmd/aitermd, di-set via `-ldflags "-X main.version=..."` (variabel milik `daemon-hello`; Makefile di sini hanya memberi ldflags).

Dipakai: tidak ada kontrak dari fitur lain.

## Langkah

### 1. `go.mod` / `go.sum`

1. Jalankan `go version` (terpasang 1.26.3). Bila `go.mod` belum ada: `go mod init aiterm`. Pastikan baris pertama `module aiterm` dan baris `go` sesuai toolchain (`go 1.26` atau yang dihasilkan `go mod init`; jangan menurunkan versinya).
2. Pastikan dependensi Wails ada: bila `go.mod` belum memuat `github.com/wailsapp/wails/v2`, jalankan `go get github.com/wailsapp/wails/v2@latest`.
3. JANGAN jalankan `go mod tidy` di fitur ini. Alasan: belum ada import Wails (`app/*.go` belum ada, ditulis fitur lain secara paralel) sehingga tidy akan membuang Wails dari `go.mod`/`go.sum`. `go mod tidy` dijalankan oleh langkah wiring setelah semua fitur selesai. (Pengecualian: lihat langkah verifikasi 8, tidak ada.)
4. Verifikasi: `grep -q '^module aiterm$' go.mod` dan `grep -q 'wailsapp/wails/v2' go.mod go.sum`.

### 2. `internal/paths/paths.go`

`package paths`. Tulis ulang/timpa stub Foundation dengan implementasi penuh. Hanya stdlib (`errors`/`fmt`, `os`, `path/filepath`).

```go
// Package paths menentukan lokasi data dan socket daemon aiterm.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// EnvDataDir adalah env var untuk meng-override DataDir (dev/test).
	EnvDataDir = "AITERM_DATA_DIR"
	// socketName adalah nama berkas unix socket di dalam DataDir.
	socketName = "aitermd.sock"
	// maxSocketPath: batas path unix socket macOS ~104 byte; sisakan margin.
	maxSocketPath = 100
)

func DataDir() (string, error)
func SocketPath() (string, error)
```

Perilaku `DataDir()`:
- Bila `os.Getenv("AITERM_DATA_DIR")` tidak kosong: `dir = filepath.Clean(nilai)`; bila bukan path absolut, ubah dengan `filepath.Abs` (agar socket path konsisten). Selain itu: `home, err := os.UserHomeDir()`; bila error, kembalikan `fmt.Errorf("tidak bisa menentukan direktori home: %w", err)`; `dir = filepath.Join(home, "Library", "Application Support", "aiterm")`.
- `os.MkdirAll(dir, 0o700)`; bila error: `fmt.Errorf("gagal membuat direktori data %s: %w", dir, err)`.
- Pastikan mode 0700 juga untuk direktori yang sudah ada tanpa mengubah milik orang lain secara agresif: setelah MkdirAll, `os.Chmod(dir, 0o700)` hanya bila `Stat` menunjukkan `Mode().Perm() != 0o700`; error chmod dikembalikan dengan pesan Indonesia. (Catatan: `MkdirAll` terkena umask; chmod memastikan 0700 persis pada direktori yang baru dibuat.)
- Kembalikan `dir, nil`.

Perilaku `SocketPath()`:
- `dir, err := DataDir()`; teruskan error.
- `p := filepath.Join(dir, socketName)`.
- Bila `len(p) > maxSocketPath` (hitung byte, `len` string): kembalikan `"", fmt.Errorf("path socket terlalu panjang (%d byte, maks %d): %s; pendekkan AITERM_DATA_DIR", len(p), maxSocketPath, p)`.
- Kembalikan `p, nil`.

Fungsi tidak boleh `panic`. Tidak ada state global selain konstanta.

### 3. `app/dist/.gitkeep`

Pastikan folder `app/dist/` ada dan berkas `app/dist/.gitkeep` ada (kosong, 0 byte). Bila sudah ada dari Foundation, biarkan. Alasan: `//go:embed all:dist` di `app/main.go` (fitur lain) harus valid sebelum frontend di-build.

### 4. `.gitignore`

Isi persis:

```
# Artefak build
build/
frontend/node_modules/
frontend/dist/

# Aset embed app (disalin dari frontend/dist saat build-app), kecuali .gitkeep
app/dist/*
!app/dist/.gitkeep

# Lain-lain
.DS_Store
*.sock
```

### 5. `Makefile`

Gunakan TAB untuk indentasi resep. `SHELL := /bin/bash`. Semua target selain file ditandai `.PHONY`. Isi:

```make
SHELL := /bin/bash
VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build build-ui build-daemon build-app run-daemon clean check

build: build-ui build-daemon build-app

build-ui:
	cd frontend && npm install && npm run build

build-daemon:
	mkdir -p build/bin
	go build -ldflags "$(LDFLAGS)" -o build/bin/aitermd ./cmd/aitermd
	go build -ldflags "$(LDFLAGS)" -o build/bin/aitermctl ./cmd/aitermctl

build-app:
	mkdir -p build/bin app/dist
	find app/dist -mindepth 1 ! -name .gitkeep -delete
	cp -R frontend/dist/. app/dist/
	go build -tags desktop,production -o build/bin/aiterm ./app

run-daemon: build-daemon
	./build/bin/aitermd

clean:
	rm -rf build frontend/dist
	find app/dist -mindepth 1 ! -name .gitkeep -delete

check:
	go build ./...
```

Catatan wajib:
- `build` bergantung berurutan pada `build-ui`, `build-daemon`, `build-app` (jangan pakai `-j`; bila ingin pasti urut, tulis `build:` resep yang memanggil `$(MAKE) build-ui`, `$(MAKE) build-daemon`, `$(MAKE) build-app` secara berurutan sebagai pengganti prerequisites). Pilih bentuk resep `$(MAKE)` berurutan agar urutan terjamin juga di bawah `make -j`.
- `build-app`: `.gitkeep` TIDAK boleh terhapus (karena `find ... ! -name .gitkeep`); bila `frontend/dist` belum ada, `cp` gagal dengan pesan jelas; tambahkan pemeriksaan sebelum `cp`: `test -d frontend/dist || { echo "frontend/dist belum ada; jalankan 'make build-ui' dulu" >&2; exit 1; }`.
- `clean` tidak menghapus `frontend/node_modules`.
- `ldflags` `-X main.version` aman walau variabel `version` belum ada (linker mengabaikan simbol yang tak ada).

### 6. `scripts/build.sh`

File dapat dieksekusi (`chmod +x scripts/build.sh`). Isi:

```bash
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
```

### 7. `README.md`

Bahasa Indonesia, singkat. Bagian wajib:
- Judul `# aiterm` dan satu paragraf: apa ini (terminal berbasis AI untuk macOS: daemon Go `aitermd`, CLI debug `aitermctl`, app Wails + frontend Svelte; saat ini skeleton Hello World).
- `## Prasyarat`: Go (versi di `go.mod`), Node.js + npm, Xcode Command Line Tools (cgo/WebKit untuk `build-app`). Wails CLI tidak wajib untuk build (`build-app` memakai `go build -tags desktop,production`).
- `## Build`: `make build` atau `scripts/build.sh`; hasil di `build/bin/` (`aitermd`, `aitermctl`, `aiterm`). Target lain: `build-ui`, `build-daemon`, `build-app`, `run-daemon`, `clean`, `check` (satu baris per target).
- `## Menjalankan daemon`: `build/bin/aitermd` (atau `make run-daemon`); catatan `AITERM_DATA_DIR` untuk override direktori data (default `~/Library/Application Support/aiterm`; socket `aitermd.sock`, path socket maks 100 byte).
- `## Mencoba CLI`: `build/bin/aitermctl ping` (mencetak `pong`), `build/bin/aitermctl info`, `build/bin/aitermctl help`.
- `## Menjalankan app`: `open`/jalankan `build/bin/aiterm` setelah daemon hidup; tanpa daemon window menampilkan "daemon tidak berjalan".
- `## Struktur` (opsional, ringkas): `cmd/`, `internal/`, `app/`, `frontend/`, `scripts/`.

### 8. Verifikasi lokal (guard fitur ini, tanpa unit test)

Jalankan berurutan dan perbaiki sampai lolos:
1. `gofmt -l internal/paths` tidak mencetak apa pun.
2. `go build ./internal/paths/...` sukses.
3. `go vet ./internal/paths/...` sukses.
4. `bash -n scripts/build.sh` sukses; `test -x scripts/build.sh`.
5. `make -n build-daemon build-app clean check run-daemon` (dry run) tidak error sintaks Makefile; pastikan resep memakai TAB.
6. Smoke manual `paths` TANPA menambah file di luar ownedPaths: jalankan satu kali dengan program sementara di scratchpad/di luar repo, atau `go run` dari file sementara di luar repo yang meng-import `aiterm/internal/paths` bila memungkinkan; set `AITERM_DATA_DIR` ke direktori sementara (mis. `$(mktemp -d)/d`), cek direktori terbuat dengan mode `drwx------`, cek `SocketPath()` berakhir `aitermd.sock`, dan cek `AITERM_DATA_DIR` berpath sangat panjang (>100 byte total) menghasilkan error. Hapus file/direktori sementara sesudahnya. Bila tidak praktis (module `aiterm` harus di dalam repo), lewati dan cukup review kode.
7. `grep -q '^module aiterm$' go.mod` dan `grep -q 'wailsapp/wails/v2' go.sum`.
8. `ls app/dist/.gitkeep` ada.
9. `make build-daemon` HANYA dicoba bila `cmd/aitermd` dan `cmd/aitermctl` sudah berisi `main.go` dari `daemon-hello`; bila belum atau gagal karena fitur lain setengah jadi, abaikan (bukan kegagalan fitur ini).

## Perubahan

Dibuat/diubah:
- `go.mod` (dibuat bila belum ada / dipastikan: `module aiterm`, `go` sesuai toolchain, require Wails)
- `go.sum` (hasil `go get` Wails; tanpa `go mod tidy`)
- `internal/paths/paths.go` (menimpa stub Foundation dengan implementasi penuh)
- `Makefile` (baru)
- `scripts/build.sh` (baru, executable)
- `.gitignore` (baru)
- `README.md` (baru)
- `app/dist/.gitkeep` (dipastikan ada, kosong)

Tidak ada file lain yang boleh berubah.

## Wiring

Fitur ini mengekspos, tanpa menyambungkannya sendiri:
- `aiterm/internal/paths.DataDir()` dan `SocketPath()` untuk `cmd/aitermd/main.go` dan `cmd/aitermctl/main.go` (milik `daemon-hello`) dan `app/bridge.go` (milik `wails-hello-ui`).
- Target Makefile `build-ui`, `build-daemon`, `build-app`, `build`, `check`: memanggil path `./cmd/aitermd`, `./cmd/aitermctl`, `./app`, `frontend/` milik fitur lain; baru lulus penuh setelah semuanya ada.
- Untuk langkah wiring akhir (dikerjakan agent lain, bukan di sini): `go mod tidy` (membersihkan `go.sum`, memverifikasi Wails tetap ada setelah `app/*.go` meng-import-nya), cek tidak ada sisa stub `internal/ipc/stub.go`, lalu `make build` dan smoke test sesuai overview bagian 5.

## Selesai bila

- Guard fitur: build path fitur ini sukses (`go build ./internal/paths/...`), `go vet ./internal/paths/...` bersih, `bash -n scripts/build.sh` lolos. Tanpa unit test.
- `go.mod` memuat `module aiterm` dan `github.com/wailsapp/wails/v2`; `go.sum` memuat Wails.
- `internal/paths` sesuai Kontrak: DataDir default `~/Library/Application Support/aiterm`, override `AITERM_DATA_DIR`, dibuat 0700; `SocketPath` = `<DataDir>/aitermd.sock` dengan error jelas (Indonesia) bila > 100 byte.
- `Makefile` punya target `build` (urut `build-ui`, `build-daemon`, `build-app`), `build-ui`, `build-daemon`, `build-app` (menyalin `frontend/dist/` ke `app/dist/` tanpa menghapus `.gitkeep`, lalu `go build -tags desktop,production -o build/bin/aiterm ./app`), `run-daemon`, `clean`, `check`; `make -n` tanpa error sintaks.
- `scripts/build.sh` executable, `set -euo pipefail`, cek `go` dan `npm`, pesan error jelas.
- `.gitignore` dan `README.md` ada dan sesuai isi di atas; `app/dist/.gitkeep` ada.
- Tidak ada file di luar ownedPaths yang dibuat atau diubah; file sementara dibersihkan.
