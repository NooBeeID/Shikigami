# Plan: shikigamictl git mr

## Metadata

- Tiket: `git-connector-ticket-04-git-mr-detail` (`docs/tickets/git-connector-ticket-04-git-mr-detail.md`)
- Fitur: `git-mr-detail-ctl` (shikigamictl git mr)
- ownedPaths: `cmd/shikigamictl/cmd_git_mr_detail.go`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; `hasFoundation=false`)
- Rujukan kontrak: `docs/plans/git-connector-ticket-04-git-mr-detail/overview.md` bagian 2.1 (IPC `git.mr.get`) dan 2.4 (ctl).

## Tujuan

Menambah subcommand alat debug `shikigamictl git mr <workspaceId> <projectId> <number>`. Subcommand memanggil IPC daemon `git.mr.get`, lalu mencetak detail MR/PR sebagai teks, satu informasi per baris. Registrasi mandiri dari `init()` lewat `registerGitSub("mr", ...)` tanpa mengubah `cmd_git.go`. File harus terkompilasi tanpa backend (fitur `git-mr-detail-data` belum tentu selesai) dan TANPA mengimpor `internal/gitdata`.

## Path yang dimiliki

HANYA file ini yang boleh dibuat atau diubah:
- `cmd/shikigamictl/cmd_git_mr_detail.go`

Jangan menyentuh file wiring atau path fitur lain. Khususnya jangan ubah `cmd_git.go`, `cmd_git_mrs.go`, `cmd_git_pipelines.go`, `registry.go`, `main.go`, `internal/gitdata/*`, `internal/ipc/*`, `cmd/shikigamid/*`, frontend, maupun `overview.md`. Membaca dan memanggil `registerGitSub`, `gitSubFunc` (di `cmd_git.go`) diperbolehkan. Jangan menulis unit test.

## Kontrak

Dipakai (sudah ada, jangan didefinisikan ulang):
- `registerGitSub(name string, fn gitSubFunc)` di `cmd/shikigamictl/cmd_git.go`; `gitSubFunc = func(ctx context.Context, c *ipc.Client, args []string) error`. Panic bila nama kembar, jadi `mr` harus unik (terpakai saat ini: `repos`, `mrs`, `pipelines`, `pipeline`).
- `(*ipc.Client).Call(method string, params any, out any) error` dari `github.com/NooBeeID/shikigami/internal/ipc`. Galat daemon dikembalikan sebagai `*ipc.Error{Code, Message}`. Metode tak dikenal daemon menghasilkan `Code == ipc.ErrMethodNotFound` (-32601), dideteksi dengan `errors.As`.
- Pola acuan: `cmd/shikigamictl/cmd_git_mrs.go` (argumen posisional, usage, `gitMrsOneLine`, `gitMrsNumber`). Simbol di file itu TIDAK boleh dipakai ulang atau diubah; buat padanan sendiri berawalan `gitMrDetail`.

IPC yang dipanggil (overview 2.1):
- `git.mr.get`, params `{"workspaceId": string, "projectId": string, "number": int}`. Hasil: OBJEK `MergeRequestDetail` langsung (bukan pembungkus), camelCase: `number, title, author, sourceBranch, targetBranch, state, draft, reviewState, pipelineStatus, updatedAt, url, description, labels[], reviewers[{name,state}], mergeable?, lastPipeline?{id,status,url?}`. Field opsional bisa hilang.

Disediakan (semua simbol baru berawalan `gitMrDetail`; tidak bentrok dengan `gitMrs*`/`gitRepos*`/`gitPipeline*`):
- `gitMrDetailUsage` (const string)
- `gitMrDetailReviewer`, `gitMrDetailPipeline`, `gitMrDetailResult` (tipe lokal)
- `gitMrDetailOneLine` (`*strings.Replacer`), `gitMrDetailNumber`, `gitMrDetailFormat`
- `gitMrDetail` (handler, bertipe `gitSubFunc`)

## Langkah

Buat `cmd/shikigamictl/cmd_git_mr_detail.go`, `package main`. Impor: `context`, `errors`, `fmt`, `strconv`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`. Tanpa `gitdata`, tanpa `flag`.

1. Konstanta usage, persis:
   ```go
   const gitMrDetailUsage = "penggunaan: shikigamictl git mr <workspaceId> <projectId> <number>"
   ```

2. Tipe lokal untuk decode:
   ```go
   type gitMrDetailReviewer struct {
       Name  string `json:"name"`
       State string `json:"state"`
   }
   type gitMrDetailPipeline struct {
       ID     string `json:"id"`
       Status string `json:"status"`
       URL    string `json:"url"`
   }
   type gitMrDetailResult struct {
       Number         int                   `json:"number"`
       Title          string                `json:"title"`
       Author         string                `json:"author"`
       SourceBranch   string                `json:"sourceBranch"`
       TargetBranch   string                `json:"targetBranch"`
       State          string                `json:"state"`
       Draft          bool                  `json:"draft"`
       ReviewState    string                `json:"reviewState"`
       PipelineStatus string                `json:"pipelineStatus"`
       UpdatedAt      string                `json:"updatedAt"`
       URL            string                `json:"url"`
       Description    string                `json:"description"`
       Labels         []string              `json:"labels"`
       Reviewers      []gitMrDetailReviewer `json:"reviewers"`
       Mergeable      string                `json:"mergeable"`
       LastPipeline   *gitMrDetailPipeline  `json:"lastPipeline"`
   }
   ```

3. `var gitMrDetailOneLine = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")` untuk membersihkan field satu-baris (BUKAN untuk deskripsi).

