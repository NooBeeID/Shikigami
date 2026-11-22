package grimoire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NooBeeID/shikigami/internal/paths"
)

// BuildCost adalah biaya build LLM terakhir.
type BuildCost struct {
	Modules    int    `json:"modules"`
	EstTokens  int    `json:"estTokens"`
	DurationMs int64  `json:"durationMs"`
	Failed     int    `json:"failed"`
	At         string `json:"at"`
	Skipped    int    `json:"skipped,omitempty"`
	Note       string `json:"note,omitempty"`
}

// LLMProgress adalah progres peringkasan build yang sedang berjalan.
type LLMProgress struct {
	Current string `json:"current"`
	Done    int    `json:"done"`
	Failed  int    `json:"failed"`
}

// ErrLLMOverLimit berarti modul antrean melewati maxModules.
var ErrLLMOverLimit = errors.New("modul di atas maxModules")

const (
	summarizeModuleTimeout = 120 * time.Second
	// summarizeDefaultModel adalah model murah bawaan; env SHIKIGAMI_GRIMOIRE_MODEL
	// yang tak kosong menimpanya.
	summarizeDefaultModel   = "haiku"
	summarizeModelEnv       = "SHIKIGAMI_GRIMOIRE_MODEL"
	summarizeMaxOutputBytes = 1 << 20
	summarizeCostFile       = "lastbuild.json"
	summarizeTruncMarker    = "[...dipotong...]"
	summarizeMaxFileBytes   = LLMMaxSourceBytes
)

var (
	summarizeRuntime atomic.Pointer[LLMDeps]
	summarizeActive  = struct {
		sync.Mutex
		m map[string]*llmSummarizer
	}{m: map[string]*llmSummarizer{}}
	summarizeHashRe = regexp.MustCompile(`(?m)^## Hash\n(\S+)`)
)

// SetLLMRuntime menyimpan runtime untuk SummarizerFor.
func SetLLMRuntime(d LLMDeps) {
	c := d
	summarizeRuntime.Store(&c)
}

// LoadBuildCost membaca lastbuild.json; nil, nil bila belum ada.
func LoadBuildCost(projectID string) (*BuildCost, error) {
	dd, err := paths.DataDir()
	if err != nil {
		return nil, err
	}
	b, err := NewStore(dd).ReadFile(projectID, summarizeCostFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var c BuildCost
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("lastbuild.json rusak: %w", err)
	}
	return &c, nil
}

// ProgressFor mengembalikan salinan progres instance aktif project, nil bila tak ada.
func ProgressFor(projectID string) *LLMProgress {
	summarizeActive.Lock()
	s := summarizeActive.m[projectID]
	summarizeActive.Unlock()
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.progress
	return &p
}

// SummarizerFor mengembalikan peringkas LLM, atau nil murni untuk mode statik.
func SummarizerFor(projectID, projectPath string) Summarizer {
	rt := summarizeRuntime.Load()
	if rt == nil {
		return nil
	}
	s, err := LoadSettings(projectID)
	if err != nil || !s.LLM {
		return nil
	}
	if _, err := exec.LookPath("claude"); err != nil {
		if rt.Logger != nil {
			rt.Logger.Info("grimoire: claude tidak terpasang, mode statik", "project", projectID)
		}
		return nil
	}
	return NewLLMSummarizer(*rt, projectID, projectPath, s)
}

// NewLLMSummarizer membuat satu instance peringkas untuk satu build.
func NewLLMSummarizer(deps LLMDeps, projectID, projectPath string, s Settings) Summarizer {
	return &llmSummarizer{deps: deps, projectID: projectID, projectPath: projectPath, settings: s, start: time.Now()}
}

type llmSummarizer struct {
	deps        LLMDeps
	projectID   string
	projectPath string
	settings    Settings
	start       time.Time

	mu          sync.Mutex
	unavailable bool
	note        string
	queued      int
	cost        BuildCost
	progress    LLMProgress
}

func (s *llmSummarizer) logger() *slog.Logger {
	if s.deps.Logger != nil {
		return s.deps.Logger
	}
	return slog.Default()
}

func (s *llmSummarizer) publish() {
	if s.deps.Publish != nil {
		s.deps.Publish(s.projectID)
	}
}

// saveCost menulis lastbuild.json (dipanggil tanpa memegang mu).
func (s *llmSummarizer) saveCost(c BuildCost) {
	c.DurationMs = time.Since(s.start).Milliseconds()
	c.At = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	st := s.deps.Store
	if st == nil {
		dd, derr := paths.DataDir()
		if derr != nil {
			s.logger().Warn("grimoire: gagal simpan biaya", "err", derr)
			return
		}
		st = NewStore(dd)
	}
	if err := st.WriteFile(s.projectID, summarizeCostFile, data); err != nil {
		s.logger().Warn("grimoire: gagal simpan biaya", "project", s.projectID, "err", err)
	}
}

