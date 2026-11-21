# Plan: Frontend: autosave dan pemulihan layout

## Metadata

- Tiket: `docs/tickets/shikigami-v2-ui-ticket-06-workspace-persist.md`
- Fitur: `autosave-restore`
- ownedPaths:
  - `frontend/src/modules/persist/` (kecuali `DeadSession.svelte`, `register.ts`, `TerminalGuard.svelte`)
  - `frontend/src/lib/stores/persist.ts`
- Depends on foundation: tidak (tidak ada `00-foundation.md`; semua yang dipakai sudah ada di repo, lihat overview bagian 2.1)
- Rujukan: `docs/plans/shikigami-v2-ui-ticket-06-workspace-persist/overview.md` (bagian 2.3 method IPC, 2.4 kontrak frontend, 7 aturan paralel). Jangan mengubah overview.

## Tujuan

Menyimpan otomatis layout pane, state sidebar kiri/kanan, dan workspace aktif ke daemon (debounce 500 ms, tanpa polling), memicu simpan posisi jendela, dan memulihkan semuanya saat app dibuka (`startPersist`) serta saat berpindah workspace. Layout tiap workspace independen. Dokumen layout null/rusak menghasilkan layout kosong tanpa menimpa data tersimpan sampai pengguna mengubah sesuatu. Fitur ini tidak membuat xterm sendiri: pemulihan terminal sepenuhnya oleh `layout.hydrate` + modul terminal yang sudah ada.

## Path yang dimiliki

HANYA ini yang boleh dibuat/diubah:
- `frontend/src/lib/stores/persist.ts`
- `frontend/src/modules/persist/index.ts`
- `frontend/src/modules/persist/autosave.ts`
- `frontend/src/modules/persist/restore.ts`
- `frontend/src/modules/persist/windowState.ts`

DILARANG: membuat atau mengimpor `DeadSession.svelte`, `register.ts`, `TerminalGuard.svelte` (milik Wiring / restart-dead-session); menyentuh `main.ts`, `App.svelte`, `modules/layout`, `modules/shell`, `modules/terminal`, `lib/stores/layout.ts`, `lib/stores/shell.ts`, `lib/stores/sessions.ts`, `lib/stores/workspaces.ts`, `lib/ipc.ts`, `app/*`, `internal/*`. Tanpa dependency baru, tanpa polling, tanpa unit test.

## Kontrak

Disediakan (bentuk persis di overview 2.4):

```ts
// lib/stores/persist.ts
export type PersistStatus = 'idle' | 'restoring' | 'ready' | 'error';
export const persistStatus: Readable<PersistStatus>;     // awal 'idle'
export const lastSavedAt: Readable<number | null>;       // epoch ms tulis sukses terakhir
export function flushNow(): Promise<void>;               // tidak pernah reject
// internal (dipakai modul persist, bukan kontrak publik):
export function setPersistStatus(s: PersistStatus): void;
export function markSaved(ts?: number): void;            // default Date.now()
export function setFlushHandler(fn: (() => Promise<void>) | null): void;

// modules/persist/index.ts
export function startPersist(): Promise<void>;           // idempoten, tidak pernah reject
```

Dipakai (hanya lewat fungsi publik yang sudah ada):
- `lib/ipc.ts`: `call<T>(method, params)`. Method: `workspace.ui.get`, `workspace.ui.set`, `workspace.layout.get`, `workspace.layout.set`, `session.list` (bentuk di overview 2.3 dan 2.1). `workspace.layout.get` -> `{workspaceId, groupId, layout: LayoutDoc|null}`; `workspace.layout.set` params `{workspaceId, groupId?, layout}`; `workspace.ui.set` params `{leftSidebar?, rightSidebar?, activeWorkspaceId?}`; `workspace.ui.get` -> `UIState` (`{leftSidebar:{visible,width}, rightSidebar:{visible,width,activeTab}, activeWorkspaceId}`).
- `lib/stores/layout.ts`: `layout.subscribe`, `layout.serialize()`, `layout.hydrate(raw: unknown)`.
- `lib/stores/shell.ts`: `leftSidebar`, `rightSidebar` (writable), `LEFT_MIN/LEFT_MAX/RIGHT_MIN/RIGHT_MAX/LEFT_DEFAULT/RIGHT_DEFAULT`.
- `lib/stores/workspaces.ts`: `workspaces`, `activeWorkspaceId`, `loadWorkspaces()`, `selectWorkspace(id)`.
- `lib/stores/sessions.ts`: `sessions`, `loadSessions()`. Jangan menambah pendengar `session.exit`.
- `modules/layout/model.ts`: tipe `LayoutDoc`, `Leaf`, `Split`, `Node` (hanya impor tipe/`leaves`).
- Window: `window.go.main.Bridge.WindowSaveState()` (metode Go milik window-state), selalu dalam `try/catch`.

