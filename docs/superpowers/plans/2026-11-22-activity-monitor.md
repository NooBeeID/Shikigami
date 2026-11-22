# Monitor Workflow dan Sub Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menu di bilah atas yang mendaftar workflow dan sub agent Claude yang sedang berjalan di workspace aktif, dengan identitas dari `args` (mis. nama tiket), sesi asal, lama berjalan, dan progres bila terbaca.

**Architecture:** Hook Claude per sesi (jalur `internal/status` yang sudah ada) memberi tahu daemon apa yang mulai dan berakhir. Paket baru `internal/runs` menyimpan daftar di memori dan menerbitkan `runs.changed`. Detail (fase, checklist) dibaca fail-soft dari berkas Claude hanya saat menu terbuka lewat `runs.detail`. UI Svelte menampilkan dropdown bergaya lonceng.

**Tech Stack:** Go (daemon, IPC `internal/ipc`), Svelte 5 + TypeScript (frontend), modul Go `github.com/NooBeeID/shikigami`.

**Spec:** `docs/superpowers/specs/2026-11-22-activity-monitor-design.md`

## Global Constraints

- Tanpa dependency baru (Go maupun npm).
- Tanpa polling saat menu tertutup; pembacaan berkas detail hanya saat dropdown terbuka.
- `ClassifyEvent` tidak boleh berubah perilakunya untuk event yang sudah ada; event baru (`PreToolUse`, `SubagentStart`) tidak mengubah status sesi.
- Daftar hanya di memori daemon, tanpa persistensi.
- Lingkup: workspace aktif saja. Hanya provider Claude.
- Kegagalan membaca berkas Claude tidak boleh menggagalkan daftar (fail-soft).
- Teks UI berbahasa Indonesia, komentar kode berbahasa Indonesia (mengikuti repo).
- Repo belum punya tes Go maupun runner tes frontend. Logika murni ditaruh di Go dan dites dengan `go test`; frontend diverifikasi dengan `npx svelte-check`, `npm run build`, dan uji manual.

## Review Focus

- Workflow berakhir tanpa event selesai (Esc, crash Claude): entri tidak boleh tampil "berjalan" selamanya; rekonsiliasi menandai atau membuangnya (Task 4).
- `args` kosong, bukan objek, atau `ticket` berupa path (`docs/tickets/x.md`): identitas jatuh ke `x` atau ke nama workflow saja, tanpa panic (Task 1).
- `args` sangat panjang: identitas dipotong, daftar tidak membengkak (Task 1).
- Payload hook kosong, bukan JSON, atau tanpa `tool_use_id`: diabaikan tanpa panic (Task 1).
- Berkas `transcript_path` hilang atau tidak terbaca: `runs.detail` tetap mengembalikan daftar tanpa detail (Task 4).
- Dua workflow dengan nama sama tetapi tiket berbeda tampil sebagai dua baris dengan identitas berbeda (Task 1).
- Sesi berakhir: semua entrinya dibuang; sesi di workspace lain tidak ikut tampil (Task 3).

---

### Task 0: Spike — payload hook dan berkas Claude saat berjalan (SELESAI, commit b5064ed)

Hasil dan fixture: `docs/superpowers/specs/2026-11-22-activity-monitor-spike.md` dan `internal/runs/testdata/`. Task 1–4 di bawah sudah disesuaikan dengan hasilnya; baca dokumen spike bila ragu.

**Files:**
- Create: `docs/superpowers/specs/2026-11-22-activity-monitor-spike.md`
- Create: `internal/runs/testdata/` (fixture hasil spike, lihat langkah)

Spike bersifat sekali pakai; tanpa tes. Hasilnya menentukan bentuk Task 1, 2, dan 4.

