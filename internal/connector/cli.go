package connector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/NooBeeID/shikigami/internal/loginenv"
)

// ErrCLIMissing menandakan CLI belum terpasang.
var ErrCLIMissing = errors.New("CLI belum terpasang")

const (
	cliTimeout = 8 * time.Second
	stderrMax  = 300
)

// IsCLIMissing melaporkan apakah err berarti CLI belum terpasang.
func IsCLIMissing(err error) bool { return errors.Is(err, ErrCLIMissing) }

// lookPath mencari biner pada PATH login-shell.
func lookPath(name string) (string, bool) {
	if strings.ContainsRune(name, filepath.Separator) {
		return name, true
	}
	for _, dir := range filepath.SplitList(loginenv.Path()) {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, name)
		info, err := os.Stat(p)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		return p, true
	}
	return "", false
}

// RunCLI menjalankan CLI dengan timeout dan PATH login-shell. Stdout
// dikembalikan apa adanya.
func RunCLI(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()

	bin, ok := lookPath(name)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrCLIMissing, name)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(os.Environ(), "PATH="+loginenv.Path())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), nil
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%w: %s", ErrCLIMissing, name)
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("perintah %s melewati batas waktu", name)
	}
	msg := truncate(strings.TrimSpace(stderr.String()), stderrMax)
	if msg == "" {
		msg = err.Error()
	}
	return "", fmt.Errorf("perintah %s gagal: %s", name, msg)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
