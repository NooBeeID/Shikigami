# Plan: Baca/tulis dan deteksi biner

## Metadata

- Tiket: `ai-terminal-ticket-03-file-service` (`docs/tickets/ai-terminal-ticket-03-file-service.md`)
- Fitur: `fs-readwrite` (Baca/tulis dan deteksi biner)
- ownedPaths: `internal/fsx/readwrite.go`, `internal/fsx/binary.go`, `internal/fsx/errors.go`
- Depends on foundation: ya. File-file di atas sudah dibuat oleh Foundation (`00-foundation.md`): `errors.go` dan `ResolveUnder` sudah lengkap, `Read`/`Write`/`IsBinary` masih stub bertanda `// STUB(foundation): diganti oleh fs-readwrite`. Plan ini menyempurnakan dan mengganti stub itu TANPA mengubah signature.

Rujukan: `overview.md` bagian 2 (kontrak bersama, semantik `ResolveUnder`, error `fsx`) dan bagian "fs-readwrite" di tiket. Plan ini tidak mengulang isinya; bila ada selisih, tiket menang.

## Tujuan

Mengimplementasikan penuh logika baca/tulis file di `package fsx`:
- `errors.go` final (tipe `Error`, sentinel -32000..-32003, `IsCode`, `Wrap`, `NewConflict`).
- `ResolveUnder` anti traversal dan anti symlink-keluar-root.
- `IsBinary` (8 KiB pertama: NUL atau UTF-8 tidak valid).
- `Read` dengan offset/limit sejajar batas rune UTF-8, tanpa batas ukuran, `Kind`, mtime nanodetik dari `Stat`.
- `Write` atomik (tmp + fsync + rename), pertahankan permission, kunci konflik `expectedMtime`, `content` wajib UTF-8 valid.

Hanya stdlib. Tanpa goroutine, tanpa timer, tanpa unit test.

## Path yang dimiliki

HANYA tiga file ini yang boleh disentuh:
- `internal/fsx/readwrite.go`
- `internal/fsx/binary.go`
- `internal/fsx/errors.go`

Dilarang menyentuh: `internal/fsx/tree.go`, `ignore.go`, `watch.go`, `ipc.go`, `cmd/aitermd/*`, `cmd/aitermctl/*`, `internal/ipc/`, `go.mod`, `go.sum`, Makefile, frontend, `overview.md`. Dilarang `go get`/`go mod tidy`. Jangan memperbaiki file fitur lain bila build paket gagal karena mereka setengah jadi.

## Kontrak

Disediakan (signature tetap, lihat overview bagian 2):

```go
// errors.go
type Error struct { Code int; Message string; Data any }
func (e *Error) Error() string
func (e *Error) Is(target error) bool
var ErrBinary, ErrConflict, ErrNotFound, ErrInvalid *Error // -32000, -32001, -32002, -32003
func IsCode(err error, code int) bool
func Wrap(base *Error, format string, args ...any) error
func NewConflict(diskMtime int64) error

// binary.go
func IsBinary(sample []byte) bool

// readwrite.go
type Kind string // KindMarkdown = "markdown", KindText = "text"
type ReadResult struct { Content string; Mtime int64; Kind Kind; Size, Offset, Length int64; EOF bool } // tag json: content,mtime,kind,size,offset,length,eof
type WriteResult struct { Mtime int64; Size int64 }                                                     // tag json: mtime,size
func ResolveUnder(root, rel string) (abs string, err error)
func Read(root, relPath string, offset, limit int64) (*ReadResult, error)
func Write(root, relPath, content string, expectedMtime int64) (*WriteResult, error)
```

Pesan error (Message sentinel, bahasa Inggris, dikunci): `binary file not supported`, `file changed on disk`, `not found`, `invalid argument`. Detail ditambahkan lewat `Wrap` (format `"<sentinel message>: <detail>"`). `NewConflict` memuat teks `file changed on disk (disk mtime <N>)` dan `Data: map[string]int64{"currentMtime": N}`.

Dipakai oleh fitur lain (jangan merusak): `fs-tree` memanggil `ResolveUnder`, `Wrap`, `ErrInvalid`, `ErrNotFound`; `fs-watch-ipc` memanggil `ResolveUnder`, `Read`, `Write`, `*Error`/`IsCode`, dan memakai `err.Error()` sebagai message IPC serta `Data` untuk `ipc.Error.Data`. Fitur ini tidak mengimpor `internal/ipc` maupun fsnotify.

