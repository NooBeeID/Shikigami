# Plan: Layout, store, dan area panel

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-06-ui-shell.md` (epic 06 UI Shell)
- Fitur: `shell-layout`
- ownedPaths: `frontend/src/App.svelte`, `frontend/src/modules/shell/`, `frontend/src/lib/stores/`, `frontend/src/styles/`
- Depends on foundation: ya. `00-foundation.md` sudah membuat `lib/stores/{types,workspaces,sessions,tabs,providers,init,index}.ts` (badan stub bertanda `STUB(foundation)` / `FINAL(foundation)`) dan stub `modules/tabs/{TabBar,TabContent}.svelte`. Fitur ini MENGGANTI badan stub store (bukan membuat file bernama lain). Bila `lib/stores/` ternyata belum ada saat implementasi, buat file-file itu dengan ekspor final sesuai kontrak di bawah (jangan berhenti), dan catat di laporan.

Konteks kode yang sudah ada (dibaca dari repo, jangan diubah): `lib/ipc.ts` mengekspor `inWails`, `call<T>(method, params?)`, `onEvent(topicPrefix, fn(topic, payload)) => unsubscribe`. `lib/registry.ts` mengekspor `panels(area): Readable<PanelSpec[]>` (urut `order` lalu `id`) dan `tabTypes()`. `main.ts` memuat modul lewat `import.meta.glob('./modules/*/index.ts')` lalu `mount(App)`; karena `main.ts` tidak boleh diedit dan `index.html` bukan milik fitur ini, CSS global harus diimpor dari `App.svelte`. Proyek memakai `verbatimModuleSyntax` (impor tipe dengan `import type`) dan Svelte 5.

## Tujuan

Mengganti placeholder Hello World di `App.svelte` dengan layout tiga area (sidebar, tab utama, statusbar) yang dirender dari registry panel; mengisi store final (`workspaces`, `sessions`, `tabs`, `providers`, `init`) dengan logika IPC nyata sesuai overview 2.3 dan 1.2; menampilkan layar error koneksi dengan tombol "Coba lagi"; dan mendefinisikan tema gelap/terang termasuk `--term-bg`, `--term-fg`, `--term-cursor`. Daftar sesi tidak boleh berubah saat project berganti, dan tidak ada polling.

## Path yang dimiliki

HANYA path berikut yang boleh dibuat/diubah:

- `frontend/src/App.svelte`
- `frontend/src/modules/shell/` (`SidebarArea.svelte`, `StatusBar.svelte`; `index.ts` TIDAK dibuat karena modul ini tidak mendaftarkan apa pun)
- `frontend/src/lib/stores/` (`types.ts`, `workspaces.ts`, `sessions.ts`, `tabs.ts`, `providers.ts`, `init.ts`, `index.ts`)
- `frontend/src/styles/` (`theme.css`, `base.css`)

Dilarang menyentuh: `lib/registry.ts`, `lib/ipc.ts`, `main.ts`, `index.html`, `package.json`, `package-lock.json`, `vite.config.ts`, `app/*`, `internal/*`, `modules/tabs/*` (milik `session-tabs`), `modules/workspace/*` (milik `workspace-sidebar`), `modules/terminal/*`. Tanpa dependency baru, tanpa unit test, tanpa impor `@xterm/*`.

## Kontrak

Rujuk `overview.md` bagian 1.2, 2.2, 2.3, 2.4, 2.5, 2.7. Ringkasan yang wajib persis:

Disediakan (ekspor dari barrel `lib/stores/index.ts`; signature sama dengan overview 2.3):
- `workspaces.ts`: `workspaces`, `activeWorkspaceId`, `activeProjectId` (writable); `activeWorkspace`, `activeProject` (derived); `loadWorkspaces`, `selectWorkspace`, `selectProject`, `createWorkspace`, `renameWorkspace`, `deleteWorkspace`, `addProject`, `removeProject`.
- `sessions.ts`: `sessions`, `loadSessions`, `createSession(providerId)`, `killSession(id)`.
- `tabs.ts`: `tabs`, `activeTabId`, `openTab`, `activateTab`, `closeTab`.
- `providers.ts`: `providers`, `loadProviders`.
- `init.ts`: `connection`, `initStores(): Promise<() => void>`.
- Tipe di `types.ts` sesuai overview 2.2 (tidak berubah dari foundation).
- Komponen `modules/shell/SidebarArea.svelte` dan `StatusBar.svelte` (tanpa prop).
- Variabel CSS overview 2.5 di `styles/theme.css`.

Dipakai:
- `call`, `onEvent` dari `../ipc` (relatif dari `lib/stores/` adalah `../ipc`), `panels` dari registry, `TabBar`/`TabContent` dari `./modules/tabs/` (stub foundation atau versi `session-tabs`; tanpa prop).
- Method daemon: `ping`, `workspace.list`, `workspace.create`, `workspace.rename`, `workspace.delete`, `workspace.addProject`, `workspace.removeProject`, `session.list`, `session.create`, `session.kill`, `provider.list`. Nama method workspace mengikuti tiket 04; bila daemon menjawab method-not-found, error dilempar ke pemanggil (tidak ditelan). Sebelum menulis, `grep -rn "workspace\.\(list\|create\|rename\|delete\|addProject\|removeProject\)" internal cmd` (hanya baca) untuk mengonfirmasi nama dan bentuk parameter; bila ada perbedaan, ikuti kode daemon dan catat di laporan.
- Parameter yang dikirim: `workspace.create {name}`, `workspace.rename {id, name}`, `workspace.delete {id}`, `workspace.addProject {workspaceId, path, name?}`, `workspace.removeProject {workspaceId, projectId}`, `session.list {workspaceId}`, `session.create {workspaceId, projectId, cwd, providerId}`, `session.kill {id}`.
- Event: `session.exit` payload `{id, exitCode}`; `workspace.changed` bila diterbitkan.

## Langkah

### 1. `styles/theme.css`
Definisikan variabel pada `:root` (terang) lalu override pada `@media (prefers-color-scheme: dark) { :root { ... } }`. Wajib: `--bg`, `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--danger`, `--ok`, `--term-bg`, `--term-fg`, `--term-cursor`, `--sidebar-w: 260px`, `--tabbar-h: 34px`, `--statusbar-h: 24px`. Contoh nilai: terang `--bg:#ffffff`, `--bg-elev:#f5f5f7`, `--fg:#1b1b1f`, `--term-bg:#ffffff`, `--term-fg:#1b1b1f`, `--term-cursor:#1b1b1f`; gelap `--bg:#1b1b1f`, `--bg-elev:#24242a`, `--fg:#e6e6ea`, `--term-bg:#1e1e1e`, `--term-fg:#d4d4d4`, `--term-cursor:#d4d4d4`. Tambahkan `color-scheme: light dark` pada `:root`. `--term-*` harus ada di blok terang DAN blok gelap.

### 2. `styles/base.css`
Reset ringan: `*{box-sizing:border-box}`, `html,body,#app{height:100%;margin:0}`, `body{font-family:system-ui,sans-serif;font-size:13px;background:var(--bg);color:var(--fg);overflow:hidden}`, gaya dasar `button`, `input`, scrollbar tipis. Tanpa framework CSS.

### 3. `lib/stores/workspaces.ts` (ganti stub)
- Helper internal (boleh diekspor tambahan bila perlu): `normalizeList<T>(res: unknown, key: string): T[]` yang menerima array langsung, objek berkunci `key`, atau `null`/lainnya menjadi `[]` (overview 1.2 poin 2).
- `loadWorkspaces()`: `call('workspace.list')` -> `normalizeList<Workspace>(res,'workspaces')` -> `workspaces.set`. Lalu: bila `activeWorkspaceId` null atau tidak ada di daftar, pilih workspace pertama (atau `null` bila kosong) dan set `activeProjectId` ke project pertamanya; bila workspace aktif masih ada, pertahankan, dan pastikan `activeProjectId` masih valid (kalau hilang, pakai project pertama atau `null`). Fungsi ini TIDAK memanggil `loadSessions` kecuali workspace aktif BERGANTI (diubah oleh fungsi ini); dalam kasus itu panggil `loadSessions()` setelahnya.
- `selectWorkspace(id)`: set `activeWorkspaceId`, `activeProjectId` = project pertama workspace itu (atau `null`), `await loadSessions()`. Boleh tidak menyentuh `tabs`.
- `selectProject(id)`: HANYA `activeProjectId.set(id)` (final; tidak menyentuh `sessions`).
- `createWorkspace(name)`: `call<Workspace>('workspace.create',{name})`; tambahkan ke `workspaces` (normalisasi bila balasan `{workspace}`); kembalikan Workspace. Tidak otomatis memilih.
- `renameWorkspace(id,name)`: `call('workspace.rename',{id,name})`; perbarui nama di store lokal.
- `deleteWorkspace(id)`: `call('workspace.delete',{id})`; hapus dari store; bila yang dihapus aktif, pilih workspace pertama sisa (atau `null`) via logika `selectWorkspace`/set `sessions=[]` bila `null`.
- `addProject(path,name?)`: butuh workspace aktif (bila tidak ada, `throw new Error('pilih workspace dulu')`); `call('workspace.addProject',{workspaceId,path,name})`; balasan `{workspace, project}`: ganti workspace itu di store dengan `workspace` bila ada, kalau tidak sisipkan `project` ke `projects`; bila `activeProjectId` null set ke project baru; kembalikan Project.
- `removeProject(id)`: `call('workspace.removeProject',{workspaceId,projectId:id})`; keluarkan dari `projects` workspace aktif; bila project aktif dihapus, pindah ke project pertama sisa atau `null`. TIDAK menyentuh `sessions`.
- `activeWorkspace = derived([workspaces, activeWorkspaceId], ...)`, `activeProject = derived([activeWorkspace, activeProjectId], ...)`.
- Semua aksi melempar `Error` dengan pesan daemon (jangan try/catch menelan). Hapus komentar `STUB(foundation)`; tandai `// FINAL(shell-layout)` bila perlu.

### 4. `lib/stores/sessions.ts` (ganti stub)
- `loadSessions()`: ambil `get(activeWorkspaceId)`; bila null, `sessions.set([])` dan kembali. Else `call('session.list',{workspaceId})` -> `normalizeList<Session>(res,'sessions')` -> filter klien `s.workspaceId === workspaceId` -> `sessions.set` (urutan dari daemon dipertahankan).
- `createSession(providerId)`: ambil `get(activeWorkspaceId)` dan `get(activeProject)`; bila salah satu null, `throw new Error('Tambah project dulu')`. `call<Session>('session.create',{workspaceId, projectId: project.id, cwd: project.path, providerId})`; tambahkan ke akhir `sessions` (hindari duplikat per `id`); kembalikan Session. TIDAK memanggil `openTab`.
- `killSession(id)`: `call('session.kill',{id})`; set `status='exited'` pada sesi itu di store (pertahankan di daftar; event `session.exit` bisa mengisi `exitCode`). TIDAK menutup tab.
- Pastikan tidak ada `$effect`/subscribe yang memicu `loadSessions` saat `activeProjectId` berubah. Jangan impor `activeProjectId` ke dalam logika pemuatan.
- Hindari impor melingkar: `sessions.ts` mengimpor dari `workspaces.ts` (`activeWorkspaceId`, `activeProject`) dan `workspaces.ts` mengimpor `loadSessions` dari `sessions.ts`. ES module menangani siklus asalkan hanya dipakai di dalam badan fungsi (bukan saat evaluasi modul); jangan memakai nilai impor pada top-level. Bila ragu, definisikan `loadSessions` dipanggil lewat impor dinamis tidak perlu; cukup ikuti aturan top-level ini dan verifikasi dengan build.

### 5. `lib/stores/providers.ts` (ganti stub)
`loadProviders()`: `call('provider.list')` -> `normalizeList<Provider>(res,'providers')` -> `providers.set`. Kegagalan tidak boleh membuat `initStores` gagal total (ditangani di `init.ts`).

### 6. `lib/stores/tabs.ts`
Biarkan bila foundation menandainya `FINAL`; verifikasi perilakunya: id `` `${type}:${refId}` ``, `openTab` dedup lalu `activeTabId.set`, judul `t.title ?? refId`, `closeTab` mengaktifkan tetangga kanan lalu kiri atau `null` (hanya bila tab yang ditutup adalah yang aktif). Perbaiki bila menyimpang.

### 7. `lib/stores/init.ts` (ganti stub)
- `connection = writable<{state:'connecting'|'ok'|'error'; message:string}>({state:'connecting',message:''})`.
- `initStores(): Promise<() => void>`:
  1. `connection.set({state:'connecting',message:''})`.
  2. `try { await call('ping') } catch (e) { connection.set({state:'error', message: e instanceof Error ? e.message : String(e)}); return () => {}; }` (tidak melempar).
  3. `await loadWorkspaces()` (ini juga memuat sesi bila workspace aktif terpilih); lalu `await loadProviders()` dan `await loadSessions()` (idempotent). Kegagalan `loadProviders`/`workspace.list` karena method belum ada di daemon: tangkap, isi store kosong, tetap `connection.set({state:'ok',...})` (hanya `ping` yang menentukan error koneksi); error lain dari ping-lanjutan tidak menampilkan layar error.
  4. Pasang SEKALI: `const off1 = onEvent('session.exit', (_t, p) => { ... })` yang membaca `{id, exitCode}` dan `sessions.update` set `status='exited'`, `exitCode`; `const off2 = onEvent('workspace.changed', () => { void loadWorkspaces(); })`. Payload dilindungi dengan pengecekan tipe (`typeof p === 'object' && p`).
  5. `connection.set({state:'ok',message:''})` dan `return () => { off1(); off2(); }`.
- Panggilan ulang `initStores` (tombol "Coba lagi") harus aman: pemanggil (App) memanggil dispose lama dulu. Tidak ada `setInterval`/`setTimeout` berulang di berkas ini.

### 8. `lib/stores/index.ts`
`export type { Project, WorkspaceConfig, Workspace, SessionStatus, Session, Provider, TabType, Tab } from './types';` dan `export * from './workspaces'; export * from './sessions'; export * from './tabs'; export * from './providers'; export * from './init';`. Bila helper `normalizeList` diekspor dari `workspaces.ts` dan dipakai file lain, pindahkan ke berkas internal tambahan `lib/stores/util.ts` (masih di ownedPaths) dan JANGAN reekspor dari barrel.

### 9. `modules/shell/SidebarArea.svelte`
Tanpa prop. `import { panels } from '../../lib/registry'; const sidebar = panels('sidebar');`. Markup: `<aside class="sidebar">{#each $sidebar as panel (panel.id)}<section class="panel"><panel.component /></section>{/each}</aside>`. Lebar `var(--sidebar-w)`, `overflow:auto`, `background:var(--bg-elev)`, `border-right:1px solid var(--border)`. Tanpa panel: elemen kosong tanpa error.

### 10. `modules/shell/StatusBar.svelte`
Tanpa prop. Sama dengan di atas untuk `panels('statusbar')`, tinggi `var(--statusbar-h)`, `display:flex; align-items:center; gap`, `border-top`. Kosong bila tanpa panel.

### 11. `App.svelte` (ganti total)
- Impor: `import './styles/theme.css'; import './styles/base.css';`, `onMount, onDestroy` dari svelte, `connection, initStores` dari `./lib/stores`, `TabBar from './modules/tabs/TabBar.svelte'`, `TabContent from './modules/tabs/TabContent.svelte'`, `SidebarArea from './modules/shell/SidebarArea.svelte'`, `StatusBar from './modules/shell/StatusBar.svelte'`.
- Logika: `let dispose: (() => void) | null = null;` `async function start() { dispose?.(); dispose = await initStores(); }`; `onMount(() => { void start(); })`; `onDestroy(() => dispose?.())` (atau kembalikan cleanup dari `onMount` yang memanggil dispose; pastikan dispose dipanggil saat destroy).
- Markup: bila `$connection.state === 'error'`: layar tengah dengan judul singkat ("Tidak bisa terhubung ke daemon"), pesan `$connection.message`, dan `<button onclick={start}>Coba lagi</button>` (teks persis "Coba lagi"). Bila `'connecting'`: teks "Menghubungkan ke daemon...". Bila `'ok'`: grid `<div class="shell">` dengan kolom `var(--sidebar-w) 1fr`, baris `1fr var(--statusbar-h)`: `<SidebarArea />` (kolom 1, rentang kedua baris), `<main class="main"><TabBar /><div class="content"><TabContent /></div></main>`, `<StatusBar />` (baris bawah, kolom 2 atau melintang; pilih satu dan konsisten). Pastikan `.content` punya `min-height:0; flex:1; overflow:hidden` agar terminal bisa mengisi tinggi.
- Tidak ada teks "Hello World", tidak ada `main-tab` panel render (area utama = TabBar+TabContent), tanpa `setInterval`.
- Gaya lokal memakai variabel tema; `:global` body ditangani `base.css`.

### 12. Verifikasi
- `cd frontend && npm run build` hijau; `npm run check` tanpa error di file milik fitur ini.
- `grep -rn "setInterval" frontend/src/App.svelte frontend/src/lib/stores frontend/src/modules/shell` kosong; `grep -rn "STUB(foundation): diganti oleh shell-layout" frontend/src/lib/stores` kosong; `grep -rn "@xterm" frontend/src/modules/shell` kosong.
- Telaah manual: `selectProject` hanya `activeProjectId.set`; tidak ada kode yang memanggil `loadSessions`/`sessions.set` sebagai reaksi pada `activeProjectId`.

## Perubahan

Diubah (isi diganti):
- `frontend/src/App.svelte`
- `frontend/src/lib/stores/workspaces.ts`, `sessions.ts`, `providers.ts`, `init.ts`, `index.ts` (dan `tabs.ts`/`types.ts` hanya bila perlu koreksi)

Dibuat:
- `frontend/src/styles/theme.css`
- `frontend/src/styles/base.css`
- `frontend/src/modules/shell/SidebarArea.svelte`
- `frontend/src/modules/shell/StatusBar.svelte`
- `frontend/src/lib/stores/util.ts` (opsional, hanya bila helper dibagi antar-file)
- `frontend/src/lib/stores/*` penuh bila foundation ternyata belum ada

## Wiring

Yang diekspos untuk langkah wiring (jangan menyambungkan sendiri):
- Barrel `lib/stores/index.ts` dengan seluruh ekspor kontrak, dipakai `workspace-sidebar` dan `session-tabs`.
- `App.svelte` mengimpor `./modules/tabs/TabBar.svelte` dan `./modules/tabs/TabContent.svelte` (diganti implementasinya oleh `session-tabs`), serta `SidebarArea`/`StatusBar`. Panel `workspace-sidebar` muncul otomatis lewat `registerPanel` dari modul `workspace` (glob di `main.ts`).
- `styles/theme.css` mendefinisikan variabel yang dipakai fitur lain (dengan fallback di sisi mereka). `--term-*` dibaca `TerminalTab`.
- Agen wiring memeriksa: tidak ada `STUB`, tidak ada `setInterval`, `--term-*` ada di terang dan gelap, `main.ts`/`registry.ts`/`ipc.ts` tak berubah.

## Selesai bila

- `cd frontend && npm run build` sukses (guard utama), idealnya `npm run check` tanpa error pada path fitur ini; build hijau walaupun `modules/workspace` belum ada dan `modules/tabs` masih stub.
- `App.svelte` menampilkan tiga area, tanpa teks Hello World; panel sidebar/statusbar dirender dari registry berurut `order`; tanpa panel, area kosong tanpa error.
- Kegagalan `ping` menampilkan layar error dengan tombol "Coba lagi" yang menjalankan ulang `initStores` (dispose lama dulu).
- Semua ekspor store sesuai overview 2.3 ada dan berbadan nyata; `createSession` mengirim `cwd` = path project aktif; `selectProject` tidak menyentuh `sessions`; `session.exit` dan `workspace.changed` ditangani lewat `onEvent` yang dipasang sekali; tanpa `setInterval`.
- `--term-bg`, `--term-fg`, `--term-cursor` terdefinisi untuk terang dan gelap; tema mengikuti `prefers-color-scheme`.
- Tidak ada file di luar ownedPaths yang berubah (`registry.ts`, `ipc.ts`, `main.ts`, `package.json` utuh).
