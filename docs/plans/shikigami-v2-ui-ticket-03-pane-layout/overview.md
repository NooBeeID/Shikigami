# Overview: Layout multi-pane (split, drag and drop, resize)

Tiket: `docs/tickets/shikigami-v2-ui-ticket-03-pane-layout.md`
Spec: `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md` (bagian 5, 8)
Frontend: Svelte 5 (runes) + Vite + TypeScript di `frontend/`, xterm 6. Epic ini 100% frontend (tanpa kode Go/daemon, tanpa dependency npm baru).

## 1. Ringkasan epic dan urutan eksekusi

Area tab v1 (`TabBar`/`TabContent`) diganti pohon pane: banyak terminal dan file terlihat bersamaan, split (Cmd+D / Cmd+Shift+D), drag header ke zona drop, seret pembatas, maksimalkan, susun grid, menu `...` pada header. Anti-goal utama: memindah/split pane TIDAK boleh membuat ulang xterm atau memutus attach sesi; yang dipindah adalah elemen host DOM yang sama. Epic ditutup dengan laporan perf 6 pane (`docs/perf/multipane-report.md`).

Urutan:

1. **Foundation** `layout-model` (plan: `00-foundation.md`): `model.ts`, `registry.ts`, `lib/stores/layout.ts`, ditambah stub antarmuka (bagian 4) agar fitur paralel bisa build sendiri-sendiri. Digabung lebih dulu.
2. **Fitur paralel** (tidak berbagi file): `layout-renderer`, `pane-header-dnd`, `pane-kinds`.
3. **Wiring** berurutan: `layout-wiring` lalu `perf-6-pane`.

Tiap fitur paralel harus lolos build guard sendiri walau fitur lain masih berupa stub dari foundation.

## 2. Kontrak bersama

### 2.1 Fakta repo v1 yang dipakai (sudah dibaca, jangan diubah)

- `frontend/src/main.ts` memuat `./modules/*/index.ts` lewat `import.meta.glob` (eager). Modul baru cukup punya `index.ts`; JANGAN edit `main.ts`.
- `lib/registry.ts`: `registerPanel`, `registerTabType` (tidak diubah). Registri baru `registerPaneKind` ada di `modules/layout/registry.ts`.
- `lib/stores/tabs.ts` (BUKAN milik epic ini, tidak boleh diedit): `tabs`, `activeTabId`, `openTab({type,refId,title})` (id tab = `${type}:${refId}`), `activateTab(id)`, `closeTab(id)`. Tipe `Tab = {id, type:'session'|'file', title, refId}`.
- `lib/stores/sessions.ts`: `sessions` (writable `Session[]`), `createSession(providerId)` (memakai `activeWorkspaceId` + `activeProject`, memanggil `call('session.create', {workspaceId, projectId, cwd, providerId})`, lalu menambah ke `sessions`), `killSession(id)` (`call('session.kill', {id})`, menandai `exited`).
- `lib/stores/types.ts`: `Session {id, workspaceId, projectId, cwd, providerId, status, createdAt, exitCode?}`, `Project {id, path, name}`, `Provider {id, name, available?}`. `lib/stores/workspaces.ts`: `activeWorkspace`, `activeWorkspaceId`, `activeProject`, `activeProjectId`.
- `modules/terminal/TerminalTab.svelte`: prop saat ini `{ sessionId }`; xterm dibuat di `onMount`, `scrollback: 2000`, `fontSize: 13`, `ResizeObserver` memanggil fit + `resizeSession` (debounce 100 ms), `markEnded()` menulis "[sesi berakhir]". Root-nya `.term-host` (width/height 100%).
- `modules/files/editor/FileTab.svelte`: menerima props bebas dan memanggil `decodeFileTab(props)` yang membaca `props.refId` (string JSON `{"path","projectId","workspaceId"}` dengan urutan kunci TETAP). `modules/files/shared/shellBridge.ts#openFileTab` memakai `openTab` dengan `refId` JSON itu.
- Status: `import StatusBadge from '../status/StatusBadge.svelte'` (props `{sessionId, compact?}`); `statusOf(sessionId)` dari `../status/store`.
- Sidebar v1 (`NewSession.svelte`, `SessionList.svelte`, `files/tree`) memanggil `openTab`/`openFileTab`; `SessionList` menyorot lewat `activeTabId === 'session:'+id`. Tidak boleh diedit; dijembatani shim (bagian 5).
- Tema: variabel `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok --term-bg --term-fg` di `styles/theme.css`. Gunakan variabel itu, jangan hardcode warna (fallback boleh).
- Kontrak tiket 02 (`appmenu`): event `menu:zoom` ditangani SATU kali oleh modul zoom tiket 02; tiket ini tidak menambah handler `menu:zoom`. Bila tiket 02 sudah menambahkan `registerTerminalInstance`/`currentTerminalFontSize` ke `TerminalTab`, pertahankan apa adanya (lihat `pane-kinds`). Dilarang memakai Cmd+O, Cmd+Shift+O, Cmd+=, Cmd+-, Cmd+0, Cmd+B, Cmd+Option+B, Cmd+K. Pintasan epic ini hanya Cmd+D dan Cmd+Shift+D.