Semantik `ResolveUnder` (dikunci overview): root bukan absolut -> `ErrInvalid`; root tak ada/bukan folder -> `ErrNotFound`; `rel` absolut/NUL/keluar root setelah Clean -> `ErrInvalid`; `""`/`"."` -> root bersih; mengembalikan path hasil `filepath.Clean(filepath.Join(root, rel))` (bukan hasil EvalSymlinks); target tak ada tidak error selama induk terdekat yang ada masih di bawah root nyata.

## Langkah

Kerjakan berurutan. Baca dulu ketiga file hasil Foundation; pertahankan yang sudah benar, ganti hanya yang kurang/stub.

1. **`errors.go` - audit dan sempurnakan.**
   - Pastikan isi sesuai kontrak di atas. `Error()` mengembalikan `Message`. `Is` memakai `t, ok := target.(*Error); return ok && t.Code == e.Code`.
   - `IsCode(err, code)`: `var fe *Error; return errors.As(err, &fe) && fe.Code == code`. Perhatikan: untuk error hasil `Wrap`, `errors.As` mengembalikan sentinel, jadi Code tetap benar.
   - `Wrap`: `fmt.Errorf("%w: %s", base, fmt.Sprintf(format, args...))`. Jangan meneruskan `format` langsung ke `fmt.Errorf` agar `%` di path tidak salah tafsir.
   - `NewConflict`: seperti kontrak; kembalikan `*Error` baru (bukan sentinel) sehingga `Data` terbawa; `errors.Is(err, ErrConflict)` tetap true lewat `Is`.
   - Jangan menambah export baru selain yang di kontrak. Tidak ada impor selain `errors` dan `fmt`.

2. **`binary.go` - ganti stub `IsBinary`.** Hapus komentar STUB.
   - Konstanta unexported `binarySampleSize = 8 * 1024` (dipakai juga oleh `readwrite.go`).
   - `func IsBinary(sample []byte) bool`:
     - Potong ke `binarySampleSize` bila lebih panjang; catat apakah terpotong (`truncated := len(sample) > binarySampleSize`).
     - `len == 0` -> `false`.
     - `bytes.IndexByte(sample, 0) >= 0` -> `true`.
     - Bila `truncated`, buang ekor yang berupa potongan rune tidak lengkap: dari akhir, mundur paling banyak `utf8.UTFMax-1` byte; jika `!utf8.FullRune(sample[i:])` untuk awal rune (byte yang bukan continuation `0b10xxxxxx`) di posisi itu, potong sampel di `i`. Hanya lakukan saat terpotong (EOF sungguhan dengan rune tidak lengkap tetap biner).
     - Kembalikan `!utf8.Valid(sample)`.
   - Impor: `bytes`, `unicode/utf8`. Tanpa I/O di fungsi ini.

3. **`readwrite.go` - tipe dan helper.**
   - Pertahankan `Kind`, konstanta, `ReadResult`, `WriteResult` dengan tag json sesuai tiket.
   - Helper unexported `kindFor(name string) Kind`: `strings.ToLower(filepath.Ext(name))` ada di `{".md", ".markdown", ".txt"}` -> `KindMarkdown`, selain itu `KindText`.
   - Helper unexported `mtimeNs(fi os.FileInfo) int64 { return fi.ModTime().UnixNano() }`.

