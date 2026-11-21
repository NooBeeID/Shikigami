# Overview: Grup sesi sebagai tab

Tiket: `docs/tickets/shikigami-v2-ui-ticket-07-session-groups.md`
Spec: `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md` (bagian 5b, bagian 6 "Save workspace")
Stack: daemon Go (`internal/workspace`, JSON-RPC lewat `internal/ipc`) + frontend Svelte 5 (runes) + Vite + TypeScript di `frontend/`. Tanpa dependency baru, tanpa unit test.

Prinsip utama: **daemon dan `workspaces.json` adalah satu-satunya sumber kebenaran** grup. Frontend tidak punya `localStorage`, sidecar, atau cermin; state grup di frontend hanya turunan dari store `workspaces` yang diisi dari hasil method dan event daemon. (Versi plan lama yang memakai sidecar `localStorage` dan slot cermin SUDAH TIDAK BERLAKU; abaikan bila terlihat di berkas lama.)

## 1. Ringkasan epic dan urutan eksekusi

Tab grup tampil di atas area layout. Satu grup = satu konteks kerja milik workspace (sekumpulan sesi + layout pane sendiri). Tiket 06 sudah menyediakan skema data (`Workspace.Groups`, `ActiveGroupID`, `Group{ID,Name,Order,Layout}`) dan hanya method `workspace.layout.get/set` dan `workspace.ui.get/set`. Epic ini menambah di daemon: keanggotaan sesi, penanda grup bawaan, enam method `workspace.group.*`, event `workspace.groupsChanged`; lalu di frontend: store `lib/stores/groups.ts` dan modul UI `modules/groups`.

Urutan:

1. **Foundation** (ada, lihat `00-foundation.md`, hanya frontend): stub `groups.ts` yang memuat seluruh kontrak store (data tiruan in-memory) dan registri slot kosong di `modules/shell/registry.ts` dan `modules/sidebar-left/registry.ts`, supaya `group-tabs-ui` bisa dikompilasi paralel dengan `groups-store`. Sisi Go tidak butuh foundation (group-daemon berdiri sendiri; frontend hanya bergantung pada kontrak IPC di bagian 2.2).
2. **Fitur paralel**: `group-daemon` (Go), `groups-store` (menimpa stub), `group-tabs-ui`.
3. **Wiring** (satu agen, terakhir, bagian 5): host slot di `ShellFrame`, `AgentSidebar`, `SessionGroups`, `SessionRow`, verifikasi ujung ke ujung.

## 2. Kontrak bersama

### 2.1 Fakta repo (sudah dibaca)

Daemon (`internal/workspace/`, berkas: `config.go`, `ipc.go`, `layout.go`, `store.go`, `types.go`):
- `types.go`: `Workspace{ID,Name,Projects,Config,Groups,ActiveGroupID}`, `Group{ID,Name,Order,Layout json.RawMessage}`, `newDefaultGroup()` (nama "Utama", order 0), `newID()`, sentinel `ErrNotFound`/`ErrInvalid`/`ErrConflict`.
- `store.go`: `Store` dengan `s.mu`, `s.workspaces`, `cloneWorkspace`, `cloneWorkspaces`, `indexLocked(id)`, `persistLocked(ws, ui)`; pola mutasi: kloning -> ubah salinan -> `persistLocked` -> tukar `s.workspaces`.
- `layout.go`: `findGroup`, `GetLayout`/`SetLayout`/`GetUI`/`SetUI`. `config.go`: `loadWorkspaces(path)`, `workspacesFileVersion` (v2, TIDAK dinaikkan).
- `ipc.go`: `mapErr`, `decode`, `publish`, `publishLayout`, `Register(srv, s)`. `cmd/shikigamid/wire_workspace.go` sudah memanggil `workspace.Register`, tidak perlu diubah.

