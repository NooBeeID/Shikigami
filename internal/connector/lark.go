package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// larkAuthStatus adalah bentuk keluaran `lark-cli auth status` yang dipakai.
// Sengaja tidak memuat field token. Field tak dikenal atau hilang diabaikan.
type larkAuthStatus struct {
	Identities struct {
		User *struct {
			Status    string `json:"status"`
			Available bool   `json:"available"`
			UserName  string `json:"userName"`
		} `json:"user"`
	} `json:"identities"`
}

// larkConnector adalah connector untuk Lark lewat lark-cli yang sudah login.
type larkConnector struct{}

func init() { Register(larkConnector{}) }

// ID mengembalikan id connector.
func (larkConnector) ID() string { return "lark" }

// Name mengembalikan nama tampilan connector.
func (larkConnector) Name() string { return "Lark" }

// Status membaca `lark-cli auth status` dan memetakannya ke State.
// Tidak menyimpan cache dan tidak memuat output mentah CLI.
func (larkConnector) Status(ctx context.Context) (Status, error) {
	out, err := RunCLI(ctx, "lark-cli", "auth", "status")
	if err != nil {
		if IsCLIMissing(err) {
			return Status{State: StateCLIMissing, Message: "lark-cli belum terpasang"}, nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "not configured") ||
			strings.Contains(strings.ToLower(err.Error()), "not_configured") {
			return Status{State: StateNotLoggedIn, Message: "Belum login ke Lark. Ikuti langkah di dialog Connect."}, nil
		}
		return Status{}, fmt.Errorf("lark-cli auth status: %w", err)
	}
	var st larkAuthStatus
	if err := json.Unmarshal([]byte(out), &st); err != nil ||
		st.Identities.User == nil || !st.Identities.User.Available {
		return Status{State: StateNotLoggedIn, Message: "Belum login ke Lark. Ikuti langkah di dialog Connect."}, nil
	}
	return Status{State: StateConnected, User: st.Identities.User.UserName}, nil
}

// ConnectGuide memberi langkah menyambungkan Lark.
func (larkConnector) ConnectGuide() Guide {
	return Guide{
		Steps: []string{
			"Pasang lark-cli bila belum terpasang di mesin ini.",
			"Salin perintah di bawah, lalu jalankan di terminal untuk login ke Lark.",
			"Kembali ke sini dan tekan Cek ulang.",
		},
		Commands: []string{"lark-cli auth login"},
	}
}
