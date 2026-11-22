# Review diff perubahan agent per sesi: indeks tiket

Sumber: `docs/superpowers/specs/2026-11-22-agent-diff-review-design.md` (status draft, 2026-11-22).

## Ringkasan tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|-------|-------|------|------------|--------------|
| `agent-diff-review-ticket-01-review-capture` | Review: tangkap baseline dan daftar file berubah | 1 | - | 4 |
| `agent-diff-review-ticket-02-review-diff` | Review: diff unified per file | 2 | ticket-01 | 2 |
| `agent-diff-review-ticket-03-review-actions` | Review: revert, terima, dan deteksi perubahan lain | 3 | ticket-02 | 2 |
| `agent-diff-review-ticket-04-review-comment` | Review: komentar ke agent | 4 | ticket-03 | 2 |

Total: 4 tiket, 10 fitur.

Fitur per tiket:
- ticket-01: `review-foundation`, `review-hooks-ipc`, `review-ui-list`, `review-capture-wiring`
- ticket-02: `review-diff-engine`, `review-diff-ui`
- ticket-03: `review-actions-daemon`, `review-actions-ui`
- ticket-04: `review-comment-daemon`, `review-comment-ui`

## Urutan eksekusi per wave

Setiap wave hanya berisi satu tiket, sehingga tidak ada eksekusi paralel di dalam wave. Wave berikutnya menunggu wave sebelumnya selesai.

1. Wave 1: ticket-01 (review-capture)
2. Wave 2: ticket-02 (review-diff), setelah ticket-01
3. Wave 3: ticket-03 (review-actions), setelah ticket-02
4. Wave 4: ticket-04 (review-comment), setelah ticket-03

## Kontrak bersama

### IPC (daemon)

Modul baru `internal/review`. Wiring di `cmd/shikigamid/wire_review.go` lewat `RegisterModule`, `d.Lookup("status.engine")`, dan `eng.AddHookListener`, mengikuti pola `wire_runs.go`.

- `review.files {sessionId}` mengembalikan `{supported:bool, files:[{path (relatif cwd), status:'modified'|'added'|'deleted', additions, deletions, reviewed, external, binary, tooLarge}]}`. `additions` dan `deletions` bernilai 0 sampai tiket review-diff. `external` bernilai false sampai tiket review-actions.
- `review.diff {sessionId, path}` mengembalikan `{hunks:[{id, oldStart, oldLines, newStart, newLines, lines:[{kind:'ctx'|'add'|'del', text, oldNo, newNo}]}], currentHash, binary, tooLarge}`.
- `review.revert {sessionId, path, hunkIds?, expectHash, force?}`. Bila hash berbeda, error berupa `[CONFLICT] ...`.
- `review.accept {sessionId, path, hunkIds?}`.
- `review.comment {sessionId, path, line, text}`.
- Event `review.changed {sessionId, count}`, dengan `count` berarti jumlah file yang belum ditinjau. Event didebounce 300 ms.

### Penyimpanan

- Baseline di `DataDir/review/<sessionId>/baseline/<sha256(path)>`.
- Metadata di `DataDir/review/<sessionId>/index.json`.
- Retensi 20 sesi yang sudah ended.
- Semua path divalidasi berada di dalam cwd sesi dengan aturan `internal/fsx`.

### Hook

- Hook selalu exit 0 dan harus cepat.
- `ClassifyEvent` dan perilaku registry runs tidak boleh berubah.
- Matcher `PreToolUse` menjadi `Workflow|Agent|Edit|Write|MultiEdit|NotebookEdit`.

### Frontend

- Modul baru `frontend/src/modules/review`, mendaftar lewat `registerRightTab({id:'review', order setelah files})`.
- `store.ts` mengekspos store daftar file per sesi aktif dan count belum ditinjau. Count ini dipakai `ReviewBadge` di `SessionRow`.
- `DiffView` menyediakan slot aksi per hunk dan handler klik baris untuk tiket berikutnya.

### Batasan umum

- Tanpa dependency baru.
- Tanpa unit test.
- Guard verifikasi per tiket hanya `go build` dan build frontend.
