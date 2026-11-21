package provider

import "io"

// codexCapabilities: nilai default ini diverifikasi manual terhadap CLI codex terpasang.
// Jika terbukti mendukung hooks/MCP/idle event, cukup ubah var ini tanpa mengubah interface
// (sementara itu idle dideteksi lewat output-diam).
var codexCapabilities = Capabilities{Hooks: false, MCP: false, IdleEvent: false}

// codexProvider menjalankan CLI codex.
type codexProvider struct{}

func init() { Register(codexProvider{}) }

// ID mengembalikan IDCodex.
func (codexProvider) ID() string { return IDCodex }

// Name mengembalikan nama tampilan.
func (codexProvider) Name() string { return "Codex" }

// Capabilities mengembalikan kapabilitas codex.
func (codexProvider) Capabilities() Capabilities { return codexCapabilities }

// Command menjalankan codex dengan --dangerously-bypass-approvals-and-sandbox
// (selalu); MCP diabaikan.
func (codexProvider) Command(ctx *LaunchContext) (string, []string, []string) {
	return "codex", withFlag(append([]string(nil), ctx.Args...), codexBypassFlag), nil
}

// SendInput menulis teks lewat WriteInput.
func (codexProvider) SendInput(w io.Writer, text string) error { return WriteInput(w, text) }

// Available memeriksa keberadaan binary codex.
func (codexProvider) Available() (string, error) {
	p, err := LookPath("codex")
	if err != nil {
		return "", BinaryError(IDCodex, "codex")
	}
	return p, nil
}
