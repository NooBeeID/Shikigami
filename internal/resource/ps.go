package resource

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PSLister menjalankan ps dan mem-parse tabel proses. Baris tak valid dilewati.
func PSLister() ([]RawProc, error) {
	cmd := exec.Command("ps", "-axo", "pid=,ppid=,rss=,time=,etime=,command=")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var procs []RawProc
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := parsePSLine(line); ok {
			procs = append(procs, p)
		}
	}
	// ps adalah anak daemon; jangan hitung dirinya sendiri.
	if cmd.Process != nil {
		procs = dropPID(procs, cmd.Process.Pid)
	}
	return procs, nil
}

// dropPID membuang baris dengan PID tertentu.
func dropPID(procs []RawProc, pid int) []RawProc {
	out := procs[:0]
	for _, p := range procs {
		if p.PID != pid {
			out = append(out, p)
		}
	}
	return out
}

// parsePSLine mem-parse satu baris "pid ppid rss time etime command".
func parsePSLine(line string) (RawProc, bool) {
	rest := strings.TrimSpace(line)
	var f [5]string
	for i := 0; i < 5; i++ {
		idx := strings.IndexAny(rest, " \t")
		if idx < 0 {
			return RawProc{}, false
		}
		f[i] = rest[:idx]
		rest = strings.TrimLeft(rest[idx:], " \t")
	}
	pid, err1 := strconv.Atoi(f[0])
	ppid, err2 := strconv.Atoi(f[1])
	rss, err3 := strconv.ParseUint(f[2], 10, 64)
	cpu, err4 := ParsePSTime(f[3])
	el, err5 := ParseElapsed(f[4])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
		return RawProc{}, false
	}
	return RawProc{PID: pid, PPID: ppid, RSSKB: rss, CPUSeconds: cpu, ElapsedSeconds: el, Command: rest}, true
}

// ParsePSTime mem-parse waktu CPU ps: [dd-][hh:]mm:ss[.ff], hasil dalam detik.
func ParsePSTime(s string) (float64, error) {
	orig := s
	var days float64
	if i := strings.Index(s, "-"); i >= 0 {
		d, err := strconv.Atoi(s[:i])
		if err != nil || d < 0 {
			return 0, fmt.Errorf("waktu ps tidak valid: %q", orig)
		}
		days = float64(d)
		s = s[i+1:]
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("waktu ps tidak valid: %q", orig)
	}
	total := 0.0
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("waktu ps tidak valid: %q", orig)
		}
		total = total*60 + v
	}
	return days*86400 + total, nil
}

// ParseElapsed mem-parse etime ps: [[dd-]hh:]mm:ss atau ss saja, hasil dalam detik.
func ParseElapsed(s string) (float64, error) {
	if !strings.Contains(s, ":") && !strings.Contains(s, "-") {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("etime ps tidak valid: %q", s)
		}
		return v, nil
	}
	v, err := ParsePSTime(s)
	if err != nil {
		return 0, fmt.Errorf("etime ps tidak valid: %q", s)
	}
	return v, nil
}
