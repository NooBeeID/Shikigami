package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/larkdata"
	"github.com/NooBeeID/shikigami/internal/workspace"
)

type ticketsListParams struct {
	WorkspaceID string `json:"workspaceId"`
	SourceID    string `json:"sourceId"`
	TableID     string `json:"tableId"`
	ViewID      string `json:"viewId"`
	Offset      int    `json:"offset"`
}

type ticketsGetParams struct {
	WorkspaceID string `json:"workspaceId"`
	SourceID    string `json:"sourceId"`
	RecordID    string `json:"recordId"`
	TableID     string `json:"tableId"`
	ViewID      string `json:"viewId"`
}

func init() {
	RegisterModule("lark-tickets", func(srv *ipc.Server, d *Deps) {
		srv.Register("lark.tickets.list", func(ctx context.Context, params json.RawMessage) (any, error) {
			var p ticketsListParams
			if len(params) > 0 {
				if err := json.Unmarshal(params, &p); err != nil {
					return nil, ticketsErr("invalid_params", "Parameter tidak valid")
				}
			}
			if strings.TrimSpace(p.WorkspaceID) == "" || strings.TrimSpace(p.SourceID) == "" {
				return nil, ticketsErr("invalid_params", "Parameter tidak valid")
			}
			tableID := strings.TrimSpace(p.TableID)
			if tableID == "" {
				return nil, ticketsErr("invalid_params", "Tabel belum dipilih")
			}
			if p.Offset < 0 {
				p.Offset = 0
			}
			store, err := ticketsStore(d)
			if err != nil {
				return nil, err
			}
			src, err := ticketsSource(store, p.WorkspaceID, p.SourceID)
			if err != nil {
				return nil, err
			}
			page, err := larkdata.ListTickets(ctx, larkdata.TicketListQuery{
				BaseToken: src.BaseToken, TableID: tableID, ViewID: p.ViewID, Offset: p.Offset,
			})
			if err != nil {
				return nil, ticketsMapErr(err)
			}
			return page, nil
		})
		srv.Register("lark.ticket.get", func(ctx context.Context, params json.RawMessage) (any, error) {
			var p ticketsGetParams
			if len(params) > 0 {
				if err := json.Unmarshal(params, &p); err != nil {
					return nil, ticketsErr("invalid_params", "Parameter tidak valid")
				}
			}
			if strings.TrimSpace(p.WorkspaceID) == "" || strings.TrimSpace(p.SourceID) == "" {
				return nil, ticketsErr("invalid_params", "Parameter tidak valid")
			}
			if strings.TrimSpace(p.RecordID) == "" {
				return nil, ticketsErr("invalid_params", "ID tiket kosong")
			}
			store, err := ticketsStore(d)
			if err != nil {
				return nil, err
			}
			src, err := ticketsSource(store, p.WorkspaceID, p.SourceID)
			if err != nil {
				return nil, err
			}
			tableID := strings.TrimSpace(p.TableID)
			if tableID == "" {
				tableID = src.TableID
			}
			if strings.TrimSpace(tableID) == "" {
				return nil, ticketsErr("invalid_params", "Tabel belum dipilih")
			}
			viewID := p.ViewID
			if viewID == "" {
				viewID = src.ViewID
			}
			detail, err := larkdata.GetTicket(ctx, larkdata.TicketGetQuery{
				BaseToken: src.BaseToken, Host: src.Host, TableID: tableID, ViewID: viewID, RecordID: p.RecordID,
			})
			if err != nil {
				return nil, ticketsMapErr(err)
			}
			return detail, nil
		})
	})
}

func ticketsStore(d *Deps) (*workspace.Store, error) {
	if v, ok := d.Lookup("workspace.store"); ok {
		if s, ok := v.(*workspace.Store); ok && s != nil {
			return s, nil
		}
	}
	return nil, ticketsErr("cli_error", "Penyimpanan workspace belum siap")
}

func ticketsSource(store *workspace.Store, wsID, sourceID string) (workspace.TicketSource, error) {
	cfg, err := store.LarkSources(wsID)
	if err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return workspace.TicketSource{}, ticketsErr("not_found", "Sumber tidak ada di workspace ini")
		}
		if errors.Is(err, workspace.ErrInvalid) {
			return workspace.TicketSource{}, ticketsErr("invalid_params", "Parameter tidak valid")
		}
		return workspace.TicketSource{}, ticketsErr("cli_error", err.Error())
	}
	for _, t := range cfg.Tickets {
		if t.ID == sourceID {
			return t, nil
		}
	}
	return workspace.TicketSource{}, ticketsErr("not_found", "Sumber tidak ada di workspace ini")
}

func ticketsErr(code, msg string) error {
	c := ipc.ErrInternal
	switch code {
	case "invalid_params":
		c = ipc.ErrInvalid
	case "not_found":
		c = ipc.ErrNotFound
	}
	return ipc.NewError(c, "["+code+"] "+msg)
}

func ticketsMapErr(err error) error {
	if err == nil {
		return nil
	}
	var ie *ipc.Error
	if errors.As(err, &ie) {
		return err
	}
	if e, ok := larkdata.AsError(err); ok {
		msg := e.Message
		if strings.HasPrefix(msg, "[") {
			return ipc.NewError(ipc.ErrInternal, msg)
		}
		return ticketsErr(string(e.Code), msg)
	}
	return ticketsErr("cli_error", fmt.Sprint(err))
}