### 2.2 Tipe (`frontend/src/modules/layout/model.ts`, persis seperti tiket)

```ts
export type PaneKind = 'terminal' | 'file';   // string terbuka lewat registerPaneKind
export interface Leaf { type: 'pane'; id: string; kind: PaneKind; sessionId?: string; projectId?: string;
  provider?: string; filePath?: string; title: string; workspaceId?: string /* tambahan: untuk pane file */ }
export interface Split { type: 'split'; dir: 'h' | 'v'; ratio: number; a: Node; b: Node }   // h: a kiri/b kanan; v: a atas/b bawah
export type Node = Leaf | Split;
export interface LayoutDoc { version: 1; root: Node | null; focusId?: string; maximizedId?: string | null }
export type DropZone = 'top' | 'bottom' | 'left' | 'right' | 'center';
```

Catatan: `workspaceId?: string` adalah satu-satunya tambahan terhadap tiket (opsional, diperlukan agar pane file bisa membentuk `refId` JSON). `PaneKind` tetap `'terminal' | 'file'` pada tipe; `Leaf.kind` boleh diisi string lain secara runtime (renderer memakai fallback "Jenis pane tidak dikenal").

Konvensi: id leaf `p_<8+ hex acak>` (dibuat sekali oleh model, `crypto.randomUUID()` dipotong atau penghitung; tidak berubah saat dipindah). `ratio` di-clamp `[0.1, 0.9]` (`clampRatio`). Rujukan simpul Split = jalur string dari root: `''` = root, `'a'`, `'b'`, `'a.b'`, dst. (`SplitPath`).

### 2.3 Fungsi murni model (`model.ts`, tidak memutasi masukan, mengembalikan dokumen baru)

```ts
export type SplitPath = string;
export interface NewLeaf { kind: PaneKind; sessionId?: string; projectId?: string; provider?: string;
  filePath?: string; workspaceId?: string; title?: string }
export const emptyDoc: () => LayoutDoc;
export const clampRatio: (r: number) => number;
export const leaves: (n: Node | LayoutDoc | null) => Leaf[];            // urutan baca kiri-atas ke kanan-bawah (a sebelum b)
export const findLeaf: (doc: LayoutDoc, id: string) => Leaf | null;
export const pathOf: (doc: LayoutDoc, id: string) => SplitPath | null;  // jalur ke leaf ('' bila leaf adalah root)
export const findLeafBy: (doc: LayoutDoc, pred: (l: Leaf) => boolean) => Leaf | null;
export function openPane(doc, leaf: NewLeaf, opts?: { dir?: 'h' | 'v' }): { doc: LayoutDoc; id: string };
export function splitPane(doc, paneId: string, dir: 'h' | 'v', newLeaf: NewLeaf | Leaf, side?: 'after' | 'before'): { doc: LayoutDoc; id: string };
export function closePane(doc, paneId: string): LayoutDoc;
export function movePane(doc, paneId: string, targetId: string, zone: DropZone): LayoutDoc;
export function swapPanes(doc, aId: string, bId: string): LayoutDoc;
export function resize(doc, path: SplitPath, ratio: number): LayoutDoc;
export function maximize(doc, paneId: string): LayoutDoc;
export function restore(doc): LayoutDoc;
export function tidyGrid(doc): LayoutDoc;
export function setFocus(doc, paneId: string): LayoutDoc;
export function renamePane(doc, paneId: string, title: string): LayoutDoc;
export function hydrateDoc(raw: unknown): LayoutDoc;     // validasi, buang leaf tidak valid, tidak melempar
```

