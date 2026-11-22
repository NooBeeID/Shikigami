# Plan: Diff hash dan build inkremental

## Metadata

- Tiket: `grimoire-ticket-03-grimoire-refresh` (`docs/tickets/grimoire-ticket-03-grimoire-refresh.md`)
- Fitur: `diff-incremental` (Diff hash dan build inkremental)
- ownedPaths: `internal/grimoire/diff.go`, `internal/grimoire/incremental.go`
- Depends on foundation: ya. `00-foundation.md` sudah membuat stub berkompilasi untuk kedua berkas (baris pertama `// STUB(foundation): ditimpa oleh diff-incremental`). Plan ini MENIMPA PENUH kedua berkas itu dan menghapus baris STUB. Signature tidak boleh berubah.
- Sumber kebenaran bersama: `overview.md` (bagian 2.2 koreksi, 2.3 signature). Plan ini tidak mengulangnya; hanya merinci implementasi.

## Tujuan

Mengganti build penuh tiket 01 yang selalu menulis ulang semua modul dengan jalur inkremental: hash isi working tree per modul dibandingkan dengan `state.json`; hanya modul Added/Modified/Stale yang ditulis ulang utuh, modul Removed dihapus `books/<nama>/`-nya beserta entri state, modul Unchanged tidak disentuh. `index.md` dibangun ulang dan `state.json` disimpan atomik. Diff kosong dan `Full=false` tidak menulis apa pun. Disediakan hook `Summarizer` (nil = statik) sebagai titik masuk tiket 4. Fitur ini hanya logika paket `grimoire`; pemicu sesi, IPC, CLI, dan UI dikerjakan fitur/langkah lain.

## Path yang dimiliki

HANYA dua berkas ini yang boleh disentuh:

- `internal/grimoire/diff.go`
- `internal/grimoire/incremental.go`

Jangan menyentuh `trigger.go`, `cmd/shikigamid/wire_grimoire_refresh.go`, `frontend/src/modules/grimoire/stale/`, berkas tiket 01/02 (`build.go`, `store.go`, `ipc.go`, dst.), berkas wiring, `go.mod`/`go.sum`, atau `docs/plans/.../overview.md`. Tidak ada unit test, tidak ada dependency baru, tidak ada berkas lain.

## Kontrak

Signature final (JANGAN diubah; sama persis dengan overview 2.3):

`diff.go`
```go
type ChangeKind int
const ( ChangeUnchanged ChangeKind = iota; ChangeAdded; ChangeModified; ChangeRemoved )
type ModuleChange struct { Name string; Kind ChangeKind; OldHash, NewHash string }
type Diff struct { Added, Modified, Removed, Unchanged []ModuleChange; Stale []ModuleChange }
func ComputeDiff(scanned []Module, hashes map[string]string, st *State) Diff
func (d Diff) Pending() []ModuleChange
func (d Diff) Empty() bool
```

`incremental.go`
```go
type Summarizer interface {
    Summarize(ctx context.Context, m Module, static string) (chapters map[string]string, err error)
}
type IncrementalOptions struct { ProjectPath string; Full bool; Summarizer Summarizer }
type Result struct { Rebuilt, Removed, Failed, Skipped int; Duration time.Duration }
type Builder struct { /* store, logger, mutex per project */ }
func NewBuilder(store *Store, logger *slog.Logger) *Builder
func (b *Builder) Store() *Store
func (b *Builder) Check(projectID, projectPath string) (Diff, error)
func (b *Builder) BuildIncremental(ctx context.Context, projectID string, opts IncrementalOptions) (Result, error)
func HashAll(root string, mods []Module) (map[string]string, error)
func ApplyStale(books []Book, st State) []Book
func StaleCount(st State) int
```

Dipakai apa adanya dari tiket 01 (sudah ada di `internal/grimoire/`, jangan diubah): `Store` (`ValidateProject`, `Dir`, `BooksDir`, `BookPath`, `IndexPath`, `Exists`, `ReadState`, `WriteState`, `WriteFile`), `State`, `ModuleState{Hash, Sources, Stale}`, `Book`, `Chapter{Name, Path, Sources, Stale}`, `Module`, `Page{Path, Content}`, `Scan(root) ([]FileInfo, error)`, `GroupModules([]FileInfo) []Module`, `HashModule(root, m) (string, error)`, `RenderBooks`, `BuildIndex`, `IndexLine`, `Snake`, `ErrNotBuilt`, `ErrInvalidProject`.