Frontend:
- `frontend/src/main.ts` memuat `./modules/*/index.ts` secara eager (`import.meta.glob`). `modules/groups/index.ts` otomatis dimuat; JANGAN edit `main.ts`/`App.svelte`.
- `ShellFrame.svelte`: `<main class="center">{@render center()}</main>` di grid; `center` = `LayoutArea`, `left` = `AgentSidebar`.
- `modules/shell/registry.ts`: pola `registerRightTab`/`registerTopbarItem` (`writable<Record<id,spec>>` + `derived` terurut `order` lalu `id` via `byOrder`). `modules/shell/helpers.ts`: `findSessionPane(sessionId)`, `focusOrOpenSession(sessionId): boolean`, `sessionTitle`, `openFilePane`, `createAndOpenSession`.
- `modules/sidebar-left/`: `AgentSidebar.svelte`, `SessionGroups.svelte` (pengelompokan per PROJECT), `SessionRow.svelte` (menu `...`, `menuOpen`, `closeMenu()`), `groups.ts` (tipe `SessionGroup` milik pengelompokan project; BUKAN `SessionGroup` epic ini; jangan diubah, impor dengan alias bila perlu).
- `lib/stores/layout.ts`: `layout` (`subscribe`, `openPane(leaf)`, `closePane(id)`, `setFocus`, `hydrate(raw)`, `serialize()`). `hydrate` membuang kunci tak dikenal (jangan menaruh metadata di dokumen layout).
- `modules/layout/model.ts` (fungsi murni, hanya diimpor): `LayoutDoc`, `Leaf`, `emptyDoc()`, `leaves(doc)`, `findLeafBy`, `findLeaf`, `openPane(doc, newLeaf)`, `closePane(doc, id)`, `hydrateDoc(raw)`. `modules/layout/dnd.ts`: `dragState: Readable<DragState|null>` publik (`DragState {paneId,title,targetId,zone,rect}`).
- `modules/persist/autosave.ts` (tiket 06): `currentGroupId`, `setBaseline(wsId, groupId)`, `flush`, `scheduleSave`, `getCurrentWsId`. `lib/stores/persist.ts`: `persistStatus`, `lastSavedAt`, `flushNow`, `setPersistStatus`. `restore.ts` memanggil `workspace.layout.get` tanpa `groupId` (= grup aktif daemon) lalu `setBaseline`.
- `lib/stores/workspaces.ts` (`workspaces`, `activeWorkspace`, `activeWorkspaceId`, `loadWorkspaces`), `lib/stores/sessions.ts` (`sessions`, `loadSessions`, `createSession`), `lib/stores/init.ts` (event `workspace.changed` -> `loadWorkspaces`; tidak diubah), `lib/ipc.ts` (`call`, `onEvent`), `lib/stores/types.ts`.
- Tema: variabel CSS `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok`; ukuran dalam `rem`/`em`; tanpa warna hardcode.

### 2.2 Skema data dan kontrak IPC daemon

Go (`internal/workspace/types.go`, aditif, skema file tetap v2):
```go
type Workspace struct {
    // ...field tiket 06 tidak berubah...
    Groups        []Group           `json:"groups"`
    ActiveGroupID string            `json:"activeGroupId"`
    Membership    map[string]string `json:"membership"` // sessionId -> groupId; non-nil saat keluar
}
type Group struct {
    ID     string          `json:"id"`
    Name   string          `json:"name"`
    Order  int             `json:"order"`
    Main   bool            `json:"main,omitempty"` // grup bawaan "Utama"; tepat satu per workspace
    Layout json.RawMessage `json:"layout"`
}
```
Aturan data:
- Identitas grup bawaan = `Group.Main`, bukan nama (ganti nama grup bawaan boleh). `newDefaultGroup()` mengisi `Main: true`.
- `normalizeGroups(w *Workspace)` dipanggil di `loadWorkspaces` per workspace (hanya memori, file tidak wajib ditulis ulang): (1) urut menurut `Order` (stabil) lalu nomor ulang 0..n-1; (2) tepat satu `Main`: bila >1 pertahankan order terkecil, bila 0 pilih grup order terkecil yang namanya "Utama" (trim, tanpa beda huruf besar/kecil), jika tak ada pilih order terkecil; tidak pernah membuat grup baru; (3) `Membership` dikosongkan saat load dan dijamin non-nil; `cloneWorkspace` menyalin map (selalu `{}`, bukan `null`).
- Sesi tanpa entri `Membership`, atau entri ke grup yang tidak ada, = milik grup `Main`. Entri ke grup yang sudah dihapus dibuang pada tiap mutasi grup; entri sesi berakhir dibiarkan sampai restart daemon.