4. **`readwrite.go` - audit `ResolveUnder`.** Verifikasi implementasi Foundation memenuhi semantik di Kontrak; perbaiki bila ada celah:
   - Urutan: root absolut (`filepath.IsAbs`) -> `Stat(root)` (tak ada -> `ErrNotFound`, bukan folder -> `ErrNotFound`) -> tolak `rel` mengandung `\x00` atau `filepath.IsAbs(rel)` (juga tolak yang berawalan `/` setelah `FromSlash`) -> `abs := filepath.Clean(filepath.Join(cleanRoot, filepath.FromSlash(rel)))` -> `abs == cleanRoot || strings.HasPrefix(abs, cleanRoot+string(filepath.Separator))` kalau tidak `ErrInvalid`.
   - `realRoot, err := filepath.EvalSymlinks(cleanRoot)`. Untuk target: `EvalSymlinks(abs)`; bila `os.IsNotExist`, naik ke induk (`filepath.Dir`) berulang sampai ada yang berhasil di-EvalSymlinks (berhenti paling jauh di `cleanRoot`), lalu cek yang itu. Hasil nyata harus `== realRoot` atau berawalan `realRoot+sep`, kalau tidak `ErrInvalid`. Symlink dangling yang menunjuk keluar root juga ditolak (EvalSymlinks gagal dengan not-exist pada target dangling: periksa dengan `os.Readlink` pada komponen terakhir bila perlu, atau cukup tolak bila `Lstat(abs)` berhasil tetapi `EvalSymlinks` gagal -> `ErrInvalid`).
   - Error lain dari EvalSymlinks (mis. permission) -> `Wrap(ErrInvalid, ...)`.
   - Kembalikan `abs` (hasil Clean). Fungsi tidak memutuskan target ada/tidak.

5. **`readwrite.go` - implementasi `Read`.** Hapus stub dan komentarnya.
   - Validasi: `offset < 0 || limit < 0` -> `Wrap(ErrInvalid, "offset/limit negatif")`. `limit <= 0` berarti sampai akhir file (limit `0` valid).
   - `abs, err := ResolveUnder(root, relPath)`; teruskan error. Larang `relPath` kosong/"." sebagai file (akan jatuh ke "folder" di langkah berikut -> `ErrInvalid`).
   - `fi, err := os.Stat(abs)`: `os.IsNotExist` -> `Wrap(ErrNotFound, "%s", relPath)`; error lain -> `Wrap(ErrInvalid, ...)`. `fi.IsDir()` -> `Wrap(ErrInvalid, "%s adalah folder", relPath)`. Bukan file biasa (`!fi.Mode().IsRegular()`, mis. FIFO/device) -> `ErrInvalid` (mencegah blok pada FIFO).
   - `f, err := os.Open(abs)`; `defer f.Close()`. Gunakan `f.Stat()` (file yang sama) untuk `size` dan `mtime` agar konsisten dengan yang dibuka.
   - Deteksi biner SEBELUM membaca isi: baca `min(size, 8192)` byte dari offset 0 dengan `f.ReadAt` (tahan `io.EOF`), panggil `IsBinary(sample)`; true -> `Wrap(ErrBinary, "%s", relPath)`. Pemeriksaan ini selalu dari awal file, bukan dari `offset`.
   - Hitung rentang byte: `start := offset`; bila `start > size` -> `start = size`. `end := size`; bila `limit > 0 && start+limit < size` -> `end = start+limit` (waspadai overflow: bandingkan `limit < size-start`).
   - Selaraskan batas UTF-8 (hanya bila potongan parsial; seluruh file/tanpa batas tidak perlu penyesuaian):
     - Majukan `start` selama `start < end` dan byte pada `start` adalah continuation byte (`b&0xC0 == 0x80`), paling banyak 3 langkah. Karena file lolos deteksi biner hanya dari 8 KiB awal, byte setelahnya bisa saja tidak valid; batasi dengan 3 langkah dan lanjutkan, lalu hasil akhir tetap divalidasi di bawah.
     - Bila `end < size`, mundurkan `end` ke batas rune. Algoritma: baca 1 byte di posisi `end` lewat `ReadAt`; bila bukan continuation byte (`b&0xC0 != 0x80`), `end` sudah di batas rune, selesai. Bila continuation byte, rune terpotong: baca potongan `[start, end)` ke `buf`, lalu pangkas ekor `buf` mulai dari byte terakhir mundur paling banyak `utf8.UTFMax-1` byte sampai byte yang baru saja dipangkas adalah awal rune (`utf8.RuneStart(b)` true; byte awal itu ikut dipangkas). Hasilnya `buf` berakhir tepat sebelum rune yang terpotong, dan `end = start + len(buf)`.
     - Bila `limit > 0` terlalu kecil sehingga potongan kosong akibat penyelarasan (limit lebih kecil dari satu rune), kembalikan `Content ""`, `Length 0`, `EOF false`, `Offset = start`; pemanggil dapat menaikkan limit.
   - Baca isi: alokasikan `buf := make([]byte, end-start)` ukuran tepat dan `io.ReadFull(io.NewSectionReader(f, start, end-start), buf)` (atau `f.ReadAt` loop); tanpa batas ukuran, tanpa `ioutil.ReadAll`/salinan ganda. Konversi ke string memakai `unsafe`-free `string(buf)` (satu salinan diterima) atau `strings.Builder` bila ingin; jangan ada batas ukuran.
   - Bila `!utf8.Valid(buf)` (byte rusak setelah 8 KiB awal) -> `Wrap(ErrBinary, "%s: bukan UTF-8 valid", relPath)` agar `Content` selalu string valid.
   - Hasil: `&ReadResult{Content: string(buf), Mtime: mtimeNs(fi), Kind: kindFor(relPath), Size: size, Offset: start, Length: int64(len(buf)), EOF: start+int64(len(buf)) >= size}`. File kosong: `Content ""`, `Length 0`, `EOF true`, `Kind` dari ekstensi.
   - Untuk `Offset`: nilai yang dilaporkan adalah offset nyata setelah penyesuaian (`start`), bukan yang diminta.

