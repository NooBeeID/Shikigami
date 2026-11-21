# Plan: Mesin status dan idle fallback

## Metadata

- Tiket: `ai-terminal-ticket-08-session-status` (`docs/tickets/ai-terminal-ticket-08-session-status.md`)
- Fitur: `status-engine`
- ownedPaths:
  - `internal/status/engine.go`
  - `internal/status/idle.go`
  - `internal/status/ipc.go`
  - `cmd/aitermd/wire_status.go`
  - `cmd/aitermctl/cmd_status.go`
- Bergantung pada Foundation: ya (`internal/status/types.go` dan stub semua fungsi kontrak sudah ada; `wire_status.go` versi final dan stub `cmd_status.go` sudah dibuat Foundation).

Rujukan desain bersama: `docs/plans/ai-terminal-ticket-08-session-status/overview.md` (bagian 2.1 T1/T2, 2.3, 2.4, 2.5, 2.6, 2.7, 5, 7). Baca bagian itu sebelum mulai; plan ini tidak mengulangnya.

## Tujuan

Mengganti stub milik fitur ini dengan implementasi nyata: state machine status per sesi (`running|waiting|done|idle`) yang event-driven, bersumber dari `Engine.Apply` (hook), output PTY, dan exit sesi. Output dan exit diterima lewat klien IPC internal yang dibuka malas ke socket daemon sendiri (T2). Launch token diikat malas ke ID sesi asli (T1). Termasuk timer output-diam sekali jalan, API `IsIdle`/`OnIdle`, method IPC `status.get|list|hook`, event `session.status`, wiring modul daemon, dan CLI `aitermctl status list|get|watch|hook`. Dilarang ada `time.Ticker`, `time.Tick`, atau loop tidur-bangun.

## Path yang dimiliki

HANYA lima file ini yang boleh diedit:
- `internal/status/engine.go`
- `internal/status/idle.go`
- `internal/status/ipc.go`
- `cmd/aitermd/wire_status.go`
- `cmd/aitermctl/cmd_status.go`

Dilarang menyentuh: `internal/status/types.go`, `hooks.go`, `install.go`, `notify.go`, `frontend/`, `internal/session/`, `internal/provider/`, `internal/ipc/`, `internal/paths/`, `cmd/aitermd/modules.go`, `cmd/aitermd/main.go`, `cmd/aitermctl/registry.go`, `cmd/aitermctl/main.go`, `app/`, `go.mod`/`go.sum`, file epic lain. Bila butuh tipe/konstanta baru, buat identifier privat di file sendiri. Tanpa unit test.

## Kontrak

Disediakan (signature persis overview 2.4, JANGAN diubah): `ErrUnknownSession`, `SessionSource`, `Engine`, `NewEngine(srv *ipc.Server) *Engine`, `SetDefault`, `Default`, metode `Engine`: `SetSessionSource`, `RegisterLaunch`, `ResolveSessionID`, `Track`, `Set`, `Apply`, `OnOutput`, `Get`, `List`, `Lookup`, `Subscribe`, `Forget`; fungsi paket `RegisterIPC(srv, eng)`, `RegisterLaunch(info)`, `IsIdle(id)`, `OnIdle(id, fn)`. Pesan IPC `status.get`, `status.list`, `status.hook`; event `session.status` (payload `StatusEvent`). Format dan aturan: overview 2.5 dan 2.6.

