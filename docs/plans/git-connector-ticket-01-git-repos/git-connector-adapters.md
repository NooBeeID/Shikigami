# Plan: Adapter connector gitlab dan github

## Metadata
- Tiket: `git-connector-ticket-01-git-repos` (`docs/tickets/git-connector-ticket-01-git-repos.md`)
- Fitur: `git-connector-adapters` (Adapter connector gitlab dan github)
- ownedPaths: `internal/connector/gitlab.go`, `internal/connector/github.go`
- Depends on foundation: tidak. Tidak ada foundation di epic ini, dan fitur ini tidak memakai `internal/gitdata`. Ia hanya memakai `connector.RunCLI`, `IsCLIMissing`, `Register`, `Status`, `Guide` yang sudah ada di `internal/connector`.
- Ketergantungan lunak: `docs/git/cli-findings.md` (milik fitur spike) belum ada saat plan ini ditulis. Plan ini memakai format keluaran yang sudah diverifikasi langsung di mesin pengembang (lihat Kontrak). Bila berkas temuan sudah ada saat Anda bekerja, cocokkan pola parsing dengannya; bila berbeda, ikuti temuan dan catat selisihnya di laporan akhir. Jangan menunggu berkas itu.

## Tujuan
Mendaftarkan dua connector, `gitlab` (CLI `glab`) dan `github` (CLI `gh`), di kerangka Connector yang ada. `Status()` membaca `glab auth status` / `gh auth status` dan memetakan ke `cli_missing`, `not_logged_in`, `connected` (dengan `User`). Parsing harus tahan keluaran tak terduga (tidak panic). Tidak ada token, output mentah CLI, atau cache. `ConnectGuide()` memberi langkah Indonesia dan perintah login. Hasilnya otomatis muncul di Pengaturan > Connector tanpa perubahan frontend atau kerangka.

## Path yang dimiliki
HANYA dua berkas ini yang boleh dibuat/diubah:
- `internal/connector/gitlab.go`
- `internal/connector/github.go`

Jangan sentuh `internal/connector/cli.go`, `registry.go`, `types.go`, `lark.go`, `fake.go`, `ipc.go`, `internal/gitdata/*`, `cmd/*`, `frontend/*`, atau berkas wiring/path fitur lain. Jangan membuat `*_test.go`. Jangan commit. Jangan mengubah `overview.md`.

## Kontrak

### Yang dipakai (sudah ada, jangan diubah)
Dari `internal/connector` (package `connector`):
- `type Connector interface { ID() string; Name() string; Status(ctx context.Context) (Status, error); ConnectGuide() Guide }`
- `type Status struct { State State; User string; Message string }` dan konstanta `StateCLIMissing`, `StateNotLoggedIn`, `StateConnected`.
- `type Guide struct { Steps []string; Commands []string }`
- `func Register(c Connector)` (dipanggil dari `init()`).
- `func RunCLI(ctx context.Context, name string, args ...string) (string, error)` (timeout 8 detik, PATH login-shell, mengembalikan stdout; saat exit non-nol galatnya `perintah <name> gagal: <stderr terpotong 300 karakter>`, stdout hilang).
- `func IsCLIMissing(err error) bool`.
- Pola acuan: `internal/connector/lark.go` (tipe privat, `init()`, `Status`, `ConnectGuide`).

### Yang disediakan (sesuai overview bagian 2.2)
```go
// gitlab.go
type gitlabConnector struct{}
func init() { Register(gitlabConnector{}) }
func (gitlabConnector) ID() string   // "gitlab"
func (gitlabConnector) Name() string // "GitLab"
func (gitlabConnector) Status(ctx context.Context) (Status, error)
func (gitlabConnector) ConnectGuide() Guide

// github.go
type githubConnector struct{}   // ID "github", Name "GitHub"; bentuk sama
func init() { Register(githubConnector{}) }
```
Plus dua helper privat (tidak diekspor) di `gitlab.go` yang dipakai kedua adapter, nama persis agar tidak bentrok dengan berkas lain di package `connector`:
```go
// loginHost = satu host yang login beserta nama akunnya.
type gitLoginHost struct{ Host, User string }

// parseGitLoginHosts mengurai keluaran teks `glab|gh auth status`.
func parseGitLoginHosts(text string) []gitLoginHost

// pickGitLoginHost memilih host utama: preferred bila ada, selain itu yang pertama.
func pickGitLoginHost(hosts []gitLoginHost, preferred string) (gitLoginHost, bool)

// looksNotLoggedIn true bila teks galat/keluaran menandakan belum login.
func looksNotLoggedIn(text string) bool
```
(Satu fitur memiliki kedua berkas, jadi `github.go` boleh memakai helper dari `gitlab.go`. Nama diawali `git`/`looks` supaya aman dari bentrok dengan helper `lark.go`/`cli.go`; cek dengan `grep -n "func parseGitLoginHosts\|func pickGitLoginHost\|func looksNotLoggedIn\|type gitLoginHost" internal/connector/*.go` bahwa nama belum dipakai.)

