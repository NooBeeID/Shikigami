package main

import (
	"aiterm/internal/ipc"
	"aiterm/internal/session"
)

func init() {
	RegisterModule("session", func(srv *ipc.Server, _ *Deps) {
		m := session.NewManager(srv.Publish)
		session.RegisterIPC(srv, m)
	})
}
