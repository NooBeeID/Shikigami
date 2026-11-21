# Overview: Save workspace otomatis dan pemulihan

Tiket: `docs/tickets/shikigami-v2-ui-ticket-06-workspace-persist.md`
Spec: `docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md` (bagian 1 poin 1, bagian 5b, bagian 6 "Save workspace", bagian 9)
Stack: daemon Go (`internal/workspace`, modul di `cmd/shikigamid`), app Wails Go (`app/`, `package main`), frontend Svelte 5 (runes) + Vite + TypeScript di `frontend/`. Tanpa dependency baru (Go maupun npm), tanpa unit test.

## 1. Ringkasan epic dan urutan eksekusi

Layout pane, sidebar, workspace aktif, dan posisi jendela disimpan otomatis dan dipulihkan saat aplikasi dibuka lagi. Daemon menaikkan `workspaces.json` ke v2 (workspace -> grup -> layout, plus blok `ui`) dengan migrasi otomatis dari v1, dan menyediakan IPC `workspace.layout.get/set` dan `workspace.ui.get/set`. Frontend menyimpan berdebounce dan memulihkan (pane terminal di-attach ke sesi hidup, pane file membuka ulang berkas, sesi mati menampilkan overlay "Sesi berakhir" + "Mulai ulang"). `app/window.go` mengingat posisi/ukuran jendela lewat appconfig.

Urutan:

1. **Foundation: TIDAK ADA** (`hasFoundation=false`, tidak ada `00-foundation.md`). Alasan: keempat fitur menyentuh path terpisah dan hanya bersambung lewat kontrak tertulis di dokumen ini (kontrak IPC, tipe layout, prop komponen). Semua yang dipakai dari tiket lain sudah ada di repo (bagian 2.1).
2. **Fitur paralel** (path terpisah): `schema-v2-daemon`, `autosave-restore`, `restart-dead-session`, `window-state`.
3. **Wiring** (satu agen, terakhir, bagian 5): `modules/persist/register.ts`, `TerminalGuard.svelte`, satu baris di `modules/persist/index.ts`, satu baris `OnBeforeClose` di `app/main.go`, uji manual end-to-end.

## 2. Kontrak bersama

### 2.1 Fakta repo yang dipakai (sudah dibaca; jangan diubah)

