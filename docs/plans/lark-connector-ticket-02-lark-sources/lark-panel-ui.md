# Plan: LarkPanel, sumber, dan dialog Tambah

Baca `overview.md` (bagian 2.4, 2.5, 2.6, 6, 7) dan `00-foundation.md` lebih dulu; plan ini tidak mengulang isinya.

## Metadata
- Tiket: `lark-connector-ticket-02-lark-sources`
- Fitur: `lark-panel-ui`
- ownedPaths:
  - `frontend/src/modules/lark/LarkPanel.svelte`
  - `frontend/src/modules/lark/LarkIcon.svelte`
  - `frontend/src/modules/lark/client.ts`
  - `frontend/src/modules/lark/store.ts`
  - `frontend/src/modules/lark/AddSourceDialog.svelte`
  - `frontend/src/modules/lark/sections/` (`TicketSection.svelte`, `WikiSection.svelte`, `WikiTree.svelte`)
- Bergantung pada Foundation: ya. Yang dipakai dari Foundation: tipe `PaneKind` `'lark-board' | 'lark-doc'` serta field `sourceId`/`nodeToken` pada `NewLeaf` di `frontend/src/modules/layout/model.ts` (agar `layout.openPane({ kind: 'lark-board', sourceId, title })` lulus type-check). Bila Foundation belum diterapkan di pohon kerja, hentikan dan laporkan; jangan mengedit `model.ts`.
- Paralel dengan fitur lain: hanya memanggil NAMA method IPC kontrak (overview 2.5), bukan implementasi Go. Verifikasi end-to-end menunggu wiring.

## Tujuan
Membangun UI panel Lark di `frontend/src/modules/lark/`: klien IPC bertipe, store sumber per workspace aktif (tanpa polling), ikon, `LarkPanel` dengan bagian lipat Tiket dan Wiki, dialog Tambah (tempel link), serta `WikiTree` placeholder yang bisa ditimpa tiket 04 tanpa mengubah `WikiSection`. Fitur ini TIDAK mendaftarkan rail item dan TIDAK menyentuh `index.ts`.

## Path yang dimiliki
HANYA path di ownedPaths di atas yang boleh dibuat/diubah. Jangan sentuh: `frontend/src/modules/lark/index.ts`, `source.ts`, `actions.ts`, `board/*`, `doc/*` (milik `lark-pane-foundation`/wiring/tiket lain), `frontend/src/modules/layout/*`, `frontend/src/lib/stores/layout.ts`, `frontend/package.json`, kode Go, `cmd/*`, modul `settings`/`shell`. Jangan stage/commit perubahan tak terkait di working tree.

## Kontrak
Dipakai (nama persis, sudah ada di repo):
- `call`, `onEvent` dari `frontend/src/lib/ipc.ts` (`call<T>(method, params)`; galat dilempar sebagai `Error` yang `message`-nya string galat mentah, mis. `"[invalid_link] Link tidak dikenali (kode -32602)"`).
- `activeWorkspaceId` (Writable<string | null>) dari `frontend/src/lib/stores/workspaces.ts`.
- `layout` dari `frontend/src/lib/stores/layout.ts` (`layout.openPane(NewLeaf): string`).
- `openSettings(pageId?)` dari `frontend/src/modules/settings` (impor `../settings`; `openSettings('connectors')`).
- `Modal` dari `frontend/src/modules/settings` (`../settings`): props `{ open, onclose, title?, children: Snippet }` (sudah menangani Esc, fokus, trap Tab). Acuan gaya: `modules/settings/connectors/ConnectDialog.svelte` (CSS `var(--fg)`, `--fg-muted`, `--bg-elev`, `--border`, `--bg-hover`, `--accent`, `--danger` dengan fallback).
- IPC (overview 2.5): `lark.sources.get|add|remove|rename`, `lark.link.parse`, `lark.tables.list`, `lark.views.list`. Konvensi galat 2.4.
- Event: topik `workspace.changed`, payload `{kind:'configChanged', workspaceId, workspace}`.

