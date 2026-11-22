# Plan: Store openWorkspaceIds, openWorkspace, closeWorkspaceTab

## Metadata
- Tiket: `welcome-screen-ticket-01-session-workspaces` (`docs/tickets/welcome-screen-ticket-01-session-workspaces.md`)
- Fitur: `open-set-store`
- ownedPaths: `frontend/src/lib/stores/workspaces.ts`, `frontend/src/lib/stores/types.ts`, `frontend/src/lib/stores/index.ts`, `frontend/src/lib/stores/init.ts`, `frontend/src/modules/persist/index.ts`
- Depends on foundation: tidak (tidak ada `00-foundation.md`). Berjalan paralel dengan fitur lain gelombang A; RPC `workspace.touch` dipanggil lewat `call` string sehingga build tidak butuh backend siap.

## Tujuan
Aplikasi mulai tanpa workspace aktif dan tanpa tab. Frontend memegang set workspace "terbuka" per sesi (`openWorkspaceIds`, hanya memori) dengan fungsi `openWorkspace` dan `closeWorkspaceTab`. `loadWorkspaces`, `deleteWorkspace`, handler event `workspace.changed`, dan `persist/index.ts` disesuaikan agar tidak lagi memilih workspace otomatis. Kontrak lengkap ada di overview bagian 2.2 dan 2.3; plan ini hanya merinci langkah.

## Path yang dimiliki
HANYA file ini yang boleh disentuh:
- `frontend/src/lib/stores/workspaces.ts`
- `frontend/src/lib/stores/types.ts`
- `frontend/src/lib/stores/index.ts`
- `frontend/src/lib/stores/init.ts`
- `frontend/src/modules/persist/index.ts`

Jangan sentuh `autosave.ts`, `restore.ts`, `groups.ts`, `sessions.ts`, `WorkspaceTabs.svelte`, `openfolder/logic.ts`, `App.svelte`, `modules/welcome`, file Go, maupun `overview.md`. Jangan membuat file baru, menambah dependency, atau menulis unit test. Jangan commit.

## Kontrak
Disediakan (dipakai fitur `tabs-close-menu` dan tiket 02, rujuk overview 2.2):
- `Workspace.lastOpenedAt?: number` (types.ts).
- `export const openWorkspaceIds: Writable<string[]>` (memori, urutan = urutan tab).
- `export async function openWorkspace(id: string): Promise<void>`
- `export async function closeWorkspaceTab(id: string): Promise<void>`
- `deleteWorkspace(id)` dan `loadWorkspaces()` dengan perilaku baru.
- RPC dipakai: `workspace.touch` params `{ id: string }` -> `Workspace` penuh (atau `{ workspace }`), best-effort.

Dipakai (sudah ada): `call` dari `../ipc`, `selectWorkspace`, `replaceWorkspace` (privat di workspaces.ts), `loadSessions`, `sessions`, `normalizeList`, `restoreWorkspace`/`applyUi` dari `./restore`.

Invarian: event `workspace.changed` tidak boleh mengaktifkan workspace atau mengubah `openWorkspaceIds` (kecuali membuang id yang sudah tidak ada di daftar).

## Langkah

### 1. types.ts
Tambah `lastOpenedAt?: number;` pada interface `Workspace` (baris field bersama `id/name/projects/config`). Jangan ubah tipe lain.

### 2. workspaces.ts
Impor tetap; tambahkan store dan helper di bawah deklarasi `activeProject`.

a. Store:
```ts
export const openWorkspaceIds: Writable<string[]> = writable<string[]>([]);
```
Komentar singkat bahasa Indonesia: memori saja, tidak dipersist, urutan = urutan tab.

b. Helper privat:
```ts
// Tetangga kanan, kalau tidak ada tetangga kiri, dari urutan `order` (id sudah ada di dalamnya).
function neighborOf(order: string[], id: string): string | null {
  const i = order.indexOf(id);
  if (i < 0) return null;
  return order[i + 1] ?? order[i - 1] ?? null;
}

function clearActive(): void {
  activeWorkspaceId.set(null);
  activeProjectId.set(null);
  sessions.set([]);
}
```

