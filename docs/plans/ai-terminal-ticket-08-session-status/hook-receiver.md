# Plan: Penerima event hooks

## Metadata

- Tiket: `ai-terminal-ticket-08-session-status` (`docs/tickets/ai-terminal-ticket-08-session-status.md`)
- Fitur: `hook-receiver` (Penerima event hooks)
- ownedPaths: `internal/status/hooks.go`, `internal/status/install.go`
- Depends on foundation: ya (`00-foundation.md` membuat `types.go` dan stub kedua file ini; tugas Anda mengganti stub miliknya dengan implementasi nyata)
- Rujukan desain bersama: `overview.md` bagian 2.1 (T1 launch token), 2.3 (tipe), 2.4 (signature), 2.7 (CLI dan pemasangan hooks), 7 (aturan paralel).

## Tujuan

Menyediakan sisi masuk hooks provider:
1. `hooks.go`: struktur `HookPayload`, `ParseHookEvent` (toleran terhadap stdin kosong, bukan JSON, atau berisi field tak dikenal; tidak pernah panic), dan `ClassifyEvent`.
2. `install.go`: `InstallHooks()` yang mendaftarkan launch contribution. Contribution mengisi env `AITERM_SESSION_ID` (berisi launch token, bukan ID sesi asli) dan `AITERM_SOCK` untuk SEMUA provider; khusus provider `claude` menulis berkas hooks per sesi dan menambahkan `--settings <path>` ke `ctx.Args`. Juga `CleanupHooks(token)`.

Tanpa polling, tanpa goroutine latar, tanpa menulis ke `~/.claude/` atau `.claude/` project.

## Path yang dimiliki

HANYA berikut yang boleh disentuh:
- `internal/status/hooks.go`
- `internal/status/install.go`

Jangan sentuh: `types.go`, `engine.go`, `idle.go`, `ipc.go`, `notify.go`, `cmd/aitermd/wire_status.go`, `cmd/aitermctl/cmd_status.go`, `frontend/`, `internal/session|provider|ipc|paths`, `go.mod`/`go.sum`, atau file wiring. Bila butuh tipe/konstanta baru, definisikan sebagai identifier privat di dua file ini. Tanpa unit test. Hanya stdlib dan paket proyek.

## Kontrak

Disediakan (signature persis, overview 2.4; jangan diubah):
```go
type HookPayload struct { /* field opsional, toleran field tak dikenal */ }
func ParseHookEvent(name string, stdin []byte) (event string, payload map[string]any)
func ClassifyEvent(event string) (Status, bool)
func InstallHooks()
func CleanupHooks(token string)
```
`ClassifyEvent` sudah final dari Foundation: UserPromptSubmit/SessionStart -> `StatusRunning`; Notification -> `StatusWaiting`; Stop -> `StatusDone`; SubagentStop dan lainnya -> `("", false)`. Pertahankan perilaku ini (boleh dirapikan/dikomentari).

Dipakai dari luar:
- `status.RegisterLaunch(LaunchInfo)` (fungsi paket milik `status-engine`, no-op bila engine default nil). `LaunchInfo{Token, WorkspaceID, ProjectID, Cwd, ProviderID, At}`; lihat `types.go`.
- Konstanta `Hook*` dan `Status*` dari `types.go`.
- `provider.RegisterLaunchContribution(fn func(*provider.LaunchContext))`; `provider.LaunchContext{Session session.SessionSpec, Provider provider.Provider, Env map[string]string, Args []string, MCPServers map[string]any}`; `provider.IDClaude`; `ctx.Provider.ID()`, `ctx.Provider.Capabilities().Hooks`. Catatan dari kode nyata: `ctx.Env` tidak nil saat contribution berjalan, `ctx.Args` bisa nil (append langsung), dan `claude` provider menyalin `ctx.Args` lalu menambah `--mcp-config` sesudahnya.
- `session.SessionSpec` (field `WorkspaceID`, `ProjectID`, `Cwd`, `ProviderID`; TIDAK ada ID; baca `internal/session/types.go` untuk nama persis).
- `paths.DataDir() (string, error)` dan `paths.SocketPath() (string, error)` (paket `aiterm/internal/paths`).

Dipakai oleh fitur lain (jangan ubah perilakunya): `status-engine` memanggil `CleanupHooks(token)` dari `Engine.Forget` dan `ParseHookEvent`/`ClassifyEvent` bila perlu; wiring memanggil `InstallHooks()` tepat sekali.

## Langkah

