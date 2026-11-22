---
id: thread-ticket-02-thread-agents
title: Tool MCP thread, worktree, dan peserta peer
spec: docs/superpowers/specs/2026-11-22-thread-design.md
status: implemented
wave: 2
depends_on: [thread-ticket-01-thread-core]
---
# Tool MCP thread, worktree, dan peserta peer

## Tujuan
Tiket 01 memberi thread satu master yang bisa diajak bicara user, tetapi master belum bisa melibatkan project lain. Epic ini memberi master kemampuan itu lewat tool MCP `shikigami-thread`: men-spawn agent peserta di project workspace, tiap agent bekerja di git worktree dan branch milik thread (bukan working tree utama project) dalam mode auto-approve, dan para peserta saling kirim pesan langsung. Semua pesan tercatat di aliran chat thread, dan pembatas anti-loop mencegah dua agent saling balas tanpa henti. UI menampilkan panel peserta dan menyaring sesi agent thread dari daftar sesi interaktif.

## Hasil yang bisa dijalankan
Di atas tiket 01 (thread, master, tab chat):
- Master thread kini punya tool `thread_spawn`, `thread_participants`, `thread_send`, `thread_read`. Meminta master "libatkan project X" membuat worktree `<DataDir>/threads/<threadId>/<projectSlug>` dengan branch `thread/<threadId>-<slug>` dari HEAD project, lalu sesi agent auto-approve baru berjalan dengan cwd di worktree itu.
- Di aliran chat muncul pesan sistem "master men-spawn project-x". Peserta menerima brief awal dan aturan thread.
- Master dan peserta saling kirim pesan lewat `thread_send`; pesan terkirim otomatis saat target idle (tanpa konfirmasi) dan tampil penuh di aliran sebagai `pengirim -> penerima`.
- `thread_spawn` untuk project non-git ditolak dengan error jelas ke master; untuk project di luar workspace thread juga ditolak. Spawn project yang sudah menjadi peserta memakai ulang agent yang ada.
- Bila pesan beruntun antar agent mencapai batas tanpa pesan user/master, thread dijeda dan muncul pesan sistem meminta user menengahi; pesan user melanjutkan.
- Panel peserta di kanan tab thread: nama, status kerja/idle, klik membuka terminal sesi agent di tab baru.
- Sesi agent thread tidak muncul di daftar sesi interaktif sidebar kiri dan ditandai milik thread.
- `go build ./...` dan build frontend lulus; aplikasi jalan.

## Batasan
In scope:
- Fungsi git worktree/branch (buat, inspeksi, hapus) lewat `os/exec` memakai binary `git`.
- Server MCP `shikigami-thread`, registri tool, token per sesi, `LaunchContribution` untuk peserta (master termasuk).
- Spawn peserta, perutean pesan lewat antrean mention, anti-loop, pesan sistem.
- Panel peserta, penyaringan sesi thread di sidebar kiri.

Out of scope:
- `thread_propose_close`, banner, dialog Close, dan cleanup penuh (tiket 04; epic ini hanya menyediakan fungsi inspeksi/remove worktree).
- Kartu aktivitas (tiket 03), pemulihan setelah restart dan pembersihan yatim (tiket 05).
- Merge/push/PR otomatis, lintas workspace, router berbasis aturan, polling.
- Mengubah perilaku mention biasa atau mode global `confirm_agent`; memakai atau menyentuh sesi interaktif milik user.
- Mengubah bentuk tipe di `types.go`; hanya boleh menambah field opsional bila mutlak perlu (diedit lewat tiket 01 bukan di sini: jika perlu, gunakan struct baru di file milik epic ini).
- Unit test (guard hanya build); dependency baru.

