package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"aiterm/internal/ipc"
)

const mentionUsage = `pemakaian: aitermctl mention <aksi>
  send [--workspace <id>] [--from <sessionId>] <to> <teks...>
  queue [sessionId]
  cancel <id>
  confirm <id>
  names <workspaceId>
  mode [auto|confirm_agent|user_only]
  info`

func init() {
	RegisterCommand(Command{
		Name:    "mention",
		Summary: "mention antar sesi (send, queue, cancel, confirm, names, mode, info)",
		Run:     runMention,
	})
}

func runMention(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(mentionUsage)
	}
	rest := args[1:]
	switch args[0] {
	case "send":
		fs := flag.NewFlagSet("mention send", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		ws := fs.String("workspace", "", "id workspace")
		from := fs.String("from", "", "id sesi pengirim")
		if err := fs.Parse(rest); err != nil {
			return fmt.Errorf("%v\npemakaian: aitermctl mention send [--workspace <id>] [--from <sessionId>] <to> <teks...>", err)
		}
		r := fs.Args()
		if len(r) < 2 {
			return errors.New("pemakaian: aitermctl mention send [--workspace <id>] [--from <sessionId>] <to> <teks...>")
		}
		params := map[string]any{
			"workspaceId": *ws,
			"to":          r[0],
			"text":        strings.Join(r[1:], " "),
		}
		if *from != "" {
			params["from"] = *from
		}
		return callAndPrint(c, "mention.send", params)
	case "queue":
		if len(rest) > 1 {
			return errors.New("pemakaian: aitermctl mention queue [sessionId]")
		}
		params := map[string]any{}
		if len(rest) == 1 {
			params["sessionId"] = rest[0]
		}
		return callAndPrint(c, "mention.queue.list", params)
	case "cancel":
		if len(rest) != 1 {
			return errors.New("pemakaian: aitermctl mention cancel <id>")
		}
		return callAndPrint(c, "mention.queue.cancel", map[string]any{"id": rest[0]})
	case "confirm":
		if len(rest) != 1 {
			return errors.New("pemakaian: aitermctl mention confirm <id>")
		}
		return callAndPrint(c, "mention.confirm", map[string]any{"id": rest[0]})
	case "names":
		if len(rest) != 1 {
			return errors.New("pemakaian: aitermctl mention names <workspaceId>")
		}
		return callAndPrint(c, "mention.names", map[string]any{"workspaceId": rest[0]})
	case "mode":
		if len(rest) > 1 {
			return errors.New("pemakaian: aitermctl mention mode [auto|confirm_agent|user_only]")
		}
		params := map[string]any{}
		if len(rest) == 1 {
			params["mode"] = rest[0]
		}
		return callAndPrint(c, "mention.config", params)
	case "info":
		return callAndPrint(c, "mention.info", struct{}{})
	}
	return fmt.Errorf("sub-aksi tidak dikenal: %q\n%s", args[0], mentionUsage)
}
