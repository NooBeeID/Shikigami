// Berkas ini adalah kontrak epic thread-ticket-02-thread-agents dan hanya boleh ditambah.
package thread

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NooBeeID/shikigami/internal/agents"
	"github.com/NooBeeID/shikigami/internal/provider"
)

// Router dipenuhi oleh router.go (thread-spawn-router); dipakai handler tool (thread-mcp-server).
type Router interface {
	// Route mencatat pesan ke log thread lalu mengirimnya. from = sessionId pengirim ("user" untuk user).
	// to = nama/sessionId peserta, "master", "user" (hanya dicatat) atau "all" (semua peserta selain pengirim).
	Route(ctx context.Context, threadID, from, to, text string) (ThreadMessage, error)
}

var (
	ErrPaused          = errors.New("thread dijeda: pesan beruntun antar agent mencapai batas; tunggu pesan dari user atau master")
	ErrTargetUnknown   = errors.New("target bukan peserta thread ini")
	ErrTargetAmbiguous = errors.New("target ambigu")
	ErrNotParticipant  = errors.New("pemanggil bukan peserta thread ini")
)

// ParticipantView adalah bentuk keluaran tool thread_participants dan dipakai ulang tiket 03/05.
type ParticipantView struct {
	Name      string `json:"name"`
	SessionID string `json:"sessionId"`
	Role      string `json:"role"`
	Project   string `json:"project"` // nama project; kosong untuk master
	Status    string `json:"status"`  // "kerja" | "idle"
	Self      bool   `json:"self"`
}

// Ditambah epic global-agents-ticket-03 (agent global, orchestrator di thread).

// AgentResolver dipenuhi *agents.Store.
type AgentResolver interface {
	Resolve(ref string) (agents.Agent, agents.Category, error)
	List() ([]agents.Category, []agents.Agent)
}

// AgentSelector: pilihan eksplisit pada thread_spawn. Keduanya kosong = default Executor.
type AgentSelector struct{ Agent, Category string }

// PeerRole: peran kategori yang disisipkan ke brief peer.
type PeerRole struct{ CategoryName, Text string }

// ErrNoAgent: tidak ada agent untuk kategori (bukan galat lain).
var ErrNoAgent = errors.New("tidak ada agent default untuk kategori")

// agentAbsent: true bila kategori tidak punya agent sama sekali; galat lain => false.
func agentAbsent(r AgentResolver, categoryID string, resolveErr error) bool {
	if resolveErr == nil || r == nil {
		return false
	}
	if errors.Is(resolveErr, agents.ErrNotFound) {
		return true
	}
	_, list := r.List()
	for _, a := range list {
		if a.CategoryID == categoryID {
			return false
		}
	}
	return true
}

// agentProvider memvalidasi provider agent: terdaftar, tersedia, dan mendukung MCP.
func agentProvider(a agents.Agent) (provider.Provider, error) {
	p, ok := provider.Get(a.ProviderID)
	if !ok {
		return nil, fmt.Errorf("agent %s: provider %q tidak terdaftar", a.Name, a.ProviderID)
	}
	if _, err := p.Available(); err != nil {
		return nil, fmt.Errorf("agent %s: provider %q tidak tersedia: %v", a.Name, a.ProviderID, err)
	}
	if !p.Capabilities().MCP {
		return nil, fmt.Errorf("agent %s: provider %q tidak mendukung MCP, tidak bisa memakai tool thread", a.Name, a.ProviderID)
	}
	return p, nil
}

// agentRoleLine memadatkan peran ke satu baris, dipotong 160 rune.
func agentRoleLine(role string) string {
	s := strings.Join(strings.Fields(role), " ")
	r := []rune(s)
	if len(r) > 160 {
		return string(r[:160]) + "…"
	}
	return s
}

// InUse melaporkan apakah agentID dipakai thread aktif (peserta Dead tetap dihitung karena bisa dihidupkan ulang).
func (m *Manager) InUse(agentID string) (threadTitle string, inUse bool) {
	if agentID == "" {
		return "", false
	}
	for _, t := range m.managerAllThreads() {
		if t.State != StateActive {
			continue
		}
		for _, p := range t.Participants {
			if p.AgentID == agentID {
				if t.Title == "" {
					return t.ID, true
				}
				return t.Title, true
			}
		}
	}
	return "", false
}
