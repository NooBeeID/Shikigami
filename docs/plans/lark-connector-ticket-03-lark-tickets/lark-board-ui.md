# Plan: Pane board grid dan kanban

## Metadata

- Tiket: `lark-connector-ticket-03-lark-tickets` (`docs/tickets/lark-connector-ticket-03-lark-tickets.md`)
- Fitur: `lark-board-ui`
- Kontrak bersama: `overview.md` (folder ini), terutama bagian 1, 2.4, 2.6, 6, 7. Jangan mengubah `overview.md`.
- Depends on foundation: **ya**. `00-foundation.md` F3 membuat `frontend/src/modules/lark/board/boardClient.ts` lengkap (tipe dan `listTickets`/`getTicket`/`listTables`/`listViews`/`parseLarkError`). Fitur ini hanya memperluasnya secara ADITIF. Bila berkas itu belum ada saat implementasi, hentikan dan laporkan (jangan membuat versi sendiri yang bentrok).
- ownedPaths:
  - `frontend/src/modules/lark/board/BoardPane.svelte` (timpa placeholder tiket 02)
  - `frontend/src/modules/lark/board/Grid.svelte` (baru)
  - `frontend/src/modules/lark/board/Kanban.svelte` (baru)
  - `frontend/src/modules/lark/board/BoardHeader.svelte` (baru)
  - `frontend/src/modules/lark/board/boardClient.ts` (tambahan aditif)

## Tujuan

Menimpa placeholder `BoardPane.svelte` menjadi pane tiket Lark Base yang lengkap: keadaan memuat/kosong/galat dengan "Coba lagi", pemilih tabel dan view, Grid dan Kanban, cari lokal, "Muat lagi" (50 per halaman), segarkan, Salin link dan Kirim ke agen tingkat Base/view, pemilih field tambahan (maks dua) dan pemilih field kanban. Pilihan tabel/view dipersist ke props pane (bukan ke sumber). Pane tidak crash pada galat dan respons usang diabaikan. Fitur ini menyediakan `selectedRecordId` dan `onOpenRecord` tetapi TIDAK mengimpor `TicketDrawer` (itu pekerjaan `lark-tickets-wiring`).

## Path yang dimiliki

HANYA lima berkas di Metadata yang boleh disentuh. Jangan sentuh: `TicketDrawer.svelte`, `cmd/shikigamictl/*`, file Go apa pun, `frontend/src/modules/lark/` selain `board/` (`store.ts`, `client.ts`, `actions.ts`, `source.ts`, `index.ts`, `LarkPanel*`, `sections/`), `frontend/src/modules/layout/*`, `frontend/src/lib/stores/layout.ts`, `frontend/package.json`, `overview.md`. Jangan menyambungkan laci tiket. Working tree punya perubahan tak terkait; jangan sentuh atau stage.

## Kontrak

Dipakai (periksa nama persis di kode; tiket 01/02 mungkin baru merge, laporkan selisih, jangan mengarang):
- Dari `./boardClient` (Foundation, overview 2.4): `FIELD_TYPE_SINGLE_SELECT`, tipe `TicketSource`, `LarkError`, `BoardField`, `BoardRecord`, `TicketsPage`, `TableInfo`, `ViewInfo`, fungsi `parseLarkError`, `listTickets`, `listTables`, `listViews`. Semua melempar objek `LarkError` saat gagal.
- Dari `../actions` (tiket 02): `copyLink(url: string): Promise<void>`, `sendToAgent(url: string): Promise<void>` (toast "Link disalin" / "Tidak ada sesi aktif" sudah ditangani di sana).
- Dari `../store` (tiket 02): `larkSources` (store; isi `{ tickets: TicketSource[]; wiki: ... }`), `larkState` (store `{ status: 'loading'|'ready'|'error'; code?; message? }`), `refreshSources()`.
- `activeWorkspaceId` dari `../../../lib/stores/workspaces` (periksa jalur; `thread` mengimpornya dari `lib/stores`).
- `layout` dari `../../../lib/stores/layout` dan tipe `Leaf` dari `../../layout/model`. Hanya `layout.serialize()/get(layout)`, `layout.renamePane` dipakai (lihat langkah 8). Jangan mengubah `layout.ts`/`model.ts`.
- Komponen `Grid`, `Kanban`, `BoardHeader` hanya dipakai oleh `BoardPane` (internal fitur ini).

