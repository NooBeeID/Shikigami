---
id: thread-ticket-04-thread-close
title: Usul tutup, banner, Close dengan konfirmasi, dan cleanup penuh
spec: docs/superpowers/specs/2026-11-22-thread-design.md
status: implemented
wave: 4
depends_on: [thread-ticket-03-thread-activity]
---
# Usul tutup, banner, Close dengan konfirmasi, dan cleanup penuh

## Tujuan
Menyelesaikan siklus hidup thread: master dapat menandai goal tercapai (banner hijau "siap ditutup"), tetapi hanya user yang menutup thread lewat Close. Sebelum menutup, user melihat dialog konfirmasi yang mendaftar tiap worktree beserta perubahan belum di-commit dan commit belum di-merge, supaya pekerjaan tidak hilang tanpa disadari (anti-goal spec). Saat dikonfirmasi, semua sesi thread dimatikan, semua worktree dan branch thread dihapus (cleanup penuh), dan thread menjadi `closed` serta read-only namun tetap terbaca.

## Hasil yang bisa dijalankan
Di atas tiket 1 sampai 3 (thread, master, peserta, worktree, kartu aktivitas):
- Master memanggil tool MCP `thread_propose_close(summary)`: pada tab thread muncul banner hijau "goal tercapai" berisi summary dan tombol Close; daftar thread di sidebar menampilkan indikator "siap ditutup" untuk thread itu. Thread tidak tertutup sendiri.
- Header tab thread punya tombol "Tandai selesai"/Close yang bisa ditekan kapan saja walau master belum mengusulkan.
- Menekan Close (di header atau banner) membuka dialog konfirmasi: tiap worktree peserta ditampilkan dengan daftar perubahan belum di-commit dan commit yang belum ada di branch dasar. User bisa Batal (tidak ada yang berubah) atau Lanjut tutup.
- Setelah Lanjut: semua sesi agent thread (master dan peserta) mati, semua worktree dan branch thread terhapus dari disk dan repo project, thread berstatus `closed`. Tab thread tetap bisa dibuka dan dibaca, input dinonaktifkan, dan pengiriman pesan baru ke thread closed ditolak daemon.
- `go build ./...` dan build frontend lulus, aplikasi berjalan.

## Batasan
In scope:
- Tool MCP `thread_propose_close` (master saja) yang menyalakan `readyToClose`.
- IPC `thread.closePreview` dan `thread.close`.
- Inspeksi worktree: perubahan belum di-commit dan commit belum di-merge ke branch dasar.
- Cleanup penuh: Kill semua sesi thread, hapus worktree dan branch thread, state `closed` + `closedAt`.
- Penolakan pesan baru (user maupun `thread_send`) ke thread closed.
- UI: banner, tombol Close di header, dialog konfirmasi, tampilan read-only, indikator "siap ditutup" di sidebar.

Out of scope:
- Merge, push, atau PR otomatis; cara hasil kerja keluar dari branch thread adalah urusan user atau instruksi ke agent sebelum Close.
- Master menutup thread sendiri (tidak boleh; tool hanya mengusulkan).
- Pemulihan setelah restart daemon dan pembersihan worktree yatim (tiket 5); epic ini hanya membuat Close idempoten dan tahan gagal sebagian sehingga tiket 5 dapat mengulanginya.
- Mengubah bentuk tipe di `types.go` (hanya boleh menambah field opsional), merombak `mcp_tools.go`/`ipc.go`/`manager.go`, mengubah mention biasa.
- Unit test, dependency baru, lintas workspace.

## Kontrak

Dipakai dari tiket sebelumnya (nama persis mengikuti kode tiket 1 sampai 3; bila nama aktual berbeda sedikit, sesuaikan tanpa mengubah arti):
- `internal/thread/types.go`: `Thread{id,workspaceId,title,goal,state(active|closed),createdAt,closedAt,participants[],readyToClose}`, `Participant{sessionId,projectId,role,name}`, `ThreadMessage{id,threadId,from,to,text,ts,kind}`, konstanta nama method/event.
- Penyimpanan: `DataDir/threads/<threadId>/thread.json`, log jsonl, direktori master `DataDir/threads/<threadId>/master`, worktree peserta `DataDir/threads/<threadId>/<projectSlug>`, branch `thread/<threadId>-<slug>`.
- `Manager` (tiket 1): akses store, publish event `thread.updated` (Thread penuh) dan `thread.message`, hook `OnClose`, dan titik penolakan pesan; `ipc.go` dengan `RegisterMethod`.
- `worktree.go` (tiket 2): fungsi pembuat/penghapus worktree dan branch serta fungsi inspeksi worktree (status perubahan dan commit di depan branch dasar). Bila inspeksi belum tersedia sebagai fungsi terpisah, `close.go` menambahkan helper inspeksi sendiri di `close.go` (memakai `git status --porcelain` dan `git log <base>..<branch>`) tanpa mengubah `worktree.go`.
- `mcp_tools.go` (tiket 2): registri tool MCP; tool didaftarkan lewat registri, pemanggil dikenali dari token sesi dan dibatasi ke thread-nya, peran master diperiksa.
- `session.Manager.Kill(sessionID)` untuk mematikan sesi; antrean `internal/mention` untuk pesan thread (pesan ke sesi yang sudah mati harus gagal bersih).
- Frontend: `frontend/src/modules/thread/types.ts`, komponen tab thread (tiket 1) dengan slot/titik render untuk header dan aliran, `lib/registry.ts`, klien IPC dan penyimpanan thread di frontend.

