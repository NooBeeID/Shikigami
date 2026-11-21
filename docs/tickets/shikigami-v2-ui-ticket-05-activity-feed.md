---
id: shikigami-v2-ui-ticket-05-activity-feed
title: Feed aktivitas (tab Activity)
spec: docs/superpowers/specs/2026-11-22-shikigami-v2-ui-design.md
status: ready
wave: 5
depends_on: [shikigami-v2-ui-ticket-04-shell-sidebars]
---
# Feed aktivitas (tab Activity)

## Tujuan
Menambah tab "Activity" di panel kanan (di samping "Files") yang menampilkan tool call terbaru dari agent Claude, ditangkap dari hooks yang sudah dipasang mesin status (tiket 08 v1). Tujuannya agar pengguna yang mengawasi banyak agent sekaligus bisa melihat apa yang sedang dikerjakan tiap agent tanpa membuka tiap terminal. Sesuai spec bagian 4b dan 11b, tab ini hanya menampilkan; tombol Approve/Reject tidak dibuat karena agent selalu berjalan tanpa prompt izin. Tanpa polling: data datang lewat event.

## Hasil yang bisa dijalankan
Di atas tiket 04 (shell dan sidebar):
- Panel kanan punya tab baru "Activity" di samping "Files". Tab dipilih pengguna, dan pilihannya ikut tersimpan oleh mekanisme `rightSidebar.activeTab` milik tiket 04/06 bila tersedia (tidak wajib di tiket ini).
- Saat sesi claude menjalankan tool (Bash, Edit, Read, dst.), baris baru muncul di atas daftar secara langsung: waktu, nama sesi, nama tool, ringkasan singkat (mis. perintah atau path).
- Dropdown filter "Semua sesi / <sesi>" mempersempit daftar.
- Klik baris memfokuskan pane sesi tersebut di layout (memakai aksi fokus/openPane yang sudah ada dari tiket 03/04).
- Saat belum ada aktivitas, tampil keadaan kosong "Belum ada aktivitas".
- `make build` lulus dan aplikasi bisa dijalankan. Tanpa tiket 06/07 fitur ini tetap berjalan sendiri (data in-memory, hilang saat daemon restart).

## Batasan
In scope:
- Ring buffer aktivitas di daemon, method IPC `activity.list`, event `activity.event`.
- Pendaftaran event hook PostToolUse pada berkas hooks claude dan pemanggilan callback dari mesin status.
- Modul UI `modules/activity` dengan tab lewat `registerRightTab`.

Out of scope:
- Approve/Reject, prompt izin, dan kontrol apa pun terhadap agent (spec 11b).
- Persistensi ke disk, pencarian teks, ekspor, dan paginasi tak terbatas.
- Aktivitas untuk provider codex dan shell (tidak punya hooks; hanya claude yang bercapability hooks).
- Meter sumber daya, Orchestration, Tasks (spec 4b).
- Polling dalam bentuk apa pun (timer pengambilan ulang). Pembaruan hanya lewat event.
- Mengubah perilaku klasifikasi status (running/waiting/done) yang sudah ada.

## Kontrak
Disediakan epic ini:

IPC (daemon, didaftarkan lewat `srv.Register`):
- `activity.list` params `{sessionId?: string, limit?: number}`. `limit` bawaan 100, maksimum 200; nilai <=0 dianggap bawaan. Hasil: array `ActivityEntry` urut dari terbaru ke terlama. `sessionId` kosong berarti semua sesi (digabung, urut `ts` menurun). `sessionId` dikenal atau tidak, tidak error: sesi tak dikenal menghasilkan array kosong `[]` (bukan `null`).
- Event `activity.event` dipublikasikan lewat `srv.Publish("activity.event", ActivityEntry)`. Frontend menerimanya lewat `onEvent('activity.event', ...)` dari `lib/ipc.ts`.

