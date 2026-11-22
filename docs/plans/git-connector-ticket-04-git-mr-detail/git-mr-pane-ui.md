# Plan: Pane git-mr

## Metadata

- Tiket: `git-connector-ticket-04-git-mr-detail` (Detail MR/PR)
- Fitur: `git-mr-pane-ui` (Pane git-mr)
- ownedPaths:
  - `frontend/src/modules/git/MrPane.svelte`
  - `frontend/src/modules/git/ReviewerList.svelte`
  - `frontend/src/modules/git/mrDetailClient.ts`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; semua yang dibutuhkan sudah ada dari tiket 1 sampai 3)
- Rujukan kontrak: `docs/plans/git-connector-ticket-04-git-mr-detail/overview.md` (bagian 2.1 dan 2.5). Jangan mengubah overview.

## Tujuan

Mengganti placeholder `MrPane.svelte` dengan pane detail MR/PR nyata, memakai klien IPC bertipe baru `mrDetailClient.ts` dan komponen `ReviewerList.svelte`. Pane dikembangkan terhadap kontrak IPC `git.mr.get` (overview 2.1) dan harus lulus build frontend walau backend fitur data belum selesai (hanya mengimpor tipe dari `mrClient.ts`, tidak ada ketergantungan kompilasi ke Go). Read-only, tanpa polling, tanpa unit test, tanpa dependensi baru.

## Path yang dimiliki

HANYA tiga file ini yang boleh dibuat/diubah:
- `frontend/src/modules/git/MrPane.svelte` (ganti isi; props tetap)
- `frontend/src/modules/git/ReviewerList.svelte` (baru)
- `frontend/src/modules/git/mrDetailClient.ts` (baru)

Jangan sentuh file lain, termasuk `index.ts`, `mrClient.ts`, `client.ts`, `actions.ts`, `StatusBadge.svelte`, `PipelinePane.svelte`, `MrList.svelte`, `MrRow.svelte`, `layout/model.ts`, file Go, dan file fitur paralel lain. Membaca dan memanggil/mengimpor dari file itu diperbolehkan.

## Kontrak

Dipakai (sudah ada, jangan diubah):
- `./client`: `parseGitError(x): GitError`, tipe `GitError {code, message}`.
- `./mrClient`: `import type { MergeRequest }` (field: `number, title, author, sourceBranch, targetBranch, state, draft, reviewState?, pipelineStatus?, updatedAt, url`).
- `./actions`: `openInBrowser(url)`, `copyLink(url)` (async), `sendToAgent(url)` (async; toast "Tidak ada sesi aktif" sudah di dalamnya).
- `./StatusBadge.svelte`: props `{status: string; kind?: 'pipeline'|'review'|'mr'}`.
- `../../lib/ipc`: `call<T>(method, params)`.
- `../../lib/stores/layout`: `layout.openPane({...})`.
- `../../lib/stores/workspaces`: `activeWorkspaceId` (dibaca lewat `get()` dari `svelte/store`).
- `../settings`: `openSettings('connectors')`.
- `../layout/model`: tipe `Leaf` (field `provider`, `projectId`, `number`, `pipelineId`, `title`).
- IPC `git.mr.get` params `{workspaceId, projectId, number}`, hasil objek `MergeRequestDetail` langsung (overview 2.1).

Disediakan (ekspor):
- `mrDetailClient.ts`: tipe `MergeableState = 'mergeable'|'conflict'|'unknown'`, `ReviewerState = 'approved'|'changes_requested'|'pending'`, `MrReviewer {name: string; state: string}`, `MrLastPipeline {id: string; status: string; url?: string}`, `MergeRequestDetail extends MergeRequest {description: string; labels: string[]; reviewers: MrReviewer[]; mergeable?: MergeableState; lastPipeline?: MrLastPipeline}`, dan `getMrDetail(workspaceId: string, projectId: string, number: number): Promise<MergeRequestDetail>`.
- `ReviewerList.svelte`: props `{ reviewers: MrReviewer[] }`.
- `MrPane.svelte`: props `{ pane: Leaf; visible: boolean }` (tidak berubah).

