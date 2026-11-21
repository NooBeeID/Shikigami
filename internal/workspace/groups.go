package workspace

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxGroupNameRunes = 40
	mainGroupName     = "Utama"
)

// GroupInfo adalah ringkasan grup tanpa layout.
type GroupInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
	Main  bool   `json:"main,omitempty"`
}

// GroupsState adalah hasil semua method grup dan payload event.
type GroupsState struct {
	WorkspaceID   string            `json:"workspaceId"`
	ActiveGroupID string            `json:"activeGroupId"`
	Groups        []GroupInfo       `json:"groups"`     // urut Order, non-nil
	Membership    map[string]string `json:"membership"` // non-nil
}

// normalizeGroups merapikan grup saat load: urut, order rapat, tepat satu Main,
// Membership dikosongkan. Tidak pernah membuat grup baru.
func normalizeGroups(w *Workspace) {
	if len(w.Groups) > 0 {
		sort.SliceStable(w.Groups, func(a, b int) bool { return w.Groups[a].Order < w.Groups[b].Order })
		for i := range w.Groups {
			w.Groups[i].Order = i
		}
		pick := -1
		for i := range w.Groups {
			if w.Groups[i].Main {
				pick = i
				break
			}
		}
		if pick < 0 {
			for i := range w.Groups {
				if strings.EqualFold(strings.TrimSpace(w.Groups[i].Name), mainGroupName) {
					pick = i
					break
				}
			}
		}
		if pick < 0 {
			pick = 0
		}
		for i := range w.Groups {
			w.Groups[i].Main = i == pick
		}
	}
	w.Membership = map[string]string{}
}

func mainGroupID(w Workspace) string {
	for _, g := range w.Groups {
		if g.Main {
			return g.ID
		}
	}
	if len(w.Groups) > 0 {
		return w.Groups[0].ID
	}
	return ""
}

func stateOf(w Workspace) GroupsState {
	gi := make([]GroupInfo, 0, len(w.Groups))
	for _, g := range w.Groups {
		gi = append(gi, GroupInfo{ID: g.ID, Name: g.Name, Order: g.Order, Main: g.Main})
	}
	m := make(map[string]string, len(w.Membership))
	for k, v := range w.Membership {
		m[k] = v
	}
	return GroupsState{WorkspaceID: w.ID, ActiveGroupID: w.ActiveGroupID, Groups: gi, Membership: m}
}

func cleanName(raw string) (string, error) {
	t := strings.TrimSpace(raw)
	if utf8.RuneCountInString(t) > maxGroupNameRunes {
		return "", fmt.Errorf("%w: nama grup maksimal %d karakter", ErrInvalid, maxGroupNameRunes)
	}
	return t, nil
}

func nameTaken(gs []Group, name, exceptID string) bool {
	for _, g := range gs {
		if g.ID != exceptID && strings.EqualFold(g.Name, name) {
			return true
		}
	}
	return false
}

func uniqueName(gs []Group, name, exceptID string) string {
	if !nameTaken(gs, name, exceptID) {
		return name
	}
	for n := 2; ; n++ {
		c := name + " " + strconv.Itoa(n)
		if !nameTaken(gs, c, exceptID) {
			return c
		}
	}
}

func defaultName(gs []Group) string {
	n := len(gs) + 1
	for {
		c := "Grup " + strconv.Itoa(n)
		if !nameTaken(gs, c, "") {
			return c
		}
		n++
	}
}

func pruneMembership(w *Workspace) {
	for k, v := range w.Membership {
		if findGroup(*w, v) < 0 {
			delete(w.Membership, k)
		}
	}
}

// mutateGroups mengkloning state, menjalankan fn pada salinan workspace target,
// dan hanya bila changed=true mempersist lalu menukar state memori.
func (s *Store) mutateGroups(wsID string, fn func(w *Workspace) (bool, error)) (GroupsState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(wsID)
	if i < 0 {
		return GroupsState{}, false, fmt.Errorf("%w: workspace %q", ErrNotFound, wsID)
	}
	next := cloneWorkspaces(s.workspaces)
	changed, err := fn(&next[i])
	if err != nil {
		return GroupsState{}, false, err
	}
	if !changed {
		return stateOf(s.workspaces[i]), false, nil
	}
	if err := s.persistLocked(next, s.ui); err != nil {
		return GroupsState{}, false, err
	}
	s.workspaces = next
	return stateOf(next[i]), true, nil
}

