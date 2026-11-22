package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/mention"
	"github.com/NooBeeID/shikigami/internal/provider"
	"github.com/NooBeeID/shikigami/internal/session"
	"github.com/NooBeeID/shikigami/internal/status"
)

func init() {
	RegisterModule("mention", func(srv *ipc.Server, d *Deps) {
		d.OnReady(func() { wireMention(srv, d) })
	})
}

type projInfo struct{ name, path string }

var _ mention.ProjectDirectory = (*workspaceDirectory)(nil)

// workspaceDirectory memenuhi mention.Directory lewat IPC internal (workspace.list).
type workspaceDirectory struct {
	socket string

	mu        sync.Mutex
	client    *ipc.Client
	cache     map[string]projInfo
	fetchedAt time.Time
}

func (w *workspaceDirectory) ProjectName(workspaceID, projectID string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cache == nil || time.Since(w.fetchedAt) > 5*time.Second {
		if !w.refreshLocked() {
			return ""
		}
	}
	return w.cache[workspaceID+"/"+projectID].name
}

// ProjectDir mengembalikan path absolut project ("" bila tak dikenal).
func (w *workspaceDirectory) ProjectDir(workspaceID, projectID string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cache == nil || time.Since(w.fetchedAt) > 5*time.Second {
		if !w.refreshLocked() {
			return ""
		}
	}
	return w.cache[workspaceID+"/"+projectID].path
}

func (w *workspaceDirectory) refreshLocked() bool {
	if w.client == nil {
		c, err := ipc.Dial(w.socket)
		if err != nil {
			return false
		}
		w.client = c
	}
	var raw json.RawMessage
	if err := w.client.Call("workspace.list", struct{}{}, &raw); err != nil {
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
	cache := map[string]projInfo{}
	for _, x := range list {
		for _, p := range x.Projects {
			cache[x.ID+"/"+p.ID] = projInfo{name: p.Name, path: p.Path}
		}
	}
	w.cache = cache
	w.fetchedAt = time.Now()
	return true
}

type exitHub struct {
	mu  sync.Mutex
	cbs []func(string)
}

func (h *exitHub) add(fn func(string)) {
	h.mu.Lock()
	h.cbs = append(h.cbs, fn)
	h.mu.Unlock()
}

func (h *exitHub) fire(id string) {
	h.mu.Lock()
	cbs := append([]func(string){}, h.cbs...)
	h.mu.Unlock()
	for _, fn := range cbs {
		fn(id)
	}
}

func wireMention(srv *ipc.Server, d *Deps) {
	v, ok := d.Lookup("session.manager")
	mgr, ok2 := v.(*session.Manager)
	if !ok || !ok2 || mgr == nil {
		d.Logger.Warn("mention: session.manager tidak tersedia, modul dinonaktifkan")
		return
	}
	dir := &workspaceDirectory{socket: d.SocketPath}
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

	var resolver mention.AgentResolver
	if v, ok := d.Lookup("agents.store"); ok {
		if r, ok2 := v.(mention.AgentResolver); ok2 && r != nil {
			resolver = r
		}
	}
	if resolver == nil {
		d.Logger.Warn("mention: agents.store tidak tersedia, mention agent dinonaktifkan")
	}
	labels := mention.NewSpawnLabels()
	namer.SetLabels(labels)

	q := mention.NewQueue(mention.QueueConfig{
		Sessions: mgr,
		Namer:    namer,
		Publish:  srv.Publish,
		Logger:   d.Logger,
		Agents:   resolver,
		Spawner:  mgr,
		Projects: dir,
		Labels:   labels,
	})

	hub := &exitHub{}
	hub.add(labels.Delete)
	hub.add(q.HandleExit)

	var subscribeExit func(attempt int)
	subscribeExit = func(attempt int) {
		c, err := ipc.Dial(d.SocketPath)
		if err == nil {
			_, err = c.Subscribe("session.exit", func(topic string, payload json.RawMessage) {
				var p struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(payload, &p) == nil && p.ID != "" {
					hub.fire(p.ID)
				}
			})
			if err != nil {
				_ = c.Close()
			}
		}
		if err != nil {
			if attempt < 5 {
				time.AfterFunc(200*time.Millisecond, func() { subscribeExit(attempt + 1) })
			} else {
				d.Logger.Warn("mention: gagal berlangganan session.exit", "err", err)
			}
		}
	}
	subscribeExit(1)

	mention.RegisterIPC(srv, q, namer, mention.IPCOptions{SessionCount: func() int {
		n := 0
		for _, s := range mgr.List() {
			if s.Status != session.StatusRunning {
				continue
			}
			if p, ok := provider.Get(s.ProviderID); ok && p.Capabilities().MCP {
				n++
			}
		}
		return n
	}})

	rt := &mention.Runtime{
		DataDir:       d.DataDir,
		Logger:        d.Logger,
		Sessions:      mgr,
		Namer:         namer,
		Enqueuer:      q,
		Reader:        q,
		Router:        q,
		OnSessionExit: hub.add,
	}
	if _, err := mention.Start(context.Background(), rt); err != nil {
		d.Logger.Warn("mention: gagal memulai server MCP", "err", err)
	}
}
