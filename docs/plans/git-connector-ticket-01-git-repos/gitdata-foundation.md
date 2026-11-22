# Plan: Paket gitdata: tipe, runner, allowlist, deteksi repo

## Metadata
- Tiket: `git-connector-ticket-01-git-repos` (`docs/tickets/git-connector-ticket-01-git-repos.md`)
- Fitur: `gitdata-foundation`
- Overview (sumber kebenaran kontrak): `docs/plans/git-connector-ticket-01-git-repos/overview.md`, terutama bagian 2.3, 1.1, 6, 7
- ownedPaths:
  - `internal/gitdata/doc.go`
  - `internal/gitdata/types.go`
  - `internal/gitdata/allowlist.go`
  - `internal/gitdata/runner.go`
  - `internal/gitdata/errors.go`
  - `internal/gitdata/repo.go`
- Depends on foundation: tidak (epic ini tidak punya foundation; lihat overview bagian 4).
- Ketergantungan lunak: `docs/git/cli-findings.md` dan `internal/gitdata/testdata/` (milik fitur `git-cli-spike`) mungkin belum ada. Bila belum ada, pakai nilai awal di plan ini. JANGAN menunggu, JANGAN membuat berkas itu.

## Tujuan
Membangun paket baca-saja `internal/gitdata` (paket `gitdata`) yang menjadi dasar tiket 01 sampai 04:
1. Tipe ternormalisasi (Repo, MR, Pipeline, Job, dst.) dan fungsi pemetaan status GitLab/GitHub.
2. Allowlist eksplisit perintah baca `glab`/`gh`/`git`; selain itu ditolak dengan `CodeNotAllowed` SEBELUM proses dijalankan.
3. Runner `Run` dengan timeout 20 detik (via `connector.RunCLITimeout`) dan semaphore global 3 pemanggilan yang menghormati `ctx`.
4. Galat terklasifikasi dengan pesan Indonesia.
5. Deteksi repo dari remote git tiap project (`DetectRepos`, `RepoFlag`), termasuk parse URL ssh/https, penentuan provider, cache host login TTL sekitar 60 detik, dan penggabungan per `host/path`.

Tanpa unit test, tanpa `*_test.go`. Guard hanya build.

## Path yang dimiliki
HANYA berkas berikut yang boleh dibuat atau diubah:
- `internal/gitdata/doc.go`
- `internal/gitdata/types.go`
- `internal/gitdata/allowlist.go`
- `internal/gitdata/runner.go`
- `internal/gitdata/errors.go`
- `internal/gitdata/repo.go`

Dilarang menyentuh: `internal/connector/*` (termasuk `cli.go`, `gitlab.go`, `github.go`), `internal/gitdata/testdata/` dan `docs/git/` (milik spike), `cmd/**` (wiring dan IPC/ctl), `frontend/**`, `overview.md`, tiket, spec. Jangan membuat berkas lain di `internal/gitdata/` (termasuk stub `mrs.go`, `pipelines.go`, `mrdetail.go`: itu milik tiket 02 sampai 04). Jangan commit.

## Kontrak
Semua signature, nama tipe, nama field, tag json, konstanta, dan pesan galat PERSIS seperti overview bagian 2.3 (`types.go`, `errors.go`, `runner.go`/`allowlist.go`, `repo.go`). Ringkasan yang disediakan paket ini untuk fitur/tiket lain:
- `types.go`: `Provider` (+`ProviderGitLab`, `ProviderGitHub`), `RepoProject`, `Repo`, `Status` (+7 konstanta), `MergeRequest`, `Reviewer`, `PipelineRef`, `MergeRequestDetail`, `Pipeline`, `Job`, `Stage`, `PipelineDetail`, `NormalizeGitLabStatus(string) Status`, `NormalizeGitHubStatus(status, conclusion string) Status`.
- `errors.go`: `ErrorCode` + 7 konstanta (`CodeCLIMissing`, `CodeNotLoggedIn`, `CodePermission`, `CodeNotFound`, `CodeTimeout`, `CodeNotAllowed`, `CodeCLI`), `Error{Code, Message}`, `(*Error).Error()`, `AsError(err) (*Error, bool)`, privat `classify(err error, tool string) *Error`.
- `runner.go`: `CallTimeout = 20 * time.Second`, `MaxConcurrent = 3`, `Run(ctx, tool string, args ...string) ([]byte, error)`.
- `allowlist.go`: privat `check(tool string, args []string) *Error` (tiket 02 sampai 04 memakai allowlist ini TANPA mengubahnya, jadi cakupan flag harus lengkap, lihat langkah 4).
- `repo.go`: `ProjectInput{ID, Name, Path}`, `DetectRepos(ctx, []ProjectInput) ([]Repo, error)`, `RepoFlag(Repo) string`.