Disediakan epic ini:
- Tool MCP `thread_propose_close(summary string)`, hanya master. Efek: set `Thread.readyToClose = true`, simpan `summary` (tambahkan field opsional `closeSummary` pada `Thread` bila perlu, atau cukup tulis pesan `system` berisi summary ke log; pilih satu dan konsisten dengan frontend), publish `thread.updated`. Tidak menutup thread. Pada thread closed, ditolak dengan error jelas. Pemanggil non-master ditolak dengan error jelas.
- IPC `thread.closePreview{id}` mengembalikan `{ threadId, worktrees: [{ participant: {sessionId,name,projectId}, path, branch, baseBranch, uncommitted: [{path,status}], unmergedCommits: [{sha,subject}], error? }] }`. Read-only, tidak mengubah apa pun. Satu worktree yang gagal diinspeksi menghasilkan entri dengan `error`, bukan kegagalan seluruh panggilan.
- IPC `thread.close{id}` mengembalikan Thread yang sudah `closed`. Idempoten: memanggil untuk thread yang sudah `closed` mengembalikan thread tanpa error. Urutan: tandai tahap penutupan agar pesan baru ditolak, Kill semua sesi peserta dan master, hapus tiap worktree (force) lalu branch-nya, hapus direktori kerja thread yang berisi worktree (direktori master boleh dihapus; `thread.json` dan log pesan WAJIB tetap), set `state=closed` dan `closedAt`, simpan, publish `thread.updated`. Kegagalan satu langkah tidak menghentikan langkah lain; error dikumpulkan dan dicatat sebagai pesan `system` ke log; thread tetap menjadi `closed` hanya bila semua sesi sudah mati (worktree/branch yang gagal dihapus dibiarkan untuk dibersihkan tiket 5 dan dilaporkan di pesan `system`).
- Konstanta nama method `thread.closePreview` dan `thread.close` di `types.go` bila belum ada (hanya menambah konstanta, dicerminkan di `types.ts`); bila menambah `types.go`/`types.ts` tidak diizinkan oleh batas kepemilikan, definisikan konstanta di `close.go` dan di `frontend/src/modules/thread/close/` dan mintalah penyatuan di tiket berikutnya (tidak mengubah file tiket lain).
- Penolakan pesan ke thread closed: `thread.send` dan `thread_send` mengembalikan error "thread sudah ditutup". Karena `manager.go`/`ipc.go` milik tiket lain, penolakan dipasang lewat hook `OnMessage` dari Manager (atau pemeriksaan state pada titik penolakan yang sudah disediakan tiket 1) yang didaftarkan dari `close.go`.
- Frontend (`frontend/src/modules/thread/close/`): komponen `CloseBanner`, `CloseButton`, `CloseDialog`, indikator `ReadyBadge` (untuk sidebar), dan helper `closeApi.ts` (pembungkus `thread.closePreview` dan `thread.close`), plus `index.ts` lokal yang mengekspor semuanya.

## Fitur

