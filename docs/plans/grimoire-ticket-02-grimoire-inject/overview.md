# Overview: grimoire-ticket-02-grimoire-inject

Tiket: `docs/tickets/grimoire-ticket-02-grimoire-inject.md`. Spec: `docs/superpowers/specs/2026-11-22-grimoire-design.md`.
Dokumen ini adalah sumber kebenaran BERSAMA untuk semua plan fitur. Bila ada perbedaan kecil dengan tiket, tiket menang kecuali dinyatakan "koreksi" di bawah. Tanpa dependency baru, tanpa unit test; guard hanya build.

Catatan keadaan repo saat overview ditulis: `internal/grimoire/` dan `frontend/src/modules/grimoire/` belum ada di working tree (tiket 01 baru berupa plan di `docs/plans/grimoire-ticket-01-grimoire-build/`). Tiket 02 `depends_on` tiket 01, jadi saat dieksekusi kode tiket 01 SUDAH ada. Setiap agent wajib membaca kode tiket 01 yang nyata (`internal/grimoire/store.go`, `index.go`, `ipc.go`, `cmd/shikigamid/wire_grimoire.go`, `cmd/shikigamictl/cmd_grimoire.go`, `frontend/src/modules/grimoire/*`) sebelum menulis, dan memakai API-nya apa adanya. Bila nama/signature nyata beda dari yang dikutip di bawah (diambil dari overview tiket 01), ikuti kode nyata dan catat di plan.

## 1. Ringkasan epic dan urutan eksekusi

Epic ini membuat agent benar-benar menerima grimoire. Daemon menyusun satu teks suntikan (isi `index.md` yang sudah memuat header aturan konstan dari tiket 01, dengan placeholder `{{grimoire_dir}}` diganti path absolut, ditambah `laws.md`), membekukannya per sesi sebagai snapshot `<DataDir>/run/grimoire-<hex>.md` (0600), lalu memasangnya ke sesi baru `claude` (`--append-system-prompt-file <snapshot>`) dan `codex` (`-c developer_instructions=<string TOML>`) lewat `provider.RegisterLaunchContribution` dengan teks byte-identik. Pengguna dapat menulis `laws.md` dari panel (`LawsEditor`), dan ukuran suntikan (byte, ~token) ditampilkan di panel dan `shikigamictl grimoire status`.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): stub berkompilasi dengan signature final untuk `inject.go`, `snapshot.go`, `laws.go`, `measure.go`. Setelah selesai `go build ./...` lulus. Tidak ada foundation frontend.
2. **Fitur paralel** (4 fitur, bagian 3): tiap agent menimpa penuh berkas ownedPaths-nya; signature TIDAK berubah. `laws-ui` murni frontend (kontrak hanya IPC); `launch-contribution` hanya mengompilasi terhadap stub Foundation.
3. **Wiring** (bagian 5): tiga sambungan kecil ke berkas tiket 01 (status, CLI, panel), verifikasi, uji manual.

Dependensi antar fitur hanya kompilasi dan semuanya terpenuhi oleh stub Foundation:
- `launch-contribution` memakai `ComposeInjection`, `WriteSnapshot`, `PruneSnapshots`, `TOMLString` (compose-snapshot) dan `RegisterInject`, `InjectDeps` (laws-measure-ipc).
- `laws-measure-ipc` memakai `ComposeInjection` (compose-snapshot) dan `Store` tiket 01.
- `compose-snapshot` memakai `Store` tiket 01 saja (membaca `laws.md` lewat `Store.ReadFile`, BUKAN `ReadLaws`, supaya tidak bergantung pada fitur lain).
- `laws-ui` tidak bergantung pada fitur Go lain saat kompilasi.

## 2. Kontrak bersama

### 2.1 Paket, folder, penamaan

- Modul Go `github.com/NooBeeID/shikigami`; paket `internal/grimoire` (package `grimoire`) sudah ada dari tiket 01. Berkas baru tiket ini: `inject.go`, `snapshot.go`, `laws.go`, `measure.go`.
- Folder data: `<DataDir>/grimoire/<projectId>/{index.md, laws.md, ...}` (layout tiket 01); snapshot di `<DataDir>/run/grimoire-<16 hex>.md`.
- Pesan galat dan komentar bahasa Indonesia; identifier Inggris. Logger `*slog.Logger` nullable (cek nil).
- Helper tak-diekspor diberi awalan berkas (`inject*`, `snapshot*`, `laws*`, `measure*`) agar tidak bentrok dengan berkas lain dalam paket (termasuk berkas tiket 01). Konstanta/helper didefinisikan hanya di berkas pemiliknya.
- Daemon: `RegisterModule("grimoire-inject", ...)` dari `init()` di `cmd/shikigamid/wire_grimoire_inject.go` (modul dipasang alfabetis: `grimoire` lalu `grimoire-inject`; layanan lain via `d.OnReady`/`d.Lookup`).