6. **`readwrite.go` - implementasi `Write`.** Hapus stub dan komentarnya.
   - Validasi awal: `!utf8.ValidString(content)` -> `Wrap(ErrInvalid, "content bukan UTF-8 valid")`. `expectedMtime < 0` -> `ErrInvalid`.
   - `abs, err := ResolveUnder(root, relPath)`. Tolak target = root (`abs == cleanRoot`; cek lewat `filepath.Clean(root)`) dengan `ErrInvalid`.
   - Induk: `dir := filepath.Dir(abs)`; `Stat(dir)`: tak ada -> `Wrap(ErrNotFound, "folder induk tidak ada")` (tidak membuat folder); bukan folder -> `ErrInvalid`. Symlink induk sudah divalidasi oleh `ResolveUnder`.
   - `fi, err := os.Stat(abs)`:
     - ada dan `IsDir()` -> `ErrInvalid`; ada tapi bukan file biasa -> `ErrInvalid`.
     - ada: `expectedMtime == 0` -> `NewConflict(mtimeNs(fi))`; `expectedMtime != mtimeNs(fi)` -> `NewConflict(mtimeNs(fi))`. File tidak diubah pada kasus konflik (cek dilakukan sebelum membuat tmp).
     - tidak ada (`os.IsNotExist`): hanya boleh bila `expectedMtime == 0`; bila `expectedMtime != 0` -> `Wrap(ErrNotFound, "%s hilang dari disk", relPath)`. Permission baru = `0o644`.
     - error Stat lain -> `Wrap(ErrInvalid, ...)`.
     - File ada: permission = `fi.Mode().Perm()`.
   - Tulis atomik: `tmp, err := os.CreateTemp(dir, "."+filepath.Base(abs)+".aiterm-tmp-*")` (pola mengikuti `.<nama>.aiterm-tmp-<rand>`). Gunakan flag sukses (`committed bool`) dan `defer` yang `tmp.Close()` bila belum ditutup lalu `os.Remove(tmp.Name())` bila `!committed`.
     - `tmp.WriteString(content)` (tanpa batas ukuran), `tmp.Chmod(perm)` (CreateTemp membuat 0600; set ke permission target/0644; abaikan umask karena Chmod eksplisit), `tmp.Sync()`, `tmp.Close()`.
     - `os.Rename(tmp.Name(), abs)`; gagal -> bersihkan tmp, `Wrap(ErrInvalid, "gagal menulis: %v", err)`.
     - Setelah rename sukses set `committed = true`. Best-effort `fsync` folder induk (`os.Open(dir)` lalu `Sync()`, abaikan error) agar rename tahan crash.
   - Hasil: `fi2, err := os.Stat(abs)`; kembalikan `&WriteResult{Mtime: mtimeNs(fi2), Size: fi2.Size()}`. Bila Stat gagal setelah rename, `Wrap(ErrInvalid, ...)`.
   - Catatan yang diterima: jendela kecil antara cek mtime dan rename tidak atomik (lihat tiket). Rename mengganti inode; owner/ACL/xattr tidak dipertahankan, hanya permission (sesuai tiket).
   - Tolak menulis lewat symlink sebagai target: bila `Lstat(abs)` menunjukkan symlink, `ResolveUnder` sudah memastikan target nyata masih di bawah root; rename akan menimpa symlink itu sendiri dengan file biasa. Putuskan konservatif: bila `Lstat` symlink -> `Wrap(ErrInvalid, "menulis lewat symlink tidak didukung")`.

