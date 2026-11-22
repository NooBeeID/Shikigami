---
id: lark-connector-ticket-02-lark-sources
title: Rail Lark dan sumber per workspace
spec: docs/superpowers/specs/2026-11-22-09-lark-connector-design.md
status: implemented
wave: 2
depends_on: [lark-connector-ticket-01-lark-connector]
---
# Rail Lark dan sumber per workspace

## Tujuan
Epic ini menghadirkan item rail **Lark** beserta `LarkPanel` (bagian Tiket dan Wiki), dan menyimpan sumber Base serta wiki **per workspace** lewat IPC per item. Sumber ditambahkan manual dengan menempel link; parser link mengurai host, token, table, view, atau space/node. Epic ini juga meletakkan fondasi pane `lark-*` di sistem layout (registrasi jenis pane, persist field referensi, keadaan "pane tidak dikenal") dan helper "Salin link" / "Kirim ke agen", sehingga tiket Lark berikutnya (`lark-tickets`, `lark-wiki`) hanya perlu menimpa placeholder tanpa menyentuh layout atau panel.

## Hasil yang bisa dijalankan
Di atas tiket `lark-connector-ticket-01-lark-connector` (connector `lark` terdaftar, status dari `lark-cli auth status`, `larkdata.Run()` tersedia) dan tiket rail `main-rail-ticket-01-main-rail`, setelah tiket ini selesai:
- Rail memuat item **Lark** (order 10, ikon Lark, `aria-label` "Lark") yang tampil bila connector `lark` tersedia. Klik membuka `LarkPanel` di panel kiri dengan dua bagian lipat: **Tiket** dan **Wiki**.
- Bagian Tiket: tombol **Tambah Base** membuka dialog. Tempel link Base (termasuk link `/wiki/...` yang berisi bitable), nama default terisi dari Base dan bisa diubah, simpan. Base muncul di daftar; bisa diganti nama dan dihapus; bila kosong tampil "Belum ada Base. Tambah Base." Klik Base membuka pane tengah `lark-board` berisi placeholder "Segera hadir".
- Bagian Wiki: **Pribadi** selalu di paling atas (tidak disimpan), lalu wiki tim. Tombol **Tambah wiki** membuka dialog (tempel link space atau node; node dijadikan akar). Wiki tim bisa diganti nama dan dihapus. Klik wiki membuka pane `lark-doc` akar berisi placeholder "Segera hadir"; pohon node masih daftar datar placeholder.
- Panel menampilkan keadaan memuat, kosong, dan galat terbaca (CLI belum terpasang, belum login, izin kurang, timeout) dengan ajakan ke Pengaturan > Connector dan tombol segarkan. Isi berganti saat workspace aktif berganti atau event `configChanged` datang.
- Sumber tersimpan per workspace dan tetap ada setelah restart aplikasi. Pane `lark-*` yang terbuka dipulihkan setelah restart; pane dengan `sourceId` yang tak ada di workspace aktif menampilkan "Sumber tidak ada di workspace ini"; jenis pane tak dikenal tampil sebagai "pane tidak dikenal" (tidak dibuang).
- `shikigamictl lark sources get|add|remove|rename` tersedia sebagai alat debug.
- `go build ./...` dan build frontend lulus; aplikasi berjalan.

## Batasan
In scope:
- `LarkConfig` di `WorkspaceConfig` (perubahan minimal satu field) dan operasi per item di `internal/workspace/lark.go`.
- IPC `lark.sources.get/add/remove/rename`, `lark.link.parse`, `lark.tables.list`, `lark.views.list`.
- Parser link Base/Wiki, resolusi nama default.
- Modul frontend `lark` (rail item, panel, dialog Tambah, client IPC, store, ikon, bagian Tiket/Wiki, placeholder `WikiTree`).
- Fondasi pane: registrasi `lark-board` dan `lark-doc` (komponen placeholder), perluasan loader layout untuk field referensi, keadaan pane tak dikenal, keadaan sumber tak ada.
- Helper `copyLink(url)` dan `sendToAgent(url)`.
- `shikigamictl lark sources`.

Out of scope:
- Connector `lark`, `larkdata/runner.go`, allowlist, fixture, `docs/lark/` (milik tiket 01).
- Daftar tiket, kanban/grid, laci detail, builder link tiket (`lark.tickets.list`, `lark.ticket.get`; tiket `lark-tickets`).
- Pohon wiki lazy, penampil dokumen, `lark.wiki.list/get` (tiket `lark-wiki`). Menimpa `board/BoardPane.svelte`, `doc/DocPane.svelte`, `sections/WikiTree.svelte` adalah tugas tiket tersebut.
- Perubahan `frontend/package.json`, render markdown.
- Membuat/mengubah isi Base atau wiki (read-only total), cache disk, menyimpan token, unit test.
- Item rail Git, deteksi tenant lain.

