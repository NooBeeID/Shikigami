# Plan: Komponen Welcome.svelte

## Metadata
- Tiket: `welcome-screen-ticket-02-welcome-screen` (`docs/tickets/welcome-screen-ticket-02-welcome-screen.md`)
- Fitur: `welcome-ui`
- ownedPaths:
  - `frontend/src/modules/welcome/Welcome.svelte`
  - `frontend/src/modules/welcome/index.ts`
- Depends on foundation: tidak (tidak ada `00-foundation.md`)
- Kontrak bersama: `docs/plans/welcome-screen-ticket-02-welcome-screen/overview.md` (bagian 2.2 dan 2.3). Jangan ubah overview.

## Tujuan
Membuat komponen layar welcome untuk area tengah: dua tombol berikon (Open Workspace, Create New Workspace), bagian "Terakhir dibuka" (maks 5, nama + path project pertama, teks kosong "Belum ada workspace terakhir"), dan input nama inline setelah Create New (Enter/blur rename lalu buka; Escape tetap buka; guard `committing`). Tema lewat variabel CSS, Svelte 5 runes.

## Path yang dimiliki
HANYA file ini yang boleh dibuat/diubah:
- `frontend/src/modules/welcome/Welcome.svelte`
- `frontend/src/modules/welcome/index.ts`

Jangan sentuh `frontend/src/modules/welcome/logic.ts` (milik `welcome-logic`, agent lain), `frontend/src/App.svelte` (milik `welcome-wiring`), `WorkspaceTabs.svelte`, `lib/stores/*`, `modules/openfolder/*`, `modules/topbar/*`, kode Go. Tanpa dependency baru, tanpa unit test, tanpa commit di luar alur implement-feature.

## Kontrak
Disediakan (lihat overview 2.3):
- `index.ts`: `export { default as Welcome } from './Welcome.svelte';` (hanya baris itu).
- `Welcome.svelte`: tanpa props.

Dipakai (lihat overview 2.2; ikuti tanda tangan persis, `logic.ts` mungkin belum ada saat kamu mengerjakan):
- Dari `./logic`: `recentWorkspaces(workspaces: Workspace[], openIds: string[], limit = 5): Workspace[]`, `openFromDisk(): Promise<void>`, `createNew(): Promise<Workspace | null>`, `openRecent(id: string): Promise<void>`.
- Dari `../../lib/stores`: `workspaces`, `openWorkspaceIds`, `openWorkspace(id)`, `renameWorkspace(id, name): Promise<void>`, tipe `Workspace`.
- Dari `../openfolder/toast-store`: `showToast(msg, 'error')`.
- Dari `../openfolder/logic`: `errMessage(e)`.
- Variabel tema (`styles/theme.css`): `--bg`, `--bg-elev`, `--bg-hover`, `--border`, `--fg`, `--fg-muted`, `--accent`, `--danger`.

Catatan: `openWorkspaceIds` dan `openWorkspace` berasal dari tiket 1. Sebelum menulis, verifikasi nama ekspor dengan `grep -n "export" frontend/src/lib/stores/workspaces.ts` (saat plan ditulis, file itu belum memuatnya; bila tiket 1 belum merge, ikuti nama di tiket/overview). Bila nama berbeda sedikit, ikuti kode.

## Langkah
1. Verifikasi awal (baca saja): `frontend/src/modules/topbar/WorkspaceTabs.svelte` (pola `focusSelect`, `commit`, `onEditKey`, flag `cancelled`/`committing`), `frontend/src/styles/theme.css` (nama variabel), `frontend/src/lib/stores/workspaces.ts` (nama ekspor), `frontend/src/modules/openfolder/toast-store.ts` (signature `showToast`). Salin pola, JANGAN impor dari `WorkspaceTabs`.
2. Buat `frontend/src/modules/welcome/index.ts` berisi tepat:
   ```ts
   export { default as Welcome } from './Welcome.svelte';
   ```
