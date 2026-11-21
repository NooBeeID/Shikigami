---
id: shikigami-v2-ui-ticket-03-pane-layout
title: Layout multi-pane (split, drag and drop, resize)
spec: docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md
status: implemented
wave: 3
depends_on: [shikigami-v2-ui-ticket-02-native-menu]
---
# Layout multi-pane (split, drag and drop, resize)

## Tujuan

Epic ini mengganti area tab v1 (`TabBar`/`TabContent`) dengan layout pohon pane (spec bagian 5): beberapa terminal terlihat bersamaan, bisa dibagi (split) horizontal/vertikal, disusun ulang dengan drag and drop, diubah ukurannya dengan menyeret pembatas, dan dimaksimalkan sementara. Pane juga bisa berisi file (viewer/editor dari tiket 07 v1), bukan hanya terminal. Alasannya: tujuan v2 adalah mengawasi banyak agent dalam satu layar, dan ini inti perubahan UI-nya. Epic ditutup dengan pengukuran ulang RAM/CPU dengan 6 pane terlihat (gerbang spec bagian 8), karena asumsi v1 "hanya tab aktif yang punya instance xterm" tidak berlaku lagi.

Anti-goal yang harus dijaga (spec bagian 3 dan 5): memindah atau men-split pane TIDAK boleh membuat ulang xterm atau memutus attach ke sesi. Yang dipindah adalah elemen DOM yang sama.

## Hasil yang bisa dijalankan

Setelah tiket ini selesai (di atas tiket 02 native-menu), pengguna bisa:

- Membuka beberapa sesi dan satu file sekaligus di area tengah, semuanya terlihat bersamaan sebagai pane.
- Menekan Cmd+D (bagi kanan) atau Cmd+Shift+D (bagi bawah): pane baru dengan sesi baru di project dan provider yang sama muncul di samping pane yang fokus.
- Menyeret header pane ke pane lain: zona tepi atas/bawah/kiri/kanan membagi pane target di sisi itu, zona tengah menukar posisi, dengan overlay penanda zona saat menyeret. Terminal tetap hidup dan scrollback tidak hilang setelah dipindah.
- Menyeret pembatas antar pane untuk mengubah ukuran; terminal ikut menyesuaikan kolom/baris.
- Memaksimalkan satu pane (tombol di header atau klik ganda header) lalu memulihkannya tanpa mengubah layout.
- Memakai menu `…` pada header: akhiri sesi, ganti nama, susun grid. Ganti nama juga lewat klik ganda judul.
- Menutup pane tanpa membunuh sesi (sesi tetap hidup di daemon dan tetap muncul di sidebar v1); "Akhiri sesi" yang membunuhnya.
- Klik sesi/file di sidebar v1 (`WorkspaceSidebar`, `FilesPanel`) membuka atau memfokuskan pane-nya.
- Pane yang fokus diberi bingkai beraksen; header menampilkan nomor, judul, lencana provider, titik status, dan chip path project.
- Laporan `docs/perf/multipane-report.md` berisi angka RAM/CPU UI dengan 6 pane terlihat.

`make build` lulus dan aplikasi tetap bisa dijalankan. Tiket ini tidak bergantung pada tiket 04 sampai 07 untuk jalan: layout tinggal di `App.svelte`/shell v1 dan sidebar v1 dipakai apa adanya. Layout belum disimpan ke disk (itu tiket 06 workspace-persist); `hydrate`/`serialize` hanya disediakan.

## Batasan

In scope:
- Model pohon layout dan store (aksi murni, serializable, group-ready), registri jenis pane `registerPaneKind`.
- Renderer layout (flex), splitter dengan pointer events, maksimalkan/pulihkan, bingkai fokus, pintasan Cmd+D dan Cmd+Shift+D.
- Header pane, menu `…`, drag and drop zona buatan sendiri (tanpa library DnD), overlay zona.
- Adapter pane `terminal` dan `file`; opsi tambahan di `modules/terminal` (scrollback, tanpa WebGL, mode tersembunyi) tanpa mengubah perilaku default untuk pemakai lain.
- Mengganti `TabBar`/`TabContent` dengan `LayoutArea` di area tengah; `modules/tabs` jadi shim atau pensiun.
- Pengukuran ulang memori 6 pane dan laporan di `docs/perf/`.

