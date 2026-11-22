// Package resource mengukur CPU dan memori per proses untuk monitor resource.
package resource

// RawProc satu baris tabel proses mentah dari ps.
type RawProc struct {
	PID, PPID  int
	RSSKB      uint64
	CPUSeconds float64
	// ElapsedSeconds umur proses sejak start (etime).
	ElapsedSeconds float64
	Command        string
}

// Lister mengambil tabel proses saat ini.
type Lister func() ([]RawProc, error)

// Root titik agregasi: proses beserta turunannya dihitung ke satu baris.
// Kind: "daemon" | "app" | "session". Extra adalah PID tambahan yang
// dihitung ke root ini beserta turunannya.
type Root struct {
	Label, Kind, SessionID string
	PID                    int
	Extra                  []int
}

// Proc hasil agregasi satu root.
type Proc struct {
	Label      string  `json:"label"`
	Kind       string  `json:"kind"`
	SessionID  string  `json:"sessionId"`
	PID        int     `json:"pid"`
	Count      int     `json:"count"`
	RSSBytes   uint64  `json:"rssBytes"`
	CPUPercent float64 `json:"cpuPercent"`
}

// Snapshot hasil satu sampel. At dalam unix milidetik.
type Snapshot struct {
	At              int64   `json:"at"`
	TotalRSSBytes   uint64  `json:"totalRssBytes"`
	TotalCPUPercent float64 `json:"totalCpuPercent"`
	Procs           []Proc  `json:"procs"`
}
