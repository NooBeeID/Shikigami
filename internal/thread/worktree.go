package thread

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	worktreeCreateTimeout  = 30 * time.Second
	worktreeInspectTimeout = 15 * time.Second
	worktreeRemoveTimeout  = 30 * time.Second
	worktreeMaxBranchTries = 50
	worktreeBranchPrefix   = "thread/"
	worktreeSlugMax        = 40
)

// WorktreeInfo menggambarkan worktree git milik thread.
type WorktreeInfo struct {
	Path        string `json:"path"`
	Branch      string `json:"branch"`
	BaseBranch  string `json:"baseBranch"` // nama branch dasar; "" bila HEAD detached
	BaseCommit  string `json:"baseCommit"` // hash HEAD saat spawn (selalu terisi)
	ProjectPath string `json:"projectPath"`
}

// WorktreeStatus adalah hasil pemeriksaan worktree.
type WorktreeStatus struct {
	Dirty      []string `json:"dirty"`    // baris `git status --porcelain`; selalu [] bukan null
	Unmerged   []string `json:"unmerged"` // "<hash> <subject>" commit yang belum di BaseBranch; selalu []
	BaseBranch string   `json:"baseBranch"`
}

// ErrNotGitRepo dikembalikan bila project bukan repo git.
var ErrNotGitRepo = errors.New("project bukan repo git; thread tidak bisa mengisolasinya")

// Slugify mengubah nama project menjadi slug aman untuk nama direktori dan branch:
// huruf kecil, [a-z0-9-], tanpa '-' di ujung; kosong -> "project"; "master" -> "master-project".
func Slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > worktreeSlugMax {
		s = strings.Trim(s[:worktreeSlugMax], "-")
	}
	if s == "" {
		return "project"
	}
	if s == "master" {
		return "master-project"
	}
	return s
}

// worktreeGit mencari binary git di PATH.
func worktreeGit() (string, error) {
	p, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("binary git tidak ditemukan di PATH; pasang git agar thread bisa membuat worktree: %w", err)
	}
	return p, nil
}

// worktreeRun menjalankan git -C dir args... dan mengembalikan stdout (tanpa newline akhir).
// Galat membungkus galat asli (mis. *exec.ExitError) dan menyertakan stderr.
func worktreeRun(ctx context.Context, gitBin, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, gitBin, append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("git %s melewati batas waktu", args[0])
		}
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\r\n"), nil
}

// worktreeExitCode mengembalikan kode exit bila err berasal dari exit non-zero git.
func worktreeExitCode(err error) (int, bool) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), true
	}
	return 0, false
}

// worktreeBranchExists memeriksa apakah refs/heads/<branch> ada di repo.
func worktreeBranchExists(ctx context.Context, gitBin, repo, branch string) (bool, error) {
	_, err := worktreeRun(ctx, gitBin, repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}
	if code, ok := worktreeExitCode(err); ok && code == 1 {
		return false, nil
	}
	return false, err
}

// worktreeLines memecah keluaran menjadi baris tak kosong; selalu non-nil.
func worktreeLines(s string) []string {
	out := []string{}
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimRight(l, "\r")
		if l == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

// worktreeSafeToDelete menjaga agar hanya milik thread yang dihapus.
func worktreeSafeToDelete(info WorktreeInfo) bool {
	return info.Path != "" &&
		filepath.IsAbs(info.Path) &&
		filepath.Clean(info.Path) != string(filepath.Separator) &&
		strings.HasPrefix(info.Branch, worktreeBranchPrefix)
}

// worktreeExists memeriksa keberadaan path di disk.
func worktreeExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Lstat(p)
	return err == nil
}

