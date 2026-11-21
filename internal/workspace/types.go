package workspace

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Project adalah satu direktori project di dalam workspace.
type Project struct {
	ID   string `json:"id"`   // ULID
	Path string `json:"path"` // absolut, sudah dibersihkan (filepath.Clean)
	Name string `json:"name"` // default: filepath.Base(Path)
}

// WorkspaceConfig adalah override konfigurasi per workspace.
type WorkspaceConfig struct {
	GrimoireURL string `json:"grimoireUrl,omitempty"`
	GrimoireKey string `json:"grimoireKey,omitempty"`
}

// Workspace berisi nama, daftar project, dan konfigurasi.
type Workspace struct {
	ID       string          `json:"id"` // ULID
	Name     string          `json:"name"`
	Projects []Project       `json:"projects"` // selalu [] (bukan null)
	Config   WorkspaceConfig `json:"config"`

	Groups        []Group `json:"groups"` // selalu >= 1, tidak pernah null
	ActiveGroupID string  `json:"activeGroupId"`

	Membership map[string]string `json:"membership"` // sessionId -> groupId; non-nil saat keluar
}

// Group adalah satu grup layout di dalam workspace.
type Group struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Order  int             `json:"order"`
	Main   bool            `json:"main,omitempty"` // grup bawaan "Utama"; tepat satu per workspace
	Layout json.RawMessage `json:"layout"`         // dokumen opaque; null bila kosong
}

// SidebarState adalah keadaan satu sidebar.
type SidebarState struct {
	Visible   bool   `json:"visible"`
	Width     int    `json:"width"`
	ActiveTab string `json:"activeTab,omitempty"` // hanya sidebar kanan
}

// UIState adalah keadaan UI global yang dipersist.
type UIState struct {
	LeftSidebar       SidebarState `json:"leftSidebar"`
	RightSidebar      SidebarState `json:"rightSidebar"`
	ActiveWorkspaceID string       `json:"activeWorkspaceId"`
}

// UIPatch adalah patch parsial untuk UIState.
type UIPatch struct {
	LeftSidebar       *SidebarState `json:"leftSidebar,omitempty"`
	RightSidebar      *SidebarState `json:"rightSidebar,omitempty"`
	ActiveWorkspaceID *string       `json:"activeWorkspaceId,omitempty"`
}

func defaultLeftSidebar() SidebarState { return SidebarState{Visible: true, Width: 260} }
func defaultRightSidebar() SidebarState {
	return SidebarState{Visible: true, Width: 280, ActiveTab: "files"}
}

func defaultUI() UIState {
	return UIState{LeftSidebar: defaultLeftSidebar(), RightSidebar: defaultRightSidebar()}
}

func newDefaultGroup() Group {
	return Group{ID: newID(), Name: "Utama", Order: 0, Main: true}
}

// GlobalConfig adalah konfigurasi global.
type GlobalConfig struct {
	GrimoireURL string `json:"grimoireUrl,omitempty"`
	GrimoireKey string `json:"grimoireKey,omitempty"`
}

// Error sentinel yang dipetakan ke kode IPC di ipc.go.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid argument")
	ErrConflict = errors.New("conflict")
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newID membuat ULID 26 karakter: 48-bit timestamp ms + 80-bit acak.
func newID() string {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	if _, err := rand.Read(b[6:]); err != nil {
		panic(fmt.Sprintf("workspace: crypto/rand gagal membuat ID: %v", err))
	}
	// 128 bit -> 26 karakter x 5 bit (130 bit, 2 bit teratas nol).
	var out [26]byte
	var hi, lo uint64
	for i := 0; i < 8; i++ {
		hi = hi<<8 | uint64(b[i])
		lo = lo<<8 | uint64(b[8+i])
	}
	for i := 25; i >= 0; i-- {
		out[i] = crockford[lo&31]
		lo = lo>>5 | hi<<59
		hi >>= 5
	}
	return string(out[:])
}
