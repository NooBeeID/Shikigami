---
id: grimoire-ticket-01-grimoire-build
title: Grimoire: generate kerangka statik + panel sidebar
spec: docs/superpowers/specs/2026-11-22-grimoire-design.md
status: ready
wave: 1
depends_on: []
---
# Grimoire: generate kerangka statik + panel sidebar

## Tujuan

Epic ini adalah kemampuan Grimoire pertama yang terlihat di aplikasi. Daemon memindai project aktif menjadi modul, menulis halaman statik (baris `Sumber:`, path, hash, tanpa LLM) dan `index.md` ke `<DataDir>/grimoire/<projectId>/`, lalu panel **GRIMOIRE** di sidebar kiri menampilkan tombol Generate atau daftar buku/bab. Klik bab membuka pane `file` read-only berisi halaman itu. Alasannya (spec "Maksud"): sesi agent mulai tanpa konteks proyek; kerangka statik ini fondasi yang disuntik dan dipertajam oleh tiket 2 sampai 4. Tiket ini berdiri sendiri di atas kode yang sudah ada (registry modul daemon, `RegisterCommand`, `registerPanel`, `fsx` dengan parameter `Root`) dan tidak bergantung pada tiket Grimoire lain.

## Hasil yang bisa dijalankan

Setelah tiket ini selesai dan `make build` lulus, aplikasi jalan dan pengguna dapat:

1. Membuka sidebar kiri dan melihat panel **GRIMOIRE** di bawah daftar sesi, dengan latar sedikit lebih gelap dari sidebar.
2. Pada project yang belum punya grimoire: melihat tombol **Generate Grimoire**. Klik tombol menampilkan status "Membangun..." lalu panel berganti ke daftar buku (grup yang bisa dilipat) berisi bab.
3. Klik sebuah bab: terbuka pane `file` baru, read-only (tanpa tombol edit/save), menampilkan Markdown halaman dengan baris pertama `Sumber: ...`.
4. Dari terminal: `shikigamictl grimoire build --project <id>` dan `shikigamictl grimoire status --project <id>` mencetak JSON; panel yang sedang terbuka ikut ter-refresh lewat event `grimoire.changed` (tanpa polling).
5. Memeriksa `<DataDir>/grimoire/<projectId>/` berisi `index.md`, `state.json`, `books/<nama_snake>/<bab_snake>.md`, dan tidak ada berkas baru di dalam repo proyek.

## Batasan

In scope:
- Paket `internal/grimoire`: tipe, store, scan/kelompok modul, hash working tree, penulis halaman, `index.md`, `state.json`, `Build`.
- Method IPC `grimoire.build`, `grimoire.status`, `grimoire.list`, `grimoire.read`; event `grimoire.changed`; build single-flight per project.
- Modul daemon `grimoire` dan subcommand `shikigamictl grimoire build|status`.
- Modul frontend `modules/grimoire` (panel sidebar, store, client) dan perluasan aditif pane `file` agar bisa read-only dengan root grimoire.

Out of scope (milik tiket lain atau di luar spec):
- Penyuntikan index ke agent, `laws.md`, snapshot `run/grimoire-<hex>.md`, pengukuran token (tiket 2).
- Diff hash terhadap `state.json` lama, penandaan `stale`, build latar belakang, pemicu saat sesi baru (tiket 3). Tiket ini selalu menulis ulang seluruh grimoire pada tiap build dan tidak pernah menandai stale; field `stale` ada di tipe tetapi selalu `false`/dihilangkan.
- Ringkasan LLM, flag `grimoire.llm`, `settings.json`, estimasi (tiket 4).
- Integrasi Grimoire eksternal (`GrimoireURL/Key`), server MCP grimoire, laws otomatis, Windows.
- Menulis apa pun di dalam repo proyek; melonggarkan akses tulis `fs.write` ke luar `Project.Path`.
- Dependency baru (Go maupun npm) dan unit test (guard hanya build).
- Mengubah `AgentSidebar`, `wire_workspace.go`, `internal/ipc`, `internal/session`, `internal/provider`, `cmd/shikigamid/main.go`, `cmd/shikigamid/modules.go`, `cmd/shikigamictl/main.go`, `cmd/shikigamictl/registry.go`.

## Kontrak

### Yang dipakai (sudah ada di repo; baca berkas aslinya sebelum menulis)

