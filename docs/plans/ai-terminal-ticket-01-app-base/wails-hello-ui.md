# Plan: Wails app dan frontend Hello World

## Metadata

- Tiket: `docs/tickets/ai-terminal-ticket-01-app-base.md` (epic 01 App Base, fitur `wails-hello-ui`)
- Desain bersama: `docs/plans/ai-terminal-ticket-01-app-base/overview.md` (baca untuk kontrak; jangan diubah)
- Fitur: `wails-hello-ui`
- Depends on foundation: **ya** (`00-foundation.md`: `go.mod` + Wails di `go.sum`, stub `internal/paths` dan `internal/ipc`, `app/dist/.gitkeep`). Fitur ini dikerjakan terhadap signature Kontrak, bukan isi stub.
- ownedPaths:
  - `wails.json`
  - `app/main.go`
  - `app/bridge.go`
  - `frontend/package.json`
  - `frontend/vite.config.ts`
  - `frontend/tsconfig.json`
  - `frontend/index.html`
  - `frontend/src/main.ts`
  - `frontend/src/App.svelte`
  - `frontend/src/lib/registry.ts`
  - `frontend/src/lib/ipc.ts`
- File pendukung yang boleh dibuat di `frontend/`: `package-lock.json` (hasil `npm install`), `src/env.d.ts`, `svelte.config.js`.

## Tujuan

Membuat aplikasi Wails (window "AI Terminal" 1200x800) dengan frontend Svelte 5 + Vite + TS yang menampilkan "Hello World", memanggil `ping` ke daemon lewat `Bridge`, dan menampilkan hasilnya (atau pesan "daemon tidak berjalan" tanpa crash) beserta jumlah panel terdaftar per area. Fitur ini juga menyediakan titik ekstensi frontend (`registry.ts`, `ipc.ts`, glob modul di `main.ts`) yang dipakai epic lain tanpa mengedit file ini. Wails CLI tidak dibutuhkan; build memakai `go build -tags desktop,production` (dijalankan target Makefile milik fitur lain).

## Path yang dimiliki

HANYA path di bagian Metadata (ownedPaths + 3 file pendukung di `frontend/`) yang boleh disentuh. Dilarang:
- Menyentuh `go.mod`, `go.sum`, `Makefile`, `.gitignore`, `README.md`, `scripts/`, `internal/`, `cmd/`, `app/dist/.gitkeep` (fitur lain). Dilarang `go mod tidy` / `go get`.
- Membuat `app/daemon.go`, `frontend/src/modules/`, `wire_*.go`, `cmd_*.go` (epic lain).
- Menaruh logika yang dipakai modul lain di `App.svelte` (taruh di `lib/`).
- Mengimpor folder `wailsjs/` hasil generate.
- Menyalin `frontend/dist` ke `app/dist` secara permanen: itu tugas target `build-app`. Untuk guard lokal boleh disalin sementara, lalu dibersihkan (lihat Langkah 12).

## Kontrak

Rujuk overview bagian 2 dan tiket bagian "Kontrak" butir 5 dan 6. Ringkasan konkret yang harus persis:

### Dipakai dari `aiterm/internal/ipc` (hanya signature ini) dan `aiterm/internal/paths`
```go
func Dial(socketPath string) (*ipc.Client, error)
func (c *Client) Call(method string, params any, out any) error // error server bertipe *ipc.Error
func (c *Client) Subscribe(topicPrefix string, fn func(topic string, payload json.RawMessage)) (unsubscribe func(), err error)
func (c *Client) Close() error
func paths.SocketPath() (string, error)
```
Dari Wails: `github.com/wailsapp/wails/v2`, `.../pkg/options`, `.../pkg/options/assetserver`, `.../pkg/runtime` (`runtime.EventsEmit`).

### Disediakan: Go (`package main` di `app/`)
```go
type Bridge struct { /* mu sync.Mutex; ctx context.Context; client *ipc.Client; unsub func() */ }
func NewBridge() *Bridge
func (b *Bridge) SetContext(ctx context.Context)                                      // dipanggil OnStartup
func (b *Bridge) Call(method string, params json.RawMessage) (json.RawMessage, error) // method ter-bind ke Wails: window.go.main.Bridge.Call
func (b *Bridge) Close() error                                                        // dipanggil OnShutdown
```
Event Wails ke frontend: nama `daemon:event`, data `{"topic": string, "payload": any}`.

