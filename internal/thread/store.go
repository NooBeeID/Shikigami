package thread

// Penyimpanan thread berbasis disk.
//
// Layout:
//
//	<baseDir>/<id>/thread.json      (ditulis atomik: tmp + rename)
//	<baseDir>/<id>/messages.jsonl   (append-only, satu pesan per baris)
//	<baseDir>/<id>/master/          (cwd netral sesi master)
//
// Jaminan: thread.json atomik, messages.jsonl ditulis dengan satu Write per
// baris di bawah mutex per thread, baris rusak dilewati saat membaca. Izin
// berkas 0600 dan direktori 0700.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("thread tidak ditemukan")
	ErrInvalid  = errors.New("masukan thread tidak valid")
)

const (
	storeFileThread   = "thread.json"
	storeFileMessages = "messages.jsonl"
	storeDirMaster    = "master"

	storePermFile os.FileMode = 0o600
	storePermDir  os.FileMode = 0o700

	storeMaxIDLen = 64
)

// Store menyimpan thread di disk. Aman dipakai banyak goroutine.
type Store struct {
	baseDir string
	mu      sync.Mutex             // melindungi peta locks
	locks   map[string]*sync.Mutex // satu mutex per threadId
	log     *slog.Logger
}

// NewStore membuat Store dengan direktori dasar baseDir (dibuat bila belum ada).
func NewStore(baseDir string) *Store {
	if baseDir != "" {
		if err := os.MkdirAll(baseDir, storePermDir); err != nil {
			slog.Warn("gagal membuat direktori thread", "dir", baseDir, "err", err)
		}
	}
	return &Store{
		baseDir: baseDir,
		locks:   make(map[string]*sync.Mutex),
		log:     slog.Default(),
	}
}

func (s *Store) logger() *slog.Logger {
	if s.log == nil {
		return slog.Default()
	}
	return s.log
}

func (s *Store) storeLock(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks == nil {
		s.locks = make(map[string]*sync.Mutex)
	}
	l, ok := s.locks[id]
	if !ok {
		l = &sync.Mutex{}
		s.locks[id] = l
	}
	return l
}

func storeValidID(id string) bool {
	if id == "" || len(id) > storeMaxIDLen {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// ThreadDir mengembalikan direktori thread (tanpa validasi).
func (s *Store) ThreadDir(id string) string { return filepath.Join(s.baseDir, id) }

// MasterDir mengembalikan cwd netral sesi master.
func (s *Store) MasterDir(id string) string {
	return filepath.Join(s.ThreadDir(id), storeDirMaster)
}

func storeWriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".thread-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	fail := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func storeNormalize(t Thread) Thread {
	if t.Participants == nil {
		t.Participants = []Participant{}
	}
	t.CreatedAt = t.CreatedAt.UTC()
	if t.ClosedAt != nil {
		c := t.ClosedAt.UTC()
		t.ClosedAt = &c
	}
	return t
}

func storeMarshalThread(t Thread) ([]byte, error) {
	data, err := json.MarshalIndent(storeNormalize(t), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func (s *Store) storeReadThread(id string) (Thread, error) {
	data, err := os.ReadFile(filepath.Join(s.ThreadDir(id), storeFileThread))
	if err != nil {
		if os.IsNotExist(err) {
			return Thread{}, fmt.Errorf("thread tidak ditemukan: %s: %w", id, ErrNotFound)
		}
		return Thread{}, fmt.Errorf("baca thread.json (%s): %w", id, err)
	}
	var t Thread
	if err := json.Unmarshal(data, &t); err != nil {
		return Thread{}, fmt.Errorf("thread.json rusak (%s): %w", id, err)
	}
	if t.Participants == nil {
		t.Participants = []Participant{}
	}
	return t, nil
}

// Create membuat thread baru (state active) beserta berkas dan direktori master.
func (s *Store) Create(workspaceID, title, goal string) (Thread, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return Thread{}, fmt.Errorf("workspaceId kosong: %w", ErrInvalid)
	}
	id := NewID()
	if _, err := os.Stat(s.ThreadDir(id)); err == nil {
		id = NewID()
		if _, err := os.Stat(s.ThreadDir(id)); err == nil {
			return Thread{}, fmt.Errorf("tabrakan ID thread: %s", id)
		}
	}
	if err := os.MkdirAll(s.baseDir, storePermDir); err != nil {
		return Thread{}, fmt.Errorf("buat direktori thread: %w", err)
	}
	dir := s.ThreadDir(id)
	if err := os.MkdirAll(s.MasterDir(id), storePermDir); err != nil {
		_ = os.RemoveAll(dir)
		return Thread{}, fmt.Errorf("buat direktori thread: %w", err)
	}
	_ = os.Chmod(dir, storePermDir)
	_ = os.Chmod(s.MasterDir(id), storePermDir)

	t := Thread{
		ID:           id,
		WorkspaceID:  workspaceID,
		Title:        title,
		Goal:         goal,
		State:        StateActive,
		CreatedAt:    time.Now().UTC(),
		Participants: []Participant{},
		ReadyToClose: false,
	}

	l := s.storeLock(id)
	l.Lock()
	defer l.Unlock()

	f, err := os.OpenFile(filepath.Join(dir, storeFileMessages), os.O_CREATE|os.O_WRONLY|os.O_APPEND, storePermFile)
	if err != nil {
		_ = os.RemoveAll(dir)
		return Thread{}, fmt.Errorf("buat messages.jsonl: %w", err)
	}
	_ = f.Close()

	data, err := storeMarshalThread(t)
	if err == nil {
		err = storeWriteFileAtomic(filepath.Join(dir, storeFileThread), data, storePermFile)
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return Thread{}, fmt.Errorf("tulis thread.json: %w", err)
	}
	return t, nil
}

// Get membaca satu thread. ID tidak valid diperlakukan sebagai ErrNotFound.
func (s *Store) Get(id string) (Thread, error) {
	if !storeValidID(id) {
		return Thread{}, fmt.Errorf("thread tidak ditemukan: %s: %w", id, ErrNotFound)
	}
	return s.storeReadThread(id)
}

// List mengembalikan thread milik workspace (termasuk closed), terbaru dulu.
func (s *Store) List(workspaceID string) ([]Thread, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Thread{}, nil
		}
		return nil, fmt.Errorf("baca direktori thread: %w", err)
	}
	out := []Thread{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !storeValidID(name) {
			continue
		}
		t, err := s.storeReadThread(name)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				s.logger().Warn("thread.json rusak, dilewati", "id", name, "err", err)
			}
			continue
		}
		if t.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out, nil
}

