# Plan: WorkspaceTabs hanya tab terbuka dan menu Tutup

## Metadata

- Tiket: `welcome-screen-ticket-01-session-workspaces` (`docs/tickets/welcome-screen-ticket-01-session-workspaces.md`)
- Fitur: `tabs-close-menu` (fitur penutup epic, gelombang B; sekaligus wiring epic)
- ownedPaths:
  - `frontend/src/modules/topbar/WorkspaceTabs.svelte`
  - `frontend/src/modules/openfolder/logic.ts`
- Depends on foundation: tidak (epic tanpa foundation). Bergantung pada fitur `open-set-store` (wajib sudah merge: menyediakan `openWorkspaceIds`, `openWorkspace`, `closeWorkspaceTab`) dan idealnya `workspace-touch-go` (RPC `workspace.touch`; touch best-effort sehingga tidak diperlukan untuk build).
- Rujukan kontrak: `docs/plans/welcome-screen-ticket-01-session-workspaces/overview.md` bagian 2.2 dan 2.4. Jangan mengubah overview.

## Tujuan

Topbar hanya menampilkan tab untuk workspace yang dibuka di sesi aplikasi (`openWorkspaceIds`), tanpa tab sama sekali saat start (hanya tombol "+"). Menu konteks tab mendapat aksi "Tutup" (sembunyikan tab, data tetap, tanpa konfirmasi) di samping "Ganti nama" dan "Hapus". Alur "+" dan "folder baru jadi workspace" membuka workspace lewat `openWorkspace` sehingga tab muncul, aktif, dan `lastOpenedAt` tercatat.

## Path yang dimiliki

HANYA dua file ini yang boleh disentuh:
- `frontend/src/modules/topbar/WorkspaceTabs.svelte`
- `frontend/src/modules/openfolder/logic.ts`

Jangan menyentuh `frontend/src/lib/stores/*` (milik `open-set-store`), `modules/persist/*`, `App.svelte`, `modules/welcome`, file Go, atau file wiring/fitur lain. Bila fungsi store yang dibutuhkan belum ada, hentikan dan laporkan; jangan membuatnya sendiri.

## Kontrak

Dipakai (disediakan `open-set-store`, diimpor dari `'../../lib/stores'`; bentuk lengkap di overview 2.2):
- `openWorkspaceIds: Writable<string[]>` (memori, urutan = urutan tab).
- `openWorkspace(id: string): Promise<void>`: menambah ke `openWorkspaceIds` (idempotent), `await selectWorkspace(id)`, lalu `workspace.touch` best-effort. Setelah promise selesai, `activeWorkspaceId` sudah bernilai `id`.
- `closeWorkspaceTab(id: string): Promise<void>`: keluarkan dari set; bila aktif pilih tetangga atau null; tidak memanggil `workspace.delete`.
- `deleteWorkspace(id)`: sudah ikut mengeluarkan id dari `openWorkspaceIds` dan memilih tetangga terbuka (ditangani store, bukan komponen).

Dipakai (sudah ada): `workspaces`, `activeWorkspaceId`, `selectWorkspace`, `createWorkspace`, `renameWorkspace`, `addProject`, `activeWorkspace` dari `lib/stores`; `defaultWorkspaceName`, `errText` dari `./util`; `basename` dan `pickDirectory` di `logic.ts`.

Disediakan (tidak ada API baru): perilaku `newWorkspaceFromFolder(path: string): Promise<void>` tetap bersignature sama, hanya internalnya memakai `openWorkspace`. Dipakai oleh tiket 02.

## Langkah

### 1. `WorkspaceTabs.svelte` (bagian `<script>`)

1. Ubah impor dari `'../../lib/stores'` menjadi:
   `workspaces, openWorkspaceIds, activeWorkspaceId, selectWorkspace, openWorkspace, closeWorkspaceTab, createWorkspace, renameWorkspace, deleteWorkspace`.
2. Tambah derived untuk tab terbuka, urut mengikuti `$openWorkspaceIds`, melewati id yang tidak ada di `$workspaces`:
   ```ts
   const openTabs = $derived(
     $openWorkspaceIds
       .map((id) => $workspaces.find((w) => w.id === id))
       .filter((w): w is NonNullable<typeof w> => !!w),
   );
   ```
   (Tipe elemen = tipe `Workspace` dari store; boleh impor tipe `Workspace` dari `'../../lib/stores/types'` bila lebih rapi.)
3. `pick(id)`: tidak berubah (`if (id === $activeWorkspaceId) return; try { await selectWorkspace(id); } catch (e) { say(errText(e)); }`).
4. `add()`: ganti `selectWorkspace(ws.id)` dengan `openWorkspace(ws.id)`:
   ```ts
   const ws = await createWorkspace(defaultWorkspaceName($workspaces));
   await openWorkspace(ws.id);
   startEdit(ws.id, ws.name);
   ```
   Pertahankan `busy` guard dan `try/catch/finally` dengan `say(errText(e))`.
5. Tambah handler `menuClose()`:
   ```ts
   async function menuClose() {
     if (!menu) return;
     const id = menu.id;
     menu = null;
     try { await closeWorkspaceTab(id); } catch (e) { say(errText(e)); }
   }
   ```
   Tanpa konfirmasi.
