# Plan Foundation: shell-frame (rangka shell, registri, helper bersama)

Tiket: `docs/tickets/shikigami-v2-ui-ticket-04-shell-sidebars.md` (fitur `shell-frame`)
Kontrak: `overview.md` bagian 2 (jangan diulang di sini; ikuti persis).
ownedPaths: `frontend/src/modules/shell/` (kecuali `register.ts` dan penghapusan `SidebarArea`/`StatusBar` v1, milik wiring) dan `frontend/src/lib/stores/shell.ts`.
Dilarang menyentuh: path fitur lain, `App.svelte`, `main.ts`, `lib/stores/index.ts`, `styles/theme.css`, modul `workspace/status/files/openfolder/appmenu/zoom/layout/tabs`. `SidebarArea.svelte` dan `StatusBar.svelte` v1 yang ada di `modules/shell/` JANGAN diubah/dihapus di plan ini (masih dipakai `App.svelte` sampai wiring).

## Prasyarat (hanya baca)

1. Pastikan ada: `frontend/src/lib/stores/layout.ts` (ekspor `layout`), `frontend/src/modules/layout/model.ts` (ekspor `findLeafBy`, `leaves`, tipe `Leaf`), `frontend/src/modules/appmenu/index.ts` (`onMenuEvent`, `emitMenuEvent`). Bila tidak ada, hentikan dan laporkan blocker.
2. Baca `lib/registry.ts` (pola registri), `lib/stores/layout.ts`, `modules/layout/model.ts` untuk nama ekspor sebenarnya, dan `modules/appmenu/types.ts` (nama event).

## Langkah

