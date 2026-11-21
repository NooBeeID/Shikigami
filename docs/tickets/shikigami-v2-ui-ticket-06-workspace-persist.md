---
id: shikigami-v2-ui-ticket-06-workspace-persist
title: Save workspace otomatis dan pemulihan
spec: docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md
status: implemented
wave: 5
depends_on: [shikigami-v2-ui-ticket-04-shell-sidebars]
---
# Save workspace otomatis dan pemulihan

## Tujuan

Epic ini membuat workspace terasa tersimpan: layout pane, sidebar, workspace aktif, dan posisi jendela disimpan otomatis dan dipulihkan saat aplikasi dibuka lagi (spec bagian 1 poin 1, bagian 6 "Save workspace"). Daemon menaikkan skema `workspaces.json` ke v2 (workspace -> grup -> layout) dengan migrasi otomatis dari v1, lalu menyediakan method IPC layout dan UI state. Frontend menyimpan berdebounce dan memulihkan: pane terminal di-attach ke sesi yang masih hidup (termasuk scrollback), pane file membuka ulang file yang sama, dan pane dengan sesi yang sudah mati menampilkan "Sesi berakhir" dengan tombol Mulai ulang. Skema grup dibuat lengkap sekarang agar tiket grup sesi (ticket-07) tidak perlu migrasi lagi.

## Hasil yang bisa dijalankan

Di atas hasil tiket-tiket sebelumnya (shell dua sidebar, layout multi-pane, menu native):
- Pengguna menyusun layout (split, drag, resize, buka file), menutup aplikasi, membukanya lagi, dan layout yang sama muncul: tata letak, rasio, pane fokus, pane yang dimaksimalkan, sidebar kiri/kanan (tampil dan lebar), dan workspace aktif.
- Pane terminal yang sesinya masih hidup di daemon langsung tersambung lagi dengan scrollback utuh. Pane file membuka ulang berkas yang sama.
- Setelah daemon mati atau mesin reboot, pane terminal menampilkan overlay "Sesi berakhir" dengan tombol "Mulai ulang" yang membuat sesi baru (project dan provider sama, tanpa melanjutkan percakapan agent) dan mengganti `sessionId` pane tanpa mengubah posisi pane.
- Posisi dan ukuran jendela diingat antar peluncuran.
- File `workspaces.json` lama (v1) tetap terbaca; setiap workspace otomatis punya grup "Utama".
- `make build` lulus dan aplikasi berjalan.

## Batasan

In scope:
- Skema `workspaces.json` v2, migrasi v1 -> v2, method IPC `workspace.layout.get/set` dan `workspace.ui.get/set`, event perubahan.
- Modul frontend `persist` (autosave berdebounce, pemulihan saat start), store `persist.ts`, komponen overlay sesi mati.
- Penyimpanan dan pemulihan posisi/ukuran jendela di `app/window.go` lewat appconfig.

Out of scope:
- UI grup (tab grup, buat/ganti nama/hapus/urutkan, pindah sesi antar grup): milik ticket-07. Epic ini hanya menyediakan field grup di skema dan menyimpan layout untuk grup aktif. Method IPC `workspace.group.*` TIDAK dibuat di sini.
- Layout bernama ("Simpan layout sebagai...", ditunda oleh spec).
- Melanjutkan percakapan agent (`claude --continue`): "Mulai ulang" hanya sesi baru.
- Menyimpan perubahan file yang belum tersimpan, isi scrollback ke disk, atau sesi terminal yang mati.
- Mengubah modul pemilik lain (layout, shell, terminal) selain lewat fungsi publik yang sudah ada (lihat Wiring).
- Unit test (guard hanya `make build`).

## Kontrak

### Skema workspaces.json v2 (kontrak bersama no. 5)

```jsonc
{
  "version": 2,
  "workspaces": [
    {
      "id": "ULID", "name": "Core",
      "projects": [{ "id": "...", "path": "/abs", "name": "core" }],
      "config": { "grimoireUrl": "...", "grimoireKey": "..." },
      "groups": [
        { "id": "ULID", "name": "Utama", "order": 0, "layout": null /* LayoutDoc | null */ }
      ],
      "activeGroupId": "ULID"
    }
  ],
  "ui": {
    "leftSidebar":  { "visible": true, "width": 260 },
    "rightSidebar": { "visible": true, "width": 280, "activeTab": "files" },
    "activeWorkspaceId": "ULID"
  }
}
```

