package larkdata

import "strings"

type spec struct {
	domain, command string
	asUser          bool
	format          []string
}

// allowed hanya berisi perintah baca. Sesuai docs/lark/cli-findings.md semua
// entri memakai --as user dan --format json (wajib untuk +record-list/+record-get).
var allowed = []spec{
	{"base", "+table-list", true, []string{"--format", "json"}},
	{"base", "+view-list", true, []string{"--format", "json"}},
	{"base", "+record-list", true, []string{"--format", "json"}},
	{"base", "+record-get", true, []string{"--format", "json"}},
	{"base", "+field-list", true, []string{"--format", "json"}},
	{"wiki", "+space-list", true, []string{"--format", "json"}},
	{"wiki", "+node-list", true, []string{"--format", "json"}},
	{"wiki", "+node-get", true, []string{"--format", "json"}},
	{"docs", "+fetch", true, []string{"--format", "json"}},
	{"sheets", "+cells-get", true, []string{"--format", "json"}},
	{"sheets", "+workbook-info", true, []string{"--format", "json"}},
}

var forbiddenFlags = []string{"--as", "--format", "--json", "--output", "--jq", "--yes", "--force"}

func check(args []string) (spec, *Error) {
	if len(args) == 0 {
		return spec{}, &Error{Code: CodeNotAllowed, Message: "Perintah Lark kosong"}
	}
	if len(args) < 2 {
		return spec{}, &Error{Code: CodeNotAllowed, Message: "Perintah Lark tidak diizinkan: " + args[0]}
	}
	var found *spec
	for i := range allowed {
		if allowed[i].domain == args[0] && allowed[i].command == args[1] {
			found = &allowed[i]
			break
		}
	}
	if found == nil {
		return spec{}, &Error{Code: CodeNotAllowed, Message: "Perintah Lark tidak diizinkan: " + args[0] + " " + args[1]}
	}
	for _, a := range args[2:] {
		for _, f := range forbiddenFlags {
			if a == f || strings.HasPrefix(a, f+"=") {
				return spec{}, &Error{Code: CodeNotAllowed, Message: "Perintah Lark tidak diizinkan: flag " + f + " diatur otomatis"}
			}
		}
	}
	return *found, nil
}
