package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const grimoireUsage = `penggunaan:
  shikigamictl grimoire build  [--project <id>]   bangun ulang grimoire project
  shikigamictl grimoire status [--project <id>]   tampilkan status grimoire project

Tanpa --project, dipakai satu-satunya project bila hanya ada satu; bila lebih dari
satu, daftar id project ditampilkan.`

func init() {
	RegisterCommand(Command{
		Name:    "grimoire",
		Summary: "Bangun dan lihat status grimoire project",
		Run:     runGrimoire,
	})
}

func grimoireError(err error) error {
	var ie *ipc.Error
	if !errors.As(err, &ie) {
		return err
	}
	switch ie.Code {
	case ipc.ErrNotFound:
		return fmt.Errorf("tidak ditemukan (%s)", ie.Message)
	case ipc.ErrInvalid:
		return fmt.Errorf("parameter tidak valid (%s)", ie.Message)
	}
	return err
}

func grimoireDefaultProject(c *ipc.Client) (string, error) {
	var list []struct {
		Projects []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"projects"`
	}
	if err := c.Call("workspace.list", map[string]any{}, &list); err != nil {
		return "", grimoireError(err)
	}
	type proj struct{ id, name, path string }
	var all []proj
	for _, w := range list {
		for _, p := range w.Projects {
			all = append(all, proj{p.ID, p.Name, p.Path})
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].id < all[j].id })
	switch len(all) {
	case 0:
		return "", errors.New("belum ada project; tambahkan project dulu atau gunakan --project <id>")
	case 1:
		return all[0].id, nil
	}
	lines := make([]string, 0, len(all))
	for _, p := range all {
		lines = append(lines, fmt.Sprintf("  %s  %s  %s", p.id, p.name, p.path))
	}
	return "", fmt.Errorf("ada %d project, pilih satu dengan --project <id>:\n%s", len(all), strings.Join(lines, "\n"))
}

func runGrimoire(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(grimoireUsage)
	}
	sub := args[0]
	if sub != "build" && sub != "status" {
		return errors.New(grimoireUsage)
	}
	fs := flag.NewFlagSet("grimoire "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "id project")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 0 {
		return errors.New(grimoireUsage)
	}
	id := *project
	if id == "" {
		var err error
		if id, err = grimoireDefaultProject(c); err != nil {
			return err
		}
	}
	params := map[string]any{"projectId": id}
	if sub == "status" {
		var out map[string]any
		if err := c.Call("grimoire.status", params, &out); err != nil {
			return grimoireError(err)
		}
		if err := printJSON(out); err != nil {
			return err
		}
		num := func(k string) int { f, _ := out[k].(float64); return int(f) }
		fmt.Fprintf(os.Stderr, "index: %d byte, ~%d token, %d halaman, laws: %d byte\n",
			num("indexBytes"), num("estTokens"), num("pages"), num("lawsBytes"))
		return nil
	}
	return grimoireError(callAndPrint(c, "grimoire."+sub, params))
}
