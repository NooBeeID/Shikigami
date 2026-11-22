package gitdata

import (
	"context"
	"time"

	"github.com/NooBeeID/shikigami/internal/connector"
)

// CallTimeout adalah batas waktu satu pemanggilan CLI.
const CallTimeout = 20 * time.Second

// MaxConcurrent adalah jumlah maksimum pemanggilan CLI bersamaan.
const MaxConcurrent = 3

var runSlots = make(chan struct{}, MaxConcurrent)

func acquire(ctx context.Context) error {
	select {
	case runSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func release() { <-runSlots }

// Run menjalankan satu perintah baca. tool = "glab" | "gh" | "git".
// Perintah di luar allowlist ditolak sebelum proses dijalankan.
func Run(ctx context.Context, tool string, args ...string) ([]byte, error) {
	if e := check(tool, args); e != nil {
		return nil, e
	}
	if err := acquire(ctx); err != nil {
		return nil, classify(err, tool)
	}
	defer release()
	out, err := connector.RunCLITimeout(ctx, CallTimeout, tool, args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, classify(ctx.Err(), tool)
		}
		return nil, classify(err, tool)
	}
	return []byte(out), nil
}
