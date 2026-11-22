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
	tp := folderFixture(t, map[string]string{"workflows/wf_r1.json": "wf_completed.json"})
	d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "r1"})
	if !d.Finished {
		t.Fatalf("detail = %+v", d)
	}
}

func TestReadDetailWorkflowRunning(t *testing.T) {
	tp := folderFixture(t, map[string]string{"subagents/workflows/r1/journal.jsonl": "journal_running.jsonl"})
	d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "r1"})
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
	tp := folderFixture(t, map[string]string{"subagents/workflows/r1/journal.jsonl": "journal_running.jsonl"})
	jp := filepath.Join(strings.TrimSuffix(tp, ".jsonl"), "subagents/workflows/r1/journal.jsonl")
	f, _ := os.OpenFile(jp, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(`{"type":"result","agentId":"a6f34`)
	f.Close()
	d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "r1"})
	if !d.Found || d.Progress == nil || d.Progress.Total != 2 || d.Progress.Done != 1 {
		t.Fatalf("detail = %+v", d)
	}
}

func TestReadDetailWorkflowNoJournalMeansUnconfirmed(t *testing.T) {
	tp := folderFixture(t, map[string]string{"subagents/other.txt": "agent.meta.json"})
	d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "r1"})
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
		if d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "r1"}); d != (Detail{}) {
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
	os.MkdirAll(filepath.Join(folder, "subagents/workflows/r1"), 0o755)
	os.WriteFile(filepath.Join(folder, "subagents/workflows/r1/journal.jsonl"), []byte("bukan json\n{{{\n"), 0o644)
	os.MkdirAll(filepath.Join(folder, "workflows"), 0o755)
	os.WriteFile(filepath.Join(folder, "workflows/wf_r1.json"), []byte("{rusak"), 0o644)
	if d := ReadDetail(tp, Run{Kind: KindSubagent, AgentID: "ag1"}); d != (Detail{}) {
		t.Fatalf("subagent = %+v", d)
	}
	if d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: "r1"}); d != (Detail{}) {
		t.Fatalf("workflow = %+v", d)
	}
}

func TestReadDetailRejectsPathTraversal(t *testing.T) {
	tp := folderFixture(t, map[string]string{
		"subagents/agent-ag1.meta.json":        "agent.meta.json",
		"subagents/workflows/r1/journal.jsonl": "journal.jsonl",
		"workflows/wf_r1.json":                 "wf_completed.json",
	})
	for _, id := range []string{"../r1", "r1/../r1", `r1\..\r1`, "..", "a/b", ""} {
		if d := ReadDetail(tp, Run{Kind: KindWorkflow, RunID: id}); d != (Detail{}) {
			t.Fatalf("runId %q = %+v", id, d)
		}
		if d := ReadDetail(tp, Run{Kind: KindSubagent, AgentID: id}); d != (Detail{}) {
			t.Fatalf("agentId %q = %+v", id, d)
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
