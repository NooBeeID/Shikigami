// Package paths menentukan lokasi data dan socket daemon aiterm.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// EnvDataDir adalah env var untuk meng-override DataDir (dev/test).
	EnvDataDir = "AITERM_DATA_DIR"
	// socketName adalah nama berkas unix socket di dalam DataDir.
	socketName = "aitermd.sock"
	// maxSocketPath: batas path unix socket macOS ~104 byte; sisakan margin.
	maxSocketPath = 100
)

// DataDir mengembalikan direktori data aplikasi (dibuat dengan mode 0700).
func DataDir() (string, error) {
	var dir string
	if v := os.Getenv(EnvDataDir); v != "" {
		dir = filepath.Clean(v)
		if !filepath.IsAbs(dir) {
			abs, err := filepath.Abs(dir)
			if err != nil {
				return "", fmt.Errorf("gagal menentukan path absolut %s: %w", dir, err)
			}
			dir = abs
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("tidak bisa menentukan direktori home: %w", err)
		}
		dir = filepath.Join(home, "Library", "Application Support", "aiterm")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("gagal membuat direktori data %s: %w", dir, err)
	}
	if st, err := os.Stat(dir); err == nil && st.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return "", fmt.Errorf("gagal mengatur mode 0700 pada direktori data %s: %w", dir, err)
		}
	}
	return dir, nil
}

// SocketPath mengembalikan path unix socket daemon: <DataDir>/aitermd.sock.
func SocketPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, socketName)
	if len(p) > maxSocketPath {
		return "", fmt.Errorf("path socket terlalu panjang (%d byte, maks %d): %s; pendekkan AITERM_DATA_DIR", len(p), maxSocketPath, p)
	}
	return p, nil
}
