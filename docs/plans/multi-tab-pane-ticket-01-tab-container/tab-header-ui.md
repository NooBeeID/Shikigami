# Plan: Deretan tab di header pane

## Metadata

- Tiket: `docs/tickets/multi-tab-pane-ticket-01-tab-container.md` (epic `multi-tab-pane`)
- Fitur: `tab-header-ui` (Deretan tab di header pane)
- ownedPaths:
  - `frontend/src/modules/layout/PaneHeader.svelte`
  - `frontend/src/modules/layout/PaneMenu.svelte`
  - `frontend/src/modules/shell/LayoutHeaderHost.svelte`
- Depends on foundation: ya (`00-foundation.md`: tipe `Tab`/`Leaf` baru, `activeTab`, `newTabId`, stub `activateTab`/`closeTab`/`renameTab` di `model.ts`). Aksi store `activateTab`/`closeTab`/`renameTab` dan `PaneKindSpec` baru disediakan fitur `tab-model-foundation`; plan ini hanya mengikuti kontrak overview 2.4 dan 2.5 persis.

Rujukan wajib: `overview.md` bagian 2 (kontrak), 6 (build guard), 7 (aturan paralel). Tanpa unit test, jangan tambah vitest.

## Tujuan

Header pane menampilkan deretan tab (judul, `StatusBadge` untuk terminal, tombol tutup) menggantikan satu judul. Klik tab memilih tab dan memfokus pane. Badge, chip proyek, nomor pane mengikuti tab aktif. Tombol "Tutup pane" pindah ke `PaneMenu` (menutup semua tab). Minimize jadi aksi per tab terminal. "Akhiri sesi" bekerja pada tab aktif lalu menutup tab itu. Tombol bagi disembunyikan pada tab aktif non-terminal. Area kosong header tetap memulai drag pane utuh. Tanpa konfirmasi file kotor dan tanpa drag tab (tiket 02 dan 03).

## Path yang dimiliki

HANYA tiga berkas ini yang boleh disentuh:

- `frontend/src/modules/layout/PaneHeader.svelte`
- `frontend/src/modules/layout/PaneMenu.svelte`
- `frontend/src/modules/shell/LayoutHeaderHost.svelte`

Jangan sentuh `LayoutArea.svelte`, `App.svelte`, `layout/index.ts`, `model.ts`, `registry.ts`, `stores/layout.ts`, `dnd.ts`, `DropOverlay.svelte`, atau path fitur lain. Temuan: `LayoutHeaderHost.svelte` saat ini hanya merender `layoutHeaders()` dari `shell/registry` (slot header layout global) dan TIDAK memakai `PaneHeader` maupun tipe `Leaf`. Maka berkas itu hanya diverifikasi; ubah hanya jika `svelte-check` menunjukkan error akibat tipe baru (kemungkinan besar tidak ada perubahan). Jangan menambahkan logika tab ke sana.

## Kontrak

Dipakai (dari overview, jangan diubah):

- Tipe `Tab`, `Leaf` (`tabs`, `activeTabId`), helper `activeTab(leaf)` dari `./model` (2.1, 2.2).
- Store `LayoutStore` (`../../lib/stores/layout`) dengan aksi (2.4): `activateTab(paneId, tabId)` (sudah memanggil `setFocus(paneId)`), `closeTab(paneId, tabId)`, `renameTab(paneId, tabId, title)`, plus yang sudah ada: `closePane(paneId)`, `setFocus(paneId)`.
- Registry (2.5): `getPaneKind(kind)` mengembalikan `PaneKindSpec` dengan `onEndSession?(tab: Tab): Promise<void>`, `badgeFor?(tab)`, `titleFor?(tab)`.
- `startPaneDrag(e, paneId, title, store)` dari `./dnd` (tidak berubah).

Disediakan (props baru `PaneHeader.svelte`):

