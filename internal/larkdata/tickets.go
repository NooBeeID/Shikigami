package larkdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const pageSize = 50

// FieldTypeSingleSelect adalah nilai normalisasi tipe field single select.
const FieldTypeSingleSelect = "single_select"

const ticketListTimeout = 20 * time.Second

const ticketUntitled = "(Tanpa judul)"

type Field struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsPrimary bool   `json:"isPrimary"`
	// Options berisi nama opsi select sesuai urutan di Lark (kosong bila bukan
	// select atau +field-list gagal). Dipakai untuk urutan kolom kanban.
	Options []string `json:"options,omitempty"`
}

type Record struct {
	ID     string            `json:"id"`
	Title  string            `json:"title"`
	Values map[string]string `json:"values"`
}

type TicketPage struct {
	Fields     []Field  `json:"fields"`
	Records    []Record `json:"records"`
	NextOffset *int     `json:"nextOffset,omitempty"`
}

type TicketDetailField struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

type TicketDetail struct {
	Fields              []TicketDetailField `json:"fields"`
	URL                 string              `json:"url,omitempty"`
	RecordLinkAvailable bool                `json:"recordLinkAvailable"`
}

type TicketListQuery struct {
	BaseToken string
	TableID   string
	ViewID    string
	Offset    int
}

type TicketGetQuery struct {
	BaseToken string
	Host      string
	TableID   string
	ViewID    string
	RecordID  string
}

// ticketMatrix adalah bentuk matriks +record-list/+record-get
// (docs/lark/cli-findings.md bagian 3).
type ticketMatrix struct {
	Fields       []string `json:"fields"`
	FieldIDList  []string `json:"field_id_list"`
	FieldTypes   []string `json:"field_type_list"`
	Rows         [][]any  `json:"data"`
	RecordIDList []string `json:"record_id_list"`
	HasMore      bool     `json:"has_more"`
}

func ticketParse(out []byte) (ticketMatrix, error) {
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(out, &env); err != nil || len(env.Data) == 0 {
		return ticketMatrix{}, &Error{Code: CodeCLI, Message: "Balasan Lark tidak terbaca"}
	}
	var m ticketMatrix
	if err := json.Unmarshal(env.Data, &m); err != nil {
		return ticketMatrix{}, &Error{Code: CodeCLI, Message: "Balasan Lark tidak terbaca"}
	}
	return m, nil
}

// ticketFieldID mengembalikan id field kolom i (fallback nama, lalu indeks).
func ticketFieldID(m ticketMatrix, i int) string {
	if i < len(m.FieldIDList) && m.FieldIDList[i] != "" {
		return m.FieldIDList[i]
	}
	if i < len(m.Fields) && m.Fields[i] != "" {
		return m.Fields[i]
	}
	return "col" + strconv.Itoa(i)
}

func ticketFieldType(m ticketMatrix, i int) string {
	if i < len(m.FieldTypes) {
		return m.FieldTypes[i]
	}
	return ""
}

func ticketColumns(m ticketMatrix) int {
	n := len(m.Fields)
	if len(m.FieldIDList) > n {
		n = len(m.FieldIDList)
	}
	if len(m.FieldTypes) > n {
		n = len(m.FieldTypes)
	}
	return n
}

// normalizeFieldType memetakan tipe CLI ke nilai stabil. Matriks record tidak
// membedakan select tunggal/jamak (hanya +field-list yang punya `multiple`),
// jadi semua "select" dianggap single_select (kandidat pengelompokan kanban).
func normalizeFieldType(raw string) string {
	switch raw {
	case "select":
		return FieldTypeSingleSelect
	case "text", "auto_number":
		return "text"
	case "number", "formula", "lookup":
		return raw
	case "user", "created_by":
		return "user"
	case "link":
		return "link"
	case "attachment":
		return "attachment"
	case "datetime", "created_at", "updated_at":
		return "date"
	case "checkbox":
		return "checkbox"
	default:
		return "other"
	}
}

func ticketFields(m ticketMatrix) []Field {
	n := ticketColumns(m)
	fields := make([]Field, 0, n)
	for i := 0; i < n; i++ {
		name := ""
		if i < len(m.Fields) {
			name = m.Fields[i]
		}
		fields = append(fields, Field{
			ID:        ticketFieldID(m, i),
			Name:      name,
			Type:      normalizeFieldType(ticketFieldType(m, i)),
			IsPrimary: i == 0, // findings bagian 5: kolom pertama = field utama
		})
	}
	return fields
}

