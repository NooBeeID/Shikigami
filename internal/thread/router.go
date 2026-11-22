package thread

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// RouterConfig konfigurasi router pesan thread.
type RouterConfig struct {
	Manager *Manager
	Guard   *AntiLoop
	Logger  *slog.Logger
}

type routerImpl struct {
	mgr   *Manager
	guard *AntiLoop
	log   *slog.Logger
}

var _ Router = (*routerImpl)(nil)

// NewRouter membuat Router. Manager wajib; Guard boleh nil.
func NewRouter(cfg RouterConfig) Router {
	if cfg.Manager == nil {
		panic("thread.NewRouter: Manager wajib diisi")
	}
	lg := cfg.Logger
	if lg == nil {
		lg = slog.Default()
	}
	return &routerImpl{mgr: cfg.Manager, guard: cfg.Guard, log: lg}
}

func routerName(p Participant) string {
	if p.Name != "" {
		return p.Name
	}
	return p.SessionID
}

func routerFindParticipant(t Thread, sessionID string) (Participant, bool) {
	return antiloopFindParticipant(t, sessionID)
}

func routerNames(ps []Participant) string {
	names := make([]string, 0, len(ps))
	for _, p := range ps {
		names = append(names, routerName(p))
	}
	return strings.Join(names, ", ")
}

func (r *routerImpl) routerResolveTargets(t Thread, from, to string) ([]Participant, string, error) {
	q := strings.ToLower(strings.TrimSpace(to))
	q = strings.TrimSpace(strings.TrimPrefix(q, "@"))
	if q == "" {
		return nil, "", fmt.Errorf("%w: target kosong", ErrTargetUnknown)
	}
	switch q {
	case ToUser:
		return nil, ToUser, nil
	case ToAll:
		var out []Participant
		for _, p := range t.Participants {
			if p.SessionID != from {
				out = append(out, p)
			}
		}
		if len(out) == 0 {
			return nil, "", fmt.Errorf("%w: tidak ada peserta lain", ErrTargetUnknown)
		}
		return out, ToAll, nil
	}

	var matches []Participant
	if q == RoleMaster {
		for _, p := range t.Participants {
			if p.Role == RoleMaster {
				matches = append(matches, p)
			}
		}
	} else {
		projName := map[string]string{}
		if r.mgr.cfg.Workspaces != nil {
			if projs, ok := r.mgr.cfg.Workspaces.Projects(t.WorkspaceID); ok {
				for _, pi := range projs {
					projName[pi.ID] = pi.Name
				}
			}
		}
		for _, p := range t.Participants {
			cands := []string{p.Name, p.SessionID, p.ProjectID}
			if n := projName[p.ProjectID]; n != "" && p.ProjectID != "" {
				cands = append(cands, n, Slugify(n))
			}
			for _, c := range cands {
				if c != "" && strings.ToLower(c) == q {
					matches = append(matches, p)
					break
				}
			}
		}
	}
	// dedup per SessionID
	seen := map[string]bool{}
	uniq := matches[:0:0]
	for _, p := range matches {
		if !seen[p.SessionID] {
			seen[p.SessionID] = true
			uniq = append(uniq, p)
		}
	}
	switch len(uniq) {
	case 0:
		return nil, "", fmt.Errorf("%w: %q; peserta: %s", ErrTargetUnknown, to, routerNames(t.Participants))
	case 1:
		if uniq[0].SessionID == from {
			return nil, "", fmt.Errorf("%w: tidak bisa mengirim pesan ke diri sendiri", ErrInvalid)
		}
		return uniq, uniq[0].SessionID, nil
	default:
		return nil, "", fmt.Errorf("%w: %q cocok dengan: %s", ErrTargetAmbiguous, to, routerNames(uniq))
	}
}

// Route mencatat pesan ke log thread lalu meneruskannya ke target.
func (r *routerImpl) Route(ctx context.Context, threadID, from, to, text string) (ThreadMessage, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return ThreadMessage{}, fmt.Errorf("%w: teks kosong", ErrInvalid)
	}
	t, err := r.mgr.Get(threadID)
	if err != nil {
		return ThreadMessage{}, err
	}
	if t.State != StateActive {
		return ThreadMessage{}, ErrClosed
	}
	sendFrom := from
	if from == FromUser {
		sendFrom = ""
	} else if _, ok := routerFindParticipant(t, from); !ok {
		return ThreadMessage{}, ErrNotParticipant
	}
	if r.guard != nil {
		if err := r.guard.Check(t, from); err != nil {
			return ThreadMessage{}, err
		}
	}
	targets, logTo, err := r.routerResolveTargets(t, from, to)
	if err != nil {
		return ThreadMessage{}, err
	}
	rec, err := r.mgr.Post(threadID, ThreadMessage{From: from, To: logTo, Text: text, Kind: KindMessage})
	if err != nil {
		return ThreadMessage{}, err
	}
	var errs []error
	for _, tg := range targets {
		if ctx.Err() != nil {
			errs = append(errs, ctx.Err())
			break
		}
		if _, err := r.mgr.SendToSession(threadID, sendFrom, tg.SessionID, "[thread] "+text); err != nil {
			name := routerName(tg)
			r.log.Warn("thread: gagal meneruskan pesan", "thread", threadID, "to", tg.SessionID, "err", err)
			r.mgr.PostSystem(threadID, "gagal meneruskan ke "+name+": "+err.Error())
			errs = append(errs, fmt.Errorf("gagal meneruskan ke %s: %w", name, err))
		}
	}
	if len(errs) > 0 {
		return rec, errors.Join(errs...)
	}
	return rec, nil
}
