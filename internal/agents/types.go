// Paket agents menyimpan registry kategori dan agent global (agents.json),
// memvalidasinya, me-resolve referensi, dan mengeksposnya lewat IPC.
package agents

import (
	"errors"

	"github.com/NooBeeID/shikigami/internal/provider"
)

// Category adalah kategori peran agent.
type Category struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	Builtin bool   `json:"builtin"`
}

// Agent adalah konfigurasi agent global.
type Agent struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CategoryID string `json:"categoryId"`
	ProviderID string `json:"providerId"`
	Model      string `json:"model"`
	IsDefault  bool   `json:"isDefault"`
}

// fileState adalah isi agents.json.
type fileState struct {
	Version    int        `json:"version"`
	Categories []Category `json:"categories"`
	Agents     []Agent    `json:"agents"`
}

const fileVersion = 1

// ErrNotFound menandai referensi agent/kategori yang tidak dikenal.
var ErrNotFound = errors.New("agent atau kategori tidak ditemukan")

// errStorage membungkus galat I/O penyimpanan.
var errStorage = errors.New("gagal mengakses penyimpanan agent")

// guardError membungkus galat penolakan dari delete guard.
type guardError struct{ err error }

func (g *guardError) Error() string { return g.err.Error() }
func (g *guardError) Unwrap() error { return g.err }

// ProviderLookup mencari provider berdasarkan id.
type ProviderLookup interface {
	Get(id string) (provider.Provider, bool)
}

type globalLookup struct{}

func (globalLookup) Get(id string) (provider.Provider, bool) { return provider.Get(id) }

const (
	idOrchestrator = "orchestrator"
	idResearcher   = "researcher"
	idPlanner      = "planner"
	idExecutor     = "executor"
)