Yang dipakai: hanya stdlib dan `github.com/NooBeeID/shikigami/internal/connector` (`RunCLITimeout`, `IsCLIMissing`). Pesan galat `RunCLITimeout` yang dicocokkan `classify`:
- `ErrCLIMissing: <name>` (pakai `connector.IsCLIMissing(err)`)
- `perintah <name> melewati batas waktu`
- `perintah <name> gagal: <stderr terpotong 300 karakter>`

Aturan impor: `gitdata` mengimpor HANYA stdlib dan `internal/connector`. DILARANG mengimpor `internal/workspace`. Helper privat diberi nama bermakna agar tidak bentrok dengan berkas tiket lain: `check`, `classify`, `acquire`, `release`, `parseRemoteURL`, `remoteHost` dan sejenisnya; hindari nama generik seperti `run`, `parse`, `helper`, `fetch`, `decode`, `trim`.

## Langkah
Kerjakan berurutan. Buat folder `internal/gitdata/` bila belum ada (jangan menyentuh `testdata/` bila sudah ada). Semua berkas `package gitdata`, komentar boleh Indonesia, identifier Inggris. Jalankan `gofmt -l internal/gitdata` di akhir.

### 1. `doc.go`
Komentar paket (`// Package gitdata ...`) berbahasa Indonesia, memuat: paket baca-saja di atas `glab`/`gh`/`git`; hanya perintah lewat allowlist; arah dependensi `gitdata -> connector`; TIDAK mengimpor `workspace` (path dan id diteruskan pemanggil sebagai argumen); tipe ternormalisasi, bukan JSON mentah CLI. Hanya `package gitdata` + komentar.

### 2. `types.go`
Salin semua tipe dan konstanta dari overview 2.3 persis (nama field dan tag json camelCase, `omitempty` pada field opsional: `ReviewState`, `PipelineStatus`, `Mergeable`, `LastPipeline`, `PipelineRef.URL`, `Reviewer.State`, `Pipeline.Author`, `Job.URL`). Tulis satu field per baris dengan komentar singkat. Jangan menambah atau mengubah nama field.

Bila `docs/git/cli-findings.md` sudah ada saat Anda bekerja, hanya bentuk field opsional yang boleh disesuaikan sesuai temuan (dan catat di komentar kode); nama field tidak berubah.

Fungsi pemetaan (lowercase + `strings.TrimSpace` pada input dulu):
```go
func NormalizeGitLabStatus(s string) Status
func NormalizeGitHubStatus(status, conclusion string) Status
```
- GitLab: `created|waiting_for_resource|preparing|pending|manual|scheduled` -> `StatusPending`; `running` -> `StatusRunning`; `success` -> `StatusSuccess`; `failed` -> `StatusFailed`; `canceled|cancelled` -> `StatusCanceled`; `skipped` -> `StatusSkipped`; lainnya (termasuk kosong) -> `StatusUnknown`.
- GitHub: bila `status` salah satu `queued|waiting|requested|pending` -> `StatusPending`; `in_progress` -> `StatusRunning`; `completed` -> lihat `conclusion`: `success` -> `StatusSuccess`; `failure|timed_out|startup_failure` -> `StatusFailed`; `cancelled|canceled` -> `StatusCanceled`; `skipped|neutral` -> `StatusSkipped`; conclusion lain atau kosong -> `StatusUnknown`. Status lain (termasuk kosong) -> `StatusUnknown`. Catatan: bila `status` kosong tetapi `conclusion` terisi (bentuk `gh pr checks`/bucket), perlakukan seolah `status = "completed"`.

