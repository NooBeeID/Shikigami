---
id: shikigami-v2-ui-ticket-07-session-groups
title: Grup sesi sebagai tab
spec: docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md
status: ready
wave: 6
depends_on: [shikigami-v2-ui-ticket-06-workspace-persist]
---
# Grup sesi sebagai tab

## Tujuan
Menambah grup sesi sebagai tab di atas area layout. Grup adalah satu konteks kerja milik workspace (mis. per fitur, cabang, atau tugas) yang berisi sekumpulan sesi dan file terbuka dengan layout pane sendiri (spec bagian 5b). Dengan begitu pengguna yang mengawasi banyak agent bisa memisahkan pekerjaan tanpa satu layout yang makin padat.

Tiket 06 sudah menyediakan skema data grup (`Workspace.Groups`, `ActiveGroupID`, `Group{ID, Name, Order, Layout}`) dan hanya method IPC `workspace.layout.get/set` dan `workspace.ui.get/set`. Belum ada cara mengelola grup (buat, ganti nama, hapus, urut, ganti grup aktif) maupun keanggotaan sesi. Tiket ini menambahnya di daemon sehingga **daemon dan `workspaces.json` menjadi satu-satunya sumber kebenaran** untuk grup; frontend hanya membaca dan menulis lewat method IPC baru (tanpa `localStorage`, tanpa file sampingan, tanpa cermin state), lalu menambah store dan UI tab di atasnya.

## Hasil yang bisa dijalankan
Di atas tiket 06 (workspace persist), 04 (shell dan sidebar), dan 03 (layout multi-pane):
- Di atas area layout tampil deretan tab grup. Tiap tab menampilkan nama dan jumlah sesi grup itu. Tiap workspace selalu punya minimal satu grup bernama "Utama" (grup bawaan, tidak bisa dihapus).
- Klik tab mengganti grup aktif; area layout langsung menampilkan layout grup itu (pane, split, dan fokus masing-masing). Terminal tidak dibuat ulang saat berpindah grup: sesi yang tidak sedang tampil tetap hidup di daemon dan di-attach kembali saat grupnya aktif lagi.
- Tombol `+` membuat grup baru (nama bawaan "Grup 2", "Grup 3", dst.) berlayout kosong dan langsung mengaktifkannya. Menu konteks tab: Ganti nama (edit inline, Enter simpan, Esc batal) dan Hapus (tidak tersedia untuk "Utama").
- Tab bisa diurutkan dengan drag.
- Sesi dipindah antar grup lewat menu sesi "Pindah ke grup" (di sidebar kiri dan menu `...` header pane) atau dengan menyeret sesi/header pane ke tab grup lain.
- Menghapus grup menutup pane-nya dan memindahkan sesinya ke "Utama"; sesi tidak dibunuh dan tetap terlihat di sidebar kiri.
- Sidebar kiri menampilkan sesi grup aktif lebih dulu, lalu grup lain sebagai bagian yang dilipat.
- Grup aktif, daftar grup, urutan, keanggotaan sesi, dan layout tiap grup tersimpan di `workspaces.json` (lewat daemon dan persist tiket 06) dan pulih setelah aplikasi atau daemon dimulai ulang. Grup yang dibuat ikut terlihat di `workspace.list`.
- `go build ./...` dan `make build` lulus dan aplikasi bisa dijalankan.

## Batasan
In scope:
- Daemon (paket `internal/workspace`): berkas baru `groups.go` dan `ipc_groups.go`, method IPC `workspace.group.*`, event `workspace.groupsChanged`, serta perluasan aditif minimal yang disebut eksplisit di bagian "Path yang dimiliki" (types, store, load, registrasi hook).
- Store `lib/stores/groups.ts`: membaca dan menulis grup lewat method daemon, mengganti layout aktif saat grup berpindah.
- Modul UI `modules/groups`: `GroupTabs.svelte`, menu konteks, drag urut, drag sesi ke tab, menu "Pindah ke grup", bagian sidebar kiri bergrup.
- Perluasan sangat kecil pada `modules/shell`, `modules/sidebar-left`, dan `modules/persist` (autosave), dicatat di Wiring.

Out of scope (spec 5b, bagian "Yang tidak ada di v2"):
- Grup lintas workspace, satu sesi di banyak grup, jalankan/hentikan semua sesi dalam grup, template grup.
- Mengubah logika pohon layout (milik tiket 03) atau mekanisme persist (milik tiket 06) selain perluasan kecil yang disebut di Wiring.
- Mode tampilan per project, layout bernama, meter sumber daya.
- Grup bawaan selain "Utama" dan arti khusus untuk nama grup.

Batasan perubahan:
- Perubahan skema `workspaces.json` hanya aditif dan **tidak menaikkan versi** (tetap v2). File v2 yang sudah ditulis tiket 06 tanpa field baru harus dinormalisasi di memori saat load, tanpa wajib menulis ulang file dan tanpa merusak data.
- Flag izin agent (`--dangerously-skip-permissions`, `--dangerously-bypass-approvals-and-sandbox`) tidak disentuh dan tidak digandakan.
- Tanpa `setInterval`/polling/timer baru (frontend maupun daemon); hanya reaksi terhadap aksi pengguna, event, dan perubahan store.
- Tanpa dependency baru (Go maupun npm).
- Jangan mengubah perilaku method `workspace.layout.get/set` dan `workspace.ui.get/set` serta event `workspace.changed` dan `workspace.layoutChanged` yang sudah ada; hanya menambah method dan event baru.

