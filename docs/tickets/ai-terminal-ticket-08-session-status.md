---
id: ai-terminal-ticket-08-session-status
title: Session Status
spec: docs/superpowers/specs/2026-11-21-ai-terminal-design.md
status: ready
wave: 5
depends_on: [ai-terminal-ticket-06-ui-shell, ai-terminal-ticket-05-provider-adapters, ai-terminal-ticket-02-session-pty]
---
# Session Status

## Tujuan

Epic ini memberi tiap sesi status `running | waiting | done | idle` yang berbasis event hooks provider (mis. event Stop dan Notification di Claude Code), tanpa polling interval tetap, sesuai spec bagian "Status sesi dan biaya" dan fase 3. Untuk provider tanpa hooks (codex, shell) dipakai cadangan deteksi output-diam lewat timer yang hanya hidup saat ada output. Status ditampilkan sebagai lencana di daftar sesi dan memicu notifikasi macOS saat sesi menunggu atau selesai. Epic ini juga menyediakan `IsIdle` dan `OnIdle` yang nanti dipakai epic mention (tiket 09).

Tiket ini harus bisa jalan dan lolos `make build` sendiri setelah tiket 01, 02, 05, dan 06 selesai. Tanpa epic mention atau file viewer, status tetap berfungsi penuh: event `session.status` terbit, lencana tampil, notifikasi muncul.

## Batasan

In scope:
- Paket `internal/status/`: penerima event hooks, state machine status per sesi, deteksi output-diam, notifikasi macOS, method IPC `status.*`.
- Pemasangan konfigurasi hooks Claude per sesi lewat `provider.RegisterLaunchContribution` (env dan args saja, tanpa mengubah file provider dan tanpa mengubah `~/.claude/settings.json` milik user).
- API Go `IsIdle(sessionID) bool` dan `OnIdle(sessionID, fn)`.
- Subperintah CLI `aitermctl status ...`, termasuk subperintah `hook` yang dipanggil oleh hooks provider.
- Modul frontend `frontend/src/modules/status/`: lencana status pada daftar sesi lewat panel registry (area `sidebar` dan `statusbar`).

Out of scope:
- Antrean mention, server MCP, routing pesan (tiket 09 session-mention). Epic ini hanya menyediakan API idle.
- Edit `internal/session/`, `internal/provider/`, `internal/ipc/`, `frontend/src/App.svelte`, `frontend/src/modules/shell/`, `frontend/src/lib/*`, atau file epic lain.
- Info git, PR, biaya token (spec: tidak ada interval tetap; di luar tiket ini).
- Knowledge/Grimoire dan penataan project (fase 4 dan 5), Windows, syntax highlighting.
- Polling dalam bentuk apa pun (ticker interval tetap, goroutine yang tidur-bangun berkala). Hanya event dan timer sekali jalan yang di-reset saat ada output.
- Unit test (guard hanya `make build`).

## Kontrak

### Yang dipakai dari epic lain

Dari tiket 01 (app-base):
- `internal/ipc`: `srv.Register(method, func(ctx context.Context, params json.RawMessage) (any, error))`, `srv.Publish(topic, payload)`, `client.Call(method, params, &out)`, `client.Subscribe(topicPrefix, fn)`; kode error `-32002 ERR_NOT_FOUND` dan `-32003 ERR_INVALID`.
- `internal/paths`: `DataDir` (`~/Library/Application Support/aiterm`) untuk berkas runtime.
- `cmd/aitermd/modules.go`: `RegisterModule("<modul>", func(srv *ipc.Server, d *Deps){...})`.
- `cmd/aitermctl/registry.go`: `RegisterCommand(...)`.
- `frontend/src/lib/registry.ts`: `registerPanel({id, area, component, order})`; `frontend/src/lib/ipc.ts` untuk `call` dan langganan event (`daemon:event`).

Dari tiket 02 (session-pty):
- Tipe `Session{id, workspaceId, projectId, cwd, providerId, status, createdAt}`, `SessionSpec`, `session.Manager` dan cara berlangganan ke output PTY serta exit sesi. Agent implementasi harus membaca `internal/session/` untuk nama persisnya. Output sesi tersedia juga lewat event `session.output.<id>` dan `session.exit` di ipc server; bila `Manager` menyediakan callback/observer yang lebih langsung, pakai itu. Dilarang mengedit paket session.
- Method `session.list` untuk memuat daftar sesi awal.

