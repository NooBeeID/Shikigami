package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/grimoire"
	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/provider"
	"github.com/NooBeeID/shikigami/internal/session"
)

func init() {
	RegisterModule("grimoire-inject", func(srv *ipc.Server, d *Deps) {
		d.OnReady(func() { wireGrimoireInject(srv, d) })
	})
}

// projectEntry adalah id dan path satu project.
type projectEntry struct{ ID, Path string }

// projectDirectory memetakan project lewat IPC internal (workspace.list), cache 5 detik.
type projectDirectory struct {
	socket string

	mu        sync.Mutex
	client    *ipc.Client
	entries   []projectEntry
	fetchedAt time.Time
}

func (p *projectDirectory) list() ([]projectEntry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.entries == nil || time.Since(p.fetchedAt) > 5*time.Second {
		if !p.refreshLocked() {
			if p.entries != nil {
				return p.entries, true
			}
			return nil, false
		}
	}
	return p.entries, true
}

func (p *projectDirectory) refreshLocked() bool {
	if p.client == nil {
		c, err := ipc.Dial(p.socket)
		if err != nil {
			return false
		}
		p.client = c
	}
	var raw json.RawMessage
	if err := p.client.Call("workspace.list", map[string]any{}, &raw); err != nil {
		_ = p.client.Close()
		p.client = nil
		return false
	}
	type proj struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	type ws struct {
		Projects []proj `json:"projects"`
	}
	var list []ws
	if err := json.Unmarshal(raw, &list); err != nil {
		var obj struct {
			Workspaces []ws `json:"workspaces"`
		}
		if err2 := json.Unmarshal(raw, &obj); err2 != nil {
			return false
		}
		list = obj.Workspaces
	}
	entries := []projectEntry{}
	for _, w := range list {
		for _, pr := range w.Projects {
			entries = append(entries, projectEntry{ID: pr.ID, Path: pr.Path})
		}
	}
	p.entries = entries
	p.fetchedAt = time.Now()
	return true
}

// Known menjadi InjectDeps.Known.
func (p *projectDirectory) Known(ctx context.Context, projectID string) (bool, error) {
	entries, ok := p.list()
	if !ok {
		return false, errors.New("daftar project tidak tersedia")
	}
	for _, e := range entries {
		if e.ID == projectID {
			return true, nil
		}
	}
	return false, nil
}

// match mengembalikan id project dengan path terpanjang yang sama dengan atau induk dari cwd.
func (p *projectDirectory) match(cwd string) string {
	if cwd == "" {
		return ""
	}
	entries, ok := p.list()
	if !ok {
		return ""
	}
	cwd = filepath.Clean(cwd)
	sep := string(filepath.Separator)
	best, bestLen := "", -1
	for _, e := range entries {
		if e.Path == "" {
			continue
		}
		path := filepath.Clean(e.Path)
		prefix := path
		if !strings.HasSuffix(prefix, sep) {
			prefix += sep
		}
		if cwd == path || strings.HasPrefix(cwd, prefix) {
			if len(path) > bestLen {
				best, bestLen = e.ID, len(path)
			}
		}
	}
	return best
}

func grimoireInjectProjectID(spec session.SessionSpec, dir *projectDirectory) string {
	if spec.ProjectID != "" {
		return spec.ProjectID
	}
	if dir == nil {
		return ""
	}
	return dir.match(spec.Cwd)
}

func grimoireInjectHasArg(args []string, name string) bool {
	for _, a := range args {
		if a == name || strings.HasPrefix(a, name+"=") {
			return true
		}
	}
	return false
}

func grimoireInjectHasCodexInstructions(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "developer_instructions=") {
			return true
		}
	}
	return false
}

func grimoireInjectLogger(l *slog.Logger) *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l
}

func grimoireInjectApply(ctx *provider.LaunchContext, dir *projectDirectory, logger *slog.Logger) {
	logger = grimoireInjectLogger(logger)
	defer func() {
		if r := recover(); r != nil {
			logger.Warn("grimoire: panic di launch contribution", "panic", r)
		}
	}()
	if ctx == nil || ctx.Provider == nil {
		return
	}
	id := ctx.Provider.ID()
	if id != provider.IDClaude && id != provider.IDCodex {
		return
	}
	if id == provider.IDClaude && grimoireInjectHasArg(ctx.Args, "--append-system-prompt-file") {
		return
	}
	if id == provider.IDCodex && grimoireInjectHasCodexInstructions(ctx.Args) {
		return
	}
	pid := grimoireInjectProjectID(ctx.Session, dir)
	if pid == "" {
		return
	}
	text, ok, err := grimoire.ComposeInjection(pid)
	if err != nil {
		logger.Warn("grimoire: gagal menyusun suntikan", "project", pid, "err", err)
		return
	}
	if !ok {
		return
	}
	path, err := grimoire.WriteSnapshot(text)
	if err != nil {
		logger.Warn("grimoire: gagal menulis snapshot", "project", pid, "err", err)
		if id == provider.IDClaude {
			return
		}
		path = ""
	} else if perr := grimoire.PruneSnapshots(nil); perr != nil {
		logger.Warn("grimoire: prune snapshot gagal", "err", perr)
	}
	switch id {
	case provider.IDClaude:
		ctx.Args = append(ctx.Args, "--append-system-prompt-file", path)
	case provider.IDCodex:
		ctx.Args = append(ctx.Args, "-c", "developer_instructions="+grimoire.TOMLString(text))
	}
}

var grimoireInjectOnce sync.Once

func installGrimoireInject(dir *projectDirectory, logger *slog.Logger) {
	grimoireInjectOnce.Do(func() {
		provider.RegisterLaunchContribution(func(ctx *provider.LaunchContext) {
			grimoireInjectApply(ctx, dir, logger)
		})
	})
}

func wireGrimoireInject(srv *ipc.Server, d *Deps) {
	store := grimoire.NewStore(d.DataDir)
	dir := &projectDirectory{socket: d.SocketPath}
	grimoire.RegisterInject(srv, grimoire.InjectDeps{Store: store, Known: dir.Known, Logger: d.Logger})
	if err := grimoire.PruneSnapshots(nil); err != nil {
		d.Logger.Warn("grimoire: prune awal gagal", "err", err)
	}
	installGrimoireInject(dir, d.Logger)
}
