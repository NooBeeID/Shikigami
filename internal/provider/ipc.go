package provider

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

// Info menggambarkan satu provider beserta ketersediaan binary-nya.
type Info struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Capabilities Capabilities `json:"capabilities"`
	Available    bool         `json:"available"`
	Path         string       `json:"path,omitempty"`
	Error        string       `json:"error,omitempty"`
}

func infoFor(p Provider) Info {
	info := Info{ID: p.ID(), Name: p.Name(), Capabilities: p.Capabilities()}
	path, err := p.Available()
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.Available = true
	info.Path = path
	return info
}

// RegisterIPC mendaftarkan method "provider.list".
func RegisterIPC(srv *ipc.Server) {
	srv.Register("provider.list", func(ctx context.Context, params json.RawMessage) (any, error) {
		ps := List()
		out := make([]Info, 0, len(ps)) // tidak pernah nil -> JSON []
		for _, p := range ps {
			out = append(out, infoFor(p))
		}
		return out, nil
	})
}

// Validate memeriksa providerID untuk guard session.create. Kegagalan selalu
// berupa *ipc.Error non-nil; sukses berupa nil murni.
func Validate(providerID string) error {
	var p Provider
	if providerID == "" {
		p = Default()
	} else {
		got, ok := Get(providerID)
		if !ok {
			ps := List()
			ids := make([]string, 0, len(ps))
			for _, q := range ps {
				ids = append(ids, q.ID())
			}
			return ipc.Errorf(ipc.ErrInvalid, "providerId %q tidak dikenal; yang tersedia: %s", providerID, strings.Join(ids, ", "))
		}
		p = got
	}
	if _, err := p.Available(); err != nil {
		return ipc.NewError(ipc.ErrBinary, err.Error())
	}
	return nil
}
