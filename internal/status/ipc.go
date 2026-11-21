package status

import (
	"context"
	"encoding/json"
	"errors"

	"aiterm/internal/ipc"
)

// RegisterIPC mendaftarkan status.get, status.list, dan status.hook.
func RegisterIPC(srv *ipc.Server, eng *Engine) {
	srv.Register("status.get", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, ipc.NewError(ipc.ErrInvalidParams, "parameter tidak valid")
		}
		if p.SessionID == "" {
			return nil, ipc.NewError(ipc.ErrInvalidParams, "sessionId wajib diisi")
		}
		_ = eng.ensureStream()
		id, ok := eng.ResolveSessionID(p.SessionID)
		if !ok {
			return nil, ipc.NewError(ipc.ErrNotFound, "sesi tidak dikenal")
		}
		st, ok := eng.Get(id)
		if !ok {
			return nil, ipc.NewError(ipc.ErrNotFound, "sesi tidak dikenal")
		}
		return st, nil
	})
	srv.Register("status.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		_ = eng.ensureStream()
		return eng.List(), nil
	})
	srv.Register("status.hook", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			SessionID string         `json:"sessionId"`
			Event     string         `json:"event"`
			Payload   map[string]any `json:"payload"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, ipc.NewError(ipc.ErrInvalidParams, "parameter tidak valid")
		}
		if p.SessionID == "" || p.Event == "" {
			return nil, ipc.NewError(ipc.ErrInvalidParams, "sessionId dan event wajib diisi")
		}
		_ = eng.ensureStream()
		if err := eng.Apply(p.SessionID, p.Event, p.Payload); err != nil {
			if errors.Is(err, ErrUnknownSession) {
				return nil, ipc.NewError(ipc.ErrNotFound, "sesi tidak dikenal")
			}
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})
}
