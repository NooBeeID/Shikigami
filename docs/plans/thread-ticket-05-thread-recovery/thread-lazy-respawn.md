# Plan: Hidupkan ulang agent secara lazy

## Metadata

- Tiket: `thread-ticket-05-thread-recovery` (`docs/tickets/thread-ticket-05-thread-recovery.md`)
- Fitur: `thread-lazy-respawn`
- ownedPaths: `internal/thread/recovery.go`
- Depends on foundation: ya (`00-foundation.md` sudah selesai sebelum plan ini dikerjakan: field `Participant.Dead`/`PreviousSessionIDs`, `recovery_contract.go`, `presend.go` + `PreSendHook`, stub `recovery.go`)
- Kontrak bersama: `docs/plans/thread-ticket-05-thread-recovery/overview.md` bagian 2.1 sampai 2.4 (jangan diubah)

## Tujuan

Mengganti stub `internal/thread/recovery.go` dengan implementasi penuh `Recovery`: menandai semua peserta thread `active` sebagai mati saat daemon start (tanpa spawn/kill), lalu menghidupkan ulang master/peer secara lazy pada pesan berikutnya di cwd yang sama (master di `MasterDir`, peer di worktree lama), dengan konteks log thread sebagai prompt awal, penggantian `SessionID` yang tercatat, mutex per thread agar tidak men-spawn ganda, dan penanganan kegagalan yang jelas (pesan sistem, `Dead` tetap true, tanpa retry otomatis, tanpa membuat worktree baru).

## Path yang dimiliki

HANYA `internal/thread/recovery.go` yang boleh ditulis/diubah (boleh ditimpa total, tanda tangan publik bagian "Kontrak" dipertahankan).

Jangan menyentuh: `types.go`, `recovery_contract.go`, `presend.go`, `manager.go`, `router.go`, `master.go`, `spawn.go`, `worktree*.go`, `prompt.go`, `orphans.go`, `frontend/`, `cmd/shikigamid/wire_thread.go`, maupun berkas lain. Bila butuh perubahan di luar berkas ini, JANGAN edit; tulis di bagian "Catatan untuk wiring" pada laporan akhir.

## Kontrak

Disediakan (tanda tangan persis; overview 2.3):

```go
type Recovery struct { /* m *Manager, mu sync.Mutex, locks map[string]*sync.Mutex */ }
func NewRecovery(m *Manager) *Recovery
func (r *Recovery) MarkDeadOnStart() error
func (r *Recovery) EnsureAlive(threadID string, participantIds ...string) error
func (r *Recovery) PreSend(threadID string, targetSessionIDs []string) (map[string]string, error) // memenuhi PreSendHook
func BuildRecoveryContext(t Thread, projects []ProjectInfo, msgs []ThreadMessage, self Participant) string
var _ PreSendHook = (*Recovery)(nil).PreSend // pertahankan dari stub
```

Dipakai (dibaca dari kode yang ada; hanya baca, jangan diubah):

- `Manager`: `Get`, `List`-setara via `managerAllThreads()`, `UpdateThread(threadID, func(*Thread)) (Thread, error)`, `Messages(id, time.Time{})`, `PostSystem(threadID, text)`, `SendToSession(threadID, from, to, text)`, `Store()` (`ThreadDir`, `MasterDir`), `managerFireParticipant(t, p)`, `managerMaster(t)`, field `m.cfg.Sessions` (`Create`, `Get`), `m.cfg.Workspaces.Projects(workspaceID)`, `m.log`.
- `provider`: `resolveMasterProvider()` (master.go), `spawnResolveProvider()` (spawn.go).
- `BuildMasterPrompt(t, projects)` (prompt.go), `BuildPeerBrief(t, projectName, brief, wt WorktreeInfo)` (spawn.go).
- `LoadWorktreeRecords(threadDir)`, `UpsertWorktreeRecord(threadDir, rec)`, `worktreeExists(path)`, `WorktreeRecord`, `WorktreeInfo` (worktree.go, worktree_records.go).
- Konstanta foundation: `RecoveryDeadNotice`, `RecoveryRespawnPrefix`, `RecoveryFailedPrefix`, `RecoveryContextMaxMessages`, `RecoveryContextMaxRunes`, `RecoveryPreviousIDsMax`.
- `session.StatusRunning`, `session.SessionSpec{WorkspaceID, ProjectID, Cwd, ProviderID}`, `mention.ErrNotFound`.
- Teks pesan sistem: HANYA lewat konstanta foundation (penanda frontend; huruf kecil, tanpa titik akhir).

