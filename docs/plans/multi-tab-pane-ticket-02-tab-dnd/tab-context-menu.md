# Plan: Menu konteks tab

## Metadata

- Tiket: `docs/tickets/multi-tab-pane-ticket-02-tab-dnd.md` (multi-tab-pane-ticket-02-tab-dnd)
- Fitur: `tab-context-menu` (Menu konteks tab)
- ownedPaths: `frontend/src/modules/layout/TabContextMenu.svelte`
- Depends on foundation: ya (`00-foundation.md`: stub `store.moveTab` harus sudah ada agar type-check lulus; implementasi penuhnya milik fitur `move-tab-model`, tidak perlu ditunggu)
- Prasyarat: tiket 01 sudah ter-merge (`Leaf.tabs`, `findLeaf`, store `layout`).

## Tujuan

Membuat komponen `TabContextMenu.svelte`: menu klik kanan untuk satu tab dengan item "Pisahkan ke pane baru", "Tutup tab", dan (opsional) "Minimize". Dapat dioperasikan dengan keyboard, ber-ARIA menu, di-clamp ke viewport, mengikuti tema terang/gelap, dan ditutup lewat pola yang sama dengan `PaneMenu.svelte`. Komponen tidak terlihat di aplikasi sampai langkah wiring menyambungkannya.

## Path yang dimiliki

HANYA ini yang boleh dibuat/diubah:

- `frontend/src/modules/layout/TabContextMenu.svelte` (berkas baru)

Jangan menyentuh: `PaneHeader.svelte`, `LayoutArea.svelte` (milik wiring), `model.ts`, `lib/stores/layout.ts`, `dnd.ts`, `DropOverlay.svelte` (fitur lain), `PaneMenu.svelte` (hanya dibaca sebagai acuan gaya), `panes/hosts.ts`. Jangan menambah unit test/vitest. Jangan mengimpor `PaneHeader`.

## Kontrak

Dipakai (lihat `overview.md` bagian 2.1, 2.2, 2.5):

- Store `layout` dari `frontend/src/lib/stores/layout.ts` (impor `import { layout } from '../../lib/stores/layout';`), method `layout.moveTab(fromPaneId, tabId, targetPaneId, zone, index?)` (stub dari foundation, implementasi dari `move-tab-model`).
- `findLeaf(doc, paneId)` dari `./model` (kontrak tiket 01) untuk membaca `leaf.tabs.length`. Bila nama aktual berbeda tipis, ikuti kode aktual dan catat di laporan.
- `svelte/store` `get` atau auto-subscription `$layout`.

Disediakan (props persis sesuai overview 2.5):

```ts
interface Props {
  paneId: string;
  tabId: string;
  x: number;
  y: number;
  onClose: () => void;          // tutup menu; dipanggil setelah item dipilih, klik luar, Escape, scroll, blur
  onCloseTab: () => void;       // wajib; jalur tutup tab yang sama dengan tombol tutup di PaneHeader
  onMinimizeTab?: () => void;   // opsional; item "Minimize" hanya dirender bila ada
}
```

Komponen tidak menduplikasi logika tutup/minimize dan tidak mengimpor registry sesi terminal; ia hanya memanggil `onCloseTab`/`onMinimizeTab`/`layout.moveTab`.

## Langkah

1. Baca `frontend/src/modules/layout/PaneMenu.svelte` (jangan diubah) untuk pola `place()`, `items()`, `run()`, `onMount` dengan listener, dan blok `<style>`. Baca juga `frontend/src/modules/layout/model.ts` untuk memastikan `findLeaf` ada dan signature-nya.
2. Buat `frontend/src/modules/layout/TabContextMenu.svelte` dengan `<script lang="ts">`:
   - Impor: `onMount` dari `svelte`; `layout` dari `../../lib/stores/layout`; `findLeaf` dari `./model`.
   - `interface Props` sesuai Kontrak; `let { paneId, tabId, x, y, onClose, onCloseTab, onMinimizeTab }: Props = $props();`
   - State: `let root: HTMLDivElement | undefined = $state(); let pos = $state({ left: x, top: y });` (mulai di titik klik agar tidak berkedip; di-clamp setelah mount).
   - `const canSplit = $derived((findLeaf($layout, paneId)?.tabs.length ?? 0) > 1);` (reaktif lewat `$layout`; impor `layout` sebagai store).
   - `place()`: bila `root` tidak ada return; ukur `root.offsetWidth/offsetHeight`; `left = Math.max(4, Math.min(x, window.innerWidth - w - 4))`; `top = Math.max(4, Math.min(y, window.innerHeight - h - 4))`; set `pos`.
   - `items()`: `Array.from(root.querySelectorAll<HTMLButtonElement>('button[role="menuitem"]:not(:disabled)'))` (item disabled dilewati navigasi panah dan fokus awal).
   - `run(fn)`: panggil `fn()` lalu `onClose()`.
   - Handler item:
     - `split()` -> `run(() => layout.moveTab(paneId, tabId, paneId, 'right'))`
     - `closeTab()` -> `run(onCloseTab)`
     - `minimize()` -> `run(() => onMinimizeTab?.())`
   - `onMount`: panggil `place()`, lalu `items()[0]?.focus()` (item aktif pertama). Pasang listener di `window` (fase capture, pola PaneMenu):
     - `pointerdown`: bila `root?.contains(e.target as Node)` abaikan; selain itu `onClose()`.
     - `keydown`: `Escape` -> `preventDefault`, `stopPropagation`, `onClose()`; `ArrowDown`/`ArrowUp` -> `preventDefault`, pindah fokus sirkular di antara `items()` (indeks dari `document.activeElement`; bila belum ada fokus, ArrowDown ke pertama, ArrowUp ke terakhir). Enter ditangani default `<button>` (jangan tangani manual, hindari klik ganda).
     - `scroll` dengan opsi `true` (capture) -> `onClose()`.
     - `blur` pada `window` -> `onClose()`.
     - Cleanup: lepas semua listener yang sama (opsi capture sama persis).
   - Tambahan: `oncontextmenu={(e) => e.preventDefault()}` pada root menu agar klik kanan di dalam menu tidak membuka menu native.