Method IPC (semua menerima `workspaceId`; galat lewat `mapErr`: `ErrNotFound` workspace/grup tak ada, `ErrInvalid` nama/parameter tidak valid, `ErrConflict` operasi ditolak untuk grup `Main`; persist gagal -> state memori tidak berubah):

| Method | Params | Efek | Hasil |
|---|---|---|---|
| `workspace.group.create` | `{workspaceId, name?}` | grup baru di akhir (`order`=n), `layout: null`, id `newID()`; tidak mengubah grup aktif | `GroupsState` + `groupId` |
| `workspace.group.rename` | `{workspaceId, groupId, name}` | ganti nama (boleh grup bawaan) | `GroupsState` |
| `workspace.group.delete` | `{workspaceId, groupId}` | grup `Main` -> `ErrConflict` tanpa ubah state; selain itu hapus grup, `Membership` grup itu pindah ke `Main`, order dirapatkan, bila aktif maka `Main` aktif; tidak membunuh sesi | `GroupsState` |
| `workspace.group.reorder` | `{workspaceId, groupId, toIndex}` | pindah ke `toIndex` (dijepit 0..n-1), order rapat | `GroupsState` |
| `workspace.group.setActive` | `{workspaceId, groupId}` | set `ActiveGroupID`; tidak menyentuh layout | `GroupsState` |
| `workspace.group.moveSession` | `{workspaceId, sessionId, toGroupId}` | `Membership[sessionId]=toGroupId` (sessionId tak boleh kosong; grup tujuan harus ada; keberadaan sesi tidak diperiksa) | `GroupsState` |

`GroupsState` (hasil semua method dan payload event; tanpa layout):
```json
{ "workspaceId": "...", "activeGroupId": "...",
  "groups": [ { "id": "...", "name": "...", "order": 0, "main": true } ],
  "membership": { "<sessionId>": "<groupId>" } }
```
Hasil `create` menambah `"groupId"`. Tipe Go: `GroupsState`, `GroupInfo` di `groups.go`; method `Store`: `CreateGroup`, `RenameGroup`, `DeleteGroup`, `ReorderGroup`, `SetActiveGroup`, `MoveSession`.

Aturan nama (create bila `name` diisi, dan rename): trim; maks 40 rune (lebih ditolak `ErrInvalid`, tidak dipotong); kosong ditolak untuk rename. Create tanpa nama: "Grup N", N = jumlah grup + 1, naik sampai unik. Bentrok nama (tanpa beda huruf besar/kecil, terhadap grup lain) tidak ditolak: akhiran " 2", " 3", dst. (hasil boleh melewati 40). Rename ke nama sendiri/beda huruf besar-kecil bukan bentrok.

Event: `workspace.groupsChanged` payload `{kind, ...GroupsState}`, `kind` salah satu `groupCreated`, `groupRenamed`, `groupDeleted`, `groupReordered`, `groupActivated`, `sessionMoved`. Dipublikasikan setelah persist berhasil, tidak untuk no-op. `publishGroups(srv, kind, state)` di `ipc_groups.go`. `workspace.changed`/`workspace.layoutChanged` tidak berubah perilaku. `workspace.list` mengeluarkan `membership` (objek) dan `main`.

### 2.3 Kontrak store frontend (`frontend/src/lib/stores/groups.ts`; Foundation membuat stub bentuk sama)

Tipe `lib/stores/types.ts` (aditif, dikerjakan `groups-store`): `Workspace` mendapat `groups?: WorkspaceGroup[]`, `activeGroupId?: string`, `membership?: Record<string,string>`; `WorkspaceGroup {id, name, order, main?: boolean, layout?: unknown | null}`.