```ts
interface Props {
  pane: Leaf;                 // pane utuh (bukan Tab)
  index: number;              // nomor pane
  focused: boolean;
  maximized: boolean;
  store?: LayoutStore;        // default layout
  onSplit: (dir: 'h' | 'v') => void;   // dipanggil hanya bila tab aktif terminal
  onMaximizeToggle: () => void;
  onTidy: () => void;
  // Legacy, DIABAIKAN (agar LayoutArea lama tetap kompilasi selama fase paralel):
  onClose?: () => void;
  onMinimize?: () => void;
  onRename?: (title: string) => void;
  onEndSession?: () => void;
}
```

Aksi tab (tutup tab, tutup pane, akhiri sesi, minimize, rename) ditangani DI DALAM `PaneHeader` lewat `store` dan registry; pemanggil tidak perlu menyuplai callback.

Disediakan (props baru `PaneMenu.svelte`):

```ts
interface Props {
  anchor: HTMLElement;
  canEndSession: boolean;     // tab aktif terminal, spec punya onEndSession, sesi belum exited
  onEndSession: () => void;   // akhiri sesi tab aktif lalu tutup tab
  onRename: () => void;       // ganti nama tab aktif
  onTidy: () => void;
  onClosePane: () => void;    // tutup SEMUA tab pane (terminal diakhiri)
  onDismiss: () => void;
}
```

(Prop `onClose` lama diganti nama `onClosePane`.)

## Langkah

### 1. Baca berkas

Baca penuh tiga berkas milik fitur ini dan `LayoutArea.svelte` (hanya baca, untuk melihat cara `PaneHeader` dipanggil), `lib/stores/layout.ts` (nama aksi), `layout/registry.ts`.

### 2. `PaneHeader.svelte`: script

1. Impor: `import { activeTab, type Leaf, type Tab } from './model';`, `onMount`/`tick`/`$effect` dari svelte seperlunya. Hapus pemakaian `pane.kind` langsung.
2. Turunan reaktif (semua dari tab aktif):
   - `const active = $derived(activeTab(pane));`
   - `spec = getPaneKind(active.kind)`, `badge = spec?.badgeFor?.(active) ?? null`.
   - `session = active.sessionId ? $sessions.find(s => s.id === active.sessionId) : undefined`.
   - `isTerminal = active.kind === 'terminal' && !!active.sessionId` (untuk tombol bagi, menu).
   - `canEnd = isTerminal && !!spec?.onEndSession && session?.status !== 'exited'`.
   - `chip` seperti sekarang tetapi memakai `active.projectId`, `active.kind`, `active.filePath`.
3. Helper per tab: `function isTermTab(t: Tab) { return t.kind === 'terminal' && !!t.sessionId; }`.
4. Fungsi aksi (async yang menelan galat dengan `console.error('[layout] ...')`, gaya `endSession` di LayoutArea):
   - `async function endTabSession(t: Tab)`: `await getPaneKind(t.kind)?.onEndSession?.(t)` dalam try/catch.
   - `function selectTab(t: Tab)`: `store.activateTab(pane.id, t.id)` (aksi store sudah memfokus pane; tidak perlu `setFocus` terpisah).
   - `function closeTabAction(t: Tab)`: `store.closeTab(pane.id, t.id)` dulu, lalu bila `isTermTab(t)` `void endTabSession(t)` (urutan sama seperti `closeAndEnd` lama: tutup layout dulu, akhiri sesi sesudahnya; tab terminal ditutup = sesi diakhiri, menjaga anti-goal sesi yatim). Tanpa konfirmasi dirty.
   - `function minimizeTab(t: Tab)`: hanya bila `isTermTab(t)`; `store.closeTab(pane.id, t.id)` TANPA `endTabSession` (sesi tetap hidup, tersedia di sidebar). Tab terakhir = pane ikut hilang (ditangani `closeTab` model).
   - `function endActiveSession()`: `const t = active; store.closeTab(pane.id, t.id); void endTabSession(t);` (bekerja pada tab aktif, lalu menutup tab itu).
   - `function closeWholePane()`: `const tabs = [...pane.tabs]; store.closePane(pane.id); for (const t of tabs) if (isTermTab(t)) void endTabSession(t);` (salin tab sebelum menutup karena `pane` reaktif).
