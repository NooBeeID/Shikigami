# Plan: Method IPC, wiring daemon, dan shikigamictl grimoire

## Metadata

- Tiket: `grimoire-ticket-01-grimoire-build` (`docs/tickets/grimoire-ticket-01-grimoire-build.md`)
- Fitur: `ipc-ctl`
- Spec: `docs/superpowers/specs/2026-11-22-grimoire-design.md`
- Sumber kebenaran bersama: `docs/plans/grimoire-ticket-01-grimoire-build/overview.md` (bagian 2.3 Deps/Register, 2.4 IPC, 7 aturan paralel)
- ownedPaths: `internal/grimoire/ipc.go`, `cmd/shikigamid/wire_grimoire.go`, `cmd/shikigamictl/cmd_grimoire.go`
- Depends on foundation: ya. `00-foundation.md` sudah membuat `types.go` final dan stub berkompilasi (`Store`, `Build`, `Deps`, `Register`, dst.) sehingga plan ini bisa dikerjakan paralel dengan fitur lain tanpa menunggu.

## Tujuan

Mengekspos Grimoire lewat IPC dan CLI: method `grimoire.build/status/list/read`, build single-flight per project dengan event `grimoire.changed`, modul daemon `grimoire` yang menyelesaikan `projectId -> Project.Path` lewat `workspace.list`, dan subcommand `shikigamictl grimoire build|status`. Tanpa unit test, tanpa dependency baru; guard hanya `go build ./...`.

## Path yang dimiliki

HANYA tiga berkas ini yang boleh dibuat atau ditimpa:

- `internal/grimoire/ipc.go` (menimpa stub Foundation; baris `// STUB(foundation)` dihapus)
- `cmd/shikigamid/wire_grimoire.go` (baru)
- `cmd/shikigamictl/cmd_grimoire.go` (baru)

Jangan menyentuh `types.go`, `store.go`, `build.go`, dan berkas `internal/grimoire` lain (milik fitur lain), `cmd/shikigamid/main.go`, `modules.go`, `wire_workspace.go`, `cmd/shikigamictl/main.go`, `registry.go`, `internal/ipc`, `internal/workspace`, frontend, `go.mod`/`go.sum`. Bila build gagal karena berkas fitur lain, jangan diperbaiki; laporkan.

## Kontrak

### Disediakan (signature final, overview 2.3)

```go
// internal/grimoire/ipc.go
type Deps struct {
    Store       *Store
    ResolvePath func(ctx context.Context, projectID string) (string, error)
    Logger      *slog.Logger
}
func Register(srv *ipc.Server, d Deps)
```

Method IPC sesuai overview 2.4 (nama dan bentuk hasil persis; `panel-ui` dan `readonly-pane` bergantung padanya):

- `grimoire.build {projectId, full?}` -> `{books, pages, indexBytes}` (JSON dari `Summary`)
- `grimoire.status {projectId}` -> `{exists, books, pages, staleCount, building, indexBytes, estTokens, lawsBytes, llmEnabled, lastBuildCost, dir}`; `dir` WAJIB selalu terisi (absolut, walau grimoire belum ada)
- `grimoire.list {projectId}` -> `[{name, tags, chapters:[{name, path, stale}]}]`, `[]` (bukan null) bila belum ada
- `grimoire.read {projectId, path}` -> `{content, size}`
- Event `grimoire.changed` payload `{projectId}`.

### Dipakai (dari Foundation / fitur lain; JANGAN diubah)

