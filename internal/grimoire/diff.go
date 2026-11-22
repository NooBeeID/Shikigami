package grimoire

import "sort"

// ChangeKind adalah jenis perubahan satu modul terhadap state.
type ChangeKind int

const (
	ChangeUnchanged ChangeKind = iota
	ChangeAdded
	ChangeModified
	ChangeRemoved
)

// ModuleChange mencatat perubahan satu modul.
type ModuleChange struct {
	Name             string
	Kind             ChangeKind
	OldHash, NewHash string
}

// Diff adalah hasil perbandingan hash modul dengan state.
// Stale berisi modul yang hash-nya sama tetapi ditandai stale di state
// (Kind tetap ChangeUnchanged); Unchanged hanya berisi yang tidak stale.
type Diff struct {
	Added, Modified, Removed, Unchanged []ModuleChange
	Stale                               []ModuleChange
}

// ComputeDiff membandingkan hash modul hasil scan dengan state (murni, tanpa I/O).
// st nil atau kosong dianggap state kosong (semua Added).
func ComputeDiff(scanned []Module, hashes map[string]string, st *State) Diff {
	old := map[string]ModuleState{}
	if st != nil && st.Modules != nil {
		old = st.Modules
	}
	var d Diff
	seen := make(map[string]bool, len(scanned))
	for _, m := range scanned {
		name := Snake(m.Name)
		if seen[name] {
			continue
		}
		seen[name] = true
		newHash, ok := hashes[m.Name]
		if !ok {
			newHash = hashes[name]
		}
		prev, exists := old[name]
		switch {
		case !exists:
			d.Added = append(d.Added, ModuleChange{Name: name, Kind: ChangeAdded, NewHash: newHash})
		case prev.Hash != newHash:
			d.Modified = append(d.Modified, ModuleChange{Name: name, Kind: ChangeModified, OldHash: prev.Hash, NewHash: newHash})
		case prev.Stale:
			d.Stale = append(d.Stale, ModuleChange{Name: name, Kind: ChangeUnchanged, OldHash: prev.Hash, NewHash: newHash})
		default:
			d.Unchanged = append(d.Unchanged, ModuleChange{Name: name, Kind: ChangeUnchanged, OldHash: prev.Hash, NewHash: newHash})
		}
	}
	for name, prev := range old {
		if !seen[name] {
			d.Removed = append(d.Removed, ModuleChange{Name: name, Kind: ChangeRemoved, OldHash: prev.Hash})
		}
	}
	for _, s := range [][]ModuleChange{d.Added, d.Modified, d.Removed, d.Unchanged, d.Stale} {
		s := s
		sort.Slice(s, func(i, j int) bool { return s[i].Name < s[j].Name })
	}
	return d
}

// Pending berisi Added, Modified, lalu Stale (slice baru).
func (d Diff) Pending() []ModuleChange {
	out := make([]ModuleChange, 0, len(d.Added)+len(d.Modified)+len(d.Stale))
	out = append(out, d.Added...)
	out = append(out, d.Modified...)
	out = append(out, d.Stale...)
	return out
}

// Empty true bila tidak ada yang perlu dibangun atau dihapus.
func (d Diff) Empty() bool {
	return len(d.Added)+len(d.Modified)+len(d.Removed)+len(d.Stale) == 0
}