Dari tiket 05 (provider-adapters):
- `provider.RegisterLaunchContribution(fn func(*provider.LaunchContext))` dengan `LaunchContext{Session, Provider, Env map[string]string, Args []string, MCPServers map}`.
- `provider.Get(id)` dan `Provider.Capabilities()` (`Hooks`, `MCP`, `IdleEvent`) untuk memutuskan apakah sebuah sesi memakai hooks atau cadangan output-diam.

Dari tiket 06 (ui-shell):
- Shell merender panel yang terdaftar di area `sidebar` dan `statusbar`, dan menyediakan store sesi. Modul status tidak mengedit shell; ia mendaftar panel sendiri dan menyimpan state status di store lokal modul.

### Yang disediakan epic ini

Tipe dan API Go (`internal/status`, module `aiterm`):

```go
type Status string
const (
    StatusRunning Status = "running"
    StatusWaiting Status = "waiting"
    StatusDone    Status = "done"
    StatusIdle    Status = "idle"
)

type StatusEvent struct {
    SessionID string `json:"sessionId"`
    Status    Status `json:"status"`
    Prev      Status `json:"prev"`
    Source    string `json:"source"` // "hook" | "output" | "exit"
    Reason    string `json:"reason,omitempty"` // mis. "Stop", "Notification"
    At        int64  `json:"at"` // unix milidetik
}

// API untuk epic mention (tiket 09)
func IsIdle(sessionID string) bool                 // true bila status idle atau done atau waiting-tanpa-kerja
func OnIdle(sessionID string, fn func()) (cancel func())
```

Definisi `IsIdle`: true bila status `idle`, `done`, atau `waiting` (agent sedang menunggu input user); false bila `running`. Sesi tak dikenal mengembalikan false. `OnIdle` memanggil `fn` tepat sekali pada transisi berikutnya ke status "idle menurut IsIdle"; bila sesi sudah idle saat dipanggil, `fn` dipanggil segera (di goroutine terpisah). `cancel` membatalkan pendaftaran. Fungsi aman dipanggil konkuren.

Pesan IPC (semua nama dalam namespace `status.*`):
- `status.get` params `{"sessionId":"..."}` hasil `{"sessionId":"...","status":"running","since":1732190000000}`. Sesi tak dikenal: `-32002`.
- `status.list` params `{}` hasil `[{"sessionId","status","since"}, ...]`.
- `status.hook` params `{"sessionId":"...","event":"Stop|Notification|UserPromptSubmit|SessionStart|SubagentStop","payload":{...}}` hasil `{"ok":true}`. Dipanggil oleh hooks provider lewat `aitermctl status hook`. Event tak dikenal diabaikan dengan `ok:true`; `sessionId` tak dikenal: `-32002`.
- Event server ke klien: topik `session.status`, payload `StatusEvent`. Satu event per transisi nyata (status baru berbeda dari status lama); tidak ada event berulang.

Pemetaan event hook ke status:
- `UserPromptSubmit`, `SessionStart` -> `running`
- `Notification` -> `waiting` (agent butuh izin atau input)
- `Stop` -> `done`
- `SubagentStop` -> tidak mengubah status
- Proses sesi keluar (`session.exit`) -> `done` dengan `source:"exit"`, lalu state sesi dibuang.

Cadangan output-diam (provider tanpa hooks, atau sesi Claude yang hooks-nya tidak pernah datang):
- Output PTY pertama setelah diam -> `running`; satu timer sekali jalan di-reset pada tiap potongan output (reset dengan `time.Timer.Reset`, tanpa goroutine ticker). Saat timer habis (default 3 detik tanpa output, dapat diubah lewat konstanta) -> `idle`. Tidak ada output = tidak ada timer aktif.
- Untuk sesi dengan `Capabilities.Hooks == true`, hooks adalah sumber utama; timer output-diam hanya dipakai sebagai cadangan dengan jeda lebih panjang (default 30 detik) dan tidak menimpa status `waiting`/`done` yang berasal dari hook.
- Echo keystroke user tidak boleh dianggap kerja agent bila bisa dihindari; bila `Manager` tidak membedakan, terima keterbatasan ini dan beri komentar kode.

