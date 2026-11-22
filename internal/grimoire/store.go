package grimoire

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/NooBeeID/shikigami/internal/fsx"
)

const maxReadBytes = 4 << 20
const stateFile = "state.json"

// Store mengelola berkas grimoire di bawah <dataDir>/grimoire/<projectId>/.
type Store struct{ dataDir string }

// NewStore membuat Store dengan akar dataDir.
func NewStore(dataDir string) *Store { return &Store{dataDir: dataDir} }

// Dir mengembalikan direktori grimoire sebuah project.
func (s *Store) Dir(projectID string) string {
	return filepath.Join(s.dataDir, "grimoire", projectID)
}

// IndexPath mengembalikan path index.md.
func (s *Store) IndexPath(projectID string) string {
	return filepath.Join(s.Dir(projectID), "index.md")
}

// LawsPath mengembalikan path laws.md.
func (s *Store) LawsPath(projectID string) string {
	return filepath.Join(s.Dir(projectID), "laws.md")
}

// StatePath mengembalikan path state.json.
func (s *Store) StatePath(projectID string) string {
	return filepath.Join(s.Dir(projectID), stateFile)
}

// BooksDir mengembalikan direktori buku.
func (s *Store) BooksDir(projectID string) string {
	return filepath.Join(s.Dir(projectID), "books")
}

// BookPath mengembalikan path satu bab (argumen sudah snake, tidak divalidasi).
func (s *Store) BookPath(projectID, book, chapter string) string {
	return filepath.Join(s.BooksDir(projectID), book, chapter+".md")
}

// ValidateProject menolak projectId kosong, ".", atau yang mengandung pemisah path, ".." atau NUL.
func (s *Store) ValidateProject(projectID string) error {
	if projectID == "" || projectID == "." ||
		strings.ContainsAny(projectID, "/\\\x00") || strings.Contains(projectID, "..") {
		return fmt.Errorf("%w: %q", ErrInvalidProject, projectID)
	}
	return nil
}

// resolve memvalidasi rel dan mengembalikan path absolut di bawah Dir project.
func (s *Store) resolve(projectID, rel string) (string, error) {
	if err := s.ValidateProject(projectID); err != nil {
		return "", err
	}
	bad := func(why string) error {
		return fmt.Errorf("%w: %s: %q", ErrInvalidPath, why, rel)
	}
	if rel == "" {
		return "", bad("kosong")
	}
	if strings.ContainsRune(rel, 0) {
		return "", bad("mengandung NUL")
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "\\") {
		return "", bad("absolut")
	}
	for _, seg := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return "", bad("mengandung ..")
		}
	}
	rel = filepath.FromSlash(rel)
	dir := s.Dir(projectID)
	abs, err := fsx.ResolveUnder(dir, rel)
	if err != nil {
		if errors.Is(err, fsx.ErrInvalid) {
			return "", fmt.Errorf("%w: %v", ErrInvalidPath, err)
		}
		return "", err
	}
	if !strings.HasPrefix(abs, filepath.Clean(dir)+string(filepath.Separator)) {
		return "", bad("keluar direktori")
	}
	return abs, nil
}

// WriteFile menulis berkas secara atomik (tmp + sync + rename) di bawah Dir project.
func (s *Store) WriteFile(projectID, rel string, data []byte) error {
	if err := s.ValidateProject(projectID); err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir(projectID), 0o700); err != nil {
		return fmt.Errorf("tulis grimoire %s: %w", rel, err)
	}
	abs, err := s.resolve(projectID, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return fmt.Errorf("tulis grimoire %s: %w", rel, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".tmp-*")
	if err != nil {
		return fmt.Errorf("tulis grimoire %s: %w", rel, err)
	}
	done := false
	defer func() {
		if !done {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("tulis grimoire %s: %w", rel, err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("tulis grimoire %s: %w", rel, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("tulis grimoire %s: %w", rel, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("tulis grimoire %s: %w", rel, err)
	}
	if err := os.Rename(tmp.Name(), abs); err != nil {
		return fmt.Errorf("tulis grimoire %s: %w", rel, err)
	}
	done = true
	return nil
}

// ReadFile membaca berkas di bawah Dir project (maks 4 MiB).
func (s *Store) ReadFile(projectID, rel string) ([]byte, error) {
	if err := s.ValidateProject(projectID); err != nil {
		return nil, err
	}
	if _, err := os.Stat(s.Dir(projectID)); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("grimoire project %s: %w", projectID, os.ErrNotExist)
		}
		return nil, err
	}
	abs, err := s.resolve(projectID, rel)
	if err != nil {
		return nil, err
	}
	tooBig := fmt.Errorf("berkas grimoire %s terlalu besar (maks 4 MiB)", rel)
	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("berkas grimoire %s: %w", rel, os.ErrNotExist)
		}
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("berkas grimoire %s adalah direktori", rel)
	}
	if st.Size() > maxReadBytes {
		return nil, tooBig
	}
	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("berkas grimoire %s: %w", rel, os.ErrNotExist)
		}
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxReadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxReadBytes {
		return nil, tooBig
	}
	return b, nil
}

// Exists melaporkan apakah index.md project sudah ada.
func (s *Store) Exists(projectID string) bool {
	if s.ValidateProject(projectID) != nil {
		return false
	}
	st, err := os.Stat(s.IndexPath(projectID))
	return err == nil && !st.IsDir()
}

// ReadState membaca state.json; ErrNotBuilt bila belum ada.
func (s *Store) ReadState(projectID string) (State, error) {
	b, err := s.ReadFile(projectID, stateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, ErrNotBuilt
		}
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, fmt.Errorf("state.json rusak: %w", err)
	}
	if st.Modules == nil {
		st.Modules = map[string]ModuleState{}
	}
	return st, nil
}

// WriteState menulis state.json (BuiltAt diisi pemanggil).
func (s *Store) WriteState(projectID string, st State) error {
	if err := s.ValidateProject(projectID); err != nil {
		return err
	}
	if st.Modules == nil {
		st.Modules = map[string]ModuleState{}
	}
	if st.ProjectID == "" {
		st.ProjectID = projectID
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("state.json: %w", err)
	}
	return s.WriteFile(projectID, stateFile, append(data, '\n'))
}

// RemoveBooks menghapus seluruh direktori buku project (tidak ada = sukses).
func (s *Store) RemoveBooks(projectID string) error {
	if err := s.ValidateProject(projectID); err != nil {
		return err
	}
	books := filepath.Clean(s.BooksDir(projectID))
	if !strings.HasPrefix(books, filepath.Clean(s.Dir(projectID))+string(filepath.Separator)) {
		return fmt.Errorf("%w: books", ErrInvalidPath)
	}
	return os.RemoveAll(books)
}

// Snake mengubah teks menjadi snake_case ASCII; hasil kosong menjadi "x".
func Snake(s string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
		} else if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "x"
	}
	return out
}

// CamelTag mengubah teks menjadi tag camelCase satu token.
func CamelTag(s string) string {
	parts := strings.Split(Snake(s), "_")
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i > 0 && p[0] >= 'a' && p[0] <= 'z' {
			p = strings.ToUpper(p[:1]) + p[1:]
		}
		b.WriteString(p)
	}
	return b.String()
}
