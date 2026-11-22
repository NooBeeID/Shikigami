# Plan: propose_close, inspeksi, dan cleanup penuh

## Metadata

- Tiket: `thread-ticket-04-thread-close` (`docs/tickets/thread-ticket-04-thread-close.md`)
- Fitur: `thread-close-daemon`
- ownedPaths: `internal/thread/close.go`, `internal/thread/close_ipc.go`
- Depends on foundation: ya (`00-foundation.md`: `Thread.CloseSummary`, konstanta `MethodClosePreview`/`MethodClose` di `types.go`, stub `close.go` berisi `RegisterClose` no-op)
- Rujukan kontrak bersama: `docs/plans/thread-ticket-04-thread-close/overview.md` (bagian 2.2 keputusan, 2.3 tipe/signature). Jangan diubah.

## Tujuan

Sisi daemon Go untuk menutup thread: `ProposeClose` (master mengusulkan, tidak menutup), `ClosePreview` (inspeksi read-only per worktree), dan `Close` (cleanup penuh: kill sesi, hapus worktree dan branch, hapus direktori master, state `closed`). Ditambah pendaftaran IPC `thread.closePreview`/`thread.close`, tool MCP `thread_propose_close`, dan pembungkus `ToolsSection` lewat `RegisterClose(m)`. Tanpa perubahan di luar ownedPaths.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `internal/thread/close.go` (stub Foundation ditimpa penuh; tidak boleh lagi mendefinisikan `RegisterClose`)
- `internal/thread/close_ipc.go` (baru)

Dilarang menyentuh: `types.go`, `manager.go`, `ipc.go`, `mcp_tools.go`, `prompt.go`, `worktree.go`, `worktree_records.go`, `agents_install.go`, `cmd/shikigamid/wire_thread.go`, `frontend/`, `internal/session`, berkas tiket/fitur lain. Butuh perubahan di luar itu: jangan dikerjakan, catat di "Catatan untuk wiring" di bawah.

## Kontrak

Signature dan bentuk JSON mengikuti overview 2.3 persis; jangan diubah. Ringkasan yang harus ada:

**Disediakan (`close.go`)**: tipe `ClosePreviewParticipant`, `CloseUncommitted`, `CloseCommit`, `CloseWorktreeEntry`, `ClosePreview`, `CloseParams`; method `(m *Manager) ProposeClose(threadID, callerSessionID, summary string) (Thread, error)`, `(m *Manager) ClosePreview(threadID string) (ClosePreview, error)`, `(m *Manager) Close(threadID string) (Thread, error)`. Semua slice di respons selalu non-nil.

**Disediakan (`close_ipc.go`)**: `func RegisterClose(m *Manager)`; method IPC `thread.closePreview{id}` -> `ClosePreview`, `thread.close{id}` -> `Thread`; tool MCP `thread_propose_close(summary)` (MasterOnly).

**Dipakai dari kode yang sudah ada (sudah diverifikasi ada)**:

- `Manager`: `Get(id)`, `UpdateThread(id, func(*Thread)) (Thread, error)` (sudah publish `thread.updated`), `PostSystem(id, text)`, `FireClose(Thread)`, `RegisterMethod(name, func(json.RawMessage) (any, error))`, `Store()` (punya `ThreadDir(id)` dan `MasterDir(id)`), `m.cfg.Sessions.Kill(id) error` (antarmuka `SessionAPI`), `m.log`.
- `worktree.go`: `WorktreeInfo{Path, Branch, BaseBranch, BaseCommit, ProjectPath}`, `WorktreeStatus{Dirty []string, Unmerged []string, BaseBranch}`, `InspectWorktree(info)`, `RemoveWorktree(info)` (idempoten).
- `worktree_records.go`: `WorktreeRecord{ProjectID, ProjectName, ProjectSlug, SessionID, Info, CreatedAt}`, `LoadWorktreeRecords(threadDir)`, `DeleteWorktreeRecord(threadDir, projectID)`.
- `mcp_tools.go`: `RegisterTool(ToolDef)` (panic pada duplikat), `ToolDef{Name, Description, InputSchema map[string]any, MasterOnly, Handler ToolFunc}`, `ToolFunc func(ctx, Caller, json.RawMessage) (any, error)`, `Caller{ThreadID, SessionID string; Master bool}`. Hasil handler bertipe string dikirim sebagai teks.
- `prompt.go`: `var ToolsSection func() string` (ditetapkan `InstallAgents` ke `AgentToolsSection`).
- Galat/konstanta: `ErrClosed`, `ErrNotFound`, `ErrInvalid`, `StateActive`/`StateClosed`, `RoleMaster`, `Participant`, `ThreadMessage`; `session.ErrNotFound` dari `github.com/NooBeeID/shikigami/internal/session` (paket ini sudah diimpor `manager.go`, jadi tidak ada siklus impor).
- Verifikasi nama aktual (mis. `StateClosed`, `FromSystem`) di `types.go`/`manager.go` sebelum menulis; bila beda, ikuti kode.

