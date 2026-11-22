package review

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

// RegisterIPC mendaftarkan metode IPC review.files.
func RegisterIPC(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool) {
	srv.Register(IPCMethodFiles, func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh {\"sessionId\": string}")
		}
		id := strings.TrimSpace(p.SessionID)
		if id == "" {
			return nil, ipc.NewError(ipc.ErrInvalid, "sessionId wajib diisi")
		}
		if supportedFn == nil || !supportedFn(id) {
			return FilesResult{Supported: false, Files: []FileEntry{}}, nil
		}
		// Handler ini tidak punya akses engine untuk fallback Lookup. Tanpa cwd di
		// index berarti belum ada file yang tersentuh, jadi daftar kosong sudah benar.
		cwd := store.Cwd(id)
		if cwd == "" {
			return FilesResult{Supported: true, Files: []FileEntry{}}, nil
		}
		files, err := store.Files(id, cwd)
		if err != nil {
			return nil, ipc.NewError(ipc.ErrInvalid, "gagal membaca daftar review: "+err.Error())
		}
		if files == nil {
			files = []FileEntry{}
		}
		FillDiffStats(store, id, files)
		FillExternal(store, id, files)
		return FilesResult{Supported: true, Files: files}, nil
	})
}