Fakta kode nyata tiket 01 yang menentukan implementasi (sudah diverifikasi saat plan ditulis; periksa ulang bila kode berubah):

- Kunci `State.Modules` = `Snake(m.Name)` (lihat `build.go`); peta hash dari `build.go` berkunci `m.Name`. `Module.Name` sudah snake, tetapi selalu terapkan `Snake()` saat membuat kunci state/nama buku agar identik dengan `Build`.
- `RenderBooks(mods)` memanggil `renderBooks(mods, nil)` sehingga footer `## Hash` halaman berisi "(belum dihitung)". `Build` memakai `renderBooks(mods, hashes)` (tak-diekspor, paket sama) sehingga footer berisi 12 karakter hash. Agar halaman hasil inkremental identik dengan hasil `Build`, untuk MENULIS halaman modul pakai `renderBooks([]Module{m}, map[string]string{m.Name: hash})`. Untuk membangun index (tidak butuh footer) pakai `RenderBooks(semua modul)` sesuai overview. Catat penyimpangan kecil ini di komentar kode.
- `Store.WriteFile` sudah atomik (tmp+sync+rename) dan membuat direktori induk; `Store.WriteState` mengisi `ProjectID` bila kosong dan `BuiltAt` diisi pemanggil.
- `BuildIndex`/`IndexLine` menambahkan akhiran ` warning: stale, jalankan grimoire build` bila ada `Chapter.Stale`; jadi cukup `ApplyStale` sebelum `BuildIndex`.
- Tidak ada API hapus berkas/folder buku per modul di `Store` (hanya `RemoveBooks` semua buku). Penghapusan per buku dilakukan di `incremental.go` dengan `os.RemoveAll`/`os.Remove` pada `filepath.Join(store.BooksDir(id), nama)` setelah `nama` divalidasi (lihat langkah 6).

## Langkah

Kerjakan berurutan. Semua helper tak-diekspor diawali `diff*` (di `diff.go`) atau `incremental*` (di `incremental.go`); komentar dan galat bahasa Indonesia.

### 1. `diff.go` (timpa stub, hapus baris STUB)

Import: `sort`.

1. Definisikan tipe persis seperti kontrak (`ChangeKind` iota urutan Unchanged, Added, Modified, Removed; `ModuleChange`; `Diff`).
2. `ComputeDiff(scanned, hashes, st)` murni, tanpa I/O, tanpa mengubah argumen:
   - `old := map[string]ModuleState{}`; bila `st != nil && st.Modules != nil` pakai `st.Modules`.
   - `seen := map[string]bool{}`. Untuk tiap `m` di `scanned`: `name := Snake(m.Name)`; lewati bila `seen[name]` (duplikat); `newHash := hashes[m.Name]` (bila kunci tak ada, coba `hashes[name]`; bila tetap tak ada, `""`).
   - Tidak ada di `old` -> `Added{Name, Kind: ChangeAdded, NewHash}`.
   - Ada dan `old.Hash != newHash` -> `Modified{OldHash: old.Hash, NewHash}` (termasuk yang Stale, tidak ganda).
   - Ada, hash sama, `old.Stale` -> `Stale` dengan `Kind: ChangeUnchanged`, kedua hash terisi.
   - Ada, hash sama, tidak stale -> `Unchanged`.
   - Setelah loop: tiap kunci di `old` yang tidak ada di `seen` -> `Removed{Name, Kind: ChangeRemoved, OldHash}`.
   - Urutkan kelima slice menurut `Name` (`sort.Slice`) agar deterministik. Slice kosong boleh nil.
3. `(Diff) Pending()`: gabungkan `Added`, `Modified`, `Stale` (urutan kelompok itu; tiap kelompok sudah terurut nama) ke slice baru.
4. `(Diff) Empty()`: `len(Added)+len(Modified)+len(Removed)+len(Stale) == 0`.

### 2. `incremental.go`: tipe dan fungsi murni

Import: `context`, `errors`, `fmt`, `log/slog`, `os`, `path/filepath`, `sort`, `strings`, `sync`, `time`.

