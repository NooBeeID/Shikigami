# Plan: Manajemen git worktree dan branch thread

## Metadata

- Tiket: `thread-ticket-02-thread-agents` (`docs/tickets/thread-ticket-02-thread-agents.md`)
- Fitur: `thread-worktree`
- ownedPaths: `internal/thread/worktree.go`
- Depends on foundation: ya (Foundation menyediakan stub `worktree.go` dengan signature final dan `worktree_records.go` yang memakai `WorktreeInfo`; fitur ini menggantikan isi stub itu)
- Rujukan wajib dibaca: `docs/plans/thread-ticket-02-thread-agents/overview.md` (bagian 2.1 konvensi path, 2.3 blok `worktree.go`), `docs/plans/thread-ticket-02-thread-agents/00-foundation.md` (langkah 3 = stub). Spec: `docs/superpowers/specs/2026-11-22-thread-design.md` (bagian isolasi worktree dan Close).

## Tujuan

Mengganti stub `internal/thread/worktree.go` dengan implementasi penuh manajemen git worktree dan branch milik thread, hanya lewat `os/exec` dengan binary `git` (tanpa dependency baru, tanpa unit test):

- `Slugify`: nama project -> slug aman untuk nama direktori dan branch.
- `CreateWorktree`: membuat worktree `<dataDir>/threads/<threadId>/<projectSlug>` dengan branch `thread/<threadId>-<slug>` dari HEAD project; bentrok nama branch diatasi sufiks `-2`, `-3`, dst; project non-git mengembalikan `ErrNotGitRepo` tanpa membuat apa pun.
- `InspectWorktree`: melaporkan `Dirty` (perubahan belum di-commit) dan `Unmerged` (commit di branch thread yang belum ada di branch dasar atau `BaseCommit`).
- `RemoveWorktree`: idempoten; menghapus worktree, branch thread, dan prune.

Working tree utama project tidak boleh berubah (selain metadata `.git/worktrees` dan ref branch baru).

## Path yang dimiliki

HANYA `internal/thread/worktree.go`. Jangan menyentuh: `worktree_records.go`, `agents_types.go`, `mcp*.go`, `launch.go`, `spawn.go`, `router.go`, `antiloop.go`, `agents_install.go`, `wire_thread.go`, berkas tiket 01, `internal/mention`, `internal/provider`, `internal/session`, `go.mod`, maupun apa pun di frontend. Bila butuh perubahan di luar berkas ini, jangan dikerjakan; catat sebagai catatan akhir untuk wiring.

## Kontrak

Disediakan (signature FINAL, persis overview 2.3; jangan diubah, jangan menambah ekspor baru yang dipakai fitur lain):

```go
type WorktreeInfo struct {
    Path        string `json:"path"`
    Branch      string `json:"branch"`
    BaseBranch  string `json:"baseBranch"`  // "" bila HEAD detached
    BaseCommit  string `json:"baseCommit"`  // hash HEAD saat spawn, selalu terisi
    ProjectPath string `json:"projectPath"`
}
type WorktreeStatus struct {
    Dirty      []string `json:"dirty"`      // selalu [] bukan null
    Unmerged   []string `json:"unmerged"`   // "<hash> <subject>"; selalu []
    BaseBranch string   `json:"baseBranch"`
}
var ErrNotGitRepo = errors.New("project bukan repo git; thread tidak bisa mengisolasinya")
func Slugify(name string) string
func CreateWorktree(projectPath, dataDir, threadID, projectSlug string) (WorktreeInfo, error)
func InspectWorktree(info WorktreeInfo) (WorktreeStatus, error)
func RemoveWorktree(info WorktreeInfo) error
```

Dipakai oleh pihak lain (tidak perlu diimplementasikan di sini, hanya jangan dipatahkan):
- `thread-spawn-router` memanggil `Slugify`, `CreateWorktree`, `RemoveWorktree`, dan meneruskan galat apa adanya (termasuk `errors.Is(err, ErrNotGitRepo)`), serta menyimpan `WorktreeInfo` lewat `UpsertWorktreeRecord` (Foundation).
- Tiket 04/05 memanggil `InspectWorktree` dan `RemoveWorktree` dari `WorktreeInfo` hasil `LoadWorktreeRecords`.

Konvensi (overview 2.1): worktree `<DataDir>/threads/<threadId>/<projectSlug>`; branch `thread/<threadId>-<slug>`; slug `master` dicadangkan sehingga `Slugify("master")` = `master-project`. Komentar, pesan galat, dan log dalam bahasa Indonesia; identifier bahasa Inggris. Helper tak-diekspor diberi awalan `worktree` (mis. `worktreeGit`, `worktreeRun`, `worktreeBranchExists`) agar tidak bentrok dengan fitur paralel di paket `thread`.

