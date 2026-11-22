package main

import (
	"testing"
	"time"
)

func TestWaitDaemonStartup_MenungguSampaiSelesai(t *testing.T) {
	done := make(chan struct{})
	go func() { time.Sleep(50 * time.Millisecond); close(done) }()
	if !waitDaemonStartup(done, 2*time.Second) {
		t.Fatal("seharusnya menunggu sampai startup selesai")
	}
}

func TestWaitDaemonStartup_Timeout(t *testing.T) {
	if waitDaemonStartup(make(chan struct{}), 30*time.Millisecond) {
		t.Fatal("seharusnya timeout")
	}
}

func TestWaitDaemonStartup_SudahSelesai(t *testing.T) {
	done := make(chan struct{})
	close(done)
	if !waitDaemonStartup(done, time.Hour) {
		t.Fatal("seharusnya langsung true")
	}
}
