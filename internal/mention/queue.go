package mention

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/provider"
	"github.com/NooBeeID/shikigami/internal/session"
	"github.com/NooBeeID/shikigami/internal/status"
)

const (
	maxQueuePerTarget = 50
	sendCooldown      = 1500 * time.Millisecond
	maxDone           = 200
)

// QueueConfig adalah dependensi Queue.
type QueueConfig struct {
	Sessions SessionSource
	Namer    *Namer
	Publish  func(topic string, payload any) // srv.Publish
	Logger   *slog.Logger                    // boleh nil -> slog.Default()

	// Alur agent (semua boleh nil; Agents/Spawner nil -> alur agent dilewati).
	Agents   AgentResolver
	Spawner  SessionSpawner
	Projects ProjectDirectory
	Labels   *SpawnLabels
}

type entry struct {
	msg     Message
	sending bool // sedang ditulis ke sesi; tidak bisa dibatalkan
}

type target struct {
	entries       []*entry
	armed         bool // OnIdle sudah terdaftar (satu per target)
	cancelIdle    func()
	idleGen       uint64 // generasi pendaftaran; callback basi diabaikan
	cooldownUntil time.Time
	timer         *time.Timer // timer cooldown sekali jalan
}

// Queue adalah antrean pesan per sesi target.
type Queue struct {
	cfg     QueueConfig
	mu      sync.Mutex
	mode    Mode
	targets map[string]*target
	byID    map[string]*entry
	done    map[string]State
	doneIDs []string

	spawnMu    sync.Mutex // melindungi spawnLocks
	spawnLocks map[string]*sync.Mutex
}

var _ Enqueuer = (*Queue)(nil)
var _ QueueReader = (*Queue)(nil)
var _ TargetRouter = (*Queue)(nil)

// NewQueue membuat Queue dengan mode default confirm_agent.
func NewQueue(cfg QueueConfig) *Queue {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Publish == nil {
		cfg.Publish = func(string, any) {}
	}
	return &Queue{
		cfg:        cfg,
		mode:       ModeConfirmAgent,
		targets:    map[string]*target{},
		byID:       map[string]*entry{},
		done:       map[string]State{},
		spawnLocks: map[string]*sync.Mutex{},
	}
}

// Enqueue memvalidasi lalu memasukkan pesan ke antrean target dan mencoba mengirim.
func (q *Queue) Enqueue(ctx context.Context, from, to, text, source string) (Message, error) {
	if strings.TrimSpace(text) == "" {
		return Message{}, fmt.Errorf("%w: teks kosong", ErrInvalid)
	}
	if (from == "" && source != SourceUser) || (from != "" && source != SourceAgent) {
		return Message{}, fmt.Errorf("%w: sumber pesan tidak konsisten", ErrInvalid)
	}
	sess, err := q.cfg.Sessions.Get(to)
	if err != nil || sess == nil || sess.Status != session.StatusRunning {
		return Message{}, ErrNotFound
	}
	if from != "" {
		if from == to {
			return Message{}, fmt.Errorf("%w: tidak bisa mengirim ke diri sendiri", ErrInvalid)
		}
		fs, err := q.cfg.Sessions.Get(from)
		if err != nil || fs == nil {
			return Message{}, fmt.Errorf("%w: sesi pengirim tidak ditemukan", ErrInvalid)
		}
		if fs.WorkspaceID != sess.WorkspaceID {
			return Message{}, ErrCrossWorkspace
		}
	}
	fromName := FromUser
	fromVal := FromUser
	if from != "" {
		fromVal = from
		fromName = ""
		if q.cfg.Namer != nil {
			fromName = q.cfg.Namer.NameOf(from)
		}
		if fromName == "" {
			fromName = from
		}
	}

	q.mu.Lock()
	if source == SourceAgent && q.mode == ModeUserOnly {
		q.mu.Unlock()
		return Message{}, ErrUserOnly
	}
	t := q.targets[to]
	if t != nil && len(t.entries) >= maxQueuePerTarget {
		q.mu.Unlock()
		return Message{}, ErrConflict
	}
	if t == nil {
		t = &target{}
		q.targets[to] = t
	}
	state := StateQueued
	if source == SourceAgent && q.mode == ModeConfirmAgent {
		state = StateAwaitingConfirm
	}
	e := &entry{msg: Message{
		ID:        NewMessageID(),
		From:      fromVal,
		FromName:  fromName,
		To:        to,
		Text:      text,
		CreatedAt: time.Now().UTC(),
		State:     state,
		Source:    source,
	}}
	t.entries = append(t.entries, e)
	q.byID[e.msg.ID] = e
	q.publishLocked(to)
	q.mu.Unlock()

	q.pump(to)

	q.mu.Lock()
	defer q.mu.Unlock()
	return e.msg, nil
}