- `grimoire.Store` dan metode: `Dir`, `ValidateProject`, `Exists`, `IndexPath`, `LawsPath`, `ReadState`, `ReadFile`, `BooksDir` (overview 2.3). Galat "tak ada" dapat dideteksi dengan `errors.Is(err, os.ErrNotExist)`.
- `grimoire.Build(store *Store, projectID, projectPath string) (Summary, error)`; `Summary{Books, Pages, IndexBytes}` dengan tag JSON `books/pages/indexBytes`.
- Tipe `Book`, `Chapter`, `State`; sentinel `ErrNotBuilt`, `ErrInvalidPath`, `ErrInvalidProject` (`types.go`).
- `ipc.NewError(code, msg)`, `ipc.ErrInvalid/ErrNotFound/ErrInternal`, `srv.Register(method, func(ctx, json.RawMessage) (any, error))`, `srv.Publish(topic, payload)`.
- Daemon: `RegisterModule`, `Deps{DataDir, SocketPath, Logger}` (`cmd/shikigamid/modules.go`), `paths.DataDir()`.
- Klien IPC: `ipc.Dial(socketPath) (*Client, error)`, `(*Client).Call(method, params, out) error`, `(*Client).Close()`.
- CLI: `RegisterCommand(Command{Name, Summary, Run})`, `printJSON`, `callAndPrint` (`registry.go`).
- Respons `workspace.list`: array `Workspace` dengan JSON `id`, `name`, `projects:[{id, path, name}]` (sudah diverifikasi pada `internal/workspace/types.go`). Catatan penting: handler `workspace.list` memanggil `decode(params, ...)` yang menolak params kosong, jadi klien harus mengirim `map[string]any{}` (bukan `nil`).

Tidak ada tipe/fungsi baru yang diekspor selain `Deps` dan `Register`. Jangan mengubah signature apa pun.

## Langkah

### 1. `internal/grimoire/ipc.go`

Hapus baris pertama stub; tulis ulang seluruh berkas. Package `grimoire`. Import: `context`, `encoding/json`, `errors`, `io/fs` (opsional), `log/slog`, `os`, `path/filepath`, `sort`, `strings`, `sync`, `github.com/NooBeeID/shikigami/internal/ipc`. Tidak memakai paket di luar stdlib dan repo.

Berkas ini memiliki `Deps` dan `Register` (ulang dengan signature foundation). Semua helper bersifat privat (huruf kecil) agar tidak bentrok dengan berkas lain; awali nama dengan `ipc`/`rpc` bila ragu bentrok (mis. `rpcDecode`, `rpcMapErr`).

**1a. Tipe privat.**

```go
type projectParams struct {
    ProjectID string `json:"projectId"`
    Full      bool   `json:"full"` // diterima dan diabaikan
}
type readParams struct {
    ProjectID string `json:"projectId"`
    Path      string `json:"path"`
}
type inflight struct {
    done chan struct{}
    sum  Summary
    err  error
}
type service struct {
    d        Deps
    srv      *ipc.Server
    mu       sync.Mutex
    inflight map[string]*inflight
}
type statusResult struct {
    Exists        bool   `json:"exists"`
    Books         int    `json:"books"`
    Pages         int    `json:"pages"`
    StaleCount    int    `json:"staleCount"`
    Building      bool   `json:"building"`
    IndexBytes    int    `json:"indexBytes"`
    EstTokens     int    `json:"estTokens"`
    LawsBytes     int    `json:"lawsBytes"`
    LLMEnabled    bool   `json:"llmEnabled"`
    LastBuildCost int    `json:"lastBuildCost"`
    Dir           string `json:"dir"`
}
type readResult struct {
    Content string `json:"content"`
    Size    int    `json:"size"`
}
```

Pastikan `LastBuildCost` bertipe angka (nilai 0) agar JSON `lastBuildCost: 0`.

**1b. Decode dan pemetaan galat.**

- `rpcDecode(params json.RawMessage, v any) error`: pola persis `fsx/ipc.go`: params kosong -> `ipc.NewError(ipc.ErrInvalid, "invalid params: kosong")`; gagal unmarshal -> `ipc.NewError(ipc.ErrInvalid, "invalid params: "+err.Error())`.
- `rpcMapErr(err error) error`: `nil` -> `nil`; bila sudah `*ipc.Error` (`errors.As`) kembalikan apa adanya; `ErrInvalidProject`/`ErrInvalidPath` (`errors.Is`) -> `ErrInvalid`; `ErrNotBuilt` atau `os.ErrNotExist` -> `ErrNotFound`; sisanya `ErrInternal`. Pesan galat memakai `err.Error()`.
- Catatan: `ErrNotBuilt` dipetakan `ErrNotFound` hanya untuk `read` dan `list`-path; untuk `status` dan `list` galat tersebut bukan galat (lihat langkah 1d/1e), jadi tangani sebelum memanggil `rpcMapErr`.

