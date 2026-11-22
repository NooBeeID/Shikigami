package thread

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/agents"
	"github.com/NooBeeID/shikigami/internal/mention"
	"github.com/NooBeeID/shikigami/internal/provider"
	"github.com/NooBeeID/shikigami/internal/session"
)

// SpawnerConfig konfigurasi Spawner.
type SpawnerConfig struct {
	Manager    *Manager
	Sessions   SessionAPI
	Workspaces WorkspaceSource
	Router     Router
	DataDir    string
	Logger     *slog.Logger
	Agents     AgentResolver // opsional; nil = perilaku lama
}

// SpawnResult adalah hasil Spawn.
type SpawnResult struct {
	Reused      bool
	Participant Participant
	ProjectName string
	Worktree    WorktreeInfo // kosong bila Reused
}

// Spawner membuat peserta peer di worktree sendiri.
type Spawner struct {
	cfg   SpawnerConfig
	log   *slog.Logger
	mu    sync.Mutex
	locks map[string]*sync.Mutex
	once  sync.Once
}

// NewSpawner membuat Spawner; konfigurasi wajib yang kosong menyebabkan panic.
func NewSpawner(cfg SpawnerConfig) *Spawner {
	switch {
	case cfg.Manager == nil:
		panic("thread.NewSpawner: Manager wajib diisi")
	case cfg.Sessions == nil:
		panic("thread.NewSpawner: Sessions wajib diisi")
	case cfg.Workspaces == nil:
		panic("thread.NewSpawner: Workspaces wajib diisi")
	case cfg.Router == nil:
		panic("thread.NewSpawner: Router wajib diisi")
	case cfg.DataDir == "":
		panic("thread.NewSpawner: DataDir wajib diisi")
	}
	lg := cfg.Logger
	if lg == nil {
		lg = slog.Default()
	}
	return &Spawner{cfg: cfg, log: lg, locks: map[string]*sync.Mutex{}}
}

func (sp *Spawner) spawnLock(threadID string) *sync.Mutex {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	l := sp.locks[threadID]
	if l == nil {
		l = &sync.Mutex{}
		sp.locks[threadID] = l
	}
	return l
}

func spawnResolveProvider() (provider.Provider, error) {
	for _, id := range []string{provider.IDClaude, provider.IDCodex} {
		p, ok := provider.Get(id)
		if !ok {
			continue
		}
		if _, err := p.Available(); err != nil {
			continue
		}
		if !p.Capabilities().MCP {
			continue
		}
		return p, nil
	}
	return nil, errors.New("tidak ada provider agent dengan dukungan MCP (claude) yang tersedia untuk peserta thread")
}

// spawnAgent hasil resolusi agent untuk satu pemanggilan Spawn.
type spawnAgent struct {
	Agent    agents.Agent    // kosong bila !HasAgent
	Category agents.Category // kosong bila !HasAgent
	HasAgent bool
}

// spawnResolveAgent meresolusi pilihan agent tanpa memanggil provider.
func (sp *Spawner) spawnResolveAgent(sel AgentSelector) (spawnAgent, error) {
	sel.Agent = strings.TrimSpace(sel.Agent)
	sel.Category = strings.TrimSpace(sel.Category)
	if sp.cfg.Agents == nil {
		if sel.Agent == "" && sel.Category == "" {
			return spawnAgent{}, nil
		}
		return spawnAgent{}, fmt.Errorf("%w: registry agent tidak tersedia; kosongkan agent/category pada thread_spawn", ErrInvalid)
	}
	switch {
	case sel.Agent == "" && sel.Category == "":
		a, c, err := sp.cfg.Agents.Resolve("executor")
		if err == nil {
			return spawnAgent{Agent: a, Category: c, HasAgent: true}, nil
		}
		if agentAbsent(sp.cfg.Agents, "executor", err) {
			return spawnAgent{}, nil
		}
		return spawnAgent{}, fmt.Errorf("agent default Executor: %w", err)
	case sel.Category == "":
		a, c, err := sp.cfg.Agents.Resolve(sel.Agent)
		if err != nil {
			return spawnAgent{}, fmt.Errorf("agent %q: %w", sel.Agent, err)
		}
		return spawnAgent{Agent: a, Category: c, HasAgent: true}, nil
	case sel.Agent == "":
		a, c, err := sp.cfg.Agents.Resolve(sel.Category)
		if err != nil {
			return spawnAgent{}, fmt.Errorf("kategori %q: %w", sel.Category, err)
		}
		return spawnAgent{Agent: a, Category: c, HasAgent: true}, nil
	}
	a, c, err := sp.cfg.Agents.Resolve(sel.Agent)
	if err != nil {
		return spawnAgent{}, fmt.Errorf("agent %q: %w", sel.Agent, err)
	}
	if !strings.EqualFold(sel.Category, c.ID) && !strings.EqualFold(sel.Category, c.Name) {
		return spawnAgent{}, fmt.Errorf("%w: agent %q berkategori %q (%s), tidak cocok dengan category %q", ErrInvalid, a.Name, c.Name, c.ID, sel.Category)
	}
	return spawnAgent{Agent: a, Category: c, HasAgent: true}, nil
}

