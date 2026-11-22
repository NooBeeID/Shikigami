package status

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/paths"
	"github.com/NooBeeID/shikigami/internal/provider"
)

var installOnce sync.Once

// hookEvents adalah event hook Claude yang dipasang (8): lima event status lama
// ditambah PreToolUse, SubagentStart, dan SubagentStop untuk registry runs.
// ClassifyEvent hanya mengenal lima event lama; tiga yang baru tidak mengubah status.
var hookEvents = []string{
	HookUserPromptSubmit, HookNotification, HookStop, HookSessionStart, HookPostToolUse,
	HookPreToolUse, HookSubagentStart, HookSubagentStop,
}

// preToolUseMatcher membatasi PreToolUse ke tool Workflow dan Agent (matcher Claude
// adalah regex atas nama tool), sehingga proses shikigamictl tidak dijalankan pada
// setiap pemanggilan tool.
const preToolUseMatcher = "Workflow|Agent"

// newToken membuat launch token acak (16 hex).
func newToken() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// shellQuote membungkus s dengan kutip tunggal; tiap ' menjadi '\”.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shikigamictlPath mencari biner shikigamictl: sebelah executable, lalu PATH, lalu nama polos.
func shikigamictlPath() string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "shikigamictl")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if p, err := exec.LookPath("shikigamictl"); err == nil {
		return p
	}
	return "shikigamictl"
}

// hookCommand menyusun command hook. Akhiran "|| true" wajib: shikigamictl keluar
// kode 1 bila daemon mati, dan agent tidak boleh gagal karenanya.
func hookCommand(ctl, token, event string) string {
	return shellQuote(ctl) + " status hook --session " + token + " --event " + event + " || true"
}

func hooksFilePath(dataDir, token string) string {
	return filepath.Join(dataDir, "run", "status-hooks-"+token+".json")
}

// writeHooksFile menulis berkas hooks per sesi untuk Claude dan mengembalikan path-nya.
// Semua event di hookEvents dipasang tanpa matcher, kecuali PreToolUse yang memakai
// preToolUseMatcher.
//
// Verifikasi skema (2026-11-22): `claude --help` pada mesin ini mengonfirmasi
// flag `--settings <file-or-json>` ("Settings file or JSON string to load more
// settings from"), jadi path berkas JSON diterima. Skema `{"hooks": {"<Event>":
// [{"hooks":[{"type":"command","command":"..."}]}]}}` dengan `matcher` opsional,
// event UserPromptSubmit/Notification/Stop/SessionStart/PostToolUse, payload JSON via stdin,
// penggabungan dengan settings user/project, dan exit 0 tidak mengganggu agent
// diasumsikan dari dokumentasi Claude Code yang diketahui; TIDAK diverifikasi
// ulang lewat jaringan di sini. Akhiran `|| true` menjamin exit 0.
func writeHooksFile(dataDir, token, ctl string) (string, error) {
	hooks := map[string]any{}
	for _, ev := range hookEvents {
		entry := map[string]any{"hooks": []any{
			map[string]any{"type": "command", "command": hookCommand(ctl, token, ev)},
		}}
		if ev == HookPreToolUse {
			entry["matcher"] = preToolUseMatcher
		}
		hooks[ev] = []any{entry}
	}
	data, err := json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	if err != nil {
		return "", err
	}
	runDir := filepath.Join(dataDir, "run")
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return "", err
	}
	_ = os.Chmod(runDir, 0o700)
	path := hooksFilePath(dataDir, token)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return path, nil
}

// validToken mencegah path traversal pada nama berkas.
func validToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// CleanupHooks menghapus berkas hooks milik token. Aman dipanggil berulang dan konkuren.
func CleanupHooks(token string) {
	if !validToken(token) {
		return
	}
	dir, err := paths.DataDir()
	if err != nil {
		return
	}
	if err := os.Remove(hooksFilePath(dir, token)); err != nil && !os.IsNotExist(err) {
		slog.Debug("status: gagal menghapus berkas hooks", "token", token, "err", err)
	}
}

// removeStaleHooks menghapus berkas hooks sisa daemon sebelumnya (sekali, tanpa polling).
func removeStaleHooks() {
	dir, err := paths.DataDir()
	if err != nil {
		return
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "run", "status-hooks-*.json"))
	for _, m := range matches {
		if err := os.Remove(m); err != nil && !os.IsNotExist(err) {
			slog.Debug("status: gagal menghapus berkas hooks sisa", "path", m, "err", err)
		}
	}
}

// InstallHooks membersihkan sisa berkas lalu mendaftarkan launch contribution.
// Hanya berlaku sekali walau dipanggil berulang.
func InstallHooks() {
	installOnce.Do(func() {
		removeStaleHooks()
		provider.RegisterLaunchContribution(contribute)
	})
}

// contribute mengisi env dan argumen peluncuran sesi.
//
// Nilai SHIKIGAMI_SESSION_ID adalah launch token (overview T1), bukan ID sesi asli,
// karena session.Manager.Create memanggil Launcher sebelum membuat ID.
// Keterbatasan: Launcher.Command bisa dipanggil tanpa sesi benar-benar terbentuk
// (mis. gagal start), sehingga launch tertunda dan berkas bisa yatim; dibersihkan
// oleh removeStaleHooks saat daemon berikutnya start.
func contribute(ctx *provider.LaunchContext) {
	if ctx == nil {
		return
	}
	if ctx.Env == nil {
		ctx.Env = map[string]string{}
	}
	dataDir, err := paths.DataDir()
	if err != nil {
		slog.Warn("status: DataDir tidak tersedia, hooks status dilewati", "err", err)
		return
	}
	sock, serr := paths.SocketPath()
	token, err := newToken()
	if err != nil {
		slog.Warn("status: gagal membuat launch token", "err", err)
		return
	}
	ctx.Env["SHIKIGAMI_SESSION_ID"] = token
	if serr == nil {
		ctx.Env["SHIKIGAMI_SOCK"] = sock
	} else {
		slog.Warn("status: SocketPath tidak tersedia, SHIKIGAMI_SOCK dilewati", "err", serr)
	}

	providerID := ctx.Session.ProviderID
	if ctx.Provider != nil {
		providerID = ctx.Provider.ID()
	}
	if providerID == provider.IDClaude && ctx.Provider != nil && ctx.Provider.Capabilities().Hooks {
		path, err := writeHooksFile(dataDir, token, shikigamictlPath())
		if err != nil {
			slog.Warn("status: gagal menulis berkas hooks, lanjut tanpa --settings", "err", err)
		} else {
			ctx.Args = append(ctx.Args, "--settings", path)
		}
	}

	RegisterLaunch(LaunchInfo{
		Token:       token,
		WorkspaceID: ctx.Session.WorkspaceID,
		ProjectID:   ctx.Session.ProjectID,
		Cwd:         ctx.Session.Cwd,
		ProviderID:  providerID,
		At:          time.Now(),
	})
}
