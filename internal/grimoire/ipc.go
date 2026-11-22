package grimoire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

// Deps adalah dependensi layanan IPC grimoire.
type Deps struct {
	Store       *Store
	ResolvePath func(ctx context.Context, projectID string) (string, error)
	Logger      *slog.Logger
}

type projectParams struct {
	ProjectID string `json:"projectId"`
	Full      bool   `json:"full"`
}

type readParams struct {
	ProjectID string `json:"projectId"`
	Path      string `json:"path"`
}

type inflight struct {
	done chan struct{}
	sum  Summary
	err  error
}

type service struct {
	d        Deps
	srv      *ipc.Server
	mu       sync.Mutex
	inflight map[string]*inflight
}

type statusResult struct {
	Exists        bool   `json:"exists"`
	Books         int    `json:"books"`
	Pages         int    `json:"pages"`
	StaleCount    int    `json:"staleCount"`
	Building      bool   `json:"building"`
	IndexBytes    int    `json:"indexBytes"`
	EstTokens     int    `json:"estTokens"`
	LawsBytes     int    `json:"lawsBytes"`
	LLMEnabled    bool   `json:"llmEnabled"`
	LastBuildCost int    `json:"lastBuildCost"`
	Dir           string `json:"dir"`
}

type buildResult struct {
	Books      int  `json:"books"`
	Pages      int  `json:"pages"`
	IndexBytes int  `json:"indexBytes"`
	Rebuilt    int  `json:"rebuilt"`
	Removed    int  `json:"removed"`
	Failed     int  `json:"failed"`
	Building   bool `json:"building,omitempty"`
}

type readResult struct {
	Content string `json:"content"`
	Size    int    `json:"size"`
}

func rpcDecode(params json.RawMessage, v any) error {
	if len(params) == 0 {
		return ipc.NewError(ipc.ErrInvalid, "invalid params: kosong")
	}
	if err := json.Unmarshal(params, v); err != nil {
		return ipc.NewError(ipc.ErrInvalid, "invalid params: "+err.Error())
	}
	return nil
}

func rpcMapErr(err error) error {
	if err == nil {
		return nil
	}
	var ie *ipc.Error
	if errors.As(err, &ie) {
		return err
	}
	switch {
	case errors.Is(err, ErrInvalidProject), errors.Is(err, ErrInvalidPath):
		return ipc.NewError(ipc.ErrInvalid, err.Error())
	case errors.Is(err, ErrNotBuilt), errors.Is(err, os.ErrNotExist):
		return ipc.NewError(ipc.ErrNotFound, err.Error())
	}
	return ipc.NewError(ipc.ErrInternal, err.Error())
}

func (s *service) checkID(projectID string) error {
	if s.d.Store == nil {
		return ipc.NewError(ipc.ErrInternal, "store grimoire belum terpasang")
	}
	if projectID == "" {
		return ipc.NewError(ipc.ErrInvalid, "projectId wajib diisi")
	}
	if err := s.d.Store.ValidateProject(projectID); err != nil {
		return ipc.NewError(ipc.ErrInvalid, err.Error())
	}
	return nil
}

func (s *service) resolve(ctx context.Context, projectID string) (string, error) {
	if err := s.checkID(projectID); err != nil {
		return "", err
	}
	if s.d.ResolvePath == nil {
		return "", ipc.NewError(ipc.ErrInternal, "resolver project belum terpasang")
	}
	p, err := s.d.ResolvePath(ctx, projectID)
	if err != nil {
		return "", rpcMapErr(err)
	}
	return p, nil
}

func (s *service) building(projectID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.inflight[projectID]
	return ok
}

func (s *service) publish(projectID string) {
	if s.srv == nil {
		return
	}
	s.srv.Publish("grimoire.changed", map[string]any{"projectId": projectID})
}

func (s *service) wait(ctx context.Context, fl *inflight) (Summary, error) {
	select {
	case <-fl.done:
		return fl.sum, fl.err
	case <-ctx.Done():
		return Summary{}, ctx.Err()
	}
}

func (s *service) build(ctx context.Context, projectID string) (Summary, error) {
	path, err := s.resolve(ctx, projectID)
	if err != nil {
		return Summary{}, err
	}
	s.mu.Lock()
	if fl, ok := s.inflight[projectID]; ok {
		s.mu.Unlock()
		return s.wait(ctx, fl)
	}
	fl := &inflight{done: make(chan struct{})}
	s.inflight[projectID] = fl
	s.mu.Unlock()

	go s.run(projectID, path, fl)
	return s.wait(ctx, fl)
}

func (s *service) run(projectID, path string, fl *inflight) {
	s.publish(projectID)
	func() {
		defer func() {
			if r := recover(); r != nil {
				fl.err = fmt.Errorf("panik saat build grimoire: %v", r)
			}
		}()
		fl.sum, fl.err = Build(s.d.Store, projectID, path)
	}()
	s.mu.Lock()
	delete(s.inflight, projectID)
	s.mu.Unlock()
	close(fl.done)
	s.publish(projectID)
	if s.d.Logger != nil {
		if fl.err != nil {
			s.d.Logger.Warn("build grimoire gagal", "project", projectID, "err", fl.err)
		} else {
			s.d.Logger.Info("build grimoire selesai", "project", projectID,
				"books", fl.sum.Books, "pages", fl.sum.Pages)
		}
	}
}

