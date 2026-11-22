package thread

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

var closeToolOnce sync.Once

// RegisterClose memasang IPC thread.closePreview/thread.close, tool thread_propose_close,
// dan perluasan prompt master. Panggil setelah InstallAgents.
func RegisterClose(m *Manager) {
	m.RegisterMethod(MethodClosePreview, func(raw json.RawMessage) (any, error) {
		var p CloseParams
		if err := ipcDecode(raw, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.ID) == "" {
			return nil, fmt.Errorf("%w: id wajib diisi", ErrInvalid)
		}
		return m.ClosePreview(p.ID)
	})
	m.RegisterMethod(MethodClose, func(raw json.RawMessage) (any, error) {
		var p CloseParams
		if err := ipcDecode(raw, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.ID) == "" {
			return nil, fmt.Errorf("%w: id wajib diisi", ErrInvalid)
		}
		return m.Close(p.ID)
	})

	closeToolOnce.Do(func() {
		RegisterTool(ToolDef{
			Name:        "thread_propose_close",
			Description: "Usulkan penutupan thread karena goal tercapai. Hanya mengusulkan: user yang menutup lewat tombol Close. Beri ringkasan hasil.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"summary": map[string]any{"type": "string", "description": "Ringkasan hasil kerja thread"},
				},
				"required": []string{"summary"},
			},
			MasterOnly: true,
			Handler: func(ctx context.Context, c Caller, args json.RawMessage) (any, error) {
				if !c.Master {
					return nil, fmt.Errorf("hanya master yang dapat mengusulkan penutupan")
				}
				var in struct {
					Summary string `json:"summary"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &in); err != nil {
						return nil, fmt.Errorf("argumen tidak valid: %w", err)
					}
				}
				if _, err := m.ProposeClose(c.ThreadID, c.SessionID, in.Summary); err != nil {
					return nil, err
				}
				return "usul penutupan dicatat; menunggu user menekan Close", nil
			},
		})

		prev := ToolsSection
		if prev == nil {
			prev = func() string { return "" }
		}
		ToolsSection = func() string {
			return prev() + "\n- thread_propose_close(summary): panggil bila goal thread sudah tercapai, dengan ringkasan hasil kerja. Ini hanya MENGUSULKAN penutupan; jangan menutup sendiri atau berasumsi thread sudah tertutup. User yang menekan Close."
		}
	})
}