### 3. `errors.go`
Tipe dan konstanta sesuai overview 2.3. `Error` memakai pointer receiver: `func (e *Error) Error() string { return e.Message }`. `AsError` memakai `errors.As` (pola `internal/larkdata/errors.go`).

`classify(err error, tool string) *Error`:
1. `err == nil` -> nil. Bila `AsError(err)` ok -> kembalikan apa adanya.
2. `connector.IsCLIMissing(err)` -> `CodeCLIMissing`, Message `"<tool> belum terpasang"` (tool = `glab`/`gh`/`git`).
3. `errors.Is(err, context.DeadlineExceeded)`, `errors.Is(err, context.Canceled)` atau pesan memuat `melewati batas waktu` -> `CodeTimeout`, Message `"<Label> tidak merespons dalam 20 detik"`, Label = `GitLab` untuk `glab`, `GitHub` untuk `gh`, `Git` untuk `git`. (Tulis helper privat `providerLabel(tool string) string`; gunakan `CallTimeout` untuk angka detiknya via `int(CallTimeout.Seconds())` supaya konsisten.)
4. Selain itu: `raw := err.Error()`; `msg := strings.TrimSpace(strings.TrimPrefix(raw, "perintah "+tool+" gagal: "))`; `lower := strings.ToLower(raw)`. Hanya untuk tool `glab`/`gh` (untuk `git` lompat ke butir 5), cocokkan berurutan:
   - not_logged_in: `not logged in`, `not logged into`, `authentication`, `401`, `unauthorized`, `gh auth login`, `glab auth login`, `no token found`. Message: `"Belum login ke <Label>. Jalankan `<tool> auth login` di Pengaturan > Connector"`.
   - permission: `403`, `forbidden`, `permission`, `insufficient`, `scope`. Message `"Izin kurang: " + msg`.
   - not_found: `404`, `not found`, `could not resolve`, `could not find`. Message `"Repo tidak ditemukan atau tidak ada akses"`.
5. Untuk `git`: `not a git repository` atau `no such remote` -> `CodeNotFound` dengan Message `"Repo tidak ditemukan atau tidak ada akses"`.
6. Default: `CodeCLI`, Message `"<Label> gagal: " + ringkas(msg)`; ringkas = potong ke 200 rune (tambah `...`), dan ganti newline dengan spasi. Helper privat `summarizeMsg(s string) string`.

Urutan pencocokan: not_logged_in, lalu permission, lalu not_found, supaya `401` tidak jatuh ke permission. Pola dicocokkan lowercase (daftar pola sebagai `[]string` privat di tingkat paket, fungsi `matchAny` privat dinamai `containsAny` agar tidak bentrok). Bila `docs/git/cli-findings.md` sudah ada, tambah/sesuaikan pola dengan bagian "pola galat" di sana.

### 4. `allowlist.go`
Tabel eksplisit; tidak ada perintah tulis. Struktur privat:
```go
type spec struct {
    tool  string
    cmd   []string        // subcommand berurutan, mis. {"mr", "list"}
    flags map[string]bool // flag diizinkan (bentuk pendek dan panjang)
}
```
Gunakan helper privat `flagSet(names ...string) map[string]bool`.

Entri awal (sesuai overview 2.3; sesuaikan flag dengan `docs/git/cli-findings.md` bila sudah ada, dan JANGAN menambah subcommand tulis):

