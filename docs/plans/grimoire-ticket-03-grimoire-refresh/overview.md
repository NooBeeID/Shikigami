# Overview: grimoire-ticket-03-grimoire-refresh

Tiket: `docs/tickets/grimoire-ticket-03-grimoire-refresh.md`. Spec: `docs/superpowers/specs/2026-11-22-grimoire-design.md`.
Dokumen ini adalah sumber kebenaran BERSAMA untuk semua plan fitur. Bila ada perbedaan kecil dengan tiket, tiket menang kecuali dinyatakan "koreksi" di bagian 2.2. Tanpa dependency baru, tanpa unit test; guard hanya build.

Keadaan repo saat overview ditulis: `internal/grimoire/` dan `frontend/src/modules/grimoire/` belum ada di working tree (tiket 01 dan 02 baru berupa plan di `docs/plans/grimoire-ticket-01-grimoire-build/` dan `docs/plans/grimoire-ticket-02-grimoire-inject/`). Tiket 03 `depends_on` tiket 02, jadi saat dieksekusi kode tiket 01/02 SUDAH ada. Setiap agent wajib membaca kode nyata tiket 01/02 (`internal/grimoire/{types,store,scan,modules,hash,pages,index,build,ipc,inject,snapshot,measure}.go`, `cmd/shikigamid/wire_grimoire*.go`, `cmd/shikigamictl/cmd_grimoire.go`, `frontend/src/modules/grimoire/*`) dan memakai API-nya apa adanya. Nama/signature yang dikutip di bawah diambil dari overview tiket 01/02; bila kode nyata berbeda, ikuti kode nyata, jaga arti, dan catat di plan.

## 1. Ringkasan epic dan urutan eksekusi

Build tiket 01 selalu menulis ulang semua modul. Epic ini membuatnya inkremental: hash isi working tree per modul dibandingkan dengan `state.json`; hanya modul Added/Modified (dan yang `Stale`) ditulis ulang, modul Removed dihapus `books/<nama>/`-nya, modul Unchanged tidak disentuh. Sesi baru (provider selain `shell`, project yang sudah punya grimoire) memicu `Trigger.Kick` yang menjalankan build di goroutine latar belakang, single-flight per project, tanpa pernah menahan peluncuran. Sesi memakai snapshot tiket 02 yang dibekukan dari `index.md` saat itu (penulisan atomik menjamin `index.md` selalu utuh). Panel menandai bab/buku stale, menampilkan indikator "sedang build", dan tombol "Perbarui". Tanpa polling, timer, atau file watcher.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): stub berkompilasi dengan signature final untuk `internal/grimoire/diff.go` dan `internal/grimoire/incremental.go` (tipe `Diff`, `ModuleChange`, `ChangeKind`, `Summarizer`, `IncrementalOptions`, `Result`, `Builder`, fungsi `ComputeDiff`, `HashAll`, `ApplyStale`, dst). Setelah selesai `go build ./...` lulus. Tidak ada foundation frontend.
2. **Fitur paralel** (3 fitur, bagian 3): `diff-incremental`, `session-trigger`, `stale-ui`. Tiap agent menimpa penuh berkas ownedPaths-nya; signature TIDAK berubah. `session-trigger` hanya mengompilasi terhadap stub Foundation; `stale-ui` murni frontend (kontrak hanya IPC aditif).
3. **Wiring** (bagian 5): sambungan kecil ke berkas tiket 01 (`ipc.go`, `cmd_grimoire.go`, `GrimoirePanel.svelte`), verifikasi, uji manual.

## 2. Kontrak bersama

### 2.1 Paket, folder, penamaan

- Modul Go `github.com/NooBeeID/shikigami`; paket `internal/grimoire` (package `grimoire`) dari tiket 01. Berkas baru tiket ini: `diff.go`, `incremental.go`, `trigger.go` (di `internal/grimoire/`), `cmd/shikigamid/wire_grimoire_refresh.go`, dan folder `frontend/src/modules/grimoire/stale/`.
- Pesan galat, log, dan komentar bahasa Indonesia; identifier bahasa Inggris. Logger `*slog.Logger` nullable (cek nil).
- Helper tak-diekspor diberi awalan berkas (`diff*`, `incremental*`, `trigger*`; di package `main`: `refresh*`) agar tidak bentrok dengan berkas lain (tiket 01/02 dan `wire_*.go`). Konstanta/helper didefinisikan hanya di berkas pemiliknya.
- Daemon: `RegisterModule("grimoire-refresh", ...)` dari `init()` di `cmd/shikigamid/wire_grimoire_refresh.go`. Modul dipasang alfabetis (`grimoire`, `grimoire-inject`, `grimoire-refresh`); layanan lain diambil lewat `d.OnReady` + `d.Lookup`, bukan saat `RegisterModule`.
- Nama buku = nama modul (`Module.Name`, snake_case) = kunci `State.Modules` (konvensi tiket 01).

