# Overview: Bilah atas dan dua sidebar (shell v2)

Tiket: `docs/tickets/shikigami-v2-ui-ticket-04-shell-sidebars.md`
Spec: `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md` (bagian 4, 4b)
Frontend: Svelte 5 (runes) + Vite + TypeScript di `frontend/`. Epic ini 100% frontend: tanpa kode Go/daemon, tanpa dependency npm baru, tanpa unit test.

## 1. Ringkasan epic dan urutan eksekusi

Shell v1 (`SidebarArea`, `StatusBar`, `TabBar`, `TabContent`) diganti shell v2: bilah atas (tab workspace, ringkasan tengah, lonceng, penghitung sesi, pengaturan), sidebar kiri "AGENT" (sesi per project), sidebar kanan "FILE" (tree project), dan palet perintah Cmd+K, mengelilingi area layout multi-pane dari tiket 03 (`modules/layout/LayoutArea.svelte`). Dua titik ekstensi baru: `registerRightTab` dan `registerTopbarItem`.

Urutan:

1. **Foundation** `shell-frame` (plan: `00-foundation.md`; plan fitur yang sama: `shell-frame.md` boleh merujuk 00-foundation). Menetapkan `lib/stores/shell.ts`, `modules/shell/registry.ts`, `ShellFrame`, `Resizer`, `RightPanel`, `shortcuts.ts`, dan helper bersama `modules/shell/helpers.ts` (bagian 2.5). Digabung lebih dulu.
2. **Fitur paralel** (path terpisah): `topbar`, `sidebar-left-agent`, `sidebar-right-files`, `command-palette`, `notification-bell`.
3. **Wiring** `shell-wiring` (sendiri, terakhir): `register.ts`, `App.svelte`, pensiun `SidebarArea`/`StatusBar` v1.

Semua fitur paralel hanya bergantung pada kontrak di dokumen ini dan pada file foundation yang sudah ada (registri, helper). Tidak perlu stub: foundation menyediakan implementasi penuh sebelum fitur paralel mulai.

## 2. Kontrak bersama

### 2.1 Fakta repo yang dipakai (sudah dibaca; jangan diubah)

