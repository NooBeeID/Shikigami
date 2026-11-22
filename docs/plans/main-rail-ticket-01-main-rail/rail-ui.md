# Plan: Komponen rail dan grid ShellFrame empat kolom

## Metadata
- Tiket: `docs/tickets/main-rail-ticket-01-main-rail.md`
- Overview: `docs/plans/main-rail-ticket-01-main-rail/overview.md` (kontrak bagian 2.1, 2.2, 2.4)
- Fitur: `rail-ui` (Tahap B, paralel dengan `rail-state-persist-ui`)
- ownedPaths:
  - `frontend/src/modules/shell/Rail.svelte` (baru)
  - `frontend/src/modules/shell/RailPanelHost.svelte` (baru)
  - `frontend/src/modules/shell/ShellFrame.svelte` (ubah)
  - `frontend/src/modules/shell/shell.css` (ubah)
- Depends on foundation: tidak (tidak ada `00-foundation.md`). Namun tahap A (`rail-registry-store`) sudah ter-merge, jadi `registry.ts` dan `lib/stores/shell.ts` sudah menyediakan simbol kontrak.

## Tujuan
Menambah rail ikon 48px di tepi kiri `ShellFrame` dan membuat panel kiri mengikuti item rail aktif. Grid body berubah dari `panel | center | right` menjadi `rail | panel | center | right`. Rail selalu tampil; `toggleLeft` dan Resizer kiri hanya memengaruhi panel. Perilaku panel kanan dan center tidak berubah.

## Path yang dimiliki
HANYA empat berkas di atas yang boleh disentuh. Jangan sentuh `registry.ts`, `lib/stores/shell.ts`, `index.ts`, `register.ts`, `Resizer.svelte`, `shortcuts.ts`, `App.svelte`, `sidebar-left/*`, `persist/*`, `layout/*`, `thread/*`, `app/*`, maupun file Go.

## Kontrak
Dipakai (sudah ada dari tahap A, lihat overview 2.1 dan 2.2; jangan didefinisikan ulang):
- `railItems(): Readable<RailItemSpec[]>` dan tipe `RailItemSpec` dari `./registry`.
- `leftSidebar: Writable<SidebarState>`, `setActiveRail(id: string): void`, `resolveActiveRail(items: { id: string }[], id: string | undefined): string`, `DEFAULT_RAIL` (`'agents'`) dari `../../lib/stores/shell`.
- `Resizer.svelte` (props `{ side: 'left' | 'right' }`), `LayoutHeaderHost.svelte`, `installShellShortcuts` yang sudah ada.

Disediakan:
- `Rail.svelte`: tanpa props. Kolom 48px, tombol per item.
- `RailPanelHost.svelte`: tanpa props. Me-mount hanya komponen item aktif dengan props `{}`.
- `ShellFrame.svelte`: props `{ topbar: Snippet; left?: Snippet; center: Snippet; right: Snippet }`; `left` opsional, fallback `<RailPanelHost />`.
- Kelas CSS di `shell.css`: `.rail`, `.rail-btn`, `.rail-btn.active`.

PENTING: impor langsung dari `./registry` dan `../../lib/stores/shell`, BUKAN dari `./index`, supaya tidak ada siklus impor (`index.ts` nanti mengekspor komponen ini).

## Langkah

### 1. Buat `frontend/src/modules/shell/Rail.svelte`
- `<script lang="ts">`:
  - `import { railItems } from './registry';`
  - `import { leftSidebar, setActiveRail, resolveActiveRail } from '../../lib/stores/shell';`
  - `const items = railItems();` (store, dipakai dengan `$items`).
  - `const active = $derived(resolveActiveRail($items, $leftSidebar.activeRail));` supaya sorotan aktif benar walau state berisi id tak dikenal/tak visible.
- Markup:
  ```svelte
  <div class="rail" role="toolbar" aria-label="Menu utama" aria-orientation="vertical">
    {#each $items as spec (spec.id)}
      {@const Icon = spec.icon}
      <button
        type="button"
        class="rail-btn"
        class:active={spec.id === active}
        aria-label={spec.title}
        title={spec.title}
        aria-current={spec.id === active ? 'true' : undefined}
        onclick={() => setActiveRail(spec.id)}
      >
        <Icon />
      </button>
    {/each}
  </div>
  ```
  Jangan memakai `role="toolbar"` bila menimbulkan peringatan a11y atau perlu penanganan panah; boleh diganti `<div class="rail">` biasa agar sederhana (Tab + Enter/Spasi sudah cukup, `<button>` native menanganinya).
