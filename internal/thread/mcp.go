package thread

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/session"
)

const (
	mcpServerName = "shikigami-thread"
	mcpMaxPending = 64
)

type mcpRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	mcpCodeParseError     = -32700
	mcpCodeInvalidRequest = -32600
	mcpCodeMethodNotFound = -32601
	mcpCodeInvalidParams  = -32602
)

func mcpWriteRPC(w http.ResponseWriter, id json.RawMessage, result any, rerr *mcpRPCError) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(mcpRPCResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rerr})
}

func mcpBearerToken(r *http.Request) (string, bool) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) < 7 || !strings.EqualFold(h[:7], "bearer ") {
		return "", false
	}
	tok := strings.TrimSpace(h[7:])
	return tok, tok != ""
}

func mcpNewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// ---- token manager (per MCPServer) ----

type mcpPendingToken struct {
	token, threadID, workspaceID, projectID, providerID, cwd string
	issuedAt                                                 time.Time
}

type mcpBinding struct{ sessionID, threadID string }

type mcpTokenManager struct {
	mu      sync.Mutex
	pending []mcpPendingToken
	bound   map[string]mcpBinding
	byID    map[string]string
}

func newMCPTokenManager() *mcpTokenManager {
	return &mcpTokenManager{bound: map[string]mcpBinding{}, byID: map[string]string{}}
}

func (t *mcpTokenManager) issue(spec session.SessionSpec, providerID, threadID string) string {
	tok := mcpNewToken()
	if tok == "" {
		return ""
	}
	pid := spec.ProviderID
	if pid == "" {
		pid = providerID
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending = append(t.pending, mcpPendingToken{
		token: tok, threadID: threadID, workspaceID: spec.WorkspaceID, projectID: spec.ProjectID,
		providerID: pid, cwd: spec.Cwd, issuedAt: time.Now(),
	})
	if len(t.pending) > mcpMaxPending {
		t.pending = append([]mcpPendingToken(nil), t.pending[len(t.pending)-mcpMaxPending:]...)
	}
	return tok
}

func (t *mcpTokenManager) lookup(token string, sessions SessionAPI) (sessionID, threadID string, ok bool) {
	if token == "" || sessions == nil {
		return "", "", false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if b, found := t.bound[token]; found {
		s, err := sessions.Get(b.sessionID)
		if err != nil || s == nil || s.Status != session.StatusRunning {
			t.releaseLocked(b.sessionID)
			return "", "", false
		}
		return b.sessionID, b.threadID, true
	}
	found := false
	for _, p := range t.pending {
		if p.token == token {
			found = true
			break
		}
	}
	if !found {
		return "", "", false
	}
	t.bindPendingLocked(sessions)
	if b, ok := t.bound[token]; ok {
		return b.sessionID, b.threadID, true
	}
	return "", "", false
}

func (t *mcpTokenManager) bindPendingLocked(sessions SessionAPI) {
	var free []session.Session
	for _, s := range sessions.List() {
		if s.Status != session.StatusRunning {
			continue
		}
		if _, taken := t.byID[s.ID]; taken {
			continue
		}
		free = append(free, s)
	}
	sort.SliceStable(free, func(i, j int) bool {
		if !free[i].CreatedAt.Equal(free[j].CreatedAt) {
			return free[i].CreatedAt.Before(free[j].CreatedAt)
		}
		return free[i].ID < free[j].ID
	})
	key := func(ws, pr, pv, cwd string) string { return ws + "|" + pr + "|" + pv + "|" + cwd }
	groups := map[string][]session.Session{}
	for _, s := range free {
		k := key(s.WorkspaceID, s.ProjectID, s.ProviderID, s.Cwd)
		groups[k] = append(groups[k], s)
	}
	remaining := t.pending[:0:0]
	for _, p := range t.pending {
		k := key(p.workspaceID, p.projectID, p.providerID, p.cwd)
		if g := groups[k]; len(g) > 0 {
			s := g[0]
			groups[k] = g[1:]
			t.bound[p.token] = mcpBinding{sessionID: s.ID, threadID: p.threadID}
			t.byID[s.ID] = p.token
			continue
		}
		remaining = append(remaining, p)
	}
	t.pending = remaining
}

func (t *mcpTokenManager) release(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.releaseLocked(sessionID)
}

func (t *mcpTokenManager) releaseLocked(sessionID string) {
	if tok, ok := t.byID[sessionID]; ok {
		delete(t.bound, tok)
		delete(t.byID, sessionID)
	}
}

func (t *mcpTokenManager) boundCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.bound)
}

// ---- server ----

// MCPConfig konfigurasi server MCP thread.
type MCPConfig struct {
	Manager  *Manager
	Sessions SessionAPI
	DataDir  string
	Tools    func() []ToolDef
	Logger   *slog.Logger
}

// MCPServer adalah server MCP loopback shikigami-thread.
type MCPServer struct {
	cfg    MCPConfig
	log    *slog.Logger
	tokens *mcpTokenManager

	mu  sync.RWMutex
	url string
	srv *http.Server
}

// NewMCPServer membuat server (belum berjalan).
func NewMCPServer(cfg MCPConfig) *MCPServer {
	lg := cfg.Logger
	if lg == nil {
		lg = slog.Default()
	}
	if cfg.Tools == nil {
		mgr := cfg.Manager
		cfg.Tools = func() []ToolDef { return mcpMergeTools(mgr) }
	}
	return &MCPServer{cfg: cfg, log: lg, tokens: newMCPTokenManager()}
}