Disediakan (dipakai wiring dan tiket 03/04):
```ts
// client.ts
export type TicketSource = { id: string; name: string; baseToken: string; host: string; tableId?: string; viewId?: string };
export type WikiSource   = { id: string; name: string; spaceId: string; rootToken?: string; host: string };
export type LarkConfig   = { tickets: TicketSource[]; wiki: WikiSource[] };
export type LarkError    = { code: string; message: string };
export type ParsedLink   = { kind: 'base' | 'wiki'; host: string; baseToken?: string; tableId?: string; viewId?: string; spaceId?: string; nodeToken?: string; defaultName?: string };
export type LarkTable = { id: string; name: string };
export type LarkView  = { id: string; name: string; type?: string };
export type AddSourceParams =
  | { kind: 'ticket'; name: string; baseToken: string; host: string; tableId?: string; viewId?: string }
  | { kind: 'wiki'; name: string; spaceId: string; rootToken?: string; host: string };
export function parseLarkError(e: unknown): LarkError;
export function larkSourcesGet(workspaceId: string): Promise<LarkConfig>;
export function larkSourceAdd(workspaceId: string, p: AddSourceParams): Promise<TicketSource | WikiSource>;
export function larkSourceRemove(workspaceId: string, sourceId: string): Promise<void>;
export function larkSourceRename(workspaceId: string, sourceId: string, name: string): Promise<void>;
export function larkLinkParse(link: string): Promise<ParsedLink>;
export function larkTablesList(workspaceId: string, sourceId: string): Promise<LarkTable[]>;
export function larkViewsList(workspaceId: string, sourceId: string, tableId: string): Promise<LarkView[]>;

// store.ts
export type LarkState = { status: 'loading' | 'ready' | 'error'; code?: string; message?: string };
export const larkSources: Writable<LarkConfig>;
export const larkState: Writable<LarkState>;
export function refreshSources(): Promise<void>;

// komponen
LarkIcon.svelte        // tanpa props
LarkPanel.svelte       // props {}
sections/WikiTree.svelte   // props { source, onOpen(nodeToken: string) }  (TIDAK boleh berubah)
```

## Langkah

### 1. `client.ts`
Impor `call` dari `../../lib/ipc`. Tulis tipe di atas (salinan lokal; tidak ada berkas tipe bersama).
- `parseLarkError(e)`: ambil `msg = e instanceof Error ? e.message : String(e)`; regex `/^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$/s`. Cocok -> `{ code: m[1], message: m[2] || msg }`; tidak cocok -> `{ code: 'unknown', message: msg }` (pesan apa adanya).
- Fungsi pembungkus `wrap<T>(method, params): Promise<T>` privat: `call<T>(method, params).catch((e) => { throw parseLarkError(e); })`, sehingga semua fungsi publik melempar objek `LarkError` (bukan `Error`).
- `larkSourcesGet` -> `call('lark.sources.get', { workspaceId })`; normalkan `{ tickets: r?.tickets ?? [], wiki: r?.wiki ?? [] }` (tahan null).
- `larkSourceAdd`: kirim `{ workspaceId, ...p }`. `larkSourceRemove`: `{ workspaceId, sourceId }`. `larkSourceRename`: `{ workspaceId, sourceId, name }`. Dua terakhir mengabaikan hasil.
- `larkLinkParse`: `{ link }`. `larkTablesList`: `{ workspaceId, sourceId }` -> `r?.tables ?? []`. `larkViewsList`: `{ workspaceId, sourceId, tableId }` -> `r?.views ?? []`.
- Jangan memanggil method di luar daftar kontrak.

