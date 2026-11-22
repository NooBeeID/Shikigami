package grimoire

import (
	"errors"
	"os"
)

// MaxLawsBytes adalah batas ukuran laws.md.
const MaxLawsBytes = 16 * 1024

// ErrLawsTooLarge dikembalikan bila laws melebihi MaxLawsBytes.
var ErrLawsTooLarge = errors.New("laws melebihi batas ukuran")

// ReadLaws membaca laws.md project ("" bila tak ada). Isi apa adanya.
func (s *Store) ReadLaws(projectID string) (string, error) {
	b, err := s.ReadFile(projectID, "laws.md")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// WriteLaws menulis laws.md project secara atomik. Ukuran dicek (byte)
// sebelum menyentuh disk.
func (s *Store) WriteLaws(projectID, content string) error {
	if len(content) > MaxLawsBytes {
		return ErrLawsTooLarge
	}
	return s.WriteFile(projectID, "laws.md", []byte(content))
}
