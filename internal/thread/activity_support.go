package thread

// Primitif pendukung kartu aktivitas (epic thread-ticket-03): log ganti-di-tempat
// tanpa memicu hook OnMessage.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// UpsertMessage mengganti baris log ber-id sama (dicari dari belakang) atau
// menambahkan baris baru bila id belum ada.
func (s *Store) UpsertMessage(m ThreadMessage) error {
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
		return fmt.Errorf("upsert pesan: %w", err)
	}
	m.TS = m.TS.UTC()
	buf, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("upsert pesan: marshal: %w", err)
	}

	l := s.storeLock(m.ThreadID)
	l.Lock()
	defer l.Unlock()

	path := filepath.Join(s.ThreadDir(m.ThreadID), storeFileMessages)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("upsert pesan: %w", err)
	}
	var lines [][]byte
	for _, ln := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(ln)) == 0 {
			continue
		}
		lines = append(lines, ln)
	}
	idx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		var probe struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(lines[i], &probe) != nil {
			continue
		}
		if probe.ID == m.ID {
			idx = i
			break
		}
	}

	if idx < 0 {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, storePermFile)
		if err != nil {
			return fmt.Errorf("upsert pesan: %w", err)
		}
		line := append(buf, '\n')
		n, err := f.Write(line)
		if err == nil && n < len(line) {
			err = fmt.Errorf("tulis sebagian (%d/%d byte)", n, len(line))
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("upsert pesan: %w", err)
		}
		return nil
	}

	lines[idx] = buf
	out := bytes.Join(lines, []byte("\n"))
	out = append(out, '\n')
	if err := storeWriteFileAtomic(path, out, storePermFile); err != nil {
		return fmt.Errorf("upsert pesan: %w", err)
	}
	return nil
}

// UpsertMessage mencatat/mengganti pesan di log thread dan mempublikasikannya.
// Sengaja TIDAK memanggil managerFireMessage: pesan activity tidak boleh masuk
// router/antrean mention, anti-loop, atau hook lain sebagai pesan baru.
func (m *Manager) UpsertMessage(threadID string, msg ThreadMessage) (ThreadMessage, error) {
	t, err := m.Get(threadID)
	if err != nil {
		return ThreadMessage{}, err
	}
	if t.State == StateClosed {
		return ThreadMessage{}, ErrClosed
	}
	msg.ThreadID = threadID
	if msg.ID == "" {
		msg.ID = NewID()
	}
	if msg.TS.IsZero() {
		msg.TS = time.Now().UTC()
	} else {
		msg.TS = msg.TS.UTC()
	}
	if msg.Kind == "" {
		msg.Kind = KindMessage
	}
	if msg.To == "" {
		msg.To = ToAll
	}
	if msg.From == "" {
		msg.From = FromSystem
	}
	if err := m.cfg.Store.UpsertMessage(msg); err != nil {
		return ThreadMessage{}, err
	}
	m.managerPublish(EventMessage, msg)
	return msg, nil
}

// ActiveThreads mengembalikan thread berstate active (slice non-nil).
func (m *Manager) ActiveThreads() []Thread {
	out := []Thread{}
	for _, t := range m.managerAllThreads() {
		if t.State == StateActive {
			out = append(out, managerNormalize(t))
		}
	}
	return out
}
