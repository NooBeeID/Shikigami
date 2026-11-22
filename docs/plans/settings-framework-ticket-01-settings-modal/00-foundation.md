# Foundation: settings-framework-ticket-01-settings-modal (stub berkompilasi)

Baca dulu `overview.md` (bagian 2 adalah sumber kebenaran; plan ini hanya urutan kerja) dan tiket `docs/tickets/settings-framework-ticket-01-settings-modal.md`. Tanpa logika lengkap, tanpa dependency baru, tanpa unit test.

Tujuan: setelah foundation, tiga fitur dapat dikerjakan paralel karena semua signature lintas-fitur ada dan build frontend lulus.

Berkas yang dibuat (semua di `frontend/src/modules/settings/`): `pages.ts`, `store.ts`, `Modal.svelte`, `index.ts`. Setiap berkas memiliki komentar `STUB(foundation): ditimpa oleh <fitur>` di baris pertama (di `.svelte`, sebagai komentar HTML paling atas).

## Langkah

1. **Prasyarat.** Pastikan `frontend/src/modules/settings/` belum ada (atau kosong) dan `frontend/src/lib/registry.ts` ada. Bila folder sudah berisi berkas non-stub, berhenti dan laporkan.
2. **`pages.ts`** (`// STUB(foundation): ditimpa oleh settings-foundation`). Ekspor `SettingsPageSpec`, `registerSettingsPage`, `settingsPages` sesuai overview 2.1. Implementasi minimal tapi benar: `writable<Record<string, SettingsPageSpec>>({})`, `registerSettingsPage` memakai `update`, `settingsPages` derived dengan sort `(order ?? 0)` lalu `id.localeCompare`. Tidak mengimpor `store.ts`.
3. **`store.ts`** (`// STUB(foundation): ditimpa oleh settings-foundation`). Ekspor `settingsOpen` (writable false), `activePage` (writable `'connectors'`), `openSettings(pageId?)`, `closeSettings()` sesuai overview 2.2; mengimpor `get` dan `settingsPages` dari `./pages`.
4. **`Modal.svelte`** (`<!-- STUB(foundation): ditimpa oleh settings-foundation -->`). Svelte 5 runes, props `open`, `onclose`, `title?`, `children` (`Snippet`). Stub: `{#if open}` merender backdrop `position: fixed; inset: 0; z-index: 2000` dan dialog `role="dialog" aria-modal="true" aria-label={title}` berisi `{@render children()}`; tanpa listener, tanpa fokus (itu milik fitur).
5. **`index.ts`** (`// STUB(foundation): ditimpa oleh settings-shell-pages`). Hanya re-export: `openSettings`, `closeSettings`, `settingsOpen`, `activePage` dari `./store`; `registerSettingsPage`, `settingsPages`, tipe `SettingsPageSpec` dari `./pages`; `default as Modal` dari `./Modal.svelte`. Tanpa registrasi halaman/panel dan tanpa ekspor `SettingsModal` (belum ada berkasnya; ditambahkan oleh `settings-shell-pages`).
6. **Verifikasi.** `cd frontend && npm run build` lulus; `npm run check` tidak menambah error; `go build ./...` lulus; `git status` hanya menampilkan empat berkas baru di atas; `main.ts`, `SettingsButton.svelte`, `App.svelte`, dan `lib/registry.ts` tidak disentuh.

## Kriteria selesai

- Semua signature overview bagian 2.1, 2.2, 2.3 (props) dan re-export barrel (kecuali `SettingsModal`) ada dan berkompilasi.
- Tiap stub bertanda `STUB(foundation)` di baris pertama.
- Build guard lulus. Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