Prasyarat (harus sudah terpenuhi sebelum mulai):
- `main-rail-ticket-01-main-rail` sudah merge (`registerRailItem`/`railItems` di `frontend/src/modules/shell/registry.ts`, panel dirender dengan props `{}`, hanya panel aktif di-mount).
- Perubahan working tree di `frontend/src/modules/layout/*`, `frontend/src/lib/stores/layout.ts`, `internal/provider/*`, `frontend/src/modules/thread/participants/terminal.ts` sudah di-commit sebelum fitur `lark-pane-foundation` dikerjakan. Fitur lain boleh jalan lebih awal.

## Kontrak
Disediakan:

Config (`internal/workspace/types.go`, json camelCase, tanpa token):
```go
// WorkspaceConfig: tambah satu field
Lark *LarkConfig `json:"lark,omitempty"`

type LarkConfig struct {
    Tickets []TicketSource `json:"tickets"`
    Wiki    []WikiSource   `json:"wiki"`
}
type TicketSource struct {
    ID        string `json:"id"`                // ULID, dibuat di Go
    Name      string `json:"name"`
    BaseToken string `json:"baseToken"`
    Host      string `json:"host"`
    TableID   string `json:"tableId,omitempty"`
    ViewID    string `json:"viewId,omitempty"`
}
type WikiSource struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    SpaceID   string `json:"spaceId"`
    RootToken string `json:"rootToken,omitempty"` // kosong = akar space
    Host      string `json:"host"`
}
```
Wiki pribadi tidak disimpan: `spaceId` `my_library`, `sourceId` literal `"personal"`.

Fungsi di `internal/workspace/lark.go` (lock + tulis atomik, publikasi `configChanged` seperti Grimoire; tidak mengimpor `larkdata`):
- `LarkSources(workspaceID) (LarkConfig, error)`
- `AddLarkTicketSource(workspaceID, TicketSource) (TicketSource, error)` dan `AddLarkWikiSource(workspaceID, WikiSource) (WikiSource, error)` (ID ULID dibuat di sini; tolak duplikat `baseToken` / `spaceId+rootToken` dengan pesan jelas)
- `RemoveLarkSource(workspaceID, sourceID) error`, `RenameLarkSource(workspaceID, sourceID, name) error` (nama tidak boleh kosong; sumber tak ada -> error)
- Nama persis mengikuti konvensi paket workspace yang ada; periksa kode Grimoire di paket yang sama.

IPC (didaftarkan di `wire_lark.go` dan `wire_lark_link.go`; hasil dinormalisasi; galat memakai kode connector yang sama + pesan Indonesia):
- `lark.sources.get {workspaceId}` -> `{tickets: TicketSource[], wiki: WikiSource[]}` (kosong bila belum ada config; tanpa wiki pribadi)
- `lark.sources.add {workspaceId, kind:'ticket', name, baseToken, host, tableId?, viewId?}` atau `{workspaceId, kind:'wiki', name, spaceId, rootToken?, host}` -> sumber yang dibuat (dengan `id`)
- `lark.sources.remove {workspaceId, sourceId}` -> `{}`
- `lark.sources.rename {workspaceId, sourceId, name}` -> `{}`
- `lark.link.parse {link}` -> `{kind:'base'|'wiki', host, baseToken?, tableId?, viewId?, spaceId?, nodeToken?, defaultName?}`; link `/wiki/<token>` berisi bitable dikembalikan `kind:'base'` dengan `baseToken` = `obj_token` hasil `wiki +node-get`; link `/wiki/` ke docs/sheet -> `kind:'wiki'` dengan `spaceId` dan `nodeToken`; link tak dikenal -> galat `invalid_link` "Link tidak dikenali".
- `lark.tables.list {workspaceId, sourceId}` -> `{tables:[{id,name}]}`
- `lark.views.list {workspaceId, sourceId, tableId}` -> `{views:[{id,name,type?}]}`
- Event `configChanged` dipakai ulang (tanpa event baru).

