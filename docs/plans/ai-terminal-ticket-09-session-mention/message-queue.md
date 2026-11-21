# Plan: Antrean pesan dan pengiriman saat idle

## Metadata

- Tiket: `ai-terminal-ticket-09-session-mention` (`docs/tickets/ai-terminal-ticket-09-session-mention.md`)
- Fitur: `message-queue` (Antrean pesan dan pengiriman saat idle)
- ownedPaths:
  - `internal/mention/queue.go`
  - `internal/mention/ipc.go`
  - `cmd/aitermd/wire_mention.go`
  - `cmd/aitermctl/cmd_mention.go`
- Depends on foundation: ya. `internal/mention/types.go` sudah dibuat Foundation (`00-foundation.md`) dan TIDAK boleh diedit di sini.
- Prasyarat lain: paket `aiterm/internal/status` (tiket 08) harus sudah ada (`status.IsIdle`, `status.OnIdle`). Bila belum ada, hentikan dan laporkan blocker; jangan membuat paket tiruan dan jangan menyentuh `go.mod`.

Baca dulu (jangan diubah): `overview.md` di folder plan ini (terutama 1.1, 1.2, 2.2 sampai 2.5, 2.8, 7), `internal/mention/types.go`, `cmd/aitermd/modules.go`, `cmd/aitermd/wire_session.go`, `cmd/aitermctl/registry.go`, `cmd/aitermctl/cmd_workspace.go`, `internal/session/ipc.go` (pola `decodeParams` dan `mapErr`), `internal/ipc/errors.go`.

## Tujuan

Menyediakan antrean pesan per sesi target yang mengirim lewat `Provider.SendInput` hanya saat target idle (digerakkan `status.OnIdle`, tanpa polling), dengan mode `auto` / `confirm_agent` (default) / `user_only`, batas 50 pesan per target, cooldown 1500ms setelah kirim, dan pembersihan saat `session.exit`. Menyediakan method IPC `mention.*`, event `mention.queue`, modul daemon `mention` (wiring semua dependensi di `d.OnReady`), dan perintah `aitermctl mention`.

## Path yang dimiliki

HANYA empat file ini yang boleh dibuat/diubah:

- `internal/mention/queue.go`
- `internal/mention/ipc.go`
- `cmd/aitermd/wire_mention.go`
- `cmd/aitermctl/cmd_mention.go`

Dilarang menyentuh: `internal/mention/types.go`, `mcp.go`, `tools.go`, `install.go`, `frontend/`, `cmd/aitermd/modules.go`, `main.go`, `cmd/aitermctl/registry.go`, `main.go`, `internal/provider|status|session|workspace|ipc|paths`, `go.mod`, `go.sum`. Tipe lokal tambahan didefinisikan di `queue.go`/`ipc.go`, bukan di `types.go`.

## Kontrak

Dipakai (lihat overview; jangan didefinisikan ulang): dari `types.go` Foundation: `State*`, `Source*`, `FromUser`, `Mode*`, `Message`, `NameInfo`, error `ErrNotFound/ErrInvalid/ErrConflict/ErrCrossWorkspace/ErrUserOnly`, `SessionSource`, `Directory`, `StatusFunc`, `Enqueuer`, `QueueReader`, `Runtime`, `Namer` (`NewNamer`, `NameOf`, `ResolveName`, `Resolve`, `List`), `NewMessageID`, `Start`, `URL`. Dari paket lain: `provider.Get/Default`, `Provider.SendInput(w io.Writer, text string) error`, `Provider.Capabilities().MCP`, `status.IsIdle(id) bool`, `status.OnIdle(id, fn) (cancel func())`, `session.Session`/`session.StatusRunning`, `ipc.Server.Register/Publish`, `ipc.Dial`, `Client.Call/Subscribe`.

Disediakan fitur ini:

