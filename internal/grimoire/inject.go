package grimoire

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/NooBeeID/shikigami/internal/paths"
)

const (
	// injectLawsSeparator memisahkan isi index dari bagian laws.
	injectLawsSeparator = "\n# Laws\n\n"
	// injectPlaceholder adalah placeholder folder grimoire di IndexHeader.
	injectPlaceholder = "{{grimoire_dir}}"
)

// injectReplacement mengembalikan nilai pengganti placeholder. IndexHeader
// menulis "{{grimoire_dir}}/books/<buku>/<bab>.md" (sudah memuat /books),
// jadi pengganti adalah folder grimoire project, bukan BooksDir.
func injectReplacement(s *Store, projectID string) string {
	return s.Dir(projectID)
}

// ComposeInjection menyusun teks suntikan grimoire untuk satu project:
// index.md (placeholder diganti path absolut) ditambah laws.md bila ada.
// Tanpa index.md hasilnya ok=false tanpa galat. Keluaran deterministik.
func ComposeInjection(projectID string) (string, bool, error) {
	dataDir, err := paths.DataDir()
	if err != nil {
		return "", false, fmt.Errorf("gagal menentukan data dir: %w", err)
	}
	s := NewStore(dataDir)
	if err := s.ValidateProject(projectID); err != nil {
		return "", false, err
	}
	idx, err := s.ReadFile(projectID, "index.md")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("gagal membaca index.md: %w", err)
	}
	text := strings.ReplaceAll(string(idx), injectPlaceholder, injectReplacement(s, projectID))

	laws, err := s.ReadFile(projectID, "laws.md")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", false, fmt.Errorf("gagal membaca laws.md: %w", err)
		}
	} else if strings.TrimSpace(string(laws)) != "" {
		text += injectLawsSeparator + string(laws)
	}
	return text, true, nil
}
