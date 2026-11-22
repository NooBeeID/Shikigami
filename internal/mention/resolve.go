package mention

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/agents"
	"github.com/NooBeeID/shikigami/internal/session"
)

const (
	spawnRetryMax      = 2 * time.Second
	spawnRetryInterval = 100 * time.Millisecond
)

// ResolveOrSpawn me-resolve ref ke sesi existing, sesi berlabel agent, atau
// men-spawn sesi baru dari agent global, lalu mengantre pesan.
func (q *Queue) ResolveOrSpawn(ctx context.Context, pc ProjectContext, ref, text, source string) (Message, bool, error) {
	if strings.TrimSpace(ref) == "" {
		return Message{}, false, ErrInvalid
	}
	from := ""
	if source == SourceAgent {
		from = pc.FromSessionID
	}
	enqueueTo := func(to string) (Message, bool, error) {
		m, err := q.Enqueue(ctx, from, to, text, source)
		return m, false, err
	}

	// 1. Sesi existing (id, nama biasa, atau nama sesi berlabel).
	id, err := q.cfg.Namer.Resolve(pc.WorkspaceID, ref)
	switch {
	case err == nil:
		if from != "" && id == from {
			return Message{}, false, errors.New("tidak dapat mengirim pesan ke sesi sendiri")
		}
		return enqueueTo(id)
	case errors.Is(err, ErrCrossWorkspace), errors.Is(err, ErrInvalid):
		return Message{}, false, err
	}

	// 2. Tanpa alur agent: perilaku lama.
	if q.cfg.Agents == nil || q.cfg.Spawner == nil {
		return Message{}, false, ErrNotFound
	}

	// 3. Sesi berlabel dengan ref mentah.
	if id := q.findLabeled(pc, normName(ref)); id != "" {
		return enqueueTo(id)
	}

	// 4. Resolusi agent/kategori.
	ag, cat, err := q.cfg.Agents.Resolve(ref)
	if err != nil {
		if errors.Is(err, agents.ErrNotFound) {
			return Message{}, false, ErrNotFound
		}
		return Message{}, false, &userFacingError{err}
	}
	agentName := normName(ag.Name)
	if agentName != normName(ref) {
		if id := q.findLabeled(pc, agentName); id != "" {
			return enqueueTo(id)
		}
	}

	// 5. user_only menolak sebelum spawn agar tidak ada sesi yatim.
	if source == SourceAgent && q.Mode() == ModeUserOnly {
		return Message{}, false, ErrUserOnly
	}

	// 6. Konteks project. Bila projectID ada tetapi cwd kosong, spawn tetap
	// jalan dengan Cwd kosong (default manager).
	projectID, cwd := pc.ProjectID, pc.Cwd
	if cwd == "" && projectID != "" && q.cfg.Projects != nil {
		cwd = q.cfg.Projects.ProjectDir(pc.WorkspaceID, projectID)
	}
	if projectID == "" && cwd == "" {
		return Message{}, false, &userFacingError{fmt.Errorf("Tidak ada project konteks untuk membuka sesi agent @%s; pilih project dulu", ref)}
	}

	// 7. Anti spawn ganda: serialkan per workspace|project|agent.
	key := pc.WorkspaceID + "|" + projectID + "|" + agentName
	q.spawnMu.Lock()
	lk := q.spawnLocks[key]
	if lk == nil {
		lk = &sync.Mutex{}
		q.spawnLocks[key] = lk
	}
	q.spawnMu.Unlock()
	lk.Lock()
	defer lk.Unlock()
	if id := q.findLabeled(pc, agentName); id != "" {
		return enqueueTo(id)
	}

	// 8. Spawn.
	sess, err := q.cfg.Spawner.Create(session.SessionSpec{
		WorkspaceID: pc.WorkspaceID,
		ProjectID:   projectID,
		Cwd:         cwd,
		ProviderID:  ag.ProviderID,
		Model:       ag.Model,
	})
	if err != nil {
		return Message{}, false, fmt.Errorf("Gagal membuka sesi agent @%s: %w", ref, err)
	}
	if q.cfg.Labels != nil {
		q.cfg.Labels.Set(sess.ID, agentName)
	}

	// 9. Antre brief; sesi baru mungkin belum Running. Pada mode confirm_agent
	// sesi kosong menunggu konfirmasi (implementasi sederhana yang diterima).
	brief := buildBrief(cat, text)
	deadline := time.Now().Add(spawnRetryMax)
	for {
		msg, err := q.Enqueue(ctx, from, sess.ID, brief, source)
		if err == nil {
			return msg, true, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Message{}, false, err
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return Message{}, false, ctx.Err()
		case <-time.After(spawnRetryInterval):
		}
	}
	if s, gerr := q.cfg.Sessions.Get(sess.ID); gerr != nil || s == nil || s.Status != session.StatusRunning {
		if q.cfg.Labels != nil {
			q.cfg.Labels.Delete(sess.ID)
		}
	}
	return Message{}, false, fmt.Errorf("Gagal membuka sesi agent @%s: sesi tidak berjalan", ref)
}

// findLabeled mencari sesi running berlabel agentName di workspace pc:
// prioritas project konteks, lalu sesi tertua di workspace yang sama.
func (q *Queue) findLabeled(pc ProjectContext, agentName string) string {
	if q.cfg.Labels == nil {
		return ""
	}
	var best, bestCtx *session.Session
	older := func(a, b *session.Session) bool {
		if b == nil {
			return true
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID < b.ID
	}
	for _, id := range q.cfg.Labels.SessionsOf(agentName) {
		s, err := q.cfg.Sessions.Get(id)
		if err != nil || s == nil || s.Status != session.StatusRunning || s.WorkspaceID != pc.WorkspaceID {
			continue
		}
		if pc.ProjectID != "" && s.ProjectID == pc.ProjectID && older(s, bestCtx) {
			bestCtx = s
		}
		if older(s, best) {
			best = s
		}
	}
	if bestCtx != nil {
		return bestCtx.ID
	}
	if best != nil {
		return best.ID
	}
	return ""
}
