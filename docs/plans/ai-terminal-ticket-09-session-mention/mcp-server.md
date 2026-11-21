# Plan: Server MCP lokal

## Metadata

- Tiket: `ai-terminal-ticket-09-session-mention` (`docs/tickets/ai-terminal-ticket-09-session-mention.md`)
- Fitur: `mcp-server` (Server MCP lokal)
- ownedPaths: `internal/mention/mcp.go`, `internal/mention/tools.go`, `internal/mention/install.go`
- Depends on foundation: ya. `internal/mention/types.go` sudah final dari Foundation (`00-foundation.md`) dan terkunci. Baca `overview.md` di folder ini (bagian 1.2 butir 2, 3, 8; 2.2 sampai 2.4; 2.6; 2.7; 7) sebelum mulai; plan ini tidak mengulangnya.

## Tujuan

Menyediakan server MCP loopback milik daemon (stdlib `net/http` + `encoding/json`) yang dipasang ke tiap sesi lewat `provider.RegisterLaunchContribution`. Server berbicara JSON-RPC 2.0 (`initialize`, `notifications/initialized`, `tools/list`, `tools/call`), mengekspos tiga tool (`list_sessions`, `send_message`, `read_output`), mengautentikasi pemanggil lewat token bearer per sesi (token tertunda lalu diikat ke sesi pada request pertama), dan mengisi hook `startMCP` lewat `init()` di `mcp.go`. Fitur ini tidak berisi logika antrean: `send_message` hanya memvalidasi, `Namer.Resolve`, `Enqueuer.Enqueue`, lalu `QueueReader.Position`.

## Path yang dimiliki

HANYA tiga file ini yang boleh dibuat/diubah:

- `internal/mention/mcp.go`
- `internal/mention/tools.go`
- `internal/mention/install.go`

Dilarang menyentuh: `internal/mention/types.go` (Foundation), `queue.go`, `ipc.go`, `cmd/aitermd/wire_mention.go`, `cmd/aitermctl/cmd_mention.go`, `frontend/`, `internal/provider|session|status|workspace|ipc|paths`, `go.mod`, `go.sum`, file wiring apa pun. Bila butuh perubahan di sana, cari solusi lokal di tiga file ini dan catat sebagai catatan di laporan. Stdlib saja (plus `aiterm/internal/session` dan `aiterm/internal/provider`); jangan mengimpor `internal/status`, `internal/ipc`, atau fitur lain.

## Kontrak

### Disediakan (ke wiring/fitur lain)

- `func init()` di `mcp.go`: `startMCP = startServer`. Dengan itu `mention.Start(ctx, rt)` (sudah ada di `types.go`, dipanggil `wire_mention.go`) otomatis menjalankan server dan mengembalikan URL.
- `func startServer(ctx context.Context, rt *Runtime) (string, error)`: bind `127.0.0.1:0`, kembalikan `http://127.0.0.1:PORT/mcp`; `Start` yang memanggil `setURL`, jadi `startServer` TIDAK memanggil `setURL`.
- `func (m *tokenManager) BoundCount() int` melalui fungsi paket `func BoundSessionCount() int`: jumlah token terikat (dipakai `mention.info` field `sessions`). `ipc.go` milik fitur lain belum tentu memanggilnya; cukup sediakan dan dokumentasikan (lihat Wiring).
- `LaunchContribution` terdaftar tepat sekali (`sync.Once`) dari `startServer`.

### Dipakai (dari `types.go`, jangan didefinisikan ulang)

`Runtime{DataDir, Logger, Sessions, Namer, Enqueuer, Reader, OnSessionExit}`, `SessionSource`, `Namer` (`NameOf`, `Resolve`, `List`), `NameInfo`, `Message`, `Enqueuer`, `QueueReader`, konstanta `SourceAgent`, sentinel `ErrNotFound/ErrInvalid/ErrConflict/ErrCrossWorkspace/ErrUserOnly`, `ErrInvalid` untuk `rt == nil`. Dari paket lain: `session.Session`, `session.StatusRunning`, `session.SessionSpec`, `session.ErrNotFound`, `provider.RegisterLaunchContribution`, `provider.LaunchContext`, `provider.Capabilities().MCP`, `provider.Get`, `provider.Default`.

### Protokol (rujuk overview 2.6)