Out of scope:
- Penyimpanan layout ke `workspaces.json`, IPC `workspace.layout.get/set`, pemulihan saat dibuka (tiket 06). Tiket ini hanya menyediakan `hydrate(doc)` dan `serialize()`.
- Tab grup sesi dan banyak dokumen layout per grup di UI (tiket 07); model di sini hanya group-ready: satu dokumen layout, store bisa diinstansiasi per grup, v2 awal memakai satu grup "Utama".
- Bilah atas, sidebar kiri/kanan baru, palet perintah, notifikasi (tiket 04). Sidebar v1 tetap dipakai.
- Menu native, zoom, open folder (tiket 02). Tiket ini hanya MEMAKAI event zoom bila sudah ada (lihat Wiring).
- Kode Go/daemon apa pun. Perubahan `package.json`/dependency npm baru (drag and drop dan splitter dibuat sendiri).
- Unit test (guard hanya `make build`).
- Keputusan opsi UI native (spec v1 bagian 9b opsi c): laporan perf hanya melaporkan angka, keputusan di pembuat.

## Kontrak

### Yang dipakai dari tiket lain

Baca kode aslinya sebelum menulis; nama di bawah adalah pegangan, identifier sebenarnya mengikuti repo.

- `frontend/src/lib/registry.ts` (v1 app-base): `registerPanel`, `registerTabType`. Tiket ini menambah registri baru sendiri `registerPaneKind` di `modules/layout/registry.ts` dan tidak mengubah `lib/registry.ts`.
- `frontend/src/lib/ipc.ts`: pembungkus `Call(method, params)` dan langganan event. Dipakai untuk `session.kill` dan `session.create` (nama sebenarnya cek `lib/stores/sessions.ts` dan `terminal-client.ts`).
- `frontend/src/lib/stores/` (sessions, workspaces, providers, types): daftar sesi, project aktif, provider. Hanya dibaca dan diimpor.
- `modules/status` (v1 tiket 08): `StatusBadge`/store status per sesi untuk titik status header. Hanya dibaca.
- `modules/terminal` (`TerminalTab.svelte`, `terminal-client.ts`): dipakai ulang sebagai isi pane terminal; hanya ditambah opsi (lihat `pane-kinds`).
- `modules/files` (`FileTab` lewat `registerTabType({type:'file'})`, dan `FilesPanel` di `tree/`): dipakai ulang sebagai isi pane file. Bentuk data tab file v1: `{path, projectId, workspaceId}`.
- Event menu native tiket 02 (kontrak bersama 3): `menu:zoom` {action:'in'|'out'|'reset'} lewat event bus `modules/appmenu`. Ukuran font terminal mengikuti skala zoom dari modul itu bila tersedia (lihat Wiring); tiket ini tidak boleh gagal build bila modul itu berbeda bentuk.
- Kontrak bersama 2: flag izin agent tetap dipertahankan dan tidak digandakan. Sesi baru dari split dibuat lewat jalur pembuatan sesi v1 yang sama, sehingga flag tetap diterapkan oleh `internal/provider`; frontend tidak menambah argumen provider.

### Yang disediakan

Tipe pohon (kontrak bersama 4; `frontend/src/modules/layout/model.ts`):

```ts
export type PaneKind = 'terminal' | 'file';           // string terbuka lewat registerPaneKind
export interface Leaf {
  type: 'pane';
  id: string;                  // stabil, mis. `p_<nanoid>`; dipakai sebagai kunci host DOM
  kind: PaneKind;
  sessionId?: string;          // kind 'terminal'
  projectId?: string;
  provider?: string;           // 'claude' | 'codex' | 'shell'
  filePath?: string;           // kind 'file'
  title: string;
}
export interface Split {
  type: 'split';
  dir: 'h' | 'v';              // 'h' = a kiri / b kanan, 'v' = a atas / b bawah
  ratio: number;               // 0..1, porsi a; di-clamp ke [0.1, 0.9]
  a: Node;
  b: Node;
}
export type Node = Leaf | Split;
export interface LayoutDoc {
  version: 1;
  root: Node | null;
  focusId?: string;
  maximizedId?: string | null;
}
export type DropZone = 'top' | 'bottom' | 'left' | 'right' | 'center';
```

Store (`frontend/src/lib/stores/layout.ts`), aksi murni atas `LayoutDoc` (fungsi tanpa efek samping di `model.ts`, dibungkus store Svelte):

