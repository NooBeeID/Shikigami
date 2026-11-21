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
Menambah grup sesi sebagai tab di atas area layout. Grup adalah satu konteks kerja milik workspace (mis. per fitur, cabang, atau tugas) yang berisi sekumpulan sesi dan file terbuka dengan layout pane sendiri (spec bagian 5b). Dengan begitu pengguna yang mengawasi banyak agent bisa memisahkan pekerjaan tanpa satu layout yang makin padat. Skema data grup (workspace -> grup -> layout) sudah disediakan tiket 06; epic ini hanya menambah store aksi dan UI di atasnya, tanpa mengubah skema atau daemon.

## Hasil yang bisa dijalankan
Di atas tiket 06 (workspace persist), 04 (shell dan sidebar), dan 03 (layout multi-pane):
- Di atas area layout tampil deretan tab grup. Tiap tab menampilkan nama dan jumlah sesi grup itu. Tiap workspace selalu punya minimal satu grup bernama "Utama".
- Klik tab mengganti grup aktif; area layout langsung menampilkan layout grup itu (pane, split, dan fokus masing-masing). Terminal tidak dibuat ulang saat berpindah grup: sesi yang tidak sedang tampil tetap hidup di daemon dan di-attach kembali saat grupnya aktif lagi.
- Tombol `+` membuat grup baru (nama bawaan "Grup 2", "Grup 3", dst.) berlayout kosong dan langsung mengaktifkannya. Menu konteks tab: Ganti nama (edit inline, Enter simpan, Esc batal) dan Hapus (tidak tersedia untuk "Utama").
- Tab bisa diurutkan dengan drag.
- Sesi dipindah antar grup lewat menu sesi "Pindah ke grup" (di sidebar kiri dan menu `...` header pane) atau dengan menyeret sesi/header pane ke tab grup lain.
- Menghapus grup menutup pane-nya dan memindahkan sesinya ke "Utama"; sesi tidak dibunuh dan tetap terlihat di sidebar kiri.
- Sidebar kiri menampilkan sesi grup aktif lebih dulu, lalu grup lain sebagai bagian yang dilipat.
- Grup aktif, daftar grup, urutan, dan layout tiap grup tersimpan lewat persist tiket 06 dan pulih setelah aplikasi ditutup dan dibuka lagi.
- `make build` lulus dan aplikasi bisa dijalankan.

## Batasan
In scope:
- Store `lib/stores/groups.ts`: daftar grup per workspace, grup aktif, aksi create/rename/delete/reorder/moveSession, pergantian layout aktif saat grup berpindah.
- Modul UI `modules/groups`: `GroupTabs.svelte`, menu konteks, drag urut, drag sesi ke tab, menu "Pindah ke grup", bagian sidebar kiri bergrup.
- Perluasan sangat kecil pada file `modules/shell` dan `modules/sidebar-left` untuk slot, dicatat di Wiring.

Out of scope (spec 5b, bagian "Yang tidak ada di v2"):
- Grup lintas workspace, satu sesi di banyak grup, jalankan/hentikan semua sesi dalam grup, template grup.
- Perubahan skema `workspaces.json`, perubahan method IPC daemon, atau kode Go apa pun.
- Mengubah logika pohon layout (milik tiket 03) atau mekanisme persist (milik tiket 06); hanya memanggil fungsi publiknya.
- Mode tampilan per project, layout bernama, meter sumber daya.
- Grup bawaan selain "Utama" dan arti khusus untuk nama grup.

## Kontrak
Dipakai dari epic lain (tidak diubah di sini):
- Skema workspace v2 (tiket 06): `Workspace {id, name, projects[], groups:[{id, name, order, layout: LayoutDoc|null}], activeGroupId}`. Grup bawaan "Utama" tidak bisa dihapus. Sesi milik tepat satu grup; hapus grup memindah sesi ke "Utama". Method IPC `workspace.layout.get/set {workspaceId, groupId, layout?}` dan `workspace.ui.get/set`.
- Dokumen layout (tiket 03): `{version:1, root:Node|null, focusId?:string, maximizedId?:string|null}`; aksi store layout `openPane`, `closePane` (tidak membunuh sesi), `hydrate(doc)`, `serialize()`. Elemen DOM/instance xterm per pane stabil, dipindah bukan dibuat ulang.
- Persist (tiket 06): `modules/persist` dan `lib/stores/persist.ts` menyimpan otomatis (debounce) perubahan workspace, grup, dan layout. Baca file itu untuk fungsi pemicu simpan (mis. `markDirty`/`schedulePersist`) dan hidrasi; ikuti nama aslinya.
- Shell (tiket 04): slot area atas layout (lihat Wiring), `registerRightTab`/`registerTopbarItem` tidak dipakai di epic ini. Sidebar kiri (tiket 04): daftar sesi dan menu sesi.
- `lib/ipc.ts` (`call`, `onEvent`) dan `lib/stores/sessions.ts` untuk daftar sesi dan nama sesi.

