# Plan: Laws dan pengukuran ukuran index

## Metadata

- Tiket: `grimoire-ticket-02-grimoire-inject` (`docs/tickets/grimoire-ticket-02-grimoire-inject.md`)
- Fitur: `laws-measure-ipc` (Laws dan pengukuran ukuran index)
- ownedPaths: `internal/grimoire/laws.go`, `internal/grimoire/measure.go`
- Depends on foundation: ya. `00-foundation.md` sudah membuat stub kedua berkas ini dengan signature final; plan ini MENIMPA penuh isinya (hapus baris `// STUB(foundation): ditimpa oleh laws-measure-ipc`).
- Rujukan wajib: `overview.md` (bagian 2.2 koreksi, 2.3 signature, 2.4 IPC, 7 aturan paralel). Jangan mengulang isinya; plan ini hanya urutan kerja.

## Tujuan

Menyediakan (1) baca/tulis `laws.md` per project dengan batas 16 KiB, (2) pengukuran ukuran teks suntikan (`MeasureInjection`), dan (3) dua method IPC `grimoire.laws.get` / `grimoire.laws.set` lewat `RegisterInject`, termasuk publish `grimoire.changed` setelah set sukses. Perluasan `grimoire.status` BUKAN bagian fitur ini (dikerjakan di Wiring); `RegisterInject` tidak boleh mendaftarkan `grimoire.status` karena `ipc.Server.Register` panik bila method ganda.

## Path yang dimiliki

HANYA boleh menyentuh:
- `internal/grimoire/laws.go`
- `internal/grimoire/measure.go`

Dilarang: `inject.go`, `snapshot.go` (milik `compose-snapshot`), `ipc.go`/`store.go`/`index.go` dan berkas tiket 01 lainnya, `cmd/**` (termasuk `wire_grimoire_inject.go`), frontend, `internal/ipc`, `internal/session`, `internal/provider`, `go.mod`/`go.sum`. Jangan membuat berkas lain. Bila build gagal karena berkas fitur lain, jangan diperbaiki; laporkan.

## Kontrak

Dipakai (tidak boleh diubah):
- Tiket 01: `Store` dengan `NewStore(dataDir)`, `Dir`, `ValidateProject`, `LawsPath`, `BooksDir`, `WriteFile(projectID, rel string, data []byte) error` (atomik, `rel` relatif terhadap `Store.Dir`), `ReadFile(projectID, rel string) ([]byte, error)` (galat tak-ada memenuhi `errors.Is(err, os.ErrNotExist)`), `ErrInvalidProject`, `ErrInvalidPath`. Baca kode nyata `internal/grimoire/store.go` lebih dulu; bila signature nyata berbeda, ikuti kode nyata dan catat di laporan akhir.
- Fitur `compose-snapshot`: `func ComposeInjection(projectID string) (text string, ok bool, err error)` di `inject.go` (stub Foundation: `"", false, nil`; implementasi nyata menyusul). Jangan membaca/menyusun teks suntikan sendiri.
- `paths.DataDir() (string, error)` dari `internal/paths` untuk membuat `Store` di `MeasureInjection`.
- `internal/ipc`: `*ipc.Server` dengan `Register(method string, h ipc.Handler)` dan `Publish(topic string, payload any)`; handler bertanda tangan `func(ctx context.Context, params json.RawMessage) (any, error)`; `ipc.NewError(code, msg)`, kode `ipc.ErrInvalid` (-32003) dan `ipc.ErrInternal` (-32603).

Disediakan (signature final, overview 2.3 dan 2.4):
```go
// laws.go
const MaxLawsBytes = 16 * 1024
var ErrLawsTooLarge = errors.New("laws melebihi batas ukuran")
func (s *Store) ReadLaws(projectID string) (string, error)
func (s *Store) WriteLaws(projectID, content string) error

// measure.go
type Measure struct{ IndexBytes, EstTokens, Pages, LawsBytes int }
func MeasureInjection(projectID string) (Measure, error)
type InjectDeps struct {
    Store  *Store
    Known  func(ctx context.Context, projectID string) (bool, error) // nil = lewati pengecekan
    Logger *slog.Logger                                              // nullable
}
func RegisterInject(srv *ipc.Server, d InjectDeps)
```
IPC: `grimoire.laws.get {projectId} -> {content}`; `grimoire.laws.set {projectId, content} -> {lawsBytes}`; event `grimoire.changed {projectId}` setelah set sukses. Nama method dan topik harus persis seperti ini (dipakai `lawsStore.ts` dan tiket 01).

