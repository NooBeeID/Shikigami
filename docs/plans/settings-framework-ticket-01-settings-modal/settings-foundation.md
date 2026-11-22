# Plan: Store, registri halaman, dan komponen Modal generik

## Metadata
- Tiket: `settings-framework-ticket-01-settings-modal` (`docs/tickets/settings-framework-ticket-01-settings-modal.md`)
- Fitur: `settings-foundation`
- ownedPaths:
  - `frontend/src/modules/settings/store.ts`
  - `frontend/src/modules/settings/pages.ts`
  - `frontend/src/modules/settings/Modal.svelte`
- Depends on foundation: ya. Stub berkompilasi dari `00-foundation.md` (bila sudah ada di worktree) ditimpa penuh oleh fitur ini. Bila ketiga berkas belum ada, buat langsung dengan isi final (tanpa komentar `STUB(foundation)`).

## Tujuan
Menyediakan tiga blok dasar modul `frontend/src/modules/settings`: registri halaman pengaturan (`pages.ts`), store buka/tutup modal dan halaman aktif (`store.ts`), dan komponen overlay `Modal.svelte` generik (Esc, klik backdrop, fokus masuk, focus trap, pengembalian fokus). Dipakai oleh `settings-shell-pages` dan kelak tiket 02. Seluruh kode frontend saja (Svelte 5 runes, TypeScript); modal hanya overlay, tidak menyentuh sesi terminal.

## Path yang dimiliki
HANYA tiga berkas ini yang boleh dibuat/diubah:
- `frontend/src/modules/settings/pages.ts`
- `frontend/src/modules/settings/store.ts`
- `frontend/src/modules/settings/Modal.svelte`

Jangan menyentuh `index.ts`, `SettingsModal.svelte`, `McpPage.svelte`, `ConnectorsPlaceholder.svelte`, `SettingsButton.svelte`, `App.svelte`, `main.ts`, `lib/registry.ts`, atau berkas lain. Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.

## Kontrak
Sumber kebenaran: `overview.md` bagian 2.1 (pages), 2.2 (store), 2.3 (Modal). Patuhi nama dan signature persis; jangan menambah ekspor atau props di luar kontrak.

Disediakan:
- `pages.ts`: `interface SettingsPageSpec { id: string; title: string; order?: number; component: Component<any>; }`, `registerSettingsPage(spec: SettingsPageSpec): void`, `settingsPages: Readable<SettingsPageSpec[]>`.
- `store.ts`: `settingsOpen: Writable<boolean>` (awal `false`), `activePage: Writable<string>` (awal `'connectors'`), `openSettings(pageId?: string): void`, `closeSettings(): void`.
- `Modal.svelte` props: `open: boolean`, `onclose: () => void`, `title?: string`, `children: Snippet`.

Dipakai: `svelte/store` (`writable`, `derived`, `get`), tipe `Component`/`Snippet` dari `svelte`. `store.ts` mengimpor `./pages`; `pages.ts` TIDAK mengimpor `store.ts` (hindari siklus). Pola registri mengikuti `panelStore`/`panels()` di `frontend/src/lib/registry.ts`.

## Langkah
1. **Prasyarat.** Cek apakah `frontend/src/modules/settings/` dan ketiga berkas ada. Bila ada dan berisi `STUB(foundation)`, timpa utuh. Bila berisi isi non-stub lain, hentikan dan laporkan. Buat folder bila belum ada. Jangan menyentuh berkas lain di folder itu.

2. **`pages.ts`.**
   ```ts
   import type { Component } from 'svelte';
   import { derived, writable, type Readable } from 'svelte/store';

   export interface SettingsPageSpec { id: string; title: string; order?: number; component: Component<any>; }

   const pageStore = writable<Record<string, SettingsPageSpec>>({});

   export function registerSettingsPage(spec: SettingsPageSpec): void {
     pageStore.update((m) => ({ ...m, [spec.id]: spec }));
   }

   export const settingsPages: Readable<SettingsPageSpec[]> = derived(pageStore, (m) =>
     Object.values(m).sort((a, b) => (a.order ?? 0) - (b.order ?? 0) || a.id.localeCompare(b.id)),
   );
   ```
   Tidak ada impor dari `./store`.

