package main

import (
	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/resource"
	"github.com/NooBeeID/shikigami/internal/session"
)

// lateSessions meneruskan ke session.Manager setelah modul siap.
type lateSessions struct{ m *session.Manager }

func (l *lateSessions) RunningPids() map[string]int {
	if l.m == nil {
		return nil
	}
	return l.m.RunningPids()
}

func init() {
	RegisterModule("resource", func(srv *ipc.Server, d *Deps) {
		src := &lateSessions{}
		resource.RegisterIPC(srv, resource.NewSampler(nil), src, resource.WebKitPIDs)
		d.OnReady(func() {
			v, ok := d.Lookup("session.manager")
			m, ok2 := v.(*session.Manager)
			if ok && ok2 {
				src.m = m
			} else if d.Logger != nil {
				d.Logger.Warn("resource: session.manager tidak tersedia")
			}
		})
	})
}