## Langkah

1. Baca `PipelinePane.svelte`, `MrList.svelte` (fungsi `openMr`) dan `MrRow.svelte` (fungsi `relativeTime`) sebagai pola gaya; jangan diubah.

2. Buat `frontend/src/modules/git/mrDetailClient.ts`:
   - `import { call } from '../../lib/ipc'; import { parseGitError } from './client'; import type { MergeRequest } from './mrClient';`
   - Ekspor tipe sesuai bagian Kontrak.
   - `export async function getMrDetail(workspaceId, projectId, number)`: `let r; try { r = await call<MergeRequestDetail>('git.mr.get', { workspaceId, projectId, number }); } catch (e) { throw parseGitError(e); }`. Bila `r` null/bukan objek, `throw parseGitError(new Error('MR tidak ditemukan'))` (atau lempar `{code: 'not_found', message: 'MR tidak ditemukan'}` bila `GitError` plain object lebih sesuai dengan `parseGitError`; cek `client.ts`). Kembalikan `{ ...r, description: typeof r.description === 'string' ? r.description : '', labels: Array.isArray(r.labels) ? r.labels : [], reviewers: Array.isArray(r.reviewers) ? r.reviewers : [] }`.

3. Buat `frontend/src/modules/git/ReviewerList.svelte` (Svelte 5 runes):
   - `import type { MrReviewer } from './mrDetailClient'; let { reviewers }: { reviewers: MrReviewer[] } = $props();`
   - Fungsi lokal `info(state)`: `approved` -> `{label: 'Disetujui', tone: 'good'}`, `changes_requested` -> `{label: 'Diminta perubahan', tone: 'bad'}`, selain itu (termasuk `pending`/kosong/tak dikenal) -> `{label: 'Belum', tone: 'warn'}`.
   - Bila `reviewers.length === 0`: `<p class="muted">Belum ada reviewer</p>`. Selain itu `<ul>` dengan `<li>`: nama (`overflow-wrap: anywhere`) dan label status berwarna. Warna lewat `var(--success, #6fbf73)`, `var(--danger, #e49a9a)`, `var(--warn, #d9b35f)`; teks muted `var(--fg-muted, #888)`. Sertakan glyph kecil (mis. `✓`, `✕`, `◌`) dengan `aria-hidden`. Bukan `StatusBadge kind="review"`. Tanpa `{@html}`. Key `{#each reviewers as r, i (r.name + i)}`.

