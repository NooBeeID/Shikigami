package provider

import (
	"os"
	"sort"
	"sync"

	"aiterm/internal/session"
)

// LaunchContext adalah konteks peluncuran yang dapat diperkaya oleh contribution.
type LaunchContext struct {
	// Session adalah spesifikasi sesi yang diluncurkan.
	Session session.SessionSpec
	// Provider adalah provider terpilih.
	Provider Provider
	// Env adalah env tambahan (tidak pernah nil saat diberikan).
	Env map[string]string
	// Args adalah argumen tambahan untuk binary provider.
	Args []string
	// MCPServers memetakan nama ke konfigurasi server MCP (tidak pernah nil saat diberikan).
	MCPServers map[string]any
}

// LaunchContribution memperkaya LaunchContext sebelum Provider.Command dipanggil.
type LaunchContribution func(*LaunchContext)

var (
	contribMu     sync.RWMutex
	contributions []LaunchContribution
)

// RegisterLaunchContribution mendaftarkan contribution; nil diabaikan. Aman konkuren.
func RegisterLaunchContribution(fn LaunchContribution) {
	if fn == nil {
		return
	}
	contribMu.Lock()
	contributions = append(contributions, fn)
	contribMu.Unlock()
}

// applyContributions menjalankan semua contribution berurutan di luar lock, lalu
// menormalisasi Env dan MCPServers agar tidak pernah nil.
func applyContributions(ctx *LaunchContext) {
	contribMu.RLock()
	fns := append([]LaunchContribution(nil), contributions...)
	contribMu.RUnlock()
	for _, fn := range fns {
		fn(ctx)
	}
	if ctx.Env == nil {
		ctx.Env = map[string]string{}
	}
	if ctx.MCPServers == nil {
		ctx.MCPServers = map[string]any{}
	}
}

// Launcher adalah session.Launcher yang mendelegasikan ke provider.
type Launcher struct{}

var _ session.Launcher = Launcher{}

// NewLauncher mengembalikan Launcher berbasis provider.
func NewLauncher() session.Launcher { return Launcher{} }

// Command menyusun perintah, argumen, dan env untuk sebuah sesi.
func (Launcher) Command(s session.SessionSpec) (string, []string, []string) {
	p, ok := Get(s.ProviderID)
	if !ok {
		p = Default()
	}
	ctx := &LaunchContext{
		Session:    s,
		Provider:   p,
		Env:        map[string]string{},
		MCPServers: map[string]any{},
	}
	applyContributions(ctx)
	// Pengecekan binary tidak dilakukan di sini (Command tidak punya error); itu tugas Validate di provider-ipc.
	cmd, args, extra := p.Command(ctx)

	env := make([]string, 0, 64+len(s.Env)+len(ctx.Env)+len(extra))
	env = append(env, os.Environ()...)
	env = append(env, "TERM=xterm-256color", "COLORTERM=truecolor")
	env = append(env, s.Env...)
	env = append(env, envList(ctx.Env)...)
	env = append(env, extra...)
	return cmd, args, env
}

// envList mengubah map menjadi KEY=VALUE terurut menurut kunci.
func envList(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}