Aturan yang dipegang semua pemakai:
- `openPane`: sesi (`sessionId`) atau file (`filePath` + `projectId`) yang sama sudah ada, fokuskan dan kembalikan id lama. Selain itu sisipkan di sebelah pane fokus (split `h` after; `opts.dir === 'v'` split bawah); pohon kosong, jadi root. Pane baru difokuskan.
- `closePane`: saudara menggantikan induk Split; tidak ada Split berkaki satu; bersihkan `maximizedId` bila pane itu; fokus pindah ke saudara terdekat; TIDAK ada IPC.
- `movePane`: `paneId === targetId` dilarang (kembalikan doc apa adanya); `center` = `swapPanes`; sisi = cabut leaf lalu bagi target di sisi itu; `maximizedId` dipulihkan dulu.
- `tidyGrid`: `cols = ceil(sqrt(n))`, baris diisi urut baca (6 jadi 3x2, 4 jadi 2x2, 5 jadi 3+2); baris = rantai Split `v` dengan ratio `1/(rows-i)`, isi baris = rantai Split `h` dengan ratio `1/(cols_baris-i)`; id leaf dipertahankan; fokus dipertahankan; `maximizedId` dibersihkan.
- Semua keluaran JSON murni.

### 2.4 Store (`frontend/src/lib/stores/layout.ts`)

```ts
export interface LayoutStore extends Readable<LayoutDoc> {
  openPane(leaf: NewLeaf, opts?: { dir?: 'h' | 'v' }): string;
  splitPane(paneId: string, dir: 'h' | 'v', newLeaf: NewLeaf | Leaf, side?: 'after' | 'before'): string;
  closePane(paneId: string): void;
  movePane(paneId: string, targetId: string, zone: DropZone): void;
  swapPanes(aId: string, bId: string): void;
  resize(path: SplitPath, ratio: number): void;
  maximize(paneId: string): void;
  restore(): void;
  tidyGrid(): void;
  setFocus(paneId: string): void;
  renamePane(paneId: string, title: string): void;
  hydrate(doc: unknown): void;       // tidak melempar
  serialize(): LayoutDoc;            // salinan JSON murni
  readonly paneCount: Readable<number>;
  readonly leaves: Readable<Leaf[]>; // urut baca, untuk nomor pane 1..n
}
export function createLayoutStore(initial?: LayoutDoc): LayoutStore;
export const layout: LayoutStore;    // instans bawaan, grup "Utama"
```

Store hanya pembungkus `writable<LayoutDoc>` yang memanggil fungsi murni model. Komponen menerima `store` (bawaan `layout`) sebagai prop opsional agar group-ready.

### 2.5 Registri jenis pane (`modules/layout/registry.ts`, persis tiket)

```ts
export interface PaneKindSpec {
  kind: string;
  component: Component<{ pane: Leaf; visible: boolean }>;
  titleFor?: (leaf: Leaf) => string;
  onEndSession?: (leaf: Leaf) => Promise<void>;
  badgeFor?: (leaf: Leaf) => string | null;
}
export function registerPaneKind(spec: PaneKindSpec): void;
export function paneKinds(): Readable<Record<string, PaneKindSpec>>;
```

