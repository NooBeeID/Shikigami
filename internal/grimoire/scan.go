package grimoire

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/NooBeeID/shikigami/internal/fsx"
)

const maxFileBytes = 1 << 20

// maxGitignoreBytes membatasi ukuran .gitignore yang dibaca.
const maxGitignoreBytes = 256 << 10

// FileInfo menggambarkan satu berkas hasil pindai.
// Field abs (tak diekspor) berisi path absolut berkas; diisi oleh Scan agar
// GroupModules dapat membaca isi untuk ekstraksi simbol tanpa root terpisah.
// Bila FileInfo dibuat manual (abs kosong), simbol dilewati tanpa galat.
type FileInfo struct {
	Path   string // relatif repo, pemisah "/"
	Size   int64
	Binary bool // NUL pada 8 KiB awal
	Large  bool // > maxFileBytes; tidak dibaca untuk simbol
	abs    string
}

type ignoreRule struct {
	base     string         // direktori (relatif root, "/", "" untuk root) tempat .gitignore berada
	pattern  string         // pola tanpa "!" / "/" awal / "/" akhir
	negate   bool           //
	dirOnly  bool           // pola berakhir "/"
	anchored bool           // mengandung "/" di awal atau di tengah
	re       *regexp.Regexp // hasil terjemahan glob; nil bila pola tak dikenali
}

// parseGitignore mengurai isi .gitignore; aturan tak dikenali diabaikan.
func parseGitignore(base string, data []byte) []ignoreRule {
	var rules []ignoreRule
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		// trim spasi akhir yang tidak ter-escape
		for strings.HasSuffix(line, " ") && !strings.HasSuffix(line, "\\ ") {
			line = line[:len(line)-1]
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r := ignoreRule{base: base}
		if strings.HasPrefix(line, "!") {
			r.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			r.dirOnly = true
			line = strings.TrimRight(line, "/")
		}
		if line == "" {
			continue
		}
		if strings.Contains(line, "/") {
			r.anchored = true
			line = strings.TrimPrefix(line, "/")
		}
		if line == "" {
			continue
		}
		r.pattern = line
		r.re = globToRegexp(line)
		if r.re == nil {
			continue
		}
		rules = append(rules, r)
	}
	return rules
}

// globToRegexp menerjemahkan glob gitignore ke regexp; nil bila tak dikenali.
func globToRegexp(p string) *regexp.Regexp {
	var sb strings.Builder
	sb.WriteString("^")
	n := len(p)
	for i := 0; i < n; {
		c := p[i]
		switch {
		case c == '*' && i+1 < n && p[i+1] == '*':
			atStart := i == 0 || p[i-1] == '/'
			if atStart && i+2 < n && p[i+2] == '/' {
				sb.WriteString("(?:.*/)?")
				i += 3
			} else if i > 0 && p[i-1] == '/' && i+2 == n {
				// "/**" di akhir: "/" sudah ditulis sebagai literal
				sb.WriteString(".*")
				i += 2
			} else {
				sb.WriteString(".*")
				i += 2
			}
		case c == '*':
			sb.WriteString("[^/]*")
			i++
		case c == '?':
			sb.WriteString("[^/]")
			i++
		case c == '[':
			end := strings.IndexByte(p[i+1:], ']')
			// ']' langsung setelah '[' atau '[!' dianggap bagian kelas; sederhanakan
			if end < 0 {
				return nil
			}
			class := p[i+1 : i+1+end]
			if class == "" {
				return nil
			}
			if class[0] == '!' {
				class = "^" + class[1:]
			}
			sb.WriteString("[" + class + "]")
			i += end + 2
		case c == '\\':
			if i+1 >= n {
				return nil
			}
			sb.WriteString(regexp.QuoteMeta(string(p[i+1])))
			i += 2
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
			i++
		}
	}
	sb.WriteString("$")
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil
	}
	return re
}

// matchRules menentukan apakah rel (relatif root) diabaikan; aturan terakhir yang cocok menang.
func matchRules(rules []ignoreRule, rel string, isDir bool) bool {
	ignored := false
	for _, r := range rules {
		if r.re == nil {
			continue
		}
		sub := rel
		if r.base != "" {
			if !strings.HasPrefix(rel, r.base+"/") {
				continue
			}
			sub = rel[len(r.base)+1:]
		}
		if r.dirOnly && !isDir {
			continue
		}
		target := sub
		if !r.anchored {
			target = path.Base(sub)
		}
		if r.re.MatchString(target) {
			ignored = !r.negate
		}
	}
	return ignored
}

// Scan menelusuri root tanpa mengikuti symlink dan mengembalikan berkas terurut menurut Path.
func Scan(root string) ([]FileInfo, error) {
	if root == "" {
		return nil, fmt.Errorf("root pindai tidak valid: %w", os.ErrInvalid)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("root pindai tidak valid: %w", err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("root pindai tidak valid: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("root pindai tidak valid: %s bukan direktori", root)
	}
	out := []FileInfo{}
	if err := walkDir(abs, "", nil, &out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func walkDir(absDir, relDir string, rules []ignoreRule, out *[]FileInfo) error {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		if relDir == "" {
			return fmt.Errorf("baca direktori %s: %w", absDir, err)
		}
		return nil // subdirektori tak terbaca: lewati
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	// muat .gitignore reguler di direktori ini
	for _, e := range entries {
		if e.Name() != ".gitignore" || !e.Type().IsRegular() {
			continue
		}
		if data, err := readLimited(filepath.Join(absDir, ".gitignore"), maxGitignoreBytes); err == nil {
			add := parseGitignore(relDir, data)
			if len(add) > 0 {
				nr := make([]ignoreRule, 0, len(rules)+len(add))
				nr = append(nr, rules...)
				nr = append(nr, add...)
				rules = nr
			}
		}
		break
	}

	for _, e := range entries {
		name := e.Name()
		if fsx.IsIgnored(name) {
			continue
		}
		typ := e.Type()
		if typ&os.ModeSymlink != 0 {
			continue
		}
		rel := name
		if relDir != "" {
			rel = path.Join(relDir, name)
		}
		if typ.IsDir() {
			if matchRules(rules, rel, true) {
				continue
			}
			if err := walkDir(filepath.Join(absDir, name), rel, rules, out); err != nil {
				return err
			}
			continue
		}
		if !typ.IsRegular() {
			continue
		}
		if matchRules(rules, rel, false) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		absPath := filepath.Join(absDir, name)
		bin, err := sniffBinary(absPath)
		if err != nil {
			continue
		}
		*out = append(*out, FileInfo{
			Path:   rel,
			Size:   info.Size(),
			Binary: bin,
			Large:  info.Size() > maxFileBytes,
			abs:    absPath,
		})
	}
	return nil
}

func readLimited(p string, limit int64) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}

// sniffBinary membaca maks 8 KiB awal dan mencari byte NUL.
func sniffBinary(p string) (bool, error) {
	f, err := os.Open(p)
	if err != nil {
		return false, err
	}
	defer f.Close()
	buf := make([]byte, 8192)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}
	return bytes.IndexByte(buf[:n], 0) >= 0, nil
}
