package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

type gitSubFunc func(ctx context.Context, c *ipc.Client, args []string) error

var gitSubcommands = map[string]gitSubFunc{}

func registerGitSub(name string, fn gitSubFunc) {
	if name == "" {
		panic("nama subcommand git kosong")
	}
	if fn == nil {
		panic(fmt.Sprintf("subcommand git %q tanpa fungsi", name))
	}
	if _, dup := gitSubcommands[name]; dup {
		panic(fmt.Sprintf("subcommand git %q sudah terdaftar", name))
	}
	gitSubcommands[name] = fn
}

func gitUsage() string {
	names := make([]string, 0, len(gitSubcommands))
	for n := range gitSubcommands {
		names = append(names, n)
	}
	sort.Strings(names)
	return "penggunaan: shikigamictl git " + strings.Join(names, "|")
}

func runGit(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(gitUsage())
	}
	fn, ok := gitSubcommands[args[0]]
	if !ok {
		return errors.New(gitUsage())
	}
	return fn(ctx, c, args[1:])
}

const gitReposUsage = "penggunaan: shikigamictl git repos --workspace <id>"

func gitRepos(_ context.Context, c *ipc.Client, args []string) error {
	fs := flag.NewFlagSet("git repos", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ws := fs.String("workspace", "", "")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v\n%s", err, gitReposUsage)
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("argumen tak dikenal\n%s", gitReposUsage)
	}
	if strings.TrimSpace(*ws) == "" {
		return fmt.Errorf("--workspace wajib\n%s", gitReposUsage)
	}
	return callAndPrint(c, "git.repos.list", map[string]any{"workspaceId": strings.TrimSpace(*ws)})
}

func init() {
	RegisterCommand(Command{Name: "git", Summary: "Connector Git: repo, MR, pipeline", Run: runGit})
	registerGitSub("repos", gitRepos)
}