**1c. Resolusi project.** Metode `(s *service) resolve(ctx, projectID) (string, error)`:
1. `projectID == ""` -> `ipc.NewError(ipc.ErrInvalid, "projectId wajib diisi")`.
2. `s.d.Store.ValidateProject(projectID)` -> galat dipetakan `ErrInvalid`.
3. `s.d.ResolvePath == nil` -> `ErrInternal` ("resolver project belum terpasang").
4. Panggil `ResolvePath(ctx, projectID)`; galat dipetakan lewat `rpcMapErr` (resolver mengembalikan `*ipc.Error` `ErrNotFound` untuk project tak dikenal, diteruskan apa adanya).
`resolve` dipakai oleh `build` saja (path project hanya perlu untuk build). `status`, `list`, `read` hanya butuh `projectId` valid dan tidak memanggil `ResolvePath`; ini membuat panel tetap bisa membaca grimoire walau daemon workspace lambat, namun tetap validasi `projectId` (kosong -> `ErrInvalid`, `ValidateProject`).
Helper kecil `(s *service) checkID(projectID string) error` untuk langkah validasi 1+2 (dipakai `resolve` dan tiga method lain).

**1d. Build single-flight per project.**

`(s *service) build(ctx context.Context, projectID string) (Summary, error)`:
1. `path, err := s.resolve(ctx, projectID)`; galat -> kembalikan (belum ada build yang jalan, tidak ada event).
2. Kunci `s.mu`. Bila `fl, ok := s.inflight[projectID]; ok`: buka kunci, tunggu `select { case <-fl.done: return fl.sum, fl.err; case <-ctx.Done(): return Summary{}, ctx.Err() }` (waiter yang kontekstnya batal tidak membatalkan build).
3. Selain itu buat `fl := &inflight{done: make(chan struct{})}`, simpan di `s.inflight[projectID]`, buka kunci, lalu jalankan build dengan konteks tersendiri: goroutine `go s.run(projectID, path, fl)`; pemanggil menunggu seperti pada langkah 2 (sehingga klien yang terputus tidak membatalkan build).
4. `run`: `s.publish(projectID)` (building true), `sum, err := Build(s.d.Store, projectID, path)` dibungkus `recover` agar panik di Build menjadi galat (isi `fl.err`, log), isi `fl.sum/fl.err`, kunci `s.mu`, `delete(s.inflight, projectID)`, buka kunci, `close(fl.done)`, lalu `s.publish(projectID)` (building false; untuk sukses maupun gagal), log hasil bila `Logger != nil`. Hapus entri inflight SEBELUM publish akhir agar `status` yang dibaca UI setelah event melihat `building=false`.
5. `building(projectID) bool`: kunci `s.mu`, cek keberadaan kunci di `s.inflight`.
6. `publish(projectID)`: `s.srv.Publish("grimoire.changed", map[string]any{"projectId": projectID})`.
Project berbeda tidak saling blokir (kunci hanya melindungi map, bukan Build).
Galat dari `Build` dipetakan lewat `rpcMapErr` di handler.

**1e. Handler.**

- `grimoire.build`: decode `projectParams`; `sum, err := s.build(ctx, p.ProjectID)`; kembalikan `sum` (struct `Summary` sudah ber-tag JSON) atau `rpcMapErr(err)`.
- `grimoire.status`: decode; `checkID`; hitung:
  - `dir := s.d.Store.Dir(id)` (absolutkan dengan `filepath.Abs` bila belum absolut, abaikan galat Abs; `dir` selalu non-kosong).
  - `exists := s.d.Store.Exists(id)`.
  - `building := s.building(id)`.
  - Bila `exists`: `indexBytes` dari `os.Stat(s.d.Store.IndexPath(id)).Size()` (galat stat -> 0); `books` = jumlah entri direktori (hanya direktori) pada `s.d.Store.BooksDir(id)`; `pages` = jumlah berkas `*.md` di setiap subfolder buku (baca dengan `os.ReadDir`; galat dibaca sebagai 0). Tidak perlu `state.json` agar tahan galat.
  - `staleCount 0`, `estTokens = indexBytes/4`, `lawsBytes 0`, `llmEnabled false`, `lastBuildCost 0`.
  - Kembalikan `statusResult`. Status tidak pernah gagal karena grimoire belum ada (kembalikan `exists:false` dengan nilai nol dan `dir`).
