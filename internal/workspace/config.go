package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	workspacesFile = "workspaces.json"
	configFile     = "config.json"
	// workspacesFileVersion untuk workspaces.json, configFileVersion untuk config.json.
	workspacesFileVersion  = 2
	configFileVersion      = 1
	workspacesBackupSuffix = ".v1.bak"
)

type workspacesDoc struct {
	Version    int         `json:"version"`
	Workspaces []Workspace `json:"workspaces"`
	UI         UIState     `json:"ui"`
}

type configDoc struct {
	Version int          `json:"version"`
	Global  GlobalConfig `json:"global"`
}

// validateURL menerima string kosong atau URL http/https dengan host.
func validateURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: url tidak valid: %v", ErrInvalid, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Host == "" {
		return fmt.Errorf("%w: url harus http/https dengan host", ErrInvalid)
	}
	return nil
}

func loadConfig(path string) (GlobalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GlobalConfig{}, nil
		}
		return GlobalConfig{}, fmt.Errorf("baca %s: %w", path, err)
	}
	var doc configDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return GlobalConfig{}, fmt.Errorf("config korup (%s): %w", path, err)
	}
	return doc.Global, nil
}

func loadWorkspaces(path string) ([]Workspace, UIState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Workspace{}, defaultUI(), nil
		}
		return nil, UIState{}, fmt.Errorf("baca %s: %w", path, err)
	}
	var doc workspacesDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, UIState{}, fmt.Errorf("workspaces korup (%s): %w", path, err)
	}
	if doc.Version > workspacesFileVersion {
		return nil, UIState{}, fmt.Errorf("workspaces.json versi %d tidak didukung (maksimum %d); file tidak diubah", doc.Version, workspacesFileVersion)
	}
	ws := doc.Workspaces
	if ws == nil {
		ws = []Workspace{}
	}
	for i := range ws {
		w := &ws[i]
		if w.Projects == nil {
			w.Projects = []Project{}
		}
		if len(w.Groups) == 0 {
			w.Groups = []Group{newDefaultGroup()}
		}
		for j := range w.Groups {
			t := bytes.TrimSpace(w.Groups[j].Layout)
			if len(t) == 0 || bytes.Equal(t, []byte("null")) {
				w.Groups[j].Layout = nil
			}
		}
		found := false
		for _, g := range w.Groups {
			if g.ID == w.ActiveGroupID {
				found = true
				break
			}
		}
		if !found {
			w.ActiveGroupID = w.Groups[0].ID
		}
		normalizeGroups(w)
	}

	ui := doc.UI
	if doc.Version < 2 {
		ui = defaultUI()
		if len(ws) > 0 {
			ui.ActiveWorkspaceID = ws[0].ID
		}
	} else {
		if ui.LeftSidebar.Width == 0 {
			ui.LeftSidebar = defaultLeftSidebar()
		}
		if ui.RightSidebar.Width == 0 {
			ui.RightSidebar = defaultRightSidebar()
		}
		if ui.RightSidebar.ActiveTab == "" {
			ui.RightSidebar.ActiveTab = "files"
		}
		ui.LeftSidebar.ActiveTab = ""
		known := false
		for _, w := range ws {
			if w.ID == ui.ActiveWorkspaceID {
				known = true
				break
			}
		}
		if !known {
			ui.ActiveWorkspaceID = ""
		}
	}

	if doc.Version < 2 {
		if err := backupV1(path, data); err != nil {
			return nil, UIState{}, err
		}
		if err := writeJSONAtomic(path, workspacesDoc{Version: workspacesFileVersion, Workspaces: ws, UI: ui}); err != nil {
			return nil, UIState{}, fmt.Errorf("migrasi workspaces ke v2: %w", err)
		}
	}
	return ws, ui, nil
}

// backupV1 menyalin bytes asli ke path+workspacesBackupSuffix bila belum ada.
func backupV1(path string, data []byte) error {
	bak := path + workspacesBackupSuffix
	f, err := os.OpenFile(bak, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return fmt.Errorf("cadangan %s: %w", bak, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(bak)
		return fmt.Errorf("cadangan %s: %w", bak, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(bak)
		return fmt.Errorf("cadangan %s: %w", bak, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(bak)
		return fmt.Errorf("cadangan %s: %w", bak, err)
	}
	return nil
}

// writeJSONAtomic menulis lewat file temp + Sync + Rename, izin 0600.
func writeJSONAtomic(path string, v any) (err error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if err = f.Chmod(0o600); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	_ = os.Chmod(path, 0o600)
	return nil
}

// mergeConfig: override workspace per field bila non-kosong.
func mergeConfig(global GlobalConfig, ws WorkspaceConfig) GlobalConfig {
	out := global
	if ws.GrimoireURL != "" {
		out.GrimoireURL = ws.GrimoireURL
	}
	if ws.GrimoireKey != "" {
		out.GrimoireKey = ws.GrimoireKey
	}
	return out
}

func (s *Store) GetConfig(wsID string) (GlobalConfig, WorkspaceConfig, GlobalConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if wsID == "" {
		return s.global, WorkspaceConfig{}, s.global, nil
	}
	i := s.indexLocked(wsID)
	if i < 0 {
		return GlobalConfig{}, WorkspaceConfig{}, GlobalConfig{}, fmt.Errorf("%w: workspace %q", ErrNotFound, wsID)
	}
	cfg := s.workspaces[i].Config
	return s.global, cfg, mergeConfig(s.global, cfg), nil
}

func (s *Store) EffectiveConfig(wsID string) (GlobalConfig, error) {
	_, _, eff, err := s.GetConfig(wsID)
	return eff, err
}

func (s *Store) SetGlobalConfig(c GlobalConfig) error {
	c.GrimoireURL = strings.TrimSpace(c.GrimoireURL)
	c.GrimoireKey = strings.TrimSpace(c.GrimoireKey)
	if err := validateURL(c.GrimoireURL); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeJSONAtomic(s.cfgPath, configDoc{Version: configFileVersion, Global: c}); err != nil {
		return fmt.Errorf("simpan config: %w", err)
	}
	s.global = c
	return nil
}

func (s *Store) SetWorkspaceConfig(wsID string, c WorkspaceConfig) (Workspace, error) {
	c.GrimoireURL = strings.TrimSpace(c.GrimoireURL)
	c.GrimoireKey = strings.TrimSpace(c.GrimoireKey)
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(wsID)
	if i < 0 {
		return Workspace{}, fmt.Errorf("%w: workspace %q", ErrNotFound, wsID)
	}
	if err := validateURL(c.GrimoireURL); err != nil {
		return Workspace{}, err
	}
	next := cloneWorkspaces(s.workspaces)
	next[i].Config = c
	if err := s.persistLocked(next, s.ui); err != nil {
		return Workspace{}, err
	}
	s.workspaces = next
	return cloneWorkspace(next[i]), nil
}
