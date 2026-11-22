package larkdata

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const wikiPageSize = 50

// PersonalSpaceID adalah ID ruang pribadi (Pribadi) di Lark wiki.
const PersonalSpaceID = "my_library"

// WikiNode adalah satu node pohon wiki yang sudah dinormalisasi.
type WikiNode struct {
	Token    string `json:"token"`
	Title    string `json:"title"`
	ObjType  string `json:"objType"`
	ObjToken string `json:"objToken"`
	HasChild bool   `json:"hasChild"`
}

// WikiPage adalah satu halaman hasil list node.
type WikiPage struct {
	Nodes   []WikiNode `json:"nodes"` // tidak pernah nil
	HasMore bool       `json:"hasMore"`
}

// WikiNodeInfo adalah metadata satu node wiki.
type WikiNodeInfo struct {
	Title    string `json:"title"`
	ObjType  string `json:"objType"`
	ObjToken string `json:"objToken"`
	URL      string `json:"url"` // kosong bila host kosong
}

// wikiCtx memberi batas waktu bila ctx belum punya deadline.
func wikiCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, ListTimeout)
}

type wikiListResp struct {
	Data struct {
		Nodes []struct {
			NodeToken string `json:"node_token"`
			ObjToken  string `json:"obj_token"`
			ObjType   string `json:"obj_type"`
			Title     string `json:"title"`
			HasChild  bool   `json:"has_child"`
		} `json:"nodes"`
		HasMore bool `json:"has_more"`
	} `json:"data"`
}

type wikiGetResp struct {
	Data struct {
		ObjToken string `json:"obj_token"`
		ObjType  string `json:"obj_type"`
		Title    string `json:"title"`
	} `json:"data"`
}

// ListWikiNodes membungkus ListWikiNodesPage dan mengembalikan node-nya saja.
func ListWikiNodes(ctx context.Context, spaceID, parentToken string) ([]WikiNode, error) {
	p, err := ListWikiNodesPage(ctx, spaceID, parentToken)
	return p.Nodes, err
}

// ListWikiNodesPage mengambil satu halaman node (satu level).
func ListWikiNodesPage(ctx context.Context, spaceID, parentToken string) (WikiPage, error) {
	empty := WikiPage{Nodes: []WikiNode{}}
	if spaceID == "" {
		return empty, &Error{Code: CodeCLI, Message: "Space wiki kosong"}
	}
	args := []string{"wiki", "+node-list", "--space-id", spaceID, "--page-size", strconv.Itoa(wikiPageSize)}
	if parentToken != "" {
		args = append(args, "--parent-node-token", parentToken)
	}
	ctx2, cancel := wikiCtx(ctx)
	defer cancel()
	out, err := Run(ctx2, args...)
	if err != nil {
		return empty, err
	}
	var resp wikiListResp
	if err := json.Unmarshal(out, &resp); err != nil {
		return empty, &Error{Code: CodeCLI, Message: "Balasan daftar wiki tidak terbaca"}
	}
	nodes := make([]WikiNode, 0, len(resp.Data.Nodes))
	for _, n := range resp.Data.Nodes {
		nodes = append(nodes, WikiNode{Token: n.NodeToken, Title: n.Title, ObjType: n.ObjType, ObjToken: n.ObjToken, HasChild: n.HasChild})
	}
	return WikiPage{Nodes: nodes, HasMore: resp.Data.HasMore}, nil
}

// GetWikiNode mengambil metadata node dan membangun URL dari host.
func GetWikiNode(ctx context.Context, spaceID, nodeToken, host string) (WikiNodeInfo, error) {
	if nodeToken == "" {
		return WikiNodeInfo{}, &Error{Code: CodeCLI, Message: "Token node kosong"}
	}
	args := []string{"wiki", "+node-get", "--node-token", nodeToken}
	if spaceID != "" && spaceID != PersonalSpaceID {
		args = append(args, "--space-id", spaceID)
	}
	ctx2, cancel := wikiCtx(ctx)
	defer cancel()
	out, err := Run(ctx2, args...)
	if err != nil {
		return WikiNodeInfo{}, err
	}
	var resp wikiGetResp
	if err := json.Unmarshal(out, &resp); err != nil {
		return WikiNodeInfo{}, &Error{Code: CodeCLI, Message: "Balasan node wiki tidak terbaca"}
	}
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimRight(host, "/")
	url := ""
	if host != "" {
		url = fmt.Sprintf("https://%s/wiki/%s", host, nodeToken)
	}
	return WikiNodeInfo{Title: resp.Data.Title, ObjType: resp.Data.ObjType, ObjToken: resp.Data.ObjToken, URL: url}, nil
}