Go internal (`internal/larkdata`):
- `ParseLink(ctx, link string) (ParsedLink, error)` di `link.go`.
- `ListTables(ctx, baseToken, host)` dan `ListViews(ctx, baseToken, tableID)` di `tables.go`; keduanya serta resolusi nama default hanya memanggil `Run()` dari `runner.go` (tiket 01).

Frontend:
- Item rail `{ id: 'lark', title: 'Lark', icon: LarkIcon, component: LarkPanel, order: 10, visible: <connector lark tersedia> }`.
- Jenis pane `lark-board` props `{ sourceId, tableId?, viewId? }` dan `lark-doc` props `{ sourceId | 'personal', nodeToken }`.
- Field Leaf baru `sourceId`, `tableId`, `viewId`, `nodeToken` masuk daftar field yang disalin loader layout.
- `frontend/src/modules/lark/actions.ts`: `copyLink(url: string): Promise<void>` dan `sendToAgent(url: string): Promise<void>`.
- Titik ekstensi untuk tiket lain: `board/BoardPane.svelte`, `doc/DocPane.svelte`, `sections/WikiTree.svelte` (placeholder yang akan ditimpa); `sections/WikiSection.svelte` memberi `WikiTree` props `{ source, onOpen(nodeToken) }` di mana `source` bertipe `WikiSource | { id: 'personal', name: 'Pribadi', spaceId: 'my_library', host }`.
- `store.ts` menyediakan: `larkSources` (writable berisi `LarkConfig` workspace aktif), `larkState` (`loading|ready|error` + pesan/kode galat), `refreshSources()`.
- `client.ts`: pembungkus IPC bertipe untuk semua method di atas, memakai klien IPC yang sudah ada di frontend.

Dipakai:
- Dari tiket 01: connector id `lark` dan Status (`connected|cli_missing|not_logged_in`, `User`), `larkdata.Run()` (allowlist baca, `--as user`, format json, timeout daftar 20 detik). Arah dependensi: `larkdata -> connector`; `workspace` tidak mengimpor `larkdata`; `wire_lark*.go` mengimpor keduanya.
- Dari rail: `registerRailItem`, `RailItemSpec`.
- Dari layout: `registerPaneKind`, perintah membuka pane (mis. fungsi `openPane`/`openThreadPane` sejenis di `lib/stores/layout.ts`; periksa nama persis), `pasteToSession` (jalur drag-drop path file), sistem toast, `activeWorkspaceId`, store status connector.
- Dari frontend lain: dialog gaya yang sudah ada (mis. `NewThreadDialog`) sebagai acuan tampilan.

Aturan tetap:
- Teks UI bahasa Indonesia, identifier kode bahasa Inggris.
- Tidak menyimpan token; tidak memanggil CLI di luar `larkdata.Run()`.
- Setiap fitur Go mendaftarkan IPC di file `wire_lark*.go` miliknya sendiri; satu file satu pemilik.
- Aturan pemotongan: bila `docs/lark/cli-findings.md` (spike tiket 01) membantah asumsi (mis. `wiki +node-get` tidak mengembalikan `obj_token` bitable, `my_library` tak bisa di-list), fitur terkait dipotong dan dilaporkan (link `/wiki/` bitable ditolak dengan pesan jelas; wiki Pribadi disembunyikan dengan keterangan), pekerjaan lain lanjut.

## Fitur

### lark-config-ipc: LarkConfig dan IPC sumber
Deskripsi:
Tambah `Lark *LarkConfig` ke `WorkspaceConfig` di `internal/workspace/types.go` (hanya satu field dan tiga tipe; jangan ubah field lain). Buat `internal/workspace/lark.go` berisi operasi per item yang aman bersamaan: ambil lock yang sama dengan penulisan config lain di paket itu, baca config, ubah satu item, tulis atomik (file sementara + rename, ikuti pola penyimpanan workspace yang ada), lalu publikasikan `configChanged` persis seperti jalur Grimoire. Jangan menyentuh pemetaan field-per-field `config.set` di `internal/workspace/ipc.go` dan pastikan config Grimoire tidak berubah saat menulis `Lark`. ID dibuat di Go (ULID; pakai pustaka ULID yang sudah ada di repo, bila tidak ada pakai generator ID yang sudah dipakai untuk thread/workspace; jangan menambah dependensi bila bisa dihindari). `add` menolak `name` kosong (default ke nama yang diberikan frontend) dan duplikat. Daftarkan IPC `lark.sources.get/add/remove/rename` di `cmd/shikigamid/wire_lark.go` mengikuti pola `wire_grimoire.go`; `get` mengembalikan daftar kosong (bukan null) bila `Lark` nil. Wiki pribadi tidak pernah ditulis. Galat memakai bentuk galat IPC yang ada dengan pesan Indonesia (mis. "Sumber tidak ditemukan", "Nama tidak boleh kosong", "Sumber sudah ada").

