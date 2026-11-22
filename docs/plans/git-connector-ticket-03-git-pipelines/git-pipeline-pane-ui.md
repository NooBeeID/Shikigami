# Plan: Pane git-pipeline dan StageList

## Metadata

- Tiket: `git-connector-ticket-03-git-pipelines` (Daftar pipeline dan detail pipeline)
- Fitur: `git-pipeline-pane-ui` (Pane git-pipeline dan StageList)
- ownedPaths:
  - `frontend/src/modules/git/PipelinePane.svelte`
  - `frontend/src/modules/git/StageList.svelte`
- Depends on foundation: tidak (`hasFoundation=false`; tipe, `StatusBadge`, `actions.ts`, pane kind, field leaf sudah ada dari tiket 1 dan 2)
- Rujukan kontrak bersama: `docs/plans/git-connector-ticket-03-git-pipelines/overview.md` (bagian 0, 2.2, 2.6, 2.7, 6). Jangan diubah.

## Tujuan

Mengganti isi placeholder `PipelinePane.svelte` dan membuat `StageList.svelte` sehingga pane tengah `git-pipeline` menampilkan header pipeline (status keseluruhan, ref, sha 7 karakter, durasi, penulis bila ada, aksi Buka/Salin/Kirim ke agen pada URL pipeline, Segarkan) dan daftar stage berisi job (nama sebagai link, status, durasi, aksi memakai URL job). Pane memuat data lewat `getPipeline` dari `pipelineClient.ts` saat mount/dipulihkan dan saat Segarkan, mengabaikan respons usang, serta menampilkan keadaan memuat, galat terbaca, dan keadaan aman (project/pipeline/workspace hilang, aksi nonaktif) tanpa crash.

## Path yang dimiliki

HANYA dua file ini yang boleh dibuat/diubah:
- `frontend/src/modules/git/PipelinePane.svelte`
- `frontend/src/modules/git/StageList.svelte`

Jangan menyentuh: `pipelineClient.ts`, `PipelineList.svelte`, `PipelineRow.svelte` (fitur `git-pipelines-list-ui`, dikerjakan agen lain), file wiring, `index.ts`, `layout/model.ts`, `actions.ts`, `client.ts`, `store.ts`, `StatusBadge.svelte`, `MrPane.svelte`, `MrRow.svelte`, `MrList.svelte`, `ProjectSection.svelte`, file Go, dan file lain apa pun. Membaca dan mengimpor dari file-file itu boleh.

## Kontrak

Dipakai (tidak didefinisikan ulang; bentuk persis di overview 2.2 dan 2.6):
- `./pipelineClient` (dibuat fitur `git-pipelines-list-ui`; JANGAN dibuat di sini, dan JANGAN membuat klien IPC lain):
  - `getPipeline(params: { workspaceId: string; projectId: string; id: string }): Promise<PipelineDetail>` (melempar `GitError` hasil `parseGitError`; `stages` dijamin array).
  - Tipe: `PipelineDetail` (= `Pipeline` + `stages`), `PipelineStage { name: string; jobs: PipelineJob[] }`, `PipelineJob { name: string; status: PipelineStatus; durationSec?: number; url?: string }`. Impor tipe dengan `import type`.
  - `durationSec` undefined/null/`0` berarti tidak tersedia, tampilkan `-`. `author` opsional.
- `./client`: `parseGitError`, `type GitError {code, message}`, `type Provider`.
- `./actions`: `openInBrowser(url)`, `copyLink(url)` (async), `sendToAgent(url)` (async; toast "Tidak ada sesi aktif" sudah di dalamnya).
- `./StatusBadge.svelte`: `<StatusBadge status={x} kind="pipeline" />`.
- `../../lib/stores/workspaces`: `activeWorkspaceId` (dibaca lewat `get()` dari `svelte/store`).
- `../settings`: `openSettings('connectors')` untuk galat `cli_missing`/`not_logged_in`.
- `../layout/model`: `type Leaf` (field dipakai: `pane.title`, `pane.provider`, `pane.projectId`, `pane.pipelineId`; BUKAN `pane.id`, itu id leaf).

