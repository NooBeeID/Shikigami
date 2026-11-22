# Thread: diskusi multi-agent lintas project

Tanggal: 2026-11-22
Status: draf menunggu persetujuan

## Maksud dan alasan

User sering punya masalah atau tujuan yang melintasi beberapa project dalam satu workspace. Contoh: issue RBAC yang melibatkan project X (konsumen) dan soulsociety (penyedia RBAC), atau setup CI/CD Jenkins yang butuh agent Infra (repo Jenkins) memahami struktur build/run project X. Hari ini user harus berpindah sesi satu per satu dan menjadi kurir pesan antar agent.

Fitur **thread**: user menulis satu goal dalam bahasa biasa. Satu agent **master** memutuskan project mana yang dilibatkan dan men-spawn agent khusus thread di project itu. Para agent berdiskusi langsung sampai goal tercapai. User memantau seluruh alur dan bisa menambah instruksi kapan saja; instruksi diteruskan master ke agent yang sesuai (termasuk "Fix issue tersebut", dan bila fix juga perlu di sisi agent lain, master meminta agent itu mengerjakannya).

Nama "thread" dipilih agar tidak bentrok dengan "group" yang sudah dipakai untuk tab grup sesi (`workspace.group.*`).

## Tujuan

- User membuat thread baru dari UI dengan satu goal teks, dalam workspace aktif.
- Master (agent LLM sungguhan, sesi Claude atau Codex) menentukan project peserta dan men-spawn agent khusus thread di project tersebut.
- Agent khusus thread hidup selama thread aktif dan dipakai ulang untuk setiap pesan user, sampai thread ditutup. Thread baru selalu men-spawn agent baru; agent dan sesi interaktif milik user tidak pernah dipakai atau disentuh.
- Peserta (termasuk master) dapat saling kirim pesan langsung di dalam thread. Pesan agent-ke-agent dalam thread dikirim otomatis tanpa konfirmasi.
- Semua pesan tercatat di log thread dan tampil sebagai satu aliran chat.
- Setiap agent thread bekerja di **git worktree terpisah milik thread itu** (satu worktree per project per thread, dengan branch sendiri), sehingga dua thread tidak pernah mengedit working tree yang sama dan working tree utama project tidak tersentuh.
- Agent thread berjalan dalam mode **auto-approve** (tanpa prompt izin), sehingga diskusi dan pekerjaan tidak macet menunggu persetujuan.
- Hanya user yang menutup thread. Master hanya dapat menandai "goal tercapai, siap ditutup" (tampil sebagai banner dengan tombol Close). Saat Close, semua agent thread dimatikan, lalu **semua worktree dan branch thread dihapus** (cleanup penuh).

## Bukan tujuan

- Master tidak boleh menutup thread sendiri.
- Tidak memakai atau menyuntik pesan ke sesi interaktif yang sudah ada milik user.
- Tidak lintas workspace; thread terikat satu workspace.
- Tidak mengubah perilaku mention biasa (mode default `confirm_agent` tetap).
- Tidak ada router berbasis aturan di daemon pengganti master, tidak ada polling.
- Bukan editor goal formal atau sistem tugas; goal adalah teks bebas.

## Anti-goal (gagal walau secara teknis jalan)

- Dua agent saling balas tanpa henti dan menghabiskan token tanpa kemajuan.
- Agent thread mengubah file di project tanpa user tahu apa yang terjadi (alur harus terlihat).
- Thread yang sudah ditutup meninggalkan proses agent yang masih hidup, atau worktree/branch yatim.
- Close menghapus pekerjaan yang belum dipindahkan keluar dari branch thread tanpa user menyadarinya (commit yang belum di-merge/push, atau perubahan yang belum di-commit).
- Agent auto-approve mengubah working tree utama project atau menyentuh file di luar worktree-nya tanpa batas yang jelas.

## Batasan

- Daemon Go (`internal/`), JSON-RPC lewat `internal/ipc`, frontend Svelte 5 + Vite + TypeScript. Tanpa dependency baru bila bisa dihindari.
- Memakai ulang: `internal/mention` (antrean per target, pengiriman saat idle lewat `status.OnIdle`, server MCP loopback dengan token per sesi, `provider.RegisterLaunchContribution`), `internal/session` (Manager: Create/Write/Kill/List), `internal/status` dan `internal/runs` (status dan aktivitas agent).
- Spawn agent hanya di project yang terdaftar pada workspace thread.
- Isolasi lewat `git worktree`: project yang bukan repo git tidak bisa diisolasi, sehingga `thread_spawn` untuk project seperti itu ditolak dengan pesan error jelas ke master (tidak ada fallback bekerja di tempat, karena agent berjalan auto-approve).
- Pesan tidak disuntik ke agent yang sedang sibuk; menunggu idle seperti mention biasa.

