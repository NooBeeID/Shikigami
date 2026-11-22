# Overview: thread-ticket-05-thread-recovery

Tiket: `docs/tickets/thread-ticket-05-thread-recovery.md`. Spec: `docs/superpowers/specs/2026-11-22-thread-design.md` (keputusan 3 dan 10). Dependensi: tiket 01 sampai 03 SUDAH ada di kode (`internal/thread/`, `frontend/src/modules/thread/`). Tiket 04 (`thread-close`) dan `Thread.CloseSummary`, `close.go`, `close/` belum tentu ada saat plan ini ditulis; epic ini TIDAK boleh memakai simbolnya. Satu-satunya hal dari tiket 04 yang diandalkan adalah makna `Thread.State == "closed"`.

Dokumen ini adalah sumber kebenaran BERSAMA untuk plan fitur. Bila beda dengan tiket, overview menang hanya pada butir berlabel "koreksi/keputusan" (bagian 2.9); selain itu tiket menang. Tanpa dependency baru, tanpa unit test; guard hanya build.

Semua nama di bawah dibaca dari kode yang ada (`types.go`, `manager.go`, `master.go`, `spawn.go`, `router.go`, `launch.go`, `worktree.go`, `worktree_records.go`, `prompt.go`, `store.go`, frontend `slots.ts`, `thread-store.ts`, `participants/`, `ThreadSlot.svelte`). Bila kode berbeda saat implementasi, ikuti kode dan catat di "Catatan untuk wiring" pada plan fitur.

## 1. Ringkasan epic dan urutan eksekusi

Thread tersimpan di disk dan tetap terbaca setelah daemon restart, tetapi semua sesi agent (master dan peer) mati bersama daemon. Epic ini menutup celah itu:

- Saat daemon start, semua peserta thread `active` DITANDAI mati (state tersimpan di `thread.json`). Tidak ada agent yang di-spawn, tidak ada polling, tidak ada timer.
- Pada pesan berikutnya ke thread aktif (user ke master lewat `thread.send`, atau `thread_send` ke peserta mati), agent target dihidupkan ulang secara lazy di cwd yang sama (master: `ThreadDir/master`; peer: worktree lama), MCP `shikigami-thread` terpasang otomatis, `Participant.SessionID` diganti, dan pesan sistem "agent dihidupkan ulang" ditulis.
- Saat daemon start, worktree, branch, dan direktori master sisa milik thread `closed` dibersihkan di goroutine (tanpa memblokir start). Thread `active` tidak disentuh.
- UI kecil menampilkan lencana "agent mati, akan dihidupkan saat pesan berikutnya" dan notis pesan sistem pemulihan.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`, hasFoundation=true): field opsional pada `Participant` (Go + TS), konstanta teks pemulihan (Go + TS), titik ekstensi `PreSendHook` di Manager dan pemanggilannya di `Manager.Send` dan `Router.Route`, patch kecil `participantName` frontend, stub `recovery.go`, `orphans.go`, `recovery/index.ts`. `go build ./...` dan build frontend lulus.
2. **Fitur paralel** (2 fitur, bagian 3): `thread-lazy-respawn`, `thread-orphan-cleanup`. Kontrak bagian 2 tidak berubah.
3. **Wiring** (bagian 5): satu agen menyambungkan `cmd/shikigamid/wire_thread.go` dan `frontend/src/modules/thread/index.ts`, lalu uji manual.

## 2. Kontrak bersama

### 2.1 Tipe Go (paket `internal/thread`, module `github.com/NooBeeID/shikigami`)

Ditambah di `types.go` oleh foundation (aditif, `omitempty`, JSON lama tetap valid):

```go
type Participant struct {
	SessionID string `json:"sessionId"`
	ProjectID string `json:"projectId"`
	Role      string `json:"role"`
	Name      string `json:"name"`
	// Ditambah epic thread-ticket-05 (pemulihan).
	Dead               bool     `json:"dead,omitempty"`               // true: sesi agent tidak hidup; akan dihidupkan lazy
	PreviousSessionIDs []string `json:"previousSessionIds,omitempty"` // ID sesi lama (terlama dulu, maks 20) agar from/to pada log lama tetap terbaca
}
```

Semantik: `Dead=false`/tidak ada = hidup (kompatibel dengan data lama). Nama, role, dan ProjectID peserta TIDAK berubah saat respawn; hanya `SessionID` (dan `Dead`, `PreviousSessionIDs`). Log pesan lama tidak pernah ditulis ulang.

Konstanta baru di berkas baru `internal/thread/recovery_contract.go` (milik foundation):

```go
const (
	RecoveryDeadNotice     = "daemon dimulai ulang: agent mati, akan dihidupkan saat pesan berikutnya"
	RecoveryRespawnPrefix  = "agent dihidupkan ulang: "          // + nama peserta
	RecoveryFailedPrefix   = "gagal menghidupkan ulang agent: "  // + nama peserta + ": " + alasan
	RecoveryContextMaxMessages = 40    // maks pesan log yang dimasukkan ke konteks
	RecoveryContextMaxRunes    = 12000 // batas total rune bagian log pada konteks
	RecoveryPreviousIDsMax     = 20
)
```

Ketiga teks pesan sistem adalah penanda yang disepakati dengan frontend (awalan teks tetap, field `ThreadMessage` TIDAK ditambah).

### 2.2 Titik ekstensi pra-kirim (foundation, `internal/thread/presend.go` + dua sisipan kecil)

```go
// PreSendHook dipanggil Manager.Send dan Router.Route SEBELUM pesan dicatat/diteruskan ke target.
// targetSessionIDs = sessionId target (tidak termasuk "user"). Mengembalikan peta sessionId lama -> baru
// untuk target yang baru dihidupkan ulang (nil/kosong bila tak ada). Galat hanya dicatat (log), tidak
// menggagalkan pengiriman; hook sendiri menulis pesan sistem kegagalan.
type PreSendHook func(threadID string, targetSessionIDs []string) (replaced map[string]string, err error)

