package agents

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Store menyimpan registry agent di berkas JSON.
type Store struct {
	mu    sync.Mutex
	path  string
	reg   ProviderLookup
	guard func(string) error
}

// DefaultPath mengembalikan lokasi agents.json di data dir.
func DefaultPath(dataDir string) string { return filepath.Join(dataDir, "agents.json") }

// Open membuat Store tanpa I/O. reg nil memakai registry provider global.
func Open(path string, reg ProviderLookup) *Store {
	if reg == nil {
		reg = globalLookup{}
	}
	return &Store{path: path, reg: reg}
}

// load membaca state dari berkas (mu harus terkunci).
//
// Keputusan: bila JSON rusak, berkas dicadangkan ke <path>.bak (menimpa
// cadangan lama) lalu di-seed ulang; bila pencadangan gagal, galat
// dikembalikan tanpa menimpa berkas.
func (s *Store) load() (fileState, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fileState{}, fmt.Errorf("%w: %v", errStorage, err)
	}
	fresh := func() (fileState, error) {
		st := fileState{Version: fileVersion, Categories: []Category{}, Agents: []Agent{}}
		seedMissing(&st)
		if err := s.save(st); err != nil {
			return fileState{}, err
		}
		return st, nil
	}
	if err != nil || len(bytes.TrimSpace(raw)) == 0 {
		return fresh()
	}
	var st fileState
	if jerr := json.Unmarshal(raw, &st); jerr != nil {
		bak := s.path + ".bak"
		if rerr := os.Rename(s.path, bak); rerr != nil {
			return fileState{}, fmt.Errorf("%w: berkas %s rusak dan tidak bisa dicadangkan: %v", errStorage, s.path, rerr)
		}
		log.Printf("agents: %s rusak (%v); dicadangkan ke %s dan di-seed ulang", s.path, jerr, bak)
		return fresh()
	}
	if st.Version > fileVersion {
		return fileState{}, fmt.Errorf("agents.json versi %d lebih baru dari yang didukung (%d); perbarui aplikasi", st.Version, fileVersion)
	}
	st.Version = fileVersion
	if st.Categories == nil {
		st.Categories = []Category{}
	}
	if st.Agents == nil {
		st.Agents = []Agent{}
	}
	if seedMissing(&st) {
		if err := s.save(st); err != nil {
			return fileState{}, err
		}
	}
	return st, nil
}

// save menulis state secara atomik (temp, fsync, rename; mode 0600).
func (s *Store) save(st fileState) error {
	st.Version = fileVersion
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("%w: %v", errStorage, err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: %v", errStorage, err)
	}
	tmp, err := os.CreateTemp(dir, "agents-*.tmp")
	if err != nil {
		return fmt.Errorf("%w: %v", errStorage, err)
	}
	name := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(name)
		return fmt.Errorf("%w: %v", errStorage, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return fmt.Errorf("%w: %v", errStorage, err)
	}
	if err := os.Rename(name, s.path); err != nil {
		os.Remove(name)
		return fmt.Errorf("%w: %v", errStorage, err)
	}
	return nil
}