### 2.6 Host DOM stabil (`modules/layout/panes/hosts.ts`)

Isi pane di-mount SEKALI ke elemen host milik pane dan di-unmount hanya saat id hilang dari pohon. API (final; stub dibuat foundation, implementasi penuh oleh `pane-kinds`, dipakai `layout-renderer`):

```ts
export function ensureHost(leaf: Leaf, visible: boolean): HTMLElement;     // buat host + mount komponen jenis pane bila belum ada; kembalikan host yang sama
export function getHost(paneId: string): HTMLElement | null;
export function attachHost(paneId: string, slot: HTMLElement): void;      // slot.appendChild(host) bila belum menjadi anak slot; TIDAK menghancurkan
export function detachHost(paneId: string): void;                          // lepas dari slot (host.remove()), tetap hidup
export function releaseHost(paneId: string): void;                         // unmount + hapus dari Map
export function syncPanes(leaves: Leaf[], visibleIds: ReadonlySet<string>): void;
// ensureHost untuk setiap leaf, perbarui prop (leaf terbaru + visible), releaseHost untuk id yang tidak ada lagi
```

Props komponen harus reaktif tanpa re-mount (mis. wrapper `PaneHostRoot.svelte` di `panes/` dengan `$state`, atau `mount` dengan props `$state`). Kind tak terdaftar: host berisi teks "Jenis pane tidak dikenal". Host diberi kelas `pane-host` dan `style="width:100%;height:100%;min-height:0;display:flex;flex-direction:column"`.

### 2.7 DOM, drag, dan overlay (kontrak antara renderer dan header)

- `LayoutArea` memberi tiap leaf elemen pembungkus dengan `data-pane-id="<id>"`; di dalamnya header (`PaneHeader`) lalu slot isi. Header diberi atribut `data-pane-header` pada elemen akarnya.
- Props `PaneHeader` (final): `{ pane: Leaf; index: number; focused: boolean; maximized: boolean; store?: LayoutStore; onSplit: (dir: 'h'|'v') => void; onMaximizeToggle: () => void; onClose: () => void; onRename: (title: string) => void; onEndSession: () => void; onTidy: () => void }`. `index` berbasis 1. Header memakai `store` (bawaan `layout`) hanya untuk drag.
- `dnd.ts` (final):

```ts
export interface DragState { paneId: string; title: string; targetId: string | null; zone: DropZone | null;
  rect: { x: number; y: number; w: number; h: number } | null /* rect viewport pane target */ }
export const dragState: Readable<DragState | null>;
export function zoneAt(rect: { x: number; y: number; w: number; h: number }, x: number, y: number): DropZone;
export function startPaneDrag(e: PointerEvent, paneId: string, title: string, store?: LayoutStore): void;
```

  `startPaneDrag` dipanggil dari `pointerdown` header; ia sendiri menerapkan ambang 4px, `setPointerCapture`, ghost, `document.elementFromPoint` + `closest('[data-pane-id]')`, Esc batal, lepas memanggil `store.movePane`. Selama drag ia menambah kelas `pane-dragging` pada `document.body`; renderer menyediakan CSS global `body.pane-dragging .pane-host { pointer-events: none }`.
- `DropOverlay.svelte` tanpa props, membaca `dragState`, menggambar sorotan `position: fixed` dari `rect`+`zone` (sisi = setengah pane, center = seluruh pane), `pointer-events: none`. `LayoutArea` merender SATU `<DropOverlay />`.
- Klik ganda: header menangani klik ganda judul (rename) dan klik ganda area lain (`onMaximizeToggle`).

### 2.8 Pintasan (`shortcuts.ts`, final)

```ts
export function installLayoutShortcuts(store?: LayoutStore): () => void;   // pasang keydown capture di window; kembalikan fungsi lepas
export function splitFocused(dir: 'h' | 'v', store?: LayoutStore): Promise<void>;
```