Tipe Go (`internal/status/activity.go`):
```go
type ActivityEntry struct {
    TS        int64  `json:"ts"`        // unix milidetik
    SessionID string `json:"sessionId"` // id sesi daemon (sudah di-resolve dari token)
    Tool      string `json:"tool"`      // tool_name dari payload hook, mis. "Bash"
    Summary   string `json:"summary"`   // ringkas, maks 160 rune, satu baris
}
type ActivityLog struct { /* ring buffer per sesi, kapasitas 200, mutex */ }
func NewActivityLog(srv *ipc.Server) *ActivityLog
func (a *ActivityLog) Record(sessionID, event string, payload map[string]any)
func (a *ActivityLog) List(sessionID string, limit int) []ActivityEntry
func (a *ActivityLog) Forget(sessionID string)
func RegisterActivityIPC(srv *ipc.Server, a *ActivityLog)
```
Tipe TypeScript (`frontend/src/modules/activity/`):
```ts
export interface ActivityEntry { ts: number; sessionId: string; tool: string; summary: string }
```
Aturan ringkasan (`summarize(tool, tool_input)`): `Bash` -> `command`; `Read`/`Edit`/`Write`/`NotebookEdit` -> `file_path`; `Grep`/`Glob` -> `pattern`; `WebFetch` -> `url`; `Task` -> `description`; lainnya -> nama field string pertama yang ada, atau string kosong. Newline diganti spasi, dipotong 160 rune. Nilai yang mungkin rahasia tidak disaring khusus; hanya field di atas yang dipakai, `tool_response` tidak pernah disimpan.

Dipakai epic ini:
- Daemon: `RegisterModule` + `Deps.Provide/Lookup/OnReady` (`cmd/shikigamid/modules.go`), `ipc.Server.Register/Publish`, `status.Engine` (`ResolveSessionID`, `Forget`/`Subscribe` untuk pembersihan saat sesi berakhir), mekanisme hooks per sesi (`install.go`, `hooks.go`).
- UI: `registerRightTab` dari `modules/shell` (tiket 04), `lib/ipc.ts` (`call`, `onEvent`), store sesi/status yang ada (`modules/status/store.ts`, daftar sesi dari modul workspace) untuk nama sesi, dan aksi fokus pane dari store layout (tiket 03: `openPane`/fokus berdasarkan `sessionId`).

## Fitur

### activity-daemon: Daemon: buffer dan event aktivitas tool call
Deskripsi:
1. `internal/status/activity.go` berisi `ActivityLog`: map `sessionID -> ring buffer` berkapasitas 200 entri, dilindungi mutex. `Record` hanya bereaksi pada event `PostToolUse`; event lain diabaikan. Ambil `tool_name` (string) dan `tool_input` (map) dari payload; bila `tool_name` kosong, abaikan. Isi `TS` dari `time.Now().UnixMilli()`. Setelah menyimpan, panggil `srv.Publish("activity.event", entry)` (tidak memblokir).
2. `List(sessionID, limit)`: bila `sessionID` diisi, ambil dari buffer sesi itu; bila kosong, gabungkan semua buffer lalu urutkan `ts` menurun lalu potong ke `limit`. Selalu kembalikan slice non-nil.
3. `RegisterActivityIPC` mendaftarkan `activity.list` (validasi params JSON; params kosong/`null` diterima sebagai default).
4. `cmd/shikigamid/wire_activity.go`: `RegisterModule("activity", ...)` membuat `ActivityLog`, `d.Provide("activity.log", log)`, memanggil `RegisterActivityIPC`, dan di `d.OnReady` mengambil `status.engine` lewat `Lookup` untuk memasang callback `eng.SetActivitySink(log.Record)` serta berlangganan agar `Forget` dipanggil saat sesi berakhir/dilupakan (hapus buffer sesi yang selesai setelah ia di-forget oleh engine, bukan saat status `done`, agar riwayat sesi yang baru selesai masih terlihat).
5. Perluasan file status yang bukan milik fitur ini hanya sekecil mungkin dan dicatat di Wiring: field dan method `SetActivitySink(func(sessionID, event string, payload map[string]any))` pada Engine, pemanggilannya di `Apply`, dan penambahan `PostToolUse` ke `hookEvents`.
6. Tanpa polling, tanpa goroutine pengambil data; semua dipicu oleh hook masuk.

