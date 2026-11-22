package larkdata

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/NooBeeID/shikigami/internal/connector"
)

// ListTimeout adalah batas waktu satu panggilan lark-cli.
const ListTimeout = 20 * time.Second

// Run menjalankan satu perintah baca lark-cli dan mengembalikan stdout (JSON).
// args = perintah + flag tanpa `--as user` dan tanpa flag format.
func Run(ctx context.Context, args ...string) ([]byte, error) {
	s, e := check(args)
	if e != nil {
		return nil, e
	}
	full := make([]string, 0, len(args)+4+len(s.format))
	full = append(full, args...)
	if s.asUser {
		full = append(full, "--as", "user")
	}
	full = append(full, s.format...)
	out, err := connector.RunCLITimeout(ctx, ListTimeout, "lark-cli", full...)
	if err != nil {
		return nil, classify(err)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return nil, &Error{Code: CodeCLI, Message: "Balasan Lark tidak terbaca"}
	}
	return []byte(trimmed), nil
}
