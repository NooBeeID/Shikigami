# Overview Plan: Epic 07 File Viewer

Tiket: `docs/tickets/ai-terminal-ticket-07-file-viewer.md`
Spec: `docs/superpowers/specs/2026-11-21-ai-terminal-design.md`

Dokumen ini adalah desain BERSAMA. Agent per fitur hanya membaca overview ini dan tiket. Bila overview dan tiket bertentangan, tiket menang, KECUALI untuk bentuk pesan `fs.*` (bagian 2.2): di sana kode `internal/fsx/` yang sudah ada adalah sumber kebenaran dan tabel tiket hanya asumsi kerja.

## 1. Ringkasan epic dan urutan eksekusi

Epic ini murni frontend (Svelte 5 + Vite), semua kode di `frontend/src/modules/files/`. Isinya: panel sidebar "Files" (tree malas per project), tab bertipe `file` (viewer markdown, teks polos berrender malas, penolakan biner), mode sunting `<textarea>`, Cmd+S, penanda belum disimpan, deteksi konflik (`ERR_CONFLICT`) dan perubahan eksternal (`fs.changed.<path>`). Tanpa kode Go, tanpa dependency npm baru, tanpa unit test.

Kondisi kode saat plan ditulis (dibaca langsung dari repo):
- `frontend/src/lib/registry.ts` ada: `registerPanel({id, area, component, order?})`, `registerTabType({type:'session'|'file', component})`.
- `frontend/src/lib/ipc.ts` ada: `call<T>(method, params?)` dan `onEvent(topicPrefix, fn(topic, payload)) => unsubscribe`. Error dari `call` adalah `Error` biasa (pesan dari Wails); kode JSON-RPC tidak dijamin ada sebagai properti terstruktur, lihat 2.3.
- `frontend/src/main.ts` memuat `./modules/*/index.ts` lewat `import.meta.glob` (eager). Modul tanpa `index.ts` TIDAK dimuat, jadi stub aman.
- `frontend/src/lib/stores/` dan `frontend/src/modules/{shell,tabs,workspace}/` (epic ui-shell, tiket 06, dependensi) BELUM ada di working tree saat ini. Foundation harus membacanya; bila belum ada atau tidak punya fungsi pembuka tab, ikuti aturan blocker di tiket (hentikan dan laporkan, jangan edit file ui-shell).
- `internal/fsx/` (tiket 03) sudah ada dan menjadi acuan pesan IPC.

Urutan eksekusi:
1. **Foundation** (`00-foundation.md`): satu agent. Membuat `frontend/src/modules/files/shared/` (adaptor `fs.*` lengkap, adaptor ui-shell, tipe) dan stub kompilasi untuk semua file kontrak di `viewer/`, `editor/`, `tree/` agar tiap fitur bisa build sendirian.
2. **Fitur paralel** (3): `file-tree-panel`, `file-tab-viewer`, `file-editor-save`.
3. **Wiring** (bagian 5): pastikan stub tergantikan, `index.ts` final, build penuh, smoke test manual.

## 2. Kontrak bersama

### 2.1 Folder, nama, konvensi
- Semua path di bawah `frontend/src/modules/files/`:
  - `index.ts` (milik `file-editor-save`)
  - `shared/` (milik Foundation; setelah Foundation selesai TIDAK diubah fitur mana pun)
  - `tree/` (`file-tree-panel`), `viewer/` (`file-tab-viewer`), `editor/` (`file-editor-save`)
- Identifier kode Inggris; teks UI dan pesan Indonesia. Komponen Svelte 5 (runes: `$props`, `$state`, `$derived`, `$effect`), TypeScript, impor relatif tanpa alias. Tanpa dependency baru, tanpa `innerHTML` dari teks mentah, tanpa `setInterval`/polling.
- Dalam modul ini, `path` pada API internal SELALU path absolut (tab `file` menyimpan path absolut). Konversi ke `{root, rel}` hanya di `shared/fsClient.ts`.
- Styling: pakai CSS variable ui-shell bila ada (baca `frontend/src/styles/`, jangan ubah), dengan fallback `inherit`/`currentColor`. Font monospace untuk teks polos dan textarea.

