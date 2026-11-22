# Overview: grimoire-ticket-01-grimoire-build

Tiket: `docs/tickets/grimoire-ticket-01-grimoire-build.md`. Spec: `docs/superpowers/specs/2026-11-22-grimoire-design.md`.
Dokumen ini adalah sumber kebenaran BERSAMA untuk semua plan fitur. Bila ada perbedaan kecil dengan tiket, tiket menang kecuali dinyatakan "koreksi" di bawah. Tanpa dependency baru, tanpa unit test; guard hanya build.

## 1. Ringkasan epic dan urutan eksekusi

Daemon memindai `Project.Path`, mengelompokkannya jadi modul, menulis halaman statik (baris `Sumber:`, tanpa LLM) dan `index.md` ke `<DataDir>/grimoire/<projectId>/`. Panel **GRIMOIRE** di sidebar kiri menampilkan tombol Generate atau daftar buku/bab; klik bab membuka pane `file` read-only berroot direktori grimoire. CLI `shikigamictl grimoire build|status`. Tidak ada tulis ke dalam repo proyek.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): menulis `types.go` final dan stub berkompilasi (signature final) untuk semua berkas `internal/grimoire/*` yang akan dimiliki fitur lain, plus stub `openPage.ts`. Setelah selesai, `go build ./...` dan build frontend lulus dengan stub.
2. **Fitur paralel** (5 fitur, lihat bagian 3): tiap agent menimpa penuh berkas ownedPaths-nya (stub diganti implementasi nyata, signature TIDAK berubah). Semua fitur dapat dikerjakan paralel karena hanya bergantung pada signature bagian 2 yang sudah ada sebagai stub.
3. **Wiring** (bagian 5): verifikasi akhir, tidak ada stub tersisa, semua titik registrasi terhubung, uji manual.

Dependensi antar fitur (hanya kompilasi, semuanya terpenuhi oleh stub Foundation):
- `pages-index` memakai `Module` (scan-modules), `Store`/`Snake` (contract-types).
- `ipc-ctl` memakai `Build`, `Store`, tipe.
- `panel-ui` memakai `openGrimoirePage` (readonly-pane) dan nama method IPC.
- `readonly-pane`/`openPage.ts` membutuhkan field `dir` dari `grimoire.status` (diisi `ipc-ctl`); bila `dir` tak ada, `openPage.ts` jatuh ke galat yang jelas.

## 2. Kontrak bersama

### 2.1 Paket, folder, penamaan

- Modul Go: `github.com/NooBeeID/shikigami`. Paket baru: `github.com/NooBeeID/shikigami/internal/grimoire` (package `grimoire`).
- Folder data: `<DataDir>/grimoire/<projectId>/{index.md, state.json, books/<buku_snake>/<bab_snake>.md}`; `laws.md` dan `settings.json` BUKAN dibuat di tiket ini (hanya helper path `LawsPath`).
- Nama buku/bab snake_case (`Snake`), tag satu token camelCase (`CamelTag`). Path di `Chapter.Path` selalu memakai `/` dan relatif ke direktori grimoire project: `books/<buku>/<bab>.md`.
- Pesan galat dan komentar dalam bahasa Indonesia; identifier Inggris. Pola logging `*slog.Logger` nullable (cek nil).
- Daemon: `RegisterModule("grimoire", ...)` dari `init()` di `cmd/shikigamid/wire_grimoire.go`. CLI: `RegisterCommand` dari `init()` di `cmd/shikigamictl/cmd_grimoire.go`. Frontend: `registerPanel` dari `modules/grimoire/index.ts` (dimuat otomatis oleh `import.meta.glob`).

### 2.2 Tipe Go (`internal/grimoire/types.go`, final; ditulis Foundation)

Persis seperti Kontrak tiket:

```go
type Chapter struct {
    Name    string   `json:"name"`
    Path    string   `json:"path"`
    Sources []string `json:"sources"`
    Stale   bool     `json:"stale"`
}
type Book struct {
    Name     string    `json:"name"`
    Tags     []string  `json:"tags"`
    Chapters []Chapter `json:"chapters"`
}
type ModuleState struct {
    Hash    string   `json:"hash"`
    Sources []string `json:"sources"`
    Stale   bool     `json:"stale,omitempty"`
}
type State struct {
    ProjectID string                 `json:"projectId"`
    BuiltAt   time.Time              `json:"builtAt"`
    Modules   map[string]ModuleState `json:"modules"`
}
type Summary struct {
    Books      int `json:"books"`
    Pages      int `json:"pages"`
    IndexBytes int `json:"indexBytes"`
}
var (
    ErrNotBuilt       = errors.New("grimoire belum dibangun")
    ErrInvalidPath    = errors.New("path grimoire tidak valid")
    ErrInvalidProject = errors.New("projectId tidak valid")
)
```

(Tag JSON pada `Summary` adalah penegasan agar hasil `grimoire.build` bernama `{books, pages, indexBytes}`.)

### 2.3 Tipe dan signature bersama di paket `grimoire` (final; stub di Foundation, implementasi oleh pemilik)

Store (`store.go`, milik `contract-types`):

```go
type Store struct{ dataDir string }
func NewStore(dataDir string) *Store
func (s *Store) Dir(projectID string) string            // <dataDir>/grimoire/<projectID>; TIDAK memvalidasi
func (s *Store) ValidateProject(projectID string) error // ErrInvalidProject: kosong, mengandung "/", "\\", "..", atau NUL
func (s *Store) IndexPath(projectID string) string
func (s *Store) LawsPath(projectID string) string
func (s *Store) StatePath(projectID string) string
func (s *Store) BooksDir(projectID string) string
func (s *Store) BookPath(projectID, book, chapter string) string // books/<book>/<chapter>.md absolut (dua argumen sudah snake)
func (s *Store) WriteFile(projectID, rel string, data []byte) error // atomik, 0700/0600, lewat fsx.ResolveUnder; ErrInvalidPath/ErrInvalidProject
func (s *Store) ReadFile(projectID, rel string) ([]byte, error)     // batas 4 MiB; ErrInvalidPath; os.ErrNotExist dibungkus bila tak ada
func (s *Store) Exists(projectID string) bool                        // index.md ada
func (s *Store) ReadState(projectID string) (State, error)           // ErrNotBuilt bila state.json tak ada
func (s *Store) WriteState(projectID string, st State) error
func (s *Store) RemoveBooks(projectID string) error
func Snake(s string) string
func CamelTag(s string) string
```

Aturan galat store: galat "berkas tidak ada" HARUS dapat dideteksi dengan `errors.Is(err, os.ErrNotExist)` (bungkus dengan `%w`). `Snake("Session PTY") == "session_pty"`, `CamelTag("session_pty") == "sessionPty"`.

Scan/kelompok/hash (milik `scan-modules`):

```go
type FileInfo struct {
    Path   string // relatif repo, pemisah "/"
    Size   int64
    Binary bool   // NUL pada 8 KiB awal
    Large  bool   // > maxFileBytes; tidak dibaca untuk simbol
}
type Module struct {
    Name    string     // unik, snake_case
    Dir     string     // relatif repo ("." untuk root)
    Kind    string     // "go" | "web" | "generic"
    Files   []FileInfo // urut path; bisa lebih banyak dari batas simbol
    Symbols []string   // ringkas, maks 60
    Sources []string   // path relatif repo, urut leksikal, "/"
    Tag     string     // satu token camelCase
    Truncated int      // jumlah berkas yang simbolnya dipotong (0 bila tidak ada)
}
func Scan(root string) ([]FileInfo, error)
func GroupModules(files []FileInfo) []Module
func HashModule(root string, m Module) (string, error)
```