Tipe Go (paket `workspace`, ditambah di `types.go`/`config.go`):

```go
type Group struct {
    ID     string          `json:"id"`
    Name   string          `json:"name"`
    Order  int             `json:"order"`
    Layout json.RawMessage `json:"layout"` // dokumen opaque; null bila kosong
}
// Workspace ditambah: Groups []Group `json:"groups"` (selalu >= 1, tidak pernah null)
//                     ActiveGroupID string `json:"activeGroupId"`
type SidebarState struct {
    Visible   bool   `json:"visible"`
    Width     int    `json:"width"`
    ActiveTab string `json:"activeTab,omitempty"` // hanya sidebar kanan
}
type UIState struct {
    LeftSidebar       SidebarState `json:"leftSidebar"`
    RightSidebar      SidebarState `json:"rightSidebar"`
    ActiveWorkspaceID string       `json:"activeWorkspaceId"`
}
```

Aturan skema:
- Grup bawaan bernama "Utama", `order` 0, id ULID baru (`newID()`), tidak bisa dihapus (penegakan penghapusan ada di ticket-07; di sini tipe dan validasi `Layout` saja). `Workspace.Create` selalu membuat grup "Utama" dan mengisi `activeGroupId`.
- Migrasi v1 -> v2 dilakukan saat `loadWorkspaces`: tiap workspace v1 mendapat satu grup "Utama" dengan `layout: null` dan `activeGroupId` itu; `ui` diisi default (`leftSidebar {visible:true,width:260}`, `rightSidebar {visible:true,width:280,activeTab:"files"}`, `activeWorkspaceId` = workspace pertama atau kosong). Setelah migrasi, file ditulis ulang atomik sebagai versi 2 dengan salinan cadangan `workspaces.json.v1.bak` (sekali saja, tidak menimpa cadangan yang sudah ada). Versi lebih besar dari 2 ditolak dengan error jelas (jangan dirusak).
- Layout adalah dokumen JSON opaque bagi daemon; daemon hanya memvalidasi bahwa isinya JSON objek valid atau null, ukuran maksimum 1 MiB. Bentuk dokumen (kontrak no. 4, dipakai frontend): `{version:1, root:Node|null, focusId?:string, maximizedId?:string|null}`; Leaf `{type:'pane', id, kind:'terminal'|'file', sessionId?, projectId?, provider?, filePath?, title}`; Split `{type:'split', dir:'h'|'v', ratio, a, b}`.

### Method IPC (didaftarkan di `internal/workspace/ipc.go`, dipasang lewat `wire_workspace.go`)

| Method | Params | Hasil |
|---|---|---|
| `workspace.layout.get` | `{workspaceId, groupId?}` | `{workspaceId, groupId, layout: LayoutDoc\|null}`; `groupId` kosong = grup aktif workspace |
| `workspace.layout.set` | `{workspaceId, groupId?, layout}` | `{ok:true}`; `layout` null menghapus layout; tidak mengirim event `workspace.changed` penuh (hanya `workspace.layoutChanged`) |
| `workspace.ui.get` | `{}` | `UIState` |
| `workspace.ui.set` | `{leftSidebar?, rightSidebar?, activeWorkspaceId?}` (patch parsial, gabung per field) | `UIState` setelah gabung |

Kesalahan: workspace/grup tidak ada -> `ipc.ErrNotFound`; layout bukan JSON objek/null atau terlalu besar -> `ipc.ErrInvalid`; `activeWorkspaceId` yang tidak ada -> `ErrInvalid`. Event baru: `workspace.layoutChanged {workspaceId, groupId}` (untuk kejadian multi-klien; frontend sendiri mengabaikan event dari tulisan miliknya). Method yang sudah ada (`workspace.list` dan lainnya) kini ikut mengembalikan `groups` dan `activeGroupId`, dan `workspace.delete` pada workspace aktif membersihkan `ui.activeWorkspaceId`.

Penulisan ke disk atomik (memakai `writeJSONAtomic` yang ada) dan dilindungi mutex Store; `layout.set` yang sering (autosave) tidak boleh menahan lock saat I/O lebih lama dari yang perlu.

### Frontend

