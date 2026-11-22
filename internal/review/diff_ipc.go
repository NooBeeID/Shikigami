package review

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"

	"github.com/NooBeeID/shikigami/internal/fsx"
	"github.com/NooBeeID/shikigami/internal/ipc"
)

// IPCMethodDiff adalah nama metode IPC untuk diff satu file.
const IPCMethodDiff = "review.diff"

// diffResponse adalah hasil IPC review.diff (tanpa additions/deletions).
type diffResponse struct {
	Hunks       []Hunk `json:"hunks"`
	CurrentHash string `json:"currentHash"`
	Binary      bool   `json:"binary"`
	TooLarge    bool   `json:"tooLarge"`
}

// RegisterDiffIPC mendaftarkan metode IPC review.diff.
func RegisterDiffIPC(srv *ipc.Server, store *Store, supportedFn func(sessionID string) bool) {
	srv.Register(IPCMethodDiff, func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			SessionID string `json:"sessionId"`
			Path      string `json:"path"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, ipc.NewError(ipc.ErrInvalidParams, "params tidak valid: butuh {\"sessionId\": string, \"path\": string}")
		}
		id := strings.TrimSpace(p.SessionID)
		path := strings.TrimSpace(p.Path)
		if id == "" {
			return nil, ipc.NewError(ipc.ErrInvalid, "sessionId wajib diisi")
		}
		if path == "" {
			return nil, ipc.NewError(ipc.ErrInvalid, "path wajib diisi")
		}
		if supportedFn == nil || !supportedFn(id) {
			return diffResponse{Hunks: []Hunk{}}, nil
		}
		res, err := DiffForFile(store, id, path)
		if err != nil {
			return nil, ipc.NewError(ipc.ErrInvalid, "gagal menghitung diff: "+err.Error())
		}
		hunks := res.Hunks
		if hunks == nil {
			hunks = []Hunk{}
		}
		return diffResponse{Hunks: hunks, CurrentHash: res.CurrentHash, Binary: res.Binary, TooLarge: res.TooLarge}, nil
	})
}

// diffCacheMax membatasi jumlah entri cache; terlampaui -> cache dikosongkan.
const diffCacheMax = 4096

// Kunci kontrak adalah (sessionId, path, currentHash); baselineSha256 ditambahkan
// sebagai pengaman bila baseline kelak berubah (superset yang aman).
type diffCacheKey struct{ session, path, hash, baseline string }

type diffCacheVal struct {
	add, del         int
	binary, tooLarge bool
}

var diffCache = struct {
	mu sync.Mutex
	m  map[diffCacheKey]diffCacheVal
}{m: map[diffCacheKey]diffCacheVal{}}

// FillDiffStats mengisi Additions/Deletions/Binary/TooLarge pada files secara
// in place. Galat per file diabaikan (file itu tetap 0/0).
func FillDiffStats(st *Store, sessionID string, files []FileEntry) {
	if st == nil || len(files) == 0 || !storeValidID(sessionID) {
		return
	}
	cwd := st.Cwd(sessionID)
	if cwd == "" {
		return
	}
	touched := map[string]TouchedFile{}
	for _, t := range st.Touched(sessionID) {
		touched[t.Path] = t
	}
	for i := range files {
		entry, ok := touched[files[i].Path]
		if !ok {
			continue
		}
		abs, err := fsx.ResolveUnder(cwd, files[i].Path)
		if err != nil {
			continue
		}
		hash := ""
		if _, err := os.Stat(abs); err == nil {
			if hash, err = storeHashFile(abs); err != nil {
				continue
			}
		} else if !os.IsNotExist(err) {
			continue
		}
		key := diffCacheKey{sessionID, files[i].Path, hash, entry.BaselineSha256}
		diffCache.mu.Lock()
		val, hit := diffCache.m[key]
		diffCache.mu.Unlock()
		if !hit {
			res, err := DiffForFile(st, sessionID, files[i].Path)
			if err != nil {
				continue
			}
			val = diffCacheVal{res.Additions, res.Deletions, res.Binary, res.TooLarge}
			diffCache.mu.Lock()
			if len(diffCache.m) >= diffCacheMax {
				diffCache.m = map[diffCacheKey]diffCacheVal{}
			}
			diffCache.m[key] = val
			diffCache.mu.Unlock()
		}
		files[i].Additions = val.add
		files[i].Deletions = val.del
		files[i].Binary = files[i].Binary || val.binary
		files[i].TooLarge = files[i].TooLarge || val.tooLarge
	}
}
