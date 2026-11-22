package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTrustClaudeFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := filepath.Join(home, ".claude.json")
	os.WriteFile(cfg, []byte(`{"theme":"dark","projects":{"/a":{"allowedTools":[]}}}`), 0o600)
	cwd := t.TempDir()
	if err := trustClaudeFolder(cwd); err != nil {
		t.Fatal(err)
	}
	if err := trustClaudeFolder("/a"); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Theme    string                    `json:"theme"`
		Projects map[string]map[string]any `json:"projects"`
	}
	b, _ := os.ReadFile(cfg)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Theme != "dark" || got.Projects["/a"]["hasTrustDialogAccepted"] != true || got.Projects["/a"]["allowedTools"] == nil {
		t.Fatalf("hasil tak terduga: %s", b)
	}
	r, _ := filepath.EvalSymlinks(cwd)
	if got.Projects[r]["hasTrustDialogAccepted"] != true {
		t.Fatalf("cwd tidak trusted: %s", b)
	}
}
