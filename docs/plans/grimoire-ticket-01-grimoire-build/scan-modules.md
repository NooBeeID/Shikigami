# Plan: Pindai project dan kelompokkan modul

## Metadata

- Tiket: `docs/tickets/grimoire-ticket-01-grimoire-build.md` (`grimoire-ticket-01-grimoire-build`)
- Fitur: `scan-modules`
- ownedPaths: `internal/grimoire/scan.go`, `internal/grimoire/modules.go`, `internal/grimoire/hash.go`
- Depends on foundation: ya. Stub `scan.go`, `modules.go`, `hash.go` (komentar baris pertama `// STUB(foundation): ditimpa oleh scan-modules`) dan `types.go` sudah ada dari Foundation. Fitur ini menimpa penuh ketiga berkas stub itu tanpa mengubah signature. Hanya memakai `Snake` dan `CamelTag` dari `store.go` (stub Foundation sudah mendeklarasikannya; implementasi nyata datang dari `contract-types`, jangan mendefinisikan ulang).
- Sumber kebenaran kontrak: `overview.md` bagian 2.3 (tipe `FileInfo`, `Module`, signature, konstanta), bagian 2.5 dan 7.

## Tujuan

Menyediakan tiga fungsi murni (tanpa LLM, tanpa proses eksternal, tanpa menulis berkas) yang menjadi masukan `Build`:

1. `Scan(root)`: menelusuri project (tanpa mengikuti symlink), menghormati `fsx.IsIgnored` dan `.gitignore` berlapis, menandai berkas biner dan besar.
2. `GroupModules(files)`: mengelompokkan berkas menjadi modul Go / web / generik dengan nama unik dan stabil, mengekstrak `Symbols` (maks 60), `Tag` camelCase satu token, `Sources` relatif repo, dan menerapkan batas `maxModuleFiles`/`maxModuleBytes`.
3. `HashModule(root, m)`: SHA-256 atas isi working tree, deterministik.

Semua keluaran deterministik (urut path/nama).

## Path yang dimiliki

HANYA berkas ini yang boleh dibuat/diubah:

- `internal/grimoire/scan.go`
- `internal/grimoire/modules.go`
- `internal/grimoire/hash.go`

Dilarang menyentuh: `types.go`, `store.go`, `pages.go`, `index.go`, `build.go`, `ipc.go`, apa pun di `cmd/`, `frontend/`, `go.mod`, `go.sum`, `overview.md`, dan plan fitur lain. Jangan membuat berkas baru di luar tiga di atas (tidak ada berkas bantu). Tanpa dependency baru: hanya stdlib dan `github.com/NooBeeID/shikigami/internal/fsx`.

## Kontrak

Disediakan (signature final, persis overview 2.3; JANGAN diubah):

```go
type FileInfo struct {
    Path   string // relatif repo, pemisah "/"
    Size   int64
    Binary bool   // NUL pada 8 KiB awal
    Large  bool   // > maxFileBytes; tidak dibaca untuk simbol
}
type Module struct {
    Name      string     // unik, snake_case
    Dir       string     // relatif repo ("." untuk root)
    Kind      string     // "go" | "web" | "generic"
    Files     []FileInfo // urut path
    Symbols   []string   // maks maxSymbols
    Sources   []string   // relatif repo, urut leksikal, "/"
    Tag       string     // satu token camelCase
    Truncated int        // jumlah berkas yang simbolnya dipotong
}
func Scan(root string) ([]FileInfo, error)
func GroupModules(files []FileInfo) []Module
func HashModule(root string, m Module) (string, error)
```

Konstanta (definisikan sekali, hanya di berkas ini): `maxFileBytes = 1 << 20` (`scan.go`); `maxModuleFiles = 200`, `maxModuleBytes = 512 << 10`, `maxSymbols = 60` (`modules.go`).

Dipakai:
- `fsx.IsIgnored(name string) bool` (nama persis, mis. `node_modules`, `.git`, `vendor`, `dist`, `build`). Baca `internal/fsx/ignore.go` sebelum memakai.
- `Snake(s string) string` dan `CamelTag(s string) string` dari paket `grimoire` (`store.go`, milik `contract-types`). `Snake("Session PTY") == "session_pty"`, `CamelTag("session_pty") == "sessionPty"`. Jangan menulis ulang; bila pada saat build masih stub (mengembalikan `s`), tetap berkompilasi, perilaku akhir benar setelah `contract-types` selesai.

