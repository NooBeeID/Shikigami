package thread

// Kartu aktivitas thread: menurunkan pesan kind=activity dari hook tool call.

import (
	"errors"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ActivitySource adalah sumber hook tool call (dipenuhi *status.Engine).
type ActivitySource interface {
	AddHookListener(fn func(sessionID, event string, payload map[string]any)) (cancel func())
}

const (
	ActivityMergeGap   = 30 * time.Second
	ActivityMaxItems   = 50
	ActivitySummaryMax = 160 // rune
)

// activityOwner adalah pemilik sesi: thread dan nama tampilnya.
type activityOwner struct{ threadID, name string }

// activityTail adalah pesan activity terakhir per thread yang masih bisa digabung.
type activityTail struct {
	sessionID string
	id        string
	count     int
	items     []string
	lastTS    time.Time
}

type activityJob struct {
	threadID string
	msg      ThreadMessage
}

type activityQueue struct {
	jobs    []activityJob
	running bool
}

type activityDaemon struct {
	m       *Manager
	log     *slog.Logger
	mu      sync.Mutex
	owners  map[string]activityOwner
	tails   map[string]*activityTail
	queues  map[string]*activityQueue
	stopped bool
}

// InstallActivity memasang penurun pesan activity.
func InstallActivity(m *Manager, src ActivitySource) (stop func()) {
	if m == nil || src == nil {
		return func() {}
	}
	d := &activityDaemon{
		m:      m,
		log:    slog.Default(),
		owners: map[string]activityOwner{},
		tails:  map[string]*activityTail{},
		queues: map[string]*activityQueue{},
	}
	d.seed()
	m.OnStart(func(*Manager) { d.guard("OnStart", d.seed) })
	m.OnParticipantAdded(func(t Thread, p Participant) {
		d.guard("OnParticipantAdded", func() {
			if t.State != "" && t.State != StateActive {
				return
			}
			d.addOwner(t.ID, p)
		})
	})
	m.OnClose(func(t Thread) { d.guard("OnClose", func() { d.removeThread(t.ID) }) })
	m.OnMessage(func(t Thread, _ ThreadMessage) { d.guard("OnMessage", func() { d.breakTail(t.ID) }) })

	cancel := src.AddHookListener(d.onHook)
	var once sync.Once
	return func() {
		once.Do(func() {
			d.mu.Lock()
			d.stopped = true
			d.owners = map[string]activityOwner{}
			d.tails = map[string]*activityTail{}
			d.queues = map[string]*activityQueue{}
			d.mu.Unlock()
			if cancel != nil {
				cancel()
			}
		})
	}
}

// guard menjalankan fn dengan recover; no-op bila daemon sudah dihentikan.
func (d *activityDaemon) guard(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			d.log.Error("thread: panic pada callback activity", "callback", name, "panic", r)
		}
	}()
	d.mu.Lock()
	stopped := d.stopped
	d.mu.Unlock()
	if stopped {
		return
	}
	fn()
}

func activityName(p Participant) string {
	if n := strings.TrimSpace(p.Name); n != "" {
		return n
	}
	if p.Role == RoleMaster {
		return "master"
	}
	r := []rune(p.SessionID)
	if len(r) > 8 {
		r = r[:8]
	}
	return string(r)
}

func (d *activityDaemon) seed() {
	threads := d.m.ActiveThreads()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	for _, t := range threads {
		for _, p := range t.Participants {
			if p.SessionID != "" {
				d.owners[p.SessionID] = activityOwner{t.ID, activityName(p)}
			}
		}
	}
}

func (d *activityDaemon) addOwner(threadID string, p Participant) {
	if p.SessionID == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	d.owners[p.SessionID] = activityOwner{threadID, activityName(p)}
}

func (d *activityDaemon) removeThread(threadID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for sid, o := range d.owners {
		if o.threadID == threadID {
			delete(d.owners, sid)
		}
	}
	delete(d.tails, threadID)
	delete(d.queues, threadID)
}

func (d *activityDaemon) breakTail(threadID string) {
	d.mu.Lock()
	delete(d.tails, threadID)
	d.mu.Unlock()
}

