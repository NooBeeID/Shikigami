package agents

import (
	"fmt"
	"sort"
	"strings"
)

// UpsertCategory membuat (id kosong) atau mengubah nama/peran kategori.
func (s *Store) UpsertCategory(id, name, role string) (Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return Category{}, err
	}
	n, err := normalizeCategoryName(name)
	if err != nil {
		return Category{}, err
	}
	role = strings.TrimSpace(role)
	cur := -1
	if id != "" {
		cur = st.findCategory(id)
		if cur < 0 {
			return Category{}, fmt.Errorf("%w: kategori %q tidak ditemukan", ErrNotFound, id)
		}
	}
	for i, c := range st.Categories {
		if i != cur && strings.EqualFold(c.Name, n) {
			return Category{}, fmt.Errorf("kategori %q sudah ada", n)
		}
	}
	var out Category
	if cur < 0 {
		out = Category{ID: uniqueCategoryID(slugify(n), st.Categories), Name: n, Role: role}
		st.Categories = append(st.Categories, out)
	} else {
		st.Categories[cur].Name = n
		st.Categories[cur].Role = role
		out = st.Categories[cur]
	}
	if err := s.save(st); err != nil {
		return Category{}, err
	}
	return out, nil
}

// DeleteMode menentukan nasib agent saat kategori dihapus.
type DeleteMode string

const (
	DeleteNone DeleteMode = ""
	DeleteMove DeleteMode = "move"
	DeleteAll  DeleteMode = "delete"
)

// DeleteCategory menghapus kategori. Semua validasi dilakukan dulu sebelum
// mengubah apa pun (gagal = tidak ada perubahan).
func (s *Store) DeleteCategory(id string, mode DeleteMode, moveTo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return err
	}
	ci := st.findCategory(id)
	if ci < 0 {
		return fmt.Errorf("%w: kategori %q tidak ditemukan", ErrNotFound, id)
	}
	cat := st.Categories[ci]
	if cat.Builtin {
		return fmt.Errorf("kategori bawaan %s tidak bisa dihapus", cat.Name)
	}
	affected := st.agentsIn(id)
	removeCat := func() { st.Categories = append(st.Categories[:ci], st.Categories[ci+1:]...) }
	if len(affected) == 0 {
		removeCat()
		return s.save(st)
	}
	if mode == DeleteNone {
		return fmt.Errorf("kategori %s masih punya %d agent; pilih pindahkan atau hapus agent-nya", cat.Name, len(affected))
	}
	if mode != DeleteMove && mode != DeleteAll {
		return fmt.Errorf("nilai agents %q tidak valid: gunakan %q atau %q", string(mode), DeleteMove, DeleteAll)
	}
	if s.guard != nil {
		for _, i := range affected {
			if err := s.guard(st.Agents[i].ID); err != nil {
				return &guardError{err: err}
			}
		}
	}
	if mode == DeleteAll {
		kept := st.Agents[:0:0]
		for _, a := range st.Agents {
			if a.CategoryID != id {
				kept = append(kept, a)
			}
		}
		st.Agents = kept
		removeCat()
		return s.save(st)
	}
	// mode move
	if moveTo == "" {
		return fmt.Errorf("kategori tujuan (moveTo) wajib diisi untuk memindahkan agent")
	}
	if moveTo == id {
		return fmt.Errorf("kategori tujuan tidak boleh sama dengan kategori yang dihapus (%q)", id)
	}
	if st.findCategory(moveTo) < 0 {
		return fmt.Errorf("%w: kategori tujuan %q tidak ditemukan", ErrNotFound, moveTo)
	}
	for _, i := range affected {
		if err := s.validateProvider(st.Agents[i].ProviderID, moveTo); err != nil {
			return fmt.Errorf("agent %s tidak bisa dipindahkan: %w", st.Agents[i].Name, err)
		}
	}
	destHasDefault := false
	for _, i := range st.agentsIn(moveTo) {
		if st.Agents[i].IsDefault {
			destHasDefault = true
		}
	}
	// pilih kandidat default dari yang dipindah: default asal, atau pertama menurut nama
	pick := -1
	if !destHasDefault {
		for _, i := range affected {
			if st.Agents[i].IsDefault {
				pick = i
			}
		}
		if pick < 0 {
			sorted := append([]int{}, affected...)
			sort.SliceStable(sorted, func(a, b int) bool { return st.Agents[sorted[a]].Name < st.Agents[sorted[b]].Name })
			pick = sorted[0]
		}
	}
	for _, i := range affected {
		st.Agents[i].CategoryID = moveTo
		st.Agents[i].IsDefault = i == pick
	}
	st.normalizeDefaults(moveTo)
	removeCat()
	return s.save(st)
}
