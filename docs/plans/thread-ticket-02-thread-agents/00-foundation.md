# Foundation: thread-ticket-02-thread-agents

Rujukan: `docs/plans/thread-ticket-02-thread-agents/overview.md` (bagian 2 = kontrak; 2.2 = koreksi). Foundation dikerjakan SENDIRIAN sebelum fitur paralel. Tanpa unit test; guard hanya build.

Tujuan: setelah selesai, berkas Go bersama ada dan berkompilasi (`agents_types.go`, `worktree_records.go`, stub `worktree.go`, stub `mcp_tools.go`), dan titik ekstensi filter sesi di sidebar kiri tersedia. Aplikasi berperilaku sama seperti sebelumnya.

Berkas yang boleh disentuh: `internal/thread/agents_types.go`, `internal/thread/worktree_records.go`, `internal/thread/worktree.go`, `internal/thread/mcp_tools.go`, `frontend/src/modules/sidebar-left/registry.ts`, `frontend/src/modules/sidebar-left/index.ts`, `frontend/src/modules/sidebar-left/SessionGroups.svelte`. Tidak ada yang lain.

## Langkah 0: verifikasi prasyarat tiket 01

1. Pastikan ada: `internal/thread/{types,store,manager,master,prompt,ipc}.go`, `cmd/shikigamid/wire_thread.go`, `frontend/src/modules/thread/{index.ts,slots.ts,thread-store.ts,types.ts}`. Jalankan `go build ./...`.
2. Baca `manager.go`, `master.go`, `prompt.go`, `ipc.go`. Pastikan simbol ini ada dengan signature kompatibel: `Manager` (`Get`, `Post`, `PostSystem`, `AddParticipant`, `UpdateThread`, `SendToSession`, `OnMessage`, `OnClose`, `Tools`, `Store`), `SessionAPI`, `WorkspaceSource`, `ProjectInfo`, `ThreadCaller`, `ToolSpec`, `ToolRegistry`, `ToolsSection` (var di `prompt.go`), `resolveMasterProvider`, `ErrClosed`.
3. Bila ada simbol yang tidak ada atau namanya berbeda: SESUAIKAN `overview.md` bagian 2 (nama saja, bukan bentuk) agar cocok dengan kode nyata dan catat setiap perubahan di bagian "Catatan" di akhir berkas ini. Bila prasyarat tiket 01 tidak ada sama sekali, berhenti dan laporkan (jangan membuat ulang tiket 01).

## Langkah 1: `internal/thread/agents_types.go`

Tulis persis `Router`, `ErrPaused`, `ErrTargetUnknown`, `ErrTargetAmbiguous`, `ErrNotParticipant`, dan `ParticipantView` dari overview 2.3 (rapikan indentasi `var (...)` dengan gofmt). Package comment/komentar bahasa Indonesia; beri komentar bahwa berkas ini kontrak epic thread-ticket-02 dan hanya boleh ditambah.

## Langkah 2: `internal/thread/worktree_records.go`

Implementasi penuh (bukan stub):

1. Tipe `WorktreeRecord` (tag json `projectId, projectName, projectSlug, sessionId, info, createdAt`) memakai `WorktreeInfo` dari `worktree.go` (stub, langkah 3).
2. `recordsPath(threadDir) = filepath.Join(threadDir, "worktrees.json")`. Mutex paket `worktreeRecordsMu sync.Mutex` menjaga baca-ubah-tulis.
3. `LoadWorktreeRecords`: berkas tidak ada -> `(nil, nil)`; JSON rusak -> galat dibungkus (`fmt.Errorf("worktrees.json rusak: %w", err)`).
4. `UpsertWorktreeRecord`: muat, ganti entri ber-`ProjectID` sama atau tambah; isi `CreatedAt` (UTC) bila nol; tulis ke `.tmp` (0600) lalu `os.Rename`; buat direktori bila belum ada (`MkdirAll` 0700). Argumen kosong (`threadDir`/`ProjectID`) -> `errors.New`.
5. `DeleteWorktreeRecord`: idempoten (berkas atau entri tak ada -> nil); menulis ulang secara atomik.

## Langkah 3: stub `internal/thread/worktree.go`

Tulis `WorktreeInfo`, `WorktreeStatus`, `ErrNotGitRepo` (pesan persis overview), dan fungsi `Slugify`, `CreateWorktree`, `InspectWorktree`, `RemoveWorktree` dengan signature persis overview 2.3.

- `Slugify` BOLEH diimplementasikan langsung (huruf kecil, ganti run karakter non `[a-z0-9]` dengan `-`, trim `-`, kosong -> `project`, persis `master` -> `master-project`); fitur `thread-worktree` boleh memperhalus tanpa mengubah signature.
- Tiga fungsi lain: badan mengembalikan nilai nol dan `errors.New("worktree belum diimplementasikan")`. Beri komentar `// STUB(foundation): diganti oleh thread-worktree`.

## Langkah 4: stub `internal/thread/mcp_tools.go`

1. Tulis `Caller`, `ToolFunc`, `ToolDef` persis overview 2.3.
2. `RegisterTool` fungsional minimal: mutex paket + slice; panic dengan pesan jelas bila `Name` kosong, `Handler` nil, atau nama sudah terdaftar. `registeredTools()` mengembalikan salinan terurut nama.
3. Deklarasikan `ToolDeps` dan stub `RegisterCommonTools(ToolDeps)` (badan kosong) serta `AgentToolsSection() string` (kembalikan string kosong). Komentar `// STUB(foundation): diperkaya oleh thread-mcp-server`.

## Langkah 5: titik ekstensi filter sesi di sidebar kiri (frontend)

1. `modules/sidebar-left/registry.ts`: tambah (gaya mengikuti `registerSessionMenuItem`):
   ```ts
   export interface SessionFilterSpec { id: string; predicate: Readable<(s: Session) => boolean> } // true = tampil
   const filterStore = writable<Record<string, SessionFilterSpec>>({});
   export function registerSessionFilter(spec: SessionFilterSpec): void { filterStore.update((m) => ({ ...m, [spec.id]: spec })); }
   export const sessionFilter: Readable<(s: Session) => boolean> = derived(filterStore, ($m, set) => {
     const stores = Object.values($m).map((f) => f.predicate);
     if (stores.length === 0) { set(() => true); return; }
     return derived(stores, (fns) => (s: Session) => fns.every((f) => f(s))).subscribe(set);
   });
   ```
   Impor `Session` sudah ada di berkas itu. Pastikan type-check lulus (`derived` dengan callback `(values, set)`; bila tipe rewel, beri anotasi eksplisit).
2. `modules/sidebar-left/index.ts`: ekspor `registerSessionFilter`, `sessionFilter` dan tipe `SessionFilterSpec` (tambahkan ke baris export yang ada).
3. `modules/sidebar-left/SessionGroups.svelte`: impor `sessionFilter` dari `./registry`; ubah derivasi menjadi `const visible = $derived(((s: Session) => $sessionFilter(s) && (filter ? filter(s) : true)))` dan pakai `$sessions.filter(visible)` pada `groupSessionsByProject`. Tidak ada perubahan lain (markup/gaya tetap). Tanpa filter terdaftar perilaku identik dengan sekarang.

## Langkah 6: verifikasi

1. `gofmt -l internal/thread` kosong; `go vet ./internal/thread/...`; `go build ./...`.
2. `cd frontend && npm run check && npm run build`.
3. Pastikan tidak ada berkas di luar daftar "boleh disentuh" yang berubah (`git status`).

## Catatan

(Foundation mengisi di sini bila ada penyesuaian nama terhadap kode tiket 01.)
