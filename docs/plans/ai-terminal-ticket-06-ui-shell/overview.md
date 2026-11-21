# Overview: Epic 06 UI Shell

Sumber: tiket `docs/tickets/ai-terminal-ticket-06-ui-shell.md` dan spec `docs/superpowers/specs/2026-11-21-ai-terminal-design.md`. Tiket adalah sumber kebenaran; bila overview ini bertentangan dengan tiket, tiket menang (kecuali bagian "Keputusan" di 1.2 yang memang melengkapi celah tiket).

## 1. Ringkasan epic dan urutan eksekusi

Epic ini menyempurnakan UI Wails: layout sidebar / tab utama / statusbar dari registry panel, store state berbasis event IPC, sidebar workspace + project + sesi, banyak tab sesi (memakai `TerminalTab` dari tiket 02), serta auto-start daemon `aitermd` dari app. Anti-goal dijaga: daftar sesi tidak berubah saat pindah project, tanpa polling (`setInterval` dilarang), ringan (tanpa dependency baru).

Urutan eksekusi:

1. **Foundation** (satu agent, `00-foundation.md`): tipe bersama dan store dengan ekspor final (badan stub), plus stub `TabBar.svelte`/`TabContent.svelte` di `modules/tabs/` agar `App.svelte` bisa dikompilasi sebelum `session-tabs` selesai.
2. **Fitur paralel**: `daemon-autostart`, `shell-layout`, `workspace-sidebar`, `session-tabs`.
3. **Wiring** (bagian 5): hampir seluruhnya verifikasi; tidak ada file bersama yang diedit.

### 1.1 Kondisi kode saat ini (hasil membaca repo)

- `frontend/src/lib/registry.ts`: `registerPanel(spec)`, `registerTabType(spec)`, `panels(area): Readable<PanelSpec[]>` (urut `order` lalu `id`), `tabTypes(): Readable<Record<string, TabTypeSpec>>`. `TabTypeSpec.type` bertipe `'session' | 'file'`.
- `frontend/src/lib/ipc.ts`: ekspor `inWails: boolean`, `call<T>(method, params?)` (reject `Error` bila bukan di Wails), `onEvent(topicPrefix, fn(topic, payload)) => unsubscribe`. Tidak ada ekspor bernama `subscribe`; gunakan `onEvent`. Event Wails `daemon:event` `{topic, payload}`.
- `frontend/src/main.ts`: `import.meta.glob('./modules/*/index.ts', { eager: true })` lalu `mount(App)`. Tidak boleh diedit.
- `frontend/src/modules/terminal/` (tiket 02): `index.ts` (mendaftarkan `registerTabType({type:'session', component: TerminalTab})` dan panel `terminal-quick`), `TerminalTab.svelte` (prop `sessionId: string`), `QuickTerminal.svelte`, `terminal-client.ts` (ekspor `SessionInfo`, `listSessions`, `createSession()` tanpa argumen, `killSession`, `subscribeOutput`, `subscribeExit`, `attachSession`, dst.). Epic ini TIDAK memakai `createSession`/`listSessions` dari `terminal-client.ts` (bentuknya terlalu sempit: tanpa workspace/project/provider); store memanggil `call('session.create', ...)` sendiri.
- `frontend/package.json`: sudah memuat `@xterm/xterm` dan `@xterm/addon-fit`; Svelte 5 (rune `$state`, `$props`, `onMount`), Vite 6, `svelte-check` (`npm run check`). `vite.config.ts` tanpa alias; impor memakai path relatif.
- `app/bridge.go` dan `app/main.go`: BELUM ada hook startup (tidak ada `RegisterStartup`); `OnStartup` di `main.go` hanya `bridge.SetContext(ctx)`. `Bridge.Call` TIDAK auto-start daemon; bila socket tidak ada ia mengembalikan error `daemon tidak berjalan di <sock>`.
- `internal/ipc`: `ipc.Dial(socketPath) (*Client, error)`, `(*Client).Call(method, params, &out) error`, `(*Client).Close()`. `internal/paths`: `paths.DataDir()`, `paths.SocketPath()`.
- Daemon (kondisi sekarang): `ping`, `daemon.info`, `session.*` (create/list/kill/write/resize/attach), `fs.*`. `workspace.*` (tiket 04) dan `provider.list` (tiket 05) menyusul; frontend hanya bergantung pada kontrak, bukan pada keberadaan saat build.

