package larkdata

import (
	"net/url"
	"strings"
)

// BuildTicketLink membangun link tabel/view Base. Sumber keputusan:
// docs/lark/cli-findings.md bagian 8: format parameter record TIDAK terbukti,
// sehingga link tingkat record tidak ditebak; fungsi mengembalikan link
// tabel/view dan recordLinkAvailable=false. Fungsi murni, tanpa CLI.
func BuildTicketLink(host, baseToken, tableID, viewID, recordID string) (string, bool) {
	host = strings.TrimSpace(host)
	baseToken = strings.TrimSpace(baseToken)
	if host == "" || baseToken == "" {
		return "", false
	}
	base := strings.TrimRight(host, "/")
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}
	link := base + "/base/" + url.PathEscape(baseToken)
	q := url.Values{}
	if t := strings.TrimSpace(tableID); t != "" {
		q.Set("table", t)
	}
	if v := strings.TrimSpace(viewID); v != "" {
		q.Set("view", v)
	}
	if enc := q.Encode(); enc != "" {
		link += "?" + enc
	}
	// Format record tidak terkonfirmasi oleh findings: selalu false.
	_ = recordID
	return link, false
}