```ts
export interface SessionGroup { id: string; name: string; order: number; main: boolean }
export const MAIN_GROUP_NAME = 'Utama';   // nama tampilan bawaan; identitas lewat flag `main`

export const groups: Readable<SessionGroup[]>;          // urut order naik; kosong bila data belum ada
export const mainGroupId: Readable<string>;
export const activeGroupId: Readable<string>;           // fallback mainGroupId bila id tak ditemukan
export const activeGroup: Readable<SessionGroup | undefined>;
export const sessionGroupMap: Readable<Record<string, string>>;    // sessionId -> groupId (tanpa entri = main)
export const groupSessionCounts: Readable<Record<string, number>>; // groupId -> jumlah sesi di store sessions
export const groupsError: Readable<string | null>;                 // null bila bersih

export function initGroups(): () => void;                          // pendengar event/store; mengembalikan cleanup
export function createGroup(name?: string): Promise<string>;       // id; lalu aktifkan
export function renameGroup(id: string, name: string): Promise<void>;
export function deleteGroup(id: string): Promise<void>;
export function reorderGroup(id: string, toIndex: number): Promise<void>;
export function setActiveGroup(id: string): Promise<void>;
export function moveSession(sessionId: string, toGroupId: string): Promise<void>;
export function groupOfSession(sessionId: string): string | undefined;
```
Perilaku (rinci di tiket, bagian "Aturan perilaku store"; ringkasan wajib):
- Tanpa state grup selain turunan `workspaces`; hasil panggilan dan event `workspace.groupsChanged` diterapkan idempoten ke entri workspace di store `workspaces` (timpa `groups` kecuali `layout` dipertahankan dari entri lama, `activeGroupId`, `membership`). Event hanya memperbarui data, tidak pernah mengganti layout.
- `setActiveGroup(id)`: no-op bila sudah aktif; `await flushNow()`; `workspace.group.setActive`; `workspace.layout.get {workspaceId, groupId:id}`; `switchGroup(id, doc)` dari `modules/persist/autosave.ts`.
- `createGroup`: `workspace.group.create` lalu `setActiveGroup(groupId)`. `deleteGroup`: bila aktif, `setActiveGroup(mainGroupId)` dulu, lalu `workspace.group.delete`. `moveSession`: tutup pane sesi di grup asal (aktif lewat `layout.closePane`; non-aktif lewat `workspace.layout.get` + model murni + `workspace.layout.set`), `workspace.group.moveSession`, bila tujuan grup aktif buka pane via `layout.openPane`.
- Sesi baru tanpa entri: pendengar `sessions` mencatat snapshot awal per workspace; id baru dengan grup aktif non-`main` memanggil `workspace.group.moveSession` ke grup aktif (tanpa menyentuh layout). Tanpa polling.
- Galat daemon -> `groupsError` (dikosongkan pada aksi sukses berikutnya), state lokal tidak berubah; bukan `alert`.

Perluasan `modules/persist/autosave.ts` (dikerjakan `groups-store`, aditif): `getCurrentGroupId(): string` dan `switchGroup(groupId: string, doc: unknown): void` (`layout.hydrate(doc)` lalu `setBaseline(currentWsId, groupId)` sinkron; tidak membersihkan timer retry).

### 2.4 Registri slot (dibuat Foundation, dikonsumsi `modules/groups/index.ts`, host dirender Wiring)

