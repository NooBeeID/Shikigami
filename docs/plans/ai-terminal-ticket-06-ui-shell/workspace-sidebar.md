# Plan: Sidebar workspace dan project

## Metadata

- Tiket: `ai-terminal-ticket-06-ui-shell` (`docs/tickets/ai-terminal-ticket-06-ui-shell.md`)
- Fitur: `workspace-sidebar`
- ownedPaths: `frontend/src/modules/workspace/`
- Depends on foundation: ya (store dan tipe di `lib/stores/` dibuat Foundation `00-foundation.md`; boleh masih berbadan stub saat fitur ini dikerjakan, nama ekspor sudah final)
- Rujukan: `overview.md` bagian 1.2 (keputusan 5 dan 6), 2.2 (tipe), 2.3 (store), 2.4 (panel), 2.5 (CSS variables), 2.7 (teks UI), 6 (build guard), 7 (aturan paralel).

## Tujuan

Membuat panel sidebar `workspace-sidebar` yang memuat: pemilih workspace (buat, ganti nama, hapus), daftar project workspace aktif (tambah lewat input path teks, hapus, penanda aktif), daftar sesi workspace aktif (buka/fokus tab, kill, tanda exited), serta tombol "Sesi baru" dengan pilihan provider. Daftar sesi hanya bergantung pada workspace aktif, sama persis saat berpindah project (anti-goal 2). Error daemon tampil sebagai pesan singkat di sidebar, tidak membuat UI crash. Tanpa polling, tanpa dependency baru, tanpa unit test.

## Path yang dimiliki

HANYA `frontend/src/modules/workspace/` (membuat file baru di dalamnya). Dilarang menyentuh: `lib/stores/` (hanya impor lewat barrel, jangan diedit), `lib/registry.ts`, `lib/ipc.ts`, `main.ts`, `App.svelte`, `modules/shell/`, `modules/tabs/`, `modules/terminal/`, `styles/`, `package.json`, `app/*`, `internal/*`. Jangan menyambungkan wiring apa pun sendiri.

## Kontrak

Dipakai (semua impor store HANYA lewat barrel `../../lib/stores`, path relatif dari `modules/workspace/`):

- Tipe: `Workspace`, `Project`, `Session`, `Provider` (gunakan `import type`, proyek memakai `verbatimModuleSyntax`).
- Store (pakai `$store`): `workspaces`, `activeWorkspaceId`, `activeProjectId`, `activeWorkspace`, `activeProject`, `sessions`, `providers`, `activeTabId`.
- Aksi workspace: `selectWorkspace(id)`, `selectProject(id)` (sinkron, hanya set `activeProjectId`), `createWorkspace(name)`, `renameWorkspace(id, name)`, `deleteWorkspace(id)`, `addProject(path, name?)`, `removeProject(id)`.
- Aksi sesi/tab: `createSession(providerId): Promise<Session>` (TIDAK membuka tab), `killSession(id)`, `openTab({type:'session', refId, title?})`.
- Registry: `registerPanel` dari `../../lib/registry`.
- Aksi store melempar `Error` bila daemon menolak; komponen WAJIB menangkap (try/catch) dan menampilkan `e.message` singkat (overview 1.2 keputusan 6).
- Bila butuh aksi/ekspor yang tidak ada di store, pakai `call` dari `../../lib/ipc` langsung dan catat di laporan; jangan edit store.

Disediakan:

- `registerPanel({ id: 'workspace-sidebar', area: 'sidebar', component: WorkspaceSidebar, order: 10 })` di `modules/workspace/index.ts`.
- `WorkspaceSidebar.svelte`: default export, tanpa prop.

Teks UI yang harus persis: "Tambah project", "Sesi baru", "Tambah project dulu". Selain itu bebas (bahasa Indonesia).

Aturan perilaku yang mengikat:

- `selectProject` hanya mengubah `activeProjectId`. Komponen tidak boleh memanggil `loadSessions`, tidak boleh menulis ke `sessions`, dan tidak ada `$effect` yang bereaksi pada `$activeProjectId` untuk memuat sesi. Daftar sesi dirender langsung dari `$sessions` tanpa filter/pengurutan berdasarkan project (urutan dari store apa adanya).
- Dilarang `setInterval`. Satu `setTimeout` sekali jalan untuk menghilangkan pesan error boleh.
- Tanpa impor `@xterm/*`.

## Langkah

### 1. Verifikasi prasyarat

