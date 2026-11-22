# Progress: git-connector-ticket-02-git-mrs

## Status

OK

## Foundation

tidak ada

## Hasil per fitur

```json
[
  {"slug":"git-mrs-ctl","ok":true,"filesChanged":["worktree/feat-git-connector-ticket-02-git-mrs/cmd/shikigamictl/cmd_git_mrs.go"]},
  {"slug":"git-mrs-data","ok":true,"filesChanged":["worktree/feat-git-connector-ticket-02-git-mrs/internal/gitdata/mrs.go","worktree/feat-git-connector-ticket-02-git-mrs/cmd/shikigamid/wire_git_mrs.go"],"reason":"Dibuat internal/gitdata/mrs.go (ListMergeRequests, GitLab/GitHub, mrRepoArg, agregasi checks, sort) dan cmd/shikigamid/wire_git_mrs.go (modul git-mrs, IPC git.mrs.list). gofmt bersih; go vet ./internal/gitdata dan go build ./internal/gitdata ./cmd/shikigamid lulus. Tidak ada flag yang ditolak allowlist yang diuji secara runtime."},
  {"slug":"git-mrs-ui","ok":true,"filesChanged":["worktree/feat-git-connector-ticket-02-git-mrs/frontend/src/modules/git/MrList.svelte","worktree/feat-git-connector-ticket-02-git-mrs/frontend/src/modules/git/MrRow.svelte","worktree/feat-git-connector-ticket-02-git-mrs/frontend/src/modules/git/StatusBadge.svelte","worktree/feat-git-connector-ticket-02-git-mrs/frontend/src/modules/git/mrClient.ts"],"reason":"npm run build sukses dan npm run check 0 error. Empat warning yang tersisa ada di PaneHostRoot dan TerminalTab, bukan file fitur ini."},
  {"slug":"git-mrs-wiring","ok":true,"filesChanged":[],"reason":"Verifikasi statis dan build lulus; tidak ada file repo yang diubah. Langkah IPC ke daemon nyata dan uji GUI dengan data glab belum dijalankan di lingkungan ini, sehingga tercatat sebagai butir terbuka, bukan lulus."}
]
```

## Wiring

Wiring sudah lengkap dari fitur sebelumnya: `RegisterModule("git-mrs", ...)` di `wire_git_mrs.go`, `registerGitSub("mrs", gitMrs)` di `cmd_git_mrs.go`, `ProjectSection.svelte` merender `<MrList {provider} {repo} />`, dan pane `git-mr` terdaftar di `modules/git/index.ts`.

## Verifikasi

- `go build ./...` lulus (diverifikasi ulang pada tahap finalisasi).
- `gofmt -l` pada `internal/gitdata`, `cmd/shikigamid`, `cmd/shikigamictl` tidak mengeluarkan file.
- `npm run build` di frontend lulus. `npm run check` 0 error, 4 warning lama di `PaneHostRoot` dan `TerminalTab`.

## Butir terbuka (belum diverifikasi)

- `shikigamictl git mrs <ws> <proj> open|merged|closed` terhadap daemon dan project gitlab.com nyata, termasuk uji `[not_found]`.
- Checklist uji GUI (daftar MR GitLab, ganti status, aksi baris, buka pane dan pulih setelah restart, galat tanpa `glab`, batas 3 proses `glab`). GUI tidak bisa dijalankan di lingkungan ini.
- Daftar PR GitHub di app menunggu `gh auth login`.

## Langkah manual yang ditunda

tidak ada
