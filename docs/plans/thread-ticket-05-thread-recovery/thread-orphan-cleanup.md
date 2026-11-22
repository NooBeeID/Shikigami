# Plan: Bersihkan worktree yatim saat daemon start dan UI pemulihan

## Metadata

- Tiket: `thread-ticket-05-thread-recovery` (`docs/tickets/thread-ticket-05-thread-recovery.md`)
- Fitur: `thread-orphan-cleanup`
- ownedPaths: `internal/thread/orphans.go`, `frontend/src/modules/thread/recovery/`
- Depends on foundation: ya (`00-foundation.md` harus sudah selesai: field `Participant.Dead`/`PreviousSessionIDs`, stub `orphans.go`, stub `recovery/index.ts`, konstanta `RECOVERY_*` di `types.ts`)
- Rujukan kontrak: `overview.md` bagian 2.5 (CleanOrphans), 2.7 (frontend), 2.9 butir 7

## Tujuan

1. Go: `CleanOrphans(m *Manager) (cleaned int, err error)` membersihkan sisa worktree, branch `thread/<threadId>-*`, direktori `master`, dan berkas `.git` worktree milik thread `closed` (mis. daemon crash di tengah Close). Thread `active` tidak disentuh. Idempoten, tahan kegagalan parsial, tiap perintah git berbatas waktu 20 detik.
2. Frontend: komponen pemulihan (lencana peserta mati, panel daftar peserta mati, notis pesan sistem pemulihan) dan `installRecovery()` idempoten yang memasangnya lewat slot.

## Path yang dimiliki

HANYA ini yang boleh disentuh:

- `internal/thread/orphans.go` (timpa total stub foundation; tanda tangan `CleanOrphans` dipertahankan)
- `frontend/src/modules/thread/recovery/` (timpa `index.ts` stub; buat `DeadBadge.svelte`, `RecoveryPanel.svelte`, `RecoveryNotice.svelte`)

Jangan menyentuh: `recovery.go`, `recovery_contract.go`, `presend.go`, `types.go`, `types.ts`, `thread-store.ts`, `cmd/shikigamid/wire_thread.go`, `frontend/src/modules/thread/index.ts`, `participants/`, `worktree.go`, `worktree_records.go`, atau berkas tiket lain. Helper tak diekspor di paket `thread` boleh dipanggil, tidak boleh diubah. Bila butuh perubahan di berkas lain, catat di bagian Wiring, jangan edit.

## Kontrak

Disediakan (tanda tangan final, jangan diubah):

```go
func CleanOrphans(m *Manager) (cleaned int, err error) // cleaned = jumlah worktree + branch yang benar-benar dihapus
```

```ts
export function installRecovery(): () => void; // idempoten
export { DeadBadge, RecoveryPanel, RecoveryNotice };
export function isRecoveryMessage(m: ThreadMessage | undefined): boolean;
export function deadParticipants(thread: Thread): Participant[];
```

Dipakai (baca-saja, sudah ada di kode):

- Go (paket `thread`): `m.cfg.Store` (`ThreadDir(id)`), `m.managerAllThreads() []Thread`, `m.cfg.Workspaces.Projects(workspaceID) ([]ProjectInfo, bool)` (boleh nil-interface: cek `m.cfg.Workspaces != nil`), `m.log` (`*slog.Logger`), `StateClosed`, `LoadWorktreeRecords(threadDir) ([]WorktreeRecord, error)` (field `ProjectPath` ada di `rec.Info.ProjectPath`; `rec.Info.Path`), `worktreeGit() (string, error)`, `worktreeRun(ctx, gitBin, dir, args...) (string, error)` (menjalankan `git -C dir args...`, sudah menangani deadline), `worktreeLines(s) []string`, `worktreeBranchExists`, `worktreeExists(path) bool`, konstanta `worktreeBranchPrefix` (`"thread/"`). `RemoveWorktree` TIDAK dipakai.
- Frontend: `registerThreadSlot` dari `../slots`; tipe `Thread`, `Participant`, `ThreadMessage`, konstanta `RECOVERY_DEAD_NOTICE`, `RECOVERY_RESPAWN_PREFIX`, `RECOVERY_FAILED_PREFIX` dari `../types` (ditambah foundation).
- Pola referensi: `frontend/src/modules/thread/participants/index.ts` (idempoten), `participants/MessageExtra.svelte` (gaya callout), `participants/ParticipantsPanel.svelte` (struktur panel). Baca sebelum menulis.

