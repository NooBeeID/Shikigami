package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

type createParams struct {
	WorkspaceID string   `json:"workspaceId"`
	ProjectID   string   `json:"projectId"`
	Cwd         string   `json:"cwd"`
	ProviderID  string   `json:"providerId"`
	Cols        uint16   `json:"cols"`
	Rows        uint16   `json:"rows"`
	Env         []string `json:"env"`
}

type idParams struct {
	ID string `json:"id"`
}

type writeParams struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type resizeParams struct {
	ID   string `json:"id"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func decodeParams(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return ipc.Errorf(ipc.ErrInvalidParams, "params tidak valid: %v", err)
	}
	return nil
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var ie *ipc.Error
	if errors.As(err, &ie) {
		return err
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return ipc.NewError(ipc.ErrNotFound, err.Error())
	case errors.Is(err, ErrInvalid):
		return ipc.NewError(ipc.ErrInvalid, err.Error())
	case errors.Is(err, ErrNotRunning):
		return ipc.NewError(ipc.ErrConflict, err.Error())
	}
	return err
}

func requireID(id string) error {
	if id == "" {
		return ipc.NewError(ipc.ErrInvalidParams, "id wajib diisi")
	}
	return nil
}

// RegisterIPC mendaftarkan enam method session.* ke server IPC.
func RegisterIPC(srv *ipc.Server, m *Manager) {
	srv.Register("session.create", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p createParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if CreateGuard != nil {
			if err := CreateGuard(p.ProviderID); err != nil {
				return nil, err
			}
		}
		s, err := m.Create(SessionSpec{
			WorkspaceID: p.WorkspaceID,
			ProjectID:   p.ProjectID,
			Cwd:         p.Cwd,
			ProviderID:  p.ProviderID,
			Cols:        p.Cols,
			Rows:        p.Rows,
			Env:         p.Env,
		})
		if err != nil {
			return nil, mapErr(err)
		}
		return s, nil
	})

	srv.Register("session.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		list := m.List()
		if list == nil {
			list = []Session{}
		}
		return map[string]any{"sessions": list}, nil
	})

	srv.Register("session.kill", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p idParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if err := requireID(p.ID); err != nil {
			return nil, err
		}
		if err := m.Kill(p.ID); err != nil {
			return nil, mapErr(err)
		}
		return map[string]any{"ok": true}, nil
	})

	srv.Register("session.write", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p writeParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if err := requireID(p.ID); err != nil {
			return nil, err
		}
		b, err := base64.StdEncoding.DecodeString(p.Data)
		if err != nil {
			return nil, ipc.Errorf(ipc.ErrInvalid, "data bukan base64 valid: %v", err)
		}
		if err := m.Write(p.ID, b); err != nil {
			return nil, mapErr(err)
		}
		return map[string]any{"ok": true}, nil
	})

	srv.Register("session.resize", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p resizeParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if err := requireID(p.ID); err != nil {
			return nil, err
		}
		if err := m.Resize(p.ID, p.Cols, p.Rows); err != nil {
			return nil, mapErr(err)
		}
		return map[string]any{"ok": true}, nil
	})

	srv.Register("session.attach", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p idParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if err := requireID(p.ID); err != nil {
			return nil, err
		}
		r, err := m.Attach(p.ID)
		if err != nil {
			return nil, mapErr(err)
		}
		return map[string]any{
			"session":    r.Session,
			"scrollback": base64.StdEncoding.EncodeToString(r.Scrollback),
			"offset":     r.Offset,
		}, nil
	})
}

// CreateGuard, bila non-nil, dipanggil oleh session.create sebelum Manager.Create.
// Error dikembalikan apa adanya ke klien (boleh *ipc.Error). Dipasang saat wiring, sebelum Serve.
var CreateGuard func(providerID string) error
