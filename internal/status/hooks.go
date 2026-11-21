package status

import (
	"bytes"
	"encoding/json"
	"strings"
)

// maxHookStdin membatasi ukuran stdin hook yang diproses (1 MiB pertama).
const maxHookStdin = 1 << 20

// HookPayload adalah bentuk umum payload JSON yang dikirim hook provider lewat
// stdin. Semua field opsional. encoding/json mengabaikan field yang tidak
// dikenal secara default; itulah perilaku yang dimaksud agar parser toleran
// terhadap perubahan skema di sisi provider.
type HookPayload struct {
	SessionID        string `json:"session_id,omitempty"`
	HookEventName    string `json:"hook_event_name,omitempty"`
	Message          string `json:"message,omitempty"`
	Cwd              string `json:"cwd,omitempty"`
	TranscriptPath   string `json:"transcript_path,omitempty"`
	Source           string `json:"source,omitempty"`
	StopHookActive   bool   `json:"stop_hook_active,omitempty"`
	Prompt           string `json:"prompt,omitempty"`
	NotificationType string `json:"notification_type,omitempty"`
}

// ParseHookEvent mengurai nama event dan stdin hook. Tidak pernah panic dan
// payload tidak pernah nil: stdin kosong, bukan JSON, atau bukan objek
// menghasilkan map kosong. Field tak dikenal dipertahankan apa adanya. Bila
// name kosong, event diisi dari hook_event_name pada payload. Tanpa I/O.
func ParseHookEvent(name string, stdin []byte) (event string, payload map[string]any) {
	event = strings.TrimSpace(name)
	payload = map[string]any{}

	if len(stdin) > maxHookStdin {
		stdin = stdin[:maxHookStdin]
	}
	stdin = bytes.TrimSpace(stdin)
	if len(stdin) > 0 {
		var obj map[string]any
		if err := json.Unmarshal(stdin, &obj); err == nil && obj != nil {
			payload = obj
		}
	}

	if event == "" {
		if v, ok := payload["hook_event_name"].(string); ok {
			event = strings.TrimSpace(v)
		}
	}
	return event, payload
}

// ClassifyEvent memetakan event hook ke status sesi. Mengembalikan false untuk
// event yang tidak mengubah status atau tidak dikenal (mis. SubagentStop).
func ClassifyEvent(event string) (Status, bool) {
	switch event {
	case HookUserPromptSubmit, HookSessionStart:
		return StatusRunning, true
	case HookNotification:
		return StatusWaiting, true
	case HookStop:
		return StatusDone, true
	}
	return "", false
}
