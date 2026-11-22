# Plan: shikigamictl git mrs

## Metadata

- Tiket: `git-connector-ticket-02-git-mrs` (docs/tickets/git-connector-ticket-02-git-mrs.md)
- Fitur: `git-mrs-ctl` (shikigamictl git mrs)
- ownedPaths: `cmd/shikigamictl/cmd_git_mrs.go`
- Depends on foundation: tidak (epic ini tidak punya foundation; lihat overview bagian 4)
- Bergantung pada kode tiket 1 yang sudah ada: `registerGitSub` / `gitSubFunc` di `cmd/shikigamictl/cmd_git.go`, `printJSON` dan `Command` di `registry.go`, `ipc.Client.Call`.
- Tidak menunggu `git-mrs-data`: build tidak butuh backend; IPC `git.mrs.list` hanya dipanggil saat runtime.

## Tujuan

Menambah subcommand debug `shikigamictl git mrs <workspaceId> <projectId> <state>`. Ia memanggil IPC `git.mrs.list` (kontrak: overview bagian 2.1), mencetak satu baris ringkas per MR/PR, lalu jumlah baris. Argumen salah (kurang, lebih, atau state di luar `open|merged|closed`) menghasilkan pesan penggunaan. Galat dari daemon dikembalikan apa adanya sebagai `error` agar `main.go` ctl mencetaknya dan keluar dengan kode bukan nol.

## Path yang dimiliki

HANYA file ini yang boleh dibuat/diubah:

- `cmd/shikigamictl/cmd_git_mrs.go` (baru)

Jangan menyentuh `cmd_git.go`, `registry.go`, `main.go`, file wiring, `internal/gitdata/*`, `cmd/shikigamid/*`, maupun frontend. Membaca dan memanggil helper di file lain diperbolehkan.

## Kontrak

Disediakan:
- Subcommand `git mrs`, terdaftar mandiri lewat `func init() { registerGitSub("mrs", gitMrs) }`. Dengan begitu `gitUsage()` di `cmd_git.go` otomatis mencantumkan `mrs`.
- Fungsi `gitMrs(ctx context.Context, c *ipc.Client, args []string) error` (tipe `gitSubFunc`).

Dipakai:
- `registerGitSub(name string, fn gitSubFunc)` dan `type gitSubFunc func(ctx context.Context, c *ipc.Client, args []string) error` dari `cmd_git.go`.
- `(*ipc.Client).Call(method string, params any, out any) error` dari `internal/ipc`.
- IPC `git.mrs.list` (overview 2.1): params `{"workspaceId": string, "projectId": string, "state": "open"|"merged"|"closed"}`, hasil `{"mrs": MergeRequest[]}` dengan field JSON camelCase: `number`, `title`, `author`, `sourceBranch`, `targetBranch`, `state`, `draft`, `reviewState` (opsional), `pipelineStatus` (opsional), `updatedAt`, `url`.
- Format argumen (overview 2.4, DIPUTUSKAN mengikuti tiket): posisional, bukan `--workspace`.

Konvensi nama: semua simbol privat paket `main` (ctl) diawali `gitMrs` agar tidak bentrok dengan fitur/tiket lain (overview bagian 7).

## Langkah

1. Buat `cmd/shikigamictl/cmd_git_mrs.go` dengan `package main` dan import: `context`, `errors`, `fmt`, `strings`, `github.com/NooBeeID/shikigami/internal/ipc`. Tidak perlu import lain.

2. Konstanta pesan penggunaan (persis):
   ```go
   const gitMrsUsage = "penggunaan: shikigamictl git mrs <workspaceId> <projectId> open|merged|closed"
   ```

3. Tipe respons lokal (nama berawalan `gitMrs`, jangan memakai `internal/gitdata` agar ctl tidak bergantung pada fitur data):
   ```go
   type gitMrsItem struct {
       Number         int    `json:"number"`
       Title          string `json:"title"`
       Author         string `json:"author"`
       SourceBranch   string `json:"sourceBranch"`
       TargetBranch   string `json:"targetBranch"`
       State          string `json:"state"`
       Draft          bool   `json:"draft"`
       ReviewState    string `json:"reviewState"`
       PipelineStatus string `json:"pipelineStatus"`
       UpdatedAt      string `json:"updatedAt"`
       URL            string `json:"url"`
   }
   type gitMrsResult struct {
       Mrs []gitMrsItem `json:"mrs"`
   }
   ```

4. Helper `func gitMrsNumber(it gitMrsItem) string`: nomor berdasarkan pola URL.
   - `strings.Contains(it.URL, "/-/merge_requests/")` -> `fmt.Sprintf("!%d", it.Number)`
   - `strings.Contains(it.URL, "/pull/")` -> `fmt.Sprintf("#%d", it.Number)`
   - selain itu -> `fmt.Sprintf("%d", it.Number)`

