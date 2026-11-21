package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"

	"aiterm/internal/ipc"
)

const sessionUsage = "penggunaan: aitermctl session new|list|attach|kill ..."

type sessionInfo struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	ProjectID   string `json:"projectId"`
	Cwd         string `json:"cwd"`
	ProviderID  string `json:"providerId"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	ExitCode    *int   `json:"exitCode"`
}

func init() {
	RegisterCommand(Command{
		Name:    "session",
		Summary: "Kelola sesi terminal: new|list|attach|kill",
		Run:     runSession,
	})
}

func runSession(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(sessionUsage)
	}
	switch args[0] {
	case "new":
		return sessionNew(ctx, c, args[1:])
	case "list":
		return sessionList(ctx, c, args[1:])
	case "attach":
		return sessionAttach(ctx, c, args[1:])
	case "kill":
		return sessionKill(ctx, c, args[1:])
	}
	return errors.New(sessionUsage)
}

func sessionNew(ctx context.Context, c *ipc.Client, args []string) error {
	fs := flag.NewFlagSet("session new", flag.ContinueOnError)
	cwd := fs.String("cwd", "", "direktori kerja")
	ws := fs.String("workspace", "", "ID workspace")
	proj := fs.String("project", "", "ID project")
	prov := fs.String("provider", "", "ID provider")
	cols := fs.Int("cols", 0, "jumlah kolom")
	rows := fs.Int("rows", 0, "jumlah baris")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cols < 0 || *cols > 65535 || *rows < 0 || *rows > 65535 {
		return errors.New("--cols/--rows harus 0..65535")
	}
	params := map[string]any{}
	if *cwd != "" {
		params["cwd"] = *cwd
	}
	if *ws != "" {
		params["workspaceId"] = *ws
	}
	if *proj != "" {
		params["projectId"] = *proj
	}
	if *prov != "" {
		params["providerId"] = *prov
	}
	if *cols > 0 {
		params["cols"] = *cols
	}
	if *rows > 0 {
		params["rows"] = *rows
	}
	var raw json.RawMessage
	if err := c.Call("session.create", params, &raw); err != nil {
		return err
	}
	var info sessionInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return err
	}
	fmt.Println(info.ID)
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	return printJSON(v)
}

func sessionList(ctx context.Context, c *ipc.Client, args []string) error {
	var out struct {
		Sessions []sessionInfo `json:"sessions"`
	}
	if err := c.Call("session.list", nil, &out); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tCWD\tPROVIDER")
	for _, s := range out.Sessions {
		st := s.Status
		if st == "exited" && s.ExitCode != nil {
			st += fmt.Sprintf(" (kode %d)", *s.ExitCode)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.ID, st, s.Cwd, s.ProviderID)
	}
	return tw.Flush()
}

func sessionKill(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) != 1 {
		return errors.New("penggunaan: aitermctl session kill <id>")
	}
	id := args[0]
	if err := c.Call("session.kill", map[string]any{"id": id}, nil); err != nil {
		return err
	}
	fmt.Printf("sesi %s dihentikan\n", id)
	return nil
}

type sessionOutEvent struct {
	data   []byte
	offset uint64
}

// sessionTermSize membaca ukuran terminal lewat stty.
func sessionTermSize() (cols, rows int, ok bool) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	b, err := cmd.Output()
	if err != nil {
		return 0, 0, false
	}
	parts := strings.Fields(string(b))
	if len(parts) != 2 {
		return 0, 0, false
	}
	r, err1 := strconv.Atoi(parts[0])
	cl, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || r <= 0 || cl <= 0 || r > 65535 || cl > 65535 {
		return 0, 0, false
	}
	return cl, r, true
}

func sessionStdinIsTTY() bool {
	st, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// sessionRawMode mengaktifkan raw mode lewat stty; mengembalikan fungsi pemulih.
func sessionRawMode() (restore func(), ok bool) {
	if !sessionStdinIsTTY() {
		return nil, false
	}
	g := exec.Command("stty", "-g")
	g.Stdin = os.Stdin
	out, err := g.Output()
	if err != nil {
		return nil, false
	}
	saved := strings.TrimSpace(string(out))
	raw := exec.Command("stty", "raw", "-echo")
	raw.Stdin = os.Stdin
	if err := raw.Run(); err != nil {
		return nil, false
	}
	return func() {
		r := exec.Command("stty", saved)
		r.Stdin = os.Stdin
		_ = r.Run()
	}, true
}

func sessionAttach(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) != 1 {
		return errors.New("penggunaan: aitermctl session attach <id>")
	}
	id := args[0]

	var (
		outMu        sync.Mutex
		ready        bool
		attachOffset uint64
		pending      []sessionOutEvent
		exited       bool
		exitCode     int
		detach       bool
		stdinErr     error
		done         = make(chan struct{})
		doneOnce     sync.Once
	)
	closeDone := func() { doneOnce.Do(func() { close(done) }) }

	emit := func(ev sessionOutEvent) { // dipanggil di bawah outMu
		if ev.offset <= attachOffset {
			return
		}
		_, _ = os.Stdout.Write(ev.data)
	}

	unsubOut, err := c.Subscribe("session.output."+id, func(topic string, payload json.RawMessage) {
		var p struct {
			Data   string `json:"data"`
			Offset uint64 `json:"offset"`
		}
		if json.Unmarshal(payload, &p) != nil {
			return
		}
		b, err := base64.StdEncoding.DecodeString(p.Data)
		if err != nil {
			return
		}
		ev := sessionOutEvent{data: b, offset: p.Offset}
		outMu.Lock()
		defer outMu.Unlock()
		if !ready {
			pending = append(pending, ev)
			return
		}
		emit(ev)
	})
	if err != nil {
		return err
	}
	defer unsubOut()

	unsubExit, err := c.Subscribe("session.exit", func(topic string, payload json.RawMessage) {
		var p struct {
			ID       string `json:"id"`
			ExitCode int    `json:"exitCode"`
		}
		if json.Unmarshal(payload, &p) != nil || p.ID != id {
			return
		}
		outMu.Lock()
		exitCode = p.ExitCode
		exited = true
		outMu.Unlock()
		closeDone()
	})
	if err != nil {
		return err
	}
	defer unsubExit()

	var res struct {
		Session    sessionInfo `json:"session"`
		Scrollback string      `json:"scrollback"`
		Offset     uint64      `json:"offset"`
	}
	if err := c.Call("session.attach", map[string]any{"id": id}, &res); err != nil {
		return err
	}
	sb, _ := base64.StdEncoding.DecodeString(res.Scrollback)

	if res.Session.Status == "exited" {
		_, _ = os.Stdout.Write(sb)
		fmt.Fprintln(os.Stderr, "[sesi berakhir]")
		code := 0
		if res.Session.ExitCode != nil {
			code = *res.Session.ExitCode
		}
		unsubOut()
		unsubExit()
		if code != 0 {
			os.Exit(code)
		}
		return nil
	}

	outMu.Lock()
	_, _ = os.Stdout.Write(sb)
	attachOffset = res.Offset
	for _, ev := range pending {
		emit(ev)
	}
	pending = nil
	ready = true
	outMu.Unlock()

	fmt.Fprintf(os.Stderr, "terhubung ke sesi %s; tekan Ctrl+] untuk lepas\n", id)
	restore, rawOK := sessionRawMode()
	if !rawOK {
		fmt.Fprintln(os.Stderr, "peringatan: stdin bukan terminal atau stty gagal; memakai mode baris (lepas dengan Ctrl+C)")
	}
	restored := false
	doRestore := func() {
		if rawOK && !restored {
			restored = true
			restore()
		}
	}
	defer doRestore()

	// Ukuran terminal awal + SIGWINCH.
	sendResize := func() {
		if cols, rows, ok := sessionTermSize(); ok {
			_ = c.Call("session.resize", map[string]any{"id": id, "cols": cols, "rows": rows}, nil)
		}
	}
	if rawOK {
		sendResize()
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)
	go func() {
		for {
			select {
			case <-sigCh:
				if rawOK {
					sendResize()
				}
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// Loop stdin.
	go func() {
		buf := make([]byte, 4096)
		write := func(b []byte) error {
			return c.Call("session.write", map[string]any{
				"id": id, "data": base64.StdEncoding.EncodeToString(b),
			}, nil)
		}
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				if i := bytes.IndexByte(chunk, 0x1d); i >= 0 {
					if i > 0 {
						_ = write(chunk[:i])
					}
					outMu.Lock()
					detach = true
					outMu.Unlock()
					closeDone()
					return
				}
				if werr := write(chunk); werr != nil {
					select {
					case <-done:
					default:
						outMu.Lock()
						stdinErr = werr
						outMu.Unlock()
					}
					closeDone()
					return
				}
			}
			if err != nil {
				if err == io.EOF || n == 0 {
					outMu.Lock()
					detach = true
					outMu.Unlock()
					closeDone()
				}
				return
			}
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}

	doRestore()
	unsubOut()
	unsubExit()

	outMu.Lock()
	wasExited, code, wasDetach, werr := exited, exitCode, detach, stdinErr
	outMu.Unlock()

	if wasExited {
		fmt.Fprintf(os.Stderr, "\r\n[sesi berakhir, kode %d]\n", code)
		if code != 0 {
			os.Exit(code)
		}
		return nil
	}
	if werr != nil && !wasDetach {
		var ie *ipc.Error
		if errors.As(werr, &ie) && ie.Code == ipc.ErrConflict {
			fmt.Fprintf(os.Stderr, "\r\n[sesi %s tidak berjalan]\n", id)
			return nil
		}
		return werr
	}
	fmt.Fprintf(os.Stderr, "\r\n[lepas dari sesi %s; sesi tetap berjalan]\n", id)
	return nil
}