Event `workspace.layoutChanged` dan `workspace.changed` TIDAK didengarkan oleh modul ini (tidak ada self-reload, tidak ada loop).

## Langkah

### 1. `frontend/src/lib/stores/persist.ts`

- `const status = writable<PersistStatus>('idle')`, `const saved = writable<number | null>(null)`.
- Ekspor `persistStatus = { subscribe: status.subscribe }` dan `lastSavedAt = { subscribe: saved.subscribe }` (Readable).
- `setPersistStatus(s)` -> `status.set(s)`; `markSaved(ts = Date.now())` -> `saved.set(ts)`.
- `let handler: (() => Promise<void>) | null = null`; `setFlushHandler(fn)` menyetelnya.
- `export async function flushNow(): Promise<void> { try { await handler?.(); } catch (e) { console.warn('[persist] flush gagal', e); } }`.
- Tidak mengimpor apa pun dari `modules/` (hindari siklus); handler didaftarkan oleh `autosave.ts`.

### 2. `frontend/src/modules/persist/windowState.ts`

```ts
export function scheduleWindowSave(): void;   // debounce 500 ms (setTimeout trailing), memanggil saveWindowNow
export async function saveWindowNow(): Promise<void>;  // batalkan timer, panggil bridge
export function startWindowWatch(): () => void; // addEventListener('resize', scheduleWindowSave); mengembalikan fungsi lepas
```

- `saveWindowNow`: `try { const b = (window as any).go?.main?.Bridge; if (b?.WindowSaveState) await b.WindowSaveState(); } catch {}` (kegagalan diabaikan, tanpa log berisik).
- Tanpa polling/interval; hanya event `resize` dan timer debounce.

### 3. `frontend/src/modules/persist/autosave.ts`

State modul (semua privat, satu instance):
- `currentWsId: string | null`: workspace yang layoutnya sedang berada di store layout.
- `currentGroupId: string`: `groupId` dari hasil `workspace.layout.get` terakhir (kosong = biarkan daemon memakai grup aktif; kirim `groupId` hanya bila tidak kosong).
- `lastLayoutJSON: string`, `lastUiJSON: string`: string JSON terakhir tersimpan/baseline.
- `writesEnabled = false`, `timer: ReturnType<typeof setTimeout> | null`, `retryTimer`, `writeChain: Promise<void>`.

Fungsi (signature):

```ts
export function setBaseline(wsId: string | null, groupId: string): void;
// currentWsId = wsId; currentGroupId = groupId;
// lastLayoutJSON = JSON.stringify(layout.serialize())  // SETELAH hydrate, agar dokumen kosong/hasil normalisasi tidak dianggap perubahan
// lastUiJSON = JSON.stringify(uiPayload())
export function enableWrites(on: boolean): void;
export function startWatching(): () => void;   // pasang subscribe + listener, kembalikan fungsi lepas
export function uiPayload(): { leftSidebar; rightSidebar; activeWorkspaceId: string };
export async function flush(): Promise<void>;  // simpan segera; dipakai flushNow, visibilitychange, beforeunload, pindah workspace
```

