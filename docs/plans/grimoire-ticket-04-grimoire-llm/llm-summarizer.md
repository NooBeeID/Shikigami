# Plan: Summarizer claude -p

## Metadata

- Tiket: `grimoire-ticket-04-grimoire-llm` (`docs/tickets/grimoire-ticket-04-grimoire-llm.md`)
- Fitur: `llm-summarizer`
- ownedPaths: `internal/grimoire/summarize.go`, `internal/grimoire/prompt.go`, `cmd/shikigamid/wire_grimoire_llm.go`
- Depends on foundation: ya. Stub `summarize.go` dan `prompt.go` (baris pertama `// STUB(foundation): ditimpa oleh llm-summarizer`) harus sudah ada, begitu juga stub `settings.go` dan `estimate.go` (`Settings`, `LLMDeps`, `RegisterLLM`, `LoadSettings`, `EstTokensFromBytes`). Bila stub belum ada, berhenti dan laporkan.
- Sumber kebenaran bersama: `docs/plans/grimoire-ticket-04-grimoire-llm/overview.md` (terutama 2.2, 2.3, 2.6, 7). Plan ini tidak mengulangnya; baca overview sebelum mulai.

## Tujuan

Mengimplementasikan `Summarizer` berbasis `claude -p` untuk modul antrean build inkremental (tiket 03): prompt tetap dan konstanta di `prompt.go`, logika per modul (timeout, fallback statik, `maxModules`, validasi `Sumber:`, progres, biaya, publish) di `summarize.go`, serta modul daemon `grimoire-llm` yang memanggil `RegisterLLM` dan `SetLLMRuntime`. Kegagalan LLM tidak pernah membuat build fatal; `llm=false` atau `claude` tidak ada berarti `SummarizerFor` mengembalikan nil murni.

## Path yang dimiliki

HANYA berikut yang boleh disentuh (menimpa penuh stub Foundation untuk dua berkas pertama, membuat berkas ketiga):

- `internal/grimoire/summarize.go`
- `internal/grimoire/prompt.go`
- `cmd/shikigamid/wire_grimoire_llm.go`

Jangan menyentuh: `settings.go`, `estimate.go`, `incremental.go`, `ipc.go`, `wire_grimoire*.go` lain, `cmd/shikigamictl/`, frontend, `internal/ipc`, `internal/session`, `internal/provider`, `go.mod`/`go.sum`. Hook satu baris di `incremental.go` dan field status di `ipc.go` adalah tugas Wiring, bukan fitur ini.

## Kontrak

Disediakan (signature final, overview 2.3; JANGAN diubah):

- `prompt.go`: `const LLMMaxSourceBytes = 48 * 1024`, `const LLMExpectedOutputBytes = 3 * 1024`, `func LLMPromptOverheadBytes() int`, tak-diekspor `promptBuild(m Module, static string, sources []promptSource) string` dan tipe `promptSource`.
- `summarize.go`: `BuildCost`, `LLMProgress`, `ErrLLMOverLimit`, `LoadBuildCost(projectID) (*BuildCost, error)`, `ProgressFor(projectID) *LLMProgress`, `SetLLMRuntime(d LLMDeps)`, `SummarizerFor(projectID, projectPath string) Summarizer`, `NewLLMSummarizer(deps LLMDeps, projectID, projectPath string, s Settings) Summarizer`.
- `wire_grimoire_llm.go`: modul daemon `grimoire-llm` (tanpa API ekspor).

Dipakai (jangan didefinisikan ulang; baca kode nyata dan ikuti bila beda kecil dari overview):