```ts
// frontend/src/lib/stores/persist.ts
export type PersistStatus = 'idle' | 'restoring' | 'ready' | 'error';
export const persistStatus: Readable<PersistStatus>;
export const lastSavedAt: Readable<number | null>;
export function flushNow(): Promise<void>;        // dipanggil saat window ditutup/di-hide
// frontend/src/modules/persist/index.ts
export function startPersist(): Promise<void>;     // dipanggil sekali saat app siap
```

Yang DIPAKAI dari tiket lain (hanya lewat fungsi publik, tanpa mengubah kodenya):
- `lib/stores/layout.ts` (ticket-03): `serialize(): LayoutDoc`, `hydrate(doc)`, dan subscribe ke store untuk mendeteksi perubahan; tipe Node/LayoutDoc.
- `lib/stores/shell.ts` (ticket-04): state sidebar kiri/kanan (visible, width, activeTab) dan workspace aktif beserta setter-nya (mis. `setSidebar`, `setActiveWorkspace`); bila setter belum ada dengan nama itu, pakai setter publik yang setara dan sebut nama sebenarnya di Wiring.
- `lib/ipc.ts`: `call(method, params)` untuk `workspace.layout.*`, `workspace.ui.*`, `session.list`, `session.create`, `session.attach`.
- `lib/registry.ts` dan `registerPaneKind` (ticket-03, `modules/layout/registry.ts`) untuk mendaftarkan overlay sesi mati pada jenis pane `terminal`.

### Konfigurasi aplikasi (window)

`app/window.go` memakai `AppConfigGet()/AppConfigSet(patch)` dari appconfig (ticket-02, field `window:{x,y,w,h}`) tanpa mengubah paket `internal/appconfig`. Wails runtime: `runtime.WindowGetPosition`, `WindowGetSize`, `WindowSetPosition`, `WindowSetSize`.

## Fitur

### schema-v2-daemon: Daemon: skema v2 (grup + layout) dan IPC layout

Deskripsi: naikkan versi skema `workspaces.json` ke 2 sesuai bagian Kontrak. Ubah `internal/workspace/config.go` (`fileVersion` untuk workspaces menjadi 2; `configFile` global tetap versi 1, jangan ikut dinaikkan, pisahkan konstanta bila perlu), `types.go` (tipe `Group`, `SidebarState`, `UIState`, field baru di `Workspace`), `store.go` (`cloneWorkspace` harus menyalin `Groups` dan `Layout` secara dalam; `Create` membuat grup "Utama"; metode baru `GetLayout`, `SetLayout`, `GetUI`, `SetUI`), dan `ipc.go` (empat method baru plus event `workspace.layoutChanged`). `workspacesDoc` ditambah `UI UIState`. Migrasi dilakukan di `loadWorkspaces` seperti dijelaskan di Kontrak, dan harus idempoten (membaca ulang file v2 tidak mengubah apa pun). `wire_workspace.go` tetap memanggil `workspace.Register`; tambahkan di sana hanya wiring yang diperlukan (mis. bila `Register` berubah tanda tangan).

Acceptance criteria:
- [ ] `workspaces.json` v1 yang ada dibaca tanpa error; setiap workspace punya satu grup "Utama" (`order` 0, `layout` null) dan `activeGroupId` terisi; file ditulis ulang sebagai `version: 2` dan `workspaces.json.v1.bak` dibuat sekali.
- [ ] Menjalankan ulang daemon pada file v2 tidak mengubah file (idempoten) dan tidak membuat cadangan baru.
- [ ] File dengan `version` > 2 ditolak dengan pesan jelas dan file tidak diubah.
- [ ] Workspace baru dari `workspace.create` punya grup "Utama" dan `activeGroupId`.
- [ ] `workspace.layout.set` lalu `workspace.layout.get` (juga setelah daemon di-restart) mengembalikan dokumen yang identik; `layout: null` menghapusnya; `groupId` kosong memakai grup aktif.
- [ ] Layout bukan objek JSON atau lebih dari 1 MiB menghasilkan `invalid`; workspace/grup tak dikenal menghasilkan `not_found`.
- [ ] `workspace.ui.set` dengan patch parsial hanya mengubah field yang dikirim dan bertahan setelah restart daemon; `activeWorkspaceId` tak dikenal ditolak.
- [ ] Menghapus workspace yang aktif membersihkan `ui.activeWorkspaceId`.
- [ ] Tidak ada data race pada Store (akses lewat mutex; hasil get adalah salinan).
- [ ] `go build ./...` lulus.