- Pastikan `frontend/src/lib/stores/index.ts` ada dan mengekspor nama pada bagian Kontrak (dibuat Foundation). Bila belum ada, hentikan dan laporkan blocker; jangan membuat file di `lib/stores/`.
- Baca `frontend/src/lib/registry.ts` (sudah ada) untuk memastikan signature `registerPanel`. Baca `lib/stores/types.ts` bila ada.
- Pastikan `frontend/src/modules/workspace/` belum ada atau kosong; buat direktori.

### 2. `modules/workspace/errors.ts`

Helper kecil tanpa dependency:

```ts
export function errMessage(e: unknown): string  // Error -> e.message; string -> itself; lainnya -> 'terjadi kesalahan'; potong ke maks ~120 karakter
```

Dan helper dialog opsional (jalur sekunder, keputusan 5 overview):

```ts
export function hasNativeFolderDialog(): boolean
export async function pickFolder(): Promise<string | null>
```

`hasNativeFolderDialog` memeriksa defensif tanpa melempar: `(window as any).go?.main?.Bridge?.OpenDirectoryDialog` atau `(window as any).runtime?.OpenDirectoryDialog` bertipe `function`. `pickFolder` memanggil yang tersedia (argumen opsi `{ Title: 'Pilih folder project' }` untuk varian runtime; tanpa argumen untuk varian Bridge), mengembalikan string path atau `null` bila batal/kosong; semua dibungkus try/catch (kegagalan -> `null`). Jangan menambah binding Go. Saat ini hampir pasti tidak tersedia, sehingga input path teks adalah jalur utama.

### 3. `modules/workspace/WorkspacePicker.svelte`

Pemilih workspace, tanpa prop. State lokal Svelte 5 (`$state`):

- `mode: 'idle' | 'create' | 'rename'`, `draft: string`, `busy: boolean`.
- Callback prop `onerror: (msg: string) => void` (`$props()`), dipakai melaporkan error ke sidebar.

Markup:

- `<select>` berisi `$workspaces` (value = `id`, label = `name`), nilai terpilih = `$activeWorkspaceId`; `onchange` memanggil `selectWorkspace(id)` dalam try/catch -> `onerror`.
- Tombol ikon/teks kecil: "Baru" (mode `create`), "Ganti nama" (mode `rename`, `draft` diisi nama aktif; nonaktif bila tidak ada workspace aktif), "Hapus" (nonaktif bila tidak ada workspace aktif).
- Saat mode `create`/`rename`: tampilkan input teks + tombol "OK" dan "Batal"; Enter = OK, Escape = batal. Validasi: `draft.trim()` tidak boleh kosong (bila kosong, `onerror('Nama tidak boleh kosong')`). `create` memanggil `createWorkspace(name)` lalu `selectWorkspace(ws.id)` (aktifkan workspace baru); `rename` memanggil `renameWorkspace(activeId, name)`.
- Hapus: konfirmasi dua langkah inline (klik "Hapus" mengubah tombol jadi "Yakin?" + "Batal"), jangan pakai `window.confirm`. Konfirmasi memanggil `deleteWorkspace(activeId)`. Store yang mengurus pergantian workspace aktif setelah hapus.
- Tombol dinonaktifkan selama `busy` (cegah klik ganda). Semua aksi: `try { ... } catch (e) { onerror(errMessage(e)) } finally { busy = false }`.
- Bila `$workspaces` kosong: tampilkan teks "Belum ada workspace" dan tombol "Baru" tetap aktif.

### 4. `modules/workspace/ProjectList.svelte`

Daftar project workspace aktif, tanpa prop selain `onerror`.

- Sumber: `$activeWorkspace?.projects ?? []`. Tiap baris: nama project (`name`), path (`path`, gaya muted, `title` = path penuh, ellipsis di tengah/ujung), penanda aktif bila `project.id === $activeProjectId` (kelas `active` + titik/aksen `var(--accent)`), tombol hapus.
- Klik baris memanggil `selectProject(project.id)` (sinkron, TANPA await, tanpa efek lain). Baris dapat difokus dan Enter/Space juga memilih (gunakan `<button>` untuk area utama baris).
- Tombol hapus (per baris) memanggil `removeProject(project.id)` dalam try/catch -> `onerror`; konfirmasi dua langkah inline seperti workspace.
- Bagian tambah project: tombol "Tambah project" yang membuka form inline berisi `<input type="text" placeholder="/path/ke/folder">` (jalur utama). Enter/tombol "Tambah" memanggil `addProject(path.trim())`; path kosong -> `onerror('Path tidak boleh kosong')`. Sukses: kosongkan input, tutup form. Bila `hasNativeFolderDialog()` true, tampilkan tombol tambahan "Pilih folder..." yang memanggil `pickFolder()` dan mengisi input (tidak otomatis submit). Nonaktifkan "Tambah project" bila tidak ada workspace aktif.
- Bila tanpa project: tampilkan "Belum ada project".

