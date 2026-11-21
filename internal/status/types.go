// Paket status menyediakan status sesi (running, waiting, done, idle)
// berbasis event hooks provider dengan cadangan deteksi output-diam.
package status

import "time"

type Status string

const (
	StatusRunning Status = "running"
	StatusWaiting Status = "waiting"
	StatusDone    Status = "done"
	StatusIdle    Status = "idle"
)

const (
	SourceHook   = "hook"
	SourceOutput = "output"
	SourceExit   = "exit"
)

// Nama event hook yang dikenal status.hook.
const (
	HookUserPromptSubmit = "UserPromptSubmit"
	HookSessionStart     = "SessionStart"
	HookNotification     = "Notification"
	HookStop             = "Stop"
	HookSubagentStop     = "SubagentStop"
)

// Jeda output-diam (konstanta; dapat diubah di sini).
const (
	SilentDelay          = 3 * time.Second  // provider tanpa hooks
	SilentDelayWithHooks = 30 * time.Second // provider berhooks, hanya cadangan sebelum hook pertama
)

type StatusEvent struct {
	SessionID string `json:"sessionId"`
	Status    Status `json:"status"`
	Prev      Status `json:"prev"`
	Source    string `json:"source"`           // SourceHook | SourceOutput | SourceExit
	Reason    string `json:"reason,omitempty"` // mis. "Stop", "Notification", "silent", "exit"
	At        int64  `json:"at"`               // unix milidetik
}

// SessionState adalah bentuk hasil status.get / elemen status.list.
type SessionState struct {
	SessionID string `json:"sessionId"`
	Status    Status `json:"status"`
	Since     int64  `json:"since"` // unix milidetik
}

// LaunchInfo mencatat peluncuran sesi yang menunggu ikatan ke ID sesi asli (lihat T1).
type LaunchInfo struct {
	Token       string
	WorkspaceID string
	ProjectID   string
	Cwd         string
	ProviderID  string
	At          time.Time
}