- `frontend/src/main.ts` memuat `./modules/*/index.ts` lewat `import.meta.glob` (eager). Jangan edit `main.ts`. Akibat: setiap `index.ts` modul baru dimuat otomatis saat start. `modules/shell/index.ts` dan `modules/topbar/index.ts` harus bebas efek samping (hanya re-export); `modules/sidebar-right/index.ts`, `modules/notifications/index.ts`, `modules/palette/index.ts` boleh mendaftar di registri (idempoten), dan `modules/shell/register.ts` (wiring) mengimpornya secara eksplisit sebagai jaminan.
- Store (`lib/stores/index.ts` mengekspor): `workspaces`, `activeWorkspaceId`, `activeWorkspace`, `activeProjectId`, `activeProject`, `selectWorkspace(id)`, `selectProject(id)`, `createWorkspace(name): Promise<Workspace>`, `renameWorkspace(id,name)`, `deleteWorkspace(id)`, `addProject(path,name?)`, `removeProject(id)`, `sessions` (`Session[]`), `createSession(providerId)` (memakai `activeWorkspaceId` + `activeProject`, jadi project target HARUS dipilih via `selectProject` sebelum memanggil), `killSession(id)` (menandai `status:'exited'`), `providers` (`Provider {id,name,available?}`), `connection`, `initStores`.
- Tipe: `Session {id, workspaceId, projectId, cwd, providerId, status: 'running'|'exited', createdAt, exitCode?}`, `Project {id,path,name}`, `Workspace {id,name,projects,config}`.
- `modules/status` (`index.ts` mengekspor `StatusBadge`; `store.ts` mengekspor `statusStore: Readable<Record<sessionId,{status,since}>>`, `statusOf(id)`, tipe `Status = 'running'|'waiting'|'done'|'idle'`). `StatusBadge` props `{sessionId, compact?}`. `startStatusSync()` sudah dipanggil `modules/status/index.ts`; JANGAN memanggil lagi atau membuat pendengar `session.status` kedua. Impor langsung dari `../status/store` dan `../status/StatusBadge.svelte`.
- `modules/appmenu` (`index.ts`): `onMenuEvent(name, fn): unsubscribe`, `emitMenuEvent(name, payload?)`, `pickDirectory()`; event `menu:zoom {action:'in'|'out'|'reset'}`, `menu:open-folder`, `menu:toggle-sidebar-left`, `menu:toggle-sidebar-right`. `modules/zoom` satu-satunya pemilik `menu:zoom`; `modules/openfolder` satu-satunya pemilik `menu:open-folder` (dialog + tambah project + toast). Epic ini memicu keduanya dengan `emitMenuEvent`, tidak menambah handler `menu:zoom` atau `menu:open-folder`.
- `modules/workspace` (`WorkspacePicker`, `NewSession`, `SessionList`, `ProjectList`, `errors.ts` dengan `fn`/helper pesan): dipakai ulang bila cocok, selain itu bungkus/tulis komponen baru yang memanggil store yang sama; jangan salin pemanggilan IPC. Komponen v1 memanggil `openTab` (tidak dipakai di shell v2).
- `modules/files/tree/FilesPanel.svelte` memakai `TreeModel` (`treeModel.svelte.ts`: `setProjects(projects)`, `rows`, `toggle(row)`, tipe `TreeRow {key,path,root,kind:'project'|'dir'|'file',depth,placeholder?,...}`), membuka file lewat `openFileTab` (tab v1; tiket 03 menjembatani ke layout lewat shim). Jangan ubah `modules/files`.
- Dari tiket 03 (sudah tergabung sebelum wave 4): `lib/stores/layout.ts` mengekspor `layout` (`LayoutStore`: `subscribe`, `openPane(leaf: NewLeaf, opts?): string`, `setFocus(paneId)`, `closePane`, `tidyGrid()`, `leaves: Readable<Leaf[]>`, `paneCount`, `serialize()`), dan `modules/layout/model.ts` mengekspor tipe `Leaf`, `NewLeaf`, `LayoutDoc`, fungsi `leaves(doc)`, `findLeafBy(doc, pred)`. `openPane` idempoten (sesi/file yang sama difokuskan, bukan digandakan). `modules/layout/LayoutArea.svelte` (prop opsional `store`) memasang `installLayoutShortcuts` sendiri. Nama persis mengikuti implementasi; baca `lib/stores/layout.ts` dan `modules/layout/model.ts` sebelum menulis kode. Impor `layout` langsung dari `../../lib/stores/layout`, JANGAN lewat `lib/stores/index.ts`.
  - `Leaf = {type:'pane', id, kind, sessionId?, projectId?, provider?, filePath?, workspaceId?, title}`.
  - `NewLeaf = {kind, sessionId?, projectId?, provider?, filePath?, workspaceId?, title?}`.
- Tema: variabel `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok` di `styles/theme.css` (jangan ubah). Gaya global tombol/input di `styles/base.css`. Komponen memakai variabel itu, tanpa warna hardcode (fallback boleh). Variabel baru shell (`--topbar-h`, `--left-w`, `--right-w`) didefinisikan di `modules/shell/shell.css`.
- Jendela: `app/main.go` tidak menyetel title bar tersembunyi (tidak ada `TitleBar`/`Frameless`), jadi inset lampu lalu-lintas TIDAK diperlukan. Topbar memakai variabel CSS `--topbar-inset-left` (bawaan `0px`, didefinisikan di `shell.css`) sehingga mudah diaktifkan kelak; area kosong topbar diberi `--wails-draggable: drag` (aman bila title bar standar), kontrol interaktif diberi `--wails-draggable: no-drag`.
- Guard izin agent: sesi dibuat HANYA lewat `createSession`; frontend tidak menyusun argumen provider.

### 2.2 State shell: `frontend/src/lib/stores/shell.ts` (milik `shell-frame`, persis kontrak tiket)

```ts
import type { Writable } from 'svelte/store';
export interface SidebarState { visible: boolean; width: number }
export interface RightSidebarState extends SidebarState { activeTab: string }
export const leftSidebar: Writable<SidebarState>;          // default {visible:true, width:260}
export const rightSidebar: Writable<RightSidebarState>;    // default {visible:true, width:280, activeTab:'files'}
export const LEFT_MIN = 200, LEFT_MAX = 420, RIGHT_MIN = 220, RIGHT_MAX = 520;
export const LEFT_DEFAULT = 260, RIGHT_DEFAULT = 280;
export function toggleLeft(): void;
export function toggleRight(): void;
export function setLeftWidth(px: number): void;   // clamp [LEFT_MIN, LEFT_MAX]; NaN/non-finite -> default
export function setRightWidth(px: number): void;  // clamp [RIGHT_MIN, RIGHT_MAX]
export function setRightTab(id: string): void;
```

