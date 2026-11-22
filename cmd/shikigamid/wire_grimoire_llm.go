package main

import (
	"github.com/NooBeeID/shikigami/internal/grimoire"
	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/paths"
)

func init() {
	RegisterModule("grimoire-llm", func(srv *ipc.Server, d *Deps) {
		dataDir := d.DataDir
		if dataDir == "" {
			dd, err := paths.DataDir()
			if err != nil {
				if d.Logger != nil {
					d.Logger.Warn("modul grimoire-llm dilewati", "err", err)
				}
				return
			}
			dataDir = dd
		}
		d.OnReady(func() {
			deps := grimoire.LLMDeps{
				Store:       grimoire.NewStore(dataDir),
				ResolvePath: newGrimoireResolver(d.SocketPath),
				Publish: func(id string) {
					srv.Publish("grimoire.changed", map[string]any{"projectId": id})
				},
				Logger: d.Logger,
			}
			grimoire.RegisterLLM(srv, deps)
			grimoire.SetLLMRuntime(deps)
		})
	})
}