### Format keluaran yang diverifikasi
- `glab auth status` (glab 1.97.0), login ke `gitlab.com`, exit 0:
  ```
  gitlab.com
    ✓ Logged in to gitlab.com as <akun> (<path config>)
    ✓ Git operations for gitlab.com configured to use https protocol.
    ...
    ✓ Token found: ****
  ```
  Baris host berdiri sendiri tanpa indentasi; baris login memuat `Logged in to <host> as <akun>`. Baris `Token found: ***` adalah topeng; JANGAN pernah dikembalikan atau dicatat. JANGAN memakai `--show-token`.
- `gh auth status` (gh 2.102.0) belum login: exit 1 dan teks `You are not logged into any GitHub hosts. To log in, run: gh auth login`. `RunCLI` mengubahnya menjadi galat `perintah gh gagal: <teks stderr>`, sehingga teks itu ada di `err.Error()` bukan di stdout.
- `gh auth status` login (ASUMSI dari dokumentasi gh, belum diverifikasi di mesin ini): `github.com` lalu `  ✓ Logged in to github.com account <akun> (keyring)` (versi lama: `Logged in to github.com as <akun> (oauth_token)`). Parser harus menerima kedua kata sambung `as` dan `account`.
- Pada multi-host, satu host bisa gagal (baris `X Failed to log in to ...`, exit non-nol) sementara host lain sukses. Karena pada exit non-nol stdout hilang di `RunCLI`, tetap periksa teks galat: bila berisi `Logged in to`, perlakukan sebagai terhubung.

### Pemetaan Status
| Kondisi | Hasil |
|---|---|
| `IsCLIMissing(err)` | `Status{State: StateCLIMissing, Message: "glab belum terpasang"}` / `"gh belum terpasang"`, `nil` |
| sukses, ada host login terparse | `Status{State: StateConnected, User: <akun host utama>}`, `nil` |
| sukses, tidak ada host login / keluaran kosong / tak terparse | `Status{State: StateNotLoggedIn, Message: "Belum login ke GitLab. Ikuti langkah di dialog Connect."}` (GitHub: "GitHub"), `nil` |
| galat, teksnya memuat `Logged in to` | parse teks galat; bila ada host -> `StateConnected` (kasus sebagian host gagal) |
| galat, `looksNotLoggedIn(err.Error())` | `StateNotLoggedIn` + pesan di atas, `nil` |
| galat lain (timeout, dll.) | `Status{}`, `fmt.Errorf("glab auth status: %w", err)` / `"gh auth status: %w"` |

Host utama: `gitlab.com` untuk GitLab, `github.com` untuk GitHub bila ada di daftar, selain itu host pertama. `User` = akun host utama (boleh kosong bila tak terbaca, state tetap `connected`).

Pola `looksNotLoggedIn` (lowercase, `strings.Contains`, any-of): `not logged in`, `not logged into`, `no token found`, `not authenticated`, `authentication required`, `gh auth login`, `glab auth login`. (Catatan: dua pola terakhir cukup spesifik karena dipakai pada teks galat status, bukan output sukses.)

## Langkah

1. Verifikasi awal (hanya baca). Jalankan `ls internal/connector`, baca `lark.go`, `types.go`, `cli.go` bila perlu, dan `grep` nama helper di atas untuk memastikan tidak bentrok. Cek apakah `docs/git/cli-findings.md` sudah ada; bila ada, baca bagian host login dan pola galat, dan sesuaikan regex/pola di bawah.

