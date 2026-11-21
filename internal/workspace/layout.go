package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// maxLayoutBytes adalah batas logis ukuran dokumen layout.
const maxLayoutBytes = 1 << 20

// validateLayout memastikan layout kosong/null (-> nil) atau objek JSON valid <= 1 MiB.
func validateLayout(raw json.RawMessage) (json.RawMessage, error) {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || bytes.Equal(t, []byte("null")) {
		return nil, nil
	}
	if len(t) > maxLayoutBytes {
		return nil, fmt.Errorf("%w: layout terlalu besar", ErrInvalid)
	}
	if !json.Valid(t) {
		return nil, fmt.Errorf("%w: layout bukan JSON valid", ErrInvalid)
	}
	if t[0] != '{' {
		return nil, fmt.Errorf("%w: layout harus objek JSON atau null", ErrInvalid)
	}
	return append(json.RawMessage(nil), t...), nil
}

func findGroup(w Workspace, groupID string) int {
	for j := range w.Groups {
		if w.Groups[j].ID == groupID {
			return j
		}
	}
	return -1
}

// GetLayout mengembalikan layout (salinan) grup; groupID kosong = grup aktif.
func (s *Store) GetLayout(wsID, groupID string) (string, json.RawMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i := s.indexLocked(wsID)
	if i < 0 {
		return "", nil, fmt.Errorf("%w: workspace %q", ErrNotFound, wsID)
	}
	w := s.workspaces[i]
	if groupID == "" {
		groupID = w.ActiveGroupID
	}
	j := findGroup(w, groupID)
	if j < 0 {
		return "", nil, fmt.Errorf("%w: grup %q", ErrNotFound, groupID)
	}
	var out json.RawMessage
	if l := w.Groups[j].Layout; l != nil {
		out = append(json.RawMessage(nil), l...)
	}
	return groupID, out, nil
}

// SetLayout menyimpan layout grup; null/kosong menghapus.
func (s *Store) SetLayout(wsID, groupID string, layout json.RawMessage) (string, error) {
	v, err := validateLayout(layout)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(wsID)
	if i < 0 {
		return "", fmt.Errorf("%w: workspace %q", ErrNotFound, wsID)
	}
	if groupID == "" {
		groupID = s.workspaces[i].ActiveGroupID
	}
	j := findGroup(s.workspaces[i], groupID)
	if j < 0 {
		return "", fmt.Errorf("%w: grup %q", ErrNotFound, groupID)
	}
	next := append([]Workspace(nil), s.workspaces...)
	nw := next[i]
	nw.Groups = append([]Group(nil), nw.Groups...)
	nw.Groups[j].Layout = v
	next[i] = nw
	if err := s.persistLocked(next, s.ui); err != nil {
		return "", err
	}
	s.workspaces = next
	return groupID, nil
}

// GetUI mengembalikan keadaan UI saat ini.
func (s *Store) GetUI() UIState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ui
}

// SetUI menggabungkan patch ke UIState dan menyimpannya.
func (s *Store) SetUI(p UIPatch) (UIState, error) {
	if (p.LeftSidebar != nil && p.LeftSidebar.Width < 0) || (p.RightSidebar != nil && p.RightSidebar.Width < 0) {
		return UIState{}, fmt.Errorf("%w: lebar sidebar negatif", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	nu := s.ui
	if p.LeftSidebar != nil {
		nu.LeftSidebar = *p.LeftSidebar
		nu.LeftSidebar.ActiveTab = ""
	}
	if p.RightSidebar != nil {
		old := nu.RightSidebar.ActiveTab
		nu.RightSidebar = *p.RightSidebar
		if nu.RightSidebar.ActiveTab == "" {
			nu.RightSidebar.ActiveTab = old
			if old == "" {
				nu.RightSidebar.ActiveTab = "files"
			}
		}
	}
	if p.ActiveWorkspaceID != nil {
		if *p.ActiveWorkspaceID != "" && s.indexLocked(*p.ActiveWorkspaceID) < 0 {
			return UIState{}, fmt.Errorf("%w: workspace %q tidak dikenal", ErrInvalid, *p.ActiveWorkspaceID)
		}
		nu.ActiveWorkspaceID = *p.ActiveWorkspaceID
	}
	if err := s.persistLocked(s.workspaces, nu); err != nil {
		return UIState{}, err
	}
	s.ui = nu
	return nu, nil
}