- Tiket 01: `Store` (`NewStore`, `Dir`, `WriteFile`, `ReadFile`, `ReadState`, `ValidateProject`), `Module{Name, Dir, Kind, Files, Symbols, Sources, Tag, Truncated}`, `FileInfo{Path,...}`, `RenderBooks(mods) ([]Book, []Page)`, `Page{Path, Content}`, `Snake`, `State`, `ModuleState{Hash, Sources, Stale}`, `ErrNotBuilt`.
- Tiket 03: interface `Summarizer { Summarize(ctx, m Module, static string) (map[string]string, error) }` di `incremental.go`. Perilaku nyata `incrementalRebuildModule`: `static` = gabungan konten halaman statik modul dipisah `"\n\n"`; bila `Summarize` mengembalikan galat, halaman statik dipakai dan modul dihitung gagal (lama = `Stale=true`, baru = dikeluarkan dari index); untuk tiap halaman, `chapters[<nama bab>]` yang TIDAK kosong menggantikan isinya, nilai kosong/kunci tak ada = isi statik (berfooter `## Hash`) dipertahankan.
- Tiket 02: `measure.go` heuristik token lewat `EstTokensFromBytes` (milik `estimate.go`; jangan menduplikasi `(n+3)/4`).
- Dari fitur `settings-estimate` (stub cukup): `Settings{LLM, MaxModules *int}`, `LoadSettings`, `LLMDeps{Store, ResolvePath, Publish, Logger}`, `RegisterLLM(srv, d)`, `EstTokensFromBytes`.
- Daemon: `Deps` (`DataDir`, `SocketPath`, `Logger`, `OnReady`, `Lookup`), `RegisterModule`, `newGrimoireResolver(d.SocketPath)` dari `wire_grimoire.go` (hanya dipanggil), `paths.DataDir()`, `ipc.Server.Publish(topic, payload)`.

Event: publish `grimoire.changed {projectId}` lewat `LLMDeps.Publish` (nullable, cek nil) di awal dan akhir tiap modul. Tidak ada method IPC baru dari fitur ini selain yang didaftarkan `RegisterLLM` milik fitur lain.

## Langkah

1. **Persiapan (baca dulu, tanpa menulis).**
   - Baca overview bagian 2.2 butir 3, 5, 7 dan bagian 2.3 (perilaku `Summarize`) dan 2.6.
   - Baca kode nyata: `internal/grimoire/incremental.go` (`Summarizer`, `incrementalRebuildModule`), `pages.go` (`RenderBooks`, nama bab, format `## Hash`), `store.go` (`WriteFile`, `ReadState`), `cmd/shikigamid/wire_grimoire.go` dan `modules.go` (bentuk `Deps`, `RegisterModule`, `d.OnReady`).
   - Jalankan `claude -p --help` dan catat flag nyata: mode teks (`--output-format text`), pemilihan model (`--model`), serta flag untuk menonaktifkan tool (mis. `--allowed-tools ""`/`--tools ""`/`--disallowed-tools`, pakai yang benar-benar ada di versi terpasang). Dilarang `--dangerously-skip-permissions` dan padanannya. Bila `claude` tidak terpasang di mesin implementasi, tulis flag sesuai kebiasaan CLI lalu catat di komentar kode bahwa verifikasi flag tertunda.

2. **`prompt.go`** (hapus baris `// STUB(foundation)`).
   - Konstanta `LLMMaxSourceBytes`, `LLMExpectedOutputBytes` (nilai persis overview).
   - `const promptTemplate` (template tetap, bahasa Indonesia) berisi instruksi: tulis ringkasan snapshot fakta, tanpa tanggal/changelog/narasi proses; jelaskan tanggung jawab modul, hubungan antarmodul, dan "mengapa" yang terlihat dari kode; baris pertama keluaran WAJIB `Sumber: <path relatif repo>, ...`; keluarkan hanya isi bab Markdown (tanpa pembuka/penutup percakapan, tanpa code fence pembungkus); jangan menjalankan tool atau membaca berkas lain. Sisakan penampung terpisah untuk (a) nama modul, (b) kerangka statik, (c) daftar sumber berikut isi.
   - `type promptSource struct { Path string; Content string; Note string }` (Note untuk penanda "dilewati: biner/terlalu besar").
   - `func LLMPromptOverheadBytes() int` mengembalikan `len(promptTemplate)` (konstan, tanpa isi modul). Nilai ini dipakai `EstimateBuild` milik fitur lain, jadi harus murni dan tanpa I/O.
   - `func promptBuild(m Module, static string, sources []promptSource) string`: merakit template + nama modul + `static` + tiap sumber dengan pembatas jelas (`=== <path> ===`). Bila gabungan isi melewati `LLMMaxSourceBytes`, dipotong dan diberi penanda `[...dipotong...]` (pemotongan total dilakukan di pemuat sumber, langkah 4; `promptBuild` tidak memotong ulang tetapi boleh memakai `Note`).
   - Helper tak-diekspor berawalan `prompt*`.

