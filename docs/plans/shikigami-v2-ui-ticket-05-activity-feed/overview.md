# Overview: Feed aktivitas (tab Activity)

Tiket: `docs/tickets/shikigami-v2-ui-ticket-05-activity-feed.md`
Spec: `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md` (bagian 4b, 11b)
Stack: daemon Go (`internal/status`, `cmd/shikigamid`) + frontend Svelte 5 (runes) + Vite + TypeScript (`frontend/`). Tanpa dependency baru, tanpa unit test, tanpa polling/timer.

## 1. Ringkasan epic dan urutan eksekusi

Tab "Activity" di panel kanan (di samping "Files") menampilkan tool call terbaru agent claude. Sumber data: event hook `PostToolUse` yang masuk lewat `status.hook` -> `Engine.Apply` -> sink aktivitas -> `ActivityLog` (ring buffer in-memory) -> `srv.Publish("activity.event")` -> UI. Hanya tampilan; tidak ada Approve/Reject (spec 11b).

Urutan:

1. **Foundation**: TIDAK ADA (`hasFoundation=false`, tidak ada `00-foundation.md`). Kontrak IPC/tipe di bagian 2 sudah cukup agar kedua fitur berjalan paralel; UI memakai data tiruan sampai daemon siap. Registri `registerRightTab` sudah ada dari tiket 04.
2. **Fitur paralel**: `activity-daemon` dan `activity-ui` (path terpisah).
3. **Wiring** (sendiri, terakhir): perluasan kecil `internal/status/{install,types,engine}.go` + verifikasi UI + build guard (bagian 5).

## 2. Kontrak bersama

### 2.1 Fakta repo (sudah dibaca; jangan diubah di luar bagian Wiring)

Daemon:
- Modul daemon: `RegisterModule(name, func(srv *ipc.Server, d *Deps))` dari `init()` (`cmd/shikigamid/modules.go`). `wireAll` memanggil modul urut alfabetis, lalu menjalankan semua `d.OnReady`. Modul `activity` < `status` secara alfabetis, jadi `status.engine` HARUS diambil di dalam `d.OnReady` lewat `d.Lookup("status.engine")` (bertipe `*status.Engine`). `d.Provide` panic bila nama dobel.
- `ipc.Server.Register(method, func(ctx context.Context, params json.RawMessage) (any, error))` (panic bila method dobel); `ipc.NewError(ipc.ErrInvalidParams, "...")`; `srv.Publish(topic, payload)` (tidak memblokir).
- `status.Engine`: `Apply(idOrToken, event string, payload map[string]any) error` (dipanggil handler `status.hook`), `ResolveSessionID(idOrToken) (string, bool)`, `Subscribe(fn func(StatusEvent)) (cancel func())`, `Forget(sessionID)` (dipanggil `handleExit` setelah event `done` bersumber `exit`). `StatusEvent{SessionID, Status, Prev, Source, Reason, At}`, konstanta `SourceHook/SourceOutput/SourceExit`, `StatusDone`.
- `ClassifyEvent` (hooks.go) mengembalikan false untuk event tak dikenal; `Apply` saat ini langsung `return nil` pada kasus itu. Karena itu sink aktivitas harus dipanggil SEBELUM pengecekan `known`.
- `internal/status/install.go`: `hookEvents = []string{HookUserPromptSubmit, HookNotification, HookStop, HookSessionStart}` dengan komentar "(tepat 4)"; `writeHooksFile` menghasilkan berkas hooks per sesi claude dengan `hookCommand(...)` yang berakhiran `|| true`. Pencarian `tepat 4` / `len(hookEvents)` hanya menemukan komentar itu.
- `internal/status/types.go` berisi konstanta `HookUserPromptSubmit`, `HookSessionStart`, `HookNotification`, `HookStop`, `HookSubagentStop`.

