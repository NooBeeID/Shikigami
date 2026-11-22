package mention

import (
	"context"
	"sort"
	"sync"

	"github.com/NooBeeID/shikigami/internal/agents"
	"github.com/NooBeeID/shikigami/internal/session"
)

// AgentResolver dipenuhi *agents.Store. Satu-satunya tempat aturan resolusi agent.
type AgentResolver interface {
	Resolve(ref string) (agents.Agent, agents.Category, error)
}

// SessionSpawner dipenuhi *session.Manager.
type SessionSpawner interface {
	Create(spec session.SessionSpec) (*session.Session, error)
}

// ProjectDirectory memberi direktori kerja project; dipenuhi workspaceDirectory.
type ProjectDirectory interface {
	ProjectDir(workspaceID, projectID string) string
}

var _ AgentResolver = (*agents.Store)(nil)
var _ SessionSpawner = (*session.Manager)(nil)

// SpawnLabels adalah registri in-memory sessionID -> nama agent, aman konkuren.
type SpawnLabels struct {
	mu   sync.Mutex
	byID map[string]string
}

// NewSpawnLabels membuat registri kosong.
func NewSpawnLabels() *SpawnLabels {
	return &SpawnLabels{byID: map[string]string{}}
}

// Set mencatat label agent untuk sesi.
func (l *SpawnLabels) Set(sessionID, agentName string) {
	l.mu.Lock()
	l.byID[sessionID] = agentName
	l.mu.Unlock()
}

// Get mengembalikan nama agent sebuah sesi.
func (l *SpawnLabels) Get(sessionID string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	name, ok := l.byID[sessionID]
	return name, ok
}

// Delete menghapus label; aman untuk id tak dikenal (cocok sebagai callback exit).
func (l *SpawnLabels) Delete(sessionID string) {
	l.mu.Lock()
	delete(l.byID, sessionID)
	l.mu.Unlock()
}

// SessionsOf mengembalikan id sesi berlabel agentName, berurut stabil.
func (l *SpawnLabels) SessionsOf(agentName string) []string {
	l.mu.Lock()
	var out []string
	for id, n := range l.byID {
		if n == agentName {
			out = append(out, id)
		}
	}
	l.mu.Unlock()
	sort.Strings(out)
	return out
}

// ProjectContext adalah konteks project pengirim untuk spawn sesi.
type ProjectContext struct{ WorkspaceID, ProjectID, Cwd, FromSessionID string }

// TargetRouter dipenuhi *Queue; dipakai tools.go lewat Runtime.Router.
type TargetRouter interface {
	ResolveOrSpawn(ctx context.Context, pc ProjectContext, ref, text, source string) (Message, bool, error)
}

// userFacingError membungkus galat yang teksnya harus sampai apa adanya ke
// pengguna dan dipetakan ke ipc.ErrInvalid oleh toIPCError.
type userFacingError struct{ err error }

func (e *userFacingError) Error() string { return e.err.Error() }
func (e *userFacingError) Unwrap() error { return e.err }
func (e *userFacingError) Is(target error) bool {
	return target == ErrInvalid
}

// buildBrief membentuk pesan pertama: peran kategori + pesan asli.
func buildBrief(cat agents.Category, text string) string {
	return "[Peran: " + cat.Name + "]\n" + cat.Role + "\n\n--- Pesan ---\n" + text
}