// pump adalah satu-satunya jalur pengiriman.
func (q *Queue) pump(to string) {
	q.mu.Lock()
	t := q.targets[to]
	if t == nil {
		q.mu.Unlock()
		return
	}
	var e *entry
	for _, c := range t.entries {
		if c.msg.State == StateQueued && !c.sending {
			e = c
			break
		}
	}
	if e == nil {
		q.disarmLocked(t)
		if len(t.entries) == 0 {
			delete(q.targets, to)
		}
		q.mu.Unlock()
		return
	}
	if now := time.Now(); now.Before(t.cooldownUntil) {
		if t.timer == nil {
			t.timer = time.AfterFunc(t.cooldownUntil.Sub(now), func() {
				q.mu.Lock()
				if q.targets[to] == t {
					t.timer = nil
				}
				q.mu.Unlock()
				q.pump(to)
			})
		}
		q.mu.Unlock()
		return
	}
	if !status.IsIdle(to) {
		if !t.armed {
			q.armLocked(to, t)
		}
		q.mu.Unlock()
		return
	}
	e.sending = true
	msg := e.msg
	q.mu.Unlock()

	// Hitungan submit sebelum menulis; dipakai membuktikan Enter benar-benar diterima.
	submitsBefore, confirmable := status.PromptSubmits(to)
	err := q.deliver(msg)
	if err == nil && confirmable {
		go q.confirmEnter(to, submitsBefore)
	}

	q.mu.Lock()
	e.sending = false
	if q.targets[to] != t {
		q.mu.Unlock()
		return
	}
	q.removeEntryLocked(t, e)
	if err == nil {
		e.msg.State = StateDelivered
		t.cooldownUntil = time.Now().Add(sendCooldown)
	} else {
		e.msg.State = StateFailed
		e.msg.Error = err.Error()
		q.cfg.Logger.Warn("mention: gagal mengirim pesan", "to", to, "err", err)
	}
	q.recordDoneLocked(e.msg.ID, e.msg.State)
	q.publishLocked(to)
	more := false
	for _, c := range t.entries {
		if c.msg.State == StateQueued {
			more = true
			break
		}
	}
	if !more && len(t.entries) == 0 && t.timer == nil {
		q.disarmLocked(t)
		// target dipertahankan selama cooldown berlaku agar cooldown tidak hilang
		if time.Now().After(t.cooldownUntil) {
			delete(q.targets, to)
		}
	}
	q.mu.Unlock()
	if more {
		q.pump(to)
	}
}

func (q *Queue) armLocked(to string, t *target) {
	t.idleGen++
	gen := t.idleGen
	t.armed = true
	t.cancelIdle = status.OnIdle(to, func() { go q.onIdle(to, gen) })
}

func (q *Queue) onIdle(to string, gen uint64) {
	q.mu.Lock()
	t := q.targets[to]
	if t == nil || t.idleGen != gen {
		q.mu.Unlock()
		return
	}
	t.armed = false
	t.cancelIdle = nil
	q.mu.Unlock()
	q.pump(to)
}

func (q *Queue) disarmLocked(t *target) {
	if t.armed {
		if t.cancelIdle != nil {
			t.cancelIdle()
		}
		t.armed = false
		t.cancelIdle = nil
		t.idleGen++
	}
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}

func (q *Queue) removeEntryLocked(t *target, e *entry) {
	for i, c := range t.entries {
		if c == e {
			t.entries = append(t.entries[:i], t.entries[i+1:]...)
			break
		}
	}
	delete(q.byID, e.msg.ID)
}

func (q *Queue) recordDoneLocked(id string, st State) {
	q.done[id] = st
	q.doneIDs = append(q.doneIDs, id)
	for len(q.doneIDs) > maxDone {
		delete(q.done, q.doneIDs[0])
		q.doneIDs = q.doneIDs[1:]
	}
}

// deliver menulis pesan ke sesi target lewat provider (tanpa lock).
func (q *Queue) deliver(m Message) error {
	sess, err := q.cfg.Sessions.Get(m.To)
	if err != nil || sess == nil || sess.Status != session.StatusRunning {
		return errors.New("sesi target tidak berjalan")
	}
	p, ok := provider.Get(sess.ProviderID)
	if !ok {
		p = provider.Default()
	}
	if p == nil {
		return errors.New("provider tidak tersedia")
	}
	text := "[pesan dari " + m.FromName + "] " + m.Text
	return p.SendInput(sessionWriter{mgr: q.cfg.Sessions, id: m.To}, text)
}

