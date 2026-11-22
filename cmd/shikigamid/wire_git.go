package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/NooBeeID/shikigami/internal/gitdata"
	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/workspace"
)

func init() {
	RegisterModule("git", func(srv *ipc.Server, d *Deps) { registerGitRepos(srv, d) })
}

func gitStore(d *Deps) (*workspace.Store, error) {
	v, ok := d.Lookup("workspace.store")
	st, isStore := v.(*workspace.Store)
	if !ok || !isStore || st == nil {
		return nil, ipc.NewError(ipc.ErrInternal, "workspace store tidak tersedia")
	}
	return st, nil
}

func gitErr(code string, ipcCode int, msg string) error {
	return ipc.NewError(ipcCode, "["+code+"] "+msg)
}

func mapGitError(err error) error {
	if err == nil {
		return nil
	}
	var ie *ipc.Error
	if errors.As(err, &ie) {
		return err
	}
	if e, ok := gitdata.AsError(err); ok {
		ipcCode := ipc.ErrInternal
		switch e.Code {
		case gitdata.CodeNotFound:
			ipcCode = ipc.ErrNotFound
		case gitdata.CodeNotAllowed:
			ipcCode = ipc.ErrInvalid
		}
		return gitErr(string(e.Code), ipcCode, e.Message)
	}
	return ipc.NewError(ipc.ErrInternal, err.Error())
}

func gitDecode(params json.RawMessage, v any) error {
	t := bytes.TrimSpace(params)
	if len(t) == 0 || bytes.Equal(t, []byte("null")) {
		t = []byte("{}")
	}
	if err := json.Unmarshal(t, v); err != nil {
		return gitErr("invalid_params", ipc.ErrInvalid, "Parameter tidak valid")
	}
	return nil
}

func gitDetect(ctx context.Context, d *Deps, workspaceID string) ([]gitdata.Repo, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, gitErr("invalid_params", ipc.ErrInvalid, "workspaceId wajib diisi")
	}
	st, err := gitStore(d)
	if err != nil {
		return nil, err
	}
	ws, err := st.Get(workspaceID)
	if err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return nil, gitErr("not_found", ipc.ErrNotFound, "Workspace tidak ditemukan")
		}
		return nil, ipc.NewError(ipc.ErrInternal, err.Error())
	}
	inputs := make([]gitdata.ProjectInput, 0, len(ws.Projects))
	for _, p := range ws.Projects {
		inputs = append(inputs, gitdata.ProjectInput{ID: p.ID, Name: p.Name, Path: p.Path})
	}
	repos, err := gitdata.DetectRepos(ctx, inputs)
	if err != nil {
		return nil, mapGitError(err)
	}
	if repos == nil {
		repos = []gitdata.Repo{}
	}
	return repos, nil
}

func gitResolveRepo(d *Deps, workspaceID, projectID string) (gitdata.Repo, error) {
	repos, err := gitDetect(context.Background(), d, workspaceID)
	if err != nil {
		return gitdata.Repo{}, err
	}
	if projectID != "" {
		for _, r := range repos {
			if r.ProjectID == projectID {
				return r, nil
			}
			for _, p := range r.Projects {
				if p.ID == projectID {
					return r, nil
				}
			}
		}
	}
	return gitdata.Repo{}, gitErr("not_found", ipc.ErrNotFound, "Project tidak ditemukan atau tidak punya remote dikenal")
}

func registerGitRepos(srv *ipc.Server, d *Deps) {
	srv.Register("git.repos.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
		}
		if err := gitDecode(params, &p); err != nil {
			return nil, err
		}
		repos, err := gitDetect(ctx, d, p.WorkspaceID)
		if err != nil {
			return nil, err
		}
		return repos, nil
	})
}