Daemon / Go:
- Modul `internal/workspace`: `config.go` (konstanta `fileVersion = 1` dipakai BERSAMA oleh `workspaces.json` dan `config.json` lewat `configDoc`; `loadWorkspaces(path) ([]Workspace, error)`; `writeJSONAtomic(path, v)` izin 0600), `types.go` (`Project`, `WorkspaceConfig`, `Workspace{ID,Name,Projects,Config}`, `GlobalConfig`, sentinel `ErrNotFound/ErrInvalid/ErrConflict`, `newID()` ULID), `store.go` (`Store{mu, wsPath, cfgPath, workspaces, global}`; `cloneWorkspace` baru menyalin `Projects`; `persistWorkspacesLocked(ws)` menulis `workspacesDoc{Version: fileVersion, Workspaces: ws}`; `Create/Rename/Delete/AddProject/RemoveProject/List/Get`), `ipc.go` (`mapErr`, `decode`, `publish(srv, kind, wsID, ws)` memublikasi `workspace.changed`, `Register(srv, s)`).
- `cmd/shikigamid/wire_workspace.go` memanggil `workspace.NewStore(dir)` lalu `workspace.Register(srv, store)` lewat `RegisterModule("workspace", ...)`.
- IPC server: `srv.Register(method, func(ctx, params json.RawMessage) (any, error))`, `srv.Publish(topic, payload)`, `ipc.NewError(ipc.ErrNotFound|ErrInvalid|ErrConflict, msg)`.
- `session.list` (tanpa param; hasil `{sessions: Session[]}` dengan `Session {id, workspaceId, projectId, cwd, providerId, status: 'running'|'exited', createdAt, exitCode?}`), `session.create` (param `{workspaceId, projectId, cwd, providerId, cols?, rows?, env?}`), `session.attach` (`{id}` -> `{session, scrollback(base64), offset}`), event `session.exit {id, exitCode}`. Flag izin agent ditambahkan adapter provider di daemon; frontend TIDAK menyusun argumen provider (kontrak no. 2).
- `internal/appconfig`: `Config{Zoom int, Window *Window{X,Y,W,H int}}`; `Store.Get()`, `Store.Patch(map[string]any)` yang menggabung (patch `window` hanya mengganti kunci `x,y,w,h` yang dikirim; `zoom` tidak tersentuh). Paket ini TIDAK diubah.
- `app/bridge.go` (milik ticket-02, jangan diubah): `type Bridge` dengan `ctx`/`ctxMu`, `AppConfigGet() (appconfig.Config, error)`, `AppConfigSet(patch map[string]any) (appconfig.Config, error)`, `configStore()`, dan mekanisme `registerStartupHook(fn func(ctx context.Context, b *Bridge))` yang dipanggil di `SetContext` (hook hanya didaftarkan dari `init()`). `app/main.go` (`package main`) menyetel `OnStartup: bridge.SetContext`, `Width 1200`, `Height 800`, `MinWidth 800`, `MinHeight 500`, `OnShutdown`, tanpa `OnBeforeClose`. Semua file `app/*.go` adalah `package main`. Wails runtime tersedia: `runtime.WindowGetPosition/WindowGetSize/WindowSetPosition/WindowSetSize/WindowIsMaximised/WindowIsFullscreen/ScreenGetAll`.

Frontend:
- `frontend/src/main.ts` memuat `./modules/*/index.ts` secara eager lewat `import.meta.glob`; JANGAN edit `main.ts`. Akibat: `modules/persist/index.ts` otomatis dimuat saat start (harus aman dimuat tanpa efek samping sampai bagian wiring menambahkan `register`).
- `lib/ipc.ts`: `call<T>(method, params?)` (membuang error bila bukan di Wails), `onEvent(topicPrefix, fn)`, `inWails`. Memanggil method Go pada Bridge (mis. `WindowSaveState`) dilakukan lewat `window.go.main.Bridge.<Method>()` yang dibungkus `try/catch`.
- `lib/stores/shell.ts` (ticket-04): `leftSidebar: Writable<{visible,width}>`, `rightSidebar: Writable<{visible,width,activeTab}>`, konstanta `LEFT_*/RIGHT_*`, setter `toggleLeft/Right`, `setLeftWidth/RightWidth`, `setRightTab`. Pemulihan memakai `leftSidebar.set(...)`/`rightSidebar.set(...)` dengan nilai yang sudah di-clamp ke `LEFT_MIN..LEFT_MAX` / `RIGHT_MIN..RIGHT_MAX`.
- `lib/stores/workspaces.ts`: `workspaces`, `activeWorkspaceId` (`Writable<string|null>`), `activeWorkspace`, `activeProjectId`, `selectWorkspace(id)` (menyetel workspace aktif, project pertama, dan `loadSessions()`), `loadWorkspaces()`. `lib/stores/sessions.ts`: `sessions: Writable<Session[]>` (sudah menandai `status:'exited'` saat event `session.exit` diterima oleh `initStores`; JANGAN menambah pendengar `session.exit` kedua), `loadSessions()`, `createSession(providerId)` (tidak dipakai di sini, karena butuh project aktif; gunakan `call('session.create', ...)` langsung). `lib/stores/init.ts`: `connection: Writable<{state:'connecting'|'ok'|'error', message}>` dan `initStores()`; event `workspace.changed` memicu `loadWorkspaces()`.
- `lib/stores/layout.ts` (ticket-03): `layout: LayoutStore` dengan `subscribe` (nilai `LayoutDoc`), `serialize(): LayoutDoc` (salinan dalam), `hydrate(raw: unknown)` (aman terhadap dokumen rusak/versi tak dikenal: jadi dokumen kosong), `leaves: Readable<Leaf[]>`, `openPane`, `closePane`, `setFocus`, `maximize`, dst. TIDAK ada fungsi "ubah atribut daun"; untuk mengganti `sessionId` satu daun gunakan pola `const d = layout.serialize(); <ubah daun di pohon>; layout.hydrate(d)` (id daun, posisi, rasio, fokus, dan maksimalkan tetap karena `hydrateDoc` mempertahankan semuanya).
- `modules/layout/model.ts`: `Leaf = {type:'pane', id, kind, sessionId?, projectId?, provider?, filePath?, title, workspaceId?}`, `Split = {type:'split', dir:'h'|'v', ratio, a, b}`, `LayoutDoc = {version:1, root: Node|null, focusId?, maximizedId?: string|null}`, `emptyDoc()`, `leaves(doc)`, `hydrateDoc(raw)`. `Leaf.provider` berisi id provider (`claude|codex|shell`) = `providerId` pada `session.create`.
- `modules/layout/registry.ts`: `registerPaneKind(spec: PaneKindSpec)` (menimpa spec dengan `kind` sama), `getPaneKind(kind)`, `paneKinds()`. `PaneKindSpec {kind, component: Component<{pane: Leaf; visible: boolean}>, titleFor?, onEndSession?, badgeFor?}`. Tidak ada slot overlay; jenis `terminal` didaftarkan di `modules/layout/panes/terminal.ts` dengan komponen `TerminalPane.svelte` (render `TerminalTab` bila `pane.sessionId` ada). Mekanisme overlay di epic ini: mendaftarkan ulang `terminal` dengan komponen pembungkus (bagian 5, langkah 3).
- Tema: variabel CSS `--bg --bg-elev --bg-hover --fg --fg-muted --border --accent --danger --ok`; komponen memakai variabel itu (fallback boleh), tanpa warna hardcode.

