# Plan: Tipe dan layout penyimpanan grimoire

## Metadata

- Tiket: `grimoire-ticket-01-grimoire-build` (`docs/tickets/grimoire-ticket-01-grimoire-build.md`)
- Fitur: `contract-types`
- ownedPaths: `internal/grimoire/types.go`, `internal/grimoire/store.go`
- Depends on foundation: ya. `types.go` (final) dan stub `store.go` (baris pertama `// STUB(foundation): ditimpa oleh contract-types`) dibuat oleh Foundation (`00-foundation.md`). Bila `internal/grimoire/types.go` ternyata belum ada saat kamu mulai, buat sesuai overview 2.2 (jangan ada definisi ganda dengan berkas lain di paket).
- Rujukan wajib: `overview.md` bagian 2.2, 2.3, 6, 7; tiket bagian fitur `contract-types`. Plan ini tidak mengulang isi overview.

## Tujuan

Menyediakan fondasi penyimpanan grimoire: tipe data final (`types.go`) dan implementasi penuh `Store` (`store.go`) yang menulis/membaca HANYA di bawah `<dataDir>/grimoire/<projectId>/`, secara atomik, dengan penolakan path keluar direktori, plus helper nama `Snake` dan `CamelTag`. Fitur lain (`scan-modules`, `pages-index`, `ipc-ctl`) mengimpor signature ini, jadi signature TIDAK boleh berubah dari overview 2.3.

## Path yang dimiliki

HANYA berkas ini yang boleh disentuh:

- `internal/grimoire/types.go`
- `internal/grimoire/store.go`

Jangan menyentuh berkas lain di `internal/grimoire/` (`scan.go`, `modules.go`, `hash.go`, `pages.go`, `index.go`, `build.go`, `ipc.go`), `cmd/*`, `frontend/*`, `go.mod`/`go.sum`, atau `overview.md`. Tanpa dependency baru, tanpa unit test.

## Kontrak

Disediakan (signature persis overview 2.3, jangan diubah):

```go
type Store struct{ dataDir string }
func NewStore(dataDir string) *Store
func (s *Store) Dir(projectID string) string
func (s *Store) ValidateProject(projectID string) error
func (s *Store) IndexPath(projectID string) string
func (s *Store) LawsPath(projectID string) string
func (s *Store) StatePath(projectID string) string
func (s *Store) BooksDir(projectID string) string
func (s *Store) BookPath(projectID, book, chapter string) string
func (s *Store) WriteFile(projectID, rel string, data []byte) error
func (s *Store) ReadFile(projectID, rel string) ([]byte, error)
func (s *Store) Exists(projectID string) bool
func (s *Store) ReadState(projectID string) (State, error)
func (s *Store) WriteState(projectID string, st State) error
func (s *Store) RemoveBooks(projectID string) error
func Snake(s string) string
func CamelTag(s string) string
```

Tipe dan sentinel di `types.go` (overview 2.2): `Chapter`, `Book`, `ModuleState`, `State`, `Summary`, `ErrNotBuilt`, `ErrInvalidPath`, `ErrInvalidProject`. Sentinel hanya didefinisikan di `types.go`; `store.go` hanya memakainya.

Dipakai: `github.com/NooBeeID/shikigami/internal/fsx` (`fsx.ResolveUnder(root, rel)`), pustaka standar. Fakta penting tentang `fsx.ResolveUnder` (sudah diverifikasi di `internal/fsx/readwrite.go`):

- Root HARUS sudah ada sebagai direktori, kalau tidak mengembalikan `fsx.ErrNotFound` (bukan `os.ErrNotExist`). Maka `WriteFile` harus `MkdirAll(Dir, 0700)` SEBELUM memanggilnya, dan `ReadFile` harus memeriksa keberadaan `Dir` sendiri sebelum memanggilnya.
- Menolak rel absolut, `..` yang keluar root, NUL, dan symlink keluar root dengan galat berbungkus `fsx.ErrInvalid`. Komponen rel yang belum ada diperbolehkan.
- Hasilnya path absolut hasil `Clean`.

Aturan galat: path tak valid (kosong, absolut, mengandung segmen `..`, NUL, atau ditolak `ResolveUnder` karena keluar root) menjadi `ErrInvalidPath` (bungkus dengan `fmt.Errorf("%w: ...", ErrInvalidPath)` agar `errors.Is` bekerja). Berkas tak ada HARUS memenuhi `errors.Is(err, os.ErrNotExist)` (bungkus dengan `%w`, termasuk saat direktori project belum ada). `projectID` tak valid menjadi `ErrInvalidProject`.