URL, header `Authorization: Bearer`, 401, versi protokol `2025-03-26`, `serverInfo {name:"aiterm-mention", version:"1"}`, bentuk hasil `tools/call`, bentuk `MCPServers["aiterm-mention"]`. Ikuti persis.

## Langkah

### 1. Periksa prasyarat

- [ ] Pastikan `internal/mention/types.go` ada dan memuat `Runtime`, `Namer`, `Enqueuer`, `QueueReader`, `startMCP`, `Start`. Bila tidak ada, laporkan blocker (jangan membuatnya).
- [ ] `grep -n "type LaunchContext\|RegisterLaunchContribution" internal/provider/launch.go` dan `grep -n "func (m \*Manager) \(List\|Get\|Attach\)" internal/session/*.go` untuk memastikan signature sesuai overview 1.1.
- [ ] Pastikan `mcp.go`, `tools.go`, `install.go` belum ada; jangan menimpa file lain di `internal/mention/`.

### 2. `internal/mention/install.go` (token tertunda/terikat dan LaunchContribution)

Paket `mention`. Impor: `crypto/rand`, `encoding/hex`, `sort`, `sync`, `time`, `aiterm/internal/provider`, `aiterm/internal/session`.

- [ ] Tipe privat:

```go
type pendingToken struct {
    token       string
    workspaceID string
    projectID   string
    providerID  string
    cwd         string
    issuedAt    time.Time
}

type tokenManager struct {
    mu      sync.Mutex
    pending []pendingToken       // FIFO, tertua di indeks 0, maks maxPending
    bound   map[string]string    // token -> sessionID
    byID    map[string]string    // sessionID -> token (agar sesi tak terikat ganda)
}

const maxPending = 64
```

- [ ] `func newTokenManager() *tokenManager` (inisialisasi map).
- [ ] `func newToken() string`: 32 byte `crypto/rand`, `hex.EncodeToString`. Bila `rand.Read` gagal, kembalikan `""` dan pemanggil tidak memasang MCP (log warn).
- [ ] `func (t *tokenManager) issue(spec session.SessionSpec, providerID string) string`: buat token, tambahkan `pendingToken` di akhir (`issuedAt: time.Now()`), bila panjang `> maxPending` buang yang tertua (indeks 0). Kembalikan token. `providerID` diisi `spec.ProviderID`, bila kosong pakai ID provider hasil kontribusi (`ctx.Provider.ID()`).
- [ ] `func (t *tokenManager) lookup(token string, rt *Runtime) (sessionID string, ok bool)`: logika inti autentikasi, urutan:
  1. Kunci mutex. Bila `token` ada di `bound`: cek malas `rt.Sessions.Get(sid)`; bila error (`session.ErrNotFound` atau lainnya) atau `Status != session.StatusRunning`, panggil `releaseLocked(sid)` dan kembalikan `("", false)`; selain itu kembalikan `(sid, true)`.
  2. Bila token tidak di `bound`, cari di `pending`; bila tidak ada, `("", false)`.
  3. Bila ada di `pending`, jalankan `bindPendingLocked(rt)` (lihat bawah), lalu cek lagi `bound[token]`; bila masih belum terikat (sesi belum tampak di `List`), kembalikan `("", false)` (klien akan mendapat 401 dan mencoba lagi; token tertunda tetap).
- [ ] `func (t *tokenManager) bindPendingLocked(rt *Runtime)`: pengikatan FIFO menurut overview 1.2 butir 2.
  1. `sessions := rt.Sessions.List()`; saring `Status == session.StatusRunning` dan `byID[s.ID] == ""` (belum terikat). Urut sudah `CreatedAt` dari `List`; tetap `sort.SliceStable` menurut `CreatedAt` lalu `ID`.
  2. Kelompokkan menurut kunci `workspaceID|projectID|providerID|cwd`. Untuk tiap token tertunda secara urut FIFO (`pending` dari indeks 0), ambil sesi tak-terikat tertua dengan kunci yang sama dan belum diambil pada putaran ini; bila ada: `bound[token] = s.ID`, `byID[s.ID] = token`, hapus token dari `pending`. Token ke-n untuk kunci tertentu mendapat sesi ke-n. Token yang tak menemukan sesi tetap di `pending`.
  3. Tidak ada goroutine; ini murni dijalankan di dalam request.
