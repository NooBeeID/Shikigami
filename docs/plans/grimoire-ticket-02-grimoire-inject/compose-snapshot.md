# Plan: Susun teks suntikan dan snapshot per sesi

## Metadata

- Tiket: `grimoire-ticket-02-grimoire-inject` (`docs/tickets/grimoire-ticket-02-grimoire-inject.md`)
- Fitur: `compose-snapshot`
- ownedPaths: `internal/grimoire/inject.go`, `internal/grimoire/snapshot.go`
- Depends on foundation: ya. `00-foundation.md` sudah menaruh stub berkas ini (baris pertama `// STUB(foundation): ditimpa oleh compose-snapshot`) dengan signature final. Plan ini menimpa penuh kedua berkas.
- Sumber kebenaran kontrak: `overview.md` bagian 2.2 (koreksi 1 dan 2) dan 2.3. Plan ini tidak mengulangnya; hanya urutan kerja.

## Tujuan

Menyediakan empat fungsi untuk paket `internal/grimoire`: `ComposeInjection` (teks suntikan deterministik dari `index.md` + `laws.md`), `WriteSnapshot` (membekukan teks ke `<DataDir>/run/grimoire-<16 hex>.md`), `PruneSnapshots` (membersihkan snapshot usang dengan kebijakan aman), dan `TOMLString` (nilai string TOML basic untuk argumen `-c` Codex). Fitur lain (`launch-contribution`, `laws-measure-ipc`) hanya memanggil signature ini.

## Path yang dimiliki

HANYA boleh disentuh:
- `internal/grimoire/inject.go`
- `internal/grimoire/snapshot.go`

Dilarang menyentuh: berkas tiket 01 (`store.go`, `index.go`, `ipc.go`, `types.go`, dst.), `laws.go`, `measure.go`, apa pun di `cmd/`, frontend, `internal/session`, `internal/provider`, `internal/ipc`, `go.mod`/`go.sum`. Tanpa berkas baru lain dan tanpa unit test.

## Kontrak

Disediakan (signature final, JANGAN diubah; lihat overview 2.3):
```go
// inject.go
func ComposeInjection(projectID string) (text string, ok bool, err error)
// snapshot.go
func WriteSnapshot(text string) (path string, err error)
func PruneSnapshots(inUse func(path string) bool) error
func TOMLString(s string) string
```
Konstanta tak-diekspor milik `inject.go`: `injectLawsSeparator = "\n# Laws\n\n"`, `injectPlaceholder = "{{grimoire_dir}}"`. Helper tak-diekspor memakai awalan `inject*` / `snapshot*`.

Dipakai dari tiket 01 (baca kode NYATA di `internal/grimoire/` dulu; bila nama/signature beda dari ini, ikuti kode nyata dan catat di bagian catatan akhir plan ini):
- `NewStore(dataDir string) *Store`, `(*Store).Dir`, `ValidateProject`, `ReadFile(projectID, rel string) ([]byte, error)` (batas 4 MiB; tak ada berkas memenuhi `errors.Is(err, os.ErrNotExist)`), `BooksDir`.
- `IndexHeader` (di `index.go`), `ErrInvalidProject`.
- `paths.DataDir()` dari `github.com/NooBeeID/shikigami/internal/paths`.

Dipakai oleh fitur lain (jangan ubah perilakunya): `launch-contribution` memanggil keempat fungsi; `laws-measure-ipc` memanggil `ComposeInjection`.

## Langkah

1. **Baca kode nyata dulu (wajib).** Buka `internal/grimoire/index.go` dan baca `IndexHeader` nyata. Tentukan strategi pengganti placeholder:
   - Bila header menulis `{{grimoire_dir}}/books/<buku>/<bab>.md` (bentuk yang diharapkan dari plan tiket 01), nilai pengganti = `Store.Dir(projectID)`.
   - Bila header memakai `{{grimoire_dir}}` sebagai folder buku langsung (tanpa `/books` sesudahnya), nilai pengganti = `Store.BooksDir(projectID)`.
   Hasil akhir harus path absolut yang benar ke `books/<buku>/<bab>.md`. Implementasikan keputusan itu sebagai satu konstanta/ekspresi di `inject.go` (mis. fungsi `injectReplacement(s *Store, projectID string) string`) disertai komentar singkat alasan. Baca juga `store.go` untuk memastikan signature `ReadFile`/`Dir`/`ValidateProject`/`BooksDir`.
   Periksa juga bahwa tidak ada identifier `inject*`/`snapshot*` yang sudah dipakai berkas lain dalam paket (`grep -rn "func inject\|func snapshot\|injectLaws\|snapshot" internal/grimoire`).

