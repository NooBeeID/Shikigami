package larkdata

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	sheetMaxRows = 50
	sheetMaxCols = 20
)

// SheetCells adalah matriks sel awal sebuah sheet (maks 50 x 20).
type SheetCells struct {
	Cells     [][]string `json:"cells"`
	Truncated bool       `json:"truncated"`
}

var docBlockTagRe = regexp.MustCompile(`^\s*</?(title|readonly-block)\b[^>]*>\s*$`)

// FetchDocMarkdown mengambil isi docx sebagai markdown.
func FetchDocMarkdown(ctx context.Context, objToken string) (string, error) {
	if objToken == "" {
		return "", &Error{Code: CodeCLI, Message: "Token dokumen kosong"}
	}
	ctx2, cancel := wikiCtx(ctx)
	defer cancel()
	out, err := Run(ctx2, "docs", "+fetch", "--doc", objToken, "--doc-format", "markdown")
	if err != nil {
		return "", err
	}
	var resp struct {
		Data struct {
			Document struct {
				Content string `json:"content"`
			} `json:"document"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", &Error{Code: CodeCLI, Message: "Balasan dokumen tidak terbaca"}
	}
	content := strings.ReplaceAll(resp.Data.Document.Content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if docBlockTagRe.MatchString(l) {
			continue
		}
		kept = append(kept, l)
	}
	return strings.Join(kept, "\n"), nil
}

func sheetColName(n int) string {
	if n < 1 || n > 26 {
		return "A"
	}
	return string(rune('A' + n - 1))
}

// sheetCellValue mengubah nilai sel mentah menjadi string.
func sheetCellValue(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

// FetchSheetCells mengambil sel awal sheet pertama yang terlihat.
func FetchSheetCells(ctx context.Context, objToken string) (SheetCells, error) {
	empty := SheetCells{Cells: [][]string{}}
	if objToken == "" {
		return empty, &Error{Code: CodeCLI, Message: "Token spreadsheet kosong"}
	}
	ctx2, cancel := wikiCtx(ctx)
	defer cancel()
	out, err := Run(ctx2, "sheets", "+workbook-info", "--spreadsheet-token", objToken)
	if err != nil {
		return empty, err
	}
	var info struct {
		Data struct {
			Sheets []struct {
				SheetID     string `json:"sheet_id"`
				IsHidden    bool   `json:"is_hidden"`
				RowCount    int    `json:"row_count"`
				ColumnCount int    `json:"column_count"`
			} `json:"sheets"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return empty, &Error{Code: CodeCLI, Message: "Balasan info spreadsheet tidak terbaca"}
	}
	sheetID, rows, cols := "", 0, 0
	for _, s := range info.Data.Sheets {
		if !s.IsHidden {
			sheetID, rows, cols = s.SheetID, s.RowCount, s.ColumnCount
			break
		}
	}
	if sheetID == "" {
		return empty, nil
	}
	rng := fmt.Sprintf("A1:%s%d", sheetColName(sheetMaxCols), sheetMaxRows)
	out, err = Run(ctx2, "sheets", "+cells-get", "--spreadsheet-token", objToken, "--sheet-id", sheetID, "--range", rng)
	if err != nil {
		return empty, err
	}
	var cells struct {
		Data struct {
			HasMore bool `json:"has_more"`
			Ranges  []struct {
				Cells [][]struct {
					Value json.RawMessage `json:"value"`
				} `json:"cells"`
			} `json:"ranges"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &cells); err != nil {
		return empty, &Error{Code: CodeCLI, Message: "Balasan sel spreadsheet tidak terbaca"}
	}
	matrix := [][]string{}
	if len(cells.Data.Ranges) > 0 {
		src := cells.Data.Ranges[0].Cells
		for i, row := range src {
			if i >= sheetMaxRows {
				break
			}
			r := make([]string, 0, len(row))
			for j, c := range row {
				if j >= sheetMaxCols {
					break
				}
				r = append(r, sheetCellValue(c.Value))
			}
			matrix = append(matrix, r)
		}
	}
	return SheetCells{
		Cells:     matrix,
		Truncated: rows > sheetMaxRows || cols > sheetMaxCols || cells.Data.HasMore,
	}, nil
}
