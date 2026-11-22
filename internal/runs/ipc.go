package runs

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/session"
)

// sessionLookup dipenuhi *status.Engine.
type sessionLookup interface {
	Lookup(sessionID string) (session.Session, bool)
}

// SessionIDsOfWorkspace membangun himpunan ID sesi milik workspace dari sesi
// yang dikenal registry dan di-resolve Engine ke workspace itu.
func SessionIDsOfWorkspace(reg *Registry, eng sessionLookup, workspaceID string) map[string]bool {
	set := map[string]bool{}
	for _, run := range reg.List(nil) {
		if set[run.SessionID] {
			continue
		}
		if s, ok := eng.Lookup(run.SessionID); ok && s.WorkspaceID == workspaceID {
			set[run.SessionID] = true
		}
	}
	return set
}

// reconcileStep adalah hasil membaca berkas untuk satu run.
type reconcileStep struct {
	run   Run
	d     Detail
	known bool
}

type reconcilePlan []reconcileStep

// scanReconcile membaca berkas Claude (I/O) tanpa mengubah registry.
func scanReconcile(reg *Registry, eng sessionLookup, workspaceID string) reconcilePlan {
	var plan reconcilePlan
	for _, run := range reg.List(nil) {
		s, known := eng.Lookup(run.SessionID)
		if known && s.WorkspaceID != workspaceID {
			continue
		}
		plan = append(plan, reconcileStep{run: run, d: ReadDetail(reg.TranscriptPath(run.SessionID), run), known: known})
	}
	return plan
}

// apply menerapkan hasil baca ke registry (cepat, hanya memori). Mengembalikan
// ID sesi yang berubah, terurut.
func (plan reconcilePlan) apply(reg *Registry) []string {
	changed := map[string]bool{}
	for _, st := range plan {
		run, d := st.run, st.d
		switch {
		case d.Finished:
			if reg.Remove(run.ID) {
				changed[run.SessionID] = true
			}
		case d.Found:
			if !run.Confirmed || !reflect.DeepEqual(run.Progress, d.Progress) {
				reg.MarkConfirmed(run.ID, d.Progress)
				changed[run.SessionID] = true
			}
			// Identitas sub agent dari meta.json mengalahkan tebakan FIFO.
			if run.Kind == KindSubagent && d.Description != "" {
				if reg.SetIdentity(run.ID, truncate(d.Description, 60)) {
					changed[run.SessionID] = true
				}
			}
		case !st.known:
			if reg.Remove(run.ID) {
				changed[run.SessionID] = true
			}
		}
	}
	out := make([]string, 0, len(changed))
	for sid := range changed {
		out = append(out, sid)
	}
	sort.Strings(out)
	return out
}

// Reconcile mencocokkan run workspace dengan berkas Claude: run selesai dibuang,
// run yang ditemukan ditandai Confirmed beserta Progress, run tak ditemukan
// yang sesinya sudah tak dikenal dibuang. Mengembalikan ID sesi yang berubah.
// Versi tanpa lock bersama; jalur produksi memakai Notifier.Reconcile.
func Reconcile(reg *Registry, eng sessionLookup, workspaceID string) []string {
	return scanReconcile(reg, eng, workspaceID).apply(reg)
}

// RegisterIPC mendaftarkan runs.list dan runs.detail. n (boleh nil) menjalankan
// rekonsiliasi dengan publish terserialisasi bersama listener hook.
func RegisterIPC(srv *ipc.Server, reg *Registry, eng sessionLookup, n *Notifier) {
	workspaceParam := func(params json.RawMessage) (string, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return "", ipc.NewError(ipc.ErrInvalidParams, "parameter tidak valid")
		}
		if p.WorkspaceID == "" {
			return "", ipc.NewError(ipc.ErrInvalidParams, "workspaceId wajib diisi")
		}
		return p.WorkspaceID, nil
	}
	srv.Register("runs.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		ws, err := workspaceParam(params)
		if err != nil {
			return nil, err
		}
		return reg.List(SessionIDsOfWorkspace(reg, eng, ws)), nil
	})
	srv.Register("runs.detail", func(ctx context.Context, params json.RawMessage) (any, error) {
		ws, err := workspaceParam(params)
		if err != nil {
			return nil, err
		}
		if n != nil {
			n.Reconcile(eng, ws)
		} else {
			Reconcile(reg, eng, ws)
		}
		return reg.List(SessionIDsOfWorkspace(reg, eng, ws)), nil
	})
}
