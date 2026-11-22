package review

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/NooBeeID/shikigami/internal/fsx"
)

// MaxDiffBytes adalah ukuran maksimum (per sisi) yang masih dihitung diff-nya.
const MaxDiffBytes = 1 << 20

// ContextLines adalah jumlah baris konteks di sekitar perubahan.
const ContextLines = 3

// diffMaxCost membatasi biaya D pada Myers. Memori trace sekitar D^2 int dan
// waktu O(N*D); 2000 menjaga keduanya wajar untuk file sekitar 1 MB. Bila
// terlampaui dipakai fallback hapus-semua/tambah-semua. Boleh disetel.
const diffMaxCost = 2000

// DiffLine adalah satu baris dalam hunk.
type DiffLine struct {
	Kind  string `json:"kind"` // "ctx" | "add" | "del"
	Text  string `json:"text"` // tanpa newline akhir
	OldNo int    `json:"oldNo"`
	NewNo int    `json:"newNo"`
}

// Hunk adalah sekelompok perubahan berikut konteksnya.
type Hunk struct {
	ID       string     `json:"id"`
	OldStart int        `json:"oldStart"`
	OldLines int        `json:"oldLines"`
	NewStart int        `json:"newStart"`
	NewLines int        `json:"newLines"`
	Lines    []DiffLine `json:"lines"`
}