## Langkah

### 1. Cek awal

1. Baca `types.go`, `manager.go` (`UpdateThread`, `Get`, `PostSystem`, `managerLock`), `worktree.go` (`InspectWorktree`, `RemoveWorktree`), `worktree_records.go`, `mcp_tools.go` (bentuk `ToolDef`), `prompt.go`. Pastikan Foundation sudah ada (`CloseSummary`, `MethodClosePreview`, `MethodClose`, stub `close.go`); bila belum, hentikan dan laporkan blocker (jangan menambahkannya sendiri ke `types.go`).
2. `go build ./...` lulus sebelum mulai.

### 2. `close.go`: tipe dan state

Paket `thread`. Komentar, log, galat dalam bahasa Indonesia. Helper tak-diekspor berawalan `close`.

1. Deklarasikan tipe respons persis seperti overview 2.3 (`ClosePreviewParticipant`, `CloseUncommitted`, `CloseCommit`, `CloseWorktreeEntry`, `ClosePreview`, `CloseParams`) dengan tag JSON camelCase di sana.
2. State per thread tanpa mengedit `Manager`: variabel tingkat paket
   ```go
   var closeState = struct {
       mu      sync.Mutex
       locks   map[string]*sync.Mutex // mutex single-flight per thread
       closing map[string]bool        // tahap penutupan (flag memori)
   }{locks: map[string]*sync.Mutex{}, closing: map[string]bool{}}
   ```
   Helper `closeLock(id string) *sync.Mutex` (buat bila belum ada, di bawah `closeState.mu`), `closeSetClosing(id string, v bool)`, `closeIsClosing(id string) bool`. Mutex ini TERPISAH dari mutex `UpdateThread` milik Manager; jangan memegang mutex close saat memanggil sesuatu yang butuh mutex lain secara terbalik (`UpdateThread` mengambil dan melepas mutexnya sendiri, aman).
3. Konstanta: `closeSummaryMax = 2000` (rune).

### 3. `close.go`: `ProposeClose`

`func (m *Manager) ProposeClose(threadID, callerSessionID, summary string) (Thread, error)`:

1. `t, err := m.Get(threadID)`; galat dikembalikan apa adanya (`ErrNotFound` dipetakan IPC).
2. `t.State == StateClosed` -> `ErrClosed`.
3. Pemanggil harus master: cari `Participant` dengan `Role == RoleMaster && SessionID == callerSessionID`; bila tidak ada -> `fmt.Errorf("hanya master yang dapat mengusulkan penutupan")` (galat biasa, bukan `ErrInvalid`).
4. `summary = strings.TrimSpace(summary)`; kosong -> `fmt.Errorf("%w: summary wajib diisi", ErrInvalid)`. Potong ke 2000 rune (`[]rune`).
5. Bila `closeIsClosing(threadID)` -> galat "thread sedang ditutup".
6. `t, err = m.UpdateThread(threadID, func(t *Thread){ t.ReadyToClose = true; t.CloseSummary = summary })`. Di dalam `fn` JANGAN mengubah `State`. (Race kecil: bila thread menjadi closed di antara Get dan UpdateThread, `UpdateThread` tetap menulis; setelah update, cek `t.State == StateClosed` dan kembalikan `ErrClosed` tanpa PostSystem. Boleh ditangani dengan memeriksa state di dalam `fn` dan mengabaikan perubahan bila closed, lalu mengembalikan `ErrClosed` setelahnya.)
7. `m.PostSystem(threadID, "master mengusulkan penutupan: "+summary)`; kembalikan `t, nil`. Idempoten: panggilan ulang menimpa summary. TIDAK menutup thread.

### 4. `close.go`: `ClosePreview`

`func (m *Manager) ClosePreview(threadID string) (ClosePreview, error)` (read-only; tidak mengubah thread, rekaman, maupun worktree):