2. **Tulis `internal/grimoire/inject.go`** (timpa stub, hapus baris `// STUB(foundation)`). Package `grimoire`; impor `errors`, `fmt`, `os`, `strings`, `github.com/NooBeeID/shikigami/internal/paths`. Komentar dan pesan galat bahasa Indonesia.
   - `const injectLawsSeparator = "\n# Laws\n\n"` dan `const injectPlaceholder = "{{grimoire_dir}}"`.
   - `func ComposeInjection(projectID string) (string, bool, error)`:
     1. `dataDir, err := paths.DataDir()`; galat -> `"", false, fmt.Errorf("gagal menentukan data dir: %w", err)`.
     2. `s := NewStore(dataDir)`; `if err := s.ValidateProject(projectID); err != nil { return "", false, err }` (project id tidak valid -> galat, bukan ok=false).
     3. `idx, err := s.ReadFile(projectID, "index.md")`. Bila `errors.Is(err, os.ErrNotExist)` -> `"", false, nil`. Galat lain -> `"", false, fmt.Errorf("gagal membaca index.md: %w", err)`. Pakai `Store.ReadFile`, BUKAN `ReadLaws` (tidak boleh bergantung pada `laws.go`).
     4. `text := strings.ReplaceAll(string(idx), injectPlaceholder, <pengganti dari langkah 1>)`.
     5. `laws, err := s.ReadFile(projectID, "laws.md")`. `os.ErrNotExist` -> lewati bagian laws. Galat lain -> `"", false, fmt.Errorf("gagal membaca laws.md: %w", err)` (pemanggil mencatat dan tidak menyuntik). Bila ada dan `strings.TrimSpace(string(laws)) != ""` -> `text += injectLawsSeparator + string(laws)` (isi apa adanya, tidak di-trim, tidak ditambah newline). Kosong/hanya spasi -> lewati seluruh bagian (tanpa pemisah).
     6. `return text, true, nil`.
   - Deterministik: tanpa tanggal, waktu, acak, iterasi map, atau nilai dinamis selain isi berkas dan path project.

3. **Tulis `internal/grimoire/snapshot.go`** (timpa stub, hapus baris STUB). Impor `crypto/rand`, `encoding/hex`, `fmt`, `os`, `path/filepath`, `strings`, `sync`, `time`, `unicode/utf8`, `github.com/NooBeeID/shikigami/internal/paths`. Pola mengikuti `writeMCPConfig` di `internal/provider/claude.go`.
   - State paket: `var snapshotMu sync.Mutex` dan `var snapshotWritten = map[string]struct{}{}` (path hasil `filepath.Clean`), `const snapshotMaxAge = 24 * time.Hour`, `const snapshotPrefix = "grimoire-"`, `const snapshotSuffix = ".md"`.
   - Helper `snapshotRunDir() (string, error)`: `paths.DataDir()` lalu `filepath.Join(base, "run")`.
   - `func WriteSnapshot(text string) (string, error)`:
     1. `dir` dari `snapshotRunDir`; `os.MkdirAll(dir, 0o700)`.
     2. 8 byte acak `crypto/rand` -> nama `grimoire-<hex 16 karakter>.md`.
     3. `os.WriteFile(path, []byte(text), 0o600)`, lalu `os.Chmod(path, 0o600)` (setiap galat dibungkus `fmt.Errorf("gagal ... %s: %w", ...)`). Bila Chmod gagal, hapus berkas (`os.Remove`, abaikan galat) sebelum mengembalikan galat.
     4. Catat path (Clean) di `snapshotWritten` di bawah `snapshotMu`; kembalikan path.
   - `func PruneSnapshots(inUse func(path string) bool) error` (aturan overview 2.3 adalah satu-satunya definisi):
     1. `os.ReadDir(dir)`; `os.IsNotExist`/`errors.Is(err, os.ErrNotExist)` -> `nil`; galat lain dikembalikan.
     2. Untuk tiap entri: lewati bila `e.IsDir()`, nama tidak berawalan `grimoire-` atau tidak berakhiran `.md`, atau `e.Type()` bukan berkas biasa (symlink dan lainnya dilewati; gunakan `e.Type().IsRegular()`).
     3. `info, err := e.Info()` (galat -> catat sebagai galat pertama bila belum ada, lanjut). Lewati bila `time.Since(info.ModTime()) <= snapshotMaxAge` (snapshot muda tidak pernah dihapus).
     4. Pemakaian: bila `inUse != nil` -> lewati bila `inUse(path)` true; bila `inUse == nil` -> lewati bila path ada di `snapshotWritten` (baca di bawah `snapshotMu`, kunci dilepas sebelum I/O).
     5. `os.Remove(path)`; gagal -> simpan galat pertama saja (`fmt.Errorf("gagal menghapus %s: %w", path, err)`) dan lanjut ke berkas berikutnya. Setelah semua dicoba kembalikan galat pertama (atau nil). Setelah penghapusan sukses, hapus path dari `snapshotWritten`.
   - `func TOMLString(s string) string`: bangun dengan `strings.Builder`, mulai dengan `"`.
     - Iterasi `for i := 0; i < len(s); { r, size := utf8.DecodeRuneInString(s[i:]) ...}`. Bila `r == utf8.RuneError && size == 1` (UTF-8 tidak valid) -> tulis `�` sebagai karakter literal `�` (U+FFFD) dan lanjut.
     - Escape: `\\` -> `\\\\`, `"` -> `\"`, `\n` -> `\n`, `\r` -> `\r`, `\t` -> `\t`, `\b` (0x08) -> `\b`, `\f` (0x0C) -> `\f`.
     - Karakter kontrol lain (< 0x20) dan DEL (0x7F) -> `fmt.Fprintf(&b, "\\u%04X", r)`.
     - Selain itu tulis rune apa adanya (UTF-8 valid dibiarkan). Akhiri dengan `"`. Hasil adalah string TOML basic satu baris valid (tanpa newline mentah), siap dipakai sebagai nilai `-c developer_instructions=...`.

