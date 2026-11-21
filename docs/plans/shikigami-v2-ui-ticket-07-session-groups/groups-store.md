# Plan: Store grup di frontend lewat daemon

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-07-session-groups.md` (epic "Grup sesi sebagai tab")
- Fitur: `groups-store` (Store grup di frontend lewat daemon)
- Rujukan wajib baca: `docs/plans/shikigami-v2-ui-ticket-07-session-groups/overview.md` (bagian 2.2 kontrak IPC, 2.3 kontrak store, 7 aturan paralel); jangan diubah.
- ownedPaths:
  - `frontend/src/lib/stores/groups.ts` (menimpa penuh stub Foundation)
  - `frontend/src/lib/stores/types.ts` (perluasan aditif)
  - `frontend/src/modules/persist/autosave.ts` (perluasan aditif)
- Depends on foundation: **ya**. Foundation sudah membuat stub `frontend/src/lib/stores/groups.ts` (komentar `// STUB(foundation)`) dan registri slot. Plan ini mengganti isi stub itu seluruhnya dengan bentuk ekspor yang SAMA (nama dan tanda tangan), karena `group-tabs-ui` dikompilasi terhadap stub tersebut secara paralel. Bila berkas stub belum ada di worktree, buat berkasnya dengan isi akhir dari plan ini.
- Tanpa unit test, tanpa dependency baru.

## Tujuan

Menyediakan state dan aksi grup di frontend dengan daemon sebagai satu-satunya sumber kebenaran: semua state grup hanyalah turunan dari store `workspaces` (data `Workspace.groups/activeGroupId/membership` hasil method dan event daemon) dan store `sessions`. Aksi `createGroup/renameGroup/deleteGroup/reorderGroup/setActiveGroup/moveSession` memanggil method `workspace.group.*`; pergantian grup aktif menukar layout area tengah dengan aman lewat `flushNow` dan `switchGroup`. Tidak ada `localStorage`, `sessionStorage`, sidecar, cermin state, polling, atau timer baru.

## Path yang dimiliki

HANYA ini yang boleh dibuat atau diubah:
- `frontend/src/lib/stores/groups.ts`
- `frontend/src/lib/stores/types.ts` (hanya tambah, tidak mengubah tipe yang ada)
- `frontend/src/modules/persist/autosave.ts` (hanya tambah dua ekspor, tidak mengubah perilaku yang ada)

Jangan menyentuh: `frontend/src/modules/groups/` (milik `group-tabs-ui`), `internal/` (Go, milik `group-daemon`), `lib/stores/index.ts` (jangan tambahkan `groups.ts` ke sana), `lib/stores/layout.ts|sessions.ts|workspaces.ts|persist.ts|init.ts`, `modules/persist/restore.ts|index.ts`, `modules/layout/*`, `modules/shell/*` (termasuk `helpers.ts`), `modules/sidebar-left/*`, `main.ts`, `App.svelte`, `app/`, `cmd/`, dan berkas wiring.

## Kontrak

Rujukan: overview bagian 2.2 (IPC daemon) dan 2.3 (store). Ringkasan konkret yang diimplementasikan di sini:

Disediakan (ekspor `groups.ts`, tanda tangan identik dengan stub Foundation):
```ts
export interface SessionGroup { id: string; name: string; order: number; main: boolean }
export const MAIN_GROUP_NAME = 'Utama';
export const groups: Readable<SessionGroup[]>;
export const mainGroupId: Readable<string>;
export const activeGroupId: Readable<string>;
export const activeGroup: Readable<SessionGroup | undefined>;
export const sessionGroupMap: Readable<Record<string, string>>;
export const groupSessionCounts: Readable<Record<string, number>>;
export const groupsError: Readable<string | null>;
export function initGroups(): () => void;
export function createGroup(name?: string): Promise<string>;
export function renameGroup(id: string, name: string): Promise<void>;
export function deleteGroup(id: string): Promise<void>;
export function reorderGroup(id: string, toIndex: number): Promise<void>;
export function setActiveGroup(id: string): Promise<void>;
export function moveSession(sessionId: string, toGroupId: string): Promise<void>;
export function groupOfSession(sessionId: string): string | undefined;
```
Keputusan penanganan galat (karena stub Foundation juga begitu dan UI ditulis terhadapnya): semua aksi TIDAK melempar. Galat ditangkap, pesannya masuk `groupsError`, state lokal tidak berubah; `createGroup` mengembalikan `''` bila gagal. `groupsError` dikosongkan (`null`) pada awal aksi yang berhasil sampai akhir.