Acceptance criteria:
- [ ] `WorkspaceConfig` memiliki `Lark *LarkConfig` dengan tag json sesuai Kontrak; config lama tanpa `lark` tetap terbaca.
- [ ] `lark.sources.add` untuk `kind:'ticket'` dan `kind:'wiki'` menyimpan sumber dengan ID ULID dan mengembalikannya; `remove` dan `rename` bekerja per `sourceId`.
- [ ] Dua `add` bersamaan tidak saling menimpa (lock + tulis atomik).
- [ ] Setiap perubahan memublikasikan `configChanged`.
- [ ] Config Grimoire dan field workspace lain tidak berubah oleh operasi Lark; `config.set` tidak dimodifikasi.
- [ ] Tidak ada token/kredensial dalam config; wiki pribadi tidak tersimpan.
- [ ] `internal/workspace` tidak mengimpor `internal/larkdata`.
- [ ] `go build ./...` lulus.

ownedPaths:
- `internal/workspace/types.go` (hanya penambahan `Lark` pada `WorkspaceConfig` dan tipe Lark)
- `internal/workspace/lark.go`
- `cmd/shikigamid/wire_lark.go`

Catatan: bisa dikerjakan paralel dengan `lark-link-parser`, `lark-pane-foundation`, dan `lark-panel-ui` (frontend memakai kontrak IPC, bukan implementasi). `lark-sources-wiring` berjalan sesudahnya.

### lark-link-parser: Parser link Base dan wiki
Deskripsi:
Di `internal/larkdata/link.go` urai link yang ditempel (trim spasi, hanya `http/https`, host dipertahankan apa adanya, mis. `xxx.larksuite.com` atau `xxx.feishu.cn`):
- Base: `https://<host>/base/<baseToken>?table=<tableId>&view=<viewId>` -> `kind:'base'` dengan `host`, `baseToken`, `tableId?`, `viewId?`. Verifikasi pola URL dengan fixture/temuan `docs/lark/cli-findings.md`; toleran terhadap fragmen dan parameter ekstra.
- Wiki: `https://<host>/wiki/<nodeToken>` -> panggil `wiki +node-get` (lewat `Run()`) untuk mendapat `space_id`, `obj_type`, `obj_token`, `title`. Bila `obj_type` = bitable: kembalikan `kind:'base'` dengan `baseToken` = `obj_token` dan `host` link asli. Selain itu `kind:'wiki'` dengan `spaceId`, `nodeToken` (dijadikan `rootToken`), `defaultName` = judul node. Link space (bila CLI/URL mendukung) -> `kind:'wiki'` tanpa `nodeToken`; bila format tak dapat dipastikan dari spike, tolak dengan pesan jelas.
- Link tak dikenal atau bukan Lark -> galat `invalid_link` "Link tidak dikenali".
Di `tables.go`: `ListTables` (`base +table-list`) dan `ListViews` (`base +view-list`) dinormalisasi ke `{id,name}` dan `{id,name,type?}`; resolusi nama default untuk Base lewat info Base/`+table-list` (nama Base bila ada, jika tidak nama tabel pertama atau "Base"). Daftarkan IPC `lark.link.parse`, `lark.tables.list`, `lark.views.list` di `cmd/shikigamid/wire_lark_link.go`; `lark.tables.list`/`lark.views.list` mengambil `baseToken` dari sumber di config lewat `workspace` (wire file mengimpor keduanya), memakai timeout daftar 20 detik, dan meneruskan galat connector (`cli_missing`, `not_logged_in`, izin kurang, `timeout`) dengan pesan Indonesia. Hanya memanggil `Run()`; jangan memanggil `exec` langsung dan jangan mengubah `runner.go`/`allowlist.go`. Bila `base +view-list` atau `wiki +node-get` perlu masuk allowlist tetapi belum ada, laporkan sebagai ketergantungan ke tiket 01 (jangan edit file miliknya).