Pemasangan hooks per sesi (kontribusi launch, hanya untuk provider `claude`):
- Env: `AITERM_SESSION_ID=<id>` dan `AITERM_SOCK=<DataDir>/aitermd.sock` ditambahkan ke `ctx.Env` untuk SEMUA provider (berguna juga bagi epic lain), bukan hanya claude.
- Untuk `claude`: tulis berkas `<DataDir>/run/status-hooks-<sessionId>.json` berisi konfigurasi hooks (event `UserPromptSubmit`, `Notification`, `Stop`, `SessionStart`, tiap hook bertipe command memanggil `aitermctl status hook --session <id> --event <Nama>` dengan payload hook dari stdin), lalu tambahkan `--settings <path>` ke `ctx.Args`. Path binary `aitermctl` dicari lewat `os.Executable()` direktori yang sama, fallback `PATH`. Tidak ada penulisan ke `~/.claude/` atau `.claude/settings.json` project.
- Berkas hooks dihapus saat sesi exit.
- Memanggil proses `aitermctl` hanya terjadi saat ada event hook (event-driven), bukan polling; ini sesuai anti-goal spec.

CLI (`cmd/aitermctl/cmd_status.go`):
- `aitermctl status list` mencetak tabel sesi dan statusnya.
- `aitermctl status get <sessionId>`.
- `aitermctl status hook --session <id> --event <Nama>` membaca payload JSON dari stdin (boleh kosong), memanggil `status.hook`, keluar dengan kode 0 selalu (jangan menggagalkan agent walau daemon mati; cukup diam atau cetak ke stderr).
- `aitermctl status watch` berlangganan topik `session.status` dan mencetak tiap event (debug).

Frontend (`frontend/src/modules/status/`):
- `index.ts` dimuat otomatis lewat `import.meta.glob`; memanggil `registerPanel` untuk lencana. Berlangganan event `session.status` lewat `frontend/src/lib/ipc.ts`, memuat status awal lewat `status.list`.
- Store Svelte lokal modul `statusStore` (peta `sessionId -> {status, since}`) dan komponen `StatusBadge.svelte` (prop `sessionId`, opsional `compact`).

## Fitur

Ketiga fitur dikerjakan berurutan menurut ketergantungan tipe, tetapi tiap fitur harus meninggalkan build hijau (`make build`) saat selesai. Urutan aman: `status-engine` menyediakan tipe dan API; `hook-receiver` dan `status-notify-ui` bergantung pada tipe `Status`/`StatusEvent` dari engine. Bila dikerjakan paralel, sepakati dulu tipe di bagian Kontrak, dan engine menerbitkan `types.go`-setara (tipe `Status`, `StatusEvent`) lebih dulu di `internal/status/engine.go`.

### status-engine: Mesin status dan idle fallback

Deskripsi:
State machine per sesi dengan status `running/waiting/done/idle`, event-driven. Sumber input: (a) panggilan `Engine.Apply(sessionID, event, payload)` dari hook receiver, (b) potongan output PTY, (c) exit sesi. Engine menyimpan peta `sessionID -> state{status, since, timer, idleWaiters}` terproteksi mutex. Setiap transisi nyata mem-publish `session.status` via `srv.Publish`, memanggil pendengar internal (notifier) dan membangunkan `OnIdle` waiters.

Rincian:
- `engine.go`: tipe `Status`, `StatusEvent`, `Engine` (`New(srv *ipc.Server, ...)`), `Engine.Set(sessionID, status, source, reason)`, `Engine.Get`, `Engine.List`, `Engine.Subscribe(fn func(StatusEvent)) (cancel func())` untuk pendengar internal (dipakai notifier), `Forget(sessionID)`. Paket menyediakan singleton default dan fungsi paket `IsIdle`/`OnIdle` yang mendelegasikan ke singleton (agar epic mention cukup `status.IsIdle(id)`).
- `idle.go`: `OnOutput(sessionID)` men-reset timer sekali jalan (`time.AfterFunc` atau `Timer.Reset`); saat habis memanggil `Set(..., StatusIdle, "output", "silent")`. Timer dihentikan saat sesi exit, saat status `done`, dan tidak dibuat sebelum ada output pertama. Pilih jeda berdasarkan `Capabilities.Hooks`. Implementasi `IsIdle` dan `OnIdle` di sini atau di `engine.go`.
- `ipc.go`: `RegisterIPC(srv, eng)` mendaftarkan `status.get`, `status.list`, `status.hook` (memetakan event ke status seperti di Kontrak lalu memanggil engine). Sambungan ke output sesi: berlangganan ke event `session.output.<id>` / `session.exit` (atau observer `Manager` bila tersedia) dan memanggil `OnOutput`/`Forget`. Satu langganan prefiks `session.output.` pada ipc server cukup; jangan membuat goroutine per sesi yang tidur-bangun.
- `cmd/aitermd/wire_status.go`: `init(){ RegisterModule("status", func(srv *ipc.Server, d *Deps){ ... }) }` membuat engine, memanggil `status.RegisterIPC`, serta (setelah fitur lain ada) memanggil `status.InstallHooks()` dan `status.StartNotifier(...)`. Pada tahap fitur ini, cukup engine + IPC; panggilan fitur lain ditambahkan bertahap sehingga build tetap hijau di tiap tahap.
- `cmd/aitermctl/cmd_status.go`: `init(){ RegisterCommand(...) }` untuk `status list`, `status get`, `status watch`, dan `status hook`.
- Tidak ada `time.Ticker`; tidak ada loop `for { sleep }`. Pastikan CPU idle ~0% saat tidak ada sesi aktif.