- `grimoire.list`: decode; `checkID`; kumpulkan `[]Book` lewat `s.listBooks(id)`; selalu kembalikan slice non-nil (`[]Book{}`).
  - `listBooks`: bila `!Exists` -> `[]Book{}`. Telusuri `BooksDir(id)`: tiap subfolder = buku (urut nama), tiap `*.md` di dalamnya = bab (urut nama, `Name` = nama berkas tanpa `.md`, `Path` = `"books/" + buku + "/" + bab + ".md"` dengan `/`, `Sources: []string{}`, `Stale: false`). `Tags`: ambil dari baris index buku (parse baris `index.md` yang diawali `<buku> (`: ambil isi dalam `[` `]` terakhir, pisah koma, trim; bila tak ada atau gagal, `[]string{}`). Baca `index.md` lewat `Store.ReadFile(id, "index.md")`; galat -> tanpa tag. Pastikan `Tags` dan `Chapters` tidak pernah `nil` (JSON `[]`). Modul tanpa bab tidak ikut.
  - Bila `BooksDir` tidak ada walau `Exists` true -> `[]Book{}` (bukan galat).
  - Alasan penjelajahan direktori: sederhana, tidak bergantung pada `state.json`, dan sesuai tiket ("cukup dari state/penjelajahan direktori").
- `grimoire.read`: decode `readParams`; `checkID`; `Path == ""` -> `ErrInvalid` ("path wajib diisi"); `data, err := s.d.Store.ReadFile(id, p.Path)`; galat dipetakan `rpcMapErr` (`ErrInvalidPath` -> `ErrInvalid`; `os.ErrNotExist` atau `ErrNotBuilt` -> `ErrNotFound`); hasil `readResult{Content: string(data), Size: len(data)}`. Validasi keluar-direktori sepenuhnya diserahkan ke `Store.ReadFile` (lewat `fsx.ResolveUnder`); jangan menduplikasi logikanya.

**1f. `Register`.**

```go
func Register(srv *ipc.Server, d Deps) {
    s := &service{d: d, srv: srv, inflight: map[string]*inflight{}}
    srv.Register("grimoire.build", ...)
    srv.Register("grimoire.status", ...)
    srv.Register("grimoire.list", ...)
    srv.Register("grimoire.read", ...)
}
```

Bila `d.Store == nil` kembalikan galat `ErrInternal` per panggilan (jangan panik; cek di `checkID`).

### 2. `cmd/shikigamid/wire_grimoire.go`

Package `main`. Import: `context`, `errors`, `fmt`, `sync` (bila perlu), `github.com/NooBeeID/shikigami/internal/grimoire`, `github.com/NooBeeID/shikigami/internal/ipc`, `github.com/NooBeeID/shikigami/internal/paths`.

```go
func init() {
    RegisterModule("grimoire", func(srv *ipc.Server, d *Deps) {
        dataDir := d.DataDir
        if dataDir == "" {
            dd, err := paths.DataDir()
            if err != nil { /* log Warn, return */ }
            dataDir = dd
        }
        grimoire.Register(srv, grimoire.Deps{
            Store:       grimoire.NewStore(dataDir),
            ResolvePath: newGrimoireResolver(d.SocketPath),
            Logger:      d.Logger,
        })
    })
}
```

(Periksa tanda tangan `paths.DataDir()` pada `internal/paths` dan ikuti pola `wire_workspace.go` untuk fallback dan penanganan galat.)

`newGrimoireResolver(socketPath string) func(ctx context.Context, projectID string) (string, error)`:
- Tidak mendial saat wire. Di dalam closure, saat dipanggil: `ipc.Dial(socketPath)` (malas, dibuat per panggilan), `defer c.Close()`. Dial per panggilan dipilih karena sederhana dan panggilan jarang (hanya per build); tidak ada klien bersama yang harus dijaga hidup. Layanan socket baru siap setelah semua modul ter-wire dan handler jalan jauh sesudahnya, jadi tidak perlu `d.OnReady`.
- `socketPath == ""` -> kembalikan `ipc.NewError(ipc.ErrInternal, "socket daemon tidak diketahui")`.
- Dial gagal -> `ipc.NewError(ipc.ErrInternal, "gagal menghubungi daemon: ...")`.
- Struct decode lokal (jangan mengimpor `internal/workspace` agar tidak ada ketergantungan baru antar paket):
  ```go
  var list []struct {
      Projects []struct {
          ID   string `json:"id"`
          Path string `json:"path"`
      } `json:"projects"`
  }
  ```
