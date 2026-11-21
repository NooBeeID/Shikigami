# Plan: Panel tree project

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-07-file-viewer.md` (epic 07 File Viewer), fitur `file-tree-panel`
- Desain bersama: `docs/plans/ai-terminal-ticket-07-file-viewer/overview.md` (bagian 2.2, 2.4, 2.5, 7)
- Fitur: `file-tree-panel` (Panel tree project)
- ownedPaths: `frontend/src/modules/files/tree/`
- Depends on foundation: ya (`00-foundation.md` harus sudah selesai; `shared/types.ts`, `shared/fsClient.ts`, `shared/shellBridge.ts`, serta stub `tree/FilesPanel.svelte` dan `tree/registerTree.ts` sudah ada)

## Tujuan

Membuat panel sidebar "Files" (id `files`, area `sidebar`, order 20) yang menampilkan satu akar per project pada workspace aktif. Isi folder dimuat malas saat folder dibuka (`fsList`), di-cache per path, dengan tombol segarkan, error inline + coba lagi, tanpa polling dan tanpa `fs.watch`. Klik file membuka tab `file` lewat `openFileTab`. Daftar dirender datar hanya untuk folder yang terbuka, dengan navigasi keyboard. Fitur ini menimpa dua stub Foundation (`FilesPanel.svelte`, `registerTree.ts`) dan menambah satu modul logika.

## Path yang dimiliki

HANYA boleh dibuat/diubah:
- `frontend/src/modules/files/tree/` (menimpa `FilesPanel.svelte` dan `registerTree.ts` stub Foundation; menambah `treeModel.svelte.ts`)

Dilarang: `frontend/src/modules/files/shared/` (perubahan kontrak = laporkan blocker), `viewer/`, `editor/`, `frontend/src/modules/files/index.ts` (JANGAN dibuat), seluruh file wiring, `frontend/package.json`, `frontend/src/lib/`, `modules/shell|tabs|workspace|terminal`, `frontend/src/styles/`, kode Go. Tanpa dependency npm baru.

## Kontrak

Disediakan (signature dikunci, lihat overview 2.5):
```ts
// tree/registerTree.ts
export function registerFilesPanel(): void   // registerPanel({ id: 'files', area: 'sidebar', component: FilesPanel, order: 20 })
// tree/FilesPanel.svelte : tanpa props
```
`registerFilesPanel` dipanggil kelak oleh `index.ts` (milik `file-editor-save`); jangan memanggilnya di level modul.

Dipakai (impor relatif dari `../shared/...`; jangan memanggil `call('fs.*')` langsung dan jangan mengimpor ui-shell langsung):
- `shared/shellBridge.ts`: `activeWorkspace: Readable<ShellWorkspace | null>` (`{id, projects: [{id, path, name}]}`), `openFileTab(data: FileTabData)` dengan `FileTabData = {path (absolut), projectId, workspaceId}`.
- `shared/fsClient.ts`: `fsList(absDir: string, root?: string): Promise<FsEntry[]>` (di panel SELALU lewatkan `root` = `project.path`), `joinPath(dirAbs, name)`, kelas error `FsNotFoundError`, `FsInvalidError` (lainnya `Error` biasa).
- `shared/types.ts`: `FsEntry { name; type: 'file'|'dir'|'symlink'; size; mtime }` (BUKAN `isDir`; tanpa field `path`), `FileTabData`.
- `lib/registry.ts`: `registerPanel` (path impor dari `tree/` = `../../../lib/registry`; periksa ekspor nyata).

Bentuk data nyata: `fs.list` sudah terurut folder dulu lalu nama dan folder abaian sudah disaring daemon; panel TIDAK mengurutkan ulang dan tidak menambah aturan penyaringan. Symlink tidak diikuti daemon: panel menampilkannya sebagai entri non-folder (ikon tautan) dan klik membukanya sebagai file lewat `openFileTab`.

## Langkah

1. **Baca konteks.** Baca `shared/types.ts`, `shared/fsClient.ts`, `shared/shellBridge.ts` hasil Foundation, `frontend/src/lib/registry.ts`, dan bagaimana sidebar ui-shell merender panel (`modules/shell/`; hanya baca, untuk tahu apakah komponen panel menerima props dan token CSS apa yang ada di `frontend/src/styles/` bila sudah ada). Bila `shellBridge.ts` belum ada, hentikan dan laporkan blocker (Foundation belum selesai).

2. **`tree/treeModel.svelte.ts`** (logika reaktif, tanpa DOM). Ekspor kelas:
   ```ts
   export type NodeStatus = 'idle' | 'loading' | 'error';
   export interface TreeRow {
     key: string;            // path absolut (unik)
     name: string;
     path: string;           // path absolut
     root: string;           // project.path pemilik baris
     depth: number;          // 0 = akar project
     kind: 'project' | 'dir' | 'file' | 'symlink';
     expanded: boolean;      // hanya bermakna untuk project/dir
     status: NodeStatus;     // status muat folder (project/dir)
     error?: string;         // pesan untuk baris folder berstatus 'error'
     // baris pesan sintetis:
     placeholder?: 'loading' | 'empty' | 'error';   // bila diisi, baris bukan node nyata
   }
   export class TreeModel {
     rows: TreeRow[];                       // $derived: daftar DATAR hanya untuk folder terbuka
     projects: ShellProject[];              // $state, di-set lewat setProjects
     setProjects(projects: ShellProject[]): void;   // reset total (cache, expanded, status) lalu set akar
     toggle(row: TreeRow): void;            // buka/tutup folder atau project; muat bila belum ada cache
     retry(row: TreeRow): void;             // muat ulang satu folder berstatus error
     refreshAll(): void;                    // bersihkan seluruh cache, muat ulang HANYA folder yang sedang terbuka
     refreshDir(row: TreeRow): void;        // bersihkan cache folder itu (dan keturunannya), muat ulang bila terbuka
   }
   ```
   Detail implementasi:
   - State: `cache = new SvelteMap<string, FsEntry[]>()`, `expanded = new SvelteSet<string>()`, `status = new SvelteMap<string, {state: NodeStatus; error?: string}>()` (impor dari `svelte/reactivity`), `rootOf` (Map path -> root, diisi saat entri anak dibuat), `generation` (angka, naik tiap `setProjects`/`refresh*`).
   - `loadDir(absDir, root)`: bila `cache.has(absDir)` kembali tanpa memanggil `fsList` (kriteria: membuka ulang tidak memanggil ulang). Jika `status` sudah `loading`, kembali (hindari panggilan ganda). Set `loading`; `const gen = generation`; `await fsList(absDir, root)`; bila `gen !== generation` atau path tidak lagi relevan, BUANG hasil (anti hasil basi setelah ganti workspace/segarkan). Sukses: `cache.set(absDir, entries)`, status `idle`. Gagal: status `error` dengan pesan Indonesia berdasar kelas error (`FsNotFoundError` -> "Folder tidak ditemukan.", `FsInvalidError` -> "Path tidak valid.", lainnya -> `e.message` atau "Gagal memuat folder."); tidak menyimpan ke cache; juga `console.warn` boleh tetapi bukan satu-satunya penanda.
   - `rows`: telusuri berurutan project -> (bila `expanded`) anak dari `cache` secara rekursif ke daftar datar. Untuk folder terbuka: status `loading` tanpa cache -> satu baris `placeholder:'loading'` ("Memuat...") di `depth+1`; status `error` -> satu baris `placeholder:'error'` membawa `error` dan `key` folder induk; cache kosong -> satu baris `placeholder:'empty'` ("Folder kosong"). Folder tertutup tidak menghasilkan baris apa pun untuk isinya. Path anak = `joinPath(parentAbs, entry.name)`; `kind` dari `entry.type` (`'dir'` -> `'dir'`, `'symlink'` -> `'symlink'`, lainnya `'file'`).
   - `toggle`: untuk `project`/`dir`: bila `expanded` hapus dari set; bila tidak, tambah ke set lalu `loadDir`. Untuk file/symlink tidak melakukan apa-apa (pembukaan file ditangani komponen).
   - `refreshAll`: `generation++`, `cache.clear()`, `status.clear()`, lalu `loadDir` untuk setiap path di `expanded` yang valid (project dan folder terbuka; gunakan `rootOf`). `refreshDir`: hapus cache path itu dan semua cache berawalan `path + '/'`, `generation++` hanya untuk mencegah hasil basi folder itu (atau gunakan counter per-path), lalu muat ulang bila terbuka. Folder keturunan yang masih terbuka ikut dimuat ulang.
   - `setProjects`: `generation++`, kosongkan cache/expanded/status/rootOf, set `projects`. Project tunggal TIDAK dibuka otomatis (konsisten dan murah); pengguna membuka sendiri.
   - Tanpa `setInterval`/`setTimeout` berkala, tanpa `watchFile`, tanpa memanggil `fs.watch`.

3. **`tree/FilesPanel.svelte`** (menimpa stub; hapus komentar `STUB(foundation)`). Svelte 5 runes, TypeScript, tanpa props.
   - Buat `const model = new TreeModel()`. Dengarkan `activeWorkspace` (impor dari `../shared/shellBridge`): `$effect` yang berlangganan (`const unsub = activeWorkspace.subscribe(ws => ...)`, kembalikan `unsub` untuk cleanup) dan memanggil `model.setProjects(ws?.projects ?? [])` HANYA bila identitas workspace atau daftar `{id,path}` project berubah (bandingkan string `ws.id + '|' + projects.map(p => p.id + ':' + p.path).join(',')` dengan nilai terakhir), supaya update store yang tidak relevan tidak mereset tree. Simpan `workspaceId` terakhir di variabel lokal untuk `openFileTab`.
   - Header: judul "Files" dan tombol segarkan kecil (`aria-label="Segarkan"`, ikon teks/SVG inline sederhana, memanggil `model.refreshAll()`). Bila tidak ada workspace aktif: pesan "Tidak ada workspace aktif." Bila workspace tanpa project: "Workspace ini belum punya project."
   - Daftar: satu `<div role="tree" tabindex="0">` dengan `{#each model.rows as row (row.key + ':' + (row.placeholder ?? ''))}`; tiap baris `<div role="treeitem" aria-level={row.depth + 1} aria-expanded={...}>` dengan indentasi `padding-left` = `depth * 12px + 8px`, tinggi baris tetap (mis. 22px), `white-space: nowrap; overflow: hidden; text-overflow: ellipsis`, `title` = path. Tanpa animasi/transisi. Tidak membuat komponen terpisah per node (satu `each` datar saja agar ribuan entri tetap responsif); bila perlu `content-visibility: auto` pada baris boleh dipakai sebagai optimasi ringan.
   - Isi baris: panah (untuk project/dir: "▸"/"▾"), nama (project memakai `project.name`, tebal), ikon sederhana untuk file/symlink. Baris `placeholder` render teks redup; `placeholder:'error'` render pesan error + tombol "Coba lagi" (`onclick` -> `model.retry` pada folder induk; hentikan propagasi agar tidak men-toggle).
   - Klik: baris project/dir -> `model.toggle(row)` dan menjadikan baris fokus; baris file/symlink -> `openFileTab({ path: row.path, projectId, workspaceId })` dengan `projectId` = id project dari `row.root` (cari di `model.projects` berdasar `path === row.root`) dan `workspaceId` = id workspace aktif terakhir. Satu klik, tanpa klik ganda. Tombol segarkan per folder: tampilkan ikon kecil di baris folder terbuka saat di-hover/difokuskan, memanggil `model.refreshDir(row)` (klik pada ikon tidak men-toggle).
   - Keyboard (saran, implementasikan): state `focusedKey`; handler `onkeydown` pada kontainer tree: ArrowDown/ArrowUp pindah fokus antar baris NODE (lewati baris placeholder), ArrowRight pada folder tertutup membuka, pada folder terbuka pindah ke anak pertama; ArrowLeft pada folder terbuka menutup, selain itu pindah ke induk (cari baris dengan `depth` lebih kecil di atasnya); Enter/Space pada folder = toggle, pada file = buka; Home/End ke baris pertama/terakhir. `preventDefault` untuk tombol yang ditangani. Pertahankan baris fokus terlihat via `scrollIntoView({block:'nearest'})` dari `$effect` kecil (bukan timer). Baris fokus diberi `aria-selected`/kelas `focused`.
   - Gaya: `<style>` scoped, gunakan variabel CSS ui-shell bila ada (hasil langkah 1) dengan fallback, mis. `color: var(--text, inherit)`, `background: var(--hover, color-mix(in srgb, currentColor 10%, transparent))`; tanpa variabel cocok pakai `inherit`/`currentColor`. Kontainer `overflow: auto`, `height: 100%`, `font-size` kecil, kursor pointer pada baris.
   - Teks UI Indonesia ("Memuat...", "Folder kosong", "Coba lagi", "Segarkan"); identifier Inggris.

4. **`tree/registerTree.ts`** (menimpa stub):
   ```ts
   import { registerPanel } from '../../../lib/registry';   // sesuaikan dengan ekspor nyata
   import FilesPanel from './FilesPanel.svelte';
   export function registerFilesPanel(): void {
     registerPanel({ id: 'files', area: 'sidebar', component: FilesPanel, order: 20 });
   }
   ```
   Tanpa efek samping saat diimpor.

5. **Verifikasi build mandiri.** `cd frontend && npm run build` harus lolos (dan `npm run check` bila ada; kegagalan lama di luar path ini bukan tanggung jawab). Untuk uji pemuatan, BOLEH membuat `frontend/src/modules/files/index.ts` sementara yang hanya memanggil `registerFilesPanel()`, tetapi HARUS dihapus sebelum selesai (file itu milik `file-editor-save`); pastikan tidak tertinggal. Jalankan pemeriksaan: `grep -rn "setInterval\|watchFile\|fs\.watch\|innerHTML\|call('fs" frontend/src/modules/files/tree/` harus kosong, dan `grep -n "STUB(foundation)" frontend/src/modules/files/tree/` harus kosong.

6. **Pemeriksaan akhir path.** `git status`/`git diff --stat` hanya menunjukkan perubahan di `frontend/src/modules/files/tree/` (lockfile dan `package.json` tidak berubah, tidak ada `index.ts`). Jangan commit; ikuti alur skill implement-feature.

## Perubahan

- Diubah (menimpa stub Foundation): `frontend/src/modules/files/tree/FilesPanel.svelte`, `frontend/src/modules/files/tree/registerTree.ts`
- Dibuat: `frontend/src/modules/files/tree/treeModel.svelte.ts`
- Tidak ada file lain; tidak ada `index.ts`.

## Wiring

Yang diekspos untuk langkah wiring (JANGAN disambungkan sendiri):
- `registerFilesPanel(): void` dari `tree/registerTree.ts`. Langkah wiring / `file-editor-save` memanggilnya dari `frontend/src/modules/files/index.ts` (`import { registerFilesPanel } from './tree/registerTree'; registerFilesPanel();`).
- Konsistensi yang diperiksa wiring: `openFileTab` (shellBridge) menghasilkan tab yang bisa didekode `decodeFileTab` milik `FileTab`; panel hanya memakai `openFileTab`, tidak tahu skema datanya.

## Selesai bila

- [ ] Panel `files` terdaftar lewat `registerFilesPanel()` dengan `area:'sidebar'`, `order: 20`, tanpa mengedit file shell/registry.
- [ ] Menampilkan satu akar per project workspace aktif; berganti workspace mereset tree ke akar project baru (hasil `fsList` basi dari workspace lama diabaikan).
- [ ] Membuka folder memanggil `fsList(absDir, project.path)` tepat sekali; membuka ulang tidak memanggil lagi sampai disegarkan (header atau per folder).
- [ ] Klik file (dan Enter) memanggil `openFileTab({path absolut, projectId, workspaceId})` dengan nilai benar.
- [ ] Error `fsList` tampil inline pada node dengan tombol "Coba lagi" yang memuat ulang folder itu.
- [ ] Daftar datar hanya berisi baris folder terbuka; navigasi panah/Enter berfungsi.
- [ ] Tidak ada `setInterval`/polling, tidak ada `fs.watch`/`watchFile`, tidak ada `call('fs.` dan tidak ada `innerHTML` di `tree/`.
- [ ] Tidak ada `index.ts` dibuat/tertinggal, tidak ada file di luar `frontend/src/modules/files/tree/` berubah, tidak ada marker `STUB(foundation)` tersisa di `tree/`.
- [ ] Guard: `cd frontend && npm run build` sukses dengan fitur ini terpasang (tanpa unit test).
