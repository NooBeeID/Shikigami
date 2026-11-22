package agents

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

func (st *fileState) findAgent(id string) int {
	for i := range st.Agents {
		if st.Agents[i].ID == id {
			return i
		}
	}
	return -1
}

func (st *fileState) findAgentByName(name string) int {
	for i := range st.Agents {
		if st.Agents[i].Name == name {
			return i
		}
	}
	return -1
}

func (st *fileState) findCategory(id string) int {
	for i := range st.Categories {
		if st.Categories[i].ID == id {
			return i
		}
	}
	return -1
}

func (st *fileState) agentsIn(categoryID string) []int {
	var out []int
	for i := range st.Agents {
		if st.Agents[i].CategoryID == categoryID {
			out = append(out, i)
		}
	}
	return out
}

func sortedAgents(st fileState) []Agent {
	out := append([]Agent{}, st.Agents...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func copyState(st fileState) fileState {
	return fileState{
		Version:    st.Version,
		Categories: append([]Category{}, st.Categories...),
		Agents:     append([]Agent{}, st.Agents...),
	}
}

// normalizeDefaults menjaga invarian default satu kategori: bila lebih dari
// satu default, yang pertama di slice dipertahankan; bila nol default dan
// agent tepat satu, agent itu menjadi default.
func (st *fileState) normalizeDefaults(categoryID string) {
	idx := st.agentsIn(categoryID)
	seen := false
	for _, i := range idx {
		if st.Agents[i].IsDefault {
			if seen {
				st.Agents[i].IsDefault = false
			}
			seen = true
		}
	}
	if !seen && len(idx) == 1 {
		st.Agents[idx[0]].IsDefault = true
	}
}

// Snapshot mengembalikan salinan kategori (urut dibuat) dan agent (urut nama).
func (s *Store) Snapshot() ([]Category, []Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return nil, nil, err
	}
	return append([]Category{}, st.Categories...), sortedAgents(st), nil
}

// List seperti Snapshot, tetapi galat dicatat dan hasilnya irisan kosong.
func (s *Store) List() ([]Category, []Agent) {
	c, a, err := s.Snapshot()
	if err != nil {
		log.Printf("agents: gagal membaca daftar: %v", err)
		return []Category{}, []Agent{}
	}
	return c, a
}

// AgentInput adalah masukan UpsertAgent.
type AgentInput struct {
	ID, Name, CategoryID, ProviderID, Model string
	IsDefault                               bool
}

// UpsertAgent membuat atau memperbarui agent.
//
// Aturan default: agent pertama di kategori tujuan otomatis default walau
// IsDefault=false; IsDefault=true mencabut default lain di kategori tujuan;
// IsDefault=false pada agent yang sedang default di kategori yang SAMA tidak
// mencabutnya (default hanya berpindah lewat penanda default lain atau
// SetDefault); agent yang pindah kategori hanya default di tujuan bila
// IsDefault=true atau tujuan belum punya agent lain.
func (s *Store) UpsertAgent(in AgentInput) (Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return Agent{}, err
	}
	name, err := normalizeAgentName(in.Name)
	if err != nil {
		return Agent{}, err
	}
	if st.findCategory(in.CategoryID) < 0 {
		return Agent{}, fmt.Errorf("kategori %q tidak ditemukan", in.CategoryID)
	}
	if err := s.validateProvider(in.ProviderID, in.CategoryID); err != nil {
		return Agent{}, err
	}
	model := strings.TrimSpace(in.Model)
	provID := strings.TrimSpace(in.ProviderID)

	cur := -1
	if in.ID != "" {
		cur = st.findAgent(in.ID)
		if cur < 0 {
			return Agent{}, fmt.Errorf("%w: agent %q tidak ditemukan", ErrNotFound, in.ID)
		}
	}
	if o := st.findAgentByName(name); o >= 0 && o != cur {
		return Agent{}, fmt.Errorf("nama agent %q sudah dipakai", name)
	}

	var a Agent
	oldCat := ""
	if cur >= 0 {
		a = st.Agents[cur]
		oldCat = a.CategoryID
	} else {
		a.ID = newAgentID()
	}
	sameCat := cur >= 0 && oldCat == in.CategoryID
	a.Name, a.CategoryID, a.ProviderID, a.Model = name, in.CategoryID, provID, model

	others := 0
	for _, i := range st.agentsIn(in.CategoryID) {
		if i != cur {
			others++
		}
	}
	switch {
	case in.IsDefault:
		a.IsDefault = true
	case sameCat:
		// pertahankan status default yang ada
	default:
		a.IsDefault = others == 0
	}
	if a.IsDefault {
		for _, i := range st.agentsIn(in.CategoryID) {
			if i != cur {
				st.Agents[i].IsDefault = false
			}
		}
	}
	if cur >= 0 {
		st.Agents[cur] = a
	} else {
		st.Agents = append(st.Agents, a)
	}
	if oldCat != "" && oldCat != in.CategoryID {
		st.normalizeDefaults(oldCat)
	}
	st.normalizeDefaults(in.CategoryID)
	if i := st.findAgent(a.ID); i >= 0 {
		a = st.Agents[i]
	}
	if err := s.save(st); err != nil {
		return Agent{}, err
	}
	return a, nil
}

// DeleteAgent menghapus agent. Guard dipanggil SEBELUM menghapus.
//
// Bila agent default dihapus dan masih ada agent lain, default TIDAK
// dialihkan otomatis, kecuali tersisa tepat satu agent: agent itu menjadi
// default (efek normalizeDefaults).
func (s *Store) DeleteAgent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return err
	}
	i := st.findAgent(id)
	if i < 0 {
		return fmt.Errorf("%w: agent %q tidak ditemukan", ErrNotFound, id)
	}
	if s.guard != nil {
		if err := s.guard(id); err != nil {
			return &guardError{err: err}
		}
	}
	cat := st.Agents[i].CategoryID
	st.Agents = append(st.Agents[:i], st.Agents[i+1:]...)
	st.normalizeDefaults(cat)
	return s.save(st)
}

// SetDefault menjadikan agent default di kategorinya.
func (s *Store) SetDefault(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return err
	}
	i := st.findAgent(id)
	if i < 0 {
		return fmt.Errorf("%w: agent %q tidak ditemukan", ErrNotFound, id)
	}
	for _, j := range st.agentsIn(st.Agents[i].CategoryID) {
		st.Agents[j].IsDefault = j == i
	}
	return s.save(st)
}

// SetDeleteGuard memasang guard penghapusan agent (nil = tanpa guard).
// Guard dipanggil dengan mu terkunci: guard TIDAK boleh memanggil kembali
// method Store (deadlock).
func (s *Store) SetDeleteGuard(fn func(agentID string) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guard = fn
}