Dipakai:
- Dari `types.go` (Foundation): `Status`, `Status*`, `Source*`, `Hook*`, `SilentDelay`, `SilentDelayWithHooks`, `StatusEvent`, `SessionState`, `LaunchInfo`. Jangan didefinisikan ulang.
- Dari `hooks.go` (hook-receiver): `ClassifyEvent(event) (Status, bool)` (sudah final dari Foundation). Dari `install.go`: `CleanupHooks(token string)`, dipanggil `Forget`.
- `internal/ipc`: `Server.Register(method, func(ctx, params json.RawMessage) (any, error))`, `Server.Publish(topic, payload)`, `ipc.Dial(socketPath) (*Client, error)`, `Client.Subscribe(prefix, fn(topic string, payload json.RawMessage)) (unsubscribe func(), err error)`, `Client.Call(method, params, out) error`, `Client.Close()`, kode `ipc.ErrNotFound`, `ipc.ErrInvalidParams`, `ipc.NewError(code, msg)`, `ipc.Errorf`.
- `internal/paths`: `paths.SocketPath() (string, error)` (perhatikan: mengembalikan DUA nilai).
- `internal/session`: `session.Session{ID, WorkspaceID, ProjectID, Cwd, ProviderID, Status, ...}`. Event `session.output.<id>` (topik; ID ada di sufiks topik) dan `session.exit` payload `{id, exitCode}`.
- `internal/provider`: `provider.Get(id) (Provider, bool)`, `Provider.Capabilities().Hooks`.
- `cmd/aitermctl`: `RegisterCommand`, `Command`, `printJSON`, `callAndPrint`; `Run` bertanda `func(ctx context.Context, c *ipc.Client, args []string) error`.

Sebelum menulis, baca ulang kode nyata (`internal/ipc/client.go`, `server.go`, `internal/session/types.go`, `manager.go`, `internal/provider/*.go`, `cmd/aitermd/modules.go`, `cmd/aitermd/wire_session.go`, contoh `cmd/aitermctl/cmd_session.go`) dan sesuaikan bila ada beda nama dengan plan ini.

## Langkah

### 1. `internal/status/engine.go` (ganti stub)

Hapus komentar `STUB(foundation)`. Struktur:

```go
type sessionState struct {
    id         string
    providerID string
    token      string      // launch token terikat; "" bila tidak terikat
    status     Status
    since      time.Time
    hookSeen   bool        // sudah ada hook pertama
    timer      *time.Timer // nil sampai output pertama
}

type Engine struct {
    mu        sync.Mutex
    srv       *ipc.Server
    src       SessionSource
    states    map[string]*sessionState // kunci: ID sesi asli
    pending   []LaunchInfo             // launch belum terikat, urut At (FIFO)
    byToken   map[string]string        // token -> ID sesi asli
    bound     map[string]bool          // ID sesi yang sudah mengikat token (satu token satu sesi)
    subs      map[int]func(StatusEvent)
    nextSub   int
    waiters   map[string]map[int]func() // OnIdle: id -> waiter
    nextWait  int
    streamMu  sync.Mutex               // melindungi ensureStream
    stream    *ipc.Client
    unsubs    []func()
}
```

Singleton: `var defaultEngine atomic.Pointer[Engine]`; `SetDefault(e)` -> `Store`, `Default()` -> `Load`. Fungsi paket `RegisterLaunch(info)`: `if e := Default(); e != nil { e.RegisterLaunch(info) }`.

`NewEngine(srv)`: inisialisasi semua peta; tidak ada goroutine, tidak Dial.

`SetSessionSource(src)`: set di bawah `mu`.

`ensureStream() error` (idempoten, pakai `streamMu`, tanpa loop retry):
- Bila `e.stream != nil` kembalikan nil.
- `sock, err := paths.SocketPath()`; `c, err := ipc.Dial(sock)`; gagal -> kembalikan error (dicoba lagi saat panggilan berikutnya; jangan retry sendiri).
- `Subscribe("session.output.", fn)`: ambil `id := strings.TrimPrefix(topic, "session.output.")`, panggil `e.OnOutput(id)`. Payload diabaikan (tidak perlu decode base64).
- `Subscribe("session.exit", fn)`: decode `{id string}`; panggil `e.handleExit(id)`.
- Bila salah satu Subscribe gagal: `c.Close()`, kembalikan error, `stream` tetap nil.
- Callback dijalankan di dispatch loop klien: JANGAN blok; jangan memanggil `Call` sinkron dari dalam callback.
- Dipanggil dari: `RegisterLaunch`, `Track`, dan handler `status.*` (lewat `RegisterIPC`). Error `ensureStream` dari `RegisterLaunch`/`Track` diabaikan (log opsional); handler IPC tetap menjawab. Komentar: sesi hanya bisa dibuat via `session.create` sehingga socket sudah hidup dan stream tersambung sebelum output pertama; tidak ada ticker atau retry loop.

`RegisterLaunch(info)`: lock, append ke `pending` (batasi ukuran, mis. buang yang tertua bila > 256 supaya tidak bocor bila sesi gagal dibuat), unlock, lalu `ensureStream()`.

