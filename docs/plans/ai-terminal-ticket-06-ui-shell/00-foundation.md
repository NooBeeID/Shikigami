# Foundation: Epic 06 UI Shell

Baca dulu `overview.md` (bagian 2 dan 4) dan tiket `docs/tickets/ai-terminal-ticket-06-ui-shell.md`. Dikerjakan SATU agent sebelum fitur paralel. Tujuan: semua ekspor store dan komponen lintas fitur sudah ada dengan nama final (badan minimal), sehingga `workspace-sidebar`, `session-tabs`, dan `shell-layout` bisa dikerjakan bersamaan dan `npm run build` tetap hijau.

Batasan: hanya membuat file di `frontend/src/lib/stores/` dan `frontend/src/modules/tabs/{TabBar,TabContent}.svelte`. Jangan ubah `package.json`, `registry.ts`, `ipc.ts`, `main.ts`, `App.svelte`, atau file lain. Tanpa dependency baru, tanpa unit test.

## Langkah

### 1. Verifikasi prasyarat
- Pastikan ada: `frontend/src/lib/registry.ts`, `frontend/src/lib/ipc.ts`, `frontend/src/main.ts`, `frontend/src/modules/terminal/{index.ts,TerminalTab.svelte,terminal-client.ts}`, dan `@xterm/xterm` + `@xterm/addon-fit` di `frontend/package.json`. Bila hilang, hentikan dan laporkan sebagai blocker.
- Baca `lib/ipc.ts` untuk memastikan nama ekspor (`call`, `onEvent`, `inWails`). Bila berbeda dari overview, catat di laporan akhir.

### 2. `lib/stores/types.ts` (final)
Tulis tipe persis seperti overview 2.2: `Project`, `WorkspaceConfig`, `Workspace`, `SessionStatus`, `Session`, `Provider`, `TabType`, `Tab`. Gunakan `export interface`/`export type` (proyek memakai `verbatimModuleSyntax`: impor tipe dengan `import type`).

### 3. Store dengan ekspor final
Buat file berikut, semua ekspor bagian 2.3 overview dengan signature persis. Badan boleh minimal tetapi tidak boleh `throw` saat modul di-import:
- `workspaces.ts`: `workspaces`, `activeWorkspaceId`, `activeProjectId` (`writable`), `activeWorkspace`, `activeProject` (`derived`), dan fungsi `loadWorkspaces`, `selectWorkspace`, `selectProject`, `createWorkspace`, `renameWorkspace`, `deleteWorkspace`, `addProject`, `removeProject`. Badan stub: fungsi async `Promise.reject(new Error('belum diimplementasi'))` atau no-op yang memuaskan tipe; `selectProject` sudah final (hanya `activeProjectId.set(id)`).
- `sessions.ts`: `sessions` (`writable<Session[]>([])`), `loadSessions`, `createSession`, `killSession` (stub seperti di atas).
- `tabs.ts`: `tabs`, `activeTabId`, `openTab`, `activateTab`, `closeTab`. Boleh langsung final (logika murni tanpa IPC): id `` `${type}:${refId}` ``, dedup, aktifkan, `closeTab` mengaktifkan tab tetangga (kanan, lalu kiri) atau `null`.
- `providers.ts`: `providers`, `loadProviders` (stub).
- `init.ts`: `connection` (`writable({state:'connecting', message:''})`), `initStores` (stub: mengembalikan `async () => () => {}`).
- `index.ts`: `export * from './types'` (hanya tipe: gunakan `export type * from './types'` atau `export type { ... }`), dan `export *` dari kelima file lain.

Tandai tiap badan yang akan diganti dengan komentar `// STUB(foundation): diganti oleh shell-layout` (kecuali `selectProject` dan `tabs.ts` bila sudah final; tandai `// FINAL(foundation)` pada yang final agar `shell-layout` tahu boleh dibiarkan).

### 4. Stub komponen tab
- `frontend/src/modules/tabs/TabBar.svelte` dan `TabContent.svelte`: komponen Svelte 5 tanpa prop, markup minimal (mis. `<div class="tabbar"></div>` dan `<div class="tabcontent"></div>`), komentar `<!-- STUB(foundation): diganti oleh session-tabs -->`. Tidak mengimpor `@xterm/*`.

### 5. Verifikasi
- `cd frontend && npm run build` hijau; `npm run check` tanpa error pada file baru.
- `git status`/diff: hanya file di `lib/stores/` dan dua file di `modules/tabs/` yang baru; `package.json` dan `package-lock.json` tidak berubah.

## Keluaran yang diharapkan
- `frontend/src/lib/stores/{types,workspaces,sessions,tabs,providers,init,index}.ts` dengan ekspor final.
- `frontend/src/modules/tabs/{TabBar,TabContent}.svelte` stub.
- Build frontend hijau. Laporkan nama ekspor nyata `lib/ipc.ts` bila berbeda dari overview.