Detail:
1. `uiPayload()`: `{ leftSidebar: get(leftSidebar), rightSidebar: get(rightSidebar), activeWorkspaceId: currentWsId ?? get(activeWorkspaceId) ?? '' }` (salinan nilai; gunakan `get` dari `svelte/store`).
2. `schedule()`: bila `!writesEnabled` atau `get(persistStatus) === 'restoring'` -> return (tulisan tidak boleh jalan saat restoring). Selain itu `clearTimeout(timer); timer = setTimeout(() => { timer = null; void flush(); }, 500)` (debounce trailing 500 ms, satu tulisan per rentetan perubahan).
3. `startWatching()`: `layout.subscribe(schedule)`, `leftSidebar.subscribe(schedule)`, `rightSidebar.subscribe(schedule)`, `activeWorkspaceId.subscribe(schedule)`. Svelte store memanggil callback sekali saat subscribe; itu aman karena `schedule` lalu `flush` melewati tulisan bila JSON identik dengan baseline. Pasang juga `document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'hidden') void flushNow(); })` dan `window.addEventListener('beforeunload', () => { void flushNow(); })`, serta `startWindowWatch()`. Kembalikan satu fungsi yang melepas semuanya. Daftarkan `setFlushHandler(flush)` (dan lepas ke `null` saat dilepas). Impor `flushNow` dari store untuk listener agar jalur sama.
4. `flush()`:
   - `clearTimeout(timer); clearTimeout(retryTimer)`; bila `!writesEnabled` atau status `restoring` -> `return` (tetap panggil `saveWindowNow()` pada kasus `restoring` tidak perlu; lewati saja).
   - Tangkap snapshot SINKRON di awal (sebelum `await`): `wsId = currentWsId`, `groupId = currentGroupId`, `layoutJSON = JSON.stringify(layout.serialize())`, `ui = uiPayload()`, `uiJSON = JSON.stringify(ui)`. Snapshot ini penting untuk pindah workspace (layout workspace lama ditangkap sebelum diganti).
   - Rangkai ke `writeChain = writeChain.then(() => doWrite(...)).catch(() => {})` agar tulisan tidak tumpang tindih; `await writeChain`.
   - Selalu akhiri dengan `await saveWindowNow()` (try/catch di dalamnya).
5. `doWrite(wsId, groupId, layoutJSON, ui, uiJSON, attempt = 0)`:
   - Daftar tulisan: bila `wsId && layoutJSON !== lastLayoutJSON` -> `call('workspace.layout.set', { workspaceId: wsId, ...(groupId ? { groupId } : {}), layout: JSON.parse(layoutJSON) })`; bila `uiJSON !== lastUiJSON` -> `call('workspace.ui.set', ui)`. Bila tidak ada yang berbeda: lewati sepenuhnya (JSON identik tidak menulis).
   - Sukses per tulisan: perbarui `lastLayoutJSON`/`lastUiJSON` terkait (hanya bila `wsId === currentWsId` untuk layout, agar baseline workspace baru tidak tertimpa), `markSaved()`, dan bila status `error` set kembali `ready`.
   - Gagal: bila `attempt === 0` -> `retryTimer = setTimeout(() => { writeChain = writeChain.then(() => doWrite(wsId, groupId, layoutJSON, ui, uiJSON, 1)); }, 2000)` (retry sekali); bila `attempt === 1` -> `setPersistStatus('error')` dan `console.warn('[persist] simpan gagal', e)`. Tanpa dialog. Perubahan berikutnya dari pengguna tetap memicu `schedule()` normal (status `error` tidak menghentikan autosave).
   - `workspace.layout.set` dengan `workspace tidak ada` (not_found) diperlakukan sebagai kegagalan biasa; namun `switchTo` (langkah 6) tidak mem-flush workspace yang sudah dihapus.
6. Tidak ada `setInterval`, tidak ada `requestAnimationFrame` loop.

### 4. `frontend/src/modules/persist/restore.ts`

```ts
export async function restoreWorkspace(wsId: string | null): Promise<void>;
// langkah 3-6 untuk satu workspace; mengatur baseline lewat setBaseline; melempar bila layout.get gagal
export function applyUi(ui: unknown): void;       // langkah 2 (sidebar saja)
export function sanitizeDoc(raw: unknown, wsId: string, liveIds: Set<string>): unknown;
```

- `applyUi(ui)`: tolak non-objek. `leftSidebar.set({ visible: typeof v === 'boolean' ? v : true, width: clamp(w, LEFT_MIN, LEFT_MAX, LEFT_DEFAULT) })`; `rightSidebar.set({ visible, width: clamp(w, RIGHT_MIN, RIGHT_MAX, RIGHT_DEFAULT), activeTab: typeof activeTab === 'string' && activeTab ? activeTab : 'files' })`. `clamp` lokal (NaN/0 -> default; `Math.round(Math.min(max, Math.max(min, n)))`). Nilai tak valid -> default, tidak pernah melempar.
- `restoreWorkspace(wsId)`:
  1. Bila `wsId` null: `layout.hydrate(null)`, `setBaseline(null, '')`, return.
  2. `const res = await call<{workspaceId:string; groupId:string; layout: unknown|null}>('workspace.layout.get', { workspaceId: wsId })` (tanpa `groupId`: daemon memakai grup aktif). Kegagalan dibiarkan melempar ke pemanggil.
  3. `const sl = await call('session.list')`; `liveIds` = id sesi dengan `status === 'running'` dari `normalize(sl, 'sessions')` (terima array langsung atau `{sessions}`; salah bentuk -> himpunan kosong). Setelah itu `await loadSessions()` agar store `sessions` segar sebelum pane dirender (deteksi sesi mati di Wiring bergantung pada store itu); kegagalan `loadSessions` ditelan.
  4. `doc = sanitizeDoc(res?.layout ?? null, wsId, liveIds)`.
  5. `layout.hydrate(doc)` (aman untuk dokumen rusak/versi tak dikenal: jadi kosong).
  6. `setBaseline(wsId, res?.groupId ?? '')`.
