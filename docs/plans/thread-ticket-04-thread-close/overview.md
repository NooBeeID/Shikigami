# Overview: thread-ticket-04-thread-close

Tiket: `docs/tickets/thread-ticket-04-thread-close.md`. Spec: `docs/superpowers/specs/2026-11-22-thread-design.md`. Dependensi: tiket 01 (`thread-core`) dan tiket 02 (`thread-agents`) SUDAH ada di kode (`internal/thread/`, `frontend/src/modules/thread/`); tiket 03 (`thread-activity`) belum tentu ada, jadi epic ini TIDAK boleh memakai simbolnya (`InstallActivity`, `UpsertMessage`, `activity/`).

Dokumen ini adalah sumber kebenaran BERSAMA untuk plan fitur. Bila beda dengan tiket, overview menang hanya pada butir berlabel "koreksi/keputusan" (2.2); selain itu tiket menang. Tanpa dependency baru, tanpa unit test; guard hanya build.

Semua nama di bawah DIBACA dari kode yang ada (`types.go`, `manager.go`, `ipc.go`, `mcp_tools.go`, `worktree.go`, `worktree_records.go`, `agents_install.go`, `prompt.go`, frontend `slots.ts`, `ThreadTab.svelte`, `ThreadHeader.svelte`, `ThreadInput.svelte`, `ThreadSection.svelte`, `thread-store.ts`, `thread-client.ts`, `participants/index.ts`). Bila kode berbeda saat implementasi, ikuti kode dan catat di "Catatan untuk wiring" plan fitur.

## 1. Ringkasan epic dan urutan eksekusi

Menyelesaikan siklus hidup thread: master hanya MENGUSULKAN tutup (`thread_propose_close` -> `readyToClose=true` + banner hijau), user menutup lewat Close dengan dialog konfirmasi yang mendaftar perubahan belum di-commit dan commit belum di-merge per worktree. Setelah konfirmasi: semua sesi mati, semua worktree dan branch thread terhapus, thread `closed` dan read-only, pesan baru ditolak.

Urutan eksekusi:

1. **Foundation** (`00-foundation.md`): field opsional `closeSummary` pada `Thread` (Go + TS), konstanta method `thread.closePreview`/`thread.close` (Go + TS), stub `close.go` (`RegisterClose` no-op) dan stub `close/index.ts` (`installClose` no-op). `go build ./...` dan build frontend lulus.
2. **Fitur paralel** (2 fitur, bagian 3): `thread-close-daemon`, `thread-close-ui`. Kontrak bagian 2 tidak berubah.
3. **Wiring** (bagian 5): satu baris `RegisterClose(m)` di `cmd/shikigamid/wire_thread.go`, satu baris `installClose()` di `frontend/src/modules/thread/index.ts`, satu sisipan `ReadyBadge` di `ThreadSection.svelte`.

## 2. Kontrak bersama

### 2.1 Paket, folder, penamaan

- Go: paket `internal/thread` (package `thread`, module `github.com/NooBeeID/shikigami`). Berkas epic ini: `close.go` (logika; stub Foundation, diganti penuh oleh `thread-close-daemon`) dan `close_ipc.go` (pendaftaran IPC/tool, baru dari `thread-close-daemon`).
- Helper tak-diekspor di dua berkas itu berawalan `close` (mis. `closeInspect`, `closeKillAll`) agar tidak bentrok dengan berkas lain di paket (konvensi paket: `manager*`, `store*`, `worktree*`, `mcp*`, `spawn*`, `router*`).
- Frontend: `frontend/src/modules/thread/close/` (Svelte 5 runes, TypeScript): `CloseBanner.svelte`, `CloseButton.svelte`, `CloseDialog.svelte`, `ClosedNotice.svelte`, `ReadyBadge.svelte`, `closeApi.ts`, `closeDialogStore.ts`, `readOnly.ts`, `index.ts` (semua milik `thread-close-ui`).
- Komentar, log, pesan galat, teks UI: bahasa Indonesia; identifier bahasa Inggris. JSON camelCase. Logger: `m.log` (tak diekspor tetapi satu paket, boleh dipakai `close.go`).

