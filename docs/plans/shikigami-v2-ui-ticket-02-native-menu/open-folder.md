# Plan: Open Folder dan drop folder

## Metadata

- Tiket: `shikigami-v2-ui-ticket-02-native-menu` (`docs/tickets/shikigami-v2-ui-ticket-02-native-menu.md`)
- Fitur: `open-folder` (Open Folder dan drop folder)
- ownedPaths: `app/dialog.go`, `frontend/src/modules/openfolder/`
- Depends on foundation: ya (`menu-bridge-contract`, plan `00-foundation.md`, sudah digabung lebih dulu; menyediakan `app/bridge.go` dan `frontend/src/modules/appmenu/`)
- Rujukan kontrak lengkap: `docs/plans/shikigami-v2-ui-ticket-02-native-menu/overview.md` (bagian 2, 3, 7). Jangan mengubah overview.

## Tujuan

Menambahkan sisi Go untuk menangkap drop file/folder dari Finder (saring ke direktori, emit `files:dropped`), dan modul frontend `openfolder` yang bereaksi pada `menu:open-folder`, `menu:new-workspace-from-folder`, dan `files:dropped`: menambah folder sebagai project, atau membuat workspace dari folder, dengan umpan balik toast di statusbar dan sorotan area drop.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `app/dialog.go` (file baru)
- `frontend/src/modules/openfolder/` (folder baru)

Dilarang menyentuh: `app/bridge.go`, `app/menu.go`, `app/main.go`, `internal/*` (termasuk `internal/appconfig/`), `frontend/src/modules/appmenu/`, `modules/zoom/`, `modules/terminal/*`, `modules/workspace/`, `modules/status/`, `lib/stores/`, `lib/ipc.ts`, `lib/registry.ts`, `main.ts`, `package.json`, `go.mod`, `Makefile`. Jangan menambah dependency. Jangan commit/merge.

## Kontrak

Dipakai (dari foundation, lihat overview 2.2 dan 2.3; jangan didefinisikan ulang):

- Go (`package main`, `app/bridge.go`): `EventFilesDropped` (= `"files:dropped"`), `func (b *Bridge) emit(name string, data ...any)`, `func registerStartupHook(fn func(ctx context.Context, b *Bridge))`. `OpenDirectoryDialog` sudah ada di `bridge.go`; JANGAN diduplikasi di `dialog.go`.
- Wails: `runtime.OnFileDrop(ctx context.Context, callback func(x, y int, paths []string))` dari `github.com/wailsapp/wails/v2/pkg/runtime`.
- Frontend: dari `'../appmenu'`: `onMenuEvent`, `pickDirectory` (mengembalikan `string | null`). Dari `'../../lib/stores'`: `activeWorkspace`, `activeWorkspaceId`, `workspaces`, `loadWorkspaces`, `selectWorkspace(id)`, `createWorkspace(name): Promise<Workspace>`, `addProject(path, name?): Promise<Project>` (melempar `Error('pilih workspace dulu')` bila tidak ada workspace aktif; daemon bisa menolak duplikat). Dari `'../../lib/registry'`: `registerPanel`. Tipe `Workspace`/`Project` dari `'../../lib/stores'` (`Project {id, path, name}`, `Workspace {id, name, projects, config}`).
- Perilaku `files:dropped` (keputusan overview 2.1): Go emit hanya bila `paths` dari Wails tidak kosong; payload `{paths: dirs}` dengan `dirs` hanya direktori (boleh `[]`). Frontend: `paths` kosong menjadi toast "Hanya folder yang bisa ditambahkan".

Disediakan: tidak ada simbol yang diekspor untuk modul lain. Efek samping modul: panel statusbar `openfolder-toast` dan tiga langganan `onMenuEvent`.

## Langkah

### 1. `app/dialog.go`

`package main`. Impor: `context`, `os`, `path/filepath`, `github.com/wailsapp/wails/v2/pkg/runtime`.

1. Fungsi murni `func filterDirs(paths []string) []string`: untuk tiap `p`, lewati bila kosong; `c := filepath.Clean(p)`; `fi, err := os.Stat(c)`; lewati bila `err != nil` atau `!fi.IsDir()`; buang duplikat memakai `map[string]struct{}`; pertahankan urutan masuk. Kembalikan slice non-nil (`make([]string, 0, len(paths))`) agar JSON menjadi `[]`, bukan `null`.
2. `func init()` memanggil `registerStartupHook(installFileDrop)`.
3. `func installFileDrop(ctx context.Context, b *Bridge)`: `runtime.OnFileDrop(ctx, func(x, y int, paths []string) { if len(paths) == 0 { return }; dirs := filterDirs(paths); b.emit(EventFilesDropped, map[string]any{"paths": dirs}) })`. Callback tidak boleh memblokir (hanya stat dan emit).
4. Jangan menambah method pada `Bridge` (agar tidak ikut ter-bind ke JS) dan jangan membuat ulang dialog direktori. Jangan menyentuh `main.go` (aktivasi `EnableFileDrop` dikerjakan fitur `native-menu-impl`).
5. Verifikasi: `go build ./...` lulus. Bila `go vet` mengeluh simbol `registerStartupHook`/`EventFilesDropped` tidak ada, berarti foundation belum ada di basis kode; hentikan dan laporkan (jangan membuat simbol itu sendiri).