Bentuk sama dengan UI state kontrak 5 spec (tiket 06 yang kelak menyimpan/memulihkan; epic ini tidak memanggil `workspace.ui.*`).

### 2.3 Registri titik ekstensi: `frontend/src/modules/shell/registry.ts` (milik `shell-frame`)

```ts
import type { Component } from 'svelte';
import type { Readable } from 'svelte/store';
export interface RightTabSpec { id: string; title: string; component: Component<any>; order?: number }
export interface TopbarItemSpec { id: string; component: Component<any>; align: 'left' | 'center' | 'right'; order?: number }
export function registerRightTab(spec: RightTabSpec): void;       // idempoten per id (timpa)
export function registerTopbarItem(spec: TopbarItemSpec): void;   // idempoten per id (timpa)
export function rightTabs(): Readable<RightTabSpec[]>;            // urut order (default 0) lalu id
export function topbarItems(align: 'left' | 'center' | 'right'): Readable<TopbarItemSpec[]>;
```

Pola mengikuti `lib/registry.ts` (writable map + derived terurut).

`frontend/src/modules/shell/index.ts` (tanpa efek samping) mengekspor: `registerRightTab`, `registerTopbarItem`, `rightTabs`, `topbarItems`, tipe spec, `ShellFrame`, `RightPanel`, dan seluruh isi `helpers.ts` (bagian 2.5). Modul lain cukup `import { registerRightTab, focusOrOpenSession } from '../shell'`.

### 2.4 Registri palet: `frontend/src/modules/palette/registry.ts` (milik `command-palette`, persis kontrak tiket)

```ts
export interface PaletteAction { id: string; title: string; hint?: string; keywords?: string[]; run: () => void | Promise<void> }
export type PaletteSourceKind = 'project' | 'session' | 'file' | 'action';
export interface PaletteItem { id: string; kind: PaletteSourceKind; title: string; subtitle?: string; run: () => void | Promise<void> }
export interface PaletteSource { id: string; kind: PaletteSourceKind; list: (query: string) => PaletteItem[] | Promise<PaletteItem[]> }
export function registerPaletteAction(a: PaletteAction): void;   // idempoten per id
export function registerPaletteSource(s: PaletteSource): void;   // idempoten per id
```

`modules/palette/index.ts` mengekspor juga `Palette` (komponen), `paletteOpen: Readable<boolean>`, `openPalette()`, `closePalette()`.

### 2.5 Helper bersama: `frontend/src/modules/shell/helpers.ts` (milik `shell-frame`; diekspor via `modules/shell/index.ts`)

Tujuan: sidebar kiri, palet, dan lonceng memerlukan perilaku yang SAMA (fokus-atau-buka sesi/file di layout) tetapi tidak boleh saling impor. Semua fitur memakai helper ini, tidak menulis ulang.

```ts
import type { Leaf } from '../layout/model';
import type { Session } from '../../lib/stores';
export function findSessionPane(sessionId: string): Leaf | null;                 // baca get(layout) lalu findLeafBy(kind==='terminal' && sessionId)
export function findFilePane(projectId: string, filePath: string): Leaf | null;  // kind==='file' && filePath && projectId cocok
export function sessionTitle(sessionId: string): string;                         // judul pane bila ada di layout, selain itu nama provider (providers store), selain itu providerId, selain itu id pendek
export function focusOrOpenSession(sessionId: string): boolean;                  // pane ada: layout.setFocus; belum: layout.openPane({kind:'terminal', sessionId, projectId, provider: providerId, title: sessionTitle}); false bila sesi tidak ada di store `sessions`
export function openFilePane(a: { projectId: string; filePath: string; workspaceId?: string; title?: string }): string; // fokus bila ada, selain itu openPane({kind:'file', ...}); title bawaan basename; workspaceId bawaan activeWorkspaceId; mengembalikan paneId
export function basename(path: string): string;
export function createAndOpenSession(providerId: string, projectId?: string): Promise<Session>; // projectId diberikan -> selectProject(projectId); lalu createSession(providerId); lalu focusOrOpenSession(s.id); melempar bila gagal (pemanggil menampilkan pesan)
export function openFolder(): void;                                              // emitMenuEvent('menu:open-folder')
export function zoom(action: 'in' | 'out' | 'reset'): void;                      // emitMenuEvent('menu:zoom', {action})
export function tidyGrid(): void;                                                // layout.tidyGrid()
```