### 2.2 Keputusan desain (WAJIB diikuti)

1. **Ringkasan usul = field `Thread.closeSummary`** (opsional, `omitempty`), ditambah Foundation. Bukan pesan `system` bertanda khusus. `thread_propose_close` mengisi `ReadyToClose=true` dan `CloseSummary=<summary>` lewat `Manager.UpdateThread` (yang sudah mem-publish `thread.updated`), lalu juga `PostSystem("master mengusulkan penutupan: <summary>")` agar tercatat di aliran. Banner UI membaca `thread.closeSummary`.
2. **Penolakan pesan ke thread closed TIDAK perlu hook baru (koreksi tiket).** Sudah ada di kode: `Manager.Post`, `Manager.Send`, `Manager.SendToSession`, `Manager.AddParticipant` mengembalikan `ErrClosed` ("thread sudah ditutup", dipetakan IPC ke `ErrInvalid` dengan pesan itu), dan tool `thread_send`/`thread_participants` menolak `StateClosed`. `thread-close-daemon` TIDAK mendaftarkan hook `OnMessage` untuk menolak (hook itu terpanggil SESUDAH pencatatan, tidak bisa menolak). Daemon cukup memastikan state menjadi `closed`. Kriteria tiket "ditolak dengan error jelas" terpenuhi oleh kode itu; verifikasi di Wiring.
3. **Tahap penutupan = flag memori + sesi mati.** `close.go` menyimpan `closing map[threadID]bool` (di bawah mutex per thread). Tahap ini dipakai untuk: `ProposeClose` menolak ("thread sedang ditutup"), `Close` bersamaan menunggu (single-flight lewat mutex per thread, panggilan kedua melihat thread sudah `closed` dan mengembalikannya). Pesan user yang menyelip sebelum `state=closed` tidak berbahaya karena sesi target sudah dimatikan (antrean `mention` gagal bersih). Tidak mengedit `manager.go`.
4. **Sumber data worktree = `worktrees.json`** lewat `LoadWorktreeRecords(Store().ThreadDir(id))` (berkas `worktree_records.go`, tiket 02). Setiap `WorktreeRecord` = satu worktree (juga rekaman tanpa `SessionID`, mis. spawn setengah jalan). Peserta dicari dari `Thread.Participants` berdasarkan `SessionID`; bila tak ada, pakai `{sessionId: rec.SessionID, name: rec.ProjectName, projectId: rec.ProjectID}`. Master tidak punya rekaman worktree.
5. **Inspeksi memakai `InspectWorktree(info WorktreeInfo)` dari `worktree.go`** (SUDAH ada; kontrak tiket mengizinkan memakainya, jadi `close.go` tidak menulis helper git sendiri). Hasil `WorktreeStatus.Dirty` = baris `git status --porcelain` (termasuk untracked `??`) dan `Unmerged` = `"<hash> <subject>"`; `close.go` mem-parse jadi struct (2.3). Basis perbandingan sudah ditangani `InspectWorktree` (branch dasar bila masih ada, selain itu `BaseCommit` = semua commit sejak spawn). Dilarang menjalankan git di working tree utama project.
6. **Cleanup memakai `RemoveWorktree(info)` dari `worktree.go`** (idempoten: `worktree remove --force --force`, `RemoveAll` sisa, `worktree prune`, `branch -D`, cek kepemilikan `worktreeSafeToDelete`). Setelah `RemoveWorktree` sukses untuk satu rekaman, `DeleteWorktreeRecord(threadDir, projectID)`. Bila gagal, rekaman DIBIARKAN (tiket 05 akan membersihkan dan mengulang) dan galat masuk pesan `system`.
7. **Urutan `Close(threadID)`** (idempoten, tiap langkah tahan gagal): (a) kunci mutex thread, `Get`; bila sudah `closed` kembalikan thread apa adanya; (b) set flag closing; (c) `Kill` tiap `Participant.SessionID` (master dan peer) lewat `m.cfg.Sessions.Kill`; galat yang artinya "sesi tidak ditemukan" dianggap sudah mati (sesi dihapus dari manager oleh `Kill` pertama, jadi `Kill` ulang gagal lookup); galat lain = sesi masih hidup; (d) untuk tiap rekaman worktree: `RemoveWorktree`, lalu hapus rekaman; (e) hapus direktori master `Store().MasterDir(id)` dengan `os.RemoveAll` HANYA bila path berada di bawah `Store().ThreadDir(id)` dan bukan `ThreadDir` itu sendiri; DILARANG menghapus `ThreadDir`, `thread.json`, `messages.jsonl`, atau `worktrees.json` (berkas terakhir hanya dikosongkan lewat `DeleteWorktreeRecord` per rekaman); (f) bila SEMUA sesi mati: `UpdateThread` set `State=StateClosed`, `ClosedAt=now UTC`, `ReadyToClose=false` (`CloseSummary` dipertahankan agar masih terbaca di tampilan closed), publish otomatis oleh `UpdateThread`, lalu `m.FireClose(t)`; (g) kumpulan galat langkah (c)-(e) dicatat SATU pesan `PostSystem` ("penutupan: <ringkas galat>") dan satu pesan "thread ditutup" saat sukses; (h) bila ada sesi yang masih hidup: thread TETAP `active`, `Close` mengembalikan galat jelas ("sesi X belum bisa dimatikan; coba lagi"), flag closing dilepas, dan panggilan ulang melanjutkan (idempoten).
8. **Wiring Go butuh satu baris di `wire_thread.go` (koreksi tiket).** Tiket meminta tidak mengedit `wire_thread.go` dan memakai `init()`, tetapi di kode tidak ada registri paket yang menerima `*Manager` dari `init()` (diperiksa: `internal/thread` tidak punya registry installer; `OnStart` butuh `Manager` yang hanya ada di `wire_thread.go`). Maka `RegisterClose(m *Manager)` dipanggil satu baris dari `wireThread` (sama seperti pola `InstallAgents`). Frontend serupa: `installClose()` satu baris di `thread/index.ts` (pola `installParticipants()`), dan `ThreadSection.svelte` satu sisipan (tidak ada slot item sidebar di kode tiket 01).
9. **Perluasan prompt master lewat pembungkus `ToolsSection`.** `RegisterClose` membungkus variabel `ToolsSection` (di `prompt.go`) agar prompt master menyebut `thread_propose_close`: `prev := ToolsSection; ToolsSection = func() string { return prev() + "\n- thread_propose_close(summary): ... " }`. Syarat: `RegisterClose` dipanggil SETELAH `InstallAgents` (yang menetapkan `ToolsSection = AgentToolsSection`). `mcp_tools.go`/`prompt.go` tidak diedit. Kalimat tambahan menegaskan: hanya mengusulkan, jangan menutup sendiri, panggil bila goal tercapai.
10. **UI pembaruan hanya dari event.** Banner, tombol, badge, penanda read-only diturunkan dari `Thread` store (`thread.updated`). Tidak ada polling/timer. Dialog memanggil `thread.closePreview` saat dibuka (RPC sekali, bukan polling).
11. **Satu dialog per tab, dihost `CloseButton`.** Banner dan tombol header memicu dialog lewat `closeDialogStore.ts` (`openCloseDialog(threadId)`, `closeCloseDialog()`, `closeDialogFor` readable). `CloseButton` SELALU me-render `<CloseDialog>` (walau tombolnya disembunyikan untuk thread closed) dan dialog tampil bila `closeDialogFor === thread.id`.

