# Plan: Bagian UI aman saat tidak ada workspace aktif

## Metadata
- Tiket: `welcome-screen-ticket-01-session-workspaces` (`docs/tickets/welcome-screen-ticket-01-session-workspaces.md`)
- Fitur: `null-workspace-safety`
- ownedPaths: lihat bagian "Path yang dimiliki"
- Depends on foundation: tidak (tidak ada `00-foundation.md`; state null diuji lewat `activeWorkspaceId.set(null)`)
- Konteks bersama: `overview.md` bagian 2.3 (invarian), 2.4 (konvensi), 6 (build guard), 7 (aturan paralel). Jangan mengulang isinya.

## Tujuan
Setelah epic ini aplikasi start dengan `activeWorkspaceId = null`, `activeProjectId = null`, `sessions = []`. Pastikan setiap konsumen di ownedPaths tidak melempar exception dan tidak mencetak error konsol pada keadaan itu, dan aksi yang butuh workspace (sesi baru, tambah project, buka folder/file, Lark/Grimoire) dinonaktifkan atau disembunyikan. Hanya guard defensif null (optional chaining, early return, `disabled`, `{#if}`). Perilaku dan tampilan saat workspace ada TIDAK boleh berubah.

Hasil audit awal (grep `activeWorkspace|activeWorkspaceId|activeProject`, sudah diverifikasi saat plan ditulis). Agent implementasi tetap menjalankan ulang grep di Langkah 1 karena file lain bisa berubah.

Sudah aman, cukup diverifikasi tanpa edit (jangan diubah bila memang aman):
- `lib/stores/sessions.ts` `loadSessions()`: sudah `if (!workspaceId) { sessions.set([]); return; }`. `createSession` melempar `Error('Tambah project dulu')` bila null; itu perilaku terdokumentasi dan pemanggil menangkapnya, jangan diubah.
- `modules/lark/source.ts`: `resolveSource` sudah `if (!workspaceId) return 'error'` sebelum RPC; `watchSource` aman.
- `modules/grimoire/store.ts`: subscriber `activeProjectId` sudah `if (id) void refresh(id)`; `state.set(empty(id))` untuk null. `LawsSlot`/`StaleSlot` sudah mengguard `projectId` null.
- `modules/topbar/CenterSummary.svelte` (`{#if ws}`), `SessionCounter.svelte` (`sessionsOfActive(..., null)` mengembalikan `[]`), `modules/sidebar-left/SessionGroups.svelte` + `groups.ts` (`?? []`, filter `workspaceId === null`), `WorkspaceHeader.svelte` (`?.name ?? 'Tanpa workspace'`, `?.projects.length ?? 0`), `modules/workspace/ProjectList.svelte` (tombol Tambah sudah `disabled={!$activeWorkspace}`), `SessionList.svelte` (`?.projects.find`), `NewSession.svelte` (sudah `disabled` tanpa `$activeProject`), `WorkspacePicker.svelte` (Ganti nama/Hapus sudah `disabled` tanpa id), `modules/files/tree/FilesPanel.svelte` dan `modules/sidebar-right/FilesTab.svelte` (`hasWorkspace`, pesan "Tidak ada workspace aktif."), `modules/files/shared/shellBridge.ts` (`derived` -> `null`), `modules/layout/PaneHeader.svelte` (`?.projects?.find`).

Perlu diperbaiki (celah nyata):
1. `lib/stores/groups.ts`: fungsi publik grup (`setActiveGroup`, `createGroup`, `renameGroup`, `reorderGroup`, `deleteGroup`, `moveSession`) lewat `run()` -> `wsId()` melempar `'Pilih workspace dulu'`; `run` menangkapnya lalu `errStore.set(...)` dan `console.warn(...)`. Tanpa workspace itu menghasilkan pesan galat dan log konsol palsu.
2. `modules/sidebar-left/NewSessionMenu.svelte`: tombol utama "+ New Session" aktif tanpa workspace dan popup menampilkan "Open Folder…" (yang akan menambah project ke workspace yang tidak ada).
3. `modules/sidebar-right/FilesTab.svelte`: sudah menyembunyikan aksi tanpa workspace, tetapi `onOpenFolder`/`onAddPath` tidak punya guard defensif sendiri (tombol hanya dirender di cabang `hasWorkspace`; cukup tambah early return).
4. `modules/grimoire/GrimoirePanel.svelte`: tanpa project menampilkan "Pilih project"; tanpa workspace pesannya menyesatkan. Tambah pesan khusus.
5. `modules/layout/shortcuts.ts`: `splitPaneWithNewSession` tanpa workspace jatuh ke `createSession` yang melempar lalu `console.error`. Tambah early return bila tidak ada workspace.
6. `modules/workspace/WorkspacePicker.svelte`: `submit` mode `create` memanggil `selectWorkspace` saja (tidak masalah), tidak perlu diubah; hanya dicatat.

