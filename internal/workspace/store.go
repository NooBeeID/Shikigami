package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store menyimpan workspace dan konfigurasi global, dipersist ke DataDir.
type Store struct {
	mu         sync.RWMutex
	dir        string
	wsPath     string
	cfgPath    string
	workspaces []Workspace // urut pembuatan; Projects selalu non-nil
	global     GlobalConfig
	ui         UIState
}

func NewStore(dataDir string) (*Store, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("%w: dataDir kosong", ErrInvalid)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("buat dataDir: %w", err)
	}
	s := &Store{
		dir:     dataDir,
		wsPath:  filepath.Join(dataDir, workspacesFile),
		cfgPath: filepath.Join(dataDir, configFile),
	}
	var err error
	if s.workspaces, s.ui, err = loadWorkspaces(s.wsPath); err != nil {
		return nil, err
	}
	if s.global, err = loadConfig(s.cfgPath); err != nil {
		return nil, err
	}
	return s, nil
}

func cloneWorkspace(w Workspace) Workspace {
	c := w
	c.Projects = make([]Project, len(w.Projects))
	copy(c.Projects, w.Projects)
	c.Groups = make([]Group, len(w.Groups))
	for i, g := range w.Groups {
		if g.Layout != nil {
			g.Layout = append(json.RawMessage(nil), g.Layout...)
		}
		c.Groups[i] = g
	}
	return c
}

func cloneWorkspaces(ws []Workspace) []Workspace {
	out := make([]Workspace, len(ws))
	for i := range ws {
		out[i] = cloneWorkspace(ws[i])
	}
	return out
}

func (s *Store) indexLocked(id string) int {
	for i := range s.workspaces {
		if s.workspaces[i].ID == id {
			return i
		}
	}
	return -1
}

func (s *Store) persistLocked(ws []Workspace, ui UIState) error {
	if err := writeJSONAtomic(s.wsPath, workspacesDoc{Version: workspacesFileVersion, Workspaces: ws, UI: ui}); err != nil {
		return fmt.Errorf("simpan workspaces: %w", err)
	}
	return nil
}

func (s *Store) List() []Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneWorkspaces(s.workspaces)
}

func (s *Store) Get(id string) (Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i := s.indexLocked(id)
	if i < 0 {
		return Workspace{}, fmt.Errorf("%w: workspace %q", ErrNotFound, id)
	}
	return cloneWorkspace(s.workspaces[i]), nil
}

func (s *Store) Create(name string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, fmt.Errorf("%w: nama workspace kosong", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g := newDefaultGroup()
	w := Workspace{ID: newID(), Name: name, Projects: []Project{}, Groups: []Group{g}, ActiveGroupID: g.ID}
	next := append(cloneWorkspaces(s.workspaces), w)
	if err := s.persistLocked(next, s.ui); err != nil {
		return Workspace{}, err
	}
	s.workspaces = next
	return cloneWorkspace(w), nil
}

func (s *Store) Rename(id, name string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, fmt.Errorf("%w: nama workspace kosong", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(id)
	if i < 0 {
		return Workspace{}, fmt.Errorf("%w: workspace %q", ErrNotFound, id)
	}
	next := cloneWorkspaces(s.workspaces)
	next[i].Name = name
	if err := s.persistLocked(next, s.ui); err != nil {
		return Workspace{}, err
	}
	s.workspaces = next
	return cloneWorkspace(next[i]), nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(id)
	if i < 0 {
		return fmt.Errorf("%w: workspace %q", ErrNotFound, id)
	}
	next := cloneWorkspaces(s.workspaces)
	next = append(next[:i], next[i+1:]...)
	nextUI := s.ui
	if nextUI.ActiveWorkspaceID == id {
		nextUI.ActiveWorkspaceID = ""
	}
	if err := s.persistLocked(next, nextUI); err != nil {
		return err
	}
	s.workspaces = next
	s.ui = nextUI
	return nil
}

func (s *Store) AddProject(wsID, path, name string) (Workspace, Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(wsID)
	if i < 0 {
		return Workspace{}, Project{}, fmt.Errorf("%w: workspace %q", ErrNotFound, wsID)
	}
	norm, err := normalizeProjectPath(path)
	if err != nil {
		return Workspace{}, Project{}, err
	}
	for _, p := range s.workspaces[i].Projects {
		if p.Path == norm {
			return Workspace{}, Project{}, fmt.Errorf("%w: path sudah ada di workspace: %s", ErrConflict, norm)
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(norm)
	}
	p := Project{ID: newID(), Path: norm, Name: name}
	next := cloneWorkspaces(s.workspaces)
	next[i].Projects = append(next[i].Projects, p)
	if err := s.persistLocked(next, s.ui); err != nil {
		return Workspace{}, Project{}, err
	}
	s.workspaces = next
	return cloneWorkspace(next[i]), p, nil
}

func (s *Store) RemoveProject(wsID, projectID string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(wsID)
	if i < 0 {
		return Workspace{}, fmt.Errorf("%w: workspace %q", ErrNotFound, wsID)
	}
	next := cloneWorkspaces(s.workspaces)
	j := -1
	for k, p := range next[i].Projects {
		if p.ID == projectID {
			j = k
			break
		}
	}
	if j < 0 {
		return Workspace{}, fmt.Errorf("%w: project %q", ErrNotFound, projectID)
	}
	next[i].Projects = append(next[i].Projects[:j], next[i].Projects[j+1:]...)
	if err := s.persistLocked(next, s.ui); err != nil {
		return Workspace{}, err
	}
	s.workspaces = next
	return cloneWorkspace(next[i]), nil
}

func normalizeProjectPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("%w: path kosong", ErrInvalid)
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%w: path harus absolut: %s", ErrInvalid, p)
	}
	clean := filepath.Clean(p)
	fi, err := os.Stat(clean)
	if err != nil {
		return "", fmt.Errorf("%w: path tidak dapat diakses: %v", ErrInvalid, err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("%w: bukan direktori: %s", ErrInvalid, clean)
	}
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = filepath.Clean(resolved)
	}
	return clean, nil
}
