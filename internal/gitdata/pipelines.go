package gitdata

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PipelineLimit adalah jumlah maksimum pipeline/run pada daftar.
const PipelineLimit = 20

var pipelineIDPattern = regexp.MustCompile("^[0-9]+$")

const pipelineGitHubRunFields = "databaseId,headBranch,headSha,status,conclusion,createdAt,updatedAt,startedAt,url,event,displayTitle,workflowName"

func pipelineRepoArg(r Repo) string {
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

func pipelineParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func pipelineUnreadable(tool string) *Error {
	return &Error{Code: CodeCLI, Message: providerLabel(tool) + " mengembalikan data yang tidak dapat dibaca"}
}

func pipelineUnsupported() *Error {
	return &Error{Code: CodeCLI, Message: "Provider git tidak didukung"}
}

func pipelineIsEmpty(t []byte) bool {
	return len(t) == 0 || bytes.Equal(t, []byte("null"))
}

func pipelineID(id int64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

func pipelineSeconds(from, to string) int {
	a, b := pipelineParseTime(from), pipelineParseTime(to)
	if a.IsZero() || b.IsZero() {
		return 0
	}
	d := b.Sub(a)
	if d < 0 {
		return 0
	}
	return int(d / time.Second)
}

type pipelineGitLabRaw struct {
	ID        int64  `json:"id"`
	Ref       string `json:"ref"`
	SHA       string `json:"sha"`
	Status    string `json:"status"`
	WebURL    string `json:"web_url"`
	UpdatedAt string `json:"updated_at"`
}

func (r pipelineGitLabRaw) pipeline() Pipeline {
	return Pipeline{
		ID:        pipelineID(r.ID),
		Ref:       r.Ref,
		SHA:       r.SHA,
		Status:    NormalizeGitLabStatus(r.Status),
		UpdatedAt: r.UpdatedAt,
		URL:       r.WebURL,
	}
}

func parseGitLabPipelines(data []byte) ([]Pipeline, error) {
	t := bytes.TrimSpace(data)
	if pipelineIsEmpty(t) {
		return []Pipeline{}, nil
	}
	var raws []pipelineGitLabRaw
	if err := json.Unmarshal(t, &raws); err != nil {
		return nil, pipelineUnreadable("glab")
	}
	out := make([]Pipeline, 0, len(raws))
	for _, r := range raws {
		out = append(out, r.pipeline())
	}
	return out, nil
}

type pipelineGitHubRaw struct {
	DatabaseID int64  `json:"databaseId"`
	HeadBranch string `json:"headBranch"`
	HeadSha    string `json:"headSha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	StartedAt  string `json:"startedAt"`
	URL        string `json:"url"`
}

func pipelineFromGitHubRun(r pipelineGitHubRaw) Pipeline {
	p := Pipeline{
		ID:        pipelineID(r.DatabaseID),
		Ref:       r.HeadBranch,
		SHA:       r.HeadSha,
		Status:    NormalizeGitHubStatus(r.Status, r.Conclusion),
		UpdatedAt: r.UpdatedAt,
		URL:       r.URL,
	}
	if p.UpdatedAt == "" {
		p.UpdatedAt = r.CreatedAt
	}
	if p.Status != StatusPending && p.Status != StatusRunning {
		p.DurationSec = pipelineSeconds(r.StartedAt, r.UpdatedAt)
	}
	return p
}

func parseGitHubRuns(data []byte) ([]Pipeline, error) {
	t := bytes.TrimSpace(data)
	if pipelineIsEmpty(t) {
		return []Pipeline{}, nil
	}
	var raws []pipelineGitHubRaw
	if err := json.Unmarshal(t, &raws); err != nil {
		return nil, pipelineUnreadable("gh")
	}
	out := make([]Pipeline, 0, len(raws))
	for _, r := range raws {
		out = append(out, pipelineFromGitHubRun(r))
	}
	return out, nil
}

func parseGitLabPipelineDetail(data []byte) (PipelineDetail, error) {
	notFound := &Error{Code: CodeNotFound, Message: "Pipeline tidak ditemukan"}
	empty := PipelineDetail{Stages: []Stage{}}
	t := bytes.TrimSpace(data)
	if pipelineIsEmpty(t) || t[0] != '{' {
		return empty, notFound
	}
	var raw struct {
		pipelineGitLabRaw
		User *struct {
			Username string `json:"username"`
		} `json:"user"`
		Duration float64 `json:"duration"`
		Jobs     []struct {
			ID       int64   `json:"id"`
			Name     string  `json:"name"`
			Stage    string  `json:"stage"`
			Status   string  `json:"status"`
			Duration float64 `json:"duration"`
			WebURL   string  `json:"web_url"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(t, &raw); err != nil {
		return empty, notFound
	}
	d := PipelineDetail{Pipeline: raw.pipelineGitLabRaw.pipeline(), Stages: []Stage{}}
	if raw.User != nil {
		d.Author = raw.User.Username
	}
	d.DurationSec = int(math.Round(raw.Duration))

	jobs := append(raw.Jobs[:0:0], raw.Jobs...)
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].ID < jobs[j].ID })
	index := map[string]int{}
	for _, j := range jobs {
		name := j.Stage
		if strings.TrimSpace(name) == "" {
			name = "Jobs"
		}
		idx, ok := index[name]
		if !ok {
			idx = len(d.Stages)
			index[name] = idx
			d.Stages = append(d.Stages, Stage{Name: name, Jobs: []Job{}})
		}
		d.Stages[idx].Jobs = append(d.Stages[idx].Jobs, Job{
			Name:        j.Name,
			Status:      NormalizeGitLabStatus(j.Status),
			DurationSec: int(math.Round(j.Duration)),
			URL:         j.WebURL,
		})
	}
	return d, nil
}

