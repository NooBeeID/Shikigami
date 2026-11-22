# Plan: Kontrak dan penyimpanan thread

## Metadata

- Tiket: `thread-ticket-01-thread-core` (`docs/tickets/thread-ticket-01-thread-core.md`)
- Fitur: `thread-foundation` (Kontrak dan penyimpanan thread)
- Rujukan wajib: `docs/plans/thread-ticket-01-thread-core/overview.md` (bagian 2.1 penamaan, 2.3 signature `types.go` dan `store.go`, bagian 7 aturan paralel). Jangan mengubah overview.
- ownedPaths: `internal/thread/types.go`, `internal/thread/store.go`
- Depends on foundation: ya. Foundation (`00-foundation.md`) sudah membuat `types.go` final dan `store.go` stub berkompilasi. Plan ini menimpa stub itu. Bila saat mulai `internal/thread/` belum ada, hentikan dan laporkan; jangan membuat berkas di luar ownedPaths.

## Tujuan

Mengganti stub `Store` dengan implementasi penuh berbasis disk: `thread.json` ditulis atomik, `messages.jsonl` append-only aman konkuren, baris rusak dilewati dengan log peringatan, izin berkas 0600 dan direktori 0700, `List` per workspace urut `createdAt` menurun termasuk thread `closed`. Menyempurnakan komentar `types.go` tanpa mengubah bentuk atau tag json. Signature publik tidak berubah (overview 2.3), sehingga fitur `thread-master-daemon` yang berkompilasi terhadap stub tetap berkompilasi.

## Path yang dimiliki

HANYA berkas berikut yang boleh disentuh:

- `internal/thread/types.go` (hanya komentar; tidak mengubah tipe, konstanta, tag json, atau signature)
- `internal/thread/store.go` (ditulis ulang penuh)

Jangan menyentuh `manager.go`, `master.go`, `prompt.go`, `ipc.go`, `cmd/shikigamid/*` (termasuk `wire_thread.go`), `internal/mention`, `internal/paths`, frontend, `go.mod`, maupun berkas wiring. Bila butuh perubahan di luar itu, catat di akhir sebagai "Catatan untuk wiring"; jangan ubah.

## Kontrak

Disediakan (signature final, overview 2.3; JANGAN diubah):

```go
var (
    ErrNotFound = errors.New("thread tidak ditemukan")
    ErrInvalid  = errors.New("masukan thread tidak valid")
)
type Store struct { /* tak diekspor */ }
func NewStore(baseDir string) *Store
func (s *Store) Create(workspaceID, title, goal string) (Thread, error)
func (s *Store) Get(id string) (Thread, error)
func (s *Store) List(workspaceID string) ([]Thread, error)
func (s *Store) Save(t Thread) error
func (s *Store) AppendMessage(m ThreadMessage) error
func (s *Store) Messages(id string, sinceTS time.Time) ([]ThreadMessage, error)
func (s *Store) ThreadDir(id string) string
func (s *Store) MasterDir(id string) string
```

Dipakai: `Thread`, `ThreadMessage`, `StateActive`, `NewID()` dari `types.go`; stdlib saja (`encoding/json`, `os`, `path/filepath`, `bufio`, `sort`, `sync`, `log/slog`, `errors`, `fmt`, `strings`, `time`, `io`). Tanpa dependency baru.

Semantik yang ditetapkan plan ini (konsumen: manager, IPC, tiket recovery/close):

- Layout disk: `<baseDir>/<id>/thread.json`, `<baseDir>/<id>/messages.jsonl`, `<baseDir>/<id>/master/`.
- `ErrNotFound` dikembalikan apa adanya atau dibungkus `fmt.Errorf("...: %w", ErrNotFound)` sehingga `errors.Is` bekerja. Begitu pula `ErrInvalid`.
- ID divalidasi (mencegah path traversal): tidak kosong, hanya `[A-Za-z0-9_-]`, panjang <= 64. ID tidak valid pada `Get`/`Messages`/`AppendMessage`/`Save` -> `ErrInvalid`; pada `Get` ID tidak valid boleh diperlakukan sebagai `ErrNotFound` (pilih `ErrNotFound` untuk `Get` dan `Messages` agar IPC tidak membocorkan perbedaan; `ErrInvalid` untuk `Save`/`AppendMessage`). `ThreadDir`/`MasterDir` tidak memvalidasi (hanya `filepath.Join`), pemanggil internal store selalu memvalidasi dulu.
- `Participants` selalu `[]` (tidak pernah `null`) pada nilai yang dikembalikan `Create`/`Get`/`List` dan pada berkas yang ditulis `Save`.
- `Messages` selalu mengembalikan slice non-nil (`[]ThreadMessage{}`), `List` juga (`[]Thread{}`).
- Store tidak menegakkan "thread closed menolak penulisan pesan"; itu tugas manager. Store tetap bisa membaca thread closed.

