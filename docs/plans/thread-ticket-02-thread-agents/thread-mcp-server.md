# Plan: Server MCP shikigami-thread dan LaunchContribution

## Metadata

- Tiket: `thread-ticket-02-thread-agents` (`docs/tickets/thread-ticket-02-thread-agents.md`)
- Fitur: `thread-mcp-server`
- ownedPaths: `internal/thread/mcp.go`, `internal/thread/mcp_tools.go`, `internal/thread/launch.go`
- Depends on foundation: ya. Foundation (`00-foundation.md`) harus sudah selesai: `agents_types.go` (`Router`, `ErrPaused`, `ErrTargetUnknown`, `ErrTargetAmbiguous`, `ErrNotParticipant`, `ParticipantView`) dan stub `mcp_tools.go` (`Caller`, `ToolFunc`, `ToolDef`, `RegisterTool`, `registeredTools`, stub `ToolDeps`/`RegisterCommonTools`/`AgentToolsSection`) sudah ada dan `go build ./...` lulus. Berkas `mcp.go` dan `launch.go` belum ada sebelum fitur ini.
- Rujukan wajib: `docs/plans/thread-ticket-02-thread-agents/overview.md` (bagian 2.2 koreksi no. 1, 2, 5; bagian 2.3 signature; tabel Tool MCP). Dokumen ini tidak mengulang isinya.

## Tujuan

Menyediakan server MCP loopback `shikigami-thread` (protokol identik `internal/mention/mcp.go`), token per sesi, registri tool penuh, tiga tool umum (`thread_participants`, `thread_send`, `thread_read`), teks prompt master (`AgentToolsSection`), dan `LaunchContribution` yang memasang server itu ke sesi peserta thread (master dan peer) yang provider-nya mendukung MCP. Pemanggil dikenali dari token; thread TIDAK pernah dari argumen. Tanpa mengubah `internal/mention`, `internal/provider`, `internal/session`.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `internal/thread/mcp.go` (BARU)
- `internal/thread/mcp_tools.go` (ada sebagai stub Foundation; ditulis ulang penuh dengan signature tetap)
- `internal/thread/launch.go` (BARU)

Dilarang menyentuh: berkas Foundation lain (`agents_types.go`, `worktree_records.go`, `worktree.go`), berkas fitur lain (`spawn.go`, `router.go`, `antiloop.go`), berkas tiket 01 (`manager.go`, `master.go`, `prompt.go`, `ipc.go`, dst.), `agents_install.go`, `cmd/shikigamid/*`, `internal/mention`, `internal/provider`, `internal/session`, frontend. Butuh perubahan di luar daftar: catat di "Wiring", jangan dikerjakan.

## Kontrak

Dipakai (sudah ada di kode tiket 01, dibaca saat menulis plan ini; ikuti kode bila berbeda):

- `Manager`: `Get(id) (Thread, error)`, `Messages(id string, sinceTS time.Time) ([]ThreadMessage, error)`, `Tools() *ToolRegistry`, `Store()`. `ToolRegistry.Tools() []ToolSpec`; `ToolSpec{Name, Description string; MasterOnly bool; Handler func(ctx, ThreadCaller, map[string]any) (any, error)}`; `ThreadCaller{ThreadID, SessionID, Role string}`. Catatan: `ToolSpec` TIDAK punya `InputSchema`; adaptasi memakai skema generik `{"type":"object","additionalProperties":true}`.
- `SessionAPI` (`Get`, `List`, `Kill`, `Create`) dan `ErrClosed` dari `manager.go`; konstanta `StateActive`, `StateClosed`, `RoleMaster`, `RolePeer`, `KindMessage`, `KindSystem`, `KindActivity`, `ToUser`, `ToAll` dari `types.go`; `ThreadMessage`, `Participant`, `Thread`.
- Foundation (`agents_types.go`): `Router` (`Route(ctx, threadID, from, to, text)`), `ErrNotParticipant`, `ParticipantView`.
- Luar paket thread: `provider.RegisterLaunchContribution`, `provider.LaunchContext{Session, Provider, Env, Args, MCPServers}`, `Provider.Capabilities().MCP`, `Provider.ID()`; `session.SessionSpec{WorkspaceID, ProjectID, Cwd, ProviderID}`, `session.Session{ID, WorkspaceID, ProjectID, ProviderID, Cwd, Status, CreatedAt}`, `session.StatusRunning`; `status.IsIdle(sessionID) bool` (`internal/status`). Verifikasi nama field dengan `grep` di `internal/session/types.go` sebelum memakai.

