# Plan: Renderer layout, split, resize, dan maksimalkan

## Metadata

- Tiket: `shikigami-v2-ui-ticket-03-pane-layout` (`docs/tickets/shikigami-v2-ui-ticket-03-pane-layout.md`)
- Fitur: `layout-renderer`
- Kontrak sumber kebenaran: `docs/plans/shikigami-v2-ui-ticket-03-pane-layout/overview.md` (bagian 2.2, 2.4, 2.5, 2.6, 2.7, 2.8, 2.10, 4, 6, 7). Baca dulu; plan ini hanya urutan kerja.
- ownedPaths:
  - `frontend/src/modules/layout/LayoutArea.svelte`
  - `frontend/src/modules/layout/Splitter.svelte`
  - `frontend/src/modules/layout/shortcuts.ts`
- Depends on foundation: ya (`layout-model`, plan `00-foundation.md`). Harus sudah ada: `modules/layout/model.ts`, `modules/layout/registry.ts`, `lib/stores/layout.ts`, serta stub `modules/layout/panes/hosts.ts`, `modules/layout/PaneHeader.svelte`, `modules/layout/dnd.ts`, `modules/layout/DropOverlay.svelte`. Bila salah satu belum ada, hentikan dan laporkan sebagai blocker (jangan membuatnya sendiri).

## Tujuan

Merender `LayoutDoc` sebagai pohon flex rekursif dengan pembatas yang bisa diseret, bingkai fokus, maksimalkan (pane lain disembunyikan, tetap ter-mount), keadaan kosong, satu `DropOverlay`, dan pintasan Cmd+D / Cmd+Shift+D yang membuat sesi baru lalu membagi pane. Memindah/split/maksimalkan tidak boleh membuat ulang isi pane: renderer hanya memindahkan elemen host stabil dari `panes/hosts.ts` ke slot posisi barunya.

## Path yang dimiliki

HANYA tiga berkas ini yang boleh dibuat/diubah:

- `frontend/src/modules/layout/LayoutArea.svelte`
- `frontend/src/modules/layout/Splitter.svelte`
- `frontend/src/modules/layout/shortcuts.ts`

Dilarang menyentuh: `model.ts`, `registry.ts`, `lib/stores/layout.ts`, `panes/`, `PaneHeader.svelte`, `PaneMenu.svelte`, `dnd.ts`, `DropOverlay.svelte`, `index.ts` (path fitur lain), `App.svelte`, `modules/shell/`, `modules/tabs/` (wiring), `lib/stores/tabs.ts`, `lib/stores/sessions.ts`, `lib/ipc.ts`, `main.ts`, `package.json`, kode Go. Bila stub/kontrak perlu diubah signature-nya, hentikan dan laporkan sebagai blocker.

## Kontrak

### Dipakai (jangan diubah; signature final di overview)

- `model.ts`: tipe `Leaf`, `Node`, `Split`, `LayoutDoc`, `DropZone`, `SplitPath`, `NewLeaf`; fungsi murni `leaves`, `findLeaf`, `pathOf`, `emptyDoc`, `clampRatio`.
- `lib/stores/layout.ts`: `layout` (bawaan), tipe `LayoutStore` (metode `openPane`, `splitPane`, `closePane`, `resize`, `maximize`, `restore`, `tidyGrid`, `setFocus`, `renamePane`; `Readable<LayoutDoc>`).
- `modules/layout/registry.ts`: `getPaneKind(kind): PaneKindSpec | undefined` (helper non-reaktif dari foundation; bila ternyata tidak ada, pakai `get(paneKinds())[kind]`), tipe `PaneKindSpec` (`onEndSession`).
- `modules/layout/panes/hosts.ts` (overview 2.6): `ensureHost(leaf, visible)`, `getHost(paneId)`, `attachHost(paneId, slot)`, `detachHost(paneId)`, `syncPanes(leaves, visibleIds)`. `releaseHost` dipanggil HANYA oleh `syncPanes`, bukan oleh renderer.
- `PaneHeader.svelte` props final (overview 2.7): `{ pane, index, focused, maximized, store?, onSplit(dir), onMaximizeToggle(), onClose(), onRename(title), onEndSession(), onTidy() }`; header punya atribut `data-pane-header`, dan menangani klik ganda sendiri.
- `DropOverlay.svelte` tanpa props. `dnd.ts` tidak diimpor langsung oleh renderer; ia menambah kelas `pane-dragging` pada `document.body`.
- `lib/ipc.ts`: `call` (generik, `call<Session>('session.create', {...})`).
- `lib/stores/sessions.ts`: `sessions` (writable), `createSession(providerId)`. `lib/stores/workspaces.ts`: `activeWorkspace`, `activeWorkspaceId`.
- Pointer: tema lewat variabel `--bg --bg-elev --border --accent --fg-muted` (fallback boleh).

