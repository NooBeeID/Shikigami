package main

import (
	"aiterm/internal/ipc"
	"aiterm/internal/session"
	"aiterm/internal/status"
)

func init() {
	RegisterModule("status", func(srv *ipc.Server, d *Deps) {
		eng := status.NewEngine(srv)
		status.SetDefault(eng)
		d.Provide("status.engine", eng)
		status.RegisterIPC(srv, eng)
		status.InstallHooks()
		status.StartNotifier(srv, eng)
		d.OnReady(func() {
			v, ok := d.Lookup("session.manager")
			m, ok2 := v.(*session.Manager)
			if ok && ok2 {
				eng.SetSessionSource(m)
			} else if d.Logger != nil {
				d.Logger.Warn("status: session.manager tidak tersedia")
			}
		})
	})
}