Perilaku wajib: overview bagian 2.4 (hidup/mati, master hanya bila `ids` kosong/memuat master, peer hanya bila terdaftar, cek ulang di bawah mutex, `ErrClosed` untuk thread closed, prompt sinkron dengan retry 5x300 ms, kegagalan tanpa retry otomatis, resume provider belum didukung).

## Langkah

Semua dalam satu berkas `internal/thread/recovery.go` (paket `thread`). Komentar dan pesan galat berbahasa Indonesia; helper tak diekspor berawalan `recovery`. Baca dulu `manager.go`, `master.go`, `spawn.go`, `worktree_records.go`, `types.go` versi terkini; bila nama berbeda dari plan ini, ikuti kode dan catat.

1. **Kerangka**. Impor: `context` (bila perlu), `errors`, `fmt`, `os`, `strings`, `sync`, `time`, `github.com/NooBeeID/shikigami/internal/mention`, `.../internal/session`. Tulis:
   ```go
   type Recovery struct {
       m     *Manager
       mu    sync.Mutex
       locks map[string]*sync.Mutex
   }
   func NewRecovery(m *Manager) *Recovery { return &Recovery{m: m, locks: map[string]*sync.Mutex{}} }
   func (r *Recovery) recoveryLock(threadID string) *sync.Mutex // buat-bila-belum-ada, pola sama dengan Spawner.spawnLock
   ```
   Komentar paket/tipe: catat bahwa resume sesi provider BELUM didukung (`provider.Capabilities` hanya `Hooks/MCP/IdleEvent`, id sesi provider tidak disimpan), sehingga jalur yang dipakai adalah spawn baru + konteks log.

2. **Helper status hidup**.
   ```go
   func (r *Recovery) recoveryIsDead(p Participant) bool
   ```
   true bila `p.Dead`, atau `r.m.cfg.Sessions.Get(p.SessionID)` galat/nil, atau `s.Status != session.StatusRunning`. Tanpa efek samping.

3. **`MarkDeadOnStart() error`**. Iterasi `r.m.managerAllThreads()`; lewati `State != StateActive`. Untuk tiap thread aktif: bila tidak ada peserta dengan `Dead == false`, lewati (tidak ada perubahan, tidak ada pesan). Selain itu `r.m.UpdateThread(t.ID, func(th *Thread){ for i := range th.Participants { th.Participants[i].Dead = true } })` (state diperiksa ulang di dalam closure; bila `th.State != StateActive` jangan ubah). Bila thread berubah, `r.m.PostSystem(t.ID, RecoveryDeadNotice)` TEPAT satu kali per thread. Jangan memanggil `Sessions.Kill`/`Create`. Galat per thread dikumpulkan dengan `errors.Join` (satu thread gagal tidak menghentikan yang lain), log `Warn`; kembalikan galat gabungan (nil bila bersih). Log `Info` ringkas: jumlah thread yang ditandai.

