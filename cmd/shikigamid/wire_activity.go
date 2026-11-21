package main

import (
	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/status"
)

func init() {
	RegisterModule("activity", func(srv *ipc.Server, d *Deps) {
		alog := status.NewActivityLog(srv)
		d.Provide("activity.log", alog)
		status.RegisterActivityIPC(srv, alog)
		d.OnReady(func() {
			v, ok := d.Lookup("status.engine")
			eng, ok2 := v.(*status.Engine)
			if !ok || !ok2 {
				if d.Logger != nil {
					d.Logger.Warn("activity: status.engine tidak tersedia")
				}
				return
			}
			// Antarmuka lokal: compile sebelum dan sesudah Wiring menambah SetActivitySink.
			if s, ok := any(eng).(interface {
				SetActivitySink(func(string, string, map[string]any))
			}); ok {
				s.SetActivitySink(alog.Record)
			} else if d.Logger != nil {
				d.Logger.Warn("activity: engine belum mendukung SetActivitySink")
			}
			eng.Subscribe(func(ev status.StatusEvent) {
				if ev.Status == status.StatusDone && ev.Source == status.SourceExit {
					alog.MarkEnded(ev.SessionID)
				}
			})
		})
	})
}