### 2.2 Skema `workspaces.json` v2 (kontrak bersama no. 5)

```jsonc
{
  "version": 2,
  "workspaces": [
    {
      "id": "ULID", "name": "Core",
      "projects": [{ "id": "...", "path": "/abs", "name": "core" }],
      "config": { "grimoireUrl": "...", "grimoireKey": "..." },
      "groups": [ { "id": "ULID", "name": "Utama", "order": 0, "layout": null /* LayoutDoc | null */ } ],
      "activeGroupId": "ULID"
    }
  ],
  "ui": {
    "leftSidebar":  { "visible": true, "width": 260 },
    "rightSidebar": { "visible": true, "width": 280, "activeTab": "files" },
    "activeWorkspaceId": "ULID"
  }
}
```

Tipe Go (paket `workspace`; `Group`, `SidebarState`, `UIState` di `types.go`):

```go
type Group struct {
    ID     string          `json:"id"`
    Name   string          `json:"name"`
    Order  int             `json:"order"`
    Layout json.RawMessage `json:"layout"` // dokumen opaque; JSON null bila kosong (jangan omitempty)
}
// Workspace ditambah:
//   Groups        []Group `json:"groups"`        // selalu >= 1, tidak pernah null
//   ActiveGroupID string  `json:"activeGroupId"`
type SidebarState struct {
    Visible   bool   `json:"visible"`
    Width     int    `json:"width"`
    ActiveTab string `json:"activeTab,omitempty"` // hanya sidebar kanan
}
type UIState struct {
    LeftSidebar       SidebarState `json:"leftSidebar"`
    RightSidebar      SidebarState `json:"rightSidebar"`
    ActiveWorkspaceID string       `json:"activeWorkspaceId"`
}
// workspacesDoc ditambah: UI UIState `json:"ui"`
```