3. **`summarize.go`: tipe dan konstanta** (menimpa stub).
   - Tipe/sentinel persis overview: `BuildCost`, `LLMProgress`, `ErrLLMOverLimit`.
   - Konstanta: `summarizeModuleTimeout = 120 * time.Second`, `summarizeDefaultModel = "haiku"`, `summarizeModelEnv = "SHIKIGAMI_GRIMOIRE_MODEL"` (didokumentasikan di komentar: nilai env tak kosong menimpa default), `summarizeMaxOutputBytes = 1 << 20`, `summarizeCostFile = "lastbuild.json"`, `summarizeTruncMarker = "[...dipotong...]"`, `summarizeMaxFileBytes` (batas per berkas sumber, mis. `LLMMaxSourceBytes`).
   - Runtime global: `var summarizeRuntime atomic.Pointer[LLMDeps]`; registry progres: `var summarizeActive = struct{ sync.Mutex; m map[string]*llmSummarizer }`. `SetLLMRuntime(d)` menyimpan salinan `d` ke `summarizeRuntime`.

4. **`summarize.go`: fungsi paket-level.**
   - `LoadBuildCost(projectID)`: `NewStore(paths.DataDir())` (pola `ComposeInjection`), `Store.ReadFile(projectID, "lastbuild.json")`; `os.ErrNotExist` (termasuk direktori project belum ada) = `nil, nil`; JSON rusak = galat. Tidak menulis apa pun.
   - `ProgressFor(projectID)`: cari instance aktif di `summarizeActive`; kembalikan SALINAN `LLMProgress` di bawah mutex, `nil` bila tak ada. Instance didaftarkan saat `Summarize` pertama dipanggil (bukan saat dibuat) dan dihapus dari registry setelah modul terakhir tak ada lagi mekanisme akhir: karena tidak ada hook akhir build, biarkan instance terdaftar sampai instance baru untuk project yang sama menggantikannya (UI hanya membaca progres saat `building == true`, overview 2.2.8). Satu entri per project, jadi tidak bocor.
   - `SummarizerFor(projectID, projectPath)`: kembalikan interface nil MURNI (`return nil`, bukan typed-nil) bila runtime belum diset, `LoadSettings(projectID)` galat atau `LLM=false`, atau `exec.LookPath("claude")` gagal. Dalam kasus `claude` tidak ada saat `llm=true`, tulis `Note: "claude tidak terpasang: mode statik"` ke `lastbuild.json` hanya bila belum ada isi lebih baru; sederhananya: log `Info` saja dan JANGAN menulis berkas (sesuai 2.2.5 build tanpa panggilan LLM tidak mengubah `lastbuild.json`; `Note` ditulis oleh latch di langkah 6). Tidak menjalankan `claude` apa pun di sini.
   - `NewLLMSummarizer(deps, projectID, projectPath, s)`: kembalikan `*llmSummarizer` baru sebagai `Summarizer`; tidak mendaftar ke registry sampai `Summarize` pertama.

5. **`summarize.go`: tipe `llmSummarizer`.** Field: `deps LLMDeps`, `projectID`, `projectPath`, `settings Settings`, `start time.Time` (saat dibuat), `mu sync.Mutex`, `unavailable bool`, `note string`, `queued int` (jumlah modul antrean yang sudah masuk `Summarize`), `cost BuildCost` (berjalan), `progress LLMProgress`. Semua akses di bawah `mu`; kunci TIDAK dipegang selama menjalankan proses `claude` atau I/O (ambil/lepas di sekitar pembaruan state).