| Aksi | Perilaku |
|---|---|
| `openPane(leaf)` | Bila sesi/file yang sama sudah ada di pohon: fokuskan dan kembalikan id. Bila belum: sisipkan di sebelah pane fokus (split sisi kanan, atau bawah bila `dir` bawaan 'v'); pohon kosong: jadi root. Mengembalikan id pane. |
| `splitPane(paneId, dir, newLeaf, side?)` | Ganti leaf target dengan Split berisi leaf lama dan `newLeaf`; `side` 'after' (bawaan) atau 'before'. Ratio 0.5. |
| `closePane(paneId)` | Hapus leaf; saudaranya menggantikan induk Split. TIDAK membunuh sesi. Bila yang ditutup pane maksimal, `maximizedId` dibersihkan. Fokus pindah ke saudara terdekat. |
| `movePane(paneId, targetId, zone)` | Cabut leaf dari posisi lama (tanpa mengubah objek leaf, id sama) lalu: `center` = `swapPanes`; `top/bottom/left/right` = bagi target di sisi itu. Dilarang bila `paneId === targetId`. |
| `swapPanes(aId, bId)` | Tukar isi dua leaf di posisi masing-masing (id ikut berpindah). |
| `resize(splitRef, ratio)` | Ubah `ratio` simpul Split (dirujuk lewat jalur indeks dari root, mis. `'a.b'`), di-clamp. |
| `maximize(paneId)` / `restore()` | Set/hapus `maximizedId`; pohon tidak berubah. |
| `tidyGrid()` | Susun ulang semua leaf (urutan baca kiri-atas ke kanan-bawah saat ini) menjadi grid otomatis: n leaf menjadi `cols = ceil(sqrt(n))` kolom, baris sisanya; contoh 6 menjadi 3x2, 4 menjadi 2x2, 5 menjadi 3+2. Id leaf dipertahankan. |
| `setFocus(paneId)` | Set `focusId` bila id ada. |
| `renamePane(paneId, title)` | Ubah `title` leaf (kosong dianggap batal). |
| `hydrate(doc)` / `serialize()` | Muat/ambil `LayoutDoc` (kontrak bersama 4); `hydrate` memvalidasi bentuk dan membuang leaf tidak valid; tidak melempar. Untuk dipakai tiket 06. |
| `paneCount`, `leaves()` | Derived: jumlah dan daftar leaf berurutan (untuk nomor pane 1..n). |

Fabrik: `createLayoutStore(initial?: LayoutDoc)` mengembalikan satu store terpisah (group-ready: tiket 07 membuat satu per grup); `layout` adalah instans bawaan untuk grup "Utama". Semua struktur adalah JSON murni (tanpa fungsi, tanpa referensi DOM).

Registri jenis pane (`frontend/src/modules/layout/registry.ts`):

```ts
export interface PaneKindSpec {
  kind: string;
  component: Component<{ pane: Leaf; visible: boolean }>;  // isi pane; `visible` false saat tersembunyi/diminimalkan
  titleFor?: (leaf: Leaf) => string;                        // judul bawaan
  onEndSession?: (leaf: Leaf) => Promise<void>;             // dipakai aksi "Akhiri sesi" (terminal: session.kill)
  badgeFor?: (leaf: Leaf) => string | null;                 // teks lencana (provider)
}
export function registerPaneKind(spec: PaneKindSpec): void;
export function paneKinds(): Readable<Record<string, PaneKindSpec>>;
```

Komponen dan modul pendukung:
- `LayoutArea.svelte` (props: `store` bawaan `layout`), `Splitter.svelte`, `shortcuts.ts`.
- `PaneHeader.svelte`, `PaneMenu.svelte`, `dnd.ts` (`startPaneDrag`, `zoneAt(rect, x, y)`), `DropOverlay.svelte`.
- `modules/layout/panes/terminal.ts` dan `file.ts` (adapter, mendaftar lewat `registerPaneKind`), `panes/hosts.ts` (registri host DOM per pane).

Host DOM stabil (wajib): `hosts.ts` memegang `Map<paneId, HTMLElement>`. Isi pane (komponen jenis pane) di-`mount` SEKALI ke elemen host milik pane itu dan baru di-`unmount` saat pane dihapus dari pohon (bukan saat berpindah posisi). Renderer hanya memindahkan elemen host dengan `appendChild` ke slot posisi barunya. Dengan begitu instance xterm tidak dibuat ulang saat split/drag/maksimalkan.

## Fitur

Urutan: `layout-model` dikerjakan lebih dulu (foundation, tipe dan aksi dipakai semua fitur). `layout-renderer`, `pane-header-dnd`, dan `pane-kinds` lalu bisa paralel karena berkas dan antarmuka sudah terkunci di Kontrak. `layout-wiring` dan `perf-6-pane` dikerjakan terakhir, berurutan.