```ts
// frontend/src/modules/shell/registry.ts  (ditambah; re-export di modules/shell/index.ts)
export interface LayoutHeaderSpec { id: string; component: Component<any>; order?: number }
export function registerLayoutHeader(spec: LayoutHeaderSpec): void;
export function layoutHeaders(): Readable<LayoutHeaderSpec[]>;          // terurut order lalu id

// frontend/src/modules/sidebar-left/registry.ts  (BARU; re-export di modules/sidebar-left/index.ts)
export interface SessionListWrapperProps {
  onerror: (msg: string) => void;
  list: Snippet<[filter: (s: Session) => boolean]>;   // render SessionGroups hanya untuk sesi yang lolos filter
}
export interface SessionListWrapperSpec { id: string; component: Component<SessionListWrapperProps> }
export function registerSessionListWrapper(spec: SessionListWrapperSpec): void;   // terakhir menang
export const sessionListWrapper: Readable<SessionListWrapperSpec | null>;

export interface SessionMenuItemProps { sessionId: string; onDone?: () => void }
export interface SessionMenuItemSpec { id: string; component: Component<SessionMenuItemProps>; order?: number }
export function registerSessionMenuItem(spec: SessionMenuItemSpec): void;
export function sessionMenuItems(): Readable<SessionMenuItemSpec[]>;
```
Tanpa registrasi, ketiganya tidak mengubah tampilan atau perilaku.

### 2.5 Komponen UI (`frontend/src/modules/groups/`, milik `group-tabs-ui`)

- `GroupTabs.svelte` (tanpa props wajib), `GroupSessionMenu.svelte` (props `{sessionId, onDone?}`), `GroupedSessionList.svelte` (props `SessionListWrapperProps`; bagian grup aktif terbuka dulu, grup lain dilipat; baris sesi tetap dirender lewat `list(filter)`), `index.ts`.
- `index.ts`: `initGroups()`; `registerLayoutHeader({id:'groups', component: GroupTabs, order: 0})`, `registerSessionListWrapper({id:'groups', component: GroupedSessionList})`, `registerSessionMenuItem({id:'move-to-group', component: GroupSessionMenu, order: 50})`. Impor registri dari `../shell/registry` dan `../sidebar-left/registry` (bukan dari `index.ts` modul itu).
- Konvensi DOM bersama dengan Wiring: tab grup memakai `data-group-tab-id="<groupId>"`; elemen akar baris sesi sidebar kiri punya `data-session-id="<sessionId>"` (ditambahkan Wiring ke `SessionRow.svelte`). Drag sesi dari sidebar: pointer events milik modul groups (delegasi `pointerdown` pada kontainer `GroupedSessionList`, ghost sendiri, `document.elementsFromPoint` ke `[data-group-tab-id]`), BUKAN HTML5 drag. Drag header pane: langganan `dragState` dari `modules/layout/dnd`; `sessionId` dari leaf terminal di layout aktif (pane file diabaikan); `moveSession` dipanggil saat pointer dilepas di atas tab non-aktif. Klik baris sesi di bagian grup LAIN: tangkap fase capture (`onclickcapture`), baca `data-session-id`, `await setActiveGroup(g)` lalu `focusOrOpenSession(id)`. Hover ~600 ms di tab non-aktif saat drag boleh mengaktifkannya (timeout sekali jalan, bukan polling).
- Pindah grup melakukan `hydrate` layout (pane grup lama di-unmount); sesi tetap hidup di daemon dan di-attach ulang saat grup aktif lagi. Jangan menyimpan instance xterm di luar kontrak layout tiket 03.
- Menu `...` header pane (`modules/layout/PaneMenu.svelte`) tidak punya titik ekstensi; JANGAN ubah `modules/layout`. "Pindah ke grup" lewat menu sesi sidebar kiri dan drag header pane ke tab; catat batasan di hasil Wiring.

### 2.6 Konvensi penamaan

