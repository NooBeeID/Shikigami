package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aiterm/internal/ipc"
	"aiterm/internal/paths"
)

// version di-override lewat -ldflags "-X main.version=<v>".
var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	dataDir, err := paths.DataDir()
	if err != nil {
		logger.Error("gagal menentukan data dir", "err", err)
		return 1
	}
	socketPath, err := paths.SocketPath()
	if err != nil {
		logger.Error("gagal menentukan path socket", "err", err)
		return 1
	}

	deps := &Deps{
		DataDir:    dataDir,
		SocketPath: socketPath,
		Logger:     logger,
		Version:    version,
		Started:    time.Now(),
	}
	srv := ipc.NewServer(socketPath, logger)
	srv.SetInfo(func() map[string]any {
		return map[string]any{
			"version":   deps.Version,
			"pid":       os.Getpid(),
			"startedAt": deps.Started.Format(time.RFC3339),
			"dataDir":   deps.DataDir,
			"socket":    deps.SocketPath,
			"modules":   moduleNames(),
		}
	})
	wireAll(srv, deps)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("aitermd mulai", "version", version, "pid", os.Getpid(), "socket", socketPath, "modules", moduleNames())

	if err := srv.Serve(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("daemon berhenti dengan galat", "err", err)
		return 1
	}
	logger.Info("aitermd berhenti")
	return 0
}
