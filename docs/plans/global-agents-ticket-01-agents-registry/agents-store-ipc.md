# Plan: Penyimpanan, validasi, resolver, dan IPC agents

## Metadata

- Tiket: `docs/tickets/global-agents-ticket-01-agents-registry.md` (fitur `agents-store-ipc`)
- Overview (kontrak bersama, jangan diubah): `docs/plans/global-agents-ticket-01-agents-registry/overview.md`
- Spec: `docs/superpowers/specs/2026-11-22-global-agents-design.md`
- Module Go: `github.com/NooBeeID/shikigami`
- ownedPaths: `internal/agents/`, `cmd/shikigamid/wire_agents.go`
- Depends on foundation: tidak (tidak ada `00-foundation.md`). Fitur ini hanya memakai API yang sudah ada: `provider.Get`, `provider.IDShell`, `Capabilities().MCP`, `ipc.Server.Register/Publish`, `RegisterModule`, `Deps.Provide`, `Deps.DataDir`.

## Tujuan

Membangun paket Go `internal/agents` (registry kategori + agent persisten di `<DataDir>/agents.json`) beserta enam method IPC `agents.*`, event `agents.changed`, resolver `Resolve`, dan `SetDeleteGuard`, serta modul daemon `wire_agents.go`. Tipe, IPC, dan Go API persis seperti overview bagian 2.1 sampai 2.3; plan ini hanya menambah keputusan implementasi yang belum dipaku di sana.

## Path yang dimiliki

HANYA ini yang boleh disentuh; jangan sentuh file wiring lain atau path fitur lain:

- `internal/agents/` (semua berkas baru, `package agents`)
- `cmd/shikigamid/wire_agents.go` (berkas baru)

Dilarang: `internal/session`, `internal/provider` (hanya diimpor/dibaca), `internal/appconfig`, `cmd/shikigamid/modules.go`, `frontend/`, `overview.md`. Jangan menambah unit test (`_test.go`). Bila butuh sesuatu di luar path ini, catat di laporan; jangan edit.

## Kontrak

Disediakan (persis seperti overview 2.1 sampai 2.3, jangan ubah nama/tag):

- Tipe `Category`, `Agent` (tag JSON camelCase, lihat overview 2.1).
- `type ProviderLookup interface { Get(id string) (provider.Provider, bool) }`
- `func DefaultPath(dataDir string) string` -> `filepath.Join(dataDir, "agents.json")`
- `func Open(path string, reg ProviderLookup) *Store` (`reg == nil` -> pembungkus `provider.Get`)
- `func (s *Store) List() ([]Category, []Agent)`
- `func (s *Store) Resolve(ref string) (Agent, Category, error)`
- `func (s *Store) SetDeleteGuard(fn func(agentID string) error)`
- `func RegisterIPC(srv *ipc.Server, s *Store)`
- `var ErrNotFound = errors.New("agent atau kategori tidak ditemukan")`
- IPC: `agents.list`, `agents.categoryUpsert`, `agents.categoryDelete`, `agents.upsert`, `agents.delete`, `agents.setDefault` (param/hasil di overview 2.2); event `agents.changed` payload `{}` lewat `srv.Publish("agents.changed", map[string]any{})`.
- Layanan daemon: `d.Provide("agents.store", store)` bertipe `*agents.Store`.

Dipakai (sudah ada, baca saja): `provider.Get`, `provider.IDShell`, `provider.Provider.Capabilities().MCP`; `ipc.Error`, `ipc.Errorf`, `ipc.ErrInvalid/ErrNotFound/ErrConflict/ErrInvalidParams/ErrInternal` (`internal/ipc/errors.go`); `ipc.Handler = func(ctx, json.RawMessage) (any, error)`; pola tulis atomik `appconfig.(*Store).write` di `internal/appconfig/appconfig.go`.

Tambahan internal (boleh diekspor bila perlu, tidak ada di kontrak): `func (s *Store) Snapshot() (categories []Category, agents []Agent, err error)` dipakai IPC `agents.list` agar galat baca berkas sampai ke UI (`List()` mencatat log dan mengembalikan irisan kosong bila galat).

## Langkah

Semua komentar, log, dan galat bahasa Indonesia; identifier bahasa Inggris. Tiap galat validasi spesifik dan memuat nilai yang salah.

### 1. `internal/agents/types.go`