### 2. `store.ts`
Impor `get, writable` dari `svelte/store`, `onEvent` dari `../../lib/ipc`, `activeWorkspaceId` dari `../../lib/stores/workspaces` (impor langsung dari berkas, bukan barrel, untuk menghindari siklus), serta `larkSourcesGet`, `parseLarkError` dari `./client`.
- `larkSources = writable<LarkConfig>({ tickets: [], wiki: [] })`; `larkState = writable<LarkState>({ status: 'loading' })`.
- Status lipat: `export const sectionOpen = writable<{ tickets: boolean; wiki: boolean }>({ tickets: true, wiki: true })` (di memori saja).
- `let seq = 0;`
- `refreshSources()`: `const wsId = get(activeWorkspaceId)`; bila null -> `seq++`, set `larkSources` kosong dan `larkState {status:'ready'}`, return. Selain itu `const my = ++seq`, `larkState.set({status:'loading'})` hanya bila belum pernah berhasil untuk workspace ini (hindari kedipan: bila `status==='ready'` biarkan isi lama tampil, jangan set loading kecuali dipanggil dari tombol segarkan/ganti workspace; sederhana: ganti workspace -> kosongkan dan set loading; refresh karena event -> tanpa loading). Terapkan hasil hanya bila `my === seq && get(activeWorkspaceId) === wsId`; sukses -> `larkSources.set(cfg)`, `larkState.set({status:'ready'})`; gagal -> `const e = parseLarkError(x)`; `larkState.set({ status:'error', code:e.code, message:e.message })` (isi `larkSources` lama dibiarkan).
- Subscriber level modul (dipasang sekali saat modul diimpor; tanpa fungsi start/stop, tidak di komponen): 
  - `activeWorkspaceId.subscribe((id) => { if (id === lastWs) return; lastWs = id; larkSources.set(kosong); larkState.set({status:'loading'}); void refreshSources(); })` dengan `let lastWs: string | null | undefined = undefined`. Panggilan pertama (saat subscribe) ikut memicu refresh awal.
  - `onEvent('workspace.changed', (_t, payload) => {...})`: abaikan bila payload bukan objek atau `kind !== 'configChanged'`; `wid = payload.workspaceId`; bila `wid` string non-kosong dan `!== get(activeWorkspaceId)` abaikan; selain itu `void refreshSources()` (tanpa loading).
- Tidak ada `setInterval`/polling. Proteksi `import.meta.hot` tidak diperlukan.
- Cakupan galat `cli_missing`/`not_logged_in` datang dari `lark.sources.get` hanya bila daemon mengembalikannya; tetap dipetakan di panel (langkah 5) karena kontrak mengizinkannya.

### 3. `LarkIcon.svelte`
SVG inline `viewBox="0 0 24 24"`, `width/height="24"`, `fill="none"`, `stroke="currentColor"`, `stroke-width="1.8"`, `stroke-linecap/linejoin="round"`, `aria-hidden="true"`. Gambar bentuk sederhana yang mengingatkan buku/dokumen berlapis (mis. `<path d="M5 4h9a4 4 0 0 1 4 4v12H9a4 4 0 0 1-4-4z"/>` dan `<path d="M9 9h5M9 13h5"/>`). Tanpa props (komponen ini dipakai wiring sebagai `icon` rail). Periksa ikon rail/topbar lain di `modules/` untuk gaya, tetapi jangan mengimpornya.

### 4. `sections/WikiTree.svelte` (placeholder; akan ditimpa tiket 04)
Props TEPAT: `let { source, onOpen }: { source: WikiSource | PersonalSource; onOpen: (nodeToken: string) => void } = $props();` dengan tipe lokal `type PersonalSource = { id: 'personal'; name: 'Pribadi'; spaceId: 'my_library'; host: string }`. Render satu tombol baris "Akar: {source.name}" yang saat diklik memanggil `onOpen(('rootToken' in source ? source.rootToken : undefined) ?? '')`, plus teks kecil "Pohon halaman segera hadir". Tidak memanggil IPC. Beri komentar berkas: "Placeholder; ditimpa lark-wiki. Jaga props { source, onOpen }."

### 5. `LarkPanel.svelte` (props `{}`)
- Impor `larkSources, larkState, refreshSources, sectionOpen` dari `./store`; `openSettings` dari `../settings`; `TicketSection`, `WikiSection` dari `./sections/...`. Pada `$effect` berlangganan store ke `$state` lokal (pola `GrimoirePanel.svelte`: `$effect(() => larkState.subscribe(v => { st = v; }))`); atau `$store` auto-subscribe bila gaya modul lain mengizinkan.
- Header: judul "LARK" kecil kapital (gaya `.title` di `GrimoirePanel.svelte`) dan tombol ikon "Segarkan" (`aria-label="Segarkan"`) yang memanggil `larkState.set({status:'loading'}); void refreshSources()`.
- Badan:
  - `status==='loading'` dan belum ada isi (`tickets`+`wiki` kosong): "Memuat...".
  - `status==='error'`: kotak galat dengan pesan terbaca dari `code`: `cli_missing` -> "lark-cli belum terpasang"; `not_logged_in` -> "Belum login Lark"; `permission` -> "Izin Lark kurang" + `message`; `timeout` -> "Waktu habis" ; lainnya -> `message`. Untuk `cli_missing`/`not_logged_in` tampilkan ajakan "Buka Pengaturan > Connector" sebagai tombol yang memanggil `openSettings('connectors')`. Tombol "Coba lagi" memanggil segarkan. Bila ada isi lama, tetap tampilkan bagian di bawah galat.
  - Selain itu: dua bagian lipat. Tiap bagian: tombol header (`aria-expanded`) berisi chevron + "Tiket"/"Wiki", status dari `sectionOpen`; isi `<TicketSection />` / `<WikiSection />` hanya saat terbuka. Keadaan kosong global tidak ada (kosong ditangani per bagian).
