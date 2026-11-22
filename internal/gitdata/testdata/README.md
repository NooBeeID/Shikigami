# Fixture gitdata

Fixture keluaran `glab` dan `gh` untuk acuan bentuk parser paket `internal/gitdata` dan adapter connector `gitlab`/`github`. Dihasilkan oleh spike `git-cli-spike` (tanggal 2026-11-22, `glab 1.97.0`, `gh 2.102.0`). Temuan lengkap ada di `docs/git/cli-findings.md`.

Epic ini tidak punya unit test; fixture dipakai sebagai acuan bentuk (tiket 02 sampai 04 dan adapter), bukan sebagai masukan tes otomatis.

## PENTING: fixture GitHub adalah ASUMSI

SEMUA fixture `gh-*` ASUMSI sampai `gh auth login` dijalankan dan diverifikasi, kecuali yang tertulis NYATA di tabel (`gh-auth-status-loggedout.txt`, `gh-auth-status-loggedout.json`, `gh-pr-list-loggedout.txt`). Strukturnya mengikuti daftar field resmi dari `gh <perintah> --help` (versi 2.102.0) dan bentuk JSON `gh` yang didokumentasikan, dengan data palsu. Penanda ASUMSI sengaja tidak ditaruh di dalam berkas agar parser menerimanya persis.

## Daftar berkas

| Berkas | Perintah penghasil | Asal |
|---|---|---|
| `glab-mr-list.json` | `glab mr list -R <owner>/<repo> -F json --all` | NYATA (anonim) |
| `glab-mr-view.json` | `glab mr view <iid> -R <owner>/<repo> -F json` | NYATA (anonim) |
| `glab-pipeline-list.json` | `glab ci list -R <owner>/<repo> -F json` (gabungan 5 pipeline: success, failed, running, canceled, manual) | NYATA (anonim) |
| `glab-pipeline-get.json` | `glab ci get -R <owner>/<repo> -p <id> -F json` (memuat `jobs` dengan `stage`) | NYATA (anonim) |
| `glab-pipeline-get-nojobs.json` | `glab ci get -R <owner>/<repo> -p <id> -F json` pada pipeline gagal tanpa job (`jobs` bernilai `null`) | NYATA (anonim) |
| `glab-auth-status.txt` | `glab auth status` (login; SEMUA teks ada di stderr, stdout kosong, exit 0) | NYATA (anonim) |
| `glab-auth-status-loggedout.txt` | `GLAB_CONFIG_DIR=<dir kosong> glab auth status` (stderr, exit 1) | NYATA (anonim) |
| `glab-config-get-user.txt` | `glab config get user --host <host>` (stdout, exit 0) | NYATA (anonim) |
| `glab-errors.txt` | Rangkuman stderr galat nyata: repo tidak ada, iid tidak ada, pipeline tidak ada, host tak terjangkau, tanpa token | NYATA (anonim) |
| `gh-pr-list.json` | `gh pr list -R <owner>/<repo> --state all --json number,title,author,headRefName,baseRefName,state,isDraft,reviewDecision,statusCheckRollup,updatedAt,url,labels,reviewRequests` | ASUMSI |
| `gh-pr-view.json` | `gh pr view <number> -R <owner>/<repo> --json number,title,body,author,headRefName,baseRefName,state,isDraft,reviewDecision,mergeable,mergeStateStatus,statusCheckRollup,updatedAt,url,labels,reviewRequests,latestReviews` | ASUMSI |
| `gh-run-list.json` | `gh run list -R <owner>/<repo> --json databaseId,displayTitle,headBranch,headSha,status,conclusion,event,workflowName,createdAt,updatedAt,startedAt,url` | ASUMSI |
| `gh-run-view.json` | `gh run view <id> -R <owner>/<repo> --json databaseId,displayTitle,headBranch,headSha,status,conclusion,event,workflowName,createdAt,updatedAt,startedAt,url,jobs` | ASUMSI |
| `gh-auth-status.txt` | `gh auth status` (bentuk login) | ASUMSI |
| `gh-auth-status.json` | `gh auth status --json hosts` (bentuk login; kunci `hosts` NYATA dari help, isi per akun ASUMSI) | ASUMSI |
| `gh-auth-status-loggedout.txt` | `gh auth status` belum login (stderr, exit 1) | NYATA |
| `gh-auth-status-loggedout.json` | `gh auth status --json hosts` belum login (stdout, exit 0; teks "You are not logged..." tambahan di stderr) | NYATA |
| `gh-pr-list-loggedout.txt` | `gh pr list -R <owner>/<repo> --json number` belum login (stderr, exit 4) | NYATA |

## Anonimisasi

Struktur, urutan, tipe, dan nilai null/kosong dipertahankan; yang diganti secara konsisten:

- user: username jadi `dev-alpha`, `dev-bravo`, ...; nama `Dev Alpha`, ...; id 1001 dan seterusnya; avatar dan `web_url` user dibuat palsu; email dikosongkan.
- id numerik: project 1001, MR 5001+, pipeline 3001+, job 4001+ (iid MR/pipeline dipertahankan).
- path repo: `acme-group/acme-app` (daftar MR dan pipeline) dan `acme-group/platform/acme-app` (contoh subgroup di `glab-mr-view.json` dan `glab-pipeline-get*.json`); semua URL dibangun ulang dari path itu (host `gitlab.com` dipertahankan).
- judul, deskripsi, pesan commit: teks palsu berbahasa Indonesia; branch `feature/contoh-N` (`main`, `staging` dipertahankan); SHA 40 hex palsu konsisten (SHA nol dipertahankan).
- dihapus/dikosongkan: runner, tag runner, email commit, avatar asli, hash avatar, token (`Token found: ****`), path konfigurasi lokal.
- timestamp asli dipertahankan (RFC3339).

Berkas `.txt` auth direkam dari stderr (glab menulis seluruh keluaran `auth status` ke stderr; `gh` menulis pesan belum login ke stderr).
