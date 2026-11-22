package gitdata

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

const mrLimit = 50

// mrRepoArg membentuk nilai --repo sesuai host (self-hosted memakai URL penuh untuk glab).
func mrRepoArg(r Repo) string {
	switch r.Provider {
	case ProviderGitLab:
		if strings.EqualFold(r.Host, "gitlab.com") {
			return r.Path
		}
		return "https://" + r.Host + "/" + r.Path
	default:
		if strings.EqualFold(r.Host, "github.com") {
			return r.Path
		}
		return r.Host + "/" + r.Path
	}
}

// ListMergeRequests mengambil daftar MR/PR ternormalisasi menurut state (open|merged|closed).
func ListMergeRequests(ctx context.Context, repo Repo, state string) ([]MergeRequest, error) {
	switch state {
	case "open", "merged", "closed":
	default:
		return []MergeRequest{}, &Error{Code: ErrorCode("invalid_params"), Message: "Status MR tidak dikenal: " + state}
	}
	var (
		list []MergeRequest
		err  error
	)
	switch repo.Provider {
	case ProviderGitLab:
		list, err = mrListGitLab(ctx, repo, state)
	case ProviderGitHub:
		list, err = mrListGitHub(ctx, repo, state)
	default:
		return nil, &Error{Code: CodeCLI, Message: "Provider git tidak didukung"}
	}
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []MergeRequest{}
	}
	mrSortByUpdated(list)
	return list, nil
}

func mrNormGitLabState(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "opened":
		return "open"
	case "merged":
		return "merged"
	case "closed", "locked":
		return "closed"
	}
	return ""
}

func mrListGitLab(ctx context.Context, repo Repo, state string) ([]MergeRequest, error) {
	args := []string{"mr", "list", "--repo", mrRepoArg(repo), "--output", "json", "--per-page", "50"}
	switch state {
	case "merged":
		args = append(args, "--merged")
	case "closed":
		args = append(args, "--closed")
	}
	out, err := Run(ctx, "glab", args...)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" || trimmed == "null" {
		return []MergeRequest{}, nil
	}
	var raw []struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		State        string `json:"state"`
		Draft        bool   `json:"draft"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		UpdatedAt    string `json:"updated_at"`
		WebURL       string `json:"web_url"`
		Author       *struct {
			Username string `json:"username"`
		} `json:"author"`
	}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, &Error{Code: CodeCLI, Message: providerLabel("glab") + " mengembalikan data yang tidak dapat dibaca"}
	}
	res := make([]MergeRequest, 0, len(raw))
	for _, it := range raw {
		st := mrNormGitLabState(it.State)
		if st == "" || st != state {
			continue
		}
		author := ""
		if it.Author != nil {
			author = it.Author.Username
		}
		res = append(res, MergeRequest{
			Number:       it.IID,
			Title:        it.Title,
			Author:       author,
			SourceBranch: it.SourceBranch,
			TargetBranch: it.TargetBranch,
			State:        st,
			Draft:        it.Draft,
			UpdatedAt:    it.UpdatedAt,
			URL:          it.WebURL,
		})
	}
	return res, nil
}

type mrCheck struct {
	TypeName   string `json:"__typename"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

func mrListGitHub(ctx context.Context, repo Repo, state string) ([]MergeRequest, error) {
	out, err := Run(ctx, "gh", "pr", "list", "--repo", mrRepoArg(repo), "--state", state, "--limit", "50",
		"--json", "number,title,author,headRefName,baseRefName,state,isDraft,reviewDecision,statusCheckRollup,updatedAt,url")
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" || trimmed == "null" {
		return []MergeRequest{}, nil
	}
	var raw []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Author *struct {
			Login string `json:"login"`
		} `json:"author"`
		HeadRefName       string    `json:"headRefName"`
		BaseRefName       string    `json:"baseRefName"`
		State             string    `json:"state"`
		IsDraft           bool      `json:"isDraft"`
		ReviewDecision    string    `json:"reviewDecision"`
		StatusCheckRollup []mrCheck `json:"statusCheckRollup"`
		UpdatedAt         string    `json:"updatedAt"`
		URL               string    `json:"url"`
	}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, &Error{Code: CodeCLI, Message: providerLabel("gh") + " mengembalikan data yang tidak dapat dibaca"}
	}
	res := make([]MergeRequest, 0, len(raw))
	for _, it := range raw {
		st := strings.ToLower(strings.TrimSpace(it.State))
		if st != "open" && st != "merged" && st != "closed" {
			continue
		}
		if st != state {
			continue
		}
		author := ""
		if it.Author != nil {
			author = it.Author.Login
		}
		res = append(res, MergeRequest{
			Number:         it.Number,
			Title:          it.Title,
			Author:         author,
			SourceBranch:   it.HeadRefName,
			TargetBranch:   it.BaseRefName,
			State:          st,
			Draft:          it.IsDraft,
			ReviewState:    mrNormReview(it.ReviewDecision),
			PipelineStatus: mrAggregateChecks(it.StatusCheckRollup),
			UpdatedAt:      it.UpdatedAt,
			URL:            it.URL,
		})
	}
	return res, nil
}

func mrNormReview(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "APPROVED":
		return "approved"
	case "CHANGES_REQUESTED":
		return "changes_requested"
	case "REVIEW_REQUIRED":
		return "review_required"
	}
	return ""
}

func mrAggregateChecks(checks []mrCheck) Status {
	if len(checks) == 0 {
		return ""
	}
	var failed, running, pending, success, other bool
	for _, c := range checks {
		var st Status
		if c.TypeName == "StatusContext" || (strings.TrimSpace(c.Status) == "" && strings.TrimSpace(c.State) != "") {
			switch strings.ToLower(strings.TrimSpace(c.State)) {
			case "success":
				st = StatusSuccess
			case "failure", "error":
				st = StatusFailed
			case "pending", "expected":
				st = StatusPending
			default:
				st = StatusUnknown
			}
		} else {
			st = NormalizeGitHubStatus(c.Status, c.Conclusion)
		}
		switch st {
		case StatusFailed:
			failed = true
		case StatusRunning:
			running = true
		case StatusPending:
			pending = true
		case StatusSuccess:
			success = true
		case StatusSkipped:
		default:
			other = true
		}
	}
	switch {
	case failed:
		return StatusFailed
	case running:
		return StatusRunning
	case pending:
		return StatusPending
	case success && !other:
		return StatusSuccess
	}
	return StatusUnknown
}

func mrSortByUpdated(list []MergeRequest) {
	ts := func(s string) time.Time {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}
		}
		return t
	}
	sort.SliceStable(list, func(i, j int) bool {
		return ts(list[i].UpdatedAt).After(ts(list[j].UpdatedAt))
	})
}
