package main

import (
	"context"
	"errors"

	"github.com/NooBeeID/shikigami/internal/grimoire"
	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/paths"
)

func init() {
	RegisterModule("grimoire", func(srv *ipc.Server, d *Deps) {
		dataDir := d.DataDir
		if dataDir == "" {
			dd, err := paths.DataDir()
			if err != nil {
				if d.Logger != nil {
					d.Logger.Warn("modul grimoire dilewati", "err", err)
				}
				return
			}
			dataDir = dd
		}
		grimoire.Register(srv, grimoire.Deps{
			Store:       grimoire.NewStore(dataDir),
			ResolvePath: newGrimoireResolver(d.SocketPath),
			Logger:      d.Logger,
		})
	})
}

// newGrimoireResolver menyelesaikan projectId menjadi Project.Path lewat workspace.list.
func newGrimoireResolver(socketPath string) func(ctx context.Context, projectID string) (string, error) {
	return func(ctx context.Context, projectID string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if socketPath == "" {
			return "", ipc.NewError(ipc.ErrInternal, "socket daemon tidak diketahui")
		}
		c, err := ipc.Dial(socketPath)
		if err != nil {
			return "", ipc.NewError(ipc.ErrInternal, "gagal menghubungi daemon: "+err.Error())
		}
		defer c.Close()

		var list []struct {
			Projects []struct {
				ID   string `json:"id"`
				Path string `json:"path"`
			} `json:"projects"`
		}
		if err := c.Call("workspace.list", map[string]any{}, &list); err != nil {
			var ie *ipc.Error
			if errors.As(err, &ie) {
				return "", err
			}
			return "", ipc.NewError(ipc.ErrInternal, err.Error())
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		for _, w := range list {
			for _, p := range w.Projects {
				if p.ID != projectID {
					continue
				}
				if p.Path == "" {
					return "", ipc.NewError(ipc.ErrNotFound, "path project kosong: "+projectID)
				}
				return p.Path, nil
			}
		}
		return "", ipc.NewError(ipc.ErrNotFound, "project tidak dikenal: "+projectID)
	}
}
