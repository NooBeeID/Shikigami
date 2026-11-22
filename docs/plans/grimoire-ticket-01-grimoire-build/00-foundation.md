# Foundation: grimoire-ticket-01-grimoire-build (tipe final + stub berkompilasi)

Baca dulu `overview.md` (bagian 2 adalah sumber kebenaran; plan ini hanya urutan kerja) dan tiket `docs/tickets/grimoire-ticket-01-grimoire-build.md`. Tanpa logika nyata, tanpa dependency baru, tanpa unit test.

Tujuan: setelah foundation, kelima fitur dapat dikerjakan paralel karena semua signature lintas-fitur sudah ada dan `go build ./...` serta build frontend lulus.

Berkas yang dibuat:
- `internal/grimoire/types.go` (final, bukan stub)
- Stub Go: `store.go`, `scan.go`, `modules.go`, `hash.go`, `pages.go`, `index.go`, `build.go`, `ipc.go` (semua di `internal/grimoire/`)
- Stub frontend: `frontend/src/modules/grimoire/openPage.ts`

## Langkah

1. **`types.go`.** Salin tipe dan sentinel dari overview 2.2 persis: `Chapter`, `Book`, `ModuleState`, `State`, `Summary` (dengan tag JSON `books/pages/indexBytes`), `ErrNotBuilt`, `ErrInvalidPath`, `ErrInvalidProject`. Import `errors`, `time`. Package `grimoire`.
2. **`store.go` (stub).** Baris pertama `// STUB(foundation): ditimpa oleh contract-types`. Deklarasikan `type Store struct{ dataDir string }`, `NewStore`, semua metode pada overview 2.3 (`Dir`, `ValidateProject`, `IndexPath`, `LawsPath`, `StatePath`, `BooksDir`, `BookPath`, `WriteFile`, `ReadFile`, `Exists`, `ReadState`, `WriteState`, `RemoveBooks`) dan fungsi `Snake`, `CamelTag`. Badan minimal: `Dir` = `filepath.Join(dataDir, "grimoire", projectID)` dan helper path lain di atasnya; metode lain mengembalikan nilai nol/`errors.New("belum diimplementasi")`; `Snake`/`CamelTag` mengembalikan `s`.
3. **`scan.go` (stub).** `// STUB(foundation): ditimpa oleh scan-modules`. Definisikan `FileInfo` dan konstanta `maxFileBytes` (`1 << 20`), plus `func Scan(root string) ([]FileInfo, error)` yang mengembalikan `nil, nil`.
4. **`modules.go` (stub).** Stub yang sama. Definisikan `Module` (semua field overview 2.3 termasuk `Truncated`), konstanta `maxModuleFiles = 200`, `maxModuleBytes = 512 << 10`, `maxSymbols = 60`, dan `func GroupModules(files []FileInfo) []Module` yang mengembalikan `nil`.
5. **`hash.go` (stub).** `func HashModule(root string, m Module) (string, error)` mengembalikan `"", nil`.
6. **`pages.go` (stub).** `// STUB(foundation): ditimpa oleh pages-index`. Definisikan `Page`, `RenderChapter` (mengembalikan `"Sumber: " + strings.Join(m.Sources, ", ") + "\n"`), `RenderBooks` (mengembalikan `nil, nil`).
7. **`index.go` (stub).** Stub yang sama. `const IndexMaxBytes = 16384`, `const IndexHeader = "# Grimoire\n..."` (isi sementara memuat `{{grimoire_dir}}`), `IndexLine` dan `BuildIndex` bentuk minimal (`BuildIndex` mengembalikan `IndexHeader`).
8. **`build.go` (stub).** Stub yang sama. `func Build(store *Store, projectID, projectPath string) (Summary, error)` mengembalikan `Summary{}, nil`.
9. **`ipc.go` (stub).** `// STUB(foundation): ditimpa oleh ipc-ctl`. Import `context`, `log/slog`, `github.com/NooBeeID/shikigami/internal/ipc`. Definisikan `Deps` (overview 2.3) dan `func Register(srv *ipc.Server, d Deps) {}`.
10. **`openPage.ts` (stub).** `// STUB(foundation): ditimpa oleh readonly-pane`. `export async function openGrimoirePage(projectId: string, relPath: string): Promise<string> { throw new Error('belum diimplementasi'); }`. Tanpa impor.
11. **Verifikasi.** `go build ./...` lulus; `cd frontend && npm run build` lulus (jalankan `npm install` bila `node_modules` belum ada, tanpa mengubah `package.json`/lockfile); `git diff --stat -- go.mod go.sum frontend/package.json frontend/package-lock.json` kosong; tidak ada berkas di luar daftar di atas yang berubah; tidak ada definisi ganda (konstanta/sentinel hanya di satu berkas).

## Kriteria selesai

- Semua signature overview 2.2/2.3/2.6 yang melintasi fitur ada dan berkompilasi.
- Setiap stub diberi komentar `// STUB(foundation): ditimpa oleh <fitur>` di baris pertama (kecuali `types.go`).
- Build guard lulus. Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