## Kontrak
Dipakai dari epic lain (tidak diubah perilakunya di sini):
- Skema workspace v2 (tiket 06): `Workspace {id, name, projects[], config, groups:[{id, name, order, layout: LayoutDoc|null}], activeGroupId}`. Kode Go: `Workspace`, `Group`, `Store` (`GetLayout`, `SetLayout`, `GetUI`, `SetUI`, `persistLocked`, `cloneWorkspace`), sentinel `ErrNotFound`/`ErrInvalid`/`ErrConflict` dan `mapErr` di `ipc.go`, `findGroup` di `layout.go`, `newID()`/`newDefaultGroup()` di `types.go`, `loadWorkspaces` di `config.go`. Method IPC yang sudah ada: `workspace.layout.get/set {workspaceId, groupId, layout?}` (`groupId` kosong = grup aktif), `workspace.ui.get/set`, event `workspace.changed` dan `workspace.layoutChanged`.
- Dokumen layout (tiket 03): `{version:1, root:Node|null, focusId?:string, maximizedId?:string|null}`; aksi store layout `openPane`, `closePane` (tidak membunuh sesi), `hydrate(doc)`, `serialize()`. Fungsi murni model (`leaves`, `closePane`) di `modules/layout/model` hanya diimpor, tidak diubah. Elemen DOM/instance xterm per pane stabil, dipindah bukan dibuat ulang.
- Persist (tiket 06): `modules/persist/autosave.ts` (`setBaseline`, `flush`, `scheduleSave`, `currentGroupId` internal), `modules/persist/restore.ts` (`restoreWorkspace`, mengambil layout lewat `workspace.layout.get` tanpa `groupId` = grup aktif daemon lalu `setBaseline(wsId, groupId)`), dan `lib/stores/persist.ts` (`flushNow`, `persistStatus`, `setPersistStatus`).
- Frontend yang sudah ada: `lib/stores/workspaces.ts` (`workspaces`, `activeWorkspace`, `activeWorkspaceId`, `loadWorkspaces`), `lib/stores/sessions.ts` (`sessions`, `loadSessions`, `createSession`), `lib/stores/layout.ts` (`layout`), `lib/stores/init.ts` (event `workspace.changed` memicu `loadWorkspaces`; tidak diubah), `lib/ipc.ts` (`call`, `onEvent`).
- Shell (tiket 04): slot area atas layout (lihat Wiring); `registerRightTab`/`registerTopbarItem` tidak dipakai. Sidebar kiri (tiket 04): daftar sesi dan menu sesi.

### Bentuk data Workspace yang diperluas (aditif, skema tetap v2)
Go (`internal/workspace/types.go`):
```go
type Workspace struct {
    // ...field tiket 06 tidak berubah...
    Groups        []Group           `json:"groups"`
    ActiveGroupID string            `json:"activeGroupId"`
    Membership    map[string]string `json:"membership"` // sessionId -> groupId; selalu non-nil saat keluar
}
type Group struct {
    ID     string          `json:"id"`
    Name   string          `json:"name"`
    Order  int             `json:"order"`
    Main   bool            `json:"main,omitempty"` // penanda grup bawaan "Utama"; tepat satu per workspace
    Layout json.RawMessage `json:"layout"`
}
```
Aturan data:
- Penanda grup bawaan adalah field `Group.Main`, bukan nama. Dengan begitu mengganti nama grup bawaan tidak memutus identitasnya. `newDefaultGroup()` mengisi `Main: true`.
- Normalisasi saat load (di `loadWorkspaces`, satu pemanggilan fungsi di `groups.go`, hanya di memori; file v2 tidak wajib ditulis ulang dan baru berubah pada persist berikutnya):
  1. Urutkan grup menurut `Order` (stabil, tie menurut posisi di file) lalu beri nomor ulang rapat 0..n-1.
  2. Pastikan tepat satu grup `Main`: bila ada lebih dari satu, pertahankan yang ber-order terkecil; bila tidak ada (file v2 tiket 06), pilih grup pertama (order terkecil) yang namanya "Utama" (tanpa beda huruf besar/kecil, setelah trim), dan bila tidak ada yang bernama itu, grup ber-order terkecil. Tidak pernah membuat grup baru.
  3. `Membership` dikosongkan saat load (sesi tidak dipersist antar restart daemon, jadi semua entri lama pasti basi) dan dijamin non-nil. `cloneWorkspace` menyalin map sehingga `workspace.list`/`Get` selalu mengeluarkan `{}`, bukan `null`.
- Keanggotaan sesi: sesi tanpa entri di `Membership`, atau dengan entri ke grup yang tidak ada, dianggap milik grup `Main`. Daemon paket workspace tidak mengenal daftar sesi, jadi pembersihan entri dilakukan dengan aturan berikut: entri yang menunjuk grup yang sudah tidak ada dibuang pada setiap mutasi grup; entri sesi yang sudah berakhir dibiarkan sampai restart daemon (peta kecil, tidak berpengaruh karena frontend hanya membaca entri untuk sesi yang ada di `sessions`).
- Tepat satu grup per sesi tetap terjaga karena `Membership` memetakan satu sesi ke satu grup.

