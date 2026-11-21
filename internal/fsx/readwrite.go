package fsx

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type Kind string

const (
	KindMarkdown Kind = "markdown"
	KindText     Kind = "text"
)

type ReadResult struct {
	Content string `json:"content"`
	Mtime   int64  `json:"mtime"`
	Kind    Kind   `json:"kind"`
	Size    int64  `json:"size"`
	Offset  int64  `json:"offset"`
	Length  int64  `json:"length"`
	EOF     bool   `json:"eof"`
}

type WriteResult struct {
	Mtime int64 `json:"mtime"`
	Size  int64 `json:"size"`
}

// ResolveUnder menggabungkan root dan rel, menolak keluar dari root (termasuk lewat symlink),
// dan mengembalikan path absolut hasil Clean (bukan hasil EvalSymlinks).
func ResolveUnder(root, rel string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", Wrap(ErrInvalid, "root harus absolut: %q", root)
	}
	root = filepath.Clean(root)
	st, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", Wrap(ErrNotFound, "root tidak ada: %s", root)
		}
		return "", Wrap(ErrInvalid, "root tidak dapat diakses: %v", err)
	}
	if !st.IsDir() {
		return "", Wrap(ErrNotFound, "root bukan folder: %s", root)
	}
	if strings.ContainsRune(rel, 0) {
		return "", Wrap(ErrInvalid, "path mengandung NUL")
	}
	rel = filepath.FromSlash(rel)
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, string(filepath.Separator)) {
		return "", Wrap(ErrInvalid, "path tidak boleh absolut: %q", rel)
	}
	abs := filepath.Clean(filepath.Join(root, rel))
	if !under(root, abs) {
		return "", Wrap(ErrInvalid, "path keluar dari root: %q", rel)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", Wrap(ErrInvalid, "root tidak dapat diselesaikan: %v", err)
	}
	// Cari ancestor terdekat yang ada, lalu evaluasi symlink.
	probe := abs
	for {
		real, err := filepath.EvalSymlinks(probe)
		if err == nil {
			if !under(realRoot, real) {
				return "", Wrap(ErrInvalid, "path keluar dari root lewat symlink: %q", rel)
			}
			break
		}
		// Komponen ada (Lstat berhasil) tetapi tidak dapat diselesaikan: symlink putus
		// atau error lain (mis. permission). Tolak.
		if _, lerr := os.Lstat(probe); lerr == nil {
			return "", Wrap(ErrInvalid, "path tidak dapat diselesaikan: %v", err)
		}
		parent := filepath.Dir(probe)
		if parent == probe || !under(root, parent) {
			break
		}
		probe = parent
	}
	return abs, nil
}

func under(base, p string) bool {
	return p == base || strings.HasPrefix(p, base+string(filepath.Separator))
}

func kindFor(name string) Kind {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown", ".txt":
		return KindMarkdown
	}
	return KindText
}

func mtimeNs(fi os.FileInfo) int64 { return fi.ModTime().UnixNano() }

// Read membaca file di bawah root. offset/limit dalam byte; limit <= 0 berarti sampai akhir.
// Rentang diselaraskan ke batas rune UTF-8. File biner ditolak dengan ErrBinary.
func Read(root, relPath string, offset, limit int64) (*ReadResult, error) {
	if offset < 0 || limit < 0 {
		return nil, Wrap(ErrInvalid, "offset/limit negatif")
	}
	abs, err := ResolveUnder(root, relPath)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, Wrap(ErrNotFound, "%s", relPath)
		}
		return nil, Wrap(ErrInvalid, "%v", err)
	}
	if fi.IsDir() {
		return nil, Wrap(ErrInvalid, "%s adalah folder", relPath)
	}
	if !fi.Mode().IsRegular() {
		return nil, Wrap(ErrInvalid, "%s bukan file biasa", relPath)
	}
	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, Wrap(ErrNotFound, "%s", relPath)
		}
		return nil, Wrap(ErrInvalid, "%v", err)
	}
	defer f.Close()
	fi, err = f.Stat()
	if err != nil {
		return nil, Wrap(ErrInvalid, "%v", err)
	}
	size := fi.Size()

	n := size
	if n > binarySampleSize {
		n = binarySampleSize
	}
	sample := make([]byte, n)
	if n > 0 {
		m, rerr := f.ReadAt(sample, 0)
		if rerr != nil && rerr != io.EOF {
			return nil, Wrap(ErrInvalid, "%v", rerr)
		}
		sample = sample[:m]
	}
	if IsBinary(sample) {
		return nil, Wrap(ErrBinary, "%s", relPath)
	}

	start := offset
	if start > size {
		start = size
	}
	end := size
	if limit > 0 && limit < size-start {
		end = start + limit
	}
	partial := start > 0 || end < size
	if partial {
		for i := 0; i < 3 && start < end; i++ {
			var b [1]byte
			if _, rerr := f.ReadAt(b[:], start); rerr != nil {
				break
			}
			if b[0]&0xC0 != 0x80 {
				break
			}
			start++
		}
	}
	var buf []byte
	if end > start {
		buf = make([]byte, end-start)
		if _, rerr := io.ReadFull(io.NewSectionReader(f, start, end-start), buf); rerr != nil {
			return nil, Wrap(ErrInvalid, "%v", rerr)
		}
		if end < size {
			var b [1]byte
			if _, rerr := f.ReadAt(b[:], end); rerr == nil && b[0]&0xC0 == 0x80 {
				// rune terpotong di ujung: pangkas sampai awal rune.
				cut := 0
				for cut < utf8.UTFMax && cut < len(buf) {
					cut++
					if utf8.RuneStart(buf[len(buf)-cut]) {
						break
					}
				}
				buf = buf[:len(buf)-cut]
			}
		}
	}
	if !utf8.Valid(buf) {
		return nil, Wrap(ErrBinary, "%s: bukan UTF-8 valid", relPath)
	}
	length := int64(len(buf))
	return &ReadResult{
		Content: string(buf),
		Mtime:   mtimeNs(fi),
		Kind:    kindFor(relPath),
		Size:    size,
		Offset:  start,
		Length:  length,
		EOF:     start+length >= size,
	}, nil
}

