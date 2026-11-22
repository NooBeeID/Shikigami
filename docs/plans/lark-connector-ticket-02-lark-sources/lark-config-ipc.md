# Plan: LarkConfig dan IPC sumber

## Metadata
- Tiket: `lark-connector-ticket-02-lark-sources` (`docs/tickets/lark-connector-ticket-02-lark-sources.md`)
- Fitur: `lark-config-ipc` (LarkConfig dan IPC sumber)
- ownedPaths:
  - `internal/workspace/lark.go`
  - `cmd/shikigamid/wire_lark.go`
  - `internal/workspace/types.go` (hanya bila Foundation belum menambahkan tipe Lark)
- Depends on foundation: ya. Foundation (`00-foundation.md`) menambah tipe `LarkConfig`/`TicketSource`/`WikiSource` + field `WorkspaceConfig.Lark` di `types.go`, stub `internal/workspace/lark.go`, dan `d.Provide("workspace.store", store)` di `wire_workspace.go`. Sebelum mulai, cek `grep -n "LarkConfig" internal/workspace/types.go` dan `grep -n workspace.store cmd/shikigamid/wire_workspace.go`. Bila tipe belum ada, tambahkan persis seperti overview 2.1 (hanya itu di `types.go`). Jangan menyentuh `wire_workspace.go` (milik Foundation); bila `Provide` belum ada, laporkan sebagai ketergantungan Foundation dan tetap selesaikan kode (handler mencari store secara malas).

## Tujuan
Mengganti stub `Store` Lark dengan implementasi nyata (aman bersamaan, deep copy, tulis atomik, ID ULID, validasi dan penolakan duplikat) dan mengekspos IPC `lark.sources.get/add/remove/rename` di modul daemon `lark`, dengan galat berformat `[code] pesan Indonesia` dan publikasi `workspace.changed` `configChanged` setelah tiap mutasi. Kontrak lengkap: `overview.md` bagian 2.1 s.d. 2.5 (jangan diulang di sini, jangan diubah).

## Path yang dimiliki
HANYA path ini yang boleh disentuh:
- `internal/workspace/lark.go` (timpa penuh stub Foundation)
- `cmd/shikigamid/wire_lark.go` (baru)
- `internal/workspace/types.go` (hanya bila tipe Lark belum ada)

Dilarang: `internal/workspace/ipc.go` (`config.set`), `config.go`, `store.go`, `wire_workspace.go`, `wire_lark_link.go` (fitur lain), `modules.go`, config Grimoire, apa pun di `internal/larkdata`, `cmd/shikigamictl`, `frontend`. `internal/workspace` tidak boleh mengimpor `larkdata`. Working tree punya perubahan tak terkait; jangan di-stage.

## Kontrak
Disediakan (signature persis overview 2.2):
```go
func (s *Store) LarkSources(wsID string) (LarkConfig, error)
func (s *Store) AddLarkTicketSource(wsID string, src TicketSource) (TicketSource, error)
func (s *Store) AddLarkWikiSource(wsID string, src WikiSource) (WikiSource, error)
func (s *Store) RemoveLarkSource(wsID, sourceID string) error
func (s *Store) RenameLarkSource(wsID, sourceID, name string) error
```
IPC (overview 2.5): `lark.sources.get`, `lark.sources.add`, `lark.sources.remove`, `lark.sources.rename`. Event: `workspace.changed` `{kind:"configChanged", workspaceId, workspace}`.

Dipakai: `newID()`, `ErrNotFound/ErrInvalid/ErrConflict`, `s.mu`, `s.workspaces`, `s.ui`, `s.indexLocked`, `s.persistLocked(ws []Workspace, ui UIState)`, `cloneWorkspaces`, `Store.Get` (semua di paket `workspace`); `ipc.Server.Register/Publish`, `ipc.NewError`, `ipc.ErrNotFound/ErrConflict/ErrInvalid/ErrInternal`; `Deps.Lookup` (`cmd/shikigamid/modules.go`), pola `RegisterModule` seperti `wire_grimoire.go`.

## Langkah