### Kontrak IPC daemon (JSON-RPC, tiket ini)
Semua method menerima `workspaceId`. Galat memakai kode yang sudah ada lewat `mapErr`: `ErrNotFound` (workspace atau grup tidak ada), `ErrInvalid` (nama tidak valid, parameter kosong), `ErrConflict` (operasi ditolak untuk grup bawaan). Mutasi berhasil dipersist lewat `persistLocked` (tulis atomik, pola salin-lalu-tukar: kloning workspace, ubah salinan, persist, baru tukar `s.workspaces`); bila persist gagal state di memori tidak berubah.

| Method | Params | Efek | Hasil |
|---|---|---|---|
| `workspace.group.create` | `{workspaceId, name?}` | Grup baru di akhir (`order` = n), `layout: null`, id `newID()`. Tidak mengubah grup aktif. | `GroupsState` + `groupId` |
| `workspace.group.rename` | `{workspaceId, groupId, name}` | Ganti nama (boleh untuk grup bawaan). | `GroupsState` |
| `workspace.group.delete` | `{workspaceId, groupId}` | Grup `Main` ditolak dengan `ErrConflict` tanpa mengubah state. Selain itu: hapus grup (layout ikut hilang), entri `Membership` grup itu dipindah ke grup `Main`, order dirapatkan, bila grup aktif maka `Main` jadi aktif. Tidak membunuh sesi. | `GroupsState` |
| `workspace.group.reorder` | `{workspaceId, groupId, toIndex}` | Pindah ke posisi `toIndex` (nilai di luar rentang dijepit ke 0..n-1); order rapat 0..n-1. | `GroupsState` |
| `workspace.group.setActive` | `{workspaceId, groupId}` | Set `ActiveGroupID`. Tidak menyentuh layout. | `GroupsState` |
| `workspace.group.moveSession` | `{workspaceId, sessionId, toGroupId}` | `Membership[sessionId] = toGroupId` (`sessionId` tidak boleh kosong; grup tujuan harus ada). Daemon tidak memeriksa keberadaan sesi. | `GroupsState` |

`GroupsState` (hasil semua method dan payload event) tidak membawa layout agar ringan:
```json
{ "workspaceId": "...", "activeGroupId": "...",
  "groups": [ { "id": "...", "name": "...", "order": 0, "main": true } ],
  "membership": { "<sessionId>": "<groupId>" } }
```
Aturan nama (create bila `name` diisi, dan rename): trim spasi; panjang maksimum 40 karakter (hitung rune; lebih dari itu ditolak `ErrInvalid`, tidak dipotong); kosong ditolak `ErrInvalid` untuk rename. Pada create tanpa nama (atau hanya spasi) nama bawaan "Grup N" dengan N = jumlah grup + 1, dinaikkan sampai unik. Bentrok nama (tanpa beda huruf besar/kecil, terhadap grup lain di workspace yang sama) tidak ditolak: diberi akhiran " 2", " 3", dst. sampai unik (akhiran boleh membuat hasil melewati 40 karakter; batas 40 berlaku untuk input). Mengganti nama grup menjadi namanya sendiri (atau hanya beda huruf besar/kecil) tidak dianggap bentrok.

Event (satu topik baru, tidak memutus konsumen tiket 06): `workspace.groupsChanged` dengan payload `{kind, ...GroupsState}`, `kind` salah satu `groupCreated`, `groupRenamed`, `groupDeleted`, `groupReordered`, `groupActivated`, `sessionMoved`. Dipublikasikan setelah persist berhasil, tidak untuk no-op (mis. `setActive` ke grup yang sudah aktif). Alasan topik baru dan bukan `workspace.changed`: `lib/stores/init.ts` menjawab `workspace.changed` dengan `loadWorkspaces()` penuh (membawa semua layout, hingga 1 MiB per grup) dan `workspace.layoutChanged` khusus layout; topik terpisah dengan payload tanpa layout menghindari muat ulang berat dan tidak mengubah perilaku konsumen yang ada.

### Disediakan frontend (`frontend/src/lib/stores/groups.ts`)
Tipe TS `Workspace` (`lib/stores/types.ts`) diperluas aditif: `groups?: WorkspaceGroup[]`, `activeGroupId?: string`, `membership?: Record<string, string>`, dengan `WorkspaceGroup {id, name, order, main?: boolean, layout?: unknown | null}`.
```ts
export interface SessionGroup { id: string; name: string; order: number; main: boolean }
export const MAIN_GROUP_NAME = 'Utama';   // hanya nama tampilan bawaan; identitas lewat flag `main`

// state turunan dari workspace aktif di store workspaces + store sessions
export const groups: Readable<SessionGroup[]>;          // urut order naik
export const mainGroupId: Readable<string>;
export const activeGroupId: Readable<string>;
export const activeGroup: Readable<SessionGroup | undefined>;
export const sessionGroupMap: Readable<Record<string, string>>;    // sessionId -> groupId (sesi tanpa entri = grup main)
export const groupSessionCounts: Readable<Record<string, number>>; // groupId -> jumlah sesi di store sessions
export const groupsError: Readable<string | null>;                 // galat terakhir dari daemon; null bila bersih

export function initGroups(): () => void;                           // pasang pendengar event/store; dipanggil index.ts modul
export function createGroup(name?: string): Promise<string>;               // kembalikan id, lalu aktifkan
export function renameGroup(id: string, name: string): Promise<void>;
export function deleteGroup(id: string): Promise<void>;                    // grup main ditolak daemon
export function reorderGroup(id: string, toIndex: number): Promise<void>;
export function setActiveGroup(id: string): Promise<void>;
export function moveSession(sessionId: string, toGroupId: string): Promise<void>;
export function groupOfSession(sessionId: string): string | undefined;
```
Komponen UI (`frontend/src/modules/groups/`): `GroupTabs.svelte` (tanpa props wajib; membaca store), `GroupSessionMenu.svelte` (props `{sessionId}`, daftar grup tujuan), `GroupedSessionList.svelte` (props `{renderSession}` atau slot untuk baris sesi milik sidebar kiri), `index.ts` (memanggil `initGroups()` dan mendaftarkan ke slot).

