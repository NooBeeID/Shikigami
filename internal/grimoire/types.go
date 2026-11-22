// Package grimoire membangun dan menyimpan knowledge internal per project.
package grimoire

import (
	"errors"
	"time"
)

// Chapter adalah satu bab (halaman) di dalam sebuah buku.
type Chapter struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Sources []string `json:"sources"`
	Stale   bool     `json:"stale"`
}

// Book adalah satu buku (per modul) berisi bab-bab.
type Book struct {
	Name     string    `json:"name"`
	Tags     []string  `json:"tags"`
	Chapters []Chapter `json:"chapters"`
}

// ModuleState menyimpan hash dan sumber satu modul pada state.json.
type ModuleState struct {
	Hash    string   `json:"hash"`
	Sources []string `json:"sources"`
	Stale   bool     `json:"stale,omitempty"`
}

// State adalah isi state.json.
type State struct {
	ProjectID string                 `json:"projectId"`
	BuiltAt   time.Time              `json:"builtAt"`
	Modules   map[string]ModuleState `json:"modules"`
}

// Summary adalah hasil grimoire.build.
type Summary struct {
	Books      int `json:"books"`
	Pages      int `json:"pages"`
	IndexBytes int `json:"indexBytes"`
}

// Sentinel galat kontrak paket grimoire.
var (
	// ErrNotBuilt berarti grimoire project belum pernah dibangun.
	ErrNotBuilt = errors.New("grimoire belum dibangun")
	// ErrInvalidPath berarti path relatif kosong, absolut, atau keluar direktori.
	ErrInvalidPath = errors.New("path grimoire tidak valid")
	// ErrInvalidProject berarti projectId kosong atau mengandung pemisah path.
	ErrInvalidProject = errors.New("projectId tidak valid")
)