c. `loadWorkspaces()` (pertahankan penanda `// FINAL(shell-layout)`):
- `workspaces.set(list)`.
- Buang id hilang: `openWorkspaceIds.update((ids) => ids.filter((x) => list.some((w) => w.id === x)))` (hanya bila ada yang berubah boleh, tapi filter murni tidak apa-apa; jangan sampai menambah id).
- `curId = get(activeWorkspaceId)`, `cur = list.find(...)`.
- Bila `!cur`: panggil `clearActive()` lalu `return`. JANGAN pilih `list[0]` dan JANGAN memanggil `loadSessions()` (sessions kosong lewat `sessions.set([])`).
- Bila `cur` ada: perilaku lama (validasi `activeProjectId`, set ke `firstProjectId(cur)` bila tidak valid). Tidak mengubah `activeWorkspaceId`, jadi event `updated` dari touch aman.

d. `openWorkspace(id)`:
```ts
export async function openWorkspace(id: string): Promise<void> {
  openWorkspaceIds.update((ids) => (ids.includes(id) ? ids : [...ids, id]));
  await selectWorkspace(id);
  try {
    const res = await call<Workspace | { workspace: Workspace }>('workspace.touch', { id });
    const ws = (res && 'workspace' in res ? res.workspace : res) as Workspace | undefined;
    if (ws && ws.id) replaceWorkspace(ws);
  } catch {
    // touch best-effort: gagal tidak membatalkan pembukaan
  }
}
```
Catatan: `replaceWorkspace` mengganti entri penuh, sehingga `lastOpenedAt` ikut terisi; entri hasil touch sudah membawa projects/groups terbaru dari backend. `selectWorkspace` tidak dibungkus try (error select memang harus naik ke pemanggil seperti sebelumnya).

e. `closeWorkspaceTab(id)`:
```ts
export async function closeWorkspaceTab(id: string): Promise<void> {
  const order = get(openWorkspaceIds);
  const next = neighborOf(order, id);
  openWorkspaceIds.set(order.filter((x) => x !== id));
  if (get(activeWorkspaceId) !== id) return;
  if (next) await selectWorkspace(next);
  else clearActive();
}
```
Tidak memanggil `workspace.delete`. Tab non-aktif: aktif tidak berubah.

f. `deleteWorkspace(id)`: setelah `await call('workspace.delete', { id })`:
```ts
const order = get(openWorkspaceIds);
const wasActive = get(activeWorkspaceId) === id;
const rest = get(workspaces).filter((w) => w.id !== id);
// tetangga dihitung dari urutan tab terbuka sebelum id dikeluarkan, hanya id yang masih ada
const candidates = order.filter((x) => x === id || rest.some((w) => w.id === x));
const next = neighborOf(candidates, id);
workspaces.set(rest);
openWorkspaceIds.set(order.filter((x) => x !== id));
if (wasActive) {
  if (next) await selectWorkspace(next);
  else clearActive();
}
```
Jangan lagi memakai `rest[0]`. Jika id yang dihapus tidak terbuka, `neighborOf` mengembalikan null, dan itu hanya relevan bila ia aktif (secara praktik aktif selalu terbuka); aman: bila aktif tetapi tidak di `order`, jatuh ke `clearActive()`.

g. Fungsi lain (`selectWorkspace`, `createWorkspace`, `renameWorkspace`, `addProject`, `removeProject`, `selectProject`) tidak diubah. `createWorkspace` TIDAK menambah ke `openWorkspaceIds` (pembukaan dilakukan pemanggil lewat `openWorkspace`).

### 3. index.ts
`export * from './workspaces'` sudah mengekspor `openWorkspaceIds`, `openWorkspace`, `closeWorkspaceTab`. Tidak perlu perubahan fungsional. Pastikan tidak ada bentrok nama ekspor (grep `openWorkspace` di `frontend/src/lib/stores/` dan cek `npm run build`). Hanya edit file ini bila build menunjukkan ekspor perlu eksplisit.

### 4. init.ts
Handler `workspace.changed` tetap `void loadWorkspaces().catch(() => {})`. Karena `loadWorkspaces` kini tidak memilih workspace, tidak perlu logika tambahan; tambahkan komentar satu baris bahasa Indonesia bahwa event `updated` (termasuk dari `workspace.touch`) tidak mengaktifkan workspace maupun mengubah `openWorkspaceIds`. Panggilan awal di `initStores` (`loadWorkspaces()` lalu `loadSessions()`) dibiarkan: `loadSessions()` dengan `activeWorkspaceId` null harus aman, tetapi itu tanggung jawab fitur `null-workspace-safety` (sessions.ts); sudah dibungkus try/catch sehingga build dan start tidak pecah.