### Disediakan

`shortcuts.ts` (final, overview 2.8):

```ts
export function installLayoutShortcuts(store?: LayoutStore): () => void;
export function splitFocused(dir: 'h' | 'v', store?: LayoutStore): Promise<void>;
// tambahan internal-untuk-renderer (boleh diekspor, bukan bagian kontrak lintas fitur):
export function splitPaneWithNewSession(paneId: string, dir: 'h' | 'v', store?: LayoutStore): Promise<void>;
```

`LayoutArea.svelte`: `let { store = layout }: { store?: LayoutStore } = $props();` tanpa event lain.

`Splitter.svelte` props: `{ dir: 'h' | 'v'; path: SplitPath; store: LayoutStore }`.

DOM yang dijanjikan ke header/dnd: tiap leaf dibungkus `<div class="pane" data-pane-id="<id>">` berisi `PaneHeader` lalu `<div class="pane-slot">` (slot host); CSS global `body.pane-dragging .pane-host { pointer-events: none }` disediakan di `LayoutArea.svelte`.

## Langkah

### 1. `shortcuts.ts`

Impor: `get` dari `svelte/store`; `call` dari `../../lib/ipc`; `layout`, `type LayoutStore` dari `../../lib/stores/layout`; `sessions`, `createSession` dari `../../lib/stores/sessions`; `activeWorkspace` dari `../../lib/stores/workspaces`; `type Session` dari `../../lib/stores/types`; `findLeaf`, `type NewLeaf` dari `./model`.

1. `splitPaneWithNewSession(paneId, dir, store = layout)` (async):
   - `let doc = get(store)`; `const src = findLeaf(doc, paneId)`; bila `!src || src.kind !== 'terminal'` return diam-diam.
   - `providerId = src.provider ?? get(sessions).find(s => s.id === src.sessionId)?.providerId`; bila kosong `console.error('[layout] provider pane tidak diketahui')` lalu return.
   - Cari project: `get(activeWorkspace)?.projects?.find(p => p.id === src.projectId)`. Bila ada: `const s = await call<Session>('session.create', { workspaceId: get(activeWorkspace)!.id, projectId: project.id, cwd: project.path, providerId })` lalu `sessions.update(l => l.some(x => x.id === s.id) ? l : [...l, s])`. Bila tidak ada: `const s = await createSession(providerId)`. Jangan menambah argumen provider/flag izin (diterapkan server).
   - Bungkus seluruh pembuatan sesi dalam try/catch; galat: `console.error('[layout] gagal membuat sesi split:', err)` dan return (tanpa UI baru).
   - Leaf baru: `const nl: NewLeaf = { kind: 'terminal', sessionId: s.id, projectId: s.projectId, provider: s.providerId }` (judul bawaan diisi model dari `provider`).
   - Setelah await: `if (get(store).maximizedId) store.restore()` (agar pane baru terlihat). Lalu `if (findLeaf(get(store), paneId)) store.splitPane(paneId, dir, nl, 'after'); else store.openPane(nl, { dir })` (pane sumber bisa hilang selama await).
   - Guard reentrancy modul-level: `let inflight = false`; bila true, return; set false di `finally`. (Mencegah Cmd+D beruntun membuat banyak sesi.)
2. `splitFocused(dir, store = layout)`: `const id = get(store).focusId; if (!id) return; await splitPaneWithNewSession(id, dir, store)`.
3. `installLayoutShortcuts(store = layout)`:
   - handler `onKeyDown(e: KeyboardEvent)`: cocok bila `e.metaKey && !e.ctrlKey && !e.altKey && e.code === 'KeyD'`; `e.shiftKey` memilih `'v'`, selain itu `'h'`. Selain itu return tanpa efek.
   - Abaikan (return, tanpa preventDefault) bila target editable non-xterm: `const t = e.target as HTMLElement | null; if (t && !t.closest('.xterm-helper-textarea') && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return;`
   - Bila cocok: `e.preventDefault(); e.stopPropagation();` (WAJIB agar xterm tidak meneruskan ke shell). `if (e.repeat) return;` setelah preventDefault. Lalu `void splitFocused(dir, store)`.
   - `window.addEventListener('keydown', onKeyDown, true)` (fase capture); kembalikan `() => window.removeEventListener('keydown', onKeyDown, true)`.
   - Jangan menangani Cmd+O, Cmd+Shift+O, Cmd+=, Cmd+-, Cmd+0, Cmd+B, Cmd+Option+B, Cmd+K, dan jangan menambah handler `menu:zoom`.

