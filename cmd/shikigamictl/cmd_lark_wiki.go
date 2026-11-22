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

const larkWikiUsage = "penggunaan: shikigamictl lark wiki list --workspace <id> --source <id|personal> [--parent <token>] | get --workspace <id> --source <id|personal> --node <token>"

type larkWikiListResult struct {
	Nodes []struct {
		Token    string `json:"token"`
		Title    string `json:"title"`
		ObjType  string `json:"objType"`
		ObjToken string `json:"objToken"`
		HasChild bool   `json:"hasChild"`
	} `json:"nodes"`
	HasMore      bool   `json:"hasMore"`
	Hidden       bool   `json:"hidden"`
	HiddenReason string `json:"hiddenReason"`
}

type larkWikiGetResult struct {
	Title     string     `json:"title"`
	ObjType   string     `json:"objType"`
	URL       string     `json:"url"`
	Markdown  *string    `json:"markdown"`
	Cells     [][]string `json:"cells"`
	Truncated bool       `json:"truncated"`
}

func init() { registerLarkSub("wiki", larkWiki) }

func larkWikiCell(s string) string {
	return strings.NewReplacer("\t", " ", "\r\n", " ", "\n", " ", "\r", " ").Replace(s)
}

func larkWiki(_ context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 || (args[0] != "list" && args[0] != "get") {
		return errors.New(larkWikiUsage)
	}
	sub := args[0]
	fs := flag.NewFlagSet("lark wiki "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ws := fs.String("workspace", "", "")
	src := fs.String("source", "", "")
	var parent, node *string
	if sub == "list" {
		parent = fs.String("parent", "", "")
	} else {
		node = fs.String("node", "", "")
	}
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 0 {
		return errors.New(larkWikiUsage)
	}
	if *ws == "" {
		return errors.New("--workspace wajib diisi")
	}
	if *src == "" {
		return errors.New("--source wajib diisi (id sumber wiki atau personal)")
	}

	if sub == "list" {
		params := map[string]any{"workspaceId": *ws, "sourceId": *src}
		if *parent != "" {
			params["parentToken"] = *parent
		}
		var out larkWikiListResult
		if err := c.Call("lark.wiki.list", params, &out); err != nil {
			return err
		}
		if out.Hidden {
			fmt.Printf("tersembunyi: %s\n", out.HiddenReason)
			return nil
		}
		if len(out.Nodes) == 0 {
			fmt.Println("(kosong)")
		}
		for _, n := range out.Nodes {
			fmt.Printf("%s\t%s\t%t\t%s\n", n.Token, n.ObjType, n.HasChild, n.Title)
		}
		if out.HasMore {
			fmt.Println("hasMore: true (menampilkan 50 pertama)")
		}
		return nil
	}

	if *node == "" {
		return errors.New("--node wajib diisi")
	}
	params := map[string]any{"workspaceId": *ws, "sourceId": *src, "nodeToken": *node}
	var out larkWikiGetResult
	if err := c.Call("lark.wiki.get", params, &out); err != nil {
		return err
	}
	fmt.Printf("title: %s\n", out.Title)
	fmt.Printf("objType: %s\n", out.ObjType)
	if out.URL == "" {
		fmt.Println("url: (kosong, host belum diketahui)")
	} else {
		fmt.Printf("url: %s\n", out.URL)
	}
	if out.Markdown != nil {
		fmt.Println()
		fmt.Println(*out.Markdown)
	}
	if out.Cells != nil {
		fmt.Println()
		for _, row := range out.Cells {
			cols := make([]string, len(row))
			for i, v := range row {
				cols[i] = larkWikiCell(v)
			}
			fmt.Println(strings.Join(cols, "\t"))
		}
		if out.Truncated {
			fmt.Println("truncated: true (rentang awal 50 baris x 20 kolom)")
		}
	}
	return nil
}