// snapshot menyalin biaya berjalan (dengan Note) di bawah mu.
func (s *llmSummarizer) snapshot() BuildCost {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.cost
	if s.note != "" {
		c.Note = s.note
	}
	return c
}

func (s *llmSummarizer) register() {
	summarizeActive.Lock()
	summarizeActive.m[s.projectID] = s
	summarizeActive.Unlock()
}

// summarizeStaticChapters mengembalikan semua nama bab dengan nilai kosong.
// BuildIncremental mempertahankan halaman statik (beserta footer hash) bila
// nilai kunci kosong, jadi nilai kosong = fallback statik.
func summarizeStaticChapters(names []string) map[string]string {
	m := make(map[string]string, len(names))
	for _, n := range names {
		m[n] = ""
	}
	return m
}

func (s *llmSummarizer) Summarize(ctx context.Context, m Module, static string) (map[string]string, error) {
	_, pages := RenderBooks([]Module{m})
	names := make([]string, 0, len(pages))
	for _, p := range pages {
		names = append(names, strings.TrimSuffix(filepath.Base(p.Path), ".md"))
	}
	target := ""
	snake := Snake(m.Name)
	for _, n := range names {
		if n == snake {
			target = n
			break
		}
	}
	if target == "" && len(names) > 0 {
		sorted := append([]string(nil), names...)
		for i := 1; i < len(sorted); i++ {
			for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
				sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			}
		}
		target = sorted[0]
	}
	hash := ""
	if mm := summarizeHashRe.FindStringSubmatch(static); mm != nil {
		hash = mm[1]
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	statics := summarizeStaticChapters(names)

	s.mu.Lock()
	s.queued++
	over := s.settings.MaxModules != nil && s.queued > *s.settings.MaxModules
	if over {
		s.cost.Skipped++
	}
	unavailable := s.unavailable
	s.mu.Unlock()

	if over {
		s.saveCost(s.snapshot())
		s.publish()
		known := false
		if s.deps.Store != nil {
			if st, err := s.deps.Store.ReadState(s.projectID); err == nil {
				_, known = st.Modules[snake]
			}
		}
		if known {
			return nil, ErrLLMOverLimit
		}
		return statics, nil
	}
	if unavailable {
		return statics, nil
	}

	s.mu.Lock()
	s.progress.Current = snake
	s.mu.Unlock()
	s.register()
	s.publish()

	sources := summarizeLoadSources(s.projectPath, m.Sources)
	prompt := promptBuild(m, static, sources)

	rctx, cancel := context.WithTimeout(ctx, summarizeModuleTimeout)
	out, stderr, err := summarizeRunClaude(rctx, s.projectPath, prompt)
	timedOut := errors.Is(rctx.Err(), context.DeadlineExceeded)
	cancel()
	if cerr := ctx.Err(); cerr != nil {
		s.mu.Lock()
		s.progress.Current = ""
		s.mu.Unlock()
		return nil, cerr
	}

	if err != nil && strings.TrimSpace(out) == "" && !timedOut {
		low := strings.ToLower(stderr)
		var ee *exec.ExitError
		notExec := !errors.As(err, &ee)
		authHit := false
		for _, k := range []string{"login", "auth", "credential", "api key", "not logged in", "unauthorized"} {
			if strings.Contains(low, k) {
				authHit = true
				break
			}
		}
		if notExec || authHit {
			note := "belum login: mode statik"
			if notExec {
				note = "claude tidak terpasang: mode statik"
			}
			s.logger().Warn("grimoire: claude tidak siap, mode statik", "project", s.projectID, "note", note)
			s.mu.Lock()
			s.unavailable = true
			s.note = note
			s.progress.Current = ""
			s.mu.Unlock()
			s.saveCost(s.snapshot())
			s.publish()
			return statics, nil
		}
	}

	output := strings.TrimSpace(out)
	if err != nil || output == "" {
		s.mu.Lock()
		s.cost.Failed++
		s.progress.Failed++
		s.progress.Current = ""
		s.cost.EstTokens += EstTokensFromBytes(len(prompt))
		s.mu.Unlock()
		s.saveCost(s.snapshot())
		s.publish()
		switch {
		case timedOut:
			return nil, fmt.Errorf("peringkas melewati batas waktu %s untuk modul %s", summarizeModuleTimeout, m.Name)
		case err != nil:
			return nil, fmt.Errorf("peringkas gagal untuk modul %s: %w", m.Name, err)
		default:
			return nil, fmt.Errorf("peringkas mengembalikan keluaran kosong untuk modul %s", m.Name)
		}
	}

	output = s.summarizeValidate(m, output, hash)

	s.mu.Lock()
	s.cost.Modules++
	s.progress.Done++
	s.progress.Current = ""
	s.cost.EstTokens += EstTokensFromBytes(len(prompt) + len(output))
	s.mu.Unlock()
	s.saveCost(s.snapshot())
	s.publish()

	statics[target] = output
	return statics, nil
}

