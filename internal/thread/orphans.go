package thread

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// orphansGitTimeout adalah batas waktu tiap perintah git (satu timeout baru per perintah).
const orphansGitTimeout = 20 * time.Second

type orphansResult struct {
	worktrees, branches int
	masterDir           bool
}

// CleanOrphans membersihkan sisa worktree, branch thread/<id>-*, direktori master, dan berkas .git
// worktree milik thread closed (mis. daemon crash di tengah Close). Thread lain tidak disentuh.
// Sinkron dan idempoten; cleaned = jumlah worktree + branch yang benar-benar dihapus.
func CleanOrphans(m *Manager) (cleaned int, err error) {
	var errs []error
	gitBin, gerr := worktreeGit()
	if gerr != nil {
		gitBin = ""
		m.log.Warn("thread: git tidak tersedia, pembersihan yatim hanya lewat penghapusan berkas", "err", gerr)
		errs = append(errs, gerr)
	}
	for _, t := range m.managerAllThreads() {
		if t.State != StateClosed {
			continue
		}
		res, e := orphansSafeCleanThread(m, gitBin, t)
		cleaned += res.worktrees + res.branches
		if e != nil {
			errs = append(errs, e)
		}
	}
	return cleaned, errors.Join(errs...)
}

// orphansSafeCleanThread membungkus orphansCleanThread agar panic satu thread tidak menghentikan loop.
func orphansSafeCleanThread(m *Manager, gitBin string, t Thread) (res orphansResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			m.log.Warn("thread: panic saat pembersihan yatim", "thread", t.ID, "panic", r)
			err = fmt.Errorf("panic membersihkan thread %s: %v", t.ID, r)
		}
	}()
	return orphansCleanThread(m, gitBin, t)
}