Cmd+D = `h` (kanan), Cmd+Shift+D = `v` (bawah). Pendengar memakai fase capture (`addEventListener('keydown', fn, true)`) dengan `preventDefault()` + `stopPropagation()` agar xterm tidak meneruskan ke shell. Abaikan bila fokus di `input`/`textarea`/`contenteditable` yang bukan textarea milik xterm (`.xterm-helper-textarea`). Sesi baru dibuat dengan project dan provider pane fokus lewat `call('session.create', {workspaceId, projectId, cwd, providerId})` (jalur `internal/provider` yang sama, flag izin agent diterapkan server; frontend tidak menambah argumen) lalu `sessions.update(...)` seperti `createSession` v1; project dicari di `activeWorkspace.projects` berdasarkan `projectId` pane fokus, fallback `createSession(providerId)` bila tidak ketemu. Tanpa pane fokus atau pane fokus bukan terminal: tidak melakukan apa pun. Kegagalan dilaporkan lewat `console.error` singkat (tanpa UI baru).

`LayoutArea.onMount` memanggil `installLayoutShortcuts(store)` dan melepasnya saat destroy.

### 2.9 Opsi `TerminalTab` (diperluas oleh `pane-kinds`)

Props opsional (default menjaga perilaku v1): `scrollback?: number` (bawaan 2000), `visible?: boolean` (bawaan true), `fontSize?: number`. `visible=false`: fit/`ResizeObserver` dijeda, penulisan output ke xterm di-batch (antrian) atau render dijeda, instance `Terminal` dan attach sesi TIDAK dihancurkan; saat true lagi: flush antrian, `fit` sekali, `term.refresh(0, rows-1)`. Tanpa addon WebGL. Bila tiket 02 sudah memasang `currentTerminalFontSize()`/`registerTerminalInstance`, prop `fontSize` hanya menimpa saat diberikan; pane terminal tidak mengirim `fontSize` (zoom tetap ditangani modul tiket 02).

### 2.10 Konvensi

- Folder: `frontend/src/modules/layout/` (+ `panes/`), `frontend/src/lib/stores/layout.ts`, `docs/perf/`.
- Nama berkas Svelte PascalCase; `.ts` kebab/lowerCamel seperti daftar di tiket. Identifier kode Inggris, teks UI/komentar Indonesia.
- Tanpa polling dan tanpa `setInterval`; `requestAnimationFrame`/`ResizeObserver`/`setTimeout` debounce boleh. Tanpa dependency baru. Hapus `console.log` sementara.
- Bahasa UI: Indonesia ("Akhiri sesi", "Ganti nama", "Susun grid", "Tutup pane", "Jenis pane tidak dikenal").

## 3. Daftar fitur

### layout-model (Foundation)
- Scope: tanpa UI. `model.ts` (tipe + fungsi murni bagian 2.2/2.3), `registry.ts` (2.5), `lib/stores/layout.ts` (2.4: `createLayoutStore`, `layout`). `hydrateDoc` memvalidasi bentuk, membuang leaf tidak valid, menormalisasi Split berkaki satu, tidak melempar. Ditambah stub antarmuka bagian 4 (dibuat foundation, diganti penuh oleh pemilik fitur).
- ownedPaths: `frontend/src/modules/layout/model.ts`, `frontend/src/modules/layout/registry.ts`, `frontend/src/lib/stores/layout.ts` (+ stub bagian 4 yang kemudian milik fitur masing-masing).
- Plan: `00-foundation.md`.