Acceptance criteria:
- [ ] `make build` lolos.
- [ ] `aitermctl status hook --session <id> --event UserPromptSubmit` mengubah status sesi menjadi `running`, `Notification` -> `waiting`, `Stop` -> `done`; masing-masing menerbitkan tepat satu event `session.status`.
- [ ] Event yang tidak mengubah status tidak menerbitkan event ulang.
- [ ] Sesi shell: mengetik dan menerima output membuat status `running`, lalu `idle` setelah jeda diam; tidak ada timer aktif saat tidak ada output (periksa kode: tidak ada ticker).
- [ ] `session.exit` mengubah status ke `done` dan membuang state serta timer sesi.
- [ ] `status.get`/`status.list` mengembalikan JSON sesuai Kontrak; sesi tak dikenal pada `status.get`/`status.hook` -> `-32002`.
- [ ] `status.IsIdle(id)` dan `status.OnIdle(id, fn)` ada dengan signature di Kontrak, aman konkuren, `OnIdle` mengembalikan `cancel` dan memanggil `fn` sekali.
- [ ] `aitermctl status watch` mencetak event saat transisi.
- [ ] Tidak ada polling interval tetap di seluruh `internal/status/`.

ownedPaths: `internal/status/engine.go`, `internal/status/idle.go`, `internal/status/ipc.go`, `cmd/aitermd/wire_status.go`, `cmd/aitermctl/cmd_status.go`.

Paralel: fitur ini fondasi tipe; `hook-receiver` dan `status-notify-ui` bisa dikerjakan paralel setelah tipe `Status`/`StatusEvent` terbit. Catatan: `wire_status.go` dan `cmd_status.go` hanya dimiliki fitur ini; fitur lain menambah fungsi di paketnya sendiri dan fitur ini yang memanggilnya dari `wire_status.go` (sepakati nama `InstallHooks()` dan `StartNotifier(srv, eng)` sejak awal; sebelum fungsi itu ada, jangan panggil).

### hook-receiver: Penerima event hooks

Deskripsi:
Sisi masuk event hooks: konversi payload hook Claude menjadi panggilan `status.hook`, dan pemasangan konfigurasi hooks per sesi lewat launch contribution. Jalur penerima memakai method IPC `status.hook` pada socket utama (tidak membuat socket baru; spec dan kontrak IPC mewajibkan satu socket `aitermd.sock`).

Rincian:
- `hooks.go`: struktur payload hook Claude (`HookPayload{SessionID?, HookEventName, Message, ...}` dengan field opsional, parser toleran terhadap field tak dikenal), fungsi `ParseHookEvent(name string, stdin []byte) (event string, payload map[string]any)`, serta `ClassifyEvent(event string) (Status, bool)` yang memegang tabel pemetaan di Kontrak (dipakai `ipc.go` milik engine).
- `install.go`: `InstallHooks()` memanggil `provider.RegisterLaunchContribution(...)`. Kontribusi: (1) selalu set `AITERM_SESSION_ID` dan `AITERM_SOCK` di `ctx.Env`; (2) bila `ctx.Provider.ID() == "claude"` dan `Capabilities().Hooks`, tulis `<DataDir>/run/status-hooks-<sessionId>.json` (buat direktori `run` bila perlu, izin 0700/0600) dan tambah `--settings <path>` ke `ctx.Args`; (3) fungsi `CleanupHooks(sessionID)` menghapus berkas (dipanggil engine saat sesi exit). Format JSON hooks mengikuti skema `hooks` Claude Code: peta nama event ke daftar `{matcher?, hooks:[{type:"command", command:"..."}]}`. Agent implementasi harus memverifikasi skema terhadap dokumentasi Claude Code yang berlaku dan bila `--settings` menerima path atau string JSON; catat hasil verifikasi di komentar kode.
- Command hook memakai quoting aman untuk path yang mengandung spasi (path `Application Support`).
- Tidak mengubah file provider, `~/.claude/`, atau `.claude/` milik project.