### 2.2 Pesan IPC `fs.*` yang SEBENARNYA (dari `internal/fsx/ipc.go`, `tree.go`, `readwrite.go`, `watch.go`)

Berbeda dari tabel tiket. Semua method membawa `root` = path absolut project (`project.path`) dan `path`/`relDir` relatif terhadap root, pemisah `/`, tanpa awalan `/`.

| Method | Params | Hasil |
|---|---|---|
| `fs.list` | `{root, relDir}` (`""` atau `"."` = root) | `{entries: [{name, type: 'file'\|'dir'\|'symlink', size, mtime}]}` (BUKAN `isDir`, dan tanpa field `path`; path anak = `relDir + '/' + name`). Sudah terurut folder dulu, lalu nama. Symlink dilaporkan tanpa diikuti. |
| `fs.read` | `{root, path, offset?, limit?}` (`limit<=0` = sampai akhir; param bernama `limit`, bukan `length`) | `{content, mtime, kind: 'markdown'\|'text', size, offset, length, eof}`. Potongan disesuaikan ke batas rune UTF-8, jadi `offset` berikutnya = `offset + length` (pakai `length` balasan, bukan `limit` permintaan). |
| `fs.write` | `{root, path, content, expectedMtime}` | `{mtime, size}`. `expectedMtime` 0 hanya untuk file baru; file ada dengan mtime beda atau 0 menghasilkan `ERR_CONFLICT`. |
| `fs.watch` / `fs.unwatch` | `{root, path}` | `{}` (objek kosong) |

Event: topik `fs.changed.<absPath>` (path ABSOLUT), payload `{root, path (relatif), op: 'write'|'remove'|'rename'|'create', mtime}`. Disubscribe lewat `onEvent('fs.changed.' + absPath, ...)`; sebagai tambahan filter `topic === 'fs.changed.' + absPath` (prefiks bisa cocok dengan path lain yang berawalan sama, mis. `a.md` vs `a.md.bak`).

Kode error: -32000 ERR_BINARY, -32001 ERR_CONFLICT (data `{currentMtime}` bila diteruskan ipc), -32002 ERR_NOT_FOUND, -32003 ERR_INVALID.

### 2.3 Dua risiko yang dikunci keputusannya di sini
1. **Pengenalan kode error di frontend.** `ipc.ts` hanya melempar `Error` biasa. `shared/fsClient.ts` (Foundation) memetakan error ke kelas bertipe dengan cara tahan banting: baca `e.code` bila ada, jika tidak cari `-3200\d` atau string `binary file not supported` / `file changed on disk` / `not found` / `invalid argument` di `e.message`. Foundation WAJIB membaca `app/bridge.go` dan `internal/ipc/errors.go` untuk melihat bentuk teks error yang sampai ke JS, dan menyesuaikan satu fungsi `classifyError()` itu. Fitur lain hanya memakai kelas hasilnya.
2. **Presisi mtime.** `mtime` dari fsx adalah Unix nanodetik (sekitar 1.7e18), melebihi `Number.MAX_SAFE_INTEGER` (9e15). Setelah JSON.parse di JS nilainya terbulatkan, sehingga mengirimnya kembali sebagai `expectedMtime` dapat memicu `ERR_CONFLICT` palsu pada setiap simpan. Aturan epic ini: `mtime` diperlakukan sebagai token opaque bertipe `number`, dibandingkan dengan `===` dan dikirim balik persis apa adanya; tidak ada konversi satuan. Foundation WAJIB memverifikasi (mis. cek apakah `Number.isSafeInteger(mtime)` pada hasil nyata, atau analisis Wails) dan mencatat hasilnya di bagian "Catatan Foundation" file `00-foundation.md`/progress. Bila terbukti hilang presisi, itu adalah cacat lintas-epic yang HARUS dilaporkan ke pemilik epic file-service (usul perbaikan: `fsx.Write` membandingkan dengan toleransi atau mengekspos mtime sebagai string/milidetik). Epic ini tidak boleh mengubah kode Go; semua penyesuaian sisi frontend hanya di `shared/fsClient.ts` (fungsi `normalizeMtime`).

