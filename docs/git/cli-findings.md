# Temuan spike glab dan gh

Kontrak data untuk tiket 02 sampai 04, `gitdata-foundation`, `git-connector-adapters`, dan Wiring. Temuan `glab` berasal dari eksekusi nyata (hanya perintah baca) dan fixture teranonimkan di `internal/gitdata/testdata/` (nama berkas dirujuk per bagian). Temuan `gh` berasal dari `gh --help` (nyata) dan dokumentasi (ASUMSI) karena `gh` belum login. Tidak ada data sensitif di dokumen ini; semua nilai contoh palsu dari fixture. Penanda: **NYATA** = teramati di mesin ini, **ASUMSI** = dari dokumentasi/help, belum diverifikasi nyata.

## 1. Versi dan lingkungan

- `glab 1.97.0` (3804f048), `gh 2.102.0` (2026-09-30), `git 2.39.5`, `jq 1.7.1`. Tanggal spike: 2026-11-22.
- `glab` login ke `gitlab.com` (satu host, protokol https). `gh` BELUM login (tidak ada host).
- Semua perintah memakai `-R` (tanpa mengubah direktori kerja) dan format JSON.
- `glab` menulis catatan tambahan ke stderr: `A new version of glab has been released ...` dan `Could not send telemetry data ...`. `RunCLI` hanya mengembalikan stdout pada sukses, jadi catatan ini tidak mengganggu; pada galat, catatan itu bisa ikut masuk ke 300 karakter stderr yang dikembalikan. Jangan menjalankan `glab check-update` atau pembaruan.
- `glab auth status` dan `gh auth status` menulis ke **stderr** (lihat bagian 7); ini temuan terpenting untuk adapter.
- Nilai contoh di dokumen ini berasal dari fixture teranonimkan (`acme-group/acme-app`, `dev-alpha`).

## 2. Tabel perintah

Flag singkat sama bisa berarti lain antar subcommand (contoh `-t`: `--show-token` di `auth status`, `--target-branch` di `mr list`, `--template` di `gh`). Allowlist HARUS per subcommand dan sebaiknya hanya menerima flag bentuk panjang atau bentuk singkat yang tercantum.

| Perintah | Flag wajib | Flag opsional penting | Format JSON | Bentuk `-R` | Fixture | Asal |
|---|---|---|---|---|---|---|
| `glab auth status` | - | `--hostname <host>`, `-a/--all` | tidak ada (teks, stderr) | - | `glab-auth-status.txt`, `glab-auth-status-loggedout.txt` | NYATA |
| `glab config get <key>` | `--host <host>` (untuk `user`) | key hanya `user` atau `host` | teks, stdout | - | `glab-config-get-user.txt` | NYATA |
| `gh auth status` | - | `--hostname <host>`, `--json hosts` | `--json hosts` (stdout) | - | `gh-auth-status.txt`, `gh-auth-status.json`, `gh-auth-status-loggedout.*` | NYATA (belum login), ASUMSI (login) |
| `glab mr list` | `-R`, `-F json` | `-P/--per-page N`, `-p/--page`, `-A/--all`, `-M/--merged`, `-c/--closed`, `-d/--draft`, `--not-draft`, `-s/--source-branch`, `-t/--target-branch` | `-F json` (`--output json`) | `OWNER/REPO`, `GROUP/SUB/REPO`, URL penuh | `glab-mr-list.json` | NYATA |
| `glab mr view <iid>` | `-R`, `-F json` | - | `-F json` | sama | `glab-mr-view.json` | NYATA |
| `glab ci list` (daftar pipeline final) | `-R`, `-F json` | `-P/--per-page`, `-p/--page`, `-r/--ref`, `-s/--status`, `--source`, `--scope`, `-u/--username`, `-o/--order`, `--sort` | `-F json` | sama | `glab-pipeline-list.json` | NYATA |
| `glab ci get` (detail pipeline final, termasuk job) | `-R`, `-F json`, `-p/--pipeline-id <id>` | `-b/--branch` | `-F json` | sama | `glab-pipeline-get.json`, `glab-pipeline-get-nojobs.json` | NYATA |
| `gh pr list` | `-R`, `--json <fields>` | `-s/--state {open,closed,merged,all}`, `-L/--limit N`, `-d/--draft`, `-B/--base`, `-H/--head`, `-A/--author` | `--json` | `OWNER/REPO`, `HOST/OWNER/REPO` | `gh-pr-list.json` | ASUMSI (flag dari help NYATA) |
| `gh pr view <number>` | `-R`, `--json <fields>` | - | `--json` | sama | `gh-pr-view.json` | ASUMSI |
| `gh run list` | `-R`, `--json <fields>` | `-L/--limit N` (default 20), `-b/--branch`, `-s/--status`, `-e/--event`, `--workflow` | `--json` | sama | `gh-run-list.json` | ASUMSI |
| `gh run view <id>` | `-R`, `--json <fields>` | - (`jobs` adalah field JSON) | `--json ...,jobs` | sama | `gh-run-view.json` | ASUMSI |
| `git remote` | `-C <path>` | - | teks, satu nama per baris, urut abjad (bukan `origin` dulu) | - | (lihat bagian 9) | NYATA |
| `git remote get-url <nama>` | `-C <path>` | - | teks satu baris | - | (lihat bagian 9) | NYATA |