5. Rename tab aktif: pertahankan state `editing`/`draft`/`committed`/`inputEl`. `startRename()` memakai `active.title`; `commitRename()` memanggil `store.renameTab(pane.id, active.id, t)` bila `t && t !== active.title`. Input rename dirender menggantikan judul TAB AKTIF saja (di dalam tombol tab aktif), dengan `onpointerdown`/`ondblclick`/`onkeydown` `stopPropagation`. Dblclick judul tab aktif = `startRename()`.
6. Keyboard tablist (`onTabsKey(e: KeyboardEvent)` pada elemen `role="tablist"`; abaikan bila `editing`):
   - `ArrowRight`/`ArrowLeft`: pindah ke tab berikutnya/sebelumnya (melingkar), `selectTab`, lalu fokuskan tombol tab baru (`tick()` lalu `querySelector('[role=tab][aria-selected=true]')?.focus()`).
   - `Home`/`End`: tab pertama/terakhir.
   - `Delete`: `closeTabAction` pada tab yang sedang terfokus (cari lewat `data-tab-id` pada `e.target.closest('[role=tab]')`, fallback tab aktif).
   - `preventDefault()` pada tombol yang ditangani.
   - Roving tabindex: tab aktif `tabindex="0"`, lainnya `tabindex="-1"`.
7. Scroll tab aktif ke tampak: `let tabsEl: HTMLElement | undefined = $state();` lalu
   ```ts
   $effect(() => {
     void active.id; void pane.tabs.length;
     tick().then(() => {
       tabsEl?.querySelector<HTMLElement>('[role=tab][aria-selected=true]')
         ?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
     });
   });
   ```
8. Drag: `onHeaderDown` tetap, tetapi guard `closest('button, input, .pane-menu')` diperluas ke `closest('button, input, .pane-menu, [role=tab]')` agar menekan tab (belum ada drag tab) tidak memulai drag pane; area kosong header (spacer, nomor, ruang di luar tab, jarak antar tab di `tablist`) tetap `startPaneDrag(e, pane.id, active.title, store)`. Catatan: `role=tab` dibuat sebagai `<div role="tab">` berisi tombol tutup; pointerdown di wilayah tab tidak memulai drag. Pastikan `tablist` itu sendiri BUKAN penerima guard (hanya `[role=tab]`), supaya sisa ruang `tablist` yang kosong tetap drag.
9. `onHeaderDbl` tidak berubah (dblclick area kosong = maksimalkan; guard `closest('button, input, [role=tab]')`).

### 3. `PaneHeader.svelte`: markup

Susunan dalam `.pane-header` (`role="toolbar"`, `aria-label="Header pane {index}"` tetap):

1. `<span class="num">{index}</span>`.
2. `<div class="tabs" role="tablist" aria-label="Tab pane {index}" bind:this={tabsEl} onkeydown={onTabsKey}>` berisi `{#each pane.tabs as t (t.id)}`:
   ```svelte
   <div class="tab" class:active={t.id === active.id} role="tab"
        aria-selected={t.id === active.id} tabindex={t.id === active.id ? 0 : -1}
        data-tab-id={t.id} title={t.title}
        onclick={() => selectTab(t)}
        onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); selectTab(t); } }}>
     {#if isTermTab(t)}<StatusBadge sessionId={t.sessionId!} compact />{/if}
     {#if editing && t.id === active.id}<input class="rename" .../>
     {:else}<span class="title" ondblclick={...jika aktif: stopPropagation + startRename}>{t.title}</span>{/if}
     {#if isTermTab(t)}<button class="tbtn" aria-label="Minimize tab" title="Minimize (sesi tetap jalan)" onclick={(e)=>{e.stopPropagation(); minimizeTab(t);}}>...</button>{/if}
     <button class="tbtn close" aria-label="Tutup tab {t.title}" title="Tutup tab" tabindex="-1"
             onclick={(e)=>{e.stopPropagation(); closeTabAction(t);}}>...</button>
   </div>
   {/each}
   </div>
   ```
   Tambahkan `<!-- svelte-ignore a11y_... -->` hanya bila `svelte-check` mengeluh (div dengan `role="tab"` interaktif sudah punya `tabindex` dan handler key, jadi biasanya aman). Ikon minimize/tutup memakai SVG yang sama dengan tombol header lama (garis `M3 11h8`, silang `M3 3l8 8M11 3l-8 8`) dalam ukuran 14px.