## Kontrak
Dipakai dari tiket 01 (jangan diubah bentuknya):
- Paket `internal/thread` (module `github.com/NooBeeID/shikigami`). Tipe: `Thread{id,workspaceId,title,goal,state,createdAt,closedAt,participants[],readyToClose}`, `Participant{sessionId,projectId,role(master|peer),name}`, `ThreadMessage{id,threadId,from,to,text,ts,kind(message|system|activity)}`. Konstanta nama IPC/event di `types.go`.
- Penyimpanan: `DataDir/threads/<threadId>/thread.json`, log pesan jsonl append-only, direktori master `.../master`. Worktree peserta di `.../<projectSlug>`.
- Manager (tiket 01) menyediakan hook `OnMessage`, `OnParticipantAdded`, `OnClose`, `OnStart` dan registri tool MCP; `ipc.go` menyediakan `RegisterMethod`. Sebelum menulis kode, baca `manager.go`, `master.go`, `prompt.go`, `ipc.go` dan pakai API yang ada (menambah peserta, menulis pesan ke log, publish `thread.updated`/`thread.message`). Bila nama hook di kode berbeda dari ini, ikuti kode dan catat di wiring.
- `provider.RegisterLaunchContribution(fn)`, `LaunchContext{Session, Provider, Env, Args, MCPServers}`; provider Claude/Codex sudah selalu menambah flag auto-approve (`--dangerously-skip-permissions`, `--dangerously-bypass-approvals-and-sandbox` di `internal/provider/launch.go`), jadi agent thread otomatis auto-approve; tidak perlu mengubah provider.
- Pola `internal/mention`: `mcp.go` (server loopback, `bearerToken`, JSON-RPC), `install.go` (`tokenManager`: `issue`/`lookup`/`release`, `registerLaunchContribution`, `hookSessionExit`), `queue.go` (`Queue.Enqueue(ctx, from, to, text, source)`, pengiriman saat `status.OnIdle`). Baca ketiganya sebelum merancang; jangan mengubah perilaku mention biasa. Bila antrean mention butuh jalur "tanpa konfirmasi" yang tidak ada, tambahkan secara aditif minimal (satu opsi/sumber) dan sebut eksplisit di wiring; `source` bertanda thread tidak boleh terpengaruh `confirm_agent`.
- `session.Manager` (Create/Write/Kill/List), `SessionSpec{WorkspaceID, ProjectID, Cwd, ProviderID, ...}`; `internal/status` untuk idle/busy; penamaan peserta memakai penamaan mention yang sudah ada.
- Frontend: `lib/registry.ts`, `lib/ipc.ts`, komponen tab thread dari tiket 01 menyediakan slot/titik render untuk subfolder `participants/`; mekanisme tab sesi yang ada untuk membuka terminal.

Disediakan epic ini:
```go
// worktree.go
type WorktreeInfo struct{ Path, Branch, BaseBranch, BaseCommit, ProjectPath string }
type WorktreeStatus struct {
    Dirty       []string // perubahan belum di-commit (git status --porcelain)
    Unmerged    []string // subject commit di branch thread yang belum ada di branch dasar
    BaseBranch  string
}
var ErrNotGitRepo error // project bukan repo git
func CreateWorktree(projectPath, dataDir, threadID, projectSlug string) (WorktreeInfo, error)
func InspectWorktree(info WorktreeInfo) (WorktreeStatus, error)
func RemoveWorktree(info WorktreeInfo) error // worktree remove --force + branch -D + prune; idempoten

// mcp_tools.go
type Caller struct{ ThreadID, SessionID string; Master bool }
type ToolFunc func(ctx context.Context, c Caller, args json.RawMessage) (any, error)
type ToolDef struct{ Name, Description string; InputSchema map[string]any; MasterOnly bool; Handler ToolFunc }
func RegisterTool(d ToolDef) // dipakai tiket 04 untuk thread_propose_close; tanpa merombak file

// antiloop.go
const MaxAgentChain = 8 // batas pesan beruntun antar agent tanpa pesan user/master
```
Tool MCP (server `shikigami-thread`): `thread_participants()`, `thread_send(to,text)`, `thread_read(last_n)` untuk semua peserta; `thread_spawn(project, brief)` hanya master.