| tool | subcommand | flag diizinkan |
|---|---|---|
| glab | `auth status` | `--hostname` (jangan izinkan `-h`; di glab itu help) |
| gh | `auth status` | `--hostname`, `-h`, `--active`, `-a` |
| glab | `mr list` | `-R`, `--repo`, `-F`, `--output`, `-P`, `--per-page`, `-p`, `--page`, `--merged`, `--closed`, `--all`, `--draft`, `--source-branch`, `--target-branch`, `-A`, `--author`, `--label`, `-l` |
| glab | `mr view` | `-R`, `--repo`, `-F`, `--output`, `-c`, `--comments` |
| glab | `ci list` dan `pipeline list` | `-R`, `--repo`, `-F`, `--output`, `-P`, `--per-page`, `-p`, `--page`, `--status`, `-s`, `--ref`, `-r`, `--orderBy`, `-o`, `--sort` |
| glab | `ci get` | `-R`, `--repo`, `-F`, `--output`, `-b`, `--branch`, `-p`, `--pipeline-id`, `-d`, `--with-job-details` (`--with-variables` TIDAK diizinkan) |
| gh | `pr list` | `-R`, `--repo`, `--json`, `-L`, `--limit`, `-s`, `--state`, `-b`, `--base`, `-H`, `--head`, `-A`, `--author`, `--draft`, `-d`, `-S`, `--search`, `-l`, `--label` |
| gh | `pr view` | `-R`, `--repo`, `--json`, `-c`, `--comments` |
| gh | `run list` | `-R`, `--repo`, `--json`, `-L`, `--limit`, `-b`, `--branch`, `-s`, `--status`, `-w`, `--workflow`, `-e`, `--event`, `-u`, `--user`, `-c`, `--commit` |
| gh | `run view` | `-R`, `--repo`, `--json`, `-j`, `--job`, `--attempt` |
| git | `remote` | (tidak ada flag; hanya bentuk polos) |
| git | `remote get-url` | (tidak ada flag; satu argumen posisi = nama remote) |

Aturan `check(tool string, args []string) *Error` (kembalikan `nil` bila lolos):
1. `tool` harus `glab`, `gh`, atau `git`; selain itu `CodeNotAllowed` ("Perintah tidak diizinkan: tool tidak dikenal"). `args` kosong -> `CodeNotAllowed` ("Perintah kosong").
2. Khusus `git`: terima prefiks opsional tepat `-C <path>` (dua argumen; `<path>` tidak boleh kosong). Hanya satu `-C`. Flag/opsi git lain sebelum subcommand (mis. `-c`, `--exec-path`, `--git-dir`, `--work-tree`) ditolak. Sisa argumen harus persis `["remote"]` atau `["remote", "get-url", <nama>]` dengan `<nama>` satu argumen tidak diawali `-` dan tidak kosong. Argumen tambahan ditolak.
3. Untuk `glab`/`gh`: dua token pertama (subcommand) harus cocok dengan satu entri; tidak cocok -> `CodeNotAllowed` dengan pesan `"Perintah tidak diizinkan: <tool> <token1> <token2>"` (pakai token yang ada; tangani args < 2 tanpa panic). Perintah tulis (`mr create|merge|approve|close|update|note|rebase|delete`, `ci run|retry|cancel|trigger|delete`, `pr create|merge|review|close|edit|comment|checkout`, `run rerun|cancel|delete`, `api`, `auth login|logout|token|refresh`, dst.) otomatis ditolak karena tidak ada di tabel; JANGAN menulis daftar blokir sebagai pengganti tabel.
4. Sisa argumen (setelah dua token subcommand) diperiksa: setiap argumen yang diawali `-` harus ada di `flags` entri itu (untuk bentuk panjang `--nama=nilai`, cek bagian sebelum `=`). Argumen tepat `--` atau `-` ditolak. Argumen yang tidak diawali `-` dianggap nilai flag atau argumen posisi (nomor MR, id run, nama branch) dan diterima. Konsekuensi untuk pemanggil: kirim flag dan nilainya sebagai argumen terpisah (`"-R", "owner/repo"`) atau `--flag=nilai`; gabungan pendek seperti `-L5` ditolak.
5. Tolak eksplisit dengan pesan jelas flag berbahaya meski seandainya ikut tabel: `--show-token`, `-t` (auth status), `--web`, `-w` pada `pr view|mr view|run view` (membuka browser; `-w` pada `run list` adalah `--workflow` dan tetap sah), `--log`, `--log-failed`, `--with-variables`, `--editor`, `--body`, `--title`, `--yes`, `--force`. Implementasi: daftar `deniedFlags` privat berisi bentuk panjang di atas dan `-t`, diperiksa SEBELUM pencocokan tabel. `-w` tidak masuk `deniedFlags`: pada `gh run list` itu `--workflow` yang sah, sedangkan pada `pr view`/`mr view`/`run view` ia ditolak otomatis karena tidak ada di `flags` entri itu.
6. Pesan `Error.Message` Indonesia dan tidak membocorkan nilai argumen selain nama flag/subcommand.

