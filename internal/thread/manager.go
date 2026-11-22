package thread

import (
	"context"
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

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/mention"
	"github.com/NooBeeID/shikigami/internal/session"
)

// ErrClosed dikembalikan bila operasi menyentuh thread yang sudah ditutup.
var ErrClosed = errors.New("thread sudah ditutup")

// ProjectInfo ringkasan satu project di workspace.
type ProjectInfo struct{ ID, Name, Path string }

// WorkspaceSource menyediakan daftar project workspace.
type WorkspaceSource interface {
	// ok=false: workspace tak dikenal / daemon workspace tak terjangkau.
	Projects(workspaceID string) (projects []ProjectInfo, ok bool)
}

// Publisher menerbitkan event ke pelanggan IPC (srv.Publish).
type Publisher func(topic string, payload any)

// Sender dipenuhi *mention.Queue (instance khusus thread).
type Sender interface {
	Enqueue(ctx context.Context, from, to, text, source string) (mention.Message, error)
}

// SessionAPI dipenuhi *session.Manager.
type SessionAPI interface {
	Create(spec session.SessionSpec) (*session.Session, error)
	Get(id string) (*session.Session, error)
	Kill(id string) error
	List() []session.Session
}

var (
	_ SessionAPI = (*session.Manager)(nil)
	_ Sender     = (*mention.Queue)(nil)
)

// ManagerConfig konfigurasi Manager.
type ManagerConfig struct {
	Store      *Store
	Sessions   SessionAPI
	Sender     Sender
	Workspaces WorkspaceSource
	Namer      *mention.Namer // untuk Participant.Name; boleh nil -> "master"
	Publish    Publisher
	Logger     *slog.Logger
	Agents     AgentResolver // opsional; nil = perilaku lama
}

// ThreadCaller identitas pemanggil tool thread.
type ThreadCaller struct{ ThreadID, SessionID, Role string }

// ToolSpec mendefinisikan satu tool thread (dipakai tiket thread-agents).
type ToolSpec struct {
	Name, Description string
	MasterOnly        bool
	Handler           func(ctx context.Context, caller ThreadCaller, args map[string]any) (any, error)
}

// ToolRegistry kumpulan tool thread; tiket thread-agents mendaftarkan tool lewat Register.
type ToolRegistry struct {
	mu    sync.Mutex
	tools map[string]ToolSpec
}

