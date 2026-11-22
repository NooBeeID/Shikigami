package review

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NooBeeID/shikigami/internal/fsx"
)

// Sentinel error aksi review; tag di awal pesan dipakai UI.
var (
	actionsErrConflict = errors.New("[CONFLICT] file berubah sejak diff dihitung, muat ulang")
	actionsErrExternal = errors.New("[EXTERNAL] file diubah di luar agent, butuh konfirmasi (force)")
)

// actionsSplitKeep memecah isi menjadi baris dengan mempertahankan terminator.
func actionsSplitKeep(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(b), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// actionsJoin menggabung baris; baris non-terakhir tanpa newline diberi newline.
func actionsJoin(lines []string) []byte {
	var sb strings.Builder
	for i, l := range lines {
		sb.WriteString(l)
		if i < len(lines)-1 && !strings.HasSuffix(l, "\n") {
			sb.WriteString("\n")
		}
	}
	return []byte(sb.String())
}

// actionsReplace mengganti rentang baris pada dst dengan potongan dari src.
func actionsReplace(dst, src []string, ds, dn, ss, sn int) ([]string, error) {
	if ds < 0 || dn < 0 || ds+dn > len(dst) || ss < 0 || sn < 0 || ss+sn > len(src) {
		return nil, errors.New("hunk di luar rentang, muat ulang diff")
	}
	out := make([]string, 0, len(dst)-dn+sn)
	out = append(out, dst[:ds]...)
	out = append(out, src[ss:ss+sn]...)
	out = append(out, dst[ds+dn:]...)
	return out, nil
}

func actionsStart(start, n int) int {
	if n == 0 || start < 1 {
		return 0
	}
	return start - 1
}

// actionsApplyReverse membalik hunk terpilih pada cur memakai baris dari base.
func actionsApplyReverse(cur, base []byte, hunks []Hunk) ([]byte, error) {
	curLines := actionsSplitKeep(cur)
	baseLines := actionsSplitKeep(base)
	hs := append([]Hunk(nil), hunks...)
	sort.SliceStable(hs, func(i, j int) bool {
		if hs[i].NewStart != hs[j].NewStart {
			return hs[i].NewStart > hs[j].NewStart
		}
		return hs[i].OldStart > hs[j].OldStart
	})
	var err error
	for _, h := range hs {
		cs := actionsStart(h.NewStart, h.NewLines)
		bs := actionsStart(h.OldStart, h.OldLines)
		curLines, err = actionsReplace(curLines, baseLines, cs, h.NewLines, bs, h.OldLines)
		if err != nil {
			return nil, err
		}
	}
	return actionsJoin(curLines), nil
}

// actionsApplyForward memajukan hunk terpilih pada base memakai baris dari cur.
func actionsApplyForward(base, cur []byte, hunks []Hunk) ([]byte, error) {
	curLines := actionsSplitKeep(cur)
	baseLines := actionsSplitKeep(base)
	hs := append([]Hunk(nil), hunks...)
	sort.SliceStable(hs, func(i, j int) bool {
		if hs[i].OldStart != hs[j].OldStart {
			return hs[i].OldStart > hs[j].OldStart
		}
		return hs[i].NewStart > hs[j].NewStart
	})
	var err error
	for _, h := range hs {
		cs := actionsStart(h.NewStart, h.NewLines)
		bs := actionsStart(h.OldStart, h.OldLines)
		baseLines, err = actionsReplace(baseLines, curLines, bs, h.OldLines, cs, h.NewLines)
		if err != nil {
			return nil, err
		}
	}
	return actionsJoin(baseLines), nil
}

// actionsWriteFile menulis file pengguna secara atomik dengan mempertahankan mode.
func actionsWriteFile(abs string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("tulis file: %w", err)
	}
	target := abs
	if li, err := os.Lstat(abs); err == nil && li.Mode()&os.ModeSymlink != 0 {
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			target = resolved
		}
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(target); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".review-tmp-*")
	if err != nil {
		return fmt.Errorf("tulis file: %w", err)
	}
	name := tmp.Name()
	fail := func(err error) error {
		os.Remove(name)
		return fmt.Errorf("tulis file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		return fail(err)
	}
	if err := os.Chmod(name, mode); err != nil {
		return fail(err)
	}
	if err := os.Rename(name, target); err != nil {
		return fail(err)
	}
	return nil
}

// actionsCurrentHash mengembalikan hash isi sekarang ("" bila file tidak ada).
func actionsCurrentHash(abs string) (string, bool, error) {
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if !fi.Mode().IsRegular() {
		return "", false, errors.New("bukan file biasa")
	}
	h, err := storeHashFile(abs)
	if err != nil {
		return "", false, err
	}
	return h, true, nil
}