Konsumen (jangan diubah): `pages-index` membaca `Module.Name/Dir/Kind/Files/Symbols/Sources/Tag/Truncated`; `Build` memanggil `Scan`, `GroupModules`, `HashModule`.

## Langkah

Semua langkah di package `grimoire`. Pesan galat dan komentar dalam bahasa Indonesia, identifier Inggris. Hanya stdlib (`os`, `io`, `io/fs`, `path`, `path/filepath`, `sort`, `strings`, `bufio`, `bytes`, `regexp`, `crypto/sha256`, `encoding/hex`, `encoding/binary` / `strconv`, `fmt`, `go/parser`, `go/token`, `go/ast`).

### 1. `scan.go`: konstanta, matcher gitignore, Scan

Baris pertama hapus komentar STUB. Isi:

1. `const maxFileBytes = 1 << 20`.
2. Matcher gitignore internal (tidak diekspor):
   ```go
   type ignoreRule struct {
       base     string // direktori (relatif root, "/" , "" untuk root) tempat .gitignore berada
       pattern  string // pola tanpa "!" / "/" awal / "/" akhir
       negate   bool
       dirOnly  bool   // pola berakhir "/"
       anchored bool   // mengandung "/" di awal atau di tengah
       re       *regexp.Regexp // hasil terjemahan glob; nil bila pola tak dikenali
   }
   func parseGitignore(base string, data []byte) []ignoreRule
   func matchRules(rules []ignoreRule, rel string, isDir bool) bool
   ```
   Aturan `parseGitignore`: per baris, buang `\r`, abaikan baris kosong dan awalan `#`; trim spasi akhir tak ter-escape; awalan `!` = negasi; akhiran `/` = `dirOnly`; bila pola mengandung `/` setelah membuang akhiran (termasuk awalan `/`) maka `anchored` (buang `/` awal); selain itu cocok pada nama (basename) di kedalaman mana pun di bawah `base`. Terjemahkan glob ke regexp: `*` = `[^/]*`, `?` = `[^/]`, `**/` di awal atau tengah = `(?:.*/)?`, `/**` di akhir = `/.*`, `**` lain = `.*`, kelas `[...]` boleh diteruskan bila valid; karakter lain di-`regexp.QuoteMeta`. Bila `regexp.Compile` gagal, atau pola memakai sintaks yang tak dikenali, set `re = nil` dan ABAIKAN aturan itu (bukan galat).
   Aturan `matchRules`: untuk setiap rule berurut (aturan dari `.gitignore` yang lebih dalam datang belakangan sehingga menang), hitung path relatif terhadap `rule.base` (lewati rule bila `rel` tidak berada di bawah `base`); anchored dicocokkan dengan path relatif penuh, non-anchored dengan basename; `dirOnly` hanya cocok bila `isDir`. Hasil akhir = status rule terakhir yang cocok (`negate` membuat false, selain itu true); tidak ada yang cocok = false. Catatan semantik: induk yang diabaikan membuat seluruh isinya dilewati (direktori tidak dimasuki), sehingga negasi pada anak dari direktori yang diabaikan tidak berlaku (sama dengan git).