3. Buat `frontend/src/modules/welcome/Welcome.svelte` dengan `<script lang="ts">`:
   - Impor: `workspaces`, `openWorkspaceIds`, `openWorkspace`, `renameWorkspace` dari `../../lib/stores`; `recentWorkspaces`, `openFromDisk`, `createNew`, `openRecent` dari `./logic`; `showToast` dari `../openfolder/toast-store`; `errMessage` dari `../openfolder/logic`.
   - State: `let pendingId = $state<string | null>(null)`, `let draftName = $state('')`, `let initialName = ''` (nama default awal, non-reaktif), `let committing = $state(false)`, `let cancelled = false` (non-reaktif), `let creating = $state(false)` (guard klik ganda tombol Create New).
   - Turunan: `const recent = $derived(recentWorkspaces($workspaces, $openWorkspaceIds));` `const inputActive = $derived(pendingId !== null);`
   - `function focusSelect(node: HTMLInputElement) { node.focus(); node.select(); }` (action).
   - `async function onCreate()`: return bila `creating || inputActive`; `creating = true`; `try { const ws = await createNew(); if (ws) { pendingId = ws.id; initialName = ws.name; draftName = ws.name; cancelled = false; committing = false; } } finally { creating = false; }`. (`createNew` sudah menangani error dengan toast dan mengembalikan `null`.)
   - `async function commit()`: `if (committing || pendingId === null) return; committing = true; const id = pendingId; const name = draftName.trim();` bila `!cancelled && name !== '' && name !== initialName` -> `try { await renameWorkspace(id, name); } catch (e) { showToast(errMessage(e), 'error'); }` (lanjut walau gagal); lalu `try { await openWorkspace(id); } catch (e) { showToast(errMessage(e), 'error'); } finally { pendingId = null; committing = false; }`. Setelah `openWorkspace` sukses `activeWorkspaceId` terisi sehingga wiring menyembunyikan Welcome; reset state di `finally` tetap aman.
   - `function onEditKey(e: KeyboardEvent)`: `Enter` -> `e.preventDefault(); commit();`; `Escape` -> `e.preventDefault(); cancelled = true; commit();` (batal rename tetapi tetap `openWorkspace`).
   - Markup (satu `<section class="welcome">` terpusat vertikal/horizontal):
     - Judul `<h1>` singkat (mis. "Selamat datang").
     - Baris aksi `.actions`:
       - `<button type="button" class="action" disabled={inputActive || creating} onclick={() => openFromDisk()}>` dengan SVG ikon folder inline (`aria-hidden="true"`) + teks "Open Workspace". Tidak perlu `await`; `openFromDisk` tidak melempar.
       - Bila `inputActive`: `<input class="name-input" type="text" aria-label="Nama workspace" bind:value={draftName} use:focusSelect onkeydown={onEditKey} onblur={commit} />` menggantikan tombol Create New. Selain itu: `<button type="button" class="action" disabled={creating} onclick={onCreate}>` dengan SVG ikon plus + teks "Create New Workspace".
       - Saat `inputActive`, tombol Open Workspace `disabled` (sudah di atas); daftar recent juga dinonaktifkan (`disabled` pada tombol baris).
     - Bagian `.recent`: `<h2>Terakhir dibuka</h2>`; bila `recent.length === 0` -> `<p class="empty">Belum ada workspace terakhir</p>`; selain itu `<ul>` dengan `{#each recent as ws (ws.id)}` berisi `<li><button type="button" class="row" disabled={inputActive} onclick={() => openRecent(ws.id)}><span class="name">{ws.name}</span><span class="path">{ws.projects[0]?.path ?? ''}</span></button></li>{/each}`. Tidak perlu slice tambahan (maks 5 sudah dari `recentWorkspaces`).
   - CSS (`<style>` scoped), hanya variabel tema, tanpa warna hard-coded:
     - `.welcome { height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 24px; padding: 24px; box-sizing: border-box; background: var(--bg); color: var(--fg); overflow-x: hidden; min-width: 0; }`
     - `.actions { display: flex; gap: 12px; flex-wrap: wrap; justify-content: center; }`
     - `.action { display: inline-flex; align-items: center; gap: 8px; padding: 10px 16px; background: var(--bg-elev); color: var(--fg); border: 1px solid var(--border); border-radius: 8px; cursor: pointer; font: inherit; }` `.action:hover:not(:disabled) { background: var(--bg-hover); }` `.action:disabled { opacity: 0.5; cursor: default; }`
     - `.name-input { padding: 10px 12px; min-width: 240px; background: var(--bg); color: var(--fg); border: 1px solid var(--accent); border-radius: 8px; font: inherit; }` `outline: none`.
     - `.recent { width: min(480px, 100%); }` `h2 { font-size: 12px; text-transform: uppercase; color: var(--fg-muted); margin: 0 0 8px; }` `ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }`
     - `.row { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; width: 100%; padding: 8px 12px; background: transparent; color: var(--fg); border: 1px solid transparent; border-radius: 6px; cursor: pointer; text-align: left; font: inherit; }` `.row:hover:not(:disabled) { background: var(--bg-hover); }` `.row:disabled { opacity: 0.5; cursor: default; }`
     - `.name { font-weight: 600; }` `.path { color: var(--fg-muted); font-size: 12px; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }`
     - `.empty { color: var(--fg-muted); margin: 0; }`
     - `.action:focus-visible, .row:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }`
   - Svelte 5 runes dan atribut `onclick`/`onkeydown`/`onblur` (bukan `on:`), konsisten dengan kode yang ada.