- Tanpa `<style>` blok; gaya ada di `shell.css`.

### 2. Buat `frontend/src/modules/shell/RailPanelHost.svelte`
- `<script lang="ts">`:
  - impor `railItems` dari `./registry`; `leftSidebar`, `setActiveRail`, `resolveActiveRail`, `DEFAULT_RAIL` dari `../../lib/stores/shell`.
  - `const items = railItems();`
  - `const activeId = $derived(resolveActiveRail($items, $leftSidebar.activeRail));`
  - `const Active = $derived($items.find((i) => i.id === activeId)?.component);`
  - Efek reset: bila `$items` sudah berisi item (panjang > 0, hindari reset prematur sebelum registrasi) dan `$leftSidebar.activeRail` ada (string tak kosong) tetapi tidak ada di `$items`, panggil `setActiveRail(DEFAULT_RAIL)`:
    ```ts
    $effect(() => {
      const cur = $leftSidebar.activeRail;
      if ($items.length === 0 || !cur) return;
      if (cur !== DEFAULT_RAIL && !$items.some((i) => i.id === cur)) setActiveRail(DEFAULT_RAIL);
    });
    ```
    Catatan: hanya reset bila hasil resolve berbeda dari nilai tersimpan dan nilai tersimpan bukan `agents`; hindari loop tulis tak berujung (setelah reset, `cur === DEFAULT_RAIL` sehingga efek berhenti).
- Markup: `{#if Active}<Active />{/if}` (props `{}`; `<Active />` tanpa atribut). Pakai `{#key activeId}` di sekitarnya agar komponen di-mount ulang bersih saat ganti item. Hanya panel aktif yang dirender.
- Bila `Active` tidak ada (belum ada item terdaftar), render kosong tanpa error.

### 3. Ubah `frontend/src/modules/shell/ShellFrame.svelte`
- Tambah impor `Rail from './Rail.svelte'` dan `RailPanelHost from './RailPanelHost.svelte'`.
- Props: `let { topbar, left, center, right }: { topbar: Snippet; left?: Snippet; center: Snippet; right: Snippet } = $props();`
- Markup `.body`:
  ```svelte
  <div class="body">
    <nav class="rail-slot" aria-label="Menu utama"><Rail /></nav>
    <aside class="side left" class:hidden={!$leftSidebar.visible}>
      {#if left}{@render left()}{:else}<RailPanelHost />{/if}
      <Resizer side="left" />
    </aside>
    <main class="center"> ... tidak berubah ... </main>
    <aside class="side right"> ... tidak berubah ... </aside>
  </div>
  ```
  Bila `Rail` sudah memakai `role="toolbar"` dengan label sendiri, hindari label ganda: beri `nav` label dan hapus `role`/`aria-label` pada `.rail`, atau sebaliknya (satu saja).
- `style` di `.shell-frame` tetap: `--left-w:{visible ? width : 0}px` (hanya panel) dan `--right-w` tidak berubah.
- Blok `<style>`:
  - `.body { grid-template-columns: 48px var(--left-w) minmax(0, 1fr) var(--right-w); }` (properti lain tetap).
  - `.rail-slot { width: 48px; min-width: 0; min-height: 0; overflow: hidden; background: var(--bg-elev); border-right: 1px solid var(--border); }`
  - `.side.left { border-right: 1px solid var(--border); }` diganti agar tidak ada border ganda saat panel 0px: `.side.left { border-right: 1px solid var(--border); } .side.left.hidden { border-right: 0; }`. Rail sudah punya `border-right`, jadi pada panel tersembunyi hanya satu garis.
  - `.side.right`, `.center`, `.center-body`, `.topbar-slot` tidak diubah.

