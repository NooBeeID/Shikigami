package fsx

import (
	"errors"
	"fmt"
)

// Error adalah error terstruktur layanan file dengan kode yang dipetakan ke IPC.
type Error struct {
	Code    int
	Message string
	Data    any
}

func (e *Error) Error() string { return e.Message }

// Is true bila target adalah *Error dengan Code yang sama.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

var (
	ErrBinary   = &Error{Code: -32000, Message: "binary file not supported"}
	ErrConflict = &Error{Code: -32001, Message: "file changed on disk"}
	ErrNotFound = &Error{Code: -32002, Message: "not found"}
	ErrInvalid  = &Error{Code: -32003, Message: "invalid argument"}
)

// IsCode melaporkan apakah err (atau yang dibungkusnya) adalah *Error dengan kode tertentu.
func IsCode(err error, code int) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

// Wrap membungkus sentinel dengan pesan detail.
func Wrap(base *Error, format string, args ...any) error {
	return fmt.Errorf("%w: %s", base, fmt.Sprintf(format, args...))
}

// NewConflict membuat ErrConflict yang membawa mtime disk.
func NewConflict(diskMtime int64) error {
	return &Error{
		Code:    -32001,
		Message: fmt.Sprintf("file changed on disk (disk mtime %d)", diskMtime),
		Data:    map[string]int64{"currentMtime": diskMtime},
	}
}