2. Buat `internal/connector/gitlab.go`:
   - `package connector`; impor `context`, `fmt`, `regexp`, `strings`.
   - Deklarasikan `var gitLoginRe = regexp.MustCompile(`(?i)logged in to\s+(\S+)\s+(?:as|account)\s+(\S+)`)`. Nama akun diambil `\S+` lalu dibersihkan dari tanda baca ujung (`strings.Trim(user, "()[],.;:")`).
   - Deklarasikan `var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")` (nama privat `gitAnsiRe` agar unik) dan buang kode warna dari teks sebelum parsing.
   - `type gitLoginHost struct{ Host, User string }`.
   - `func parseGitLoginHosts(text string) []gitLoginHost`: buang ANSI, `strings.Split` per baris, untuk tiap baris pakai `gitLoginRe.FindStringSubmatch`; bila cocok, host = grup 1 (trim tanda baca ujung), user = grup 2; lewati host kosong atau duplikat (pertahankan urutan kemunculan). Tidak boleh panic pada input kosong, biner, atau sangat panjang; tidak mengembalikan teks mentah. Kembalikan slice (boleh nil).
   - `func pickGitLoginHost(hosts []gitLoginHost, preferred string) (gitLoginHost, bool)`: kembalikan entri dengan `Host == preferred` bila ada, selain itu `hosts[0]`; `false` bila kosong.
   - `func looksNotLoggedIn(text string) bool` memakai daftar pola di Kontrak.
   - `type gitlabConnector struct{}`; `func init() { Register(gitlabConnector{}) }`.
   - `ID()` -> `"gitlab"`, `Name()` -> `"GitLab"`.
   - `Status(ctx)`:
     ```go
     out, err := RunCLI(ctx, "glab", "auth", "status")
     text := out
     if err != nil {
         if IsCLIMissing(err) { return Status{State: StateCLIMissing, Message: "glab belum terpasang"}, nil }
         text = err.Error()
         if !strings.Contains(strings.ToLower(text), "logged in to") {
             if looksNotLoggedIn(text) { return notLoggedInGitLab(), nil }
             return Status{}, fmt.Errorf("glab auth status: %w", err)
         }
     }
     h, ok := pickGitLoginHost(parseGitLoginHosts(text), "gitlab.com")
     if !ok { return notLoggedInGitLab(), nil }
     return Status{State: StateConnected, User: h.User}, nil
     ```
     dengan helper privat `notLoggedInGitLab() Status` (nama unik) yang mengembalikan `StateNotLoggedIn` dan Message `"Belum login ke GitLab. Ikuti langkah di dialog Connect."`. Jangan memasukkan `out`/`text`/`err` ke `Status`. Tidak ada variabel paket yang menyimpan hasil (tanpa cache).
   - `ConnectGuide()`:
     ```go
     Guide{
         Steps: []string{
             "Pasang glab (GitLab CLI) bila belum terpasang di mesin ini.",
             "Salin perintah di bawah, lalu jalankan di terminal untuk login ke GitLab.",
             "Kembali ke sini dan tekan Cek ulang.",
         },
         Commands: []string{"glab auth login"},
     }
     ```

3. Buat `internal/connector/github.go`:
   - `package connector`; impor `context`, `fmt`, `strings`.
   - `type githubConnector struct{}`; `func init() { Register(githubConnector{}) }`; `ID()` -> `"github"`, `Name()` -> `"GitHub"`.
   - `Status(ctx)` bentuk sama dengan GitLab, tetapi `RunCLI(ctx, "gh", "auth", "status")`, pesan CLI hilang `"gh belum terpasang"`, host pilihan `"github.com"`, awalan galat `"gh auth status"`, helper `notLoggedInGitHub()` dengan Message `"Belum login ke GitHub. Ikuti langkah di dialog Connect."`. Gunakan ulang `parseGitLoginHosts`, `pickGitLoginHost`, `looksNotLoggedIn` dari `gitlab.go` (jangan menduplikasi).
   - `ConnectGuide()`: Steps `"Pasang gh (GitHub CLI) bila belum terpasang di mesin ini."`, `"Salin perintah di bawah, lalu jalankan di terminal untuk login ke GitHub."`, `"Kembali ke sini dan tekan Cek ulang."`; `Commands: []string{"gh auth login"}`.