1. Definisikan `Summarizer`, `IncrementalOptions`, `Result`, `Builder` persis kontrak. Struct `Builder`: `store *Store`, `logger *slog.Logger`, `mu sync.Mutex`, `locks map[string]*sync.Mutex`.
2. `NewBuilder(store, logger)`: inisialisasi `locks`. `Store()` mengembalikan `b.store`.
3. `incrementalLock(projectID string) *sync.Mutex`: di bawah `b.mu`, buat-bila-belum-ada mutex per projectID. `BuildIncremental` memegang mutex itu selama proses; `Check` tidak mengambil lock (hanya baca).
4. `incrementalLogger()`: kembalikan `b.logger` atau `slog.Default()` bila nil (atau cek nil di tiap pemakaian; logger nullable).
5. `HashAll(root, mods)`: untuk tiap modul `HashModule(root, m)`; galat -> `fmt.Errorf("hash modul %s: %w", m.Name, err)`; hasil peta berkunci `m.Name`.
6. `ApplyStale(books, st)`: murni; buat salinan `[]Book` dan salinan `Chapters` tiap buku (jangan mutasi masukan); set `Chapter.Stale = st.Modules[book.Name].Stale` (buku tak ada di state -> false). Argumen `st.Modules` nil aman.
7. `StaleCount(st)`: hitung entri `st.Modules` dengan `Stale == true`.
8. `incrementalRoot(projectPath string) (string, error)`: sama dengan validasi `Build`: wajib absolut (`filepath.IsAbs`), `os.Stat` sukses dan direktori; galat `fmt.Errorf("path project tidak valid: %s", ...)`. Kembalikan `filepath.Clean`.
9. `incrementalScan(store, projectID, root) (mods []Module, hashes map[string]string, st *State, err error)`: `Scan(root)` -> `GroupModules` -> `HashAll`; `store.ReadState(projectID)`: `ErrNotBuilt` -> `st = nil`; galat lain dikembalikan (state rusak = galat, jangan menimpa diam-diam); sukses -> `&state`.

### 3. `Builder.Check`

`Check(projectID, projectPath)`: validasi store non-nil, `store.ValidateProject`, `incrementalRoot`, `incrementalScan`, lalu `ComputeDiff(mods, hashes, st)`. Hanya baca; tidak menulis apa pun, tidak mengambil lock. Dipakai `session-trigger` untuk memutuskan `Diff.Empty()`.

### 4. `Builder.BuildIncremental` (inti)

Signature kontrak. Alur:

1. Validasi: `b.store == nil` -> `errors.New("store grimoire kosong")`; `opts.ProjectPath == ""` -> `errors.New("projectPath wajib")`; `ValidateProject`; `incrementalRoot`; `ctx.Err() != nil` -> kembalikan `Result{}, ctx.Err()` sebelum apa pun.
2. Ambil lock per project (`defer Unlock`); catat `start := time.Now()`; `defer` isi `res.Duration`.
3. `mods, hashes, st, err := incrementalScan(...)`; `diff := ComputeDiff(mods, hashes, st)`.
4. Penentuan rencana (`todo []ModuleChange`):
   - `Full=false`: `todo = diff.Pending()`. Bila `diff.Empty()` -> kembalikan `Result{Skipped: len(diff.Unchanged)}, nil` TANPA menulis apa pun (tidak ada WriteFile/RemoveAll/WriteState, mtime tidak berubah).
   - `Full=true`: abaikan state; `todo` = SEMUA modul hasil scan (nama `Snake`, `NewHash` dari hashes, `OldHash` dari state bila ada), diperlakukan seperti Modified. Modul di state yang tidak ada di scan tetap `diff.Removed` dan tetap dihapus.
