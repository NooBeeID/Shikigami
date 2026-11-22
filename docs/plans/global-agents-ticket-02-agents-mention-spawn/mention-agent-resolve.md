# Plan: Resolusi mention ke agent dan spawn sesi

## Metadata

- Tiket: `docs/tickets/global-agents-ticket-02-agents-mention-spawn.md` (fitur `mention-agent-resolve`)
- Overview: `docs/plans/global-agents-ticket-02-agents-mention-spawn/overview.md` (kontrak bagian 2, aturan resolusi 2.3, IPC 2.4)
- Spec: `docs/superpowers/specs/2026-11-22-global-agents-design.md`
- ownedPaths: `internal/mention/`, `cmd/shikigamid/wire_mention.go`
- Depends on foundation: tidak (tidak ada `00-foundation.md`). Bergantung pada tiket 1 (`internal/agents`: `Agent`, `Category`, `Store.Resolve`, `ErrNotFound`; `session.SessionSpec.Model`). Keduanya BELUM ada di working tree saat plan ditulis. Bila saat implementasi `internal/agents` atau `SessionSpec.Model` belum ada, itu pelanggaran dependensi: hentikan dan laporkan, jangan ditambal.

## Tujuan

`@nama-agent` atau `@kategori` (di `mention.send` dari UI dan di tool MCP `send_message`) men-spawn sesi baru dari agent global di project konteks pengirim bila belum ada sesinya, lalu antre brief (peran kategori + pesan) lewat `Queue` yang ada. Sesi existing (nama biasa atau sesi hasil spawn berlabel agent) selalu diprioritaskan sehingga tidak ada spawn ganda. Bila `agents.store` tidak tersedia, mention lama tetap jalan tanpa panik.

## Path yang dimiliki

HANYA ini yang boleh disentuh:
- `internal/mention/` (berkas baru `agents.go`, `resolve.go`; ubah `types.go`, `queue.go`, `ipc.go`, `tools.go`)
- `cmd/shikigamid/wire_mention.go`

Jangan menyentuh `internal/agents/`, `internal/session/`, `internal/provider/`, `internal/thread/`, `frontend/`, maupun file wiring lain. `internal/mention/mcp.go` dan `install.go` tidak perlu diubah. Jangan menambah unit test; berkas `*_test.go` yang ada (`confirm_test.go`) tidak boleh dirusak, sesuaikan seperlunya bila signature berubah (seharusnya tidak, semua perubahan bersifat aditif).

## Kontrak

Dari tiket 1 (dipakai, jangan didefinisikan ulang): `agents.Agent{ID, Name, CategoryID, ProviderID, Model string; IsDefault bool}`, `agents.Category{ID, Name, Role string; Builtin bool}`, `(*agents.Store).Resolve(ref) (agents.Agent, agents.Category, error)`, `agents.ErrNotFound`, `session.SessionSpec.Model`. `internal/mention` mengimpor `internal/agents` hanya untuk tipe dan `ErrNotFound`.

Disediakan (bentuk persis, melengkapi overview 2.2):

```go
// agents.go
type AgentResolver interface {
    Resolve(ref string) (agents.Agent, agents.Category, error)
}
type SessionSpawner interface {
    Create(spec session.SessionSpec) (*session.Session, error)
}
type ProjectDirectory interface {
    ProjectDir(workspaceID, projectID string) string
}
type SpawnLabels struct { /* mu sync.Mutex; byID map[string]string */ }
func NewSpawnLabels() *SpawnLabels
func (l *SpawnLabels) Set(sessionID, agentName string)
func (l *SpawnLabels) Get(sessionID string) (agentName string, ok bool)
func (l *SpawnLabels) Delete(sessionID string)          // cocok sebagai callback exit: func(string)
func (l *SpawnLabels) SessionsOf(agentName string) []string

type ProjectContext struct{ WorkspaceID, ProjectID, Cwd, FromSessionID string }

// TargetRouter dipenuhi *Queue; dipakai tools.go lewat Runtime.Router.
type TargetRouter interface {
    ResolveOrSpawn(ctx context.Context, pc ProjectContext, ref, text, source string) (Message, bool, error)
}
```

