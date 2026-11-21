// Package loginenv mengambil PATH login-shell pengguna agar daemon yang
// dibuka dari Finder tetap menemukan binary seperti claude dan codex.
package loginenv

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	markerStart  = "__SHIKIGAMI_PATH__"
	markerEnd    = "__END__"
	timeout      = 3 * time.Second
	defaultShell = "/bin/zsh"
)

var (
	once     sync.Once
	resolved string
)

// Merge menggabungkan string PATH dan lokasi umum tanpa duplikat; argumen dulu, lokasi umum di belakang.
func Merge(parts ...string) string {
	seen := map[string]bool{}
	var out []string
	add := func(e string) {
		if e == "" || seen[e] {
			return
		}
		seen[e] = true
		out = append(out, e)
	}
	for _, p := range parts {
		for _, e := range strings.Split(p, ":") {
			add(e)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	add("/opt/homebrew/bin")
	add("/usr/local/bin")
	if home != "" {
		m, _ := filepath.Glob(home + "/.nvm/versions/node/*/bin")
		sort.Sort(sort.Reverse(sort.StringSlice(m)))
		for _, e := range m {
			add(e)
		}
		add(home + "/.local/bin")
	}
	for _, e := range []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin"} {
		add(e)
	}
	return strings.Join(out, ":")
}

// fromLoginShell menjalankan login shell interaktif untuk membaca $PATH.
func fromLoginShell() (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = defaultShell
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "-l", "-i", "-c", `printf "__SHIKIGAMI_PATH__%s__END__" "$PATH"`)
	cmd.Stdin = nil
	cmd.Stderr = nil
	cmd.WaitDelay = 500 * time.Millisecond
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", errors.New("timeout menjalankan login shell")
		}
		return "", errors.New("gagal menjalankan login shell: " + err.Error())
	}
	s := string(out)
	i := strings.Index(s, markerStart)
	if i < 0 {
		return "", errors.New("keluaran tanpa penanda")
	}
	s = s[i+len(markerStart):]
	j := strings.Index(s, markerEnd)
	if j < 0 {
		return "", errors.New("keluaran tanpa penanda")
	}
	p := strings.TrimSpace(s[:j])
	if p == "" {
		return "", errors.New("PATH kosong")
	}
	return p, nil
}

// Resolve menjalankan resolusi PATH (idempoten) dan mengembalikan hasil final. logger boleh nil.
func Resolve(logger *slog.Logger) string {
	once.Do(func() {
		p, err := fromLoginShell()
		if err == nil {
			resolved = Merge(p)
			if logger != nil {
				logger.Info("loginenv: PATH login-shell dipakai", "entries", len(strings.Split(resolved, ":")))
			}
			return
		}
		resolved = Merge(os.Getenv("PATH"))
		if logger != nil {
			logger.Warn("loginenv: PATH login-shell gagal, memakai fallback",
				"reason", err.Error(), "entries", len(strings.Split(resolved, ":")))
		}
	})
	return resolved
}

// Path mengembalikan PATH login-shell yang di-cache; memicu Resolve(nil) bila belum.
func Path() string { return Resolve(nil) }