4. Tulis ulang `frontend/src/modules/git/MrPane.svelte`, mengikuti `PipelinePane.svelte`:
   - Impor: `get` (`svelte/store`), `untrack` (`svelte`), `activeWorkspaceId`, `openSettings` dari `../settings`, `Leaf`, `parseGitError`/`GitError`, `getMrDetail`/`MergeRequestDetail` dari `./mrDetailClient`, `openInBrowser, copyLink, sendToAgent` dari `./actions`, `layout` dari `../../lib/stores/layout`, `StatusBadge`, `ReviewerList`.
   - Props: `let { pane, visible }: { pane: Leaf; visible: boolean } = $props();` Wrapper luar tetap `<div class="git-pane" hidden={!visible}>`.
   - State: `type Phase = 'loading'|'ready'|'error'|'missing'`; `phase`, `detail: MergeRequestDetail | null`, `err: GitError | null`, `let seq = 0` (non-reaktif).
   - `async function load(provider?: string, projectId?: string, number?: number)`: `const my = ++seq; const ws = get(activeWorkspaceId);` bila `!provider || !projectId || !number || ws === null` -> `detail = null; err = null; phase = 'missing'; return`. Selain itu `phase = 'loading'; err = null;` `try { const r = await getMrDetail(ws, projectId, number); if (my !== seq) return; detail = r; phase = 'ready'; } catch (x) { if (my !== seq) return; err = parseGitError(x); detail = null; phase = 'error'; }`.
   - `$effect(() => { const p = pane.provider, pr = pane.projectId, n = pane.number; untrack(() => { void load(p, pr, n); }); return () => { seq++; }; });` Ini memuat saat mount/pemulihan layout dan saat pane berganti. Tanpa `setInterval`/polling.
   - `function refresh() { void load(pane.provider, pane.projectId, pane.number); }`
   - Turunan: `prefix = pane.provider === 'gitlab' ? '!' : '#'`; `title = detail?.title || pane.title || 'Merge Request'`; `numLabel = pane.number ? prefix + pane.number : ''`; `url = detail?.url ?? ''`; `canAct = phase === 'ready' && url !== ''`; `errTitle` dan `needSettings` sama seperti `PipelinePane` (`cli_missing`, `not_logged_in` memicu tombol "Buka Pengaturan > Connector" via `openSettings('connectors')`; selalu ada tombol "Coba lagi" yang memanggil `refresh`).
   - `relativeTime(iso)`: fungsi lokal Indonesia ("baru saja" bila < 1 menit, "N menit lalu", "N jam lalu", "N hari lalu", "N bulan lalu", "N tahun lalu"); `iso` kosong/tidak valid -> `''`.
   - Header (`.head`): baris judul `<span class="num">{numLabel}</span> <span class="title">{title}</span>` dan tombol aksi dengan aria-label/title: "Buka di browser" (`↗`, `openInBrowser(url)`), "Salin link" (`⧉`, `void copyLink(url)`), "Kirim ke agen" (`➤`, `void sendToAgent(url)`), semuanya `disabled={!canAct}`; tombol teks "Segarkan" `disabled={phase === 'loading' || phase === 'missing'}`.
   - Body saat `phase === 'ready' && detail`, urutan:
     1. Baris meta: `<StatusBadge status={detail.state} kind="mr" />`, penanda `{#if detail.draft}<span class="draft">Draf</span>{/if}`, penulis `{#if detail.author}@{detail.author}{/if}`, `{detail.sourceBranch} -> {detail.targetBranch}` (monospace, ellipsis), waktu relatif dari `detail.updatedAt` (title berisi nilai ISO).
     2. Label: `{#if detail.labels.length > 0}` chip `<span class="chip">` per label (disembunyikan bila kosong).
     3. Bagian "Reviewer" dengan `<ReviewerList reviewers={detail.reviewers} />`.
     4. Status merge: `{#if detail.mergeable === 'mergeable'}` "Bisa digabung" (nada baik) `{:else if detail.mergeable === 'conflict'}` "Konflik" (nada `--danger`); `unknown`/hilang tidak dirender.
     5. Pipeline terakhir `{#if detail.lastPipeline}`: tombol `<button type="button" class="pipe" onclick={openPipeline}>` berisi `<StatusBadge status={detail.lastPipeline.status} kind="pipeline" />` dan `#{detail.lastPipeline.id}`. `openPipeline()` memanggil `layout.openPane({ kind: 'git-pipeline', provider: pane.provider, projectId: pane.projectId, pipelineId: detail.lastPipeline.id, title: 'Pipeline ' + detail.lastPipeline.id })` (ketik `provider`/`projectId` sebagai string; sudah dijamin non-kosong pada fase ready; field leaf bernama `pipelineId`, bukan `id`).
     6. Deskripsi: bagian "Deskripsi"; `{#if detail.description.trim()}<div class="desc">{detail.description}</div>{:else}<p class="muted">Tidak ada deskripsi</p>{/if}`. CSS `.desc { white-space: pre-wrap; overflow-wrap: anywhere; }`. DILARANG `{@html}`; interpolasi teks Svelte sudah meng-escape HTML.
   - Fase lain: `missing` -> `<p class="muted">Project atau MR tidak ditemukan</p>` (aksi nonaktif); `error` -> `.errbox role="alert"` seperti `PipelinePane`; selain itu `<p class="muted">Memuat...</p>`.
   - Body dibungkus `.body { flex: 1; min-height: 0; overflow: auto; padding: 8px 12px; display: flex; flex-direction: column; gap: 12px; }`.
   - Gaya: variabel CSS `--fg`, `--fg-muted`, `--bg-elev`, `--bg-hover`, `--border`, `--danger`, `--accent` dengan fallback (salin gaya `.head`, `.act`, `.refresh`, `.errbox`, `.errs`, `.muted` dari `PipelinePane`); `.chip` (border `--border`, radius 10px, padding 0 8px, font 12px); `.draft` (muted, border, font 12px); nada baik `var(--success, #6fbf73)`; `.pipe` tombol datar dengan hover `--bg-hover` dan `:focus-visible` outline `--accent`. Tanpa `@html`, tanpa dependensi baru.

