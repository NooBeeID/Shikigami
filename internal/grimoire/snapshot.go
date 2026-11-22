package grimoire

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/NooBeeID/shikigami/internal/paths"
)

const (
	snapshotMaxAge = 24 * time.Hour
	snapshotPrefix = "grimoire-"
	snapshotSuffix = ".md"
)

var (
	snapshotMu      sync.Mutex
	snapshotWritten = map[string]struct{}{}
)

func snapshotRunDir() (string, error) {
	base, err := paths.DataDir()
	if err != nil {
		return "", fmt.Errorf("gagal menentukan data dir: %w", err)
	}
	return filepath.Join(base, "run"), nil
}

// WriteSnapshot membekukan teks suntikan ke <DataDir>/run/grimoire-<16 hex>.md
// (dir 0700, berkas 0600) dan mencatat path-nya di daftar in-memory proses.
func WriteSnapshot(text string) (string, error) {
	dir, err := snapshotRunDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("gagal membuat folder %s: %w", dir, err)
	}
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("gagal membuat nama acak: %w", err)
	}
	path := filepath.Join(dir, snapshotPrefix+hex.EncodeToString(raw[:])+snapshotSuffix)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return "", fmt.Errorf("gagal menulis snapshot %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("gagal chmod snapshot %s: %w", path, err)
	}
	snapshotMu.Lock()
	snapshotWritten[filepath.Clean(path)] = struct{}{}
	snapshotMu.Unlock()
	return path, nil
}

// PruneSnapshots menghapus grimoire-*.md berkas biasa di <DataDir>/run yang
// berumur > 24 jam dan tidak dipakai. Galat pertama dikembalikan setelah
// semua berkas dicoba.
func PruneSnapshots(inUse func(path string) bool) error {
	dir, err := snapshotRunDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("gagal membaca folder %s: %w", dir, err)
	}
	var first error
	note := func(e error) {
		if first == nil {
			first = e
		}
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, snapshotPrefix) || !strings.HasSuffix(name, snapshotSuffix) || !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			note(fmt.Errorf("gagal membaca info %s: %w", name, err))
			continue
		}
		if time.Since(info.ModTime()) <= snapshotMaxAge {
			continue
		}
		path := filepath.Join(dir, name)
		if inUse != nil {
			if inUse(path) {
				continue
			}
		} else {
			snapshotMu.Lock()
			_, written := snapshotWritten[filepath.Clean(path)]
			snapshotMu.Unlock()
			if written {
				continue
			}
		}
		if err := os.Remove(path); err != nil {
			note(fmt.Errorf("gagal menghapus %s: %w", path, err))
			continue
		}
		snapshotMu.Lock()
		delete(snapshotWritten, filepath.Clean(path))
		snapshotMu.Unlock()
	}
	return first
}

// TOMLString mengubah s menjadi string TOML basic satu baris yang ter-quote.
func TOMLString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r == utf8.RuneError && size == 1 {
			b.WriteRune('�')
			continue
		}
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 || r == 0x7F {
				fmt.Fprintf(&b, "\\u%04X", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
