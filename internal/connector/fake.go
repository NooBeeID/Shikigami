package connector

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/NooBeeID/shikigami/internal/paths"
)

// fakeConnector adalah connector contoh untuk menguji tampilan.
type fakeConnector struct{}

func init() { Register(fakeConnector{}) }

func (fakeConnector) ID() string   { return "fake" }
func (fakeConnector) Name() string { return "Contoh (fake)" }

// fakeStateDir mengembalikan folder penanda state; kosong bila DataDir tak diketahui.
func fakeStateDir() string {
	dir := getDataDir()
	if dir == "" {
		dir, _ = paths.DataDir()
	}
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "connector-fake")
}

func validState(s string) (State, bool) {
	switch State(s) {
	case StateCLIMissing, StateNotLoggedIn, StateConnected:
		return State(s), true
	}
	return "", false
}

func (fakeConnector) Status(ctx context.Context) (Status, error) {
	state := StateNotLoggedIn
	if s, ok := validState(strings.TrimSpace(os.Getenv("SHIKIGAMI_FAKE_CONNECTOR"))); ok {
		state = s
	} else if dir := fakeStateDir(); dir != "" {
		if b, err := os.ReadFile(filepath.Join(dir, "state")); err == nil {
			if s, ok := validState(strings.TrimSpace(string(b))); ok {
				state = s
			}
		}
	}
	switch state {
	case StateConnected:
		return Status{State: StateConnected, User: "fake-user"}, nil
	case StateCLIMissing:
		return Status{State: StateCLIMissing, Message: "CLI contoh belum terpasang (simulasi)"}, nil
	default:
		return Status{State: StateNotLoggedIn, Message: "Belum login. Ikuti langkah di dialog Connect."}, nil
	}
}

func (fakeConnector) ConnectGuide() Guide {
	dir := fakeStateDir()
	if dir == "" {
		dir = "<DataDir>/connector-fake"
	}
	return Guide{
		Steps: []string{
			"Ini connector contoh untuk menguji tampilan.",
			"Salin perintah di bawah lalu jalankan di terminal.",
			"Kembali ke sini dan tekan Cek ulang.",
		},
		Commands: []string{
			`mkdir -p "` + dir + `" && echo connected > "` + dir + `/state"`,
			`echo not_logged_in > "` + dir + `/state"`,
		},
	}
}
