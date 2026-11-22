# Plan: DashboardPanel, AgentRow, DashboardRailIcon

## Metadata

- Tiket: `docs/tickets/agent-dashboard-ticket-01-agent-dashboard.md` (id `agent-dashboard-ticket-01-agent-dashboard`)
- Fitur: `dashboard-panel-ui` (Tahap B, paralel dengan `dashboard-store`, `rail-badge`)
- ownedPaths:
  - `frontend/src/modules/dashboard/DashboardPanel.svelte`
  - `frontend/src/modules/dashboard/AgentRow.svelte`
  - `frontend/src/modules/dashboard/DashboardRailIcon.svelte`
- Depends on foundation: ya. Memakai `types.ts` dan stub `store.ts` dari fitur `dashboard-foundation` (tahap A, sudah di-merge sebelum plan ini dikerjakan). Tidak memakai store nyata; kode ditulis terhadap kontrak overview bagian 2.2 dan 2.3.

## Tujuan

Membangun tiga komponen Svelte untuk panel rail "Dashboard": panel dengan ringkasan empat chip dan kelompok status, baris sesi agent yang dapat diklik dan dapat diakses (a11y), dan ikon rail SVG inline. Tidak ada logika data: semua data datang dari `agentRows`/`groupRows` di `./store`. Perilaku detail mengikuti bagian fitur `dashboard-panel-ui` di tiket.

## Path yang dimiliki

HANYA tiga file di atas yang boleh dibuat. Jangan menyentuh:
- `dashboard/types.ts`, `dashboard/store.ts`, `dashboard/index.ts` (fitur lain, termasuk wiring).
- `shell/registry.ts`, `shell/Rail.svelte`, `status/store.ts`, `activity/*`, `shell/helpers.ts`.
- `frontend/src/modules/sidebar-right/FilesTab.svelte` dan dokumen `docs/` (perubahan belum di-commit milik user).

Jika `types.ts`/`store.ts` belum ada di working tree saat mulai, hentikan dan laporkan; jangan membuatnya.

## Kontrak

Dipakai (lihat overview 2.2, 2.3, 2.6; tidak diulang di sini):
- Dari `./types`: tipe `AgentRow` (alias `AgentRowData` saat diimpor ke `AgentRow.svelte` agar tidak bentrok dengan nama komponen), `AgentStatus`, konstanta `STATUS_ORDER`, `STATUS_LABEL`.
- Dari `./store`: `agentRows: Readable<AgentRow[]>` dan `groupRows(rows): Record<AgentStatus, AgentRow[]>` (keempat kunci selalu ada, urutan input dipertahankan). Panel TIDAK mengimpor hal lain dari `./store` (bukan `waitingCount`).
- Dari `../activity/store`: `ensureActivityStore(): void` (dipanggil di `onMount`).
- Dari `../shell`: `focusOrOpenSession(sessionId: string): boolean` (pola impor sama dengan `notifications/Bell.svelte`: `import { focusOrOpenSession } from '../shell';`).

Disediakan (untuk wiring nanti):
- `DashboardPanel.svelte`: tanpa props, default export komponen.
- `AgentRow.svelte`: props `{ row: AgentRow; now: number }`.
- `DashboardRailIcon.svelte`: tanpa props.

Teks UI persis: "Menunggu", "Berjalan", "Idle", "Selesai" (dari `STATUS_LABEL`), "Belum ada sesi agent", "+N lainnya", "menunggu: <reason>" (tanpa reason: "menunggu"), durasi "baru saja", "N dtk", "N mnt", "N jam".

## Langkah

Gunakan Svelte 5 runes (`$props`, `$state`, `$derived`, `$effect`) dengan `<script lang="ts">`, seperti `Bell.svelte`. Auto-subscribe store dengan `$agentRows`. Warna memakai CSS variabel tema yang sudah ada dengan fallback: `--accent`, `--danger`, `--ok`, `--warn`, `--fg`, `--fg-muted`, `--border`, `--bg-elev`, `--bg-hover`. Gaya di blok `<style>` komponen; jangan edit CSS global.

### 1. `DashboardRailIcon.svelte`