Aturan skema:
- Grup bawaan "Utama", `order` 0, id ULID baru dari `newID()`. `Workspace.Create` selalu membuat grup itu dan mengisi `activeGroupId`. Penegakan "tidak bisa dihapus" milik ticket-07.
- Versi: pisahkan konstanta. `workspacesFileVersion = 2` untuk `workspaces.json`; `configFileVersion = 1` untuk `config.json` (`SetGlobalConfig` memakai `configFileVersion`; nama `fileVersion` lama dihapus atau dialihkan, semua referensi diperbarui di paket `workspace`).
- Migrasi di `loadWorkspaces` (v1 -> v2, idempoten): tiap workspace v1 mendapat satu grup "Utama" (`layout` null) dan `activeGroupId`; `ui` default (`leftSidebar {true,260}`, `rightSidebar {true,280,"files"}`, `activeWorkspaceID` = workspace pertama atau kosong). Setelah migrasi: salin file asli ke `workspaces.json.v1.bak` hanya bila belum ada, lalu tulis ulang atomik sebagai versi 2. Membaca ulang file v2 tidak menulis apa pun. `version` > 2 -> error jelas, file tidak diubah, tidak ada cadangan. File v2 yang kurang field (grup kosong/`activeGroupId` kosong/`ui` nol) dinormalisasi di memori tanpa wajib menulis ulang.
- Layout opaque bagi daemon: hanya divalidasi sebagai JSON objek valid atau `null`, maksimum 1 MiB (`1<<20` byte). Bentuk dokumen (kontrak no. 4) dipakai frontend: `{version:1, root:Node|null, focusId?, maximizedId?}`, Leaf `{type:'pane', id, kind:'terminal'|'file', sessionId?, projectId?, provider?, filePath?, workspaceId?, title}`, Split `{type:'split', dir:'h'|'v', ratio, a, b}`.

### 2.3 Method IPC (didaftarkan di `internal/workspace/ipc.go` lewat `Register`)

| Method | Params | Hasil |
|---|---|---|
| `workspace.layout.get` | `{workspaceId, groupId?}` | `{workspaceId, groupId, layout: LayoutDoc\|null}`; `groupId` kosong = grup aktif workspace |
| `workspace.layout.set` | `{workspaceId, groupId?, layout}` | `{ok:true}`; `layout` null/absen-null menghapus; HANYA memublikasi `workspace.layoutChanged`, bukan `workspace.changed` |
| `workspace.ui.get` | `{}` | `UIState` |
| `workspace.ui.set` | `{leftSidebar?, rightSidebar?, activeWorkspaceId?}` (patch parsial; gabung per field; sidebar dikirim utuh per sisi) | `UIState` hasil gabung |

Kesalahan: workspace/grup tak ada -> `ipc.ErrNotFound`; layout bukan objek/null atau > 1 MiB -> `ipc.ErrInvalid`; `activeWorkspaceId` tak dikenal -> `ErrInvalid` (string kosong diizinkan untuk mengosongkan). Event baru `workspace.layoutChanged {workspaceId, groupId}`. `workspace.list/create/rename/...` kini ikut mengembalikan `groups` (dengan `layout`) dan `activeGroupId`; `workspace.delete` pada workspace aktif membersihkan `ui.activeWorkspaceId`.

Catatan tipe: `workspace.layout.get` mengembalikan `layout` apa adanya (objek atau `null`). `workspace.list` boleh mengembalikan layout di dalam `groups`; frontend mengabaikannya (sumber kebenaran layout hanya `workspace.layout.get`).

Konkurensi: semua mutasi di bawah `Store.mu`; hasil get adalah salinan dalam (`cloneWorkspace` menyalin `Groups` beserta `Layout` byte-nya). Untuk `layout.set` yang sering, lakukan validasi/serialisasi sebelum mengambil lock, dan tahan lock hanya selama membangun salinan baru + tulis atomik + tukar slice (pola salin-lalu-tukar yang sama dengan metode lain).