### 2.3 Tipe dan signature bersama

**Go, `types.go` (Foundation, tambahan saja):**

```go
// pada struct Thread:
CloseSummary string `json:"closeSummary,omitempty"` // ringkasan usul tutup dari master (tiket thread-close)

// konstanta method:
MethodClosePreview = "thread.closePreview"
MethodClose        = "thread.close"
```

**Go, `close.go` (milik `thread-close-daemon`; signature final):**

```go
// Respons thread.closePreview. Semua slice selalu non-nil ([]).
type ClosePreviewParticipant struct {
    SessionID string `json:"sessionId"`
    Name      string `json:"name"`
    ProjectID string `json:"projectId"`
}
type CloseUncommitted struct {
    Path   string `json:"path"`
    Status string `json:"status"` // kode porcelain tanpa spasi, mis. "M", "A", "D", "??", "R"
}
type CloseCommit struct {
    SHA     string `json:"sha"`     // singkat (hasil %h)
    Subject string `json:"subject"`
}
type CloseWorktreeEntry struct {
    Participant     ClosePreviewParticipant `json:"participant"`
    Path            string                  `json:"path"`
    Branch          string                  `json:"branch"`
    BaseBranch      string                  `json:"baseBranch"` // "" bila HEAD detached saat spawn
    Uncommitted     []CloseUncommitted      `json:"uncommitted"`
    UnmergedCommits []CloseCommit           `json:"unmergedCommits"`
    Error           string                  `json:"error,omitempty"`
}
type ClosePreview struct {
    ThreadID  string               `json:"threadId"`
    Worktrees []CloseWorktreeEntry `json:"worktrees"`
}

type CloseParams struct {
    ID string `json:"id"`
}

// Logika (method pada Manager atau fungsi paket; pilih satu, nama persis):
func (m *Manager) ProposeClose(threadID, callerSessionID, summary string) (Thread, error)
func (m *Manager) ClosePreview(threadID string) (ClosePreview, error) // read-only
func (m *Manager) Close(threadID string) (Thread, error)              // idempoten, single-flight per thread

// close_ipc.go, dipanggil dari wire_thread.go (setelah InstallAgents, sebelum FireStart):
func RegisterClose(m *Manager)
```

