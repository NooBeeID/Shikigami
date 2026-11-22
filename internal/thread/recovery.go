package thread

// Pemulihan agent thread setelah daemon restart.
//
// Saat start, semua peserta thread aktif hanya DITANDAI mati (tanpa spawn/kill). Agent dihidupkan
// ulang secara lazy pada pesan berikutnya, di cwd yang sama (master: MasterDir; peer: worktree lama).
//
// Catatan: resume sesi provider BELUM didukung (provider.Capabilities hanya Hooks/MCP/IdleEvent dan
// id sesi provider tidak disimpan), sehingga jalur yang dipakai adalah spawn sesi baru dengan konteks
// log thread sebagai prompt awal.

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/mention"
	"github.com/NooBeeID/shikigami/internal/session"
)

const (
	recoveryMsgMaxRunes   = 600
	recoverySendTries     = 5
	recoverySendRetryWait = 300 * time.Millisecond
)

// Recovery menandai dan menghidupkan ulang agent thread yang mati.
type Recovery struct {
	m     *Manager
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewRecovery membuat Recovery.
func NewRecovery(m *Manager) *Recovery {
	return &Recovery{m: m, locks: map[string]*sync.Mutex{}}
}

func (r *Recovery) recoveryLock(threadID string) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	l := r.locks[threadID]
	if l == nil {
		l = &sync.Mutex{}
		r.locks[threadID] = l
	}
	return l
}

// recoveryIsDead: true bila ditandai Dead atau sesi nyata tidak berjalan.
func (r *Recovery) recoveryIsDead(p Participant) bool {
	if p.Dead {
		return true
	}
	if r.m.cfg.Sessions == nil {
		return true
	}
	s, err := r.m.cfg.Sessions.Get(p.SessionID)
	if err != nil || s == nil {
		return true
	}
	return s.Status != session.StatusRunning
}

// MarkDeadOnStart menandai semua peserta thread aktif sebagai mati (tanpa spawn/kill).
func (r *Recovery) MarkDeadOnStart() error {
	var errs []error
	marked := 0
	for _, t := range r.m.managerAllThreads() {
		if t.State != StateActive {
			continue
		}
		anyAlive := false
		for _, p := range t.Participants {
			if !p.Dead {
				anyAlive = true
				break
			}
		}
		if !anyAlive {
			continue
		}
		changed := false
		_, err := r.m.UpdateThread(t.ID, func(th *Thread) {
			if th.State != StateActive {
				return
			}
			for i := range th.Participants {
				if !th.Participants[i].Dead {
					th.Participants[i].Dead = true
					changed = true
				}
			}
		})
		if err != nil {
			r.m.log.Warn("thread: gagal menandai agent mati", "thread", t.ID, "err", err)
			errs = append(errs, fmt.Errorf("tandai mati thread %s: %w", t.ID, err))
			continue
		}
		if changed {
			marked++
			r.m.PostSystem(t.ID, RecoveryDeadNotice)
		}
	}
	if marked > 0 {
		r.m.log.Info("thread: agent ditandai mati saat start", "threads", marked)
	}
	return errors.Join(errs...)
}

func recoveryTruncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}

// recoveryMatch: id cocok dengan SessionID saat ini atau salah satu ID lama peserta.
func recoveryMatch(p Participant, id string) bool {
	if id == "" {
		return false
	}
	if p.SessionID == id {
		return true
	}
	for _, o := range p.PreviousSessionIDs {
		if o == id {
			return true
		}
	}
	return false
}

func recoveryParticipantLabel(t Thread, id string) string {
	switch id {
	case FromUser, FromSystem, ToAll:
		return id
	}
	for _, p := range t.Participants {
		if recoveryMatch(p, id) {
			return p.Name
		}
	}
	return id
}

func recoveryProjectName(projects []ProjectInfo, projectID string) string {
	if projectID == "" {
		return "-"
	}
	for _, pr := range projects {
		if pr.ID == projectID {
			return pr.Name
		}
	}
	return projectID
}