## Fitur

### thread-worktree: Manajemen git worktree dan branch thread
Deskripsi: `internal/thread/worktree.go`. `CreateWorktree` menjalankan `git -C <projectPath> rev-parse --is-inside-work-tree`; gagal berarti `ErrNotGitRepo` dengan pesan jelas ("project bukan repo git; thread tidak bisa mengisolasinya") tanpa fallback bekerja di tempat. Catat branch dasar (`rev-parse --abbrev-ref HEAD`, bila detached simpan commit) dan commit HEAD. Buat `<dataDir>/threads/<threadId>/<projectSlug>` dengan `git worktree add -b thread/<threadId>-<slug> <path> HEAD`. Bila branch sudah ada (bentrok), coba ulang dengan sufiks `-2`, `-3`, ... (batas wajar). `projectSlug`/`slug` diturunkan dari nama project (huruf kecil, `[a-z0-9-]`). `InspectWorktree`: `git status --porcelain` di worktree untuk `Dirty`; `git log <BaseCommit atau BaseBranch>..HEAD --format=%h %s` untuk `Unmerged` (commit yang belum ada di branch dasar). `RemoveWorktree`: `git worktree remove --force`, `git branch -D`, `git worktree prune`; boleh dipanggil ulang tanpa error bila sudah hilang. Semua lewat `os/exec` dengan konteks dan timeout; tidak ada dependency baru. Info worktree dikembalikan ke pemanggil agar spawn menyimpannya (lihat thread-spawn-router).
Acceptance criteria:
- [ ] Repo git: worktree dan branch `thread/<threadId>-<slug>` terbentuk dari HEAD, di bawah `DataDir/threads/<threadId>/<projectSlug>`.
- [ ] Bentrok nama branch ditangani dengan sufiks, bukan gagal.
- [ ] Project non-git mengembalikan `ErrNotGitRepo` dan tidak membuat apa pun.
- [ ] `InspectWorktree` melaporkan file belum di-commit dan commit belum ada di branch dasar.
- [ ] `RemoveWorktree` menghapus direktori worktree dan branch; aman dipanggil dua kali.
- [ ] Working tree utama project tidak berubah (selain metadata `.git/worktrees`).
- [ ] `go build ./...` lulus.
ownedPaths: `internal/thread/worktree.go`
Paralel: independen; dapat dikerjakan paralel dengan fitur lain. thread-spawn-router memanggil API ini sehingga cukup bersepakat pada signature di atas.