5. Siapkan `newMods := map[string]ModuleState` = salinan `st.Modules` (kosong bila `st == nil`). `byName := map[string]Module` dari `mods` (kunci `Snake(m.Name)`). Hitung `res.Skipped = len(diff.Unchanged)` (Full: 0). Sertakan juga `processed := map[string]bool` dan `changed := false`.
6. **Modul Pending**, urut sesuai `todo`. Sebelum tiap modul: `if ctx.Err() != nil { ctxErr = ctx.Err(); break }`. Per modul (fungsi helper `incrementalRebuildModule`):
   - `m := byName[name]`, `hash := hashes[m.Name]`, `_, existed := newMods[name]`.
   - Halaman statik: `_, pages := renderBooks([]Module{m}, map[string]string{m.Name: hash})`.
   - Bila `opts.Summarizer != nil`: bangun string `static` = isi semua `pages[i].Content` digabung `"\n\n"`; panggil `Summarize(ctx, m, static)` terbungkus `recover` (panic dianggap galat). Sukses -> untuk tiap `Page`, nama bab = `strings.TrimSuffix(filepath.Base(p.Path), ".md")`; bila `chapters[nama]` ada dan tidak kosong, ganti `Content`; kunci tak dikenal diabaikan. Galat (termasuk `ctx` dibatalkan di tengah) -> pakai halaman statik utuh, `summarizeFailed = true` (dicatat log WARN; bukan menggagalkan modul).
   - Tulis semua halaman lewat `store.WriteFile(projectID, p.Path, []byte(p.Content))` (atomik). Setelah SEMUA halaman sukses: hapus berkas `.md` lama di `books/<nama>/` yang tidak ada di himpunan halaman baru (`incrementalPruneBook`, `os.ReadDir` + `os.Remove`, hanya berkas `*.md`, abaikan galat hapus dengan log WARN). Ini menangani modul yang jumlah bab-nya berubah.
   - Sukses: `newMods[name] = ModuleState{Hash: hash, Sources: salinan m.Sources, Stale: summarizeFailed}`; `res.Rebuilt++`; `changed = true`; `processed[name] = true`.
   - Gagal tulis (galat `WriteFile`):
     - `existed == true`: halaman lama dipertahankan (penulisan atomik per berkas), `newMods[name]` = entri lama dengan `Stale = true` (hash lama tidak diubah); `res.Failed++`; `changed = true` (state perlu menyimpan stale); log galat; LANJUT ke modul berikutnya.
     - `existed == false` (modul baru): hapus sisa `books/<nama>/` (`incrementalRemoveBook`), JANGAN masukkan ke `newMods`; `res.Failed++`; log; lanjut. Modul ini otomatis muncul lagi sebagai Added pada build berikutnya.
7. **Modul Removed** (`diff.Removed`, urut nama; juga di jalur Full): cek `ctx.Err()` seperti di atas; `incrementalRemoveBook(name)` (lihat langkah 6 helper), `delete(newMods, name)`, `res.Removed++`, `changed = true`. Galat hapus -> log, entri state TETAP (dicoba lagi build berikutnya), tidak menghentikan modul lain.
8. **Index dan state** (hanya bila `changed || opts.Full`, kalau tidak jangan menulis apa pun):
   - `books, _ := RenderBooks(mods)` (semua modul hasil scan).
   - Saring: buang buku yang namanya tidak ada di `newMods` (modul baru yang gagal/belum diproses dikeluarkan dari index).
   - Untuk buku yang halamannya TIDAK sedang ditulis ulang pada build ini namun struktur bab bisa berbeda dari disk — yaitu modul Pending yang gagal (lama) dan modul Pending yang belum diproses karena pembatalan — gantikan `Chapters` dengan bab yang benar-benar ada di disk (`incrementalDiskChapters(name)`: `os.ReadDir(books/<nama>)`, berkas `*.md` urut nama, `Chapter{Name: tanpa .md, Path: "books/<nama>/<bab>.md", Sources: m.Sources}`); bila kosong/galat baca, pertahankan hasil `RenderBooks`. Tujuannya agar index tidak menunjuk bab yang tidak ada.
   - `newState := State{ProjectID: projectID, BuiltAt: time.Now().UTC(), Modules: newMods}`.
   - `books = ApplyStale(books, newState)`; `index := BuildIndex(books)`.
   - Urutan tulis: (halaman dan penghapusan sudah dilakukan di atas) lalu `store.WriteFile(projectID, "index.md", []byte(index))`, TERAKHIR `store.WriteState(projectID, newState)`. Galat tulis index/state dikembalikan: `fmt.Errorf("tulis index.md: %w", err)` / `fmt.Errorf("tulis state.json: %w", err)` (state tidak ditulis bila index gagal).
9. Pembatalan: bila loop berhenti karena `ctx`, tetap lanjut ke langkah 8 bila `changed` (state parsial konsisten: modul belum selesai tetap berhash lama / tidak ada di state), lalu kembalikan `res, ctxErr`. Bila belum ada perubahan sama sekali (`!changed`), jangan menulis index, kembalikan `Result{...}, ctx.Err()`. Pembatalan tidak dihitung `Failed`.
10. Kembalikan `res, nil` (galat per-modul tidak membuat fungsi galat; terlihat lewat `Failed` dan stale).

Helper tambahan (di `incremental.go`, awalan `incremental*`):