Aturan: tidak ada timer/polling; tidak memanggil `killSession` kecuali dari aksi eksplisit "Akhiri sesi". Menutup pane tidak membunuh sesi (spec bagian 5).

### 2.6 Penyatuan toggle dan pintasan (milik `shell-frame`, `modules/shell/shortcuts.ts`)

```ts
export function installShellShortcuts(): () => void;  // dipanggil ShellFrame saat mount, dilepas saat destroy
```

- Berlangganan `onMenuEvent('menu:toggle-sidebar-left'|'menu:toggle-sidebar-right')` dan `keydown` di `window` (fase capture): Cmd+B (metaKey, tanpa alt/shift, `code==='KeyB'`) -> kiri; Cmd+Opt+B (metaKey+altKey, `code==='KeyB'`) -> kanan. Pakai `event.code`, bukan `event.key` (Option mengubah `key`).
- Dedupe: satu fungsi `fire(side)` dengan penjaga waktu per sisi (`performance.now()` selisih < 250 ms dari pemicu sebelumnya pada sisi yang sama diabaikan). Keydown memanggil `preventDefault()` + `stopPropagation()` agar xterm tidak menerima. Keduanya tembak -> toggle tepat sekali.
- Cmd+B/Cmd+Opt+B diproses walau fokus di input teks? Tidak: abaikan bila `event.target` adalah `input`, `textarea`, atau `select` (bukan terminal; xterm memakai textarea tersembunyi, jadi pengecualian: textarea dengan kelas `xterm-helper-textarea` tetap diproses).
- Cmd+K bukan urusan shell: dimiliki `command-palette` (komponen `Palette` memasang handler sendiri). Jangan memakai Cmd+K, Cmd+D, Cmd+Shift+D, Cmd+O, zoom di `shortcuts.ts`.

### 2.7 Komponen shell: props snippet dan slot

`ShellFrame.svelte` (Svelte 5), props:

```ts
import type { Snippet } from 'svelte';
let { topbar, left, center, right }: { topbar: Snippet; left: Snippet; center: Snippet; right: Snippet } = $props();
```

Grid CSS: baris topbar `var(--topbar-h)` + baris isi `1fr`; kolom isi `var(--left-w) | 1fr | var(--right-w)`; `--left-w`/`--right-w` bernilai `0px` saat tersembunyi atau `${width}px` saat terlihat; konten sidebar tetap ter-mount (`overflow:hidden`, `visibility` tidak perlu). `ShellFrame` tidak mengimpor modul fitur. Resizer (`Resizer.svelte`, props `{side:'left'|'right'}`) berada di tepi dalam kolom sidebar, memperbarui CSS variable pada elemen root shell lewat `requestAnimationFrame` selama seret dan memanggil `setLeftWidth`/`setRightWidth` saat lepas dan di akhir tiap frame bila perlu; dblclick mengembalikan `LEFT_DEFAULT`/`RIGHT_DEFAULT`. `RightPanel.svelte` (tanpa props) membaca `rightTabs()` dan `rightSidebar.activeTab`.

Komponen fitur yang menjadi isi slot (nama file dan default export yang dipakai wiring):

| Slot | Komponen | Berkas |
|------|----------|--------|
| `topbar` | `Topbar` | `modules/topbar/Topbar.svelte` |
| `left` | `AgentSidebar` | `modules/sidebar-left/AgentSidebar.svelte` |
| `center` | `LayoutArea` | `modules/layout/LayoutArea.svelte` (tiket 03) |
| `right` | `RightPanel` | `modules/shell/RightPanel.svelte` |
| overlay | `Palette` | `modules/palette/Palette.svelte` |

Semua komponen di atas tidak punya props wajib (props opsional diperbolehkan).

