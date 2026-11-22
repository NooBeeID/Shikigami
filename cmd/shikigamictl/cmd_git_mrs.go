package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const gitMrsUsage = "penggunaan: shikigamictl git mrs <workspaceId> <projectId> open|merged|closed"

type gitMrsItem struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	SourceBranch   string `json:"sourceBranch"`
	TargetBranch   string `json:"targetBranch"`
	State          string `json:"state"`
	Draft          bool   `json:"draft"`
	ReviewState    string `json:"reviewState"`
	PipelineStatus string `json:"pipelineStatus"`
	UpdatedAt      string `json:"updatedAt"`
	URL            string `json:"url"`
}

type gitMrsResult struct {
	Mrs []gitMrsItem `json:"mrs"`
}

var gitMrsOneLine = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

func gitMrsNumber(it gitMrsItem) string {
	switch {
	case strings.Contains(it.URL, "/-/merge_requests/"):
		return fmt.Sprintf("!%d", it.Number)
	case strings.Contains(it.URL, "/pull/"):
		return fmt.Sprintf("#%d", it.Number)
	}
	return fmt.Sprintf("%d", it.Number)
}

// gitMrsLine membentuk satu baris; kolom draft, pipeline, dan review hanya
// ada bila relevan (posisi kolom boleh bergeser, ini alat debug).
func gitMrsLine(it gitMrsItem) string {
	cols := []string{gitMrsNumber(it), gitMrsOneLine.Replace(it.State)}
	if it.Draft {
		cols = append(cols, "draft")
	}
	cols = append(cols,
		gitMrsOneLine.Replace(it.Title),
		"@"+gitMrsOneLine.Replace(it.Author),
		gitMrsOneLine.Replace(it.SourceBranch)+"->"+gitMrsOneLine.Replace(it.TargetBranch),
	)
	if it.PipelineStatus != "" {
		cols = append(cols, "pipeline="+gitMrsOneLine.Replace(it.PipelineStatus))
	}
	if it.ReviewState != "" {
		cols = append(cols, "review="+gitMrsOneLine.Replace(it.ReviewState))
	}
	cols = append(cols, it.URL)
	return strings.Join(cols, "\t")
}

func gitMrs(_ context.Context, c *ipc.Client, args []string) error {
	if len(args) != 3 {
		return errors.New(gitMrsUsage)
	}
	ws := strings.TrimSpace(args[0])
	project := strings.TrimSpace(args[1])
	state := strings.TrimSpace(args[2])
	if ws == "" || project == "" {
		return errors.New(gitMrsUsage)
	}
	switch state {
	case "open", "merged", "closed":
	default:
		return errors.New(gitMrsUsage)
	}
	var res gitMrsResult
	if err := c.Call("git.mrs.list", map[string]any{
		"workspaceId": ws, "projectId": project, "state": state,
	}, &res); err != nil {
		return err
	}
	for _, it := range res.Mrs {
		fmt.Println(gitMrsLine(it))
	}
	fmt.Printf("%d baris\n", len(res.Mrs))
	return nil
}

func init() { registerGitSub("mrs", gitMrs) }