// CreateWorktree membuat worktree <dataDir>/threads/<threadID>/<slug> dengan branch
// thread/<threadID>-<slug> dari HEAD project. Bentrok nama branch diatasi sufiks -2, -3, dst.
func CreateWorktree(projectPath, dataDir, threadID, projectSlug string) (WorktreeInfo, error) {
	if projectPath == "" || dataDir == "" || threadID == "" || projectSlug == "" {
		return WorktreeInfo{}, errors.New("argumen worktree tidak lengkap: projectPath, dataDir, threadID, dan projectSlug wajib diisi")
	}
	if strings.ContainsAny(threadID, `/\`) || strings.Contains(threadID, "..") {
		return WorktreeInfo{}, fmt.Errorf("threadID tidak valid: %q", threadID)
	}
	slug := Slugify(projectSlug)
	projectAbs, err := filepath.Abs(projectPath)
	if err != nil {
		return WorktreeInfo{}, fmt.Errorf("path project tidak valid: %w", err)
	}
	dataAbs, err := filepath.Abs(dataDir)
	if err != nil {
		return WorktreeInfo{}, fmt.Errorf("path data tidak valid: %w", err)
	}
	gitBin, err := worktreeGit()
	if err != nil {
		return WorktreeInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), worktreeCreateTimeout)
	defer cancel()

	out, err := worktreeRun(ctx, gitBin, projectAbs, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		if _, ok := worktreeExitCode(err); ok {
			return WorktreeInfo{}, ErrNotGitRepo
		}
		return WorktreeInfo{}, err
	}
	if strings.TrimSpace(out) != "true" {
		return WorktreeInfo{}, ErrNotGitRepo
	}
	baseCommit, err := worktreeRun(ctx, gitBin, projectAbs, "rev-parse", "HEAD")
	if err != nil {
		if _, ok := worktreeExitCode(err); ok {
			return WorktreeInfo{}, errors.New("repo project belum punya commit; buat commit pertama sebelum melibatkan project ini")
		}
		return WorktreeInfo{}, err
	}
	baseCommit = strings.TrimSpace(baseCommit)
	baseBranch := ""
	if abbrev, err := worktreeRun(ctx, gitBin, projectAbs, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		if abbrev = strings.TrimSpace(abbrev); abbrev != "HEAD" {
			baseBranch = abbrev
		}
	}

	path := filepath.Join(dataAbs, "threads", threadID, slug)
	if worktreeExists(path) {
		return WorktreeInfo{}, fmt.Errorf("worktree sudah ada di %s", path)
	}

	cleanup := func() {
		if worktreeExists(path) {
			_ = os.RemoveAll(path)
		}
		_, _ = worktreeRun(ctx, gitBin, projectAbs, "worktree", "prune")
	}

	for i := 1; i <= worktreeMaxBranchTries; i++ {
		name := worktreeBranchPrefix + threadID + "-" + slug
		if i > 1 {
			name += "-" + strconv.Itoa(i)
		}
		exists, err := worktreeBranchExists(ctx, gitBin, projectAbs, name)
		if err != nil {
			cleanup()
			return WorktreeInfo{}, err
		}
		if exists {
			continue
		}
		_, err = worktreeRun(ctx, gitBin, projectAbs, "worktree", "add", "-b", name, path, baseCommit)
		if err == nil {
			return WorktreeInfo{
				Path:        path,
				Branch:      name,
				BaseBranch:  baseBranch,
				BaseCommit:  baseCommit,
				ProjectPath: projectAbs,
			}, nil
		}
		msg := err.Error()
		if strings.Contains(msg, "already exists") || strings.Contains(msg, "already used") || strings.Contains(msg, "is already checked out") {
			cleanup()
			continue
		}
		cleanup()
		return WorktreeInfo{}, fmt.Errorf("membuat worktree: %w", err)
	}
	return WorktreeInfo{}, fmt.Errorf("tidak menemukan nama branch bebas setelah %d percobaan", worktreeMaxBranchTries)
}

// InspectWorktree melaporkan perubahan belum di-commit (Dirty) dan commit yang belum
// masuk ke branch dasar atau BaseCommit (Unmerged).
func InspectWorktree(info WorktreeInfo) (WorktreeStatus, error) {
	st := WorktreeStatus{Dirty: []string{}, Unmerged: []string{}, BaseBranch: info.BaseBranch}
	if info.ProjectPath == "" || (info.Path == "" && info.Branch == "") {
		return st, errors.New("info worktree tidak lengkap")
	}
	gitBin, err := worktreeGit()
	if err != nil {
		return st, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), worktreeInspectTimeout)
	defer cancel()

	pathExists := worktreeExists(info.Path)
	if pathExists {
		out, err := worktreeRun(ctx, gitBin, info.Path, "--no-optional-locks", "status", "--porcelain")
		if err != nil {
			return st, fmt.Errorf("inspeksi worktree %s: %w", info.Path, err)
		}
		st.Dirty = worktreeLines(out)
	}

	base := ""
	if info.BaseBranch != "" {
		if ok, err := worktreeBranchExists(ctx, gitBin, info.ProjectPath, info.BaseBranch); err == nil && ok {
			base = "refs/heads/" + info.BaseBranch
		}
	}
	if base == "" {
		base = info.BaseCommit
	}
	if base == "" {
		return st, nil
	}

	var out string
	switch {
	case pathExists:
		out, err = worktreeRun(ctx, gitBin, info.Path, "log", "--no-color", "--format=%h %s", base+"..HEAD")
	case info.Branch != "":
		ok, berr := worktreeBranchExists(ctx, gitBin, info.ProjectPath, info.Branch)
		if berr != nil {
			return st, fmt.Errorf("inspeksi worktree %s: %w", info.Branch, berr)
		}
		if !ok {
			return st, nil
		}
		out, err = worktreeRun(ctx, gitBin, info.ProjectPath, "log", "--no-color", "--format=%h %s", base+"..refs/heads/"+info.Branch)
	default:
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("inspeksi worktree %s: %w", info.Path, err)
	}
	st.Unmerged = worktreeLines(out)
	return st, nil
}

// RemoveWorktree menghapus worktree, branch thread, dan melakukan prune. Idempoten.
func RemoveWorktree(info WorktreeInfo) error {
	if info.Path == "" && info.Branch == "" {
		return nil
	}
	if !worktreeSafeToDelete(info) {
		return errors.New("info worktree tidak aman dihapus")
	}
	gitBin, err := worktreeGit()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), worktreeRemoveTimeout)
	defer cancel()

	if _, err := os.Stat(info.ProjectPath); info.ProjectPath == "" || err != nil {
		if err := os.RemoveAll(info.Path); err != nil {
			return fmt.Errorf("menghapus direktori worktree: %w", err)
		}
		return nil
	}

	var errs []error
	if worktreeExists(info.Path) {
		_, err := worktreeRun(ctx, gitBin, info.ProjectPath, "worktree", "remove", "--force", "--force", info.Path)
		if err != nil && !strings.Contains(err.Error(), "not a working tree") {
			errs = append(errs, err)
			if rerr := os.RemoveAll(info.Path); rerr != nil {
				errs = append(errs, fmt.Errorf("menghapus direktori worktree: %w", rerr))
			}
		}
	}
	if worktreeExists(info.Path) {
		if err := os.RemoveAll(info.Path); err != nil {
			errs = append(errs, fmt.Errorf("menghapus direktori worktree: %w", err))
		}
	}
	if _, err := worktreeRun(ctx, gitBin, info.ProjectPath, "worktree", "prune"); err != nil {
		errs = append(errs, err)
	}
	if info.Branch != "" {
		ok, err := worktreeBranchExists(ctx, gitBin, info.ProjectPath, info.Branch)
		if err != nil {
			errs = append(errs, err)
		} else if ok {
			if _, err := worktreeRun(ctx, gitBin, info.ProjectPath, "branch", "-D", info.Branch); err != nil && !strings.Contains(err.Error(), "not found") {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