### 4. Ubah `frontend/src/modules/shell/shell.css`
Tambahkan (di bawah aturan yang ada, pakai token dari `styles/theme.css`: `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`):
```css
:root { --rail-w: 48px; }

.rail {
  width: var(--rail-w);
  height: 100%;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 8px 0;
  overflow-y: auto;
  overflow-x: hidden;
}
.rail-btn {
  position: relative;
  width: 36px;
  height: 36px;
  flex: none;
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--fg-muted);
  cursor: pointer;
}
.rail-btn :global(svg) { width: 20px; height: 20px; }  /* hapus :global bila di shell.css biasa */
.rail-btn:hover { background: var(--bg-hover); color: var(--fg); }
.rail-btn.active { background: var(--bg-hover); color: var(--fg); }
.rail-btn.active::before {
  content: '';
  position: absolute;
  left: -6px;
  top: 8px;
  bottom: 8px;
  width: 3px;
  border-radius: 2px;
  background: var(--accent);
}
.rail-btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
```
Karena `shell.css` adalah CSS global biasa, tulis `.rail-btn svg { width: 20px; height: 20px; }` (tanpa `:global`). Tetapkan `.shell-frame .body` tidak butuh perubahan di CSS ini. Jangan mengubah `:root` `--left-w` yang ada. Rail tidak punya Resizer dan lebarnya tidak berubah.

### 5. Verifikasi build
Jalankan `npm run build` di `frontend/` (opsional `npm run check`; kegagalan yang sudah ada sebelum perubahan dilaporkan saja).

## Perubahan
- Baru: `frontend/src/modules/shell/Rail.svelte`
- Baru: `frontend/src/modules/shell/RailPanelHost.svelte`
- Diubah: `frontend/src/modules/shell/ShellFrame.svelte` (props `left` opsional, kolom rail, grid empat kolom, kelas `hidden`)
- Diubah: `frontend/src/modules/shell/shell.css` (gaya `.rail`, `.rail-btn`, `.rail-btn.active`, `--rail-w`)

## Wiring
Fitur ini hanya mengekspos; penyambungan dilakukan fitur `agents-rail-item` (Tahap C). Jangan lakukan sendiri:
- `RailPanelHost.svelte` dan `Rail.svelte` sebagai berkas yang dapat diimpor langsung (`./modules/shell/RailPanelHost.svelte`); ekspor ulang di `modules/shell/index.ts` dilakukan oleh wiring.
- `ShellFrame` bekerja tanpa snippet `left` (fallback `RailPanelHost`) maupun dengan snippet `left` dari `App.svelte`; `App.svelte` tidak diubah di fitur ini.
- Item rail muncul hanya setelah ada yang memanggil `registerRailItem` (item `agents` didaftarkan oleh wiring). Tanpa itu rail kosong dan panel kosong; ini diharapkan sampai wiring.

Catatan untuk laporan (jangan diperbaiki di luar ownedPaths): `Resizer.svelte` menghitung lebar kiri dari `clientX` mentah (`compute`), padahal panel kini bergeser 48px oleh rail, sehingga lebar saat drag bisa lebih besar 48px dari posisi kursor. Laporkan ketidaksesuaian ini; jangan edit `Resizer.svelte`.

## Selesai bila
- `Rail.svelte` dan `RailPanelHost.svelte` ada dan memakai impor langsung `./registry` dan `../../lib/stores/shell` (tidak ada impor dari `./index`).
- Rail 48px tetap, tampil walau `leftSidebar.visible` false; tidak ada Resizer pada rail.
- Tombol rail `<button type="button">` dengan `aria-label` dan `title` = `spec.title`, `aria-current="true"` hanya pada item aktif, class `active`, indikator `:focus-visible`.
- `RailPanelHost` hanya me-mount komponen item aktif dengan props `{}` dan mereset `activeRail` ke `'agents'` bila item aktif tidak lagi visible, tanpa loop.
- `ShellFrame` menerima `left` opsional dengan fallback `RailPanelHost`; grid `48px var(--left-w) minmax(0,1fr) var(--right-w)`; tidak ada border ganda saat panel tersembunyi; center dan panel kanan tak berubah.
- Hanya empat ownedPaths yang berubah (cek `git status`/`git diff --stat` untuk path fitur ini).
- Guard: `npm run build` di `frontend/` sukses. Tanpa unit test.