### layout-renderer
- Scope: `LayoutArea.svelte` (rekursif flex, `flex: ratio` / `flex: 1 - ratio`, `min-width/height: 0`, keadaan kosong, `data-pane-id`, slot per leaf yang diisi `attachHost`, `syncPanes` pada tiap perubahan pohon, maksimalkan lewat `display:none` + `visible=false` untuk pane lain, bingkai fokus `--accent`, klik memanggil `setFocus`, satu `<DropOverlay />`, CSS global `body.pane-dragging .pane-host`, memanggil `installLayoutShortcuts`), `Splitter.svelte` (4px + hit area lebih lebar, pointer events + `setPointerCapture`, rAF, klik ganda 0.5, kursor `col-resize`/`row-resize`; memanggil `store.resize(path, ratio)`), `shortcuts.ts` (bagian 2.8). Memakai `PaneHeader` stub/final lewat props 2.7; menyusun callback header dari store (`onSplit` memakai `splitFocused`-sejenis untuk pane itu, `onClose` = `closePane`, `onEndSession` memanggil `spec.onEndSession(leaf)` lalu tidak menutup pane, `onTidy` = `tidyGrid`, `onMaximizeToggle`, `onRename`).
- ownedPaths: `frontend/src/modules/layout/LayoutArea.svelte`, `frontend/src/modules/layout/Splitter.svelte`, `frontend/src/modules/layout/shortcuts.ts`.

### pane-header-dnd
- Scope: `PaneHeader.svelte`, `PaneMenu.svelte`, `dnd.ts`, `DropOverlay.svelte` sesuai tiket dan kontrak 2.7. Header: nomor, judul (klik ganda rename inline, Enter simpan, Esc batal, kosong membatalkan), lencana dari `badgeFor` (CL/CX/SH), `StatusBadge compact` untuk pane terminal, chip path project (nama project + subfolder relatif, ellipsis, tooltip path penuh; data dari `activeWorkspace`/`sessions`/`cwd` sesi), tombol split kanan/bawah, maksimalkan/pulihkan, tutup, `...`. `PaneMenu`: Akhiri sesi (hanya terminal, konfirmasi singkat), Ganti nama, Susun grid, Tutup pane; tutup oleh klik di luar dan Esc. `zoneAt` deterministik: titik dalam 25% tepi terdekat memberi sisi, sudut memilih sisi dengan jarak tepi terkecil (seri: urutan top, bottom, left, right), selain itu `center`.
- ownedPaths: `frontend/src/modules/layout/PaneHeader.svelte`, `frontend/src/modules/layout/PaneMenu.svelte`, `frontend/src/modules/layout/dnd.ts`, `frontend/src/modules/layout/DropOverlay.svelte`.

### pane-kinds
- Scope: `panes/terminal.ts` (`registerPaneKind` untuk `terminal`: `TerminalPane.svelte` membungkus `TerminalTab` dengan `sessionId`, `visible`, `scrollback: 1000`; `titleFor` = nama provider/judul sesi; `badgeFor` = CL/CX/SH dari `provider`; `onEndSession` = `killSession(sessionId)` dari `lib/stores`), `panes/file.ts` (`FilePane.svelte` membungkus `FileTab` dengan `refId = JSON.stringify({path, projectId, workspaceId})` urutan kunci tetap; `titleFor` = basename; tanpa titik status), `panes/hosts.ts` (implementasi penuh 2.6 + wrapper reaktif), `modules/layout/index.ts` (impor `./panes/terminal` dan `./panes/file` agar terdaftar; tanpa side effect lain), dan perluasan `modules/terminal/TerminalTab.svelte` (2.9). Tidak ada import WebGL di `modules/terminal` dan `modules/layout`.
- ownedPaths: `frontend/src/modules/layout/panes/`, `frontend/src/modules/layout/index.ts`, `frontend/src/modules/terminal/` (hanya penambahan opsi).

### layout-wiring (Wiring)
- Scope: lihat bagian 5.
- ownedPaths: `frontend/src/App.svelte`, `frontend/src/modules/tabs/`, `frontend/src/modules/shell/`.

### perf-6-pane (Wiring, terakhir)
- Scope: ukur RAM/CPU UI dengan 6 pane (prosedur `docs/perf/spike-report.md`), tulis `docs/perf/multipane-report.md` (bahasa Indonesia, semua bagian di tiket, angka nyata, label ESTIMASI bila perkiraan; hanya melaporkan, tidak memutuskan opsi UI native). Bersihkan proses/direktori sementara; data dir pengguna tidak disentuh.
- ownedPaths: `docs/perf/multipane-report.md`.

