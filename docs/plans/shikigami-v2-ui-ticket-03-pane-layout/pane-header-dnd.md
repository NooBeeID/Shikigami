# Plan: Header pane dan drag and drop zona

## Metadata

- Tiket: `shikigami-v2-ui-ticket-03-pane-layout` (`docs/tickets/shikigami-v2-ui-ticket-03-pane-layout.md`)
- Fitur: `pane-header-dnd` (Header pane dan drag and drop zona)
- Overview (sumber kontrak, baca dulu, jangan diubah): `docs/plans/shikigami-v2-ui-ticket-03-pane-layout/overview.md` (bagian 2.2, 2.4, 2.5, 2.7, 4, 6, 7)
- ownedPaths:
  - `frontend/src/modules/layout/PaneHeader.svelte`
  - `frontend/src/modules/layout/PaneMenu.svelte`
  - `frontend/src/modules/layout/dnd.ts`
  - `frontend/src/modules/layout/DropOverlay.svelte`
- Depends on foundation: ya (`layout-model`, plan `00-foundation.md`). Foundation sudah menyediakan `model.ts`, `registry.ts`, `lib/stores/layout.ts`, dan STUB `PaneHeader.svelte`, `dnd.ts`, `DropOverlay.svelte` (baris pertama `// STUB(foundation)` atau komentar HTML setara). Fitur ini MENIMPA ketiga stub itu seluruhnya (bukan menambal) dan membuat `PaneMenu.svelte` baru. Signature ekspor stub wajib dipertahankan persis.

## Tujuan

Menyediakan header tiap pane (nomor, judul dengan rename inline, lencana provider, titik status untuk terminal, chip path project, tombol split/maksimalkan/tutup/menu `...`), menu `...` (`PaneMenu`), dan drag and drop zona buatan sendiri (tanpa library DnD): `dnd.ts` (state drag, `zoneAt`, `startPaneDrag`, ghost, Esc batal, `movePane` saat lepas) serta `DropOverlay` yang menyorot zona. Seluruh perilaku mengikuti acceptance criteria fitur `pane-header-dnd` di tiket dan kontrak overview 2.7. Anti-goal tiket: drag hanya memanggil `store.movePane` (mengubah pohon); fitur ini tidak menyentuh host DOM atau xterm.

## Path yang dimiliki

HANYA empat berkas ini yang boleh dibuat/diubah:

- `frontend/src/modules/layout/PaneHeader.svelte` (timpa stub)
- `frontend/src/modules/layout/PaneMenu.svelte` (baru)
- `frontend/src/modules/layout/dnd.ts` (timpa stub)
- `frontend/src/modules/layout/DropOverlay.svelte` (timpa stub)

Dilarang menyentuh: `model.ts`, `registry.ts`, `lib/stores/layout.ts` (foundation), `LayoutArea.svelte`, `Splitter.svelte`, `shortcuts.ts` (layout-renderer), `panes/`, `index.ts`, `modules/terminal/` (pane-kinds), `App.svelte`, `modules/shell/`, `modules/tabs/` (layout-wiring), `lib/stores/sessions.ts`, `lib/stores/workspaces.ts`, `modules/status/`, `modules/files/`, `package.json`, kode Go. Bila signature stub foundation tidak cukup, hentikan dan laporkan sebagai blocker; jangan ubah kontrak sepihak. Dilarang menambah dependency npm, unit test, dan `console.log`.

## Kontrak

### Disediakan (final, overview 2.7)

`PaneHeader.svelte` props (Svelte 5 `$props()`):

```ts
{ pane: Leaf; index: number /* berbasis 1 */; focused: boolean; maximized: boolean; store?: LayoutStore;
  onSplit: (dir: 'h' | 'v') => void; onMaximizeToggle: () => void; onClose: () => void;
  onRename: (title: string) => void; onEndSession: () => void; onTidy: () => void }
```

Akar header memakai atribut `data-pane-header`. `store` (bawaan `layout`) hanya diteruskan ke `startPaneDrag`.

`dnd.ts`:

```ts
export interface DragState { paneId: string; title: string; targetId: string | null; zone: DropZone | null;
  rect: { x: number; y: number; w: number; h: number } | null }
export const dragState: Readable<DragState | null>;
export function zoneAt(rect: { x: number; y: number; w: number; h: number }, x: number, y: number): DropZone;
export function startPaneDrag(e: PointerEvent, paneId: string, title: string, store?: LayoutStore): void;
```

`DropOverlay.svelte`: tanpa props. `PaneMenu.svelte`: komponen internal header (props di Langkah 3), tidak dirujuk fitur lain.

Selama drag, `startPaneDrag` menambah kelas `pane-dragging` pada `document.body` dan menghapusnya saat selesai/batal. CSS `body.pane-dragging .pane-host { pointer-events: none }` disediakan `layout-renderer` (jangan tulis di sini).

### Dipakai (hanya baca/impor; path relatif dari `frontend/src/modules/layout/`)

- Dari `./model`: tipe `Leaf`, `DropZone`.
- Dari `../../lib/stores/layout`: tipe `LayoutStore`, instans `layout` (bawaan param `store`); aksi `store.movePane(paneId, targetId, zone)`.
- Dari `./registry`: `getPaneKind(kind): PaneKindSpec | undefined` (helper foundation; `badgeFor`, `onEndSession`).
- Dari `../status/StatusBadge.svelte`: `<StatusBadge sessionId compact />`.
- Dari `../../lib/stores/workspaces`: `activeWorkspace` (Readable; `projects: {id, path, name}[]`). Dari `../../lib/stores/sessions`: `sessions` (`Session {id, cwd, status, projectId, ...}`).
- Variabel tema: `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger` (fallback hardcode boleh). Teks UI Indonesia.

Tidak ada pesan IPC di fitur ini; semua efek lewat callback props dan `store.movePane`.

## Langkah

