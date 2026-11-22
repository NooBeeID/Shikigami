package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const connectorUsage = "penggunaan: shikigamictl connector list|status <id>|guide <id>"

type connectorStatusInfo struct {
	State   string `json:"state"`
	User    string `json:"user"`
	Message string `json:"message"`
}

type connectorRow struct {
	ID     string              `json:"id"`
	Name   string              `json:"name"`
	Status connectorStatusInfo `json:"status"`
}

func init() {
	RegisterCommand(Command{
		Name:    "connector",
		Summary: "Connector layanan luar: list|status|guide",
		Run:     runConnector,
	})
}

func runConnector(_ context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(connectorUsage)
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New(connectorUsage)
		}
		return connectorList(c)
	case "status":
		if len(args) != 2 || args[1] == "" {
			return errors.New(connectorUsage)
		}
		return callAndPrint(c, "connector.status", map[string]any{"id": args[1]})
	case "guide":
		if len(args) != 2 || args[1] == "" {
			return errors.New(connectorUsage)
		}
		return callAndPrint(c, "connector.guide", map[string]any{"id": args[1]})
	default:
		return errors.New(connectorUsage)
	}
}

func connectorList(c *ipc.Client) error {
	var rows []connectorRow
	if err := c.Call("connector.list", nil, &rows); err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("(tidak ada connector)")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAMA\tSTATE\tUSER\tPESAN")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Name, r.Status.State, r.Status.User, r.Status.Message)
	}
	return tw.Flush()
}