4. **`BuildRecoveryContext`** (fungsi paket, murni, tanpa I/O). Susun dengan `strings.Builder`, bahasa Indonesia, urutan:
   - (a) Kalimat: ini pemulihan setelah daemon dimulai ulang; sesi sebelumnya mati; lanjutkan pekerjaan dari log di bawah, bukan mulai dari awal. Sebut nama dan role `self`.
   - (b) `Goal thread:` + `t.Goal` (dan judul).
   - (c) `Peserta thread:` satu baris per `t.Participants`: `- <Name> (role: <Role>, project: <nama project atau "-">, <mati|hidup>)`. Nama project dari `projects` berdasar `ProjectID` (fallback `ProjectID`; master "-"). Status mati = `p.Dead`.
   - (d) `Log thread terakhir:` dari `msgs`: saring `Kind` `message` dan `system` saja (abaikan `activity`), urut waktu naik (urutkan stabil by `TS` bila perlu), ambil maks `RecoveryContextMaxMessages` yang TERAKHIR. Format baris: `[<pengirim>] -> [<penerima>]: <teks>` dengan teks dipotong 600 rune (tambahkan `…`), newline di teks diganti spasi. Nama pengirim/penerima: `user`/`system`/`all` apa adanya; selain itu cari peserta yang `SessionID == id` ATAU `PreviousSessionIDs` memuat id; fallback `id` mentah. Total rune bagian log dibatasi `RecoveryContextMaxRunes`: buang baris dari yang TERLAMA hingga muat; bila ada yang dibuang, awali dengan baris `(… N pesan lebih lama dipotong)`. Bila log kosong: `(belum ada pesan)`.
   - (e) Hanya bila `self.Role == RoleMaster`: peringatan bahwa peserta yang mati dihidupkan otomatis saat master `thread_send` ke mereka, dan jangan `thread_spawn` ulang project yang sudah punya peserta.
   Tulis helper kecil tak diekspor (`recoveryTruncate(s string, n int) string`, `recoveryParticipantLabel(t Thread, id string) string`) agar tidak mengulang kode.

5. **Resolusi target** (`recoveryResolveTargets`). Dari `t Thread` dan `ids []string` hasilkan daftar `Participant` mati yang harus dihidupkan, dalam urutan: master dulu, lalu peer. Aturan:
   - Petakan tiap id ke peserta saat ini: cocok `SessionID` atau ada di `PreviousSessionIDs` (id lama diterima). Id tak dikenal diabaikan.
   - Master dipilih bila `len(ids) == 0` ATAU salah satu id terpetakan ke master.
   - Peer dipilih hanya bila terpetakan dari `ids` (bila `ids` kosong, TIDAK ada peer).
   - Hanya yang `recoveryIsDead` yang dikembalikan.

6. **`EnsureAlive(threadID string, participantIds ...string) error`**. Bungkus `r.ensureAlive(threadID, participantIds)` dan buang peta: `_, err := r.ensureAlive(...); return err`.

7. **`ensureAlive(threadID string, ids []string) (map[string]string, error)`** (inti, peta lama->baru; kunci memakai `SessionID` yang dikirim pemanggil, yaitu id peserta saat itu; bila pemanggil memberi id lama dari `PreviousSessionIDs`, tambahkan juga entri itu ke peta):
   1. `l := r.recoveryLock(threadID); l.Lock(); defer l.Unlock()`.
   2. `t, err := r.m.Get(threadID)`; galat -> kembalikan. `t.State != StateActive` -> `ErrClosed`.
   3. Hitung target (langkah 5) dari `t` yang BARU dibaca setelah mengunci (cek ulang; bila kosong, kembalikan `nil, nil` tanpa efek).
   4. Muat `projects` via `r.m.cfg.Workspaces.Projects(t.WorkspaceID)` bila `Workspaces != nil` (abaikan ok=false; projects kosong).
   5. Untuk tiap target (master dulu), panggil `r.respawnOne(t, projects, target)`. Galat satu peserta dikumpulkan (`errors.Join`) dan TIDAK menghentikan peserta lain. Sukses -> `replaced[lama] = baru`. Setelah tiap sukses, `t` di-refresh dengan `r.m.Get` agar konteks berikutnya memuat peserta terbaru.
   6. Kembalikan `replaced`, galat gabungan.

