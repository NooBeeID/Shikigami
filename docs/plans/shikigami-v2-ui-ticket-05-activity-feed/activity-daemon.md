# Plan: Daemon: buffer dan event aktivitas tool call

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-05-activity-feed.md` (shikigami-v2-ui-ticket-05-activity-feed)
- Fitur: `activity-daemon`
- ownedPaths: `internal/status/activity.go`, `cmd/shikigamid/wire_activity.go`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; kontrak ada di overview)
- Bahasa: Go, paket `status` dan `main`. Tanpa dependency baru, tanpa unit test, tanpa timer/polling.

## Tujuan

Menyediakan buffer in-memory aktivitas tool call agent claude (event hook `PostToolUse`), method IPC `activity.list`, dan event `activity.event`, plus modul daemon `activity` yang menyambungkan log ke `status.Engine` tanpa menyentuh file engine. Detail kontrak dan perilaku terkunci ada di `overview.md` bagian 2 (terutama 2.2 sampai 2.5) dan catatan `activity-daemon` di bagian 3; rujuk, jangan diubah.

## Path yang dimiliki

HANYA boleh membuat/mengubah:
- `internal/status/activity.go` (baru)
- `cmd/shikigamid/wire_activity.go` (baru)

Dilarang menyentuh: `internal/status/{engine,install,types,hooks,ipc}.go`, `cmd/shikigamid/{main,modules,wire_*}.go` lain, `frontend/**` (milik `activity-ui`), `docs/**`, `Makefile`. Kebutuhan perubahan di file itu dicatat untuk agen Wiring, tidak dikerjakan.

## Kontrak

Disediakan (bentuk persis seperti overview 2.3):

```go
// internal/status/activity.go
type ActivityEntry struct {
    TS        int64  `json:"ts"`
    SessionID string `json:"sessionId"`
    Tool      string `json:"tool"`
    Summary   string `json:"summary"`
}
type ActivityLog struct { /* lihat Langkah 2 */ }
func NewActivityLog(srv *ipc.Server) *ActivityLog
func (a *ActivityLog) Record(sessionID, event string, payload map[string]any)
func (a *ActivityLog) List(sessionID string, limit int) []ActivityEntry
func (a *ActivityLog) Forget(sessionID string)
func (a *ActivityLog) MarkEnded(sessionID string)
func RegisterActivityIPC(srv *ipc.Server, a *ActivityLog)
```

- IPC `activity.list` params `{sessionId?, limit?}`; event `activity.event` payload `ActivityEntry` via `srv.Publish("activity.event", entry)`.
- Layanan modul: `d.Provide("activity.log", log)` bertipe `*status.ActivityLog`.
- Bentuk sink yang dipasang ke engine: `func(sessionID, event string, payload map[string]any)`; `log.Record` memenuhinya.

Dipakai (sudah ada di repo, jangan diubah):
- `ipc.Server.Register(method string, h ipc.Handler)` dengan `Handler func(ctx context.Context, params json.RawMessage) (any, error)`; `ipc.NewError(ipc.ErrInvalidParams, msg)`; `srv.Publish(topic string, payload any)`.
- `RegisterModule(name, func(srv *ipc.Server, d *Deps))`, `d.Provide`, `d.Lookup(name) (any, bool)`, `d.OnReady(fn)`, `d.Logger` (`*slog.Logger`, cek nil) di `cmd/shikigamid/modules.go`.
- `*status.Engine` (`Provide` nama `status.engine`), `eng.Subscribe(fn func(status.StatusEvent)) (cancel func())`, `status.StatusEvent{SessionID, Status, Prev, Source, Reason, At}`, konstanta `status.StatusDone`, `status.SourceExit`.
- Modul panggilan pola referensi: `cmd/shikigamid/wire_status.go` (pola `init()` + `RegisterModule` + `d.OnReady` + `Lookup` + type assertion).

## Langkah

### 1. `internal/status/activity.go`: tipe dan konstanta

Paket `status`. Impor: `context`, `encoding/json`, `sort`, `strings`, `sync`, `time`, `github.com/NooBeeID/shikigami/internal/ipc`.

Konstanta:
```go
const (
    activityRingCap     = 200 // kapasitas buffer per sesi
    activityListDefault = 100
    activityListMax     = 200
    activitySummaryMax  = 160 // rune
    activityEndedKeep   = 20  // maks buffer sesi ended
    activityEvent       = "PostToolUse"
    activityTopic       = "activity.event"
)
```
Jangan memakai `HookPostToolUse` (konstanta itu baru ada setelah Wiring); pakai konstanta lokal `activityEvent` agar compile sebelum Wiring.

`ActivityEntry` persis seperti Kontrak.

### 2. `ActivityLog` dan konstruktor

```go
type activityBuf struct {
    entries []ActivityEntry // urut lama -> baru, panjang <= activityRingCap
}
type ActivityLog struct {
    srv   *ipc.Server
    mu    sync.Mutex
    bufs  map[string]*activityBuf
    ended []string // urutan MarkEnded, tertua di depan, tanpa duplikat
}
func NewActivityLog(srv *ipc.Server) *ActivityLog // inisialisasi bufs
```
Implementasi ring boleh berupa slice yang dipotong (`entries = entries[1:]` lalu append; salin ke slice baru bila kapasitas tumbuh berlebih, atau pakai ring index) selama panjang tidak pernah melebihi 200 dan entri tertua dibuang lebih dulu. Tidak ada goroutine, `time.Ticker`, atau `time.AfterFunc`.

### 3. `summarize`

```go
func summarize(tool string, input map[string]any) string
```
- Pilih field kunci: `Bash`->`command`; `Read`,`Edit`,`Write`,`NotebookEdit`->`file_path`; `Grep`,`Glob`->`pattern`; `WebFetch`->`url`; `Task`->`description`.
- Untuk tool di tabel: ambil `input[key]` dengan `s, ok := v.(string)`; bukan string/tidak ada -> `""`.
- Untuk tool lain: urutkan kunci `input` alfabetis, ambil nilai string pertama (bukan sekadar kunci pertama; lewati nilai non-string), atau `""`.
- `input == nil` (tool_input bukan objek) -> `""`.
- Normalisasi: ganti `\r\n`, `\r`, `\n` menjadi spasi (mis. `strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")`), lalu potong ke 160 rune dengan konversi `[]rune` (jangan potong byte). Tidak perlu trim tambahan selain itu.
- Tidak pernah membaca `tool_response`.

