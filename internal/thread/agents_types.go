// Berkas ini adalah kontrak epic thread-ticket-02-thread-agents dan hanya boleh ditambah.
package thread

import (
	"context"
	"errors"
)

// Router dipenuhi oleh router.go (thread-spawn-router); dipakai handler tool (thread-mcp-server).
type Router interface {
	// Route mencatat pesan ke log thread lalu mengirimnya. from = sessionId pengirim ("user" untuk user).
	// to = nama/sessionId peserta, "master", "user" (hanya dicatat) atau "all" (semua peserta selain pengirim).
	Route(ctx context.Context, threadID, from, to, text string) (ThreadMessage, error)
}

var (
	ErrPaused          = errors.New("thread dijeda: pesan beruntun antar agent mencapai batas; tunggu pesan dari user atau master")
	ErrTargetUnknown   = errors.New("target bukan peserta thread ini")
	ErrTargetAmbiguous = errors.New("target ambigu")
	ErrNotParticipant  = errors.New("pemanggil bukan peserta thread ini")
)

// ParticipantView adalah bentuk keluaran tool thread_participants dan dipakai ulang tiket 03/05.
type ParticipantView struct {
	Name      string `json:"name"`
	SessionID string `json:"sessionId"`
	Role      string `json:"role"`
	Project   string `json:"project"` // nama project; kosong untuk master
	Status    string `json:"status"`  // "kerja" | "idle"
	Self      bool   `json:"self"`
}
