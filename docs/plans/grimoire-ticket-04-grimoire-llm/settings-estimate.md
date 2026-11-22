# Plan: Settings project dan dry-run estimasi

## Metadata

- Tiket: `docs/tickets/grimoire-ticket-04-grimoire-llm.md` (`grimoire-ticket-04-grimoire-llm`)
- Fitur: `settings-estimate` (Settings project dan dry-run estimasi)
- ownedPaths:
  - `internal/grimoire/settings.go`
  - `internal/grimoire/estimate.go`
  - `cmd/shikigamictl/cmd_grimoire.go` (hanya tambahan aditif, lihat Langkah 4)
- Depends on foundation: **ya**. `00-foundation.md` sudah membuat stub `settings.go`, `estimate.go`, `summarize.go`, `prompt.go` (baris pertama `// STUB(foundation): ditimpa oleh ...`). Plan ini MENIMPA PENUH `settings.go` dan `estimate.go` (hapus baris STUB, signature tidak berubah). `prompt.go` dan `summarize.go` milik fitur `llm-summarizer`; hanya dipakai.
- Sumber kebenaran kontrak: `docs/plans/grimoire-ticket-04-grimoire-llm/overview.md` bagian 2.2 (koreksi), 2.3 (signature), 2.4 (IPC), 2.5 (CLI). Jangan memodifikasi overview.

## Tujuan

1. Settings per project `{llm, maxModules}` di `<DataDir>/grimoire/<projectId>/settings.json` (default `llm=false`, `maxModules=nil`), dibaca/ditulis atomik lewat `Store.WriteFile`.
2. Tiga method IPC `grimoire.settings.get`, `grimoire.settings.set`, `grimoire.estimate` lewat `RegisterLLM`.
3. Dry-run estimasi (`EstimateBuild`): jumlah modul antrean dan estimasi token, baca-saja, tanpa `claude`, tanpa menulis apa pun.
4. CLI: `shikigamictl grimoire status --dry-run` dan baris stderr tambahan `llm:` / `build terakhir:` pada `status`.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `internal/grimoire/settings.go`
- `internal/grimoire/estimate.go`
- `cmd/shikigamictl/cmd_grimoire.go` (aditif: flag `--dry-run`, baris stderr, usage)

DILARANG menyentuh: `summarize.go`, `prompt.go`, `cmd/shikigamid/*` (termasuk `wire_grimoire_llm.go`), `frontend/*`, `incremental.go`, `ipc.go`, `diff.go`, `measure.go`, `store.go`, `internal/ipc`, `go.mod`/`go.sum`, serta berkas wiring lain. Jangan menyambungkan `RegisterLLM` ke daemon sendiri; jangan mengubah `statusResult` di `ipc.go` (itu Wiring).

## Kontrak

Disediakan (signature final, JANGAN diubah; lihat overview 2.3):

```go
// settings.go
type Settings struct {
    LLM        bool `json:"llm"`
    MaxModules *int `json:"maxModules"`
}
var ErrInvalidMaxModules = errors.New("maxModules harus integer >= 1")
func LoadSettings(projectID string) (Settings, error)
func SaveSettings(projectID string, s Settings) error
type LLMDeps struct {
    Store       *Store
    ResolvePath func(ctx context.Context, projectID string) (string, error)
    Publish     func(projectID string)
    Logger      *slog.Logger
}
func RegisterLLM(srv *ipc.Server, d LLMDeps)

// estimate.go
type Estimate struct {
    Modules   int `json:"modules"`
    EstTokens int `json:"estTokens"`
    OverLimit int `json:"overLimit,omitempty"`
}
func EstimateBuild(projectID, projectPath string) (Estimate, error)
func EstTokensFromBytes(n int) int
```

IPC (overview 2.4): `grimoire.settings.get {projectId}` -> `{llm, maxModules|null}`; `grimoire.settings.set {projectId, llm?, maxModules?}` -> settings terbaru; `grimoire.estimate {projectId}` -> `{modules, estTokens, overLimit?}`. Galat lewat `ipc.NewError` (`ErrInvalid`, `ErrNotFound`, `ErrInternal`). Event `grimoire.changed {projectId}` dipublish setelah `set` sukses.

Dipakai (dari kode nyata yang sudah ada; verifikasi signature dengan membaca berkasnya sebelum menulis):

