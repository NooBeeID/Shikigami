# Plan: Sidebar kanan File dan Open Folder

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-04-shell-sidebars.md` (epic `shikigami-v2-ui-ticket-04-shell-sidebars`)
- Fitur: `sidebar-right-files`
- ownedPaths: `frontend/src/modules/sidebar-right/`
- Depends on foundation: ya (`shell-frame`: butuh `modules/shell/registry.ts`, `modules/shell/helpers.ts`, `modules/shell/index.ts` sudah ada; lihat `overview.md` bagian 2.3 dan 2.5)

## Tujuan

Menyediakan isi tab kanan "Files" untuk `RightPanel`: pemilih project (sinkron dengan `activeProjectId` lewat `selectProject`), tree project terpilih memakai `TreeModel` dari `modules/files/tree` dengan klik file membuka pane file di layout (bukan tab) lewat `openFilePane`, tombol "Open Folder…" lewat `openFolder()`, dan state kosong ("Belum ada project") dengan input path teks cadangan memakai `addProject`. Tab didaftarkan lewat `registerRightTab` di `index.ts`. Kontrak umum: `overview.md` (jangan diulang; ikuti persis).

## Path yang dimiliki

HANYA `frontend/src/modules/sidebar-right/` (berkas baru: `FilesTab.svelte`, `index.ts`; boleh menambah komponen kecil lain di folder ini). Jangan sentuh `App.svelte`, `modules/shell/` (termasuk `register.ts`), `modules/files/`, `modules/openfolder/`, `modules/workspace/`, `lib/stores/*`, `main.ts`, `package.json`, maupun path fitur lain (topbar, sidebar-left, palette, notifications). Bila butuh perubahan di luar folder ini, laporkan sebagai blocker.

## Kontrak

Disediakan:
- `frontend/src/modules/sidebar-right/FilesTab.svelte`: komponen tanpa props wajib (default export Svelte). Dirender `RightPanel` lewat registri.
- `frontend/src/modules/sidebar-right/index.ts`: efek samping `registerRightTab({ id: 'files', title: 'Files', component: FilesTab, order: 10 })` (idempoten per id). Boleh juga `export { default as FilesTab } from './FilesTab.svelte'`.

Dipakai (sudah ada, jangan diubah):
- Dari `../shell` (foundation, overview 2.3 dan 2.5): `registerRightTab`, `openFilePane({projectId, filePath, workspaceId?, title?}): string`, `openFolder(): void`, `basename(path)`.
- Dari `../../lib/stores` (`lib/stores/index.ts`): `activeWorkspace`, `activeWorkspaceId`, `activeProjectId`, `selectProject(id)`, `addProject(path, name?): Promise<Project>`; tipe `Project {id,path,name}`.
- Dari `../files/tree/treeModel.svelte`: `TreeModel` (`setProjects(projects)`, `rows`, `projects`, `toggle(row)`, `retry(row)`, `refreshAll()`, `refreshDir(row)`) dan tipe `TreeRow {key,name,path,root,depth,kind:'project'|'dir'|'file'|'symlink',expanded,status,error?,placeholder?}`. Tipe proyek yang diterima `setProjects` adalah `ShellProject {id,path,name}` (kompatibel dengan `Project`).
- Dari `../workspace/errors`: `errMessage(e)` untuk pesan galat singkat (impor langsung berkas ini; jangan salin).
- Tema: variabel CSS `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger` (tanpa warna hardcode; fallback boleh).
- Menu event `menu:open-folder` ditangani `modules/openfolder` (dialog + tambah project + toast); fitur ini TIDAK menambah handler untuk event itu.

## Langkah

Prasyarat (hanya baca): pastikan ada `modules/shell/index.ts` yang mengekspor `registerRightTab`, `openFilePane`, `openFolder`, `basename`; `modules/files/tree/treeModel.svelte.ts`. Baca `modules/files/tree/FilesPanel.svelte` sebagai acuan perilaku tree (jangan diubah). Bila ekspor foundation belum ada, hentikan dan laporkan blocker.

1. **Buat folder dan `FilesTab.svelte`** di `frontend/src/modules/sidebar-right/`. Svelte 5 runes, `<script lang="ts">`. Struktur dari atas ke bawah:
   - Header: pemilih project (`<select>`), dan tombol segarkan kecil (memanggil `model.refreshAll()`).
   - Badan: salah satu dari tiga keadaan (lihat langkah 6): tanpa workspace, tanpa project (state kosong), atau tree.
   - Footer: tombol "Open Folder…" (selalu tampil bila ada workspace aktif).
2. **State dan sumber data.**
   - `const model = new TreeModel();`
   - `let projects = $state<Project[]>([])`, `let hasWorkspace = $state(false)`, `let workspaceId = ''`, diisi dari `activeWorkspace.subscribe(...)` di dalam `$effect` (kembalikan fungsi unsubscribe), seperti pola `FilesPanel`. `projects = ws?.projects ?? []`.
   - `activeProjectId` dibaca lewat `$effect` + `subscribe` ke `let activeId = $state<string | null>(null)` (atau auto-subscribe `$activeProjectId`).
   - `const selected = $derived(projects.find(p => p.id === activeId) ?? projects[0] ?? null)`. Bila `activeId` tidak cocok dengan project mana pun, tampilkan project pertama sebagai terpilih (tampilan saja; tidak memanggil `selectProject` otomatis).
3. **Sinkron tree dengan project terpilih.** `$effect` yang bergantung pada `selected`: hitung `sig = selected ? selected.id + '|' + selected.path : ''`; bila sama dengan `lastSig` jangan lakukan apa pun (hindari reset tree yang menutup folder terbuka saat store workspace berubah karena hal lain); selain itu `lastSig = sig; focusedKey = null; model.setProjects(selected ? [selected] : [])`. Tree hanya menampilkan SATU project (project terpilih); jangan memanggil `setProjects` dengan semua project.
4. **Pemilih project.** `<select aria-label="Pilih project" value={selected?.id ?? ''} onchange={(e) => selectProject(e.currentTarget.value)}>` dengan `<option value={p.id}>{p.name}</option>` (atribut `title={p.path}`); tampilkan hanya bila `projects.length > 0`. Memilih memanggil `selectProject(id)` sehingga sidebar kiri/palet ikut sinkron; sebaliknya perubahan `activeProjectId` dari luar langsung tercermin di `<select>` lewat `selected`. Pemilih memenuhi lebar tab, teks dipotong (`text-overflow: ellipsis`, `min-width: 0`).
5. **Tree dan klik file.** Salin perilaku `FilesPanel` ke komponen ini (jangan impor atau ubah `FilesPanel`): `nodeRows = $derived(model.rows.filter(r => !r.placeholder))`, render `model.rows` dengan placeholder `Memuat...` / `Folder kosong` / galat + tombol "Coba lagi" (`model.retry`), ikon segarkan folder (`model.refreshDir`), navigasi keyboard (ArrowUp/Down/Left/Right, Home, End, Enter, Space) dan `role="tree"`/`treeitem`, penanda `focusedKey`, `scrollIntoView({block:'nearest'})` pada baris fokus. Pembeda UTAMA dari v1:
   ```ts
   function openFile(row: TreeRow) {
     if (!selected) return;
     openFilePane({ projectId: selected.id, filePath: row.path, workspaceId, title: basename(row.path) });
   }
   function activate(row: TreeRow) {
     if (row.kind === 'project' || row.kind === 'dir') model.toggle(row);
     else openFile(row); // file dan symlink: klik tunggal; tidak memakai openFileTab (jalur tab)
   }
   ```
   Klik folder/baris project hanya memperluas atau menutup. Tidak ada klik ganda. `openFilePane` sendiri memfokuskan pane yang sudah ada bila `projectId` + `filePath` sama.
6. **State kosong dan input cadangan.**
   - `!hasWorkspace`: pesan "Tidak ada workspace aktif." (tanpa tombol).
   - `hasWorkspace && projects.length === 0`: blok state kosong berisi teks "Belum ada project", ajakan singkat, tombol "Open Folder…" (memanggil `openFolder()`), dan form input path cadangan: `<form onsubmit>` dengan `<input type="text" placeholder="/path/ke/project" aria-label="Path project">` dan tombol "Tambah". Submit (Enter atau klik): `preventDefault`, trim; kosong -> tidak melakukan apa pun; selain itu `busy = true`, `const p = await addProject(path)` lalu `selectProject(p.id)` dan kosongkan input; galat -> `error = errMessage(e)` ditampilkan inline (warna `--danger`), input dipertahankan; `finally busy = false`. Tombol dan input `disabled` selama `busy`.
   - Form input path yang sama juga disediakan sebagai bagian collapsible kecil di footer bila `projects.length > 0`? TIDAK perlu: cukup di state kosong sesuai spec bagian 6 (input path tetap ada sebagai cadangan).
7. **Open Folder dan seleksi project baru.** Tombol "Open Folder…" memanggil `openFolder()` (helper foundation: `emitMenuEvent('menu:open-folder')`; dialog dan penambahan project ditangani `modules/openfolder`). Dialog dibatalkan -> tidak ada perubahan (tidak ada yang perlu ditangani). Karena `addProject` hanya memilih project baru bila belum ada project aktif, agar "project baru langsung terpilih di pemilih" (kriteria tiket) tambahkan deteksi:
   - `let pendingPick = false; let pendingTimer: ReturnType<typeof setTimeout> | undefined;` dan `let knownIds = new Set<string>()` (diisi dari `projects` setiap perubahan).
   - Saat tombol diklik: `pendingPick = true; clearTimeout(pendingTimer); pendingTimer = setTimeout(() => (pendingPick = false), 60000)` (timer sekali jalan, bukan polling), lalu `openFolder()`.
   - Di `$effect` yang memantau `projects`: bila `pendingPick` dan ada id yang tidak ada di `knownIds` (project baru) -> `pendingPick = false; clearTimeout(pendingTimer); selectProject(newId)` (ambil id baru terakhir bila lebih dari satu); selalu perbarui `knownIds` setelahnya. Bersihkan timer saat komponen dihancurkan (return cleanup di `$effect`/`onDestroy`).
   - Perubahan workspace aktif (`workspaceId` berbeda) mereset `pendingPick = false` dan `knownIds` tanpa memilih apa pun.
8. **Gaya.** Blok `<style>` di komponen: `:host`-setara `.files-tab { height:100%; display:flex; flex-direction:column; min-width:0; min-height:0; font-size:12px; color: var(--fg) }`; header `flex:none` dengan padding 6px 8px dan `border-bottom: 1px solid var(--border, ...)`; `.tree { flex:1; overflow:auto; outline:none }`; baris 22px dengan `padding-left: {depth*12+8}px` dan hover `var(--bg-hover)`; footer `flex:none` dengan `border-top`. Tombol primer "Open Folder…" lebar penuh, memakai gaya global tombol `styles/base.css`. Hanya variabel tema, fallback `color-mix(...)` boleh. Layout harus tidak meluber pada lebar `RIGHT_MIN` (220px). Teks UI bahasa Indonesia kecuali label baku `Files`, `Open Folder…`.
9. **`index.ts`.**
   ```ts
   import { registerRightTab } from '../shell';
   import FilesTab from './FilesTab.svelte';

   registerRightTab({ id: 'files', title: 'Files', component: FilesTab, order: 10 });

   export { FilesTab };
   ```
   Impor `registerRightTab` dari `'../shell'` (index modul foundation). Dimuat otomatis oleh `import.meta.glob` di `main.ts` dan dijamin oleh `modules/shell/register.ts` (wiring); idempoten, jadi dobel muat aman.
10. **Guard.** `cd frontend && npm run build` (jalankan `npm install` dulu bila `node_modules` belum ada di worktree). Perbaiki galat tipe/svelte-check pada berkas folder ini. `make build` dari root bila sempat. Tidak ada `console.log`, tidak ada polling, tidak ada dependency baru, tidak ada unit test.

## Perubahan

Dibuat:
- `frontend/src/modules/sidebar-right/FilesTab.svelte`
- `frontend/src/modules/sidebar-right/index.ts`

Diubah: tidak ada. Tidak ada file di luar `frontend/src/modules/sidebar-right/` yang disentuh.

## Wiring

Yang diekspos untuk disambungkan nanti oleh langkah `shell-wiring` (JANGAN disambungkan di fitur ini):
- `modules/sidebar-right/index.ts` mendaftarkan tab `files` (`order: 10`) lewat `registerRightTab`. Wiring cukup memastikan `modules/shell/register.ts` mengimpor `'../sidebar-right'` (side-effect). `RightPanel` (foundation) lalu merender `FilesTab` dari registri tanpa impor langsung.
- Tidak ada perubahan `App.svelte`, `register.ts`, atau `modules/shell/` dari fitur ini.

## Selesai bila

- `FilesTab.svelte` dan `index.ts` ada di `frontend/src/modules/sidebar-right/` dan `index.ts` mendaftarkan `{id:'files', title:'Files', order:10}` lewat `registerRightTab` dari `../shell`.
- Pemilih project mencerminkan `activeWorkspace.projects`, memanggil `selectProject`, dan tree berganti ke project terpilih tanpa me-reset tree saat store berubah karena hal lain.
- Tree memuat anak folder secara malas lewat `TreeModel`; klik folder hanya memperluas/menutup; klik file memanggil `openFilePane` (bukan `openFileTab`), dan klik file yang sama memfokuskan pane yang ada.
- "Open Folder…" memanggil `openFolder()`; project baru yang muncul sesudahnya otomatis terpilih; dialog dibatalkan tidak mengubah apa pun.
- State kosong tanpa project menampilkan "Belum ada project", tombol Open Folder…, dan input path cadangan yang memakai `addProject` lalu `selectProject`; galat tampil inline.
- Tidak ada file di luar ownedPaths yang berubah; tidak ada polling, `console.log`, atau dependency baru.
- Guard: `cd frontend && npm run build` sukses (dan `make build` bila sempat). Tanpa unit test.
