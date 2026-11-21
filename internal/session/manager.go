package session

import (
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/NooBeeID/shikigami/internal/session/pty"
)

const (
	readBufSize     = 32 * 1024
	maxExitedKept   = 64
	killGracePeriod = 2 * time.Second
	defaultCols     = 80
	defaultRows     = 24
	defaultProvider = "shell"
)

type entry struct {
	mu      sync.Mutex // melindungi sess dan size
	sess    Session
	size    pty.Size
	p       pty.PTY
	sb      *scrollback
	wmu     sync.Mutex    // serialisasi p.Write
	done    chan struct{} // ditutup pembaca setelah exit dipublikasikan
	exitSeq uint64        // 0 = belum exit
}

// Manager mengelola banyak sesi PTY.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*entry
	launcher Launcher
	pub      func(topic string, payload any)
	exitSeq  uint64
	closed   bool
}

type outputPayload struct {
	Data   string `json:"data"`
	Offset uint64 `json:"offset"`
}

type exitPayload struct {
	ID       string `json:"id"`
	ExitCode int    `json:"exitCode"`
}

// NewManager membuat Manager; pub nil menjadi no-op.
func NewManager(pub func(topic string, payload any)) *Manager {
	if pub == nil {
		pub = func(string, any) {}
	}
	return &Manager{
		sessions: make(map[string]*entry),
		launcher: DefaultLauncher(),
		pub:      pub,
	}
}

// SetLauncher mengganti launcher; nil mengembalikan default.
func (m *Manager) SetLauncher(l Launcher) {
	if l == nil {
		l = DefaultLauncher()
	}
	m.mu.Lock()
	m.launcher = l
	m.mu.Unlock()
}

func snapshotSession(e *entry) Session {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sess
	if e.sess.ExitCode != nil {
		c := *e.sess.ExitCode
		s.ExitCode = &c
	}
	return s
}

func (m *Manager) lookup(id string) (*entry, error) {
	m.mu.RLock()
	e, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	return e, nil
}

func (e *entry) running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sess.Status == StatusRunning
}

// Create membuat sesi baru.
func (m *Manager) Create(spec SessionSpec) (*Session, error) {
	m.mu.RLock()
	closed := m.closed
	l := m.launcher
	m.mu.RUnlock()
	if closed {
		return nil, fmt.Errorf("%w: manajer sudah dihentikan", ErrInvalid)
	}
	if spec.Cols == 0 {
		spec.Cols = defaultCols
	}
	if spec.Rows == 0 {
		spec.Rows = defaultRows
	}
	if spec.ProviderID == "" {
		spec.ProviderID = defaultProvider
	}
	if spec.Cwd == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("%w: tidak dapat menentukan direktori home: %v", ErrInvalid, err)
		}
		spec.Cwd = home
	}
	st, err := os.Stat(spec.Cwd)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("%w: cwd %q tidak ada atau bukan direktori", ErrInvalid, spec.Cwd)
	}

	cmd, args, env := l.Command(spec)
	p := pty.New()
	size := pty.Size{Cols: spec.Cols, Rows: spec.Rows}
	if err := p.Start(cmd, args, env, spec.Cwd, size); err != nil {
		return nil, fmt.Errorf("gagal memulai sesi: %w", err)
	}

	e := &entry{
		sess: Session{
			ID:          newID(),
			WorkspaceID: spec.WorkspaceID,
			ProjectID:   spec.ProjectID,
			Cwd:         spec.Cwd,
			ProviderID:  spec.ProviderID,
			Status:      StatusRunning,
			CreatedAt:   time.Now(),
		},
		size: size,
		p:    p,
		sb:   newScrollback(),
		done: make(chan struct{}),
	}
	m.mu.Lock()
	m.sessions[e.sess.ID] = e
	m.mu.Unlock()

	go m.readLoop(e)

	s := snapshotSession(e)
	return &s, nil
}