Konstanta (di scan.go/modules.go): `maxFileBytes = 1 << 20`, `maxModuleFiles = 200`, `maxModuleBytes = 512 << 10`, `maxSymbols = 60`.

Penulis halaman dan index (milik `pages-index`):

```go
type Page struct {
    Path    string // relatif direktori grimoire: books/<buku>/<bab>.md
    Content string
}
const IndexMaxBytes = 16384
const IndexHeader = "..."                                     // konstanta teks tetap, lihat 2.5
func RenderChapter(m Module) string                           // baris pertama "Sumber: ..."
func RenderBooks(mods []Module) ([]Book, []Page)              // buku+bab (Stale=false) dan halaman terkait, urut nama
func IndexLine(b Book) string                                 // "nama_buku (bab1, bab2) [tag,camelTag]" + " warning: stale, jalankan grimoire build" bila ada bab Stale
func BuildIndex(books []Book) string                          // header + baris; terapkan IndexMaxBytes
```

Build (milik `pages-index`, dipakai `ipc-ctl`):

```go
func Build(store *Store, projectID, projectPath string) (Summary, error)
```

IPC (milik `ipc-ctl`, dipakai `wire_grimoire.go` di fitur yang sama):

```go
type Deps struct {
    Store       *Store
    ResolvePath func(ctx context.Context, projectID string) (string, error)
    Logger      *slog.Logger
}
func Register(srv *ipc.Server, d Deps)
```

### 2.4 IPC (JSON; galat lewat `ipc.NewError`, kode: `ErrConflict -32001`, `ErrNotFound -32002`, `ErrInvalid -32003`, `ErrInternal -32603`)

| Method | Params | Hasil |
|---|---|---|
| `grimoire.build` | `{projectId, full?}` | `{books, pages, indexBytes}`; `full` diabaikan; single-flight per project |
| `grimoire.status` | `{projectId}` | `{exists, books, pages, staleCount, building, indexBytes, estTokens, lawsBytes, llmEnabled, lastBuildCost, dir}` |
| `grimoire.list` | `{projectId}` | `[{name, tags, chapters:[{name, path, stale}]}]`, `[]` bila belum ada |
| `grimoire.read` | `{projectId, path}` | `{content, size}` |

Nilai status di tiket ini: `staleCount 0`, `estTokens indexBytes/4`, `lawsBytes 0`, `llmEnabled false`, `lastBuildCost 0`, `dir` absolut selalu terisi. `pages` = jumlah berkas bab (dihitung dari `state.json`/penjelajahan `books/`), `books` = jumlah folder buku.
Pemetaan galat: `projectId` kosong atau `ErrInvalidProject`/`ErrInvalidPath` -> `ErrInvalid`; project tak dikenal, berkas tak ada, `ErrNotBuilt` pada read -> `ErrNotFound`; sisanya `ErrInternal`. Params salah bentuk -> `ErrInvalid` dengan pesan `invalid params: ...` (pola `fsx/ipc.go`).
Event: topik `grimoire.changed`, payload `{projectId}`. Terbit saat build mulai (building true), selesai sukses, dan selesai gagal (building kembali false). (Koreksi kecil: tiket menyebut dua kalimat berbeda soal gagal; ambil perilaku bagian `ipc-ctl`: terbit juga saat gagal agar `building` UI kembali.)
`workspace.list` mengembalikan `[]Workspace{ID, Name, Projects:[{ID, Path, Name}]}` (field JSON huruf kecil sesuai `internal/workspace`; verifikasi tag JSON `id/path/name`/`projects` pada `internal/workspace/config.go` sebelum decode).

### 2.5 Format berkas