Flag final yang diputuskan:
- `-R/--repo` selalu dipakai untuk `glab` dan `gh` (jangan bergantung direktori kerja). `glab ci get` TANPA `-p` memakai branch direktori kerja saat ini, jadi `-p` wajib.
- JSON: `glab` memakai `-F json` (nama panjang `--output json`); `gh` memakai `--json <daftar field dipisah koma>`.
- Batas jumlah: `glab` `-P/--per-page` (default 30), `gh` `-L/--limit` (`pr list` default 30, `run list` default 20).
- Filter state MR: default hanya terbuka; `--merged/-M`, `--closed/-c`, `--all/-A`. Filter state PR: `--state open|closed|merged|all`.
- `--hostname` ada pada `glab auth status` dan `gh auth status` (NYATA, dari help).

Flag dan perintah yang HARUS ditolak allowlist:
- `--show-token` / `-t` pada `auth status` (keduanya); `glab config get token` dan key apa pun selain `user`/`host`.
- `glab ci get --with-variables` (membuka variabel pipeline) dan `-d/--with-job-details` (tidak perlu: job sudah ada).
- Semua subcommand tulis: `mr create|merge|approve|close|note|update|rebase|subscribe`, `ci run|retry|cancel|trigger|run-trig|delete|artifact`, `pr create|merge|review|close|comment|edit|checkout`, `run rerun|cancel|delete|download`, `auth login|logout|refresh|token|setup-git`, `config set|edit`, `api` (semua method).
- Subcommand interaktif atau streaming: `glab ci view`, `glab ci status` (terutama `--live`), `glab ci trace`, `-w/--web` (membuka browser) di semua perintah, `gh run view --log|--log-failed`, `gh pr view -c`.
- `glab mr view -c/--comments` (tidak dipakai).

### Rekomendasi tabel allowlist

Baris literal untuk `allowlist.go` (tool, subcommand, flag diizinkan; arg posisional dibatasi sesuai kolom terakhir):

```
glab auth status      flags: --hostname <host> | -a | --all
glab config get       arg: user | host ; flags: --host <host>
glab mr list          flags: -R --repo | -F --output (nilai json) | -P --per-page | -p --page | -A --all | -M --merged | -c --closed | -d --draft | --not-draft | -s --source-branch | -t --target-branch
glab mr view          arg: <iid angka> ; flags: -R --repo | -F --output (nilai json)
glab ci list          flags: -R --repo | -F --output (json) | -P --per-page | -p --page | -r --ref | -s --status | --source | --scope | -u --username | -o --order | --sort
glab ci get           flags: -R --repo | -F --output (json) | -p --pipeline-id | -b --branch
gh   auth status      flags: --hostname <host> | --json hosts
gh   pr list          flags: -R --repo | --json <fields> | -s --state | -L --limit | -d --draft | -B --base | -H --head | -A --author
gh   pr view          arg: <number> ; flags: -R --repo | --json <fields>
gh   run list         flags: -R --repo | --json <fields> | -L --limit | -b --branch | -s --status | -e --event | --workflow
gh   run view         arg: <id angka> ; flags: -R --repo | --json <fields>
git  [-C <path>] remote
git  [-C <path>] remote get-url <nama>
```

Catatan allowlist:
- Subcommand pipeline final: `glab ci list` dan `glab ci get`. `glab pipeline ...` adalah alias grup `ci` (help-nya sama; `glab pipeline list` bekerja) tetapi tidak perlu diizinkan; cukup `ci`.
- Baris `glab config get` dan `gh auth status --json hosts` adalah TAMBAHAN terhadap daftar awal overview 2.3 (alasan di bagian 7): perlu agar nama akun dan daftar host login bisa dibaca karena `glab auth status` tidak menulis apa pun ke stdout. Batasan keras: arg `glab config get` hanya `user` atau `host` (key `token` mengembalikan token).
- Nilai `-F` HARUS `json`; nilai `--json` HARUS daftar field dari huruf/angka/koma; tolak `-q/--jq` dan `-t/--template` (tidak dipakai).
- Untuk `gh`, bentuk singkat `-h` berarti `--hostname` pada `auth status`; sebaiknya terima hanya bentuk panjang agar tidak ambigu dengan `--help`.

## 3. Skema JSON ringkas dan pemetaan field

Notasi: `?` = bisa null/kosong. Seluruh bagian GitLab NYATA; bagian GitHub ASUMSI kecuali daftar field (help, NYATA).

### 3.1 `glab mr list -F json` (`glab-mr-list.json`)

Array objek. Field top-level yang ada: `id, iid, project_id, title, description, state, draft, created_at, updated_at, closed_at?, merged_at?, author{id,username,name,state,avatar_url,web_url}, assignee?, assignees[], reviewers[{id,username,name,state,avatar_url,web_url}], source_branch, target_branch, labels[] (string), label_details?, detailed_merge_status, has_conflicts, sha, merge_commit_sha, user_notes_count, web_url, references{short,relative,full}, closed_by?, merged_by?, merge_user?` dan lainnya yang tidak dipakai.
Tidak ada: `head_pipeline`, `pipeline`, `approved_by`, `approvals_before_merge`, dan `merge_status` (key ada tetapi kosong/null). `work_in_progress` null; gunakan `draft`.