- `cmd/shikigamid/modules.go`: `RegisterModule(name, func(srv *ipc.Server, d *Deps))`; `Deps{DataDir, SocketPath, Logger, ...}`, `d.Provide/Lookup/OnReady`. Modul dipasang urut alfabetis; layanan modul lain hanya lewat `d.OnReady`/`d.Lookup` atau secara malas di handler. Contoh pola: `wire_runs.go`, `wire_fs.go`.
- `cmd/shikigamictl/registry.go`: `RegisterCommand(Command{Name, Summary, Run func(ctx, c *ipc.Client, args []string) error})`, helper `printJSON`, `callAndPrint`. Contoh: `cmd_fs.go` (termasuk pemetaan `ipc.Error` ke pesan, `ipc.ErrNotFound`, `ipc.ErrInvalid`).
- `internal/ipc`: `srv.Register(method, func(ctx, json.RawMessage) (any, error))`, `srv.Publish(topic, payload)`, `ipc.NewError(code, msg)` dengan kode `ErrConflict -32001`, `ErrNotFound -32002`, `ErrInvalid -32003`, `ErrInternal -32603`. Bentuk decode parameter mengikuti `fsx/ipc.go` (`decode`, error `invalid params: ...`).
- `internal/paths`: `paths.DataDir()` (dibuat 0700; override `SHIKIGAMI_DATA_DIR`). Di daemon pakai `d.DataDir`, bila kosong jatuh ke `paths.DataDir()` (pola `wire_workspace.go`).
- `internal/fsx`: `fsx.IsIgnored(name)` / `fsx.DefaultIgnore` (nama persis: `node_modules`, `.git`, `vendor`, `dist`, `build`, ...), `fsx.ResolveUnder(root, rel)` (pencegah path keluar root), `fsx.Read(root, rel, offset, limit)`. Method `fs.read {root, path, offset, limit}` menerima `root` sembarang direktori; inilah yang dipakai pane read-only dengan root grimoire. `fsx/ignore.go` hanya mengenal daftar nama, bukan sintaks `.gitignore`; fungsi baca `.gitignore` ditulis di `internal/grimoire/scan.go` (lihat fitur scan-modules). Tidak ada helper tulis atomik yang diekspor; `store.go` menulis atomik sendiri (tulis ke berkas sementara di direktori yang sama, `Sync`, `os.Rename`).
- `internal/workspace`: `Workspace{ID, Name, Projects []Project}`, `Project{ID, Path, Name}`; method IPC `workspace.list`. Store workspace TIDAK disediakan lewat `d.Provide`, jadi resolusi `projectId -> Project.Path` dilakukan lewat `workspace.list` (lihat fitur ipc-ctl).
- Frontend: `lib/registry.ts` (`registerPanel({id, area, component, order})`; area `sidebar` dirender urut `order`), `lib/ipc.ts` (`call`, `onEvent(topicPrefix, fn)`), store `lib/stores` (`activeProject`, `activeProjectId`, `activeWorkspace`), pola `modules/files/tree/registerTree.ts` (Files `order: 20`), `modules/layout/model.ts` (`Leaf`, `NewLeaf`, `makeLeaf`), `modules/layout/registry.ts` (`registerPaneKind`), `modules/shell/helpers.ts` (`openFilePane`), penampil `modules/files/viewer/MarkdownView.svelte`. Semua `modules/*/index.ts` dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts', { eager: true })` di `frontend/src/main.ts`, jadi modul baru cukup membuat `modules/grimoire/index.ts` (tidak ada berkas entri yang perlu diedit).

### Yang disediakan

#### Penyimpanan (`internal/grimoire/store.go`)

```
<DataDir>/grimoire/<projectId>/
  index.md                          # ditulis tiket ini, disuntik tiket 2
  laws.md                           # ditulis user (tiket 2); tiket ini tidak membuat
  state.json                        # {projectId, builtAt, modules:{<nama>:{hash, sources[], stale?:bool}}}
  settings.json                     # tiket 4; tidak dibuat di sini
  books/<nama_snake>/<bab_snake>.md
```
Satu `Project.Path` = satu grimoire (worktree = project sendiri). Nama buku dan bab snake_case. Tidak ada berkas ditulis di dalam repo proyek.

#### Tipe Go (`internal/grimoire/types.go`)

```go
type Chapter struct {
    Name    string   `json:"name"`            // snake_case
    Path    string   `json:"path"`            // relatif ke direktori grimoire project: books/<buku>/<bab>.md
    Sources []string `json:"sources"`         // path relatif repo
    Stale   bool     `json:"stale"`
}
type Book struct {
    Name     string    `json:"name"`          // snake_case
    Tags     []string  `json:"tags"`          // satu token camelCase tiap tag
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
type Summary struct { Books, Pages int; IndexBytes int }   // hasil Build
var ErrNotBuilt, ErrInvalidPath, ErrInvalidProject = errors.New(...) // sentinel
```
Field baru oleh tiket 2 sampai 4 ditambahkan secara aditif; jangan mengubah arti field di atas.

#### Format bab dan `index.md`

- Bab: baris pertama `Sumber: <path relatif repo>, <path relatif repo>, ...`; isi snapshot fakta tanpa tanggal, narasi, atau changelog; ditulis ulang utuh tiap build.
- `index.md`: header aturan konstan (teks tetap, tanpa tanggal/jumlah agar byte-stabil), lalu satu baris per buku: `nama_buku (bab1, bab2) [tag,camelTag]`. Bila ada bab stale (tiket 3 yang mengisinya) baris ditambah ` warning: stale, jalankan grimoire build`; tiket ini hanya menyediakan fungsi pembuat baris yang mendukung penanda itu.
- Tag statik dari nama folder/package, satu token camelCase (mis. folder `session_pty` menjadi `sessionPty`).
- Batas ukuran index: konstanta `IndexMaxBytes` (default 16384). Bila terlewati, buku yang paling jarang berubah (tiket ini: urutan terbesar jumlah bab/ukuran halaman, deterministik) dilipat menjadi satu baris per kelompok, mis. `internal/provider/* (12 buku) [provider]`, sampai muat.

