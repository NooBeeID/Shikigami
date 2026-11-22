package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

func decode(params json.RawMessage, v any) error {
	if len(strings.TrimSpace(string(params))) == 0 || string(params) == "null" {
		return nil
	}
	if err := json.Unmarshal(params, v); err != nil {
		return ipc.Errorf(ipc.ErrInvalidParams, "parameter tidak valid: %v", err)
	}
	return nil
}

// toIPCError memetakan galat store ke kode galat IPC.
func toIPCError(err error) error {
	var ie *ipc.Error
	var ge *guardError
	switch {
	case errors.As(err, &ie):
		return err
	case errors.As(err, &ge):
		return ipc.NewError(ipc.ErrConflict, err.Error())
	case errors.Is(err, ErrNotFound):
		return ipc.NewError(ipc.ErrNotFound, err.Error())
	case errors.Is(err, errStorage):
		return ipc.NewError(ipc.ErrInternal, err.Error())
	default:
		return ipc.NewError(ipc.ErrInvalid, err.Error())
	}
}

// RegisterIPC mendaftarkan method agents.* dan menerbitkan agents.changed
// setelah tiap mutasi sukses.
func RegisterIPC(srv *ipc.Server, s *Store) {
	changed := func() { srv.Publish("agents.changed", map[string]any{}) }

	srv.Register("agents.list", func(ctx context.Context, p json.RawMessage) (any, error) {
		c, a, err := s.Snapshot()
		if err != nil {
			return nil, toIPCError(err)
		}
		return struct {
			Categories []Category `json:"categories"`
			Agents     []Agent    `json:"agents"`
		}{c, a}, nil
	})

	srv.Register("agents.categoryUpsert", func(ctx context.Context, p json.RawMessage) (any, error) {
		var in struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Role string `json:"role"`
		}
		if err := decode(p, &in); err != nil {
			return nil, err
		}
		c, err := s.UpsertCategory(in.ID, in.Name, in.Role)
		if err != nil {
			return nil, toIPCError(err)
		}
		changed()
		return c, nil
	})

	srv.Register("agents.categoryDelete", func(ctx context.Context, p json.RawMessage) (any, error) {
		var in struct {
			ID     string `json:"id"`
			Agents string `json:"agents"`
			MoveTo string `json:"moveTo"`
		}
		if err := decode(p, &in); err != nil {
			return nil, err
		}
		if err := s.DeleteCategory(in.ID, DeleteMode(in.Agents), in.MoveTo); err != nil {
			return nil, toIPCError(err)
		}
		changed()
		return map[string]any{}, nil
	})

	srv.Register("agents.upsert", func(ctx context.Context, p json.RawMessage) (any, error) {
		var in struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			CategoryID string `json:"categoryId"`
			ProviderID string `json:"providerId"`
			Model      string `json:"model"`
			IsDefault  bool   `json:"isDefault"`
		}
		if err := decode(p, &in); err != nil {
			return nil, err
		}
		a, err := s.UpsertAgent(AgentInput{ID: in.ID, Name: in.Name, CategoryID: in.CategoryID, ProviderID: in.ProviderID, Model: in.Model, IsDefault: in.IsDefault})
		if err != nil {
			return nil, toIPCError(err)
		}
		changed()
		return a, nil
	})

	byID := func(op func(string) error) ipc.Handler {
		return func(ctx context.Context, p json.RawMessage) (any, error) {
			var in struct {
				ID string `json:"id"`
			}
			if err := decode(p, &in); err != nil {
				return nil, err
			}
			if err := op(in.ID); err != nil {
				return nil, toIPCError(err)
			}
			changed()
			return map[string]any{}, nil
		}
	}
	srv.Register("agents.delete", byID(s.DeleteAgent))
	srv.Register("agents.setDefault", byID(s.SetDefault))
}
