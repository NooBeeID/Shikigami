---
id: ai-terminal-ticket-03-file-service
title: File Service
spec: docs/superpowers/specs/2026-11-21-ai-terminal-design.md
status: implemented
wave: 2
depends_on: [ai-terminal-ticket-01-app-base]
---
# File Service

## Tujuan

Menyediakan layanan di daemon (`aitermd`) untuk membaca tree project, membaca file, menulis file, dan memantau perubahan file di disk, semuanya lewat IPC JSON-RPC (method `fs.*`). Spec (bagian "Cara kerja baca file") menuntut tree malas, tanpa batas ukuran file, biner ditolak, deteksi konflik saat simpan, dan watch hanya untuk file yang sedang tampil, tanpa watcher global. Epic ini backend saja; UI viewer/editor dikerjakan tiket lain (File Viewer). Tiket ini harus langsung jalan dan build hijau sendiri: setelah selesai, `aitermd` mengekspos `fs.*` dan `aitermctl fs ls/cat` bisa dipakai untuk uji manual.

## Batasan

In scope:
- Package `internal/fsx` (logika murni, tanpa ketergantungan ke package epic lain selain `internal/ipc` di file `ipc.go`).
- Method IPC `fs.list`, `fs.read`, `fs.write`, `fs.watch`, `fs.unwatch` dan event `fs.changed.<path>`.
- Modul daemon `cmd/aitermd/wire_fs.go` dan perintah debug `cmd/aitermctl/cmd_fs.go`.
- Ignore default, deteksi biner, kunci konflik berbasis mtime, validasi path di bawah root (anti traversal).

Out of scope:
- Semua kode frontend (tree UI, tab file, editor, markdown render) milik File Viewer.
- Pencarian isi file, syntax highlighting, LSP, git info.
- Pemantauan filesystem global/rekursif; watcher untuk tree; polling apa pun.
- Pembuatan/penghapusan/rename file dan folder (hanya baca dan tulis file yang ada atau baru lewat `fs.write`; lihat catatan di fitur fs-readwrite).
- Pengetahuan tentang workspace/project: root selalu dikirim oleh pemanggil sebagai parameter. Epic ini TIDAK mengimpor `internal/workspace`.
- Mengedit file milik epic lain. Satu-satunya pengecualian: `go get github.com/fsnotify/fsnotify` yang menambah baris require di `go.mod`/`go.sum` (lihat Wiring).

## Kontrak

### Yang dipakai dari app-base (tiket 01)
- `internal/ipc`: `srv.Register(method string, func(ctx context.Context, params json.RawMessage) (any, error))`, `srv.Publish(topic string, payload any)`; kode error JSON-RPC `-32000 ERR_BINARY`, `-32001 ERR_CONFLICT`, `-32002 ERR_NOT_FOUND`, `-32003 ERR_INVALID`. Sebelum menulis `errors.go`/`ipc.go`, baca `internal/ipc` untuk melihat cara resmi sebuah handler mengembalikan error berkode (tipe error ber-kode atau konstanta kode). Gunakan itu; jangan mengubah `internal/ipc`.
- `cmd/aitermd/modules.go`: `RegisterModule(name string, func(srv *ipc.Server, d *Deps))`.
- `cmd/aitermctl/registry.go`: `RegisterCommand(...)` (baca signature aktual di file itu) dan `client.Call(method, params, &out)`.

### Yang disediakan: API Go `internal/fsx`

```go
package fsx

type EntryType string // "file" | "dir" | "symlink"

type Entry struct {
    Name  string    `json:"name"`
    Type  EntryType `json:"type"`
    Size  int64     `json:"size"`   // byte; 0 untuk dir
    Mtime int64     `json:"mtime"`  // Unix nanodetik, UTC
}

type Kind string // "markdown" | "text"

type ReadResult struct {
    Content   string `json:"content"`
    Mtime     int64  `json:"mtime"`      // mtime file di disk saat dibaca (Unix ns)
    Kind      Kind   `json:"kind"`
    Size      int64  `json:"size"`       // ukuran total file (byte)
    Offset    int64  `json:"offset"`     // offset byte awal potongan
    Length    int64  `json:"length"`     // byte yang dikembalikan
    EOF       bool   `json:"eof"`
}

type WriteResult struct {
    Mtime int64 `json:"mtime"` // mtime baru setelah menulis
    Size  int64 `json:"size"`
}

func List(root, relDir string) ([]Entry, error)
func Read(root, relPath string, offset, limit int64) (*ReadResult, error)
func Write(root, relPath, content string, expectedMtime int64) (*WriteResult, error)
func ResolveUnder(root, rel string) (abs string, err error) // anti traversal

type Watcher struct{ /* ... */ }
func NewWatcher(onChange func(root, relPath string, ev ChangeEvent)) (*Watcher, error)
func (w *Watcher) Watch(root, relPath string) error
func (w *Watcher) Unwatch(root, relPath string) error
func (w *Watcher) Close() error

type ChangeEvent struct {
    Op    string `json:"op"`    // "write" | "remove" | "rename" | "create"
    Mtime int64  `json:"mtime"` // 0 bila file hilang
}

// errors.go
type Error struct { Code int; Message string }  // implement error
var ErrBinary, ErrConflict, ErrNotFound, ErrInvalid  // sentinel yang memakai kode -32000..-32003
func IsCode(err error, code int) bool
```