### 5. persist/index.ts `run()`
- Hapus komputasi `list`, `wanted`, dan `target` lama. Ganti dengan:
```ts
const target: string | null = null; // start selalu kosong: tanpa workspace aktif
```
- Pertahankan `ui = await call('workspace.ui.get')` (try/catch) dan `applyUi(ui)` agar lebar/visibilitas sidebar dipulihkan.
- Pertahankan `if (get(workspaces).length === 0) await loadWorkspaces()` (agar daftar siap), tetap dalam try/catch.
- Hapus blok `if (target && target !== get(activeWorkspaceId)) { selectWorkspace ... }` seluruhnya.
- `await restoreWorkspace(target)` tetap (kini `restoreWorkspace(null)`), dengan blok catch lama (`layout.hydrate(null)`, `setBaseline(target, '')`, error status, `startWatching()`, `switching = false`, return) tidak diubah.
- Sisa alur tidak berubah: `enableWrites(true)`, `setPersistStatus('ready')`, `startWatching()`, subscriber `activeWorkspaceId`, `switching = false`.
- Bersihkan impor yang tak terpakai: buang `selectWorkspace` dari impor `../../lib/stores/workspaces` (tetap `activeWorkspaceId, loadWorkspaces, workspaces`). `switchTo()` tidak diubah. `autosave.ts` dan `restore.ts` TIDAK diubah.

Catatan perilaku: `ui.activeWorkspaceId` dari `workspace.ui.get` kini diabaikan saat start; `uiPayload()` di autosave menulis `''` untuk null sehingga tidak menimpa data workspace.

## Perubahan
- Diubah: `frontend/src/lib/stores/types.ts` (field `lastOpenedAt?`)
- Diubah: `frontend/src/lib/stores/workspaces.ts` (store `openWorkspaceIds`, `openWorkspace`, `closeWorkspaceTab`, helper `neighborOf`/`clearActive`, `loadWorkspaces`, `deleteWorkspace`)
- Diubah (komentar saja / bila perlu): `frontend/src/lib/stores/init.ts`, `frontend/src/lib/stores/index.ts`
- Diubah: `frontend/src/modules/persist/index.ts` (target selalu null, buang `selectWorkspace`)
- Tidak ada file baru.

## Wiring
Fitur ini hanya mengekspos; JANGAN menyambungkannya sendiri:
- `openWorkspaceIds`, `openWorkspace`, `closeWorkspaceTab` (via `lib/stores`) untuk `WorkspaceTabs.svelte` dan `newWorkspaceFromFolder` yang disambungkan fitur `tabs-close-menu`.
- Pemakai tiket 02 (`Welcome`) memakai `openWorkspace` dan `lastOpenedAt`.
- Jangan mengubah `WorkspaceTabs.svelte`, `openfolder/logic.ts`, atau `App.svelte`. Sampai wiring selesai, tab lama di topbar dapat tampak kosong/tidak sinkron; itu diharapkan.

## Selesai bila
- `npm run build` di `frontend/` sukses (opsional `npm run check` tanpa error baru dari path yang dimiliki).
- Setelah start dengan daftar workspace tidak kosong: `activeWorkspaceId` dan `activeProjectId` null, `sessions` kosong, `openWorkspaceIds` kosong; `persist/index.ts` tidak memanggil `selectWorkspace`.
- `openWorkspace` idempotent pada `openWorkspaceIds`, memanggil `workspace.touch`, dan kegagalan touch tidak menimbulkan error tak tertangani.
- `closeWorkspaceTab` tidak memanggil `workspace.delete`; tutup tab aktif memilih kanan lalu kiri, tab terakhir menghasilkan null; tab non-aktif tidak mengubah aktif.
- `deleteWorkspace` mengeluarkan id dan memilih tetangga dari workspace terbuka atau null, tanpa `rest[0]`.
- `workspace.changed` tidak mengaktifkan workspace dan tidak menambah `openWorkspaceIds`.
- Tidak ada file di luar ownedPaths yang berubah (cek `git status`).
