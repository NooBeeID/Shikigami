# Plan: Persist dan restore activeRail di frontend

## Metadata

- Tiket: `main-rail-ticket-01-main-rail` (`docs/tickets/main-rail-ticket-01-main-rail.md`)
- Fitur: `rail-state-persist-ui`
- ownedPaths:
  - `frontend/src/modules/persist/restore.ts`
  - `frontend/src/modules/persist/autosave.ts`
- Depends on foundation: tidak (`hasFoundation=false`). Tahap B, paralel dengan `rail-ui`, dikerjakan setelah tahap A (`rail-registry-store`, `rail-state-persist-go`) di-merge. Kontrak tahap A (bagian 2 `overview.md`) boleh dianggap sudah ada.

## Tujuan

Pilihan item rail aktif (`leftSidebar.activeRail`) ikut dipulihkan saat aplikasi dibuka dan ikut tersimpan saat berubah. State UI lama tanpa `activeRail` (atau berisi nilai kosong / bukan string) dimigrasikan menjadi `'agents'`. Nilai string tak dikenal dibiarkan; fallback ke `'agents'` dilakukan oleh `resolveActiveRail` di komponen rail (di luar fitur ini).

## Path yang dimiliki

HANYA dua berkas ini yang boleh disentuh:

- `frontend/src/modules/persist/restore.ts`
- `frontend/src/modules/persist/autosave.ts` (edit hanya bila verifikasi langkah 2 menemukan penyaringan yang menjatuhkan `activeRail`; kondisi saat plan ini ditulis: tidak perlu edit)

Jangan menyentuh `lib/stores/shell.ts`, `modules/shell/*`, `App.svelte`, file Go, `lib/stores/layout.ts`, `layout/*`, `thread/*`, `provider/*`, `app/*`, maupun file wiring lain.

## Kontrak

Dipakai (disediakan tahap A, lihat overview bagian 2.2 dan 2.3):

- `leftSidebar: Writable<SidebarState>` dari `frontend/src/lib/stores/shell.ts`, dengan `SidebarState = { visible: boolean; width: number; activeRail?: string }`.
- Pesan IPC yang sudah ada `workspace.ui.set` (payload `uiPayload()`, `leftSidebar` kini berisi `activeRail`) dan data UI dari daemon yang diteruskan ke `applyUi(ui: unknown)`. Field Go `activeRail,omitempty` disediakan `rail-state-persist-go`.

Disediakan: tidak ada simbol baru. Perilaku yang dijamin:

- `applyUi` menghasilkan `leftSidebar.activeRail` bertipe string non-kosong (`'agents'` bila input tidak valid).
- `uiPayload()` dan `startWatching()` di `autosave.ts` membawa dan memantau `activeRail`.

Tidak ada pesan IPC baru.

## Langkah

1. Edit `frontend/src/modules/persist/restore.ts`, fungsi `applyUi`. Pada pemanggilan `leftSidebar.set({...})` tambahkan field `activeRail` setelah `width`, sehingga menjadi:

   ```ts
   leftSidebar.set({
     visible: typeof l.visible === 'boolean' ? l.visible : true,
     width: clamp(l.width, LEFT_MIN, LEFT_MAX, LEFT_DEFAULT),
     activeRail: typeof l.activeRail === 'string' && l.activeRail ? l.activeRail : 'agents',
   });
   ```

   Aturan: logika `visible` dan `width` tidak diubah sama sekali; nilai string non-kosong yang tak dikenal dibiarkan apa adanya (tidak divalidasi terhadap registry); `rightSidebar.set` tidak diubah. Tidak perlu impor baru (memakai literal `'agents'` sesuai overview 2.3).

2. Verifikasi `frontend/src/modules/persist/autosave.ts` (baca saja dulu). Pastikan tiga hal berikut, semuanya sudah benar pada kode saat ini:
   - `uiPayload()` memakai `leftSidebar: { ...get(leftSidebar) }` (spread utuh, tidak memilih field satu per satu), sehingga `activeRail` ikut.
   - `startWatching()` memuat `leftSidebar.subscribe(scheduleSave)`; setiap `leftSidebar.update/set`, termasuk `setActiveRail`, memicu `scheduleSave`.
   - `flush`/`doWrite` membandingkan `JSON.stringify(uiPayload())` dengan `lastUiJSON`, jadi perubahan `activeRail` saja menghasilkan JSON berbeda dan memicu `workspace.ui.set`.

   Bila ketiganya benar (diharapkan), JANGAN mengedit `autosave.ts`. Hanya bila ada penyaringan field yang menjatuhkan `activeRail` (mis. `uiPayload` diubah jadi memilih `visible`/`width` saja), perbaiki secara minimal agar `activeRail` ikut, tanpa mengubah perilaku lain.

3. Jalankan guard (lihat "Selesai bila") dan perbaiki kegagalan yang berasal dari perubahan di dua berkas ini. Kegagalan yang sudah ada sebelumnya atau di luar ownedPaths hanya dilaporkan, tidak diperbaiki. Bila `SidebarState.activeRail` ternyata belum ada di `shell.ts` (tahap A belum ter-merge), laporkan; jangan mengedit `shell.ts`.

## Perubahan

- Diubah: `frontend/src/modules/persist/restore.ts` (satu field `activeRail` di `leftSidebar.set` pada `applyUi`).
- `frontend/src/modules/persist/autosave.ts`: tidak diubah, kecuali verifikasi langkah 2 menemukan penyaringan yang menjatuhkan `activeRail`. Catat hasil verifikasi di laporan.
- Tidak ada berkas baru.

## Wiring

Tidak ada yang perlu disambungkan. Fitur ini hanya mengubah perilaku `applyUi` yang sudah dipanggil jalur restore yang ada. Konsumen `activeRail` (`Rail.svelte`, `RailPanelHost.svelte`) dibuat `rail-ui`, dan item `agents` didaftarkan oleh `agents-rail-item` pada tahap C; keduanya bukan bagian fitur ini.

## Selesai bila

- Restore state tanpa `activeRail`, string kosong, atau non-string (angka, null, objek) menghasilkan `activeRail === 'agents'`; string non-kosong dipertahankan apa adanya.
- `visible` dan `width` hasil restore tidak berubah dibanding sebelumnya.
- Terverifikasi (dengan membaca kode) bahwa perubahan `activeRail` saja memicu `scheduleSave` dan patch `workspace.ui.set` berisi `leftSidebar.activeRail`; `autosave.ts` hanya diedit bila perlu.
- Hanya `restore.ts` (dan `autosave.ts` bila terpaksa) yang berubah di `git diff` untuk fitur ini.
- Guard: build frontend sukses (`npm run build` di `frontend/`; opsional `npm run check`). Tanpa unit test.
