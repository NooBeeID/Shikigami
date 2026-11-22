package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var trustMu sync.Mutex

// trustClaudeFolder menandai cwd sebagai trusted di ~/.claude.json agar Claude Code
// tidak menampilkan dialog "trust this folder" (yang menahan sesi tanpa input user).
// Field lain di berkas dipertahankan apa adanya; tulis atomik lewat rename.
func trustClaudeFolder(cwd string) error {
	if cwd == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".claude.json")
	trustMu.Lock()
	defer trustMu.Unlock()

	root := map[string]json.RawMessage{}
	mode := os.FileMode(0o600)
	if b, err := os.ReadFile(path); err == nil {
		if st, serr := os.Stat(path); serr == nil {
			mode = st.Mode().Perm()
		}
		if err := json.Unmarshal(b, &root); err != nil {
			return fmt.Errorf("~/.claude.json tidak valid, tidak diubah: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	projects := map[string]map[string]json.RawMessage{}
	if raw, ok := root["projects"]; ok {
		if err := json.Unmarshal(raw, &projects); err != nil {
			return fmt.Errorf("field projects tidak valid: %w", err)
		}
	}
	keys := []string{cwd}
	if r, err := filepath.EvalSymlinks(cwd); err == nil && r != cwd {
		keys = append(keys, r)
	}
	changed := false
	for _, k := range keys {
		entry := projects[k]
		if entry == nil {
			entry = map[string]json.RawMessage{}
		}
		if string(entry["hasTrustDialogAccepted"]) == "true" {
			continue
		}
		entry["hasTrustDialogAccepted"] = json.RawMessage("true")
		projects[k] = entry
		changed = true
	}
	if !changed {
		return nil
	}
	raw, err := json.Marshal(projects)
	if err != nil {
		return err
	}
	root["projects"] = raw
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".claude.json.shikigami-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
