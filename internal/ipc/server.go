package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

// Handler menangani satu method JSON-RPC.
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

// Server adalah server JSON-RPC lewat unix socket.
type Server struct {
	path     string
	logger   *slog.Logger
	mu       sync.RWMutex
	handlers map[string]Handler
	info     func() map[string]any
	conns    map[*conn]struct{}
	wg       sync.WaitGroup
}

type conn struct {
	srv       *Server
	nc        net.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	writeMu   sync.Mutex
	events    chan []byte
	subMu     sync.Mutex
	subs      map[string]struct{}
	closeOnce sync.Once
}

func NewServer(socketPath string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		path:     socketPath,
		logger:   logger,
		handlers: map[string]Handler{},
		conns:    map[*conn]struct{}{},
	}
	s.registerBuiltins()
	return s
}

func (s *Server) Register(method string, h Handler) {
	if h == nil {
		panic("ipc: handler nil untuk method: " + method)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.handlers[method]; ok {
		panic("ipc: method sudah terdaftar: " + method)
	}
	s.handlers[method] = h
}

func (s *Server) Methods() []string {
	s.mu.RLock()
	out := make([]string, 0, len(s.handlers))
	for k := range s.handlers {
		out = append(out, k)
	}
	s.mu.RUnlock()
	sort.Strings(out)
	return out
}

func (s *Server) SetInfo(fn func() map[string]any) {
	s.mu.Lock()
	s.info = fn
	s.mu.Unlock()
}

func (s *Server) Serve(ctx context.Context) error {
	if _, err := os.Stat(s.path); err == nil {
		if nc, derr := net.DialTimeout("unix", s.path, 2*time.Second); derr == nil {
			nc.Close()
			return fmt.Errorf("daemon sudah berjalan (socket %s)", s.path)
		}
		if rerr := os.Remove(s.path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			return fmt.Errorf("gagal menghapus socket basi %s: %w", s.path, rerr)
		}
	}
	ln, err := net.Listen("unix", s.path)
	if err != nil {
		return fmt.Errorf("gagal membuka socket %s: %w", s.path, err)
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		ln.Close()
		return fmt.Errorf("gagal mengatur izin socket %s: %w", s.path, err)
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			ln.Close()
		case <-done:
		}
	}()

	var serveErr error
	for {
		nc, aerr := ln.Accept()
		if aerr != nil {
			if ctx.Err() != nil {
				break
			}
			var ne net.Error
			if errors.As(aerr, &ne) && ne.Timeout() {
				s.logger.Warn("ipc: accept sementara gagal", "err", aerr)
				continue
			}
			serveErr = fmt.Errorf("ipc: accept gagal: %w", aerr)
			break
		}
		cctx, cancel := context.WithCancel(ctx)
		c := &conn{
			srv:    s,
			nc:     nc,
			ctx:    cctx,
			cancel: cancel,
			events: make(chan []byte, 256),
			subs:   map[string]struct{}{},
		}
		s.mu.Lock()
		s.conns[c] = struct{}{}
		s.mu.Unlock()
		s.wg.Add(1)
		go c.serve()
	}

	ln.Close()
	s.mu.RLock()
	for c := range s.conns {
		c.close()
	}
	s.mu.RUnlock()
	s.wg.Wait()
	os.Remove(s.path)
	s.logger.Info("ipc berhenti")
	return serveErr
}

func (c *conn) serve() {
	s := c.srv
	defer func() {
		c.close()
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
		s.wg.Done()
	}()
	go func() {
		for {
			select {
			case b := <-c.events:
				c.write(b)
			case <-c.ctx.Done():
				return
			}
		}
	}()
	hctx := context.WithValue(c.ctx, connKey{}, c)
	r := bufio.NewReaderSize(c.nc, 64*1024)
	for {
		line, err := readMessage(r)
		if err != nil {
			return
		}
		go c.handle(hctx, line)
	}
}

func (c *conn) handle(ctx context.Context, line []byte) {
	if line[0] == '[' {
		c.write(errorResponse(nil, NewError(ErrInvalidRequest, "batch tidak didukung")))
		return
	}
	var m message
	if err := json.Unmarshal(line, &m); err != nil {
		c.write(errorResponse(nil, NewError(ErrParse, "JSON tidak valid")))
		return
	}
	isNotif := len(m.ID) == 0
	if m.JSONRPC != jsonrpcVersion || m.Method == "" {
		if !isNotif {
			c.write(errorResponse(m.ID, NewError(ErrInvalidRequest, "request tidak valid")))
		} else {
			c.write(errorResponse(nil, NewError(ErrInvalidRequest, "request tidak valid")))
		}
		return
	}
	c.srv.mu.RLock()
	h := c.srv.handlers[m.Method]
	c.srv.mu.RUnlock()
	if h == nil {
		if !isNotif {
			c.write(errorResponse(m.ID, Errorf(ErrMethodNotFound, "method tidak dikenal: %s", m.Method)))
		}
		return
	}
	res, err := c.call(ctx, h, m.Params)
	if isNotif {
		return
	}
	if err != nil {
		c.write(errorResponse(m.ID, toError(err)))
		return
	}
	raw, merr := json.Marshal(res)
	if merr != nil {
		c.write(errorResponse(m.ID, NewError(ErrInternal, "gagal mengenkode hasil: "+merr.Error())))
		return
	}
	c.write(resultResponse(m.ID, raw))
}

func (c *conn) call(ctx context.Context, h Handler, params json.RawMessage) (res any, err error) {
	defer func() {
		if r := recover(); r != nil {
			c.srv.logger.Error("ipc: handler panik", "panic", r, "stack", string(debug.Stack()))
			res, err = nil, NewError(ErrInternal, fmt.Sprintf("galat internal: %v", r))
		}
	}()
	return h(ctx, params)
}

func (c *conn) write(b []byte) {
	c.writeMu.Lock()
	_, err := c.nc.Write(b)
	c.writeMu.Unlock()
	if err != nil {
		c.close()
	}
}

func (c *conn) close() {
	c.closeOnce.Do(func() {
		c.cancel()
		c.nc.Close()
	})
}

func (c *conn) subscribe(prefix string) {
	c.subMu.Lock()
	c.subs[prefix] = struct{}{}
	c.subMu.Unlock()
}

func (c *conn) unsubscribe(prefix string) {
	c.subMu.Lock()
	delete(c.subs, prefix)
	c.subMu.Unlock()
}

func (c *conn) matches(topic string) bool {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	for p := range c.subs {
		if strings.HasPrefix(topic, p) {
			return true
		}
	}
	return false
}

// Publish mengirim event ke semua koneksi yang prefix langganannya cocok. Tidak memblokir.
func (s *Server) Publish(topic string, payload any) {
	pb, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("ipc: gagal mengenkode payload event", "topic", topic, "err", err)
		return
	}
	params, err := json.Marshal(eventParams{Topic: topic, Payload: pb})
	if err != nil {
		s.logger.Error("ipc: gagal mengenkode event", "topic", topic, "err", err)
		return
	}
	b, err := encodeLine(message{JSONRPC: jsonrpcVersion, Method: "event", Params: params})
	if err != nil {
		s.logger.Error("ipc: gagal mengenkode event", "topic", topic, "err", err)
		return
	}
	s.mu.RLock()
	conns := make([]*conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.RUnlock()
	for _, c := range conns {
		if !c.matches(topic) {
			continue
		}
		select {
		case c.events <- b:
		default:
			s.logger.Warn("klien lambat, koneksi dijatuhkan", "topic", topic)
			c.close()
		}
	}
}