- Bab: baris 1 `Sumber: a, b, c` (maks 20 path lalu `, ...`), lalu `## Berkas`, `## Simbol`, `## Hash`. Tanpa tanggal/narasi. Modul terpotong menambah baris `(dipotong: N berkas lagi)`.
- `index.md`: `IndexHeader` (konstan, mengandung placeholder literal `{{grimoire_dir}}`), baris kosong, satu baris per buku urut nama. Byte-identik untuk input sama; tidak ada tanggal/jumlah.
- Pelipatan bila > `IndexMaxBytes`: kelompokkan buku berdasar awalan (segmen pertama nama buku sebelum `_`), lipat kelompok terbesar dulu (deterministik) menjadi satu baris `kelompok/* (N buku) [tagKelompok]` sampai muat; bila masih lebih, potong kelompok lagi sampai muat.
- `state.json`: JSON terindentasi dua spasi, `State` di atas; modul keyed nama modul (= nama buku).
- Pemecahan modul besar (> 40 berkas): bab per sub-direktori langsung (`Snake(subdir)`), sisa ke `lainnya`; satu buku tetap bernama modul.

### 2.6 Frontend

- Impor: `call`, `onEvent` dari `lib/ipc`; `activeProject`/`activeProjectId` dari `lib/stores`; `registerPanel` dari `lib/registry`; `layout` dari `lib/stores/layout`.
- Tipe TS (`modules/grimoire/client.ts`, milik `panel-ui`):

```ts
export interface GrimoireChapter { name: string; path: string; stale: boolean }
export interface GrimoireBook { name: string; tags: string[]; chapters: GrimoireChapter[] }
export interface GrimoireStatus {
  exists: boolean; books: number; pages: number; staleCount: number; building: boolean;
  indexBytes: number; estTokens: number; lawsBytes: number; llmEnabled: boolean; lastBuildCost: number; dir: string;
}
export interface GrimoireSummary { books: number; pages: number; indexBytes: number }
export function grimoireBuild(projectId: string): Promise<GrimoireSummary>
export function grimoireStatus(projectId: string): Promise<GrimoireStatus>
export function grimoireList(projectId: string): Promise<GrimoireBook[]>
export function grimoireRead(projectId: string, path: string): Promise<{ content: string; size: number }>
```

- `modules/grimoire/openPage.ts` (milik `readonly-pane`): `export async function openGrimoirePage(projectId: string, relPath: string): Promise<string>`. KOREKSI atas tiket: tiket menulis return `string` sinkron, tetapi `dir` hanya didapat lewat `grimoire.status` (async), jadi signature final adalah `Promise<string>` (mengembalikan id pane). `panel-ui` memanggilnya tanpa `await` yang menghalangi (`void openGrimoirePage(...)` dengan `.catch` ke `error` store).
- `Leaf`/`NewLeaf` memperoleh `root?: string; readOnly?: boolean` (milik `readonly-pane`, `model.ts`).
- `fileApi.ts`: `readChunk(path, offset, length, root?)`, `readAll(path, root?)`. Bila `root` ada, `path` yang diberikan absolut dan relatifnya dihitung dengan memotong `root + '/'`; panggil `call('fs.read', {root, path: rel, offset, limit})`.
- `ReadOnlyFileView.svelte` props: `{ root: string; path: string; projectId?: string }`.
- Slot komentar wajib di `GrimoirePanel.svelte`: `<!-- slot: laws (tiket 2) -->` dan `<!-- slot: stale/llm (tiket 3, 4) -->`.

## 3. Daftar fitur

Lima fitur persis dari tiket (tidak ditambah/digabung). Slug dipakai sebagai nama berkas plan `<slug>.md`.

### contract-types: Tipe dan layout penyimpanan grimoire
Scope: melengkapi `types.go` (sudah final dari Foundation; hanya boleh menambah komentar/helper aditif) dan menimpa `store.go` dengan implementasi penuh (helper path, `WriteFile` atomik, `ReadFile`, `Exists`, `ReadState/WriteState`, `RemoveBooks`, `Snake`, `CamelTag`, validasi project, penolakan path keluar).
ownedPaths: `internal/grimoire/types.go`, `internal/grimoire/store.go`

