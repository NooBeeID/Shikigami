# 00 Foundation: Epic 01 App Base

Baca dulu `overview.md` di folder ini. Foundation dikerjakan SEBELUM fitur paralel. Tujuan: repo punya `go.mod`, dependensi Wails di `go.sum`, dan stub yang kompilasi sehingga empat fitur bisa berjalan paralel.

Working directory: root repo `/Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal`. Bukan git repo; jangan `git init` atau commit.

## Langkah

### 1. Struktur direktori
Buat: `cmd/aitermd`, `cmd/aitermctl`, `app/dist`, `frontend`, `scripts`, `internal/paths`, `internal/ipc`.

### 2. `go.mod`
- Jalankan `go mod init aiterm`; pastikan baris `go` sesuai toolchain (`go version`: 1.26.3, jadi `go 1.26` atau versi yang dihasilkan `go mod init`).
- Jalankan `go get github.com/wailsapp/wails/v2@latest` agar `go.sum` memuat Wails. (Tanpa import, `go mod tidy` akan membuangnya; itu tidak masalah, tetapi JANGAN jalankan `go mod tidy` di Foundation. Wiring yang menjalankannya.)

### 3. Stub `internal/paths/paths.go`
`package paths` dengan signature Kontrak, implementasi nyata minimal (sudah cukup kecil):
- `DataDir()`: bila env `AITERM_DATA_DIR` terisi pakai itu; selain itu `os.UserHomeDir()` + `Library/Application Support/aiterm`; `os.MkdirAll(dir, 0o700)`.
- `SocketPath()`: `filepath.Join(DataDir, "aitermd.sock")`; bila panjang > 100 byte, kembalikan error jelas.
`go-skeleton-build` akan menyempurnakan/menimpa file ini.

### 4. Stub `internal/ipc/stub.go`
`package ipc` dengan seluruh signature Kontrak (lihat overview bagian 2), badan fungsi `panic("belum diimplementasi")` (atau nilai nol). Wajib ada:
- Tipe: `Handler`, `Error` (dengan method `Error() string`), `Server`, `Client`.
- Fungsi: `NewError`, `Errorf`, `NewServer`, `Dial`.
- Method `Server`: `Register`, `Publish`, `Serve`, `Methods`, `SetInfo`. Method `Client`: `Call`, `Subscribe`, `Close`.
- Konstanta: `ErrBinary`, `ErrConflict`, `ErrNotFound`, `ErrInvalid`, `ErrParse`, `ErrInvalidRequest`, `ErrMethodNotFound`, `ErrInvalidParams`, `ErrInternal`.
`ipc-transport` MENGHAPUS `stub.go` dan mengganti dengan `server.go`, `client.go`, `errors.go`, `protocol.go`, `builtin.go`.

### 5. `app/dist/.gitkeep`
File kosong, agar `//go:embed all:dist` valid sebelum frontend di-build.

### 6. Verifikasi
- `go build ./internal/...` sukses.
- `go vet ./internal/...` sukses.
- `ls app/dist/.gitkeep` ada.
- Pastikan `go.sum` mengandung `github.com/wailsapp/wails/v2`.

## Di luar Foundation
Makefile, `.gitignore`, README, `scripts/build.sh`, `cmd/*`, `app/*.go`, `wails.json`, `frontend/*` dikerjakan fitur masing-masing.
