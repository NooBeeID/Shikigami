# Plan: UI daftar pipeline di panel

## Metadata

- Tiket: `git-connector-ticket-03-git-pipelines` (`docs/tickets/git-connector-ticket-03-git-pipelines.md`)
- Fitur: `git-pipelines-list-ui` (UI daftar pipeline di panel)
- Overview kontrak: `docs/plans/git-connector-ticket-03-git-pipelines/overview.md` (bagian 0, 2.1, 2.2, 2.6, 2.7, 6, 7)
- ownedPaths:
  - `frontend/src/modules/git/PipelineList.svelte`
  - `frontend/src/modules/git/PipelineRow.svelte`
  - `frontend/src/modules/git/pipelineClient.ts`
- Bergantung pada foundation: tidak (`hasFoundation=false`; semua titik ekstensi sudah ada dari tiket 1 dan 2).

## Tujuan

Mengganti placeholder `PipelineList.svelte` dengan daftar 20 pipeline (GitLab) / run (GitHub Actions) terbaru per project, membuat `PipelineRow.svelte`, dan menyediakan `pipelineClient.ts` sebagai satu-satunya pembungkus IPC pipeline (dipakai juga oleh `PipelinePane` milik fitur lain). Klik baris membuka pane `git-pipeline`. Tanpa unit test, tanpa dependensi baru, tanpa polling.

## Path yang dimiliki

HANYA tiga file ini yang boleh dibuat/diubah:
- `frontend/src/modules/git/pipelineClient.ts` (baru)
- `frontend/src/modules/git/PipelineList.svelte` (ganti isi placeholder; props tetap)
- `frontend/src/modules/git/PipelineRow.svelte` (baru)

Jangan sentuh file wiring atau path fitur lain: `PipelinePane.svelte`, `StageList.svelte`, `ProjectSection.svelte`, `StatusBadge.svelte`, `actions.ts`, `client.ts`, `store.ts`, `mrClient.ts`, `MrList.svelte`, `MrRow.svelte`, `index.ts`, `layout/model.ts`, file Go apa pun. Boleh dibaca dan diimpor.

## Kontrak

Disediakan (dipegang persis; dipakai `git-pipeline-pane-ui`, yang mengimpor file ini):

```ts
// frontend/src/modules/git/pipelineClient.ts
export type PipelineStatus = 'pending'|'running'|'success'|'failed'|'canceled'|'skipped'|'unknown';
export interface Pipeline {
  id: string; ref: string; sha: string; status: PipelineStatus;
  author?: string; durationSec?: number; updatedAt: string; url: string;
}
export interface PipelineJob { name: string; status: PipelineStatus; durationSec?: number; url?: string }
export interface PipelineStage { name: string; jobs: PipelineJob[] }
export interface PipelineDetail extends Pipeline { stages: PipelineStage[] }
export async function listPipelines(params: { workspaceId: string; projectId: string }): Promise<Pipeline[]>;
export async function getPipeline(params: { workspaceId: string; projectId: string; id: string }): Promise<PipelineDetail>;
```

IPC (overview 2.1/2.2, diimplementasi fitur `git-pipelines-data`, tidak perlu ada agar build lulus):
- `git.pipelines.list` params `{workspaceId, projectId}` -> array `Pipeline[]` langsung (bukan objek pembungkus; klien toleran `null`).
- `git.pipeline.get` params `{workspaceId, projectId, id}` -> `PipelineDetail`.
- `durationSec` undefined/null/`0` = tidak tersedia. GitLab daftar: `author` dan durasi selalu kosong.

Dipakai (jangan didefinisikan ulang):
- `call` dari `../../lib/ipc`; `parseGitError`, `Provider`, `Repo`, `GitError` dari `./client`.
- `openInBrowser`, `copyLink`, `sendToAgent` dari `./actions`.
- `StatusBadge` dari `./StatusBadge.svelte`, props `{status: string; kind?: 'pipeline'|'review'|'mr'}`; pakai `kind="pipeline"`.
- `layout` dari `../../lib/stores/layout`: `layout.openPane({ kind: 'git-pipeline', provider, projectId, pipelineId, title })` (field leaf bernama `pipelineId`, bukan `id`; sudah ada di `NewLeaf`). `workspaceId` leaf tidak diisi.
- `activeWorkspaceId` dari `../../lib/stores/workspaces`; `openSettings` dari `../settings`.

## Langkah

1. Baca `MrList.svelte`, `MrRow.svelte`, `mrClient.ts` sebagai pola (jangan diubah).