- Tiket 01: `Store` (`NewStore`, `ValidateProject`, `ReadFile`, `WriteFile`, `ReadState`), `ErrNotBuilt`, `ErrInvalidProject`, `ErrInvalidPath`, `Scan(root) ([]FileInfo, error)`, `GroupModules([]FileInfo) []Module`, `Module{Name, Sources, Files, ...}`, `FileInfo{Path, Size, ...}`, `Snake`. Dari `ipc.go`: helper tak-diekspor `rpcDecode(params, &v)` dan `rpcMapErr(err)` (satu paket; boleh dipanggil, tidak diedit).
- Tiket 03: `ComputeDiff(scanned []Module, hashes map[string]string, st *State) Diff`, `Diff.Pending() []ModuleChange` (urutan Added, Modified, Stale; `ModuleChange.Name` = nama snake), `HashAll(root, mods)`, dan helper tak-diekspor `incrementalRoot(projectPath)` (validasi path absolut direktori) serta `incrementalScan(store, projectID, root) ([]Module, map[string]string, *State, error)` (scan + group + hash + ReadState, `ErrNotBuilt` jadi state nil). Bila `incrementalScan` tidak ada/berbeda di kode nyata, lakukan urutan yang sama inline (`Scan` -> `GroupModules` -> `HashAll` -> `ReadState`).
- Tiket 02: `measure.go` memakai heuristik `(len+3)/4` secara inline (`MeasureInjection`) dan tidak punya helper terekspor; maka `EstTokensFromBytes` tetap `(n+3)/4`. Bila saat implementasi ada helper token terekspor di `measure.go`, jadikan `EstTokensFromBytes` pembungkus tipis atasnya.
- `paths.DataDir()` (`internal/paths`) untuk fungsi paket-level tanpa Store, pola yang sama dengan `MeasureInjection`.
- Dari stub/fitur `llm-summarizer` (`prompt.go`): `LLMMaxSourceBytes` (48 KiB), `LLMExpectedOutputBytes` (3 KiB), `LLMPromptOverheadBytes() int`. Hanya dipanggil, tidak didefinisikan ulang di sini.

## Langkah

### 1. `internal/grimoire/settings.go` (timpa penuh)

Hapus baris `// STUB(foundation)...`. Import: `context`, `encoding/json`, `errors`, `log/slog`, `sync`, `internal/ipc`, `internal/paths`.

1. Definisikan `Settings`, `ErrInvalidMaxModules`, `LLMDeps` persis seperti Kontrak. Konstanta `settingsFile = "settings.json"`.
2. Helper tak-diekspor (awalan `settings`):
   - `settingsStore() (*Store, error)`: `paths.DataDir()` lalu `NewStore(dir)`.
   - `settingsLoad(st *Store, projectID string) (Settings, error)`: `st.ReadFile(projectID, settingsFile)`. `errors.Is(err, os.ErrNotExist)` -> `Settings{}, nil`. Galat lain dikembalikan apa adanya (termasuk `ErrInvalidProject`). `json.Unmarshal` gagal -> `fmt.Errorf("settings.json rusak: %w", err)` (galat, bukan default). Nilai `MaxModules != nil && *MaxModules < 1` di berkas dianggap `nil` (tidak galat).
   - `settingsSave(st *Store, projectID string, s Settings) error`: tolak `s.MaxModules != nil && *s.MaxModules < 1` dengan `ErrInvalidMaxModules`; `json.MarshalIndent(s, "", "  ")` + newline; `st.WriteFile(projectID, settingsFile, data)` (atomik, 0600; berkas dibuat hanya saat save pertama; tidak ada penulisan di repo proyek).
   - `settingsMu sync.Mutex` paket-level, men-serialisasi read-modify-write pada handler `set` (cukup satu mutex global; operasi ringan).
