package larkdata

import (
	"context"
	"errors"
	"strings"

	"github.com/NooBeeID/shikigami/internal/connector"
)

// ErrorCode adalah kode galat terklasifikasi untuk pemanggilan lark-cli.
type ErrorCode string

const (
	CodeCLIMissing  ErrorCode = "cli_missing"
	CodeNotLoggedIn ErrorCode = "not_logged_in"
	CodePermission  ErrorCode = "permission"
	CodeTimeout     ErrorCode = "timeout"
	CodeNotAllowed  ErrorCode = "not_allowed"
	CodeCLI         ErrorCode = "cli_error"
)

// Error adalah galat terklasifikasi dengan pesan berbahasa Indonesia.
type Error struct {
	Code    ErrorCode
	Message string
}

func (e *Error) Error() string { return e.Message }

// AsError mengekstrak *Error dari err bila ada.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Pola dicocokkan tanpa membedakan huruf besar/kecil.
var (
	permissionPatterns  = []string{"missing_scope", "99991679", "99991672", "permission"}
	notLoggedInPatterns = []string{"not_configured", "not configured", "not logged in", "\"type\": \"authorization\"", "login", "unauthorized"}
)

func matchAny(msg string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

func classify(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := AsError(err); ok {
		return e
	}
	if connector.IsCLIMissing(err) {
		return &Error{Code: CodeCLIMissing, Message: "lark-cli belum terpasang"}
	}
	raw := err.Error()
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(raw, "melewati batas waktu") {
		return &Error{Code: CodeTimeout, Message: "Lark tidak merespons dalam 20 detik"}
	}
	msg := strings.TrimPrefix(raw, "perintah lark-cli gagal: ")
	lower := strings.ToLower(raw)
	if matchAny(lower, permissionPatterns) {
		return &Error{Code: CodePermission, Message: "Izin Lark kurang: " + msg}
	}
	if matchAny(lower, notLoggedInPatterns) {
		return &Error{Code: CodeNotLoggedIn, Message: "Belum login ke Lark. Jalankan lark-cli auth login."}
	}
	return &Error{Code: CodeCLI, Message: "Lark gagal: " + msg}
}