## Langkah

Baca dulu `internal/thread/worktree.go`, `worktree_records.go`, `manager.go` (`managerAllThreads`), dan `participants/*` agar nama sesuai kode sebenarnya. Bila kode berbeda dari plan ini, ikuti kode dan catat di Wiring.

### Langkah 1: `internal/thread/orphans.go` (timpa total)

Impor: `bufio`/`strings`, `context`, `errors`, `fmt`, `os`, `path/filepath`, `time`.

Konstanta dan tipe:

```go
const orphansGitTimeout = 20 * time.Second

type orphansResult struct{ worktrees, branches int; masterDir bool }
```

Fungsi (semua tak diekspor, awalan `orphans`):

1. `func CleanOrphans(m *Manager) (cleaned int, err error)`:
   - `gitBin, gerr := worktreeGit()`. Bila galat, tetap lanjut (gitBin = "") dan hanya pakai penghapusan berkas; catat `Warn` sekali, tambahkan ke galat gabungan.
   - Loop `for _, t := range m.managerAllThreads()`; lewati bila `t.State != StateClosed` (thread active/lain TIDAK disentuh).
   - Per thread panggil `orphansCleanThread(m, gitBin, t)` di dalam fungsi pembungkus dengan `defer recover()` (panic satu thread dicatat `Warn` dan menjadi galat, loop lanjut).
   - Kumpulkan `cleaned += res.worktrees + res.branches`; galat dikumpulkan `errs = append(errs, e)`; akhir: `return cleaned, errors.Join(errs...)`.
2. `func orphansCleanThread(m *Manager, gitBin string, t Thread) (orphansResult, error)`:
   - `dir := m.cfg.Store.ThreadDir(t.ID)`. Bila `dir` tidak ada (`os.Stat`) -> kembalikan hasil kosong tanpa log. Jaga: `t.ID` kosong atau mengandung pemisah path -> kembalikan galat, jangan lanjut.
   - Kumpulkan `repos` (daftar unik path repo, urut stabil) lewat `orphansCandidateRepos(m, t, dir)`.
   - Daftar `leftovers`: hasil `orphansListLeftovers(dir)` (lihat butir 4).
   - Untuk tiap `path` di `leftovers`: `orphansRemoveWorktree(m, gitBin, repos, dir, path)`; bila sukses `res.worktrees++`; galat dikumpulkan, lanjut.
   - Setelah worktree dihapus, jalankan `git worktree prune` sekali per repo (galat dicatat `Warn`, tidak fatal) hanya bila ada worktree yang dihapus atau ada entri porcelain di bawah `dir`.
   - Branch: `orphansDeleteBranches(m, gitBin, repos, t.ID)` -> `res.branches`. Dilakukan SETELAH worktree dihapus (git menolak `branch -D` untuk branch yang masih ter-checkout di worktree).
   - Direktori master: `masterPath := filepath.Join(dir, "master")`; bila `worktreeExists(masterPath)` -> `orphansSafeRemoveAll(dir, masterPath)`; sukses -> `res.masterDir = true`; `m.log.Info("thread: hapus direktori master", "thread", t.ID, "path", masterPath)`. Direktori master TIDAK dihitung ke `cleaned`.
   - Ringkasan: hanya bila ada tindakan (`worktrees+branches>0 || masterDir`): `m.log.Info("thread: pembersihan yatim", "thread", t.ID, "worktrees", res.worktrees, "branches", res.branches, "masterDir", res.masterDir)`. Tanpa tindakan -> tidak ada log `Info` (idempoten, senyap).
   - Kembalikan `res, errors.Join(errs...)`.