3. **`store.ts`.**
   ```ts
   import { get, writable, type Writable } from 'svelte/store';
   import { settingsPages } from './pages';

   export const settingsOpen: Writable<boolean> = writable(false);
   export const activePage: Writable<string> = writable('connectors');

   export function openSettings(pageId?: string): void {
     const pages = get(settingsPages);
     if (pageId !== undefined && pages.some((p) => p.id === pageId)) activePage.set(pageId);
     else if (pages.length > 0) activePage.set(pages[0].id);
     // daftar kosong: biarkan activePage apa adanya
     settingsOpen.set(true);
   }

   export function closeSettings(): void { settingsOpen.set(false); }
   ```
   Store tidak menyimpan elemen pemicu; pengembalian fokus urusan `Modal.svelte`.

4. **`Modal.svelte`** (Svelte 5 runes, `<script lang="ts">`).
   - Props: `let { open, onclose, title, children }: { open: boolean; onclose: () => void; title?: string; children: Snippet } = $props();`
   - Markup: `{#if open}` backdrop `<div class="backdrop" onpointerdown={onBackdrop}>` berisi dialog `<div class="dialog" role="dialog" aria-modal="true" aria-label={title} tabindex="-1" bind:this={dialogEl}>`. Di dalamnya `<header>` dengan `<span class="title">{title}</span>` dan `<button type="button" class="close" aria-label="Tutup" onclick={onclose}>` (karakter x/multiplication), lalu `<div class="body">{@render children()}</div>`. Saat `open=false` tidak ada elemen yang dirender. Bila ada peringatan a11y svelte soal `onpointerdown` pada div non-interaktif, tambahkan `<!-- svelte-ignore a11y_no_static_element_interactions -->` di atasnya (Esc ditangani global, jadi aman).
   - `onBackdrop(e: PointerEvent)`: `if (e.target === e.currentTarget) onclose();` (pakai `pointerdown`, bukan `click`, agar seleksi teks yang berakhir di luar dialog tidak menutup).
   - Efek buka/tutup dengan `$effect(() => { if (!open) return; ... return cleanup; })`:
     - Simpan `const trigger = document.activeElement as HTMLElement | null`.
     - Pasang `window.addEventListener('keydown', onKey, true)` (capture). Cleanup melepasnya, sehingga saat `open=false` tidak ada listener global.
     - Fokus masuk: setelah dialog ter-mount (gunakan `queueMicrotask` atau `tick()` agar `dialogEl` terisi), fokus ke elemen fokusable pertama di dalam `.body` bila ada, selain itu ke `dialogEl` (`tabindex="-1"`). Boleh juga selalu ke `dialogEl` bila elemen pertama adalah tombol x; pilih yang pertama kali memberi pengalaman wajar, tetapi fokus harus berada di dalam dialog.
     - Cleanup (dijalankan saat `open` menjadi false maupun saat komponen di-unmount): lepas listener; bila `trigger && trigger.isConnected` panggil `trigger.focus()`.
   - `onKey(e: KeyboardEvent)`:
     - `Escape`: `e.stopPropagation(); e.preventDefault(); onclose(); return;`
     - `Tab`: ambil daftar fokusable di dalam `dialogEl` (selector `a[href], button:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])`, saring yang tersembunyi bila perlu). Bila kosong: `preventDefault()` dan fokus ke `dialogEl`. Bila `Shift+Tab` dan fokus di elemen pertama (atau di `dialogEl`/di luar dialog): `preventDefault()`, fokus ke terakhir. Bila `Tab` tanpa shift dan fokus di terakhir (atau di luar dialog): `preventDefault()`, fokus ke pertama.
   - Gaya (`<style>`), hanya variabel CSS yang ada dengan fallback:
     - `.backdrop { position: fixed; inset: 0; z-index: 2000; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; }`
     - `.dialog { background: var(--bg-elev, #222); color: var(--fg, #eee); border: 1px solid var(--border, #444); border-radius: 8px; box-shadow: 0 12px 40px rgba(0,0,0,0.5); width: 760px; height: 520px; max-width: calc(100vw - 48px); max-height: calc(100vh - 48px); display: flex; flex-direction: column; outline: none; }`
     - `header` flex baris, `justify-content: space-between`, padding sekitar 10px 14px, `border-bottom: 1px solid var(--border, #444)`; `.title` tebal; `.close` tombol transparan `color: var(--fg-muted, #999)`, hover `background: var(--bg-hover, #333)` dan `color: var(--fg, #eee)`, radius 4px.
     - `.body { flex: 1; min-height: 0; overflow: hidden; }` (konten anak mengatur scroll sendiri).
   - Modal tidak memanggil fungsi terminal, tidak mengubah state sesi.

