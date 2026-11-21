package ipc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
)

const jsonrpcVersion = "2.0"

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type eventParams struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
}

type prefixParams struct {
	Prefix string `json:"prefix"`
}

func normID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}
	return id
}

func errorResponse(id json.RawMessage, e *Error) []byte {
	b, err := json.Marshal(response{JSONRPC: jsonrpcVersion, ID: normID(id), Error: e})
	if err != nil {
		b = []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"galat internal"}}`)
	}
	return append(b, '\n')
}

func resultResponse(id json.RawMessage, res json.RawMessage) []byte {
	if len(res) == 0 {
		res = json.RawMessage("null")
	}
	b, err := json.Marshal(response{JSONRPC: jsonrpcVersion, ID: normID(id), Result: res})
	if err != nil {
		return errorResponse(id, NewError(ErrInternal, err.Error()))
	}
	return append(b, '\n')
}

// readMessage membaca satu baris tak kosong. Tanpa batas ukuran.
func readMessage(r *bufio.Reader) ([]byte, error) {
	for {
		line, err := r.ReadBytes('\n')
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			return trimmed, nil
		}
		if err != nil {
			if err == io.EOF {
				return nil, io.EOF
			}
			return nil, err
		}
	}
}

func encodeLine(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
