---
id: ai-terminal-ticket-07-file-viewer
title: File Viewer
spec: docs/superpowers/specs/2026-11-21-ai-terminal-design.md
status: ready
wave: 5
depends_on: [ai-terminal-ticket-06-ui-shell, ai-terminal-ticket-03-file-service]
---
# File Viewer

## Tujuan

Epic ini menambahkan sisi UI untuk membaca dan menulis file (spec bagian 2 dan fase 1b): panel tree project per workspace di sidebar, dan tab bertipe `file` di area tab utama, berdampingan dengan tab terminal. File `.txt`/`.md` dibuka dengan markdown view otomatis, jenis lain sebagai teks polos yang dirender malas (hanya baris terlihat), file biner ditolak dengan pesan. Pengguna bisa menyunting teks polos, menyimpan dengan Cmd+S, melihat penanda "belum disimpan", dan diperingatkan (bukan ditimpa diam-diam) bila agent mengubah file yang sama di disk. Epic ini murni frontend; semua akses file lewat IPC `fs.*` milik epic file-service. Setelah selesai, `make build` tetap lolos dan aplikasi sudah bisa membuka serta menyimpan file.

## Batasan

In scope:
- Panel sidebar "Files": tree per project dalam workspace aktif, dimuat malas per folder (`fs.list`).
- Tab `file`: viewer markdown (`.txt`/`.md`), viewer teks polos berrender malas, penolakan file biner.
- Mode sunting teks polos (tanpa syntax highlighting), Cmd+S, penanda belum disimpan, deteksi konflik disk (`ERR_CONFLICT`) dan perubahan eksternal (`fs.changed.<path>`), tawaran muat ulang.
- Registrasi lewat `registerPanel` dan `registerTabType` di `frontend/src/modules/files/index.ts`.

Out of scope:
- Syntax highlighting, autocomplete, LSP, pencarian isi file, fitur IDE (spec bagian 3).
- Tombol "mention ke sesi" (ide lanjutan, bukan versi awal).
- Memonitor filesystem terus-menerus: tree tidak di-watch; `fs.watch` hanya untuk file yang sedang tampil dan harus `fs.unwatch` saat tab ditutup/disembunyikan (anti-goal polling, spec bagian 4).
- Kode Go, daemon, `fs.*` itu sendiri (epic file-service), registry/shell/store (epic app-base dan ui-shell). Tidak ada file di luar `frontend/src/modules/files/` yang disentuh, termasuk `frontend/package.json`.
- Dependency npm baru. Karena `package.json` milik app-base, markdown dirender dengan renderer kecil buatan sendiri di `viewer/` (HTML di-escape lebih dulu, lalu hanya tag hasil konversi yang dibuat; tautan hanya skema http/https/mailto; tanpa `innerHTML` dari teks mentah). Bila kelak ingin `marked` + `DOMPurify`, itu perubahan terkoordinasi ke app-base, bukan bagian tiket ini.
- Unit test (guard hanya `make build`).

## Kontrak

### Yang dipakai dari epic lain

Baca file aslinya sebelum menulis kode; nama di bawah adalah pegangan, identifier sebenarnya mengikuti kode di repo.

- `frontend/src/lib/registry.ts` (app-base): `registerPanel({id, area:'sidebar'|'main-tab'|'statusbar', component, order})` dan `registerTabType({type:'session'|'file', component})`. Dirender oleh ui-shell. Modul dimuat otomatis lewat `import.meta.glob` dari `frontend/src/modules/<modul>/index.ts`.
- `frontend/src/lib/ipc.ts` (app-base): pembungkus `Call(method, params)` dan langganan event (`daemon:event` Wails, bentuk `{topic, payload}`). Pakai untuk semua panggilan `fs.*`. Error JSON-RPC membawa kode: -32000 `ERR_BINARY`, -32001 `ERR_CONFLICT`, -32002 `ERR_NOT_FOUND`, -32003 `ERR_INVALID`.
- Store workspace aktif dan cara membuka tab (ui-shell, `frontend/src/lib/stores/` dan `frontend/src/modules/shell/`): cari store workspace aktif (daftar `projects[{id,path,name}]`) dan fungsi pembuka tab (mis. `openTab({type:'file', ...})`). Hanya BACA dan IMPOR; jangan ubah. Bila fungsi pembuka tab belum ada pada kode ui-shell, hentikan dan laporkan sebagai blocker alih-alih mengedit file ui-shell.
- Bentuk tab `file`: data tab minimal `{type:'file', path:string, projectId:string, workspaceId:string}`; judul tab = nama file (basename). Tab yang sama (path sama) tidak dibuka dua kali, cukup difokuskan bila API shell mendukung.