Field baru:
- `QueueConfig`: `Agents AgentResolver`, `Spawner SessionSpawner`, `Projects ProjectDirectory`, `Labels *SpawnLabels` (semua boleh nil; `Agents == nil` atau `Spawner == nil` berarti alur agent dilewati).
- `Runtime`: `Router TargetRouter` (nil -> tool memakai jalur lama `Namer.Resolve` + `Enqueuer`). Keputusan plan: tool MCP memanggil `Router` (satu `*Queue` yang sama dengan jalur IPC) alih-alih `Runtime.Agents`/`Runtime.Spawner` terpisah, supaya kunci anti spawn ganda dibagi antara UI dan MCP. Pada wiring, `Runtime.Router: q` menggantikan butir "`Runtime.Agents`/`Runtime.Spawner`" di overview 5.1.
- `Namer`: `func (n *Namer) SetLabels(l *SpawnLabels)`.
- `(*Queue).ResolveOrSpawn` (semantik: mengembalikan pesan yang sudah diantre, `spawned` true bila sesi baru dibuat; sumber `SourceUser` -> from "" ; `SourceAgent` -> from = `pc.FromSessionID`).
- `caller` (tools.go) tambah `ProjectID`, `Cwd`.
- IPC `mention.send`: param opsional `projectId`; hasil = `Message` + `spawned` + `spawnedSession` (flat, `omitempty`), tag JSON persis `spawned` dan `spawnedSession`.

Galat Indonesia (teks persis):
- Tanpa project konteks: `Tidak ada project konteks untuk membuka sesi agent @<ref>; pilih project dulu` (lolos `toIPCError` sebagai `ipc.ErrInvalid` dengan pesan sama, tanpa spawn).
- Gagal spawn/antre: `Gagal membuka sesi agent @<ref>: <sebab>`.
- Galat resolver selain `agents.ErrNotFound` (mis. kategori tanpa default) diteruskan teksnya apa adanya dan dipetakan ke `ipc.ErrInvalid`. `agents.ErrNotFound` menjadi `ErrNotFound` mention.

## Langkah

### 1. `internal/mention/agents.go` (baru)
Berisi `AgentResolver`, `SessionSpawner`, `ProjectDirectory`, `ProjectContext`, `TargetRouter`, `SpawnLabels` (implementasi map + `sync.Mutex`; `SessionsOf` mengembalikan id berurut stabil, `Delete` aman untuk id tak dikenal), serta:
- `type userFacingError struct{ err error }` dengan `Error()` mengembalikan teks `err` apa adanya, `Unwrap()` mengembalikan `err`, dan `Is(target) bool { return target == ErrInvalid }`. Dipakai membungkus galat resolver non-NotFound dan galat "tidak ada project konteks" agar teks tidak berubah tetapi `toIPCError` memetakan ke `ipc.ErrInvalid` (cabang `errors.Is(err, ErrInvalid)` yang sudah ada; tidak perlu mengubah `toIPCError`).
- `func buildBrief(cat agents.Category, text string) string` menghasilkan persis `"[Peran: " + cat.Name + "]\n" + cat.Role + "\n\n--- Pesan ---\n" + text`.
- `var _ AgentResolver = (*agents.Store)(nil)` boleh ditambahkan untuk pemeriksaan kompilasi.
Komentar bahasa Indonesia.

### 2. `types.go`: Namer membaca label
- Tambah field `labels *SpawnLabels` pada `Namer`; metode `SetLabels(l *SpawnLabels)` (nil boleh).
- Di `snapshot`, ganti `base := baseName(s.ProviderID, n.projectName(s))` menjadi: bila `n.labels != nil` dan `name, ok := n.labels.Get(s.ID)` -> `baseName(name, n.projectName(s))`; selain itu perilaku lama. Penghitung duplikat (`-2`, `-3`) tetap memakai kunci `workspace + "\x00" + base` yang sama, sehingga konsisten untuk `NameOf`, `Resolve`, `List`.
- Tambah `Router TargetRouter` pada `Runtime` (komentar: nil -> jalur lama).
- `NewNamer` tidak berubah signature.

### 3. `queue.go`: field konfigurasi
Tambah `Agents`, `Spawner`, `Projects`, `Labels` pada `QueueConfig`. Tambah ke `Queue` field `spawnMu sync.Mutex` dan `spawnLocks map[string]*sync.Mutex` (diinisialisasi di `NewQueue`). Tambah `var _ TargetRouter = (*Queue)(nil)`. `Enqueue`, `pump`, dan lain-lain tidak diubah. Bila `cfg.Labels != nil` dan `cfg.Namer != nil`, `NewQueue` boleh memanggil `cfg.Namer.SetLabels(cfg.Labels)` bila Namer belum berlabel (opsional; wiring juga memanggilnya).

