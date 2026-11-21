package status

import (
	"sync"
	"time"
)

// OnOutput dipanggil untuk tiap output PTY sesi. Catatan: echo keystroke user
// terhitung output (keterbatasan yang diterima). Tidak ada ticker; timer sekali
// jalan baru dibuat saat output pertama.
func (e *Engine) OnOutput(sessionID string) {
	e.mu.Lock()
	state := e.bindLocked(sessionID)
	if state == nil {
		e.mu.Unlock()
		return
	}
	if state.status == StatusDone {
		e.mu.Unlock()
		return
	}
	capable := hookCapable(state)
	var delay time.Duration
	switch {
	case !capable:
		delay = SilentDelay
	case !state.hookSeen:
		delay = SilentDelayWithHooks
		if state.status == StatusWaiting {
			// jangan menimpa waiting/done oleh output sebelum hook pertama
			e.mu.Unlock()
			return
		}
	default:
		stopTimerLocked(state)
		e.mu.Unlock()
		return
	}
	if state.timer == nil {
		id := sessionID
		state.timer = time.AfterFunc(delay, func() { e.onSilent(id) })
	} else {
		state.timer.Reset(delay)
	}
	needRun := state.status != StatusRunning
	e.mu.Unlock()
	if needRun {
		e.Set(sessionID, StatusRunning, SourceOutput, "output")
	}
}

// onSilent dipanggil saat timer output-diam habis. Race tipis (output tiba tepat
// saat timer habis) diterima: output berikutnya mengembalikan status running.
func (e *Engine) onSilent(id string) {
	e.mu.Lock()
	state := e.states[id]
	if state == nil || state.status == StatusDone ||
		(hookCapable(state) && (state.hookSeen || state.status == StatusWaiting)) {
		e.mu.Unlock()
		return
	}
	e.mu.Unlock()
	e.Set(id, StatusIdle, SourceOutput, "silent")
}

func stopTimerLocked(state *sessionState) {
	if state.timer != nil {
		state.timer.Stop()
	}
}

// IsIdle true untuk idle|done|waiting; false untuk running dan sesi tak dikenal.
func IsIdle(sessionID string) bool {
	e := Default()
	if e == nil {
		return false
	}
	s, ok := e.Get(sessionID)
	return ok && isIdleStatus(s.Status)
}

// OnIdle memanggil fn sekali pada transisi (atau segera, di goroutine) ke status idle.
func OnIdle(sessionID string, fn func()) (cancel func()) {
	noop := func() {}
	e := Default()
	if e == nil || fn == nil {
		return noop
	}
	e.mu.Lock()
	state := e.bindLocked(sessionID)
	if state == nil {
		e.mu.Unlock()
		return noop
	}
	if isIdleStatus(state.status) {
		e.mu.Unlock()
		go fn()
		return noop
	}
	id := state.id
	wid := e.nextWait
	e.nextWait++
	if e.waiters[id] == nil {
		e.waiters[id] = map[int]func(){}
	}
	e.waiters[id][wid] = fn
	e.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			e.mu.Lock()
			delete(e.waiters[id], wid)
			e.mu.Unlock()
		})
	}
}