| Field CLI | Field tipe `MergeRequest` | Catatan |
|---|---|---|
| `iid` (int) | `Number` | bukan `id` |
| `title` | `Title` | awalan `Draft: ` ikut judul |
| `author.username` | `Author` | |
| `source_branch`, `target_branch` | `SourceBranch`, `TargetBranch` | |
| `state` | `State` | `opened`->`open`, `merged`->`merged`, `closed`->`closed`, `locked`->`closed` |
| `draft` (bool) | `Draft` | |
| `updated_at` (RFC3339 dengan milidetik) | `UpdatedAt` | disalin apa adanya |
| `web_url` | `URL` | |
| (tidak ada) | `ReviewState` | kosong (omitempty) untuk GitLab |
| (tidak ada) | `PipelineStatus` | kosong (omitempty) untuk GitLab |

### 3.2 `glab mr view <iid> -F json` (`glab-mr-view.json`)

Semua field daftar ditambah `head_pipeline`, `pipeline`, `changes_count`, `diff_refs`, `merge_error`, `latest_build_started_at`, dan lain-lain. `reviewers[]` hanya berisi data user (TANPA state review). `merge_status` null; `detailed_merge_status` string.

| Field CLI | Field tipe `MergeRequestDetail` | Catatan |
|---|---|---|
| (semua field 3.1) | `MergeRequest` embedded | |
| `description` | `Description` | markdown; boleh kosong |
| `labels` (array string; `[]` bila kosong) | `Labels` | `null` -> `[]` |
| `reviewers[].username` | `Reviewers[].Name` | `State` kosong (GitLab tidak menyediakan) |
| `has_conflicts` true | `Mergeable` = `conflict` | prioritas pertama |
| `detailed_merge_status` | `Mergeable` | nilai apa adanya (`mergeable`, `unchecked`, `checking`, `ci_must_pass`, `not_approved`, `discussions_not_resolved`, `draft_status`, `need_rebase`, ...); `not_open` -> kosong (MR sudah selesai). Teramati: `mergeable`, `unchecked`, `not_open` |
| `head_pipeline{id,status,web_url}` (fallback `pipeline`) | `LastPipeline{ID,Status,URL}` | `id` angka -> string, `status` -> `NormalizeGitLabStatus`; `null` -> `LastPipeline` nil (omitempty). Teramati: `null` pada MR yang tidak punya pipeline dan objek pada MR dengan pipeline |

### 3.3 `glab ci list -F json` (`glab-pipeline-list.json`)

Array objek dengan HANYA: `id, iid, project_id, status, source, ref, sha, name, web_url, created_at, updated_at`. Tidak ada `user`, `duration`, `finished_at`.

| Field CLI | Field tipe `Pipeline` | Catatan |
|---|---|---|
| `id` (int) | `ID` | -> string |
| `ref` | `Ref` | pipeline MR: `refs/merge-requests/<iid>/head` |
| `sha` | `SHA` | |
| `status` | `Status` | `NormalizeGitLabStatus` |
| (tidak ada) | `Author` | kosong di daftar (omitempty) |
| (tidak ada) | `DurationSec` | 0 di daftar |
| `updated_at` | `UpdatedAt` | |
| `web_url` | `URL` | |

### 3.4 `glab ci get -p <id> -F json` (`glab-pipeline-get.json`, `glab-pipeline-get-nojobs.json`)

Objek pipeline lengkap: `id, iid, project_id, status, source, ref, name, sha, before_sha, tag, yaml_errors, user{...}, created_at, started_at, finished_at, committed_at, duration (int detik), queued_duration, coverage, web_url, detailed_status{...}, variables (null), jobs[]`.
`jobs` adalah array objek (job) dengan: `id, name, stage, status, duration (float detik), queued_duration, allow_failure, created_at, started_at?, finished_at?, failure_reason, web_url, ref, commit{...}, pipeline{id,status,...}, user{...}, runner{...}` dan lain-lain. Urutan `jobs` dari server adalah id MENURUN (terbaru dulu) dan TIDAK ada daftar stage terurut. `jobs` bernilai `null` (bukan `[]`) pada pipeline tanpa job (mis. gagal saat dibuat). Flag `-d/--with-job-details` tidak menambah field (NYATA, diff identik).

| Field CLI | Field tipe | Catatan |
|---|---|---|
| (semua 3.3) | `Pipeline` | `user.username` -> `Author`, `duration` -> `DurationSec` (pipeline running: 0) |
| `jobs` urut id naik, dikelompokkan per `stage` menurut kemunculan pertama | `Stages[].Name` dan urutannya | ASUMSI wajar: job dibuat berurutan per stage, jadi id naik mengikuti urutan stage; urutan nyata stage tidak tersedia di JSON |
| `jobs[].name` | `Job.Name` | |
| `jobs[].status` | `Job.Status` | `NormalizeGitLabStatus` (teramati `success`, `manual`, `canceled`) |
| `jobs[].duration` (float) | `Job.DurationSec` | dibulatkan ke int terdekat |
| `jobs[].web_url` | `Job.URL` | |
| `jobs` null | `Stages` | `[]` |

### 3.5 GitHub (ASUMSI)

