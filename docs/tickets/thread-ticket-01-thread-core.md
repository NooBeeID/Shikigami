---
id: thread-ticket-01-thread-core
title: Thread inti: entitas, penyimpanan, master, dan tab chat
spec: docs/superpowers/specs/2026-11-22-thread-design.md
status: implemented
wave: 1
depends_on: []
---
# Thread inti: entitas, penyimpanan, master, dan tab chat

## Tujuan
Fitur thread (diskusi multi-agent lintas project dalam satu workspace) butuh fondasi yang berdiri sendiri: entitas thread yang tersimpan di disk, sesi master yang di-spawn daemon, dan UI untuk melihat serta mengirim pesan. Epic ini membuat kemampuan pertama: user menulis satu goal, daemon membuat thread, men-spawn master (provider default, auto-approve, direktori netral) dengan prompt awal, dan user memantau aliran chat serta mengirim pesan ke master dari tab thread. Epic ini juga menetapkan kontrak (tipe, nama IPC, titik ekstensi) yang dipakai tiket thread-agents, thread-activity, thread-close, dan thread-recovery sehingga tiket-tiket itu hanya menambah file.

## Hasil yang bisa dijalankan
Setelah tiket ini selesai (tiket pertama, tanpa dependensi):
- Sidebar kiri punya bagian "Thread" di atas daftar sesi, berisi tombol "Thread baru" dan daftar thread workspace aktif, masing-masing dengan indikator aktif/idle.
- Klik "Thread baru" membuka dialog dengan kolom goal teks bebas. Setelah dikirim, thread muncul di daftar, tab thread terbuka di area tengah, dan daemon men-spawn sesi master dengan prompt awal (goal, daftar project workspace, penjelasan peran).
- Tab thread menampilkan header (judul, status), aliran chat satu kolom (avatar berwarna, "pengirim -> penerima", pesan sistem seperti "master dibuat"), dan kolom input. Pesan user dikirim ke master saat master idle dan tercatat di aliran.
- Thread tersimpan di disk dan tetap terbaca setelah UI dibuka ulang. Master belum punya tool thread (belum bisa men-spawn peserta); balasan master terlihat di terminal sesinya, bukan di aliran chat (itu datang dari tiket thread-agents).
- `go build ./...` dan build frontend lulus; aplikasi berjalan seperti sebelumnya untuk fitur lain.

## Batasan
In scope:
- Paket baru `internal/thread`: tipe, store disk, manager, spawn master, prompt awal, IPC.
- Modul daemon `thread` (`RegisterModule` di `cmd/shikigamid/wire_thread.go`).
- Titik ekstensi untuk tiket berikut: hook Manager, registri tool MCP (placeholder), `RegisterMethod` IPC.
- Modul frontend `frontend/src/modules/thread/` (client, store, sidebar, dialog, tab chat, slot render) dan perluasan minimal `TabTypeSpec.type` dengan `'thread'`.

Out of scope (jangan dibuat, milik tiket lain):
- Tool MCP `shikigami-thread`, `thread_spawn`, worktree git, peserta peer, router pesan agent ke agent, pembatas anti-loop (thread-agents).
- Kartu aktivitas dan pesan `kind=activity` (thread-activity).
- Banner "siap ditutup", konfirmasi Close, `thread.close`, cleanup worktree/branch (thread-close).
- Pemulihan setelah restart daemon, pembersihan worktree yatim (thread-recovery).
- Panel peserta di kanan, penyaringan sesi thread dari daftar sesi UI (tiket berikut; di sini hanya menyediakan penanda).
- Merge otomatis, lintas workspace, router berbasis aturan, polling, unit test, dependency baru.
- Mengubah perilaku mention global (mode `confirm_agent` tetap).

## Kontrak