### 1.2 Keputusan yang melengkapi tiket

1. **`session.create` butuh `cwd`.** Daemon (`internal/session/ipc.go`) menerima `{workspaceId, projectId, cwd, providerId, cols?, rows?}`. Store `createSession(providerId)` mengirim `cwd = path project aktif`.
2. **Bentuk balasan defensif.** `session.list` mengembalikan `{sessions: Session[] | null}`; `workspace.list` per tiket 04 mengembalikan `Workspace[]`; `workspace.addProject` mengembalikan `{workspace, project}`; `provider.list` mengembalikan array. Store menormalkan: terima array langsung ATAU objek berkunci (`sessions`, `workspaces`, `providers`), `null` menjadi `[]`.
3. **Aksi workspace tambahan.** Selain aksi pada tiket, store menyediakan `createWorkspace(name)`, `renameWorkspace(id, name)`, `deleteWorkspace(id)` (dibutuhkan sidebar; tiket hanya menyebut "workspace.* list, create, rename, delete").
4. **Hook startup Go belum ada.** Sesuai tiket, `app/daemon.go` tidak mengedit `main.go`/`bridge.go`. Karena `RegisterStartup` tidak ada, `daemon.go` menyediakan `EnsureDaemon` dan, sebagai pengganti hook, `init()` memulai satu goroutine sekali jalan `EnsureDaemon(context.Background())` dan menyimpan hasilnya (`DaemonStartupError() error`). Ini bukan polling; hanya penantian startup satu kali. Bila UI melakukan `ping` lebih awal dan gagal, layar error + "Coba lagi" menyelamatkan. Bila suatu saat `app/bridge.go` menyediakan hook, ganti ke hook itu. Laporkan kekurangan hook di laporan akhir.
5. **Dialog pilih folder.** Runtime JS Wails v2 tidak mengekspos `OpenDirectoryDialog` (hanya sisi Go) dan `bridge.go` tidak boleh diedit. Maka `workspace-sidebar` memakai input path teks sebagai jalur utama, dengan deteksi opsional (`window.go?.main?.Bridge?.OpenDirectoryDialog`, atau `window.runtime?.OpenDirectoryDialog`) bila suatu saat tersedia. Jangan menambah binding Go.
6. **Aksi store melempar `Error`** (`throw`) dengan pesan dari daemon; pemanggil UI (sidebar) menangkap dan menampilkan pesan singkat. Store tidak menelan error aksi pengguna.

## 2. Kontrak bersama

### 2.1 Struktur folder

```
app/daemon.go                                  (daemon-autostart)
frontend/src/App.svelte                        (shell-layout)
frontend/src/styles/{theme.css,base.css}       (shell-layout)
frontend/src/lib/stores/                       (Foundation membuat; shell-layout memiliki isinya)
    types.ts  workspaces.ts  sessions.ts  tabs.ts  providers.ts  init.ts  index.ts
frontend/src/modules/shell/{SidebarArea.svelte,StatusBar.svelte}   (shell-layout)
frontend/src/modules/tabs/{TabBar.svelte,TabContent.svelte,...}    (session-tabs; stub awal dari Foundation)
frontend/src/modules/workspace/{index.ts,WorkspaceSidebar.svelte,...} (workspace-sidebar)
frontend/src/modules/terminal/{index.ts,QuickTerminal.svelte}      (hanya session-tabs: edit index.ts, hapus QuickTerminal)
```