Disediakan:
- `BoardPane.svelte`: props `{ pane: Leaf; visible: boolean }` (TIDAK boleh berubah). Di dalamnya: `let selectedRecordId = $state<string | null>(null)` dan `function onOpenRecord(recordId: string)`; keduanya titik sambung untuk wiring.
- `Grid.svelte`: props `{ fields: BoardField[]; records: BoardRecord[]; extraFieldIds?: string[]; onOpen: (recordId: string) => void; selectedRecordId?: string | null }`.
- `Kanban.svelte`: props `{ fields: BoardField[]; records: BoardRecord[]; groupFieldId: string; extraFieldIds: string[]; onOpen: (recordId: string) => void; selectedRecordId?: string | null }`. (`selectedRecordId` opsional hanya tambahan aditif untuk menandai item terpilih.)
- Tambahan aditif di `boardClient.ts` (nama di langkah 1; yang sudah ada tidak diubah nama/signature-nya).

Aturan UI (overview 2.4 dan 2.6): teks Indonesia, identifier Inggris; tanpa polling; seq guard; gaya token tema (`var(--fg)`, `var(--fg-muted)`, `var(--border)`, `var(--bg-hover)` dengan fallback seperti `ThreadSection.svelte`); tanpa dependensi baru; tanpa unit test.

## Langkah

### 1. Tambahan aditif `boardClient.ts`

Baca berkas dulu; tambahkan di bagian bawah (impor `type` yang diperlukan dari berkas yang sama), tanpa mengubah ekspor yang ada:

```ts
export type ViewMode = 'grid' | 'kanban';
export const MAX_EXTRA_FIELDS = 2;
export const NO_VALUE_LABEL = 'Tanpa nilai';

// Normalisasi galat apa pun menjadi LarkError (listX sudah melempar LarkError; ini untuk jaga-jaga).
export function toLarkError(e: unknown): LarkError;
//   objek dengan code & message bertipe string -> kembalikan apa adanya; selain itu parseLarkError(e).

export function isSingleSelect(f: BoardField): boolean;       // f.type === FIELD_TYPE_SINGLE_SELECT
export function primaryField(fields: BoardField[]): BoardField | undefined; // isPrimary, fallback fields[0]
export function singleSelectFields(fields: BoardField[]): BoardField[];
export function eligibleExtraFields(fields: BoardField[]): BoardField[];    // semua field kecuali primary

// Cari lokal: cocokkan q (trim, lowercase) terhadap title dan semua nilai values; q kosong -> records apa adanya.
export function filterRecords(records: BoardRecord[], q: string): BoardRecord[];

// Gabung halaman berikutnya; buang record dengan id yang sudah ada (jaga duplikat).
export function mergeRecords(existing: BoardRecord[], incoming: BoardRecord[]): BoardRecord[];

// Link tingkat Base/view, dibangun di frontend. Kembalikan '' bila host atau baseToken kosong.
// Format: https://<host>/base/<baseToken>?table=<tableId>&view=<viewId>
//   - host: buang awalan http(s):// dan "/" di akhir; paksa skema https.
//   - hanya sertakan table bila tableId ada, view bila viewId ada (encodeURIComponent).
// Bila docs/lark/cli-findings.md menetapkan format link Base/view lain, ikuti findings dan catat di komentar.
export function buildBaseLink(source: Pick<TicketSource, 'host' | 'baseToken'>, tableId: string, viewId?: string): string;
```

`boardClient.ts` tetap tidak mengimpor komponen Svelte, `store.ts`, atau `layout`.

### 2. `Grid.svelte` (baru)

`let { fields, records, extraFieldIds = [], onOpen, selectedRecordId = null }: {...} = $props();`

- Kolom: kolom judul (field primary; fallback `fields[0]`) tetap di kiri (`position: sticky; left: 0`, latar solid), diikuti semua field non-primary. Urutan: field di `extraFieldIds` lebih dulu (sesuai urutan itu), sisanya mengikuti urutan `fields`. Sel judul memakai `record.title`; sel lain `record.values[field.id] ?? ''`.
- `<table>` dalam wadah `overflow: auto` (kedua arah), `thead` sticky di atas. Teks sel `white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 280px` dengan atribut `title` berisi teks penuh.
- Baris: `tr` dengan `tabindex="0"`, `role="button"`, `onclick={() => onOpen(r.id)}`, `onkeydown` Enter/Spasi memanggil `onOpen` (cegah scroll pada Spasi). Kelas `.selected` bila `r.id === selectedRecordId`. Hover memakai `var(--bg-hover, #0001)`. `{#each records as r (r.id)}`.
- Tidak menangani keadaan kosong/memuat (milik `BoardPane`).