3. `func orphansCandidateRepos(m *Manager, t Thread, dir string) []string`: gabungan unik, hanya yang lolos `orphansIsGitRepo(p)` (path ada, `os.Stat(filepath.Join(p, ".git"))` berhasil, atau path adalah repo bare/dir dengan `HEAD`; cukup cek `.git` ada):
   - (a) `LoadWorktreeRecords(dir)` -> `rec.Info.ProjectPath` (galat baca diabaikan dengan log `Debug`).
   - (b) tiap subdirektori `dir/<x>` selain `master`: baca berkas `.git` (file, bukan direktori); bila diawali `gitdir:`, ambil path setelahnya (trim spasi), cari segmen `/.git/worktrees/` dan ambil bagian sebelum `/.git/` sebagai repo induk (`orphansRepoFromGitFile(content string) (string, bool)`).
   - (c) `m.cfg.Workspaces != nil` -> `Projects(t.WorkspaceID)`, tiap `Path`.
   Semua path dibersihkan `filepath.Clean`.
4. `func orphansListLeftovers(dir string) []string`: `os.ReadDir(dir)`; ambil entri direktori yang namanya bukan `master` dan bukan berkas (`thread.json`, `messages.jsonl`, `worktrees.json` adalah berkas, jadi tersaring oleh `IsDir()`; jangan pernah menghapus berkas apa pun di `dir`). Kembalikan `filepath.Join(dir, name)`. Tambahkan juga path di bawah `dir` yang muncul di `git worktree list --porcelain` tiap repo (baris `worktree <path>`, bandingkan dengan `orphansUnder(dir, path)`), meski direktorinya sudah hilang dari disk (entri basi); deduplikasi. Perintah porcelain memakai `orphansGit(m, gitBin, repo, args...)`.
5. `func orphansRemoveWorktree(m *Manager, gitBin string, repos []string, dir, path string) error`:
   - Guard awal WAJIB: `orphansUnder(dir, path)` harus true (path di bawah `dir`, bukan `dir` itu sendiri, setelah `filepath.Clean` + `filepath.Rel` tidak diawali `..`); bila tidak, kembalikan galat dan jangan hapus.
   - Tentukan repo pemilik: dari berkas `.git` worktree (`orphansRepoFromGitFile`) bila ada; bila tidak, coba setiap repo kandidat.
   - Bila `gitBin != ""` dan ada repo: `orphansGit(m, gitBin, repo, "worktree", "remove", "--force", "--force", path)`. Sukses -> `Info("thread: worktree yatim dihapus", "thread", ..., "path", path)`, kembalikan nil (tapi hitung hanya bila path benar-benar ada/terdaftar sebelumnya; entri basi yang hanya di-prune tidak dihitung).
   - Gagal atau tak ada repo/git: `Warn` lalu fallback `orphansSafeRemoveAll(dir, path)` (mencakup berkas `.git` worktree di dalam path). Sukses -> `Info("thread: worktree yatim dihapus lewat penghapusan berkas", ...)`.
   - Galat fallback dikembalikan (dibungkus `fmt.Errorf("hapus worktree %s: %w", path, err)`).
6. `func orphansDeleteBranches(m *Manager, gitBin string, repos []string, threadID string) (int, error)`:
   - Per repo: `out, err := orphansGit(m, gitBin, repo, "branch", "--list", "--format=%(refname:short)", worktreeBranchPrefix+threadID+"-*")`. Galat -> kumpulkan, lanjut repo berikutnya.
   - Untuk tiap nama dari `worktreeLines(out)` (trim): WAJIB `strings.HasPrefix(name, worktreeBranchPrefix+threadID+"-")` sebelum `git branch -D <name>` (pola glob tidak dipercaya sendirian). Sukses -> `Info("thread: branch yatim dihapus", "thread", threadID, "repo", repo, "branch", name)`, hitung `+1`. Gagal -> `Warn` + kumpulkan galat.
   - Bila `gitBin == ""` kembalikan 0 tanpa galat tambahan.