### Pesan IPC `fs.*` (disediakan epic file-service)

Bentuk berikut adalah asumsi kerja dari spec dan kontrak bersama. Verifikasi dengan `internal/fsx/` dan `cmd/aitermctl/cmd_fs.go` yang sudah ada; bila nama field berbeda, ikuti kode file-service dan sesuaikan SATU lapisan adaptor (`viewer/fileApi.ts`) saja, jangan menyebar ke komponen.

| Method | Params | Hasil |
|---|---|---|
| `fs.list` | `{path}` | `{entries:[{name, path, isDir, size, mtime}]}`, folder abaian (`node_modules`, `.git`, `target`, dll.) sudah disaring daemon, urut folder dulu lalu nama |
| `fs.read` | `{path, offset?, length?}` | `{content, size, mtime, offset, eof}`; baca berjenjang per potongan byte; mtime dipakai sebagai token konflik; `ERR_BINARY` (-32000) untuk file biner |
| `fs.write` | `{path, content, expectedMtime}` | `{mtime}` baru; `ERR_CONFLICT` (-32001) bila mtime disk != `expectedMtime` |
| `fs.watch` / `fs.unwatch` | `{path}` | `{ok:true}`; event `fs.changed.<path>` dengan payload `{path, mtime}` (atau serupa) |

Subscribe event lewat prefix topik `fs.changed.<path>` (client.Subscribe di sisi Go; di frontend lewat pembungkus `ipc.ts`).

### API internal modul (antar-fitur, ikuti signature persis agar fitur bisa paralel)

Semua di dalam `frontend/src/modules/files/`:

```ts
// viewer/fileApi.ts  (milik file-tab-viewer)
export type FileKind = 'markdown' | 'text';
export function fileKind(path: string): FileKind;            // .md .markdown .txt => 'markdown', selain itu 'text'
export interface Chunk { content: string; size: number; mtime: number; eof: boolean }
export function readChunk(path: string, offset: number, length: number): Promise<Chunk>;
export function readAll(path: string): Promise<Chunk>;        // gabung semua potongan; dipakai markdown view dan mode sunting
export class BinaryFileError extends Error {}                 // dilempar saat -32000
export class FileNotFoundError extends Error {}               // dilempar saat -32002

// viewer/MarkdownView.svelte   props: { source: string }
// viewer/PlainTextView.svelte  props: { path: string; size: number }   // render malas, baca berjenjang
// viewer/BinaryNotice.svelte   props: { path: string }

// editor/writeApi.ts   (milik file-editor-save)
export class ConflictError extends Error {}                   // dilempar saat -32001
export function writeFile(path: string, content: string, expectedMtime: number): Promise<{ mtime: number }>;

// editor/FileTab.svelte (komponen tab 'file')   props: tab data {path, projectId, workspaceId}

// tree/FilesPanel.svelte (komponen panel sidebar)  tanpa props wajib
// tree/registerTree.ts: export function registerFilesPanel(): void   // memanggil registerPanel({id:'files', area:'sidebar', component: FilesPanel, order: ...})
```

Penanda "belum disimpan": cek dulu apakah ui-shell menyediakan cara menandai tab kotor (store/prop). Bila ada, pakai. Bila tidak, `FileTab` (a) menampilkan titik "●" di bilah judul file di dalam kontennya dan (b) mengirim `window.dispatchEvent(new CustomEvent('aiterm:tab-dirty', {detail:{path, dirty}}))` sehingga ui-shell dapat memakainya kelak tanpa mengubah epic ini.