1. **`dnd.ts` (timpa stub).** Hapus baris `// STUB(foundation)`. Impor `writable`, `type Readable` dari `svelte/store`; `get` tidak perlu. Impor tipe `DropZone` dari `./model`, `type LayoutStore` dan `layout` dari `../../lib/stores/layout`.
   1. Ekspor `DragState`; `const state = writable<DragState | null>(null)`; `export const dragState: Readable<DragState | null> = { subscribe: state.subscribe }`.
   2. `zoneAt(rect, x, y)` deterministik: jepit (clamp) `x,y` ke dalam rect; `dL = x - rect.x`, `dR = rect.x + rect.w - x`, `dT = y - rect.y`, `dB = rect.y + rect.h - y`. Kandidat sisi ada bila jarak <= 25% dimensi tegak lurusnya: `top`/`bottom` bila `dT/dB <= 0.25 * rect.h`, `left`/`right` bila `dL/dR <= 0.25 * rect.w`. Tanpa kandidat: `center`. Ada kandidat: pilih jarak piksel terkecil; seri diputus urutan tetap `top, bottom, left, right` (iterasi berurutan dengan `<` ketat). Rect berdimensi 0: kembalikan `center`. Fungsi murni, tanpa DOM.
   3. Helper internal `sideRect`-tidak perlu di sini (dihitung di `DropOverlay`).
   4. `startPaneDrag(e, paneId, title, store = layout)`:
      - Abaikan bila `e.button !== 0` atau drag lain sedang aktif (variabel modul `active: boolean`).
      - Catat `startX/startY = e.clientX/Y`, `pointerId`, `source = e.currentTarget as HTMLElement | null` (header). Belum ada efek sampai gerak >= 4px: `Math.hypot(dx, dy) >= 4` (ambang agar klik ganda tidak memulai drag). Sebelum ambang terlewati, `pointerup` hanya membersihkan listener (tanpa efek, tanpa `preventDefault`).
      - Pasang listener pada `window`: `pointermove`, `pointerup`, `pointercancel`, `keydown` (fase capture), dan `blur`. Simpan referensi agar bisa dilepas.
      - Saat ambang pertama kali terlewati (`started = true`): `source?.setPointerCapture(pointerId)` dalam try/catch; buat ghost (`div` `position: fixed; z-index: 10000; pointer-events: none; max-width: 240px`, teks `title` dengan ellipsis, gaya memakai `--bg-elev`, `--border`, `--fg`, opacity 0.9, padding kecil, radius 4px) dan tempel ke `document.body`; `document.body.classList.add('pane-dragging')`; set `document.body.style.userSelect = 'none'`; `state.set({ paneId, title, targetId: null, zone: null, rect: null })`.
      - Tiap `pointermove` setelah mulai: posisikan ghost `left = clientX + 12`, `top = clientY + 12` (`transform`/left-top langsung, tanpa polling). Cari target: `const el = document.elementFromPoint(clientX, clientY)`; `const t = el?.closest('[data-pane-id]') as HTMLElement | null`. Bila `t` ada dan `t.dataset.paneId !== paneId`: `r = t.getBoundingClientRect()`, `rect = {x: r.left, y: r.top, w: r.width, h: r.height}`, `zone = zoneAt(rect, clientX, clientY)`, set state `{paneId, title, targetId, zone, rect}`. Selain itu (di luar pane, atau di pane sendiri): `targetId/zone/rect = null`. Hindari `state.set` bila targetId/zone/rect tidak berubah (bandingkan nilai sebelumnya yang disimpan lokal).
      - `keydown` dengan `key === 'Escape'`: `preventDefault()`, `stopPropagation()`, batalkan (cleanup tanpa `movePane`); sebelum drag dimulai Esc juga hanya membersihkan.
      - `pointerup`: bila `started` dan state punya `targetId` dan `zone`: `store.movePane(paneId, targetId, zone)`. Lepas di luar pane atau di pane sendiri: tidak melakukan apa pun. Jika `started`, pasang listener `click` sekali di `window` (fase capture, `{ once: true }`, `stopPropagation()`, dan dilepas otomatis lewat `setTimeout(..., 0)` bila tidak ada click) agar klik sintetis setelah drag tidak memicu aksi tombol/fokus di bawahnya.
      - `pointercancel`, `blur`, dan `lostpointercapture` yang tidak diikuti `pointerup`: batalkan.
      - `cleanup()` (idempoten): lepas semua listener, `releasePointerCapture` (try/catch), hapus ghost, `body.classList.remove('pane-dragging')`, kembalikan `body.style.userSelect` ke nilai semula, `state.set(null)`, `active = false`.
   5. Tidak ada `setInterval`/polling; `requestAnimationFrame` tidak wajib. Tidak ada `console.log`.

2. **`DropOverlay.svelte` (timpa stub).** Tanpa props. `import { dragState } from './dnd'`. Gunakan `$derived` dari `$dragState`: bila `null`, atau `rect`/`zone` null, jangan render apa pun. Hitung kotak sorotan dari `rect` dan `zone`: `top` = `{x, y, w, h: h/2}`, `bottom` = `{x, y: y + h/2, w, h: h/2}`, `left` = `{x, y, w: w/2, h}`, `right` = `{x: x + w/2, y, w: w/2, h}`, `center` = seluruh rect. Render satu `div.drop-zone` dengan `style="left:..px;top:..px;width:..px;height:..px"`. CSS: `position: fixed; pointer-events: none; z-index: 9999; box-sizing: border-box; border: 2px solid var(--accent, #4f8cff); background: color-mix(in srgb, var(--accent, #4f8cff) 22%, transparent); border-radius: 4px; transition: left/top/width/height 80ms` (transisi dinonaktifkan di `prefers-reduced-motion`). Komponen ini hanya tampil selama drag (karena `dragState` null di luar drag).