### 4. `resolve.go` (baru): `ResolveOrSpawn`
`func (q *Queue) ResolveOrSpawn(ctx context.Context, pc ProjectContext, ref, text, source string) (Message, bool, error)`. Alur:

1. Validasi: `strings.TrimSpace(ref) == ""` -> `ErrInvalid`. `from := ""`; bila `source == SourceAgent` `from = pc.FromSessionID`. Helper lokal `enqueue(to string)` memanggil `q.Enqueue(ctx, from, to, text, source)`.
2. Sesi existing: `id, err := q.cfg.Namer.Resolve(pc.WorkspaceID, ref)`.
   - `err == nil`: bila `from != "" && id == from` kembalikan `errors.New("tidak dapat mengirim pesan ke sesi sendiri")`; selain itu `Enqueue` dengan teks asli (tanpa brief), `spawned=false`.
   - `ErrCrossWorkspace` atau `ErrInvalid`: kembalikan (tidak jatuh ke spawn).
   - `ErrNotFound`: lanjut.
3. Bila `q.cfg.Agents == nil || q.cfg.Spawner == nil`: kembalikan `ErrNotFound` (perilaku lama).
4. Sesi berlabel dengan ref mentah: `q.findLabeled(pc, normName(ref))` -> bila ada, `Enqueue` ke sesi itu (teks asli, tanpa brief), `spawned=false`.
5. `ag, cat, err := q.cfg.Agents.Resolve(ref)`: `errors.Is(err, agents.ErrNotFound)` -> `ErrNotFound`; galat lain -> `&userFacingError{err}`. Bila `ag.Name` (slug via `normName`) berbeda dari ref, ulangi `findLabeled` dengan `ag.Name` (kasus ref = kategori); bila ada, `Enqueue` ke sesi itu tanpa brief.
6. `user_only`: `source == SourceAgent && q.Mode() == ModeUserOnly` -> `ErrUserOnly` SEBELUM spawn (tidak ada sesi yatim).
7. Konteks project: `projectID := pc.ProjectID`, `cwd := pc.Cwd`. Bila `cwd == ""` dan `projectID != ""` dan `q.cfg.Projects != nil`: `cwd = q.cfg.Projects.ProjectDir(pc.WorkspaceID, projectID)`. Bila `projectID == "" && cwd == ""` -> `&userFacingError{fmt.Errorf("Tidak ada project konteks untuk membuka sesi agent @%s; pilih project dulu", ref)}` (tanpa spawn). Bila projectID ada tetapi cwd tetap kosong, tetap spawn dengan Cwd kosong (default manager); catat di komentar.
8. Anti spawn ganda: kunci `pc.WorkspaceID + "|" + projectID + "|" + normName(ag.Name)`; ambil mutex per kunci dari `spawnLocks` (dibuat di bawah `spawnMu`), `Lock()` lalu `defer Unlock()`. Setelah mendapat kunci, ulangi `findLabeled` (pemeriksaan ganda) dan pakai sesi itu bila sudah ada (permintaan kedua yang menunggu kunci kini menemukan sesi hasil spawn pertama). Peta kunci boleh tumbuh (jumlah kunci kecil); tidak perlu refcount.
9. Spawn: `sess, err := q.cfg.Spawner.Create(session.SessionSpec{WorkspaceID: pc.WorkspaceID, ProjectID: projectID, Cwd: cwd, ProviderID: ag.ProviderID, Model: ag.Model})`. Galat -> `fmt.Errorf("Gagal membuka sesi agent @%s: %w", ref, err)` (tanpa label). Sukses: `q.cfg.Labels.Set(sess.ID, normName(ag.Name))` (bila `Labels != nil`) SEBELUM antre agar `Namer` langsung menamai `<agent>-<project>`.
10. Antre brief: `brief := buildBrief(cat, text)`. Loop coba ulang `Enqueue(ctx, from, sess.ID, brief, source)` sampai maksimum 2 detik dengan interval 100 ms hanya selama galat `ErrNotFound` (sesi belum `Running`); hormati `ctx.Done()`. Galat lain (`ErrConflict`, `ErrUserOnly`, dll.) langsung dikembalikan apa adanya. Bila tetap gagal setelah batas waktu: cek `q.cfg.Sessions.Get(sess.ID)`; bila sesi tidak ada atau bukan `Running`, `Labels.Delete(sess.ID)`; kembalikan `fmt.Errorf("Gagal membuka sesi agent @%s: sesi tidak berjalan", ref)`. Sukses: kembalikan `(msg, true, nil)`.
11. Mode `confirm_agent`: tidak ada cabang khusus. Spawn tetap dilakukan segera dan `Queue.Enqueue` otomatis menandai pesan pertama `awaiting_confirm` untuk `SourceAgent`. Tulis komentar di langkah 10 bahwa sesi kosong menunggu konfirmasi (implementasi sederhana yang diterima tiket).
12. Helper `findLabeled(pc ProjectContext, agentName string) string`: iterasi `Labels.SessionsOf(agentName)`, ambil sesi via `Sessions.Get`, saring `Status == Running` dan `WorkspaceID == pc.WorkspaceID`; prioritas `ProjectID == pc.ProjectID` (bila `pc.ProjectID != ""`), selain itu sesi tertua (`CreatedAt`, lalu `ID`) di workspace yang sama. Kembalikan "" bila tidak ada; kembalikan "" juga bila `Labels == nil`. Penting: bila `pc.ProjectID` kosong, jangan memilih sesi project lain secara acak di luar aturan di atas (aturan tetap: project konteks dulu, lalu project lain di workspace yang sama).

