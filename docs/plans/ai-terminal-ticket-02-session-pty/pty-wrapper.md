# Plan: Antarmuka PTY tipis

## Metadata

- Tiket: `ai-terminal-ticket-02-session-pty` (`docs/tickets/ai-terminal-ticket-02-session-pty.md`)
- Fitur: `pty-wrapper` (Antarmuka PTY tipis)
- ownedPaths: `internal/session/pty/`
- Depends on foundation: ya. Foundation sudah memasang `github.com/creack/pty` di `go.mod`/`go.sum` dan membuat `internal/session/pty/pty.go` (interface final) serta `internal/session/pty/pty_unix.go` (stub `New()`). Plan ini menimpa stub itu dengan implementasi nyata.
- Rujukan bersama: `docs/plans/ai-terminal-ticket-02-session-pty/overview.md` (bagian 2 Kontrak, bagian 6 Build guard, bagian 7 Aturan paralel). Jangan diubah.

## Tujuan

Menyediakan pembungkus PTY tipis untuk macOS/Unix di paket `aiterm/internal/session/pty`: interface `PTY` (`Start, Read, Write, Resize, Close, Wait`), tipe `Size`, dan satu implementasi berbasis `creack/pty` yang dibuat lewat `New()`. Tidak ada goroutine di paket ini (pembacaan dilakukan pemanggil, yaitu `session-manager`), dan tidak ada tipe `creack/pty` yang bocor ke luar paket, sehingga port ConPTY nanti cukup mengganti satu implementasi.

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `internal/session/pty/pty.go`
- `internal/session/pty/pty_unix.go`

Dilarang menyentuh: file lain di `internal/session/` (`types.go`, `launcher.go`, `scrollback.go`, `manager.go`, `ipc.go`), `cmd/`, `frontend/`, `docs/` (termasuk overview), `internal/ipc/`, `internal/paths/`, `go.mod`, `go.sum`. Dilarang menjalankan `go get`, `go mod tidy`, atau menambah dependency apa pun. Tidak ada file wiring.

## Kontrak

Disediakan (signature final, tidak boleh berubah; sama persis dengan overview bagian 2):
```go
package pty

type Size struct{ Cols, Rows uint16 }

type PTY interface {
    Start(cmd string, args []string, env []string, cwd string, size Size) error
    Read(p []byte) (int, error)
    Write(p []byte) (int, error)
    Resize(size Size) error
    Close() error                    // idempoten
    Wait() (exitCode int, err error) // blok sampai proses selesai
}

func New() PTY
```

Perilaku yang dijanjikan ke konsumen (`session-manager`):
- `Read` mengembalikan error (umumnya `io.EOF` atau `EIO` di macOS/Linux) setelah proses anak selesai atau master fd ditutup; pemanggil memperlakukan error apa pun sebagai akhir aliran.
- `Wait` memblok sampai proses selesai; exit code normal = kode proses, bila di-signal = `128+signal`. `Wait` tidak menutup master fd (itu tugas `Close`).
- `Close` aman dipanggil berulang dan dari goroutine berbeda dengan `Read`/`Wait`; tidak memblok selamanya; tidak panic. `Close` membuat `Read` yang sedang menunggu berhenti dengan error.
- Pemakaian yang diharapkan manager: satu goroutine memanggil `Read` berulang; saat error, memanggil `Wait`, lalu `Close`. `Kill` di manager memanggil `Close` (SIGHUP ke process group + tutup master fd). Paket ini TIDAK mengekspos PID atau method kill tambahan (interface dikunci) dan tidak melakukan SIGKILL; lihat Langkah 4 (`Close`).

Dipakai: hanya `github.com/creack/pty` (diimpor dengan alias `cpty`) dan stdlib. Tidak memakai `internal/ipc` atau paket `session`.

## Langkah

### 1. Prasyarat
- Pastikan `internal/session/pty/pty.go` dan `internal/session/pty/pty_unix.go` ada dari Foundation, dan `go.mod` memuat `github.com/creack/pty`. Bila `go.mod` tidak memuatnya, hentikan dan laporkan (jangan menjalankan `go get`).

### 2. `internal/session/pty/pty.go`
Isi sudah final dari Foundation. Hanya rapikan: tambahkan komentar dokumentasi bahasa Indonesia pada paket, `Size`, dan tiap metode interface (mis. "Close idempoten", "Wait memblok", "Read mengembalikan error saat proses selesai"). JANGAN mengubah nama, urutan parameter, atau tipe. Tidak ada import `creack/pty` di file ini.