6. **`(*llmSummarizer).Summarize(ctx, m, static)`** (urutan wajib):
   1. `_, pages := RenderBooks([]Module{m})`; `names` = nama bab (`strings.TrimSuffix(filepath.Base(p.Path), ".md")`) urut sesuai halaman; `target` = bab pengganti sesuai koreksi 2.2.3: bab bernama `Snake(m.Name)` bila ada, selain itu bab pertama urut nama. Ekstrak hash footer dari `static` (regexp `(?m)^## Hash\n(\S+)`) bila ada, untuk langkah 8.
   2. Cek `ctx.Err()`; bila batal kembalikan galat konteks (bukan latch).
   3. Naikkan `queued`. Bila `settings.MaxModules != nil && queued > *settings.MaxModules`: `cost.Skipped++`, `cost.Modules` TIDAK bertambah; simpan `lastbuild.json` dan publish; lalu bila modul sudah ada di `state.json` (`Store.ReadState`, `ErrNotBuilt`/galat baca = anggap tidak ada; `ModuleState` kunci `Snake(m.Name)`; bila kunci aktual di kode tiket 03 berbeda ikuti kode nyata) kembalikan `nil, ErrLLMOverLimit`; bila modul baru kembalikan peta statik (langkah 7, tanpa galat).
   4. Bila `unavailable` true: kembalikan peta statik tanpa galat dan tanpa menjalankan claude.
   5. Set `progress.Current = Snake(m.Name)`, daftarkan instance ke `summarizeActive[projectID]`, simpan `lastbuild.json` TIDAK (belum ada hasil), publish `grimoire.changed` (modul mulai).
   6. Muat sumber (`summarizeLoadSources`): iterasi `m.Sources` berurutan, baca `projectPath/<path>` (bersihkan path; tolak yang keluar dari `projectPath`; lewati symlink keluar root), lewati biner (ada byte NUL di 8 KiB awal) dan berkas > batas per-berkas dengan `Note`, akumulasi hingga `LLMMaxSourceBytes` total lalu potong dengan `summarizeTruncMarker` dan berhenti. Bangun `prompt := promptBuild(m, static, sources)`.
   7. Jalankan claude lewat `summarizeRunClaude(ctx, projectPath, prompt) (out string, stderr string, err error)` dengan `context.WithTimeout(ctx, summarizeModuleTimeout)` (aturan 2.6: `exec.LookPath("claude")` lalu `exec.CommandContext`, `cmd.Env = os.Environ()`, `cmd.Dir = projectPath`, prompt via stdin, `--output-format text`, `--model` dari env/default, tool dinonaktifkan sesuai flag nyata dari langkah 1; stdout/stderr dibaca lewat penampung berbatas `summarizeMaxOutputBytes` (writer yang membuang kelebihan); tanpa `pkill`, proses mati hanya lewat context; `cmd.WaitDelay` singkat agar `Wait` tidak menggantung).
   8. Klasifikasi hasil:
      - Galat `exec` (biner hilang saat Start) atau exit non-nol dengan stdout kosong dan stderr (lowercase) memuat salah satu: `login`, `auth`, `credential`, `api key`, `not logged in`, `unauthorized`: set `unavailable = true`, `note` = `"belum login: mode statik"` (atau `"claude tidak terpasang: mode statik"` untuk galat exec/`ErrNotFound`), log `Warn` (tanpa isi prompt), simpan `lastbuild.json` (membawa `Note`), publish, kembalikan peta statik TANPA galat. Hitungan `Failed` tidak naik.
      - `context.DeadlineExceeded` dari timeout modul, exit non-nol lain, atau keluaran kosong setelah `TrimSpace`: `cost.Failed++`, `progress.Failed++`, `progress.Current = ""`, hitung `EstTokens` hanya dari prompt terkirim, simpan `lastbuild.json`, publish, kembalikan `nil, fmt.Errorf(...)` berpesan Indonesia (tiket 03 menandai stale). Pembatalan konteks induk: kembalikan galat konteks tanpa menghitung gagal.
      - Sukses: validasi (langkah 9).
   9. Validasi keluaran: `TrimLeft` spasi/newline; buang pagar kode pembungkus ```` ``` ```` bila seluruh keluaran terbungkus. Bila baris pertama tidak berawalan `Sumber:`, sisipkan `Sumber: <ModuleState.Sources atau m.Sources dipisah ", ">` + newline di atas keluaran (urutan prioritas: sumber dari `state.json` bila ada dan tidak kosong, kalau tidak `m.Sources`; bila keduanya kosong `Sumber: -`). Tanpa tanggal/changelog. Bila footer hash ditemukan di langkah 1 dan keluaran belum memuat `## Hash`, tambahkan `\n\n## Hash\n<hash>\n` agar format bab konsisten dengan bab statik.
   10. Hasil: `cost.Modules++`, `progress.Done++`, `progress.Current = ""`, `cost.EstTokens += EstTokensFromBytes(len(prompt)+len(output))`; `cost.DurationMs = time.Since(start).Milliseconds()`, `cost.At = time.Now().UTC().Format(time.RFC3339)`; salin `note` bila ada; simpan `lastbuild.json`; publish. Kembalikan peta dengan satu entri `{target: output}` saja.