// summarizeValidate memastikan baris pertama Sumber: dan footer hash.
func (s *llmSummarizer) summarizeValidate(m Module, out, hash string) string {
	out = strings.TrimLeft(out, " \t\r\n")
	if strings.HasPrefix(out, "```") && strings.HasSuffix(strings.TrimRight(out, " \t\r\n"), "```") {
		if i := strings.Index(out, "\n"); i >= 0 {
			out = strings.TrimSpace(out[i+1:])
			out = strings.TrimSpace(strings.TrimSuffix(out, "```"))
		}
	}
	if !strings.HasPrefix(out, "Sumber:") {
		srcs := m.Sources
		if s.deps.Store != nil {
			if st, err := s.deps.Store.ReadState(s.projectID); err == nil {
				if ms, ok := st.Modules[Snake(m.Name)]; ok && len(ms.Sources) > 0 {
					srcs = ms.Sources
				}
			}
		}
		line := "Sumber: -"
		if len(srcs) > 0 {
			line = "Sumber: " + strings.Join(srcs, ", ")
		}
		out = line + "\n" + out
	}
	if hash != "" && !strings.Contains(out, "## Hash") {
		out = strings.TrimRight(out, "\n") + "\n\n## Hash\n" + hash + "\n"
	}
	return out
}

// summarizeLoadSources membaca sumber modul berurutan dengan batas total.
func summarizeLoadSources(projectPath string, srcs []string) []promptSource {
	root, err := filepath.EvalSymlinks(projectPath)
	if err != nil {
		return nil
	}
	var out []promptSource
	total := 0
	for _, rel := range srcs {
		clean := filepath.Clean(rel)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			continue
		}
		abs, err := filepath.EvalSymlinks(filepath.Join(root, clean))
		if err != nil {
			continue
		}
		if r, err := filepath.Rel(root, abs); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			continue
		}
		fi, err := os.Stat(abs)
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		if fi.Size() > summarizeMaxFileBytes {
			out = append(out, promptSource{Path: rel, Note: "dilewati: terlalu besar"})
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		head := data
		if len(head) > 8192 {
			head = head[:8192]
		}
		if bytes.IndexByte(head, 0) >= 0 {
			out = append(out, promptSource{Path: rel, Note: "dilewati: biner"})
			continue
		}
		if total+len(data) > LLMMaxSourceBytes {
			room := LLMMaxSourceBytes - total
			if room > 0 {
				out = append(out, promptSource{Path: rel, Content: string(data[:room]) + "\n" + summarizeTruncMarker})
			} else {
				out = append(out, promptSource{Path: rel, Content: summarizeTruncMarker})
			}
			break
		}
		total += len(data)
		out = append(out, promptSource{Path: rel, Content: string(data)})
	}
	return out
}

// summarizeLimitWriter membuang byte melebihi batas.
type summarizeLimitWriter struct {
	buf bytes.Buffer
	max int
}

func (w *summarizeLimitWriter) Write(p []byte) (int, error) {
	if room := w.max - w.buf.Len(); room > 0 {
		if len(p) > room {
			w.buf.Write(p[:room])
		} else {
			w.buf.Write(p)
		}
	}
	return len(p), nil
}

// summarizeRunClaude menjalankan `claude -p` dengan prompt lewat stdin.
// Flag diverifikasi dari `claude -p --help`: --output-format text, --model,
// dan --tools "" (menonaktifkan semua tool bawaan). Tanpa flag izin berbahaya.
func summarizeRunClaude(ctx context.Context, projectPath, prompt string) (string, string, error) {
	bin, err := exec.LookPath("claude")
	if err != nil {
		return "", "", err
	}
	model := summarizeDefaultModel
	if v := strings.TrimSpace(os.Getenv(summarizeModelEnv)); v != "" {
		model = v
	}
	cmd := exec.CommandContext(ctx, bin, "-p", "--output-format", "text", "--model", model, "--tools", "")
	cmd.Env = os.Environ()
	cmd.Dir = projectPath
	cmd.Stdin = strings.NewReader(prompt)
	stdout := &summarizeLimitWriter{max: summarizeMaxOutputBytes}
	stderr := &summarizeLimitWriter{max: summarizeMaxOutputBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = 3 * time.Second
	err = cmd.Run()
	return stdout.buf.String(), stderr.buf.String(), err
}