### 2.4 Kontrak frontend

```ts
// frontend/src/lib/stores/persist.ts   (milik autosave-restore)
import type { Readable } from 'svelte/store';
export type PersistStatus = 'idle' | 'restoring' | 'ready' | 'error';
export const persistStatus: Readable<PersistStatus>;   // awal 'idle'
export const lastSavedAt: Readable<number | null>;     // epoch ms tulis sukses terakhir
export function flushNow(): Promise<void>;             // simpan segera (layout + ui + window state)
// internal (diekspor untuk modul persist, bukan kontrak publik): setPersistStatus(s), markSaved(ts)

// frontend/src/modules/persist/index.ts   (milik autosave-restore; satu baris `import './register'` ditambah saat Wiring)
export function startPersist(): Promise<void>;         // idempoten (panggilan kedua no-op), tidak pernah reject
// index.ts TIDAK me-re-export DeadSession (lihat bagian 7); TerminalGuard (Wiring) mengimpor './DeadSession.svelte' langsung.
```

```ts
// frontend/src/modules/persist/DeadSession.svelte   (milik restart-dead-session)
// Props (Svelte 5 $props):
//   leaf: Leaf                                // daun terminal (modules/layout/model)
//   onRestarted?: (newSessionId: string) => void
// Tidak menyentuh store layout selain mengganti sessionId daun (serialize -> ubah -> hydrate).
```

Pemetaan dokumen/IPC di frontend:
- Simpan layout: `call('workspace.layout.set', {workspaceId, groupId, layout: layout.serialize()})`, `groupId` = `activeGroupId` workspace dari data `workspaces` (atau dari `workspace.layout.get` terakhir); state sidebar + workspace aktif: `call('workspace.ui.set', {leftSidebar, rightSidebar, activeWorkspaceId})`.
- Window: `window.go.main.Bridge.WindowSaveState()` (metode Go baru di `app/window.go`, bagian 2.5) dipanggil dari autosave-restore, dibungkus `try/catch`, debounce 500 ms pada event `resize` jendela dan saat `flushNow()`; gagal diabaikan.
- Dokumen rusak/versi tak dikenal: `layout.hydrate` sudah menghasilkan dokumen kosong; modul persist TIDAK menimpa data tersimpan sampai pengguna mengubah sesuatu (set `lastSavedJSON` dari data yang dimuat; lewati simpan sampai ada perubahan nyata dibanding dokumen awal pasca-hydrate).
- Deteksi sesi mati (dipakai wrapper di Wiring dan dapat dipakai `DeadSession` host): daun `kind==='terminal'` dianggap mati bila `persistStatus === 'ready'` dan (`sessionId` kosong, atau `sessionId` tidak ada di store `sessions`, atau sesinya `status === 'exited'`). Store `sessions` hanya memuat sesi workspace aktif; itu cukup karena layout hanya memuat daun workspace aktif. Tidak ada pendengar event baru dan tidak ada polling.

### 2.5 Kontrak Go `app/window.go` (milik window-state, `package main`)

```go
// Pendaftaran restore saat start tanpa mengubah main.go/bridge.go:
func init() { registerStartupHook(func(ctx context.Context, b *Bridge) { restoreWindow(ctx, b) }) }

func restoreWindow(ctx context.Context, b *Bridge)   // baca appconfig, validasi terhadap runtime.ScreenGetAll, terapkan
func saveWindow(ctx context.Context, b *Bridge)      // simpan x,y,w,h normal lewat appconfig Patch({"window": {...}}); lewati bila maximised/fullscreen; min 800x500
func (b *Bridge) WindowSaveState() error             // metode ter-bind; dipanggil frontend (debounce 500 ms di sisi frontend); memanggil saveWindow dengan ctx Bridge
func onBeforeCloseWindow(ctx context.Context, b *Bridge) bool  // memanggil saveWindow, mengembalikan false (jangan cegah tutup)
```

