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