Disediakan epic ini (`frontend/src/lib/stores/groups.ts`):
```ts
export interface SessionGroup { id: string; name: string; order: number; layout: LayoutDoc | null }
export const MAIN_GROUP_NAME = 'Utama';

// state turunan (Svelte store)
export const groups: Readable<SessionGroup[]>;          // grup workspace aktif, urut order naik
export const activeGroupId: Readable<string>;
export const activeGroup: Readable<SessionGroup | undefined>;
export const sessionGroupMap: Readable<Record<string, string>>; // sessionId -> groupId
export const groupSessionCounts: Readable<Record<string, number>>; // groupId -> jumlah sesi

// aksi
export function createGroup(name?: string): Promise<string>;               // kembalikan id, aktifkan
export function renameGroup(id: string, name: string): Promise<void>;      // trim, tidak boleh kosong
export function deleteGroup(id: string): Promise<void>;                    // tolak untuk "Utama"
export function reorderGroup(id: string, toIndex: number): Promise<void>;
export function setActiveGroup(id: string): Promise<void>;
export function moveSession(sessionId: string, toGroupId: string): Promise<void>;
export function groupOfSession(sessionId: string): string | undefined;
```
Komponen UI (`frontend/src/modules/groups/`): `GroupTabs.svelte` (tanpa props wajib; membaca store), `GroupSessionMenu.svelte` (props `{sessionId}`, daftar grup tujuan), `GroupedSessionList.svelte` (props `{renderSession}` atau slot untuk baris sesi milik sidebar kiri), `index.ts` (mendaftarkan ke slot).

Aturan perilaku store:
- Identitas "Utama" ditentukan oleh grup pertama yang dibuat workspace (order 0) dan flag/ID dari skema tiket 06; ikuti cara tiket 06 menandainya (baca `internal/workspace/` dan `frontend/src/lib/stores/persist.ts`). Aksi `deleteGroup` pada "Utama" menjadi no-op yang mengembalikan galat terkendali (UI sudah menyembunyikan menunya).
- `setActiveGroup(id)`: simpan `serialize()` layout grup lama ke grupnya, lalu `hydrate(layoutGrupBaru ?? kosong)` pada store layout aktif, set `activeGroupId`, picu persist. Grup yang pindah tidak menghancurkan instance xterm: pane grup lama disembunyikan/ditahan sesuai kontrak layout tiket 03.
- `createGroup`: id unik (mis. `crypto.randomUUID()`), `order` = maks+1, `layout:null`; nama bawaan unik di workspace.
- `deleteGroup(id)`: tutup semua pane grup itu (`closePane`, tidak membunuh), pindahkan sesi grup ke "Utama" (`moveSession`), hapus grup, bila itu grup aktif aktifkan "Utama", picu persist.
- `moveSession(sessionId, toGroupId)`: bila sesi punya pane di layout grup asal, tutup pane itu dari grup asal; sesi menjadi milik grup tujuan (tanpa pane sampai dibuka; membukanya memakai perilaku klik sesi sidebar kiri). Bila grup tujuan adalah grup aktif, buka pane sesinya di layout aktif. Sesi tidak dibunuh. Pane file tidak dipindah (milik grup tempat dibuka).
- Sesi baru yang dibuat tanpa grup eksplisit menjadi milik grup aktif saat dibuat.

## Fitur

