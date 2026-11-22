package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/larkdata"
	"github.com/NooBeeID/shikigami/internal/workspace"
)

func init() {
	RegisterModule("lark-link", func(srv *ipc.Server, d *Deps) {
		srv.Register("lark.link.parse", func(ctx context.Context, params json.RawMessage) (any, error) {
			var p struct {
				Link string `json:"link"`
			}
			if err := linkDecode(params, &p); err != nil {
				return nil, err
			}
			if strings.TrimSpace(p.Link) == "" {
				return nil, ipc.NewError(ipc.ErrInvalid, "[invalid_link] Link tidak dikenali")
			}
			res, err := larkdata.ParseLink(ctx, p.Link)
			if err != nil {
				return nil, linkErr(err)
			}
			return res, nil
		})
		srv.Register("lark.tables.list", func(ctx context.Context, params json.RawMessage) (any, error) {
			var p struct {
				WorkspaceID string `json:"workspaceId"`
				SourceID    string `json:"sourceId"`
			}
			if err := linkDecode(params, &p); err != nil {
				return nil, err
			}
			src, err := linkSource(d, p.WorkspaceID, p.SourceID)
			if err != nil {
				return nil, err
			}
			tables, err := larkdata.ListTables(ctx, src.BaseToken, src.Host)
			if err != nil {
				return nil, linkErr(err)
			}
			if tables == nil {
				tables = []larkdata.Table{}
			}
			return map[string]any{"tables": tables}, nil
		})
		srv.Register("lark.views.list", func(ctx context.Context, params json.RawMessage) (any, error) {
			var p struct {
				WorkspaceID string `json:"workspaceId"`
				SourceID    string `json:"sourceId"`
				TableID     string `json:"tableId"`
			}
			if err := linkDecode(params, &p); err != nil {
				return nil, err
			}
			src, err := linkSource(d, p.WorkspaceID, p.SourceID)
			if err != nil {
				return nil, err
			}
			if p.TableID == "" {
				return nil, ipc.NewError(ipc.ErrInvalid, "[invalid_params] Parameter tidak lengkap: tableId")
			}
			views, err := larkdata.ListViews(ctx, src.BaseToken, p.TableID)
			if err != nil {
				return nil, linkErr(err)
			}
			if views == nil {
				views = []larkdata.View{}
			}
			return map[string]any{"views": views}, nil
		})
	})
}

func linkStore(d *Deps) (*workspace.Store, error) {
	v, ok := d.Lookup("workspace.store")
	if ok {
		if s, ok := v.(*workspace.Store); ok && s != nil {
			return s, nil
		}
	}
	return nil, ipc.NewError(ipc.ErrInternal, "[cli_error] workspace store tidak tersedia")
}

func linkDecode(params json.RawMessage, v any) error {
	b := bytes.TrimSpace(params)
	if len(b) == 0 || string(b) == "null" {
		b = []byte("{}")
	}
	if err := json.Unmarshal(b, v); err != nil {
		return ipc.NewError(ipc.ErrInvalid, "[invalid_params] params tidak valid")
	}
	return nil
}

func linkErr(err error) error {
	if err == nil {
		return nil
	}
	var ie *ipc.Error
	if errors.As(err, &ie) {
		return err
	}
	if e, ok := larkdata.AsError(err); ok {
		if e.Code == larkdata.CodeInvalidLink {
			return ipc.NewError(ipc.ErrInvalid, "["+string(e.Code)+"] "+e.Message)
		}
		return ipc.NewError(ipc.ErrInternal, "["+string(e.Code)+"] "+e.Message)
	}
	return ipc.NewError(ipc.ErrInternal, "[cli_error] "+err.Error())
}

func linkSource(d *Deps, wsID, sourceID string) (workspace.TicketSource, error) {
	if wsID == "" {
		return workspace.TicketSource{}, ipc.NewError(ipc.ErrInvalid, "[invalid_params] Parameter tidak lengkap: workspaceId")
	}
	if sourceID == "" {
		return workspace.TicketSource{}, ipc.NewError(ipc.ErrInvalid, "[invalid_params] Parameter tidak lengkap: sourceId")
	}
	store, err := linkStore(d)
	if err != nil {
		return workspace.TicketSource{}, err
	}
	cfg, err := store.LarkSources(wsID)
	if err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return workspace.TicketSource{}, ipc.NewError(ipc.ErrNotFound, "[not_found] Sumber tidak ditemukan")
		}
		return workspace.TicketSource{}, ipc.NewError(ipc.ErrInternal, "[cli_error] "+err.Error())
	}
	for _, t := range cfg.Tickets {
		if t.ID == sourceID {
			return t, nil
		}
	}
	return workspace.TicketSource{}, ipc.NewError(ipc.ErrNotFound, "[not_found] Sumber tidak ditemukan")
}
