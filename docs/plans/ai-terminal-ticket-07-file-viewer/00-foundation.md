# Foundation: Epic 07 File Viewer

Dikerjakan SATU agent sebelum fitur paralel. Baca `overview.md` (bagian 2 dan 4) dan tiket terlebih dahulu. Tujuan: `cd frontend && npm run build` hijau dengan semua file kontrak sudah ada (sebagian stub) dan adaptor `fs.*` serta adaptor ui-shell sudah lengkap, sehingga `file-tree-panel`, `file-tab-viewer`, `file-editor-save` bisa dikerjakan bersamaan.

Batasan path: hanya buat di `frontend/src/modules/files/shared/` dan stub di `viewer/`, `editor/`, `tree/`. JANGAN membuat `index.ts`. Jangan sentuh `package.json`, `lib/`, atau modul lain.

## Langkah

1. **Baca kode nyata.** `frontend/src/lib/ipc.ts`, `frontend/src/lib/registry.ts`, `app/bridge.go`, `internal/ipc/errors.go` dan `internal/ipc/server.go`/`protocol.go` (bagaimana error JSON-RPC sampai ke JS), `internal/fsx/ipc.go`, `tree.go`, `readwrite.go`, `watch.go`. Lalu baca kode ui-shell: `frontend/src/lib/stores/` (workspaces, tabs; aksi `openTab`), `frontend/src/modules/tabs/` (bagaimana `TabContent` meneruskan data tab ke komponen), `frontend/src/modules/shell/`. Bila `lib/stores/` atau fungsi `openTab` tidak ada, berhenti dan laporkan blocker (jangan mengedit ui-shell).

2. **`shared/types.ts`.** Tipe pada overview 2.4 (`FsEntryType`, `FsEntry`, `FileTabData`, `ReadResultRaw`, `FileChangedEvent`) plus `export const CHUNK_SIZE = 256 * 1024;`.

3. **`shared/fsClient.ts` (lengkap).**
   - Kelas error `FsBinaryError`, `FsConflictError` (field `currentMtime?`), `FsNotFoundError`, `FsInvalidError`.
   - `classifyError(e: unknown): unknown`: petakan ke kelas di atas berdasar `e.code` bila ada, kalau tidak berdasar teks `e.message` (kode -32000..-32003 atau frasa pesan fsx: `binary file not supported`, `file changed on disk`, `not found`, `invalid argument`). Sesuaikan dengan bentuk teks error nyata hasil pembacaan langkah 1; error lain dilempar apa adanya. Bila `currentMtime` bisa diambil dari `data` atau teks `disk mtime N`, isi field-nya.
   - `splitPath(absPath)`: ambil project dari `get(activeWorkspace)` (shellBridge) dengan `project.path` terpanjang yang menjadi prefiks (batas pemisah `/`); `rel` = sisa tanpa awalan `/`. Bila tidak ada project cocok, lempar `FsInvalidError`.
   - `fsList(absDir, root?)`, `fsRead`, `fsWrite`, `watchFile`, `joinPath`, `basename` sesuai signature overview 2.4 dan bentuk pesan nyata overview 2.2: `fs.list {root, relDir}`; `fs.read {root, path, offset, limit}`; `fs.write {root, path, content, expectedMtime}`; `fs.watch/unwatch {root, path}`. `fsList` mengembalikan `entries` apa adanya (`type`, bukan `isDir`).
   - `watchFile`: panggil `fs.watch` dan `onEvent('fs.changed.' + absPath, ...)`; filter `topic === 'fs.changed.' + absPath`; teruskan payload sebagai `FileChangedEvent`. Fungsi pengembalian idempoten: berhenti langganan lalu `fs.unwatch` (abaikan error unwatch). Jangan ada timer.
   - `normalizeMtime(n: number): number`: saat ini identitas; satu-satunya tempat penyesuaian bila risiko presisi (langkah 6) terbukti.
   - Semua fungsi membungkus `call` dengan `try/catch` yang melempar `classifyError(e)`.

4. **`shared/shellBridge.ts` (nyata terhadap ui-shell).** Implementasikan `activeWorkspace` (Readable turunan dari store workspaces + `activeWorkspaceId`), `openFileTab(data)` (panggil `openTab` ui-shell dengan `type:'file'`, judul = basename; hindari duplikat tab dengan path sama bila API mendukung, bila tidak cari di store `tabs` dulu lalu aktifkan), `decodeFileTab(props)` (skema penyandian mengikuti cara `TabContent` meneruskan data; satu skema, didokumentasikan dengan komentar), dan `setTabDirty(path, dirty)` (pakai API tab kotor ui-shell bila ada, selain itu `window.dispatchEvent(new CustomEvent('aiterm:tab-dirty', {detail:{path, dirty}}))`). Bila `TabContent` tidak dapat meneruskan data file ke komponen tanpa mengedit ui-shell, laporkan blocker.

