# Plan: Tree malas dan ignore

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-03-file-service.md` (epic 03 File Service, fitur `fs-tree`)
- Overview: `docs/plans/ai-terminal-ticket-03-file-service/overview.md` (baca bagian 2, 3, 7)
- Fitur: `fs-tree` - Tree malas dan ignore
- ownedPaths: `internal/fsx/tree.go`, `internal/fsx/ignore.go`
- Depends on foundation: ya (`00-foundation.md` harus sudah selesai; menyediakan `Error`/sentinel, `ResolveUnder`, dan stub `List`, `Entry`, `EntryType`, `DefaultIgnore`, `IsIgnored` yang diganti plan ini)

## Tujuan

Mengganti stub Foundation dengan implementasi penuh `List(root, relDir)` yang membaca SATU level folder secara malas (tanpa rekursi, goroutine, cache, watcher, timer), plus daftar ignore berbasis nama (`DefaultIgnore`, `IsIgnored`). Hasil terurut folder dulu lalu file, alfabetis tanpa peka huruf besar. Symlink dilaporkan sebagai `symlink` tanpa diikuti.

## Path yang dimiliki

HANYA dua file ini yang boleh dibuat/diubah:

- `internal/fsx/tree.go`
- `internal/fsx/ignore.go`

Dilarang menyentuh: `errors.go`, `readwrite.go`, `binary.go`, `watch.go`, `ipc.go`, `cmd/aitermd/*`, `cmd/aitermctl/*`, `internal/ipc/`, `go.mod`/`go.sum` (tanpa `go get`/`go mod tidy`), Makefile, frontend, `overview.md`. Pada `tree.go`/`ignore.go` hapus komentar `// STUB(foundation): diganti oleh fs-tree` saat stub diganti.

## Kontrak

Disediakan (signature dikunci overview bagian 2 dan tiket; jangan diubah):

```go
package fsx

type EntryType string
const (
    EntryFile    EntryType = "file"
    EntryDir     EntryType = "dir"
    EntrySymlink EntryType = "symlink"
)

type Entry struct {
    Name  string    `json:"name"`
    Type  EntryType `json:"type"`
    Size  int64     `json:"size"`  // byte; 0 untuk dir
    Mtime int64     `json:"mtime"` // Unix nanodetik UTC
}

func List(root, relDir string) ([]Entry, error)

var DefaultIgnore []string
func IsIgnored(name string) bool
```

Dipakai dari fsx lain (sudah ada dari Foundation, jangan diubah):

- `ResolveUnder(root, rel string) (abs string, err error)`: mengembalikan path absolut hasil Clean; `rel` `""`/`"."` = root; `root` tidak absolut -> `ErrInvalid`, `root` tidak ada -> `ErrNotFound`; `rel` absolut/NUL/keluar root/symlink keluar root -> `ErrInvalid`; target yang tidak ada TIDAK error di sini (pemanggil yang memutuskan `ErrNotFound`).
- `Wrap(base *Error, format string, args ...any) error`, `ErrNotFound`, `ErrInvalid`.

Konsumen: `ipc.go` (fitur `fs-watch-ipc`) memanggil `List` untuk `fs.list` dan membungkus hasilnya `{entries: Entry[]}`. Karena itu `List` HARUS mengembalikan slice non-nil (`[]Entry{}` untuk folder kosong) agar JSON menjadi `[]`, bukan `null`.

Aturan tambahan: `tree.go` dan `ignore.go` hanya memakai stdlib (`os`, `path/filepath`, `sort`, `strings`, `errors`, `io/fs`); dilarang mengimpor `aiterm/internal/ipc` atau fsnotify.

## Langkah

1. **Baca keadaan awal.** Buka `internal/fsx/tree.go`, `ignore.go`, `errors.go`, dan `readwrite.go` (hanya baca) untuk memastikan nama tipe/konstanta/sentinel sesuai Kontrak. Bila Foundation sudah mendeklarasikan `EntryType`, `EntryFile/EntryDir/EntrySymlink`, `Entry` di `tree.go`, pertahankan deklarasinya (tag json harus tepat) dan hanya ganti isi `List`.

2. **`internal/fsx/ignore.go`** (tulis ulang penuh, hapus komentar STUB):
   - `var DefaultIgnore = []string{"node_modules", ".git", "target", "dist", "build", ".next", ".nuxt", ".svelte-kit", ".cache", ".venv", "venv", "__pycache__", ".idea", ".DS_Store", "vendor", "Pods", ".gradle", "coverage"}` (tepat 18 nama dari tiket; `.env`, `.gitignore`, dan file tersembunyi lain TIDAK masuk).
   - `var ignoreSet = buildIgnoreSet(DefaultIgnore)` dengan `func buildIgnoreSet(names []string) map[string]struct{}` (dibangun sekali saat inisialisasi paket; tanpa `init()` berat).
   - `func IsIgnored(name string) bool { _, ok := ignoreSet[name]; return ok }`. Cocok nama persis, peka huruf besar, bukan path (nama berisi `/` tidak pernah cocok). Tanpa membaca `.gitignore`.
   - Komentar doc singkat pada `DefaultIgnore` bahwa slice ini sumber kebenaran dan `ignoreSet` dibangun darinya; mengubah slice setelah startup tidak berpengaruh.

3. **`internal/fsx/tree.go`** (ganti stub `List`; tipe tetap):
   1. Validasi path: `abs, err := ResolveUnder(root, relDir)`; bila `err != nil` kembalikan apa adanya (sudah `ErrInvalid`/`ErrNotFound` berbungkus).
   2. Cek target folder: `info, err := os.Stat(abs)`. Bila `errors.Is(err, fs.ErrNotExist)` -> `Wrap(ErrNotFound, "folder %q tidak ada", relDir)`. Bila error lain -> kembalikan `fmt.Errorf("stat %q: %w", relDir, err)` (error non-`*Error`; `ipc.go` memetakannya ke internal error). Bila `!info.IsDir()` -> `Wrap(ErrInvalid, "%q bukan folder", relDir)`. (Symlink-ke-folder yang masih di bawah root sudah diloloskan `ResolveUnder`; `os.Stat` mengikutinya, itu benar untuk folder yang dibuka eksplisit oleh pemanggil.)
   3. Baca satu level: `dirents, err := os.ReadDir(abs)` (SATU panggilan; DILARANG `filepath.Walk/WalkDir`, rekursi, goroutine, cache, timer). Error `fs.ErrNotExist` -> `ErrNotFound` (folder hilang di antara Stat dan ReadDir); error lain -> `fmt.Errorf("baca folder %q: %w", relDir, err)`. Catatan: `os.ReadDir` bisa mengembalikan sebagian entry bersama error; perlakukan error sebagai kegagalan (kembalikan error).
   4. Iterasi `dirents` membentuk `entries := make([]Entry, 0, len(dirents))`:
      - `name := de.Name()`; bila `IsIgnored(name)` -> `continue` (berlaku untuk file maupun folder, termasuk symlink bernama ignore).
      - `fi, err := de.Info()`; bila `err != nil` -> `continue` (entry yang gagal di-stat, mis. hilang saat race, dilewati; jangan gagalkan seluruh list). `DirEntry.Info()` dari `os.ReadDir` memakai `Lstat` sehingga symlink TIDAK diikuti; jangan memanggil `os.Stat` pada entry.
      - Tentukan tipe dari `fi.Mode()`: `fi.Mode()&os.ModeSymlink != 0` -> `EntrySymlink` (periksa PERTAMA, sebelum `IsDir`); `fi.IsDir()` -> `EntryDir`; selain itu (file biasa, socket, device, dll.) -> `EntryFile`.
      - `Size`: `0` untuk `EntryDir`; selain itu `fi.Size()` (untuk symlink adalah panjang target dari Lstat; tidak diikuti). `Mtime: fi.ModTime().UnixNano()`.
      - `append` `Entry{Name: name, Type: t, Size: size, Mtime: mtime}`.
   5. Urutkan dengan `sort.SliceStable` (satu fungsi pembanding):
      - Kelompok: `Type == EntryDir` lebih dulu; semua tipe lain (`file` dan `symlink`) satu kelompok file. Symlink (termasuk yang menunjuk folder) diurutkan bersama file karena dilaporkan tanpa diikuti.
      - Dalam kelompok: bandingkan `strings.ToLower(a.Name)` vs `strings.ToLower(b.Name)`; bila sama setelah lower-case, pecah seri dengan `a.Name < b.Name` agar urutan deterministik. (Hitung kunci lower-case sekali per entry bila perlu, mis. slice pembantu; jangan alokasi berlebihan, tapi kesederhanaan lebih diutamakan.)
   6. Kembalikan `entries, nil` (non-nil walau kosong).
   - Tambahkan komentar doc pada `List`: satu level, malas, tanpa cache, ignore berbasis nama, symlink tidak diikuti, entry gagal-stat dilewati.

4. **Verifikasi lokal.** Dari root worktree: `gofmt -l internal/fsx/tree.go internal/fsx/ignore.go` (harus kosong), `go build ./internal/fsx/...`, dan opsional `go vet ./internal/fsx/...`. Build paket bisa gagal sementara akibat file fitur lain yang setengah jadi; bila error berasal dari file bukan milik plan ini, JANGAN memperbaikinya, cukup pastikan tidak ada error di `tree.go`/`ignore.go` (`go vet` / pesan kompilasi menunjuk file). Uji sementara boleh lewat program kecil di luar repo (mis. di scratchpad) yang memanggil `fsx.List` pada folder sementara berisi folder, file campuran huruf besar/kecil, symlink, `node_modules`, dan `.env`; hapus semua file sementara sebelum selesai. Jangan menambah file `_test.go` di repo.

5. **Pemeriksaan akhir terhadap acceptance tiket.** `grep -nE "filepath\.Walk|WalkDir|go func|time\.|fsnotify|internal/ipc" internal/fsx/tree.go internal/fsx/ignore.go` harus kosong; `grep -n "STUB(foundation)" internal/fsx/tree.go internal/fsx/ignore.go` harus kosong; tidak ada `panic`.

## Perubahan

- Diubah: `internal/fsx/tree.go` (stub `List` diganti implementasi; tipe `Entry`/`EntryType`/konstanta dipertahankan).
- Diubah: `internal/fsx/ignore.go` (stub diganti `DefaultIgnore` berisi 18 nama, `ignoreSet`, `IsIgnored`).
- Tidak ada file lain dibuat atau diubah.

## Wiring

Fitur ini hanya mengekspos API Go; TIDAK menyambungkan apa pun sendiri.

- `fsx.List(root, relDir) ([]Entry, error)`: dipanggil oleh `internal/fsx/ipc.go` (milik `fs-watch-ipc`) untuk method `fs.list`, dibungkus `{entries: Entry[]}`; error `*Error` dipetakan ke kode -32002/-32003 oleh `ipc.go`.
- `fsx.IsIgnored` / `fsx.DefaultIgnore`: tersedia untuk konsumen lain, tidak ada wiring tambahan.
- Wiring akhir (modul daemon `fs`, perintah `aitermctl fs ls`, `go mod tidy`, hapus sisa stub) dikerjakan langkah wiring di overview bagian 5, bukan di sini.

## Selesai bila

- [ ] `go build ./internal/fsx/...` sukses terhadap `tree.go` dan `ignore.go` (guard fitur ini; tanpa unit test). Bila gagal hanya karena file fitur lain, laporkan itu dan pastikan tidak ada error di dua file milik plan ini.
- [ ] `gofmt -l` pada dua file kosong; komentar `STUB(foundation)` di dua file sudah hilang.
- [ ] `List` hanya memanggil `ResolveUnder`, `os.Stat` pada folder target, dan satu `os.ReadDir`; tanpa rekursi/Walk, goroutine, timer, cache, watcher.
- [ ] Entry bernama sesuai `DefaultIgnore` tidak muncul; `.env`/`.gitignore` tetap muncul.
- [ ] Urutan: folder dulu lalu file/symlink, alfabetis tanpa peka huruf besar, deterministik.
- [ ] Symlink bertipe `symlink` tanpa diikuti; entry yang gagal `Info()` dilewati tanpa menggagalkan list.
- [ ] `relDir` keluar root -> `ErrInvalid`; folder tak ada -> `ErrNotFound`; `relDir` menunjuk file -> `ErrInvalid`; folder kosong menghasilkan slice non-nil.
- [ ] Tidak ada file di luar `ownedPaths` yang berubah.
