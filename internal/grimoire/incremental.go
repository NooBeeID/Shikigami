package grimoire

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Summarizer adalah hook peringkas (tiket 4); nil = statik.
type Summarizer interface {
	Summarize(ctx context.Context, m Module, static string) (chapters map[string]string, err error)
}

// IncrementalOptions mengatur BuildIncremental.
type IncrementalOptions struct {
	ProjectPath string
	Full        bool
	Summarizer  Summarizer
}

// Result adalah ringkasan build inkremental.
type Result struct {
	Rebuilt, Removed, Failed, Skipped int
	Duration                          time.Duration
}

// Builder membangun grimoire secara inkremental.
type Builder struct {
	store  *Store
	logger *slog.Logger
	mu     sync.Mutex
	locks  map[string]*sync.Mutex
}

// NewBuilder membuat Builder.
func NewBuilder(store *Store, logger *slog.Logger) *Builder {
	return &Builder{store: store, logger: logger, locks: map[string]*sync.Mutex{}}
}

// Store mengembalikan store yang dipakai.
func (b *Builder) Store() *Store { return b.store }

func (b *Builder) incrementalLock(projectID string) *sync.Mutex {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.locks == nil {
		b.locks = map[string]*sync.Mutex{}
	}
	l, ok := b.locks[projectID]
	if !ok {
		l = &sync.Mutex{}
		b.locks[projectID] = l
	}
	return l
}

func (b *Builder) incrementalLogger() *slog.Logger {
	if b.logger != nil {
		return b.logger
	}
	return slog.Default()
}

// HashAll menghitung hash semua modul (kunci = Module.Name).
func HashAll(root string, mods []Module) (map[string]string, error) {
	out := make(map[string]string, len(mods))
	for _, m := range mods {
		h, err := HashModule(root, m)
		if err != nil {
			return nil, fmt.Errorf("hash modul %s: %w", m.Name, err)
		}
		out[m.Name] = h
	}
	return out, nil
}

// ApplyStale menandai bab stale sesuai state (murni, tidak mengubah masukan).
func ApplyStale(books []Book, st State) []Book {
	out := make([]Book, len(books))
	for i, bk := range books {
		cp := bk
		cp.Chapters = make([]Chapter, len(bk.Chapters))
		copy(cp.Chapters, bk.Chapters)
		stale := st.Modules[bk.Name].Stale
		for j := range cp.Chapters {
			cp.Chapters[j].Stale = stale
		}
		out[i] = cp
	}
	return out
}

// StaleCount menghitung modul stale.
func StaleCount(st State) int {
	n := 0
	for _, m := range st.Modules {
		if m.Stale {
			n++
		}
	}
	return n
}

func incrementalRoot(projectPath string) (string, error) {
	if !filepath.IsAbs(projectPath) {
		return "", fmt.Errorf("path project tidak valid: %s", projectPath)
	}
	root := filepath.Clean(projectPath)
	fi, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("path project tidak valid: %s: %w", projectPath, err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("path project tidak valid: %s", projectPath)
	}
	return root, nil
}

func incrementalScan(store *Store, projectID, root string) ([]Module, map[string]string, *State, error) {
	files, err := Scan(root)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("pindai project: %w", err)
	}
	mods := GroupModules(files)
	hashes, err := HashAll(root, mods)
	if err != nil {
		return nil, nil, nil, err
	}
	state, err := store.ReadState(projectID)
	if err != nil {
		if errors.Is(err, ErrNotBuilt) {
			return mods, hashes, nil, nil
		}
		return nil, nil, nil, err
	}
	return mods, hashes, &state, nil
}

// Check menghitung Diff tanpa menulis dan tanpa lock.
func (b *Builder) Check(projectID, projectPath string) (Diff, error) {
	if b.store == nil {
		return Diff{}, errors.New("store grimoire kosong")
	}
	if err := b.store.ValidateProject(projectID); err != nil {
		return Diff{}, err
	}
	root, err := incrementalRoot(projectPath)
	if err != nil {
		return Diff{}, err
	}
	mods, hashes, st, err := incrementalScan(b.store, projectID, root)
	if err != nil {
		return Diff{}, err
	}
	return ComputeDiff(mods, hashes, st), nil
}