Acceptance criteria:
- [ ] Link Base dengan/ tanpa `table` dan `view` terurai menjadi host, `baseToken`, `tableId?`, `viewId?`.
- [ ] Link `/wiki/` yang berisi bitable di-resolve ke `baseToken` lewat `wiki +node-get`; link `/wiki/` docs/sheet menghasilkan `kind:'wiki'` dengan `spaceId` dan `nodeToken`.
- [ ] Link tak dikenal menghasilkan galat `invalid_link` berpesan Indonesia, bukan panic.
- [ ] `lark.tables.list` dan `lark.views.list` mengembalikan daftar dinormalisasi; galat CLI diteruskan dengan kode connector yang sama.
- [ ] Semua panggilan CLI lewat `Run()`; tidak ada `exec` langsung.
- [ ] IPC terdaftar hanya di `wire_lark_link.go`.
- [ ] `go build ./...` lulus.

ownedPaths:
- `internal/larkdata/link.go`
- `internal/larkdata/tables.go`
- `cmd/shikigamid/wire_lark_link.go`

Catatan: bisa dikerjakan paralel dengan `lark-config-ipc`, `lark-pane-foundation`, dan `lark-panel-ui`. Membaca (bukan mengubah) `larkdata/runner.go` dari tiket 01.

### lark-pane-foundation: Fondasi pane lark-* di layout
Deskripsi:
Hanya dikerjakan setelah perubahan `layout/*` di working tree masuk (commit). Isi:
- Buat `frontend/src/modules/lark/board/BoardPane.svelte` dan `frontend/src/modules/lark/doc/DocPane.svelte` sebagai komponen placeholder berisi teks "Segera hadir" (menerima props `{ sourceId, tableId?, viewId? }` dan `{ sourceId, nodeToken }`; tampilkan judul jenis pane). File ini AKAN DITIMPA tiket `lark-tickets` dan `lark-wiki`; jaga signature props sesuai Kontrak. Sertakan di placeholder pemeriksaan sumber: bila `sourceId` bukan `'personal'` dan tidak ada di sumber workspace aktif (store `larkSources` dari `lark-panel-ui`, atau cek langsung lewat `lark.sources.get` bila store belum tersedia), tampilkan "Sumber tidak ada di workspace ini". Karena tiket lain menimpa file ini, logika pemeriksaan itu ditaruh juga di komponen pembungkus kecil `frontend/src/modules/lark/index.ts` (mis. fungsi `resolveSource(sourceId)` yang diekspor) agar bisa dipakai ulang oleh pane pengganti.
- `frontend/src/modules/lark/index.ts` mendaftarkan `registerPaneKind('lark-board', ...)` dan `registerPaneKind('lark-doc', ...)` (judul tab dari props, mis. nama sumber). Bagian pendaftaran rail item ada di `lark-sources-wiring`; fitur ini hanya menambah pendaftaran pane dan ekspor helper, dengan struktur file yang mudah digabung.
- Perluas loader layout di `frontend/src/modules/layout/model.ts` dan `frontend/src/lib/stores/layout.ts`: daftar field Leaf yang disalin (`sessionId`, `projectId`, `filePath`, `threadId`, dst.) ditambah `sourceId`, `tableId`, `viewId`, `nodeToken` (ikut tipe Leaf, serialisasi, dan pemulihan). Jenis pane tak dikenal (mis. build tanpa modul Lark) dirender sebagai keadaan "Pane tidak dikenal" (menampilkan jenisnya), tidak dibuang diam-diam, dan tetap ikut tersimpan ulang. Pilih perubahan paling kecil di kode yang ada; jangan merombak model layout.
- `frontend/src/modules/lark/actions.ts`: `copyLink(url)` menyalin ke clipboard (`navigator.clipboard.writeText`, fallback bila gagal) dan menampilkan toast "Link disalin"; `sendToAgent(url)` memakai `pasteToSession` ke sesi aktif tanpa Enter otomatis; bila tak ada sesi aktif tampilkan toast "Tidak ada sesi aktif". Hanya sesi terminal (bukan peserta thread).

Acceptance criteria:
- [ ] Jenis pane `lark-board` dan `lark-doc` terdaftar dan bisa dibuka lewat perintah buka pane; menampilkan placeholder "Segera hadir".
- [ ] Field `sourceId`, `tableId`, `viewId`, `nodeToken` ikut tersimpan dan pulih setelah restart.
- [ ] Jenis pane tak dikenal tampil sebagai "Pane tidak dikenal" tanpa crash dan tidak hilang dari layout tersimpan.
- [ ] Pane dengan `sourceId` yang tidak ada di workspace aktif menampilkan "Sumber tidak ada di workspace ini".
- [ ] `copyLink` menyalin dan menampilkan toast; `sendToAgent` menempel link ke sesi aktif tanpa Enter, atau toast "Tidak ada sesi aktif".
- [ ] Perilaku layout untuk pane yang ada (terminal, file, thread, dst.) tidak berubah.
- [ ] Build frontend lulus.