- Gaya: token `var(--fg)`, `--fg-muted`, `--bg-elev`, `--border`, `--bg-hover`, `--danger` dengan fallback seperti `GrimoirePanel.svelte`; panel mengisi tinggi penuh dan isi `overflow:auto`; lebar menyesuaikan kolom (tanpa lebar tetap).

### 6. `sections/TicketSection.svelte`
- Data: `larkSources.tickets`. `activeWorkspaceId` untuk panggilan.
- Header bagian: tombol "Tambah Base" -> `addOpen = true` (render `<AddSourceDialog kind="ticket" open={addOpen} onclose={() => addOpen = false} />`; impor `../AddSourceDialog.svelte`).
- Kosong: teks persis "Belum ada Base. Tambah Base."
- Daftar: tiap item baris tombol (ikon kecil inline + nama, `title=` nama + host) yang pada klik memanggil `layout.openPane({ kind: 'lark-board', sourceId: s.id, title: s.name })` (impor `layout` dari `../../../lib/stores/layout`). Dedupe ditangani `lark-pane-foundation` di `model.ts`; jangan mengurusnya di sini.
- Menu per item: tombol "..." (`aria-label="Aksi Base {nama}"`) membuka menu kecil (state `menuFor: string | null`, tutup saat klik di luar/Esc) dengan "Ganti nama" dan "Hapus".
  - Ganti nama inline: `editingId`, `draft`; `<input>` fokus otomatis; Enter = simpan, Esc = batal, blur = batal. Nama kosong (trim) -> tampilkan galat inline "Nama tidak boleh kosong" dan jangan panggil IPC. Simpan: `larkSourceRename(ws, id, name)`; galat `LarkError` -> tampil inline `.err` di bawah item. Tidak perlu mengubah store secara optimistis: `configChanged` memicu `refreshSources`; sebagai jaring pengaman panggil `void refreshSources()` setelah sukses (idempoten berkat `seq`).
  - Hapus dengan konfirmasi singkat inline: klik "Hapus" mengubah baris menjadi "Hapus '{nama}'? [Hapus] [Batal]"; konfirmasi memanggil `larkSourceRemove`, lalu `refreshSources()`; galat tampil inline. (Pane `lark-board` yang terbuka dibiarkan; tampilannya ditangani `lark-pane-foundation`.)
- Selalu `ws = get(activeWorkspaceId)`; bila null, nonaktifkan aksi tulis.

### 7. `sections/WikiSection.svelte`
- Konstanta lokal `PERSONAL = { id: 'personal', name: 'Pribadi', spaceId: 'my_library', host: '' } as const`. `host`: kosong tidak berbahaya untuk placeholder; bila ada `wiki[0]?.host` atau `tickets[0]?.host` dari store, pakai itu (turunan `$derived`), selain itu `''`.
- Baris pertama SELALU Pribadi (tanpa menu ganti nama/hapus; tampilkan ikon gembok kecil atau label "otomatis" sebagai tanda tak bisa diubah). Tidak ada probe `my_library` di fitur ini (daftar node adalah `lark.wiki.list` milik tiket 04); aturan "sembunyikan Pribadi bila `my_library` gagal" TIDAK diimplementasikan di sini dan dicatat sebagai penyimpangan sadar di bawah.
- Lalu `larkSources.wiki` sebagai baris dengan menu "Ganti nama"/"Hapus" (perilaku identik langkah 6; salin pola, boleh mengekstrak komponen kecil privat HANYA di dalam folder `sections/` bila ingin menghindari duplikasi, mis. `SourceRow.svelte`, ini tetap di ownedPaths).
- Klik baris = `open(source)`: `layout.openPane({ kind: 'lark-doc', sourceId: source.id /* 'personal' untuk Pribadi */, nodeToken: ('rootToken' in source ? source.rootToken : undefined) ?? '', title: source.name })`, dan set `expandedId = source.id` (satu wiki terbuka sekaligus; klik lagi pada wiki yang terbuka tidak menutup pane tetapi bisa menciutkan pohon lewat chevron terpisah).
- Saat `expandedId === source.id`, render `<WikiTree {source} onOpen={(nodeToken) => layout.openPane({ kind: 'lark-doc', sourceId: source.id, nodeToken, title: source.name })} />`. Kontrak `{ source, onOpen }` tetap; tiket 04 hanya menimpa `WikiTree.svelte`.
- Tombol "Tambah wiki" -> `<AddSourceDialog kind="wiki" ...>`.
- Bila `expandedId` menunjuk sumber yang sudah dihapus (hilang dari daftar), reset `expandedId = null` lewat `$effect`.