### 1. Cek Foundation
Pastikan tipe Lark ada di `types.go` (lihat Metadata). Baca `internal/workspace/store.go` (`persistLocked`, `cloneWorkspaces`, `indexLocked`) dan `config.go` (`SetWorkspaceConfig`) untuk pola kunci. `s.mu` adalah `sync.RWMutex`; mutasi memakai `s.mu.Lock()`, baca memakai `RLock`.

### 2. Timpa `internal/workspace/lark.go`
Package `workspace`. Hapus komentar "Stub Foundation". Impor: `fmt`, `strings`. Isi:

Helper:
```go
// cloneLark menyalin dalam LarkConfig; hasil tak pernah memuat slice nil.
func cloneLark(c *LarkConfig) LarkConfig {
    out := LarkConfig{Tickets: []TicketSource{}, Wiki: []WikiSource{}}
    if c == nil { return out }
    out.Tickets = append(out.Tickets, c.Tickets...)
    out.Wiki = append(out.Wiki, c.Wiki...)
    return out
}
```
(Struct sumber hanya berisi string sehingga salinan slice sudah deep copy.)

Helper mutasi bersama untuk menghindari duplikasi kode:
```go
// mutateLark: kunci tulis, salin cloneWorkspaces(s.workspaces), jalankan fn pada
// salinan LarkConfig workspace, persist, lalu ganti s.workspaces.
func (s *Store) mutateLark(wsID string, fn func(lc *LarkConfig) error) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    i := s.indexLocked(wsID)
    if i < 0 { return fmt.Errorf("%w: workspace %q", ErrNotFound, wsID) }
    next := cloneWorkspaces(s.workspaces)
    lc := cloneLark(next[i].Config.Lark)   // salinan baru; pointer lama tidak dimutasi
    if err := fn(&lc); err != nil { return err }
    next[i].Config.Lark = &lc
    if err := s.persistLocked(next, s.ui); err != nil { return err }
    s.workspaces = next
    return nil
}
```
Penting: `cloneWorkspaces` menyalin `Config` dangkal, sehingga pointer `Lark` dibagi antar salinan; karena itu selalu buat `LarkConfig` baru lewat `cloneLark` dan tetapkan pointer baru, jangan memutasi `*next[i].Config.Lark`. Jangan menyentuh `GrimoireURL/GrimoireKey` atau field lain. Bila `persistLocked` gagal, `s.workspaces` tidak berubah.

Metode:
- `LarkSources(wsID)`: `RLock`; workspace tak ada -> `ErrNotFound`; kembalikan `cloneLark(s.workspaces[i].Config.Lark)` (slice tak nil). `wsID` kosong -> `ErrInvalid` ("workspaceId kosong").
- `AddLarkTicketSource(wsID, src)`: trim `Name`, `BaseToken`, `Host`, `TableID`, `ViewID`. `Name==""` -> `fmt.Errorf("%w: nama tidak boleh kosong", ErrInvalid)`. `BaseToken==""` atau `Host==""` -> `ErrInvalid` ("baseToken dan host wajib diisi"). Di dalam `mutateLark`: tolak bila ada `Tickets[k].BaseToken == src.BaseToken` -> `fmt.Errorf("%w: sumber sudah ada", ErrConflict)`; set `src.ID = newID()` (ULID dibuat di sini, abaikan ID kiriman pemanggil); `append`; kembalikan `src`. Duplikat hanya berdasarkan `baseToken` (table/view tidak ikut).
- `AddLarkWikiSource(wsID, src)`: trim `Name`, `SpaceID`, `RootToken`, `Host`. Validasi nama kosong -> `ErrInvalid`; `SpaceID==""` atau `Host==""` -> `ErrInvalid`. Tolak `SpaceID == "my_library"` -> `ErrInvalid` ("wiki pribadi tidak disimpan"). Duplikat bila `SpaceID` dan `RootToken` keduanya sama dengan entri ada -> `ErrConflict`. `ID = newID()`.
- `RemoveLarkSource(wsID, sourceID)`: cari `sourceID` di `Tickets` lalu `Wiki` (ULID unik, tak perlu kind); hapus dengan membangun slice baru (jangan `append(a[:i], a[i+1:]...)` pada slice yang dibagi). Tak ketemu -> `fmt.Errorf("%w: sumber %q", ErrNotFound, sourceID)`.
- `RenameLarkSource(wsID, sourceID, name)`: trim `name`; kosong -> `ErrInvalid` (dicek sebelum mencari sumber). Ubah `Name` pada elemen slice salinan; tak ketemu -> `ErrNotFound`.

