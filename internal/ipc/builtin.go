package ipc

import (
	"context"
	"encoding/json"
	"os"
	"time"
)

type connKey struct{}

func connFromContext(ctx context.Context) *conn {
	c, _ := ctx.Value(connKey{}).(*conn)
	return c
}

func (s *Server) registerBuiltins() {
	s.Register("ping", func(ctx context.Context, params json.RawMessage) (any, error) {
		return map[string]any{"pong": true, "ts": time.Now().Format(time.RFC3339Nano)}, nil
	})
	s.Register("daemon.info", func(ctx context.Context, params json.RawMessage) (any, error) {
		out := map[string]any{
			"pid":     os.Getpid(),
			"socket":  s.path,
			"modules": []string{},
		}
		s.mu.RLock()
		fn := s.info
		s.mu.RUnlock()
		if fn != nil {
			for k, v := range fn() {
				out[k] = v
			}
		}
		if m, ok := out["modules"].([]string); ok && m == nil {
			out["modules"] = []string{}
		}
		out["methods"] = s.Methods()
		return out, nil
	})
	s.Register("events.subscribe", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p prefixParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, NewError(ErrInvalidParams, "params tidak valid: butuh {\"prefix\": string}")
		}
		c := connFromContext(ctx)
		if c == nil {
			return nil, NewError(ErrInternal, "koneksi tidak ditemukan")
		}
		c.subscribe(p.Prefix)
		return map[string]any{}, nil
	})
	s.Register("events.unsubscribe", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p prefixParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, NewError(ErrInvalidParams, "params tidak valid: butuh {\"prefix\": string}")
		}
		c := connFromContext(ctx)
		if c == nil {
			return nil, NewError(ErrInternal, "koneksi tidak ditemukan")
		}
		c.unsubscribe(p.Prefix)
		return map[string]any{}, nil
	})
}
