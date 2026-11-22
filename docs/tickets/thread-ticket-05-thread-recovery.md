---
id: thread-ticket-05-thread-recovery
title: Pemulihan setelah restart daemon dan pembersihan yatim
spec: docs/superpowers/specs/2026-11-22-thread-design.md
status: implemented
wave: 5
depends_on: [thread-ticket-04-thread-close]
---
# Pemulihan setelah restart daemon dan pembersihan yatim

## Tujuan
Thread disimpan di disk dan tetap terbaca setelah daemon restart, tetapi sesi agent (master dan peserta) ikut mati bersama daemon. Tiket ini menutup celah itu (keputusan 3 dan 10 spec): thread aktif tetap bisa dipakai karena agent dihidupkan ulang secara lazy pada pesan user berikutnya, tanpa polling dan tanpa menghidupkan semua agent saat start. Selain itu, worktree dan branch yatim milik thread berstatus `closed` (mis. daemon crash di tengah Close) dibersihkan saat daemon start, sehingga anti-goal "thread tertutup meninggalkan worktree/branch yatim" terpenuhi walau terjadi crash.

## Hasil yang bisa dijalankan
Di atas tiket 1-4, setelah tiket ini selesai:
- Setelah daemon di-restart, thread aktif tetap muncul di daftar Thread dan tab thread tetap menampilkan seluruh log. Peserta (termasuk master) ditampilkan dengan status "agent mati, akan dihidupkan saat pesan berikutnya", dan aliran memuat pesan sistem dengan makna yang sama.
- Saat user mengirim pesan ke thread aktif tersebut dari kolom input, daemon menghidupkan ulang master (dan peserta yang diperlukan) di worktree yang sama, memasang ulang MCP `shikigami-thread`, lalu pesan user diteruskan ke master setelah master siap/idle. Provider yang mendukung resume sesi dipakai dengan resume; jika tidak, agent baru dispawn dengan log thread sebagai konteks awal. Tampil pesan sistem "agent dihidupkan ulang".
- Peserta yang tidak disebut/diperlukan tidak dihidupkan sampai ada pesan yang ditujukan kepadanya (mis. `thread_send` dari master).
- Saat daemon start, worktree dan branch sisa milik thread `closed` dihapus dan dicatat di log daemon; thread aktif tidak disentuh sama sekali.
- `go build ./...` dan build frontend lulus; aplikasi berjalan.

## Batasan
In scope:
- Penandaan peserta thread aktif sebagai mati saat daemon start (state tersimpan, bukan penanda sementara di memori saja).
- Respawn lazy master dan peserta di worktree yang sama, resume sesi provider bila didukung, fallback spawn baru dengan konteks log thread, pemasangan ulang MCP thread.
- Hook respawn pada jalur pengiriman pesan (router/manager) lewat titik ekstensi yang sudah ada.
- Pembersihan worktree dan branch yatim thread `closed` saat daemon start (OnReady), pencatatan ke log daemon.
- UI kecil di `frontend/src/modules/thread/recovery/`: lencana/keterangan status "agent mati, akan dihidupkan saat pesan berikutnya" pada peserta dan pesan sistem di aliran.

Out of scope:
- Polling atau timer apa pun; hidupkan-semua-saat-start.
- Merge/push otomatis, pemulihan thread lintas workspace, router berbasis aturan.
- Mengubah bentuk tipe di `types.go` (hanya boleh menambah field opsional bila mutlak perlu), mengubah alur Close milik tiket 4, mengubah alur spawn/router milik tiket 2 selain memakai hook.
- Menyentuh sesi interaktif milik user; menyentuh thread aktif pada pembersihan yatim.
- Dependency baru, unit test.
- Mengedit berkas milik tiket lain, kecuali penambahan aditif seperlunya pada titik ekstensi (sebutkan di Wiring).

## Kontrak
Dipakai (dari tiket sebelumnya; nama persis ikuti kode yang ada, periksa saat implementasi):
- `types.go` (tiket 1): `Thread`, `Participant`, `ThreadMessage`, konstanta event/method. `Thread.state` (`active|closed`).
- `store.go`/`manager.go` (tiket 1): baca daftar thread dan peserta dari `DataDir/threads/<threadId>/thread.json`, log jsonl append-only, hook Manager (`OnMessage`, `OnParticipantAdded`, `OnClose`, `OnStart`), publish `thread.updated` dan `thread.message`. Direktori master `DataDir/threads/<threadId>/master`.
- `prompt.go`, `master.go` (tiket 1): pembentuk prompt awal master dan cara men-spawn master; dipakai ulang untuk respawn.
- `spawn.go`, `launch.go`, `mcp.go`, `worktree.go`, `router.go` (tiket 2): spawn peserta di worktree (`DataDir/threads/<threadId>/<projectSlug>`), `provider.RegisterLaunchContribution` untuk MCP thread dengan token per sesi, antrean pesan via `internal/mention` (`status.OnIdle`), penanda sesi milik thread (metadata `threadId`), auto-approve di `provider/launch.go`.
- `close.go` (tiket 4): fungsi cleanup worktree/branch (dipakai ulang untuk pembersihan yatim bila bentuknya cocok; bila tidak, tulis fungsi hapus sendiri di `orphans.go` dengan perintah git yang sama: `git worktree remove --force`, `git worktree prune`, `git branch -D`).
- `internal/session` Manager (Create/Kill/List), `internal/provider` (kemampuan resume per provider), `internal/runs`/`internal/status`.

