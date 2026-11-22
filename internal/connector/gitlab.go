package connector

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// gitLoginRe mengenali baris `Logged in to <host> as|account <akun>`.
var gitLoginRe = regexp.MustCompile(`(?i)logged in to\s+(\S+)\s+(?:as|account)\s+(\S+)`)

// gitAnsiRe membuang kode warna ANSI dari keluaran CLI.
var gitAnsiRe = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// gitLoginHost adalah satu host yang login beserta nama akunnya.
type gitLoginHost struct{ Host, User string }

// parseGitLoginHosts mengurai keluaran teks `glab|gh auth status`.
// Tidak mengembalikan teks mentah dan tidak panic pada input apa pun.
func parseGitLoginHosts(text string) []gitLoginHost {
	text = gitAnsiRe.ReplaceAllString(text, "")
	var hosts []gitLoginHost
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		m := gitLoginRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		host := strings.Trim(m[1], "()[],.;:")
		user := strings.Trim(m[2], "()[],.;:")
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		hosts = append(hosts, gitLoginHost{Host: host, User: user})
	}
	return hosts
}

// pickGitLoginHost memilih host utama: preferred bila ada, selain itu yang pertama.
func pickGitLoginHost(hosts []gitLoginHost, preferred string) (gitLoginHost, bool) {
	if len(hosts) == 0 {
		return gitLoginHost{}, false
	}
	for _, h := range hosts {
		if h.Host == preferred {
			return h, true
		}
	}
	return hosts[0], true
}

// looksNotLoggedIn true bila teks galat/keluaran menandakan belum login.
func looksNotLoggedIn(text string) bool {
	t := strings.ToLower(text)
	for _, p := range []string{
		"not logged in", "not logged into", "no token found", "not authenticated",
		"authentication required", "gh auth login", "glab auth login",
	} {
		if strings.Contains(t, p) {
			return true
		}
	}
	return false
}

func notLoggedInGitLab() Status {
	return Status{State: StateNotLoggedIn, Message: "Belum login ke GitLab. Ikuti langkah di dialog Connect."}
}

// gitlabConnector adalah connector untuk GitLab lewat glab yang sudah login.
type gitlabConnector struct{}

func init() { Register(gitlabConnector{}) }

// ID mengembalikan id connector.
func (gitlabConnector) ID() string { return "gitlab" }

// Name mengembalikan nama tampilan connector.
func (gitlabConnector) Name() string { return "GitLab" }

// Status membaca `glab auth status` dan memetakannya ke State.
// Tidak menyimpan cache dan tidak memuat output mentah CLI.
func (gitlabConnector) Status(ctx context.Context) (Status, error) {
	out, err := RunCLICombined(ctx, "glab", "auth", "status")
	text := out
	if err != nil {
		if IsCLIMissing(err) {
			return Status{State: StateCLIMissing, Message: "glab belum terpasang"}, nil
		}
		text = err.Error()
		if !strings.Contains(strings.ToLower(text), "logged in to") {
			if looksNotLoggedIn(text) {
				return notLoggedInGitLab(), nil
			}
			return Status{}, fmt.Errorf("glab auth status: %w", err)
		}
	}
	h, ok := pickGitLoginHost(parseGitLoginHosts(text), "gitlab.com")
	if !ok {
		return notLoggedInGitLab(), nil
	}
	return Status{State: StateConnected, User: h.User}, nil
}

// ConnectGuide memberi langkah menyambungkan GitLab.
func (gitlabConnector) ConnectGuide() Guide {
	return Guide{
		Steps: []string{
			"Pasang glab (GitLab CLI) bila belum terpasang di mesin ini.",
			"Salin perintah di bawah, lalu jalankan di terminal untuk login ke GitLab.",
			"Kembali ke sini dan tekan Cek ulang.",
		},
		Commands: []string{"glab auth login"},
	}
}
