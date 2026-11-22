package main

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/grimoire"
	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/paths"
	"github.com/NooBeeID/shikigami/internal/provider"
	"github.com/NooBeeID/shikigami/internal/session"
)

func init() {
	RegisterModule("grimoire-refresh", func(srv *ipc.Server, d *Deps) {
		d.OnReady(func() { wireGrimoireRefresh(srv, d) })
	})
}

type refreshProjectEntry struct {
	ID   string
	Path string
}

// refreshProjectDirectory memetakan sesi ke project lewat workspace.list (malas, cache 5 detik).
type refreshProjectDirectory struct {
	socket    string
	mu        sync.Mutex
	client    *ipc.Client
	entries   []refreshProjectEntry
	fetchedAt time.Time
}

func (d *refreshProjectDirectory) list() ([]refreshProjectEntry, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.entries == nil || time.Since(d.fetchedAt) > 5*time.Second {
		if !d.refreshLocked() && d.entries == nil {
			return nil, false
		}
	}
	return d.entries, true
}

func (d *refreshProjectDirectory) refreshLocked() bool {
	if d.socket == "" {
		return false
	}
	if d.client == nil {
		c, err := ipc.Dial(d.socket)
		if err != nil {
			return false
		}
		d.client = c
	}
	var raw json.RawMessage
	if err := d.client.Call("workspace.list", map[string]any{}, &raw); err != nil {
		_ = d.client.Close()
		d.client = nil
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
	entries := []refreshProjectEntry{}
	for _, w := range list {
		for _, p := range w.Projects {
			entries = append(entries, refreshProjectEntry{ID: p.ID, Path: p.Path})
		}
	}
	d.entries = entries
	d.fetchedAt = time.Now()
	return true
}

func (d *refreshProjectDirectory) resolve(spec session.SessionSpec) (id, path string, ok bool) {
	if spec.ProjectID == "" && spec.Cwd == "" {
		return "", "", false
	}
	entries, ok := d.list()
	if !ok {
		return "", "", false
	}
	if spec.ProjectID != "" {
		for _, e := range entries {
			if e.ID == spec.ProjectID {
				if e.Path == "" {
					return "", "", false
				}
				return e.ID, e.Path, true
			}
		}
		return "", "", false
	}
	cwd := filepath.Clean(spec.Cwd)
	best := -1
	var found refreshProjectEntry
	for _, e := range entries {
		if e.Path == "" {
			continue
		}
		p := filepath.Clean(e.Path)
		prefix := p
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		if cwd == p || strings.HasPrefix(cwd, prefix) {
			if len(p) > best {
				best = len(p)
				found = e
			}
		}
	}
	if best < 0 {
		return "", "", false
	}
	return found.ID, found.Path, true
}

func refreshLogger(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.Default()
}

// refreshApply tidak mengubah ctx dan tidak menunda peluncuran.
func refreshApply(ctx *provider.LaunchContext, dir *refreshProjectDirectory, trig *grimoire.Trigger, logger *slog.Logger) {
	if ctx == nil || ctx.Provider == nil || trig == nil {
		return
	}
	if ctx.Provider.ID() == provider.IDShell {
		return
	}
	spec := ctx.Session
	log := refreshLogger(logger)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Warn("grimoire-refresh: panic saat memicu build", "panic", r)
			}
		}()
		id, path, ok := dir.resolve(spec)
		if !ok {
			return
		}
		trig.Kick(id, path)
	}()
}

var refreshOnce sync.Once

func installGrimoireRefresh(dir *refreshProjectDirectory, trig *grimoire.Trigger, logger *slog.Logger) {
	refreshOnce.Do(func() {
		provider.RegisterLaunchContribution(func(c *provider.LaunchContext) {
			refreshApply(c, dir, trig, logger)
		})
	})
}

func wireGrimoireRefresh(srv *ipc.Server, d *Deps) {
	dataDir := d.DataDir
	if dataDir == "" {
		dd, err := paths.DataDir()
		if err != nil {
			if d.Logger != nil {
				d.Logger.Warn("modul grimoire-refresh dilewati", "err", err)
			}
			return
		}
		dataDir = dd
	}
	store := grimoire.NewStore(dataDir)
	b := grimoire.NewBuilder(store, d.Logger)
	trig := grimoire.NewTrigger(b, srv.Publish)
	grimoire.RegisterRefresh(grimoire.RefreshDeps{Builder: b, Trigger: trig})
	installGrimoireRefresh(&refreshProjectDirectory{socket: d.SocketPath}, trig, d.Logger)
}