Aturan perilaku store (daemon = sumber kebenaran):
- Tidak ada state grup yang disimpan di frontend selain turunan dari `workspaces`. Hasil setiap method daemon (`GroupsState`) dan event `workspace.groupsChanged` diterapkan ke entri workspace terkait di store `workspaces` (menimpa `groups` kecuali `layout` yang dipertahankan dari entri lama, `activeGroupId`, `membership`); penerapan idempoten sehingga hasil panggilan dan event untuk perubahan yang sama tidak bentrok. Event hanya memperbarui data, tidak pernah memicu pergantian layout (pergantian layout hanya dari aksi `setActiveGroup` di jendela ini).
- Identitas grup bawaan memakai `main` (bukan nama). Tombol/menu hapus disembunyikan untuk grup `main`; `deleteGroup` pada grup itu tetap mengirim ke daemon dan galat `ErrConflict` ditampilkan lewat `groupsError` (state tidak berubah).
- `setActiveGroup(id)`: (1) bila `id` sudah aktif, selesai; (2) `await flushNow()` sehingga `layout.serialize()` grup lama ditulis lewat `workspace.layout.set` ke grup lama (autosave menulis ke `currentGroupId` yang lama); (3) `workspace.group.setActive`; (4) `workspace.layout.get {workspaceId, groupId: id}`; (5) `switchGroup(id, layoutDoc)` dari `modules/persist/autosave.ts` (perluasan, lihat Wiring) yang menjalankan `layout.hydrate(doc)` lalu `setBaseline(wsId, id)` secara sinkron, sehingga autosave berikutnya menulis ke grup baru dan tidak menimpa grup lama dengan layout grup baru. Tidak menghancurkan instance xterm: pane grup lama disembunyikan/ditahan sesuai kontrak layout tiket 03.
- `createGroup(name?)`: `workspace.group.create`, lalu `setActiveGroup(groupId)`; layout area tengah menjadi kosong.
- `deleteGroup(id)`: bila `id` grup aktif, panggil `setActiveGroup(mainGroupId)` lebih dulu (layout grup yang akan dihapus disimpan dulu lalu dibuang bersama grupnya, artinya semua panenya tertutup tanpa membunuh sesi); lalu `workspace.group.delete` (daemon memindahkan keanggotaan sesinya ke grup `main`).
- `moveSession(sessionId, toGroupId)`: bila sesi punya pane di layout grup asal, tutup pane itu: untuk grup asal yang aktif lewat `layout.closePane` pada leaf berisi `sessionId`; untuk grup asal non-aktif lewat `workspace.layout.get`, `closePane`/`leaves` pada model murni, dan `workspace.layout.set` ke grup asal. Lalu `workspace.group.moveSession`. Bila grup tujuan adalah grup aktif, buka pane sesinya di layout aktif dengan `layout.openPane` (bentuk `NewLeaf` dibaca dari `modules/layout/model`, mengikuti klik sesi sidebar kiri); selain itu sesi tanpa pane sampai dibuka. Sesi tidak dibunuh. Pane file tidak dipindah (milik grup tempat dibuka).
- Sesi baru tanpa keanggotaan menjadi milik grup aktif saat dibuat. Daemon tidak mengamati pembuatan sesi, jadi aturannya dijalankan frontend: pendengar pada store `sessions` di `groups.ts` mencatat id sesi yang sudah ada pada snapshot awal tiap workspace (hasil `loadSessions`, dianggap sudah punya keanggotaan: entri atau default `main`); id baru setelah snapshot yang belum punya entri dan grup aktifnya bukan `main` memanggil `workspace.group.moveSession` ke grup aktif (tanpa menyentuh layout). Bila grup aktif adalah `main`, tidak perlu panggilan (tanpa entri = `main`). Tanpa polling.
- Kegagalan memanggil daemon tidak mengubah state lokal (state hanya berubah dari hasil/event daemon) dan galatnya ditampilkan lewat `groupsError` dan jalur galat persist tiket 06 (`setPersistStatus('error')` hanya untuk kegagalan simpan layout), bukan `alert`.

## Fitur

