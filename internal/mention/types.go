// Paket mention menyediakan mention antar-sesi: tipe bersama, penamaan sesi,
// dan hook start server MCP. Berkas ini adalah kontrak Foundation.
package mention

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/session"
)

// State adalah state sebuah pesan mention.
type State string

const (
	StateQueued          State = "queued"
	StateAwaitingConfirm State = "awaiting_confirm"
	StateDelivered       State = "delivered"
	StateFailed          State = "failed"
)

// Nilai Message.Source.
const (
	SourceAgent = "agent"
	SourceUser  = "user"
)

// FromUser adalah nilai Message.From untuk mention dari user.
const FromUser = "user"

// Mode adalah mode pengiriman mention dari agent.
type Mode string

const (
	ModeAuto         Mode = "auto"
	ModeConfirmAgent Mode = "confirm_agent" // default
	ModeUserOnly     Mode = "user_only"
)

// Message adalah satu pesan mention.
type Message struct {
	ID        string    `json:"id"`              // ULID
	From      string    `json:"from"`            // "user" atau sessionID pengirim
	FromName  string    `json:"fromName"`        // nama mention pengirim, "user" untuk user
	To        string    `json:"to"`              // sessionID target
	Text      string    `json:"text"`            // isi pesan
	CreatedAt time.Time `json:"createdAt"`       // RFC3339
	State     State     `json:"state"`           // state pesan
	Source    string    `json:"source"`          // "agent" | "user"
	Error     string    `json:"error,omitempty"` // terisi bila state failed
}

// NameInfo adalah informasi nama mention sebuah sesi.
type NameInfo struct {
	SessionID   string `json:"sessionId"`
	Name        string `json:"name"`
	ProjectName string `json:"projectName"`
	ProviderID  string `json:"providerId"`
	Status      string `json:"status"` // running|waiting|done|idle
}

// Error sentinel. Pemetaan ke kode IPC dilakukan di ipc.go (fitur
// message-queue): ErrNotFound -> ipc.ErrNotFound; ErrInvalid,
// ErrCrossWorkspace, ErrUserOnly -> ipc.ErrInvalid; ErrConflict ->
// ipc.ErrConflict.
var (
	ErrNotFound       = errors.New("sesi tidak ditemukan")
	ErrInvalid        = errors.New("masukan tidak valid")
	ErrConflict       = errors.New("antrean penuh")
	ErrCrossWorkspace = errors.New("bukan workspace yang sama")
	ErrUserOnly       = errors.New("mode user_only: pesan dari agent ditolak")
)

// SessionSource dipenuhi langsung oleh *session.Manager.
type SessionSource interface {
	List() []session.Session
	Get(id string) (*session.Session, error)
	Attach(id string) (session.AttachResult, error)
	Write(id string, data []byte) error
}

var _ SessionSource = (*session.Manager)(nil)

// Directory memberi nama project; diimplementasikan di wire_mention.go.
type Directory interface {
	ProjectName(workspaceID, projectID string) string
}

// StatusFunc mengembalikan status sesi (running|waiting|done|idle).
type StatusFunc func(sessionID string) string

// Enqueuer dipenuhi oleh *Queue. from == "" berarti user (source harus
// SourceUser); selain itu from = sessionID pengirim (source SourceAgent).
// to = sessionID target yang sudah di-resolve.
type Enqueuer interface {
	Enqueue(ctx context.Context, from, to, text, source string) (Message, error)
}

// QueueReader dipenuhi *Queue; dipakai tools.go untuk menghitung position.
type QueueReader interface {
	// Position bernilai 1-based di antrean pending target; 0 bila bukan pending.
	Position(messageID string) int
}

// Runtime dibuat oleh wire_mention.go dan dikonsumsi mcp.go/install.go.
type Runtime struct {
	DataDir       string
	Logger        *slog.Logger
	Sessions      SessionSource
	Namer         *Namer
	Enqueuer      Enqueuer
	Reader        QueueReader
	OnSessionExit func(fn func(sessionID string)) // boleh dipanggil beberapa kali
}

// ---- ULID ----

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewMessageID membuat ULID 26 karakter (48 bit ms + 80 bit acak); aman konkuren.
func NewMessageID() string {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	for i := 0; i < 6; i++ {
		b[i] = byte(ms >> (8 * (5 - i)))
	}
	if _, err := rand.Read(b[6:]); err != nil {
		// crypto/rand tidak pernah gagal pada platform yang didukung.
		for i := 6; i < 16; i++ {
			b[i] = byte(time.Now().UnixNano() >> (i % 8))
		}
	}
	// 128 bit dikodekan sebagai 26 x 5 bit (2 bit awal nol).
	out := make([]byte, 26)
	var acc uint32
	bits := 0
	idx := 25
	for i := 15; i >= 0; i-- {
		acc |= uint32(b[i]) << bits
		bits += 8
		for bits >= 5 && idx >= 0 {
			out[idx] = crockford[acc&31]
			acc >>= 5
			bits -= 5
			idx--
		}
	}
	for idx >= 0 {
		out[idx] = crockford[acc&31]
		acc >>= 5
		idx--
	}
	return string(out)
}

// ---- Namer ----

// Namer menghitung nama mention sesi.
type Namer struct {
	src      SessionSource
	dir      Directory
	statusOf StatusFunc
}

