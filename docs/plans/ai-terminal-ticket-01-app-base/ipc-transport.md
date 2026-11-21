# Plan: Transport IPC JSON-RPC

## Metadata

- Tiket: `ai-terminal-ticket-01-app-base` (`docs/tickets/ai-terminal-ticket-01-app-base.md`)
- Fitur: `ipc-transport` (Transport IPC JSON-RPC)
- ownedPaths: `internal/ipc/`
- Depends on foundation: **ya**. Foundation sudah menyediakan `go.mod` (`module aiterm`) dan `internal/ipc/stub.go` (signature Kontrak, badan `panic("belum diimplementasi")`). Plan ini menggantikan stub itu.
- Bahasa: pesan galat untuk pengguna dalam bahasa Indonesia; identifier kode bahasa Inggris. Hanya stdlib Go (tanpa dependensi baru).

Baca `overview.md` (bagian 2 untuk signature, bagian 6-7 untuk guard dan aturan paralel) dan bagian "Kontrak > IPC" di tiket sebagai sumber kebenaran. Plan ini tidak mengulangnya; hanya menjabarkan cara implementasi.

## Tujuan

Mengimplementasikan seluruh paket `internal/ipc` (`package ipc`): server dan client JSON-RPC 2.0 lewat unix socket, satu pesan per baris, pub/sub notifikasi `event` per koneksi, method bawaan `ping`, `daemon.info`, `events.subscribe`, `events.unsubscribe`. Pesan besar (beberapa MB) aman karena pembacaan memakai `bufio.Reader.ReadBytes('\n')`. Socket dibuat 0600, socket basi dibersihkan saat start, daemon ganda ditolak dengan "daemon sudah berjalan", socket dihapus saat shutdown. Tanpa polling dan tanpa timer interval: semuanya digerakkan I/O dan channel.

## Path yang dimiliki

HANYA `internal/ipc/` yang boleh disentuh:
- hapus `internal/ipc/stub.go`
- buat `internal/ipc/errors.go`, `protocol.go`, `builtin.go`, `server.go`, `client.go`

Dilarang: menyentuh `go.mod`/`go.sum` (jangan `go mod tidy`/`go get`), `internal/paths/`, `cmd/`, `app/`, `frontend/`, `Makefile`, atau file wiring apa pun. Dilarang mengubah signature Kontrak. File smoke test sementara boleh dibuat di luar ownedPaths (lihat Langkah 8) tetapi WAJIB dihapus sebelum selesai.

## Kontrak

Disediakan (signature persis seperti overview bagian 2; jangan diubah):

```go
type Handler func(ctx context.Context, params json.RawMessage) (any, error)
type Error struct { Code int; Message string; Data any }  // implements error
func NewError(code int, msg string) *Error
func Errorf(code int, format string, args ...any) *Error
const ( ErrBinary = -32000; ErrConflict = -32001; ErrNotFound = -32002; ErrInvalid = -32003 )
const ( ErrParse = -32700; ErrInvalidRequest = -32600; ErrMethodNotFound = -32601; ErrInvalidParams = -32602; ErrInternal = -32603 )
func NewServer(socketPath string, logger *slog.Logger) *Server
func (s *Server) Register(method string, h Handler)      // panic bila nama ganda
func (s *Server) Publish(topic string, payload any)
func (s *Server) Serve(ctx context.Context) error
func (s *Server) Methods() []string                      // terurut
func (s *Server) SetInfo(fn func() map[string]any)
func Dial(socketPath string) (*Client, error)
func (c *Client) Call(method string, params any, out any) error
func (c *Client) Subscribe(topicPrefix string, fn func(topic string, payload json.RawMessage)) (unsubscribe func(), err error)
func (c *Client) Close() error
```

Pesan di kabel (juga di overview "Pesan IPC"):
- Request: `{"jsonrpc":"2.0","id":<n>,"method":"...","params":...}`; tanpa `id` = notifikasi (tanpa respons).
- Event server ke klien: `{"jsonrpc":"2.0","method":"event","params":{"topic":"...","payload":...}}`.
- `ping` -> `{"pong":true,"ts":"<RFC3339Nano>"}`.
- `daemon.info` -> `{version,pid,startedAt,dataDir,socket,modules,methods}`.
- `events.subscribe` / `events.unsubscribe` params `{"prefix":string}`; hasil `{}`.

Dipakai: hanya stdlib. Tidak memakai `internal/paths` (path socket diberikan pemanggil lewat argumen).

