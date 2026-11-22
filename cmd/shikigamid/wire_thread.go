package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/agents"
	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/mention"
	"github.com/NooBeeID/shikigami/internal/paths"
	"github.com/NooBeeID/shikigami/internal/session"
	"github.com/NooBeeID/shikigami/internal/status"
	"github.com/NooBeeID/shikigami/internal/thread"
)

func init() {
	RegisterModule("thread", func(srv *ipc.Server, d *Deps) {
		d.OnReady(func() { wireThread(srv, d) })
	})
}

// threadWorkspaceDirectory memenuhi thread.WorkspaceSource dan mention.Directory
// lewat IPC internal workspace.list.
type threadWorkspaceDirectory struct {
	socket string

	mu        sync.Mutex
	client    *ipc.Client
	cache     map[string][]thread.ProjectInfo
	fetchedAt time.Time
}

func (w *threadWorkspaceDirectory) refreshLocked() bool {
	if w.client == nil {
		c, err := ipc.Dial(w.socket)
		if err != nil {
			return false
		}
		w.client = c
	}
	var raw json.RawMessage
	if err := w.client.Call("workspace.list", struct{}{}, &raw); err != nil {
		_ = w.client.Close()
		w.client = nil
		return false
	}
	type proj struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	type ws struct {
		ID       string `json:"id"`
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
	cache := map[string][]thread.ProjectInfo{}
	for _, x := range list {
		ps := make([]thread.ProjectInfo, 0, len(x.Projects))
		for _, p := range x.Projects {
			ps = append(ps, thread.ProjectInfo{ID: p.ID, Name: p.Name, Path: p.Path})
		}
		cache[x.ID] = ps
	}
	w.cache = cache
	w.fetchedAt = time.Now()
	return true
}

func (w *threadWorkspaceDirectory) Projects(workspaceID string) ([]thread.ProjectInfo, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cache == nil || time.Since(w.fetchedAt) > 5*time.Second {
		if !w.refreshLocked() {
			return nil, false
		}
	}
	ps, ok := w.cache[workspaceID]
	if !ok {
		// workspace baru bisa belum ada di cache: paksa satu refresh.
		if !w.refreshLocked() {
			return nil, false
		}
		ps, ok = w.cache[workspaceID]
		if !ok {
			return nil, false
		}
	}
	return append([]thread.ProjectInfo{}, ps...), true
}

func (w *threadWorkspaceDirectory) ProjectName(workspaceID, projectID string) string {
	ps, ok := w.Projects(workspaceID)
	if !ok {
		return ""
	}
	for _, p := range ps {
		if p.ID == projectID {
			return p.Name
		}
	}
	return ""
}

func threadWireSubscribeExit(d *Deps, onExit func(string)) {
	var try func(attempt int)
	try = func(attempt int) {
		c, err := ipc.Dial(d.SocketPath)
		if err == nil {
			_, err = c.Subscribe("session.exit", func(topic string, payload json.RawMessage) {
				var p struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(payload, &p) == nil && p.ID != "" {
					onExit(p.ID)
				}
			})
			if err != nil {
				_ = c.Close()
			}
		}
		if err != nil {
			if attempt < 5 {
				time.AfterFunc(200*time.Millisecond, func() { try(attempt + 1) })
			} else {
				d.Logger.Warn("thread: gagal berlangganan session.exit", "err", err)
			}
		}
	}
	try(1)
}

func wireThread(srv *ipc.Server, d *Deps) {
	defer func() {
		if r := recover(); r != nil {
			d.Logger.Error("thread: panic saat wiring", "panic", r)
		}
	}()
	v, ok := d.Lookup("session.manager")
	mgr, ok2 := v.(*session.Manager)
	if !ok || !ok2 || mgr == nil {
		d.Logger.Warn("thread: session.manager tidak tersedia, modul dinonaktifkan")
		return
	}
	dataDir := d.DataDir
	if dataDir == "" {
		dd, err := paths.DataDir()
		if err != nil {
			d.Logger.Warn("thread: data dir tidak tersedia", "err", err)
			return
		}
		dataDir = dd
	}
	store := thread.NewStore(filepath.Join(dataDir, "threads"))
	dir := &threadWorkspaceDirectory{socket: d.SocketPath}
	statusOf := func(id string) string {
		if e := status.Default(); e != nil {
			if st, ok := e.Get(id); ok && st.Status != "" {
				return string(st.Status)
			}
		}
		if status.IsIdle(id) {
			return "idle"
		}
		return "running"
	}
	namer := mention.NewNamer(mgr, dir, statusOf)
	// Antrean khusus thread: mode auto, tanpa publish agar tidak muncul di badge mention.
	q := mention.NewQueue(mention.QueueConfig{Sessions: mgr, Namer: namer, Publish: func(string, any) {}, Logger: d.Logger})
	if err := q.SetMode(mention.ModeAuto); err != nil {
		d.Logger.Warn("thread: gagal set mode antrean", "err", err)
	}
	var exitMu sync.Mutex
	exitFns := []func(string){q.HandleExit}
	threadWireSubscribeExit(d, func(id string) {
		exitMu.Lock()
		fns := append([]func(string){}, exitFns...)
		exitMu.Unlock()
		for _, fn := range fns {
			fn(id)
		}
	})

	// Registry agent (tiket 01): opsional. Jangan memberi interface berisi nil-pointer.
	var resolver thread.AgentResolver
	var agentStore *agents.Store
	if av, aok := d.Lookup("agents.store"); aok {
		if as, aok2 := av.(*agents.Store); aok2 && as != nil {
			agentStore = as
			resolver = as
		}
	}
	if resolver == nil {
		d.Logger.Warn("thread: agents.store tidak tersedia, thread memakai Claude polos")
	}

	m := thread.NewManager(thread.ManagerConfig{
		Agents:     resolver,
		Store:      store,
		Sessions:   mgr,
		Sender:     q,
		Workspaces: dir,
		Namer:      namer,
		Publish:    srv.Publish,
		Logger:     d.Logger,
	})
	d.Provide("thread.manager", m)
	thread.RegisterIPC(srv, m)
	if _, err := thread.InstallAgents(context.Background(), thread.AgentsConfig{
		Manager:    m,
		Agents:     resolver,
		Sessions:   mgr,
		Workspaces: dir,
		DataDir:    dataDir,
		Logger:     d.Logger,
		OnSessionExit: func(fn func(string)) {
			exitMu.Lock()
			exitFns = append(exitFns, fn)
			exitMu.Unlock()
		},
	}); err != nil {
		d.Logger.Warn("thread: gagal memasang tool agent", "err", err)
	}
	if v, ok := d.Lookup("status.engine"); ok {
		if eng, ok2 := v.(*status.Engine); ok2 && eng != nil {
			thread.InstallActivity(m, eng)
		} else {
			d.Logger.Warn("thread: status.engine bertipe tak terduga, kartu aktivitas dinonaktifkan")
		}
	} else {
		d.Logger.Warn("thread: status.engine tidak tersedia, kartu aktivitas dinonaktifkan")
	}
	thread.RegisterClose(m)
	rec := thread.NewRecovery(m)
	m.SetPreSend(rec.PreSend)
	m.OnStart(func(*thread.Manager) {
		if err := rec.MarkDeadOnStart(); err != nil {
			d.Logger.Warn("thread: gagal menandai agent mati saat start", "err", err)
		}
	})
	m.OnStart(func(mm *thread.Manager) {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					d.Logger.Error("thread: panic pembersihan yatim", "panic", r)
				}
			}()
			if n, err := thread.CleanOrphans(mm); err != nil {
				d.Logger.Warn("thread: pembersihan yatim selesai dengan galat", "cleaned", n, "err", err)
			} else if n > 0 {
				d.Logger.Info("thread: pembersihan yatim selesai", "cleaned", n)
			}
		}()
	})
	if agentStore != nil {
		// Guard dipanggil dengan kunci store terpegang: jangan memanggil agentStore.List()
		// di dalamnya. Nama agent diambil dari cache yang dimuat sebelum guard berjalan.
		var namesMu sync.RWMutex
		names := map[string]string{}
		reload := func() {
			_, as := agentStore.List()
			nm := make(map[string]string, len(as))
			for _, a := range as {
				nm[a.ID] = a.Name
			}
			namesMu.Lock()
			names = nm
			namesMu.Unlock()
		}
		reload()
		go func() {
			defer func() { _ = recover() }()
			c, err := ipc.Dial(d.SocketPath)
			if err != nil {
				return
			}
			if _, err := c.Subscribe("agents.changed", func(string, json.RawMessage) { reload() }); err != nil {
				_ = c.Close()
			}
		}()
		agentStore.SetDeleteGuard(func(id string) error {
			title, used := m.InUse(id)
			if !used {
				return nil
			}
			namesMu.RLock()
			name := names[id]
			namesMu.RUnlock()
			if name == "" {
				name = id
			}
			return fmt.Errorf("agent %q dipakai thread aktif %q", name, title)
		})
	}
	m.FireStart()
}