4. Build guard: `cd frontend && npm run build` lalu `npm run check`. Bila gagal hanya karena `./logic` atau ekspor tiket 1 belum ada (fitur paralel belum merge), pastikan kesalahan terbatas pada impor itu dan tanda tangan sesuai kontrak; jangan membuat stub `logic.ts` (bukan milikmu). Bila `logic.ts` sudah ada, build harus bersih.

## Perubahan
- Baru: `frontend/src/modules/welcome/index.ts`
- Baru: `frontend/src/modules/welcome/Welcome.svelte`
- Tidak ada file lain yang diubah (folder `modules/welcome/` dibuat bila belum ada).

## Wiring
Yang diekspos: `Welcome` dari `frontend/src/modules/welcome/index.ts` (komponen tanpa props). Disambungkan nanti oleh `welcome-wiring` di `App.svelte` (`{#if $activeWorkspaceId === null}<Welcome />{:else}<LayoutArea />{/if}` pada snippet `center`). JANGAN mengedit `App.svelte` atau mendaftarkan apa pun (`registerPanel`) di fitur ini.

## Selesai bila
- [ ] `index.ts` hanya mengekspor `Welcome`; `Welcome.svelte` tanpa props.
- [ ] Dua tombol berikon dengan teks "Open Workspace" dan "Create New Workspace"; masing-masing memanggil `openFromDisk()` dan alur `createNew()`.
- [ ] "Terakhir dibuka" memakai `recentWorkspaces($workspaces, $openWorkspaceIds)`, menampilkan nama + path project pertama, klik memanggil `openRecent(id)`; kosong menampilkan "Belum ada workspace terakhir".
- [ ] Setelah `createNew()` non-null, input inline terfokus dan terseleksi; Enter/blur rename (bila berubah, tidak kosong) lalu `openWorkspace`; Escape tetap `openWorkspace` tanpa rename; guard `committing` mencegah proses ganda; tombol lain `disabled` saat input aktif.
- [ ] Hanya variabel tema (tanpa warna hard-coded), `height: 100%`, tanpa scroll horizontal, `:focus-visible` terlihat.
- [ ] Tidak ada file di luar ownedPaths yang berubah (`git status` hanya menunjukkan dua file ini).
- [ ] Guard: `cd frontend && npm run build` (dan `npm run check`) sukses untuk path fitur ini (dengan `logic.ts` dan store tiket 1 tersedia); tanpa unit test.