func (s *service) status(id string) statusResult {
	dir := s.d.Store.Dir(id)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	res := statusResult{Dir: dir, Building: s.building(id) || RefreshBuilding(id), Exists: s.d.Store.Exists(id)}
	res.StaleCount = RefreshStaleCount(id)
	if !res.Exists {
		return res
	}
	if st, err := os.Stat(s.d.Store.IndexPath(id)); err == nil {
		res.IndexBytes = int(st.Size())
	}
	booksDir := s.d.Store.BooksDir(id)
	entries, _ := os.ReadDir(booksDir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		res.Books++
		files, _ := os.ReadDir(filepath.Join(booksDir, e.Name()))
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
				res.Pages++
			}
		}
	}
	res.EstTokens = res.IndexBytes / 4
	// Ukuran suntikan penuh (index + laws) dari tiket 02; galat hanya dicatat.
	if m, err := MeasureInjection(id); err != nil {
		if s.d.Logger != nil {
			s.d.Logger.Warn("ukur suntikan grimoire gagal", "project", id, "err", err)
		}
	} else {
		res.IndexBytes, res.EstTokens, res.Pages, res.LawsBytes = m.IndexBytes, m.EstTokens, m.Pages, m.LawsBytes
	}
	return res
}

// indexTags mengambil tag dari baris index buku: "<buku> (... [a, b])".
func indexTags(index, book string) []string {
	tags := []string{}
	for _, line := range strings.Split(index, "\n") {
		line = strings.TrimLeft(line, "-* \t")
		if !strings.HasPrefix(line, book+" (") {
			continue
		}
		l := strings.LastIndex(line, "[")
		r := strings.LastIndex(line, "]")
		if l < 0 || r <= l {
			return tags
		}
		for _, t := range strings.Split(line[l+1:r], ",") {
			if t = strings.TrimSpace(t); t != "" {
				tags = append(tags, t)
			}
		}
		return tags
	}
	return tags
}

func (s *service) listBooks(id string) []Book {
	books := []Book{}
	if !s.d.Store.Exists(id) {
		return books
	}
	booksDir := s.d.Store.BooksDir(id)
	entries, err := os.ReadDir(booksDir)
	if err != nil {
		return books
	}
	index := ""
	if data, err := s.d.Store.ReadFile(id, "index.md"); err == nil {
		index = string(data)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(booksDir, e.Name()))
		if err != nil {
			continue
		}
		sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
		chapters := []Chapter{}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(f.Name(), ".md")
			chapters = append(chapters, Chapter{
				Name:    name,
				Path:    "books/" + e.Name() + "/" + name + ".md",
				Sources: []string{},
			})
		}
		if len(chapters) == 0 {
			continue
		}
		books = append(books, Book{Name: e.Name(), Tags: indexTags(index, e.Name()), Chapters: chapters})
	}
	return books
}

// Register memasang method IPC grimoire.* pada server.
func Register(srv *ipc.Server, d Deps) {
	s := &service{d: d, srv: srv, inflight: map[string]*inflight{}}

	srv.Register("grimoire.build", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p projectParams
		if err := rpcDecode(params, &p); err != nil {
			return nil, err
		}
		if RefreshActive() {
			path, err := s.resolve(ctx, p.ProjectID)
			if err != nil {
				return nil, err
			}
			res, started, err := RefreshRun(ctx, p.ProjectID, path, p.Full)
			if err != nil {
				return nil, rpcMapErr(err)
			}
			if !started {
				return buildResult{Building: true}, nil
			}
			st := s.status(p.ProjectID)
			return buildResult{Books: st.Books, Pages: st.Pages, IndexBytes: st.IndexBytes,
				Rebuilt: res.Rebuilt, Removed: res.Removed, Failed: res.Failed}, nil
		}
		sum, err := s.build(ctx, p.ProjectID)
		if err != nil {
			return nil, rpcMapErr(err)
		}
		return sum, nil
	})
	srv.Register("grimoire.status", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p projectParams
		if err := rpcDecode(params, &p); err != nil {
			return nil, err
		}
		if err := s.checkID(p.ProjectID); err != nil {
			return nil, err
		}
		return s.status(p.ProjectID), nil
	})
	srv.Register("grimoire.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p projectParams
		if err := rpcDecode(params, &p); err != nil {
			return nil, err
		}
		if err := s.checkID(p.ProjectID); err != nil {
			return nil, err
		}
		return RefreshApplyStale(p.ProjectID, s.listBooks(p.ProjectID)), nil
	})
	srv.Register("grimoire.read", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p readParams
		if err := rpcDecode(params, &p); err != nil {
			return nil, err
		}
		if err := s.checkID(p.ProjectID); err != nil {
			return nil, err
		}
		if p.Path == "" {
			return nil, ipc.NewError(ipc.ErrInvalid, "path wajib diisi")
		}
		data, err := s.d.Store.ReadFile(p.ProjectID, p.Path)
		if err != nil {
			return nil, rpcMapErr(err)
		}
		return readResult{Content: string(data), Size: len(data)}, nil
	})
}
