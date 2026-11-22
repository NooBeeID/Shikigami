package thread

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/session"
)

// ClosePreviewParticipant mengidentifikasi pemilik worktree pada preview penutupan.
type ClosePreviewParticipant struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	ProjectID string `json:"projectId"`
}

// CloseUncommitted adalah satu berkas yang belum di-commit.
type CloseUncommitted struct {
	Path   string `json:"path"`
	Status string `json:"status"` // kode porcelain tanpa spasi, mis. "M", "A", "D", "??", "R"
}

// CloseCommit adalah satu commit yang belum di-merge.
type CloseCommit struct {
	SHA     string `json:"sha"` // singkat (hasil %h)
	Subject string `json:"subject"`
}

// CloseWorktreeEntry adalah hasil inspeksi satu worktree.
type CloseWorktreeEntry struct {
	Participant     ClosePreviewParticipant `json:"participant"`
	Path            string                  `json:"path"`
	Branch          string                  `json:"branch"`
	BaseBranch      string                  `json:"baseBranch"` // "" bila HEAD detached saat spawn
	Uncommitted     []CloseUncommitted      `json:"uncommitted"`
	UnmergedCommits []CloseCommit           `json:"unmergedCommits"`
	Error           string                  `json:"error,omitempty"`
}

// ClosePreview adalah respons thread.closePreview. Semua slice selalu non-nil.
type ClosePreview struct {
	ThreadID  string               `json:"threadId"`
	Worktrees []CloseWorktreeEntry `json:"worktrees"`
}

// CloseParams adalah parameter thread.closePreview dan thread.close.
type CloseParams struct {
	ID string `json:"id"`
}

const closeSummaryMax = 2000

// closeState menyimpan state penutupan per thread tanpa mengedit Manager.
var closeState = struct {
	mu      sync.Mutex
	locks   map[string]*sync.Mutex // mutex single-flight per thread
	closing map[string]bool        // tahap penutupan (flag memori)
}{locks: map[string]*sync.Mutex{}, closing: map[string]bool{}}

func closeLock(id string) *sync.Mutex {
	closeState.mu.Lock()
	defer closeState.mu.Unlock()
	l, ok := closeState.locks[id]
	if !ok {
		l = &sync.Mutex{}
		closeState.locks[id] = l
	}
	return l
}

func closeSetClosing(id string, v bool) {
	closeState.mu.Lock()
	defer closeState.mu.Unlock()
	if v {
		closeState.closing[id] = true
	} else {
		delete(closeState.closing, id)
	}
}

func closeIsClosing(id string) bool {
	closeState.mu.Lock()
	defer closeState.mu.Unlock()
	return closeState.closing[id]
}

// ProposeClose mencatat usul penutupan dari master. Tidak menutup thread.
func (m *Manager) ProposeClose(threadID, callerSessionID, summary string) (Thread, error) {
	t, err := m.Get(threadID)
	if err != nil {
		return Thread{}, err
	}
	if t.State == StateClosed {
		return Thread{}, ErrClosed
	}
	isMaster := false
	for _, p := range t.Participants {
		if p.Role == RoleMaster && p.SessionID == callerSessionID {
			isMaster = true
			break
		}
	}
	if !isMaster {
		return Thread{}, fmt.Errorf("hanya master yang dapat mengusulkan penutupan")
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return Thread{}, fmt.Errorf("%w: summary wajib diisi", ErrInvalid)
	}
	if r := []rune(summary); len(r) > closeSummaryMax {
		summary = string(r[:closeSummaryMax])
	}
	if closeIsClosing(threadID) {
		return Thread{}, fmt.Errorf("thread sedang ditutup")
	}
	wasClosed := false
	t, err = m.UpdateThread(threadID, func(t *Thread) {
		if t.State == StateClosed {
			wasClosed = true
			return
		}
		t.ReadyToClose = true
		t.CloseSummary = summary
	})
	if err != nil {
		return Thread{}, err
	}
	if wasClosed {
		return Thread{}, ErrClosed
	}
	m.PostSystem(threadID, "master mengusulkan penutupan: "+summary)
	return t, nil
}

// ClosePreview memeriksa tiap worktree thread secara read-only.
func (m *Manager) ClosePreview(threadID string) (ClosePreview, error) {
	res := ClosePreview{ThreadID: threadID, Worktrees: []CloseWorktreeEntry{}}
	t, err := m.Get(threadID)
	if err != nil {
		return ClosePreview{}, err
	}
	if t.State == StateClosed {
		return res, nil
	}
	recs, err := LoadWorktreeRecords(m.Store().ThreadDir(threadID))
	if err != nil {
		return ClosePreview{}, fmt.Errorf("membaca rekaman worktree: %w", err)
	}
	for _, rec := range recs {
		e := CloseWorktreeEntry{
			Participant:     closeParticipantFor(t, rec),
			Path:            rec.Info.Path,
			Branch:          rec.Info.Branch,
			BaseBranch:      rec.Info.BaseBranch,
			Uncommitted:     []CloseUncommitted{},
			UnmergedCommits: []CloseCommit{},
		}
		st, ierr := closeInspect(rec.Info)
		if ierr != nil {
			e.Error = ierr.Error()
			res.Worktrees = append(res.Worktrees, e)
			continue
		}
		e.Uncommitted = closeParseDirty(st.Dirty)
		e.UnmergedCommits = closeParseUnmerged(st.Unmerged)
		if st.BaseBranch != "" {
			e.BaseBranch = st.BaseBranch
		}
		res.Worktrees = append(res.Worktrees, e)
	}
	return res, nil
}