### groups-store: Foundation: store grup dan aksi
Deskripsi:
1. Buat `frontend/src/lib/stores/groups.ts` yang membaca daftar grup dan `activeGroupId` workspace aktif dari store workspace/persist tiket 06 (satu sumber kebenaran; jangan menduplikasi state). Bangun store turunan `groups`, `activeGroup`, `sessionGroupMap`, `groupSessionCounts` sesuai Kontrak. Keanggotaan sesi diambil dari skema tiket 06; bila skema menyimpannya di layout pane (`sessionId` pada leaf) dan/atau daftar sesi per grup, baca persis sesuai implementasi tiket 06 dan tulis perubahan lewat jalurnya.
2. Implementasikan aksi create/rename/delete/reorder/setActive/moveSession sesuai aturan perilaku di Kontrak. Semua perubahan memicu persist yang sudah ada (tiket 06), tanpa timer baru.
3. Satu layout per grup: saat `setActiveGroup`, simpan layout grup lama lewat `serialize()` lalu `hydrate()` layout grup tujuan lewat fungsi publik `lib/stores/layout.ts`. Jangan mengimpor atau menyentuh detail internal layout selain fungsi publik itu.
4. Saat workspace aktif berganti (aksi tiket 04/06), grup aktif mengikuti `activeGroupId` workspace itu dan layout di-hydrate dari grup tersebut; koordinasi dengan hidrasi awal tiket 06 agar tidak hidrasi ganda (bila persist sudah menghidrasi layout grup aktif saat start, store grup hanya membaca, tidak menghidrasi ulang).
5. Jaga invarian: workspace selalu punya minimal grup "Utama"; `activeGroupId` selalu menunjuk grup yang ada (jika tidak, fallback ke "Utama"); `order` rapat 0..n-1 setelah reorder/delete.
6. Penamaan: `renameGroup` men-trim, menolak string kosong, dan menjaga nama tetap unik (akhiran " 2" bila bentrok) atau menolak; pilih salah satu dan konsisten. Panjang maksimum 40 karakter.
7. Penanganan galat: kegagalan persist tidak membuat state UI mundur; tampilkan lewat jalur galat yang dipakai persist tiket 06, bukan `alert`.

Acceptance criteria:
- [ ] `make build` lulus dengan `groups.ts` tanpa galat tipe.
- [ ] Workspace baru dan workspace hasil migrasi v1 punya tepat satu grup "Utama" dan itu grup aktif.
- [ ] `createGroup` menambah grup di akhir, mengaktifkannya, dan layout area tengah menjadi kosong.
- [ ] `setActiveGroup` mengganti layout aktif; kembali ke grup sebelumnya memulihkan pane, split, dan fokusnya; terminal tidak di-attach ulang dari nol (scrollback tetap).
- [ ] `deleteGroup` pada grup berisi sesi memindahkan semua sesinya ke "Utama" dan tidak membunuh satu pun sesi (sesi tetap ada di daftar sesi daemon).
- [ ] `deleteGroup("Utama")` ditolak tanpa mengubah state.
- [ ] `reorderGroup` mengubah urutan dan `order` tetap rapat; urutan bertahan setelah restart.
- [ ] `moveSession` menutup pane sesi di grup asal, mengubah kepemilikan, dan `groupSessionCounts` ikut berubah.
- [ ] Semua perubahan tersimpan lewat persist tiket 06 dan pulih setelah restart (daftar grup, grup aktif, layout tiap grup).
- [ ] Tidak ada `setInterval`/polling baru dan tidak ada perubahan di luar `ownedPaths`.

ownedPaths: `frontend/src/lib/stores/groups.ts`.

Paralel: ini fitur fondasi. group-tabs-ui bergantung pada antarmuka store di Kontrak dan dapat dikembangkan paralel dengan data tiruan yang mengikuti tipe di Kontrak, tetapi integrasi akhir menunggu groups-store selesai.