### 2.2 Koreksi atas tiket (WAJIB diikuti)

1. **Tidak ada `InjectionHeader` baru.** Tiket 01 sudah menaruh header aturan konstan (`IndexHeader`, memuat placeholder literal `{{grimoire_dir}}`) di dalam `index.md`. Maka teks suntikan = isi `index.md` (sudah berisi header) dengan placeholder diganti + bagian laws. Jangan menduplikasi header dan jangan mendefinisikan `InjectionHeader`.
2. **Nilai pengganti `{{grimoire_dir}}`** = `Store.Dir(projectID)` (`<DataDir>/grimoire/<projectId>`, absolut, tanpa slash akhir). Header tiket 01 menulis `{{grimoire_dir}}/books/<buku>/<bab>.md`, jadi `/books` sudah ada di header; tiket 02 yang menyebut `.../books` ditafsirkan sebagai path folder buku hasil akhirnya. Agent `compose-snapshot` WAJIB membuka `IndexHeader` nyata di `internal/grimoire/index.go` untuk memastikan; bila header nyata memakai `{{grimoire_dir}}` sebagai folder buku langsung, ganti dengan `Store.BooksDir(projectID)`. Hasil akhir harus berupa path absolut yang benar ke `books/<buku>/<bab>.md`.
3. **`Register` bentrok nama.** Tiket 01 sudah mendefinisikan `func Register(srv *ipc.Server, d Deps)` di `ipc.go`. Fungsi tiket ini diberi nama `RegisterInject` dengan tipe `InjectDeps` (bukan `Register`).
4. **`grimoire.status` tidak bisa dibungkus dari luar** (`ipc.Server.Register` panik bila method terdaftar dua kali dan `internal/ipc` tidak boleh disentuh). Maka `RegisterInject` HANYA mendaftarkan `grimoire.laws.get` dan `grimoire.laws.set`. Perluasan field status dilakukan di Wiring dengan satu blok kecil di handler status `internal/grimoire/ipc.go` milik tiket 01 yang memanggil `MeasureInjection` (lihat bagian 5, ini pengecualian yang dijelaskan di Wiring, setara "hook kecil" yang diizinkan tiket).
5. **`InjectionInfo` membaca status lewat `grimoire.status`** sendiri (via `grimoireStatus` dari `../client` milik tiket 01), karena store panel tiket 01 tidak menyimpan `indexBytes`/`estTokens`.
6. Store tiket 01 tidak di-`Provide` ke `d.Lookup`; `wire_grimoire_inject.go` membuat `grimoire.NewStore(d.DataDir)` sendiri.

### 2.3 Signature Go bersama (final; stub di Foundation, implementasi oleh pemilik)

Dari tiket 01 (JANGAN diubah): `Store` dan metode `Dir`, `ValidateProject`, `IndexPath`, `LawsPath`, `BooksDir`, `WriteFile` (atomik, `MkdirAll` Dir), `ReadFile` (batas 4 MiB; galat tak-ada memenuhi `errors.Is(err, os.ErrNotExist)`), `Exists`, `ReadState`; `NewStore(dataDir)`; `ErrInvalidProject`, `ErrInvalidPath`, `ErrNotBuilt`; `IndexHeader`; `Deps`/`Register` (ipc.go).

`inject.go` (milik `compose-snapshot`):
```go
func ComposeInjection(projectID string) (text string, ok bool, err error)
```
- Memakai `paths.DataDir()` lalu `NewStore(...)`. Membaca `index.md` lewat `Store.ReadFile`; tidak ada (`os.ErrNotExist`) -> `"", false, nil`. Project id tak valid -> `"", false, err`.
- `text = indexDenganPlaceholderDiganti + laws`, di mana bila `laws.md` ada dan `strings.TrimSpace(laws) != ""`: tambahkan konstanta pemisah tak-diekspor `injectLawsSeparator = "\n# Laws\n\n"` lalu isi `laws.md` apa adanya (tidak di-trim). Tanpa laws (tidak ada/kosong/hanya spasi) bagian itu dilewati sepenuhnya. Tanpa tanggal/waktu/nilai dinamis. Deterministik byte per byte.
- `laws.md` tak terbaca selain tak-ada -> galat dikembalikan (pemanggil mencatat dan tidak menyuntik).