func parseGitHubRunDetail(data []byte) (PipelineDetail, error) {
	empty := PipelineDetail{Stages: []Stage{}}
	t := bytes.TrimSpace(data)
	if pipelineIsEmpty(t) {
		return empty, pipelineUnreadable("gh")
	}
	var raw struct {
		pipelineGitHubRaw
		WorkflowName string `json:"workflowName"`
		Jobs         []struct {
			Name        string `json:"name"`
			Status      string `json:"status"`
			Conclusion  string `json:"conclusion"`
			StartedAt   string `json:"startedAt"`
			CompletedAt string `json:"completedAt"`
			URL         string `json:"url"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(t, &raw); err != nil {
		return empty, pipelineUnreadable("gh")
	}
	d := PipelineDetail{Pipeline: pipelineFromGitHubRun(raw.pipelineGitHubRaw), Stages: []Stage{}}
	if len(raw.Jobs) == 0 {
		return d, nil
	}
	name := strings.TrimSpace(raw.WorkflowName)
	if name == "" {
		name = "Jobs"
	}
	jobs := make([]Job, 0, len(raw.Jobs))
	for _, j := range raw.Jobs {
		jobs = append(jobs, Job{
			Name:        j.Name,
			Status:      NormalizeGitHubStatus(j.Status, j.Conclusion),
			DurationSec: pipelineSeconds(j.StartedAt, j.CompletedAt),
			URL:         j.URL,
		})
	}
	d.Stages = []Stage{{Name: name, Jobs: jobs}}
	return d, nil
}

// ListPipelines mengambil maksimal PipelineLimit pipeline/run terbaru.
func ListPipelines(ctx context.Context, repo Repo) ([]Pipeline, error) {
	arg := pipelineRepoArg(repo)
	var (
		list []Pipeline
		err  error
		out  []byte
	)
	switch repo.Provider {
	case ProviderGitLab:
		out, err = Run(ctx, "glab", "ci", "list", "--repo", arg, "--output", "json", "--per-page", "20")
		if err != nil {
			return []Pipeline{}, err
		}
		list, err = parseGitLabPipelines(out)
	case ProviderGitHub:
		out, err = Run(ctx, "gh", "run", "list", "--repo", arg, "--limit", "20", "--json", pipelineGitHubRunFields)
		if err != nil {
			return []Pipeline{}, err
		}
		list, err = parseGitHubRuns(out)
	default:
		return []Pipeline{}, pipelineUnsupported()
	}
	if err != nil {
		return []Pipeline{}, err
	}
	sort.SliceStable(list, func(i, j int) bool {
		return pipelineParseTime(list[i].UpdatedAt).After(pipelineParseTime(list[j].UpdatedAt))
	})
	if len(list) > PipelineLimit {
		list = list[:PipelineLimit]
	}
	if list == nil {
		list = []Pipeline{}
	}
	return list, nil
}

// GetPipeline mengambil detail satu pipeline/run beserta stage dan job.
func GetPipeline(ctx context.Context, repo Repo, id string) (PipelineDetail, error) {
	empty := PipelineDetail{Stages: []Stage{}}
	if !pipelineIDPattern.MatchString(id) {
		return empty, &Error{Code: ErrorCode("invalid_params"), Message: "ID pipeline tidak valid"}
	}
	arg := pipelineRepoArg(repo)
	switch repo.Provider {
	case ProviderGitLab:
		out, err := Run(ctx, "glab", "ci", "get", "--repo", arg, "--pipeline-id", id, "--output", "json")
		if err != nil {
			return empty, err
		}
		return parseGitLabPipelineDetail(out)
	case ProviderGitHub:
		out, err := Run(ctx, "gh", "run", "view", id, "--repo", arg, "--json", pipelineGitHubRunFields+",jobs")
		if err != nil {
			return empty, err
		}
		return parseGitHubRunDetail(out)
	}
	return empty, pipelineUnsupported()
}