// Write menulis content secara atomik (tmp + fsync + rename) dengan kunci konflik expectedMtime.
// expectedMtime 0 berarti file baru (harus belum ada).
func Write(root, relPath, content string, expectedMtime int64) (*WriteResult, error) {
	if !utf8.ValidString(content) {
		return nil, Wrap(ErrInvalid, "content bukan UTF-8 valid")
	}
	if expectedMtime < 0 {
		return nil, Wrap(ErrInvalid, "expectedMtime negatif")
	}
	abs, err := ResolveUnder(root, relPath)
	if err != nil {
		return nil, err
	}
	if abs == filepath.Clean(root) {
		return nil, Wrap(ErrInvalid, "target adalah root")
	}
	dir := filepath.Dir(abs)
	dst, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, Wrap(ErrNotFound, "folder induk tidak ada")
		}
		return nil, Wrap(ErrInvalid, "%v", err)
	}
	if !dst.IsDir() {
		return nil, Wrap(ErrInvalid, "induk bukan folder")
	}
	if li, lerr := os.Lstat(abs); lerr == nil && li.Mode()&os.ModeSymlink != 0 {
		return nil, Wrap(ErrInvalid, "menulis lewat symlink tidak didukung")
	}
	perm := os.FileMode(0o644)
	fi, err := os.Stat(abs)
	switch {
	case err == nil:
		if fi.IsDir() || !fi.Mode().IsRegular() {
			return nil, Wrap(ErrInvalid, "%s bukan file biasa", relPath)
		}
		if expectedMtime == 0 || expectedMtime != mtimeNs(fi) {
			return nil, NewConflict(mtimeNs(fi))
		}
		perm = fi.Mode().Perm()
	case os.IsNotExist(err):
		if expectedMtime != 0 {
			return nil, Wrap(ErrNotFound, "%s hilang dari disk", relPath)
		}
	default:
		return nil, Wrap(ErrInvalid, "%v", err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(abs)+".shikigami-tmp-*")
	if err != nil {
		return nil, Wrap(ErrInvalid, "gagal membuat file sementara: %v", err)
	}
	committed, closed := false, false
	defer func() {
		if !closed {
			tmp.Close()
		}
		if !committed {
			os.Remove(tmp.Name())
		}
	}()
	if _, err := tmp.WriteString(content); err != nil {
		return nil, Wrap(ErrInvalid, "gagal menulis: %v", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		return nil, Wrap(ErrInvalid, "gagal menulis: %v", err)
	}
	if err := tmp.Sync(); err != nil {
		return nil, Wrap(ErrInvalid, "gagal menulis: %v", err)
	}
	closed = true
	if err := tmp.Close(); err != nil {
		return nil, Wrap(ErrInvalid, "gagal menulis: %v", err)
	}
	if err := os.Rename(tmp.Name(), abs); err != nil {
		return nil, Wrap(ErrInvalid, "gagal menulis: %v", err)
	}
	committed = true
	if d, derr := os.Open(dir); derr == nil {
		d.Sync()
		d.Close()
	}
	fi2, err := os.Stat(abs)
	if err != nil {
		return nil, Wrap(ErrInvalid, "%v", err)
	}
	return &WriteResult{Mtime: mtimeNs(fi2), Size: fi2.Size()}, nil
}
