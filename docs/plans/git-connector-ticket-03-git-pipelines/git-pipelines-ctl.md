# Plan: shikigamictl git pipelines dan pipeline

## Metadata

- Tiket: `git-connector-ticket-03-git-pipelines` (`docs/tickets/git-connector-ticket-03-git-pipelines.md`)
- Fitur: `git-pipelines-ctl` (shikigamictl git pipelines dan pipeline)
- ownedPaths: `cmd/shikigamictl/cmd_git_pipelines.go`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; `hasFoundation=false`)
- Rujukan kontrak: `docs/plans/git-connector-ticket-03-git-pipelines/overview.md` bagian 2.1, 2.2, 2.5, 7.

## Tujuan

Menambah dua subcommand alat debug, `shikigamictl git pipelines` dan `shikigamictl git pipeline`. Keduanya memanggil IPC daemon `git.pipelines.list` dan `git.pipeline.get`, lalu mencetak hasil dalam teks yang ringkas. Registrasi dilakukan mandiri dari `init()` di file ini lewat `registerGitSub`, tanpa mengubah `cmd_git.go`. File harus bisa dikompilasi tanpa backend dan tanpa mengimpor `internal/gitdata`.

## Path yang dimiliki

HANYA file ini yang boleh dibuat atau diubah:
- `cmd/shikigamictl/cmd_git_pipelines.go`

Jangan menyentuh file wiring atau path fitur lain. Khususnya jangan ubah `cmd_git.go`, `cmd_git_mrs.go`, `registry.go`, `main.go`, `internal/gitdata/*`, `cmd/shikigamid/*`, maupun `overview.md`. Membaca dan memanggil `registerGitSub`, `gitSubFunc` (di `cmd_git.go`) diperbolehkan.

## Kontrak

Dipakai (sudah ada, jangan didefinisikan ulang):
- `type gitSubFunc func(ctx context.Context, c *ipc.Client, args []string) error` dan `registerGitSub(name string, fn gitSubFunc)` di `cmd/shikigamictl/cmd_git.go`. `registerGitSub` panic bila nama kembar, jadi nama `pipelines` dan `pipeline` harus unik.
- `(*ipc.Client).Call(method string, params any, out any) error` dari `github.com/NooBeeID/shikigami/internal/ipc`. Galat daemon dikembalikan sebagai `error` dan diteruskan apa adanya.
- Pola acuan: `cmd/shikigamictl/cmd_git_mrs.go` (bentuk panggilan, `OneLine` replacer) dan `gitRepos` di `cmd_git.go` (pola `flag.NewFlagSet` + `SetOutput(io.Discard)`).

IPC yang dipanggil (overview 2.1, 2.2):
- `git.pipelines.list`, params `{"workspaceId": string, "projectId": string}`; hasil ARRAY JSON `Pipeline[]` langsung (bukan objek pembungkus), maks 20. Toleran terhadap `null` (decode ke slice nil lalu cetak `0 baris`).
- `git.pipeline.get`, params `{"workspaceId": string, "projectId": string, "id": string}`; hasil objek `PipelineDetail` (field `Pipeline` di level atas ditambah `stages`).

Bentuk JSON (camelCase): Pipeline `{id, ref, sha, status, author?, durationSec?, updatedAt, url}`; detail menambah `stages: [{name, jobs: [{name, status, durationSec?, url?}]}]`. `durationSec` 0 atau hilang berarti tidak tersedia, dicetak `-`.

Disediakan (simbol baru, semua berawalan `gitPipelines`/`gitPipeline`; tidak bentrok dengan `gitMrs*`/`gitRepos*`):
- `gitPipelinesListUsage`, `gitPipelineGetUsage` (const string)
- `gitPipelinesItem`, `gitPipelinesJob`, `gitPipelinesStage`, `gitPipelinesDetail` (tipe lokal)
- `gitPipelinesOneLine`, `gitPipelinesDuration`, `gitPipelinesShortSHA`, `gitPipelinesLine`
- `gitPipelines` (handler list) dan `gitPipeline` (handler detail), keduanya `gitSubFunc`

## Langkah

Buat `cmd/shikigamictl/cmd_git_pipelines.go`, `package main`. Impor: `context`, `errors`, `flag`, `fmt`, `io`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`. Tanpa `gitdata`.

1. Konstanta penggunaan, persis:
   ```go
   const gitPipelinesListUsage = "penggunaan: shikigamictl git pipelines --workspace <id> --project <id>"
   const gitPipelineGetUsage = "penggunaan: shikigamictl git pipeline --workspace <id> --project <id> --id <id>"
   ```

2. Tipe lokal untuk decode hasil IPC:
   ```go
   type gitPipelinesItem struct {
       ID          string `json:"id"`
       Ref         string `json:"ref"`
       SHA         string `json:"sha"`
       Status      string `json:"status"`
       Author      string `json:"author"`
       DurationSec int    `json:"durationSec"`
       UpdatedAt   string `json:"updatedAt"`
       URL         string `json:"url"`
   }
   type gitPipelinesJob struct {
       Name        string `json:"name"`
       Status      string `json:"status"`
       DurationSec int    `json:"durationSec"`
       URL         string `json:"url"`
   }
   type gitPipelinesStage struct {
       Name string            `json:"name"`
       Jobs []gitPipelinesJob `json:"jobs"`
   }
   type gitPipelinesDetail struct {
       gitPipelinesItem
       Stages []gitPipelinesStage `json:"stages"`
   }
   ```
   Catatan: embedding struct tanpa tag membuat field JSON level-atas ter-flatten oleh `encoding/json`, sesuai bentuk `PipelineDetail`.

3. Helper:
   - `var gitPipelinesOneLine = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")`
   - `func gitPipelinesDuration(sec int) string`: `sec <= 0` -> `"-"`, selain itu `fmt.Sprintf("%ds", sec)`.
   - `func gitPipelinesShortSHA(sha string) string`: bersihkan lewat `gitPipelinesOneLine`, potong ke 7 karakter bila lebih panjang (aman terhadap panjang < 7).
   - `func gitPipelinesLine(it gitPipelinesItem) string`: `strings.Join` dengan tab dari `[]string{clean(ID), clean(Ref), clean(Status), shortSHA(SHA), duration(DurationSec), clean(URL)}`; semua string melewati `gitPipelinesOneLine.Replace`.