### thread-close-daemon: propose_close, inspeksi, dan cleanup penuh
Deskripsi: file `internal/thread/close.go` dan `internal/thread/close_ipc.go`.
- `close.go`: logika `ProposeClose(threadID, callerSessionID, summary)`, `Preview(threadID)`, `Close(threadID)`, dan helper inspeksi worktree. Inspeksi per worktree peserta: perubahan belum di-commit (`git status --porcelain`, termasuk untracked) dan commit yang belum ada di branch dasar (`git log <baseBranch>..<threadBranch>`; branch dasar adalah branch/HEAD project saat spawn yang dicatat tiket 2; bila tidak tercatat, pakai HEAD saat ini dari repo utama project; bila branch dasar sudah tidak ada, laporkan semua commit di branch thread). Gunakan `exec.CommandContext` dengan timeout; jangan memakai shell string.
- Cleanup: `session.Manager.Kill` untuk tiap `sessionId` (master dan peserta), lalu `git worktree remove --force <path>` dan `git branch -D <branch>` dijalankan dengan `-C <repo utama project>`, lalu `git worktree prune`. Hapus direktori sisa bila masih ada. Master tidak punya worktree sehingga hanya sesinya dimatikan.
- `close_ipc.go`: mendaftarkan method `thread.closePreview` dan `thread.close` lewat `RegisterMethod`, serta mendaftarkan tool `thread_propose_close` lewat registri di `mcp_tools.go`. Pendaftaran dilakukan dari fungsi `RegisterClose(m *Manager)` (atau setara) yang dipanggil dari titik inisialisasi/hook modul thread (lihat Wiring).
- Penolakan pesan ke thread closed (lihat Kontrak) dan penolakan `thread_propose_close` pada thread closed.
- Concurrency: `Close` single-flight per thread (mutex per thread); panggilan kedua yang bersamaan menunggu atau mengembalikan hasil yang sama.

Acceptance criteria:
- [ ] `thread_propose_close(summary)` hanya tersedia untuk master; peserta peer mendapat error jelas; hasilnya `readyToClose=true` dan event `thread.updated` terbit; thread tidak berubah menjadi closed.
- [ ] `thread.closePreview` mengembalikan satu entri per worktree peserta dengan daftar perubahan belum di-commit dan commit belum di-merge; tidak mengubah worktree; satu worktree gagal diinspeksi tidak menggagalkan panggilan.
- [ ] `thread.close` mematikan semua sesi thread (master dan peserta), menghapus semua worktree dan branch thread, set `state=closed` dan `closedAt`, publish `thread.updated`.
- [ ] Setelah Close tidak ada proses agent thread yang hidup, tidak ada worktree di `DataDir/threads/<threadId>/<projectSlug>`, tidak ada branch `thread/<threadId>-*` di repo project, dan `git worktree list` project bersih.
- [ ] `thread.json` dan log pesan tetap ada; `thread.get` dan `thread.messages` untuk thread closed tetap berfungsi.
- [ ] `thread.send` dan `thread_send` ke thread closed ditolak dengan error jelas.
- [ ] `thread.close` idempoten, dan kegagalan satu langkah (mis. branch sudah hilang) tidak menghentikan langkah lain; kegagalan dicatat sebagai pesan `system`.
- [ ] Working tree utama project tidak pernah disentuh oleh preview maupun close (semua perintah git menarget worktree thread atau hanya `worktree remove`/`branch -D` untuk artefak thread).
- [ ] `go build ./...` lulus.

ownedPaths: `internal/thread/close.go`, `internal/thread/close_ipc.go`.
Bisa dikerjakan paralel dengan `thread-close-ui` (kontrak IPC dan bentuk respons di atas menjadi pegangan bersama).

### thread-close-ui: Banner goal tercapai, tombol Close, dan dialog konfirmasi
Deskripsi: komponen Svelte 5 + TypeScript di `frontend/src/modules/thread/close/`.
- `CloseBanner`: tampil di aliran tab thread bila `thread.readyToClose && thread.state === 'active'`: banner hijau "Goal tercapai" dengan summary dan tombol Close. Memakai token warna tema yang sudah ada (jangan hard-code warna baru di luar pola komponen lain).
- `CloseButton`: tombol "Tandai selesai"/Close di header tab thread, aktif kapan saja selama `state === 'active'`; disembunyikan atau dinonaktifkan untuk thread closed.
- `CloseDialog`: dipicu oleh banner maupun tombol header. Saat dibuka memanggil `thread.closePreview`, menampilkan loading, lalu mendaftar tiap worktree (nama peserta, project, branch) dengan dua daftar: perubahan belum di-commit (path + status) dan commit belum di-merge/push (sha singkat + subject). Worktree yang bersih ditandai "tidak ada yang akan hilang"; entri dengan `error` ditampilkan sebagai peringatan "tidak bisa diperiksa". Peringatan merah yang menyebut bahwa semua worktree dan branch akan dihapus permanen. Tombol "Batal" dan "Lanjut tutup" (memanggil `thread.close`, menampilkan status proses, menutup dialog saat sukses, menampilkan error bila gagal tanpa menutup). Tombol tidak bisa ditekan ganda.
- Tampilan thread closed read-only: tersedia helper/flag (mis. `isThreadReadOnly(thread)`) dan komponen penanda "Thread ditutup pada <closedAt>" yang dipasang di area input sehingga input dinonaktifkan; aliran pesan tetap tampil.
- `ReadyBadge`: indikator kecil "siap ditutup" untuk item daftar thread di sidebar, dipasang lewat titik render item yang disediakan tiket 1 (bila tiket 1 hanya menyediakan indikator status generik, tambahkan lewat slot yang ada tanpa mengubah file tiket lain).
- Semua data dari store thread yang ada (mengikuti event `thread.updated`), tanpa polling; pembaruan UI setelah Close berasal dari event `thread.updated`.