`gh pr list --json` field resmi (help NYATA): `additions, assignees, author, autoMergeRequest, baseRefName, baseRefOid, body, changedFiles, closed, closedAt, closingIssuesReferences, comments, commits, createdAt, deletions, files, fullDatabaseId, headRefName, headRefOid, headRepository, headRepositoryOwner, id, isCrossRepository, isDraft, labels, latestReviews, maintainerCanModify, mergeCommit, mergeStateStatus, mergeable, mergedAt, mergedBy, milestone, number, potentialMergeCommit, projectCards, projectItems, reactionGroups, reviewDecision, reviewRequests, reviews, state, statusCheckRollup, title, updatedAt, url` (sama untuk `pr view`).
`gh run list --json`: `attempt, conclusion, createdAt, databaseId, displayTitle, event, headBranch, headSha, name, number, startedAt, status, updatedAt, url, workflowDatabaseId, workflowName`. `gh run view --json` menambah `jobs`.
Field yang DIMINTA (hindari field besar):
- daftar PR: `number,title,author,headRefName,baseRefName,state,isDraft,reviewDecision,statusCheckRollup,updatedAt,url`
- detail PR: daftar PR + `body,labels,reviewRequests,latestReviews,mergeable`
- daftar run: `databaseId,displayTitle,headBranch,headSha,status,conclusion,event,workflowName,createdAt,updatedAt,startedAt,url`
- detail run: field daftar run + `jobs`

| Field `gh` | Field tipe | Catatan |
|---|---|---|
| `number` | `MergeRequest.Number` | |
| `title` | `Title` | |
| `author.login` | `Author` | bot: `login` bisa berawalan `app/` |
| `headRefName`, `baseRefName` | `SourceBranch`, `TargetBranch` | |
| `state` (`OPEN`/`MERGED`/`CLOSED`, huruf besar) | `State` | huruf kecil -> `open`/`merged`/`closed` |
| `isDraft` | `Draft` | |
| `reviewDecision` (`APPROVED`/`CHANGES_REQUESTED`/`REVIEW_REQUIRED`/`""`) | `ReviewState` | `approved` / `changes_requested` / `review_required` / kosong |
| `statusCheckRollup[]` | `PipelineStatus` | agregasi, lihat bagian 6 |
| `updatedAt`, `url` | `UpdatedAt`, `URL` | |
| `body` | `Description` | |
| `labels[].name` | `Labels` | |
| `reviewRequests[]` (`login` untuk User, `slug`/`name` untuk Team) | `Reviewers[].Name` | `State` dari `latestReviews[].state` yang cocok `author.login` (`APPROVED`, `CHANGES_REQUESTED`, `COMMENTED`); tanpa review kosong |
| `mergeable` (`MERGEABLE`/`CONFLICTING`/`UNKNOWN`) | `Mergeable` | `mergeable` / `conflict` / `unchecked` |
| (run) `databaseId` | `Pipeline.ID` | angka -> string |
| (run) `headBranch`, `headSha` | `Ref`, `SHA` | |
| (run) `status`+`conclusion` | `Status` | `NormalizeGitHubStatus` |
| (run) `displayTitle`, `workflowName` | tidak dipetakan (tipe `Pipeline` tidak punya judul) | boleh diabaikan |
| (run) `updatedAt`, `url` | `UpdatedAt`, `URL` | |
| (run) `Author`, `DurationSec` | kosong / 0 di daftar; `DurationSec` detail = `updatedAt - startedAt` bila `status` completed | ASUMSI, tidak ada field durasi |
| `jobs[]` (run view) | `Stages` | GitHub tidak punya stage; satu `Stage` per workflow (`Name` = `workflowName`) berisi semua job |
| `jobs[].name`, `status`+`conclusion`, `url` | `Job.Name`, `Job.Status`, `Job.URL` | |
| `jobs[].completedAt - startedAt` | `Job.DurationSec` | `completedAt` nol `0001-01-01T00:00:00Z` berarti belum selesai -> 0 |

## 4. Jawaban empat pertanyaan spike