Disediakan (signature FINAL, overview 2.3; jangan diubah):

```go
// mcp.go
type MCPConfig struct {
    Manager  *Manager
    Sessions SessionAPI
    DataDir  string
    Tools    func() []ToolDef
    Logger   *slog.Logger
}
type MCPServer struct{ /* tak diekspor */ }
func NewMCPServer(cfg MCPConfig) *MCPServer
func (s *MCPServer) Start(ctx context.Context) (url string, err error)
func (s *MCPServer) URL() string
func (s *MCPServer) ReleaseSession(sessionID string)

// mcp_tools.go
type Caller struct{ ThreadID, SessionID string; Master bool }
type ToolFunc func(ctx context.Context, c Caller, args json.RawMessage) (any, error)
type ToolDef struct{ Name, Description string; InputSchema map[string]any; MasterOnly bool; Handler ToolFunc }
func RegisterTool(d ToolDef)
func registeredTools() []ToolDef
type ToolDeps struct{ Manager *Manager; Router Router; Logger *slog.Logger }
func RegisterCommonTools(d ToolDeps)
func AgentToolsSection() string

// launch.go
func ThreadIDFromCwd(dataDir, cwd string) (threadID string, ok bool)
func InstallLaunchContribution(s *MCPServer, m *Manager, dataDir string, logger *slog.Logger)
```

Tool yang didaftarkan fitur ini lewat `RegisterCommonTools`: `thread_participants`, `thread_send`, `thread_read` (argumen dan hasil persis tabel Tool MCP di overview 2.3). `thread_spawn` didaftarkan fitur `thread-spawn-router`; `thread_propose_close` oleh tiket 04; keduanya lewat `RegisterTool` tanpa mengedit berkas ini.

Konstanta nama: server `shikigami-thread`; berkas URL `<DataDir>/thread-mcp.json` berisi `{"url": "..."}`.

## Langkah

Seluruh helper tak-diekspor berawalan berkas: `mcp*`, `launch*`. Komentar dan pesan galat bahasa Indonesia. Logger nullable (fallback `slog.Default()`).

### 1. `internal/thread/mcp_tools.go` (tulis ulang penuh)

1. Pertahankan persis `Caller`, `ToolFunc`, `ToolDef`, `RegisterTool`, `registeredTools` dari stub (mutex paket + slice/map; panic bila nama kosong, Handler nil, atau duplikat; `registeredTools` salinan terurut nama). Hapus tanda `STUB(foundation)` pada bagian yang kini lengkap. `RegisterTool` dilarang memegang kunci saat memanggil kode luar.
2. `ToolDeps{Manager, Router, Logger}`.
3. Helper `mcpToolText(v any) string`: `string` -> apa adanya; selain itu `json.MarshalIndent`-bebas (pakai `json.Marshal`); dipakai `mcp.go` untuk membentuk blok `text`. Taruh di `mcp_tools.go` agar tipe hasil satu tempat.
4. `RegisterCommonTools(d ToolDeps)`: guard `sync.Once` paket (panggilan kedua no-op); `d.Manager == nil` -> panic pesan jelas; `Router` boleh nil saat register tetapi handler `thread_send` mengembalikan galat "router belum terpasang" bila nil. Daftarkan tiga tool dengan `RegisterTool`:
   - `thread_participants` (semua peran; schema `{"type":"object","properties":{}}`): `t, err := mgr.Get(c.ThreadID)`; thread `closed` -> `ErrClosed`. Untuk tiap `Participant` bentuk `ParticipantView`: `Name`; `SessionID`; `Role`; `Project` = nama project dari `Workspaces`-nya? TIDAK tersedia di `ToolDeps`, jadi isi `Project` dari `ProjectID` lewat helper `mcpProjectName(mgr, t, projectID)` yang memanggil `mgr.cfg.Workspaces` HANYA bila field itu dapat diakses dari paket yang sama (satu paket `thread`, boleh baca `mgr.cfg.Workspaces.Projects(t.WorkspaceID)`); cocokkan `ID`, fallback `ProjectID`; kosong untuk master. `Status` = `"idle"` bila `status.IsIdle(sessionID)` else `"kerja"`; `Self` = `sessionID == c.SessionID`. Urutan: master dulu lalu peer sesuai urutan `Participants`. Kembalikan `[]ParticipantView` (selalu array, bukan null).
   - `thread_send` (semua peran; schema: object, properties `to` string, `text` string, required `["to","text"]`): decode args ke struct `{To, Text string}`; `to`/`text` kosong setelah trim -> galat jelas. Thread aktif dicek (`ErrClosed`). Panggil `d.Router.Route(ctx, c.ThreadID, c.SessionID, to, text)`; galat dikembalikan APA ADANYA (agar pesan `ErrPaused`/`ErrTargetUnknown`/ambigu sampai ke agent). Sukses: `msg.To == ToUser` -> `"dicatat (user)"`; selain itu `"terkirim ke <nama>"` dengan nama = hasil `mcpDisplayName(t, msg.To)` (nama peserta bila `msg.To` sessionId, `"semua peserta"` bila `ToAll`, selain itu nilai mentah). Thread di-ambil ulang via `mgr.Get` sesudah Route untuk nama.
   - `thread_read` (semua peran; schema: `last_n` integer minimum 1 maksimum 200 default 20): decode `{LastN *int}`; default 20; clamp ke 1..200 (di luar rentang di-clamp, bukan galat). `msgs, err := mgr.Messages(c.ThreadID, time.Time{})`; saring hanya `KindMessage` dan `KindSystem`; ambil `last_n` terakhir, urut naik; kembalikan `[]ThreadMessage` (selalu array). Thread tak ada -> galat dari `Get`.
   Semua handler: pemanggil bukan peserta thread (mis. peserta sudah dihapus) -> `ErrNotParticipant`. Pengecekan peserta dilakukan di `mcp.go` saat resolve Caller (langkah 2), handler cukup memakai `c`.
