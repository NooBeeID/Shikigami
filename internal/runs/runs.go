// Package runs menyimpan di memori daftar workflow dan sub agent Claude yang
// sedang berjalan, dibangun dari payload hook.
package runs

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
)

type Kind string

const (
	KindWorkflow Kind = "workflow"
	KindSubagent Kind = "subagent"
)

// AgentLine adalah satu baris agent dalam progres workflow.
type AgentLine struct {
	Label string `json:"label"`
	Phase string `json:"phase"`
	Done  bool   `json:"done"`
}

// Progress: workflow = agent selesai/dimulai dari journal; sub agent = checklist.
type Progress struct {
	PhaseTitle string      `json:"phaseTitle,omitempty"`
	Done       int         `json:"done"`
	Total      int         `json:"total"`
	Agents     []AgentLine `json:"agents,omitempty"`
}

// Run adalah satu workflow atau sub agent. ID: tool_use_id (workflow) atau
// agent_id (sub agent). StartedAt dalam unix milidetik.
type Run struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Kind      Kind      `json:"kind"`
	Name      string    `json:"name"`
	Identity  string    `json:"identity"`
	TaskID    string    `json:"taskId,omitempty"`
	RunID     string    `json:"runId,omitempty"`
	AgentID   string    `json:"agentId,omitempty"`
	StartedAt int64     `json:"startedAt"`
	Confirmed bool      `json:"confirmed"`
	Progress  *Progress `json:"progress,omitempty"`
}

// pending adalah Agent yang baru lewat PreToolUse dan belum jadi run.
type pending struct {
	sessionID   string
	toolUseID   string
	description string
	agentID     string // terisi dari PostToolUse bila sudah terkait
	seq         int64
}

type Registry struct {
	mu          sync.Mutex
	runs        map[string]*Run
	pend        []*pending
	transcripts map[string]string
	seq         int64
}

func NewRegistry() *Registry {
	return &Registry{runs: map[string]*Run{}, transcripts: map[string]string{}}
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func obj(m map[string]any, k string) map[string]any {
	o, _ := m[k].(map[string]any)
	return o
}

func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n-1]) + "…"
}

