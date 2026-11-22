package agents

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/NooBeeID/shikigami/internal/provider"
)

var agentNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// normalizeAgentName menormalkan nama agent menjadi slug.
func normalizeAgentName(raw string) (string, error) {
	n := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(raw))), "-")
	if n == "" {
		return "", errors.New("nama agent wajib diisi")
	}
	if !agentNameRe.MatchString(n) {
		return "", fmt.Errorf("nama agent %q tidak valid: hanya huruf kecil, angka, dan \"-\" (tidak boleh diawali \"-\")", raw)
	}
	return n, nil
}

// slugify membuat id kategori dari nama.
func slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "kategori"
	}
	return out
}

// uniqueCategoryID menambah sufiks -2, -3, ... sampai id unik.
func uniqueCategoryID(base string, cats []Category) string {
	used := func(id string) bool {
		for _, c := range cats {
			if c.ID == id {
				return true
			}
		}
		return false
	}
	if !used(base) {
		return base
	}
	for i := 2; ; i++ {
		id := fmt.Sprintf("%s-%d", base, i)
		if !used(id) {
			return id
		}
	}
}

func newAgentID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "agt_" + hex.EncodeToString(b)
}

func normalizeCategoryName(raw string) (string, error) {
	n := strings.TrimSpace(raw)
	if n == "" {
		return "", errors.New("nama kategori wajib diisi")
	}
	return n, nil
}

// validateProvider memeriksa provider untuk sebuah kategori.
func (s *Store) validateProvider(providerID, categoryID string) error {
	id := strings.TrimSpace(providerID)
	if id == "" {
		return errors.New("provider wajib diisi")
	}
	if id == provider.IDShell {
		return errors.New("provider shell tidak boleh dipakai sebagai agent")
	}
	p, ok := s.reg.Get(id)
	if !ok {
		return fmt.Errorf("provider %q tidak terdaftar", id)
	}
	if categoryID == idOrchestrator && !p.Capabilities().MCP {
		return fmt.Errorf("provider %s tidak mendukung MCP, tidak bisa jadi Orchestrator", id)
	}
	return nil
}
