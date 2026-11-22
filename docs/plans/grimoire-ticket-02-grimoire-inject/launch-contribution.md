# Plan: Pasang LaunchContribution untuk claude dan codex

## Metadata

- Tiket: `grimoire-ticket-02-grimoire-inject` (`docs/tickets/grimoire-ticket-02-grimoire-inject.md`)
- Fitur: `launch-contribution`
- ownedPaths: `cmd/shikigamid/wire_grimoire_inject.go`
- Depends on foundation: ya. Stub berkompilasi dari `00-foundation.md` (`ComposeInjection`, `WriteSnapshot`, `PruneSnapshots`, `TOMLString` di paket `internal/grimoire`, serta `InjectDeps` dan `RegisterInject`) harus sudah ada. Fitur paralel `compose-snapshot` dan `laws-measure-ipc` menimpa isinya tanpa mengubah signature. Hanya berkompilasi terhadap stub, tidak menunggu implementasi nyata.

Sumber kebenaran bersama adalah `overview.md` di folder plan ini (bagian 2.1, 2.2 butir 3 dan 6, 2.3, 2.5, 7). Plan ini tidak mengulangnya, hanya urutan kerja untuk satu berkas.

## Tujuan

Membuat satu berkas wiring daemon yang:

1. Mendaftarkan modul `grimoire-inject`.
2. Di `d.OnReady`, membuat `grimoire.Store`, memanggil `grimoire.RegisterInject` (method IPC `grimoire.laws.get/set`), menjalankan prune snapshot awal, dan memasang tepat satu `provider.RegisterLaunchContribution`.
3. Contribution menyuntikkan teks grimoire ke sesi baru `claude` (`--append-system-prompt-file <snapshot>`) dan `codex` (`-c developer_instructions=<string TOML>`) dengan teks yang sama. Sesi `shell`, id provider lain, dan project tanpa grimoire tidak berubah.
4. Kegagalan apa pun (baca grimoire, tulis snapshot, lookup project, prune) hanya dicatat di log dan tidak pernah menggagalkan peluncuran sesi.

## Path yang dimiliki

HANYA berkas ini yang boleh dibuat/disentuh:

- `cmd/shikigamid/wire_grimoire_inject.go`

Dilarang menyentuh: `cmd/shikigamid/wire_grimoire.go`, `modules.go`, `main.go`, `wire_workspace.go`, `wire_mention.go`, apa pun di `internal/` (termasuk `internal/session`, `internal/provider`, `internal/ipc`, `internal/grimoire`, `internal/mention`), frontend, `cmd/shikigamictl`, `go.mod`, `go.sum`, dan `overview.md`. Jangan membuat berkas lain.

## Kontrak

Dipakai (dari repo, baca dulu kode nyata sebelum menulis):

- `cmd/shikigamid/modules.go`: `RegisterModule(name string, fn func(srv *ipc.Server, d *Deps))`; `Deps` punya `DataDir`, `SocketPath`, `Logger *slog.Logger`, `OnReady(fn func())`, `Lookup`, `Provide`.
- `provider.RegisterLaunchContribution(fn func(*provider.LaunchContext))`; `LaunchContext{Session session.SessionSpec, Provider provider.Provider, Env map[string]string, Args []string, MCPServers map[string]any}` (`internal/provider/launch.go`). `Session` berisi `WorkspaceID`, `ProjectID`, `Cwd`, `ProviderID`. Konstanta `provider.IDClaude`, `provider.IDCodex`. Contribution dipanggil sinkron dari `Launcher.Command`, SEBELUM `Provider.Command`; provider kemudian menambah flag izin sendiri lewat `withFlag`, jadi jangan menyentuh flag izin.
- Dari `internal/grimoire` (signature final overview 2.3; kode tiket 01 dan Foundation sudah ada saat dieksekusi, bila nama nyata beda ikuti kode nyata dan catat):
  - `grimoire.NewStore(dataDir string) *Store`
  - `grimoire.ComposeInjection(projectID string) (text string, ok bool, err error)`
  - `grimoire.WriteSnapshot(text string) (path string, err error)`
  - `grimoire.PruneSnapshots(inUse func(path string) bool) error`
  - `grimoire.TOMLString(s string) string`
  - `grimoire.InjectDeps{Store *Store; Known func(ctx context.Context, projectID string) (bool, error); Logger *slog.Logger}` dan `grimoire.RegisterInject(srv *ipc.Server, d InjectDeps)`