func spawnListProjects(projects []ProjectInfo) string {
	parts := make([]string, 0, len(projects))
	for _, p := range projects {
		parts = append(parts, fmt.Sprintf("%s (%s)", p.Name, p.ID))
	}
	return strings.Join(parts, ", ")
}

func spawnResolveProject(projects []ProjectInfo, query string) (ProjectInfo, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return ProjectInfo{}, fmt.Errorf("%w: project kosong; project tersedia: %s", ErrInvalid, spawnListProjects(projects))
	}
	var matches []ProjectInfo
	qs := Slugify(q)
	for _, p := range projects {
		if p.ID == q || strings.EqualFold(strings.TrimSpace(p.Name), q) || Slugify(p.Name) == qs {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return ProjectInfo{}, fmt.Errorf("project %q tidak ditemukan di workspace thread; project tersedia: %s", q, spawnListProjects(projects))
	}
	return ProjectInfo{}, fmt.Errorf("project %q ambigu; kandidat: %s", q, spawnListProjects(matches))
}

// Spawn membuat (atau memakai ulang) peserta peer untuk project.
func (sp *Spawner) Spawn(ctx context.Context, threadID, projectQuery, brief string, sel AgentSelector) (SpawnResult, error) {
	brief = strings.TrimSpace(brief)
	if brief == "" {
		return SpawnResult{}, fmt.Errorf("%w: brief kosong", ErrInvalid)
	}
	l := sp.spawnLock(threadID)
	l.Lock()
	defer l.Unlock()

	mgr := sp.cfg.Manager
	t, err := mgr.Get(threadID)
	if err != nil {
		return SpawnResult{}, err
	}
	if t.State != StateActive {
		return SpawnResult{}, ErrClosed
	}
	projects, ok := sp.cfg.Workspaces.Projects(t.WorkspaceID)
	if !ok {
		return SpawnResult{}, errors.New("workspace thread tidak dikenal")
	}
	project, err := spawnResolveProject(projects, projectQuery)
	if err != nil {
		return SpawnResult{}, err
	}
	master, hasMaster := managerMaster(t)
	ag, err := sp.spawnResolveAgent(sel)
	if err != nil {
		return SpawnResult{}, err
	}
	agentID := ""
	if ag.HasAgent {
		agentID = ag.Agent.ID
	}

	// Reuse peserta yang sudah ada.
	for _, p := range t.Participants {
		if p.Role != RolePeer || p.ProjectID != project.ID || p.AgentID != agentID {
			continue
		}
		s, gerr := sp.cfg.Sessions.Get(p.SessionID)
		if gerr != nil || s == nil || s.Status != session.StatusRunning {
			return SpawnResult{}, fmt.Errorf("agen project %s sudah berhenti", project.Name)
		}
		if !hasMaster {
			return SpawnResult{}, errors.New("thread tidak punya master")
		}
		if _, rerr := sp.cfg.Router.Route(ctx, threadID, master.SessionID, p.SessionID, brief); rerr != nil {
			return SpawnResult{}, rerr
		}
		return SpawnResult{Reused: true, Participant: p, ProjectName: project.Name}, nil
	}
	if !hasMaster {
		return SpawnResult{}, errors.New("thread tidak punya master")
	}

	var (
		prov  provider.Provider
		model string
	)
	if ag.HasAgent {
		prov, err = agentProvider(ag.Agent)
		model = ag.Agent.Model
	} else {
		prov, err = spawnResolveProvider()
	}
	if err != nil {
		return SpawnResult{}, err
	}

	threadDir := mgr.Store().ThreadDir(threadID)
	projSlug := Slugify(project.Name)
	slug := projSlug
	recs, _ := LoadWorktreeRecords(threadDir)
	if worktreeRecordsSlugTaken(recs, slug) {
		if ag.HasAgent {
			slug = Slugify(project.Name + "-" + ag.Agent.Name)
		}
		if worktreeRecordsSlugTaken(recs, slug) {
			base := slug
			found := false
			for i := 2; i <= 20; i++ {
				cand := Slugify(base + "-" + strconv.Itoa(i))
				if !worktreeRecordsSlugTaken(recs, cand) {
					slug = cand
					found = true
					break
				}
			}
			if !found {
				return SpawnResult{}, fmt.Errorf("gagal menentukan slug worktree unik untuk %s", project.Name)
			}
		}
	}
	wt, err := CreateWorktree(project.Path, sp.cfg.DataDir, threadID, slug)
	if err != nil {
		mgr.PostSystem(threadID, "gagal men-spawn "+project.Name+": "+err.Error())
		return SpawnResult{}, fmt.Errorf("gagal membuat worktree untuk %s: %w", project.Name, err)
	}

	var (
		createdSessionID string
		addedParticipant bool
	)
	rollback := func(reason error) error {
		if addedParticipant {
			sid := createdSessionID
			if _, uerr := mgr.UpdateThread(threadID, func(th *Thread) {
				kept := th.Participants[:0:0]
				for _, p := range th.Participants {
					if p.SessionID != sid {
						kept = append(kept, p)
					}
				}
				th.Participants = kept
			}); uerr != nil {
				sp.log.Warn("thread: rollback peserta gagal", "thread", threadID, "err", uerr)
			}
		}
		if createdSessionID != "" {
			if kerr := sp.cfg.Sessions.Kill(createdSessionID); kerr != nil {
				sp.log.Warn("thread: rollback kill sesi gagal", "thread", threadID, "session", createdSessionID, "err", kerr)
			}
		}
		if rerr := RemoveWorktree(wt); rerr != nil {
			sp.log.Warn("thread: rollback worktree gagal", "thread", threadID, "err", rerr)
		}
		if derr := DeleteWorktreeRecordSlug(threadDir, project.ID, slug); derr != nil {
			sp.log.Warn("thread: rollback record worktree gagal", "thread", threadID, "err", derr)
		}
		mgr.PostSystem(threadID, "gagal men-spawn "+project.Name+": "+reason.Error())
		return fmt.Errorf("gagal men-spawn %s: %w", project.Name, reason)
	}

	rec := WorktreeRecord{ProjectID: project.ID, ProjectName: project.Name, ProjectSlug: slug, Info: wt}
	if err := UpsertWorktreeRecord(threadDir, rec); err != nil {
		return SpawnResult{}, rollback(err)
	}
	s, err := sp.cfg.Sessions.Create(session.SessionSpec{
		WorkspaceID: t.WorkspaceID,
		ProjectID:   project.ID,
		Cwd:         wt.Path,
		ProviderID:  prov.ID(),
		Model:       model,
	})
	if err != nil {
		return SpawnResult{}, rollback(err)
	}
	createdSessionID = s.ID

	name := ""
	if mgr.cfg.Namer != nil {
		name = mgr.cfg.Namer.NameOf(s.ID)
	}
	if name == "" {
		if ag.HasAgent {
			name = ag.Agent.Name + "-" + projSlug
		} else {
			name = prov.ID() + "-" + projSlug
		}
	}
	part := Participant{SessionID: s.ID, ProjectID: project.ID, Role: RolePeer, Name: name, ProviderID: prov.ID(), Model: model}
	role := PeerRole{}
	sysMsg := "master men-spawn " + project.Name
	if ag.HasAgent {
		part.AgentID = ag.Agent.ID
		part.AgentName = ag.Agent.Name
		part.Category = ag.Category.ID
		role = PeerRole{CategoryName: ag.Category.Name, Text: ag.Category.Role}
		sysMsg += " (agent " + ag.Agent.Name + ", " + ag.Category.Name + ")"
	}
	if _, err := mgr.AddParticipant(threadID, part); err != nil {
		return SpawnResult{}, rollback(err)
	}
	addedParticipant = true

	rec.SessionID = s.ID
	if err := UpsertWorktreeRecord(threadDir, rec); err != nil {
		return SpawnResult{}, rollback(err)
	}
	mgr.PostSystem(threadID, sysMsg)

	if _, err := mgr.Post(threadID, ThreadMessage{From: master.SessionID, To: s.ID, Text: brief, Kind: KindMessage}); err != nil {
		return SpawnResult{}, rollback(err)
	}
	if err := sp.spawnSendWithRetry(ctx, threadID, master.SessionID, s.ID, BuildPeerBrief(t, project.Name, brief, wt, role)); err != nil {
		return SpawnResult{}, rollback(err)
	}
	return SpawnResult{Participant: part, ProjectName: project.Name, Worktree: wt}, nil
}

