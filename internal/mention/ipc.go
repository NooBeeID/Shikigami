package mention

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"aiterm/internal/ipc"
)

// IPCOptions mengatur RegisterIPC.
type IPCOptions struct {
	SessionCount func() int // jumlah sesi ber-MCP untuk mention.info; nil -> 0
}

func decodeMentionParams(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return ipc.Errorf(ipc.ErrInvalidParams, "params tidak valid: %v", err)
	}
	return nil
}

func toIPCError(err error) error {
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
	case errors.Is(err, ErrInvalid), errors.Is(err, ErrCrossWorkspace), errors.Is(err, ErrUserOnly):
		return ipc.NewError(ipc.ErrInvalid, err.Error())
	case errors.Is(err, ErrConflict):
		return ipc.NewError(ipc.ErrConflict, err.Error())
	}
	return ipc.NewError(ipc.ErrInternal, err.Error())
}

// RegisterIPC mendaftarkan method mention.* ke server IPC.
func RegisterIPC(srv *ipc.Server, q *Queue, namer *Namer, opts IPCOptions) {
	srv.Register("mention.send", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			To          string `json:"to"`
			Text        string `json:"text"`
			From        string `json:"from"`
		}
		if err := decodeMentionParams(raw, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.To) == "" {
			return nil, toIPCError(ErrInvalid)
		}
		ws := p.WorkspaceID
		if ws == "" {
			if sess, err := q.cfg.Sessions.Get(p.To); err == nil && sess != nil {
				ws = sess.WorkspaceID
			} else {
				return nil, ipc.NewError(ipc.ErrInvalid, "workspaceId wajib bila to berupa nama")
			}
		}
		toID, err := namer.Resolve(ws, p.To)
		if err != nil {
			return nil, toIPCError(err)
		}
		var msg Message
		if p.From == "" {
			msg, err = q.Enqueue(ctx, "", toID, p.Text, SourceUser)
		} else {
			fromID, ferr := namer.Resolve(ws, p.From)
			if ferr != nil {
				return nil, toIPCError(ferr)
			}
			msg, err = q.Enqueue(ctx, fromID, toID, p.Text, SourceAgent)
		}
		if err != nil {
			return nil, toIPCError(err)
		}
		return msg, nil
	})

	srv.Register("mention.queue.list", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p struct {
			SessionID string `json:"sessionId"`
		}
		if err := decodeMentionParams(raw, &p); err != nil {
			return nil, err
		}
		if p.SessionID == "" {
			return q.PendingAll(), nil
		}
		return q.Pending(p.SessionID), nil
	})

	srv.Register("mention.queue.cancel", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p struct {
			ID string `json:"id"`
		}
		if err := decodeMentionParams(raw, &p); err != nil {
			return nil, err
		}
		if p.ID == "" {
			return nil, toIPCError(ErrInvalid)
		}
		if err := q.Cancel(p.ID); err != nil {
			return nil, toIPCError(err)
		}
		return map[string]bool{"ok": true}, nil
	})

	srv.Register("mention.confirm", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p struct {
			ID string `json:"id"`
		}
		if err := decodeMentionParams(raw, &p); err != nil {
			return nil, err
		}
		if p.ID == "" {
			return nil, toIPCError(ErrInvalid)
		}
		msg, err := q.Confirm(p.ID)
		if err != nil {
			return nil, toIPCError(err)
		}
		return msg, nil
	})

	srv.Register("mention.names", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
		}
		if err := decodeMentionParams(raw, &p); err != nil {
			return nil, err
		}
		if p.WorkspaceID == "" {
			return nil, toIPCError(ErrInvalid)
		}
		return namer.List(p.WorkspaceID), nil
	})

	srv.Register("mention.config", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p struct {
			Mode string `json:"mode"`
		}
		if err := decodeMentionParams(raw, &p); err != nil {
			return nil, err
		}
		if p.Mode != "" {
			if err := q.SetMode(Mode(p.Mode)); err != nil {
				return nil, toIPCError(err)
			}
		}
		return map[string]any{"mode": q.Mode()}, nil
	})

	srv.Register("mention.info", func(ctx context.Context, raw json.RawMessage) (any, error) {
		n := 0
		if opts.SessionCount != nil {
			n = opts.SessionCount()
		}
		return map[string]any{"url": URL(), "sessions": n}, nil
	})
}