7. **Rapikan.** Pastikan tidak ada tanda `STUB(foundation)` tersisa di ketiga file, tidak ada `panic`, semua impor terpakai, dan komentar doc singkat (bahasa Indonesia atau Inggris, konsisten) untuk simbol exported. `gofmt -l internal/fsx/errors.go internal/fsx/binary.go internal/fsx/readwrite.go` harus kosong.

8. **Verifikasi.** `go build ./internal/fsx/...`. Bila gagal hanya karena file fitur lain (`tree.go`, `watch.go`, dst.), jangan diperbaiki; pastikan tidak ada error dari tiga file milik fitur ini (`go vet ./internal/fsx/...` opsional). Tanpa unit test; bila membuat skrip smoke sementara, taruh di luar repo (scratchpad) dan hapus sebelum selesai.

## Perubahan

Diubah (sudah ada dari Foundation):
- `internal/fsx/errors.go`: audit/finalisasi kontrak error (`Wrap` aman terhadap `%`, `NewConflict` dengan `Data`).
- `internal/fsx/binary.go`: stub `IsBinary` diganti implementasi penuh + konstanta `binarySampleSize`.
- `internal/fsx/readwrite.go`: `ResolveUnder` diaudit, helper `kindFor`/`mtimeNs`, `Read` dan `Write` penuh menggantikan stub.

Tidak ada file baru, tidak ada file lain yang diubah.

## Wiring

Yang diekspos untuk disambungkan nanti (JANGAN disambungkan di fitur ini):
- `fsx.Read`, `fsx.Write`, `fsx.ResolveUnder`, `fsx.IsBinary`, tipe `ReadResult`/`WriteResult`/`Kind`.
- `fsx.Error` + sentinel + `IsCode`/`Wrap`/`NewConflict`: dipakai `fs-watch-ipc` (`ipc.go`) untuk memetakan ke `ipc.NewError(code, err.Error())`; `Data` (`{"currentMtime": N}`) pada error konflik diteruskan ke `ipc.Error.Data`. Peta penanganan: `errors.As` ke `*fsx.Error`; bila error hasil `Wrap`, `Code` sentinel dipakai dan pesan detail dari `err.Error()`.
- `fs-tree` memakai `ResolveUnder`/`Wrap`/sentinel dari file ini apa adanya.
- Wiring akhir (langkah wiring bersama): verifikasi tidak ada `STUB(foundation)` tersisa dan `go build ./...`; fitur ini tidak menyentuh `go.mod`, `modules.go`, `registry.go`, atau file `cmd/*`.

## Selesai bila

- `go build ./internal/fsx/...` sukses untuk bagian milik fitur ini (guard fitur; tanpa unit test). Kegagalan yang murni berasal dari file fitur lain tidak dihitung, tetapi tiga file milik fitur ini harus bebas error kompilasi dan `gofmt`.
- Tidak ada `STUB(foundation)` tersisa di `errors.go`, `binary.go`, `readwrite.go`.
- Signature publik identik dengan Kontrak (tidak ada perubahan signature dari Foundation).
- Perilaku sesuai kriteria tiket: `Read` `.md/.markdown/.txt` -> `markdown`, lainnya `text`; biner (NUL atau non-UTF-8 di 8 KiB awal) -> kode -32000 tanpa membaca isi penuh; `offset/limit` menghasilkan UTF-8 valid dengan `Offset/Length/EOF` nyata; tanpa `limit` membaca seluruh file; `Write` dengan `expectedMtime` beda -> -32001 dan file tidak berubah; penulisan atomik (tmp+fsync+rename), permission lama dipertahankan, mtime/size baru dari `Stat`; `ResolveUnder` menolak `..`, absolut, dan symlink keluar root (-32003); tak ada -> -32002; folder dibaca/ditulis sebagai file -> -32003; `content` non-UTF-8 -> -32003.
- Hanya tiga ownedPaths yang berubah; file sementara di luar ownedPaths sudah dihapus.