3. **`PaneMenu.svelte` (baru).** Props:

   ```ts
   { anchor: HTMLElement; canEndSession: boolean; onEndSession: () => void; onRename: () => void;
     onTidy: () => void; onClose: () => void; onDismiss: () => void }
   ```

   - Elemen akar `div.pane-menu[role="menu"]` dengan `position: fixed; z-index: 9000`. Posisi dihitung di `onMount` dari `anchor.getBoundingClientRect()`: `top = r.bottom + 4`, rata kanan ke `r.right` (`left = r.right - menuWidth`), dijepit ke viewport (`>= 4`, tidak melewati `innerWidth/innerHeight`). Fixed dipakai agar tidak terpotong `overflow: hidden` pembungkus pane.
   - Item (urutan): "Akhiri sesi" (hanya bila `canEndSession`), "Ganti nama", "Susun grid", pemisah, "Tutup pane". Tiap item `button[role="menuitem"]`.
   - "Akhiri sesi" meminta konfirmasi singkat inline: klik pertama mengganti isi menu menjadi teks "Akhiri sesi ini?" dengan tombol "Akhiri" (kelas `danger`, warna `--danger`) dan "Batal"; "Akhiri" memanggil `onEndSession()` lalu `onDismiss()`; "Batal" kembali ke daftar item. Tidak ada `window.confirm`.
   - "Ganti nama": `onRename()` lalu `onDismiss()`. "Susun grid": `onTidy()` lalu `onDismiss()`. "Tutup pane": `onClose()` lalu `onDismiss()` (menutup pane TIDAK membunuh sesi; itu urusan callback renderer).
   - Penutupan: di `onMount` pasang `window` `pointerdown` (fase capture) yang memanggil `onDismiss()` bila target di luar akar menu DAN di luar `anchor` (klik pada tombol `...` ditangani toggle oleh header), dan `window` `keydown` (fase capture) untuk `Escape`: `preventDefault()`, `stopPropagation()`, `onDismiss()`. Lepas semua listener di cleanup `onMount`. Fokus pertama ke item pertama (`focus()`); panah atas/bawah memindahkan fokus antar item (opsional, kecil).
   - Gaya: `--bg-elev`, `--border`, `--fg`, hover `--bg-hover`, radius 6px, `box-shadow` halus, `min-width: 168px`, font 12px.