### 2.8 Pembagian tanggung jawab antar fitur (ringkas)

- Item topbar: `Topbar` merender langsung `WorkspaceTabs` (kiri), `CenterSummary` (tengah), `SessionCounter` + `SettingsButton` (kanan), ditambah item dari `topbarItems(align)` di tiap zona (lonceng datang dari registri, `align:'right'`, `order:10`). Urutan zona kanan: item registri dengan `order < 50`, `SessionCounter`, item registri `order >= 50`, `SettingsButton` terakhir.
- Hanya `notification-bell` yang membuat `Bell.svelte` dan `registerTopbarItem({id:'notifications', component: Bell, align:'right', order:10})`; hanya `sidebar-right-files` yang membuat `registerRightTab({id:'files', title:'Files', component: FilesTab, order:10})`, keduanya di `index.ts` modul masing-masing.
- Aksi palet bawaan memakai helper bagian 2.5 dan `toggleLeft`/`toggleRight` dari `lib/stores/shell`.

### 2.9 Konvensi

- Folder modul: `modules/<nama-kebab>/`; komponen Svelte `PascalCase.svelte`; util/store `camelCase.ts`; hanya `index.ts` yang diimpor modul lain.
- Impor store layout/shell langsung dari berkasnya (`../../lib/stores/layout`, `../../lib/stores/shell`), bukan `lib/stores/index.ts`.
- Svelte 5 runes (`$state`, `$derived`, `$props`, `$effect`), snippet untuk slot; langganan store Svelte lewat `$store` auto-subscribe atau `subscribe` dibersihkan di `$effect`.
- Teks UI dalam bahasa Indonesia kecuali label baku dari spec (`AGENT`, `FILE`, `New Session`, `Open Folder…`, `Running`, `Files`).
- Tanpa polling (`setInterval`/`setTimeout` berulang). Hanya langganan store/event. Timer sekali jalan (debounce, dedupe) diperbolehkan.
- Tanpa dependency npm baru; tanpa `console.log`.
- Komentar kode ringkas dan hanya untuk batasan yang tidak jelas.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|------|-------|------------|
| `shell-frame` (Foundation) | `lib/stores/shell.ts`, registri `registerRightTab`/`registerTopbarItem`, `ShellFrame`, `Resizer`, `RightPanel`, `shortcuts.ts`, `shell.css`, `helpers.ts`, `index.ts` | `frontend/src/modules/shell/` (kecuali `register.ts` dan penghapusan `SidebarArea`/`StatusBar` v1, milik wiring), `frontend/src/lib/stores/shell.ts` |
| `topbar` | `Topbar`, `WorkspaceTabs`, `SessionCounter`, `SettingsButton`, `CenterSummary`, `index.ts` | `frontend/src/modules/topbar/` |
| `sidebar-left-agent` | `AgentSidebar`, `WorkspaceHeader`, `SessionGroups`, `NewSessionMenu`, `index.ts` | `frontend/src/modules/sidebar-left/` |
| `sidebar-right-files` | `FilesTab`, Open Folder + state kosong + input path cadangan, registrasi tab `files` di `index.ts` | `frontend/src/modules/sidebar-right/` |
| `command-palette` | `Palette`, `fuzzy.ts`, `registry.ts`, `sources.ts`, `actions.ts`, `index.ts` | `frontend/src/modules/palette/` |
| `notification-bell` | `store.ts`, `Bell.svelte`, registrasi topbar di `index.ts` | `frontend/src/modules/notifications/` |
| `shell-wiring` | `register.ts`, `App.svelte` memakai `ShellFrame`, pensiun komponen v1 | `frontend/src/App.svelte`, `frontend/src/modules/shell/` (hanya `register.ts` dan penghapusan/shim `SidebarArea.svelte`, `StatusBar.svelte`) |

Catatan scope tambahan per fitur (keputusan desain bersama):