5. `AgentToolsSection() string`: teks Indonesia untuk prompt master, ringkas (sekitar 8-12 baris), WAJIB menyebut nama `thread_spawn`, `thread_send`, `thread_read`, `thread_participants`. Isi: server MCP `shikigami-thread`; `thread_spawn(project, brief)` men-spawn agent peer di project workspace (di git worktree dan branch milik thread, auto-approve; project non-git ditolak; spawn ulang project sama memakai agent yang ada); `thread_send(to, text)` ke peserta (nama, `master`, `user`, `all`) dan balasan peserta tiba di terminal sebagai `[pesan dari <nama>] [thread] ...`; `thread_read(last_n)` membaca log; `thread_participants()` melihat peserta dan status; pesan user datang dengan awalan `[pesan dari user]` lalu teruskan ke peserta yang sesuai lewat `thread_send`, dan laporkan hasil ke user dengan `thread_send(to="user", ...)`; komunikasi hanya lewat tool, bukan mengetik ke terminal agent lain; peringatan: jika terjadi pesan beruntun antar agent thread bisa dijeda. JANGAN menyebut `thread_propose_close` (milik tiket 04; teksnya ditambah tiket itu). Catatan kecil dalam kode: "tiket 04 menambah kalimat propose_close".
6. Adaptasi registri tiket 01 (dipakai `mcp.go`, taruh di berkas ini): `mcpAdaptManagerTools(m *Manager) []ToolDef`: untuk tiap `ToolSpec` di `m.Tools().Tools()` bentuk `ToolDef{Name, Description, InputSchema: {"type":"object","additionalProperties":true}, MasterOnly, Handler}` dengan Handler yang meng-unmarshal `json.RawMessage` ke `map[string]any` (kosong/`null` -> map kosong), membentuk `ThreadCaller{ThreadID, SessionID, Role}` (`Role` = `RoleMaster` bila `c.Master` else `RolePeer`), lalu memanggil `spec.Handler`. `m == nil` -> slice kosong.
7. `mcpMergeTools(m *Manager) []ToolDef` (default `MCPConfig.Tools`): `registeredTools()` ditambah hasil adaptasi; nama ganda -> registri global menang, adaptasi dilewati dengan `Warn` (overview 2.2 no. 5); urut nama.

### 2. `internal/thread/mcp.go` (BARU)