`app/main.go` hanya menambah satu baris di `options.App`: `OnBeforeClose: func(ctx context.Context) bool { return onBeforeCloseWindow(ctx, bridge) },` (dikerjakan agen Wiring, bukan window-state).

### 2.6 Konvensi penamaan dan lokasi

- Slug fitur = nama berkas plan: `schema-v2-daemon.md`, `autosave-restore.md`, `restart-dead-session.md`, `window-state.md`, `wiring.md` di folder plan ini.
- Go: identifier Inggris; komentar dan pesan error boleh Indonesia mengikuti gaya repo. Svelte: runes (`$props`, `$state`, `$derived`), TypeScript.
- Event daemon: `workspace.layoutChanged`; method `workspace.layout.get/set`, `workspace.ui.get/set`. Tidak ada method `workspace.group.*` (ticket-07).
- Tulis ke disk hanya lewat `writeJSONAtomic`.
- Penanda hasil final di kode mengikuti pola repo bila relevan (`// FINAL(<slug>)` opsional).

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `schema-v2-daemon` | Skema v2 (grup + layout + ui), migrasi v1->v2 dengan cadangan, `Store.GetLayout/SetLayout/GetUI/SetUI`, method IPC dan event `workspace.layoutChanged`, perluasan `workspace.list` dan `workspace.delete` | `internal/workspace/`, `cmd/shikigamid/wire_workspace.go` |
| `autosave-restore` | `modules/persist` (autosave debounce 500 ms, `startPersist` dengan 7 langkah pemulihan, ganti workspace dengan flush), store `persist.ts`, `flushNow`, panggilan debounce `WindowSaveState`, `index.ts` modul | `frontend/src/modules/persist/` (kecuali `DeadSession.svelte`; `register.ts` dan `TerminalGuard.svelte` dikecualikan: milik Wiring), `frontend/src/lib/stores/persist.ts` |
| `restart-dead-session` | Overlay `DeadSession.svelte` ("Sesi berakhir", info project/provider, tombol "Mulai ulang" via `session.create`, ganti `sessionId` daun, status "Memulai...", error, "Project tidak ditemukan") | `frontend/src/modules/persist/DeadSession.svelte` |
| `window-state` | Simpan/pulihkan posisi + ukuran jendela via appconfig, validasi layar, debounce dari frontend, `OnBeforeClose` helper | `app/window.go` |

Rincian perilaku dan acceptance criteria tiap fitur: lihat tiket (bagian Fitur). Jangan menambah atau menggabung fitur.

Catatan khusus per fitur (turunan kontrak di atas):
- `schema-v2-daemon`: fix konstanta `fileVersion` bersama (bagian 2.2). `Delete` workspace aktif membersihkan `ui.activeWorkspaceID` dan menulis `ui` bersama daftar workspace dalam satu tulis atomik. `SetUI` memvalidasi `activeWorkspaceId` (kosong boleh). `persistWorkspacesLocked` menulis `workspacesDoc{Version:2, Workspaces, UI}`; semua metode mutasi yang ada harus tetap menulis blok `ui` saat ini. Tidak memakai `go test`.
- `autosave-restore`: bergantung pada kontrak IPC bagian 2.3 (boleh memakai mock sampai daemon siap). Deteksi perubahan hanya lewat `layout.subscribe`, `leftSidebar.subscribe`, `rightSidebar.subscribe`, `activeWorkspaceId.subscribe`; bandingkan string JSON terakhir tersimpan. Jangan membuat instance xterm; pemulihan terminal sepenuhnya oleh `hydrate` + modul terminal yang sudah ada. Abaikan event `workspace.layoutChanged` milik tulisan sendiri (lacak penanda tulisan terakhir, mis. timestamp/hash) dan tidak memuat ulang karena `workspace.changed`.
- `restart-dead-session`: `session.create` params `{workspaceId: <aktif>, projectId: leaf.projectId, cwd: <path project dari store workspaces>, providerId: leaf.provider}`; tidak ada `env`/argumen tambahan. Mengganti `sessionId` dengan pola serialize->ubah->hydrate, lalu menambahkan sesi baru ke store `sessions` (`sessions.update`, hindari duplikat) agar `TerminalTab` dan header menemukannya; panggil `onRestarted(newId)`. Provider `shell` didukung. Komponen mandiri dan tidak bergantung pada fitur lain.
- `window-state`: lihat bagian 2.5. Validasi layar: persegi window harus beririsan dengan salah satu layar dari `runtime.ScreenGetAll`; jika tidak, pakai 1200x800 dan tengahkan (`runtime.WindowCenter`). Patch appconfig hanya `{"window": {x,y,w,h}}` (jangan menyentuh `zoom`). Tanpa goroutine timer polling; debounce ada di frontend dan tidak diulang di Go.