Frontend:
- `frontend/src/main.ts` memuat `./modules/*/index.ts` secara eager lewat `import.meta.glob`. AKIBAT: `frontend/src/modules/activity/index.ts` otomatis ter-load saat start; TIDAK perlu edit `main.ts` atau `App.svelte`. `modules/shell/register.ts` mengimpor modul secara eksplisit hanya untuk jaminan; tidak diubah epic ini.
- `modules/shell/index.ts` mengekspor `registerRightTab`, `registerTopbarItem`, `rightTabs`, tipe `RightTabSpec { id; title; component: Component<any>; order? }`, dan helper (`focusOrOpenSession(sessionId): boolean`, `sessionTitle(sessionId)`, `findSessionPane`, `basename`, dll.). `registerRightTab` idempoten per id. Tab Files terdaftar dengan `order: 10` (di `modules/sidebar-right/index.ts`); Activity memakai `order: 20`.
- `lib/ipc.ts`: `call<T>(method, params?)`, `onEvent(topicPrefix, fn(topic, payload)): unsubscribe`, `inWails: boolean`. `call` gagal (di luar Wails) harus ditangani diam-diam.
- `lib/stores/index.ts` mengekspor `sessions` (`Session { id, workspaceId, projectId, cwd, providerId, status: 'running'|'exited', createdAt, exitCode? }`), `providers`.
- `modules/status/store.ts`: `statusStore`, `statusOf`; JANGAN memanggil `startStatusSync()` lagi atau membuat pendengar `session.status` kedua.
- Aksi fokus pane: pakai `focusOrOpenSession(sessionId)` dari `modules/shell` (membungkus `layout.setFocus`/`layout.openPane`; tidak perlu impor layout langsung). Mengembalikan `false` bila sesi tidak ada di store `sessions` -> baris dinonaktifkan.
- Tema: variabel CSS `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok`; tanpa warna hardcode (fallback boleh).
- Jangan mengubah file `modules/shell` dan `modules/sidebar-right`.

### 2.2 IPC

- `activity.list` params `{sessionId?: string, limit?: number}`. `limit` bawaan 100, maks 200; `<=0` dianggap bawaan. Params kosong/`null`/`{}` diterima. Hasil: array `ActivityEntry` urut terbaru -> terlama (`ts` menurun). `sessionId` kosong = semua sesi digabung. Sesi tak dikenal -> `[]` (BUKAN `null`, bukan error). JSON tidak valid -> `ipc.ErrInvalidParams`.
- Event `activity.event` payload `ActivityEntry`, dipublikasikan via `srv.Publish("activity.event", entry)`. Frontend: `onEvent('activity.event', (topic, payload) => ...)`.

### 2.3 Tipe

Go (`internal/status/activity.go`, paket `status`):

```go
type ActivityEntry struct {
    TS        int64  `json:"ts"`        // unix milidetik
    SessionID string `json:"sessionId"` // id sesi daemon (sudah di-resolve dari token)
    Tool      string `json:"tool"`
    Summary   string `json:"summary"`   // maks 160 rune, satu baris
}
type ActivityLog struct { /* map sessionID -> ring buffer (kapasitas 200), sync.Mutex */ }
func NewActivityLog(srv *ipc.Server) *ActivityLog
func (a *ActivityLog) Record(sessionID, event string, payload map[string]any) // hanya event "PostToolUse"
func (a *ActivityLog) List(sessionID string, limit int) []ActivityEntry       // selalu non-nil
func (a *ActivityLog) Forget(sessionID string)
func RegisterActivityIPC(srv *ipc.Server, a *ActivityLog)
```

TypeScript (`frontend/src/modules/activity/types.ts`, diekspor ulang dari `index.ts`):

```ts
export interface ActivityEntry { ts: number; sessionId: string; tool: string; summary: string }
```

Sink engine (didefinisikan di Wiring, bentuk dikunci di sini):

```go
// internal/status/engine.go
func (e *Engine) SetActivitySink(fn func(sessionID, event string, payload map[string]any))
```

`ActivityLog.Record` memenuhi bentuk itu sehingga dipasang `eng.SetActivitySink(log.Record)`.

### 2.4 Aturan ringkasan

`summarize(tool, tool_input)` (Go di `activity.go`; UI tidak menghitung ulang, hanya menampilkan `summary`):
`Bash` -> `command`; `Read`/`Edit`/`Write`/`NotebookEdit` -> `file_path`; `Grep`/`Glob` -> `pattern`; `WebFetch` -> `url`; `Task` -> `description`; lainnya -> nama field string pertama (urut kunci alfabetis agar deterministik) atau `""`. Newline (`\r`, `\n`) diganti spasi, dipotong 160 rune. `tool_input` bukan objek atau field bukan string -> `""`. `tool_response` TIDAK pernah dibaca/disimpan/dikirim.