### Disediakan: TypeScript
`frontend/src/lib/registry.ts` (persis tiket butir 5):
```ts
export type PanelArea = 'sidebar' | 'main-tab' | 'statusbar';
export interface PanelSpec { id: string; area: PanelArea; component: Component<any>; order?: number; }
export interface TabTypeSpec { type: 'session' | 'file'; component: Component<any>; }
export function registerPanel(spec: PanelSpec): void;
export function registerTabType(spec: TabTypeSpec): void;
export function panels(area: PanelArea): Readable<PanelSpec[]>;
export function tabTypes(): Readable<Record<string, TabTypeSpec>>;
```
`frontend/src/lib/ipc.ts` (persis tiket butir 6):
```ts
export function call<T = unknown>(method: string, params?: unknown): Promise<T>;
export function onEvent(topicPrefix: string, fn: (topic: string, payload: unknown) => void): () => void;
export const inWails: boolean;
```

## Langkah

Working directory: root repo `/Users/ascendiz/Documents/Projects/noobeeid/research/ai-terminal`. Tulis file dengan Write tool. Teks UI dan pesan galat bahasa Indonesia; identifier bahasa Inggris. Tanpa timer polling.

### 1. `wails.json`
```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "aiterm",
  "outputfilename": "aiterm",
  "frontend:dir": "frontend",
  "frontend:install": "npm install",
  "frontend:build": "npm run build"
}
```

### 2. `frontend/package.json`
`name: "aiterm-frontend"`, `private: true`, `version: "0.0.0"`, `type: "module"`.
- scripts: `"dev": "vite"`, `"build": "vite build"`, `"check": "svelte-check --tsconfig ./tsconfig.json"`.
- devDependencies: `svelte ^5.0.0`, `vite ^6.0.0`, `@sveltejs/vite-plugin-svelte ^5.0.0`, `typescript ^5.6.0`, `svelte-check ^4.0.0`, `tslib ^2.8.0`.
Tanpa dependensi runtime (store memakai `svelte/store`). Bila `npm install` menolak kombinasi versi (peer dependency), sesuaikan rentang versi sampai `npm install` bersih; jangan memakai `--force`.

### 3. `frontend/vite.config.ts`
```ts
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
export default defineConfig({ base: './', plugins: [svelte()], build: { outDir: 'dist', emptyOutDir: true } });
```

### 4. `frontend/svelte.config.js`
```js
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
export default { preprocess: vitePreprocess() };
```

### 5. `frontend/tsconfig.json` dan `frontend/src/env.d.ts`
tsconfig (mandiri, tanpa `extends`): `target ESNext`, `module ESNext`, `moduleResolution bundler`, `strict true`, `verbatimModuleSyntax true`, `isolatedModules true`, `skipLibCheck true`, `noEmit true`, `lib ["ESNext","DOM","DOM.Iterable"]`, `types ["svelte","vite/client"]`; `include ["src/**/*.ts","src/**/*.svelte","src/**/*.d.ts"]`.
`src/env.d.ts`:
```ts
/// <reference types="svelte" />
/// <reference types="vite/client" />
```

### 6. `frontend/index.html`
HTML5 standar, `lang="id"`, meta charset dan viewport, `<title>AI Terminal</title>`, `<div id="app"></div>`, `<script type="module" src="/src/main.ts"></script>`.