### 4. `Record`

```go
func (a *ActivityLog) Record(sessionID, event string, payload map[string]any)
```
1. `if a == nil || event != activityEvent || sessionID == "" { return }`.
2. `tool, ok := payload["tool_name"].(string)`; `if !ok || strings.TrimSpace(tool) == "" { return }` (payload nil aman: indeks map nil mengembalikan nilai nol).
3. `input, _ := payload["tool_input"].(map[string]any)` (nil bila bukan objek). Hitung `summary := summarize(tool, input)`.
4. Bangun `entry := ActivityEntry{TS: time.Now().UnixMilli(), SessionID: sessionID, Tool: tool, Summary: summary}`.
5. Kunci `a.mu`: ambil/buat buffer, append (buang tertua bila sudah 200), hapus `sessionID` dari `a.ended` (rekaman baru mengembalikan sesi ke aktif), buka kunci.
6. DI LUAR lock: `if a.srv != nil { a.srv.Publish(activityTopic, entry) }`.
7. Bungkus seluruh isi dengan `defer func(){ _ = recover() }()` tambahan sebagai jaring pengaman terhadap payload aneh (type assertion tetap memakai `, ok`). Jangan menyimpan atau meneruskan `payload` itu sendiri atau `tool_response`.

### 5. `List`

```go
func (a *ActivityLog) List(sessionID string, limit int) []ActivityEntry
```
- Normalisasi `limit`: `<=0` -> 100; `>200` -> 200.
- Di bawah lock: bila `sessionID != ""`, salin buffer sesi itu (tak ada -> kosong); bila kosong, salin semua buffer digabung.
- Urutkan terbaru -> terlama: `sort.SliceStable` dengan `TS` menurun. Untuk satu sesi, agar stabil pada `ts` sama, sebelum sort balik urutan (susun dari paling baru ke paling lama) lalu `SliceStable` dengan `TS` menurun sehingga entri yang masuk belakangan tetap di depan pada `ts` kembar.
- Potong ke `limit`. SELALU kembalikan slice non-nil (`make([]ActivityEntry, 0, n)`), supaya JSON jadi `[]`, bukan `null`.

### 6. `Forget` dan `MarkEnded`

- `Forget(sessionID)`: di bawah lock, `delete(a.bufs, sessionID)` dan hapus dari `a.ended`. Aman dipanggil untuk id tak dikenal.
- `MarkEnded(sessionID)`: di bawah lock; abaikan bila `sessionID == ""` atau tidak ada buffer untuk sesi itu. Hapus dulu dari `a.ended` (hindari duplikat), append ke belakang; selama `len(a.ended) > activityEndedKeep`, buang `a.ended[0]` beserta `delete(a.bufs, id)`. Tanpa timer.

### 7. `RegisterActivityIPC`

```go
func RegisterActivityIPC(srv *ipc.Server, a *ActivityLog)
```
`srv.Register("activity.list", handler)`:
- Parse params: `var p struct{ SessionID string `json:"sessionId"`; Limit int `json:"limit"` }`. Bila `len(bytes.TrimSpace(params)) == 0` atau `string(trimmed) == "null"` -> pakai default (jangan Unmarshal). Selain itu `json.Unmarshal`; error -> `nil, ipc.NewError(ipc.ErrInvalidParams, "parameter tidak valid")`. (`{}` otomatis diterima.) Tambahkan impor `bytes`.
- Kembalikan `a.List(p.SessionID, p.Limit), nil`. Sesi tak dikenal -> `[]`, bukan error.
- `limit` bertipe `int`; nilai non-angka pada JSON menghasilkan error invalid params, wajar.

