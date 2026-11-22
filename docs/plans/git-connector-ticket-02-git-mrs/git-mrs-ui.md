# Plan: UI daftar MR/PR di panel

## Metadata
- Tiket: `git-connector-ticket-02-git-mrs` (`docs/tickets/git-connector-ticket-02-git-mrs.md`)
- Fitur: `git-mrs-ui` (UI daftar MR/PR di panel)
- ownedPaths:
  - `frontend/src/modules/git/MrList.svelte`
  - `frontend/src/modules/git/MrRow.svelte`
  - `frontend/src/modules/git/StatusBadge.svelte`
  - `frontend/src/modules/git/mrClient.ts`
- Depends on foundation: tidak (epic ini tanpa foundation; semua yang dibutuhkan sudah ada dari tiket 1).
- Kontrak sumber: `overview.md` bagian 0, 2.1, 2.5, 2.6 (rujuk, jangan diubah).

## Tujuan
Mengganti placeholder `MrList.svelte` dengan daftar MR (GitLab) / PR (GitHub) sungguhan per project: pemilih status Terbuka | Digabung | Tertutup, baris lengkap (`MrRow`), badge status generik (`StatusBadge`), pembungkus IPC bertipe (`mrClient.ts`), aksi per baris, dan klik baris membuka pane `git-mr`. Frontend hanya bergantung pada kontrak IPC `git.mrs.list` (overview 2.1), tidak menunggu backend selesai untuk build.

## Path yang dimiliki
HANYA empat file ini yang boleh dibuat/diubah:
- `frontend/src/modules/git/MrList.svelte` (ganti isi placeholder)
- `frontend/src/modules/git/MrRow.svelte` (baru)
- `frontend/src/modules/git/StatusBadge.svelte` (baru)
- `frontend/src/modules/git/mrClient.ts` (baru)

Jangan sentuh file wiring atau path fitur lain: `store.ts`, `client.ts`, `actions.ts`, `GitPanel.svelte`, `ProjectSection.svelte`, `index.ts`, `PipelineList.svelte`, `MrPane.svelte`, `lib/stores/layout`, `layout/model.ts`, file Go, dan file global CSS. Membaca dan mengimpor dari file-file itu diperbolehkan. Bila kebutuhan tak terpenuhi tanpa mengubah file lain, laporkan di hasil; jangan ubah file itu.

## Kontrak
Dipakai (sudah ada, diverifikasi):
- `./client`: `Provider` (`'gitlab' | 'github'`), `Repo` (`projectId`, `projectName`, `projects`, `provider`, `host`, `path`, `webUrl`; TIDAK ada `workspaceId`), `parseGitError(x): {code, message}`.
- `./store`: `selectedState: Writable<Record<string, 'open'|'merged'|'closed'>>`, `repoKey(repo) => host + '/' + path`.
- `./actions`: `openInBrowser(url)`, `copyLink(url)` (async), `sendToAgent(url)` (async; toast "Tidak ada sesi aktif" sudah ditangani di dalamnya).
- `../../lib/ipc`: `call<T>(method, params)`.
- `../../lib/stores/workspaces`: `activeWorkspaceId` (store; nilai `string | null`), dibaca dengan `get()` dari `svelte/store`.
- `../../lib/stores/layout`: `layout.openPane(leaf)`; bentuk leaf: `{ kind: 'git-mr', provider, projectId, number, title }` (field `workspaceId` TIDAK diisi).
- `../settings`: `openSettings('connectors')` (pola `GitPanel.svelte`).
- IPC `git.mrs.list`: params `{workspaceId, projectId, state}`, hasil `{mrs: MergeRequest[] | null}` (overview 2.1). Galat IPC berpesan `"[kode] pesan"`, diurai `parseGitError`.