8. **`respawnOne(t Thread, projects []ProjectInfo, old Participant) (newID string, err error)`**.
   - Fungsi `fail := func(reason error) (string, error)`: `r.m.log.Warn("thread: pemulihan agent gagal", "thread", t.ID, "participant", old.Name, "err", reason)`; `r.m.PostSystem(t.ID, RecoveryFailedPrefix+old.Name+": "+reason.Error())`; kembalikan `"", fmt.Errorf("hidupkan ulang %s: %w", old.Name, reason)`. Tidak mengubah `Dead` (tetap true; bila peserta belum `Dead` karena mati sendiri, biarkan, cek sesi nyata sudah menangani).
   - Master (`old.Role == RoleMaster`): `prov, err := resolveMasterProvider()`; `dir := r.m.Store().MasterDir(t.ID)`; `os.MkdirAll(dir, 0o700)`; `Cwd = dir`, `ProjectID = ""`. Prompt = `BuildMasterPrompt(t, projects) + "\n\n" + BuildRecoveryContext(t, projects, msgs, old)`.
   - Peer: `prov, err := spawnResolveProvider()`; `recs, err := LoadWorktreeRecords(r.m.Store().ThreadDir(t.ID))`; cari rekaman dengan `ProjectID == old.ProjectID`. Bila tidak ada, atau `!worktreeExists(rec.Info.Path)`: `fail(errors.New("worktree tidak ditemukan (<path atau 'tidak ada rekaman'>); agent tidak dihidupkan, master perlu memutuskan"))`. JANGAN memanggil `CreateWorktree`/`RemoveWorktree`, JANGAN fallback ke `Info.ProjectPath`. `Cwd = rec.Info.Path`, `ProjectID = old.ProjectID`. Nama project: `rec.ProjectName` (fallback dari `projects`). Prompt = `BuildPeerBrief(t, rec.ProjectName, brief, rec.Info)` dengan `brief` = kalimat ringkas "Kamu dihidupkan ulang setelah daemon dimulai ulang; lanjutkan tugasmu di worktree ini; berikut konteks" + `"\n\n"` + `BuildRecoveryContext(t, projects, msgs, old)`.
   - `msgs, _ := r.m.Messages(t.ID, time.Time{})` (galat baca log -> lanjut dengan `nil`, catat `Warn`).
   - `s, err := r.m.cfg.Sessions.Create(session.SessionSpec{WorkspaceID: t.WorkspaceID, ProjectID: ..., Cwd: ..., ProviderID: prov.ID()})`; galat -> `fail(fmt.Errorf("buat sesi: %w", err))`. MCP `shikigami-thread` terpasang otomatis oleh `LaunchContribution` (thread diturunkan dari cwd, token baru per sesi); JANGAN menyentuh jalur itu.
   - Setelah `Create` sukses, perbarui peserta lewat `r.m.UpdateThread(t.ID, func(th *Thread){...})`: cari entri dengan `SessionID == old.SessionID`; set `SessionID = s.ID`, `Dead = false`, `PreviousSessionIDs = append(lama, old.SessionID)` dipotong ke `RecoveryPreviousIDsMax` terbaru (buang yang terlama). Nama, role, ProjectID TIDAK diubah. Bila thread berubah menjadi closed di tengah jalan atau entri tidak ditemukan, `Sessions.Kill(s.ID)` (sesi milik sendiri) dan kembalikan galat. Galat `UpdateThread` -> `Kill(s.ID)` + `fail`.
   - Peer: `UpsertWorktreeRecord(threadDir, rec)` dengan `rec.SessionID = s.ID` (galat hanya `Warn`, bukan fatal).
   - Ambil peserta terbaru dari thread hasil `UpdateThread`, panggil `r.m.managerFireParticipant(updated, newPart)` (hook peserta). `r.m.PostSystem(t.ID, RecoveryRespawnPrefix+old.Name)`.
   - Kirim prompt konteks SINKRON sebelum kembali: loop maks 5 kali, `r.m.SendToSession(t.ID, "", s.ID, prompt)`; ulang (tidur 300 ms) hanya bila `errors.Is(err, mention.ErrNotFound)`. Bila akhirnya gagal: `Warn` + `PostSystem(t.ID, "peringatan: gagal mengirim konteks pemulihan ke "+old.Name+": "+err.Error())`; sesi sudah hidup sehingga tetap kembalikan `s.ID, nil`. Sleep ini hanya jeda retry terbatas, bukan polling/timer.
   - Kembalikan `s.ID, nil`.

