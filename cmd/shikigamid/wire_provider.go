package main

import (
	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/provider"
	"github.com/NooBeeID/shikigami/internal/session"
)

// Impor internal/provider menjalankan init() claude/codex/shell sehingga
// registri terisi. Modul dipasang alfabetis ("provider" sebelum "session"),
// jadi launcher WAJIB dipasang lewat OnReady, bukan langsung.
// session.CreateGuard dan d.Provide("session.manager", m) disediakan agen
// Wiring (wire_session.go dan session/ipc.go), bukan fitur ini.
func init() {
	RegisterModule("provider", func(srv *ipc.Server, d *Deps) {
		provider.RegisterIPC(srv)
		session.CreateGuard = provider.Validate
		d.OnReady(func() {
			v, ok := d.Lookup("session.manager")
			if !ok {
				warnProvider(d, "provider: layanan session.manager tidak tersedia; launcher provider tidak dipasang")
				return
			}
			m, ok := v.(*session.Manager)
			if !ok || m == nil {
				warnProvider(d, "provider: session.manager bertipe tidak terduga; launcher provider tidak dipasang")
				return
			}
			m.SetLauncher(provider.NewLauncher())
		})
	})
}

// warnProvider mencatat peringatan; aman bila Logger nil.
func warnProvider(d *Deps, msg string) {
	if d.Logger != nil {
		d.Logger.Warn(msg)
	}
}