4. Parsing flag (satu helper privat agar dua handler tidak menduplikasi): `func gitPipelinesParse(name, usage string, withID bool, args []string) (ws, project, id string, err error)`.
   - `fs := flag.NewFlagSet(name, flag.ContinueOnError)`, `fs.SetOutput(io.Discard)`.
   - Definisikan `--workspace`, `--project`; bila `withID` juga `--id`.
   - `fs.Parse(args)` gagal -> `fmt.Errorf("%v\n%s", err, usage)`. Argumen sisa (`fs.NArg() > 0`) -> `errors.New(usage)`.
   - `strings.TrimSpace` pada tiap nilai. Bila `ws == ""`, `project == ""`, atau (`withID` dan `id == ""`) -> `errors.New(usage)`. Tidak boleh panic dalam kasus apa pun.

5. Handler list:
   ```go
   func gitPipelines(_ context.Context, c *ipc.Client, args []string) error
   ```
   - Panggil `gitPipelinesParse("git pipelines", gitPipelinesListUsage, false, args)`.
   - `var res []gitPipelinesItem`; `c.Call("git.pipelines.list", map[string]any{"workspaceId": ws, "projectId": project}, &res)`; galat dikembalikan langsung.
   - Cetak `gitPipelinesLine(it)` per item via `fmt.Println`, lalu `fmt.Printf("%d baris\n", len(res))`.

6. Handler detail:
   ```go
   func gitPipeline(_ context.Context, c *ipc.Client, args []string) error
   ```
   - Parse dengan `withID=true`, usage `gitPipelineGetUsage`.
   - `var res gitPipelinesDetail`; `c.Call("git.pipeline.get", map[string]any{"workspaceId": ws, "projectId": project, "id": id}, &res)`; galat dikembalikan langsung.
   - Cetak baris header: `gitPipelinesLine(res.gitPipelinesItem)`.
   - Untuk tiap stage: `fmt.Printf("[stage] %s\n", clean(stage.Name))`; untuk tiap job: `fmt.Printf("  %s\t%s\t%s\t%s\n", clean(Name), clean(Status), duration(DurationSec), clean(URL))` (URL kosong tetap dicetak sebagai kolom kosong).
   - Stage tanpa job tetap mencetak baris `[stage]`. Pipeline tanpa stage hanya mencetak header.

7. Registrasi mandiri di akhir file:
   ```go
   func init() {
       registerGitSub("pipelines", gitPipelines)
       registerGitSub("pipeline", gitPipeline)
   }
   ```

8. Jalankan `gofmt -l cmd/shikigamictl/cmd_git_pipelines.go` (harus tanpa keluaran) dan `go build ./...` dari akar repo. Bila build gagal karena file di luar ownedPaths (mis. fitur paralel belum selesai), pastikan galat bukan berasal dari file ini; laporkan, jangan perbaiki file lain.

## Perubahan

- Dibuat: `cmd/shikigamictl/cmd_git_pipelines.go`
- Diubah: tidak ada.

## Wiring

File ini mengekspos, untuk diverifikasi langkah wiring (`git-pipelines-wiring`, overview bagian 5 butir 2):
- `registerGitSub("pipelines", gitPipelines)` dan `registerGitSub("pipeline", gitPipeline)` dari `init()`, sehingga `shikigamictl git` tanpa argumen mencantumkan `pipeline|pipelines` lewat `gitUsage()`.
- Pemanggilan IPC `git.pipelines.list` dan `git.pipeline.get` yang secara runtime dilayani `cmd/shikigamid/wire_git_pipelines.go` (milik fitur `git-pipelines-data`).

Jangan menyambungkan apa pun sendiri dan jangan mengubah `cmd_git.go`.

## Selesai bila

- `cmd/shikigamictl/cmd_git_pipelines.go` ada, terformat `gofmt`, dan tidak mengimpor `internal/gitdata`.
- `go build ./...` dari akar repo sukses (guard; tanpa unit test).
- `registerGitSub` dipanggil untuk `pipelines` dan `pipeline` dari `init()`; tidak ada file lain yang berubah (`git status` hanya menunjukkan file ini sebagai tambahan dari fitur ini).
- Flag wajib hilang (`--workspace`, `--project`, atau `--id` untuk detail) menghasilkan galat berisi teks penggunaan persis seperti di atas, bukan panic.
- List mencetak satu baris per pipeline berkolom tab `id, ref, status, sha 7 karakter, durasi (Ns atau -), url`, diakhiri `<n> baris`; detail mencetak header lalu `[stage] <nama>` dan baris job berindentasi `  <nama>\t<status>\t<durasi>\t<url>`.
- Galat daemon dikembalikan sebagai `error` (dicetak dan keluar non-nol oleh `main`).