Catatan penting: `restoreWorkspace` yang disebut tiket di "groups.ts" sebenarnya ada di `frontend/src/modules/persist/restore.ts` (BUKAN ownedPath, jangan diedit). Isinya sudah aman untuk `null` (`layout.hydrate(null)` + `setBaseline(null, '')`, return tanpa RPC). Cukup dicatat di laporan. `loadSessions()` tanpa workspace sudah menghasilkan `sessions = []` (lihat di atas).

## Path yang dimiliki
HANYA path ini yang boleh disentuh. Jangan sentuh file wiring (`App.svelte`, `modules/welcome`) atau path fitur lain:
- `frontend/src/modules/sidebar-left`
- `frontend/src/modules/sidebar-right`
- `frontend/src/modules/workspace`
- `frontend/src/modules/files`
- `frontend/src/modules/lark/source.ts`
- `frontend/src/modules/grimoire`
- `frontend/src/modules/topbar/CenterSummary.svelte`
- `frontend/src/modules/topbar/SessionCounter.svelte`
- `frontend/src/modules/layout`
- `frontend/src/lib/stores/groups.ts`
- `frontend/src/lib/stores/sessions.ts`

Milik fitur paralel (JANGAN disentuh): `lib/stores/workspaces.ts`, `types.ts`, `index.ts`, `init.ts`, `modules/persist/*` (termasuk `restore.ts`, `autosave.ts`), `modules/topbar/WorkspaceTabs.svelte`, `modules/openfolder/logic.ts`, seluruh `internal/workspace/*`, `cmd/shikigamictl/*`. Juga di luar ownedPaths: `modules/lark/*` selain `source.ts`, `modules/mention`, `modules/palette`, `modules/runs`, `modules/shell`, `modules/thread`, `modules/topbar/util.ts`.

## Kontrak
Disediakan (invarian untuk tiket 02 dan fitur lain; lihat overview 2.3):
- Dengan `activeWorkspaceId === null` semua modul di ownedPaths tidak melempar dan tidak mencetak `console.error`/`console.warn`.
- Fungsi grup di `groups.ts` (`setActiveGroup`, `createGroup`, `renameGroup`, `reorderGroup`, `deleteGroup`, `moveSession`) menjadi no-op yang mengembalikan nilai fallback-nya (`false`, `''`, `null`/`undefined`) tanpa mengisi `groupsError` saat tidak ada workspace aktif.
- `loadSessions(): Promise<void>` tanpa workspace -> `sessions.set([])`, tanpa RPC (sudah, dipertahankan).
- Tidak ada ekspor baru, tidak ada perubahan signature publik.

Dipakai (sudah ada, tidak diubah): `activeWorkspace`, `activeWorkspaceId`, `activeProject`, `activeProjectId` dari `lib/stores/workspaces.ts`; `get` dari `svelte/store`. Dengan fitur `open-set-store` nilai-nilai itu bisa `null` saat start; fitur ini tidak bergantung pada kode barunya (uji lewat `activeWorkspaceId.set(null)`).

## Langkah
Kerjakan berurutan. Setelah tiap file selesai, jangan ubah apa pun selain guard yang disebut. Gaya: ikuti file sekitar; komentar bahasa Indonesia; pertahankan penanda `// FINAL(...)`.

1. Audit ulang. Dari `frontend/`, jalankan `grep -rnE "activeWorkspace|activeWorkspaceId|activeProject" src/modules/sidebar-left src/modules/sidebar-right src/modules/workspace src/modules/files src/modules/lark/source.ts src/modules/grimoire src/modules/topbar/CenterSummary.svelte src/modules/topbar/SessionCounter.svelte src/modules/layout src/lib/stores/groups.ts src/lib/stores/sessions.ts` dan cocokkan dengan daftar "sudah aman" di Tujuan. Bila ada konsumen baru yang tidak guard null, tambahkan guard serupa. Catat konsumen di luar ownedPaths (lihat Langkah 8).