## Fitur

### file-tree-panel: Panel tree project

Deskripsi: panel sidebar (`area:'sidebar'`) bernama "Files" yang menampilkan, untuk workspace aktif, satu akar per project (`projects[].name`), masing-masing dapat dikembangkan menjadi tree folder/file. Isi folder baru dimuat saat folder dibuka (`fs.list {path}`), tidak memindai seluruh project. Klik file membuka tab `file`.

Detail:
- Akar tiap project = `project.path`. Node folder: panah buka/tutup; saat pertama dibuka panggil `fs.list`, tampilkan status "memuat...", lalu isi. Hasil di-cache per path selama panel hidup; membuka ulang folder yang sudah dimuat tidak memanggil ulang. Tombol segarkan kecil di header panel (dan per folder) membersihkan cache dan memuat ulang folder terbuka. Tidak ada timer/polling dan tidak ada `fs.watch` untuk tree.
- Ganti workspace aktif: tree di-reset ke akar project workspace baru. Ganti-ganti workspace tidak mengubah daftar sesi (tidak disentuh epic ini).
- Folder yang disaring (`node_modules`, `.git`, `target`, dll.) sudah ditangani daemon; panel tidak menambah aturan sendiri.
- Klik file memanggil fungsi pembuka tab milik ui-shell dengan `{type:'file', path, projectId, workspaceId}`. Klik ganda tidak perlu; satu klik cukup.
- Error `fs.list` (mis. -32002 folder hilang) ditampilkan inline di node itu dengan tombol coba lagi; tidak melempar ke konsol saja.
- Pemakaian ringan: node dirender hanya untuk folder yang terbuka; tidak membuat komponen untuk isi folder tertutup. Folder dengan ribuan entri tetap harus responsif (render daftar datar, tanpa animasi berat).
- Aksesibel dasar: navigasi panah atas/bawah/kanan/kiri dan Enter opsional tetapi disarankan.
- Gaya mengikuti token CSS ui-shell (`frontend/src/styles/` hanya dibaca/dipakai variabelnya; jangan ubah). Tanpa variabel yang cocok, pakai nilai warisan (`inherit`/`currentColor`).

Acceptance criteria:
- [ ] Panel "Files" muncul di sidebar setelah modul dimuat, tanpa mengedit file shell.
- [ ] Menampilkan satu akar per project dari workspace aktif; berganti workspace memperbarui akar.
- [ ] Membuka folder memanggil `fs.list` tepat sekali untuk folder itu; membuka ulang tidak memanggil lagi sampai disegarkan.
- [ ] Mengklik file membuka tab `file` dengan `path`, `projectId`, `workspaceId` benar.
- [ ] Error `fs.list` tampil inline dengan opsi coba lagi.
- [ ] Tidak ada `setInterval`/polling dan tidak ada panggilan `fs.watch` dari panel.
- [ ] `make build` lolos hanya dengan fitur ini terpasang (komponen tab boleh stub sementara, lihat Paralel).

ownedPaths:
- `frontend/src/modules/files/tree/`

Paralel: tidak bergantung pada implementasi `file-tab-viewer` maupun `file-editor-save`; hanya butuh fungsi pembuka tab ui-shell dan `fs.list`. `tree/registerTree.ts` mengekspor `registerFilesPanel()` yang dipanggil oleh `index.ts` (milik `file-editor-save`). Penulis fitur ini TIDAK membuat `index.ts`; untuk memverifikasi build sendiri, buat file sementara di luar repo atau di working tree lokal yang tidak di-commit, lalu hapus.

### file-tab-viewer: Tab viewer markdown dan teks

Deskripsi: lapisan baca file. Menyediakan adaptor `fs.read`, klasifikasi jenis file, renderer markdown, viewer teks polos berrender malas, dan pesan penolakan biner. Komponen-komponen ini dirangkai oleh `FileTab.svelte` (fitur `file-editor-save`).

