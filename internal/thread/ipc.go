package thread

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

func ipcDecode(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return ipc.Errorf(ipc.ErrInvalidParams, "params tidak valid: %v", err)
	}
	return nil
}

func ipcToError(err error) error {
	if err == nil {
		return nil
	}
	var ie *ipc.Error
	if errors.As(err, &ie) {
		return err
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return ipc.NewError(ipc.ErrNotFound, err.Error())
	case errors.Is(err, ErrInvalid):
		return ipc.NewError(ipc.ErrInvalid, err.Error())
	case errors.Is(err, ErrClosed):
		return ipc.NewError(ipc.ErrInvalid, "thread sudah ditutup")
	}
	return ipc.NewError(ipc.ErrInternal, err.Error())
}

func ipcAdapt(h func(json.RawMessage) (any, error)) ipc.Handler {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		v, err := h(raw)
		return v, ipcToError(err)
	}
}

// RegisterMethod mendaftarkan metode IPC tambahan (mis. thread.closePreview/thread.close
// dari tiket thread-close). Aman dipanggil sebelum maupun sesudah RegisterIPC.
func (m *Manager) RegisterMethod(name string, handler func(params json.RawMessage) (any, error)) {
	if strings.TrimSpace(name) == "" {
		panic("thread: nama metode IPC kosong")
	}
	if handler == nil {
		panic(fmt.Sprintf("thread: handler metode %q nil", name))
	}
	m.methodMu.Lock()
	defer m.methodMu.Unlock()
	if _, dup := m.methods[name]; dup {
		panic(fmt.Sprintf("thread: metode IPC %q sudah terdaftar", name))
	}
	m.methods[name] = handler
	if m.srv != nil {
		m.srv.Register(name, ipcAdapt(handler))
	}
}

// RegisterIPC memasang metode thread.* ke server. Dipanggil sekali.
func RegisterIPC(srv *ipc.Server, m *Manager) {
	m.methodMu.Lock()
	m.srv = srv
	for name, h := range m.methods {
		srv.Register(name, ipcAdapt(h))
	}
	m.methodMu.Unlock()

	srv.Register(MethodCreate, func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p CreateParams
		if err := ipcDecode(raw, &p); err != nil {
			return nil, err
		}
		t, err := m.Create(p.WorkspaceID, p.Goal, p.Title)
		if err != nil {
			return nil, ipcToError(err)
		}
		return t, nil
	})
	srv.Register(MethodList, func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p ListParams
		if err := ipcDecode(raw, &p); err != nil {
			return nil, err
		}
		list, err := m.List(p.WorkspaceID)
		if err != nil {
			return nil, ipcToError(err)
		}
		return list, nil
	})
	srv.Register(MethodGet, func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p GetParams
		if err := ipcDecode(raw, &p); err != nil {
			return nil, err
		}
		if p.ID == "" {
			return nil, ipc.NewError(ipc.ErrInvalid, "id kosong")
		}
		t, err := m.Get(p.ID)
		if err != nil {
			return nil, ipcToError(err)
		}
		return t, nil
	})
	srv.Register(MethodMessages, func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p MessagesParams
		if err := ipcDecode(raw, &p); err != nil {
			return nil, err
		}
		var since time.Time
		if p.SinceTS != "" {
			ts, err := time.Parse(time.RFC3339Nano, p.SinceTS)
			if err != nil {
				return nil, ipc.NewError(ipc.ErrInvalid, "sinceTs tidak valid")
			}
			since = ts
		}
		msgs, err := m.Messages(p.ID, since)
		if err != nil {
			return nil, ipcToError(err)
		}
		return msgs, nil
	})
	srv.Register(MethodSend, func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p SendParams
		if err := ipcDecode(raw, &p); err != nil {
			return nil, err
		}
		msg, err := m.Send(p.ID, p.Text)
		if err != nil {
			return nil, ipcToError(err)
		}
		return msg, nil
	})
	srv.Register(MethodOwnedSessions, func(ctx context.Context, raw json.RawMessage) (any, error) {
		ids := m.OwnedSessions()
		if ids == nil {
			ids = []string{}
		}
		return OwnedSessionsResult{SessionIDs: ids}, nil
	})
}