### 2. `Splitter.svelte`

- Script: `let { dir, path, store }: { dir: 'h'|'v'; path: SplitPath; store: LayoutStore } = $props();` impor `clampRatio`, `type SplitPath` dari `./model`, `type LayoutStore` dari `../../lib/stores/layout`.
- Markup: `<div class="splitter {dir}" role="separator" aria-orientation={dir === 'h' ? 'vertical' : 'horizontal'} tabindex="-1" onpointerdown={down} onpointermove={move} onpointerup={up} onpointercancel={up} ondblclick={reset}></div>` (selesaikan peringatan a11y svelte-check dengan atribut itu; jangan membiarkan galat baru).
- `down(e)`: hanya tombol utama (`e.button === 0`); `e.preventDefault()`; `(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)`; simpan `dragging = true`, `rect = el.parentElement!.getBoundingClientRect()` (container Split = induk langsung pembatas); `document.body.classList.add('pane-resizing')`.
- `move(e)`: bila `dragging`, simpan `lastX/lastY` dan jadwalkan SATU `requestAnimationFrame` (flag `raf`): di callback hitung `r = dir === 'h' ? (lastX - rect.left) / rect.width : (lastY - rect.top) / rect.height`; `store.resize(path, clampRatio(r))`. Tanpa polling/`setInterval`.
- `up(e)`: `releasePointerCapture` (try/catch), `dragging = false`, `cancelAnimationFrame` bila tertunda (lalu terapkan posisi terakhir sekali agar tidak hilang), hapus kelas `pane-resizing`.
- `reset()` (dblclick): `store.resize(path, 0.5)`.
- `onDestroy`/cleanup: bersihkan kelas `pane-resizing` dan rAF tertunda.
- CSS (scoped): `.splitter { position: relative; flex: 0 0 4px; background: var(--border, #333); z-index: 3; touch-action: none; }`; `.splitter.h { cursor: col-resize }`, `.splitter.v { cursor: row-resize }`; hit area lebar lewat `::before { content:''; position:absolute; }` (`h`: `inset: 0 -3px`; `v`: `inset: -3px 0`); `.splitter:hover, .splitter:active { background: var(--accent, #4c8dff) }`.

### 3. `LayoutArea.svelte`

Script:

- Props: `let { store = layout }: { store?: LayoutStore } = $props();`
- State: `let doc = $state<LayoutDoc>(get(store));` dan `$effect.pre(() => store.subscribe((v) => { doc = v; }));` (kembalikan fungsi unsubscribe).
- Derived: `const all = $derived(leaves(doc))`; `const indexOf = $derived(new Map(all.map((l, i) => [l.id, i + 1])))`; `const maxId = $derived(doc.maximizedId && findLeaf(doc, doc.maximizedId) ? doc.maximizedId : null)`; `const maxPath = $derived(maxId ? pathOf(doc, maxId) : null)`; `const visibleIds = $derived(new Set(maxId ? [maxId] : all.map((l) => l.id)))`.
- Sinkron host: `$effect.pre(() => { syncPanes(all, visibleIds); });` Dijalankan sebelum DOM diperbarui agar host sudah ada saat slot baru muncul; `releaseHost` untuk id yang hilang ditangani `syncPanes`.
- `onMount(() => installLayoutShortcuts(store))` (nilai kembali = fungsi lepas, dipanggil saat destroy).
- Helper `inMax(path: string): boolean` = `maxPath === null || maxPath === path || path === '' || maxPath.startsWith(path + '.')`. Simpul di luar jalur pane maksimal mendapat `display: none`.
- Action slot (di script modul atau instance):

  ```ts
  function hostSlot(node: HTMLElement, p: { leaf: Leaf; visible: boolean }) {
    ensureHost(p.leaf, p.visible);
    attachHost(p.leaf.id, node);
    const id = p.leaf.id;
    return { destroy() { const h = getHost(id); if (h && h.parentElement === node) detachHost(id); } };
  }
  ```

  Guard `parentElement === node` WAJIB: slot baru bisa sudah mengambil host sebelum slot lama dihancurkan; tanpa guard host tercabut dari slot baru. Action hanya MELEPAS, tidak pernah `releaseHost`.
