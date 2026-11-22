package larkdata

import (
	"context"
	"net/url"
	"strings"
)

// CodeInvalidLink adalah kode galat untuk link Lark yang tidak dikenali.
const CodeInvalidLink ErrorCode = "invalid_link"

func invalidLink() *Error {
	return &Error{Code: CodeInvalidLink, Message: "Link tidak dikenali"}
}

// ParsedLink adalah hasil normalisasi link Base atau wiki.
type ParsedLink struct {
	Kind        string `json:"kind"` // "base" | "wiki"
	Host        string `json:"host"`
	BaseToken   string `json:"baseToken,omitempty"`
	TableID     string `json:"tableId,omitempty"`
	ViewID      string `json:"viewId,omitempty"`
	SpaceID     string `json:"spaceId,omitempty"`
	NodeToken   string `json:"nodeToken,omitempty"`
	DefaultName string `json:"defaultName,omitempty"`
}

// fatalForName: galat yang harus diteruskan ke pemanggil saat resolusi nama.
func fatalForName(err error) bool {
	e, ok := AsError(err)
	if !ok {
		return false
	}
	return e.Code == CodeCLIMissing || e.Code == CodeNotLoggedIn || e.Code == CodePermission
}

func resolveBaseName(ctx context.Context, baseToken, host string) (string, error) {
	name, err := DefaultBaseName(ctx, baseToken, host)
	if err != nil {
		if fatalForName(err) {
			return "", err
		}
		return "Base", nil
	}
	if name == "" {
		name = "Base"
	}
	return name, nil
}

// ParseLink mengurai link Base atau wiki Lark.
func ParseLink(ctx context.Context, link string) (ParsedLink, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return ParsedLink{}, invalidLink()
	}
	u, err := url.Parse(link)
	if err != nil {
		return ParsedLink{}, invalidLink()
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Hostname() == "" {
		return ParsedLink{}, invalidLink()
	}
	host := u.Host

	var segs []string
	for _, s := range strings.Split(u.Path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	marker, token := "", ""
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "base" || segs[i] == "wiki" {
			marker, token = segs[i], segs[i+1]
			break
		}
	}
	if marker == "" || token == "" {
		return ParsedLink{}, invalidLink()
	}

	q := u.Query()
	tableID, viewID := q.Get("table"), q.Get("view")
	if (tableID == "" || viewID == "") && strings.Contains(u.Fragment, "=") {
		if fq, err := url.ParseQuery(u.Fragment); err == nil {
			if tableID == "" {
				tableID = fq.Get("table")
			}
			if viewID == "" {
				viewID = fq.Get("view")
			}
		}
	}

	if marker == "base" {
		name, err := resolveBaseName(ctx, token, host)
		if err != nil {
			return ParsedLink{}, err
		}
		return ParsedLink{Kind: "base", Host: host, BaseToken: token, TableID: tableID, ViewID: viewID, DefaultName: name}, nil
	}

	// wiki
	if token == "space" {
		return ParsedLink{}, invalidLink()
	}
	out, err := Run(ctx, "wiki", "+node-get", "--node-token", token)
	if err != nil {
		return ParsedLink{}, err
	}
	v, err := decodeJSON(out)
	if err != nil {
		return ParsedLink{}, err
	}
	n := findObject(v, "node")
	spaceID := pickString(n, "space_id")
	objType := strings.ToLower(pickString(n, "obj_type"))
	objToken := pickString(n, "obj_token")
	title := pickString(n, "title")

	if objType == "bitable" || objType == "base" {
		if objToken == "" {
			return ParsedLink{}, &Error{Code: CodeCLI, Message: "Wiki berisi Base tetapi token Base tidak tersedia"}
		}
		name := title
		if name == "" {
			name, err = resolveBaseName(ctx, objToken, host)
			if err != nil {
				return ParsedLink{}, err
			}
		}
		return ParsedLink{Kind: "base", Host: host, BaseToken: objToken, TableID: tableID, ViewID: viewID, DefaultName: name}, nil
	}
	if spaceID == "" {
		return ParsedLink{}, &Error{Code: CodeCLI, Message: "Balasan Lark tidak terbaca"}
	}
	if title == "" {
		title = "Wiki"
	}
	return ParsedLink{Kind: "wiki", Host: host, SpaceID: spaceID, NodeToken: token, DefaultName: title}, nil
}
