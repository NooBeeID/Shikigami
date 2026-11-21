package main

import (
	"aiterm/internal/ipc"
	"aiterm/internal/paths"
	"aiterm/internal/workspace"
)

func init() {
	RegisterModule("workspace", func(srv *ipc.Server, d *Deps) {
		dir := d.DataDir
		if dir == "" {
			p, err := paths.DataDir()
			if err != nil {
				logWorkspaceErr(d, err)
				return
			}
			dir = p
		}
		store, err := workspace.NewStore(dir)
		if err != nil {
			logWorkspaceErr(d, err)
			return
		}
		workspace.Register(srv, store)
	})
}

func logWorkspaceErr(d *Deps, err error) {
	if d.Logger != nil {
		d.Logger.Error("modul workspace dilewati", "err", err)
	}
}