#### IPC (JSON, error mengikuti `internal/ipc/errors.go`)

| Method | Params | Hasil |
|---|---|---|
| `grimoire.build` | `{projectId, full?}` | `{books, pages, indexBytes}`; `full` diterima dan diabaikan (selalu penuh) agar kompatibel dengan tiket 3. Bila project yang sama sedang build, join ke build berjalan (single-flight) dan mengembalikan hasil yang sama. |
| `grimoire.status` | `{projectId}` | `{exists, books, pages, staleCount, building, indexBytes, estTokens, lawsBytes, llmEnabled, lastBuildCost, dir}`. `dir` adalah field aditif tiket ini: path absolut direktori grimoire project (selalu diisi walau belum ada), dipakai `openGrimoirePage`. Tiket ini mengisi `exists`, `books`, `pages`, `building`, `indexBytes`, `dir`; `staleCount: 0`; `estTokens: indexBytes/4`; `lawsBytes: 0`; `llmEnabled: false`; `lastBuildCost: 0` (tiket 2 dan 4 menyempurnakan nilainya, nama field tetap). |
| `grimoire.list` | `{projectId}` | `[{name, tags, chapters:[{name, path, stale}]}]` (array kosong `[]`, bukan null, bila belum ada) |
| `grimoire.read` | `{projectId, path}` | `{content, size}`; `path` relatif ke direktori grimoire project, harus lewat `fsx.ResolveUnder`; `ErrInvalid` bila keluar dari direktori, `ErrNotFound` bila tidak ada. |

`projectId` yang tidak dikenal (tidak ada di `workspace.list`) memberi `ErrNotFound`; `projectId` kosong memberi `ErrInvalid`. Event: `grimoire.changed {projectId}` dipublikasikan satu kali setelah build sukses (dan saat build gagal tanpa perubahan, tidak dipublikasikan). Event juga dipublikasikan bila `building` berubah (mulai dan selesai) supaya UI dapat menampilkan progres tanpa polling.

#### CLI

`shikigamictl grimoire build [--project <id>]` dan `shikigamictl grimoire status [--project <id>]`. Bila `--project` dihilangkan: pakai satu-satunya project bila hanya ada satu di `workspace.list`, selain itu galat berisi daftar id project. Keluaran JSON terformat (`callAndPrint`).

#### Frontend

- `registerPanel({id:'grimoire', area:'sidebar', component: GrimoirePanel, order: 30})` (di bawah daftar sesi dan Files `order: 20`).
- Store `grimoire` per project aktif (`modules/grimoire/store.ts`): `{projectId, exists, building, books[], error}`.
- Client `modules/grimoire/client.ts`: satu-satunya tempat memanggil `call('grimoire.*')`.
- Pane read-only: `NewLeaf`/`Leaf` memperoleh field aditif `root?: string` dan `readOnly?: boolean`. Helper `openGrimoirePage(projectId: string, relPath: string): string` di `modules/grimoire/openPage.ts`.

## Fitur

Kelima fitur ini bisa dikerjakan paralel kecuali dinyatakan lain per fitur. Antarmuka antar fitur disepakati lewat bagian Kontrak di atas, sehingga masing-masing bisa dikerjakan tanpa menunggu yang lain. `contract-types` adalah fondasi: tipe dan helper path di `types.go`/`store.go` harus dibuat lebih dulu (atau cukup mengikuti signature di bawah), karena `scan-modules`, `pages-index`, dan `ipc-ctl` mengimpornya. Agar build lolos di tiap tahap, bila fitur yang bergantung dikerjakan sebelum `contract-types` selesai, buat dulu `types.go` dengan isi sesuai Kontrak lalu `contract-types` melengkapinya (jangan ada definisi ganda).

### contract-types: Tipe dan layout penyimpanan grimoire

Deskripsi: buat `internal/grimoire/types.go` (tipe pada Kontrak) dan `internal/grimoire/store.go`:
- `type Store struct{ dataDir string }`, `NewStore(dataDir string) *Store`.
- Helper path: `Dir(projectID) string` (`<dataDir>/grimoire/<projectId>`), `IndexPath`, `LawsPath`, `StatePath`, `BookPath(book, chapter)`, `BooksDir`. `projectID` divalidasi (non-kosong, tanpa `/`, `..`, atau NUL; ULID biasa lolos) dengan `ErrInvalidProject`.
- `WriteFile(projectID, rel string, data []byte) error` tulis atomik (berkas sementara + `Sync` + `Rename`, direktori dibuat 0700, berkas 0600), selalu lewat `fsx.ResolveUnder(Dir(projectID), rel)`.
- `ReadFile(projectID, rel)` (batas ukuran wajar, mis. 4 MiB), `Exists(projectID) bool` (ada `index.md`), `ReadState/WriteState` (JSON terindentasi; `ErrNotBuilt` bila tidak ada), `RemoveBooks(projectID)` menghapus `books/` utuh sebelum penulisan ulang.
- `Snake(s string) string`: huruf kecil, karakter non `[a-z0-9]` menjadi `_`, deret `_` diringkas, tanpa `_` di tepi, hasil kosong menjadi `x`; `CamelTag(s string) string`: satu token camelCase dari nama folder/package (pisah di non-alfanumerik, kata pertama huruf kecil, kata berikutnya huruf awal besar).
- Sentinel error: `ErrNotBuilt`, `ErrInvalidPath`, `ErrInvalidProject`.