### thread-mcp-server: Server MCP shikigami-thread dan pemasangan lewat LaunchContribution
Deskripsi: `internal/thread/mcp.go`, `mcp_tools.go`, `launch.go`. Server MCP loopback sendiri (mengikuti pola `internal/mention/mcp.go`, transport sama, URL ditulis ke berkas run bila pola mention melakukannya) bernama `shikigami-thread`; builder boleh memilih memperluas server mention bila lebih sederhana, selama nama server di konfigurasi MCP sesi tetap `shikigami-thread`. Token per sesi mengikuti pola `tokenManager` mention: diterbitkan saat peluncuran sesi peserta, di-resolve menjadi `Caller{ThreadID, SessionID, Master}` lewat registri peserta Manager, dilepas saat sesi exit. `launch.go` mendaftarkan `provider.RegisterLaunchContribution` yang memasang `MCPServers["shikigami-thread"]` (URL + header bearer, format sama dengan yang dipakai contribution mention untuk Claude/Codex) hanya bila sesi diluncurkan sebagai peserta thread (ditandai metadata/penanda spawn dari Manager, bukan sesi interaktif biasa; provider `shell` dilewati). Master dari tiket 01 kini ikut mendapat MCP ini: tiket ini menyambungkan peluncuran master ke penanda yang sama lewat hook yang disediakan tiket 01. `mcp_tools.go` berisi registri `RegisterTool`, daftar tool dengan JSON schema (`tools/list` hanya menampilkan tool sesuai peran: `MasterOnly` disembunyikan dari peer dan ditolak bila dipanggil), serta handler tool umum: `thread_participants()` mengembalikan peserta thread pemanggil (nama, role, project, status kerja/idle), `thread_send(to,text)` memvalidasi `to` hanya peserta thread yang sama (atau `all`/`user` sesuai aturan) lalu memanggil router, `thread_read(last_n)` mengembalikan N pesan terakhir log thread pemanggil (batas atas wajar). Handler `thread_spawn` didaftarkan oleh thread-spawn-router lewat `RegisterTool`; `thread_propose_close` ditambah tiket 04. Pemanggil tidak pernah bisa menyebut thread lain: thread selalu dari token.
Acceptance criteria:
- [ ] Sesi peserta (master dan peer) Claude/Codex meluncur dengan MCP `shikigami-thread`; sesi interaktif biasa dan `shell` tidak.
- [ ] Token tak dikenal atau sudah dilepas ditolak (401/error JSON-RPC).
- [ ] `tools/list` untuk peer tidak memuat tool master-only; panggilan paksa ke tool master-only ditolak.
- [ ] `thread_participants`, `thread_send` (target di luar thread ditolak dengan pesan jelas), `thread_read` bekerja dan terbatas ke thread pemanggil.
- [ ] `RegisterTool` dapat dipakai dari file lain tanpa mengedit `mcp_tools.go`.
- [ ] Mention biasa tidak berubah perilakunya.
- [ ] `go build ./...` lulus.
ownedPaths: `internal/thread/mcp.go`, `internal/thread/mcp_tools.go`, `internal/thread/launch.go`
Paralel: dapat dikerjakan paralel dengan thread-worktree dan thread-participants-ui. Handler `thread_send` dan `thread_spawn` bergantung pada router/spawn lewat antarmuka kecil (mis. `Router` interface yang diisi saat wiring), sehingga tidak harus menunggu fitur itu.

