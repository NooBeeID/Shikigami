package grimoire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sync"

	"github.com/NooBeeID/shikigami/internal/ipc"
	"github.com/NooBeeID/shikigami/internal/paths"
)

const settingsFile = "settings.json"

// Settings adalah pengaturan LLM per project.
type Settings struct {
	LLM        bool `json:"llm"`
	MaxModules *int `json:"maxModules"` // nil = tanpa batas
}

// ErrInvalidMaxModules dikembalikan bila maxModules < 1.
var ErrInvalidMaxModules = errors.New("maxModules harus integer >= 1")

// LLMDeps dipakai bersama oleh fitur LLM.
type LLMDeps struct {
	Store       *Store
	ResolvePath func(ctx context.Context, projectID string) (string, error)
	Publish     func(projectID string) // nullable; publikasi grimoire.changed {projectId}
	Logger      *slog.Logger
}

// settingsMu men-serialisasi read-modify-write pada settings.
var settingsMu sync.Mutex

func settingsStore() (*Store, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return nil, err
	}
	return NewStore(dir), nil
}

func settingsLoad(st *Store, projectID string) (Settings, error) {
	data, err := st.ReadFile(projectID, settingsFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Settings{}, nil
		}
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("settings.json rusak: %w", err)
	}
	if s.MaxModules != nil && *s.MaxModules < 1 {
		s.MaxModules = nil
	}
	return s, nil
}

func settingsSave(st *Store, projectID string, s Settings) error {
	if s.MaxModules != nil && *s.MaxModules < 1 {
		return ErrInvalidMaxModules
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return st.WriteFile(projectID, settingsFile, data)
}

// LoadSettings membaca settings project (berkas tak ada = default).
func LoadSettings(projectID string) (Settings, error) {
	st, err := settingsStore()
	if err != nil {
		return Settings{}, err
	}
	return settingsLoad(st, projectID)
}

// SaveSettings menyimpan settings project secara atomik.
func SaveSettings(projectID string, s Settings) error {
	st, err := settingsStore()
	if err != nil {
		return err
	}
	return settingsSave(st, projectID, s)
}

type settingsService struct {
	d   LLMDeps
	srv *ipc.Server
}

func (s *settingsService) store() (*Store, error) {
	if s.d.Store != nil {
		return s.d.Store, nil
	}
	st, err := settingsStore()
	if err != nil {
		return nil, ipc.NewError(ipc.ErrInternal, err.Error())
	}
	return st, nil
}

func (s *settingsService) check(projectID string) (*Store, error) {
	if projectID == "" {
		return nil, ipc.NewError(ipc.ErrInvalid, "projectId wajib diisi")
	}
	st, err := s.store()
	if err != nil {
		return nil, err
	}
	if err := st.ValidateProject(projectID); err != nil {
		return nil, ipc.NewError(ipc.ErrInvalid, err.Error())
	}
	return st, nil
}

func (s *settingsService) logErr(msg string, id string, err error) {
	if s.d.Logger != nil && err != nil {
		s.d.Logger.Error(msg, "project", id, "err", err)
	}
}

// RegisterLLM mendaftarkan grimoire.settings.get, grimoire.settings.set, dan grimoire.estimate.
func RegisterLLM(srv *ipc.Server, d LLMDeps) {
	svc := &settingsService{d: d, srv: srv}

	srv.Register("grimoire.settings.get", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			ProjectID string `json:"projectId"`
		}
		if err := rpcDecode(params, &p); err != nil {
			return nil, err
		}
		st, err := svc.check(p.ProjectID)
		if err != nil {
			return nil, err
		}
		set, err := settingsLoad(st, p.ProjectID)
		if err != nil {
			svc.logErr("gagal membaca settings grimoire", p.ProjectID, err)
			return nil, rpcMapErr(err)
		}
		return set, nil
	})

	srv.Register("grimoire.settings.set", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			ProjectID  string          `json:"projectId"`
			LLM        *bool           `json:"llm"`
			MaxModules json.RawMessage `json:"maxModules"`
		}
		if err := rpcDecode(params, &p); err != nil {
			return nil, err
		}
		st, err := svc.check(p.ProjectID)
		if err != nil {
			return nil, err
		}
		invalidMax := ipc.NewError(ipc.ErrInvalid, "invalid params: "+ErrInvalidMaxModules.Error())
		raw := bytes.TrimSpace(p.MaxModules)
		sent := len(raw) > 0
		clearMax := sent && bytes.Equal(raw, []byte("null"))
		var maxVal int
		if sent && !clearMax {
			var f float64
			if err := json.Unmarshal(raw, &f); err != nil || f != math.Trunc(f) || f < 1 || f > math.MaxInt32 {
				return nil, invalidMax
			}
			maxVal = int(f)
		}

		settingsMu.Lock()
		cur, err := settingsLoad(st, p.ProjectID)
		if err != nil {
			settingsMu.Unlock()
			svc.logErr("gagal membaca settings grimoire", p.ProjectID, err)
			return nil, ipc.NewError(ipc.ErrInternal, err.Error())
		}
		if p.LLM == nil && !sent {
			settingsMu.Unlock()
			return cur, nil // no-op
		}
		if p.LLM != nil {
			cur.LLM = *p.LLM
		}
		if clearMax {
			cur.MaxModules = nil
		} else if sent {
			v := maxVal
			cur.MaxModules = &v
		}
		if err := settingsSave(st, p.ProjectID, cur); err != nil {
			settingsMu.Unlock()
			if errors.Is(err, ErrInvalidMaxModules) {
				return nil, invalidMax
			}
			svc.logErr("gagal menyimpan settings grimoire", p.ProjectID, err)
			return nil, rpcMapErr(err)
		}
		settingsMu.Unlock()

		if d.Publish != nil {
			d.Publish(p.ProjectID)
		} else {
			srv.Publish("grimoire.changed", map[string]any{"projectId": p.ProjectID})
		}
		return cur, nil
	})

	srv.Register("grimoire.estimate", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			ProjectID string `json:"projectId"`
		}
		if err := rpcDecode(params, &p); err != nil {
			return nil, err
		}
		if _, err := svc.check(p.ProjectID); err != nil {
			return nil, err
		}
		if d.ResolvePath == nil {
			return nil, ipc.NewError(ipc.ErrInternal, "resolver project belum terpasang")
		}
		path, err := d.ResolvePath(ctx, p.ProjectID)
		if err != nil {
			return nil, rpcMapErr(err)
		}
		est, err := EstimateBuild(p.ProjectID, path)
		if err != nil {
			svc.logErr("gagal menghitung estimasi grimoire", p.ProjectID, err)
			return nil, rpcMapErr(err)
		}
		return est, nil
	})
}