### 2.2 Koreksi atas tiket (WAJIB diikuti)

1. **Hash tidak ada di `Module`.** Overview tiket 01 mendefinisikan `Module` tanpa field hash; hash dihitung `HashModule(root, m)` (I/O). Maka `ComputeDiff` tetap murni dengan menerima peta hash yang sudah dihitung: `ComputeDiff(scanned []Module, hashes map[string]string, st *State) Diff`. Fungsi I/O terpisah `HashAll(root string, mods []Module) (map[string]string, error)` (di `incremental.go`) memanggil `HashModule` per modul. Bila kode nyata tiket 01 ternyata punya field `Hash` pada `Module`, tetap pakai peta `hashes` sebagai satu-satunya sumber di epic ini.
2. **`BuildIncremental` butuh path project.** Signature tiket tidak membawa `projectPath`. Ditambahkan field wajib `IncrementalOptions.ProjectPath string`. Kosong -> galat `errors.New("projectPath wajib")`.
3. **`Builder` belum ada di tiket 01** (tiket 01 hanya punya fungsi bebas `Build(store, projectID, projectPath)`). `Builder` didefinisikan di `incremental.go` (milik `diff-incremental`): `NewBuilder(store *Store, logger *slog.Logger) *Builder`. Wiring membuat `grimoire.NewStore(d.DataDir)` sendiri (tiket 01 tidak `Provide` store), jadi tidak ada ketergantungan pada layanan tiket 01.
4. **Konteks daemon tidak tersedia** (`Deps` tidak memberi context/hook berhenti, `main.go` dilarang disentuh). `Trigger` memegang context akar sendiri (`context.WithCancel(context.Background())`) dan menyediakan `Close()` yang membatalkannya; semua goroutine build turunan dari context itu. Tanpa hook berhenti, goroutine berakhir saat proses daemon keluar; tidak ada data rusak karena semua tulisan atomik. Ini tafsir "dibatalkan saat daemon berhenti" yang bisa dipenuhi tanpa mengubah `main.go`.
5. **Handler IPC tidak bisa ditimpa dari luar** (`ipc.Server.Register` panik bila method terdaftar dua kali dan `internal/ipc` dilarang disentuh). Maka `RegisterRefresh` hanya menyimpan Trigger/Builder aktif di variabel paket (tidak mendaftarkan method IPC), dan Wiring mengubah handler `grimoire.build`/`status`/`list` di `internal/grimoire/ipc.go` (berkas tiket 01, pengecualian yang sama dengan tiket 02) agar memanggil fungsi helper di bagian 2.3. Pengecualian tiket yang menyebut `wire_grimoire.go` ditafsirkan sebagai berkas tempat handler benar-benar hidup; periksa dulu mana yang berisi handler (kemungkinan `ipc.go`).
6. **Single-flight satu pintu.** Semua jalur build (pemicu sesi, IPC `grimoire.build`, CLI) lewat `Trigger.Run`/`Kick` dengan peta single-flight yang sama. `Builder` tambahan menjaga dengan mutex per project agar dua pemanggil langsung tidak menulis bersamaan. `Build` penuh tiket 01 tidak lagi dipanggil dari IPC setelah Wiring (diganti `BuildIncremental{Full}`); fungsinya tetap ada dan tidak diubah.

### 2.3 Signature Go bersama (final; stub di Foundation, implementasi oleh pemilik)

Dari tiket 01 (JANGAN diubah): `Store` (`NewStore`, `Dir`, `ValidateProject`, `IndexPath`, `StatePath`, `BooksDir`, `BookPath`, `WriteFile`, `ReadFile`, `Exists`, `ReadState`, `WriteState`, `RemoveBooks`), `Snake`, `CamelTag`, `State`, `ModuleState{Hash, Sources, Stale}`, `Book`, `Chapter{Name, Path, Sources, Stale}`, `Summary`, `Module`, `FileInfo`, `Scan`, `GroupModules`, `HashModule`, `Page{Path, Content}`, `RenderChapter`, `RenderBooks(mods []Module) ([]Book, []Page)`, `IndexLine`, `BuildIndex(books []Book) string`, `IndexMaxBytes`, `Build`, `ErrNotBuilt`, `ErrInvalidProject`, `Deps`/`Register` (ipc.go). Dari tiket 02 hanya perlu diingat: `ComposeInjection` membaca `index.md` sekali per sesi (tidak diubah).