- [ ] **Step 1: Catat payload hook asli.** Buat berkas settings sementara di scratchpad yang memasang hook `PreToolUse` (matcher `Workflow|Agent|Task`), `PostToolUse`, `SubagentStart`, `SubagentStop`, `Stop`, masing-masing menjalankan `cat >> <scratchpad>/hooks.log` (stdin JSON). Jalankan `claude --settings <berkas>` di folder proyek ini, minta ia menjalankan satu sub agent (tool Agent) dan satu workflow kecil (`Workflow` dengan skrip dua agent), dengan `args` berisi `{ticket: "x"}`.
- [ ] **Step 2: Jawab empat pertanyaan di dokumen spike**, dengan cuplikan payload nyata: (a) nama tool sub agent (`Agent` atau `Task`) dan field `tool_input` yang memuat `description`; (b) apakah payload `SubagentStart/Stop` memuat id yang bisa dicocokkan dengan `tool_use_id` dari `PreToolUse`; (c) apakah `workflows/wf_*.json` sudah ada dan berisi `status`/fase selama workflow berjalan, dan nilai `status` saat berjalan; (d) sinyal apa yang menandai workflow berakhir (event hook atau berubahnya `status` berkas). Salin juga satu contoh `subagents/agent-*.meta.json` dan potongan transkrip `.jsonl` yang memuat checklist (TodoWrite/TaskCreate).
- [ ] **Step 3: Simpan fixture.** Taruh payload nyata (tanpa data sensitif) sebagai `internal/runs/testdata/pretooluse_workflow.json`, `pretooluse_agent.json`, `subagentstart.json`, `subagentstop.json`, `wf_running.json` (atau catat "tidak ada"), `wf_completed.json`, `agent.meta.json`, `agent_transcript.jsonl`.
- [ ] **Step 4: Keputusan cabang.** Tulis di dokumen spike satu baris per cabang: bila `wf_*.json` tidak ada selama berjalan, `Progress` workflow selalu nil dan Task 4 hanya mengonfirmasi sub agent; bila payload tidak bisa dihubungkan ke agent, sub agent diidentifikasi dari `PreToolUse` saja dan berakhir saat `SubagentStop` pertama dengan `agent_type` cocok atau saat rekonsiliasi.
- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/specs/2026-11-22-activity-monitor-spike.md internal/runs/testdata
git commit -m "docs(runs): hasil spike payload hook dan berkas Claude"
```

### Task 1: Registry run di memori (`internal/runs`)

**Files:**
- Create: `internal/runs/runs.go`
- Test: `internal/runs/runs_test.go`

**Interfaces:**
- Produces:
  - `type Kind string` dengan konstanta `KindWorkflow = "workflow"`, `KindSubagent = "subagent"`.
  - `type Run struct { ID, SessionID string; Kind Kind; Name, Identity string; TaskID, RunID, AgentID string; StartedAt int64; Confirmed bool; Progress *Progress }` (tag JSON camelCase, `omitempty` untuk `TaskID/RunID/AgentID/Progress`; `StartedAt` unix milidetik). `ID`: `tool_use_id` untuk workflow, `agent_id` untuk sub agent.
  - `type AgentLine struct { Label, Phase string; Done bool }`.
  - `type Progress struct { PhaseTitle string; Done, Total int; Agents []AgentLine }` (tag JSON, `omitempty`). Workflow: `Done/Total` = agent selesai/dimulai dari journal. Sub agent: `Done/Total` = checklist.
  - `type Registry struct{ ... }`; `func NewRegistry() *Registry`.
  - `func (r *Registry) Apply(sessionID, event string, payload map[string]any) (changed bool)` — aturan hasil spike: (1) `PreToolUse` dengan `tool_name=Workflow` membuat run workflow (`ID=tool_use_id`, identitas dari `tool_input.args`); `tool_name=Agent` menyimpan `description` sebagai pending (belum jadi run). (2) `PostToolUse` Workflow melengkapi run itu dari `tool_response`: `Name=workflowName`, `TaskID=taskId`, `RunID=runId` (ini peluncuran, BUKAN selesai). `PostToolUse` Agent mengaitkan `tool_response.agentId` ke pending-nya. (3) `SubagentStart` dengan `agent_type != "workflow-subagent"` membuat run sub agent (`ID=AgentID=agent_id`, identitas dari pending yang cocok, jatuh ke pending tertua yang belum diklaim; `agent_type` workflow-subagent diabaikan). (4) `SubagentStop` menghapus run sub agent dengan `agent_id` itu. (5) `Stop` dan `SubagentStop` membawa `background_tasks` (daftar `{id,type,status,...}`): workflow sesi itu yang `TaskID`-nya terisi tetapi tidak ada di daftar (type workflow) dihapus. Event lain diabaikan (`false`).
  - `func (r *Registry) List(sessionIDs map[string]bool) []Run` — urut `StartedAt` naik; `nil` map = semua sesi.
  - `func (r *Registry) Forget(sessionID string) (changed bool)`.
  - `func (r *Registry) TranscriptPath(sessionID string) string` — `transcript_path` terakhir yang terlihat dari payload sesi itu.
  - `func (r *Registry) Remove(id string) bool` dan `func (r *Registry) MarkConfirmed(id string, p *Progress)` (dipakai Task 4).
  - `func identityOf(kind Kind, toolInput map[string]any) string` (tidak diekspor, dites langsung).
- Aturan identitas (dari spec): workflow — bila `args.ticket` ada, nilainya tanpa path dan `.md`; bila tidak, pasangan `key=value` pertama dari `args` (urut key alfabet), dipotong 40 rune dengan `…`; `args` kosong atau bukan objek → string kosong. Sub agent — `description`, dipotong 60 rune. `Name` workflow baru tersedia dari `PostToolUse` (`workflowName`); sebelum itu `"workflow"`.

- [ ] **Step 1: Tulis tes gagal** di `runs_test.go`, memakai fixture Task 0 dan literal kecil: `TestIdentityTicketStripsPath` (`{"ticket":"docs/tickets/T01.md"}` → `"T01"`), `TestIdentityFallsBackToFirstPair`, `TestIdentityTruncates` (args 200 rune → ≤ 40 rune berakhiran `…`), `TestIdentityNonObjectArgs` (string/nil/array → `""`), `TestApplyStartAddsRun` (PreToolUse workflow → `List` berisi 1 run `Kind=workflow`, `Identity="T01"`), `TestTwoWorkflowsSameNameDifferentTicket` (dua `PreToolUse` nama sama, tiket T01 dan T02 → dua run berbeda dengan identitas berbeda), `TestSubagentStopRemoves`, `TestWorkflowLaunchFillsNameAndTaskID` (fixture `pretooluse_workflow.json` lalu `posttooluse_workflow.json`), `TestWorkflowRemovedWhenGoneFromBackgroundTasks` (`stop_with_workflow_running.json` mempertahankan, `stop_no_background.json` menghapus), `TestWorkflowSubagentIgnored` (`subagentstart_workflow.json` tidak menambah run), `TestSubagentStartUsesPendingDescription` (`pretooluse_agent.json` + `subagentstart_agent.json`), `TestApplyMalformedPayload` (map kosong, `tool_input` bukan objek, tanpa `tool_use_id` → tidak panic, `changed=false`), `TestForgetRemovesAllOfSession`, `TestListFiltersBySessions`, `TestUnrelatedEventIgnored` (`Notification` → `false`).
- [ ] **Step 2: Jalankan** `go test ./internal/runs/ -run . -v`. Harapan: gagal kompilasi (`undefined: NewRegistry`).
- [ ] **Step 3: Implementasikan** `runs.go` sesuai signature di atas. Satu `sync.Mutex`; peta `id → *Run`. `ID` = `tool_use_id` payload (untuk sub agent yang dilaporkan `SubagentStart` tanpa PreToolUse, `ID` = `agent_id`). Sesuaikan korelasi PreToolUse↔SubagentStart dengan keputusan cabang di dokumen spike.
- [ ] **Step 4: Jalankan** `go test ./internal/runs/ -v`. Harapan: semua PASS.
- [ ] **Step 5: Commit**

```bash
git add internal/runs
git commit -m "feat(runs): registry workflow dan sub agent di memori"
```

### Task 2: Pasang hook baru tanpa mengubah klasifikasi status

**Files:**
- Modify: `internal/status/install.go` (`hookEvents`, `writeHooksFile`, komentar "tepat 5")
- Modify: `internal/status/types.go` (konstanta `HookPreToolUse`, `HookSubagentStart`)
- Test: `internal/status/install_test.go`

**Interfaces:**
- Produces: `HookPreToolUse = "PreToolUse"`, `HookSubagentStart = "SubagentStart"`; berkas hooks per sesi memuat `PreToolUse` dengan `"matcher"` yang dibatasi ke nama tool hasil spike (mis. `"Workflow|Agent"`), serta `SubagentStart`, `SubagentStop`. Event lama tetap tanpa matcher.

- [ ] **Step 1: Tulis tes gagal:** `TestWriteHooksFileIncludesRunEvents` (tulis ke `t.TempDir()`, parse JSON; assert kunci `PreToolUse`, `SubagentStart`, `SubagentStop` ada, `PreToolUse[0].matcher` berisi `Workflow`, dan lima event lama tetap ada tanpa `matcher`); `TestClassifyEventUnchanged` (tabel: `UserPromptSubmit/SessionStart→running`, `Notification→waiting`, `Stop→done`; `PreToolUse`, `SubagentStart`, `SubagentStop`, `PostToolUse` → tidak dikenal).
- [ ] **Step 2: Jalankan** `go test ./internal/status/ -run 'TestWriteHooksFile|TestClassifyEvent' -v`. Harapan: `TestWriteHooksFileIncludesRunEvents` GAGAL.
- [ ] **Step 3: Implementasikan:** perluas `hookEvents`; di `writeHooksFile` sisipkan `matcher` hanya untuk `PreToolUse`. Jangan ubah `ClassifyEvent`. Perbarui komentar jumlah event dan catat bahwa matcher mencegah proses `shikigamictl` dijalankan pada setiap pemanggilan tool.
- [ ] **Step 4: Jalankan** `go test ./internal/status/ -v`. Harapan: PASS.
- [ ] **Step 5: Commit**

```bash
git add internal/status
git commit -m "feat(status): pasang hook PreToolUse, SubagentStart, SubagentStop"
```

### Task 3: Sambungkan ke daemon (listener, IPC, event)

**Files:**
- Modify: `internal/status/engine.go` (tambah `AddHookListener`, panggil di `Apply`)
- Create: `internal/runs/ipc.go`
- Create: `cmd/shikigamid/wire_runs.go`
- Test: `internal/status/engine_listener_test.go`

**Interfaces:**
- Consumes: `Registry` dari Task 1; `Engine.Lookup(sessionID) (session.Session, bool)` (sudah ada, memuat `WorkspaceID`).
- Produces:
  - `func (e *Engine) AddHookListener(fn func(sessionID, event string, payload map[string]any)) (cancel func())` — dipanggil di `Apply` setelah sink aktivitas, tiap listener dibungkus `recover`.
  - IPC `runs.list` params `{ "workspaceId": string }` → `Run[]` milik sesi di workspace itu.
  - Event IPC `runs.changed` payload `{ "sessionId": string, "runs": Run[] }` (seluruh run sesi itu setelah perubahan; `runs` kosong = sesi tak punya run).
  - `func RegisterIPC(srv *ipc.Server, reg *Registry, eng *status.Engine)` di `internal/runs/ipc.go` (hanya `runs.list` di task ini; `runs.detail` ditambah Task 4).

- [ ] **Step 1: Tulis tes gagal** `TestAddHookListenerReceivesEvents` (daftarkan listener, panggil `Apply` pada sesi terdaftar, assert dipanggil dengan event dan payload yang sama), `TestHookListenerPanicIsContained` (listener panic → `Apply` tetap mengembalikan nil), `TestHookListenerCancel` (setelah `cancel()` tidak dipanggil lagi). Tes `runs` untuk `list` per workspace: `TestListOnlyActiveWorkspaceSessions` memakai `Registry.List` dengan peta sesi — sesi workspace lain tidak muncul.
- [ ] **Step 2: Jalankan** `go test ./internal/status/ -run 'Listener' -v`. Harapan: gagal kompilasi (`AddHookListener` belum ada).
- [ ] **Step 3: Implementasikan** `AddHookListener` (slice listener di bawah `e.mu`). Di `wire_runs.go` ikuti pola `wire_activity.go`: `RegisterModule("runs", ...)`, di `d.OnReady` ambil `status.engine`, daftarkan listener yang memanggil `reg.Apply` lalu, bila `changed`, `srv.Publish("runs.changed", ...)`; `eng.Subscribe` pada `StatusEvent` dengan `Source == SourceExit` memanggil `reg.Forget` dan menerbitkan `runs.changed` dengan `runs: []`. `runs.list` memakai `Engine.Lookup` untuk menyaring sesi per `workspaceId`.
- [ ] **Step 4: Jalankan** `go build ./... && go test ./internal/... -v`. Harapan: build sukses, tes PASS. Lalu cek manual: jalankan daemon, panggil `shikigamictl` atau `ipc` client `runs.list` → `[]`.
- [ ] **Step 5: Commit**

```bash
git add internal/status internal/runs cmd/shikigamid
git commit -m "feat(runs): hubungkan registry ke hook, IPC runs.list dan runs.changed"
```

### Task 4: Detail dan rekonsiliasi dari berkas Claude (fail-soft)

**Files:**
- Create: `internal/runs/detail.go`
- Modify: `internal/runs/ipc.go` (daftarkan `runs.detail`)
- Test: `internal/runs/detail_test.go`

**Interfaces:**
- Consumes: `Registry.TranscriptPath`, `Registry.MarkConfirmed`, `Registry.Remove` (Task 1); fixture Task 0.
- Produces:
  - `type Detail struct { Found, Finished bool; Progress *Progress }`.
  - `func ReadDetail(transcriptPath string, run Run) Detail` — tidak pernah error dan tidak panic. Folder sesi Claude = `transcriptPath` tanpa akhiran `.jsonl`. Workflow (butuh `run.RunID`): baca `<folder>/subagents/workflows/<runId>/journal.jsonl` — `Found=true` bila ada; `Progress.Agents` dari baris `started` (label, phase) yang ditandai `Done` bila ada baris `result` ber-`agentId` sama; `Progress.PhaseTitle` = `phase` agent terakhir dimulai; `Done/Total` = jumlah result/started; `Finished=true` bila `<folder>/workflows/wf_<runId>.json` ada dan `status` bukan berjalan (cadangan; penanda utama selesai adalah `background_tasks` di Registry). Sub agent: baca `<folder>/subagents/agent-<agent_id>.meta.json` (`Found`, `Finished` dari `finished`) dan checklist terakhir dari `<folder>/subagents/agent-<agent_id>.jsonl` (`Done/Total`; TIDAK terverifikasi di spike, upaya terbaik: baris assistant dengan `tool_use` bernama `TodoWrite`; bila tak ada, `Progress=nil`).
  - IPC `runs.detail` params `{ "workspaceId": string }` → `Run[]` setelah rekonsiliasi: run dengan `Finished` dibuang; run dengan `Found` ditandai `Confirmed` dan diberi `Progress`; run tak ditemukan yang sesinya sudah tak ada dibuang; sisanya `Confirmed=false`. Setelah itu terbitkan `runs.changed` per sesi yang berubah.

- [ ] **Step 1: Tulis tes gagal** memakai fixture: `TestReadDetailWorkflowFinished` (susun folder sesi sementara di `t.TempDir()` dari `wf_completed.json` → `Finished=true`), `TestReadDetailWorkflowRunning` (`journal_running.jsonl`: `Progress.Total=2`, `Done=1`, `PhaseTitle="Probe"`, dua `Agents`, satu `Done`), `TestReadDetailWorkflowNoJournalMeansUnconfirmed` (`Found=false`), `TestReadDetailSubagentChecklist` (`agent_transcript.jsonl` → `Done`/`Total` sesuai isi), `TestReadDetailMissingPath` (path tak ada/tak terbaca → `Detail{}` tanpa panic), `TestReadDetailCorruptJSON` (berkas rusak → `Detail{}`).
- [ ] **Step 2: Jalankan** `go test ./internal/runs/ -run ReadDetail -v`. Harapan: gagal kompilasi (`undefined: ReadDetail`).
- [ ] **Step 3: Implementasikan** `detail.go` dan handler `runs.detail`. Parser toleran: field tak dikenal diabaikan, tipe tak sesuai → `Detail{}`.
- [ ] **Step 4: Jalankan** `go test ./internal/runs/ -v && go build ./...`. Harapan: PASS dan build sukses.
- [ ] **Step 5: Commit**

```bash
git add internal/runs
git commit -m "feat(runs): rekonsiliasi dan detail progres dari berkas Claude"
```

### Task 5: Store frontend

**Files:**
- Create: `frontend/src/modules/runs/store.ts`
- Create: `frontend/src/modules/runs/types.ts`
- Modify: tempat `startStatusSync()` dipanggil (cari dengan `grep -rn startStatusSync frontend/src`) — panggil `startRunsSync()` di sampingnya.

**Interfaces:**
- Consumes: `call`, `onEvent` dari `frontend/src/lib/ipc.ts`; `activeWorkspaceId` dari `lib/stores`.
- Produces:
  - `export interface Run { id: string; sessionId: string; kind: 'workflow'|'subagent'; name: string; identity: string; parentId?: string; startedAt: number; confirmed: boolean; progress?: Progress }` dan `Progress { phase?: number; phaseTotal?: number; phaseTitle?: string; done?: number; total?: number }`.
  - `export const runs: Readable<Run[]>` (hanya workspace aktif, urut `startedAt`), `export const runsCount: Readable<number>` (hanya yang `kind==='workflow'` atau tanpa `parentId`).
  - `export function startRunsSync(): () => void` — idempoten, berlangganan `runs.changed` lalu memuat `runs.list`; muat ulang saat workspace aktif berganti.
  - `export async function refreshDetail(): Promise<void>` — memanggil `runs.detail`, mengganti isi store.

- [ ] **Step 1:** Implementasikan store mengikuti pola `modules/status/store.ts` (berlangganan dahulu, baru muat awal; abaikan payload tak valid).
- [ ] **Step 2: Verifikasi** `cd frontend && npx svelte-check && npm run build`. Harapan: 0 error.
- [ ] **Step 3: Commit**

```bash
git add frontend/src
git commit -m "feat(runs): store frontend untuk daftar run"
```

### Task 6: Menu `RunsMenu` di bilah atas

**Files:**
- Create: `frontend/src/modules/runs/RunsMenu.svelte`
- Create: `frontend/src/modules/runs/index.ts` (mendaftarkan `registerTopbarItem({ id: 'runs', component: RunsMenu, align: 'right', order: 9 })`, meniru `modules/notifications/index.ts`)
- Modify: `frontend/src/main.ts` atau berkas impor modul (cek bagaimana `modules/notifications` diimpor) agar modul baru ikut termuat.

**Interfaces:**
- Consumes: `runs`, `runsCount`, `refreshDetail` (Task 5); `focusOrOpenSession`, `sessionTitle` dari `../shell`; `sessions` dari `../../lib/stores`.

- [ ] **Step 1:** Tulis `RunsMenu.svelte` meniru pola `notifications/Bell.svelte` (toggle, tutup saat klik luar dan Escape, dropdown `position: absolute`). Isi: tombol ikon dengan lencana `$runsCount` bila > 0; dropdown berjudul "Aktivitas (N)"; tiap workflow satu baris `name · identity`, nama sesi (`sessionTitle`), lama berjalan (`relTime`-gaya lonceng, diperbarui hanya saat terbuka), progres `fase p/total` bila ada; sub agent milik workflow bersarang di bawahnya, tertutup secara bawaan dengan jumlah; sub agent mandiri datar dengan `☑ done/total` bila ada; entri `confirmed=false` diberi penanda "belum terkonfirmasi". Klik baris → `focusOrOpenSession(sessionId)` lalu tutup. Baris yang sesinya tak ada di `$sessions` redup dan `disabled`. Keadaan kosong: "Tidak ada workflow atau sub agent berjalan".
- [ ] **Step 2:** Saat dropdown terbuka: panggil `refreshDetail()` sekali, lalu `setInterval` tiap 3 detik; hentikan dan bersihkan saat ditutup (`$effect` dengan fungsi pembersih).
- [ ] **Step 3: Verifikasi** `cd frontend && npx svelte-check && npm run build`. Harapan: 0 error.
- [ ] **Step 4: Uji manual** (`make build`, jalankan app): jalankan dua `/plan-from-tiket` dengan tiket berbeda di dua sesi → dua baris dengan identitas tiket berbeda; klik baris melompat ke pane sesinya; tutup menu → pastikan tidak ada pembacaan berkas (cek log daemon tidak mencatat `runs.detail`); akhiri sesi → barisnya hilang.
- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(runs): menu Aktivitas di bilah atas"
```
