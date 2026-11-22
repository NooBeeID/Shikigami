package main

import (
	"github.com/NooBeeID/shikigami/internal/agents"
	"github.com/NooBeeID/shikigami/internal/ipc"
)

func init() {
	RegisterModule("agents", func(srv *ipc.Server, d *Deps) {
		store := agents.Open(agents.DefaultPath(d.DataDir), nil)
		d.Provide("agents.store", store)
		agents.RegisterIPC(srv, store)
	})
}