Disediakan:
- Go (paket `internal/thread`):
  - `recovery.go`: `type Recovery struct{...}` dan konstruktor `NewRecovery(m *Manager, ...) *Recovery`.
  - `func (r *Recovery) MarkDeadOnStart() error` memuat semua thread `active`, mengosongkan/menandai `sessionId` peserta sebagai mati dan menyimpan status itu (mis. field opsional `Participant.alive bool`/`dead`, atau penanda di store terpisah per thread; field baru wajib opsional dan tidak mengubah JSON lama). Tidak memanggil session Manager.
  - `func (r *Recovery) EnsureAlive(threadId string, participantIds ...string) error` menghidupkan master dan/atau peserta yang berstatus mati (idempoten, aman dipanggil konkuren per thread lewat mutex per thread). Master selalu dihidupkan pada pesan user. Peserta lain hanya dihidupkan bila terdaftar di `participantIds`.
  - Hook: Recovery dipasang sebagai pra-pengirim di jalur kirim pesan thread (user ke master dari `thread.send`, dan `thread_send` ke peserta mati) lewat hook/titik ekstensi Manager atau router; pesan baru dikirim setelah `EnsureAlive` selesai dan target idle (lewat antrean mention biasa).
  - Bila peserta di-respawn, `sessionId` berubah: simpan `Participant` terbaru lewat store, publish `thread.updated`, tambahkan `ThreadMessage{kind: system}` "agent dihidupkan ulang: <name>", dan catat penggantian ID agar `to`/`from` pada pesan lama tetap terbaca (log lama tidak ditulis ulang).
  - `orphans.go`: `func CleanOrphans(m *Manager) (cleaned int, err error)` dan pendaftaran lewat `OnStart`/`d.OnReady`. Hanya menyentuh thread `closed`.
- IPC: tidak ada method baru. Respons `thread.get` dan event `thread.updated` membawa status mati/hidup peserta lewat field opsional yang ditambahkan ke `Participant` (aditif). Pesan sistem memakai `thread.message` yang sudah ada dengan `kind: system`.
- Frontend: komponen di `frontend/src/modules/thread/recovery/` (mis. `DeadBadge.svelte`, `RecoveryNotice.svelte`) diekspor lewat `index.ts` milik subfolder dan dipasang ke slot peserta dan slot aliran yang disediakan komponen tab thread (tiket 1) dan panel peserta (tiket 2).

## Fitur

### thread-lazy-respawn: Hidupkan ulang agent secara lazy
Deskripsi detail:
- Saat daemon start (via `OnStart`), `MarkDeadOnStart` menandai semua peserta thread `active` sebagai mati dan menyimpannya. Sesi lama sudah tidak ada setelah restart; tidak ada pemanggilan Kill. Thread `closed` dilewati. Tidak ada agent yang di-spawn saat start.
- Pada pesan user berikutnya ke thread aktif (jalur `thread.send`), sebelum meneruskan pesan ke master, panggil `EnsureAlive`:
  1. Cek status mati/hidup tiap target; bila semua hidup, tidak melakukan apa pun.
  2. Master: spawn ulang dengan cwd `DataDir/threads/<threadId>/master`. Bila provider mendukung resume sesi (periksa kemampuan di `internal/provider`; simpan id sesi provider saat spawn awal bila belum disimpan, sebagai field opsional), gunakan resume. Jika tidak, spawn baru dengan prompt awal dari `prompt.go` ditambah ringkasan konteks: goal, daftar peserta (nama, project), dan log thread (dipotong ke N pesan terakhir; N ditentukan builder dan dipotong agar muat, pesan `activity` boleh diringkas/diabaikan).
  3. Peserta yang diperlukan: spawn ulang dengan cwd di worktree yang sama (`DataDir/threads/<threadId>/<projectSlug>`). Bila worktree hilang/rusak, jangan membuat worktree baru diam-diam: tambahkan pesan sistem kesalahan yang jelas dan biarkan master memutuskan (tidak ada fallback bekerja di repo utama).
  4. Pasang ulang MCP `shikigami-thread` dengan token baru lewat jalur `LaunchContribution` yang sama, tandai sesi milik thread (metadata `threadId`), auto-approve seperti spawn awal.
  5. Perbarui `Participant.sessionId`, simpan, publish `thread.updated`, tulis pesan sistem "agent dihidupkan ulang".