Disediakan (ekspor):
- `mrClient.ts`:
  ```ts
  export type MergeRequestState = 'open' | 'merged' | 'closed';
  export interface MergeRequest {
    number: number; title: string; author: string;
    sourceBranch: string; targetBranch: string;
    state: MergeRequestState; draft: boolean;
    reviewState?: string; pipelineStatus?: string;
    updatedAt: string; url: string;
  }
  export async function listMergeRequests(params: { workspaceId: string; projectId: string; state: MergeRequestState }): Promise<MergeRequest[]>;
  ```
- `StatusBadge.svelte`: props `{ status: string; kind?: 'pipeline' | 'review' | 'mr' }`. Generik: tidak mengimpor apa pun dari MR (dipakai ulang tiket 3 dan 4).
- `MrRow.svelte`: props `{ mr: MergeRequest; provider: Provider; onOpen: (mr: MergeRequest) => void }`.
- `MrList.svelte`: props tetap `{ provider: Provider; repo: Repo }` (Svelte 5 runes, `$props()`).

## Langkah
Gaya kode: Svelte 5 runes (`$props`, `$state`, `$derived`, `$effect`), `<script lang="ts">`, teks UI bahasa Indonesia, CSS dengan variabel yang dipakai tiket 1 (`--fg`, `--fg-muted`, `--bg-elev`, `--bg-hover`, `--border`, `--danger`, dengan fallback gelap). Warna status: `var(--success, #4caf7a)`, `var(--warn, #d9a441)`, `var(--danger, #e49a9a)`, `var(--accent, #6aa8e8)`; sebelum memakai, grep `frontend/src` (mis. `shell.css`/CSS global) untuk memastikan nama variabel tema yang ada; bila tak ada pakai fallback itu. Jangan menambah file global atau dependensi.

1. `mrClient.ts`
   - Impor `call` dari `'../../lib/ipc'` dan `parseGitError` dari `'./client'` (impor, jangan salin).
   - Deklarasikan dan ekspor `MergeRequestState` dan `MergeRequest` persis seperti di Kontrak.
   - `listMergeRequests`: `try { r = await call<{ mrs: MergeRequest[] | null }>('git.mrs.list', params); } catch (e) { throw parseGitError(e); }` lalu `return Array.isArray(r?.mrs) ? r.mrs : [];` (guard `r` null).

2. `StatusBadge.svelte`
   - Props `{ status, kind }: { status: string; kind?: 'pipeline' | 'review' | 'mr' }`.
   - Tabel label per kind (Indonesia): pipeline `pending` Menunggu, `running` Berjalan, `success` Berhasil, `failed` Gagal, `canceled` Dibatalkan, `skipped` Dilewati, `unknown` Tidak diketahui; review `approved` Disetujui, `changes_requested` Perlu perubahan, `review_required` Belum direview; mr `open` Terbuka, `merged` Digabung, `closed` Tertutup.
   - Bila `kind` tidak diberikan: cari `status` berurutan di tabel pipeline, lalu review, lalu mr (nilai tak bentrok antar tabel), sehingga kind efektif diturunkan lewat `$derived`. Status di luar tabel: tampilkan teks status apa adanya dengan nada netral.
   - Tiap entri punya glyph kecil (bukan hanya warna) dan nada warna: success/approved/merged -> hijau (glyph "✓"); failed/changes_requested -> merah ("✕"); running -> aksen ("●"); pending/review_required/open -> kuning/aksen ("◌"/"○"); canceled/skipped/closed/unknown -> netral ("⊘"/"–"/"?"). Glyph `aria-hidden="true"`; elemen akar `<span class="badge tone-…" title={label}>` memuat glyph + label (teks tetap terbaca screen reader). Gunakan `color-mix` atau latar semi-transparan dari variabel tema agar terbaca di tema terang dan gelap (jangan hardcode putih/hitam).
   - Ukuran kecil (font 12px, padding 0 6px, border-radius 8px, `white-space: nowrap`).