func (s *Store) CreateGroup(wsID, name string) (GroupsState, string, error) {
	var id string
	st, _, err := s.mutateGroups(wsID, func(w *Workspace) (bool, error) {
		t, err := cleanName(name)
		if err != nil {
			return false, err
		}
		var final string
		if t == "" {
			final = defaultName(w.Groups)
		} else {
			final = uniqueName(w.Groups, t, "")
		}
		g := Group{ID: newID(), Name: final, Order: len(w.Groups)}
		w.Groups = append(w.Groups, g)
		pruneMembership(w)
		id = g.ID
		return true, nil
	})
	if err != nil {
		return GroupsState{}, "", err
	}
	return st, id, nil
}

func (s *Store) RenameGroup(wsID, groupID, name string) (GroupsState, bool, error) {
	return s.mutateGroups(wsID, func(w *Workspace) (bool, error) {
		t, err := cleanName(name)
		if err != nil {
			return false, err
		}
		if t == "" {
			return false, fmt.Errorf("%w: nama grup kosong", ErrInvalid)
		}
		j := findGroup(*w, groupID)
		if j < 0 {
			return false, fmt.Errorf("%w: grup %q", ErrNotFound, groupID)
		}
		final := uniqueName(w.Groups, t, groupID)
		if final == w.Groups[j].Name {
			return false, nil
		}
		w.Groups[j].Name = final
		pruneMembership(w)
		return true, nil
	})
}

func (s *Store) DeleteGroup(wsID, groupID string) (GroupsState, error) {
	st, _, err := s.mutateGroups(wsID, func(w *Workspace) (bool, error) {
		j := findGroup(*w, groupID)
		if j < 0 {
			return false, fmt.Errorf("%w: grup %q", ErrNotFound, groupID)
		}
		if w.Groups[j].Main {
			return false, fmt.Errorf("%w: grup bawaan tidak bisa dihapus", ErrConflict)
		}
		mainID := mainGroupID(*w)
		for k, v := range w.Membership {
			if v == groupID {
				w.Membership[k] = mainID
			}
		}
		w.Groups = append(w.Groups[:j], w.Groups[j+1:]...)
		for i := range w.Groups {
			w.Groups[i].Order = i
		}
		if w.ActiveGroupID == groupID {
			w.ActiveGroupID = mainID
		}
		pruneMembership(w)
		return true, nil
	})
	return st, err
}

func (s *Store) ReorderGroup(wsID, groupID string, toIndex int) (GroupsState, bool, error) {
	return s.mutateGroups(wsID, func(w *Workspace) (bool, error) {
		from := findGroup(*w, groupID)
		if from < 0 {
			return false, fmt.Errorf("%w: grup %q", ErrNotFound, groupID)
		}
		if toIndex < 0 {
			toIndex = 0
		}
		if toIndex > len(w.Groups)-1 {
			toIndex = len(w.Groups) - 1
		}
		if toIndex == from {
			return false, nil
		}
		g := w.Groups[from]
		rest := make([]Group, 0, len(w.Groups)-1)
		rest = append(rest, w.Groups[:from]...)
		rest = append(rest, w.Groups[from+1:]...)
		out := make([]Group, 0, len(w.Groups))
		out = append(out, rest[:toIndex]...)
		out = append(out, g)
		out = append(out, rest[toIndex:]...)
		for i := range out {
			out[i].Order = i
		}
		w.Groups = out
		return true, nil
	})
}

func (s *Store) SetActiveGroup(wsID, groupID string) (GroupsState, bool, error) {
	return s.mutateGroups(wsID, func(w *Workspace) (bool, error) {
		if findGroup(*w, groupID) < 0 {
			return false, fmt.Errorf("%w: grup %q", ErrNotFound, groupID)
		}
		if w.ActiveGroupID == groupID {
			return false, nil
		}
		w.ActiveGroupID = groupID
		return true, nil
	})
}

func (s *Store) MoveSession(wsID, sessionID, toGroupID string) (GroupsState, bool, error) {
	return s.mutateGroups(wsID, func(w *Workspace) (bool, error) {
		if strings.TrimSpace(sessionID) == "" {
			return false, fmt.Errorf("%w: sessionId kosong", ErrInvalid)
		}
		if findGroup(*w, toGroupID) < 0 {
			return false, fmt.Errorf("%w: grup %q", ErrNotFound, toGroupID)
		}
		cur, ok := w.Membership[sessionID]
		if !ok || findGroup(*w, cur) < 0 {
			cur = mainGroupID(*w)
		}
		if cur == toGroupID {
			return false, nil
		}
		w.Membership[sessionID] = toGroupID
		pruneMembership(w)
		return true, nil
	})
}