2. `frontend/src/lib/stores/groups.ts`, fungsi `run<T>(label, fn, fallback)`: tambahkan early return di awal fungsi, sebelum `try`:
   ```ts
   // Tanpa workspace aktif tidak ada grup untuk dioperasikan: no-op, bukan galat.
   if (!get(activeWorkspaceId)) return fallback;
   ```
   `get` dan `activeWorkspaceId` sudah diimpor di file ini. Jangan ubah `wsId()` (tetap melempar untuk jalur internal lain) dan jangan ubah `errStore`. `initGroups()` tidak perlu diubah (subscriber sudah `if (!ws) return`; `applyGroupsState` sudah menolak workspace tak dikenal). Pastikan hanya fungsi yang memang membutuhkan workspace yang memakai `run` (semua pemakainya saat ini begitu).

3. `frontend/src/lib/stores/sessions.ts`: verifikasi saja; JANGAN ubah `loadSessions` yang sudah benar. Satu-satunya perubahan yang boleh: bila `call('session.list')` dipanggil dari jalur lain tanpa guard, tambahkan guard serupa (saat plan ditulis tidak ada). Bila tidak ada yang perlu diubah, biarkan file tak tersentuh.

4. `frontend/src/modules/sidebar-left/NewSessionMenu.svelte`:
   - Tambah `import { activeWorkspaceId }` ke impor `../../lib/stores` yang sudah ada (gabungkan dengan `activeWorkspace, activeProjectId, providers`).
   - Tambah `const noWorkspace = $derived(!$activeWorkspace);`.
   - Di `start()` tambahkan `if (busy || noWorkspace) return;` (gantikan `if (busy) return;`).
   - Tombol utama: `disabled={noWorkspace}` dan `title={noWorkspace ? 'Buka atau buat workspace dulu' : undefined}`; CSS `.main:disabled` sudah ada.
   - Popup: bungkus ulang kondisi menjadi `{#if open && !noWorkspace}`, dan di `$effect` pembuka popup tambahkan `if (noWorkspace) { open = false; return; }`-setara (cukup `if (!open) return;` tetap, ditambah efek terpisah `$effect(() => { if (noWorkspace) open = false; })`). Dengan workspace ada, markup dan perilaku identik.

5. `frontend/src/modules/sidebar-right/FilesTab.svelte`: di awal `onOpenFolder()` tambahkan `if (!hasWorkspace) return;` dan di awal `onAddPath` setelah `e.preventDefault()` tambahkan `if (!hasWorkspace) return;`. Tidak ada perubahan markup (cabang `{#if !hasWorkspace}` sudah menampilkan "Tidak ada workspace aktif."). Jangan ubah `FilesPanel.svelte` kecuali audit menemukan celah (saat plan ditulis aman).

6. `frontend/src/modules/grimoire/GrimoirePanel.svelte`: tambah impor `activeWorkspaceId` dari `'../../lib/stores'` (impor yang sudah ada: `activeProject`). Di rantai `{#if}` ubah cabang pesan menjadi:
   ```svelte
   {:else if !$activeWorkspaceId}
     <div class="msg">Tidak ada workspace aktif</div>
   {:else if !$activeProject || !s.projectId}
     <div class="msg">Pilih project</div>
   ```
   Letakkan cabang baru tepat sebelum cabang `!$activeProject || !s.projectId`, setelah cabang editor laws (`$lawsViewOpen && ...`). Tombol "Bangun ulang" sudah tersembunyi tanpa `s.projectId`; biarkan. Jangan ubah `store.ts`, `LawsSlot`, `StaleSlot`.

7. `frontend/src/modules/layout/shortcuts.ts`: di `splitPaneWithNewSession`, setelah baris `if (!src || src.kind !== 'terminal') return;` tambahkan `if (!get(activeWorkspace)) return;`. `get` dan `activeWorkspace` sudah diimpor. Jangan ubah cabang `session.create`/`createSession` lainnya. `PaneHeader.svelte`, `LayoutArea.svelte` dan sisanya cukup diverifikasi aman.