Detail:
- `fileApi.ts` seperti di Kontrak. `readChunk` membungkus `fs.read` dengan `offset`/`length`; ukuran potongan bawaan 256 KiB. `readAll` mengulang sampai `eof`. Petakan error -32000 ke `BinaryFileError`, -32002 ke `FileNotFoundError`, lainnya dilempar apa adanya.
- Penentuan jenis: ekstensi (case-insensitive) `.md`, `.markdown`, `.txt` = markdown view otomatis; selainnya (`.go`, `.js`, `.json`, tanpa ekstensi, dll.) = teks polos. Tombol tombol-ganti "Rendered / Teks" di tab untuk `.md`/`.txt` dikelola `FileTab`, viewer cukup menyediakan kedua komponen.
- `MarkdownView.svelte`: renderer sendiri mendukung heading, paragraf, penekanan (tebal/miring), kode inline, blok kode berpagar, daftar berurutan/tak berurutan, kutipan, garis pemisah, tautan, tabel sederhana bila murah. Aturan keamanan: escape `& < > " '` pada seluruh teks mentah sebelum konversi; atribut `href` hanya `http:`/`https:`/`mailto:`; tautan membuka lewat mekanisme browser luar yang ada di Wails/shell bila tersedia, atau `target="_blank" rel="noopener noreferrer"`; tidak ada gambar jarak jauh otomatis (tampilkan teks alt atau tautan) agar tidak ada permintaan jaringan diam-diam. Dokumen markdown besar dirender sebagai satu pohon DOM; untuk file `.txt` besar yang tidak berstruktur, bila `size` melebihi 2 MiB, tampilkan lewat `PlainTextView` dengan banner "file besar, ditampilkan sebagai teks" agar RAM tidak membengkak.
- `PlainTextView.svelte`: virtual scrolling buatan sendiri. Tinggi baris tetap (font monospace), kontainer menggulir, hanya baris dalam viewport + margin (mis. 20 baris) yang ada di DOM. Memakai baca berjenjang: indeks awal baris dibangun bertahap dari potongan yang dibaca berurutan; potongan di luar jendela yang sudah jauh boleh dibuang dari memori (cache LRU kecil, mis. beberapa potongan). Tinggi total dapat diperkirakan dari `size` dan rata-rata panjang baris sampai indeks lengkap. Teks tidak dibungkus (scroll horizontal) agar tinggi baris tetap; tampilkan nomor baris opsional. Pembacaan lanjutan hanya dipicu oleh gulir, tidak ada pekerjaan latar.
- `BinaryNotice.svelte`: pesan jelas, mis. "File biner tidak dapat ditampilkan." plus path; tanpa tombol sunting.
- File tidak ditemukan: komponen kosong dengan pesan dan saran menutup tab; ditangani `FileTab` memakai `FileNotFoundError`.
- Encoding: konten diperlakukan sebagai UTF-8 sesuai keluaran daemon; tidak ada deteksi encoding lain.

Acceptance criteria:
- [ ] `.md` dan `.txt` dirender sebagai markdown; `.go`, `.js`, dan lainnya muncul sebagai teks polos monospace.
- [ ] Teks mentah yang berisi `<script>` atau HTML di markdown tampil sebagai teks (tidak dieksekusi); tautan `javascript:` tidak menjadi tautan aktif.
- [ ] File teks sangat besar (puluhan MB) dapat dibuka dan digulir tanpa seluruh isi dimuat ke DOM; jumlah node baris di DOM terbatas (di bawah ~200).
- [ ] File biner menampilkan `BinaryNotice` dan bukan isi.
- [ ] Hanya potongan yang dibutuhkan dibaca via `fs.read` pada tampilan teks polos (terlihat dari offset berurutan).
- [ ] Tidak ada dependency baru di `package.json`.
- [ ] `make build` lolos hanya dengan fitur ini terpasang.

ownedPaths:
- `frontend/src/modules/files/viewer/`

Paralel: dapat dikerjakan bersamaan dengan `file-tree-panel` dan `file-editor-save` selama signature di bagian Kontrak dipatuhi. `file-editor-save` memakai `readAll`, `MarkdownView`, `PlainTextView`, `BinaryNotice` dari sini; penulisnya boleh memakai stub sementara untuk berkembang, lalu mengganti dengan impor nyata.

