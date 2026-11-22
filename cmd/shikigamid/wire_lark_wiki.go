package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/larkdata"
	"github.com/NooBeeID/shikigami/internal/workspace"
)

const (
	wikiCodeSourceNotFound = "source_not_found"
	wikiCodeInvalidParams  = "invalid_params"
	wikiPersonalID         = "personal"
)

func init() {
	RegisterModule("lark-wiki", func(srv *ipc.Server, d *Deps) {
		srv.Register("lark.wiki.list", wikiListHandler(d))
		srv.Register("lark.wiki.get", wikiGetHandler(d))
	})
}

func wikiErr(code int, tag, msg string) *ipc.Error {
	return ipc.NewError(code, "["+tag+"] "+msg)
}

func wikiMapErr(err error) error {
	var ie *ipc.Error
	if errors.As(err, &ie) {
		return ie
	}
	if le, ok := larkdata.AsError(err); ok {
		return wikiErr(ipc.ErrInternal, string(le.Code), le.Message)
	}
	return wikiErr(ipc.ErrInternal, "cli_error", err.Error())
}

func wikiStore(d *Deps) (*workspace.Store, error) {
	v, ok := d.Lookup("workspace.store")
	if ok {
		if s, ok := v.(*workspace.Store); ok && s != nil {
			return s, nil
		}
	}
	return nil, wikiErr(ipc.ErrInternal, "cli_error", "workspace store tidak tersedia")
}

type wikiTarget struct {
	SpaceID, RootToken, Host string
	Personal                 bool
}

func wikiSourceNotFound() error {
	return wikiErr(ipc.ErrNotFound, wikiCodeSourceNotFound, "Sumber tidak ada di workspace ini")
}

func wikiResolve(d *Deps, wsID, sourceID string) (wikiTarget, error) {
	if wsID == "" || sourceID == "" {
		return wikiTarget{}, wikiErr(ipc.ErrInvalid, wikiCodeInvalidParams, "workspaceId dan sourceId wajib diisi")
	}
	store, err := wikiStore(d)
	if err != nil {
		return wikiTarget{}, err
	}
	cfg, err := store.LarkSources(wsID)
	if err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return wikiTarget{}, wikiSourceNotFound()
		}
		return wikiTarget{}, wikiMapErr(err)
	}
	if sourceID == wikiPersonalID {
		return wikiTarget{SpaceID: larkdata.PersonalSpaceID, Personal: true, Host: wikiHostFallback(cfg)}, nil
	}
	for _, src := range cfg.Wiki {
		if src.ID == sourceID {
			return wikiTarget{SpaceID: src.SpaceID, RootToken: src.RootToken, Host: src.Host}, nil
		}
	}
	return wikiTarget{}, wikiSourceNotFound()
}

func wikiHostFallback(cfg workspace.LarkConfig) string {
	for _, w := range cfg.Wiki {
		if h := strings.TrimSpace(w.Host); h != "" {
			return h
		}
	}
	for _, t := range cfg.Tickets {
		if h := strings.TrimSpace(t.Host); h != "" {
			return h
		}
	}
	return ""
}

func wikiListHandler(d *Deps) func(ctx context.Context, params json.RawMessage) (any, error) {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			SourceID    string `json:"sourceId"`
			ParentToken string `json:"parentToken"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, wikiErr(ipc.ErrInvalid, wikiCodeInvalidParams, "Parameter tidak valid")
		}
		t, err := wikiResolve(d, p.WorkspaceID, p.SourceID)
		if err != nil {
			return nil, err
		}
		parent := strings.TrimSpace(p.ParentToken)
		if parent == "" && t.RootToken != "" {
			parent = t.RootToken
		}
		page, err := larkdata.ListWikiNodesPage(ctx, t.SpaceID, parent)
		if err != nil {
			if t.Personal {
				if le, ok := larkdata.AsError(err); ok && (le.Code == larkdata.CodePermission || le.Code == larkdata.CodeCLI) {
					return map[string]any{
						"nodes":        []larkdata.WikiNode{},
						"hidden":       true,
						"hiddenReason": "Wiki Pribadi tidak bisa ditampilkan: " + le.Message,
					}, nil
				}
			}
			return nil, wikiMapErr(err)
		}
		if page.Nodes == nil {
			page.Nodes = []larkdata.WikiNode{}
		}
		return map[string]any{"nodes": page.Nodes, "hasMore": page.HasMore}, nil
	}
}

func wikiGetHandler(d *Deps) func(ctx context.Context, params json.RawMessage) (any, error) {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			SourceID    string `json:"sourceId"`
			NodeToken   string `json:"nodeToken"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, wikiErr(ipc.ErrInvalid, wikiCodeInvalidParams, "Parameter tidak valid")
		}
		nodeToken := strings.TrimSpace(p.NodeToken)
		if nodeToken == "" {
			return nil, wikiErr(ipc.ErrInvalid, wikiCodeInvalidParams, "nodeToken wajib diisi")
		}
		t, err := wikiResolve(d, p.WorkspaceID, p.SourceID)
		if err != nil {
			return nil, err
		}
		info, err := larkdata.GetWikiNode(ctx, t.SpaceID, nodeToken, t.Host)
		if err != nil {
			return nil, wikiMapErr(err)
		}
		res := map[string]any{"title": info.Title, "objType": info.ObjType, "url": info.URL}
		switch info.ObjType {
		case "docx":
			md, err := larkdata.FetchDocMarkdown(ctx, info.ObjToken)
			if err != nil {
				return nil, wikiMapErr(err)
			}
			res["markdown"] = md
		case "sheet":
			sc, err := larkdata.FetchSheetCells(ctx, info.ObjToken)
			if err != nil {
				return nil, wikiMapErr(err)
			}
			res["cells"] = sc.Cells
			res["truncated"] = sc.Truncated
		}
		return res, nil
	}
}