- Pola referensi (baca, jangan ubah): `workspaceDirectory` di `cmd/shikigamid/wire_mention.go` (klien IPC malas `ipc.Dial(socket)`, cache 5 detik, `Call("workspace.list", ...)`) dan `contribOnce`/`registerLaunchContribution` di `internal/mention/install.go`.
- Bentuk respons `workspace.list`: array `Workspace{ID, Name, Projects []Project{ID, Path, Name}}` (tag JSON `id`, `name`, `path`, `projects` terverifikasi di `internal/workspace/types.go`). Pola mention juga menoleransi bentuk objek `{workspaces:[...]}`; ikuti toleransi itu.

Disediakan: tidak ada simbol Go yang diekspor. Efek samping: modul `grimoire-inject` terdaftar, dua method IPC `grimoire.laws.get/set` (lewat `RegisterInject`), dan satu launch contribution.

## Langkah

Semua di satu berkas `cmd/shikigamid/wire_grimoire_inject.go`, `package main`. Nama identifier tak-diekspor diberi awalan `grimoireInject` / `projectDirectory` agar tidak bentrok dengan berkas lain di `package main` (nama `workspaceDirectory`, `exitHub`, `wireMention` sudah dipakai di `wire_mention.go`; jangan definisikan ulang). Komentar dan pesan log bahasa Indonesia.

1. **Impor.** `context`, `encoding/json`, `errors`, `log/slog`, `path/filepath`, `strings`, `sync`, `time`, `github.com/NooBeeID/shikigami/internal/grimoire`, `.../internal/ipc`, `.../internal/provider`. Tambah `.../internal/session` hanya bila perlu. Tanpa dependency baru.

2. **Modul.**
   ```go
   func init() {
       RegisterModule("grimoire-inject", func(srv *ipc.Server, d *Deps) {
           d.OnReady(func() { wireGrimoireInject(srv, d) })
       })
   }
   ```

3. **Direktori project malas** (`projectDirectory`), meniru `workspaceDirectory` tetapi menyimpan daftar project:
   - Tipe `projectEntry struct{ ID, Path string }`.
   - Struct `projectDirectory{ socket string; mu sync.Mutex; client *ipc.Client; entries []projectEntry; fetchedAt time.Time }`.
   - `func (p *projectDirectory) list() ([]projectEntry, bool)`: kunci `mu`; bila `entries == nil` atau `time.Since(fetchedAt) > 5*time.Second` panggil `refreshLocked()`. Bila refresh gagal dan masih ada cache lama, kembalikan cache lama; bila tidak ada, kembalikan `nil, false`.
   - `refreshLocked() bool`: dial malas `ipc.Dial(p.socket)` bila `client == nil`. Panggil `client.Call("workspace.list", map[string]any{}, &raw)` dengan `raw json.RawMessage` (params `map[string]any{}`, BUKAN nil dan bukan `struct{}{}` agar sesuai overview 2.5). Unmarshal ke `[]ws` dengan fallback objek `{workspaces:[...]}` seperti di mention. Bila `Call` gagal, tutup klien (`_ = client.Close()`), set `client = nil` agar dial ulang pada panggilan berikutnya, kembalikan false. Isi `entries` dari semua `Projects`, set `fetchedAt = time.Now()`. Tipe lokal `proj{ID, Path}` dan `ws{Projects}` didefinisikan di dalam fungsi.
   - `func (p *projectDirectory) Known(ctx context.Context, projectID string) (bool, error)`: ambil `list()`; bila gagal kembalikan `false, errors.New("daftar project tidak tersedia")`; kembalikan true bila ada entri dengan `ID == projectID`. Ini yang dipasang sebagai `InjectDeps.Known`. Catatan: project baru yang belum masuk cache 5 detik terakhir bisa ditolak sesaat; dapat diterima.
   - `func (p *projectDirectory) match(cwd string) string`: kembalikan `ID` project yang `Path`-nya sama dengan `cwd` atau merupakan induk dari `cwd`, pilih awalan terpanjang; `""` bila tak ada. Normalisasi kedua sisi dengan `filepath.Clean`; cek induk dengan `cwd == path || strings.HasPrefix(cwd, path+string(filepath.Separator))` (path `/` ditangani agar tidak menjadi `//`). `cwd == ""` langsung `""`.

