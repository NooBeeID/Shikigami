package thread

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// AgentsConfig konfigurasi InstallAgents.
type AgentsConfig struct {
	Manager       *Manager
	Sessions      SessionAPI
	Workspaces    WorkspaceSource
	DataDir       string
	Logger        *slog.Logger
	Agents        AgentResolver                   // opsional; nil = perilaku lama
	OnSessionExit func(fn func(sessionID string)) // boleh dipanggil beberapa kali
}

// Agents adalah kumpulan komponen agent thread yang terpasang.
type Agents struct {
	MCP     *MCPServer
	Router  Router
	Spawner *Spawner
	Guard   *AntiLoop
}

var agentsInstallOnce sync.Once

// InstallAgents menyambungkan MCP server, router, anti-loop, dan spawner. Hanya sekali.
func InstallAgents(ctx context.Context, cfg AgentsConfig) (*Agents, error) {
	called := false
	var out *Agents
	var err error
	agentsInstallOnce.Do(func() {
		called = true
		out, err = installAgents(ctx, cfg)
	})
	if !called {
		return nil, errors.New("thread.InstallAgents sudah dipanggil")
	}
	return out, err
}

func installAgents(ctx context.Context, cfg AgentsConfig) (*Agents, error) {
	if cfg.Manager == nil {
		return nil, errors.New("thread.InstallAgents: Manager wajib diisi")
	}
	srv := NewMCPServer(MCPConfig{Manager: cfg.Manager, Sessions: cfg.Sessions, DataDir: cfg.DataDir, Logger: cfg.Logger})
	if _, err := srv.Start(ctx); err != nil {
		return nil, err
	}
	if cfg.OnSessionExit != nil {
		cfg.OnSessionExit(srv.ReleaseSession)
	}
	InstallLaunchContribution(srv, cfg.Manager, cfg.DataDir, cfg.Logger)
	guard := NewAntiLoop(cfg.Manager)
	router := NewRouter(RouterConfig{Manager: cfg.Manager, Guard: guard, Logger: cfg.Logger})
	RegisterCommonTools(ToolDeps{Manager: cfg.Manager, Router: router, Logger: cfg.Logger})
	sp := NewSpawner(SpawnerConfig{
		Manager:    cfg.Manager,
		Sessions:   cfg.Sessions,
		Workspaces: cfg.Workspaces,
		Router:     router,
		DataDir:    cfg.DataDir,
		Logger:     cfg.Logger,
		Agents:     cfg.Agents,
	})
	sp.RegisterTool()
	ToolsSection = AgentToolsSection
	return &Agents{MCP: srv, Router: router, Spawner: sp, Guard: guard}, nil
}