func (m *Manager) readLoop(e *entry) {
	id := e.sess.ID // immutable setelah dibuat
	topic := "session.output." + id
	buf := make([]byte, readBufSize)
	for {
		n, err := e.p.Read(buf)
		if n > 0 {
			off := e.sb.Append(buf[:n])
			m.pub(topic, outputPayload{
				Data:   base64.StdEncoding.EncodeToString(buf[:n]),
				Offset: off,
			})
		}
		if err != nil {
			break
		}
	}
	code, _ := e.p.Wait()

	m.mu.Lock()
	m.exitSeq++
	seq := m.exitSeq
	m.mu.Unlock()

	e.mu.Lock()
	e.sess.Status = StatusExited
	c := code
	e.sess.ExitCode = &c
	e.exitSeq = seq
	e.mu.Unlock()

	m.pub("session.exit", exitPayload{ID: id, ExitCode: code})
	_ = e.p.Close()
	close(e.done)
	m.pruneExited()
}

func (m *Manager) pruneExited() {
	m.mu.Lock()
	defer m.mu.Unlock()
	type ex struct {
		id  string
		seq uint64
	}
	var exited []ex
	for id, e := range m.sessions {
		e.mu.Lock()
		seq := e.exitSeq
		e.mu.Unlock()
		if seq > 0 {
			exited = append(exited, ex{id, seq})
		}
	}
	if len(exited) <= maxExitedKept {
		return
	}
	sort.Slice(exited, func(i, j int) bool { return exited[i].seq < exited[j].seq })
	for _, x := range exited[:len(exited)-maxExitedKept] {
		delete(m.sessions, x.id)
	}
}

// List mengembalikan semua sesi menurut waktu pembuatan.
func (m *Manager) List() []Session {
	m.mu.RLock()
	out := make([]Session, 0, len(m.sessions))
	for _, e := range m.sessions {
		out = append(out, snapshotSession(e))
	}
	m.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Get mengembalikan salinan sesi.
func (m *Manager) Get(id string) (*Session, error) {
	e, err := m.lookup(id)
	if err != nil {
		return nil, err
	}
	s := snapshotSession(e)
	return &s, nil
}

// Attach mengembalikan sesi beserta scrollback dan offset.
func (m *Manager) Attach(id string) (AttachResult, error) {
	e, err := m.lookup(id)
	if err != nil {
		return AttachResult{}, err
	}
	data, off := e.sb.Snapshot()
	return AttachResult{Session: snapshotSession(e), Scrollback: data, Offset: off}, nil
}

// Write menulis data ke input sesi.
func (m *Manager) Write(id string, data []byte) error {
	e, err := m.lookup(id)
	if err != nil {
		return err
	}
	if !e.running() {
		return ErrNotRunning
	}
	if len(data) == 0 {
		return nil
	}
	e.wmu.Lock()
	_, werr := e.p.Write(data)
	e.wmu.Unlock()
	if werr != nil {
		if !e.running() {
			return ErrNotRunning
		}
		return fmt.Errorf("gagal menulis ke sesi: %w", werr)
	}
	return nil
}

// Resize mengubah ukuran terminal sesi.
func (m *Manager) Resize(id string, cols, rows uint16) error {
	e, err := m.lookup(id)
	if err != nil {
		return err
	}
	if cols == 0 || rows == 0 {
		return fmt.Errorf("%w: ukuran harus lebih dari nol", ErrInvalid)
	}
	if !e.running() {
		return ErrNotRunning
	}
	size := pty.Size{Cols: cols, Rows: rows}
	if err := e.p.Resize(size); err != nil {
		return fmt.Errorf("gagal mengubah ukuran sesi: %w", err)
	}
	e.mu.Lock()
	e.size = size
	e.mu.Unlock()
	return nil
}

type pidder interface{ Pid() int }

// Kill menghentikan sesi (SIGHUP lewat Close, SIGKILL setelah masa tenggang)
// lalu menghapusnya dari daftar.
func (m *Manager) Kill(id string) error {
	e, err := m.lookup(id)
	if err != nil {
		return err
	}
	if e.running() {
		_ = e.p.Close()
		timer := time.NewTimer(killGracePeriod)
		select {
		case <-e.done:
			timer.Stop()
		case <-timer.C:
			if pp, ok := e.p.(pidder); ok && pp.Pid() > 0 {
				_ = syscall.Kill(-pp.Pid(), syscall.SIGKILL)
				_ = syscall.Kill(pp.Pid(), syscall.SIGKILL)
			} else {
				_ = e.p.Close()
			}
			<-e.done
		}
	}
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
	return nil
}

// Shutdown menghentikan semua sesi; idempoten.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.closed = true
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_ = m.Kill(id)
		}(id)
	}
	wg.Wait()
}