4. **Penentuan projectId.** `func grimoireInjectProjectID(spec session.SessionSpec, dir *projectDirectory) string`: kembalikan `spec.ProjectID` bila tidak kosong; selain itu `dir.match(spec.Cwd)` (hanya bila `dir != nil`). Tidak ketemu = `""`.

5. **Argumen dengan guard duplikat.** Dua helper murni:
   - `func grimoireInjectHasArg(args []string, name string) bool`: true bila ada elemen `== name` atau berawalan `name+"="`.
   - `func grimoireInjectHasCodexInstructions(args []string) bool`: true bila ada elemen berawalan `developer_instructions=` (nilai pasangan `-c`; cukup cek elemen, termasuk bentuk `-cdeveloper_instructions=` tidak perlu).

6. **Logika contribution** `func grimoireInjectApply(ctx *provider.LaunchContext, dir *projectDirectory, logger *slog.Logger)`; semua cabang gagal melakukan `return` tanpa mengubah `ctx`:
   1. `ctx == nil || ctx.Provider == nil` -> return. `id := ctx.Provider.ID()`; bila bukan `provider.IDClaude` dan bukan `provider.IDCodex` -> return.
   2. Cek guard duplikat lebih awal supaya tidak menulis snapshot yang percuma: claude dan `grimoireInjectHasArg(ctx.Args, "--append-system-prompt-file")` -> return; codex dan `grimoireInjectHasCodexInstructions(ctx.Args)` -> return.
   3. `pid := grimoireInjectProjectID(ctx.Session, dir)`; kosong -> return (tanpa log, kasus normal).
   4. `text, ok, err := grimoire.ComposeInjection(pid)`. `err != nil`: `logger.Warn("grimoire: gagal menyusun suntikan", "project", pid, "err", err)` lalu return. `!ok` -> return diam-diam.
   5. `path, err := grimoire.WriteSnapshot(text)` dipanggil untuk KEDUA provider (codex memakainya hanya untuk audit). Bila gagal: log Warn. Untuk claude -> return (butuh berkasnya). Untuk codex, snapshot gagal TIDAK menghalangi suntikan argumen; lanjut dengan `path = ""`.
   6. Bila `err == nil` (snapshot sukses), jalankan prune: `if perr := grimoire.PruneSnapshots(nil); perr != nil { logger.Warn("grimoire: prune snapshot gagal", "err", perr) }`. `inUse` selalu `nil`: `session.Manager` tidak membuka cara memeriksa argumen/sesi hidup (verifikasi singkat dengan `grep -n "func (m \*Manager)" internal/session/*.go`; bila ternyata ada API yang benar-benar memetakan sesi hidup ke path snapshot, boleh dipakai, tetapi default dan yang diharapkan adalah `nil`). Prune dijalankan SETELAH snapshot ditulis dan SEBELUM args diubah tidak masalah; yang penting kegagalannya hanya dicatat.
   7. Tambah argumen dengan `append` saja:
      - claude: `ctx.Args = append(ctx.Args, "--append-system-prompt-file", path)`.
      - codex: `ctx.Args = append(ctx.Args, "-c", "developer_instructions="+grimoire.TOMLString(text))`.
      Tidak ada flag izin, tidak menghapus/mengurutkan ulang `Args`, tidak menyentuh `Env`/`MCPServers`.
   8. Bungkus seluruh isi fungsi dengan `defer func(){ if r := recover(); r != nil { logger.Warn("grimoire: panic di launch contribution", "panic", r) } }()` supaya panic tak menggagalkan peluncuran. `logger` yang nil diganti `slog.Default()` di awal (helper kecil `grimoireInjectLogger(l *slog.Logger) *slog.Logger`).

7. **Pemasangan tepat sekali.** Variabel paket `var grimoireInjectOnce sync.Once`. `func installGrimoireInject(dir *projectDirectory, logger *slog.Logger)` melakukan `grimoireInjectOnce.Do(func(){ provider.RegisterLaunchContribution(func(ctx *provider.LaunchContext){ grimoireInjectApply(ctx, dir, logger) }) })`. Bila `d.OnReady` terpanggil lebih dari sekali, contribution tetap satu.