### 7. `frontend/src/lib/registry.ts`
Berbasis `svelte/store`:
- `import type { Component } from 'svelte'; import { derived, writable, type Readable } from 'svelte/store';`
- Ekspor tipe `PanelArea`, `PanelSpec`, `TabTypeSpec` persis Kontrak.
- Store privat `panelStore = writable<Record<string, PanelSpec>>({})` (kunci `id`) dan `tabTypeStore = writable<Record<string, TabTypeSpec>>({})` (kunci `type`).
- `registerPanel(spec)`: `panelStore.update(m => ({ ...m, [spec.id]: spec }))` (id ganda: menimpa). `registerTabType(spec)` analog dengan kunci `spec.type`. Selalu buat objek baru agar subscriber reaktif.
- `panels(area)`: `derived(panelStore, m => Object.values(m).filter(p => p.area === area).sort((a, b) => (a.order ?? 0) - (b.order ?? 0) || a.id.localeCompare(b.id)))`.
- `tabTypes()`: `return { subscribe: tabTypeStore.subscribe }` bertipe `Readable<Record<string, TabTypeSpec>>`.

### 8. `frontend/src/lib/ipc.ts`
Tanpa impor `wailsjs`. Deklarasi tipe sendiri:
```ts
interface WailsBridge { Call(method: string, params: unknown): Promise<unknown>; }
interface WailsWindow {
  go?: { main?: { Bridge?: WailsBridge } };
  runtime?: { EventsOn(name: string, cb: (...data: any[]) => void): () => void };
}
const w = (): WailsWindow => window as unknown as WailsWindow;
function hasBridge(): boolean { return typeof window !== 'undefined' && !!w().go?.main?.Bridge; }
export const inWails: boolean = hasBridge();
```
- `call<T>(method, params?)`: bila `!hasBridge()` kembalikan `Promise.reject(new Error('bukan di dalam Wails'))` (cek dinamis, bukan konstanta, karena binding bisa diinjeksi setelah modul dimuat). Selain itu `return w().go!.main!.Bridge!.Call(method, params ?? null) as Promise<T>`. Galat dari Go (string) dibungkus: `.catch(e => { throw e instanceof Error ? e : new Error(String(e)); })`.
- `onEvent(prefix, fn)`: bila `!w().runtime?.EventsOn` kembalikan fungsi no-op. Selain itu `return w().runtime!.EventsOn('daemon:event', (ev: { topic: string; payload: unknown }) => { if (ev && typeof ev.topic === 'string' && ev.topic.startsWith(prefix)) fn(ev.topic, ev.payload); })`; nilai kembalian `EventsOn` adalah fungsi unsubscribe. Prefix kosong cocok semua topik.

### 9. `frontend/src/main.ts`
```ts
import { mount } from 'svelte';
import App from './App.svelte';

// Modul epic lain mendaftarkan panel/tab saat di-import. Glob kosong aman bila folder belum ada.
import.meta.glob('./modules/*/index.ts', { eager: true });

const app = mount(App, { target: document.getElementById('app')! });
export default app;
```
Glob harus dievaluasi sebelum `mount`.

### 10. `frontend/src/App.svelte`
Svelte 5 (runes), `<script lang="ts">`. Hanya placeholder (tiket 06 menggantinya):
- Impor `onMount` dari `svelte`, `call`, `inWails` dari `./lib/ipc`, `panels`, `tabTypes` dari `./lib/registry`.
- State: `let status = $state<'idle' | 'loading' | 'ok' | 'error'>('idle'); let result = $state<unknown>(null); let errorMessage = $state('');`
- `async function ping()`: set `loading`; `result = await call('ping')` -> `ok`; catch -> `errorMessage = e instanceof Error ? e.message : String(e)`, status `error`. Tidak boleh melempar ke luar.
- `onMount(ping)`.
- Store: `const sidebar = panels('sidebar'); const mainTab = panels('main-tab'); const statusbar = panels('statusbar'); const tabs = tabTypes();` dipakai dengan `$sidebar.length`, `$mainTab.length`, `$statusbar.length`, `Object.keys($tabs).length`.
- Markup: `<h1>Hello World</h1>`; baris status ("Memeriksa daemon..." / `pong` dengan `JSON.stringify(result)` / galat; bila galat tampilkan `errorMessage` apa adanya, yang memuat "daemon tidak berjalan di <socket>" atau "bukan di dalam Wails"); tombol "Ping ulang" memanggil `ping`; daftar "Panel terdaftar: sidebar N, main-tab N, statusbar N, tipe tab N"; teks kecil "Mode: Wails" atau "Mode: browser (bukan di dalam Wails)" memakai `inWails`.
- CSS di `<style>` lokal, minimal, mendukung `prefers-color-scheme` gelap/terang.

