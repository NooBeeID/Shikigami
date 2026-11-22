# Plan: Penulis halaman, index.md, dan state.json

## Metadata

- Tiket: `docs/tickets/grimoire-ticket-01-grimoire-build.md` (id `grimoire-ticket-01-grimoire-build`)
- Fitur: `pages-index` (Penulis halaman, index.md, dan state.json)
- ownedPaths: `internal/grimoire/pages.go`, `internal/grimoire/index.go`, `internal/grimoire/build.go`
- Depends on foundation: ya. Foundation (`00-foundation.md`) sudah menaruh `types.go` final dan stub berkompilasi untuk `store.go`, `scan.go`, `modules.go`, `hash.go`, dan stub awal ketiga berkasmu. Tugasmu menimpa PENUH ketiga stub itu. Sumber kebenaran kontrak ada di `overview.md` bagian 2 (jangan diubah). Bila folder `internal/grimoire/` belum ada saat kamu mulai, foundation belum jalan: berhenti dan laporkan, jangan membuat berkas di luar ownedPaths.

## Tujuan

Menulis tiga berkas yang mengubah daftar `Module` (dari `scan-modules`) menjadi isi grimoire statik di disk: halaman bab Markdown (baris pertama `Sumber:`), `index.md` (header konstan, satu baris per buku, dilipat deterministik sampai `<= IndexMaxBytes`), dan `state.json`. Plus `Build(store, projectID, projectPath)` yang merangkai scan, grup, hash, tulis halaman, lalu `index.md` dan `state.json` paling akhir. Tanpa LLM, tanpa tanggal/narasi di halaman, tanpa menulis apa pun di dalam `projectPath`.

## Path yang dimiliki

HANYA tiga berkas ini yang boleh dibuat/diubah:

- `internal/grimoire/pages.go`
- `internal/grimoire/index.go`
- `internal/grimoire/build.go`

Dilarang menyentuh: `types.go`, `store.go`, `scan.go`, `modules.go`, `hash.go`, `ipc.go` (milik fitur lain), `cmd/*`, `frontend/*`, `go.mod`/`go.sum`, `overview.md`, plan lain. Tanpa dependency baru (hanya stdlib + `Store`/tipe paket sendiri). Tanpa unit test. Bila ada galat build yang berasal dari berkas fitur lain, jangan diperbaiki; laporkan.

Aturan nama: semua konstanta/helper tak-diekspor milikmu diberi nama khas fitur ini (lihat langkah) supaya tidak bertabrakan dengan identifier tak-diekspor di berkas fitur lain dalam paket `grimoire`. `IndexMaxBytes` dan `IndexHeader` HANYA didefinisikan di `index.go`; `Page` HANYA di `pages.go`. Sentinel error sudah ada di `types.go`; jangan definisikan ulang.

## Kontrak

Disediakan (signature persis, overview 2.3; JANGAN diubah):

```go
// pages.go
type Page struct {
    Path    string // relatif direktori grimoire: books/<buku>/<bab>.md (selalu "/")
    Content string
}
func RenderChapter(m Module) string
func RenderBooks(mods []Module) ([]Book, []Page)

// index.go
const IndexMaxBytes = 16384
const IndexHeader = "..." // teks tetap, memuat literal {{grimoire_dir}}
func IndexLine(b Book) string
func BuildIndex(books []Book) string

// build.go
func Build(store *Store, projectID, projectPath string) (Summary, error)
```

Tambahan tak-diekspor (internal fitur ini, tidak dirujuk fitur lain): `renderBooks(mods []Module, hashes map[string]string) ([]Book, []Page)` dan `renderChapterBody(...)` (lihat langkah). Alasan: `Module` tidak punya field hash dan signature `RenderChapter`/`RenderBooks` dikunci, sedangkan bagian `## Hash` butuh hash modul. Versi diekspor memanggil versi tak-diekspor dengan `hashes == nil` (hash tampil `(belum dihitung)`); `Build` memanggil versi dengan hash nyata.

Dipakai (sudah ada sebagai stub foundation atau implementasi fitur lain; pakai sesuai signature, jangan ubah):