Binding malas `bindLocked(sessionID string) *sessionState` (dipanggil di bawah `mu`; boleh memanggil `src.Get` karena SessionSource tidak memanggil balik engine; bila `src == nil` lewati):
- Bila state untuk `sessionID` sudah ada, kembalikan.
- `s, err := src.Get(sessionID)`; gagal -> `nil`.
- Buat state `{id, providerID: s.ProviderID, status: StatusIdle, since: now}`.
- Cari di `pending` yang pertama (tertua) dengan `ProviderID, Cwd, ProjectID, WorkspaceID` sama; bila ketemu: `st.token = info.Token`, `byToken[token] = sessionID`, hapus dari `pending`. Komentar keterbatasan T1 (dua sesi serentak dengan spec identik bisa tertukar).
- Simpan di `states`, kembalikan. State baru dibuat TANPA event (status awal idle, senyap).

`Track(sessionID) bool`: lock; `bindLocked`; return `st != nil`; panggil `ensureStream()` di luar lock.

`ResolveSessionID(idOrToken) (string, bool)`: lock; bila `byToken[x]` ada -> kembalikan; bila `states[x]` ada -> `(x, true)`; selain itu coba `bindLocked(x)` (kasus ID asli yang baru muncul); bila gagal dan `x` cocok dengan token di `pending` -> coba ikat: iterasi `src.List()` (hanya sesi `running` yang belum ada di `states`) lalu `bindLocked` tiap kandidat sampai `byToken[x]` terisi; bila terisi kembalikan. Selain itu `("", false)`.

`Set(sessionID, st, source, reason) bool`:
- lock; `state := bindLocked(sessionID)`; nil -> unlock, false. `prev := state.status`; sama -> false.
- Ubah `status`, `since`. Bila `st == StatusDone` hentikan timer (`stopTimerLocked`). Kumpulkan `ev := StatusEvent{..., At: now.UnixMilli()}`, daftar subscriber (salin), dan waiter yang harus dibangunkan (bila `isIdleStatus(st)`: ambil semua waiter sesi itu, hapus dari peta). Unlock.
- DI LUAR mutex: `e.srv.Publish("session.status", ev)` (nil-check `srv`), panggil tiap subscriber (recover panic per subscriber), panggil tiap waiter di goroutine terpisah (`go fn()`). Return true.
- Helper `isIdleStatus(s) bool` = `idle|done|waiting` (satu sumber kebenaran; dipakai `idle.go`).

`Apply(idOrToken, event string, payload map[string]any) error`:
- `id, ok := ResolveSessionID(idOrToken)`; tidak ok -> `ErrUnknownSession`.
- Tandai `hookSeen = true` dan hentikan timer silent (hanya bila sesi berhooks, lihat `hookCapable`) untuk event yang dikenal; lakukan di bawah lock.
- `st, ok := ClassifyEvent(event)`; tidak ok -> nil (abaikan, tidak ada event). Panggil `Set(id, st, SourceHook, event)`; return nil.
- `payload` hanya dipakai untuk logging bila perlu; tidak wajib.

`hookCapable(state) bool`: `state.token != ""` DAN `p, ok := provider.Get(state.providerID); ok && p.Capabilities().Hooks`. Implementasikan sebagai fungsi biasa yang dipanggil di bawah lock (provider.Get tidak memanggil balik engine).

`OnOutput(sessionID)` ada di `idle.go` (langkah 2).

`handleExit(id)`: `Set(id, StatusDone, SourceExit, "exit")` lalu `Forget(id)`. Bila state belum ada (sesi tak pernah terlihat), `Set` mengembalikan false dan tidak ada event; tetap panggil `Forget`.

`Get(id) (SessionState, bool)`: lock; resolusi hanya lewat `states` + `bindLocked`; `SessionState{SessionID, Status, Since: since.UnixMilli()}`.

`List() []SessionState`: lock; bila `src != nil`, untuk tiap `session.Session` berstatus `"running"` dari `src.List()` panggil `bindLocked(s.ID)` (Track otomatis, senyap); kembalikan slice (BUKAN nil: `make([]SessionState, 0, n)`) terurut stabil menurut `since` lalu `SessionID` (`sort.Slice`). Panggil `ensureStream()` di luar lock oleh handler IPC, bukan di sini.

