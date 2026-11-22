# Plan: Rail Git, panel per provider, titik ekstensi pane

## Metadata
- Tiket: `git-connector-ticket-01-git-repos` (`docs/tickets/git-connector-ticket-01-git-repos.md`)
- Fitur: `git-rail-panel-ui`
- Overview (kontrak bersama, jangan diulang di sini): `docs/plans/git-connector-ticket-01-git-repos/overview.md`, terutama bagian 2.1, 2.6, 6, 7.
- ownedPaths: lihat bagian "Path yang dimiliki".
- Depends on foundation: tidak. Fitur ini hanya bergantung pada bentuk JSON `Repo` dan nama IPC `git.repos.list` (teks di overview 2.6); tidak bergantung pada fitur Go mana pun. Backend belum tentu ada saat build; kode frontend harus tetap build tanpa itu.

## Tujuan
Membangun frontend `frontend/src/modules/git/`: klien IPC `git.repos.list`, store deteksi repo per workspace aktif (dengan penjaga urutan), aksi bersama (buka di browser, salin link, kirim ke agen), ikon, `GitPanel` (satu komponen untuk dua provider), `ProjectSection` (bagian per repo yang bisa dilipat), placeholder `MrList`/`PipelineList`/`MrPane`/`PipelinePane`, serta registrasi dua item rail (`git-gitlab` order 20, `git-github` order 21) dan dua pane kind (`git-mr`, `git-pipeline`). Memperluas `modules/layout/model.ts` dengan `PaneKind` baru dan field leaf `number`/`pipelineId`. Ini adalah titik ekstensi untuk tiket 02 sampai 04; props placeholder TIDAK boleh berubah kelak.

## Path yang dimiliki
HANYA ini yang boleh disentuh; jangan sentuh file wiring (`frontend/src/main.ts`, `modules/shell/*`, `modules/layout/registry.ts`, dll.) atau path fitur lain (Go, `docs/git/`, `internal/*`, `cmd/*`):
- `frontend/src/modules/git/index.ts`
- `frontend/src/modules/git/store.ts`
- `frontend/src/modules/git/client.ts`
- `frontend/src/modules/git/actions.ts`
- `frontend/src/modules/git/GitIcon.svelte`
- `frontend/src/modules/git/GitPanel.svelte`
- `frontend/src/modules/git/ProjectSection.svelte`
- `frontend/src/modules/git/MrList.svelte`
- `frontend/src/modules/git/PipelineList.svelte`
- `frontend/src/modules/git/MrPane.svelte`
- `frontend/src/modules/git/PipelinePane.svelte`
- `frontend/src/modules/layout/model.ts`

Jangan commit. Jangan menambah unit test. Jangan membuat file lain (folder `modules/git/` belum ada; buat lewat penulisan berkas).

## Kontrak
Disediakan (bentuk persis di overview 2.6; ringkas di sini sebagai checklist nama):
- `client.ts`: `type Provider = 'gitlab' | 'github'`, `interface RepoProject { id; name }`, `interface Repo { projectId; projectName; projects: RepoProject[]; provider; host; path; webUrl }`, `listRepos(workspaceId: string): Promise<Repo[]>` (IPC `git.repos.list` `{workspaceId}`), `parseGitError(x: unknown): { code: string; message: string }`.
- `store.ts`: `gitRepos` (writable `Repo[]`), `gitState` (writable `{status:'loading'|'ready'|'error'; code?; message?}`), `refreshRepos(): Promise<void>`, `repoKey(r: Repo): string` (`host + '/' + path`), `reposFor(provider: Provider)` (Readable `Repo[]`), `collapsed` (writable `Record<string, boolean>`), `selectedState` (writable `Record<string, 'open'|'merged'|'closed'>`, default dibaca `?? 'open'`), plus helper `isOpen`/`toggleSection` (lihat langkah).
- `actions.ts`: `openInBrowser(url)`, `copyLink(url)`, `sendToAgent(url)`, semuanya `(url: string) => void | Promise<void>`.
- Komponen: `GitIcon` (prop `provider?`), `GitPanel` (prop `provider`), `ProjectSection` (props `{provider, repo}`), `MrList` dan `PipelineList` (props `{provider, repo}` TETAP), `MrPane` dan `PipelinePane` (props `{ pane: Leaf; visible: boolean }` sesuai `PaneKindSpec`).
- `model.ts`: `PaneKind` memuat `'git-mr' | 'git-pipeline'`; `Leaf` dan `NewLeaf` memuat `number?: number` dan `pipelineId?: string`.