Acceptance criteria:
- [ ] `make build` lolos.
- [ ] Sesi `claude` baru memiliki env `AITERM_SESSION_ID` dan `AITERM_SOCK` dan args `--settings <path>`; berkas hooks ada dan berisi 4 event (`UserPromptSubmit`, `Notification`, `Stop`, `SessionStart`).
- [ ] Sesi `shell` dan `codex` hanya mendapat env, tanpa `--settings`.
- [ ] Path dengan spasi pada command hook tidak merusak eksekusi (quoting benar).
- [ ] Berkas hooks dihapus saat sesi exit (via `CleanupHooks` dipanggil engine).
- [ ] Payload hook tak dikenal atau kosong tidak menyebabkan panic.
- [ ] Tidak ada berkas di `~/.claude/` atau project yang dibuat atau diubah.

ownedPaths: `internal/status/hooks.go`, `internal/status/install.go`.

Paralel: independen dari `status-notify-ui`. Bergantung pada tipe `Status` dari `status-engine` (cukup tipe; build hijau tanpa engine lengkap bila tipe sudah terbit) dan pada `provider.RegisterLaunchContribution` dari tiket 05. Pemanggilan `InstallHooks()` dari `wire_status.go` dilakukan oleh pemilik `wire_status.go` (fitur `status-engine`) setelah fungsi ini ada.

### status-notify-ui: Notifikasi dan lencana UI

Deskripsi:
Dua hal: notifikasi macOS yang dipicu event (bukan polling), dan lencana status pada daftar sesi di UI.

Rincian:
- `notify.go`: `StartNotifier(srv *ipc.Server, eng *Engine)` berlangganan `eng.Subscribe`. Pada transisi ke `waiting` atau `done` (bukan dari `exit` saat daemon berhenti, dan hanya bila sebelumnya bukan status yang sama) kirim notifikasi macOS. Mekanisme: `osascript -e 'display notification "<pesan>" with title "<judul>"'` dijalankan hanya saat event (spawn proses per notifikasi dapat diterima karena event-driven); pesan di-escape untuk AppleScript (kutip dan backslash). Judul memuat nama sesi/project bila tersedia (`session.list`/Manager), isi pesan: "Menunggu input Anda" atau "Selesai". Debounce: maksimal satu notifikasi per sesi per 2 detik; jangan memberi notifikasi saat jendela app sedang fokus bila daemon bisa mengetahuinya, bila tidak, abaikan kriteria itu dan beri komentar. Opsi lain, notifikasi lewat Wails runtime dari sisi UI, boleh dipakai di frontend sebagai pengganti hanya bila tidak mengedit file milik `app/`; defaultnya osascript dari daemon. Preferensi bisa dimatikan lewat variabel lingkungan `AITERM_NOTIFY=0`.
- `frontend/src/modules/status/index.ts`: registrasi panel lewat `registerPanel`: (1) panel `status-badges` area `sidebar` (order tinggi agar muncul setelah daftar sesi atau sebagai penghias yang dibaca shell; periksa kontrak `registry.ts` untuk cara panel menerima daftar sesi), (2) panel `status-summary` area `statusbar` berisi ringkasan jumlah sesi menurut status (mis. "2 berjalan, 1 menunggu"). Bila registry tidak mengizinkan penyisipan lencana ke dalam baris sesi milik shell, ekspor komponen `StatusBadge` sehingga shell dapat mengimpornya kemudian, dan tampilkan daftar ringkas sesi bersama lencana di panel sidebar milik modul ini; catat pilihan di komentar.
- `frontend/src/modules/status/store.ts`: store (`svelte/store`) yang memuat awal via `status.list` dan memperbarui dari event `session.status`; satu langganan global, dibersihkan saat modul dilepas.
- `frontend/src/modules/status/StatusBadge.svelte`: titik berwarna dengan label aksesibel: `running` (biru berdenyut halus), `waiting` (kuning), `done` (hijau), `idle` (abu). Animasi hanya CSS, tanpa timer JS.
- `frontend/src/modules/status/StatusBar.svelte` dan `SidebarBadges.svelte` sesuai panel di atas.
- Semua file hanya di `frontend/src/modules/status/`. Tidak mengedit `App.svelte`, shell, atau `lib/`.