- `incrementalSafeBook(name string) bool`: `name != "" && name == Snake(name)` (hanya `[a-z0-9_]`, tanpa pemisah path) — wajib sebelum `RemoveAll`/`ReadDir`.
- `incrementalBookDir(store, projectID, name) string` = `filepath.Join(store.BooksDir(projectID), name)`.
- `incrementalRemoveBook(store, projectID, name) error`: tolak bila `!incrementalSafeBook`; `os.RemoveAll(incrementalBookDir(...))` (tidak ada = sukses).
- `incrementalPruneBook`, `incrementalDiskChapters`, `incrementalWrite`/`incrementalRebuildModule` sebagaimana di atas.

### 5. Verifikasi (guard)

1. `gofmt -l internal/grimoire` kosong; `go build ./...` lulus; `go vet ./internal/grimoire/...` bersih.
2. Tidak ada baris `STUB(foundation)` tersisa di `diff.go`/`incremental.go`.
3. `git status` hanya menunjukkan kedua berkas itu berubah oleh fitur ini; `git diff --stat -- go.mod go.sum` kosong.
4. Periksa manual (membaca kode): `ComputeDiff` tidak mengimpor `os`; tidak ada `time.Ticker`/goroutine/`fsnotify` di kedua berkas; tidak ada penulisan di dalam `ProjectPath`.

## Perubahan

- Diubah (menimpa stub foundation): `internal/grimoire/diff.go`
- Diubah (menimpa stub foundation): `internal/grimoire/incremental.go`
- Tidak ada berkas lain dibuat/diubah.

## Wiring

Fitur ini hanya mengekspos API paket; JANGAN menyambungkannya sendiri. Dipakai oleh langkah lain nanti:

- `session-trigger` (`trigger.go`) memakai `Builder.Check`, `Builder.BuildIncremental`, `Diff.Empty`, `StaleCount`, `ApplyStale`, `Result`.
- Wiring (overview bagian 5) membuat `NewBuilder(grimoire.NewStore(d.DataDir), logger)` di `wire_grimoire_refresh.go`, dan handler IPC di `ipc.go` memakai `RefreshRun`/`RefreshStaleCount`/`RefreshApplyStale` yang memanggil fungsi dari plan ini. `grimoire.build` tiket 01 tetap memakai `Build` penuh sampai wiring mengganti jalurnya.
- Pemanggil wajib mengisi `IncrementalOptions.ProjectPath` (path absolut project). `Summarizer` dibiarkan nil sampai tiket 4.
- Catatan untuk wiring: `Builder` hanya menjaga satu build per project lewat mutex (pemanggil kedua menunggu); single-flight "kembalikan building: true" dilakukan `Trigger`, bukan `Builder`.

## Selesai bila

- `diff.go` dan `incremental.go` mendefinisikan semua tipe/fungsi kontrak dengan signature identik overview 2.3; baris `STUB(foundation)` hilang dari keduanya.
- `ComputeDiff` murni (tanpa I/O): state nil/kosong -> semua Added; state sama -> semua Unchanged; satu hash beda -> satu Modified; modul hilang dari scan -> Removed; Stale dengan hash sama -> `Stale`, Stale dengan hash beda -> `Modified`; semua slice terurut nama; `Pending()` urut Added, Modified, Stale; `Empty()` benar.
- `BuildIncremental` dengan diff kosong dan `Full=false` tidak menulis apa pun (tanpa `WriteFile`/`RemoveAll`/`WriteState`) dan mengembalikan `Result{Skipped: n}`.
- Modul Added/Modified ditulis ulang utuh (footer hash sama dengan hasil `Build`), `Hash`/`Sources` state diperbarui, `Stale=false`; Removed menghapus `books/<nama>/` dan entri state; index dibangun dari `RenderBooks`+`ApplyStale`+`BuildIndex`; urutan tulis halaman -> hapus -> `index.md` -> `state.json`, semuanya lewat `Store` (atomik).
- Modul lama yang gagal: halaman dan hash lama dipertahankan, `Stale=true`, baris index berakhiran ` warning: stale, jalankan grimoire build`; modul baru yang gagal tidak ada di index maupun state; kegagalan satu modul tidak menghentikan modul lain.
- `Summarizer` nil = statik; galat/panic `Summarizer` jatuh ke statik dan modul `Stale=true`.
- `ctx` dihormati antar modul; pembatalan menyimpan state parsial konsisten dan mengembalikan `ctx.Err()`.
- `projectPath` kosong -> galat `projectPath wajib`; tidak ada penulisan di dalam repo proyek.
- Guard: `go build ./...` sukses (tanpa unit test), `gofmt -l internal/grimoire` kosong, `go.mod`/`go.sum` tidak berubah.