- `sanitizeDoc` (langkah 5 spec, "bersihkan dokumen"): kloning dalam via JSON; telusuri pohon (rekursif bertahan terhadap bentuk salah; bila bukan objek kembalikan apa adanya agar `hydrate` yang menolak). Aturan: daun terminal TANPA `sessionId` dipertahankan; daun terminal dengan `sessionId` yang tidak ada di `liveIds` tetap dipertahankan (overlay "Sesi berakhir" milik Wiring/DeadSession yang menangani); daun file yang berkasnya tidak ada tetap dipertahankan (viewer file menampilkan status hilang). Isi daun tidak diubah sama sekali; jika ragu, kembalikan dokumen tanpa perubahan. Fungsi ini boleh sangat tipis (hanya kloning + penjagaan bentuk), yang penting tidak membuang daun apa pun dan tidak melempar.

### 5. `frontend/src/modules/persist/index.ts`

```ts
import { get } from 'svelte/store';
export async function startPersist(): Promise<void>;
```

Modul harus aman dimuat tanpa efek samping (dimuat eager oleh `main.ts` lewat glob). Tidak mengekspor `DeadSession`; tidak mengimpor `./register`, `./DeadSession.svelte`, `./TerminalGuard.svelte`.

`startPersist` (idempoten: `let started: Promise<void> | null`; panggilan kedua mengembalikan promise yang sama; seluruh isi dibungkus try/catch, tidak pernah reject):
1. `setPersistStatus('restoring')`; `enableWrites(false)`.
2. `workspace.ui.get` (try/catch; gagal -> `ui = null`, lanjut dengan default). `await loadWorkspaces()` bila `get(workspaces)` kosong (try/catch). Tentukan `target`: `ui?.activeWorkspaceId` bila ada di `get(workspaces)`; jika tidak, workspace pertama; jika tidak ada workspace, `null`. `applyUi(ui)`. Bila `target` tidak null dan berbeda dari `get(activeWorkspaceId)` -> `await selectWorkspace(target)` (menyetel project pertama dan `loadSessions()`); bila sama, tidak perlu. Set `switching = true` selama langkah ini agar subscribe pindah workspace (langkah 6) mengabaikan perubahan ini.
3. `await restoreWorkspace(target)` (mencakup `workspace.layout.get`, `session.list`, pembersihan, `layout.hydrate`, `setBaseline`). Bila melempar: `layout.hydrate(null)`, `setBaseline(target, '')`, `setPersistStatus('error')`, `console.warn`, tetap pasang `startWatching()` TAPI biarkan `enableWrites(false)` (jangan menimpa data tersimpan karena kegagalan baca); return.
4. `enableWrites(true)`; `setPersistStatus('ready')`; `startWatching()`; pasang watcher pindah workspace (langkah 6). Baseline sudah diset sehingga tidak ada tulisan sampai pengguna benar-benar mengubah sesuatu (menjamin layout null/rusak tidak menimpa data tersimpan).
5. `markSaved` TIDAK dipanggil di sini (hanya pada tulisan sukses).

### 6. Pindah workspace saat berjalan (di `index.ts`)

- Setelah ready: `activeWorkspaceId.subscribe((id) => { if (switching || id === currentWorkspace()) return; void switchTo(); })`, dengan nilai awal dilewati. `currentWorkspace()` mengembalikan `currentWsId` yang diekspor lewat getter `getCurrentWsId()` dari `autosave.ts`.
- `async function switchTo()` (latest-wins, tidak paralel): `switching = true`; loop `while (get(activeWorkspaceId) !== getCurrentWsId())`:
  1. `const old = getCurrentWsId(); const next = get(activeWorkspaceId)`.
  2. Flush layout workspace lama SEBELUM mengganti: bila `old` masih ada di `get(workspaces)` -> `await flush()` (menangkap snapshot sinkron dengan `currentWsId = old`; layout store masih berisi layout lama saat ini). Bila `old` sudah dihapus -> lewati flush (hindari not_found).
  3. `setPersistStatus('restoring')`; `enableWrites(false)`; batalkan timer yang menggantung.
  4. `try { await restoreWorkspace(next); enableWrites(true); setPersistStatus('ready'); schedule ui (lewat `flush` terjadwal) } catch { layout.hydrate(null); setBaseline(next, ''); setPersistStatus('error'); }` (pada error, writes tetap off sampai pindah workspace berikutnya yang sukses).
  5. Selesai loop: `switching = false`. Setelah pindah berhasil, panggil `scheduleSave()` yang diekspor `autosave.ts` agar `activeWorkspaceId` baru tersimpan lewat `workspace.ui.set` (JSON ui berbeda dari baseline).