func incrementalSafeBook(name string) bool {
	return name != "" && name == Snake(name)
}

func incrementalBookDir(store *Store, projectID, name string) string {
	return filepath.Join(store.BooksDir(projectID), name)
}

func incrementalRemoveBook(store *Store, projectID, name string) error {
	if !incrementalSafeBook(name) {
		return fmt.Errorf("nama buku tidak aman: %q", name)
	}
	return os.RemoveAll(incrementalBookDir(store, projectID, name))
}

// incrementalMdFiles mengembalikan nama berkas *.md (urut) di folder buku.
func incrementalMdFiles(store *Store, projectID, name string) []string {
	if !incrementalSafeBook(name) {
		return nil
	}
	ents, err := os.ReadDir(incrementalBookDir(store, projectID, name))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// incrementalPruneBook menghapus berkas .md lama yang tidak ada di halaman baru.
func (b *Builder) incrementalPruneBook(projectID, name string, keep map[string]bool) {
	for _, f := range incrementalMdFiles(b.store, projectID, name) {
		if keep[f] {
			continue
		}
		p := filepath.Join(incrementalBookDir(b.store, projectID, name), f)
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			b.incrementalLogger().Warn("grimoire: gagal hapus bab lama", "project", projectID, "path", p, "err", err)
		}
	}
}

// incrementalDiskChapters membaca bab yang benar-benar ada di disk.
func incrementalDiskChapters(store *Store, projectID string, m Module) []Chapter {
	name := Snake(m.Name)
	var out []Chapter
	for _, f := range incrementalMdFiles(store, projectID, name) {
		out = append(out, Chapter{
			Name:    strings.TrimSuffix(f, ".md"),
			Path:    "books/" + name + "/" + f,
			Sources: pagesCopy(m.Sources),
		})
	}
	return out
}

// incrementalSummarize memanggil Summarizer dengan recover; panic dianggap galat.
func incrementalSummarize(ctx context.Context, s Summarizer, m Module, static string) (ch map[string]string, err error) {
	defer func() {
		if r := recover(); r != nil {
			ch, err = nil, fmt.Errorf("summarizer panic: %v", r)
		}
	}()
	return s.Summarize(ctx, m, static)
}

// incrementalRebuildModule menulis ulang utuh satu modul. Mengembalikan
// summarizeFailed bila Summarizer gagal (halaman statik dipakai).
func (b *Builder) incrementalRebuildModule(ctx context.Context, projectID string, m Module, hash string, sum Summarizer) (summarizeFailed bool, err error) {
	// Penyimpangan kecil: footer hash memakai renderBooks agar identik dengan Build.
	_, pages := renderBooks([]Module{m}, map[string]string{m.Name: hash})
	if sum != nil {
		parts := make([]string, len(pages))
		for i, p := range pages {
			parts[i] = p.Content
		}
		chapters, serr := incrementalSummarize(ctx, sum, m, strings.Join(parts, "\n\n"))
		if serr != nil {
			summarizeFailed = true
			b.incrementalLogger().Warn("grimoire: peringkas gagal, pakai statik", "project", projectID, "module", m.Name, "err", serr)
		} else {
			for i := range pages {
				n := strings.TrimSuffix(filepath.Base(pages[i].Path), ".md")
				if c := chapters[n]; c != "" {
					pages[i].Content = c
				}
			}
		}
	}
	keep := make(map[string]bool, len(pages))
	for _, p := range pages {
		if err := b.store.WriteFile(projectID, p.Path, []byte(p.Content)); err != nil {
			return summarizeFailed, fmt.Errorf("tulis halaman %s: %w", p.Path, err)
		}
		keep[filepath.Base(p.Path)] = true
	}
	b.incrementalPruneBook(projectID, Snake(m.Name), keep)
	return summarizeFailed, nil
}