3. `LoadSettings(projectID)`: `settingsStore()` lalu `settingsLoad`. `SaveSettings(projectID, s)`: `settingsStore()` lalu `settingsSave`.
4. Tipe respons `settingsView struct { LLM bool \`json:"llm"\`; MaxModules *int \`json:"maxModules"\` }` (tanpa `omitempty` agar `maxModules` tampil sebagai `null`); bisa memakai `Settings` langsung karena tag-nya sudah benar.
5. `RegisterLLM(srv *ipc.Server, d LLMDeps)`; bentuk `svc := &settingsService{d: d, srv: srv}` dan daftarkan tiga handler (`srv.Register(method, func(ctx context.Context, params json.RawMessage) (any, error))`). Store handler: `d.Store` bila non-nil, selain itu `settingsStore()`. Validasi project di semua handler: `projectId` kosong -> `ipc.NewError(ipc.ErrInvalid, "projectId wajib diisi")`; `st.ValidateProject(id)` gagal -> `ErrInvalid`.
   - `grimoire.settings.get`: decode `{projectId}` via `rpcDecode`; `settingsLoad`; galat dipetakan `rpcMapErr` (JSON rusak -> `ErrInternal`). Tidak menulis berkas.
   - `grimoire.settings.set`: decode ke struct `{ProjectID string \`json:"projectId"\`; LLM *bool \`json:"llm"\`; MaxModules json.RawMessage \`json:"maxModules"\`}`. Pembeda "tidak dikirim" vs `null`: `RawMessage` nil/len 0 = tidak dikirim (tak diubah); `bytes.Equal(trimmed, []byte("null"))` = hapus batas (`MaxModules = nil`); selain itu `json.Unmarshal` ke `float64`/`json.Number`, harus integer (tidak ada pecahan; tolak `1.5`, string, bool) dan `>= 1`, bila tidak -> `ipc.NewError(ipc.ErrInvalid, "invalid params: "+ErrInvalidMaxModules.Error())`. Perhatikan: `encoding/json` memberi `RawMessage("null")` untuk nilai `null` eksplisit pada field `json.RawMessage` (bukan nil), jadi pembedaan ini valid; tulis uji mental dengan tiga input (`{}`, `{"maxModules":null}`, `{"maxModules":3}`). Di bawah `settingsMu`: `settingsLoad` (galat JSON rusak: pada `set`, tetap kembalikan `ErrInternal`; pengguna dapat memperbaiki berkas), terapkan field yang dikirim, `settingsSave` (`ErrInvalidMaxModules` -> `ErrInvalid`), lalu setelah lepas lock: publish. Publish: bila `d.Publish != nil` panggil `d.Publish(id)`, selain itu `srv.Publish("grimoire.changed", map[string]any{"projectId": id})`. Kembalikan settings terbaru. `set` tanpa field apa pun: tidak menulis berkas, tidak publish, kembalikan settings saat ini (settings.json "dibuat hanya saat set pertama" yang benar-benar mengubah/menyimpan; `set` kosong boleh dianggap no-op, putuskan no-op).
   - `grimoire.estimate`: decode `{projectId}`; `d.ResolvePath == nil` -> `ErrInternal "resolver project belum terpasang"`; `p, err := d.ResolvePath(ctx, id)` galat -> `rpcMapErr(err)` (project tak dikenal -> `ErrNotFound` mengikuti pemetaan `rpcMapErr`; bila galat resolver bukan `ipc.Error` dan tidak cocok sentinel, hasilnya `ErrInternal`, sama seperti `grimoire.status`); `EstimateBuild(id, p)`; galat dipetakan `rpcMapErr`. Kembalikan `Estimate` apa adanya. Handler ini TIDAK menulis berkas dan TIDAK menjalankan `claude`.
6. Logging: bila `d.Logger != nil`, log galat internal dengan pesan bahasa Indonesia; selalu cek nil.
7. Tidak ada goroutine, timer, atau watcher.

### 2. `internal/grimoire/estimate.go` (timpa penuh)

Hapus baris `// STUB(foundation)...`. Import: `os`, `path/filepath`, `errors` (bila perlu), `fmt`. 

