package main

import (
	"os"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/loginenv"
	"github.com/NooBeeID/shikigami/internal/provider"
)

func init() {
	// Resolve sinkron dan dibatasi 3 detik, sehingga start daemon tertunda paling lama 3 detik.
	// Modul terurut alfabetis sebelum provider dan session, jadi PATH siap sebelum sesi pertama.
	RegisterModule("loginenv", func(srv *ipc.Server, d *Deps) {
		p := loginenv.Resolve(d.Logger)
		_ = os.Setenv("PATH", p)
		provider.SetPathSource(loginenv.Path)
		d.Provide("loginenv.path", loginenv.Path)
	})
}
