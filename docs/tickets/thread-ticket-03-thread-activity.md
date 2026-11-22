---
id: thread-ticket-03-thread-activity
title: Kartu aktivitas agent di aliran thread
spec: docs/superpowers/specs/2026-11-22-thread-design.md
status: implemented
wave: 3
depends_on: [thread-ticket-02-thread-agents]
---
# Kartu aktivitas agent di aliran thread

## Tujuan
Di thread, user harus bisa melihat apa yang dikerjakan tiap agent (baca/edit file, jalankan perintah) tanpa aliran chat tenggelam oleh baris aksi. Anti-goal spec: agent auto-approve tidak boleh mengubah file tanpa user tahu apa yang terjadi. Epic ini menurunkan pesan `kind=activity` dari data aktivitas yang sudah ada (hook tool call di `internal/status`), menyimpannya ke log thread, lalu menampilkannya sebagai kartu aktivitas per agent yang bisa dibuka-tutup. Pesan diskusi antar agent tetap tampil penuh, tidak dilipat (keputusan 5 spec).

## Hasil yang bisa dijalankan
Di atas tiket 1 dan 2, setelah tiket ini selesai:
- Saat peserta thread (master maupun peer) memakai tool (Read, Edit, Write, Bash, Grep, dst.), aliran chat thread menampilkan satu kartu aktivitas per agent per rentetan aksi, mis. "project-x: 5 aksi" dengan status "mengerjakan". Kartu bertambah hitungannya secara langsung selama agent bekerja.
- Kartu bisa dibuka untuk melihat daftar aksi (mis. "Edit auth/claims.go", "Bash go build ./...") dan ditutup lagi. Default tertutup. Saat agent berhenti bekerja (ada pesan diskusi dari agent itu, atau jeda panjang), status kartu menjadi "selesai".
- Pesan diskusi (`message`) dan pesan sistem tetap tampil penuh di antara kartu. Kartu dari agent yang berbeda tidak digabung; rentetan aksi satu agent yang terpotong pesan diskusi memulai kartu baru setelah pesan itu.
- Thread yang `closed` atau dibuka ulang setelah restart menampilkan kartu yang sama dari log (kartu dibangun dari log, bukan dari state memori).
- Aksi yang dilakukan user langsung di terminal peserta (keputusan 7 spec) juga muncul di kartu, karena sumbernya hook tool call sesi itu.
- Aktivitas sesi interaktif milik user (bukan peserta thread) tidak pernah masuk log thread.
- `go build ./...` dan build frontend lulus; aplikasi berjalan.

## Batasan
In scope:
- `internal/thread/activity.go`: listener hook tool call, penyaringan sesi peserta, penurunan ringkasan, penggabungan beruntun, simpan ke log thread, publish event.
- `frontend/src/modules/thread/activity/`: komponen kartu aktivitas, logika pengelompokan pesan, satu titik render yang disisipkan ke aliran chat.
- Perubahan kecil di komponen aliran chat tiket 1 (satu titik render, disebut di Wiring).

Out of scope:
- Mengubah bentuk tipe di `types.go` (hanya boleh ditambah field opsional bila mutlak perlu), skema log, atau method IPC tiket 1.
- Mengubah `internal/status` atau `internal/runs` (hanya dipakai lewat API publik yang sudah ada).
- Melipat pesan diskusi; panel peserta (tiket 2); banner/Close (tiket 4); pemulihan restart (tiket 5).
- Polling. Seluruh penurunan berbasis event hook.
- Unit test, dependency baru.

