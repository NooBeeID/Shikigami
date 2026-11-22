# Foundation: thread-ticket-01-thread-core

Rujukan: `docs/plans/thread-ticket-01-thread-core/overview.md` (bagian 2 = kontrak; bagian 2.2 = koreksi). Foundation dikerjakan SENDIRIAN sebelum fitur paralel. Tanpa unit test; guard hanya build.

Tujuan: setelah selesai, `internal/thread/types.go` final ada, `internal/thread/store.go` berupa stub yang berkompilasi dengan signature final, dan host frontend siap menampung pane `thread` serta bagian sidebar. Aplikasi berperilaku sama seperti sebelumnya.

Berkas yang boleh disentuh: `internal/thread/types.go`, `internal/thread/store.go`, `frontend/src/modules/layout/model.ts`, `frontend/src/modules/sidebar-left/registry.ts`, `frontend/src/modules/sidebar-left/AgentSidebar.svelte`. Tidak ada yang lain.

## Langkah 1: `internal/thread/types.go`

1. Buat berkas dengan package comment (bahasa Indonesia) yang menjelaskan paket thread dan konvensi path: `DataDir/threads/<threadId>/{thread.json,messages.jsonl,master/,<projectSlug>/}` dan branch `thread/<threadId>-<slug>` (worktree diimplementasi tiket thread-02, bukan di sini).
2. Tulis persis tipe, konstanta, dan tipe param/respons dari overview 2.3 (`Thread`, `Participant`, `ThreadMessage`, konstanta state/role/kind/from/to, konstanta metode & event IPC, `CreateParams`, `ListParams`, `GetParams`, `MessagesParams`, `SendParams`, `OwnedSessionsResult`). Tag json: `id, workspaceId, title, goal, state, createdAt, closedAt(omitempty), participants, readyToClose`; `sessionId, projectId, role, name`; `id, threadId, from, to, text, ts, kind`; param: `workspaceId, goal, title(omitempty)`, `workspaceId`, `id`, `id` + `sinceTs(omitempty)`, `id` + `text`; hasil: `sessionIds`.
3. `func NewID() string { return mention.NewMessageID() }` (impor `github.com/NooBeeID/shikigami/internal/mention`).
4. Komentar di atas tipe: "Bentuk stabil; tiket lain hanya boleh menambah field opsional dengan omitempty."
5. Verifikasi: `go build ./internal/thread/...`.

## Langkah 2: `internal/thread/store.go` (stub)

1. Impor seperlunya (`errors`, `time`). Definisikan `ErrNotFound`, `ErrInvalid` (teks pada overview 2.3), `type Store struct{ baseDir string }`, dan semua method pada overview 2.3 dengan signature persis (`NewStore`, `Create`, `Get`, `List`, `Save`, `AppendMessage`, `Messages`, `ThreadDir`, `MasterDir`).
2. Badan method yang butuh disk mengembalikan nilai nol dan `errors.New("store thread belum diimplementasikan")`; `ThreadDir`/`MasterDir` boleh langsung `filepath.Join` (aman). Beri komentar `// STUB(foundation): diganti oleh thread-foundation`.
3. Verifikasi: `go build ./...`.

## Langkah 3: `frontend/src/modules/layout/model.ts` (koreksi 2.2 no. 2)

Perubahan aditif minimal:

1. `export type PaneKind = 'terminal' | 'file' | 'thread';`
2. Tambah `threadId?: string;` pada `interface Leaf` dan `interface NewLeaf`.
3. `makeLeaf`: tambah `if (nl.threadId !== undefined) leaf.threadId = nl.threadId;`.
4. `hydrateNode`: tambahkan `'threadId'` ke daftar kunci string yang disalin (`['sessionId', 'projectId', 'provider', 'filePath', 'workspaceId', 'root', 'threadId']`).
5. `openPane` (dedup `findLeafBy`): tambah baris `if (nl.threadId !== undefined && l.threadId === nl.threadId) return true;`.
6. Jangan mengubah fungsi lain. `modules/tabs/index.ts` (shim) mengembalikan `null` untuk leaf non-terminal/non-file, jadi leaf thread tidak ikut jembatan; tidak perlu diubah.

## Langkah 4: slot bagian sidebar (koreksi 2.2 no. 1)

1. `frontend/src/modules/sidebar-left/registry.ts`: tambah (gaya sama dengan `registerSessionMenuItem`):
   ```ts
   export interface SidebarSectionSpec { id: string; component: Component<any>; order?: number }
   const sectionStore = writable<Record<string, SidebarSectionSpec>>({});
   export function registerSidebarSection(spec: SidebarSectionSpec): void { sectionStore.update((m) => ({ ...m, [spec.id]: spec })); }
   export function sidebarSections(): Readable<SidebarSectionSpec[]> { return derived(sectionStore, (m) => Object.values(m).sort(byOrder)); }
   ```
2. `frontend/src/modules/sidebar-left/index.ts`: ekspor `registerSidebarSection`, `sidebarSections`, dan tipe `SidebarSectionSpec`.
3. `AgentSidebar.svelte`: impor `sidebarSections` dari `./registry`; `const sections = sidebarSections();` lalu di markup, tepat setelah `<WorkspaceHeader {onerror} />` dan sebelum blok daftar sesi, render `{#each $sections as s (s.id)}{@const S = s.component}<S {onerror} />{/each}`. Tanpa section terdaftar tidak ada keluaran. Jangan mengubah gaya lain.

## Langkah 5: verifikasi guard

1. `go build ./...` lulus.
2. `cd frontend && npm run check && npm run build` lulus.
3. Konfirmasi tidak ada perubahan di luar lima berkas di atas (`git status`).

Hasil: fitur paralel (`thread-foundation`, `thread-master-daemon`, `thread-ui`) dapat dimulai.