`diff.go` (milik `diff-incremental`):

```go
type ChangeKind int
const (
    ChangeUnchanged ChangeKind = iota
    ChangeAdded
    ChangeModified
    ChangeRemoved
)
type ModuleChange struct {
    Name             string
    Kind             ChangeKind
    OldHash, NewHash string
}
type Diff struct {
    Added, Modified, Removed, Unchanged []ModuleChange
    Stale []ModuleChange // Unchanged-by-hash tetapi State.Modules[x].Stale == true; Kind tetap ChangeUnchanged
}
func ComputeDiff(scanned []Module, hashes map[string]string, st *State) Diff // murni, tanpa I/O; st nil = state kosong (semua Added)
func (d Diff) Pending() []ModuleChange // Added + Modified + Stale (urut: Added, Modified, Stale; tiap kelompok urut nama)
func (d Diff) Empty() bool             // tidak ada Added/Modified/Removed dan tidak ada Stale
```

Modul `Stale` yang hash-nya juga berubah masuk `Modified` (bukan `Stale`) agar tidak ganda. `Unchanged` hanya berisi yang tidak stale. Semua slice diurutkan menurut nama (deterministik).

`incremental.go` (milik `diff-incremental`):

```go
type Summarizer interface { // hook tiket 4; nil = statik
    Summarize(ctx context.Context, m Module, static string) (chapters map[string]string, err error)
}
type IncrementalOptions struct {
    ProjectPath string      // wajib (koreksi 2.2.2)
    Full        bool        // true = abaikan state, semua modul diperlakukan Modified
    Summarizer  Summarizer  // opsional
}
type Result struct {
    Rebuilt, Removed, Failed, Skipped int
    Duration                          time.Duration
}
type Builder struct{ /* store, logger, mutex per project */ }
func NewBuilder(store *Store, logger *slog.Logger) *Builder
func (b *Builder) Store() *Store
func (b *Builder) Check(projectID, projectPath string) (Diff, error) // Scan + GroupModules + HashAll + ComputeDiff; hanya baca; tanpa lock
func (b *Builder) BuildIncremental(ctx context.Context, projectID string, opts IncrementalOptions) (Result, error)
func HashAll(root string, mods []Module) (map[string]string, error)
func ApplyStale(books []Book, st State) []Book // murni: set Chapter.Stale = st.Modules[book.Name].Stale
func StaleCount(st State) int                  // jumlah modul dengan Stale == true
```

Keluaran `Summarize`: kunci = nama bab snake (tanpa `.md`), nilai = isi bab. Perilaku `BuildIncremental` mengikuti bagian "Perilaku kontrak" tiket persis, ditambah aturan berikut:

- Untuk membangun `index.md`: `books, _ := RenderBooks(semua modul hasil scan)` (murni), lalu `ApplyStale(books, stateBaru)`, lalu `BuildIndex`. Halaman hanya ditulis untuk modul Pending, memakai `Page` dari `RenderBooks([]Module{m})` (statik) atau hasil `Summarizer`.
- Modul baru (tidak ada di state lama) yang gagal diproses: tidak punya halaman lama; buku itu DIKELUARKAN dari `index.md` dan tidak dimasukkan ke state (dicoba lagi pada build berikut karena akan muncul lagi sebagai Added). Dihitung `Failed`.
- Modul lama yang gagal: halaman dan hash lama dipertahankan, `Stale=true`, baris index berakhiran ` warning: stale, jalankan grimoire build` (dari `IndexLine`).
- Urutan tulis: halaman modul, hapus modul Removed, `index.md`, terakhir `state.json` (atomik semua lewat `Store.WriteFile`/`WriteState`). Pembatalan context: berhenti di antara modul; bila ada modul yang sudah ditulis tetap lanjutkan ke `index.md` dan `state.json` parsial yang konsisten (modul belum selesai tetap hash lama), kembalikan `ctx.Err()`; bila tidak ada yang berubah, jangan menulis index.
- `Diff.Empty()` dan `Full=false`: kembalikan `Result{}` tanpa menulis apa pun.
- `BuildIncremental` mengambil lock per project; tanpa state (`ErrNotBuilt`) = semua Added.

