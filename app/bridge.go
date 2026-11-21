package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/paths"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Bridge menghubungkan frontend Wails ke daemon lewat IPC.
type Bridge struct {
	mu     sync.Mutex
	ctxMu  sync.RWMutex
	ctx    context.Context
	client *ipc.Client
	unsub  func()
}

func NewBridge() *Bridge { return &Bridge{} }

func (b *Bridge) SetContext(ctx context.Context) {
	b.ctxMu.Lock()
	b.ctx = ctx
	b.ctxMu.Unlock()
}

// connect harus dipanggil dengan b.mu terkunci.
func (b *Bridge) connect() (*ipc.Client, error) {
	if b.client != nil {
		return b.client, nil
	}
	sock, err := paths.SocketPath()
	if err != nil {
		return nil, fmt.Errorf("gagal menentukan path socket: %w", err)
	}
	c, err := ipc.Dial(sock)
	if err != nil {
		return nil, fmt.Errorf("daemon tidak berjalan di %s", sock)
	}
	unsub, err := c.Subscribe("", b.relay)
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("gagal berlangganan event daemon: %w", err)
	}
	b.client = c
	b.unsub = unsub
	return c, nil
}

// relay tidak boleh mengambil b.mu (dipanggil dari goroutine client).
func (b *Bridge) relay(topic string, payload json.RawMessage) {
	b.ctxMu.RLock()
	ctx := b.ctx
	b.ctxMu.RUnlock()
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, "daemon:event", map[string]any{"topic": topic, "payload": payload})
}

func (b *Bridge) drop(c *ipc.Client) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.client != c {
		return
	}
	if b.unsub != nil {
		b.unsub()
	}
	_ = c.Close()
	b.client = nil
	b.unsub = nil
}

func (b *Bridge) Call(method string, params json.RawMessage) (json.RawMessage, error) {
	b.mu.Lock()
	c, err := b.connect()
	b.mu.Unlock()
	if err != nil {
		return nil, err
	}
	var p any
	if len(params) > 0 && string(params) != "null" {
		p = params
	}
	var out json.RawMessage
	if err := c.Call(method, p, &out); err != nil {
		var ie *ipc.Error
		if errors.As(err, &ie) {
			return nil, err
		}
		b.drop(c)
		return nil, fmt.Errorf("koneksi ke daemon terputus: %w", err)
	}
	if len(out) == 0 {
		return json.RawMessage("null"), nil
	}
	return out, nil
}

func (b *Bridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.unsub != nil {
		b.unsub()
		b.unsub = nil
	}
	if b.client != nil {
		err := b.client.Close()
		b.client = nil
		return err
	}
	return nil
}