func (m *Manager) SetPreSend(h PreSendHook)                                   // hook tunggal, menimpa
func (m *Manager) runPreSend(threadID string, ids []string) map[string]string // aman bila hook nil; panic dipulihkan
```

Pemanggilan: `Manager.Send` (setelah menemukan master, sebelum menghitung `to` dan `Post`) dan `routerImpl.Route` (setelah `routerResolveTargets`, sebelum `Post`). Keduanya mengganti SessionID target dengan nilai baru dari peta, sehingga `ThreadMessage.To` dan antrean memakai ID baru.

### 2.3 API Go yang disediakan fitur (nama final)

`recovery.go` (milik `thread-lazy-respawn`):

```go
type Recovery struct { /* m *Manager, mutex per thread, dll */ }
func NewRecovery(m *Manager) *Recovery
func (r *Recovery) MarkDeadOnStart() error
func (r *Recovery) EnsureAlive(threadID string, participantIds ...string) error
func (r *Recovery) PreSend(threadID string, targetSessionIDs []string) (map[string]string, error) // memenuhi PreSendHook
func BuildRecoveryContext(t Thread, projects []ProjectInfo, msgs []ThreadMessage, self Participant) string
```

`orphans.go` (milik `thread-orphan-cleanup`):

```go
func CleanOrphans(m *Manager) (cleaned int, err error) // cleaned = jumlah worktree + branch yang dihapus
```

Foundation menulis stub dengan tanda tangan persis ini (badan no-op) sehingga wiring dan fitur lain selalu compile.

### 2.4 Perilaku Recovery (dipatuhi `thread-lazy-respawn`; dipakai pembaca lain)

- Hidup/mati: peserta dianggap MATI bila `Participant.Dead == true` ATAU `Sessions.Get(sessionId)` galat / `Status != session.StatusRunning` (menyembuhkan juga agent yang mati sendiri). `MarkDeadOnStart` hanya menyetel `Dead=true` pada semua peserta thread `active` yang belum `Dead` (lewat `Manager.UpdateThread`, publish `thread.updated`), dan menulis SATU pesan sistem `RecoveryDeadNotice` per thread yang ada perubahan. Thread `closed` dilewati. Tidak memanggil `Sessions.Kill` atau `Sessions.Create`.
- `EnsureAlive(threadID, ids...)`: `ids` = sessionId peserta (ID lama dari `PreviousSessionIDs` juga diterima dan dipetakan ke peserta saat ini). Bila `ids` kosong atau memuat master, master yang mati dihidupkan; peer hanya bila ID-nya ada di `ids`. Mutex per thread milik Recovery; setelah mengunci, `Get` ulang dan cek ulang mati/hidup (idempoten, tidak men-spawn ganda). Thread `closed` -> `ErrClosed`, tidak ada respawn.
- Respawn master: `resolveMasterProvider()`, cwd `Store().MasterDir(id)` (`MkdirAll`), `Sessions.Create(session.SessionSpec{WorkspaceID, ProjectID:"", Cwd, ProviderID})`. Prompt = `BuildMasterPrompt(t, projects)` + `"\n\n"` + `BuildRecoveryContext(...)`.
- Respawn peer: ambil `WorktreeRecord` lewat `LoadWorktreeRecords(Store().ThreadDir(id))` berkunci `ProjectID`; jalur worktree = `rec.Info.Path`. Bila rekaman tidak ada atau `worktreeExists(rec.Info.Path)` false: JANGAN membuat worktree baru; tulis pesan sistem `RecoveryFailedPrefix + nama + ": worktree tidak ditemukan..."`, kembalikan galat, peserta tetap `Dead`. Sesi dibuat dengan `spawnResolveProvider()`, `Cwd = rec.Info.Path`, `ProjectID = p.ProjectID`. Prompt = `BuildPeerBrief(t, rec.ProjectName, brief, rec.Info)` dengan `brief` berisi ringkasan pemulihan + `BuildRecoveryContext`.
- MCP `shikigami-thread`: otomatis, karena `InstallLaunchContribution` menurunkan thread dari cwd (`ThreadIDFromCwd`) dan hanya memasang bila thread `active`; token baru terbit per `Sessions.Create`. Auto-approve dari `provider/launch.go`. Recovery TIDAK perlu menyentuh jalur itu.
- Setelah `Sessions.Create` sukses: ganti entri peserta di tempat lewat `Manager.UpdateThread` (cari berdasar `SessionID` lama): `SessionID=baru`, `Dead=false`, `PreviousSessionIDs=append(lama..., SessionIDLama)` dipotong ke `RecoveryPreviousIDsMax` terbaru. Lalu `UpsertWorktreeRecord` (peer, perbarui `SessionID`), panggil `m.managerFireParticipant(t, p)` (hook peserta), `PostSystem(RecoveryRespawnPrefix + nama)`. `thread.updated` terbit otomatis dari `UpdateThread`.
- Pengiriman prompt konteks: SINKRON dengan retry 5x300 ms bila `errors.Is(err, mention.ErrNotFound)` lewat `m.SendToSession(threadID, "", newSessionID, prompt)` SEBELUM `EnsureAlive` kembali, agar urutan antrean memastikan prompt konteks mendahului pesan user/thread. Kegagalan kirim prompt: pesan sistem peringatan, bukan galat fatal (sesi sudah hidup).
- Kegagalan `Sessions.Create` atau provider tidak ada: log `Warn`, `PostSystem(RecoveryFailedPrefix ...)`, kembalikan galat, `Dead` tetap true, tanpa retry otomatis. Pesan user tetap tercatat di log (karena `Send` memanggil hook sebelum `Post` dan tidak menghentikan `Post` bila hook gagal).
- Resume sesi provider: provider saat ini TIDAK punya kapabilitas resume (`provider.Capabilities` hanya `Hooks/MCP/IdleEvent`, dan id sesi provider tidak disimpan). Maka jalur yang diimplementasikan adalah fallback: spawn baru dengan konteks log. Jangan menambah field/kemampuan provider; cukup catat di komentar bahwa resume belum didukung.
- `BuildRecoveryContext`: bagian teks Indonesia berisi (a) pernyataan bahwa ini pemulihan setelah restart daemon dan agent diminta melanjutkan, bukan mulai dari awal; (b) goal; (c) daftar peserta (nama, role, nama project, mati/hidup); (d) log thread terakhir: maks `RecoveryContextMaxMessages` pesan berurutan waktu, kind `message` dan `system` saja (activity diabaikan), tiap teks dipotong ke 600 rune, total dipotong dari yang terlama hingga <= `RecoveryContextMaxRunes`; nama pengirim/penerima diselesaikan lewat nama peserta (termasuk `PreviousSessionIDs`); (e) untuk master: peringatan "peserta mati dihidupkan otomatis saat kamu thread_send ke mereka; jangan thread_spawn ulang project yang sudah punya peserta".
- Anti-loop tidak disentuh.

### 2.5 Perilaku CleanOrphans (dipatuhi `thread-orphan-cleanup`)

- Hanya thread `closed` (semua workspace; sumber `m.managerAllThreads()` atau `Store` yang setara). Thread `active` TIDAK disentuh. `thread.json`, `messages.jsonl`, `worktrees.json` tidak dihapus.
- Per thread `T` dengan dir `D = Store().ThreadDir(T.ID)`:
  1. Kumpulkan repo project kandidat: (a) `ProjectPath` dari `LoadWorktreeRecords(D)`; (b) repo induk dari berkas `.git` (format `gitdir: <repo>/.git/worktrees/<n>`) di tiap subdirektori `D/<x>` selain `master`; (c) path project dari `m.cfg.Workspaces.Projects(T.WorkspaceID)` bila tersedia. Hanya repo yang ada di disk dan repo git.
  2. Worktree sisa: tiap subdirektori `D/<x>` (selain `master`) yang ada dan/atau terdaftar di `git worktree list --porcelain` repo dengan path di bawah `D`. Hapus: `git worktree remove --force --force <path>`, lalu `git worktree prune`; bila repo hilang atau perintah gagal, `os.RemoveAll(path)` HANYA bila path berada di bawah `D` dan bukan `D` itu sendiri.
  3. Branch sisa: tiap repo kandidat, `git branch --list "thread/<T.ID>-*"` -> `git branch -D <nama>`. Polanya WAJIB memuat ID thread penuh; tidak menyentuh branch lain.
  4. Direktori master `D/master`: `os.RemoveAll` bila ada.
  5. Tiap perintah git memakai `context.WithTimeout` (20 detik). Helper yang boleh dipakai ulang tanpa mengubahnya (baca-saja): `worktreeGit()`, `worktreeRun(ctx, gitBin, dir, args...)`, `worktreeLines`, `worktreeBranchExists`, `worktreeExists`, `LoadWorktreeRecords`. `RemoveWorktree` TIDAK dipakai (mensyaratkan prefix branch `thread/` pada info, tidak cocok untuk sisa tanpa rekaman).
  6. Log per tindakan dan per kegagalan lewat `m.log` (`Info` untuk tindakan, `Warn` untuk gagal), ringkasan per thread: `thread: pembersihan yatim` dengan field `thread`, `worktrees`, `branches`, `masterDir`. Satu kegagalan tidak menghentikan thread/project lain; galat dikumpulkan dengan `errors.Join` dan dikembalikan di akhir.
  7. Idempoten: thread yang sudah bersih -> tidak ada tindakan, tidak ada log `Info`, tanpa galat. `cleaned` hanya menghitung penghapusan nyata.
- `CleanOrphans` sendiri sinkron; pemanggil (wiring) yang menjalankannya di goroutine.

### 2.6 IPC dan event

Tidak ada method IPC baru. `thread.get`, `thread.list`, dan event `thread.updated` membawa `participants[].dead` dan `participants[].previousSessionIds`. Pesan pemulihan memakai event `thread.message` dengan `kind: "system"` dan teks berawalan konstanta bagian 2.1.

### 2.7 Frontend

Mirror di `frontend/src/modules/thread/types.ts` (oleh foundation):

```ts
export interface Participant { sessionId: string; projectId: string; role: ThreadRole; name: string;
  dead?: boolean; previousSessionIds?: string[] }   // ditambah epic thread-ticket-05
