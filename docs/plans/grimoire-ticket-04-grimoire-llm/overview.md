# Overview: grimoire-ticket-04-grimoire-llm

Tiket: `docs/tickets/grimoire-ticket-04-grimoire-llm.md`. Spec: `docs/superpowers/specs/2026-11-22-grimoire-design.md`.
Dokumen ini adalah sumber kebenaran BERSAMA untuk semua plan fitur. Bila ada perbedaan kecil dengan tiket, tiket menang kecuali dinyatakan "koreksi" di bagian 2.2. Tanpa dependency baru, tanpa unit test; guard hanya build.

Keadaan repo saat overview ditulis: `internal/grimoire/` (tiket 01: `types, store, scan, modules, hash, pages, index, build, ipc`) dan `frontend/src/modules/grimoire/` (`GrimoirePanel.svelte`, `client.ts`, `store.ts`, `openPage.ts`) sudah ada. Tiket 02 dan 03 baru berupa plan (`docs/plans/grimoire-ticket-02-grimoire-inject/`, `docs/plans/grimoire-ticket-03-grimoire-refresh/`). Tiket 04 `depends_on` tiket 03, jadi saat dieksekusi kode tiket 02/03 SUDAH ada (`measure.go`, `diff.go`, `incremental.go`, `trigger.go`, `wire_grimoire_inject.go`, `wire_grimoire_refresh.go`, `frontend/src/modules/grimoire/{laws,stale}/`). Setiap agent wajib membaca kode nyata tiket 01-03 dan memakai API-nya apa adanya. Nama/signature tiket 03 yang dikutip di bawah diambil dari `docs/plans/grimoire-ticket-03-grimoire-refresh/overview.md`; bila kode nyata berbeda, ikuti kode nyata, jaga arti, dan catat di plan.

## 1. Ringkasan epic dan urutan eksekusi

Hasil tiket 01-03 hanya kerangka statik. Epic ini menambah ringkasan LLM opt-in: per project ada `settings.json` `{llm, maxModules}`; dry-run `grimoire.estimate` menampilkan antrean modul dan estimasi token tanpa memanggil LLM; bila `llm=true` dan `claude` siap, `BuildIncremental` (tiket 03) memakai `Summarizer` berbasis `claude -p` HANYA untuk modul antrean (Added/Modified/Stale). Kegagalan modul = stale dan build lanjut; `claude` tidak ada/belum login = seluruh build jatuh ke statik. Biaya build terakhir dicatat dan ditampilkan. Panel punya toggle, pratinjau estimasi, input `maxModules`, progres per modul, dan baris biaya. Build LLM selalu di latar belakang (pemicu tiket 03 sudah melepas peluncuran sesi).

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): stub berkompilasi dengan signature final di empat berkas `internal/grimoire/{settings,estimate,summarize,prompt}.go`. Setelah selesai `go build ./...` lulus. Tidak ada foundation frontend.
2. **Fitur paralel** (3 fitur, bagian 3): `settings-estimate`, `llm-summarizer`, `llm-ui`. Tiap agent menimpa penuh berkas ownedPaths-nya; signature bagian 2.3 TIDAK berubah. `llm-ui` murni frontend (kontrak hanya IPC, boleh di-mock).
3. **Wiring** (bagian 5): satu baris hook di `incremental.go`, field aditif status di `ipc.go`, CLI, slot panel, verifikasi, uji manual.

## 2. Kontrak bersama

### 2.1 Paket, folder, penamaan

- Modul Go `github.com/NooBeeID/shikigami`; paket `internal/grimoire` (package `grimoire`). Berkas tiket ini: `settings.go`, `estimate.go`, `summarize.go`, `prompt.go` (di `internal/grimoire/`), `cmd/shikigamid/wire_grimoire_llm.go`, folder `frontend/src/modules/grimoire/llm/`.
- Pesan galat, log, dan komentar bahasa Indonesia; identifier bahasa Inggris. Logger `*slog.Logger` nullable (cek nil).
- Helper tak-diekspor diberi awalan berkas (`settings*`, `estimate*`, `summarize*`, `prompt*`; di package `main`: `llmWire*`) agar tidak bentrok dengan berkas lain (tiket 01-03, `wire_*.go`). Konstanta/helper didefinisikan hanya di berkas pemiliknya.
- Daemon: `RegisterModule("grimoire-llm", ...)` dari `init()` di `cmd/shikigamid/wire_grimoire_llm.go`. Modul dipasang alfabetis (`grimoire`, `grimoire-inject`, `grimoire-llm`, `grimoire-refresh`); layanan lain diambil lewat `d.OnReady` + `d.Lookup`, bukan saat `RegisterModule`. Resolver path project memakai `newGrimoireResolver(d.SocketPath)` dari `wire_grimoire.go` (satu paket `main`, hanya dipanggil, tidak diedit). Store dibuat sendiri `grimoire.NewStore(d.DataDir)` (tiket 01 tidak `Provide` store); bila `d.DataDir` kosong pakai `paths.DataDir()` seperti `wire_grimoire.go`.
- Fungsi paket-level yang tidak menerima Store (`LoadSettings`, `SaveSettings`, `LoadBuildCost`) memakai `paths.DataDir()` + `NewStore(...)`, pola yang sama dengan `ComposeInjection` tiket 02. Penulisan selalu lewat `Store.WriteFile` (atomik, 0600/0700, hanya di bawah `<DataDir>/grimoire/<projectId>/`); tidak ada penulisan di dalam repo proyek.
- Nama buku = nama modul snake_case = kunci `State.Modules` (konvensi tiket 01).