### 1. Persiapan
Baca `internal/status/types.go`, stub `hooks.go`/`install.go` dari Foundation, `internal/provider/launch.go`, `internal/provider/claude*.go` (cara `Command` memakai `ctx.Args`), `internal/session/types.go` (`SessionSpec`), `internal/paths`. Bila nama berbeda dari plan ini, ikuti kode nyata.

### 2. Verifikasi skema hooks dan `--settings` (wajib, catat di komentar kode)
Verifikasi terhadap dokumentasi Claude Code yang berlaku (cari di docs.anthropic.com / code.claude.com: halaman "Hooks reference", "Settings", dan "CLI reference"; gunakan WebFetch/WebSearch, dan bila ada binary `claude`, `claude --help`). Pastikan:
- Skema berkas: objek `{"hooks": {"<EventName>": [ {"matcher": "<opsional>", "hooks": [ {"type":"command","command":"<cmd>"} ]} ]}}`. `matcher` opsional dan tidak relevan untuk `UserPromptSubmit`, `Stop`, `SessionStart`; untuk `Notification` boleh dihilangkan (cocok semua).
- Event `UserPromptSubmit`, `Notification`, `Stop`, `SessionStart` ada dan hook command menerima payload JSON lewat stdin (`session_id`, `hook_event_name`, `transcript_path`, `cwd`, untuk Notification juga `message`; `Stop` juga `stop_hook_active`; SessionStart juga `source`).
- Flag `--settings <file-or-json>` menerima path berkas JSON (atau string JSON), dan konfigurasi dari flag ini DIGABUNG dengan settings user/project, bukan menggantinya.
- Perilaku exit code hook (exit 0 tidak mengganggu agent); itu alasan akhiran `|| true`.
Tulis hasil verifikasi (tanggal, sumber, hasil, serta bagian yang tak bisa dipastikan) sebagai komentar di atas fungsi penulis berkas hooks di `install.go`. Bila dokumentasi ternyata berbeda dari asumsi di atas (mis. `--settings` hanya menerima string JSON), sesuaikan implementasi (mis. sisipkan JSON inline sebagai argumen) dan catat penyimpangannya. Bila tidak bisa memverifikasi (tanpa akses jaringan), tetap implementasikan skema di atas dan tulis eksplisit di komentar bahwa verifikasi tidak dapat dilakukan serta apa yang diasumsikan; jangan berhenti karena hal ini.

### 3. `hooks.go`
- Komentar paket sudah ada di `types.go`; jangan duplikasi.
- `type HookPayload struct`: field opsional dengan tag `json:"...,omitempty"`: `SessionID string "session_id"`, `HookEventName string "hook_event_name"`, `Message string "message"`, `Cwd string "cwd"`, `TranscriptPath string "transcript_path"`, `Source string "source"`, `StopHookActive bool "stop_hook_active"`, `Prompt string "prompt"`, `NotificationType string "notification_type"`. `encoding/json` default mengabaikan field tak dikenal, itu perilaku yang dimaksud; dokumentasikan.
- `func ParseHookEvent(name string, stdin []byte) (event string, payload map[string]any)`:
  - `event`: `strings.TrimSpace(name)`; bila kosong, coba isi dari `hook_event_name` pada payload hasil parse; bila tetap kosong kembalikan `""`.
  - `payload`: JANGAN pernah nil. Trim stdin (batasi pemrosesan ke 1 MiB pertama; potong bila lebih). Bila kosong -> `map[string]any{}`. `json.Unmarshal` ke `map[string]any`; bila gagal atau hasilnya bukan objek (array, string, `null`) -> `map[string]any{}`. Bila stdin valid, payload adalah objek itu apa adanya (field tak dikenal dipertahankan). Opsional: validasi tambahan dengan `HookPayload` hanya untuk melengkapi `event` dari `hook_event_name`.
  - Tidak ada panic pada input apa pun (nil slice, biner, JSON terpotong).
  - Fungsi ini tidak melakukan I/O.
- `func ClassifyEvent(event string) (Status, bool)`: `switch` pada konstanta `Hook*`; hasil sesuai tabel di bagian Kontrak; default `("", false)`.

### 4. `install.go`
Impor yang diharapkan: `crypto/rand`, `encoding/hex`, `encoding/json`, `log/slog`, `os`, `os/exec`, `path/filepath`, `strings`, `time`, `aiterm/internal/paths`, `aiterm/internal/provider`.

Fungsi privat dan publik (urutan):

