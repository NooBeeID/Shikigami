package session

import (
	"crypto/rand"
	"errors"
	"sync"
	"time"
)

// Status adalah status sesi.
type Status string

const (
	StatusRunning Status = "running"
	StatusExited  Status = "exited"
)

// Session adalah deskripsi sebuah sesi PTY.
type Session struct {
	ID          string    `json:"id"`          // ULID
	WorkspaceID string    `json:"workspaceId"` // string opak, boleh kosong
	ProjectID   string    `json:"projectId"`   // string opak, boleh kosong
	Cwd         string    `json:"cwd"`
	ProviderID  string    `json:"providerId"` // default "shell"
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`          // RFC3339 di JSON
	ExitCode    *int      `json:"exitCode,omitempty"` // terisi saat exited
}

// SessionSpec adalah parameter pembuatan sesi.
type SessionSpec struct {
	WorkspaceID, ProjectID, Cwd, ProviderID string
	Cols, Rows                              uint16
	Env                                     []string
}

// AttachResult adalah hasil Attach: sesi, scrollback, dan offset kumulatif.
type AttachResult struct {
	Session    Session
	Scrollback []byte
	Offset     uint64
}

var (
	ErrNotFound   = errors.New("sesi tidak ditemukan")
	ErrInvalid    = errors.New("masukan sesi tidak valid")
	ErrNotRunning = errors.New("sesi tidak berjalan")
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var (
	idMu       sync.Mutex
	idLastMs   int64
	idLastRand [10]byte
)

// newID membuat ULID 26 karakter yang monotonik dalam proses.
func newID() string {
	idMu.Lock()
	ms := time.Now().UnixMilli()
	if ms > idLastMs {
		idLastMs = ms
		if _, err := rand.Read(idLastRand[:]); err != nil {
			idMu.Unlock()
			panic("session: gagal membaca crypto/rand untuk ULID: " + err.Error())
		}
	} else {
		// ms sama atau jam mundur: pakai idLastMs dan increment bagian acak.
		for i := len(idLastRand) - 1; i >= 0; i-- {
			idLastRand[i]++
			if idLastRand[i] != 0 {
				break
			}
		}
	}
	var b [16]byte
	t := uint64(idLastMs)
	for i := 5; i >= 0; i-- {
		b[i] = byte(t)
		t >>= 8
	}
	copy(b[6:], idLastRand[:])
	idMu.Unlock()

	// 130 bit (2 bit nol di depan), 5 bit per karakter dari MSB.
	out := make([]byte, 26)
	for i := 0; i < 26; i++ {
		// posisi bit awal karakter i dalam aliran 130 bit; bit 0-1 adalah nol.
		start := i*5 - 2
		var v byte
		for j := 0; j < 5; j++ {
			pos := start + j
			v <<= 1
			if pos >= 0 {
				v |= (b[pos/8] >> (7 - uint(pos%8))) & 1
			}
		}
		out[i] = crockford[v]
	}
	return string(out)
}