ownedPaths: `internal/workspace/`, `cmd/shikigamid/wire_workspace.go`.

Paralel: bisa dikerjakan paralel dengan fitur lain; fitur frontend mengkode terhadap kontrak IPC di atas (boleh memakai mock sampai daemon siap). Fitur ini tidak bergantung pada fitur lain.

### autosave-restore: Frontend: autosave dan pemulihan layout

Deskripsi: modul `modules/persist` dan store `persist.ts`.

Autosave: berlangganan ke store layout (lewat subscribe, bukan polling) dan store shell. Setiap perubahan menjadwalkan simpan dengan debounce 500 ms (trailing); perubahan beruntun selama drag/resize hanya menghasilkan satu tulisan. Layout disimpan lewat `workspace.layout.set {workspaceId, groupId: <grup aktif>, layout: serialize()}`; state sidebar dan workspace aktif lewat `workspace.ui.set` (patch). Lewati tulisan bila dokumen serial identik dengan yang terakhir tersimpan (bandingkan string JSON). Tulisan tidak boleh berjalan saat status `restoring` (agar layout kosong awal tidak menimpa data tersimpan). `flushNow()` memaksa simpan segera dan dipanggil pada `visibilitychange` ke hidden, `beforeunload`, dan event tutup jendela. Kegagalan simpan: coba lagi sekali setelah 2 detik, lalu set `persistStatus = 'error'` tanpa mengganggu pemakaian (cukup indikator kecil di konsol dan nilai store; tanpa dialog).

Pemulihan (urutan di `startPersist()`): (1) set status `restoring`; (2) `workspace.ui.get` lalu terapkan sidebar dan workspace aktif (bila workspace aktif tersimpan sudah tidak ada, pakai workspace pertama); (3) `workspace.layout.get` untuk workspace aktif dan grupnya; (4) `session.list` untuk mengetahui sesi yang hidup; (5) bersihkan dokumen: daun terminal tanpa `sessionId` tetap dipertahankan sebagai pane "sesi berakhir"; daun file yang berkasnya tidak ada tetap dipertahankan dan menampilkan status berkas hilang dari viewer file; (6) panggil `hydrate(doc)`; pane terminal yang sesinya ada di daftar sesi hidup di-attach ulang oleh modul terminal yang sudah ada (termasuk replay scrollback dari `session.attach`); (7) set status `ready` dan mulai autosave. Bila `layout` null atau tidak valid (versi tidak dikenal, JSON rusak), mulai dengan layout kosong tanpa menghapus data tersimpan sampai pengguna mengubah sesuatu. Pergantian workspace aktif saat berjalan: simpan layout workspace lama (flush) lalu muat layout workspace baru dengan alur yang sama; layout tiap workspace independen.

Acceptance criteria:
- [ ] Menyeret pembatas pane terus-menerus menghasilkan satu panggilan `workspace.layout.set` setelah berhenti 500 ms, bukan satu per gerakan.
- [ ] Restart aplikasi memulihkan pohon layout, rasio, fokus, pane yang dimaksimalkan, lebar dan visibilitas kedua sidebar, tab aktif sidebar kanan, dan workspace aktif.
- [ ] Pane terminal dengan sesi hidup tersambung kembali dengan scrollback; xterm tidak dibuat dua kali per pane.
- [ ] Pane file membuka ulang berkas yang sama; perubahan belum tersimpan tidak dipulihkan.
- [ ] Layout kosong awal tidak menimpa layout tersimpan sebelum pemulihan selesai.
- [ ] Layout tiap workspace independen; berpindah workspace memuat layout masing-masing dan tidak kehilangan perubahan terakhir workspace sebelumnya (flush saat pindah).
- [ ] Dokumen layout rusak atau berversi tak dikenal tidak membuat aplikasi crash; mulai dengan layout kosong.
- [ ] Tidak ada polling (hanya subscribe dan timer debounce) dan tidak menambah dependency.
- [ ] Build frontend dan `go build ./...` lulus.