### Tipe Go (`internal/thread/types.go`, module `github.com/NooBeeID/shikigami`)
Bentuk berikut tidak boleh berubah setelah tiket ini (tiket lain hanya menambah field opsional dengan `omitempty`). JSON memakai camelCase seperti kode lain.
- `Thread{ID, WorkspaceID, Title, Goal string; State ("active"|"closed"); CreatedAt time.Time; ClosedAt *time.Time; Participants []Participant; ReadyToClose bool}` dengan tag json `id, workspaceId, title, goal, state, createdAt, closedAt(omitempty), participants, readyToClose`.
- `Participant{SessionID, ProjectID, Role ("master"|"peer"), Name string}` tag json `sessionId, projectId, role, name`. Master: `ProjectID` kosong.
- `ThreadMessage{ID, ThreadID, From, To, Text string; TS time.Time; Kind ("message"|"system"|"activity")}` tag json `id, threadId, from, to, text, ts, kind`. `From`: `"user"` atau sessionId; `To`: sessionId, `"user"`, atau `"all"`. Pesan sistem memakai `From` = `"system"`, `To` = `"all"`.
- Konstanta string untuk state, role, kind.
- ID thread dan pesan: ULID atau string unik monotonik (ikuti pola `newID` di `internal/session` / `internal/mention`; tanpa dependency baru).

### Konstanta IPC (didefinisikan di `types.go`, dicerminkan di `frontend/src/modules/thread/types.ts`)
Metode: `thread.create`, `thread.list`, `thread.get`, `thread.messages`, `thread.send`. Event publish: `thread.updated`, `thread.message`. Tiket berikut menambah `thread.closePreview` dan `thread.close` lewat `RegisterMethod`.
- `thread.create{workspaceId, goal, title?}` -> `Thread`. Bila `title` kosong, turunkan dari goal (baris pertama, dipotong sekitar 60 karakter). Goal kosong -> error invalid.
- `thread.list{workspaceId}` -> `Thread[]` (urut `createdAt` menurun; termasuk yang `closed`).
- `thread.get{id}` -> `Thread`; id tidak dikenal -> error not found.
- `thread.messages{id, sinceTs?}` -> `ThreadMessage[]` urut waktu naik; `sinceTs` (RFC3339) hanya mengembalikan pesan yang lebih baru.
- `thread.send{id, text}` -> `ThreadMessage` (pesan user ke master). Ditolak bila thread `closed` atau teks kosong.
- Tambahan kecil untuk UI/tiket berikut: `thread.ownedSessions{}` -> `{sessionIds: string[]}` berisi semua sessionId milik thread (penanda "milik thread" yang bisa dipakai UI untuk menyaring daftar sesi; lihat catatan metadata di fitur thread-master-daemon).
- Event `thread.updated` membawa `Thread` penuh; `thread.message` membawa `ThreadMessage`. Terbitkan lewat mekanisme publish `internal/ipc` yang sudah dipakai modul lain (ikuti pola `internal/mention/ipc.go`).

### Penyimpanan (`internal/thread/store.go`)
- `DataDir/threads/<threadId>/thread.json` (ditulis atomik: tulis berkas sementara lalu rename).
- `DataDir/threads/<threadId>/messages.jsonl`, append-only, satu `ThreadMessage` per baris, dengan mutex per thread.
- Direktori master: `DataDir/threads/<threadId>/master` (dibuat saat thread dibuat).
- Thread `closed` tetap terbaca. Store menyediakan: `Create`, `Get`, `List(workspaceID)`, `Save(Thread)`, `AppendMessage`, `Messages(id, sinceTs)`, `MasterDir(id)`, `ThreadDir(id)`. Baris jsonl yang rusak dilewati (log peringatan), tidak menggagalkan pembacaan.
- Path dasar dari `internal/paths.DataDir()` (menghormati `SHIKIGAMI_DATA_DIR`). Konstruktor menerima direktori dasar agar bisa diinjeksi.

