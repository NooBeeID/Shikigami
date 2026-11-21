package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"aiterm/internal/ipc"
)

const statusUsage = "penggunaan: aitermctl status list|get <sessionId>|watch|hook --session <id> --event <Nama>"

type statusRow struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
	Since     int64  `json:"since"`
}

type statusEventRow struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
	Prev      string `json:"prev"`
	Source    string `json:"source"`
	Reason    string `json:"reason"`
	At        int64  `json:"at"`
}

func init() {
	RegisterCommand(Command{
		Name:    "status",
		Summary: "Status sesi: list|get|watch|hook",
		Run:     runStatus,
	})
}

func runStatus(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(statusUsage)
	}
	switch args[0] {
	case "list":
		return statusList(c)
	case "get":
		if len(args) != 2 {
			return errors.New(statusUsage)
		}
		return callAndPrint(c, "status.get", map[string]any{"sessionId": args[1]})
	case "watch":
		return statusWatch(ctx, c)
	case "hook":
		return statusHook(c, args[1:])
	}
	return errors.New(statusUsage)
}

func statusList(c *ipc.Client) error {
	var rows []statusRow
	if err := c.Call("status.list", nil, &rows); err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("(tidak ada sesi)")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SESSION\tSTATUS\tSEJAK")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.SessionID, r.Status, time.UnixMilli(r.Since).Format("15:04:05"))
	}
	return tw.Flush()
}

func statusWatch(ctx context.Context, c *ipc.Client) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	unsub, err := c.Subscribe("session.status", func(_ string, payload json.RawMessage) {
		var ev statusEventRow
		if json.Unmarshal(payload, &ev) != nil {
			return
		}
		fmt.Printf("%s %s %s -> %s (%s/%s)\n", time.UnixMilli(ev.At).Format("15:04:05"),
			ev.SessionID, ev.Prev, ev.Status, ev.Source, ev.Reason)
	})
	if err != nil {
		return err
	}
	defer unsub()
	<-ctx.Done()
	return nil
}

// statusHook selalu mengembalikan nil; galat hanya ke stderr. main.go aitermctl
// keluar kode 1 bila Dial gagal sebelum command jalan; itu ditangani dengan
// pembungkus `|| true` pada command di berkas hooks (milik hook-receiver).
func statusHook(c *ipc.Client, args []string) error {
	fs := flag.NewFlagSet("status hook", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	session := fs.String("session", "", "ID sesi atau launch token")
	event := fs.String("event", "", "nama event hook")
	if err := fs.Parse(args); err != nil || *session == "" || *event == "" {
		fmt.Fprintln(os.Stderr, "status hook: --session dan --event wajib diisi")
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		fmt.Fprintf(os.Stderr, "status hook: gagal membaca stdin: %v\n", err)
	}
	payload := map[string]any{}
	if json.Unmarshal(data, &payload) != nil || payload == nil {
		payload = map[string]any{}
	}
	if err := c.Call("status.hook", map[string]any{"sessionId": *session, "event": *event, "payload": payload}, nil); err != nil {
		fmt.Fprintf(os.Stderr, "status hook: %v\n", err)
	}
	return nil
}
