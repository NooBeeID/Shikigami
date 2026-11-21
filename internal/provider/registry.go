package provider

import (
	"io"
	"os"
	"sort"
	"sync"
)

var (
	regMu    sync.RWMutex
	registry = map[string]Provider{}
)

// listOrder adalah urutan tetap provider bawaan pada List.
var listOrder = []string{IDClaude, IDCodex, IDShell}

// Register mendaftarkan provider; panic bila nil, id kosong, atau id sudah terdaftar.
func Register(p Provider) {
	if p == nil {
		panic("provider: provider nil")
	}
	id := p.ID()
	if id == "" {
		panic("provider: id kosong")
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, ok := registry[id]; ok {
		panic("provider: id sudah terdaftar: " + id)
	}
	registry[id] = p
}

// Get mengambil provider berdasarkan id.
func Get(id string) (Provider, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	p, ok := registry[id]
	return p, ok
}

// List mengembalikan provider terdaftar: claude, codex, shell, lalu id lain urut alfabet.
func List() []Provider {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]Provider, 0, len(registry))
	seen := map[string]bool{}
	for _, id := range listOrder {
		if p, ok := registry[id]; ok {
			out = append(out, p)
			seen[id] = true
		}
	}
	var rest []string
	for id := range registry {
		if !seen[id] {
			rest = append(rest, id)
		}
	}
	sort.Strings(rest)
	for _, id := range rest {
		out = append(out, registry[id])
	}
	return out
}

// Default mengembalikan provider shell, atau fallbackProvider bila belum terdaftar.
func Default() Provider {
	if p, ok := Get(IDShell); ok {
		return p
	}
	return fallbackProvider{}
}

// fallbackProvider adalah login shell $SHELL -l dipakai bila provider shell belum terdaftar.
type fallbackProvider struct{}

func fallbackShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/zsh"
}

func (fallbackProvider) ID() string                 { return IDShell }
func (fallbackProvider) Name() string               { return "Shell" }
func (fallbackProvider) Capabilities() Capabilities { return Capabilities{} }

func (fallbackProvider) Command(ctx *LaunchContext) (string, []string, []string) {
	args := []string{"-l"}
	if ctx != nil {
		args = append(args, ctx.Args...)
	}
	return fallbackShell(), args, nil
}

func (fallbackProvider) SendInput(w io.Writer, text string) error { return WriteInput(w, text) }

func (fallbackProvider) Available() (string, error) {
	sh := fallbackShell()
	p, err := LookPath(sh)
	if err != nil {
		return "", BinaryError(IDShell, sh)
	}
	return p, nil
}