`snapshot.go` (milik `compose-snapshot`):
```go
func WriteSnapshot(text string) (path string, err error)   // <DataDir>/run/grimoire-<16 hex>.md, 0700 dir, 0600 file + Chmod; mencatat path di daftar in-memory proses
func PruneSnapshots(inUse func(path string) bool) error    // lihat aturan di bawah
func TOMLString(s string) string                           // string TOML basic ter-quote
```
Aturan `PruneSnapshots` (satu-satunya definisi; hanya menghapus berkas bernama `grimoire-*.md` langsung di `<DataDir>/run/`, bukan symlink, bukan direktori): sebuah berkas dihapus bila (a) umurnya (mtime) > 24 jam, DAN (b) tidak dipakai: bila `inUse != nil` maka `inUse(path)` harus false; bila `inUse == nil` maka path tidak boleh ada di daftar in-memory `WriteSnapshot` proses ini. Snapshot muda (<= 24 jam) tidak pernah dihapus (melindungi snapshot yang baru ditulis tetapi sesinya belum terdaftar). Berkas yang gagal dihapus dilewati; galat pertama dikembalikan setelah semua dicoba; run/ tidak ada = nil. `TOMLString`: `"` + escape `\\`, `\"`, `\n`, `\r`, `\t`, `\b`, `\f`, karakter kontrol lain dan DEL sebagai `\u00XX`, UTF-8 valid dibiarkan apa adanya; UTF-8 tidak valid diganti `�` + `"`.

`laws.go` (milik `laws-measure-ipc`):
```go
const MaxLawsBytes = 16 * 1024
var ErrLawsTooLarge = errors.New("laws melebihi batas ukuran")
func (s *Store) ReadLaws(projectID string) (string, error)     // "" bila berkas tak ada
func (s *Store) WriteLaws(projectID, content string) error      // ErrLawsTooLarge bila len(content) > MaxLawsBytes; atomik lewat Store.WriteFile; isi apa adanya
```

`measure.go` (milik `laws-measure-ipc`):
```go
type Measure struct{ IndexBytes, EstTokens, Pages, LawsBytes int }
func MeasureInjection(projectID string) (Measure, error)       // IndexBytes=len(teks suntikan penuh); EstTokens=(len+3)/4; Pages=jumlah berkas .md di books/ ; LawsBytes=len(laws.md); tanpa grimoire: Measure{} , nil

type InjectDeps struct {
    Store  *Store
    Known  func(ctx context.Context, projectID string) (bool, error) // opsional; nil = lewati pengecekan "project dikenal"
    Logger *slog.Logger
}
func RegisterInject(srv *ipc.Server, d InjectDeps)             // mendaftarkan grimoire.laws.get dan grimoire.laws.set
```

### 2.4 IPC

Galat lewat `ipc.NewError`; kode `ErrInvalid -32003`, `ErrNotFound -32002`, `ErrInternal -32603` (`internal/ipc/errors.go`). Pola decode params seperti `internal/fsx/ipc.go` / tiket 01 (`invalid params: ...`).

| Method | Params | Hasil | Galat |
|---|---|---|---|
| `grimoire.laws.get` | `{projectId}` | `{content: string}` (`""` bila belum ada berkas) | `projectId` kosong, tak valid (`ErrInvalidProject`), atau `Known` mengembalikan false -> `ErrInvalid` |
| `grimoire.laws.set` | `{projectId, content}` | `{lawsBytes: int}` | sama; `ErrLawsTooLarge` -> `ErrInvalid` (pesan menyebut batas 16384 byte); lain -> `ErrInternal` |

Setelah `laws.set` sukses: `srv.Publish("grimoire.changed", map[string]any{"projectId": id})` (topik dan payload sama dengan tiket 01).