Semua impor memakai path relatif dari file masing-masing, mis. dari `modules/workspace/X.svelte`: `../../lib/stores`, `../../lib/ipc`, `../../lib/registry`. Impor store SELALU lewat barrel `lib/stores/index.ts` (`import { sessions, createSession } from '../../lib/stores'`).

### 2.2 Tipe (`lib/stores/types.ts`, final dari Foundation)

```ts
export interface Project { id: string; path: string; name: string }
export interface WorkspaceConfig { grimoireUrl?: string; grimoireKey?: string }
export interface Workspace { id: string; name: string; projects: Project[]; config: WorkspaceConfig }

export type SessionStatus = 'running' | 'exited';
export interface Session {
  id: string; workspaceId: string; projectId: string; cwd: string;
  providerId: string; status: SessionStatus; createdAt: string; exitCode?: number;
}

export interface Provider {
  id: string; name: string;
  capabilities?: { hooks?: boolean; mcp?: boolean; idleEvent?: boolean };
  available?: boolean; path?: string; error?: string;
}

export type TabType = 'session' | 'file';
export interface Tab { id: string; type: TabType; title: string; refId: string }
```

ID tab: `` `${type}:${refId}` `` (stabil, dipakai untuk dedup).

### 2.3 Store (`lib/stores/`, ekspor final; semua dari barrel `index.ts`)

Svelte store klasik (`writable`/`derived` dari `svelte/store`), dipakai di komponen dengan `$store`.

```ts
// workspaces.ts
export const workspaces: Writable<Workspace[]>;
export const activeWorkspaceId: Writable<string | null>;
export const activeProjectId: Writable<string | null>;       // hanya menentukan cwd sesi baru
export const activeWorkspace: Readable<Workspace | null>;
export const activeProject: Readable<Project | null>;
export async function loadWorkspaces(): Promise<void>;       // workspace.list; pilih aktif bila belum/ hilang
export async function selectWorkspace(id: string): Promise<void>;  // set aktif, project aktif = project pertama, loadSessions()
export function selectProject(id: string): void;             // HANYA set activeProjectId
export async function createWorkspace(name: string): Promise<Workspace>;
export async function renameWorkspace(id: string, name: string): Promise<void>;
export async function deleteWorkspace(id: string): Promise<void>;
export async function addProject(path: string, name?: string): Promise<Project>; // workspace aktif
export async function removeProject(id: string): Promise<void>;

// sessions.ts
export const sessions: Writable<Session[]>;                  // milik workspace aktif, BUKAN project
export async function loadSessions(): Promise<void>;         // session.list {workspaceId}; filter klien workspaceId juga
export async function createSession(providerId: string): Promise<Session>; // workspace+project aktif, cwd=project.path; menambah ke store; TIDAK membuka tab
export async function killSession(id: string): Promise<void>;

// tabs.ts
export const tabs: Writable<Tab[]>;
export const activeTabId: Writable<string | null>;
export function openTab(t: { type: TabType; refId: string; title?: string }): Tab; // dedup by id, aktifkan
export function activateTab(id: string): void;
export function closeTab(id: string): void;                  // tidak membunuh sesi; aktifkan tetangga

// providers.ts
export const providers: Writable<Provider[]>;
export async function loadProviders(): Promise<void>;

// init.ts
export const connection: Writable<{ state: 'connecting' | 'ok' | 'error'; message: string }>;
export async function initStores(): Promise<() => void>;     // ping -> loadWorkspaces -> loadProviders -> loadSessions; subscribe event; kembalikan fungsi dispose
```

Aturan yang mengikat semua fitur:

- `sessions` hanya diubah oleh `loadSessions`, `createSession`, `killSession`, event IPC, dan `selectWorkspace`/`deleteWorkspace` (ganti workspace). `selectProject`/`activeProjectId` TIDAK boleh menyentuh `sessions` (anti-goal 2). Tidak ada `$effect` yang memuat ulang sesi saat `activeProjectId` berubah.
- Urutan `sessions`: urutan dari daemon (pembuatan), sesi baru ditambahkan di akhir. Tidak diurutkan ulang menurut project.
- Event IPC (lewat `onEvent` dari `lib/ipc.ts`, dipasang SEKALI oleh `initStores`): `session.exit` (payload `{id, exitCode}`) -> set `status='exited'`, `exitCode` pada sesi itu; `workspace.changed` (bila diterbitkan tiket 04) -> `loadWorkspaces()`. Aksi sendiri memperbarui store langsung dari balasan; bila event tidak ada, tidak ada polling.
- Tab sesi: `openTab({type:'session', refId: sessionId, title})`. Penampil tab menurunkan judul tampilan dari `sessions` + `providers` + `activeWorkspace.projects` (fallback `tab.title`, lalu `refId`).

### 2.4 Komponen lintas fitur

- `modules/tabs/TabBar.svelte` dan `modules/tabs/TabContent.svelte`: default export Svelte, TANPA prop; membaca store `tabs`, `activeTabId`, `sessions`, `providers`, `activeWorkspace` dan registry `tabTypes()` sendiri. Diimpor `App.svelte` sebagai `./modules/tabs/TabBar.svelte` dan `./modules/tabs/TabContent.svelte`.
- `modules/shell/SidebarArea.svelte` dan `StatusBar.svelte`: tanpa prop; membaca `panels('sidebar')` / `panels('statusbar')` dari registry dan merender `<panel.component />` berurut.
- Panel sidebar workspace: komponen tanpa prop, didaftarkan `registerPanel({id:'workspace-sidebar', area:'sidebar', component, order:10})`.
- `TerminalTab` dirender dengan `<spec.component sessionId={tab.refId} />`; komponen tab `file` (tiket 07) kelak menerima `refId` sebagai prop `path`-nya sendiri, jadi `TabContent` meneruskan `{...{ sessionId: refId, refId, tabId }}` ke komponen tab (komponen mengabaikan prop yang tak dikenal; di Svelte 5 prop ekstra tidak error).
- Konten tab aktif di-mount dengan blok `{#key activeTab.id}` sehingga hanya tab aktif yang punya instance terminal.

### 2.5 CSS variables dan tema (`styles/theme.css`, milik shell-layout)

Variabel minimum (terang di `:root`, gelap di `@media (prefers-color-scheme: dark)`):
`--bg`, `--bg-elev`, `--bg-hover`, `--fg`, `--fg-muted`, `--border`, `--accent`, `--danger`, `--ok`, `--term-bg`, `--term-fg`, `--term-cursor`, `--sidebar-w` (mis. 260px), `--tabbar-h` (mis. 34px), `--statusbar-h` (mis. 24px). Fitur lain boleh MEMAKAI variabel ini (dengan fallback `var(--x, #fallback)`) tetapi tidak mendefinisikannya. Gaya komponen lokal (`<style>` di komponen) untuk hal spesifik; tanpa framework CSS.

### 2.6 Go (`app/daemon.go`, package `main`)

```go
func EnsureDaemon(ctx context.Context) error     // ping; bila gagal spawn aitermd terlepas, tunggu ping (timeout 5 dtk, interval 100 ms)
func DaemonStartupError() error                  // hasil EnsureDaemon dari init(); nil bila sukses atau belum selesai
```

Detail: `exec.Cmd` dengan `SysProcAttr{Setsid: true}`, stdio ke `<DataDir>/aitermd.log` (append, 0600; fallback `os.DevNull`), `cmd.Process.Release()`; cari biner: sebelah `os.Executable()` (dan `../Resources`/`../MacOS` dalam bundle `.app`), lalu `exec.LookPath("aitermd")`; socket basi dibiarkan (daemon baru yang membersihkan); tidak pernah mengirim sinyal ke daemon. Paket yang boleh diimpor: stdlib, `aiterm/internal/ipc`, `aiterm/internal/paths`. Tanpa dependency baru.