// ListTickets membaca satu halaman record. Paginasi memakai has_more dari CLI
// (findings bagian 6); nextOffset hanya bila has_more dan halaman tidak kosong.
func ListTickets(ctx context.Context, q TicketListQuery) (TicketPage, error) {
	if strings.TrimSpace(q.BaseToken) == "" || strings.TrimSpace(q.TableID) == "" {
		return TicketPage{}, errors.New("base token dan table id wajib diisi")
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	ctx, cancel := context.WithTimeout(ctx, ticketListTimeout)
	defer cancel()
	args := []string{"base", "+record-list", "--base-token", q.BaseToken, "--table-id", q.TableID}
	if q.ViewID != "" {
		args = append(args, "--view-id", q.ViewID)
	}
	args = append(args, "--limit", strconv.Itoa(pageSize), "--offset", strconv.Itoa(offset))
	out, err := Run(ctx, args...)
	if err != nil {
		return TicketPage{}, err
	}
	m, err := ticketParse(out)
	if err != nil {
		return TicketPage{}, err
	}
	page := TicketPage{Fields: ticketFields(m), Records: []Record{}}
	if offset == 0 {
		attachFieldOptions(ctx, q, page.Fields)
	}
	for r, row := range m.Rows {
		rec := Record{Values: map[string]string{}}
		if r < len(m.RecordIDList) {
			rec.ID = m.RecordIDList[r]
		}
		for i, f := range page.Fields {
			var cell any
			if i < len(row) {
				cell = row[i]
			}
			rec.Values[f.ID] = ticketDisplayText(cell, ticketFieldType(m, i))
		}
		title := ""
		if len(page.Fields) > 0 {
			title = strings.TrimSpace(rec.Values[page.Fields[0].ID])
		}
		if title == "" {
			title = ticketUntitled
		}
		rec.Title = title
		page.Records = append(page.Records, rec)
	}
	if m.HasMore && len(page.Records) > 0 {
		next := offset + len(page.Records)
		page.NextOffset = &next
	}
	return page, nil
}

// GetTicket membaca satu record; urutan field mengikuti kolom matriks.
func GetTicket(ctx context.Context, q TicketGetQuery) (TicketDetail, error) {
	if strings.TrimSpace(q.BaseToken) == "" || strings.TrimSpace(q.TableID) == "" || strings.TrimSpace(q.RecordID) == "" {
		return TicketDetail{}, errors.New("base token, table id, dan record id wajib diisi")
	}
	ctx, cancel := context.WithTimeout(ctx, ticketListTimeout)
	defer cancel()
	out, err := Run(ctx, "base", "+record-get", "--base-token", q.BaseToken, "--table-id", q.TableID, "--record-id", q.RecordID)
	if err != nil {
		return TicketDetail{}, err
	}
	m, err := ticketParse(out)
	if err != nil {
		return TicketDetail{}, err
	}
	d := TicketDetail{Fields: []TicketDetailField{}}
	if len(m.Rows) > 0 {
		row := m.Rows[0]
		for i := 0; i < ticketColumns(m) && i < len(row); i++ {
			text := strings.TrimSpace(ticketDisplayText(row[i], ticketFieldType(m, i)))
			if text == "" {
				continue
			}
			name := ""
			if i < len(m.Fields) {
				name = m.Fields[i]
			}
			d.Fields = append(d.Fields, TicketDetailField{Name: name, Text: text})
		}
	}
	d.URL, d.RecordLinkAvailable = BuildTicketLink(q.Host, q.BaseToken, q.TableID, q.ViewID, q.RecordID)
	return d, nil
}

// ticketDisplayText mengubah nilai sel menjadi teks tampilan (findings bagian 4).
func ticketDisplayText(v any, fieldType string) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		if fieldType == "datetime" || fieldType == "created_at" || fieldType == "updated_at" {
			return ticketDate(x)
		}
		return x
	case bool:
		if x {
			return "Ya"
		}
		return "Tidak"
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e15 {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case []any:
		if fieldType == "link" {
			if len(x) == 0 {
				return ""
			}
			return strconv.Itoa(len(x)) + " tertaut"
		}
		parts := make([]string, 0, len(x))
		for _, e := range x {
			if s := strings.TrimSpace(ticketDisplayText(e, "")); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		for _, k := range []string{"name", "text", "title", "value", "url", "email", "id"} {
			if s, ok := x[k].(string); ok && s != "" {
				return s
			}
		}
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(b)
	default:
		return fmt.Sprint(x)
	}
}

// ticketDate memformat ISO8601 beroffset menjadi tanggal setempat; gagal parse
// mengembalikan teks asli.
func ticketDate(s string) string {
	for _, layout := range []string{"2006-01-02T15:04:05.000Z07:00", time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
				return t.Format("2006-01-02")
			}
			return t.Format("2006-01-02 15:04")
		}
	}
	return s
}

// attachFieldOptions mengisi Field.Options dari +field-list. Gagal tidak fatal:
// kanban tetap jalan dengan urutan kolom bawaan.
func attachFieldOptions(ctx context.Context, q TicketListQuery, fields []Field) {
	out, err := Run(ctx, "base", "+field-list", "--base-token", q.BaseToken, "--table-id", q.TableID)
	if err != nil {
		return
	}
	var env struct {
		Data struct {
			Fields []struct {
				ID      string `json:"id"`
				Options []struct {
					Name string `json:"name"`
				} `json:"options"`
			} `json:"fields"`
		} `json:"data"`
	}
	if json.Unmarshal(out, &env) != nil {
		return
	}
	byID := make(map[string][]string, len(env.Data.Fields))
	for _, f := range env.Data.Fields {
		if len(f.Options) == 0 {
			continue
		}
		names := make([]string, 0, len(f.Options))
		for _, o := range f.Options {
			names = append(names, o.Name)
		}
		byID[f.ID] = names
	}
	for i := range fields {
		if opts, ok := byID[fields[i].ID]; ok && fields[i].Type == FieldTypeSingleSelect {
			fields[i].Options = opts
		}
	}
}