- Layout workspace baru `null` -> `hydrate(null)` memberi layout kosong; baseline = dokumen kosong sehingga tidak menimpa apa pun sampai pengguna mengubah sesuatu. Layout tiap workspace independen karena disimpan/dimuat per `workspaceId`.

Ekspor tambahan yang diperlukan dari `autosave.ts`: `getCurrentWsId()`, `scheduleSave()` (alias `schedule`). Ini internal modul, bukan kontrak publik.

### 7. Verifikasi bangunan

Jalankan `cd frontend && npm run build` (guard). Pastikan tidak ada impor yang dilarang dan tidak ada `any` yang merusak `svelte-check`/tsc bila dipakai pada build (gunakan tipe sempit atau cast lokal untuk `window.go`).

## Perubahan

Dibuat:
- `frontend/src/lib/stores/persist.ts`
- `frontend/src/modules/persist/index.ts`
- `frontend/src/modules/persist/autosave.ts`
- `frontend/src/modules/persist/restore.ts`
- `frontend/src/modules/persist/windowState.ts`

Tidak ada file yang diubah.

## Wiring

Yang diekspos untuk disambungkan Wiring (jangan disambungkan di sini):
- `startPersist()` dari `modules/persist/index.ts`: dipanggil sekali oleh `register.ts` saat store `connection.state === 'ok'`. Idempoten dan tidak pernah reject.
- `persistStatus` dari `lib/stores/persist.ts`: dipakai `TerminalGuard.svelte` (daun dianggap mati hanya bila `persistStatus === 'ready'`; lihat overview 2.4).
- `flushNow()`: dapat dipakai pemanggil lain (mis. aksi tutup); `visibilitychange` dan `beforeunload` sudah dipasang internal oleh `startPersist`.
- `lastSavedAt`: untuk indikator opsional (tidak wajib).
- Tidak ada `import './register'` di `index.ts`; baris itu ditambahkan Wiring.

## Selesai bila

- `cd frontend && npm run build` sukses (guard; tanpa unit test).
- Hanya lima file di "Path yang dimiliki" yang dibuat; tidak ada file lain tersentuh; tidak ada impor `DeadSession.svelte`/`register`/`TerminalGuard`.
- Menyeret pembatas terus-menerus hanya menghasilkan satu `workspace.layout.set` setelah diam 500 ms; JSON identik tidak menulis; tidak ada tulisan saat `restoring`; kegagalan dicoba ulang sekali setelah 2 detik lalu `persistStatus = 'error'`.
- `startPersist` mengikuti urutan: restoring -> `workspace.ui.get` (sidebar + workspace aktif, fallback workspace pertama) -> `workspace.layout.get` -> `session.list` -> sanitasi -> `layout.hydrate` -> ready; layout null/rusak = layout kosong tanpa menimpa data tersimpan sebelum perubahan pengguna.
- Pindah workspace mem-flush layout workspace lama (snapshot sebelum layout diganti) lalu memuat layout workspace baru; layout independen per workspace.
- `flushNow` (visibilitychange hidden, beforeunload, pindah workspace) juga memanggil `WindowSaveState` dalam try/catch; resize didebounce 500 ms.
- Tidak ada polling, tidak ada dependency baru, tidak ada xterm dibuat di sini.

## Asumsi (catat bila kontrak tampak kurang)

- `Workspace` di `lib/stores/types.ts` tidak punya field grup; `groupId` diambil dari respons `workspace.layout.get` dan dikirim kembali saat simpan (kosong = grup aktif di daemon).
- `session.list` dipanggil langsung di `restore.ts` sesuai kontrak; `loadSessions()` tambahan hanya memastikan store `sessions` segar untuk deteksi sesi mati.
- Event `workspace.layoutChanged`/`workspace.changed` tidak didengarkan; ini memenuhi aturan "abaikan event tulisan sendiri" tanpa pelacakan penanda.
