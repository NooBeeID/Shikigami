package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

// Command adalah satu subcommand shikigamictl.
type Command struct {
	Name    string
	Summary string
	Run     func(ctx context.Context, c *ipc.Client, args []string) error
}

var commands = map[string]Command{}

// RegisterCommand mendaftarkan subcommand; dipanggil dari init().
func RegisterCommand(cmd Command) {
	if cmd.Name == "" {
		panic("nama command kosong")
	}
	if cmd.Run == nil {
		panic(fmt.Sprintf("command %q tidak punya fungsi Run", cmd.Name))
	}
	if _, ok := commands[cmd.Name]; ok {
		panic(fmt.Sprintf("command %q sudah terdaftar", cmd.Name))
	}
	commands[cmd.Name] = cmd
}

func sortedCommands() []Command {
	out := make([]Command, 0, len(commands))
	for _, c := range commands {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func lookupCommand(name string) (Command, bool) {
	c, ok := commands[name]
	return c, ok
}

// printJSON mencetak v sebagai JSON terformat ke stdout.
// Boleh dipakai cmd_<modul>.go milik epic lain.
func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(b, '\n'))
	return err
}

// callAndPrint memanggil method IPC lalu mencetak hasilnya sebagai JSON terformat.
func callAndPrint(c *ipc.Client, method string, params any) error {
	var out json.RawMessage
	if err := c.Call(method, params, &out); err != nil {
		return err
	}
	var v any
	if err := json.Unmarshal(out, &v); err != nil {
		return err
	}
	return printJSON(v)
}
