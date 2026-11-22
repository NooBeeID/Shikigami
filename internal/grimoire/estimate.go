package grimoire

import (
	"os"
	"path/filepath"

	"github.com/NooBeeID/shikigami/internal/paths"
)

// Estimate adalah hasil dry-run antrean build LLM.
type Estimate struct {
	Modules   int `json:"modules"`
	EstTokens int `json:"estTokens"`
	OverLimit int `json:"overLimit,omitempty"`
}

// EstTokensFromBytes memperkirakan jumlah token dari jumlah byte ((n+3)/4).
// Ini satu-satunya heuristik token di epic LLM.
func EstTokensFromBytes(n int) int {
	if n < 0 {
		n = 0
	}
	return (n + 3) / 4
}

// EstimateBuild menghitung antrean modul dan estimasi token build LLM.
// Baca-saja: tanpa claude dan tanpa menulis apa pun.
func EstimateBuild(projectID, projectPath string) (Estimate, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return Estimate{}, err
	}
	store := NewStore(dir)
	if err := store.ValidateProject(projectID); err != nil {
		return Estimate{}, err
	}
	root, err := incrementalRoot(projectPath)
	if err != nil {
		return Estimate{}, err
	}
	mods, hashes, st, err := incrementalScan(store, projectID, root)
	if err != nil {
		return Estimate{}, err
	}
	queue := ComputeDiff(mods, hashes, st).Pending()

	set, err := LoadSettings(projectID)
	if err != nil {
		set = Settings{}
	}
	over := 0
	if set.MaxModules != nil && len(queue) > *set.MaxModules {
		over = len(queue) - *set.MaxModules
		queue = queue[:*set.MaxModules]
	}

	byName := make(map[string]Module, len(mods))
	for _, m := range mods {
		k := Snake(m.Name)
		if _, ok := byName[k]; !ok {
			byName[k] = m
		}
	}
	total := 0
	for _, ch := range queue {
		m, ok := byName[Snake(ch.Name)]
		if !ok {
			continue
		}
		src := estimateSourceBytes(root, m)
		if src > LLMMaxSourceBytes {
			src = LLMMaxSourceBytes
		}
		total += EstTokensFromBytes(src + LLMPromptOverheadBytes() + LLMExpectedOutputBytes)
	}
	return Estimate{Modules: len(queue), EstTokens: total, OverLimit: over}, nil
}

// estimateSourceBytes menjumlahkan ukuran berkas sumber modul tanpa membaca isinya.
func estimateSourceBytes(root string, m Module) int {
	rels := m.Sources
	if len(rels) == 0 {
		rels = make([]string, 0, len(m.Files))
		for _, f := range m.Files {
			rels = append(rels, f.Path)
		}
	}
	n := 0
	for _, p := range rels {
		fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		n += int(fi.Size())
	}
	return n
}
