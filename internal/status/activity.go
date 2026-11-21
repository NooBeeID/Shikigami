package status

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const (
	activityRingCap     = 200 // kapasitas buffer per sesi
	activityListDefault = 100
	activityListMax     = 200
	activitySummaryMax  = 160 // rune
	activityEndedKeep   = 20  // maks buffer sesi ended
	activityEvent       = "PostToolUse"
	activityTopic       = "activity.event"
)

// ActivityEntry adalah satu catatan tool call agent.
type ActivityEntry struct {
	TS        int64  `json:"ts"`        // unix milidetik
	SessionID string `json:"sessionId"` // id sesi daemon
	Tool      string `json:"tool"`
	Summary   string `json:"summary"`
}

type activityBuf struct {
	entries []ActivityEntry // urut lama -> baru
}

// ActivityLog menyimpan buffer in-memory aktivitas per sesi.
type ActivityLog struct {
	srv   *ipc.Server
	mu    sync.Mutex
	bufs  map[string]*activityBuf
	ended []string // urutan MarkEnded, tertua di depan
}

// NewActivityLog membuat log aktivitas kosong.
func NewActivityLog(srv *ipc.Server) *ActivityLog {
	return &ActivityLog{srv: srv, bufs: make(map[string]*activityBuf)}
}

var activityNewlines = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")

// summarize meringkas tool_input menjadi satu baris maks 160 rune.
func summarize(tool string, input map[string]any) string {
	if input == nil {
		return ""
	}
	var s string
	key := ""
	switch tool {
	case "Bash":
		key = "command"
	case "Read", "Edit", "Write", "NotebookEdit":
		key = "file_path"
	case "Grep", "Glob":
		key = "pattern"
	case "WebFetch":
		key = "url"
	case "Task":
		key = "description"
	}
	if key != "" {
		s, _ = input[key].(string)
	} else {
		keys := make([]string, 0, len(input))
		for k := range input {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if v, ok := input[k].(string); ok {
				s = v
				break
			}
		}
	}
	s = activityNewlines.Replace(s)
	if r := []rune(s); len(r) > activitySummaryMax {
		s = string(r[:activitySummaryMax])
	}
	return s
}

// removeEnded menghapus id dari daftar ended (pemanggil memegang lock).
func (a *ActivityLog) removeEnded(id string) {
	for i, v := range a.ended {
		if v == id {
			a.ended = append(a.ended[:i], a.ended[i+1:]...)
			return
		}
	}
}

// Record mencatat event PostToolUse; event lain diabaikan.
func (a *ActivityLog) Record(sessionID, event string, payload map[string]any) {
	defer func() { _ = recover() }()
	if a == nil || event != activityEvent || sessionID == "" {
		return
	}
	tool, ok := payload["tool_name"].(string)
	if !ok || strings.TrimSpace(tool) == "" {
		return
	}
	input, _ := payload["tool_input"].(map[string]any)
	entry := ActivityEntry{
		TS:        time.Now().UnixMilli(),
		SessionID: sessionID,
		Tool:      tool,
		Summary:   summarize(tool, input),
	}
	a.mu.Lock()
	b := a.bufs[sessionID]
	if b == nil {
		b = &activityBuf{}
		a.bufs[sessionID] = b
	}
	if len(b.entries) >= activityRingCap {
		n := make([]ActivityEntry, 0, activityRingCap)
		n = append(n, b.entries[len(b.entries)-activityRingCap+1:]...)
		b.entries = n
	}
	b.entries = append(b.entries, entry)
	a.removeEnded(sessionID)
	a.mu.Unlock()
	if a.srv != nil {
		a.srv.Publish(activityTopic, entry)
	}
}

// List mengembalikan entri terbaru -> terlama; selalu non-nil.
func (a *ActivityLog) List(sessionID string, limit int) []ActivityEntry {
	if limit <= 0 {
		limit = activityListDefault
	}
	if limit > activityListMax {
		limit = activityListMax
	}
	var all []ActivityEntry
	if a != nil {
		a.mu.Lock()
		collect := func(b *activityBuf) {
			for i := len(b.entries) - 1; i >= 0; i-- {
				all = append(all, b.entries[i])
			}
		}
		if sessionID != "" {
			if b := a.bufs[sessionID]; b != nil {
				collect(b)
			}
		} else {
			for _, b := range a.bufs {
				collect(b)
			}
		}
		a.mu.Unlock()
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].TS > all[j].TS })
	if len(all) > limit {
		all = all[:limit]
	}
	out := make([]ActivityEntry, len(all))
	copy(out, all)
	return out
}

// Forget menghapus buffer sesi.
func (a *ActivityLog) Forget(sessionID string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.bufs, sessionID)
	a.removeEnded(sessionID)
}

// MarkEnded menandai sesi selesai dan membatasi jumlah buffer sesi ended.
func (a *ActivityLog) MarkEnded(sessionID string) {
	if a == nil || sessionID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.bufs[sessionID] == nil {
		return
	}
	a.removeEnded(sessionID)
	a.ended = append(a.ended, sessionID)
	for len(a.ended) > activityEndedKeep {
		delete(a.bufs, a.ended[0])
		a.ended = a.ended[1:]
	}
}

// RegisterActivityIPC mendaftarkan method activity.list.
func RegisterActivityIPC(srv *ipc.Server, a *ActivityLog) {
	srv.Register("activity.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			SessionID string `json:"sessionId"`
			Limit     int    `json:"limit"`
		}
		trimmed := bytes.TrimSpace(params)
		if len(trimmed) > 0 && string(trimmed) != "null" {
			if err := json.Unmarshal(trimmed, &p); err != nil {
				return nil, ipc.NewError(ipc.ErrInvalidParams, "parameter tidak valid")
			}
		}
		return a.List(p.SessionID, p.Limit), nil
	})
}
