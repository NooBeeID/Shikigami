package workspace

import (
	"fmt"
	"strings"
)

// cloneLark menyalin dalam LarkConfig; hasil tak pernah memuat slice nil.
func cloneLark(c *LarkConfig) LarkConfig {
	out := LarkConfig{Tickets: []TicketSource{}, Wiki: []WikiSource{}}
	if c == nil {
		return out
	}
	out.Tickets = append(out.Tickets, c.Tickets...)
	out.Wiki = append(out.Wiki, c.Wiki...)
	return out
}

// mutateLark: kunci tulis, salin workspace, jalankan fn pada salinan LarkConfig,
// persist, lalu ganti s.workspaces. Gagal persist tidak mengubah state.
func (s *Store) mutateLark(wsID string, fn func(lc *LarkConfig) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(wsID)
	if i < 0 {
		return fmt.Errorf("%w: workspace %q", ErrNotFound, wsID)
	}
	next := cloneWorkspaces(s.workspaces)
	lc := cloneLark(next[i].Config.Lark)
	if err := fn(&lc); err != nil {
		return err
	}
	next[i].Config.Lark = &lc
	if err := s.persistLocked(next, s.ui); err != nil {
		return err
	}
	s.workspaces = next
	return nil
}

// LarkSources mengembalikan salinan sumber Lark workspace (slice tak nil).
func (s *Store) LarkSources(wsID string) (LarkConfig, error) {
	if strings.TrimSpace(wsID) == "" {
		return LarkConfig{}, fmt.Errorf("%w: workspaceId kosong", ErrInvalid)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	i := s.indexLocked(wsID)
	if i < 0 {
		return LarkConfig{}, fmt.Errorf("%w: workspace %q", ErrNotFound, wsID)
	}
	return cloneLark(s.workspaces[i].Config.Lark), nil
}

// AddLarkTicketSource menambah sumber Base; ID dibuat di sini.
func (s *Store) AddLarkTicketSource(wsID string, src TicketSource) (TicketSource, error) {
	src.Name = strings.TrimSpace(src.Name)
	src.BaseToken = strings.TrimSpace(src.BaseToken)
	src.Host = strings.TrimSpace(src.Host)
	src.TableID = strings.TrimSpace(src.TableID)
	src.ViewID = strings.TrimSpace(src.ViewID)
	if src.Name == "" {
		return TicketSource{}, fmt.Errorf("%w: nama tidak boleh kosong", ErrInvalid)
	}
	if src.BaseToken == "" || src.Host == "" {
		return TicketSource{}, fmt.Errorf("%w: baseToken dan host wajib diisi", ErrInvalid)
	}
	err := s.mutateLark(wsID, func(lc *LarkConfig) error {
		for _, t := range lc.Tickets {
			if t.BaseToken == src.BaseToken {
				return fmt.Errorf("%w: sumber sudah ada", ErrConflict)
			}
		}
		src.ID = newID()
		lc.Tickets = append(lc.Tickets, src)
		return nil
	})
	if err != nil {
		return TicketSource{}, err
	}
	return src, nil
}

// AddLarkWikiSource menambah sumber wiki; ID dibuat di sini.
func (s *Store) AddLarkWikiSource(wsID string, src WikiSource) (WikiSource, error) {
	src.Name = strings.TrimSpace(src.Name)
	src.SpaceID = strings.TrimSpace(src.SpaceID)
	src.RootToken = strings.TrimSpace(src.RootToken)
	src.Host = strings.TrimSpace(src.Host)
	if src.Name == "" {
		return WikiSource{}, fmt.Errorf("%w: nama tidak boleh kosong", ErrInvalid)
	}
	if src.SpaceID == "" || src.Host == "" {
		return WikiSource{}, fmt.Errorf("%w: spaceId dan host wajib diisi", ErrInvalid)
	}
	if src.SpaceID == "my_library" {
		return WikiSource{}, fmt.Errorf("%w: wiki pribadi tidak disimpan", ErrInvalid)
	}
	err := s.mutateLark(wsID, func(lc *LarkConfig) error {
		for _, w := range lc.Wiki {
			if w.SpaceID == src.SpaceID && w.RootToken == src.RootToken {
				return fmt.Errorf("%w: sumber sudah ada", ErrConflict)
			}
		}
		src.ID = newID()
		lc.Wiki = append(lc.Wiki, src)
		return nil
	})
	if err != nil {
		return WikiSource{}, err
	}
	return src, nil
}

// RemoveLarkSource menghapus sumber (ticket atau wiki) berdasarkan ID.
func (s *Store) RemoveLarkSource(wsID, sourceID string) error {
	return s.mutateLark(wsID, func(lc *LarkConfig) error {
		for k, t := range lc.Tickets {
			if t.ID == sourceID {
				nt := make([]TicketSource, 0, len(lc.Tickets)-1)
				nt = append(nt, lc.Tickets[:k]...)
				lc.Tickets = append(nt, lc.Tickets[k+1:]...)
				return nil
			}
		}
		for k, w := range lc.Wiki {
			if w.ID == sourceID {
				nw := make([]WikiSource, 0, len(lc.Wiki)-1)
				nw = append(nw, lc.Wiki[:k]...)
				lc.Wiki = append(nw, lc.Wiki[k+1:]...)
				return nil
			}
		}
		return fmt.Errorf("%w: sumber %q", ErrNotFound, sourceID)
	})
}

// RenameLarkSource mengganti nama sumber.
func (s *Store) RenameLarkSource(wsID, sourceID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: nama tidak boleh kosong", ErrInvalid)
	}
	return s.mutateLark(wsID, func(lc *LarkConfig) error {
		for k := range lc.Tickets {
			if lc.Tickets[k].ID == sourceID {
				lc.Tickets[k].Name = name
				return nil
			}
		}
		for k := range lc.Wiki {
			if lc.Wiki[k].ID == sourceID {
				lc.Wiki[k].Name = name
				return nil
			}
		}
		return fmt.Errorf("%w: sumber %q", ErrNotFound, sourceID)
	})
}