// Register mendaftarkan tool. Nama kosong atau duplikat menyebabkan panic.
func (r *ToolRegistry) Register(name string, tool ToolSpec) {
	if strings.TrimSpace(name) == "" {
		panic("thread: nama tool kosong")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tools == nil {
		r.tools = map[string]ToolSpec{}
	}
	if _, dup := r.tools[name]; dup {
		panic(fmt.Sprintf("thread: tool %q sudah terdaftar", name))
	}
	tool.Name = name
	r.tools[name] = tool
}

// Tools mengembalikan salinan tool terurut nama (selalu non-nil).
func (r *ToolRegistry) Tools() []ToolSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Manager mengelola thread: pesan, peserta, master, hook.
type Manager struct {
	cfg   ManagerConfig
	tools *ToolRegistry
	log   *slog.Logger

	hookMu             sync.Mutex
	onStart            []func(*Manager)
	onMessage          []func(Thread, ThreadMessage)
	onParticipantAdded []func(Thread, Participant)
	onClose            []func(Thread)
	preSend            PreSendHook

	lockMu sync.Mutex
	locks  map[string]*sync.Mutex // mutex per thread untuk baca-ubah-simpan

	methodMu sync.Mutex // dipakai ipc.go (RegisterMethod)
	methods  map[string]func(json.RawMessage) (any, error)
	srv      *ipc.Server
}

// NewManager membuat Manager.
func NewManager(cfg ManagerConfig) *Manager {
	lg := cfg.Logger
	if lg == nil {
		lg = slog.Default()
	}
	return &Manager{
		cfg:     cfg,
		tools:   &ToolRegistry{tools: map[string]ToolSpec{}},
		log:     lg,
		locks:   map[string]*sync.Mutex{},
		methods: map[string]func(json.RawMessage) (any, error){},
	}
}

// Tools mengembalikan registri tool thread.
func (m *Manager) Tools() *ToolRegistry { return m.tools }

// Store mengembalikan store thread.
func (m *Manager) Store() *Store { return m.cfg.Store }

func (m *Manager) managerLock(id string) *sync.Mutex {
	m.lockMu.Lock()
	defer m.lockMu.Unlock()
	l, ok := m.locks[id]
	if !ok {
		l = &sync.Mutex{}
		m.locks[id] = l
	}
	return l
}

func (m *Manager) managerPublish(topic string, payload any) {
	if m.cfg.Publish == nil {
		return
	}
	m.cfg.Publish(topic, payload)
}

func managerNormalize(t Thread) Thread {
	if t.Participants == nil {
		t.Participants = []Participant{}
	}
	return t
}

func (m *Manager) managerPublishThread(t Thread) {
	m.managerPublish(EventUpdated, managerNormalize(t))
}

// Hook: boleh didaftarkan banyak pihak, dipanggil berurutan pendaftaran di
// goroutine pemanggil; panic di hook dipulihkan dan dicatat.

// OnStart mendaftarkan hook yang dipanggil oleh FireStart.
func (m *Manager) OnStart(fn func(*Manager)) {
	m.hookMu.Lock()
	m.onStart = append(m.onStart, fn)
	m.hookMu.Unlock()
}

// OnMessage mendaftarkan hook untuk setiap pesan yang tercatat.
func (m *Manager) OnMessage(fn func(Thread, ThreadMessage)) {
	m.hookMu.Lock()
	m.onMessage = append(m.onMessage, fn)
	m.hookMu.Unlock()
}

// OnParticipantAdded mendaftarkan hook untuk peserta baru.
func (m *Manager) OnParticipantAdded(fn func(Thread, Participant)) {
	m.hookMu.Lock()
	m.onParticipantAdded = append(m.onParticipantAdded, fn)
	m.hookMu.Unlock()
}

// OnClose mendaftarkan hook penutupan thread.
func (m *Manager) OnClose(fn func(Thread)) {
	m.hookMu.Lock()
	m.onClose = append(m.onClose, fn)
	m.hookMu.Unlock()
}

func (m *Manager) managerSafe(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			m.log.Error("thread: hook panic", "hook", name, "panic", r)
		}
	}()
	fn()
}

// FireStart dipanggil wire_thread.go setelah semua metode terdaftar.
func (m *Manager) FireStart() {
	m.hookMu.Lock()
	hooks := append([]func(*Manager){}, m.onStart...)
	m.hookMu.Unlock()
	for _, h := range hooks {
		h := h
		m.managerSafe("start", func() { h(m) })
	}
}

// FireClose dipanggil tiket thread-close.
func (m *Manager) FireClose(t Thread) {
	m.hookMu.Lock()
	hooks := append([]func(Thread){}, m.onClose...)
	m.hookMu.Unlock()
	for _, h := range hooks {
		h := h
		m.managerSafe("close", func() { h(t) })
	}
}

func (m *Manager) managerFireMessage(t Thread, msg ThreadMessage) {
	m.hookMu.Lock()
	hooks := append([]func(Thread, ThreadMessage){}, m.onMessage...)
	m.hookMu.Unlock()
	for _, h := range hooks {
		h := h
		m.managerSafe("message", func() { h(t, msg) })
	}
}

func (m *Manager) managerFireParticipant(t Thread, p Participant) {
	m.hookMu.Lock()
	hooks := append([]func(Thread, Participant){}, m.onParticipantAdded...)
	m.hookMu.Unlock()
	for _, h := range hooks {
		h := h
		m.managerSafe("participant", func() { h(t, p) })
	}
}

// Get mengambil thread.
func (m *Manager) Get(id string) (Thread, error) {
	t, err := m.cfg.Store.Get(id)
	if err != nil {
		return Thread{}, err
	}
	return managerNormalize(t), nil
}

// List mengembalikan thread suatu workspace (selalu non-nil).
func (m *Manager) List(workspaceID string) ([]Thread, error) {
	list, err := m.cfg.Store.List(workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Thread, 0, len(list))
	for _, t := range list {
		out = append(out, managerNormalize(t))
	}
	return out, nil
}