Tambahkan komentar singkat Indonesia di tiap tipe/fungsi publik (gaya repo).

### 8. `cmd/shikigamid/wire_activity.go`

Paket `main`. Impor: `github.com/NooBeeID/shikigami/internal/ipc`, `github.com/NooBeeID/shikigami/internal/status`.

```go
func init() {
    RegisterModule("activity", func(srv *ipc.Server, d *Deps) {
        log := status.NewActivityLog(srv)
        d.Provide("activity.log", log)
        status.RegisterActivityIPC(srv, log)
        d.OnReady(func() {
            v, ok := d.Lookup("status.engine")
            eng, ok2 := v.(*status.Engine)
            if !ok || !ok2 {
                if d.Logger != nil { d.Logger.Warn("activity: status.engine tidak tersedia") }
                return
            }
            // Antarmuka lokal: compile sebelum dan sesudah Wiring menambah SetActivitySink.
            if s, ok := any(eng).(interface {
                SetActivitySink(func(sessionID, event string, payload map[string]any))
            }); ok {
                s.SetActivitySink(log.Record)
            } else if d.Logger != nil {
                d.Logger.Warn("activity: engine belum mendukung SetActivitySink")
            }
            eng.Subscribe(func(ev status.StatusEvent) {
                if ev.Status == status.StatusDone && ev.Source == status.SourceExit {
                    log.MarkEnded(ev.SessionID)
                }
            })
        })
    })
}
```
Catatan:
- Pakai `any(eng).(interface{...})` karena type assertion memerlukan nilai antarmuka; jangan memanggil `eng.SetActivitySink` langsung (tidak compile sebelum Wiring). Nama parameter pada tipe fungsi di antarmuka tidak memengaruhi kecocokan; tanda tangan harus identik secara tipe: `func(string, string, map[string]any)`.
- Nama variabel `log` boleh diganti (mis. `alog`) bila menimbulkan bentrok impor paket `log`; file ini tidak mengimpor `log`.
- Hasil `eng.Subscribe` (cancel) boleh diabaikan; daemon hidup sepanjang proses.
- Modul `activity` terpanggil sebelum `status` (alfabetis), maka `Lookup("status.engine")` HARUS di dalam `OnReady` (sudah demikian).
- Jangan membuat goroutine/timer; callback Subscribe dipicu engine.

### 9. Verifikasi

Jalankan `go build ./...` di root repo dan `go vet ./internal/status/ ./cmd/shikigamid/`. Perbaiki error hanya di dua file milik fitur ini. Periksa manual bahwa tidak ada `time.Ticker`/`AfterFunc`/`go func` di kedua file, dan `Publish` dipanggil setelah `Unlock`.

## Perubahan

Dibuat:
- `internal/status/activity.go`
- `cmd/shikigamid/wire_activity.go`

Tidak ada file lain yang diubah.

## Wiring

Yang diekspos untuk agen Wiring (JANGAN dikerjakan di fitur ini):
- Modul `activity` terwire otomatis oleh `wireAll` (tanpa edit `main.go`/`modules.go`).
- Menunggu Wiring (overview bagian 5): `HookPostToolUse` di `types.go`, penambahan ke `hookEvents` di `install.go` (komentar "tepat 4" -> "tepat 5"), dan di `engine.go` field `activitySink` + `func (e *Engine) SetActivitySink(fn func(sessionID, event string, payload map[string]any))` serta pemanggilan sink di `Apply` (setelah `ResolveSessionID`, sebelum `ClassifyEvent`, di luar lock, dengan `recover`). Begitu `SetActivitySink` ada, type assertion di `wire_activity.go` otomatis memasangnya tanpa perubahan kode.
- Layanan `activity.log` tersedia lewat `d.Lookup("activity.log")` bila ada yang butuh.

## Selesai bila

- `go build ./...` sukses (guard utama; `go vet` pada kedua paket bersih bila dijalankan).
- `activity.go` memuat `ActivityEntry`, `ActivityLog`, `NewActivityLog`, `Record`, `List`, `Forget`, `MarkEnded`, `RegisterActivityIPC`, `summarize` sesuai kontrak.
- `List` selalu non-nil; params kosong/`null`/`{}` diterima; JSON tidak valid -> `ErrInvalidParams`; `limit` default 100, maks 200.
- Buffer per sesi maks 200 (tertua terbuang); maks 20 buffer sesi ended; rekaman baru mengaktifkan kembali sesi ended.
- `Record` hanya bereaksi pada `PostToolUse`, mengabaikan `tool_name` kosong/bukan string dan `sessionID` kosong, tidak panic, tidak menyimpan `tool_response`, `Publish` di luar lock.
- `wire_activity.go` memakai type assertion antarmuka lokal untuk `SetActivitySink` dan `Subscribe` (done + exit -> `MarkEnded`); compile tanpa perubahan engine.
- Tidak ada timer/polling/goroutine baru; tidak ada file di luar ownedPaths yang berubah.