Tipe baru di `lib/stores/types.ts`:
```ts
export interface WorkspaceGroup { id: string; name: string; order: number; main?: boolean; layout?: unknown | null }
// Workspace ditambah field opsional:
//   groups?: WorkspaceGroup[]; activeGroupId?: string; membership?: Record<string, string>;
```

Ekspor baru `modules/persist/autosave.ts`:
```ts
export function getCurrentGroupId(): string;
export function switchGroup(groupId: string, doc: unknown): void;
```

Dipakai (sudah ada, jangan diubah):
- `call`, `onEvent` dari `../ipc`.
- `workspaces`, `activeWorkspace`, `activeWorkspaceId` dari `./workspaces`; `sessions` dari `./sessions`; `layout` dari `./layout`; `flushNow` dari `./persist`; `providers` dari `./providers` (periksa nama ekspor aktual di berkas itu; dipakai untuk judul pane).
- Model murni dari `../../modules/layout/model`: `hydrateDoc`, `leaves`, `findLeafBy`, `closePane` (alias `closePaneDoc`), tipe `LayoutDoc`.
- `switchGroup`, `getCurrentGroupId` dari `../../modules/persist/autosave` (impor langsung berkas ini, JANGAN dari `modules/persist/index.ts` karena punya efek samping).
- Method daemon (overview 2.2): `workspace.group.create|rename|delete|reorder|setActive|moveSession` dengan hasil `GroupsState` (`create` menambah `groupId`); `workspace.layout.get {workspaceId, groupId}` -> `{workspaceId, groupId, layout}`; `workspace.layout.set {workspaceId, groupId, layout}`; event `workspace.groupsChanged` payload `{kind, ...GroupsState}`.
- Sesuai aturan paralel: bila `group-daemon` belum ter-merge, kode tetap dikompilasi; tidak perlu respons tiruan di kode akhir.

## Langkah

### 1. `frontend/src/lib/stores/types.ts` (aditif)

Tambahkan `export interface WorkspaceGroup {...}` (di bawah `WorkspaceConfig`) dan ubah baris `Workspace` menjadi menambah tiga field opsional `groups?`, `activeGroupId?`, `membership?` (field lama tidak berubah). Jangan menyentuh tipe lain.

### 2. `frontend/src/modules/persist/autosave.ts` (aditif)

Setelah `getCurrentWsId`, tambahkan:
```ts
export function getCurrentGroupId(): string {
  return currentGroupId;
}

// Ganti layout aktif ke grup lain. Pemanggil sudah menjalankan flushNow() sebelumnya.
// Sinkron: hydrate lalu baseline, sehingga autosave berikutnya menulis ke grup baru dan
// tidak menganggap hasil hydrate sebagai perubahan. Timer retry sengaja tidak dibersihkan:
// doWrite menangkap groupId per tulis, jadi tulis ulang grup lama tetap menuju grup lama.
export function switchGroup(groupId: string, doc: unknown): void {
  layout.hydrate(doc);
  setBaseline(currentWsId, groupId);
}
```
Tidak mengubah fungsi lain. `layout` sudah diimpor di berkas itu.

### 3. `frontend/src/lib/stores/groups.ts` (tulis ulang penuh; baca stub dulu dengan Read sebelum Write)

Hapus komentar `STUB(foundation)`. Bagian-bagian berurutan:

**3.1 Impor dan konstanta.** `svelte/store` (`writable`, `derived`, `get`, `type Readable`), `../ipc`, `./workspaces`, `./sessions`, `./layout`, `./persist`, `./providers`, `./types` (tipe `Workspace`, `WorkspaceGroup`, `Session`), model layout, `../../modules/persist/autosave`. `export const MAIN_GROUP_NAME = 'Utama'`. `export interface SessionGroup {id; name; order; main}`.

**3.2 Tipe `GroupsState`** (internal, tidak diekspor): `{ workspaceId: string; activeGroupId: string; groups: Array<{id; name; order; main?: boolean}>; membership: Record<string,string> }`.

**3.3 Store turunan (semua `derived`, tanpa state tulis kecuali `groupsError`).**
- `groups`: dari `activeWorkspace`; salin `ws.groups ?? []`, urut `order` naik (stabil), petakan ke `SessionGroup` dengan `main: !!g.main`. Tanpa workspace/tanpa groups -> `[]` (tidak melempar).
- `mainGroupId`: id grup `main`; bila tak ada pakai `groups[0]?.id`; bila kosong `''`.
- `activeGroupId`: dari `[activeWorkspace, groups, mainGroupId]`: `ws.activeGroupId` bila ada di `groups`, selain itu `mainGroupId`.
- `activeGroup`: grup di `groups` dengan id `activeGroupId`, atau `undefined`.
- `sessionGroupMap`: dari `[activeWorkspace, sessions, groups, mainGroupId]`; untuk tiap sesi dengan `workspaceId === ws.id`: `membership[s.id]` bila id itu ada di `groups`, selain itu `mainGroupId`. `groups` kosong -> `{}`.
- `groupSessionCounts`: dari `[groups, sessionGroupMap]`; inisialisasi 0 untuk tiap grup, tambah per entri peta.
- `groupsError`: `writable<string | null>(null)` internal (`errStore`), diekspor sebagai `Readable` (`{ subscribe }`).

**3.4 `groupOfSession(sessionId)`**: `get(sessionGroupMap)[sessionId]` (`undefined` bila sesi tak dikenal).

**3.5 `applyGroupsState(state: GroupsState)`** (internal, idempoten). Bila `state` tidak berbentuk benar (`workspaceId` string, `groups` array) atau workspace tidak ada di `workspaces`, abaikan. Selain itu `workspaces.update`: untuk entri ber-id sama ganti `groups` dengan `state.groups` dipetakan ke `WorkspaceGroup`, mempertahankan `layout` dari grup lama dengan id sama (`old.layout` bila ada); ganti `activeGroupId` dan `membership` (`state.membership ?? {}`). Entri workspace lain tidak disentuh (pertahankan referensi objek). Fungsi ini tidak menyentuh store `layout` dan tidak memanggil `switchGroup` (event/hasil hanya memperbarui data).

**3.6 Pembantu `run`**: `async function run<T>(label: string, fn: () => Promise<T>, fallback: T): Promise<T>` yang membungkus try/catch: sukses -> `errStore.set(null)` dan kembalikan hasil; galat -> `errStore.set(message)` dengan `message = e instanceof Error && e.message ? e.message : \`Gagal ${label}\`` (label Indonesia, mis. `'membuat grup'`), `console.warn`, kembalikan `fallback`. Pembantu `wsId()` = `get(activeWorkspaceId)`; aksi tanpa workspace aktif mengisi `groupsError` dengan `'Pilih workspace dulu'` dan berhenti.

**3.7 Pembantu `groupCall<T>(method, extra)`**: `const workspaceId = wsId(); const res = await call<GroupsState & T>(method, { workspaceId, ...extra }); applyGroupsState(res); return res;` (workspaceId hasil harus sama dengan yang dikirim; bila `res.workspaceId` kosong, isi dengan `workspaceId` sebelum `applyGroupsState`).