### 3. `Kanban.svelte` (baru)

`let { fields, records, groupFieldId, extraFieldIds, onOpen, selectedRecordId = null } = $props();`

- `columns = $derived(...)`: iterasi `records` berurutan; kunci = `(r.values[groupFieldId] ?? '').trim()`. Kolom bernilai dibuat menurut urutan kemunculan pertama; record dengan kunci kosong masuk kolom "Tanpa nilai" (`NO_VALUE_LABEL`) yang SELALU diletakkan paling akhir dan hanya dirender bila tidak kosong ATAU tidak ada kolom lain (agar papan tidak kosong total). Opsi single-select tanpa record tidak tampil (urutan opsi tidak tersedia di kontrak).
- Tata letak: baris horizontal `overflow-x: auto`; tiap kolom lebar tetap (~260px), judul kolom + jumlah kartu, isi kolom `overflow-y: auto` sendiri.
- Kartu: `button` (type="button") satu per record; judul = `record.title` (maks 3 baris, clamp); metadata maksimal dua field dari `extraFieldIds` (ambil dua pertama yang ada di `fields` dan bernilai tidak kosong) tampil sebagai baris kecil `nama: nilai` dengan `var(--fg-muted)`. `onclick={() => onOpen(r.id)}`; `.selected` bila id cocok; `{#each ... (r.id)}`.
- Komponen tidak memeriksa apakah `groupFieldId` valid; `BoardPane` menjamin itu.

### 4. `BoardHeader.svelte` (baru)

Props (semua dari `BoardPane`, komponen hanya presentasi dan memanggil callback):

```ts
{
  tables: TableInfo[]; tableId: string;
  views: ViewInfo[]; viewId: string;           // '' = view bawaan
  mode: ViewMode; canKanban: boolean;
  search: string;
  kanbanFields: BoardField[]; groupFieldId: string;
  extraCandidates: BoardField[]; extraFieldIds: string[];
  loading: boolean; loadingMore: boolean; hasMore: boolean;
  loadedCount: number; shownCount: number;
  linkAvailable: boolean;                       // buildBaseLink(...) !== ''
  onTable(id: string): void; onView(id: string): void; onMode(m: ViewMode): void;
  onSearch(q: string): void; onGroup(id: string): void; onExtra(ids: string[]): void;
  onLoadMore(): void; onRefresh(): void; onCopy(): void; onSend(): void;
}
```

Isi (satu bilah `display: flex; flex-wrap: wrap; gap: 6px; align-items: center; padding: 6px 8px; border-bottom: 1px solid var(--border, #8884)`):
- `<select>` tabel (`aria-label="Tabel"`); `<select>` view (`aria-label="View"`) dengan opsi pertama `value=""` "View bawaan" lalu `views`. Nonaktif saat `loading` atau daftar kosong.
- Kontrol segmented Grid | Kanban (dua `button` dengan `aria-pressed`). Tombol Kanban `disabled={!canKanban}` dan `title="Tabel ini tidak punya field single-select"`; saat `!canKanban` tampilkan teks keterangan kecil "Tabel ini tidak punya field single-select" di bilah.
- Saat `mode === 'kanban' && canKanban`: `<select aria-label="Kelompokkan menurut">` dari `kanbanFields` (nilai `groupFieldId`).
- Pemilih field tambahan: tombol "Field tambahan (n/2)" yang membuka dropdown `<details>`/popover sederhana berisi checkbox per `extraCandidates` (nama field); checkbox yang belum dicentang `disabled` bila sudah `MAX_EXTRA_FIELDS` terpilih; perubahan memanggil `onExtra(newIds)` (pertahankan urutan pemilihan). Tutup dropdown saat klik di luar atau Esc (listener di `svelte:window`, bersihkan otomatis oleh Svelte).
- `<input type="search" placeholder="Cari tiket…" aria-label="Cari tiket">`, `value={search}`, `oninput` -> `onSearch`. Teks kecil `shownCount/loadedCount` bila `search` tidak kosong ("n dari m tiket termuat").
- Tombol "Muat lagi": `disabled={!hasMore || loadingMore || loading}`, label "Memuat…" saat `loadingMore`.
- Tombol segarkan (ikon teks "↻", `aria-label="Segarkan"`, `title="Segarkan"`), `disabled={loading}`.
- Tombol "Salin link" dan "Kirim ke agen": `disabled={!linkAvailable}`; `title` menjelaskan ("Salin link Base/view ini" / "Tempel link ke sesi agen aktif"; bila tidak tersedia "Link Base tidak tersedia").
- Gaya mengikuti `.new` di `ThreadSection.svelte` (`font: inherit; font-size: 11px; border: 1px solid var(--border, #8884); border-radius: 4px; padding: 2px 8px; background: transparent; cursor: pointer`).