## Langkah

Kerjakan berurutan. Semua file `package ipc`.

### 1. `errors.go`
- Konstanta kode (dua blok `const` di atas), persis nama dan nilainya.
- `type Error struct { Code int \`json:"code"\`; Message string \`json:"message"\`; Data any \`json:"data,omitempty"\` }`.
- `func (e *Error) Error() string` mengembalikan `e.Message` (bila `Code != 0` boleh diawali kode, mis. `fmt.Sprintf("%s (kode %d)", ...)`; pilih satu dan konsisten).
- `NewError`, `Errorf` (`fmt.Sprintf`).
- Helper internal `toError(err error) *Error`: `errors.As` ke `*Error` dipetakan apa adanya; selain itu `&Error{Code: ErrInternal, Message: err.Error()}`.

### 2. `protocol.go`
- `const version = "2.0"` (nama bebas, unexported).
- `type message struct` satu struct untuk semua arah agar mudah di-decode: `JSONRPC string \`json:"jsonrpc"\``, `ID json.RawMessage \`json:"id,omitempty"\``, `Method string \`json:"method,omitempty"\``, `Params json.RawMessage \`json:"params,omitempty"\``, `Result json.RawMessage \`json:"result,omitempty"\``, `Error *Error \`json:"error,omitempty"\``. Catatan: respons error untuk parse error memerlukan `"id":null`; tulis helper `errorResponse(id json.RawMessage, e *Error) []byte` yang membangun JSON dengan `id` diisi `null` bila `len(id)==0` (struct terpisah `response` tanpa `omitempty` pada `ID` dan `Result` `json.RawMessage` `omitempty` boleh dipakai untuk ini; sukses selalu menyertakan `result`, gunakan literal `null` bila hasil handler nil).
- `type eventParams struct { Topic string \`json:"topic"\`; Payload json.RawMessage \`json:"payload"\` }`.
- `type prefixParams struct { Prefix string \`json:"prefix"\` }`.
- `func readMessage(r *bufio.Reader) ([]byte, error)`: `ReadBytes('\n')`, trim `\r\n`/spasi; bila baris kosong lewati (loop); bila `io.EOF` dengan data sisa tanpa newline, kembalikan data itu dahulu lalu EOF pada pemanggilan berikutnya (atau abaikan sisa; pilih yang sederhana, tetapi jangan panik). Tidak ada batas ukuran. Buat reader dengan `bufio.NewReaderSize(conn, 64*1024)`.
- `func encodeLine(v any) ([]byte, error)`: `json.Marshal` lalu tambah `'\n'` (json.Marshal tidak pernah menghasilkan newline mentah, jadi aman satu pesan per baris).

### 3. `builtin.go`
- `func (s *Server) registerBuiltins()` dipanggil dari `NewServer`; mendaftarkan lewat `s.Register`:
  - `ping`: abaikan params; kembalikan `map[string]any{"pong": true, "ts": time.Now().Format(time.RFC3339Nano)}`.
  - `daemon.info`: bangun map dasar `{"pid": os.Getpid(), "socket": s.path, "modules": []string{}}`, timpa/gabung dengan hasil `s.info()` bila `SetInfo` sudah dipanggil (baca di bawah lock), lalu set `"methods": s.Methods()` (selalu milik ipc, menimpa apa pun dari info). Pastikan `modules` tidak nil (JSON `[]`, bukan `null`); bila info memberi `[]string(nil)`, ganti dengan slice kosong.
  - `events.subscribe` / `events.unsubscribe`: decode `prefixParams` (gagal -> `ErrInvalidParams`), ambil koneksi dari ctx lewat `connFromContext(ctx)` (tidak ada -> `ErrInternal`), panggil `c.subscribe(prefix)` / `c.unsubscribe(prefix)`, hasil `map[string]any{}`. Idempoten: subscribe ganda prefix sama tetap satu entri; unsubscribe prefix yang tak ada tidak error.
- Subscription per koneksi disimpan sebagai `map[string]struct{}` di `conn` yang dijaga mutex.