## Kontrak
Dipakai (dari tiket sebelumnya; periksa nama persis di kode saat implementasi):
- `internal/thread/types.go` (tiket 1): `ThreadMessage{id,threadId,from,to,text,ts,kind}` dengan `kind=activity`, `Participant{sessionId,projectId,role,name}`, konstanta event `thread.message`.
- `Manager` (tiket 1): hook `OnStart`, `OnParticipantAdded`, `OnClose`, `OnMessage`; fungsi untuk menambah pesan ke log append-only dan mempublikasikan `thread.message`; pencarian thread aktif dari `sessionId` peserta (dipakai tiket 2 untuk token MCP, pakai ulang bukan duplikasi).
- `*status.Engine` dari `d.Lookup("status.engine")`: `AddHookListener(fn func(sessionID, event string, payload map[string]any)) (cancel func())`. Event relevan: `PostToolUse` dengan `payload["tool_name"]` dan `payload["tool_input"]` (pola sama dengan `status.ActivityLog.Record`).
- Nama peserta (`Participant.name`) untuk label ringkasan; bila `projectId` kosong (master) pakai name master.
- Frontend: komponen aliran chat tab thread (tiket 1) yang merender daftar `ThreadMessage`, store thread (event `thread.message`), dan `types.ts`.

Disediakan:
- Go (paket `internal/thread`):
  - `type ActivitySource interface { AddHookListener(fn func(sessionID, event string, payload map[string]any)) (cancel func()) }` (dipenuhi `*status.Engine`, antarmuka lokal agar tidak menambah import siklik).
  - `func InstallActivity(m *Manager, src ActivitySource) (stop func())` yang memasang listener dan mendaftarkan hook Manager.
  - Format `ThreadMessage` activity: `kind="activity"`, `from=<sessionId peserta>`, `to="all"`, `text=<ringkasan satu baris>`. Ringkasan berbentuk `<nama peserta>: <Tool> <target ringkas>` (mis. `project-x: Edit auth/claims.go`; target Bash = perintah, Read/Edit/Write = path relatif terhadap worktree bila bisa, Grep/Glob = pola; satu baris, maks 160 rune, newline jadi spasi).
  - Penggabungan: aksi beruntun satu agent digabung ke satu pesan activity terakhir milik agent itu selama belum ada pesan lain (message/system/activity agent lain) di log setelahnya dan selang waktu antar aksi di bawah ambang (konstanta, mis. 30 detik). Cara representasi gabungan: field opsional aditif pada `ThreadMessage`, `count int` (jumlah aksi) dan `items []string` (ringkasan per aksi, dibatasi maks 50 terbaru); `text` tetap ringkasan aksi terakhir. Pesan yang sama diperbarui di tempat (id sama) dan dipublikasikan ulang lewat `thread.message` sehingga UI mengganti pesan ber-id sama. Bila tiket 1 tidak mendukung pembaruan pesan di log (append-only), tulis entri baru ber-id sama dan pembaca (UI dan `thread.messages`) mengambil entri terakhir per id; putuskan di plan setelah membaca `store.go`, tanpa mengubah perilaku tiket 1.
- Frontend (`frontend/src/modules/thread/activity/`):
  - `ActivityCard.svelte` (props: `agentName`, `color`, `items`, `count`, `running`), `groupActivity.ts` (fungsi murni yang mengubah `ThreadMessage[]` menjadi daftar item aliran: pesan biasa atau grup activity), `index.ts` yang mengekspor `renderActivityGroup` / komponen untuk titik render.
  - Titik render: komponen aliran chat memanggil satu fungsi/komponen `ActivityFlowItem` untuk pesan `kind==='activity'`.

## Fitur

### thread-activity-daemon: Turunkan pesan activity dari status/runs
Deskripsi: `internal/thread/activity.go` memasang listener hook lewat `ActivitySource.AddHookListener` (event-driven, tanpa polling). Untuk tiap event `PostToolUse` dari `sessionID`, cari thread aktif dan peserta pemilik sesi itu lewat Manager; bila sesi bukan peserta thread aktif, abaikan (sesi interaktif user tidak boleh tersentuh). Bangun ringkasan (lihat Kontrak), lalu gabungkan dengan pesan activity terakhir agent itu bila memenuhi syarat penggabungan, atau buat pesan activity baru. Simpan lewat fungsi log milik Manager dan publikasikan `thread.message`. Panggilan harus aman terhadap panic (recover) dan tidak memblok jalur hook status; pekerjaan tulis log dikerjakan di goroutine/antrean pendek per thread dengan urutan terjaga. Pasang lewat `InstallActivity` yang menyambung ke hook Manager (`OnStart`/`OnParticipantAdded`) tiket 1 agar daftar peserta selalu mutakhir; `OnClose` melepas keadaan penggabungan thread itu. Thread `closed` tidak menerima pesan baru. Jaga agar pesan activity tidak masuk antrean pengiriman mention (`OnMessage` milik router tiket 2 harus mengabaikan `kind=activity`; bila router belum menyaringnya, tambahkan penyaringan di sisi activity dengan tidak memicu `OnMessage` untuk activity, atau jelaskan di Wiring) dan tidak dihitung pembatas anti-loop.

