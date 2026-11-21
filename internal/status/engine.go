package status

import (
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aiterm/internal/ipc"
	"aiterm/internal/paths"
	"aiterm/internal/provider"
	"aiterm/internal/session"
)

var ErrUnknownSession = errors.New("sesi tidak dikenal")

// SessionSource dipenuhi *session.Manager; dipasang malas (lihat Wiring).
type SessionSource interface {
	Get(id string) (*session.Session, error)
	List() []session.Session
}

// maxPending membatasi launch tertunda agar tidak bocor bila sesi gagal dibuat.
const maxPending = 256

type sessionState struct {
	id         string
	providerID string
	token      string // launch token terikat; "" bila tidak terikat
	status     Status
	since      time.Time
	hookSeen   bool        // sudah ada hook pertama
	timer      *time.Timer // nil sampai output pertama
}

// Engine adalah state machine status per sesi. Event-driven: tanpa ticker,
// tanpa loop tidur-bangun; hanya timer sekali jalan setelah output.
type Engine struct {
	mu       sync.Mutex
	srv      *ipc.Server
	src      SessionSource
	states   map[string]*sessionState // kunci: ID sesi asli
	pending  []LaunchInfo             // launch belum terikat, urut At (FIFO)
	byToken  map[string]string        // token -> ID sesi asli
	subs     map[int]func(StatusEvent)
	nextSub  int
	waiters  map[string]map[int]func() // OnIdle: id -> waiter
	nextWait int

	streamMu sync.Mutex // melindungi ensureStream
	stream   *ipc.Client
	unsubs   []func()
}

var defaultEngine atomic.Pointer[Engine]

// NewEngine membuat engine; tidak ada goroutine dan tidak ada Dial.
func NewEngine(srv *ipc.Server) *Engine {
	return &Engine{
		srv:     srv,
		states:  map[string]*sessionState{},
		byToken: map[string]string{},
		subs:    map[int]func(StatusEvent){},
		waiters: map[string]map[int]func(){},
	}
}

func SetDefault(e *Engine) { defaultEngine.Store(e) }

func Default() *Engine { return defaultEngine.Load() }

func (e *Engine) SetSessionSource(src SessionSource) {
	e.mu.Lock()
	e.src = src
	e.mu.Unlock()
}

// ensureStream membuka (malas) klien IPC internal ke socket daemon sendiri untuk
// menerima session.output.* dan session.exit. Idempoten, tanpa loop retry: bila
// Dial gagal, dicoba lagi pada panggilan berikutnya. Sesi hanya bisa dibuat via
// session.create sehingga socket sudah hidup dan stream tersambung sebelum output
// pertama. Callback berjalan di dispatch loop klien: jangan blok dan jangan
// memanggil Call sinkron dari dalam callback.
func (e *Engine) ensureStream() error {
	e.streamMu.Lock()
	defer e.streamMu.Unlock()
	if e.stream != nil {
		return nil
	}
	sock, err := paths.SocketPath()
	if err != nil {
		return err
	}
	c, err := ipc.Dial(sock)
	if err != nil {
		return err
	}
	unsubOut, err := c.Subscribe("session.output.", func(topic string, _ json.RawMessage) {
		e.OnOutput(strings.TrimPrefix(topic, "session.output."))
	})
	if err != nil {
		_ = c.Close()
		return err
	}
	unsubExit, err := c.Subscribe("session.exit", func(_ string, payload json.RawMessage) {
		var p struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(payload, &p) != nil || p.ID == "" {
			return
		}
		e.handleExit(p.ID)
	})
	if err != nil {
		unsubOut()
		_ = c.Close()
		return err
	}
	e.stream = c
	e.unsubs = []func(){unsubOut, unsubExit}
	return nil
}

func (e *Engine) RegisterLaunch(info LaunchInfo) {
	e.mu.Lock()
	e.pending = append(e.pending, info)
	if len(e.pending) > maxPending {
		e.pending = e.pending[len(e.pending)-maxPending:]
	}
	e.mu.Unlock()
	if err := e.ensureStream(); err != nil {
		slog.Debug("status: stream belum tersambung", "err", err)
	}
}

// bindLocked membuat state untuk ID sesi asli dan mengikat launch token secara malas.
// Dipanggil di bawah e.mu. Keterbatasan (T1): dua sesi serentak dengan spec identik
// bisa tertukar ikatannya; dampaknya hanya status tertukar antar dua sesi yang sama.
func (e *Engine) bindLocked(sessionID string) *sessionState {
	if st, ok := e.states[sessionID]; ok {
		return st
	}
	if e.src == nil {
		return nil
	}
	s, err := e.src.Get(sessionID)
	if err != nil || s == nil {
		return nil
	}
	now := time.Now()
	st := &sessionState{id: sessionID, providerID: s.ProviderID, status: StatusIdle, since: now}
	for i, info := range e.pending {
		if info.ProviderID == s.ProviderID && info.Cwd == s.Cwd &&
			info.ProjectID == s.ProjectID && info.WorkspaceID == s.WorkspaceID {
			st.token = info.Token
			e.byToken[info.Token] = sessionID
			e.pending = append(e.pending[:i], e.pending[i+1:]...)
			break
		}
	}
	e.states[sessionID] = st
	return st
}

func (e *Engine) Track(sessionID string) bool {
	e.mu.Lock()
	st := e.bindLocked(sessionID)
	e.mu.Unlock()
	_ = e.ensureStream()
	return st != nil
}

