# Plan: Method IPC session dan CLI

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-02-session-pty.md` (epic 02 Session PTY)
- Fitur: `session-ipc-ctl`
- ownedPaths: `internal/session/ipc.go`, `cmd/aitermd/wire_session.go`, `cmd/aitermctl/cmd_session.go`
- Depends on foundation: ya (`00-foundation.md`). Foundation sudah memasang `creack/pty` dan menyediakan stub paket `internal/session` (`Manager`, tipe, error domain) serta `internal/session/pty`. Fitur ini mengkompilasi terhadap stub `Manager` itu; `session-manager` menimpa isinya secara paralel.
- Bahasa: pesan galat/teks CLI bahasa Indonesia; identifier kode bahasa Inggris.

## Tujuan

Menyambungkan `Manager` ke IPC (enam method `session.*`) dan menyediakan perintah uji manual `aitermctl session new|list|attach|kill`, termasuk attach interaktif (raw mode, Ctrl+] untuk lepas, resize, kode exit sesi). Daemon mendaftarkan modul `session` lewat `init()`.

Baca dulu (jangan diubah): `overview.md` di folder ini dan tiket. Bila kode ticket-01 yang nyata di repo kerja berbeda dari nama di bawah, ikuti kode nyata. Referensi simbol ticket-01 (terverifikasi di worktree ticket-01):

- `ipc.Handler = func(ctx context.Context, params json.RawMessage) (any, error)`; `ipc.NewError(code, msg) *ipc.Error`; `ipc.Errorf(code, fmt, ...)`; konstanta `ipc.ErrConflict` (-32001), `ipc.ErrNotFound` (-32002), `ipc.ErrInvalid` (-32003), `ipc.ErrInvalidParams` (-32602). `*ipc.Error` punya field `Code`, `Message`.
- `cmd/aitermd/modules.go`: `RegisterModule(name string, wire func(srv *ipc.Server, d *Deps))`; `Deps` punya `OnReady(fn func())`.
- `cmd/aitermctl/registry.go`: `Command{Name, Summary string; Run func(ctx context.Context, c *ipc.Client, args []string) error}`, `RegisterCommand(Command)`. `main.go` sudah mem-Dial socket, membuat `ctx` (SIGINT/SIGTERM), memanggil `cmd.Run`, dan mengubah error non-nil menjadi exit code 1. Ada helper `callAndPrint(c, method, params)` di `main.go` (boleh dipakai bila ada dan signature-nya cocok; bila ragu, tulis helper sendiri dengan nama berawalan `session` agar tidak bentrok).
- `ipc.Client`: `Call(method, params, out) error`, `Subscribe(topicPrefix, fn func(topic string, payload json.RawMessage)) (unsubscribe func(), err error)`, `Close()`. Callback `Subscribe` dipanggil berurutan dari satu goroutine dispatch klien; `Subscribe` melakukan RPC `events.subscribe` secara sinkron sehingga setelah ia kembali langganan sudah aktif.

## Path yang dimiliki

HANYA ini yang boleh disentuh (buat dari nol, ketiganya belum ada):

- `internal/session/ipc.go`
- `cmd/aitermd/wire_session.go`
- `cmd/aitermctl/cmd_session.go`

Dilarang menyentuh: file lain di `internal/session/` (`types.go`, `launcher.go`, `scrollback.go`, `manager.go`, `pty/`), `cmd/aitermd/main.go`, `cmd/aitermd/modules.go`, `cmd/aitermctl/main.go`, `cmd/aitermctl/registry.go`, `internal/ipc/`, `internal/paths/`, `go.mod`/`go.sum`, `frontend/`, `docs/`. Dilarang `go get`, `go mod tidy`, atau menambah `golang.org/x/term`. Hapus file sementara bila ada.

## Kontrak

### Disediakan fitur ini

```go
// internal/session/ipc.go  (satu-satunya file paket session yang boleh impor aiterm/internal/ipc)
func RegisterIPC(srv *ipc.Server, m *Manager)
```
Mendaftarkan: `session.create`, `session.list`, `session.kill`, `session.write`, `session.resize`, `session.attach`. Params/result persis tabel "Method IPC" di tiket (camelCase; `data` dan `scrollback` base64 std encoding; `offset` angka).

Pemetaan error (lihat overview "Pemetaan error di RegisterIPC"):
- `session.ErrNotFound` -> `ipc.ErrNotFound` (-32002)
- `session.ErrInvalid` -> `ipc.ErrInvalid` (-32003)
- `session.ErrNotRunning` -> `ipc.ErrConflict` (-32001)
- base64 tidak valid -> `ipc.ErrInvalid`
- JSON params rusak / `id` kosong -> `ipc.ErrInvalidParams` (-32602) untuk JSON rusak; `id` kosong -> `ipc.ErrInvalidParams` juga
- error lain -> kembalikan apa adanya (server ipc menjadikannya internal error)

Modul daemon `session` (di `cmd/aitermd/wire_session.go`) dan command CLI `session` (di `cmd/aitermctl/cmd_session.go`).

### Dipakai (dari fitur paralel lain, ikuti signature Kontrak, jangan menebak)

```go
// session (types.go / manager.go, milik session-manager)
type Session struct{ ID, WorkspaceID, ProjectID, Cwd, ProviderID string; Status Status; CreatedAt time.Time; ExitCode *int }  // tag JSON camelCase
type SessionSpec struct{ WorkspaceID, ProjectID, Cwd, ProviderID string; Cols, Rows uint16; Env []string }
type AttachResult struct{ Session Session; Scrollback []byte; Offset uint64 }
var ErrNotFound, ErrInvalid, ErrNotRunning error
func NewManager(pub func(topic string, payload any)) *Manager
func (m *Manager) Create(spec SessionSpec) (*Session, error)
func (m *Manager) List() []Session
func (m *Manager) Kill(id string) error
func (m *Manager) Write(id string, data []byte) error
func (m *Manager) Resize(id string, cols, rows uint16) error
func (m *Manager) Attach(id string) (AttachResult, error)
func (m *Manager) Shutdown()
```
Event yang diterbitkan Manager lewat `pub` (bukan tugas fitur ini, tapi dikonsumsi CLI): `session.output.<id>` payload `{"data": base64, "offset": uint64}` dan `session.exit` payload `{"id": string, "exitCode": number}`.

## Langkah

### 1. `internal/session/ipc.go`

`package session`. Impor: `context`, `encoding/base64`, `encoding/json`, `errors`, `aiterm/internal/ipc`.

1. Tipe params privat (camelCase JSON):
   - `createParams{WorkspaceID string "workspaceId"; ProjectID string "projectId"; Cwd string "cwd"; ProviderID string "providerId"; Cols uint16 "cols"; Rows uint16 "rows"; Env []string "env"}`
   - `idParams{ID string "id"}`
   - `writeParams{ID string "id"; Data string "data"}`
   - `resizeParams{ID string "id"; Cols uint16 "cols"; Rows uint16 "rows"}`
2. Helper privat:
   - `decodeParams(raw json.RawMessage, v any) error`: bila `len(raw)==0` atau `string(raw)=="null"` lewati (params opsional, mis. `create` tanpa params dan `list`); selain itu `json.Unmarshal`, gagal -> `ipc.Errorf(ipc.ErrInvalidParams, "params tidak valid: %v", err)`. Nilai `cols`/`rows` negatif atau > 65535 gagal unmarshal ke `uint16`; itu ikut jadi `ErrInvalidParams`.
   - `mapErr(err error) error`: `nil` -> `nil`; `errors.Is(err, ErrNotFound)` -> `ipc.NewError(ipc.ErrNotFound, err.Error())`; `ErrInvalid` -> `ipc.ErrInvalid`; `ErrNotRunning` -> `ipc.ErrConflict`; selain itu `err`. PENTING: bila err sudah `*ipc.Error` kembalikan apa adanya.
   - `requireID(id string) error`: kosong -> `ipc.NewError(ipc.ErrInvalidParams, "id wajib diisi")`.
3. `func RegisterIPC(srv *ipc.Server, m *Manager)` memanggil `srv.Register(...)` enam kali, tiap handler `func(ctx context.Context, params json.RawMessage) (any, error)`:
   - `session.create`: decode `createParams` -> `m.Create(SessionSpec{...})` -> kembalikan `*Session` (hasil `Create`, diserialisasi sebagai `Session`).
   - `session.list`: abaikan params; `list := m.List()`; bila `list == nil` ganti `[]Session{}` (WAJIB `{"sessions": []}`, bukan `null`); kembalikan `map[string]any{"sessions": list}`.
   - `session.kill`: decode `idParams`, `requireID`, `m.Kill(id)` -> `map[string]any{"ok": true}`.
   - `session.write`: decode `writeParams`, `requireID`; `base64.StdEncoding.DecodeString(p.Data)` gagal -> `ipc.Errorf(ipc.ErrInvalid, "data bukan base64 valid: %v", err)`; `m.Write(id, b)` -> `{"ok": true}`. Urutan pengecekan: JSON -> id kosong -> base64 -> Manager (sehingga base64 jelek dengan id palsu tetap `ErrInvalid`).
   - `session.resize`: decode `resizeParams`, `requireID`, `m.Resize(id, cols, rows)` -> `{"ok": true}`.
   - `session.attach`: decode `idParams`, `requireID`, `r, err := m.Attach(id)`; kembalikan `map[string]any{"session": r.Session, "scrollback": base64.StdEncoding.EncodeToString(r.Scrollback), "offset": r.Offset}`. `Scrollback` nil -> string kosong `""` (pastikan bukan `null`).
   Semua error dilewatkan `mapErr`.
4. Tidak ada goroutine, tidak ada ticker. Jangan impor `internal/ipc` dari file lain di paket ini.

### 2. `cmd/aitermd/wire_session.go`

`package main`. Impor `aiterm/internal/ipc`, `aiterm/internal/session`.

```go
func init() {
    RegisterModule("session", func(srv *ipc.Server, d *Deps) {
        m := session.NewManager(srv.Publish)
        session.RegisterIPC(srv, m)
    })
}
```
Hanya `NewManager(srv.Publish)` + `RegisterIPC(srv, m)`; parameter `d` tidak dipakai (tulis `_ *Deps` bila perlu). Catatan shutdown: `Deps` ticket-01 tidak punya hook shutdown daemon yang jelas (`OnReady` jalan saat siap, bukan saat berhenti) dan `main.go`/`modules.go` tidak boleh diedit, jadi `Manager.Shutdown` TIDAK dipanggil di sini; cukup disediakan Manager. Jangan simpan Manager di variabel paket (tidak diperlukan sekarang). Jangan memakai isi `Deps` apa pun.

### 3. `cmd/aitermctl/cmd_session.go`

`package main`. `init()` memanggil `RegisterCommand(Command{Name: "session", Summary: "Kelola sesi terminal: new|list|attach|kill", Run: runSession})` dengan `func runSession(ctx context.Context, c *ipc.Client, args []string) error`. Tanpa subcommand atau nama tidak dikenal -> kembalikan error berisi usage (`penggunaan: aitermctl session new|list|attach|kill ...`). Dispatch `args[0]` ke `sessionNew`, `sessionList`, `sessionAttach`, `sessionKill` (semua menerima `ctx, c, args[1:]`). Beri semua fungsi helper awalan `session`/`sess` agar tidak bentrok nama dengan file lain di `package main`.

Tipe lokal (decode hasil; jangan impor `internal/session` dari CLI agar tidak menarik `pty`): `type sessionInfo struct{ ID, WorkspaceID, ProjectID, Cwd, ProviderID, Status string; CreatedAt string; ExitCode *int }` dengan tag JSON `id, workspaceId, projectId, cwd, providerId, status, createdAt, exitCode`.

**`session new`**: `flag.NewFlagSet("session new", flag.ContinueOnError)` dengan `--cwd`, `--workspace`, `--project`, `--provider` (string), `--cols`, `--rows` (int, default 0). Bangun params map hanya dengan field yang diisi (cols/rows hanya bila > 0; validasi 0..65535). `c.Call("session.create", params, &raw)` dengan `raw json.RawMessage`. Cetak baris pertama ID sesi, lalu JSON `Session` ber-indent (`json.MarshalIndent`). Tambahan untuk mode atur ukuran: bila `--cols/--rows` tidak diberikan dan stdin adalah terminal, boleh kirim ukuran terminal saat ini (opsional; default biarkan server 80x24).

**`session list`**: `c.Call("session.list", nil, &out)` dengan `out struct{ Sessions []sessionInfo }`. Cetak tabel dengan `text/tabwriter`: header `ID  STATUS  CWD  PROVIDER`, satu baris per sesi (status `exited` boleh ditambah ` (kode N)` bila `ExitCode != nil`). Daftar kosong: cetak hanya header.

**`session kill <id>`**: wajib satu argumen, `c.Call("session.kill", map[string]any{"id": id}, nil)`, cetak `sesi <id> dihentikan`.

**`session attach <id>`** (inti). Alur persis, urutan penting:
1. Parse satu argumen `id`. Siapkan `outMu sync.Mutex` dan `os.Stdout`.
2. State: `ready bool`, `attachOffset uint64`, `pending []outEvent` (antrean event sebelum attach selesai), `exited bool`, `exitCode int`, channel `done chan struct{}` (ditutup sekali via `sync.Once`).
3. **Subscribe SEBELUM attach**:
   - `unsubOut, err := c.Subscribe("session.output."+id, fn)`: `fn` men-decode payload `{Data string; Offset uint64}` (`json.Unmarshal`), base64-decode `Data`; di bawah `outMu`: bila `!ready` tambah ke `pending`; bila `ready` dan `Offset <= attachOffset` BUANG; selain itu tulis byte ke stdout.
   - `unsubExit, err := c.Subscribe("session.exit", fn)`: decode `{ID string; ExitCode int}`; abaikan bila `ID != id`; set `exitCode`, `exited=true`, tutup `done`. (Prefix `session.exit` sama dengan topik persisnya, aman.)
   - `defer` kedua unsubscribe.
4. `c.Call("session.attach", map[string]any{"id": id}, &res)` dengan `res struct{ Session sessionInfo; Scrollback string; Offset uint64 }`. Error -> kembalikan. Bila `res.Session.Status == "exited"`: tulis scrollback ke stdout, cetak `[sesi berakhir]` ke stderr, set exit code dari `res.Session.ExitCode` (0 bila nil), keluar tanpa masuk raw mode (lihat "Exit code").
5. Di bawah `outMu`: base64-decode `res.Scrollback`, tulis ke stdout, set `attachOffset = res.Offset`, lalu alirkan `pending` dengan aturan yang sama (buang `Offset <= attachOffset`), kosongkan `pending`, `ready = true`. Event exit yang datang sebelum attach selesai tetap ditangani lewat `done` (tulis dulu sisa output baru keluar; event output dari goroutine dispatch yang sama berurutan sebelum exit, jadi urutan terjaga).
6. **Raw mode** (hanya bila stdin terminal; deteksi lewat `os.Stdin.Stat()` dan `Mode()&os.ModeCharDevice != 0`): `golang.org/x/term` DILARANG ditambahkan. Pakai `stty` lewat `os/exec`:
   - simpan keadaan: `exec.Command("stty", "-g")` dengan `Stdin = os.Stdin`, ambil output ter-trim sebagai `saved`.
   - `exec.Command("stty", "raw", "-echo")` dengan `Stdin = os.Stdin`.
   - `defer` pemulihan: `exec.Command("stty", saved)` dengan `Stdin = os.Stdin` (jalankan juga saat sinyal/keluar lewat jalur normal; pastikan dipanggil sebelum `os.Exit`, lihat Exit code).
   - Bila stdin bukan terminal atau `stty` gagal: fallback mode baris (stdin dibaca per baris biasa dan dikirim apa adanya) dengan peringatan singkat ke stderr; Ctrl+] tidak tersedia di mode ini (lepas dengan Ctrl+C, ditangani `ctx`).
   Cetak petunjuk ke stderr sebelum raw mode: `terhubung ke sesi <id>; tekan Ctrl+] untuk lepas`.
7. **Ukuran terminal**: fungsi privat `sessionTermSize() (cols, rows int, ok bool)` memakai `exec.Command("stty", "size")` dengan `Stdin = os.Stdin`, parse `"<rows> <cols>"`. Kirim `session.resize` sekali setelah attach siap (bila ok), lalu setiap `SIGWINCH`: `sigCh := make(chan os.Signal, 1); signal.Notify(sigCh, syscall.SIGWINCH)`; goroutine `select` pada `sigCh`, `done`, `ctx.Done()` (event-driven; tanpa ticker); `defer signal.Stop(sigCh)`. Kegagalan resize diabaikan (tidak menghentikan attach).
8. **Loop stdin** di goroutine terpisah (hanya satu): baca `buf := make([]byte, 4096)` dari `os.Stdin`. Untuk tiap potongan, cari byte `0x1d` (Ctrl+]) dengan `bytes.IndexByte`: bila ada, kirim byte sebelum `0x1d` (jika ada) lalu sinyal lepas (set `detach = true`, tutup `done` via Once) dan berhenti; bila tidak, `c.Call("session.write", {"id": id, "data": base64(chunk)}, nil)`. Galat `session.write` (mis. sesi baru saja exited, kode -32001) -> abaikan bila `done` sudah/akan ditutup, selain itu tutup `done` dengan galat tersimpan. EOF stdin -> perlakukan sebagai lepas.
   Catatan: goroutine stdin bisa tetap terblok di `Read` saat sesi exit; itu aman karena proses segera keluar (tidak perlu menunggunya).
9. Tunggu: `select { case <-done: case <-ctx.Done(): }`. Setelah itu: pulihkan stty, `unsub*`, lalu putuskan hasil:
   - Lepas (Ctrl+]/EOF/ctx): cetak `\r\n[lepas dari sesi <id>; sesi tetap berjalan]` ke stderr, kembalikan `nil` (exit 0). Sesi TIDAK dibunuh dan tidak ada `session.kill`.
   - Sesi exit: cetak `\r\n[sesi berakhir, kode N]` ke stderr dan keluar dengan kode exit sesi.
10. **Exit code sesi**: `main.go` mengubah error menjadi exit 1, jadi untuk meneruskan kode exit sesi, setelah semua cleanup (stty dipulihkan, unsubscribe) panggil `os.Exit(code)` langsung dari `sessionAttach` bila `code != 0`; bila `code == 0` kembalikan `nil`. Pastikan `c.Close()` tidak penting di jalur ini (proses berakhir). Jangan memanggil `os.Exit` sebelum stty dipulihkan.

Pengecekan galat umum: id kurang/lebih -> usage; `*ipc.Error` dibiarkan naik ke `main.go` (ia sudah mencetak `galat <kode>: <pesan>`).

## Perubahan

Dibuat (semua baru):
- `internal/session/ipc.go`
- `cmd/aitermd/wire_session.go`
- `cmd/aitermctl/cmd_session.go`

Tidak ada file yang diubah.

## Wiring

Tidak menyambungkan apa pun sendiri. Yang diekspos:
- `session.RegisterIPC(srv *ipc.Server, m *Manager)` dipakai oleh `wire_session.go` (file milik fitur ini sendiri).
- Modul daemon `session` terdaftar otomatis lewat `init()` -> `RegisterModule("session", ...)`; muncul di `daemon.info` (`modules` memuat `session`, `methods` memuat enam `session.*`).
- Command CLI `session` terdaftar otomatis lewat `init()` -> `RegisterCommand`; muncul di `aitermctl help`.
- Langkah Wiring epic (di luar fitur ini) yang memverifikasi: tidak ada sisa stub, `go mod tidy`, build penuh, dan smoke test manual. Hook `Manager.Shutdown` saat daemon berhenti dibiarkan untuk Wiring bila ada hook yang tersedia; fitur ini tidak mengedit `main.go`/`modules.go`.

## Selesai bila

- `go build ./internal/session/... ./cmd/...` sukses (terhadap stub Foundation atau implementasi `session-manager` bila sudah ada). Opsional: `go vet ./internal/session/... ./cmd/aitermd/... ./cmd/aitermctl/...`. Kegagalan build akibat file fitur lain yang setengah jadi bukan urusan fitur ini.
- `grep -rn "internal/ipc" internal/session/` hanya cocok di `internal/session/ipc.go`.
- `cmd_session.go` tidak mengimpor `golang.org/x/term`, `creack/pty`, maupun `internal/session`; tidak ada `time.Ticker`/polling di ketiga file.
- `RegisterIPC` mendaftarkan tepat enam method dengan pemetaan error sesuai Kontrak; `session.list` kosong menghasilkan `{"sessions": []}`; `attach` selalu mengembalikan `scrollback` string base64 (boleh kosong) dan `offset`.
- `attach` di CLI: subscribe sebelum `session.attach`, membuang event `offset <= offset attach`, raw mode via `stty` (fallback mode baris), Ctrl+] melepas tanpa membunuh sesi, resize awal + SIGWINCH, exit code mengikuti sesi.
- Tidak ada file di luar ownedPaths yang dibuat atau diubah (`go.mod`/`go.sum` tidak berubah oleh fitur ini); tanpa unit test.