Dipakai (sudah ada, diverifikasi di kode):
- `call` dan `onEvent` dari `frontend/src/lib/ipc` (dipakai `modules/lark/client.ts` dan `store.ts`).
- `activeWorkspaceId` (`Writable<string|null>`) dari `frontend/src/lib/stores/workspaces`.
- `registerRailItem` dari `modules/shell/registry` (spec: `{id,title,icon,component,order?,visible?: Readable<boolean>}`); `registerPaneKind` dari `modules/layout/registry` (spec: `{kind, component: Component<{pane: Leaf; visible: boolean}>, titleFor?}`).
- `layout` store (`layout`, `layout.leaves`) dari `lib/stores/layout`, `pasteToSession` dari `modules/terminal`, `showToast` dari `modules/openfolder/toast-store`, `openSettings` dari `modules/settings` (`openSettings('connectors')`).
- Event `workspace.changed` payload: `{kind, workspaceId, workspace}`; nilai `kind` dari backend: `created`, `renamed`, `deleted`, `projectAdded`, `projectRemoved`, `configChanged` (`workspaceId` kosong untuk `configChanged` global).
- Acuan pola: `modules/lark/{client,store,actions,index,LarkIcon,LarkPanel}`, `modules/lark/doc/DocPane.svelte` (fungsi `openExternal` memakai `window.runtime.BrowserOpenURL`).

## Langkah

### 1. `modules/layout/model.ts` (perubahan minimal, aditif)
Baca file dulu. Lalu ubah HANYA yang berikut:
1. `PaneKind`: `'terminal' | 'file' | 'thread' | 'lark-board' | 'lark-doc' | 'git-mr' | 'git-pipeline'`.
2. Interface `Leaf`: tambah `number?: number; // nomor MR/PR` dan `pipelineId?: string;` setelah `nodeToken?`.
3. Interface `NewLeaf`: tambah dua field yang sama.
4. `makeLeaf`: tambah `if (nl.number !== undefined) leaf.number = nl.number;` dan `if (nl.pipelineId !== undefined) leaf.pipelineId = nl.pipelineId;` setelah baris `nodeToken`.
5. `hydrateNode`: tambah `'pipelineId'` ke daftar `for (const k of [...] as const)` (string), dan sesudah blok `readOnly`/`threadAgent` tambah `if (typeof r.number === 'number' && Number.isFinite(r.number)) leaf.number = r.number;`.
6. Jangan menyentuh fungsi lain. Periksa apakah ada fungsi lain di file yang memetakan field leaf eksplisit (mis. `grep -n "nodeToken" model.ts`); setiap tempat yang menyebut `nodeToken` harus juga menyebut dua field baru. Bila ada tempat serialisasi di luar `model.ts` (mis. `lib/stores/persist.ts`) yang memetakan field eksplisit, JANGAN diubah (bukan milik fitur ini); catat di laporan akhir.

### 2. `modules/git/client.ts`
```ts
import { call } from '../../lib/ipc';
export type Provider = 'gitlab' | 'github';
export interface RepoProject { id: string; name: string }
export interface Repo { projectId: string; projectName: string; projects: RepoProject[]; provider: Provider; host: string; path: string; webUrl: string }
export type GitError = { code: string; message: string };
export function parseGitError(x: unknown): GitError   // salin logika parseLarkError (objek {code,message}; regex /^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$/s; fallback code 'unknown')
export async function listRepos(workspaceId: string): Promise<Repo[]>
```
`listRepos`: `call<Repo[] | null>('git.repos.list', { workspaceId })`, tangkap galat dan lempar ulang `parseGitError(e)`; kembalikan `(r ?? []).map(...)` dengan `projects` dinormalkan menjadi `[{id: projectId, name: projectName}]` bila `projects` tidak array/kosong (tahan data tak terduga). Tipe `MergeRequest`, `Pipeline` dll. TIDAK ditambah di sini (tiket lain, file klien sendiri).

