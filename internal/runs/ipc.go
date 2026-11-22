package runs

import (
	"context"
	"encoding/json"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/status"
)

// SessionIDsOfWorkspace membangun himpunan ID sesi milik workspace dari sesi
// yang dikenal registry dan di-resolve Engine ke workspace itu.
func SessionIDsOfWorkspace(reg *Registry, eng *status.Engine, workspaceID string) map[string]bool {
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

// RegisterIPC mendaftarkan runs.list. Event runs.changed diterbitkan oleh wiring daemon.
func RegisterIPC(srv *ipc.Server, reg *Registry, eng *status.Engine) {
	srv.Register("runs.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, ipc.NewError(ipc.ErrInvalidParams, "parameter tidak valid")
		}
		if p.WorkspaceID == "" {
			return nil, ipc.NewError(ipc.ErrInvalidParams, "workspaceId wajib diisi")
		}
		return reg.List(SessionIDsOfWorkspace(reg, eng, p.WorkspaceID)), nil
	})
}
