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

func TestReconcile(t *testing.T) {
	// Folder sesi s1: wf r-run berjalan, wf r-done selesai, wf r-none tanpa jejak.
	tp := folderFixture(t, map[string]string{
		"subagents/workflows/r-run/journal.jsonl": "journal_running.jsonl",
		"workflows/wf_r-done.json":                "wf_completed.json",
	})
	reg := NewRegistry()
	reg.Apply("s1", "SubagentStart", map[string]any{"agent_id": "x", "agent_type": "t", "transcript_path": tp})
	reg.mu.Lock()
	reg.transcripts["s1"] = tp
	reg.runs["w1"] = &Run{ID: "w1", SessionID: "s1", Kind: KindWorkflow, RunID: "r-run"}
	reg.runs["w2"] = &Run{ID: "w2", SessionID: "s1", Kind: KindWorkflow, RunID: "r-done"}
	reg.runs["w3"] = &Run{ID: "w3", SessionID: "s1", Kind: KindWorkflow, RunID: "r-none"}
	reg.runs["g1"] = &Run{ID: "g1", SessionID: "ghost", Kind: KindWorkflow, RunID: "r-none"}
	reg.mu.Unlock()
	lk := fakeLookup{"s1": "ws1"}

	changed := Reconcile(reg, lk, "ws1")
	got := map[string]*Run{}
	for _, r := range reg.List(nil) {
		r := r
		got[r.ID] = &r
	}
	if got["w1"] == nil || !got["w1"].Confirmed || got["w1"].Progress == nil || got["w1"].Progress.Total != 2 {
		t.Fatalf("w1 = %+v", got["w1"])
	}
	if got["w2"] != nil {
		t.Fatal("w2 (selesai) harus dibuang")
	}
	if got["w3"] == nil || got["w3"].Confirmed {
		t.Fatalf("w3 = %+v", got["w3"])
	}
	if got["g1"] != nil {
		t.Fatal("g1 (sesi hilang, tak ditemukan) harus dibuang")
	}
	if len(changed) != 2 {
		t.Fatalf("changed = %v", changed)
	}
	// Panggilan kedua: tidak ada perubahan lagi.
	if c := Reconcile(reg, lk, "ws1"); len(c) != 0 {
		t.Fatalf("changed kedua = %v", c)
	}
}