### group-tabs-ui: Tab grup dan pemindahan sesi
Deskripsi:
1. `GroupTabs.svelte` (di `frontend/src/modules/groups/`): deretan tab horizontal di atas area layout. Tiap tab: nama grup dan lencana jumlah sesi (`groupSessionCounts`), tab aktif diberi gaya aksen. Tombol `+` di ujung memanggil `createGroup()` lalu memasuki mode ganti nama inline. Bila tab melebihi lebar, deretan bisa digulir horizontal (tanpa memecah baris); tab aktif digulir ke tampilan.
2. Menu konteks tab (klik kanan dan tombol `...` saat hover): "Ganti nama" (input inline; Enter simpan, Esc/blur batal bila kosong) dan "Hapus" (disembunyikan/nonaktif untuk "Utama"; konfirmasi singkat bila grup punya sesi, dengan teks bahwa sesi dipindah ke "Utama" dan tidak dihentikan). Klik ganda tab juga mengganti nama.
3. Urut dengan drag: pointer events sendiri (tanpa library), mengikuti pola drag di modul layout tiket 03 bila ada utilitas yang bisa dipakai tanpa mengubah file tiket itu (impor saja). Penanda posisi sisip saat drag; lepas memanggil `reorderGroup`.
4. Seret sesi ke tab grup lain: sumbernya header pane terminal dan baris sesi sidebar kiri. Karena drag header pane sudah dipakai layout untuk menyusun ulang pane, gunakan kanal data yang tidak bentrok: saat drag pane/sesi melintas di atas tab grup, tab itu menyala sebagai zona drop dan lepas memanggil `moveSession(sessionId, groupId)`. Cara mendeteksi: baca mekanisme drag tiket 03 (pointer events + payload global/store); bila mengekspos payload drag publik (mis. `dragState` berisi `sessionId`), pakai itu. Bila tidak ada kait publik, gunakan HTML5 drag untuk baris sidebar kiri lewat atribut/handler yang ditambahkan di `GroupedSessionList` dan batasi drag-dari-header-pane ke menu "Pindah ke grup" (catat batasan di Wiring). Hover lama (sekitar 600 ms) di atas tab non-aktif saat menyeret boleh mengaktifkannya (opsional).
5. `GroupSessionMenu.svelte`: item "Pindah ke grup" dengan submenu daftar grup selain grup sesi saat ini, ditandai grup asal; memilih memanggil `moveSession`. Dapat dipasang di menu sesi sidebar kiri dan menu `...` header pane lewat Wiring.
6. `GroupedSessionList.svelte`: menyusun daftar sesi sidebar kiri: sesi grup aktif tampil lebih dulu (bagian terbuka, judul nama grup), lalu tiap grup lain sebagai bagian yang dilipat (judul nama grup dan jumlah sesi; klik membuka/menutup, status lipat tidak perlu disimpan permanen). Klik baris sesi di grup lain menjadi aktifkan grup itu lalu fokus pane sesinya (membuka pane bila belum ada). Baris sesi di dalam bagian tetap dirender oleh komponen baris sidebar kiri yang sudah ada (diterima lewat slot/props), jadi pengelompokan per project dan indikator status tidak ditulis ulang.
7. Aksesibilitas dan tampilan: role `tablist`/`tab`, navigasi panah kiri/kanan, warna dari variabel CSS tema yang sudah ada (gelap bawaan), tinggi tab ringkas, mengikuti zoom (ukuran dalam unit relatif).
8. Seluruh state berasal dari `groups.ts`; modul ini tidak menyimpan state grup sendiri (kecuali state UI sementara: mode edit nama, hasil drag, bagian terlipat).
9. `index.ts` modul mendaftarkan `GroupTabs` ke slot area-atas-layout shell dan `GroupedSessionList` ke slot sidebar kiri lewat fungsi pendaftaran yang disebut di Wiring.

Acceptance criteria:
- [ ] `make build` lulus tanpa galat tipe/svelte-check pada modul groups.
- [ ] Tab grup tampil di atas area layout dengan nama dan jumlah sesi yang benar, dan jumlah ikut berubah saat sesi dibuat, dipindah, atau berakhir.
- [ ] `+` membuat grup, mengaktifkannya, dan membuka edit nama inline; Enter menyimpan, Esc membatalkan.
- [ ] Ganti nama lewat menu konteks dan klik ganda berfungsi; nama kosong ditolak.
- [ ] Hapus grup lewat menu: pane tertutup, sesi pindah ke "Utama", tidak ada sesi yang mati; menu Hapus tidak tersedia untuk "Utama".
- [ ] Drag tab mengubah urutan dan urutan tetap setelah restart.
- [ ] Menu sesi "Pindah ke grup" memindahkan sesi; pane-nya hilang dari grup asal dan sesi muncul di bagian grup tujuan sidebar kiri.
- [ ] Menyeret sesi (sekurangnya baris sidebar kiri; header pane bila kait drag tiket 03 mengizinkan) ke tab grup lain memindahkannya, dengan penanda zona drop pada tab.
- [ ] Sidebar kiri menampilkan sesi grup aktif dulu, lalu grup lain terlipat; semua sesi workspace tetap terlihat.
- [ ] Beberapa grup memiliki layout masing-masing dan pulih setelah restart aplikasi (grup aktif, urutan, split, pane).
- [ ] Berpindah grup tidak mereset terminal (scrollback tetap) dan tidak memutus attach sesi.
- [ ] Komponen terdaftar hanya lewat slot; tidak ada perubahan di luar `ownedPaths` selain perluasan slot yang dicatat di Wiring.