Tidak memakai tipe tiket 01 sama sekali; berkas ini hanya bergantung pada stdlib.

## Langkah

Semua dikerjakan di satu berkas `internal/thread/worktree.go` (timpa isi stub; buang komentar `// STUB(foundation)`). Impor stdlib saja: `bytes`, `context`, `errors`, `fmt`, `os`, `os/exec`, `path/filepath`, `strings`, `time`.

1. **Baca keadaan awal.** Buka `internal/thread/worktree.go` (stub) dan `worktree_records.go`. Pertahankan tag json dan nama field `WorktreeInfo`/`WorktreeStatus` persis seperti kontrak. Bila stub tidak ada (Foundation belum jalan), tulis semua tipe sendiri persis kontrak.

2. **Konstanta dan helper eksekusi.**
   - `const ( worktreeCreateTimeout = 30 * time.Second; worktreeInspectTimeout = 15 * time.Second; worktreeRemoveTimeout = 30 * time.Second; worktreeMaxBranchTries = 50; worktreeBranchPrefix = "thread/"; worktreeSlugMax = 40 )`.
   - `func worktreeGit() (string, error)`: `exec.LookPath("git")`; bila gagal kembalikan `fmt.Errorf("binary git tidak ditemukan di PATH; pasang git agar thread bisa membuat worktree: %w", err)`.
   - `func worktreeRun(ctx context.Context, gitBin, dir string, args ...string) (string, error)`: `exec.CommandContext(ctx, gitBin, append([]string{"-C", dir}, args...)...)`; `cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")` (LC_ALL=C agar pencocokan teks stderr stabil); stdout dan stderr ke `bytes.Buffer` terpisah. Sukses -> `strings.TrimRight(stdout, "\r\n")` (JANGAN `TrimSpace`, karena spasi di awal baris `--porcelain` bermakna). Gagal -> galat `fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr))`; bila `ctx.Err() == context.DeadlineExceeded` kembalikan galat jelas "git %s melewati batas waktu". Galat gagal-jalan perlu dibedakan dari exit non-zero: kembalikan juga tipe `*worktreeExitError`-ringan atau cukup cek `errors.As(err, &*exec.ExitError)` oleh pemanggil; simpan stderr di galat agar bisa dicocokkan dengan `strings.Contains`.
   - `func worktreeBranchExists(ctx, gitBin, repo, branch string) (bool, error)`: `git -C repo show-ref --verify --quiet refs/heads/<branch>`; exit 0 -> true; `*exec.ExitError` dengan kode 1 -> false; selain itu galat.
   - `func worktreeLines(s string) []string`: pecah `\n`, buang `\r` di ujung, buang baris kosong, SELALU kembalikan slice non-nil (`[]string{}`).
   - `func worktreeSafeToDelete(info WorktreeInfo) bool`: true hanya bila `info.Path` tidak kosong, `filepath.IsAbs(info.Path)`, `filepath.Clean(info.Path) != string(filepath.Separator)`, dan `strings.HasPrefix(info.Branch, worktreeBranchPrefix)`. Dipakai sebelum `os.RemoveAll` dan `branch -D` agar branch/direktori milik user tidak pernah terhapus.

3. **`Slugify(name string) string`.**
   - `strings.ToLower`, ubah setiap run karakter di luar `[a-z0-9]` (termasuk non-ASCII dan spasi) menjadi satu `-`, trim `-` di kedua ujung.
   - Potong ke `worktreeSlugMax` karakter lalu trim `-` lagi.
   - Kosong -> `"project"`; persis `"master"` -> `"master-project"`.
   - Harus idempoten: `Slugify(Slugify(x)) == Slugify(x)`.