Acceptance criteria:
- [ ] `types.go` memuat `Book`, `Chapter`, `ModuleState`, `State`, `Summary` dengan tag JSON seperti Kontrak.
- [ ] `store.go` menyediakan helper path, `WriteFile` atomik, `ReadFile`, `Exists`, `ReadState`, `WriteState`, `RemoveBooks`, `Snake`, `CamelTag`.
- [ ] Tidak ada fungsi yang menulis di luar `<DataDir>/grimoire/<projectId>/`; path dengan `..` atau absolut ditolak dengan `ErrInvalidPath`.
- [ ] `Snake("Session PTY")` = `session_pty`, `CamelTag("session_pty")` = `sessionPty`.
- [ ] `go build ./...` lulus.

ownedPaths: `internal/grimoire/types.go`, `internal/grimoire/store.go`. Fondasi tiket; fitur lain di epic ini mengimpornya, tetapi boleh dikerjakan paralel selama signature mengikuti Kontrak.

### scan-modules: Pindai project dan kelompokkan modul

Deskripsi: `internal/grimoire/scan.go`, `modules.go`, `hash.go`. Tanpa panggilan LLM, tanpa proses eksternal wajib.
- `Scan(root string) ([]FileInfo, error)`: telusuri `Project.Path` (tidak mengikuti symlink; berkas bukan reguler dilewati). Hormati ignore berlapis: nama di `fsx.IsIgnored`, ditambah `.gitignore` di root dan sub-direktori (baca sendiri; dukung pola nama, `dir/`, `*.ext`, `/anchored`, `**`, dan negasi `!`; pola yang tak dikenali diabaikan, bukan galat). Lewati berkas biner (cek NUL pada 8 KiB awal) dan berkas lebih besar dari `maxFileBytes` (default 1 MiB) hanya untuk keperluan simbol, tetapi tetap tercatat di daftar berkas modul. Folder tanpa git didukung penuh; tidak ada ketergantungan pada `.git`.
- `GroupModules(files []FileInfo) []Module`: satu modul per unit: (a) direktori yang berisi berkas `.go` dikelompokkan per package/direktori; (b) direktori berisi `.svelte`/`.ts`/`.js` per direktori (`frontend/src/modules/<x>` menjadi satu modul `<x>` termasuk subfoldernya, subfolder lain mengikuti direktori terdekat yang berisi berkas kode); (c) sisa direktori berisi berkas teks/konfigurasi/dokumen per direktori tingkat pertama atau kedua (generik). Batas per modul: `maxModuleFiles` (default 200) dan `maxModuleBytes` (default 512 KiB) untuk bahan simbol; kelebihan tetap dicantumkan sebagai daftar berkas tetapi simbolnya dipotong dan halaman memuat baris `(dipotong: N berkas lagi)`. Nama modul unik dan stabil (path relatif repo, `/` diganti `_`, lalu `Snake`), tabrakan diselesaikan dengan akhiran `_2`, `_3` berdasar urutan path.
- `Module{Name string; Dir string; Kind string; Files []FileInfo; Symbols []string; Sources []string; Tag string}`. `Symbols` ringkas: untuk Go, deklarasi top-level yang diekspor (`func`, `type`, `const`, `var` berawal huruf besar) via `go/parser` atau regex ringan; untuk Svelte/TS, `export function/const/class/type/interface` dan nama komponen `.svelte`; untuk generik, kosong. Maksimum simbol per modul dibatasi (mis. 60). `Sources`: path relatif repo (urut leksikal, `/`).
- `Tag`: `CamelTag(basename direktori)` (untuk Go: nama package dari berkas pertama bila ada); satu token camelCase.
- `hash.go`: `HashModule(root string, m Module) (string, error)` = SHA-256 hex atas isi working tree: untuk tiap berkas modul urut path, tulis path + panjang + isi (bukan HEAD git; edit yang belum di-commit ikut terhitung). Deterministik (tanpa mtime).
- Urutan keluaran seluruhnya deterministik (urut path / nama).

Acceptance criteria:
- [ ] `Scan` menghormati `fsx.IsIgnored` dan `.gitignore` (nama, `dir/`, `*.ext`, anchored, negasi) dan tidak membaca berkas biner.
- [ ] `GroupModules` menghasilkan modul untuk package Go, direktori Svelte/TS, dan direktori generik, dengan batas ukuran per modul.
- [ ] `HashModule` stabil antar-jalan, berubah bila isi berkas modul berubah (termasuk yang belum di-commit), dan bekerja pada folder tanpa `.git`.
- [ ] `Module.Tag` selalu satu token camelCase, `Module.Sources` path relatif repo.
- [ ] Tidak ada pemanggilan LLM atau penulisan berkas di paket ini selain pada `store.go`.
- [ ] `go build ./...` lulus.

ownedPaths: `internal/grimoire/scan.go`, `internal/grimoire/modules.go`, `internal/grimoire/hash.go`. Bisa dikerjakan paralel dengan fitur lain; hanya butuh tipe dari `contract-types` (`Snake`, `CamelTag`).