Catatan: `ipc.go` bertugas memetakan `*fsx.Error` ke error berkode milik `internal/ipc`. Fungsi-fungsi lain tidak mengimpor `internal/ipc`.

### Yang disediakan: method IPC

Semua parameter objek JSON. `root` = path absolut folder project (dikirim pemanggil); `path`/`relDir` = path relatif terhadap `root`, memakai `/` sebagai pemisah, `""` atau `"."` berarti root itu sendiri (hanya untuk `fs.list`).

| Method | Params | Hasil |
|---|---|---|
| `fs.list` | `{root, relDir}` | `{entries: Entry[]}` urut: folder dulu, lalu file, masing-masing alfabetis tanpa peka huruf besar |
| `fs.read` | `{root, path, offset?, limit?}` | `ReadResult` |
| `fs.write` | `{root, path, content, expectedMtime}` | `WriteResult` |
| `fs.watch` | `{root, path}` | `{}` |
| `fs.unwatch` | `{root, path}` | `{}` |

Event (server ke klien, topik via `srv.Publish`): `fs.changed.<absPath>` dengan payload `{root, path, op, mtime}`. `<absPath>` adalah path absolut file hasil `ResolveUnder` (bukan relatif), agar klien cukup `Subscribe("fs.changed.")` lalu memfilter.

Kode error:
- `-32000 ERR_BINARY`: file biner (`fs.read`). Message: `binary file not supported`.
- `-32001 ERR_CONFLICT`: `expectedMtime` tidak sama dengan mtime disk saat `fs.write`. Message memuat mtime disk; `data` error (bila ipc mendukung) berisi `{currentMtime}`.
- `-32002 ERR_NOT_FOUND`: root/folder/file tidak ada.
- `-32003 ERR_INVALID`: parameter salah, path keluar dari root, target adalah folder (untuk read/write) atau file (untuk list), offset/limit negatif.

## Fitur

Ketiga fitur bisa dikerjakan paralel karena kontrak API Go di atas sudah tetap. Satu-satunya dependensi: `fs-watch-ipc` memanggil fungsi dari dua fitur lain (`List`, `Read`, `Write`, `ResolveUnder`, tipe `Error`). Agent yang mengerjakan `fs-watch-ipc` membuat file stub sementara TIDAK boleh; cukup kerjakan `watch.go` dulu, lalu `ipc.go`/wiring setelah dua fitur lain merge (atau kerjakan berurutan). `ResolveUnder` dan `Error` dimiliki `fs-readwrite` (`errors.go`, `readwrite.go`); `fs-tree` juga memakainya, jadi kedua fitur memakai kontrak di atas, dan integrasi akhir dilakukan di wiring. Build per fitur tetap harus hijau: bila fungsi yang dipakai belum ada saat fitur dikerjakan sendirian, kerjakan `errors.go` + `ResolveUnder` terlebih dahulu (foundation kecil, dimiliki `fs-readwrite`).

### fs-tree: Tree malas dan ignore

