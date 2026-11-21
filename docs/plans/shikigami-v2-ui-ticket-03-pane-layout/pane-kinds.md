# Plan: Adapter pane terminal dan pane file

## Metadata

- Tiket: `shikigami-v2-ui-ticket-03-pane-layout` (`docs/tickets/shikigami-v2-ui-ticket-03-pane-layout.md`)
- Fitur: `pane-kinds` (Adapter pane terminal dan pane file)
- Overview (kontrak bersama, jangan diulang di sini): `docs/plans/shikigami-v2-ui-ticket-03-pane-layout/overview.md`, terutama bagian 2.5, 2.6, 2.9, 4, 6, 7
- ownedPaths:
  - `frontend/src/modules/layout/panes/`
  - `frontend/src/modules/layout/index.ts`
  - `frontend/src/modules/terminal/` (hanya penambahan opsi)
- Depends on foundation: ya (`layout-model`, plan `00-foundation.md`). Foundation sudah menyediakan `modules/layout/model.ts` (tipe `Leaf`), `modules/layout/registry.ts` (`registerPaneKind`, `paneKinds`, `getPaneKind`), dan STUB `modules/layout/panes/hosts.ts` yang harus ditimpa penuh oleh fitur ini dengan signature ekspor yang sama.

## Tujuan

1. Mendaftarkan jenis pane `terminal` dan `file` ke registri jenis pane.
2. Mengimplementasikan penuh `panes/hosts.ts`: host DOM stabil per pane, komponen jenis pane di-mount SEKALI, props (`pane`, `visible`) diperbarui reaktif tanpa re-mount, dan di-unmount hanya saat id pane hilang dari pohon.
3. Memperluas `TerminalTab` dengan props opsional `scrollback`, `visible`, `fontSize` (default menjaga perilaku v1). `visible=false` menjeda fit dan menahan penulisan output (antrian) tanpa menghancurkan xterm atau attach sesi.
4. Memastikan tidak ada import WebGL di `modules/terminal` dan `modules/layout`.

## Path yang dimiliki

HANYA path berikut yang boleh disentuh:

- `frontend/src/modules/layout/panes/` (berkas baru dan penimpaan stub `hosts.ts`)
- `frontend/src/modules/layout/index.ts` (baru)
- `frontend/src/modules/terminal/` (hanya `TerminalTab.svelte`; tambah opsi saja)

Dilarang menyentuh: `model.ts`, `registry.ts`, `LayoutArea.svelte`, `Splitter.svelte`, `shortcuts.ts`, `PaneHeader.svelte`, `PaneMenu.svelte`, `dnd.ts`, `DropOverlay.svelte`, `lib/stores/*` (termasuk `layout.ts`, `tabs.ts`, `sessions.ts`), `App.svelte`, `modules/shell/`, `modules/tabs/`, `modules/files/`, `modules/status/`, `modules/appmenu/`, `modules/zoom/`, `main.ts`, `package.json`, kode Go. Jangan menambah dependency npm, unit test, atau `console.log`.

## Kontrak

### Disediakan (final, dipakai `layout-renderer` dan wiring)

`frontend/src/modules/layout/panes/hosts.ts` (signature persis overview 2.6):

```ts
export function ensureHost(leaf: Leaf, visible: boolean): HTMLElement;
export function getHost(paneId: string): HTMLElement | null;
export function attachHost(paneId: string, slot: HTMLElement): void;
export function detachHost(paneId: string): void;
export function releaseHost(paneId: string): void;
export function syncPanes(leaves: Leaf[], visibleIds: ReadonlySet<string>): void;
```

Pendaftaran jenis pane (lewat `registerPaneKind` dari `../registry`, bentuk `PaneKindSpec` overview 2.5):

| kind | component | titleFor | badgeFor | onEndSession |
| --- | --- | --- | --- | --- |
| `terminal` | `TerminalPane.svelte` | nama provider, fallback `leaf.title`, fallback `'Terminal'` | `CL` / `CX` / `SH` dari provider | `killSession(sessionId)` |
| `file` | `FilePane.svelte` | basename `filePath`, fallback `leaf.title` | tidak ada (`null`) | tidak ada |

`TerminalTab.svelte` menerima props opsional baru (overview 2.9): `scrollback?: number` (bawaan 2000), `visible?: boolean` (bawaan `true`), `fontSize?: number`.

