package review

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/NooBeeID/shikigami/internal/fsx"
)

// Store mengelola penyimpanan baseline review per sesi. Mutex per sesi
// melindungi siklus baca-ubah-tulis index.json.
type Store struct {
	dataDir string
	mu      sync.Mutex
	locks   map[string]*sync.Mutex
}

// storeIndex adalah isi index.json: {cwd, files[]}.
type storeIndex struct {
	Cwd   string        `json:"cwd"`
	Files []TouchedFile `json:"files"`
}

// NewStore membuat Store dengan akar penyimpanan dataDir.
func NewStore(dataDir string) *Store {
	return &Store{dataDir: dataDir, locks: map[string]*sync.Mutex{}}
}

// storeValidID menolak id sesi yang bisa keluar dari <dataDir>/review.
func storeValidID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	return !strings.ContainsAny(id, "/\\")
}

// storeLock mengunci mutex sesi dan mengembalikan fungsi unlock.
// Pemakaian: defer s.storeLock(id)().
func (s *Store) storeLock(sessionID string) func() {
	s.mu.Lock()
	m, ok := s.locks[sessionID]
	if !ok {
		m = &sync.Mutex{}
		s.locks[sessionID] = m
	}
	s.mu.Unlock()
	m.Lock()
	return m.Unlock
}

func (s *Store) storeIndexPath(sessionID string) string {
	return filepath.Join(s.SessionDir(sessionID), "index.json")
}

// storeLoad membaca index; galat atau JSON rusak dianggap kosong. Tidak mengunci.
func (s *Store) storeLoad(sessionID string) storeIndex {
	var idx storeIndex
	data, err := os.ReadFile(s.storeIndexPath(sessionID))
	if err != nil || json.Unmarshal(data, &idx) != nil {
		return storeIndex{Files: []TouchedFile{}}
	}
	if idx.Files == nil {
		idx.Files = []TouchedFile{}
	}
	return idx
}

// storeSave menulis index secara atomik (tmp + rename). Tidak mengunci.
func (s *Store) storeSave(sessionID string, idx storeIndex) error {
	if idx.Files == nil {
		idx.Files = []TouchedFile{}
	}
	if err := os.MkdirAll(s.SessionDir(sessionID), 0o700); err != nil {
		return fmt.Errorf("review: simpan index: %w", err)
	}
	data, err := json.Marshal(idx)
	if err != nil {
		return fmt.Errorf("review: simpan index: %w", err)
	}
	dst := s.storeIndexPath(sessionID)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("review: simpan index: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("review: simpan index: %w", err)
	}
	return nil
}

// storeHashBytes menghitung sha256 hex dari byte.
func storeHashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// storeHashFile menghitung sha256 hex isi file secara streaming.
func storeHashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Touched mengembalikan salinan entri index sesi.
func (s *Store) Touched(sessionID string) []TouchedFile {
	if !storeValidID(sessionID) {
		return []TouchedFile{}
	}
	defer s.storeLock(sessionID)()
	idx := s.storeLoad(sessionID)
	return append([]TouchedFile{}, idx.Files...)
}

// Cwd mengembalikan cwd yang tersimpan pada index sesi ("" bila belum ada).
func (s *Store) Cwd(sessionID string) string {
	if !storeValidID(sessionID) {
		return ""
	}
	defer s.storeLock(sessionID)()
	return s.storeLoad(sessionID).Cwd
}

// Files menghitung daftar file berubah, diurut Path naik, selalu non-nil.
func (s *Store) Files(sessionID, cwd string) ([]FileEntry, error) {
	entries := []FileEntry{}
	if !storeValidID(sessionID) {
		return entries, nil
	}
	if cwd == "" {
		cwd = s.Cwd(sessionID)
	}
	if cwd == "" {
		return entries, nil
	}
	files := s.Touched(sessionID)
	for _, t := range files {
		abs, err := fsx.ResolveUnder(cwd, t.Path)
		if err != nil {
			continue
		}
		exists := false
		fi, err := os.Stat(abs)
		switch {
		case err == nil && fi.Mode().IsRegular():
			exists = true
		case err == nil:
			continue // direktori atau bukan file biasa
		case os.IsNotExist(err):
		default:
			continue
		}
		hash := ""
		if exists {
			if hash, err = storeHashFile(abs); err != nil {
				continue
			}
		}
		var st Status
		switch {
		case !t.ExistedBefore && exists:
			st = StatusAdded
		case t.ExistedBefore && !exists:
			st = StatusDeleted
		case t.ExistedBefore && exists && hash != t.BaselineSha256:
			st = StatusModified
		default:
			continue
		}
		entries = append(entries, FileEntry{
			Path:     t.Path,
			Status:   st,
			Reviewed: t.Reviewed,
			Binary:   t.Binary,
			TooLarge: t.TooLarge,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// Prune menghapus data sesi berakhir lama, menyimpan keepEnded teratas.
// Sesi di activeSessionIDs tidak pernah dihapus. Best effort.
func (s *Store) Prune(keepEnded int, activeSessionIDs []string) {
	if keepEnded < 0 {
		keepEnded = 0
	}
	root := filepath.Join(s.dataDir, "review")
	dirs, err := os.ReadDir(root)
	if err != nil {
		return
	}
	active := make(map[string]bool, len(activeSessionIDs))
	for _, id := range activeSessionIDs {
		active[id] = true
	}
	type ended struct {
		id        string
		lastTouch int64
	}
	var list []ended
	for _, d := range dirs {
		name := d.Name()
		if !d.IsDir() || active[name] || !storeValidID(name) {
			continue
		}
		var last int64
		for _, f := range s.storeLoad(name).Files {
			if f.FirstTouchAt > last {
				last = f.FirstTouchAt
			}
		}
		if last == 0 {
			if info, err := d.Info(); err == nil {
				last = info.ModTime().UnixMilli()
			}
		}
		list = append(list, ended{id: name, lastTouch: last})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].lastTouch > list[j].lastTouch })
	if len(list) <= keepEnded {
		return
	}
	for _, e := range list[keepEnded:] {
		unlock := s.storeLock(e.id)
		_ = os.RemoveAll(filepath.Join(root, e.id))
		unlock()
		s.mu.Lock()
		delete(s.locks, e.id)
		s.mu.Unlock()
	}
}

// SessionDir mengembalikan <dataDir>/review/<sessionID>.
func (s *Store) SessionDir(sessionID string) string {
	return filepath.Join(s.dataDir, "review", sessionID)
}

// BaselinePath mengembalikan lokasi salinan baseline sebuah file.
func (s *Store) BaselinePath(sessionID, relPath string) string {
	return filepath.Join(s.SessionDir(sessionID), "baseline", s.HashPath(relPath))
}

// HashPath mengembalikan sha256 hex dari path relatif.
func (s *Store) HashPath(relPath string) string {
	return storeHashBytes([]byte(relPath))
}