- `shell-frame` juga memiliki `helpers.ts` (bagian 2.5) agar tidak ada duplikasi di tiga fitur; ini berada di dalam `modules/shell/` yang memang dimilikinya.
- `topbar`: `SettingsButton` memanggil `zoom`, `toggleLeft`, `toggleRight`, `tidyGrid` dari helper/store (bagian 2.5), tidak memodifikasi `modules/zoom`. `SessionCounter` menghitung sesi workspace aktif dengan `Session.status==='running'` dan `statusStore` yang `running`; tooltip rincian per status (running/waiting/done/idle) dari `statusStore`. `CenterSummary`: lencana provider singkat CL/CX/SH (`claude`->CL, `codex`->CX, `shell`->SH, lainnya 2 huruf pertama kapital).
- `sidebar-left-agent`: header memakai ulang `WorkspacePicker` atau komponen sendiri yang memanggil `selectWorkspace`; sesi `exited` redup, klik tetap membuka pane (`focusOrOpenSession`). "Akhiri sesi" dengan konfirmasi inline memanggil `killSession`. New Session memakai `createAndOpenSession(providerId, projectId)`; tanpa project tampilkan ajakan memanggil `openFolder()`.
- `sidebar-right-files`: `FilesPanel` v1 menampilkan semua project dan membuka lewat `openFileTab` (jalur tab). Karena itu `FilesTab` membuat tree sendiri dengan `TreeModel` (`setProjects([projekTerpilih])`) dan menangani klik file dengan `openFilePane`. `modules/files` tidak diubah. Open Folder: `openFolder()`; fallback input path memanggil `addProject(path)`. Dialog dibatalkan tidak mengubah apa pun.
- `command-palette`: pencarian file dibatasi pada baris file yang sudah dimuat `TreeModel` bila tidak ada metode cari di `lib/ipc` (catat di komentar). Sesi: `focusOrOpenSession`. Project: `selectProject`. Aksi bawaan: "Sesi baru: claude/codex/shell" (`createAndOpenSession(provider)` di project aktif), "Open Folder…", "Zoom in/out/reset", "Toggle sidebar kiri/kanan", "Susun grid". Handler Cmd+K di `Palette` (komponen) dengan listener `window` fase capture, tetap aktif saat fokus di input atau terminal; simpan `document.activeElement` sebelum buka dan kembalikan fokus saat tutup.
- `notification-bell`: store memakai `statusStore.subscribe` dengan peta status sebelumnya; snapshot pertama (isi awal store, atau entri sesi yang belum pernah terlihat) hanya mengisi peta tanpa membuat notifikasi. Judul lewat `sessionTitle`. Klik entri: `focusOrOpenSession` lalu `markRead`; sesi tidak ada di `sessions` -> entri redup tanpa aksi. Store dibuat saat modul dimuat (sekali) dan tidak berlangganan ulang saat dropdown ditutup.

## 4. Foundation

Ada: `shell-frame` (plan lengkap di `00-foundation.md`). Isi yang harus ada sebelum fitur paralel: `lib/stores/shell.ts`, `modules/shell/registry.ts`, `modules/shell/helpers.ts`, `modules/shell/index.ts`, `modules/shell/shell.css`, `ShellFrame.svelte`, `Resizer.svelte`, `RightPanel.svelte`, `shortcuts.ts`. Prasyarat luar: tiket 02 dan 03 sudah tergabung (ada `modules/appmenu`, `modules/openfolder`, `lib/stores/layout.ts`, `modules/layout/`). Bila tidak ada, hentikan dan laporkan blocker.

## 5. Wiring checklist (untuk agen `shell-wiring`)

Dikerjakan setelah semua fitur lain tergabung. Hanya menyentuh `frontend/src/App.svelte` dan `frontend/src/modules/shell/` (`register.ts`, hapus/shim `SidebarArea.svelte` dan `StatusBar.svelte`).

1. `modules/shell/register.ts` (BARU, side-effect, tanpa ekspor): `import '../sidebar-right'; import '../notifications'; import '../palette';` (supaya tab `files`, item topbar `notifications`, aksi/sumber palet terdaftar). Pastikan tidak diimpor oleh `modules/shell/index.ts` (hindari siklus); `App.svelte` mengimpornya.
2. `App.svelte`:
   - Hapus impor `TabBar`, `TabContent`, `SidebarArea`, `StatusBar`.
   - Tambah impor: `ShellFrame`, `RightPanel` dari `./modules/shell` (atau berkas langsung), `./modules/shell/register`, `Topbar` dari `./modules/topbar/Topbar.svelte`, `AgentSidebar` dari `./modules/sidebar-left/AgentSidebar.svelte`, `LayoutArea` dari `./modules/layout/LayoutArea.svelte`, `Palette` dari `./modules/palette/Palette.svelte`.
   - Cabang `{:else}` merender `<ShellFrame>` dengan snippet `{#snippet topbar()}<Topbar />{/snippet}`, `left` -> `<AgentSidebar />`, `center` -> `<LayoutArea />`, `right` -> `<RightPanel />`, dan `<Palette />` sebagai sibling di lapisan atas (di dalam cabang yang sama).
   - Pertahankan apa adanya: `initStores()`/`start()`/`dispose`, cabang `connection.state === 'error'` dan `'connecting'`, gaya `.center`/`.msg`. Hapus CSS `.shell`, `.side`, `.main`, `.content`, `.status` v1 yang tak terpakai.