## Langkah

### 1. `internal/thread/types.go`: penyempurnaan komentar saja

1. Pastikan package comment memuat: ringkasan paket, konvensi path `DataDir/threads/<threadId>/{thread.json,messages.jsonl,master/,<projectSlug>/}` dan branch `thread/<threadId>-<slug>` (worktree diimplementasi tiket thread-02, bukan di sini), serta catatan format penyimpanan (JSON camelCase, waktu UTC RFC3339Nano). Lengkapi bila kurang.
2. Tambahkan/lengkapi doc comment di tiap tipe dan konstanta: arti nilai `From`/`To` (`"user"`, sessionId, `"all"`, `"system"`), `Participant.ProjectID` kosong untuk master, `Thread.Participants` selalu `[]` bukan `null`, `ReadyToClose` hanya ditulis tiket thread-agents/thread-close, `ClosedAt` nil selama aktif, `MessagesParams.SinceTS` berformat RFC3339/RFC3339Nano.
3. Pertahankan kalimat "Bentuk stabil; tiket lain hanya boleh menambah field opsional dengan omitempty."
4. DILARANG: mengganti nama/urutan field, mengubah tag json, menghapus konstanta, mengubah `NewID`. Jangan menambah field baru (tidak diperlukan oleh plan ini). Verifikasi dengan `git diff internal/thread/types.go`: hanya baris komentar yang berubah.

### 2. `internal/thread/store.go`: struktur dan konstruktor

1. Hapus semua badan stub dan komentar `// STUB(foundation)`. Tulis komentar paket-tingkat berkas dalam bahasa Indonesia yang menjelaskan layout disk dan jaminan (atomik, append-only, mutex per thread).
2. Konstanta tak diekspor: `storeFileThread = "thread.json"`, `storeFileMessages = "messages.jsonl"`, `storeDirMaster = "master"`, `storePermFile os.FileMode = 0o600`, `storePermDir os.FileMode = 0o700`, `storeMaxIDLen = 64`.
3. Struct:

   ```go
   type Store struct {
       baseDir string
       mu      sync.Mutex             // melindungi peta locks
       locks   map[string]*sync.Mutex // satu mutex per threadId
       log     *slog.Logger
   }
   ```

4. `func NewStore(baseDir string) *Store`: `os.MkdirAll(baseDir, storePermDir)`; galat dicatat `slog.Warn` (konstruktor tidak mengembalikan error; operasi berikutnya akan mengembalikan galat nyata). Inisialisasi `locks` dan `log = slog.Default()` (logger nullable: bila nil di masa depan fallback ke `slog.Default()`). Bila `baseDir` kosong, simpan apa adanya (jangan panic).
5. Helper `func (s *Store) storeLock(id string) *sync.Mutex`: ambil/buat mutex per id di bawah `s.mu`. Mutex per thread ini melindungi `AppendMessage`, `Save`, dan `Create` untuk id yang sama. Peta tidak dibersihkan (jumlah thread kecil).
6. Helper `storeValidID(id string) bool` sesuai aturan di bagian Kontrak.
7. `ThreadDir(id) string` = `filepath.Join(s.baseDir, id)`; `MasterDir(id)` = `filepath.Join(s.ThreadDir(id), storeDirMaster)`.

### 3. `store.go`: helper I/O

1. `func storeWriteFileAtomic(path string, data []byte, perm os.FileMode) error`: buat file sementara di direktori yang sama (`os.CreateTemp(dir, ".thread-*.tmp")`), `Chmod(perm)`, tulis, `Sync()`, `Close()`, lalu `os.Rename(tmp, path)`. Pada galat apa pun hapus berkas sementara (`os.Remove`). Rename di direktori yang sama menjamin atomik di macOS/Linux.
2. `func (s *Store) storeReadThread(id string) (Thread, error)`: baca `thread.json`; `os.IsNotExist` -> `ErrNotFound`; JSON tidak valid -> `fmt.Errorf("thread.json rusak (%s): %w", id, err)` (BUKAN `ErrNotFound`); normalisasi `Participants == nil` menjadi `[]Participant{}`.
3. `func storeNormalize(t Thread) Thread`: `Participants` nil -> `[]Participant{}`; `CreatedAt` dan `ClosedAt` (bila non-nil) dikonversi `.UTC()`.

### 4. `store.go`: `Create`

`func (s *Store) Create(workspaceID, title, goal string) (Thread, error)`:

1. `strings.TrimSpace(workspaceID) == ""` -> `fmt.Errorf("workspaceId kosong: %w", ErrInvalid)`. Judul dan goal TIDAK divalidasi/diturunkan di sini (manager yang menurunkan judul); simpan apa adanya.
2. `id := NewID()`; bila direktori `ThreadDir(id)` sudah ada (tabrakan sangat tidak mungkin), buat ID baru satu kali lagi, lalu galat bila masih tabrakan.
3. Buat `ThreadDir(id)` lalu `MasterDir(id)` dengan `os.MkdirAll(..., 0o700)`; `os.Chmod` ke 0700 agar tidak bergantung umask.
4. Susun `Thread{ID: id, WorkspaceID: workspaceID, Title: title, Goal: goal, State: StateActive, CreatedAt: time.Now().UTC(), Participants: []Participant{}, ReadyToClose: false}`.
5. Di bawah `storeLock(id)`: buat `messages.jsonl` kosong dengan `os.OpenFile(path, O_CREATE|O_WRONLY|O_APPEND, 0o600)` lalu tutup; tulis `thread.json` lewat `storeWriteFileAtomic` (JSON `MarshalIndent` dua spasi, akhiri newline). Urutan: `messages.jsonl` dulu, `thread.json` terakhir, sehingga thread baru "terlihat" oleh `List` hanya setelah lengkap.
6. Bila langkah 3 atau 5 gagal, hapus `ThreadDir(id)` dengan `os.RemoveAll` (hanya direktori yang baru dibuat langkah ini) dan kembalikan galat berbungkus konteks.
7. Kembalikan thread.

### 5. `store.go`: `Get`, `List`, `Save`

1. `Get(id)`: `!storeValidID(id)` -> `ErrNotFound` (dibungkus pesan "thread tidak ditemukan: <id>"); selainnya `storeReadThread`.
2. `List(workspaceID)`: `os.ReadDir(baseDir)`; `os.IsNotExist` -> `[]Thread{}, nil`. Untuk tiap entri direktori dengan nama valid, `storeReadThread`; jika `ErrNotFound` (direktori tanpa thread.json, mis. sisa `Create` yang gagal) lewati diam-diam; jika galat lain (JSON rusak) lewati dengan `s.log.Warn("thread.json rusak, dilewati", "id", name, "err", err)`. Saring `t.WorkspaceID == workspaceID` (string kosong tidak berarti "semua"; hanya cocok bila persis sama). Termasuk `closed`. Urutkan dengan `sort.SliceStable`: `CreatedAt` menurun, lalu `ID` menurun sebagai pemutus seri. Kembalikan `[]Thread{}` bila kosong.
3. `Save(t Thread)`: `!storeValidID(t.ID)` -> `ErrInvalid`; pastikan `thread.json` yang ada ada di disk (direktori thread harus sudah dibuat `Create`): `os.Stat(ThreadDir)` tidak ada -> `ErrNotFound` (Save tidak membuat thread baru). `storeNormalize`, marshal (format sama dengan Create), tulis lewat `storeWriteFileAtomic` di bawah `storeLock(t.ID)`.

### 6. `store.go`: `AppendMessage` dan `Messages`

1. `AppendMessage(m ThreadMessage)`:
   - `!storeValidID(m.ThreadID)` -> `ErrInvalid`. `m.ID` kosong atau `m.TS` nol -> `ErrInvalid` (kontrak: diisi pemanggil). Store tidak mengisi nilai default.
   - `os.Stat(ThreadDir)` tidak ada -> `ErrNotFound`.
   - `m.TS = m.TS.UTC()`; `json.Marshal(m)` (tanpa indentasi; `encoding/json` sudah meng-escape newline di dalam string sehingga satu pesan = satu baris); tambahkan `'\n'` ke buffer yang sama.
   - Di bawah `storeLock(m.ThreadID)`: `os.OpenFile(path, O_CREATE|O_WRONLY|O_APPEND, 0o600)`, SATU kali `Write` untuk seluruh buffer (baris + newline), `Close`. Bila `Write` menulis sebagian, kembalikan galat (baris parsial akan dilewati pembaca sebagai rusak). Tanpa `Sync` per pesan (cukup; hindari biaya); pembacaan setelah proses baru tetap konsisten pada penutupan normal.
