package connector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func fakeCLI(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fakecli")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunCLIIgnoresStderrOnSuccess(t *testing.T) {
	bin := fakeCLI(t, `echo "✓ Logged in to gitlab.com as dev" >&2`)
	out, err := RunCLI(context.Background(), bin)
	if err != nil || out != "" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestRunCLICombinedIncludesStderr(t *testing.T) {
	bin := fakeCLI(t, `echo "out-line"; echo "✓ Logged in to gitlab.com as dev" >&2`)
	out, err := RunCLICombined(context.Background(), bin)
	if err != nil {
		t.Fatal(err)
	}
	hosts := parseGitLoginHosts(out)
	if len(hosts) != 1 || hosts[0].Host != "gitlab.com" || hosts[0].User != "dev" {
		t.Fatalf("hosts=%+v out=%q", hosts, out)
	}
}

func TestRunCLICombinedFailureAndMissing(t *testing.T) {
	bin := fakeCLI(t, `echo "boom" >&2; exit 1`)
	if _, err := RunCLICombined(context.Background(), bin); err == nil {
		t.Fatal("harus galat saat exit non-zero")
	}
	_, err := RunCLICombined(context.Background(), "cli-yang-tidak-ada-xyz")
	if !IsCLIMissing(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestParseGitLoginHostsRealGlabOutput(t *testing.T) {
	text := "gitlab.com\n  ✓ Logged in to gitlab.com as j_noobee (/x/config.yml)\n  ✓ Token found: ***\n"
	h, ok := pickGitLoginHost(parseGitLoginHosts(text), "gitlab.com")
	if !ok || h.User != "j_noobee" {
		t.Fatalf("h=%+v ok=%v", h, ok)
	}
}
