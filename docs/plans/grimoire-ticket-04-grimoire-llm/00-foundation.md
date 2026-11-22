# Foundation: grimoire-ticket-04-grimoire-llm (stub berkompilasi)

Baca dulu `overview.md` (bagian 2 adalah sumber kebenaran; plan ini hanya urutan kerja) dan tiket `docs/tickets/grimoire-ticket-04-grimoire-llm.md`. Tanpa logika nyata, tanpa dependency baru, tanpa unit test.

Tujuan: setelah foundation, ketiga fitur dapat dikerjakan paralel karena semua signature lintas-fitur sudah ada dan `go build ./...` lulus.

Berkas yang dibuat (semua di `internal/grimoire/`):
- `settings.go`, `estimate.go`, `summarize.go`, `prompt.go` (stub)

## Langkah

1. **Prasyarat.** Pastikan tiket 01-03 sudah ada: `store.go` (`Store`, `NewStore`, `WriteFile`, `ReadFile`, `ReadState`, `ValidateProject`) bukan stub; `modules.go` (`Module`), `scan.go` (`Scan`), `pages.go` (`RenderBooks`), `ipc.go` (`Register`, `statusResult`); tiket 03: `diff.go` (`ComputeDiff`, `Diff.Pending`) dan `incremental.go` (`Summarizer`, `IncrementalOptions`, `BuildIncremental`, `HashAll`) tanpa baris `// STUB(foundation)`; tiket 02: `measure.go` ada. Baca signature nyata dan bandingkan dengan overview bagian 2.3; bila berbeda kecil, catat dan sesuaikan stub tanpa mengubah arti. Bila prasyarat tidak terpenuhi, berhenti dan laporkan (jangan menulis stub).
2. **`settings.go`.** Baris pertama `// STUB(foundation): ditimpa oleh settings-estimate`. Import `context`, `errors`, `log/slog`, `internal/ipc`. Definisikan persis `Settings`, `ErrInvalidMaxModules`, `LLMDeps`, serta `func LoadSettings(projectID string) (Settings, error) { return Settings{}, nil }`, `func SaveSettings(projectID string, s Settings) error { return errors.New("belum diimplementasi") }`, `func RegisterLLM(srv *ipc.Server, d LLMDeps) {}`.
3. **`estimate.go`.** `// STUB(foundation): ditimpa oleh settings-estimate`. Definisikan persis `Estimate` (termasuk `OverLimit` `omitempty`), `func EstimateBuild(projectID, projectPath string) (Estimate, error) { return Estimate{}, errors.New("belum diimplementasi") }`, `func EstTokensFromBytes(n int) int { return (n + 3) / 4 }`. Cek `measure.go`: bila sudah ada helper token setara yang diekspor, `EstTokensFromBytes` memanggilnya.
4. **`prompt.go`.** `// STUB(foundation): ditimpa oleh llm-summarizer`. `const LLMMaxSourceBytes = 48 * 1024`, `const LLMExpectedOutputBytes = 3 * 1024`, `func LLMPromptOverheadBytes() int { return 1024 }`. Fungsi tak-diekspor `promptBuild` TIDAK dibuat di stub (milik fitur).
5. **`summarize.go`.** `// STUB(foundation): ditimpa oleh llm-summarizer`. Definisikan persis `BuildCost` (termasuk `Skipped`, `Note` `omitempty`), `LLMProgress`, `ErrLLMOverLimit`, `func LoadBuildCost(projectID string) (*BuildCost, error) { return nil, nil }`, `func ProgressFor(projectID string) *LLMProgress { return nil }`, `func SetLLMRuntime(d LLMDeps) {}`, `func SummarizerFor(projectID, projectPath string) Summarizer { return nil }`, `func NewLLMSummarizer(deps LLMDeps, projectID, projectPath string, s Settings) Summarizer { return nil }`. Pastikan tidak ada impor tak terpakai (pakai `_ =` bila perlu) dan `Summarizer` yang dikembalikan stub adalah nil murni.
6. **Verifikasi.** `go build ./...` lulus; `gofmt -l internal/grimoire` kosong; `git diff --stat -- go.mod go.sum` kosong; hanya empat berkas di atas yang baru; tidak ada definisi ganda dengan berkas tiket 01-03 (cek nama `Settings`, `Estimate`, `BuildCost`, `LLMDeps`, `Result` tidak bentrok; bila bentrok, berhenti dan laporkan); tidak ada perubahan di `cmd/` atau frontend; `incremental.go`/`ipc.go` tidak disentuh (hook dan field status adalah Wiring).

## Kriteria selesai

- Semua signature overview 2.3 untuk empat berkas ada dan berkompilasi.
- Tiap stub diberi komentar `// STUB(foundation): ditimpa oleh <fitur>` di baris pertama.
- Build guard lulus. Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