1. `Estimate` persis seperti Kontrak (`OverLimit` `omitempty`).
2. `EstTokensFromBytes(n int) int`: `if n < 0 { n = 0 }; return (n + 3) / 4` (atau pembungkus helper `measure.go` bila ada, lihat Kontrak). Ini satu-satunya heuristik di epic ini.
3. `EstimateBuild(projectID, projectPath string) (Estimate, error)`:
   1. `dir, err := paths.DataDir()`; `store := NewStore(dir)`; `store.ValidateProject(projectID)` gagal -> kembalikan galat (`ErrInvalidProject`).
   2. `root, err := incrementalRoot(projectPath)`; lalu `mods, hashes, st, err := incrementalScan(store, projectID, root)` (`ErrNotBuilt` sudah menjadi `st == nil` = semua Added; bila inline, tangani `errors.Is(err, ErrNotBuilt)` sendiri). Tidak ada penulisan.
   3. `diff := ComputeDiff(mods, hashes, st)`; `queue := diff.Pending()` (urutan Added, Modified, Stale; JANGAN diurut ulang). Modul Removed tidak masuk antrean.
   4. `settings, err := LoadSettings(projectID)`. Galat baca settings TIDAK menggagalkan estimasi: gunakan `Settings{}` (tanpa batas) bila galat. Bila `settings.MaxModules != nil && len(queue) > *settings.MaxModules`: `over := len(queue) - N`, `queue = queue[:N]`.
   5. Peta cepat nama -> `Module`: iterasi `mods` dengan kunci `Snake(m.Name)` (sama seperti `ComputeDiff`). Untuk tiap `ModuleChange` dalam antrean, ambil modulnya; bila tidak ketemu, lewati token modul itu tetapi tetap dihitung di `Modules` (jangan panik).
   6. Byte sumber modul: `estimateSourceBytes(root string, m Module) int` (helper tak-diekspor berawalan `estimate`): jumlahkan ukuran tiap path di `m.Sources` lewat `os.Stat(filepath.Join(root, filepath.FromSlash(p)))` (`.Size()`; gagal stat atau bukan berkas biasa = 0). Bila `m.Sources` kosong, pakai `m.Files[i].Path` sebagai cadangan dengan cara sama. Jangan membaca isi berkas.
   7. Token per modul = `EstTokensFromBytes(min(sourceBytes, LLMMaxSourceBytes) + LLMPromptOverheadBytes() + LLMExpectedOutputBytes)`. `EstTokens` = jumlah semua modul antrean (setelah dipangkas). `Modules` = `len(queue)` setelah dipangkas. `OverLimit` = `over` (0 bila tidak ada; `omitempty` menyembunyikannya).
   8. Kembalikan `Estimate{Modules, EstTokens, OverLimit}`.
4. Invarian wajib: tidak ada `exec`, tidak ada penulisan berkas apa pun (tidak memanggil `WriteFile`/`WriteState`), tidak memanggil `BuildIncremental`. Project tanpa grimoire -> semua modul Added; setelah build penuh tanpa perubahan -> `Modules == 0`.

### 3. Kompilasi dan periksa

1. `gofmt -l internal/grimoire` kosong; `go build ./...` lulus (stub `prompt.go` Foundation atau versi final `llm-summarizer` menyediakan konstanta; bila build gagal karena berkas fitur lain, jangan diperbaiki, laporkan).
2. Pastikan tidak ada nama ganda: `grep -rn "type Settings\|type LLMDeps\|func RegisterLLM\|type Estimate\|func EstimateBuild\|func EstTokensFromBytes" internal/grimoire` hanya satu hit tiap nama (di berkas milik fitur ini). Helper berawalan `settings*`/`estimate*` tidak bentrok dengan berkas lain.

### 4. `cmd/shikigamictl/cmd_grimoire.go` (aditif minimum; baca berkas aktual dulu, tiket 02/03 mungkin sudah menambah baris stderr)

1. Usage `grimoireUsage`: ubah baris status menjadi `shikigamictl grimoire status [--project <id>] [--dry-run]   tampilkan status grimoire project (--dry-run: estimasi modul dan token tanpa memanggil LLM)`. Baris `build` tidak berubah.
2. Di `runGrimoire`: tambah `dryRun := fs.Bool("dry-run", false, "estimasi build tanpa memanggil LLM (hanya untuk status)")`. Setelah parse: `if *dryRun && sub != "status" { return errors.New(grimoireUsage) }` (pola yang sama dengan penanganan `--full` hanya untuk build; flag `--full` pada status tetap perilaku lama).
3. Cabang `sub == "status"` dengan `*dryRun` (ditaruh sebelum pemanggilan `grimoire.status`, setelah `params` dibentuk): 
   - `var out map[string]any; c.Call("grimoire.estimate", params, &out)` (galat lewat `grimoireError`); `printJSON(out)`; hitung `modules`, `estTokens` dari `out` (`float64` -> `int`, pola `num` yang sama); cetak ke stderr `fmt.Fprintf(os.Stderr, "estimasi: %d modul, ~%d token\n", modules, estTokens)`. Bila `out["overLimit"]` ada dan > 0, tambah satu baris stderr `di atas batas maxModules: %d modul tidak dihitung`. `return nil`. Tidak memanggil `grimoire.status`.