// actionsLocate mencari entri dan path absolut; dipakai di bawah lock.
func actionsLocate(idx storeIndex, path string) (string, int, error) {
	if path == "" {
		return "", 0, errors.New("path wajib diisi")
	}
	found := -1
	for i, f := range idx.Files {
		if f.Path == path {
			found = i
			break
		}
	}
	if found < 0 {
		return "", 0, fmt.Errorf("file tidak ada dalam daftar review sesi ini: %s", path)
	}
	if idx.Cwd == "" {
		return "", 0, errors.New("cwd sesi tidak diketahui")
	}
	abs, err := fsx.ResolveUnder(idx.Cwd, path)
	if err != nil {
		return "", 0, fmt.Errorf("path tidak valid: %w", err)
	}
	return abs, found, nil
}

func actionsExternal(t TouchedFile, curHash string, exists bool) bool {
	if t.ExpectedHash == "" {
		return false
	}
	now := curHash
	if !exists {
		now = HashAbsent
	}
	return now != t.ExpectedHash
}

// actionsDrop membuang entri i dari index, lalu menghapus baseline (best effort).
func (s *Store) actionsDrop(sessionID string, idx storeIndex, i int) error {
	path := idx.Files[i].Path
	idx.Files = append(idx.Files[:i], idx.Files[i+1:]...)
	if err := s.storeSave(sessionID, idx); err != nil {
		return fmt.Errorf("simpan index: %w", err)
	}
	_ = os.Remove(s.BaselinePath(sessionID, path))
	return nil
}

func actionsIDSet(ids []string) map[string]bool {
	set := map[string]bool{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			set[id] = true
		}
	}
	return set
}

func actionsSelect(res DiffResult, ids map[string]bool) ([]Hunk, error) {
	known := map[string]bool{}
	for _, h := range res.Hunks {
		known[h.ID] = true
	}
	for id := range ids {
		if !known[id] {
			return nil, fmt.Errorf("hunkId tidak dikenal: %s (diff mungkin sudah berubah, muat ulang)", id)
		}
	}
	var sel []Hunk
	for _, h := range res.Hunks {
		if ids[h.ID] {
			sel = append(sel, h)
		}
	}
	return sel, nil
}

const actionsErrBinaryHunk = "file biner/terlalu besar hanya mendukung aksi seluruh file"

// actionsReadBaseline membaca baseline bila file ada sebelum sesi, selain itu nil.
func (s *Store) actionsReadBaseline(sessionID string, t TouchedFile) ([]byte, error) {
	if !t.ExistedBefore {
		return nil, nil
	}
	base, err := os.ReadFile(s.BaselinePath(sessionID, t.Path))
	if err != nil {
		return nil, fmt.Errorf("baseline tidak ditemukan: %w", err)
	}
	return base, nil
}

// actionsReadCur membaca isi sekarang dan memverifikasi hash (pengaman balapan).
func actionsReadCur(abs, curHash string, exists bool) ([]byte, error) {
	if !exists {
		return nil, nil
	}
	cur, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("baca file: %w", err)
	}
	if storeHashBytes(cur) != curHash {
		return nil, actionsErrConflict
	}
	return cur, nil
}

// Revert mengembalikan file (atau hunk terpilih) ke baseline sesi.
func (s *Store) Revert(sessionID, path string, hunkIDs []string, expectHash string, force bool) (removed bool, err error) {
	if !storeValidID(sessionID) {
		return false, errors.New("sessionId tidak valid")
	}
	defer s.storeLock(sessionID)()

	idx := s.storeLoad(sessionID)
	abs, i, err := actionsLocate(idx, path)
	if err != nil {
		return false, err
	}
	t := idx.Files[i]
	ids := actionsIDSet(hunkIDs)
	if len(ids) > 0 && (t.Binary || t.TooLarge) {
		return false, errors.New(actionsErrBinaryHunk)
	}
	curHash, exists, err := actionsCurrentHash(abs)
	if err != nil {
		return false, err
	}
	if curHash != expectHash {
		return false, actionsErrConflict
	}
	if actionsExternal(t, curHash, exists) && !force {
		return false, actionsErrExternal
	}

	if len(ids) > 0 {
		cur, err := actionsReadCur(abs, curHash, exists)
		if err != nil {
			return false, err
		}
		base, err := s.actionsReadBaseline(sessionID, t)
		if err != nil {
			return false, err
		}
		res := ComputeDiff(base, cur)
		if res.Binary || res.TooLarge {
			return false, errors.New(actionsErrBinaryHunk)
		}
		sel, err := actionsSelect(res, ids)
		if err != nil {
			return false, err
		}
		if len(sel) != len(res.Hunks) {
			out, err := actionsApplyReverse(cur, base, sel)
			if err != nil {
				return false, err
			}
			if err := actionsWriteFile(abs, out); err != nil {
				return false, err
			}
			newHash := storeHashBytes(out)
			if t.ExistedBefore && newHash == t.BaselineSha256 {
				if err := s.actionsDrop(sessionID, idx, i); err != nil {
					return false, err
				}
				return true, nil
			}
			idx.Files[i].ExpectedHash = newHash
			if err := s.storeSave(sessionID, idx); err != nil {
				return false, fmt.Errorf("simpan index: %w", err)
			}
			return false, nil
		}
		// semua hunk terpilih: revert penuh
	}

	// Revert seluruh file.
	if !t.ExistedBefore {
		if exists {
			if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
				return false, fmt.Errorf("hapus file: %w", err)
			}
		}
	} else if !(exists && curHash == t.BaselineSha256) {
		if t.Binary || t.TooLarge {
			return false, errors.New("baseline tidak tersimpan untuk file biner/terlalu besar, tidak bisa dikembalikan")
		}
		base, err := os.ReadFile(s.BaselinePath(sessionID, path))
		if err != nil {
			return false, fmt.Errorf("baseline tidak ditemukan: %w", err)
		}
		if err := actionsWriteFile(abs, base); err != nil {
			return false, err
		}
	}
	if err := s.actionsDrop(sessionID, idx, i); err != nil {
		return false, err
	}
	return true, nil
}

