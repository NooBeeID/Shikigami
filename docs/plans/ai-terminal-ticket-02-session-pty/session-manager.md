# Plan: Manajer sesi dan scrollback

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-02-session-pty.md` (epic 02 Session PTY)
- Fitur: `session-manager` (Manajer sesi dan scrollback)
- ownedPaths:
  - `internal/session/types.go`
  - `internal/session/launcher.go`
  - `internal/session/scrollback.go`
  - `internal/session/manager.go`
- Depends on foundation: ya. Foundation (`00-foundation.md`) sudah memasang `creack/pty` dan membuat stub kompilasi untuk keempat file ini serta `internal/session/pty/{pty.go,pty_unix.go}`. Agent ini MENIMPA isi stub keempat file tersebut dengan implementasi nyata. Baca `overview.md` (kontrak bersama) dan tiket sebelum mulai; plan ini tidak mengulang isinya.

## Tujuan

Mengimplementasikan inti sesi di paket `aiterm/internal/session`: tipe dan error domain, `Launcher` login shell default, ring buffer scrollback 256 KiB, dan `Manager` yang membuat/mengelola banyak sesi PTY. Setiap sesi punya tepat satu goroutine pembaca yang `Append` ke scrollback lalu mempublikasikan `session.output.<id>`; `session.exit` terbit tepat sekali. Tanpa ticker/polling dan tanpa mengimpor `internal/ipc` (event keluar lewat fungsi `pub` yang disuntikkan).

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `internal/session/types.go`
- `internal/session/launcher.go`
- `internal/session/scrollback.go`
- `internal/session/manager.go`

Dilarang: `internal/session/ipc.go` (milik `session-ipc-ctl`, jangan dibuat), `internal/session/pty/` (milik `pty-wrapper`), `cmd/`, `frontend/`, `go.mod`/`go.sum` (jangan `go get`/`go mod tidy`), `internal/ipc/`, `internal/paths/`, `docs/`, serta file wiring apa pun. Jangan membuat file baru lain di `internal/session/` (mis. tidak ada `ulid.go`; ULID ada di `types.go`).

## Kontrak

Semua signature publik dikunci oleh overview bagian 2 dan tiket bagian "Kontrak"; JANGAN diubah. Ringkas yang disediakan paket ini:

- `types.go`: `Status`, `StatusRunning = "running"`, `StatusExited = "exited"`, `Session` (tag JSON camelCase sesuai tiket, `ExitCode *int` `omitempty`), `SessionSpec`, `AttachResult{Session Session; Scrollback []byte; Offset uint64}`, `var ErrNotFound, ErrInvalid, ErrNotRunning` (`errors.New`, pesan Indonesia), `func newID() string` (privat).
- `launcher.go`: `type Launcher interface{ Command(s SessionSpec) (cmd string, args []string, env []string) }`, `func DefaultLauncher() Launcher`.
- `scrollback.go`: `const ScrollbackSize = 256 * 1024`, tipe privat `scrollback` dengan `Append`, `Snapshot() (data []byte, offset uint64)`.
- `manager.go`: `type Manager`, `NewManager(pub func(topic string, payload any)) *Manager`, metode `Create(spec SessionSpec) (*Session, error)`, `List() []Session`, `Get(id string) (*Session, error)`, `Kill(id string) error`, `Write(id string, data []byte) error`, `Resize(id string, cols, rows uint16) error`, `Attach(id string) (AttachResult, error)`, `SetLauncher(l Launcher)`, `Shutdown()`.

Yang dipakai: `aiterm/internal/session/pty` (`pty.PTY`, `pty.Size`, `pty.New()`), diimpor TANPA alias. Hindari nama variabel `pty` di paket `session` (menutupi import). Kontrak payload event (dikunci overview, dipublikasikan lewat `pub`):
- `session.output.<id>` -> `map[string]any{"data": <base64 std string>, "offset": <uint64>}` (atau struct kecil privat dengan tag JSON `data`/`offset`), `offset` = total kumulatif SESUDAH potongan.
- `session.exit` -> `{"id": string, "exitCode": int}`.

Paket ini tidak mengimpor `internal/ipc`, `creack/pty`, maupun `golang.org/x/*`.

Catatan interface PTY: `pty.PTY` tidak punya metode sinyal/pid. Untuk eskalasi SIGKILL di `Kill`, manager memakai type assertion opsional (lihat langkah 5) dan jatuh balik ke `Close()` berulang bila implementasi tidak menyediakannya. Jangan mengubah interface `PTY`.

## Langkah

### 1. `types.go`
Package `session`. Import: `crypto/rand`, `errors`, `sync`, `time`.
1. Deklarasikan `Status`, dua konstanta, `Session`, `SessionSpec`, `AttachResult` persis tiket.
2. Error domain:
   ```go
   var (
       ErrNotFound   = errors.New("sesi tidak ditemukan")
       ErrInvalid    = errors.New("masukan sesi tidak valid")
       ErrNotRunning = errors.New("sesi tidak berjalan")
   )
   ```
   Saat memberi detail, bungkus dengan `fmt.Errorf("%w: ...", ErrInvalid)` agar `errors.Is` tetap bekerja (lapisan IPC memakai `errors.Is`).
3. `newID() string` (ULID 26 karakter):
   - Alfabet Crockford: `"0123456789ABCDEFGHJKMNPQRSTVWXYZ"`.
   - 16 byte: 6 byte timestamp ms big-endian (`time.Now().UnixMilli()`) + 10 byte acak `crypto/rand`.
   - Monotonik dalam proses: variabel paket `idMu sync.Mutex`, `idLastMs int64`, `idLastRand [10]byte`. Bila ms sama dengan `idLastMs`, pakai `idLastRand` ditambah 1 (increment big-endian dengan carry); bila ms lebih baru, isi acak baru. Bila jam mundur (ms < last), pakai `idLastMs` yang lama (jangan turun). Ini menjamin urut waktu pembuatan walau dibuat pada ms yang sama.
   - Encode 128 bit ke 26 karakter base32: gunakan `math/big`-free cara manual: baca bit dari byte, 130 bit (2 bit nol di depan), 5 bit per karakter, dari MSB. Hasil harus tepat 26 karakter huruf besar.
   - Bila `crypto/rand.Read` gagal, `panic` boleh (kondisi tak terpulihkan) atau isi dari waktu; pilih panic dengan pesan jelas.

### 2. `scrollback.go`
Import `sync`. 
```go
const ScrollbackSize = 256 * 1024

type scrollback struct {
    mu    sync.Mutex
    buf   []byte // panjang tetap ScrollbackSize, dialokasikan sekali
    start int    // indeks byte tertua
    size  int    // jumlah byte valid (<= ScrollbackSize)
    total uint64 // kumulatif, tidak pernah turun
}
func newScrollback() *scrollback
func (s *scrollback) Append(p []byte) uint64 // mengembalikan total sesudah append
func (s *scrollback) Snapshot() (data []byte, offset uint64)
```
- `Append`: kunci mutex; bila `len(p) >= ScrollbackSize`, ambil hanya `ScrollbackSize` byte terakhir dari `p` dan isi ulang buffer (start=0,size=penuh); selain itu salin dengan wraparound (maks dua `copy`), geser `start` bila melewati kapasitas. `total += uint64(len(p))` (panjang ASLI `p`, bukan yang dipotong). Salin data (jangan menyimpan slice `p`, karena buffer baca dipakai ulang).
- `Snapshot`: kunci mutex yang sama; kembalikan salinan isi berurutan (tertua ke terbaru, sepanjang `size`) dan `total` pada saat itu. Atomik terhadap `Append`. Mengembalikan slice kosong non-nil bila kosong.
- Alokasi `buf` lazy atau di `newScrollback`; satu alokasi 256 KiB per sesi sudah memenuhi batas memori.

### 3. `launcher.go`
Import `os`.
```go
type defaultLauncher struct{}
func DefaultLauncher() Launcher { return defaultLauncher{} }
func (defaultLauncher) Command(s SessionSpec) (string, []string, []string)
```
- `cmd`: `os.Getenv("SHELL")`, bila kosong `/bin/zsh`.
- `args`: `[]string{"-l"}`.
- `env`: `append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")` lalu `append(..., s.Env...)`. Urutan ini penting: entri belakang menang atas duplikat (`os/exec` mendedup dengan mempertahankan yang terakhir), sehingga `s.Env` dari pemanggil bisa menimpa TERM. Buat slice baru (jangan memodifikasi hasil `os.Environ()` bersama).

### 4. `manager.go` struktur data
Import: `encoding/base64`, `fmt`, `os`, `sort`, `sync`, `syscall`, `time`, dan `aiterm/internal/session/pty`.
```go
const (
    readBufSize      = 32 * 1024
    maxExitedKept    = 64
    killGracePeriod  = 2 * time.Second
    defaultCols      = 80
    defaultRows      = 24
    defaultProvider  = "shell"
)

type entry struct {
    mu       sync.Mutex   // melindungi sess dan size
    sess     Session
    size     pty.Size
    p        pty.PTY
    sb       *scrollback
    wmu      sync.Mutex   // serialisasi p.Write
    done     chan struct{} // ditutup oleh goroutine pembaca setelah exit dipublikasikan
    exitSeq  uint64        // urutan exit untuk pemangkasan; 0 = belum exit
}

type Manager struct {
    mu       sync.RWMutex
    sessions map[string]*entry
    launcher Launcher
    pub      func(topic string, payload any)
    exitSeq  uint64
    closed   bool
}
```
- `NewManager(pub)`: `pub` nil diganti no-op. `launcher = DefaultLauncher()`.
- `SetLauncher(l)`: kunci `m.mu`, set (bila `l == nil` kembalikan ke `DefaultLauncher()`).
- Helper privat `snapshotSession(e *entry) Session` (kunci `e.mu`, salin struct; salin juga `ExitCode` ke int baru agar tidak berbagi pointer) dan `lookup(id) (*entry, error)` (RLock; `ErrNotFound` bila tak ada).
- Jangan menahan `m.mu` atau `e.mu` saat memanggil `m.pub`, `p.Write`, `p.Wait`, atau menunggu `done`.

### 5. `manager.go` metode
**`Create(spec SessionSpec) (*Session, error)`**
1. Tolak bila `m.closed` (`fmt.Errorf("%w: manajer sudah dihentikan", ErrInvalid)`).
2. Default: `Cols==0 -> 80`, `Rows==0 -> 24`, `ProviderID=="" -> "shell"`, `Cwd==""` -> `os.UserHomeDir()` (gagal -> `ErrInvalid` terbungkus). `os.Stat(cwd)` harus ada dan `IsDir()`, kalau tidak `fmt.Errorf("%w: cwd %q ...", ErrInvalid, cwd)`.
3. Ambil launcher (RLock), `cmd, args, env := l.Command(spec)` (spec sudah ber-default).
4. `p := pty.New()`; `p.Start(cmd, args, env, spec.Cwd, pty.Size{Cols: spec.Cols, Rows: spec.Rows})`. Gagal -> kembalikan `fmt.Errorf("gagal memulai sesi: %w", err)` (bukan error domain), tanpa mendaftarkan sesi.
5. Buat `entry` (`sess` dengan `ID: newID()`, `Status: StatusRunning`, `CreatedAt: time.Now()`, field spec; `done: make(chan struct{})`, `sb: newScrollback()`), daftarkan di map di bawah `m.mu`.
6. `go m.readLoop(e)` (SATU goroutine per sesi, satu-satunya yang memanggil `p.Read`/`p.Wait`).
7. Kembalikan pointer ke salinan `Session`.

**`readLoop(e *entry)`** (privat)
```
buf := make([]byte, readBufSize)   // dipakai ulang, satu per sesi
for {
    n, err := e.p.Read(buf)
    if n > 0 {
        off := e.sb.Append(buf[:n])
        m.pub("session.output."+id, {data: base64.StdEncoding.EncodeToString(buf[:n]), offset: off})
    }
    if err != nil { break }   // EOF atau EIO (macOS saat anak selesai) sama-sama berarti selesai
}
code, _ := e.p.Wait()          // blok sampai proses selesai
// set exited di bawah e.mu: Status=StatusExited, ExitCode=&code
// beri exitSeq dari m.exitSeq++ (di bawah m.mu)
m.pub("session.exit", {id, exitCode: code})   // tepat sekali: hanya jalur ini yang menerbitkan
_ = e.p.Close()
close(e.done)
m.pruneExited()
```
- Urutan penting: `Append` lalu `pub` di goroutine yang sama, sehingga `offset` event monoton dan `Attach` (snapshot di bawah mutex scrollback) konsisten. Base64-encode ke string SEBELUM `pub` (buffer dipakai ulang; payload tidak boleh merujuk `buf`).
- Event exit terbit setelah semua output terakhir; `Close()` dipanggil sesudah `Wait` agar aman. Karena hanya goroutine ini yang menerbitkan exit, "tepat sekali" terjamin untuk exit alami maupun kill.

**`pruneExited()`**: di bawah `m.mu` (write): kumpulkan entry exited (`exitSeq > 0`, baca lewat `e.mu`), bila jumlah > 64 urutkan menurut `exitSeq` naik dan hapus yang terlama dari map hingga tersisa 64. Pemangkasan tidak mengirim event.

**`List() []Session`**: RLock, salin tiap entry lewat `snapshotSession`, `sort.SliceStable` menurut `CreatedAt` naik lalu `ID` naik sebagai tie-break. Kembalikan slice non-nil (kosong `[]Session{}`) agar lapisan IPC bisa mengirim `[]` bukan `null`.

**`Get(id) (*Session, error)`**: `lookup` lalu pointer ke salinan.

**`Attach(id) (AttachResult, error)`**: `lookup`; `data, off := e.sb.Snapshot()`; `Session: snapshotSession(e)`. Tidak mengubah state apa pun.

**`Write(id, data) error`**: `lookup`; bila status bukan running -> `ErrNotRunning`; kunci `e.wmu`; `e.p.Write(data)`. Bila error dan status kini exited -> `ErrNotRunning`; selain itu kembalikan error terbungkus. `len(data)==0` -> nil tanpa menulis.

**`Resize(id, cols, rows) error`**: `lookup`; `cols==0 || rows==0` -> `ErrInvalid`; status bukan running -> `ErrNotRunning`; `e.p.Resize(pty.Size{...})`; sukses -> simpan `e.size` di bawah `e.mu`.

**`Kill(id) error`**:
1. `lookup` (`ErrNotFound` bila tak ada).
2. Bila sudah exited: hapus dari map (di bawah `m.mu`), return nil.
3. Bila running: `_ = e.p.Close()` (menutup master fd + SIGHUP ke process group, idempoten). Lalu tunggu `e.done` dengan satu `time.NewTimer(killGracePeriod)` (BUKAN ticker; hentikan timer bila `done` tercapai lebih dulu).
4. Bila timer habis sebelum `done`: eskalasi SIGKILL. Gunakan type assertion opsional pada `e.p`:
   ```go
   type pidder interface{ Pid() int }
   if pp, ok := e.p.(pidder); ok && pp.Pid() > 0 {
       _ = syscall.Kill(-pp.Pid(), syscall.SIGKILL) // process group; jatuh ke pid tunggal bila gagal
       _ = syscall.Kill(pp.Pid(), syscall.SIGKILL)
   } else {
       _ = e.p.Close() // implementasi yang `Close`-nya sudah mematikan proses
   }
   ```
   Lalu `<-e.done` (tunggu `Wait` selesai; pembaca yang menerbitkan `session.exit`). Jangan memanggil `Wait` dari `Kill` (hanya `readLoop` yang boleh).
5. Hapus entry dari map (di bawah `m.mu`) dan return nil. `Kill` bersamaan pada sesi yang sama aman (kedua pemanggil menunggu `done`; `delete` idempoten).

**`Shutdown()`**: set `closed = true` di bawah `m.mu`, ambil daftar ID, `Kill` tiap sesi secara paralel (goroutine + `sync.WaitGroup`; goroutine ini berumur pendek, bukan pembaca), tunggu semua. Idempoten.

### 6. Verifikasi lokal
- `gofmt -l internal/session/types.go internal/session/launcher.go internal/session/scrollback.go internal/session/manager.go` (tanpa keluaran).
- `go build ./internal/session/...` sukses (butuh stub/implementasi `pty`; bila build gagal karena file fitur lain yang setengah jadi, itu bukan urusan agent ini, build ulang paket sendiri).
- `go vet ./internal/session/` bila paket lain sudah kompilasi (opsional).
- Grep jaminan (semua harus tanpa hasil di keempat file milik fitur ini):
  - `grep -nE "time\.(NewTicker|Tick)\(" internal/session/{types,launcher,scrollback,manager}.go`
  - `grep -n "internal/ipc" internal/session/{types,launcher,scrollback,manager}.go`
  - `grep -n "creack/pty" internal/session/{types,launcher,scrollback,manager}.go`
  - `grep -n 'panic("belum diimplementasi")' internal/session/{types,launcher,scrollback,manager}.go` (kecuali panic `crypto/rand` yang disengaja, dengan pesan berbeda)
- Tidak ada file sementara tersisa.

## Perubahan

Diubah (menimpa stub Foundation):
- `internal/session/types.go`
- `internal/session/launcher.go`
- `internal/session/scrollback.go`
- `internal/session/manager.go`

Tidak ada file baru dan tidak ada file lain yang diubah.

## Wiring

Fitur ini hanya mengekspos API Go; tidak menyambungkan apa pun sendiri:
- `session.NewManager(pub)` dengan `pub = srv.Publish` akan dipanggil oleh `cmd/aitermd/wire_session.go` (milik `session-ipc-ctl`); `session.RegisterIPC(srv, m)` memakai `Create/List/Get/Kill/Write/Resize/Attach` dan memetakan `ErrNotFound/ErrInvalid/ErrNotRunning` lewat `errors.Is`.
- `Manager.Shutdown()` disediakan untuk dipanggil oleh langkah Wiring epic bila ada hook shutdown daemon.
- `Manager.SetLauncher` + `Launcher` + `SessionSpec` disediakan untuk ticket-05 (provider adapter).
- Tidak ada singleton global, tidak ada `init()`, tidak ada pendaftaran modul di paket ini.

## Selesai bila

- `go build ./internal/session/...` sukses (guard fitur; tanpa unit test).
- Keempat file memuat semua tipe/fungsi/metode di bagian Kontrak dengan signature persis; tidak ada stub `panic("belum diimplementasi")` tersisa.
- `scrollback`: kapasitas 256 KiB, membuang byte terlama saat melebihi, `Snapshot` atomik terhadap `Append`, `offset` kumulatif tidak pernah turun (termasuk saat `Append` lebih besar dari kapasitas).
- Hanya satu goroutine pembaca per sesi; `session.output.<id>` dipublikasikan setelah `Append` di goroutine yang sama; `session.exit` terbit tepat sekali, juga pada kill.
- `Kill`: SIGHUP (via `Close`) lalu SIGKILL setelah 2 detik memakai satu timer, bukan ticker; sesi exited yang di-kill langsung dihapus; maksimal 64 sesi exited disimpan.
- Default launcher: `$SHELL -l` fallback `/bin/zsh`, `TERM=xterm-256color`, `COLORTERM=truecolor`, lalu `s.Env`.
- ID berupa ULID 26 karakter dan urut menurut waktu pembuatan.
- Grep jaminan di langkah 6 bersih: tidak ada ticker/polling, tidak ada impor `internal/ipc` atau `creack/pty` di keempat file.
- Tidak ada file di luar `ownedPaths` yang diubah atau dibuat.
