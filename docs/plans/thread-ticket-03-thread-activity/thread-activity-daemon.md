# Plan: Turunkan pesan activity dari status/runs

## Metadata

- Tiket: `thread-ticket-03-thread-activity` (`docs/tickets/thread-ticket-03-thread-activity.md`)
- Fitur: `thread-activity-daemon`
- ownedPaths: `internal/thread/activity.go`
- Depends on foundation: ya (`00-foundation.md`: `ThreadMessage.Count/Items`, `Manager.UpsertMessage`, `Manager.ActiveThreads`, stub `activity.go`)
- Rujukan kontrak: `docs/plans/thread-ticket-03-thread-activity/overview.md` bagian 2 (jangan diubah). Tanpa unit test, tanpa dependency baru.

## Tujuan

Menimpa stub `internal/thread/activity.go` dengan implementasi penuh: listener hook `PostToolUse` dari `status.Engine` (event-driven, tanpa polling) yang menurunkan pesan `kind=activity` per agent peserta thread aktif, menggabungkan aksi beruntun menjadi satu pesan (`count`/`items`), dan menyimpannya lewat `Manager.UpsertMessage` tanpa memicu `OnMessage`. Signature publik (`ActivitySource`, `ActivityMergeGap`, `ActivityMaxItems`, `ActivitySummaryMax`, `InstallActivity`) sesuai overview 2.3 dan tidak boleh berubah.

## Path yang dimiliki

HANYA `internal/thread/activity.go`. Jangan sentuh: `activity_support.go`, `types.go`, `store.go`, `manager.go`, `ipc.go`, `internal/status`, `internal/runs`, `internal/mention`, `cmd/shikigamid/wire_thread.go`, `frontend/**`, atau path fitur lain. Butuh perubahan di luar itu: jangan dikerjakan, catat di laporan akhir sebagai "Catatan untuk wiring".

## Kontrak

Disediakan (signature final, overview 2.3):

```go
type ActivitySource interface {
    AddHookListener(fn func(sessionID, event string, payload map[string]any)) (cancel func())
}
const (
    ActivityMergeGap   = 30 * time.Second
    ActivityMaxItems   = 50
    ActivitySummaryMax = 160 // rune
)
func InstallActivity(m *Manager, src ActivitySource) (stop func())
```

Format pesan yang dihasilkan: `kind="activity"`, `from=<sessionId>`, `to="all"`, `text="<nama>: <Tool> <target>"` (aksi terakhir), `count` (total aksi digabung), `items` (maks 50 terbaru, urut lama -> baru, TANPA awalan nama), `ts` = waktu aksi terakhir, `id` stabil selama penggabungan.

Dipakai (nama dari kode tiket 01 / Foundation; bila beda di kode, ikuti kode dan catat):
- `Manager.ActiveThreads() []Thread`, `Manager.UpsertMessage(threadID string, msg ThreadMessage) (ThreadMessage, error)`, `Manager.Store().ThreadDir(threadID)`.
- Hook Manager: `OnStart(func(*Manager))`, `OnParticipantAdded(func(Thread, Participant))`, `OnClose(func(Thread))`, `OnMessage(func(Thread, ThreadMessage))`. Hook Manager hanya bisa didaftarkan, tidak bisa dilepas: pasang sekali per `InstallActivity` dan guard dengan flag `stopped` agar callback jadi no-op setelah `stop`.
- Tipe/konstanta: `Thread` (`ID`, `State`, `Participants`), `Participant{SessionID, ProjectID, Role, Name}`, `StateActive`/`StateClosed`, `KindActivity`, `ToAll`, `ErrClosed`, `NewID()`.
- Logger: `slog.Default()` (field `m.log` tidak diekspor).

Aturan keputusan yang wajib diikuti (overview 2.2): no. 2 (tanpa `OnMessage` untuk activity), no. 3 (`OnMessage` memutus ekor; aksi agent lain mengganti ekor), no. 5 (`ts` = aksi terakhir), no. 6 (peta memori sessionId -> {threadId, name}), no. 7 (bentuk items/text), no. 8 (batas).

## Langkah

Semua di `internal/thread/activity.go`. Komentar dan log bahasa Indonesia; identifier Inggris; helper tak-diekspor berawalan `activity`. Hanya stdlib dan impor yang sudah ada di modul (mis. `log/slog`, `path/filepath`, `sort`, `strings`, `sync`, `time`, `fmt`).

1. **Tulis ulang berkas**: pertahankan package comment, `ActivitySource`, tiga konstanta, dan signature `InstallActivity`; hapus komentar `STUB(foundation)`.