Disediakan:
- `PipelinePane.svelte`: props `{ pane: Leaf; visible: boolean }` (tidak berubah dari placeholder); wrapper luar tetap `<div class="git-pane" hidden={!visible}>`.
- `StageList.svelte`: props `{ stages: PipelineStage[] }`.

Pola acuan wajib diikuti: `frontend/src/modules/git/MrList.svelte` (seq, `$effect` + `untrack`, `get(activeWorkspaceId)`, kotak galat dengan "Coba lagi" dan "Buka Pengaturan > Connector") dan `MrRow.svelte` (tombol aksi ikon dengan `aria-label`, `title`, `stopPropagation`). Format durasi (fungsi lokal di tiap file, overview bagian 2.6): `<60` -> `"Ns"`; `<3600` -> `"Nm Ns"` (mis. `1m 23s`); selain itu `"Nj Nm"`; tidak tersedia (undefined/null/<=0) -> `"-"`.

## Langkah

1. Baca dulu (tanpa mengubah): `frontend/src/modules/git/MrList.svelte`, `MrRow.svelte`, `StatusBadge.svelte`, `actions.ts`, `client.ts`, `frontend/src/modules/layout/model.ts` (pastikan `Leaf` punya `provider?`, `projectId?`, `pipelineId?`, `title?`). Periksa apakah `pipelineClient.ts` sudah ada; bila belum, tetap impor sesuai kontrak (jangan membuatnya).

2. Buat `frontend/src/modules/git/StageList.svelte` (Svelte 5 runes, `<script lang="ts">`):
   - Impor: `import type { PipelineStage, PipelineJob } from './pipelineClient';`, `import { openInBrowser, copyLink, sendToAgent } from './actions';`, `import StatusBadge from './StatusBadge.svelte';`.
   - `let { stages }: { stages: PipelineStage[] } = $props();`
   - Fungsi lokal `formatDuration(sec?: number | null): string` sesuai format di atas (`"-"` bila tidak ada atau <= 0; bulatkan dengan `Math.round`; `>=3600` -> `${h}j ${m}m`).
   - Markup:
     - Bila `stages.length === 0`: `<p class="muted">Tidak ada stage</p>`.
     - Selain itu `{#each stages as stage, i (stage.name + ':' + i)}` merender `<section class="stage">` berisi header `<div class="stage-head"><span class="stage-name">{stage.name}</span><span class="count">{stage.jobs.length} job</span></div>` (toleran `stage.jobs ?? []`). Stage tanpa job: `<p class="muted">Tidak ada job</p>`.
     - Tiap job `{#each jobs as job, j (job.name + ':' + j)}` satu baris `<div class="job">`: nama (bila `job.url` ada -> `<button type="button" class="link" onclick={() => openInBrowser(job.url!)}>{job.name}</button>` atau `<a href={job.url} onclick={(e)=>{e.preventDefault(); openInBrowser(job.url!);}}>`; tanpa url -> `<span class="name">{job.name}</span>`), `<StatusBadge status={job.status} kind="pipeline" />`, durasi `formatDuration(job.durationSec)`, dan tiga tombol aksi (`↗` Buka di browser, `⧉` Salin link, `➤` Kirim ke agen) dengan `aria-label` dan `title` Indonesia ("Buka di browser", "Salin link", "Kirim ke agen") memanggil `openInBrowser(job.url)`, `void copyLink(job.url)`, `void sendToAgent(job.url)` dengan URL JOB (bukan URL pipeline).
     - Aksi nonaktif bila `!job.url`: atribut `disabled`, `title="Job ini tidak punya link"`.
   - Stile `<style>` scoped dengan variabel tema dan fallback (`--fg`, `--fg-muted`, `--bg-elev`, `--bg-hover`, `--border`, `--accent`), meniru kelas `.act`, `.muted` dari `MrRow`/`MrList`; `.act:disabled { opacity: .4; cursor: not-allowed; }`; fokus keyboard terlihat (`:focus-visible`). Nama job dipotong dengan ellipsis (`min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap`). Warna status HANYA lewat `StatusBadge`.