3. `func Scan(root string) ([]FileInfo, error)`:
   - `root` kosong atau bukan direktori -> galat Indonesia (`fmt.Errorf("root pindai tidak valid: %w", err)`).
   - Telusuri manual dengan `os.ReadDir` rekursif (bukan `filepath.WalkDir` agar `.gitignore` per-direktori bisa dimuat sebelum anak diproses). Fungsi bantu rekursif `walkDir(absDir, relDir string, rules []ignoreRule, out *[]FileInfo) error`.
   - Pada tiap direktori: bila ada `.gitignore` reguler (bukan symlink) di dalamnya, baca (batasi 256 KiB; galat baca diabaikan) dan tambahkan `parseGitignore(relDir, data)` ke salinan `rules` untuk subtree itu (jangan mengotori saudara: buat slice baru).
   - Entri diurutkan nama (`os.ReadDir` sudah terurut; tetap `sort` untuk kepastian). Lewati entri bila `fsx.IsIgnored(name)` atau `matchRules(rules, rel, isDir)`. Jangan mengabaikan `.gitignore` itu sendiri secara khusus (ia ikut terdaftar sebagai berkas teks jika tidak diabaikan).
   - Gunakan `entry.Type()`: symlink (`ModeSymlink`) dan non-reguler (socket, device, pipe) DILEWATI total (tidak diikuti, tidak dicatat). Direktori: rekursi. Berkas reguler: `Info()` untuk `Size`; `Large = Size > maxFileBytes`; `Binary`: untuk berkas tidak `Large` (dan juga untuk Large, cukup baca awalnya) buka berkas, baca maks 8192 byte, `Binary = bytes.IndexByte(buf, 0) >= 0`. Galat buka/baca satu berkas: lewati berkas itu (jangan gagalkan seluruh Scan). Berkas kosong: `Binary=false`.
   - Berkas biner dan besar TETAP dicatat di hasil (kelak masuk `Module.Files`), hanya tidak dibaca untuk simbol.
   - `Path` memakai `path.Join` dengan `/` (jangan `filepath` untuk nilai tersimpan); hasil akhir diurutkan leksikal menurut `Path` dan deterministik.
   - Folder tanpa `.git` didukung penuh; jangan bergantung pada `.git` (direktori `.git` hanya terlewati lewat `fsx.IsIgnored`).
   - Hormati `ctx`? Tidak ada ctx pada signature; jangan menambah parameter.

### 2. `modules.go`: konstanta, pengelompokan, simbol, tag

Baris pertama hapus komentar STUB. Isi:

1. Konstanta `maxModuleFiles = 200`, `maxModuleBytes = 512 << 10`, `maxSymbols = 60`, dan konstanta nama `kindGo = "go"`, `kindWeb = "web"`, `kindGeneric = "generic"` (tidak diekspor; hindari bentrok nama dengan berkas lain di paket: awali bantu dengan `grp`/`sym` bila perlu, periksa `go vet`/build).
2. Klasifikasi berkas (fungsi bantu):
   - `isGoFile(p)`: ekstensi `.go`.
   - `isWebFile(p)`: ekstensi `.svelte`, `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, `.cjs`.
   - `isTextFile(fi)`: `!fi.Binary` (berkas teks apa pun: konfigurasi/dokumen/dll). Berkas `Binary` tidak membentuk modul sendiri dan tidak mempengaruhi klasifikasi, tetapi bila ia berada di direktori yang menjadi unit modul, ia tetap dimasukkan ke `Files` modul itu (daftar berkas lengkap). Direktori yang HANYA berisi biner: berkas ke unit induk terdekat per aturan (c).
3. `func GroupModules(files []FileInfo) []Module`. Input boleh tidak terurut: salin dan urutkan menurut `Path`. Algoritma penentuan unit (setiap berkas masuk TEPAT satu modul):
   a. Hitung `dir(p) = path.Dir(p)` (`"."` untuk root).
   b. **Unit web khusus `frontend/src/modules/<x>`**: bila path berawalan `frontend/src/modules/<x>/...` (minimal `<x>/<file>`), unit = `frontend/src/modules/<x>` (termasuk semua subfolder dan SEMUA jenis berkas di dalamnya), Kind `web`, Dir `frontend/src/modules/<x>`. Prioritas tertinggi.
   c. **Unit Go**: direktori yang langsung berisi berkas `.go` -> unit = direktori itu (satu modul per package/direktori), Kind `go`. Berkas non-Go di direktori yang sama ikut ke modul Go itu. Berkas `.go` berupa `_test.go` tetap dicatat di `Files` tetapi tidak dipakai untuk simbol (lihat 5).
   d. **Unit web umum**: direktori yang langsung berisi berkas web (selain yang tertangkap (b)) -> unit = direktori itu, Kind `web`. Bila direktori berisi `.go` dan web sekaligus, Go menang (c).
   e. **Unit generik**: berkas yang tidak termasuk (b)-(d): cari leluhur terdekat yang merupakan unit (b)/(c)/(d); bila ada, berkas bergabung ke sana ("subfolder lain mengikuti direktori terdekat yang berisi berkas kode"). Bila tidak ada leluhur unit kode: unit = direktori tingkat pertama atau kedua. Aturan: berkas di root (`dir == "."`) -> unit `"."` (Dir `"."`, Kind `generic`); berkas di `a/...` tanpa leluhur unit kode -> unit `a` bila `dir` hanya satu segmen, selain itu unit `a/b` (dua segmen pertama dari path direktori). Kind `generic`.
   Implementasi: dua lintasan. Lintasan 1: kumpulkan himpunan direktori unit kode (`codeUnits map[string]string` dir -> kind) dari aturan (b)(c)(d). Lintasan 2: untuk tiap berkas tentukan unit: (b) bila cocok; selain itu telusuri dari `dir(p)` naik ke leluhur sampai menemukan dir di `codeUnits` (leluhur terdekat, termasuk dirinya); bila tidak ketemu pakai aturan (e) generik. Catatan: unit root `"."` hanya jadi unit kode bila root sendiri berisi `.go`/web; leluhur `"."` TIDAK dipakai untuk menyerap berkas dari subdirektori generik (agar `docs/` tetap modul `docs`); berkas yang tidak punya leluhur unit kode selain root pakai aturan (e).
   f. Kelompokkan ke `map[unitDir][]FileInfo`, lalu urutkan kunci unit leksikal.
4. **Penamaan unik stabil**: nama dasar = `Snake(strings.ReplaceAll(unitDir, "/", "_"))` (untuk `"."` pakai `"root"`; `frontend/src/modules/<x>` -> nama dasar `Snake(<x>)` sesuai tiket "satu modul `<x>`"; bila bentrok dengan modul lain, akhiran di bawah menyelesaikannya). Iterasi unit berurut leksikal menurut `Dir`; pertahankan `used map[string]bool`; bila nama sudah dipakai, coba `base_2`, `base_3`, ... sampai bebas (berdasar urutan path, sehingga stabil antar-jalan untuk input sama). Hasil akhir `Module` diurutkan menurut `Name`.
5. **Batas dan simbol** per modul (setelah `Files` terurut menurut `Path`):
   - `Sources`: semua `Files[i].Path` (urut leksikal, `/`), disalin ke slice baru.
   - Berkas kandidat simbol = berkas yang `!Binary && !Large`. Terapkan batas: iterasi berurut path; hanya `maxModuleFiles` berkas pertama dan total byte kumulatif `<= maxModuleBytes` (berkas yang membuat total melampaui batas, dan semua setelahnya, tidak dibaca) yang boleh dibaca untuk simbol. `Truncated` = jumlah berkas kandidat simbol (non-biner, non-large, bertipe kode yang relevan) yang dipotong oleh batas tersebut, plus berkas di luar batas `maxModuleFiles` sebagai bagian dari daftar; definisikan tegas: `Truncated = len(Files) - maxModuleFiles` bila `len(Files) > maxModuleFiles`, ditambah jumlah berkas kandidat di dalam `maxModuleFiles` pertama yang tidak terbaca karena batas byte; 0 bila tidak ada pemotongan. `Files` tetap memuat SEMUA berkas (tidak dipotong), begitu pula `Sources`.
   - Sumber isi simbol: `GroupModules` hanya menerima `[]FileInfo` tanpa `root`, jadi tambahkan satu field TAK DIEKSPOR ke `FileInfo` di `scan.go`: `abs string` (path absolut berkas, diisi `Scan`). Ini tidak mengubah signature maupun kontrak eksternal (konsumen lain memakai field bernama, field tak diekspor tidak terlihat). `GroupModules` membaca isi dari `fi.abs`; bila `abs == ""` (FileInfo dibuat manual) simbol dilewati tanpa galat. Jangan memakai variabel global untuk menyimpan root. Dokumentasikan field ini di komentar tipe.
   - Ekstraksi simbol (kumpulkan lalu urut kemunculan per berkas dalam urutan path berkas, dedup, potong ke `maxSymbols`):
     - **Go** (berkas `.go` bukan `_test.go`): `go/parser.ParseFile(fset, name, src, parser.SkipObjectResolution)`; untuk `*ast.GenDecl` dan `*ast.FuncDecl` top-level kumpulkan nama diekspor (`ast.IsExported`): `func Name`, `func (Recv) Name` ditulis `Recv.Name` (tipe penerima tanpa `*`), `type Name`, `const Name`, `var Name`. Format simbol: `func Name`, `type Name`, `const Name`, `var Name`, `method Recv.Name`. Bila parse gagal, jatuh ke regex baris `^(func|type|const|var)\s+(?:\([^)]*\)\s*)?([A-Z]\w*)`.
     - **Svelte/TS/JS**: regex per baris `^\s*export\s+(?:default\s+)?(?:async\s+)?(function|const|let|class|type|interface|enum)\s+([A-Za-z_$][\w$]*)` -> `function Name` dst.; untuk berkas `.svelte` tambahkan satu simbol nama komponen = `strings.TrimSuffix(base, ".svelte")` dengan format `component Name` (selalu, tanpa perlu `export`), dan jalankan juga regex export pada isinya.
     - **Generik**: tidak ada simbol (kosong).
     Modul Kind `go` mengekstrak hanya dari `.go`; Kind `web` hanya dari berkas web/`.svelte`; Kind `generic` kosong. Modul `web` dari unit (b) boleh berisi `.go`/lainnya; abaikan untuk simbol. Batas `maxSymbols` diterapkan pada daftar akhir; hentikan lebih awal setelah tercapai.
   - Baca isi berkas dengan batas aman (`io.LimitReader` ke `maxFileBytes`), galat baca: lewati berkas, jangan gagal.
6. **`Tag`** satu token camelCase: untuk Kind `go`: nama package dari berkas `.go` non-test pertama (urut path) yang berhasil di-parse (`parser.PackageClauseOnly`); jika tak ada, basename `Dir`. Untuk lainnya: basename `Dir` (untuk `"."` pakai `"root"`; untuk unit `frontend/src/modules/<x>` pakai `<x>`). Tag = `CamelTag(sumber)`; bila hasil kosong atau mengandung karakter selain `[A-Za-z0-9]` (token tunggal tanpa pemisah), ganti dengan `CamelTag` hasil paksa: buang semua karakter non-alfanumerik; bila masih kosong gunakan `"root"`. Jamin tag selalu cocok `^[a-z][A-Za-z0-9]*$` (jika diawali angka, awali `m`).
7. Seluruh `Module.Files`, `Sources`, `Symbols` bertipe slice non-nil (kosong bila tak ada) agar JSON/render stabil. Modul tidak pernah kosong: unit tanpa berkas tidak dibuat. Input kosong -> kembalikan slice kosong non-nil.

### 3. `hash.go`: HashModule

Baris pertama hapus komentar STUB. Isi:

```go
func HashModule(root string, m Module) (string, error)
```
- Salin `m.Files`, urutkan menurut `Path`. Buat `sha256.New()`.
- Untuk tiap berkas (termasuk biner dan besar; hash menutup seluruh isi modul): `p := filepath.Join(root, filepath.FromSlash(f.Path))`; buka dengan `os.Open`; `Lstat` terlebih dahulu dan tolak symlink (galat). Berkas hilang (`errors.Is(err, os.ErrNotExist)`) -> perlakukan sebagai kosong dengan penanda: tulis path dan panjang `-1` tanpa isi, sehingga edit hapus mengubah hash dan Build tidak gagal total; galat lain dikembalikan dengan konteks Indonesia (`fmt.Errorf("hash %s: %w", f.Path, err)`).
- Tulis ke hash secara berurutan: `path` (byte), pemisah `0x00`, panjang isi sebagai desimal ASCII (`strconv.FormatInt`) , pemisah `0x00`, lalu isi (`io.Copy` dari berkas; panjang diambil dari jumlah byte yang sungguh disalin: baca isi ke hash via `io.Copy(h, f)` setelah mencatat `Stat().Size()`; bila jumlah byte yang disalin berbeda dari ukuran stat, hitung ulang dengan membaca ke buffer dulu supaya panjang = isi sebenarnya), pemisah `0x00`. Dilarang memakai mtime, urutan map, atau data non-deterministik. Bukan HEAD git; tidak butuh `.git`.
- Kembalikan `hex.EncodeToString(h.Sum(nil))` (64 karakter).
- Modul tanpa berkas: hash dari input kosong (tetap valid, deterministik).

### 4. Verifikasi (guard)

- `go build ./...`. Bila gagal karena berkas fitur lain (bukan milikmu), jangan memperbaikinya; laporkan.
- `go vet ./internal/grimoire/...` bila cepat.
- Periksa manual: tidak ada definisi ganda konstanta (`maxFileBytes` hanya di `scan.go`; `maxModuleFiles`/`maxModuleBytes`/`maxSymbols` hanya di `modules.go`), tidak ada impor ke paket di luar stdlib + `internal/fsx`, tidak ada penulisan berkas, tidak ada `STUB(foundation)` tersisa di ketiga berkas, `git diff --stat -- go.mod go.sum frontend` kosong.

## Perubahan

- Ditimpa penuh (stub Foundation menjadi implementasi nyata, signature publik tidak berubah):
  - `internal/grimoire/scan.go` (konstanta `maxFileBytes`, matcher `.gitignore`, `Scan`; `FileInfo` bertambah satu field tak diekspor `abs`)
  - `internal/grimoire/modules.go` (konstanta batas, `GroupModules`, ekstraksi simbol, tag, penamaan unik)
  - `internal/grimoire/hash.go` (`HashModule`)
- Tidak ada berkas lain dibuat atau diubah.

## Wiring

Tidak ada wiring sendiri. Fitur ini hanya mengekspos fungsi paket untuk dipanggil `Build` (`pages-index`, `internal/grimoire/build.go`) dalam urutan `Scan(projectPath)` -> `GroupModules(files)` -> `HashModule(projectPath, m)` per modul. Tidak ada registrasi IPC/CLI/UI. JANGAN mengedit `build.go` atau berkas wiring apa pun. Konsumen memakai `Module.Name` sebagai nama buku dan kunci `State.Modules`; `Module.Sources`/`Files`/`Symbols`/`Tag`/`Truncated` untuk render halaman dan index. Catatan untuk langkah wiring akhir: pastikan `grep -rn "STUB(foundation)" internal/grimoire` tidak lagi menemukan `scan.go`, `modules.go`, `hash.go`.

## Selesai bila

- `Scan`: menghormati `fsx.IsIgnored` dan `.gitignore` berlapis (nama, `dir/`, `*.ext`, `/anchored`, `**`, negasi `!`); pola tak dikenali diabaikan; tidak mengikuti symlink dan melewati berkas non-reguler; menandai `Binary` (NUL di 8 KiB awal) dan `Large` (> 1 MiB) tanpa membaca isi besar untuk simbol; folder tanpa `.git` berjalan penuh; hasil terurut menurut `Path` dan deterministik.
- `GroupModules`: menghasilkan modul untuk package Go, direktori Svelte/TS (dengan `frontend/src/modules/<x>` satu modul `<x>`), dan direktori generik; batas `maxModuleFiles`/`maxModuleBytes` diterapkan ke bahan simbol dengan `Truncated` terisi dan daftar berkas tetap lengkap; nama unik, stabil, snake_case (akhiran `_2`, `_3` untuk tabrakan); `Symbols` maks 60; `Tag` satu token camelCase; `Sources` relatif repo, urut leksikal, `/`; `Files`/`Sources`/`Symbols` non-nil.
- `HashModule`: SHA-256 hex 64 karakter atas path + panjang + isi working tree urut path, stabil antar-jalan, berubah bila isi berkas berubah (termasuk yang belum di-commit), tanpa mtime, bekerja tanpa `.git`.
- Tidak ada pemanggilan LLM, proses eksternal, atau penulisan berkas di tiga berkas ini; hanya stdlib + `internal/fsx`; tidak ada dependency baru.
- Guard: `go build ./...` sukses (path fitur ini kompilasi tanpa galat); tanpa unit test; signature `FileInfo`/`Module`/`Scan`/`GroupModules`/`HashModule` dan konstanta identik dengan overview 2.3.