Semua pesan galat berbahasa Indonesia. Tidak ada impor `larkdata`; tidak ada panggilan jaringan/CLI.

### 3. Buat `cmd/shikigamid/wire_lark.go`
Package `main`. Impor: `context`, `encoding/json`, `errors`, `strings`, `internal/ipc`, `internal/workspace`.

Pendaftaran, `init()`:
```go
func init() {
    RegisterModule("lark", func(srv *ipc.Server, d *Deps) { registerLarkSources(srv, d) })
}
```
Nama modul `lark` harus unik di antara file `wire_*.go` (cek `grep -rn 'RegisterModule("lark"' cmd/shikigamid`; `lark-link` milik fitur lain).

Helper (nama persis, milik file ini):
```go
func larkStore(d *Deps) (*workspace.Store, error) {
    v, ok := d.Lookup("workspace.store")
    st, isStore := v.(*workspace.Store)
    if !ok || !isStore || st == nil {
        return nil, ipc.NewError(ipc.ErrInternal, "workspace store tidak tersedia")
    }
    return st, nil
}
func larkErr(code string, ipcCode <ipc code type>, msg string) error {
    return ipc.NewError(ipcCode, "["+code+"] "+msg)
}
```
Sesuaikan tipe kode IPC dengan signature `ipc.NewError` yang ada (baca `internal/ipc`). Lookup dilakukan malas di dalam handler (modul `lark` berjalan sebelum `workspace` secara alfabetis, jadi jangan Lookup saat wiring). Pesan galat Indonesia tambahan: bila `larkStore` gagal, tetap `ipc.ErrInternal` tanpa awalan kode Lark.

`mapLarkStoreErr(err error) error` memetakan sentinel store:
- `errors.Is(err, workspace.ErrNotFound)` -> `[not_found] Sumber tidak ditemukan` dengan `ipc.ErrNotFound`. Bila pesan memuat "workspace", boleh pakai "Workspace tidak ditemukan" (tetap kode `not_found`).
- `workspace.ErrConflict` -> `[duplicate] Sumber sudah ada` dengan `ipc.ErrConflict`.
- `workspace.ErrInvalid` -> `[invalid_params] <err.Error()>` dengan `ipc.ErrInvalid` (kasus nama kosong sudah ditangkap handler sebagai `invalid_name` sebelum memanggil store).
- lainnya: `ipc.NewError(ipc.ErrInternal, err.Error())`.

Dekode params: pakai `json.Unmarshal` pada `json.RawMessage` (tangani kosong/`null` sebagai `{}`); gagal -> `[invalid_params] Parameter tidak valid`, `ipc.ErrInvalid`. Salin pola `decode` dari `internal/workspace/ipc.go` (tidak diekspor; tulis versi lokal bernama `larkDecode`).

Publikasi:
```go
func publishLarkChanged(srv *ipc.Server, st *workspace.Store, wsID string) {
    ws, err := st.Get(wsID)
    if err != nil { return }
    srv.Publish("workspace.changed", map[string]any{
        "kind": "configChanged", "workspaceId": wsID, "workspace": &ws,
    })
}
```
Bentuk identik dengan `publish()` di `internal/workspace/ipc.go`.