### pages-index: Penulis halaman, index.md, dan state.json

Deskripsi: `internal/grimoire/pages.go`, `index.go`, `build.go`.
- `pages.go`: `RenderChapter(m Module) string`: baris pertama `Sumber: a, b, c` (path relatif repo, dipotong ke 20 path pertama + `, ...` bila lebih, tetap satu baris); lalu bagian Markdown `## Berkas` (daftar berkas dengan ukuran), `## Simbol` (daftar ringkas), `## Hash` (hash modul, 12 karakter awal). Tanpa tanggal, changelog, atau narasi. Satu modul menjadi satu buku dengan satu bab bernama sama (`books/<modul>/<modul>.md`); modul besar (lebih dari 40 berkas) dipecah menjadi bab per sub-direktori langsung, nama bab `Snake(subdir)`, dan sisa berkas ke bab `lainnya`. Hasil `[]Book` dengan `Chapter.Path`/`Sources`.
- `index.go`: `BuildIndex(books []Book) string`: header aturan konstan (konstanta string; isi: baca index ini dulu, buka halaman buku/bab yang relevan sebelum membaca source, path sumber tertera di baris pertama tiap bab, halaman ada di `<direktori grimoire>/books/<buku>/<bab>.md`; berikan path absolut lewat placeholder tetap `{{grimoire_dir}}` yang diganti saat suntikan oleh tiket 2, bukan di sini), satu baris per buku urut nama, `nama_buku (bab1, bab2) [tag,camelTag]`, mendukung akhiran stale. Terapkan `IndexMaxBytes`: lipat bertahap seperti di Kontrak sampai muat; hasil tetap deterministik. Tag diambil dari `Module.Tag` ditambah tag nama package induk bila berbeda, tanpa duplikat.
- `build.go`: `Build(store *Store, projectID, projectPath string) (Summary, error)`: validasi `projectPath` (absolut, direktori ada) dan `projectID`; `Scan` lalu `GroupModules`, hitung hash tiap modul, render semua halaman; tulis ke direktori sementara lalu ganti `books/` (hapus `books/` lama lewat `RemoveBooks`, tulis halaman baru atomik per berkas), tulis `index.md`, tulis `state.json` terakhir (`ProjectID`, `BuiltAt`, `Modules[nama]{Hash, Sources}`). Project kosong (tanpa modul) tetap menulis `index.md` berisi header saja dan `Summary{0,0,...}`; bukan galat. Kegagalan di tengah tidak boleh meninggalkan `index.md` yang menunjuk bab yang tidak ada: tulis `index.md` dan `state.json` setelah semua halaman tertulis. Tidak menyentuh apa pun di dalam `projectPath`.

Acceptance criteria:
- [ ] Setiap bab diawali baris `Sumber: ...` berisi path relatif repo.
- [ ] `index.md` berisi header konstan dan satu baris per buku berformat `nama_buku (bab1, bab2) [tag,camelTag]`; output byte-identik untuk input yang sama.
- [ ] Index yang melewati `IndexMaxBytes` dilipat menjadi baris per kelompok dan akhirnya ukuran <= `IndexMaxBytes`.
- [ ] `state.json` memuat `projectId`, `builtAt`, dan `modules.<nama>.{hash, sources}`.
- [ ] `Build` mengembalikan `Summary` (buku, halaman, ukuran index) dan tidak menulis apa pun di `projectPath`.
- [ ] Build ulang atas project yang sama menghapus buku yang modulnya sudah hilang.
- [ ] `go build ./...` lulus.

ownedPaths: `internal/grimoire/pages.go`, `internal/grimoire/index.go`, `internal/grimoire/build.go`. Bisa dikerjakan paralel; mengimpor `Module`/`Scan` dari `scan-modules` dan `Store` dari `contract-types` sesuai signature di Kontrak.

### ipc-ctl: Method IPC, wiring daemon, dan shikigamictl grimoire