- `Category`, `Agent` persis seperti overview 2.1.
- `type fileState struct { Version int \`json:"version"\`; Categories []Category \`json:"categories"\`; Agents []Agent \`json:"agents"\` }`; `const fileVersion = 1`.
- `var ErrNotFound`, `var errStorage = errors.New("gagal mengakses penyimpanan agent")` (sentinel pembungkus galat I/O; dipakai IPC untuk memetakan ke `ipc.ErrInternal`).
- `type ProviderLookup interface{...}`; `type globalLookup struct{}` dengan `Get` memanggil `provider.Get`.
- Konstanta id bawaan: `idOrchestrator = "orchestrator"`, `idResearcher`, `idPlanner`, `idExecutor`.

### 2. `internal/agents/seed.go`

- `func builtinCategories() []Category` urut: orchestrator, researcher, planner, executor (`Builtin: true`), nama `Orchestrator`, `Researcher`, `Planner`, `Executor`, peran persis dari tiket:
  - Orchestrator: "Mengoordinasi pekerjaan: memecah tujuan, memilih agent yang tepat, dan memantau hasilnya; tidak mengerjakan implementasi sendiri."
  - Researcher: "Menyelidiki dan mengumpulkan informasi; hanya membaca, tidak mengubah kode."
  - Planner: "Hanya merencanakan: menyusun rencana langkah demi langkah; tidak mengubah kode."
  - Executor: "Mengerjakan perubahan kode sesuai rencana dan melaporkan hasilnya."
- `func seedMissing(st *fileState) (changed bool)`: untuk tiap kategori bawaan yang id-nya belum ada di `st.Categories`, tambahkan di akhir. Tidak menyentuh kategori yang sudah ada (nama/peran hasil ubahan pengguna dipertahankan). Juga paksa `Builtin = true` pada kategori ber-id bawaan bila tidak (kembalikan `changed`).

### 3. `internal/agents/storage.go`

- `type Store struct { mu sync.Mutex; path string; reg ProviderLookup; guard func(string) error }`.
- `func DefaultPath`, `func Open(path string, reg ProviderLookup) *Store` (jika `reg == nil` pakai `globalLookup{}`; `Open` tidak melakukan I/O).
- `func (s *Store) load() (fileState, error)` (dipanggil dengan `mu` terkunci):
  1. `os.ReadFile(path)`; `fs.ErrNotExist` atau isi kosong (trim) -> state kosong + seed, lalu `save` (berkas terbentuk 0600).
  2. JSON tidak valid: KEPUTUSAN, cadangkan lalu seed ulang. `os.Rename(path, path+".bak")` (menimpa `.bak` lama), `log.Printf("agents: %s rusak (%v); dicadangkan ke %s dan di-seed ulang", ...)`, lalu seed + `save`. Bila rename gagal, kembalikan galat `fmt.Errorf("%w: berkas %s rusak dan tidak bisa dicadangkan: %v", errStorage, ...)` tanpa menimpa. Dokumentasikan keputusan ini di komentar doc `load`.
  3. `Version > fileVersion`: galat "agents.json versi N lebih baru dari yang didukung (1); perbarui aplikasi" (tanpa menimpa). `Version` 0 atau 1 diterima; saat disimpan selalu 1.
  4. Setelah decode: pastikan `Categories`/`Agents` tidak nil, panggil `seedMissing`; bila `changed`, `save`.
- `func (s *Store) save(st fileState) error`: tiru `appconfig.write` -> `os.MkdirAll(dir, 0o700)`, `json.MarshalIndent`, `os.CreateTemp(dir, "agents-*.tmp")`, `Chmod(0o600)`, `Write`, `Sync()` (fsync, tambahan dari appconfig), `Close`, `os.Rename`; bersihkan temp saat gagal. Bungkus galat dengan `%w` + `errStorage`.
- Pola mutasi untuk semua operasi tulis: `Lock` -> `load` -> ubah salinan state -> validasi -> `save` -> kembalikan hasil. Bila validasi gagal, tidak ada tulis (state di disk tidak berubah). Sumber kebenaran adalah berkas (dibaca ulang tiap operasi, seperti `appconfig`), jadi restart otomatis konsisten.

### 4. `internal/agents/validate.go`

