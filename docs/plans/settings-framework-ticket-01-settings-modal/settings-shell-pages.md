# Plan: SettingsModal, halaman MCP kerangka, placeholder Connector, barrel index

## Metadata
- Tiket: `settings-framework-ticket-01-settings-modal` (`docs/tickets/settings-framework-ticket-01-settings-modal.md`)
- Fitur: `settings-shell-pages`
- Overview: `docs/plans/settings-framework-ticket-01-settings-modal/overview.md` (bagian 2 adalah sumber kebenaran kontrak)
- ownedPaths:
  - `frontend/src/modules/settings/SettingsModal.svelte`
  - `frontend/src/modules/settings/McpPage.svelte`
  - `frontend/src/modules/settings/ConnectorsPlaceholder.svelte`
  - `frontend/src/modules/settings/index.ts`
- Depends on foundation: ya. Stub foundation (`pages.ts`, `store.ts`, `Modal.svelte`, `index.ts`) sudah ada di worktree dengan signature final. `index.ts` stub ditimpa penuh oleh fitur ini. `store.ts`, `pages.ts`, `Modal.svelte` milik `settings-foundation` (agent lain, mungkin masih stub); jangan diedit.

## Tujuan
Membangun UI modal Pengaturan: `SettingsModal.svelte` (sidebar tab vertikal + panel konten), dua halaman bawaan (`ConnectorsPlaceholder`, `McpPage`), dan barrel `index.ts` yang mendaftarkan halaman, mendaftarkan panel `settings-modal` di area `statusbar`, serta me-re-export API publik modul. Semua teks UI bahasa Indonesia, identifier bahasa Inggris.

## Path yang dimiliki
HANYA empat berkas di bagian Metadata yang boleh dibuat/diubah. Jangan menyentuh: `store.ts`, `pages.ts`, `Modal.svelte` (fitur foundation), `frontend/src/modules/topbar/SettingsButton.svelte` (fitur gear-entry), `frontend/src/App.svelte`, `frontend/src/main.ts`, `frontend/src/lib/registry.ts`, `overview.md`, dan folder `frontend/src/modules/settings/connectors/` (milik tiket 02; jangan dibuat).

## Kontrak
Dipakai (lihat overview 2.1 sampai 2.3, jangan diubah):
- Dari `./store`: `settingsOpen: Writable<boolean>`, `activePage: Writable<string>`, `closeSettings(): void`, `openSettings(pageId?: string): void`.
- Dari `./pages`: `registerSettingsPage(spec: SettingsPageSpec): void`, `settingsPages: Readable<SettingsPageSpec[]>` (sudah terurut), tipe `SettingsPageSpec { id; title; order?; component: Component<any> }`.
- Dari `./Modal.svelte` (default): props `open: boolean`, `onclose: () => void`, `title?: string`, `children: Snippet`.
- Dari `../../lib/registry`: `registerPanel({ id, area, order, component })`.
- Variabel CSS dengan fallback: `--bg-elev`, `--border`, `--fg`, `--fg-muted`, `--bg-hover`.

Disediakan (overview 2.4 dan 2.5):
- `ConnectorsPlaceholder.svelte` dan `McpPage.svelte`: tanpa props.
- `SettingsModal.svelte`: tanpa props, juga komponen host panel.
- `index.ts` ekspor: `openSettings`, `closeSettings`, `settingsOpen`, `activePage`, `registerSettingsPage`, `settingsPages`, tipe `SettingsPageSpec`, `Modal`, `SettingsModal`. Side effect: registrasi halaman `connectors` (title "Connector", order 10) dan `mcp` (title "MCP", order 20), serta `registerPanel({ id: 'settings-modal', area: 'statusbar', order: 95, component: SettingsModal })`.

## Langkah
1. **Prasyarat.** Konfirmasi `frontend/src/modules/settings/` berisi stub foundation (`pages.ts`, `store.ts`, `Modal.svelte`, `index.ts`). Bila belum ada, tunggu/laporkan; jangan membuatnya sendiri. Baca `store.ts` dan `pages.ts` untuk memastikan nama ekspor sesuai kontrak.