`trigger.go` (milik `session-trigger`):

```go
type Trigger struct{ /* single-flight per projectId, context akar, publish */ }
func NewTrigger(b *Builder, publish func(event string, payload any)) *Trigger // publish nullable; payload event: map[string]any{"projectId": id}
func (t *Trigger) Kick(projectID, projectPath string)                  // non-blocking; tak ada grimoire -> no-op
func (t *Trigger) Run(ctx context.Context, projectID, projectPath string, full bool) (res Result, started bool, err error) // sinkron untuk IPC/CLI; started=false bila sudah building
func (t *Trigger) Building(projectID string) bool
func (t *Trigger) Close()                                              // batalkan context akar

type RefreshDeps struct {
    Builder *Builder
    Trigger *Trigger
}
func RegisterRefresh(d RefreshDeps) // menyimpan pasangan aktif di variabel paket (atomic); panggilan ulang menimpa

// Helper yang dipanggil handler IPC (Wiring). Aman dipanggil sebelum RegisterRefresh (kembali nilai nol / ok=false).
func RefreshActive() bool
func RefreshRun(ctx context.Context, projectID, projectPath string, full bool) (Result, bool, error) // = Trigger.Run; galat bila belum terdaftar
func RefreshBuilding(projectID string) bool
func RefreshStaleCount(projectID string) int // StaleCount(state) atau 0 bila tak ada state
func RefreshApplyStale(projectID string, books []Book) []Book // ApplyStale dengan state project; state tak ada = books apa adanya
```

Semantik `Kick`: (1) `store.Exists(projectID)` false atau `ValidateProject` gagal -> return tanpa efek; (2) bila projectID sudah ada di peta building -> return; (3) tandai building, publish `grimoire.changed`, jalankan goroutine: `Check`; bila `Diff.Empty()` -> tidak menulis, bersihkan building, publish `grimoire.changed` sekali; selain itu `BuildIncremental(ctx akar, ...)`, bersihkan building (via `defer`, juga saat panic/galat), publish `grimoire.changed`. Galat hanya di-log. `Run` memakai semantik sama tetapi sinkron dan dengan `ctx` pemanggil (digabung dengan context akar); `full=true` melewati `Check`. `Run` pada project tanpa state (belum pernah build) diperbolehkan (Generate lewat IPC/CLI); hanya `Kick` yang tidak memicu pembuatan pertama.

### 2.4 IPC (aditif; arti field lama tidak berubah)

| Method | Params | Hasil (tambahan epic ini ditebalkan) |
|---|---|---|
| `grimoire.build` | `{projectId, full?}` | `{books, pages, indexBytes}` (tiket 01) **plus `rebuilt`, `removed`, `failed`** dan `building: true` bila build project itu sedang berjalan (tanpa memulai yang kedua; field lain nol) |
| `grimoire.status` | `{projectId}` | field tiket 01/02 plus **`staleCount`** (jumlah modul stale, bukan lagi 0) dan **`building`** |
| `grimoire.list` | `{projectId}` | `[{name, tags, chapters:[{name, path, stale}]}]`; `stale` = modul induk stale |

- Event `grimoire.changed {projectId}` (topik/payload tiket 01) terbit saat build mulai dan selesai (sukses/sebagian gagal/no-op). Tanpa event untuk project lain. Tidak ada event baru.
- `grimoire.build` memanggil `RefreshRun(ctx, projectID, path, full)`; `books/pages/indexBytes` untuk hasil diambil dari store setelah build (hitung dari `index.md`/state seperti handler tiket 01 menghitung status). Bila `RefreshActive()` false (modul belum terpasang), handler jatuh ke `Build` penuh tiket 01 (perilaku lama).
- Pemetaan galat tiket 01 tidak berubah.

### 2.5 CLI

`shikigamictl grimoire build [--project <id>] [--full]` mengirim `grimoire.build {projectId, full}` (`full` hanya dikirim bila flag dipakai atau selalu boolean; keduanya diterima). `grimoire status` mencetak JSON mentah yang kini memuat `staleCount` dan `building`; ringkasan stderr tiket 02 ditambah `stale: N, building: ya|tidak`.