### layout-model: Foundation: model pohon layout, store, dan aksi

Deskripsi: tanpa UI. Implementasi `model.ts` (tipe di Kontrak, fungsi murni atas `LayoutDoc`, tidak memutasi masukan: kembalikan dokumen baru), `registry.ts` (`registerPaneKind`, `paneKinds`), dan `lib/stores/layout.ts` (`createLayoutStore`, instans `layout`, pembungkus `writable` yang memanggil fungsi murni). Detail aturan: id leaf dibuat sekali (`crypto.randomUUID()` atau penghitung) dan tidak berubah saat pindah; `ratio` di-clamp ke [0.1, 0.9]; pohon tidak boleh punya Split dengan anak kosong (normalisasi setelah `closePane`/`movePane`); `movePane` ke pane maksimal memulihkan maksimal lebih dulu; `tidyGrid` memakai pohon seimbang dari kolom/baris sehingga ratio tiap simpul sama rata (mis. kolom ke-i dari n memakai ratio `1/(n-i)`). Sertakan `findLeaf`, `pathOf`, `leaves` sebagai helper bersama.

Acceptance criteria:
- [ ] Tipe `Leaf`, `Split`, `Node`, `LayoutDoc`, `DropZone` persis seperti Kontrak dan diekspor dari `model.ts`.
- [ ] Semua aksi di tabel Kontrak ada dan murni (masukan tidak dimutasi); `closePane` tidak memanggil IPC apa pun.
- [ ] `splitPane`, `movePane` (4 zona), `swapPanes`, `closePane` menghasilkan pohon valid (tanpa Split berkaki satu) dan tidak mengubah `id` leaf yang dipindah.
- [ ] `tidyGrid` untuk n=1..9 menghasilkan grid sesuai aturan (6 menjadi 3x2) dengan id leaf dipertahankan.
- [ ] `serialize()` menghasilkan JSON murni; `hydrate(serialize())` menghasilkan pohon yang sama; `hydrate` pada data rusak tidak melempar dan menghasilkan dokumen kosong/valid.
- [ ] `registerPaneKind` bisa dipanggil dari modul lain; kind tak terdaftar tidak membuat renderer crash (fallback "Jenis pane tidak dikenal").
- [ ] `createLayoutStore()` menghasilkan store independen (group-ready); `layout` adalah instans bawaan.
- [ ] `make build` lulus (type-check lulus).

ownedPaths: `frontend/src/modules/layout/model.ts`, `frontend/src/modules/layout/registry.ts`, `frontend/src/lib/stores/layout.ts`.

Paralel: ini foundation; fitur lain di epic ini menunggu tipe/antarmukanya (cukup berkas ini selesai atau kontraknya diikuti).

### layout-renderer: Renderer layout, split, resize, dan maksimalkan

Deskripsi: `LayoutArea.svelte` merender pohon secara rekursif dengan CSS flex (`flex-direction` row untuk `h`, column untuk `v`; anak `a` memakai `flex: ratio`, `b` memakai `flex: 1 - ratio`, `min-width/height: 0`). Tiap leaf punya slot kosong yang diisi elemen host stabil dari `panes/hosts.ts` lewat `appendChild` (action Svelte yang memasang host saat slot muncul dan hanya melepas, bukan menghancurkan, saat slot hilang). Pane dihancurkan (`unmount` + hapus host) hanya bila id-nya tidak lagi ada di pohon. `Splitter.svelte` adalah pembatas 4px (hit area lebih lebar) dengan pointer events (`pointerdown` + `setPointerCapture`, `pointermove` menghitung ratio dari rect induk dengan `requestAnimationFrame`, `pointerup` selesai), kursor `col-resize`/`row-resize`, klik ganda mengembalikan 0.5; tanpa library. Selama drag, `resize` dipanggil terus tetapi `fit` terminal di-debounce (lewat `ResizeObserver` di host, bukan polling). Maksimalkan: bila `maximizedId` ada, hanya pane itu ditampilkan (yang lain tetap ter-mount, disembunyikan dengan `display:none` dan `visible=false`), pohon tidak berubah; dapat dipicu tombol dan klik ganda header (disambungkan dari header lewat callback `onMaximizeToggle`). Pane fokus diberi bingkai aksen (`outline`/`box-shadow` memakai variabel CSS `--accent`); klik di dalam pane memanggil `setFocus`. `shortcuts.ts`: pendengar `keydown` pada window untuk Cmd+D (bagi kanan) dan Cmd+Shift+D (bagi bawah) yang membuat sesi baru dengan project dan provider pane fokus, lalu `splitPane` dengan leaf baru; tidak aktif bila fokus ada di input teks non-terminal; Cmd+D di dalam xterm juga ditangkap (cegah `preventDefault` agar tidak diteruskan ke shell). Daftar pintasan (Cmd+D, Cmd+Shift+D) tidak bentrok dengan pintasan menu tiket 02 (Cmd+O, Cmd+=, Cmd+-, Cmd+0, Cmd+B).