3. `MrRow.svelte`
   - Props `{ mr, provider, onOpen }`. Impor `openInBrowser`, `copyLink`, `sendToAgent` dari `'./actions'`, `StatusBadge` dari `'./StatusBadge.svelte'`, tipe `MergeRequest` dari `'./mrClient'`, `Provider` dari `'./client'`.
   - Turunan: `prefix = provider === 'gitlab' ? '!' : '#'`; `rel = relativeTime(mr.updatedAt)`.
   - Fungsi lokal `relativeTime(iso: string): string` (belum ada util bersama): parse `Date.parse`; NaN -> `''`; selisih detik `d = max(0, (now - t)/1000)`; `< 60` -> "baru saja"; menit `< 60` -> "N menit lalu"; jam `< 24` -> "N jam lalu"; hari `< 30` -> "N hari lalu"; bulan `< 12` -> "N bulan lalu"; selain itu "N tahun lalu". Dihitung saat render (tanpa polling/timer).
   - Elemen akar `<div class="row" role="button" tabindex="0" onclick={() => onOpen(mr)} onkeydown={...}>`. Handler keydown: bila `e.target !== e.currentTarget` abaikan (agar Enter/Spasi pada tombol aksi tidak ikut membuka); bila `e.key === 'Enter' || e.key === ' '` -> `e.preventDefault(); onOpen(mr)`. Suppress peringatan a11y Svelte bila perlu dengan komentar `<!-- svelte-ignore -->` yang tepat.
   - Isi baris (dua baris visual):
     - Baris 1: `<span class="num">{prefix}{mr.number}</span>`, judul `<span class="title" title={mr.title}>` (ellipsis: `overflow:hidden; text-overflow:ellipsis; white-space:nowrap; min-width:0; flex:1`), penanda `<span class="draft">Draft</span>` bila `mr.draft`.
     - Baris 2 (meta, `flex-wrap`): `mr.author`, `{mr.sourceBranch} → {mr.targetBranch}` (ellipsis/`title`), `<StatusBadge kind="review" status={mr.reviewState} />` HANYA `{#if mr.reviewState}`, `<StatusBadge kind="pipeline" status={mr.pipelineStatus} />` HANYA `{#if mr.pipelineStatus}`, waktu relatif (dengan `title` = `updatedAt` mentah) bila `rel` tidak kosong.
     - Tiga tombol ikon (`<button type="button" class="act">`) dengan `aria-label` dan `title`: "Buka di browser" (glyph "↗") -> `openInBrowser(mr.url)`, "Salin link" (glyph "⧉") -> `void copyLink(mr.url)`, "Kirim ke agen" (glyph "➤") -> `void sendToAgent(mr.url)`. Tiap handler onclick memulai dengan `event.stopPropagation()`. Tombol juga menghentikan propagasi `keydown` (`onkeydown={(e) => e.stopPropagation()}`). Tombol tampil terus (atau saat hover/focus-within; tetap dapat difokus keyboard).
   - Stile: hover `var(--bg-hover, #2c2c2c)`, focus-visible outline, padding 4px 10px, kursor pointer.