Catatan: `RegisterClose` di Foundation berupa stub no-op dengan signature yang sama; `thread-close-daemon` mengisinya (memindahkan definisinya ke `close_ipc.go` dan menghapus stub dari `close.go`; Foundation menaruh stub di `close.go`, jadi `close.go` yang diganti penuh tidak boleh lagi mendefinisikan `RegisterClose`). Aman dipanggil sekali; panggilan ganda tidak panic (`sync.Once` untuk `RegisterTool`, karena `RegisterTool` dan `RegisterMethod` panic pada duplikat).

**Semantik `ProposeClose`:** pemanggil harus master thread itu (`Participant.Role == RoleMaster` dan `SessionID == callerSessionID`), selain itu galat "hanya master yang dapat mengusulkan penutupan"; thread closed -> `ErrClosed`; summary kosong (setelah TrimSpace) -> `ErrInvalid` "summary wajib diisi"; dipotong 2000 rune. Idempoten (panggilan ulang menimpa summary). TIDAK menutup thread.

**Tool MCP** (didaftarkan lewat `RegisterTool(ToolDef{...})`, `MasterOnly: true`, handler memeriksa `c.Master` lagi):

```
Name: "thread_propose_close"
Description: "Usulkan penutupan thread karena goal tercapai. Hanya mengusulkan: user yang menutup lewat tombol Close. Beri ringkasan hasil."
InputSchema: object{ summary: string (wajib) }
Hasil sukses: string "usul penutupan dicatat; menunggu user menekan Close"
```

**IPC** (didaftarkan `m.RegisterMethod(name, handler)`):