2. `Messages(id, sinceTS)`:
   - ID tidak valid -> `ErrNotFound`; `thread.json`/direktori tidak ada -> `ErrNotFound`; `messages.jsonl` tidak ada tetapi thread ada -> `[]ThreadMessage{}, nil`.
   - Baca di bawah `storeLock(id)` agar tidak membaca baris yang sedang ditulis separuh oleh `AppendMessage` dari goroutine lain di proses yang sama. Baca seluruh isi dengan `os.ReadFile` lalu pisah per baris (`bytes.Split(data, "\n")`); jangan memakai `bufio.Scanner` (batas token 64 KB menggagalkan pesan panjang). Lepas lock setelah `ReadFile`, parse di luar lock.
   - Tiap baris: `TrimSpace`; kosong -> lewati diam-diam. `json.Unmarshal` ke `ThreadMessage`; gagal, atau `ID == ""`, atau `ThreadID != id` -> lewati dengan `s.log.Warn("baris pesan thread rusak, dilewati", "thread", id, "line", nomorBaris, "err", err)`; JANGAN menggagalkan pembacaan.
   - Filter: `sinceTS.IsZero()` -> semua; selainnya hanya `m.TS.After(sinceTS)` (strictly lebih baru).
   - Urutkan `sort.SliceStable` menurut `TS` naik (urutan berkas dipertahankan untuk TS sama). Kembalikan `[]ThreadMessage{}` bila kosong.

### 7. Verifikasi build (tanpa unit test)

1. `gofmt -l internal/thread` tidak menghasilkan keluaran untuk dua berkas ini; `go vet ./internal/thread/` (bila berkas fitur lain sedang tidak berkompilasi di luar kendali Anda, cukup pastikan tidak ada galat yang menunjuk `types.go`/`store.go`).
2. `go build ./internal/thread/...` dan `go build ./...` lulus (guard).
3. `git diff --stat` hanya menunjukkan `internal/thread/types.go` dan `internal/thread/store.go` (di antara berkas milik fitur ini).
4. Pemeriksaan manual opsional di scratchpad (bukan berkas repo, bukan unit test): program `main` sekali pakai di luar repo yang membuat `NewStore(tmp)`, `Create`, `AppendMessage` beberapa kali dari banyak goroutine, menyuntik baris sampah ke `messages.jsonl`, lalu memanggil `Messages` pada `NewStore(tmp)` kedua untuk mensimulasikan proses baru. Hapus setelah selesai; jangan di-commit.

## Perubahan

- Diubah: `internal/thread/types.go` (hanya komentar/dokumentasi).
- Diubah (ditulis ulang penuh): `internal/thread/store.go` (menggantikan stub Foundation).
- Tidak ada berkas dibuat/dihapus di repo; tidak ada perubahan `go.mod`.

## Wiring

Tidak ada yang disambungkan oleh fitur ini. Yang diekspos untuk langkah wiring dan fitur lain:

- `thread.NewStore(filepath.Join(dataDir, "threads"))` dipanggil oleh `wire_thread.go` (milik `thread-master-daemon`; `dataDir = d.DataDir`, fallback `paths.DataDir()`), hasilnya diteruskan ke `ManagerConfig.Store`.
- `Store` aman dipakai bersamaan dari banyak goroutine (mutex per thread untuk `Save`/`AppendMessage`/baca pesan). Read-modify-write atas `Thread` (Get lalu Save) TIDAK atomik di tingkat store; manager menyerialkannya dengan mutex sendiri (`UpdateThread`).
- Galat yang dapat dipetakan manager/IPC: `errors.Is(err, ErrNotFound)` -> `ipc.ErrNotFound`, `errors.Is(err, ErrInvalid)` -> `ipc.ErrInvalid`, lainnya -> `ipc.ErrInternal`.
- `ThreadDir`/`MasterDir` dipakai `master.go` (cwd master) dan tiket worktree berikutnya.

Catatan untuk wiring: tidak ada. Bila selama implementasi ditemukan kebutuhan di luar ownedPaths, tulis di sini pada hasil kerja, jangan ubah berkasnya.

## Selesai bila

- [ ] `store.go` tidak lagi berisi stub (`grep -n "belum diimplementasikan\|STUB" internal/thread/store.go` kosong).
- [ ] Semua signature `Store` identik dengan overview 2.3; `ErrNotFound` dan `ErrInvalid` masih didefinisikan di `store.go` dengan teks yang sama.
- [ ] `Create` membuat `<id>/thread.json` (0600), `<id>/messages.jsonl` kosong (0600), `<id>/master/` (0700), state `active`, `participants: []`.
- [ ] `Save` atomik (tmp di direktori yang sama lalu rename; tmp dibersihkan saat galat); `AppendMessage` satu `Write` per baris dengan `O_APPEND` di bawah mutex per thread.
- [ ] `Messages` melewati baris rusak dengan log peringatan, memfilter `TS > sinceTS`, urut naik; `List` terfilter per workspace, `createdAt` menurun, termasuk `closed`, melewati `thread.json` rusak dengan log.
- [ ] `types.go` hanya berubah di komentar (diff menunjukkan tidak ada perubahan tipe/tag/konstanta).
- [ ] Guard: `go build ./internal/thread/...` dan `go build ./...` sukses. Tanpa unit test; tanpa menyentuh path fitur lain.