### thread-spawn-router: Spawn peserta, perutean pesan, dan anti-loop
Deskripsi: `internal/thread/spawn.go`, `router.go`, `antiloop.go`.
- `spawn.go`: handler `thread_spawn(project, brief)` (didaftarkan via `RegisterTool`, master saja). Langkah: resolusi `project` (id atau nama) terhadap project workspace thread; di luar workspace -> error jelas. Bila project sudah peserta thread ini -> pakai ulang agent, kirim `brief` sebagai pesan baru (tanpa membuat worktree/sesi baru). Selain itu `CreateWorktree` (error `ErrNotGitRepo` diteruskan ke master apa adanya), lalu buat sesi lewat `session.Manager` dengan `Cwd` di worktree, `ProjectID` project, provider default aplikasi, ditandai milik thread (metadata `threadId` pada sesi, dan penanda spawn agar `launch.go` memasang MCP), auto-approve lewat flag provider. Daftarkan `Participant{role: peer}` ke Manager (memicu `OnParticipantAdded`, publish `thread.updated`), simpan info worktree di data thread/berkas samping di direktori thread agar tiket 04/05 bisa membacanya (berkas milik epic ini, mis. `worktrees.json`, ditulis atomik; tidak mengubah bentuk `Thread`). Catat pesan sistem `kind=system` "master men-spawn <project>" ke log. Kirim pesan awal ke peserta berisi brief dan aturan thread (singkat: bekerja hanya di worktree sendiri, komunikasi lewat `thread_send`, jangan meminta izin, commit pekerjaan di branch thread, laporkan ke master) lewat router. Bila gagal setelah worktree dibuat, `RemoveWorktree` dan error ke master.
- `router.go`: `Route(threadID, from, to, text)` memvalidasi peserta, mencatat `ThreadMessage{kind=message}` ke log (publish `thread.message`), lalu memasukkan ke antrean `internal/mention` untuk sesi target dengan source bertanda thread: kirim otomatis saat target idle tanpa konfirmasi, tidak terpengaruh mode global. Pesan user dari `thread.send` (tiket 01) tetap menuju master lewat jalur yang sama; ganti/sambung hook agar memakai `Route` bila tiket 01 membuat jalur sementara. Pesan ke `user` hanya dicatat. Format teks yang disuntikkan ke target memuat pengirim agar agent tahu siapa yang bicara (mis. `[thread: <nama pengirim>] ...`).
- `antiloop.go`: hitung per thread jumlah pesan beruntun antar agent (pengirim dan penerima keduanya agent, tanpa pesan dari user atau master di antaranya? Aturan: pesan dari user atau dari master mereset hitungan; pesan peer ke peer menambah). Saat hitungan mencapai `MaxAgentChain` (8): hentikan pengiriman lebih lanjut antar agent, tandai thread dijeda (state internal, bukan `Thread.state`), catat pesan sistem "Thread dijeda: pesan beruntun antar agent mencapai batas. Kirim pesan untuk melanjutkan." Pesan agent lain saat dijeda ditolak ke pengirim dengan error jelas lewat tool. Pesan user (atau master) mereset hitungan dan melanjutkan. Ketik langsung di terminal peserta tidak lewat router sehingga tidak dihitung.
Acceptance criteria:
- [ ] `thread_spawn` project git di workspace: worktree + sesi peserta auto-approve dengan cwd di worktree, peserta muncul di `Thread.participants`, pesan sistem tercatat.
- [ ] Project non-git dan project di luar workspace ditolak dengan error jelas dan tidak membuat sesi/worktree.
- [ ] Spawn kedua untuk project yang sama memakai ulang agent (tanpa sesi/worktree baru).
- [ ] Peserta menerima brief awal dan aturan thread.
- [ ] `thread_send` dan pesan user tercatat di log, terkirim otomatis saat idle tanpa konfirmasi walau mode global `confirm_agent`.
- [ ] Setelah `MaxAgentChain` pesan beruntun antar agent, thread dijeda dengan pesan sistem; pesan user melanjutkan.
- [ ] Gagal spawn tidak meninggalkan worktree yatim.
- [ ] `go build ./...` lulus.
ownedPaths: `internal/thread/spawn.go`, `internal/thread/router.go`, `internal/thread/antiloop.go`
Paralel: bergantung pada signature worktree dan registri tool dari fitur lain di epic ini (cukup bersepakat pada kontrak); implementasi dapat paralel, penggabungan akhir diuji di wiring.

### thread-participants-ui: Panel peserta dan penyaringan sesi thread
Deskripsi: `frontend/src/modules/thread/participants/`. Komponen `ParticipantsPanel.svelte` dipasang ke slot kanan tab thread yang disediakan komponen tab thread tiket 01 (daftarkan lewat titik render/slot itu, bukan dengan mengedit file tiket 01 selain wiring minimal yang disebut di bawah). Isi: daftar peserta thread (dari `thread.get`/event `thread.updated`) dengan avatar berwarna per agent, nama, role, project, dan status kerja/idle dari modul `status` yang ada (store status per sesi). Klik membuka terminal sesi agent di tab baru lewat mekanisme tab sesi yang ada (`modules/tabs`/`modules/terminal`), memakai ulang tab bila sudah terbuka. Pesan `kind=system` (mis. "master men-spawn project-x", "Thread dijeda ...") dan pesan antar agent tampil penuh di aliran: sediakan renderer/gaya untuk pesan sistem dan label `pengirim -> penerima` berwarna yang dipasang ke slot render pesan tiket 01 bila ada, atau diekspor sebagai komponen untuk dipakai wiring. Penyaringan: sesi agent thread disaring dari daftar sesi interaktif sidebar kiri dan ditandai milik thread (lencana kecil "thread"). Penyaringan memakai metadata sesi `threadId` yang diisi daemon; sediakan predikat `isThreadSession(session)` di folder ini dan titik ekstensi filter kecil di modul `sidebar-left` (mis. daftar `sessionFilters` di `lib/registry.ts` atau satu pemanggilan predikat di komponen daftar sesi). Perubahan di sidebar-left kecil dan disebut eksplisit di wiring.
Acceptance criteria:
- [ ] Panel peserta menampilkan peserta dengan status kerja/idle yang hidup mengikuti status sesi.
- [ ] Klik peserta membuka terminal sesinya di tab baru (atau memfokuskan bila sudah ada).
- [ ] Pesan sistem dan pesan antar agent tampil penuh di aliran, tidak dilipat.
- [ ] Sesi agent thread tidak muncul di daftar sesi interaktif sidebar kiri; sesi biasa tidak berubah.
- [ ] Peserta baru muncul tanpa reload saat `thread.updated` tiba.
- [ ] Build frontend lulus.
ownedPaths: `frontend/src/modules/thread/participants/`
Paralel: independen dari fitur Go; dapat dikerjakan paralel. Perlu field metadata sesi `threadId` dari thread-spawn-router (kontrak: ada di objek sesi yang dikirim daemon).

