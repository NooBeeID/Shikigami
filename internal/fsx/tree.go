package fsx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

type EntryType string

const (
	EntryFile    EntryType = "file"
	EntryDir     EntryType = "dir"
	EntrySymlink EntryType = "symlink"
)

type Entry struct {
	Name  string    `json:"name"`
	Type  EntryType `json:"type"`
	Size  int64     `json:"size"`
	Mtime int64     `json:"mtime"`
}

// List membaca SATU level folder secara malas, tanpa cache. Entry bernama
// sesuai DefaultIgnore dilewati, symlink dilaporkan tanpa diikuti, dan entry
// yang gagal di-stat dilewati. Hasil: folder dulu, lalu file/symlink,
// alfabetis tanpa peka huruf besar. Selalu non-nil.
func List(root, relDir string) ([]Entry, error) {
	abs, err := ResolveUnder(root, relDir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, Wrap(ErrNotFound, "folder %q tidak ada", relDir)
		}
		return nil, fmt.Errorf("stat %q: %w", relDir, err)
	}
	if !info.IsDir() {
		return nil, Wrap(ErrInvalid, "%q bukan folder", relDir)
	}
	dirents, err := os.ReadDir(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, Wrap(ErrNotFound, "folder %q tidak ada", relDir)
		}
		return nil, fmt.Errorf("baca folder %q: %w", relDir, err)
	}
	entries := make([]Entry, 0, len(dirents))
	for _, de := range dirents {
		name := de.Name()
		if IsIgnored(name) {
			continue
		}
		fi, err := de.Info()
		if err != nil {
			continue
		}
		var t EntryType
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			t = EntrySymlink
		case fi.IsDir():
			t = EntryDir
		default:
			t = EntryFile
		}
		var size int64
		if t != EntryDir {
			size = fi.Size()
		}
		entries = append(entries, Entry{Name: name, Type: t, Size: size, Mtime: fi.ModTime().UnixNano()})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		ad, bd := a.Type == EntryDir, b.Type == EntryDir
		if ad != bd {
			return ad
		}
		la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name)
		if la != lb {
			return la < lb
		}
		return a.Name < b.Name
	})
	return entries, nil
}
