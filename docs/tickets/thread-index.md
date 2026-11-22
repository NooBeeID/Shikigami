# Thread: indeks tiket

Sumber: `docs/superpowers/specs/2026-11-22-thread-design.md`

## Ringkasan tiket

| Tiket | Judul | Wave | depends_on | Jumlah fitur |
|---|---|---|---|---|
| thread-ticket-01-thread-core | Thread inti: entitas, penyimpanan, master, dan tab chat | 1 | - | 3 |
| thread-ticket-02-thread-agents | Tool MCP thread, worktree, dan peserta peer | 2 | thread-ticket-01-thread-core | 4 |
| thread-ticket-03-thread-activity | Kartu aktivitas agent di aliran thread | 3 | thread-ticket-02-thread-agents | 2 |
| thread-ticket-04-thread-close | Usul tutup, banner, Close dengan konfirmasi, dan cleanup penuh | 4 | thread-ticket-03-thread-activity | 2 |
| thread-ticket-05-thread-recovery | Pemulihan setelah restart daemon dan pembersihan yatim | 5 | thread-ticket-04-thread-close | 2 |

Total: 5 tiket, 13 fitur.

Fitur per tiket:
- thread-ticket-01-thread-core: thread-foundation, thread-master-daemon, thread-ui
- thread-ticket-02-thread-agents: thread-worktree, thread-mcp-server, thread-spawn-router, thread-participants-ui
- thread-ticket-03-thread-activity: thread-activity-daemon, thread-activity-ui
- thread-ticket-04-thread-close: thread-close-daemon, thread-close-ui
- thread-ticket-05-thread-recovery: thread-lazy-respawn, thread-orphan-cleanup

## Urutan eksekusi

Setiap wave bergantung pada wave sebelumnya, sehingga tidak ada dua tiket yang bisa berjalan paralel.

1. Wave 1: thread-ticket-01-thread-core
2. Wave 2: thread-ticket-02-thread-agents (setelah wave 1)
3. Wave 3: thread-ticket-03-thread-activity (setelah wave 2)
4. Wave 4: thread-ticket-04-thread-close (setelah wave 3)
5. Wave 5: thread-ticket-05-thread-recovery (setelah wave 4)

## Kontrak bersama

Kontrak ini berlaku untuk semua tiket. Tiket berikutnya hanya menambah file dan tidak mengubah bentuk yang sudah dibuat.

1. Paket Go `internal/thread` (module `github.com/NooBeeID/shikigami`). Tipe di `types.go` dibuat oleh thread-core dan tidak diubah bentuknya setelahnya (hanya boleh ditambah field opsional):
   - `Thread{id, workspaceId, title, goal, state(active|closed), createdAt, closedAt, participants[], readyToClose}`
   - `Participant{sessionId, projectId (kosong untuk master), role(master|peer), name}`
   - `ThreadMessage{id, threadId, from(user|sessionId), to(sessionId|user|all), text, ts, kind(message|system|activity)}`
2. Penyimpanan:
   - Thread: `DataDir/threads/<threadId>/thread.json`
   - Log pesan: append-only jsonl
   - Direktori master: `DataDir/threads/<threadId>/master`
   - Worktree peserta: `DataDir/threads/<threadId>/<projectSlug>`
   - Branch: `thread/<threadId>-<slug>`
   - Thread yang sudah closed tetap bisa dibaca.
3. IPC lewat `internal/ipc`. Modul daemon didaftarkan lewat `RegisterModule('thread')` di `cmd/shikigamid/wire_thread.go` dengan `d.OnReady`.
   - Method: `thread.create{workspaceId, goal, title?}`, `thread.list{workspaceId}`, `thread.get{id}`, `thread.messages{id, sinceTs?}`, `thread.send{id, text}` (user ke master).
   - Thread-ticket-04 menambah `thread.closePreview{id}` dan `thread.close{id}`.
   - Event: `thread.updated` (Thread penuh) dan `thread.message` (ThreadMessage).
   - Nama konstanta didefinisikan di `internal/thread/types.go` dan dicerminkan di `frontend/src/modules/thread/types.ts`.
4. Tool MCP `shikigami-thread` (server loopback, token per sesi). Hanya dipasang ke peserta lewat `provider.RegisterLaunchContribution`, dan pemanggil dibatasi ke thread-nya sendiri.
   - Semua peserta: `thread_participants()`, `thread_send(to, text)`, `thread_read(last_n)`.
   - Master saja: `thread_spawn(project, brief)`, `thread_propose_close(summary)`.
   - Registri tool ada di `mcp_tools.go` (dibuat thread-agents). Thread-close menambah `thread_propose_close` lewat registri itu tanpa mengubah struktur file.
5. Titik ekstensi di `internal/thread` yang disediakan tiket awal:
   - Manager menyediakan hook `OnMessage`, `OnParticipantAdded`, `OnClose`, `OnStart`, serta registri tool MCP.
   - `ipc.go` menyediakan `RegisterMethod` untuk method tambahan.
6. Pesan thread (`thread_send` dan input user) dikirim lewat antrean `internal/mention` (`status.OnIdle`) tanpa konfirmasi. Antrean ini terpisah dari mode mention global, dan mention biasa tidak berubah.
   - Agent thread selalu auto-approve (flag di `provider/launch.go`).
   - Cwd selalu di worktree (peserta) atau direktori netral (master).
   - Project non-git ditolak di `thread_spawn`.
7. Sesi agent thread ditandai milik thread lewat metadata sesi (`threadId`), agar disaring dari daftar sesi interaktif. Sesi interaktif milik user tidak pernah dipakai atau disentuh.
8. Frontend:
   - Semua kode thread ada di `frontend/src/modules/thread/`, dengan satu `index.ts` yang mendaftar lewat `lib/registry.ts`.
   - `registerPanel` untuk area sidebar bagian Thread.
   - `registerTabType('thread')`. Union tipe diperluas oleh thread-core.
   - Subfolder per tiket (`participants/`, `activity/`, `close/`, `recovery/`) dipasang ke tab thread lewat slot render yang disediakan komponen tab thread dari thread-core.
9. Cakupan:
   - Fase di luar spec (merge otomatis, lintas workspace, router berbasis aturan, polling) tidak dibuat.
   - Tanpa unit test. Guard hanya build: `go build ./...` dan build frontend.
   - `.superpowers/` sudah ada di `.gitignore` (keputusan 11 spec terpenuhi), jadi tidak perlu tiket.