1. Salin protokol dari `internal/mention/mcp.go` TANPA mengimpor `internal/mention` (tipe di sana tak diekspor): tipe `mcpRPCRequest`, `mcpRPCResponse`, `mcpRPCError`, konstanta kode galat (`mcpCodeParseError` -32700, `mcpCodeInvalidRequest` -32600, `mcpCodeMethodNotFound` -32601, `mcpCodeInvalidParams` -32602), `mcpWriteRPC`, `mcpBearerToken`. Perilaku HTTP identik: hanya `POST` (selain itu 405 + header `Allow: POST`); tanpa/unknown token -> 401 + `WWW-Authenticate: Bearer`; body maks `1<<20` via `http.MaxBytesReader`; body diawali `[` -> galat "batch tidak didukung"; notifikasi tanpa `id` -> 202 tanpa body; `recover` panic -> 500 + log. Jangan menyalin `init()` hook `startMCP`.
2. Token manager tak diekspor `mcpTokens` (instance per `MCPServer`, BUKAN global, agar tidak bentrok dengan `tokens` milik mention): meniru `tokenManager` mention. `mcpPendingToken{token, threadID, workspaceID, projectID, providerID, cwd string; issuedAt time.Time}`; `mcpTokenManager{mu; pending []mcpPendingToken; bound map[string]mcpBinding (token -> {sessionID, threadID}); byID map[string]string (sessionID -> token)}`. Metode:
   - `issue(spec session.SessionSpec, providerID, threadID string) string`: token 32 byte hex dari `crypto/rand` (`mcpNewToken`, "" bila gagal); `providerID` kosong di spec -> pakai argumen; tambah ke `pending`; bila `len > 64` potong menjadi 64 terbaru.
   - `lookup(token string, sessions SessionAPI) (sessionID, threadID string, ok bool)`: token terikat -> verifikasi `sessions.Get(sid)` ada dan `Status == session.StatusRunning`, selain itu `releaseLocked` dan `ok=false`; token tertunda -> `bindPendingLocked(sessions)` lalu cek ulang; bila belum terikat `ok=false` (klien mencoba lagi; ini yang menghasilkan 401 sementara, sama seperti mention).
   - `bindPendingLocked`: kumpulkan `sessions.List()` berstatus running yang belum di `byID`, urut `CreatedAt` lalu `ID`, kelompokkan dengan kunci `workspaceID|projectID|providerID|cwd`, ikat tiap pending FIFO ke sesi pertama grup yang cocok; pending tak cocok tetap tinggal. Master punya `ProjectID` kosong (pastikan kunci memakai nilai spec apa adanya, yang sama dengan `Session.ProjectID`).
   - `release(sessionID)`, `boundCount()`.
3. `MCPServer` (field: `cfg MCPConfig`, `log`, `tokens *mcpTokenManager`, `mu sync.RWMutex`, `url string`, `srv *http.Server`).
   - `NewMCPServer(cfg)`: logger fallback; `cfg.Tools == nil` -> `func() []ToolDef { return mcpMergeTools(cfg.Manager) }`.
   - `Start(ctx)`: wajib `cfg.Manager != nil && cfg.Sessions != nil` (galat `errors.New` Indonesia bila tidak). Sudah berjalan -> kembalikan `url` yang ada (idempoten). `net.Listen("tcp", "127.0.0.1:0")`; `url = "http://" + addr + "/mcp"`; mux `/mcp`; `http.Server{ReadHeaderTimeout: 10s}`; simpan `url`; tulis `<DataDir>/thread-mcp.json` bila `DataDir != ""` (`MkdirAll` 0755, tulis `.tmp` lalu `os.Rename`; kegagalan hanya `Warn`, tidak menggagalkan Start); `go srv.Serve(ln)`; bila `ctx.Done()` non-nil, goroutine `Shutdown` dengan timeout 2 dtk; log Info "server MCP thread aktif".
   - `URL()`: baca di bawah `RLock`; "" sebelum Start.
   - `ReleaseSession(sessionID)`: `tokens.release`.
   - Method internal `issueToken(spec, providerID, threadID) string` dipakai `launch.go` (satu paket, tak diekspor, nama `mcpIssueToken`).