### 4. `server.go`
Tipe:
```go
type Server struct {
    path   string
    logger *slog.Logger          // bila nil pakai slog.Default()
    mu     sync.RWMutex          // jaga handlers, info, conns
    handlers map[string]Handler
    info   func() map[string]any
    conns  map[*conn]struct{}
}
type conn struct {
    srv    *Server
    nc     net.Conn
    ctx    context.Context; cancel context.CancelFunc
    writeMu sync.Mutex           // serialisasi penulisan ke nc
    events chan []byte           // buffer event, cap 256
    subMu  sync.Mutex; subs map[string]struct{}
    closeOnce sync.Once
}
```
- `NewServer`: isi field, `registerBuiltins()`.
- `Register`: panic dengan pesan jelas (mis. `"ipc: method sudah terdaftar: "+method`) bila nama ganda atau `h == nil`. Boleh dipanggil sebelum maupun sesudah `Serve` (lock).
- `Methods`: salin kunci handlers, `sort.Strings`.
- `SetInfo`: simpan fungsi di bawah lock.
- `Serve(ctx)`:
  1. Bila file di `s.path` ada: `net.DialTimeout("unix", path, ...)` (timeout pendek hanya untuk dial lokal, bukan polling). Bila dial sukses, tutup lalu kembalikan `NewError(ErrInternal, "daemon sudah berjalan")`-setara (cukup `fmt.Errorf("daemon sudah berjalan")`; pesan harus memuat teks persis "daemon sudah berjalan"). Bila dial gagal, anggap socket basi: `os.Remove(path)` (abaikan `ENOENT`).
  2. `net.Listen("unix", path)`; error dibungkus dengan konteks bahasa Indonesia. Segera `os.Chmod(path, 0o600)` (gagal -> tutup listener dan return error).
  3. `ctx` sudah batal? Lanjut ke penutupan. Jalankan goroutine yang menunggu `<-ctx.Done()` lalu `ln.Close()` (membangunkan `Accept`; ini event-driven, bukan polling). Pastikan goroutine ini juga berhenti bila `Serve` keluar karena error Accept (gunakan channel `done` yang ditutup via `defer`).
  4. Loop `Accept`: bila error dan `ctx.Err() != nil` -> keluar loop normal; error sementara (`net.Error` Temporary) -> log dan lanjut; error lain -> keluar dan return error. Tiap koneksi: buat `conn`, daftarkan ke `s.conns`, `wg.Add(1)`, `go c.serve()`.
  5. Penutupan: `ln.Close()`, tutup semua conn (`c.close()`), `wg.Wait()`, `os.Remove(path)` (abaikan error), log "ipc berhenti". Return `nil` bila berhenti karena ctx, selain itu error.
- `conn.serve()`: `defer` close + hapus dari `s.conns` + `wg.Done`. Jalankan goroutine penulis event: `for { select { case b := <-c.events: c.write(b); case <-c.ctx.Done(): return } }`. Loop baca: `readMessage`; tiap pesan ditangani di goroutine sendiri (`go c.handle(line)`), dengan `ctx` konteks koneksi yang diberi nilai `conn` lewat `context.WithValue(c.ctx, connKey{}, c)` (`type connKey struct{}`; `connFromContext` di `builtin.go` atau `server.go`). Saat baca gagal (EOF/putus) -> `c.close()`, yang membatalkan ctx sehingga handler yang sedang berjalan tahu koneksi putus.
- `conn.handle(line)`:
  - Bila `line` diawali `[` (batch): balas `ErrInvalidRequest` "batch tidak didukung" dengan id null.
  - `json.Unmarshal` ke `message`; gagal -> respons `ErrParse` id null.
  - Validasi `JSONRPC == "2.0"` dan `Method != ""`; salah -> `ErrInvalidRequest` (hanya dibalas bila ada id; id kosong -> id null).
  - Cari handler di bawah RLock; tidak ada -> `ErrMethodNotFound` ("method tidak dikenal: x") bila request punya id.
  - Panggil handler dengan `recover()` (panic -> `ErrInternal`, log stack singkat). Notifikasi (`len(ID)==0`): jalankan handler, JANGAN kirim respons apa pun (termasuk error).
  - Hasil: `json.Marshal(res)`; gagal -> `ErrInternal`. Error handler lewat `toError`. Kirim respons lewat `c.write`.