ownedPaths:
- `frontend/src/modules/lark/index.ts` (bagian registrasi pane dan helper; wiring rail oleh `lark-sources-wiring`)
- `frontend/src/modules/lark/actions.ts`
- `frontend/src/modules/lark/board/BoardPane.svelte`
- `frontend/src/modules/lark/doc/DocPane.svelte`
- `frontend/src/modules/layout/model.ts`
- `frontend/src/lib/stores/layout.ts`
- komponen renderer pane tak dikenal di `frontend/src/modules/layout/` bila perlu (mis. `LayoutArea.svelte`; sentuh seminimal mungkin dan hanya setelah perubahan working tree di-commit)

Catatan: bisa dikerjakan paralel dengan fitur Go dan `lark-panel-ui`, dengan syarat prasyarat commit layout terpenuhi. Hanya fitur ini yang menyentuh `layout/*`.

### lark-panel-ui: LarkPanel, sumber, dan dialog Tambah
Deskripsi:
Bangun UI panel di `frontend/src/modules/lark/`:
- `client.ts`: fungsi bertipe untuk IPC `lark.sources.get/add/remove/rename`, `lark.link.parse`, `lark.tables.list`, `lark.views.list` memakai klien IPC frontend yang ada (lihat `grimoire` atau `thread-client.ts` sebagai pola). Galat dinormalisasi menjadi `{ code, message }`.
- `store.ts`: `larkSources`, `larkState`, `refreshSources()`; berlangganan `activeWorkspaceId` dan event `configChanged` (pola sama modul grimoire/workspace) sehingga isi berganti saat workspace aktif berganti atau config berubah. Subscriber event dipasang di level modul (bukan di komponen) agar tidak hilang saat panel di-unmount. Tidak ada polling.
- `LarkIcon.svelte`: SVG inline 24x24, `stroke="currentColor"`, `fill="none"`, `aria-hidden="true"`, mengikuti gaya ikon rail lain.
- `LarkPanel.svelte` (props `{}`): dua bagian lipat **Tiket** dan **Wiki** (status lipat boleh disimpan di store modul), tombol segarkan, keadaan memuat, galat (pesan terbaca: "lark-cli belum terpasang", "Belum login Lark", "Izin Lark kurang" + pesan CLI, "Waktu habis"; untuk `cli_missing`/`not_logged_in` sertakan ajakan menuju Pengaturan > Connector, memakai cara membuka pengaturan yang sudah ada).
- `sections/TicketSection.svelte`: daftar Base (nama + ikon), tombol **Tambah Base**, menu per item (ganti nama inline, hapus dengan konfirmasi singkat), keadaan kosong "Belum ada Base. Tambah Base."; klik membuka pane `lark-board` `{sourceId}` lewat perintah buka pane layout.
- `sections/WikiSection.svelte`: **Pribadi** paling atas (disembunyikan dengan keterangan bila ternyata `my_library` gagal, mengikuti aturan pemotongan), lalu wiki tim; tombol **Tambah wiki**; ganti nama/hapus untuk wiki tim saja; klik membuka pane `lark-doc` akar `{sourceId, nodeToken: rootToken ?? ''}` dan menyerahkan `{ source, onOpen(nodeToken) }` ke `sections/WikiTree.svelte`.
- `sections/WikiTree.svelte`: placeholder daftar datar sederhana (hanya nama sumber dan satu baris akar yang bisa diklik memanggil `onOpen`); akan DITIMPA tiket `lark-wiki`, jaga props `{ source, onOpen }`.
- `AddSourceDialog.svelte`: props `kind: 'ticket' | 'wiki'`; satu kolom tempel link; saat ditempel/blur panggil `lark.link.parse`; tampilkan hasil singkat (Base: nama default editable; wiki: nama default editable dan keterangan "akar: <judul node>" bila node). Tombol Simpan memanggil `lark.sources.add`; kesalahan jenis (mis. menempel link wiki di dialog Base yang bukan bitable) ditolak dengan pesan jelas; galat parse ditampilkan inline dan dialog tetap terbuka. Untuk Base bila link tanpa tabel, tidak perlu memilih tabel (dipilih di header pane `lark-board` oleh tiket lain).
- Daftarkan komponen yang dipakai wiring lewat ekspor jelas; pendaftaran `registerRailItem` dilakukan di `lark-sources-wiring`.
Komponen memakai token tema/CSS yang sudah ada di modul lain; tidak ada dependensi baru.