2. **Tipe internal**:
   ```go
   type activityOwner struct{ threadID, name string }
   type activityTail struct {
       sessionID string
       id        string   // id pesan activity ekor
       count     int
       items     []string // maks ActivityMaxItems terbaru
       lastTS    time.Time
   }
   type activityDaemon struct {
       m       *Manager
       log     *slog.Logger
       mu      sync.Mutex
       owners  map[string]activityOwner // sessionId -> pemilik
       tails   map[string]*activityTail // threadId -> ekor (satu per thread)
       queues  map[string]*activityQueue // threadId -> antrean tulis
       stopped bool
   }
   type activityQueue struct {
       jobs    []activityJob
       running bool
   }
   type activityJob struct{ threadID string; msg ThreadMessage } // snapshot pesan penuh
   ```
   Semua map dijaga `mu`. Salin slice `items` ke snapshot job (jangan berbagi backing array dengan ekor).

3. **`InstallActivity(m, src)`**:
   - `m == nil || src == nil` (cek juga interface berisi nil pointer bila mudah) -> `return func() {}`.
   - Buat `activityDaemon`; `seed()` dari `m.ActiveThreads()`; daftarkan `m.OnStart(func(*Manager){ d.seed() })`, `m.OnParticipantAdded(func(t Thread, p Participant){ d.addOwner(t.ID, p) })`, `m.OnClose(func(t Thread){ d.removeThread(t.ID) })`, `m.OnMessage(func(t Thread, _ ThreadMessage){ d.breakTail(t.ID) })`. Setiap callback: `defer recover` (log) dan return bila `stopped`.
   - `cancel := src.AddHookListener(d.onHook)`.
   - `stop`: `sync.Once`; set `stopped=true`, panggil `cancel()`, kosongkan `owners`/`tails`/`queues` (job yang sedang jalan boleh selesai).
   - Aman dipanggil sebelum/sesudah `FireStart`: seed langsung di install DAN di `OnStart`; seed bersifat idempoten (menimpa entri peserta dari `ActiveThreads`, tidak menghapus entri yang ditambah `OnParticipantAdded`).

4. **Peta pemilik**:
   - `activityName(p Participant) string`: `strings.TrimSpace(p.Name)`; kosong dan `p.Role == "master"` -> `"master"`; kosong lainnya -> 8 karakter pertama `SessionID` (aman bila lebih pendek). Cek konstanta role master di kode (mis. `RoleMaster`); bila tidak ada, bandingkan string `"master"`.
   - `seed()`: untuk tiap `Thread` aktif, tiap `Participant` dengan `SessionID != ""` -> `owners[sid] = {t.ID, name}`.
   - `addOwner(threadID, p)`: sama untuk satu peserta (abaikan thread closed bila `State` tersedia di argumen).
   - `removeThread(threadID)`: hapus semua entri `owners` ber-threadID itu, hapus `tails[threadID]` dan `queues[threadID]` (job tertunda dibuang; thread closed tidak menulis).

5. **Ringkasan** (fungsi murni, berawalan `activity`):
   - `activityTarget(tool string, input map[string]any) string`: kunci per tool sesuai overview 2.3 (Bash `command`; Read/Edit/Write `file_path`; NotebookEdit `file_path` lalu `notebook_path`; Grep/Glob `pattern`; WebFetch `url`; Task `description`; lainnya string pertama dari kunci terurut abjad). `input == nil` -> `""`.
   - `activityRelPath(threadDir, target string) string`: hanya bila `target` absolut (`filepath.IsAbs`) dan berada di bawah `threadDir`: ambil sisa setelah `threadDir + sep`, buang SATU segmen pertama (slug worktree) -> path relatif worktree; bila sisa hanya satu segmen atau kosong, kembalikan sisa apa adanya. Path absolut lain dibiarkan. Hanya diterapkan pada tool berkunci path (Read/Edit/Write/NotebookEdit) agar perintah Bash tidak diutak-atik.
   - `activityOneLine(s string) string`: ganti `\r\n`, `\r`, `\n`, `\t` dan karakter kontrol dengan spasi, `strings.Join(strings.Fields(s), " ")`.
   - `activityItem(tool, target string) string`: `strings.TrimSpace(tool + " " + target)` lalu satu baris, potong ke `ActivitySummaryMax` rune (hitung per `[]rune`).
   - `activityText(name, item string) string`: `name + ": " + item`; bila total rune > `ActivitySummaryMax`, potong BAGIAN item (bukan awalan nama) sehingga total <= 160; bila nama saja >= 160, potong keseluruhan ke 160. Selalu satu baris.