type sessionWriter struct {
	mgr SessionSource
	id  string
}

func (w sessionWriter) Write(b []byte) (int, error) {
	if err := w.mgr.Write(w.id, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (q *Queue) pendingOfLocked(t *target) []Message {
	out := []Message{}
	if t == nil {
		return out
	}
	for _, e := range t.entries {
		out = append(out, e.msg)
	}
	return out
}

func (q *Queue) publishLocked(to string) {
	q.cfg.Publish("mention.queue", map[string]any{
		"sessionId": to,
		"queue":     q.pendingOfLocked(q.targets[to]),
	})
}

// Position mengembalikan posisi 1-based pesan di antrean pending; 0 bila bukan pending.
func (q *Queue) Position(messageID string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.byID[messageID]
	if e == nil {
		return 0
	}
	t := q.targets[e.msg.To]
	if t == nil {
		return 0
	}
	for i, c := range t.entries {
		if c == e {
			return i + 1
		}
	}
	return 0
}

// Pending mengembalikan pesan pending satu target, urut FIFO.
func (q *Queue) Pending(sessionID string) []Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pendingOfLocked(q.targets[sessionID])
}

// PendingAll mengembalikan semua pesan pending, urut CreatedAt lalu ID.
func (q *Queue) PendingAll() []Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := []Message{}
	for _, t := range q.targets {
		out = append(out, q.pendingOfLocked(t)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (q *Queue) missingLocked(id string) error {
	if _, ok := q.done[id]; ok {
		return fmt.Errorf("%w: pesan sudah selesai", ErrInvalid)
	}
	return ErrNotFound
}

// Cancel membatalkan pesan pending.
func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.byID[id]
	if e == nil {
		return q.missingLocked(id)
	}
	if e.sending {
		return fmt.Errorf("%w: pesan sedang dikirim", ErrInvalid)
	}
	to := e.msg.To
	t := q.targets[to]
	if t != nil {
		q.removeEntryLocked(t, e)
	} else {
		delete(q.byID, id)
	}
	q.recordDoneLocked(id, StateFailed)
	q.publishLocked(to)
	if t != nil && len(t.entries) == 0 {
		q.disarmLocked(t)
		if time.Now().After(t.cooldownUntil) {
			delete(q.targets, to)
		}
	}
	return nil
}

// Confirm mengubah pesan awaiting_confirm menjadi queued dan memicu pengiriman.
func (q *Queue) Confirm(id string) (Message, error) {
	q.mu.Lock()
	e := q.byID[id]
	if e == nil {
		err := q.missingLocked(id)
		q.mu.Unlock()
		return Message{}, err
	}
	if e.msg.State != StateAwaitingConfirm {
		q.mu.Unlock()
		return Message{}, fmt.Errorf("%w: pesan tidak menunggu konfirmasi", ErrInvalid)
	}
	e.msg.State = StateQueued
	to := e.msg.To
	q.publishLocked(to)
	q.mu.Unlock()

	q.pump(to)

	q.mu.Lock()
	defer q.mu.Unlock()
	return e.msg, nil
}

// Mode mengembalikan mode saat ini.
func (q *Queue) Mode() Mode {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.mode
}

// SetMode mengubah mode; mode tak dikenal -> ErrInvalid.
func (q *Queue) SetMode(m Mode) error {
	switch m {
	case ModeAuto, ModeConfirmAgent, ModeUserOnly:
	default:
		return fmt.Errorf("%w: mode tidak dikenal %q", ErrInvalid, string(m))
	}
	q.mu.Lock()
	q.mode = m
	q.mu.Unlock()
	return nil
}

// HandleExit membersihkan antrean sesi yang berakhir.
func (q *Queue) HandleExit(sessionID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	t := q.targets[sessionID]
	if t == nil {
		return
	}
	for _, e := range t.entries {
		if !e.sending {
			e.msg.State = StateFailed
			e.msg.Error = "sesi target berakhir"
		}
		q.recordDoneLocked(e.msg.ID, StateFailed)
		delete(q.byID, e.msg.ID)
	}
	q.disarmLocked(t)
	delete(q.targets, sessionID)
	q.publishLocked(sessionID)
}