Handler (di `registerLarkSources`, semua `srv.Register(name, func(ctx context.Context, params json.RawMessage) (any, error)`):
- `lark.sources.get {workspaceId}`: `workspaceId` kosong -> `[invalid_params] workspaceId wajib diisi`. Panggil `st.LarkSources`. Kembalikan `map[string]any{"tickets": lc.Tickets, "wiki": lc.Wiki}`; pastikan keduanya non-nil (ganti nil menjadi slice kosong) agar JSON `[]`, bukan `null`.
- `lark.sources.add {workspaceId, kind, name, baseToken?, host?, tableId?, viewId?, spaceId?, rootToken?}`: validasi di handler sebelum store, dalam urutan: `workspaceId` kosong -> `invalid_params`; `kind` bukan `ticket`/`wiki` -> `[invalid_params] kind harus ticket atau wiki`; `strings.TrimSpace(name)==""` -> `[invalid_name] Nama tidak boleh kosong` (`ipc.ErrInvalid`); ticket: `baseToken` atau `host` kosong -> `[invalid_params] baseToken dan host wajib diisi`; wiki: `spaceId` atau `host` kosong -> `[invalid_params] spaceId dan host wajib diisi`. Lalu `AddLarkTicketSource`/`AddLarkWikiSource`; sukses -> `publishLarkChanged`, kembalikan sumber yang dibuat (struct dengan `id`). `ipc.ErrInvalid` untuk semua `invalid_*`.
- `lark.sources.remove {workspaceId, sourceId}`: field kosong -> `invalid_params`; `RemoveLarkSource`; sukses -> publish, kembalikan `map[string]any{}`.
- `lark.sources.rename {workspaceId, sourceId, name}`: field id kosong -> `invalid_params`; `name` kosong -> `invalid_name`; `RenameLarkSource`; sukses -> publish, `map[string]any{}`.

Publikasi hanya setelah mutasi sukses; gagal tidak mempublikasikan apa pun. Wiki pribadi (`personal`/`my_library`) tidak pernah ditulis.

### 4. Build
Jalankan `go build ./...` dari root repo; perbaiki galat di path sendiri. Jalankan juga `gofmt -l internal/workspace cmd/shikigamid` dan pastikan file milik fitur ini bersih. `go vet ./internal/workspace ./cmd/shikigamid` bila cepat. Tanpa unit test.

## Perubahan
- Diubah (timpa stub): `internal/workspace/lark.go`
- Dibuat: `cmd/shikigamid/wire_lark.go`
- Diubah hanya bila Foundation belum menambahkannya: `internal/workspace/types.go` (satu field `Lark` + tiga tipe)

## Wiring
Yang diekspos untuk langkah wiring (`lark-sources-wiring`, jangan disambungkan sendiri):
- Method IPC `lark.sources.get/add/remove/rename` terdaftar otomatis lewat `init()` modul `lark`; `modules.go` tidak perlu diubah.
- Fungsi `larkStore(d *Deps)` ada di paket `main` (`wire_lark.go`); `wire_lark_link.go` memakai helper sendiri, jangan diimpor/dirujuk silang.
- Metode `Store.LarkSources` dipakai fitur `lark-link-parser` (`lark.tables.list`/`lark.views.list`) dan ctl.
- Bergantung pada `d.Provide("workspace.store", ...)` dari Foundation (diverifikasi di tahap Wiring lewat `shikigamictl`/IPC).
- Catat penyimpangan kontrak (bila ada) di akhir laporan implementasi. Satu keputusan di plan ini: wiki dengan `spaceId` `my_library` ditolak `ErrInvalid`, duplikat ticket hanya per `baseToken`.

## Selesai bila
- `go build ./...` dari root repo lulus (guard utama; tanpa unit test).
- `lark.go` tidak lagi berisi stub "belum diimplementasi"; kelima metode memakai `s.mu`, deep copy `LarkConfig`, `persistLocked`, `newID()`.
- Duplikat -> `ErrConflict`, nama kosong -> `ErrInvalid`, sumber/workspace tak ada -> `ErrNotFound`; `get` mengembalikan `[]` bukan `null`.
- Galat IPC berbentuk `[invalid_name|not_found|duplicate|invalid_params] pesan Indonesia`; `workspace.changed` `configChanged` dipublikasikan setelah tiap mutasi sukses.
- `internal/workspace` tidak mengimpor `larkdata`; `config.set`, config Grimoire, dan file di luar ownedPaths tidak berubah (`git diff --stat` hanya menampilkan path fitur ini, selain perubahan tak terkait yang sudah ada sebelumnya).