Acceptance criteria:
- [ ] `LarkPanel` (props `{}`) menampilkan dua bagian lipat Tiket dan Wiki, tombol segarkan, dan state memuat/kosong/galat dengan pesan Indonesia.
- [ ] Keadaan kosong Tiket menampilkan "Belum ada Base. Tambah Base."
- [ ] `AddSourceDialog` menempel link, mengurai lewat IPC, menampilkan nama default yang bisa diubah, dan menyimpan; galat parse tampil inline.
- [ ] Ganti nama dan hapus sumber bekerja dari panel dan langsung terlihat setelah `configChanged`.
- [ ] Wiki Pribadi selalu di paling atas dan tidak bisa dihapus/diganti nama.
- [ ] `WikiSection` menyerahkan `{ source, onOpen }` ke `WikiTree`; `WikiTree` placeholder bisa ditimpa tanpa mengubah `WikiSection`.
- [ ] Isi berganti saat workspace aktif berganti; subscriber event tidak dipasang di komponen.
- [ ] Build frontend lulus.

ownedPaths:
- `frontend/src/modules/lark/LarkPanel.svelte`
- `frontend/src/modules/lark/LarkIcon.svelte`
- `frontend/src/modules/lark/client.ts`
- `frontend/src/modules/lark/store.ts`
- `frontend/src/modules/lark/AddSourceDialog.svelte`
- `frontend/src/modules/lark/sections/` (`TicketSection.svelte`, `WikiSection.svelte`, `WikiTree.svelte`)

Catatan: bisa dikerjakan paralel dengan semua fitur lain di epic ini; bergantung pada kontrak IPC (bukan implementasi Go) dan hanya memanggil nama method di Kontrak. Verifikasi end-to-end menunggu `lark-sources-wiring`.

### lark-sources-wiring: Wiring rail Lark dan ctl sources
Deskripsi:
Fitur WIRING; dikerjakan setelah empat fitur lain selesai.
- `frontend/src/modules/lark/index.ts`: tambahkan `registerRailItem({ id: 'lark', title: 'Lark', icon: LarkIcon, component: LarkPanel, order: 10, visible: <connector lark tersedia> })` di samping registrasi pane dari `lark-pane-foundation` (satu berkas, gabungkan tanpa menghapus bagian itu). `visible` mengikuti status connector `lark` dari store connector yang sudah ada (bukan `cli_missing`; bila store belum bisa menyatakan "tersedia", tampilkan selalu dan biarkan panel menampilkan galat).
- Impor modul lark di entry modul frontend (satu baris `import './modules/lark'` pada file bootstrap modul yang sudah mengimpor modul lain; ikuti konvensi, jangan ubah shell/rail tiket 1).
- `cmd/shikigamid/modules.go`: pastikan `wire_lark.go` dan `wire_lark_link.go` (dan file `wire_lark*.go` lain yang ada) terdaftar mengikuti konvensi file tersebut; tambah hanya baris yang diperlukan.
- `cmd/shikigamictl/cmd_lark_sources.go`: subperintah `shikigamictl lark sources get|add|remove|rename` memakai klien IPC ctl yang ada dan didaftarkan lewat konvensi `registry.go` (satu berkas ini tidak boleh mengubah `cmd_lark.go` milik tiket 01; bila perlu hook, pakai mekanisme registrasi subperintah yang ada, jika tidak ada laporkan). Contoh: `lark sources get --workspace <id>`, `lark sources add --workspace <id> --kind ticket --link <url> [--name ...]` (ctl memanggil `lark.link.parse` lalu `lark.sources.add`), `remove --workspace <id> --source <sid>`, `rename --workspace <id> --source <sid> --name ...`. Keluaran JSON.
- Verifikasi build dan jalankan aplikasi (lihat Wiring).

Acceptance criteria:
- [ ] Item rail `lark` terdaftar dengan order 10 dan judul "Lark"; modul lark diimpor di bootstrap.
- [ ] `wire_lark*.go` terdaftar di `modules.go`; semua method IPC di Kontrak terjawab oleh daemon.
- [ ] `shikigamictl lark sources get|add|remove|rename` bekerja dan perubahannya terlihat di panel (event `configChanged`).
- [ ] Tambah/hapus/ganti nama Base dan wiki dari panel tetap ada setelah restart aplikasi.
- [ ] `go build ./...` dan build frontend lulus.

