package runs

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/session"
)

// sessionLookup dipenuhi *status.Engine.
type sessionLookup interface {
	Lookup(sessionID string) (session.Session, bool)
}

// SessionIDsOfWorkspace membangun himpunan ID sesi milik workspace dari sesi
// yang dikenal registry dan di-resolve Engine ke workspace itu.
func SessionIDsOfWorkspace(reg *Registry, eng sessionLookup, workspaceID string) map[string]bool {
	set := map[string]bool{}
	for _, run := range reg.List(nil) {
		if set[run.SessionID] {
			continue
		}
		if s, ok := eng.Lookup(run.SessionID); ok && s.WorkspaceID == workspaceID {
			set[run.SessionID] = true
		}
	}
	return set
}

// Reconcile mencocokkan run workspace dengan berkas Claude: run selesai dibuang,
// run yang ditemukan ditandai Confirmed beserta Progress, run tak ditemukan
// yang sesinya sudah tak dikenal dibuang. Mengembalikan ID sesi yang berubah.
func Reconcile(reg *Registry, eng sessionLookup, workspaceID string) []string {
	changed := map[string]bool{}
	for _, run := range reg.List(nil) {
		s, known := eng.Lookup(run.SessionID)
		if known && s.WorkspaceID != workspaceID {
			continue
		}
		d := ReadDetail(reg.TranscriptPath(run.SessionID), run)
		switch {
		case d.Finished:
			if reg.Remove(run.ID) {
				changed[run.SessionID] = true
			}
		case d.Found:
			if !run.Confirmed || !reflect.DeepEqual(run.Progress, d.Progress) {
				reg.MarkConfirmed(run.ID, d.Progress)
				changed[run.SessionID] = true
			}
		case !known:
			if reg.Remove(run.ID) {
				changed[run.SessionID] = true
			}
		}
	}
	out := make([]string, 0, len(changed))
	for sid := range changed {
		out = append(out, sid)
	}
	sort.Strings(out)
	return out
}

// RegisterIPC mendaftarkan runs.list dan runs.detail. publish (boleh nil)
// dipanggil per sesi yang berubah setelah rekonsiliasi.
func RegisterIPC(srv *ipc.Server, reg *Registry, eng sessionLookup, publish func(sessionID string)) {
	workspaceParam := func(params json.RawMessage) (string, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return "", ipc.NewError(ipc.ErrInvalidParams, "parameter tidak valid")
		}
		if p.WorkspaceID == "" {
			return "", ipc.NewError(ipc.ErrInvalidParams, "workspaceId wajib diisi")
		}
		return p.WorkspaceID, nil
	}
	srv.Register("runs.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		ws, err := workspaceParam(params)
		if err != nil {
			return nil, err
		}
		return reg.List(SessionIDsOfWorkspace(reg, eng, ws)), nil
	})
	srv.Register("runs.detail", func(ctx context.Context, params json.RawMessage) (any, error) {
		ws, err := workspaceParam(params)
		if err != nil {
			return nil, err
		}
		for _, sid := range Reconcile(reg, eng, ws) {
			if publish != nil {
				publish(sid)
			}
		}
		return reg.List(SessionIDsOfWorkspace(reg, eng, ws)), nil
	})
}
