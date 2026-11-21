package fsx

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"

	"aiterm/internal/ipc"
)

type listParams struct {
	Root   string `json:"root"`
	RelDir string `json:"relDir"`
}
type readParams struct {
	Root   string `json:"root"`
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
	Limit  int64  `json:"limit"`
}
type writeParams struct {
	Root          string `json:"root"`
	Path          string `json:"path"`
	Content       string `json:"content"`
	ExpectedMtime int64  `json:"expectedMtime"`
}
type watchParams struct {
	Root string `json:"root"`
	Path string `json:"path"`
}
type changedPayload struct {
	Root  string `json:"root"`
	Path  string `json:"path"`
	Op    string `json:"op"`
	Mtime int64  `json:"mtime"`
}

func decode(params json.RawMessage, v any) error {
	if len(params) == 0 {
		return ipc.NewError(ipc.ErrInvalid, "invalid params: kosong")
	}
	if err := json.Unmarshal(params, v); err != nil {
		return ipc.NewError(ipc.ErrInvalid, "invalid params: "+err.Error())
	}
	return nil
}

func toIPCError(err error) error {
	if err == nil {
		return nil
	}
	var fe *Error
	if errors.As(err, &fe) {
		ie := ipc.NewError(fe.Code, err.Error())
		if fe.Data != nil {
			ie.Data = fe.Data
		}
		return ie
	}
	return ipc.NewError(ipc.ErrInternal, err.Error())
}

// Register mendaftarkan method fs.* dan mengembalikan fungsi penutup.
func Register(srv *ipc.Server) (closeFn func()) {
	var mu sync.Mutex
	var w *Watcher

	getWatcher := func() (*Watcher, error) {
		mu.Lock()
		defer mu.Unlock()
		if w != nil {
			return w, nil
		}
		nw, err := NewWatcher(func(root, rel string, ev ChangeEvent) {
			abs := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
			srv.Publish("fs.changed."+abs, changedPayload{root, rel, ev.Op, ev.Mtime})
		})
		if err != nil {
			return nil, err
		}
		w = nw
		return w, nil
	}

	srv.Register("fs.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p listParams
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		entries, err := List(p.Root, p.RelDir)
		if err != nil {
			return nil, toIPCError(err)
		}
		if entries == nil {
			entries = []Entry{}
		}
		return map[string]any{"entries": entries}, nil
	})
	srv.Register("fs.read", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p readParams
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		res, err := Read(p.Root, p.Path, p.Offset, p.Limit)
		if err != nil {
			return nil, toIPCError(err)
		}
		return res, nil
	})
	srv.Register("fs.write", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p writeParams
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		res, err := Write(p.Root, p.Path, p.Content, p.ExpectedMtime)
		if err != nil {
			return nil, toIPCError(err)
		}
		return res, nil
	})
	srv.Register("fs.watch", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p watchParams
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		cw, err := getWatcher()
		if err != nil {
			return nil, toIPCError(err)
		}
		if err := cw.Watch(p.Root, p.Path); err != nil {
			return nil, toIPCError(err)
		}
		return struct{}{}, nil
	})
	srv.Register("fs.unwatch", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p watchParams
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		mu.Lock()
		cw := w
		mu.Unlock()
		if cw == nil {
			return struct{}{}, nil
		}
		if err := cw.Unwatch(p.Root, p.Path); err != nil {
			return nil, toIPCError(err)
		}
		return struct{}{}, nil
	})

	return func() {
		mu.Lock()
		cw := w
		w = nil
		mu.Unlock()
		if cw != nil {
			_ = cw.Close()
		}
	}
}