### 2.6 Frontend (`frontend/src/modules/grimoire/stale/`)

Impor: `call`, `onEvent` dari `../../../lib/ipc`; `activeProjectId` dari `../../../lib/stores` (bila nama aktual berbeda, ikuti kode tiket 01 panel). Tipe `GrimoireBook`/`GrimoireStatus` dari `../client` (tiket 01), tetapi komponen TIDAK mengedit `client.ts`: `staleStore.ts` memanggil `call('grimoire.status', {projectId})`, `call('grimoire.list', {projectId})`, `call('grimoire.build', {projectId})` langsung. Field yang belum ada diperlakukan `false`/`0`. Svelte 5 runes, token tema yang ada (`--bg`, `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--danger`; pakai yang sudah dipakai `GrimoirePanel.svelte`/`laws/`), tanpa token global baru, tanpa `setInterval`/`setTimeout`/polling, tanpa warna keras.

```ts
// staleStore.ts (tanpa efek saat impor modul)
export interface StaleState { building: boolean; staleCount: number; staleChapters: Set<string> /* path bab stale */; staleBooks: Set<string> /* nama buku yang memuat bab stale */; error: string | null }
export const staleState: Readable<StaleState>
export function startStaleStore(projectId: string): () => void   // muat awal + langganan 'grimoire.changed' hanya untuk projectId itu; kembalikan fungsi berhenti; project lain diabaikan
export function runUpdate(projectId: string): Promise<void>      // call('grimoire.build', {projectId}) tanpa 'full'; galat -> state.error
export function isChapterStale(s: StaleState, path: string): boolean
export function isBookStale(s: StaleState, book: string): boolean
```

Komponen (folder `stale/`):
- `StaleBadge.svelte` props `{ label?: string; title?: string }`: titik/label "stale" kecil.
- `BuildIndicator.svelte` props `{ building: boolean }`: teks "Sedang membangun..." + animasi CSS (keyframes), tidak merender apa pun bila `building` false.
- `UpdateButton.svelte` props `{ projectId: string; building: boolean; staleCount: number; onupdate?: () => void }`: tombol "Perbarui" (+ "(N stale)" bila `staleCount > 0`); nonaktif dan berlabel "Membangun..." saat `building`.
- `StaleSlot.svelte` props `{ projectId: string | null; exists: boolean }`: komponen header tunggal yang ditanam panel: kosong bila `exists` false atau `projectId` null; selain itu menjalankan `startStaleStore(projectId)` lewat `$effect` (berhenti saat project berganti/unmount) dan merender `BuildIndicator` + `UpdateButton`. Ekspor juga `staleState` (dari store) agar panel memakai `isChapterStale/isBookStale` untuk `StaleBadge` di barisnya.

Catatan: karena `GrimoirePanel.svelte` sudah punya store sendiri (tiket 01) yang menyegarkan daftar lewat `grimoire.changed`, `chapters[].stale` dari `grimoire.list` panel dapat langsung dipakai untuk badge baris; store stale hanya dibutuhkan untuk `building`/`staleCount` dan set buku stale. Agent `stale-ui` boleh menyediakan keduanya.

## 3. Daftar fitur

Tiga fitur persis dari tiket (tidak ditambah/digabung). Slug dipakai sebagai nama berkas plan `<slug>.md`.

### diff-incremental: Diff hash dan build inkremental
Scope: `ComputeDiff`/`Diff.Pending`/`Diff.Empty`/`ChangeKind`/`ModuleChange` (murni), `HashAll`, `ApplyStale`, `StaleCount`, `Builder` (`NewBuilder`, `Store`, `Check`, `BuildIncremental` dengan lock per project, hook `Summarizer`, penghapusan modul Removed, index dan state atomik, penanganan modul gagal = stale, `Full`, pembatalan context).
ownedPaths: `internal/grimoire/diff.go`, `internal/grimoire/incremental.go`

### session-trigger: Pemicu build saat sesi baru
Scope: `Trigger` (`NewTrigger`, `Kick`, `Run`, `Building`, `Close`), `RegisterRefresh` dan helper `Refresh*` (bagian 2.3), modul daemon `grimoire-refresh` dengan `provider.RegisterLaunchContribution` yang hanya memanggil `Kick` (tanpa mengubah `Args`, provider `shell` dilewati, project tanpa grimoire tidak dipicu, tanpa menunggu).
ownedPaths: `internal/grimoire/trigger.go`, `cmd/shikigamid/wire_grimoire_refresh.go`