func (sp *Spawner) spawnSendWithRetry(ctx context.Context, threadID, from, to, text string) error {
	var err error
	for i := 0; i < 5; i++ {
		_, err = sp.cfg.Manager.SendToSession(threadID, from, to, text)
		if err == nil || !errors.Is(err, mention.ErrNotFound) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return err
}

// BuildPeerBrief menyusun teks brief awal peer: konteks, aturan thread, tugas master.
func BuildPeerBrief(t Thread, projectName, brief string, wt WorktreeInfo, role PeerRole) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Kamu peserta thread \"%s\" untuk project %s.\n", t.Title, projectName)
	if g := strings.TrimSpace(t.Goal); g != "" {
		fmt.Fprintf(&b, "Tujuan thread: %s\n", g)
	}
	b.WriteString("Aturan thread:\n")
	fmt.Fprintf(&b, "- Bekerja HANYA di worktree sendiri (%s). Jangan menyentuh working tree utama project (%s) atau path lain di luarnya.\n", wt.Path, wt.ProjectPath)
	b.WriteString("- Jangan meminta izin (mode auto-approve).\n")
	fmt.Fprintf(&b, "- Commit pekerjaan di branch %s.\n", wt.Branch)
	b.WriteString("- Komunikasi hanya lewat tool thread_send, thread_read, thread_participants (bukan mengetik ke terminal agent lain). Balas pesan yang berawalan [thread] dengan thread_send, bukan mengetik di terminal.\n")
	b.WriteString("- Laporkan hasil ke master (to: \"master\").\n")
	b.WriteString("- Jangan mem-push atau merge kecuali diminta.\n")
	if strings.TrimSpace(role.Text) != "" {
		fmt.Fprintf(&b, "Peranmu (%s):\n%s\n", role.CategoryName, role.Text)
	}
	b.WriteString("Tugas dari master:\n")
	b.WriteString(brief)
	return b.String()
}