### file-editor-save: Editor, simpan, dan konflik

Deskripsi: komponen tab `file` (`editor/FileTab.svelte`) yang merangkai viewer, mode sunting teks polos, Cmd+S, penanda belum disimpan, serta penanganan perubahan disk. Juga memiliki `index.ts` modul yang mendaftarkan panel dan tipe tab.

Detail:
- Alur buka: `fileKind(path)`; muat konten (`readAll` untuk markdown dan mode sunting; `PlainTextView` untuk teks polos baca-saja). Simpan `mtime` hasil baca terakhir sebagai `baseMtime`. `BinaryFileError` -> `BinaryNotice`; `FileNotFoundError` -> pesan tidak ditemukan.
- Mode: tombol "Edit" pada bilah file beralih ke mode sunting: `<textarea>` polos monospace (tanpa highlighting) dengan konten penuh yang dimuat via `readAll`. Untuk `.md`/`.txt` ada tombol ganti "Tampilan / Edit". Catatan: sunting file raksasa memuat seluruh isi ke memori; tampilkan peringatan satu baris bila `size` > 5 MiB sebelum masuk mode sunting dan biarkan pengguna memutuskan. Teks polos baca-saja tetap memakai render malas.
- Dirty: `dirty = (isi sekarang !== isi terakhir tersimpan/dibaca)`. Tampilkan "●" di bilah file dan kirim event `aiterm:tab-dirty` (lihat Kontrak) atau pakai API ui-shell bila ada. Menutup tab kotor: bila ui-shell menyediakan hook tutup-tab, minta konfirmasi; bila tidak ada, cukup penanda (jangan mengedit ui-shell).
- Cmd+S: listener `keydown` pada komponen (`metaKey && key === 's'`, `preventDefault`) yang aktif hanya saat tab ini fokus/terlihat. Memanggil `writeFile(path, content, baseMtime)`. Sukses: perbarui `baseMtime` dan `lastSaved`, hapus dirty, tampilkan "Tersimpan" singkat. Selama menyimpan, nonaktifkan tombol/handler ganda.
- Konflik (`ConflictError`/-32001): jangan menimpa. Tampilkan banner "File berubah di disk" dengan pilihan: "Muat ulang" (buang suntingan, baca ulang, perbarui `baseMtime`), "Timpa" (konfirmasi eksplisit: baca `mtime` terbaru lewat `readChunk(path,0,1)` lalu `writeFile` ulang dengan mtime itu), dan "Batal" (tetap dengan suntingan). Suntingan pengguna tidak hilang sebelum ia memilih.
- Perubahan eksternal: saat tab terlihat, `fs.watch {path}` dan subscribe `fs.changed.<path>` lewat `ipc.ts`. Saat tab disembunyikan/ditutup/komponen dihancurkan, `fs.unwatch` dan berhenti langganan (tidak ada watch untuk tab yang tak tampil). Saat event datang dengan mtime != `baseMtime`: bila tidak dirty, muat ulang otomatis diam-diam atau tawarkan "Muat ulang" (pilih tawaran banner agar pengguna sadar; tampilkan banner "File berubah di disk, Muat ulang"); bila dirty, tampilkan banner konflik yang sama dan jangan menyentuh suntingan. Event akibat simpan sendiri (mtime == `baseMtime` baru) diabaikan.
- `index.ts`: 
  ```ts
  import { registerTabType } from '../../lib/registry';
  import FileTab from './editor/FileTab.svelte';
  import { registerFilesPanel } from './tree/registerTree';
  registerTabType({ type: 'file', component: FileTab });
  registerFilesPanel();
  ```
  Sesuaikan path impor dan signature dengan `registry.ts` sebenarnya. Satu-satunya file di luar subfolder yang menjadi milik fitur ini adalah `index.ts`.
- Pesan dan label UI dalam bahasa Indonesia; identifier kode dalam bahasa Inggris.