3. Setelah tablist: `{#if badge}<span class="badge">{badge}</span>{/if}` dan `{#if chip}<span class="chip" title={chip.tip}>{chip.text}</span>{/if}` (dari tab aktif, di luar area scroll tab agar tetap tampak).
4. `<span class="spacer"></span>` (area kosong drag).
5. Tombol kanan, urutan: Bagi kanan, Bagi bawah (keduanya `{#if isTerminal}` agar disembunyikan pada tab aktif non-terminal; `onclick={() => onSplit('h'|'v')}`), Maksimalkan/Pulihkan (tetap), Menu (tetap). HAPUS tombol "Minimize" tingkat pane dan tombol "Tutup pane" dari header.
6. Menu: `<PaneMenu anchor={menuBtn} canEndSession={canEnd} onEndSession={endActiveSession} onRename={startRename} {onTidy} onClosePane={closeWholePane} onDismiss=... />`.

### 4. `PaneHeader.svelte`: style

- `.tabs { display:flex; align-items:center; gap:2px; flex: 0 1 auto; min-width:0; overflow-x:auto; overflow-y:hidden; scrollbar-width:none; }` plus `.tabs::-webkit-scrollbar { display:none; }`. Biarkan scroll lewat wheel horizontal: tambahkan `onwheel` pada `tablist` yang menerjemahkan `deltaY` ke `scrollLeft` bila `deltaX === 0` dan konten overflow (`tabsEl.scrollLeft += e.deltaY`, `preventDefault` hanya saat overflow).
- `.tab { display:inline-flex; align-items:center; gap:6px; height:26px; padding:0 4px 0 8px; max-width:200px; min-width:60px; flex:none; border-radius:4px; cursor:pointer; color: var(--fg-muted,#999); }` `.tab:hover { background: var(--bg-hover,#333); }` `.tab.active { background: var(--bg-hover,#333); color: var(--fg,#ddd); }` `.pane-header.focused .tab.active .title { font-weight:600; }` `.tab:focus-visible { outline:1px solid var(--accent,#4f8cff); }`.
- `.tbtn` kecil 18x18, transparan, tampil hanya saat `.tab:hover`, `.tab.active`, atau `:focus-within` (opacity); `.tbtn.close:hover { color: var(--danger,#e5534b); }`.
- Pertahankan `.title`, `.rename`, `.badge`, `.chip`, `.spacer`, `.hbtn`, `.num` apa adanya; hapus `.hbtn.close` yang tak dipakai.
- Pane satu tab harus tampil mirip sekarang: satu tab aktif tanpa latar mencolok bila `pane.tabs.length === 1` (`.tabs.single .tab { background: transparent; padding-left:0 }`, tombol tutup tetap tersedia).

### 5. `PaneMenu.svelte`

- Ganti prop `onClose` menjadi `onClosePane`; item terakhir tetap berlabel "Tutup pane" dan memanggil `run(onClosePane)`. Ubah label menjadi "Tutup pane (semua tab)" bila muat; boleh tetap "Tutup pane" dengan `title="Menutup semua tab; sesi terminal diakhiri"`.
- Teks konfirmasi "Akhiri sesi ini?" diganti "Akhiri sesi tab ini?" (menu bekerja pada tab aktif). Perilaku `confirming` dan fokus tidak berubah.
- "Ganti nama" mengganti nama tab aktif (label "Ganti nama tab").
- Tidak ada konfirmasi dirty untuk "Tutup pane" (tiket 03).

### 6. `LayoutHeaderHost.svelte`

Tidak ada perubahan yang diharapkan (lihat Path yang dimiliki). Jalankan `svelte-check` dan filter ke berkas ini; ubah hanya bila ada error nyata dari tipe baru.

