package provider

import (
	"io"
	"os"
)

// shellCapabilities: idle dideteksi lewat output-diam, jadi semua kapabilitas false.
var shellCapabilities = Capabilities{}

// shellProvider menjalankan login shell pengguna.
type shellProvider struct{}

func init() { Register(shellProvider{}) }

// shellPath mengembalikan $SHELL, fallback /bin/zsh.
func shellPath() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/zsh"
}

// ID mengembalikan IDShell.
func (shellProvider) ID() string { return IDShell }

// Name mengembalikan nama tampilan.
func (shellProvider) Name() string { return "Shell" }

// Capabilities mengembalikan kapabilitas shell.
func (shellProvider) Capabilities() Capabilities { return shellCapabilities }

// Command menjalankan login shell; MCP diabaikan.
func (shellProvider) Command(ctx *LaunchContext) (string, []string, []string) {
	args := append([]string{"-l"}, ctx.Args...)
	return shellPath(), args, nil
}

// SendInput menulis teks lewat WriteInput.
func (shellProvider) SendInput(w io.Writer, text string) error { return WriteInput(w, text) }

// Available memeriksa keberadaan shell.
func (shellProvider) Available() (string, error) {
	p, err := LookPath(shellPath())
	if err != nil {
		return "", BinaryError(IDShell, shellPath())
	}
	return p, nil
}