`Lookup(id) (session.Session, bool)`: `src.Get(id)`; `*Session` -> nilai; error/nil -> zero, false. Jangan pegang `mu` saat memanggil src (ambil `src` ke variabel lokal di bawah lock).

`Subscribe(fn) (cancel func())`: daftar di `subs` dengan ID unik; `cancel` idempoten menghapus.

`Forget(id)`: lock; ambil state; bila ada: stop timer, hapus dari `states`, ambil `token`, hapus `byToken[token]`; hapus waiter sesi itu (tanpa memanggilnya); unlock; bila token tidak kosong panggil `CleanupHooks(token)` (di luar lock).

Catatan eksplisit: ketika `Forget` dipanggil karena exit, subscriber sudah menerima event `done` dari `handleExit` lebih dulu.

### 2. `internal/status/idle.go` (ganti stub)

`func (e *Engine) OnOutput(sessionID string)` (metode di sini agar dekat dengan timer; boleh dipindah ke engine.go bila lebih rapi, tetapi tetap di file milik fitur ini):
1. Lock; `state := bindLocked(sessionID)`; nil -> unlock, return.
2. Tentukan `delay`:
   - tanpa hooks (`!hookCapable(state)`): `SilentDelay`.
   - berhooks dan `!state.hookSeen`: `SilentDelayWithHooks`.
   - berhooks dan `state.hookSeen`: `stopTimerLocked`, unlock, return (output diabaikan untuk status).
3. Bila `state.status == StatusDone`: unlock, return (timer tidak dibuat/dihidupkan lagi).
4. Bila berhooks (belum ada hook) dan status `waiting`/`done`: jangan menimpa; tidak mengubah status dan tidak menjadwalkan timer; return.
5. Jadwalkan timer sekali jalan: bila `state.timer == nil` -> `state.timer = time.AfterFunc(delay, func(){ e.onSilent(sessionID) })`; selain itu `state.timer.Reset(delay)` (callback `AfterFunc` yang sama; aman dipanggil pada timer yang sudah habis). Karena delay bisa berbeda antar kondisi, `Reset(delay)` dengan nilai terkini.
6. Unlock. Bila `state.status != StatusRunning` (tanpa hooks, atau berhooks sebelum hook pertama dan status `idle`): `e.Set(sessionID, StatusRunning, SourceOutput, "output")` (di luar lock).

`onSilent(id)`: lock; ambil state; bila nil atau `status == done` -> return; bila berhooks dan `hookSeen` -> return; bila berhooks dan status `waiting` -> return; unlock; `e.Set(id, StatusIdle, SourceOutput, "silent")`. Race tipis (output tiba tepat saat timer habis) dapat diterima: output berikutnya mengembalikan `running`. Beri komentar.

`stopTimerLocked(state)`: `if state.timer != nil { state.timer.Stop() }` (biarkan timer ada untuk dipakai ulang; tidak ada goroutine tersisa karena `AfterFunc` tidak membuat goroutine sampai habis).

Komentar kode wajib: echo keystroke user terhitung output (keterbatasan yang diterima); tidak ada ticker; tidak ada timer sebelum output pertama.

`func IsIdle(sessionID string) bool`: `e := Default()`; nil -> false; `s, ok := e.Get(sessionID)`; `ok && isIdleStatus(s.Status)`. ID dapat berupa ID sesi asli (epic 09 memakai ID asli).

`func OnIdle(sessionID string, fn func()) (cancel func())`:
- `e := Default()`; nil atau `fn == nil` -> kembalikan `func(){}`.
- Lock; `state := bindLocked(sessionID)`; nil (sesi tak dikenal) -> unlock; kembalikan no-op (tidak memanggil fn). Bila `isIdleStatus(state.status)` -> unlock; `go fn()`; kembalikan no-op.
- Selain itu daftarkan waiter dengan ID unik di `e.waiters[state.id]`; `cancel` (idempoten, `sync.Once`) menghapusnya. `fn` dipanggil sekali oleh `Set` pada transisi ke status idle-menurut-IsIdle (sudah ditulis di langkah 1, waiter dihapus saat dibangunkan sehingga tepat sekali). Race cancel vs dibangunkan: waiter dihapus dari peta di bawah lock oleh siapa pun yang lebih dulu; bila `Set` sudah mengambilnya, `fn` tetap berjalan (diterima).

