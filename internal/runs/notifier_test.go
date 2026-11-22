package runs

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

type snapshotRecorder struct {
	mu   sync.Mutex
	last map[string][]Run
	n    int
}

func (s *snapshotRecorder) emit(sid string, runs []Run) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		s.last = map[string][]Run{}
	}
	s.last[sid] = runs
	s.n++
}

// Snapshot terakhir yang dipublish untuk sebuah sesi harus sama dengan keadaan
// akhir registry, apa pun urutan penjadwalan goroutine hook yang serentak.
func TestNotifierLastPublishedMatchesFinalState(t *testing.T) {
	for round := 0; round < 200; round++ {
		reg := NewRegistry()
		rec := &snapshotRecorder{}
		n := NewNotifier(reg, rec.emit)
		const k = 8
		for i := 0; i < k; i++ {
			n.Apply("s1", "SubagentStart", map[string]any{"agent_id": fmt.Sprint("a", i), "agent_type": "t"})
		}
		var wg sync.WaitGroup
		for i := 0; i < k-1; i++ { // satu agent tersisa
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				n.Apply("s1", "SubagentStop", map[string]any{"agent_id": fmt.Sprint("a", i)})
			}(i)
		}
		wg.Wait()
		want := reg.List(map[string]bool{"s1": true})
		rec.mu.Lock()
		got := rec.last["s1"]
		rec.mu.Unlock()
		if len(want) != 1 || !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d: terakhir dipublish %+v, registry %+v", round, got, want)
		}
	}
}

func TestNotifierForgetPublishesEmptyAndSkipsNoChange(t *testing.T) {
	reg := NewRegistry()
	rec := &snapshotRecorder{}
	n := NewNotifier(reg, rec.emit)
	n.Apply("s1", "Stop", map[string]any{}) // tidak ada perubahan
	if rec.n != 0 {
		t.Fatalf("publish tanpa perubahan: %d", rec.n)
	}
	n.Apply("s1", "SubagentStart", map[string]any{"agent_id": "a", "agent_type": "t"})
	n.Forget("s1")
	if len(rec.last["s1"]) != 0 || rec.n != 2 {
		t.Fatalf("last=%+v n=%d", rec.last["s1"], rec.n)
	}
	n.Forget("s1")
	if rec.n != 2 {
		t.Fatalf("n = %d", rec.n)
	}
}

func TestNotifierReconcilePublishesChangedSessions(t *testing.T) {
	tp := folderFixture(t, map[string]string{"workflows/wf_done-1.json": "wf_completed.json"})
	reg := NewRegistry()
	reg.mu.Lock()
	reg.transcripts["s1"] = tp
	reg.runs["w"] = &Run{ID: "w", SessionID: "s1", Kind: KindWorkflow, RunID: "wf_done-1"}
	reg.mu.Unlock()
	rec := &snapshotRecorder{}
	n := NewNotifier(reg, rec.emit)
	n.Reconcile(fakeLookup{"s1": "ws1"}, "ws1")
	if rec.n != 1 || len(rec.last["s1"]) != 0 {
		t.Fatalf("n=%d last=%+v", rec.n, rec.last["s1"])
	}
}
