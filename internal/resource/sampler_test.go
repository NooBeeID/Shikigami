package resource

import (
	"testing"
	"time"
)

func jamPalsu(t *time.Time) func() time.Time { return func() time.Time { return *t } }

func cari(s Snapshot, pid int) (Proc, bool) {
	for _, p := range s.Procs {
		if p.PID == pid {
			return p, true
		}
	}
	return Proc{}, false
}

func TestSampleAggregatesChildrenIntoRoot(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSampler(jamPalsu(&now))
	snap := s.Sample([]Root{{Label: "a", Kind: "session", PID: 10}}, []RawProc{
		{PID: 10, PPID: 1, RSSKB: 100},
		{PID: 11, PPID: 10, RSSKB: 200},
		{PID: 12, PPID: 11, RSSKB: 300},
		{PID: 99, PPID: 1, RSSKB: 999},
	})
	p, ok := cari(snap, 10)
	if !ok {
		t.Fatal("root tidak ada")
	}
	if p.Count != 3 || p.RSSBytes != 600*1024 {
		t.Errorf("got count=%d rss=%d", p.Count, p.RSSBytes)
	}
}

func TestSampleAttributesToNearestRoot(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSampler(jamPalsu(&now))
	snap := s.Sample([]Root{
		{Label: "d", Kind: "daemon", PID: 1},
		{Label: "s", Kind: "session", PID: 20},
	}, []RawProc{
		{PID: 1, PPID: 0, RSSKB: 10},
		{PID: 20, PPID: 1, RSSKB: 20},
		{PID: 21, PPID: 20, RSSKB: 30},
	})
	d, _ := cari(snap, 1)
	se, _ := cari(snap, 20)
	if d.Count != 1 || d.RSSBytes != 10*1024 {
		t.Errorf("daemon: %+v", d)
	}
	if se.Count != 2 || se.RSSBytes != 50*1024 {
		t.Errorf("sesi: %+v", se)
	}
	if snap.TotalRSSBytes != 60*1024 {
		t.Errorf("total ganda: %d", snap.TotalRSSBytes)
	}
}

func TestSampleCPUFromDelta(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSampler(jamPalsu(&now))
	roots := []Root{{Kind: "session", PID: 10}}
	s1 := s.Sample(roots, []RawProc{{PID: 10, CPUSeconds: 5}})
	if s1.Procs[0].CPUPercent != 0 {
		t.Errorf("sampel pertama: %v", s1.Procs[0].CPUPercent)
	}
	now = now.Add(10 * time.Second)
	s2 := s.Sample(roots, []RawProc{{PID: 10, CPUSeconds: 10}})
	if s2.Procs[0].CPUPercent != 50 {
		t.Errorf("got %v, want 50", s2.Procs[0].CPUPercent)
	}
}

func TestSampleCPUNeverNegative(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSampler(jamPalsu(&now))
	roots := []Root{{Kind: "session", PID: 10}}
	s.Sample(roots, []RawProc{{PID: 10, CPUSeconds: 50}})
	now = now.Add(10 * time.Second)
	snap := s.Sample(roots, []RawProc{{PID: 10, CPUSeconds: 1}})
	if snap.Procs[0].CPUPercent != 0 {
		t.Errorf("got %v, want 0", snap.Procs[0].CPUPercent)
	}
}

func TestSampleMissingRootPID(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSampler(jamPalsu(&now))
	snap := s.Sample([]Root{{Kind: "session", PID: 777}}, []RawProc{{PID: 10, PPID: 1}})
	if len(snap.Procs) != 0 {
		t.Errorf("seharusnya kosong: %+v", snap.Procs)
	}
}

func TestSampleExtraPIDs(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSampler(jamPalsu(&now))
	snap := s.Sample([]Root{{Kind: "app", PID: 10, Extra: []int{50}}}, []RawProc{
		{PID: 10, PPID: 1, RSSKB: 1},
		{PID: 50, PPID: 1, RSSKB: 2},
		{PID: 51, PPID: 50, RSSKB: 4},
		{PID: 60, PPID: 1, RSSKB: 8},
	})
	p, _ := cari(snap, 10)
	if p.Count != 3 || p.RSSBytes != 7*1024 {
		t.Errorf("got %+v", p)
	}
}

func TestSampleTotals(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSampler(jamPalsu(&now))
	roots := []Root{{Kind: "daemon", PID: 1}, {Kind: "session", PID: 2}}
	s.Sample(roots, []RawProc{{PID: 1, CPUSeconds: 0, RSSKB: 5}, {PID: 2, CPUSeconds: 0, RSSKB: 7}})
	now = now.Add(10 * time.Second)
	snap := s.Sample(roots, []RawProc{{PID: 1, CPUSeconds: 1, RSSKB: 5}, {PID: 2, CPUSeconds: 3, RSSKB: 7}})
	var rss uint64
	var cpu float64
	for _, p := range snap.Procs {
		rss += p.RSSBytes
		cpu += p.CPUPercent
	}
	if snap.TotalRSSBytes != rss || snap.TotalCPUPercent != cpu || cpu != 40 {
		t.Errorf("total=%d/%v jumlah=%d/%v", snap.TotalRSSBytes, snap.TotalCPUPercent, rss, cpu)
	}
	if snap.At != now.UnixMilli() {
		t.Errorf("At=%d", snap.At)
	}
}

func TestSampleBornInWindowCountsFullCPU(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSampler(jamPalsu(&now))
	roots := []Root{{Label: "a", Kind: "session", PID: 10}}
	s.Sample(roots, []RawProc{{PID: 10, PPID: 1, ElapsedSeconds: 500}})
	now = now.Add(5 * time.Second)
	snap := s.Sample(roots, []RawProc{
		{PID: 10, PPID: 1, ElapsedSeconds: 505},
		{PID: 11, PPID: 10, CPUSeconds: 2, ElapsedSeconds: 3},
	})
	p, _ := cari(snap, 10)
	if p.CPUPercent < 39.9 || p.CPUPercent > 40.1 {
		t.Errorf("cpu = %v, want 40", p.CPUPercent)
	}
}

func TestSampleOldPIDFirstSeenCountsZero(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSampler(jamPalsu(&now))
	roots := []Root{{Label: "a", Kind: "session", PID: 10}}
	s.Sample(roots, []RawProc{{PID: 10, PPID: 1, ElapsedSeconds: 500}})
	now = now.Add(5 * time.Second)
	snap := s.Sample(roots, []RawProc{
		{PID: 10, PPID: 1, ElapsedSeconds: 505},
		{PID: 11, PPID: 10, CPUSeconds: 100, ElapsedSeconds: 3000},
	})
	if p, _ := cari(snap, 10); p.CPUPercent != 0 {
		t.Errorf("cpu = %v, want 0", p.CPUPercent)
	}
}

func TestSampleFirstCallZeroCPU(t *testing.T) {
	now := time.Unix(1000, 0)
	s := NewSampler(jamPalsu(&now))
	snap := s.Sample([]Root{{Label: "a", Kind: "session", PID: 10}}, []RawProc{
		{PID: 10, PPID: 1, CPUSeconds: 5, ElapsedSeconds: 1},
	})
	if p, _ := cari(snap, 10); p.CPUPercent != 0 {
		t.Errorf("cpu = %v, want 0", p.CPUPercent)
	}
}
