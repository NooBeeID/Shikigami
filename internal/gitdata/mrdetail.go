package gitdata

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var mrDetailRunPattern = regexp.MustCompile(`^(.*?/actions/runs/([0-9]+))`)

// GetMergeRequest mengambil detail satu MR/PR ternormalisasi (read-only).
func GetMergeRequest(ctx context.Context, repo Repo, number int) (MergeRequestDetail, error) {
	if number <= 0 {
		return MergeRequestDetail{}, &Error{Code: ErrorCode("invalid_params"), Message: "Nomor MR tidak valid"}
	}
	switch repo.Provider {
	case ProviderGitLab:
		out, err := Run(ctx, "glab", "mr", "view", strconv.Itoa(number), "--repo", mrRepoArg(repo), "--output", "json")
		if err != nil {
			return MergeRequestDetail{}, err
		}
		return parseGitLabMrDetail(out)
	case ProviderGitHub:
		out, err := Run(ctx, "gh", "pr", "view", strconv.Itoa(number), "--repo", mrRepoArg(repo),
			"--json", "number,title,body,author,headRefName,baseRefName,state,isDraft,reviewDecision,reviewRequests,latestReviews,labels,mergeable,statusCheckRollup,updatedAt,url")
		if err != nil {
			return MergeRequestDetail{}, err
		}
		return parseGitHubPrDetail(out)
	}
	return MergeRequestDetail{}, &Error{Code: CodeCLI, Message: "Provider git tidak didukung"}
}

func mrDetailEnsure(d *MergeRequestDetail) {
	if d.Labels == nil {
		d.Labels = []string{}
	}
	if d.Reviewers == nil {
		d.Reviewers = []Reviewer{}
	}
}

// mrDetailIDString mengubah id angka/string JSON menjadi string; hilang/0/kosong menjadi "".
func mrDetailIDString(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if strings.HasPrefix(s, `"`) {
		var str string
		if json.Unmarshal(raw, &str) != nil {
			return ""
		}
		s = strings.TrimSpace(str)
	}
	if s == "" || s == "0" {
		return ""
	}
	return s
}

type mrDetailGitLabPipe struct {
	ID     json.RawMessage `json:"id"`
	Status string          `json:"status"`
	WebURL string          `json:"web_url"`
}

type mrDetailGitLabRaw struct {
	IID                 int    `json:"iid"`
	Title               string `json:"title"`
	State               string `json:"state"`
	Draft               bool   `json:"draft"`
	SourceBranch        string `json:"source_branch"`
	TargetBranch        string `json:"target_branch"`
	UpdatedAt           string `json:"updated_at"`
	WebURL              string `json:"web_url"`
	Description         string `json:"description"`
	HasConflicts        bool   `json:"has_conflicts"`
	DetailedMergeStatus string `json:"detailed_merge_status"`
	Author              *struct {
		Username string `json:"username"`
	} `json:"author"`
	Labels       []json.RawMessage   `json:"labels"`
	Reviewers    []json.RawMessage   `json:"reviewers"`
	HeadPipeline *mrDetailGitLabPipe `json:"head_pipeline"`
	Pipeline     *mrDetailGitLabPipe `json:"pipeline"`
}

func mrDetailGitLabMergeable(hasConflicts bool, detailed string) string {
	if hasConflicts {
		return "conflict"
	}
	switch d := strings.ToLower(strings.TrimSpace(detailed)); d {
	case "mergeable":
		return "mergeable"
	case "", "not_open":
		return ""
	}
	return "unknown"
}

func mrDetailGitLabPipeRef(p *mrDetailGitLabPipe) *PipelineRef {
	if p == nil {
		return nil
	}
	id := mrDetailIDString(p.ID)
	if id == "" {
		return nil
	}
	return &PipelineRef{ID: id, Status: NormalizeGitLabStatus(p.Status), URL: p.WebURL}
}

func parseGitLabMrDetail(data []byte) (MergeRequestDetail, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" || !strings.HasPrefix(trimmed, "{") {
		return MergeRequestDetail{}, &Error{Code: CodeNotFound, Message: "MR tidak ditemukan"}
	}
	var raw mrDetailGitLabRaw
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return MergeRequestDetail{}, &Error{Code: CodeCLI, Message: providerLabel("glab") + " mengembalikan data yang tidak dapat dibaca"}
	}
	author := ""
	if raw.Author != nil {
		author = raw.Author.Username
	}
	d := MergeRequestDetail{
		MergeRequest: MergeRequest{
			Number:       raw.IID,
			Title:        raw.Title,
			Author:       author,
			SourceBranch: raw.SourceBranch,
			TargetBranch: raw.TargetBranch,
			State:        mrNormGitLabState(raw.State),
			Draft:        raw.Draft,
			UpdatedAt:    raw.UpdatedAt,
			URL:          raw.WebURL,
		},
		Description: raw.Description,
		Mergeable:   mrDetailGitLabMergeable(raw.HasConflicts, raw.DetailedMergeStatus),
	}
	for _, l := range raw.Labels {
		var s string
		if json.Unmarshal(l, &s) != nil {
			continue
		}
		if s = strings.TrimSpace(s); s != "" {
			d.Labels = append(d.Labels, s)
		}
	}
	for _, r := range raw.Reviewers {
		var u struct {
			Username string `json:"username"`
		}
		if json.Unmarshal(r, &u) != nil || strings.TrimSpace(u.Username) == "" {
			continue
		}
		d.Reviewers = append(d.Reviewers, Reviewer{Name: u.Username, State: "pending"})
	}
	d.LastPipeline = mrDetailGitLabPipeRef(raw.HeadPipeline)
	if d.LastPipeline == nil {
		d.LastPipeline = mrDetailGitLabPipeRef(raw.Pipeline)
	}
	if d.LastPipeline != nil {
		d.PipelineStatus = d.LastPipeline.Status
	}
	mrDetailEnsure(&d)
	return d, nil
}