ownedPaths: `frontend/src/modules/persist/` (kecuali `DeadSession.svelte`), `frontend/src/lib/stores/persist.ts`.

Paralel: bisa dikerjakan paralel dengan `schema-v2-daemon` (kontrak IPC tetap), `restart-dead-session`, dan `window-state`. Integrasi akhir uji end-to-end menunggu `schema-v2-daemon`.

### restart-dead-session: Sesi berakhir dan tombol Mulai ulang

Deskripsi: komponen `frontend/src/modules/persist/DeadSession.svelte`, overlay yang menutupi isi pane terminal saat sesi pane tidak ada lagi di daemon (mis. setelah reboot) atau berstatus berakhir. Overlay menampilkan teks "Sesi berakhir", info singkat (nama project dan provider dari daun: `projectId` dipetakan ke nama lewat store workspace, `provider`), dan tombol "Mulai ulang". Aksi tombol: `session.create` dengan `workspaceId` aktif, `projectId` dan `provider` dari daun (flag izin provider tetap diterapkan oleh adapter di daemon; frontend tidak menambah flag apa pun), lalu mengganti `sessionId` daun lewat fungsi publik store layout (pembaruan atribut daun, bukan menutup dan membuat ulang pane) sehingga pane tetap di posisi, ukuran, dan judulnya, dan terminal baru ter-attach. Tidak melanjutkan percakapan agent. Selama proses tombol nonaktif dengan indikator "Memulai..."; bila gagal tampilkan pesan error singkat dan izinkan coba lagi. Bila `projectId` sudah dihapus dari workspace, tombol dinonaktifkan dengan keterangan "Project tidak ditemukan". Overlay hanya untuk `kind: 'terminal'`; provider `shell` juga didukung.

Deteksi "mati": daun terminal yang `sessionId`-nya tidak ada pada hasil `session.list` saat pemulihan, atau yang menerima event sesi berakhir/di-kill dari daemon saat berjalan (pakai event sesi yang sudah ada dari modul session/status; jangan polling). Pane yang sesinya dibunuh lewat aksi "Akhiri sesi" oleh pengguna tetap ditangani oleh perilaku tiket lain; overlay ini cukup memakai kondisi yang sama bila pane masih tampil.

Acceptance criteria:
- [ ] Daun terminal dengan sesi yang tidak ada di daemon menampilkan "Sesi berakhir" dan tombol "Mulai ulang" di atas pane.
- [ ] Klik "Mulai ulang" membuat sesi baru di project dan provider yang sama, `sessionId` daun berganti, overlay hilang, terminal baru aktif, posisi/judul pane tidak berubah.
- [ ] Percakapan agent lama tidak dilanjutkan (sesi benar-benar baru, tanpa argumen `--continue`/`--resume`).
- [ ] Kegagalan membuat sesi menampilkan pesan error dan tombol bisa dipakai lagi; project terhapus menonaktifkan tombol.
- [ ] Komponen tidak memodifikasi file modul lain; didaftarkan hanya lewat registri (lihat Wiring).
- [ ] Build frontend lulus.

ownedPaths: `frontend/src/modules/persist/DeadSession.svelte`.

Paralel: bisa dikerjakan paralel dengan fitur lain; antarmuka ke `autosave-restore` hanya lewat prop komponen (`leaf`, `onRestarted(newSessionId)`), jadi tidak saling menunggu. Pendaftaran ke jenis pane dilakukan saat Wiring.

### window-state: Ingat posisi dan ukuran jendela

Deskripsi: `app/window.go` menyimpan posisi dan ukuran jendela ke appconfig (field `window:{x,y,w,h}`) saat berubah dan saat tutup, serta memulihkannya saat start. Saat start (hook `OnDomReady`/`OnStartup` yang sudah ada, dipanggil dari `app/main.go` lewat fungsi yang diekspor `window.go`; bila `main.go` perlu satu baris pemanggilan, itu disebut di Wiring): baca appconfig, validasi bahwa jendela masih berada sepenuhnya atau sebagian di salah satu layar yang terpasang (cegah jendela di luar layar setelah monitor dicabut; bila tidak valid pakai ukuran/posisi bawaan dan tengahkan), terapkan lewat `WindowSetSize` dan `WindowSetPosition`. Simpan: tangkap perubahan dengan debounce 500 ms (event resize/move dari sisi frontend dikirim lewat bridge atau pemeriksaan di hook tutup `OnBeforeClose`), dan simpan sekali lagi di `OnBeforeClose`. Gunakan ukuran minimum agar jendela tidak tersimpan terlalu kecil (mis. 800x500). Jendela yang sedang fullscreen/maximized tidak menimpa ukuran normal tersimpan.