Acceptance criteria:
- [ ] Banner hijau muncul hanya saat `readyToClose` dan thread aktif, menampilkan summary dan tombol Close.
- [ ] Tombol Close di header bisa dipakai kapan saja selama thread aktif.
- [ ] Dialog mendaftar tiap worktree dengan perubahan belum di-commit dan commit belum di-merge; menangani loading, error, dan worktree bersih.
- [ ] Batal menutup dialog tanpa memanggil `thread.close`; Lanjut memanggil `thread.close` sekali, lalu thread tampil sebagai closed.
- [ ] Thread closed: input dinonaktifkan, penanda "ditutup" tampil, aliran tetap terbaca, tombol Close hilang atau nonaktif.
- [ ] Indikator "siap ditutup" tampil di daftar sidebar untuk thread dengan `readyToClose` aktif dan hilang setelah closed.
- [ ] Tidak ada polling/timer; semua pembaruan lewat event.
- [ ] Build frontend lulus (termasuk type check).

ownedPaths: `frontend/src/modules/thread/close/`.
Bisa dikerjakan paralel dengan `thread-close-daemon`; pengujian end-to-end butuh keduanya terpasang lewat Wiring.

## Path yang dimiliki
- `internal/thread/close.go`
- `internal/thread/close_ipc.go`
- `frontend/src/modules/thread/close/`

Hanya path ini yang boleh disentuh. Titik sambung ke file milik tiket lain hanya lewat titik ekstensi yang sudah disediakan (hook Manager, `RegisterMethod`, registri `mcp_tools.go`, slot komponen tab thread). Bila sebuah titik ekstensi ternyata tidak ada, jangan mengedit file pemiliknya; laporkan sebagai blocker dalam catatan implementasi.

## Wiring
Langkah penutup tiket ini, hanya lewat titik ekstensi yang sudah ada dari tiket 1 sampai 3:
1. Daemon: dari fungsi pendaftaran yang dieksekusi saat modul thread start (hook `OnStart` Manager, atau init paket `internal/thread` lewat fungsi `RegisterClose` yang dipanggil dari hook itu; `cmd/shikigamid/wire_thread.go` tidak diedit), daftarkan `thread.closePreview`, `thread.close`, tool `thread_propose_close` ke registri `mcp_tools.go`, dan pemeriksaan penolakan pesan thread closed lewat hook `OnMessage`. Bila hook belum bisa dipasang tanpa mengedit file tiket lain, gunakan fungsi `init()` di `close_ipc.go` yang mendaftarkan callback ke registri paket (pola registrasi seperti `provider.RegisterLaunchContribution`).
2. Frontend: dari `frontend/src/modules/thread/close/index.ts`, pasang `CloseButton` ke slot header tab thread, `CloseBanner` ke slot aliran (di atas input), penanda read-only ke slot input, dan `ReadyBadge` ke slot item daftar sidebar, memakai mekanisme slot/titik render yang disediakan komponen tab thread tiket 1 (registrasi sisi-efek yang diimpor `index.ts` modul thread; bila impor itu perlu satu baris di `frontend/src/modules/thread/index.ts` milik tiket 1 dan titik ekstensi otomatis tidak ada, catat sebagai satu-satunya perubahan lintas-path yang diizinkan).
3. Verifikasi manual: buat thread, spawn minimal satu peserta, biarkan ada satu perubahan belum di-commit di worktree, panggil `thread_propose_close` (banner muncul), tekan Close, cek dialog mendaftar perubahan, Lanjut, lalu cek sesi mati, worktree dan branch hilang, thread closed dan read-only, dan kirim pesan ke thread itu ditolak.
Tiket ini tidak bergantung pada tiket 5 untuk bisa dijalankan.

## Guard
- `go build ./...` berhasil.
- Build frontend berhasil (mis. `npm run build` di `frontend/`, termasuk type check Svelte/TS).
- Tanpa unit test.