### 5. `BoardPane.svelte` (timpa)

Buka berkas placeholder dulu; pertahankan impor `Leaf`, signature `let { pane, visible }: { pane: Leaf; visible: boolean } = $props();` dan nama berkas. Hapus teks "Segera hadir" dan watch placeholder.

State (Svelte 5 runes):
```ts
type Phase = 'loading' | 'ready' | 'error';
let phase = $state<Phase>('loading');
let error = $state<LarkError | null>(null);
let tables = $state<TableInfo[]>([]);
let views = $state<ViewInfo[]>([]);
let tableId = $state(''); let viewId = $state('');
let fields = $state<BoardField[]>([]); let records = $state<BoardRecord[]>([]);
let nextOffset = $state<number | undefined>(undefined);
let loadingMore = $state(false); let moreError = $state('');
let search = $state(''); let mode = $state<ViewMode>('grid');
let groupFieldId = $state(''); let extraFieldIds = $state<string[]>([]);
let selectedRecordId = $state<string | null>(null);   // WIRING: dipakai lark-tickets-wiring
let seq = 0; let metaSeq = 0; let viewsSeq = 0;         // penghitung stale guard (bukan state)
```

Turunan:
- `source = $derived($larkSources.tickets?.find((s) => s.id === pane.sourceId))`; `sourceKey = $derived(source ? source.id + '|' + source.baseToken : '')`; `wsId = $derived($activeWorkspaceId ?? '')`.
- `effectiveMode = $derived(mode === 'kanban' && kanbanFields.length > 0 ? 'kanban' : 'grid')`; `kanbanFields = $derived(singleSelectFields(fields))`; `canKanban = $derived(kanbanFields.length > 0)`.
- `visibleRecords = $derived(filterRecords(records, search))`.
- `baseLink = $derived(source ? buildBaseLink(source, tableId, viewId) : '')`.

Alur (semua async fungsi memakai `try/catch`, tidak ada throw keluar):