- `func normalizeAgentName(raw string) (string, error)`: `strings.TrimSpace`, `ToLower`, urutan spasi/tab menjadi satu `-` (`strings.Fields` lalu `Join(..., "-")`); wajib cocok `^[a-z0-9][a-z0-9-]*$`. Galat: kosong -> "nama agent wajib diisi"; ilegal -> `nama agent %q tidak valid: hanya huruf kecil, angka, dan "-" (tidak boleh diawali "-")`.
- `func slugify(name string) string` untuk id kategori: lowercase, karakter di luar `[a-z0-9]` menjadi `-`, rapatkan `-` beruntun, pangkas `-` di tepi; kosong -> `"kategori"`.
- `func uniqueCategoryID(base string, cats []Category) string`: `base`, bila bentrok `base-2`, `base-3`, ... sampai unik (juga terhadap id bawaan, sudah ada di `cats`).
- `func newAgentID() string`: `"agt_" + hex(8 byte crypto/rand)`.
- `func normalizeCategoryName(raw string) (string, error)`: trim; kosong -> "nama kategori wajib diisi". Keunikan (case-insensitive, `strings.EqualFold`, kecuali kategori itu sendiri) diperiksa di `categoryUpsert`.
- `func (s *Store) validateProvider(providerID, categoryID string) error`: trim id; kosong -> "provider wajib diisi"; `id == provider.IDShell` -> `provider shell tidak boleh dipakai sebagai agent`; `s.reg.Get(id)` tidak ada -> `provider %q tidak terdaftar`; bila `categoryID == idOrchestrator` dan `!p.Capabilities().MCP` -> `provider %s tidak mendukung MCP, tidak bisa jadi Orchestrator`. Tidak memeriksa ketersediaan binary dan tidak memvalidasi nama model (model hanya di-`TrimSpace`).

### 5. `internal/agents/store.go` (agent + default)

- Helper murni atas `*fileState`: `findAgent(id)`, `findAgentByName(name)`, `findCategory(id)`, `agentsIn(categoryID) []int`, `sortedAgents(st)` (urut nama), `copyState`.
- `func (st *fileState) normalizeDefaults(categoryID string)`: untuk satu kategori: bila lebih dari satu default, pertahankan yang pertama di slice dan cabut sisanya (pemulihan data rusak); bila nol default dan agent di kategori itu tepat satu, jadikan default. Dipanggil setelah setiap mutasi pada kategori yang terdampak.
- `func (s *Store) List() ([]Category, []Agent)` memanggil `Snapshot`; galat -> `log.Printf` dan kembalikan `[]Category{}, []Agent{}` (tidak pernah nil). `Snapshot()`: kunci, `load`, salin; kategori urut slice (urutan dibuat), agent urut nama; irisan tidak pernah nil.
- `type AgentInput struct { ID, Name, CategoryID, ProviderID, Model string; IsDefault bool }`
- `func (s *Store) UpsertAgent(in AgentInput) (Agent, error)`:
  1. Normalisasi nama, kategori harus ada (galat `kategori %q tidak ditemukan`), `validateProvider(in.ProviderID, in.CategoryID)`, `Model = TrimSpace`.
  2. Nama unik global: bentrok dengan agent lain (id berbeda) -> `nama agent %q sudah dipakai`.
  3. `in.ID != ""` tapi tidak ada -> galat `agent %q tidak ditemukan` (bungkus `ErrNotFound`); kosong -> buat agent baru dengan `newAgentID()`.
  4. Aturan default (didokumentasikan di komentar): agent pertama di kategori tujuan otomatis default walau `IsDefault=false`; `IsDefault=true` mencabut default lain di kategori tujuan; `IsDefault=false` pada agent yang sedang default di kategori yang SAMA tidak mencabutnya (default hanya berpindah lewat penanda default lain atau `SetDefault`, sehingga kategori tidak diam-diam kehilangan default); agent yang pindah kategori memakai `IsDefault` input di kategori tujuan dan hanya default bila `IsDefault=true` atau tujuan belum punya agent lain.
  5. Kategori asal bila berubah: `normalizeDefaults(asal)` dan `normalizeDefaults(tujuan)` (aturan "tersisa tepat satu menjadi default").
  6. `save`, kembalikan `Agent` hasil.
- `func (s *Store) DeleteAgent(id string) error`: agent harus ada (`ErrNotFound`); panggil guard (`if s.guard != nil { if err := s.guard(id); err != nil { return err } }`) SEBELUM menghapus; hapus; `normalizeDefaults(kategori)`. Komentar kode wajib menjelaskan: bila agent default dihapus dan masih ada agent lain, default TIDAK dialihkan otomatis, kecuali tersisa tepat satu agent, maka agent itu jadi default (efek `normalizeDefaults`). Catatan: guard dipanggil dengan `mu` terkunci; guard tidak boleh memanggil kembali method `Store` (deadlock), dokumentasikan di komentar `SetDeleteGuard`.
- `func (s *Store) SetDefault(id string) error`: agent harus ada; cabut default lain di kategorinya, set default; `save`.
- `func (s *Store) SetDeleteGuard(fn func(agentID string) error)`: kunci `mu`, simpan; default nil.