`grimoire.status {projectId}`: field aditif `indexBytes` (= ukuran teks suntikan penuh), `estTokens`, `pages`, `lawsBytes` dari `MeasureInjection` (diisi lewat blok Wiring di `ipc.go`); field lain dan artinya tidak berubah. Project tanpa grimoire: semuanya 0.

### 2.5 Argumen launch

- Claude: `ctx.Args = append(ctx.Args, "--append-system-prompt-file", snapshotPath)`; tidak ditambah bila `--append-system-prompt-file` sudah ada di `ctx.Args`.
- Codex: `ctx.Args = append(ctx.Args, "-c", "developer_instructions="+grimoire.TOMLString(text))`; tidak ditambah bila sudah ada argumen berawalan `developer_instructions=`. Teks yang sama dengan Claude; snapshot berkas tetap ditulis (audit).
- Provider `shell`/id lain dan project tanpa grimoire: tanpa perubahan. Provider sudah menambah flag izin lewat `withFlag`; contribution TIDAK menambah flag izin.
- Penentuan `projectId`: `ctx.Session.ProjectID`; bila kosong cocokkan `ctx.Session.Cwd` dengan `Project.Path` (sama dengan atau di dalam; pilih awalan terpanjang) lewat `workspace.list` (klien IPC malas + cache 5 detik, pola `workspaceDirectory` di `cmd/shikigamid/wire_mention.go`; kirim params `map[string]any{}`, bukan nil). Bentuk respons: array `Workspace` dengan `id`, `name`, `projects:[{id,path,name}]` (verifikasi tag JSON di `internal/workspace/types.go`).

### 2.6 Frontend (`frontend/src/modules/grimoire/laws/`)

Impor: `call`, `onEvent` dari `../../../lib/ipc`; `grimoireStatus` dan tipe `GrimoireStatus` dari `../client` (tiket 01). Svelte 5 runes, token tema yang ada (`--bg`, `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--danger`), tanpa token global baru, tanpa `setInterval`/polling.

`lawsStore.ts` (store per project aktif; tanpa efek saat impor modul):
```ts
export interface LawsInjectionInfo { exists: boolean; indexBytes: number; estTokens: number }
export const lawsViewOpen: Writable<boolean>                  // true saat LawsEditor tampil inline menggantikan daftar buku
export const injectionInfo: Readable<LawsInjectionInfo | null> // null bila belum dimuat/ tidak ada grimoire
export function startLawsStore(projectId: string): () => void  // berlangganan 'grimoire.changed' untuk projectId itu, memuat status; mengembalikan fungsi berhenti; idempoten per pemanggilan
export function getLaws(projectId: string): Promise<string>    // call('grimoire.laws.get', {projectId}) -> content
export function setLaws(projectId: string, content: string): Promise<number> // call('grimoire.laws.set', ...) -> lawsBytes
export function utf8Bytes(s: string): number                   // new TextEncoder().encode(s).length
export const MAX_LAWS_BYTES = 16 * 1024
```
Komponen (semua di folder `laws/`):
- `LawsEditor.svelte` props `{ projectId: string; onback: () => void }`: textarea monospace, penghitung byte `N / 16384`, tombol Simpan (nonaktif bila melebihi batas), indikator "tersimpan"/galat, catatan "Perubahan berlaku untuk sesi baru", tombol kembali (`onback`).
- `InjectionInfo.svelte` props `{ projectId: string }`: baris "Disuntik ke sesi baru: N byte, ~M token" dari `injectionInfo`; tidak merender apa pun bila belum ada grimoire.
- `LawsSlot.svelte` props `{ projectId: string | null; exists: boolean }`: komponen tunggal yang ditanam di slot panel. Bila `exists` false atau `projectId` null: kosong. Selain itu: bila `$lawsViewOpen` false render entri "Laws" (button) lalu `InjectionInfo`; bila true render `LawsEditor` (tombol kembali menutup). Menjalankan `startLawsStore(projectId)` lewat `$effect` dan menutup editor (`lawsViewOpen.set(false)`) saat `projectId` berganti atau unmount.
- Panel (Wiring) membaca `lawsViewOpen` untuk menyembunyikan daftar buku saat editor terbuka.

## 3. Daftar fitur

Empat fitur persis dari tiket (tidak ditambah/digabung). Slug dipakai sebagai nama berkas plan `<slug>.md`.