- `Module{Name, Dir, Kind, Files []FileInfo, Symbols, Sources []string, Tag string, Truncated int}` dan `FileInfo{Path, Size, Binary, Large}`.
- `Scan(root string) ([]FileInfo, error)`, `GroupModules(files []FileInfo) []Module`, `HashModule(root string, m Module) (string, error)`.
- `Store`: `ValidateProject`, `Dir`, `IndexPath`, `StatePath`, `RemoveBooks`, `WriteFile(projectID, rel, data)`, `WriteState`. Galat "tidak ada" dari store dapat dideteksi `errors.Is(err, os.ErrNotExist)`.
- `Snake`, `CamelTag`.
- Tipe `Book`, `Chapter`, `State`, `ModuleState`, `Summary` dan sentinel (`ErrInvalidProject`, dst.) dari `types.go`.

Format yang harus dipenuhi: overview 2.5 (bab, `index.md`, pelipatan, `state.json`, pemecahan modul >40 berkas).

## Langkah

Semua komentar dan pesan galat dalam bahasa Indonesia; identifier Inggris. Baris pertama berkas hasil TIDAK lagi memuat komentar `STUB(foundation)` (hapus). Jalankan `gofmt` di akhir.

### 1. `index.go`

Package `grimoire`; import `sort`, `strings`, `fmt`.

1. Konstanta:
   ```go
   const IndexMaxBytes = 16384
   const IndexHeader = "# Grimoire\n" +
       "Index pengetahuan project ini. Baca index ini dulu, lalu buka halaman buku/bab yang relevan sebelum membaca source.\n" +
       "Halaman ada di {{grimoire_dir}}/books/<buku>/<bab>.md\n" +
       "Baris pertama tiap bab berisi \"Sumber:\" (path relatif repo) untuk verifikasi ke source.\n" +
       "Format baris: nama_buku (bab1, bab2) [tag,camelTag]\n" +
       "Baris berakhiran \"warning: stale\" berarti halaman tertinggal dari source; jalankan grimoire build.\n"
   ```
   Teks konstan: tanpa tanggal, jumlah, atau nilai dinamis. `{{grimoire_dir}}` ditulis literal, TIDAK diganti di sini. Header pasti jauh di bawah `IndexMaxBytes`.
2. `func IndexLine(b Book) string`: `b.Name + " (" + strings.Join(nama bab berurutan sesuai b.Chapters, ", ") + ")"`; bila `len(b.Tags) > 0` tambah `" [" + strings.Join(b.Tags, ",") + "]"`; bila ada `Chapter.Stale == true` tambah `" warning: stale, jalankan grimoire build"`. Tanpa newline di ujung.
3. `func BuildIndex(books []Book) string`:
   - Salin `books` lalu urutkan dengan `sort.SliceStable` menurut `Name` (jangan memutasi argumen).
   - Buat helper tak-diekspor `indexRender(entries []indexEntry, dropped int) string` dengan `type indexEntry struct{ key, line string }`: hasil = `IndexHeader + "\n"` (satu baris kosong) + tiap `line` diakhiri `"\n"`; bila `dropped > 0` tambah baris penutup `... (N entri dipotong)\n`. Bila `len(entries) == 0 && dropped == 0` kembalikan `IndexHeader` saja (project kosong: header saja).
   - Entri awal: satu `indexEntry{key: b.Name, line: IndexLine(b)}` per buku.
   - Bila `len(indexRender(entries,0)) <= IndexMaxBytes` kembalikan itu.
   - Pelipatan: kelompokkan buku berdasarkan awalan = bagian nama buku sebelum `_` pertama (seluruh nama bila tak ada `_`). Hanya kelompok dengan >= 2 buku yang boleh dilipat. Urutkan kelompok kandidat deterministik: jumlah buku menurun, lalu total bab menurun, lalu awalan menaik. Lipat satu per satu: ganti semua entri anggota kelompok dengan satu `indexEntry{key: awalan, line: awalan + "/* (" + N + " buku) [" + CamelTag(awalan) + "]"}` (tambahkan juga akhiran stale `" warning: stale, jalankan grimoire build"` bila ada anggota yang stale). Urutkan ulang entri berdasarkan `key` (stabil; tie-break `line`) setelah tiap lipatan, hitung ulang ukuran, berhenti begitu muat.
   - Jika semua kelompok >= 2 sudah dilipat dan masih `> IndexMaxBytes`: potong dari belakang. Ulang: buang entri terakhir (urutan sudah terurut), `dropped++`, hitung ulang dengan baris penutup, sampai muat. Hasil akhir DIJAMIN `<= IndexMaxBytes` (loop berhenti saat `entries` kosong; header sendiri muat).
   - Seluruh langkah deterministik: tanpa map iteration tak terurut yang memengaruhi keluaran (urutkan kunci map sebelum dipakai), tanpa waktu/acak.