- Slug = nama berkas plan: `group-daemon.md`, `groups-store.md`, `group-tabs-ui.md` (+ `00-foundation.md`, `wiring.md`) di folder plan ini.
- Go: paket `workspace`, nama method `Store` berawalan kata kerja (`CreateGroup`...), method IPC `workspace.group.<aksi>`, event `workspace.groupsChanged`.
- Svelte 5 runes (`$props`, `$state`, `$derived`, `$effect`), snippet untuk slot, TypeScript. Identifier kode Inggris; teks UI dan pesan galat Indonesia.
- Impor store dari berkasnya langsung, bukan `lib/stores/index.ts`; `groups.ts` TIDAK ditambahkan ke `lib/stores/index.ts`.
- `role="tablist"`/`role="tab"`, `aria-selected`, panah kiri/kanan berpindah tab. Warna dari variabel tema.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `group-daemon` | Daemon: `Membership`, `Group.Main`, normalisasi saat load, `GroupsState`, enam method `Store`, enam method IPC `workspace.group.*`, event `workspace.groupsChanged`; perluasan aditif `types.go`, `store.go`, `config.go`, `ipc.go` (satu baris `registerGroupMethods`) | `internal/workspace/groups.go`, `internal/workspace/ipc_groups.go` (+ perluasan aditif `internal/workspace/types.go`, `store.go`, `config.go`, `ipc.go`) |
| `groups-store` | Implementasi nyata `groups.ts` menggantikan stub: store turunan, aksi lewat `workspace.group.*`, penerapan `GroupsState`/event, `setActiveGroup` dengan `flushNow`+`switchGroup`, penugasan sesi baru, `groupsError`; perluasan `lib/stores/types.ts` dan `modules/persist/autosave.ts` (`getCurrentGroupId`, `switchGroup`) | `frontend/src/lib/stores/groups.ts` (+ perluasan aditif `frontend/src/lib/stores/types.ts`, `frontend/src/modules/persist/autosave.ts`) |
| `group-tabs-ui` | `GroupTabs`, menu konteks tab, edit nama inline, drag urut tab, drag sesi (sidebar + header pane) ke tab, `GroupSessionMenu`, `GroupedSessionList`, `index.ts` pendaftaran slot, aksesibilitas | `frontend/src/modules/groups/` |

Rincian perilaku dan acceptance criteria: tiket bagian Fitur. Jangan menambah/menggabung fitur. Penyesuaian terhadap tiket: `GroupedSessionList` menerima snippet `list(filter)` alih-alih `renderSession`; drag sidebar memakai pointer events; perluasan `autosave.ts` dikerjakan `groups-store` (bukan Wiring) karena kode `groups.ts` mengimpornya.

## 4. Foundation

Ada (`hasFoundation=true`): lihat `00-foundation.md`. Hanya frontend: (a) stub `groups.ts` sesuai 2.3 dengan data tiruan in-memory; (b) registri slot 2.4 (`modules/shell/registry.ts`, `modules/shell/index.ts`, `modules/sidebar-left/registry.ts`, `modules/sidebar-left/index.ts`), aditif dan tanpa perubahan perilaku. Foundation tidak menyentuh Go, `types.ts`, `autosave.ts`, maupun `modules/groups/`.

## 5. Wiring checklist (agen Wiring, setelah ketiga fitur selesai dan ter-merge)

Boleh menyentuh HANYA: `frontend/src/modules/shell/ShellFrame.svelte`, `frontend/src/modules/shell/LayoutHeaderHost.svelte` (baru), `frontend/src/modules/sidebar-left/AgentSidebar.svelte`, `frontend/src/modules/sidebar-left/SessionGroups.svelte` (hanya prop `filter` opsional), `frontend/src/modules/sidebar-left/SessionRow.svelte`. Baca bentuk aktual berkas dulu; perubahan sekecil mungkin; tanpa registrasi harus identik secara tampilan.

