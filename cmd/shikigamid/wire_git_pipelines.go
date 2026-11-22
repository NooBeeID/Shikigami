package main

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/NooBeeID/shikigami/internal/gitdata"
	"github.com/NooBeeID/shikigami/internal/ipc"
)

var gitPipelinesIDPattern = regexp.MustCompile("^[0-9]+$")

func init() {
	RegisterModule("git-pipelines", func(srv *ipc.Server, d *Deps) { registerGitPipelines(srv, d) })
}

func registerGitPipelines(srv *ipc.Server, d *Deps) {
	srv.Register("git.pipelines.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			ProjectID   string `json:"projectId"`
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
		repo, err := gitResolveRepo(d, p.WorkspaceID, p.ProjectID)
		if err != nil {
			return nil, mapGitError(err)
		}
		list, err := gitdata.ListPipelines(ctx, repo)
		if err != nil {
			return nil, mapGitError(err)
		}
		if list == nil {
			list = []gitdata.Pipeline{}
		}
		return list, nil
	})

	srv.Register("git.pipeline.get", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			ProjectID   string `json:"projectId"`
			ID          string `json:"id"`
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
		if !gitPipelinesIDPattern.MatchString(p.ID) {
			return nil, gitErr("invalid_params", ipc.ErrInvalid, "id pipeline wajib berupa angka")
		}
		repo, err := gitResolveRepo(d, p.WorkspaceID, p.ProjectID)
		if err != nil {
			return nil, mapGitError(err)
		}
		det, err := gitdata.GetPipeline(ctx, repo, p.ID)
		if err != nil {
			return nil, mapGitError(err)
		}
		if det.Stages == nil {
			det.Stages = []gitdata.Stage{}
		}
		return det, nil
	})
}
