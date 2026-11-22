package review

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const (
	// IPCMethodRevert adalah metode IPC untuk mengembalikan file/hunk ke baseline.
	IPCMethodRevert = "review.revert"
	// IPCMethodAccept adalah metode IPC untuk menerima perubahan (baseline maju).
	IPCMethodAccept = "review.accept"
)

type actionsResponse struct {
	OK      bool `json:"ok"`
	Removed bool `json:"removed"`
}

type actionsParams struct {
	SessionID  string   `json:"sessionId"`
	Path       string   `json:"path"`
	HunkIDs    []string `json:"hunkIds"`
	ExpectHash string   `json:"expectHash"`
	Force      bool     `json:"force"`
}

// RegisterActions mendaftarkan metode IPC review.revert dan review.accept.
func RegisterActions(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool) {
	srv.Register(IPCMethodRevert, actionsHandle(srv, store, supportedFn, true))
	srv.Register(IPCMethodAccept, actionsHandle(srv, store, supportedFn, false))
}

func actionsHandle(srv *ipc.Server, store *Store, supportedFn func(string) bool, revert bool) func(context.Context, json.RawMessage) (any, error) {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p actionsParams
		if err := json.Unmarshal(params, &p); err != nil {
			shape := `{"sessionId": string, "path": string, "hunkIds"?: string[]}`
			if revert {
				shape = `{"sessionId": string, "path": string, "hunkIds"?: string[], "expectHash": string, "force"?: bool}`
			}
			return nil, ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh "+shape)
		}
		id := strings.TrimSpace(p.SessionID)
		path := strings.TrimSpace(p.Path)
		expect := strings.TrimSpace(p.ExpectHash)
		if id == "" {
			return nil, ipc.NewError(ipc.ErrInvalid, "sessionId wajib diisi")
		}
		if path == "" {
			return nil, ipc.NewError(ipc.ErrInvalid, "path wajib diisi")
		}
		if supportedFn == nil || !supportedFn(id) {
			return nil, ipc.NewError(ipc.ErrInvalid, "provider sesi tidak mendukung review")
		}
		var (
			removed bool
			err     error
		)
		if revert {
			removed, err = store.Revert(id, path, p.HunkIDs, expect, p.Force)
		} else {
			removed, err = store.Accept(id, path, p.HunkIDs)
		}
		if err != nil {
			return nil, ipc.NewError(ipc.ErrInvalid, err.Error())
		}
		actionsPublish(srv, store, id)
		return actionsResponse{OK: true, Removed: removed}, nil
	}
}

// actionsPublish menerbitkan review.changed dengan jumlah file belum ditinjau.
func actionsPublish(srv *ipc.Server, store *Store, id string) {
	cwd := store.Cwd(id)
	files, _ := store.Files(id, cwd)
	n := 0
	for _, f := range files {
		if !f.Reviewed {
			n++
		}
	}
	srv.Publish(EventChanged, map[string]any{"sessionId": id, "count": n})
}