1. **`lib/stores/shell.ts`.** Implementasikan persis kontrak overview 2.2: dua `writable`, konstanta `LEFT_MIN/MAX`, `RIGHT_MIN/MAX`, `LEFT_DEFAULT=260`, `RIGHT_DEFAULT=280`, fungsi `clamp(n,min,max)` (non-finite -> default), `toggleLeft/Right` (flip `visible`), `setLeftWidth/RightWidth` (clamp, simpan bilangan bulat), `setRightTab(id)`. `setLeftWidth(9999)` = `LEFT_MAX`, `setLeftWidth(0)` = `LEFT_MIN`.
2. **`modules/shell/registry.ts`.** `writable<Record<string,RightTabSpec>>` dan `writable<Record<string,TopbarItemSpec>>`; `registerRightTab`/`registerTopbarItem` menimpa per id; `rightTabs()` derived terurut `(order ?? 0)` lalu `id.localeCompare`; `topbarItems(align)` memfilter `align` lalu urut sama. Ekspor tipe spec.
3. **`modules/shell/helpers.ts`.** Implementasikan semua fungsi overview 2.5. Detail: baca layout dengan `get(layout)` dan `findLeafBy`; `sessionTitle`: pane -> `leaf.title`; selain itu `get(providers).find(id===providerId)?.name ?? providerId`; sesi tak ada -> `sessionId.slice(0,6)`. `focusOrOpenSession` mengembalikan `false` bila sesi tidak ada di `get(sessions)`. `createAndOpenSession` memanggil `selectProject(projectId)` bila diberikan, lalu `createSession(providerId)`, lalu `focusOrOpenSession`. `openFolder`/`zoom` memakai `emitMenuEvent`. Impor `layout` dari `../../lib/stores/layout`, store lain dari `../../lib/stores`.
4. **`modules/shell/shell.css`.** Definisikan di `:root`: `--topbar-h: 38px`, `--topbar-inset-left: 0px`, `--left-w: 260px`, `--right-w: 280px` (nilai awal; nilai efektif ditimpa inline pada root `.shell-frame`). Gaya dasar `.shell-frame`. Tidak mengubah `styles/theme.css`.
5. **`modules/shell/ShellFrame.svelte`.** Props snippet `topbar/left/center/right` (overview 2.7). Impor `./shell.css`. Elemen root `.shell-frame` (`height:100%`, `display:grid`, `grid-template-rows: var(--topbar-h) 1fr`, `min-width:0`, `min-height:0`). Baris isi: `grid-template-columns: var(--left-w) minmax(0,1fr) var(--right-w)`. Nilai inline `style="--left-w:{$leftSidebar.visible ? width : 0}px; --right-w: ..."`. Setiap sidebar dibungkus `<aside>` `overflow:hidden; min-width:0` plus `Resizer` di tepi dalam. Isi sidebar tetap ter-mount saat tersembunyi. Area tengah `min-width:0; min-height:0; overflow:hidden`. Panggil `installShellShortcuts()` di `onMount` dan lepas di cleanup. Pada jendela 900px dengan kedua sidebar terbuka area tengah minimal tetap terpakai (`minmax(0,1fr)`), tanpa overflow horizontal.
6. **`modules/shell/Resizer.svelte`.** Props `{side:'left'|'right'}`. Lebar pegangan 4px, kursor `col-resize`, absolute di sisi dalam. `pointerdown` -> `setPointerCapture`; `pointermove` menghitung lebar dari `clientX` relatif tepi jendela (kiri: `clientX`; kanan: `innerWidth - clientX`), di-clamp memakai konstanta min/max, ditulis ke CSS variable `--left-w`/`--right-w` pada elemen `.shell-frame` terdekat lewat `requestAnimationFrame` (satu rAF tertunda, tanpa memicu store tiap gerakan); `pointerup` memanggil `setLeftWidth`/`setRightWidth` dengan nilai akhir dan melepas capture. `dblclick` memanggil setter dengan `LEFT_DEFAULT`/`RIGHT_DEFAULT`. Nonaktif (tersembunyi) bila sidebar tidak terlihat.
7. **`modules/shell/RightPanel.svelte`.** Baca `rightTabs()` dan `rightSidebar`. Aktif = tab dengan `id === activeTab`, jatuh ke tab pertama bila tidak ada. Satu tab -> header judul `FILE` (huruf kapital, `title` tab diabaikan untuk label; bila id bukan `files`, pakai `title.toUpperCase()`), tanpa bar tab; lebih dari satu -> bar tab (klik memanggil `setRightTab`). Isi: `{#key active.id}` merender `<active.component />` (`<svelte:component>` tidak dipakai di Svelte 5: gunakan `{@const C = active.component}<C />`). Tanpa tab terdaftar: placeholder kosong.
8. **`modules/shell/shortcuts.ts`.** `installShellShortcuts()` persis overview 2.6 (event menu + keydown capture + dedupe 250 ms per sisi, `event.code==='KeyB'`, pengecualian input/textarea kecuali `.xterm-helper-textarea`).
9. **`modules/shell/index.ts`.** Re-export sesuai overview 2.3: registri, tipe, `ShellFrame`, `RightPanel`, `export * from './helpers'`. Tanpa efek samping dan tanpa mengimpor `register.ts`. `SidebarArea`/`StatusBar` v1 tidak diekspor.
10. **Guard.** Jalankan `cd frontend && npm run build`, lalu `make build` dari root. Pastikan tidak ada galat tipe di berkas yang dimiliki. `ShellFrame` belum dipakai `App.svelte` (wiring), jadi tampilan aplikasi belum berubah; ini wajar.

## Selesai bila

- Semua simbol kontrak 2.2, 2.3, 2.5, 2.6, 2.7 ada dan diekspor sesuai overview.
- `setLeftWidth(9999)`/`setLeftWidth(0)` ke `LEFT_MAX`/`LEFT_MIN`; registri idempoten dan terurut.
- `ShellFrame` merender empat slot; menyembunyikan sidebar tidak meng-unmount isinya.
- `make build` lulus. Commit/merge sesuai alur `implement-feature` sebelum fitur paralel dimulai.