7. **Peta statik (fallback tanpa galat).** Aturan: `BuildIncremental` kontrak nyata mempertahankan halaman statik berhash bila nilai kunci kosong/tak ada. Maka fungsi `summarizeStaticChapters(names)` mengembalikan `map[string]string` berisi SEMUA nama bab statik dengan nilai `""` (kunci ada sesuai kontrak overview 2.2.3, nilai kosong = pertahankan halaman statik utuh beserta footer hash). Catat di komentar kode alasan nilai kosong. Bila kode nyata `incrementalRebuildModule` ternyata tidak mengabaikan nilai kosong (berbeda dari kutipan di atas), ganti nilai menjadi isi `pages[i].Content` statik dan catat penyimpangan di komentar.

8. **Penyimpanan biaya.** `summarizeSaveCost` (di luar kunci `mu`, salin `cost` dulu): `json.MarshalIndent(BuildCost)` lalu `Store.WriteFile(projectID, "lastbuild.json", data)` (atomik; `deps.Store` bila ada, kalau nil bentuk dari `paths.DataDir()`). Galat tulis hanya di-log `Warn`, tidak menggagalkan modul. Jangan menulis apa pun di dalam repo proyek. `Failed` pada `BuildCost` dihitung dari modul gagal, bukan dari fallback `unavailable` (alasan fallback ada di `Note`).

9. **Publish.** `summarizePublish`: `if deps.Publish != nil { deps.Publish(projectID) }`; panggilan `Publish` tidak boleh memegang `mu` (hindari deadlock).

10. **`cmd/shikigamid/wire_grimoire_llm.go`** (baru).
    - `package main`; impor `grimoire`, `ipc`, `paths`.
    - `func init() { RegisterModule("grimoire-llm", func(srv *ipc.Server, d *Deps) { ... }) }`.
    - Di dalam: tentukan `dataDir` (pola `wire_grimoire.go`: `d.DataDir` atau `paths.DataDir()`; gagal = log `Warn` dan return). Layanan lain diambil di `d.OnReady`; bentuk `grimoire.LLMDeps{Store: grimoire.NewStore(dataDir), ResolvePath: newGrimoireResolver(d.SocketPath), Publish: func(id string){ srv.Publish("grimoire.changed", map[string]any{"projectId": id}) }, Logger: d.Logger}`; panggil `grimoire.RegisterLLM(srv, deps)` TEPAT SEKALI dan `grimoire.SetLLMRuntime(deps)` di dalam callback `d.OnReady` (sesuaikan dengan bentuk nyata `Deps.OnReady` di `modules.go`; bila `OnReady` tidak dibutuhkan untuk `RegisterLLM` tetapi aturan overview mengharuskannya, ikuti overview 2.1/2.3). Tidak ada `Provide`, `LaunchContribution`, ticker, atau goroutine.
    - Helper tak-diekspor berawalan `llmWire*` bila perlu.