### group-daemon: Daemon: grup, keanggotaan sesi, dan method IPC
Deskripsi:
1. `internal/workspace/types.go` (perluasan aditif): tambah `Workspace.Membership` dan `Group.Main` sesuai Kontrak; `newDefaultGroup()` mengisi `Main: true`.
2. `internal/workspace/groups.go` (baru): fungsi normalisasi `normalizeGroups(w *Workspace)` (urut dan nomor ulang order, tepat satu `Main`, kosongkan `Membership`, jamin non-nil), pembantu nama (trim, batas 40 rune, nama bawaan "Grup N", pembeda " 2"/" 3" tanpa beda huruf besar/kecil), tipe `GroupsState`/`GroupInfo`, dan method `Store`: `CreateGroup`, `RenameGroup`, `DeleteGroup`, `ReorderGroup`, `SetActiveGroup`, `MoveSession`. Setiap mutasi memegang `s.mu.Lock()`, mengkloning workspace (`cloneWorkspaces`), mengubah salinan, `persistLocked`, baru menukar `s.workspaces`; membuang entri `Membership` ke grup yang sudah tidak ada. Memakai ulang `findGroup`, `newID`, dan sentinel error yang sudah ada.
3. `internal/workspace/store.go` (perluasan): `cloneWorkspace` menyalin `Membership` (selalu non-nil) dan `Store.Create` menginisialisasi `Membership` kosong. Tidak ada perubahan perilaku lain.
4. `internal/workspace/config.go` (perluasan): `loadWorkspaces` memanggil `normalizeGroups` untuk tiap workspace (setelah normalisasi grup tiket 06 yang ada). Tidak menaikkan `workspacesFileVersion` dan tidak memaksa menulis ulang file v2.
5. `internal/workspace/ipc_groups.go` (baru): `registerGroupMethods(srv, s)` mendaftarkan enam method `workspace.group.*` dengan pola `decode`/`mapErr` yang sama seperti `ipc.go`, dan `publishGroups(srv, kind, state)` untuk event `workspace.groupsChanged`. `internal/workspace/ipc.go` (perluasan): satu baris `registerGroupMethods(srv, s)` di akhir `Register`. `cmd/shikigamid/wire_workspace.go` tidak perlu berubah karena `Register` sudah dipanggil.
6. Perilaku sesuai tabel Kontrak: grup `Main` tidak bisa dihapus (`ErrConflict`, state tidak berubah); hapus grup memindahkan keanggotaan ke `Main` dan, bila aktif, mengaktifkan `Main`; order selalu rapat; nama diatur sesuai aturan.
7. Hasil `workspace.list` mengeluarkan `membership` (objek, tidak pernah `null`) dan `main` pada grup bawaan.

Acceptance criteria:
- [ ] `go build ./...` dan `make build` lulus.
- [ ] File v2 buatan tiket 06 (tanpa `membership`/`main`) dimuat tanpa galat, tidak menambah grup baru, grup bernama "Utama" (atau grup ber-order terkecil bila tidak ada) ditandai `main`, dan file tidak ditulis ulang sampai ada mutasi.
- [ ] `workspace.group.create` menambah grup di akhir tanpa mengubah grup aktif; grup baru terlihat di `workspace.list` dan bertahan di `workspaces.json` setelah restart daemon.
- [ ] `rename`, `reorder`, `setActive`, dan `moveSession` mengubah state, mempersist atomik, dan mempublikasikan `workspace.groupsChanged`; order tetap rapat 0..n-1.
- [ ] `workspace.group.delete` pada grup `main` mengembalikan galat `ErrConflict` tanpa mengubah state; pada grup lain, keanggotaan sesinya pindah ke `main`, grup aktif bergeser ke `main` bila perlu, dan tidak ada sesi yang dibunuh.
- [ ] Aturan nama: trim, maksimum 40 karakter, kosong ditolak pada rename, bentrok nama (tanpa beda huruf besar/kecil) mendapat akhiran " 2", " 3".
- [ ] Galat memakai kode `ErrNotFound`/`ErrInvalid`/`ErrConflict` yang sudah ada; method layout/ui dan event `workspace.changed`/`workspace.layoutChanged` yang sudah ada tidak berubah perilaku.
- [ ] Tidak ada dependency baru, timer baru, atau perubahan di luar berkas yang disebut.

ownedPaths: `internal/workspace/groups.go`, `internal/workspace/ipc_groups.go`.

Perluasan berkas tiket 06 (bukan kepemilikan baru, aditif dan minimal): `internal/workspace/types.go`, `internal/workspace/store.go`, `internal/workspace/config.go`, `internal/workspace/ipc.go`.

Paralel: fondasi. groups-store bergantung pada kontrak IPC di atas dan dapat dikembangkan paralel dengan respons tiruan yang mengikuti `GroupsState`, tetapi integrasi akhir menunggu group-daemon selesai.

### groups-store: Store grup di frontend lewat daemon
Deskripsi:
1. Buat `frontend/src/lib/stores/groups.ts` yang menurunkan `groups`, `mainGroupId`, `activeGroupId`, `activeGroup`, `sessionGroupMap`, `groupSessionCounts` dari `activeWorkspace` (store `workspaces`) dan `sessions`, sesuai Kontrak. Tidak ada `localStorage`, file sampingan, slot cermin, atau rekonsiliasi: daemon satu sumber kebenaran, state frontend hanya turunan data workspace.
2. Perluas tipe `Workspace` di `frontend/src/lib/stores/types.ts` (aditif, opsional) dengan `groups`, `activeGroupId`, `membership`.
3. Implementasikan aksi `createGroup`/`renameGroup`/`deleteGroup`/`reorderGroup`/`setActiveGroup`/`moveSession` lewat method `workspace.group.*` (`call` dari `lib/ipc.ts`), menerapkan `GroupsState` hasil panggilan ke store `workspaces`. `initGroups()` memasang `onEvent('workspace.groupsChanged', ...)` (menerapkan payload secara idempoten, mempertahankan `layout` di entri lama) dan pendengar sesi baru; mengembalikan fungsi pembersih.
4. `setActiveGroup` mengikuti urutan di Kontrak (`flushNow`, `workspace.group.setActive`, `workspace.layout.get {groupId}`, `switchGroup`). Jangan mengimpor atau menyentuh detail internal layout selain fungsi publik `layout.ts` dan fungsi murni model yang disebut di Kontrak.
5. Saat workspace aktif berganti (aksi tiket 04/06), hidrasi layout tetap dikerjakan `restoreWorkspace` tiket 06 (layout grup aktif daemon); store grup hanya membaca `activeGroupId` workspace itu dan tidak menghidrasi ulang.
6. Invarian bergantung pada daemon (selalu ada grup `main`, `activeGroupId` selalu valid, order rapat). Di frontend hanya pengaman baca: bila `activeGroupId` tidak ditemukan di `groups`, pakai `mainGroupId`; bila `groups` kosong (data lama belum dinormalisasi, tidak seharusnya terjadi) kembalikan daftar kosong tanpa melempar galat.
7. Sesi baru tanpa keanggotaan ditugaskan ke grup aktif sesuai aturan di Kontrak; penghitungan jumlah sesi memakai store `sessions` (sesi tanpa entri dihitung ke grup `main`).
8. Penanganan galat: `groupsError` diisi pesan galat daemon dan dikosongkan pada aksi berikutnya yang berhasil; tampilan lewat UI (tab/toast yang ada), bukan `alert`.