## Langkah

1. **Baca keadaan awal.** Buka `internal/grimoire/types.go` dan `store.go` (stub Foundation). Baca `internal/fsx/readwrite.go` bagian `ResolveUnder` dan `internal/fsx/errors.go` (`Wrap`, `ErrInvalid`, `ErrNotFound`) agar paham perilaku di atas.

2. **`types.go`: verifikasi dan tambahan aditif saja.** Pastikan isinya persis overview 2.2 (nama field, tag JSON, `Summary` bertag `books/pages/indexBytes`, tiga sentinel dengan pesan Indonesia persis: `grimoire belum dibangun`, `path grimoire tidak valid`, `projectId tidak valid`). Bila sudah sesuai, hanya tambahkan komentar dokumentasi Go (bahasa Indonesia) pada tiap tipe/field/sentinel dan komentar paket `// Package grimoire ...` bila belum ada di berkas lain paket (cek `grep -n "^// Package grimoire" internal/grimoire/*.go` agar tidak ganda). Jangan mengubah nama/tag/arti field, jangan menambah field baru kecuali benar-benar kurang dari overview 2.2, jangan menambah import selain `errors` dan `time`.

3. **`store.go`: timpa penuh.** Hapus komentar `STUB(foundation)`. Import: `encoding/json`, `errors`, `fmt`, `io`, `os`, `path/filepath`, `strings`, dan `github.com/NooBeeID/shikigami/internal/fsx`. Deklarasi konstanta privat di berkas ini saja: `const maxReadBytes = 4 << 20` dan `const stateFile = "state.json"`.

4. **Struct dan path (tanpa validasi, murni komposisi).**
   - `type Store struct{ dataDir string }`; `func NewStore(dataDir string) *Store { return &Store{dataDir: dataDir} }`.
   - `Dir` = `filepath.Join(s.dataDir, "grimoire", projectID)`.
   - `IndexPath` = `Dir/index.md`; `LawsPath` = `Dir/laws.md`; `StatePath` = `Dir/state.json`; `BooksDir` = `Dir/books`.
   - `BookPath(projectID, book, chapter)` = `filepath.Join(BooksDir, book, chapter+".md")` (argumen sudah snake; tidak memvalidasi).