3. Ganti isi `frontend/src/modules/git/PipelinePane.svelte` (props tetap `{ pane, visible }`):
   - Impor: `get` dari `svelte/store`, `untrack` dari `svelte`, `activeWorkspaceId`, `openSettings` dari `../settings`, `type Leaf` dari `../layout/model`, `parseGitError`/`type GitError` dari `./client`, `getPipeline`/`type PipelineDetail` dari `./pipelineClient`, `openInBrowser, copyLink, sendToAgent` dari `./actions`, `StatusBadge`, `StageList`.
   - State: `type Phase = 'loading' | 'ready' | 'error' | 'missing'`; `let phase = $state<Phase>('loading'); let detail = $state<PipelineDetail | null>(null); let err = $state<GitError | null>(null); let seq = 0;`.
   - `async function load(provider?: string, projectId?: string, pipelineId?: string)`:
     - `const my = ++seq;`
     - Bila `provider`, `projectId` atau `pipelineId` kosong, atau `get(activeWorkspaceId) === null`: `detail = null; err = null; phase = 'missing'; return;` (tanpa memanggil IPC).
     - `phase = 'loading'; err = null;` lalu `try { const r = await getPipeline({ workspaceId: ws, projectId, id: pipelineId }); if (my !== seq) return; detail = r; phase = 'ready'; } catch (x) { if (my !== seq) return; err = parseGitError(x); detail = null; phase = 'error'; }`.
     - Pada Segarkan, jaga detail lama tetap tampil selama memuat bila `detail` sudah ada (opsional); minimal tombol Segarkan `disabled` saat `phase === 'loading'`.
   - Muat saat mount/dipulihkan dan saat identitas berubah: `$effect(() => { const p = pane.provider, pr = pane.projectId, id = pane.pipelineId; untrack(() => { void load(p, pr, id); }); return () => { seq++; }; });` (cleanup membuang respons usang saat `pipelineId` berganti atau pane di-unmount). Jangan memuat ulang hanya karena `visible` berubah; jangan polling.
   - `function refresh() { void load(pane.provider, pane.projectId, pane.pipelineId); }` dan `retry` memakai fungsi yang sama.
   - Turunan: `title = $derived(pane.title || 'Pipeline ' + (pane.pipelineId ?? ''))`; `url = $derived(detail?.url ?? '')`; `canAct = $derived(phase === 'ready' && url !== '')`; `shortSha = $derived((detail?.sha ?? '').slice(0, 7))`; `errTitle` dan `needSettings` seperti `MrList` (kode `cli_missing`, `not_logged_in`, `permission`, `timeout`, `not_found`, default `err.message || 'Terjadi galat'`); format durasi lokal `formatDuration` (fungsi yang sama seperti di StageList; didefinisikan ulang karena tiap file mandiri).
   - Markup (wrapper luar `<div class="git-pane" hidden={!visible}>`):
     - `.head`: baris 1 judul (`{title}`) dan `.acts` berisi tombol `↗` Buka di browser, `⧉` Salin link, `➤` Kirim ke agen (memakai `url`, `disabled={!canAct}`; `aria-label`/`title` Indonesia) serta tombol `Segarkan` (`disabled={phase === 'loading'}`, `onclick={refresh}`; boleh tetap aktif pada `error` agar bisa dicoba lagi, nonaktif pada `missing`). Baris 2 (hanya bila `detail`): `<StatusBadge status={detail.status} kind="pipeline" />`, ref (`detail.ref`, dengan `title`), `shortSha` (monospace), durasi `formatDuration(detail.durationSec)`, penulis `{#if detail.author}...{/if}`.
     - Isi: `phase === 'loading'` (dan belum ada detail) -> `<p class="muted">Memuat...</p>`; `phase === 'missing'` -> `<p class="muted">Project atau pipeline tidak ditemukan</p>` (aksi nonaktif); `phase === 'error'` -> kotak `.errbox role="alert"` dengan `errTitle`, tombol "Buka Pengaturan > Connector" bila `needSettings` (`openSettings('connectors')`), dan "Coba lagi" (`retry`); `phase === 'ready' && detail` -> `<div class="body"><StageList stages={detail.stages ?? []} /></div>` dengan `overflow: auto` agar pane bisa digulir.
   - Stile: meniru placeholder (`.git-pane { display:flex; flex-direction:column; height:100%; color: var(--fg,#ddd) } .git-pane[hidden] { display:none }`), `.head` berlatar `var(--bg-elev,#222)` dengan border bawah `var(--border,#333)`, `.body { flex:1; min-height:0; overflow:auto }`, kelas `.act`, `.muted`, `.errbox`, `.acts button` disalin gayanya dari `MrRow`/`MrList` (variabel tema dengan fallback; `:focus-visible` terlihat; `.act:disabled` redup). Warna status hanya lewat `StatusBadge`.