Acceptance criteria:
- [ ] `go build ./...` lulus dengan `activity.go` dan `wire_activity.go`.
- [ ] `activity.list` tanpa params mengembalikan `[]` (bukan `null`) saat belum ada data.
- [ ] Setelah event `PostToolUse` dengan `tool_name:"Bash"` dan `tool_input.command` masuk lewat `status.hook`, `activity.list` memuat entri dengan `tool:"Bash"` dan `summary` berisi command, `sessionId` sudah berupa id sesi daemon (bukan token launch).
- [ ] `activity.event` terkirim ke klien yang berlangganan prefix `activity.` untuk tiap entri baru.
- [ ] Buffer per sesi tidak melebihi 200 entri; entri tertua terbuang lebih dulu.
- [ ] `limit` dihormati (maks 200) dan filter `sessionId` bekerja.
- [ ] Event hook selain `PostToolUse` tidak menambah entri dan tidak mengubah klasifikasi status yang sudah ada.
- [ ] Payload tanpa `tool_name`, JSON aneh, atau `tool_input` bukan objek tidak menyebabkan panic.
- [ ] `tool_response` tidak pernah disimpan atau dikirim.
- [ ] Tidak ada timer atau polling baru.

ownedPaths: `internal/status/activity.go`, `cmd/shikigamid/wire_activity.go`.

Paralel: dapat dikerjakan paralel dengan activity-ui. UI memakai kontrak di bagian Kontrak dan dapat dikembangkan dengan data tiruan sampai daemon siap. Wiring penutup menyatukan keduanya.

### activity-ui: UI: tab Activity di sidebar kanan
Deskripsi:
1. `frontend/src/modules/activity/` mengekspor modul yang pada inisialisasi memanggil `registerRightTab({ id: 'activity', title: 'Activity', order: <setelah Files>, component })` (nama field mengikuti definisi registri tiket 04; baca `modules/shell` dan ikuti bentuk aslinya).
2. Store (`store.ts`): daftar `ActivityEntry` terbatas 300 entri di memori, urut terbaru di atas. Saat tab pertama kali dipasang: `call('activity.list', { limit: 100 })` sekali untuk mengisi riwayat, lalu `onEvent('activity.event', ...)` menambah entri baru di depan (buang yang melebihi batas). Tidak ada `setInterval`. Bila `call` gagal (di luar Wails), tampilkan keadaan kosong tanpa error mencolok.
3. Komponen daftar: tiap baris menampilkan waktu relatif/jam (HH:MM:SS), nama sesi (dari store sesi; bila tidak ada, 8 karakter pertama id), lencana tool, dan ringkasan (satu baris, terpotong dengan ellipsis, tooltip berisi teks penuh). Tampilan padat, mengikuti variabel CSS tema yang sudah ada (gelap bawaan). Daftar besar dirender ringan (batas 300 baris cukup tanpa virtualisasi).
4. Filter: dropdown "Semua sesi" plus sesi yang punya entri. Filter dilakukan di sisi klien dari data store (tidak memanggil ulang IPC).
5. Klik baris: fokuskan pane sesi lewat aksi store layout tiket 03; bila sesi belum punya pane di layout, buka pane terminal untuk sesi itu (perilaku sama seperti klik sesi di sidebar kiri tiket 04). Bila sesi sudah tidak ada, baris dinonaktifkan (opacity turun, tanpa aksi).
6. Keadaan kosong: teks "Belum ada aktivitas". Tidak ada tombol Approve/Reject.
7. Auto-scroll: daftar menempel ke atas; bila pengguna sudah menggulir ke bawah, entri baru tidak menggeser posisi gulir.