```go
// queue.go
const (
    maxQueuePerTarget = 50
    sendCooldown      = 1500 * time.Millisecond
)

type QueueConfig struct {
    Sessions SessionSource
    Namer    *Namer
    Publish  func(topic string, payload any) // srv.Publish
    Logger   *slog.Logger                    // boleh nil -> slog.Default()
}

type Queue struct{ /* satu sync.Mutex */ }

func NewQueue(cfg QueueConfig) *Queue
func (q *Queue) Enqueue(ctx context.Context, from, to, text, source string) (Message, error) // memenuhi Enqueuer
func (q *Queue) Position(messageID string) int                                                // memenuhi QueueReader
func (q *Queue) Pending(sessionID string) []Message  // pending satu target, urut FIFO, tidak pernah nil
func (q *Queue) PendingAll() []Message               // semua target, urut CreatedAt lalu ID, tidak pernah nil
func (q *Queue) Cancel(id string) error
func (q *Queue) Confirm(id string) (Message, error)
func (q *Queue) Mode() Mode
func (q *Queue) SetMode(m Mode) error                // mode tak dikenal -> ErrInvalid
func (q *Queue) HandleExit(sessionID string)         // dipanggil saat session.exit

type sessionWriter struct{ mgr SessionSource; id string }
func (w sessionWriter) Write(b []byte) (int, error)  // Manager.Write(id, b); kembalikan len(b)

var _ Enqueuer = (*Queue)(nil)
var _ QueueReader = (*Queue)(nil)
```

```go
// ipc.go
type IPCOptions struct {
    SessionCount func() int // jumlah sesi ber-MCP untuk mention.info; nil -> 0
}
func RegisterIPC(srv *ipc.Server, q *Queue, namer *Namer, opts IPCOptions)
```

Method IPC dan event persis seperti overview 2.5 (`mention.send`, `mention.queue.list`, `mention.queue.cancel`, `mention.confirm`, `mention.names`, `mention.config`, `mention.info`; event `mention.queue` payload `{sessionId, queue: Message[]}` berisi seluruh antrean pending sesi itu, `[]` bila kosong).

Penegasan kecil agar CLI praktis: pada `mention.send`, `workspaceId` boleh kosong bila `to` adalah id sesi (workspace diambil dari sesi target). Bila kosong dan `to` bukan id sesi: `ErrInvalid` ("workspaceId wajib bila to berupa nama").

## Langkah

### 1. `internal/mention/queue.go`

Paket `mention`. Impor: `context`, `errors`, `fmt`, `io` (bila perlu), `log/slog`, `sort`, `strings`, `sync`, `time`, `aiterm/internal/provider`, `aiterm/internal/session`, `aiterm/internal/status`. Komentar dan pesan error Indonesia.

Struktur internal:

```go
type entry struct {
    msg     Message
    sending bool // sedang ditulis ke sesi; tidak bisa dibatalkan
}

type target struct {
    entries       []*entry
    armed         bool          // OnIdle sudah terdaftar (satu per target)
    cancelIdle    func()
    idleGen       uint64        // generasi pendaftaran; callback basi diabaikan
    cooldownUntil time.Time
    timer         *time.Timer   // timer cooldown sekali jalan, nil bila tidak ada
}

type Queue struct {
    cfg     QueueConfig
    mu      sync.Mutex
    mode    Mode                 // default ModeConfirmAgent
    targets map[string]*target
    byID    map[string]*entry    // semua entri pending untuk Position/Cancel/Confirm
    done    map[string]State     // id pesan selesai (delivered/failed) untuk membedakan ErrInvalid vs ErrNotFound
    doneIDs []string             // ring maksimum 200; buang yang tertua dari done
}
```

`NewQueue`: isi nilai default (`Logger`, `Publish` nil -> no-op), `mode = ModeConfirmAgent`, map terinisialisasi.

Sentinel timer: dilarang `time.Ticker`/`time.Tick`/loop tidur. Timer hanya `time.AfterFunc` sekali jalan untuk cooldown.

**`Enqueue(ctx, from, to, text, source)`**, urutan validasi:

1. `strings.TrimSpace(text) == ""` -> `ErrInvalid` (bungkus dengan pesan "teks kosong" via `fmt.Errorf("%w: teks kosong", ErrInvalid)`; pemetaan IPC memakai `errors.Is`).
2. Konsistensi sumber: `from == ""` mensyaratkan `source == SourceUser`; `from != ""` mensyaratkan `source == SourceAgent`; selain itu `ErrInvalid`.
3. Target: `cfg.Sessions.Get(to)`; error atau `Status != session.StatusRunning` -> `ErrNotFound`.
4. Bila `from != ""`: `from == to` -> `ErrInvalid` ("tidak bisa mengirim ke diri sendiri"); `Get(from)` gagal -> `ErrInvalid`; `WorkspaceID` beda -> `ErrCrossWorkspace`.
5. Mode (baca di bawah lock): `source == SourceAgent && mode == ModeUserOnly` -> `ErrUserOnly`.
6. Hitung `fromName`: `"user"` bila `from == ""`, selain itu `cfg.Namer.NameOf(from)` (kosong -> pakai `from`). Hitung SEBELUM mengambil lock.
7. Lock. `len(target.entries) >= maxQueuePerTarget` -> `ErrConflict`. Buat `Message{ID: NewMessageID(), From: from atau FromUser, FromName, To: to, Text, CreatedAt: time.Now().UTC(), Source: source}`. State awal: `StateAwaitingConfirm` bila `source == SourceAgent && mode == ModeConfirmAgent`, selain itu `StateQueued`. Tambahkan ke `entries` dan `byID`. Publish antrean target.
8. Unlock, lalu panggil `q.pump(to)` secara sinkron (bila pesan langsung terkirim, pemanggil melihat hasilnya). Setelah itu ambil lock sebentar, salin `msg` (state akhir `delivered`/`failed`/`queued`/`awaiting_confirm`; pesan sudah keluar dari `byID` bila final, jadi simpan pointer `e` lokal dan baca `e.msg`), kembalikan. State `failed` dikembalikan sebagai `Message` dengan `Error` terisi dan `err == nil` (kegagalan kirim bukan kegagalan validasi).

Pesan `awaiting_confirm` tidak memicu `pump` yang berarti apa-apa (pump mengabaikannya); tidak ada pendaftaran `OnIdle` sampai dikonfirmasi.

**`pump(to string)`** adalah satu-satunya jalur pengiriman (dipanggil dari Enqueue, Confirm, callback OnIdle, callback timer cooldown):

```
lock
t := targets[to]; bila nil -> return
cari e = entri pertama (urut FIFO) dengan state == StateQueued dan !sending
bila e == nil:
    disarm(t)        // batalkan OnIdle bila armed (panggil cancelIdle, armed=false, idleGen++), hentikan timer
    bersihkan target bila entries kosong
    return
bila sekarang < t.cooldownUntil:
    bila t.timer == nil: t.timer = time.AfterFunc(sisa, func(){ lock; if targets[to]==t { t.timer=nil }; unlock; q.pump(to) })
    return                   // setelah timer, pump mengevaluasi ulang IsIdle
bila !status.IsIdle(to):
    bila !t.armed: arm(to, t)   // satu pendaftaran per target
    return
// idle dan tidak cooldown: kirim
e.sending = true
unlock
err := q.deliver(e.msg)        // di luar lock
lock
e.sending = false
bila targets[to] != t (sesi sudah exit): return    // HandleExit sudah menandai failed
hapus e dari t.entries dan byID
bila err == nil: e.msg.State = StateDelivered; t.cooldownUntil = now + sendCooldown
selain itu: e.msg.State = StateFailed; e.msg.Error = err.Error()
catat id di done (ring 200)
publish antrean target
more := masih ada entri StateQueued
unlock
bila more: q.pump(to)    // akan jatuh ke cabang cooldown dan memasang SATU timer; bukan rekursi tak terbatas
```

Catatan penting:

- `arm(to, t)` (dipanggil di bawah lock): `t.idleGen++`; `gen := t.idleGen`; `t.armed = true`; `t.cancelIdle = status.OnIdle(to, func(){ go q.onIdle(to, gen) })`. Callback WAJIB hanya melempar ke goroutine (`go`) agar aman bila `OnIdle` memanggil `fn` sinkron saat sesi sudah idle; jangan mengambil lock `q.mu` langsung di callback.
- `onIdle(to, gen)`: lock; `t := targets[to]`; bila `t == nil || t.idleGen != gen` -> return (callback basi); `t.armed = false; t.cancelIdle = nil`; unlock; `q.pump(to)`.
- `disarm(t)`: bila `armed` panggil `cancelIdle` (boleh nil-check), set `armed=false`, `idleGen++`, hentikan `timer` bila ada.
- Pesan pertama target idle dan tanpa cooldown terkirim sinkron di dalam `Enqueue`. FIFO: `e` selalu entri `queued` terdepan; pesan `awaiting_confirm` di depan TIDAK memblokir pesan `queued` di belakangnya, tetapi urutan relatif antar pesan `queued` terjaga.
- Setelah satu pesan terkirim, pesan berikutnya wajib menunggu (cooldown 1500ms lalu `IsIdle` lagi). Itu memenuhi kriteria "tidak menyuntik berturut-turut".
- `status.IsIdle`/`status.OnIdle` boleh dipanggil saat memegang `q.mu`, asalkan callback `OnIdle` tidak mengambil `q.mu` sinkron (lihat di atas).