4. Format dan build: jalankan `gofmt -l internal/connector` (harus kosong untuk dua berkas ini; perbaiki dengan `gofmt -w` hanya pada dua berkas ini) lalu `go vet ./internal/connector/` dan `go build ./...` dari root repo.

5. Pemeriksaan manual sekali jalan (tanpa menambah berkas test): bila `shikigamictl`/daemon bisa dibangun lokal, `go build ./...` sudah cukup. Opsional, bila praktis: skrip sementara di direktori scratchpad (BUKAN di repo) yang memanggil `parseGitLoginHosts` tidak mungkin karena privat; jadi cukup tinjau kode dengan kasus mental: keluaran `glab` di atas -> host `gitlab.com`, user akun; teks `You are not logged into any GitHub hosts...` pada galat -> `not_logged_in`; teks kosong -> `not_logged_in`; teks acak/biner -> `not_logged_in` tanpa panic. Jangan menjalankan `glab/gh auth status --show-token` dan jangan menempel keluaran asli (akun, path config) ke berkas repo atau laporan.

6. Pemeriksaan keamanan: `grep -n "show-token\|token" internal/connector/gitlab.go internal/connector/github.go` hanya boleh menemukan komentar/pola `no token found`; tidak ada pencatatan atau pengembalian output mentah CLI.

## Perubahan
- Dibuat: `internal/connector/gitlab.go` (connector GitLab + helper privat `gitLoginHost`, `parseGitLoginHosts`, `pickGitLoginHost`, `looksNotLoggedIn`, `notLoggedInGitLab`, regex privat).
- Dibuat: `internal/connector/github.go` (connector GitHub + `notLoggedInGitHub`).
- Tidak ada berkas lain yang dibuat atau diubah.

## Wiring
Tidak ada yang disambungkan oleh fitur ini. Yang diekspos untuk fase Wiring (jangan dikerjakan sekarang):
- Dua connector terdaftar otomatis lewat `init()` di package `connector`: `connector.Get("gitlab")` (Name "GitLab") dan `connector.Get("github")` (Name "GitHub"). IPC `connector.list|status|guide` dan Pengaturan > Connector memuatnya tanpa perubahan.
- Fase Wiring memeriksa bahwa package `internal/connector` termuat di binary `shikigamid` (sama seperti `fake` dan `lark`), menjalankan `shikigamictl connector status gitlab|github`, membuka dialog Connect, dan menyelaraskan pola parsing dengan `docs/git/cli-findings.md` (overview bagian 5 butir 1 dan 6). Format login `gh` yang sebenarnya masih ASUMSI sampai `gh auth login` dijalankan; itu butir terbuka, tidak memblokir.

## Selesai bila
- `internal/connector/gitlab.go` dan `internal/connector/github.go` ada; `gofmt -l` pada keduanya kosong.
- `go build ./...` dari root repo lulus (guard utama; tanpa unit test, tanpa `*_test.go`). Bila build gagal karena fitur paralel lain (mis. `internal/gitdata` belum ada), pastikan `go build ./internal/connector/` lulus dan laporkan sebabnya; jangan membuat stub di path fitur lain.
- `go vet ./internal/connector/` lulus.
- `connector.Get("gitlab").Name()` = "GitLab" dan `connector.Get("github").Name()` = "GitHub" (terdaftar via `init()`).
- Pemetaan sesuai tabel: CLI tak ada -> `cli_missing` ("glab belum terpasang"/"gh belum terpasang"); keluaran tanpa host login atau tak terparse, atau galat bertanda belum login -> `not_logged_in`; host login terparse -> `connected` dengan `User` (host utama `gitlab.com`/`github.com`, selain itu host pertama); galat lain dikembalikan sebagai `error`.
- `Status` tidak memuat token atau output mentah CLI; tidak ada cache atau variabel paket yang menyimpan hasil; `--show-token` tidak dipakai.
- `ConnectGuide()` berisi tiga langkah Indonesia dan `Commands` `["glab auth login"]` / `["gh auth login"]`.
- Tidak ada berkas lain di luar dua ownedPaths yang berubah (`git status` hanya menunjukkan dua berkas ini dari fitur ini).
