# Plan: Hook edit, review.files, dan event review.changed

## Metadata

- Tiket: `agent-diff-review-ticket-01-review-capture` (`docs/tickets/agent-diff-review-ticket-01-review-capture.md`)
- Fitur: `review-hooks-ipc`
- ownedPaths: `internal/review/ipc.go`, `cmd/shikigamid/wire_review.go`, `internal/status/install.go`, `internal/status/hooks.go`
- Depends on foundation: ya. `internal/review/types.go`, `store.go`, `capture.go` (stub dengan signature final) harus sudah ada dari `00-foundation.md`. Jika belum ada saat mulai, hentikan dan lapor; jangan membuatnya sendiri (itu milik Foundation dan `review-foundation`).
- Rujukan wajib: `docs/plans/agent-diff-review-ticket-01-review-capture/overview.md` (bagian 2.2 butir 1-6, 9, 10; 2.3; 2.4). Plan ini tidak mengulang isinya.

## Tujuan

1. Matcher hook `PreToolUse` per sesi mencakup tool edit (`Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit`).
2. IPC `review.files {sessionId}` mengembalikan `{supported, files}`.
3. Daemon menangkap baseline pada `PreToolUse` tool edit, mencatat hash pasca-edit pada `PostToolUse`, dan mempublikasikan `review.changed {sessionId, count}` dengan debounce 300 ms per sesi.
4. Retensi awal (`Prune`) dan pembersihan timer saat sesi exit.

Tidak mengubah `ClassifyEvent`, `internal/runs`, maupun perilaku `PostToolUse` yang ada (sudah diverifikasi aman, lihat overview 2.2 butir 2).

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `internal/review/ipc.go` (baru)
- `cmd/shikigamid/wire_review.go` (baru)
- `internal/status/install.go` (ubah konstanta dan komentar)
- `internal/status/hooks.go` (hanya bila butuh helper; default: tidak diubah)

Jangan menyentuh: `internal/review/types.go|store.go|capture.go` (milik foundation), `frontend/**`, `cmd/shikigamid/modules.go`, `internal/runs/**`, `ClassifyEvent`, berkas tiket 02-04 (`diff*.go`, `actions*.go`, `comment.go`), maupun file wiring lain. Bila signature Store terasa keliru, tulis catatan di laporan, jangan ubah berkas foundation.

## Kontrak

Disediakan (nama final, lihat overview 2.4):

- `review.RegisterIPC(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool)` di `internal/review/ipc.go`; mendaftarkan metode `review.files` (`IPCMethodFiles`).
- Hasil `review.files`: `FilesResult{Supported bool, Files []FileEntry}`, `files` selalu `[]` (bukan `null`).
- Event `review.changed` (`EventChanged`) dengan payload `map[string]any{"sessionId": id, "count": n}`; `n` = jumlah `FileEntry` dengan `Reviewed == false`.
- Layanan daemon: `d.Provide("review.store", store)` (`*review.Store`), dipanggil sekali di fase wire (bukan di dalam `OnReady`) agar tiket 02-04 dapat `d.Lookup("review.store")` di `OnReady` mereka.
- Modul daemon `"review"` lewat `RegisterModule`.

Dipakai:

- Dari foundation (stub saat ini, implementasi penuh menyusul): `review.NewStore(dataDir)`, `(*Store).Capture(sessionID, cwd, path)`, `MarkChanged(sessionID, cwd, path)`, `Files(sessionID, cwd)`, `Cwd(sessionID)`, `Prune(keepEnded int, activeSessionIDs []string)`, konstanta `IPCMethodFiles`, `EventChanged`, tipe `FilesResult`, `FileEntry`.
- Dari `internal/status`: `*status.Engine` (`d.Lookup("status.engine")`), `AddHookListener(fn func(sessionID, event string, payload map[string]any)) (cancel func())`, `AddExitListener(fn func(sessionID string)) (cancel func())`, `Lookup(sessionID) (session.Session, bool)` (field `Cwd`, `ProviderID`), `List() []SessionState` (field `SessionID`), konstanta `status.HookPreToolUse` ("PreToolUse") dan `status.HookPostToolUse` ("PostToolUse").
- Dari `internal/provider`: `provider.Get(id) (Provider, bool)`, `Capabilities().Hooks`.
- Dari `internal/ipc`: `ipc.NewError(code, msg)`, `ipc.ErrInvalidParams`, `ipc.ErrInvalid`, `(*ipc.Server).Register`, `Publish`.
- Pola modul: ikuti `cmd/shikigamid/wire_runs.go` (log `Warn` bila `status.engine` tidak tersedia).