- `conn.write(b []byte)`: lock `writeMu`, tulis penuh ke `nc` (loop hingga semua byte terkirim; `net.Conn.Write` sudah menjamin atau error); error -> `c.close()`.
- `conn.close()`: `closeOnce.Do`: `cancel()`, `nc.Close()`.
- `conn.subscribe/unsubscribe/matches(topic)`: `matches` benar bila ada prefix `p` dengan `strings.HasPrefix(topic, p)` (prefix kosong cocok semua). Koneksi tanpa langganan tidak cocok apa pun.
- `Publish(topic, payload)`: marshal sekali notifikasi event (`message{JSONRPC:"2.0", Method:"event", Params: marshal(eventParams{topic, payload marshaled})}` + `\n`); gagal marshal -> log error dan return. Ambil snapshot conns di bawah RLock; untuk tiap conn yang `matches`: non-blocking `select { case c.events <- b: default: log peringatan "klien lambat, koneksi dijatuhkan"; c.close() }`. `Publish` tidak pernah memblokir. Aman dipanggil dari goroutine mana pun, termasuk dari dalam handler.

### 5. `client.go`
```go
type Client struct {
    nc       net.Conn
    writeMu  sync.Mutex
    mu       sync.Mutex                 // jaga pending, subs, closed, closeErr
    nextID   int64                      // atomic
    pending  map[int64]chan *message
    subs     map[uint64]*subscription   // {prefix string; fn func(string, json.RawMessage)}
    nextSub  uint64
    closed   bool
    closeErr error
    queue    ... // antrean event untuk dispatcher
    done     chan struct{}
}
```
- `Dial(socketPath)`: `net.Dial("unix", path)`; error dibungkus `fmt.Errorf("daemon tidak berjalan di %s: %w", path, err)` (pesan Indonesia, memuat teks "daemon tidak berjalan di <socket>"). Mulai goroutine `readLoop` dan `dispatchLoop`.
- `readLoop`: `readMessage` berulang. Pesan dengan `Method == "event"` dan tanpa id -> parse `eventParams`, masukkan ke antrean dispatcher. Pesan dengan id -> parse id sebagai int64, ambil dan hapus channel dari `pending`, kirim `*message` (channel buffered 1). Baris rusak: log/abaikan, lanjut. Saat baca gagal: `c.fail(err)`.
- `fail(err)`: di bawah lock, set `closed`, `closeErr` (`fmt.Errorf("koneksi ke daemon terputus")` bila bukan hasil `Close()` eksplisit, yang memakai "koneksi ditutup"), kirim error ke SEMUA pending (tutup channel atau kirim respons sintetis; pastikan tiap `Call` yang menunggu bangun dengan error), kosongkan pending, tutup `done` (sekali), tutup `nc`.
- Dispatcher event: antrean tak berbatas (slice dijaga mutex + channel sinyal berkapasitas 1) agar callback yang lambat atau yang memanggil `c.Call` tidak mem-block `readLoop` (hindari deadlock). `dispatchLoop` mengambil event berurutan dan memanggil setiap `subscription` yang `HasPrefix(topic, prefix)` (salin snapshot subs di bawah lock, panggil di luar lock). Berhenti saat `done` ditutup. Event dikirim berurutan sesuai kedatangan.
- `Call(method, params, out)`: bila `closed` -> kembalikan `closeErr`. `id := atomic.AddInt64(&c.nextID, 1)`; daftarkan channel di `pending`; marshal `params` (nil -> tanpa field `params`); tulis di bawah `writeMu` (gagal -> hapus pending, `c.fail`, return error). Tunggu channel. Respons `Error != nil` -> return `*Error` (Code/Message/Data utuh). Sukses: bila `out != nil` dan `len(Result) > 0`, `json.Unmarshal(Result, out)`. Aman konkuren; tidak ada lock yang dipegang saat menunggu.
- `Subscribe(prefix, fn)`: tambah `subscription` lokal di bawah lock, hitung apakah ada langganan lain dengan prefix sama (`first := ...`). Bila `first`, `Call("events.subscribe", prefixParams{prefix}, nil)`; gagal -> cabut entri lokal, return error. `unsubscribe` (dibungkus `sync.Once`): cabut entri; bila tidak ada callback lain dengan prefix itu, `Call("events.unsubscribe", ...)` dan abaikan error (koneksi mungkin sudah putus).
- `Close()`: tandai penutupan eksplisit lewat `fail(errClosed)`, kembalikan nil; aman dipanggil berulang.

### 6. Hapus stub
Hapus `internal/ipc/stub.go` setelah semua file di atas ada (hindari deklarasi ganda). Pastikan tidak ada `panic("belum diimplementasi")` tersisa di `internal/ipc/`.