### Titik ekstensi (disediakan epic ini untuk tiket berikut)
- `Manager` menyediakan hook yang bisa didaftarkan banyak pihak: `OnStart(func(*Manager))`, `OnMessage(func(Thread, ThreadMessage))` (dipanggil setelah pesan tercatat), `OnParticipantAdded(func(Thread, Participant))`, `OnClose(func(Thread))`. Metode publik yang dibutuhkan tiket lain: `Get`, `List`, `AppendMessage`/`Post` (catat + publish + panggil hook), `AddParticipant`, `UpdateThread`, `SendToSession(threadID, from, to, text)` (antrean mention tanpa konfirmasi, lihat di bawah).
- Registri tool MCP: tipe `ToolRegistry` di `manager.go` (atau berkas milik epic ini) dengan `Register(name string, tool ToolSpec)` dan `Tools()`; di epic ini kosong dan hanya kerangka (`ToolSpec{Name, Description, MasterOnly bool, Handler func(ctx, ThreadCaller, args map[string]any) (any, error)}`; `ThreadCaller{ThreadID, SessionID string, Role string}`). Tiket thread-agents memakainya di `mcp_tools.go`.
- `ipc.go` menyediakan `RegisterMethod(name string, handler func(params json.RawMessage) (any, error))` agar tiket thread-close dkk. menambah metode tanpa mengedit berkas ini.
- Pengiriman pesan thread: `Manager` memakai instance `mention.Queue` yang sudah ada (`internal/mention/queue.go`, `Enqueue` dengan `source` agent/user, pengiriman saat idle lewat `status.OnIdle`) dalam jalur pengiriman khusus thread yang tanpa konfirmasi, terpisah dari `Queue.Mode()` global. Mention biasa tidak berubah. Bila `Queue.Enqueue` tunduk pada mode `confirm_agent` untuk source agent, gunakan `source=user` untuk pesan user ke master, dan sediakan pembungkus tersendiri (atau parameter bypass aditif yang tidak mengubah perilaku default) di sisi thread; jangan mengedit `internal/mention` kecuali sama sekali tidak ada jalan lain, dan sebutkan perubahan itu di PR bila terjadi.

### Dipakai dari kode yang ada
- `internal/session.Manager`: `Create(SessionSpec)`, `Write`, `Kill`, `List`, `Get`.
- `internal/provider`: provider default aplikasi dan flag auto-approve di `launch.go` (`claudeSkipPermissionsFlag`, `codexBypassFlag`; sudah diterapkan otomatis pada peluncuran agent, periksa bahwa sesi master mendapatkannya).
- `internal/mention`: antrean per target, `status.OnIdle`, penamaan sesi.
- `internal/ipc`: `Server`, pola modul di `cmd/shikigamid/modules.go` (`RegisterModule`, `Deps.OnReady`) dan contoh `wire_mention.go`.
- Frontend: `lib/registry.ts` (`registerPanel`, `registerTabType`), `lib/ipc.ts` (`call`, `onEvent`), pola `modules/mention/` (client, store, `asList`, `errorText`) dan `modules/sidebar-left/` untuk letak panel sidebar.

### Frontend (`frontend/src/modules/thread/`)
- `types.ts`: cermin `Thread`, `Participant`, `ThreadMessage`, konstanta nama IPC/event.
- Slot render di komponen tab thread (lihat fitur thread-ui) agar subfolder `participants/`, `activity/`, `close/`, `recovery/` bisa memasang komponen tanpa mengedit file lama: store kecil `slots.ts` dengan `registerThreadSlot({slot: 'header-actions'|'above-input'|'message-extra'|'side-panel', component, order?})` dan komponen `<ThreadSlot name=... {thread}/>` yang merender semuanya.

## Fitur

### thread-foundation: Kontrak dan penyimpanan thread
Kerjakan lebih dulu; fitur lain bergantung pada tipe dan konstanta di sini.

