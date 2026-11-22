// Paket thread: percakapan multi-agent (master + peer) yang disimpan di disk.
//
// Format penyimpanan: JSON camelCase; waktu UTC dalam RFC3339Nano.
//
// Konvensi path (worktree peer diimplementasikan tiket thread-02, bukan di sini):
//
//	DataDir/threads/<threadId>/thread.json
//	DataDir/threads/<threadId>/messages.jsonl
//	DataDir/threads/<threadId>/master/          (cwd netral sesi master)
//	DataDir/threads/<threadId>/<projectSlug>/   (worktree per project)
//
// Branch worktree: thread/<threadId>-<slug>.
package thread

import (
	"time"

	"github.com/NooBeeID/shikigami/internal/mention"
)

const (
	StateActive = "active"
	StateClosed = "closed"

	RoleMaster = "master"
	RolePeer   = "peer"

	KindMessage  = "message"
	KindSystem   = "system"
	KindActivity = "activity"

	FromUser   = "user"
	FromSystem = "system"
	ToUser     = "user"
	ToAll      = "all"

	MethodCreate        = "thread.create"
	MethodList          = "thread.list"
	MethodGet           = "thread.get"
	MethodMessages      = "thread.messages"
	MethodSend          = "thread.send"
	MethodOwnedSessions = "thread.ownedSessions"
	MethodClosePreview  = "thread.closePreview"
	MethodClose         = "thread.close"

	EventUpdated = "thread.updated"
	EventMessage = "thread.message"
)

// Thread adalah satu percakapan. Participants selalu [] (bukan null), ReadyToClose
// hanya ditulis tiket thread-agents/thread-close, ClosedAt nil selama aktif. Bentuk stabil; tiket lain hanya boleh menambah field opsional dengan omitempty.
type Thread struct {
	ID           string        `json:"id"`
	WorkspaceID  string        `json:"workspaceId"`
	Title        string        `json:"title"`
	Goal         string        `json:"goal"`
	State        string        `json:"state"`
	CreatedAt    time.Time     `json:"createdAt"`
	ClosedAt     *time.Time    `json:"closedAt,omitempty"`
	Participants []Participant `json:"participants"` // selalu [] (bukan null)
	ReadyToClose bool          `json:"readyToClose"`
	CloseSummary string        `json:"closeSummary,omitempty"` // ringkasan usul tutup dari master (tiket thread-close)
}

// Participant adalah sesi yang terlibat dalam thread (ProjectID kosong untuk master). Bentuk stabil; tiket lain hanya boleh menambah field opsional dengan omitempty.
type Participant struct {
	SessionID string `json:"sessionId"`
	ProjectID string `json:"projectId"` // kosong untuk master
	Role      string `json:"role"`
	Name      string `json:"name"`
	// Ditambah epic thread-ticket-05 (pemulihan).
	Dead               bool     `json:"dead,omitempty"`
	PreviousSessionIDs []string `json:"previousSessionIds,omitempty"`
}

// ThreadMessage adalah satu baris log pesan. From: "user", sessionId, atau "system";
// To: "user", sessionId, atau "all". Bentuk stabil; tiket lain hanya boleh menambah field opsional dengan omitempty.
type ThreadMessage struct {
	ID       string    `json:"id"`
	ThreadID string    `json:"threadId"`
	From     string    `json:"from"`
	To       string    `json:"to"`
	Text     string    `json:"text"`
	TS       time.Time `json:"ts"`
	Kind     string    `json:"kind"`
	// Field di bawah ditambah epic thread-ticket-03 (kartu aktivitas).
	Count int      `json:"count,omitempty"` // hanya kind=activity: jumlah aksi yang digabung
	Items []string `json:"items,omitempty"` // hanya kind=activity: ringkasan per aksi (maks 50 terbaru), tanpa awalan nama
}

type CreateParams struct {
	WorkspaceID string `json:"workspaceId"`
	Goal        string `json:"goal"`
	Title       string `json:"title,omitempty"`
}

type ListParams struct {
	WorkspaceID string `json:"workspaceId"`
}

type GetParams struct {
	ID string `json:"id"`
}

type MessagesParams struct {
	ID      string `json:"id"`
	SinceTS string `json:"sinceTs,omitempty"` // RFC3339/RFC3339Nano
}

type SendParams struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type OwnedSessionsResult struct {
	SessionIDs []string `json:"sessionIds"` // selalu []
}

// NewID membuat ID thread/pesan (ULID) lewat generator mention.
func NewID() string { return mention.NewMessageID() }