func orphansCleanThread(m *Manager, gitBin string, t Thread) (orphansResult, error) {
	var res orphansResult
	if t.ID == "" || strings.ContainsAny(t.ID, `/\`) || t.ID == "." || t.ID == ".." {
		return res, fmt.Errorf("ID thread tidak valid untuk pembersihan: %q", t.ID)
	}
	dir := m.cfg.Store.ThreadDir(t.ID)
	if _, serr := os.Stat(dir); serr != nil {
		return res, nil
	}
	var errs []error
	repos := orphansCandidateRepos(m, t, dir)
	leftovers := orphansListLeftovers(m, gitBin, repos, dir)

	for _, path := range leftovers {
		removed, e := orphansRemoveWorktree(m, gitBin, repos, dir, path)
		if e != nil {
			errs = append(errs, e)
			continue
		}
		if removed {
			res.worktrees++
		}
	}

	if gitBin != "" && len(leftovers) > 0 {
		for _, repo := range repos {
			if _, e := orphansGit(m, gitBin, repo, "worktree", "prune"); e != nil {
				m.log.Warn("thread: git worktree prune gagal", "thread", t.ID, "repo", repo, "err", e)
			}
		}
	}

	// Branch dihapus setelah worktree (git menolak branch -D untuk branch yang masih ter-checkout).
	n, e := orphansDeleteBranches(m, gitBin, repos, t.ID)
	res.branches = n
	if e != nil {
		errs = append(errs, e)
	}

	masterPath := filepath.Join(dir, "master")
	if worktreeExists(masterPath) {
		if e := orphansSafeRemoveAll(dir, masterPath); e != nil {
			m.log.Warn("thread: gagal menghapus direktori master", "thread", t.ID, "err", e)
			errs = append(errs, fmt.Errorf("hapus direktori master %s: %w", masterPath, e))
		} else {
			res.masterDir = true
			m.log.Info("thread: hapus direktori master", "thread", t.ID, "path", masterPath)
		}
	}

	if res.worktrees+res.branches > 0 || res.masterDir {
		m.log.Info("thread: pembersihan yatim", "thread", t.ID,
			"worktrees", res.worktrees, "branches", res.branches, "masterDir", res.masterDir)
	}
	return res, errors.Join(errs...)
}

// orphansIsGitRepo memeriksa path ada dan memuat .git.
func orphansIsGitRepo(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(p, ".git"))
	return err == nil
}

// orphansCandidateRepos mengumpulkan repo project kandidat (unik, urut stabil).
func orphansCandidateRepos(m *Manager, t Thread, dir string) []string {
	set := map[string]bool{}
	add := func(p string) {
		if p == "" {
			return
		}
		p = filepath.Clean(p)
		if orphansIsGitRepo(p) {
			set[p] = true
		}
	}
	recs, err := LoadWorktreeRecords(dir)
	if err != nil {
		m.log.Debug("thread: gagal membaca worktrees.json saat pembersihan", "thread", t.ID, "err", err)
	}
	for _, rec := range recs {
		add(rec.Info.ProjectPath)
	}
	if entries, rerr := os.ReadDir(dir); rerr == nil {
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "master" {
				continue
			}
			b, rerr := os.ReadFile(filepath.Join(dir, e.Name(), ".git"))
			if rerr != nil {
				continue
			}
			if repo, ok := orphansRepoFromGitFile(string(b)); ok {
				add(repo)
			}
		}
	}
	if m.cfg.Workspaces != nil {
		if projects, ok := m.cfg.Workspaces.Projects(t.WorkspaceID); ok {
			for _, p := range projects {
				add(p.Path)
			}
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// orphansListLeftovers mendaftar direktori sisa di bawah dir (bukan master, bukan berkas) dan entri
// worktree basi yang terdaftar di git di bawah dir. Tidak pernah memuat berkas.
func orphansListLeftovers(m *Manager, gitBin string, repos []string, dir string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = filepath.Clean(p)
		if seen[p] || !orphansUnder(dir, p) {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "master" {
				continue
			}
			add(filepath.Join(dir, e.Name()))
		}
	}
	if gitBin != "" {
		for _, repo := range repos {
			outp, err := orphansGit(m, gitBin, repo, "worktree", "list", "--porcelain")
			if err != nil {
				m.log.Debug("thread: git worktree list gagal", "repo", repo, "err", err)
				continue
			}
			for _, l := range worktreeLines(outp) {
				if p, ok := strings.CutPrefix(l, "worktree "); ok {
					p = strings.TrimSpace(p)
					if filepath.Clean(p) != filepath.Join(dir, "master") {
						add(p)
					}
				}
			}
		}
	}
	return out
}

// orphansRemoveWorktree menghapus satu worktree sisa. removed=false untuk entri basi yang hanya di-prune.
func orphansRemoveWorktree(m *Manager, gitBin string, repos []string, dir, path string) (bool, error) {
	if !orphansUnder(dir, path) {
		return false, fmt.Errorf("menolak menghapus worktree di luar %s: %s", dir, path)
	}
	existed := worktreeExists(path)
	if !existed {
		// entri basi: cukup di-prune oleh pemanggil.
		return false, nil
	}
	var candidates []string
	if b, err := os.ReadFile(filepath.Join(path, ".git")); err == nil {
		if repo, ok := orphansRepoFromGitFile(string(b)); ok && orphansIsGitRepo(repo) {
			candidates = []string{filepath.Clean(repo)}
		}
	}
	if len(candidates) == 0 {
		candidates = repos
	}
	if gitBin != "" {
		for _, repo := range candidates {
			if _, err := orphansGit(m, gitBin, repo, "worktree", "remove", "--force", "--force", path); err == nil {
				m.log.Info("thread: worktree yatim dihapus", "path", path, "repo", repo)
				return true, nil
			} else {
				m.log.Warn("thread: git worktree remove gagal", "path", path, "repo", repo, "err", err)
			}
		}
	}
	if err := orphansSafeRemoveAll(dir, path); err != nil {
		return false, fmt.Errorf("hapus worktree %s: %w", path, err)
	}
	m.log.Info("thread: worktree yatim dihapus lewat penghapusan berkas", "path", path)
	return true, nil
}

// orphansDeleteBranches menghapus branch thread/<id>-* di tiap repo kandidat.
func orphansDeleteBranches(m *Manager, gitBin string, repos []string, threadID string) (int, error) {
	if gitBin == "" {
		return 0, nil
	}
	prefix := worktreeBranchPrefix + threadID + "-"
	var errs []error
	count := 0
	for _, repo := range repos {
		out, err := orphansGit(m, gitBin, repo, "branch", "--list", "--format=%(refname:short)", prefix+"*")
		if err != nil {
			errs = append(errs, fmt.Errorf("daftar branch %s: %w", repo, err))
			continue
		}
		for _, name := range worktreeLines(out) {
			name = strings.TrimSpace(name)
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			if _, err := orphansGit(m, gitBin, repo, "branch", "-D", name); err != nil {
				m.log.Warn("thread: gagal menghapus branch yatim", "thread", threadID, "repo", repo, "branch", name, "err", err)
				errs = append(errs, fmt.Errorf("hapus branch %s: %w", name, err))
				continue
			}
			m.log.Info("thread: branch yatim dihapus", "thread", threadID, "repo", repo, "branch", name)
			count++
		}
	}
	return count, errors.Join(errs...)
}

// orphansGit menjalankan git dengan timeout baru per perintah.
func orphansGit(m *Manager, gitBin, repo string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), orphansGitTimeout)
	defer cancel()
	return worktreeRun(ctx, gitBin, repo, args...)
}

// orphansUnder true bila p berada di bawah base (bukan base itu sendiri).
func orphansUnder(base, p string) bool {
	if base == "" || p == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(base), filepath.Clean(p))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	return true
}

// orphansSafeRemoveAll menghapus p hanya bila di bawah base; symlink hanya dilepas, tidak diikuti.
func orphansSafeRemoveAll(base, p string) error {
	if !orphansUnder(base, p) {
		return fmt.Errorf("menolak menghapus di luar %s: %s", base, p)
	}
	fi, err := os.Lstat(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return os.Remove(p)
	}
	return os.RemoveAll(p)
}

// orphansRepoFromGitFile mengurai isi berkas .git worktree ("gitdir: <repo>/.git/worktrees/<n>").
func orphansRepoFromGitFile(content string) (string, bool) {
	line := strings.TrimSpace(strings.SplitN(content, "\n", 2)[0])
	rest, ok := strings.CutPrefix(line, "gitdir:")
	if !ok {
		return "", false
	}
	p := filepath.ToSlash(strings.TrimSpace(rest))
	const marker = "/.git/worktrees/"
	i := strings.Index(p, marker)
	if i <= 0 {
		return "", false
	}
	return filepath.FromSlash(p[:i]), true
}