### stale-ui: Penanda stale dan progres di panel
Scope: `StaleBadge.svelte`, `BuildIndicator.svelte`, `UpdateButton.svelte`, `StaleSlot.svelte`, `staleStore.ts` (bagian 2.6). Tidak menyentuh `GrimoirePanel.svelte` (Wiring).
ownedPaths: `frontend/src/modules/grimoire/stale/`

Dependensi kompilasi: `session-trigger` memakai `Builder`, `Diff`, `Result`, `BuildIncremental`, `Check`, `StaleCount`, `ApplyStale` dari stub Foundation. `stale-ui` tidak bergantung pada fitur Go.

## 4. Foundation

Ada (hasFoundation = true); rinci di `00-foundation.md`. Isi singkat:
- Prasyarat: tiket 01 dan 02 sudah ter-merge (`internal/grimoire/store.go` bukan stub, `ipc.go` punya `Register`, `inject.go` ada). Bila tidak, berhenti dan laporkan.
- Stub Go berkompilasi (baris pertama `// STUB(foundation): ditimpa oleh diff-incremental`) dengan signature final bagian 2.3 untuk `diff.go` dan `incremental.go`, termasuk semua tipe/konstanta.
- Bukan milik Foundation: `trigger.go` (tak ada fitur lain yang bergantung padanya saat kompilasi), apa pun di `cmd/`, frontend, dan berkas tiket 01/02.

## 5. Wiring checklist

Dikerjakan agen wiring setelah semua fitur selesai. Perubahan pada berkas tiket 01 seminimal mungkin, aditif, dan dicatat di ringkasan.

1. Tidak ada stub tersisa: `grep -rn "STUB(foundation)" internal/grimoire cmd frontend/src` kosong (hanya yang berasal dari epic ini; stub tiket 01/02 sudah hilang).
2. **Daemon**: `cmd/shikigamid/wire_grimoire_refresh.go` memanggil `RegisterModule("grimoire-refresh", ...)` dari `init()` (nama unik, bukan `grimoire`/`grimoire-inject`). Di `d.OnReady`: buat `grimoire.NewStore(d.DataDir)`, `grimoire.NewBuilder`, `grimoire.NewTrigger(b, srv.Publish)`, panggil `grimoire.RegisterRefresh(grimoire.RefreshDeps{Builder, Trigger})`, dan pasang `provider.RegisterLaunchContribution` tepat sekali (`sync.Once`) yang memanggil `Trigger.Kick` tanpa mengubah `LaunchContext.Args` dan tanpa menunggu. `modules.go`, `main.go`, `wire_grimoire.go`, `wire_grimoire_inject.go`, `wire_workspace.go` tidak berubah (kecuali satu baris pengecualian pada butir 3 bila handler hidup di `wire_grimoire.go`).
3. **IPC** (`internal/grimoire/ipc.go`, tiket 01): (a) `grimoire.build`: baca `full` (bool opsional) dari params; bila `RefreshActive()` panggil `RefreshRun(ctx, id, path, full)`; bila `started == false` kembalikan `{building: true}` tanpa galat; selain itu isi `rebuilt/removed/failed` dari `Result` plus `books/pages/indexBytes` seperti sebelumnya; bila tidak aktif, jalur lama `Build`. Hapus single-flight lokal tiket 01 hanya bila redundan; jangan sampai event `grimoire.changed` terbit ganda (Trigger sudah publish: bila handler tiket 01 juga publish, hilangkan salah satunya, pertahankan publish Trigger bila `RefreshActive()`). (b) `grimoire.status`: set `staleCount = RefreshStaleCount(id)` dan `building = RefreshBuilding(id)` (menggantikan nilai sementara 0/false tiket 01). (c) `grimoire.list`: bungkus hasil dengan `RefreshApplyStale(id, books)` sebelum dikembalikan. Hanya menambah field.
4. **CLI** (`cmd/shikigamictl/cmd_grimoire.go`, tiket 01/02): tambah flag `--full` (bool) pada `grimoire build`, kirim sebagai `full` di payload; cabang `status` menambah ke ringkasan stderr tiket 02 `stale: N, building: ya|tidak` dari `staleCount`/`building`. Tidak ada subcommand baru.
5. **Panel** (`frontend/src/modules/grimoire/GrimoirePanel.svelte`, tiket 01/02): ganti komentar `<!-- slot: stale/llm (tiket 3, 4) -->` dengan `<StaleSlot projectId={...} exists={...} />` (nama variabel state nyata panel; pertahankan bagian "llm" tiket 4 sebagai komentar baru `<!-- slot: llm (tiket 4) -->`), impor `./stale/StaleSlot.svelte`, `./stale/StaleBadge.svelte`; render `StaleBadge` pada baris bab bila `chapter.stale` dan pada header buku bila ada bab stale. Tombol Generate tiket 01 dan slot laws tiket 02 tidak diubah. Tanpa polling; refresh lewat `grimoire.changed`.
6. Konsistensi: nama method/field IPC di `staleStore.ts` (`grimoire.build`, `grimoire.status`, `grimoire.list`, `building`, `staleCount`, `chapters[].stale`) identik dengan yang dikirim handler; `ComputeDiff` dipanggil dengan peta hash (bukan field `Module`); tidak ada `time.Ticker`/`setInterval`/watcher (`grep -rnE "time\.(NewTicker|Tick)|setInterval|fsnotify" internal/grimoire cmd/shikigamid/wire_grimoire_refresh.go frontend/src/modules/grimoire` kosong); tidak ada perubahan di `internal/session`, `internal/provider`, `internal/ipc`, `go.mod`, `go.sum`, `frontend/package.json`/lockfile (`git diff --stat` untuk berkas itu kosong).
7. Jalankan build guard (bagian 6). Uji manual bila lingkungan memungkinkan: `make build`, jalankan aplikasi, Generate grimoire di sebuah project, edit satu file modul (tanpa commit), buat sesi baru (peluncuran tidak tertunda), amati indikator build lalu pembaruan bab di panel; hapus satu folder modul dan pastikan halaman/entri index hilang; jalankan `shikigamictl grimoire build` kedua tanpa perubahan dan pastikan tidak ada penulisan (mtime `index.md` tidak berubah); `--full` menulis ulang semua; `shikigamictl grimoire status --project <id>` menampilkan `staleCount` dan `building`.