1. **Badge pipeline dan status review di daftar MR (GitLab)**: TIDAK tersedia di `glab mr list -F json`. Elemen daftar tidak memuat `head_pipeline`/`pipeline` dan tidak ada info review apa pun selain array `reviewers` (hanya user, tanpa state; tidak ada `approved_by`/`approvals_before_merge`). Dengan `glab mr view` pun hanya `head_pipeline`/`pipeline` yang ada; state review tetap tidak ada. Putusan GitLab: badge pipeline di baris MR DIHILANGKAN (butuh panggilan per MR); status review di baris MR DIHILANGKAN dan juga tidak ada di detail (`Reviewers[].State` kosong); `LastPipeline` di detail AKTIF. GitHub (ASUMSI): `reviewDecision` dan `statusCheckRollup` ada di `gh pr list --json` tanpa panggilan tambahan, jadi untuk GitHub badge pipeline dan status review di baris AKTIF; UI wajib toleran bila field kosong. Tipe `ReviewState`/`PipelineStatus` tetap `omitempty`, sehingga baris GitLab otomatis tanpa badge.
2. **Perintah pipeline**: GitLab daftar = `glab ci list -R <path> -F json [-P N]` (field di 3.3: tanpa author dan durasi). GitLab detail (stage dan job) = `glab ci get -R <path> -p <id> -F json` (non-interaktif; `jobs[]` dengan `stage`, tanpa panggilan `api`). `glab ci view`/`status` tidak dipakai (interaktif/streaming). GitHub daftar = `gh run list -R <path> --json <field> [-L N]`; detail = `gh run view <id> -R <path> --json <field>,jobs` (ASUMSI). `api` TIDAK perlu masuk allowlist.
3. **Cara membaca host login**: `glab auth status` menulis SELURUH keluarannya ke stderr dan stdout KOSONG; exit 0 bila semua host terkonfigurasi sehat, exit 1 bila ada yang gagal. Karena `RunCLI` hanya mengembalikan stdout pada sukses, teks `Logged in to <host> as <user>` TIDAK bisa dibaca dari sukses. Cara baca yang diputuskan: exit 0 = login; nama akun lewat `glab config get user --host <host>` (stdout, host utama dari `glab config get host`, bawaan `gitlab.com`). Untuk host lain: `glab config get user --host <host>` stdout kosong = tidak dikenal. GitHub: `gh auth status --json hosts` (stdout, exit 0 walau belum login; `{"hosts":{}}` = belum login; kunci `hosts` adalah host, nilainya array akun dengan `login`, `active` (ASUMSI)). `--hostname` ada di keduanya; `--show-token`/`-t` TIDAK PERNAH dipakai.
4. **`gh` tanpa login**: `gh auth status` -> exit 1, stderr `You are not logged into any GitHub hosts. To log in, run: gh auth login`, stdout kosong (NYATA, `gh-auth-status-loggedout.txt`). `gh auth status --json hosts` -> exit 0, stdout `{"hosts":{}}`, stderr teks yang sama (NYATA). `gh pr list -R x/y --json number` -> exit 4, stderr dua baris `To get started with GitHub CLI, please run:  gh auth login` dan `Alternatively, populate the GH_TOKEN environment variable ...` (NYATA, `gh-pr-list-loggedout.txt`). Fixture GitHub lain berstatus ASUMSI; verifikasi nyata menunggu `gh auth login` dan tidak memblokir.

## 5. Ringkasan temuan yang menyimpang dari nilai awal overview

- `glab auth status` tidak punya stdout; parser host/user dari stdout (overview 2.2 dan `repo.go`) tidak akan bekerja. Pengganti ada di bagian 7.
- Allowlist perlu dua tambahan baca-saja: `glab config get user|host --host` dan `gh auth status --json hosts`.
- `-R` self-hosted `HOST/PATH` tidak andal di `glab` (bagian 9); pakai URL penuh.
- `NormalizeGitHubStatus` harus peka huruf besar/kecil (rollup PR huruf besar, run huruf kecil) dan menangani `StatusContext` (bagian 6).

## 6. Pemetaan status

### GitLab (NYATA untuk nilai teramati; sisanya dari dokumentasi API, ASUMSI)

Teramati pada pipeline dan job: `success`, `failed`, `canceled`, `running`, `manual`. MR: `opened`, `merged`, `closed`.

| Nilai GitLab | `Status` | Asal |
|---|---|---|
| `success` | `success` | NYATA |
| `failed` | `failed` | NYATA |
| `canceled` | `canceled` | NYATA |
| `running` | `running` | NYATA |
| `manual` | `pending` | NYATA (pipeline menunggu aksi manual) |
| `created`, `waiting_for_resource`, `preparing`, `pending`, `scheduled` | `pending` | ASUMSI |
| `skipped` | `skipped` | ASUMSI |
| `canceling` | `running` | ASUMSI; tambahan baru dari nilai awal (masih berjalan, belum canceled) |
| lainnya | `unknown` | |

Tidak ada selisih lain dari overview 2.3 (`manual` tetap `pending`). State MR: `opened`->`open`, `merged`->`merged`, `closed`->`closed`, `locked`->`closed` (ASUMSI, tidak teramati).

### GitHub (ASUMSI; masukan dari lowercase)

Normalisasi HARUS memakai huruf kecil untuk kedua argumen (rollup PR huruf besar: `COMPLETED`, `SUCCESS`; run huruf kecil).

| `status` | `conclusion` | `Status` |
|---|---|---|
| `queued`, `waiting`, `requested`, `pending` | apa pun | `pending` |
| `in_progress` | apa pun | `running` |
| `completed` | `success` | `success` |
| `completed` | `failure`, `timed_out`, `startup_failure` | `failed` |
| `completed` | `cancelled`, `canceled` | `canceled` |
| `completed` | `skipped`, `neutral` | `skipped` |
| `completed` | `action_required` | `pending` (menunggu persetujuan manusia; SELISIH dari overview yang memetakan ke `unknown`) |
| `completed` | `stale` | `skipped` (SELISIH dari overview) |
| `completed` | lainnya/kosong | `unknown` |

`StatusContext` (entri rollup tanpa `conclusion`, bidang `state`): panggil `NormalizeGitHubStatus(state, "")` dengan tambahan `success`->`success`, `failure`/`error`->`failed`, `pending`/`expected`->`pending` (tambahan dari nilai awal). State PR: `OPEN`->`open`, `MERGED`->`merged`, `CLOSED`->`closed`.

Agregasi `PipelineStatus` PR dari `statusCheckRollup[]` (ASUMSI): kosong -> tanpa badge; ada `failed` -> `failed`; selain itu ada `running` -> `running`; selain itu ada `pending` -> `pending`; semua `success`/`skipped` (minimal satu `success`) -> `success`; selain itu `unknown`.