2. Buat `frontend/src/modules/git/pipelineClient.ts`:
   - `import { call } from '../../lib/ipc'; import { parseGitError } from './client';`
   - Ekspor tipe persis seperti bagian Kontrak.
   - `listPipelines`: `let r: Pipeline[] | null | undefined; try { r = await call<Pipeline[] | null>('git.pipelines.list', params); } catch (e) { throw parseGitError(e); } return Array.isArray(r) ? r : [];`
   - `getPipeline`: `let r: PipelineDetail; try { r = await call<PipelineDetail>('git.pipeline.get', params); } catch (e) { throw parseGitError(e); } return { ...r, stages: Array.isArray(r.stages) ? r.stages : [] };`
   - Tidak ada klien IPC pipeline lain di file mana pun.

3. Buat `frontend/src/modules/git/PipelineRow.svelte` (Svelte 5 runes):
   - Props: `let { pipeline, provider, onOpen }: { pipeline: Pipeline; provider: Provider; onOpen: (p: Pipeline) => void } = $props();`
   - Fungsi lokal `relativeTime(iso)` salin pola `MrRow` ("baru saja", "N menit lalu", "N jam lalu", "N hari lalu", "N bulan lalu", "N tahun lalu"; string kosong bila tidak terurai).
   - Fungsi lokal `formatDuration(sec?: number): string`: `undefined`/`null`/`<=0`/NaN -> `'-'`; `<60` -> `"Ns"`; `<3600` -> `"Nm Ns"` (mis. `1m 23s`); selain itu `"Nj Nm"`. Bulatkan dengan `Math.round` dulu.
   - Turunan: `isTag = pipeline.ref.startsWith('refs/tags/')`; glyph teks sederhana `⑂` untuk branch dan `◆` untuk tag (tanpa emoji), dengan `title` "Branch"/"Tag". `shortSha = pipeline.sha.slice(0, 7)`; `rel = relativeTime(pipeline.updatedAt)`; `hasDuration = (pipeline.durationSec ?? 0) > 0`. Ref ditampilkan apa adanya (termasuk `refs/merge-requests/<iid>/head`); `title` atribut berisi ref penuh, teks dipotong ellipsis.
   - Markup: `<div class="row" role="button" tabindex="0" onclick={() => onOpen(pipeline)} onkeydown={onKey}>` dengan komentar `<!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->` seperti `MrRow`. `onKey`: abaikan bila `e.target !== e.currentTarget`; Enter atau Spasi -> `preventDefault()` lalu `onOpen(pipeline)`.
   - Baris 1: glyph + ref (flex 1, ellipsis), `<StatusBadge status={pipeline.status} kind="pipeline" />`, lalu tiga tombol ikon `.act` (`↗` aria-label "Buka di browser", `⧉` "Salin link", `➤` "Kirim ke agen"), masing-masing `type="button"`, `onclick` memanggil `e.stopPropagation()` lalu `openInBrowser(pipeline.url)` / `void copyLink(pipeline.url)` / `void sendToAgent(pipeline.url)`, dan `onkeydown={(e) => e.stopPropagation()}`. Tombol selalu memakai URL pipeline.
   - Baris 2 (`.meta`): `<span class="sha">{shortSha}</span>`; `{#if pipeline.author}<span>{pipeline.author}</span>{/if}`; durasi: `{#if hasDuration}<span>{formatDuration(pipeline.durationSec)}</span>{:else if provider === 'github'}<span>-</span>{/if}` (GitLab daftar tanpa durasi: kolom disembunyikan, bukan "-"); `{#if rel}<span title={pipeline.updatedAt}>{rel}</span>{/if}`.
   - Gaya: salin gaya `MrRow` (variabel `--fg`, `--fg-muted`, `--bg-elev`, `--bg-hover`, `--border`, `--accent` dengan fallback); `.sha` pakai `font-family: var(--font-mono, monospace)`. Warna status hanya lewat `StatusBadge`.