a. `func newToken() (string, error)`: 8 byte `crypto/rand` -> `hex.EncodeToString` (16 hex). Bila `rand.Read` gagal kembalikan error.

b. `func shellQuote(s string) string`: bungkus dengan `'`, ganti tiap `'` dengan `'\''`. Contoh: `/Users/a b/it's` -> `'/Users/a b/it'\''s'`.

c. `func aitermctlPath() string`: (1) `os.Executable()`; bila sukses, `filepath.Join(filepath.Dir(exe), "aitermctl")` dan cek `os.Stat` ada (bukan direktori); pakai bila ada; (2) `exec.LookPath("aitermctl")`; (3) fallback string `"aitermctl"`. Tidak di-cache dengan cara yang menyulitkan; boleh dihitung sekali per contribution.

d. `func hookCommand(ctl, token, event string) string`: `shellQuote(ctl) + " status hook --session " + token + " --event " + event + " || true"`. Token hex dan event dari daftar tetap sehingga aman tanpa quote. Akhiran `|| true` wajib (alasan: `aitermctl` keluar kode 1 bila daemon mati sebelum command jalan; lihat overview 2.7).

e. `var hookEvents = []string{HookUserPromptSubmit, HookNotification, HookStop, HookSessionStart}` (konstanta dari `types.go`; tepat 4 event).

f. `func hooksFilePath(dataDir, token string) string`: `filepath.Join(dataDir, "run", "status-hooks-"+token+".json")`.

g. `func writeHooksFile(dataDir, token, ctl string) (string, error)`:
   - Susun struktur: `map[string]any{"hooks": map[string]any{ev: []any{ map[string]any{"hooks": []any{ map[string]any{"type":"command","command": hookCommand(ctl, token, ev)} }} } ...}}`.
   - `json.MarshalIndent`, `os.MkdirAll(filepath.Join(dataDir,"run"), 0o700)` lalu `os.Chmod` ke 0700 bila perlu (MkdirAll tunduk umask/direktori lama), tulis berkas dengan `os.WriteFile(path, data, 0o600)` (tulis ke berkas sementara di dir yang sama lalu `os.Rename` agar atomik; opsional, boleh langsung WriteFile). Kembalikan path.
   - Letakkan komentar hasil verifikasi (langkah 2) di sini.

h. `func CleanupHooks(token string)`: bila token kosong, return. Validasi token hanya berisi karakter hex/`[A-Za-z0-9_-]` (cegah path traversal; bila tidak valid, return). `dir, err := paths.DataDir()`; bila error return. `os.Remove(hooksFilePath(dir, token))`; abaikan `os.IsNotExist`, log `slog.Debug` untuk error lain. Aman dipanggil berulang dan konkuren.

i. `func removeStaleHooks()`: `dir, err := paths.DataDir()`; `matches, _ := filepath.Glob(filepath.Join(dir, "run", "status-hooks-*.json"))`; `os.Remove` tiap hasil (satu kali, bukan polling). Error diabaikan/log debug.

j. `func InstallHooks()`: 
   1. `removeStaleHooks()` (sisa daemon sebelumnya, dijalankan sekali saat dipanggil).
   2. `provider.RegisterLaunchContribution(contribute)`.
   Tidak mengembalikan error. Dilindungi `sync.Once` agar pemanggilan ganda tidak mendaftarkan dua kontribusi.

k. `func contribute(ctx *provider.LaunchContext)` (penutup/fungsi privat):
   - Defensif: `ctx == nil` -> return; `ctx.Env == nil` -> `make`.
   - `dataDir, err := paths.DataDir()`; `sock, serr := paths.SocketPath()`. Bila `err != nil`, `slog.Warn` dan return (jangan panic; sesi tetap jalan tanpa status hooks).
   - `token, err := newToken()`; bila gagal, warn dan return.
   - `ctx.Env["AITERM_SESSION_ID"] = token`; bila `serr == nil` -> `ctx.Env["AITERM_SOCK"] = sock` (bila gagal, log warn dan lewati AITERM_SOCK saja). Berlaku untuk semua provider.
   - `providerID := ctx.Session.ProviderID`; bila `ctx.Provider != nil`, gunakan `ctx.Provider.ID()`.
   - Bila `providerID == provider.IDClaude` dan `ctx.Provider != nil` dan `ctx.Provider.Capabilities().Hooks`: `path, err := writeHooksFile(dataDir, token, aitermctlPath())`; bila sukses: `ctx.Args = append(ctx.Args, "--settings", path)`; bila gagal: `slog.Warn` dan lanjut tanpa `--settings` (token tetap di env).
   - Panggil `RegisterLaunch(LaunchInfo{Token: token, WorkspaceID: ctx.Session.WorkspaceID, ProjectID: ctx.Session.ProjectID, Cwd: ctx.Session.Cwd, ProviderID: providerID, At: time.Now()})` SETELAH env/args terisi. Catatan keterbatasan (komentar): `Launcher.Command` bisa dipanggil tanpa sesi benar-benar terbentuk (mis. gagal start), sehingga launch tertunda dan berkasnya bisa yatim; dibersihkan oleh `removeStaleHooks` saat daemon berikutnya start.
   - Komentar kode: nilai `AITERM_SESSION_ID` adalah launch token (overview T1), bukan ID sesi asli, karena `session.Manager.Create` memanggil Launcher sebelum membuat ID.
   - Contribution TIDAK boleh memanggil provider lain, mengedit `~/.claude/`, atau menyentuh `.claude/` project.