### 3. `modules/git/store.ts` (ikuti `modules/lark/store.ts`)
- Impor `get, writable, derived, type Readable` dari `svelte/store`; `onEvent` dari `../../lib/ipc`; `activeWorkspaceId`; `listRepos, parseGitError, type Repo, type Provider` dari `./client`.
- `export type GitState = { status: 'loading' | 'ready' | 'error'; code?: string; message?: string }`.
- `export const gitRepos = writable<Repo[]>([])`, `export const gitState = writable<GitState>({ status: 'loading' })`.
- `export const repoKey = (r: Repo): string => r.host + '/' + r.path`.
- `export function reposFor(provider: Provider): Readable<Repo[]>` = `derived(gitRepos, ($r) => $r.filter((r) => r.provider === provider))`. Buat instance derived sekali per provider di modul (dua konstanta privat, `reposFor` mengembalikan yang sesuai) agar tidak membuat store baru tiap pemanggilan.
- `export const collapsed = writable<Record<string, boolean>>({})` dan `export const selectedState = writable<Record<string, 'open'|'merged'|'closed'>>({})`. Peta di memori saja (bertahan selama app hidup, bukan persist disk).
- `export function isSectionOpen(map: Record<string, boolean>, key: string, index: number): boolean`: bila `key in map` -> `!map[key]` (nilai `true` = terlipat); bila belum ada -> `index === 0` (bagian pertama terbuka default). `export function toggleSection(key: string, currentlyOpen: boolean): void` menulis `collapsed[key] = currentlyOpen` (membalik: bila sekarang terbuka maka diset terlipat `true`). Dengan begitu state eksplisit terpilih bertahan setelah panel di-unmount.
- `refreshRepos()` dengan `let seq = 0`: baca `wsId = get(activeWorkspaceId)`; bila `null`: `seq++`, `gitRepos.set([])`, `gitState.set({status:'ready'})`, return. Selain itu `const my = ++seq`; `try { const rs = await listRepos(wsId); if (my !== seq || get(activeWorkspaceId) !== wsId) return; gitRepos.set(rs); gitState.set({status:'ready'}); } catch (x) { if (my !== seq || get(activeWorkspaceId) !== wsId) return; const e = parseGitError(x); gitRepos.set([]); gitState.set({status:'error', code: e.code, message: e.message}); }`. Galat tidak boleh dilempar keluar (tidak mengganggu app); `visible` rail otomatis false karena repo dikosongkan.
- Pemicu:
  - Inisialisasi modul + pergantian workspace: `let lastWs: string | null | undefined = undefined; activeWorkspaceId.subscribe((id) => { if (id === lastWs) return; lastWs = id; gitRepos.set([]); gitState.set({status:'loading'}); void refreshRepos(); });` (subscribe langsung memicu deteksi pertama saat app mulai). Saat `id === null`, `refreshRepos` langsung menetapkan `ready` kosong.
  - `onEvent('workspace.changed', (_t, payload) => {...})`: tolak payload non-objek; ambil `kind` dan `workspaceId`. Proses hanya bila `kind` salah satu `projectAdded`, `projectRemoved`, `deleted`, `configChanged`. Untuk `projectAdded`/`projectRemoved`/`deleted`/`configChanged`: bila `workspaceId` string tak kosong dan berbeda dari `get(activeWorkspaceId)` -> abaikan (workspace lain). `workspaceId` kosong (configChanged global) -> tetap refresh. Untuk `deleted` pada workspace aktif: `activeWorkspaceId` akan berganti sendiri lewat stores; cukup `void refreshRepos()` (penjaga seq menangani balapan). Jangan mengosongkan repo pada refresh karena event (hindari flicker); hanya ganti hasil saat selesai.
  - Segarkan manual: `GitPanel` memanggil `refreshRepos()` (lihat langkah 7), dengan `gitState.set({status:'loading'})` lebih dulu bila belum ada repo.
- Tidak ada polling.

### 4. `modules/git/actions.ts` (ikuti `modules/lark/actions.ts`)
- `copyLink(url)` dan `sendToAgent(url)`: salin persis implementasi lark (clipboard dengan fallback `execCommand`, toast `'Link disalin'` / `'Gagal menyalin link'` error; `sendToAgent` cari leaf terminal fokus lalu terminal pertama dengan `sessionId`, `pasteToSession(sessionId, url + ' ')` tanpa Enter, bila gagal `showToast('Tidak ada sesi aktif', 'error')`). Impor `layout` dari `../../lib/stores/layout`, `showToast` dari `../openfolder/toast-store`, `pasteToSession` dari `../terminal`. Jangan mengimpor dari `modules/lark` (hindari kopling lintas modul).
- `openInBrowser(url: string): void`: hanya terima `/^https?:\/\//i`; kalau tidak, `showToast('Link tidak valid', 'error')` dan return. Pakai `(window as any).runtime?.BrowserOpenURL` bila fungsi; jika tidak, `window.open(url, '_blank', 'noopener,noreferrer')`.