3. Pensiun v1: hapus `modules/shell/SidebarArea.svelte` dan `modules/shell/StatusBar.svelte` bila tidak ada impor tersisa (grep `SidebarArea`/`shell/StatusBar` di `frontend/src`); jika masih diimpor modul lain, ubah menjadi shim kosong dengan komentar. Registrasi `registerPanel` area `sidebar`/`statusbar` (workspace-sidebar, status-badges, status-summary, panel files, openfolder-toast) dibiarkan: tidak dirender lagi. Catatan: `openfolder-toast` (statusbar) tidak lagi tampil; bila `modules/openfolder` menyediakan komponen `Toast.svelte`, render `<Toast />` di `App.svelte` agar umpan balik Open Folder tetap terlihat (impor `./modules/openfolder/Toast.svelte`; tanpa mengubah modulnya).
4. `modules/tabs` (v1) tidak dihapus dan tidak diubah (milik tiket 03; shim tab-ke-layout tetap hidup).
5. Impor side-effect `modules/appmenu`, `modules/openfolder`, `modules/zoom`, `modules/layout`: sudah dimuat oleh glob `main.ts`; periksa, tambah impor di `App.svelte` hanya bila `import.meta.glob` tidak mencakup (jangan edit `main.ts`).
6. Verifikasi (manual, bukan guard): tab dummy `registerRightTab({id:'dummy', title:'Dummy', component: ...})` dari modul lain terlihat sebagai tab baru tanpa mengubah kode shell; hapus setelah verifikasi. Cmd+B, Cmd+Opt+B, Cmd+K berfungsi; klik sesi memfokuskan pane; klik file membuka pane file; layar error/connecting tetap berfungsi.

## 6. Build guard

Satu-satunya guard: `make build` dari root repo (mencakup `cd frontend && npm install && npm run build`, `go build` daemon dan CLI, dan build app Wails). Tanpa unit test. Tiap fitur menjalankan minimal `cd frontend && npm run build` (dan `make build` bila sempat) sebelum selesai; foundation dan wiring wajib `make build` penuh. Untuk fitur paralel yang dikerjakan di worktree, `frontend/node_modules` mungkin perlu `npm install` dulu. Verifikasi interaktif lewat `make dev`/biner hasil build bersifat cek manual, bukan guard.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya (bagian 3). Dilarang menyentuh path fitur lain dan file wiring (`App.svelte`, `modules/shell/register.ts`).
- Fitur paralel TIDAK mengedit `modules/shell/` (foundation) walau butuh perubahan; laporkan sebagai blocker. Impor dari `../shell` saja.
- Dilarang mengedit: `main.ts`, `lib/stores/index.ts`, `lib/stores/layout.ts`, `lib/stores/workspaces.ts`, `lib/stores/sessions.ts`, `lib/registry.ts`, `styles/theme.css`, `styles/base.css`, `package.json`, `Makefile`, kode Go, serta modul `workspace`, `status`, `files`, `openfolder`, `appmenu`, `zoom`, `layout`, `tabs`, `terminal`, `mention`.
- Fitur paralel tidak saling mengimpor (topbar, sidebar-left, sidebar-right, palette, notifications). Koordinasi hanya lewat registri dan helper `modules/shell` (bagian 2.3, 2.5).
- Semua fitur harus build sendiri-sendiri; `register.ts`/`App.svelte` belum menyambungkan modul sampai wiring, jadi tidak ada efek visual sebelum itu.
- Identifier kode memakai bahasa Inggris; komentar/teks UI bahasa Indonesia.