### 5. `runner.go`
```go
const CallTimeout = 20 * time.Second
const MaxConcurrent = 3

var runSlots = make(chan struct{}, MaxConcurrent)

func acquire(ctx context.Context) error  // select { case runSlots <- struct{}{}: return nil; case <-ctx.Done(): return ctx.Err() }
func release()                           // <-runSlots

func Run(ctx context.Context, tool string, args ...string) ([]byte, error)
```
Alur `Run`:
1. `if e := check(tool, args); e != nil { return nil, e }` (JANGAN mengembalikan `*Error` nil sebagai `error` non-nil; selalu bandingkan `e != nil` dulu). Ini terjadi sebelum semaphore dan sebelum proses apa pun.
2. `acquire(ctx)`; bila gagal -> `return nil, &Error{Code: CodeTimeout, Message: <pesan timeout tool>}` (pakai `classify(ctx.Err(), tool)`). `defer release()` sesudah sukses.
3. `out, err := connector.RunCLITimeout(ctx, CallTimeout, tool, args...)`. Bila `err != nil`: bila `ctx.Err() != nil` kembalikan galat timeout terklasifikasi; selain itu `return nil, classify(err, tool)`.
4. Sukses: `return []byte(out), nil` (stdout apa adanya; TIDAK divalidasi sebagai JSON karena `git` mengembalikan teks; pemanggil tiket lain yang mem-parse JSON).
5. Jangan mengubah direktori kerja untuk `glab`/`gh` (selalu `-R`); `git` memakai `-C <path>` yang dikirim pemanggil. Jangan menambah flag otomatis (berbeda dari `larkdata.Run`): pemanggil mengirim flag lengkap.

### 6. `repo.go`
```go
type ProjectInput struct{ ID, Name, Path string }
func DetectRepos(ctx context.Context, projects []ProjectInput) ([]Repo, error)
func RepoFlag(r Repo) string
```
Helper privat (nama bermakna):

a. `parseRemoteURL(raw string) (host, path string, ok bool)`:
   - Trim spasi. Bentuk scp-like (tidak mengandung `://`, memuat `:`; bagian sebelum `:` adalah `[user@]host`): `git@host:owner/repo.git` -> host = bagian setelah `@` (atau seluruh bagian bila tanpa `@`), path = bagian setelah `:`. Waspadai path lokal absolut/Windows (`/...`, `C:\...`): bila bagian sebelum `:` kosong, mengandung `/` atau `\`, atau panjangnya 1 -> `ok=false`.
   - Bentuk berskema (`ssh://`, `https://`, `http://`, `git://`): gunakan `net/url`; host = `u.Hostname()` (buang kredensial dan port), path = `u.Path`.
   - Normalisasi: host lowercase; path `strings.Trim(path, "/")`, buang sufiks `.git` (setelah trim slash ujung; jalankan trim slash lagi), buang `/` awal. Subgroup TIDAK dipotong. Host kosong atau path tanpa minimal dua segmen (`owner/repo`) -> `ok=false`.

b. `readRemoteURL(ctx, dir string) (string, bool)`: 
   - `Run(ctx, "git", "-C", dir, "remote")`; galat apa pun -> `("", false)` (diabaikan diam-diam).
   - Pecah per baris, `TrimSpace`, buang yang kosong. Tidak ada remote -> `false`. Pilih `origin` bila ada, jika tidak remote pertama.
   - `Run(ctx, "git", "-C", dir, "remote", "get-url", name)`; ambil baris pertama yang di-trim; galat/kosong -> `false`.

