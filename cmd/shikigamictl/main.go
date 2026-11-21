package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/paths"
)

func init() {
	RegisterCommand(Command{
		Name:    "ping",
		Summary: "Cek daemon hidup (method ping)",
		Run: func(ctx context.Context, c *ipc.Client, args []string) error {
			return callAndPrint(c, "ping", nil)
		},
	})
	RegisterCommand(Command{
		Name:    "info",
		Summary: "Tampilkan info daemon (method daemon.info)",
		Run: func(ctx context.Context, c *ipc.Client, args []string) error {
			return callAndPrint(c, "daemon.info", nil)
		},
	})
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("shikigamictl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	socket := fs.String("socket", "", "path unix socket daemon (default: <DataDir>/shikigamid.sock)")
	fs.Usage = func() { printHelp(stderr) }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printHelp(stdout)
			return 0
		}
		return 1
	}
	rest := fs.Args()
	if len(rest) == 0 || rest[0] == "help" || rest[0] == "-h" || rest[0] == "--help" {
		printHelp(stdout)
		return 0
	}

	cmd, ok := lookupCommand(rest[0])
	if !ok {
		fmt.Fprintf(stderr, "command tidak dikenal: %q\nJalankan \"shikigamictl help\" untuk daftar command.\n", rest[0])
		return 1
	}

	sock := *socket
	if sock == "" {
		p, err := paths.SocketPath()
		if err != nil {
			fmt.Fprintf(stderr, "shikigamictl: %v\n", err)
			return 1
		}
		sock = p
	}

	c, err := ipc.Dial(sock)
	if err != nil {
		fmt.Fprintf(stderr, "daemon tidak berjalan di %s\n", sock)
		return 1
	}
	defer c.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cmd.Run(ctx, c, rest[1:]); err != nil {
		var ipcErr *ipc.Error
		if errors.As(err, &ipcErr) {
			fmt.Fprintf(stderr, "shikigamictl: galat %d: %s\n", ipcErr.Code, ipcErr.Message)
		} else {
			fmt.Fprintf(stderr, "shikigamictl: %v\n", err)
		}
		return 1
	}
	return 0
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "Penggunaan: shikigamictl [--socket <path>] <command> [argumen]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flag:")
	fmt.Fprintln(w, "  --socket <path>  path unix socket daemon (default: <DataDir>/shikigamid.sock)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Command:")
	cmds := sortedCommands()
	width := len("help")
	for _, c := range cmds {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}
	for _, c := range cmds {
		fmt.Fprintf(w, "  %-*s  %s\n", width, c.Name, c.Summary)
	}
	fmt.Fprintf(w, "  %-*s  %s\n", width, "help", "Tampilkan bantuan ini")
}
