package resource

import (
	"context"
	"encoding/json"
	"os"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

// SessionSource menyediakan PID sesi yang sedang berjalan (ID sesi -> PID).
type SessionSource interface {
	RunningPids() map[string]int
}

// RegisterIPC mendaftarkan resource.snapshot. extra boleh nil; bila diisi,
// dipanggil untuk root App dan mengembalikan PID tambahan milik App.
func RegisterIPC(srv *ipc.Server, s *Sampler, src SessionSource, extra func(appPid int, procs []RawProc) []int) {
	srv.Register("resource.snapshot", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			AppPid int `json:"appPid"`
		}
		if len(params) > 0 && string(params) != "null" {
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, ipc.NewError(ipc.ErrInvalidParams, "parameter tidak valid")
			}
		}
		return snapshot(s, src, extra, p.AppPid)
	})
}

func snapshot(s *Sampler, src SessionSource, extra func(int, []RawProc) []int, appPid int) (Snapshot, error) {
	procs, err := PSLister()
	if err != nil {
		return Snapshot{}, ipc.Errorf(ipc.ErrInternal, "gagal menjalankan ps: %v", err)
	}
	roots := []Root{{Label: "Daemon", Kind: "daemon", PID: os.Getpid()}}
	if appPid > 0 {
		r := Root{Label: "App", Kind: "app", PID: appPid}
		if extra != nil {
			r.Extra = extra(appPid, procs)
		}
		roots = append(roots, r)
	}
	if src != nil {
		for id, pid := range src.RunningPids() {
			short := id
			if len(short) > 8 {
				short = short[:8]
			}
			roots = append(roots, Root{Label: "Sesi " + short, Kind: "session", SessionID: id, PID: pid})
		}
	}
	return s.Sample(roots, procs), nil
}