Deskripsi:
- `internal/grimoire/ipc.go`: `type Deps struct { Store *Store; ResolvePath func(ctx context.Context, projectID string) (string, error); Logger *slog.Logger }` dan `Register(srv *ipc.Server, d Deps)` yang mendaftarkan `grimoire.build/status/list/read` sesuai Kontrak. Build single-flight per project: `map[projectID]*inflight` dengan mutex; panggilan kedua untuk project yang sama menunggu hasil yang sama; `building` pada `status` bernilai true selama berjalan. Build berjalan dengan konteks tersendiri (tidak ikut dibatalkan bila klien terputus). Publish `grimoire.changed {projectId}` saat build mulai, selesai sukses, dan selesai gagal (agar `building` UI kembali). `list` memuat `Store.ReadState` + struktur direktori `books/` (nama buku/bab dari berkas, `tags` dari `index.md` bila perlu; cukup dari state/penjelajahan direktori, tidak perlu parsing LLM). `read` memakai `Store.ReadFile` dengan path tervalidasi. Pemetaan galat: `ErrInvalidProject`/`ErrInvalidPath` -> `ipc.ErrInvalid`, project tak dikenal/berkas tak ada/`ErrNotBuilt` pada `read` -> `ipc.ErrNotFound`, lainnya -> `ipc.ErrInternal`.
- `cmd/shikigamid/wire_grimoire.go`: `RegisterModule("grimoire", ...)`. Tentukan `dataDir` (`d.DataDir`, fallback `paths.DataDir()`), buat `Store`, dan `ResolvePath` yang memanggil `workspace.list` lewat klien IPC ke `d.SocketPath` (baca `internal/ipc/client.go` untuk konstruktor/dial; buat klien secara malas dan tutup setelah pakai; panggil di dalam handler, bukan saat wire, karena modul workspace dipasang setelah `grimoire` secara alfabetis dan layanan socket baru siap setelah semua modul ter-wire; bila `d.OnReady` diperlukan untuk memulai, gunakan itu). Bila `d.Lookup("workspace.store")` kelak tersedia, boleh dipakai lebih dulu, tetapi tiket ini tidak mengubah `wire_workspace.go`. `ResolvePath` mengembalikan `Project.Path` untuk `projectId` (cari di semua workspace) atau `ipc.ErrNotFound`.
- `cmd/shikigamictl/cmd_grimoire.go`: `RegisterCommand(Command{Name:"grimoire", Summary:"Bangun dan lihat status grimoire project", Run: runGrimoire})` dengan sub `build` dan `status`, flag `--project <id>` (parsing manual atau `flag.FlagSet`), penentuan project default seperti Kontrak, pemetaan galat gaya `fsError`. Teks usage dalam bahasa Indonesia.

Acceptance criteria:
- [ ] `grimoire.build/status/list/read` terdaftar dan mengembalikan bentuk sesuai Kontrak; `list` mengembalikan `[]` bila belum ada grimoire.
- [ ] Dua `grimoire.build` bersamaan untuk project yang sama hanya menjalankan satu build (single-flight); project berbeda berjalan independen.
- [ ] `grimoire.changed {projectId}` terbit setelah build selesai (dan saat `building` berubah).
- [ ] `grimoire.read` menolak path yang keluar dari direktori grimoire (`ErrInvalid`) dan melaporkan `ErrNotFound` untuk berkas yang tidak ada.
- [ ] `shikigamictl grimoire build|status [--project <id>]` jalan terhadap daemon dan mencetak JSON; tanpa `--project` perilaku default sesuai Kontrak.
- [ ] Modul `grimoire` tidak panik bila store workspace tidak tersedia (error dikembalikan per panggilan).
- [ ] `go build ./...` lulus.

ownedPaths: `internal/grimoire/ipc.go`, `cmd/shikigamid/wire_grimoire.go`, `cmd/shikigamictl/cmd_grimoire.go`. Bisa dikerjakan paralel dengan fitur lain di epic ini (memakai `Build`, `Store`, dan tipe sesuai signature Kontrak); penyambungan uji akhir bergantung pada `pages-index` sudah ada.

### panel-ui: Panel Grimoire di sidebar kiri

Deskripsi: `frontend/src/modules/grimoire/` berisi `index.ts`, `GrimoirePanel.svelte`, `store.ts`, `client.ts`.
- `index.ts`: `registerPanel({id:'grimoire', area:'sidebar', component: GrimoirePanel, order: 30})` mengikuti pola `modules/files/tree/registerTree.ts` (boleh lewat fungsi `registerGrimoirePanel()` bila rapi); tidak mengubah `AgentSidebar`.
- `client.ts`: `grimoireBuild(projectId)`, `grimoireStatus(projectId)`, `grimoireList(projectId)`, `grimoireRead(projectId, path)` memanggil `call('grimoire.*')`; tipe TypeScript untuk `GrimoireBook`, `GrimoireChapter`, `GrimoireStatus` sesuai Kontrak.
- `store.ts`: store (Svelte `writable`/`derived`) per project aktif (ikut `activeProject`/`activeProjectId` dari `lib/stores`). Saat project aktif berganti: muat `status` dan, bila `exists`, `list`. Berlangganan `onEvent('grimoire.changed', ...)` sekali (idempoten, dibersihkan saat tidak dipakai), dan memuat ulang hanya bila `projectId` payload sama dengan project aktif. Aksi `generate()`: set `building = true`, panggil `grimoireBuild`, lalu muat ulang (hasil juga datang lewat event). Tanpa `setInterval`/polling. Galat disimpan di `error` dan ditampilkan ringkas di panel.
- `GrimoirePanel.svelte`: tajuk **GRIMOIRE** (huruf kapital, gaya seragam dengan tajuk panel lain), latar sedikit lebih gelap dari sidebar memakai token tema yang sudah ada (mis. `color-mix(in srgb, var(--bg-sidebar, ...) 88%, black)` atau token yang setara; periksa variabel CSS di panel Files/AgentSidebar dan jangan menambah token global). Keadaan: tanpa project aktif -> teks netral "Pilih project"; belum ada -> tombol **Generate Grimoire**; saat build -> status "Membangun..." dengan tombol nonaktif; sudah ada -> buku sebagai grup yang bisa dilipat (default terlipat, status lipat disimpan di memori komponen), bab sebagai item; bab dengan `stale` diberi penanda kecil (belum pernah terjadi di tiket ini). Tombol kecil "Bangun ulang" di header daftar. Klik bab memanggil `openGrimoirePage(projectId, chapter.path)` dari `openPage.ts` (fitur `readonly-pane`). Item bisa dioperasikan keyboard (button/role yang tepat) dan teks panjang dipotong dengan ellipsis.
- Slot aditif: sediakan di `GrimoirePanel.svelte` dua area kosong berkomentar (`<!-- slot: laws (tiket 2) -->`, `<!-- slot: stale/llm (tiket 3, 4) -->`) agar tiket lain menambah tanpa merombak struktur.

