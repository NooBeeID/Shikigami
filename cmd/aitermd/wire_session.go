package main

import (
	"aiterm/internal/ipc"
	"aiterm/internal/session"
)

func init() {
	RegisterModule("session", func(srv *ipc.Server, d *Deps) {
		m := session.NewManager(srv.Publish)
		d.Provide("session.manager", m)
		session.RegisterIPC(srv, m)
	})
}
