package grimoire

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/paths"
)

// Measure adalah ukuran teks suntikan.
type Measure struct{ IndexBytes, EstTokens, Pages, LawsBytes int }

// MeasureInjection mengukur teks suntikan project. Tanpa grimoire: Measure{}, nil.
func MeasureInjection(projectID string) (Measure, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return Measure{}, err
	}
	s := NewStore(dir)
	text, ok, err := ComposeInjection(projectID)
	if err != nil {
		return Measure{}, err
	}
	if !ok {
		return Measure{}, nil
	}
	m := Measure{IndexBytes: len(text), EstTokens: (len(text) + 3) / 4}

	b, err := s.ReadFile(projectID, "laws.md")
	switch {
	case err == nil:
		m.LawsBytes = len(b)
	case errors.Is(err, os.ErrNotExist):
	default:
		return Measure{}, err
	}

	werr := filepath.WalkDir(s.BooksDir(projectID), func(_ string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || !e.Type().IsRegular() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			m.Pages++
		}
		return nil
	})
	if werr != nil && !errors.Is(werr, fs.ErrNotExist) {
		return Measure{}, werr
	}
	return m, nil
}

// InjectDeps adalah dependensi RegisterInject.
type InjectDeps struct {
	Store  *Store
	Known  func(ctx context.Context, projectID string) (bool, error)
	Logger *slog.Logger
}

type measureLawsGetParams struct {
	ProjectID string `json:"projectId"`
}

type measureLawsSetParams struct {
	ProjectID string `json:"projectId"`
	Content   string `json:"content"`
}

func measureDecode(params json.RawMessage, v any) error {
	if len(params) == 0 {
		return ipc.NewError(ipc.ErrInvalid, "invalid params: kosong")
	}
	if err := json.Unmarshal(params, v); err != nil {
		return ipc.NewError(ipc.ErrInvalid, "invalid params: "+err.Error())
	}
	return nil
}

func measureLog(d InjectDeps, msg string, err error) {
	if d.Logger != nil {
		d.Logger.Error(msg, "err", err)
	}
}

func measureCheckProject(ctx context.Context, d InjectDeps, id string) error {
	if id == "" {
		return ipc.NewError(ipc.ErrInvalid, "projectId kosong")
	}
	if err := d.Store.ValidateProject(id); err != nil {
		return ipc.NewError(ipc.ErrInvalid, "projectId tidak valid")
	}
	if d.Known != nil {
		ok, err := d.Known(ctx, id)
		if err != nil {
			measureLog(d, "grimoire: cek project dikenal gagal", err)
			return ipc.NewError(ipc.ErrInternal, err.Error())
		}
		if !ok {
			return ipc.NewError(ipc.ErrInvalid, "project tidak dikenal")
		}
	}
	return nil
}

func measureMapErr(d InjectDeps, err error) error {
	var ie *ipc.Error
	switch {
	case errors.As(err, &ie):
		return err
	case errors.Is(err, ErrLawsTooLarge):
		return ipc.NewError(ipc.ErrInvalid, "laws melebihi batas 16384 byte")
	case errors.Is(err, ErrInvalidProject), errors.Is(err, ErrInvalidPath):
		return ipc.NewError(ipc.ErrInvalid, err.Error())
	default:
		measureLog(d, "grimoire: laws gagal", err)
		return ipc.NewError(ipc.ErrInternal, err.Error())
	}
}

// RegisterInject mendaftarkan grimoire.laws.get dan grimoire.laws.set.
func RegisterInject(srv *ipc.Server, d InjectDeps) {
	if srv == nil || d.Store == nil {
		return
	}
	srv.Register("grimoire.laws.get", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p measureLawsGetParams
		if err := measureDecode(params, &p); err != nil {
			return nil, err
		}
		if err := measureCheckProject(ctx, d, p.ProjectID); err != nil {
			return nil, err
		}
		content, err := d.Store.ReadLaws(p.ProjectID)
		if err != nil {
			return nil, measureMapErr(d, err)
		}
		return map[string]any{"content": content}, nil
	})
	srv.Register("grimoire.laws.set", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p measureLawsSetParams
		if err := measureDecode(params, &p); err != nil {
			return nil, err
		}
		if err := measureCheckProject(ctx, d, p.ProjectID); err != nil {
			return nil, err
		}
		if err := d.Store.WriteLaws(p.ProjectID, p.Content); err != nil {
			return nil, measureMapErr(d, err)
		}
		srv.Publish("grimoire.changed", map[string]any{"projectId": p.ProjectID})
		return map[string]any{"lawsBytes": len(p.Content)}, nil
	})
}