4. **`PaneHeader.svelte` (timpa stub).** Hapus komentar stub. Skrip: `$props()` sesuai kontrak (default `store = layout`).
   - State (`$state`): `editing: boolean`, `draft: string`, `menuOpen: boolean`; `menuBtn: HTMLElement | undefined` lewat `bind:this`; `inputEl`.
   - Turunan (`$derived`): `spec = getPaneKind(pane.kind)`; `badge = spec?.badgeFor?.(pane) ?? null`; `isTerminal = pane.kind === 'terminal' && !!pane.sessionId`; `canEnd = isTerminal && !!spec?.onEndSession && sessionStatus !== 'exited'` dengan `sessionStatus` dari `$sessions.find(s => s.id === pane.sessionId)?.status`.
   - **Chip path project** (`$derived`): `project = $activeWorkspace?.projects?.find(p => p.id === pane.projectId)`; untuk terminal ambil `cwd` dari sesi (`$sessions`); untuk file ambil direktori dari `pane.filePath`. Fungsi helper lokal `relativeTo(root, abs)`: bila `abs === root` kosong; bila `abs` diawali `root + '/'` kembalikan sisa tanpa awalan; selain itu `''`. Teks chip = `project.name` ditambah `/subfolder` bila relatif tidak kosong; tanpa `project` tidak ada chip (jangan membuat nama palsu). Tooltip (`title`) = path absolut lengkap (`cwd` atau `project.path`/`filePath`). CSS chip: `max-width` terbatas, `overflow: hidden; text-overflow: ellipsis; white-space: nowrap`, font 11px, `--fg-muted`, latar `--bg-hover`.
   - **Markup** (urut kiri ke kanan), akar `<div class="pane-header" class:focused data-pane-header onpointerdown={onHeaderDown} ondblclick={onHeaderDbl} role="toolbar" aria-label="Header pane {index}">`:
     1. `span.num` berisi `{index}`.
     2. Judul: bila `editing`, `<input bind:this={inputEl} bind:value={draft} onkeydown={onInputKey} onblur={commitRename} />` (stopPropagation pada `keydown`/`pointerdown`/`dblclick` agar tidak memicu drag atau maksimalkan); bila tidak, `span.title` (`title={pane.title}`, ellipsis, `ondblclick` = `startRename` dengan `e.stopPropagation()` sehingga klik ganda judul TIDAK memaksimalkan).
     3. Lencana: `{#if badge}<span class="badge">{badge}</span>{/if}`.
     4. Status: `{#if isTerminal}<StatusBadge sessionId={pane.sessionId!} compact />{/if}` (pane file tanpa titik).
     5. Chip path (bila ada).
     6. Spacer `flex: 1`, lalu tombol (semua `button type="button"` dengan `aria-label`/`title` Indonesia, ikon SVG inline kecil, `class="hbtn"`): "Bagi kanan" (`onSplit('h')`), "Bagi bawah" (`onSplit('v')`), "Maksimalkan"/"Pulihkan" bergantung `maximized` (`onMaximizeToggle()`), "Tutup pane" (`onClose()`), "Menu" `...` (`bind:this={menuBtn}`, toggle `menuOpen`, `aria-haspopup="menu"`, `aria-expanded={menuOpen}`).
     7. `{#if menuOpen && menuBtn}<PaneMenu anchor={menuBtn} canEndSession={canEnd} onEndSession={onEndSession} onRename={startRename} onTidy={onTidy} onClose={onClose} onDismiss={() => (menuOpen = false)} />{/if}`.
   - **Rename inline**: `startRename()`: `draft = pane.title; editing = true;` lalu fokus dan `select()` pada input setelah `tick()` (atau `$effect` pada `inputEl`). `commitRename()` (idempoten, jaga dengan flag agar Enter lalu blur tidak ganda): `const t = draft.trim(); editing = false; if (t && t !== pane.title) onRename(t);` (judul kosong atau tidak berubah tidak memanggil apa-apa). `onInputKey`: `Enter` -> `commitRename()`; `Escape` -> `editing = false` tanpa commit (dan `stopPropagation`); tombol lain `stopPropagation` agar tidak sampai ke pintasan global.
   - **`onHeaderDown(e: PointerEvent)`**: abaikan bila `e.button !== 0`, bila `editing`, atau bila `(e.target as HTMLElement).closest('button, input, .pane-menu')`; selain itu `startPaneDrag(e, pane.id, pane.title, store)`. Jangan `preventDefault()` (agar klik ganda dan fokus pane oleh renderer tetap berfungsi). `touch-action: none` pada header.
   - **`onHeaderDbl(e: MouseEvent)`**: abaikan bila target di dalam `button, input`; selain itu `onMaximizeToggle()`. (Klik ganda judul sudah dihentikan di handler judul.)
   - Gaya: tinggi tetap ~28px, `display: flex; align-items: center; gap: 6px; padding: 0 6px; background: var(--bg-elev); border-bottom: 1px solid var(--border); color: var(--fg); font-size: 12px; cursor: grab; user-select: none; min-width: 0; flex: none`. `.focused` menebalkan nomor/teks (mis. `border-bottom-color: var(--accent)`; bingkai pane utama milik renderer, jangan duplikat). `.hbtn`: 22x22, transparan, hover `--bg-hover`, `--fg-muted`; tombol tutup hover `--danger`. Nomor `span.num` kecil bulat/pil dengan latar `--bg-hover`. `.badge`: huruf besar kecil 10px, border `--border`, radius 3px.

5. **Verifikasi build.** Dari `frontend/`: `npm run check && npm run build`. Perbaiki semua galat type pada empat berkas milik fitur ini (galat bawaan di berkas lain: bandingkan, jangan diperbaiki). Jalankan `grep -rni "webgl" frontend/src/modules/layout` (tidak boleh ada impor WebGL) dan `grep -n "console.log" ` pada empat berkas (kosong). Pastikan tidak ada impor dari `lib/stores/index.ts`, dan tidak ada impor melingkar (`dnd.ts` tidak mengimpor komponen Svelte; `DropOverlay` hanya mengimpor `dnd`; `PaneHeader` mengimpor `PaneMenu` dan `dnd`; `PaneMenu` tidak mengimpor apa pun dari layout).