5. **Stub kontrak** (tiap file diawali komentar `// STUB(foundation): diganti oleh <slug>` atau `<!-- STUB(foundation) ... -->`), dengan signature persis overview 2.5 dan berbunyi sederhana:
   - `viewer/fileApi.ts` (`file-tab-viewer`): `fileKind` (implementasi ekstensi sederhana boleh), `Chunk`, `readChunk`/`readAll` yang memanggil `fsRead` dan memetakan error ke `BinaryFileError`/`FileNotFoundError`, kelas error.
   - `viewer/MarkdownView.svelte` (props `source`), `viewer/PlainTextView.svelte` (props `path`, `size`), `viewer/BinaryNotice.svelte` (props `path`): menampilkan `<pre>` teks sederhana / pesan, tanpa `innerHTML`.
   - `editor/writeApi.ts` (`file-editor-save`): `ConflictError`, `writeFile` yang memanggil `fsWrite` dan memetakan `FsConflictError`.
   - `editor/FileTab.svelte` (`file-editor-save`): placeholder yang memanggil `decodeFileTab` dan menampilkan path.
   - `tree/FilesPanel.svelte` dan `tree/registerTree.ts` (`file-tree-panel`): panel kosong berjudul "Files"; `registerFilesPanel()` memanggil `registerPanel({id:'files', area:'sidebar', component: FilesPanel, order: 20})`.

6. **Verifikasi risiko.** (a) Presisi mtime: telaah jalur Wails (`Bridge.Call` mengembalikan `json.RawMessage`, JS menerima hasil JSON.parse) dan, bila memungkinkan tanpa menjalankan app, jelaskan apakah nilai ns terbulatkan. Tulis kesimpulan dan rekomendasi di akhir file ini pada bagian "Catatan Foundation". (b) Bentuk error: catat contoh teks error nyata yang dipakai `classifyError`.

7. **Build.** `cd frontend && npm run build` harus lolos (juga `npm run check` bila bersih; kegagalan check yang sudah ada sebelumnya bukan tanggung jawab epic ini). `go build ./...` tetap lolos. Pastikan `git status` hanya menunjukkan path di `frontend/src/modules/files/` (dan lockfile tidak berubah).

## Catatan Foundation
(diisi oleh agent Foundation: hasil verifikasi presisi mtime, bentuk error, skema data tab)

- Skema data tab: `TabContent` hanya meneruskan `sessionId`/`refId`/`tabId`. `openFileTab` menyandikan `FileTabData` sebagai JSON (kunci path, projectId, workspaceId) di `refId`; id tab = `file:<refId>` sehingga `openTab` ui-shell sudah dedup/aktifkan; `decodeFileTab` mengurai `props.refId`. Tab kotor: tidak ada API di ui-shell, dipakai CustomEvent `aiterm:tab-dirty`.
- Bentuk error: `Bridge.Call` mengembalikan `*ipc.Error` apa adanya; teks di JS = `<pesan> (kode N)`, mis. `file changed on disk (disk mtime 1700000000000000000) (kode -32001)`, `binary file not supported: a.bin (kode -32000)`, `not found: x (kode -32002)`. `classifyError` mencocokkan `-3200[0-3]` lalu frasa pesan; `currentMtime` diambil dari `disk mtime N`.
- Presisi mtime (RISIKO TERBUKTI secara analisis, belum dijalankan): fsx memakai `UnixNano` int64 (~1.7e18 > 2^53). Hasil `fs.read` melewati `json.RawMessage` lalu JSON.parse di WebView sehingga dibulatkan ke double (kelipatan 256 pada rentang ini). `JSON.stringify` mengirim ulang angka bulat terdekat, yang umumnya BUKAN nilai ns asli, sehingga `fs.write` dapat menghasilkan `ERR_CONFLICT` palsu (terutama di APFS/ext4 dengan ns penuh). `normalizeMtime` masih identitas. Rekomendasi (lintas-epic, epic file-service): ekspos mtime sebagai milidetik atau string, atau bandingkan dengan toleransi di `fsx.Write`. Verifikasi smoke test: simpan tanpa perubahan eksternal.
