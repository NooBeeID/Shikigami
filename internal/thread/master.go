package thread

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/NooBeeID/shikigami/internal/agents"
	"github.com/NooBeeID/shikigami/internal/mention"
	"github.com/NooBeeID/shikigami/internal/provider"
	"github.com/NooBeeID/shikigami/internal/session"
)

// resolveMasterProvider memilih provider agent: claude lalu codex yang tersedia.
// Bukan provider.Default() (itu shell).
func resolveMasterProvider() (provider.Provider, error) {
	for _, id := range []string{provider.IDClaude, provider.IDCodex} {
		p, ok := provider.Get(id)
		if !ok {
			continue
		}
		if _, err := p.Available(); err == nil {
			return p, nil
		}
	}
	return nil, errors.New("tidak ada provider agent (claude/codex) yang tersedia")
}

// masterChoice hasil pemilihan provider master.
type masterChoice struct {
	Prov           provider.Provider
	Model          string
	Agent          agents.Agent // nol bila Claude polos
	NoOrchestrator bool         // true: registry ada tetapi tak ada agent Orchestrator
}

// masterResolve memilih provider+model master. Tanpa registry: perilaku lama.
// Orchestrator valid: provider+model agent (tanpa fallback bila gagal validasi).
// Orchestrator tidak ada: Claude polos dengan NoOrchestrator=true.
func (m *Manager) masterResolve() (masterChoice, error) {
	if m.cfg.Agents == nil {
		p, err := resolveMasterProvider()
		if err != nil {
			return masterChoice{}, err
		}
		return masterChoice{Prov: p}, nil
	}
	ag, _, err := m.cfg.Agents.Resolve("orchestrator")
	if err == nil {
		p, perr := agentProvider(ag)
		if perr != nil {
			return masterChoice{}, fmt.Errorf("master: %w", perr)
		}
		return masterChoice{Prov: p, Model: ag.Model, Agent: ag}, nil
	}
	if agentAbsent(m.cfg.Agents, "orchestrator", err) {
		p, perr := resolveMasterProvider()
		if perr != nil {
			return masterChoice{}, perr
		}
		return masterChoice{Prov: p, NoOrchestrator: true}, nil
	}
	return masterChoice{}, fmt.Errorf("agent orchestrator: %w", err)
}

// SpawnMaster membuat sesi master (cwd netral, auto-approve lewat launcher provider),
// menyimpannya sebagai peserta, lalu mengirim prompt awal lewat antrean saat idle.
func (m *Manager) SpawnMaster(t Thread) (Participant, error) {
	ch, err := m.masterResolve()
	if err != nil {
		return Participant{}, err
	}
	dir := m.cfg.Store.MasterDir(t.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Participant{}, fmt.Errorf("siapkan direktori master: %w", err)
	}
	s, err := m.cfg.Sessions.Create(session.SessionSpec{
		WorkspaceID: t.WorkspaceID,
		ProjectID:   "",
		Cwd:         dir,
		ProviderID:  ch.Prov.ID(),
		Model:       ch.Model,
	})
	if err != nil {
		return Participant{}, fmt.Errorf("buat sesi master: %w", err)
	}
	if ch.NoOrchestrator {
		if _, err := m.UpdateThread(t.ID, func(x *Thread) { x.NoOrchestrator = true }); err != nil {
			_ = m.cfg.Sessions.Kill(s.ID) // sesi milik sendiri
			return Participant{}, err
		}
		t.NoOrchestrator = true
	}
	part := Participant{SessionID: s.ID, ProjectID: "", Role: RoleMaster, Name: "master", ProviderID: ch.Prov.ID()}
	if ch.Agent.ID != "" {
		part.AgentID = ch.Agent.ID
		part.AgentName = ch.Agent.Name
		part.Category = "orchestrator"
		part.Model = ch.Model
	}
	if _, err := m.AddParticipant(t.ID, part); err != nil {
		_ = m.cfg.Sessions.Kill(s.ID) // sesi milik sendiri
		return Participant{}, err
	}
	var projects []ProjectInfo
	if m.cfg.Workspaces != nil {
		projects, _ = m.cfg.Workspaces.Projects(t.WorkspaceID)
	}
	prompt := BuildMasterPrompt(t, projects, m.agentsSection())
	go m.masterSendInitialPrompt(t.ID, s.ID, prompt)
	return part, nil
}

func (m *Manager) masterSendInitialPrompt(threadID, sessionID, prompt string) {
	defer func() {
		if r := recover(); r != nil {
			m.log.Error("thread: panic saat kirim prompt awal", "thread", threadID, "panic", r)
		}
	}()
	var err error
	for i := 0; i < 5; i++ {
		_, err = m.SendToSession(threadID, "", sessionID, prompt)
		if err == nil || !errors.Is(err, mention.ErrNotFound) {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err != nil {
		m.PostSystem(threadID, "gagal mengirim prompt awal ke master: "+err.Error())
		return
	}
	m.PostSystem(threadID, "prompt awal dikirim ke master")
}
