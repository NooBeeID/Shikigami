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
			publish := func(sessionID string) {
				srv.Publish("runs.changed", map[string]any{
					"sessionId": sessionID,
					"runs":      reg.List(map[string]bool{sessionID: true}),
				})
			}
			runs.RegisterIPC(srv, reg, eng, publish)
			eng.AddHookListener(func(sessionID, event string, payload map[string]any) {
				if reg.Apply(sessionID, event, payload) {
					publish(sessionID)
				}
			})
			// Listener exit (bukan event status): tetap jalan walau sesi sudah Done lewat Stop.
			eng.AddExitListener(func(sessionID string) {
				if reg.Forget(sessionID) {
					publish(sessionID)
				}
			})
		})
	})
}
