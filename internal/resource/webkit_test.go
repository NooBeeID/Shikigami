package resource

import (
	"reflect"
	"testing"
)

func TestFilterWebKit(t *testing.T) {
	const app = 100
	procs := []RawProc{
		{PID: 100, Command: "/Applications/Shikigami.app/Contents/MacOS/shikigami"},
		{PID: 201, PPID: 1, Command: "/System/.../com.apple.WebKit.GPU.xpc/Contents/MacOS/com.apple.WebKit.GPU"},
		{PID: 202, PPID: 1, Command: "/System/.../com.apple.WebKit.WebContent"},
		{PID: 203, PPID: 1, Command: "/System/.../com.apple.WebKit.Networking"},
		{PID: 301, PPID: 1, Command: "/System/.../com.apple.WebKit.GPU"},        // milik app lain
		{PID: 302, PPID: 1, Command: "/System/.../com.apple.WebKit.WebContent"}, // pencarian gagal
		{PID: 400, PPID: 100, Command: "/bin/zsh"},                              // bukan WebKit
	}
	resp := map[int]int{100: 100, 201: 100, 202: 100, 203: 100, 301: 634, 400: 100}
	calls := 0
	lookup := func(pid int) (int, bool) {
		calls++
		r, ok := resp[pid]
		return r, ok
	}
	got := filterWebKit(app, procs, lookup)
	if want := []int{201, 202, 203}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if calls != 5 {
		t.Fatalf("lookup dipanggil %d kali, want 5 (hanya baris WebKit)", calls)
	}
}

func TestFilterWebKitNoApp(t *testing.T) {
	procs := []RawProc{{PID: 201, Command: "com.apple.WebKit.GPU"}}
	lookup := func(int) (int, bool) { return 0, true }
	if got := filterWebKit(0, procs, lookup); got != nil {
		t.Fatalf("appPid 0: got %v, want nil", got)
	}
	if got := filterWebKit(100, procs, nil); got != nil {
		t.Fatalf("lookup nil: got %v, want nil", got)
	}
}