4. Ganti isi `frontend/src/modules/git/PipelineList.svelte` (props tetap `{ provider: Provider; repo: Repo }`), mengikuti `MrList` tanpa segmented filter:
   - Impor: `get` dari `svelte/store`, `untrack` dari `svelte`, `activeWorkspaceId`, `layout`, `openSettings`, `parseGitError`/`Provider`/`Repo`/`GitError`, `listPipelines`/`Pipeline` dari `./pipelineClient`, `PipelineRow`.
   - State: `status = $state<'loading'|'ready'|'error'>('loading')`, `pipelines = $state<Pipeline[]>([])`, `err = $state<GitError | null>(null)`, `let seq = 0`.
   - `async function load(repoArg: Repo)`: `const my = ++seq; const ws = get(activeWorkspaceId);` bila `ws === null` -> `pipelines = []; err = null; status = 'ready'; return;`. Selain itu `status = 'loading'; err = null;` lalu `try { const r = await listPipelines({ workspaceId: ws, projectId: repoArg.projectId }); if (my !== seq) return; pipelines = r; status = 'ready'; } catch (x) { if (my !== seq) return; err = parseGitError(x); pipelines = []; status = 'error'; }`.
   - `$effect(() => { const r = repo; untrack(() => { void load(r); }); return () => { seq++; }; });` (bergantung pada identitas `repo`, sehingga Segarkan panel memuat ulang; tidak ada polling).
   - `retry()` -> `void load(repo)`.
   - `openPipeline(p: Pipeline)`: `layout.openPane({ kind: 'git-pipeline', provider, projectId: repo.projectId, pipelineId: p.id, title: 'Pipeline ' + p.id + ' (' + p.ref + ')' })`.
   - `errTitle` dan `needSettings` sama dengan `MrList` (cli_missing -> `err.message || '<glab|gh> belum terpasang'`, not_logged_in -> "Belum login", permission -> "Izin kurang", timeout -> "Waktu habis", not_found -> `err.message`, default -> `err.message || 'Terjadi galat'`; `needSettings` untuk `cli_missing`/`not_logged_in`).
   - Template: wrapper `<div class="pipelines">`; `loading` -> `<p class="muted">Memuat...</p>`; `error` -> `.errbox role="alert"` berisi `errTitle`, tombol "Buka Pengaturan > Connector" (`openSettings('connectors')`, hanya bila `needSettings`) dan "Coba lagi"; `pipelines.length === 0` -> `<p class="muted">Tidak ada pipeline</p>` (GitLab dan GitHub sama); selain itu `{#each pipelines as p (p.id)}<PipelineRow pipeline={p} {provider} onOpen={openPipeline} />{/each}`.
   - Salin blok `<style>` `.muted`, `.errbox`, `.acts` dari `MrList`.
   - Galat terisolasi di bagian ini; tidak melempar keluar komponen.

5. Guard: dari `frontend/` jalankan `npm run build`. Bila perlu, tambah `npm run check` dan perbaiki galat tipe/a11y hanya di tiga file milik fitur ini. Galat dari file fitur lain (mis. `PipelinePane` belum selesai) dicatat di laporan, bukan diperbaiki.

## Perubahan

- Baru: `frontend/src/modules/git/pipelineClient.ts`
- Baru: `frontend/src/modules/git/PipelineRow.svelte`
- Diubah (isi placeholder diganti, props tetap): `frontend/src/modules/git/PipelineList.svelte`

## Wiring

Yang diekspos untuk disambungkan langkah wiring (jangan disambungkan sendiri):
- `pipelineClient.ts`: `listPipelines`, `getPipeline`, dan tipe `PipelineStatus`, `Pipeline`, `PipelineJob`, `PipelineStage`, `PipelineDetail`, untuk diimpor `PipelinePane`/`StageList`.
- `PipelineList.svelte` dengan props `{provider, repo}` yang sudah dirender `ProjectSection` (tidak ada perubahan di sana).
- `PipelineRow` membuka pane lewat `layout.openPane({ kind: 'git-pipeline', provider, projectId, pipelineId, title })`; wiring memverifikasi `PipelinePane` membaca `pane.pipelineId`.
- Kebutuhan IPC `git.pipelines.list` dan `git.pipeline.get` dipenuhi fitur `git-pipelines-data`; bila kurang di file lain, laporkan, jangan ubah.

## Selesai bila

- [ ] `pipelineClient.ts` mengekspor tipe dan dua fungsi dengan signature persis kontrak; satu-satunya pembungkus IPC pipeline.
- [ ] `PipelineList` menampilkan memuat, kosong "Tidak ada pipeline", dan galat dengan "Coba lagi" serta "Buka Pengaturan > Connector" untuk `cli_missing`/`not_logged_in`; respons usang dibuang lewat `seq`; props tidak berubah.
- [ ] `PipelineRow` menampilkan ref (glyph branch/tag), `StatusBadge`, sha 7 karakter, penulis bila ada, durasi hanya bila > 0 ("-" hanya untuk GitHub), waktu relatif, dan tombol Buka/Salin/Kirim ke agen memakai URL pipeline dengan `stopPropagation`.
- [ ] Klik atau Enter/Spasi pada baris memanggil `layout.openPane` dengan `kind: 'git-pipeline'`, `provider`, `projectId`, `pipelineId`, dan `title`.
- [ ] Hanya tiga `ownedPaths` yang berubah (`git status` memastikan).
- [ ] Guard: `cd frontend && npm run build` sukses (tanpa unit test).