11. **Verifikasi lokal.** `gofmt -l internal/grimoire cmd/shikigamid` kosong; `go build ./...` lulus; `go vet ./internal/grimoire/... ./cmd/shikigamid/...` bila cepat. Pastikan: `grep -rn '"claude"' internal/grimoire` hanya di `summarize.go`; tidak ada `time.NewTicker`/`time.Tick`/`setInterval`/`fsnotify`; tidak ada `pkill`/`os/signal`/pencarian proses; tidak ada `--dangerously-skip-permissions`; `git diff --stat -- go.mod go.sum` kosong; tidak ada definisi ganda `Settings`/`Estimate`/`LLMDeps` di berkas milikmu. Jika build gagal karena berkas milik fitur lain, jangan perbaiki; laporkan.

## Perubahan

- Diubah (menimpa stub Foundation): `internal/grimoire/summarize.go`, `internal/grimoire/prompt.go`
- Dibuat: `cmd/shikigamid/wire_grimoire_llm.go`
- Dibuat saat runtime (bukan di repo): `<DataDir>/grimoire/<projectId>/lastbuild.json`

## Wiring

Fitur ini hanya mengekspos; agen wiring yang menyambungkan (JANGAN dikerjakan di sini):

1. Hook di `internal/grimoire/incremental.go` (`BuildIncremental`, setelah validasi opts): `if opts.Summarizer == nil { opts.Summarizer = SummarizerFor(projectID, opts.ProjectPath) }`. `SummarizerFor` menjamin nil murni untuk mode statik.
2. `internal/grimoire/ipc.go`: `grimoire.status` memakai `LoadBuildCost(id)` (`*BuildCost`), `ProgressFor(id)` (`*LLMProgress`), dan `LoadSettings` untuk `llmEnabled`.
3. Modul `grimoire-llm` sudah mendaftar sendiri lewat `init()`; `modules.go`/`main.go` tidak berubah. Pastikan `RegisterLLM` hanya dipanggil dari modul ini (server panik bila method ganda).
4. Hanya `BuildCost`, `LLMProgress`, `ErrLLMOverLimit`, `LoadBuildCost`, `ProgressFor`, `SetLLMRuntime`, `SummarizerFor`, `NewLLMSummarizer`, `LLMMaxSourceBytes`, `LLMExpectedOutputBytes`, `LLMPromptOverheadBytes` yang boleh dipakai pihak luar.

## Selesai bila

- `go build ./...` sukses (guard; tanpa unit test), `gofmt -l` kosong, tanpa perubahan `go.mod`/`go.sum`.
- Tidak ada baris `// STUB(foundation)` tersisa di `summarize.go` dan `prompt.go`; signature sama persis dengan overview 2.3.
- `SummarizerFor` mengembalikan nil murni untuk runtime belum diset, `llm=false`, atau `claude` tidak ada; `claude` tidak pernah dijalankan di luar `Summarize`.
- `Summarize`: timeout 120 dtk per modul; gagal/timeout/keluaran kosong mengembalikan galat; claude tidak ada/belum login memasang latch `unavailable` + `Note` dan mengembalikan statik tanpa galat; modul di atas `maxModules` menghasilkan `ErrLLMOverLimit` (modul lama) atau statik (modul baru) dan dihitung `Skipped`; baris pertama keluaran selalu `Sumber:`; hanya map bab yang dikembalikan, tidak ada penulisan bab oleh summarizer.
- `lastbuild.json` ditulis atomik setelah tiap modul (sukses, gagal, dilewati), `grimoire.changed` dipublikasikan di awal dan akhir tiap modul, progres terbaca lewat `ProgressFor`.
- `wire_grimoire_llm.go` mendaftarkan modul `grimoire-llm`, memanggil `RegisterLLM` dan `SetLLMRuntime` lewat `d.OnReady`, tanpa ticker/goroutine/`Provide`.
- Tidak ada berkas ditulis di dalam repo proyek, tanpa `pkill`, tanpa flag izin berbahaya, hanya berkas ownedPaths yang berubah.
