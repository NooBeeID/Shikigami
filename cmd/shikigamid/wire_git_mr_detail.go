package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/NooBeeID/shikigami/internal/gitdata"
	"github.com/NooBeeID/shikigami/internal/ipc"
)

func init() {
	RegisterModule("git-mr-detail", func(srv *ipc.Server, d *Deps) { registerGitMrDetail(srv, d) })
}

func registerGitMrDetail(srv *ipc.Server, d *Deps) {
	srv.Register("git.mr.get", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			ProjectID   string `json:"projectId"`
			Number      int    `json:"number"`
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
		if p.Number <= 0 {
			return nil, gitErr("invalid_params", ipc.ErrInvalid, "number wajib berupa bilangan bulat lebih dari 0")
		}
		repo, err := gitResolveRepo(d, p.WorkspaceID, p.ProjectID)
		if err != nil {
			return nil, mapGitError(err)
		}
		det, err := gitdata.GetMergeRequest(ctx, repo, p.Number)
		if err != nil {
			return nil, mapGitError(err)
		}
		if det.Labels == nil {
			det.Labels = []string{}
		}
		if det.Reviewers == nil {
			det.Reviewers = []gitdata.Reviewer{}
		}
		return det, nil
	})
}