// activityTarget memilih target ringkas dari input tool.
func activityTarget(tool string, input map[string]any) string {
	if input == nil {
		return ""
	}
	str := func(k string) string {
		s, _ := input[k].(string)
		return s
	}
	var keys []string
	switch tool {
	case "Bash":
		keys = []string{"command"}
	case "Read", "Edit", "Write":
		keys = []string{"file_path"}
	case "NotebookEdit":
		keys = []string{"file_path", "notebook_path"}
	case "Grep", "Glob":
		keys = []string{"pattern"}
	case "WebFetch":
		keys = []string{"url"}
	case "Task":
		keys = []string{"description"}
	}
	for _, k := range keys {
		if s := str(k); s != "" {
			return s
		}
	}
	if len(keys) > 0 {
		return ""
	}
	all := make([]string, 0, len(input))
	for k := range input {
		all = append(all, k)
	}
	sort.Strings(all)
	for _, k := range all {
		if s, ok := input[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func activityIsPathTool(tool string) bool {
	switch tool {
	case "Read", "Edit", "Write", "NotebookEdit":
		return true
	}
	return false
}

// activityRelPath mengubah path absolut di bawah threadDir menjadi relatif worktree.
func activityRelPath(threadDir, target string) string {
	if !filepath.IsAbs(target) || threadDir == "" {
		return target
	}
	prefix := strings.TrimRight(threadDir, string(filepath.Separator)) + string(filepath.Separator)
	if !strings.HasPrefix(target, prefix) {
		return target
	}
	rest := target[len(prefix):]
	if i := strings.IndexRune(rest, filepath.Separator); i >= 0 && i+1 < len(rest) {
		return rest[i+1:]
	}
	return rest
}

func activityOneLine(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func activityTruncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func activityItem(tool, target string) string {
	return activityTruncate(activityOneLine(strings.TrimSpace(tool+" "+target)), ActivitySummaryMax)
}

func activityText(name, item string) string {
	name = activityOneLine(name)
	prefix := name + ": "
	pr := len([]rune(prefix))
	if pr >= ActivitySummaryMax {
		return activityTruncate(prefix+item, ActivitySummaryMax)
	}
	return prefix + activityTruncate(item, ActivitySummaryMax-pr)
}

// onHook memproses event hook status; harus cepat dan tidak memblok.
func (d *activityDaemon) onHook(sessionID, event string, payload map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			d.log.Error("thread: panic pada pemroses hook activity", "panic", r)
		}
	}()
	if event != "PostToolUse" || sessionID == "" {
		return
	}
	tool, _ := payload["tool_name"].(string)
	if tool == "" {
		return
	}
	d.mu.Lock()
	owner, ok := d.owners[sessionID]
	stopped := d.stopped
	d.mu.Unlock()
	if stopped || !ok {
		return
	}
	input, _ := payload["tool_input"].(map[string]any)
	target := activityOneLine(activityTarget(tool, input))
	if activityIsPathTool(tool) {
		target = activityRelPath(d.m.Store().ThreadDir(owner.threadID), activityTarget(tool, input))
	}
	item := activityItem(tool, target)
	text := activityText(owner.name, item)
	now := time.Now().UTC()

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	if cur, ok := d.owners[sessionID]; !ok || cur.threadID != owner.threadID {
		return
	}
	tail := d.tails[owner.threadID]
	if tail != nil && tail.sessionID == sessionID && now.Sub(tail.lastTS) < ActivityMergeGap {
		tail.count++
		tail.items = append(tail.items, item)
		if len(tail.items) > ActivityMaxItems {
			tail.items = tail.items[len(tail.items)-ActivityMaxItems:]
		}
		tail.lastTS = now
	} else {
		tail = &activityTail{sessionID: sessionID, id: NewID(), count: 1, items: []string{item}, lastTS: now}
		d.tails[owner.threadID] = tail
	}
	msg := ThreadMessage{
		ID:       tail.id,
		ThreadID: owner.threadID,
		From:     sessionID,
		To:       ToAll,
		Text:     text,
		TS:       tail.lastTS,
		Kind:     KindActivity,
		Count:    tail.count,
		Items:    append([]string(nil), tail.items...),
	}
	d.enqueue(owner.threadID, msg)
}

// enqueue dipanggil dengan d.mu terkunci.
func (d *activityDaemon) enqueue(threadID string, msg ThreadMessage) {
	q := d.queues[threadID]
	if q == nil {
		q = &activityQueue{}
		d.queues[threadID] = q
	}
	q.jobs = append(q.jobs, activityJob{threadID, msg})
	if !q.running {
		q.running = true
		go d.drain(threadID, q)
	}
}

func (d *activityDaemon) drain(threadID string, q *activityQueue) {
	defer func() {
		if r := recover(); r != nil {
			d.log.Error("thread: panic pada antrean activity", "thread", threadID, "panic", r)
			d.mu.Lock()
			q.running = false
			restart := !d.stopped && d.queues[threadID] == q && len(q.jobs) > 0
			if restart {
				q.running = true
			}
			d.mu.Unlock()
			if restart {
				go d.drain(threadID, q)
			}
		}
	}()
	for {
		d.mu.Lock()
		if d.stopped || d.queues[threadID] != q || len(q.jobs) == 0 {
			q.running = false
			d.mu.Unlock()
			return
		}
		// Buang job lama ber-id sama yang langsung diikuti job terbaru.
		for len(q.jobs) > 1 && q.jobs[0].msg.ID == q.jobs[1].msg.ID {
			q.jobs = q.jobs[1:]
		}
		job := q.jobs[0]
		q.jobs = q.jobs[1:]
		d.mu.Unlock()
		d.write(job)
	}
}

func (d *activityDaemon) write(job activityJob) {
	defer func() {
		if r := recover(); r != nil {
			d.log.Error("thread: panic saat menyimpan pesan activity", "thread", job.threadID, "panic", r)
		}
	}()
	_, err := d.m.UpsertMessage(job.threadID, job.msg)
	if err == nil {
		return
	}
	if errors.Is(err, ErrClosed) {
		d.removeThread(job.threadID)
		return
	}
	d.log.Warn("thread: gagal menyimpan pesan activity", "thread", job.threadID, "err", err)
}
