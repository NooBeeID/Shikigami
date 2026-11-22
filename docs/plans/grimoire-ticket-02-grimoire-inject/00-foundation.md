# Foundation: grimoire-ticket-02-grimoire-inject (stub berkompilasi)

Baca dulu `overview.md` (bagian 2 adalah sumber kebenaran; plan ini hanya urutan kerja) dan tiket `docs/tickets/grimoire-ticket-02-grimoire-inject.md`. Tanpa logika nyata, tanpa dependency baru, tanpa unit test.

Tujuan: setelah foundation, keempat fitur dapat dikerjakan paralel karena semua signature lintas-fitur sudah ada dan `go build ./...` lulus.

Berkas yang dibuat (semua di `internal/grimoire/`):
- `inject.go`, `snapshot.go`, `laws.go`, `measure.go` (stub)

## Langkah

1. **Prasyarat.** Pastikan tiket 01 sudah ada: `internal/grimoire/store.go` ada dan tidak berawalan `// STUB(foundation)`, `index.go` mendefinisikan `IndexHeader`, `ipc.go` mendefinisikan `Register(srv *ipc.Server, d Deps)`. Baca signature nyata `Store` (`Dir`, `ReadFile`, `WriteFile`, `LawsPath`, `BooksDir`, `NewStore`). Bila prasyarat tidak terpenuhi, berhenti dan laporkan (jangan menulis stub).
2. **`inject.go`.** Baris pertama `// STUB(foundation): ditimpa oleh compose-snapshot`. Package `grimoire`. `func ComposeInjection(projectID string) (string, bool, error) { return "", false, nil }`.
3. **`snapshot.go`.** `// STUB(foundation): ditimpa oleh compose-snapshot`. `func WriteSnapshot(text string) (string, error) { return "", errors.New("belum diimplementasi") }`, `func PruneSnapshots(inUse func(path string) bool) error { return nil }`, `func TOMLString(s string) string { return strconv.Quote(s) }`.
4. **`laws.go`.** `// STUB(foundation): ditimpa oleh laws-measure-ipc`. `const MaxLawsBytes = 16 * 1024`, `var ErrLawsTooLarge = errors.New("laws melebihi batas ukuran")`, `func (s *Store) ReadLaws(projectID string) (string, error) { return "", nil }`, `func (s *Store) WriteLaws(projectID, content string) error { return errors.New("belum diimplementasi") }`.
5. **`measure.go`.** `// STUB(foundation): ditimpa oleh laws-measure-ipc`. Import `context`, `log/slog`, `github.com/NooBeeID/shikigami/internal/ipc`. Definisikan persis `type Measure struct{ IndexBytes, EstTokens, Pages, LawsBytes int }`, `func MeasureInjection(projectID string) (Measure, error) { return Measure{}, nil }`, `type InjectDeps struct { Store *Store; Known func(ctx context.Context, projectID string) (bool, error); Logger *slog.Logger }`, `func RegisterInject(srv *ipc.Server, d InjectDeps) {}`. Pastikan tidak ada impor tak terpakai (pakai `_ =` bila perlu).
6. **Verifikasi.** `go build ./...` lulus; `gofmt -l internal/grimoire` kosong; `git diff --stat -- go.mod go.sum` kosong; hanya empat berkas di atas yang baru; tidak ada definisi ganda dengan berkas tiket 01 (terutama nama `Register` TIDAK dipakai ulang, gunakan `RegisterInject`); tidak ada perubahan di `cmd/` atau frontend.

## Kriteria selesai

- Semua signature overview 2.3 yang melintasi fitur ada dan berkompilasi.
- Tiap stub diberi komentar `// STUB(foundation): ditimpa oleh <fitur>` di baris pertama.
- Build guard lulus. Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
