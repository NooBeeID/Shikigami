package grimoire

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// HashModule menghitung SHA-256 deterministik atas path, panjang, dan isi
// working tree seluruh berkas modul (urut path). Tidak memakai mtime maupun git.
//
// Isi tiap berkas dicerna dulu (SHA-256 per berkas) sehingga panjang yang
// ditulis selalu sama dengan jumlah byte yang sungguh terbaca, tanpa buffer
// besar di memori.
func HashModule(root string, m Module) (string, error) {
	files := make([]FileInfo, len(m.Files))
	copy(files, m.Files)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	h := sha256.New()
	sep := []byte{0}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f.Path))
		h.Write([]byte(f.Path))
		h.Write(sep)

		st, err := os.Lstat(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				h.Write([]byte("-1"))
				h.Write(sep)
				h.Write(sep)
				continue
			}
			return "", fmt.Errorf("hash %s: %w", f.Path, err)
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("hash %s: symlink tidak diizinkan", f.Path)
		}
		fh, err := os.Open(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				h.Write([]byte("-1"))
				h.Write(sep)
				h.Write(sep)
				continue
			}
			return "", fmt.Errorf("hash %s: %w", f.Path, err)
		}
		fileSum := sha256.New()
		n, err := io.Copy(fileSum, fh)
		fh.Close()
		if err != nil {
			return "", fmt.Errorf("hash %s: %w", f.Path, err)
		}
		h.Write([]byte(strconv.FormatInt(n, 10)))
		h.Write(sep)
		h.Write(fileSum.Sum(nil))
		h.Write(sep)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
