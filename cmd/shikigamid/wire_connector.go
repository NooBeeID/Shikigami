package main

import (
	"github.com/NooBeeID/shikigami/internal/connector"
	"github.com/NooBeeID/shikigami/internal/ipc"
)

func init() {
	RegisterModule("connector", func(srv *ipc.Server, d *Deps) {
		connector.SetDataDir(d.DataDir)
		connector.RegisterIPC(srv)
	})
}
