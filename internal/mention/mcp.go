package mention

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// init mengisi hook startMCP dari types.go agar wire_mention.go tidak perlu diubah.
func init() { startMCP = startServer }

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// startServer menjalankan server MCP loopback dan mengembalikan URL-nya.
func startServer(ctx context.Context, rt *Runtime) (string, error) {
	if rt == nil || rt.Sessions == nil || rt.Namer == nil {
		return "", fmt.Errorf("%w: runtime mention tidak lengkap", ErrInvalid)
	}
	logger := rt.Logger
	if logger == nil {
		logger = slog.Default()
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	url := "http://" + ln.Addr().String() + "/mcp"

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", handler(rt))
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	if rt.DataDir != "" {
		if err := writeURLFile(rt.DataDir, url); err != nil {
			logger.Warn("gagal menulis mention.json", "err", err)
		}
	}

	registerLaunchContribution(rt, url)
	hookSessionExit(rt)

	go func() { _ = srv.Serve(ln) }()
	if done := ctx.Done(); done != nil {
		go func() {
			<-done
			sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = srv.Shutdown(sctx)
		}()
	}
	logger.Info("server MCP mention aktif", "url", url)
	return url, nil
}

func writeURLFile(dir, url string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(map[string]string{"url": url})
	if err != nil {
		return err
	}
	final := filepath.Join(dir, "mention.json")
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func writeRPC(w http.ResponseWriter, id json.RawMessage, result any, rerr *rpcError) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rerr})
}

func bearerToken(r *http.Request) (string, bool) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) < 7 || !strings.EqualFold(h[:7], "bearer ") {
		return "", false
	}
	tok := strings.TrimSpace(h[7:])
	return tok, tok != ""
}

func handler(rt *Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rt.Logger != nil {
					rt.Logger.Error("panic di handler MCP", "panic", rec)
				}
				http.Error(w, "kesalahan internal", http.StatusInternalServerError)
			}
		}()

		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method tidak diizinkan", http.StatusMethodNotAllowed)
			return
		}
		token, ok := bearerToken(r)
		var sid string
		if ok {
			sid, ok = tokens.lookup(token, rt)
		}
		if !ok {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "tidak berwenang", http.StatusUnauthorized)
			return
		}

		body := http.MaxBytesReader(w, r.Body, 1<<20)
		raw, err := io.ReadAll(body)
		if err != nil {
			writeRPC(w, nil, nil, &rpcError{Code: codeParseError, Message: "body tidak dapat dibaca"})
			return
		}
		if strings.HasPrefix(strings.TrimSpace(string(raw)), "[") {
			writeRPC(w, nil, nil, &rpcError{Code: codeInvalidRequest, Message: "batch tidak didukung"})
			return
		}
		var req rpcRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			writeRPC(w, nil, nil, &rpcError{Code: codeParseError, Message: "JSON tidak valid"})
			return
		}
		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		switch req.Method {
		case "initialize":
			writeRPC(w, req.ID, map[string]any{
				"protocolVersion": "2025-03-26",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "aiterm-mention", "version": "1"},
			}, nil)
		case "ping":
			writeRPC(w, req.ID, map[string]any{}, nil)
		case "tools/list":
			writeRPC(w, req.ID, map[string]any{"tools": toolDefs()}, nil)
		case "tools/call":
			var p struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
				writeRPC(w, req.ID, nil, &rpcError{Code: codeInvalidParams, Message: "params tidak valid"})
				return
			}
			c, err := resolveCaller(rt, sid)
			if err != nil {
				writeRPC(w, req.ID, toolError(err), nil)
				return
			}
			writeRPC(w, req.ID, callTool(r.Context(), rt, c, p.Name, p.Arguments), nil)
		default:
			writeRPC(w, req.ID, nil, &rpcError{Code: codeMethodNotFound, Message: "method tidak ditemukan"})
		}
	}
}