## Langkah

### 1. `internal/status/install.go`: matcher

1. Ubah konstanta (sekitar baris 32):
   `const preToolUseMatcher = "Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit"`
2. Perbarui komentar di atasnya: PreToolUse dibatasi ke tool Workflow, Agent (registry runs) dan tool edit Edit, Write, MultiEdit, NotebookEdit (baseline review); matcher Claude adalah regex atas nama tool sehingga proses `shikigamictl` tidak jalan pada tool lain.
3. Perbarui komentar `hookEvents` (baris 21-23) dan komentar `writeHooksFile` (baris ~73-74) agar tidak lagi menyebut "hanya Workflow dan Agent"; tambahkan bahwa `ClassifyEvent` tetap hanya mengenal lima event status lama sehingga PreToolUse tool edit tidak mengubah status.
4. Tidak ada logika merge: berkas hooks dibuat baru per sesi, jadi matcher lama otomatis tergantikan (tidak ada duplikasi). Jangan ubah logika lain di berkas ini.
5. `internal/status/hooks.go`: jangan diubah kecuali benar-benar perlu. Helper ekstraksi path diletakkan di `wire_review.go`, bukan di sini.

### 2. `internal/review/ipc.go`

Package `review`. Impor: `context`, `encoding/json`, `strings`, `log/slog` (bila perlu), `internal/ipc`.

```go
// RegisterIPC mendaftarkan metode IPC review.files.
func RegisterIPC(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool)
```

Perilaku handler `review.files` (`srv.Register(IPCMethodFiles, func(ctx context.Context, params json.RawMessage) (any, error))`):

1. Decode `params` ke `struct{ SessionID string \`json:"sessionId"\` }`. Gagal decode -> `ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh {\"sessionId\": string}")`. `SessionID` kosong (setelah `strings.TrimSpace`) -> `ipc.NewError(ipc.ErrInvalid, "sessionId wajib diisi")`.
2. Bila `supportedFn == nil` atau `!supportedFn(id)` -> kembalikan `FilesResult{Supported: false, Files: []FileEntry{}}`.
3. Cwd: `cwd := store.Cwd(id)`. Overview 2.2 butir 3 menyebut fallback ke `Lookup` bila index belum ada; handler ini tidak punya akses engine, jadi bila `cwd == ""` kembalikan `FilesResult{Supported: true, Files: []FileEntry{}}` (belum ada sentuhan = daftar kosong, benar secara semantik karena tanpa index tidak ada entri). Tulis komentar yang menjelaskan hal ini.
4. `files, err := store.Files(id, cwd)`. Galat -> kembalikan `ipc.NewError(code, "gagal membaca daftar review: "+err.Error())` dengan `code` konstanta galat internal di `internal/ipc/errors.go` bila ada; bila tidak ada, pakai `ipc.ErrInvalid`. `files == nil` dinormalisasi menjadi `[]FileEntry{}`.
5. Kembalikan `FilesResult{Supported: true, Files: files}`.

Hanya fungsi ini yang dimiliki berkas ini; jangan menambah metode lain (milik tiket 02-04, berkas lain).

### 3. `cmd/shikigamid/wire_review.go`

Package `main`. Semua helper tak-diekspor diawali `reviewWire` (overview 2.1). Impor: `encoding/json` (tidak perlu bila tak dipakai), `path/filepath`, `strings`, `sync`, `time`, `log/slog`, `internal/ipc`, `internal/provider`, `internal/review`, `internal/status`.

#### 3.1 Komentar spike `tool_input` (tulis di kepala berkas, bahasa Indonesia)

Catat hasil verifikasi format. Kerjakan spike singkat: baca dokumentasi/skema tool Claude Code yang tersedia di repo atau sesi (mis. cari contoh payload di `internal/status/*_test.go`, `docs/`, atau log aktivitas `internal/activity`) tanpa menjalankan jaringan. Bentuk yang diasumsikan dan harus dicatat sebagai komentar (tandai "belum diverifikasi di lapangan" bila tidak bisa dibuktikan):

