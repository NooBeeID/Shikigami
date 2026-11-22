package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const gitMrDetailUsage = "penggunaan: shikigamictl git mr <workspaceId> <projectId> <number>"

type gitMrDetailReviewer struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type gitMrDetailPipeline struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

type gitMrDetailResult struct {
	Number         int                   `json:"number"`
	Title          string                `json:"title"`
	Author         string                `json:"author"`
	SourceBranch   string                `json:"sourceBranch"`
	TargetBranch   string                `json:"targetBranch"`
	State          string                `json:"state"`
	Draft          bool                  `json:"draft"`
	ReviewState    string                `json:"reviewState"`
	PipelineStatus string                `json:"pipelineStatus"`
	UpdatedAt      string                `json:"updatedAt"`
	URL            string                `json:"url"`
	Description    string                `json:"description"`
	Labels         []string              `json:"labels"`
	Reviewers      []gitMrDetailReviewer `json:"reviewers"`
	Mergeable      string                `json:"mergeable"`
	LastPipeline   *gitMrDetailPipeline  `json:"lastPipeline"`
}

var gitMrDetailOneLine = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

func gitMrDetailNumber(r gitMrDetailResult) string {
	switch {
	case strings.Contains(r.URL, "/-/merge_requests/"):
		return "!" + strconv.Itoa(r.Number)
	case strings.Contains(r.URL, "/pull/"):
		return "#" + strconv.Itoa(r.Number)
	default:
		return strconv.Itoa(r.Number)
	}
}

func gitMrDetailFormat(r gitMrDetailResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", gitMrDetailNumber(r), gitMrDetailOneLine.Replace(r.Title))

	status := gitMrDetailOneLine.Replace(r.State)
	if r.Draft {
		status += " (draft)"
	}
	fmt.Fprintf(&b, "status: %s\n", status)
	fmt.Fprintf(&b, "penulis: @%s\n", gitMrDetailOneLine.Replace(r.Author))
	fmt.Fprintf(&b, "branch: %s -> %s\n",
		gitMrDetailOneLine.Replace(r.SourceBranch), gitMrDetailOneLine.Replace(r.TargetBranch))

	labels := make([]string, 0, len(r.Labels))
	for _, l := range r.Labels {
		labels = append(labels, gitMrDetailOneLine.Replace(l))
	}
	if len(labels) == 0 {
		b.WriteString("label: -\n")
	} else {
		fmt.Fprintf(&b, "label: %s\n", strings.Join(labels, ", "))
	}

	reviewers := make([]string, 0, len(r.Reviewers))
	for _, rv := range r.Reviewers {
		st := gitMrDetailOneLine.Replace(rv.State)
		if st == "" {
			st = "-"
		}
		reviewers = append(reviewers, fmt.Sprintf("%s (%s)", gitMrDetailOneLine.Replace(rv.Name), st))
	}
	if len(reviewers) == 0 {
		b.WriteString("reviewer: -\n")
	} else {
		fmt.Fprintf(&b, "reviewer: %s\n", strings.Join(reviewers, ", "))
	}

	mergeable := gitMrDetailOneLine.Replace(r.Mergeable)
	if mergeable == "" {
		mergeable = "-"
	}
	fmt.Fprintf(&b, "mergeable: %s\n", mergeable)

	if r.LastPipeline == nil || r.LastPipeline.ID == "" {
		b.WriteString("pipeline: -\n")
	} else {
		line := gitMrDetailOneLine.Replace(r.LastPipeline.ID) + " " + gitMrDetailOneLine.Replace(r.LastPipeline.Status)
		if u := gitMrDetailOneLine.Replace(r.LastPipeline.URL); u != "" {
			line += " " + u
		}
		fmt.Fprintf(&b, "pipeline: %s\n", line)
	}

	fmt.Fprintf(&b, "url: %s\n", gitMrDetailOneLine.Replace(r.URL))
	b.WriteString("\ndeskripsi:\n")
	desc := strings.TrimRight(r.Description, "\n")
	if desc == "" {
		desc = "-"
	}
	b.WriteString(desc)
	b.WriteString("\n")
	return b.String()
}

func gitMrDetail(_ context.Context, c *ipc.Client, args []string) error {
	if len(args) != 3 {
		return errors.New(gitMrDetailUsage)
	}
	ws := strings.TrimSpace(args[0])
	project := strings.TrimSpace(args[1])
	if ws == "" || project == "" {
		return errors.New(gitMrDetailUsage)
	}
	n, err := strconv.Atoi(strings.TrimSpace(args[2]))
	if err != nil || n <= 0 {
		return errors.New(gitMrDetailUsage)
	}

	var res gitMrDetailResult
	err = c.Call("git.mr.get", map[string]any{"workspaceId": ws, "projectId": project, "number": n}, &res)
	if err != nil {
		var ie *ipc.Error
		if errors.As(err, &ie) && ie.Code == ipc.ErrMethodNotFound {
			return errors.New("daemon belum mendukung git.mr.get (perbarui shikigamid)")
		}
		return err
	}
	fmt.Print(gitMrDetailFormat(res))
	return nil
}

func init() { registerGitSub("mr", gitMrDetail) }