5. Helper `func gitMrsLine(it gitMrsItem) string` membentuk satu baris, kolom dipisah tab (`\t`) agar mudah dibaca/dipotong. Urutan kolom:
   1. nomor (`gitMrsNumber`)
   2. state
   3. `draft` bila `it.Draft`, jika tidak string kosong (kolom tetap ada agar posisi konsisten)
   4. judul; ganti `\t`, `\n`, `\r` di judul dengan spasi agar satu MR tetap satu baris (mis. `strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")`), sama untuk penulis dan nama branch bila perlu
   5. `@` + author
   6. `sourceBranch->targetBranch` (tanpa spasi, tanda panah `->`)
   7. `pipeline=<status>` hanya bila `PipelineStatus != ""`
   8. `review=<state>` hanya bila `ReviewState != ""`
   9. URL

   Kolom opsional 7 dan 8 dilewati sepenuhnya (tanpa tab kosong) bila tidak ada; implementasi paling sederhana: kumpulkan ke `[]string` lalu `strings.Join(cols, "\t")`, dan untuk kolom draft cukup tambahkan hanya bila draft (tidak apa-apa posisi bergeser; ini alat debug). Pilih satu pendekatan dan konsisten.

6. Fungsi utama `func gitMrs(_ context.Context, c *ipc.Client, args []string) error`:
   - Jika `len(args) != 3` -> `return errors.New(gitMrsUsage)`.
   - `ws := strings.TrimSpace(args[0])`, `project := strings.TrimSpace(args[1])`, `state := strings.TrimSpace(args[2])`.
   - Jika `ws == "" || project == ""` -> `errors.New(gitMrsUsage)`.
   - Validasi `state` dengan `switch state { case "open", "merged", "closed": default: return errors.New(gitMrsUsage) }` (nilai huruf kecil persis; tanpa normalisasi/alias).
   - Panggil: 
     ```go
     var res gitMrsResult
     if err := c.Call("git.mrs.list", map[string]any{
         "workspaceId": ws, "projectId": project, "state": state,
     }, &res); err != nil {
         return err
     }
     ```
     Kembalikan `err` apa adanya (jangan dibungkus atau dicetak sendiri); pesan daemon sudah berbentuk `"[kode] pesan"` dan `main.go` ctl yang menangani cetak + kode keluar bukan nol. Jangan `panic`, jangan `os.Exit`.
   - Untuk tiap elemen `res.Mrs` cetak `fmt.Println(gitMrsLine(it))`. `res.Mrs` yang `nil` diperlakukan sebagai kosong (loop tidak berjalan).
   - Akhiri dengan `fmt.Printf("%d baris\n", len(res.Mrs))` (tetap dicetak untuk hasil kosong: `0 baris`).
   - `return nil`.

7. Registrasi mandiri di akhir file:
   ```go
   func init() { registerGitSub("mrs", gitMrs) }
   ```
   Urutan inisialisasi `init()` antar file aman: `gitSubcommands` adalah variabel tingkat paket (`map` diinisialisasi saat deklarasi, sebelum semua `init()`), dan `registerGitSub` hanya menulis ke map. Jangan mengubah `cmd_git.go`.

8. Jalankan guard: `gofmt -l cmd/shikigamictl/` (harus kosong) lalu `go build ./...` dari akar repo. Perbaiki bila gagal, hanya di `cmd_git_mrs.go`. Jika build gagal karena file fitur paralel lain (mis. `internal/gitdata/mrs.go` belum utuh), build sendiri dengan `go build ./cmd/shikigamictl/` dan laporkan; jangan menyentuh file itu.

## Perubahan

- Dibuat: `cmd/shikigamictl/cmd_git_mrs.go`
- Diubah: tidak ada.

## Wiring

Fitur ini mendaftar sendiri lewat `init()` memanggil `registerGitSub("mrs", gitMrs)`; tidak ada yang perlu disambungkan di file lain. Untuk langkah wiring (`git-mrs-wiring`), yang diekspos dan perlu diverifikasi:
- `shikigamictl git` tanpa argumen mencantumkan `mrs` di `gitUsage()`.
- `shikigamictl git mrs <workspaceId> <projectId> open|merged|closed` terhadap daemon yang berjalan memanggil `git.mrs.list` (tersedia setelah `git-mrs-data` terdaftar) dan mencetak baris MR plus `<n> baris`.
- Tidak boleh menyambungkan apa pun dari dalam fitur ini.

## Selesai bila

- [ ] `cmd/shikigamictl/cmd_git_mrs.go` ada dan hanya file itu yang dibuat/diubah (cek `git status`: tidak ada perubahan lain dari fitur ini).
- [ ] `cmd_git.go` tidak berubah; subcommand terdaftar lewat `init()` dengan `registerGitSub("mrs", ...)`.
- [ ] Argumen kurang/lebih, `workspaceId`/`projectId` kosong, atau state selain `open|merged|closed` mengembalikan error berisi `penggunaan: shikigamictl git mrs <workspaceId> <projectId> open|merged|closed`, tanpa memanggil IPC.
- [ ] Galat dari `c.Call` dikembalikan sebagai `error` tanpa panic.
- [ ] Keluaran: satu baris per MR (nomor `!n` untuk URL berisi `/-/merge_requests/`, `#n` untuk `/pull/`, selain itu `n`; state; draft; judul; penulis; `source->target`; pipeline/review bila ada; URL), diakhiri `<n> baris`.
- [ ] Guard: `gofmt -l cmd/shikigamictl/` kosong dan `go build ./...` (atau minimal `go build ./cmd/shikigamictl/`) sukses. Tanpa unit test.