3. Markup:
   ```svelte
   <div class="tab-menu" role="menu" aria-label="Menu tab" bind:this={root}
        style="left:{pos.left}px;top:{pos.top}px">
     <button type="button" role="menuitem" disabled={!canSplit} onclick={split}>Pisahkan ke pane baru</button>
     <button type="button" role="menuitem" onclick={closeTab}>Tutup tab</button>
     {#if onMinimizeTab}
       <div class="sep" role="separator"></div>
       <button type="button" role="menuitem" onclick={minimize}>Minimize</button>
     {/if}
   </div>
   ```
   Tambahkan `aria-disabled={!canSplit}` pada item Split. Catatan: `disabled` pada button otomatis tidak bisa difokus/diklik; jika `canSplit` berubah saat menu terbuka, fokus bisa hilang, itu dapat diterima.
4. `<style>` (salin pola `PaneMenu.svelte` dengan kelas `.tab-menu`, tanpa mengubah PaneMenu): `position: fixed; z-index: 9000; min-width: 168px; padding: 4px; background: var(--bg-elev, #222); border: 1px solid var(--border, #444); border-radius: 6px; box-shadow: 0 4px 14px rgba(0,0,0,0.35); color: var(--fg, #ddd); font-size: 14px; display: flex; flex-direction: column;`. Tombol: `background:none; border:0; color:inherit; font:inherit; text-align:left; padding:6px 10px; border-radius:4px; cursor:pointer;`. `button:hover:not(:disabled), button:focus-visible { background: var(--bg-hover, #333); outline: none; }`. `button:disabled { opacity: 0.45; cursor: default; }`. `.sep { height:1px; margin:4px 0; background: var(--border, #444); }`. Hanya pakai variabel tema yang sama dengan PaneMenu (otomatis terang/gelap); jangan hardcode warna di luar fallback.
5. Verifikasi: dari `frontend`, `npm run check 2>&1 | grep TabContextMenu` tidak menampilkan error/warning (termasuk a11y svelte-check). Lalu pastikan `npm run build` tidak gagal karena berkas ini (komponen tidak diimpor di mana pun sampai wiring, jadi build penuh mungkin tidak menyentuhnya; `svelte-check` adalah verifikasi utama). Kegagalan type di luar path ini dilaporkan, bukan diperbaiki.

## Perubahan

- Dibuat: `frontend/src/modules/layout/TabContextMenu.svelte`
- Tidak ada berkas lain yang diubah.

## Wiring

Yang diekspos untuk langkah wiring (`tab-dnd-wiring`, JANGAN disambungkan di fitur ini):

- Komponen default-export `TabContextMenu` dengan props `paneId, tabId, x, y, onClose, onCloseTab, onMinimizeTab?`.
- Penyambung di `PaneHeader.svelte` akan merender `{#if ctxMenu}<TabContextMenu paneId={pane.id} tabId={ctxMenu.tabId} x={ctxMenu.x} y={ctxMenu.y} onClose={() => (ctxMenu = null)} onCloseTab={() => closeTabAction(tab)} onMinimizeTab={isTermTab(tab) ? () => minimizeTab(tab) : undefined} />{/if}` dan menetapkan `ctxMenu` dari `oncontextmenu` tab (lihat overview bagian 5).
- Komponen memanggil sendiri `layout.moveTab(paneId, tabId, paneId, 'right')` untuk "Pisahkan"; tidak perlu wiring tambahan untuk itu.

## Selesai bila

- `TabContextMenu.svelte` ada, menerima props sesuai Kontrak, dan tampil di (x, y) yang di-clamp ke viewport.
- "Pisahkan ke pane baru" memanggil `layout.moveTab(paneId, tabId, paneId, 'right')` lalu `onClose()`; disabled bila pane hanya punya satu tab.
- "Tutup tab" memanggil `onCloseTab` lalu `onClose`; "Minimize" hanya muncul bila `onMinimizeTab` ada, memanggilnya lalu `onClose`.
- Menu tertutup pada klik luar, Escape, scroll, blur, dan setelah memilih item.
- `role="menu"`/`role="menuitem"`, fokus awal pada item aktif pertama, panah atas/bawah melewati item disabled, Enter mengaktifkan; tema terang/gelap lewat variabel yang sama dengan PaneMenu.
- Guard: `npm run check` tanpa error baru pada `TabContextMenu.svelte` dan build path fitur ini sukses; tanpa unit test; hanya satu berkas dibuat dan tidak ada berkas lain yang disentuh.