- `Edit`, `Write`: `tool_input.file_path` (path absolut).
- `MultiEdit`: `tool_input.file_path` + `edits[]` (path tunggal untuk seluruh edit).
- `NotebookEdit`: `tool_input.notebook_path` (versi dokumentasi Claude Code); beberapa versi memakai `file_path`. Ekstraksi mencoba `notebook_path` lalu `file_path`.

Bila kunci path tidak ditemukan atau bukan string non-kosong, event diabaikan tanpa galat (log Debug).

#### 3.2 Helper

```go
var reviewWireEditTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

// reviewWireToolPath mengambil tool_name (harus tool edit) dan path dari payload hook.
func reviewWireToolPath(payload map[string]any) (tool, path string, ok bool)
```

- `tool_name` dibaca sebagai string; bukan tool edit -> `ok=false`.
- `tool_input` dibaca sebagai `map[string]any`; urutan kunci: untuk `NotebookEdit` `notebook_path` lalu `file_path`; untuk lainnya `file_path`. Nilai harus string non-kosong. Tidak panic pada tipe tak terduga (gunakan type assertion dengan `, ok`).

```go
// reviewWireCwd menentukan cwd sesi: Lookup(sessionID).Cwd, lalu payload["cwd"].
func reviewWireCwd(eng *status.Engine, sessionID string, payload map[string]any) string
```

- `s, ok := eng.Lookup(sessionID)`; bila `ok && s.Cwd != ""` pakai itu; selain itu `payload["cwd"]` string; bila kosong kembalikan `""` (pemanggil mengabaikan event, log Debug).

```go
// reviewWireSupported memetakan sesi -> dukungan hooks provider.
func reviewWireSupported(eng *status.Engine) func(sessionID string) bool
```

- `s, ok := eng.Lookup(id)`; `!ok` -> false; `p, ok := provider.Get(s.ProviderID)`; `!ok` -> false; kembalikan `p.Capabilities().Hooks`.

#### 3.3 Debouncer

```go
type reviewWireDebouncer struct {
    mu     sync.Mutex
    timers map[string]*time.Timer
    delay  time.Duration // 300 * time.Millisecond
}
func (b *reviewWireDebouncer) schedule(sessionID string, fn func())  // reset timer sesi; time.AfterFunc; hapus entri saat fn akan jalan
func (b *reviewWireDebouncer) forget(sessionID string)               // Stop timer dan hapus entri
```

- `schedule`: di bawah mutex, bila ada timer lama `Stop()` lalu buat `time.AfterFunc(delay, ...)`. Di callback: ambil mutex, hapus entri hanya bila entri masih timer itu (bandingkan pointer; hindari menghapus timer pengganti), lepas mutex, lalu panggil `fn()`. Callback dilindungi `defer recover()` (tanpa panic ke daemon).
- `forget`: di bawah mutex `Stop()` dan `delete`.

#### 3.4 Modul

```go
func init() {
    RegisterModule("review", func(srv *ipc.Server, d *Deps) {
        store := review.NewStore(d.DataDir)
        d.Provide("review.store", store)   // sekali, di fase wire
        d.OnReady(func() { ... })
    })
}
```

Di dalam `OnReady`:

1. `v, ok := d.Lookup("status.engine")`; `eng, ok2 := v.(*status.Engine)`; bila gagal log `Warn("review: status.engine tidak tersedia")` (cek `d.Logger != nil`) dan `return`.
2. `review.RegisterIPC(srv, store, reviewWireSupported(eng))`.
3. Logger lokal: `log := d.Logger; if log == nil { log = slog.Default() }`.
4. `deb := &reviewWireDebouncer{timers: map[string]*time.Timer{}, delay: 300 * time.Millisecond}`.
5. Listener hook (`eng.AddHookListener(func(sessionID, event string, payload map[string]any) {...})`), tidak pernah panik atau mengembalikan galat, semua galat hanya di-log Debug/Warn:
   - `tool, path, ok := reviewWireToolPath(payload)`; `!ok` -> return (ini mengabaikan semua tool selain empat tool edit, termasuk Workflow/Agent).
   - `cwd := reviewWireCwd(eng, sessionID, payload)`; kosong -> Debug "review: cwd kosong" dan return.
   - `event == status.HookPreToolUse`: `store.Capture(sessionID, cwd, path)` sinkron dan cepat (target < 20 ms); galat -> `log.Debug`.
   - `event == status.HookPostToolUse`: `store.MarkChanged(sessionID, cwd, path)`; galat -> `log.Debug`. Lalu `deb.schedule(sessionID, func(){ files, err := store.Files(sessionID, store.Cwd(sessionID) /* fallback cwd bila kosong */); ...; srv.Publish(review.EventChanged, map[string]any{"sessionId": sessionID, "count": n}) })` dengan `n` = jumlah `!f.Reviewed`. Ambil cwd untuk `Files` dari `store.Cwd(sessionID)`, jatuh ke `cwd` event bila kosong. Galat `Files` -> `log.Warn` dan tidak publish.
   - Event lain -> abaikan.