- `c.Call("workspace.list", map[string]any{}, &list)` (params `{}`, WAJIB bukan `nil`, karena `decode` workspace menolak params kosong). Galat `Call` -> `ipc.NewError(ipc.ErrInternal, err.Error())` bila bukan `*ipc.Error`, atau teruskan apa adanya.
- Cari `projectID` di semua workspace; ketemu -> kembalikan `Path` (tolak path kosong dengan `ErrNotFound`); tidak ketemu -> `ipc.NewError(ipc.ErrNotFound, "project tidak dikenal: "+projectID)`.
- Hormati `ctx`: sebelum Dial cek `ctx.Err()`; jalankan `Call` dalam goroutine bila ingin dapat dibatalkan, atau cukup cek `ctx.Err()` sebelum dan sesudah (cukup untuk tiket ini; jangan menambah kerumitan).
- Handler tidak boleh panik bila workspace tidak tersedia; semua jalur mengembalikan galat per panggilan.

Tidak ada `d.Provide` (nama layanan tidak diperlukan) dan tidak mengubah `wire_workspace.go`. Nama modul `grimoire` harus unik.

### 3. `cmd/shikigamictl/cmd_grimoire.go`

Package `main`. Import: `context`, `errors`, `flag`, `fmt`, `io`, `sort`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`.

Konstanta usage (bahasa Indonesia):

```go
const grimoireUsage = `penggunaan:
  shikigamictl grimoire build  [--project <id>]   bangun ulang grimoire project
  shikigamictl grimoire status [--project <id>]   tampilkan status grimoire project

Tanpa --project, dipakai satu-satunya project bila hanya ada satu; bila lebih dari
satu, daftar id project ditampilkan.`
```

`init()`: `RegisterCommand(Command{Name: "grimoire", Summary: "Bangun dan lihat status grimoire project", Run: runGrimoire})`.

`runGrimoire(ctx, c, args)`:
1. `len(args) == 0` -> `errors.New(grimoireUsage)`. `sub := args[0]`; selain `build`/`status` -> `errors.New(grimoireUsage)`.
2. `flag.NewFlagSet("grimoire "+sub, flag.ContinueOnError)`, `SetOutput(io.Discard)`, flag `--project` string; `Parse(args[1:])` galat -> `errors.New(grimoireUsage)`. Argumen sisa (`fs.NArg() > 0`) -> usage.
3. `id := *project`; bila kosong `id, err = grimoireDefaultProject(c)`; galat dikembalikan.
4. Panggil: `build` -> `callAndPrint(c, "grimoire.build", map[string]any{"projectId": id})`; `status` -> `callAndPrint(c, "grimoire.status", ...)`. Bungkus galat dengan `grimoireError`.

`grimoireDefaultProject(c *ipc.Client) (string, error)`: `c.Call("workspace.list", map[string]any{}, &list)` dengan struct lokal `[]struct{ Projects []struct{ ID, Name, Path string } }` (tag JSON `id`, `name`, `path`, `projects`). Kumpulkan semua project (urut berdasarkan ID untuk keluaran stabil). 0 project -> `errors.New("belum ada project; tambahkan project dulu atau gunakan --project <id>")`. 1 project -> id-nya. Lebih -> `fmt.Errorf("ada %d project, pilih satu dengan --project <id>:\n%s", n, daftar)` dengan satu baris per project `  <id>  <nama>  <path>`.

`grimoireError(err error) error` (gaya `fsError` di `cmd_fs.go`): `errors.As(err, &ie)` untuk `*ipc.Error`; `ErrNotFound` -> `fmt.Errorf("tidak ditemukan (%s)", ie.Message)`; `ErrInvalid` -> `fmt.Errorf("parameter tidak valid (%s)", ie.Message)`; lainnya kembalikan `err` apa adanya. Nama fungsi dan konstanta HARUS unik di package `main` (awali `grimoire`); jangan mendefinisikan ulang `fsError`, `printJSON`, `callAndPrint`.

### 4. Verifikasi (guard)

1. `gofmt -l internal/grimoire/ipc.go cmd/shikigamid/wire_grimoire.go cmd/shikigamictl/cmd_grimoire.go` tidak mencetak apa pun.
2. `go build ./...` lulus. Jika gagal karena berkas di luar ownedPaths, jangan diperbaiki; laporkan.
3. `go vet ./internal/grimoire/... ./cmd/...` bila cepat.
4. `git diff --stat -- go.mod go.sum` kosong; `git status --short` hanya menampilkan tiga berkas milik plan ini (ditambah berkas fitur lain oleh agent lain, jangan disentuh).
5. Uji manual opsional bila `pages-index` sudah selesai: jalankan daemon, `shikigamictl grimoire status --project <id>` mencetak JSON dengan `dir` terisi; `build` lalu `status` memberi `exists: true`.

## Perubahan

- Ditimpa: `internal/grimoire/ipc.go` (stub -> implementasi; signature `Deps`/`Register` tidak berubah)
- Baru: `cmd/shikigamid/wire_grimoire.go`
- Baru: `cmd/shikigamictl/cmd_grimoire.go`

Tidak ada perubahan lain.

## Wiring

Fitur ini hanya mengekspos; penyambungan akhir (checklist overview bagian 5) dilakukan agen wiring:

- Daemon: modul `grimoire` terdaftar sendiri lewat `init()` + `RegisterModule`; tidak ada berkas bersama yang perlu diedit. Wiring tinggal memverifikasi bahwa `grimoire.Register(srv, deps)` terpanggil dan nama modul unik.
- CLI: `RegisterCommand` dari `init()`; `registry.go`/`main.go` tidak diubah.
- Kontrak untuk fitur lain: nama method `grimoire.build/status/list/read` dan event `grimoire.changed {projectId}` dipakai `panel-ui` (`client.ts`/`store.ts`); field `dir` pada `grimoire.status` dipakai `openGrimoirePage` (`readonly-pane`).
- Jangan menyambungkan apa pun sendiri di luar tiga berkas ini.

## Selesai bila

- [ ] `grimoire.build/status/list/read` terdaftar dengan nama dan bentuk hasil persis overview 2.4; `list` mengembalikan `[]` (bukan null) bila belum ada grimoire; `Tags`/`Chapters` tidak pernah null.
- [ ] `status` selalu mengisi `dir` (absolut) walau grimoire belum ada; `staleCount 0`, `estTokens = indexBytes/4`, `lawsBytes 0`, `llmEnabled false`, `lastBuildCost 0`.
- [ ] Dua `grimoire.build` bersamaan untuk project sama hanya menjalankan satu `Build` dan menerima hasil yang sama; project berbeda independen; build tidak terbatalkan bila klien terputus; `building` pada `status` true selama berjalan.
- [ ] `grimoire.changed {projectId}` terbit saat build mulai, selesai sukses, dan selesai gagal (entri inflight dihapus sebelum publish akhir).
- [ ] Pemetaan galat sesuai overview 2.4 (`projectId` kosong/`ErrInvalidProject`/`ErrInvalidPath` -> `ErrInvalid`; project tak dikenal/berkas tak ada/`ErrNotBuilt` pada read -> `ErrNotFound`; sisanya `ErrInternal`); params salah bentuk -> `ErrInvalid` dengan `invalid params: ...`; tidak ada panik bila resolver/workspace tidak tersedia.
- [ ] `ResolvePath` memakai `workspace.list` lewat klien IPC yang didial malas ke `d.SocketPath` dengan params `{}` dan ditutup setelah pakai.
- [ ] `shikigamictl grimoire build|status [--project <id>]` mencetak JSON; tanpa `--project` memakai satu-satunya project, atau galat berisi daftar id; usage dan pesan galat dalam bahasa Indonesia.
- [ ] Hanya tiga ownedPaths yang berubah; tidak ada dependency baru; `go build ./...` lulus.