Deskripsi:
- `types.go`: definisikan tipe, konstanta state/role/kind, konstanta nama metode dan event IPC (`MethodCreate = "thread.create"`, dst., `EventUpdated = "thread.updated"`, `EventMessage = "thread.message"`), plus tipe request/response IPC (`CreateParams`, `ListParams`, `GetParams`, `MessagesParams`, `SendParams`). Tambahkan komentar yang menyatakan bentuk tipe stabil dan hanya boleh ditambah field opsional.
- `store.go`: implementasi penyimpanan seperti pada bagian Kontrak. Operasi pada thread `closed` untuk membaca selalu diizinkan; penulisan pesan oleh manager ditolak setelah closed (penegakannya di manager).
- Fungsi bantu untuk path (`ThreadDir`, `MasterDir`) yang akan dipakai tiket worktree: dokumentasikan konvensi `DataDir/threads/<id>/<projectSlug>` dan branch `thread/<id>-<slug>` di komentar paket (tidak diimplementasi di sini).

Acceptance criteria:
- [ ] `internal/thread` ter-compile; `go build ./...` lulus.
- [ ] `Thread`, `Participant`, `ThreadMessage` sesuai bentuk dan tag json pada Kontrak.
- [ ] Konstanta metode dan event IPC didefinisikan sekali di `types.go`.
- [ ] `Store.Create` membuat `threads/<id>/thread.json`, `messages.jsonl` (boleh kosong), dan direktori `master`.
- [ ] Penulisan `thread.json` atomik; `AppendMessage` append-only dan aman terhadap akses bersamaan.
- [ ] `List(workspaceID)` hanya mengembalikan thread workspace itu, termasuk yang `closed`; `Messages(id, sinceTs)` memfilter dengan benar dan melewati baris rusak.
- [ ] Pemuatan ulang store dari disk (proses baru) mengembalikan thread dan pesan yang sama.

ownedPaths:
- `internal/thread/types.go`
- `internal/thread/store.go`

Paralel: tidak, harus selesai lebih dulu (kontrak). Dua fitur lain bisa mulai setelah `types.go` ada, dan dapat memakai stub store sampai `store.go` selesai.

### thread-master-daemon: Manager thread, spawn master, dan IPC
Deskripsi:
- `manager.go`: `Manager` berisi store, `*session.Manager`, pembungkus antrean thread (mention queue), publisher event, hook (OnStart/OnMessage/OnParticipantAdded/OnClose), dan `ToolRegistry` kerangka. `Create(workspaceID, goal, title)`:
  1. validasi (goal tidak kosong, workspace dikenal lewat daftar project workspace yang diambil seperti `workspaceDirectory` di `wire_mention.go`, atau error invalid),
  2. buat thread di store (state `active`),
  3. panggil `SpawnMaster`,
  4. catat pesan sistem ("master dibuat"), publish `thread.updated`, kembalikan Thread.
  Bila spawn master gagal, thread tetap dibuat dengan pesan sistem berisi galat yang jelas dan UI menampilkannya; jangan panic.
