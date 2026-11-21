# aiterm

Terminal berbasis AI untuk macOS. Terdiri dari daemon Go `aitermd`, CLI debug `aitermctl`, dan app Wails dengan frontend Svelte. Saat ini masih skeleton Hello World.

## Prasyarat

- Go (versi sesuai `go.mod`)
- Node.js dan npm
- Xcode Command Line Tools (cgo/WebKit untuk `build-app`)

Wails CLI tidak wajib; `build-app` memakai `go build -tags desktop,production`.

## Build

```
make build
# atau
scripts/build.sh
```

Hasil ada di `build/bin/` (`aitermd`, `aitermctl`, `aiterm`). Target lain:

- `make build-ui`: build frontend (`frontend/dist`)
- `make build-daemon`: build `aitermd` dan `aitermctl`
- `make build-app`: salin `frontend/dist` ke `app/dist` lalu build app Wails
- `make run-daemon`: build dan jalankan daemon
- `make clean`: hapus artefak build
- `make check`: `go build ./...`

## Menjalankan daemon

```
build/bin/aitermd
# atau
make run-daemon
```

Direktori data default `~/Library/Application Support/aiterm`; override dengan `AITERM_DATA_DIR`. Socket bernama `aitermd.sock` di dalamnya (path socket maksimal 100 byte).

## Mencoba CLI

```
build/bin/aitermctl ping   # mencetak pong
build/bin/aitermctl info
build/bin/aitermctl help
```

## Menjalankan app

Jalankan `build/bin/aiterm` setelah daemon hidup. Tanpa daemon, window menampilkan "daemon tidak berjalan".

## Struktur

- `cmd/`: `aitermd` dan `aitermctl`
- `internal/`: paket internal (`paths`, `ipc`)
- `app/`: app Wails
- `frontend/`: UI Svelte
- `scripts/`: skrip build