func (e *Engine) ResolveSessionID(idOrToken string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if id, ok := e.byToken[idOrToken]; ok {
		return id, true
	}
	if _, ok := e.states[idOrToken]; ok {
		return idOrToken, true
	}
	if e.bindLocked(idOrToken) != nil {
		return idOrToken, true
	}
	isPending := false
	for _, info := range e.pending {
		if info.Token == idOrToken {
			isPending = true
			break
		}
	}
	if !isPending || e.src == nil {
		return "", false
	}
	for _, s := range e.src.List() {
		if s.Status != "running" {
			continue
		}
		if _, ok := e.states[s.ID]; ok {
			continue
		}
		e.bindLocked(s.ID)
		if id, ok := e.byToken[idOrToken]; ok {
			return id, true
		}
	}
	return "", false
}

func isIdleStatus(s Status) bool {
	return s == StatusIdle || s == StatusDone || s == StatusWaiting
}

func (e *Engine) Set(sessionID string, st Status, source, reason string) bool {
	e.mu.Lock()
	state := e.bindLocked(sessionID)
	if state == nil || state.status == st {
		e.mu.Unlock()
		return false
	}
	prev := state.status
	now := time.Now()
	state.status = st
	state.since = now
	if st == StatusDone {
		stopTimerLocked(state)
	}
	ev := StatusEvent{SessionID: state.id, Status: st, Prev: prev, Source: source, Reason: reason, At: now.UnixMilli()}
	subs := make([]func(StatusEvent), 0, len(e.subs))
	for _, fn := range e.subs {
		subs = append(subs, fn)
	}
	var wake []func()
	if isIdleStatus(st) {
		for _, fn := range e.waiters[state.id] {
			wake = append(wake, fn)
		}
		delete(e.waiters, state.id)
	}
	srv := e.srv
	e.mu.Unlock()

	if srv != nil {
		srv.Publish("session.status", ev)
	}
	for _, fn := range subs {
		callSub(fn, ev)
	}
	for _, fn := range wake {
		go fn()
	}
	return true
}

func callSub(fn func(StatusEvent), ev StatusEvent) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("status: subscriber panic", "err", r)
		}
	}()
	fn(ev)
}

func (e *Engine) Apply(idOrToken, event string, payload map[string]any) error {
	id, ok := e.ResolveSessionID(idOrToken)
	if !ok {
		return ErrUnknownSession
	}
	st, known := ClassifyEvent(event)
	if !known {
		return nil
	}
	e.mu.Lock()
	if state := e.states[id]; state != nil {
		state.hookSeen = true
		if hookCapable(state) {
			stopTimerLocked(state)
		}
	}
	e.mu.Unlock()
	e.Set(id, st, SourceHook, event)
	return nil
}

// hookCapable: provider mendukung hooks DAN sesi terikat ke launch token
// (hooks terpasang). Dipanggil di bawah lock.
func hookCapable(state *sessionState) bool {
	if state.token == "" {
		return false
	}
	p, ok := provider.Get(state.providerID)
	return ok && p.Capabilities().Hooks
}

func (e *Engine) handleExit(id string) {
	e.Set(id, StatusDone, SourceExit, "exit")
	e.Forget(id)
}

func (e *Engine) Get(sessionID string) (SessionState, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := e.bindLocked(sessionID)
	if st == nil {
		return SessionState{}, false
	}
	return SessionState{SessionID: st.id, Status: st.status, Since: st.since.UnixMilli()}, true
}

func (e *Engine) List() []SessionState {
	e.mu.Lock()
	if e.src != nil {
		for _, s := range e.src.List() {
			if s.Status == "running" {
				e.bindLocked(s.ID)
			}
		}
	}
	type row struct {
		s     SessionState
		since time.Time
	}
	rows := make([]row, 0, len(e.states))
	for _, st := range e.states {
		rows = append(rows, row{SessionState{SessionID: st.id, Status: st.status, Since: st.since.UnixMilli()}, st.since})
	}
	e.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].since.Equal(rows[j].since) {
			return rows[i].since.Before(rows[j].since)
		}
		return rows[i].s.SessionID < rows[j].s.SessionID
	})
	out := make([]SessionState, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.s)
	}
	return out
}

func (e *Engine) Lookup(sessionID string) (session.Session, bool) {
	e.mu.Lock()
	src := e.src
	e.mu.Unlock()
	if src == nil {
		return session.Session{}, false
	}
	s, err := src.Get(sessionID)
	if err != nil || s == nil {
		return session.Session{}, false
	}
	return *s, true
}

func (e *Engine) Subscribe(fn func(StatusEvent)) (cancel func()) {
	e.mu.Lock()
	id := e.nextSub
	e.nextSub++
	e.subs[id] = fn
	e.mu.Unlock()
	return func() {
		e.mu.Lock()
		delete(e.subs, id)
		e.mu.Unlock()
	}
}

// Forget membuang state sesi. Bila dipanggil karena exit, subscriber sudah
// menerima event done dari handleExit lebih dulu.
func (e *Engine) Forget(sessionID string) {
	e.mu.Lock()
	st := e.states[sessionID]
	token := ""
	if st != nil {
		stopTimerLocked(st)
		delete(e.states, sessionID)
		token = st.token
		if token != "" {
			delete(e.byToken, token)
		}
	}
	delete(e.waiters, sessionID)
	e.mu.Unlock()
	if token != "" {
		CleanupHooks(token)
	}
}

// RegisterLaunch meneruskan ke engine bawaan (no-op bila belum di-set).
func RegisterLaunch(info LaunchInfo) {
	if e := Default(); e != nil {
		e.RegisterLaunch(info)
	}
}
