// Package appconfig menyimpan konfigurasi aplikasi (zoom, posisi jendela) di config.json.
package appconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/NooBeeID/shikigami/internal/paths"
)

type Window struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type Config struct {
	Zoom   int     `json:"zoom"`
	Window *Window `json:"window,omitempty"`
}

func Default() Config { return Config{Zoom: 100} }

type Store struct {
	mu   sync.Mutex
	path string
}

func Open(path string) *Store { return &Store{path: path} }

func DefaultPath() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func clampZoom(z float64) int {
	if math.IsNaN(z) || math.IsInf(z, 0) || z == 0 {
		return 100
	}
	r := int(math.Round(z))
	if r < 50 {
		return 50
	}
	if r > 300 {
		return 300
	}
	return r
}

func (s *Store) readRaw() (map[string]any, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil || m == nil {
		log.Printf("appconfig: %s rusak, memakai bawaan: %v", s.path, err)
		return map[string]any{}, nil
	}
	return m, nil
}

func decode(raw map[string]any) Config {
	cfg := Default()
	if b, err := json.Marshal(raw); err == nil {
		var c Config
		if json.Unmarshal(b, &c) == nil {
			cfg = c
		}
	}
	cfg.Zoom = clampZoom(float64(cfg.Zoom))
	return cfg
}

func (s *Store) Get() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := s.readRaw()
	if err != nil {
		return Default(), err
	}
	return decode(raw), nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func (s *Store) Patch(patch map[string]any) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := s.readRaw()
	if err != nil {
		return Default(), err
	}
	for k, v := range patch {
		switch k {
		case "zoom":
			if f, ok := toFloat(v); ok {
				raw["zoom"] = clampZoom(f)
			}
		case "window":
			pm, ok := v.(map[string]any)
			if !ok {
				continue
			}
			cur, _ := raw["window"].(map[string]any)
			if cur == nil {
				cur = map[string]any{}
			}
			for _, wk := range []string{"x", "y", "w", "h"} {
				if f, ok := toFloat(pm[wk]); ok {
					cur[wk] = int(math.Round(f))
				}
			}
			raw["window"] = cur
		}
	}
	if err := s.write(raw); err != nil {
		return Default(), err
	}
	return decode(raw), nil
}

func (s *Store) write(raw map[string]any) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("gagal membuat folder config: %w", err)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := func() { _ = os.Remove(name) }
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(name, s.path); err != nil {
		cleanup()
		return err
	}
	return nil
}