## Langkah

1. **Prasyarat.** Buka `internal/grimoire/store.go`, `inject.go` (stub/impl), `ipc.go` (lihat gaya decode params dan pola `Deps`), `internal/fsx/ipc.go` (pola `decode`/`toIPCError`). Pastikan stub Foundation `laws.go` dan `measure.go` ada. Bila tidak, berhenti dan laporkan.

2. **Tulis ulang `laws.go`** (package `grimoire`; tanpa baris STUB). Impor: `errors`, `fmt`, `os`.
   - `const MaxLawsBytes = 16 * 1024` dan `var ErrLawsTooLarge = errors.New("laws melebihi batas ukuran")`.
   - `ReadLaws(projectID)`: `b, err := s.ReadFile(projectID, "laws.md")`; bila `errors.Is(err, os.ErrNotExist)` kembalikan `"", nil`; galat lain (termasuk `ErrInvalidProject`) kembalikan `"", err`; sukses `string(b), nil`. Isi apa adanya, tanpa trim.
   - `WriteLaws(projectID, content)`: bila `len(content) > MaxLawsBytes` kembalikan `ErrLawsTooLarge` (cek ukuran SEBELUM menyentuh disk; `len` = byte, bukan rune). Selain itu `return s.WriteFile(projectID, "laws.md", []byte(content))`. `WriteFile` sudah atomik dan membuat direktori project; jangan menulis temp+rename sendiri. Konten kosong sah (menulis berkas kosong). Tidak ada normalisasi.
   - Helper tak-diekspor, bila ada, diawali `laws`.