7. `func orphansGit(m *Manager, gitBin, repo string, args ...string) (string, error)`: `ctx, cancel := context.WithTimeout(context.Background(), orphansGitTimeout)`; `defer cancel()`; `return worktreeRun(ctx, gitBin, repo, args...)`. Satu timeout baru PER perintah.
8. `func orphansUnder(base, p string) bool` dan `func orphansSafeRemoveAll(base, p string) error`: hapus lewat `os.RemoveAll` hanya bila `orphansUnder(base, p)`; selain itu kembalikan galat `fmt.Errorf("menolak menghapus di luar %s: %s", base, p)`. Jangan mengikuti symlink keluar: `os.Lstat(p)` dulu; bila symlink, `os.Remove(p)` saja.
9. `func orphansRepoFromGitFile(content string) (string, bool)`: parse `gitdir: <path>`; cari `"/.git/worktrees/"` (gunakan `filepath.ToSlash`), ambil awalan sebelum `/.git/`; kembalikan `filepath.FromSlash`.

Aturan umum: komentar dan pesan galat Bahasa Indonesia; tanpa goroutine, timer, atau dependency baru; `CleanOrphans` sinkron (pemanggil wiring yang membuat goroutine). Jangan pernah menyentuh `thread.json`, `messages.jsonl`, `worktrees.json`.

### Langkah 2: `frontend/src/modules/thread/recovery/DeadBadge.svelte`

Svelte 5 runes + TypeScript. Props `{ participant }: { participant: Participant }` (impor tipe dari `../types`). Render hanya bila `participant.dead === true`: `<span class="badge" role="status" title="...">agent mati, akan dihidupkan saat pesan berikutnya</span>`. Gaya kecil (font 10-11px, latar kuning lembut `rgba(217,164,0,0.14)`, border-radius 999px), gunakan variabel `--fg` seperti komponen tetangga. Tanpa logika lain.

### Langkah 3: `RecoveryPanel.svelte`

Props `{ thread }: { thread: Thread; message?: ThreadMessage }`. `const dead = $derived(thread.state === 'closed' ? [] : deadParticipants(thread));` (impor `deadParticipants` dari `./util`, lihat Langkah 5). Render `{#if dead.length > 0}` sebuah `<section class="panel">` dengan judul kecil "Agent mati", daftar `{#each dead as p (p.sessionId)}` berisi nama (`p.name || p.role`) + `<DeadBadge participant={p} />`. Tanpa `{#if}` terpenuhi -> tidak merender apa pun (agar kolom tidak menambah ruang kosong). Reaktif murni dari prop, sehingga hilang otomatis saat `thread.updated` membawa `dead` kosong. Gaya selaras `ParticipantsPanel`.

### Langkah 4: `RecoveryNotice.svelte`

Props `{ thread, message }: { thread: Thread; message?: ThreadMessage }`. Turunan: `kind = $derived(recoveryKind(message))` dari `./util` (nilai `'dead' | 'respawn' | 'failed' | null`). Render hanya bila `kind !== null`: callout ringan di bawah pesan (`role="note"`); `failed` bergaya peringatan (aksen merah/oranye `#c0392b`), lainnya info (aksen biru `#3b82c4`). Teks tambahan singkat: `dead` -> "Agent akan dihidupkan lagi otomatis saat ada pesan berikutnya."; `respawn` -> "Agent baru memakai ringkasan log thread sebagai konteks."; `failed` -> "Kirim pesan lagi untuk mencoba ulang." Jangan menduplikasi teks pesan sistem asli (renderer tiket 1 tetap menampilkannya). Gaya mengikuti `.callout` di `participants/MessageExtra.svelte`.

### Langkah 5: `util.ts` (berkas pembantu di folder recovery)

Hindari impor siklik dengan `index.ts`:

```ts
import type { Participant, Thread, ThreadMessage } from '../types';
import { RECOVERY_DEAD_NOTICE, RECOVERY_RESPAWN_PREFIX, RECOVERY_FAILED_PREFIX } from '../types';

export type RecoveryKind = 'dead' | 'respawn' | 'failed';
export function recoveryKind(m: ThreadMessage | undefined): RecoveryKind | null; // kind==='system' saja; === RECOVERY_DEAD_NOTICE -> 'dead'; startsWith respawn/failed prefix
export function isRecoveryMessage(m: ThreadMessage | undefined): boolean;        // recoveryKind(m) !== null
export function deadParticipants(thread: Thread): Participant[];                 // participants.filter(p => p.dead === true); aman bila participants undefined
```

Tidak ada literal teks duplikat: selalu pakai konstanta dari `types.ts`.

