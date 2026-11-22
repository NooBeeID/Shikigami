//go:build unix

package pty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"

	cpty "github.com/creack/pty"
)

type unixPTY struct {
	mu     sync.Mutex // melindungi cmd, master, closed
	cmd    *exec.Cmd
	master *os.File
	closed bool
}

// New mengembalikan implementasi PTY berbasis creack/pty.
func New() PTY { return &unixPTY{} }

func (p *unixPTY) Start(cmd string, args []string, env []string, cwd string, size Size) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil {
		return errors.New("pty sudah dimulai")
	}
	c := exec.Command(cmd, args...)
	c.Env = env
	c.Dir = cwd
	// SysProcAttr dibiarkan nil: creack/pty mengisi Setsid dan Setctty.
	m, err := cpty.StartWithSize(c, &cpty.Winsize{Cols: size.Cols, Rows: size.Rows})
	if err != nil {
		return fmt.Errorf("gagal memulai pty: %w", err)
	}
	p.cmd = c
	p.master = m
	return nil
}

func (p *unixPTY) getMaster() *os.File {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.master
}

func (p *unixPTY) Read(b []byte) (int, error) {
	m := p.getMaster()
	if m == nil {
		return 0, errors.New("pty belum dimulai")
	}
	return m.Read(b)
}

func (p *unixPTY) Write(b []byte) (int, error) {
	m := p.getMaster()
	if m == nil {
		return 0, errors.New("pty belum dimulai")
	}
	return m.Write(b)
}

func (p *unixPTY) Resize(size Size) error {
	p.mu.Lock()
	m, closed := p.master, p.closed
	p.mu.Unlock()
	if m == nil || closed {
		return errors.New("pty tidak aktif")
	}
	return cpty.Setsize(m, &cpty.Winsize{Cols: size.Cols, Rows: size.Rows})
}

// Pid mengembalikan PID proses anak, atau 0 bila belum start.
func (p *unixPTY) Pid() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p *unixPTY) Close() error {
	p.mu.Lock()
	if p.closed || p.cmd == nil {
		p.closed = true
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	pid := p.cmd.Process.Pid
	master := p.master
	p.mu.Unlock()

	// Setsid membuat PGID == PID; error (mis. ESRCH) diabaikan.
	_ = syscall.Kill(-pid, syscall.SIGHUP)
	if err := master.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return err
	}
	return nil
}

// Wait memanggil exec.Cmd.Wait, yang hanya boleh dipanggil sekali per sesi.
// Wait tidak menutup master fd.
func (p *unixPTY) Wait() (int, error) {
	p.mu.Lock()
	cmd := p.cmd
	p.mu.Unlock()
	if cmd == nil {
		return -1, errors.New("pty belum dimulai")
	}
	werr := cmd.Wait()
	if cmd.ProcessState == nil {
		return -1, werr
	}
	code := cmd.ProcessState.ExitCode()
	if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		code = 128 + int(ws.Signal())
	}
	var ee *exec.ExitError
	if werr != nil && !errors.As(werr, &ee) {
		return -1, werr
	}
	return code, nil
}