Acceptance criteria:
- [ ] `LayoutArea` merender pohon sembarang kedalaman dengan arah dan ratio yang benar; pohon kosong menampilkan keadaan kosong (teks petunjuk).
- [ ] Pembatas bisa diseret dengan mouse, ratio ter-clamp [0.1, 0.9], terminal di kedua sisi menyesuaikan ukuran setelah drag tanpa kedip berlebih; klik ganda pembatas mengembalikan 0.5.
- [ ] Memindahkan pane (mengubah posisi di pohon) tidak membuat ulang isi pane: host DOM yang sama dipindah (diperiksa lewat identitas elemen host dan isi terminal tidak ter-reset).
- [ ] Maksimalkan/pulihkan bekerja lewat store; pane lain tetap ter-mount dan pohon tidak berubah setelah pulihkan.
- [ ] Pane fokus punya bingkai aksen; klik pane mengubah fokus.
- [ ] Cmd+D dan Cmd+Shift+D membagi pane fokus ke kanan/bawah dengan sesi baru di project dan provider yang sama; tidak menggandakan flag izin agent (jalur pembuatan sesi v1 dipakai apa adanya).
- [ ] Tanpa polling dan tanpa dependency baru; `make build` lulus.

ownedPaths: `frontend/src/modules/layout/LayoutArea.svelte`, `frontend/src/modules/layout/Splitter.svelte`, `frontend/src/modules/layout/shortcuts.ts`.

Paralel: bisa dikerjakan paralel dengan `pane-header-dnd` dan `pane-kinds` setelah `layout-model`. Kontrak antarmuka dengan header: `LayoutArea` menyusun `PaneHeader` di atas isi pane dengan props `{pane, index, focused, maximized, onSplit, onMaximizeToggle, onClose, onRename, onEndSession, onTidy}`; sampai `PaneHeader` jadi, pakai header sementara internal di berkas sendiri yang dibuang saat digabung.

### pane-header-dnd: Header pane dan drag and drop zona

Deskripsi: `PaneHeader.svelte` menampilkan nomor pane (indeks 1..n dari `leaves()`), judul (klik ganda masuk mode ganti nama inline, Enter simpan, Esc batal), lencana provider (CL/CX/SH dari `badgeFor`), titik status (komponen/store `modules/status` per `sessionId`; pane file tanpa titik), chip path project (nama project dan subfolder relatif ke root project, dipotong dengan ellipsis, tooltip path penuh), tombol split kanan dan bawah, maksimalkan/pulihkan, tutup, dan tombol menu `…`. `PaneMenu.svelte` (popover kecil, ditutup saat klik di luar/Esc) berisi: Akhiri sesi (hanya terminal; konfirmasi singkat; memanggil `onEndSession` jenis pane), Ganti nama, Susun grid, dan Tutup pane. `dnd.ts`: header diseret dengan pointer events (`pointerdown` di header di luar tombol, ambang gerak 4px agar klik ganda tidak memulai drag, `setPointerCapture`), selama drag memakai `document.elementFromPoint` untuk menemukan pane target dan `zoneAt(rect, x, y)` untuk menentukan zona: bila titik di 25% tepi terdekat maka zona sisi itu, selain itu `center`; elemen hantu (ghost) kecil berisi judul mengikuti pointer; `Esc` membatalkan; lepas di zona memanggil `movePane`. `DropOverlay.svelte` menggambar sorotan zona (setengah pane untuk sisi, seluruh pane untuk center) di atas pane target, `pointer-events: none`. Overlay hanya ditampilkan selama drag; terminal tidak boleh menelan pointer event selama drag (host diberi `pointer-events: none` sementara). Kode sendiri tanpa library DnD.