### 8. `AddSourceDialog.svelte`
Props: `{ kind: 'ticket' | 'wiki'; open: boolean; onclose: () => void }`. Bungkus dalam `Modal` (`title`: "Tambah Base" / "Tambah wiki").
- State: `link`, `parsed: ParsedLink | null`, `name`, `parsing`, `saving`, `error: string | null` (galat inline), `parseSeq`.
- Reset semua state saat `open` berubah dari false ke true (`$effect`).
- Kolom `<input type="text">` berlabel "Tempel link Base"/"Tempel link wiki" (placeholder contoh `https://xxx.larksuite.com/base/...`). Parse dipicu pada `onpaste` (setelah `queueMicrotask`/`setTimeout(0)` agar nilai terbarui), `onblur`, dan Enter; hanya bila `link.trim()` tidak kosong dan berbeda dari link yang terakhir di-parse. Link diubah setelah parse -> `parsed = null`.
- `parse()`: `const my = ++parseSeq`; `parsing = true; error = null`; `larkLinkParse(link.trim())`; abaikan hasil usang (`my !== parseSeq`). Sukses -> cek jenis (di bawah), lalu `parsed = r`, `name = r.defaultName ?? ''` (bila tidak ada default, nama kosong; pengguna wajib mengisi). Gagal -> `error = e.message` (mis. "Link tidak dikenali" atau pesan galat CLI; untuk `cli_missing`/`not_logged_in` tambahkan teks "Buka Pengaturan > Connector"; cukup teks, tanpa menutup dialog) dan `parsed = null`. Dialog tetap terbuka.
- Penolakan jenis salah (client-side, setelah parse sukses):
  - `kind==='ticket'` dan `r.kind !== 'base'` -> `error = 'Link ini bukan Base. Gunakan Tambah wiki untuk halaman wiki.'`
  - `kind==='wiki'` dan `r.kind !== 'wiki'` -> `error = 'Link ini adalah Base. Gunakan Tambah Base.'`
  - `parsed` tetap null; simpan nonaktif.
- Setelah parse valid tampilkan ringkasan: `<input>` Nama (bisa diubah, label "Nama"), dan untuk wiki dengan `parsed.nodeToken` baris keterangan `Akar: {parsed.defaultName ?? parsed.nodeToken}`; tanpa `nodeToken`: "Akar: seluruh space". Untuk Base tanpa tabel tidak ada pemilih tabel (dipilih di header pane oleh tiket 03).
- Tombol "Simpan" (nonaktif bila `!parsed || saving || !name.trim()`), "Batal". Simpan: `ws = get(activeWorkspaceId)` (null -> `error = 'Tidak ada workspace aktif'`); panggil `larkSourceAdd(ws, ...)`:
  - ticket: `{ kind:'ticket', name: name.trim(), baseToken: parsed.baseToken!, host: parsed.host, tableId: parsed.tableId, viewId: parsed.viewId }`
  - wiki: `{ kind:'wiki', name: name.trim(), spaceId: parsed.spaceId!, rootToken: parsed.nodeToken, host: parsed.host }`
  - Sukses: `await refreshSources()` lalu `onclose()`. Gagal: `error = e.message` (mis. `duplicate` -> "Sumber sudah ada"), dialog tetap terbuka, `saving=false`.
