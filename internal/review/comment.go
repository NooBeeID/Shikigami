package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/NooBeeID/shikigami/internal/fsx"
	"github.com/NooBeeID/shikigami/internal/ipc"
)

// IPCMethodComment adalah nama metode IPC untuk komentar review ke agent.
const IPCMethodComment = "review.comment"

const commentMaxRunes = 2000

const commentBusyMsg = "[BUSY] agent sedang bekerja, tunggu selesai lalu kirim lagi"

// ErrSessionGone dikembalikan adaptor Write/Cwd bila sesi tidak ada atau sudah berakhir.
var ErrSessionGone = errors.New("sesi tidak ada atau sudah berakhir")

// StateReader melaporkan apakah sesi sedang running; known=false bila engine tidak mengenal sesi.
type StateReader func(sessionID string) (running bool, known bool)

// CommentDeps adalah dependensi review.comment.
type CommentDeps struct {
	Cwd   func(sessionID string) (cwd string, err error)
	Write func(sessionID string, data []byte) error
	State StateReader
}

// CommentParams adalah params IPC review.comment.
type CommentParams struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Text      string `json:"text"`
}

// CommentResult adalah hasil IPC review.comment.
type CommentResult struct {
	OK    bool   `json:"ok"`
	Sent  string `json:"sent"`
	Enter bool   `json:"enter"`
}

func commentIsControl(r rune) bool { return r < 0x20 || r == 0x7f }

// commentStripControl membuang karakter kontrol.
func commentStripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if commentIsControl(r) {
			return -1
		}
		return r
	}, s)
}

// commentSanitizeText menormalkan teks menjadi satu baris tanpa kontrol.
func commentSanitizeText(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	prevNL := false
	for _, r := range s {
		if r == '\r' || r == '\n' {
			if !prevNL {
				b.WriteRune(' ')
			}
			prevNL = true
			continue
		}
		prevNL = false
		if commentIsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// FormatComment membentuk satu baris "di <path>:<line>: <text>" tanpa Enter.
func FormatComment(path string, line int, text string) (string, error) {
	p := commentStripControl(strings.TrimSpace(path))
	if p == "" {
		return "", ipc.NewError(ipc.ErrInvalid, "path wajib diisi")
	}
	if line < 1 {
		return "", ipc.NewError(ipc.ErrInvalid, "line harus >= 1")
	}
	t := commentSanitizeText(text)
	if t == "" {
		return "", ipc.NewError(ipc.ErrInvalid, "text wajib diisi")
	}
	return fmt.Sprintf("di %s:%d: %s", p, line, t), nil
}

// Comment memvalidasi, memeriksa state, lalu menulis komentar ke sesi.
func Comment(ctx context.Context, deps CommentDeps, p CommentParams) (CommentResult, error) {
	if err := ctx.Err(); err != nil {
		return CommentResult{}, err
	}
	id := strings.TrimSpace(p.SessionID)
	path := strings.TrimSpace(p.Path)
	text := strings.TrimSpace(p.Text)
	switch {
	case id == "":
		return CommentResult{}, ipc.NewError(ipc.ErrInvalid, "sessionId wajib diisi")
	case path == "":
		return CommentResult{}, ipc.NewError(ipc.ErrInvalid, "path wajib diisi")
	case p.Line < 1:
		return CommentResult{}, ipc.NewError(ipc.ErrInvalid, "line harus >= 1")
	case text == "":
		return CommentResult{}, ipc.NewError(ipc.ErrInvalid, "text wajib diisi")
	case utf8.RuneCountInString(text) > commentMaxRunes:
		return CommentResult{}, ipc.NewError(ipc.ErrInvalid, fmt.Sprintf("text maksimal %d karakter", commentMaxRunes))
	}
	if deps.Cwd == nil || deps.Write == nil {
		return CommentResult{}, ipc.NewError(ipc.ErrInvalid, "review.comment belum dikonfigurasi")
	}
	cwd, err := deps.Cwd(id)
	if err != nil || cwd == "" {
		return CommentResult{}, ipc.NewError(ipc.ErrNotFound, "sesi tidak ditemukan")
	}
	if _, err := fsx.ResolveUnder(cwd, path); err != nil {
		return CommentResult{}, ipc.NewError(ipc.ErrInvalid, "path tidak valid: "+err.Error())
	}
	if deps.State != nil {
		if running, _ := deps.State(id); running {
			return CommentResult{}, ipc.NewError(ipc.ErrConflict, commentBusyMsg)
		}
	}
	sent, err := FormatComment(path, p.Line, p.Text)
	if err != nil {
		return CommentResult{}, err
	}
	if err := deps.Write(id, []byte(sent)); err != nil {
		if errors.Is(err, ErrSessionGone) {
			return CommentResult{}, ipc.NewError(ipc.ErrNotFound, "sesi sudah berakhir")
		}
		return CommentResult{}, ipc.NewError(ipc.ErrInvalid, "gagal menulis ke sesi: "+err.Error())
	}
	return CommentResult{OK: true, Sent: sent, Enter: false}, nil
}

// RegisterCommentIPC mendaftarkan metode IPC review.comment.
func RegisterCommentIPC(srv *ipc.Server, deps CommentDeps) {
	srv.Register(IPCMethodComment, func(ctx context.Context, params json.RawMessage) (any, error) {
		var p CommentParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh {\"sessionId\": string, \"path\": string, \"line\": number, \"text\": string}")
		}
		res, err := Comment(ctx, deps, p)
		if err != nil {
			return nil, err
		}
		return res, nil
	})
}