4. Cabang `status` biasa (tanpa `--dry-run`): setelah `printJSON(out)` dan baris stderr `index: ...` yang SUDAH ADA (pertahankan semua baris tiket 02/03 apa adanya, jangan ubah teksnya), tambahkan:
   - `llm: aktif` bila `out["llmEnabled"] == true`, selain itu (false atau field tak ada karena daemon lama) `llm: mati`.
   - Bila `out["lastBuildCost"]` adalah `map[string]any` non-nil: `build terakhir: M modul, ~T token, D ms, gagal F` dengan `modules`, `estTokens`, `durationMs`, `failed` dibaca sebagai `float64` -> `int`. Bila `skipped` ada dan > 0, tambah `, dilewati S`; bila `note` string tak kosong, tambah baris stderr `catatan: <note>`. Bila `lastBuildCost` null atau tak ada, tidak ada baris itu.
5. Jangan menambah subcommand, jangan mengubah `grimoireError`, `grimoireDefaultProject`, atau jalur `build`. `callAndPrint` tetap dipakai oleh `build`.

## Perubahan

- Diubah (timpa stub Foundation): `internal/grimoire/settings.go`, `internal/grimoire/estimate.go`.
- Diubah (aditif): `cmd/shikigamictl/cmd_grimoire.go`.
- Tidak ada berkas baru, tidak ada perubahan `go.mod`/`go.sum`, tidak ada unit test.

## Wiring

Yang diekspos untuk disambungkan nanti (JANGAN disambungkan di fitur ini):

- `grimoire.RegisterLLM(srv *ipc.Server, d grimoire.LLMDeps)`: dipanggil tepat sekali dari `cmd/shikigamid/wire_grimoire_llm.go` (milik `llm-summarizer`) di `d.OnReady`; mendaftarkan `grimoire.settings.get`, `grimoire.settings.set`, `grimoire.estimate`. Jangan didaftarkan dua kali (server panik bila ganda).
- `grimoire.LoadSettings(projectID)`: dipakai Wiring `ipc.go` untuk mengisi `statusResult.LLMEnabled` (galat = false), dan dipakai `summarize.go` (`SummarizerFor`).
- `grimoire.EstimateBuild`/`EstTokensFromBytes`: dipakai handler `grimoire.estimate` dan `summarize.go` (estimasi token kumulatif).
- CLI: perubahan `cmd_grimoire.go` sudah final di fitur ini; langkah Wiring hanya memverifikasi (overview 5 butir 5). Baris `llm:`/`build terakhir:` bergantung pada field `llmEnabled`/`lastBuildCost` pada `grimoire.status` yang ditambah Wiring di `ipc.go`; sebelum itu, CLI tetap benar (mencetak `llm: mati` dan tanpa baris biaya).

## Selesai bila

- `settings.go` dan `estimate.go` tidak lagi memuat `STUB(foundation)`; signature sama persis dengan overview 2.3.
- `go build ./...` sukses (guard utama); `gofmt -l internal/grimoire cmd/shikigamictl` kosong; opsional `go vet ./internal/grimoire/... ./cmd/shikigamictl/...`.
- Secara perilaku (ditinjau dari kode, tanpa unit test): tanpa `settings.json` `get` memberi `{llm:false, maxModules:null}` dan tidak menulis berkas; `set` dengan `maxModules` 0/negatif/non-integer -> `ErrInvalid`; `maxModules:null` menghapus batas; field tak dikirim tak berubah; `set` sukses publish `grimoire.changed {projectId}`; `estimate` tidak menulis apa pun dan tidak menjalankan `claude`; project tanpa grimoire = semua modul Added; setelah build penuh tanpa perubahan `modules: 0`; `--dry-run` hanya sah untuk `status`, mencetak JSON dan `estimasi: N modul, ~T token`.
- `git diff --stat` hanya menunjukkan tiga berkas ownedPaths; tidak ada perubahan di `go.mod`, `go.sum`, `frontend/`, `cmd/shikigamid/`, `incremental.go`, `ipc.go`.
- Tidak ada `time.Ticker`/polling/watcher, tidak ada `exec` di kedua berkas Go fitur ini.