### 11. `app/bridge.go` (`package main`)
Import: `context`, `encoding/json`, `errors`, `fmt`, `sync`, `aiterm/internal/ipc`, `aiterm/internal/paths`, `github.com/wailsapp/wails/v2/pkg/runtime`.
- `type Bridge struct { mu sync.Mutex; ctx context.Context; client *ipc.Client; unsub func() }`; `NewBridge() *Bridge { return &Bridge{} }`.
- `SetContext(ctx)`: kunci mutex, simpan ctx.
- Method internal `connect() (*ipc.Client, error)` (dipanggil dengan mutex terkunci): bila `b.client != nil` kembalikan. Selain itu `sock, err := paths.SocketPath()` (error dibungkus `fmt.Errorf("gagal menentukan path socket: %w", err)`); `c, err := ipc.Dial(sock)`; bila gagal kembalikan `fmt.Errorf("daemon tidak berjalan di %s", sock)`. Lalu `unsub, err := c.Subscribe("", b.relay)`; bila gagal `c.Close()` dan kembalikan `fmt.Errorf("gagal berlangganan event daemon: %w", err)`. Simpan `b.client = c; b.unsub = unsub`.
- `relay(topic string, payload json.RawMessage)` (callback Subscribe; dipanggil dari goroutine client, JANGAN mengambil `b.mu` di sini agar tidak deadlock): salin `ctx` dengan cara aman (simpan ctx juga di `atomic.Value` atau baca di bawah mutex terpisah `ctxMu`; pilih satu, mis. `ctxMu sync.RWMutex`). Bila ctx nil, abaikan; selain itu `runtime.EventsEmit(ctx, "daemon:event", map[string]any{"topic": topic, "payload": payload})`.
- `Call(method string, params json.RawMessage) (json.RawMessage, error)`:
  1. Kunci `mu`, `c, err := b.connect()`, buka kunci (jangan menahan mutex selama `c.Call`, supaya panggilan konkuren jalan). Bila err, kembalikan `nil, err`.
  2. Siapkan `var p any`; bila `len(params) > 0 && string(params) != "null"` set `p = params`, selain itu `p = nil`.
  3. `var out json.RawMessage; err = c.Call(method, p, &out)`.
  4. Bila `err != nil`: `var ie *ipc.Error; if errors.As(err, &ie) { return nil, err }` (error server, koneksi masih sehat). Selain itu error transport: panggil `b.drop(c)` (kunci `mu`; bila `b.client == c` panggil `b.unsub()` bila tidak nil, `c.Close()`, set `client`/`unsub` nil), lalu `return nil, fmt.Errorf("koneksi ke daemon terputus: %w", err)`. Tanpa retry otomatis (hindari eksekusi ganda untuk method bermutasi); `Call` berikutnya menyambung ulang secara malas lewat `connect()`. Tanpa timer.
  5. Bila `out` kosong kembalikan `json.RawMessage("null")`; selain itu `out`.
- `Close() error`: kunci `mu`; bila `unsub != nil` panggil; bila `client != nil` `Close()` dan nil-kan; kembalikan error Close. Aman dipanggil berulang.
Tidak ada panic path; semua kegagalan menjadi error.

### 12. `app/main.go` (`package main`)
```go
//go:embed all:dist
var assets embed.FS
```
Import: `embed`, `io/fs`, `log`, `context`, Wails `wails`, `options`, `assetserver`.
`main()`:
- `sub, err := fs.Sub(assets, "dist")`; bila err `log.Fatal`.
- `bridge := NewBridge()`.
- `err = wails.Run(&options.App{ Title: "AI Terminal", Width: 1200, Height: 800, AssetServer: &assetserver.Options{Assets: sub}, OnStartup: func(ctx context.Context) { bridge.SetContext(ctx) }, OnShutdown: func(ctx context.Context) { _ = bridge.Close() }, Bind: []any{bridge} })`; bila err `log.Fatalf("gagal menjalankan aplikasi: %v", err)`.
Tidak ada daemon autostart. Bila Wails menolak `go build` tanpa tag, itu wajar; guard memakai `-tags desktop,production`.

