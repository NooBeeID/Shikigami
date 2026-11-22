package runs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// folderFixture menyusun folder sesi Claude sementara; mengembalikan transcriptPath.
func folderFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	folder := filepath.Join(root, "sess1")
	for dst, src := range files {
		data, err := os.ReadFile(filepath.Join("testdata", src))
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(folder, dst)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return folder + ".jsonl"
}

func TestReadDetailWorkflowFinished(t *testing.T) {
	tp := folderFixture(t, map[string]string{"workflows/wf_e8ae7cd1-a8d.json": "wf_completed.json"})
	d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "wf_e8ae7cd1-a8d"})
	if !d.Finished {
		t.Fatalf("detail = %+v", d)
	}
}

func TestReadDetailWorkflowRunning(t *testing.T) {
	tp := folderFixture(t, map[string]string{"subagents/workflows/wf_e8ae7cd1-a8d/journal.jsonl": "journal_running.jsonl"})
	d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "wf_e8ae7cd1-a8d"})
	if !d.Found || d.Finished || d.Progress == nil {
		t.Fatalf("detail = %+v", d)
	}
	p := d.Progress
	if p.Total != 2 || p.Done != 1 || p.PhaseTitle != "Probe" || len(p.Agents) != 2 {
		t.Fatalf("progress = %+v", p)
	}
	if !p.Agents[0].Done || p.Agents[1].Done || p.Agents[0].Label != "probe-1" {
		t.Fatalf("agents = %+v", p.Agents)
	}
}

func TestReadDetailWorkflowPartialLastLine(t *testing.T) {
	tp := folderFixture(t, map[string]string{"subagents/workflows/wf_e8ae7cd1-a8d/journal.jsonl": "journal_running.jsonl"})
	jp := filepath.Join(strings.TrimSuffix(tp, ".jsonl"), "subagents/workflows/wf_e8ae7cd1-a8d/journal.jsonl")
	f, _ := os.OpenFile(jp, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(`{"type":"result","agentId":"a6f34`)
	f.Close()
	d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "wf_e8ae7cd1-a8d"})
	if !d.Found || d.Progress == nil || d.Progress.Total != 2 || d.Progress.Done != 1 {
		t.Fatalf("detail = %+v", d)
	}
}

func TestReadDetailWorkflowNoJournalMeansUnconfirmed(t *testing.T) {
	tp := folderFixture(t, map[string]string{"subagents/other.txt": "agent.meta.json"})
	d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "wf_e8ae7cd1-a8d"})
	if d.Found || d.Finished {
		t.Fatalf("detail = %+v", d)
	}
}

func TestReadDetailSubagentChecklist(t *testing.T) {
	tp := folderFixture(t, map[string]string{
		"subagents/agent-ag1.meta.json": "agent.meta.json",
		"subagents/agent-ag1.jsonl":     "agent_transcript.jsonl",
	})
	d := ReadDetail(tp, Run{Kind: KindSubagent, AgentID: "ag1"})
	if !d.Found || !d.Finished || d.Progress == nil || d.Progress.Done != 2 || d.Progress.Total != 3 {
		t.Fatalf("detail = %+v progress=%+v", d, d.Progress)
	}
}

func TestReadDetailSubagentNoChecklist(t *testing.T) {
	tp := folderFixture(t, map[string]string{"subagents/agent-ag1.meta.json": "workflow_agent.meta.json"})
	d := ReadDetail(tp, Run{Kind: KindSubagent, AgentID: "ag1"})
	if !d.Found || d.Finished || d.Progress != nil {
		t.Fatalf("detail = %+v", d)
	}
}

func TestReadDetailMissingPath(t *testing.T) {
	for _, tp := range []string{"", "/nonexistent/x/sess.jsonl"} {
		if d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "wf_e8ae7cd1-a8d"}); d != (Detail{}) {
			t.Fatalf("detail = %+v", d)
		}
		if d := ReadDetail(tp, Run{Kind: KindSubagent, AgentID: "a"}); d != (Detail{}) {
			t.Fatalf("detail = %+v", d)
		}
	}
}