Deskripsi:
- `List(root, relDir)` membaca satu folder saja dengan `os.ReadDir`, tanpa rekursi dan tanpa pemindaian penuh. Untuk tiap entry kembalikan nama, tipe, ukuran, mtime (gunakan `entry.Info()`; bila gagal pada satu entry, lewati entry itu, jangan gagalkan seluruh list).
- Symlink dilaporkan bertipe `symlink` (tanpa diikuti); jangan rekursi lewat symlink. Bila symlink menunjuk folder, tetap `symlink`; UI memutuskan.
- Ignore default (cocok nama persis entry, bukan path): `node_modules`, `.git`, `target`, `dist`, `build`, `.next`, `.nuxt`, `.svelte-kit`, `.cache`, `.venv`, `venv`, `__pycache__`, `.idea`, `.DS_Store`, `vendor`, `Pods`, `.gradle`, `coverage`. Daftar ini ada di `ignore.go` sebagai variabel `DefaultIgnore` (slice string) dengan fungsi `IsIgnored(name string) bool` (map lookup). Hanya ignore berbasis nama; JANGAN membaca `.gitignore` di versi awal.
- File tersembunyi lain (mis. `.env`, `.gitignore`) TIDAK di-ignore.
- Validasi lewat `ResolveUnder(root, relDir)`; `relDir` yang menunjuk file mengembalikan `ErrInvalid`; tidak ada mengembalikan `ErrNotFound`.
- Tanpa cache dan tanpa goroutine latar; tiap panggilan membaca disk.

Acceptance criteria:
- [ ] `go build ./...` hijau dengan `internal/fsx/tree.go` dan `ignore.go` ada.
- [ ] `List` hanya membaca satu level folder (tidak ada `filepath.Walk`/rekursi).
- [ ] Entry bernama sesuai `DefaultIgnore` tidak muncul di hasil.
- [ ] Urutan: folder dulu lalu file, alfabetis tanpa peka huruf besar.
- [ ] `relDir` dengan `..` yang keluar root ditolak `ErrInvalid`.
- [ ] Folder tak ada mengembalikan `ErrNotFound`; satu entry yang gagal di-stat tidak menggagalkan list.
- [ ] Tidak ada goroutine, watcher, atau timer di file ini.

ownedPaths: `internal/fsx/tree.go`, `internal/fsx/ignore.go`. Bisa paralel dengan fitur lain, kecuali memakai `ResolveUnder`/`Error` dari `fs-readwrite` (lihat catatan atas).

### fs-readwrite: Baca/tulis dan deteksi biner

Deskripsi:
- `errors.go`: tipe `Error{Code int; Message string}`, sentinel `ErrBinary` (-32000), `ErrConflict` (-32001), `ErrNotFound` (-32002), `ErrInvalid` (-32003), helper `IsCode`. Konstruksi error dengan detail memakai `fmt.Errorf` yang membungkus (`errors.Is/As` harus bekerja).
- `ResolveUnder(root, rel)`: `root` harus absolut dan ada; bersihkan dengan `filepath.Clean(filepath.Join(root, rel))`; tolak bila hasil bukan `root` atau berawalan `root + string(filepath.Separator)`; tolak `rel` absolut dan yang mengandung byte NUL. Untuk file yang sudah ada, evaluasi symlink (`filepath.EvalSymlinks`) pada hasil dan pada root lalu cek ulang bahwa hasilnya masih di bawah root (cegah keluar lewat symlink). Untuk `Write` ke file baru, cek symlink pada folder induknya.
- `binary.go`: `IsBinary(sample []byte) bool`: biner bila 8 KiB pertama mengandung byte NUL, atau bukan UTF-8 valid (`utf8.Valid`, dengan toleransi potongan multibyte di akhir sampel). File kosong = teks. Pemeriksaan hanya membaca awal file, bukan seluruh isi.
- `Read(root, relPath, offset, limit)`: buka file, deteksi biner dari 8 KiB pertama SEBELUM membaca isi (biner -> `ErrBinary`), lalu baca. `limit <= 0` berarti baca sampai akhir file (tanpa batas ukuran). Dengan `offset/limit` kembalikan potongan byte; sesuaikan batas potongan ke batas karakter UTF-8 (mundurkan akhir potongan ke awal rune valid; majukan offset bila jatuh di tengah rune) agar `Content` selalu string valid; `Length`, `Offset`, dan `EOF` mencerminkan potongan nyata. `Mtime` diambil dari `Stat` file yang sama (Unix nanodetik). `Kind`: ekstensi `.md`, `.markdown`, `.txt` (tanpa peka huruf besar) -> `markdown`; lainnya `text`. `Size` = ukuran total. Hindari menyalin ganda isi besar (baca langsung ke buffer ukuran tepat).
- Penolakan: target folder -> `ErrInvalid`; tak ada -> `ErrNotFound`.
- `Write(root, relPath, content, expectedMtime)`: `Stat` file; bila file ada dan `expectedMtime != 0` dan mtime disk (Unix ns) berbeda -> `ErrConflict` (bawa mtime disk di message). Bila `expectedMtime == 0` hanya diizinkan bila file belum ada (membuat file baru; folder induk harus sudah ada, tidak membuat folder); bila file ada dan `expectedMtime == 0` -> `ErrConflict`. Penulisan atomik: tulis ke file sementara di folder yang sama (`.<nama>.aiterm-tmp-<rand>`), `fsync`, pertahankan permission file lama (default 0644 untuk baru), lalu `os.Rename` ke target; hapus sementara bila gagal. Mengembalikan mtime dan size baru dari `Stat` setelah rename. Tolak menulis ke file biner yang sudah ada? Tidak: yang dicek hanya konflik; tapi `content` bukan UTF-8 valid -> `ErrInvalid`. Tanpa batas ukuran `content`.
- Catatan: pengecekan mtime dan penulisan tidak atomik terhadap penulis lain (jendela kecil); diterima untuk versi awal.