func closeParticipantFor(t Thread, rec WorktreeRecord) ClosePreviewParticipant {
	if rec.SessionID != "" {
		for _, p := range t.Participants {
			if p.SessionID == rec.SessionID {
				return ClosePreviewParticipant{SessionID: p.SessionID, Name: p.Name, ProjectID: p.ProjectID}
			}
		}
	}
	return ClosePreviewParticipant{SessionID: rec.SessionID, Name: rec.ProjectName, ProjectID: rec.ProjectID}
}

// closeInspect membungkus InspectWorktree agar panic menjadi galat.
func closeInspect(info WorktreeInfo) (st WorktreeStatus, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("inspeksi worktree panik: %v", r)
		}
	}()
	return InspectWorktree(info)
}

func closeParseDirty(lines []string) []CloseUncommitted {
	out := []CloseUncommitted{}
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		status := strings.TrimSpace(line[:2])
		if status == "" {
			status = "?"
		}
		path := line[3:]
		if strings.HasPrefix(path, `"`) {
			if u, err := strconv.Unquote(path); err == nil {
				path = u
			}
		}
		out = append(out, CloseUncommitted{Path: path, Status: status})
	}
	return out
}

func closeParseUnmerged(lines []string) []CloseCommit {
	out := []CloseCommit{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		sha := parts[0]
		if len(sha) > 7 {
			sha = sha[:7]
		}
		subject := ""
		if len(parts) > 1 {
			subject = parts[1]
		}
		out = append(out, CloseCommit{SHA: sha, Subject: subject})
	}
	return out
}

// Close menutup thread: kill sesi, hapus worktree dan direktori master, state closed.
// Idempoten, single-flight per thread, tahan gagal sebagian.
func (m *Manager) Close(threadID string) (Thread, error) {
	l := closeLock(threadID)
	l.Lock()
	defer l.Unlock()

	t, err := m.Get(threadID)
	if err != nil {
		return Thread{}, err
	}
	if t.State == StateClosed {
		return t, nil
	}
	closeSetClosing(threadID, true)
	defer closeSetClosing(threadID, false)

	var errs []string
	threadDir := m.Store().ThreadDir(threadID)
	recs, rerr := LoadWorktreeRecords(threadDir)
	if rerr != nil {
		errs = append(errs, "membaca rekaman worktree: "+rerr.Error())
	}

	alive := m.closeKillAll(t, recs, &errs)
	m.closeRemoveWorktrees(threadDir, recs, &errs)
	m.closeRemoveMasterDir(threadID, &errs)

	if len(alive) > 0 {
		m.closePostErrors(threadID, errs)
		return Thread{}, fmt.Errorf("sesi %s belum bisa dimatikan; coba lagi", strings.Join(alive, ", "))
	}

	closed, uerr := m.UpdateThread(threadID, func(t *Thread) {
		now := time.Now().UTC()
		t.State = StateClosed
		t.ClosedAt = &now
		t.ReadyToClose = false
	})
	if uerr != nil {
		errs = append(errs, "menyimpan state closed: "+uerr.Error())
		m.closePostErrors(threadID, errs)
		return Thread{}, fmt.Errorf("menutup thread: %w", uerr)
	}
	m.FireClose(closed)
	m.closePostErrors(threadID, errs)
	m.PostSystem(threadID, "thread ditutup")
	return closed, nil
}

// closePostErrors mencatat galat terkumpul sebagai satu pesan system ringkas.
func (m *Manager) closePostErrors(threadID string, errs []string) {
	if len(errs) == 0 {
		return
	}
	msg := "penutupan: " + strings.Join(errs, "; ")
	if r := []rune(msg); len(r) > 1000 {
		msg = string(r[:1000]) + "..."
	}
	m.PostSystem(threadID, msg)
}

// closeKillAll mematikan semua sesi thread dan mengembalikan id yang masih hidup.
func (m *Manager) closeKillAll(t Thread, recs []WorktreeRecord, errs *[]string) []string {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, p := range t.Participants {
		add(p.SessionID)
	}
	for _, r := range recs {
		add(r.SessionID)
	}
	var alive []string
	for _, id := range ids {
		err := m.cfg.Sessions.Kill(id)
		if err == nil || errors.Is(err, session.ErrNotFound) {
			continue
		}
		*errs = append(*errs, fmt.Sprintf("sesi %s: %v", id, err))
		alive = append(alive, id)
	}
	return alive
}

// closeRemoveWorktrees menghapus worktree; rekaman dihapus hanya bila sukses.
func (m *Manager) closeRemoveWorktrees(threadDir string, recs []WorktreeRecord, errs *[]string) {
	for _, rec := range recs {
		if err := RemoveWorktree(rec.Info); err != nil {
			*errs = append(*errs, fmt.Sprintf("worktree %s: %v", rec.ProjectName, err))
			continue
		}
		if err := DeleteWorktreeRecord(threadDir, rec.ProjectID); err != nil {
			*errs = append(*errs, fmt.Sprintf("rekaman worktree %s: %v", rec.ProjectName, err))
		}
	}
}

// closeRemoveMasterDir menghapus direktori master hanya bila berada di bawah ThreadDir.
func (m *Manager) closeRemoveMasterDir(threadID string, errs *[]string) {
	dir := filepath.Clean(m.Store().MasterDir(threadID))
	root := filepath.Clean(m.Store().ThreadDir(threadID))
	if dir == root || !strings.HasPrefix(dir, root+string(filepath.Separator)) {
		*errs = append(*errs, "direktori master di luar direktori thread, tidak dihapus")
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		*errs = append(*errs, "hapus direktori master: "+err.Error())
	}
}