c. Cache host login (paket-level, dilindungi `sync.Mutex`):
   ```go
   const hostCacheTTL = 60 * time.Second
   type loginHosts struct{ gitlab, github map[string]bool }
   var hostCache struct { mu sync.Mutex; at time.Time; val loginHosts; valid bool }
   func cachedLoginHosts(ctx context.Context) loginHosts
   ```
   `cachedLoginHosts`: kunci mutex; bila `valid && time.Since(at) < hostCacheTTL` kembalikan `val`; selain itu jalankan `Run(ctx, "glab", "auth", "status")` dan `Run(ctx, "gh", "auth", "status")`, parse masing-masing dengan `parseLoginHosts(text string) []string`, simpan hasil (set kosong bila CLI hilang/galat) dengan `at = time.Now()`, kembalikan. Jangan simpan hasil bila `ctx.Err() != nil` (jangan meracuni cache saat dibatalkan). Hanya dipanggil bila ada host selain `github.com`/`gitlab.com` (lazy; jangan panggil bila tak perlu).
   - Sumber teks: stdout bila ada; bila `Run` mengembalikan galat, pakai `err.Error()` (pada exit non-nol stderr berisi teks status, lihat overview 2.2). Catatan keterbatasan: `RunCLITimeout` membuang stderr saat exit 0, sehingga bila CLI menulis status ke stderr, host tidak terbaca; ini diterima (host self-hosted tidak dikenali = diabaikan diam-diam) dan diselaraskan di fase Wiring bila temuan spike menuntut.
   - `parseLoginHosts(text)`: tahan keluaran tak terduga (tanpa panic; keluaran tak terparse -> slice kosong). Algoritma awal: per baris, lewati baris yang mengandung `not logged in` atau `not logged into` (case-insensitive); ekstrak host dengan regexp `(?i)logged in to\s+(\S+)` (buang tanda baca ujung seperti `.`/`,`); host lowercase, deduplikasi. Sesuaikan dengan format di `docs/git/cli-findings.md` bila ada. Parser ini PRIVAT milik `gitdata` (jangan mengimpor parser adapter).

d. `providerFor(host string, hosts func() loginHosts) (Provider, bool)`: `github.com` -> `ProviderGitHub`; `gitlab.com` -> `ProviderGitLab` (tanpa cek login); host lain: ambil `hosts()` (lazy, memanggil `cachedLoginHosts` maksimal sekali per `DetectRepos`); bila `gitlab[host]` -> GitLab (diutamakan), lalu `github[host]` -> GitHub; selain itu `false`.

e. `DetectRepos`:
   1. Hasil awal `[]Repo{}` non-nil. `projects` kosong -> kembalikan slice kosong, `nil`.
   2. Tahap 1 (paralel): untuk tiap project dengan `Path != ""`, jalankan goroutine yang memanggil `readRemoteURL` lalu `parseRemoteURL`, tulis ke slot hasil berindeks sama dengan input (tanpa data race; gunakan `sync.WaitGroup`). Batas 3 proses sudah dijamin semaphore di `Run`. Hormati `ctx`.
   3. Tahap 2 (berurutan sesuai urutan input): untuk tiap project yang punya `(host, path)`, tentukan provider via `providerFor` (tidak cocok -> lewati diam-diam). Gabungkan dengan kunci `host + "/" + path` memakai `map[string]int` (indeks ke slice hasil) agar urutan hasil stabil mengikuti project pertama tiap repo. Project pertama pada kunci itu mengisi `ProjectID`, `ProjectName`; setiap project (termasuk yang pertama) ditambahkan ke `Projects` (`RepoProject{ID, Name}`) sesuai urutan input. `WebURL = "https://" + host + "/" + path`. `Provider`, `Host`, `Path` terisi.
   4. Galat satu project tidak menggagalkan project lain. `DetectRepos` mengembalikan galat hanya bila `ctx.Err() != nil` (kembalikan `classify(ctx.Err(), "git")`); selain itu `nil` error. Slice hasil selalu non-nil.

f. `RepoFlag(r Repo) string`: `r.Host` adalah `gitlab.com` atau `github.com` -> `r.Path`; selain itu `r.Host + "/" + r.Path`.