### 2.5 Perilaku penting yang dikunci

- Ring buffer per sesi kapasitas 200; entri tertua terbuang lebih dulu. `List` gabungan: kumpulkan semua buffer, urut `ts` menurun (stabil), potong `limit`.
- `Record` mengabaikan: event selain `PostToolUse`, `tool_name` kosong/bukan string, `sessionID` kosong. Tidak pernah panic pada payload aneh (type assertion dengan `, ok`). Publish dilakukan di luar mutex.
- Pembersihan buffer: callback `Forget` engine tidak tersedia bagi modul tanpa mengubah engine lebih jauh, dan `Forget` engine terjadi tepat setelah `done` bersumber exit (riwayat sesi yang baru selesai akan hilang seketika bila dihapus saat itu). Keputusan: `ActivityLog` memiliki method internal `MarkEnded(sessionID)` (tidak diekspor ke kontrak publik tambahan selain di paket) dan menjaga paling banyak 20 buffer sesi "ended"; yang tertua (urutan MarkEnded) dibuang. `wire_activity.go` di `OnReady` memakai `eng.Subscribe` dan memanggil `MarkEnded` untuk `StatusEvent` dengan `Status == StatusDone && Source == SourceExit`. Tanpa timer. `Forget(sessionID)` publik menghapus buffer langsung (dipakai pembersihan/ujian). Rekaman baru ke sesi yang "ended" mengembalikannya ke status aktif.
- UI: store maks 300 entri di memori (terbaru di depan); `activity.list` dipanggil tepat sekali saat inisialisasi store (lazy saat komponen tab pertama dipasang, dijaga flag modul agar mount ulang tidak memanggil ulang); langganan `activity.event` dibuat sekali. Dedupe entri riwayat vs event (kunci `ts|sessionId|tool|summary`) agar entri yang tiba antara subscribe dan list tidak ganda; subscribe lebih dulu, baru `list`, lalu gabung.
- Nama sesi di UI: `sessionTitle(sessionId)` dari `modules/shell`; bila hasilnya kosong, 8 karakter pertama id. Sesi dianggap "ada" bila ada di store `sessions` (dan bukan `exited` tidak disyaratkan; cukup ada di store).
- Tab: `registerRightTab({ id: 'activity', title: 'Activity', order: 20, component: ActivityTab })`, dilakukan di `modules/activity/index.ts` (idempoten).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|------|-------|------------|
| `activity-daemon` | `ActivityLog` ring buffer + `summarize` + `Record/List/Forget/MarkEnded`, IPC `activity.list`, event `activity.event`, modul daemon `activity` (Provide `activity.log`, RegisterActivityIPC, OnReady: `SetActivitySink` + langganan `Subscribe` untuk `MarkEnded`). | `internal/status/activity.go`, `cmd/shikigamid/wire_activity.go` |
| `activity-ui` | Modul `modules/activity`: `types.ts`, `store.ts` (list sekali + `onEvent`, batas 300, filter sesi di klien), `ActivityTab.svelte`, `ActivityRow.svelte` (opsional), `index.ts` (registerRightTab, ekspor tipe). Klik baris -> `focusOrOpenSession`; sesi hilang -> baris nonaktif; keadaan kosong "Belum ada aktivitas"; auto-scroll menempel atas. | `frontend/src/modules/activity/` |

Catatan `activity-daemon`: karena `activitySink` belum ada di engine sampai Wiring, `wire_activity.go` memanggil `eng.SetActivitySink(...)` yang baru ada setelah Wiring; agar `go build ./...` tetap lulus sebelum Wiring, agen fitur TIDAK boleh menyentuh file engine. Solusinya antarmuka lokal: di `wire_activity.go` lakukan type assertion `if s, ok := v.(interface{ SetActivitySink(func(string, string, map[string]any)) }); ok { s.SetActivitySink(log.Record) }` sehingga kode ter-compile sebelum dan sesudah Wiring. Gunakan pola assertion ini (wajib). Bila assertion gagal, `d.Logger.Warn` (cek `d.Logger != nil`).

Catatan `activity-ui`: tanpa Wails, `call` gagal -> tangkap, keadaan kosong, tanpa `console.error` berulang (maks satu `console.debug`/tanpa log).