### 3. `internal/status/ipc.go` (ganti stub)

`RegisterIPC(srv *ipc.Server, eng *Engine)`:

- `status.get`: decode `{"sessionId"}` (galat decode -> `ipc.NewError(ipc.ErrInvalidParams, ...)`); kosong -> `ErrInvalidParams`; `eng.ensureStream()` (abaikan error); `id, ok := eng.ResolveSessionID(p.SessionID)`; tidak ok -> `ipc.NewError(ipc.ErrNotFound, "sesi tidak dikenal")`; `eng.Get(id)`; tidak ok -> NotFound; kembalikan `SessionState`.
- `status.list`: `eng.ensureStream()`; kembalikan `eng.List()` langsung (array JSON, `[]` bila kosong, bukan objek pembungkus).
- `status.hook`: decode `{"sessionId","event","payload" map[string]any}`; `sessionId` atau `event` kosong -> `ErrInvalidParams`; `eng.ensureStream()`; `err := eng.Apply(p.SessionID, p.Event, p.Payload)`; `errors.Is(err, ErrUnknownSession)` -> NotFound; err lain -> kembalikan; sukses -> `map[string]any{"ok": true}`. Event tak dikenal sudah menghasilkan `ok:true` dari `Apply`.
- Handler memakai signature `func(ctx context.Context, params json.RawMessage) (any, error)`; error bertipe `*ipc.Error` dikembalikan apa adanya.

### 4. `cmd/aitermd/wire_status.go` (verifikasi, edit seperlunya)

File ini sudah final dari Foundation. Verifikasi isinya sesuai urutan overview bagian 5 butir 1 (`NewEngine` -> `SetDefault` -> `d.Provide("status.engine", eng)` -> `RegisterIPC` -> `InstallHooks()` -> `StartNotifier(srv, eng)` -> `d.OnReady` yang memasang `SetSessionSource` dari `d.Lookup("session.manager")` dengan type-assert `*session.Manager`, log warning bila gagal, nil-check `d.Logger`). Perbaiki hanya bila menyimpang atau tidak mengompilasi. Pastikan `*session.Manager` memenuhi `SessionSource`. Jangan mengubah `modules.go`.

### 5. `cmd/aitermctl/cmd_status.go` (ganti stub)

Hapus komentar STUB. `init()` mendaftarkan satu command `status` (`Summary: "Status sesi: list|get|watch|hook"`) dengan `Run: runStatus`. Konstanta `statusUsage = "penggunaan: aitermctl status list|get <sessionId>|watch|hook --session <id> --event <Nama>"`.

`runStatus(ctx, c, args)` dispatch berdasarkan `args[0]`; kosong/tak dikenal -> `errors.New(statusUsage)`.