**`deliver(m Message) error`** (tanpa lock):

```go
sess, err := q.cfg.Sessions.Get(m.To)
if err != nil || sess.Status != session.StatusRunning { return errors.New("sesi target tidak berjalan") }
p, ok := provider.Get(sess.ProviderID)
if !ok { p = provider.Default() }
text := "[pesan dari " + m.FromName + "] " + m.Text
return p.SendInput(sessionWriter{mgr: q.cfg.Sessions, id: m.To}, text)
```

Teks multi-baris dikirim apa adanya; bracketed paste dan Enter ditangani `SendInput`. Bila `provider.Default()` nilainya nol/tak ada, kembalikan error ("provider tidak tersedia") alih-alih panic.

**`sessionWriter.Write(b)`**: `if err := w.mgr.Write(w.id, b); err != nil { return 0, err }; return len(b), nil`.

**`Position(id)`**: lock; `e := byID[id]`; bila nil -> 0; hitung indeks (1-based) `e` dalam `targets[e.msg.To].entries` (semua entri pending, termasuk `awaiting_confirm`); entri `sending` dihitung.

**`Pending(sessionID)`/`PendingAll()`**: salin `msg` semua entri pending (state `queued`/`awaiting_confirm`; entri `sending` dianggap masih pending). Selalu kembalikan slice non-nil. `PendingAll` diurutkan `CreatedAt` lalu `ID`.

**`Cancel(id)`**: lock; `e := byID[id]`; bila nil: `ErrNotFound` bila id tidak ada di `done`, atau `ErrInvalid` ("pesan sudah selesai") bila ada di `done`. Bila `e.sending` -> `ErrInvalid`. Hapus dari `entries`/`byID`, set `done` (state tetap dicatat sebagai `StateFailed` dengan alasan dibatalkan; cukup catat id), publish antrean; bila entries kosong panggil `disarm` dan hapus target; bila masih ada, panggil pump setelah unlock tidak perlu.

**`Confirm(id)`**: lock; `e` nil -> `ErrNotFound` (atau `ErrInvalid` bila di `done`); `e.msg.State != StateAwaitingConfirm` -> `ErrInvalid`; set `StateQueued`; publish; salin `msg`; unlock; `q.pump(e.msg.To)`; kembalikan salinan state terbaru (ambil ulang dari `e.msg` di bawah lock, sehingga bisa `delivered`).

**`Mode()/SetMode(m)`**: di bawah lock; `SetMode` menerima hanya tiga konstanta, selain itu `ErrInvalid`. Mengubah mode tidak mengubah state pesan yang sudah ada.

**`HandleExit(sessionID)`**: lock; `t := targets[sessionID]`; bila nil return. Untuk tiap entri: bila `!sending` set `StateFailed`, `Error = "sesi target berakhir"`, catat di `done`, hapus dari `byID` (entri `sending` ditangani pump yang melihat `targets[to] != t`; hapus juga dari `byID` supaya bersih). `disarm(t)`, `delete(targets, sessionID)`, publish `{sessionId, queue: []}`. Pesan yang pengirimnya (`from`) adalah sesi yang exit dibiarkan.

**`publishLocked(to)`**: bangun `queue := pendingOf(t)` (non-nil) dan panggil `cfg.Publish("mention.queue", map[string]any{"sessionId": to, "queue": queue})`. Dipanggil dengan `q.mu` dipegang agar urutan event konsisten (`srv.Publish` tidak memanggil balik ke Queue). Bila target sudah dihapus, kirim `queue: []`.