## 6. Build guard

Satu-satunya guard; tanpa unit test:
- `go build ./...` (opsional `go vet ./internal/grimoire/... ./cmd/...` bila cepat).
- Frontend: `cd frontend && npm run check && npm run build` (bila skrip `check` ada) atau `make build-ui`. Penutup epic: `make build`.
- Tanpa dependency baru: `git diff --stat -- go.mod go.sum frontend/package.json frontend/package-lock.json` kosong.

Selama fase paralel: fitur Go menjalankan `go build ./...`; `stale-ui` menjalankan build frontend. Bila build gagal karena berkas fitur lain (bukan miliknya), jangan memperbaikinya; laporkan.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3) dan menulis plan/progress sesuai instruksi workflow. Jangan menyentuh path fitur lain, berkas Foundation selain yang dimilikinya, berkas tiket 01/02, atau berkas wiring (`ipc.go`, `wire_grimoire.go`, `wire_grimoire_inject.go`, `cmd_grimoire.go`, `GrimoirePanel.svelte`, `client.ts`).
- Menimpa stub Foundation milik fiturmu wajib (hapus baris `// STUB(foundation)`); jangan mengubah signature bagian 2.3/2.6. Bila signature terasa salah, catat di plan dan tetap ikuti kontrak.
- Dilarang: `internal/session`, `internal/provider`, `internal/ipc`, `cmd/shikigamid/main.go`, `modules.go`, `wire_workspace.go`, `cmd/shikigamictl/main.go`, `registry.go`, `go.mod`/`go.sum`, `package.json`/lockfile, `AgentSidebar`, inti `Build`/`Scan`/`Store` tiket 01, `ComposeInjection`/snapshot tiket 02.
- Tidak ada definisi ganda: tipe diff hanya di `diff.go`; `Builder`/`Result`/`Summarizer`/`IncrementalOptions` hanya di `incremental.go`; `Trigger`/`RefreshDeps`/`Refresh*` hanya di `trigger.go`; helper tak-diekspor memakai awalan berkas.
- Larangan perilaku: tanpa polling/timer/watcher; kontribusi launch tidak boleh menunggu, memblokir, atau menggagalkan peluncuran dan tidak mengubah `LaunchContext.Args`; tidak menulis apa pun di dalam repo proyek; tidak memperbarui snapshot sesi yang berjalan.
- Tidak membuat berkas tiruan di luar ownedPaths.