### 2. `pages.go`

Package `grimoire`; import `fmt`, `path`, `sort`, `strings`.

1. Konstanta tak-diekspor: `pagesSplitFiles = 40` (modul dengan `len(m.Files) > 40` dipecah), `pagesMaxSources = 20`, `pagesHashLen = 12`, `pagesRestChapter = "lainnya"`.
2. `type Page struct { Path, Content string }`.
3. Helper `pagesSourcesLine(sources []string) string`: kosong -> `"Sumber: -"`; <= 20 -> `"Sumber: " + Join(sources, ", ")`; > 20 -> 20 path pertama dijoin `", "` lalu `", ..."` (tetap satu baris).
4. Helper `renderChapterBody(sources []string, files []FileInfo, symbols []string, symbolNote string, truncated int, hash string) string`. Isi, tiap bagian dipisah satu baris kosong dan berkas berakhir `"\n"`:
   ```
   Sumber: a, b, c

   ## Berkas
   - path/relatif.go (1234 B)
   - gambar.bin (99 B, biner)
   - besar.txt (2000000 B, besar)

   ## Simbol
   - NamaSimbol
   (dipotong: N berkas lagi)

   ## Hash
   abcdef123456
   ```
   - Ukuran selalu dalam byte (`%d B`); tambahkan `, biner` bila `Binary` dan/atau `, besar` bila `Large` (urutan: biner dulu).
   - Bagian Berkas: `files` menurut urutan path (sudah urut dari `Scan`; urutkan ulang aman). Kosong -> `(tidak ada)`.
   - Bagian Simbol: satu `- sym` per simbol; bila `symbols` kosong tulis `symbolNote` bila tidak kosong, selain itu `(tidak ada)`. Baris `(dipotong: N berkas lagi)` ditambahkan HANYA bila `truncated > 0`.
   - Bagian Hash: `hash` dipotong ke 12 karakter awal; bila `hash == ""` tulis `(belum dihitung)`.
   - Tanpa tanggal, changelog, atau narasi.