## Path yang dimiliki
Hanya ini yang boleh disentuh epic ini:
- `internal/thread/worktree.go`
- `internal/thread/mcp.go`
- `internal/thread/mcp_tools.go`
- `internal/thread/spawn.go`
- `internal/thread/router.go`
- `internal/thread/antiloop.go`
- `internal/thread/launch.go`
- `frontend/src/modules/thread/participants/`

Perubahan kecil di luar daftar hanya sebagai wiring yang disebut eksplisit di bawah.

## Wiring
Langkah penutup tiket ini, hanya lewat titik ekstensi tiket 01:
1. `cmd/shikigamid/wire_thread.go` (milik tiket 01): tambahkan satu pemanggilan di `d.OnReady`/`OnStart` ke fungsi inisialisasi epic ini (mis. `thread.InstallAgents(mgr, sessions, queue, ...)` yang didefinisikan di `launch.go`/`spawn.go`) yang: memulai server MCP `shikigami-thread`, memanggil `provider.RegisterLaunchContribution`, mendaftarkan handler `thread_spawn` dan tool umum via `RegisterTool`, memasang router sebagai pengirim pesan Manager (menggantikan jalur kirim-ke-master sementara bila ada), dan menyambungkan hook `OnParticipantAdded`/`OnMessage`. Bila nama di kode tiket 01 berbeda, ikuti kode.
2. Sambungkan peluncuran master (tiket 01) ke penanda sesi-peserta agar master ikut mendapat MCP `shikigami-thread`, dan perbarui prompt awal master (`prompt.go` milik tiket 01 menyediakan titik tambah daftar tool; bila tidak, tambahkan teks tool lewat hook prompt) agar menyebut `thread_spawn`, `thread_send`, `thread_read`, `thread_participants`.
3. Antrean mention: bila perlu, tambahan aditif minimal agar pesan bersumber thread dikirim tanpa konfirmasi (sebut di commit).
4. Frontend: daftarkan `ParticipantsPanel` ke slot kanan tab thread, dan pasang predikat penyaringan ke titik filter kecil di `frontend/src/modules/sidebar-left` (satu-satunya perubahan di luar folder `participants/`, disebut eksplisit; dan satu baris impor di `index.ts` thread bila perlu).
5. Pastikan sesi peserta diberi metadata `threadId` sehingga daftar sesi yang dikirim ke UI membawanya.
6. Jalankan aplikasi: buat thread, minta master men-spawn project git; verifikasi worktree dan branch terbentuk, peserta muncul di panel, pesan bolak-balik terlihat di aliran, sesi thread tidak tampil di sidebar kiri.

Tiket ini harus bisa dijalankan dengan hanya bergantung pada tiket 01.

## Guard
- `go build ./...`
- Build frontend (`npm run build` di `frontend/`, atau `make build` bila tersedia)
- Tanpa unit test.
