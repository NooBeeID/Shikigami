package grimoire

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

const (
	maxModuleFiles = 200
	maxModuleBytes = 512 << 10
	maxSymbols     = 60
)

const (
	kindGo      = "go"
	kindWeb     = "web"
	kindGeneric = "generic"
)

const webModulesPrefix = "frontend/src/modules/"

var (
	symGoLineRe  = regexp.MustCompile(`^(func|type|const|var)\s+(?:\([^)]*\)\s*)?([A-Z]\w*)`)
	symWebLineRe = regexp.MustCompile(`^\s*export\s+(?:default\s+)?(?:async\s+)?(function|const|let|class|type|interface|enum)\s+([A-Za-z_$][\w$]*)`)
	tagValidRe   = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
)

type Module struct {
	Name      string     // unik, snake_case
	Dir       string     // relatif repo ("." untuk root)
	Kind      string     // "go" | "web" | "generic"
	Files     []FileInfo // urut path
	Symbols   []string   // ringkas, maks 60
	Sources   []string   // path relatif repo, urut leksikal, "/"
	Tag       string     // satu token camelCase
	Truncated int        // jumlah berkas yang simbolnya dipotong
}

func isGoFile(p string) bool { return path.Ext(p) == ".go" }

func isWebFile(p string) bool {
	switch path.Ext(p) {
	case ".svelte", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return true
	}
	return false
}

// grpWebSpecialUnit mengembalikan unit frontend/src/modules/<x> bila p berada di dalamnya.
func grpWebSpecialUnit(p string) (string, bool) {
	if !strings.HasPrefix(p, webModulesPrefix) {
		return "", false
	}
	rest := p[len(webModulesPrefix):]
	i := strings.Index(rest, "/")
	if i <= 0 || i == len(rest)-1 {
		return "", false
	}
	return webModulesPrefix + rest[:i], true
}

// GroupModules mengelompokkan berkas menjadi modul Go / web / generik.
func GroupModules(files []FileInfo) []Module {
	sorted := make([]FileInfo, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	// Lintasan 1: direktori unit kode.
	codeUnits := map[string]string{}
	for _, f := range sorted {
		if _, ok := grpWebSpecialUnit(f.Path); ok {
			continue
		}
		d := path.Dir(f.Path)
		switch {
		case isGoFile(f.Path):
			codeUnits[d] = kindGo
		case isWebFile(f.Path):
			if codeUnits[d] != kindGo {
				codeUnits[d] = kindWeb
			}
		}
	}

	// Lintasan 2: tiap berkas ke tepat satu unit.
	type unit struct {
		kind  string
		files []FileInfo
		web   bool // unit khusus frontend/src/modules/<x>
	}
	units := map[string]*unit{}
	add := func(dir, kind string, special bool, f FileInfo) {
		u := units[dir]
		if u == nil {
			u = &unit{kind: kind, web: special}
			units[dir] = u
		}
		u.files = append(u.files, f)
	}
	for _, f := range sorted {
		if u, ok := grpWebSpecialUnit(f.Path); ok {
			add(u, kindWeb, true, f)
			continue
		}
		d := path.Dir(f.Path)
		placed := false
		if d == "." {
			if k, ok := codeUnits["."]; ok {
				add(".", k, false, f)
				placed = true
			}
		} else {
			for a := d; a != "." && a != "/" && a != ""; a = path.Dir(a) {
				if k, ok := codeUnits[a]; ok {
					add(a, k, false, f)
					placed = true
					break
				}
			}
		}
		if placed {
			continue
		}
		// generik
		if d == "." {
			add(".", kindGeneric, false, f)
			continue
		}
		segs := strings.Split(d, "/")
		g := segs[0]
		if len(segs) >= 2 {
			g = segs[0] + "/" + segs[1]
		}
		add(g, kindGeneric, false, f)
	}

	dirs := make([]string, 0, len(units))
	for d := range units {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	used := map[string]bool{}
	mods := make([]Module, 0, len(dirs))
	for _, d := range dirs {
		u := units[d]
		var baseSrc string
		switch {
		case u.web:
			baseSrc = path.Base(d)
		case d == ".":
			baseSrc = "root"
		default:
			baseSrc = strings.ReplaceAll(d, "/", "_")
		}
		base := Snake(baseSrc)
		if base == "" {
			base = "modul"
		}
		name := base
		for n := 2; used[name]; n++ {
			name = fmt.Sprintf("%s_%d", base, n)
		}
		used[name] = true

		m := Module{Name: name, Dir: d, Kind: u.kind, Files: u.files}
		m.Sources = make([]string, len(m.Files))
		for i, f := range m.Files {
			m.Sources[i] = f.Path
		}
		m.Symbols, m.Truncated = grpExtractSymbols(m)
		m.Tag = grpTag(m)
		mods = append(mods, m)
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Name < mods[j].Name })
	return mods
}

// symRelevant: apakah berkas dipakai untuk simbol pada modul berkind ini.
func symRelevant(kind string, f FileInfo) bool {
	switch kind {
	case kindGo:
		return isGoFile(f.Path) && !strings.HasSuffix(f.Path, "_test.go")
	case kindWeb:
		return isWebFile(f.Path)
	}
	return false
}

func grpExtractSymbols(m Module) ([]string, int) {
	symbols := []string{}
	truncated := 0
	if len(m.Files) > maxModuleFiles {
		truncated = len(m.Files) - maxModuleFiles
	}
	if m.Kind == kindGeneric {
		return symbols, truncated
	}
	seen := map[string]bool{}
	var total int64
	byteCut := false
	for i, f := range m.Files {
		if i >= maxModuleFiles {
			break
		}
		if f.Binary || f.Large {
			continue
		}
		if !byteCut {
			if total+f.Size > maxModuleBytes {
				byteCut = true
			} else {
				total += f.Size
			}
		}
		if !symRelevant(m.Kind, f) {
			continue
		}
		if byteCut {
			truncated++
			continue
		}
		if len(symbols) >= maxSymbols || f.abs == "" {
			continue
		}
		src, err := grpReadFile(f.abs)
		if err != nil {
			continue
		}
		var syms []string
		if m.Kind == kindGo {
			syms = grpGoSymbols(f.Path, src)
		} else {
			syms = grpWebSymbols(f.Path, src)
		}
		for _, s := range syms {
			if seen[s] {
				continue
			}
			seen[s] = true
			symbols = append(symbols, s)
			if len(symbols) >= maxSymbols {
				break
			}
		}
	}
	return symbols, truncated
}

func grpReadFile(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxFileBytes))
}