Acceptance criteria:
- [ ] Header menampilkan nomor, judul, lencana provider, titik status (terminal), chip path project, dan tombol split/maksimalkan/tutup/`…`.
- [ ] Ganti nama inline dan lewat menu; judul kosong tidak mengubah apa-apa.
- [ ] Menu `…` memuat Akhiri sesi, Ganti nama, Susun grid, Tutup; menu tertutup oleh klik di luar dan Esc.
- [ ] Klik ganda area header (bukan judul) memicu maksimalkan/pulihkan; klik ganda judul memicu ganti nama.
- [ ] Drag header menampilkan ghost dan overlay zona yang benar: tepi atas/bawah/kiri/kanan (25%) membagi, tengah menukar; Esc membatalkan; lepas di luar pane membatalkan.
- [ ] `zoneAt` deterministik untuk titik di sudut (pilih sisi dengan jarak tepi terkecil).
- [ ] Setelah drop, sesi terminal tidak ter-reset (diverifikasi bersama `layout-renderer`).
- [ ] Tanpa library DnD baru; `make build` lulus.

ownedPaths: `frontend/src/modules/layout/PaneHeader.svelte`, `frontend/src/modules/layout/PaneMenu.svelte`, `frontend/src/modules/layout/dnd.ts`, `frontend/src/modules/layout/DropOverlay.svelte`.

Paralel: bisa paralel dengan `layout-renderer` dan `pane-kinds` setelah `layout-model`; hanya bergantung pada props yang dikunci di Kontrak/`layout-renderer`.

### pane-kinds: Adapter pane terminal dan pane file

Deskripsi: daftarkan jenis pane lewat `registerPaneKind` di `modules/layout/panes/`. (1) `terminal`: komponen pembungkus `TerminalPane.svelte` yang merender `TerminalTab` dengan `sessionId` dan prop `visible`; `titleFor` memakai judul sesi/nama provider; `badgeFor` memakai provider; `onEndSession` memanggil `session.kill` lewat IPC (menutup pane tetap tidak membunuh sesi). (2) `file`: pembungkus `FilePane.svelte` yang merender `FileTab` dari `modules/files` dengan `{path, projectId, workspaceId}` dari leaf; judul bawaan = basename; tidak punya titik status. Perluasan `modules/terminal` HANYA lewat penambahan opsi `TerminalTab` (props opsional, default menjaga perilaku v1): `scrollback` (bawaan 2000 untuk pemanggil lama; pane terminal memakai 1000), `webgl` (tidak ada addon WebGL dimuat; cukup pastikan tidak ada import WebGL), `visible` (bila false: `ResizeObserver`/fit dijeda dan penulisan ke xterm di-batch sampai terlihat, atau render dihentikan, tetapi instance `Terminal` dan attach ke sesi TIDAK dihancurkan), dan `fontSize` reaktif (untuk zoom tiket 02; bila belum ada, tetap 13). Terminal yang `visible` kembali true melakukan `fit` sekali dan `refresh`. Perluasan ini disebut di Wiring. Pendaftaran dilakukan dari `modules/layout/index.ts` (diimpor otomatis oleh `import.meta.glob` v1) agar jenis pane tersedia tanpa mengubah modul lain.

Acceptance criteria:
- [ ] `registerPaneKind` dipanggil untuk `terminal` dan `file` dan keduanya muncul di `paneKinds()`.
- [ ] Pane terminal membuat xterm dengan `scrollback: 1000` dan tanpa addon WebGL (grep tidak menemukan import WebGL di `modules/terminal` dan `modules/layout`).
- [ ] Pane tersembunyi/diminimalkan (`visible=false`) tidak merender/fit tetapi instance xterm dan attach sesi tetap ada; saat terlihat lagi isinya utuh (termasuk output yang datang saat tersembunyi).
- [ ] `TerminalTab` bila dipakai tanpa opsi baru berperilaku sama seperti v1 (scrollback 2000, sudah tidak dipakai langsung setelah wiring tetapi tetap valid).
- [ ] "Akhiri sesi" memanggil `session.kill` untuk `sessionId` pane dan pane menampilkan keadaan sesi berakhir yang sudah ada di `TerminalTab`; "Tutup" tidak memanggil `session.kill`.
- [ ] Pane file membuka `FileTab` dengan file yang benar; perubahan belum tersimpan tidak hilang saat pane dipindah (isi komponen tidak dibuat ulang).
- [ ] `make build` lulus.

ownedPaths: `frontend/src/modules/layout/panes/`, `frontend/src/modules/layout/index.ts`, `frontend/src/modules/terminal/` (hanya penambahan opsi).

Paralel: bisa paralel dengan `layout-renderer` dan `pane-header-dnd` setelah `layout-model`. Berkas `modules/terminal/*` hanya disentuh fitur ini.

### layout-wiring: Wiring: ganti TabContent dengan LayoutArea