Acceptance criteria:
- [ ] Tool call peserta thread menghasilkan pesan `kind=activity` di log thread (`thread.messages` mengembalikannya) dan event `thread.message` terbit.
- [ ] Teks ringkasan berformat `<nama>: <Tool> <target>`, satu baris, maks 160 rune.
- [ ] 10 aksi beruntun satu agent dalam jendela waktu menghasilkan satu pesan activity dengan `count=10`, bukan 10 baris log.
- [ ] Pesan lain (diskusi, sistem, activity agent lain) di antara dua aksi memutus penggabungan; aksi berikutnya membuat pesan activity baru.
- [ ] Tool call sesi yang bukan peserta thread aktif tidak menghasilkan apa pun.
- [ ] Pesan activity tidak dikirim ke agent mana pun lewat antrean mention dan tidak menambah hitungan anti-loop.
- [ ] Setelah thread `closed`, tool call sisa tidak menulis log.
- [ ] Panic atau error tulis log tidak menjatuhkan daemon dan tidak menghambat pemrosesan hook status.
- [ ] `go build ./...` lulus.

ownedPaths:
- `internal/thread/activity.go`

Fitur ini bisa dikerjakan paralel dengan `thread-activity-ui`; pegangan bersama adalah format `ThreadMessage` activity (`kind`, `text`, `count?`, `items?`) pada bagian Kontrak.

### thread-activity-ui: Kartu aktivitas buka-tutup per agent
Deskripsi: di `frontend/src/modules/thread/activity/`, `groupActivity.ts` mengubah daftar `ThreadMessage` aliran menjadi item aliran: pesan non-activity diteruskan apa adanya, pesan activity beruntun milik agent yang sama (tanpa pesan lain di antaranya) dikelompokkan menjadi satu grup. Untuk pesan activity yang sudah digabung daemon (`count`/`items`), gunakan nilai itu; untuk yang tidak membawa `items`, hitung dari `text`. Pesan ber-id sama digantikan oleh versi terbaru (pembaruan di tempat dari event). `ActivityCard.svelte` menampilkan "`<nama>: N aksi`", status "mengerjakan" bila aksi terakhir baru (mis. < 30 detik dan belum ada pesan lain dari agent itu) atau "selesai", penanda warna agent yang sama dengan avatar di aliran chat (pakai fungsi warna dari tiket 1; jangan membuat skema warna baru), toggle buka-tutup (default tertutup) dengan tombol yang dapat diakses keyboard (`aria-expanded`), dan daftar aksi satu baris per item saat terbuka (teks panjang dipotong dengan ellipsis, terpanjang tampil penuh lewat atribut `title`). Status buka-tutup disimpan per kartu di memori komponen (boleh `localStorage` dibungkus try/catch). Gaya mengikuti token tema aplikasi (terang/gelap) dan tidak memakai warna keras. Satu titik render: komponen aliran chat tiket 1 memanggil `ActivityFlowItem` (diekspor `index.ts` modul activity) untuk pesan activity; perubahan di komponen aliran dibatasi pada satu cabang render dan impor, serta disebut di Wiring. Pesan diskusi tidak pernah dilipat.