2. **`ConnectorsPlaceholder.svelte`.** Svelte 5, tanpa props, tanpa logika. Markup: `<section>` berisi `<h2>Connector</h2>` dan `<p>Daftar connector segera hadir</p>`. Gaya lokal: `h2 { margin: 0 0 8px; font-size: 16px; color: var(--fg, #ddd); }`, `p { margin: 0; color: var(--fg-muted, #888); font-size: 13px; }`.

3. **`McpPage.svelte`.** Sama bentuknya: `<h2>MCP</h2>` dan `<p>Pengelolaan server MCP segera hadir</p>`, gaya identik.

4. **`SettingsModal.svelte`.** Svelte 5 runes, `<script lang="ts">`, tanpa `$props`.
   - Impor: `Modal from './Modal.svelte'`, `{ settingsOpen, activePage, closeSettings } from './store'`, `{ settingsPages } from './pages'`.
   - Turunan: `const pages = $derived($settingsPages)`; `const current = $derived(pages.find((p) => p.id === $activePage) ?? pages[0])`. Id aktif efektif = `current?.id`. Gunakan sintaks store `$store` auto-subscribe (valid di runes mode untuk store top-level).
   - Markup: `<Modal open={$settingsOpen} onclose={closeSettings} title="Pengaturan">` berisi `<div class="settings-body">` dengan dua anak:
     - `<div class="settings-tabs" role="tablist" aria-orientation="vertical" aria-label="Halaman pengaturan">`; `{#each pages as p (p.id)}` render `<button type="button" role="tab" id={'settings-tab-' + p.id} aria-selected={p.id === current?.id} aria-controls="settings-panel" tabindex={p.id === current?.id ? 0 : -1} class:active={p.id === current?.id} onclick={() => select(p.id)} onkeydown={onTabKey}>{p.title}</button>`.
     - `<div class="settings-panel" id="settings-panel" role="tabpanel" aria-labelledby={current ? 'settings-tab-' + current.id : undefined}>`; di dalamnya `{#if current}{@const Page = current.component}<Page />{/if}`.
   - Fungsi `select(id: string): void` set `activePage.set(id)`.
   - `onTabKey(e: KeyboardEvent)`: ambil `idx` dari indeks `current` di `pages`. `ArrowDown` -> `(idx+1) % n`; `ArrowUp` -> `(idx-1+n) % n`; `Home` -> 0; `End` -> `n-1`; tombol lain return. Untuk tombol yang ditangani: `e.preventDefault()`, `e.stopPropagation()`, `select(pages[next].id)`, lalu setelah DOM diperbarui (`await tick()` dari `svelte`) fokuskan tombol tab dengan `document.getElementById('settings-tab-' + id)?.focus()`. Abaikan bila `n === 0`.
   - Gaya (lokal, hanya variabel yang ada dengan fallback): `.settings-body { display: flex; min-height: 0; height: 100%; width: 100%; }`; `.settings-tabs { flex: 0 0 180px; width: 180px; display: flex; flex-direction: column; gap: 2px; padding: 8px; border-right: 1px solid var(--border, #333); overflow-y: auto; }`; tab: `button[role='tab'] { text-align: left; background: transparent; border: 0; border-radius: 6px; padding: 8px 10px; color: var(--fg, #ddd); font: inherit; cursor: pointer; }`, `:hover` dan `.active` memakai `background: var(--bg-hover, #333)`, `:focus-visible { outline: 1px solid var(--fg-muted, #888); outline-offset: -1px; }`; `.settings-panel { flex: 1; min-width: 0; padding: 16px; overflow: auto; }`.
   - Karena `Modal` tidak merender apa pun saat `open=false`, komponen ini aman dipakai sebagai host panel tanpa elemen tambahan di DOM. Jangan menambah elemen di luar `<Modal>`.
   - Catatan layout: bila dialog `Modal` (milik foundation) tidak memberi tinggi, `.settings-body` tetap dibatasi `min-height: 360px` agar sidebar/konten terlihat; jangan mengubah `Modal.svelte`.

