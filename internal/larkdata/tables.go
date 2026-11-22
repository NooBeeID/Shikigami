package larkdata

import (
	"context"
	"encoding/json"
	"strconv"
)

// Table adalah tabel dalam sebuah Base.
type Table struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// View adalah tampilan (view) dalam sebuah tabel Base.
type View struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// decodeJSON mengurai keluaran CLI ke any.
func decodeJSON(b []byte) (any, error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, &Error{Code: CodeCLI, Message: "Balasan Lark tidak terbaca"}
	}
	return v, nil
}

// pickString mengembalikan nilai string tak kosong pertama dari keys.
func pickString(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		switch x := m[k].(type) {
		case string:
			if x != "" {
				return x
			}
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		}
	}
	return ""
}

func asObject(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// findObject membuka pembungkus "data" lalu salah satu keys (mis. "node") bila
// ada; bila tidak ada pembungkus, mengembalikan objek itu sendiri.
func findObject(v any, keys ...string) map[string]any {
	m := asObject(v)
	if m == nil {
		return nil
	}
	if d := asObject(m["data"]); d != nil {
		m = d
	}
	for _, k := range keys {
		if o := asObject(m[k]); o != nil {
			return o
		}
	}
	return m
}

func objectsOf(a []any) []map[string]any {
	out := []map[string]any{}
	for _, e := range a {
		if o := asObject(e); o != nil {
			out = append(out, o)
		}
	}
	return out
}

// findList mencari array di bawah salah satu keys (maksimal dua tingkat).
func findList(v any, keys ...string) []map[string]any {
	if a, ok := v.([]any); ok {
		return objectsOf(a)
	}
	m := asObject(v)
	if m == nil {
		return []map[string]any{}
	}
	for _, level := range []map[string]any{m, asObject(m["data"])} {
		if level == nil {
			continue
		}
		for _, k := range keys {
			if a, ok := level[k].([]any); ok {
				return objectsOf(a)
			}
		}
	}
	return []map[string]any{}
}

// listTablesRaw memanggil +table-list sekali dan mengembalikan JSON mentah
// beserta tabel yang sudah dinormalisasi.
func listTablesRaw(ctx context.Context, baseToken string) (any, []Table, error) {
	if baseToken == "" {
		return nil, nil, &Error{Code: CodeCLI, Message: "Base token kosong"}
	}
	out, err := Run(ctx, "base", "+table-list", "--base-token", baseToken)
	if err != nil {
		return nil, nil, err
	}
	v, err := decodeJSON(out)
	if err != nil {
		return nil, nil, err
	}
	tables := []Table{}
	for _, e := range findList(v, "tables", "items", "data") {
		id := pickString(e, "table_id", "id")
		if id == "" {
			continue
		}
		tables = append(tables, Table{ID: id, Name: pickString(e, "name", "table_name", "title")})
	}
	return v, tables, nil
}

// ListTables mengembalikan tabel sebuah Base. host hanya untuk konsistensi
// tanda tangan: +table-list tidak membutuhkan host.
func ListTables(ctx context.Context, baseToken, host string) ([]Table, error) {
	_, tables, err := listTablesRaw(ctx, baseToken)
	if err != nil {
		return nil, err
	}
	return tables, nil
}

// ListViews mengembalikan view sebuah tabel Base.
func ListViews(ctx context.Context, baseToken, tableID string) ([]View, error) {
	if baseToken == "" || tableID == "" {
		return nil, &Error{Code: CodeCLI, Message: "Base token atau table id kosong"}
	}
	out, err := Run(ctx, "base", "+view-list", "--base-token", baseToken, "--table-id", tableID)
	if err != nil {
		return nil, err
	}
	v, err := decodeJSON(out)
	if err != nil {
		return nil, err
	}
	views := []View{}
	for _, e := range findList(v, "views", "items", "data") {
		id := pickString(e, "view_id", "id")
		if id == "" {
			continue
		}
		views = append(views, View{
			ID:   id,
			Name: pickString(e, "view_name", "name", "title"),
			Type: pickString(e, "view_type", "type"),
		})
	}
	return views, nil
}

// DefaultBaseName meresolusi nama default Base dari satu panggilan +table-list:
// nama Base bila ada di respons, selain itu nama tabel pertama, selain itu "Base".
func DefaultBaseName(ctx context.Context, baseToken, host string) (string, error) {
	v, tables, err := listTablesRaw(ctx, baseToken)
	if err != nil {
		return "", err
	}
	if root := findObject(v); root != nil {
		if n := pickString(root, "base_name", "app_name"); n != "" {
			return n, nil
		}
		for _, k := range []string{"base", "app"} {
			if n := pickString(asObject(root[k]), "name"); n != "" {
				return n, nil
			}
		}
	}
	if len(tables) > 0 && tables[0].Name != "" {
		return tables[0].Name, nil
	}
	return "Base", nil
}