### 3. `internal/session/pty/pty_unix.go`
Timpa stub dengan implementasi berikut. Build tag: `//go:build unix` (atau `darwin || linux`) di baris pertama agar port Windows nanti menambah file lain.

Import:
```go
import (
    "errors"
    "fmt"
    "os"
    "os/exec"
    "sync"
    "syscall"

    cpty "github.com/creack/pty"
)
```

Tipe (privat, tidak diekspor):
```go
type unixPTY struct {
    mu     sync.Mutex // melindungi cmd, master, closed
    cmd    *exec.Cmd
    master *os.File
    closed bool
}

func New() PTY { return &unixPTY{} }
```

### 4. Metode `unixPTY`

- `Start(cmd string, args []string, env []string, cwd string, size Size) error`
  - Kunci `mu`. Bila `p.cmd != nil` kembalikan `errors.New("pty sudah dimulai")`.
  - `c := exec.Command(cmd, args...)`; `c.Env = env`; `c.Dir = cwd` (kosong berarti mewarisi cwd proses).
  - Setsid/Setctty: `cpty.StartWithSize` (lewat `StartWithAttrs`) mengisi `SysProcAttr{Setsid: true, Setctty: true}` sendiri bila `c.SysProcAttr` masih nil. Periksa sumber versi terpasang di module cache (`go list -m -f '{{.Dir}}' github.com/creack/pty`). Bila perilaku itu benar, biarkan `c.SysProcAttr` nil; bila tidak, set `&syscall.SysProcAttr{Setsid: true, Setctty: true}` eksplisit. Hasil akhir wajib: proses anak menjadi session leader dengan controlling TTY, sehingga PID == PGID. Jangan menambah `Ctty` manual kecuali library mensyaratkannya.
  - `m, err := cpty.StartWithSize(c, &cpty.Winsize{Cols: size.Cols, Rows: size.Rows})`. Bila err, kembalikan `err` (dibungkus `fmt.Errorf`, tambah import `fmt`, pesan Indonesia mis. "gagal memulai pty: %w"). Jangan menyimpan state bila gagal.
  - Simpan `p.cmd = c`, `p.master = m`.
  - Size nol tidak divalidasi di sini (manager menjamin default 80x24).

- `Read(p []byte) (int, error)`
  - Ambil `master` di bawah `mu` (salin pointer, lepas lock), lalu `master.Read(p)`. Jangan menahan `mu` selama `Read` (akan memblok `Close`). Bila `master == nil` (belum Start) kembalikan `0, errors.New("pty belum dimulai")`.

- `Write(p []byte) (int, error)`
  - Sama pola dengan `Read`: salin pointer di bawah lock, `master.Write(p)` di luar lock. Belum Start -> error.

- `Resize(size Size) error`
  - Ambil `master` di bawah lock; bila nil atau sudah `closed`, kembalikan error ("pty tidak aktif"). Panggil `cpty.Setsize(master, &cpty.Winsize{Cols: size.Cols, Rows: size.Rows})` dan kembalikan hasilnya.

- `Close() error` (idempoten)
  - Kunci `mu`. Bila `p.closed` atau `p.cmd == nil`, set `closed = true` bila perlu dan `return nil`.
  - Set `p.closed = true`. Ambil `pid := p.cmd.Process.Pid` dan `master := p.master`. Lepas lock.
  - Kirim SIGHUP ke process group: `_ = syscall.Kill(-pid, syscall.SIGHUP)` (abaikan `ESRCH`/error karena proses mungkin sudah selesai). Karena `Setsid`, PGID == PID.
  - Tutup master fd: `err := master.Close()`; abaikan `os.ErrClosed`. Menutup master juga memberi SIGHUP kernel ke session leader dan membuat `Read` yang sedang memblok berhenti.
  - Keputusan: `Close` hanya SIGHUP ke process group + tutup master fd. Tidak ada eskalasi SIGKILL, tidak ada goroutine/timer di paket ini, dan interface tidak boleh ditambah metode. Eskalasi SIGKILL setelah 2 detik (bila ada) adalah urusan `session-manager`, bukan paket ini.
  - Kembalikan `nil` (atau error `master.Close()` selain `os.ErrClosed`).

