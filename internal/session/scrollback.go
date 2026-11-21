package session

import "sync"

// ScrollbackSize adalah kapasitas ring buffer scrollback per sesi.
const ScrollbackSize = 256 * 1024

type scrollback struct {
	mu    sync.Mutex
	buf   []byte
	start int
	size  int
	total uint64
}

func newScrollback() *scrollback {
	return &scrollback{buf: make([]byte, ScrollbackSize)}
}

// Append menambahkan p dan mengembalikan total kumulatif sesudahnya.
func (s *scrollback) Append(p []byte) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total += uint64(len(p))
	if len(p) >= ScrollbackSize {
		copy(s.buf, p[len(p)-ScrollbackSize:])
		s.start = 0
		s.size = ScrollbackSize
		return s.total
	}
	if len(p) == 0 {
		return s.total
	}
	end := (s.start + s.size) % ScrollbackSize
	n := copy(s.buf[end:], p)
	if n < len(p) {
		copy(s.buf, p[n:])
	}
	s.size += len(p)
	if s.size > ScrollbackSize {
		s.start = (s.start + s.size - ScrollbackSize) % ScrollbackSize
		s.size = ScrollbackSize
	}
	return s.total
}

// Snapshot mengembalikan salinan isi (tertua ke terbaru) dan total kumulatif.
func (s *scrollback) Snapshot() (data []byte, offset uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data = make([]byte, s.size)
	n := copy(data, s.buf[s.start:minInt(s.start+s.size, ScrollbackSize)])
	if n < s.size {
		copy(data[n:], s.buf[:s.size-n])
	}
	return data, s.total
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