Acceptance criteria:
- [ ] Panel GRIMOIRE muncul di sidebar kiri di bawah daftar sesi, dengan latar sedikit lebih gelap dari sidebar dan memakai token tema yang ada.
- [ ] Project tanpa grimoire menampilkan tombol Generate Grimoire; klik memicu build dan menampilkan status progres lalu daftar buku.
- [ ] Project dengan grimoire menampilkan buku (grup yang bisa dilipat) dan bab (item); klik bab memanggil `openGrimoirePage`.
- [ ] Panel mengikuti project aktif dan ter-refresh lewat event `grimoire.changed` tanpa `setInterval`/polling.
- [ ] `AgentSidebar` dan berkas di luar `frontend/src/modules/grimoire/` tidak diubah oleh fitur ini.
- [ ] Build frontend (`make build`) lulus.

ownedPaths: `frontend/src/modules/grimoire/index.ts`, `GrimoirePanel.svelte`, `store.ts`, `client.ts` (seluruhnya di `frontend/src/modules/grimoire/`, kecuali `openPage.ts` milik `readonly-pane`). Bisa dikerjakan paralel; hanya butuh nama `openGrimoirePage(projectId, relPath)` dari `readonly-pane` (bila belum ada, buat stub sementara di `openPage.ts` dengan signature sama lalu `readonly-pane` mengisinya) dan nama method IPC pada Kontrak.

### readonly-pane: Pane file read-only berroot grimoire

Deskripsi: perluasan kecil dan aditif agar leaf pane `file` menerima root eksplisit dan flag `readOnly`, tanpa jenis pane baru dan tanpa membuka akses tulis.
- `modules/layout/model.ts`: tambahkan `root?: string` dan `readOnly?: boolean` ke `Leaf` dan `NewLeaf`; `makeLeaf` menyalin keduanya bila terdefinisi (pola yang sama dengan `filePath`). Hanya penambahan; perilaku leaf lama tidak berubah. (Berkas ini sengaja ditambahkan ke daftar milik epic karena tanpa itu field dibuang oleh `makeLeaf`.)
- `modules/layout/panes/file.ts`: `titleFor` tetap, tambahkan awalan kecil untuk leaf `readOnly` bila membantu (opsional, mis. tanpa perubahan judul).
- `modules/layout/panes/FilePane.svelte`: bila `pane.readOnly && pane.root`, render komponen read-only baru `ReadOnlyFileView.svelte` (di folder yang sama) dan JANGAN memakai `FileTab` (yang berisi editor/save serta `splitPath` yang hanya mengenal project). Selain itu perilaku lama persis. `ReadOnlyFileView.svelte` menerima `root` dan `path` (absolut: `root + '/' + rel`), membaca lewat `readAll`/`readChunk` dari `fileApi.ts` dengan parameter root, menampilkan Markdown lewat `MarkdownView.svelte` (atau `PlainTextView` bila bukan markdown), menangani keadaan loading/not found/binary/error, memuat ulang saat event `grimoire.changed` untuk project terkait diterima (opsional tetapi disarankan; tanpa polling), tanpa tombol edit/save, tanpa `fs.write`, tanpa `fs.watch`.
- `modules/files/viewer/fileApi.ts`: `readChunk(path, offset, length, root?)` dan `readAll(path, root?)` menerima `root` opsional; bila ada, memanggil `fs.read` dengan `{root, path: rel, offset, limit}` langsung (tambahkan fungsi aditif `fsReadRooted(root, rel, offset, limit)` bila perlu; `fsClient.ts` bukan milik epic, jadi panggil `call('fs.read', ...)` dari `fileApi.ts` dan petakan galat dengan `classifyError` yang diekspor `fsClient.ts`). Tanpa `root` perilaku lama (via `splitPath`) tidak berubah.
- `modules/grimoire/openPage.ts`: `openGrimoirePage(projectId: string, relPath: string): string`: ambil direktori grimoire project (absolut, `<DataDir>/grimoire/<projectId>`) dari field aditif `dir` pada hasil `grimoire.status` (diisi oleh fitur `ipc-ctl`), lalu `layout.openPane({kind:'file', projectId, filePath: dir + '/' + relPath, root: dir, readOnly: true, title: <nama bab>})`. Bila pane dengan `filePath` sama sudah ada, fokuskan (pola `findFilePane` di `modules/shell/helpers.ts`; boleh mengimpor `layout` dari `lib/stores`). Tiket 2 sampai 4 tidak mengubah arti field `dir`.
- Akses tulis: tidak ada perubahan pada `fs.write`, `FileTab`, atau `splitPath`; pane read-only tidak pernah memanggil `fs.write`.