5. `func RenderChapter(m Module) string` = `renderChapterBody(m.Sources, m.Files, m.Symbols, "", m.Truncated, "")`.
6. `func RenderBooks(mods []Module) ([]Book, []Page) { return renderBooks(mods, nil) }`.
7. `func renderBooks(mods []Module, hashes map[string]string) ([]Book, []Page)`:
   - Salin `mods`, urutkan menurut `Name` (stabil). Hasil `books`/`pages` selalu urut nama buku lalu urutan bab; kembalikan slice kosong non-nil bila `mods` kosong.
   - Per modul: `book := Snake(m.Name)`; `Book.Name = book`; `Book.Tags` = daftar tak-duplikat dengan urutan: `m.Tag` (bila tak kosong), lalu tag induk `CamelTag(path.Base(path.Dir(m.Dir)))` bila `m.Dir` punya induk bukan `"."`/`"/"`/kosong dan hasilnya tak kosong dan berbeda dari `m.Tag`. `Tags` tidak boleh nil (`[]string{}` bila kosong).
   - Hash modul: `hashes[m.Name]` (aman untuk map nil).
   - Modul tidak dipecah (`len(m.Files) <= 40`, atau pemecahan hanya menghasilkan satu bab): satu bab bernama `book` (sama dengan nama buku), path `books/<book>/<book>.md`, `Chapter.Sources = m.Sources` (salinan, non-nil), konten `renderChapterBody(m.Sources, m.Files, m.Symbols, "", m.Truncated, hash)`.
   - Modul dipecah (`len(m.Files) > 40`): relatifkan tiap `FileInfo.Path` terhadap `m.Dir` (`"."` -> path apa adanya; selain itu `strings.TrimPrefix(p, m.Dir+"/")`). Segmen pertama sebelum `/` = sub-direktori langsung; berkas tanpa `/` (langsung di `m.Dir`) masuk bab sisa. Bab per sub-direktori bernama `Snake(subdir)`, urut nama; bab sisa `lainnya` selalu terakhir dan hanya dibuat bila ada berkas. Bila tidak ada sub-direktori sama sekali, perlakukan sebagai modul tidak dipecah. Nama bab dijamin unik dalam buku (tabrakan, termasuk dengan `lainnya`, diberi akhiran `_2`, `_3` menurut urutan). Tiap bab: `Chapter.Sources` = path repo (bukan relatif) berkas bab itu, urut; konten dari `renderChapterBody(sources_bab, files_bab, ...)`. Simbol modul tidak bisa dipetakan per berkas: taruh SEMUA `m.Symbols` dan baris `Truncated` di bab pertama (urutan bab) saja; bab lain mendapat `symbolNote = "(simbol modul tercatat di bab " + <nama bab pertama> + ")"` dan `truncated = 0`. Hash modul yang sama dicantumkan di semua bab.
   - `Chapter.Path = "books/" + book + "/" + chapterName + ".md"`, `Chapter.Stale = false`. `Page{Path: chapter.Path, Content: ...}` satu per bab, urutan sama dengan bab.

### 3. `build.go`

Package `grimoire`; import `errors`, `fmt`, `os`, `path/filepath`, `time`.

`func Build(store *Store, projectID, projectPath string) (Summary, error)` urutan:

1. Validasi: `store == nil` -> galat `errors.New("store grimoire kosong")`. `store.ValidateProject(projectID)` (kembalikan galat apa adanya agar `errors.Is(err, ErrInvalidProject)` tetap berlaku). `projectPath` harus `filepath.IsAbs` dan `os.Stat` menunjukkan direktori; selain itu `fmt.Errorf("path project tidak valid: %s", projectPath)` (atau bungkus galat stat dengan `%w`). Bersihkan dengan `filepath.Clean`.
2. `files, err := Scan(root)`; `mods := GroupModules(files)`; untuk tiap modul `h, err := HashModule(root, m)`, simpan di `hashes[m.Name]`; galat dibungkus `fmt.Errorf("hash modul %s: %w", m.Name, err)`. Proyek kosong (tanpa modul) BUKAN galat.
3. `books, pages := renderBooks(mods, hashes)` dan `index := BuildIndex(books)` DIHITUNG sebelum menyentuh disk (galat render tidak merusak grimoire lama).
4. Persiapan tulis ulang (urutan penting agar kegagalan di tengah tidak meninggalkan `index.md` yang menunjuk bab tak ada): hapus lebih dulu `store.IndexPath(projectID)` dan `store.StatePath(projectID)` dengan `os.Remove` (abaikan galat bila `errors.Is(err, os.ErrNotExist)`); lalu `store.RemoveBooks(projectID)` (abaikan galat not-exist). Ini juga menghapus buku milik modul yang sudah hilang.
5. Tulis semua halaman: untuk tiap `Page` `store.WriteFile(projectID, p.Path, []byte(p.Content))`; galat dibungkus `fmt.Errorf("tulis halaman %s: %w", p.Path, err)`.
6. Setelah SEMUA halaman tertulis: `store.WriteFile(projectID, "index.md", []byte(index))`.
7. Terakhir: `store.WriteState(projectID, State{ProjectID: projectID, BuiltAt: time.Now().UTC(), Modules: map nama modul (= nama buku = Snake(m.Name)) -> ModuleState{Hash: hashes[m.Name], Sources: salinan m.Sources non-nil}})`. `Stale` tidak diisi (false/omitempty). `Modules` selalu map non-nil.
8. Kembalikan `Summary{Books: len(books), Pages: len(pages), IndexBytes: len(index)}`. Proyek kosong: `Summary{0, 0, len(IndexHeader)}`.