- Pesan `thread_send` ke peserta mati (dari master atau peserta lain) juga memicu `EnsureAlive` untuk target itu sebelum masuk antrean. Pesan ditahan di antrean sampai target idle, seperti biasa.
- Konkurensi: dua pesan beruntun saat respawn berlangsung tidak boleh men-spawn dua kali (mutex per thread, cek ulang status setelah mengunci).
- Kegagalan spawn: catat ke log daemon, tulis pesan sistem ke thread, pesan user tetap tersimpan di log; status tetap mati agar percobaan berikutnya mengulang. Tidak ada retry otomatis/polling.
- Anti-loop (`antiloop.go`) tidak direset oleh respawn kecuali pesan user, sama seperti perilaku biasa.

Acceptance criteria:
- [ ] Setelah restart daemon, `thread.list` dan `thread.get` mengembalikan thread aktif dengan log utuh dan peserta berstatus mati; tidak ada sesi agent baru yang muncul sebelum ada pesan user.
- [ ] Mengirim pesan user ke thread aktif pascarestart menghidupkan master, pesan sampai ke master setelah idle, dan UI menampilkan pesan sistem "agent dihidupkan ulang".
- [ ] Peserta peer dihidupkan hanya saat ada pesan yang ditujukan kepadanya; worktree dan branch yang sama dipakai (tidak ada worktree baru).
- [ ] Resume dipakai bila provider mendukung; bila tidak, agent baru menerima log thread sebagai konteks awal.
- [ ] MCP `shikigami-thread` terpasang pada sesi hasil respawn dan tool bekerja (token baru, dibatasi ke thread-nya).
- [ ] Pesan beruntun saat respawn tidak menghasilkan agent ganda untuk peserta yang sama.
- [ ] Kegagalan respawn dan worktree hilang menghasilkan pesan sistem jelas, tanpa crash dan tanpa fallback ke repo utama.
- [ ] Thread `closed` tidak pernah dihidupkan ulang; mengirim pesan ke thread `closed` tetap ditolak seperti sebelumnya.
- [ ] Tidak ada polling/timer baru; sesi interaktif milik user tidak tersentuh.

ownedPaths: `internal/thread/recovery.go`

Fitur ini bisa dikerjakan paralel dengan `thread-orphan-cleanup`. Satu-satunya titik temu adalah `OnStart`/OnReady di Wiring, dan nama fungsi sudah ditetapkan di Kontrak.

### thread-orphan-cleanup: Bersihkan worktree yatim saat daemon start
Deskripsi detail:
- `orphans.go`: `CleanOrphans(m)` dipanggil sekali saat daemon start (`d.OnReady`, atau hook `OnStart`). Untuk setiap thread `closed`:
  - Cari worktree sisa: direktori di `DataDir/threads/<threadId>/<projectSlug>` yang masih ada dan/atau terdaftar di `git worktree list` repo project. Hapus dengan `git worktree remove --force` lalu `git worktree prune`; bila gagal karena repo project hilang, hapus direktori lewat penghapusan berkas biasa dan lanjut.
  - Cari branch sisa `thread/<threadId>-*` di repo project terkait (`git branch --list`), hapus dengan `git branch -D`.
  - Direktori master `DataDir/threads/<threadId>/master` boleh dihapus; `thread.json` dan log jsonl TIDAK dihapus (thread closed tetap terbaca).
  - Catat tiap tindakan dan tiap kegagalan ke log daemon (jumlah worktree/branch dibersihkan per thread). Satu kegagalan tidak menghentikan thread/project lain.
  - Tidak menyentuh thread `active` sama sekali, dan tidak menyentuh branch/worktree di luar pola `thread/<threadId>-*` dan di luar `DataDir/threads/<threadId>/`.
  - Idempoten: menjalankan ulang pada thread yang sudah bersih tidak melakukan apa-apa dan tidak error. Berjalan tidak boleh memblokir startup lama (jalankan di goroutine setelah OnReady dengan batas waktu per perintah git).