- Callback header per leaf (semua lewat `store`):
  - `onSplit = (dir) => void splitPaneWithNewSession(leaf.id, dir, store)`
  - `onMaximizeToggle = () => (doc.maximizedId === leaf.id ? store.restore() : store.maximize(leaf.id))`
  - `onClose = () => store.closePane(leaf.id)` (tanpa IPC, tidak membunuh sesi)
  - `onRename = (t) => store.renamePane(leaf.id, t)`
  - `onEndSession = async () => { try { await getPaneKind(leaf.kind)?.onEndSession?.(leaf); } catch (err) { console.error('[layout] gagal mengakhiri sesi:', err); } }` (pane TIDAK ditutup)
  - `onTidy = () => store.tidyGrid()`
- Fokus: pada pembungkus pane pasang `onpointerdowncapture={() => { if (doc.focusId !== leaf.id) store.setFocus(leaf.id); }}` (capture agar klik di dalam xterm/editor tetap terdeteksi).

Markup: rekursi lewat snippet (jangan impor komponen lain untuk rekursi):

```svelte
{#snippet renderNode(n: Node, path: string, grow: number)}
  {#if n.type === 'pane'}
    {#key n.id}
      <div class="pane" class:focused={doc.focusId === n.id} data-pane-id={n.id}
           style:flex="{maxPath !== null ? 1 : grow} 1 0" style:display={inMax(path) ? null : 'none'}
           onpointerdowncapture={...}>
        <PaneHeader pane={n} index={indexOf.get(n.id) ?? 0} focused={doc.focusId === n.id}
                    maximized={maxId === n.id} {store} onSplit=... />
        <div class="pane-slot" use:hostSlot={{ leaf: n, visible: visibleIds.has(n.id) }}></div>
      </div>
    {/key}
  {:else}
    <div class="split {n.dir}" style:flex="{maxPath !== null ? 1 : grow} 1 0" style:display={inMax(path) ? 'flex' : 'none'}>
      {@render renderNode(n.a, path ? path + '.a' : 'a', n.ratio)}
      {#if maxPath === null}<Splitter dir={n.dir} {path} {store} />{/if}
      {@render renderNode(n.b, path ? path + '.b' : 'b', 1 - n.ratio)}
    </div>
  {/if}
{/snippet}

<div class="layout-area">
  {#if doc.root}
    {@render renderNode(doc.root, '', 1)}
  {:else}
    <div class="empty">Belum ada pane. Buka sesi dari sidebar atau file dari panel file.</div>
  {/if}
  <DropOverlay />
</div>
```

Catatan kritis:
- `{#key n.id}` WAJIB: tanpa key, `swapPanes` membuat Svelte memakai ulang DOM leaf dengan param action lama sehingga host salah slot.
- `path` untuk Split yang dioper ke `Splitter` adalah jalur simpul Split itu (`''` = root), sama dengan `SplitPath` model.
- Bila `maxPath` tidak null, ancestor dan pane maksimal memakai `flex: 1 1 0` agar mengisi seluruh area; Splitter disembunyikan; pohon dokumen tidak berubah.
- Pane tersembunyi memakai `display: none` pada pembungkus `.pane` (host tetap ter-mount); `visible=false` diteruskan lewat `syncPanes`.
- Kind tak terdaftar ditangani host (teks "Jenis pane tidak dikenal"); renderer tidak crash.

Style:

- `.layout-area { position: relative; display: flex; flex: 1; width: 100%; height: 100%; min-width: 0; min-height: 0; background: var(--bg); overflow: hidden; }`
- `.split { display: flex; min-width: 0; min-height: 0; } .split.h { flex-direction: row } .split.v { flex-direction: column }` (juga `.split > *` aman karena anak punya `min-width/height: 0`).
- `.pane { position: relative; display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: hidden; background: var(--bg-elev, var(--bg)); }`
- `.pane-slot { flex: 1; min-height: 0; min-width: 0; position: relative; }`
- Bingkai fokus: `.pane.focused::after { content: ''; position: absolute; inset: 0; border: 1px solid var(--accent, #4c8dff); pointer-events: none; z-index: 2; }` (pseudo-elemen agar tidak tertutup latar anak).
- `.empty { margin: auto; color: var(--fg-muted); font-size: 13px; text-align: center; padding: 16px; }`
- Global (dalam `<style>` yang sama): `:global(body.pane-dragging .pane-host) { pointer-events: none; }` dan `:global(body.pane-dragging), :global(body.pane-resizing) { user-select: none; }`.