### 5. `ipc.go`: `mention.send`
- Tambah `ProjectID string \`json:"projectId"\`` pada struct params.
- Pertahankan penentuan `ws` seperti sekarang. Bangun `pc := ProjectContext{WorkspaceID: ws, ProjectID: p.ProjectID}`. Bila `p.From != ""`: `fromID, ferr := namer.Resolve(ws, p.From)` (galat -> `toIPCError`), `pc.FromSessionID = fromID`, ambil `q.cfg.Sessions.Get(fromID)` dan isi `pc.ProjectID`/`pc.Cwd` dari sesi pengirim bila `p.ProjectID` kosong; sumber `SourceAgent`. Bila `p.From == ""`: sumber `SourceUser`.
- Panggil `msg, spawned, err := q.ResolveOrSpawn(ctx, pc, p.To, p.Text, source)` menggantikan `namer.Resolve` + `q.Enqueue`; galat lewat `toIPCError`.
- Kembalikan struct:
  ```go
  type sendResult struct {
      Message
      Spawned        bool   `json:"spawned,omitempty"`
      SpawnedSession string `json:"spawnedSession,omitempty"`
  }
  ```
  `SpawnedSession = namer.NameOf(msg.To)` bila `spawned`. Params lama tidak berubah; `mention.names` dan metode lain tidak berubah. (`IPCOptions` tidak perlu field baru.)

### 6. `tools.go`: MCP `send_message`
- `caller` tambah `ProjectID`, `Cwd`; `resolveCaller` mengisinya dari `s.ProjectID`, `s.Cwd`.
- Deskripsi tool `send_message`: tambahkan kalimat bahwa target dapat berupa nama agent atau kategori dan sesi baru akan dibuka di project sesi pemanggil bila belum ada. Deskripsi parameter `to`: `"Nama mention atau id sesi target; boleh juga nama agent atau kategori (membuka sesi baru di project Anda bila belum ada)"`. `list_sessions` dan `read_output` tidak berubah.
- `toolSendMessage`: validasi `to`/`text` tetap. Bila `rt.Router != nil`: `msg, spawned, err := rt.Router.ResolveOrSpawn(ctx, ProjectContext{WorkspaceID: c.WorkspaceID, ProjectID: c.ProjectID, Cwd: c.Cwd, FromSessionID: c.SessionID}, a.To, a.Text, SourceAgent)`; galat -> `toolError(err)` (otomatis `isError` ke agen pengirim, termasuk teks "Gagal membuka sesi agent ..."). Bila `rt.Router == nil`, jalur lama (`rt.Enqueuer` + `rt.Namer.Resolve`) dipertahankan utuh.
- Hasil JSON tetap memuat `queued`, `delivered`, `position` dengan perhitungan sama; tambahkan `Spawned bool \`json:"spawned,omitempty"\`` dan `Session string \`json:"session,omitempty"\`` (nama sesi baru via `rt.Namer.NameOf(msg.To)` bila `spawned`).
- Mode `confirm_agent` menahan pesan (state `awaiting_confirm` -> `queued: true`), `user_only` menolak tanpa spawn, keduanya sudah dijamin langkah 4.6 dan `Enqueue`.