1. **Status sumber** (`$effect` pertama, hanya membaca `$larkState`, `source`, `wsId`): bila `wsId` kosong -> `phase='error'`, `error={code:'invalid_params', message:'Workspace belum dipilih'}`. Bila `$larkState.status === 'loading'` dan `source` belum ada -> `phase='loading'`. Bila `$larkState.status === 'error'` dan `source` tidak ada -> `phase='error'` dengan `$larkState.message`. Bila status `ready` dan `source` tidak ada -> `phase='error'`, `error={code:'not_found', message:'Sumber tidak ada di workspace ini'}`. Bila `source` ada -> jalankan `untrack(() => bootstrap())` HANYA bila `sourceKey`/`wsId` berubah dari yang terakhir di-bootstrap (simpan `lastBootKey`); efek tidak boleh berjalan ulang karena perubahan state lokal (bungkus pembacaan state lokal dengan `untrack`). Pemulihan pane (mount setelah restart) otomatis melewati jalur ini sehingga data selalu diambil ulang.
2. **`bootstrap()`**: `const my = ++metaSeq; ++seq; phase='loading'; error=null; records=[]; fields=[]; nextOffset=undefined; selectedRecordId=null`. `tables = await listTables({workspaceId: wsId, sourceId: source.id})`; `if (my !== metaSeq) return`. Tabel kosong -> `phase='ready'`, tampilkan keadaan kosong "Base ini belum punya tabel". Pilih `tableId`: `pane.tableId` bila ada di `tables`, jika tidak `source.tableId` bila ada, jika tidak `tables[0].id`. Pilih `viewId`: hanya bila tabel terpilih sama dengan `pane.tableId` pakai `pane.viewId`, atau bila sama dengan `source.tableId` pakai `source.viewId`; selain itu `''`. Panggil `await loadViews(tableId)` (validasi: `viewId` yang tidak ada di daftar view -> `''`), lalu `await loadTickets(true)`. Galat -> `setError(e)` (jika `my` masih terkini). `persistPane()` setelah pilihan awal ditentukan (langkah 8).
3. **`loadViews(tid)`**: `const my = ++viewsSeq`; `listViews(...)`; abaikan bila `my !== viewsSeq` atau `tid !== tableId`; galat memuat view TIDAK fatal (`views = []`, lanjut dengan view bawaan).
4. **`loadTickets(reset: boolean)`**: bila `reset`: `const my = ++seq; phase='loading'; error=null; moreError=''; loadingMore=false`, panggil `listTickets({workspaceId: wsId, sourceId: source.id, tableId, viewId: viewId || undefined, offset: 0})`; `if (my !== seq) return` (respons usang dibuang); set `fields`, `records`, `nextOffset`; `phase='ready'`; panggil `syncFieldSelections()`. Galat -> `if (my === seq) setError(e)`.
5. **`loadMore()`**: guard `if (nextOffset === undefined || loadingMore || phase !== 'ready') return`; `const my = seq; loadingMore=true; moreError=''`; `listTickets({... offset: nextOffset})`; `if (my !== seq) return` (jangan reset `loadingMore` milik muat baru; `loadTickets(true)` sudah mereset); `records = mergeRecords(records, page.records)`; `nextOffset = page.nextOffset`; galat -> `moreError = toLarkError(e).message` (data lama tetap, tampilkan pesan kecil di dekat tombol); `finally { if (my === seq) loadingMore=false }`. `fields` tidak diganti dari halaman berikutnya kecuali masih kosong.
6. **`syncFieldSelections()`** (setelah `fields` dimuat/berubah di reset): `groupFieldId` tetap bila masih ada di `kanbanFields`, jika tidak `kanbanFields[0]?.id ?? ''`; `extraFieldIds` = saring yang masih ada di `eligibleExtraFields(fields)`; bila hasilnya kosong dan belum pernah diatur pengguna untuk tabel ini (`extraTouched === false`), isi default dua field pertama dari `eligibleExtraFields(fields)` selain `groupFieldId`. `extraTouched` direset saat tabel berganti dan di-set `true` oleh `onExtra`. Bila `mode==='kanban'` tapi `!canKanban` biarkan `effectiveMode` jatuh ke grid.
7. **Handler header**:
   - `onTable(id)`: `if (id === tableId) return; tableId=id; viewId=''; views=[]; records=[]; fields=[]; nextOffset=undefined; search=''; selectedRecordId=null; extraTouched=false; persistPane(); await loadViews(id) (tanpa menunggu lama: jalankan paralel dengan loadTickets), loadTickets(true)`.
   - `onView(id)`: `viewId=id; records=[]; nextOffset=undefined; selectedRecordId=null; persistPane(); loadTickets(true)` (tabel/field tetap; `search` dipertahankan).
   - `onMode`, `onSearch` (set `search`), `onGroup(id)` (set `groupFieldId`; keluarkan dari `extraFieldIds` bila ada), `onExtra(ids)` (`extraTouched=true`, potong ke `MAX_EXTRA_FIELDS`).
   - `onRefresh()`: ambil ulang daftar tabel/view dan tiket dengan mempertahankan pilihan saat ini: `const keepTable = tableId, keepView = viewId`, jalankan alur `bootstrap()` yang menghormati `keepTable/keepView` (parameter opsional `bootstrap(keep?: {tableId: string; viewId: string})`; bila tabel tersimpan sudah tidak ada, jatuh ke aturan awal).
   - `onCopy()`: `if (baseLink) void copyLink(baseLink)`. `onSend()`: `if (baseLink) void sendToAgent(baseLink)`.
   - `onOpenRecord(recordId: string)`: `selectedRecordId = recordId`. Di sini TIDAK ada impor atau render `TicketDrawer`; tinggalkan komentar `<!-- WIRING lark-tickets-wiring: render TicketDrawer bila selectedRecordId -->` pada tempat yang cocok di markup (dalam wadah pane, setelah area daftar).
   - `retry()`: bila `tables.length === 0` atau `tableId === ''` -> `bootstrap()`; bila status sumber `error` karena `$larkState` -> `void refreshSources()`; selain itu `loadTickets(true)`.