### Dipakai

- `../model`: tipe `Leaf`. `../registry`: `registerPaneKind`, `getPaneKind`, tipe `PaneKindSpec` (dari foundation).
- `svelte`: `mount`, `unmount`.
- `frontend/src/lib/stores`: `killSession`, `sessions`, `providers` (impor dari `../../../lib/stores/sessions` dan `../../../lib/stores/providers` agar tidak membawa efek samping lain; keduanya ada).
- `modules/files/editor/FileTab.svelte` (props `refId`: string JSON `{"path","projectId","workspaceId"}`, urutan kunci TETAP) dan `modules/files/shared/shellBridge.ts#decodeFileTab` (hanya dibaca, tidak diubah).
- `modules/status/*`: TIDAK dipakai (titik status ada di header, bukan di isi pane).
- Zoom (tiket 02): `modules/terminal/font-scale.ts` SUDAH ada (`currentTerminalFontSize`, `registerTerminalInstance`). Pertahankan apa adanya. Pane terminal TIDAK mengirim `fontSize`; zoom tetap ditangani modul tiket 02. Jangan menambah handler `menu:zoom`.

## Langkah

Jalankan berurutan. Baca `TerminalTab.svelte`, stub `hosts.ts`, dan `registry.ts` dari foundation sebelum menulis.

### 1. Perluas `modules/terminal/TerminalTab.svelte` (hanya penambahan opsi)

Perilaku tanpa props baru HARUS identik dengan v1 (scrollback 2000, fit + resize saat mount, `ResizeObserver` debounce 100 ms, `markEnded`, registrasi font-scale). Perubahan:

1. Props:
   ```ts
   let { sessionId, scrollback = 2000, visible = true, fontSize }:
     { sessionId: string; scrollback?: number; visible?: boolean; fontSize?: number } = $props();
   ```
2. `new Terminal({... scrollback, fontSize: fontSize ?? currentTerminalFontSize(), ...})`. Tanpa import addon WebGL (hanya `FitAddon`).
3. Buka xterm secara tertunda hingga host benar-benar terpasang di DOM. Alasan: `hosts.ts` me-mount komponen ke `div` yang belum menjadi anak dokumen, dan xterm mengukur ukuran sel saat `open`. Implementasi:
   - Variabel `let opened = false;` dan fungsi `tryOpen()`: bila `!opened && host.isConnected && visibleNow`, ambil warna tema dari `getComputedStyle(host)` (fallback sama seperti v1), set `host.style.background`, set `term.options.theme`, panggil `term.open(host)`, `opened = true`, lalu `safeFit()`.
   - Panggil `tryOpen()` saat mount (pada pemakaian v1 host sudah terhubung dan `visible` true sehingga terbuka langsung seperti sebelumnya) dan dari callback `ResizeObserver` (host yang dipindah/ditempel mengubah ukuran sehingga observer menyala).
   - `term.write` sebelum `open` aman di xterm; tetap diarahkan lewat `emit` (butir 4).
   - `safeFit` hanya berjalan bila `opened`.
4. Antrian tulis saat tersembunyi. Semua penulisan ke xterm (scrollback hasil attach, event output sesudah offset attach, pesan `markEnded`) dialihkan lewat fungsi `emit(chunk: Uint8Array | string)`:
   - `visibleNow` true: `term.write(chunk)` langsung.
   - `visibleNow` false: dorong ke `queue` (urutan terjaga). Beri batas total byte (mis. 8 MiB); bila terlampaui buang potongan tertua (scrollback hanya 1000 baris, jadi aman) agar memori terbatas.
   - Logika `pending`/`attachOffset` v1 (subscribe sebelum attach, filter `ev.offset > attachOffset`) TIDAK diubah, hanya `term.write(...)` diganti `emit(...)`.