Acceptance criteria:
- [ ] Mengubah ukuran dan memindah jendela lalu menutup dan membuka aplikasi memulihkan posisi dan ukuran yang sama.
- [ ] Posisi tersimpan di luar semua layar tidak membuat jendela hilang; jendela jatuh ke bawaan di tengah layar.
- [ ] Appconfig yang tidak punya field `window` (instalasi baru) memakai ukuran awal bawaan tanpa error.
- [ ] Penulisan ke appconfig tidak menimpa field lain (mis. `zoom`): gunakan patch yang menggabung.
- [ ] Tidak ada polling timer; hanya event dan debounce.
- [ ] `go build ./...` lulus.

ownedPaths: `app/window.go`.

Paralel: bisa dikerjakan paralel dengan fitur lain. Ketergantungan: bergantung pada appconfig dan `app/main.go` dari ticket-02 yang sudah selesai (tidak diubah di sini selain yang disebut di Wiring).

## Path yang dimiliki

Hanya path berikut yang boleh disentuh:
- `internal/workspace/`
- `cmd/shikigamid/wire_workspace.go`
- `frontend/src/modules/persist/` (termasuk `DeadSession.svelte`)
- `frontend/src/lib/stores/persist.ts`
- `app/window.go`

Perluasan file milik tiket lain (hanya bila benar-benar perlu, minimal, dan disebut di Wiring): satu baris pemanggilan `startPersist()` pada titik masuk frontend dan satu baris pemanggilan fungsi window pada `app/main.go`. Jika titik ekstensi sudah menyediakan registri yang cukup, tidak ada file lain yang disentuh.

## Wiring

Langkah penutup tiket ini, memastikan hasilnya terlihat dan jalan tanpa bergantung pada tiket lain yang belum selesai:

1. Daemon: pastikan `wire_workspace.go` memanggil `workspace.Register` versi baru sehingga `workspace.layout.get/set` dan `workspace.ui.get/set` aktif; jalankan daemon terhadap `workspaces.json` v1 contoh dan cek migrasi (`shikigamictl` atau klien IPC yang ada boleh dipakai untuk uji manual, tanpa menambah file di luar path milik).
2. Frontend: `modules/persist/index.ts` mendaftarkan dirinya lewat registri modul UI (`lib/registry.ts`, pola yang sama dengan modul lain: modul di-import oleh titik masuk modul dan memanggil `startPersist()` saat aplikasi siap, setelah store shell dan layout terinisialisasi). Bila titik masuk belum meng-import modul secara otomatis, tambahkan satu baris import/panggilan di sana dan catat.
3. Daftarkan `DeadSession.svelte` sebagai overlay jenis pane `terminal` lewat `registerPaneKind` (ticket-03, `modules/layout/registry.ts`) atau mekanisme overlay yang disediakannya; jangan mengubah isi modul layout/terminal.
4. Window: panggil fungsi inisialisasi di `app/window.go` dari `app/main.go` (satu baris, disebut sebagai perluasan file ticket-02) untuk restore saat start dan save saat tutup.
5. Uji manual end-to-end (build dan jalan): `make build`; jalankan aplikasi; buat 2 workspace, split beberapa terminal, buka satu file, ubah lebar sidebar, geser jendela; tutup aplikasi; buka lagi dan cek semuanya pulih (sesi hidup ter-attach dengan scrollback). Lalu hentikan `shikigamid` secara paksa dan buka ulang aplikasi tanpa sesi: pane menampilkan "Sesi berakhir", klik "Mulai ulang" membuat sesi baru di posisi pane yang sama.
6. Pastikan flag izin agent tidak digandakan dan tidak diubah (kontrak no. 2): "Mulai ulang" memakai `session.create` biasa sehingga adapter provider yang menambah flag.

## Guard

Cukup `make build` berhasil (`go build ./...` untuk daemon dan app, plus build frontend). Tanpa unit test. Pastikan aplikasi tetap bisa dijalankan setelah build.