Jaminan yang harus dipegang: tidak ada pemanggilan tulis/hapus/buat apa pun di bawah `projectPath` (hanya baca lewat `Scan`/`HashModule`); semua tulis lewat `Store` atau `os.Remove` pada path yang diturunkan dari `store.IndexPath`/`store.StatePath`. Kunci nama di `State.Modules` harus sama dengan `Book.Name` (kunci tunggal untuk tiket 3 nanti).

### 4. Verifikasi

`gofmt -l internal/grimoire/pages.go internal/grimoire/index.go internal/grimoire/build.go` (harus kosong), lalu `go build ./...` dan opsional `go vet ./internal/grimoire/...`. Pastikan `git diff --stat -- go.mod go.sum` kosong dan `git status` hanya menunjukkan tiga berkas milikmu (selain pekerjaan fitur lain).

## Perubahan

- Menimpa penuh `internal/grimoire/pages.go`: `Page`, `RenderChapter`, `RenderBooks`, tak-diekspor `renderBooks`, `renderChapterBody`, `pagesSourcesLine`, konstanta `pages*`.
- Menimpa penuh `internal/grimoire/index.go`: `IndexMaxBytes`, `IndexHeader`, `IndexLine`, `BuildIndex`, tak-diekspor `indexEntry`, `indexRender`.
- Menimpa penuh `internal/grimoire/build.go`: `Build`.
- Tidak ada berkas lain dibuat/diubah.

## Wiring

Yang diekspos untuk penyambungan nanti (JANGAN disambungkan sendiri):

- `grimoire.Build(store, projectID, projectPath) (Summary, error)`: dipanggil oleh `ipc.go` (fitur `ipc-ctl`) di handler `grimoire.build`. Galat `ErrInvalidProject` tetap dapat dideteksi `errors.Is`.
- `IndexHeader` berisi placeholder literal `{{grimoire_dir}}` untuk diganti saat penyuntikan (tiket 2), bukan di fitur ini.
- Tata letak keluaran di `<DataDir>/grimoire/<projectId>/`: `index.md`, `state.json`, `books/<buku>/<bab>.md`; `Chapter.Path` selalu `books/<buku>/<bab>.md` dengan `/`. Kunci `state.json.modules` = nama buku.
- `ipc-ctl` tidak perlu memanggil `RenderBooks`/`BuildIndex`; keduanya sudah dipanggil di dalam `Build`.

Tidak ada edit di `cmd/`, `frontend/`, atau titik registrasi lain.

## Selesai bila

- `go build ./...` sukses (guard utama; tanpa unit test). `gofmt -l` pada tiga berkas kosong.
- Ketiga stub foundation sudah diganti implementasi penuh; tidak ada komentar `STUB(foundation)` tersisa di `pages.go`, `index.go`, `build.go`; signature persis sesuai overview 2.3.
- `RenderChapter` menghasilkan baris pertama `Sumber: ...` (maks 20 path lalu `, ...`) diikuti `## Berkas`, `## Simbol`, `## Hash` (hash 12 karakter), tanpa tanggal/narasi; modul terpotong menambah `(dipotong: N berkas lagi)`.
- `RenderBooks`: satu modul satu buku; modul >40 berkas dipecah per sub-direktori langsung + bab `lainnya`; keluaran urut nama dan deterministik; `Stale` selalu false.
- `IndexLine` berformat `nama (bab1, bab2) [tag,camelTag]` dengan akhiran stale bila ada bab stale; `BuildIndex` byte-identik untuk input sama, melipat per awalan sampai `<= IndexMaxBytes`, dengan fallback pemotongan yang menjamin batas.
- `Build`: urutan scan, grup, hash, tulis semua halaman, lalu `index.md`, lalu `state.json` terakhir; `index.md` dan `state.json` lama dihapus sebelum penulisan ulang; buku lama dihapus; project kosong menghasilkan `index.md` header saja dan `Summary{0,0,n}` tanpa galat; tidak ada penulisan di `projectPath`.
- Tidak ada perubahan pada `go.mod`/`go.sum` maupun berkas di luar tiga ownedPaths.