- `thread.closePreview{id}` -> `ClosePreview`. Id kosong -> `ErrInvalid`; thread tak ada -> `ErrNotFound` (dipetakan otomatis oleh `ipcToError`). Thread `closed` -> `worktrees: []`, tanpa galat.
- `thread.close{id}` -> `Thread` (closed). Thread tak ada -> `ErrNotFound`.

**TypeScript, `types.ts` (Foundation menambah):**

```ts
// pada interface Thread:
closeSummary?: string;
// pada THREAD_METHODS:
closePreview: 'thread.closePreview', close: 'thread.close',
```

**TypeScript, `close/closeApi.ts` (milik `thread-close-ui`):**

```ts
export interface CloseUncommitted { path: string; status: string }
export interface CloseCommit { sha: string; subject: string }
export interface CloseWorktreeEntry {
  participant: { sessionId: string; name: string; projectId: string };
  path: string; branch: string; baseBranch: string;
  uncommitted: CloseUncommitted[]; unmergedCommits: CloseCommit[]; error?: string;
}
export interface ClosePreview { threadId: string; worktrees: CloseWorktreeEntry[] }
export function fetchClosePreview(threadId: string): Promise<ClosePreview>  // normalisasi: array null -> []
export function closeThread(threadId: string): Promise<Thread>              // memakai call() dari lib/ipc
```

Pembungkus memakai `call` dari `../../../lib/ipc` dan `THREAD_METHODS` dari `../types`; galat ditampilkan lewat `errorText` dari `../thread-client`.

```ts
// readOnly.ts
export function isThreadReadOnly(thread: Thread): boolean // thread.state === 'closed'
// closeDialogStore.ts
export const closeDialogFor: Readable<string | null>
export function openCloseDialog(threadId: string): void
export function closeCloseDialog(): void
// index.ts
export function installClose(): () => void  // idempoten; registrasi slot (2.4); mengembalikan pembersih
export { default as ReadyBadge } from './ReadyBadge.svelte'
export { isThreadReadOnly } from './readOnly'
```

### 2.4 Kontrak frontend: slot dan komponen

Memakai mekanisme `registerThreadSlot` dari `../slots` (sudah ada; komponen slot menerima props `{thread: Thread, message?: ThreadMessage}`) dan token tema seperti `MessageItem.svelte`/`ThreadHeader.svelte` (`var(--ok, #30a46c)`, `var(--danger, #e5484d)`, `var(--fg-muted, #888)`, `var(--bg-elev, #fff)`, `var(--border, #8884)`, `var(--bg-hover, #0001)`). Tidak boleh menambah warna hard-code di luar fallback `var(...)` itu. Warna hijau banner: `var(--ok, #30a46c)` sebagai batas/aksen dengan latar `color-mix` atau latar transparan tipis; teks tetap terbaca di tema gelap dan terang.

`installClose()` mendaftarkan (id tetap agar idempoten):

| slot | id | komponen | order | tampil bila |
|---|---|---|---|---|
| `header-actions` | `close-button` | `CloseButton` | 100 | selalu dirender (hosting dialog); tombol hanya bila `state === 'active'` |
| `above-input` | `close-banner` | `CloseBanner` | -10 | `thread.readyToClose && thread.state === 'active'` |
| `above-input` | `close-notice` | `ClosedNotice` | 0 | `thread.state === 'closed'` |

Perilaku komponen:

- `CloseBanner` (props `{thread}`): banner hijau "Goal tercapai", `thread.closeSummary` (pre-wrap, `overflow-wrap:anywhere`), tombol "Close" -> `openCloseDialog(thread.id)`. `role="status"`.
- `CloseButton` (props `{thread}`): tombol "Tandai selesai" aktif kapan saja saat `active`; tidak dirender (atau nonaktif) saat `closed`. Selalu me-mount `<CloseDialog {thread} open={$closeDialogFor === thread.id} />`.
- `CloseDialog` (props `{thread, open}`): modal mengikuti pola `NewThreadDialog.svelte` (Escape menutup bila tak sibuk). Saat `open` berubah true memanggil `fetchClosePreview` SEKALI (guard generasi agar respons usang diabaikan). Status: loading, galat (dengan "Coba lagi"), daftar. Per worktree: nama peserta, nama project (dari `participant.projectId`/branch), branch; dua daftar (uncommitted: status + path; commit belum di-merge: sha + subject); bersih -> "tidak ada yang akan hilang"; entri `error` -> peringatan "tidak bisa diperiksa" + pesan. Peringatan merah permanen: semua worktree dan branch thread dihapus permanen, semua agent dimatikan. Tombol "Batal" (tidak memanggil `thread.close`) dan "Lanjut tutup" (memanggil `closeThread` sekali; tombol nonaktif selama proses; sukses -> `closeCloseDialog()`; gagal -> tampilkan `errorText`, dialog tetap terbuka, tombol aktif lagi). Daftar panjang discroll (max-height), tanpa scroll horizontal.
- `ClosedNotice` (props `{thread}`): "Thread ditutup pada <closedAt diformat>" (pakai `formatTime` dari `../format` bila cocok, kalau tidak `toLocaleString`).
- `ReadyBadge` (props `{thread}`): chip kecil "siap ditutup" bila `thread.readyToClose && thread.state === 'active'`, selain itu tidak merender apa pun. DIPAKAI Wiring di `ThreadSection.svelte` (tidak diregistrasi lewat slot).
- Input read-only: `ThreadTab.svelte` sudah menonaktifkan `ThreadInput` saat `state === 'closed'` (placeholder "Thread sudah ditutup"), jadi tidak ada perubahan input; `ClosedNotice` melengkapi.

## 3. Daftar fitur

| slug | scope | ownedPaths |
|---|---|---|
| `thread-close-daemon` | `ProposeClose`, `ClosePreview`, `Close` (single-flight per thread, idempoten, tahan gagal sebagian), parsing hasil `InspectWorktree`, cleanup via `RemoveWorktree`, pendaftaran IPC `thread.closePreview`/`thread.close`, tool MCP `thread_propose_close`, pembungkus `ToolsSection`, `RegisterClose`. | `internal/thread/close.go`, `internal/thread/close_ipc.go` |
| `thread-close-ui` | `CloseBanner`, `CloseButton`, `CloseDialog`, `ClosedNotice`, `ReadyBadge`, `closeApi.ts`, `closeDialogStore.ts`, `readOnly.ts`, `installClose()` di `index.ts`. | `frontend/src/modules/thread/close/` |

Dependensi antar fitur setelah Foundation: tidak ada; lintas fitur hanya lewat bentuk IPC dan `Thread.closeSummary`/`readyToClose` (2.3).

## 4. Foundation

Ada (`00-foundation.md`, hasFoundation = true). Isi ringkas: `CloseSummary` + konstanta method di `types.go`; `closeSummary` + `THREAD_METHODS` di `types.ts`; stub `internal/thread/close.go` (`RegisterClose` no-op) dan `frontend/src/modules/thread/close/index.ts` (`installClose` no-op); guard build Go dan frontend. Foundation menyentuh `types.go`/`types.ts` hanya secara aditif (diizinkan: "hanya boleh menambah field opsional" dan konstanta).

## 5. Wiring checklist (setelah semua fitur selesai)

Hanya berkas berikut yang boleh disentuh agen wiring (semua perubahan aditif kecil):

1. **`cmd/shikigamid/wire_thread.go`**: di `wireThread`, SETELAH blok `thread.InstallAgents(...)` dan SEBELUM `m.FireStart()`, tambahkan:
   ```go
   thread.RegisterClose(m)
   ```
   (urutan penting: `RegisterClose` membungkus `ToolsSection` yang ditetapkan `InstallAgents`). Bila tiket 03 sudah memasang `InstallActivity`, letakkan baris ini setelahnya; urutannya tidak bergantung satu sama lain selain harus setelah `InstallAgents`.
