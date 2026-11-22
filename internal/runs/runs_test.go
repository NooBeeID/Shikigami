package runs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// fixture membaca payload hook asli dari testdata.
func fixture(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func event(m map[string]any) string { return m["hook_event_name"].(string) }

func apply(r *Registry, sid string, m map[string]any) bool {
	return r.Apply(sid, event(m), m)
}

func TestIdentityTicketStripsPath(t *testing.T) {
	got := identityOf(KindWorkflow, map[string]any{"args": map[string]any{"ticket": "docs/tickets/T01.md"}})
	if got != "T01" {
		t.Fatalf("got %q", got)
	}
}

func TestIdentityFallsBackToFirstPair(t *testing.T) {
	got := identityOf(KindWorkflow, map[string]any{"args": map[string]any{"zeta": "z", "alpha": "a"}})
	if got != "alpha=a" {
		t.Fatalf("got %q", got)
	}
}

func TestIdentityTruncates(t *testing.T) {
	got := identityOf(KindWorkflow, map[string]any{"args": map[string]any{"k": strings.Repeat("x", 200)}})
	if utf8.RuneCountInString(got) > 40 || !strings.HasSuffix(got, "…") {
		t.Fatalf("got %q", got)
	}
}

func TestIdentityNonObjectArgs(t *testing.T) {
	for _, a := range []any{"teks", nil, []any{"a"}} {
		if got := identityOf(KindWorkflow, map[string]any{"args": a}); got != "" {
			t.Fatalf("args %v: got %q", a, got)
		}
	}
	if got := identityOf(KindWorkflow, nil); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyStartAddsRun(t *testing.T) {
	r := NewRegistry()
	ch := r.Apply("s1", "PreToolUse", map[string]any{
		"tool_name":   "Workflow",
		"tool_use_id": "tu1",
		"tool_input":  map[string]any{"args": map[string]any{"ticket": "docs/T01.md"}},
	})
	l := r.List(nil)
	if !ch || len(l) != 1 || l[0].Kind != KindWorkflow || l[0].Identity != "T01" || l[0].Name != "workflow" {
		t.Fatalf("ch=%v list=%+v", ch, l)
	}
}

func TestTwoWorkflowsSameNameDifferentTicket(t *testing.T) {
	r := NewRegistry()
	for _, tk := range []string{"T01", "T02"} {
		r.Apply("s1", "PreToolUse", map[string]any{
			"tool_name": "Workflow", "tool_use_id": "tu" + tk,
			"tool_input": map[string]any{"args": map[string]any{"ticket": tk}},
		})
	}
	l := r.List(nil)
	if len(l) != 2 || l[0].ID == l[1].ID || l[0].Identity == l[1].Identity {
		t.Fatalf("%+v", l)
	}
}

func TestSubagentStopRemoves(t *testing.T) {
	r := NewRegistry()
	apply(r, "s1", fixture(t, "subagentstart_agent.json"))
	if len(r.List(nil)) != 1 {
		t.Fatal("run tidak dibuat")
	}
	if !apply(r, "s1", fixture(t, "subagentstop_agent.json")) {
		t.Fatal("changed false")
	}
	if len(r.List(nil)) != 0 {
		t.Fatal("run tidak terhapus")
	}
}

func TestWorkflowLaunchFillsNameAndTaskID(t *testing.T) {
	r := NewRegistry()
	apply(r, "s1", fixture(t, "pretooluse_workflow.json"))
	apply(r, "s1", fixture(t, "posttooluse_workflow.json"))
	l := r.List(nil)
	if len(l) != 1 {
		t.Fatalf("%+v", l)
	}
	w := l[0]
	if w.Name != "spike-probe" || w.TaskID != "wkt61tgun" || w.RunID != "wf_e8ae7cd1-a8d" || w.Identity != "T-SPIKE" {
		t.Fatalf("%+v", w)
	}
	if w.ID != "toolu_01R758uTvejVZBSSZ71y1Abh" || w.StartedAt == 0 {
		t.Fatalf("%+v", w)
	}
	if r.TranscriptPath("s1") == "" {
		t.Fatal("transcript path kosong")
	}
}

func TestWorkflowRemovedWhenGoneFromBackgroundTasks(t *testing.T) {
	r := NewRegistry()
	apply(r, "s1", fixture(t, "pretooluse_workflow.json"))
	apply(r, "s1", fixture(t, "posttooluse_workflow.json"))
	if apply(r, "s1", fixture(t, "stop_with_workflow_running.json")) || len(r.List(nil)) != 1 {
		t.Fatal("workflow harus bertahan")
	}
	if !apply(r, "s1", fixture(t, "stop_no_background.json")) || len(r.List(nil)) != 0 {
		t.Fatal("workflow harus terhapus")
	}
}

func TestWorkflowSubagentIgnored(t *testing.T) {
	r := NewRegistry()
	if apply(r, "s1", fixture(t, "subagentstart_workflow.json")) || len(r.List(nil)) != 0 {
		t.Fatal("workflow-subagent harus diabaikan")
	}
}

func TestSubagentStartUsesPendingDescription(t *testing.T) {
	r := NewRegistry()
	if apply(r, "s1", fixture(t, "pretooluse_agent.json")) || len(r.List(nil)) != 0 {
		t.Fatal("PreToolUse Agent belum boleh jadi run")
	}
	apply(r, "s1", fixture(t, "subagentstart_agent.json"))
	l := r.List(nil)
	if len(l) != 1 || l[0].Kind != KindSubagent || l[0].Identity != "Spike probe agent" ||
		l[0].ID != "ace33d88e5cb23509" || l[0].AgentID != l[0].ID {
		t.Fatalf("%+v", l)
	}
}

func TestSubagentPostToolUseLinksAgentID(t *testing.T) {
	r := NewRegistry()
	apply(r, "s1", fixture(t, "pretooluse_agent.json"))
	apply(r, "s1", fixture(t, "posttooluse_agent.json"))
	apply(r, "s1", fixture(t, "subagentstart_agent.json"))
	l := r.List(nil)
	if len(l) != 1 || l[0].Identity != "Spike probe agent" {
		t.Fatalf("%+v", l)
	}
}

func TestApplyMalformedPayload(t *testing.T) {
	r := NewRegistry()
	cases := []struct {
		ev string
		p  map[string]any
	}{
		{"PreToolUse", map[string]any{}},
		{"PreToolUse", map[string]any{"tool_name": "Workflow", "tool_input": "x", "tool_use_id": "a"}},
		{"PreToolUse", map[string]any{"tool_name": "Workflow", "tool_input": map[string]any{}}},
		{"PostToolUse", map[string]any{"tool_name": "Workflow", "tool_response": "x"}},
		{"SubagentStart", map[string]any{}},
		{"SubagentStop", map[string]any{}},
		{"Stop", map[string]any{"background_tasks": "x"}},
		{"Stop", nil},
	}
	for i, c := range cases {
		if r.Apply("s1", c.ev, c.p) {
			t.Fatalf("kasus %d: changed true", i)
		}
	}
	if len(r.List(nil)) != 0 {
		t.Fatal("tidak boleh ada run")
	}
}

func TestForgetRemovesAllOfSession(t *testing.T) {
	r := NewRegistry()
	apply(r, "s1", fixture(t, "pretooluse_workflow.json"))
	apply(r, "s1", fixture(t, "subagentstart_agent.json"))
	r.Apply("s2", "SubagentStart", map[string]any{"agent_id": "agent-s2", "agent_type": "general-purpose"})
	if !r.Forget("s1") {
		t.Fatal("changed false")
	}
	l := r.List(nil)
	if len(l) != 1 || l[0].SessionID != "s2" {
		t.Fatalf("%+v", l)
	}
	if r.Forget("s1") {
		t.Fatal("forget kedua harus false")
	}
}

func TestListFiltersBySessions(t *testing.T) {
	r := NewRegistry()
	apply(r, "s1", fixture(t, "pretooluse_workflow.json"))
	r.Apply("s2", "SubagentStart", map[string]any{"agent_id": "agent-s2", "agent_type": "general-purpose"})
	if n := len(r.List(map[string]bool{"s2": true})); n != 1 {
		t.Fatalf("n=%d", n)
	}
	if n := len(r.List(map[string]bool{})); n != 0 {
		t.Fatalf("n=%d", n)
	}
	if n := len(r.List(nil)); n != 2 {
		t.Fatalf("n=%d", n)
	}
}

func TestUnrelatedEventIgnored(t *testing.T) {
	r := NewRegistry()
	if r.Apply("s1", "Notification", map[string]any{"message": "hai"}) {
		t.Fatal("harus false")
	}
}

func TestRemoveAndMarkConfirmed(t *testing.T) {
	r := NewRegistry()
	apply(r, "s1", fixture(t, "pretooluse_workflow.json"))
	id := r.List(nil)[0].ID
	r.MarkConfirmed(id, &Progress{Done: 1, Total: 2})
	got := r.List(nil)[0]
	if !got.Confirmed || got.Progress == nil || got.Progress.Total != 2 {
		t.Fatalf("%+v", got)
	}
	if !r.Remove(id) || r.Remove(id) {
		t.Fatal("remove")
	}
}

func TestIdentityLongTicketTruncates(t *testing.T) {
	got := identityOf(KindWorkflow, map[string]any{"args": map[string]any{"ticket": strings.Repeat("x", 200)}})
	if utf8.RuneCountInString(got) > 40 || !strings.HasSuffix(got, "…") {
		t.Fatalf("got %q", got)
	}
}

func TestProgressNotAliased(t *testing.T) {
	r := NewRegistry()
	apply(r, "s1", fixture(t, "pretooluse_workflow.json"))
	id := r.List(nil)[0].ID
	in := &Progress{PhaseTitle: "a", Done: 1, Total: 2, Agents: []AgentLine{{Label: "x"}}}
	r.MarkConfirmed(id, in)
	in.Done = 99
	in.Agents[0].Label = "diubah"
	l := r.List(nil)
	if l[0].Progress.Done != 1 || l[0].Progress.Agents[0].Label != "x" {
		t.Fatalf("state terpengaruh caller: %+v", l[0].Progress)
	}
	l[0].Progress.Done = 77
	l[0].Progress.Agents[0].Label = "diubah2"
	again := r.List(nil)[0].Progress
	if again.Done != 1 || again.Agents[0].Label != "x" {
		t.Fatalf("state terpengaruh hasil List: %+v", again)
	}
}

func preWorkflow(r *Registry, sid, toolUseID string) bool {
	return r.Apply(sid, "PreToolUse", map[string]any{
		"tool_name": "Workflow", "tool_use_id": toolUseID,
		"tool_input": map[string]any{"args": map[string]any{"ticket": "docs/T01.md"}},
	})
}

func preAgent(r *Registry, sid, toolUseID, desc string) {
	r.Apply(sid, "PreToolUse", map[string]any{
		"tool_name": "Agent", "tool_use_id": toolUseID,
		"tool_input": map[string]any{"description": desc},
	})
}

func TestStopDropsWorkflowNeverLaunched(t *testing.T) {
	r := NewRegistry()
	preWorkflow(r, "s1", "tu1") // izin ditolak: PostToolUse tidak pernah datang
	if !r.Apply("s1", "Stop", map[string]any{}) {
		t.Fatal("Stop harus melaporkan perubahan")
	}
	if l := r.List(nil); len(l) != 0 {
		t.Fatalf("list = %+v", l)
	}
	// Stop kedua tidak mengubah apa pun.
	if r.Apply("s1", "Stop", map[string]any{}) {
		t.Fatal("Stop kedua tidak boleh berubah")
	}
}

func TestStopClearsOrphanedAgentPending(t *testing.T) {
	r := NewRegistry()
	preAgent(r, "s1", "tuX", "agent yang ditolak")
	r.Apply("s1", "Stop", map[string]any{})
	r.Apply("s1", "SubagentStart", map[string]any{"agent_id": "ag1", "agent_type": "general-purpose"})
	l := r.List(nil)
	if len(l) != 1 || l[0].Identity != "" {
		t.Fatalf("identitas basi menempel: %+v", l)
	}
}

func TestStopKeepsLaunchedWorkflowsRunningSubagentsAndOtherSessions(t *testing.T) {
	r := NewRegistry()
	apply(r, "s1", fixture(t, "pretooluse_workflow.json"))
	apply(r, "s1", fixture(t, "posttooluse_workflow.json")) // punya TaskID
	apply(r, "s1", fixture(t, "subagentstart_agent.json"))  // sub agent sudah jalan
	preWorkflow(r, "s2", "tu-s2")                           // sesi lain
	preAgent(r, "s2", "tu-agent-s2", "milik s2")
	preWorkflow(r, "s1", "tu-ghost")
	r.Apply("s1", "Stop", fixture(t, "stop_with_workflow_running.json"))
	got := map[string]bool{}
	for _, run := range r.List(nil) {
		got[run.SessionID+"/"+string(run.Kind)+"/"+run.ID] = true
	}
	if len(got) != 3 || got["s1/workflow/tu-ghost"] {
		t.Fatalf("got = %v", got)
	}
	if !got["s2/workflow/tu-s2"] {
		t.Fatalf("run sesi lain tersentuh: %v", got)
	}
	// Pending sesi lain tetap ada.
	r.Apply("s2", "SubagentStart", map[string]any{"agent_id": "ag-s2", "agent_type": "t"})
	for _, run := range r.List(nil) {
		if run.ID == "ag-s2" && run.Identity != "milik s2" {
			t.Fatalf("pending sesi lain hilang: %+v", run)
		}
	}
}