SVG inline tanpa script, mengikuti `modules/grimoire/GrimoireRailIcon.svelte`: `width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"`. Gambar kisi 2x2 / daftar bertingkat sederhana, mis. `<rect x="3.5" y="3.5" width="7" height="7" rx="1.5"/>`, `<rect x="13.5" y="3.5" width="7" height="7" rx="1.5"/>`, `<rect x="3.5" y="13.5" width="7" height="7" rx="1.5"/>`, `<path d="M14 15.5h6.5M14 19h4"/>`. Tanpa dependensi.

### 2. `AgentRow.svelte`

Script:
```ts
import { focusOrOpenSession } from '../shell';
import type { AgentRow as AgentRowData } from './types';
import { STATUS_LABEL } from './types';

let { row, now }: { row: AgentRowData; now: number } = $props();

export function formatDuration(since: number, ref: number): string  // di module-level helper non-export juga boleh
```
- Helper lokal `formatDuration(since, now)`: `s = Math.max(0, Math.floor((now - since) / 1000))`; `s < 5` atau `s < 1` -> "baru saja" (pakai `s < 5`), `s < 60` -> `"${s} dtk"`, `m < 60` -> `"${m} mnt"`, selain itu `"${Math.floor(m / 60)} jam"`.
- `const duration = $derived(formatDuration(row.since, now))`.
- `const statusLabel = $derived(STATUS_LABEL[row.status])`.
- `const waitText = $derived(row.status === 'waiting' ? (row.reason ? \`menunggu: ${row.reason}\` : 'menunggu') : '')`.
- `const ariaLabel = $derived(\`${row.title}, ${statusLabel}, ${duration}\`)` (nama, status, lama; tambahkan `, ${row.projectName}` bila ada, opsional).
- `function open() { try { focusOrOpenSession(row.sessionId); } catch { /* abaikan */ } }`.
- Panah atas/bawah: handler `onkeydown` pada tombol; untuk `ArrowDown`/`ArrowUp` panggil `e.preventDefault()`, lalu cari tombol baris tetangga lewat DOM: dari `e.currentTarget` ambil kontainer terdekat `closest('[role="list"]')`... karena panel memakai beberapa kelompok, gunakan `closest('.rows-root')` (kelas pada wadah panel) lalu `querySelectorAll<HTMLButtonElement>('button.agent-row')`, cari indeks tombol saat ini, fokuskan indeks +/-1 (berhenti di ujung, tanpa melingkar). Tombol grup Selesai bukan `.agent-row` sehingga tidak ikut. `Home`/`End` opsional.

Markup (satu elemen per baris):
```svelte
<div role="listitem" class="item" class:waiting={row.status === 'waiting'}>
  <button type="button" class="agent-row" aria-label={ariaLabel} onclick={open} onkeydown={onKey}>
    <span class="status"><span class="dot dot-{row.status}" aria-hidden="true"></span><span class="status-label">{statusLabel}</span></span>
    <span class="main">
      <span class="name">{row.title}</span>
      <span class="project">{row.projectName}</span>
      <span class="duration">{duration}</span>
    </span>
    {#if waitText}<span class="wait">{waitText}</span>{/if}
    {#if row.lastActivity}<span class="activity" title={row.lastActivity}>{row.lastActivity}</span>{/if}
  </button>
</div>
```
- Catatan a11y: `role="listitem"` ada di wrapper `div`, bukan pada `<button>`, agar peran tombol tidak tertimpa. `aria-label` pada tombol memuat nama, status, lama (status diumumkan sebagai teks, selain `.status-label` yang terlihat).
- Warna titik per status lewat kelas: `.dot-waiting { background: var(--warn, #f5a524) }`, `.dot-running { background: var(--ok, #30a46c) }`, `.dot-idle { background: var(--fg-muted, #8b8d98) }`, `.dot-done { background: var(--accent, #3e63dd) }`. Titik 8px bulat; label teks selalu tampil.
- `.waiting` (pada `.item`): `border-left: 3px solid var(--warn, #f5a524)` dan `background: color-mix(in srgb, var(--warn, #f5a524) 10%, transparent)`; `.wait` berwarna `--warn`, `font-weight: 600`. Item non-waiting memakai `border-left: 3px solid transparent` agar lebar sejajar.
- `.agent-row`: `display: flex; flex-direction: column; gap: 2px; width: 100%; text-align: left; padding: 6px 10px; background: none; border: 0; color: var(--fg, inherit); cursor: pointer; font: inherit;`; `:hover`/`:focus-visible` memakai `var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent))`, `:focus-visible` tambah `outline: 1px solid var(--accent, currentColor)`.
- `.activity`: satu baris terpotong: `display: block; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: var(--fg-muted); font-size: 12px; min-width: 0`. Pastikan `.agent-row` dan `.main` punya `min-width: 0` agar ellipsis bekerja.
- `.name` tebal dan terpotong; `.project` dan `.duration` `color: var(--fg-muted)`, `.duration` rata kanan (`margin-left: auto`) dengan `flex: none`.