// DiffResult adalah hasil diff satu file.
type DiffResult struct {
	Hunks       []Hunk `json:"hunks"`
	CurrentHash string `json:"currentHash"`
	Binary      bool   `json:"binary"`
	TooLarge    bool   `json:"tooLarge"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
}

// diffOp adalah satu operasi edit; ai/bi indeks 0-based di sisi lama/baru.
type diffOp struct {
	kind   byte // '=', '-', '+'
	ai, bi int
}

// diffSplitLines memecah isi menjadi baris tanpa newline akhir.
func diffSplitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// diffKey menormalkan CRLF hanya untuk perbandingan.
func diffKey(line string) string {
	return strings.TrimSuffix(line, "\r")
}

// diffIntern memetakan baris ke id int agar perbandingan O(1).
func diffIntern(ids map[string]int, lines []string) []int {
	out := make([]int, len(lines))
	for i, l := range lines {
		k := diffKey(l)
		id, ok := ids[k]
		if !ok {
			id = len(ids)
			ids[k] = id
		}
		out[i] = id
	}
	return out
}

// diffMyers menghitung skrip edit minimal. ok=false bila biaya melebihi diffMaxCost.
func diffMyers(a, b []int) (ops []diffOp, ok bool) {
	// Pangkas prefiks dan sufiks yang sama.
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	ma := a[pre : len(a)-suf]
	mb := b[pre : len(b)-suf]

	ops = make([]diffOp, 0, len(a)+len(b))
	for i := 0; i < pre; i++ {
		ops = append(ops, diffOp{'=', i, i})
	}
	mid, ok := diffMyersCore(ma, mb)
	if !ok {
		// Fallback tanpa memori kuadrat: hapus semua lalu tambah semua.
		mid = mid[:0]
		for i := range ma {
			mid = append(mid, diffOp{'-', i, 0})
		}
		for j := range mb {
			mid = append(mid, diffOp{'+', 0, j})
		}
	}
	for _, o := range mid {
		switch o.kind {
		case '=':
			o.ai += pre
			o.bi += pre
		case '-':
			o.ai += pre
		case '+':
			o.bi += pre
		}
		ops = append(ops, o)
	}
	for i := 0; i < suf; i++ {
		ops = append(ops, diffOp{'=', len(a) - suf + i, len(b) - suf + i})
	}
	return ops, ok
}

func diffMyersCore(a, b []int) ([]diffOp, bool) {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil, true
	}
	if n == 0 {
		ops := make([]diffOp, m)
		for j := range ops {
			ops[j] = diffOp{'+', 0, j}
		}
		return ops, true
	}
	if m == 0 {
		ops := make([]diffOp, n)
		for i := range ops {
			ops[i] = diffOp{'-', i, 0}
		}
		return ops, true
	}
	max := n + m
	if max > diffMaxCost {
		max = diffMaxCost
	}
	off := max + 1
	v := make([]int, 2*max+3)
	trace := make([][]int, 0, 64)
	found := false
	for d := 0; d <= max && !found; d++ {
		// Simpan keadaan v sebelum iterasi d untuk backtrack, rentang k [-d-1, d+1].
		snap := make([]int, 2*d+3)
		copy(snap, v[off-d-1:off+d+2])
		trace = append(trace, snap)
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[off+k] = x
			if x >= n && y >= m {
				found = true
				break
			}
		}
	}
	if !found {
		return nil, false
	}
	// Backtrack.
	rev := make([]diffOp, 0, n+m)
	x, y := n, m
	for d := len(trace) - 1; d >= 0; d-- {
		vv := trace[d]
		at := func(k int) int { return vv[k+d+1] }
		k := x - y
		var prevX, prevY int
		if d == 0 {
			prevX, prevY = 0, 0
		} else {
			var prevK int
			if k == -d || (k != d && at(k-1) < at(k+1)) {
				prevK = k + 1
			} else {
				prevK = k - 1
			}
			prevX = at(prevK)
			prevY = prevX - prevK
		}
		for x > prevX && y > prevY {
			rev = append(rev, diffOp{'=', x - 1, y - 1})
			x--
			y--
		}
		if d > 0 {
			if x == prevX {
				rev = append(rev, diffOp{'+', 0, prevY})
			} else {
				rev = append(rev, diffOp{'-', prevX, 0})
			}
			x, y = prevX, prevY
		}
	}
	ops := make([]diffOp, len(rev))
	for i, o := range rev {
		ops[len(rev)-1-i] = o
	}
	return ops, true
}

// diffHunkID menghitung id stabil hunk (12 hex pertama sha256).
func diffHunkID(h Hunk) string {
	var sb strings.Builder
	sb.WriteString(strconv.Itoa(h.OldStart) + "," + strconv.Itoa(h.OldLines) + "," +
		strconv.Itoa(h.NewStart) + "," + strconv.Itoa(h.NewLines) + "\n")
	for _, l := range h.Lines {
		sb.WriteString(l.Kind + "\x00" + l.Text + "\n")
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])[:12]
}

// diffGroupHunks mengelompokkan ops menjadi hunk berikut konteks.
func diffGroupHunks(ops []diffOp, oldLines, newLines []string) (hunks []Hunk, adds, dels int) {
	hunks = []Hunk{}
	flat := make([]DiffLine, len(ops))
	var changes []int
	for i, o := range ops {
		switch o.kind {
		case '=':
			flat[i] = DiffLine{Kind: "ctx", Text: newLines[o.bi], OldNo: o.ai + 1, NewNo: o.bi + 1}
		case '-':
			flat[i] = DiffLine{Kind: "del", Text: oldLines[o.ai], OldNo: o.ai + 1}
			dels++
			changes = append(changes, i)
		case '+':
			flat[i] = DiffLine{Kind: "add", Text: newLines[o.bi], NewNo: o.bi + 1}
			adds++
			changes = append(changes, i)
		}
	}
	if len(changes) == 0 {
		return hunks, 0, 0
	}
	emit := func(first, last int) {
		start := first - ContextLines
		if start < 0 {
			start = 0
		}
		end := last + ContextLines
		if end > len(flat)-1 {
			end = len(flat) - 1
		}
		h := Hunk{Lines: append([]DiffLine(nil), flat[start:end+1]...)}
		for _, l := range h.Lines {
			if l.OldNo > 0 {
				if h.OldLines == 0 {
					h.OldStart = l.OldNo
				}
				h.OldLines++
			}
			if l.NewNo > 0 {
				if h.NewLines == 0 {
					h.NewStart = l.NewNo
				}
				h.NewLines++
			}
		}
		h.ID = diffHunkID(h)
		hunks = append(hunks, h)
	}
	first, last := changes[0], changes[0]
	for _, c := range changes[1:] {
		if c-last-1 <= 2*ContextLines {
			last = c
			continue
		}
		emit(first, last)
		first, last = c, c
	}
	emit(first, last)
	return hunks, adds, dels
}

// ComputeDiff menghitung diff murni; nil berarti sisi itu tidak ada.
func ComputeDiff(old, cur []byte) DiffResult {
	res := DiffResult{Hunks: []Hunk{}}
	if cur != nil {
		res.CurrentHash = storeHashBytes(cur)
	}
	res.Binary = fsx.IsBinary(old) || fsx.IsBinary(cur)
	res.TooLarge = len(old) > MaxDiffBytes || len(cur) > MaxDiffBytes
	if res.Binary || res.TooLarge {
		return res
	}
	oldLines := diffSplitLines(old)
	newLines := diffSplitLines(cur)
	ids := map[string]int{}
	a := diffIntern(ids, oldLines)
	b := diffIntern(ids, newLines)
	ops, _ := diffMyers(a, b)
	res.Hunks, res.Additions, res.Deletions = diffGroupHunks(ops, oldLines, newLines)
	return res
}

// DiffForFile membaca baseline dari Store dan isi file sekarang lalu menghitung diff.
func DiffForFile(st *Store, sessionID, path string) (DiffResult, error) {
	if st == nil || !storeValidID(sessionID) {
		return DiffResult{}, errors.New("sessionId tidak valid")
	}
	if path == "" {
		return DiffResult{}, errors.New("path wajib diisi")
	}
	var entry *TouchedFile
	for _, t := range st.Touched(sessionID) {
		if t.Path == path {
			t := t
			entry = &t
			break
		}
	}
	if entry == nil {
		return DiffResult{}, fmt.Errorf("file tidak ada dalam daftar review sesi ini: %s", path)
	}
	cwd := st.Cwd(sessionID)
	if cwd == "" {
		return DiffResult{}, errors.New("cwd sesi tidak diketahui")
	}
	abs, err := fsx.ResolveUnder(cwd, path)
	if err != nil {
		return DiffResult{}, fmt.Errorf("path tidak valid: %w", err)
	}
	curExists := true
	var size int64
	fi, err := os.Stat(abs)
	switch {
	case err == nil:
		if !fi.Mode().IsRegular() {
			return DiffResult{}, errors.New("bukan file biasa")
		}
		size = fi.Size()
	case os.IsNotExist(err):
		curExists = false
	default:
		return DiffResult{}, err
	}
	hash := ""
	if curExists {
		if hash, err = storeHashFile(abs); err != nil {
			return DiffResult{}, err
		}
	}
	tooLarge := entry.TooLarge || (curExists && size > MaxDiffBytes)
	binary := entry.Binary
	if curExists && !tooLarge {
		sample, err := captureSample(abs)
		if err != nil {
			return DiffResult{}, err
		}
		binary = binary || fsx.IsBinary(sample)
	}
	if binary || tooLarge {
		return DiffResult{Hunks: []Hunk{}, CurrentHash: hash, Binary: binary, TooLarge: tooLarge}, nil
	}
	var old []byte
	if entry.ExistedBefore {
		old, err = os.ReadFile(st.BaselinePath(sessionID, path))
		if err != nil {
			return DiffResult{}, fmt.Errorf("baseline tidak ditemukan: %w", err)
		}
	}
	var cur []byte
	if curExists {
		cur, err = os.ReadFile(abs)
		if err != nil {
			return DiffResult{}, err
		}
		if cur == nil {
			cur = []byte{}
		}
	}
	return ComputeDiff(old, cur), nil
}
