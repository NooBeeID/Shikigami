package grimoire

import (
	"fmt"
	"sort"
	"strings"
)

// IndexMaxBytes adalah batas ukuran index.md.
const IndexMaxBytes = 16384

// IndexHeader adalah header konstan index.md. Placeholder {{grimoire_dir}}
// ditulis literal dan diganti saat penyuntikan.
const IndexHeader = "# Grimoire\n" +
	"Index pengetahuan project ini. Baca index ini dulu, lalu buka halaman buku/bab yang relevan sebelum membaca source.\n" +
	"Halaman ada di {{grimoire_dir}}/books/<buku>/<bab>.md\n" +
	"Baris pertama tiap bab berisi \"Sumber:\" (path relatif repo) untuk verifikasi ke source.\n" +
	"Format baris: nama_buku (bab1, bab2) [tag,camelTag]\n" +
	"Baris berakhiran \"warning: stale\" berarti halaman tertinggal dari source; jalankan grimoire build.\n"

const indexStaleSuffix = " warning: stale, jalankan grimoire build"

// IndexLine membuat satu baris index untuk sebuah buku (tanpa newline).
func IndexLine(b Book) string {
	names := make([]string, 0, len(b.Chapters))
	stale := false
	for _, c := range b.Chapters {
		names = append(names, c.Name)
		if c.Stale {
			stale = true
		}
	}
	line := b.Name + " (" + strings.Join(names, ", ") + ")"
	if len(b.Tags) > 0 {
		line += " [" + strings.Join(b.Tags, ",") + "]"
	}
	if stale {
		line += indexStaleSuffix
	}
	return line
}

type indexEntry struct{ key, line string }

func indexRender(entries []indexEntry, dropped int) string {
	if len(entries) == 0 && dropped == 0 {
		return IndexHeader
	}
	var sb strings.Builder
	sb.WriteString(IndexHeader)
	sb.WriteString("\n")
	for _, e := range entries {
		sb.WriteString(e.line)
		sb.WriteString("\n")
	}
	if dropped > 0 {
		fmt.Fprintf(&sb, "... (%d entri dipotong)\n", dropped)
	}
	return sb.String()
}

func indexSort(entries []indexEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].key != entries[j].key {
			return entries[i].key < entries[j].key
		}
		return entries[i].line < entries[j].line
	})
}

// BuildIndex membuat isi index.md, melipat per awalan dan memotong bila
// melebihi IndexMaxBytes. Keluaran deterministik.
func BuildIndex(books []Book) string {
	sorted := make([]Book, len(books))
	copy(sorted, books)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	entries := make([]indexEntry, 0, len(sorted))
	for _, b := range sorted {
		entries = append(entries, indexEntry{key: b.Name, line: IndexLine(b)})
	}
	if out := indexRender(entries, 0); len(out) <= IndexMaxBytes {
		return out
	}

	type group struct {
		prefix   string
		books    []Book
		chapters int
		stale    bool
	}
	byPrefix := map[string]*group{}
	for _, b := range sorted {
		p := b.Name
		if i := strings.Index(p, "_"); i >= 0 {
			p = p[:i]
		}
		g := byPrefix[p]
		if g == nil {
			g = &group{prefix: p}
			byPrefix[p] = g
		}
		g.books = append(g.books, b)
		g.chapters += len(b.Chapters)
		for _, c := range b.Chapters {
			if c.Stale {
				g.stale = true
			}
		}
	}
	prefixes := make([]string, 0, len(byPrefix))
	for p, g := range byPrefix {
		if len(g.books) >= 2 {
			prefixes = append(prefixes, p)
		}
	}
	sort.Strings(prefixes)
	sort.SliceStable(prefixes, func(i, j int) bool {
		a, b := byPrefix[prefixes[i]], byPrefix[prefixes[j]]
		if len(a.books) != len(b.books) {
			return len(a.books) > len(b.books)
		}
		return a.chapters > b.chapters
	})

	for _, p := range prefixes {
		g := byPrefix[p]
		member := map[string]bool{}
		for _, b := range g.books {
			member[b.Name] = true
		}
		kept := make([]indexEntry, 0, len(entries))
		for _, e := range entries {
			// entri awal punya key == nama buku; entri lipatan lain tidak
			// bertabrakan karena awalan kelompok berbeda.
			if member[e.key] {
				continue
			}
			kept = append(kept, e)
		}
		line := fmt.Sprintf("%s/* (%d buku) [%s]", g.prefix, len(g.books), CamelTag(g.prefix))
		if g.stale {
			line += indexStaleSuffix
		}
		kept = append(kept, indexEntry{key: g.prefix, line: line})
		indexSort(kept)
		entries = kept
		if out := indexRender(entries, 0); len(out) <= IndexMaxBytes {
			return out
		}
	}

	dropped := 0
	for len(entries) > 0 {
		entries = entries[:len(entries)-1]
		dropped++
		if out := indexRender(entries, dropped); len(out) <= IndexMaxBytes {
			return out
		}
	}
	return indexRender(nil, dropped)
}