// Start menjalankan server dan mengembalikan URL-nya (idempoten).
func (s *MCPServer) Start(ctx context.Context) (string, error) {
	if s.cfg.Manager == nil || s.cfg.Sessions == nil {
		return "", errors.New("server MCP thread: Manager dan Sessions wajib diisi")
	}
	s.mu.Lock()
	if s.url != "" {
		u := s.url
		s.mu.Unlock()
		return u, nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	url := "http://" + ln.Addr().String() + "/mcp"
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.mcpHandle)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	s.url = url
	s.srv = srv
	s.mu.Unlock()

	if s.cfg.DataDir != "" {
		if err := mcpWriteURLFile(s.cfg.DataDir, url); err != nil {
			s.log.Warn("gagal menulis thread-mcp.json", "err", err)
		}
	}
	go func() { _ = srv.Serve(ln) }()
	if done := ctx.Done(); done != nil {
		go func() {
			<-done
			sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = srv.Shutdown(sctx)
		}()
	}
	s.log.Info("server MCP thread aktif", "url", url)
	return url, nil
}

func mcpWriteURLFile(dir, url string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(map[string]string{"url": url})
	if err != nil {
		return err
	}
	final := filepath.Join(dir, "thread-mcp.json")
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// URL mengembalikan URL server ("" sebelum Start).
func (s *MCPServer) URL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.url
}

// ReleaseSession melepas token milik sesi.
func (s *MCPServer) ReleaseSession(sessionID string) { s.tokens.release(sessionID) }

func (s *MCPServer) mcpIssueToken(spec session.SessionSpec, providerID, threadID string) string {
	return s.tokens.issue(spec, providerID, threadID)
}

// mcpResolveCaller membentuk Caller dari binding token; thread selalu dari token.
func (s *MCPServer) mcpResolveCaller(sessionID, threadID string) (Caller, error) {
	t, err := s.cfg.Manager.Get(threadID)
	if err != nil {
		return Caller{}, fmt.Errorf("thread tidak dapat dibaca: %w", err)
	}
	if t.State == StateClosed {
		return Caller{}, ErrClosed
	}
	for _, p := range t.Participants {
		if p.SessionID == sessionID {
			return Caller{ThreadID: threadID, SessionID: sessionID, Master: p.Role == RoleMaster}, nil
		}
	}
	return Caller{}, errors.New("sesi belum terdaftar sebagai peserta thread; coba lagi sebentar")
}

func mcpToolResult(text string, isErr bool) map[string]any {
	r := map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}
	if isErr {
		r["isError"] = true
	}
	return r
}

func (s *MCPServer) mcpHandle(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			s.log.Error("panic di handler MCP thread", "panic", rec)
			http.Error(w, "kesalahan internal", http.StatusInternalServerError)
		}
	}()
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}
	token, ok := mcpBearerToken(r)
	var sid, tid string
	if ok {
		sid, tid, ok = s.tokens.lookup(token, s.cfg.Sessions)
	}
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "tidak berwenang", http.StatusUnauthorized)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		mcpWriteRPC(w, nil, nil, &mcpRPCError{Code: mcpCodeParseError, Message: "body tidak dapat dibaca"})
		return
	}
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "[") {
		mcpWriteRPC(w, nil, nil, &mcpRPCError{Code: mcpCodeInvalidRequest, Message: "batch tidak didukung"})
		return
	}
	var req mcpRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		mcpWriteRPC(w, nil, nil, &mcpRPCError{Code: mcpCodeParseError, Message: "JSON tidak valid"})
		return
	}
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	switch req.Method {
	case "initialize":
		mcpWriteRPC(w, req.ID, map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": mcpServerName, "version": "1"},
		}, nil)
	case "ping":
		mcpWriteRPC(w, req.ID, map[string]any{}, nil)
	case "tools/list":
		caller, _ := s.mcpResolveCaller(sid, tid) // galat -> non-master
		list := []any{}
		for _, t := range s.cfg.Tools() {
			if t.MasterOnly && !caller.Master {
				continue
			}
			schema := t.InputSchema
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			list = append(list, map[string]any{"name": t.Name, "description": t.Description, "inputSchema": schema})
		}
		mcpWriteRPC(w, req.ID, map[string]any{"tools": list}, nil)
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
			mcpWriteRPC(w, req.ID, nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "params tidak valid"})
			return
		}
		mcpWriteRPC(w, req.ID, s.mcpCallTool(r.Context(), sid, tid, p.Name, p.Arguments), nil)
	default:
		mcpWriteRPC(w, req.ID, nil, &mcpRPCError{Code: mcpCodeMethodNotFound, Message: "method tidak ditemukan"})
	}
}

func (s *MCPServer) mcpCallTool(ctx context.Context, sid, tid, name string, args json.RawMessage) (res map[string]any) {
	caller, err := s.mcpResolveCaller(sid, tid)
	if err != nil {
		return mcpToolResult(err.Error(), true)
	}
	var tool *ToolDef
	for _, t := range s.cfg.Tools() {
		if t.Name == name {
			t := t
			tool = &t
			break
		}
	}
	if tool == nil {
		return mcpToolResult("tool tidak dikenal: "+name, true)
	}
	if tool.MasterOnly && !caller.Master {
		return mcpToolResult("tool hanya untuk master", true)
	}
	if t, err := s.cfg.Manager.Get(tid); err != nil {
		return mcpToolResult(err.Error(), true)
	} else if t.State == StateClosed {
		return mcpToolResult("thread sudah ditutup", true)
	}
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	defer func() {
		if rec := recover(); rec != nil {
			s.log.Error("panic di tool thread", "tool", name, "panic", rec)
			res = mcpToolResult(fmt.Sprintf("kesalahan internal pada tool %s", name), true)
		}
	}()
	v, err := tool.Handler(ctx, caller, args)
	if err != nil {
		return mcpToolResult(err.Error(), true)
	}
	return mcpToolResult(mcpToolText(v), false)
}