Deskripsi: langkah penutup penyambungan. Lihat bagian Wiring; fitur ini adalah eksekusinya (App.svelte/shell memakai `LayoutArea`, `modules/tabs` jadi shim atau pensiun, sidebar v1 membuka sesi/file lewat `openPane`). Dikerjakan setelah semua fitur di atas selesai.

Acceptance criteria:
- [ ] Area tengah aplikasi memakai `LayoutArea`; `TabBar`/`TabContent` tidak lagi dirender.
- [ ] Klik sesi di `WorkspaceSidebar` memanggil `openPane({kind:'terminal', sessionId, ...})`: pane baru di layout atau fokus ke pane yang sudah ada.
- [ ] Klik file di `FilesPanel` memanggil `openPane({kind:'file', filePath, projectId, ...})`.
- [ ] Beberapa terminal dan satu file terlihat bersamaan; split, drag, resize, maksimalkan, dan susun grid bekerja dalam aplikasi nyata.
- [ ] Sesi baru (tombol `+ sesi` v1) otomatis muncul sebagai pane.
- [ ] Sesi yang berakhir/dihapus: pane tampil keadaan berakhir; menutup pane tidak membunuh sesi.
- [ ] `go build ./...` dan `make build` lulus; aplikasi berjalan.

ownedPaths: `frontend/src/App.svelte`, `frontend/src/modules/tabs/`, `frontend/src/modules/shell/`.

Paralel: tidak; menunggu `layout-model`, `layout-renderer`, `pane-header-dnd`, `pane-kinds`.

### perf-6-pane: Ukur ulang RAM/CPU dengan 6 pane terlihat

Deskripsi: fitur terakhir. Bangun (`make build`), buat 6 sesi terminal (provider `shell`, sama seperti spike v1) lewat `shikigamictl session new` atau UI, jalankan aplikasi dengan data dir sementara (`mktemp -d` via variabel data dir yang dipakai daemon), tampilkan keenamnya sekaligus sebagai grid 3x2 (aksi "Susun grid"), lalu ukur dengan prosedur yang sama seperti `docs/perf/spike-report.md`: RSS lewat `ps -o rss= -p PID` (KB dibagi 1024), hanya PID `com.apple.WebKit*` baru milik aplikasi (bandingkan daftar sebelum/sesudah dijalankan), jeda 5 detik setelah perubahan keadaan, CPU idle dari selisih `ps -o time=` dibagi selisih waktu dinding (interval 10 detik), tanpa kolom `%cpu`. Ukur setidaknya: UI dengan 1 pane, UI dengan 6 pane idle, dan UI dengan 6 pane saat ada output ringan (mis. `yes | head -n 20000` di tiap pane lalu idle lagi). Bandingkan dengan target 200 MB dan ambang 1,5x (300 MB) serta dengan baseline v1 187,19 MB. Tulis `docs/perf/multipane-report.md` (bahasa Indonesia) dengan: ringkasan, metadata mesin dan build (OS, arsitektur, versi Go, versi xterm, waktu, commit), kondisi lingkungan (load average, proses berat lain), metode dan perintah, tabel data mentah (PID, RSS KB/MB, CPU time awal/akhir, delta wall, CPU idle), tabel pembanding terhadap target, keterbatasan, dan kesimpulan gerbang. Bila hasil melewati 300 MB, tulis jelas bahwa keputusan opsi UI native (spec v1 bagian 9b opsi c) dibawa ke pembuat; laporan hanya melaporkan, tidak memutuskan. Bersihkan semua proses dan direktori sementara setelah ukur.

Acceptance criteria:
- [ ] `docs/perf/multipane-report.md` ada dengan semua bagian di atas dan angka hasil ukur nyata (bukan estimasi); bila ada angka estimasi, diberi label ESTIMASI.
- [ ] Prosedur sama dengan spike v1 dan perbedaannya (bila ada) dicatat.
- [ ] Tabel pembanding memuat rasio terhadap 200 MB dan 300 MB serta baseline 187,19 MB, dengan status LULUS/LEWAT.
- [ ] Laporan menyatakan konfigurasi mitigasi yang aktif saat ukur (scrollback 1000, tanpa WebGL, pane tersembunyi tidak merender).
- [ ] Data dir pengguna nyata tidak tersentuh; proses dan direktori sementara dibersihkan.
- [ ] Tidak ada keputusan opsi UI native di laporan, hanya rekomendasi data.

ownedPaths: `docs/perf/multipane-report.md`.