4. **`CreateWorktree`.** Urutan (semua validasi dan pemeriksaan git SEBELUM menyentuh disk, agar project non-git tidak membuat apa pun):
   1. Validasi argumen: `projectPath`, `dataDir`, `threadID`, `projectSlug` tidak kosong; `threadID` tidak boleh mengandung `/`, `\` atau `..`. Galat `fmt.Errorf` bahasa Indonesia.
   2. `slug := Slugify(projectSlug)` (dipakai untuk path DAN nama branch). `projectAbs, _ := filepath.Abs(projectPath)`; `dataAbs, _ := filepath.Abs(dataDir)`.
   3. `gitBin, err := worktreeGit()`; `ctx, cancel := context.WithTimeout(context.Background(), worktreeCreateTimeout)`.
   4. `git -C projectAbs rev-parse --is-inside-work-tree`: bila exit non-zero (bukan repo, direktori tidak ada, atau bare) ATAU keluaran bukan `true` -> `return WorktreeInfo{}, ErrNotGitRepo` (kembalikan sentinel itu langsung, bukan dibungkus, agar `errors.Is` dan pesan tetap persis). Galat non-exit (mis. timeout) dikembalikan sebagai galat biasa. Opsional: bungkus `fmt.Errorf("%w (%s)", ErrNotGitRepo, projectAbs)` hanya bila `errors.Is` tetap berlaku.
   5. `baseCommit := git rev-parse HEAD`. Gagal (repo tanpa commit) -> galat jelas "repo project belum punya commit; buat commit pertama sebelum melibatkan project ini".
   6. `abbrev := git rev-parse --abbrev-ref HEAD`; bila hasil `"HEAD"` (detached) -> `baseBranch = ""`, selain itu `baseBranch = abbrev`.
   7. `path := filepath.Join(dataAbs, "threads", threadID, slug)`. Bila `path` sudah ada (`os.Lstat` nil) -> galat "worktree sudah ada di <path>" (jangan menimpa). Catat `existedBefore=false`. TIDAK memanggil `MkdirAll`; `git worktree add` membuat direktori induk sendiri.
   8. Loop `i := 1..worktreeMaxBranchTries`: `name := worktreeBranchPrefix + threadID + "-" + slug`, untuk `i > 1` tambahkan `"-" + strconv.Itoa(i)` (impor `strconv`; hasil `-2`, `-3`, ...). Lewati kandidat bila `worktreeBranchExists` true. Lalu `git -C projectAbs worktree add -b <name> <path> <baseCommit>`. Sukses -> keluar loop. Gagal dan stderr mengandung `already exists` atau `already used`/`is already checked out` terkait branch (balapan) -> coba kandidat berikutnya; galat lain -> bersihkan (langkah 9) dan kembalikan. Habis percobaan -> galat "tidak menemukan nama branch bebas setelah N percobaan".
   9. Pembersihan saat gagal (best effort, abaikan galatnya): bila `path` ada dan sebelumnya tidak ada -> `os.RemoveAll(path)`; lalu `git -C projectAbs worktree prune`. Jangan menghapus direktori induk `threads/<threadId>`.
   10. Kembalikan `WorktreeInfo{Path: path, Branch: name, BaseBranch: baseBranch, BaseCommit: baseCommit, ProjectPath: projectAbs}`.

5. **`InspectWorktree(info WorktreeInfo) (WorktreeStatus, error)`.**
   - `st := WorktreeStatus{Dirty: []string{}, Unmerged: []string{}, BaseBranch: info.BaseBranch}`; ctx timeout `worktreeInspectTimeout`; `worktreeGit()`.
   - Validasi: `info.ProjectPath` dan (`info.Path` atau `info.Branch`) tidak kosong.
   - `Dirty`: bila direktori `info.Path` ada -> `git -C info.Path --no-optional-locks status --porcelain` (default sudah menyertakan untracked), `st.Dirty = worktreeLines(out)`. Bila direktori tidak ada -> `Dirty` tetap `[]`.
   - Ref dasar: bila `info.BaseBranch != ""` dan `worktreeBranchExists(ProjectPath, BaseBranch)` -> `base = "refs/heads/" + BaseBranch`; selain itu `base = info.BaseCommit` (bila kosong -> kembalikan `st` tanpa Unmerged).
   - `Unmerged`: bila direktori worktree ada -> `git -C info.Path log --no-color "--format=%h %s" <base>..HEAD`. Bila tidak ada tetapi branch thread masih ada di repo -> jalankan di `ProjectPath` dengan `<base>..refs/heads/<info.Branch>`. Bila keduanya tidak ada -> `[]`. `st.Unmerged = worktreeLines(out)`.
   - Galat git lain dikembalikan dibungkus (`fmt.Errorf("inspeksi worktree %s: %w", ...)`).

6. **`RemoveWorktree(info WorktreeInfo) error`** (idempoten; tiap langkah toleran terhadap kondisi "sudah hilang").
   - Bila `info.Path == "" && info.Branch == ""` -> `nil`.
   - ctx timeout `worktreeRemoveTimeout`; `worktreeGit()`.
   - Bila `!worktreeSafeToDelete(info)` -> kembalikan galat "info worktree tidak aman dihapus" (jangan melakukan apa pun).
   - Bila `info.ProjectPath` tidak ada di disk: hanya `os.RemoveAll(info.Path)` lalu `nil` (repo asal sudah hilang; tidak ada yang bisa di-prune).
   - Kumpulkan galat nyata di `var errs []error`; pada akhir `return errors.Join(errs...)` (nil bila kosong).
   - Langkah a: bila `os.Lstat(info.Path)` ada -> `git -C ProjectPath worktree remove --force --force <Path>` (dua `--force` agar worktree terkunci tetap terhapus). Bila gagal dengan stderr berisi `is not a working tree`/`not a working tree` -> abaikan; galat lain -> catat, lalu fallback `os.RemoveAll(info.Path)` (catat bila gagal).
   - Langkah b: bila direktori masih ada setelah langkah a -> `os.RemoveAll`.
   - Langkah c: `git -C ProjectPath worktree prune` (catat galat nyata).
   - Langkah d (SETELAH prune, karena branch yang sedang di-checkout tidak bisa dihapus): bila `info.Branch != ""` dan `worktreeBranchExists` true -> `git -C ProjectPath branch -D <Branch>`; stderr berisi `not found` -> abaikan; galat lain dicatat. Tidak pernah menghapus branch yang tidak berawalan `thread/` (dijaga `worktreeSafeToDelete`).
   - Panggilan kedua pada info yang sama harus mengembalikan `nil` tanpa galat.

7. **Rapikan.** `gofmt -w internal/thread/worktree.go`; pastikan tidak ada impor tak terpakai; komentar dokumentasi bahasa Indonesia pada setiap simbol ekspor; tidak ada `log`/`slog` (fungsi murni, galat dikembalikan).

8. **Verifikasi (guard).** Dari akar repo: `gofmt -l internal/thread/worktree.go` kosong, `go vet ./internal/thread/` (abaikan galat jelas milik berkas fitur lain), `go build ./...`. Verifikasi manual opsional di repo sementara di scratchpad (bukan berkas proyek, bukan unit test): buat repo git dengan satu commit, jalankan potongan `go run` kecil di luar repo proyek, atau cukup uji perintah git-nya lewat shell: `git worktree add -b thread/X-p <path> HEAD`, bentrok branch, `worktree remove --force --force`, `branch -D`. Pastikan project non-git tidak membuat direktori `threads/<id>`. Jangan commit berkas uji apa pun.

## Perubahan

- Diubah: `internal/thread/worktree.go` (stub Foundation diganti implementasi penuh; tipe dan signature tetap).
- Tidak ada berkas lain yang dibuat atau diubah.

## Wiring

Fitur ini tidak menyambungkan apa pun sendiri dan tidak butuh titik wiring khusus. Yang diekspos untuk pihak lain:

- `Slugify`, `CreateWorktree`, `RemoveWorktree`: dipakai `spawn.go` (thread-spawn-router); `errors.Is(err, ErrNotGitRepo)` valid untuk diteruskan ke master apa adanya.
- `InspectWorktree`, `RemoveWorktree`: dipakai tiket 04 (dialog konfirmasi Close dan cleanup) dan tiket 05 (pembersihan yatim), dengan `WorktreeInfo` dari `LoadWorktreeRecords`.
- Perilaku yang perlu diketahui penyambung: `Info.ProjectPath` selalu absolut; `Info.BaseBranch` kosong bila HEAD detached; `RemoveWorktree` menolak (galat) info yang branch-nya tidak berawalan `thread/`; slug diturunkan ulang lewat `Slugify` di dalam `CreateWorktree` sehingga `Info.Path` harus selalu dibaca dari hasil, bukan dihitung ulang oleh pemanggil.
- Tidak ada perubahan di `wire_thread.go`, `agents_install.go`, maupun frontend. Bila selama implementasi ditemukan kebutuhan di luar `worktree.go`, tulis sebagai "Catatan untuk wiring" di laporan akhir agent, jangan dikerjakan.

## Selesai bila

- `internal/thread/worktree.go` berisi implementasi penuh (tidak ada lagi galat "belum diimplementasikan" dan komentar `STUB(foundation)`), signature identik overview 2.3.
- `CreateWorktree` pada repo git membuat `<dataDir>/threads/<threadId>/<slug>` dan branch `thread/<threadId>-<slug>` dari HEAD; bentrok nama branch menghasilkan `-2`, `-3`, dst, bukan galat; project non-git mengembalikan `ErrNotGitRepo` tanpa membuat direktori apa pun; working tree utama project tidak berubah.
- `InspectWorktree` mengisi `Dirty` dari `git status --porcelain` dan `Unmerged` dari commit di luar branch dasar (atau `BaseCommit`), keduanya `[]` bukan `null` saat kosong.
- `RemoveWorktree` menghapus direktori worktree dan branch thread, memanggil `prune`, aman dipanggil dua kali, dan tidak pernah menghapus branch di luar prefiks `thread/`.
- Semua perintah git memakai `exec.CommandContext` dengan timeout, `GIT_TERMINAL_PROMPT=0`, dan stderr disertakan di pesan galat; binary `git` tak ditemukan memberi pesan jelas.
- Tidak ada dependency baru; hanya `internal/thread/worktree.go` yang berubah (`git status` memastikan).
- Guard: `gofmt -l internal/thread/worktree.go` kosong dan `go build ./...` sukses (tanpa unit test).