### 13. Verifikasi (guard lokal)
1. `cd frontend && npm install && npm run build` harus sukses dan menghasilkan `frontend/dist/index.html`. Opsional `npm run check` (svelte-check) bersih.
2. `go build ./app/...` (sebelum `app/dist` terisi; harus lolos berkat `.gitkeep` dan `fs.Sub`). Bila tanpa tag gagal karena cgo/Wails, jalankan `go build -tags desktop,production -o /dev/null ./app` (butuh Xcode CLT). Kegagalan akibat stub `internal/ipc` yang belum diganti atau file fitur lain yang setengah jadi bukan urusan fitur ini; pastikan error hanya berasal dari file milik fitur ini.
3. Opsional: salin sementara `frontend/dist/*` ke `app/dist/`, jalankan `go build -tags desktop,production -o /dev/null ./app`, lalu HAPUS isi tambahan di `app/dist/` kecuali `.gitkeep`. Jangan menyisakan file di `app/dist/`.
4. Jangan menyisakan file sementara di luar ownedPaths. `frontend/node_modules/` dan `frontend/dist/` adalah artefak yang di-gitignore (oleh fitur lain), boleh tetap ada.
5. Tinjau kode: `ipc.ts` tidak mengimpor `wailsjs`; `onEvent` memakai event `daemon:event`.

## Perubahan

Dibuat:
- `wails.json`
- `app/main.go`
- `app/bridge.go`
- `frontend/package.json`
- `frontend/package-lock.json` (hasil `npm install`)
- `frontend/vite.config.ts`
- `frontend/svelte.config.js`
- `frontend/tsconfig.json`
- `frontend/index.html`
- `frontend/src/env.d.ts`
- `frontend/src/main.ts`
- `frontend/src/App.svelte`
- `frontend/src/lib/registry.ts`
- `frontend/src/lib/ipc.ts`

Tidak ada file yang diubah di luar daftar ini.

## Wiring

Fitur ini hanya mengekspos; penyambungan dikerjakan langkah wiring (overview bagian 5, butir 5, 6, 7). Jangan disambungkan sendiri:
- `Bridge` (`NewBridge`, `SetContext`, `Call`, `Close`) dan `app/main.go` yang sudah mem-bind-nya; `go:embed all:dist` mengharapkan `app/dist/` berisi hasil salinan `frontend/dist/` oleh target `build-app` (Makefile milik `go-skeleton-build`).
- `frontend/package.json` skrip `build` (`vite build`) dan `frontend/dist/` untuk target `build-ui`.
- `main.ts` sudah memuat `./modules/*/index.ts`; epic lain cukup menambah modul, `registerPanel`/`registerTabType` dari `lib/registry.ts`, dan `call`/`onEvent` dari `lib/ipc.ts`.
- Ketergantungan runtime: `internal/ipc` asli (menggantikan stub), `internal/paths`, daemon `aitermd` untuk hasil `pong` di window.

## Selesai bila

- Semua file di bagian Perubahan ada; tidak ada file di luar ownedPaths + 3 pendukung yang disentuh; `go.mod`/`go.sum` tidak diubah.
- Guard: `cd frontend && npm install && npm run build` sukses dan menghasilkan `frontend/dist/`; `go build ./app/...` (atau dengan `-tags desktop,production`) sukses untuk path fitur ini.
- `import.meta.glob('./modules/*/index.ts', { eager: true })` tidak membuat build gagal tanpa folder `modules/`.
- `registry.ts` dan `ipc.ts` persis sesuai Kontrak; `ipc.ts` tidak mengimpor `wailsjs/`; `call` menolak dengan "bukan di dalam Wails" bila bukan di Wails.
- `Bridge.Call` mengembalikan error "daemon tidak berjalan di <socket>" bila daemon mati (tanpa panic), menyambung ulang malas pada `Call` berikutnya, dan merelay event ke `daemon:event` dengan data `{topic, payload}`.
- `app/dist/` tidak berisi sisa salinan (hanya `.gitkeep`); tidak ada file sementara tersisa.
- Tanpa unit test.
