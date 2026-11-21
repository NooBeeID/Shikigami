package mention

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/provider"
	"github.com/NooBeeID/shikigami/internal/session"
)

// maxPending membatasi jumlah token tertunda.
const maxPending = 64

// pendingToken adalah token yang sudah dipasang ke sesi baru tetapi belum
// diikat ke sessionID.
type pendingToken struct {
	token       string
	workspaceID string
	projectID   string
	providerID  string
	cwd         string
	issuedAt    time.Time
}

// tokenManager mengelola token tertunda (FIFO) dan token terikat.
type tokenManager struct {
	mu      sync.Mutex
	pending []pendingToken    // tertua di indeks 0
	bound   map[string]string // token -> sessionID
	byID    map[string]string // sessionID -> token
}

func newTokenManager() *tokenManager {
	return &tokenManager{
		bound: make(map[string]string),
		byID:  make(map[string]string),
	}
}

// newToken membuat token acak 32 byte (hex); "" bila sumber acak gagal.
func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// issue membuat token tertunda untuk sesi yang akan diluncurkan.
func (t *tokenManager) issue(spec session.SessionSpec, providerID string) string {
	tok := newToken()
	if tok == "" {
		return ""
	}
	pid := spec.ProviderID
	if pid == "" {
		pid = providerID
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending = append(t.pending, pendingToken{
		token:       tok,
		workspaceID: spec.WorkspaceID,
		projectID:   spec.ProjectID,
		providerID:  pid,
		cwd:         spec.Cwd,
		issuedAt:    time.Now(),
	})
	if len(t.pending) > maxPending {
		t.pending = append([]pendingToken(nil), t.pending[len(t.pending)-maxPending:]...)
	}
	return tok
}

// lookup memetakan token ke sessionID; mengikat token tertunda bila perlu.
func (t *tokenManager) lookup(token string, rt *Runtime) (string, bool) {
	if token == "" || rt == nil || rt.Sessions == nil {
		return "", false
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if sid, ok := t.bound[token]; ok {
		s, err := rt.Sessions.Get(sid)
		if err != nil || s == nil || s.Status != session.StatusRunning {
			t.releaseLocked(sid)
			return "", false
		}
		return sid, true
	}

	found := false
	for _, p := range t.pending {
		if p.token == token {
			found = true
			break
		}
	}
	if !found {
		return "", false
	}
	t.bindPendingLocked(rt)
	if sid, ok := t.bound[token]; ok {
		return sid, true
	}
	// Sesi belum tampak di List; token tetap tertunda, klien mencoba lagi.
	return "", false
}

// bindPendingLocked mengikat token tertunda ke sesi tak-terikat secara FIFO.
func (t *tokenManager) bindPendingLocked(rt *Runtime) {
	var free []session.Session
	for _, s := range rt.Sessions.List() {
		if s.Status != session.StatusRunning {
			continue
		}
		if _, taken := t.byID[s.ID]; taken {
			continue
		}
		free = append(free, s)
	}
	sort.SliceStable(free, func(i, j int) bool {
		if !free[i].CreatedAt.Equal(free[j].CreatedAt) {
			return free[i].CreatedAt.Before(free[j].CreatedAt)
		}
		return free[i].ID < free[j].ID
	})
	key := func(ws, pr, pv, cwd string) string { return ws + "|" + pr + "|" + pv + "|" + cwd }
	groups := make(map[string][]session.Session)
	for _, s := range free {
		k := key(s.WorkspaceID, s.ProjectID, s.ProviderID, s.Cwd)
		groups[k] = append(groups[k], s)
	}

	remaining := t.pending[:0:0]
	for _, p := range t.pending {
		k := key(p.workspaceID, p.projectID, p.providerID, p.cwd)
		if g := groups[k]; len(g) > 0 {
			s := g[0]
			groups[k] = g[1:]
			t.bound[p.token] = s.ID
			t.byID[s.ID] = p.token
			continue
		}
		remaining = append(remaining, p)
	}
	t.pending = remaining
}

// release melepas token milik sesi.
func (t *tokenManager) release(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.releaseLocked(sessionID)
}

func (t *tokenManager) releaseLocked(sessionID string) {
	if tok, ok := t.byID[sessionID]; ok {
		delete(t.bound, tok)
		delete(t.byID, sessionID)
	}
}

func (t *tokenManager) boundCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.bound)
}

var tokens = newTokenManager()

// BoundSessionCount mengembalikan jumlah sesi ber-token terikat; dipakai
// oleh mention.info (field sessions).
func BoundSessionCount() int { return tokens.boundCount() }

var contribOnce sync.Once

// registerLaunchContribution mendaftarkan kontribusi MCP sekali saja.
//
// Keterbatasan: SessionSpec belum punya ID saat dipanggil (Manager.Create
// memanggil launcher.Command sebelum newID()), sehingga token baru diikat ke
// sesi pada request MCP pertama. Catatan untuk tiket 02/08.
func registerLaunchContribution(rt *Runtime, url string) {
	contribOnce.Do(func() {
		provider.RegisterLaunchContribution(func(ctx *provider.LaunchContext) {
			if ctx == nil || ctx.Provider == nil || !ctx.Provider.Capabilities().MCP {
				return
			}
			u := URL()
			if u == "" {
				u = url
			}
			if u == "" {
				return
			}
			tok := tokens.issue(ctx.Session, ctx.Provider.ID())
			if tok == "" {
				if rt != nil && rt.Logger != nil {
					rt.Logger.Warn("gagal membuat token mention; MCP tidak dipasang")
				}
				return
			}
			if ctx.MCPServers == nil {
				ctx.MCPServers = make(map[string]any)
			}
			ctx.MCPServers["shikigami-mention"] = map[string]any{
				"type": "http",
				"url":  u,
				"headers": map[string]any{
					"Authorization": "Bearer " + tok,
				},
			}
		})
	})
}

// hookSessionExit mendaftarkan pelepasan token saat sesi exit.
func hookSessionExit(rt *Runtime) {
	if rt != nil && rt.OnSessionExit != nil {
		rt.OnSessionExit(tokens.release)
	}
}
