package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const workspaceUsage = `pemakaian: shikigamictl workspace <aksi>
  list
  create <nama>
  rename <id> <nama>
  delete <id>
  touch <id>
  add-project <workspaceId> <path> [nama]
  remove-project <workspaceId> <projectId>
  config get [workspaceId]
  config set [--workspace <id>] [--url <u>] [--key <k>]`

func init() {
	RegisterCommand(Command{
		Name:    "workspace",
		Summary: "kelola workspace dan project (list, create, rename, delete, touch, add-project, remove-project, config)",
		Run:     runWorkspace,
	})
}

func runWorkspace(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(workspaceUsage)
	}
	rest := args[1:]
	switch args[0] {
	case "list":
		return callAndPrint(c, "workspace.list", struct{}{})
	case "create":
		if len(rest) < 1 {
			return errors.New("pemakaian: shikigamictl workspace create <nama>")
		}
		return callAndPrint(c, "workspace.create", map[string]any{"name": strings.Join(rest, " ")})
	case "rename":
		if len(rest) != 2 {
			return errors.New("pemakaian: shikigamictl workspace rename <id> <nama>")
		}
		return callAndPrint(c, "workspace.rename", map[string]any{"id": rest[0], "name": rest[1]})
	case "delete":
		if len(rest) != 1 {
			return errors.New("pemakaian: shikigamictl workspace delete <id>")
		}
		return callAndPrint(c, "workspace.delete", map[string]any{"id": rest[0]})
	case "touch":
		if len(rest) != 1 {
			return errors.New("pemakaian: shikigamictl workspace touch <id>")
		}
		return callAndPrint(c, "workspace.touch", map[string]any{"id": rest[0]})
	case "add-project":
		if len(rest) < 2 || len(rest) > 3 {
			return errors.New("pemakaian: shikigamictl workspace add-project <workspaceId> <path> [nama]")
		}
		p := rest[1]
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		params := map[string]any{"workspaceId": rest[0], "path": p}
		if len(rest) == 3 {
			params["name"] = rest[2]
		}
		return callAndPrint(c, "workspace.addProject", params)
	case "remove-project":
		if len(rest) != 2 {
			return errors.New("pemakaian: shikigamictl workspace remove-project <workspaceId> <projectId>")
		}
		return callAndPrint(c, "workspace.removeProject", map[string]any{"workspaceId": rest[0], "projectId": rest[1]})
	case "config":
		return runWorkspaceConfig(c, rest)
	default:
		return fmt.Errorf("sub-aksi tidak dikenal: %q\n%s", args[0], workspaceUsage)
	}
}

func runWorkspaceConfig(c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(workspaceUsage)
	}
	switch args[0] {
	case "get":
		params := map[string]any{}
		if len(args) > 1 {
			params["workspaceId"] = args[1]
		}
		return callAndPrint(c, "config.get", params)
	case "set":
		fs := flag.NewFlagSet("workspace config set", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		ws := fs.String("workspace", "", "id workspace (kosong = global)")
		url := fs.String("url", "", "URL Grimoire")
		key := fs.String("key", "", "key Grimoire")
		if err := fs.Parse(args[1:]); err != nil {
			return fmt.Errorf("flag tidak valid: %v\n%s", err, workspaceUsage)
		}
		params := map[string]any{}
		changed := false
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "workspace":
				params["workspaceId"] = *ws
			case "url":
				params["grimoireUrl"] = *url
				changed = true
			case "key":
				params["grimoireKey"] = *key
				changed = true
			}
		})
		if !changed {
			return errors.New("config set butuh --url dan/atau --key\n" + workspaceUsage)
		}
		return callAndPrint(c, "config.set", params)
	default:
		return fmt.Errorf("sub-aksi config tidak dikenal: %q\n%s", args[0], workspaceUsage)
	}
}