// Save menimpa thread.json secara atomik. Tidak membuat thread baru.
func (s *Store) Save(t Thread) error {
	if !storeValidID(t.ID) {
		return fmt.Errorf("id thread tidak valid: %w", ErrInvalid)
	}
	if _, err := os.Stat(s.ThreadDir(t.ID)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("thread tidak ditemukan: %s: %w", t.ID, ErrNotFound)
		}
		return err
	}
	data, err := storeMarshalThread(t)
	if err != nil {
		return fmt.Errorf("marshal thread: %w", err)
	}
	l := s.storeLock(t.ID)
	l.Lock()
	defer l.Unlock()
	return storeWriteFileAtomic(filepath.Join(s.ThreadDir(t.ID), storeFileThread), data, storePermFile)
}

// AppendMessage menambahkan satu baris JSON ke messages.jsonl. ID/TS diisi pemanggil.
func (s *Store) AppendMessage(m ThreadMessage) error {
	if !storeValidID(m.ThreadID) {
		return fmt.Errorf("threadId tidak valid: %w", ErrInvalid)
	}
	if m.ID == "" {
		return fmt.Errorf("id pesan kosong: %w", ErrInvalid)
	}
	if m.TS.IsZero() {
		return fmt.Errorf("ts pesan kosong: %w", ErrInvalid)
	}
	if _, err := os.Stat(s.ThreadDir(m.ThreadID)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("thread tidak ditemukan: %s: %w", m.ThreadID, ErrNotFound)
		}
		return err
	}
	m.TS = m.TS.UTC()
	buf, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal pesan: %w", err)
	}
	buf = append(buf, '\n')

	l := s.storeLock(m.ThreadID)
	l.Lock()
	defer l.Unlock()
	f, err := os.OpenFile(filepath.Join(s.ThreadDir(m.ThreadID), storeFileMessages), os.O_CREATE|os.O_WRONLY|os.O_APPEND, storePermFile)
	if err != nil {
		return fmt.Errorf("buka messages.jsonl: %w", err)
	}
	n, err := f.Write(buf)
	if err == nil && n < len(buf) {
		err = fmt.Errorf("tulis sebagian (%d/%d byte)", n, len(buf))
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("tulis pesan: %w", err)
	}
	return nil
}

// Messages membaca pesan thread dengan TS > sinceTS (nol = semua), urut naik.
func (s *Store) Messages(id string, sinceTS time.Time) ([]ThreadMessage, error) {
	if !storeValidID(id) {
		return nil, fmt.Errorf("thread tidak ditemukan: %s: %w", id, ErrNotFound)
	}
	if _, err := os.Stat(s.ThreadDir(id)); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("thread tidak ditemukan: %s: %w", id, ErrNotFound)
		}
		return nil, err
	}

	l := s.storeLock(id)
	l.Lock()
	data, err := os.ReadFile(filepath.Join(s.ThreadDir(id), storeFileMessages))
	l.Unlock()
	if err != nil {
		if os.IsNotExist(err) {
			return []ThreadMessage{}, nil
		}
		return nil, fmt.Errorf("baca messages.jsonl: %w", err)
	}

	out := []ThreadMessage{}
	for i, raw := range bytes.Split(data, []byte("\n")) {
		line := bytes.TrimSpace(raw)
		if len(line) == 0 {
			continue
		}
		var m ThreadMessage
		err := json.Unmarshal(line, &m)
		if err == nil && m.ID == "" {
			err = errors.New("id pesan kosong")
		}
		if err == nil && m.ThreadID != id {
			err = errors.New("threadId tidak cocok")
		}
		if err != nil {
			s.logger().Warn("baris pesan thread rusak, dilewati", "thread", id, "line", i+1, "err", err)
			continue
		}
		if !sinceTS.IsZero() && !m.TS.After(sinceTS) {
			continue
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TS.Before(out[j].TS) })
	return out, nil
}
