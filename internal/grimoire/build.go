package grimoire

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Build memindai project, menulis halaman bab, lalu index.md dan state.json
// paling akhir. Tidak menulis apa pun di dalam projectPath.
func Build(store *Store, projectID, projectPath string) (Summary, error) {
	if store == nil {
		return Summary{}, errors.New("store grimoire kosong")
	}
	if err := store.ValidateProject(projectID); err != nil {
		return Summary{}, err
	}
	if !filepath.IsAbs(projectPath) {
		return Summary{}, fmt.Errorf("path project tidak valid: %s", projectPath)
	}
	root := filepath.Clean(projectPath)
	st, err := os.Stat(root)
	if err != nil {
		return Summary{}, fmt.Errorf("path project tidak valid: %s: %w", projectPath, err)
	}
	if !st.IsDir() {
		return Summary{}, fmt.Errorf("path project tidak valid: %s", projectPath)
	}

	files, err := Scan(root)
	if err != nil {
		return Summary{}, fmt.Errorf("pindai project: %w", err)
	}
	mods := GroupModules(files)
	hashes := make(map[string]string, len(mods))
	for _, m := range mods {
		h, err := HashModule(root, m)
		if err != nil {
			return Summary{}, fmt.Errorf("hash modul %s: %w", m.Name, err)
		}
		hashes[m.Name] = h
	}

	books, pages := renderBooks(mods, hashes)
	index := BuildIndex(books)

	// Hapus index/state lebih dulu agar kegagalan di tengah tidak
	// meninggalkan index yang menunjuk bab yang tidak ada.
	for _, p := range []string{store.IndexPath(projectID), store.StatePath(projectID)} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Summary{}, fmt.Errorf("hapus %s: %w", filepath.Base(p), err)
		}
	}
	if err := store.RemoveBooks(projectID); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Summary{}, fmt.Errorf("hapus buku lama: %w", err)
	}

	for _, p := range pages {
		if err := store.WriteFile(projectID, p.Path, []byte(p.Content)); err != nil {
			return Summary{}, fmt.Errorf("tulis halaman %s: %w", p.Path, err)
		}
	}
	if err := store.WriteFile(projectID, "index.md", []byte(index)); err != nil {
		return Summary{}, fmt.Errorf("tulis index.md: %w", err)
	}

	state := State{
		ProjectID: projectID,
		BuiltAt:   time.Now().UTC(),
		Modules:   make(map[string]ModuleState, len(mods)),
	}
	for _, m := range mods {
		src := make([]string, len(m.Sources))
		copy(src, m.Sources)
		state.Modules[Snake(m.Name)] = ModuleState{Hash: hashes[m.Name], Sources: src}
	}
	if err := store.WriteState(projectID, state); err != nil {
		return Summary{}, fmt.Errorf("tulis state.json: %w", err)
	}

	return Summary{Books: len(books), Pages: len(pages), IndexBytes: len(index)}, nil
}