1. Hasil awal `ClosePreview{ThreadID: threadID, Worktrees: []CloseWorktreeEntry{}}`.
2. `t, err := m.Get(threadID)`; galat dikembalikan. Thread `closed` -> kembalikan hasil kosong tanpa galat.
3. `recs, err := LoadWorktreeRecords(m.Store().ThreadDir(threadID))`. Galat baca -> kembalikan galat terbungkus ("membaca rekaman worktree: ..."); nil -> kosong.
4. Per rekaman, bangun `CloseWorktreeEntry`:
   - `Participant` lewat helper `closeParticipantFor(t Thread, rec WorktreeRecord) ClosePreviewParticipant`: cari di `t.Participants` berdasarkan `SessionID` (hanya bila `rec.SessionID != ""`); bila ketemu pakai `{SessionID, Name, ProjectID}` peserta itu; bila tidak, `{rec.SessionID, rec.ProjectName, rec.ProjectID}`.
   - `Path = rec.Info.Path`, `Branch = rec.Info.Branch`, `BaseBranch = rec.Info.BaseBranch`, `Uncommitted: []CloseUncommitted{}`, `UnmergedCommits: []CloseCommit{}`.
   - `st, err := InspectWorktree(rec.Info)`. Bila `err != nil`: isi `Error = err.Error()` dan lanjut ke rekaman berikutnya (satu worktree gagal TIDAK menggagalkan panggilan). Sukses: `Uncommitted = closeParseDirty(st.Dirty)`, `UnmergedCommits = closeParseUnmerged(st.Unmerged)`; bila `st.BaseBranch != ""` boleh menimpa `BaseBranch`.
   - Jalankan inspeksi berurutan (cukup); tiap `InspectWorktree` sudah punya timeout sendiri. Bungkus pemanggilan dengan `recover` kecil (helper `closeInspect(info) (WorktreeStatus, error)`) agar panic satu worktree menjadi `Error`.
5. Urutan entri = urutan rekaman.

Helper parsing:

- `closeParseDirty(lines []string) []CloseUncommitted`: hasil non-nil. Format porcelain v1: dua karakter status + spasi + path (`XY path`). Lewati baris kosong atau lebih pendek dari 4 karakter. `status = strings.TrimSpace(line[:2])` (mis. `M`, `A`, `D`, `??`, `R`; bila kosong setelah trim, pakai `"?"`); `path = line[3:]`. Untuk rename (`R  old -> new`) biarkan path utuh `old -> new`. Bila path dikutip (diawali `"`), pakai `strconv.Unquote` dan abaikan galatnya (pakai teks asli bila gagal).
- `closeParseUnmerged(lines []string) []CloseCommit`: hasil non-nil. Format `"<hash> <subject>"`: `strings.SplitN(strings.TrimSpace(line), " ", 2)`; sha = bagian pertama dipotong ke 7 karakter bila lebih panjang (cocok dengan makna `%h`), subject = bagian kedua (boleh kosong). Lewati baris kosong.

### 5. `close.go`: `Close`

`func (m *Manager) Close(threadID string) (Thread, error)`; idempoten, single-flight, tahan gagal sebagian. Ikuti urutan overview 2.2 butir 7:

1. `l := closeLock(threadID); l.Lock(); defer l.Unlock()` (panggilan bersamaan kedua menunggu, lalu melihat `closed` dan mengembalikannya).
2. `t, err := m.Get(threadID)`; galat dikembalikan. `t.State == StateClosed` -> `return t, nil`.
3. `closeSetClosing(threadID, true)`; `defer closeSetClosing(threadID, false)`.
4. Kumpulkan galat di `var errs []string`.
5. **Kill sesi** (`closeKillAll`): kumpulkan `SessionID` unik tak-kosong dari SEMUA `t.Participants` (master dan peer) ditambah `rec.SessionID` tak-kosong dari rekaman worktree (agar sesi spawn yang belum sempat masuk `Participants` ikut mati; baca rekaman dulu dengan `LoadWorktreeRecords`; galat baca dicatat ke `errs` dan lanjut). Per id: `err := m.cfg.Sessions.Kill(id)`; `err == nil` atau `errors.Is(err, session.ErrNotFound)` = mati (sesi sudah dihapus manager); selain itu catat `"sesi <id>: <err>"` ke `errs` dan masukkan id ke slice `alive`. Jangan berhenti pada galat satu sesi.
6. **Hapus worktree** (`closeRemoveWorktrees`): untuk tiap rekaman: `RemoveWorktree(rec.Info)`; bila sukses, `DeleteWorktreeRecord(threadDir, rec.ProjectID)` (galat hapus rekaman dicatat); bila `RemoveWorktree` gagal, catat `"worktree <ProjectName>: <err>"` dan BIARKAN rekaman (tiket 5 akan membersihkan). Tetap dijalankan walau ada sesi yang belum mati? Ya, tiap langkah tahan gagal dan tidak saling menghentikan; namun lihat langkah 8 untuk akibat pada state.
7. **Hapus direktori master** (`closeRemoveMasterDir`): `dir := m.Store().MasterDir(threadID)`, `root := m.Store().ThreadDir(threadID)`. Gunakan `filepath.Clean` pada keduanya; hapus dengan `os.RemoveAll(dir)` HANYA bila `dir != root` dan `strings.HasPrefix(dir, root+string(filepath.Separator))`; selain itu catat galat "direktori master di luar direktori thread, tidak dihapus". DILARANG menghapus `ThreadDir`, `thread.json`, `messages.jsonl`, atau `worktrees.json`. Galat `RemoveAll` dicatat ke `errs`.
8. **State**: bila `len(alive) == 0`: `closed, err := m.UpdateThread(threadID, func(t *Thread){ now := time.Now().UTC(); t.State = StateClosed; t.ClosedAt = &now; t.ReadyToClose = false })` (`CloseSummary` dipertahankan). Galat `UpdateThread` -> catat pesan system galat dan kembalikan galat terbungkus (state belum closed). Sukses: `m.FireClose(closed)`.
9. **Catat galat**: bila `len(errs) > 0`, satu `m.PostSystem(threadID, "penutupan: "+strings.Join(errs, "; "))` (ringkas, potong total ke ~1000 rune). Bila sukses tanpa galat sama sekali, `PostSystem(threadID, "thread ditutup")`; bila sukses dengan galat worktree/direktori, tetap kirim pesan "thread ditutup" setelah pesan galat. Catatan: `PostSystem` tidak menolak thread closed, jadi aman dipanggil setelah state closed.
10. **Sesi masih hidup** (`len(alive) > 0`): thread TETAP `active`; kirim pesan system galat (langkah 9, tanpa "thread ditutup"), lalu kembalikan `Thread{}, fmt.Errorf("sesi %s belum bisa dimatikan; coba lagi", strings.Join(alive, ", "))`. Flag closing dilepas oleh `defer`; panggilan ulang melanjutkan (idempoten karena `RemoveWorktree` idempoten dan rekaman yang sudah dihapus tidak dimuat lagi).
11. Sukses: `return closed, nil`.
12. Semua panic di langkah yang memanggil kode luar tidak perlu ditangkap kecuali di `closeInspect`; jangan menambah goroutine.

### 6. `close_ipc.go`: `RegisterClose`

Paket `thread`. Impor: `context`, `encoding/json`, `fmt`, `strings`, `sync`.

1. `var closeToolOnce sync.Once`.
2. `func RegisterClose(m *Manager)`:
   - Daftar IPC (dipanggil sekali per Manager; `RegisterMethod` panic pada duplikat, jadi `RegisterClose` memang cukup dipanggil sekali dari wiring; lindungi hanya `RegisterTool` dan pembungkus dengan `sync.Once`):
     ```go
     m.RegisterMethod(MethodClosePreview, func(raw json.RawMessage) (any, error) {
         var p CloseParams
         if err := ipcDecode(raw, &p); err != nil { return nil, err }
         if strings.TrimSpace(p.ID) == "" { return nil, fmt.Errorf("%w: id wajib diisi", ErrInvalid) }
         return m.ClosePreview(p.ID)
     })
     m.RegisterMethod(MethodClose, ... sama, memanggil m.Close(p.ID))
     ```
     Cek bentuk `ipcDecode(raw, v)` di `ipc.go` dan sesuaikan. Pemetaan galat otomatis lewat `ipcToError` (jangan petakan sendiri). Handler tidak boleh mengembalikan `ClosePreview` dengan slice nil (sudah dijamin `ClosePreview`).
   - `closeToolOnce.Do(func(){ RegisterTool(ToolDef{...}); prev := ToolsSection; ToolsSection = func() string { return prev() + "\n- thread_propose_close(summary): ..." } })`.