### 5. `modules/workspace/SessionList.svelte`

Daftar sesi workspace aktif, tanpa prop selain `onerror`.

- Sumber: `$sessions` apa adanya (jangan filter berdasarkan `$activeProjectId`, jangan sort ulang).
- Tiap baris menampilkan: nama provider (cari di `$providers` berdasarkan `session.providerId` -> `name`; fallback `providerId`), nama project asal (cari di `$activeWorkspace?.projects` berdasarkan `projectId` -> `name`; fallback basename `cwd`), dan tanda status: `running` = titik hijau `var(--ok)`; `exited` = teks "exited" (+ `exitCode` bila ada) berwarna muted dan baris diredupkan.
- Baris aktif (tab sesi sedang aktif: `$activeTabId === 'session:' + session.id`) diberi kelas `active`.
- Klik area utama baris: `openTab({ type: 'session', refId: session.id, title: <provider name> })` (dedup/aktifkan ditangani store). Sesi `exited` tetap boleh dibuka (menampilkan scrollback).
- Tombol "Kill" (hanya untuk `running`; sembunyikan atau nonaktifkan untuk `exited`): `killSession(session.id)` dalam try/catch -> `onerror`. Tidak menutup tab (itu urusan `session-tabs`/pengguna).
- Bila kosong: "Belum ada sesi".
- Gunakan `{#each $sessions as s (s.id)}` dengan key `id`.

### 6. `modules/workspace/NewSession.svelte`

Tombol "Sesi baru" + pilihan provider, tanpa prop selain `onerror`.

- State: `providerId: string` (`$state`), `busy: boolean`.
- Provider yang dapat dipilih: `$providers` yang `available !== false`. Default `providerId`: pertahankan pilihan bila masih valid; bila tidak, pilih provider pertama yang tersedia (turunkan dengan `$derived`/`$effect` sederhana yang HANYA bergantung pada `$providers` dan `providerId`, bukan pada `activeProjectId`). Provider tidak tersedia (`available === false`) boleh ditampilkan sebagai opsi `disabled` dengan label "(tidak tersedia)".
- `<select>` provider + tombol "Sesi baru".
- Tombol nonaktif bila: tidak ada `$activeProject` (tampilkan petunjuk teks persis "Tambah project dulu" di dekat tombol, juga sebagai `title` tombol), atau tidak ada provider terpilih, atau `busy`. Bila ada project tetapi tidak ada provider: petunjuk "Provider tidak tersedia".
- Klik: 
  ```ts
  async function start() {
    busy = true;
    try {
      const s = await createSession(providerId);   // workspace + project aktif, cwd = project.path
      const p = $providers.find((x) => x.id === s.providerId);
      openTab({ type: 'session', refId: s.id, title: p?.name ?? s.providerId });
    } catch (e) { onerror(errMessage(e)); }
    finally { busy = false; }
  }
  ```
  `createSession` tidak membuka tab, jadi `openTab` WAJIB dipanggil di sini.

### 7. `modules/workspace/WorkspaceSidebar.svelte`

Komponen akar panel, tanpa prop.

- State `error: string` (`$state('')`) dan `timer` (variabel biasa untuk id `setTimeout`).
- Fungsi `showError(msg: string)`: set `error = msg`, `clearTimeout(timer)`, `timer = setTimeout(() => (error = ''), 6000)` (satu kali jalan, bukan interval). Bersihkan timer di `onDestroy`.
- Susunan vertikal, tiap bagian diberi judul kecil (huruf kapital muted): "Workspace" (`<WorkspacePicker onerror={showError} />`), "Project" (`<ProjectList onerror={showError} />`), "Sesi" (`<SessionList onerror={showError} />`), lalu `<NewSession onerror={showError} />` (boleh diletakkan di atas daftar sesi atau di bawahnya, tetapi selalu terlihat tanpa scroll bila memungkinkan).
- Banner error: bila `error` tidak kosong, tampilkan satu baris singkat (`role="alert"`, warna `var(--danger, #d33)`) dengan tombol tutup kecil.
- Gaya (`<style>` lokal, tanpa framework): pakai variabel tema dengan fallback, mis. `var(--fg, #222)`, `var(--fg-muted, #888)`, `var(--bg-hover, #0001)`, `var(--border, #8884)`, `var(--accent, #3b82f6)`, `var(--ok, #22c55e)`, `var(--danger, #d33)`. Kontainer `display:flex; flex-direction:column; height:100%; overflow:auto; font-size:13px; gap:12px; padding:8px`. Jangan mendefinisikan variabel tema (milik `shell-layout`). Jangan mengatur lebar sidebar (milik `SidebarArea`/`--sidebar-w`).
- Teks panjang diberi `text-overflow: ellipsis; white-space: nowrap; overflow: hidden` agar sidebar tidak melebar.
- Aksesibilitas dasar: tombol berupa `<button type="button">` dengan `aria-label`/`title`, input berlabel.