## 4. Foundation

Tidak ada. Tidak ada skeleton paket baru atau tipe bersama yang harus ada sebelum paralel: tipe Go ada di path `schema-v2-daemon`, tipe TS memakai tipe tiket-03/04 yang sudah ada, dan antarmuka antar fitur hanya prop komponen + kontrak IPC (bagian 2). `00-foundation.md` tidak ditulis.

## 5. Wiring checklist (agen Wiring, setelah keempat fitur selesai dan ter-merge)

Boleh menyentuh HANYA: `frontend/src/modules/persist/register.ts` (baru), `frontend/src/modules/persist/TerminalGuard.svelte` (baru), `frontend/src/modules/persist/index.ts` (tambah satu baris `import './register';`), `app/main.go` (satu baris `OnBeforeClose`). Baca nama sebenarnya dari kode hasil fitur sebelum menulis.

1. **Daemon**: pastikan `cmd/shikigamid/wire_workspace.go` memanggil `workspace.Register(srv, store)` versi final (tanda tangan berubah => sesuaikan di sana; itu path milik `schema-v2-daemon`, jadi hanya periksa, jangan ubah kecuali build gagal akibat tanda tangan). Jalankan `shikigamid` terhadap `workspaces.json` v1 contoh di data dir sementara (`SHIKIGAMI_*`/`paths.DataDir` sesuai kode) dan cek: file jadi `version: 2`, ada `workspaces.json.v1.bak`, grup "Utama", `workspace.layout.get/set`, `workspace.ui.get/set` lewat `shikigamictl`/klien IPC yang ada, tanpa menambah file di luar path milik.
2. **Mulai pemulihan saat app siap**: `modules/persist/register.ts` berlangganan store `connection` (`lib/stores/init.ts`); pada transisi pertama ke `state === 'ok'` panggil `startPersist()` satu kali (di dalam `try/catch`, tidak pernah melempar), lalu berhenti berlangganan. Tambahkan satu baris `import './register';` di `modules/persist/index.ts` (dimuat otomatis oleh `main.ts` glob; tidak mengedit `main.ts`/`App.svelte`). Bila `startPersist` ternyata butuh store shell/layout terinisialisasi lebih dulu, itu sudah terpenuhi karena keduanya singleton modul.
3. **Overlay sesi mati**: `TerminalGuard.svelte` (props `{pane: Leaf; visible: boolean}`) membungkus `TerminalPane.svelte` dari `modules/layout/panes/` dan menampilkan `DeadSession` (`leaf={pane}`) di atasnya (posisi absolut menutupi pane) bila daun dianggap mati (aturan bagian 2.4). `register.ts` mendaftarkan ulang jenis `terminal`: ambil spec asli dengan `getPaneKind('terminal')` (pastikan `import '../layout/panes/terminal'` sudah dievaluasi sebelum membaca, atau tunggu lewat microtask/`paneKinds()` subscribe sampai ada), lalu `registerPaneKind({ ...asli, component: TerminalGuard })`. Jangan mengubah isi `modules/layout` atau `modules/terminal`. Pastikan `TerminalTab` tidak dibuat dua kali dan tidak dipasang bila daun mati (hindari attach ke sesi yang tak ada; guard merender `DeadSession` saja dan tidak merender `TerminalPane` saat mati; setelah "Mulai ulang" `sessionId` berganti dan guard merender `TerminalPane` baru).
4. **Window**: tambah satu baris di `app/main.go` pada `options.App`: `OnBeforeClose: func(ctx context.Context) bool { return onBeforeCloseWindow(ctx, bridge) },`. Restore start sudah terpasang oleh `init()` di `app/window.go` lewat `registerStartupHook`; verifikasi bahwa hook tidak bentrok dengan hook lain.
5. **Uji manual end-to-end** (`make build`, jalankan app): buat 2 workspace, split beberapa terminal, buka satu file, ubah lebar sidebar, geser/ubah ukuran jendela; tutup app; buka lagi; periksa layout, rasio, fokus, maksimalkan, sidebar kiri/kanan (tampil, lebar, tab), workspace aktif, jendela, sesi hidup ter-attach dengan scrollback, file terbuka lagi. Lalu hentikan paksa `shikigamid`, buka ulang app tanpa sesi: pane menampilkan "Sesi berakhir"; "Mulai ulang" membuat sesi baru di pane yang sama. Pindah workspace dan pastikan layout masing-masing independen.
6. **Guard izin agent**: pastikan "Mulai ulang" memakai `session.create` biasa (tanpa argumen/flag tambahan dari frontend); adapter provider yang menambah flag.
7. Jalankan `make build`; pastikan app tetap berjalan.

