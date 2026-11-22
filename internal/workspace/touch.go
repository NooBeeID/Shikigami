package workspace

import (
	"context"
	"encoding/json"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

// registerTouch mendaftarkan RPC workspace.touch {id} -> Workspace.
func registerTouch(srv *ipc.Server, s *Store) {
	srv.Register("workspace.touch", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			ID string `json:"id"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		ws, err := s.Touch(p.ID)
		if err != nil {
			return nil, mapErr(err)
		}
		publish(srv, "updated", ws.ID, &ws)
		return ws, nil
	})
}
