package status

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"aiterm/internal/ipc"
	"aiterm/internal/provider"
)

const (
	notifyDebounce   = 2 * time.Second
	notifyMaxEntries = 256
)

// StartNotifier memasang subscriber notifikasi macOS pada engine.
// Parameter srv dipertahankan sesuai kontrak dan tidak dipakai. Fungsi cancel
// dari Subscribe tidak dipakai karena notifier hidup selama daemon.
// Notifikasi dapat dimatikan dengan AITERM_NOTIFY=0 (dibaca sekali di sini).
func StartNotifier(srv *ipc.Server, eng *Engine) {
	_ = srv
	if eng == nil || os.Getenv("AITERM_NOTIFY") == "0" {
		return
	}
	n := &notifier{eng: eng, last: map[string]time.Time{}}
	eng.Subscribe(n.handle)
}

type notifier struct {
	eng  *Engine
	mu   sync.Mutex
	last map[string]time.Time
}

func (n *notifier) handle(ev StatusEvent) {
	if ev.Status != StatusWaiting && ev.Status != StatusDone {
		return
	}
	if ev.Prev == ev.Status || ev.Source == SourceExit {
		return
	}

	now := time.Now()
	n.mu.Lock()
	if t, ok := n.last[ev.SessionID]; ok && now.Sub(t) < notifyDebounce {
		n.mu.Unlock()
		return
	}
	n.last[ev.SessionID] = now
	// Peta tidak dibersihkan lewat Forget (sengaja); ukurannya dibatasi dengan
	// satu kali sapuan entri lama saat menyisipkan.
	if len(n.last) > notifyMaxEntries {
		for id, t := range n.last {
			if now.Sub(t) > notifyDebounce {
				delete(n.last, id)
			}
		}
	}
	n.mu.Unlock()

	msg := "Selesai"
	if ev.Status == StatusWaiting {
		msg = "Menunggu input Anda"
	}
	go send(n.title(ev.SessionID), msg)
}

func (n *notifier) title(id string) string {
	if s, ok := n.eng.Lookup(id); ok {
		name := s.ProviderID
		if p, ok := provider.Get(s.ProviderID); ok {
			name = p.Name()
		}
		if s.Cwd != "" {
			return name + " · " + filepath.Base(s.Cwd)
		}
		return name
	}
	return "Sesi " + id[:min(8, len(id))]
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// send menampilkan satu notifikasi lewat osascript (argumen terpisah, tanpa
// shell). Kriteria "jendela app fokus" diabaikan karena daemon tidak tahu
// fokus jendela (keputusan tiket). Proses hanya di-spawn saat ada event.
func send(title, msg string) {
	if runtime.GOOS != "darwin" {
		return
	}
	script := `display notification "` + escapeAppleScript(msg) + `" with title "` + escapeAppleScript(title) + `"`
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		log.Printf("status: gagal mengirim notifikasi: %v", err)
	}
}
