# Plan: Foundation: rangka shell dan registri panel kanan

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-04-shell-sidebars.md`
- Fitur: `shell-frame` (Foundation: rangka shell dan registri panel kanan)
- ownedPaths: `frontend/src/modules/shell/`, `frontend/src/lib/stores/shell.ts`
- Depends on foundation: tidak (fitur ini ADALAH foundation; dikerjakan pertama, sebelum fitur paralel)
- Kontrak lengkap: `docs/plans/shikigami-v2-ui-ticket-04-shell-sidebars/overview.md` bagian 2 (2.2, 2.3, 2.5, 2.6, 2.7). Ikuti persis; jangan mengubah `overview.md`. Rencana ringkas yang sama ada di `00-foundation.md`.

## Tujuan

Menyediakan rangka shell v2 yang dipakai semua fitur paralel dan wiring: state sidebar (`lib/stores/shell.ts`), registri titik ekstensi (`registerRightTab`, `registerTopbarItem`), helper bersama (fokus-atau-buka sesi/file di layout, dll.), `ShellFrame` (grid dengan slot snippet), `Resizer`, `RightPanel`, pintasan Cmd+B / Cmd+Opt+B dengan dedupe, serta token CSS. Hasil harus build sendiri dan tidak mengubah tampilan aplikasi (belum disambungkan ke `App.svelte`).

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `frontend/src/modules/shell/` (file BARU: `registry.ts`, `helpers.ts`, `shell.css`, `ShellFrame.svelte`, `Resizer.svelte`, `RightPanel.svelte`, `shortcuts.ts`, `index.ts`)
- `frontend/src/lib/stores/shell.ts` (BARU)

Dilarang: `modules/shell/register.ts` (milik wiring), `SidebarArea.svelte` dan `StatusBar.svelte` v1 yang sudah ada di `modules/shell/` (jangan diubah, jangan dihapus, jangan diekspor dari `index.ts`; masih dipakai `App.svelte` sampai wiring), `App.svelte`, `main.ts`, `lib/stores/index.ts`, `lib/stores/layout.ts`, `lib/registry.ts`, `styles/theme.css`, `styles/base.css`, `package.json`, `Makefile`, kode Go, modul `workspace`, `status`, `files`, `openfolder`, `appmenu`, `zoom`, `layout`, `tabs`, dan path fitur lain (`topbar`, `sidebar-left`, `sidebar-right`, `palette`, `notifications`).

## Kontrak

Disediakan (bentuk persis di overview 2.2, 2.3, 2.5, 2.6, 2.7):

- `lib/stores/shell.ts`: `SidebarState`, `RightSidebarState`, `leftSidebar`, `rightSidebar`, `LEFT_MIN=200`, `LEFT_MAX=420`, `RIGHT_MIN=220`, `RIGHT_MAX=520`, `LEFT_DEFAULT=260`, `RIGHT_DEFAULT=280`, `toggleLeft`, `toggleRight`, `setLeftWidth`, `setRightWidth`, `setRightTab`.
- `modules/shell/registry.ts`: `RightTabSpec`, `TopbarItemSpec`, `registerRightTab`, `registerTopbarItem`, `rightTabs()`, `topbarItems(align)`.
- `modules/shell/helpers.ts`: `findSessionPane`, `findFilePane`, `sessionTitle`, `focusOrOpenSession`, `openFilePane`, `basename`, `createAndOpenSession`, `openFolder`, `zoom`, `tidyGrid`.
- `modules/shell/shortcuts.ts`: `installShellShortcuts(): () => void`.
- `modules/shell/ShellFrame.svelte` (props snippet `topbar/left/center/right`), `Resizer.svelte` (`{side}`), `RightPanel.svelte` (tanpa props).
- `modules/shell/index.ts`: re-export registri, tipe spec, `ShellFrame`, `RightPanel`, `export * from './helpers'`.

Dipakai (sudah ada, jangan diubah; nama ekspor sudah diverifikasi di repo):

- `lib/stores/layout.ts`: `layout` (`LayoutStore`: `subscribe`, `openPane(leaf: NewLeaf, opts?): string`, `setFocus(paneId)`, `tidyGrid()`). Impor langsung dari `../../lib/stores/layout`.
- `modules/layout/model.ts`: tipe `Leaf` (`{type:'pane', id, kind, sessionId?, projectId?, provider?, filePath?, title, workspaceId?}`), `NewLeaf`, fungsi `findLeafBy(doc, pred): Leaf | null`.
- `lib/stores` (`index.ts`): `sessions`, `providers`, `activeWorkspaceId`, `selectProject`, `createSession`, tipe `Session`.
- `modules/appmenu` (`index.ts`): `onMenuEvent(name, fn): unsubscribe`, `emitMenuEvent(name, payload?)`. Nama event: `menu:zoom` (payload `{action:'in'|'out'|'reset'}`), `menu:open-folder`, `menu:toggle-sidebar-left`, `menu:toggle-sidebar-right`.
- Pola registri: `lib/registry.ts` (writable map + derived terurut `(order ?? 0)` lalu `id.localeCompare`).

## Langkah

Prasyarat: pastikan `frontend/src/lib/stores/layout.ts`, `frontend/src/modules/layout/model.ts`, `frontend/src/modules/appmenu/index.ts` ada (sudah ada saat plan ditulis). Bila hilang, hentikan dan laporkan blocker. Bila `node_modules` kosong, `cd frontend && npm install` dulu.

1. **`frontend/src/lib/stores/shell.ts`.**
   - `import { writable } from 'svelte/store'`. Ekspor interface `SidebarState {visible:boolean; width:number}`, `RightSidebarState extends SidebarState {activeTab:string}`.
   - Konstanta ekspor: `LEFT_MIN=200, LEFT_MAX=420, RIGHT_MIN=220, RIGHT_MAX=520, LEFT_DEFAULT=260, RIGHT_DEFAULT=280`.
   - `leftSidebar = writable<SidebarState>({visible:true,width:LEFT_DEFAULT})`; `rightSidebar = writable<RightSidebarState>({visible:true,width:RIGHT_DEFAULT,activeTab:'files'})`.
   - Fungsi privat `clamp(n:number,min:number,max:number,def:number):number`: non-finite -> `def`; selain itu `Math.round(Math.min(max, Math.max(min, n)))`.
   - `toggleLeft()`/`toggleRight()`: `update` membalik `visible`, lebar tidak berubah.
   - `setLeftWidth(px)`/`setRightWidth(px)`: `update` dengan lebar ter-clamp (bilangan bulat; NaN/Infinity -> default). `setLeftWidth(9999)` = 420, `setLeftWidth(0)` = 200.
   - `setRightTab(id)`: `update` mengganti `activeTab`.
   - Tanpa persist ke daemon (tiket 06 yang menambah).

2. **`modules/shell/registry.ts`.**
   - Ekspor `RightTabSpec`, `TopbarItemSpec` persis overview 2.3 (`import type { Component } from 'svelte'`).
   - Dua `writable<Record<string,...>>`. `registerRightTab(spec)`/`registerTopbarItem(spec)`: `update((m) => ({...m, [spec.id]: spec}))` (timpa per id, idempoten).
   - `rightTabs(): Readable<RightTabSpec[]>`: `derived(store, m => Object.values(m).sort(cmp))`. `topbarItems(align)`: `derived`, filter `t.align === align`, lalu `cmp`. `cmp = (a,b) => (a.order ?? 0) - (b.order ?? 0) || a.id.localeCompare(b.id)`.

3. **`modules/shell/helpers.ts`.** Impor: `get` dari `svelte/store`; `layout` dari `../../lib/stores/layout`; `findLeafBy`, tipe `Leaf` dari `../layout/model`; `sessions`, `providers`, `activeWorkspaceId`, `selectProject`, `createSession`, tipe `Session` dari `../../lib/stores`; `emitMenuEvent` dari `../appmenu`.
   - `findSessionPane(sessionId): Leaf | null` = `findLeafBy(get(layout), l => l.kind==='terminal' && l.sessionId===sessionId)`.
   - `findFilePane(projectId, filePath): Leaf | null` = `kind==='file' && l.filePath===filePath && l.projectId===projectId`.
   - `basename(path)`: potong trailing slash, ambil segmen setelah `/` atau `\` terakhir; fallback `path`.
   - `sessionTitle(sessionId): string`: pane ada -> `leaf.title` (jika tidak kosong); selain itu sesi ada di `get(sessions)` -> `get(providers).find(p=>p.id===s.providerId)?.name ?? s.providerId`; sesi tak ada -> `sessionId.slice(0,6)`.
   - `focusOrOpenSession(sessionId): boolean`: sesi tak ada di `get(sessions)` -> `false`. Pane ada -> `layout.setFocus(leaf.id)`; belum -> `layout.openPane({kind:'terminal', sessionId, projectId: s.projectId, provider: s.providerId, title: sessionTitle(sessionId)})`. Kembalikan `true`. (`openPane` idempoten.)
   - `openFilePane({projectId,filePath,workspaceId,title}): string`: pane ada -> `setFocus` dan kembalikan `leaf.id`; selain itu `layout.openPane({kind:'file', projectId, filePath, workspaceId: workspaceId ?? get(activeWorkspaceId) ?? undefined, title: title ?? basename(filePath)})`.
   - `createAndOpenSession(providerId, projectId?): Promise<Session>`: bila `projectId` -> `selectProject(projectId)`; `const s = await createSession(providerId)`; `focusOrOpenSession(s.id)`; kembalikan `s`. Tidak menangkap galat (pemanggil menampilkan pesan). Frontend tidak menyusun argumen provider.
   - `openFolder()` = `emitMenuEvent('menu:open-folder')`; `zoom(action)` = `emitMenuEvent('menu:zoom', {action})`; `tidyGrid()` = `layout.tidyGrid()`.
   - Tanpa timer/polling; tidak memanggil `killSession`. Periksa tipe `activeWorkspaceId` (store) dan field `Session` di `lib/stores/types.ts` sebelum menulis; sesuaikan bila nama berbeda.

4. **`modules/shell/shell.css`.** `:root { --topbar-h: 38px; --topbar-inset-left: 0px; --left-w: 260px; --right-w: 280px; }` (nilai efektif ditimpa inline pada `.shell-frame`). Gaya `.shell-frame` (grid, lihat langkah 5), `.shell-frame .topbar-slot`, `.sidebar`, `.sidebar-left`, `.sidebar-right`, `.center-slot`. Hanya variabel tema yang ada (`--bg --bg-elev --bg-hover --fg --fg-muted --border --accent`), tanpa warna hardcode. Jangan ubah `styles/theme.css`.

5. **`modules/shell/ShellFrame.svelte`.**
   - `<script lang="ts">`: `import './shell.css'`, `onMount` dari `svelte`, `leftSidebar`/`rightSidebar` dari `../../lib/stores/shell`, `Resizer`, `installShellShortcuts`, `type Snippet`. `let { topbar, left, center, right }: { topbar: Snippet; left: Snippet; center: Snippet; right: Snippet } = $props();`. `onMount(() => installShellShortcuts());` (nilai kembalian = cleanup).
   - Markup: `<div class="shell-frame" style="--left-w:{$leftSidebar.visible ? $leftSidebar.width : 0}px; --right-w:{$rightSidebar.visible ? $rightSidebar.width : 0}px">` berisi: `<header class="topbar-slot">{@render topbar()}</header>`; `<div class="body">` dengan `<aside class="sidebar sidebar-left">{@render left()}<Resizer side="left" /></aside>`, `<main class="center-slot">{@render center()}</main>`, `<aside class="sidebar sidebar-right">{@render right()}<Resizer side="right" /></aside>`.
   - CSS: `.shell-frame {height:100%; display:grid; grid-template-rows: var(--topbar-h) minmax(0,1fr); min-width:0; min-height:0; overflow:hidden}`; `.body {display:grid; grid-template-columns: var(--left-w) minmax(0,1fr) var(--right-w); min-width:0; min-height:0}`; `.sidebar {position:relative; overflow:hidden; min-width:0; min-height:0}` dengan border dalam sisi dalam; `.center-slot {min-width:0; min-height:0; overflow:hidden}`. Konten sidebar tetap ter-mount saat lebar 0 (jangan `{#if}` pada slot). Pada jendela 900px dengan kedua sidebar terbuka tidak boleh ada overflow horizontal.
   - `ShellFrame` tidak mengimpor modul fitur apa pun.

6. **`modules/shell/Resizer.svelte`.** Props `{side:'left'|'right'}` (`$props`). Elemen `div.resizer` absolute, lebar 4px, tepi dalam (kiri: `right:0`; kanan: `left:0`), `cursor:col-resize`, `touch-action:none`, `z-index` di atas isi, hover/aktif memakai `--accent`. Disembunyikan (`{#if visible}` dari store sisi terkait) bila sidebar tidak terlihat.
   - `pointerdown`: `setPointerCapture`, simpan `dragging=true`, ambil root `el.closest('.shell-frame') as HTMLElement`.
   - `pointermove` (saat dragging): `raw = side==='left' ? e.clientX : window.innerWidth - e.clientX`; clamp ke min/max sisi (konstanta dari store, bulatkan); simpan `pending`; jadwalkan SATU `requestAnimationFrame` bila belum ada, yang menulis `root.style.setProperty('--left-w'|'--right-w', pending+'px')`. Tidak memanggil store tiap gerakan.
   - `pointerup`/`pointercancel`: `releasePointerCapture`, batalkan rAF tertunda, panggil `setLeftWidth`/`setRightWidth(pending)` (hanya bila ada nilai), `dragging=false`. Setelah store diperbarui, style inline `ShellFrame` menjadi sumber kebenaran lagi.
   - `ondblclick`: `setLeftWidth(LEFT_DEFAULT)` / `setRightWidth(RIGHT_DEFAULT)`.
   - Selama drag set `document.body.style.userSelect='none'`, kembalikan saat selesai.

7. **`modules/shell/RightPanel.svelte`.**
   - Baca `rightTabs()` (`const tabs = rightTabs()`; pakai `$tabs`) dan `$rightSidebar.activeTab`. `active = $derived($tabs.find(t=>t.id===activeTab) ?? $tabs[0])`.
   - Tanpa tab: placeholder kosong (`div.empty`, teks "Belum ada panel").
   - Satu tab: header teks judul saja, tanpa bar tab; label: `active.id==='files' ? 'FILE' : active.title.toUpperCase()`. Lebih dari satu: bar tab (tombol per tab, kelas aktif bergaya aksen; klik -> `setRightTab(t.id)`; label `t.title`).
   - Isi: `{#key active.id}{@const C = active.component}<C />{/key}` (jangan `<svelte:component>`). Container isi `flex:1; min-height:0; overflow:auto`. Root `display:flex; flex-direction:column; height:100%`.

8. **`modules/shell/shortcuts.ts`.** `installShellShortcuts(): () => void` persis overview 2.6.
   - `onMenuEvent('menu:toggle-sidebar-left', () => fire('left'))` dan `...-right` -> `fire('right')`; simpan unsubscribe.
   - `last = {left:-Infinity, right:-Infinity}`; `fire(side)`: `now = performance.now()`; bila `now - last[side] < 250` return; `last[side]=now`; panggil `toggleLeft()`/`toggleRight()`.
   - `window.addEventListener('keydown', handler, true)`. Handler: bila bukan `e.metaKey` atau `e.code !== 'KeyB'` -> return. `e.shiftKey` -> return. `alt = e.altKey`. Target: `t = e.target as HTMLElement`; bila `t` adalah `INPUT`/`TEXTAREA`/`SELECT` dan bukan `t.classList.contains('xterm-helper-textarea')` -> return (abaikan). Lalu `e.preventDefault(); e.stopPropagation(); fire(alt ? 'right' : 'left')`. Gunakan `event.code`, bukan `event.key`.
   - Cleanup: lepas listener keydown dan semua unsubscribe menu. Jangan menangani Cmd+K, Cmd+D, Cmd+Shift+D, Cmd+O, atau zoom.

9. **`modules/shell/index.ts`.** Tanpa efek samping:
   ```ts
   export { registerRightTab, registerTopbarItem, rightTabs, topbarItems } from './registry';
   export type { RightTabSpec, TopbarItemSpec } from './registry';
   export { default as ShellFrame } from './ShellFrame.svelte';
   export { default as RightPanel } from './RightPanel.svelte';
   export * from './helpers';
   ```
   Jangan mengimpor `register.ts`, `SidebarArea`, atau `StatusBar`. Catatan: `main.ts` memuat `modules/*/index.ts` lewat glob eager, jadi `index.ts` ini baru dimuat saat start; pastikan benar-benar bebas efek samping dan tidak membentuk siklus impor (helpers hanya mengimpor `lib/stores`, `modules/layout/model`, `modules/appmenu`).

10. **Guard.** `cd frontend && npm run build` (periksa tidak ada galat tipe/svelte-check di berkas milik fitur ini), lalu `make build` dari root repo. `ShellFrame` belum dipakai `App.svelte`, jadi tampilan aplikasi tidak berubah; itu wajar.

## Perubahan

Dibuat:

- `frontend/src/lib/stores/shell.ts`
- `frontend/src/modules/shell/registry.ts`
- `frontend/src/modules/shell/helpers.ts`
- `frontend/src/modules/shell/shell.css`
- `frontend/src/modules/shell/ShellFrame.svelte`
- `frontend/src/modules/shell/Resizer.svelte`
- `frontend/src/modules/shell/RightPanel.svelte`
- `frontend/src/modules/shell/shortcuts.ts`
- `frontend/src/modules/shell/index.ts`

Diubah: tidak ada. `SidebarArea.svelte` dan `StatusBar.svelte` v1 tidak disentuh.

## Wiring

Yang diekspos untuk disambungkan oleh langkah `shell-wiring` (JANGAN menyambungkannya sendiri; jangan membuat `register.ts`, jangan mengedit `App.svelte`):

- `ShellFrame` (props snippet `topbar`, `left`, `center`, `right`) dan `RightPanel` untuk dipasang di `App.svelte` melalui `modules/shell/index.ts`.
- `registerRightTab`/`registerTopbarItem`/`rightTabs`/`topbarItems` untuk fitur `sidebar-right-files` (tab `files`), `notification-bell` (item `notifications`), dan `Topbar`.
- Helper `modules/shell/helpers.ts` (diekspor via `../shell`) untuk `sidebar-left-agent`, `command-palette`, `notification-bell`, `topbar`, `sidebar-right-files`.
- `lib/stores/shell.ts` (`toggleLeft`, `toggleRight`, state) dipakai langsung oleh `topbar` (SettingsButton) dan `palette`.
- `installShellShortcuts` dipasang otomatis oleh `ShellFrame` saat mount; tidak perlu wiring tambahan.

## Selesai bila

- Semua simbol kontrak 2.2, 2.3, 2.5, 2.6, 2.7 ada dan diekspor sesuai overview.
- `setLeftWidth(9999)` = `LEFT_MAX`, `setLeftWidth(0)` = `LEFT_MIN`; NaN -> default; `setRightWidth` analog.
- `registerRightTab` dan `registerTopbarItem` idempoten per id dan terurut `order` lalu id.
- `ShellFrame` merender empat slot; menyembunyikan sidebar (lebar 0) tidak meng-unmount isinya.
- `Resizer` menyeret mulus (CSS variable via rAF), menghormati min/max, dblclick mengembalikan default.
- Cmd+B / Cmd+Opt+B dan event `menu:toggle-sidebar-left/right` memanggil toggle tepat sekali per aksi (dedupe 250 ms per sisi); input teks non-xterm diabaikan.
- Tidak ada overflow horizontal pada jendela 900px dengan kedua sidebar terbuka.
- Hanya path milik fitur ini yang berubah; `SidebarArea`/`StatusBar` v1 dan `register.ts` tidak disentuh.
- Guard: `cd frontend && npm run build` sukses, dan `make build` sukses. Tanpa unit test.