4. **Verifikasi.** `gofmt -l internal/grimoire/inject.go internal/grimoire/snapshot.go` kosong; `go build ./...` lulus; `go vet ./internal/grimoire/...` bila cepat. Bila build gagal karena berkas fitur lain (bukan milikmu), jangan perbaiki; laporkan. Pastikan tidak ada baris `STUB(foundation)` tersisa di kedua berkas, tidak ada definisi ganda dengan berkas lain, dan `git diff --stat -- go.mod go.sum` kosong.

## Perubahan

- Diubah (menimpa stub Foundation): `internal/grimoire/inject.go`
- Diubah (menimpa stub Foundation): `internal/grimoire/snapshot.go`
- Tidak ada berkas lain yang dibuat atau diubah.

## Wiring

Fitur ini hanya mengekspos fungsi; JANGAN menyambungkannya sendiri.
- `ComposeInjection`, `WriteSnapshot`, `PruneSnapshots`, `TOMLString`: dipanggil oleh `cmd/shikigamid/wire_grimoire_inject.go` (fitur `launch-contribution`). Claude memakai path hasil `WriteSnapshot`; Codex memakai `TOMLString(text)` dan tetap menulis snapshot untuk audit. `PruneSnapshots(nil)` aman dipanggil saat modul siap dan setelah tiap `WriteSnapshot`.
- `ComposeInjection`: juga dipanggil `MeasureInjection` (fitur `laws-measure-ipc`); `IndexBytes` = `len(text)` dari fungsi ini.
- Tidak ada perubahan di `ipc.go`, `wire_grimoire.go`, `cmd_grimoire.go`, atau frontend dari fitur ini.

## Selesai bila

- `inject.go` dan `snapshot.go` bebas dari `STUB(foundation)` dan signature persis sesuai overview 2.3.
- `ComposeInjection`: tanpa `index.md` -> `ok=false` tanpa galat; project id tak valid -> galat; placeholder `{{grimoire_dir}}` terganti sehingga path ke `books/<buku>/<bab>.md` absolut dan benar (sesuai pengecekan `IndexHeader` nyata); laws ditambahkan dengan pemisah `"\n# Laws\n\n"` dan isi apa adanya hanya bila tidak kosong/bukan hanya spasi; dua pemanggilan berturut-turut byte-identik.
- `WriteSnapshot`: `run/` 0700, berkas `grimoire-<16 hex>.md` 0600 (plus Chmod) berisi teks persis, path tercatat di daftar in-memory.
- `PruneSnapshots`: hanya menghapus `grimoire-*.md` berkas biasa berumur > 24 jam yang tidak dipakai (`inUse` false, atau tidak tercatat di daftar proses bila `inUse == nil`); `run/` tidak ada -> nil; galat pertama dikembalikan setelah semua dicoba.
- `TOMLString` meng-escape `\\ \" \n \r \t \b \f` dan `\u00XX` untuk kontrol lain/DEL; UTF-8 tidak valid diganti `�`.
- Guard: `go build ./...` sukses (khususnya path fitur ini); tanpa unit test; tanpa dependency baru; tidak ada berkas di luar ownedPaths yang berubah.