### 2. `internal/mention/ipc.go`

Impor: `context`, `encoding/json`, `errors`, `strings`, `aiterm/internal/ipc`. Salin pola `decodeParams` (params kosong/`null` diterima; JSON salah -> `ipc.ErrInvalidParams`), dengan nama lokal `decodeMentionParams` agar tidak bentrok.

- `toIPCError(err error) error`: `nil` -> nil; bila sudah `*ipc.Error` (`errors.As`) kembalikan apa adanya; `errors.Is(ErrNotFound)` -> `ipc.NewError(ipc.ErrNotFound, err.Error())`; `ErrInvalid`/`ErrCrossWorkspace`/`ErrUserOnly` -> `ipc.ErrInvalid`; `ErrConflict` -> `ipc.ErrConflict`; selain itu `ipc.NewError(ipc.ErrInternal, err.Error())`.
- `RegisterIPC(srv, q, namer, opts)` mendaftarkan tujuh method:
  - `mention.send` params `{workspaceId, to, text, from?}`. Logika: `to` kosong -> `ErrInvalid`. `ws := workspaceId`; bila kosong, coba `q.cfg.Sessions.Get(to)`; bila ada, `ws = sess.WorkspaceID`; bila tidak, `ErrInvalid` ("workspaceId wajib bila to berupa nama"). `toID, err := namer.Resolve(ws, to)`. `from` kosong -> `q.Enqueue(ctx, "", toID, text, SourceUser)`; terisi -> `fromID, err := namer.Resolve(ws, from)` lalu `q.Enqueue(ctx, fromID, toID, text, SourceAgent)`. Hasil `Message`. Semua error lewat `toIPCError`.
  - `mention.queue.list` params `{sessionId?}`: kosong -> `q.PendingAll()`, terisi -> `q.Pending(sessionId)`.
  - `mention.queue.cancel` params `{id}` -> `{"ok": true}`. `id` kosong -> `ErrInvalid`.
  - `mention.confirm` params `{id}` -> `Message`.
  - `mention.names` params `{workspaceId}` -> `namer.List(ws)` (`workspaceId` kosong -> `ErrInvalid`).
  - `mention.config` params `{mode?}`: bila `mode` terisi, `q.SetMode(Mode(mode))` (tak dikenal -> `ipc.ErrInvalid`); hasil selalu `{"mode": q.Mode()}`.
  - `mention.info` -> `{"url": URL(), "sessions": n}` dengan `n = opts.SessionCount()` bila tidak nil, selain itu 0.
- Handler bertipe `func(ctx context.Context, raw json.RawMessage) (any, error)`.

### 3. `cmd/aitermd/wire_mention.go`

`package main`. Impor: `context`, `encoding/json`, `sync`, `time`, `aiterm/internal/ipc`, `aiterm/internal/mention`, `aiterm/internal/provider`, `aiterm/internal/session`, `aiterm/internal/status`, dan stdlib seperlunya.

```go
func init() {
    RegisterModule("mention", func(srv *ipc.Server, d *Deps) {
        d.OnReady(func() { wireMention(srv, d) })
    })
}
```

Semua kerja ada di `wireMention` (dipanggil di `d.OnReady`, karena modul di-wire alfabetis dan "mention" lebih awal dari "provider"/"session"):

1. `v, ok := d.Lookup("session.manager")`; `mgr, ok2 := v.(*session.Manager)`. Bila tidak ada: `d.Logger.Warn("mention: session.manager tidak tersedia, modul dinonaktifkan")` lalu return (tanpa panic).
2. `dir := &workspaceDirectory{socket: d.SocketPath, logger: d.Logger}` memenuhi `mention.Directory`:
   - klien IPC internal dibuat malas (`ipc.Dial(d.SocketPath)` saat pertama kali dibutuhkan, dilindungi mutex; bila gagal kembalikan `""` dan coba lagi pada pemanggilan berikutnya). Klien tidak ditutup selama daemon hidup.
   - `ProjectName(workspaceID, projectID)`: panggil `workspace.list` (hasil bisa array `Workspace` atau objek `{"workspaces": [...]}`; decode ke `json.RawMessage`, coba array dulu lalu objek). Struktur minimal: `{id, projects: [{id, name}]}`. Cache `map[workspaceID+"/"+projectID]string` dengan cap waktu `fetchedAt` dan TTL 5 detik (perbandingan waktu saat dipanggil; BUKAN timer/goroutine). Bila gagal, kembalikan `""`.