### compose-snapshot: Susun teks suntikan dan snapshot per sesi
Scope: `ComposeInjection` (index dengan placeholder diganti + laws, deterministik, `ok=false` tanpa index), `WriteSnapshot` (pola `writeMCPConfig`: `run/` 0700, `grimoire-<16 hex>.md` 0600 + Chmod, catat path in-memory), `PruneSnapshots` (aturan 2.3), `TOMLString`.
ownedPaths: `internal/grimoire/inject.go`, `internal/grimoire/snapshot.go`

### launch-contribution: Pasang LaunchContribution untuk claude dan codex
Scope: `cmd/shikigamid/wire_grimoire_inject.go`: modul `grimoire-inject`; di `d.OnReady`: buat `grimoire.NewStore(d.DataDir)`, panggil `grimoire.RegisterInject(srv, InjectDeps{...})` dengan `Known` dari direktori project (workspace.list), pasang `provider.RegisterLaunchContribution` sekali (`sync.Once`; pola `internal/mention/install.go`), jalankan `PruneSnapshots` awal dan setiap selesai menulis snapshot (gagal hanya dicatat; tulis snapshot dan prune tidak boleh menggagalkan peluncuran). Cek apakah `session.Manager` membuka cara memeriksa argumen/sesi hidup; bila tidak (kemungkinan besar), kirim `inUse=nil`. Logika contribution sesuai 2.5 dan tiket.
ownedPaths: `cmd/shikigamid/wire_grimoire_inject.go`

### laws-measure-ipc: Laws dan pengukuran ukuran index
Scope: `ReadLaws`/`WriteLaws`/`MaxLawsBytes`/`ErrLawsTooLarge`, `Measure`/`MeasureInjection`, `InjectDeps`/`RegisterInject` (`grimoire.laws.get/set`, publish `grimoire.changed`). Perluasan `grimoire.status` TIDAK dikerjakan fitur ini (Wiring).
ownedPaths: `internal/grimoire/laws.go`, `internal/grimoire/measure.go`

### laws-ui: Entri Laws dan info suntikan di panel
Scope: `lawsStore.ts`, `LawsEditor.svelte`, `InjectionInfo.svelte`, `LawsSlot.svelte` (2.6). Tidak menyentuh `GrimoirePanel.svelte` (Wiring).
ownedPaths: `frontend/src/modules/grimoire/laws/`

## 4. Foundation

Ada (hasFoundation = true); rinci di `00-foundation.md`. Isi singkat:
- Prasyarat: tiket 01 sudah ter-merge (`internal/grimoire/store.go` bukan stub, `index.go` punya `IndexHeader`, `ipc.go` punya `Register`). Bila tidak, berhenti dan laporkan.
- Stub Go berkompilasi (baris pertama `// STUB(foundation): ditimpa oleh <fitur>`) dengan signature final bagian 2.3 untuk `inject.go`, `snapshot.go`, `laws.go`, `measure.go` (termasuk `Measure`, `InjectDeps`, `MaxLawsBytes`, `ErrLawsTooLarge`).
- Bukan milik Foundation: apa pun di `cmd/`, frontend, berkas tiket 01.

## 5. Wiring checklist

Dikerjakan agen wiring setelah semua fitur selesai. Satu-satunya berkas bersama yang disentuh adalah tiga berkas tiket 01 berikut, masing-masing dengan perubahan sekecil mungkin dan aditif:

1. Tidak ada stub tersisa: `grep -rn "STUB(foundation)" internal/grimoire cmd frontend/src` kosong (hanya yang dari epic ini; stub tiket 01 sudah hilang).
2. **Status** (`internal/grimoire/ipc.go`, tiket 01): pada handler `grimoire.status`, setelah field lama terisi, panggil `MeasureInjection(projectID)`; bila tanpa galat, set `indexBytes`, `estTokens`, `pages`, `lawsBytes` dari hasilnya (menggantikan nilai sementara tiket 01; field lain tidak berubah, galat ukur hanya dicatat dan nilai lama dibiarkan). Catat perubahan di ringkasan wiring.
3. **Daemon**: pastikan `cmd/shikigamid/wire_grimoire_inject.go` mendaftarkan `RegisterModule("grimoire-inject", ...)` dari `init()`, memanggil `grimoire.RegisterInject`, memasang `RegisterLaunchContribution` tepat sekali, dan menjalankan prune awal. `modules.go`, `main.go`, `wire_grimoire.go`, `wire_workspace.go` tidak berubah. Tidak ada method `grimoire.laws.*` ganda dengan tiket 01 (server panik bila ganda).
4. **CLI** (`cmd/shikigamictl/cmd_grimoire.go`, tiket 01): `callAndPrint` mencetak JSON mentah sehingga field baru otomatis tampil. Untuk baris ringkas, ubah cabang `status` menjadi satu blok kecil: panggil `grimoire.status` ke `map[string]any`, `printJSON`, lalu cetak ke stderr `index: N byte, ~M token, P halaman, laws: L byte` dari `indexBytes`, `estTokens`, `pages`, `lawsBytes`. Tidak ada subcommand baru.
5. **Panel** (`frontend/src/modules/grimoire/GrimoirePanel.svelte`, tiket 01): (a) impor `LawsSlot` dari `./laws/LawsSlot.svelte` dan `lawsViewOpen` dari `./laws/lawsStore`; (b) ganti komentar `<!-- slot: laws (tiket 2) -->` dengan `<LawsSlot projectId={s.projectId} exists={s.exists} />` (gunakan nama variabel state nyata panel; pertahankan komentar `<!-- slot: stale/llm (tiket 3, 4) -->`); (c) bungkus badan daftar buku/status dengan `{#if !$lawsViewOpen} ... {/if}` agar editor menggantikan daftar saat terbuka. Daftar buku tidak dirombak.
6. Konsistensi: nama method `grimoire.laws.get/set` di `lawsStore.ts` identik dengan yang didaftarkan `RegisterInject`; `status` memuat `indexBytes/estTokens/pages/lawsBytes`; tidak ada perubahan di `internal/session`, `internal/provider`, `internal/ipc`, `go.mod`, `go.sum`, `frontend/package.json`/lockfile (`git diff --stat` untuk berkas itu kosong).
7. Jalankan build guard (bagian 6). Uji manual bila lingkungan memungkinkan: `make build`, jalankan aplikasi, Generate grimoire pada sebuah project, simpan laws dari panel, buat sesi `claude` dan `codex` baru; periksa argumen proses (`ps`) memuat `--append-system-prompt-file <path>` / `-c developer_instructions=...`, isi `<DataDir>/run/grimoire-*.md` (mode 0600) memuat index dan laws dengan path absolut terisi, sesi `shell` tanpa tambahan, `shikigamictl grimoire status --project <id>` menampilkan `indexBytes`/`estTokens`, dan baris info panel menampilkan angka yang sama setelah laws disimpan.

## 6. Build guard

Satu-satunya guard; tanpa unit test:
- `go build ./...` (opsional `go vet ./internal/grimoire/... ./cmd/...` bila cepat).
- Frontend: `cd frontend && npm run check && npm run build` (atau `make build-ui`). Penutup epic: `make build`.
- Tanpa dependency baru: `git diff --stat -- go.mod go.sum frontend/package.json frontend/package-lock.json` kosong.

Selama fase paralel: fitur Go menjalankan `go build ./...`; `laws-ui` menjalankan build frontend. Bila build gagal karena berkas fitur lain (bukan miliknya), jangan memperbaikinya; laporkan.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3) dan menulis planmu/progress sesuai instruksi workflow. Jangan menyentuh path fitur lain, berkas Foundation selain yang dimilikinya, berkas tiket 01, atau berkas wiring (`ipc.go`, `wire_grimoire.go`, `cmd_grimoire.go`, `GrimoirePanel.svelte`).
- Menimpa stub Foundation milik fiturmu wajib (hapus baris `// STUB(foundation)`); jangan mengubah signature bagian 2.3/2.6. Bila signature terasa salah, catat di plan dan tetap ikuti kontrak.
- Dilarang: `internal/session`, `internal/provider`, `internal/ipc`, `cmd/shikigamid/main.go`, `modules.go`, `wire_workspace.go`, `wire_grimoire.go`, `cmd/shikigamictl/main.go`, `registry.go`, `go.mod`/`go.sum`, `package.json`/lockfile, `AgentSidebar`.
- Tidak ada definisi ganda: `MaxLawsBytes`/`ErrLawsTooLarge` hanya di `laws.go`; `Measure`/`InjectDeps` hanya di `measure.go`; helper tak-diekspor memakai awalan berkas.
- Tidak membuat berkas tiruan di luar ownedPaths. Tidak mengubah `Args` yang sudah ada selain `append`; tidak menambah flag izin.