8. Verifikasi manual kode (tanpa mengedit) dan catat temuan di luar ownedPaths untuk laporan. Kandidat yang perlu dicatat, bukan diedit:
   - `modules/persist/restore.ts` `restoreWorkspace(null)`: aman; lokasi sebenarnya bukan di `groups.ts`.
   - `modules/shell/helpers.ts` `createAndOpenSession`: bila dipanggil tanpa workspace melempar `'Tambah project dulu'` (ditangkap pemanggil; UI sudah menonaktifkan pemicunya setelah Langkah 4).
   - `modules/lark/*` selain `source.ts`, `modules/mention`, `modules/palette`, `modules/runs`, `modules/thread`, `modules/topbar/util.ts`, `modules/persist/DeadSession.svelte`: jalankan grep yang sama pada folder-folder ini dan catat setiap konsumen yang tidak guard null, tanpa mengeditnya.

9. Guard build: dari `frontend/` jalankan `npm run build` (wajib lulus). Opsional `npm run check` (tidak boleh menambah error baru dibanding sebelum perubahan).

10. Verifikasi perilaku bila memungkinkan (tanpa unit test): jalankan app/`npm run dev`, set `activeWorkspaceId` ke `null` (mis. start kosong dari fitur `open-set-store` atau lewat konsol/devtool), buka sidebar kiri, kanan (Files, Grimoire), lalu amati konsol: tidak ada error/warn. Lalu pilih workspace dan pastikan tampilan/perilaku sama seperti sebelumnya.

## Perubahan
Diubah (hanya guard):
- `frontend/src/lib/stores/groups.ts` (early return di `run`)
- `frontend/src/modules/sidebar-left/NewSessionMenu.svelte` (disabled + guard tanpa workspace)
- `frontend/src/modules/sidebar-right/FilesTab.svelte` (early return di `onOpenFolder`/`onAddPath`)
- `frontend/src/modules/grimoire/GrimoirePanel.svelte` (pesan "Tidak ada workspace aktif")
- `frontend/src/modules/layout/shortcuts.ts` (early return tanpa workspace)

Diverifikasi, diubah hanya bila audit Langkah 1 menemukan celah baru: `lib/stores/sessions.ts`, `modules/lark/source.ts`, `modules/topbar/CenterSummary.svelte`, `modules/topbar/SessionCounter.svelte`, `modules/sidebar-left/*` lain, `modules/workspace/*`, `modules/files/*`, `modules/layout/*` lain, `modules/grimoire/*` lain.

Dibuat: tidak ada. Tidak ada dependency baru, unit test, atau file di luar ownedPaths.

## Wiring
Tidak ada yang perlu disambungkan: fitur ini hanya guard internal tanpa ekspor baru atau registrasi. Yang diekspos bagi fitur lain: jaminan invarian overview 2.3 untuk modul di ownedPaths (konsumen `activeWorkspace*`/`activeProject` aman null; fungsi grup no-op tanpa workspace). Jangan menyentuh `App.svelte`, `modules/welcome`, `persist/index.ts`, atau `WorkspaceTabs.svelte`. Laporan fitur wajib memuat daftar temuan konsumen null yang tidak aman di luar ownedPaths (Langkah 8) untuk tiket 02/epic lain.

## Selesai bila
- [ ] `npm run build` di `frontend/` sukses (guard utama; tanpa unit test).
- [ ] Tanpa workspace aktif: `run()` di `groups.ts` mengembalikan fallback tanpa mengisi `groupsError` dan tanpa `console.warn`; `loadSessions()` menghasilkan `sessions = []` tanpa RPC.
- [ ] Tombol "+ New Session" nonaktif dan popup tidak terbuka tanpa workspace; Files tab menampilkan "Tidak ada workspace aktif."; Grimoire menampilkan "Tidak ada workspace aktif"; Cmd+D split tanpa workspace diam (tanpa `console.error`); "Tambah project" tetap nonaktif.
- [ ] Dengan workspace aktif, markup, perilaku, dan tampilan identik dengan sebelumnya (diff hanya berisi guard yang disebut di Langkah 2 sampai 7).
- [ ] Tidak ada file di luar ownedPaths yang berubah (`git status` hanya menunjukkan file di Perubahan).
- [ ] Laporan memuat: catatan bahwa `restoreWorkspace` ada di `modules/persist/restore.ts` dan sudah aman null, serta temuan konsumen null di luar ownedPaths.