### 6. `internal/agents/categories.go`

- `func (s *Store) UpsertCategory(id, name, role string) (Category, error)`:
  - nama dinormalisasi; unik case-insensitive terhadap kategori lain (`kategori %q sudah ada`); `role` di-`TrimSpace` (boleh kosong).
  - `id == ""`: buat baru, `ID = uniqueCategoryID(slugify(name), cats)`, `Builtin=false`, tambahkan di akhir slice.
  - `id != ""`: harus ada (`ErrNotFound`); ubah hanya `Name` dan `Role` (juga untuk bawaan); `ID` dan `Builtin` tidak berubah.
- `type DeleteMode string`; konstanta `DeleteNone = ""`, `DeleteMove = "move"`, `DeleteAll = "delete"`.
- `func (s *Store) DeleteCategory(id string, mode DeleteMode, moveTo string) error`, semua validasi dulu, baru ubah (atomik: gagal = tidak ada perubahan):
  1. Kategori harus ada; `Builtin` ditolak ("kategori bawaan %s tidak bisa dihapus").
  2. `affected := agentsIn(id)`. Kosong: hapus kategori (abaikan `mode`).
  3. Tidak kosong dan `mode == ""`: galat "kategori %s masih punya N agent; pilih pindahkan atau hapus agent-nya". Mode selain `move`/`delete`: galat nilai tidak valid.
  4. Panggil guard untuk SETIAP agent terdampak (mode `move` maupun `delete`); galat pertama dikembalikan apa adanya.
  5. `delete`: hapus agent-nya lalu kategori.
  6. `move`: `moveTo` wajib, harus ada, tidak boleh sama dengan `id`; untuk tiap agent yang dipindah jalankan `validateProvider(a.ProviderID, moveTo)` (cek ulang aturan Orchestrator/MCP; satu saja gagal membatalkan semuanya, galat menyebut nama agent). Pindahkan: set `CategoryID = moveTo`; semua agent yang dipindah `IsDefault=false` bila tujuan sudah punya default (tidak merebut); bila tujuan belum punya default, jadikan default agent yang berstatus default di asal, atau yang pertama menurut nama bila tak ada; lalu `normalizeDefaults(moveTo)`. Hapus kategori asal.
  7. `save`.

### 7. `internal/agents/resolve.go`

`func (s *Store) Resolve(ref string) (Agent, Category, error)` (satu-satunya tempat aturan resolusi; kunci, `load`, baca saja):
1. `ref` di-`TrimSpace`; kosong -> `ErrNotFound` terbungkus.
2. Nama agent: normalisasi slug (bila normalisasi gagal, lewati langkah ini, jangan galat); cocok persis dengan `Agent.Name` -> kembalikan agent beserta kategorinya.
3. Id kategori persis (`Category.ID == ref`, juga bandingkan `ToLower(ref)`).
4. Nama kategori `strings.EqualFold`.
5. Untuk langkah 3 dan 4: cari agent `IsDefault` di kategori itu; tidak ada (kategori kosong atau tanpa default) -> `fmt.Errorf("kategori %s belum punya agent default", cat.Name)` (BUKAN `ErrNotFound`, agar pemanggil membedakan; jangan menebak agent lain).
6. Tidak ada yang cocok: `fmt.Errorf("%w: %q", ErrNotFound, ref)` (bisa dibedakan dengan `errors.Is`).
Catatan komentar: nama agent didahulukan atas kategori bila bentrok (mis. agent bernama `planner`).

### 8. `internal/agents/ipc.go`

