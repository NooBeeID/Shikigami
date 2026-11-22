package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

func init() {
	RegisterCommand(Command{
		Name:    "resource",
		Summary: "Snapshot CPU dan memori (method resource.snapshot) [--app-pid <pid>]",
		Run: func(ctx context.Context, c *ipc.Client, args []string) error {
			fs := flag.NewFlagSet("resource", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			appPid := fs.Int("app-pid", 0, "PID app untuk baris App")
			if err := fs.Parse(args); err != nil {
				return fmt.Errorf("%w\npemakaian: shikigamictl resource [--app-pid <pid>]", err)
			}
			if *appPid > 0 {
				return callAndPrint(c, "resource.snapshot", map[string]int{"appPid": *appPid})
			}
			return callAndPrint(c, "resource.snapshot", nil)
		},
	})
}