export const RECOVERY_DEAD_NOTICE = 'daemon dimulai ulang: agent mati, akan dihidupkan saat pesan berikutnya';
export const RECOVERY_RESPAWN_PREFIX = 'agent dihidupkan ulang: ';
export const RECOVERY_FAILED_PREFIX = 'gagal menghidupkan ulang agent: ';
```

Foundation juga mem-patch `participantName` di `thread-store.ts` agar mencocokkan `previousSessionIds` (nama peserta tetap tampil untuk pesan lama).

Folder `frontend/src/modules/thread/recovery/` (milik `thread-orphan-cleanup`), Svelte 5 runes, TypeScript:

- `DeadBadge.svelte` props `{participant: Participant}`: lencana kecil bertuliskan "agent mati, akan dihidupkan saat pesan berikutnya"; tidak merender apa pun bila `!participant.dead`.
- `RecoveryPanel.svelte` props `{thread: Thread}` (+ `message?` diabaikan): bagian kolom kanan yang mendaftar peserta dengan `dead === true` (nama + `DeadBadge`); tidak merender apa pun bila thread `closed` atau tidak ada yang mati. Hilang otomatis saat `thread.updated` membawa `dead` kosong, karena membaca prop reaktif.
- `RecoveryNotice.svelte` props `{thread: Thread; message?: ThreadMessage}`: hanya merender bila `message.kind === 'system'` dan teks sama dengan `RECOVERY_DEAD_NOTICE` atau berawalan `RECOVERY_RESPAWN_PREFIX`/`RECOVERY_FAILED_PREFIX`; gaya ringan (info; kegagalan bergaya peringatan). TIDAK menggantikan renderer pesan sistem milik tiket 1 (hanya tambahan di bawah pesan, lewat slot `message-extra`).
- `index.ts`: `export function installRecovery(): () => void` (idempoten, pola sama dengan `installParticipants`) yang memanggil `registerThreadSlot` dua kali: `{slot:'side-panel', component: RecoveryPanel, order: 20, id:'recovery-panel'}` dan `{slot:'message-extra', component: RecoveryNotice, order: 20, id:'recovery-notice'}`; plus ekspor `DeadBadge`, `RecoveryPanel`, `RecoveryNotice`, `isRecoveryMessage(m)`, `deadParticipants(thread)`.

Slot yang tersedia (dari `slots.ts`): `header-actions`, `above-input`, `message-extra`, `side-panel`. `ParticipantRow.svelte` (milik tiket 2) TIDAK punya slot; karena itu lencana dipasang sebagai bagian `side-panel` terpisah di bawah panel peserta (order 0), bukan di dalam baris. Tidak ada berkas tiket lain yang diedit untuk itu.

### 2.8 Konvensi penamaan

- Go: berkas dan simbol baru berawalan sesuai fitur (`recovery*`, `orphans*`) untuk helper tak diekspor (pola yang sama dengan `worktree*`, `spawn*`). Komentar dan pesan galat berbahasa Indonesia.
- Pesan sistem: huruf kecil di awal, tanpa titik akhir, sesuai konstanta.
- Frontend: komponen PascalCase, `.ts` camelCase, props `{thread}`/`{participant}` seperti komponen tiket sebelumnya.

### 2.9 Koreksi/keputusan terhadap tiket

1. **Titik ekstensi pra-kirim dibuat di foundation, bukan di wiring.** Tiket membolehkan "hook aditif kecil di titik itu"; hook yang ada (`OnMessage`) terpanggil sesudah pencatatan dan tidak bisa mencegat. Foundation menambah `PreSendHook` (bagian 2.2). Wiring cukup `m.SetPreSend(rec.PreSend)`.
2. **`EnsureAlive` tetap `error`; peta penggantian ID lewat `PreSend`.** `PreSend` membungkus logika internal yang mengembalikan peta lama->baru.
3. **Master dihidupkan hanya bila `ids` kosong atau memuat master** (bukan setiap panggilan). Jalur `thread.send` memberi ID master; jalur router memberi ID target. Ini penyempurnaan dari "master selalu dihidupkan pada pesan user" agar `thread_send` ke peer tidak membangunkan master tanpa perlu.
4. **Tanda mati = field `Participant.Dead` + cek sesi nyata**, bukan penanda memori.
5. **Resume provider belum didukung** (tidak ada kapabilitas atau id sesi provider); implementasi hanya fallback spawn baru + konteks log.
6. **Lencana di slot `side-panel`**, bukan di dalam `ParticipantRow` (tidak ada slot per baris).
7. Pembersihan yatim menulis fungsi hapus sendiri di `orphans.go` (tiket membolehkan; `RemoveWorktree` tidak cocok).
8. Batas yang diketahui, TIDAK diperbaiki di epic ini: `thread_spawn` pada project yang peer-nya mati menghasilkan "agen ... sudah berhenti" (`spawn.go`, tiket 2); master diberi tahu lewat konteks pemulihan untuk memakai `thread_send` yang memicu respawn. `thread_participants` tidak menandai status mati (tiket 2). `OwnedSessions` memuat ID sesi lama yang sudah mati; tidak berbahaya.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `thread-lazy-respawn` | `Recovery` (`NewRecovery`, `MarkDeadOnStart`, `EnsureAlive`, `PreSend`, `BuildRecoveryContext`): menandai peserta mati saat start; respawn lazy master/peer di cwd yang sama, fallback konteks log, penggantian SessionID, pesan sistem, mutex per thread, penanganan kegagalan dan worktree hilang. | `internal/thread/recovery.go` |
| `thread-orphan-cleanup` | `CleanOrphans` (bagian 2.5) dan UI `recovery/` (`DeadBadge`, `RecoveryPanel`, `RecoveryNotice`, `installRecovery`, bagian 2.7). | `internal/thread/orphans.go`, `frontend/src/modules/thread/recovery/` |

Kedua fitur independen penuh (tanpa impor silang) setelah foundation. Hanya titik temu: nama field `dead`/`previousSessionIds` dan konstanta teks, semuanya dari foundation.

## 4. Foundation

Ada (`hasFoundation=true`): lihat `00-foundation.md`. Isi: field `Participant` (Go + TS), `recovery_contract.go`, `presend.go` + sisipan di `manager.go`/`router.go`, konstanta + patch `participantName` di frontend, stub `recovery.go`, `orphans.go`, `recovery/index.ts`. Tidak ada paket baru.

## 5. Wiring checklist

Dikerjakan SETELAH kedua fitur selesai, oleh satu agen. Hanya edit aditif, jangan merombak.

1. **`cmd/shikigamid/wire_thread.go`**, di `wireThread`, setelah `InstallActivity` dan setelah pemasangan lain milik tiket 04 bila ada (mis. `RegisterClose`), TEPAT sebelum `m.FireStart()`:
   ```go
   rec := thread.NewRecovery(m)
   m.SetPreSend(rec.PreSend)
   m.OnStart(func(*thread.Manager) {
       if err := rec.MarkDeadOnStart(); err != nil {
           d.Logger.Warn("thread: gagal menandai agent mati saat start", "err", err)
       }
   })
   m.OnStart(func(mm *thread.Manager) {
       go func() {
           defer func() { if r := recover(); r != nil { d.Logger.Error("thread: panic pembersihan yatim", "panic", r) } }()
           if n, err := thread.CleanOrphans(mm); err != nil {
               d.Logger.Warn("thread: pembersihan yatim selesai dengan galat", "cleaned", n, "err", err)
           } else if n > 0 {
               d.Logger.Info("thread: pembersihan yatim selesai", "cleaned", n)
           }
       }()
   })
   ```
   Hook `OnStart` berjalan berurutan; `MarkDeadOnStart` terdaftar lebih dulu dan sinkron, `CleanOrphans` di goroutine sehingga tidak memblokir start. Jangan menghapus atau mengubah hook `OnStart` yang sudah ada. Urutan `FireStart` tetap yang terakhir.
2. **`frontend/src/modules/thread/index.ts`**: tambah `import { installRecovery } from './recovery';` dan panggil `installRecovery();` tepat setelah `installParticipants();` (dan setelah `installClose()`/`installActivity()` bila ada; urutan tidak kritis, slot memakai `order`).
3. **Verifikasi tidak ada edit lain yang diperlukan**: pra-kirim sudah dipasang foundation (`Manager.Send`, `Router.Route`), `participantName` sudah dipatch foundation. Pastikan `thread.updated` membawa `dead` (cek `thread.get` lewat `shikigamictl` atau devtools).
4. **Uji jalan manual** (jalankan aplikasi dengan `make run-daemon`/app):
   - Buat thread aktif dengan satu peer (master men-spawn project git). Catat `git worktree list` dan nama branch `thread/<id>-<slug>`.
   - Hentikan daemon lalu jalankan ulang. `thread.list`/`thread.get`: peserta `dead: true`, log utuh, tidak ada sesi agent baru (`shikigamictl session list`). UI: lencana "agent mati, akan dihidupkan saat pesan berikutnya" di panel kanan dan pesan sistem pemulihan di aliran.
   - Kirim pesan dari kolom input: master hidup ulang (cwd `.../threads/<id>/master`), pesan sistem "agent dihidupkan ulang: master", prompt konteks lalu pesan user sampai ke master setelah idle, lencana master hilang. Peer masih mati sampai master `thread_send` ke peer; lalu peer hidup di worktree yang SAMA (`git worktree list` tidak bertambah).
   - Kirim dua pesan beruntun cepat saat respawn: hanya satu sesi baru per peserta.
   - Hapus worktree peer secara manual lalu `thread_send` ke peer mati: pesan sistem kegagalan berawalan "gagal menghidupkan ulang agent:", tidak ada worktree baru, tidak ada crash.
   - Tutup satu thread (tiket 04), buat ulang sisa secara manual (`git worktree add` di `<DataDir>/threads/<id>/<slug>` dengan branch `thread/<id>-x`), restart daemon: sisa hilang, ada baris log pembersihan, `thread.json` dan `messages.jsonl` utuh, thread active tidak berubah. Restart lagi: tidak ada tindakan, tanpa galat.
   - Kirim pesan ke thread `closed`: tetap ditolak "thread sudah ditutup".
5. Commit wiring menyebut: pendaftaran di `wire_thread.go` dan `thread/index.ts`.

## 6. Build guard

Satu-satunya guard (tanpa unit test):

- Go: `go build ./...` (dari root repo).
- Frontend: `cd frontend && npm run build` (lalu `npm run check` opsional bila bersih; kegagalan lama yang bukan dari epic ini tidak diperbaiki).
- Penuh sebelum akhir wiring: `make build-ui && make build-daemon` (aplikasi harus tetap berjalan).

## 7. Aturan paralel

- Tiap agent fitur hanya menyentuh `ownedPaths`-nya (bagian 3). Jangan menyentuh path fitur lain, berkas foundation, atau berkas wiring (`cmd/shikigamid/wire_thread.go`, `frontend/src/modules/thread/index.ts`).
- Berkas yang dibuat foundation (`recovery_contract.go`, `presend.go`, stub) hanya dibaca; `recovery.go`, `orphans.go`, `recovery/index.ts` BOLEH ditimpa total oleh pemiliknya selama tanda tangan bagian 2.3 dan ekspor `installRecovery` dipertahankan.
- Boleh memanggil helper tak diekspor milik tiket sebelumnya dalam paket yang sama (baca-saja), tidak mengubahnya. Bila butuh perubahan di berkas lain, JANGAN edit; tulis di "Catatan untuk wiring" pada plan fitur.
- Tanpa dependency baru, tanpa unit test, tanpa polling/timer, tanpa menyentuh sesi interaktif milik user.
- Setiap agent menjalankan build guard (bagian 6) sebelum melapor selesai.
