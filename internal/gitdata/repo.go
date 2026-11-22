package gitdata

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ProjectInput adalah project workspace yang diperiksa remote-nya.
type ProjectInput struct{ ID, Name, Path string }

// parseRemoteURL mengurai URL remote git menjadi host dan path (owner/repo).
func parseRemoteURL(raw string) (host, path string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", "", false
		}
		host, path = u.Hostname(), u.Path
	} else {
		i := strings.Index(raw, ":")
		if i < 0 {
			return "", "", false
		}
		left := raw[:i]
		if left == "" || len(left) == 1 || strings.ContainsAny(left, `/\`) {
			return "", "", false
		}
		if at := strings.LastIndex(left, "@"); at >= 0 {
			left = left[at+1:]
		}
		host, path = left, raw[i+1:]
	}
	host = strings.ToLower(strings.TrimSpace(host))
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")
	if host == "" || len(strings.Split(path, "/")) < 2 {
		return "", "", false
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			return "", "", false
		}
	}
	return host, path, true
}

// readRemoteURL membaca URL remote (origin bila ada, jika tidak remote pertama).
func readRemoteURL(ctx context.Context, dir string) (string, bool) {
	out, err := Run(ctx, "git", "-C", dir, "remote")
	if err != nil {
		return "", false
	}
	var names []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			names = append(names, l)
		}
	}
	if len(names) == 0 {
		return "", false
	}
	name := names[0]
	for _, n := range names {
		if n == "origin" {
			name = n
			break
		}
	}
	out, err = Run(ctx, "git", "-C", dir, "remote", "get-url", name)
	if err != nil {
		return "", false
	}
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l, true
		}
	}
	return "", false
}

const hostCacheTTL = 60 * time.Second

type loginHosts struct{ gitlab, github map[string]bool }

var hostCache struct {
	mu    sync.Mutex
	at    time.Time
	val   loginHosts
	valid bool
}

var loggedInRe = regexp.MustCompile(`(?i)logged in to\s+(\S+)`)

// parseLoginHosts mengekstrak host login dari keluaran `auth status`.
func parseLoginHosts(text string) []string {
	var hosts []string
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		low := strings.ToLower(line)
		if strings.Contains(low, "not logged in") || strings.Contains(low, "not logged into") {
			continue
		}
		m := loggedInRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		h := strings.ToLower(strings.Trim(m[1], ".,;:()"))
		if h != "" && !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}
	return hosts
}

func loginHostsOf(ctx context.Context, tool string) map[string]bool {
	set := map[string]bool{}
	out, err := Run(ctx, tool, "auth", "status")
	text := string(out)
	if err != nil {
		text = err.Error()
	}
	for _, h := range parseLoginHosts(text) {
		set[h] = true
	}
	return set
}

func cachedLoginHosts(ctx context.Context) loginHosts {
	hostCache.mu.Lock()
	defer hostCache.mu.Unlock()
	if hostCache.valid && time.Since(hostCache.at) < hostCacheTTL {
		return hostCache.val
	}
	val := loginHosts{gitlab: loginHostsOf(ctx, "glab"), github: loginHostsOf(ctx, "gh")}
	if ctx.Err() == nil {
		hostCache.val, hostCache.at, hostCache.valid = val, time.Now(), true
	}
	return val
}

func providerFor(host string, hosts func() loginHosts) (Provider, bool) {
	switch host {
	case "github.com":
		return ProviderGitHub, true
	case "gitlab.com":
		return ProviderGitLab, true
	}
	h := hosts()
	if h.gitlab[host] {
		return ProviderGitLab, true
	}
	if h.github[host] {
		return ProviderGitHub, true
	}
	return "", false
}

type remoteRef struct {
	host, path string
	ok         bool
}

// DetectRepos mendeteksi repo dari remote git tiap project, digabung per host/path.
func DetectRepos(ctx context.Context, projects []ProjectInput) ([]Repo, error) {
	repos := []Repo{}
	if len(projects) == 0 {
		return repos, nil
	}
	refs := make([]remoteRef, len(projects))
	var wg sync.WaitGroup
	for i, p := range projects {
		if p.Path == "" {
			continue
		}
		wg.Add(1)
		go func(i int, dir string) {
			defer wg.Done()
			raw, ok := readRemoteURL(ctx, dir)
			if !ok {
				return
			}
			h, pa, ok := parseRemoteURL(raw)
			refs[i] = remoteRef{host: h, path: pa, ok: ok}
		}(i, p.Path)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return repos, classify(ctx.Err(), "git")
	}

	var once sync.Once
	var hosts loginHosts
	lazyHosts := func() loginHosts {
		once.Do(func() { hosts = cachedLoginHosts(ctx) })
		return hosts
	}
	index := map[string]int{}
	for i, p := range projects {
		r := refs[i]
		if !r.ok {
			continue
		}
		prov, ok := providerFor(r.host, lazyHosts)
		if !ok {
			continue
		}
		key := r.host + "/" + r.path
		idx, exists := index[key]
		if !exists {
			idx = len(repos)
			index[key] = idx
			repos = append(repos, Repo{
				ProjectID: p.ID, ProjectName: p.Name,
				Projects: []RepoProject{},
				Provider: prov, Host: r.host, Path: r.path,
				WebURL: "https://" + r.host + "/" + r.path,
			})
		}
		repos[idx].Projects = append(repos[idx].Projects, RepoProject{ID: p.ID, Name: p.Name})
	}
	if ctx.Err() != nil {
		return repos, classify(ctx.Err(), "git")
	}
	return repos, nil
}

// RepoFlag mengembalikan nilai -R: PATH untuk host bawaan, HOST/PATH untuk self-hosted.
func RepoFlag(r Repo) string {
	if r.Host == "gitlab.com" || r.Host == "github.com" {
		return r.Path
	}
	return r.Host + "/" + r.Path
}
