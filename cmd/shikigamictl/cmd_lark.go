package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

type larkSubFunc func(ctx context.Context, c *ipc.Client, args []string) error

var larkSubcommands = map[string]larkSubFunc{}

// registerLarkSub mendaftarkan subcommand `lark`; dipanggil dari init().
func registerLarkSub(name string, fn larkSubFunc) {
	if name == "" {
		panic("nama subcommand lark kosong")
	}
	if fn == nil {
		panic(fmt.Sprintf("subcommand lark %q tidak punya fungsi", name))
	}
	if _, ok := larkSubcommands[name]; ok {
		panic(fmt.Sprintf("subcommand lark %q sudah terdaftar", name))
	}
	larkSubcommands[name] = fn
}

func larkUsage() string {
	names := make([]string, 0, len(larkSubcommands))
	for n := range larkSubcommands {
		names = append(names, n)
	}
	sort.Strings(names)
	return "penggunaan: shikigamictl lark " + strings.Join(names, "|")
}

func runLark(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(larkUsage())
	}
	fn, ok := larkSubcommands[args[0]]
	if !ok {
		return errors.New(larkUsage())
	}
	return fn(ctx, c, args[1:])
}

func larkStatus(_ context.Context, c *ipc.Client, args []string) error {
	if len(args) != 0 {
		return errors.New(larkUsage())
	}
	return callAndPrint(c, "connector.status", map[string]any{"id": "lark"})
}

func init() {
	RegisterCommand(Command{Name: "lark", Summary: "Connector Lark: status", Run: runLark})
	registerLarkSub("status", larkStatus)
}