### 8. `modules/workspace/index.ts`

```ts
import { registerPanel } from '../../lib/registry';
import WorkspaceSidebar from './WorkspaceSidebar.svelte';

registerPanel({ id: 'workspace-sidebar', area: 'sidebar', component: WorkspaceSidebar, order: 10 });
```

Tidak ada efek samping lain (jangan memanggil `loadWorkspaces` dsb.; inisialisasi store milik `initStores` dari `shell-layout`).

### 9. Verifikasi

- `cd frontend && npm run build` hijau (walaupun store masih stub dan `shell-layout`/`session-tabs` belum selesai).
- `cd frontend && npm run check`: tidak ada error tipe pada file di `modules/workspace/`.
- Pemeriksaan grep (semua harus kosong):
  - `grep -rn "@xterm" frontend/src/modules/workspace`
  - `grep -rn "setInterval" frontend/src/modules/workspace`
  - `grep -rnE "from '(\.\./)+lib/stores/[a-z]" frontend/src/modules/workspace` (impor store hanya lewat barrel `lib/stores`)
  - `grep -rn "loadSessions" frontend/src/modules/workspace`
- Pastikan tidak ada perubahan di luar `frontend/src/modules/workspace/` (`package.json`, `package-lock.json`, `lib/`, `App.svelte` tak berubah).
- Uji manual opsional bila store sudah final: ganti project berkali-kali, daftar sesi tidak berubah isi maupun urutan.

## Perubahan

Dibuat (semua di `frontend/src/modules/workspace/`):

- `index.ts`
- `WorkspaceSidebar.svelte`
- `WorkspacePicker.svelte`
- `ProjectList.svelte`
- `SessionList.svelte`
- `NewSession.svelte`
- `errors.ts`

Tidak ada file yang diubah atau dihapus di luar path tersebut.

## Wiring

Fitur ini hanya mengekspos:

- `modules/workspace/index.ts` yang memanggil `registerPanel({ id: 'workspace-sidebar', area: 'sidebar', component, order: 10 })`. Ditemukan otomatis oleh `import.meta.glob('./modules/*/index.ts', { eager: true })` di `main.ts`; tidak ada yang perlu diedit.
- Panel dirender oleh `SidebarArea` milik `shell-layout`; inisialisasi store (`initStores`) dan event `session.exit` juga milik `shell-layout`. Tab yang dibuka via `openTab` dirender oleh `TabBar`/`TabContent` milik `session-tabs`.

Jangan menyambungkan apa pun sendiri. Agen wiring hanya memverifikasi bahwa panel muncul di sidebar dan perilaku bagian "Selesai bila". Catat di laporan akhir: dialog folder native tidak tersedia dari JS Wails v2 sehingga input path teks dipakai sebagai jalur utama; juga aksi store yang kurang bila ada.

## Selesai bila

- [ ] `cd frontend && npm run build` sukses (guard utama; tanpa unit test); `npm run check` tanpa error pada `modules/workspace/`.
- [ ] Workspace bisa dibuat, dipilih, diganti nama, dan dihapus dari sidebar (hapus dengan konfirmasi inline).
- [ ] Project bisa ditambah lewat input path teks (tombol "Tambah project"; dialog native hanya bila terdeteksi) dan dihapus; daftar mengikuti workspace aktif; project aktif bertanda.
- [ ] Memilih project hanya memanggil `selectProject`; daftar sesi tidak berubah isi/urutan saat berpindah project (tidak ada filter/sort/`loadSessions` berdasarkan project).
- [ ] Klik sesi memanggil `openTab({type:'session', refId})`; tombol kill memanggil `killSession`; sesi `exited` bertanda.
- [ ] "Sesi baru" dengan pilihan provider memanggil `createSession(providerId)` lalu `openTab`; nonaktif dengan petunjuk "Tambah project dulu" bila tanpa project.
- [ ] Error dari aksi store tampil sebagai pesan singkat (banner `role="alert"`), UI tidak crash.
- [ ] Panel terdaftar via `registerPanel` id `workspace-sidebar`, area `sidebar`, order 10.
- [ ] Hanya `frontend/src/modules/workspace/` yang berubah; impor store hanya lewat barrel `lib/stores`; tanpa `@xterm`, tanpa `setInterval`, tanpa dependency baru.
