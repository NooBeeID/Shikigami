package mention

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"aiterm/internal/session"
)

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError"`
}

func toolDefs() []toolDef {
	return []toolDef{
		{
			Name:        "list_sessions",
			Description: "Daftar sesi yang sedang berjalan di workspace yang sama, lengkap dengan nama mention.",
			InputSchema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "send_message",
			Description: "Kirim pesan ke sesi lain di workspace yang sama. Pesan diantrekan dan disuntik saat sesi target idle.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"to":   map[string]any{"type": "string", "description": "Nama mention atau id sesi target"},
					"text": map[string]any{"type": "string", "description": "Isi pesan"},
				},
				"required": []string{"to", "text"},
			},
		},
		{
			Name:        "read_output",
			Description: "Baca baris terakhir output terminal sesi lain di workspace yang sama (tanpa escape ANSI).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"to":     map[string]any{"type": "string", "description": "Nama mention atau id sesi"},
					"last_n": map[string]any{"type": "integer", "minimum": 1, "maximum": 500, "default": 50},
				},
				"required": []string{"to"},
			},
		},
	}
}

type caller struct {
	SessionID   string
	WorkspaceID string
}

func resolveCaller(rt *Runtime, sessionID string) (caller, error) {
	s, err := rt.Sessions.Get(sessionID)
	if err != nil || s == nil || s.Status != session.StatusRunning {
		return caller{}, ErrNotFound
	}
	return caller{SessionID: s.ID, WorkspaceID: s.WorkspaceID}, nil
}

func toolText(s string) toolResult {
	return toolResult{Content: []toolContent{{Type: "text", Text: s}}}
}

func toolError(err error) toolResult {
	return toolResult{Content: []toolContent{{Type: "text", Text: err.Error()}}, IsError: true}
}

func toolJSON(v any) toolResult {
	b, err := json.Marshal(v)
	if err != nil {
		return toolError(err)
	}
	return toolText(string(b))
}

func callTool(ctx context.Context, rt *Runtime, c caller, name string, args json.RawMessage) toolResult {
	if len(strings.TrimSpace(string(args))) == 0 {
		args = json.RawMessage("{}")
	}
	switch name {
	case "list_sessions":
		return toolListSessions(rt, c)
	case "send_message":
		return toolSendMessage(ctx, rt, c, args)
	case "read_output":
		return toolReadOutput(rt, c, args)
	default:
		return toolError(fmt.Errorf("tool tidak dikenal: %s", name))
	}
}

type sessionEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProjectName string `json:"projectName"`
	ProviderID  string `json:"providerId"`
	Status      string `json:"status"`
	Self        bool   `json:"self"`
}

func toolListSessions(rt *Runtime, c caller) toolResult {
	infos := rt.Namer.List(c.WorkspaceID)
	out := make([]sessionEntry, 0, len(infos))
	for _, in := range infos {
		out = append(out, sessionEntry{
			ID:          in.SessionID,
			Name:        in.Name,
			ProjectName: in.ProjectName,
			ProviderID:  in.ProviderID,
			Status:      in.Status,
			Self:        in.SessionID == c.SessionID,
		})
	}
	return toolJSON(out)
}

func toolSendMessage(ctx context.Context, rt *Runtime, c caller, args json.RawMessage) toolResult {
	var a struct {
		To   string `json:"to"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return toolError(errors.New("argumen tidak valid"))
	}
	if strings.TrimSpace(a.To) == "" {
		return toolError(fmt.Errorf("%w: to wajib diisi", ErrInvalid))
	}
	if strings.TrimSpace(a.Text) == "" {
		return toolError(fmt.Errorf("%w: text wajib tidak kosong", ErrInvalid))
	}
	if rt.Enqueuer == nil {
		return toolError(errors.New("antrean tidak tersedia"))
	}
	target, err := rt.Namer.Resolve(c.WorkspaceID, a.To)
	if err != nil {
		return toolError(err)
	}
	if target == c.SessionID {
		return toolError(errors.New("tidak dapat mengirim pesan ke sesi sendiri"))
	}
	msg, err := rt.Enqueuer.Enqueue(ctx, c.SessionID, target, a.Text, SourceAgent)
	if err != nil {
		return toolError(err)
	}
	pos := 0
	if rt.Reader != nil {
		pos = rt.Reader.Position(msg.ID)
	}
	return toolJSON(struct {
		Queued    bool `json:"queued"`
		Delivered bool `json:"delivered"`
		Position  int  `json:"position"`
	}{
		Queued:    msg.State == StateQueued || msg.State == StateAwaitingConfirm,
		Delivered: msg.State == StateDelivered,
		Position:  pos,
	})
}

func toolReadOutput(rt *Runtime, c caller, args json.RawMessage) toolResult {
	var a struct {
		To    string `json:"to"`
		LastN *int   `json:"last_n"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		if strings.Contains(err.Error(), "last_n") {
			return toolError(errors.New("last_n tidak valid"))
		}
		return toolError(errors.New("argumen tidak valid"))
	}
	if strings.TrimSpace(a.To) == "" {
		return toolError(fmt.Errorf("%w: to wajib diisi", ErrInvalid))
	}
	n := 50
	if a.LastN != nil && *a.LastN > 0 {
		n = *a.LastN
	}
	if n > 500 {
		n = 500
	}
	target, err := rt.Namer.Resolve(c.WorkspaceID, a.To)
	if err != nil {
		return toolError(err)
	}
	res, err := rt.Sessions.Attach(target)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return toolError(ErrNotFound)
		}
		return toolError(fmt.Errorf("gagal membaca output: %w", err))
	}
	return toolText(lastLines(stripANSI(string(res.Scrollback)), n))
}

var (
	oscRe  = regexp.MustCompile("\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)")
	csiRe  = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")
	escRe  = regexp.MustCompile("\x1b[@-Z\\\\-_]|\x1b[()][0-9A-Za-z]")
	ansiRe = []*regexp.Regexp{oscRe, csiRe, escRe}
)

// stripANSI membuang escape ANSI dan karakter kontrol selain \n dan \t.
func stripANSI(s string) string {
	s = strings.ToValidUTF8(s, "")
	for _, re := range ansiRe {
		s = re.ReplaceAllString(s, "")
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if idx := strings.LastIndex(ln, "\r"); idx >= 0 {
			if seg := ln[idx+1:]; seg != "" {
				ln = seg
			} else {
				ln = strings.ReplaceAll(ln, "\r", "")
			}
			ln = strings.ReplaceAll(ln, "\r", "")
		}
		lines[i] = ln
	}
	s = strings.Join(lines, "\n")
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// lastLines mengambil n baris terakhir.
func lastLines(s string, n int) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