Acceptance criteria:
- [ ] `go build ./...` hijau dengan `readwrite.go`, `binary.go`, `errors.go`.
- [ ] `Read` pada `.md`/`.txt` mengembalikan `Kind=markdown`; `.go`/`.js`/lainnya `text`.
- [ ] File biner (mengandung NUL atau bukan UTF-8) mengembalikan error dengan kode -32000 tanpa membaca isi penuh.
- [ ] `Read` dengan `offset/limit` mengembalikan potongan valid UTF-8, `EOF` benar; tanpa `limit` membaca seluruh file tanpa batas ukuran.
- [ ] `Write` dengan `expectedMtime` berbeda dari disk mengembalikan kode -32001 dan tidak mengubah file.
- [ ] `Write` atomik (tmp + rename), mtime baru dikembalikan, permission lama dipertahankan.
- [ ] `ResolveUnder` menolak `..`, path absolut di luar root, dan symlink yang keluar root (kode -32003).
- [ ] Path tak ada -> -32002; folder dibaca sebagai file -> -32003.

ownedPaths: `internal/fsx/readwrite.go`, `internal/fsx/binary.go`, `internal/fsx/errors.go`. Bisa paralel dengan fitur lain; `errors.go` + `ResolveUnder` dipakai fitur lain jadi kerjakan paling awal.

### fs-watch-ipc: Watch file tampil dan method IPC

Deskripsi:
- `watch.go`: `Watcher` membungkus satu `fsnotify.Watcher`. `Watch(root, relPath)` meresolusi path absolut (`ResolveUnder`), memastikan target adalah file biasa, lalu mendaftarkan FOLDER INDUK ke fsnotify (agar rename-atomik oleh editor/agent tetap terdeteksi) dan mencatat file yang dipantau dalam peta `absPath -> refcount`. Tidak merekursi dan tidak memantau folder lain. Event fsnotify disaring: hanya path yang ada di peta yang diteruskan. Debounce per path 100 ms (satu timer per path aktif; hentikan timer saat unwatch). Tiap event meneruskan `onChange(root, relPath, ChangeEvent{Op, Mtime})` dengan mtime dari `Stat` terbaru (0 bila file hilang). Bila folder induk tidak lagi dipantau oleh file mana pun (refcount folder 0), `Remove` dari fsnotify. `Close()` menutup watcher dan goroutine. Satu goroutine pembaca event untuk seluruh Watcher; tidak ada polling dan tidak ada ticker.
- Pemanggilan `Watch` berulang untuk path yang sama menaikkan refcount (beberapa tab/klien); `Unwatch` menurunkannya.
- `ipc.go`: `func Register(srv *ipc.Server) (closeFn func())` mendaftarkan `fs.list`, `fs.read`, `fs.write`, `fs.watch`, `fs.unwatch`: decode params (JSON ke struct, error decode -> `ERR_INVALID`), panggil fungsi `fsx`, petakan `*fsx.Error` ke error berkode milik `internal/ipc`, dan kembalikan hasil sesuai tabel di Kontrak. Callback watcher memanggil `srv.Publish("fs.changed."+absPath, payload)`. Setelah `fs.write` sukses TIDAK mengirim event manual; event berasal dari watcher (UI yang menyimpan memakai mtime hasil `WriteResult` untuk membedakan tulis sendiri).
- `cmd/aitermd/wire_fs.go`: `init()` memanggil `RegisterModule("fs", func(srv *ipc.Server, d *Deps){ ... fsx.Register(srv) ... })`. Bila `Deps` punya mekanisme shutdown/cleanup (baca `modules.go`), daftarkan `closeFn` di sana; bila tidak, biarkan (proses daemon berhenti membersihkan sendiri).
- `cmd/aitermctl/cmd_fs.go`: `init()` memanggil `RegisterCommand` untuk grup `fs` dengan subperintah: `fs ls <root> [relDir]` (cetak satu entry per baris: tipe, ukuran, nama) dan `fs cat <root> <path>` (cetak `content` ke stdout; error cetak ke stderr dengan kode keluar non-nol, termasuk pesan jelas untuk ERR_BINARY/ERR_NOT_FOUND). Opsional: `fs write <root> <path> <expectedMtime>` membaca isi dari stdin. Ikuti gaya registrasi yang sudah dipakai `registry.go`.

