package resource

import (
	"sort"
	"sync"
	"time"
)

type prevCPU struct {
	cpu float64
	at  time.Time
}

// Sampler menghitung agregasi per root dan CPU dari delta antar sampel.
type Sampler struct {
	now  func() time.Time
	mu   sync.Mutex
	prev map[int]prevCPU
	// lastAt waktu sampel sebelumnya; zero berarti belum pernah sampel.
	lastAt time.Time
}

// NewSampler membuat sampler; now nil berarti time.Now.
func NewSampler(now func() time.Time) *Sampler {
	if now == nil {
		now = time.Now
	}
	return &Sampler{now: now, prev: map[int]prevCPU{}}
}

// Sample mengagregasi procs ke root terdekat (leluhur-atau-diri) dan
// menghitung CPU% = delta waktu CPU / delta waktu dinding x 100.
func (s *Sampler) Sample(roots []Root, procs []RawProc) Snapshot {
	at := s.now()

	byPID := make(map[int]RawProc, len(procs))
	for _, p := range procs {
		byPID[p.PID] = p
	}
	// PID titik awal -> indeks root; PID root utama menang atas Extra.
	start := map[int]int{}
	for i, r := range roots {
		for _, x := range r.Extra {
			if _, dup := start[x]; !dup {
				start[x] = i
			}
		}
	}
	for i, r := range roots {
		start[r.PID] = i
	}

	// Telusur naik dari tiap proses ke titik awal terdekat.
	owner := func(pid int) (int, bool) {
		seen := 0
		for cur := pid; seen < 4096; seen++ {
			if i, ok := start[cur]; ok {
				return i, true
			}
			p, ok := byPID[cur]
			if !ok || p.PPID == cur {
				return 0, false
			}
			cur = p.PPID
		}
		return 0, false
	}

	type agg struct {
		count int
		rss   uint64
		cpu   float64
	}
	aggs := make([]agg, len(roots))
	present := make([]bool, len(roots))

	s.mu.Lock()
	defer s.mu.Unlock()
	next := make(map[int]prevCPU, len(procs))
	// Jendela sejak sampel sebelumnya (detik); 0 bila ini sampel pertama.
	window := 0.0
	if !s.lastAt.IsZero() {
		window = at.Sub(s.lastAt).Seconds()
	}
	for _, p := range procs {
		i, ok := owner(p.PID)
		if !ok {
			continue
		}
		present[i] = true
		a := &aggs[i]
		a.count++
		a.rss += p.RSSKB * 1024
		if pv, had := s.prev[p.PID]; had {
			// Delta dibatasi >= 0 (PID dipakai ulang), pembagian nol dilindungi.
			if d, w := p.CPUSeconds-pv.cpu, at.Sub(pv.at).Seconds(); d > 0 && w > 0 {
				a.cpu += d / w * 100
			}
		} else if window > 0 && p.CPUSeconds > 0 && p.ElapsedSeconds <= window {
			// Lahir di dalam jendela ini: seluruh waktu CPU-nya adalah delta.
			a.cpu += p.CPUSeconds / window * 100
		}
		next[p.PID] = prevCPU{cpu: p.CPUSeconds, at: at}
	}
	s.prev = next
	s.lastAt = at

	snap := Snapshot{At: at.UnixMilli(), Procs: []Proc{}}
	for i, r := range roots {
		if _, ok := byPID[r.PID]; !ok || !present[i] {
			continue
		}
		pct := aggs[i].cpu
		snap.Procs = append(snap.Procs, Proc{
			Label: r.Label, Kind: r.Kind, SessionID: r.SessionID, PID: r.PID,
			Count: aggs[i].count, RSSBytes: aggs[i].rss, CPUPercent: pct,
		})
		snap.TotalRSSBytes += aggs[i].rss
		snap.TotalCPUPercent += pct
	}
	rank := map[string]int{"daemon": 0, "app": 1}
	sort.SliceStable(snap.Procs, func(a, b int) bool {
		pa, pb := snap.Procs[a], snap.Procs[b]
		ra, oa := rank[pa.Kind]
		rb, ob := rank[pb.Kind]
		if !oa {
			ra = 2
		}
		if !ob {
			rb = 2
		}
		if ra != rb {
			return ra < rb
		}
		return ra == 2 && pa.RSSBytes > pb.RSSBytes
	})
	return snap
}