// identityOf menurunkan identitas ringkas dari tool_input.
func identityOf(kind Kind, toolInput map[string]any) string {
	if kind == KindSubagent {
		return truncate(str(toolInput, "description"), 60)
	}
	args, ok := toolInput["args"].(map[string]any)
	if !ok || len(args) == 0 {
		return ""
	}
	if t, ok := args["ticket"].(string); ok && t != "" {
		t = t[strings.LastIndex(t, "/")+1:]
		return truncate(strings.TrimSuffix(t, ".md"), 40)
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	v := args[keys[0]]
	var vs string
	if s, ok := v.(string); ok {
		vs = s
	} else {
		vs = jsonString(v)
	}
	return truncate(keys[0]+"="+vs, 40)
}

func (r *Registry) Apply(sessionID, event string, payload map[string]any) bool {
	if payload == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if tp := str(payload, "transcript_path"); tp != "" {
		r.transcripts[sessionID] = tp
	}
	switch event {
	case "PreToolUse":
		return r.preToolUse(sessionID, payload)
	case "PostToolUse":
		return r.postToolUse(sessionID, payload)
	case "SubagentStart":
		return r.subagentStart(sessionID, payload)
	case "SubagentStop":
		changed := false
		if id := str(payload, "agent_id"); id != "" {
			if run, ok := r.runs[id]; ok && run.Kind == KindSubagent {
				delete(r.runs, id)
				changed = true
			}
		}
		return r.pruneWorkflows(sessionID, payload) || changed
	case "Stop":
		// Akhir giliran: PostToolUse selalu sudah datang sebelum Stop, jadi
		// sisa PreToolUse tanpa pasangan adalah panggilan yang batal.
		dropped := r.dropUnlaunched(sessionID)
		return r.pruneWorkflows(sessionID, payload) || dropped
	}
	return false
}

// dropUnlaunched membuang workflow sesi ini yang tak pernah mendapat TaskID
// (izin ditolak, validasi gagal, Esc) dan semua pending Agent sesi ini yang
// tak pernah menjadi sub agent. Sub agent yang sudah berjalan dan workflow
// ber-TaskID tidak disentuh karena bisa hidup melewati Stop.
func (r *Registry) dropUnlaunched(sid string) bool {
	changed := false
	for id, run := range r.runs {
		if run.SessionID == sid && run.Kind == KindWorkflow && run.TaskID == "" {
			delete(r.runs, id)
			changed = true
		}
	}
	kept := r.pend[:0]
	for _, pe := range r.pend {
		if pe.sessionID != sid {
			kept = append(kept, pe)
		}
	}
	r.pend = kept
	return changed
}

func (r *Registry) preToolUse(sid string, p map[string]any) bool {
	id := str(p, "tool_use_id")
	in := obj(p, "tool_input")
	if id == "" || in == nil {
		return false
	}
	switch str(p, "tool_name") {
	case "Workflow":
		if _, ok := r.runs[id]; ok {
			return false
		}
		r.runs[id] = &Run{
			ID: id, SessionID: sid, Kind: KindWorkflow, Name: "workflow",
			Identity: identityOf(KindWorkflow, in), StartedAt: time.Now().UnixMilli(),
		}
		return true
	case "Agent":
		r.seq++
		r.pend = append(r.pend, &pending{sessionID: sid, toolUseID: id, description: str(in, "description"), seq: r.seq})
	}
	return false
}

func (r *Registry) postToolUse(sid string, p map[string]any) bool {
	id := str(p, "tool_use_id")
	resp := obj(p, "tool_response")
	if id == "" || resp == nil {
		return false
	}
	switch str(p, "tool_name") {
	case "Workflow":
		run, ok := r.runs[id]
		if !ok {
			return false
		}
		if v := str(resp, "workflowName"); v != "" {
			run.Name = v
		}
		run.TaskID = str(resp, "taskId")
		run.RunID = str(resp, "runId")
		return true
	case "Agent":
		agentID := str(resp, "agentId")
		if agentID == "" {
			return false
		}
		for i, pe := range r.pend {
			if pe.toolUseID != id {
				continue
			}
			if run, ok := r.runs[agentID]; ok {
				// SubagentStart sudah lebih dulu: lengkapi identitas lalu buang pending.
				if run.Identity == "" {
					run.Identity = truncate(pe.description, 60)
				}
				r.pend = append(r.pend[:i], r.pend[i+1:]...)
				return true
			}
			pe.agentID = agentID
			return false
		}
	}
	return false
}

func (r *Registry) subagentStart(sid string, p map[string]any) bool {
	agentID := str(p, "agent_id")
	if agentID == "" || str(p, "agent_type") == "workflow-subagent" {
		return false
	}
	if _, ok := r.runs[agentID]; ok {
		return false
	}
	// Pending yang cocok lebih dulu, lalu yang tertua dan belum diklaim.
	pick := -1
	for i, pe := range r.pend {
		if pe.sessionID == sid && pe.agentID == agentID {
			pick = i
			break
		}
	}
	if pick < 0 {
		for i, pe := range r.pend {
			if pe.sessionID == sid && pe.agentID == "" {
				pick = i
				break
			}
		}
	}
	identity := ""
	if pick >= 0 {
		identity = truncate(r.pend[pick].description, 60)
		r.pend = append(r.pend[:pick], r.pend[pick+1:]...)
	}
	name := str(p, "agent_type")
	if name == "" {
		name = "agent"
	}
	r.runs[agentID] = &Run{
		ID: agentID, SessionID: sid, Kind: KindSubagent, Name: name, Identity: identity,
		AgentID: agentID, StartedAt: time.Now().UnixMilli(),
	}
	return true
}

// pruneWorkflows menghapus workflow sesi ini yang TaskID-nya tidak lagi ada di
// background_tasks (type workflow).
func (r *Registry) pruneWorkflows(sid string, p map[string]any) bool {
	list, ok := p["background_tasks"].([]any)
	if !ok {
		return false
	}
	alive := map[string]bool{}
	for _, it := range list {
		m, _ := it.(map[string]any)
		if m != nil && str(m, "type") == "workflow" {
			alive[str(m, "id")] = true
		}
	}
	changed := false
	for id, run := range r.runs {
		if run.SessionID == sid && run.Kind == KindWorkflow && run.TaskID != "" && !alive[run.TaskID] {
			delete(r.runs, id)
			changed = true
		}
	}
	return changed
}

func (r *Registry) List(sessionIDs map[string]bool) []Run {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Run{}
	for _, run := range r.runs {
		if sessionIDs != nil && !sessionIDs[run.SessionID] {
			continue
		}
		c := *run
		c.Progress = copyProgress(run.Progress)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt != out[j].StartedAt {
			return out[i].StartedAt < out[j].StartedAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (r *Registry) Forget(sessionID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := false
	for id, run := range r.runs {
		if run.SessionID == sessionID {
			delete(r.runs, id)
			changed = true
		}
	}
	kept := r.pend[:0]
	for _, pe := range r.pend {
		if pe.sessionID != sessionID {
			kept = append(kept, pe)
		}
	}
	r.pend = kept
	delete(r.transcripts, sessionID)
	return changed
}

func (r *Registry) TranscriptPath(sessionID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.transcripts[sessionID]
}

func (r *Registry) Remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.runs[id]
	delete(r.runs, id)
	return ok
}

// MarkConfirmed menandai run terkonfirmasi (hidup) dan menyimpan progresnya.
func (r *Registry) MarkConfirmed(id string, p *Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if run, ok := r.runs[id]; ok {
		run.Confirmed = true
		run.Progress = copyProgress(p)
	}
}

// SetIdentity mengganti identitas run; true bila nilainya benar-benar berubah.
func (r *Registry) SetIdentity(id, identity string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok || run.Identity == identity {
		return false
	}
	run.Identity = identity
	return true
}

func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// copyProgress menyalin Progress beserta Agents agar tidak berbagi state.
func copyProgress(p *Progress) *Progress {
	if p == nil {
		return nil
	}
	c := *p
	if p.Agents != nil {
		c.Agents = append([]AgentLine(nil), p.Agents...)
	}
	return &c
}
