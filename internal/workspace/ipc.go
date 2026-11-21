package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"aiterm/internal/ipc"
)

// mapErr memetakan error sentinel Store ke kode error IPC.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNotFound):
		return ipc.NewError(ipc.ErrNotFound, err.Error())
	case errors.Is(err, ErrInvalid):
		return ipc.NewError(ipc.ErrInvalid, err.Error())
	case errors.Is(err, ErrConflict):
		return ipc.NewError(ipc.ErrConflict, err.Error())
	default:
		return err
	}
}

// decode mendekode params; params kosong/null dianggap {}.
func decode(params json.RawMessage, v any) error {
	t := bytes.TrimSpace(params)
	if len(t) == 0 || bytes.Equal(t, []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(t, v); err != nil {
		return ipc.NewError(ipc.ErrInvalid, "params tidak valid: "+err.Error())
	}
	return nil
}

func publish(srv *ipc.Server, kind, wsID string, ws *Workspace) {
	srv.Publish("workspace.changed", map[string]any{
		"kind":        kind,
		"workspaceId": wsID,
		"workspace":   ws,
	})
}

func configResult(s *Store, wsID string) (any, error) {
	g, wc, eff, err := s.GetConfig(wsID)
	if err != nil {
		return nil, mapErr(err)
	}
	if wsID == "" {
		return map[string]any{"global": g}, nil
	}
	return map[string]any{"global": g, "workspace": wc, "effective": eff}, nil
}

// Register mendaftarkan method IPC workspace.* dan config.* untuk Store.
func Register(srv *ipc.Server, s *Store) {
	srv.Register("workspace.create", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Name string `json:"name"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		ws, err := s.Create(p.Name)
		if err != nil {
			return nil, mapErr(err)
		}
		publish(srv, "created", ws.ID, &ws)
		return ws, nil
	})

	srv.Register("workspace.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct{}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return s.List(), nil
	})

	srv.Register("workspace.rename", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		ws, err := s.Rename(p.ID, p.Name)
		if err != nil {
			return nil, mapErr(err)
		}
		publish(srv, "renamed", ws.ID, &ws)
		return ws, nil
	})

	srv.Register("workspace.delete", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			ID string `json:"id"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		if err := s.Delete(p.ID); err != nil {
			return nil, mapErr(err)
		}
		publish(srv, "deleted", p.ID, nil)
		return map[string]bool{"ok": true}, nil
	})

	srv.Register("workspace.addProject", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			Path        string `json:"path"`
			Name        string `json:"name"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		ws, proj, err := s.AddProject(p.WorkspaceID, p.Path, p.Name)
		if err != nil {
			return nil, mapErr(err)
		}
		publish(srv, "projectAdded", ws.ID, &ws)
		return map[string]any{"workspace": ws, "project": proj}, nil
	})

	srv.Register("workspace.removeProject", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			ProjectID   string `json:"projectId"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		ws, err := s.RemoveProject(p.WorkspaceID, p.ProjectID)
		if err != nil {
			return nil, mapErr(err)
		}
		publish(srv, "projectRemoved", ws.ID, &ws)
		return ws, nil
	})

	srv.Register("config.get", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return configResult(s, p.WorkspaceID)
	})

	srv.Register("config.set", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string  `json:"workspaceId"`
			GrimoireURL *string `json:"grimoireUrl"`
			GrimoireKey *string `json:"grimoireKey"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		g, wc, _, err := s.GetConfig(p.WorkspaceID)
		if err != nil {
			return nil, mapErr(err)
		}
		if p.WorkspaceID == "" {
			if p.GrimoireURL != nil {
				g.GrimoireURL = *p.GrimoireURL
			}
			if p.GrimoireKey != nil {
				g.GrimoireKey = *p.GrimoireKey
			}
			if err := s.SetGlobalConfig(g); err != nil {
				return nil, mapErr(err)
			}
			publish(srv, "configChanged", "", nil)
		} else {
			if p.GrimoireURL != nil {
				wc.GrimoireURL = *p.GrimoireURL
			}
			if p.GrimoireKey != nil {
				wc.GrimoireKey = *p.GrimoireKey
			}
			ws, err := s.SetWorkspaceConfig(p.WorkspaceID, wc)
			if err != nil {
				return nil, mapErr(err)
			}
			publish(srv, "configChanged", p.WorkspaceID, &ws)
		}
		return configResult(s, p.WorkspaceID)
	})
}
