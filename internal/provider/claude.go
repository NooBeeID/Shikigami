package provider

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/NooBeeID/shikigami/internal/paths"
)

var claudeCapabilities = Capabilities{Hooks: true, MCP: true, IdleEvent: true}

// claudeProvider menjalankan Claude Code.
type claudeProvider struct{}

func init() { Register(claudeProvider{}) }

// ID mengembalikan IDClaude.
func (claudeProvider) ID() string { return IDClaude }

// Name mengembalikan nama tampilan.
func (claudeProvider) Name() string { return "Claude Code" }

// Capabilities mengembalikan kapabilitas claude.
func (claudeProvider) Capabilities() Capabilities { return claudeCapabilities }

// SendInput menulis teks lewat WriteInput.
func (claudeProvider) SendInput(w io.Writer, text string) error { return WriteInput(w, text) }

// Available memeriksa keberadaan binary claude.
func (claudeProvider) Available() (string, error) {
	p, err := LookPath("claude")
	if err != nil {
		return "", BinaryError(IDClaude, "claude")
	}
	return p, nil
}

// writeMCPConfig menulis konfigurasi MCP ke <DataDir>/run/mcp-<16 hex>.json (mode 0600).
// Nama memakai hex acak karena ID sesi belum ada saat Command dipanggil.
// Berkas tidak dibersihkan oleh epic ini.
func writeMCPConfig(servers map[string]any) (string, error) {
	base, err := paths.DataDir()
	if err != nil {
		return "", fmt.Errorf("gagal menentukan data dir: %w", err)
	}
	dir := filepath.Join(base, "run")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("gagal membuat direktori %s: %w", dir, err)
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("gagal membuat nama acak: %w", err)
	}
	path := filepath.Join(dir, "mcp-"+hex.EncodeToString(b)+".json")
	data, err := json.Marshal(map[string]any{"mcpServers": servers})
	if err != nil {
		return "", fmt.Errorf("gagal meng-encode konfigurasi MCP: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("gagal menulis %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("gagal chmod %s: %w", path, err)
	}
	return path, nil
}

// Command menjalankan claude; menambah --mcp-config bila ada server MCP.
func (claudeProvider) Command(ctx *LaunchContext) (string, []string, []string) {
	args := append([]string(nil), ctx.Args...)
	if len(ctx.MCPServers) > 0 {
		path, err := writeMCPConfig(ctx.MCPServers)
		if err != nil {
			slog.Warn("provider claude: gagal menulis konfigurasi MCP; lanjut tanpa --mcp-config", "err", err)
		} else {
			args = append(args, "--mcp-config", path)
		}
	}
	return "claude", args, nil
}