3. **Tulis ulang `measure.go`** (tanpa baris STUB). Impor: `context`, `encoding/json`, `errors`, `io/fs`, `log/slog`, `os`, `path/filepath`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`, `github.com/NooBeeID/shikigami/internal/paths` (sesuaikan dengan yang benar-benar dipakai).
   - `type Measure struct{ IndexBytes, EstTokens, Pages, LawsBytes int }` persis, tanpa tag JSON tambahan.
   - `MeasureInjection(projectID string) (Measure, error)`:
     1. `dir, err := paths.DataDir()`; galat -> `Measure{}, err`. `s := NewStore(dir)`.
     2. `text, ok, err := ComposeInjection(projectID)`; err -> `Measure{}, err`; `!ok` -> `Measure{}, nil` (tanpa grimoire: semuanya nol, galat nil).
     3. `IndexBytes = len(text)`; `EstTokens = (len(text)+3)/4`.
     4. `LawsBytes`: baca `laws.md` lewat `s.ReadFile(projectID, "laws.md")`; tak-ada (`os.ErrNotExist`) -> 0; galat lain -> kembalikan galat; sukses `len(b)`. (Panjang berkas mentah, bukan hasil trim.)
     5. `Pages`: `filepath.WalkDir(s.BooksDir(projectID), ...)` menghitung berkas reguler berakhiran `.md` (`strings.HasSuffix(strings.ToLower(name), ".md")`, bukan direktori, bukan symlink). Bila `books/` tidak ada (`errors.Is(err, fs.ErrNotExist)`) -> 0 tanpa galat; galat walk lain dikembalikan.
     6. Kembalikan `Measure{...}, nil`.
   - `InjectDeps` persis seperti kontrak.
   - `RegisterInject(srv *ipc.Server, d InjectDeps)`:
     - Definisikan tipe params lokal berawalan `measure`: `measureLawsGetParams{ProjectID string \`json:"projectId"\`}` dan `measureLawsSetParams{ProjectID string \`json:"projectId"\`; Content string \`json:"content"\`}`.
     - Helper `measureDecode(params json.RawMessage, v any) error`: params kosong -> `ipc.NewError(ipc.ErrInvalid, "invalid params: kosong")`; unmarshal gagal -> `ipc.NewError(ipc.ErrInvalid, "invalid params: "+err.Error())`.
     - Helper `measureCheckProject(ctx, d, id) error`: `id == ""` -> `ErrInvalid` ("projectId kosong"); `d.Store.ValidateProject(id)` gagal -> `ErrInvalid` ("projectId tidak valid"); bila `d.Known != nil`: `ok, err := d.Known(ctx, id)`; err -> `ErrInternal` (catat log bila `d.Logger != nil`); `!ok` -> `ErrInvalid` ("project tidak dikenal"). Urutan pengecekan: kosong, valid, Known.
     - Helper `measureMapErr(err error) error`: sudah `*ipc.Error` -> kembalikan apa adanya; `errors.Is(err, ErrLawsTooLarge)` -> `ipc.NewError(ipc.ErrInvalid, "laws melebihi batas 16384 byte")`; `errors.Is(err, ErrInvalidProject)` atau `ErrInvalidPath` -> `ErrInvalid`; lain -> `ErrInternal` dengan `err.Error()` (log bila Logger ada).
     - Daftarkan `grimoire.laws.get`: decode, cek project, `content, err := d.Store.ReadLaws(id)`, hasil `map[string]any{"content": content}` (string kosong bila tak ada berkas; bukan galat).
     - Daftarkan `grimoire.laws.set`: decode, cek project, `d.Store.WriteLaws(id, p.Content)`; galat dipetakan via `measureMapErr`; sukses `srv.Publish("grimoire.changed", map[string]any{"projectId": id})` lalu hasil `map[string]any{"lawsBytes": len(p.Content)}`.
     - JANGAN mendaftarkan `grimoire.status` atau method lain. Panik bila `d.Store == nil` tidak perlu ditangani khusus, tetapi `RegisterInject` boleh langsung `return` bila `srv == nil || d.Store == nil`.
   - Catatan: handler tidak memakai `MeasureInjection` (itu dipakai Wiring di handler status tiket 01).

4. **Verifikasi.** `gofmt -l internal/grimoire` kosong; `go build ./...` lulus (opsional `go vet ./internal/grimoire/...`); tidak ada identifier duplikat dengan berkas lain di paket (`Register` tidak dipakai; `Measure*`, `InjectDeps`, `MaxLawsBytes`, `ErrLawsTooLarge` hanya di berkas ini); `git diff --stat -- go.mod go.sum` kosong; `git status` menunjukkan hanya `laws.go` dan `measure.go` yang diubah oleh fitur ini.

## Perubahan

- Diubah (timpa stub Foundation): `internal/grimoire/laws.go`
- Diubah (timpa stub Foundation): `internal/grimoire/measure.go`
- Tidak ada berkas lain dibuat atau diubah.

## Wiring

Yang diekspos untuk disambungkan nanti (JANGAN disambungkan di fitur ini):
- `grimoire.RegisterInject(srv, grimoire.InjectDeps{Store, Known, Logger})`: dipanggil oleh `cmd/shikigamid/wire_grimoire_inject.go` (fitur `launch-contribution`) di `d.OnReady`, dengan `Known` dari daftar project workspace.
- `grimoire.MeasureInjection(projectID)`: dipakai langkah Wiring di handler `grimoire.status` pada `internal/grimoire/ipc.go` (tiket 01) untuk mengisi `indexBytes`, `estTokens`, `pages`, `lawsBytes`.
- Method IPC `grimoire.laws.get/set` dan event `grimoire.changed` dipakai `lawsStore.ts` (fitur `laws-ui`).
- Tiket 01 tidak boleh sudah mendaftarkan `grimoire.laws.*`; Wiring memeriksa tidak ada pendaftaran ganda (server panik bila ganda).

## Selesai bila

- `laws.go` dan `measure.go` tidak lagi berisi penanda `STUB(foundation)` dan mengimplementasikan seluruh langkah di atas dengan signature persis kontrak.
- `WriteLaws` menolak `len(content) > 16384` dengan `ErrLawsTooLarge` tanpa menyentuh disk; `ReadLaws` mengembalikan `""` bila berkas tak ada.
- `MeasureInjection` mengembalikan `Measure{}, nil` tanpa grimoire; `EstTokens = (IndexBytes+3)/4`.
- `RegisterInject` hanya mendaftarkan `grimoire.laws.get` dan `grimoire.laws.set`, memetakan galat sesuai overview 2.4, dan mem-publish `grimoire.changed` setelah set sukses.
- Guard: `go build ./...` sukses (tanpa unit test); `gofmt -l internal/grimoire` kosong; hanya dua path milik fitur ini yang berubah.