- `master.go`: `SpawnMaster(thread)` memakai `session.Manager.Create` dengan provider default aplikasi (ambil dari konfigurasi/registri provider yang sudah ada; bila tidak ada default, pakai provider agent pertama yang tersedia), `Cwd = DataDir/threads/<id>/master`, `ProjectID` kosong, `WorkspaceID` thread. Flag auto-approve diterapkan oleh `provider/launch.go`; verifikasi bahwa jalur peluncuran yang dipakai memang menambahkannya, dan bila tidak, lewatkan lewat jalur yang sama dengan agent biasa (jangan menduplikasi flag). Setelah sesi hidup, simpan `Participant{SessionID, ProjectID:"", Role:"master", Name}` (nama memakai penamaan mention yang ada, mis. "master"), kirim prompt awal saat sesi siap (idle) lewat antrean thread, dan catat ke log sebagai pesan sistem ringkas (bukan seluruh prompt).
- Penanda "milik thread": `session.Session` saat ini tidak punya field metadata dan `internal/session` bukan milik tiket ini. Karena itu penanda diwujudkan di sisi thread: seluruh `Participant.SessionID` adalah sesi milik thread, `Manager.OwnedSessions()` mengembalikan himpunannya, dan diekspos lewat IPC `thread.ownedSessions` serta event `thread.updated`. Bila saat implementasi ternyata ada mekanisme metadata sesi yang sudah ada, pakai itu sebagai tambahan (key `threadId`).
- `prompt.go`: `BuildMasterPrompt(thread, projects)` dalam bahasa Indonesia: peran master (merutekan, memutuskan project yang dilibatkan, tidak menutup thread sendiri), goal verbatim, daftar project workspace (id, nama, path), aturan umum (pesan user datang lewat thread, master bekerja di direktori netral dan tidak mengedit project), dan tool thread yang dijelaskan sebagai "segera tersedia" (kalimat ini dapat dikosongkan oleh tiket thread-agents lewat fungsi/variabel yang mudah diganti, mis. `ToolsSection func() string`). Prompt singkat tapi lengkap; tanpa jargon implementasi.
- `ipc.go`: daftarkan `thread.create`, `thread.list`, `thread.get`, `thread.messages`, `thread.send`, `thread.ownedSessions` lewat `internal/ipc.Server`, plus `RegisterMethod` untuk ekstensi. `thread.send`: validasi thread `active`, catat `ThreadMessage{from:"user", to:<master sessionId>, kind:"message"}`, publish `thread.message`, lalu teruskan ke master lewat antrean mention (kirim saat idle, tanpa konfirmasi); kegagalan kirim dicatat sebagai pesan sistem. Panggil hook `OnMessage` setelah pesan tercatat.
- Penerbitan event: `thread.updated` (Thread penuh) setiap perubahan thread; `thread.message` setiap pesan baru.
- Hook `OnStart` dipanggil saat modul siap (`d.OnReady`) agar tiket recovery bisa mengaitkan diri.

Acceptance criteria:
- [ ] `thread.create` membuat thread, men-spawn sesi master di `DataDir/threads/<id>/master` dengan `ProjectID` kosong, provider default, dan auto-approve; sesi muncul di `session.Manager.List`.
- [ ] Prompt awal berisi goal, daftar project workspace, dan penjelasan peran master, terkirim ke master saat idle dan tidak mengganggu sesi lain.
- [ ] Master tercatat di `participants` dengan role `master`; `thread.ownedSessions` mengembalikan sessionId-nya.
- [ ] `thread.send` dari user tercatat di log, di-publish sebagai `thread.message`, dan diteruskan ke master saat idle tanpa konfirmasi; mode mention global tidak berubah dan mention biasa berperilaku sama seperti sebelumnya.
- [ ] `thread.list`, `thread.get`, `thread.messages` mengembalikan data sesuai kontrak, termasuk `sinceTs`.
- [ ] `thread.send` ke thread `closed` ditolak dengan error jelas.
- [ ] Kegagalan spawn master tidak membuat daemon crash dan terlihat di aliran sebagai pesan sistem.
- [ ] Hook `OnStart/OnMessage/OnParticipantAdded/OnClose`, `ToolRegistry`, dan `RegisterMethod` tersedia dan terdokumentasi di komentar.
- [ ] Sesi interaktif milik user tidak pernah ditulis atau di-kill oleh kode thread.
- [ ] `go build ./...` lulus.

ownedPaths:
- `internal/thread/manager.go`
- `internal/thread/master.go`
- `internal/thread/prompt.go`
- `internal/thread/ipc.go`
- `cmd/shikigamid/wire_thread.go`

Paralel: bisa dikerjakan paralel dengan thread-ui setelah `types.go` (thread-foundation) tersedia; bergantung pada `store.go` untuk integrasi akhir.