## 4. Foundation

Ada: `layout-model`. Alasan: tiga fitur paralel mengimpor tipe/store/registri dari `model.ts`, `registry.ts`, `lib/stores/layout.ts`, dan saling mengimpor antarmuka `hosts.ts`, `PaneHeader`, `dnd.ts`, `DropOverlay`. Agar masing-masing lolos build tanpa menunggu yang lain, foundation membuat STUB berikut dengan signature final (bagian 2.6/2.7). Pemilik fitur MENIMPA stub seluruhnya (bukan menambal) dan harus mempertahankan signature ekspor.

| Stub | Pemilik yang menimpa | Isi stub |
| --- | --- | --- |
| `modules/layout/panes/hosts.ts` | `pane-kinds` | ekspor 2.6; `ensureHost` membuat `div.pane-host` berisi teks placeholder, `attachHost`/`detachHost`/`releaseHost`/`syncPanes` sederhana tanpa mount komponen |
| `modules/layout/PaneHeader.svelte` | `pane-header-dnd` | props 2.7; menampilkan `{index} · {pane.title}` dan tombol tutup yang memanggil `onClose` |
| `modules/layout/dnd.ts` | `pane-header-dnd` | `dragState` (readable `null`), `zoneAt` (implementasi benar sederhana), `startPaneDrag` kosong |
| `modules/layout/DropOverlay.svelte` | `pane-header-dnd` | komponen kosong tanpa props |

`LayoutArea.svelte`, `Splitter.svelte`, `shortcuts.ts`, `PaneMenu.svelte`, `panes/terminal.ts`, `panes/file.ts`, `panes/*.svelte`, `index.ts` TIDAK dibuat foundation. `hasFoundation = true`; plan langkah ada di `00-foundation.md`.

## 5. Wiring checklist

Dikerjakan satu agent (`layout-wiring`) setelah foundation dan tiga fitur paralel tergabung. Satu-satunya berkas di luar ownedPaths yang tidak boleh disentuh: `lib/stores/tabs.ts`, `lib/stores/sessions.ts`, `modules/workspace/`, `modules/files/`, `lib/registry.ts`, `main.ts`, `package.json`.

1. Pastikan `modules/layout/index.ts` (dimuat otomatis oleh glob `main.ts`) mendaftarkan `terminal` dan `file`; `main.ts` dan `lib/registry.ts` tidak diubah.
2. `frontend/src/App.svelte`: hapus impor `TabBar` dan `TabContent`, impor `LayoutArea` dari `./modules/layout/LayoutArea.svelte`; di `<main class="main">` ganti `<TabBar /><div class="content"><TabContent /></div>` menjadi `<div class="content"><LayoutArea /></div>` (hapus bar tab). Perubahan sekecil mungkin; `modules/shell/` hanya disentuh bila ada rujukan langsung ke tab (`SidebarArea`/`StatusBar` saat ini tidak perlu diubah).
3. Shim tab-ke-pane di `frontend/src/modules/tabs/index.ts` (BARU, dimuat otomatis oleh glob; `lib/stores/tabs.ts` tidak diedit). Mekanisme: berlangganan store `tabs` dan `activeTabId` yang sudah ada.
   - Tab baru di `tabs` (id belum punya pane): `type 'session'` memanggil `layout.openPane({kind:'terminal', sessionId: refId, projectId/provider dari store sessions, title})`; `type 'file'` mendekode `refId` JSON (`{path, projectId, workspaceId}`) lalu `layout.openPane({kind:'file', filePath, projectId, workspaceId, title})`. `openPane` idempoten (pane yang sama hanya difokuskan).
   - `activeTabId` berubah ke tab yang pane-nya sudah ada: `layout.setFocus(paneId)`; bila pane belum ada: `openPane` seperti di atas.
   - Sinkron balik: berlangganan `layout`; fokus pane berubah menjadi `activeTabId.set(<tabId>)` (`session:<sessionId>` atau `file:<refId JSON>`); pane hilang dari pohon memanggil `closeTab(<tabId>)` agar klik ulang sesi di sidebar membuka lagi pane-nya (menutup pane TIDAK memanggil `killSession`).
   - Guard anti-loop (flag sedang-menyinkron) dan pemetaan `tabId` ke `paneId` lewat pencarian `sessionId`/`filePath` pada `leaves`.
   - Bila ada pemanggil `openTab` yang tidak bisa dijembatani, hentikan dan laporkan sebagai blocker; jangan menyunting berkas di luar ownedPaths.
   - `TabBar.svelte`, `TabContent.svelte`, `tab-title.ts` boleh dibiarkan (tidak dirender) atau dihapus; hapus bila tidak ada impor tersisa.