ownedPaths: `frontend/src/modules/groups/`.

Paralel: dapat dikerjakan paralel dengan groups-store memakai antarmuka di Kontrak (data tiruan). Pengujian ujung ke ujung dan Wiring dilakukan setelah groups-store selesai.

## Path yang dimiliki
Hanya path ini yang boleh dibuat atau diubah oleh tiket ini:
- `frontend/src/modules/groups/`
- `frontend/src/lib/stores/groups.ts`

Perluasan kecil file tiket lain dicatat eksplisit di Wiring (bukan kepemilikan baru).

## Wiring
Langkah penutup tiket ini, dikerjakan setelah kedua fitur selesai. Tiket ini bergantung pada tiket 06 (skema grup dan persist) dan secara tidak langsung 03/04 (layout, shell, sidebar); tidak ada ketergantungan pada tiket 05.

1. Baca bentuk asli registri slot di `frontend/src/modules/shell/` dan `frontend/src/modules/sidebar-left/` (tiket 04) sebelum mengubah apa pun. Prioritaskan titik ekstensi yang sudah ada. Spec menyebut tab grup terdaftar lewat slot shell; bila tiket 04 sudah menyediakan slot untuk area di atas layout, pakai itu. Bila belum, tambahkan perluasan minimal berikut (satu-satunya pengecualian atas kepemilikan, dengan perubahan sekecil mungkin dan tidak mengubah perilaku yang ada):
   - `modules/shell`: registri slot `registerLayoutHeader({ id, order?, component })` (mengikuti gaya `registerRightTab`/`registerTopbarItem`) dan satu elemen di tata letak shell yang me-render komponen terdaftar tepat di atas area layout. Tanpa registrasi, area itu tidak menampilkan apa pun (tidak ada perubahan tampilan).
   - `modules/sidebar-left`: slot/hook `registerSessionListWrapper` (atau prop slot setara) agar daftar sesi bisa dibungkus `GroupedSessionList`; dan satu slot item menu sesi untuk `GroupSessionMenu`. Tanpa registrasi, sidebar kiri berperilaku seperti sebelumnya.
   - Menu `...` header pane (milik modul layout tiket 03): bila menyediakan titik ekstensi menu, daftarkan `GroupSessionMenu`; bila tidak, cukup lewat menu sesi sidebar kiri (jangan mengubah `modules/layout`) dan catat di hasil.
2. `frontend/src/modules/groups/index.ts` memanggil pendaftaran slot di atas dan di-impor satu baris pada titik masuk modul frontend (pola impor yang sama dengan modul lain di `main.ts`/registri modul), sehingga pendaftaran terjadi saat aplikasi mulai.
3. Hubungkan `groups.ts` ke persist tiket 06: pastikan hidrasi awal menghasilkan grup aktif dan layout yang benar, dan perubahan grup memicu simpan otomatis lewat fungsi publik persist. Jangan mengubah `modules/persist`, `lib/stores/persist.ts`, atau `internal/workspace/`. Bila diperlukan fungsi tambahan di sana, catat sebagai temuan alih-alih mengubahnya.
4. Sesi baru (tombol `+ sesi`, split, atau klik file) harus menjadi milik grup aktif; bila jalur pembuatan sesi tiket 04/03 tidak memberi tahu store grup, pasang pendengar pada daftar sesi (`lib/stores/sessions.ts`) di `groups.ts` yang menugaskan sesi tak berkelompok ke grup aktif (tanpa polling, hanya reaksi terhadap perubahan store/event).
5. Flag izin agent (`--dangerously-skip-permissions`, `--dangerously-bypass-approvals-and-sandbox`) tidak disentuh dan tidak digandakan; tiket ini tidak mengubah kode Go.
6. Verifikasi manual di aplikasi yang berjalan: buat dua grup tambahan, isi tiap grup dengan layout dan sesi berbeda (split, pane file), ganti nama, urutkan, pindahkan sesi lewat menu dan drag, hapus satu grup berisi sesi (sesi pindah ke "Utama" dan tetap hidup), lalu tutup dan buka lagi aplikasi dan pastikan semuanya pulih.

## Guard
- `go build ./...` berhasil (tidak ada perubahan Go; tetap dijalankan).
- `make build` berhasil (termasuk build frontend).
- Aplikasi dapat dijalankan dan tab grup tampil di atas area layout. Tanpa unit test.