func grpGoSymbols(name string, src []byte) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil || file == nil {
		return grpLineSymbols(src, func(line string) (string, bool) {
			mm := symGoLineRe.FindStringSubmatch(line)
			if mm == nil {
				return "", false
			}
			return mm[1] + " " + mm[2], true
		})
	}
	var out []string
	for _, d := range file.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			if !ast.IsExported(dd.Name.Name) {
				continue
			}
			if dd.Recv != nil && len(dd.Recv.List) > 0 {
				out = append(out, "method "+grpRecvName(dd.Recv.List[0].Type)+"."+dd.Name.Name)
			} else {
				out = append(out, "func "+dd.Name.Name)
			}
		case *ast.GenDecl:
			for _, sp := range dd.Specs {
				switch s := sp.(type) {
				case *ast.TypeSpec:
					if ast.IsExported(s.Name.Name) {
						out = append(out, "type "+s.Name.Name)
					}
				case *ast.ValueSpec:
					kw := "var"
					if dd.Tok == token.CONST {
						kw = "const"
					}
					for _, n := range s.Names {
						if ast.IsExported(n.Name) {
							out = append(out, kw+" "+n.Name)
						}
					}
				}
			}
		}
	}
	return out
}

func grpRecvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return grpRecvName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return grpRecvName(t.X)
	case *ast.IndexListExpr:
		return grpRecvName(t.X)
	case *ast.ParenExpr:
		return grpRecvName(t.X)
	}
	return "?"
}

func grpWebSymbols(p string, src []byte) []string {
	var out []string
	if strings.HasSuffix(p, ".svelte") {
		out = append(out, "component "+strings.TrimSuffix(path.Base(p), ".svelte"))
	}
	out = append(out, grpLineSymbols(src, func(line string) (string, bool) {
		mm := symWebLineRe.FindStringSubmatch(line)
		if mm == nil {
			return "", false
		}
		return mm[1] + " " + mm[2], true
	})...)
	return out
}

func grpLineSymbols(src []byte, f func(string) (string, bool)) []string {
	var out []string
	for _, line := range strings.Split(string(src), "\n") {
		if s, ok := f(strings.TrimRight(line, "\r")); ok {
			out = append(out, s)
		}
	}
	return out
}

// grpTag menghitung tag camelCase satu token untuk modul.
func grpTag(m Module) string {
	src := ""
	if m.Kind == kindGo {
		for _, f := range m.Files {
			if !isGoFile(f.Path) || strings.HasSuffix(f.Path, "_test.go") || f.abs == "" || f.Binary {
				continue
			}
			data, err := grpReadFile(f.abs)
			if err != nil {
				continue
			}
			pf, err := parser.ParseFile(token.NewFileSet(), f.Path, data, parser.PackageClauseOnly)
			if err != nil || pf == nil || pf.Name == nil {
				continue
			}
			src = pf.Name.Name
			break
		}
	}
	if src == "" {
		if m.Dir == "." {
			src = "root"
		} else {
			src = path.Base(m.Dir)
		}
	}
	tag := CamelTag(src)
	if !tagValidRe.MatchString(tag) {
		var sb strings.Builder
		for _, r := range tag {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				sb.WriteRune(r)
			}
		}
		tag = sb.String()
		if tag == "" {
			tag = "root"
		}
		if tag[0] >= '0' && tag[0] <= '9' {
			tag = "m" + tag
		}
		if tag[0] >= 'A' && tag[0] <= 'Z' {
			tag = strings.ToLower(tag[:1]) + tag[1:]
		}
	}
	return tag
}
