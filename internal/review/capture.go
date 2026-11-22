package review

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NooBeeID/shikigami/internal/fsx"
)

const captureSampleSize = 8 << 10

// captureOutside memeriksa apakah rel berada di luar root.
func captureOutside(rel string) bool {
	return rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// captureRel mengubah path (absolut atau relatif) menjadi path relatif cwd
// dengan pemisah '/'. false bila di luar cwd atau tidak valid.
func captureRel(cwd, path string) (string, bool) {
	if cwd == "" || path == "" {
		return "", false
	}
	cwd = filepath.Clean(cwd)
	var rel string
	if filepath.IsAbs(path) {
		clean := filepath.Clean(path)
		r, err := filepath.Rel(cwd, clean)
		if err != nil {
			return "", false
		}
		rel = r
		if captureOutside(rel) {
			// cwd/path bisa berbeda bentuk karena symlink (mis. /var -> /private/var).
			realCwd, err1 := filepath.EvalSymlinks(cwd)
			realDir, err2 := filepath.EvalSymlinks(filepath.Dir(clean))
			if err1 != nil || err2 != nil {
				return "", false
			}
			r, err := filepath.Rel(realCwd, filepath.Join(realDir, filepath.Base(clean)))
			if err != nil {
				return "", false
			}
			rel = r
		}
	} else {
		rel = filepath.Clean(path)
	}
	if captureOutside(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// captureSample membaca hingga 8 KiB pertama file.
func captureSample(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, captureSampleSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return buf[:n], nil
}

// captureWriteAtomic menulis berkas lewat tmp + rename.
func captureWriteAtomic(dst string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Capture menyalin baseline file pada sentuhan pertama.
func (s *Store) Capture(sessionID, cwd, path string) error {
	if !storeValidID(sessionID) {
		return nil
	}
	rel, ok := captureRel(cwd, path)
	if !ok {
		return nil
	}
	abs, err := fsx.ResolveUnder(cwd, rel)
	if err != nil {
		return nil
	}
	defer s.storeLock(sessionID)()
	idx := s.storeLoad(sessionID)
	for _, t := range idx.Files {
		if t.Path == rel {
			return nil
		}
	}
	entry := TouchedFile{Path: rel}
	fi, err := os.Stat(abs)
	switch {
	case err == nil && !fi.Mode().IsRegular():
		return nil
	case err == nil:
		entry.ExistedBefore = true
		if fi.Size() > maxBaselineBytes {
			sample, err := captureSample(abs)
			if err != nil {
				return nil
			}
			hash, err := storeHashFile(abs)
			if err != nil {
				return nil
			}
			entry.BaselineSha256 = hash
			entry.TooLarge = true
			entry.Binary = fsx.IsBinary(sample)
		} else {
			data, err := os.ReadFile(abs)
			if err != nil {
				return nil
			}
			entry.BaselineSha256 = storeHashBytes(data)
			entry.Binary = fsx.IsBinary(data[:min(len(data), captureSampleSize)])
			if !entry.Binary {
				if err := captureWriteAtomic(s.BaselinePath(sessionID, rel), data); err != nil {
					return fmt.Errorf("review: tulis baseline: %w", err)
				}
			}
		}
	case os.IsNotExist(err):
		// file baru: tanpa salinan
	default:
		return nil
	}
	entry.FirstTouchAt = time.Now().UnixMilli()
	idx.Files = append(idx.Files, entry)
	if idx.Cwd == "" {
		idx.Cwd = filepath.Clean(cwd)
	}
	return s.storeSave(sessionID, idx)
}

// MarkChanged mencatat hash pasca-edit agent.
func (s *Store) MarkChanged(sessionID, cwd, path string) error {
	if !storeValidID(sessionID) {
		return nil
	}
	rel, ok := captureRel(cwd, path)
	if !ok {
		return nil
	}
	abs, err := fsx.ResolveUnder(cwd, rel)
	if err != nil {
		return nil
	}
	defer s.storeLock(sessionID)()
	idx := s.storeLoad(sessionID)
	i := -1
	for n, t := range idx.Files {
		if t.Path == rel {
			i = n
			break
		}
	}
	if i < 0 {
		return nil
	}
	fi, err := os.Stat(abs)
	switch {
	case err == nil && fi.Mode().IsRegular():
		hash, err := storeHashFile(abs)
		if err != nil {
			return nil
		}
		idx.Files[i].ExpectedHash = hash
	case err == nil:
		return nil
	case os.IsNotExist(err):
		idx.Files[i].ExpectedHash = HashAbsent
	default:
		return nil
	}
	return s.storeSave(sessionID, idx)
}