2. **`frontend/src/modules/thread/index.ts`**: tambah `import { installClose } from './close';` dan panggil `installClose();` tepat setelah `installParticipants();` (pola yang sama).
3. **`frontend/src/modules/thread/ThreadSection.svelte`**: `import { ReadyBadge } from './close';` dan di dalam `<button class="row">` setelah `.title`, sebelum `{#if t.state === 'closed'}`, sisipkan `<ReadyBadge thread={t} />`. Tidak ada perubahan lain (tag "ditutup" yang ada tetap).
4. **Tidak diubah** (verifikasi saja): `manager.go`, `ipc.go`, `mcp_tools.go`, `prompt.go`, `ThreadTab.svelte`, `ThreadHeader.svelte` (sudah punya badge "siap ditutup" sendiri, biarkan), `ThreadInput.svelte`.
5. **Verifikasi manual** (jalankan aplikasi): buat thread, minta master men-spawn minimal satu peserta, buat satu perubahan belum di-commit dan satu commit di worktree peserta; master memanggil `thread_propose_close` (banner hijau + summary muncul, badge "siap ditutup" di sidebar, thread belum closed); tombol "Tandai selesai" di header juga bekerja tanpa usul; Close membuka dialog yang mendaftar perubahan dan commit; Batal tidak mengubah apa pun; Lanjut tutup: sesi mati (`shikigamictl session list` tidak memuat sesi thread), `git worktree list` dan `git branch --list 'thread/<id>-*'` project bersih, direktori `<DataDir>/threads/<id>/<slug>` hilang tetapi `thread.json` dan `messages.jsonl` ada, tab menampilkan "Thread ditutup pada ...", input nonaktif; `thread.send` dan `thread_send` ke thread itu ditolak "thread sudah ditutup"; memanggil `thread.close` dua kali tidak error; working tree utama project tidak berubah (`git status` project bersih seperti sebelumnya).

## 6. Build guard

Satu-satunya guard, tanpa unit test:

- `go build ./...` (akar repo) dan `gofmt -l internal/thread cmd/shikigamid` kosong; dijalankan Foundation, `thread-close-daemon`, dan Wiring.
- Frontend: `cd frontend && npm run check && npm run build`; dijalankan Foundation, `thread-close-ui`, dan Wiring.
- Tidak ada dependency baru di `go.mod`/`package.json`.

## 7. Aturan paralel

- Tiap agent HANYA menyentuh `ownedPaths` fiturnya (bagian 3). Dilarang menyentuh path fitur lain, berkas Foundation (`types.go`, `types.ts`), berkas wiring (`wire_thread.go`, `thread/index.ts`, `ThreadSection.svelte`), dan berkas tiket lain (`manager.go`, `store.go`, `ipc.go`, `mcp_tools.go`, `worktree.go`, `worktree_records.go`, `prompt.go`, `internal/session`, `internal/mention`, `internal/status`).
- `thread-close-daemon` menimpa stub `close.go` (membuang stub `RegisterClose` dari `close.go` dan memindahkannya ke `close_ipc.go`, signature 2.3 tetap) dan membuat `close_ipc.go`. `thread-close-ui` menimpa stub `close/index.ts` dan membuat berkas lain di folder itu; sebelum wiring folder itu belum dirender siapa pun, jadi uji dengan data/`thread` lokal bila perlu.
- Butuh perubahan di luar `ownedPaths`: JANGAN dikerjakan; catat di bagian "Catatan untuk wiring" plan fitur.
- Jangan mengubah signature/konstanta/format 2.2-2.4; jangan menambah ekspor baru yang dipakai fitur lain.
- Tiap agent menjalankan guard bagian 6 yang relevan sebelum melapor selesai; kegagalan build akibat berkas fitur lain hanya dilaporkan, bukan diperbaiki.