// BuildIncremental membangun ulang hanya modul yang berubah.
func (b *Builder) BuildIncremental(ctx context.Context, projectID string, opts IncrementalOptions) (res Result, retErr error) {
	if b.store == nil {
		return Result{}, errors.New("store grimoire kosong")
	}
	if opts.ProjectPath == "" {
		return Result{}, errors.New("projectPath wajib")
	}
	if err := b.store.ValidateProject(projectID); err != nil {
		return Result{}, err
	}
	if opts.Summarizer == nil {
		opts.Summarizer = SummarizerFor(projectID, opts.ProjectPath)
	}
	root, err := incrementalRoot(opts.ProjectPath)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	lock := b.incrementalLock(projectID)
	lock.Lock()
	defer lock.Unlock()
	start := time.Now()
	defer func() { res.Duration = time.Since(start) }()
	log := b.incrementalLogger()

	mods, hashes, st, err := incrementalScan(b.store, projectID, root)
	if err != nil {
		return Result{}, err
	}
	diff := ComputeDiff(mods, hashes, st)

	byName := make(map[string]Module, len(mods))
	for _, m := range mods {
		n := Snake(m.Name)
		if _, ok := byName[n]; !ok {
			byName[n] = m
		}
	}

	newMods := map[string]ModuleState{}
	if st != nil {
		for k, v := range st.Modules {
			newMods[k] = v
		}
	}

	var todo []ModuleChange
	if opts.Full {
		names := make([]string, 0, len(byName))
		for n := range byName {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			c := ModuleChange{Name: n, Kind: ChangeModified, NewHash: hashes[byName[n].Name]}
			if prev, ok := newMods[n]; ok {
				c.OldHash = prev.Hash
			}
			todo = append(todo, c)
		}
	} else {
		if diff.Empty() {
			return Result{Skipped: len(diff.Unchanged)}, nil
		}
		todo = diff.Pending()
		res.Skipped = len(diff.Unchanged)
	}

	changed := false
	processed := map[string]bool{}
	var ctxErr error

	for _, c := range todo {
		if e := ctx.Err(); e != nil {
			ctxErr = e
			break
		}
		m := byName[c.Name]
		hash := hashes[m.Name]
		prev, existed := newMods[c.Name]
		failedSum, werr := b.incrementalRebuildModule(ctx, projectID, m, hash, opts.Summarizer)
		if werr != nil {
			res.Failed++
			log.Error("grimoire: gagal menulis modul", "project", projectID, "module", c.Name, "err", werr)
			if existed {
				prev.Stale = true
				newMods[c.Name] = prev
				changed = true
			} else if rerr := incrementalRemoveBook(b.store, projectID, c.Name); rerr != nil {
				log.Warn("grimoire: gagal bersihkan buku baru yang gagal", "project", projectID, "module", c.Name, "err", rerr)
			}
			continue
		}
		src := make([]string, len(m.Sources))
		copy(src, m.Sources)
		newMods[c.Name] = ModuleState{Hash: hash, Sources: src, Stale: failedSum}
		res.Rebuilt++
		changed = true
		processed[c.Name] = true
	}

	if ctxErr == nil {
		for _, c := range diff.Removed {
			if e := ctx.Err(); e != nil {
				ctxErr = e
				break
			}
			if rerr := incrementalRemoveBook(b.store, projectID, c.Name); rerr != nil {
				log.Error("grimoire: gagal hapus buku", "project", projectID, "module", c.Name, "err", rerr)
				continue
			}
			delete(newMods, c.Name)
			res.Removed++
			changed = true
		}
	}

	if changed || opts.Full && ctxErr == nil {
		books, _ := RenderBooks(mods)
		pending := map[string]bool{}
		for _, c := range todo {
			if !processed[c.Name] {
				pending[c.Name] = true
			}
		}
		kept := books[:0:0]
		for _, bk := range books {
			if _, ok := newMods[bk.Name]; !ok {
				continue
			}
			if pending[bk.Name] {
				if ch := incrementalDiskChapters(b.store, projectID, byName[bk.Name]); len(ch) > 0 {
					bk.Chapters = ch
				}
			}
			kept = append(kept, bk)
		}
		newState := State{ProjectID: projectID, BuiltAt: time.Now().UTC(), Modules: newMods}
		kept = ApplyStale(kept, newState)
		index := BuildIndex(kept)
		if err := b.store.WriteFile(projectID, "index.md", []byte(index)); err != nil {
			return res, fmt.Errorf("tulis index.md: %w", err)
		}
		if err := b.store.WriteState(projectID, newState); err != nil {
			return res, fmt.Errorf("tulis state.json: %w", err)
		}
	}

	if ctxErr != nil {
		return res, ctxErr
	}
	return res, nil
}
