package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

var errClosed = errors.New("koneksi ditutup")

type subscription struct {
	prefix string
	fn     func(string, json.RawMessage)
}

// Client adalah klien JSON-RPC lewat unix socket.
type Client struct {
	nc      net.Conn
	writeMu sync.Mutex
	nextID  int64

	mu       sync.Mutex
	pending  map[int64]chan *message
	subs     map[uint64]*subscription
	nextSub  uint64
	closed   bool
	closeErr error

	qmu    sync.Mutex
	queue  []eventParams
	signal chan struct{}
	done   chan struct{}
}

func Dial(socketPath string) (*Client, error) {
	nc, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("daemon tidak berjalan di %s: %w", socketPath, err)
	}
	c := &Client{
		nc:      nc,
		pending: map[int64]chan *message{},
		subs:    map[uint64]*subscription{},
		signal:  make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	go c.readLoop()
	go c.dispatchLoop()
	return c, nil
}

func (c *Client) readLoop() {
	r := bufio.NewReaderSize(c.nc, 64*1024)
	for {
		line, err := readMessage(r)
		if err != nil {
			c.fail(fmt.Errorf("koneksi ke daemon terputus"))
			return
		}
		var m message
		if json.Unmarshal(line, &m) != nil {
			continue
		}
		if len(m.ID) == 0 {
			if m.Method == "event" {
				var ep eventParams
				if json.Unmarshal(m.Params, &ep) == nil {
					c.qmu.Lock()
					c.queue = append(c.queue, ep)
					c.qmu.Unlock()
					select {
					case c.signal <- struct{}{}:
					default:
					}
				}
			}
			continue
		}
		id, err := strconv.ParseInt(string(m.ID), 10, 64)
		if err != nil {
			continue
		}
		c.mu.Lock()
		ch := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if ch != nil {
			ch <- &m
		}
	}
}

func (c *Client) dispatchLoop() {
	for {
		select {
		case <-c.done:
			return
		case <-c.signal:
		}
		for {
			c.qmu.Lock()
			if len(c.queue) == 0 {
				c.qmu.Unlock()
				break
			}
			ev := c.queue[0]
			c.queue[0] = eventParams{}
			c.queue = c.queue[1:]
			c.qmu.Unlock()

			c.mu.Lock()
			targets := make([]*subscription, 0, len(c.subs))
			for _, sub := range c.subs {
				if strings.HasPrefix(ev.Topic, sub.prefix) {
					targets = append(targets, sub)
				}
			}
			c.mu.Unlock()
			for _, sub := range targets {
				sub.fn(ev.Topic, ev.Payload)
			}
			select {
			case <-c.done:
				return
			default:
			}
		}
	}
}

func (c *Client) fail(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.closeErr = err
	pending := c.pending
	c.pending = map[int64]chan *message{}
	c.mu.Unlock()
	for _, ch := range pending {
		ch <- &message{Error: &Error{Code: ErrInternal, Message: err.Error()}}
	}
	close(c.done)
	c.nc.Close()
}

func (c *Client) Call(method string, params any, out any) error {
	c.mu.Lock()
	if c.closed {
		err := c.closeErr
		c.mu.Unlock()
		return err
	}
	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan *message, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req := message{JSONRPC: jsonrpcVersion, ID: json.RawMessage(strconv.FormatInt(id, 10)), Method: method}
	if params != nil {
		pb, err := json.Marshal(params)
		if err != nil {
			c.mu.Lock()
			delete(c.pending, id)
			c.mu.Unlock()
			return fmt.Errorf("gagal mengenkode params: %w", err)
		}
		req.Params = pb
	}
	b, err := encodeLine(req)
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("gagal mengenkode request: %w", err)
	}
	c.writeMu.Lock()
	_, werr := c.nc.Write(b)
	c.writeMu.Unlock()
	if werr != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		c.fail(fmt.Errorf("koneksi ke daemon terputus"))
		return fmt.Errorf("gagal menulis ke daemon: %w", werr)
	}
	resp := <-ch
	if resp.Error != nil {
		return resp.Error
	}
	if out != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return fmt.Errorf("gagal membaca hasil: %w", err)
		}
	}
	return nil
}

func (c *Client) Subscribe(topicPrefix string, fn func(topic string, payload json.RawMessage)) (unsubscribe func(), err error) {
	c.mu.Lock()
	first := true
	for _, s := range c.subs {
		if s.prefix == topicPrefix {
			first = false
			break
		}
	}
	c.nextSub++
	sid := c.nextSub
	c.subs[sid] = &subscription{prefix: topicPrefix, fn: fn}
	c.mu.Unlock()

	if first {
		if err := c.Call("events.subscribe", prefixParams{Prefix: topicPrefix}, nil); err != nil {
			c.mu.Lock()
			delete(c.subs, sid)
			c.mu.Unlock()
			return nil, err
		}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			delete(c.subs, sid)
			other := false
			for _, s := range c.subs {
				if s.prefix == topicPrefix {
					other = true
					break
				}
			}
			c.mu.Unlock()
			if !other {
				_ = c.Call("events.unsubscribe", prefixParams{Prefix: topicPrefix}, nil)
			}
		})
	}, nil
}

func (c *Client) Close() error {
	c.fail(errClosed)
	return nil
}