5. **`index.ts`** (timpa penuh stub; hapus baris `STUB(foundation)`). Isi:
   ```ts
   import { registerPanel } from '../../lib/registry';
   import { registerSettingsPage } from './pages';
   import ConnectorsPlaceholder from './ConnectorsPlaceholder.svelte';
   import McpPage from './McpPage.svelte';
   import SettingsModal from './SettingsModal.svelte';

   registerSettingsPage({ id: 'connectors', title: 'Connector', order: 10, component: ConnectorsPlaceholder });
   registerSettingsPage({ id: 'mcp', title: 'MCP', order: 20, component: McpPage });
   registerPanel({ id: 'settings-modal', area: 'statusbar', order: 95, component: SettingsModal });

   export { openSettings, closeSettings, settingsOpen, activePage } from './store';
   export { registerSettingsPage, settingsPages } from './pages';
   export type { SettingsPageSpec } from './pages';
   export { default as Modal } from './Modal.svelte';
   export { default as SettingsModal } from './SettingsModal.svelte';
   ```
   Tambahkan komentar singkat bahwa tiket 02 menimpa `connectors` lewat `registerSettingsPage`.

6. **Verifikasi.** `cd frontend && npm run build` lulus; `npm run check` tanpa error baru dari empat berkas ini; `go build ./...` lulus. `git status` hanya menampilkan perubahan pada empat ownedPaths. `grep -n "STUB(foundation)" frontend/src/modules/settings/index.ts` kosong. Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.

## Perubahan
- Baru: `frontend/src/modules/settings/SettingsModal.svelte`
- Baru: `frontend/src/modules/settings/McpPage.svelte`
- Baru: `frontend/src/modules/settings/ConnectorsPlaceholder.svelte`
- Diubah (menimpa stub foundation): `frontend/src/modules/settings/index.ts`

## Wiring
Fitur ini mengekspos, untuk disambungkan nanti oleh langkah Wiring (JANGAN dikerjakan di sini):
- `SettingsModal` (ekspor bernama dari `modules/settings/index.ts`, tanpa props) agar `App.svelte` dapat `import { SettingsModal } from './modules/settings'` dan merender `<SettingsModal />` setelah `<Palette />`, karena belum ada renderer `panels('statusbar')`.
- Panel `settings-modal` terdaftar di `statusbar` (order 95) sebagai cadangan bila kelak ada renderer panel.
- Halaman `connectors` dan `mcp` terdaftar; tiket 02 menimpa `connectors` lewat `registerSettingsPage` tanpa mengubah `SettingsModal.svelte`.
- `openSettings`/`closeSettings`/`settingsOpen`/`activePage` untuk dipakai `SettingsButton.svelte` lewat `'../settings'`.

## Selesai bila
- Empat berkas ownedPaths ada dengan isi sesuai langkah; hanya berkas tersebut yang berubah.
- Sidebar menampilkan "Connector" lalu "MCP" (urutan `settingsPages`), item aktif berlatar `--bg-hover`, roving tabindex benar, `aria-selected` sesuai.
- Panah atas/bawah (wrap) dan Home/End memindahkan fokus dan halaman; klik juga berpindah halaman.
- Konten kanan merender komponen `id === $activePage` dengan fallback halaman pertama; teks placeholder Connector dan MCP persis sesuai overview 2.4.
- Mendaftarkan ulang id `connectors` dengan komponen lain mengganti isi tanpa mengubah `SettingsModal.svelte`.
- `index.ts` mengekspor semua nama pada overview 2.5, mendaftarkan dua halaman dan panel `settings-modal` (statusbar, order 95), tanpa sisa `STUB(foundation)`.
- Guard: `cd frontend && npm run build` sukses (path fitur ini), `npm run check` tanpa error baru, `go build ./...` sukses. Tanpa unit test.
