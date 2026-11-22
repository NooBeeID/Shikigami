package grimoire

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

const (
	pagesSplitFiles  = 40
	pagesMaxSources  = 20
	pagesHashLen     = 12
	pagesRestChapter = "lainnya"
)

// Page adalah satu halaman bab yang akan ditulis ke disk.
type Page struct {
	Path    string // relatif direktori grimoire: books/<buku>/<bab>.md
	Content string
}

func pagesSourcesLine(sources []string) string {
	if len(sources) == 0 {
		return "Sumber: -"
	}
	if len(sources) <= pagesMaxSources {
		return "Sumber: " + strings.Join(sources, ", ")
	}
	return "Sumber: " + strings.Join(sources[:pagesMaxSources], ", ") + ", ..."
}

func renderChapterBody(sources []string, files []FileInfo, symbols []string, symbolNote string, truncated int, hash string) string {
	sorted := make([]FileInfo, len(files))
	copy(sorted, files)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	var sb strings.Builder
	sb.WriteString(pagesSourcesLine(sources))
	sb.WriteString("\n\n## Berkas\n")
	if len(sorted) == 0 {
		sb.WriteString("(tidak ada)\n")
	}
	for _, f := range sorted {
		extra := ""
		if f.Binary {
			extra += ", biner"
		}
		if f.Large {
			extra += ", besar"
		}
		fmt.Fprintf(&sb, "- %s (%d B%s)\n", f.Path, f.Size, extra)
	}

	sb.WriteString("\n## Simbol\n")
	if len(symbols) > 0 {
		for _, s := range symbols {
			sb.WriteString("- " + s + "\n")
		}
	} else if symbolNote != "" {
		sb.WriteString(symbolNote + "\n")
	} else {
		sb.WriteString("(tidak ada)\n")
	}
	if truncated > 0 {
		fmt.Fprintf(&sb, "(dipotong: %d berkas lagi)\n", truncated)
	}

	sb.WriteString("\n## Hash\n")
	if hash == "" {
		sb.WriteString("(belum dihitung)\n")
	} else {
		if len(hash) > pagesHashLen {
			hash = hash[:pagesHashLen]
		}
		sb.WriteString(hash + "\n")
	}
	return sb.String()
}

// RenderChapter merender satu modul sebagai satu bab (hash belum dihitung).
func RenderChapter(m Module) string {
	return renderChapterBody(m.Sources, m.Files, m.Symbols, "", m.Truncated, "")
}

// RenderBooks merender modul menjadi buku dan halaman, urut nama.
func RenderBooks(mods []Module) ([]Book, []Page) {
	return renderBooks(mods, nil)
}

func pagesTags(m Module) []string {
	tags := []string{}
	if m.Tag != "" {
		tags = append(tags, m.Tag)
	}
	if m.Dir != "" && m.Dir != "." && m.Dir != "/" {
		parent := path.Dir(m.Dir)
		if parent != "" && parent != "." && parent != "/" {
			t := CamelTag(path.Base(parent))
			if t != "" && t != m.Tag {
				tags = append(tags, t)
			}
		}
	}
	return tags
}

func pagesCopy(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	return out
}

type pagesChapterSpec struct {
	name    string
	sources []string
	files   []FileInfo
}

func pagesSplit(m Module) []pagesChapterSpec {
	subs := map[string]*pagesChapterSpec{}
	var rest []FileInfo
	for _, f := range m.Files {
		rel := f.Path
		if m.Dir != "." {
			rel = strings.TrimPrefix(rel, m.Dir+"/")
		}
		i := strings.Index(rel, "/")
		if i < 0 {
			rest = append(rest, f)
			continue
		}
		sub := rel[:i]
		s := subs[sub]
		if s == nil {
			s = &pagesChapterSpec{name: Snake(sub)}
			subs[sub] = s
		}
		s.files = append(s.files, f)
	}
	if len(subs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(subs))
	for k := range subs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	specs := make([]pagesChapterSpec, 0, len(keys)+1)
	for _, k := range keys {
		specs = append(specs, *subs[k])
	}
	if len(rest) > 0 {
		specs = append(specs, pagesChapterSpec{name: pagesRestChapter, files: rest})
	}
	// nama bab unik
	used := map[string]bool{}
	for i := range specs {
		name := specs[i].name
		if used[name] {
			for n := 2; ; n++ {
				cand := fmt.Sprintf("%s_%d", specs[i].name, n)
				if !used[cand] {
					name = cand
					break
				}
			}
		}
		used[name] = true
		specs[i].name = name
		srcs := make([]string, 0, len(specs[i].files))
		for _, f := range specs[i].files {
			srcs = append(srcs, f.Path)
		}
		sort.Strings(srcs)
		specs[i].sources = srcs
	}
	return specs
}

func renderBooks(mods []Module, hashes map[string]string) ([]Book, []Page) {
	sorted := make([]Module, len(mods))
	copy(sorted, mods)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	books := []Book{}
	pages := []Page{}
	for _, m := range sorted {
		bookName := Snake(m.Name)
		hash := hashes[m.Name]
		book := Book{Name: bookName, Tags: pagesTags(m), Chapters: []Chapter{}}

		var specs []pagesChapterSpec
		if len(m.Files) > pagesSplitFiles {
			specs = pagesSplit(m)
		}
		if len(specs) == 0 {
			p := "books/" + bookName + "/" + bookName + ".md"
			book.Chapters = append(book.Chapters, Chapter{Name: bookName, Path: p, Sources: pagesCopy(m.Sources)})
			pages = append(pages, Page{Path: p, Content: renderChapterBody(m.Sources, m.Files, m.Symbols, "", m.Truncated, hash)})
		} else {
			first := specs[0].name
			for i, s := range specs {
				p := "books/" + bookName + "/" + s.name + ".md"
				book.Chapters = append(book.Chapters, Chapter{Name: s.name, Path: p, Sources: pagesCopy(s.sources)})
				var content string
				if i == 0 {
					content = renderChapterBody(s.sources, s.files, m.Symbols, "", m.Truncated, hash)
				} else {
					content = renderChapterBody(s.sources, s.files, nil, "(simbol modul tercatat di bab "+first+")", 0, hash)
				}
				pages = append(pages, Page{Path: p, Content: content})
			}
		}
		books = append(books, book)
	}
	return books, pages
}