5. Kendali `visible`:
   - Deklarasikan di luar `onMount`: `let visibleNow = visible;` dan `let applyVisible: (v: boolean) => void = () => {};`.
   - `$effect(() => { applyVisible(visible); });` (membaca prop `visible` agar reaktif).
   - Di dalam `onMount`, definisikan `applyVisible = (v) => {...}`:
     - `v === visibleNow`: tidak melakukan apa pun.
     - ke `false`: set `visibleNow = false`; batalkan `timer` debounce resize. Jangan dispose, jangan unsubscribe, jangan detach sesi.
     - ke `true`: set `visibleNow = true`; `tryOpen()`; flush `queue` berurutan ke `term.write` lalu kosongkan; `safeFit()` SEKALI; `sendResize()`; `term.refresh(0, term.rows - 1)` (dibungkus try/catch).
   - Callback `ResizeObserver`: bila `!visibleNow` langsung return (fit dan resize dijeda); selain itu `tryOpen()`, lalu perilaku v1 (`safeFit`, debounce `sendResize` 100 ms).
   - `registerTerminalInstance(term, refit)`: `refit` hanya menjalankan `safeFit` + `sendResize` bila `visibleNow && opened`.
6. `fontSize` reaktif: `$effect(() => { if (fontSize !== undefined) { setFontSize(fontSize); } });` dengan `setFontSize` terdefinisi di `onMount` (set `term.options.fontSize`, lalu refit bila `visibleNow && opened`). Bila `fontSize` tidak diberikan, jangan menimpa nilai zoom tiket 02.
7. Cleanup `onMount` tetap seperti v1 (dispose, unsubscribe, `ro.disconnect`, `clearTimeout`, `unregisterFont`); kosongkan `queue`.
8. CSS tetap; tidak ada perubahan gaya yang mengubah tampilan v1.
9. `modules/terminal/index.ts` dan `font-scale.ts` TIDAK diubah.

### 2. `panes/PaneHostRoot.svelte` (baru): wrapper reaktif

Satu komponen kecil yang menjadi akar mount tiap pane agar props bisa berubah tanpa re-mount.

- Props awal (lewat `mount`): `{ component: Component<{ pane: Leaf; visible: boolean }>; pane: Leaf; visible: boolean }`.
- State internal `$state`: `cur = { pane, visible }`.
- Ekspor fungsi instans: `export function update(pane: Leaf, visible: boolean) { cur = { pane, visible }; }` (diakses dari nilai kembalian `mount`).
- Markup: `<div class="pane-root"><Comp pane={cur.pane} visible={cur.visible} /></div>` dengan `const Comp = component` (komponen dinamis Svelte 5, tanpa `svelte:component`). CSS `.pane-root { flex: 1; min-height: 0; min-width: 0; display: flex; flex-direction: column; position: relative; }` dan `.pane-root > :global(*) { flex: 1; min-height: 0; }`.

### 3. `panes/TerminalPane.svelte` (baru)

- `let { pane, visible }: { pane: Leaf; visible: boolean } = $props();`
- Bila `pane.sessionId` ada: `<TerminalTab sessionId={pane.sessionId} {visible} scrollback={1000} />` (impor `../../terminal/TerminalTab.svelte`). JANGAN mengirim `fontSize`.
- Bila tidak ada `sessionId`: teks "Sesi tidak tersedia" dengan gaya `--fg-muted`.
- `TerminalTab` hanya membaca `sessionId` sekali saat mount; itu aman karena `sessionId` leaf tidak berubah selama id pane yang sama hidup.

### 4. `panes/FilePane.svelte` (baru)

- `let { pane }: { pane: Leaf; visible: boolean } = $props();` (`visible` diterima agar sesuai kontrak; `FileTab` memantau keterlihatan sendiri lewat `IntersectionObserver`, jadi pane `display:none` otomatis jeda).
- `const refId = $derived(JSON.stringify({ path: pane.filePath ?? '', projectId: pane.projectId ?? '', workspaceId: pane.workspaceId ?? '' }));` urutan kunci TETAP `path`, `projectId`, `workspaceId`.
- Render `<FileTab {refId} />` (impor `../../files/editor/FileTab.svelte`) bila `pane.filePath` ada; selain itu teks "File tidak tersedia". Jangan mengirim props lain (FileTab membaca `props.refId` lewat `decodeFileTab`).
- Karena `refId` berupa string yang nilainya sama, memperbarui props leaf (mis. ganti nama) tidak memicu muat ulang berkas dan perubahan belum tersimpan tidak hilang.

### 5. `panes/terminal.ts` (baru)

```ts
import { get } from 'svelte/store';
import { registerPaneKind } from '../registry';
import { killSession } from '../../../lib/stores/sessions';
import { sessions } from '../../../lib/stores/sessions';
import { providers } from '../../../lib/stores/providers';
import TerminalPane from './TerminalPane.svelte';
```

