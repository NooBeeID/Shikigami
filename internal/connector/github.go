package connector

import (
	"context"
	"fmt"
	"strings"
)

func notLoggedInGitHub() Status {
	return Status{State: StateNotLoggedIn, Message: "Belum login ke GitHub. Ikuti langkah di dialog Connect."}
}

// githubConnector adalah connector untuk GitHub lewat gh yang sudah login.
type githubConnector struct{}

func init() { Register(githubConnector{}) }

// ID mengembalikan id connector.
func (githubConnector) ID() string { return "github" }

// Name mengembalikan nama tampilan connector.
func (githubConnector) Name() string { return "GitHub" }

// Status membaca `gh auth status` dan memetakannya ke State.
// Tidak menyimpan cache dan tidak memuat output mentah CLI.
func (githubConnector) Status(ctx context.Context) (Status, error) {
	out, err := RunCLI(ctx, "gh", "auth", "status")
	text := out
	if err != nil {
		if IsCLIMissing(err) {
			return Status{State: StateCLIMissing, Message: "gh belum terpasang"}, nil
		}
		text = err.Error()
		if !strings.Contains(strings.ToLower(text), "logged in to") {
			if looksNotLoggedIn(text) {
				return notLoggedInGitHub(), nil
			}
			return Status{}, fmt.Errorf("gh auth status: %w", err)
		}
	}
	h, ok := pickGitLoginHost(parseGitLoginHosts(text), "github.com")
	if !ok {
		return notLoggedInGitHub(), nil
	}
	return Status{State: StateConnected, User: h.User}, nil
}

// ConnectGuide memberi langkah menyambungkan GitHub.
func (githubConnector) ConnectGuide() Guide {
	return Guide{
		Steps: []string{
			"Pasang gh (GitHub CLI) bila belum terpasang di mesin ini.",
			"Salin perintah di bawah, lalu jalankan di terminal untuk login ke GitHub.",
			"Kembali ke sini dan tekan Cek ulang.",
		},
		Commands: []string{"gh auth login"},
	}
}