### thread-ui: UI daftar thread, dialog baru, dan tab chat
Deskripsi:
- `frontend/src/modules/thread/index.ts` (satu-satunya titik registrasi): `registerPanel({id:'thread-sidebar', area:'sidebar', component: ThreadSection, order: <kecil agar di atas daftar sesi>})` dan `registerTabType({type:'thread', component: ThreadTab})`. Perluas union `TabTypeSpec.type` di `lib/registry.ts` menjadi `'session' | 'file' | 'thread'` (perubahan minimal satu baris). Modul dimuat otomatis lewat `import.meta.glob('./modules/*/index.ts')` di `main.ts`, jadi tidak perlu mengedit `main.ts`; pastikan urutan `order` panel membuat bagian Thread tampil di atas daftar sesi (periksa bagaimana `sidebar-left` merender panel area `sidebar` dan sesuaikan `order` saja; bila panel sidebar tidak dirender di posisi atas oleh komponen yang ada, tambahkan penyesuaian paling kecil dan sebutkan di Wiring).
- File di folder modul (tetap hanya satu `index.ts` yang mendaftar): `types.ts`, `thread-client.ts` (pembungkus `call`: create/list/get/messages/send/ownedSessions), `thread-store.ts` (store Svelte: daftar thread per workspace aktif, pesan per thread, subscribe `thread.updated` dan `thread.message` lewat `onEvent`, muat awal lewat `thread.list` dan `thread.messages`, dedupe pesan by id, urut waktu), `slots.ts` + `ThreadSlot.svelte` (lihat Kontrak), `ThreadSection.svelte`, `NewThreadDialog.svelte`, `ThreadTab.svelte`, `ThreadHeader.svelte`, `MessageList.svelte`, `MessageItem.svelte`, `ThreadInput.svelte`.
- `ThreadSection.svelte`: judul "Thread", tombol "Thread baru", daftar thread workspace aktif (judul, indikator aktif/idle; thread `closed` ditampilkan redup). Klik membuka/fokus tab thread (gunakan mekanisme tab yang ada di `modules/tabs`; ikuti cara tab sesi/file dibuka, termasuk id tab stabil berbasis thread id agar tidak duplikat).
- `NewThreadDialog.svelte`: textarea goal bebas, tombol Buat/Batal, validasi goal tidak kosong, memanggil `thread.create` dengan workspace aktif (ambil dari store workspace yang ada), menampilkan galat lewat `errorText`, lalu menutup dialog dan membuka tab thread baru.
- `ThreadTab.svelte`: header (judul, status aktif/closed; slot `header-actions` untuk tombol dari tiket lain), `MessageList` aliran satu kolom dengan gulir otomatis ke bawah saat pesan baru kecuali user sedang menggulir ke atas, slot `above-input` dan `side-panel`, `ThreadInput` (Enter kirim, Shift+Enter baris baru, dinonaktifkan bila thread closed). `MessageItem`: avatar berwarna deterministik per pengirim (hash sessionId/nama ke palet), label "pengirim -> penerima" memakai nama peserta (fallback "user", "master"), waktu, teks (pertahankan baris baru, teks polos tanpa HTML mentah), gaya berbeda untuk `kind=system`; slot `message-extra` per pesan. Pesan `kind=activity` dirender minimal sebagai baris sistem sampai tiket thread-activity memasang tampilannya lewat slot.
- Gunakan token tema dan komponen/gaya yang sudah dipakai modul lain (periksa `modules/mention` dan `sidebar-left`); mendukung dark/light yang sama dengan aplikasi.

Acceptance criteria:
- [ ] Bagian "Thread" muncul di sidebar kiri di atas daftar sesi dengan tombol "Thread baru" dan daftar thread workspace aktif.
- [ ] Dialog membuat thread lewat `thread.create`; setelah berhasil, tab thread terbuka dan thread muncul di daftar tanpa muat ulang.
- [ ] Tab thread menampilkan header, aliran pesan (avatar berwarna, "pengirim -> penerima", pesan sistem), dan pesan baru muncul real-time lewat event `thread.message`.
- [ ] Kolom input mengirim lewat `thread.send`; pesan user muncul di aliran; input nonaktif untuk thread closed.
- [ ] Membuka ulang aplikasi/tab memuat thread dan riwayat pesan dari daemon.
- [ ] Union `TabTypeSpec.type` memuat `'thread'`; hanya `index.ts` yang memanggil `registerPanel`/`registerTabType` untuk modul ini.
- [ ] Mekanisme slot (`registerThreadSlot` + `ThreadSlot`) tersedia dan terdokumentasi di komentar untuk tiket lain.
- [ ] Galat IPC (daemon tidak tersedia, goal kosong) tampil sebagai pesan yang bisa dibaca, bukan crash.
- [ ] Build frontend lulus (termasuk type-check bila ada skripnya).