Acceptance criteria:
- [ ] Mengetik di mode sunting menampilkan penanda "belum disimpan" di bilah file (dan event `aiterm:tab-dirty` bila API shell tidak ada); setelah simpan, penanda hilang.
- [ ] Cmd+S memanggil `fs.write` dengan `expectedMtime` = mtime hasil baca/simpan terakhir dan memperbarui mtime dari balasan.
- [ ] Bila file diubah di luar (mis. oleh agent) lalu pengguna menyimpan, muncul banner konflik dan file di disk TIDAK tertimpa; "Muat ulang" membaca ulang, "Timpa" hanya setelah konfirmasi.
- [ ] Perubahan eksternal pada tab yang terbuka memunculkan banner tawaran muat ulang; suntingan yang belum disimpan tidak hilang tanpa pilihan pengguna.
- [ ] `fs.watch` dipanggil hanya untuk tab yang tampil, dan `fs.unwatch` dipanggil saat tab ditutup/disembunyikan.
- [ ] Mode sunting tanpa syntax highlighting; tab `.md`/`.txt` dapat berganti antara tampilan markdown dan edit.
- [ ] `index.ts` mendaftarkan tab `file` dan panel "Files" tanpa mengedit file bersama; modul terbawa otomatis oleh `import.meta.glob`.
- [ ] `make build` lolos.

ownedPaths:
- `frontend/src/modules/files/editor/`
- `frontend/src/modules/files/index.ts`

Paralel: dapat dikerjakan bersamaan dengan fitur lain; bergantung pada signature `viewer/fileApi.ts`, komponen viewer, dan `tree/registerTree.ts` seperti di Kontrak. `index.ts` adalah titik temu terakhir: baru final setelah `tree/registerTree.ts` dan komponen viewer ada. Urutan merge yang disarankan: viewer dan tree lebih dulu (tanpa `index.ts`, build tetap lolos karena modul tanpa `index.ts` tidak dimuat), lalu editor beserta `index.ts`. Bila editor selesai lebih dulu, gunakan stub viewer/tree sementara yang nanti ditimpa fitur pemiliknya.

## Path yang dimiliki

Hanya path berikut yang boleh dibuat/diubah oleh epic ini:
- `frontend/src/modules/files/` (`index.ts`, `tree/`, `viewer/`, `editor/`)

Dilarang menyentuh: `frontend/package.json`, `frontend/vite.config.ts`, `frontend/tsconfig.json`, `frontend/src/main.ts`, `frontend/src/App.svelte`, `frontend/src/lib/` (registry, ipc, stores), `frontend/src/modules/shell/`, `terminal/`, `workspace/`, `frontend/src/styles/`, semua kode Go (`internal/`, `cmd/`, `app/`), dan path epic lain.

## Wiring

- Registrasi frontend: otomatis. `frontend/src/modules/files/index.ts` dimuat oleh `import.meta.glob`; memanggil `registerTabType({type:'file', ...})` dan `registerPanel({id:'files', area:'sidebar', ...})`. Tidak ada edit ke registry atau shell.
- Ke dalam: memakai `fs.list`, `fs.read`, `fs.write`, `fs.watch`, `fs.unwatch`, event `fs.changed.<path>` (epic file-service) lewat `frontend/src/lib/ipc.ts`; memakai store workspace aktif dan fungsi pembuka tab dari ui-shell (hanya impor/baca).
- Ke luar: tidak ada konsumen lain. Epic mention (wave 6) tidak memakai modul ini di versi awal (tombol "mention ke sesi" ditunda).
- Tidak ada langkah manual setelah merge selain `make build`.

## Guard

Cukup build berhasil, tanpa unit test:
- [ ] `go build ./...` (tidak berubah karena tidak ada kode Go yang disentuh, tetap harus lolos)
- [ ] `make build` (build frontend/Vite dan wails; modul `files` ikut terbundel)
- [ ] Opsional uji manual: jalankan daemon dan app, tambah project ke workspace, buka tree, buka `.md` (tampilan markdown) dan `.go` (teks polos), edit lalu Cmd+S, ubah file dari terminal lalu simpan lagi dan pastikan banner konflik muncul.