9. **`PreSend(threadID string, targetSessionIDs []string) (map[string]string, error)`**. Panggil `r.ensureAlive(threadID, targetSessionIDs)` dan kembalikan peta serta galatnya. Catatan: `Manager.Send` memberi `[master.SessionID]` sehingga master dihidupkan; `Router.Route` memberi id target sehingga hanya target itu (dan master hanya bila termasuk target). `ErrClosed` dikembalikan apa adanya (pemanggil hanya mencatat; pengecekan closed di `Send/Route` tetap menolak). Jangan menulis pesan sistem ganda: kegagalan sudah ditulis di `respawnOne`; `ErrClosed` tidak menulis pesan.

10. **Kebersihan**. `gofmt`; tanpa dependency baru, tanpa goroutine/timer/polling baru, tanpa unit test; panic aman karena pemanggil (`runPreSend`, `FireStart`) sudah memulihkannya. Pertahankan baris `var _ PreSendHook = (*Recovery)(nil).PreSend`.

11. **Guard**: jalankan `go build ./...` dari root repo; perbaiki sampai lulus. Jangan jalankan build frontend perlu (tidak ada perubahan frontend), kecuali sekadar memastikan tidak rusak.

## Perubahan

- Diubah (ditimpa total): `internal/thread/recovery.go`
- Tidak ada berkas lain dibuat atau diubah.

## Wiring

Fitur ini hanya mengekspos; JANGAN menyambungkan sendiri. Untuk langkah wiring (overview bagian 5, butir 1) di `cmd/shikigamid/wire_thread.go`, sebelum `m.FireStart()`:

- `rec := thread.NewRecovery(m)`
- `m.SetPreSend(rec.PreSend)` (jalur `thread.send` dan `thread_send` router, sudah dicegat foundation)
- `m.OnStart(func(*thread.Manager){ if err := rec.MarkDeadOnStart(); err != nil { /* log Warn */ } })`, didaftarkan lebih dulu daripada hook `CleanOrphans`.

`EnsureAlive` juga diekspos untuk pemanggilan langsung bila diperlukan, tetapi jalur normal adalah lewat `PreSend`. Bila kode saat implementasi berbeda dari asumsi plan ini (nama helper, tanda tangan), tulis penyesuaian di "Catatan untuk wiring" pada laporan akhir.

## Selesai bila

- `go build ./...` sukses (guard; tanpa unit test).
- `recovery.go` memuat persis tanda tangan publik bagian "Kontrak" dan `var _ PreSendHook = (*Recovery)(nil).PreSend`.
- `MarkDeadOnStart` tidak memanggil `Sessions.Create/Kill`, melewati thread closed, dan menulis satu `RecoveryDeadNotice` per thread yang berubah.
- `EnsureAlive`/`PreSend`: mutex per thread + cek ulang; master hanya bila `ids` kosong/memuat master; peer hanya bila terdaftar; thread closed -> `ErrClosed`; tidak ada pembuatan worktree baru; worktree hilang -> pesan `RecoveryFailedPrefix`, peserta tetap `Dead`.
- Setelah respawn: `SessionID` baru, `Dead=false`, `PreviousSessionIDs` terisi (maks 20), worktree record diperbarui (peer), hook peserta dipicu, `RecoveryRespawnPrefix+nama` diposting, prompt konteks terkirim sinkron dengan retry sebelum kembali.
- Tidak ada berkas di luar `internal/thread/recovery.go` yang berubah (`git status` hanya menunjukkan berkas itu).
