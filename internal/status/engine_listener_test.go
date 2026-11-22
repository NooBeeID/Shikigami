package status

import (
	"errors"
	"testing"

	"github.com/NooBeeID/shikigami/internal/session"
)

type fakeSrc struct{}

func (fakeSrc) Get(id string) (*session.Session, error) {
	if id == "" {
		return nil, errors.New("kosong")
	}
	return &session.Session{ID: id, ProviderID: "claude", WorkspaceID: "ws1"}, nil
}
func (fakeSrc) List() []session.Session { return nil }

// newListenerEngine: NewEngine(nil) aman karena Set hanya Publish bila srv != nil.
func newListenerEngine() *Engine {
	e := NewEngine(nil)
	e.SetSessionSource(fakeSrc{})
	return e
}

func TestAddHookListenerReceivesEvents(t *testing.T) {
	e := newListenerEngine()
	var gotID, gotEv string
	var gotP map[string]any
	e.AddHookListener(func(id, ev string, p map[string]any) { gotID, gotEv, gotP = id, ev, p })
	// event tak dikenal ClassifyEvent tetap harus sampai ke listener
	if err := e.Apply("s1", "SubagentStop", map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	if gotID != "s1" || gotEv != "SubagentStop" || gotP["k"] != "v" {
		t.Fatalf("got %q %q %v", gotID, gotEv, gotP)
	}
}

func TestHookListenerPanicIsContained(t *testing.T) {
	e := newListenerEngine()
	e.AddHookListener(func(string, string, map[string]any) { panic("boom") })
	if err := e.Apply("s1", "Stop", nil); err != nil {
		t.Fatalf("Apply = %v", err)
	}
}

func TestHookListenerCancel(t *testing.T) {
	e := newListenerEngine()
	n := 0
	cancel := e.AddHookListener(func(string, string, map[string]any) { n++ })
	_ = e.Apply("s1", "Stop", nil)
	cancel()
	_ = e.Apply("s1", "Stop", nil)
	if n != 1 {
		t.Fatalf("n = %d, want 1", n)
	}
}

func TestExitListenerFiresEvenWhenAlreadyDone(t *testing.T) {
	e := newListenerEngine()
	if err := e.Apply("s1", "Stop", nil); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.Get("s1"); st.Status != StatusDone {
		t.Fatalf("status = %v, want done", st.Status)
	}
	var got []string
	e.AddExitListener(func(id string) { got = append(got, id) })
	e.handleExit("s1")
	if len(got) != 1 || got[0] != "s1" {
		t.Fatalf("got %v", got)
	}
}

func TestExitListenerPanicIsContained(t *testing.T) {
	e := newListenerEngine()
	_ = e.Apply("s1", "Stop", nil)
	called := false
	e.AddExitListener(func(string) { panic("boom") })
	e.AddExitListener(func(string) { called = true })
	e.handleExit("s1")
	if _, ok := e.states["s1"]; ok {
		t.Fatal("Forget tidak jalan setelah panic")
	}
	// Tiap listener punya recover sendiri, jadi listener kedua selalu jalan.
	if !called {
		t.Fatal("listener kedua tidak dipanggil setelah listener pertama panic")
	}
}

func TestExitListenerCancel(t *testing.T) {
	e := newListenerEngine()
	n := 0
	cancel := e.AddExitListener(func(string) { n++ })
	cancel()
	cancel()
	e.handleExit("s1")
	if n != 0 {
		t.Fatalf("n = %d", n)
	}
}