Catatan ketergantungan antar fitur: `openPage.ts` membutuhkan field `dir` pada `grimoire.status` dari `ipc-ctl` (field aditif yang disepakati di sini; `ipc-ctl` WAJIB mengisinya). Jika `dir` belum ada saat dikerjakan paralel, `openPage.ts` memanggil `grimoire.status` dan memakai `dir` bila ada.

Acceptance criteria:
- [ ] `Leaf`/`NewLeaf` punya `root?` dan `readOnly?`, dan `makeLeaf` menyalinnya; leaf lama tidak berubah perilakunya.
- [ ] Leaf `file` dengan `readOnly` dan `root` menampilkan Markdown halaman grimoire tanpa editor/save/watch.
- [ ] `fs.read` untuk halaman memakai `root` = direktori grimoire project (di luar `Project.Path`) dan tidak ada panggilan `fs.write`.
- [ ] Pane file biasa (tanpa `root`/`readOnly`) berperilaku sama seperti sebelumnya.
- [ ] `openGrimoirePage` membuka pane read-only atau memfokuskan pane yang sudah ada untuk halaman yang sama.
- [ ] Build frontend (`make build`) lulus.

ownedPaths: `frontend/src/modules/layout/model.ts` (hanya penambahan field), `frontend/src/modules/layout/panes/file.ts`, `frontend/src/modules/layout/panes/FilePane.svelte`, `frontend/src/modules/layout/panes/ReadOnlyFileView.svelte` (baru), `frontend/src/modules/files/viewer/fileApi.ts`, `frontend/src/modules/grimoire/openPage.ts`. Bisa dikerjakan paralel dengan fitur lain di epic ini; kesepakatan yang mengikat: field `dir` pada `grimoire.status` (dari `ipc-ctl`) dan nama `openGrimoirePage`.

## Path yang dimiliki

Hanya path berikut yang boleh disentuh oleh tiket ini:
- `internal/grimoire/` (tiket 1 membuat: `types.go`, `store.go`, `scan.go`, `modules.go`, `hash.go`, `pages.go`, `index.go`, `build.go`, `ipc.go`; berkas lain di folder ini milik tiket 2 sampai 4)
- `cmd/shikigamid/wire_grimoire.go`
- `cmd/shikigamictl/cmd_grimoire.go`
- `frontend/src/modules/grimoire/` (`index.ts`, `GrimoirePanel.svelte`, `store.ts`, `client.ts`, `openPage.ts`; subfolder `laws/`, `stale/`, `llm/` milik tiket lain)
- Tambahan aditif untuk fitur `readonly-pane` (diperkaya dari dekomposisi awal, tidak dipakai epic lain): `frontend/src/modules/layout/model.ts` (hanya field `root?`/`readOnly?` dan penyalinannya di `makeLeaf`), `frontend/src/modules/layout/panes/file.ts`, `frontend/src/modules/layout/panes/FilePane.svelte`, `frontend/src/modules/layout/panes/ReadOnlyFileView.svelte`, `frontend/src/modules/files/viewer/fileApi.ts`

## Wiring

Langkah penutup tiket ini, memakai titik ekstensi yang sudah ada sehingga hasilnya terlihat dan jalan tanpa tiket lain:
1. Daemon: `cmd/shikigamid/wire_grimoire.go` memanggil `RegisterModule("grimoire", ...)` dari `init()` (satu berkas, tanpa mengedit `modules.go` atau `main.go`); wire memanggil `grimoire.Register(srv, deps)`. Karena `RegisterModule` dipakai, nama `grimoire` harus unik; tiket 2 sampai 4 memakai nama sendiri (`grimoire-inject`, dst.).
2. CLI: `cmd/shikigamictl/cmd_grimoire.go` memanggil `RegisterCommand` dari `init()` (tanpa mengedit `registry.go`/`main.go`).
3. Frontend: `modules/grimoire/index.ts` memanggil `registerPanel` dan otomatis dimuat oleh `import.meta.glob('./modules/*/index.ts')` di `main.ts` (tidak ada berkas entri yang perlu diedit). `openGrimoirePage` dipakai `GrimoirePanel.svelte`; `FilePane.svelte` menyalurkan leaf read-only ke `ReadOnlyFileView.svelte`.
4. Uji pemakaian akhir secara manual: jalankan aplikasi (`make build` lalu `make dev`/binary), pilih project, klik Generate Grimoire, buka sebuah bab (pane read-only), dan jalankan `shikigamictl grimoire status --project <id>` untuk memastikan `exists: true`.

Titik ekstensi untuk tiket berikutnya (tidak boleh mengubah tiket ini): modul wire tambahan tiap tiket lewat `RegisterModule` baru dan fungsi `Register*` tambahan di `internal/grimoire`; slot komentar di `GrimoirePanel.svelte`; field aditif pada `grimoire.status`.

## Guard

Cukup build berhasil, tanpa unit test:
- `go build ./...`
- `make build` (menyertakan build frontend)
- Pastikan juga tidak ada dependency baru: `go.mod`, `go.sum`, `frontend/package.json`, dan lockfile tidak berubah (`git diff --stat` pada berkas itu kosong).