type namedSession struct {
	sess session.Session
	name string
}

// NewNamer membuat Namer. dir atau statusOf boleh nil.
func NewNamer(src SessionSource, dir Directory, statusOf StatusFunc) *Namer {
	return &Namer{src: src, dir: dir, statusOf: statusOf}
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// baseName membentuk nama dasar dari provider dan project.
func baseName(providerID, projectName string) string {
	if providerID == "" {
		providerID = "shell"
	}
	clean := func(s string) string {
		return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(s), "-"), "-")
	}
	p := clean(providerID)
	if p == "" {
		p = "shell"
	}
	proj := clean(projectName)
	if proj == "" {
		proj = "noproject"
	}
	return p + "-" + proj
}

func (n *Namer) projectName(s session.Session) string {
	if n.dir == nil {
		return ""
	}
	return n.dir.ProjectName(s.WorkspaceID, s.ProjectID)
}

// snapshot mengembalikan sesi running bernama. workspaceID kosong berarti
// lintas workspace (penamaan tetap dihitung per workspace).
func (n *Namer) snapshot(workspaceID string) []namedSession {
	all := n.src.List()
	var list []session.Session
	for _, s := range all {
		if s.Status != session.StatusRunning {
			continue
		}
		if workspaceID != "" && s.WorkspaceID != workspaceID {
			continue
		}
		list = append(list, s)
	}
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		}
		return list[i].ID < list[j].ID
	})
	counts := map[string]int{} // kunci: workspace + "\x00" + nama dasar
	out := make([]namedSession, 0, len(list))
	for _, s := range list {
		base := baseName(s.ProviderID, n.projectName(s))
		key := s.WorkspaceID + "\x00" + base
		counts[key]++
		name := base
		if c := counts[key]; c > 1 {
			name = base + "-" + itoa(c)
		}
		out = append(out, namedSession{sess: s, name: name})
	}
	return out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return string(b[p:])
}

// NameOf mengembalikan nama mention sesi, "" bila sesi tak ada atau exited.
func (n *Namer) NameOf(sessionID string) string {
	s, err := n.src.Get(sessionID)
	if err != nil || s == nil || s.Status != session.StatusRunning {
		return ""
	}
	for _, ns := range n.snapshot("") {
		if ns.sess.ID == sessionID {
			return ns.name
		}
	}
	return ""
}

func normName(name string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "@")
}

// ResolveName memetakan nama mention ke sessionID dalam workspace.
func (n *Namer) ResolveName(workspaceID, name string) (string, error) {
	want := normName(name)
	if want == "" {
		return "", ErrNotFound
	}
	for _, ns := range n.snapshot(workspaceID) {
		if ns.name == want {
			return ns.sess.ID, nil
		}
	}
	return "", ErrNotFound
}

// Resolve memetakan `to` (id sesi atau nama) ke sessionID dalam workspace.
func (n *Namer) Resolve(workspaceID, to string) (string, error) {
	if strings.TrimSpace(to) == "" {
		return "", ErrInvalid
	}
	if s, err := n.src.Get(to); err == nil && s != nil {
		if s.Status != session.StatusRunning {
			return "", ErrNotFound
		}
		if s.WorkspaceID != workspaceID {
			return "", ErrCrossWorkspace
		}
		return s.ID, nil
	}
	if id, err := n.ResolveName(workspaceID, to); err == nil {
		return id, nil
	}
	want := normName(to)
	for _, ns := range n.snapshot("") {
		if ns.name == want && ns.sess.WorkspaceID != workspaceID {
			return "", ErrCrossWorkspace
		}
	}
	return "", ErrNotFound
}

// List mengembalikan sesi running di workspace (urut CreatedAt), tidak pernah nil.
func (n *Namer) List(workspaceID string) []NameInfo {
	snap := n.snapshot(workspaceID)
	out := make([]NameInfo, 0, len(snap))
	for _, ns := range snap {
		st := ""
		if n.statusOf != nil {
			st = n.statusOf(ns.sess.ID)
		}
		if st == "" {
			st = "running"
		}
		out = append(out, NameInfo{
			SessionID:   ns.sess.ID,
			Name:        ns.name,
			ProjectName: n.projectName(ns.sess),
			ProviderID:  ns.sess.ProviderID,
			Status:      st,
		})
	}
	return out
}

// ---- Start / URL ----

// startMCP diisi init() di mcp.go (fitur mcp-server).
var startMCP func(ctx context.Context, rt *Runtime) (url string, err error)

var (
	urlMu  sync.RWMutex
	mcpURL string
)

// Start menjalankan server MCP bila tersedia; bila belum ada mengembalikan ("", nil).
func Start(ctx context.Context, rt *Runtime) (string, error) {
	if rt == nil {
		return "", ErrInvalid
	}
	if startMCP == nil {
		return "", nil
	}
	u, err := startMCP(ctx, rt)
	if err != nil {
		return "", err
	}
	setURL(u)
	return u, nil
}

// URL mengembalikan alamat MCP yang aktif ("" bila belum jalan); thread-safe.
func URL() string {
	urlMu.RLock()
	defer urlMu.RUnlock()
	return mcpURL
}

func setURL(u string) {
	urlMu.Lock()
	mcpURL = u
	urlMu.Unlock()
}
