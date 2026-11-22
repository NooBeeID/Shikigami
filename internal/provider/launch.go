package provider

import (
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/NooBeeID/shikigami/internal/session"
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
	// Model adalah nama model yang diminta (kosong = default provider). Diisi
	// Launcher dari Session.Model; Command tiap provider menerjemahkannya ke flag.
	Model string
	// MCPServers memetakan nama ke konfigurasi server MCP (tidak pernah nil saat diberikan).
	MCPServers map[string]any
}

// LaunchContribution memperkaya LaunchContext sebelum Provider.Command dipanggil.
type LaunchContribution func(*LaunchContext)

// Flag yang selalu dipakai saat menjalankan agent agar tidak ada prompt izin
// (keputusan pembuat: agent selalu berjalan tanpa pemeriksaan izin).
const (
	claudeSkipPermissionsFlag = "--dangerously-skip-permissions"
	codexBypassFlag           = "--dangerously-bypass-approvals-and-sandbox"
)

// withFlag mengembalikan args dengan flag di depan bila belum ada.
func withFlag(args []string, flag string) []string {
	for _, a := range args {
		if a == flag {
			return args
		}
	}
	return append([]string{flag}, args...)
}

// withValueFlag mengembalikan args dengan "<flag> <value>" ditambahkan di akhir
// bila value tidak kosong dan args belum memuat flag tersebut. Args yang sudah
// ada MENANG: bila args memuat flag, alias (bila tidak kosong), atau bentuk
// "flag=..." / "alias=...", args dikembalikan apa adanya (tidak digandakan,
// tidak ditimpa). value kosong (setelah strings.TrimSpace) = args apa adanya.
func withValueFlag(args []string, flag, alias, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return args
	}
	for _, a := range args {
		if a == flag || (alias != "" && a == alias) ||
			strings.HasPrefix(a, flag+"=") || (alias != "" && strings.HasPrefix(a, alias+"=")) {
			return args
		}
	}
	return append(args, flag, value)
}

var pathSource atomic.Pointer[func() string]

// SetPathSource memasang sumber PATH untuk env sesi. Nil berarti memakai os.Environ apa adanya.
func SetPathSource(fn func() string) {
	if fn == nil {
		pathSource.Store(nil)
		return
	}
	pathSource.Store(&fn)
}

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
		Model:      s.Model,
		Env:        map[string]string{},
		MCPServers: map[string]any{},
	}
	applyContributions(ctx)
	// Pengecekan binary tidak dilakukan di sini (Command tidak punya error); itu tugas Validate di provider-ipc.
	cmd, args, extra := p.Command(ctx)

	env := make([]string, 0, 64+len(s.Env)+len(ctx.Env)+len(extra))
	env = append(env, os.Environ()...)
	if src := pathSource.Load(); src != nil {
		if p := (*src)(); p != "" {
			env = append(env, "PATH="+p)
		}
	}
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