Acceptance criteria:
- [ ] `make build` lulus dengan `groups.ts` tanpa galat tipe, dan tidak ada rujukan `localStorage`/`sessionStorage`/sidecar di `groups.ts`.
- [ ] Workspace baru dan workspace hasil migrasi v1 punya tepat satu grup `main` ("Utama") dan itu grup aktif.
- [ ] `createGroup` memanggil `workspace.group.create`, mengaktifkan grup baru, dan layout area tengah menjadi kosong.
- [ ] `setActiveGroup` menyimpan layout grup lama ke grup lama (bukan ke grup baru), mengganti layout aktif, dan autosave berikutnya menulis ke grup baru; kembali ke grup sebelumnya memulihkan pane, split, dan fokusnya; terminal tidak di-attach ulang dari nol (scrollback tetap).
- [ ] `deleteGroup` pada grup berisi sesi memindahkan semua sesinya ke "Utama" dan tidak membunuh satu pun sesi (sesi tetap ada di daftar sesi daemon); bila grup aktif, "Utama" menjadi aktif.
- [ ] `deleteGroup` pada grup `main` ditolak daemon tanpa mengubah state dan galat muncul di `groupsError`.
- [ ] `reorderGroup` mengubah urutan dan `order` tetap rapat; urutan bertahan setelah restart daemon dan aplikasi.
- [ ] `moveSession` menutup pane sesi di grup asal (aktif maupun non-aktif), mengubah keanggotaan di daemon, dan `groupSessionCounts` ikut berubah.
- [ ] Sesi baru yang dibuat saat grup non-"Utama" aktif menjadi milik grup itu dan tetap demikian setelah muat ulang aplikasi (selama daemon hidup).
- [ ] Perubahan dari event `workspace.groupsChanged` memperbarui tab tanpa memuat ulang `workspace.list` dan tanpa mengganti layout.
- [ ] Tidak ada `setInterval`/polling baru dan tidak ada perubahan di luar `ownedPaths` selain perluasan yang dicatat.

ownedPaths: `frontend/src/lib/stores/groups.ts`.

Perluasan berkas lain (bukan kepemilikan baru, minimal): `frontend/src/lib/stores/types.ts` (field opsional di `Workspace`) dan `frontend/src/modules/persist/autosave.ts` (lihat Wiring).

Paralel: bergantung pada kontrak IPC group-daemon; group-tabs-ui bergantung pada antarmuka store di Kontrak dan dapat dikembangkan paralel dengan data tiruan, tetapi integrasi akhir menunggu groups-store selesai.

