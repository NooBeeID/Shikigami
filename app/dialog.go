package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// filterDirs menyaring paths menjadi direktori unik yang sudah dibersihkan.
func filterDirs(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		c := filepath.Clean(p)
		fi, err := os.Stat(c)
		if err != nil || !fi.IsDir() {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

// cleanPaths membersihkan paths: buang yang kosong dan duplikat, urutan asli dipertahankan.
func cleanPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		c := filepath.Clean(p)
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

func init() {
	registerStartupHook(installFileDrop)
}

func installFileDrop(ctx context.Context, b *Bridge) {
	runtime.OnFileDrop(ctx, func(x, y int, paths []string) {
		fmt.Fprintf(os.Stderr, "[DEBUG-DROP] go callback x=%d y=%d paths=%q\n", x, y, paths) // TEMP: hapus setelah debug
		if len(paths) == 0 {
			return
		}
		dirs := filterDirs(paths)
		// paths = folder saja (untuk "buka sebagai project"); all = semua yang dijatuhkan,
		// x/y = titik jatuh (piksel CSS) agar UI bisa menentukan pane tujuannya.
		b.emit(EventFilesDropped, map[string]any{"paths": dirs, "all": cleanPaths(paths), "x": x, "y": y})
	})
}
