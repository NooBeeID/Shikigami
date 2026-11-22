package runs

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sync"
)

// Usage adalah total token satu agent. Input = input_tokens + cache_creation
// (token baru yang diproses), CacheRead = token yang dibaca dari cache prompt.
type Usage struct {
	Input     int64 `json:"input"`
	CacheRead int64 `json:"cacheRead"`
	Output    int64 `json:"output"`
}

func (u Usage) add(o Usage) Usage {
	return Usage{u.Input + o.Input, u.CacheRead + o.CacheRead, u.Output + o.Output}
}

func (u Usage) zero() bool { return u == Usage{} }

// usageState adalah hasil baca inkremental satu transkrip. Transkrip Claude
// menulis pesan yang sama berkali-kali (satu baris per blok konten) dengan usage
// identik, jadi dedupe per message.id (yang terakhir menang).
type usageState struct {
	offset int64
	byMsg  map[string]Usage
	model  string
}

var (
	usageMu    sync.Mutex
	usageCache = map[string]*usageState{}
)

const maxUsageCache = 128

// readUsage mengembalikan total usage dan model terakhir dari transkrip agent.
// Hanya byte baru sejak pembacaan terakhir yang diproses. Fail-soft: berkas tak
// ada/rusak menghasilkan (nil, "").
func readUsage(path string) (u *Usage, model string) {
	defer func() {
		if recover() != nil {
			u, model = nil, ""
		}
	}()
	f, err := os.Open(path)
	if err != nil {
		return nil, ""
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return nil, ""
	}

	usageMu.Lock()
	defer usageMu.Unlock()
	s := usageCache[path]
	if s == nil || st.Size() < s.offset {
		if len(usageCache) >= maxUsageCache {
			usageCache = map[string]*usageState{}
		}
		s = &usageState{byMsg: map[string]Usage{}}
		usageCache[path] = s
	}
	if st.Size() > s.offset {
		if _, err := f.Seek(s.offset, io.SeekStart); err != nil {
			return nil, ""
		}
		data, err := io.ReadAll(io.LimitReader(f, st.Size()-s.offset))
		if err != nil {
			return nil, ""
		}
		// Proses hanya baris lengkap; sisa (baris yang sedang ditulis) dibaca lagi nanti.
		end := bytes.LastIndexByte(data, '\n') + 1
		for _, ln := range bytes.Split(data[:end], []byte("\n")) {
			s.ingest(ln)
		}
		s.offset += int64(end)
	}

	var total Usage
	for _, v := range s.byMsg {
		total = total.add(v)
	}
	if total.zero() {
		return nil, s.model
	}
	return &total, s.model
}

func (s *usageState) ingest(ln []byte) {
	ln = bytes.TrimSpace(ln)
	if len(ln) == 0 {
		return
	}
	var e struct {
		Type    string `json:"type"`
		Message struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage *struct {
				Input         int64 `json:"input_tokens"`
				CacheCreation int64 `json:"cache_creation_input_tokens"`
				CacheRead     int64 `json:"cache_read_input_tokens"`
				Output        int64 `json:"output_tokens"`
			} `json:"usage"`
		} `json:"message"`
	}
	if json.Unmarshal(ln, &e) != nil || e.Type != "assistant" {
		return
	}
	m := e.Message
	if m.Model != "" && m.Model != "<synthetic>" {
		s.model = m.Model
	}
	if m.Usage == nil || m.ID == "" {
		return
	}
	s.byMsg[m.ID] = Usage{
		Input:     m.Usage.Input + m.Usage.CacheCreation,
		CacheRead: m.Usage.CacheRead,
		Output:    m.Usage.Output,
	}
}