- [ ] `func (t *tokenManager) release(sessionID string)`: kunci mutex, `releaseLocked`. `releaseLocked(sid)`: hapus `byID[sid]` dan `bound[token]` terkait.
- [ ] `func (t *tokenManager) boundCount() int`.
- [ ] Variabel paket `var tokens = newTokenManager()` dan `func BoundSessionCount() int { return tokens.boundCount() }` dengan komentar bahwa dipakai `mention.info`.
- [ ] `var contribOnce sync.Once` dan `func registerLaunchContribution(rt *Runtime, url string)`: dalam `contribOnce.Do`, panggil `provider.RegisterLaunchContribution(func(ctx *provider.LaunchContext) { ... })`. Isi callback:
  - Bila `ctx == nil` atau `ctx.Provider == nil` atau `!ctx.Provider.Capabilities().MCP`: return tanpa membuat token (overview 1.2 butir 8).
  - Ambil URL aktif lewat `URL()` (bukan argumen yang membeku) dan bila `""` return.
  - `tok := tokens.issue(ctx.Session, ctx.Provider.ID())`; bila `tok == ""` return.
  - Bila `ctx.MCPServers == nil` buat map baru; isi `ctx.MCPServers["aiterm-mention"] = map[string]any{"type": "http", "url": url, "headers": map[string]any{"Authorization": "Bearer " + tok}}`.
  - Jangan menyentuh `ctx.Env`/`ctx.Args`.
  - Komentar Indonesia yang menjelaskan keterbatasan: `SessionSpec` belum punya ID saat dipanggil (`Manager.Create` memanggil `launcher.Command` sebelum `newID()`), sehingga token baru diikat ke sesi pada request MCP pertama; catat sebagai catatan untuk tiket 02/08.
- [ ] Pendaftaran pelepasan token: `func hookSessionExit(rt *Runtime)`: bila `rt.OnSessionExit != nil` panggil `rt.OnSessionExit(tokens.release)`. Dipanggil `startServer` sekali.

### 3. `internal/mention/tools.go` (tiga tool dan resolusi pemanggil)

Impor: `context`, `encoding/json`, `errors`, `fmt`, `regexp`, `strings`, `aiterm/internal/session`.

- [ ] Tipe definisi tool dan hasil:

```go
type toolDef struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    InputSchema map[string]any `json:"inputSchema"`
}

type toolResult struct {
    Content []toolContent `json:"content"`
    IsError bool          `json:"isError"`
}
type toolContent struct {
    Type string `json:"type"` // "text"
    Text string `json:"text"`
}
```

- [ ] `func toolDefs() []toolDef` mengembalikan tepat tiga tool, urut `list_sessions`, `send_message`, `read_output`, dengan `inputSchema` JSON Schema valid:
  - `list_sessions`: `{"type":"object","properties":{},"additionalProperties":false}`; deskripsi Indonesia singkat.
  - `send_message`: properti `to` (string, nama mention atau id sesi) dan `text` (string), `required: ["to","text"]`.
  - `read_output`: `to` (string), `last_n` (integer, `minimum` 1, `maximum` 500, default 50); `required: ["to"]`.
- [ ] `type caller struct { SessionID, WorkspaceID string }` dan `func resolveCaller(rt *Runtime, sessionID string) (caller, error)`: `rt.Sessions.Get(sessionID)`; gagal atau tidak `running` -> `ErrNotFound`.
- [ ] `func callTool(ctx context.Context, rt *Runtime, c caller, name string, args json.RawMessage) toolResult`: dispatch menurut `name`; tool tak dikenal -> `toolError(fmt.Errorf("tool tidak dikenal: %s", name))`. Semua error tool dikembalikan sebagai `toolResult{IsError:true, Content:[{text: err.Error()}]}` (helper `toolError(err error) toolResult`, `toolText(s string) toolResult`, `toolJSON(v any) toolResult` yang marshal ke teks).
- [ ] `list_sessions`: `infos := rt.Namer.List(c.WorkspaceID)`; petakan ke
  `type sessionEntry struct { ID string "json:\"id\""; Name string "json:\"name\""; ProjectName string "json:\"projectName\""; ProviderID string "json:\"providerId\""; Status string "json:\"status\""; Self bool "json:\"self\"" }`,
  `Self = info.SessionID == c.SessionID`. Hasil slice tidak pernah nil (`[]sessionEntry{}`), `toolJSON`.