- Helper `providerIdOf(leaf)`: `leaf.provider ?? get(sessions).find(s => s.id === leaf.sessionId)?.providerId`.
- `titleFor`: nama dari `get(providers).find(p => p.id === providerIdOf(leaf))?.name`, fallback `leaf.title`, fallback `'Terminal'`.
- `badgeFor`: peta `claude` -> `CL`, `codex` -> `CX`, `shell` -> `SH`; id lain: dua huruf pertama kapital; tanpa provider: `null`.
- `onEndSession: async (leaf) => { if (leaf.sessionId) await killSession(leaf.sessionId); }`. Menutup pane TIDAK memanggil ini. Keadaan "[sesi berakhir]" dimunculkan `TerminalTab` sendiri lewat `subscribeExit`.
- `registerPaneKind({ kind: 'terminal', component: TerminalPane, titleFor, badgeFor, onEndSession })`.
- Jika `killSession` diekspor dari satu berkas yang sama dengan `sessions`, gabungkan menjadi satu `import`.

### 6. `panes/file.ts` (baru)

- `titleFor`: basename dari `leaf.filePath` (pisah `/` dan `\`, ambil segmen terakhir tidak kosong), fallback `leaf.title`, fallback `'File'`.
- Tanpa `badgeFor` dan tanpa `onEndSession`.
- `registerPaneKind({ kind: 'file', component: FilePane, titleFor })`.

### 7. `panes/hosts.ts` (TIMPA stub foundation sepenuhnya)

Hapus komentar `STUB(foundation)`. Struktur:

```ts
interface Entry {
  host: HTMLElement;
  kind: string;
  instance: { update(pane: Leaf, visible: boolean): void } | null; // dari mount(PaneHostRoot)
  unmountFn: (() => void) | null;
  pane: Leaf;
  visible: boolean;
}
const entries = new Map<string, Entry>();
```

- `ensureHost(leaf, visible)`: bila entri ada, panggil `updateEntry` (butir di bawah) dan kembalikan `entry.host` (identitas elemen sama). Bila belum: buat `div.pane-host` dengan `style="width:100%;height:100%;min-height:0;display:flex;flex-direction:column"`, simpan entri, lalu `mountInto(entry)`.
- `mountInto(entry)`: `getPaneKind(entry.kind)`. Bila terdaftar: `const inst = mount(PaneHostRoot, { target: entry.host, props: { component: spec.component, pane: entry.pane, visible: entry.visible } })`; simpan `instance = inst` (yang punya `update`) dan `unmountFn = () => unmount(inst)`. Bila tidak terdaftar: isi `entry.host.textContent = 'Jenis pane tidak dikenal'` (gaya `--fg-muted`, padding) dan `instance = null`.
- `updateEntry(entry, leaf, visible)`: set `entry.pane = leaf`, `entry.visible = visible`; jika `instance` null dan kind kini terdaftar (registrasi datang terlambat) bersihkan `host.textContent` lalu `mountInto`; jika `instance` ada panggil `instance.update(leaf, visible)`. Jika `leaf.kind` berubah untuk id yang sama: `unmountFn()`, kosongkan host, `mountInto` ulang.
- `getHost(id)`: `entries.get(id)?.host ?? null`.
- `attachHost(id, slot)`: bila entri ada dan `host.parentElement !== slot`, `slot.appendChild(host)`. Tidak menghancurkan apa pun. Jangan mengosongkan `slot`.
- `detachHost(id)`: `host.remove()` saja (entri dan komponen tetap hidup).
- `releaseHost(id)`: `unmountFn?.()`, `host.remove()`, `entries.delete(id)`.
- `syncPanes(leaves, visibleIds)`: untuk tiap leaf `ensureHost(leaf, visibleIds.has(leaf.id))` (sudah memperbarui props); lalu untuk tiap id di `entries` yang tidak ada di `leaves` panggil `releaseHost`.
- Semua operasi idempoten dan tidak melempar (bungkus `mount`/`unmount` dengan try/catch; kegagalan `mount` mengisi host dengan teks galat singkat, bukan `console.log`).

### 8. `modules/layout/index.ts` (baru)

Hanya dua baris impor efek-samping (dimuat otomatis oleh glob `main.ts`; JANGAN edit `main.ts`):

```ts
import './panes/terminal';
import './panes/file';
```

Tanpa side effect lain, tanpa ekspor tambahan.

### 9. Verifikasi

1. `cd frontend && npm run check && npm run build`. Bila `check` punya galat bawaan sebelum epic, bandingkan; tidak boleh ada galat baru di berkas milik fitur ini.
2. `grep -rni "webgl" frontend/src/modules/terminal frontend/src/modules/layout` tidak menemukan import addon WebGL.
3. `grep -rn "console.log" frontend/src/modules/layout/panes frontend/src/modules/layout/index.ts frontend/src/modules/terminal` kosong.
4. Periksa `git diff` pada `modules/terminal/` (bila repo git tersedia di worktree) hanya berisi penambahan opsi; `font-scale.ts` dan `index.ts` tidak berubah.
5. Cek mental (tanpa unit test): `TerminalTab sessionId` saja berperilaku v1; `visible=false` lalu `true` mengalirkan antrian berurutan, fit sekali, `refresh`; `ensureHost` dua kali dengan leaf sama mengembalikan elemen identik; `syncPanes` tanpa suatu id memanggil `releaseHost` untuk id itu.

## Perubahan

Dibuat:
- `frontend/src/modules/layout/panes/PaneHostRoot.svelte`
- `frontend/src/modules/layout/panes/TerminalPane.svelte`
- `frontend/src/modules/layout/panes/FilePane.svelte`
- `frontend/src/modules/layout/panes/terminal.ts`
- `frontend/src/modules/layout/panes/file.ts`
- `frontend/src/modules/layout/index.ts`

Ditimpa penuh (stub foundation):
- `frontend/src/modules/layout/panes/hosts.ts`

Diubah (hanya penambahan opsi):
- `frontend/src/modules/terminal/TerminalTab.svelte`

## Wiring

Yang diekspos untuk disambungkan nanti oleh `layout-renderer` dan `layout-wiring` (JANGAN disambungkan oleh fitur ini):

- `panes/hosts.ts`: `ensureHost`, `getHost`, `attachHost`, `detachHost`, `releaseHost`, `syncPanes`. `LayoutArea` memanggil `syncPanes(leaves, visibleIds)` pada tiap perubahan pohon dan `attachHost(id, slot)` untuk slot tiap leaf; pane yang disembunyikan karena maksimalkan tidak ada di `visibleIds`.
- Jenis pane `terminal` dan `file` terdaftar otomatis begitu `modules/layout/index.ts` dimuat oleh `import.meta.glob` di `main.ts` (langkah wiring 1 hanya memastikan ini; tidak ada perubahan `main.ts` atau `lib/registry.ts`).
- `PaneKindSpec.onEndSession` (terminal = `killSession`) dipanggil oleh callback `onEndSession` yang disusun `LayoutArea`; `badgeFor`/`titleFor` dipakai `PaneHeader` dan model.
- `TerminalTab` tetap terdaftar sebagai tab type `session` oleh `modules/terminal/index.ts` (tidak diubah) sehingga tetap valid dipakai tanpa opsi baru.

## Selesai bila

- `paneKinds()` memuat `terminal` dan `file` setelah `modules/layout/index.ts` dimuat.
- `hosts.ts` mengimplementasikan seluruh enam fungsi: host stabil (elemen sama untuk id sama), mount sekali, props `pane`/`visible` reaktif tanpa re-mount, `releaseHost` hanya untuk id yang hilang, kind tak dikenal menampilkan "Jenis pane tidak dikenal".
- Pane terminal memakai `scrollback: 1000`; `TerminalTab` tanpa props baru tetap scrollback 2000 dan perilaku v1 lain.
- `visible=false` menjeda fit/resize dan menahan penulisan, tanpa dispose xterm atau memutus attach; saat `true`: flush antrian, `fit` sekali, `refresh`; output yang datang saat tersembunyi tampil utuh.
- `FilePane` meneruskan `refId` JSON dengan urutan kunci `path`, `projectId`, `workspaceId`.
- Tidak ada import WebGL, tidak ada `console.log`, tidak ada dependency baru, tidak ada berkas di luar ownedPaths yang berubah.
- Guard: `cd frontend && npm run check && npm run build` sukses (tanpa galat baru di path fitur ini) dan `grep -rni "webgl" frontend/src/modules/terminal frontend/src/modules/layout` bersih. Tanpa unit test; jangan commit/merge sendiri kecuali alur `implement-feature` meminta.