8. **`persistPane()`**: simpan `tableId`/`viewId` ke props pane (bukan ke sumber; jangan memanggil `lark.sources.*`). Karena store `layout` tidak punya setter props leaf dan `layout.ts`/`model.ts` tidak boleh diubah di fitur ini: pertama grep `model.ts`/`layout.ts` untuk setter yang sudah ada (mis. `patchLeaf`/`setLeafProps`/`updateLeaf`); bila ada, pakai. Bila tidak ada, pakai solusi lokal: telusuri `get(layout).root` (node `type === 'pane'` atau `{a, b}`) mencari leaf dengan `id === pane.id`; bila `leaf.tableId === tableId && (leaf.viewId ?? '') === viewId` -> selesai (tanpa emit); jika tidak, tetapkan `leaf.tableId = tableId; if (viewId) leaf.viewId = viewId; else delete leaf.viewId;` lalu `layout.renamePane(pane.id, leaf.title || 'Tiket Lark')` (renamePane membuat salinan leaf beserta field barunya dan memancarkan store sehingga autosave menyimpan). Bungkus try/catch (kegagalan persist tidak boleh mengganggu pane). Catat di laporan bahwa setter resmi di `layout.ts` lebih baik (di luar ownedPaths). State lokal `tableId`/`viewId` adalah sumber kebenaran; props pane hanya dibaca saat `bootstrap()`.

Markup:
- Wadah akar `display: flex; flex-direction: column; flex: 1; min-height: 0; position: relative; background: var(--bg, transparent)`.
- Selama `phase !== 'error'` dan `source` ada: `<BoardHeader .../>` (props dari state; `linkAvailable={baseLink !== ''}`, `loading={phase === 'loading'}`, `hasMore={nextOffset !== undefined}`, `loadedCount={records.length}`, `shownCount={visibleRecords.length}`, `extraCandidates={eligibleExtraFields(fields)}`). Bila `phase === 'error'` dengan sumber hilang, header tidak dirender.
- Badan (`flex: 1; min-height: 0; overflow: hidden; display: flex`):
  - `phase === 'loading'` dan `records.length === 0`: teks "Memuat tiket…" (`role="status"`).
  - `phase === 'error'`: blok tengah berisi `error.message` (`role="alert"`) dan tombol "Coba lagi" -> `retry()`. Untuk `code === 'not_found'` ("Sumber tidak ada di workspace ini") tombol coba lagi tetap ada tetapi sekunder.
  - `phase === 'ready'` dan `records.length === 0`: "Tidak ada tiket" (bila tabel tidak punya tabel sama sekali: "Base ini belum punya tabel").
  - `phase === 'ready'`, `records.length > 0`, `visibleRecords.length === 0`: "Tidak ada tiket yang cocok dengan pencarian" (+ keterangan "Pencarian hanya menyaring tiket yang sudah dimuat" bila `nextOffset !== undefined`).
  - Selain itu: `effectiveMode === 'kanban'` -> `<Kanban fields {records: visibleRecords} {groupFieldId} {extraFieldIds} onOpen={onOpenRecord} {selectedRecordId} />`; jika tidak -> `<Grid fields records={visibleRecords} {extraFieldIds} onOpen={onOpenRecord} {selectedRecordId} />`.
  - Bila `moreError`, tampilkan baris kecil "Gagal memuat halaman berikutnya: {moreError}" di bawah header.
- Saat `phase === 'loading'` dengan data lama (segarkan): tampilkan data lama tetap, tombol nonaktif; tidak berkedip kosong.
- `<style>` biasa (bukan `:global`), fallback warna seperti komponen lain.