### 2.2 Koreksi atas tiket (WAJIB diikuti)

1. **`EstimateBuild` butuh path project.** Tiket menulis `EstimateBuild(projectId)`, tetapi antrean modul butuh `Scan(root)`. Signature final: `EstimateBuild(projectID, projectPath string) (Estimate, error)`. IPC `grimoire.estimate` menyelesaikan path lewat `LLMDeps.ResolvePath`.
2. **`lastBuildCost` disimpan di berkas pendamping**, bukan di `state.json`. `BuildIncremental` (tiket 03) menulis ulang `State` utuh dari struct tanpa field itu, sehingga field aditif akan hilang pada setiap build. Berkas: `<DataDir>/grimoire/<projectId>/lastbuild.json` berisi `BuildCost` (JSON). `Build` penuh tiket 01 tidak menghapusnya (hanya index/state/books).
3. **`Summarizer` hanya mengembalikan isi bab; penulisan oleh `BuildIncremental`.** Tiket menyebut Summarizer menulis ke `books/...` lewat `fsx`; kontrak tiket 03 (`Summarize(ctx, m, static) (map[string]string, error)`) membuat `BuildIncremental` yang menulis bab secara atomik lewat `Store.WriteFile`. Summarizer TIDAK menulis bab sendiri. Kunci map = nama bab snake (tanpa `.md`) dan HARUS persis nama bab statik modul itu (`RenderBooks([]Module{m})` memberi `Page.Path` `books/<buku>/<bab>.md`), karena `index.md` dibangun dari kerangka statik. Aturan pemetaan: bab bernama sama dengan buku (kasus umum, modul tidak dipecah) diganti ringkasan LLM; bila modul dipecah (tidak ada bab bernama buku), ringkasan menggantikan bab pertama (urut nama), bab lain tetap isi statik. Semua kunci statik selalu ada di map keluaran.
4. **Titik pasang Summarizer = satu baris di `incremental.go` (tiket 03).** Trigger/IPC tiket 03 memanggil `BuildIncremental` tanpa Summarizer, jadi tidak ada titik pilih. Wiring menambah SATU baris di awal `BuildIncremental` setelah validasi opts: `if opts.Summarizer == nil { opts.Summarizer = SummarizerFor(projectID, opts.ProjectPath) }`. `SummarizerFor` mengembalikan interface nil MURNI (bukan typed-nil) untuk mode statik. Bila kode nyata tiket 03 sudah punya hook pemilihan lain (variabel paket/opsi), pakai itu dan catat; jangan menambah hook kedua.
5. **Pencatatan biaya tanpa hook akhir build.** `BuildIncremental` tidak memanggil callback di akhir. Maka instance `Summarizer` (dibuat segar per pemanggilan `SummarizerFor`) menyimpan `lastbuild.json` setelah SETIAP modul selesai (total berjalan; panggilan terakhir = total akhir). `DurationMs` = sejak instance dibuat sampai modul terakhir selesai. Build tanpa satu pun panggilan LLM (antrean kosong, `llm=false`, claude tidak ada) tidak menulis/mengubah `lastbuild.json`.
6. **`grimoire.status` field lama bertipe berbeda.** `statusResult.LastBuildCost` di `ipc.go` tiket 01 adalah `int` (placeholder 0) dan `LLMEnabled bool` sudah ada. Wiring mengubah `LastBuildCost` menjadi `*BuildCost` (`json:"lastBuildCost"`, `null` bila belum ada) dan mengisi `LLMEnabled`. Ini perubahan tipe placeholder, bukan arti field yang dipakai UI; `client.ts` (`lastBuildCost: number`) TIDAK diedit, `llmStore.ts` memakai tipenya sendiri.
7. **`maxModules` terlewati.** Modul antrean ke-(N+1) dst: bila modul sudah ada di `state.json` -> `Summarize` mengembalikan `ErrLLMOverLimit` (tiket 03 menandainya `Stale=true`, bab lama dipertahankan, index `warning: stale`). Bila modul BARU (belum di state): kembalikan bab statik tanpa galat agar tetap ada di index (tiket 03 mengeluarkan modul baru yang gagal dari index). Keduanya dihitung `Skipped` di `BuildCost`, bukan `Failed`. Konsekuensi diterima: modul baru yang dilewati tetap statik sampai hash-nya berubah atau `shikigamictl grimoire build --full`.
8. **Progres.** Tiket meminta progres dari "data status/list". Ditambah field aditif `llmProgress` pada `grimoire.status` (bagian 2.4). UI hanya menampilkannya saat `building == true`; backend tidak perlu hook akhir untuk menghapusnya.