### 3. `DashboardPanel.svelte`

Script:
```ts
import { onMount } from 'svelte';
import { agentRows, groupRows } from './store';
import { STATUS_ORDER, STATUS_LABEL } from './types';
import type { AgentStatus } from './types';
import { ensureActivityStore } from '../activity/store';
import AgentRow from './AgentRow.svelte';

const DONE_LIMIT = 10;
let now = $state(Date.now());
let doneOpen = $state(true);   // Selesai terbuka secara default
let visible = $state(typeof document === 'undefined' ? true : !document.hidden);

const groups = $derived(groupRows($agentRows));
const total = $derived($agentRows.length);
const visibleGroups = $derived(STATUS_ORDER.filter((s) => groups[s].length > 0));
const doneRows = $derived(groups.done.slice(0, DONE_LIMIT));
const doneHidden = $derived(Math.max(0, groups.done.length - DONE_LIMIT));
```
- `onMount(() => { ensureActivityStore(); })`. (Idempotensi ditangani store; jangan dipanggil di tempat lain.)
- Pelacak visibilitas: `$effect` yang memasang `document.addEventListener('visibilitychange', ...)` untuk memperbarui `visible = !document.hidden` (dan `now = Date.now()` saat kembali terlihat), dibersihkan di return.
- Timer: `$effect(() => { if (!visible) return; now = Date.now(); const id = setInterval(() => { now = Date.now(); }, 1000); return () => clearInterval(id); });`. Tepat satu `setInterval`, hidup hanya saat terpasang dan tidak hidden; cleanup `$effect` otomatis saat komponen dilepas. Jangan ada timer lain, jangan ada `subscribe` manual (hanya `$agentRows` auto-subscribe, yang dilepas Svelte saat unmount).
- `counts`: `const counts = $derived(Object.fromEntries(STATUS_ORDER.map((s) => [s, groups[s].length])) as Record<AgentStatus, number>)`.

Markup:
```svelte
<div class="panel rows-root">
  <div class="title">Dashboard</div>
  <div class="summary" role="group" aria-label="Ringkasan sesi agent">
    {#each STATUS_ORDER as s (s)}
      <span class="chip chip-{s}"><span class="dot dot-{s}" aria-hidden="true"></span>{STATUS_LABEL[s]} {counts[s]}</span>
    {/each}
  </div>

  {#if total === 0}
    <div class="empty">Belum ada sesi agent</div>
  {:else}
    {#each visibleGroups as s (s)}
      <section class="group" aria-label={STATUS_LABEL[s]}>
        {#if s === 'done'}
          <button type="button" class="group-head toggle" aria-expanded={doneOpen} onclick={() => (doneOpen = !doneOpen)}>
            <span class="caret" aria-hidden="true">{doneOpen ? '▾' : '▸'}</span>{STATUS_LABEL[s]} <span class="count">{groups.done.length}</span>
          </button>
          {#if doneOpen}
            <div role="list" aria-label={STATUS_LABEL[s]}>
              {#each doneRows as row (row.sessionId)}<AgentRow {row} {now} />{/each}
            </div>
            {#if doneHidden > 0}<div class="more">+{doneHidden} lainnya</div>{/if}
          {/if}
        {:else}
          <div class="group-head">{STATUS_LABEL[s]} <span class="count">{groups[s].length}</span></div>
          <div role="list" aria-label={STATUS_LABEL[s]}>
            {#each groups[s] as row (row.sessionId)}<AgentRow {row} {now} />{/each}
          </div>
        {/if}
      </section>
    {/each}
  {/if}
</div>
```
- Kelompok kosong tidak dirender (lewat `visibleGroups`). Ringkasan empat chip selalu tampil, termasuk saat kosong (semua 0).
- Header kelompok non-Selesai bukan tombol; hanya Selesai yang dapat diciutkan (`aria-expanded`). Karakter caret boleh diganti SVG kecil; jangan pakai emoji.
- Class `rows-root` pada wadah terluar dipakai `AgentRow` untuk navigasi panah (lihat langkah 2). Pastikan seluruh daftar baris berada di dalam wadah ini.
- Gaya: panel `padding: 8px 0; overflow-y: auto; height: 100%; color: var(--fg); font-size: 13px;` `.title` huruf besar tebal `var(--fg-muted)` meniru `GrimoirePanel` (`font-weight: 600; text-transform: uppercase; font-size: 13px; padding: 0 10px 6px`). `.summary` `display: flex; flex-wrap: wrap; gap: 6px; padding: 0 10px 8px`. `.chip` `display: inline-flex; align-items: center; gap: 5px; padding: 2px 8px; border: 1px solid var(--border); border-radius: 999px; background: var(--bg-elev)`; titik chip memakai warna status yang sama dengan `AgentRow` (definisikan ulang `.dot-*` di panel). `.group-head` `padding: 6px 10px 2px; font-weight: 600; color: var(--fg-muted)`; `.toggle` tombol tanpa border selebar penuh dengan hover/focus-visible seperti baris. `.more` dan `.empty` `padding: 6px 10px; color: var(--fg-muted)`.
- Baris `AgentRow` memakai `{#each ... (row.sessionId)}` dengan key agar fokus tidak hilang saat urutan berubah.

