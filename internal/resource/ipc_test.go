package resource

import (
	"os"
	"strings"
	"testing"
)

type fakeSrc map[string]int

func (f fakeSrc) RunningPids() map[string]int { return f }

func TestSnapshotBarisDaemonDanSesi(t *testing.T) {
	src := fakeSrc{"abcdefghijkl": os.Getppid()}
	snap, err := snapshot(NewSampler(nil), src, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var daemon, sesi bool
	for _, p := range snap.Procs {
		if p.Kind == "daemon" && p.PID == os.Getpid() && p.RSSBytes > 0 {
			daemon = true
		}
		if p.Kind == "session" && p.Label == "Sesi abcdefgh" && p.SessionID == "abcdefghijkl" {
			sesi = true
		}
		if strings.HasPrefix(p.Label, "App") {
			t.Error("root App tidak diharapkan tanpa appPid")
		}
	}
	if !daemon {
		t.Error("baris Daemon dengan RSS > 0 tidak ada")
	}
	if !sesi {
		t.Error("baris sesi tidak ada")
	}
}
