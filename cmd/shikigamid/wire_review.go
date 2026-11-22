package main

// Spike format tool_input hook PreToolUse/PostToolUse (belum diverifikasi di
// lapangan; berdasarkan dokumentasi Claude Code yang diketahui):
//   - Edit, Write: tool_input.file_path (path absolut).
//   - MultiEdit: tool_input.file_path + edits[] (satu path untuk seluruh edit).
//   - NotebookEdit: tool_input.notebook_path; beberapa versi memakai file_path,
//     jadi ekstraksi mencoba notebook_path lalu file_path.
// Bila kunci path tidak ada atau bukan string non-kosong, event diabaikan tanpa galat.

import (
	"log/slog"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/provider"
	"github.com/NooBeeID/shikigami/internal/review"
	"github.com/NooBeeID/shikigami/internal/status"
)

var reviewWireEditTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

// reviewWireToolPath mengambil tool_name (harus tool edit) dan path dari payload hook.
func reviewWireToolPath(payload map[string]any) (tool, path string, ok bool) {
	tool, _ = payload["tool_name"].(string)
	if !reviewWireEditTools[tool] {
		return "", "", false
	}
	input, isMap := payload["tool_input"].(map[string]any)
	if !isMap {
		return "", "", false
	}
	keys := []string{"file_path"}
	if tool == "NotebookEdit" {
		keys = []string{"notebook_path", "file_path"}
	}
	for _, k := range keys {
		if v, isStr := input[k].(string); isStr && v != "" {
			return tool, v, true
		}
	}
	return "", "", false
}

// reviewWireCwd menentukan cwd sesi: Lookup(sessionID).Cwd, lalu payload["cwd"].
func reviewWireCwd(eng *status.Engine, sessionID string, payload map[string]any) string {
	if s, ok := eng.Lookup(sessionID); ok && s.Cwd != "" {
		return s.Cwd
	}
	if c, ok := payload["cwd"].(string); ok {
		return c
	}
	return ""
}

// reviewWireSupported memetakan sesi -> dukungan hooks provider.
func reviewWireSupported(eng *status.Engine) func(sessionID string) bool {
	return func(id string) bool {
		s, ok := eng.Lookup(id)
		if !ok {
			return false
		}
		p, ok := provider.Get(s.ProviderID)
		if !ok {
			return false
		}
		return p.Capabilities().Hooks
	}
}

type reviewWireDebouncer struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
	delay  time.Duration
}

// schedule mereset timer sesi; fn jalan setelah delay tanpa event baru.
func (b *reviewWireDebouncer) schedule(sessionID string, fn func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if old, ok := b.timers[sessionID]; ok {
		old.Stop()
	}
	var t *time.Timer
	t = time.AfterFunc(b.delay, func() {
		b.mu.Lock()
		if b.timers[sessionID] == t {
			delete(b.timers, sessionID)
		}
		b.mu.Unlock()
		defer func() { _ = recover() }()
		fn()
	})
	b.timers[sessionID] = t
}

// forget membatalkan timer tertunda sesi.
func (b *reviewWireDebouncer) forget(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if t, ok := b.timers[sessionID]; ok {
		t.Stop()
		delete(b.timers, sessionID)
	}
}

func init() {
	RegisterModule("review", func(srv *ipc.Server, d *Deps) {
		store := review.NewStore(d.DataDir)
		d.Provide("review.store", store)
		d.OnReady(func() {
			v, ok := d.Lookup("status.engine")
			eng, ok2 := v.(*status.Engine)
			if !ok || !ok2 {
				if d.Logger != nil {
					d.Logger.Warn("review: status.engine tidak tersedia")
				}
				return
			}
			review.RegisterIPC(srv, store, reviewWireSupported(eng))
			review.RegisterDiffIPC(srv, store, reviewWireSupported(eng))
			review.RegisterActions(srv, store, reviewWireSupported(eng))
			log := d.Logger
			if log == nil {
				log = slog.Default()
			}
			deb := &reviewWireDebouncer{timers: map[string]*time.Timer{}, delay: 300 * time.Millisecond}

			eng.AddHookListener(func(sessionID, event string, payload map[string]any) {
				defer func() { _ = recover() }()
				_, path, ok := reviewWireToolPath(payload)
				if !ok {
					return
				}
				cwd := reviewWireCwd(eng, sessionID, payload)
				if cwd == "" {
					log.Debug("review: cwd kosong", "session", sessionID)
					return
				}
				switch event {
				case status.HookPreToolUse:
					if err := store.Capture(sessionID, cwd, path); err != nil {
						log.Debug("review: capture gagal", "session", sessionID, "err", err)
					}
				case status.HookPostToolUse:
					if err := store.MarkChanged(sessionID, cwd, path); err != nil {
						log.Debug("review: mark changed gagal", "session", sessionID, "err", err)
					}
					deb.schedule(sessionID, func() {
						fcwd := store.Cwd(sessionID)
						if fcwd == "" {
							fcwd = cwd
						}
						files, err := store.Files(sessionID, fcwd)
						if err != nil {
							log.Warn("review: baca daftar gagal", "session", sessionID, "err", err)
							return
						}
						n := 0
						for _, f := range files {
							if !f.Reviewed {
								n++
							}
						}
						srv.Publish(review.EventChanged, map[string]any{"sessionId": sessionID, "count": n})
					})
				}
			})

			eng.AddExitListener(func(sessionID string) { deb.forget(sessionID) })

			go func() {
				defer func() { _ = recover() }()
				ids := make([]string, 0)
				for _, st := range eng.List() {
					ids = append(ids, st.SessionID)
				}
				store.Prune(20, ids)
			}()
		})
	})
}