4. Jalankan guard (lihat "Selesai bila"). Bila build gagal HANYA karena `./pipelineClient` belum ada (fitur `git-pipelines-list-ui` paralel belum selesai), itu diharapkan: pastikan tidak ada galat lain (mis. dengan `npm run check` dan memeriksa bahwa semua galat merujuk modul itu), catat di laporan, jangan membuat file itu. Verifikasi penuh dilakukan oleh wiring.

## Perubahan

- Diubah: `frontend/src/modules/git/PipelinePane.svelte` (placeholder diganti pane lengkap).
- Dibuat: `frontend/src/modules/git/StageList.svelte`.
- Tidak ada file lain dibuat atau diubah; tanpa unit test, tanpa dependensi baru, tanpa polling.

## Wiring

Fitur ini hanya mengekspos dua komponen; penyambungan dilakukan oleh `git-pipelines-wiring`, bukan di sini:
- `PipelinePane.svelte` sudah terdaftar sebagai pane kind `git-pipeline` di `modules/git/index.ts` (props `{pane, visible}` tidak berubah); tidak ada perubahan di file itu.
- Pane membaca `pane.provider`, `pane.projectId`, `pane.pipelineId` yang diisi `PipelineRow` lewat `layout.openPane({ kind: 'git-pipeline', ... })` (fitur `git-pipelines-list-ui`); dipersist/dipulihkan oleh `layout/model.ts` yang sudah ada.
- Pane bergantung pada `getPipeline` di `pipelineClient.ts` (fitur `git-pipelines-list-ui`) yang memanggil IPC `git.pipeline.get` (fitur `git-pipelines-data`).
- `StageList.svelte` dipakai hanya oleh `PipelinePane.svelte` dengan prop `stages`.
- Yang dicek wiring: pane memuat stage dan job, aksi header memakai URL pipeline dan aksi baris job memakai URL job, pane dipulihkan setelah restart, repo hilang/CLI gagal menampilkan keadaan aman.

## Selesai bila

- [ ] `PipelinePane.svelte` memuat `getPipeline` dari `./pipelineClient` saat mount/dipulihkan dan saat Segarkan; respons usang dibuang lewat `seq`; tidak ada klien IPC lain.
- [ ] Header menampilkan `StatusBadge` keseluruhan, ref, sha 7 karakter, durasi (`-` bila tidak ada), penulis bila ada, tombol Buka/Salin/Kirim ke agen (URL pipeline), dan Segarkan.
- [ ] Keadaan memuat, galat (dengan "Coba lagi" dan ajakan Pengaturan > Connector untuk `cli_missing`/`not_logged_in`), dan aman bila provider/projectId/pipelineId kosong atau workspace aktif null ("Project atau pipeline tidak ditemukan", aksi nonaktif) tampil tanpa crash.
- [ ] `StageList.svelte` (props `{stages}`) menampilkan stage dengan nama dan jumlah job, baris job (nama link via `openInBrowser` bila ada url, `StatusBadge`, durasi, Buka/Salin/Kirim ke agen memakai URL job; nonaktif dengan `title` "Job ini tidak punya link" bila tanpa url), "Tidak ada job" dan "Tidak ada stage".
- [ ] Hanya dua file di ownedPaths yang berubah (`git status` tidak menunjukkan perubahan lain dari fitur ini); `pipelineClient.ts` tidak dibuat.
- [ ] Guard: `cd frontend && npm run build` (tambahkan `npm run check` bila perlu) sukses untuk path fitur ini. Bila satu-satunya galat adalah modul `./pipelineClient` yang belum ada karena fitur paralel belum selesai, dicatat di laporan dan dianggap normal.