4. Handler `/mcp` (method `s.mcpHandle`): alur seperti mention: validasi method, bearer, `lookup` (gagal -> 401), baca body, tolak batch, parse, notifikasi 202, lalu switch:
   - `initialize`: `protocolVersion "2025-03-26"`, `capabilities {"tools":{}}`, `serverInfo {"name":"shikigami-thread","version":"1"}`.
   - `ping`: `{}`.
   - `tools/list`: resolve Caller secara lunak (`mcpResolveCaller`, langkah 5); bila galat (belum peserta) -> perlakukan `Master=false`. Kembalikan `{"tools":[...]}` dari `cfg.Tools()` disaring: tool `MasterOnly` dibuang bila `!caller.Master`. Tiap tool: `{"name","description","inputSchema"}`; `InputSchema == nil` -> `{"type":"object","properties":{}}`.
   - `tools/call`: parse `{name, arguments}` (kosong -> `codeInvalidParams`); resolve Caller (galat -> hasil tool error, bukan JSON-RPC error); cari tool di `cfg.Tools()` (tak ada -> hasil error "tool tidak dikenal: <nama>"); `MasterOnly && !caller.Master` -> hasil error "tool hanya untuk master"; thread `closed` -> hasil error "thread sudah ditutup" (cek via `Manager.Get`); panggil `Handler(r.Context(), caller, args)` dengan `recover` (panic -> hasil error). Hasil: sukses -> `{"content":[{"type":"text","text": mcpToolText(v)}]}`; galat -> `{"content":[{"type":"text","text": err.Error()}],"isError":true}`. `arguments` kosong -> `json.RawMessage("{}")`.
   - default: `codeMethodNotFound`.
5. `mcpResolveCaller(sessionID, threadID string) (Caller, error)`: `t, err := cfg.Manager.Get(threadID)`; galat dibungkus; `t.State == StateClosed` -> `ErrClosed`; cari `Participant` dengan `SessionID == sessionID`; tidak ada -> `errors.New("sesi belum terdaftar sebagai peserta thread; coba lagi sebentar")`; sukses -> `Caller{ThreadID: threadID, SessionID: sessionID, Master: p.Role == RoleMaster}`. ThreadID SELALU dari binding token, tidak pernah dari argumen. Untuk `tools/list` galat `ErrClosed` juga dianggap non-master (daftar tetap dikembalikan; panggilan yang menolak).
6. Aksesor internal untuk `launch.go`: `s.mcpIssueToken(spec session.SessionSpec, providerID, threadID string) string`.

### 3. `internal/thread/launch.go` (BARU)

1. `ThreadIDFromCwd(dataDir, cwd string) (string, bool)`: kedua argumen kosong -> false; `filepath.Clean` keduanya (dan `EvalSymlinks` best-effort pada kedua sisi bila sukses, fallback ke hasil Clean agar cwd yang belum ada tetap terbaca); `rel, err := filepath.Rel(filepath.Join(dataDir,"threads"), cwd)`; `err != nil`, `rel` diawali `..` atau `.` -> false; `id := first segmen` (pisah `filepath.Separator`); `id` harus persis 26 karakter alfanumerik huruf besar/angka ULID (`[0-9A-Z]{26}`); wajib ada minimal satu segmen lanjutan setelah id (cwd `<dataDir>/threads/<id>` polos tanpa subdir -> false, sesuai kontrak "di bawah `<id>/`"). Kembalikan `(id, true)`. Gunakan fungsi validasi `launchIsULID`.
2. `InstallLaunchContribution(s *MCPServer, m *Manager, dataDir string, logger *slog.Logger)`: `sync.Once` paket (`launchOnce`); `s == nil || m == nil` -> `Warn` dan return tanpa memakai Once. Isi `provider.RegisterLaunchContribution(func(ctx *provider.LaunchContext){...})` dengan logika berurutan dan setiap kegagalan = return diam-diam (sesi tetap meluncur tanpa MCP thread):
   1. `ctx == nil || ctx.Provider == nil || !ctx.Provider.Capabilities().MCP` -> return (shell dan codex otomatis lolos filter; overview 2.2 no. 2).
   2. `id, ok := ThreadIDFromCwd(dataDir, ctx.Session.Cwd)`; `!ok` -> return.
   3. `t, err := m.Get(id)`; `err != nil || t.State != StateActive` -> return (cwd thread tapi thread tak ada/closed tidak dipasang).
   4. `u := s.URL()`; kosong -> return.
   5. `tok := s.mcpIssueToken(ctx.Session, ctx.Provider.ID(), id)`; kosong -> `Warn` "gagal membuat token thread; MCP tidak dipasang" lalu return.
   6. `ctx.MCPServers == nil` -> `make(map[string]any)`; `ctx.MCPServers["shikigami-thread"] = map[string]any{"type":"http","url":u,"headers": map[string]any{"Authorization":"Bearer "+tok}}` (format identik mention).
3. Fungsi tidak mendaftarkan `OnSessionExit`; pelepasan token dirangkai wiring lewat `ReleaseSession`. Tidak ada state global selain `launchOnce`.

### 4. Verifikasi