### 2.3 Signature Go bersama (final; stub di Foundation, implementasi oleh pemilik)

Dari tiket 01-03 (JANGAN diubah, hanya dipakai): `Store` (`NewStore`, `Dir`, `ValidateProject`, `StatePath`, `ReadState`, `WriteFile`, `ReadFile`, `Exists`), `Module{Name, Dir, Kind, Files, Symbols, Sources, Tag, Truncated}`, `FileInfo{Path,...}`, `Scan`, `GroupModules`, `HashModule`, `RenderBooks`, `RenderChapter`, `Snake`, `State`, `ModuleState{Hash, Sources, Stale}`, `Page{Path, Content}`, `ErrNotBuilt`, `ErrInvalidProject`; tiket 03: `Diff`, `ComputeDiff(scanned, hashes, st)`, `Diff.Pending()`, `HashAll(root, mods)`, `Summarizer` (interface), `IncrementalOptions`, `Result`, `Builder`; tiket 02 `measure.go` (heuristik token `(len+3)/4`).

`settings.go` (milik `settings-estimate`):

```go
type Settings struct {
    LLM        bool `json:"llm"`
    MaxModules *int `json:"maxModules"` // nil = tanpa batas
}
var ErrInvalidMaxModules = errors.New("maxModules harus integer >= 1")
func LoadSettings(projectID string) (Settings, error)         // berkas tak ada = Settings{} (llm=false, nil); JSON rusak = galat; MaxModules < 1 di berkas dianggap nil
func SaveSettings(projectID string, s Settings) error         // ErrInvalidMaxModules bila *MaxModules < 1; atomik lewat Store.WriteFile("settings.json")

// LLMDeps dipakai bersama (didefinisikan di settings.go).
type LLMDeps struct {
    Store       *Store
    ResolvePath func(ctx context.Context, projectID string) (string, error)
    Publish     func(projectID string) // nullable; publikasi grimoire.changed {projectId}
    Logger      *slog.Logger
}
func RegisterLLM(srv *ipc.Server, d LLMDeps) // mendaftarkan grimoire.settings.get, grimoire.settings.set, grimoire.estimate; set sukses publish lewat srv.Publish("grimoire.changed", map[string]any{"projectId": id})
```

`estimate.go` (milik `settings-estimate`):

```go
type Estimate struct {
    Modules   int `json:"modules"`
    EstTokens int `json:"estTokens"`
    OverLimit int `json:"overLimit,omitempty"` // aditif: modul antrean di atas maxModules (tidak dihitung di Modules/EstTokens)
}
func EstimateBuild(projectID, projectPath string) (Estimate, error) // baca-saja; tanpa claude; tanpa menulis apa pun
func EstTokensFromBytes(n int) int                                  // = (n+3)/4; SATU-SATUNYA heuristik; bila measure.go (tiket 02) sudah mengekspor helper setara, panggil itu dan jadikan ini pembungkus tipis
```

Algoritma `EstimateBuild`: `Scan(root)` -> `GroupModules` -> `HashAll` -> `ReadState` (`ErrNotBuilt` = state nil) -> `ComputeDiff` -> antrean = `Diff.Pending()` (Added, Modified, Stale; urutan itu) -> bila `LoadSettings` memberi `MaxModules=N`, ambil N pertama, sisanya `OverLimit`. Token per modul antrean = `EstTokensFromBytes( min(jumlah byte sumber modul, LLMMaxSourceBytes) + LLMPromptOverheadBytes() + LLMExpectedOutputBytes )`; jumlah byte sumber dari ukuran berkas (`os.Stat` path `projectPath/<FileInfo.Path>`, gagal stat = 0). Project tanpa grimoire = semua modul Added. Setelah build penuh tanpa perubahan `Modules == 0`.

`prompt.go` (milik `llm-summarizer`):

```go
const LLMMaxSourceBytes = 48 * 1024     // batas isi sumber per modul pada prompt; lebih = dipotong dengan penanda eksplisit
const LLMExpectedOutputBytes = 3 * 1024 // perkiraan keluaran per modul (untuk estimasi)
func LLMPromptOverheadBytes() int       // len(template prompt tetap) tanpa isi modul
func promptBuild(m Module, static string, sources []promptSource) string // tak diekspor; isi bebas selama kontrak 2.2.3 dan aturan baris pertama "Sumber:"
```