Paralel: tidak; dikerjakan terakhir setelah `layout-wiring` karena butuh aplikasi yang jalan.

## Path yang dimiliki

Hanya path berikut yang boleh disentuh oleh tiket ini:

- `frontend/src/modules/layout/` (termasuk `model.ts`, `registry.ts`, `index.ts`, `LayoutArea.svelte`, `Splitter.svelte`, `shortcuts.ts`, `PaneHeader.svelte`, `PaneMenu.svelte`, `dnd.ts`, `DropOverlay.svelte`, `panes/`)
- `frontend/src/lib/stores/layout.ts`
- `frontend/src/modules/tabs/` (jadi shim atau pensiun)
- `frontend/src/modules/terminal/` (hanya penambahan opsi)
- `frontend/src/App.svelte` dan `frontend/src/modules/shell/` (hanya untuk Wiring, perluasan minimal)
- `docs/perf/`

Pembacaan (tanpa ubah) boleh ke `frontend/src/lib/` lain, `modules/files`, `modules/status`, `modules/workspace`, `modules/appmenu`. Tiket 04 memiliki `modules/shell/` secara penuh setelah tiket ini: perubahan di sini ke `shell/` dan `App.svelte` dibatasi pada mengganti area tab dengan `LayoutArea` dan harus disebut di bagian Wiring.

## Wiring

Langkah penutup tiket ini (fitur `layout-wiring`), memakai titik ekstensi yang ada:

1. `modules/layout/index.ts` (dimuat otomatis oleh `import.meta.glob` v1) memanggil `registerPaneKind` untuk `terminal` dan `file`. Tidak ada perubahan di `lib/registry.ts`.
2. Perluasan file tiket lain (wajib disebut): (a) `modules/terminal/TerminalTab.svelte` ditambah props opsional `scrollback`, `visible`, `fontSize` (default menjaga perilaku v1); (b) `App.svelte` dan `modules/shell/` mengganti `TabBar`/`TabContent` dengan `<LayoutArea />` sebagai area tengah, dengan perubahan sekecil mungkin agar tiket 04 bisa merombak shell sesudahnya.
3. Sidebar v1: `WorkspaceSidebar`/`SessionList` memanggil `openPane` (dari `lib/stores/layout.ts`) saat sesi diklik, alih-alih `openTab`; `FilesPanel` memanggil `openPane({kind:'file', ...})`. Karena kedua komponen berada di `modules/workspace` dan `modules/files/tree` (bukan milik tiket ini), perubahan dilakukan tanpa menyentuh berkasnya: `modules/tabs` dipertahankan sebagai shim dengan `openTab`/`activateTab`/`closeTab` yang sama dan diteruskan ke `openPane`/`setFocus`/`closePane` (sesi `refId` menjadi `sessionId`, tab `file` menjadi pane file). Dengan shim ini kode sidebar v1 tidak perlu diubah dan `lib/stores/tabs.ts` tetap menjadi satu-satunya antarmuka yang mereka kenal. Bila ternyata `openTab` dipanggil dari tempat yang tidak bisa diteruskan lewat shim, hentikan dan laporkan sebagai blocker, bukan mengedit berkas di luar `ownedPaths`.
4. Sesi baru: setelah sesi dibuat lewat `+ sesi` v1, shim memanggil `openPane`; Cmd+D/Cmd+Shift+D membuat sesi lewat jalur yang sama.
5. Zoom (opsional, hanya bila tiket 02 sudah menyediakan skala): `TerminalTab` membaca skala dari event bus `modules/appmenu` (event `menu:zoom`) untuk `fontSize`; jangan menambah handler `menu:zoom` kedua. Bila modul belum tersedia, `fontSize` tetap 13.
6. Terlihat: jalankan aplikasi, buka 6 sesi shell dan satu file, pastikan split, seret, resize, maksimalkan, dan "Susun grid" bekerja dan terminal tidak ter-reset saat dipindah. Tiket ini tidak bergantung pada tiket 04 sampai 07 untuk bisa dijalankan.
7. Terakhir jalankan `perf-6-pane` dan tulis `docs/perf/multipane-report.md`.

## Guard

Cukup build berhasil, tanpa unit test:

- `make build` (satu-satunya guard; mencakup `go build ./...` dan build frontend termasuk type-check Svelte).
- Aplikasi hasil build bisa dijalankan dan menampilkan beberapa pane terminal sekaligus.
- `grep -rn "webgl" frontend/src/modules/terminal frontend/src/modules/layout` tidak menemukan import addon WebGL.
