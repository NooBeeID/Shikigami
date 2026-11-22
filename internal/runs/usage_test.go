package runs

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	lineA  = `{"type":"assistant","message":{"id":"m1","model":"claude-sonnet-5-5","usage":{"input_tokens":2,"cache_creation_input_tokens":100,"cache_read_input_tokens":0,"output_tokens":10}}}` + "\n"
	lineB  = `{"type":"assistant","message":{"id":"m2","model":"claude-haiku-5-5","usage":{"input_tokens":3,"cache_creation_input_tokens":0,"cache_read_input_tokens":102,"output_tokens":20}}}` + "\n"
	userLn = `{"type":"user","message":{"role":"user","content":"hai"}}` + "\n"
)

func appendFile(t *testing.T, p, s string) {
	t.Helper()
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

func TestReadUsageDedupeAndModel(t *testing.T) {
	p := filepath.Join(t.TempDir(), "agent.jsonl")
	// m1 ditulis tiga kali (satu baris per blok konten) dan harus dihitung sekali.
	appendFile(t, p, userLn+lineA+lineA+lineA+lineB)
	u, model := readUsage(p)
	if u == nil || *u != (Usage{Input: 105, CacheRead: 102, Output: 30}) {
		t.Fatalf("usage = %+v", u)
	}
	if model != "claude-haiku-5-5" {
		t.Fatalf("model = %q", model)
	}
}

func TestReadUsageIncrementalAndPartialLine(t *testing.T) {
	p := filepath.Join(t.TempDir(), "agent.jsonl")
	appendFile(t, p, lineA)
	if u, _ := readUsage(p); u == nil || u.Output != 10 {
		t.Fatalf("awal = %+v", u)
	}
	// Baris B baru separuh tertulis: belum boleh dihitung.
	appendFile(t, p, lineB[:40])
	if u, _ := readUsage(p); u == nil || u.Output != 10 {
		t.Fatalf("baris parsial dihitung: %+v", u)
	}
	appendFile(t, p, lineB[40:])
	u, model := readUsage(p)
	if u == nil || u.Output != 30 || model != "claude-haiku-5-5" {
		t.Fatalf("setelah lengkap = %+v %q", u, model)
	}
}

func TestReadUsageMissingFile(t *testing.T) {
	if u, m := readUsage(filepath.Join(t.TempDir(), "tidak-ada.jsonl")); u != nil || m != "" {
		t.Fatalf("u=%+v m=%q", u, m)
	}
}

func TestReadDetailSubagentUsage(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "sess1")
	sub := filepath.Join(folder, "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	appendFile(t, filepath.Join(sub, "agent-a1.meta.json"), `{"description":"x"}`)
	appendFile(t, filepath.Join(sub, "agent-a1.jsonl"), lineA)
	d := ReadDetail(folder+".jsonl", Run{Kind: KindSubagent, AgentID: "a1"})
	if d.Progress == nil || d.Progress.Model != "claude-sonnet-5-5" || d.Progress.Usage == nil || d.Progress.Usage.Input != 102 {
		t.Fatalf("progress = %+v", d.Progress)
	}
}

func TestReadDetailWorkflowUsage(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "sess1")
	dir := filepath.Join(folder, "subagents", "workflows", "wf_x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	appendFile(t, filepath.Join(dir, "journal.jsonl"),
		`{"type":"started","agentId":"a1","label":"satu","phase":"P"}`+"\n"+
			`{"type":"started","agentId":"a2","label":"dua","phase":"P"}`+"\n")
	appendFile(t, filepath.Join(dir, "agent-a1.jsonl"), lineA)
	appendFile(t, filepath.Join(dir, "agent-a2.jsonl"), lineB)
	d := ReadDetail(folder+".jsonl", Run{Kind: KindWorkflow, RunID: "wf_x"})
	p := d.Progress
	if p == nil || p.Usage == nil || *p.Usage != (Usage{Input: 105, CacheRead: 102, Output: 30}) {
		t.Fatalf("progress = %+v", p)
	}
	models := map[string]string{}
	for _, a := range p.Agents {
		models[a.Label] = a.Model
	}
	if models["satu"] != "claude-sonnet-5-5" || models["dua"] != "claude-haiku-5-5" {
		t.Fatalf("models = %v", models)
	}
}
