package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const larkSourcesUsage = `penggunaan:
  shikigamictl lark sources get    --workspace <id>
  shikigamictl lark sources add    --workspace <id> --kind ticket|wiki --link <url> [--name <n>]
  shikigamictl lark sources remove --workspace <id> --source <sid>
  shikigamictl lark sources rename --workspace <id> --source <sid> --name <n>`

func init() { registerLarkSub("sources", runLarkSources) }

type larkParsedLink struct {
	Kind        string `json:"kind"`
	Host        string `json:"host"`
	BaseToken   string `json:"baseToken"`
	TableID     string `json:"tableId"`
	ViewID      string `json:"viewId"`
	SpaceID     string `json:"spaceId"`
	NodeToken   string `json:"nodeToken"`
	DefaultName string `json:"defaultName"`
}

func runLarkSources(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(larkSourcesUsage)
	}
	action := args[0]
	switch action {
	case "get", "add", "remove", "rename":
	default:
		return errors.New(larkSourcesUsage)
	}
	fs := flag.NewFlagSet("lark sources "+action, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ws := fs.String("workspace", "", "")
	sid := fs.String("source", "", "")
	name := fs.String("name", "", "")
	link := fs.String("link", "", "")
	kind := fs.String("kind", "", "")
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("%v\n%s", err, larkSourcesUsage)
	}
	if *ws == "" {
		return fmt.Errorf("--workspace wajib\n%s", larkSourcesUsage)
	}
	switch action {
	case "get":
		return callAndPrint(c, "lark.sources.get", map[string]any{"workspaceId": *ws})
	case "remove":
		if *sid == "" {
			return fmt.Errorf("--source wajib\n%s", larkSourcesUsage)
		}
		return callAndPrint(c, "lark.sources.remove", map[string]any{"workspaceId": *ws, "sourceId": *sid})
	case "rename":
		if *sid == "" {
			return fmt.Errorf("--source wajib\n%s", larkSourcesUsage)
		}
		n := strings.TrimSpace(*name)
		if n == "" {
			return fmt.Errorf("--name wajib\n%s", larkSourcesUsage)
		}
		return callAndPrint(c, "lark.sources.rename", map[string]any{"workspaceId": *ws, "sourceId": *sid, "name": n})
	}
	// add
	if *kind != "ticket" && *kind != "wiki" {
		return fmt.Errorf("--kind harus ticket atau wiki\n%s", larkSourcesUsage)
	}
	if strings.TrimSpace(*link) == "" {
		return fmt.Errorf("--link wajib\n%s", larkSourcesUsage)
	}
	var p larkParsedLink
	if err := c.Call("lark.link.parse", map[string]any{"link": *link}, &p); err != nil {
		return err
	}
	want := "base"
	if *kind == "wiki" {
		want = "wiki"
	}
	if p.Kind != want {
		return fmt.Errorf("link ini adalah %s, bukan %s; gunakan --kind %s", p.Kind, want, map[bool]string{true: "ticket", false: "wiki"}[p.Kind == "base"])
	}
	n := strings.TrimSpace(*name)
	if n == "" {
		n = p.DefaultName
	}
	if n == "" {
		return errors.New("nama kosong; berikan --name")
	}
	params := map[string]any{"workspaceId": *ws, "kind": *kind, "name": n, "host": p.Host}
	if *kind == "ticket" {
		params["baseToken"] = p.BaseToken
		if p.TableID != "" {
			params["tableId"] = p.TableID
		}
		if p.ViewID != "" {
			params["viewId"] = p.ViewID
		}
	} else {
		params["spaceId"] = p.SpaceID
		if p.NodeToken != "" {
			params["rootToken"] = p.NodeToken
		}
	}
	return callAndPrint(c, "lark.sources.add", params)
}