### scan-modules: Pindai project dan kelompokkan modul
Scope: `Scan` (ignore berlapis + `.gitignore`, tanpa symlink, deteksi biner/besar), `GroupModules` (Go/web/generik, batas ukuran, nama unik stabil), ekstraksi simbol, `Tag`, `HashModule` (SHA-256 working tree).
ownedPaths: `internal/grimoire/scan.go`, `internal/grimoire/modules.go`, `internal/grimoire/hash.go`

### pages-index: Penulis halaman, index.md, dan state.json
Scope: `RenderChapter`, `RenderBooks`, `IndexLine`, `BuildIndex` dengan `IndexMaxBytes`, `Build` (scan, grup, hash, tulis halaman, lalu `index.md` dan `state.json` terakhir; bersihkan buku lama).
ownedPaths: `internal/grimoire/pages.go`, `internal/grimoire/index.go`, `internal/grimoire/build.go`

### ipc-ctl: Method IPC, wiring daemon, dan shikigamictl grimoire
Scope: `grimoire.build/status/list/read`, single-flight per project, event `grimoire.changed`, modul daemon `grimoire` dengan `ResolvePath` lewat `workspace.list` (klien IPC malas), subcommand `grimoire build|status` dengan `--project`.
ownedPaths: `internal/grimoire/ipc.go`, `cmd/shikigamid/wire_grimoire.go`, `cmd/shikigamictl/cmd_grimoire.go`

### panel-ui: Panel Grimoire di sidebar kiri
Scope: `index.ts`, `client.ts`, `store.ts`, `GrimoirePanel.svelte` (tombol Generate, daftar buku/bab terlipat, status membangun, event `grimoire.changed`, tanpa polling, slot komentar).
ownedPaths: `frontend/src/modules/grimoire/index.ts`, `frontend/src/modules/grimoire/GrimoirePanel.svelte`, `frontend/src/modules/grimoire/store.ts`, `frontend/src/modules/grimoire/client.ts`

### readonly-pane: Pane file read-only berroot grimoire
Scope: field `root?`/`readOnly?` di `Leaf`/`NewLeaf`/`makeLeaf`, percabangan di `FilePane.svelte`, `ReadOnlyFileView.svelte`, `fileApi.ts` dengan parameter `root`, `openGrimoirePage`.
ownedPaths: `frontend/src/modules/layout/model.ts` (hanya penambahan field + penyalinan di `makeLeaf`), `frontend/src/modules/layout/panes/file.ts`, `frontend/src/modules/layout/panes/FilePane.svelte`, `frontend/src/modules/layout/panes/ReadOnlyFileView.svelte`, `frontend/src/modules/files/viewer/fileApi.ts`, `frontend/src/modules/grimoire/openPage.ts`

## 4. Foundation

Ada (hasFoundation = true); rinci di `00-foundation.md`. Isi singkat:
- `internal/grimoire/types.go` final (bagian 2.2).
- Stub berkompilasi (komentar baris pertama `// STUB(foundation): ditimpa oleh <fitur>`) dengan signature final bagian 2.3 untuk: `store.go`, `scan.go`, `modules.go`, `hash.go`, `pages.go`, `index.go`, `build.go`, `ipc.go`.
- `frontend/src/modules/grimoire/openPage.ts` stub dengan signature final.
- Bukan milik Foundation: apa pun di `cmd/`, `GrimoirePanel.svelte`, `client.ts`, `store.ts`, `index.ts`, `layout/*`, `fileApi.ts` (tidak diperlukan agar kompilasi lulus).

## 5. Wiring checklist

Dikerjakan agen wiring setelah semua fitur selesai. Seluruh registrasi memakai titik ekstensi yang sudah ada, jadi tidak ada berkas wiring bersama yang diedit. Periksa dan perbaiki hanya bila ada yang kurang:

1. Tidak ada stub tersisa: `grep -rn "STUB(foundation)" internal/grimoire frontend/src` harus kosong.
2. Daemon: `cmd/shikigamid/wire_grimoire.go` memanggil `RegisterModule("grimoire", ...)` dari `init()` dan `grimoire.Register(srv, deps)`; nama modul unik; TIDAK mengubah `modules.go`/`main.go`/`wire_workspace.go`.
3. CLI: `cmd/shikigamictl/cmd_grimoire.go` memanggil `RegisterCommand(Command{Name:"grimoire", ...})` dari `init()`; `registry.go`/`main.go` tidak berubah.
4. Frontend: `modules/grimoire/index.ts` memanggil `registerPanel({id:'grimoire', area:'sidebar', component: GrimoirePanel, order: 30})`; `main.ts` tidak diubah (glob memuatnya). `GrimoirePanel.svelte` mengimpor `openGrimoirePage` dari `./openPage`. `FilePane.svelte` mengarahkan leaf `readOnly && root` ke `ReadOnlyFileView.svelte`.
5. Konsistensi kontrak: field `dir` ada di hasil `grimoire.status`; `grimoire.read` dipakai `ReadOnlyFileView` hanya lewat `fs.read` berroot (bukan `grimoire.read`); nama method IPC di `client.ts` identik dengan yang didaftarkan `ipc.go`; `openGrimoirePage` memakai signature `Promise<string>` dan pemanggilnya sesuai.
6. Tidak ada perubahan pada `AgentSidebar`, `internal/ipc`, `internal/session`, `internal/provider`, `go.mod`, `go.sum`, `frontend/package.json`, lockfile (`git diff --stat` untuk berkas-berkas itu kosong).
7. Jalankan build guard (bagian 6). Uji manual bila lingkungan memungkinkan: `make build`, jalankan aplikasi, Generate Grimoire, buka bab, `shikigamictl grimoire status --project <id>` memberi `exists: true`, dan `<DataDir>/grimoire/<projectId>/` berisi `index.md`, `state.json`, `books/...` tanpa berkas baru di repo.

## 6. Build guard

Satu-satunya guard; tanpa unit test:
- `go build ./...` (juga `go vet ./internal/grimoire/... ./cmd/...` opsional, hanya bila cepat).
- Frontend: `cd frontend && npm run check && npm run build` (bila skrip `check` ada), atau `make build-ui`. Penutup epic: `make build`.
- Tidak ada dependency baru: `git diff --stat -- go.mod go.sum frontend/package.json frontend/package-lock.json` kosong.

Selama fase paralel, tiap agent cukup menjalankan guard yang relevan: fitur Go menjalankan `go build ./...`; fitur frontend menjalankan build frontend. Bila build gagal karena berkas fitur lain (bukan miliknya), jangan memperbaikinya; laporkan.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3) dan menulis planmu/progress sesuai instruksi workflow. Jangan menyentuh path fitur lain, berkas Foundation selain yang dimilikinya, atau berkas wiring bersama.
- Menimpa stub Foundation milik fiturmu adalah wajib; jangan mengubah signature bagian 2.3/2.6. Bila signature terasa salah, catat di plan dan tetap ikuti kontrak; jangan mengubahnya sepihak.
- Dilarang: mengedit `cmd/shikigamid/main.go`, `modules.go`, `cmd/shikigamictl/main.go`, `registry.go`, `internal/ipc`, `internal/session`, `internal/provider`, `AgentSidebar`, `go.mod`/`go.sum`, `package.json`/lockfile, `wire_workspace.go`.
- Tidak ada definisi ganda: konstanta dan helper ditaruh di berkas pemiliknya (mis. `IndexMaxBytes` hanya di `index.go`; `maxFileBytes` dan kawan-kawan hanya di `scan.go`/`modules.go`; sentinel error hanya di `types.go`).
- Dependensi runtime antar fitur yang belum hadir saat dikerjakan paralel ditangani oleh stub Foundation; jangan membuat berkas tiruan baru di luar ownedPaths.