### 2.4 Modul `shared/` (dibuat Foundation, dipakai semua fitur)

```ts
// shared/types.ts
export type FsEntryType = 'file' | 'dir' | 'symlink';
export interface FsEntry { name: string; type: FsEntryType; size: number; mtime: number }
export interface FileTabData { path: string; projectId: string; workspaceId: string }   // path absolut
export interface ReadResultRaw { content: string; mtime: number; kind: 'markdown'|'text'; size: number; offset: number; length: number; eof: boolean }
export interface FileChangedEvent { root: string; path: string; op: 'write'|'remove'|'rename'|'create'; mtime: number }

// shared/fsClient.ts  (SATU-SATUNYA tempat memanggil call('fs.*') dan memegang bentuk pesan)
export class FsBinaryError extends Error {}      // -32000
export class FsConflictError extends Error { currentMtime?: number }   // -32001
export class FsNotFoundError extends Error {}    // -32002
export class FsInvalidError extends Error {}     // -32003
export function splitPath(absPath: string): { root: string; rel: string }  // cari project dengan project.path terpanjang yang menjadi prefiks; lempar FsInvalidError bila tidak ada
export function fsList(absDir: string, root?: string): Promise<FsEntry[]>  // root opsional; bila ada dipakai langsung (tree tahu root-nya), bila tidak pakai splitPath
export function fsRead(absPath: string, offset?: number, limit?: number): Promise<ReadResultRaw>
export function fsWrite(absPath: string, content: string, expectedMtime: number): Promise<{ mtime: number; size: number }>
export function watchFile(absPath: string, onChange: (ev: FileChangedEvent) => void): () => void   // fs.watch lalu langganan event; fungsi pengembalian = berhenti langganan + fs.unwatch (idempoten)
export function joinPath(dirAbs: string, name: string): string
export function basename(path: string): string

// shared/shellBridge.ts  (SATU-SATUNYA tempat mengimpor store/komponen ui-shell)
export interface ShellProject { id: string; path: string; name: string }
export interface ShellWorkspace { id: string; projects: ShellProject[] }
export const activeWorkspace: Readable<ShellWorkspace | null>   // svelte store turunan dari store workspace aktif ui-shell
export function openFileTab(data: FileTabData): void            // memanggil openTab ui-shell; judul = basename; fokus bila path sama sudah terbuka (bila API mendukung)
export function decodeFileTab(props: Record<string, unknown>): FileTabData   // menerjemahkan props yang diberikan TabContent ke FileTabData
```

Keputusan tentang `FileTabData` dan props `FileTab`: cara `TabContent` (ui-shell) meneruskan data tab ke komponen belum pasti (tab sesi memakai `refId`). Foundation membaca `frontend/src/modules/tabs/` dan menetapkan skema di `shellBridge.ts`: `openFileTab` menyandikan data (mis. ke `refId` JSON atau field data bila ada) dan `decodeFileTab` mengurainya. `FileTab` HANYA memanggil `decodeFileTab($props())`; tidak ada fitur lain yang tahu skema ini.

### 2.5 API internal antar-fitur (signature dikunci, sama dengan tiket)

