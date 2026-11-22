package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const larkTicketsUsage = `penggunaan:
  shikigamictl lark tickets list --workspace <id> --source <id> --table <id> [--view <id>] [--offset N]
  shikigamictl lark tickets get  --workspace <id> --source <id> --record <id> [--table <id>] [--view <id>]`

func init() { registerLarkSub("tickets", runLarkTickets) }

type larkTicketsField struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsPrimary bool   `json:"isPrimary"`
}
type larkTicketsRecord struct {
	ID     string            `json:"id"`
	Title  string            `json:"title"`
	Values map[string]string `json:"values"`
}
type larkTicketsPage struct {
	Fields     []larkTicketsField  `json:"fields"`
	Records    []larkTicketsRecord `json:"records"`
	NextOffset *int                `json:"nextOffset"`
}
type larkTicketsDetailField struct {
	Name string `json:"name"`
	Text string `json:"text"`
}
type larkTicketsDetail struct {
	Fields              []larkTicketsDetailField `json:"fields"`
	URL                 string                   `json:"url"`
	RecordLinkAvailable bool                     `json:"recordLinkAvailable"`
}

func runLarkTickets(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 || (args[0] != "list" && args[0] != "get") {
		return errors.New(larkTicketsUsage)
	}
	action := args[0]
	fs := flag.NewFlagSet("lark tickets "+action, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ws := fs.String("workspace", "", "")
	src := fs.String("source", "", "")
	table := fs.String("table", "", "")
	view := fs.String("view", "", "")
	record := fs.String("record", "", "")
	offset := fs.Int("offset", 0, "")
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("%v\n%s", err, larkTicketsUsage)
	}
	need := func(name, v string) error {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("--%s wajib\n%s", name, larkTicketsUsage)
		}
		return nil
	}
	if err := need("workspace", *ws); err != nil {
		return err
	}
	if err := need("source", *src); err != nil {
		return err
	}
	*table = strings.TrimSpace(*table)
	*view = strings.TrimSpace(*view)

	if action == "list" {
		if err := need("table", *table); err != nil {
			return err
		}
		if *offset < 0 {
			return fmt.Errorf("--offset tidak boleh negatif\n%s", larkTicketsUsage)
		}
		params := map[string]any{"workspaceId": *ws, "sourceId": *src, "tableId": *table, "offset": *offset}
		if *view != "" {
			params["viewId"] = *view
		}
		var out larkTicketsPage
		if err := c.Call("lark.tickets.list", params, &out); err != nil {
			return err
		}
		w := os.Stdout
		fmt.Fprintln(w, "fields:")
		for _, f := range out.Fields {
			line := fmt.Sprintf("  %s\t%s\t%s", f.ID, f.Name, f.Type)
			if f.IsPrimary {
				line += "\t(primary)"
			}
			fmt.Fprintln(w, line)
		}
		fmt.Fprintln(w, "records:")
		for _, r := range out.Records {
			fmt.Fprintf(w, "  %s\t%s\n", r.ID, r.Title)
		}
		if out.NextOffset != nil {
			fmt.Fprintf(w, "nextOffset: %d\n", *out.NextOffset)
		} else {
			fmt.Fprintln(w, "nextOffset: -")
		}
		return nil
	}

	if err := need("record", *record); err != nil {
		return err
	}
	params := map[string]any{"workspaceId": *ws, "sourceId": *src, "recordId": *record}
	if *table != "" {
		params["tableId"] = *table
	}
	if *view != "" {
		params["viewId"] = *view
	}
	var out larkTicketsDetail
	if err := c.Call("lark.ticket.get", params, &out); err != nil {
		return err
	}
	for _, f := range out.Fields {
		fmt.Printf("%s: %s\n", f.Name, f.Text)
	}
	url := out.URL
	if url == "" {
		url = "-"
	}
	fmt.Printf("url: %s\nrecordLinkAvailable: %t\n", url, out.RecordLinkAvailable)
	return nil
}
