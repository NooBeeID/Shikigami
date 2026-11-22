package main

import (
	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/runs"
	"github.com/NooBeeID/shikigami/internal/status"
)

func init() {
	RegisterModule("runs", func(srv *ipc.Server, d *Deps) {
		reg := runs.NewRegistry()
		d.Provide("runs.registry", reg)
		d.OnReady(func() {
			v, ok := d.Lookup("status.engine")
			eng, ok2 := v.(*status.Engine)
			if !ok || !ok2 {
				if d.Logger != nil {
					d.Logger.Warn("runs: status.engine tidak tersedia")
				}
				return
			}
			// Satu Notifier (satu mutex) untuk listener hook, listener exit, dan
			// runs.detail agar runs.changed tidak tiba terbalik urutan.
			n := runs.NewNotifier(reg, func(sessionID string, list []runs.Run) {
				srv.Publish("runs.changed", map[string]any{"sessionId": sessionID, "runs": list})
			})
			runs.RegisterIPC(srv, reg, eng, n)
			eng.AddHookListener(n.Apply)
			// Listener exit (bukan event status): tetap jalan walau sesi sudah Done lewat Stop.
			eng.AddExitListener(n.Forget)
		})
	})
}