### 5. Build dan pemeriksaan
- `gofmt -l internal/status` kosong (khusus file Anda), `go build ./...` lolos, `go vet ./internal/status/...` bersih bila tersedia.
- `grep -n "STUB(foundation)" internal/status/hooks.go internal/status/install.go` kosong (hapus penanda stub).
- `grep -rnE "time\.(NewTicker|Tick)\(" internal/status/hooks.go internal/status/install.go` kosong.
- Pemeriksaan manual cepat bila memungkinkan (tanpa membuat file test di repo): `go run` skrip sementara di scratchpad yang memanggil `ParseHookEvent` dengan `nil`, `[]byte("")`, `[]byte("not json")`, `[]byte("[1]")`, `[]byte("{\"message\":\"x\",\"zzz\":1}")` dan memastikan tidak panic serta payload tidak nil. Jangan commit skrip itu.

## Perubahan

- Diubah (stub Foundation diganti implementasi): `internal/status/hooks.go`
- Diubah (stub Foundation diganti implementasi): `internal/status/install.go`
- Tidak ada file lain dibuat atau diubah.

## Wiring

Yang diekspos untuk disambungkan oleh langkah wiring (JANGAN disambungkan sendiri; `wire_status.go` milik `status-engine`):
- `status.InstallHooks()` dipanggil tepat sekali dari `wire_status.go` (setelah `status.SetDefault(eng)` dan `status.RegisterIPC`), supaya `RegisterLaunch` di dalam contribution menemukan engine default.
- `status.CleanupHooks(token)` dipanggil oleh `Engine.Forget` untuk token terikat.
- `ParseHookEvent`/`ClassifyEvent` boleh dipakai `ipc.go` atau `cmd_status.go`; tidak ada pemanggil wajib.
- Efek yang bisa dicek saat wiring: sesi baru mendapat env `AITERM_SESSION_ID` (token) dan `AITERM_SOCK` di semua provider; sesi `claude` mendapat argumen `--settings <DataDir>/run/status-hooks-<token>.json` dan berkas tersebut berisi 4 event; sesi `shell`/`codex` tanpa `--settings`.

## Selesai bila

- [ ] `go build ./...` sukses (guard: build path fitur ini); `gofmt` dan `go vet ./internal/status/...` bersih.
- [ ] `hooks.go` memuat `HookPayload`, `ParseHookEvent` (tidak pernah panic, payload tidak pernah nil, toleran stdin kosong/tak valid/field tak dikenal), dan `ClassifyEvent` sesuai tabel.
- [ ] `install.go` memuat `InstallHooks` (sekali, `sync.Once`, menghapus `status-hooks-*.json` sisa lewat satu `filepath.Glob`, mendaftarkan contribution), `CleanupHooks`, dan helper privat; env `AITERM_SESSION_ID`/`AITERM_SOCK` untuk semua provider; berkas hooks claude 4 event, dir 0700, berkas 0600; command dengan path tanda kutip tunggal dan akhiran `|| true`; `--settings <path>` hanya untuk claude; `RegisterLaunch` dipanggil.
- [ ] Hasil verifikasi skema `hooks` dan `--settings` (atau pernyataan tak dapat diverifikasi beserta asumsi) tercatat di komentar kode `install.go`.
- [ ] Tidak ada tulisan ke `~/.claude/` atau `.claude/` project; tidak ada ticker/polling; tidak ada marker `STUB(foundation)` tersisa di kedua file; hanya dua path milik fitur ini yang berubah.