### group-tabs-ui: Tab grup dan pemindahan sesi
Deskripsi:
1. `GroupTabs.svelte` (di `frontend/src/modules/groups/`): deretan tab horizontal di atas area layout. Tiap tab: nama grup dan lencana jumlah sesi (`groupSessionCounts`), tab aktif diberi gaya aksen. Tombol `+` di ujung memanggil `createGroup()` lalu memasuki mode ganti nama inline. Bila tab melebihi lebar, deretan bisa digulir horizontal (tanpa memecah baris); tab aktif digulir ke tampilan.
2. Menu konteks tab (klik kanan dan tombol `...` saat hover): "Ganti nama" (input inline; Enter simpan, Esc/blur batal bila kosong) dan "Hapus" (disembunyikan/nonaktif untuk grup `main`; konfirmasi singkat bila grup punya sesi, dengan teks bahwa sesi dipindah ke "Utama" dan tidak dihentikan). Klik ganda tab juga mengganti nama. Nama final yang tampil adalah yang dikembalikan daemon (mungkin berakhiran " 2" bila bentrok).
3. Urut dengan drag: pointer events sendiri (tanpa library), mengikuti pola drag di modul layout tiket 03 bila ada utilitas yang bisa dipakai tanpa mengubah file tiket itu (impor saja). Penanda posisi sisip saat drag; lepas memanggil `reorderGroup`.
4. Seret sesi ke tab grup lain: sumbernya header pane terminal dan baris sesi sidebar kiri. Karena drag header pane sudah dipakai layout untuk menyusun ulang pane, gunakan kanal data yang tidak bentrok: saat drag pane/sesi melintas di atas tab grup, tab itu menyala sebagai zona drop dan lepas memanggil `moveSession(sessionId, groupId)`. Cara mendeteksi: baca mekanisme drag tiket 03 (pointer events + payload global/store); bila mengekspos payload drag publik (mis. `dragState` berisi `sessionId`), pakai itu. Bila tidak ada kait publik, gunakan HTML5 drag untuk baris sidebar kiri lewat atribut/handler yang ditambahkan di `GroupedSessionList` dan batasi drag-dari-header-pane ke menu "Pindah ke grup" (catat batasan di Wiring). Hover lama (sekitar 600 ms) di atas tab non-aktif saat menyeret boleh mengaktifkannya (opsional; memakai timeout sekali jalan per hover, bukan polling).
5. `GroupSessionMenu.svelte`: item "Pindah ke grup" dengan submenu daftar grup selain grup sesi saat ini, ditandai grup asal; memilih memanggil `moveSession`. Dapat dipasang di menu sesi sidebar kiri dan menu `...` header pane lewat Wiring.
6. `GroupedSessionList.svelte`: menyusun daftar sesi sidebar kiri: sesi grup aktif tampil lebih dulu (bagian terbuka, judul nama grup), lalu tiap grup lain sebagai bagian yang dilipat (judul nama grup dan jumlah sesi; klik membuka/menutup; status lipat hanya state UI sementara). Klik baris sesi di grup lain menjadi aktifkan grup itu lalu fokus pane sesinya (membuka pane bila belum ada). Baris sesi di dalam bagian tetap dirender oleh komponen baris sidebar kiri yang sudah ada (diterima lewat slot/props), jadi pengelompokan per project dan indikator status tidak ditulis ulang.
7. Aksesibilitas dan tampilan: role `tablist`/`tab`, navigasi panah kiri/kanan, warna dari variabel CSS tema yang sudah ada (gelap bawaan), tinggi tab ringkas, mengikuti zoom (ukuran dalam unit relatif).
8. Seluruh state berasal dari `groups.ts`; modul ini tidak menyimpan state grup sendiri di mana pun (kecuali state UI sementara di memori komponen: mode edit nama, hasil drag, bagian terlipat). Galat dari `groupsError` ditampilkan ringkas di area tab (bukan `alert`).
9. `index.ts` modul memanggil `initGroups()` dan mendaftarkan `GroupTabs` ke slot area-atas-layout shell dan `GroupedSessionList` ke slot sidebar kiri lewat fungsi pendaftaran yang disebut di Wiring.

Acceptance criteria:
- [ ] `make build` lulus tanpa galat tipe/svelte-check pada modul groups.
- [ ] Tab grup tampil di atas area layout dengan nama dan jumlah sesi yang benar, dan jumlah ikut berubah saat sesi dibuat, dipindah, atau berakhir.
- [ ] `+` membuat grup, mengaktifkannya, dan membuka edit nama inline; Enter menyimpan, Esc membatalkan.
- [ ] Ganti nama lewat menu konteks dan klik ganda berfungsi; nama kosong ditolak; nama bentrok tampil dengan akhiran dari daemon.
- [ ] Hapus grup lewat menu: pane tertutup, sesi pindah ke "Utama", tidak ada sesi yang mati; menu Hapus tidak tersedia untuk "Utama".
- [ ] Drag tab mengubah urutan dan urutan tetap setelah restart.
- [ ] Menu sesi "Pindah ke grup" memindahkan sesi; pane-nya hilang dari grup asal dan sesi muncul di bagian grup tujuan sidebar kiri.
- [ ] Menyeret sesi (sekurangnya baris sidebar kiri; header pane bila kait drag tiket 03 mengizinkan) ke tab grup lain memindahkannya, dengan penanda zona drop pada tab.
- [ ] Sidebar kiri menampilkan sesi grup aktif dulu, lalu grup lain terlipat; semua sesi workspace tetap terlihat.
- [ ] Beberapa grup memiliki layout masing-masing dan pulih setelah restart aplikasi (grup aktif, urutan, split, pane).
- [ ] Berpindah grup tidak mereset terminal (scrollback tetap) dan tidak memutus attach sesi.
- [ ] Komponen terdaftar hanya lewat slot; tidak ada perubahan di luar `ownedPaths` selain perluasan slot yang dicatat di Wiring.

ownedPaths: `frontend/src/modules/groups/`.

Paralel: dapat dikerjakan paralel dengan groups-store memakai antarmuka di Kontrak (data tiruan). Pengujian ujung ke ujung dan Wiring dilakukan setelah group-daemon dan groups-store selesai.

## Path yang dimiliki
Hanya path ini yang boleh dibuat atau diubah sebagai milik tiket ini, tanpa tumpang tindih antar fitur:
- group-daemon: `internal/workspace/groups.go`, `internal/workspace/ipc_groups.go`
- groups-store: `frontend/src/lib/stores/groups.ts`
- group-tabs-ui: `frontend/src/modules/groups/`