### 6. Pemeriksaan kontrak sebelum selesai
- Pastikan `BoardPane.svelte` tidak mengimpor `TicketDrawer` dan tidak ada siklus impor (`boardClient` tidak mengimpor komponen).
- Pastikan tidak ada pemanggilan `lark.sources.*` dan tidak ada `setInterval`/polling.
- Pastikan `loadTickets`/`loadMore`/`bootstrap`/`loadViews` semuanya memeriksa penghitung `seq`/`metaSeq`/`viewsSeq` sebelum menulis state.

### 7. Verifikasi
- `cd frontend && npm run build` sukses. Jalankan juga `npm run check` dan pastikan tidak ada galat baru pada lima berkas ini.
- `git status` hanya menampilkan lima ownedPaths (tiga baru, dua diubah) sebagai perubahan fitur ini; jangan men-stage berkas tak terkait.
- Aplikasi tidak perlu dijalankan di tahap ini (verifikasi akhir ada di wiring).

## Perubahan

Dibuat:
- `frontend/src/modules/lark/board/Grid.svelte`
- `frontend/src/modules/lark/board/Kanban.svelte`
- `frontend/src/modules/lark/board/BoardHeader.svelte`

Diubah:
- `frontend/src/modules/lark/board/BoardPane.svelte` (timpa penuh placeholder tiket 02)
- `frontend/src/modules/lark/board/boardClient.ts` (hanya penambahan aditif langkah 1)

## Wiring

Diekspos untuk `lark-tickets-wiring` (JANGAN disambungkan di fitur ini):
- Di `BoardPane.svelte`: state `selectedRecordId` dan fungsi `onOpenRecord(recordId)` (sudah dipasang sebagai `onOpen` Grid/Kanban), serta variabel `source` (TicketSource aktif), `tableId`, `viewId`. Wiring menambah impor `TicketDrawer`, merender laci pada tempat komentar `WIRING` dengan `source`, `tableId`, `viewId`, `recordId={selectedRecordId}` dan `onClose` yang mengosongkan `selectedRecordId`. Fitur ini sudah mengosongkan `selectedRecordId` saat sumber/tabel/view berganti; wiring cukup memverifikasinya.
- `boardClient.ts`: ekspor aditif `buildBaseLink`, `toLarkError`, dll. yang boleh dipakai `TicketDrawer`.
- Catatan penyimpangan untuk wiring/laporan: persist props pane memakai solusi lokal (`renamePane` pemancar) karena `layout.ts` tidak punya setter; bila setter resmi ditambahkan nanti, ganti isi `persistPane()` saja. Prop opsional `selectedRecordId` pada `Grid`/`Kanban` adalah tambahan aditif atas kontrak overview 2.4.

## Selesai bila

- [ ] `cd frontend && npm run build` sukses (guard path fitur ini); `npm run check` tanpa galat baru; tanpa unit test.
- [ ] `BoardPane` memuat tiket Base: tabel/view bisa diganti (ganti tabel mereset view dan data), "Muat lagi" menambah halaman bila `nextOffset`, segarkan mengambil ulang.
- [ ] Grid: kolom judul (field utama) tetap di kiri, kolom per field, baris klik/Enter memanggil `onOpen`.
- [ ] Kanban: kolom per nilai field single-select terpilih (default yang pertama), kolom "Tanpa nilai" paling akhir, kartu berisi judul dan maks dua field tambahan; tombol Kanban nonaktif dengan keterangan "Tabel ini tidak punya field single-select" bila tak ada field single-select, grid tetap jalan.
- [ ] Kotak cari menyaring data termuat secara lokal tanpa panggilan IPC; pemilih field tambahan membatasi maksimal dua.
- [ ] Salin link dan Kirim ke agen tingkat Base/view memakai `buildBaseLink` + `copyLink`/`sendToAgent` (nonaktif bila link tak dapat dibangun).
- [ ] Keadaan memuat/kosong/galat aman dengan "Coba lagi"; "Sumber tidak ada di workspace ini" dan jaringan gagal tidak membuat pane crash.
- [ ] Respons usang (ganti tabel/view/sumber/segarkan saat permintaan berjalan) tidak menimpa data baru.
- [ ] `tableId`/`viewId` dipersist ke props pane (bukan ke sumber); pane dipulihkan mengambil ulang data.
- [ ] `selectedRecordId` dan `onOpenRecord` tersedia; `TicketDrawer` tidak diimpor; tidak ada polling; hanya lima ownedPaths yang berubah.