### 5. `modules/git/GitIcon.svelte`
SVG inline 28x28 viewBox 24 (gaya `LarkIcon.svelte`: `fill="none" stroke="currentColor" stroke-width="1.8"`, `aria-hidden="true"`). Prop `provider?: 'gitlab' | 'github'` (Svelte 5: `let { provider }: { provider?: Provider } = $props();`). Bentuk: `github` = ikon cabang/merge sederhana (tiga lingkaran terhubung garis); `gitlab` = ikon cabang yang sama ditambah penanda berbeda (mis. segitiga/rubah sederhana dari path garis); default (tanpa provider) = ikon cabang. Hindari logo merek persis; cukup ikon generik yang membedakan keduanya secara visual.

### 6. Placeholder komponen
- `MrList.svelte`: `let { provider, repo }: { provider: Provider; repo: Repo } = $props();`. Render `<p class="muted">Segera hadir</p>` (teks bisa menyebut "Merge Request"/"Pull Request" menurut provider). Props harus dideklarasikan walau belum dipakai (pakai `void provider; void repo;` atau gunakan di atribut `data-repo={repo.path}` untuk menghindari peringatan unused).
- `PipelineList.svelte`: sama, teks "Segera hadir".
- `MrPane.svelte`: `let { pane, visible }: { pane: Leaf; visible: boolean } = $props();`. Elemen root `<div class="git-pane" hidden={!visible}>` dengan header (judul `pane.title || 'Merge Request'`) dan teks sementara berisi `pane.provider`, `pane.projectId`, `pane.number` (mis. "Detail MR segera hadir"). Tanpa IPC.
- `PipelinePane.svelte`: sama, memakai `pane.pipelineId`, teks "Detail pipeline segera hadir".
- Gaya: variabel CSS yang sudah dipakai (`var(--fg, #ddd)`, `var(--fg-muted, #888)`, `var(--bg-elev, #222)`, `var(--border, #333)`).

### 7. `modules/git/ProjectSection.svelte`
- Props `{ provider, repo }`, ditambah prop internal opsional `index?: number` (default 0) untuk menentukan "bagian pertama terbuka". Karena props `{provider, repo}` kontrak untuk `MrList`/`PipelineList`, penambahan `index` hanya di `ProjectSection` (boleh, dipakai hanya oleh `GitPanel`).
- State: berlangganan `collapsed` lewat `$effect(() => collapsed.subscribe(v => { map = v; }))` (pola lark). `open = $derived(isSectionOpen(map, key, index))`, `key = repoKey(repo)`.
- Header `<button type="button" class="head" aria-expanded={open} onclick={() => toggleSection(key, open)}>` dengan chevron (`▾`/`▸`), judul `"<nama project> - <path>"`; nama project = `repo.projects.map(p => p.name).join(', ')` (fallback `repo.projectName`); atribut `title` berisi `repo.webUrl`.
- Isi HANYA dirender bila `open` (`{#if open}`): dua sub-bagian bertajuk menurut provider: `provider === 'gitlab' ? 'Merge Request' : 'Pull Request'` berisi `<MrList {provider} {repo} />`, dan `provider === 'gitlab' ? 'Pipeline' : 'Actions'` berisi `<PipelineList {provider} {repo} />`. Sub-bagian ini tidak perlu bisa dilipat di tiket ini.
- Opsional: tombol kecil "Buka repo" (memanggil `openInBrowser(repo.webUrl)`) di header? JANGAN (hindari tombol di dalam tombol); lewati.