- UI kecil di `frontend/src/modules/thread/recovery/`:
  - Komponen lencana untuk peserta berstatus mati dengan teks "agent mati, akan dihidupkan saat pesan berikutnya" (slot panel peserta).
  - Tampilan pesan sistem pemulihan di aliran (pesan `kind: system` terkait pemulihan, gaya ringan, tidak menggantikan renderer pesan sistem milik tiket 1; hanya dipasang bila ada penanda pesan/pola yang disepakati, mis. field opsional atau awalan teks tetap yang didefinisikan di Kontrak).
  - Lencana hilang otomatis ketika event `thread.updated` membawa peserta hidup.

Acceptance criteria:
- [ ] Setelah menyisakan worktree/branch secara manual untuk thread `closed` lalu restart daemon, keduanya hilang dan ada baris log daemon yang mencatat pembersihan.
- [ ] Thread `active` (worktree dan branch-nya) tidak berubah sama sekali setelah start.
- [ ] `thread.json` dan log pesan thread `closed` tetap utuh dan terbaca.
- [ ] Menjalankan pembersihan dua kali berturut-turut aman (tanpa error).
- [ ] Kegagalan pada satu thread/project tidak menghentikan pembersihan thread lain; startup daemon tidak terblokir.
- [ ] Di UI, peserta thread aktif pascarestart menampilkan lencana "agent mati, akan dihidupkan saat pesan berikutnya", dan lencana hilang setelah agent dihidupkan.
- [ ] Pesan sistem pemulihan tampil di aliran thread tanpa merusak renderer pesan lain.
- [ ] Build frontend lulus dan komponen hanya terpasang lewat slot, tanpa mengedit berkas milik tiket lain selain pemasangan aditif.

ownedPaths: `internal/thread/orphans.go`, `frontend/src/modules/thread/recovery/`

Fitur ini bisa dikerjakan paralel dengan `thread-lazy-respawn`. Kontrak antarmuka (field status mati pada `Participant`, bentuk pesan sistem) harus disepakati lewat bagian Kontrak: sisi frontend membaca field opsional yang sama yang ditulis `MarkDeadOnStart`; bila nama field belum ada saat implementasi, ikuti nama yang dipilih di `recovery.go` dan cerminkan di `frontend/src/modules/thread/types.ts` hanya sebagai tambahan aditif.

## Path yang dimiliki
Hanya path berikut yang boleh disentuh:
- `internal/thread/recovery.go`
- `internal/thread/orphans.go`
- `frontend/src/modules/thread/recovery/`

Pengecualian sempit (aditif, sebutkan di commit): satu baris pendaftaran hook di `cmd/shikigamid/wire_thread.go` dan penambahan field opsional pada `internal/thread/types.go` serta `frontend/src/modules/thread/types.ts`, bila titik ekstensi tiket sebelumnya tidak cukup.

## Wiring
Langkah penutup tiket ini, memakai titik ekstensi tiket 1 dan 2 sehingga hasilnya terlihat dan jalan tanpa bergantung pada tiket lain di luar `depends_on`:
1. Di `cmd/shikigamid/wire_thread.go` (dalam `RegisterModule('thread')` / `d.OnReady`), setelah Manager dibuat: buat `Recovery` dan daftarkan `Recovery.MarkDeadOnStart` lewat hook `OnStart` Manager (dijalankan sebelum IPC melayani permintaan thread), lalu daftarkan `CleanOrphans` pada `d.OnReady` (jalan di goroutine, tidak memblokir start). Ini satu-satunya perubahan di berkas milik tiket 1; bila hook sudah dipakai, tambahkan pendaftaran tanpa merombak.
2. Pasang `Recovery.EnsureAlive` sebagai pra-pengirim pada jalur pengiriman: handler `thread.send` (user ke master) dan jalur `thread_send` di router untuk target peserta, lewat hook `OnMessage`/titik ekstensi pengiriman Manager. Jika hook yang ada tidak bisa mencegat sebelum pesan masuk antrean, tambahkan hook aditif kecil di titik itu dan tulis di commit.
3. Frontend: di `frontend/src/modules/thread/recovery/index.ts` ekspor komponen lencana dan notis, lalu pasang ke slot peserta (panel peserta tiket 2) dan slot aliran (tab thread tiket 1) lewat titik render yang tersedia. Pendaftaran ke `lib/registry.ts` tetap lewat `index.ts` utama modul thread; bila sudah ada pola impor subfolder, ikuti.
4. Uji jalan manual: buat thread aktif dengan satu peserta, hentikan lalu jalankan ulang daemon, pastikan lencana tampil, kirim pesan dan pastikan agent hidup ulang di worktree yang sama; tutup satu thread, tinggalkan worktree sisa secara manual, restart daemon, pastikan dibersihkan.

## Guard
Cukup build berhasil, tanpa unit test:
- `go build ./...`
- build frontend (`make build` atau perintah build frontend proyek ini; aplikasi harus tetap berjalan).