Acceptance criteria:
- [ ] `go build ./...` hijau (termasuk `cmd/aitermd` dan `cmd/aitermctl`).
- [ ] `aitermd` yang berjalan melayani `fs.list/read/write/watch/unwatch`; `aitermctl fs ls <root>` dan `aitermctl fs cat <root> <file>` menghasilkan keluaran benar terhadap daemon nyata.
- [ ] Mengubah file yang di-watch dari luar (mis. `echo >> file`) memicu event `fs.changed.<absPath>` sekali (setelah debounce) dengan payload `{root, path, op, mtime}`.
- [ ] Tidak ada watcher untuk file yang belum di-`fs.watch`; setelah `fs.unwatch` (refcount 0) event berhenti dan folder induk dilepas dari fsnotify.
- [ ] Tidak ada ticker/polling; daemon idle tanpa watch tidak membuat goroutine fsx tambahan selain yang dibuat saat `NewWatcher` (buat `Watcher` malas saat `fs.watch` pertama).
- [ ] Error dari `fsx` muncul di klien dengan kode -32000/-32001/-32002/-32003 yang benar.
- [ ] Menyimpan file via `fs.write` lalu event berikutnya membawa mtime yang sama dengan `WriteResult.mtime`.

ownedPaths: `internal/fsx/watch.go`, `internal/fsx/ipc.go`, `cmd/aitermd/wire_fs.go`, `cmd/aitermctl/cmd_fs.go`. Dikerjakan bersama fitur lain bila kontrak API Go dipatuhi; integrasi `ipc.go` butuh `fs-tree` dan `fs-readwrite` sudah ada agar build hijau, jadi kerjakan di akhir (atau kerjakan `watch.go` dulu, `ipc.go`, `wire_fs.go`, `cmd_fs.go` setelahnya).

## Path yang dimiliki

Hanya path berikut yang boleh dibuat/diubah:
- `internal/fsx/` (`tree.go`, `ignore.go`, `readwrite.go`, `binary.go`, `errors.go`, `watch.go`, `ipc.go`)
- `cmd/aitermd/wire_fs.go`
- `cmd/aitermctl/cmd_fs.go`

Pengecualian tunggal: `go.mod`/`go.sum` hanya berubah akibat `go get github.com/fsnotify/fsnotify` (versi stabil terbaru, mis. v1.8.x) lalu `go mod tidy`; jangan menyunting file itu secara manual dan jangan mengubah baris lain.

## Wiring

- `cmd/aitermd/wire_fs.go` mendaftarkan modul `fs` lewat `RegisterModule` (registri `init()` milik app-base); tidak ada edit ke `modules.go` atau `main.go`.
- `cmd/aitermctl/cmd_fs.go` mendaftarkan perintah lewat `RegisterCommand`; tidak ada edit ke `registry.go`.
- Tambah dependensi fsnotify di `go.mod`/`go.sum` (pengecualian di atas). Epic wave 2 lain mungkin juga menambah require; bila konflik, gabungkan dengan menjalankan `go mod tidy`.
- Konsumen di luar epic (tidak dikerjakan di sini): File Viewer (tiket 07) memanggil `fs.list/read/write/watch/unwatch` lewat `frontend/src/lib/ipc.ts` dan berlangganan `fs.changed.`. Kontrak method dan payload di bagian Kontrak tidak boleh berubah tanpa memperbarui tiket itu.
- `root` selalu diberikan pemanggil (UI yang mengambilnya dari workspace project); epic ini tidak membaca konfigurasi workspace.

## Guard

Cukup build berhasil, tanpa unit test:
- `go build ./...` harus lolos.
- `make build` (go build ./... + wails/vite build milik app-base) harus tetap hijau; epic ini tidak menyentuh frontend sehingga build frontend tidak boleh terpengaruh.
- Uji manual opsional (bukan guard): jalankan `aitermd`, lalu `aitermctl fs ls <folder>` dan `aitermctl fs cat <file>`.