5. **`ValidateProject`.** Kembalikan `fmt.Errorf("%w: %q", ErrInvalidProject, projectID)` bila `projectID` kosong, sama dengan `"."`, atau mengandung `/`, `\`, `..`, atau byte NUL; selain itu `nil`. (Penolakan `"."` adalah tambahan pengaman agar `Dir` tidak menunjuk folder `grimoire` induk; ULID tetap lolos.)

6. **Helper privat `resolve(projectID, rel string) (abs string, err error)`.**
   a. `ValidateProject(projectID)`.
   b. Validasi leksikal `rel` sebelum menyentuh disk: tolak (`ErrInvalidPath`) bila kosong, mengandung NUL, `filepath.IsAbs(rel)`, diawali `/` atau `\`, atau salah satu segmen (pisah di `/` dan `\`) sama dengan `..`. Normalisasi dengan `filepath.FromSlash`.
   c. `fsx.ResolveUnder(s.Dir(projectID), rel)`. Bila galat `errors.Is(err, fsx.ErrInvalid)` bungkus jadi `fmt.Errorf("%w: %v", ErrInvalidPath, err)`. Galat lain (mis. `fsx.ErrNotFound` karena root belum ada) dikembalikan oleh pemanggil sesuai konteks (lihat langkah 7 dan 8; `resolve` sendiri tidak membuat direktori).
   d. Pastikan hasil `abs` ada di bawah `Dir` (cek awalan `Dir + separator`) sebagai sabuk pengaman kedua; bila tidak, `ErrInvalidPath`.

7. **`WriteFile(projectID, rel, data)` atomik.**
   a. `ValidateProject`; lalu `os.MkdirAll(s.Dir(projectID), 0o700)` (perlu agar `ResolveUnder` lolos untuk project baru).
   b. `abs, err := s.resolve(projectID, rel)`; kembalikan galatnya apa adanya.
   c. `os.MkdirAll(filepath.Dir(abs), 0o700)`.
   d. `os.CreateTemp(filepath.Dir(abs), ".tmp-*")` (direktori sama agar `Rename` atomik di satu filesystem); `defer` yang menghapus berkas sementara bila belum berhasil di-rename.
   e. `tmp.Chmod(0o600)`, `tmp.Write(data)`, `tmp.Sync()`, `tmp.Close()`, lalu `os.Rename(tmp.Name(), abs)`. Bungkus tiap galat dengan `fmt.Errorf("tulis grimoire %s: %w", rel, err)`.
   f. Bila `abs` sudah berupa direktori, `Rename` gagal; biarkan galat itu naik (bukan `ErrInvalidPath`).

8. **`ReadFile(projectID, rel)`.**
   a. `ValidateProject`. Bila `os.Stat(s.Dir(projectID))` gagal dengan `os.IsNotExist`, kembalikan `fmt.Errorf("grimoire project %s: %w", projectID, os.ErrNotExist)` (BUKAN memanggil `ResolveUnder`, yang akan memberi `fsx.ErrNotFound`).
   b. `abs, err := s.resolve(projectID, rel)`.
   c. `os.Stat(abs)`: bila `os.IsNotExist` kembalikan `fmt.Errorf("berkas grimoire %s: %w", rel, os.ErrNotExist)`; bila direktori, galat `berkas grimoire %s adalah direktori`; bila `Size() > maxReadBytes`, galat `berkas grimoire %s terlalu besar (maks 4 MiB)`.
   d. `os.Open` + `io.ReadAll(io.LimitReader(f, maxReadBytes+1))`; bila panjang > `maxReadBytes` galat terlalu besar (jaga kondisi balapan). Galat `os.IsNotExist` pada tahap ini juga dibungkus `%w` dengan `os.ErrNotExist`.

9. **`Exists(projectID) bool`.** `false` bila `ValidateProject` gagal; selain itu `os.Stat(s.IndexPath(projectID))` sukses dan bukan direktori.

10. **`ReadState` / `WriteState`.**
    - `ReadState`: `b, err := s.ReadFile(projectID, stateFile)`; bila `errors.Is(err, os.ErrNotExist)` kembalikan `State{}, ErrNotBuilt`; galat lain apa adanya (termasuk `ErrInvalidProject`). `json.Unmarshal` ke `State`; galat dibungkus `fmt.Errorf("state.json rusak: %w", err)`. Bila `Modules == nil` setelah decode, isi `map[string]ModuleState{}`.
    - `WriteState`: `ValidateProject`; salin `st`; bila `st.Modules == nil` set map kosong dan bila `st.ProjectID == ""` isi dengan `projectID`; `json.MarshalIndent(st, "", "  ")`, tambahkan `"\n"` di akhir, lalu `s.WriteFile(projectID, stateFile, data)`. Tidak mengubah `BuiltAt` (pemanggil yang mengisi).

11. **`RemoveBooks(projectID)`.** `ValidateProject`; `os.RemoveAll(s.BooksDir(projectID))` (tidak ada = sukses, `RemoveAll` sudah demikian). Karena `BooksDir` selalu `Dir/books` dengan `projectID` tervalidasi, tidak perlu `ResolveUnder`, tetapi tambahkan guard: tolak bila `BooksDir` bukan bagian di bawah `Dir`.

12. **`Snake(s string) string`.** Ubah huruf ASCII A-Z menjadi huruf kecil; setiap rune selain `[a-z0-9]` (termasuk non-ASCII dan spasi) menjadi `_`; ringkas deret `_` menjadi satu; potong `_` di tepi; hasil kosong menjadi `"x"`. Implementasi dengan satu `strings.Builder` dan flag "terakhir adalah underscore". Contoh wajib: `Snake("Session PTY") == "session_pty"`, `Snake("") == "x"`, `Snake("--a//b--") == "a_b"`, `Snake("frontend/src/modules/files") == "frontend_src_modules_files"`. Deterministik dan idempoten (`Snake(Snake(x)) == Snake(x)`).

13. **`CamelTag(s string) string`.** Hitung `parts := strings.Split(Snake(s), "_")` (memakai `Snake` agar pemisahan konsisten, non-alfanumerik sebagai pemisah). Kata pertama huruf kecil; tiap kata berikutnya huruf pertama dibesarkan (`strings.ToUpper` pada byte pertama ASCII) dan sisanya tetap huruf kecil; gabungkan tanpa pemisah. Hasil selalu satu token tanpa spasi/`_`/`-`/`.`/`/`. Contoh wajib: `CamelTag("session_pty") == "sessionPty"`, `CamelTag("files") == "files"`, `CamelTag("My-Module.v2") == "myModuleV2"`, `CamelTag("") == "x"`.

14. **Komentar dan gaya.** Komentar Go dalam bahasa Indonesia; identifier Inggris; pesan galat Indonesia. Tidak ada logger di berkas ini. Tidak ada variabel/konstanta paket selain `maxReadBytes` dan `stateFile` agar tidak bentrok dengan berkas lain di paket (cek `grep -n "maxReadBytes\|stateFile" internal/grimoire/*.go`).

15. **Guard.** Jalankan `gofmt -l internal/grimoire/types.go internal/grimoire/store.go` (harus kosong) dan `go build ./...`. Bila build gagal karena berkas fitur lain (bukan `types.go`/`store.go`), jangan perbaiki; laporkan. Pastikan `git diff --stat -- go.mod go.sum` kosong.

## Perubahan

- Diubah: `internal/grimoire/types.go` (hanya komentar dokumentasi/penegasan aditif; isi tipe tetap sesuai overview 2.2)
- Ditimpa penuh: `internal/grimoire/store.go` (stub Foundation diganti implementasi nyata, komentar `STUB(foundation)` dihapus)
- Tidak ada berkas lain dibuat atau diubah.

## Wiring

Tidak ada yang disambungkan oleh fitur ini. Yang diekspos untuk langkah wiring dan fitur lain (semua sudah ada sebagai stub, kini nyata):

- `grimoire.NewStore(dataDir)` dipanggil nanti oleh `cmd/shikigamid/wire_grimoire.go` (fitur `ipc-ctl`, dengan `d.DataDir` atau fallback `paths.DataDir()`).
- `Store.Dir` dipakai `ipc-ctl` untuk field `dir` pada `grimoire.status`; `ReadFile`/`ReadState`/`Exists`/`ValidateProject` untuk `read`/`list`/`status`.
- `Store.WriteFile`/`WriteState`/`RemoveBooks`/`BookPath` dipakai `pages-index` (`Build`); `Snake`/`CamelTag` dipakai `scan-modules` dan `pages-index`.
- Galat kontrak untuk pemetaan IPC: `ErrInvalidProject`/`ErrInvalidPath` (-> `ErrInvalid`), `os.ErrNotExist` via `errors.Is` dan `ErrNotBuilt` (-> `ErrNotFound`).

Wiring sebenarnya (registrasi modul/perintah/panel) dikerjakan langkah wiring di akhir epic (overview bagian 5), bukan di sini.

## Selesai bila

- `types.go` memuat `Chapter`, `Book`, `ModuleState`, `State`, `Summary` dan tiga sentinel sesuai overview 2.2, tag JSON tidak berubah.
- `store.go` menyediakan seluruh signature overview 2.3 tanpa perubahan, tanpa sisa komentar `STUB(foundation)`, dan tanpa definisi ganda terhadap berkas lain di paket.
- Semua penulisan memakai tmp + `Sync` + `Rename`, direktori 0700, berkas 0600, lewat `fsx.ResolveUnder`; tidak ada penulisan di luar `<dataDir>/grimoire/<projectId>/`; rel kosong, absolut, atau mengandung `..` mengembalikan `ErrInvalidPath`; `projectID` tak valid mengembalikan `ErrInvalidProject`.
- `ReadFile` menolak berkas > 4 MiB dan galat tak-ada (termasuk direktori project belum ada) memenuhi `errors.Is(err, os.ErrNotExist)`; `ReadState` mengembalikan `ErrNotBuilt` bila `state.json` tak ada.
- `Snake("Session PTY") == "session_pty"` dan `CamelTag("session_pty") == "sessionPty"` (periksa mental atau lewat berkas uji sementara di scratchpad yang dihapus lagi; jangan commit unit test).
- Guard: `go build ./...` sukses untuk path fitur ini, `gofmt -l` pada kedua berkas kosong, `go.mod`/`go.sum` tidak berubah.