`func RegisterIPC(srv *ipc.Server, s *Store)` mendaftarkan enam handler (`srv.Register(name, ipc.Handler)`); helper `decode(params json.RawMessage, v any) error` (params kosong/`null` = struct nol; JSON salah -> `ipc.Errorf(ipc.ErrInvalidParams, "parameter tidak valid: %v", err)`); helper `changed := func() { srv.Publish("agents.changed", map[string]any{}) }` dipanggil HANYA setelah mutasi sukses (kelima method mutasi), tidak untuk `agents.list`.
- `agents.list`: `Snapshot()` -> `struct{Categories []Category \`json:"categories"\`; Agents []Agent \`json:"agents"\`}` (tidak pernah null).
- `agents.categoryUpsert` param `{id?, name, role}` -> `Category`.
- `agents.categoryDelete` param `{id, agents?: string, moveTo?: string}` (field JSON `agents` dipetakan ke `DeleteMode`) -> `map[string]any{}`.
- `agents.upsert` param `{id?, name, categoryId, providerId, model, isDefault}` -> `Agent`.
- `agents.delete` param `{id}` -> `{}`; `agents.setDefault` param `{id}` -> `{}`.
- Pemetaan galat `func toIPCError(err error) error`: sudah `*ipc.Error` -> apa adanya; `errors.Is(err, ErrNotFound)` -> `ipc.NewError(ipc.ErrNotFound, err.Error())`; `errors.Is(err, errStorage)` -> `ipc.ErrInternal`; galat guard -> `ipc.ErrConflict` (bungkus galat guard dalam tipe internal `guardError` yang dikenali `errors.As`, dibuat di `DeleteAgent`/`DeleteCategory` saat guard menolak; pesan tetap persis pesan guard); sisanya (validasi) -> `ipc.ErrInvalid`. Pesan selalu `err.Error()` bahasa Indonesia, tanpa menambah prefiks.

### 9. `cmd/shikigamid/wire_agents.go`

```go
package main

import (
	"github.com/NooBeeID/shikigami/internal/agents"
	"github.com/NooBeeID/shikigami/internal/ipc"
)

func init() {
	RegisterModule("agents", func(srv *ipc.Server, d *Deps) {
		store := agents.Open(agents.DefaultPath(d.DataDir), nil)
		d.Provide("agents.store", store)
		agents.RegisterIPC(srv, store)
	})
}
```

Impor `internal/provider` TIDAK perlu di file ini (registry global dipakai lewat `globalLookup` di paket `agents`, yang mengimpor `internal/provider` sehingga `init()` provider ikut jalan). Jangan ubah `modules.go`.

### 10. Verifikasi

Jalankan `go build ./...` dan `go vet ./internal/agents/ ./cmd/shikigamid/` dari root repo; perbaiki sampai bersih. Jangan membuat `_test.go`. Bila memungkinkan, cek manual tanpa menyimpan berkas ke repo (mis. skrip sementara di luar repo) bahwa data dir kosong menghasilkan `agents.json` mode 0600 dengan empat kategori.

## Perubahan

Dibuat (semua baru):
- `internal/agents/types.go`
- `internal/agents/seed.go`
- `internal/agents/storage.go`
- `internal/agents/validate.go`
- `internal/agents/store.go`
- `internal/agents/categories.go`
- `internal/agents/resolve.go`
- `internal/agents/ipc.go`
- `cmd/shikigamid/wire_agents.go`

Tidak ada berkas yang diubah.

## Wiring

Fitur ini sudah mengerjakan sendiri bagian backend wiring yang ia miliki (`wire_agents.go`, lewat `init()` + `RegisterModule`; berkas `wire_*.go` otomatis ikut build). Yang diekspos untuk langkah wiring/tiket berikutnya, TANPA disambungkan di sini:
- Layanan `agents.store` (`*agents.Store`) lewat `d.Lookup("agents.store")`.
- `Store.Resolve`, `Store.List`, `Store.SetDeleteGuard` (tiket 02 dan 03; guard dipasang tiket 03).
- Method IPC `agents.*` dan event `agents.changed` untuk halaman Settings (fitur `agents-settings-page`).
- Penyambungan frontend (`import './agents';` di `frontend/src/modules/settings/index.ts`) dan verifikasi `Launcher`/`Spec.Model` adalah langkah wiring terpisah; jangan dikerjakan di sini.

## Selesai bila

- `go build ./...` dari root repo sukses (guard; tanpa unit test).
- Hanya berkas di bagian "Perubahan" yang dibuat/diubah; `git status` tidak menunjukkan perubahan di `internal/session`, `internal/provider`, `modules.go`, `frontend/`, atau `overview.md`.
- Seluruh perilaku acceptance criteria fitur `agents-store-ipc` pada tiket terpenuhi secara desain: seed empat kategori + `agents.json` 0600; agent pertama otomatis default dan perebutan default; penolakan provider tak terdaftar, `shell`, dan Orchestrator tanpa MCP; nama dinormalisasi dan unik; `categoryDelete` bawaan/ber-agent/`move`/`delete`; `Resolve` untuk nama agent, id kategori, nama kategori, kategori tanpa default (galat jelas), ref tak dikenal (`errors.Is(err, ErrNotFound)`); guard menolak `agents.delete`; event `agents.changed` setelah tiap mutasi sukses; data bertahan setelah restart.
- Komentar, log, dan galat berbahasa Indonesia; identifier berbahasa Inggris.