6. **Pengecekan mental `zoneAt`** (tanpa unit test, tanpa berkas test di repo): rect 400x200 di (0,0): titik (200,100) -> `center`; (10,100) -> `left`; (390,100) -> `right`; (200,5) -> `top`; (200,195) -> `bottom`; sudut (10,10): dL=10, dT=10, seri -> `top` (urutan top dulu); (5,40): dL=5 (kandidat, <=100), dT=40 (kandidat, <=50) -> `left`; titik di luar rect dijepit ke tepi; w atau h = 0 -> `center`.

## Perubahan

- UBAH (timpa stub foundation): `frontend/src/modules/layout/PaneHeader.svelte`
- UBAH (timpa stub foundation): `frontend/src/modules/layout/dnd.ts`
- UBAH (timpa stub foundation): `frontend/src/modules/layout/DropOverlay.svelte`
- BARU: `frontend/src/modules/layout/PaneMenu.svelte`

Tidak ada berkas lain yang dibuat atau diubah.

## Wiring

Fitur ini TIDAK menyambungkan dirinya sendiri. Yang diekspos untuk langkah wiring / `layout-renderer`:

- `PaneHeader.svelte` dengan props kontrak 2.7; `LayoutArea` (milik `layout-renderer`) yang merendernya per leaf di atas slot isi dan menyusun callback (`onSplit`, `onMaximizeToggle`, `onClose`, `onRename`, `onEndSession`, `onTidy`) dari store.
- `DropOverlay.svelte` tanpa props: `LayoutArea` merender SATU instans.
- `dnd.ts`: `dragState`, `zoneAt`, `startPaneDrag` (dipanggil internal oleh `PaneHeader`; `LayoutArea` hanya perlu atribut `data-pane-id` pada pembungkus tiap leaf dan CSS global `body.pane-dragging .pane-host { pointer-events: none }`).
- Persyaratan pada pihak lain (dicatat, bukan dikerjakan di sini): pembungkus leaf harus punya `data-pane-id="<id>"`; pane kind terdaftar (`pane-kinds`) menyediakan `badgeFor`/`onEndSession` agar lencana dan menu "Akhiri sesi" aktif.

## Selesai bila

- Header menampilkan nomor, judul, lencana provider (dari `badgeFor`), `StatusBadge compact` untuk pane terminal (pane file tanpa titik), chip path project (ellipsis + tooltip path penuh), serta tombol split kanan/bawah, maksimalkan/pulihkan, tutup, dan `...`.
- Klik ganda judul masuk rename inline (Enter simpan, Esc batal, kosong/tidak berubah tidak memanggil `onRename`); klik ganda area header lain memanggil `onMaximizeToggle`.
- Menu `...` berisi Akhiri sesi (hanya terminal, dengan konfirmasi inline), Ganti nama, Susun grid, Tutup pane; tertutup oleh klik di luar dan Esc.
- `startPaneDrag` menerapkan ambang 4px, `setPointerCapture`, ghost, `data-pane-id` lewat `elementFromPoint`, Esc membatalkan, lepas di zona memanggil `store.movePane`, lepas di luar pane atau di pane sendiri tidak melakukan apa pun, kelas `pane-dragging` pada `body` selalu dibersihkan.
- `zoneAt` deterministik sesuai Langkah 1 (25% tepi, sudut memilih jarak tepi terkecil, seri top, bottom, left, right).
- `DropOverlay` menyorot setengah pane (sisi) atau seluruh pane (center), `position: fixed`, `pointer-events: none`, hanya saat drag.
- Hanya empat berkas ownedPaths yang berubah; signature ekspor stub dipertahankan; tanpa library DnD, tanpa dependency baru, tanpa `console.log`, tanpa unit test.
- Guard: `cd frontend && npm run check && npm run build` sukses (tanpa galat baru di berkas fitur ini), dan `grep -rni "webgl" frontend/src/modules/layout` tidak menemukan impor addon WebGL.
- Jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
