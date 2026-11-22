package gitdata

import "strings"

type spec struct {
	tool  string
	cmd   []string
	flags map[string]bool
}

func flagSet(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// deniedFlags selalu ditolak, diperiksa sebelum pencocokan tabel.
var deniedFlags = map[string]bool{
	"--show-token": true, "-t": true, "--web": true,
	"--log": true, "--log-failed": true, "--with-variables": true,
	"--editor": true, "--body": true, "--title": true,
	"--yes": true, "--force": true,
}

var allowSpecs = []spec{
	{"glab", []string{"auth", "status"}, flagSet("--hostname")},
	{"gh", []string{"auth", "status"}, flagSet("--hostname", "-h", "--active", "-a")},
	{"glab", []string{"mr", "list"}, flagSet("-R", "--repo", "-F", "--output", "-P", "--per-page", "-p", "--page", "--merged", "--closed", "--all", "--draft", "--source-branch", "--target-branch", "-A", "--author", "--label", "-l")},
	{"glab", []string{"mr", "view"}, flagSet("-R", "--repo", "-F", "--output", "-c", "--comments")},
	{"glab", []string{"ci", "list"}, flagSet("-R", "--repo", "-F", "--output", "-P", "--per-page", "-p", "--page", "--status", "-s", "--ref", "-r", "--orderBy", "-o", "--sort")},
	{"glab", []string{"pipeline", "list"}, flagSet("-R", "--repo", "-F", "--output", "-P", "--per-page", "-p", "--page", "--status", "-s", "--ref", "-r", "--orderBy", "-o", "--sort")},
	{"glab", []string{"ci", "get"}, flagSet("-R", "--repo", "-F", "--output", "-b", "--branch", "-p", "--pipeline-id", "-d", "--with-job-details")},
	{"gh", []string{"pr", "list"}, flagSet("-R", "--repo", "--json", "-L", "--limit", "-s", "--state", "-b", "--base", "-H", "--head", "-A", "--author", "--draft", "-d", "-S", "--search", "-l", "--label")},
	{"gh", []string{"pr", "view"}, flagSet("-R", "--repo", "--json", "-c", "--comments")},
	{"gh", []string{"run", "list"}, flagSet("-R", "--repo", "--json", "-L", "--limit", "-b", "--branch", "-s", "--status", "-w", "--workflow", "-e", "--event", "-u", "--user", "-c", "--commit")},
	{"gh", []string{"run", "view"}, flagSet("-R", "--repo", "--json", "-j", "--job", "--attempt")},
}

func notAllowed(msg string) *Error {
	return &Error{Code: CodeNotAllowed, Message: "Perintah tidak diizinkan: " + msg}
}

// check mengembalikan nil bila perintah lolos allowlist.
func check(tool string, args []string) *Error {
	if tool != "glab" && tool != "gh" && tool != "git" {
		return notAllowed("tool tidak dikenal")
	}
	if len(args) == 0 {
		return &Error{Code: CodeNotAllowed, Message: "Perintah kosong"}
	}
	if tool == "git" {
		return checkGit(args)
	}
	for _, a := range args {
		name := a
		if strings.HasPrefix(a, "--") {
			if i := strings.Index(a, "="); i >= 0 {
				name = a[:i]
			}
		}
		if deniedFlags[name] {
			return notAllowed("flag " + name + " ditolak")
		}
	}
	var found *spec
	if len(args) >= 2 {
		for i := range allowSpecs {
			s := &allowSpecs[i]
			if s.tool == tool && s.cmd[0] == args[0] && s.cmd[1] == args[1] {
				found = s
				break
			}
		}
	}
	if found == nil {
		return notAllowed(strings.Join(append([]string{tool}, firstN(args, 2)...), " "))
	}
	for _, a := range args[2:] {
		if a == "--" || a == "-" {
			return notAllowed("argumen " + a + " ditolak")
		}
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := a
		if strings.HasPrefix(a, "--") {
			if i := strings.Index(a, "="); i >= 0 {
				name = a[:i]
			}
		}
		if !found.flags[name] {
			return notAllowed("flag " + name + " tidak diizinkan untuk " + tool + " " + args[0] + " " + args[1])
		}
	}
	return nil
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func checkGit(args []string) *Error {
	rest := args
	if rest[0] == "-C" {
		if len(rest) < 2 || rest[1] == "" {
			return notAllowed("git -C membutuhkan path")
		}
		rest = rest[2:]
	}
	if len(rest) == 0 {
		return notAllowed("git tanpa subcommand")
	}
	if rest[0] != "remote" {
		if strings.HasPrefix(rest[0], "-") {
			return notAllowed("opsi git " + rest[0] + " ditolak")
		}
		return notAllowed("git " + rest[0])
	}
	switch len(rest) {
	case 1:
		return nil
	case 3:
		if rest[1] == "get-url" && rest[2] != "" && !strings.HasPrefix(rest[2], "-") {
			return nil
		}
	}
	return notAllowed("git remote dengan argumen tersebut")
}