ownedPaths:
- `cmd/shikigamid/modules.go`
- `cmd/shikigamictl/cmd_lark_sources.go`
- `frontend/src/modules/lark/index.ts`

Catatan: bergantung pada `lark-config-ipc`, `lark-link-parser`, `lark-pane-foundation`, dan `lark-panel-ui` (urutan terakhir).

## Path yang dimiliki
Hanya ini yang boleh disentuh:
- `internal/workspace/types.go` (hanya penambahan `Lark` dan tipe Lark)
- `internal/workspace/lark.go`
- `internal/larkdata/link.go`
- `internal/larkdata/tables.go`
- `cmd/shikigamid/wire_lark.go`
- `cmd/shikigamid/wire_lark_link.go`
- `cmd/shikigamid/modules.go` (hanya baris pendaftaran)
- `cmd/shikigamictl/cmd_lark_sources.go`
- `frontend/src/modules/lark/index.ts`
- `frontend/src/modules/lark/LarkPanel.svelte`
- `frontend/src/modules/lark/client.ts`
- `frontend/src/modules/lark/store.ts`
- `frontend/src/modules/lark/actions.ts`
- `frontend/src/modules/lark/AddSourceDialog.svelte`
- `frontend/src/modules/lark/LarkIcon.svelte`
- `frontend/src/modules/lark/sections/` (`TicketSection.svelte`, `WikiSection.svelte`, `WikiTree.svelte`)
- `frontend/src/modules/lark/board/BoardPane.svelte` (placeholder; ditimpa `lark-tickets`)
- `frontend/src/modules/lark/doc/DocPane.svelte` (placeholder; ditimpa `lark-wiki`)
- `frontend/src/modules/layout/model.ts` dan `frontend/src/lib/stores/layout.ts` (hanya penambahan field referensi Lark)
- renderer pane tak dikenal di `frontend/src/modules/layout/` (seminimal mungkin, bila diperlukan)
- Entry bootstrap modul frontend (hanya satu baris impor modul lark)

Tidak boleh disentuh: `internal/connector/*`, `internal/larkdata/runner.go`, `allowlist.go`, `testdata/`, `docs/lark/`, `cmd/shikigamictl/cmd_lark.go` (tiket 01); `internal/larkdata/tickets.go`, `ticketlink.go`, `frontend/src/modules/lark/board/*` selain placeholder awal (tiket 03); `internal/larkdata/wiki.go`, `doc.go`, `frontend/src/modules/lark/doc/*` selain placeholder awal, `frontend/package.json` (tiket 04); shell/rail tiket `main-rail`.

## Wiring
Langkah penutup tiket ini, memakai titik ekstensi tiket `main-rail-ticket-01-main-rail` (`registerRailItem`) dan registrasi modul daemon/ctl yang ada:
1. Pastikan `lark-config-ipc`, `lark-link-parser`, `lark-pane-foundation`, dan `lark-panel-ui` selesai dan prasyarat commit `layout/*` terpenuhi.
2. Selesaikan `lark-sources-wiring`: item rail `lark` terdaftar, modul lark diimpor di bootstrap, `wire_lark*.go` terdaftar di `modules.go`, `shikigamictl lark sources` tersedia.
3. Jalankan aplikasi dan periksa: rail memuat item Lark; panel menampilkan Tiket dan Wiki (Pribadi di paling atas); tempel link Base valid, nama default terisi, simpan, Base muncul; klik membuka pane `lark-board` placeholder; tambah wiki dari link node, muncul dan klik membuka pane `lark-doc` placeholder; ganti nama dan hapus bekerja; berganti workspace mengganti isi panel; restart aplikasi, sumber dan pane yang terbuka tetap ada; hapus Base lalu pane yang masih terbuka menampilkan "Sumber tidak ada di workspace ini"; link tak valid menampilkan galat di dialog; tanpa login `lark-cli` panel menampilkan ajakan ke Pengaturan > Connector; `shikigamictl lark sources get` mengembalikan data yang sama dengan panel.
4. Tiket ini berjalan sendiri di atas tiket 01 dan tiket rail; tidak bergantung pada `lark-tickets` maupun `lark-wiki` (placeholder tetap jalan tanpa keduanya).

## Guard
- `go build ./...` lulus.
- `cd frontend && npm run build` (atau perintah build frontend yang dipakai proyek) lulus tanpa galat tipe/svelte-check.
- Tanpa unit test.