`summarize.go` (milik `llm-summarizer`):

```go
type BuildCost struct {
    Modules    int    `json:"modules"`
    EstTokens  int    `json:"estTokens"`
    DurationMs int64  `json:"durationMs"`
    Failed     int    `json:"failed"`
    At         string `json:"at"`                // RFC3339 UTC
    Skipped    int    `json:"skipped,omitempty"` // aditif: dilewati karena maxModules
    Note       string `json:"note,omitempty"`    // aditif: mis. "claude tidak terpasang: mode statik" / "belum login: mode statik"
}
type LLMProgress struct {
    Current string `json:"current"` // nama buku yang sedang diringkas, "" bila antar modul
    Done    int    `json:"done"`    // modul selesai diringkas pada build ini
    Failed  int    `json:"failed"`
}
var ErrLLMOverLimit = errors.New("modul di atas maxModules")

func LoadBuildCost(projectID string) (*BuildCost, error)     // nil, nil bila lastbuild.json tak ada
func ProgressFor(projectID string) *LLMProgress              // nil bila tak ada instance aktif; salinan, aman konkuren
func SetLLMRuntime(d LLMDeps)                                // menyimpan runtime (atomic) untuk SummarizerFor: Store, Publish, Logger
func SummarizerFor(projectID, projectPath string) Summarizer // nil MURNI bila runtime belum diset, LoadSettings.LLM=false, atau claude tidak ada di PATH
func NewLLMSummarizer(deps LLMDeps, projectID, projectPath string, s Settings) Summarizer // memenuhi Summarizer; satu instance = satu build
```

Perilaku `Summarize` per modul (instance dibuat segar per build, mutex internal):
1. Hitung `static` pages dari `RenderBooks([]Module{m})`. Antrean ke-(N+1) dst (N = `Settings.MaxModules`): lihat 2.2.7.
2. Latch `unavailable`: bila `true`, kembalikan bab statik tanpa galat (tanpa menjalankan claude).
3. Jalankan `claude -p` (lihat 2.6) dengan timeout per modul (`summarizeModuleTimeout`, 120 detik, via `context.WithTimeout` dari `ctx`). Kegagalan autentikasi/biner (indikasi: exit non-nol sebelum ada keluaran berhasil pertama dan stderr memuat kata login/auth/credential/API key, atau `exec` error) -> set latch `unavailable`, `Note` diisi, bab statik, tanpa galat. Timeout, exit non-nol lain, keluaran kosong -> kembalikan galat (tiket 03 menandai stale), `Failed++`.
4. Validasi keluaran: baris pertama (setelah trim awal) harus berawalan `Sumber:`; bila tidak, sisipkan baris `Sumber: <ModuleState.Sources atau m.Sources dipisah ", ">` di atas keluaran. Tanpa tanggal/changelog. Bab ditulis ulang utuh.
5. Setelah tiap modul (sukses, gagal, atau dilewati): perbarui progres, simpan `lastbuild.json` (2.2.5), panggil `Publish(projectID)` (event `grimoire.changed`). Juga publish saat modul mulai (`Current` terisi) agar UI menampilkan modul berjalan.
6. `EstTokens` kumulatif = `EstTokensFromBytes(byte prompt terkirim + byte keluaran)` per modul yang dipanggil (perkiraan, bukan tagihan).

`wire_grimoire_llm.go` (milik `llm-summarizer`, package main, tanpa API ekspor): di `d.OnReady`: buat `grimoire.NewStore`, bentuk `grimoire.LLMDeps{Store, ResolvePath: newGrimoireResolver(d.SocketPath), Publish: func(id string){ srv.Publish("grimoire.changed", map[string]any{"projectId": id}) }, Logger: d.Logger}`, panggil `grimoire.RegisterLLM(srv, deps)` dan `grimoire.SetLLMRuntime(deps)`. Tidak ada `Provide`/`LaunchContribution`.

### 2.4 IPC (aditif; arti field lama tidak berubah)

Galat lewat `ipc.NewError` (`ErrInvalid`, `ErrNotFound`, `ErrInternal`), pola decode `invalid params: ...` seperti `ipc.go` tiket 01. `projectId` kosong/tak valid -> `ErrInvalid`.