Acceptance criteria:
- [ ] `make build` lolos (go build ./... dan build frontend/vite).
- [ ] Transisi ke `waiting` atau `done` memunculkan satu notifikasi macOS; transisi lain tidak.
- [ ] Notifikasi dengan teks mengandung kutip tidak merusak perintah osascript (escape benar).
- [ ] Tidak ada proses yang di-spawn kecuali saat ada event notifikasi.
- [ ] Modul frontend dimuat otomatis tanpa edit file lain; panel `sidebar` dan `statusbar` terdaftar.
- [ ] Lencana berubah real-time saat event `session.status` tiba, tanpa polling di frontend.
- [ ] Status awal dimuat dari `status.list` saat UI dibuka (UI yang tersambung kembali menampilkan status benar).
- [ ] `AITERM_NOTIFY=0` mematikan notifikasi.

ownedPaths: `internal/status/notify.go`, `frontend/src/modules/status/`.

Paralel: independen dari `hook-receiver`. Bergantung pada tipe `Status`/`StatusEvent` dan `Engine.Subscribe` dari `status-engine` untuk sisi Go; sisi frontend hanya bergantung pada kontrak event dan `status.list`, jadi bisa dikerjakan paralel dengan fitur Go. Pemanggilan `StartNotifier(...)` dari `wire_status.go` dilakukan oleh pemilik `wire_status.go`.

## Path yang dimiliki

Hanya path berikut yang boleh disentuh:
- `internal/status/` (termasuk `engine.go`, `idle.go`, `ipc.go`, `hooks.go`, `install.go`, `notify.go`)
- `cmd/aitermd/wire_status.go`
- `cmd/aitermctl/cmd_status.go`
- `frontend/src/modules/status/`

Dilarang menyentuh `go.mod`/`go.sum` kecuali dependensi baru benar-benar perlu (mestinya tidak; hanya stdlib dan paket proyek). Tidak ada perubahan di `internal/session/`, `internal/provider/`, `internal/ipc/`, `cmd/aitermd/modules.go`, `cmd/aitermctl/registry.go`, `frontend/src/App.svelte`, `frontend/src/lib/`, `frontend/src/modules/shell/`, atau file epic lain.

## Wiring

Setelah semua fitur selesai:
1. Modul `status` terdaftar otomatis lewat `init()` di `cmd/aitermd/wire_status.go` (`RegisterModule("status", ...)`); tidak ada edit `modules.go`. Di dalamnya: buat engine, `status.RegisterIPC`, `status.InstallHooks()` (launch contribution), `status.StartNotifier(...)`, dan sambung ke event output/exit sesi.
2. Launch contribution terpasang ke `provider.RegisterLaunchContribution` sehingga sesi baru otomatis mendapat env dan hooks. Sesi yang sudah berjalan sebelum daemon restart tidak punya hooks dan jatuh ke deteksi output-diam (terima keterbatasan ini).
3. Perintah CLI terdaftar lewat `init()` di `cmd/aitermctl/cmd_status.go` (`RegisterCommand`).
4. Frontend: `frontend/src/modules/status/index.ts` dimuat oleh `import.meta.glob` dari `registry`/`main.ts`; panel `sidebar` dan `statusbar` dirender oleh ui-shell.
5. Untuk epic hilir (tidak dikerjakan di sini): tiket 09 (session-mention) memanggil `status.IsIdle(id)` dan `status.OnIdle(id, fn)` untuk mengirim pesan antrean saat sesi idle.
6. Urutan inisialisasi modul: bila `RegisterModule` tidak menjamin `session` dan `provider` sudah siap, ambil dependensi secara malas (saat event pertama atau saat sesi dibuat), jangan mengasumsikan urutan.

## Guard

Cukup build berhasil: `make build` (menjalankan `go build ./...` dan build wails/vite). Tanpa unit test. Opsional `go vet ./internal/status/...` bila tersedia. Build harus tetap hijau setelah tiap fitur digabung, tidak hanya di akhir epic; tiket ini tidak boleh merusak build bila tiket 07 (file viewer), 09, atau 10 belum atau sudah selesai.