Acceptance criteria:
- [ ] Beberapa pesan activity beruntun satu agent tampil sebagai satu kartu "`<nama>: N aksi`" dengan status "mengerjakan" atau "selesai".
- [ ] Kartu tertutup secara default; klik atau Enter/Space pada header membuka daftar aksi dan menutupnya lagi; `aria-expanded` mengikuti.
- [ ] Pesan diskusi antar agent dan pesan sistem tampil penuh di antara kartu dan tidak pernah masuk ke dalam kartu.
- [ ] Aksi dua agent berbeda menghasilkan dua kartu terpisah; pesan lain di tengah rentetan memulai kartu baru.
- [ ] Kartu bertambah hitungannya langsung ketika event `thread.message` memperbarui pesan activity ber-id sama, tanpa muat ulang.
- [ ] Tampilan di tema terang dan gelap terbaca, di lebar sempit tidak menimbulkan scroll horizontal, dan aliran tetap menempel ke bawah saat pesan baru masuk seperti perilaku tiket 1.
- [ ] Thread `closed` menampilkan kartu dari log dengan status "selesai".
- [ ] Build frontend lulus tanpa error tipe.

ownedPaths:
- `frontend/src/modules/thread/activity/`

Fitur ini bisa dikerjakan paralel dengan `thread-activity-daemon`; boleh memakai data uji lokal berformat `ThreadMessage` sampai daemon selesai.

## Path yang dimiliki
Hanya ini yang boleh disentuh:
- `internal/thread/activity.go`
- `frontend/src/modules/thread/activity/`
- (aditif, minimal, disebut di Wiring) satu cabang render di komponen aliran chat milik tiket 1 di `frontend/src/modules/thread/`
- (aditif, hanya bila tidak ada titik ekstensi lain, disebut di Wiring) satu baris pemanggilan `InstallActivity` di `cmd/shikigamid/wire_thread.go`

## Wiring
Langkah penutup tiket ini, agar hasilnya terlihat dan jalan tanpa bergantung pada tiket 4/5:
1. Daemon: panggil `InstallActivity(m, eng)` setelah Manager dan `status.Engine` tersedia. Utamakan titik ekstensi yang ada: bila tiket 1 menyediakan pendaftaran ekstensi/hook yang dijalankan dari `d.OnReady` (mis. daftar fungsi yang dipanggil dengan Manager), daftarkan dari `activity.go` (`init()`), tanpa menyentuh `wire_thread.go`. Bila tidak ada, tambahkan satu baris di callback `d.OnReady` pada `cmd/shikigamid/wire_thread.go`: `Lookup("status.engine")` lalu `thread.InstallActivity(mgr, eng)`; bila engine tidak tersedia, log peringatan dan lanjut tanpa panic (thread tetap jalan tanpa kartu).
2. Pastikan `OnMessage` (router tiket 2) mengabaikan `kind=activity` untuk pengiriman dan anti-loop; bila router tiket 2 belum menyaring, `InstallActivity` menulis pesan activity lewat jalur log+publish yang tidak memanggil `OnMessage`. Catat pilihan yang diambil di commit.
3. Frontend: di komponen aliran chat tiket 1, tambahkan satu cabang render: untuk item dari `groupActivity(messages)` yang berjenis grup activity, render `ActivityFlowItem` dari `modules/thread/activity`; item lain dirender seperti semula. Pengelompokan dipanggil sekali atas daftar pesan sebelum `{#each}`. Tidak ada perubahan perilaku lain pada komponen itu.
4. Event `thread.message` yang membawa id sudah ada harus mengganti pesan lama di store tab thread; bila store tiket 1 hanya menambahkan, ubah penanganannya menjadi upsert berdasarkan `id` (perubahan kecil, sebutkan di commit).
5. Verifikasi manual: buat thread, biarkan master men-spawn satu peer yang mengedit file; kartu muncul di aliran, bertambah hitungannya, bisa dibuka-tutup, dan pesan diskusi tetap penuh. Sesi interaktif user di project yang sama tidak memunculkan kartu.

## Guard
Cukup build berhasil, tanpa unit test:
- `go build ./...`
- build frontend (`npm run build` di `frontend/`, atau `make build` bila tersedia).
