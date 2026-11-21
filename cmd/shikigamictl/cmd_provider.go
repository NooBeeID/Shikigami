package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

// Catatan: "session new --provider" sudah disediakan epic 02 di cmd_session.go,
// sehingga "provider new <id>" hanya pintasan ke session.create.

const providerUsage = "penggunaan: shikigamictl provider list|new ..."

type providerCaps struct {
	Hooks     bool `json:"hooks"`
	MCP       bool `json:"mcp"`
	IdleEvent bool `json:"idleEvent"`
}

type providerInfo struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Capabilities providerCaps `json:"capabilities"`
	Available    bool         `json:"available"`
	Path         string       `json:"path"`
	Error        string       `json:"error"`
}

func init() {
	RegisterCommand(Command{
		Name:    "provider",
		Summary: "Daftar dan uji provider: list|new",
		Run:     runProvider,
	})
}

func runProvider(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(providerUsage)
	}
	switch args[0] {
	case "list":
		return providerList(c, args[1:])
	case "new":
		return providerNew(c, args[1:])
	}
	return errors.New(providerUsage)
}

func providerYesNo(b bool) string {
	if b {
		return "ya"
	}
	return "tidak"
}

func providerList(c *ipc.Client, args []string) error {
	fs := flag.NewFlagSet("provider list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "cetak JSON mentah")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("penggunaan: shikigamictl provider list [--json]")
	}
	var raw json.RawMessage
	if err := c.Call("provider.list", map[string]any{}, &raw); err != nil {
		return err
	}
	if *asJSON {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		return printJSON(v)
	}
	var list []providerInfo
	if err := json.Unmarshal(raw, &list); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tHOOKS\tMCP\tIDLE\tAVAILABLE\tPATH")
	for _, p := range list {
		col := p.Path
		if !p.Available {
			col = strings.TrimSpace(p.Error)
			if col == "" {
				col = "-"
			} else if r := []rune(col); len(r) > 60 {
				col = string(r[:57]) + "..."
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", p.ID, p.Name,
			providerYesNo(p.Capabilities.Hooks), providerYesNo(p.Capabilities.MCP),
			providerYesNo(p.Capabilities.IdleEvent), providerYesNo(p.Available), col)
	}
	return tw.Flush()
}

func providerNew(c *ipc.Client, args []string) error {
	const usage = "penggunaan: shikigamictl provider new <id> [--cwd DIR] [--workspace ID] [--project ID]"
	fs := flag.NewFlagSet("provider new", flag.ContinueOnError)
	cwd := fs.String("cwd", "", "direktori kerja")
	ws := fs.String("workspace", "", "ID workspace")
	proj := fs.String("project", "", "ID project")
	var id string
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		id, rest = args[0], args[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if id == "" {
		if fs.NArg() != 1 {
			return errors.New(usage)
		}
		id = fs.Arg(0)
	} else if fs.NArg() != 0 {
		return errors.New(usage)
	}
	params := map[string]any{"providerId": id}
	if *cwd != "" {
		params["cwd"] = *cwd
	}
	if *ws != "" {
		params["workspaceId"] = *ws
	}
	if *proj != "" {
		params["projectId"] = *proj
	}
	var raw json.RawMessage
	if err := c.Call("session.create", params, &raw); err != nil {
		return err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	fmt.Println(out.ID)
	return nil
}
