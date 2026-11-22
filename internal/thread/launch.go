package thread

import (
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/NooBeeID/shikigami/internal/provider"
)

var launchOnce sync.Once

func launchIsULID(s string) bool {
	if len(s) != 26 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

func launchClean(p string) string {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// ThreadIDFromCwd menurunkan id thread dari cwd di bawah <dataDir>/threads/<id>/...
func ThreadIDFromCwd(dataDir, cwd string) (string, bool) {
	if dataDir == "" || cwd == "" {
		return "", false
	}
	base := launchClean(filepath.Join(dataDir, "threads"))
	c := launchClean(cwd)
	rel, err := filepath.Rel(base, c)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 || !launchIsULID(parts[0]) {
		return "", false
	}
	return parts[0], true
}

// InstallLaunchContribution memasang server MCP thread ke sesi peserta thread (sekali).
func InstallLaunchContribution(s *MCPServer, m *Manager, dataDir string, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	if s == nil || m == nil {
		logger.Warn("launch contribution thread tidak dipasang: server atau manager nil")
		return
	}
	launchOnce.Do(func() {
		provider.RegisterLaunchContribution(func(ctx *provider.LaunchContext) {
			if ctx == nil || ctx.Provider == nil || !ctx.Provider.Capabilities().MCP {
				return
			}
			id, ok := ThreadIDFromCwd(dataDir, ctx.Session.Cwd)
			if !ok {
				return
			}
			t, err := m.Get(id)
			if err != nil || t.State != StateActive {
				return
			}
			u := s.URL()
			if u == "" {
				return
			}
			tok := s.mcpIssueToken(ctx.Session, ctx.Provider.ID(), id)
			if tok == "" {
				logger.Warn("gagal membuat token thread; MCP tidak dipasang")
				return
			}
			if ctx.MCPServers == nil {
				ctx.MCPServers = make(map[string]any)
			}
			ctx.MCPServers[mcpServerName] = map[string]any{
				"type":    "http",
				"url":     u,
				"headers": map[string]any{"Authorization": "Bearer " + tok},
			}
		})
	})
}
