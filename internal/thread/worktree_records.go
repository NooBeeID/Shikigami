package thread

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WorktreeRecord mencatat worktree milik thread untuk satu project (dibaca tiket 04/05).
type WorktreeRecord struct {
	ProjectID   string       `json:"projectId"`
	ProjectName string       `json:"projectName"`
	ProjectSlug string       `json:"projectSlug"`
	SessionID   string       `json:"sessionId"` // "" sampai sesi dibuat
	Info        WorktreeInfo `json:"info"`
	CreatedAt   time.Time    `json:"createdAt"`
}

var worktreeRecordsMu sync.Mutex

func worktreeRecordsPath(threadDir string) string {
	return filepath.Join(threadDir, "worktrees.json")
}

func worktreeRecordsRead(threadDir string) ([]WorktreeRecord, error) {
	b, err := os.ReadFile(worktreeRecordsPath(threadDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var recs []WorktreeRecord
	if err := json.Unmarshal(b, &recs); err != nil {
		return nil, fmt.Errorf("worktrees.json rusak: %w", err)
	}
	return recs, nil
}

func worktreeRecordsWrite(threadDir string, recs []WorktreeRecord) error {
	if recs == nil {
		recs = []WorktreeRecord{}
	}
	if err := os.MkdirAll(threadDir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	path := worktreeRecordsPath(threadDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// LoadWorktreeRecords membaca catatan worktree; berkas belum ada -> (nil, nil).
func LoadWorktreeRecords(threadDir string) ([]WorktreeRecord, error) {
	worktreeRecordsMu.Lock()
	defer worktreeRecordsMu.Unlock()
	return worktreeRecordsRead(threadDir)
}

// UpsertWorktreeRecord mengganti/menambah catatan berkunci ProjectID secara atomik.
func UpsertWorktreeRecord(threadDir string, rec WorktreeRecord) error {
	if threadDir == "" {
		return errors.New("threadDir kosong")
	}
	if rec.ProjectID == "" {
		return errors.New("ProjectID kosong")
	}
	worktreeRecordsMu.Lock()
	defer worktreeRecordsMu.Unlock()
	recs, err := worktreeRecordsRead(threadDir)
	if err != nil {
		return err
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	replaced := false
	for i := range recs {
		if recs[i].ProjectID == rec.ProjectID {
			recs[i] = rec
			replaced = true
			break
		}
	}
	if !replaced {
		recs = append(recs, rec)
	}
	return worktreeRecordsWrite(threadDir, recs)
}

// DeleteWorktreeRecord menghapus catatan; idempoten.
func DeleteWorktreeRecord(threadDir, projectID string) error {
	worktreeRecordsMu.Lock()
	defer worktreeRecordsMu.Unlock()
	recs, err := worktreeRecordsRead(threadDir)
	if err != nil {
		return err
	}
	out := recs[:0:0]
	for _, r := range recs {
		if r.ProjectID != projectID {
			out = append(out, r)
		}
	}
	if len(out) == len(recs) {
		return nil
	}
	return worktreeRecordsWrite(threadDir, out)
}