func TestReadDetailCorruptJSON(t *testing.T) {
	tp := folderFixture(t, map[string]string{"subagents/x": "agent.meta.json"})
	folder := strings.TrimSuffix(tp, ".jsonl")
	os.WriteFile(filepath.Join(folder, "subagents/agent-ag1.meta.json"), []byte("{rusak"), 0o644)
	os.MkdirAll(filepath.Join(folder, "subagents/workflows/wf_e8ae7cd1-a8d"), 0o755)
	os.WriteFile(filepath.Join(folder, "subagents/workflows/wf_e8ae7cd1-a8d/journal.jsonl"), []byte("bukan json\n{{{\n"), 0o644)
	os.MkdirAll(filepath.Join(folder, "workflows"), 0o755)
	os.WriteFile(filepath.Join(folder, "workflows/wf_e8ae7cd1-a8d.json"), []byte("{rusak"), 0o644)
	if d := ReadDetail(tp, Run{Kind: KindSubagent, AgentID: "ag1"}); d != (Detail{}) {
		t.Fatalf("subagent = %+v", d)
	}
	if d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "wf_e8ae7cd1-a8d"}); d != (Detail{}) {
		t.Fatalf("workflow = %+v", d)
	}
}

func TestReadDetailRejectsPathTraversal(t *testing.T) {
	tp := folderFixture(t, map[string]string{
		"subagents/agent-ag1.meta.json":                     "agent.meta.json",
		"subagents/workflows/wf_e8ae7cd1-a8d/journal.jsonl": "journal.jsonl",
		"workflows/wf_e8ae7cd1-a8d.json":                    "wf_completed.json",
	})
	// Setelah filepath.Join membersihkan path, ID ini mendarat di berkas nyata.
	for _, id := range []string{"wf_e8ae7cd1-a8d/../wf_e8ae7cd1-a8d", "x/../wf_e8ae7cd1-a8d", "../wf_e8ae7cd1-a8d/../wf_e8ae7cd1-a8d"} {
		if d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: id}); d != (Detail{}) {
			t.Errorf("runId %q = %+v", id, d)
		}
	}
	// "agent-" + "x/../agent-ag1" bersih menjadi agent-ag1 (fixture ada).
	for _, id := range []string{"x/../agent-ag1", "../subagents/agent-ag1"} {
		if d := ReadDetail(tp, Run{Kind: KindSubagent, AgentID: id}); d != (Detail{}) {
			t.Errorf("agentId %q = %+v", id, d)
		}
	}
	// Kontrol: ID sah memang menemukan berkasnya.
	if d := ReadDetail(tp, Run{Kind: KindSubagent, AgentID: "ag1"}); !d.Found {
		t.Fatalf("kontrol ag1 = %+v", d)
	}
	if d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "wf_e8ae7cd1-a8d"}); !d.Found {
		t.Fatalf("kontrol wf = %+v", d)
	}
}

func TestSafeID(t *testing.T) {
	bad := []string{"", ".", "..", "a/b", `a\b`, "/etc/passwd", "../x", "x/..", "a..b", "a\x00b"}
	for _, id := range bad {
		if safeID(id) {
			t.Errorf("safeID(%q) = true, harus ditolak", id)
		}
	}
	for _, id := range []string{"r1", "wf_e8ae7cd1-a8d", "a760caa2a8227c09d"} {
		if !safeID(id) {
			t.Errorf("safeID(%q) = false", id)
		}
	}
}

func TestReadTailBounded(t *testing.T) {
	p := filepath.Join(t.TempDir(), "big.jsonl")
	line := strings.Repeat("x", 1000) + "\n"
	big := strings.Repeat(line, maxReadBytes/len(line)+50)
	os.WriteFile(p, []byte(big+"{\"ok\":1}\n"), 0o644)
	lines, ok := readTailLines(p)
	if !ok {
		t.Fatal("tidak terbaca")
	}
	total := 0
	for _, l := range lines {
		total += len(l) + 1
	}
	if total > maxReadBytes || lines[len(lines)-1] != `{"ok":1}` {
		t.Fatalf("total=%d last=%q", total, lines[len(lines)-1])
	}
}

func TestReadDetailSubagentDescription(t *testing.T) {
	tp := folderFixture(t, map[string]string{"subagents/agent-ag1.meta.json": "agent.meta.json"})
	d := ReadDetail(tp, Run{Kind: KindSubagent, AgentID: "ag1"})
	if d.Description != "Spike probe agent" {
		t.Fatalf("description = %q", d.Description)
	}
}
