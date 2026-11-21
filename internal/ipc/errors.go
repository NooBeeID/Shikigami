package ipc

import (
	"errors"
	"fmt"
)

// Kode galat kustom aplikasi.
const (
	ErrBinary   = -32000
	ErrConflict = -32001
	ErrNotFound = -32002
	ErrInvalid  = -32003
)

// Kode galat standar JSON-RPC 2.0.
const (
	ErrParse          = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternal       = -32603
)

// Error adalah galat JSON-RPC.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *Error) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("%s (kode %d)", e.Message, e.Code)
	}
	return e.Message
}

// NewError membuat galat dengan kode dan pesan.
func NewError(code int, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

// Errorf membuat galat dengan pesan terformat.
func Errorf(code int, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func toError(err error) *Error {
	var e *Error
	if errors.As(err, &e) && e != nil {
		return e
	}
	return &Error{Code: ErrInternal, Message: err.Error()}
}
