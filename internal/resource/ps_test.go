package resource

import "testing"

func TestParsePSTime(t *testing.T) {
	kasus := []struct {
		masuk string
		want  float64
	}{
		{"0:01.50", 1.5},
		{"1:02.25", 62.25},
		{"1:02:03.00", 3723},
		{"2-01:00:00", 176400},
	}
	for _, k := range kasus {
		got, err := ParsePSTime(k.masuk)
		if err != nil {
			t.Fatalf("%q: error tak terduga: %v", k.masuk, err)
		}
		if got != k.want {
			t.Errorf("%q: got %v, want %v", k.masuk, got, k.want)
		}
	}
	if _, err := ParsePSTime("abc"); err == nil {
		t.Error("\"abc\" seharusnya error")
	}
}

func TestParsePSLine(t *testing.T) {
	p, ok := parsePSLine("  123   1  2048  0:01.50 01:05 /usr/bin/zsh -l")
	if !ok {
		t.Fatal("baris valid ditolak")
	}
	want := RawProc{PID: 123, PPID: 1, RSSKB: 2048, CPUSeconds: 1.5, ElapsedSeconds: 65, Command: "/usr/bin/zsh -l"}
	if p != want {
		t.Errorf("got %+v, want %+v", p, want)
	}
	if _, ok := parsePSLine("bukan baris"); ok {
		t.Error("baris tak valid seharusnya dilewati")
	}
}

func TestParseElapsed(t *testing.T) {
	kasus := []struct {
		masuk string
		want  float64
	}{
		{"42", 42},
		{"01:05", 65},
		{"1:02:03", 3723},
		{"2-01:00:00", 176400},
	}
	for _, k := range kasus {
		got, err := ParseElapsed(k.masuk)
		if err != nil || got != k.want {
			t.Errorf("%q: got %v err %v, want %v", k.masuk, got, err, k.want)
		}
	}
	for _, bad := range []string{"", "abc", "1:x", "-5"} {
		if _, err := ParseElapsed(bad); err == nil {
			t.Errorf("%q seharusnya error", bad)
		}
	}
}

func TestDropPID(t *testing.T) {
	got := dropPID([]RawProc{{PID: 1}, {PID: 2}, {PID: 3}}, 2)
	if len(got) != 2 || got[0].PID != 1 || got[1].PID != 3 {
		t.Errorf("got %+v", got)
	}
}