1. **Daemon**: pastikan `Register` di `internal/workspace/ipc.go` memanggil `registerGroupMethods(srv, s)` (satu baris di akhir), `cmd/shikigamid/wire_workspace.go` tidak berubah, `workspacesFileVersion` tetap v2. Hanya verifikasi; perbaikan kecil boleh bila baris hilang.
2. **Persist**: pastikan `autosave.ts` mengekspor `getCurrentGroupId` dan `switchGroup` (dari `groups-store`); `restore.ts` dan `index.ts` modul persist tidak diubah.
3. **Layout header**: buat `LayoutHeaderHost.svelte` (membaca `layoutHeaders()` dari `./registry`, merender komponen terurut; kosong -> tidak merender elemen). Di `ShellFrame.svelte` ubah `<main class="center">` jadi kolom: `LayoutHeaderHost` lalu `<div class="center-body">{@render center()}</div>` (`.center {display:grid; grid-template-rows:auto minmax(0,1fr)}`, `.center-body {min-width:0; min-height:0; overflow:hidden}`); `LayoutArea` tetap mengisi tinggi penuh. Bila tiket 04 sudah punya slot atas-layout, pakai itu.
4. **Pembungkus daftar sesi**: `SessionGroups.svelte` terima prop opsional `filter?: (s: Session) => boolean` (default semua; terapkan sebelum pengelompokan per project). `AgentSidebar.svelte`: baca `sessionListWrapper`; bila ada render `<W.component {onerror} list={listSnippet} />` dengan `{#snippet listSnippet(filter)}<SessionGroups {onerror} {filter} />{/snippet}`; bila tidak, `<SessionGroups {onerror} />` seperti semula.
5. **Menu sesi + hook DOM**: `SessionRow.svelte`: `data-session-id={session.id}` pada elemen akar; di menu `...` render `sessionMenuItems()` (`<It.component sessionId={session.id} onDone={closeMenu} />`) di atas "Akhiri sesi".
6. **Impor modul**: `modules/groups/index.ts` dimuat otomatis oleh glob `main.ts`; tidak ada impor tambahan. Pastikan registri diimpor langsung (tanpa `undefined` akibat urutan muat).
7. **Sesi baru**: pastikan sesi dari `+ sesi`, split, atau klik file masuk grup aktif lewat pendengar `sessions` di `groups.ts`; bila tidak, catat sebagai temuan (jangan ubah jalur pembuatan sesi).
8. **Flag izin agent** (`--dangerously-skip-permissions`, `--dangerously-bypass-approvals-and-sandbox`): tidak disentuh dan tidak digandakan.
9. **Verifikasi manual** (`make build`, jalankan app): buat dua grup tambahan, isi tiap grup dengan layout dan sesi berbeda, ganti nama (menu, klik ganda, `+` langsung edit; Esc batal; kosong ditolak), urutkan dengan drag, pindahkan sesi lewat menu dan drag, hapus grup berisi sesi (sesi pindah ke "Utama" dan tetap hidup), "Utama" tanpa Hapus, grup baru terlihat di `workspace.list` dan `workspaces.json`, tutup-buka aplikasi dan restart `shikigamid`: grup, urutan, grup aktif, layout pulih; uji `workspaces.json` v2 lama tanpa `membership`/`main`: dimuat tanpa grup baru. Catat batasan menu `...` header pane.
10. Jalankan build guard (bagian 6).

## 6. Build guard

Satu-satunya guard: `go build ./...` dan `make build` (frontend `cd frontend && npm install && npm run build`, `go build` daemon/ctl, `go build -tags desktop,production ./app`). Per fitur frontend di worktree boleh `cd frontend && npm run build` (dan `npm run check` bila ada); per fitur Go `go build ./...`. Tanpa unit test. Aplikasi harus tetap bisa dijalankan.

## 7. Aturan paralel

- Tiap agent hanya menyentuh `ownedPaths` fiturnya beserta perluasan aditif yang tercantum di bagian 3 untuk fitur itu. Foundation hanya menyentuh berkas di `00-foundation.md`.
- Jangan menyentuh path fitur lain, berkas milik tiket lain di luar perluasan yang dicatat (`modules/layout`, `modules/persist` selain `autosave.ts` oleh `groups-store`, `lib/stores/layout.ts|sessions.ts|workspaces.ts|persist.ts|shell.ts|init.ts`, `main.ts`, `App.svelte`, `app/`, `cmd/`), maupun berkas Wiring (bagian 5).
- Antar fitur hanya lewat kontrak bagian 2. `group-tabs-ui` memakai stub Foundation tanpa mengandalkan detail dalam `groups.ts`; `groups-store` memakai kontrak IPC 2.2 (respons tiruan bila daemon belum siap). Bila kontrak tampak kurang, ikuti kontrak dan catat asumsi di akhir plan fitur.
- Tanpa dependency baru, tanpa `setInterval`/polling/timer baru, tanpa warna hardcode, tanpa `alert`, tanpa `localStorage`/sidecar untuk state grup.