## 6. Build guard

Satu-satunya guard: `make build` (setara `cd frontend && npm install && npm run build`, `go build` untuk `shikigamid`/`shikigamictl`, dan `go build -tags desktop,production ./app` dengan `CGO_LDFLAGS="-framework UniformTypeIdentifiers"`). Untuk fitur Go murni boleh memakai `go build ./...` (`make check`) dan untuk fitur frontend `cd frontend && npm run build` saat bekerja di worktree. Tanpa unit test. Aplikasi harus tetap bisa dijalankan setelah build.

## 7. Aturan paralel

- Tiap agen hanya menyentuh `ownedPaths` fiturnya (bagian 3). Jangan menyentuh path fitur lain, file milik tiket lain (`app/main.go`, `app/bridge.go`, `internal/appconfig`, `main.ts`, `App.svelte`, `modules/layout`, `modules/shell`, `modules/terminal`, `lib/stores/layout.ts`, `lib/stores/shell.ts`, `lib/stores/sessions.ts`), maupun file wiring (`register.ts`, `TerminalGuard.svelte`, `modules/persist/index.ts` baris import register, `OnBeforeClose` di `main.ts` app).
- `modules/persist/` dibagi: `DeadSession.svelte` milik `restart-dead-session`; sisanya (`index.ts`, `persist.ts`/modul lain di folder itu) milik `autosave-restore`. `autosave-restore` MEMBUAT `index.ts` yang hanya mengekspor `startPersist`; ia tidak boleh membuat atau mengimpor `DeadSession.svelte` (agen `restart-dead-session` yang membuatnya). Penggunaan `DeadSession` dilakukan langsung dari `TerminalGuard.svelte` di Wiring.
- Tanpa dependency baru, tanpa polling (hanya subscribe/event dan timer debounce), tanpa mengubah modul pemilik lain selain lewat fungsi publik yang sudah ada.
- Berkomunikasi antar fitur hanya lewat kontrak bagian 2; bila kontrak tampak kurang, ikuti kontrak dan catat asumsinya di akhir plan fitur, jangan mengubah path lain.