- `Wait() (exitCode int, err error)`
  - Ambil `cmd` di bawah lock; bila nil kembalikan `-1, errors.New("pty belum dimulai")`. Panggil `werr := cmd.Wait()` di luar lock.
  - Tentukan kode keluar dari `cmd.ProcessState` (non-nil setelah `Wait` selesai, juga saat `werr` adalah `*exec.ExitError`):
    - `ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)`; bila `ok && ws.Signaled()` -> `exitCode = 128 + int(ws.Signal())`; selain itu `exitCode = cmd.ProcessState.ExitCode()`.
  - Error: bila `werr` bertipe `*exec.ExitError` (exit non-nol atau signal), itu BUKAN error `Wait`; kembalikan `exitCode, nil`. Bila `werr` error lain (mis. `cmd.ProcessState == nil`), kembalikan `-1, werr`.
  - `Wait` tidak boleh menutup master fd.
  - Catatan: `exec.Cmd.Wait` hanya boleh dipanggil sekali; manager memanggilnya sekali per sesi. Dokumentasikan di komentar.

### 5. Kebersihan paket
- Tidak ada goroutine (`go ...`), tidak ada `time.Ticker`/`time.After`, tidak ada channel di paket ini.
- Tidak ada ekspor selain `Size`, `PTY`, `New`. Tipe `unixPTY` privat. Tidak ada tipe `cpty.*` di signature mana pun yang diekspor.
- Jalankan `gofmt -l internal/session/pty/` (harus kosong).

### 6. Guard lokal
- `go build ./internal/session/pty/...` harus sukses.
- `go vet ./internal/session/pty/...` (disarankan).
- `grep -rn "creack/pty" --include=*.go .` hanya boleh cocok di `internal/session/pty/pty_unix.go`.
- `grep -n "go func\|go [a-zA-Z]*(" internal/session/pty/*.go` tidak boleh menemukan goroutine.
- Smoke manual opsional (jangan tinggalkan file): program sementara di luar repo (scratchpad) yang memanggil `New().Start("/bin/sh", []string{"-c","echo hi; exit 3"}, os.Environ(), "", Size{80,24})`, membaca sampai error, `Wait` -> 3, `Close` dua kali -> keduanya nil. Hapus setelah selesai. Jangan membuat `_test.go`.

## Perubahan

- Diubah: `internal/session/pty/pty.go` (hanya komentar; signature tetap).
- Diubah (stub ditimpa): `internal/session/pty/pty_unix.go` (implementasi `unixPTY` + `New()`).
- Tidak ada file lain dibuat atau diubah. Tidak ada perubahan `go.mod`/`go.sum`.

## Wiring

Tidak ada yang disambungkan oleh fitur ini. Yang diekspos untuk konsumen: `pty.Size`, `pty.PTY`, `pty.New()` di paket `aiterm/internal/session/pty`. `session-manager` mengimpornya (`aiterm/internal/session/pty`, tanpa alias) dan memanggil `pty.New()` per sesi. Langkah Wiring epic (overview bagian 5, butir 1) kelak memverifikasi bahwa `pty_unix.go` bukan stub lagi dan bahwa `creack/pty` hanya diimpor di paket ini; fitur ini tidak mengerjakannya.

## Selesai bila

- [ ] `go build ./internal/session/pty/...` sukses (guard utama; tanpa unit test).
- [ ] Interface `PTY` memiliki tepat metode `Start, Read, Write, Resize, Close, Wait` dengan signature Kontrak; `Size`, `New() PTY` ada; tidak ada ekspor lain.
- [ ] `creack/pty` hanya diimpor (alias `cpty`) di `internal/session/pty/pty_unix.go`; tidak ada tipe `cpty` di API publik.
- [ ] `Start` memakai `cpty.StartWithSize` dengan `Setsid` + `Setctty` efektif; `Close` mengirim SIGHUP ke process group (`-pid`) dan menutup master fd, idempoten, tanpa panic, tidak memblok.
- [ ] `Wait` mengembalikan exit code proses, atau `128+signal` bila di-signal; exit non-nol bukan error.
- [ ] Tidak ada goroutine/ticker/timer di paket; `gofmt` bersih.
- [ ] Hanya `internal/session/pty/` yang berubah; `go.mod`/`go.sum` tidak disentuh; tidak ada file sementara tersisa.
