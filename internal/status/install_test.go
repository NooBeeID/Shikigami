package status

import (
	"encoding/json"
	"os"
	"testing"
)

func TestWriteHooksFileIncludesRunEvents(t *testing.T) {
	path, err := writeHooksFile(t.TempDir(), "tok123", "/bin/shikigamictl")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Hooks map[string][]map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, ev := range []string{"PreToolUse", "SubagentStart", "SubagentStop"} {
		if len(doc.Hooks[ev]) == 0 {
			t.Fatalf("event %s tidak terpasang", ev)
		}
	}
	if m, _ := doc.Hooks["PreToolUse"][0]["matcher"].(string); m != "Workflow|Agent" {
		t.Fatalf("matcher PreToolUse = %q", m)
	}
	for _, ev := range []string{"UserPromptSubmit", "Notification", "Stop", "SessionStart", "PostToolUse", "SubagentStart", "SubagentStop"} {
		if len(doc.Hooks[ev]) == 0 {
			t.Fatalf("event lama %s hilang", ev)
		}
		if _, has := doc.Hooks[ev][0]["matcher"]; has {
			t.Fatalf("event %s tidak boleh punya matcher", ev)
		}
	}
}

func TestClassifyEventUnchanged(t *testing.T) {
	known := map[string]Status{
		"UserPromptSubmit": StatusRunning,
		"SessionStart":     StatusRunning,
		"Notification":     StatusWaiting,
		"Stop":             StatusDone,
	}
	for ev, want := range known {
		got, ok := ClassifyEvent(ev)
		if !ok || got != want {
			t.Errorf("ClassifyEvent(%q) = %v,%v", ev, got, ok)
		}
	}
	for _, ev := range []string{"PreToolUse", "SubagentStart", "SubagentStop", "PostToolUse"} {
		if _, ok := ClassifyEvent(ev); ok {
			t.Errorf("ClassifyEvent(%q) seharusnya tidak dikenal", ev)
		}
	}
}