**3.8 Aksi sederhana** (masing-masing dibungkus `run`):
- `renameGroup(id, name)`: `groupCall('workspace.group.rename', {groupId: id, name})`. Nama final = yang dikembalikan daemon (mungkin berakhiran " 2"); tidak ada normalisasi di frontend (validasi nama milik daemon).
- `reorderGroup(id, toIndex)`: `workspace.group.reorder {groupId, toIndex}`.
- `createGroup(name?)`: `workspace.group.create {name}` (hanya sertakan `name` bila ada) -> ambil `groupId` dari hasil; lalu `await setActiveGroupInner(groupId)` (lihat 3.9). Kembalikan `groupId`; `''` bila gagal. Layout area tengah menjadi kosong karena grup baru berlayout `null`.
- `deleteGroup(id)`: bila `id === get(activeGroupId)` dan bukan grup `main`, `await setActiveGroupInner(get(mainGroupId))` dulu (gagal -> berhenti, galat sudah tercatat). Lalu `workspace.group.delete {groupId: id}`. Untuk grup `main` langsung kirim ke daemon; `ErrConflict` muncul lewat `groupsError` dan state tidak berubah.

**3.9 `setActiveGroup(id)`.** Pisahkan `setActiveGroupInner(id): Promise<boolean>` (melempar ke `run` pemanggil) dan ekspor `setActiveGroup` = `run`-nya. Serialisasi dengan rantai promise modul `let switchChain: Promise<unknown> = Promise.resolve()` (antrikan: `switchChain = switchChain.then(() => doSwitch(id)).catch(() => {})`, dan pemanggil menunggu rantai itu) agar dua pergantian tidak bertumpuk. Isi `doSwitch(id)`:
1. `const workspaceId = wsId()`; tanpa workspace -> berhenti. Bila `id === get(activeGroupId)` -> selesai (no-op). Bila `id` tidak ada di `get(groups)` -> isi `groupsError` `'Grup tidak ditemukan'`, berhenti.
2. `const prev = get(activeGroupId)`.
3. `await flushNow()` (layout grup lama ditulis ke grup lama oleh autosave lewat `currentGroupId` lama).
4. `await groupCall('workspace.group.setActive', {groupId: id})`.
5. `let res; try { res = await call('workspace.layout.get', {workspaceId, groupId: id}); } catch (e) { /* pulihkan: */ try { await groupCall('workspace.group.setActive', {groupId: prev}); } catch {} ; throw e; }`.
6. Bila `get(activeWorkspaceId) !== workspaceId` (workspace berganti selama await) -> jangan `switchGroup`; selesai.
7. `switchGroup(id, res.layout)` (sinkron; `res.layout` boleh `null`).
Tidak memakai `localStorage`; tidak memanggil `restoreWorkspace`.

**3.10 `moveSession(sessionId, toGroupId)`** (dibungkus `run`):
1. `workspaceId`; `from = groupOfSession(sessionId) ?? get(mainGroupId)`. Bila `toGroupId` tidak ada di `groups` -> `groupsError` `'Grup tujuan tidak ditemukan'`, berhenti. Bila `from === toGroupId` -> selesai tanpa panggilan.
2. Tutup pane sesi di grup asal:
   - Bila `from === get(activeGroupId)`: ulangi `findLeafBy(get(layout), l => l.kind === 'terminal' && l.sessionId === sessionId)` dan `layout.closePane(leaf.id)` sampai tidak ada (aman bila ada lebih dari satu).
   - Selain itu: `res = await call('workspace.layout.get', {workspaceId, groupId: from})`; `let doc = hydrateDoc(res.layout)`; selama ada leaf terminal berisi `sessionId` (`leaves(doc)`), `doc = closePaneDoc(doc, leaf.id)`; bila ada yang ditutup, `await call('workspace.layout.set', {workspaceId, groupId: from, layout: doc})`.
   Pane file tidak dipindah.
3. `await groupCall('workspace.group.moveSession', {sessionId, toGroupId})`.
4. Bila `toGroupId === get(activeGroupId)` (hasil sudah diterapkan, jadi baca lagi setelah langkah 3) dan belum ada pane terminal untuk sesi itu di `get(layout)`: ambil `Session` dari `get(sessions)`; bila ada, `layout.openPane({kind: 'terminal', sessionId, projectId: s.projectId, provider: s.providerId, title})` dengan `title = get(providers).find(p => p.id === s.providerId)?.name ?? s.providerId`. Sesi yang tidak ada di store `sessions` -> lewati pembukaan pane.
Sesi tidak pernah dibunuh (tidak memanggil `killSession`).