4. `func gitMrDetailNumber(r gitMrDetailResult) string`: `!<n>` bila `r.URL` memuat `/-/merge_requests/`, `#<n>` bila memuat `/pull/`, selain itu `<n>` polos (seperti `gitMrsNumber`).

5. `func gitMrDetailFormat(r gitMrDetailResult) string` mengembalikan teks lengkap (diakhiri newline), baris berurutan:
   - `<nomor> <judul>` (judul lewat `gitMrDetailOneLine`)
   - `status: <state>` ditambah ` (draft)` bila `r.Draft`
   - `penulis: @<author>`
   - `branch: <source> -> <target>`
   - `label: a, b` (join `", "`, tiap label dibersihkan), atau `label: -` bila kosong
   - `reviewer: <nama> (<state>), <nama> (<state>)`; state kosong dicetak `(-)`; atau `reviewer: -` bila kosong
   - `mergeable: <nilai>` atau `mergeable: -` bila kosong
   - `pipeline: <id> <status> <url>` (bagian url dilewati bila kosong), atau `pipeline: -` bila `LastPipeline == nil` atau `ID == ""`
   - `url: <url>`
   - baris kosong
   - `deskripsi:` lalu isi `r.Description` APA ADANYA (tab/newline dipertahankan; bila kosong cetak `-`). Pastikan keluaran berakhir tepat dengan satu newline (`strings.TrimRight(desc, "\n")` lalu tambah `"\n"`).
   Bangun dengan `strings.Builder`/`fmt.Fprintf`.

6. Handler:
   ```go
   func gitMrDetail(_ context.Context, c *ipc.Client, args []string) error
   ```
   - `len(args) != 3` -> `errors.New(gitMrDetailUsage)`.
   - `ws := strings.TrimSpace(args[0])`, `project := strings.TrimSpace(args[1])`; kosong -> usage.
   - `n, err := strconv.Atoi(strings.TrimSpace(args[2]))`; `err != nil || n <= 0` -> usage (`errors.New(gitMrDetailUsage)`).
   - `var res gitMrDetailResult`; `err := c.Call("git.mr.get", map[string]any{"workspaceId": ws, "projectId": project, "number": n}, &res)`.
   - Bila `err != nil`: `var ie *ipc.Error`; jika `errors.As(err, &ie) && ie.Code == ipc.ErrMethodNotFound` kembalikan `errors.New("daemon belum mendukung git.mr.get (perbarui shikigamid)")`; selain itu kembalikan `err` apa adanya (main sudah mencetak galat dan keluar non-nol; jangan memanggil `os.Exit` atau mencetak sendiri).
   - Sukses: `fmt.Print(gitMrDetailFormat(res))`; kembalikan `nil`. Toleran terhadap hasil kosong/`null` (struct nol tetap dicetak tanpa panic).

7. Registrasi mandiri di akhir file:
   ```go
   func init() { registerGitSub("mr", gitMrDetail) }
   ```

8. Jalankan `gofmt -l cmd/shikigamictl` (harus kosong) dan `go build ./...` dari akar repo. Bila build gagal karena file fitur lain (mis. `mrdetail.go` yang sedang dikerjakan agen lain) dan bukan karena file ini, bangun hanya paket ini: `go build ./cmd/shikigamictl/` dan `go vet ./cmd/shikigamictl/`.

## Perubahan

- Baru: `cmd/shikigamictl/cmd_git_mr_detail.go`
- Tidak ada file lain yang dibuat atau diubah.

## Wiring

Fitur ini mengekspos hanya subcommand `git mr` yang terdaftar mandiri lewat `init()` + `registerGitSub("mr", gitMrDetail)`. Tidak ada edit ke `cmd_git.go`, `main.go`, atau `registry.go`. Langkah wiring (`git-mr-detail-wiring`) nanti hanya memverifikasi: `shikigamictl git` tanpa argumen mencantumkan `mr`; argumen kurang atau `number` bukan angka menghasilkan pesan penggunaan; `shikigamictl git mr <ws> <proj> <n>` mencetak detail nyata bila daemon dengan `git.mr.get` berjalan. JANGAN menyambungkan atau memverifikasi lewat daemon sendiri di langkah ini.

## Selesai bila

- `cmd/shikigamictl/cmd_git_mr_detail.go` ada, `gofmt` bersih, dan tidak mengimpor `internal/gitdata`.
- `registerGitSub("mr", ...)` dipanggil dari `init()` di file ini; `cmd_git.go` tidak berubah (`git diff --stat` hanya menampilkan file ini sebagai baru).
- Argumen != 3, kosong, atau `number` bukan bilangan bulat > 0 menghasilkan galat `penggunaan: shikigamictl git mr <workspaceId> <projectId> <number>`.
- Galat IPC dikembalikan sebagai `error` (exit non-nol lewat `main`); `ErrMethodNotFound` menghasilkan pesan `daemon belum mendukung git.mr.get (perbarui shikigamid)`.
- Keluaran memuat judul dengan prefiks `!`/`#`, status (+draft), penulis, branch, label, reviewer + state, mergeable, pipeline terakhir, url, lalu `deskripsi:` dengan isi apa adanya.
- Guard: `go build ./...` sukses (tanpa unit test).