### 4. Verifikasi (guard fitur ini)

1. `cd frontend && npm run check && npm run build` lulus; tidak ada galat/peringatan baru di tiga berkas milik fitur (bandingkan bila `check` sudah punya galat bawaan).
2. `grep -rni "webgl" frontend/src/modules/layout` kosong.
3. `grep -n "console.log\|setInterval" frontend/src/modules/layout/LayoutArea.svelte frontend/src/modules/layout/Splitter.svelte frontend/src/modules/layout/shortcuts.ts` kosong.
4. Pastikan hanya tiga berkas ownedPaths yang berubah.
5. Pengecekan mental (tanpa unit test, tanpa berkas test di repo): pohon `h` bersarang `v` merender flex arah benar; `swapPanes` memindah host lewat `{#key}` tanpa `releaseHost`; maksimalkan lalu pulihkan tidak mengubah `doc.root`; Cmd+D di dalam xterm di-`preventDefault` pada fase capture.

## Perubahan

Dibuat (baru):
- `frontend/src/modules/layout/LayoutArea.svelte`
- `frontend/src/modules/layout/Splitter.svelte`
- `frontend/src/modules/layout/shortcuts.ts`

Diubah: tidak ada. Tidak ada berkas lain yang disentuh (stub foundation tetap apa adanya).

## Wiring

Yang diekspos untuk langkah `layout-wiring` (JANGAN disambungkan di fitur ini):

- `LayoutArea` (default export Svelte) dari `./modules/layout/LayoutArea.svelte`, prop opsional `store`; `App.svelte` nanti menggantinya menjadi isi `<div class="content">` (menggantikan `TabBar`/`TabContent`). Komponen sudah memanggil `installLayoutShortcuts` sendiri di `onMount`; wiring tidak perlu memasangnya lagi.
- `splitFocused` / `splitPaneWithNewSession` membuat sesi lewat `session.create` dan menambah leaf langsung ke `layout` tanpa memanggil `openTab`. Shim `modules/tabs/index.ts` perlu menyadari bahwa pane bisa muncul tanpa entri di store `tabs` (sinkron balik dari `layout` ke `activeTabId` saja; jangan menutup pane yang tidak punya tab).
- Pane tampil benar hanya setelah `panes/hosts.ts` (pane-kinds) dan `PaneHeader`/`dnd`/`DropOverlay` (pane-header-dnd) menimpa stub foundation; renderer tidak bergantung pada isi stub, hanya pada signature.

## Selesai bila

- Tiga berkas ownedPaths ada dengan signature/ekspor persis bagian Kontrak.
- `LayoutArea` merender pohon sembarang kedalaman dengan arah (`h` row, `v` column) dan ratio (`flex: ratio` / `1 - ratio`, `min-width/height: 0`) benar; pohon kosong menampilkan teks petunjuk.
- Splitter: pointer events + `setPointerCapture` + rAF, clamp [0.1, 0.9] lewat `clampRatio`, klik ganda mengembalikan 0.5, kursor `col-resize`/`row-resize`.
- Host dipasang lewat `attachHost` dalam slot per leaf dengan `{#key leaf.id}`; renderer tidak pernah memanggil `releaseHost` (hanya `syncPanes` yang melakukan); `syncPanes` dipanggil pada tiap perubahan pohon/maksimal.
- Maksimalkan: hanya pane maksimal terlihat, yang lain `display:none` + `visible=false`, pohon tidak berubah.
- Bingkai fokus `--accent`; klik pane (capture) memanggil `setFocus`; satu `<DropOverlay />`; CSS global `body.pane-dragging .pane-host { pointer-events: none }` ada.
- Cmd+D (kanan) / Cmd+Shift+D (bawah) pada fase capture dengan `preventDefault` + `stopPropagation`, diabaikan di input non-xterm, membuat sesi baru dengan project dan provider pane fokus lewat `session.create` (fallback `createSession`), tanpa argumen tambahan; tanpa pane fokus atau pane bukan terminal: tidak melakukan apa pun.
- Guard: `cd frontend && npm run check && npm run build` sukses untuk path fitur ini, `grep -rni webgl frontend/src/modules/layout` kosong, tanpa polling, tanpa dependency baru, tanpa `console.log`, tanpa unit test.
