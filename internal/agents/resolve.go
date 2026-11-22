package agents

import (
	"fmt"
	"strings"
)

// Resolve memetakan referensi ke agent dan kategorinya. Urutan: nama agent
// persis (setelah slug), id kategori, lalu nama kategori (case-insensitive).
// Nama agent didahulukan atas kategori bila bentrok (mis. agent "planner").
// Kategori dipetakan ke agent default-nya; tanpa default -> galat biasa
// (bukan ErrNotFound) agar pemanggil bisa membedakan.
func (s *Store) Resolve(ref string) (Agent, Category, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return Agent{}, Category{}, fmt.Errorf("%w: referensi kosong", ErrNotFound)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return Agent{}, Category{}, err
	}
	if n, nerr := normalizeAgentName(ref); nerr == nil {
		if i := st.findAgentByName(n); i >= 0 {
			a := st.Agents[i]
			if ci := st.findCategory(a.CategoryID); ci >= 0 {
				return a, st.Categories[ci], nil
			}
		}
	}
	ci := st.findCategory(ref)
	if ci < 0 {
		ci = st.findCategory(strings.ToLower(ref))
	}
	if ci < 0 {
		for i, c := range st.Categories {
			if strings.EqualFold(c.Name, ref) {
				ci = i
				break
			}
		}
	}
	if ci < 0 {
		return Agent{}, Category{}, fmt.Errorf("%w: %q", ErrNotFound, ref)
	}
	cat := st.Categories[ci]
	for _, i := range st.agentsIn(cat.ID) {
		if st.Agents[i].IsDefault {
			return st.Agents[i], cat, nil
		}
	}
	return Agent{}, Category{}, fmt.Errorf("kategori %s belum punya agent default", cat.Name)
}