### 7. Verifikasi lokal (guard)
- `cd /Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal && gofmt -l internal/gitdata` (harus kosong untuk berkas milik fitur ini)
- `go build ./internal/gitdata/...` lalu `go vet ./internal/gitdata/...`
- `go build ./...` bila lulus utuh (kegagalan akibat berkas milik fitur lain yang belum ada, mis. `wire_git.go` mereferensikan gitdata yang belum ada, bukan masalah fitur ini; laporkan saja)
- `grep -rn "internal/workspace" internal/gitdata` harus kosong.
- `grep -rn "show-token" internal/gitdata` hanya boleh muncul pada daftar penolakan di `allowlist.go`.

## Perubahan
Dibuat (semua baru):
- `internal/gitdata/doc.go`
- `internal/gitdata/types.go`
- `internal/gitdata/allowlist.go`
- `internal/gitdata/runner.go`
- `internal/gitdata/errors.go`
- `internal/gitdata/repo.go`

Tidak ada berkas yang diubah atau dihapus. Tidak ada berkas test.

## Wiring
Fitur ini tidak menyambungkan apa pun sendiri (paket pustaka murni; tanpa `init()` yang mendaftarkan sesuatu). Yang diekspos untuk langkah wiring dan fitur lain:
- `gitdata.DetectRepos`, `gitdata.ProjectInput`, `gitdata.Repo`, `gitdata.RepoProject`, `gitdata.Provider*`: dipakai `cmd/shikigamid/wire_git.go` (fitur `git-repos-ipc-ctl`, `gitResolveRepo`, handler `git.repos.list`).
- `gitdata.Error`, `gitdata.ErrorCode`, `gitdata.Code*`, `gitdata.AsError`: dipakai `mapGitError` di `wire_git.go`.
- `gitdata.Run`, `gitdata.RepoFlag`, tipe MR/Pipeline, `NormalizeGitLabStatus`, `NormalizeGitHubStatus`: dipakai tiket 02 sampai 04.
- Item untuk fase Wiring (overview bagian 5, butir 6 dan 7): cocokkan flag allowlist, pola `classify`, pemetaan status, dan parser `parseLoginHosts` dengan `docs/git/cli-findings.md` bila berubah; pastikan `grep -rn "internal/workspace" internal/gitdata` kosong dan tidak ada perintah tulis di allowlist. Catat keterbatasan stderr-pada-exit-0 untuk deteksi host self-hosted bila relevan.

## Selesai bila
- [ ] Enam berkas ada di `internal/gitdata/` dan tidak ada berkas lain yang dibuat/diubah oleh fitur ini.
- [ ] Tipe, nama field, tag json camelCase, dan 7 konstanta `Status` persis seperti overview 2.3; `NormalizeGitLabStatus` dan `NormalizeGitHubStatus` ada dengan pemetaan di langkah 2.
- [ ] `Run` menjalankan `check` lebih dulu: perintah di luar allowlist (termasuk `mr create`, `pr merge`, `run rerun`, `api`, `--show-token`) menghasilkan `*Error` `CodeNotAllowed` tanpa menjalankan proses dan tanpa menyentuh semaphore; tidak ada perintah tulis di tabel.
- [ ] `CallTimeout = 20s`, `MaxConcurrent = 3`; semaphore menunggu dengan menghormati `ctx` (batal saat menunggu -> `CodeTimeout`).
- [ ] `classify` menghasilkan pesan Indonesia untuk `cli_missing`, `not_logged_in`, `permission`, `not_found`, `timeout`, `not_allowed`, `cli_error` sesuai overview 2.3.
- [ ] `DetectRepos`: ssh dan https, `origin` atau remote pertama, `github.com`/`gitlab.com` tanpa cek login, host lain hanya bila CLI login (GitLab diutamakan, cache ~60 detik), subgroup tidak dipotong, project tanpa remote atau path salah diabaikan, penggabungan per `host/path`, slice hasil non-nil; `RepoFlag` benar untuk host bawaan dan self-hosted.
- [ ] `gofmt -l internal/gitdata` kosong; `go build ./internal/gitdata/...` dan `go vet ./internal/gitdata/...` sukses (guard = build path fitur ini sukses; `go build ./...` dijalankan dan kegagalan karena berkas fitur lain dilaporkan, bukan diperbaiki).
- [ ] `grep -rn "internal/workspace" internal/gitdata` kosong; tidak ada `*_test.go`; tidak ada token/secret.
