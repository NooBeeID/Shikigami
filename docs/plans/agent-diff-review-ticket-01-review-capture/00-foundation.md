# Foundation: agent-diff-review-ticket-01-review-capture

Baca `overview.md` (terutama 2.2 dan 2.3) lebih dulu. Tujuan: paket `internal/review` berkompilasi dengan tipe final dan stub `Store` supaya `review-foundation`, `review-hooks-ipc`, dan `review-ui-list` dapat dikerjakan paralel. Guard: `go build ./...`. Tanpa unit test, tanpa dependency baru.

## Langkah

1. Buat `internal/review/types.go` (package `review`) persis sesuai blok `types.go` di overview 2.3: `Status` + tiga konstanta, `maxBaselineBytes`, `HashAbsent`, `IPCMethodFiles`, `EventChanged`, `FileEntry`, `TouchedFile`, `FilesResult`. Komentar bahasa Indonesia. Boleh diberi komentar paket: penyimpanan di `<DataDir>/review/<sessionId>/baseline/<sha256(pathRelatif)>` dan `index.json`.
2. Buat `internal/review/store.go` berisi stub:
   - `type Store struct{ dataDir string }` dan `func NewStore(dataDir string) *Store`.
   - Method (signature final, overview 2.3): `Touched`, `Files`, `Prune(keepEnded int, activeSessionIDs []string)`, `Cwd`, `SessionDir`, `BaselinePath`, `HashPath`. Badan stub: `Touched` mengembalikan `[]TouchedFile{}`; `Files` mengembalikan `[]FileEntry{}, nil`; `Prune` kosong; `Cwd` mengembalikan `""`; `SessionDir` mengembalikan `filepath.Join(s.dataDir, "review", sessionID)`; `BaselinePath` mengembalikan `filepath.Join(s.SessionDir(sessionID), "baseline", s.HashPath(relPath))`; `HashPath` mengembalikan sha256 hex (`crypto/sha256`, `encoding/hex`) dari `relPath` (dua method terakhir langsung diimplementasi benar karena sederhana).
3. Buat `internal/review/capture.go` berisi stub `func (s *Store) Capture(sessionID, cwd, path string) error` dan `func (s *Store) MarkChanged(sessionID, cwd, path string) error` yang mengembalikan `nil`. Tambahkan komentar `// TODO review-foundation: implementasi penuh`.
4. Jalankan `go build ./...` dari root repo; harus lulus. Perbaiki impor yang tak terpakai (`_ = ` bila perlu).
5. Jangan membuat `ipc.go`, `wire_review.go`, ataupun file frontend; itu milik fitur paralel.

## Selesai bila

- `go build ./...` lulus.
- `internal/review` mengekspor semua simbol di overview 2.3 dengan signature persis.