| Method | Params | Hasil | Catatan |
|---|---|---|---|
| `grimoire.settings.get` | `{projectId}` | `{llm:boolean, maxModules:int\|null}` | tanpa berkas = `{llm:false, maxModules:null}`; tidak menulis berkas |
| `grimoire.settings.set` | `{projectId, llm?, maxModules?}` | settings terbaru (bentuk sama) | field tak dikirim tak diubah; `maxModules: null` eksplisit menghapus batas (bedakan "tidak ada" vs `null` dengan `json.RawMessage`); `maxModules` bukan integer atau < 1 -> `ErrInvalid`; sukses -> tulis `settings.json`, publish `grimoire.changed {projectId}` |
| `grimoire.estimate` | `{projectId}` | `{modules, estTokens, overLimit?}` | tanpa LLM, tanpa menulis; path via `LLMDeps.ResolvePath` (galat dipetakan seperti `rpcMapErr`: project tak dikenal -> `ErrNotFound`) |
| `grimoire.status` (tambahan Wiring) | `{projectId}` | + `llmEnabled:boolean`, `lastBuildCost:{modules,estTokens,durationMs,failed,at,skipped?,note?}\|null`, `llmProgress:{current,done,failed}\|null` | `llmEnabled` dari `LoadSettings` (galat = false); `lastBuildCost` dari `LoadBuildCost`; `llmProgress` dari `ProgressFor` |

Event `grimoire.changed {projectId}` terbit juga saat settings berubah dan di awal/akhir tiap modul yang diringkas (oleh summarizer lewat `LLMDeps.Publish`). Tidak ada event baru.

### 2.5 CLI

`shikigamictl grimoire status [--project <id>] [--dry-run]`: dengan `--dry-run` memanggil `grimoire.estimate {projectId}`, mencetak JSON (via `callAndPrint`) dan ke stderr `estimasi: N modul, ~T token`. `--dry-run` hanya sah untuk `status`; pada `build` -> pakai usage. Tanpa `--dry-run`, `status` mencetak JSON yang kini memuat `llmEnabled`/`lastBuildCost`, dan ke stderr ditambah satu baris `llm: aktif|mati` serta, bila `lastBuildCost` ada, `build terakhir: M modul, ~T token, D ms, gagal F`. Tidak ada subcommand baru; tambahan stderr tiket 02/03 dipertahankan.

### 2.6 Menjalankan `claude -p` (aturan untuk llm-summarizer)

- Cari biner dengan `exec.LookPath("claude")` memakai PATH proses daemon (modul `loginenv` sudah mengisi `PATH` via `os.Setenv`); `cmd.Env = os.Environ()`, `cmd.Dir = projectPath`.
- Prompt dikirim lewat stdin; keluaran dibaca dari stdout (`--output-format text`). Model murah sebagai default lewat konstanta `summarizeDefaultModel` (mis. `haiku`), dapat diganti env `SHIKIGAMI_GRIMOIRE_MODEL` (didokumentasikan di komentar kode). Verifikasi flag nyata dengan `claude -p --help` di mesin; hanya flag yang diperlukan mode `-p`; DILARANG `--dangerously-skip-permissions` dan padanannya (peringkas tidak butuh tool: nonaktifkan tool bila ada flag-nya).
- `exec.CommandContext` dengan context ber-timeout; proses dimatikan hanya lewat context/PID miliknya sendiri (jangan `pkill`, jangan mencari proses lain).
- Batas ukuran stdout/stderr yang dibaca (mis. 1 MiB) agar proses nakal tidak menghabiskan memori. Isi sumber diambil dari `projectPath/<path>` per `m.Sources` (urut), melewati biner/besar, dipotong total ke `LLMMaxSourceBytes` dengan penanda `[...dipotong...]`.
- Tidak ada penulisan apa pun di dalam repo proyek oleh peringkas.

### 2.7 Frontend (`frontend/src/modules/grimoire/llm/`)

Impor: `call`, `onEvent` dari `../../../lib/ipc`; `activeProjectId` dari `../../../lib/stores` (bila nama aktual berbeda, ikuti `GrimoirePanel.svelte`/`store.ts` tiket 01). TIDAK mengedit `client.ts`: `llmStore.ts` memanggil `call('grimoire.settings.get'|'grimoire.settings.set'|'grimoire.estimate'|'grimoire.status', {...})` langsung dengan tipe sendiri. Svelte 5 runes, token tema yang ada (`--bg`, `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--danger`; pakai yang sudah dipakai `GrimoirePanel.svelte`/`laws/`/`stale/`), tanpa token global baru, tanpa `setInterval`/`setTimeout`/polling, tanpa warna keras.