### 7. Verifikasi build
Jalankan `gofmt -l internal/ipc` (harus kosong), `go build ./internal/ipc/...`, dan `go vet ./internal/ipc/...`. Perbaiki sampai bersih. Jangan menjalankan `go build ./...` sebagai syarat (fitur paralel lain bisa setengah jadi).

### 8. Smoke test manual (sementara, di luar ownedPaths)
Buat `tmp_ipc_smoke/main.go` di root repo (`package main`, import `aiterm/internal/ipc`; `internal` dapat diimpor dari dalam modul yang sama). Pakai direktori pendek `os.MkdirTemp("", "ipc")` untuk socket (hindari batas ~104 byte). Jalankan dengan `go run ./tmp_ipc_smoke` dan periksa:
1. `ping` dan `daemon.info` (ada `methods` berisi empat method bawaan plus yang terdaftar, `modules` `[]`).
2. Method tak dikenal -> `*ipc.Error` kode -32601; kirim JSON rusak lewat `net.Dial` mentah -> respons -32700; request tanpa `id` -> tidak ada respons.
3. Handler mengembalikan `ipc.NewError(ipc.ErrNotFound, "x")` -> klien menerima `*ipc.Error` kode -32002.
4. Dua klien: A `Subscribe("a.")`, B `Subscribe("")`, C tanpa langganan; `Publish("a.x", ...)` sampai ke A dan B saja; `Publish("b.x", ...)` hanya ke B.
5. Payload besar (mis. string 8 MB) lewat `Call` echo dan lewat `Publish`, diterima utuh.
6. `Register` nama ganda panik (recover).
7. Mode file socket 0600 (`os.Stat`); `Serve` kedua pada socket yang sama mengembalikan error berisi "daemon sudah berjalan"; socket basi (buat file socket tanpa listener, mis. listen lalu tutup tanpa unlink via `net.UnixListener.SetUnlinkOnClose(false)`) dibersihkan saat `Serve` baru; setelah ctx batal file socket hilang.
8. 100 `Call` konkuren dari banyak goroutine semuanya benar; saat server dihentikan, `Call` yang sedang menunggu (handler yang sengaja diblok) kembali dengan error, bukan menggantung.
Setelah selesai HAPUS `tmp_ipc_smoke/` seluruhnya dan file socket sementara.

## Perubahan

- Dihapus: `internal/ipc/stub.go`
- Dibuat: `internal/ipc/errors.go`, `internal/ipc/protocol.go`, `internal/ipc/builtin.go`, `internal/ipc/server.go`, `internal/ipc/client.go`
- Tidak ada perubahan di luar `internal/ipc/` (file smoke sementara dihapus).

## Wiring

Fitur ini tidak menyambungkan apa pun sendiri. Yang diekspos untuk langkah wiring dan fitur lain (`daemon-hello`, `wails-hello-ui`):
- `ipc.NewServer`, `Register`, `Publish`, `Serve`, `Methods`, `SetInfo` untuk `cmd/aitermd` (`main.go` memanggil `SetInfo` dengan version/pid/startedAt/dataDir/socket/modules dan `wireAll` sebelum `Serve`; `Serve` mengembalikan error berisi "daemon sudah berjalan" untuk daemon ganda).
- `ipc.Dial`, `Call`, `Subscribe`, `Close` untuk `cmd/aitermctl` dan `app/bridge.go` (pesan galat Dial memuat "daemon tidak berjalan di <socket>").
- Kode dan tipe galat (`ipc.Error`, `ErrBinary`, `ErrConflict`, `ErrNotFound`, `ErrInvalid`, kode standar) untuk modul epic lain.
- Wiring checklist overview bagian 5 butir 1 (tidak ada stub tersisa) dipenuhi oleh penghapusan `stub.go` di sini; sisanya dikerjakan agent wiring.

## Selesai bila

- `internal/ipc/stub.go` terhapus; hanya `errors.go`, `protocol.go`, `builtin.go`, `server.go`, `client.go` yang ada di `internal/ipc/`.
- `go build ./internal/ipc/...` sukses; `go vet ./internal/ipc/...` bersih; `gofmt -l internal/ipc` kosong.
- Signature publik persis sesuai Kontrak (overview bagian 2), termasuk `SetInfo`, dan sembilan konstanta kode.
- Seluruh butir smoke test Langkah 8 lulus, dan `tmp_ipc_smoke/` serta file socket sementara sudah dihapus.
- Tidak ada file di luar `internal/ipc/` yang berubah; tidak ada polling/timer interval; tidak ada dependensi non-stdlib.