Acceptance criteria:
- [ ] Build frontend lulus (`make build`) tanpa error tipe.
- [ ] Tab "Activity" tampil di panel kanan di samping "Files" hanya lewat `registerRightTab`, tanpa mengubah file `modules/shell` atau `modules/sidebar-right`.
- [ ] Entri baru muncul dalam hitungan detik setelah agent claude menjalankan tool, tanpa memuat ulang.
- [ ] Filter sesi mempersempit daftar dan kembali ke semua dengan memilih "Semua sesi".
- [ ] Klik baris memfokuskan pane sesi yang benar; sesi tanpa pane dibuatkan pane.
- [ ] Tidak ada `setInterval`/polling; hanya satu `activity.list` saat inisialisasi.
- [ ] Memori daftar dibatasi 300 entri.
- [ ] Tidak ada tombol atau teks yang mengisyaratkan Approve/Reject.
- [ ] Keadaan kosong dan kegagalan IPC tidak melempar error ke konsol secara berulang.

ownedPaths: `frontend/src/modules/activity/`.

Paralel: dapat dikerjakan paralel dengan activity-daemon (bergantung hanya pada kontrak IPC di atas). Pengujian ujung ke ujung baru mungkin setelah keduanya selesai dan Wiring dijalankan.

## Path yang dimiliki
Hanya path ini yang boleh dibuat atau diubah oleh tiket ini:
- `internal/status/activity.go`
- `cmd/shikigamid/wire_activity.go`
- `frontend/src/modules/activity/`

Perluasan kecil file tiket lain dicatat eksplisit di Wiring (bukan kepemilikan baru).

## Wiring
Langkah penutup tiket ini, dikerjakan setelah kedua fitur selesai. Tiket ini hanya bergantung pada tiket 04 (registri `registerRightTab`) agar bisa dijalankan.

Daemon:
1. `wire_activity.go` mendaftar modul `activity` lewat `RegisterModule` (modul ter-wire otomatis oleh `wireAll`; urutan alfabetis, jadi akses `status.engine` dilakukan di `d.OnReady`, bukan saat wire).
2. Perluasan file `internal/status` yang sudah ada (tidak dimiliki tiket ini, dibatasi pada hal berikut, dan tanpa mengubah klasifikasi status):
   - `install.go`: tambahkan `PostToolUse` ke `hookEvents` (komentar "tepat 4" diperbarui menjadi 5 dan tidak ada tempat lain yang mengasumsikan jumlah itu; periksa dengan pencarian). Hook ini dipasang hanya untuk claude lewat berkas hooks per sesi yang sudah ada, dengan akhiran `|| true` yang sudah ada.
   - `types.go`: konstanta `HookPostToolUse = "PostToolUse"`.
   - `engine.go`: field callback `activitySink`, method `SetActivitySink`, dan di `Apply` panggil sink (dalam recover, di luar lock) dengan id sesi yang sudah di-resolve SEBELUM `ClassifyEvent` mengembalikan false untuk event tak dikenal. `hooks.go` tidak perlu diubah selain bila perlu pembantu kecil; `ClassifyEvent` tetap mengembalikan false untuk `PostToolUse` sehingga status tidak berubah.
3. Flag izin agent (`--dangerously-skip-permissions`, `--dangerously-bypass-approvals-and-sandbox`) tidak disentuh dan tidak digandakan.

UI:
4. Daftarkan modul activity pada titik masuk modul frontend (mengikuti pola impor modul yang ada di `main.ts`/registri modul; cukup satu baris impor `modules/activity`), sehingga `registerRightTab` terpanggil saat aplikasi mulai.
5. Verifikasi manual: jalankan aplikasi, buat sesi claude, minta agent menjalankan satu perintah, lalu pastikan baris muncul di tab Activity, filter bekerja, dan klik memfokuskan pane. Pastikan status sesi (running/done) tetap berubah seperti sebelumnya.

## Guard
- `go build ./...` berhasil.
- `make build` berhasil (termasuk build frontend).
- Aplikasi dapat dijalankan dan tab Activity tampil. Tanpa unit test.