```ts
// llmStore.ts (tanpa efek saat impor modul)
export interface LlmCost { modules: number; estTokens: number; durationMs: number; failed: number; at: string; skipped?: number; note?: string }
export interface LlmProgress { current: string; done: number; failed: number }
export interface LlmEstimate { modules: number; estTokens: number; overLimit?: number }
export interface LlmState {
  projectId: string | null; loaded: boolean;
  llm: boolean; maxModules: number | null;
  building: boolean; progress: LlmProgress | null; lastBuildCost: LlmCost | null;
  estimate: LlmEstimate | null;   // pratinjau konfirmasi; non-null = menunggu konfirmasi
  busy: boolean; error: string | null;
}
export const llmState: Readable<LlmState>
export function startLlmStore(projectId: string): () => void  // muat awal (settings.get + status); langganan 'grimoire.changed' hanya untuk projectId itu (memuat ulang status); kembalikan fungsi berhenti; project lain diabaikan; field baru undefined = llm false, progress null, lastBuildCost null
export function requestEnable(): Promise<void>   // call('grimoire.estimate') -> state.estimate (belum menyimpan flag)
export function confirmEnable(): Promise<void>   // call('grimoire.settings.set', {projectId, llm: true}) lalu estimate = null
export function cancelEnable(): void             // estimate = null
export function disableLlm(): Promise<void>      // settings.set {llm:false}, tanpa konfirmasi
export function setMaxModules(v: number | null): Promise<void> // null = hapus batas (kirim maxModules:null); v < 1 atau bukan integer -> state.error, tidak memanggil IPC
export function formatTokens(n: number): string; export function formatDuration(ms: number): string
```

Komponen (folder `llm/`):
- `LlmSettings.svelte` props `{ projectId: string | null; exists: boolean }`: komponen tunggal yang ditanam panel. Kosong bila `projectId` null. Menjalankan `startLlmStore(projectId)` lewat `$effect` (berhenti saat project berganti/unmount). Merender: toggle "Ringkasan LLM" (mati = `requestEnable`, hidup = `disableLlm`), pratinjau "N modul, ~T token" dengan tombol Konfirmasi/Batal, input angka `maxModules` (kosong = tanpa batas), lalu `LlmProgress` dan `LlmCost`. Tidak bergantung pada `exists` untuk toggle (settings bisa diatur sebelum Generate); progres/biaya hanya bila ada datanya.
- `LlmProgress.svelte` props `{ building: boolean; progress: LlmProgress | null }`: "Meringkas <current> (N selesai)" hanya saat `building` dan `progress` non-null; kosong selain itu.
- `LlmCost.svelte` props `{ cost: LlmCost | null }`: baris biaya (modul diringkas, token perkiraan, durasi, gagal, dilewati bila ada) dan `note` (mode statik) bila ada; kosong bila `cost` null.

## 3. Daftar fitur

Tiga fitur persis dari tiket (tidak ditambah/digabung). Slug dipakai sebagai nama berkas plan `<slug>.md`.

### settings-estimate: Settings project dan dry-run estimasi
Scope: `Settings`/`LoadSettings`/`SaveSettings`/`ErrInvalidMaxModules`/`LLMDeps`/`RegisterLLM` (`grimoire.settings.get/set`, `grimoire.estimate`, publish saat set), `Estimate`/`EstimateBuild`/`EstTokensFromBytes` (bagian 2.3). Perluasan CLI `status --dry-run` dan baris `llmEnabled`/biaya (2.5).
ownedPaths: `internal/grimoire/settings.go`, `internal/grimoire/estimate.go`, (aditif, disebut di Wiring butir 5) `cmd/shikigamictl/cmd_grimoire.go`

Catatan: tiket memasukkan `cmd_grimoire.go` ke ownedPaths fitur ini secara aditif. Agen `settings-estimate` BOLEH mengedit berkas itu (hanya menambah flag `--dry-run` dan baris stderr, 2.5). Agen wiring hanya memverifikasi dan tidak mengedit ulang kecuali ada yang kurang.

### llm-summarizer: Summarizer claude -p
Scope: `BuildCost`/`LLMProgress`/`ErrLLMOverLimit`/`LoadBuildCost`/`ProgressFor`/`SetLLMRuntime`/`SummarizerFor`/`NewLLMSummarizer` dan seluruh perilaku 2.3 `Summarize`, prompt tetap dan konstanta di `prompt.go`, modul daemon `grimoire-llm` (`wire_grimoire_llm.go`) yang memanggil `RegisterLLM` dan `SetLLMRuntime`.
ownedPaths: `internal/grimoire/summarize.go`, `internal/grimoire/prompt.go`, `cmd/shikigamid/wire_grimoire_llm.go`

### llm-ui: Toggle LLM, estimasi, dan biaya di panel
Scope: `llmStore.ts`, `LlmSettings.svelte`, `LlmProgress.svelte`, `LlmCost.svelte` (bagian 2.7). Tidak menyentuh `GrimoirePanel.svelte` (Wiring).
ownedPaths: `frontend/src/modules/grimoire/llm/`

Dependensi kompilasi: `llm-summarizer` memakai `LoadSettings`/`Settings`/`LLMDeps`/`EstTokensFromBytes` (stub Foundation cukup), `LLMMaxSourceBytes` dkk milik berkasnya sendiri. `settings-estimate` memakai `LLMMaxSourceBytes`, `LLMExpectedOutputBytes`, `LLMPromptOverheadBytes` dari stub `prompt.go` Foundation. `llm-ui` tidak bergantung pada fitur Go.