`reviewState`: GitLab tidak ada (kosong). GitHub: `APPROVED`->`approved`, `CHANGES_REQUESTED`->`changes_requested`, `REVIEW_REQUIRED`->`review_required`, `""`->kosong.

## 7. Host login

| Keadaan | Perintah | stdout | stderr | exit |
|---|---|---|---|---|
| glab login | `glab auth status` | KOSONG | `gitlab.com` lalu baris ` ✓ Logged in to gitlab.com as <user> (<path config>)` dan baris protokol/endpoint, `Token found: ****` | 0 |
| glab belum login | `glab auth status` (env config kosong) | kosong | `gitlab.com`, ` x gitlab.com: API call failed: GET https://gitlab.com/api/v4/user: 401 {message: 401 Unauthorized}`, ..., ` ! No token found (checked config file, keyring, and environment variables).`, blok `ERROR` ` X could not authenticate to one or more of the configured GitLab instances..` | 1 |
| glab nama akun | `glab config get user --host gitlab.com` | `<user>` + newline | (kosong; catatan versi/telemetri bisa muncul) | 0 (stdout kosong bila host tak dikenal/belum login) |
| glab host bawaan | `glab config get host` | `gitlab.com` | - | 0 |
| gh belum login | `gh auth status` | kosong | `You are not logged into any GitHub hosts. To log in, run: gh auth login` | 1 |
| gh belum login (JSON) | `gh auth status --json hosts` | `{"hosts":{}}` | teks yang sama | 0 |
| gh login | `gh auth status --json hosts` | `{"hosts":{"github.com":[{"login":...,"active":true,...}]}}` | - | 0 (ASUMSI) |
| gh login (teks) | `gh auth status` | `github.com` + ` ✓ Logged in to github.com account <user> (...)` | - | 0 (ASUMSI; stdout atau stderr tidak terverifikasi) |

Aturan parse:
- **glab, adapter `Status()`**: panggil `RunCLI("glab","auth","status")`.
  - Sukses (exit 0) -> `StateConnected`. `User`: panggil `RunCLI("glab","config","get","host")` (trim; kosong -> `gitlab.com`) lalu `RunCLI("glab","config","get","user","--host",<host>)`; trim stdout; bila gagal atau kosong biarkan `User` kosong (jangan gagalkan Status).
  - Galat `perintah glab gagal: <stderr terpotong 300>` -> bila pesan memuat salah satu dari `No token found`, `401`, `could not authenticate`, `not logged in` (tanpa peka huruf) -> `StateNotLoggedIn`. Selain itu kembalikan sebagai `error` (mis. jaringan: `no such host`, `i/o timeout`).
  - `IsCLIMissing(err)` -> `StateCLIMissing` (lebih dulu).
  - Pesan galat terpotong 300 karakter (`...` di ujung): stderr belum login di atas sudah memuat `401` dan `No token found` dalam 300 karakter pertama pada kasus satu host. Untuk host kedua dan seterusnya teks terpotong, jadi JANGAN bergantung pada baris host kedua; keputusan berbasis exit code dan penanda di atas.
  - Regex cadangan bila teks tersedia (mis. pada fallback): `Logged in to (\S+) as (\S+)` ; `^\s*x\s+(\S+):` untuk host gagal. Tahan terhadap perubahan format: bila tak terparse, jangan panic.
- **glab, host pemilih repo (`repo.go`)**: `gitlab.com` selalu dikenal tanpa cek. Host lain H: `glab config get user --host H` stdout tidak kosong = glab login ke H. Cache memori TTL 60 detik. Jangan memakai `glab auth status` untuk daftar host.
- **gh, adapter `Status()`**: panggil `RunCLI("gh","auth","status","--json","hosts")` (exit 0 pada kedua keadaan). Urai JSON: `hosts` kosong atau JSON tidak valid -> `StateNotLoggedIn`; selain itu host utama `github.com` bila ada, selain itu host pertama (urut abjad kunci); `User` = `login` akun `active` (bila tidak ada `active`, akun pertama), bentuk isi akun ASUMSI sehingga fallback `User` kosong bila tak ada `login`. Cadangan: galat dengan `You are not logged into any GitHub hosts` atau `gh auth login` -> `StateNotLoggedIn`.
- **gh, host pemilih repo**: host H dikenal bila kunci H ada di `hosts` JSON yang sama. `github.com` dikenal tanpa cek.
- Putusan pemetaan `Status()`: untuk `glab`, exit non-nol dengan penanda 401/No token = belum login (NYATA); exit non-nol lainnya = galat. Untuk `gh`, `--json hosts` exit 0 selalu; belum login ditentukan oleh `hosts` kosong, bukan exit code.
- `--show-token` tidak pernah dipakai; fixture memuat `Token found: ****` saja.

## 8. Pola galat

Stdout selalu kosong pada galat (kecuali `glab ci get` pipeline tak ada yang menulis `✘ No pipeline with the given ID: <id>` ke stdout); `RunCLI` hanya mengembalikan `perintah <name> gagal: <stderr terpotong 300>`. Catatan versi/telemetri glab di stderr bisa ikut.