```ts
// viewer/fileApi.ts  (file-tab-viewer; membungkus shared/fsClient, tanpa memanggil call('fs.*') langsung)
export type FileKind = 'markdown' | 'text';
export function fileKind(path: string): FileKind;             // .md .markdown .txt (case-insensitive) => 'markdown'
export interface Chunk { content: string; size: number; mtime: number; eof: boolean; offset: number; length: number }
export function readChunk(path: string, offset: number, length: number): Promise<Chunk>;
export function readAll(path: string): Promise<Chunk>;         // loop potongan 256 KiB sampai eof; mtime = mtime potongan PERTAMA... lihat catatan
export class BinaryFileError extends Error {}                  // dari FsBinaryError
export class FileNotFoundError extends Error {}                // dari FsNotFoundError

// viewer/MarkdownView.svelte   props: { source: string }
// viewer/PlainTextView.svelte  props: { path: string; size: number }
// viewer/BinaryNotice.svelte   props: { path: string }

// editor/writeApi.ts  (file-editor-save)
export class ConflictError extends Error { currentMtime?: number }
export function writeFile(path: string, content: string, expectedMtime: number): Promise<{ mtime: number }>;

// editor/FileTab.svelte   props: diterima dari TabContent, didekode lewat decodeFileTab
// tree/FilesPanel.svelte  tanpa props
// tree/registerTree.ts    export function registerFilesPanel(): void   // registerPanel({id:'files', area:'sidebar', component: FilesPanel, order: 20})
```
Catatan `Chunk`: field `offset`/`length` ditambahkan (tambahan kompatibel) agar pembaca berjenjang tahu offset berikutnya. `readAll` memakai `mtime` dari potongan TERAKHIR yang dibaca, dan bila mtime antar-potongan berbeda (file berubah saat dibaca) mengulang dari awal sekali; `size` = ukuran total.

Konstanta bersama: ukuran potongan baca bawaan 256 KiB (`CHUNK_SIZE = 256 * 1024`, ada di `shared/types.ts`), ambang file besar markdown 2 MiB, peringatan sunting 5 MiB (konstanta lokal di fitur pemakainya; angka mengikuti tiket).

Event dirty bila ui-shell tidak punya API: `window.dispatchEvent(new CustomEvent('aiterm:tab-dirty', {detail:{path, dirty}}))`. Foundation memeriksa apakah store `tabs` ui-shell punya cara menandai kotor dan, bila ada, mengeksposnya sebagai `setTabDirty(path, dirty): void` di `shellBridge.ts` (bila tidak ada, fungsi itu hanya mengirim event CustomEvent di atas). `FileTab` selalu memanggil `setTabDirty`.

## 3. Daftar fitur

Tiga fitur, sesuai tiket (tidak ditambah/digabung).

| slug | scope | ownedPaths |
|---|---|---|
| `file-tree-panel` | Panel sidebar "Files": satu akar per project workspace aktif (`activeWorkspace`), tree dimuat malas per folder lewat `fsList` (cache per path, segarkan manual, error inline + coba lagi, tanpa polling/`fs.watch`), klik file memanggil `openFileTab`. Mengekspor `registerFilesPanel()`. Navigasi keyboard disarankan. | `frontend/src/modules/files/tree/` |
| `file-tab-viewer` | Lapisan baca: `fileApi.ts`, `MarkdownView.svelte` (renderer buatan sendiri, escape dulu, href hanya http/https/mailto, tanpa gambar jarak jauh), `PlainTextView.svelte` (virtual scroll baris tetap, baca berjenjang, cache LRU potongan, DOM < ~200 baris), `BinaryNotice.svelte`. | `frontend/src/modules/files/viewer/` |
| `file-editor-save` | `FileTab.svelte` (merangkai viewer, mode sunting textarea, Cmd+S, dirty, banner konflik/perubahan eksternal, watch hanya saat tampil), `writeApi.ts`, dan `index.ts` modul. | `frontend/src/modules/files/editor/`, `frontend/src/modules/files/index.ts` |

Ketergantungan antar fitur hanya lewat signature bagian 2.4 dan 2.5. Setiap fitur memulai dari stub Foundation lalu menimpa file miliknya.

## 4. Foundation

Ada (`hasFoundation=true`), detail langkah di `00-foundation.md`. Ringkas:
- `shared/types.ts`, `shared/fsClient.ts` (implementasi lengkap termasuk `classifyError`, `normalizeMtime`, `splitPath`, `watchFile`), `shared/shellBridge.ts` (implementasi nyata terhadap ui-shell).
- Stub bertanda `// STUB(foundation): diganti oleh <slug>` untuk: `viewer/fileApi.ts`, `viewer/MarkdownView.svelte`, `viewer/PlainTextView.svelte`, `viewer/BinaryNotice.svelte`, `editor/writeApi.ts`, `editor/FileTab.svelte`, `tree/FilesPanel.svelte`, `tree/registerTree.ts`. Stub memiliki signature kontrak persis dan build lolos.
- TIDAK membuat `index.ts` (milik `file-editor-save`); tanpa `index.ts` modul tidak dimuat sehingga stub tidak pernah tampil di app.
- Verifikasi risiko 2.3 (bentuk error dan presisi mtime) dan skema data tab.
- Guard: `cd frontend && npm run build` lolos (dan `npm run check` bila ada, tidak wajib).

