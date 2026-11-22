# Plan: Entri 'Semua pengaturan…' di dropdown gear

## Metadata

- Tiket: `settings-framework-ticket-01-settings-modal` (`docs/tickets/settings-framework-ticket-01-settings-modal.md`, bagian fitur `settings-gear-entry`)
- Fitur: `settings-gear-entry`
- ownedPaths: `frontend/src/modules/topbar/SettingsButton.svelte`
- Depends on foundation: ya. Memakai barrel `frontend/src/modules/settings/index.ts` hasil stub foundation (ekspor `openSettings` sudah ada di stub, sehingga build lulus di worktree ini walau fitur lain belum selesai). Plan ini tidak membaca atau mengubah plan fitur lain.

## Tujuan

Menambah item "Semua pengaturan…" pada dropdown gear. Mengkliknya menutup dropdown dan membuka modal pengaturan, dan saat modal ditutup fokus kembali ke tombol gear. Caranya: fokus dipindah ke tombol gear lebih dulu, baru `openSettings()` dipanggil, sehingga elemen aktif yang disimpan `Modal.svelte` (lihat overview 2.3) adalah tombol gear, bukan item menu yang segera dihapus dari DOM.

## Path yang dimiliki

HANYA berkas ini yang boleh disentuh:

- `frontend/src/modules/topbar/SettingsButton.svelte`

Jangan menyentuh: `frontend/src/App.svelte`, `frontend/src/main.ts`, `frontend/src/lib/registry.ts`, apa pun di `frontend/src/modules/settings/`, berkas topbar lain, maupun `overview.md`. Jangan menyambungkan modal ke `App.svelte` (itu langkah Wiring terpisah).

## Kontrak

Dipakai (didefinisikan di overview 2.2 dan 2.5, jangan diubah):

- `import { openSettings } from '../settings';` (barrel). Signature: `openSettings(pageId?: string): void`. Dipanggil tanpa argumen.
- Aturan konvensi overview 2.6: topbar boleh mengimpor settings; settings TIDAK boleh mengimpor topbar. Berkas ini tidak boleh membuat impor siklus.

Disediakan: tidak ada ekspor baru. Perilaku baru hanya item menu "Semua pengaturan…".

Tipe lokal yang sudah ada dan dipakai apa adanya: `type Item = { label: string; hint?: string; run: () => void }`.

## Langkah

Semua edit di `frontend/src/modules/topbar/SettingsButton.svelte`. Baca berkas dulu. Jangan ubah markup, gaya, atau logika menu lain selain tiga titik berikut.

1. **Impor.** Di blok `<script lang="ts">`, tambahkan satu baris setelah impor `errText`:
   ```ts
   import { openSettings } from '../settings';
   ```
2. **Ref tombol gear.** Setelah deklarasi `let root ...`, tambahkan:
   ```ts
   let gearBtn: HTMLButtonElement | undefined = $state();
   ```
   Pada elemen `<button type="button" class="gear" ...>`, tambahkan atribut `bind:this={gearBtn}` tanpa mengubah atribut lain.
3. **Grup baru di `groups`.** Pada array `$derived<Item[][]>`, tepat setelah grup `[{ label: 'Susun grid', run: tidyGrid }],` tambahkan grup keempat:
   ```ts
   [{
     label: 'Semua pengaturan…',
     run: () => {
       gearBtn?.focus();
       openSettings();
     },
   }],
   ```
   Separator otomatis muncul lewat `{#if gi > 0}` yang sudah ada. Fungsi `act()` yang ada menutup dropdown (`open = false`) setelah `run()` sukses; jangan diubah. Urutan wajib: `gearBtn?.focus()` sebelum `openSettings()`.
4. **Verifikasi.** Jalankan `cd frontend && npm run build` (harus lulus), lalu `npm run check` bila tersedia (tidak boleh menambah error baru). Periksa `git diff` hanya menampilkan perubahan pada berkas ini dan hanya tiga titik di atas. Pastikan tidak ada impor dari `settings` yang mengimpor balik `topbar` (tidak ada, karena berkas settings bukan milik fitur ini).

## Perubahan

- Diubah: `frontend/src/modules/topbar/SettingsButton.svelte` (satu impor, satu variabel `gearBtn` plus `bind:this`, satu grup baru di `groups`).
- Dibuat: tidak ada.

## Wiring

Fitur ini tidak mengekspos apa pun baru. Untuk langkah Wiring (overview bagian 5, butir 3 dan 5), yang perlu diverifikasi di sana: `SettingsButton.svelte` memanggil `openSettings()` dari `'../settings'` lewat grup "Semua pengaturan…" dan menfokuskan tombol gear lebih dulu; modal baru tampil setelah `<SettingsModal />` dipasang di `App.svelte` oleh langkah Wiring (JANGAN dikerjakan di plan ini).

## Selesai bila

- Dropdown gear menampilkan grup baru berisi "Semua pengaturan…" dengan separator di atasnya, setelah "Susun grid".
- Mengklik item memanggil `gearBtn?.focus()` lalu `openSettings()` dan menutup dropdown.
- Menu lain (zoom, sidebar, susun grid), markup, gaya, dan navigasi keyboard tidak berubah.
- Hanya `SettingsButton.svelte` yang berubah; tidak ada impor siklus settings -> topbar.
- Guard: `cd frontend && npm run build` sukses (dan `npm run check` tanpa error baru bila tersedia). Tanpa unit test. Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