### 7. Verifikasi (guard fitur)

Dari `/Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal/frontend`:

```
npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E "layout/PaneHeader.svelte|layout/PaneMenu.svelte|shell/LayoutHeaderHost.svelte"
```

Tidak boleh ada error di tiga berkas itu. Error di berkas fitur lain (mis. `LayoutArea.svelte` masih memakai `renamePane`, atau aksi store belum ada karena `tab-model-foundation` belum selesai) DIABAIKAN dan dicatat di laporan. Bila aksi store `activateTab`/`closeTab`/`renameTab` belum ada di `LayoutStore`, tetap tulis kode persis sesuai kontrak 2.4; jangan edit `stores/layout.ts`.

## Perubahan

- Diubah: `frontend/src/modules/layout/PaneHeader.svelte` (props baru, tablist, aksi tab, hapus tombol minimize/tutup tingkat pane).
- Diubah: `frontend/src/modules/layout/PaneMenu.svelte` (prop `onClosePane`, label tab aktif).
- Diverifikasi, kemungkinan tidak diubah: `frontend/src/modules/shell/LayoutHeaderHost.svelte`.
- Tidak ada berkas baru.

## Wiring

Yang diekspos untuk disambungkan langkah `tab-wiring` (JANGAN dikerjakan di fitur ini):

- `LayoutArea.svelte` (milik `tab-hosts-render`/wiring) memanggil `PaneHeader` cukup dengan: `pane={n}`, `index`, `focused`, `maximized`, `{store}`, `onSplit={(dir) => void splitPaneWithNewSession(n.id, dir, store)}`, `onMaximizeToggle`, `onTidy`. Prop `onClose`, `onMinimize`, `onRename`, `onEndSession` sudah tidak dipakai (masih diterima opsional dan diabaikan); wiring sebaiknya menghapusnya beserta `closeAndEnd`/`endSession` yang jadi mati di `LayoutArea`.
- `splitPaneWithNewSession` (di `shortcuts.ts`, milik `tab-callers`) harus membaca tab aktif pane; tombol bagi di header hanya muncul pada tab aktif terminal.
- `PaneHeader` bergantung pada `LayoutStore.activateTab/closeTab/renameTab` (milik `tab-model-foundation`) dan `PaneKindSpec.onEndSession(tab)` per tab (diisi `tab-hosts-render`/registrasi kind).
- Jalur lama `renamePane` tidak dipakai lagi oleh header.
- `LayoutHeaderHost.svelte` tidak butuh disambungkan.

## Selesai bila

- `svelte-check` tidak menampilkan error di `PaneHeader.svelte`, `PaneMenu.svelte`, `LayoutHeaderHost.svelte` (guard fitur; build penuh baru diwajibkan di `tab-wiring`).
- Header memuat `role="tablist"` dengan `role="tab"`, `aria-selected`, roving tabindex, navigasi ArrowLeft/ArrowRight/Home/End/Delete, scroll horizontal, dan tab aktif di-scroll ke tampak.
- Klik tab memanggil `store.activateTab(pane.id, tab.id)`; tombol tutup memanggil `store.closeTab` (tab terminal juga mengakhiri sesi); tab terakhir menutup pane lewat model.
- Badge, chip proyek, dan `StatusBadge` mengikuti data tab (chip/badge dari tab aktif, `StatusBadge` per tab terminal); nomor pane tetap per pane.
- Tombol header hanya: bagi kanan, bagi bawah (hanya tab aktif terminal), maksimalkan, menu. "Tutup pane" ada di `PaneMenu` dan menutup semua tab dengan mengakhiri sesi terminal.
- Minimize hanya pada tab terminal, memanggil `closeTab` tanpa `onEndSession`.
- "Akhiri sesi" di menu mengakhiri sesi tab aktif lalu menutup tab itu.
- Menekan area kosong header masih memulai `startPaneDrag`; menekan tab tidak.
- Tidak ada konfirmasi dirty, tidak ada drag tab, tidak ada berkas di luar ownedPaths yang diubah.
