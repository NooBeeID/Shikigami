package main

import (
	"aiterm/internal/fsx"
	"aiterm/internal/ipc"
)

func init() {
	RegisterModule("fs", func(srv *ipc.Server, d *Deps) {
		// Deps tidak punya hook shutdown; watcher dibuat malas dan ikut proses.
		fsx.Register(srv)
	})
}
