package gitdata

import (
	"context"
	"errors"
	"strings"

	"github.com/NooBeeID/shikigami/internal/connector"
)

// ErrorCode adalah kode galat terklasifikasi.
type ErrorCode string

const (
	CodeCLIMissing  ErrorCode = "cli_missing"
	CodeNotLoggedIn ErrorCode = "not_logged_in"
	CodePermission  ErrorCode = "permission"
	CodeNotFound    ErrorCode = "not_found"
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

// Pola dicocokkan lowercase.
var (
	notLoggedInPatterns = []string{"not logged in", "not logged into", "authentication", "401", "unauthorized", "gh auth login", "glab auth login", "no token found"}
	permissionPatterns  = []string{"403", "forbidden", "permission", "insufficient", "scope"}
	notFoundPatterns    = []string{"404", "not found", "could not resolve", "could not find"}
)

func containsAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func providerLabel(tool string) string {
	switch tool {
	case "glab":
		return "GitLab"
	case "gh":
		return "GitHub"
	}
	return "Git"
}

// summarizeMsg meringkas pesan: newline jadi spasi, maksimal 200 rune.
func summarizeMsg(s string) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
	r := []rune(s)
	if len(r) > 200 {
		return string(r[:200]) + "..."
	}
	return s
}

func classify(err error, tool string) *Error {
	if err == nil {
		return nil
	}
	if e, ok := AsError(err); ok {
		return e
	}
	label := providerLabel(tool)
	if connector.IsCLIMissing(err) {
		return &Error{Code: CodeCLIMissing, Message: tool + " belum terpasang"}
	}
	raw := err.Error()
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || strings.Contains(raw, "melewati batas waktu") {
		return &Error{Code: CodeTimeout, Message: label + " tidak merespons dalam " + itoa(int(CallTimeout.Seconds())) + " detik"}
	}
	msg := strings.TrimSpace(strings.TrimPrefix(raw, "perintah "+tool+" gagal: "))
	lower := strings.ToLower(raw)
	if tool == "glab" || tool == "gh" {
		if containsAny(lower, notLoggedInPatterns) {
			return &Error{Code: CodeNotLoggedIn, Message: "Belum login ke " + label + ". Jalankan `" + tool + " auth login` di Pengaturan > Connector"}
		}
		if containsAny(lower, permissionPatterns) {
			return &Error{Code: CodePermission, Message: "Izin kurang: " + msg}
		}
		if containsAny(lower, notFoundPatterns) {
			return &Error{Code: CodeNotFound, Message: "Repo tidak ditemukan atau tidak ada akses"}
		}
	} else if strings.Contains(lower, "not a git repository") || strings.Contains(lower, "no such remote") {
		return &Error{Code: CodeNotFound, Message: "Repo tidak ditemukan atau tidak ada akses"}
	}
	return &Error{Code: CodeCLI, Message: label + " gagal: " + summarizeMsg(msg)}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
