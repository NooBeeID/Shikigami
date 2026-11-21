// Paket provider berisi adapter tipis untuk program yang berjalan di dalam sesi terminal
// (claude, codex, shell) beserta model kapabilitasnya.
package provider

import (
	"fmt"
	"io"
	"os/exec"
)

// Identitas provider bawaan.
const (
	// IDClaude adalah ID provider Claude Code.
	IDClaude = "claude"
	// IDCodex adalah ID provider Codex.
	IDCodex = "codex"
	// IDShell adalah ID provider login shell.
	IDShell = "shell"
)

// Capabilities menyatakan fitur integrasi yang didukung sebuah provider.
type Capabilities struct {
	// Hooks true bila provider mendukung hook.
	Hooks bool `json:"hooks"`
	// MCP true bila provider mendukung konfigurasi server MCP.
	MCP bool `json:"mcp"`
	// IdleEvent true bila provider dapat melaporkan event idle.
	IdleEvent bool `json:"idleEvent"`
}

// Provider adalah adapter untuk satu jenis program terminal.
type Provider interface {
	// ID mengembalikan pengenal unik provider.
	ID() string
	// Name mengembalikan nama tampilan provider.
	Name() string
	// Capabilities mengembalikan kapabilitas provider.
	Capabilities() Capabilities
	// Command membangun perintah, argumen, dan env tambahan khusus provider.
	Command(ctx *LaunchContext) (cmd string, args []string, env []string)
	// SendInput mengirim teks ke provider (bracketed paste + Enter).
	SendInput(w io.Writer, text string) error
	// Available memeriksa apakah binary provider tersedia.
	Available() (path string, err error)
}

// BracketedPaste mengembalikan "\x1b[200~" + text + "\x1b[201~" + "\r" (Enter sudah termasuk).
func BracketedPaste(text string) []byte {
	return []byte("\x1b[200~" + text + "\x1b[201~" + "\r")
}

// WriteInput menulis BracketedPaste(text) ke w dalam satu panggilan Write.
func WriteInput(w io.Writer, text string) error {
	_, err := w.Write(BracketedPaste(text))
	return err
}

// LookPath membungkus exec.LookPath dengan pesan error berbahasa Indonesia.
func LookPath(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("binary %q tidak ditemukan di PATH", name)
	}
	return p, nil
}

// BinaryError membangun error untuk binary provider yang tidak ditemukan.
func BinaryError(providerID, bin string) error {
	return fmt.Errorf("provider %q: binary %q tidak ditemukan di PATH; pasang atau tambahkan ke PATH", providerID, bin)
}