8. **Wiring OnReady** `func wireGrimoireInject(srv *ipc.Server, d *Deps)`:
   1. `store := grimoire.NewStore(d.DataDir)`.
   2. `dir := &projectDirectory{socket: d.SocketPath}`.
   3. `grimoire.RegisterInject(srv, grimoire.InjectDeps{Store: store, Known: dir.Known, Logger: d.Logger})`. Ini sekali per proses (server panik bila method ganda); jangan dibungkus ulang. Jangan mendaftarkan method `grimoire.status` atau `grimoire.laws.*` dengan cara lain.
   4. Prune awal: `if err := grimoire.PruneSnapshots(nil); err != nil { d.Logger.Warn("grimoire: prune awal gagal", "err", err) }`.
   5. `installGrimoireInject(dir, d.Logger)`.
   Tidak memakai `d.Lookup`/`d.Provide` untuk store tiket 01 (overview 2.2 butir 6).

9. **Guard.** Jalankan `gofmt -l cmd/shikigamid` (harus kosong), `go build ./...`, dan opsional `go vet ./cmd/...`. Bila build gagal karena berkas fitur lain (bukan `wire_grimoire_inject.go`), jangan diperbaiki; laporkan. Pastikan `git status --short` hanya menunjukkan berkas ini sebagai perubahan baru milik fitur ini, dan `git diff --stat -- go.mod go.sum internal/session` kosong.

## Perubahan

- Dibuat: `cmd/shikigamid/wire_grimoire_inject.go`
- Diubah: tidak ada.

## Wiring

Fitur ini sendiri adalah berkas wiring daemon dan sudah mandiri; tidak ada yang perlu disambungkan ke `modules.go` atau `main.go` (modul terdaftar otomatis lewat `init()`, urutan alfabetis menempatkan `grimoire-inject` setelah `grimoire`). Yang diekspos untuk langkah wiring akhir (overview bagian 5, butir 3 dan 7), TIDAK dikerjakan di sini:

- Modul `grimoire-inject`, method IPC `grimoire.laws.get` / `grimoire.laws.set` lewat `grimoire.RegisterInject`.
- Satu launch contribution untuk `claude` dan `codex`, prune snapshot awal dan per-peluncuran.
- Agen wiring memverifikasi: tidak ada pendaftaran ganda `grimoire.laws.*` dengan tiket 01, `modules.go`/`main.go`/`wire_grimoire.go`/`wire_workspace.go` tak berubah, dan uji manual (argumen proses sesi `claude`/`codex` baru memuat `--append-system-prompt-file <path>` / `-c developer_instructions=...`; sesi `shell` tanpa tambahan; flag izin tidak ganda).
- Perluasan `grimoire.status` (blok `MeasureInjection` di `internal/grimoire/ipc.go`), CLI, dan panel adalah milik langkah wiring, bukan fitur ini.

## Selesai bila

- `cmd/shikigamid/wire_grimoire_inject.go` ada, `gofmt -l cmd/shikigamid` kosong, dan `go build ./...` sukses (terhadap stub Foundation atau implementasi fitur paralel; kegagalan akibat berkas milik fitur lain dilaporkan, bukan diperbaiki).
- Berkas memuat: `RegisterModule("grimoire-inject", ...)` dari `init()`; di `OnReady` memanggil `grimoire.NewStore(d.DataDir)`, `grimoire.RegisterInject` dengan `Known` dari `workspace.list` (klien malas, cache 5 detik, params `map[string]any{}`), prune awal, dan `provider.RegisterLaunchContribution` via `sync.Once`.
- Logika contribution sesuai langkah 6: keluar untuk ctx/Provider nil dan provider selain claude/codex; `projectId` dari `Session.ProjectID` atau `Cwd` terhadap `Project.Path` (awalan terpanjang); `ok=false`/error = tanpa suntikan (error dicatat); snapshot ditulis untuk kedua provider; claude menambah `--append-system-prompt-file <path>`, codex menambah `-c developer_instructions=`+`TOMLString(text)`; hanya `append`, tidak menambah bila flag sudah ada; `inUse=nil`; kegagalan snapshot/prune hanya dicatat dan tidak menggagalkan peluncuran.
- Tidak ada perubahan di `internal/session`, `internal/provider`, `internal/ipc`, `go.mod`, `go.sum`, tidak ada flag izin yang ditambah, dan tidak ada berkas lain yang disentuh.
- Tanpa unit test (guard hanya build).