## Pendekatan

Dua pendekatan dipertimbangkan, keduanya dengan entitas thread di daemon (modul baru `internal/thread`, disimpan ke disk):

- **A. Hub-and-spoke:** semua pesan lewat master, peserta hanya membalas ke master. Alur mudah dipantau tetapi master jadi bottleneck dan boros token; tidak cocok untuk diskusi Infra dan Project langsung.
- **B. Peer di dalam thread (dipilih):** master men-spawn dan mengusulkan penutupan; semua peserta bisa saling kirim pesan langsung; semua pesan dicatat di log thread. Dipilih user karena skenario Jenkins membutuhkan percakapan langsung antar peserta. Risiko loop balas-membalas ditangani dengan pembatas (lihat bagian "Diserahkan ke builder").

Ditolak: router berbasis aturan tanpa master LLM (tidak bisa memutuskan siapa yang dilibatkan), dan pertukaran lewat berkas bersama yang di-polling (tidak andal, melanggar aturan tanpa polling).

## Desain

### Entitas thread (daemon)

`Thread{id, workspaceId, title, goal, state (active|closed), createdAt, closedAt, participants[], readyToClose}`.
`Participant{sessionId, projectId, role (master|peer), name}`. Nama peserta memakai penamaan mention yang sudah ada.
`ThreadMessage{id, threadId, from (user|sessionId), to (sessionId|user|all), text, ts, kind (message|system|activity)}`. Pesan `activity` adalah baris ringkas aksi agent (mis. "project-x: Edit auth/claims.go") yang diturunkan dari data aktivitas yang sudah ada.
Thread dan log disimpan di `DataDir`. Thread yang `closed` tetap bisa dibaca (read-only).

### Tool MCP thread

Server MCP loopback baru (atau ekstensi server mention; builder memilih) bernama `shikigami-thread`, dipasang hanya ke sesi yang menjadi peserta thread lewat `LaunchContribution`. Pemanggil dikenali dari token per sesi dan dibatasi pada thread-nya sendiri.
- Semua peserta: `thread_participants()`, `thread_send(to, text)` (hanya ke peserta thread yang sama), `thread_read(last_n)`.
- Hanya master: `thread_spawn(project, brief)` (men-spawn agent khusus thread di project workspace itu, dengan brief awal) dan `thread_propose_close(summary)` (menyalakan banner "siap ditutup"; tidak menutup).
- Pesan `thread_send` dan pesan user dari UI masuk antrean mention, dikirim otomatis saat target idle, dan dicatat ke log thread.

### Alur

1. User membuat thread (UI): memilih/mengisi goal. Daemon membuat thread dan men-spawn sesi master (provider default) dengan prompt awal berisi goal, daftar project workspace, dan penjelasan tool thread.
2. Master memanggil `thread_spawn` untuk project yang relevan. Daemon membuat worktree dan branch khusus thread untuk project itu (`git worktree add`, branch `thread/<threadId>-<slug>`, bercabang dari HEAD project saat spawn), lalu men-spawn agent auto-approve dengan cwd di worktree tersebut. Peserta baru menerima brief dan aturan thread. Bila project sudah menjadi peserta thread ini, agent yang ada dipakai ulang.
3. Peserta berdiskusi lewat `thread_send`. Pesan user dari kolom input thread dikirim ke master; master meneruskan sesuai kebutuhan.
4. Master memanggil `thread_propose_close` saat goal tercapai. User menekan Close (boleh kapan saja walau master belum mengusulkan). Sebelum menutup, UI menampilkan konfirmasi yang mendaftar tiap worktree beserta perubahan yang belum di-commit dan commit yang belum ada di branch dasar (belum di-merge/push); user bisa lanjut atau batal. Saat dikonfirmasi: semua sesi thread di-Kill, semua worktree dihapus, semua branch thread dihapus, thread berstatus `closed`.
5. Selama thread aktif, tidak ada agent yang dimatikan; agent peserta hidup lintas pesan user.
6. Cara hasil kerja keluar dari branch thread (merge/push/PR) adalah urusan user atau instruksi ke agent di dalam thread sebelum Close; fitur ini tidak merge otomatis.