1. `gofmt -l internal/thread` kosong; `go vet ./internal/thread/...`; `go build ./...` (guard).
2. `git status`: hanya tiga path milik fitur ini yang berubah (selain berkas fitur lain yang dikerjakan agent paralel; jangan disentuh).
3. Pastikan tidak ada impor `internal/mention` baru yang tidak perlu dan tidak ada perubahan `go.mod`.

## Perubahan

- `internal/thread/mcp.go` (baru): `MCPConfig`, `MCPServer`, `NewMCPServer`, `Start`, `URL`, `ReleaseSession`, helper JSON-RPC `mcp*`, token manager `mcpTokenManager`, resolve Caller, penulisan `thread-mcp.json`.
- `internal/thread/mcp_tools.go` (ditulis ulang dari stub): `RegisterTool` utuh, `ToolDeps`, `RegisterCommonTools`, `AgentToolsSection`, adaptasi `Manager.Tools()`, `mcpMergeTools`, `mcpToolText`.
- `internal/thread/launch.go` (baru): `ThreadIDFromCwd`, `InstallLaunchContribution`.

## Wiring

Diekspos untuk langkah wiring (`agents_install.go`; JANGAN disambungkan oleh fitur ini). Urutan di `InstallAgents` per overview bagian 5 butir 1:

1. `srv := NewMCPServer(MCPConfig{Manager, Sessions, DataDir, Logger})` lalu `srv.Start(ctx)`; galat dikembalikan.
2. `cfg.OnSessionExit(srv.ReleaseSession)`; di `wire_thread.go` pastikan `session.exit` fan-out ke `q.HandleExit` dan `srv.ReleaseSession`.
3. `InstallLaunchContribution(srv, mgr, dataDir, logger)` SEBELUM sesi master pertama diluncurkan (jalankan sebelum `mgr.FireStart()`), dan thread harus sudah tersimpan (state `active`) sebelum sesi master di-`Create`; periksa urutan di `Manager.Create`/`master.go` (kode tiket 01) dan catat bila berbeda.
4. `RegisterCommonTools(ToolDeps{Manager: mgr, Router: router, Logger})` sekali setelah `NewRouter`; `Router` harus dipasang sebelum agent memanggil `thread_send`.
5. `ToolsSection = AgentToolsSection` (variabel paket di `prompt.go`).
6. Catatan keterbatasan untuk wiring: server hanya terpasang ke provider dengan `Capabilities().MCP` true (Claude); master yang jatuh ke codex tidak mendapat tool thread (overview 2.2 no. 2). Token hanya terikat saat request MCP pertama (seperti mention); sebelum itu klien mendapat 401 sementara. Peserta baru muncul di `Participants` setelah `AddParticipant`; pemanggilan tool sebelum itu menjawab galat "sesi belum terdaftar sebagai peserta thread; coba lagi sebentar". Berkas `thread-mcp.json` ditulis ke `<DataDir>/` (bukan `.../threads`). Cek manual: `/mcp` di sesi master Claude menampilkan `shikigami-thread`; sesi interaktif biasa dan `shell` tidak.
7. Tiket 04 menambah `thread_propose_close` dengan `RegisterTool` dan menambah kalimatnya ke prompt; tidak perlu mengedit berkas fitur ini kecuali `AgentToolsSection` (catat bila perlu).

## Selesai bila

- Ketiga berkas ada, `gofmt -l internal/thread` kosong, `go vet ./internal/thread/...` bersih.
- `go build ./...` sukses (guard; tanpa unit test). Kegagalan build akibat berkas fitur paralel yang jelas bukan milik fitur ini dilaporkan, bukan diperbaiki.
- Seluruh signature pada bagian "Kontrak" persis sama dengan overview 2.3; tidak ada ekspor baru yang dipakai fitur lain.
- Perilaku sesuai spesifikasi: 401 untuk token tak dikenal; `tools/list` peer tanpa tool master-only; panggilan master-only oleh peer ditolak (`isError`); `thread_send` galat router diteruskan apa adanya; `thread_read` dibatasi 1..200 dan hanya thread pemanggil; launch contribution hanya memasang MCP untuk provider ber-MCP dengan cwd di `<DataDir>/threads/<id>/...` pada thread aktif; `thread-mcp.json` ditulis.
- Tidak ada perubahan di `internal/mention`, `internal/provider`, `internal/session`, berkas tiket 01, Foundation, atau fitur lain.
