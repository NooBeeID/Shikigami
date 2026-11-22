package thread

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/status"
)

// Caller mengidentifikasi pemanggil tool (di-resolve dari token MCP).
type Caller struct {
	ThreadID, SessionID string
	Master              bool
}

// ToolFunc adalah handler tool; keluaran string -> teks, selain itu JSON.
type ToolFunc func(ctx context.Context, c Caller, args json.RawMessage) (any, error)

// ToolDef mendefinisikan satu tool MCP thread.
type ToolDef struct {
	Name, Description string
	InputSchema       map[string]any
	MasterOnly        bool
	Handler           ToolFunc
}

var (
	mcpToolsMu sync.Mutex
	mcpTools   []ToolDef
)

// RegisterTool mendaftarkan tool global; nama kosong/duplikat atau Handler nil -> panic.
func RegisterTool(d ToolDef) {
	if d.Name == "" {
		panic("thread.RegisterTool: Name kosong")
	}
	if d.Handler == nil {
		panic(fmt.Sprintf("thread.RegisterTool: Handler nil untuk tool %q", d.Name))
	}
	mcpToolsMu.Lock()
	defer mcpToolsMu.Unlock()
	for _, t := range mcpTools {
		if t.Name == d.Name {
			panic(fmt.Sprintf("thread.RegisterTool: tool %q sudah terdaftar", d.Name))
		}
	}
	mcpTools = append(mcpTools, d)
}