4. Sesi baru lewat `+ sesi` v1 (`NewSession` memanggil `openTab`) otomatis menjadi pane lewat shim; Cmd+D/Cmd+Shift+D memakai `shortcuts.ts` (sudah ada dari `layout-renderer`).
5. Zoom: jangan menambah handler `menu:zoom`; periksa apakah `TerminalTab` sudah memakai skala tiket 02 dan pastikan pane terminal ikut. Bila modul `appmenu` belum ada, `fontSize` tetap 13 tanpa error build.
6. Pastikan stub foundation sudah tertimpa semua (tidak ada teks placeholder tersisa di `hosts.ts`, `PaneHeader.svelte`, `dnd.ts`, `DropOverlay.svelte`) dan tidak ada impor melingkar antar modul layout.
7. Jalankan seluruh build guard (bagian 6). Verifikasi manual di aplikasi nyata (`build/bin/shikigami`): beberapa terminal + satu file terlihat bersamaan; Cmd+D / Cmd+Shift+D; seret header ke 4 tepi dan tengah (terminal tidak ter-reset); seret pembatas; klik ganda pembatas; maksimalkan/pulihkan; Susun grid (6 jadi 3x2); rename; Akhiri sesi vs Tutup pane; klik sesi/file di sidebar membuka/memfokuskan pane.
8. Terakhir `perf-6-pane` menulis `docs/perf/multipane-report.md`.

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- `cd frontend && npm run check && npm run build` (`check` = svelte-check type-check; `build` = vite build). Bila `check` sudah punya galat bawaan sebelum epic, bandingkan dan pastikan tidak ada galat baru di berkas milik epic ini.
- `make build` (mencakup `go build ./...` dan build frontend) dijalankan oleh agen `layout-wiring` dan `perf-6-pane`; agent fitur paralel cukup menjalankan perintah frontend di atas.
- `grep -rn "webgl" frontend/src/modules/terminal frontend/src/modules/layout` tidak menemukan import addon WebGL (huruf besar/kecil diabaikan, `grep -rni`).

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya. Dilarang menyentuh path fitur lain, `App.svelte`, `modules/shell/`, `modules/tabs/` (milik `layout-wiring`), `lib/stores/tabs.ts`, `lib/stores/sessions.ts`, `lib/registry.ts`, `lib/ipc.ts`, `main.ts`, `modules/workspace/`, `modules/files/`, `modules/status/`, `modules/appmenu/`, `package.json`, `Makefile`, kode Go/daemon.
- Fitur paralel hanya memakai nama ekspor yang tertulis di bagian 2; jangan mengubah kontrak sepihak. Bila stub foundation perlu perubahan signature, hentikan dan laporkan sebagai blocker.
- `modules/terminal/*` hanya disentuh `pane-kinds` dan hanya berupa penambahan opsi (default mempertahankan perilaku v1; grep tanpa WebGL).
- Dilarang menambah dependency npm. Tanpa unit test. Tanpa `console.log` tersisa. Jangan commit/merge sendiri kecuali diminta alur `implement-feature`.
- Model bersifat murni: dilarang memanggil IPC dari `model.ts`/`lib/stores/layout.ts`.
