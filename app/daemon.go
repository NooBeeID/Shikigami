package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/paths"
)

const (
	daemonBinaryName   = "shikigamid"
	daemonLogName      = "shikigamid.log"
	daemonWaitTimeout  = 5 * time.Second
	daemonWaitInterval = 100 * time.Millisecond
)

var (
	daemonStartupMu  sync.RWMutex
	daemonStartupErr error
	// daemonStartupDone ditutup setelah auto-start awal (init) selesai.
	daemonStartupDone = make(chan struct{})
)

// waitDaemonStartup menunggu auto-start awal selesai (maks timeout). Mengembalikan
// true bila sudah selesai. Dipakai Bridge agar ping pertama frontend tidak
// berlomba dengan spawn daemon.
func waitDaemonStartup(done <-chan struct{}, timeout time.Duration) bool {
	select {
	case <-done:
		return true
	default:
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-done:
		return true
	case <-t.C:
		return false
	}
}

// daemonPing menghubungi daemon dan memanggil method ping satu kali.
func daemonPing(socketPath string) error {
	c, err := ipc.Dial(socketPath)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Call("ping", nil, nil)
}

// findDaemonBinary mencari biner shikigamid di sebelah executable (termasuk
// bundle .app), lalu di PATH.
func findDaemonBinary() (string, error) {
	if exe, err := os.Executable(); err == nil {
		if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
			exe = resolved
		}
		dir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(dir, daemonBinaryName),
			filepath.Join(dir, "..", "Resources", daemonBinaryName),
			filepath.Join(dir, "..", "MacOS", daemonBinaryName),
		}
		for _, c := range candidates {
			st, serr := os.Stat(c)
			if serr == nil && !st.IsDir() && st.Mode().Perm()&0o111 != 0 {
				return c, nil
			}
		}
	}
	if p, err := exec.LookPath(daemonBinaryName); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("biner %s tidak ditemukan (dicari di sebelah aplikasi dan di PATH)", daemonBinaryName)
}

// daemonLogPath mengembalikan path log daemon, atau penanda bila tak tersedia.
func daemonLogPath() string {
	dir, err := paths.DataDir()
	if err != nil {
		return "(log tidak tersedia)"
	}
	return filepath.Join(dir, daemonLogName)
}

// openDaemonLog membuka file log daemon (append); fallback ke /dev/null, lalu nil.
func openDaemonLog() *os.File {
	if dir, err := paths.DataDir(); err == nil {
		f, oerr := os.OpenFile(filepath.Join(dir, daemonLogName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if oerr == nil {
			return f
		}
	}
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return nil
	}
	return f
}

// spawnDaemon menjalankan daemon sebagai proses terlepas (setsid, tanpa Wait).
func spawnDaemon(bin string) error {
	cmd := exec.Command(bin)
	cmd.Stdin = nil
	logf := openDaemonLog()
	if logf != nil {
		cmd.Stdout = logf
		cmd.Stderr = logf
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	err := cmd.Start()
	if logf != nil {
		logf.Close()
	}
	if err != nil {
		return fmt.Errorf("gagal menjalankan %s: %w", bin, err)
	}
	_ = cmd.Process.Release()
	return nil
}

// EnsureDaemon memastikan daemon hidup: ping dulu, bila gagal spawn shikigamid
// lalu tunggu ping berhasil (timeout 5 dtk, interval 100 ms).
func EnsureDaemon(ctx context.Context) error {
	sock, err := paths.SocketPath()
	if err != nil {
		return fmt.Errorf("gagal menentukan path socket daemon: %w", err)
	}
	if daemonPing(sock) == nil {
		return nil
	}
	bin, err := findDaemonBinary()
	if err != nil {
		return err
	}
	if err := spawnDaemon(bin); err != nil {
		return err
	}
	deadline := time.NewTimer(daemonWaitTimeout)
	ticker := time.NewTicker(daemonWaitInterval)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("menunggu daemon dibatalkan: %w", ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("daemon tidak siap dalam %s (socket: %s); periksa log di %s", daemonWaitTimeout, sock, daemonLogPath())
		case <-ticker.C:
			if daemonPing(sock) == nil {
				return nil
			}
		}
	}
}

// DaemonStartupError mengembalikan hasil EnsureDaemon dari init(); nil bila
// sukses atau belum selesai.
func DaemonStartupError() error {
	daemonStartupMu.RLock()
	defer daemonStartupMu.RUnlock()
	return daemonStartupErr
}

// init menjalankan auto-start daemon sekali jalan. Ini pengganti hook
// RegisterStartup yang belum ada di bridge.go; ganti dengan hook itu kelak.
func init() {
	go func() {
		var err error
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("auto-start daemon panic: %v", r)
			}
			daemonStartupMu.Lock()
			daemonStartupErr = err
			daemonStartupMu.Unlock()
			close(daemonStartupDone)
		}()
		err = EnsureDaemon(context.Background())
	}()
}