## 4. Foundation

Ada (hasFoundation = true); rinci di `00-foundation.md`. Isi singkat:
- Prasyarat: tiket 01-03 sudah ter-merge (`incremental.go` bukan stub, memuat `Summarizer`, `BuildIncremental`; `diff.go` memuat `ComputeDiff`, `Diff.Pending`; `measure.go` ada). Bila tidak, berhenti dan laporkan.
- Stub Go berkompilasi (baris pertama `// STUB(foundation): ditimpa oleh <fitur>`) dengan signature final bagian 2.3 di `settings.go`, `estimate.go`, `summarize.go`, `prompt.go`, termasuk semua tipe/konstanta/sentinel.
- Bukan milik Foundation: `wire_grimoire_llm.go`, `cmd_grimoire.go`, frontend, dan berkas tiket 01-03.

## 5. Wiring checklist

Dikerjakan agen wiring setelah semua fitur selesai. Perubahan pada berkas tiket lain seminimal mungkin, aditif, dan dicatat di ringkasan/deskripsi commit.

1. Tidak ada stub tersisa: `grep -rn "STUB(foundation)" internal/grimoire cmd frontend/src` kosong dari penanda epic ini (stub tiket sebelumnya sudah hilang).
2. **Hook Summarizer** (`internal/grimoire/incremental.go`, tiket 03): tambah SATU baris di `BuildIncremental` setelah validasi `opts`: `if opts.Summarizer == nil { opts.Summarizer = SummarizerFor(projectID, opts.ProjectPath) }` (lihat koreksi 2.2.4; bila ada hook lain di kode nyata, pakai itu). Pastikan jalur `Full` juga lewat sini. Catat di deskripsi commit.
3. **Daemon**: `cmd/shikigamid/wire_grimoire_llm.go` memanggil `RegisterModule("grimoire-llm", ...)` dari `init()` (nama unik); di `d.OnReady` memanggil `grimoire.RegisterLLM(srv, deps)` tepat sekali dan `grimoire.SetLLMRuntime(deps)`. `modules.go`, `main.go`, `wire_grimoire.go`, `wire_grimoire_inject.go`, `wire_grimoire_refresh.go`, `wire_workspace.go` tidak berubah. Tidak ada method `grimoire.settings.*`/`grimoire.estimate` ganda (server panik bila ganda).
4. **Status** (`internal/grimoire/ipc.go`): ubah `statusResult.LastBuildCost` dari `int` ke `*BuildCost` (`json:"lastBuildCost"`), tambah `LLMProgress *LLMProgress` (`json:"llmProgress"`); pada handler `grimoire.status`, setelah field lama terisi (blok tiket 02/03 dipertahankan): `LoadSettings` -> `LLMEnabled` (galat = false, dicatat), `LoadBuildCost` -> `LastBuildCost` (galat/nil = nil), `ProgressFor(id)` -> `LLMProgress`. Hanya menambah; tidak mengubah field lain.
5. **CLI** (`cmd/shikigamictl/cmd_grimoire.go`): verifikasi pekerjaan `settings-estimate` (flag `--dry-run` hanya pada `status`, memanggil `grimoire.estimate`; baris stderr `estimasi:` dan `llm:`/`build terakhir:` sesuai 2.5; subcommand lain tak berubah; usage diperbarui).
6. **Panel** (`frontend/src/modules/grimoire/GrimoirePanel.svelte`): impor `./llm/LlmSettings.svelte`; ganti komentar `<!-- slot: llm (tiket 4) -->` (sisa dari wiring tiket 03; bila masih berbentuk `<!-- slot: stale/llm (tiket 3, 4) -->`, pecah menjadi slot stale + llm) dengan `<LlmSettings projectId={s.projectId} exists={s.exists} />` (nama variabel state nyata panel). Satu impor dan satu elemen; logika panel, slot laws/stale, tombol Generate tidak diubah.
7. Konsistensi: nama method/field IPC di `llmStore.ts` (`grimoire.settings.get/set`, `grimoire.estimate`, `grimoire.status`, `llm`, `maxModules`, `modules`, `estTokens`, `overLimit`, `llmEnabled`, `lastBuildCost`, `llmProgress`) identik dengan yang dikirim handler; `SummarizerFor` mengembalikan nil murni untuk statik; tidak ada `time.Ticker`/`setInterval`/watcher baru (`grep -rnE "time\.(NewTicker|Tick)|setInterval|fsnotify" internal/grimoire cmd/shikigamid/wire_grimoire_llm.go frontend/src/modules/grimoire/llm` kosong); `claude` hanya dieksekusi dari `summarize.go` (`grep -rn '"claude"' internal/grimoire` hanya di sana); tidak ada perubahan di `internal/session`, `internal/provider`, `internal/ipc`, `go.mod`, `go.sum`, `frontend/package.json`/lockfile (`git diff --stat` untuk berkas itu kosong).
8. Periksa batas waktu IPC: `grimoire.build` sinkron (tiket 03 `Trigger.Run`) bisa berjalan menit bila `llm=true`. Pastikan klien (`frontend/src/lib/ipc.ts`/jembatan Wails, `internal/ipc` client) tidak memutus panggilan lebih awal; bila ada timeout tetap, laporkan (jangan ubah `internal/ipc`) dan pastikan UI tetap benar karena status akhir diambil dari event `grimoire.changed`.
9. Jalankan build guard (bagian 6). Uji manual bila lingkungan memungkinkan: `make build`, jalankan aplikasi; nyalakan toggle (pratinjau estimasi N modul/~T token, Konfirmasi baru menyimpan; cek `<DataDir>/grimoire/<id>/settings.json`), isi `maxModules`, edit satu-dua file modul, jalankan Generate/Perbarui; pastikan bab berubah berisi ringkasan dengan `Sumber:` di baris pertama, progres tampil lalu hilang, baris biaya muncul (`lastbuild.json`), modul di atas batas/gagal `warning: stale` di index dan panel; `shikigamictl grimoire status --dry-run` dan `status` menampilkan estimasi/`llmEnabled`/biaya; buka sesi baru (peluncuran tidak tertunda, sesi pertama memakai index statik); ubah `PATH` tanpa `claude` untuk memeriksa fallback statik tanpa error.