- [ ] `send_message`: parse `args` ke `struct{To, Text string}` (tag `to`, `text`); JSON rusak -> error "argumen tidak valid". Validasi: `strings.TrimSpace(To) == ""` atau `strings.TrimSpace(Text) == ""` -> `ErrInvalid` dengan pesan jelas ("to wajib diisi", "text wajib tidak kosong"). Lalu:
  1. `target, err := rt.Namer.Resolve(c.WorkspaceID, To)`; error diteruskan apa adanya (`ErrNotFound` "sesi tidak ditemukan", `ErrCrossWorkspace` "bukan workspace yang sama").
  2. Tolak mengirim ke diri sendiri: `target == c.SessionID` -> error "tidak dapat mengirim pesan ke sesi sendiri".
  3. `msg, err := rt.Enqueuer.Enqueue(ctx, c.SessionID, target, Text, SourceAgent)`; error diteruskan apa adanya (`ErrUserOnly`, `ErrConflict`, dll; teks `err.Error()`).
  4. Hasil: `struct{Queued bool "json:\"queued\""; Delivered bool "json:\"delivered\""; Position int "json:\"position\""}`. `Delivered = msg.State == StateDelivered`; `Queued = msg.State == StateQueued || msg.State == StateAwaitingConfirm`; `Position = rt.Reader.Position(msg.ID)` bila `rt.Reader != nil` (0 bila terkirim). Tambahkan tidak ada field lain. `rt.Enqueuer == nil` -> error "antrean tidak tersedia".
- [ ] `read_output`: parse `struct{To string; LastN *int}` (tag `to`, `last_n`); angka dari klien bisa `float64` JSON, `*int` memadai karena `encoding/json` menolak pecahan, tangani sebagai error "last_n tidak valid". `To` kosong -> `ErrInvalid`. `last_n` nil atau `<= 0` -> 50; `> 500` -> 500. Resolusi: `rt.Namer.Resolve(c.WorkspaceID, To)`. Ambil `rt.Sessions.Attach(target)`; error `session.ErrNotFound` -> `ErrNotFound`; error lain dibungkus `fmt.Errorf("gagal membaca output: %w", err)`. Teks: `lastLines(stripANSI(string(res.Scrollback)), n)`. Kembalikan `toolText` (teks polos, string kosong bila tak ada output; tetap `isError:false`).
- [ ] `var ansiRe = regexp.MustCompile(...)` dan `func stripANSI(s string) string` yang membuang berurutan: OSC (`\x1b\][^\x07\x1b]*(\x07|\x1b\\)`), CSI (`\x1b\[[0-?]*[ -/]*[@-~]`), escape dua karakter lain (`\x1b[@-Z\\-_]` dan `\x1b[()][0-9A-Za-z]`), lalu normalisasi `\r\n` -> `\n`, buang `\r` yang tersisa (progress bar overwrite: untuk tiap baris ambil segmen setelah `\r` terakhir bila hasilnya tidak kosong), dan buang semua karakter kontrol selain `\n` dan `\t` (`r < 0x20` atau `0x7f`) lewat `strings.Map`. Pastikan UTF-8 tidak rusak (iterasi per rune; scrollback bisa terpotong di tengah rune, ganti byte tak valid lewat `strings.ToValidUTF8(s, "")` lebih dulu).
- [ ] `func lastLines(s string, n int) string`: pisah `\n`, buang trailing kosong terakhir (akibat `\n` penutup), ambil `n` terakhir, gabung `\n`.
- [ ] Hindari goroutine, timer, dan polling di seluruh file.

### 4. `internal/mention/mcp.go` (server HTTP + JSON-RPC)

Impor: `context`, `encoding/json`, `errors`, `fmt`, `log/slog` (hanya bila dipakai), `net`, `net/http`, `os`, `path/filepath`, `strings`, `time`.

- [ ] `func init() { startMCP = startServer }` dengan komentar bahwa ini mengisi hook dari `types.go` agar `wire_mention.go` tidak perlu diubah.
- [ ] Tipe JSON-RPC:

```go
type rpcRequest struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      json.RawMessage `json:"id,omitempty"` // kosong = notifikasi
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}
type rpcResponse struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      json.RawMessage `json:"id"`
    Result  any             `json:"result,omitempty"`
    Error   *rpcError       `json:"error,omitempty"`
}
type rpcError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}
```

  Konstanta kode: parse error -32700, invalid request -32600, method not found -32601, invalid params -32602.
- [ ] `func startServer(ctx context.Context, rt *Runtime) (string, error)`:
  1. `rt == nil || rt.Sessions == nil || rt.Namer == nil` -> `ErrInvalid` terbungkus pesan jelas; `rt.Logger == nil` -> `slog.Default()`.
  2. `ln, err := net.Listen("tcp", "127.0.0.1:0")`; gagal -> kembalikan error. Dilarang bind alamat lain.
  3. `url := "http://" + ln.Addr().String() + "/mcp"`.
  4. `mux := http.NewServeMux()`; `mux.HandleFunc("/mcp", handler(rt))`; `srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}`.
  5. Tulis `<DataDir>/mention.json` berisi `{"url":"<url>"}`: `os.MkdirAll(rt.DataDir, 0o755)` bila `DataDir != ""`, tulis ke berkas sementara lalu `os.Rename` (atomik); kegagalan hanya `Logger.Warn`, bukan error fatal. Jika `DataDir == ""` lewati.
  6. `registerLaunchContribution(rt, url)` dan `hookSessionExit(rt)`.
  7. Jalankan `go srv.Serve(ln)` (goroutine milik `http.Server`, bukan polling) dan satu goroutine penutup `go func(){ <-ctx.Done(); shutdown dengan timeout 2s }()` yang menunggu konteks (bukan polling; tidak berjalan bila `ctx.Done()` nil, yaitu `context.Background()`, goroutine tetap menunggu selamanya yang dapat diterima, namun hindari bila `ctx.Done() == nil`: cek dan lewati goroutine itu). Log `Logger.Info("server MCP mention aktif", "url", url)`.
  8. Kembalikan `(url, nil)`. Jangan panggil `setURL`.
- [ ] `func handler(rt *Runtime) http.HandlerFunc`:
  1. Method selain `POST`: `405` dengan header `Allow: POST` (termasuk GET dan DELETE); lakukan SEBELUM autentikasi? Tidak: autentikasi dulu agar tak membocorkan; urutan: cek method (405 untuk GET sesuai overview) lalu auth.
  2. Auth: ambil header `Authorization`, wajib awalan `Bearer ` (case-insensitive untuk "Bearer"), token `strings.TrimSpace`. `sid, ok := tokens.lookup(token, rt)`; `!ok` -> `401` dengan header `WWW-Authenticate: Bearer` dan body singkat. Bandingkan token di `lookup` lewat map (tidak perlu constant-time karena token acak 256 bit).
  3. Batasi body: `http.MaxBytesReader(w, r.Body, 1<<20)`; decode `rpcRequest`; JSON rusak -> balas rpcError -32700 (HTTP 200) dengan `id: null`. Batch (array) tidak didukung: bila body diawali `[` balas -32600.
  4. Notifikasi (tanpa `id`): untuk `notifications/initialized` dan notifikasi lain balas HTTP `202` tanpa body.
  5. Dispatch `Method`:
     - `initialize`: result `{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"aiterm-mention","version":"1"}}`.
     - `ping`: result `{}`.
     - `tools/list`: result `{"tools": toolDefs()}`.
     - `tools/call`: parse params `struct{Name string "json:\"name\""; Arguments json.RawMessage "json:\"arguments\""}`; gagal parse atau `Name == ""` -> -32602. `c, err := resolveCaller(rt, sid)`; error -> hasil `toolError(err)` (bukan error JSON-RPC). Lalu `callTool(r.Context(), rt, c, name, args)`; `Arguments` kosong dianggap `{}`.
     - lainnya: -32601 "method tidak ditemukan".
  6. Tulis respons `Content-Type: application/json`, `json.NewEncoder(w).Encode`. Selalu sertakan `jsonrpc:"2.0"` dan `id` apa adanya (`RawMessage`).
  7. Recover panic per request (`defer recover`) -> HTTP 500 dan log, supaya satu tool buruk tidak mematikan daemon.