### 7. `cmd/shikigamid/wire_mention.go`
- Perluas `workspaceDirectory`: ganti `cache map[string]string` menjadi `map[string]projInfo` dengan `type projInfo struct{ name, path string }`; di `refreshLocked` tambah `Path string \`json:"path"\`` pada struct `proj` (nama field sebenarnya di `internal/workspace/types.go`: `Path string \`json:"path"\``, absolut). `ProjectName` mengembalikan `.name`; tambah `ProjectDir(workspaceID, projectID string) string` dengan logika cache/refresh yang sama mengembalikan `.path`. Tambah `var _ mention.ProjectDirectory = (*workspaceDirectory)(nil)`.
- Di `wireMention`, setelah `mgr` didapat:
  - `var resolver mention.AgentResolver`; `if v, ok := d.Lookup("agents.store"); ok { if r, ok2 := v.(mention.AgentResolver); ok2 && r != nil { resolver = r } }`; bila `resolver == nil` -> `d.Logger.Warn("mention: agents.store tidak tersedia, mention agent dinonaktifkan")` (mention lama tetap jalan). Jangan menyimpan typed-nil ke interface.
  - `labels := mention.NewSpawnLabels()`; `namer.SetLabels(labels)`; `hub.add(labels.Delete)` (pembersihan label saat sesi keluar; `hub` sudah menjadi `Runtime.OnSessionExit`).
  - `QueueConfig` ditambah `Agents: resolver`, `Spawner: mgr`, `Projects: dir`, `Labels: labels`.
  - `Runtime` ditambah `Router: q`.
  - `mention.RegisterIPC` dan `mention.Start` dipanggil seperti sekarang (opsi param baru sudah ditangani di dalam `ipc.go`).

## Perubahan

Dibuat:
- `internal/mention/agents.go`
- `internal/mention/resolve.go`

Diubah:
- `internal/mention/types.go` (Namer label, `SetLabels`, `Runtime.Router`)
- `internal/mention/queue.go` (field `QueueConfig` baru, peta kunci spawn di `Queue`)
- `internal/mention/ipc.go` (param `projectId`, `ResolveOrSpawn`, respons `spawned`/`spawnedSession`)
- `internal/mention/tools.go` (`caller`, deskripsi, `toolSendMessage`)
- `cmd/shikigamid/wire_mention.go` (`ProjectDir`, label, lookup `agents.store`, field baru)

## Wiring

Fitur ini sudah menyambungkan dirinya di `wire_mention.go` (path miliknya). Yang diekspos untuk pemeriksaan wiring akhir (overview bagian 5, butir 1 sampai 4 dan 6):
- `mention.AgentResolver` (dipenuhi `*agents.Store` dari `d.Lookup("agents.store")`), `mention.SessionSpawner` (`mgr`), `mention.ProjectDirectory` (`workspaceDirectory`), `mention.SpawnLabels` + `Namer.SetLabels`, `QueueConfig.{Agents,Spawner,Projects,Labels}`, `Runtime.Router`.
- Kontrak JSON `mention.send`: param `projectId`; respons `spawned`, `spawnedSession` (dipakai fitur frontend).
- Catat di laporan: penyimpangan kecil dari overview (`Runtime.Router` menggantikan `Runtime.Agents`/`Runtime.Spawner`; `QueueConfig.Projects` menggantikan `Dir`). Jangan menyentuh file di luar `ownedPaths`; kebutuhan lain dicatat saja.

## Selesai bila

- `go build ./...` dari root repo sukses dan tidak ada impor siklik (`internal/agents` tidak mengimpor `internal/mention`); `go vet ./internal/mention/ ./cmd/shikigamid/` bersih. Guard: build path fitur ini sukses; tanpa unit test baru dan `confirm_test.go` tetap terkompilasi.
- Semua kriteria fitur di tiket terpenuhi secara desain: urutan sesi existing -> sesi berlabel -> `AgentResolver.Resolve` -> spawn dengan `Model` agent; kunci `workspace|project|agent` mencegah spawn ganda; sesi hasil spawn bernama `<agent>-<project>` (duplikat `-2`, `-3`), sesi lain tetap `<provider>-<project>`; nama sesi existing diprioritaskan; `ErrCrossWorkspace` tidak jatuh ke spawn; `user_only` menolak sebelum spawn; `confirm_agent` memberi pesan `awaiting_confirm`; tanpa project konteks galat Indonesia sebagai `ipc.ErrInvalid` tanpa spawn; kegagalan `Create` memberi "Gagal membuka sesi agent @<ref>: <sebab>" tanpa label yatim; label dibersihkan saat sesi keluar; `Agents == nil` tidak panik dan perilaku lama utuh.
- Tidak ada berkas di luar `ownedPaths` yang berubah (`git status` diperiksa), komentar/galat berbahasa Indonesia, identifier berbahasa Inggris.