6. `menuDelete` dan `doDelete`: pertahankan alur konfirmasi `confirmId` dan `deleteWorkspace`. Pertahankan guard "Tidak bisa menghapus workspace terakhir" berbasis `$workspaces.length <= 1` apa adanya (perilaku Hapus tetap seperti sekarang; spec tidak meminta mengubahnya). Jangan diubah ke basis `openTabs`.
7. `onContext`: sesuaikan klamp tinggi karena menu kini tiga item (kira-kira 4+3*28+4 px dengan border): ganti `window.innerHeight - 80` menjadi `window.innerHeight - 120`. Klamp lebar `window.innerWidth - 160` tetap.
8. `$effect` scroll-ke-tab-aktif tidak diubah (sudah aman untuk id null via `id ? tabEls[id] : undefined`).

### 2. `WorkspaceTabs.svelte` (markup)

1. Ganti `{#each $workspaces as w (w.id)}` menjadi `{#each openTabs as w (w.id)}`. Isi blok (input edit dan tombol tab) tidak berubah. Bila `openTabs` kosong, `.tabs` kosong dan hanya tombol "+" yang terlihat; tidak perlu elemen placeholder.
2. Menu konteks, urutan item: Ganti nama, Tutup, Hapus (role `menu` dan `menuitem` dipertahankan):
   ```svelte
   <button type="button" role="menuitem" onclick={menuRename}>Ganti nama</button>
   <button type="button" role="menuitem" onclick={menuClose}>Tutup</button>
   <button type="button" role="menuitem" class:dim={$workspaces.length <= 1} aria-disabled={$workspaces.length <= 1} onclick={menuDelete}>Hapus</button>
   ```
3. Bagian `<style>` tidak berubah (kelas `.ctx button` sudah mencakup item baru).

### 3. `openfolder/logic.ts`

1. Ubah impor dari `'../../lib/stores'`: tambah `openWorkspace`; hapus `selectWorkspace` (tidak lagi dipakai). `activeWorkspaceId` tidak lagi dipakai di file ini setelah langkah 2: hapus dari impor. `activeWorkspace`, `addProject`, `createWorkspace` tetap (grep dulu sebelum menghapus impor untuk memastikan memang tak terpakai di tempat lain di file).
2. Ganti isi `newWorkspaceFromFolder`:
   ```ts
   export async function newWorkspaceFromFolder(path: string): Promise<void> {
     const ws = await createWorkspace(basename(path));
     await openWorkspace(ws.id);
     await addProject(path);
   }
   ```
   Hapus baris `await selectWorkspace(ws.id);` dan `if (get(activeWorkspaceId) !== ws.id) activeWorkspaceId.set(ws.id);`. Karena `openWorkspace` menunggu `selectWorkspace`, `addProject` melihat workspace baru sebagai aktif.
3. Fungsi lain (`addFolders`, `handleOpenFolder`, `handleNewWorkspaceFromFolder`, `isDuplicate`, drop handler) tidak diubah. Catatan: `addFolders` memakai cabang `get(activeWorkspace) === null` untuk memanggil `newWorkspaceFromFolder`; ini tetap benar dan kini cocok dengan start tanpa workspace aktif.

### 4. Verifikasi build

- `cd frontend && npm run build` (opsional `npm run check` untuk tipe Svelte).
- `go build ./...` dari root repo (tidak ada perubahan Go di fitur ini, tetap dijalankan sebagai guard epic).

## Perubahan

- Diubah: `frontend/src/modules/topbar/WorkspaceTabs.svelte` (impor, `openTabs`, `add()` memakai `openWorkspace`, `menuClose`, item menu "Tutup", klamp tinggi menu, `{#each openTabs}`).
- Diubah: `frontend/src/modules/openfolder/logic.ts` (impor, `newWorkspaceFromFolder`).
- Tidak ada file dibuat atau dihapus.

## Wiring

Fitur ini sendiri adalah wiring epic; tidak ada yang dibiarkan untuk langkah lain. Yang dieksposnya:
- Perilaku tab dan menu (Ganti nama, Tutup, Hapus) yang kini membaca `openWorkspaceIds`.
- `newWorkspaceFromFolder(path)` yang memakai `openWorkspace`, dipakai ulang oleh tiket 02 (`Welcome.svelte`).
Jangan menyambungkan apa pun ke `App.svelte` atau `modules/welcome` (tiket 02). Setelah merge, daemon perlu dibangun ulang agar `workspace.touch` tersedia; tanpa itu aplikasi tetap berjalan (touch best-effort).

## Selesai bila

- [ ] Start: topbar hanya berisi tombol "+" (tanpa tab) karena `openWorkspaceIds` kosong.
- [ ] Tab hanya untuk id di `$openWorkspaceIds`, sesuai urutan; id yang tidak ada di `$workspaces` dilewati; tab baru muncul di kanan.
- [ ] "+" membuat workspace via `createWorkspace(defaultWorkspaceName(...))`, membukanya lewat `openWorkspace`, dan masuk mode ganti nama.
- [ ] Menu konteks berisi Ganti nama, Tutup, Hapus (role `menu`/`menuitem`); Tutup memanggil `closeWorkspaceTab` tanpa konfirmasi, menutup menu, dan error lewat `say(errText(e))`; Hapus tetap lewat konfirmasi `confirmId` dan `deleteWorkspace`.
- [ ] Menu tidak terpotong di tepi bawah (klamp `window.innerHeight - 120`).
- [ ] `newWorkspaceFromFolder` = `createWorkspace(basename(path))` -> `openWorkspace(ws.id)` -> `addProject(path)`; tidak ada lagi `selectWorkspace`/`activeWorkspaceId.set` di file itu dan tidak ada impor tak terpakai.
- [ ] Hanya dua file ownedPaths yang berubah (`git status` menunjukkan tidak ada file lain dari fitur ini).
- [ ] Guard: `npm run build` di `frontend/` sukses dan `go build ./...` sukses. Tanpa unit test.
