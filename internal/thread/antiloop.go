package thread

import "sync"

// MaxAgentChain adalah batas pesan beruntun antar agent sebelum thread dijeda.
const MaxAgentChain = 8

const (
	antiloopPauseText  = "Thread dijeda: pesan beruntun antar agent mencapai batas. Kirim pesan untuk melanjutkan."
	antiloopResumeText = "Thread dilanjutkan."
)

type antiloopState struct {
	count  int
	paused bool
}

// AntiLoop menghitung pesan beruntun antar agent dan menjeda thread di MaxAgentChain.
type AntiLoop struct {
	mu    sync.Mutex
	state map[string]*antiloopState
	mgr   *Manager
}

// NewAntiLoop membuat AntiLoop dan mendaftarkan hook OnMessage/OnClose sendiri.
func NewAntiLoop(m *Manager) *AntiLoop {
	a := &AntiLoop{state: map[string]*antiloopState{}, mgr: m}
	m.OnMessage(a.antiloopOnMessage)
	m.OnClose(a.antiloopOnClose)
	return a
}

func (a *AntiLoop) antiloopGet(id string) *antiloopState {
	st := a.state[id]
	if st == nil {
		st = &antiloopState{}
		a.state[id] = st
	}
	return st
}

func (a *AntiLoop) antiloopOnMessage(t Thread, msg ThreadMessage) {
	if msg.Kind != KindMessage {
		return
	}
	reset := false
	add := false
	if msg.From == FromUser {
		reset = true
	} else {
		p, ok := antiloopFindParticipant(t, msg.From)
		if !ok {
			return
		}
		switch p.Role {
		case RoleMaster:
			reset = true
		case RolePeer:
			add = msg.To != ToUser
		}
	}

	var postText string
	a.mu.Lock()
	st := a.antiloopGet(t.ID)
	if reset {
		st.count = 0
		if st.paused {
			st.paused = false
			postText = antiloopResumeText
		}
	} else if add {
		st.count++
		if st.count >= MaxAgentChain && !st.paused {
			st.paused = true
			postText = antiloopPauseText
		}
	}
	a.mu.Unlock()

	if postText != "" {
		go a.mgr.PostSystem(t.ID, postText)
	}
}

func (a *AntiLoop) antiloopOnClose(t Thread) {
	a.mu.Lock()
	delete(a.state, t.ID)
	a.mu.Unlock()
}

func antiloopFindParticipant(t Thread, sessionID string) (Participant, bool) {
	for _, p := range t.Participants {
		if p.SessionID == sessionID {
			return p, true
		}
	}
	return Participant{}, false
}

// Paused melaporkan apakah thread sedang dijeda.
func (a *AntiLoop) Paused(threadID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.state[threadID]
	return st != nil && st.paused
}

// Check mengembalikan ErrPaused bila thread dijeda dan pengirim adalah peer.
func (a *AntiLoop) Check(t Thread, fromSessionID string) error {
	p, ok := antiloopFindParticipant(t, fromSessionID)
	if !ok || p.Role != RolePeer {
		return nil
	}
	if a.Paused(t.ID) {
		return ErrPaused
	}
	return nil
}

// Reset mengosongkan hitungan dan mencabut jeda tanpa pesan sistem.
func (a *AntiLoop) Reset(threadID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if st := a.state[threadID]; st != nil {
		st.count = 0
		st.paused = false
	}
}