3. `statusOf := func(id string) string`: coba `status.get {sessionId}` lewat klien internal yang sama (field `status`); bila gagal atau kosong, turunkan: `status.IsIdle(id)` -> `"idle"`, selain itu `"running"`. Bila paket `internal/status` ternyata punya getter Go publik, boleh dipakai langsung sebagai pengganti (cek dengan `grep -n "^func [A-Z]" internal/status/*.go`).
4. `namer := mention.NewNamer(mgr, dir, statusOf)`; `q := mention.NewQueue(mention.QueueConfig{Sessions: mgr, Namer: namer, Publish: srv.Publish, Logger: d.Logger})`.
5. Registrasi callback exit: struktur `exitHub` dengan `mu`, `cbs []func(string)`; `add(fn)` menambah, `fire(id)` memanggil semua salinan. `hub.add(q.HandleExit)` pertama.
6. Langganan `session.exit` (server belum `Serve` saat `OnReady`, jadi retry terbatas): fungsi `subscribeExit(attempt int)`: `c, err := ipc.Dial(d.SocketPath)`; bila sukses `c.Subscribe("session.exit", func(topic string, payload json.RawMessage){ var p struct{ID string `json:"id"`}; json.Unmarshal(payload,&p); if p.ID != "" { hub.fire(p.ID) } })`; bila Dial atau Subscribe gagal dan `attempt < 5`: `time.AfterFunc(200*time.Millisecond, func(){ subscribeExit(attempt+1) })`; bila sudah 5 kali: `d.Logger.Warn` dan berhenti. Dilarang `Ticker`/loop. Panggil `subscribeExit(1)` sekali. Klien langganan tidak ditutup.
7. `mention.RegisterIPC(srv, q, namer, mention.IPCOptions{SessionCount: func() int { ... }})`: `SessionCount` menghitung `mgr.List()` berstatus `running` yang providernya (`provider.Get(s.ProviderID)`) `Capabilities().MCP == true`.
8. `rt := &mention.Runtime{DataDir: d.DataDir, Logger: d.Logger, Sessions: mgr, Namer: namer, Enqueuer: q, Reader: q, OnSessionExit: hub.add}`; `if _, err := mention.Start(context.Background(), rt); err != nil { d.Logger.Warn("mention: gagal memulai server MCP", "err", err) }`. Kegagalan ini tidak boleh menghentikan daemon. Selama `mcp.go` belum ada, `Start` mengembalikan `("", nil)` (no-op); jangan membuat jalur kondisional lain.

Jangan memanggil `d.Provide` (nama layanan baru tidak diperlukan).

### 4. `cmd/aitermctl/cmd_mention.go`

`package main`; impor `context`, `errors`, `flag`, `fmt`, `io`, `strings`, `encoding/json`, `aiterm/internal/ipc`. Pola mengikuti `cmd_workspace.go`.

```go
const mentionUsage = `pemakaian: aitermctl mention <aksi>
  send [--workspace <id>] [--from <sessionId>] <to> <teks...>
  queue [sessionId]
  cancel <id>
  confirm <id>
  names <workspaceId>
  mode [auto|confirm_agent|user_only]
  info`

func init() { RegisterCommand(Command{Name: "mention", Summary: "mention antar sesi (send, queue, cancel, confirm, names, mode, info)", Run: runMention}) }
```

`runMention(ctx, c, args)`; `args` kosong -> `errors.New(mentionUsage)`; `switch args[0]`:

- `send`: parse flag dengan `flag.NewFlagSet`, `SetOutput(io.Discard)`, `--workspace`, `--from`. Sisa argumen minimal 2 (`<to>` lalu teks yang digabung `strings.Join(rest[1:], " ")`). Params `{workspaceId, to, text}` plus `from` bila diisi. Bila `--workspace` kosong, kirim `workspaceId` kosong (server menurunkannya dari sesi bila `to` berupa id; bila `to` berupa nama dan workspace kosong, server menjawab error jelas, yang diteruskan apa adanya). Method `mention.send`.
- `queue [sessionId]` -> `mention.queue.list` params `{sessionId}` hanya bila argumen ada.
- `cancel <id>` -> `mention.queue.cancel {id}`; `confirm <id>` -> `mention.confirm {id}`.
- `names <workspaceId>` -> `mention.names {workspaceId}`.
- `mode [m]` -> `mention.config` dengan `{mode: m}` bila ada argumen, selain itu `{}`.
- `info` -> `mention.info` params `struct{}{}`.
- Lainnya: `fmt.Errorf("sub-aksi tidak dikenal: %q\n%s", args[0], mentionUsage)`.

Semua memakai `callAndPrint(c, method, params)` dari `registry.go` (jangan diubah). Validasi jumlah argumen dengan pesan pemakaian per sub-aksi.

### 5. Guard

- `gofmt -l internal/mention cmd/aitermd/wire_mention.go cmd/aitermctl/cmd_mention.go` tidak mengeluarkan apa pun.
- `go build ./...` lolos. Bila gagal karena `internal/status` tidak ada, laporkan blocker (jangan membuat paket itu).
- `grep -n "time.Tick\|NewTicker\|setInterval" internal/mention/queue.go internal/mention/ipc.go cmd/aitermd/wire_mention.go cmd/aitermctl/cmd_mention.go` kosong.
- Pastikan `git status`/diff hanya memuat empat file milik fitur ini.

## Perubahan

Dibuat:

- `internal/mention/queue.go`
- `internal/mention/ipc.go`
- `cmd/aitermd/wire_mention.go`
- `cmd/aitermctl/cmd_mention.go`

Tidak ada file yang diubah.

## Wiring

Fitur ini tidak memerlukan edit file bersama. Yang diekspos:

- Daemon: modul `"mention"` terdaftar sendiri lewat `RegisterModule` di `init()` `wire_mention.go`; semua dependensi diambil di `d.OnReady`. `mention.Start(ctx, rt)` dipanggil di sana; saat `mcp.go` (fitur `mcp-server`) ada, `init()`-nya mengisi `startMCP` sehingga tidak perlu edit `wire_mention.go`.
- `Runtime.OnSessionExit` (hub callback exit) tersedia untuk `install.go`; `Runtime.Enqueuer` dan `Runtime.Reader` adalah `*Queue`.
- IPC: tujuh method `mention.*` dan event `mention.queue` (kontrak overview 2.5) untuk dipakai modul frontend `mention-ui`.
- CLI: perintah `mention` terdaftar lewat `RegisterCommand` di `init()` `cmd_mention.go`.
- Langkah wiring nanti hanya memverifikasi: nama modul `"mention"` tidak bentrok, `aitermctl` tanpa argumen mencantumkan `mention`, nama method/event di CLI dan frontend sama persis dengan `ipc.go`.

## Selesai bila

- Keempat file ada dan hanya itu yang berubah; `types.go` tidak diedit.
- `go build ./...` lolos dan `gofmt -l` bersih (guard; tanpa unit test).
- `*Queue` memenuhi `Enqueuer` dan `QueueReader` (assertion `var _` ada).
- Perilaku sesuai kriteria tiket: kirim segera bila idle (`delivered`), antre FIFO bila tidak idle dengan satu pendaftaran `OnIdle` per target, cooldown 1500ms via `time.AfterFunc` sekali jalan sebelum pesan berikutnya, `confirm_agent` menahan pesan agent di `awaiting_confirm` tanpa mendaftarkan `OnIdle`, `user_only` menolak agent (`ErrUserOnly` -> `ipc.ErrInvalid`), batas 50 -> `ErrConflict`, `SendInput` gagal -> `failed` dan pesan lain lanjut, `session.exit` membersihkan antrean (pending jadi `failed`, event `queue: []`).
- Event `mention.queue` terbit pada tiap perubahan antrean (tambah, kirim, batal, konfirmasi, exit).
- Tidak ada `time.Ticker`, `time.Tick`, atau loop tidur-bangun di keempat file.
- Laporan akhir menyebut penyimpangan dari plan (idealnya tidak ada) dan blocker (mis. `internal/status` belum ada).