// BuildRecoveryContext menyusun prompt konteks pemulihan (murni, tanpa I/O).
func BuildRecoveryContext(t Thread, projects []ProjectInfo, msgs []ThreadMessage, self Participant) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Ini pemulihan setelah daemon dimulai ulang: sesi sebelumnya untuk kamu (%s, role: %s) sudah mati. Lanjutkan pekerjaan dari log di bawah, bukan mulai dari awal.\n\n", self.Name, self.Role)

	b.WriteString("Goal thread: ")
	if t.Title != "" {
		fmt.Fprintf(&b, "%s (judul: %s)", t.Goal, t.Title)
	} else {
		b.WriteString(t.Goal)
	}
	b.WriteString("\n\nPeserta thread:\n")
	for _, p := range t.Participants {
		status := "hidup"
		if p.Dead {
			status = "mati"
		}
		fmt.Fprintf(&b, "- %s (role: %s, project: %s, %s)\n", p.Name, p.Role, recoveryProjectName(projects, p.ProjectID), status)
	}

	// Log: hanya message dan system, urut waktu naik, maks N terakhir.
	var logMsgs []ThreadMessage
	for _, m := range msgs {
		if m.Kind == KindMessage || m.Kind == KindSystem {
			logMsgs = append(logMsgs, m)
		}
	}
	sort.SliceStable(logMsgs, func(i, j int) bool { return logMsgs[i].TS.Before(logMsgs[j].TS) })
	if len(logMsgs) > RecoveryContextMaxMessages {
		logMsgs = logMsgs[len(logMsgs)-RecoveryContextMaxMessages:]
	}
	lines := make([]string, 0, len(logMsgs))
	for _, m := range logMsgs {
		text := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(m.Text)
		text = recoveryTruncate(text, recoveryMsgMaxRunes)
		lines = append(lines, fmt.Sprintf("[%s] -> [%s]: %s", recoveryParticipantLabel(t, m.From), recoveryParticipantLabel(t, m.To), text))
	}
	dropped := 0
	total := 0
	for _, l := range lines {
		total += len([]rune(l)) + 1
	}
	for total > RecoveryContextMaxRunes && dropped < len(lines) {
		total -= len([]rune(lines[dropped])) + 1
		dropped++
	}
	lines = lines[dropped:]

	b.WriteString("\nLog thread terakhir:\n")
	if len(lines) == 0 && dropped == 0 {
		b.WriteString("(belum ada pesan)\n")
	} else {
		if dropped > 0 {
			fmt.Fprintf(&b, "(… %d pesan lebih lama dipotong)\n", dropped)
		}
		for _, l := range lines {
			b.WriteString(l)
			b.WriteString("\n")
		}
	}

	if self.Role == RoleMaster {
		b.WriteString("\nPeringatan: peserta yang mati dihidupkan otomatis saat kamu thread_send ke mereka; jangan thread_spawn ulang project yang sudah punya peserta.\n")
	}
	return b.String()
}

// recoveryResolveTargets menghasilkan peserta mati yang harus dihidupkan (master dulu, lalu peer).
func (r *Recovery) recoveryResolveTargets(t Thread, ids []string) []Participant {
	var master []Participant
	var peers []Participant
	for _, p := range t.Participants {
		matched := false
		for _, id := range ids {
			if recoveryMatch(p, id) {
				matched = true
				break
			}
		}
		if p.Role == RoleMaster {
			if len(ids) == 0 || matched {
				if r.recoveryIsDead(p) {
					master = append(master, p)
				}
			}
			continue
		}
		if matched && r.recoveryIsDead(p) {
			peers = append(peers, p)
		}
	}
	return append(master, peers...)
}

// EnsureAlive menghidupkan ulang peserta mati yang relevan (idempoten).
func (r *Recovery) EnsureAlive(threadID string, participantIds ...string) error {
	_, err := r.ensureAlive(threadID, participantIds)
	return err
}

// PreSend memenuhi PreSendHook.
func (r *Recovery) PreSend(threadID string, targetSessionIDs []string) (map[string]string, error) {
	return r.ensureAlive(threadID, targetSessionIDs)
}

func (r *Recovery) ensureAlive(threadID string, ids []string) (map[string]string, error) {
	l := r.recoveryLock(threadID)
	l.Lock()
	defer l.Unlock()

	t, err := r.m.Get(threadID)
	if err != nil {
		return nil, err
	}
	if t.State != StateActive {
		return nil, ErrClosed
	}
	targets := r.recoveryResolveTargets(t, ids)
	if len(targets) == 0 {
		return nil, nil
	}
	var projects []ProjectInfo
	if r.m.cfg.Workspaces != nil {
		if ps, ok := r.m.cfg.Workspaces.Projects(t.WorkspaceID); ok {
			projects = ps
		}
	}

	replaced := map[string]string{}
	var errs []error
	for _, target := range targets {
		newID, err := r.respawnOne(t, projects, target)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		replaced[target.SessionID] = newID
		for _, id := range ids {
			if recoveryMatch(target, id) {
				replaced[id] = newID
			}
		}
		if nt, gerr := r.m.Get(threadID); gerr == nil {
			t = nt
		}
	}
	return replaced, errors.Join(errs...)
}