- Aksesibilitas: label terhubung ke input (`for/id` unik per instance), galat `role="alert"`, fokus awal ke kolom link.
- Gaya mengikuti `ConnectDialog.svelte` (kelas `.body`, `.error`, `.actions`, tombol sama).

### 9. Pemeriksaan
- Jalankan `cd frontend && npm run check` dan `npm run build`; perbaiki galat baru yang berasal dari berkas fitur ini. Galat di berkas lain (mis. `model.ts` belum memuat tipe Foundation) -> laporkan, jangan diperbaiki.
- Pastikan tidak ada impor dari `../index` (hindari siklus dengan `index.ts` wiring) dan `frontend/package.json` tidak berubah (`git diff --stat`).

## Perubahan
Dibuat (semua baru):
- `frontend/src/modules/lark/client.ts`
- `frontend/src/modules/lark/store.ts`
- `frontend/src/modules/lark/LarkIcon.svelte`
- `frontend/src/modules/lark/LarkPanel.svelte`
- `frontend/src/modules/lark/AddSourceDialog.svelte`
- `frontend/src/modules/lark/sections/TicketSection.svelte`
- `frontend/src/modules/lark/sections/WikiSection.svelte`
- `frontend/src/modules/lark/sections/WikiTree.svelte`
- opsional di `sections/`: komponen baris privat (mis. `SourceRow.svelte`) bila dipakai bersama TicketSection dan WikiSection.

Tidak ada berkas yang diubah di luar daftar ini.

## Wiring
Diekspos untuk disambungkan oleh `lark-sources-wiring` (jangan disambungkan di sini):
- `import LarkPanel from './LarkPanel.svelte'` dan `import LarkIcon from './LarkIcon.svelte'` di `frontend/src/modules/lark/index.ts`, dipakai sebagai `registerRailItem({ id: 'lark', title: 'Lark', icon: LarkIcon, component: LarkPanel, order: 10, visible })` (overview 2.6). `LarkPanel` bertipe props `{}`; `LarkIcon` tanpa props.
- Mengimpor `LarkPanel` otomatis mengimpor `store.ts` sehingga subscriber `activeWorkspaceId` dan `workspace.changed` aktif di level modul sejak `index.ts` dimuat (glob `modules/*/index.ts` di `main.ts`); tidak perlu start/stop.
- Untuk tiket 03/04: `store.ts` (`larkSources`, `larkState`, `refreshSources`) dan `client.ts` (`larkTablesList`, `larkViewsList`, `parseLarkError`, tipe) boleh diimpor; `sections/WikiTree.svelte` ditimpa dengan props `{ source, onOpen }` yang sama.

Penyimpangan sadar yang dicatat untuk wiring: (1) wiki Pribadi selalu ditampilkan; aturan pemotongan "sembunyikan bila `my_library` gagal" tidak dapat diuji di fitur ini (tidak ada panggilan wiki) dan jatuh ke `lark-wiki`/wiring; (2) `host` untuk sumber Pribadi diambil dari sumber lain bila ada, selain itu string kosong.

## Selesai bila
- Semua berkas di bagian Perubahan ada dan hanya path ownedPaths yang berubah (`git status` untuk path lain tidak berubah oleh fitur ini).
- `LarkPanel` (props `{}`) menampilkan bagian Tiket dan Wiki yang bisa dilipat, tombol segarkan, keadaan memuat/galat (pesan Indonesia; ajakan `openSettings('connectors')` untuk `cli_missing`/`not_logged_in`).
- Tiket kosong menampilkan persis "Belum ada Base. Tambah Base."; Pribadi paling atas tanpa aksi ubah/hapus; wiki tim bisa diganti nama/dihapus dengan konfirmasi.
- `AddSourceDialog` menolak jenis link yang salah, menampilkan galat parse inline tanpa menutup dialog, nama default bisa diubah, simpan via `lark.sources.add`.
- `WikiSection` menyerahkan `{ source, onOpen }` ke `WikiTree`; subscriber event/workspace hanya di `store.ts` level modul; tanpa polling; tanpa dependensi baru; tanpa unit test.
- Guard: `cd frontend && npm run build` sukses dan `npm run check` tanpa galat baru dari path fitur ini.