| Kelas | Substring stderr yang stabil | Exit | Rekomendasi pola `classify` | Asal |
|---|---|---|---|---|
| cli_missing | bukan stderr: `connector.IsCLIMissing(err)` (`ErrCLIMissing: <name>`) | - | cek lebih dulu | NYATA (kode `cli.go`) |
| not_logged_in | glab: `No token found`, `could not authenticate`; `gh`: `gh auth login`, `To get started with GitHub CLI`, `GH_TOKEN`, `not logged in` | glab 1, gh 4 | cocokkan sebelum `not_found`; sertakan `401` hanya bersama `Unauthorized` | NYATA (token tidak ada). Token kedaluwarsa/salah: `401 Unauthorized` ASUMSI |
| permission | `403`, `Forbidden`, `Resource not accessible`, `insufficient_scope` | 1 | `Izin kurang: <pesan>` | ASUMSI: tidak teramati (repo tanpa akses di GitLab justru menjawab 404) |
| not_found | glab: `404 Not Found` (juga `Failed to get merge request N: 404 Not Found.`); gh: `Could not resolve to a Repository`, `HTTP 404`, `Not Found` | 1 | cocokkan `404` / `Not Found` / `Could not resolve` | NYATA (glab), ASUMSI (gh). GitLab menjawab 404 untuk repo tidak ada DAN repo privat tanpa akses DAN tanpa token, jadi `not_found` memakai pesan "tidak ditemukan atau tidak ada akses" |
| timeout | `perintah <name> melewati batas waktu` (dari `RunCLITimeout`); ctx batal menunggu semaphore | - | cocokkan `melewati batas waktu`, `context deadline exceeded`, `i/o timeout` | NYATA (kode `cli.go`) |
| jaringan | `dial tcp: lookup <host>: no such host`, `connection refused`, `TLS handshake` | 1 | petakan ke `cli_error` (pesan "Host tidak dapat dijangkau"), BUKAN `timeout` (pesan timeout menyebut 20 detik) | NYATA (`no such host`), sisanya ASUMSI |
| cli_error | selain di atas | non-nol | ringkas stderr | - |

Urutan cek: cli_missing, timeout, not_logged_in, permission, not_found, jaringan, cli_error. Contoh teks nyata ada di `glab-errors.txt`.

Konsekuensi pemotongan 300 karakter: teks `glab auth status` belum login (kasus satu host) muat; stderr gabungan banyak host terpotong, jadi `classify` dan `Status()` memakai penanda yang muncul di awal teks. `RunCLI` hanya mengembalikan stdout pada sukses dan stdout itu kosong untuk `auth status`.

## 9. Remote git dan bentuk `-R`

`git remote` mengurutkan nama abjad, jadi pilih `origin` bila ada, jika tidak nama pertama. Project tanpa remote: `git remote` exit 0 stdout kosong (NYATA); `git remote get-url origin` exit 2 `error: No such remote 'origin'`; path tidak ada: `git -C <path> remote` exit 128 `fatal: cannot change to ...`. Ketiganya diabaikan diam-diam.

Contoh `git remote get-url <nama>` (NYATA; nilai palsu) dan hasil urai yang diharapkan:

| Keluaran | host | path |
|---|---|---|
| `git@gitlab.com:acme-group/platform/acme-app.git` (scp-like, subgroup) | `gitlab.com` | `acme-group/platform/acme-app` |
| `git@github.com:acme-org/acme-repo.git` | `github.com` | `acme-org/acme-repo` |
| `ssh://git@gitlab.com/acme-group/acme-app.git` | `gitlab.com` | `acme-group/acme-app` |
| `ssh://git@git.acme.example:2222/acme-group/acme-app.git` (port) | `git.acme.example` | `acme-group/acme-app` |
| `https://USER@gitlab.com/acme-group/acme-app.git` (kredensial) | `gitlab.com` | `acme-group/acme-app` |
| `https://github.com/acme-org/acme-repo` (tanpa `.git`) | `github.com` | `acme-org/acme-repo` |
| `https://github.com/acme-org/acme-repo.git/` (slash ujung) | `github.com` | `acme-org/acme-repo` |

`-R` GitLab:
- `OWNER/REPO` untuk `gitlab.com` (NYATA) dan `GROUP/SUB/REPO` bertingkat (NYATA; contoh empat segmen berhasil).
- URL penuh `https://gitlab.com/<path>` diterima (NYATA, exit 0).
- `gitlab.com/<path>` (tanpa skema) juga berhasil untuk host bawaan (NYATA).
- `HOST/PATH` untuk host yang BUKAN bawaan tidak andal: `glab mr list -R tidak-ada.example.invalid/a/b` tidak menuju host itu melainkan dijawab `404 Not Found` (segmen pertama dianggap grup di `gitlab.com`), sedangkan `-R https://tidak-ada.example.invalid/a/b` benar-benar menuju host itu (galat DNS). Putusan: untuk self-hosted gunakan URL penuh `https://HOST/PATH`; untuk `gitlab.com` cukup `PATH`. Ini menyimpang dari `RepoFlag` overview 2.3 (`HOST/PATH`). Self-hosted nyata tidak teramati (tidak ada host login lain): ASUMSI.
- `gh`: `OWNER/REPO` dan `HOST/OWNER/REPO` (NYATA dari help `-R, --repo [HOST/]OWNER/REPO`; GHE ASUMSI).

## 10. Pengecualian dan pemotongan fitur