ownedPaths:
- `frontend/src/modules/thread/`
- `frontend/src/lib/registry.ts`

Paralel: bisa dikerjakan paralel dengan thread-master-daemon; kontrak IPC dan tipe diambil dari `types.go` (thread-foundation) dan dicerminkan di `types.ts`. Pengujian end-to-end baru bermakna setelah kedua fitur selesai.

## Path yang dimiliki
Hanya path berikut yang boleh disentuh oleh tiket ini:
- `internal/thread/types.go`
- `internal/thread/store.go`
- `internal/thread/manager.go`
- `internal/thread/master.go`
- `internal/thread/prompt.go`
- `internal/thread/ipc.go`
- `cmd/shikigamid/wire_thread.go`
- `frontend/src/modules/thread/`
- `frontend/src/lib/registry.ts` (hanya perluasan union tipe tab dengan `'thread'`)

Pengecualian sempit (hanya bila tidak ada jalan lain, dan harus disebut eksplisit di hasil kerja): tambahan aditif kecil yang tidak mengubah perilaku default pada `internal/mention` untuk pengiriman tanpa konfirmasi, serta penyesuaian urutan render panel sidebar. Tidak ada berkas lain yang disentuh, termasuk file milik tiket thread-02 sampai thread-05 (`worktree.go`, `mcp*.go`, `spawn.go`, `router.go`, `antiloop.go`, `launch.go`, `activity.go`, `close*.go`, `recovery.go`, `orphans.go` dan subfolder frontend `participants/`, `activity/`, `close/`, `recovery/`).

## Wiring
Langkah penutup tiket ini, dikerjakan setelah ketiga fitur selesai, dan tidak bergantung pada tiket lain:
1. `cmd/shikigamid/wire_thread.go`: `func init() { RegisterModule("thread", func(srv *ipc.Server, d *Deps) { d.OnReady(func() { wireThread(srv, d) }) }) }`. `wireThread` membuat store di `DataDir/threads`, `Manager` (dengan `session.Manager`, antrean mention, provider default, dan direktori workspace/project seperti pola `wire_mention.go`), mendaftarkan metode IPC `thread.*`, memasang publisher event ke server, lalu memanggil hook `OnStart`. Ikuti pola modul yang ada di `modules.go` dan `Deps` (ambil dependensi yang sama dengan `wire_mention.go`/`wire_session.go`); jangan mengubah `main.go`/`modules.go` bila pendaftaran lewat `init()` sudah cukup.
2. Frontend: `index.ts` modul thread otomatis ter-import lewat glob di `main.ts`; verifikasi bagian "Thread" tampil di sidebar kiri dan tipe tab `thread` terdaftar (union di `lib/registry.ts` sudah diperluas).
3. Verifikasi manual alur penuh di aplikasi: buat thread dengan goal, master muncul sebagai sesi (terminal) dengan prompt awal, kirim pesan dari tab thread dan lihat terkirim ke master saat idle, tutup dan buka ulang aplikasi lalu thread dan riwayatnya masih ada.
4. Pastikan fitur lain tidak berubah: sesi biasa, mention (mode `confirm_agent`), grup tab, dan panel lain tetap berfungsi.

## Guard
- `go build ./...` berhasil.
- Build frontend berhasil (`make build` atau perintah build frontend proyek, termasuk type-check bila ada).
- Aplikasi berjalan dan alur pada "Hasil yang bisa dijalankan" dapat diperagakan.
- Tanpa unit test (sesuai keputusan cakupan spec).