## 5. Wiring checklist

Dikerjakan SATU agent setelah ketiga fitur selesai. Registrasi frontend otomatis; tidak ada edit ke registry, shell, `App.svelte`, atau `main.ts`.
1. `grep -rn "STUB(foundation)" frontend/src/modules/files/` harus kosong. Bila ada yang tersisa, fitur pemilik belum selesai; laporkan, jangan menulis ulang sendiri.
2. `frontend/src/modules/files/index.ts` ada dan persis memanggil `registerTabType({ type: 'file', component: FileTab })` (impor dari `./editor/FileTab.svelte`) dan `registerFilesPanel()` (impor dari `./tree/registerTree`); path impor ke `../../lib/registry` sesuai ekspor sebenarnya.
3. `FilesPanel` memakai `openFileTab` yang menghasilkan tab yang `FileTab` bisa dekode (satu skema `shellBridge`): cek bahwa `openFileTab` dan `decodeFileTab` konsisten.
4. Tidak ada file di luar `frontend/src/modules/files/` yang berubah, termasuk `frontend/package.json` dan `package-lock.json` (periksa `git status`/`git diff --stat`; `npm install` saat build tidak boleh mengubah lockfile, bila berubah kembalikan).
5. Tidak ada `setInterval`, `innerHTML` dari teks mentah, atau panggilan langsung `call('fs.` di luar `shared/fsClient.ts` (`grep`).
6. Jalankan build guard (bagian 6), lalu smoke test manual opsional: tambah project, buka tree, buka `.md` (rendered) dan `.go` (teks polos), sunting + Cmd+S, ubah file dari terminal lalu simpan lagi dan pastikan banner konflik muncul tanpa menimpa. Bila konflik palsu muncul pada simpan pertama tanpa perubahan eksternal, itu risiko mtime 2.3 (laporkan sebagai blocker lintas-epic).

## 6. Build guard

Satu-satunya guard, tanpa unit test:
- `go build ./...` (harus tetap lolos; tidak ada kode Go yang disentuh)
- `make build` (build frontend/Vite lalu daemon dan app Wails; modul `files` ikut terbundel)
- Untuk iterasi cepat per fitur: `cd frontend && npm run build`.

## 7. Aturan paralel

- Tiap agent hanya membuat/mengubah `ownedPaths` fiturnya (bagian 3). Foundation hanya `frontend/src/modules/files/shared/` dan stub di `viewer/`, `editor/`, `tree/`.
- Jangan menyentuh path fitur lain, `shared/` (bila butuh perubahan kontrak, laporkan sebagai blocker, jangan ubah), file wiring (`index.ts` hanya milik `file-editor-save`), maupun daftar terlarang di tiket: `frontend/package.json`, `vite.config.ts`, `tsconfig.json`, `main.ts`, `App.svelte`, `frontend/src/lib/`, `modules/shell|tabs|workspace|terminal`, `frontend/src/styles/`, dan semua kode Go.
- Fitur lain boleh memakai stub Foundation untuk file milik fitur sebelah; jangan mengedit stub itu. Saat fitur pemilik selesai, stub tertimpa dengan signature yang sama.
- Hanya `fsClient.ts` yang boleh memanggil `call('fs.*')` dan hanya `shellBridge.ts` yang boleh mengimpor dari ui-shell.
- Tidak ada polling/timer berkala; `fs.watch` hanya untuk file yang sedang tampil di `FileTab` dan wajib `fs.unwatch` saat disembunyikan/dihancurkan; tree tidak pernah di-watch.
- Jangan commit; ikuti alur skill implement-feature untuk worktree/merge.