type mrDetailGitHubCheck struct {
	mrCheck
	DetailsURL string `json:"detailsUrl"`
}

type mrDetailGitHubReview struct {
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	State string `json:"state"`
}

type mrDetailGitHubRequest struct {
	Login string `json:"login"`
	Slug  string `json:"slug"`
	Name  string `json:"name"`
}

type mrDetailGitHubRaw struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	State          string `json:"state"`
	IsDraft        bool   `json:"isDraft"`
	ReviewDecision string `json:"reviewDecision"`
	HeadRefName    string `json:"headRefName"`
	BaseRefName    string `json:"baseRefName"`
	Mergeable      string `json:"mergeable"`
	UpdatedAt      string `json:"updatedAt"`
	URL            string `json:"url"`
	Author         *struct {
		Login string `json:"login"`
	} `json:"author"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	ReviewRequests    []mrDetailGitHubRequest `json:"reviewRequests"`
	LatestReviews     []mrDetailGitHubReview  `json:"latestReviews"`
	StatusCheckRollup []mrDetailGitHubCheck   `json:"statusCheckRollup"`
}

func mrDetailGitHubReviewers(latest []mrDetailGitHubReview, requests []mrDetailGitHubRequest) []Reviewer {
	res := []Reviewer{}
	seen := map[string]bool{}
	for _, r := range latest {
		if r.Author == nil {
			continue
		}
		name := strings.TrimSpace(r.Author.Login)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		state := "pending"
		switch strings.ToUpper(strings.TrimSpace(r.State)) {
		case "APPROVED":
			state = "approved"
		case "CHANGES_REQUESTED":
			state = "changes_requested"
		}
		res = append(res, Reviewer{Name: name, State: state})
	}
	for _, r := range requests {
		name := strings.TrimSpace(r.Login)
		if name == "" {
			name = strings.TrimSpace(r.Slug)
		}
		if name == "" {
			name = strings.TrimSpace(r.Name)
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		res = append(res, Reviewer{Name: name, State: "pending"})
	}
	return res
}

func mrDetailGitHubMergeable(state, mergeable string) string {
	if state != "open" {
		return ""
	}
	switch strings.ToUpper(strings.TrimSpace(mergeable)) {
	case "MERGEABLE":
		return "mergeable"
	case "CONFLICTING":
		return "conflict"
	case "UNKNOWN":
		return "unknown"
	}
	return ""
}

func mrDetailGitHubLastPipeline(checks []mrDetailGitHubCheck, status Status) *PipelineRef {
	for _, c := range checks {
		if m := mrDetailRunPattern.FindStringSubmatch(c.DetailsURL); m != nil {
			return &PipelineRef{ID: m[2], Status: status, URL: m[1]}
		}
	}
	return nil
}

func parseGitHubPrDetail(data []byte) (MergeRequestDetail, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" || !strings.HasPrefix(trimmed, "{") {
		return MergeRequestDetail{}, &Error{Code: CodeNotFound, Message: "PR tidak ditemukan"}
	}
	var raw mrDetailGitHubRaw
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return MergeRequestDetail{}, &Error{Code: CodeCLI, Message: providerLabel("gh") + " mengembalikan data yang tidak dapat dibaca"}
	}
	author := ""
	if raw.Author != nil {
		author = raw.Author.Login
	}
	state := strings.ToLower(strings.TrimSpace(raw.State))
	if state != "open" && state != "merged" && state != "closed" {
		state = ""
	}
	checks := make([]mrCheck, 0, len(raw.StatusCheckRollup))
	for _, c := range raw.StatusCheckRollup {
		checks = append(checks, c.mrCheck)
	}
	agg := mrAggregateChecks(checks)
	d := MergeRequestDetail{
		MergeRequest: MergeRequest{
			Number:         raw.Number,
			Title:          raw.Title,
			Author:         author,
			SourceBranch:   raw.HeadRefName,
			TargetBranch:   raw.BaseRefName,
			State:          state,
			Draft:          raw.IsDraft,
			ReviewState:    mrNormReview(raw.ReviewDecision),
			PipelineStatus: agg,
			UpdatedAt:      raw.UpdatedAt,
			URL:            raw.URL,
		},
		Description:  raw.Body,
		Reviewers:    mrDetailGitHubReviewers(raw.LatestReviews, raw.ReviewRequests),
		Mergeable:    mrDetailGitHubMergeable(state, raw.Mergeable),
		LastPipeline: mrDetailGitHubLastPipeline(raw.StatusCheckRollup, agg),
	}
	for _, l := range raw.Labels {
		if n := strings.TrimSpace(l.Name); n != "" {
			d.Labels = append(d.Labels, n)
		}
	}
	mrDetailEnsure(&d)
	return d, nil
}