### 8. `modules/git/GitPanel.svelte`
- Prop `{ provider }: { provider: Provider }`. Berlangganan `gitState`, dan `reposFor(provider)` lewat `$effect` (karena `provider` tetap per instance, aman; gunakan `$effect(() => reposFor(provider).subscribe(v => { repos = v; }))`).
- Header: judul `provider === 'gitlab' ? 'GITLAB' : 'GITHUB'` (gaya `.title` lark: font 13px, letter-spacing) dan tombol `Segarkan` (`aria-label="Segarkan"`, glyph `&#8635;`) memanggil fungsi `reload()`: `if (repos.length === 0) gitState.set({status:'loading'}); void refreshRepos();`.
- Badan:
  - `status === 'loading'` dan tidak ada repo: `<p class="muted">Memuat repo...</p>`.
  - `status === 'error'`: kotak galat (`role="alert"`) dengan teks sesuai kode: `cli_missing` -> `"<glab|gh> belum terpasang"` (ambil dari `message` bila ada), `not_logged_in` -> "Belum login", `permission` -> `Izin kurang`, `timeout` -> "Waktu habis", default `message ?? 'Terjadi galat'`; tombol "Buka Pengaturan > Connector" (`openSettings('connectors')`) bila kode `cli_missing`/`not_logged_in`; tombol "Coba lagi" (`reload`).
  - `status === 'ready'` dan repos kosong: `Tidak ada project dengan remote GitLab di workspace ini` / `... GitHub ...` menurut provider.
  - Repo ada: `{#each repos as repo, i (repoKey(repo))}<ProjectSection {provider} {repo} index={i} />{/each}`, urutan sesuai store.
  - Satu kalimat ajakan umum (muted, di bawah daftar atau di placeholder): "Bila daftar belum muncul, pastikan CLI sudah login di Pengaturan > Connector." Boleh ditaruh di footer panel.
- Gaya: salin pola `.lark`, `.header`, `.body`, `.errbox`, `.acts`, `.muted` dari `LarkPanel.svelte`. Fokus dan keyboard: semua kontrol `<button>`.

### 9. `modules/git/index.ts`
```ts
import { registerRailItem } from '../shell/registry';
import { registerPaneKind } from '../layout/registry';
import { reposFor } from './store';
import GitPanel from './GitPanel.svelte';
import GitIcon from './GitIcon.svelte';
import MrPane from './MrPane.svelte';
import PipelinePane from './PipelinePane.svelte';
```
- Fakta terverifikasi: `RailPanelHost.svelte` merender `<Active />` dan `Rail.svelte` merender `<Icon />`, keduanya TANPA props. Karena itu provider harus diikat lewat pembungkus tanpa berkas baru. Komponen Svelte 5 adalah fungsi `(anchor, props)`, jadi di `index.ts`: `import type { Component } from 'svelte';` lalu `const bind = (C: Component<any>, provider: Provider): Component<any> => ((anchor: any, props: Record<string, unknown> = {}) => (C as any)(anchor, { ...props, provider })) as unknown as Component<any>;`. Pakai `bind(GitPanel, 'gitlab')` / `bind(GitPanel, 'github')` untuk `component` dan `bind(GitIcon, 'gitlab')` / `bind(GitIcon, 'github')` untuk `icon`. Buat tiap pembungkus SEKALI di level modul (identitas stabil agar `{#key}`/derived tidak me-remount). Bila pembungkus ini gagal saat build atau runtime (mis. `npm run build` galat tipe atau panel tidak menerima provider), alternatif yang tetap di ownedPaths: jadikan `GitPanel` dan `GitIcon` menerima prop `provider` opsional dan sediakan dua modul-level wrapper lewat `export` kedua `<script module>` yang masing-masing mengembalikan fungsi komponen sama seperti `bind`; laporkan pendekatan yang dipakai.
- `registerRailItem({ id: 'git-gitlab', title: 'GitLab', icon: bind(GitIcon, 'gitlab'), component: bind(GitPanel, 'gitlab'), order: 20, visible: <derived gitlab> })` dan `git-github` (title 'GitHub', order 21, provider 'github'). `visible` = `derived(reposFor('gitlab'), (rs) => rs.length > 0)`; saat `loading`/galat repo kosong sehingga false (tidak flicker).
- `registerPaneKind({ kind: 'git-mr', component: MrPane, titleFor: (leaf) => leaf.title || (leaf.number !== undefined ? '#' + leaf.number : 'Merge Request') })` dan `registerPaneKind({ kind: 'git-pipeline', component: PipelinePane, titleFor: (leaf) => leaf.title || (leaf.pipelineId ? 'Pipeline ' + leaf.pipelineId : 'Pipeline') })`.
- Ekspor ulang bantu untuk tiket lain: `export { copyLink, openInBrowser, sendToAgent } from './actions';`.
- Impor `store.ts` di `index.ts` (sudah via `reposFor`) memastikan inisialisasi deteksi saat modul dimuat.