### 4. Verifikasi lokal

Dari `frontend/`: `npm run build`, lalu `npm run check` bila skrip tersedia. Perbaiki semua galat tipe/Svelte di tiga file milik fitur ini. Jangan menambah dependensi; jangan menulis unit test.

## Perubahan

Dibuat (baru):
- `frontend/src/modules/dashboard/DashboardPanel.svelte`
- `frontend/src/modules/dashboard/AgentRow.svelte`
- `frontend/src/modules/dashboard/DashboardRailIcon.svelte`

Tidak ada file yang diubah.

## Wiring

Yang diekspos untuk langkah `dashboard-wiring` (jangan disambungkan di sini; jangan buat atau edit `dashboard/index.ts`):
- `DashboardPanel.svelte` sebagai `component` rail (tanpa props, dirender `{}`).
- `DashboardRailIcon.svelte` sebagai `icon` rail.
- `AgentRow.svelte` hanya dipakai internal oleh panel.

`ensureActivityStore()` sudah dipanggil di `onMount` panel; wiring tidak perlu memanggilnya. Pendaftaran `registerRailItem({... order: 50, badge: waitingCount })` sepenuhnya urusan `dashboard-wiring`.

## Selesai bila

- [ ] Tiga file ada di path yang benar dan tidak ada file lain yang disentuh (`git status` hanya menampilkan ketiganya sebagai file baru dari fitur ini).
- [ ] Ringkasan empat chip "Menunggu N / Berjalan N / Idle N / Selesai N" tampil dan akurat; kelompok kosong tersembunyi; keadaan kosong menampilkan "Belum ada sesi agent".
- [ ] Selesai maksimal 10 baris, header tombol dengan `aria-expanded`, "+N lainnya" saat terpotong.
- [ ] Baris menampilkan titik + label teks status, nama, proyek, lama (format "baru saja"/"N dtk"/"N mnt"/"N jam"), aktivitas terpotong satu baris; waiting menampilkan "menunggu: <reason>" (atau "menunggu") dan tampak menonjol.
- [ ] Klik baris memanggil `focusOrOpenSession(sessionId)` dalam try/catch.
- [ ] `role="list"`/`role="listitem"`, `aria-label` gabungan nama+status+lama, panah atas/bawah memindahkan fokus antar baris.
- [ ] Satu `setInterval` saja, hanya saat terpasang dan `!document.hidden`, dibersihkan saat dilepas; `ensureActivityStore()` hanya di `onMount`; tanpa subscription manual.
- [ ] `./store` hanya diimpor `agentRows` dan `groupRows`; tipe/konstanta dari `./types`; tanpa dependensi baru.
- [ ] Guard: `npm run build` di `frontend/` sukses (dan `npm run check` bila ada) tanpa galat dari path fitur ini; tanpa unit test.