- [ ] Tidak ada SSE; server hanya merespons JSON langsung (streamable HTTP mode non-stream).

### 5. Guard

- [ ] `gofmt -l internal/mention` tidak mengeluarkan nama berkas milik fitur ini.
- [ ] `go vet ./internal/mention/` bersih untuk tiga file ini (peringatan di file fitur lain yang belum ada dibiarkan).
- [ ] `go build ./...` lolos (fitur paralel mungkin belum ada; build tidak boleh bergantung pada `queue.go`, `ipc.go`, `wire_mention.go`).
- [ ] `grep -n "time.Tick\|NewTicker\|setInterval" internal/mention/mcp.go internal/mention/tools.go internal/mention/install.go` kosong.
- [ ] Pastikan tidak ada identifier ganda terhadap `types.go` (mis. jangan definisikan ulang `Enqueuer`, `Message`, `URL`, `setURL`).

## Perubahan

Dibuat:
- `internal/mention/install.go`: `pendingToken`, `tokenManager` (`issue`, `lookup`, `bindPendingLocked`, `release`, `boundCount`), `tokens`, `BoundSessionCount`, `registerLaunchContribution`, `hookSessionExit`.
- `internal/mention/tools.go`: `toolDef`, `toolResult`, `toolDefs`, `caller`, `resolveCaller`, `callTool`, tiga implementasi tool, `stripANSI`, `lastLines`.
- `internal/mention/mcp.go`: `init()`, `startServer`, `handler`, tipe JSON-RPC.

Diubah: tidak ada.

## Wiring

Tidak ada yang disambungkan oleh fitur ini. Yang diekspos untuk langkah wiring:

- `mention.Start(ctx, rt)` (dari `types.go`) otomatis memakai `startServer` karena `init()` di `mcp.go`; `wire_mention.go` (fitur `message-queue`) hanya perlu memberi `Runtime` terisi: `Sessions`, `Namer`, `Enqueuer`, `Reader`, `DataDir`, `Logger`, `OnSessionExit`. Bila `Runtime.OnSessionExit` nil, token tetap dilepas secara malas pada request berikutnya (`Sessions.Get` menunjukkan sesi hilang/exited).
- `mention.BoundSessionCount()` untuk field `sessions` pada `mention.info` (opsional; `ipc.go` boleh memakainya).
- `LaunchContribution` didaftarkan oleh `startServer` sekali; sesi baru dari provider ber-`Capabilities().MCP` mendapat `MCPServers["aiterm-mention"]`. Penerjemahan ke format per provider adalah tanggung jawab adapter tiket 05. Catatan untuk laporan: (1) keterbatasan `SessionSpec` tanpa ID saat launch (tiket 02/08); (2) bila adapter provider belum menerjemahkan `MCPServers`, laporkan kekurangan di tiket 05.
- Verifikasi manual setelah wiring (bukan guard): `curl -s -X POST <url> -H "Authorization: Bearer <token>" -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'` mengembalikan tiga tool; tanpa token 401; `GET` 405.

## Selesai bila

- Tiga file ada dan hanya tiga file itu yang berubah (`git status`/diff tidak menunjukkan path lain).
- `go build ./...` lolos; `gofmt -l` bersih untuk path fitur ini.
- Server bind hanya `127.0.0.1:0`, path `/mcp`; `mention.json` ditulis; `init()` mengisi `startMCP`.
- `tools/list` memuat tepat tiga tool dengan `inputSchema`; request tanpa/dengan token tak dikenal -> 401; `GET` -> 405; method tak dikenal -> -32601; `notifications/initialized` -> 202.
- `list_sessions` memuat sesi running workspace pemanggil dengan `self`; `send_message` hanya validasi + `Resolve` + `Enqueue` + `Position`, error lewat `isError:true`; `read_output` memakai default 50, maks 500, tanpa ANSI.
- Token tertunda FIFO (maks 64) diikat pada request pertama, dilepas saat exit (callback) dan secara malas; kontribusi tidak membuat token bila provider tanpa MCP.
- Tidak ada polling/ticker, tidak ada dependency baru, tidak ada perubahan di file terlarang.
- Laporan menyebut catatan keterbatasan token tertunda dan simbol ekspor baru (`BoundSessionCount`).