### 2.7 Konvensi penamaan

Identifier kode, nama file, dan nama store dalam bahasa Inggris; teks UI dan komentar boleh bahasa Indonesia. Komponen Svelte PascalCase, file TS camelCase/kebab. Teks UI yang harus persis: "Coba lagi", "Tambah project dulu", "Sesi baru", "Tambah project", "tipe tab tidak tersedia".

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `daemon-autostart` | `EnsureDaemon` + `DaemonStartupError`, spawn `aitermd` terlepas (setsid, release), cari biner, log ke `<DataDir>/aitermd.log`, tunggu ping 5 dtk/100 ms, pendaftaran lewat `init()` (goroutine satu kali karena hook startup bridge belum ada). | `app/daemon.go` |
| `shell-layout` | `App.svelte` (tiga area, layar error koneksi + "Coba lagi"), `SidebarArea`/`StatusBar`, isi final semua store di `lib/stores/` + `initStores` + event, `styles/` (tema gelap/terang, `--term-*`). | `frontend/src/App.svelte`, `frontend/src/modules/shell/`, `frontend/src/lib/stores/`, `frontend/src/styles/` |
| `workspace-sidebar` | Panel sidebar: pemilih workspace (buat/ganti nama/hapus), project (tambah via input path/dialog bila ada, hapus, penanda aktif), daftar sesi workspace aktif (klik buka tab, kill), tombol "Sesi baru" dengan pilihan provider, error singkat; `index.ts` mendaftarkan panel. | `frontend/src/modules/workspace/` |
| `session-tabs` | `TabBar` + `TabContent` (banyak tab, hanya tab aktif di-mount, tutup tab tidak kill sesi, penanda exit, placeholder tipe tak dikenal), serta pembersihan `terminal-quick` (edit `modules/terminal/index.ts`, hapus `QuickTerminal.svelte`). | `frontend/src/modules/tabs/`, plus sentuhan terbatas: `frontend/src/modules/terminal/index.ts`, `frontend/src/modules/terminal/QuickTerminal.svelte` |

Tidak ada fitur yang ditambah/digabung dari tiket. Catatan kepemilikan: Foundation menulis versi awal `lib/stores/*` dan `modules/tabs/{TabBar,TabContent}.svelte`; kepemilikan resminya tetap pada `shell-layout` dan `session-tabs`, yang MENGGANTI isinya (bukan membuat file baru dengan nama lain).

## 4. Foundation

`hasFoundation=true`. Rincian di `00-foundation.md`. Isi: `lib/stores/types.ts` final; `workspaces.ts`, `sessions.ts`, `tabs.ts`, `providers.ts`, `init.ts`, `index.ts` dengan ekspor final (bagian 2.3) dan badan minimal; stub `TabBar.svelte`/`TabContent.svelte`. Tujuan: `workspace-sidebar` dan `session-tabs` dapat mengimpor store tanpa menunggu `shell-layout`, dan `shell-layout` dapat mengimpor tabs tanpa menunggu `session-tabs`; `npm run build` tetap hijau pada setiap tahap.

## 5. Wiring checklist

Tidak ada file bersama yang diedit (`main.ts`, `registry.ts`, `ipc.ts`, `package.json`, `app/main.go`, `app/bridge.go` tidak disentuh). Agen wiring memverifikasi dan menambal seminimal mungkin dalam ownedPaths:

1. `modules/workspace/index.ts` memanggil `registerPanel({id:'workspace-sidebar', area:'sidebar', component, order:10})`; ditemukan oleh glob `./modules/*/index.ts` di `main.ts`.
2. `modules/terminal/index.ts` hanya berisi `registerTabType({type:'session', component: TerminalTab})`; tidak ada `terminal-quick`; `QuickTerminal.svelte` tidak ada (`grep -rn "terminal-quick\|QuickTerminal" frontend/src` kosong). `TerminalTab.svelte` dan `terminal-client.ts` tidak berubah.
3. `App.svelte` mengimpor `TabBar`/`TabContent` dari `./modules/tabs/` dan `SidebarArea`/`StatusBar` dari `./modules/shell/`; memanggil `initStores()` saat mount dan memanggil dispose saat destroy; tidak ada teks "Hello World".
4. Tidak ada sisa stub: `grep -rn "STUB" frontend/src app/daemon.go` kosong; tidak ada `setInterval`/`setTimeout` berulang di `frontend/src/{App.svelte,lib/stores,modules/shell,modules/tabs,modules/workspace}` (satu `setTimeout` untuk menghilangkan toast boleh; `setInterval` dilarang).
5. `grep -rn "@xterm" frontend/src/modules/tabs frontend/src/modules/workspace frontend/src/modules/shell` kosong.
6. `--term-bg`, `--term-fg`, `--term-cursor` ada di `styles/theme.css` untuk terang dan gelap.
7. `app/daemon.go`: `EnsureDaemon` ada; tidak ada impor di luar stdlib + `internal/ipc` + `internal/paths`; `app/main.go` dan `app/bridge.go` tak berubah. Catat di laporan: hook `RegisterStartup` tidak ada di `bridge.go` (kekurangan tiket 01), dan dialog folder native tidak tersedia dari JS (input path dipakai).
8. `package.json`/`package-lock.json` tak berubah dibanding sebelum epic.
9. Jalankan build guard (bagian 6). Uji manual opsional: `make build && open build/bin/aiterm` (atau jalankan `./build/bin/aiterm`; daemon harus menyala otomatis).

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- Go: `go build ./...` (mencakup `app/daemon.go`; paket `app` memakai `//go:embed all:dist` sehingga `app/dist` harus ada, `.gitkeep` cukup) dan `gofmt -l app` (kosong).
- Frontend: `cd frontend && npm run build` (`vite build`), disarankan juga `npm run check` (`svelte-check`; error tipe pada file milik fitur sendiri harus nol).
- Penuh (tahap Wiring/akhir): `make build`.
- Per fitur: fitur Go cukup `go build ./...`; fitur frontend cukup `cd frontend && npm run build` (+ `npm run check`). Build harus hijau walaupun fitur lain belum selesai, berkat stub Foundation.

## 7. Aturan paralel

- Tiap agent hanya membuat/mengubah `ownedPaths` fiturnya. Dilarang menyentuh path fitur lain dan file wiring/bersama: `frontend/src/lib/registry.ts`, `frontend/src/lib/ipc.ts`, `frontend/src/main.ts`, `frontend/package.json`, `frontend/package-lock.json`, `frontend/vite.config.ts`, `app/main.go`, `app/bridge.go`, `Makefile`, `go.mod`, `go.sum`, semua `internal/*`, `cmd/*`, dan `modules/terminal/{TerminalTab.svelte,terminal-client.ts}`.
- Patuhi nama ekspor store, nama komponen, dan teks UI pada bagian 2 persis. Jangan menambah ekspor yang diwajibkan fitur lain; boleh menambah ekspor pelengkap di file milik sendiri.
- `workspace-sidebar` dan `session-tabs` hanya MEMBACA/memanggil store lewat barrel; mereka tidak mengedit `lib/stores/`. Bila butuh aksi/ekspor yang kurang, pakai `call` dari `lib/ipc.ts` langsung di komponen dan catat di laporan, jangan edit store.
- Tanpa dependency baru (npm maupun Go), tanpa `setInterval`/polling, tanpa unit test, tanpa impor `@xterm/*` di luar `modules/terminal/`.
- Svelte 5: gunakan rune (`$state`, `$derived`, `$props`, `$effect`) atau `$store` untuk store klasik; konsisten dengan `App.svelte` lama.