4. `MrList.svelte` (ganti seluruh isi)
   - Impor: `get` dari `svelte/store`; `activeWorkspaceId` dari `'../../lib/stores/workspaces'`; `layout` dari `'../../lib/stores/layout'`; `openSettings` dari `'../settings'`; `Provider, Repo, GitError, parseGitError` dari `'./client'`; `selectedState, repoKey` dari `'./store'`; `listMergeRequests, MergeRequest, MergeRequestState` dari `'./mrClient'`; `MrRow`.
   - State: `let { provider, repo }: { provider: Provider; repo: Repo } = $props();` `let sel = $state<Record<string, MergeRequestState>>({});` dengan `$effect(() => selectedState.subscribe((v) => { sel = v; }))` (pola `ProjectSection`); `let key = $derived(repoKey(repo)); let state = $derived<MergeRequestState>(sel[key] ?? 'open');` `let status = $state<'loading' | 'ready' | 'error'>('loading'); let mrs = $state<MergeRequest[]>([]); let err = $state<GitError | null>(null); let seq = 0;`
   - `function setState(s: MergeRequestState)`: `selectedState.update((m) => ({ ...m, [key]: s }))` (tidak memuat langsung; efek di bawah yang memuat karena `state` berubah).
   - `async function load(repoArg: Repo, st: MergeRequestState)`: `const my = ++seq; const ws = get(activeWorkspaceId); if (ws === null) { mrs = []; err = null; status = 'ready'; return; }` (keadaan kosong tanpa IPC); `status = 'loading'; err = null;` lalu `try { const r = await listMergeRequests({ workspaceId: ws, projectId: repoArg.projectId, state: st }); if (my !== seq) return; mrs = r; status = 'ready'; } catch (x) { if (my !== seq) return; err = parseGitError(x); mrs = []; status = 'error'; }`. Respons usang diabaikan lewat penghitung urutan `seq`.
   - Efek pemuatan: `$effect(() => { const r = repo; const s = state; void load(r, s); return () => { seq++; }; });` Efek ini bergantung pada `repo` (identitas objek berubah saat Segarkan membuat objek `Repo` baru), `state`, dan tidak memuat apa pun selain `repo`/`state` (baca `activeWorkspaceId` lewat `get()` agar bukan dependensi reaktif; `load` dipanggil dengan `untrack` bila perlu agar tulisan state di dalamnya tidak menjadi dependensi). Fungsi cleanup menaikkan `seq` sehingga respons setelah unmount/perubahan diabaikan dan tidak ada pemuatan setelah unmount. Efek berjalan saat mount (pemuatan awal), saat status berganti, dan saat identitas `repo` berubah (Segarkan).
   - `function retry() { void load(repo, state); }`.
   - `function openMr(mr: MergeRequest)`: `layout.openPane({ kind: 'git-mr', provider, projectId: repo.projectId, number: mr.number, title: (provider === 'gitlab' ? '!' : '#') + mr.number + ' ' + mr.title });`.
   - Label dan teks: `const stateLabel = { open: 'Terbuka', merged: 'Digabung', closed: 'Tertutup' }`; teks kosong: `` `Tidak ada ${provider === 'gitlab' ? 'MR' : 'PR'} ${stateLabel[state].toLowerCase()}` `` (menghasilkan "Tidak ada MR terbuka" / "Tidak ada PR digabung" / "... tertutup").
   - Judul galat: sama dengan `GitPanel`: `cli_missing` -> `message || '<glab|gh> belum terpasang'`, `not_logged_in` -> "Belum login", `permission` -> "Izin kurang", `timeout` -> "Waktu habis", `not_found` -> `message`, default -> `message || 'Terjadi galat'`. `needSettings` bila kode `cli_missing` atau `not_logged_in`.
   - Markup: segmented control `<div class="seg" role="tablist" aria-label="Status {provider === 'gitlab' ? 'MR' : 'PR'}">` berisi tiga `<button type="button" role="tab" aria-selected={state === s} aria-pressed={state === s} class:on={state === s} onclick={() => setState(s)}>`; (kombinasi `role="tab"` dengan `aria-pressed` sesuai spec; pastikan svelte-check tidak mengeluh, bila mengeluh gunakan `role="tablist"` pada pembungkus dan tombol biasa dengan `aria-pressed`). Dapat difokus keyboard (tombol native). Lalu:
     - `{#if status === 'loading'}` -> `<p class="muted">Memuat...</p>`
     - `{:else if status === 'error' && err}` -> kotak galat `role="alert"` (gaya `.errbox` dari `GitPanel`), judul galat, tombol "Coba lagi" (`retry`), dan "Buka Pengaturan > Connector" (`openSettings('connectors')`) bila `needSettings`. Galat hanya di bagian ini (komponen per bagian, tidak memengaruhi bagian lain).
     - `{:else if mrs.length === 0}` -> `<p class="muted">{emptyText}</p>`
     - `{:else}` -> `{#each mrs as mr (mr.number)} <MrRow {mr} {provider} onOpen={openMr} />{/each}`.
   - Stile: segmented control kecil (border `var(--border, #333)`, tombol aktif `var(--bg-hover, #2c2c2c)` + `var(--fg, #ddd)`), `.muted` seperti placeholder lama, `.errbox` seperti `GitPanel`.