## 4. Foundation

Tidak ada. Tidak ada skeleton paket atau tipe bersama yang harus ada lebih dulu: tipe Go dan TS didefinisikan di dalam ownedPaths masing-masing fitur dan hanya dihubungkan lewat kontrak IPC JSON di bagian 2.

## 5. Wiring checklist (dikerjakan agen wiring setelah kedua fitur selesai)

Daemon (file tiket lain, perubahan sekecil mungkin, tanpa mengubah klasifikasi status):
1. `internal/status/types.go`: tambah konstanta `HookPostToolUse = "PostToolUse"`.
2. `internal/status/install.go`: tambah `HookPostToolUse` ke `hookEvents` (menjadi 5 event) dan perbarui komentar "(tepat 4)" menjadi "(tepat 5)". Jalankan `grep -rn "tepat 4\|len(hookEvents)" internal cmd` untuk memastikan tidak ada asumsi jumlah lain. Perbarui juga komentar daftar event pada doc `writeHooksFile` bila menyebut empat event. Flag izin agent tidak disentuh.
3. `internal/status/engine.go`: tambah field `activitySink func(sessionID, event string, payload map[string]any)` (dilindungi `e.mu` saat set/baca), method `SetActivitySink(fn)`, dan di `Apply` setelah `ResolveSessionID` berhasil dan SEBELUM `ClassifyEvent`: salin sink di bawah lock, lalu panggil di luar lock dalam fungsi dengan `defer recover()` (log `slog.Warn`) dengan `id` ter-resolve, `event`, `payload`. `ClassifyEvent` tetap false untuk `PostToolUse` sehingga `hookSeen`/status/timer tidak berubah. `hooks.go` tidak diubah.
4. Konfirmasi `wire_activity.go` ter-wire otomatis (nama modul `activity` unik; tidak ada edit `main.go`/`modules.go`).

UI:
5. Konfirmasi tidak ada edit diperlukan: `main.ts` memuat `modules/activity/index.ts` lewat glob eager. Pastikan `index.ts` memanggil `registerRightTab` dan bebas error saat dimuat sebelum `RightPanel` dipasang (registri adalah store, aman). Tidak mengubah `modules/shell/register.ts` kecuali bukti nyata bahwa glob tidak memuatnya; bila perlu, satu baris `import '../activity';` di `register.ts` (dicatat sebagai perluasan kecil).
6. Verifikasi manual (bila memungkinkan): jalankan aplikasi, buat sesi claude, minta satu perintah, cek baris muncul di tab Activity, filter, klik fokus pane, dan status sesi tetap berubah running/done seperti sebelumnya.

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` (di root repo).
- `make build` (mencakup `cd frontend && npm install && npm run build`, build daemon dan app).
- Untuk agen `activity-ui` saja: `cd frontend && npm run build` boleh dipakai sebagai guard cepat (tanpa error tipe).
- Agen `activity-daemon` cukup `go build ./...` (dan `go vet ./internal/status/ ./cmd/shikigamid/` bila ingin).

## 7. Aturan paralel

- Tiap agen hanya membuat/mengubah `ownedPaths` fiturnya: `activity-daemon` = `internal/status/activity.go`, `cmd/shikigamid/wire_activity.go`; `activity-ui` = `frontend/src/modules/activity/`.
- Dilarang menyentuh path fitur lain dan file wiring (`internal/status/{install,types,engine,hooks}.go`, `modules/shell/**`, `modules/sidebar-right/**`, `main.ts`, `App.svelte`, `Makefile`). Kebutuhan perubahan di sana dicatat untuk agen Wiring, tidak dikerjakan sendiri.
- Tanpa polling: tidak ada `time.Ticker`/`time.AfterFunc`/goroutine pengambil data di daemon, tidak ada `setInterval`/`setTimeout` berulang di UI.
- Konvensi nama: file Go `snake_case.go`; komponen Svelte `PascalCase.svelte`; util/store `camelCase.ts`; hanya `index.ts` yang diimpor modul lain; identifier kode berbahasa Inggris, komentar/teks UI mengikuti ticket (teks UI Indonesia: "Belum ada aktivitas", "Semua sesi").
- Tidak ada tombol/teks Approve/Reject; `tool_response` tidak pernah disimpan.