**3.11 Penugasan sesi baru (di dalam `initGroups`).** Pendengar `sessions` tanpa polling:
- State modul (di-reset di cleanup): `known = new Set<string>()`, `snapshotWs: string | null`, `pending = true`, `assigning = new Set<string>()`.
- Berlangganan `activeWorkspaceId`: bila id berubah (termasuk nilai awal), `known = new Set()`, `snapshotWs = id`, `pending = true`.
- Berlangganan `sessions` (`list`): `const ws = get(activeWorkspaceId)`; bila `!ws` berhenti. `mine = list.filter(s => s.workspaceId === ws)`. Bila `list` memuat sesi workspace lain (`list.length !== mine.length`) -> abaikan emisi ini sepenuhnya (balasan/penambahan basi dari workspace sebelumnya; `pending` tidak dikonsumsi). Bila `pending`: `mine` seluruhnya masuk `known`, `pending = false`, selesai (ini snapshot awal hasil `loadSessions`, dianggap sudah punya keanggotaan: entri atau default `main`). Selain itu, untuk tiap `s` di `mine` yang `!known.has(s.id)`: tambahkan ke `known`; bila `get(sessionGroupMap)` belum punya entri `membership` eksplisit untuk `s.id` (periksa `get(activeWorkspace)?.membership?.[s.id]` kosong) dan `gid = get(activeGroupId)` bukan `get(mainGroupId)` dan `gid` tidak kosong dan `!assigning.has(s.id)`: `assigning.add(s.id)`, lalu `void run('memindahkan sesi', () => groupCall('workspace.group.moveSession', {sessionId: s.id, toGroupId: gid}), null).finally(() => assigning.delete(s.id))`. Hanya memanggil daemon; TIDAK menyentuh layout dan tidak memanggil `moveSession()` publik (yang mengubah layout). Bila grup aktif `main`, tidak ada panggilan.
- Saat `initGroups()` dipanggil, jika `activeWorkspaceId` sudah ada dan `sessions` sudah berisi sesi workspace itu, snapshot awal langsung terisi lewat pemanggilan pertama callback `subscribe` (karena `subscribe` Svelte memanggil seketika): emisi pertama dengan `pending = true` menjadi snapshot. Catat (asumsi, lihat bawah): bila emisi pertama kosong padahal baru kemudian `loadSessions` mengisi, emisi berikutnya dianggap snapshot.

**3.12 `initGroups(): () => void`** (idempoten, hanya dipasang sekali): variabel modul `let teardown: (() => void) | null = null`. Bila `teardown` sudah ada, kembalikan `teardown` yang sama tanpa memasang ulang. Selain itu pasang: `const off = onEvent('workspace.groupsChanged', (_topic, payload) => applyGroupsState(payload as GroupsState))` (hanya data; `kind` diabaikan; layout dipertahankan oleh 3.5), langganan 3.11, lalu `teardown = () => { off(); unsubA(); unsubB(); reset state sesi baru; teardown = null; }` dan kembalikan `teardown`. Jangan memanggil `loadWorkspaces`/`loadSessions` dan jangan menghidrasi layout dari event atau perubahan workspace aktif (hidrasi saat ganti workspace tetap milik `restoreWorkspace` tiket 06).

**3.13 Pengaman akhir berkas.** Tidak ada referensi `localStorage`/`sessionStorage`/`setInterval`/`setTimeout` di `groups.ts`; tidak ada `alert`; tidak ada warna/UI.

## Perubahan