5. Guard (dijalankan oleh pelaksana sebelum selesai):
   - `cd frontend && npm run build`
   - `cd frontend && npm run check` (skrip ada: `svelte-check`). Perbaiki galat/peringatan yang berasal dari empat file milik fitur ini; abaikan temuan di file lain (jangan diubah, laporkan bila relevan).

## Perubahan
- Diubah: `frontend/src/modules/git/MrList.svelte` (placeholder diganti implementasi penuh; props tetap `{provider, repo}`).
- Dibuat: `frontend/src/modules/git/MrRow.svelte`
- Dibuat: `frontend/src/modules/git/StatusBadge.svelte`
- Dibuat: `frontend/src/modules/git/mrClient.ts`
- Tidak ada perubahan di file lain.

## Wiring
Fitur ini mengekspos, untuk disambungkan/diverifikasi oleh langkah `git-mrs-wiring` (JANGAN disambungkan sendiri; tidak ada perubahan di file tiket 1):
- `MrList.svelte` dengan props `{provider, repo}` yang sudah dirender `ProjectSection.svelte` di dalam `{#if open}` (bagian terlipat tidak memuat); tidak perlu diubah.
- `mrClient.listMergeRequests` memanggil IPC `git.mrs.list` yang didaftarkan backend (`wire_git_mrs.go`, fitur `git-mrs-data`). Tanpa backend, UI menampilkan keadaan galat dengan Coba lagi (bukan crash).
- Klik/Enter/Spasi baris memanggil `layout.openPane({kind:'git-mr', provider, projectId, number, title})` ke pane `git-mr` yang sudah terdaftar di `modules/git/index.ts`.
- `StatusBadge.svelte` generik, tersedia untuk dipakai ulang tiket 3 (pipeline) dan 4 (review/MR).
- Pilihan status tersimpan di `selectedState` (kunci `repoKey(repo)`), bertahan selama app hidup.

## Selesai bila
- [ ] Empat file ada sesuai Kontrak (nama ekspor, props, signature persis).
- [ ] `MrList` default status Terbuka; berganti status memuat ulang; pilihan dibaca/ditulis lewat `selectedState` dengan kunci `repoKey(repo)` dan bertahan setelah unmount/mount ulang.
- [ ] Pemuatan terjadi saat mount, saat status berganti, dan saat identitas objek `repo` berubah (Segarkan); respons usang diabaikan lewat penghitung `seq`; tidak ada pemuatan setelah unmount; `activeWorkspaceId === null` -> keadaan kosong tanpa IPC.
- [ ] Keadaan memuat, kosong ("Tidak ada MR/PR terbuka|digabung|tertutup" menurut provider dan status), dan galat terbaca dengan "Coba lagi" (+ "Buka Pengaturan > Connector" untuk `cli_missing`/`not_logged_in`).
- [ ] Baris: judul ellipsis dengan `title` penuh, nomor `!`/`#`, penulis, `sourceBranch → targetBranch`, "Draft", badge review/pipeline HANYA bila datanya ada, waktu relatif bahasa Indonesia.
- [ ] Tiga tombol aksi (Buka di browser, Salin link, Kirim ke agen) dengan `aria-label`, memakai `actions.ts`, `stopPropagation` sehingga tidak membuka pane; baris `role="button"` `tabindex="0"` dengan Enter/Spasi memanggil `onOpen`.
- [ ] Klik baris membuka pane `git-mr` dengan `provider/projectId/number` dan judul `!n`/`#n` + judul.
- [ ] `StatusBadge` generik (label Indonesia sesuai tabel, glyph + teks bukan hanya warna, terbaca di tema terang/gelap), tanpa impor dari MR.
- [ ] Guard: `cd frontend && npm run build` sukses (dan `npm run check` tanpa galat baru dari empat file ini). Tanpa unit test, tanpa dependensi baru, tidak ada file di luar ownedPaths yang berubah (`git status` hanya menampilkan empat path ini).
