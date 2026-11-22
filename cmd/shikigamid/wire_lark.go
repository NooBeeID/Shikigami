package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/workspace"
)

func init() {
	RegisterModule("lark", func(srv *ipc.Server, d *Deps) { registerLarkSources(srv, d) })
}

func larkStore(d *Deps) (*workspace.Store, error) {
	v, ok := d.Lookup("workspace.store")
	st, isStore := v.(*workspace.Store)
	if !ok || !isStore || st == nil {
		return nil, ipc.NewError(ipc.ErrInternal, "workspace store tidak tersedia")
	}
	return st, nil
}

func larkErr(code string, ipcCode int, msg string) error {
	return ipc.NewError(ipcCode, "["+code+"] "+msg)
}

func mapLarkStoreErr(err error) error {
	switch {
	case errors.Is(err, workspace.ErrNotFound):
		if strings.Contains(err.Error(), "workspace") {
			return larkErr("not_found", ipc.ErrNotFound, "Workspace tidak ditemukan")
		}
		return larkErr("not_found", ipc.ErrNotFound, "Sumber tidak ditemukan")
	case errors.Is(err, workspace.ErrConflict):
		return larkErr("duplicate", ipc.ErrConflict, "Sumber sudah ada")
	case errors.Is(err, workspace.ErrInvalid):
		return larkErr("invalid_params", ipc.ErrInvalid, err.Error())
	default:
		return ipc.NewError(ipc.ErrInternal, err.Error())
	}
}

func larkDecode(params json.RawMessage, v any) error {
	t := bytes.TrimSpace(params)
	if len(t) == 0 || bytes.Equal(t, []byte("null")) {
		t = []byte("{}")
	}
	if err := json.Unmarshal(t, v); err != nil {
		return larkErr("invalid_params", ipc.ErrInvalid, "Parameter tidak valid")
	}
	return nil
}

func publishLarkChanged(srv *ipc.Server, st *workspace.Store, wsID string) {
	ws, err := st.Get(wsID)
	if err != nil {
		return
	}
	srv.Publish("workspace.changed", map[string]any{
		"kind": "configChanged", "workspaceId": wsID, "workspace": &ws,
	})
}

func registerLarkSources(srv *ipc.Server, d *Deps) {
	srv.Register("lark.sources.get", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
		}
		if err := larkDecode(params, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.WorkspaceID) == "" {
			return nil, larkErr("invalid_params", ipc.ErrInvalid, "workspaceId wajib diisi")
		}
		st, err := larkStore(d)
		if err != nil {
			return nil, err
		}
		lc, err := st.LarkSources(p.WorkspaceID)
		if err != nil {
			return nil, mapLarkStoreErr(err)
		}
		if lc.Tickets == nil {
			lc.Tickets = []workspace.TicketSource{}
		}
		if lc.Wiki == nil {
			lc.Wiki = []workspace.WikiSource{}
		}
		return map[string]any{"tickets": lc.Tickets, "wiki": lc.Wiki}, nil
	})

	srv.Register("lark.sources.add", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			Kind        string `json:"kind"`
			Name        string `json:"name"`
			BaseToken   string `json:"baseToken"`
			Host        string `json:"host"`
			TableID     string `json:"tableId"`
			ViewID      string `json:"viewId"`
			SpaceID     string `json:"spaceId"`
			RootToken   string `json:"rootToken"`
		}
		if err := larkDecode(params, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.WorkspaceID) == "" {
			return nil, larkErr("invalid_params", ipc.ErrInvalid, "workspaceId wajib diisi")
		}
		if p.Kind != "ticket" && p.Kind != "wiki" {
			return nil, larkErr("invalid_params", ipc.ErrInvalid, "kind harus ticket atau wiki")
		}
		if strings.TrimSpace(p.Name) == "" {
			return nil, larkErr("invalid_name", ipc.ErrInvalid, "Nama tidak boleh kosong")
		}
		host := strings.TrimSpace(p.Host)
		if p.Kind == "ticket" {
			if strings.TrimSpace(p.BaseToken) == "" || host == "" {
				return nil, larkErr("invalid_params", ipc.ErrInvalid, "baseToken dan host wajib diisi")
			}
		} else if strings.TrimSpace(p.SpaceID) == "" || host == "" {
			return nil, larkErr("invalid_params", ipc.ErrInvalid, "spaceId dan host wajib diisi")
		}
		st, err := larkStore(d)
		if err != nil {
			return nil, err
		}
		if p.Kind == "ticket" {
			src, err := st.AddLarkTicketSource(p.WorkspaceID, workspace.TicketSource{
				Name: p.Name, BaseToken: p.BaseToken, Host: p.Host, TableID: p.TableID, ViewID: p.ViewID,
			})
			if err != nil {
				return nil, mapLarkStoreErr(err)
			}
			publishLarkChanged(srv, st, p.WorkspaceID)
			return src, nil
		}
		src, err := st.AddLarkWikiSource(p.WorkspaceID, workspace.WikiSource{
			Name: p.Name, SpaceID: p.SpaceID, RootToken: p.RootToken, Host: p.Host,
		})
		if err != nil {
			return nil, mapLarkStoreErr(err)
		}
		publishLarkChanged(srv, st, p.WorkspaceID)
		return src, nil
	})

	srv.Register("lark.sources.remove", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			SourceID    string `json:"sourceId"`
		}
		if err := larkDecode(params, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.WorkspaceID) == "" || strings.TrimSpace(p.SourceID) == "" {
			return nil, larkErr("invalid_params", ipc.ErrInvalid, "workspaceId dan sourceId wajib diisi")
		}
		st, err := larkStore(d)
		if err != nil {
			return nil, err
		}
		if err := st.RemoveLarkSource(p.WorkspaceID, p.SourceID); err != nil {
			return nil, mapLarkStoreErr(err)
		}
		publishLarkChanged(srv, st, p.WorkspaceID)
		return map[string]any{}, nil
	})

	srv.Register("lark.sources.rename", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			SourceID    string `json:"sourceId"`
			Name        string `json:"name"`
		}
		if err := larkDecode(params, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.WorkspaceID) == "" || strings.TrimSpace(p.SourceID) == "" {
			return nil, larkErr("invalid_params", ipc.ErrInvalid, "workspaceId dan sourceId wajib diisi")
		}
		if strings.TrimSpace(p.Name) == "" {
			return nil, larkErr("invalid_name", ipc.ErrInvalid, "Nama tidak boleh kosong")
		}
		st, err := larkStore(d)
		if err != nil {
			return nil, err
		}
		if err := st.RenameLarkSource(p.WorkspaceID, p.SourceID, p.Name); err != nil {
			return nil, mapLarkStoreErr(err)
		}
		publishLarkChanged(srv, st, p.WorkspaceID)
		return map[string]any{}, nil
	})
}
