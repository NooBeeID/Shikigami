package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/NooBeeID/shikigami/internal/gitdata"
	"github.com/NooBeeID/shikigami/internal/ipc"
)

func init() {
	RegisterModule("git-mrs", func(srv *ipc.Server, d *Deps) { registerGitMrs(srv, d) })
}

func registerGitMrs(srv *ipc.Server, d *Deps) {
	srv.Register("git.mrs.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			ProjectID   string `json:"projectId"`
			State       string `json:"state"`
		}
		if err := gitDecode(params, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.WorkspaceID) == "" {
			return nil, gitErr("invalid_params", ipc.ErrInvalid, "workspaceId wajib diisi")
		}
		if strings.TrimSpace(p.ProjectID) == "" {
			return nil, gitErr("invalid_params", ipc.ErrInvalid, "projectId wajib diisi")
		}
		switch p.State {
		case "open", "merged", "closed":
		default:
			return nil, gitErr("invalid_params", ipc.ErrInvalid, "state harus open, merged, atau closed")
		}
		repo, err := gitResolveRepo(d, p.WorkspaceID, p.ProjectID)
		if err != nil {
			return nil, mapGitError(err)
		}
		mrs, err := gitdata.ListMergeRequests(ctx, repo, p.State)
		if err != nil {
			return nil, mapGitError(err)
		}
		if mrs == nil {
			mrs = []gitdata.MergeRequest{}
		}
		return map[string]any{"mrs": mrs}, nil
	})
}