| Butir | Keputusan | Dampak ke tiket 02 sampai 04 | Fitur yang menyesuaikan |
|---|---|---|---|
| Badge pipeline di baris MR | GitLab: DIHILANGKAN (field tidak ada di daftar, butuh satu panggilan per MR). GitHub: AKTIF (ASUMSI, dari `statusCheckRollup`) | Tiket 02 menampilkan badge hanya bila `pipelineStatus` terisi; baris GitLab tanpa badge, pipeline MR dilihat di detail | `gitdata-foundation` (tipe tetap omitempty), tiket 02 (UI) |
| Status review di baris MR | GitLab: DIHILANGKAN (tidak ada state review di CLI). GitHub: AKTIF (ASUMSI, `reviewDecision`) | Tiket 02 menampilkan `reviewState` hanya bila terisi | tiket 02, tiket 04 |
| Detail pipeline GitLab (stage/job) | TERSEDIA lewat `glab ci get -p <id> -F json` (`jobs[]` dengan `stage`); stage terurut diturunkan dari id job naik; `jobs: null` -> `Stages: []` | Tiket 03 memanggil `ci get` per pipeline; tanpa `api` | tiket 03 |
| Daftar pipeline GitLab tanpa author dan durasi | `Author` kosong dan `DurationSec` 0 di daftar; terisi di detail | Tiket 03 jangan menampilkan kolom itu di daftar GitLab | tiket 03 |
| `Mergeable` di detail MR | AKTIF: GitLab dari `has_conflicts` lalu `detailed_merge_status` (`not_open` -> kosong); GitHub dari `mergeable` (ASUMSI). Tipe tetap string `omitempty` | Tiket 04 menampilkan teks apa adanya | tiket 04 |
| `LastPipeline` | AKTIF untuk GitLab dari `head_pipeline` (fallback `pipeline`); null -> nil. GitHub: nil (tidak ada pipeline per PR selain rollup) | Tiket 04 tampilkan hanya bila ada | tiket 04 |
| `Reviewers[].State` | GitLab kosong (tidak tersedia); GitHub dari `latestReviews` (ASUMSI) | Tiket 04 tampilkan nama saja bila state kosong | tiket 04 |
| Baca login `glab` | stdout kosong -> `Status()` dan `repo.go` memakai exit code dan `glab config get`; allowlist ditambah dua perintah | Tidak berdampak ke tiket 02 sampai 04 | `git-connector-adapters`, `gitdata-foundation` (diselaraskan saat Wiring butir 6) |
| `RepoFlag` self-hosted | URL penuh `https://HOST/PATH` untuk `glab` | Tiket 02 sampai 04 selalu memakai `RepoFlag` | `gitdata-foundation` (`repo.go`) |
| GitHub tanpa login | Fixture `gh-*` ASUMSI; verifikasi nyata menunggu `gh auth login`, tidak memblokir | Parser GitHub harus toleran terhadap field kosong dan perbedaan huruf | semua fitur sisi GitHub |

## 11. Ringkasan keputusan

- **Allowlist**: `glab auth status` (`--hostname`, `-a/--all`); `glab config get user|host` (`--host`); `glab mr list` / `mr view`; `glab ci list` / `ci get -p`; `gh auth status` (`--hostname`, `--json hosts`); `gh pr list` / `pr view`; `gh run list` / `run view`; `git [-C path] remote` / `remote get-url <nama>`. Flag per subcommand seperti bagian 2. Tolak `--show-token`, `-t` pada auth, `--with-variables`, `-w/--web`, `--log*`, `api`, semua perintah tulis, dan key `config get` selain `user`/`host`.
- **Bentuk `-R`**: GitLab `PATH` (gitlab.com) atau `https://HOST/PATH` (self-hosted); GitHub `OWNER/REPO` atau `HOST/OWNER/REPO`. JSON: `-F json` (glab), `--json <field>` (gh).
- **Pemetaan status**: tabel bagian 6 (GitLab `manual` -> `pending`, `canceling` -> `running`; GitHub huruf kecil, `action_required` -> `pending`, `stale` -> `skipped`, `StatusContext` didukung). State MR: `opened`->`open`, `locked`->`closed`.
- **Pola galat** (`classify`): cli_missing, timeout (`melewati batas waktu`), not_logged_in (`No token found`, `could not authenticate`, `gh auth login`, `To get started with GitHub CLI`), permission (`403`, `Forbidden`), not_found (`404`, `Not Found`, `Could not resolve`), jaringan -> cli_error, lainnya cli_error.
- **Host login**: glab exit 0 = login, nama akun lewat `glab config get user --host <host>`; belum login = exit 1 dengan `No token found`/`401`. gh lewat `gh auth status --json hosts` (`hosts` kosong = belum login). Host utama `gitlab.com`/`github.com` bila ada, selain itu host pertama. Pemilih repo: `glab config get user --host H` tak kosong, atau `H` ada di `hosts` gh.
- **Bentuk opsional tipe**: `ReviewState` dan `PipelineStatus` kosong untuk GitLab (omitempty); `Mergeable` string (`conflict` | nilai `detailed_merge_status` | `mergeable`/`conflict`/`unchecked` untuk GitHub); `LastPipeline` dari `head_pipeline`; `Reviewers[].State` kosong untuk GitLab; `Pipeline.Author` dan `DurationSec` hanya di detail GitLab.
- **Badge baris MR**: GitLab tanpa badge pipeline dan tanpa status review; GitHub dengan keduanya (ASUMSI).