func registeredTools() []ToolDef {
	mcpToolsMu.Lock()
	out := make([]ToolDef, len(mcpTools))
	copy(out, mcpTools)
	mcpToolsMu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ToolDeps adalah dependensi tool umum.
type ToolDeps struct {
	Manager *Manager
	Router  Router
	Logger  *slog.Logger
}

// mcpToolText membentuk teks hasil tool: string apa adanya, selain itu JSON.
func mcpToolText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

var mcpCommonOnce sync.Once

// RegisterCommonTools mendaftarkan thread_participants, thread_send, thread_read (sekali).
func RegisterCommonTools(d ToolDeps) {
	if d.Manager == nil {
		panic("thread.RegisterCommonTools: Manager nil")
	}
	mcpCommonOnce.Do(func() {
		mgr := d.Manager
		router := d.Router

		RegisterTool(ToolDef{
			Name:        "thread_participants",
			Description: "Daftar peserta thread ini (nama, peran, project, status kerja/idle). Self true untuk dirimu.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
			Handler: func(ctx context.Context, c Caller, args json.RawMessage) (any, error) {
				t, err := mgr.Get(c.ThreadID)
				if err != nil {
					return nil, err
				}
				if t.State == StateClosed {
					return nil, ErrClosed
				}
				views := make([]ParticipantView, 0, len(t.Participants))
				add := func(p Participant) {
					st := "kerja"
					if status.IsIdle(p.SessionID) {
						st = "idle"
					}
					proj := ""
					if p.Role != RoleMaster {
						proj = mcpProjectName(mgr, t, p.ProjectID)
					}
					views = append(views, ParticipantView{
						Name: p.Name, SessionID: p.SessionID, Role: p.Role,
						Project: proj, Status: st, Self: p.SessionID == c.SessionID,
					})
				}
				for _, p := range t.Participants {
					if p.Role == RoleMaster {
						add(p)
					}
				}
				for _, p := range t.Participants {
					if p.Role != RoleMaster {
						add(p)
					}
				}
				return views, nil
			},
		})

		RegisterTool(ToolDef{
			Name:        "thread_send",
			Description: "Kirim pesan ke peserta thread. to: nama peserta, sessionId, \"master\", \"user\" atau \"all\".",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"to":   map[string]any{"type": "string", "description": "nama peserta, master, user, atau all"},
					"text": map[string]any{"type": "string", "description": "isi pesan"},
				},
				"required": []string{"to", "text"},
			},
			Handler: func(ctx context.Context, c Caller, args json.RawMessage) (any, error) {
				var a struct{ To, Text string }
				if len(args) > 0 {
					if err := json.Unmarshal(args, &a); err != nil {
						return nil, fmt.Errorf("argumen tidak valid: %w", err)
					}
				}
				to := strings.TrimSpace(a.To)
				text := strings.TrimSpace(a.Text)
				if to == "" {
					return nil, errors.New("argumen to wajib diisi")
				}
				if text == "" {
					return nil, errors.New("argumen text wajib diisi")
				}
				t, err := mgr.Get(c.ThreadID)
				if err != nil {
					return nil, err
				}
				if t.State == StateClosed {
					return nil, ErrClosed
				}
				if router == nil {
					return nil, errors.New("router belum terpasang")
				}
				msg, err := router.Route(ctx, c.ThreadID, c.SessionID, to, a.Text)
				if err != nil {
					return nil, err
				}
				if msg.To == ToUser {
					return "dicatat (user)", nil
				}
				if t2, err := mgr.Get(c.ThreadID); err == nil {
					t = t2
				}
				return "terkirim ke " + mcpDisplayName(t, msg.To), nil
			},
		})

		RegisterTool(ToolDef{
			Name:        "thread_read",
			Description: "Baca log thread ini (pesan dan pesan sistem terakhir, urut naik).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"last_n": map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "default": 20},
				},
			},
			Handler: func(ctx context.Context, c Caller, args json.RawMessage) (any, error) {
				var a struct {
					LastN *int `json:"last_n"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &a); err != nil {
						return nil, fmt.Errorf("argumen tidak valid: %w", err)
					}
				}
				n := 20
				if a.LastN != nil {
					n = *a.LastN
				}
				if n < 1 {
					n = 1
				}
				if n > 200 {
					n = 200
				}
				msgs, err := mgr.Messages(c.ThreadID, time.Time{})
				if err != nil {
					return nil, err
				}
				out := make([]ThreadMessage, 0, len(msgs))
				for _, m := range msgs {
					if m.Kind == KindMessage || m.Kind == KindSystem {
						out = append(out, m)
					}
				}
				if len(out) > n {
					out = out[len(out)-n:]
				}
				return out, nil
			},
		})
	})
}

// mcpProjectName mencari nama project dari ProjectID; fallback ke ProjectID.
func mcpProjectName(m *Manager, t Thread, projectID string) string {
	if projectID == "" {
		return ""
	}
	if m.cfg.Workspaces != nil {
		if ps, ok := m.cfg.Workspaces.Projects(t.WorkspaceID); ok {
			for _, p := range ps {
				if p.ID == projectID {
					return p.Name
				}
			}
		}
	}
	return projectID
}

// mcpDisplayName menamai target pesan untuk konfirmasi.
func mcpDisplayName(t Thread, to string) string {
	if to == ToAll {
		return "semua peserta"
	}
	for _, p := range t.Participants {
		if p.SessionID == to {
			if p.Name != "" {
				return p.Name
			}
			return to
		}
	}
	return to
}

// AgentToolsSection menghasilkan teks tool untuk prompt master.
// Catatan: tiket 04 menambah kalimat propose_close.
func AgentToolsSection() string {
	return strings.Join([]string{
		"Kamu punya server MCP `shikigami-thread` dengan tool berikut:",
		"- thread_spawn(project, brief, agent?, category?): men-spawn agent peer di project workspace, di git worktree dan branch milik thread (auto-approve). agent = nama agent, category = id/nama kategori (mis. planner); keduanya opsional, kosong berarti agent default Executor. Project non-git ditolak; spawn ulang project yang sama dengan agent yang sama memakai peserta yang sudah ada, agent berbeda membuat peserta baru.",
		"- thread_send(to, text): kirim pesan ke peserta (nama, `master`, `user`, atau `all`). Balasan peserta tiba di terminalmu sebagai `[pesan dari <nama>] [thread] ...`.",
		"- thread_read(last_n): baca log thread terakhir.",
		"- thread_participants(): lihat peserta beserta status kerja/idle.",
		"Pesan dari user datang dengan awalan `[pesan dari user]`: teruskan ke peserta yang sesuai lewat thread_send, lalu laporkan hasilnya ke user dengan thread_send(to=\"user\", ...).",
		"Berkomunikasilah hanya lewat tool ini, bukan dengan mengetik ke terminal agent lain.",
		"Perhatian: bila terjadi pesan beruntun antar agent, thread bisa dijeda sampai ada pesan dari user atau master.",
	}, "\n")
}

// mcpAdaptManagerTools mengadaptasi registri Manager.Tools() menjadi ToolDef.
func mcpAdaptManagerTools(m *Manager) []ToolDef {
	if m == nil {
		return []ToolDef{}
	}
	specs := m.Tools().Tools()
	out := make([]ToolDef, 0, len(specs))
	for _, spec := range specs {
		spec := spec
		out = append(out, ToolDef{
			Name:        spec.Name,
			Description: spec.Description,
			InputSchema: map[string]any{"type": "object", "additionalProperties": true},
			MasterOnly:  spec.MasterOnly,
			Handler: func(ctx context.Context, c Caller, args json.RawMessage) (any, error) {
				m := map[string]any{}
				if t := strings.TrimSpace(string(args)); t != "" && t != "null" {
					if err := json.Unmarshal(args, &m); err != nil {
						return nil, fmt.Errorf("argumen tidak valid: %w", err)
					}
					if m == nil {
						m = map[string]any{}
					}
				}
				role := RolePeer
				if c.Master {
					role = RoleMaster
				}
				return spec.Handler(ctx, ThreadCaller{ThreadID: c.ThreadID, SessionID: c.SessionID, Role: role}, m)
			},
		})
	}
	return out
}

// mcpMergeTools menyatukan registri global dan adaptasi Manager.Tools().
func mcpMergeTools(m *Manager) []ToolDef {
	out := registeredTools()
	seen := make(map[string]bool, len(out))
	for _, t := range out {
		seen[t.Name] = true
	}
	for _, t := range mcpAdaptManagerTools(m) {
		if seen[t.Name] {
			if m != nil {
				m.log.Warn("tool thread ganda; registri global dipakai", "tool", t.Name)
			}
			continue
		}
		seen[t.Name] = true
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