### 2. `frontend/src/modules/openfolder/toast-store.ts`

```ts
import { writable } from 'svelte/store';
export interface ToastMessage { id: number; kind: 'info' | 'error'; text: string }
export const toast = writable<ToastMessage | null>(null);
export function showToast(text: string, kind: 'info' | 'error' = 'info', ms = 4000): void;
export function clearToast(): void;
```

`showToast` membatalkan timer sebelumnya (variabel modul `timer`), set `toast` dengan `id` naik (counter modul), lalu `setTimeout(clearToast, ms)` (bukan `setInterval`). `clearToast` membersihkan timer dan `toast.set(null)`.

### 3. `frontend/src/modules/openfolder/Toast.svelte`

Svelte 5 (runes diperbolehkan, mengikuti `modules/status/StatusBar.svelte`). Impor `toast` dari `./toast-store`. Render `{#if $toast}<span class="toast" class:error={$toast.kind === 'error'} role="status" aria-live="polite">{$toast.text}</span>{/if}`. Gaya: `font-size: 12px; white-space: nowrap; max-width: 420px; overflow: hidden; text-overflow: ellipsis; color: var(--fg-muted, #8b949e)`; `.error { color: #e5534b }`. Tanpa tombol.

### 4. `frontend/src/modules/openfolder/drop.css`

```css
body { --wails-drop-target: drop; }
body.wails-drop-target-active,
.wails-drop-target-active { outline: 2px dashed var(--accent, #2f6fed); outline-offset: -4px; }
```

(Wails menambah kelas `wails-drop-target-active` ke elemen yang punya `--wails-drop-target: drop` saat diseret.) Pastikan sorotan terlihat di tepi jendela; boleh memakai `box-shadow: inset 0 0 0 2px ...` bila `outline` terpotong.

### 5. `frontend/src/modules/openfolder/logic.ts`

Semua fungsi murni/async tanpa side effect modul:

1. `export function basename(path: string): string`: buang trailing `/` (dan `\`) berulang; ambil segmen terakhir setelah `/` terakhir; bila hasil kosong (root `/` atau string kosong) kembalikan `'root'`. Contoh: `/Users/x/code/core/` menjadi `core`, `/` menjadi `root`.
2. `export function errMessage(e: unknown): string`: salin logika kecil dari `modules/workspace/errors.ts` (ambil `Error.message` / string / bawaan `'terjadi kesalahan'`, potong 120 karakter). JANGAN impor dari `modules/workspace/`.
3. `export function isDuplicate(path: string): boolean`: baca `get(activeWorkspace)`; true bila ada project dengan `normalize(p.path) === normalize(path)` (`normalize` = buang trailing slash).
4. `export async function newWorkspaceFromFolder(path: string): Promise<void>`: `const ws = await createWorkspace(basename(path)); await selectWorkspace(ws.id); await addProject(path);`. Bila `selectWorkspace` tidak mengisi `activeWorkspaceId` pada saat itu (periksa `get(activeWorkspaceId) === ws.id`), set eksplisit lewat `activeWorkspaceId.set(ws.id)` sebelum `addProject`. Jangan menangkap error di sini (penangan memanggilnya).
5. `export async function addFolders(paths: string[]): Promise<void>` (dipakai Open Folder dan drop), berurutan (`for ... of` dengan `await`, tanpa paralel):
   - Bila tidak ada workspace aktif (`get(activeWorkspace) === null`): `first = paths[0]`; `try { await newWorkspaceFromFolder(first) } catch (e) { catat gagal }`; lanjutkan dengan sisa `paths.slice(1)` ke jalur biasa.
   - Jalur biasa per path: bila `isDuplicate(path)` hitung ke `dupes` (tanpa memanggil `addProject`); selain itu `try { await addProject(path); ok++ } catch (e) { failed.push(`${basename(path)}: ${errMessage(e)}`) }`. Kegagalan satu path tidak menghentikan yang lain.
   - Ringkasan di akhir via `showToast`: semua sukses tanpa duplikat/gagal, tampilkan `"N folder ditambahkan"` (info); bila ada duplikat saja `"Folder sudah menjadi project"` (satu path) atau `"N folder sudah menjadi project"` (info); bila ada gagal `"Gagal menambahkan: ..."` digabung `; ` dan ditambah hitungan sukses bila ada (error). Satu pesan ringkas, bukan satu per path.
6. `export async function handleOpenFolder(): Promise<void>`: `const p = await pickDirectory(); if (!p) return;` lalu `try { await addFolders([p]) } catch (e) { showToast(errMessage(e), 'error') }`.
7. `export async function handleNewWorkspaceFromFolder(): Promise<void>`: `pickDirectory()`; `null` berhenti diam-diam; `try { await newWorkspaceFromFolder(p); showToast(`Workspace "${basename(p)}" dibuat`) } catch (e) { showToast(errMessage(e), 'error') }`.
8. `export async function handleDropped(p: { paths: string[] }): Promise<void>`: `const paths = p?.paths ?? []`; bila kosong `showToast('Hanya folder yang bisa ditambahkan', 'error'); return`; selain itu `await addFolders(paths)` dibungkus try/catch ke toast.

Penjaga re-entrancy: variabel modul `busy`; bila `busy` true saat event baru tiba (mis. dialog sedang terbuka atau drop sedang diproses), abaikan event baru; `busy` di-reset di `finally`. Terapkan di tiga `handle*`.

### 6. `frontend/src/modules/openfolder/index.ts`

```ts
import './drop.css';
import { registerPanel } from '../../lib/registry';
import { onMenuEvent } from '../appmenu';
import Toast from './Toast.svelte';
import { handleOpenFolder, handleNewWorkspaceFromFolder, handleDropped } from './logic';

registerPanel({ id: 'openfolder-toast', area: 'statusbar', component: Toast, order: 90 });
onMenuEvent('menu:open-folder', () => { void handleOpenFolder(); });
onMenuEvent('menu:new-workspace-from-folder', () => { void handleNewWorkspaceFromFolder(); });
onMenuEvent('files:dropped', (p) => { void handleDropped(p); });
```

Modul memuat otomatis lewat glob `main.ts` (jangan sunting). Simpan fungsi unsubscribe hanya bila perlu; tidak perlu karena modul hidup selama aplikasi. Tanpa `console.log`, tanpa `setInterval`. Bila `window.runtime` tidak ada, `onMenuEvent` tetap aman (tidak ada error).

### 7. Verifikasi

- `go build ./...`
- `cd frontend && npm run build` (dan, bila tersedia, `npm run check` untuk tipe Svelte; abaikan peringatan lama di luar path fitur ini).
- `git status`/diff mental: hanya `app/dialog.go` dan `frontend/src/modules/openfolder/*` yang berubah.

## Perubahan

Dibuat:

- `app/dialog.go`
- `frontend/src/modules/openfolder/index.ts`
- `frontend/src/modules/openfolder/logic.ts`
- `frontend/src/modules/openfolder/toast-store.ts`
- `frontend/src/modules/openfolder/Toast.svelte`
- `frontend/src/modules/openfolder/drop.css`

Diubah: tidak ada.

## Wiring

Yang diekspos fitur ini untuk disambungkan langkah Wiring (jangan disambungkan sendiri):

- Go: `init()` di `app/dialog.go` mendaftarkan `installFileDrop` lewat `registerStartupHook`; berjalan otomatis dari `Bridge.SetContext` pada `OnStartup`. Wiring memastikan `app/main.go` memanggil `bridge.SetContext(ctx)` di `OnStartup` dan `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}` aktif (milik `native-menu-impl`); tanpa itu `OnFileDrop` tidak menerima apa pun.
- Frontend: `modules/openfolder/index.ts` termuat otomatis oleh glob `main.ts` dan mendaftarkan panel statusbar `openfolder-toast` (order 90). Wiring memastikan statusbar v1 merendernya, dan event `menu:open-folder`, `menu:new-workspace-from-folder` dikirim oleh menu File.
- Verifikasi manual di aplikasi nyata (dikerjakan Wiring): Cmd+O, Cmd+Shift+O, drop folder/file dari Finder, sorotan area drop, folder duplikat, kondisi tanpa workspace aktif.

## Selesai bila

- `go build ./...` sukses dengan `app/dialog.go` hadir.
- `cd frontend && npm run build` sukses tanpa dependency baru.
- `filterDirs` menghasilkan hanya direktori, bersih (`filepath.Clean`), tanpa duplikat, dan slice non-nil; callback drop tidak emit bila `paths` kosong, dan emit `files:dropped` dengan `paths: []` bila semua file biasa.
- `dialog.go` tidak menduplikasi `OpenDirectoryDialog` dan tidak menambah method pada `Bridge`.
- Modul `openfolder` memakai `onMenuEvent`/`pickDirectory` dari `../appmenu` dan store v1 dari `lib/stores`; batal pada dialog tidak memunculkan error; duplikat menghasilkan pesan, bukan crash; kegagalan satu path tidak menghentikan yang lain, dengan satu ringkasan toast; tanpa workspace aktif tetap menghasilkan workspace valid dari folder pertama; `basename('/Users/x/code/core/')` = `core`.
- `drop.css` diimpor dari `index.ts`; `Toast.svelte` terdaftar sebagai `openfolder-toast` area `statusbar` order 90.
- Tanpa `console.log`, tanpa `setInterval`, tanpa unit test. Tidak ada file di luar ownedPaths yang berubah.