6. **Pemroses hook `d.onHook(sessionID, event string, payload map[string]any)`** (dipanggil di goroutine hook status; harus cepat dan tidak memblok):
   - `defer recover` + log `Error`.
   - Abaikan bila `event != "PostToolUse"`, `sessionID == ""`, atau `tool_name` (string) kosong.
   - `d.mu.Lock()`; bila `stopped` atau `sessionID` tidak ada di `owners` -> unlock, return (sesi interaktif user tidak tersentuh). Ambil `owner`.
   - `tool_input` diambil sebagai `map[string]any` (type assertion aman).
   - Hitung `threadDir := d.m.Store().ThreadDir(owner.threadID)` (boleh dihitung di luar lock), `target`, `item`, `text`, `now := time.Now().UTC()`.
   - **Penggabungan** (di bawah lock): `tail := d.tails[owner.threadID]`. Bila `tail != nil && tail.sessionID == sessionID && now.Sub(tail.lastTS) < ActivityMergeGap` -> gabung: `count++`, tambah `item` ke `items`, bila `len > ActivityMaxItems` buang yang tertua (`items = items[len-ActivityMaxItems:]`), `lastTS = now`. Selain itu (tail nil, agent lain, atau melewati jendela) -> buat ekor baru: `id = NewID()`, `count=1`, `items=[item]`, `lastTS=now`, dan GANTI `d.tails[threadID]` (aksi agent lain mengganti ekor).
   - Bangun snapshot `ThreadMessage{ID, ThreadID, From: sessionID, To: ToAll, Text: text, TS: lastTS, Kind: KindActivity, Count, Items: salinan}` lalu `d.enqueue(threadID, msg)` (masih di bawah lock agar urutan enqueue = urutan penggabungan), unlock.

7. **Antrean per thread (urutan terjaga, tidak memblok hook)**: `enqueue` menambahkan job ke `queues[threadID]`; bila `!running`, set `running=true` dan `go d.drain(threadID)`. `drain`: loop { lock; ambil job pertama (jika kosong -> `running=false`, unlock, return); bila `stopped` atau antrean thread sudah dihapus -> return; unlock; jalankan `d.write(job)` }. Optimasi wajib kecil: bila di antrean ada beberapa job ber-`ID` sama, cukup tulis yang terbaru (snapshot terakhir sudah memuat seluruh state) — hanya buang job lama yang id-nya sama dengan job berikutnya secara berurutan, jangan melewati job ber-id lain agar urutan antar-id terjaga. `drain` dibungkus `defer recover` yang juga mereset `running=false` (supaya panic tidak mengunci antrean) dan, bila ada sisa job, memulai `drain` baru.

8. **`d.write(job)`** (di luar lock, `defer recover`): `_, err := d.m.UpsertMessage(job.threadID, job.msg)`. Bila `errors.Is(err, ErrClosed)` -> panggil `d.removeThread(threadID)` (tanpa log berisik). Galat lain -> `d.log.Warn("thread: gagal menyimpan pesan activity", ...)` lalu lanjut; JANGAN menghentikan daemon atau memblok hook. Tidak memanggil `Post`/`AppendMessage` (itu memicu `OnMessage`).

9. **Pemutus ekor `breakTail(threadID)`** (dari `OnMessage`): lock, `delete(d.tails, threadID)`, unlock. Karena activity tidak memicu `OnMessage` (keputusan 2.2 no. 2), setiap panggilan berarti pesan non-activity baru.

10. **Pemeriksaan akhir kode**: tidak ada goroutine menetap (goroutine `drain` berakhir saat antrean kosong); tidak ada polling/timer; semua callback aman panic; `stop` idempoten; tidak ada impor tak terpakai; `gofmt`.

## Perubahan

- Diubah (ditimpa penuh): `internal/thread/activity.go`.
- Tidak ada berkas lain dibuat atau diubah.

## Wiring

Fitur ini hanya mengekspos `thread.InstallActivity(m *Manager, src ActivitySource) (stop func())` (`*status.Engine` memenuhi `ActivitySource`). JANGAN menyambungkannya sendiri. Langkah wiring nanti (overview bagian 5 no. 1): di `cmd/shikigamid/wire_thread.go`, setelah `thread.NewManager`/`RegisterIPC` (dan `InstallAgents` bila ada) dan sebelum `m.FireStart()`, panggil `thread.InstallActivity(m, eng)` dengan `eng` dari `d.Lookup("status.engine")` (log peringatan bila tidak tersedia). Catatan untuk commit wiring: "activity ditulis lewat jalur log+publish tanpa OnMessage".

## Selesai bila

- `go build ./...` (akar repo) dan `go build ./cmd/shikigamid` lulus; `gofmt -l internal/thread` kosong; `go vet ./internal/thread/...` bersih.
- `git status` hanya menunjukkan `internal/thread/activity.go` berubah.
- Signature dan konstanta sama persis dengan overview 2.3.
- Review kode terhadap acceptance criteria tiket: hanya sesi peserta thread aktif yang diproses; teks `<nama>: <Tool> <target>` satu baris maks 160 rune; 10 aksi beruntun dalam 30 dtk -> satu id dengan `count=10`; `OnMessage` atau aksi agent lain memutus gabungan; thread closed tidak menulis; panic tidak menjatuhkan daemon; hook status tidak diblok (kerja tulis hanya lewat goroutine antrean); pesan activity tidak memicu `OnMessage`.
- Build gagal akibat berkas fitur lain hanya dilaporkan, tidak diperbaiki.