3. Tool `thread_propose_close`:
   - `Name: "thread_propose_close"`, `Description: "Usulkan penutupan thread karena goal tercapai. Hanya mengusulkan: user yang menutup lewat tombol Close. Beri ringkasan hasil."`, `MasterOnly: true`.
   - `InputSchema: map[string]any{"type":"object","properties":map[string]any{"summary":map[string]any{"type":"string","description":"Ringkasan hasil kerja thread"}},"required":[]string{"summary"}}` (ikuti gaya `InputSchema` tool lain di `mcp_tools.go`).
   - Handler: `if !c.Master { return nil, fmt.Errorf("hanya master yang dapat mengusulkan penutupan") }`; decode `args` ke `struct{ Summary string \`json:"summary"\` }`; `_, err := m.ProposeClose(c.ThreadID, c.SessionID, in.Summary)`; sukses mengembalikan string `"usul penutupan dicatat; menunggu user menekan Close"`. Thread closed -> `ErrClosed` ("thread sudah ditutup") diteruskan apa adanya.
   - Closure menangkap `m` dari panggilan pertama `RegisterClose` (di produksi hanya ada satu Manager).
4. Pembungkus `ToolsSection`: kalimat tambahan (bahasa Indonesia) menyebut `thread_propose_close(summary)`: hanya mengusulkan, jangan menutup sendiri atau berasumsi thread tertutup, panggil bila goal tercapai dengan ringkasan hasil, user yang menekan Close. Syarat: `RegisterClose` dipanggil SETELAH `InstallAgents` (tugas wiring, lihat bawah). Tangani `prev == nil` dengan `prev = func() string { return "" }`.
5. Tidak ada hook `OnMessage` untuk menolak pesan ke thread closed (overview 2.2 butir 2): `ErrClosed` sudah ditangani Manager/router. Jangan menambahkannya.

### 7. Guard

1. `gofmt -l internal/thread` kosong.
2. `go build ./...` dari akar repo lulus. Bila gagal akibat berkas di luar ownedPaths, laporkan saja.
3. `go vet ./internal/thread/` bersih (opsional, bila cepat).
4. `git status` hanya menunjukkan `internal/thread/close.go` dan `internal/thread/close_ipc.go`; tidak ada perubahan `go.mod`.
5. Periksa manual dari kode: tidak ada pemanggilan git di working tree utama project (semua lewat `InspectWorktree`/`RemoveWorktree`); `ClosePreview` tidak memanggil fungsi penulis; `RegisterClose` tidak didefinisikan di `close.go`.

## Perubahan

- Diubah (ditimpa penuh dari stub Foundation): `internal/thread/close.go`
- Dibuat: `internal/thread/close_ipc.go`

## Wiring

Yang diekspos untuk disambungkan langkah wiring (JANGAN disambungkan oleh fitur ini):

- `thread.RegisterClose(m *Manager)`: dipanggil satu baris di `cmd/shikigamid/wire_thread.go`, di dalam `wireThread`, SETELAH `thread.InstallAgents(...)` (dan setelah `InstallActivity` bila ada) dan SEBELUM `m.FireStart()`. Panggil tepat sekali (`RegisterMethod` panic pada duplikat).
- Efek terdaftar: IPC `thread.closePreview` dan `thread.close`, tool MCP `thread_propose_close` (master only), dan prompt master diperluas lewat `ToolsSection`.
- Tidak ada hook `OnMessage` atau perubahan `manager.go`/`ipc.go`/`mcp_tools.go`/`prompt.go`.

Catatan untuk wiring: (isi bila ada nama simbol di kode yang berbeda dari asumsi plan, atau titik ekstensi yang ternyata tidak ada).

## Selesai bila

- `internal/thread/close.go` memuat tipe 2.3, `ProposeClose`, `ClosePreview`, `Close`, dan helper `close*`; tidak mendefinisikan `RegisterClose`.
- `internal/thread/close_ipc.go` memuat `RegisterClose` yang mendaftarkan dua method IPC, tool `thread_propose_close` (MasterOnly, `sync.Once`), dan pembungkus `ToolsSection`.
- `ProposeClose` menolak non-master, thread closed, summary kosong, dan thread yang sedang ditutup; sukses mengisi `ReadyToClose` dan `CloseSummary` lewat `UpdateThread` dan mencatat `PostSystem`; tidak menutup thread.
- `ClosePreview` read-only, hasil selalu non-nil, satu worktree gagal menjadi entri `error`, thread closed -> `worktrees: []`.
- `Close` single-flight per thread, idempoten, urutan kill, RemoveWorktree + DeleteWorktreeRecord, hapus direktori master (aman, `ThreadDir`/`thread.json`/`messages.jsonl` tidak terhapus), state closed + `closedAt` + `FireClose` hanya bila semua sesi mati; galat dikumpulkan ke satu pesan system.
- Guard: `go build ./...` sukses dan `gofmt -l internal/thread` kosong; tanpa unit test; hanya dua path ownedPaths yang berubah.