// Messages mengembalikan pesan thread sejak sinceTS (selalu non-nil).
func (m *Manager) Messages(id string, sinceTS time.Time) ([]ThreadMessage, error) {
	if _, err := m.Get(id); err != nil {
		return nil, err
	}
	msgs, err := m.cfg.Store.Messages(id, sinceTS)
	if err != nil {
		return nil, err
	}
	if msgs == nil {
		msgs = []ThreadMessage{}
	}
	return msgs, nil
}

// Post mencatat pesan ke log thread, mempublikasikan, lalu memicu hook.
// Ditolak bila thread sudah closed.
func (m *Manager) Post(threadID string, msg ThreadMessage) (ThreadMessage, error) {
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
	if err := m.cfg.Store.AppendMessage(msg); err != nil {
		return ThreadMessage{}, err
	}
	m.managerPublish(EventMessage, msg)
	m.managerFireMessage(t, msg)
	return msg, nil
}

// AppendMessage alias Post.
func (m *Manager) AppendMessage(threadID string, msg ThreadMessage) (ThreadMessage, error) {
	return m.Post(threadID, msg)
}

// PostSystem mencatat pesan sistem; tidak menolak thread closed, kegagalan hanya dilog.
func (m *Manager) PostSystem(threadID, text string) {
	t, err := m.Get(threadID)
	if err != nil {
		m.log.Warn("thread: pesan sistem gagal (thread tidak terbaca)", "thread", threadID, "err", err)
		return
	}
	msg := ThreadMessage{
		ID:       NewID(),
		ThreadID: threadID,
		From:     FromSystem,
		To:       ToAll,
		Text:     text,
		TS:       time.Now().UTC(),
		Kind:     KindSystem,
	}
	if err := m.cfg.Store.AppendMessage(msg); err != nil {
		m.log.Warn("thread: pesan sistem gagal dicatat", "thread", threadID, "err", err)
		return
	}
	m.managerPublish(EventMessage, msg)
	m.managerFireMessage(t, msg)
}

// UpdateThread baca-ubah-simpan di bawah mutex thread lalu publish thread.updated.
func (m *Manager) UpdateThread(threadID string, fn func(*Thread)) (Thread, error) {
	l := m.managerLock(threadID)
	l.Lock()
	t, err := m.cfg.Store.Get(threadID)
	if err != nil {
		l.Unlock()
		return Thread{}, err
	}
	id0 := t.ID
	fn(&t)
	t.ID = id0
	t = managerNormalize(t)
	if err := m.cfg.Store.Save(t); err != nil {
		l.Unlock()
		return Thread{}, err
	}
	l.Unlock()
	m.managerPublishThread(t)
	return t, nil
}

// AddParticipant menambah/mengganti peserta (idempoten per SessionID).
func (m *Manager) AddParticipant(threadID string, p Participant) (Thread, error) {
	if p.SessionID == "" || (p.Role != RoleMaster && p.Role != RolePeer) {
		return Thread{}, fmt.Errorf("%w: peserta tidak valid", ErrInvalid)
	}
	cur, err := m.Get(threadID)
	if err != nil {
		return Thread{}, err
	}
	if cur.State == StateClosed {
		return Thread{}, ErrClosed
	}
	t, err := m.UpdateThread(threadID, func(t *Thread) {
		for i := range t.Participants {
			if t.Participants[i].SessionID == p.SessionID {
				t.Participants[i] = p
				return
			}
		}
		t.Participants = append(t.Participants, p)
	})
	if err != nil {
		return Thread{}, err
	}
	m.managerFireParticipant(t, p)
	return t, nil
}

// SendToSession mengirim teks lewat antrean thread. from "" = user, selain itu sessionId agent.
// Pencatatan ke log thread dilakukan pemanggil (Send, router thread-agents), bukan di sini.
func (m *Manager) SendToSession(threadID, from, to, text string) (mention.Message, error) {
	if strings.TrimSpace(text) == "" {
		return mention.Message{}, fmt.Errorf("%w: teks kosong", ErrInvalid)
	}
	t, err := m.Get(threadID)
	if err != nil {
		return mention.Message{}, err
	}
	if t.State != StateActive {
		return mention.Message{}, ErrClosed
	}
	if m.cfg.Sender == nil {
		return mention.Message{}, errors.New("antrean pesan thread tidak tersedia")
	}
	source := mention.SourceUser
	if from != "" {
		source = mention.SourceAgent
	}
	return m.cfg.Sender.Enqueue(context.Background(), from, to, text, source)
}