- UBAH `frontend/src/lib/stores/groups.ts`: stub Foundation diganti implementasi penuh (langkah 3).
- UBAH `frontend/src/lib/stores/types.ts`: tambah `WorkspaceGroup` dan tiga field opsional di `Workspace` (langkah 1).
- UBAH `frontend/src/modules/persist/autosave.ts`: tambah ekspor `getCurrentGroupId` dan `switchGroup` (langkah 2).
- Tidak ada berkas lain dibuat atau diubah.

## Wiring

Fitur ini TIDAK menyambungkan apa pun sendiri. Yang diekspos untuk langkah wiring dan fitur lain (overview bagian 5):
- `modules/groups/index.ts` (milik `group-tabs-ui`) memanggil `initGroups()` dari `lib/stores/groups.ts`; fitur ini hanya menjamin `initGroups()` idempoten dan mengembalikan cleanup.
- `autosave.ts` kini mengekspor `getCurrentGroupId` dan `switchGroup`; wiring langkah 2 hanya memverifikasi keberadaannya (`restore.ts` dan `modules/persist/index.ts` tidak diubah).
- Penugasan sesi baru ke grup aktif berjalan otomatis lewat pendengar `sessions` begitu `initGroups()` dipanggil; wiring langkah 7 hanya memverifikasi bahwa sesi dari `+ sesi`, split, dan klik file ikut masuk grup aktif (catat sebagai temuan bila tidak, jangan ubah jalur pembuatan sesi).
- `groups.ts` tidak ditambahkan ke `lib/stores/index.ts`; konsumen mengimpor langsung dari `lib/stores/groups`.

## Selesai bila

- [ ] `cd frontend && npm run build` sukses (dan `npm run check` bila skrip itu ada) tanpa galat tipe di `groups.ts`, `types.ts`, `autosave.ts`.
- [ ] `groups.ts` mengekspor persis semua simbol kontrak (nama dan tanda tangan sama dengan stub Foundation) sehingga `modules/groups/` tetap dikompilasi tanpa perubahan.
- [ ] `grep -nE "localStorage|sessionStorage|setInterval|setTimeout|alert\(" frontend/src/lib/stores/groups.ts` tidak menghasilkan apa pun.
- [ ] `setActiveGroup` mengikuti urutan `flushNow` -> `workspace.group.setActive` -> `workspace.layout.get {groupId}` -> `switchGroup`, no-op bila sudah aktif, dan diserialisasi.
- [ ] `createGroup` memanggil `workspace.group.create` lalu mengaktifkan grup baru; `deleteGroup` pada grup aktif lebih dulu mengaktifkan `main`; `moveSession` menutup pane grup asal (aktif lewat `layout.closePane`, non-aktif lewat `layout.get` + model murni + `layout.set`) sebelum `workspace.group.moveSession`.
- [ ] Event `workspace.groupsChanged` hanya memperbarui data workspace (layout lama dipertahankan) dan tidak mengganti layout.
- [ ] Galat daemon masuk `groupsError` tanpa mengubah state lokal dan tanpa melempar.
- [ ] Diff hanya menyentuh tiga path di atas; tidak ada dependency baru.

### Asumsi yang dicatat

- Aksi tidak melempar (galat lewat `groupsError`, `createGroup` mengembalikan `''`), konsisten dengan stub Foundation.
- Sinyal snapshot sesi: karena store `sessions` hanya mengemisi saat `set`/`update`, emisi pertama setelah workspace aktif berganti dianggap hasil `loadSessions`. Bila workspace baru kebetulan tidak punya sesi dan sesi pertamanya dibuat sebelum emisi snapshot, sesi itu tidak ditugaskan ke grup aktif (jatuh ke `main`); kegagalan ini lebih ringan dibanding memindahkan seluruh sesi lama secara keliru.
- Sesi baru yang muncul lewat `loadSessions` ulang di luar pergantian workspace diperlakukan sebagai sesi baru dan ditugaskan ke grup aktif non-`main` bila belum punya entri keanggotaan.
- Judul pane saat `moveSession` ke grup aktif memakai nama provider (tanpa mengimpor `modules/shell/helpers` demi menghindari ketergantungan lib -> modules).