// RegisterTool mendaftarkan tool MCP thread_spawn (sekali per instance).
func (sp *Spawner) RegisterTool() {
	sp.once.Do(func() {
		RegisterTool(ToolDef{
			Name:        "thread_spawn",
			Description: "Men-spawn agent peer untuk sebuah project di workspace thread ini. Agent bekerja di git worktree dan branch miliknya sendiri. Bila project sudah punya agent yang sama, brief dikirim sebagai pesan baru. Parameter agent dan category opsional; keduanya kosong memakai agent default Executor bila ada.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"project":  map[string]any{"type": "string", "description": "Nama, slug, atau id project di workspace thread."},
					"brief":    map[string]any{"type": "string", "description": "Tugas untuk agent project tersebut."},
					"agent":    map[string]any{"type": "string", "description": "Opsional. Nama agent global yang dipakai (lihat daftar agent di prompt). Bila diisi bersama category, kategori agent harus cocok."},
					"category": map[string]any{"type": "string", "description": "Opsional. Id atau nama kategori (mis. planner, researcher, executor); memakai agent default kategori itu."},
				},
				"required": []string{"project", "brief"},
			},
			MasterOnly: true,
			Handler: func(ctx context.Context, c Caller, args json.RawMessage) (any, error) {
				if !c.Master {
					return nil, errors.New("tool hanya untuk master")
				}
				var in struct {
					Project  string `json:"project"`
					Brief    string `json:"brief"`
					Agent    string `json:"agent"`
					Category string `json:"category"`
				}
				if err := json.Unmarshal(args, &in); err != nil {
					return nil, fmt.Errorf("argumen tidak valid: %w", err)
				}
				res, err := sp.Spawn(ctx, c.ThreadID, in.Project, in.Brief, AgentSelector{Agent: in.Agent, Category: in.Category})
				if err != nil {
					return nil, err
				}
				if res.Reused {
					return fmt.Sprintf("agen %s untuk project %s sudah ada; brief dikirim sebagai pesan baru (dipakai ulang)", res.Participant.Name, res.ProjectName), nil
				}
				if res.Participant.AgentName != "" {
					return fmt.Sprintf("peserta %s (project %s, agent %s, kategori %s) dibuat; worktree: %s; branch: %s", res.Participant.Name, res.ProjectName, res.Participant.AgentName, res.Participant.Category, res.Worktree.Path, res.Worktree.Branch), nil
				}
				return fmt.Sprintf("peserta %s (project %s) dibuat; worktree: %s; branch: %s", res.Participant.Name, res.ProjectName, res.Worktree.Path, res.Worktree.Branch), nil
			},
		})
	})
}