5. Jalankan guard: `cd frontend && npm run build`. Bila ada skrip `check`, boleh jalankan `npm run check` dan perbaiki galat tipe/a11y hanya pada tiga file milik fitur ini. Jangan mengubah file lain untuk meloloskan build; bila terhalang, laporkan.

## Perubahan

- Diubah: `frontend/src/modules/git/MrPane.svelte` (placeholder diganti pane detail penuh; props dan wrapper tetap).
- Dibuat: `frontend/src/modules/git/ReviewerList.svelte`.
- Dibuat: `frontend/src/modules/git/mrDetailClient.ts`.

## Wiring

Tidak ada yang disambungkan oleh fitur ini. Yang diekspos untuk langkah wiring (`git-mr-detail-wiring`) untuk diverifikasi, bukan diubah di sini:
- `MrPane.svelte` tetap terdaftar sebagai kind `git-mr` di `index.ts` dengan props yang sama; `MrList.openMr` sudah membuka `git-mr` dengan `provider`, `projectId`, `number`, `title`.
- `MrPane` membuka `git-pipeline` lewat `layout.openPane({kind:'git-pipeline', provider, projectId, pipelineId, title})`; `PipelinePane` membaca `pane.pipelineId`.
- `MrPane` memanggil IPC `git.mr.get` hanya lewat `getMrDetail`; pengujian dengan data nyata menunggu fitur `git-mr-detail-data` dan wiring.
- Selisih untuk dilaporkan: leaf memakai `pipelineId` (tiket menulis `id`).

## Selesai bila

- [ ] `mrDetailClient.ts` mengekspor `getMrDetail`, `MergeRequestDetail`, `MrReviewer`, `MrLastPipeline`, `MergeableState`, `ReviewerState`; galat diterjemahkan lewat `parseGitError`; `labels`/`reviewers`/`description` terjamin bertipe aman.
- [ ] `ReviewerList` menampilkan Disetujui / Diminta perubahan / Belum dan "Belum ada reviewer" bila kosong.
- [ ] `MrPane` menampilkan berurutan: header (judul, nomor `!`/`#`, Buka, Salin, Kirim ke agen, Segarkan), `StatusBadge kind="mr"` + penanda Draf, penulis, branch sumber -> target, waktu relatif, label chip (tersembunyi bila kosong), reviewer, Bisa digabung/Konflik (tersembunyi bila unknown/hilang), pipeline terakhir (klik membuka `git-pipeline` dengan `pipelineId`), deskripsi teks biasa (`pre-wrap`, tanpa `{@html}`, "Tidak ada deskripsi" bila kosong).
- [ ] Fase `loading|ready|error|missing` dengan guard `seq`; "Coba lagi" dan ajakan Pengaturan > Connector untuk `cli_missing`/`not_logged_in`; tanpa polling; memuat ulang saat mount/pemulihan layout dan saat Segarkan.
- [ ] Hanya tiga path milik fitur ini yang berubah (`git status` memperlihatkan hanya ketiganya di bawah `frontend/`).
- [ ] Guard: `cd frontend && npm run build` sukses.
