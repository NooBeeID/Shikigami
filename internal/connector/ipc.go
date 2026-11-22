package connector

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const perConnectorTimeout = 10 * time.Second

var (
	dataDirMu sync.RWMutex
	dataDir   string
)

// SetDataDir menyetel DataDir aplikasi (dibaca connector fake).
func SetDataDir(dir string) {
	dataDirMu.Lock()
	dataDir = dir
	dataDirMu.Unlock()
}

func getDataDir() string {
	dataDirMu.RLock()
	defer dataDirMu.RUnlock()
	return dataDir
}

type info struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
}

// resolveStatus menghitung status connector; galat dipetakan ke Status.
func resolveStatus(ctx context.Context, c Connector) (st Status) {
	defer func() {
		if r := recover(); r != nil {
			st = Status{State: StateNotLoggedIn, Message: "status connector gagal dihitung"}
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, perConnectorTimeout)
	defer cancel()
	s, err := c.Status(ctx)
	if err != nil {
		if IsCLIMissing(err) {
			return Status{State: StateCLIMissing}
		}
		return Status{State: StateNotLoggedIn, Message: err.Error()}
	}
	return s
}

func normalizeGuide(g Guide) Guide {
	if g.Steps == nil {
		g.Steps = []string{}
	}
	if g.Commands == nil {
		g.Commands = []string{}
	}
	return g
}

func parseID(params json.RawMessage) (string, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.ID == "" {
		return "", ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh {\"id\": string}")
	}
	return p.ID, nil
}

func lookup(id string) (Connector, error) {
	c, ok := Get(id)
	if !ok {
		return nil, ipc.NewError(ipc.ErrNotFound, "connector tidak dikenal: "+id)
	}
	return c, nil
}

// RegisterIPC mendaftarkan method connector.list|status|guide.
func RegisterIPC(srv *ipc.Server) {
	srv.Register("connector.list", func(ctx context.Context, _ json.RawMessage) (any, error) {
		cs := List()
		out := make([]info, len(cs))
		var wg sync.WaitGroup
		for i, c := range cs {
			wg.Add(1)
			go func(i int, c Connector) {
				defer wg.Done()
				out[i] = info{ID: c.ID(), Name: c.Name(), Status: resolveStatus(ctx, c)}
			}(i, c)
		}
		wg.Wait()
		return out, nil
	})
	srv.Register("connector.status", func(ctx context.Context, params json.RawMessage) (any, error) {
		id, err := parseID(params)
		if err != nil {
			return nil, err
		}
		c, err := lookup(id)
		if err != nil {
			return nil, err
		}
		return resolveStatus(ctx, c), nil
	})
	srv.Register("connector.guide", func(_ context.Context, params json.RawMessage) (any, error) {
		id, err := parseID(params)
		if err != nil {
			return nil, err
		}
		c, err := lookup(id)
		if err != nil {
			return nil, err
		}
		return normalizeGuide(c.ConnectGuide()), nil
	})
}
