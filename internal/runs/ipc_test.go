package runs

import (
	"encoding/json"
	"testing"

	"github.com/NooBeeID/shikigami/internal/session"
)

type fakeLookup map[string]string // sessionID -> workspaceID

func (f fakeLookup) Lookup(id string) (session.Session, bool) {
	ws, ok := f[id]
	if !ok {
		return session.Session{}, false
	}
	return session.Session{ID: id, WorkspaceID: ws}, true
}

func TestListOnlyActiveWorkspaceSessions(t *testing.T) {
	reg := NewRegistry()
	for _, sid := range []string{"a", "b", "ghost"} {
		reg.Apply(sid, "SubagentStart", map[string]any{"agent_id": "x-" + sid, "agent_type": "t"})
	}
	lk := fakeLookup{"a": "ws1", "b": "ws2"}

	set := SessionIDsOfWorkspace(reg, lk, "ws1")
	if !set["a"] || set["b"] || set["ghost"] || len(set) != 1 {
		t.Fatalf("set = %v", set)
	}
	for _, r := range reg.List(set) {
		if r.SessionID != "a" {
			t.Fatalf("run sesi lain muncul: %+v", r)
		}
	}
	if set := SessionIDsOfWorkspace(reg, lk, "ws3"); len(set) != 0 {
		t.Fatalf("ws3 = %v", set)
	}
	b, _ := json.Marshal(reg.List(SessionIDsOfWorkspace(reg, lk, "ws3")))
	if string(b) != "[]" {
		t.Fatalf("json = %s", b)
	}
}