### Langkah 6: `index.ts` (timpa stub)

```ts
import { registerThreadSlot } from '../slots';
import RecoveryPanel from './RecoveryPanel.svelte';
import RecoveryNotice from './RecoveryNotice.svelte';
import DeadBadge from './DeadBadge.svelte';

export { DeadBadge, RecoveryPanel, RecoveryNotice };
export { isRecoveryMessage, deadParticipants } from './util';

let stopper: (() => void) | null = null;

// Idempoten: panggilan kedua mengembalikan pembersih yang sama.
export function installRecovery(): () => void {
  if (stopper) return stopper;
  registerThreadSlot({ slot: 'side-panel', component: RecoveryPanel, order: 20, id: 'recovery-panel' });
  registerThreadSlot({ slot: 'message-extra', component: RecoveryNotice, order: 20, id: 'recovery-notice' });
  const stop = () => { if (stopper === stop) stopper = null; };
  stopper = stop;
  return stop;
}
```

(`registerThreadSlot` sudah menimpa berdasarkan `id`, jadi pemasangan ulang aman.)

### Langkah 7: guard

- `go build ./...` (dari root repo)
- `cd frontend && npm run build` (opsional `npm run check`; kegagalan lama yang bukan dari berkas ini tidak diperbaiki)

Telaah ulang kode sendiri terhadap checklist: tidak ada penghapusan di luar `ThreadDir/<id>`, tidak menyentuh thread non-closed, branch selalu diverifikasi berawalan `thread/<id>-`.

## Perubahan

Dibuat/diubah:

- `internal/thread/orphans.go` (diganti total dari stub)
- `frontend/src/modules/thread/recovery/index.ts` (diganti total dari stub)
- `frontend/src/modules/thread/recovery/util.ts` (baru)
- `frontend/src/modules/thread/recovery/DeadBadge.svelte` (baru)
- `frontend/src/modules/thread/recovery/RecoveryPanel.svelte` (baru)
- `frontend/src/modules/thread/recovery/RecoveryNotice.svelte` (baru)

Tidak ada berkas lain yang diubah.

## Wiring

Fitur ini hanya mengekspos; JANGAN menyambungkannya. Langkah wiring (agen terpisah, `overview.md` bagian 5):

- Go: `thread.CleanOrphans(m)` dipanggil dari hook `OnStart` di `cmd/shikigamid/wire_thread.go` di dalam goroutine dengan `recover` (sinkron di dalam fungsi; kembalinya `(cleaned, err)`; galat gabungan bersifat peringatan, bukan fatal).
- Frontend: `installRecovery()` dari `./recovery` dipanggil di `frontend/src/modules/thread/index.ts` setelah `installParticipants()`. Ia mendaftarkan slot `side-panel` (id `recovery-panel`, order 20) dan `message-extra` (id `recovery-notice`, order 20).
- Catat di sini bila saat implementasi ada perbedaan antara plan dan kode (nama helper, bentuk `Workspaces`, dsb.): (diisi implementer).

## Selesai bila

- `go build ./...` sukses dan `cd frontend && npm run build` sukses (guard; tanpa unit test).
- `CleanOrphans` memiliki tanda tangan persis `func CleanOrphans(m *Manager) (cleaned int, err error)`; hanya thread `closed` diproses; `thread.json`, `messages.jsonl`, `worktrees.json` tidak pernah dihapus.
- Setiap penghapusan berkas dijaga `orphansUnder(ThreadDir, path)`; branch dihapus hanya bila berawalan `thread/<threadId>-`; tiap perintah git memakai timeout 20 detik sendiri; kegagalan satu thread/repo/worktree tidak menghentikan yang lain (galat digabung `errors.Join`); run ulang pada thread bersih tanpa tindakan, tanpa log `Info`, tanpa galat.
- `installRecovery()` idempoten, mendaftarkan dua slot sesuai id/order di atas; `DeadBadge`/`RecoveryPanel`/`RecoveryNotice` tidak merender apa pun saat tidak relevan (tidak ada peserta mati, thread closed, pesan bukan pesan pemulihan).
- Tidak ada berkas di luar ownedPaths yang berubah (`git status` hanya menunjukkan path di bagian Perubahan).