5. **Verifikasi lokal.** `cd frontend && npm run build` (dan `npm run check` bila ada) lulus tanpa error baru; `git status` hanya menampilkan tiga berkas milik fitur ini; `grep -n "STUB(foundation)"` pada ketiganya kosong. Tanpa unit test.

## Perubahan
- Dibuat/ditimpa: `frontend/src/modules/settings/pages.ts`
- Dibuat/ditimpa: `frontend/src/modules/settings/store.ts`
- Dibuat/ditimpa: `frontend/src/modules/settings/Modal.svelte`

## Wiring
Tidak menyambungkan apa pun sendiri. Yang diekspos:
- `registerSettingsPage`, `settingsPages`, tipe `SettingsPageSpec` (`./pages`) untuk `index.ts` dan `SettingsModal.svelte`.
- `settingsOpen`, `activePage`, `openSettings`, `closeSettings` (`./store`) untuk `SettingsModal.svelte` dan barrel `index.ts` (yang kelak dipakai `SettingsButton.svelte` lewat `'../settings'`).
- `Modal.svelte` (default) untuk `SettingsModal.svelte` dan tiket 02.
Re-export lewat barrel dan mount `SettingsModal` di `App.svelte` adalah tugas fitur lain dan langkah Wiring akhir; jangan dikerjakan di sini.

## Selesai bila
- [ ] `openSettings()` membuka dan memilih halaman pertama terurut; `openSettings('mcp')` memilih `mcp` bila terdaftar; id tak dikenal jatuh ke halaman pertama; daftar kosong tidak mengubah `activePage`.
- [ ] `closeSettings()` menutup; `settingsOpen` dapat di-subscribe.
- [ ] `registerSettingsPage` dengan id sama menimpa; `settingsPages` terurut `order ?? 0` lalu `id.localeCompare`; `pages.ts` tidak mengimpor `store.ts`.
- [ ] `Modal.svelte` tidak merender apa pun dan tidak memasang listener global saat `open=false`.
- [ ] Esc (capture, `stopPropagation`) dan `pointerdown` pada backdrop (target tepat backdrop) memanggil `onclose`; klik di dalam dialog tidak menutup.
- [ ] Dialog punya `role="dialog"`, `aria-modal="true"`, `aria-label`; fokus masuk saat buka, Tab/Shift+Tab berputar di dalam dialog, fokus kembali ke pemicu saat tutup atau unmount; tombol x ber-`aria-label="Tutup"`; backdrop `z-index: 2000`.
- [ ] Gaya hanya memakai `--bg-elev`, `--border`, `--fg`, `--fg-muted`, `--bg-hover` dengan fallback.
- [ ] Guard: build frontend path fitur ini sukses (`cd frontend && npm run build`; `npm run check` tanpa error baru bila tersedia). Tanpa unit test. Hanya tiga ownedPaths yang berubah.
