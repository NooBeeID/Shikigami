# Foundation: grimoire-ticket-03-grimoire-refresh (stub berkompilasi)

Baca dulu `overview.md` (bagian 2 adalah sumber kebenaran; plan ini hanya urutan kerja) dan tiket `docs/tickets/grimoire-ticket-03-grimoire-refresh.md`. Tanpa logika nyata, tanpa dependency baru, tanpa unit test.

Tujuan: setelah foundation, ketiga fitur dapat dikerjakan paralel karena semua signature lintas-fitur sudah ada dan `go build ./...` lulus.

Berkas yang dibuat (semua di `internal/grimoire/`):
- `diff.go`, `incremental.go` (stub)

## Langkah

1. **Prasyarat.** Pastikan tiket 01 dan 02 sudah ada: `internal/grimoire/store.go` ada dan tidak berawalan `// STUB(foundation)`, `index.go` mendefinisikan `IndexHeader`/`BuildIndex`, `pages.go` mendefinisikan `RenderBooks`, `scan.go`/`modules.go`/`hash.go` mendefinisikan `Scan`/`GroupModules`/`HashModule`/`Module`, `ipc.go` mendefinisikan `Register(srv *ipc.Server, d Deps)`, `inject.go` ada. Baca signature nyata dan bandingkan dengan overview bagian 2.3; bila berbeda kecil, catat dan sesuaikan stub tanpa mengubah arti. Bila prasyarat tidak terpenuhi, berhenti dan laporkan (jangan menulis stub).
2. **`diff.go`.** Baris pertama `// STUB(foundation): ditimpa oleh diff-incremental`. Package `grimoire`. Definisikan persis `ChangeKind` dan konstanta `ChangeUnchanged`, `ChangeAdded`, `ChangeModified`, `ChangeRemoved` (iota, urutan itu), `ModuleChange`, `Diff` (termasuk field `Stale []ModuleChange`), serta `func ComputeDiff(scanned []Module, hashes map[string]string, st *State) Diff { return Diff{} }`, `func (d Diff) Pending() []ModuleChange { return nil }`, `func (d Diff) Empty() bool { return true }`.
3. **`incremental.go`.** `// STUB(foundation): ditimpa oleh diff-incremental`. Import `context`, `errors`, `log/slog`, `time`. Definisikan persis `Summarizer`, `IncrementalOptions` (field `ProjectPath`, `Full`, `Summarizer`), `Result`, `Builder` (struct dengan field `store *Store` dan `logger *slog.Logger` saja; fitur pemilik boleh menambah field lain), `NewBuilder`, `(*Builder).Store()`, `(*Builder).Check(projectID, projectPath string) (Diff, error)` (kembalikan `Diff{}, errors.New("belum diimplementasi")`), `(*Builder).BuildIncremental(ctx, projectID, opts) (Result, error)` (kembalikan `Result{}, errors.New("belum diimplementasi")`), `HashAll`, `ApplyStale` (kembalikan `books` apa adanya), `StaleCount` (kembalikan 0). Pastikan tidak ada impor tak terpakai (pakai `_ =` bila perlu).
4. **Verifikasi.** `go build ./...` lulus; `gofmt -l internal/grimoire` kosong; `git diff --stat -- go.mod go.sum` kosong; hanya dua berkas di atas yang baru; tidak ada definisi ganda dengan berkas tiket 01/02 (cek nama `Builder`, `Result`, `Diff` tidak bentrok; bila bentrok, berhenti dan laporkan); tidak ada perubahan di `cmd/` atau frontend.

## Kriteria selesai

- Semua signature overview 2.3 untuk `diff.go` dan `incremental.go` ada dan berkompilasi.
- Tiap stub diberi komentar `// STUB(foundation): ditimpa oleh diff-incremental` di baris pertama.
- Build guard lulus. Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