Perluasan eksplisit berkas milik tiket lain (bukan kepemilikan baru; aditif, sekecil mungkin, tanpa mengubah perilaku yang ada):
- Tiket 06 (Go), dikerjakan oleh group-daemon: `internal/workspace/types.go` (field `Membership`, `Group.Main`), `internal/workspace/store.go` (`cloneWorkspace`, `Create`), `internal/workspace/config.go` (satu pemanggilan normalisasi di `loadWorkspaces`), `internal/workspace/ipc.go` (satu pemanggilan `registerGroupMethods`).
- Tiket 06 (frontend), dikerjakan oleh groups-store: `frontend/src/modules/persist/autosave.ts` (ekspor `switchGroup`, `getCurrentGroupId`) dan `frontend/src/lib/stores/types.ts` (field opsional `Workspace`).
- Tiket 04 (frontend), dikerjakan saat Wiring: `modules/shell` dan `modules/sidebar-left` (slot).

## Wiring
Langkah penutup tiket ini, dikerjakan setelah ketiga fitur selesai. Tiket ini bergantung pada tiket 06 (skema grup, daemon workspace, dan persist) dan secara tidak langsung 03/04 (layout, shell, sidebar); tidak ada ketergantungan pada tiket 05.

1. Daemon: pastikan `Register` di `internal/workspace/ipc.go` memanggil `registerGroupMethods(srv, s)` (satu baris, di akhir) dan bahwa `cmd/shikigamid/wire_workspace.go` tidak perlu diubah (modul `workspace` sudah memanggil `workspace.Register`). Perluasan berkas tiket 06 lainnya sudah dicatat di "Path yang dimiliki".
2. Persist (perluasan kecil tiket 06): di `frontend/src/modules/persist/autosave.ts` tambah dua ekspor tanpa mengubah perilaku yang ada:
   - `getCurrentGroupId(): string` mengembalikan `currentGroupId`.
   - `switchGroup(groupId: string, doc: unknown): void` menjalankan `layout.hydrate(doc)` lalu `setBaseline(currentWsId, groupId)` secara sinkron (tanpa membersihkan timer retry yang sedang berjalan, agar tulis ulang layout grup lama tetap sampai ke grup lama karena `groupId` ditangkap per tulis). Pemanggil (`setActiveGroup`) sudah memanggil `flushNow()` sebelumnya.
   `restore.ts` dan `index.ts` modul persist tidak diubah: `restoreWorkspace` sudah memanggil `setBaseline(wsId, res.groupId)` dengan grup aktif dari daemon sehingga `currentGroupId` benar setelah restore dan setelah pindah workspace.
3. Baca bentuk asli registri slot di `frontend/src/modules/shell/` dan `frontend/src/modules/sidebar-left/` (tiket 04) sebelum mengubah apa pun. Prioritaskan titik ekstensi yang sudah ada. Spec menyebut tab grup terdaftar lewat slot shell; bila tiket 04 sudah menyediakan slot untuk area di atas layout, pakai itu. Bila belum, tambahkan perluasan minimal berikut (dengan perubahan sekecil mungkin dan tidak mengubah perilaku yang ada):
   - `modules/shell`: registri slot `registerLayoutHeader({ id, order?, component })` (mengikuti gaya `registerRightTab`/`registerTopbarItem`) dan satu elemen di tata letak shell yang me-render komponen terdaftar tepat di atas area layout. Tanpa registrasi, area itu tidak menampilkan apa pun.
   - `modules/sidebar-left`: slot/hook `registerSessionListWrapper` (atau prop slot setara) agar daftar sesi bisa dibungkus `GroupedSessionList`; dan satu slot item menu sesi untuk `GroupSessionMenu`. Tanpa registrasi, sidebar kiri berperilaku seperti sebelumnya.
   - Menu `...` header pane (milik modul layout tiket 03): bila menyediakan titik ekstensi menu, daftarkan `GroupSessionMenu`; bila tidak, cukup lewat menu sesi sidebar kiri (jangan mengubah `modules/layout`) dan catat di hasil.
4. `frontend/src/modules/groups/index.ts` memanggil `initGroups()` dan pendaftaran slot di atas, dan di-impor satu baris pada titik masuk modul frontend (pola impor yang sama dengan modul lain di `main.ts`/registri modul), sehingga pendaftaran terjadi saat aplikasi mulai.
5. Sesi baru (tombol `+ sesi`, split, atau klik file) menjadi milik grup aktif lewat pendengar `sessions` di `groups.ts` (lihat Kontrak), tanpa mengubah jalur pembuatan sesi tiket 04/03. Bila jalur itu belakangan memberi sinyal eksplisit, pendengar boleh diganti tanpa mengubah kontrak daemon.
6. Flag izin agent (`--dangerously-skip-permissions`, `--dangerously-bypass-approvals-and-sandbox`) tidak disentuh dan tidak digandakan.
7. Verifikasi manual di aplikasi yang berjalan: buat dua grup tambahan, isi tiap grup dengan layout dan sesi berbeda (split, pane file), ganti nama, urutkan, pindahkan sesi lewat menu dan drag, hapus satu grup berisi sesi (sesi pindah ke "Utama" dan tetap hidup), pastikan grup baru terlihat di `workspace.list` dan di `workspaces.json`, lalu tutup dan buka lagi aplikasi, dan juga hentikan lalu jalankan ulang `shikigamid`, dan pastikan daftar grup, urutan, grup aktif, dan layout tiap grup pulih. Uji juga file `workspaces.json` v2 lama tanpa field `membership`/`main`: dimuat tanpa grup baru.

## Guard
- `go build ./...` berhasil (ada perubahan Go di `internal/workspace`).
- `make build` berhasil (termasuk build frontend).
- Aplikasi dapat dijalankan dan tab grup tampil di atas area layout. Tanpa unit test.