func (r *Recovery) respawnOne(t Thread, projects []ProjectInfo, old Participant) (string, error) {
	fail := func(reason error) (string, error) {
		r.m.log.Warn("thread: pemulihan agent gagal", "thread", t.ID, "participant", old.Name, "err", reason)
		r.m.PostSystem(t.ID, RecoveryFailedPrefix+old.Name+": "+reason.Error())
		return "", fmt.Errorf("hidupkan ulang %s: %w", old.Name, reason)
	}

	msgs, merr := r.m.Messages(t.ID, time.Time{})
	if merr != nil {
		r.m.log.Warn("thread: gagal membaca log untuk konteks pemulihan", "thread", t.ID, "err", merr)
		msgs = nil
	}

	var (
		providerID string
		cwd        string
		projectID  string
		prompt     string
		peerRec    *WorktreeRecord
	)
	threadDir := r.m.Store().ThreadDir(t.ID)
	if old.Role == RoleMaster {
		prov, err := resolveMasterProvider()
		if err != nil {
			return fail(err)
		}
		dir := r.m.Store().MasterDir(t.ID)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fail(fmt.Errorf("siapkan direktori master: %w", err))
		}
		providerID = prov.ID()
		cwd = dir
		prompt = BuildMasterPrompt(t, projects) + "\n\n" + BuildRecoveryContext(t, projects, msgs, old)
	} else {
		prov, err := spawnResolveProvider()
		if err != nil {
			return fail(err)
		}
		recs, err := LoadWorktreeRecords(threadDir)
		if err != nil {
			return fail(fmt.Errorf("baca rekaman worktree: %w", err))
		}
		for i := range recs {
			if recs[i].ProjectID == old.ProjectID {
				rec := recs[i]
				peerRec = &rec
				break
			}
		}
		if peerRec == nil {
			return fail(errors.New("worktree tidak ditemukan (tidak ada rekaman); agent tidak dihidupkan, master perlu memutuskan"))
		}
		if !worktreeExists(peerRec.Info.Path) {
			return fail(fmt.Errorf("worktree tidak ditemukan (%s); agent tidak dihidupkan, master perlu memutuskan", peerRec.Info.Path))
		}
		if peerRec.ProjectName == "" {
			peerRec.ProjectName = recoveryProjectName(projects, old.ProjectID)
		}
		providerID = prov.ID()
		cwd = peerRec.Info.Path
		projectID = old.ProjectID
		brief := "Kamu dihidupkan ulang setelah daemon dimulai ulang; lanjutkan tugasmu di worktree ini; berikut konteks.\n\n" + BuildRecoveryContext(t, projects, msgs, old)
		prompt = BuildPeerBrief(t, peerRec.ProjectName, brief, peerRec.Info)
	}

	s, err := r.m.cfg.Sessions.Create(session.SessionSpec{
		WorkspaceID: t.WorkspaceID,
		ProjectID:   projectID,
		Cwd:         cwd,
		ProviderID:  providerID,
	})
	if err != nil {
		return fail(fmt.Errorf("buat sesi: %w", err))
	}

	found := false
	updated, err := r.m.UpdateThread(t.ID, func(th *Thread) {
		if th.State != StateActive {
			return
		}
		for i := range th.Participants {
			if th.Participants[i].SessionID != old.SessionID {
				continue
			}
			prev := append(append([]string(nil), th.Participants[i].PreviousSessionIDs...), old.SessionID)
			if len(prev) > RecoveryPreviousIDsMax {
				prev = prev[len(prev)-RecoveryPreviousIDsMax:]
			}
			th.Participants[i].SessionID = s.ID
			th.Participants[i].Dead = false
			th.Participants[i].PreviousSessionIDs = prev
			found = true
			return
		}
	})
	if err != nil {
		_ = r.m.cfg.Sessions.Kill(s.ID)
		return fail(fmt.Errorf("perbarui peserta: %w", err))
	}
	if !found {
		_ = r.m.cfg.Sessions.Kill(s.ID)
		return fail(errors.New("peserta tidak ditemukan atau thread sudah ditutup"))
	}

	if peerRec != nil {
		peerRec.SessionID = s.ID
		if uerr := UpsertWorktreeRecord(threadDir, *peerRec); uerr != nil {
			r.m.log.Warn("thread: gagal memperbarui rekaman worktree", "thread", t.ID, "err", uerr)
		}
	}

	for _, p := range updated.Participants {
		if p.SessionID == s.ID {
			r.m.managerFireParticipant(updated, p)
			break
		}
	}
	r.m.PostSystem(t.ID, RecoveryRespawnPrefix+old.Name)

	// Prompt konteks dikirim sinkron agar mendahului pesan berikutnya di antrean.
	var serr error
	for i := 0; i < recoverySendTries; i++ {
		_, serr = r.m.SendToSession(t.ID, "", s.ID, prompt)
		if serr == nil || !errors.Is(serr, mention.ErrNotFound) {
			break
		}
		if i < recoverySendTries-1 {
			time.Sleep(recoverySendRetryWait)
		}
	}
	if serr != nil {
		r.m.log.Warn("thread: gagal mengirim konteks pemulihan", "thread", t.ID, "participant", old.Name, "err", serr)
		r.m.PostSystem(t.ID, "peringatan: gagal mengirim konteks pemulihan ke "+old.Name+": "+serr.Error())
	}
	return s.ID, nil
}

var _ PreSendHook = (*Recovery)(nil).PreSend