### 10. Verifikasi build
- `cd frontend && npm run build` harus sukses. Jalankan juga `npm run check` bila ada dan bandingkan hanya galat di file milik fitur ini (galat pra-ada di file lain diabaikan dan dilaporkan).
- Pastikan tidak ada impor dari `modules/lark` di `modules/git`.

## Perubahan
Dibuat:
- `frontend/src/modules/git/index.ts`
- `frontend/src/modules/git/store.ts`
- `frontend/src/modules/git/client.ts`
- `frontend/src/modules/git/actions.ts`
- `frontend/src/modules/git/GitIcon.svelte`
- `frontend/src/modules/git/GitPanel.svelte`
- `frontend/src/modules/git/ProjectSection.svelte`
- `frontend/src/modules/git/MrList.svelte`
- `frontend/src/modules/git/PipelineList.svelte`
- `frontend/src/modules/git/MrPane.svelte`
- `frontend/src/modules/git/PipelinePane.svelte`

Diubah (aditif):
- `frontend/src/modules/layout/model.ts` (PaneKind, `Leaf`, `NewLeaf`, `makeLeaf`, `hydrateNode`; tidak ada fungsi lain)

## Wiring
Fitur ini mengekspos tetapi TIDAK menyambungkan apa pun sendiri:
- `frontend/src/modules/git/index.ts` dimuat otomatis oleh `import.meta.glob('./modules/*/index.ts', { eager: true })` di `frontend/src/main.ts` (JANGAN diubah). Efeknya: dua rail item dan dua pane kind terdaftar, dan store memulai deteksi.
- Bergantung pada IPC `git.repos.list` dari fitur `git-repos-ipc-ctl` (daemon). Sebelum itu tersambung, `listRepos` akan gagal dan `gitState` menjadi `error`; rail Git tetap tersembunyi, aplikasi tidak terganggu. Langkah Wiring (overview bagian 5, butir 4 dan 5) memverifikasi rail/panel di aplikasi.
- Tiket 02 sampai 04 mengambil alih `MrList.svelte`, `PipelineList.svelte`, `PipelinePane.svelte`, `MrPane.svelte` tanpa mengubah props, dan memakai `selectedState`, `collapsed`, `actions.ts`, serta field leaf `provider`, `projectId`, `number`, `pipelineId`.
- Bila `lib/stores/persist.ts` atau serialisasi layout lain di luar `model.ts` memetakan field leaf secara eksplisit, itu bukan milik fitur ini: laporkan di Wiring.

## Selesai bila
- [ ] `cd frontend && npm run build` sukses (guard utama; tanpa unit test).
- [ ] Hanya berkas di "Path yang dimiliki" yang berubah (`git status` memuat hanya `frontend/src/modules/git/*` dan `frontend/src/modules/layout/model.ts` untuk fitur ini).
- [ ] `model.ts`: `PaneKind` memuat `'git-mr' | 'git-pipeline'`; `Leaf` dan `NewLeaf` memuat `number?` dan `pipelineId?`; `makeLeaf` menyalin keduanya; `hydrateNode` memulihkan `pipelineId` (string) dan `number` (cek `typeof === 'number'`); layout lama tanpa field itu tetap termuat; tidak ada perubahan perilaku lain.
- [ ] `client.ts`, `store.ts`, `actions.ts` mengekspor semua nama pada bagian Kontrak; `refreshRepos` memakai penjaga `seq` dan membuang hasil basi; pemicu: inisialisasi, pergantian `activeWorkspaceId`, event `workspace.changed` (`projectAdded`/`projectRemoved`/`deleted`/`configChanged` yang relevan), tombol Segarkan.
- [ ] `index.ts` mendaftarkan `git-gitlab` (order 20) dan `git-github` (order 21) dengan `visible` dari `reposFor(provider)`, serta pane kind `git-mr` dan `git-pipeline`.
- [ ] `GitPanel` menampilkan memuat/kosong/galat, tombol Segarkan, satu `ProjectSection` per repo; `ProjectSection` terlipat/terbuka dengan `aria-expanded`, bagian pertama terbuka default, state di `collapsed` store, isi hanya dirender saat terbuka; `MrList`/`PipelineList` ("Segera hadir") menerima props `{provider, repo}`.
- [ ] `sendToAgent` mengirim URL + satu spasi tanpa Enter; tanpa sesi aktif toast "Tidak ada sesi aktif".
- [ ] Tidak ada impor dari `modules/lark`, tidak ada perubahan pada `main.ts` atau file wiring lain.