- `list`: `c.Call("status.list", nil, &[]statusRow)`; cetak tabel dengan `text/tabwriter` kolom `SESSION`, `STATUS`, `SEJAK` (waktu dari `since` ms, format `15:04:05` lokal); bila kosong cetak `(tidak ada sesi)`.
- `get <sessionId>`: wajib satu argumen; `callAndPrint(c, "status.get", map[string]any{"sessionId": id})`.
- `watch`: `unsub, err := c.Subscribe("session.status", fn)` dengan `fn` men-decode payload ke struct lokal (field setara `StatusEvent`; jangan import `internal/status` bila bisa dihindari, boleh juga import untuk tipe) dan mencetak satu baris per event: `HH:MM:SS <sessionId> <prev> -> <status> (<source>/<reason>)`. Blok sampai `ctx.Done()` atau sinyal `SIGINT/SIGTERM` (`signal.NotifyContext`); `defer unsub()`. Tanpa ticker; blokir dengan `<-ctx.Done()`.
- `hook`: `flag.NewFlagSet("status hook", flag.ContinueOnError)` dengan `--session` dan `--event`. Alur WAJIB selalu `return nil` pada akhirnya:
  1. Parse flag; bila galat atau salah satu kosong: cetak peringatan ke stderr, `return nil`.
  2. Baca stdin: `io.ReadAll(io.LimitReader(os.Stdin, 1<<20))` (galat dicetak ke stderr, lanjut dengan data kosong). Jangan memblokir bila stdin terminal: baca saja, hook Claude selalu memberi stdin yang ditutup.
  3. `payload := map[string]any{}`; `json.Unmarshal` ke map; bila bukan object valid -> `{}`. (Bila `status.ParseHookEvent` dari hook-receiver dibutuhkan, JANGAN dipakai: cukup logika lokal ini agar tidak bergantung fitur lain.)
  4. `c.Call("status.hook", map[string]any{"sessionId": session, "event": event, "payload": payload}, nil)`; galat dicetak ke stderr (`fmt.Fprintf(os.Stderr, "status hook: %v\n", err)`), TIDAK dikembalikan.
  5. `return nil`.
  Catatan komentar kode: `main.go` aitermctl keluar kode 1 bila Dial gagal sebelum command jalan; itu ditangani command hook di berkas hooks yang dibungkus `|| true` (milik hook-receiver). `main.go` tidak boleh diedit.

## Perubahan

Diubah (stub Foundation diganti implementasi):
- `internal/status/engine.go`
- `internal/status/idle.go`
- `internal/status/ipc.go`
- `cmd/aitermctl/cmd_status.go`
- `cmd/aitermd/wire_status.go` (hanya verifikasi; ubah bila perlu)

Tidak ada file baru dan tidak ada file lain yang disentuh.

## Wiring

Yang diekspos untuk langkah wiring (JANGAN disambungkan sendiri di luar `wire_status.go`):
- `status.NewEngine`, `status.SetDefault`, `status.Default`, `status.RegisterIPC`, `Engine.SetSessionSource` (dipanggil dari `wire_status.go` milik fitur ini, versi Foundation).
- `status.RegisterLaunch(LaunchInfo)` dan `Engine.ResolveSessionID`: dipanggil oleh `install.go` (hook-receiver) dan epic hilir; hook-receiver memanggil `RegisterLaunch` dari launch contribution-nya.
- `Engine.Subscribe` dan `Engine.Lookup`: dipakai `notify.go` (status-notify-ui).
- `Engine.Forget` memanggil `CleanupHooks(token)` (milik hook-receiver).
- `status.IsIdle` / `status.OnIdle`: dipakai tiket 09.
- Event `session.status` dan method `status.get|list|hook`: dipakai frontend dan `aitermctl`.
- Command `status` di aitermctl lewat `init()` + `RegisterCommand`.

Tugas wiring (agen lain): cek `grep -rn "STUB(foundation)" internal/status cmd frontend/src/modules/status` kosong, build penuh, dan smoke test overview bagian 5 butir 8.

## Selesai bila

- [ ] `go build ./...` sukses (guard utama; setelah itu `gofmt -l internal/status cmd` kosong; opsional `go vet ./internal/status/... ./cmd/...`).
- [ ] Tidak ada komentar `STUB(foundation)` tersisa di kelima file milik fitur ini.
- [ ] `grep -rnE "time\.(NewTicker|Tick)\(" internal/status` kosong dan tidak ada loop tidur-bangun (`for { time.Sleep }`) di `internal/status/`.
- [ ] Signature di overview 2.4 tidak berubah; `types.go` tidak diubah; tidak ada file di luar ownedPaths yang berubah.
- [ ] Logika sesuai aturan overview 2.6: `Set` status sama tidak menerbitkan event; publish, subscriber, dan waiter dipanggil di luar mutex; timer sekali jalan hanya setelah output pertama dan dihentikan saat `done`/`Forget`; berhooks yang sudah `hookSeen` mengabaikan output; exit -> `done` (`source:"exit"`) lalu `Forget` (state, timer, hooks dibuang).
- [ ] `status.get` / `status.hook` untuk ID/token tak dikenal -> `-32002`; `status.list` mengembalikan `[]` (bukan `null`) saat kosong.
- [ ] `aitermctl status hook` selalu mengembalikan `nil` dari `Run` (error hanya ke stderr).
