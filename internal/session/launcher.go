package session

import "os"

// Launcher menentukan perintah yang dijalankan untuk sebuah sesi.
type Launcher interface {
	Command(s SessionSpec) (cmd string, args []string, env []string)
}

type defaultLauncher struct{}

// DefaultLauncher mengembalikan launcher login shell default.
func DefaultLauncher() Launcher { return defaultLauncher{} }

func (defaultLauncher) Command(s SessionSpec) (string, []string, []string) {
	cmd := os.Getenv("SHELL")
	if cmd == "" {
		cmd = "/bin/zsh"
	}
	base := os.Environ()
	env := make([]string, 0, len(base)+2+len(s.Env))
	env = append(env, base...)
	env = append(env, "TERM=xterm-256color", "COLORTERM=truecolor")
	env = append(env, s.Env...)
	return cmd, []string{"-l"}, env
}