### UI (layout 1 dikombinasikan dengan kartu aktivitas layout 5, sesuai pilihan user)

- Daftar thread di sidebar kiri (bagian "Thread" di atas daftar sesi), dengan indikator aktif/idle/siap ditutup. Tombol "Thread baru".
- Thread terbuka sebagai tab di area tengah. Kolom utama: header (judul, status, tombol "Tandai selesai"/Close), aliran chat satu kolom dengan avatar berwarna per agent (`pengirim → penerima`), pesan sistem (mis. "master men-spawn project-x"), banner hijau "goal tercapai" dengan tombol Close, dan kolom input di bawah (dikirim ke master).
- Aksi agent (baca/edit file, jalankan perintah) tampil sebagai **kartu aktivitas per agent** yang bisa dibuka-tutup di dalam aliran, berisi ringkasan seperti "project-x: 5 aksi" dan status "mengerjakan". Pesan diskusi antar agent TIDAK dilipat; tetap tampil penuh sebagai pesan biasa (penulis menolak sifat layout 5 yang melipat diskusi karena user perlu memantau percakapan). Keputusan ini usulan penulis.
- Panel peserta di kanan: nama, status (kerja/idle), klik membuka terminal sesi agent tersebut di tab baru (memakai mekanisme tab sesi yang ada).
- Sesi agent thread tidak muncul di daftar sesi biasa sebagai sesi interaktif; mereka ditandai milik thread.

## Diserahkan ke builder

- Skema penyimpanan dan nama method IPC (`thread.*`) serta event publish ke UI.
- Pembatas anti-loop (mis. batas pesan beruntun antar agent tanpa pesan user/master sebelum thread berhenti dan meminta user menengahi), angkanya ditentukan builder.
- Provider default master dan peserta, dan cara adapter provider memasang MCP `shikigami-thread` serta mengaktifkan mode auto-approve per provider (flag Claude/Codex).
- Perintah git persis untuk membuat dan membersihkan worktree/branch, penanganan bentrok nama branch, dan deteksi perubahan/commit yang belum di-merge untuk dialog konfirmasi Close.
- Bentuk prompt awal master dan peserta.
- Perilaku saat daemon restart (lihat daftar keputusan di bawah).
- Cara menurunkan baris `activity` dari modul status/runs yang ada.

## Keputusan yang diambil penulis (user dapat menolak)

1. Hanya user yang menutup thread; master hanya mengusulkan. (Disepakati dalam percakapan.)
2. Pesan di dalam thread dikirim otomatis tanpa konfirmasi, terpisah dari mode mention global. (Disepakati sebagai prinsip; mekanisme teknis usulan penulis.)
3. Saat daemon restart, thread tetap tersimpan dan terbaca, tetapi agent-nya mati. Agent dihidupkan kembali secara lazy pada pesan user berikutnya ke thread aktif (resume sesi provider bila didukung, jika tidak, spawn baru dengan log thread sebagai konteks). Ini usulan penulis; belum dibahas.
4. Master menggunakan provider default aplikasi. Usulan penulis.
5. Kartu aktivitas hanya melipat aksi agent, bukan pesan diskusi antar agent.
6. Master berjalan di direktori netral milik thread (di bawah `DataDir`, bukan di salah satu project); karena tugasnya merutekan, `projectId` peserta master boleh kosong. Worktree peserta juga ditaruh di bawah `DataDir/threads/<threadId>/`, di luar repo project, agar tidak mengotori project.
7. Input thread hanya dikirim ke master. Mengetik langsung di terminal peserta adalah aksi interaktif biasa, tidak dihitung pembatas loop, dan aksinya tetap muncul di kartu aktivitas.
8. Konfirmasi sebelum Close (daftar perubahan yang akan hilang) ditambahkan penulis karena cleanup penuh menghapus commit yang belum di-merge; user tetap bisa melanjutkan.
9. Project non-git ditolak saat `thread_spawn` (lihat Batasan).
10. Sebuah worktree yatim (mis. daemon crash saat Close) dibersihkan saat daemon start untuk thread berstatus `closed`; detailnya diserahkan ke builder.
11. Folder `.superpowers/` ditambahkan ke `.gitignore` (artefak mockup).