## 6. Build guard

Satu-satunya guard; tanpa unit test:
- `go build ./...` (opsional `go vet ./internal/grimoire/... ./cmd/...` bila cepat).
- Frontend: `cd frontend && npm run check && npm run build`. Penutup epic: `make build`.
- Tanpa dependency baru: `git diff --stat -- go.mod go.sum frontend/package.json frontend/package-lock.json` kosong.

Selama fase paralel: fitur Go menjalankan `go build ./...`; `llm-ui` menjalankan build/`check` frontend. Bila build gagal karena berkas fitur lain (bukan miliknya), jangan memperbaikinya; laporkan.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3) dan menulis plan/progress sesuai instruksi workflow. Jangan menyentuh path fitur lain, berkas Foundation selain yang dimilikinya, berkas tiket 01-03, atau berkas wiring (`incremental.go`, `ipc.go`, `wire_grimoire.go`, `wire_grimoire_inject.go`, `wire_grimoire_refresh.go`, `GrimoirePanel.svelte`, `client.ts`). Satu-satunya pengecualian: `settings-estimate` mengedit `cmd/shikigamictl/cmd_grimoire.go` secara aditif.
- Menimpa stub Foundation milik fiturmu wajib (hapus baris `// STUB(foundation)`); jangan mengubah signature bagian 2.3/2.7. Bila signature terasa salah, catat di plan dan tetap ikuti kontrak.
- Dilarang: `internal/session`, `internal/provider`, `internal/ipc`, `cmd/shikigamid/main.go`, `modules.go`, `wire_workspace.go`, `cmd/shikigamictl/main.go`, `registry.go`, `go.mod`/`go.sum`, `package.json`/lockfile, `AgentSidebar`, inti `Build`/`Scan`/`Store`/format bab/index/`state.json` tiket 01-03, `ComposeInjection`/snapshot tiket 02, `Trigger`/`BuildIncremental` tiket 03 (kecuali satu baris Wiring butir 2).
- Tidak ada definisi ganda: `Settings`/`LLMDeps`/`RegisterLLM` hanya di `settings.go`; `Estimate`/`EstimateBuild`/`EstTokensFromBytes` hanya di `estimate.go`; `BuildCost`/`LLMProgress`/`ErrLLMOverLimit`/`SummarizerFor`/`NewLLMSummarizer`/`SetLLMRuntime`/`ProgressFor`/`LoadBuildCost` hanya di `summarize.go`; konstanta prompt hanya di `prompt.go`; helper tak-diekspor memakai awalan berkas.
- Larangan perilaku: tanpa polling/timer/watcher; `claude` tidak pernah dijalankan saat `llm=false` atau di dalam `EstimateBuild`/IPC settings/estimate; peringkas tidak menunggu/memblokir peluncuran sesi dan tidak menambah kunci build baru (single-flight milik `Trigger` tiket 03); tidak menulis apa pun di dalam repo proyek; hanya mematikan proses `claude` milik sendiri (context/PID), tanpa `pkill` pola luas; kegagalan LLM tidak pernah membuat build gagal fatal.
- Tidak membuat berkas tiruan di luar ownedPaths.