func managerMaster(t Thread) (Participant, bool) {
	for _, p := range t.Participants {
		if p.Role == RoleMaster {
			return p, true
		}
	}
	return Participant{}, false
}

// Send menangani pesan user ke master (thread.send).
func (m *Manager) Send(threadID, text string) (ThreadMessage, error) {
	if strings.TrimSpace(text) == "" {
		return ThreadMessage{}, fmt.Errorf("%w: teks kosong", ErrInvalid)
	}
	t, err := m.Get(threadID)
	if err != nil {
		return ThreadMessage{}, err
	}
	if t.State == StateClosed {
		return ThreadMessage{}, ErrClosed
	}
	master, hasMaster := managerMaster(t)
	if hasMaster {
		if rep := m.runPreSend(threadID, []string{master.SessionID}); rep[master.SessionID] != "" {
			master.SessionID = rep[master.SessionID]
		}
	}
	to := ToAll
	if hasMaster {
		to = master.SessionID
	}
	rec, err := m.Post(threadID, ThreadMessage{From: FromUser, To: to, Text: text, Kind: KindMessage})
	if err != nil {
		return ThreadMessage{}, err
	}
	if !hasMaster {
		m.PostSystem(threadID, "gagal meneruskan ke master: master belum ada")
		return rec, nil
	}
	if _, err := m.SendToSession(threadID, "", master.SessionID, text); err != nil {
		m.PostSystem(threadID, "gagal meneruskan ke master: "+err.Error())
	}
	return rec, nil
}

func (m *Manager) managerAllThreads() []Thread {
	root := filepath.Dir(m.cfg.Store.ThreadDir("x"))
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []Thread
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := m.cfg.Store.Get(e.Name())
		if err != nil {
			m.log.Debug("thread: lewati thread tak terbaca", "id", e.Name(), "err", err)
			continue
		}
		out = append(out, t)
	}
	return out
}

// OwnedSessions mengembalikan sesi milik semua thread aktif (unik, terurut, non-nil).
func (m *Manager) OwnedSessions() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range m.managerAllThreads() {
		if t.State != StateActive {
			continue
		}
		for _, p := range t.Participants {
			if p.SessionID != "" && !seen[p.SessionID] {
				seen[p.SessionID] = true
				out = append(out, p.SessionID)
			}
		}
	}
	sort.Strings(out)
	return out
}

func managerDeriveTitle(goal string) string {
	for _, line := range strings.Split(goal, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		r := []rune(line)
		if len(r) > 60 {
			return strings.TrimSpace(string(r[:60])) + "…"
		}
		return line
	}
	return "Thread baru"
}

func (m *Manager) managerSpawnSafe(t Thread) (p Participant, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic saat membuat master: %v", r)
		}
	}()
	return m.SpawnMaster(t)
}

// Create membuat thread dan men-spawn master. Kegagalan spawn bukan galat Create.
func (m *Manager) Create(workspaceID, goal, title string) (Thread, error) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return Thread{}, fmt.Errorf("%w: goal tidak boleh kosong", ErrInvalid)
	}
	if workspaceID == "" {
		return Thread{}, fmt.Errorf("%w: workspaceId kosong", ErrInvalid)
	}
	if m.cfg.Workspaces == nil {
		return Thread{}, fmt.Errorf("%w: workspace tidak dikenal", ErrInvalid)
	}
	if _, ok := m.cfg.Workspaces.Projects(workspaceID); !ok {
		return Thread{}, fmt.Errorf("%w: workspace tidak dikenal", ErrInvalid)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = managerDeriveTitle(goal)
	}
	t, err := m.cfg.Store.Create(workspaceID, title, goal)
	if err != nil {
		return Thread{}, fmt.Errorf("buat thread: %w", err)
	}
	m.managerPublishThread(t)
	p, err := m.managerSpawnSafe(t)
	if err != nil {
		m.log.Warn("thread: gagal membuat master", "thread", t.ID, "err", err)
		m.PostSystem(t.ID, "gagal membuat master: "+err.Error())
	} else {
		m.log.Info("thread: master dibuat", "thread", t.ID, "session", p.SessionID)
		m.PostSystem(t.ID, "master dibuat")
	}
	t2, err := m.Get(t.ID)
	if err != nil {
		return managerNormalize(t), nil
	}
	return t2, nil
}
