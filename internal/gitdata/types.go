package gitdata

import "strings"

// Provider adalah penyedia hosting git.
type Provider string

const (
	ProviderGitLab Provider = "gitlab"
	ProviderGitHub Provider = "github"
)

// RepoProject adalah project workspace yang memakai satu repo.
type RepoProject struct {
	ID   string `json:"id"`   // id project workspace
	Name string `json:"name"` // nama project workspace
}

// Repo adalah satu repo remote, digabung per host/path.
type Repo struct {
	ProjectID   string        `json:"projectId"`   // project pertama
	ProjectName string        `json:"projectName"` // nama project pertama
	Projects    []RepoProject `json:"projects"`    // semua project pemakai repo
	Provider    Provider      `json:"provider"`    // gitlab atau github
	Host        string        `json:"host"`        // mis. gitlab.com
	Path        string        `json:"path"`        // owner/repo (subgroup utuh)
	WebURL      string        `json:"webUrl"`      // https://host/path
}

// Status adalah status ternormalisasi MR/pipeline/job.
type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
	StatusSkipped  Status = "skipped"
	StatusUnknown  Status = "unknown"
)

// MergeRequest adalah MR (GitLab) atau PR (GitHub) ternormalisasi.
type MergeRequest struct {
	Number         int    `json:"number"`                   // nomor MR/PR
	Title          string `json:"title"`                    // judul
	Author         string `json:"author"`                   // penulis
	SourceBranch   string `json:"sourceBranch"`             // branch sumber
	TargetBranch   string `json:"targetBranch"`             // branch tujuan
	State          string `json:"state"`                    // "open"|"merged"|"closed"
	Draft          bool   `json:"draft"`                    // draf
	ReviewState    string `json:"reviewState,omitempty"`    // opsional
	PipelineStatus Status `json:"pipelineStatus,omitempty"` // opsional
	UpdatedAt      string `json:"updatedAt"`                // RFC3339
	URL            string `json:"url"`                      // tautan web
}

// Reviewer adalah reviewer MR/PR.
type Reviewer struct {
	Name  string `json:"name"`            // nama
	State string `json:"state,omitempty"` // opsional
}

// PipelineRef adalah rujukan ringkas ke pipeline.
type PipelineRef struct {
	ID     string `json:"id"`            // id pipeline/run
	Status Status `json:"status"`        // status
	URL    string `json:"url,omitempty"` // opsional
}

// MergeRequestDetail adalah detail MR/PR.
type MergeRequestDetail struct {
	MergeRequest
	Description  string       `json:"description"`            // deskripsi
	Labels       []string     `json:"labels"`                 // label
	Reviewers    []Reviewer   `json:"reviewers"`              // reviewer
	Mergeable    string       `json:"mergeable,omitempty"`    // opsional
	LastPipeline *PipelineRef `json:"lastPipeline,omitempty"` // opsional
}

// Pipeline adalah pipeline (GitLab) atau run (GitHub).
type Pipeline struct {
	ID          string `json:"id"`               // id
	Ref         string `json:"ref"`              // branch/tag
	SHA         string `json:"sha"`              // commit
	Status      Status `json:"status"`           // status
	Author      string `json:"author,omitempty"` // opsional
	DurationSec int    `json:"durationSec"`      // durasi detik
	UpdatedAt   string `json:"updatedAt"`        // RFC3339
	URL         string `json:"url"`              // tautan web
}

// Job adalah satu job pipeline.
type Job struct {
	Name        string `json:"name"`          // nama job
	Status      Status `json:"status"`        // status
	DurationSec int    `json:"durationSec"`   // durasi detik
	URL         string `json:"url,omitempty"` // opsional
}

// Stage adalah kelompok job.
type Stage struct {
	Name string `json:"name"` // nama stage
	Jobs []Job  `json:"jobs"` // job di stage
}

// PipelineDetail adalah pipeline beserta stage dan job.
type PipelineDetail struct {
	Pipeline
	Stages []Stage `json:"stages"` // stage
}

// NormalizeGitLabStatus memetakan status GitLab ke Status.
func NormalizeGitLabStatus(s string) Status {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "created", "waiting_for_resource", "preparing", "pending", "manual", "scheduled":
		return StatusPending
	case "running":
		return StatusRunning
	case "success":
		return StatusSuccess
	case "failed":
		return StatusFailed
	case "canceled", "cancelled":
		return StatusCanceled
	case "skipped":
		return StatusSkipped
	}
	return StatusUnknown
}

// NormalizeGitHubStatus memetakan status/conclusion GitHub ke Status.
// Bila status kosong tetapi conclusion terisi, dianggap "completed".
func NormalizeGitHubStatus(status, conclusion string) Status {
	st := strings.ToLower(strings.TrimSpace(status))
	co := strings.ToLower(strings.TrimSpace(conclusion))
	if st == "" && co != "" {
		st = "completed"
	}
	switch st {
	case "queued", "waiting", "requested", "pending":
		return StatusPending
	case "in_progress":
		return StatusRunning
	case "completed":
		switch co {
		case "success":
			return StatusSuccess
		case "failure", "timed_out", "startup_failure":
			return StatusFailed
		case "cancelled", "canceled":
			return StatusCanceled
		case "skipped", "neutral":
			return StatusSkipped
		}
	}
	return StatusUnknown
}