6. Listener exit: `eng.AddExitListener(func(sessionID string){ deb.forget(sessionID) })`. Timer tertunda dibatalkan; tidak ada publish susulan setelah sesi exit.
7. Prune awal tanpa memblokir: `go func(){ defer recover; ids := make([]string, 0); for _, st := range eng.List() { ids = append(ids, st.SessionID) }; store.Prune(20, ids) }()`. Bila `SessionState` ternyata tak punya `SessionID`, baca `internal/status/types.go` dan sesuaikan (saat plan ditulis field itu ada).

Catatan: nilai balik `cancel` dari `AddHookListener`/`AddExitListener` boleh diabaikan (daemon berumur panjang, sama seperti `wire_runs.go`).

### 4. Verifikasi

Jalankan dari root repo: `gofmt -l cmd/shikigamid internal/review internal/status` (harus kosong untuk berkas yang disentuh), lalu `go build ./...`. Perbaiki impor tak terpakai. Bila `go build` gagal karena simbol foundation hilang, itu masalah prasyarat (lihat Metadata), bukan alasan membuat berkas foundation.

## Perubahan

- Baru: `internal/review/ipc.go`
- Baru: `cmd/shikigamid/wire_review.go`
- Diubah: `internal/status/install.go` (konstanta `preToolUseMatcher` dan komentar `hookEvents`, `preToolUseMatcher`, `writeHooksFile`)
- `internal/status/hooks.go`: tidak diubah (kecuali ternyata butuh helper; catat bila berubah)

## Wiring

Fitur ini mengekspos, untuk disambungkan/diverifikasi oleh `review-capture-wiring` (JANGAN dikerjakan di sini; jangan menyentuh `modules.go`, `main.ts`, `SessionRow.svelte`):

- Modul daemon `"review"` terdaftar otomatis lewat `init()` di `wire_review.go` (semua berkas package `main` ikut dikompilasi); wiring hanya memverifikasi nama unik dan `Provide("review.store")` sekali.
- Layanan `review.store` (`*review.Store`) untuk `d.Lookup` oleh tiket 02-04.
- IPC `review.files` dan event `review.changed {sessionId, count}` untuk dikonsumsi `review-ui-list`.
- Matcher hook baru berlaku hanya untuk sesi yang dibuat setelah daemon baru berjalan.

## Selesai bila

- `go build ./...` lulus (guard satu-satunya; tanpa unit test, tanpa dependency baru).
- `preToolUseMatcher == "Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit"` dan komentar terkait diperbarui; tidak ada logika merge ditambahkan.
- `review.RegisterIPC` ada dengan signature persis di Kontrak; `review.files` mengembalikan `supported:false, files:[]` untuk provider tanpa hooks dan `files` tidak pernah `null`.
- `wire_review.go` memuat `RegisterModule("review", ...)`, `d.Provide("review.store", store)`, listener Pre (Capture) dan Post (MarkChanged + debounce 300 ms per sesi + Publish `review.changed`), `AddExitListener` yang membersihkan timer, `Prune(20, ids)` awal di goroutine, dan komentar spike format `tool_input` MultiEdit/NotebookEdit.
- Hanya empat path milik fitur ini yang berubah (`git status` tidak menunjukkan berkas lain tersentuh oleh pekerjaan ini); `ClassifyEvent` dan `internal/runs` tidak berubah.