// Accept memajukan baseline ke isi sekarang (seluruh file atau hunk terpilih).
func (s *Store) Accept(sessionID, path string, hunkIDs []string) (removed bool, err error) {
	if !storeValidID(sessionID) {
		return false, errors.New("sessionId tidak valid")
	}
	defer s.storeLock(sessionID)()

	idx := s.storeLoad(sessionID)
	abs, i, err := actionsLocate(idx, path)
	if err != nil {
		return false, err
	}
	t := idx.Files[i]
	ids := actionsIDSet(hunkIDs)
	if len(ids) > 0 && (t.Binary || t.TooLarge) {
		return false, errors.New(actionsErrBinaryHunk)
	}

	if len(ids) > 0 {
		curHash, exists, err := actionsCurrentHash(abs)
		if err != nil {
			return false, err
		}
		cur, err := actionsReadCur(abs, curHash, exists)
		if err != nil {
			return false, err
		}
		base, err := s.actionsReadBaseline(sessionID, t)
		if err != nil {
			return false, err
		}
		res := ComputeDiff(base, cur)
		if res.Binary || res.TooLarge {
			return false, errors.New(actionsErrBinaryHunk)
		}
		sel, err := actionsSelect(res, ids)
		if err != nil {
			return false, err
		}
		if len(sel) != len(res.Hunks) {
			out, err := actionsApplyForward(base, cur, sel)
			if err != nil {
				return false, err
			}
			if err := captureWriteAtomic(s.BaselinePath(sessionID, path), out); err != nil {
				return false, fmt.Errorf("tulis baseline: %w", err)
			}
			newHash := storeHashBytes(out)
			idx.Files[i].BaselineSha256 = newHash
			idx.Files[i].ExistedBefore = true
			if exists && newHash == curHash {
				if err := s.actionsDrop(sessionID, idx, i); err != nil {
					return false, err
				}
				return true, nil
			}
			if err := s.storeSave(sessionID, idx); err != nil {
				return false, fmt.Errorf("simpan index: %w", err)
			}
			return false, nil
		}
	}

	if err := s.actionsDrop(sessionID, idx, i); err != nil {
		return false, err
	}
	return true, nil
}

// actionsExternalFor menghitung flag external dari entri dan isi file sekarang.
func actionsExternalFor(cwd string, t TouchedFile) bool {
	if t.ExpectedHash == "" {
		return false
	}
	abs, err := fsx.ResolveUnder(cwd, t.Path)
	if err != nil {
		return false
	}
	now := HashAbsent
	fi, err := os.Stat(abs)
	switch {
	case err == nil:
		if !fi.Mode().IsRegular() {
			return false
		}
		if now, err = storeHashFile(abs); err != nil {
			return false
		}
	case os.IsNotExist(err):
	default:
		return false
	}
	return now != t.ExpectedHash
}

// IsExternal melaporkan apakah file diubah di luar agent.
func (s *Store) IsExternal(sessionID, path string) bool {
	if !storeValidID(sessionID) {
		return false
	}
	for _, t := range s.Touched(sessionID) {
		if t.Path == path {
			cwd := s.Cwd(sessionID)
			if cwd == "" {
				return false
			}
			return actionsExternalFor(cwd, t)
		}
	}
	return false
}

// FillExternal mengisi files[i].External in place.
func FillExternal(st *Store, sessionID string, files []FileEntry) {
	if st == nil || len(files) == 0 || !storeValidID(sessionID) {
		return
	}
	touched := map[string]TouchedFile{}
	for _, t := range st.Touched(sessionID) {
		touched[t.Path] = t
	}
	cwd := st.Cwd(sessionID)
	if cwd == "" {
		return
	}
	for i := range files {
		if t, ok := touched[files[i].Path]; ok {
			files[i].External = actionsExternalFor(cwd, t)
		}
	}
}
